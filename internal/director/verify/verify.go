// Package verify decides whether an action actually did what it was meant to.
//
// It works by COMPARING WORLD STATES — the snapshot before the action against the
// snapshot after. That constraint is the whole point of the package. The executor
// already knows it sent a click; the interesting question is whether anything
// happened, and only the world can answer that.
//
// What is explicitly not verification: that the click was sent, that the coordinates
// were where the target was, that no error was returned. All three are true of a
// click that landed on a disabled control, on a window that moved a moment earlier,
// or on nothing at all. Replaying what was done proves only that it was done.
//
// Evidence is weighted rather than counted. "Focus moved to the element I clicked"
// is close to proof; "the number of elements changed" is a hint. Collapsing them
// into a boolean would make a strong verdict and a weak one indistinguishable to the
// caller deciding whether to retry.
package verify

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Verifier compares world states to judge an action's effect.
type Verifier struct {
	// MinConfidence is the weighted-evidence total at which an action counts as
	// verified.
	MinConfidence float64
	// BoundsTolerance is how many pixels a window may miss its requested rectangle
	// by and still count as placed. Window managers adjust for borders, shadows and
	// snapping, so an exact match is the wrong test.
	BoundsTolerance int

	// Sources contribute evidence the Director cannot produce itself — "the craft queue
	// gained an entry" is a fact about a craft queue, and the Director does not know what
	// one is. Optional; without any, verification is exactly what it always was.
	//
	// Additive, weighed and capped. See contributed.go for why each of those is what
	// keeps a contributed source from being able to declare its own action verified.
	Sources []EvidenceSource
}

// New returns a Verifier with the default thresholds.
func New() *Verifier { return &Verifier{MinConfidence: 0.5, BoundsTolerance: 12} }

// Verify judges one executed action.
func (v *Verifier) Verify(action directorapi.Action, target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	// An after-state that is not actually after is no evidence at all. Comparing a
	// snapshot against itself, or against an older one, would "verify" anything.
	if !after.Timestamp.After(before.Timestamp) {
		return directorapi.VerificationResult{
			Inconclusive: true,
			Reason:       "the after-state is not newer than the before-state, so nothing could be compared",
		}
	}

	var res directorapi.VerificationResult
	switch a := action.(type) {
	case directorapi.ClickAction:
		res = v.verifyClick(a, target, before, after)
	case directorapi.FocusAction:
		res = v.verifyFocus(a, target, before, after)
	case directorapi.MoveWindowAction:
		res = v.verifyMove(a, target, before, after)
	case directorapi.EditAction:
		res = v.verifyEdit(a, target, before, after)
	case directorapi.SemanticAction:
		res = v.verifySemantic(a, target, before, after)
	default:
		res = directorapi.VerificationResult{
			Inconclusive: true,
			Reason:       fmt.Sprintf("no way to verify a %s action", action.ActionType()),
		}
	}
	// And what a source that understands this application makes of it. LAST, so a
	// contributed source can strengthen or rescue a verdict and never replace the
	// Director's own reading of the same change.
	return v.withContributed(res, action, target, before, after)
}

