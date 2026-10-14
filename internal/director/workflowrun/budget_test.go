package workflowrun_test

// How many actions a run may send, enforced where the actions are sent.
//
// A limit a caller imposes is a limit on what the caller does with a report. By the time
// `OutcomePressed` has been returned, a program has been lowered, compiled, run, and has reached
// whatever is at the bottom — so a caller that stopped the run on seeing one press would be
// acting on a report of something that already happened. The only place "no more than one" can
// be true is inside the thing that sends them.

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

// A BUDGET OF ONE SENDS THE FIRST ACTION AND REFUSES THE SECOND.
//
// The whole walk, so the refusal lands on a step that is plannable in every other way: the
// screen is established, the control was found again, somebody agreed to it, and somebody would
// have authorised it. Only the budget is in the way.
func TestABudgetOfOneSendsOneActionAndRefusesTheNext(t *testing.T) {
	g := newRig(t)
	g.run.WithBudget(1)
	ctx := context.Background()

	if got := g.run.Advance(ctx, g.s, firstStep()); got.Outcome !=
		workflowrun.OutcomePressed {
		t.Fatalf("step 0: %s — %s", got.Outcome, got.Because)
	}
	if g.run.Sent() != 1 {
		t.Fatalf("the run says it sent %d action(s), want 1", g.run.Sent())
	}

	// THE FIRST STEP ARRIVES, so the walk is on step 1 and step 1 is plannable.
	g.auth.epoch = 6
	arrived := g.run.Advance(ctx, g.s, on("now_2", 6, "control_2", rect(0.2, 0.3, 0.1, 0.04)))
	if arrived.Outcome != workflowrun.OutcomeArrived {
		t.Fatalf("after the press: %s — %s", arrived.Outcome, arrived.Because)
	}

	asked := len(g.auth.asked)
	got := g.run.Advance(ctx, g.s, on("now_2", 6, "control_2", rect(0.2, 0.3, 0.1, 0.04)))
	if got.Outcome == workflowrun.OutcomePressed {
		t.Fatal("a second action was sent by a run allowed to send one")
	}
	if g.book.clicks() != 1 {
		t.Fatalf("%d press(es) reached Marco, want 1", g.book.clicks())
	}
	if g.run.Sent() != 1 {
		t.Errorf("the run says it sent %d action(s), want 1", g.run.Sent())
	}
	// AND NOBODY WAS ASKED FOR PERMISSION THAT COULD NOT BE USED.
	if len(g.auth.asked) != asked {
		t.Errorf("somebody was asked to authorise an action the budget forbids")
	}
	if !contains(got.Because, "may send 1 action(s) and has sent 1") {
		t.Errorf("the refusal does not say it was the budget: %s", got.Because)
	}
}

// AND THE DEFAULT IS UNBOUNDED, or every offline caller would silently send one action.
func TestARunWithNoBudgetSendsAsManyActionsAsItReaches(t *testing.T) {
	g := newRig(t)
	ctx := context.Background()
	if g.run.Budget() >= 0 {
		t.Fatalf("the default budget is %d, want unbounded", g.run.Budget())
	}
	g.run.Advance(ctx, g.s, firstStep())
	g.auth.epoch = 6
	g.run.Advance(ctx, g.s, on("now_2", 6, "control_2", rect(0.2, 0.3, 0.1, 0.04)))
	g.run.Advance(ctx, g.s, on("now_2", 6, "control_2", rect(0.2, 0.3, 0.1, 0.04)))
	if g.book.clicks() != 2 {
		t.Fatalf("%d press(es) reached Marco, want 2", g.book.clicks())
	}
}

// AND THE REPORT SAYS WHAT THE RUN WAS ALLOWED TO SEND.
//
// "One action" is only a meaningful claim beside the limit it was made under: a run that sent
// one because it was allowed one and a run that sent one because it stopped are different
// outcomes and the numbers have to tell them apart.
func TestTheReportCarriesTheBudgetAndWhatWasSent(t *testing.T) {
	g := newRig(t)
	g.run.WithBudget(1)
	g.run.Advance(context.Background(), g.s, firstStep())
	rep := g.run.Report()
	if rep.Budget != 1 || rep.Sent != 1 {
		t.Errorf("the report says budget %d, sent %d; want 1 and 1",
			rep.Budget, rep.Sent)
	}
}

// ── a run describing itself ───────────────────────────────────────────────────

// A RUN THAT ACTED AND DID NOT ARRIVE DID NOT STOP BEFORE THE FIRST STEP.
//
// Found in the first supervised run. One action was lowered, compiled, run and delivered, its
// destination never appeared, and the summary line said "stopped before the first step" — which
// is true of the cursor and reads as a run where nothing happened, directly above a line counting
// what it did.
func TestARunThatActedAndDidNotArriveDoesNotSayItStoppedBeforeTheFirstStep(t *testing.T) {
	g := newRig(t)
	ctx := context.Background()
	if got := g.run.Advance(ctx, g.s, firstStep()); got.Outcome !=
		workflowrun.OutcomePressed {
		t.Fatalf("step 0: %s — %s", got.Outcome, got.Because)
	}
	// The destination never arrives, however long it is given.
	for i := 0; i <= g.run.Report().Progress.Steps*50; i++ {
		a := g.run.Advance(ctx, g.s, firstStep())
		if a.Outcome == workflowrun.OutcomeRefused {
			break
		}
	}
	summary := g.run.Report().Progress.Summary()
	if strings.Contains(summary, "before the first step") {
		t.Errorf("a run that performed an action says it stopped before the first "+
			"step:%s%s", "\n  ", summary)
	}
	if !strings.Contains(summary, "acted on") {
		t.Errorf("the summary does not say the step was acted on:%s%s", "\n  ", summary)
	}
}

