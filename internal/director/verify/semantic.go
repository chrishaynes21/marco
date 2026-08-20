package verify

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Verification for the semantic action vocabulary.
//
//	Verification remains evidence-driven.
//	Verification must never rely solely on visual change.
//
// The second rule is the one that shapes this file. A region of the screen changing
// after an action is consistent with the action having worked and equally consistent
// with an unrelated repaint — a caret blinking, a clock ticking, a spinner spinning. It
// is corroboration and never a verdict, so nothing here concludes from it alone.
//
// What each verb is verified BY is its own semantics, not a generic "something moved":
// an expand is proved by the node reporting itself expanded or by its children
// appearing, a dismiss by the thing being gone, a back by the view having changed to a
// different one. That is the difference between "the Director did something" and "the
// Director did what was asked".

// verifySemantic judges one semantic action.
//
// A satisfied action — the control was already in the requested state — never reaches
// here: there is no change to observe, and demanding one would report a correct outcome
// as a failure. See the executor.
func (v *Verifier) verifySemantic(a directorapi.SemanticAction, target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	switch a.Kind {
	case directorapi.SemanticExpand:
		return v.verifyExpansion(a, target, before, after, true)
	case directorapi.SemanticCollapse:
		return v.verifyExpansion(a, target, before, after, false)

	case directorapi.SemanticSelect, directorapi.SemanticChoose:
		return v.verifySelection(target, before, after, true)
	case directorapi.SemanticDeselect:
		return v.verifySelection(target, before, after, false)

	case directorapi.SemanticCheck:
		return v.verifyChecked(target, before, after, true)
	case directorapi.SemanticUncheck:
		return v.verifyChecked(target, before, after, false)
	case directorapi.SemanticToggle, directorapi.SemanticPin, directorapi.SemanticUnpin:
		return v.verifyFlipped(target, before, after)

	case directorapi.SemanticDismiss, directorapi.SemanticCancel, directorapi.SemanticClose:
		return v.verifyGone(a, target, before, after)

	case directorapi.SemanticBack, directorapi.SemanticForward, directorapi.SemanticRefresh:
		return v.verifyNavigation(a, before, after)

	case directorapi.SemanticUndo, directorapi.SemanticRedo:
		return v.verifyDocumentChanged(a, before, after)

	case directorapi.SemanticMaximize, directorapi.SemanticMinimize, directorapi.SemanticRestore:
		return v.verifyWindowState(a, target, before, after)

	case directorapi.SemanticShowContextMenu:
		return v.verifyContextMenu(before, after)

	case directorapi.SemanticScrollHere:
		return v.verifyScrolledIntoView(target, before, after)
	}

	// Everything else — invoke, open, submit, confirm, paste, copy, next — has an
	// effect the Director cannot name in advance, exactly like a click. It is verified
	// the same way: gather what plausibly follows and take the strongest, rather than
	// demanding a specific outcome that would be wrong for most controls.
	return v.verifyOpenEnded(a, target, before, after)
}

