package execute

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Every test here runs the real replay loop against scripted worlds. No desktop is
// touched, which is what makes the failure cases testable at all: there is no way to
// make a live application go ambiguous, stall, or vanish on demand.

// seed runs one real action so the graph has something to repeat, then returns the
// harness positioned for a replay.
func seed(t *testing.T, worlds ...directorapi.WorldState) *harness {
	t.Helper()
	h := newHarness(worlds...)
	out := h.pipeline.Handle(context.Background(), "click File")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("seeding the graph failed: %s", out.Message)
	}
	return h
}

// menuVariant returns the menu bar with the File item at a given position, so a
// replay can be made to see a moved target.
func menuVariant(fileX, fileY int) []directorapi.Observation {
	return []directorapi.Observation{
		obs("uia:1", directorapi.RoleWindow, "Untitled - Notepad", rect(100, 100, 800, 600)),
		obs("uia:2", directorapi.RoleMenuItem, "File", rect(fileX, fileY, 40, 30)),
		obs("uia:3", directorapi.RoleMenuItem, "Edit", rect(fileX+50, fileY, 40, 30)),
		obs("uia:5", directorapi.RoleTextField, "Text editor", rect(100, 180, 800, 520)),
	}
}

// opened is the same window with a menu revealed, distinguished by a marker label so
// consecutive iterations produce different fingerprints.
func opened(marker string) []directorapi.Observation {
	out := menuVariant(110, 140)
	return append(out,
		obs("uia:10", directorapi.RoleMenuItem, "New "+marker, rect(110, 175, 200, 28)),
		obs("uia:11", directorapi.RoleMenuItem, "Open "+marker, rect(110, 203, 200, 28)),
		obs("uia:12", directorapi.RoleMenuItem, "Save "+marker, rect(110, 231, 200, 28)),
	)
}

func at(n int) time.Time { return t0.Add(time.Duration(n) * time.Second) }

// ── successful replay ─────────────────────────────────────────────────────────

func TestSuccessfulReplay(t *testing.T) {
	h := seed(t,
		scene(at(0), nil, menuVariant(110, 140)...), // observe
		scene(at(1), nil, opened("a")...),           // after the seeded click
		scene(at(2), nil, menuVariant(110, 140)...), // replay: observe
		scene(at(3), nil, opened("b")...),           // replay: after
	)

	out := h.pipeline.Handle(context.Background(), "do that again")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	if out.Replay == nil {
		t.Fatal("a repeat should report its replay")
	}
	if out.Replay.Completed != 1 {
		t.Errorf("completed = %d, want 1", out.Replay.Completed)
	}
	// Two clicks total: the seeded one and the replay.
	if len(h.actuator.clicks) != 2 {
		t.Fatalf("want 2 clicks (original + replay), got %d", len(h.actuator.clicks))
	}
	if !out.Replay.Iterations[0].Verified {
		t.Error("the replay should have verified")
	}
}

// ── replay produces a new node ────────────────────────────────────────────────

// A replay appends; it never edits the action it came from. The original stays
// exactly as it was, which is what makes the two comparable afterwards.
func TestReplayProducesANewNode(t *testing.T) {
	h := seed(t,
		scene(at(0), nil, menuVariant(110, 140)...),
		scene(at(1), nil, opened("a")...),
		scene(at(2), nil, menuVariant(110, 140)...),
		scene(at(3), nil, opened("b")...),
	)
	source, _ := h.graph.Last()

	out := h.pipeline.Handle(context.Background(), "do that again")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}

	if h.graph.Len() != 2 {
		t.Fatalf("want 2 nodes (original + replay), got %d", h.graph.Len())
	}
	replayed, _ := h.graph.Last()
	if replayed.ID == source.ID {
		t.Fatal("the replay must be a new node, not the original")
	}
	// The original is untouched.
	original, err := h.graph.Get(source.ID)
	if err != nil || !original.Outcome.Success {
		t.Error("the source node must be unchanged")
	}
	// The new node knows what it repeated, and is chained to it.
	if replayed.Metadata["replay_of"] != string(source.ID) {
		t.Errorf("the replay should record what it repeated, got %v", replayed.Metadata["replay_of"])
	}
	if replayed.Parent == nil || *replayed.Parent != source.ID {
		t.Error("the replay should chain to the action it repeated")
	}
	// And it is the SAME action, semantically — which is the whole point.
	if !actiongraph.Equivalent(original, replayed) {
		t.Error("a replay should be semantically equivalent to what it repeated")
	}
}

