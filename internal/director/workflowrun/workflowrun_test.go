package workflowrun_test

// ADVANCING A SAVED WORKFLOW, ONE GROUNDED ACTION AT A TIME.
//
// Everything here runs offline against a `directorapi.MarcoRunner` that writes the program down
// and performs nothing. That is the seam: the lowering, the validation and the operation are the
// production ones, and the last step is a notebook.
//
// It is deliberately NOT the whole seam. This package may not import `internal/driver` or
// `internal/runtime` — the Director's own boundary test walks these files too — so the real
// compiler cannot be reached from here. `cmd/director` has the other half: the same runner over
// the real front end and `recordhost`, which proves the program this builds actually compiles.
// Two tests because the boundary is real, not because one of them is weaker.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

var (
	saved  = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	looked = time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	// window is a target that does NOT fill the desktop, so every point this test checks
	// would be wrong if the offset were dropped.
	window = directorapi.Rect{X: 100, Y: 50, Width: 1000, Height: 500}
)

func rect(x, y, w, h float64) observe.Region {
	return observe.Region{X: x, Y: y, Width: w, Height: h}
}

// twoSteps is a saved workflow of two clicks across three screens.
func twoSteps(t *testing.T) workflowstore.Artifact {
	t.Helper()
	d := workflow.Draft{Name: "open free play"}
	d.Steps = []workflow.Step{
		{From: "visual_1", To: "visual_2", Target: "target_1", Cause: observe.CauseClick,
			Seen: 2, Readings: []observe.Region{rect(0.4, 0.5, 0.1, 0.04)}},
		{From: "visual_2", To: "visual_3", Target: "target_2", Cause: observe.CauseClick,
			Seen: 2, Readings: []observe.Region{rect(0.2, 0.3, 0.1, 0.04)}},
	}
	a, err := workflowstore.Save(d, map[observe.VisualScreenID]observe.VisualArrangement{
		"visual_1": {Cells: map[string]int{"0,0": 9}, Total: 9},
		"visual_2": {Cells: map[string]int{"1,1": 7}, Total: 7},
		"visual_3": {Cells: map[string]int{"2,2": 5}, Total: 5},
	}, saved)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// ── the seam ──────────────────────────────────────────────────────────────────

// notebook is a MarcoRunner that writes the program down and performs nothing.
type notebook struct {
	programs []string
	names    []string
	err      error
	// after runs once the program has been written down, so a test can make the world
	// move in the gap between two operations of one action.
	after func()
}

// clicks is how many of the programs written down actually press something.
//
// An action is a move and then a press — see DefaultDwell — so counting PROGRAMS answers a
// different question from the one most of these tests are asking. Where the assertion is "one
// press went out", this is the count it means.
func (n *notebook) clicks() int {
	c := 0
	for _, p := range n.programs {
		if strings.Contains(p, "OS's Click") {
			c++
		}
	}
	return c
}

func (n *notebook) Run(ctx context.Context, name, program string) (
	directorapi.MarcoResult, error) {

	n.programs = append(n.programs, program)
	n.names = append(n.names, name)
	if n.after != nil {
		n.after()
	}
	if n.err != nil {
		return directorapi.MarcoResult{}, n.err
	}
	return directorapi.MarcoResult{Output: "[dryrun] OS's Click\n"}, nil
}

// ── the fixtures for the four things the coordinator may not do itself ────────

// oracle stands where a person looking at the screen would. SIMULATED: it answers instantly and
// correctly, which no person does.
type oracle map[observe.VisualScreenID]string

func (o oracle) Establish(ctx context.Context, s *workflowstore.SessionBinding,
	at workflowrun.Sighting) error {

	label, ok := o[at.Screen]
	if !ok {
		return errors.New("nobody could say what " + string(at.Screen) + " is")
	}
	return s.Confirm(label, at.Screen, at.Epoch, at.Generation, "SIMULATED fixture", at.At)
}

// grants authorises everything it is asked, in the observation it was asked in.
type grants struct {
	asked []workflowrun.Intent
	// before runs before answering, so a test can have somebody press stop at the prompt.
	before func()
	// bend spoils the authorisation, so each way of being unspendable gets its own test.
	bend func(workflowrun.Authorisation) workflowrun.Authorisation
	no   bool
	// epoch and generation are the observation the fixture claims to have been asked in,
	// so a test can make a grant deliberately stale.
	epoch, generation uint64
}

func (g *grants) Authorise(ctx context.Context, in workflowrun.Intent) (
	workflowrun.Authorisation, bool) {

	g.asked = append(g.asked, in)
	if g.before != nil {
		g.before()
	}
	if g.no {
		return workflowrun.Authorisation{}, false
	}
	a := workflowrun.Authorisation{
		Step: in.Step, By: "SIMULATED fixture",
		Epoch: g.epoch, Generation: g.generation,
		// THE WINDOW FROM THE QUESTION, which is what a well-behaved granter
		// does: it copies back what it was asked about. Copying it proves
		// nothing about the world — the runner re-observes that — and it is what
		// makes a grant about ANOTHER window unspendable here.
		Window:  in.Window,
		Expires: looked.Add(time.Minute),
	}
	if g.bend != nil {
		a = g.bend(a)
	}
	return a, true
}

// eyes replays a scripted sequence of observations.
type eyes struct {
	looks []workflowrun.Sighting
	n     int
}

func (e *eyes) Look(ctx context.Context) (workflowrun.Sighting, error) {
	if e.n >= len(e.looks) {
		return workflowrun.Sighting{}, errors.New("the script ran out of observations")
	}
	e.n++
	return e.looks[e.n-1], nil
}

// ── the harness ───────────────────────────────────────────────────────────────

// rig builds a runner that CAN act, over a notebook, with everything else satisfied.
type rig struct {
	art  workflowstore.Artifact
	run  *workflowrun.Runner
	s    *workflowstore.SessionBinding
	book *notebook
	auth *grants
	// front is what the platform answers about the foreground, and moved whether the
	// observation has outrun the look the plan was made from.
	front bool
	moved bool
}

func newRig(t *testing.T) *rig {
	t.Helper()
	art := twoSteps(t)
	book := &notebook{}
	auth := &grants{epoch: 5, generation: 1}
	g := &rig{art: art, book: book, auth: auth, front: true}
	g.s = workflowstore.NewSessionBinding()
	// A PERSON AGREED TO THE STEPS. Separate from authorising them: agreeing that this is
	// the Play menu, agreeing to press something on it, and permitting the press now are
	// three sentences, and the coordinator conflates none of them.
	for i := range art.Steps {
		if err := g.s.ConfirmStep(i, "SIMULATED fixture"); err != nil {
			t.Fatal(err)
		}
	}
	g.run = workflowrun.New(art, nil,
		oracle{"now_1": "screen_a", "now_2": "screen_b", "now_3": "screen_c"}, auth).
		WithExecutor(marcoexec.New(book), false).
		WithConfirm(func(ctx context.Context, planned workflowrun.Sighting) (
			bool, string) {
			if g.moved {
				return false, "the screen changed while the question was open"
			}
			if g.front {
				return true, ""
			}
			return false, "Notepad is in front"
		}).
		WithClock(func() time.Time { return looked })
	return g
}

// on is one observation of one screen, with the step's control found where the file says.
func on(screen observe.VisualScreenID, epoch uint64, control string,
	where observe.Region) workflowrun.Sighting {

	return workflowrun.Sighting{
		Screen: screen, Epoch: epoch, Generation: 1,
		Window: "hwnd:1", Bounds: window, Front: true, At: looked,
		Outcomes: []observe.Reacquisition{{
			Target: observe.VisualTargetID(control), Screen: screen,
			Result: observe.ReacquireMatched, Where: where, Controls: 1,
			Because: "a fixture outcome",
		}},
	}
}

// firstStep is the observation on which step 0 is plannable.
func firstStep() workflowrun.Sighting {
	return on("now_1", 5, "control_1", rect(0.4, 0.5, 0.1, 0.04))
}

// ── what it does ──────────────────────────────────────────────────────────────

// ONE GROUNDED ACTION, THEN THE DESTINATION, THEN THE NEXT STEP.
func TestAWorkflowAdvancesOneGroundedActionAtATime(t *testing.T) {
	g := newRig(t)
	ctx := context.Background()

	first := g.run.Advance(ctx, g.s, firstStep())
	if first.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("step 0: %s — %s", first.Outcome, first.Because)
	}
	if g.book.clicks() != 1 {
		t.Fatalf("%d press(es) reached Marco, want 1", g.book.clicks())
	}
	if g.run.Cursor() != 0 {
		t.Errorf("the cursor moved to %d on a press; a step moves on by arriving",
			g.run.Cursor())
	}

	// THE OBSERVATION AFTER THE ACTION BOUNDARY. The screen moved, so the epoch did.
	g.auth.epoch = 6
	second := g.run.Advance(ctx, g.s, on("now_2", 6, "control_2", rect(0.2, 0.3, 0.1, 0.04)))
	if second.Outcome != workflowrun.OutcomeArrived {
		t.Fatalf("after the press: %s — %s", second.Outcome, second.Because)
	}
	if second.Arrival == nil || !second.Arrival.Established() {
		t.Fatal("the destination was not established")
	}
	if g.run.Cursor() != 1 {
		t.Errorf("the walk is on step %d, want 1", g.run.Cursor())
	}

	// AND THE NEXT STEP IS A SEPARATE ACTION, ASKED ABOUT SEPARATELY.
	third := g.run.Advance(ctx, g.s, on("now_2", 6, "control_2", rect(0.2, 0.3, 0.1, 0.04)))
	if third.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("step 1: %s — %s", third.Outcome, third.Because)
	}
	if len(g.auth.asked) != 2 {
		t.Errorf("somebody was asked %d time(s) for 2 actions; an authorisation is "+
			"for one step", len(g.auth.asked))
	}
}

