package execute

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Visual verification, and the retry guard it feeds.
//
// This is the point of the whole visual milestone, and it came from a real failure.
// Clicking Back in Chrome navigates the page — but the navigation had not finished
// within the settle delay, so structural verification failed and the retry clicked Back
// a SECOND time, sending the browser two pages back when the user asked for one.
//
// The guard that fixed it was: retry only when the world is byte-for-byte what it was.
// That is correct and it is BLIND. A page mid-navigation has an unchanged accessibility
// tree — the old one, because the new one does not exist yet — so "nothing changed"
// and "everything is changing" look identical from the structural side.
//
// Watching the pixels answers exactly that question and nothing more:
//
//	identical       → nothing happened; a retry cannot double-apply
//	minor_change    → noise; the structural guard still decides
//	meaningful      → something happened; a retry might do it twice
//	still_changing  → something is happening RIGHT NOW; wait, do not retry
//
// The principle the guard exists to protect, restated because every branch here has to
// honour it: FAILURE TO CONFIRM IS NOT PROOF THAT NOTHING HAPPENED.

// RegionWatcher observes whether a region of the screen changed.
//
// An interface so the pipeline never learns what a fingerprint is, and so a Director
// with no capture layer simply has none — the field is nil and every path below falls
// back to the structural guard alone.
type RegionWatcher interface {
	// Before captures a region and returns an opaque handle to compare against.
	Before(ctx context.Context, region directorapi.Rect) (RegionSnapshot, error)
	// After compares the region now with the handle, following it while it is still
	// changing, up to the watcher's own bound.
	After(ctx context.Context, before RegionSnapshot) (RegionChange, error)
}

// RegionSnapshot is an opaque before-state.
type RegionSnapshot struct {
	// Handle is whatever the watcher needs to compare later. The pipeline never reads
	// it; it exists so this interface can be satisfied without exporting a fingerprint.
	Handle any
	Region directorapi.Rect
	Taken  time.Time
	// Valid is false when no capture could be taken. A caller must treat that as "no
	// visual evidence", never as "no change".
	Valid bool
}

// RegionChange is what the watcher concluded.
type RegionChange struct {
	// Changed is whether something meaningful happened.
	Changed bool
	// StillChanging is whether it is happening now.
	StillChanging bool
	// Identical is whether the region is pixel-for-pixel what it was. Distinct from
	// !Changed, which is also true for a change dismissed as noise.
	Identical bool
	// Overlay is whether the change looks like something drawn on top — a menu, a
	// dropdown, a dialog.
	Overlay bool
	Detail  string
	// Rounds is how many follow-up captures were taken.
	Rounds int
	// Valid is false when the comparison could not be made.
	Valid bool
}

// RetryVerdict is whether a failed action may be attempted again.
type RetryVerdict struct {
	Allow  bool
	Reason string
	// Wait asks the caller to re-observe before deciding, because the screen is still
	// settling. Never combined with Allow: something in flight is never a reason to
	// act, only a reason to look again.
	Wait bool
}

// visualRetryVerdict decides whether a retry is safe, given what the pixels showed.
//
// Ordered from the strongest evidence to the weakest, and every branch that is not
// certain refuses. A wrong "allow" double-applies a non-idempotent action; a wrong
// "refuse" costs the user one re-issued command. Those are not close.
func visualRetryVerdict(change RegionChange, structuralChanged bool) RetryVerdict {
	switch {
	case !change.Valid:
		// No visual evidence. Falls back to the structural guard exactly as before —
		// this milestone must not make the no-capture case behave differently.
		if structuralChanged {
			return RetryVerdict{Reason: "the world changed, so the action may have landed"}
		}
		return RetryVerdict{Allow: true,
			Reason: "the world is unchanged and no visual evidence was available"}

	case change.StillChanging:
		// The case the structural guard could not see. Something is in flight; acting
		// now would land in the middle of it, and the screen a moment from now is not
		// the screen that was measured.
		return RetryVerdict{Wait: true,
			Reason: "the region is still changing — the action may be taking effect right " +
				"now, and repeating it would apply it twice"}

	case change.Changed:
		return RetryVerdict{
			Reason: "the region changed, so the action may have landed even though it " +
				"could not be confirmed: " + change.Detail}

	case structuralChanged:
		// Pixels quiet, structure moved. Structure is the stronger source; a repaint
		// that averages away is still a repaint.
		return RetryVerdict{
			Reason: "the world changed structurally even though the region looks the same"}

	case change.Identical:
		return RetryVerdict{Allow: true,
			Reason: "the region is pixel-for-pixel what it was and the world is unchanged, " +
				"so nothing happened and repeating it cannot double-apply"}
	}

	// Neither identical nor meaningfully changed: minor noise, unchanged structure.
	// Permitted, because the structural guard would have permitted it and the visual
	// evidence adds nothing that argues otherwise.
	return RetryVerdict{Allow: true,
		Reason: "only rendering noise differs and the world is unchanged"}
}

