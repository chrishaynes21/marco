package demo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A DEMONSTRATION, SAVED UNDER A NAME, AND REPLAYED.
//
//	The next milestone is a named, demonstrated sequence that can be saved and replayed.
//	Preserve arbitrary step sequences rather than reducing the demonstration to one
//	parameterized goal.
//	"my settings tour" doesn't need to mean anything to Marco.
//
// Experiment 074 measured the two halves: a Director demonstration knows the verbs, the targets
// and the verification; a workflow artifact is named, durable and inert. These hold the join.

// tourDemo is three navigations and nothing generalizable: no deictic step, no typing, no goal
// shape. Exactly the routine the goal extractor has no template for.
func tourDemo() *demo.Demonstration {
	return &demo.Demonstration{
		ID: "demo-tour", Started: t0, Completed: t0.Add(time.Minute),
		Status: demo.Completed, Application: "applicationframehost",
		Requests: []string{"go to gaming", "go to accessibility", "go to personalization"},
		Steps: []demo.Step{
			step(1, demo.StepSemantic, directorapi.SemanticOpen,
				demo.Target{Label: "Gaming"}),
			step(2, demo.StepSemantic, directorapi.SemanticOpen,
				demo.Target{Label: "Accessibility"}),
			step(3, demo.StepSemantic, directorapi.SemanticOpen,
				demo.Target{Label: "Personalization"}),
		},
		Nodes: []actiongraph.NodeID{"action_1", "action_2", "action_3"},
	}
}

func TestADemonstrationSavesAsANamedArtifact(t *testing.T) {
	a, err := demo.FromDemonstration("my settings tour", tourDemo(), t0)
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	// THE NAME IS THE PERSON'S, as typed. Not derived, not normalised, and not required
	// to mean anything to Marco.
	if a.Name != "my settings tour" {
		t.Errorf("name = %q", a.Name)
	}
	// EVERY STEP, IN ORDER. The thing the goal extractor cannot do: it generalizes the
	// first step into a parameter and freezes the rest, so three destinations become one.
	if len(a.Steps) != 3 {
		t.Fatalf("%d steps, want the three that were demonstrated: %+v", len(a.Steps), a.Steps)
	}
	var went []string
	byLabel := map[string]workflowstore.Control{}
	for _, c := range a.Controls {
		byLabel[c.Label] = c
	}
	for i, st := range a.Steps {
		if st.Position != i {
			t.Errorf("step %d is at position %d", i, st.Position)
		}
		if st.Verb != string(directorapi.SemanticOpen) {
			t.Errorf("step %d does %q", i, st.Verb)
		}
		c := byLabel[st.Control]
		if len(c.Text) != 1 {
			t.Fatalf("step %d's control carries %v, not the one label it bore", i, c.Text)
		}
		went = append(went, c.Text[0])
	}
	if strings.Join(went, ",") != "Gaming,Accessibility,Personalization" {
		t.Errorf("it saved a tour of %v", went)
	}
	// AND IT READS BACK. FromDemonstration validates by round-trip, so this asserts the
	// one thing that would otherwise be untested: that the loader accepts what it wrote.
	body, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := workflowstore.Unmarshal(body)
	if err != nil {
		t.Fatalf("the artifact just written could not be read back: %v", err)
	}
	if back.Name != a.Name || len(back.Steps) != len(a.Steps) {
		t.Errorf("the reloaded sequence is a different one: %+v", back)
	}
}