// THE PROGRAM IS LEGAL MARCO AND IT IS THE ONLY WAY ANYTHING IS SENT.
func TestEveryActionGoesThroughMarcoAsAClickAtAPoint(t *testing.T) {
	g := newRig(t)
	if got := g.run.Advance(context.Background(), g.s, firstStep()); !got.Acted() {
		t.Fatalf("nothing was run: %s — %s", got.Outcome, got.Because)
	}
	// THE PRESS, which is the second program: an action is a move and then a press.
	src := g.book.programs[len(g.book.programs)-1]
	for _, want := range []string{
		"use os.",
		"the DirectorClick is an actor.",
		"the at is a Point with X 550, Y 310.",
		"do OS's Click with at.",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the program does not contain %q:\n%s", want, src)
		}
	}
	// AND NOTHING ELSE. A coordinator that also typed, or keyed, or activated something
	// would be doing more than the step said.
	for _, forbidden := range []string{"OS's Type", "OS's Key", "OS's Activate", "Secret"} {
		if strings.Contains(src, forbidden) {
			t.Errorf("the program reaches %s:\n%s", forbidden, src)
		}
	}
}

// THE POINT IS IN DESKTOP COORDINATES, which is the window's offset plus the control's centre.
//
// Its own test because the failure is invisible: a coordinator that forgot the offset clicks
// 100px up and left of the control, every time, and the program it produced is still legal
// Marco that compiles and runs.
func TestThePressLandsAtTheControlOnTheDesktopAndNotInTheWindowsOwnCoordinates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		bounds directorapi.Rect
		where  observe.Region
		want   workflowrun.Point
	}{
		{"an offset window", directorapi.Rect{X: 100, Y: 50, Width: 1000, Height: 500},
			rect(0.4, 0.5, 0.1, 0.04), workflowrun.Point{X: 550, Y: 310}},
		{"a window at the origin", directorapi.Rect{Width: 1000, Height: 500},
			rect(0.4, 0.5, 0.1, 0.04), workflowrun.Point{X: 450, Y: 260}},
		{"a window on a second monitor",
			directorapi.Rect{X: -1920, Y: 0, Width: 1920, Height: 1080},
			// A REAL REGION whose centre is the middle of the window, rather
			// than a zero-extent one at the middle. The arithmetic under test is
			// the same and a region with no extent is now refused outright: it
			// locates nothing, which is observe.Region.Locates's first sentence.
			rect(0.45, 0.45, 0.1, 0.1), workflowrun.Point{X: -960, Y: 540}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t)
			look := firstStep()
			look.Bounds = tc.bounds
			look.Outcomes[0].Where = tc.where
			got := g.run.Advance(context.Background(), g.s, look)
			if !got.Acted() {
				t.Fatalf("nothing was run: %s — %s", got.Outcome, got.Because)
			}
			want := fmt.Sprintf("the at is a Point with X %d, Y %d.",
				tc.want.X, tc.want.Y)
			if !strings.Contains(g.book.programs[0], want) {
				t.Errorf("want %q in:\n%s", want, g.book.programs[0])
			}
			if got := g.auth.asked[0].At; got != tc.want {
				t.Errorf("the person was asked to authorise a press at %v and the "+
					"program presses at %v; they must be the same point",
					got, tc.want)
			}
		})
	}
}

