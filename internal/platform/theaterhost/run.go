package theaterhost

import (
	"context"
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/activate"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Running what an Actor casts, through Marco.
//
// # The invariant this file holds
//
// Every real input passes through Marco compilation. `MarcoRunner.Run` compiles first and returns
// a compile error "BEFORE any desktop mutation", which is what makes attempting an unsupported
// operation safe — and what a dry run records.
//
// The Actor expresses its part as legal Marco and performs nothing. The Production boundary owns
// the runner. That way there is exactly one route from a decision to an effect, whether the
// decision came from a live rehearsal or from a saved play, and a dry run of either records the
// same program.
//
// An Actor that called a host directly would be a second route: no compile gate, nothing to
// record, and the two bodies drifting apart again — which is the whole of what Roadmap 34E exists
// to end.

// WithRunner installs the Marco runner every production is performed through.
//
// Returned rather than set, so a Theater built without one cannot be silently upgraded: the
// caller that has a runner is the caller that says so.
func (t *Theater) WithRunner(r directorapi.MarcoRunner) *Theater {
	t.runner = r
	return t
}

// run performs one cast candidate by walking the canonical ladder.
//
// The LADDER lives here rather than in the Actor because which ways there are, and in what order,
// is a production decision — the same one the live rehearsal path makes. The Actor only says what
// Marco expresses each way.
//
// Deleting the runner — calling a host directly — must fail TestAnActorNeverReachesAHostDirectly.
func (t *Theater) run(ctx context.Context, a Actor, c Candidate) (string, error) {
	if t.runner == nil {
		return "", fmt.Errorf("nothing can run a production here: no Marco runner is wired")
	}
	// The LAST program written, which is the one that decided the outcome — a control that
	// took three rungs to reach was activated by the third, and a ladder that ran out is
	// best explained by the way it gave up on.
	var ran string
	_, err := activate.Activate(ctx, func(ctx context.Context, w activate.Way) (bool, error) {
		program, ok := a.Cast(c, w)
		ran = program
		if !ok {
			// This Actor cannot express that way at all. Not the control refusing —
			// there is nothing to send — so the ladder moves on.
			return true, fmt.Errorf("%s cannot express %s", a.Name(), w)
		}
		// THE COMPILE GATE. An unexpressible program fails here, before any host.
		res, runErr := t.runner.Run(ctx, "theater", program)
		if runErr != nil {
			return activate.Unsupported(runErr.Error()), runErr
		}
		if why := failureIn(res); why != "" {
			return activate.Unsupported(why), fmt.Errorf("%s", why)
		}
		return false, nil
	})
	return ran, err
}

// failureIn is the host's own sentence when a run failed, empty when it did not.
// The FIRST failed capability, because a cast program calls exactly one and a later failure
// would be a consequence rather than the cause.
func failureIn(res directorapi.MarcoResult) string {
	if len(res.Failed) == 0 {
		return ""
	}
	first := res.Failed[0]
	if v, ok := res.Returned[first]; ok && v.Error != "" {
		return v.Error
	}
	return first + " failed"
}
