package workflowrun_test

// TWO ACTIONS, AND THE SECOND ONE WAITS FOR THE FIRST ONE'S ANSWER.
//
// # The condition
//
// Everything this subsystem does for a one-action workflow was built and measured. The step this
// file is about is the one that makes a WORKFLOW out of it: a second action, authorised only if
// the first one did what the demonstration said it would.
//
// It did not hold. A step whose two ends are DIFFERENT screens advanced the moment its
// destination was established — `arrival.Established()` and then `advance()`, with its own
// post-action evidence never consulted. So on a two-step routine the second action could be
// authorised and sent while the first action's outcome was still being decided, and the verdict
// would then be reached about a screen the second press had already changed.
// [[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]] traced it and deferred it;
// this is it closed.
//
// A screen change is weaker evidence than an outcome, not stronger: the destination arriving says
// the interface moved, the outcome says it moved the way it was shown to. `verify.go` already held
// the policy for the same-screen case — nothing proceeds to another action on an unconfirmed
// one — and these are that policy on the path it had been written around.
//
// # What these drive
//
// The production coordinator, the production verifier, and a notebook where the mouse would be.
// Each one asserts the thing an acceptance has to assert and a unit test usually does not: not
// only the verdict, but whether the SECOND ACTION WAS EVER ASKED ABOUT.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aTwoStepWalk is two presses across three screens, each step with an outcome of its own.
//
// Different labels at each end, which is the case this file exists for: such a step CAN arrive,
// and arriving is exactly what used to finish it.
func aTwoStepWalk() workflowstore.Artifact {
	control := func(label string, y float64) workflowstore.Control {
		return workflowstore.Control{Label: label,
			Readings: []observe.Region{
				{X: 0.410, Y: y, Width: 0.072, Height: 0.028}},
			Containers: []observe.Region{
				{X: 0.027, Y: y - 0.011, Width: 0.946, Height: 0.050}}}
	}
	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: "two taps",
		SavedAt: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
		Screens: []workflowstore.Screen{
			{Label: "screen_a"}, {Label: "screen_b"}, {Label: "screen_c"},
		},
		Controls: []workflowstore.Control{
			control("control_1", 0.241), control("control_2", 0.291),
		},
		Steps: []workflowstore.Step{
			{
				Position: 0, From: "screen_a", To: "screen_b",
				Control: "control_1", Cause: observe.CauseClick, Seen: 2,
				Outcome: []string{"bloom", "foliage", "presets", "shadows",
					"textures"},
			},
			{
				Position: 1, From: "screen_b", To: "screen_c",
				Control: "control_2", Cause: observe.CauseClick, Seen: 2,
				Outcome: []string{"ambient", "master", "music", "speech",
					"voice"},
			},
		},
	}
}

// aWalkingGrounder names each live screen as the saved screen a test says it is.
//
// Stands where `promptGrounder` stands, and establishes nothing by itself: a label per observed
// screen id, which is the correspondence a person makes. The destination of a step arrives when
// the label of its `To` is established in the observation — see `workflowstore.DestinationAt`.
type aWalkingGrounder struct {
	says map[observe.VisualScreenID]string
}

func (g aWalkingGrounder) Establish(ctx context.Context, s *workflowstore.SessionBinding,
	at workflowrun.Sighting) error {

	label, ok := g.says[at.Screen]
	if !ok {
		return nil
	}
	err := s.Confirm(label, at.Screen, at.Epoch, at.Generation, "SIMULATED fixture", at.At)
	if err != nil && strings.Contains(err.Error(), "already") {
		return nil
	}
	return err
}

// theWalk is one observation of a two-step walk: which live screen, which control is findable,
// and what was read on it.
func theWalk(screen observe.VisualScreenID, epoch uint64, control string,
	at time.Time, words []observe.ReadWord) workflowrun.Sighting {

	y := 0.241
	if control == "control_2" {
		y = 0.291
	}
	return workflowrun.Sighting{
		Screen: screen, Epoch: epoch, Generation: 1,
		Window: "hwnd:1", Bounds: window, Front: true, At: at,
		Outcomes: []observe.Reacquisition{{
			Target: observe.VisualTargetID(control), Screen: screen,
			Result:   observe.ReacquireMatched,
			Where:    observe.Region{X: 0.410, Y: y, Width: 0.072, Height: 0.028},
			Controls: 1, Because: "a fixture outcome",
		}},
		Words: words,
	}
}