// ── the central guarantee ─────────────────────────────────────────────────────

// Replay must re-derive everything. The stored plan, element id and coordinates are
// history; only the intent carries over.
func TestReplayReResolvesRatherThanReplayingCoordinates(t *testing.T) {
	// The File item is somewhere different by the time the replay runs.
	h := seed(t,
		scene(at(0), nil, menuVariant(110, 140)...),
		scene(at(1), nil, opened("a")...),
		scene(at(2), nil, menuVariant(600, 400)...), // moved
		scene(at(3), nil, opened("b")...),
	)

	out := h.pipeline.Handle(context.Background(), "do that again")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	if len(h.actuator.clicks) != 2 {
		t.Fatalf("want 2 clicks, got %d", len(h.actuator.clicks))
	}

	first, second := h.actuator.clicks[0], h.actuator.clicks[1]
	if first == second {
		t.Fatal("the replay clicked the ORIGINAL coordinates — it must re-resolve")
	}
	// It clicked where the target is NOW: the centre of (600,400 40x30).
	if second != (directorapi.Point{X: 620, Y: 415}) {
		t.Errorf("replay clicked %+v, want the target's current centre (620,415)", second)
	}
	// A fresh plan was built, not the stored one re-executed.
	replayed, _ := h.graph.Last()
	spec, _ := replayed.Action()
	if spec.Query == nil || spec.Query.Label != "File" {
		t.Error("the replay's plan should have been rebuilt from the query")
	}
}

// ── TARGET_MOVED ──────────────────────────────────────────────────────────────

func TestReplayWhenTargetMoved(t *testing.T) {
	h := seed(t,
		scene(at(0), nil, menuVariant(110, 140)...),
		scene(at(1), nil, opened("a")...),
		scene(at(2), nil, menuVariant(600, 400)...),
		scene(at(3), nil, opened("b")...),
	)

	out := h.pipeline.Handle(context.Background(), "do that again")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("a moved target is still replayable: %s (%s)", out.Status, out.Message)
	}
	if got := out.Replay.Iterations[0].Analysis.Status; got != actiongraph.ReplayTargetMoved {
		t.Errorf("analysis = %s, want TARGET_MOVED", got)
	}
	// Confidence should note the move rather than hide it.
	if out.Replay.Confidence.Target >= 1.0 {
		t.Error("a moved target should reduce target confidence slightly")
	}
	if len(out.Replay.Confidence.Notes) == 0 {
		t.Error("the move should be explained in the confidence notes")
	}
}

// ── APP_NOT_RUNNING ───────────────────────────────────────────────────────────

func TestReplayRefusedWhenAppNotRunning(t *testing.T) {
	elsewhere := scene(at(2), []directorapi.Window{{
		ID: "hwnd:9", Application: "chrome", Title: "Chrome",
		Bounds: rect(0, 0, 800, 600), Focused: true, Visible: true, MonitorID: "monitor:1",
	}},
		obs("uia:20", directorapi.RoleWindow, "Chrome", rect(0, 0, 800, 600)),
		obs("uia:21", directorapi.RoleButton, "Back", rect(10, 10, 40, 24)),
		obs("uia:22", directorapi.RoleButton, "Forward", rect(60, 10, 40, 24)),
	)

	h := seed(t,
		scene(at(0), nil, menuVariant(110, 140)...),
		scene(at(1), nil, opened("a")...),
		elsewhere,
	)
	before := len(h.actuator.clicks)

	out := h.pipeline.Handle(context.Background(), "do that again")
	if out.Status == directorapi.ResultDone {
		t.Fatal("replay must not succeed when the application is gone")
	}
	if got := out.Replay.Iterations[0].Analysis.Status; got != actiongraph.ReplayAppNotRunning {
		t.Errorf("analysis = %s, want APP_NOT_RUNNING", got)
	}
	if len(h.actuator.clicks) != before {
		t.Error("nothing should have been clicked")
	}
	if out.Replay.Completed != 0 {
		t.Errorf("completed = %d, want 0", out.Replay.Completed)
	}
}