// watchRegion is the region an action's visible effect is expected in.
//
// The target's own bounds plus a margin, because a click's effect is frequently just
// OUTSIDE the control: a menu opens below it, a dropdown beside it, a tooltip above.
// A region clipped to the control alone would call all of that "no change" and permit
// a retry that opens the menu twice.
// The target's bounds come from the WORLD rather than from the resolved target, which
// records a click point and not a rectangle. Looked up rather than reconstructed: the
// point alone would give a region centred on the click with no idea how big the control
// is, and a menu item is a very different size from a toolbar.
func watchRegion(target directorapi.ResolvedTarget, w directorapi.WorldState) (directorapi.Rect, bool) {
	var r directorapi.Rect
	if el, ok := w.Element(target.ElementID); ok && !el.Bounds.Empty() {
		r = el.Bounds
	} else if win, ok := w.Window(target.WindowID); ok && !win.Bounds.Empty() {
		r = win.Bounds
	}
	if r.Width <= 0 || r.Height <= 0 {
		return directorapi.Rect{}, false
	}
	dx, dy := r.Width/2, r.Height*2
	if dx < 16 {
		dx = 16
	}
	if dy < 48 {
		dy = 48
	}
	return directorapi.Rect{
		X: r.X - dx, Y: r.Y - dy/4,
		Width: r.Width + 2*dx, Height: r.Height + dy,
	}, true
}

// visualEvidence turns a region change into verification evidence.
//
// Deliberately WEAK evidence. A region changing after a click is consistent with the
// click having worked and equally consistent with an unrelated repaint, an animation,
// or a notification arriving. It supports a verdict and never establishes one, which is
// why the confidence is low and why the description says what was observed rather than
// what it means.
func visualEvidence(change RegionChange) (directorapi.Evidence, bool) {
	if !change.Valid {
		return directorapi.Evidence{}, false
	}
	switch {
	case change.Overlay:
		return directorapi.Evidence{
			Kind:     "visual_overlay_appeared",
			Observed: true,
			Weight:   0.6,
			Source:   directorapi.SourceVision,
			Detail:   "something was drawn on top near the target: " + change.Detail,
		}, true
	case change.StillChanging:
		return directorapi.Evidence{
			Kind:     "visual_still_changing",
			Observed: false,
			Weight:   0.5,
			Source:   directorapi.SourceVision,
			Detail: "the region is still changing, so whether the action took effect " +
				"cannot be decided yet: " + change.Detail,
		}, true
	case change.Changed:
		return directorapi.Evidence{
			Kind:     "visual_region_changed",
			Observed: true,
			Weight:   0.45,
			Source:   directorapi.SourceVision,
			Detail: "the target region changed appearance, which is consistent with " +
				"the action having landed without establishing that it did: " + change.Detail,
		}, true
	case change.Identical:
		return directorapi.Evidence{
			Kind:     "visual_region_unchanged",
			Observed: false,
			Weight:   0.5,
			Source:   directorapi.SourceVision,
			Detail: "the target region is pixel-for-pixel what it was, so nothing " +
				"visible happened",
		}, true
	}
	return directorapi.Evidence{}, false
}

func describeChange(c RegionChange) string {
	if !c.Valid {
		return "no visual evidence"
	}
	switch {
	case c.StillChanging:
		return fmt.Sprintf("still changing after %d rounds", c.Rounds)
	case c.Changed:
		return "changed"
	case c.Identical:
		return "identical"
	}
	return "unchanged but for noise"
}

// visualChangeSummary renders a change for the action record.
//
// A short string rather than the structure: the action graph is durable and append-only,
// and a fingerprint grid stored forever would be a large, unreadable artifact describing
// pixels nobody can see again. What survives is the CONCLUSION and why.
func visualChangeSummary(c RegionChange) string {
	if !c.Valid {
		return ""
	}
	return describeChange(c) + " — " + c.Detail
}

// VisualChangeResult reconstructs the change verdict from an action record.
//
// The record stores a summary string rather than the structure, because the action
// graph is durable and a fingerprint grid stored forever would be an unreadable
// artifact about pixels nobody can see again. The retry guard needs only the verdict,
// which the summary carries — and when no watcher ran, an invalid result, which is
// what makes the no-capture path behave exactly as it did before.
func visualChangeOf(record directorapi.ActionRecord) RegionChange {
	s := record.VisualChange
	if s == "" {
		return RegionChange{}
	}
	c := RegionChange{Valid: true, Detail: s}
	switch {
	case strings.HasPrefix(s, "still changing"):
		c.StillChanging, c.Changed = true, true
	case strings.HasPrefix(s, "changed"):
		c.Changed = true
	case strings.HasPrefix(s, "identical"):
		c.Identical = true
	}
	return c
}