// beforeWords is a reading of the screen a step is standing ON, before anything is pressed.
//
// Needed, and not scenery: a step with an outcome expectation will not be acted on until the
// screen it stands on has been read, because a verdict needs both sides of the action and the
// before side comes from exactly these readings. See verify.go. Deliberately NOT the outcome's
// words — an outcome already standing before the press is not an outcome the press produced.
func beforeWords() []observe.ReadWord {
	return panelWords("brightness", "contrast", "gamma")
}

// answersWhereItWasAsked is a granter that stamps its answer with the observation it was asked
// in, which is what a well-behaved one does.
//
// The package's shared `grants` fixture carries FIXED epoch and generation, which is right for
// the tests that make a grant deliberately stale and wrong for a walk: the second action is asked
// about in a later observation than the first, and a grant stamped with the first one is refused —
// correctly, by the guard that catches a granter handing back a cached permission from an earlier
// screen.
//
// Copying the numbers proves nothing about the world and is not meant to. Whether the world stood
// still is asked at the instant of the press, by the runner's `WithConfirm`.
type answersWhereItWasAsked struct {
	asked []workflowrun.Intent
}

func (g *answersWhereItWasAsked) Authorise(ctx context.Context, in workflowrun.Intent) (
	workflowrun.Authorisation, bool) {

	g.asked = append(g.asked, in)
	return workflowrun.Authorisation{
		Step: in.Step, By: "SIMULATED fixture",
		Epoch: in.Epoch, Generation: in.Generation, Window: in.Window,
		// LONG ENOUGH FOR THE WHOLE WALK, because what is being tested here is the
		// order of two steps and not how long a permission lasts. ADR-183's window
		// has its own tests.
		Expires: looked.Add(time.Hour),
	}, true
}

// askedAbout is how many times a step was put to the granter.
func askedAbout(g *answersWhereItWasAsked, step int) int {
	n := 0
	for _, in := range g.asked {
		if in.Step == step {
			n++
		}
	}
	return n
}

// aTwoStepRun is the production coordinator over that walk, with a clock a test can advance.
func aTwoStepRun(t *testing.T, now *time.Time) (*workflowrun.Runner,
	*workflowstore.SessionBinding, *answersWhereItWasAsked, *notebook) {

	t.Helper()
	return aTwoStepRunOf(t, now, aTwoStepWalk())
}

// aTwoStepRunOf is the same over an artifact a test has altered.
func aTwoStepRunOf(t *testing.T, now *time.Time, art workflowstore.Artifact) (
	*workflowrun.Runner, *workflowstore.SessionBinding, *answersWhereItWasAsked, *notebook) {

	t.Helper()
	s := workflowstore.NewSessionBinding()
	// BOTH STEPS AGREED TO, so what is being tested is the outcome gate and not the
	// separate question of whether a step belongs in the run at all.
	for i := range art.Steps {
		if err := s.ConfirmStep(i, "SIMULATED fixture"); err != nil {
			t.Fatal(err)
		}
	}
	grant := &answersWhereItWasAsked{}
	book := &notebook{}
	run := workflowrun.New(art, nil, aWalkingGrounder{says: map[observe.VisualScreenID]string{
		"now_1": "screen_a", "now_2": "screen_b", "now_3": "screen_c",
	}}, grant).
		WithExecutor(marcoexec.New(book), false).
		WithClock(func() time.Time { return *now }).
		WithConfirm(func(context.Context, workflowrun.Sighting, observe.PointerAt) (bool, string) {
			return true, ""
		}).
		WithPatience(60)
	return run, s, grant, book
}

// drive runs looks until the run reaches an outcome a test is waiting for, or gives up.
func drive(t *testing.T, run *workflowrun.Runner, s *workflowstore.SessionBinding,
	now *time.Time, look func() workflowrun.Sighting,
	until func(workflowrun.Attempt) bool) workflowrun.Attempt {

	t.Helper()
	var last workflowrun.Attempt
	for i := 0; i < 60; i++ {
		*now = (*now).Add(100 * time.Millisecond)
		last = run.Advance(context.Background(), s, look())
		if until(last) {
			return last
		}
	}
	return last
}