// ── what it refuses ───────────────────────────────────────────────────────────

// A RUNNER WITH NO EXECUTOR CANNOT ACT, AND THAT IS WHAT IT IS BUILT AS.
func TestNothingIsPressedWithoutAnExecutor(t *testing.T) {
	art := twoSteps(t)
	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED fixture"); err != nil {
		t.Fatal(err)
	}
	r := workflowrun.New(art, nil, oracle{"now_1": "screen_a"},
		&grants{epoch: 5, generation: 1}).
		WithConfirm(func(context.Context, workflowrun.Sighting) (bool, string) {
			return true, ""
		}).
		WithClock(func() time.Time { return looked })
	if r.Acts() {
		t.Fatal("a runner nobody gave an executor reports that it can act")
	}
	got := r.Advance(context.Background(), s, firstStep())
	if got.Outcome != workflowrun.OutcomeRefused {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if got.Acted() {
		t.Fatal("something was run through a runner that has no executor")
	}
	if !strings.Contains(got.Because, "no executor") {
		t.Errorf("the refusal does not say why: %s", got.Because)
	}
}

// A RUNNER THAT CANNOT RE-OBSERVE THE WORLD REFUSES TO ACT.
//
// Not "assumes it is fine". An unverifiable precondition on an operation that clicks wherever
// the pointer is, is not a precondition — and `WithConfirm(nil)` satisfies every structural check
// while switching the gate off, which is why this asserts on the argument and not on the call.
func TestARunnerThatCannotRecheckTheWorldRefusesToAct(t *testing.T) {
	for _, tc := range []struct {
		name    string
		install bool
	}{
		{"nobody installed the question", false},
		{"somebody installed nil", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t)
			if tc.install {
				g.run.WithConfirm(nil)
			} else {
				g.run = workflowrun.New(g.art, nil,
					oracle{"now_1": "screen_a"}, g.auth).
					WithExecutor(marcoexec.New(g.book), false).
					WithClock(func() time.Time { return looked })
			}
			got := g.run.Advance(context.Background(), g.s, firstStep())
			if got.Outcome != workflowrun.OutcomeRefused {
				t.Fatalf("%s — %s", got.Outcome, got.Because)
			}
			if len(g.book.programs) != 0 {
				t.Fatal("a program was sent without the foreground being confirmed")
			}
			if !strings.Contains(got.Because, "question about the world") {
				t.Errorf("the refusal does not name what is missing: %s", got.Because)
			}
		})
	}
}

