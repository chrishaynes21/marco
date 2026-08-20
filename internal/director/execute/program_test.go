package execute

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// These tests drive the real Pipeline over the existing scripted-world harness, so
// they exercise the actual sequential loop rather than a model of it.
//
// The fixtures are already exactly right for this milestone: menuBar() is a Notepad
// with File/Edit/View and NO Save; menuOpen() is the same window with the File menu
// revealed and Save present. "Open File then click Save" is therefore a real test of
// the central claim — Save does not exist in the world until File has been clicked, so
// it cannot be resolved before.

// scenes builds a run of worlds with INCREASING timestamps.
//
// Every scene needs its own moment: the pipeline refuses to verify against an
// after-state that is not newer than the before-state, which is a real guard (comparing
// a snapshot against itself would "verify" anything) and which a fixture reusing one
// timestamp trips immediately.
func scenes(obs ...[]directorapi.Observation) []directorapi.WorldState {
	out := make([]directorapi.WorldState, 0, len(obs))
	for i, o := range obs {
		out = append(out, scene(t0.Add(time.Duration(i+1)*time.Second), nil, o...))
	}
	return out
}

// menuFlow is what a real menu does: closed, opened by the first click, still open
// while the second step looks, and closed again by the second click.
//
// The world CHANGING is not decoration. A click into a world that does not change is
// correctly reported as unverified — "the click produced no observable change" — so a
// fixture that held the screen still would be testing the failure path while claiming
// to test the success path.
func menuFlow() []directorapi.WorldState {
	return scenes(menuBar(), menuOpen(), menuOpen(), menuBar())
}

// menuDismissed is the same opening click, followed by the menu closing on its own —
// so the second step observes a world with no Save in it and cannot resolve.
func menuDismissed() []directorapi.WorldState {
	return scenes(menuBar(), menuOpen(), menuBar(), menuBar())
}

func testIntent(s string) directorapi.Intent { return intent.New().Parse(s) }

// twoStep decomposes a request and fails the test if it is not two steps.
func twoStep(t *testing.T, request string) program.Program {
	t.Helper()
	prog, err := program.Decompose(request, testIntent)
	if err != nil {
		t.Fatalf("decompose %q: %v", request, err)
	}
	if len(prog.Steps) != 2 {
		t.Fatalf("%q produced %d steps, want 2", request, len(prog.Steps))
	}
	return prog
}

func TestEachStepResolvesAgainstAFreshlyObservedWorld(t *testing.T) {
	// The world CHANGES between the steps: Save appears only after File is clicked.
	// An implementation that resolved both targets up front could not find it.
	h := newHarness(menuFlow()...)

	out := h.pipeline.RunProgram(context.Background(),
		twoStep(t, "open File then click Save"), program.Context{}, 0)

	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	// "Open File" is a SEMANTIC action now, so it does not click anything: it asks the
	// menu item to activate. Only the explicit "click Save" reaches the pointer, and
	// that difference is the semantic-action milestone visible in a sequencing test.
	if got := h.operations.kinds(); len(got) != 1 || got[0] != marcoexec.KindInvoke {
		t.Fatalf("step 1 ran %v, want a single structural invoke — \"open File\" must not "+
			"decompose into a click at File's coordinates", got)
	}
	if len(h.actuator.clicks) != 1 {
		t.Fatalf("clicks = %v, want one — only the explicit \"click Save\" clicks",
			h.actuator.clicks)
	}
	// That one click landed on Save, which was not resolvable when the program started.
	// That is the sequencing milestone in one assertion.
	save := h.worlds[2].Elements
	var savePoint directorapi.Point
	for _, el := range save {
		if el.Label == "Save" {
			savePoint = el.ClickPoint()
		}
	}
	if h.actuator.clicks[0] != savePoint {
		t.Fatalf("the click landed at %v, want Save at %v", h.actuator.clicks[0], savePoint)
	}
	// Each step observes before AND after. Fewer observations would mean a step reused
	// a world it did not observe itself.
	if h.observed < 4 {
		t.Fatalf("observed %d times for 2 steps; each must observe before and after", h.observed)
	}
}