// AND A RUN THAT PERFORMED SOMETHING DOES NOT SAY NOTHING WAS EXECUTED.
//
// The claim that matters — that arriving is not proof of causing — is the same in both cases and
// is stated once. The sentence around it is a fact about the run, and has to be this run's.
func TestARunThatPerformedSomethingDoesNotSayNothingWasExecuted(t *testing.T) {
	g := newRig(t)
	g.run.Advance(context.Background(), g.s, firstStep())
	rep := g.run.Report()
	if rep.Sent == 0 {
		t.Fatal("the fixture sent nothing, so there is nothing to describe")
	}
	ok, why := rep.Verified()
	if ok {
		t.Fatal("a run reported an action as independently verified")
	}
	if strings.Contains(why, "nothing was executed") {
		t.Errorf("a run that performed %d action(s) says nothing was executed: %s",
			rep.Sent, why)
	}
	if !strings.Contains(why, "not by itself proof") {
		t.Errorf("the verdict drops the claim it exists to make: %s", why)
	}
}

// AND A RUN THAT PERFORMED NOTHING STILL SAYS SO.
func TestARunThatPerformedNothingSaysNothingWasExecuted(t *testing.T) {
	g := newRig(t)
	_, why := g.run.Report().Verified()
	if !strings.Contains(why, "nothing was executed") {
		t.Errorf("a run that sent nothing does not say so: %s", why)
	}
}

// ── the window an authorisation was given about ───────────────────────────────

// A GRANT GIVEN ABOUT ANOTHER WINDOW BUYS NOTHING HERE.
//
// Separate from the observation check, because the two fail differently and a person needs to be
// told which happened: a generation moves when the SAME window is replaced, and this changes
// when the press would go to a DIFFERENT window — a second copy of the application, a relaunch,
// or a selector that matched something else.
func TestAnAuthorisationGivenAboutAnotherWindowIsRefused(t *testing.T) {
	g := newRig(t)
	g.auth.bend = func(a workflowrun.Authorisation) workflowrun.Authorisation {
		a.Window = "hwnd:99"
		return a
	}
	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome == workflowrun.OutcomePressed {
		t.Fatal("a grant about another window was spent here")
	}
	if len(g.book.programs) != 0 {
		t.Fatal("a program reached Marco on a grant about another window")
	}
	if !contains(got.Because, "permission to click in one window is not permission") {
		t.Errorf("the refusal does not name the window: %s", got.Because)
	}
}

// AND A GRANT NAMING NO WINDOW AT ALL IS REFUSED, because a grant that named no window would be
// spendable in every window.
func TestAnAuthorisationNamingNoWindowIsRefused(t *testing.T) {
	g := newRig(t)
	g.auth.bend = func(a workflowrun.Authorisation) workflowrun.Authorisation {
		a.Window = ""
		return a
	}
	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome == workflowrun.OutcomePressed {
		t.Fatal("a grant naming no window was spent")
	}
}

// AND THE QUESTION SAYS WHICH WINDOW IT IS ABOUT, or a granter could not stamp it.
func TestTheQuestionNamesTheWindowThePressWouldGoTo(t *testing.T) {
	g := newRig(t)
	g.run.Advance(context.Background(), g.s, firstStep())
	if len(g.auth.asked) == 0 {
		t.Fatal("nobody was asked")
	}
	if got := g.auth.asked[0].Window; got != "hwnd:1" {
		t.Errorf("the question named window %q, want hwnd:1", got)
	}
}

// ── a region that locates nothing ─────────────────────────────────────────────

// A CONTROL RE-ACQUIRED AS A BAND ACROSS THE WINDOW IS NOT PRESSED.
//
// Measured, not imagined: a saved Rocket League workflow carried four controls whose readings
// ran the full width of the window, three of them covering it almost entirely. Every one of them
// re-acquired perfectly and every one of them offered the same point — the middle of the screen.
func TestAControlReacquiredAsABandAcrossTheWindowIsNotPressed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		where observe.Region
	}{
		{"the whole window", rect(0, 0, 1, 0.957)},
		{"a band across the width", rect(0, 0.468, 1, 0.461)},
		{"a column down the height", rect(0.3, 0, 0.2, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t)
			look := firstStep()
			look.Outcomes[0].Where = tc.where
			got := g.run.Advance(context.Background(), g.s, look)
			if got.Outcome == workflowrun.OutcomePressed {
				t.Fatal("a press was sent at the middle of the window")
			}
			if len(g.book.programs) != 0 {
				t.Fatal("something reached Marco for a control that " +
					"located nothing")
			}
			// AND NOBODY WAS ASKED TO PERMIT IT. A question showing the
			// middle of the screen as the place a control is would be asking
			// somebody to agree to something untrue.
			if len(g.auth.asked) != 0 {
				t.Errorf("somebody was asked to permit a press at %v",
					g.auth.asked[0].At)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
