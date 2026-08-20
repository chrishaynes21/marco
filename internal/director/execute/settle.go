package execute

import (
	"context"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/wait/conditions"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/engine"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/evaluation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Settling after an action: the sleep this milestone exists to remove.
//
//	Time is never evidence. Observations are evidence.
//	The Director waits because it cannot yet prove the required condition,
//	not because it guesses something will eventually happen.
//
// What was here before was 350ms, chosen because a menu usually renders in less than
// that. It is wrong in both directions and invisible in both. Too short on a loaded
// machine and verification runs against a half-drawn screen, concluding the action
// failed; too long and every command in the Director is slower than it needs to be,
// forever, with nothing to point at.
//
// A condition is falsifiable where a duration is not. "The region around the target has
// stopped changing" is answered by looking, can be Unknown, and finishes the instant it
// becomes true rather than when a number elapses.

// SettleWaiter waits for a semantic condition. Supplied by the composition root;
// without one the pipeline keeps its old behaviour exactly.
type SettleWaiter interface {
	Wait(ctx context.Context, condition conditions.Condition, opts engine.Options) (engine.Result, error)
}

// settleTimeout bounds the post-action wait.
//
// Generous compared with the old fixed delay, and it costs nothing to be: a wait that
// is satisfied stops immediately. The bound exists for the interface that never
// settles — a spinner, a video, a progress bar — where the honest answer after a while
// is "this is still going", not a longer sleep.
const settleTimeout = 3 * time.Second

// settleFor waits for the screen around a target to stop changing.
//
// Falls back to the old delay in exactly two cases, both of which are the absence of a
// condition rather than a preference for time:
//
//   - no waiter or no region watcher is wired, so the condition cannot be evaluated;
//   - the action has no target region, so there is nothing to watch.
//
// Those are the "existing sleeps may remain only where no observable condition exists"
// case, and they are the only two.
func (p *Pipeline) settleFor(ctx context.Context, action directorapi.Action, target directorapi.ResolvedTarget,
	world directorapi.WorldState, add func(string, string, bool)) *engine.Result {

	// A semantic edit needs no settle at all. The editor already waited for the
	// control's VALUE to reflect the change and read it back — evidence about the one
	// thing the edit was about. Watching pixels afterwards would add up to three
	// seconds per keystroke-free edit to answer a weaker question, and on a control
	// the capture cannot reach (a protected window, a hidden surface) it answers
	// nothing at all while still spending the time.
	if _, isEdit := action.(directorapi.EditAction); isEdit {
		add("settle", "not needed: the edit was verified against the control's value", true)
		return nil
	}

	region, ok := watchRegion(target, world)
	if p.Waiter == nil || p.Watcher == nil || !ok {
		// No observable condition. Documented at the call site rather than silently
		// sleeping, so a reader can tell a considered fallback from an overlooked one.
		reason := "no region watcher is wired"
		if p.Waiter == nil {
			reason = "no wait engine is wired"
		} else if !ok {
			reason = "the action has no target region to watch"
		}
		add("settle", "fixed delay ("+reason+")", true)
		select {
		case <-time.After(p.settle()):
		case <-ctx.Done():
		}
		return nil
	}

	cond := conditions.RegionStable{
		Region:  region,
		Sampler: p.sampler(ctx),
	}
	res, err := p.Waiter.Wait(ctx, cond, engine.Options{
		Timeout: settleTimeout,
		// Two consecutive quiet looks. One is not stability: an animation is
		// momentarily identical between frames, and a page part-way through loading is
		// quiet between paints.
		StableObservations: 2,
		PollInterval:       80 * time.Millisecond,
	})
	if err != nil {
		add("settle", "wait failed, falling back to a fixed delay: "+err.Error(), false)
		select {
		case <-time.After(p.settle()):
		case <-ctx.Done():
		}
		return nil
	}

	add("settle", "waited "+string(res.Condition)+": "+res.Summary(),
		res.Final.State != evaluation.TimedOut)
	return &res
}

// sampler adapts the pipeline's region watcher to the wait layer.
//
// Each call is a fresh before/after pair rather than a running comparison, because a
// condition must be answerable at any moment and from scratch — the engine may evaluate
// it once or twenty times, and the answer has to mean the same thing every time.
func (p *Pipeline) sampler(ctx context.Context) conditions.RegionSampler {
	if p.Watcher == nil {
		return nil
	}
	return func(region directorapi.Rect) conditions.Sample {
		before, err := p.Watcher.Before(ctx, region)
		if err != nil || !before.Valid {
			return conditions.Sample{Region: region}
		}
		change, err := p.Watcher.After(ctx, before)
		if err != nil || !change.Valid {
			return conditions.Sample{Region: region}
		}
		return conditions.Sample{
			Observed:      true,
			Changed:       change.Changed,
			StillChanging: change.StillChanging,
			Identical:     change.Identical,
			Detail:        change.Detail,
			Region:        region,
		}
	}
}

// awaitStable waits for the target region to settle, then re-observes.
//
// The deferral path. It exists because of a real failure: a page mid-navigation has an
// unchanged accessibility tree, so verification fails, and the retry clicks the same
// control again. Waiting is the correct response to "still changing", and it is not
// the same as retrying — the action is not repeated, only the LOOKING is.
func (p *Pipeline) awaitStable(ctx context.Context, target directorapi.ResolvedTarget,
	world directorapi.WorldState, add func(string, string, bool)) (string, directorapi.WorldState, bool) {

	region, ok := watchRegion(target, world)
	if p.Waiter == nil || p.Watcher == nil || !ok {
		return "", directorapi.WorldState{}, false
	}
	add("verify", "the region is still changing — waiting for it to settle rather than "+
		"retrying, because repeating a non-idempotent action mid-flight applies it twice", true)

	res, err := p.Waiter.Wait(ctx, conditions.RegionStable{
		Region: region, Sampler: p.sampler(ctx),
	}, engine.Options{
		Timeout:            deferTimeout,
		StableObservations: 2,
		PollInterval:       120 * time.Millisecond,
	})
	if err != nil || res.Final.State == evaluation.Cancelled {
		return "", directorapi.WorldState{}, false
	}

	fresh, oerr := p.Observe(ctx)
	if oerr != nil {
		return "", directorapi.WorldState{}, false
	}
	return res.Summary(), fresh, true
}

// deferTimeout bounds the wait before verification gives up and reports Unverified.
//
// Longer than the ordinary settle, because this path is only reached when something IS
// demonstrably happening — a page loading, a dialog animating — and the whole point is
// to sit through it. Still bounded: an interface that never settles gets an honest
// "could not confirm", not an indefinite wait.
const deferTimeout = 8 * time.Second