// ── UNOBSERVABLE ──────────────────────────────────────────────────────────────

// A window that exposes nothing must stop the replay, and must not be reported as
// the target being gone.
func TestReplayRefusedWhenUnobservable(t *testing.T) {
	opaque := scene(at(2), nil,
		obs("uia:1", directorapi.RoleWindow, "Untitled - Notepad", rect(0, 0, 1200, 800)),
		obs("uia:2", directorapi.RolePane, "", rect(0, 0, 1200, 800)),
		obs("uia:3", directorapi.RolePane, "", rect(0, 40, 1200, 760)),
		obs("uia:4", directorapi.RolePane, "", rect(240, 40, 960, 760)),
	)
	h := seed(t,
		scene(at(0), nil, menuVariant(110, 140)...),
		scene(at(1), nil, opened("a")...),
		opaque,
	)
	before := len(h.actuator.clicks)

	out := h.pipeline.Handle(context.Background(), "do that again")
	if out.Status == directorapi.ResultDone {
		t.Fatal("replay must not proceed into a window it cannot read")
	}
	if got := out.Replay.Iterations[0].Analysis.Status; got != actiongraph.ReplayUnobservable {
		t.Errorf("analysis = %s, want UNOBSERVABLE", got)
	}
	if out.Replay.StoppedBecause != "unobservable" {
		t.Errorf("stopped because %q, want unobservable", out.Replay.StoppedBecause)
	}
	if !strings.Contains(out.Message, "cannot see") {
		t.Errorf("the message should say it could not see, not that the target is gone: %q", out.Message)
	}
	if len(h.actuator.clicks) != before {
		t.Error("nothing should have been clicked")
	}
}

// ── ambiguity requiring clarification ─────────────────────────────────────────

func TestReplayStopsForAmbiguity(t *testing.T) {
	twoFiles := scene(at(2), nil,
		obs("uia:1", directorapi.RoleWindow, "Untitled - Notepad", rect(100, 100, 800, 600)),
		obs("uia:2", directorapi.RoleMenuItem, "File", rect(110, 140, 40, 30)),
		obs("uia:9", directorapi.RoleMenuItem, "File", rect(400, 140, 40, 30)),
		obs("uia:5", directorapi.RoleTextField, "Text editor", rect(100, 180, 800, 520)),
	)
	h := seed(t,
		scene(at(0), nil, menuVariant(110, 140)...),
		scene(at(1), nil, opened("a")...),
		twoFiles,
	)
	before := len(h.actuator.clicks)

	out := h.pipeline.Handle(context.Background(), "do that again")
	if out.Status != directorapi.ResultNeedsClarification {
		t.Fatalf("status = %s (%s), want clarification", out.Status, out.Message)
	}
	if len(h.actuator.clicks) != before {
		t.Fatal("an ambiguous replay must not pick one and act")
	}
	if out.Replay.StoppedBecause != "ambiguous" {
		t.Errorf("stopped because %q, want ambiguous", out.Replay.StoppedBecause)
	}
}

// ── bounded repeat count ──────────────────────────────────────────────────────

