package workflowrun_test

// THE SINGLE-INTERACTION CONTRACT, AND THE FOUR WAYS IT COULD HAVE BEEN A HOLE.
//
// `Begin` exists so a caller whose order lives somewhere else — a saved `.marco` play, whose
// sentences are the order — can say WHICH interaction it is performing. Without it a reordered
// play would silently perform the artifact's original order.
//
// The danger in giving a cursor an external setter is that the cursor is not the only state an
// interaction leaves behind, and every guarantee this package enforces is enforced at a moment.
// So each test here is one of those guarantees, asked from the new door:
//
//	an unknown interaction           nothing to perform
//	a pending verification           ADR-242's gate, which must not be re-aimed past
//	a press in flight                a press would be abandoned mid-air
//	a stopped run                    cancellation is a floor
//	the accounting                   shared across interactions, never reset
//
// The last is not a refusal and is the one a reviewer should look at hardest: a `Begin` that
// reset the press count would hand every interaction a fresh budget, which turns a bounded
// supervised run into an unbounded one by arithmetic rather than by decision.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

func TestBeginRefusesAnInteractionTheRoutineDoesNotHave(t *testing.T) {
	now := looked
	run, _, _, _ := aTwoStepRun(t, &now)
	for _, step := range []int{-1, 2, 99} {
		err := run.Begin(step)
		if err == nil {
			t.Fatalf("interaction %d was accepted and this routine has 2", step)
		}
		if !strings.Contains(err.Error(), "interaction") {
			t.Errorf("the refusal does not say what was wrong: %v", err)
		}
	}
	// AND THE RUN IS UNMOVED. A refused Begin that had already written the cursor would
	// leave the runner standing somewhere nobody asked for.
	if run.Cursor() != 0 {
		t.Fatalf("a refused Begin moved the run to interaction %d", run.Cursor())
	}
}

// A PENDING VERIFICATION CANNOT BE RE-AIMED PAST.
//
// [[ADR-242-the-second-action-waits-for-the-first-ones-answer]] is the rule that the second
// action is not authorised until the first one's verdict is in. `Advance` enforces it on the walk
// it owns. A caller that could call `Begin(1)` while interaction 0's outcome was still being
// decided would have exactly the defect ADR-242 closed — the second press reaching the machine
// while the first verdict was still forming — reopened through a different door.
func TestBeginRefusesWhileTheLastInteractionIsStillBeingVerified(t *testing.T) {
	now := looked
	run, s, _, book := aTwoStepRun(t, &now)

	// The press goes out, and then the destination arrives while the outcome stays
	// undecided — which is the exact window ADR-242 is about.
	drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_1", 5, "control_1", now, beforeWords())
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomePressed })
	drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_2", 6, "control_2", now, nil)
	}, func(a workflowrun.Attempt) bool { return a.Arrival != nil && a.Arrival.Established() })

	if err := run.Begin(1); err == nil {
		t.Fatal("the second interaction was begun with the first one's outcome still " +
			"being decided")
	} else if !strings.Contains(err.Error(), "verified") {
		t.Errorf("the refusal does not say what it is waiting for: %v", err)
	}
	if book.clicks() != 1 {
		t.Fatalf("%d press(es) went out", book.clicks())
	}
}

// A PRESS IN FLIGHT CANNOT BE ABANDONED.
//
// A press has gone out and its destination is not established yet. Beginning something else
// there would drop the only record that an action is outstanding, and the run would report
// nothing at all about a press that reached somebody's computer.
func TestBeginRefusesWhileAPressIsStillInFlight(t *testing.T) {
	now := looked
	// A ROUTINE WITH NO OUTCOME EXPECTATION, so the refusal under test is the in-flight
	// one and not the verification one: with no outcome recorded there is nothing to
	// verify, and `awaitingOutcome` is false by design.
	art := aTwoStepWalk()
	for i := range art.Steps {
		art.Steps[i].Outcome = nil
	}
	run, s, _, book := aTwoStepRunOf(t, &now, art)

	drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_1", 5, "control_1", now, nil)
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomePressed })
	if book.clicks() != 1 {
		t.Fatalf("the press never went out: %d", book.clicks())
	}

	if err := run.Begin(1); err == nil {
		t.Fatal("an interaction was begun with a press still in flight")
	} else if !strings.Contains(err.Error(), "destination is not established") {
		t.Errorf("the refusal does not say what is outstanding: %v", err)
	}
}