// A ROLE OUTRANKS A LABEL IN THE SAVED FORM TOO, and one control addressed twice is one control.
func TestASavedSequenceKeepsRolesAndMergesRepeatedControls(t *testing.T) {
	d := tourDemo()
	d.Steps = []demo.Step{
		step(1, demo.StepSemantic, directorapi.SemanticInvoke,
			demo.Target{Label: "Rename", Role: goal.RoleRenameCommand}),
		editStep(2, "Budget", demo.Target{DerivedEditor: true, Label: "Alpha.txt"}),
		step(3, demo.StepSemantic, directorapi.SemanticInvoke,
			demo.Target{Label: "Rename", Role: goal.RoleRenameCommand}),
	}
	a, err := demo.FromDemonstration("double rename", d, t0)
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	// TWO distinct controls, not three: the rename command was addressed twice.
	if len(a.Controls) != 2 {
		t.Fatalf("%d controls: %+v", len(a.Controls), a.Controls)
	}
	var rename workflowstore.Control
	for _, c := range a.Controls {
		if c.Role != "" {
			rename = c
		}
	}
	if rename.Role != string(goal.RoleRenameCommand) {
		t.Fatalf("the rename command saved as %+v", rename)
	}
	// THE LABEL IS NOT KEPT BESIDE THE ROLE. A role is the localisable form, and a saved
	// label beside it is a second answer that can disagree with the first.
	if len(rename.Text) != 0 {
		t.Errorf("the role-named control also saved the label %v", rename.Text)
	}
	if a.Steps[0].Seen != 2 || a.Steps[2].Seen != 2 {
		t.Errorf("the twice-addressed control reports Seen %d/%d",
			a.Steps[0].Seen, a.Steps[2].Seen)
	}
	// AND THE EDIT CARRIES ITS VALUE AND ITS EDITOR, never a search for "Alpha.txt".
	edit := a.Steps[1]
	if edit.Verb != "edit" || edit.Value != "Budget" {
		t.Errorf("the edit step saved as %+v", edit)
	}
	for _, c := range a.Controls {
		if c.Editor && len(c.Text) > 0 {
			t.Errorf("the inline editor saved a name to search for: %v", c.Text)
		}
	}
}

// AND IT REFUSES WHAT IT CANNOT REPLAY, with a sentence rather than at step three.
func TestASequenceRefusesWhatItCannotReplay(t *testing.T) {
	cases := map[string]func(*demo.Demonstration){
		"secret": func(d *demo.Demonstration) { d.Steps[1].Sensitive = true },
		"a captured value": func(d *demo.Demonstration) {
			d.Steps[1].ValueRef = "value_3"
		},
		"unverified": func(d *demo.Demonstration) { d.Steps[1].Verified = false },
		"clarified":  func(d *demo.Demonstration) { d.Steps[1].Clarified = true },
		"pointed at": func(d *demo.Demonstration) {
			d.Steps[1].Target = demo.Target{Deictic: true, Phrase: "this one"}
		},
	}
	for what, break_ := range cases {
		d := tourDemo()
		break_(d)
		if _, err := demo.FromDemonstration("doomed", d, t0); err == nil {
			t.Errorf("a %s step was saved as a replayable sequence", what)
		}
		if why := demo.RefusalToSave(d); why == "" {
			t.Errorf("a %s step reports no reason it cannot be saved", what)
		}
	}
	// AND AN INCOMPLETE DEMONSTRATION IS NOT A RECORD OF ANYTHING.
	open := tourDemo()
	open.Status = demo.Recording
	if _, err := demo.FromDemonstration("open", open, t0); err == nil {
		t.Error("a demonstration still being recorded was saved as a sequence")
	}
	// AND A SEQUENCE NEEDS A NAME, for the reason the artifact has always needed one.
	if _, err := demo.FromDemonstration("  ", tourDemo(), t0); err == nil {
		t.Error("a sequence was saved with no name")
	}
}

// NOTHING MECHANICAL SURVIVES A SAVED SEQUENCE.
//
// The same assertion TestNothingMechanicalSurvivesExtraction makes about a learned procedure,
// over the durable form this time — which is the one that outlives the session and the one
// `workflowstore` has always forbidden these fields in.
func TestNothingMechanicalSurvivesASavedSequence(t *testing.T) {
	d := tourDemo()
	d.Steps[0].Target.ElementRole = directorapi.RoleListItem
	a, err := demo.FromDemonstration("my settings tour", d, t0)
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	body, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	raw := string(body)
	for _, forbidden := range []string{
		`"hwnd`, `"native_id"`, `"element_id"`, `"runtime_id"`, `"window_id"`,
		`"x":`, `"y":`, `"bounds"`, `"point"`, `"value_ref"`,
	} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("the saved sequence contains %s:\n%s", forbidden, raw)
		}
	}
}