// verifyClick looks for any of the things a click can plausibly do.
//
// A click's effect is genuinely not knowable in advance: the same gesture opens a
// menu, moves focus, closes a dialog, toggles a checkbox or navigates a page. So
// this gathers alternatives and takes the strongest, rather than demanding a
// specific outcome that would be wrong for most controls.
func (v *Verifier) verifyClick(a directorapi.ClickAction, target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	var ev []directorapi.Evidence

	// A menu opening is the clearest signal there is, and it is what clicking a menu
	// item does — the case that motivated putting menu detection first.
	if opened, detail := menuOpened(before, after); opened {
		ev = append(ev, directorapi.Evidence{
			Kind: "menu_opened", Observed: true, Weight: 0.9, Detail: detail,
			Source: directorapi.SourceAccessibility,
		})
	}

	// Focus landing ON the element that was clicked is near-proof.
	if after.Cursor.Over != nil {
		_ = after.Cursor.Over // cursor position is not evidence; see the package doc
	}
	beforeFocus, afterFocus := focusedID(before), focusedID(after)
	switch {
	case afterFocus != "" && afterFocus == target.ElementID:
		ev = append(ev, directorapi.Evidence{
			Kind: "focus_on_target", Observed: true, Weight: 0.85,
			Detail: "keyboard focus is now on the element that was clicked",
			Source: directorapi.SourceAccessibility,
		})
	case beforeFocus != afterFocus:
		ev = append(ev, directorapi.Evidence{
			Kind: "focus_changed", Observed: true, Weight: 0.6,
			Detail: fmt.Sprintf("keyboard focus moved from %s to %s",
				labelOf(before, beforeFocus), labelOf(after, afterFocus)),
			Source: directorapi.SourceAccessibility,
		})
	}

	// A dialog or new window appearing.
	if len(after.Windows) > len(before.Windows) {
		ev = append(ev, directorapi.Evidence{
			Kind: "window_appeared", Observed: true, Weight: 0.8,
			Detail: fmt.Sprintf("a new window appeared (%d → %d)",
				len(before.Windows), len(after.Windows)),
			Source: directorapi.SourceWindowSystem,
		})
	}

	// The window's title changing — navigation, a document opening, a mode switch.
	if bt, at := windowTitle(before), windowTitle(after); bt != "" && at != "" && bt != at {
		ev = append(ev, directorapi.Evidence{
			Kind: "window_title_changed", Observed: true, Weight: 0.75,
			Detail: fmt.Sprintf("the window title changed from %q to %q", bt, at),
			Source: directorapi.SourceWindowSystem,
		})
	}

	// The target's own state changing: a checkbox ticking, a tab becoming selected.
	if changed, detail := targetStateChanged(target.ElementID, before, after); changed {
		ev = append(ev, directorapi.Evidence{
			Kind: "target_state_changed", Observed: true, Weight: 0.8, Detail: detail,
			Source: directorapi.SourceAccessibility,
		})
	}

	// The target disappearing — a dialog dismissed by its own OK button.
	if _, wasThere := before.Element(target.ElementID); wasThere {
		if _, stillThere := after.Element(target.ElementID); !stillThere {
			ev = append(ev, directorapi.Evidence{
				Kind: "target_gone", Observed: true, Weight: 0.7,
				Detail: "the element that was clicked is no longer present",
				Source: directorapi.SourceAccessibility,
			})
		}
	}

	// The weakest signal, and deliberately weak: the element set changed. It is
	// suggestive on its own and corroborating alongside anything else, but a busy
	// application changes its tree constantly for reasons unrelated to the click.
	if d := len(after.Elements) - len(before.Elements); d != 0 {
		ev = append(ev, directorapi.Evidence{
			Kind: "element_count_changed", Observed: true, Weight: 0.35,
			Detail: fmt.Sprintf("the number of elements changed by %+d", d),
			Source: directorapi.SourceAccessibility,
		})
	}

	if len(ev) == 0 {
		return directorapi.VerificationResult{
			Success: false,
			Evidence: []directorapi.Evidence{{
				Kind: "nothing_changed", Observed: true, Weight: 1,
				Detail: "the screen is identical before and after",
			}},
			Reason: "the click produced no observable change",
		}
	}
	return conclude(ev, v.MinConfidence, "the click had no clearly observable effect")
}