// verifyExpansion proves an expand or a collapse.
//
// Two independent proofs, and either is enough. The node's own reported state is the
// strongest — it is the application answering the exact question. Its CHILDREN
// appearing or disappearing is nearly as strong and is what carries applications whose
// providers never report the state at all, which in practice is most trees.
func (v *Verifier) verifyExpansion(a directorapi.SemanticAction, target directorapi.ResolvedTarget,
	before, after directorapi.WorldState, want bool) directorapi.VerificationResult {

	word := map[bool]string{true: "expanded", false: "collapsed"}[want]
	var ev []directorapi.Evidence

	wasState, hadBefore := expandedState(before, target.ElementID)
	isState, hasAfter := expandedState(after, target.ElementID)
	switch {
	case hasAfter && isState == want && (!hadBefore || wasState != want):
		ev = append(ev, directorapi.Evidence{
			Kind: "expanded_state_changed", Observed: true, Weight: 0.95,
			Detail: fmt.Sprintf("%q now reports itself %s", target.Label, word),
			Source: directorapi.SourceAccessibility,
		})
	case hasAfter && isState != want:
		// The application is answering the exact question and the answer is no. That
		// outranks any amount of corroborating movement: a tree that says it is still
		// collapsed was not expanded, whatever else changed on screen.
		return directorapi.VerificationResult{
			Success: false, Confidence: 0.9,
			Evidence: []directorapi.Evidence{{
				Kind: "expanded_state_unchanged", Observed: true, Weight: 0.9,
				Detail: fmt.Sprintf("%q still reports itself %s", target.Label,
					map[bool]string{true: "expanded", false: "collapsed"}[isState]),
				Source: directorapi.SourceAccessibility,
			}},
			Reason: "the control reports it was not " + word,
		}
	}

	// Children arriving or leaving. Counted under the target specifically rather than
	// across the window, so a notification appearing elsewhere cannot stand in for a
	// tree node that did nothing.
	kidsBefore, kidsAfter := childCount(before, target.ElementID), childCount(after, target.ElementID)
	switch {
	case want && kidsAfter > kidsBefore:
		ev = append(ev, directorapi.Evidence{
			Kind: "children_appeared", Observed: true, Weight: 0.85,
			Detail: fmt.Sprintf("%d child element(s) appeared under %q (%d → %d)",
				kidsAfter-kidsBefore, target.Label, kidsBefore, kidsAfter),
			Source: directorapi.SourceAccessibility,
		})
	case !want && kidsAfter < kidsBefore:
		ev = append(ev, directorapi.Evidence{
			Kind: "children_disappeared", Observed: true, Weight: 0.85,
			Detail: fmt.Sprintf("%d child element(s) disappeared from under %q (%d → %d)",
				kidsBefore-kidsAfter, target.Label, kidsBefore, kidsAfter),
			Source: directorapi.SourceAccessibility,
		})
	}

	if len(ev) == 0 {
		return directorapi.VerificationResult{
			Success: false,
			Evidence: []directorapi.Evidence{{
				Kind: "no_expansion_evidence", Observed: true, Weight: 1,
				Detail: fmt.Sprintf("%q neither reports itself %s nor changed its children",
					target.Label, word),
			}},
			Reason: "nothing showed the control was " + word,
		}
	}
	return conclude(ev, v.MinConfidence, "the control was not observably "+word)
}

// verifySelection proves a select or a deselect.
func (v *Verifier) verifySelection(target directorapi.ResolvedTarget,
	before, after directorapi.WorldState, want bool) directorapi.VerificationResult {

	el, ok := after.Element(target.ElementID)
	if !ok {
		return directorapi.VerificationResult{
			Inconclusive: true,
			Reason: fmt.Sprintf("%q is no longer in the world, so its selection could not be read",
				target.Label),
		}
	}
	if el.Selected == want {
		wasSelected := false
		if b, ok := before.Element(target.ElementID); ok {
			wasSelected = b.Selected
		}
		weight := 0.95
		detail := fmt.Sprintf("%q is now %s", target.Label, selectedWord(want))
		if wasSelected == want {
			// It already held the requested state before the action. That is not proof
			// the action did anything, and reporting it as such would verify a no-op.
			weight, detail = 0.5, fmt.Sprintf("%q is %s, as it already was",
				target.Label, selectedWord(want))
		}
		return conclude([]directorapi.Evidence{{
			Kind: "selection_state", Observed: true, Weight: weight, Detail: detail,
			Source: directorapi.SourceAccessibility,
		}}, v.MinConfidence, "the selection did not change")
	}
	return directorapi.VerificationResult{
		Success: false, Confidence: 0.9,
		Evidence: []directorapi.Evidence{{
			Kind: "selection_unchanged", Observed: true, Weight: 0.9,
			Detail: fmt.Sprintf("%q is %s", target.Label, selectedWord(el.Selected)),
			Source: directorapi.SourceAccessibility,
		}},
		Reason: "the control is not " + selectedWord(want),
	}
}