// A SAVED SEQUENCE REPLAYS THROUGH THE ORDINARY PROGRAM PATH.
//
//	Replay through existing Director grounding, policy, and Marco execution mechanisms.
//	Reacquire targets from fresh observations; do not store or depend on transient
//	accessibility handles or absolute coordinates as target identity.
//
// So the assertion is that what comes out is a program the ordinary validator accepts, whose
// every step is aimed by something a fresh observation can resolve.
func TestASavedSequenceReplaysThroughTheOrdinaryProgramPath(t *testing.T) {
	a, err := demo.FromDemonstration("my settings tour", tourDemo(), t0)
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	ex, err := demo.ExpandSequence(a, 1, fakeBinder{})
	if err != nil {
		t.Fatalf("expanding: %v", err)
	}
	if err := program.Validate(ex.Program); err != nil {
		t.Fatalf("a saved sequence expanded into a program the Director cannot run: %v", err)
	}
	if err := program.ValidateBound(ex.Program); err != nil {
		t.Fatalf("the bound validator refuses it: %v", err)
	}
	if len(ex.Program.Steps) != 3 {
		t.Fatalf("%d steps", len(ex.Program.Steps))
	}
	var aimedAtNothing int
	var at []string
	for _, s := range ex.Program.Steps {
		if len(s.Operation.Targets) == 0 {
			aimedAtNothing++
			continue
		}
		ref := s.Operation.Targets[0]
		if ref.Query != nil && ref.Query.Focused != nil && *ref.Query.Focused {
			aimedAtNothing++
			continue
		}
		at = append(at, aimedAt(ref))
	}
	if aimedAtNothing > 0 {
		t.Errorf("%d replayed step(s) are aimed at whatever holds focus", aimedAtNothing)
	}
	if strings.Join(at, ",") != "Gaming,Accessibility,Personalization" {
		t.Errorf("the replay goes to %v", at)
	}
	// AND IT IS NOT A GOAL. A sequence must never acquire a phrase, because the words it
	// is made of belong to the ordinary request path.
	if _, ok := goal.Parse("run my settings tour"); ok {
		t.Error("a named sequence became a goal phrase and will hijack requests")
	}
	if demo.SequenceKind.Known() {
		t.Error("the sequence kind is a known goal, so goal.Expand will accept it")
	}
}

// ── the store ─────────────────────────────────────────────────────────────────