func TestAProgramStopsAtTheFirstStepThatDoesNotVerify(t *testing.T) {
	// Program success is the conjunction of verified steps. Here the menu never opens,
	// so Save never appears and step 2 cannot resolve.
	h := newHarness(menuDismissed()...)

	out := h.pipeline.RunProgram(context.Background(),
		twoStep(t, "open File then click Save"), program.Context{}, 0)

	if out.Status == directorapi.ResultDone {
		t.Fatal("a program whose second step could not resolve reported success")
	}
	if out.StoppedAt != 2 {
		t.Fatalf("stopped at step %d, want 2", out.StoppedAt)
	}
	if out.Program.Status != program.StatusFailed {
		t.Fatalf("program status = %s, want failed", out.Program.Status)
	}
	// Only the first step acted, and it acted structurally — "open File" is a semantic
	// invoke. The second stopped at resolution, before any input.
	if got := h.operations.kinds(); len(got) != 1 {
		t.Fatalf("operations = %v, want just step 1's invoke", got)
	}
	if len(h.actuator.clicks) != 0 {
		t.Fatalf("clicks = %v, want none — the failed step must not have acted, and the "+
			"first step invokes rather than clicking", h.actuator.clicks)
	}
}

func TestALaterStepNeverRunsAfterAFailure(t *testing.T) {
	// Three steps, the second impossible. The third must not run at all — it would be
	// acting on a world the second step failed to produce.
	h := newHarness(menuDismissed()...)
	prog, err := program.Decompose("open File then click Save then click Exit", testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	if len(prog.Steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(prog.Steps))
	}

	out := h.pipeline.RunProgram(context.Background(), prog, program.Context{}, 0)

	if out.StoppedAt != 2 {
		t.Fatalf("stopped at %d, want 2", out.StoppedAt)
	}
	if len(out.Steps) != 2 {
		t.Fatalf("ran %d steps, want 2 — the third must never be attempted", len(out.Steps))
	}
}

func TestCancellationStopsBetweenStepsAndNotDuringOne(t *testing.T) {
	h := newHarness(menuFlow()...)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out := h.pipeline.RunProgram(ctx, twoStep(t, "open File then click Save"),
		program.Context{}, 0)

	if out.Status != directorapi.ResultCancelled {
		t.Fatalf("status = %s, want cancelled", out.Status)
	}
	if out.StoppedAt != 1 {
		t.Fatalf("stopped at %d, want 1 — cancellation prevents the NEXT step", out.StoppedAt)
	}
	if len(h.actuator.clicks) != 0 {
		t.Fatalf("clicked %v after cancellation", h.actuator.clicks)
	}
}

func TestResumingFromAStepDoesNotRerunCompletedOnes(t *testing.T) {
	// How a clarification resumes. Re-running a completed step would repeat input that
	// already landed — the double-application hazard, at program scale.
	// Resuming at step 2 means step 1 ALREADY RAN, so the world it left behind is the
	// one with the menu open — which is why this fixture starts there rather than at a
	// closed menu.
	h := newHarness(scenes(menuOpen(), menuBar())...)

	out := h.pipeline.RunProgram(context.Background(),
		twoStep(t, "open File then click Save"), program.Context{}, 1)

	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	if len(h.actuator.clicks) != 1 {
		t.Fatalf("clicks = %v, want one — step 1 was already done", h.actuator.clicks)
	}
	if out.Completed != 2 {
		t.Fatalf("completed = %d, want 2 (one carried over, one run)", out.Completed)
	}
}

func TestEveryCompletedStepBecomesANodeChainedToTheOneBefore(t *testing.T) {
	h := newHarness(menuFlow()...)
	out := h.pipeline.RunProgram(context.Background(),
		twoStep(t, "open File then click Save"), program.Context{}, 0)

	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	first, second := out.Steps[0].Node, out.Steps[1].Node
	if first == nil || second == nil {
		t.Fatal("a completed step did not produce an action node")
	}
	if second.Parent == nil {
		t.Fatal("the second step's node has no parent — a program must read as a chain")
	}
	if string(*second.Parent) != string(first.ID) {
		t.Fatalf("parent = %s, want the previous step's node %s", *second.Parent, first.ID)
	}
}