// verifyChecked proves a check or an uncheck.
func (v *Verifier) verifyChecked(target directorapi.ResolvedTarget,
	before, after directorapi.WorldState, want bool) directorapi.VerificationResult {

	is, known := checkedState(after, target.ElementID)
	if !known {
		// The provider does not report the state, so the request cannot be proved from
		// structure. Inconclusive rather than failed: the box may well be ticked, and
		// claiming it is not would be as wrong as claiming it is.
		return directorapi.VerificationResult{
			Inconclusive: true,
			Reason: fmt.Sprintf("%q does not report a checked state, so the result could not be read",
				target.Label),
		}
	}
	word := map[bool]string{true: "checked", false: "unchecked"}[want]
	if is == want {
		return conclude([]directorapi.Evidence{{
			Kind: "checked_state", Observed: true, Weight: 0.95,
			Detail: fmt.Sprintf("%q is now %s", target.Label, word),
			Source: directorapi.SourceAccessibility,
		}}, v.MinConfidence, "the control is not "+word)
	}
	return directorapi.VerificationResult{
		Success: false, Confidence: 0.9,
		Evidence: []directorapi.Evidence{{
			Kind: "checked_state_wrong", Observed: true, Weight: 0.9,
			Detail: fmt.Sprintf("%q is not %s", target.Label, word),
			Source: directorapi.SourceAccessibility,
		}},
		Reason: "the control is not " + word,
	}
}

// verifyFlipped proves a toggle: the state is the OTHER one, whichever it was.
//
// The only verb whose success is defined by a difference rather than by a value, which
// is why it cannot share verifyChecked: "toggle" against an unknown starting state has
// nothing to compare, and saying so is more useful than guessing.
func (v *Verifier) verifyFlipped(target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	was, hadBefore := checkedState(before, target.ElementID)
	is, hasAfter := checkedState(after, target.ElementID)
	if !hadBefore || !hasAfter {
		// Fall back to any state change at all on the target — a pressed style, a
		// selection, a label change. Weaker, and honest about being weaker.
		if changed, detail := targetStateChanged(target.ElementID, before, after); changed {
			return conclude([]directorapi.Evidence{{
				Kind: "target_state_changed", Observed: true, Weight: 0.7, Detail: detail,
				Source: directorapi.SourceAccessibility,
			}}, v.MinConfidence, "the control's state did not change")
		}
		return directorapi.VerificationResult{
			Inconclusive: true,
			Reason: fmt.Sprintf("%q does not report a two-state value, so the flip could not be read",
				target.Label),
		}
	}
	if was != is {
		return conclude([]directorapi.Evidence{{
			Kind: "toggled", Observed: true, Weight: 0.95,
			Detail: fmt.Sprintf("%q changed from %v to %v", target.Label, was, is),
			Source: directorapi.SourceAccessibility,
		}}, v.MinConfidence, "the control did not flip")
	}
	return directorapi.VerificationResult{
		Success: false, Confidence: 0.9,
		Evidence: []directorapi.Evidence{{
			Kind: "not_toggled", Observed: true, Weight: 0.9,
			Detail: fmt.Sprintf("%q is still %v", target.Label, is),
			Source: directorapi.SourceAccessibility,
		}},
		Reason: "the control did not change state",
	}
}