// AND ONE THAT ASKS AND IS TOLD NO SENDS NOTHING.
func TestAnActionIsRefusedWhenTheTargetDoesNotLeadTheDesktop(t *testing.T) {
	g := newRig(t)
	g.front = false
	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomeRefused {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if len(g.book.programs) != 0 {
		t.Fatal("a click was sent while something else led the desktop")
	}
	if !strings.Contains(got.Because, "no longer the one this action was planned") ||
		!strings.Contains(got.Because, "Notepad") {
		t.Errorf("the refusal does not say what is in front: %s", got.Because)
	}
}

// A STOP DURING THE LOOK THAT PLANNED AN ACTION PREVENTS THAT ACTION.
//
// Not the one after it. Somebody pressing stop at the authorisation prompt is asking for THIS
// press not to happen, and a stop that only took effect at the next step would be the one thing
// they were trying to prevent.
func TestAStopAtThePromptPreventsThatPress(t *testing.T) {
	g := newRig(t)
	g.auth.before = func() { g.run.Stop() }
	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomeStopped {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if len(g.book.programs) != 0 {
		t.Fatal("a press was sent after somebody stopped the run")
	}
	if !g.run.Stopped() {
		t.Error("the runner does not report that it was stopped")
	}
}

// AND A CANCELLED CONTEXT DOES NOTHING AT ALL — not "sends nothing".
//
// The distinction is what a mutation run found. Removing the check at the TOP of Advance left
// this test passing, because the check before the press caught it — and in between, a cancelled
// run grounded a screen and PUT A QUESTION TO SOMEBODY. A run that has been cancelled must not
// ask anybody anything, so the assertions are about the whole observation and not only about
// what reached Marco.
func TestACancelledContextDoesNothingAtAll(t *testing.T) {
	g := newRig(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := g.run.Advance(ctx, g.s, firstStep())
	if got.Outcome != workflowrun.OutcomeStopped {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if len(g.book.programs) != 0 {
		t.Error("a press was sent on a cancelled context")
	}
	if len(g.auth.asked) != 0 {
		t.Error("somebody was asked to authorise an action on a cancelled run")
	}
	if _, ok := g.s.Established("screen_a", 5, 1); ok {
		t.Error("a screen was established on a cancelled run")
	}
	if got.Stage != "" {
		t.Errorf("the step was examined on a cancelled run, reaching %q", got.Stage)
	}
}

// NOBODY AUTHORISED IT, SO NOTHING WAS SENT.
func TestNobodyAuthorisedMeansNothingIsSent(t *testing.T) {
	for _, tc := range []struct {
		name string
		rig  func(*rig)
		want string
	}{
		{"somebody said no", func(g *rig) { g.auth.no = true }, "nobody authorised"},
		{"there is nobody to ask", func(g *rig) {
			g.run = workflowrun.New(g.art, nil,
				oracle{"now_1": "screen_a"}, nil).
				WithExecutor(marcoexec.New(g.book), false).
				WithConfirm(func(context.Context,
					workflowrun.Sighting) (bool, string) {
					return true, ""
				}).
				WithClock(func() time.Time { return looked })
		}, "nobody authorised"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t)
			tc.rig(g)
			got := g.run.Advance(context.Background(), g.s, firstStep())
			if got.Outcome != workflowrun.OutcomeRefused {
				t.Fatalf("%s — %s", got.Outcome, got.Because)
			}
			if len(g.book.programs) != 0 {
				t.Fatal("a press was sent with no authorisation")
			}
			if !strings.Contains(got.Because, tc.want) {
				t.Errorf("want %q in: %s", tc.want, got.Because)
			}
		})
	}
}

// AN AUTHORISATION IS FOR ONE STEP, IN ONE OBSERVATION, FROM SOMEBODY, AND IT EXPIRES.
//
// Four separate ways to be unspendable, each with its own answer, because a caller that gets one
// "no" cannot tell a person whether to look again, agree again, or stop.
func TestAnAuthorisationThatCannotBeSpentHereIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		bend func(workflowrun.Authorisation) workflowrun.Authorisation
		want string
	}{
		{"for another step", func(a workflowrun.Authorisation) workflowrun.Authorisation {
			a.Step = 7
			return a
		}, "is for step 7"},
		{"from another observation",
			func(a workflowrun.Authorisation) workflowrun.Authorisation {
				a.Epoch = 99
				return a
			}, "is not the screen standing now"},
		{"from another window",
			func(a workflowrun.Authorisation) workflowrun.Authorisation {
				a.Generation = 99
				return a
			}, "is not the screen standing now"},
		{"from nobody", func(a workflowrun.Authorisation) workflowrun.Authorisation {
			a.By = ""
			return a
		}, "names nobody"},
		{"with no expiry", func(a workflowrun.Authorisation) workflowrun.Authorisation {
			a.Expires = time.Time{}
			return a
		}, "no expiry"},
		{"lapsed", func(a workflowrun.Authorisation) workflowrun.Authorisation {
			a.Expires = looked.Add(-time.Second)
			return a
		}, "lapsed at"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t)
			g.auth.bend = tc.bend
			got := g.run.Advance(context.Background(), g.s, firstStep())
			if got.Outcome != workflowrun.OutcomeRefused {
				t.Fatalf("%s — %s", got.Outcome, got.Because)
			}
			if len(g.book.programs) != 0 {
				t.Fatal("a press was sent on an authorisation that could not be spent")
			}
			if !strings.Contains(got.Because, tc.want) {
				t.Errorf("want %q in: %s", tc.want, got.Because)
			}
		})
	}
}