func TestASavedSequenceIsFoundByItsName(t *testing.T) {
	s, err := demo.Open(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := demo.FromDemonstration("My Settings Tour", tourDemo(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSequence(a); err != nil {
		t.Fatalf("saving: %v", err)
	}
	// ASKED FOR IN THE WORDS SOMEBODY SAYS, which are not the words they typed.
	back, ok := s.Sequence("my settings tour")
	if !ok {
		t.Fatal("a saved sequence could not be found by its own name in another casing")
	}
	if back.Name != "My Settings Tour" {
		t.Errorf("it came back as %q rather than as it was named", back.Name)
	}
	if len(back.Steps) != 3 {
		t.Errorf("%d steps survived the store", len(back.Steps))
	}
	if got := s.Sequences(); len(got) != 1 || got[0].Name != "My Settings Tour" {
		t.Errorf("the listing is %+v", got)
	}
	// A NAME NOBODY SAVED IS ABSENT, and that is not a problem to report.
	if _, ok := s.Sequence("something else"); ok {
		t.Error("a name nobody saved came back")
	}
	if why := s.SequenceProblem("something else"); why != "" {
		t.Errorf("a missing sequence is reported as a problem: %s", why)
	}
	if err := s.ForgetSequence("MY SETTINGS TOUR"); err != nil {
		t.Errorf("forgetting: %v", err)
	}
	if _, ok := s.Sequence("my settings tour"); ok {
		t.Error("a forgotten sequence is still there")
	}
}

// AND A SEQUENCE THIS BUILD CANNOT VOUCH FOR IS A PROBLEM, not a silent absence.
func TestAnUnreadableSequenceIsReportedRatherThanSkipped(t *testing.T) {
	dir := t.TempDir()
	s, err := demo.Open(filepath.Join(dir, "home"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := demo.FromDemonstration("my settings tour", tourDemo(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSequence(a); err != nil {
		t.Fatal(err)
	}
	// A FILE FROM A NEWER MARCO. The version check is the whole point: a reader that
	// ignored it would replay a sequence missing whatever that version added.
	path := filepath.Join(dir, "home", "sequences", "my_settings_tour.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	m["version"] = workflowstore.Version + 1
	next, _ := json.Marshal(m)
	if err := os.WriteFile(path, next, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Sequence("my settings tour"); ok {
		t.Error("a sequence from a newer Marco was read anyway")
	}
	if why := s.SequenceProblem("my settings tour"); why == "" {
		t.Error("a sequence that is there and unreadable is reported as absent")
	}
}

// AND TWO NAMES THAT FLATTEN TO ONE FILE COLLIDE rather than one replacing the other.
//
// A name is the person's and a file name is where it was put. Silently replacing somebody's
// routine because their new name flattens to the same characters is a loss noticed weeks later.
func TestTwoNamesThatFlattenToOneFileCollide(t *testing.T) {
	s, err := demo.Open(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := demo.FromDemonstration("My Settings Tour", tourDemo(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSequence(first); err != nil {
		t.Fatal(err)
	}
	// A DIFFERENT NAME, the same file.
	clash, err := demo.FromDemonstration("my-settings-tour!", tourDemo(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSequence(clash); err == nil {
		t.Error("a differently-named sequence overwrote one that was already saved")
	}
	// SAVING THE SAME NAME AGAIN IS AN UPDATE, which is not a collision. Re-demonstrating
	// a routine you already have is the ordinary case.
	again, err := demo.FromDemonstration("my settings tour", tourDemo(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSequence(again); err != nil {
		t.Errorf("re-saving a sequence under the same name was refused: %v", err)
	}
	back, ok := s.Sequence("my settings tour")
	if !ok || back.Name != "my settings tour" {
		t.Errorf("the update did not take: %+v", back)
	}
}

// ── asking for one by name ────────────────────────────────────────────────────

// RUNNING A NAME NOBODY SAVED IS AN ORDINARY REQUEST.
//
// The property that makes the invocation path safe to add. A prefix table deciding this on
// words alone would claim every sentence beginning with "run", and the ordinary request path
// would stop seeing them.
func TestRunningANameNobodySavedIsAnOrdinaryRequest(t *testing.T) {
	s, err := demo.Open(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := demo.FromDemonstration("my settings tour", tourDemo(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSequence(a); err != nil {
		t.Fatal(err)
	}

	// NOT SEQUENCES, and every one of them is a sentence somebody might really say.
	for _, request := range []string{
		"run the installer", "run diagnostics", "run", "running total",
		"replay the last action", "click run", "open my settings tour",
	} {
		if got, _, ok := s.Asked(request); ok {
			t.Errorf("%q was taken as a request for the saved sequence %q",
				request, got.Name)
		}
	}
	// AND THESE ARE.
	for _, request := range []string{
		"run my settings tour", "replay my settings tour", "Run My Settings Tour.",
	} {
		if _, _, ok := s.Asked(request); !ok {
			t.Errorf("%q did not ask for the sequence of that name", request)
		}
	}
}

// A BOUNDED REPEAT IS THE STEPS AGAIN, and the bound is enforced.
func TestABoundedRepeatRunsTheStepsThatManyTimes(t *testing.T) {
	s, err := demo.Open(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := demo.FromDemonstration("my settings tour", tourDemo(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSequence(a); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		request string
		times   int
	}{
		{"run my settings tour", 1},
		{"run my settings tour 3 times", 3},
		{"run my settings tour three times", 3},
		{"replay my settings tour 2 more times", 2},
	} {
		got, times, ok := s.Asked(c.request)
		if !ok {
			t.Fatalf("%q did not ask for a sequence", c.request)
		}
		if times != c.times {
			t.Errorf("%q asked for %d runs, want %d", c.request, times, c.times)
		}
		ex, _, err := s.ExpandRequest(c.request, fakeBinder{})
		if err != nil {
			t.Fatalf("%q: %v", c.request, err)
		}
		if want := len(got.Steps) * c.times; len(ex.Program.Steps) != want {
			t.Errorf("%q expanded into %d steps, want %d",
				c.request, len(ex.Program.Steps), want)
		}
		if err := program.Validate(ex.Program); err != nil {
			t.Errorf("%q expanded into an invalid program: %v", c.request, err)
		}
	}
	// AND THE CEILING IS A REFUSAL, not a silent clamp: somebody who asked for a
	// hundred runs and got twenty-five would be told their routine finished.
	_, err = demo.ExpandSequence(a, demo.MostRunsOfASequence+1, fakeBinder{})
	if err == nil {
		t.Fatal("a run count past the ceiling was accepted")
	}
	// AND IT REFUSES AS A RUN COUNT. Written as "it errors" first, and the mutation run
	// found that replacing the refusal with a silent clamp to 25 still errored -- from the
	// program validator, about the number of STEPS. Somebody who asked for a hundred runs
	// and got twenty-five would be told their routine finished.
	if !strings.Contains(err.Error(), "runs of") {
		t.Errorf("the ceiling refuses with %q, which says nothing about runs", err)
	}
}

// A SEQUENCE REQUEST BECOMES AN ORDINARY PROGRAM, through the one method the pipeline uses.
func TestASequenceRequestBecomesAnOrdinaryProgram(t *testing.T) {
	s, err := demo.Open(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := demo.FromDemonstration("my settings tour", tourDemo(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSequence(a); err != nil {
		t.Fatal(err)
	}
	// NOT A SEQUENCE: false, and no error, so the caller goes on to the ordinary path.
	if _, ok, err := s.ExpandRequest("click the save button", fakeBinder{}); ok || err != nil {
		t.Errorf("an ordinary request was claimed: ok=%v err=%v", ok, err)
	}
	ex, ok, err := s.ExpandRequest("run my settings tour", fakeBinder{})
	if !ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if ex.Procedure != `saved sequence "my settings tour"` {
		t.Errorf("procedure = %q", ex.Procedure)
	}
	// LEARNED, so the plan says where these steps came from. A person about to watch
	// Marco act is entitled to see that nobody wrote them by hand.
	if !strings.Contains(ex.Why, "demonstration") {
		t.Errorf("why = %q", ex.Why)
	}
	if err := program.ValidateBound(ex.Program); err != nil {
		t.Errorf("the bound validator refuses a replayed sequence: %v", err)
	}
	// ASKED WITH A NIL BINDER, which is how the pipeline tests recognition before it
	// observes anything. It must recognise and refuse rather than panic.
	if _, ok, _ := s.ExpandRequest("run my settings tour", nil); !ok {
		t.Error("recognition needs a binder, so a request costs an observation to reject")
	}
}

// A BOUNDED REPEAT DECLARES ITS RUNS, all the way to the program.
//
// The defect this is for, measured live: `run my settings tour 8 times` — a three-step routine —
// was refused as *"the request needs 24 steps and the limit is 10"*, which is the COMPLEXITY
// ceiling answering a question about REPETITION. See
// [[ADR-231-complexity-and-repetition-are-two-limits]].
//
// Through the production path, because the fix is a value travelling three hops — the artifact's
// run count to the procedure, the procedure to the expansion, the expansion to the program — and
// a test that built a Program itself would assert the limit and not the wiring. Deleting any of
// the three must fail this.
func TestABoundedRepeatDeclaresItsRunsToTheProgram(t *testing.T) {
	s, err := demo.Open(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := demo.FromDemonstration("my settings tour", tourDemo(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSequence(a); err != nil {
		t.Fatal(err)
	}

	// EIGHT RUNS OF THREE STEPS. Twenty-four steps, which is more than program.MaxSteps
	// and is not a decomposition of anything — it is eight runs of a three-step routine.
	ex, ok, err := s.ExpandRequest("run my settings tour 8 times", fakeBinder{})
	if !ok {
		t.Fatal("the request was not recognised as a sequence")
	}
	if err != nil {
		t.Fatalf("eight runs of a three-step routine was refused: %v", err)
	}
	if ex.Program.Runs != 8 {
		t.Errorf("the program says it is %d run(s), so the limits will be applied to "+
			"the wrong numbers", ex.Program.Runs)
	}
	if got := len(ex.Program.Steps); got != 24 {
		t.Fatalf("%d steps, want 24", got)
	}
	if got := ex.Program.Body(); got != 3 {
		t.Errorf("one run is %d step(s), want 3", got)
	}
	// AND THE ORDINARY VALIDATOR ACCEPTS IT, which is the thing that refused it live.
	if err := program.Validate(ex.Program); err != nil {
		t.Errorf("the ordinary validator still refuses a bounded repeat: %v", err)
	}
	// A SINGLE RUN DECLARES ONE, so a plain replay is not quietly a repeat of itself.
	one, _, err := s.ExpandRequest("run my settings tour", fakeBinder{})
	if err != nil {
		t.Fatal(err)
	}
	if one.Program.Runs > 1 {
		t.Errorf("a single run says it is %d runs", one.Program.Runs)
	}
	if one.Program.Body() != 3 {
		t.Errorf("a single run's body is %d step(s)", one.Program.Body())
	}
}