// verifyGone proves a dismiss, a cancel or a close: the thing is not there any more.
func (v *Verifier) verifyGone(a directorapi.SemanticAction, target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	var ev []directorapi.Evidence

	// A window disappearing is the clearest form of "it closed".
	if len(after.Windows) < len(before.Windows) {
		ev = append(ev, directorapi.Evidence{
			Kind: "window_disappeared", Observed: true, Weight: 0.9,
			Detail: fmt.Sprintf("a window closed (%d → %d)", len(before.Windows), len(after.Windows)),
			Source: directorapi.SourceWindowSystem,
		})
	}
	// A dialog specifically. Counted separately from the window total because a dialog
	// closing while another window opens leaves the count unchanged.
	if wasDialog, isDialog := dialogCount(before), dialogCount(after); wasDialog > isDialog {
		ev = append(ev, directorapi.Evidence{
			Kind: "dialog_disappeared", Observed: true, Weight: 0.9,
			Detail: fmt.Sprintf("a dialog closed (%d → %d)", wasDialog, isDialog),
			Source: directorapi.SourceAccessibility,
		})
	}
	// The named control itself going.
	if target.ElementID != "" {
		if _, wasThere := before.Element(target.ElementID); wasThere {
			if _, stillThere := after.Element(target.ElementID); !stillThere {
				ev = append(ev, directorapi.Evidence{
					Kind: "target_gone", Observed: true, Weight: 0.8,
					Detail: fmt.Sprintf("%q is no longer present", target.Label),
					Source: directorapi.SourceAccessibility,
				})
			}
		}
	}

	if len(ev) == 0 {
		return directorapi.VerificationResult{
			Success: false,
			Evidence: []directorapi.Evidence{{
				Kind: "still_present", Observed: true, Weight: 1,
				Detail: "nothing closed: the same windows and dialogs are still there",
			}},
			Reason: "nothing was observably " + a.Kind.Describe() + "ed",
		}
	}
	return conclude(ev, v.MinConfidence, "nothing observably closed")
}

// verifyNavigation proves a back, a forward or a refresh.
//
// The view must have become a DIFFERENT view. A title change is the ordinary proof; a
// wholesale replacement of the element set is what a refresh looks like in an
// application whose title never changes.
func (v *Verifier) verifyNavigation(a directorapi.SemanticAction,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	var ev []directorapi.Evidence

	if bt, at := windowTitle(before), windowTitle(after); bt != "" && at != "" && bt != at {
		ev = append(ev, directorapi.Evidence{
			Kind: "window_title_changed", Observed: true, Weight: 0.85,
			Detail: fmt.Sprintf("the view changed from %q to %q", bt, at),
			Source: directorapi.SourceWindowSystem,
		})
	}
	// Content replacement: how many of the elements that were there are still there. A
	// refresh that reloads the same page keeps the title and replaces the tree, and
	// this is the signal that catches it.
	if churn, total := elementChurn(before, after); total > 0 && churn >= 0.5 {
		ev = append(ev, directorapi.Evidence{
			Kind: "content_replaced", Observed: true, Weight: 0.7,
			Detail: fmt.Sprintf("%.0f%% of the view's elements were replaced", churn*100),
			Source: directorapi.SourceAccessibility,
		})
	}

	if len(ev) == 0 {
		return directorapi.VerificationResult{
			Success: false,
			Evidence: []directorapi.Evidence{{
				Kind: "view_unchanged", Observed: true, Weight: 1,
				Detail: "the view is the same one, with the same content",
			}},
			// Stated as the honest ambiguity it is. "Back" at the start of the history
			// does nothing and reports nothing, and that is not a fault to be retried.
			Reason: "the view did not change — the application may have had nowhere to " +
				a.Kind.Describe(),
		}
	}
	return conclude(ev, v.MinConfidence, "the view did not observably change")
}

// verifyDocumentChanged proves an undo or a redo.
//
// By the CONTENT of the focused control, because that is what undo is about. An
// application with nothing to undo swallows the keystroke silently, and "the chord was
// sent" would report that as success — the same trap the editing milestone documented
// for Ctrl+Z.
func (v *Verifier) verifyDocumentChanged(a directorapi.SemanticAction,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	beforeID, afterID := focusedID(before), focusedID(after)
	if beforeID != "" && beforeID == afterID {
		b, okB := before.Element(beforeID)
		c, okA := after.Element(afterID)
		if okB && okA && b.Value != c.Value {
			return conclude([]directorapi.Evidence{{
				Kind: "document_changed", Observed: true, Weight: 0.9,
				Detail: fmt.Sprintf("the focused control's contents changed (%d → %d characters)",
					len(b.Value), len(c.Value)),
				Source: directorapi.SourceAccessibility,
			}}, v.MinConfidence, "the document did not change")
		}
	}

	if churn, total := elementChurn(before, after); total > 0 && churn >= 0.25 {
		return conclude([]directorapi.Evidence{{
			Kind: "content_changed", Observed: true, Weight: 0.6,
			Detail: fmt.Sprintf("%.0f%% of the view's elements changed", churn*100),
			Source: directorapi.SourceAccessibility,
		}}, v.MinConfidence, "the document did not change")
	}

	return directorapi.VerificationResult{
		Success: false,
		Evidence: []directorapi.Evidence{{
			Kind: "document_unchanged", Observed: true, Weight: 1,
			Detail: "the focused control's contents are unchanged",
		}},
		// The honest reading: an application with an empty undo stack behaves exactly
		// like one that ignored the chord, and the Director cannot tell them apart.
		Reason: "nothing changed — there may have been nothing to " + string(a.Kind),
	}
}

