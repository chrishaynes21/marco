package main

// THE ENGINE SIDE OF THE DEMONSTRATED BRIDGE: the act's host, and running the saved bytes.
//
// # Why this is in cmd/director and not beside the policy
//
// Because `internal/director` may not import the engine. `boundary_test.go` walks every import
// under that tree and refuses `internal/runtime` and `internal/driver` with the reason:
//
//	The Director must not depend on platform or engine code: Marco programs are run through
//	directorapi.MarcoRunner, wired in cmd/director.
//
// The first draft of the bridge put the `runtime.Host` and the `driver` call in
// `internal/director/demonstrated`, and the guard caught it on five files and two counts — the
// second being `TestDirectorHasNoDuplicatePlatformImplementation`, which flags a `runtime.HostCall`
// in that tree because a host call skips the parser and compiler.
//
// It is right. So the split is: the POLICY is in `internal/director/demonstrated` — bindings,
// preflight, the supervised driver, the account, the outcome — and everything that knows the
// engine exists is here, which is where `performworkflow.go` already builds act host maps.
//
// # What a play may reach from here
//
//	Demonstrated   demonstratedHost, over a preflighted plan
//	*              everything else — REFUSED, by name, with a sentence
//
// The catch-all matters. `runtime` falls back to `DryRunHost` for an act nobody registered, and
// that one LOGS THE CALL AND ANSWERS OK. So a play that had grown `use os.` and
// `do OS's Click with at` would reach no mouse — and would return ok, and the `when ok?` after it
// would carry on as though a verified interaction had happened. No input, and a lie.
//
// `Preflight`'s digest gate already refuses a play that has been edited, so this is the second of
// two independent reasons that is closed. Both exist because they fail differently: the digest
// catches the file being different, and this catches a sentence being answered by something that
// was not asked to answer it.

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// marcoChecks answers `demonstrated.Plays` over the production checker.
//
// One line, deliberately: the question a saved play is checked with must be the question the
// runtime asks, and `driver.CheckSource` builds the graph with the same resolver and the same
// built-in modules `RunSourceWithHostsCtx` does. A second implementation of "is this legal Marco"
// is the two-lists-one-fact defect waiting to happen — a spec test was once green while
// production refused the same play, because the two were asking different things about one source.
type marcoChecks struct{}

func (marcoChecks) Check(src string) error { return driver.CheckSource(src) }

// demonstratedHost answers the Demonstrated act for one preflighted play.
type demonstratedHost struct {
	plan demonstrated.Plan
	sup  demonstrated.Supervisor
	acc  *demonstrated.Account
	// scenes is where this host says WHICH of the Act's interactions it is performing,
	// for the run's structured account of itself. Nil does nothing.
	//
	// Here because this is the only place that knows: the name came out of the play's own
	// sentence, and everything downstream is handed a step — which does not identify a
	// Scene, and for an Act composed by picking is not the number a person is watching.
	// See interactionidentity.go.
	scenes *performedInteractions
}

// Invoke is the foreign action. One capability, and `ok` means verified.
func (h *demonstratedHost) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	if c.Action != "Press" {
		return "", runtime.Value{}, fmt.Errorf("the Demonstrated act has no %q", c.Action)
	}
	name := c.Input.AsText()
	if name == "" {
		return "", runtime.Value{}, fmt.Errorf("a press needs the name of the " +
			"interaction to perform")
	}
	step, bound := h.plan.Steps[name]
	if !bound {
		// UNREACHABLE WHEN THE PAIR MATCHES, because the play is generated from the
		// bindings and preflight compares the digests. Kept because the wrong answer
		// here is not an error but a SEARCH: a bridge that fell back to finding a
		// control of that name would be the Theater's authority arriving through a
		// door built to exclude it.
		why := fmt.Sprintf("nothing is bound to the interaction called %q, so there is "+
			"no demonstration to perform", name)
		h.acc.Note(demonstrated.Ask{Name: name, Step: -1, Refused: why})
		return "", runtime.Value{}, fmt.Errorf("%s", why)
	}
	// AIMED BEFORE THE PRESS AND RELEASED AFTER IT, so every question put and every
	// verdict reported while this sentence is running carries the Scene's own identity —
	// and nothing asked between two sentences claims to be about either.
	h.scenes.aim(h.position(name), name, step)
	defer h.scenes.over()
	v, because, err := h.sup.Press(c.Ctx, step)
	verdict := demonstrated.VerdictOf(v)
	if err != nil {
		// THE VERDICT TOO, WHEN THERE IS ONE. A contradicted interaction comes back as
		// an error AND a verdict, and a record that kept only the error would report
		// "Marco would not press anything" about a press that went out and was
		// disagreed with.
		h.acc.Note(demonstrated.Ask{Name: name, Step: step, Verdict: verdict,
			Decided: v != nil, Because: because, Refused: err.Error(),
			Cancelled: isCancelled(err)})
		return "", runtime.Value{}, err
	}
	h.acc.Note(demonstrated.Ask{Name: name, Step: step, Verdict: verdict,
		Decided: v != nil, Because: because})
	if verdict != workflowrun.VerdictVerified {
		// OK IS VERIFIED AND NOTHING ELSE. `contradicted` is evidence against the
		// action, `unconfirmed` is the deadline passing with nothing to go on, and
		// `unasked` is an interaction whose demonstration recorded no outcome. None of
		// the three is a performed-and-working action, and returning ok for any of
		// them would put the next press behind a `when ok?` that means nothing.
		return "", runtime.Value{}, fmt.Errorf("%q: %s", name,
			demonstrated.Sentence(verdict, because))
	}
	return "ok", runtime.Absent(), nil
}