// A LOOK NOBODY COULD TAKE GROUNDS NOTHING AND PRESSES NOTHING.
func TestALookNobodyCouldTakeIsNotActedOn(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spoil func(*workflowrun.Sighting)
		want  string
	}{
		{"no pixels", func(s *workflowrun.Sighting) { s.Missing = true }, "no pixels"},
		{"something else in front", func(s *workflowrun.Sighting) { s.Front = false },
			"not the window in front"},
		{"no screen standing", func(s *workflowrun.Sighting) { s.Screen = "" },
			"no screen hypothesis"},
		{"a window with no rectangle",
			func(s *workflowrun.Sighting) { s.Bounds = directorapi.Rect{} },
			"no rectangle"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t)
			look := firstStep()
			tc.spoil(&look)
			got := g.run.Advance(context.Background(), g.s, look)
			if got.Outcome != workflowrun.OutcomeUnobserved {
				t.Fatalf("%s — %s", got.Outcome, got.Because)
			}
			if len(g.book.programs) != 0 {
				t.Fatal("a press was sent from a look nobody could take")
			}
			if len(g.auth.asked) != 0 {
				t.Fatal("somebody was asked to authorise a press on a look " +
					"nobody could take")
			}
			if !strings.Contains(got.Because, tc.want) {
				t.Errorf("want %q in: %s", tc.want, got.Because)
			}
			// AND NOTHING WAS ESTABLISHED FROM IT.
			if _, ok := g.s.Established("screen_a", look.Epoch, look.Generation); ok {
				t.Error("a screen was established from a look nobody could take")
			}
		})
	}
}

// A STEP ALREADY ACTED ON IS NEVER ACTED ON AGAIN.
//
// A second click on a menu that DID respond is a click on whatever replaced the control.
func TestAPressedStepIsNeverPressedTwice(t *testing.T) {
	g := newRig(t)
	ctx := context.Background()
	if got := g.run.Advance(ctx, g.s, firstStep()); got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	// THE SAME OBSERVATION AGAIN: the screen did not move, so nothing arrived.
	again := g.run.Advance(ctx, g.s, firstStep())
	if again.Outcome != workflowrun.OutcomeWaiting {
		t.Fatalf("%s — %s", again.Outcome, again.Because)
	}
	if g.book.clicks() != 1 {
		t.Fatalf("%d press(es) reached Marco for one step", g.book.clicks())
	}
	if !strings.Contains(again.Because, "acted on") {
		t.Errorf("the wait does not say the press already went out: %s", again.Because)
	}
}