func TestBoundedRepeatCount(t *testing.T) {
	// Alternating worlds so each iteration makes progress and the loop is bounded
	// only by the requested count.
	worlds := []directorapi.WorldState{
		scene(at(0), nil, menuVariant(110, 140)...),
		scene(at(1), nil, opened("a")...),
	}
	for i := range 6 {
		worlds = append(worlds,
			scene(at(2+i*2), nil, menuVariant(110+i, 140)...),
			scene(at(3+i*2), nil, opened(string(rune('b'+i)))...),
		)
	}
	h := seed(t, worlds...)
	before := len(h.actuator.clicks)

	out := h.pipeline.Handle(context.Background(), "repeat that 3 times")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	if out.Replay.Completed != 3 {
		t.Fatalf("completed = %d, want exactly 3", out.Replay.Completed)
	}
	if got := len(h.actuator.clicks) - before; got != 3 {
		t.Errorf("want exactly 3 replayed clicks, got %d", got)
	}
	if h.graph.Len() != 4 {
		t.Errorf("want 4 nodes (original + 3 replays), got %d", h.graph.Len())
	}
}

// A count above the ceiling is refused outright rather than silently truncated. A
// misheard "fifty" for "five" must not become fifty clicks.
func TestRepeatCountCeiling(t *testing.T) {
	h := seed(t,
		scene(at(0), nil, menuVariant(110, 140)...),
		scene(at(1), nil, opened("a")...),
	)
	h.pipeline.MaxReplayIterations = 5
	before := len(h.actuator.clicks)

	out := h.pipeline.Handle(context.Background(), "repeat that 20 times")
	if out.Status != directorapi.ResultBlocked {
		t.Fatalf("status = %s (%s), want blocked", out.Status, out.Message)
	}
	if len(h.actuator.clicks) != before {
		t.Error("nothing should have been executed")
	}
	if !strings.Contains(out.Message, "limit") {
		t.Errorf("the message should name the limit, got %q", out.Message)
	}
}

// ── cancellation ──────────────────────────────────────────────────────────────

// Stop takes effect BETWEEN iterations, so it interrupts the loop rather than
// arriving after it has finished doing what it was told to stop.
func TestCancellationStopsTheLoop(t *testing.T) {
	worlds := []directorapi.WorldState{
		scene(at(0), nil, menuVariant(110, 140)...),
		scene(at(1), nil, opened("a")...),
	}
	for i := range 6 {
		worlds = append(worlds,
			scene(at(2+i*2), nil, menuVariant(110+i, 140)...),
			scene(at(3+i*2), nil, opened(string(rune('b'+i)))...),
		)
	}
	h := seed(t, worlds...)
	before := len(h.actuator.clicks)

	// Ask to stop after the second iteration has run.
	stop := false
	done := 0
	h.pipeline.StopCheck = func() bool {
		done++
		return stop
	}
	h.pipeline.Now = func() time.Time {
		if done >= 3 {
			stop = true
		}
		return t0
	}

	out := h.pipeline.Handle(context.Background(), "repeat that 5 times")
	if out.Status != directorapi.ResultCancelled {
		t.Fatalf("status = %s (%s), want cancelled", out.Status, out.Message)
	}
	if out.Replay.Completed >= 5 {
		t.Error("the loop should have stopped short of the requested count")
	}
	if got := len(h.actuator.clicks) - before; got >= 5 {
		t.Errorf("the loop kept clicking after being stopped: %d", got)
	}
	// It reports how far it got, which is what the user needs to know.
	if !strings.Contains(out.Message, "of 5") {
		t.Errorf("the message should report progress, got %q", out.Message)
	}
}

// A context cancelled before the loop begins must stop it immediately.
func TestContextCancellationStopsImmediately(t *testing.T) {
	h := seed(t,
		scene(at(0), nil, menuVariant(110, 140)...),
		scene(at(1), nil, opened("a")...),
	)
	before := len(h.actuator.clicks)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out := h.pipeline.Handle(ctx, "repeat that 3 times")
	if out.Status != directorapi.ResultCancelled {
		t.Fatalf("status = %s, want cancelled", out.Status)
	}
	if len(h.actuator.clicks) != before {
		t.Error("a cancelled context must stop the loop before it acts")
	}
}

// ── progress stall ────────────────────────────────────────────────────────────

