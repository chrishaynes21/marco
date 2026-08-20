package main

import (
	"context"
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/visualstate"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The watcher adapter: the pipeline's RegionWatcher, over the visual provider.
//
// It lives here, in the composition root, because it is the one place that is allowed
// to see both halves. The pipeline knows what a region is and nothing about
// fingerprints; the provider knows about fingerprints and nothing about actions. This
// joins them and holds no opinion of its own.

// regionWatcher adapts the visual-state provider to the execution pipeline.
type regionWatcher struct {
	provider *visualstate.Provider
	// window supplies the window a region belongs to, for the staleness check.
	window func(context.Context) (directorapi.Window, bool)
}

var _ execute.RegionWatcher = (*regionWatcher)(nil)

// Before captures the region an action's effect is expected in.
//
// A failure here is NOT an error the caller must handle. Visual evidence is an
// optional strengthening; a capture that could not be taken leaves the snapshot invalid
// and every downstream branch falls back to the structural guard exactly as it did
// before this milestone. Making it fatal would let a locked screen or a minimised
// window turn a working Director into a broken one.
func (w *regionWatcher) Before(ctx context.Context, region directorapi.Rect) (execute.RegionSnapshot, error) {
	win, _ := w.window(ctx)
	snap, err := w.provider.Capture(ctx, region, win)
	if err != nil {
		return execute.RegionSnapshot{Region: region}, nil
	}
	return execute.RegionSnapshot{
		Handle: snap, Region: region, Taken: snap.At, Valid: true,
	}, nil
}

// After compares the region now with the before-state, following it while it changes.
func (w *regionWatcher) After(ctx context.Context, before execute.RegionSnapshot) (execute.RegionChange, error) {
	prior, ok := before.Handle.(visualstate.Snapshot)
	if !ok || !before.Valid {
		return execute.RegionChange{}, nil
	}
	win, _ := w.window(ctx)

	res, taken, err := w.provider.Watch(ctx, prior, win)
	if err != nil {
		// Same reasoning as Before: no evidence, not a failure.
		return execute.RegionChange{}, nil
	}

	out := execute.RegionChange{
		Valid:         true,
		Rounds:        len(taken),
		Detail:        res.Reason,
		Changed:       res.Kind.Meaningful(),
		StillChanging: res.Kind == visualstate.ChangeStillChanging,
		Identical:     res.Kind == visualstate.ChangeIdentical,
	}
	// Whether the change looks like something drawn ON TOP — a menu, a dropdown — as
	// opposed to the region repainting. The distinction is what makes "clicking File
	// opened a menu" verifiable rather than merely "something happened".
	if len(taken) > 0 && out.Changed {
		if overlay, why := visualstate.OverlayAppeared(
			prior.Fingerprint, taken[len(taken)-1].Fingerprint,
			visualstate.DefaultThresholds()); overlay {
			out.Overlay = true
			out.Detail = why
		}
	}
	return out, nil
}

// ReadRegion runs one visual diagnostic pass over a region.
func (r *Runtime) ReadRegion(ctx context.Context, region *directorapi.Rect) visualstate.Diagnostics {
	if r.visual == nil {
		return visualstate.Diagnostics{Provider: "visual", Available: false,
			Error: "no visual provider is configured"}
	}
	// Observe first, so the region is interpreted against the window the Director
	// currently believes is in front.
	if _, err := r.pipeline.Observe(ctx); err != nil {
		return visualstate.Diagnostics{Provider: "visual", Available: false,
			Error: "could not observe before looking: " + err.Error()}
	}

	win, ok := r.activeWindow(ctx)
	if !ok {
		return visualstate.Diagnostics{Provider: "visual", Available: false,
			Error: "nothing is in front to look at"}
	}
	target := win.Bounds
	if region != nil {
		target = *region
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Two captures with a settle between them, which is the only way to say anything
	// about CHANGE — the thing this provider is for. A single snapshot can report
	// appearance and cannot report movement.
	before, err := r.visual.Capture(ctx, target, win)
	if err != nil {
		return visualstate.Diagnostics{Provider: "visual", Available: true, Error: err.Error()}
	}
	res, taken, err := r.visual.Watch(ctx, before, win)
	diag := r.visual.LastDiagnostics()
	diag.Provider, diag.Available = "visual", true
	diag.WindowID = win.ID
	diag.Regions = 1 + len(taken)
	diag.Change = &res
	if err != nil {
		diag.Error = err.Error()
		return diag
	}
	if diag.Detail == nil {
		diag.Detail = map[string]any{}
	}
	diag.Detail["region"] = fmt.Sprintf("%d,%d %dx%d",
		target.X, target.Y, target.Width, target.Height)
	diag.Detail["before_digest"] = before.Fingerprint.Digest
	if len(taken) > 0 {
		diag.Detail["after_digest"] = taken[len(taken)-1].Fingerprint.Digest
	}
	if len(taken) > 0 {
		if overlay, why := visualstate.OverlayAppeared(before.Fingerprint,
			taken[len(taken)-1].Fingerprint, visualstate.DefaultThresholds()); overlay {
			diag.Detail["overlay"] = why
		}
	}
	return diag
}