// verifyWindowState proves a maximize, minimize or restore.
func (v *Verifier) verifyWindowState(a directorapi.SemanticAction, target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	want := map[directorapi.SemanticActionKind]directorapi.WindowState{
		directorapi.SemanticMaximize: directorapi.WindowMaximized,
		directorapi.SemanticMinimize: directorapi.WindowMinimized,
		directorapi.SemanticRestore:  directorapi.WindowNormal,
	}[a.Kind]

	id := target.WindowID
	if id == "" {
		id = activeWindowID(after)
	}
	win, ok := windowByID(after, id)
	if !ok {
		return directorapi.VerificationResult{
			Inconclusive: true,
			Reason:       "the window is no longer present, so its state could not be read",
		}
	}
	got := windowStateOf(win)
	if got == want {
		return conclude([]directorapi.Evidence{{
			Kind: "window_state", Observed: true, Weight: 0.95,
			Detail: fmt.Sprintf("the window is %s", want),
			Source: directorapi.SourceWindowSystem,
		}}, v.MinConfidence, "the window did not reach the requested state")
	}
	return directorapi.VerificationResult{
		Success: false, Confidence: 0.9,
		Evidence: []directorapi.Evidence{{
			Kind: "window_state_wrong", Observed: true, Weight: 0.9,
			Detail: fmt.Sprintf("the window is %s, not %s", got, want),
			Source: directorapi.SourceWindowSystem,
		}},
		Reason: "the window is not " + string(want),
	}
}

// windowStateOf collapses the two booleans a Window carries into the state vocabulary
// the window verbs are expressed in.
//
// Minimized is checked FIRST: a window can be both minimized and maximized — the window
// manager remembers what a minimized window will restore TO — and what the user sees,
// and therefore what "minimize it" asked for, is that it is minimized.
func windowStateOf(w directorapi.Window) directorapi.WindowState {
	switch {
	case w.Minimized:
		return directorapi.WindowMinimized
	case w.Maximized:
		return directorapi.WindowMaximized
	}
	return directorapi.WindowNormal
}

// verifyContextMenu proves a context menu opened.
func (v *Verifier) verifyContextMenu(before, after directorapi.WorldState) directorapi.VerificationResult {
	if opened, detail := menuOpened(before, after); opened {
		return conclude([]directorapi.Evidence{{
			Kind: "menu_opened", Observed: true, Weight: 0.9, Detail: detail,
			Source: directorapi.SourceAccessibility,
		}}, v.MinConfidence, "no menu appeared")
	}
	return directorapi.VerificationResult{
		Success: false,
		Evidence: []directorapi.Evidence{{
			Kind: "no_menu", Observed: true, Weight: 1,
			Detail: "no menu appeared",
		}},
		Reason: "no context menu opened",
	}
}