// An iteration that changes nothing means further iterations would change nothing
// either — but faster, and as many times as were asked for.
func TestProgressStallStopsTheLoop(t *testing.T) {
	still := menuVariant(110, 140)
	h := seed(t,
		scene(at(0), nil, still...),
		scene(at(1), nil, opened("a")...),
		// Every replay iteration now sees, and leaves, the same screen.
		scene(at(2), nil, still...),
		scene(at(3), nil, still...),
		scene(at(4), nil, still...),
		scene(at(5), nil, still...),
	)
	before := len(h.actuator.clicks)

	out := h.pipeline.Handle(context.Background(), "repeat that 5 times")
	if out.Status == directorapi.ResultDone {
		t.Fatal("a stalled loop must not report success")
	}
	if out.Replay.Completed >= 5 {
		t.Errorf("the loop should have stopped early, completed %d", out.Replay.Completed)
	}
	if got := len(h.actuator.clicks) - before; got > 2 {
		t.Errorf("a stalled loop kept going: %d clicks", got)
	}
	if out.Replay.StoppedBecause != "no_progress" && out.Replay.StoppedBecause != "unverified" {
		t.Errorf("stopped because %q, want no_progress or unverified", out.Replay.StoppedBecause)
	}
}

// ── verification failure ──────────────────────────────────────────────────────

// A replay whose verification fails stops rather than continuing. Repeating an
// action whose effect is unknown is how one unverified click becomes five.
func TestVerificationFailureStopsTheLoop(t *testing.T) {
	// The first replay iteration changes something, but nothing that verifies the
	// click, so it is UNVERIFIED rather than a clean failure.
	unrelated := append(menuVariant(110, 140),
		obs("uia:99", directorapi.RoleText, "loading", rect(400, 400, 60, 16)))

	h := seed(t,
		scene(at(0), nil, menuVariant(110, 140)...),
		scene(at(1), nil, opened("a")...),
		scene(at(2), nil, menuVariant(110, 140)...),
		scene(at(3), nil, unrelated...),
		scene(at(4), nil, unrelated...),
		scene(at(5), nil, unrelated...),
	)
	before := len(h.actuator.clicks)

	out := h.pipeline.Handle(context.Background(), "repeat that 4 times")
	if out.Status == directorapi.ResultDone {
		t.Fatal("an unverified replay must not report success")
	}
	if got := len(h.actuator.clicks) - before; got > 1 {
		t.Errorf("the loop should have stopped after the first unverified iteration, got %d clicks", got)
	}
	if out.Replay.StoppedBecause != "unverified" {
		t.Errorf("stopped because %q, want unverified", out.Replay.StoppedBecause)
	}
	// The failed iteration is still recorded — history that only holds successes
	// cannot explain anything.
	if h.graph.Len() != 2 {
		t.Errorf("the unverified attempt should still be recorded, got %d nodes", h.graph.Len())
	}
}

// ── nothing to repeat ─────────────────────────────────────────────────────────

func TestRepeatWithNothingToRepeat(t *testing.T) {
	h := newHarness(scene(at(0), nil, menuVariant(110, 140)...))
	out := h.pipeline.Handle(context.Background(), "do that again")
	if out.Status == directorapi.ResultDone {
		t.Fatal("there is nothing to repeat")
	}
	if len(h.actuator.clicks) != 0 {
		t.Error("nothing should have been clicked")
	}
	if !strings.Contains(out.Message, "no completed action") {
		t.Errorf("the message should explain, got %q", out.Message)
	}
}