// AND A PRESS WHOSE DESTINATION NEVER ARRIVES STOPS THE RUN SAFELY.
//
// The single most important outcome in this package: the press was sent, nothing grounded the
// screen it was supposed to produce, and that is neither a completed step nor a failed one.
func TestAPressWhoseDestinationNeverArrivesStopsSafely(t *testing.T) {
	g := newRig(t)
	g.run.WithPatience(3)
	ctx := context.Background()
	if got := g.run.Advance(ctx, g.s, firstStep()); got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	var last workflowrun.Attempt
	for i := 0; i < 4; i++ {
		last = g.run.Advance(ctx, g.s, firstStep())
	}
	if last.Outcome != workflowrun.OutcomeRefused {
		t.Fatalf("after running out of patience: %s — %s", last.Outcome, last.Because)
	}
	for _, want := range []string{"was performed", "was not established", "nor a failed one"} {
		if !strings.Contains(last.Because, want) {
			t.Errorf("want %q in: %s", want, last.Because)
		}
	}
	rep := g.run.Report()
	if !rep.Progress.Partial() && rep.Progress.Reached != 0 {
		t.Errorf("progress: %s", rep.Progress.Summary())
	}
	if rep.Pressed != 1 {
		t.Errorf("%d press(es) recorded, want 1", rep.Pressed)
	}
	if rep.Established != 0 {
		t.Errorf("%d destination(s) established, want 0", rep.Established)
	}
	if g.book.clicks() != 1 {
		t.Fatalf("%d press(es) sent while waiting", g.book.clicks())
	}
}

// A REFUSAL IS RETRIED AND THEN THE RUN STOPS SAFELY.
//
// Retried because a control mid-animation is the ordinary case; bounded because a refusal that
// never resolves is the commonest one there is.
func TestARefusalIsRetriedAndThenStopsSafely(t *testing.T) {
	g := newRig(t)
	g.run.WithPatience(2)
	ctx := context.Background()
	blind := firstStep()
	blind.Outcomes = nil // nothing was offered for the control on this look

	var last workflowrun.Attempt
	waits := 0
	for i := 0; i < 4; i++ {
		last = g.run.Advance(ctx, g.s, blind)
		if last.Outcome == workflowrun.OutcomeWaiting {
			waits++
		}
	}
	if waits != 2 {
		t.Errorf("the step was retried %d time(s) with a patience of 2", waits)
	}
	if last.Outcome != workflowrun.OutcomeRefused {
		t.Fatalf("%s — %s", last.Outcome, last.Because)
	}
	if last.Stage != workflowstore.StepControlUnresolved {
		t.Errorf("stopped at stage %q, want the control refusal", last.Stage)
	}
	if len(g.auth.asked) != 0 {
		t.Error("somebody was asked to authorise a press for a control nobody found")
	}
	if rep := g.run.Report(); rep.Progress.Complete() {
		t.Errorf("a run that never reached a step reports complete: %s",
			rep.Progress.Summary())
	}
}