// position is where in the ACT an interaction of this name is, counting from 1, or 0.
//
// The play's sentence order, which `Plan.Order` carries because it is not recoverable from the
// bindings map: a subset or a reordered Act numbers its own interactions and the demonstration
// numbers its steps, and those are different numbers.
func (h *demonstratedHost) position(name string) int {
	for i, n := range h.plan.Order {
		if n == name {
			return i + 1
		}
	}
	return 0
}

// performDemonstratedPlay runs a saved play's PERSISTED BYTES and returns the account.
//
// The path rather than a string, deliberately: "the saved source controls execution" is a claim
// about a file, and a caller that passed source it had just generated would be proving something
// weaker. `driver.RunFileWithHostsCtx` reads it, resolves `use demonstrated.` through the
// embedded act surface, compiles, and dispatches each sentence.
//
// The returned error is the RUNTIME's — a file that could not be read or compiled. It is not how
// the routine went: see `demonstrated.Account`, and `Cut` for the case this function has to
// report itself, which is the run being ended from outside before every sentence was reached.
func performDemonstratedPlay(ctx context.Context, reference string,
	plan demonstrated.Plan, sup demonstrated.Supervisor,
	scenes *performedInteractions, out io.Writer) (*demonstrated.Account, error) {

	if plan.Play == "" {
		return demonstrated.NewAccount(reference, 0),
			fmt.Errorf("this plan carries no play; only bytes that have been " +
				"preflighted are performed")
	}
	acc := demonstrated.NewAccount(reference, len(plan.Steps))
	h := &demonstratedHost{plan: plan, sup: sup, acc: acc, scenes: scenes}
	// AND THE STOP REACHES THE COORDINATOR, not only the runtime.
	//
	// `HostCall.Ctx` is the calling FRAME's context, and there is a window between the run
	// being cancelled and a frame noticing. One run in six used that window to ground and
	// authorise the next interaction — a prompt nobody should have seen. See
	// `demonstrated.Supervisor.Halt`.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			sup.Halt()
		case <-done:
		}
	}()
	// THE VALIDATED BYTES, AND NO PATH.
	//
	// `RunFileWithHostsCtx` would reread the file, and the first version of this used it.
	// An edit or a replacement between preflight and execution therefore reached the
	// runtime unchecked: it would be compiled, which establishes that it is legal Marco —
	// not that it is the play whose bindings were resolved, and not that it still has the
	// nested continuation that is the only gate between one action and the next. A
	// replacement could be legal Marco with two consecutive sentences and no gate at all.
	//
	// `dir` is EMPTY, which is a second thing this buys. With a directory, `use
	// demonstrated.` resolves a sibling `demonstrated.marco` ahead of the built-in act
	// surface — so a file dropped next to a play could redefine the act it is performed
	// through. Empty means built-ins only, and there is no file beside a play that can
	// change what its sentences mean.
	err := driver.RunSourceWithHostsCtx(ctx, plan.Play, "", plan.PlayName(), out,
		map[string]runtime.Host{
			"Demonstrated": h,
			"*":            refuseEveryOtherAct{},
		})
	// THE RUN ENDING FROM OUTSIDE IS THIS FUNCTION'S TO REPORT.
	//
	// Cancelling the context aborts the frame tree, so a run cancelled as its first
	// interaction finished never reaches the second sentence and the host is never called
	// about it. Without this the account would hold nothing but verified interactions and
	// would say so — `verified`, about half a routine.
	if cerr := ctx.Err(); cerr != nil {
		// EXPIRY IS NOT A CANCELLATION, and the flag is what the account reports.
		// `Cut(…, true)` makes the outcome `cancelled`, which would say somebody
		// chose to stop a run that simply used the time it was given.
		if errors.Is(cerr, context.DeadlineExceeded) {
			acc.Cut("the run reached its time limit before every interaction "+
				"was reached", false)
		} else {
			acc.Cut("the run was cancelled before every interaction was reached",
				true)
		}
	} else if n := len(acc.Asked()); n < len(plan.Steps) && err != nil {
		acc.Cut(fmt.Sprintf("the play stopped after %d of %d interaction(s): %v",
			n, len(plan.Steps), err), false)
	}
	return acc, err
}

// refuseEveryOtherAct answers any act this bridge did not register.
type refuseEveryOtherAct struct{}

func (refuseEveryOtherAct) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	return "", runtime.Value{}, fmt.Errorf("a demonstrated play may only ask for "+
		"Demonstrated's Press; this one asked for %s's %s", c.Act, c.Action)
}

// isCancelled is whether a refusal was a stop rather than a failure.
func isCancelled(err error) bool { return errors.Is(err, demonstrated.ErrCancelled) }