func TestStepBoundariesAreReportedAsTheyHappen(t *testing.T) {
	h := newHarness(menuFlow()...)
	var verified int
	h.pipeline.OnProgram = func(ev ProgramProgress) {
		if ev.Status == "verified" {
			verified++
		}
		if ev.Total != 2 {
			t.Errorf("progress reported total %d, want 2", ev.Total)
		}
	}
	h.pipeline.RunProgram(context.Background(),
		twoStep(t, "open File then click Save"), program.Context{}, 0)

	if verified != 2 {
		t.Fatalf("reported %d verified boundaries, want 2", verified)
	}
}

func TestASingleClauseRequestTakesTheUnchangedPath(t *testing.T) {
	// Compatibility. Everything built before this milestone must behave identically,
	// and that is decided before any program machinery is entered.
	h := newHarness(menuDismissed()...)
	out := h.pipeline.HandleRequest(context.Background(), "click File")

	if out.Program != nil {
		t.Fatal("a single-clause request was turned into a program")
	}
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
}

func TestARejectedRequestRunsNothing(t *testing.T) {
	h := newHarness(menuDismissed()...)
	out := h.pipeline.HandleRequest(context.Background(), "click File and scroll down")

	if out.Status == directorapi.ResultDone {
		t.Fatal("a request with an unsupported step succeeded")
	}
	if len(h.actuator.clicks) != 0 {
		t.Fatalf("clicked %v for a rejected request — validation must precede execution",
			h.actuator.clicks)
	}
	if !strings.Contains(out.Message, "scroll") {
		t.Fatalf("message = %q, want it to name the unsupported operation", out.Message)
	}
}

func TestAMultiClauseRequestBecomesAProgram(t *testing.T) {
	h := newHarness(menuFlow()...)
	out := h.pipeline.HandleRequest(context.Background(), "open File then click Save")

	if out.Program == nil {
		t.Fatal("a two-clause request did not become a program")
	}
	if out.Program.Completed != 2 {
		t.Fatalf("completed = %d, want 2", out.Program.Completed)
	}
	if out.Program.Program.ID == "" {
		t.Fatal("the program has no id, so a clarification could not name it")
	}
}

func TestAnUnverifiableStepStopsTheProgramUnlessItIsBestEffort(t *testing.T) {
	// Best-effort is a closed list in the planner, not a judgement made in the loop.
	// A step that is merely hard to check must NOT be able to talk its way past
	// verification, which is what this pair of cases pins down.
	for _, c := range []struct {
		name string
		req  program.VerificationRequirement
		want directorapi.ResultStatus
	}{
		{"required stops", program.VerifyRequired, directorapi.ResultPartial},
		{"best effort continues", program.VerifyBestEffort, directorapi.ResultDone},
	} {
		t.Run(c.name, func(t *testing.T) {
			// menuDismissed makes step 1's click verifiable and step 2's target absent;
			// here both steps click File, and the second one changes nothing, so it comes
			// back unverified.
			h := newHarness(scenes(menuBar(), menuOpen(), menuOpen(), menuOpen())...)
			prog := twoStep(t, "click File then click File")
			for i := range prog.Steps {
				prog.Steps[i].Verification = c.req
			}
			out := h.pipeline.RunProgram(context.Background(), prog, program.Context{}, 0)
			if out.Status != c.want {
				t.Fatalf("status = %s (%s), want %s", out.Status, out.Message, c.want)
			}
		})
	}
}

func TestOperationsTheWorldCannotObserveAreMarkedBestEffort(t *testing.T) {
	// The planner's list, checked directly — because the loop trusts it completely.
	prog, err := program.Decompose("type hello and press enter", testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	if prog.Steps[0].Verification != program.VerifyRequired {
		t.Fatalf("typing must be verified: %s", prog.Steps[0].Verification)
	}
	if prog.Steps[1].Verification != program.VerifyBestEffort {
		t.Fatalf("press enter = %s; the World Model cannot always see it",
			prog.Steps[1].Verification)
	}
}