// verifyFocus checks the one thing focusing means.
func (v *Verifier) verifyFocus(a directorapi.FocusAction, target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	beforeFocus, afterFocus := focusedID(before), focusedID(after)

	if afterFocus != "" && afterFocus == target.ElementID {
		return directorapi.VerificationResult{
			Success: true, Confidence: 0.95,
			Evidence: []directorapi.Evidence{{
				Kind: "focus_on_target", Observed: true, Weight: 0.95,
				Detail: fmt.Sprintf("%q holds keyboard focus", labelOf(after, afterFocus)),
				Source: directorapi.SourceAccessibility,
			}},
			Reason: "focus is on the requested element",
		}
	}

	// Focus moved, but not to the element that was asked for. This is a FAILURE, not
	// a partial success: the request named something specific, and something else
	// now has focus. Reporting it as success is how a system ends up typing into the
	// wrong field.
	if beforeFocus != afterFocus {
		return directorapi.VerificationResult{
			Success: false, Confidence: 0.8,
			Evidence: []directorapi.Evidence{{
				Kind: "focus_changed_elsewhere", Observed: true, Weight: 0.8,
				Detail: fmt.Sprintf("focus moved to %q, not the requested element",
					labelOf(after, afterFocus)),
				Source: directorapi.SourceAccessibility,
			}},
			Reason: "focus moved somewhere else",
		}
	}

	// Some applications do not report focus at all. That is genuinely different from
	// having failed, and it must not be reported as either outcome.
	if afterFocus == "" {
		return directorapi.VerificationResult{
			Inconclusive: true,
			Evidence: []directorapi.Evidence{{
				Kind: "focus_not_reported", Observed: false, Weight: 0,
				Detail: "this application does not report which element has focus",
			}},
			Reason: "focus could not be observed, so the result is unknown",
		}
	}

	return directorapi.VerificationResult{
		Success: false, Confidence: 0.7,
		Evidence: []directorapi.Evidence{{
			Kind: "focus_unchanged", Observed: true, Weight: 0.7,
			Detail: "keyboard focus did not move",
		}},
		Reason: "focus did not move",
	}
}

// verifyMove checks a window against the rectangle that was requested.
//
// Two separate questions: did it move at all, and did it land where it was asked to.
// A window that moved to the wrong place is a failure the user needs told about, and
// only comparing against the REQUESTED bounds can detect it.
func (v *Verifier) verifyMove(a directorapi.MoveWindowAction, target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	id := a.Window.ID
	if id == "" {
		id = target.WindowID
	}
	was, hadBefore := before.Window(id)
	now, hasAfter := after.Window(id)
	if !hadBefore || !hasAfter {
		return directorapi.VerificationResult{
			Inconclusive: true,
			Reason:       "the window could not be found in both snapshots",
		}
	}

	var ev []directorapi.Evidence
	moved := was.Bounds != now.Bounds
	ev = append(ev, directorapi.Evidence{
		Kind: "window_bounds_changed", Observed: moved, Weight: 0.5,
		Detail: fmt.Sprintf("bounds %s → %s", rectStr(was.Bounds), rectStr(now.Bounds)),
		Source: directorapi.SourceWindowSystem,
	})

	if want := a.Placement.Bounds; want != nil {
		placed := withinTolerance(*want, now.Bounds, v.BoundsTolerance)
		ev = append(ev, directorapi.Evidence{
			Kind: "window_at_requested_bounds", Observed: placed, Weight: 0.9,
			Detail: fmt.Sprintf("requested %s, got %s", rectStr(*want), rectStr(now.Bounds)),
			Source: directorapi.SourceWindowSystem,
		})
		if !placed {
			reason := "the window did not move"
			if moved {
				reason = "the window moved, but not to the requested position"
			}
			return directorapi.VerificationResult{
				Success: false, Confidence: 0.9, Evidence: ev, Reason: reason,
			}
		}
	}

	if want := a.Placement.MonitorID; want != "" {
		onTarget := now.MonitorID == want
		ev = append(ev, directorapi.Evidence{
			Kind: "window_on_requested_monitor", Observed: onTarget, Weight: 0.9,
			Detail: fmt.Sprintf("requested %s, on %s", want, now.MonitorID),
			Source: directorapi.SourceWindowSystem,
		})
		if !onTarget {
			return directorapi.VerificationResult{
				Success: false, Confidence: 0.9, Evidence: ev,
				Reason: "the window is not on the requested monitor",
			}
		}
	}

	if !moved {
		return directorapi.VerificationResult{
			Success: false, Confidence: 0.9, Evidence: ev,
			Reason: "the window did not move",
		}
	}
	return conclude(ev, v.MinConfidence, "the window move could not be confirmed")
}

