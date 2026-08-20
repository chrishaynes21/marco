package main

import (
	"context"
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// The Director's own work, entering the service's command registry.
//
// # Why this file exists at all
//
// The service already owns a registry: one mutating command at a time, visible to `director
// status`, refusable when something else is running, and — the reason any of it matters — the
// thing CANCEL_ACTIVE cancels. An executed phrase enters it in server.go. A performance enters it
// in service/perform.go. Both of those are requests the SERVER routes, so the server can Begin
// around them.
//
// A rehearsal is not routed that way. It has two entrances, and only one of them is a request:
//
//	`director rehearse --live`     — the observation door, which the server passes straight through
//	the Learn coordinator saying yes to "want me to try it once?" — no request at all
//
// The second is the one that mattered. It reaches `Runtime.Rehearse` from inside a Learn episode,
// several layers below any handler, and it types on the real desktop. Putting Begin/Finish in the
// server would have covered the first entrance and left the second exactly as it was: unstoppable,
// and reported by `director stop` as "nothing is running" while it typed.
//
// So the Runtime is handed the registry, and the ONE function both entrances funnel through claims
// the slot. There is no second registry and no second lifecycle — this is the service's, borrowed.

// UseCommands installs the service's command registry and the service's own lifetime.
//
// Called once, by the server, at construction. A Runtime that never receives one still works — the
// tests in this package build Runtimes directly and drive dry walks through them — it simply has
// nowhere to publish what it is doing. That is a legitimate degraded state and never a silent one:
// see beginPerformance.
func (r *Runtime) UseCommands(serviceCtx context.Context, reg *service.Registry) {
	r.commands, r.serviceCtx = reg, serviceCtx
}

// serviceContext is the service's own lifetime, and the ONLY place in this package that may fall
// back to a fresh context.
//
// # Why the fallback is here and nowhere else
//
// `Runtime.Observation` is a request handler with no context of its own — the service.Runtime
// interface has never given it one — so the rehearsal it dispatches needs a parent from somewhere.
// Every other candidate is worse: a `context.Background()` written at the call site is precisely
// the defect TestNothingThatCanReachTheWalkerInventsItsOwnContext exists to forbid, and it is
// forbidden because it is invisible at the point it does harm.
//
// Here it is visible, named, and harmless: the value it falls back to belongs to a Runtime that no
// server ever adopted, which is a test's Runtime. And even then the walk is still stoppable, because
// `Registry.Begin` derives its command context with `WithCancel` — `Registry.Cancel` cuts it
// whatever the parent is. The parent decides only whether SERVICE SHUTDOWN also ends the walk.
func (r *Runtime) serviceContext() context.Context {
	if r.serviceCtx != nil {
		return r.serviceCtx
	}
	return context.Background()
}

// beginPerformance claims the mutating slot for something that is about to drive the desktop, and
// returns the context that "stop" cuts.
//
// The returned context descends from the CALLER's, not merely from the service's. That is the
// difference between a rehearsal that a Learn episode can abandon and one that outlives it: when
// the episode is cancelled its context ends, and the walk it authorised ends with it at the next
// step boundary. `director stop` reaches the same walk from the other direction, through
// Registry.Cancel. Both, at once, is the point.
//
// The finish function must be called exactly once. It takes the refusal (empty when nothing was
// refused) and how many steps actually happened, because the command record is the honest account
// of what the desktop got, and "you stopped it" is a different fact from "it failed".
func (r *Runtime) beginPerformance(ctx context.Context, phrase string) (
	context.Context, func(refusal string, steps int), error) {

	if r.commands == nil {
		// NO REGISTRY, AND THAT IS NOT AN ERROR. A Runtime nobody published is a test's,
		// and refusing to walk would make every dry fixture in this package fail for a
		// reason that has nothing to do with what it is testing. The caller's context still
		// governs the walk, so what is lost is visibility, not stoppability.
		return ctx, func(string, int) {}, nil
	}
	if ctx == nil {
		ctx = r.serviceContext()
	}
	cmd, cmdCtx, err := r.commands.Begin(ctx, "", phrase)
	if err != nil {
		// REFUSED, NOT QUEUED, and the same rule a performance follows: two things driving
		// one desktop is the state the mutating slot exists to prevent. The error already
		// reads as a sentence — see service.ErrBusy — so it is passed up rather than
		// reworded into a refusal vocabulary that has no word for it.
		return nil, nil, fmt.Errorf("%w", err)
	}
	done := func(refusal string, steps int) {
		r.commands.Finish(cmd.ID, performanceState(refusal), steps, refusal)
	}
	return cmdCtx, done, nil
}

// performanceState classifies a finished walk for the command record.
//
// CANCELLED IS ITS OWN STATE, for the reason service.performState gives: "you stopped it" and "it
// failed" are different facts about the same half-finished walk, and a history that rendered them
// alike would tell somebody their play is broken when they are the one who stopped it.
//
// The cancelled word is the WALKER's, taken from the same constant perform.go's is — see
// TestTheCancelledWordIsTheWalkersWord, which holds the two spellings against each other.
func performanceState(refusal string) service.CommandState {
	switch refusal {
	case "":
		return service.CommandCompleted
	case cancelledWord:
		return service.CommandCancelled
	default:
		return service.CommandFailed
	}
}