// A STOPPED RUN BEGINS NOTHING.
func TestBeginRefusesAfterTheRunIsStopped(t *testing.T) {
	now := looked
	run, _, _, _ := aTwoStepRun(t, &now)
	run.Stop()
	err := run.Begin(0)
	if err == nil {
		t.Fatal("an interaction was begun after the run was stopped")
	}
	if !strings.Contains(err.Error(), "stopped") {
		t.Errorf("the refusal does not say the run was stopped: %v", err)
	}
}

// THE ACCOUNTING IS THE RUN'S, NOT THE INTERACTION'S.
//
// What `Begin` resets describes the interaction being started. What it keeps describes the run,
// and the run is still the same run. A budget reset per interaction would be a supervised run
// that could send any number of presses as long as each one arrived under a different name.
func TestBeginKeepsTheRunsAccountingAndResetsOnlyTheInteraction(t *testing.T) {
	now := looked
	art := aTwoStepWalk()
	for i := range art.Steps {
		art.Steps[i].Outcome = nil
	}
	run, s, _, book := aTwoStepRunOf(t, &now, art)

	// One interaction, performed and arrived, so the run has a press to its name.
	drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_1", 5, "control_1", now, nil)
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomePressed })
	drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_2", 6, "control_2", now, nil)
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomeArrived })

	sent := run.Sent()
	if sent != 1 {
		t.Fatalf("the run accounts for %d press(es) before the second interaction", sent)
	}
	if err := run.Begin(0); err != nil {
		t.Fatalf("beginning an interaction again was refused: %v", err)
	}
	if run.Sent() != sent {
		t.Fatalf("Begin reset the press count from %d to %d", sent, run.Sent())
	}
	if run.Cursor() != 0 {
		t.Fatalf("Begin left the run on interaction %d", run.Cursor())
	}
	if book.clicks() != 1 {
		t.Fatalf("Begin itself sent something: %d press(es)", book.clicks())
	}
}

// AND A BUDGET IS STILL A BUDGET ACROSS INTERACTIONS.
//
// One press allowed for the whole run, one already spent. The second interaction has to be
// refused for being over budget — not handed a fresh allowance because it arrived under a
// different name. This is the arithmetic version of a safety decision, which is the kind that
// nobody notices.
func TestABudgetSurvivesBeginningAnotherInteraction(t *testing.T) {
	now := looked
	art := aTwoStepWalk()
	for i := range art.Steps {
		art.Steps[i].Outcome = nil
	}
	run, s, _, book := aTwoStepRunOf(t, &now, art)
	run.WithBudget(1)

	drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_1", 5, "control_1", now, nil)
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomePressed })
	drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_2", 6, "control_2", now, nil)
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomeArrived })
	if book.clicks() != 1 || run.Sent() != 1 {
		t.Fatalf("the first interaction sent %d press(es) of %d accounted",
			book.clicks(), run.Sent())
	}

	if err := run.Begin(1); err != nil {
		t.Fatalf("the second interaction could not be begun: %v", err)
	}
	last := drive(t, run, s, &now, func() workflowrun.Sighting {
		return theWalk("now_2", 7, "control_2", now, nil)
	}, func(a workflowrun.Attempt) bool { return a.Outcome == workflowrun.OutcomeRefused })
	if last.Outcome != workflowrun.OutcomeRefused {
		t.Fatalf("the second interaction was %s, not refused: %s",
			last.Outcome, last.Because)
	}
	if book.clicks() != 1 {
		t.Fatalf("%d press(es) went out against a budget of 1", book.clicks())
	}
	// AND THE SENTENCE SAYS BOTH HALVES: the allowance, and that nobody was asked. The
	// second is the one a press count cannot see — a run that asked and was refused is
	// not the same product as one that never put the question.
	if !strings.Contains(last.Because, "has sent 1") {
		t.Errorf("the refusal does not say the allowance was spent: %s", last.Because)
	}
	if !strings.Contains(last.Because, "nobody was asked") {
		t.Errorf("the refusal does not say the person was never asked: %s", last.Because)
	}
}