// conclude turns weighted evidence into a verdict.
//
// Confidence is the strongest single piece of evidence, raised by corroboration:
// several weak signals should not add up to more certainty than one strong one, but
// they should count for something.
func conclude(ev []directorapi.Evidence, min float64, failReason string) directorapi.VerificationResult {
	best := 0.0
	for _, e := range ev {
		if e.Observed && e.Weight > best {
			best = e.Weight
		}
	}
	conf := best
	for _, e := range ev {
		if !e.Observed || e.Weight == best {
			continue
		}
		conf += (1 - conf) * 0.4 * e.Weight
	}
	if conf > 0.99 {
		conf = 0.99
	}

	res := directorapi.VerificationResult{
		Success: conf >= min, Confidence: conf, Evidence: ev,
	}
	if res.Success {
		for _, e := range ev {
			if e.Observed && e.Weight == best {
				res.Reason = e.Detail
				break
			}
		}
	} else {
		res.Reason = failReason
	}
	return res
}

// menuOpened detects a menu having been revealed: menu items that were not there
// before now are.
func menuOpened(before, after directorapi.WorldState) (bool, string) {
	beforeItems := map[directorapi.ElementID]bool{}
	for id, el := range before.Elements {
		if el.Role == directorapi.RoleMenuItem && el.Visible {
			beforeItems[id] = true
		}
	}
	fresh := 0
	var sample string
	for id, el := range after.Elements {
		if el.Role != directorapi.RoleMenuItem || !el.Visible || beforeItems[id] {
			continue
		}
		fresh++
		if sample == "" && el.Label != "" {
			sample = el.Label
		}
	}
	// One new item is noise — a menu bar re-reporting an entry. A menu OPENING
	// reveals several at once.
	if fresh < 2 {
		return false, ""
	}
	detail := fmt.Sprintf("%d menu items appeared", fresh)
	if sample != "" {
		detail += fmt.Sprintf(", including %q", sample)
	}
	return true, detail
}

// targetStateChanged reports whether the element acted on changed its own state.
func targetStateChanged(id directorapi.ElementID, before, after directorapi.WorldState) (bool, string) {
	was, ok1 := before.Element(id)
	now, ok2 := after.Element(id)
	if !ok1 || !ok2 {
		return false, ""
	}
	switch {
	case was.Selected != now.Selected:
		return true, fmt.Sprintf("%q became %s", now.Label, selectedWord(now.Selected))
	case was.Value != now.Value:
		return true, fmt.Sprintf("%q changed from %q to %q", now.Label, was.Value, now.Value)
	case was.Enabled != now.Enabled:
		return true, fmt.Sprintf("%q became %s", now.Label, enabledWord(now.Enabled))
	}
	return false, ""
}

func focusedID(w directorapi.WorldState) directorapi.ElementID {
	for id, el := range w.Elements {
		if el.Focused {
			return id
		}
	}
	return ""
}

func labelOf(w directorapi.WorldState, id directorapi.ElementID) string {
	if id == "" {
		return "nothing"
	}
	if el, ok := w.Element(id); ok && el.Label != "" {
		return el.Label
	}
	return string(id)
}

func windowTitle(w directorapi.WorldState) string {
	if win, ok := w.FocusedWindow(); ok {
		return win.Title
	}
	if len(w.Windows) > 0 {
		return w.Windows[0].Title
	}
	return ""
}

// withinTolerance reports whether two rectangles agree to within a few pixels.
// Window managers adjust for invisible borders, drop shadows and snapping, so
// demanding an exact match would fail every real move.
func withinTolerance(want, got directorapi.Rect, tol int) bool {
	return abs(want.X-got.X) <= tol && abs(want.Y-got.Y) <= tol &&
		abs(want.Width-got.Width) <= tol && abs(want.Height-got.Height) <= tol
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func rectStr(r directorapi.Rect) string {
	return fmt.Sprintf("(%d,%d %dx%d)", r.X, r.Y, r.Width, r.Height)
}

func selectedWord(b bool) string {
	if b {
		return "selected"
	}
	return "unselected"
}

func enabledWord(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}
