package theaterhost

import (
	"context"
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/production"
)

// The Theater as the Director may ask for it.
//
// # Why this exists beside Activate
//
// Because there were two bodies. A saved play reached the Theater through `Activate`; a live
// rehearsal never reached it at all and had its own resolution, its own execution and its own
// verification. Only one of them knew a Settings navigation item is a selection item.
//
// `Perform` is the one entry both callers use. Activate becomes the saved-play caller's thin way
// in, so the play path is unchanged in behaviour and identical in code.
//
// # What is added over Activate
//
// Authority, which a saved play does not need and a rehearsal may never be without; and a
// Verifier's expectation, so "did it work" is asked of the Director's own machinery rather than
// guessed at here.

var _ production.Producer = (*Theater)(nil)

// Perform puts on one production: claim authority, resolve, cast, act, verify.
//
// # The order is the contract
//
// Authority FIRST, before anything is resolved or looked at. A production that turns out to be
// impossible must still not have been permitted, and a Theater that resolved a target before
// checking whether it was allowed to act would leak "what is on your screen" past the permission
// boundary.
//
// Deleting the Claim call must fail TestTheaterRefusesWithoutAuthority.
func (t *Theater) Perform(ctx context.Context, r production.Request,
	a production.Authority, v production.Verifier) production.Report {

	if a == nil {
		// Nil authority is not "permitted by default". Every caller has one: a rehearsal
		// spends its grant, and a saved play carries the Audience's own request to run it.
		return production.Refuse(production.NotPermitted,
			"nothing authorises this production")
	}
	if err := a.Claim(r); err != nil {
		return production.Refuse(production.NotPermitted, "%s", err)
	}

	out := production.Report{Attempted: true}
	// WINDOW travels with the target. Which window a production belongs to is the Director's
	// knowledge and the Theater cannot guess it — and a search that ignored it would find a
	// control of the right name in whatever else happens to be open.
	done := t.Activate(ctx, Target{Name: r.Target.Name, Kind: r.Target.Kind, Window: r.Window})
	out.Performed, out.Cast, out.Program = done.Performed, done.Cast, done.Program
	if done.Refused != "" {
		out.Refused, out.Detail = production.Refusal(done.Refused), done.Detail
		// A production that never acted is finished here. One that acted and could not be
		// verified is reported below, because the world may have changed regardless.
		if !done.Performed {
			return out
		}
	}
	if !out.Performed {
		return out
	}

	// VERIFICATION, asked of the caller's own machinery when it brought some.
	//
	// Nil is honest and produces `not_verified` — never success. See production.Verifier.
	if v == nil {
		out.Refused = production.NotVerified
		if out.Detail == "" {
			out.Detail = "nothing here can check the result"
		}
		return out
	}
	observed, ok := v.Verify(ctx, r)
	out.Observed = observed
	if !ok {
		out.Refused = production.NotVerified
		if out.Detail == "" {
			out.Detail = fmt.Sprintf("the production ran and the world is not where it "+
				"was expected to be (%s)", orNothing(observed))
		}
		return out
	}
	out.Verified, out.Refused, out.Detail = true, "", ""
	return out
}

func orNothing(s string) string {
	if s == "" {
		return "nowhere it recognises"
	}
	return s
}
