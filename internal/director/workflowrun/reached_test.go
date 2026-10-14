package workflowrun_test

// Goal reached versus goal recognised.
//
// The two disagreed on the first real navigation: Marco opened Main Menu -> Play -> Training ->
// Free Play with three real clicks, and the third destination was never established. A report
// that counted only established destinations filed that as a step that failed, which is the
// wrong finding about the one thing that worked.
//
// So the run counts both, per step, and these hold the arithmetic -- because the obvious way to
// write it is a subtraction between two numbers that can cross.

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

// AN ACTION THAT MOVED AN UNRECOGNISED SCREEN IS COUNTED AS ITS OWN THING.
func TestAPressWhoseDestinationWasNeverEstablishedIsCountedApart(t *testing.T) {
	g := newRig(t)
	ctx := context.Background()

	if got := g.run.Advance(ctx, g.s, firstStep()); got.Outcome !=
		workflowrun.OutcomePressed {
		t.Fatalf("%s - %s", got.Outcome, got.Because)
	}
	// A later observation of a different screen, with nobody able to name it: something
	// happened and nothing established what.
	g.run.WithGrounder(nil)
	g.run.Advance(ctx, g.s, on("now_3", 9, "control_2", rect(0.2, 0.3, 0.1, 0.04)))

	rep := g.run.Report()
	if rep.StepsPressed != 1 {
		t.Fatalf("%d step(s) pressed, want 1", rep.StepsPressed)
	}
	if rep.Established != 0 {
		t.Fatalf("%d destination(s) established with nobody grounding anything",
			rep.Established)
	}
	if rep.Unrecognised != 1 {
		t.Errorf("%d action(s) moved an unrecognised screen, want 1", rep.Unrecognised)
	}
	// AND THE RUN SAYS SOMETHING HAPPENED, which is the other half of the finding.
	if !rep.Changed {
		t.Error("the screen changed after the action and the report does not say so")
	}
}

// AND A PRESS THAT ARRIVED IS NOT COUNTED AS ONE THAT DID NOT.
func TestAPressThatArrivedIsNotCountedAsUnrecognised(t *testing.T) {
	g := newRig(t)
	ctx := context.Background()

	if got := g.run.Advance(ctx, g.s, firstStep()); got.Outcome !=
		workflowrun.OutcomePressed {
		t.Fatalf("%s - %s", got.Outcome, got.Because)
	}
	if got := g.run.Advance(ctx, g.s, on("now_2", 6, "control_2",
		rect(0.2, 0.3, 0.1, 0.04))); got.Outcome != workflowrun.OutcomeArrived {
		t.Fatalf("step 0's destination: %s - %s", got.Outcome, got.Because)
	}
	rep := g.run.Report()
	if rep.StepsPressed != 1 || rep.Established != 1 {
		t.Fatalf("%d step(s) pressed and %d destination(s) established",
			rep.StepsPressed, rep.Established)
	}
	if rep.Unrecognised != 0 {
		t.Errorf("%d action(s) moved an unrecognised screen after one that arrived",
			rep.Unrecognised)
	}
}

// AND A STEP ASKED ABOUT OVER MANY LOOKS IS ONE PRESS, NOT MANY.
//
// A step presses once and is then asked about over however many later observations its patience
// allows, so a run of attempts carries one press and a dozen answers about it. Counting
// attempts rather than steps would report twelve actions for one click.
func TestAStepAskedAboutOverManyLooksIsOneActionNotMany(t *testing.T) {
	g := newRig(t)
	ctx := context.Background()

	g.run.Advance(ctx, g.s, firstStep())
	for i := 0; i < 5; i++ {
		g.run.Advance(ctx, g.s, firstStep())
	}
	rep := g.run.Report()
	if rep.StepsPressed != 1 {
		t.Errorf("%d step(s) pressed over six looks, want 1", rep.StepsPressed)
	}
	if rep.Unrecognised != 1 {
		t.Errorf("%d unrecognised, want 1", rep.Unrecognised)
	}
	if rep.Sent != 1 {
		t.Errorf("%d action(s) sent, want 1", rep.Sent)
	}
}

// AND A DESTINATION ESTABLISHED WITH NO PRESS BEHIND IT IS NOT A NEGATIVE NUMBER.
//
// The reason this is counted rather than subtracted. A step whose destination is already
// standing when it is examined arrives without anything being sent, so established destinations
// can outnumber actions -- and `Sent - Established` then prints as a negative count of
// unrecognised screens, which is not a quantity of anything.
func TestMoreDestinationsThanActionsIsNotANegativeCount(t *testing.T) {
	g := newRig(t)
	ctx := context.Background()

	// STEP 0 ARRIVES WITH NOTHING SENT: the first observation already shows the screen
	// the step was meant to produce.
	if got := g.run.Advance(ctx, g.s, on("now_2", 5, "control_1",
		rect(0.4, 0.5, 0.1, 0.04))); got.Outcome != workflowrun.OutcomeArrived {
		t.Fatalf("%s - %s", got.Outcome, got.Because)
	}
	rep := g.run.Report()
	if rep.Established < 1 {
		t.Fatalf("the fixture did not establish a destination without a press: %+v",
			rep.Established)
	}
	if rep.Sent != 0 {
		t.Fatalf("%d action(s) sent for an arrival nobody pressed for", rep.Sent)
	}
	// THE CONDITION THE SUBTRACTION COULD NOT SURVIVE.
	if rep.Established <= rep.Sent {
		t.Fatalf("the fixture is not the case under test: %d established, %d sent",
			rep.Established, rep.Sent)
	}
	if rep.Unrecognised != 0 {
		t.Errorf("%d action(s) moved an unrecognised screen, and none was sent",
			rep.Unrecognised)
	}
	if rep.StepsPressed != 0 {
		t.Errorf("%d step(s) pressed, and nothing was sent", rep.StepsPressed)
	}
}