// POSITIVE: the first action verifies, and only then is the second one asked about.
//
// The acceptance this file exists for. Every assertion is about the ORDER of two things the
// product has to keep apart — a destination arriving, and an outcome being decided — and the one
// that must come first is the slower one.
func TestTheSecondActionIsAskedAboutOnlyAfterTheFirstIsVerified(t *testing.T) {
	// THE PACKAGE FIXTURE CLOCK. A test starting anywhere else is handed a grant that
	// expired before it was asked for.
	now := looked
	run, s, grant, book := aTwoStepRun(t, &now)

	// ── STEP 1: THE SCREEN IS NAMED, THE CONTROL IS THERE, THE PRESS GOES OUT ──
	first := drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_1", 5, "control_1", now, beforeWords())
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomePressed })
	if first.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("the first action never went out: %s — %s", first.Outcome, first.Because)
	}
	if n := askedAbout(grant, 1); n != 0 {
		t.Fatalf("the second action was asked about %d time(s) before the first "+
			"had even been sent", n)
	}

	// ── THE SCREEN CHANGES, WHICH IS NOT THE SAME THING AS THE OUTCOME ARRIVING ──
	//
	// The destination `screen_b` is established on this look: the step has arrived.
	// Before this gate existed, that alone advanced the walk and the second action was
	// authorised on the next observation — with the first action's outcome still being
	// decided, and then decided about a screen the second press had changed.
	arrived := drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_2", 6, "control_2", now, nil)
	}, func(a workflowrun.Attempt) bool { return a.Arrival != nil && a.Arrival.Established() })
	if arrived.Arrival == nil || !arrived.Arrival.Established() {
		t.Fatalf("the destination never arrived: %s — %s",
			arrived.Outcome, arrived.Because)
	}
	if arrived.Outcome == workflowrun.OutcomeArrived {
		t.Fatalf("the step finished on the arrival alone, with its outcome still "+
			"being decided: %s", arrived.Because)
	}
	if n := askedAbout(grant, 1); n != 0 {
		t.Fatalf("the second action was asked about %d time(s) while the first "+
			"action's outcome was still being decided", n)
	}
	if book.clicks() != 1 {
		t.Fatalf("%d press(es) went out before the first outcome was decided",
			book.clicks())
	}

	// ── NOW THE OUTCOME ARRIVES, AND HOLDS ──
	panel := panelWords("bloom", "foliage", "presets", "shadows", "textures")
	verified := drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_2", 6, "control_2", now, panel)
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomeVerified })
	if verified.Outcome != workflowrun.OutcomeVerified {
		t.Fatalf("the first action's outcome was never verified: %s — %s",
			verified.Outcome, verified.Because)
	}
	if verified.Verified == nil || verified.Verified.Verdict != workflowrun.VerdictVerified {
		t.Fatalf("the attempt carries %+v", verified.Verified)
	}

	// ── AND ONLY NOW IS THE SECOND ACTION ASKED ABOUT, AND SENT ──
	second := drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_2", 6, "control_2", now, panel)
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomePressed })
	if second.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("the second action never went out after the first verified: "+
			"%s — %s", second.Outcome, second.Because)
	}
	if second.Step != 1 {
		t.Errorf("the second press was attributed to step %d", second.Step)
	}
	if n := askedAbout(grant, 1); n != 1 {
		t.Errorf("the second action was asked about %d time(s), want 1", n)
	}
	if book.clicks() != 2 {
		t.Errorf("%d press(es) went out in total, want 2", book.clicks())
	}
}

// NEGATIVE: an outcome that says something else stops the walk, and the second action is never
// asked about.
//
// The half that makes the positive case worth anything. A workflow that pressed on regardless
// would be a workflow whose verification is a log line.
func TestAContradictedFirstOutcomeNeverReachesTheSecondAction(t *testing.T) {
	now := looked
	run, s, grant, book := aTwoStepRun(t, &now)

	if a := drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_1", 5, "control_1", now, beforeWords())
	}, func(a workflowrun.Attempt) bool {
		return a.Outcome == workflowrun.OutcomePressed
	}); a.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("the first action never went out: %s — %s", a.Outcome, a.Because)
	}

	// THE DESTINATION ARRIVES AND THE PANEL SAYS SOMETHING ELSE ENTIRELY.
	elsewhere := panelWords("ambient", "master", "music", "speech", "voice")
	end := drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_2", 6, "control_2", now, elsewhere)
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomeRefused })
	if end.Outcome != workflowrun.OutcomeRefused {
		t.Fatalf("a contradicted outcome did not stop the run: %s — %s",
			end.Outcome, end.Because)
	}
	if end.Verified == nil || end.Verified.Verdict != workflowrun.VerdictContradicted {
		t.Fatalf("the attempt carries %+v", end.Verified)
	}
	if !strings.Contains(end.Because, "contradicted") {
		t.Errorf("the refusal does not say what was wrong: %s", end.Because)
	}
	// AND THE SECOND ACTION WAS NEVER PUT TO ANYBODY, which is the whole point: a
	// person is not asked to authorise an action whose predecessor did not work.
	if n := askedAbout(grant, 1); n != 0 {
		t.Errorf("the second action was asked about %d time(s) after the first was "+
			"contradicted", n)
	}
	if book.clicks() != 1 {
		t.Errorf("%d press(es) went out, want 1", book.clicks())
	}
}