// "That" means the last action that WORKED. Repeating a failure would turn one wrong
// thing into several.
func TestRepeatSkipsFailedActions(t *testing.T) {
	still := menuVariant(110, 140)
	h := newHarness(
		scene(at(0), nil, still...),
		scene(at(1), nil, opened("a")...), // the good click verifies
		scene(at(2), nil, opened("a")...), // a later click changes nothing
		scene(at(3), nil, opened("a")...),
		scene(at(4), nil, opened("a")...),
		scene(at(5), nil, opened("a")...),
	)
	if out := h.pipeline.Handle(context.Background(), "click File"); out.Status != directorapi.ResultDone {
		t.Fatalf("seed: %s", out.Message)
	}
	// A second action that fails to verify.
	if out := h.pipeline.Handle(context.Background(), "click Edit"); out.Status == directorapi.ResultDone {
		t.Fatal("the second action was supposed to fail verification")
	}

	source, ok := h.pipeline.lastSuccessful()
	if !ok {
		t.Fatal("there should still be a successful action to repeat")
	}
	if source.ResolvedTarget.Label != "File" {
		t.Errorf("repeat should refer to the action that worked, got %q", source.ResolvedTarget.Label)
	}
}

// ── confidence diagnostics ────────────────────────────────────────────────────

// A replay in a different application should be visibly less trustworthy, even if
// something there happens to match.
func TestConfidenceFallsWhenTheContextChanges(t *testing.T) {
	// Notepad's File menu exists in both, but the second world is a different app.
	otherApp := scene(at(2), []directorapi.Window{{
		ID: "hwnd:1", Application: "wordpad", Title: "Document - WordPad",
		Bounds: rect(100, 100, 800, 600), Focused: true, Visible: true, MonitorID: "monitor:1",
	}}, menuVariant(110, 140)...)

	h := seed(t,
		scene(at(0), nil, menuVariant(110, 140)...),
		scene(at(1), nil, opened("a")...),
		otherApp,
	)

	out := h.pipeline.Handle(context.Background(), "do that again")
	// It stops at APP_NOT_RUNNING before confidence matters — which is the right
	// outcome, and confirms the application gate runs first.
	if out.Status == directorapi.ResultDone {
		t.Fatal("replaying into a different application must not silently succeed")
	}
	if got := out.Replay.Iterations[0].Analysis.Status; got != actiongraph.ReplayAppNotRunning {
		t.Errorf("analysis = %s, want APP_NOT_RUNNING", got)
	}
}

// The three dimensions are independent, and Overall is the weakest — a perfect
// intent match must not paper over acting on the wrong thing.
func TestConfidenceOverallIsTheWeakestDimension(t *testing.T) {
	c := ReplayConfidence{Intent: 1, Target: 0.2, Context: 1}
	c.Overall = min3(c.Intent, c.Target, c.Context)
	if c.Overall != 0.2 {
		t.Errorf("overall = %v, want the weakest (0.2)", c.Overall)
	}
}

// ── intent parsing ────────────────────────────────────────────────────────────

func TestRepeatPhrasesParse(t *testing.T) {
	h := newHarness(scene(at(0), nil, menuVariant(110, 140)...))
	cases := map[string]int{
		"do that again":          1,
		"repeat that":            1,
		"repeat the last action": 1,
		"do it again":            1,
		"again":                  1,
		"repeat that 5 times":    5,
		"repeat that five times": 5,
		"do that again 3 times":  3,
	}
	for phrase, wantCount := range cases {
		in := h.pipeline.Intent(phrase)
		if in.Kind != directorapi.IntentRepeat {
			t.Errorf("%q: kind = %s, want repeat", phrase, in.Kind)
			continue
		}
		if in.Count != wantCount {
			t.Errorf("%q: count = %d, want %d", phrase, in.Count, wantCount)
		}
	}
}

// The commands this milestone does NOT support must not be quietly treated as plain
// repeats — they name different targets and would run against the wrong thing.
func TestOutOfScopeRepeatPhrasesAreRefused(t *testing.T) {
	h := newHarness(scene(at(0), nil, menuVariant(110, 140)...))
	for _, phrase := range []string{
		"do the same thing to Chrome",
		"apply that to every selected file",
		"repeat that in Discord",
		"undo that",
	} {
		if in := h.pipeline.Intent(phrase); in.Kind == directorapi.IntentRepeat {
			t.Errorf("%q should not parse as a plain repeat", phrase)
		}
	}
}