// MARCO REFUSING THE PROGRAM MEANS THE DESKTOP WAS NEVER TOUCHED, AND THE RUN STOPS.
func TestAProgramMarcoRefusesStopsTheRun(t *testing.T) {
	g := newRig(t)
	g.book.err = errors.New("compile: OS has no capability \"Click\"")
	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomeFailed {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if got.Result == nil || got.Result.Status != marcoexec.StatusCompileFailed {
		t.Fatalf("status %v — a compile failure is the one that proves nothing ran",
			got.Result)
	}
	if rep := g.run.Report(); rep.Pressed != 0 {
		t.Errorf("%d press(es) counted for a program that did not compile", rep.Pressed)
	}
}

// THE SCREEN CHANGED WHILE SOMEBODY WAS READING THE PROMPT, SO NOTHING WAS SENT.
//
// The hole this closes is specific and would never look like a bug. Asking a person for
// authorisation is an unbounded pause in the middle of the one sequence that must not have one:
// the look that found the control is seconds old by the time they answer, and in those seconds a
// menu can animate, a match can start, a notification can take the window. The authorisation
// carries the observation it was granted in, which catches the case where somebody else noticed
// — and this is the case where nobody did.
//
// So the world is re-observed at the instant of the press, and a moved observation refuses.
func TestAWorldThatMovedWhileTheQuestionWasOpenSendsNothing(t *testing.T) {
	g := newRig(t)
	// Somebody takes long enough to answer that the screen changes under them.
	g.auth.before = func() { g.moved = true }

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomeRefused {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if len(g.book.programs) != 0 {
		t.Fatal("a click was sent at a point on a screen that had already changed")
	}
	if len(g.auth.asked) != 1 {
		t.Error("the person was not asked, so this is not the case being tested")
	}
	for _, want := range []string{"no longer the one this action was planned against",
		"while the question was open"} {
		if !strings.Contains(got.Because, want) {
			t.Errorf("want %q in: %s", want, got.Because)
		}
	}
}

// AND THE RE-OBSERVATION IS HANDED THE LOOK THE PLAN WAS MADE FROM.
//
// Without the argument it could only report what it sees, and "has this moved" is a comparison.
func TestTheRecheckIsToldWhichObservationThePlanWasMadeFrom(t *testing.T) {
	g := newRig(t)
	var saw []workflowrun.Sighting
	g.run.WithConfirm(func(ctx context.Context, planned workflowrun.Sighting) (bool, string) {
		saw = append(saw, planned)
		return true, ""
	})
	look := firstStep()
	if got := g.run.Advance(context.Background(), g.s, look); !got.Acted() {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	// TWICE, and both times about the look the plan was made from.
	//
	// Once before the pointer moves and once after it has rested, because the dwell is a
	// gap like any other and the world can change inside it. Both are told about the
	// PLANNED sighting rather than about each other: the question is always whether the
	// world is still the one this action was planned against.
	if len(saw) != 2 {
		t.Fatalf("the world was re-observed %d time(s) for one action, want 2 — "+
			"before the move and after the rest", len(saw))
	}
	for i, got := range saw {
		if got.Screen != saw[0].Screen || got.Epoch != saw[0].Epoch ||
			got.Generation != saw[0].Generation || got.Window != saw[0].Window {
			t.Errorf("re-observation %d was told about a different sighting from "+
				"the first", i)
		}
	}
	if saw[0].Epoch != look.Epoch || saw[0].Generation != look.Generation ||
		saw[0].Screen != look.Screen {
		t.Errorf("the recheck was told about %s %d/%d and the plan was made from "+
			"%s %d/%d", saw[0].Screen, saw[0].Epoch, saw[0].Generation,
			look.Screen, look.Epoch, look.Generation)
	}
}

// ── a wait, and the two grounds an arrival can have ───────────────────────────

// A WAIT ADVANCES BY ARRIVING AND PRESSES NOTHING.
func TestAWaitAdvancesByArrivingAndPressesNothing(t *testing.T) {
	d := workflow.Draft{Name: "wait for the load"}
	d.Steps = []workflow.Step{
		{From: "visual_1", To: "visual_2", Cause: observe.CauseUnknown, Seen: 2},
	}
	art, err := workflowstore.Save(d, map[observe.VisualScreenID]observe.VisualArrangement{
		"visual_1": {Cells: map[string]int{"0,0": 9}, Total: 9},
		"visual_2": {Cells: map[string]int{"1,1": 7}, Total: 7},
	}, saved)
	if err != nil {
		t.Fatal(err)
	}
	book := &notebook{}
	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED fixture"); err != nil {
		t.Fatal(err)
	}
	r := workflowrun.New(art, nil, oracle{"now_1": "screen_a", "now_2": "screen_b"},
		&grants{epoch: 5, generation: 1}).
		WithExecutor(marcoexec.New(book), false).
		WithConfirm(func(context.Context, workflowrun.Sighting) (bool, string) {
			return true, ""
		}).
		WithClock(func() time.Time { return looked })
	ctx := context.Background()

	waiting := r.Advance(ctx, s, on("now_1", 5, "control_1", rect(0, 0, 0, 0)))
	if waiting.Outcome != workflowrun.OutcomeWaiting {
		t.Fatalf("%s — %s", waiting.Outcome, waiting.Because)
	}
	if waiting.Stage != workflowstore.StepNotRepeatable {
		t.Errorf("stage %q, want the wait", waiting.Stage)
	}
	arrived := r.Advance(ctx, s, on("now_2", 6, "control_1", rect(0, 0, 0, 0)))
	if arrived.Outcome != workflowrun.OutcomeArrived {
		t.Fatalf("%s — %s", arrived.Outcome, arrived.Because)
	}
	if len(book.programs) != 0 {
		t.Fatal("a wait pressed something")
	}
	done := r.Advance(ctx, s, on("now_2", 6, "control_1", rect(0, 0, 0, 0)))
	if done.Outcome != workflowrun.OutcomeComplete {
		t.Fatalf("%s — %s", done.Outcome, done.Because)
	}
	if rep := r.Report(); !rep.Progress.Complete() {
		t.Errorf("progress: %s", rep.Progress.Summary())
	}
}

// AN INFERRED ARRIVAL IS COUNTED AND IS NOT COUNTED AS INDEPENDENT.
//
// The circularity the arrival contract exists to label: the workflow said to expect this
// destination, so the workflow cannot cite it as corroboration of its own step.
func TestAnInferredArrivalIsNotCountedAsIndependentEvidence(t *testing.T) {
	g := newRig(t)
	ctx := context.Background()
	if got := g.run.Advance(ctx, g.s, firstStep()); got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	// NOBODY LOOKED THIS TIME. The workflow's own transition plus an agreeing
	// arrangement is what establishes screen_b, and the grounder adds nothing — so the
	// only thing standing behind the arrival is the workflow's own expectation.
	g.run.WithGrounder(inferrer{})
	if _, err := g.s.Infer(g.art, "screen_a", "screen_b", "now_2", 1.0, 6, 1); err != nil {
		t.Fatal(err)
	}
	got := g.run.Advance(ctx, g.s, on("now_2", 6, "control_2", rect(0.2, 0.3, 0.1, 0.04)))
	if got.Outcome != workflowrun.OutcomeArrived {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if !got.Arrival.Established() {
		t.Fatal("not established")
	}
	if got.Arrival.Independent() {
		t.Fatal("an inference drawn from the workflow's own transition is being " +
			"counted as independent evidence about that workflow")
	}
	rep := g.run.Report()
	if rep.Established != 1 || rep.Independent != 0 {
		t.Errorf("%d established, %d independent — want 1 and 0",
			rep.Established, rep.Independent)
	}
}

// inferrer establishes nothing, so the test above has only the workflow's own transition.
type inferrer struct{}

func (inferrer) Establish(context.Context, *workflowstore.SessionBinding,
	workflowrun.Sighting) error {
	return nil
}

// AND NO ARRIVAL, HOWEVER GROUNDED, IS A VERIFIED ACTION.
func TestNoRunReportsAVerifiedAction(t *testing.T) {
	g := newRig(t)
	ctx := context.Background()
	g.run.Advance(ctx, g.s, firstStep())
	g.auth.epoch = 6
	got := g.run.Advance(ctx, g.s, on("now_2", 6, "control_2", rect(0.2, 0.3, 0.1, 0.04)))
	if got.Arrival == nil || !got.Arrival.Established() {
		t.Fatalf("the fixture did not establish the destination: %s", got.Because)
	}
	ok, why := g.run.Report().Verified()
	if ok {
		t.Fatal("a run reported an action as independently verified")
	}
	// THE CLAIM IS THE SAME WHATEVER THE RUN DID. What varies is the sentence around
	// it, and this run performed an action — so a reason saying nothing was executed
	// would be describing a different run. See Report.Verified.
	if !strings.Contains(why, "not by itself proof") {
		t.Errorf("the verdict drops the claim it exists to make: %s", why)
	}
	if strings.Contains(why, "nothing was executed") {
		t.Errorf("a run that performed an action says nothing was executed: %s", why)
	}
}

// ── the loop ──────────────────────────────────────────────────────────────────

// RUN DRIVES THE SAME WALK AND REPORTS PARTIAL PROGRESS.
func TestRunStopsSafelyPartwayAndSaysSo(t *testing.T) {
	g := newRig(t)
	g.run.WithPatience(2)
	// Step 0 is pressed and arrives; step 1's control is never offered.
	blind := on("now_2", 6, "control_9", rect(0.2, 0.3, 0.1, 0.04))
	e := &eyes{looks: []workflowrun.Sighting{
		firstStep(),
		on("now_2", 6, "control_2", rect(0.2, 0.3, 0.1, 0.04)),
		blind, blind, blind, blind,
	}}
	r := workflowrun.New(g.art, e,
		oracle{"now_1": "screen_a", "now_2": "screen_b", "now_3": "screen_c"},
		&grants{epoch: 5, generation: 1}).
		WithExecutor(marcoexec.New(g.book), false).
		WithConfirm(func(context.Context, workflowrun.Sighting) (bool, string) {
			return true, ""
		}).
		WithClock(func() time.Time { return looked }).
		WithPatience(2)
	rep, err := r.Run(context.Background(), g.s, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Progress.Partial() {
		t.Fatalf("progress: %s", rep.Progress.Summary())
	}
	if rep.Progress.Reached != 1 || rep.Progress.Stopped != 1 {
		t.Errorf("reached %d, stopped at %d — want 1 and 1",
			rep.Progress.Reached, rep.Progress.Stopped)
	}
	if !strings.Contains(rep.Progress.Summary(), "STOPPED SAFELY") {
		t.Errorf("summary: %s", rep.Progress.Summary())
	}
	if rep.Real {
		t.Error("a dry run reports that it reached a computer")
	}
}

// AND A RUNNER WITH NO EYES CANNOT BE DRIVEN.
func TestARunnerWithNoEyesCannotBeDriven(t *testing.T) {
	g := newRig(t)
	if _, err := g.run.Run(context.Background(), g.s, 4); err == nil {
		t.Fatal("a runner with no eyes ran")
	}
}