// NEGATIVE: an outcome nobody could decide stops the walk too.
//
// The third answer, and the one a verifier must never confuse with the first: no evidence is not
// evidence of success. The deadline passes, the verdict is `unconfirmed`, and the run stops —
// which is also what bounds the wait this gate introduces.
func TestAnUnconfirmedFirstOutcomeNeverReachesTheSecondAction(t *testing.T) {
	now := looked
	run, s, grant, book := aTwoStepRun(t, &now)

	if a := drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_1", 5, "control_1", now, beforeWords())
	}, func(a workflowrun.Attempt) bool {
		return a.Outcome == workflowrun.OutcomePressed
	}); a.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("the first action never went out: %s — %s", a.Outcome, a.Because)
	}

	// THE DESTINATION ARRIVES AND NOTHING IS EVER READ ON IT. The run waits, and the
	// wait is bounded by the verification deadline rather than by nothing at all.
	end := drive(t, run, s, &now, func() workflowrun.Sighting {
		now = now.Add(400 * time.Millisecond)
		return theWalk("now_2", 6, "control_2", now, nil)
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomeRefused })
	if end.Outcome != workflowrun.OutcomeRefused {
		t.Fatalf("an undecided outcome did not stop the run: %s — %s",
			end.Outcome, end.Because)
	}
	if end.Verified == nil || end.Verified.Verdict != workflowrun.VerdictUnconfirmed {
		t.Fatalf("the attempt carries %+v", end.Verified)
	}
	if n := askedAbout(grant, 1); n != 0 {
		t.Errorf("the second action was asked about %d time(s) after the first "+
			"could not be confirmed", n)
	}
	if book.clicks() != 1 {
		t.Errorf("%d press(es) went out, want 1", book.clicks())
	}
}

// AND A STEP DEMONSTRATED WITH NO OUTCOME STILL ARRIVES, which is the limit of the gate.
//
// Every routine saved before outcomes were recorded is in this state, and so is step 1 of the
// first routine this product ever performed by hand. There is nothing to wait for: the verdict is
// `unasked` forever, and a walk that gated on it would stop on every such step. So the arrival
// carries it exactly as it did before — and the report still says the action could not be
// verified, which is the honest end for a file that cannot be checked.
func TestAStepWithNoOutcomeStillAdvancesOnItsArrival(t *testing.T) {
	now := looked
	// THE SAME WALK WITH THE FIRST STEP'S EXPECTATION TAKEN AWAY, which is what an
	// older demonstration looks like.
	art := aTwoStepWalk()
	art.Steps[0].Outcome = nil
	run, s, grant, book := aTwoStepRunOf(t, &now, art)

	if a := drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_1", 5, "control_1", now, beforeWords())
	}, func(a workflowrun.Attempt) bool {
		return a.Outcome == workflowrun.OutcomePressed
	}); a.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("the first action never went out: %s — %s", a.Outcome, a.Because)
	}

	// THE DESTINATION ARRIVES, and that finishes the step: there was no outcome to wait
	// for. The look carries words because the NEXT step has an expectation of its own and
	// will not be acted on until the screen it stands on has been read.
	arrived := drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_2", 6, "control_2", now, beforeWords())
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomeArrived })
	if arrived.Outcome != workflowrun.OutcomeArrived {
		t.Fatalf("a step with no outcome expectation did not advance on its "+
			"arrival: %s — %s", arrived.Outcome, arrived.Because)
	}
	// AND THE SECOND ACTION IS THEN REACHED, because there was never a question in
	// flight to wait for.
	second := drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_2", 6, "control_2", now, beforeWords())
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomePressed })
	if second.Outcome != workflowrun.OutcomePressed || second.Step != 1 {
		t.Fatalf("the second action was not reached: %s — %s (step %d)",
			second.Outcome, second.Because, second.Step)
	}
	if n := askedAbout(grant, 1); n != 1 {
		t.Errorf("the second action was asked about %d time(s), want 1", n)
	}
	if book.clicks() != 2 {
		t.Errorf("%d press(es) went out in total, want 2", book.clicks())
	}
}
