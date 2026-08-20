package main

import (
	"context"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/platform/screenhost"
)

// One answer to "what place is showing right now".
//
// # Why this file is four lines of logic and forty of prose
//
// Because the tempting version of it is a second recogniser. There were two answers to this
// question in the tree and one of them was a stub: `cmd/director` took a genuinely fresh look
// through `freshPlace`, and `cmd/marco`'s `liveScreens.CurrentSubject` returned `unavailable`
// unconditionally with a comment explaining that standalone Marco has no eyes. The comment was
// true and the consequence was not survivable: every learned play's generated Marco opens with
// `do Screen's Showing with "<place>"`, and an EDITED learned play is not delegated to the
// Director — editing it makes it an ordinary play, which is the authority policy working exactly
// as designed — so it took the local runner and refused at its own first line with "Marco could
// not check". The Plays view puts an edit button on every row.
//
// The wrong fixes, both of which were available and neither of which is here:
//
//   - Let `CurrentSubject` guess. 34F's own risk table names this: "The fastest way to make this
//     go away is to let CurrentSubject guess. That destroys ADR-031-the-user-names-the-stage and
//     the 'silence is never yes' invariant in one line."
//   - Route edited plays to the Director too. Worse, not better: `performLearned` sends only
//     Name/Application/Subject, so the Director re-plans from the goal and the person's edits are
//     silently discarded. A play that runs something other than its own source is not that play.
//
// So the answer is to let the local runner SEE, by asking the one thing in this system that can.
//
// # Why freshPlace itself, exposed, rather than a read-only sibling
//
// `freshPlace` IS the read-only body. It looks — `placeNowIn` when a live session is already
// watching this application, otherwise `lookNow`, which starts a short bounded observation
// through `StartObservation`, the same production path the Sight surface uses — and it resolves
// what it saw with `observe.PlaceNow`, the canonical resolver. Nothing on that path presses a
// key, activates a window, mints an authority or writes to memory. `PerformGoal` brings the
// application forward BEFORE it calls this, and that foregrounding is the caller's, not
// `freshPlace`'s: a query answered here disturbs nothing.
//
// A sibling that took its own look would be the second recogniser this whole design refuses, and
// it would be the second recogniser in the place it hurts most — one opinion planning a route and
// a different opinion deciding whether the play may begin.

// ShowingNow answers which remembered place is in front right now, in one application.
//
// It reports an outcome in `screenhost`'s closed vocabulary and never an assumption. The four
// arms below are exhaustive over what `freshPlace` can return, and every one of them that is not
// a positive identification is a refusal at the far end.
//
// The vocabulary crosses the wire as a bare string because `internal/director` may not import
// platform code — an enforced boundary, and the right one. The conversion happens HERE, at the
// composition root, from `screenhost`'s own constants; the far end converts back the same way.
// No word is written out by hand at either end, so there is still one vocabulary and not two.
//
// The error return is for the `Observation` switch's signature, and is always nil. A refusal is
// an ANSWER — "I looked and recognised nothing" is a fact about the world — and turning it into a
// transport error would flatten four distinct outcomes into one, which is the collapse ADR-031
// keeps five outcomes apart to prevent.
//
// Deleting the `freshPlace` call and answering from a stored place must fail
// TestShowingNowTakesAFreshLookRatherThanReadingHistory.
func (r *Runtime) ShowingNow(ctx context.Context, q service.ObserveShowing) (
	service.ShowingView, error) {

	application := strings.TrimSpace(q.Application)
	out := service.ShowingView{Application: application}

	// NOBODY NAMED AN APPLICATION. Not "look at whatever is in front": a play's entry
	// condition is about the application the play is in, and a guard satisfiable by a
	// different program is worse than no guard.
	if application == "" {
		out.Outcome = string(screenhost.Unavailable)
		out.Why = "no application was named to look in"
		return out, nil
	}
	if r.observations == nil {
		out.Outcome = string(screenhost.Unavailable)
		out.Why = "this Director has no observation registry"
		return out, nil
	}

	subject, why := r.freshPlace(ctx, application)
	switch {
	case subject != "":
		out.Outcome, out.Subject = string(screenhost.Recognised), subject
	case why != "":
		// A look that could not be TAKEN — something else is being watched, or there is no
		// window to look at. `freshPlace` returns its reason with a leading ": " because its
		// own caller splices it onto a sentence; this one is a field, not a fragment.
		out.Outcome = string(screenhost.Unobservable)
		out.Why = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(why), ":"))
	case ctx.Err() != nil:
		// STOPPED. `freshPlace` abandons its poll when the context ends and reports the same
		// empty pair it reports for a timeout. Those are different facts, and calling a
		// cancelled look "unrecognised" would tell whoever reads the diagnostics that Marco
		// looked at a screen it never saw.
		out.Outcome = string(screenhost.Unobservable)
		out.Why = "the look was stopped before it could answer"
	default:
		// LOOKED, AND MATCHED NOTHING REMEMBERED. The honest end of the road, and the one
		// that must stay a refusal however inconvenient it is.
		out.Outcome = string(screenhost.Unknown)
		out.Why = "nothing remembered in " + application + " matches what is in front"
	}
	return out, nil
}
