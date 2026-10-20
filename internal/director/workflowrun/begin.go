package workflowrun

// ONE INTERACTION, NAMED BY WHOEVER IS DRIVING, AND NOTHING ELSE.
//
// # The condition
//
// `Advance` walks a cursor that only ever increments, and `advance` is private. That is right for
// the caller it was built for: `director perform-workflow` performs a saved routine from the
// front, in the order the file records, and the cursor IS the order.
//
// It is not enough for a caller whose order lives somewhere else. A saved `.marco` play's
// sentences are the order — see
// [[ADR-243-a-door-of-its-own-and-the-order-in-the-file]] — and a host answering one
// sentence has to be able to say WHICH interaction it is answering. With only `Advance`, a
// reordered play would silently perform the file's original order while reading as though it
// performed the play's, which is the worst available outcome: no error, wrong presses.
//
// # Why this is not a setter
//
// Because `cursor` is not the only state an interaction leaves behind, and the first sketch of
// this was one guarded assignment. The runner also holds the step it ACTED on and whose
// destination is not yet established (`acted`), how many observations the current step has
// consumed (`looks`), the verification being gathered (`verify`), the stop flag, and the shared
// accounting (`sent`, `budget`). Resetting too much loses a press that has already gone out;
// resetting too little leaves the next interaction standing in the last one's evidence.
//
// So this is an explicit contract with four refusals and a stated division of state:
//
//	RESET, because they describe the interaction being started
//	  cursor   the step to attempt
//	  acted    nothing has been sent for this one yet
//	  looks    no observations consumed yet
//
//	KEPT, because they describe THE RUN and the run is still the same run
//	  sent, budget      the press accounting, shared by every interaction in the play
//	  attempts          what happened, in order
//	  verifications     each action's verdict, for the report
//	  reads             the recent readings, which are the BEFORE side of the next verdict
//	  repairs, regrounded, stopped
//
// `reads` being kept is load-bearing rather than tidy: a step with an outcome expectation will
// not be acted on until the screen it stands on has been read, and those readings arrived before
// this interaction was asked for. Clearing them would make every interaction after the first wait
// for evidence it already had.
//
// # The four refusals
//
// Each one is a way the caller could otherwise step around a guarantee that already exists:
//
//  1. AN UNKNOWN STEP. Out of range, or no such position.
//  2. A PENDING VERIFICATION. The step the walk is on has an outcome still being decided, which
//     is exactly what [[ADR-242-the-second-action-waits-for-the-first-ones-answer]] built. A
//     caller that could re-aim past it would have the second action's press reaching the machine
//     while the first action's verdict was still being formed — the defect ADR-242 closed,
//     reopened through a different door.
//  3. AN ACTION IN FLIGHT. A press has gone out for a step whose destination is not yet
//     established. Beginning another interaction there would abandon a press mid-air and report
//     nothing about it.
//  4. A STOPPED RUN. Cancellation is a floor, not a suggestion: nothing begins after a stop, and
//     the caller is told why rather than receiving a quiet refusal to act.
//
// Deleting any of the four must fail `begin_test.go`. Deleting the whole thing must fail the
// bridge's acceptance, which cannot name an interaction without it.

import "fmt"

// Begin starts one named interaction, so the caller's order is the order performed.
//
// After it returns nil, `Advance` works exactly as it always has — the runner is simply standing
// on the step the caller named rather than on the next one. It performs nothing itself and sends
// nothing: being able to press something is still a decision made by `WithExecutor`, and this
// changes none of the grounding, authorisation or verification that stands between a cursor and
// a press.
func (r *Runner) Begin(step int) error {
	if r == nil {
		return fmt.Errorf("there is no run to begin an interaction in")
	}
	if step < 0 || step >= len(r.art.Steps) {
		return fmt.Errorf("this routine has %d interaction(s); there is no interaction %d",
			len(r.art.Steps), step)
	}
	if r.Stopped() {
		return fmt.Errorf("this run was stopped; interaction %d will not be started", step)
	}
	if r.awaitingOutcome() {
		return fmt.Errorf("interaction %d is still being verified; nothing else may start "+
			"until it has an answer", r.cursor)
	}
	if r.acted != -1 {
		return fmt.Errorf("a press for interaction %d has gone out and its destination is "+
			"not established yet; nothing else may start until it is", r.acted)
	}
	// THE LIVE ASSIGNMENT IS THE CURSOR. Mutating it to a no-op fails the bridge's
	// reordering acceptance, which is what this whole function exists for.
	r.cursor = step
	// AND THESE TWO SURVIVED MUTATION, which is recorded rather than quietly kept.
	//
	// Removing either leaves every test green, and so does removing both: `acted` is
	// always already -1 here because the guard above refuses otherwise, and `looks` is
	// reset by `advance` on every path that finishes an interaction. They are redundant
	// with the guards and with each other.
	//
	// Kept anyway, and the same way `verify.go` keeps its `v.step != r.cursor` clause:
	// what they hold is that THIS function leaves a coherent interaction state, so a
	// later change to a guard or to `advance` cannot make them necessary without
	// anybody noticing. A reader deserves to know which of the three lines is load
	// bearing today, and it is the one above.
	r.acted = -1
	r.looks = 0
	return nil
}