// verifyScrolledIntoView proves the target became visible.
func (v *Verifier) verifyScrolledIntoView(target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	el, ok := after.Element(target.ElementID)
	if !ok {
		return directorapi.VerificationResult{
			Inconclusive: true,
			Reason:       "the element is no longer in the world, so its visibility could not be read",
		}
	}
	if !el.Offscreen && el.Visible {
		return conclude([]directorapi.Evidence{{
			Kind: "target_in_view", Observed: true, Weight: 0.9,
			Detail: fmt.Sprintf("%q is in view", target.Label),
			Source: directorapi.SourceAccessibility,
		}}, v.MinConfidence, "the element is still out of view")
	}
	return directorapi.VerificationResult{
		Success: false, Confidence: 0.85,
		Evidence: []directorapi.Evidence{{
			Kind: "target_still_offscreen", Observed: true, Weight: 0.85,
			Detail: fmt.Sprintf("%q is still out of view", target.Label),
			Source: directorapi.SourceAccessibility,
		}},
		Reason: "the element was not brought into view",
	}
}

// verifyOpenEnded is the verification for verbs whose effect is not knowable in
// advance — invoke, open, submit, paste, next.
//
// It reuses the click verification's evidence gathering deliberately: the question is
// identical ("did activating this do anything?"), and a second set of rules for the
// same question would drift from it.
func (v *Verifier) verifyOpenEnded(a directorapi.SemanticAction, target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	res := v.verifyClick(directorapi.ClickAction{Target: a.Target}, target, before, after)
	if res.Reason == "the click produced no observable change" {
		res.Reason = "nothing observably changed after " + a.Describe()
	}
	return res
}

// ── evidence helpers ──────────────────────────────────────────────────────────

// expandedState reads an element's reported expansion.
//
// From Attributes, for the reason uiact.stateOf gives: StateEvidence records who
// established a state, not what it is, and an absent fact must stay unknown rather than
// become false.
func expandedState(w directorapi.WorldState, id directorapi.ElementID) (bool, bool) {
	return boolAttribute(w, id, directorapi.StateExpanded)
}

func checkedState(w directorapi.WorldState, id directorapi.ElementID) (bool, bool) {
	return boolAttribute(w, id, directorapi.StateChecked)
}

func boolAttribute(w directorapi.WorldState, id directorapi.ElementID, name string) (bool, bool) {
	el, ok := w.Element(id)
	if !ok {
		return false, false
	}
	switch v := el.Attributes[name].(type) {
	case bool:
		return v, true
	case string:
		switch v {
		case "true", "expanded", "checked", "on", "1":
			return true, true
		case "false", "collapsed", "unchecked", "off", "0":
			return false, true
		}
	}
	return false, false
}

// childCount counts the elements whose parent is this one.
func childCount(w directorapi.WorldState, id directorapi.ElementID) int {
	if id == "" {
		return 0
	}
	n := 0
	for i := range w.Elements {
		if p := w.Elements[i].ParentID; p != nil && *p == id {
			n++
		}
	}
	return n
}

// dialogCount counts dialog-role elements.
func dialogCount(w directorapi.WorldState) int {
	n := 0
	for i := range w.Elements {
		if w.Elements[i].Role == directorapi.RoleDialog {
			n++
		}
	}
	return n
}

// elementChurn is the fraction of the before-state's elements that are gone, and the
// number it was measured over.
//
// A ratio rather than a count, because "40 elements changed" means something different
// in a dialog and in a file tree. The count comes back so a caller can refuse to
// conclude anything from a world with three elements in it.
func elementChurn(before, after directorapi.WorldState) (float64, int) {
	total := len(before.Elements)
	if total == 0 {
		return 0, 0
	}
	present := make(map[directorapi.ElementID]bool, len(after.Elements))
	for i := range after.Elements {
		present[after.Elements[i].ID] = true
	}
	gone := 0
	for i := range before.Elements {
		if !present[before.Elements[i].ID] {
			gone++
		}
	}
	return float64(gone) / float64(total), total
}

func activeWindowID(w directorapi.WorldState) directorapi.WindowID {
	for i := range w.Windows {
		if w.Windows[i].Focused {
			return w.Windows[i].ID
		}
	}
	return ""
}

func windowByID(w directorapi.WorldState, id directorapi.WindowID) (directorapi.Window, bool) {
	for i := range w.Windows {
		if w.Windows[i].ID == id {
			return w.Windows[i], true
		}
	}
	return directorapi.Window{}, false
}
