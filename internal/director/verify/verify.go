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
// Evidence is weighted rather than counted. "The application now says Saved" is close to proof;
// "the number of elements changed" is a hint. Collapsing them into a boolean would make a strong
// verdict and a weak one indistinguishable to the caller deciding whether to retry.
//
// And the weights divide along one line, which is the line the paragraph above draws:
//
//	DELIVERED   the action reached the control Marco aimed at
//	EFFECTIVE   the application then did what was asked
//
// Evidence of delivery — focus landing on the thing that was clicked — is real, is worth recording,
// and is never sufficient, because it is the control responding to being clicked rather than to
// what the click was for. See [WeightDelivered]. This paragraph used to say that focus landing on
// the target was "close to proof", which contradicted the one above it; the code agreed with the
// wrong half, and an acceptance against a button built to do nothing found it.
package verify

import (
	"fmt"
	"strings"

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

// WeightDelivered is what evidence of DELIVERY is worth: real, and never sufficient.
//
// # The distinction this constant exists to hold
//
//	DELIVERED   the action reached the control Marco aimed at
//	EFFECTIVE   the application then did what was asked
//
// The first is about Marco and is worth knowing: it is what separates "the button did nothing"
// from "Marco clicked the wrong thing", and those send a reader to two different places. The second
// is what the word "verified" means to everybody who reads it.
//
// Deliberately below `MinConfidence`, and that is the whole of its job. Evidence at this weight
// corroborates anything else and concludes nothing on its own — so an action whose only trace is
// that it was delivered comes back unverified, with its delivery recorded.
//
// It is also deliberately below the point where two such signals combine past the threshold:
// `conclude` adds a second signal as `conf + (1-conf)*0.4*w`, so two hints at this weight reach
// 0.48 and still refuse. Two mechanical consequences of a click are not one effect.
const WeightDelivered = 0.4

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

	// WHAT THE APPLICATION DID, independent of how the action was delivered. Shared with
	// every other verifier that needs to ask the same question — see changeEvidence.
	ev := v.changeEvidence(target, before, after)

	// Focus landing ON the element that was clicked proves DELIVERY, not effect.
	//
	// # Why this is weighted below the threshold
	//
	// Because taking focus is how a control responds to BEING CLICKED. It says the click
	// reached the thing Marco aimed at — which is genuinely valuable, and is the best evidence
	// there is against the wrong-target failure this package exists to catch — and it says
	// nothing whatever about whether the application then did anything.
	//
	// It was 0.85, above MinConfidence, so it could conclude on its own. The consequence,
	// found by the real-desktop acceptance harness on its first run against a button built to
	// accept a press and do nothing: Marco clicked, focus landed, Marco reported `performed`
	// with verification ok, and an independent oracle confirmed the application had not acted.
	// Every inert button in every application verified — a failed validation, a swallowed
	// exception, a control that is enabled and does nothing.
	//
	// This package's own documentation already said so, and the weight disagreed with it:
	// "What is explicitly not verification: that the click was sent … All three are true of a
	// click that landed on a disabled control."
	//
	// So it is DEMOTED rather than removed. Below MinConfidence it cannot carry a verdict
	// alone; beside any real signal it still corroborates, which is what keeps a genuine click
	// as easy to confirm as it was.
	//
	// Deleting the demotion must fail TestAClickThatOnlyTookFocusIsNotVerified.
	if after.Cursor.Over != nil {
		_ = after.Cursor.Over // cursor position is not evidence; see the package doc
	}
	beforeFocus, afterFocus := focusedID(before), focusedID(after)
	switch {
	case afterFocus != "" && afterFocus == target.ElementID:
		ev = append(ev, directorapi.Evidence{
			Kind: "focus_on_target", Observed: true, Weight: WeightDelivered,
			Detail: "keyboard focus is now on the element that was clicked, which shows " +
				"the click was delivered there and not that it had an effect",
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

	// The target.s own state changing: a checkbox ticking, a tab becoming selected. Appended
	// here rather than shared, because for a verb whose whole subject IS that state — select,
	// check, expand — it would be the same fact counted twice.
	if changed, detail := targetStateChanged(target.ElementID, before, after); changed {
		ev = append(ev, directorapi.Evidence{
			Kind: "target_state_changed", Observed: true, Weight: 0.8, Detail: detail,
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

// maxTargetedTextChanges is how many labels may change before it reads as churn rather than as an
// answer.
//
// Two. A control responding to an action says one thing — "Saved", "3 items", an error — and
// occasionally two, when a status line and a count move together. Beyond that the application is
// repainting, and the existing `element_count_changed` reasoning already treats that volume of
// change as a hint.
const maxTargetedTextChanges = 2

// saidSomethingDifferent counts the labels that exist in both worlds and now read differently.
//
// # Scoped to the target's own window
//
// Because a notification in another application is not this application answering. The world holds
// every window Marco observed, and a verifier that read all of them would accept a chat message
// arriving as proof that a button worked.
//
// The target's own element is excluded: its state is covered by `targetStateChanged`, and counting
// it twice would let one fact carry two pieces of evidence.
//
// Returns the count and a sentence naming at most two of them, because a reason is read by a person
// and "11 labels changed" tells them less than the first two do.
func saidSomethingDifferent(target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) (int, string) {

	window := target.WindowID
	if window == "" {
		if el, ok := after.Element(target.ElementID); ok {
			window = el.WindowID
		}
	}

	n := 0
	var named []string
	for id, now := range after.Elements {
		if id == target.ElementID {
			continue
		}
		if window != "" && now.WindowID != window {
			continue
		}
		was, ok := before.Element(id)
		if !ok {
			// Appearing is not saying something different; it is covered above.
			continue
		}
		if was.Label == now.Label {
			continue
		}
		n++
		if len(named) < 2 {
			named = append(named, fmt.Sprintf("%q → %q", was.Label, now.Label))
		}
	}
	if n == 0 {
		return 0, ""
	}
	detail := "the application now says " + strings.Join(named, " and ")
	if n > len(named) {
		detail = fmt.Sprintf("%d labels changed, including %s", n, strings.Join(named, " and "))
	}
	return n, detail
}

// changeEvidence is what the APPLICATION did, independent of how the action was delivered.
//
// # Why this is shared
//
// Because "did the world change, and how much" is the same question whatever verb was asked for,
// and it used to be answered only inside `verifyClick`. Every other verifier read one specific
// signal — a node's expanded flag, an item's selected flag — and had no way to see that the
// application had also said "Saved", revealed a panel or opened a window.
//
// That asymmetry is what let a selection verify on the control's own word: there was nothing else
// for it to be weighed against. Extracted verbatim rather than rewritten, so `verifyClick` behaves
// exactly as it did and the weights stay in one place.
//
// # What is deliberately NOT here
//
// `focus_on_target` and `focus_changed`, because focus is a fact about DELIVERY and belongs to the
// verb that caused it — see WeightDelivered.
//
// `targetStateChanged`, because for several verbs it reports the very fact being verified. A
// selection verified partly by "the target's selected state changed" would be one self-report
// carrying two pieces of evidence past the threshold. `verifyClick` appends it itself, where a
// checkbox ticking after a click genuinely is a separate effect.
func (v *Verifier) changeEvidence(target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) []directorapi.Evidence {

	var ev []directorapi.Evidence

	// A menu opening is the clearest signal there is, and it is what clicking a menu item
	// does — the case that motivated putting menu detection first.
	if opened, detail := menuOpened(before, after); opened {
		ev = append(ev, directorapi.Evidence{
			Kind: "menu_opened", Observed: true, Weight: 0.9, Detail: detail,
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

	// THE APPLICATION SAYING SOMETHING DIFFERENT — a status line, a validation message, a
	// count, a filename, a mode. The commonest way software reports an outcome, and for a long
	// time not evidence at all: a click that made an application say "Saved" produced exactly
	// the verdict of one that made it say nothing.
	//
	// Graded by volume, because a response to an action touches one or two labels and an
	// application repainting touches many — the same reasoning `element_count_changed` already
	// applies to the same screen. See saidSomethingDifferent for the known limit.
	if n, detail := saidSomethingDifferent(target, before, after); n > 0 {
		weight, kind := 0.7, "text_changed"
		if n > maxTargetedTextChanges {
			weight, kind = 0.35, "text_churn"
		}
		ev = append(ev, directorapi.Evidence{
			Kind: kind, Observed: true, Weight: weight, Detail: detail,
			Source: directorapi.SourceAccessibility,
		})
	}

	// ANOTHER CONTROL CHANGING STATE BECAUSE OF WHAT WAS DONE.
	//
	// A Rename button becoming enabled once something is selected; a Save button enabling once a
	// form is valid; a checkbox clearing because a mode changed. The application responding
	// through a control other than the one that was acted on — which was invisible here, the same
	// way a changed label was until it was added.
	//
	// `targetStateChanged` has read exactly these three facts about the TARGET all along; this is
	// the same question asked of its neighbours, excluding the target so one fact cannot be
	// counted twice. Graded by volume for the reason everything else here is.
	if n, detail := stateChangedNearby(target, before, after); n > 0 {
		weight, kind := 0.7, "state_changed"
		if n > maxTargetedTextChanges {
			weight, kind = 0.35, "state_churn"
		}
		ev = append(ev, directorapi.Evidence{
			Kind: kind, Observed: true, Weight: weight, Detail: detail,
			Source: directorapi.SourceAccessibility,
		})
	}

	// The target disappearing — a dialog dismissed by its own OK button.
	if _, wasThere := before.Element(target.ElementID); wasThere {
		if _, stillThere := after.Element(target.ElementID); !stillThere {
			ev = append(ev, directorapi.Evidence{
				Kind: "target_gone", Observed: true, Weight: 0.7,
				Detail: "the element that was acted on is no longer present",
				Source: directorapi.SourceAccessibility,
			})
		}
	}

	// The weakest signal, and deliberately weak: the element set changed. Suggestive on its own
	// and corroborating alongside anything else, but a busy application changes its tree
	// constantly for reasons unrelated to the action.
	if d := len(after.Elements) - len(before.Elements); d != 0 {
		ev = append(ev, directorapi.Evidence{
			Kind: "element_count_changed", Observed: true, Weight: 0.35,
			Detail: fmt.Sprintf("the number of elements changed by %+d", d),
			Source: directorapi.SourceAccessibility,
		})
	}
	return ev
}

// stateChangedNearby counts controls OTHER than the target whose reported state changed.
//
// The same three facts `targetStateChanged` reads — selected, value, enabled — asked of the
// target's neighbours. A control becoming enabled is one of the clearest things an application can
// say about an action: it is why a file manager's Rename lights up once something is chosen.
//
// Scoped to the target's own window, and the target itself excluded, for the reasons
// `saidSomethingDifferent` gives: another application responding is not this one answering, and
// one fact must not carry two pieces of evidence.
func stateChangedNearby(target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) (int, string) {

	window := target.WindowID
	if window == "" {
		if el, ok := after.Element(target.ElementID); ok {
			window = el.WindowID
		}
	}

	n := 0
	var named []string
	for id := range after.Elements {
		if id == target.ElementID {
			continue
		}
		if now, ok := after.Element(id); !ok || (window != "" && now.WindowID != window) {
			continue
		}
		if _, ok := before.Element(id); !ok {
			continue
		}
		changed, detail := targetStateChanged(id, before, after)
		if !changed {
			continue
		}
		n++
		if len(named) < 2 {
			named = append(named, detail)
		}
	}
	if n == 0 {
		return 0, ""
	}
	if n > len(named) {
		return n, fmt.Sprintf("%d controls changed state, including %s", n,
			strings.Join(named, " and "))
	}
	return n, strings.Join(named, " and ")
}
