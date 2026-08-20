package uiact

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The ladders: for each verb, the acceptable implementations in preference order.
//
// Every ladder ends in refusal, and that is the design rather than a formality. A
// ladder whose last rung was "click somewhere sensible" would make every unsupported
// verb succeed at doing something nobody asked for — which is worse than refusing,
// because a refusal can be reported and a wrong click cannot be taken back.
//
// The preference order is the same argument every time: prefer the implementation that
// cannot miss and can report failure, over the one that can do neither. A pattern call
// is the application performing the operation. A click is a guess about geometry that
// succeeds identically whether it hit the control or the thing behind it.

// requirement helpers. Each returns the predicate AND the sentence explaining a
// rejection, so an unavailable rung always says what was missing rather than "not
// available".

// needsPattern requires the accessibility source to expose a control by id and, when
// the provider reports patterns at all, to report this one.
func needsPattern(pattern string) func(Target) (bool, string) {
	return func(t Target) (bool, string) {
		if t.NativeID == "" {
			return false, "the accessibility source does not expose this control by id"
		}
		if !t.Supports(pattern) {
			return false, "the control does not implement the " + pattern + " pattern"
		}
		return true, ""
	}
}

// needsInvokable requires an id and, where known, the invoke pattern.
func needsInvokable(t Target) (bool, string) {
	if t.NativeID == "" {
		return false, "the accessibility source does not expose this control by id"
	}
	if !t.Supports(PatternInvoke) {
		return false, "the control does not implement the invoke pattern"
	}
	return true, ""
}

// needsBounds requires somewhere real to aim.
func needsBounds(t Target) (bool, string) {
	if !t.HasBounds {
		return false, "the control has no on-screen rectangle to aim at"
	}
	if t.Offscreen {
		return false, "the control is scrolled out of view"
	}
	return true, ""
}

// needsWindow requires a window for a window-level verb.
func needsWindow(t Target) (bool, string) {
	if t.WindowID == "" {
		return false, "no window was identified"
	}
	return true, ""
}

// pattern builds the top rung of a structural ladder.
func pattern(name, detail string) Rung {
	return Rung{Mechanism: MechPattern, Detail: detail, available: needsPattern(name)}
}

func invokeRung(detail string) Rung {
	return Rung{Mechanism: MechInvoke, Detail: detail, available: needsInvokable}
}

func clickRung(detail string) Rung {
	return Rung{Mechanism: MechClick, Detail: detail, available: needsBounds}
}

func keyRung(chord, detail string) Rung {
	return Rung{Mechanism: MechKeyboard, Detail: detail, Chord: chord}
}

func refuseRung() Rung {
	return Rung{Mechanism: MechRefuse, Detail: "refuse rather than approximate"}
}

// Ladder returns the ordered implementations for a verb.
//
// A function over a map so each ladder can be read as a paragraph, and so a verb that
// is in the vocabulary but has no ladder returns the bare refusal rather than an empty
// slice a caller might read as "anything goes".
func Ladder(kind directorapi.SemanticActionKind) []Rung {
	switch kind {

	// ── structural, element-directed ─────────────────────────────────────────

	case directorapi.SemanticExpand:
		return []Rung{
			pattern(PatternExpandCollapse, "ask the control to expand through its ExpandCollapse pattern"),
			invokeRung("activate the control, which expands it in most tree implementations"),
			clickRung("click the disclosure arrow"),
			refuseRung(),
		}
	case directorapi.SemanticCollapse:
		return []Rung{
			pattern(PatternExpandCollapse, "ask the control to collapse through its ExpandCollapse pattern"),
			invokeRung("activate the control, which collapses an expanded node in most trees"),
			clickRung("click the disclosure arrow"),
			refuseRung(),
		}
	case directorapi.SemanticToggle:
		return []Rung{
			pattern(PatternToggle, "flip the control through its Toggle pattern"),
			invokeRung("activate the control, which flips a checkbox or a toggle button"),
			clickRung("click the control"),
			refuseRung(),
		}
	case directorapi.SemanticCheck, directorapi.SemanticUncheck:
		// Reached only when the control is NOT already in the requested state — see
		// alreadySatisfied — so flipping it is exactly right. When the state could not
		// be read at all, flipping is still the best available answer and the
		// verification is what catches a wrong guess.
		return []Rung{
			pattern(PatternToggle, "flip the control through its Toggle pattern"),
			invokeRung("activate the control"),
			clickRung("click the control"),
			refuseRung(),
		}
	case directorapi.SemanticSelect:
		return []Rung{
			pattern(PatternSelectionItem, "select the item through its SelectionItem pattern"),
			clickRung("click the item, which selects it in a list, a tree or a tab strip"),
			refuseRung(),
		}
	case directorapi.SemanticDeselect:
		// No click rung, deliberately. A plain click on an item SELECTS it, and
		// ctrl+click deselecting is a convention rather than a guarantee — an
		// application that does not honour it would leave the item selected while the
		// Director reported a deselection.
		return []Rung{
			pattern(PatternSelectionItem, "remove the item from the selection through its SelectionItem pattern"),
			refuseRung(),
		}
	case directorapi.SemanticInvoke:
		return []Rung{
			invokeRung("activate the control through its Invoke pattern"),
			clickRung("click the control"),
			refuseRung(),
		}
	case directorapi.SemanticOpen:
		return []Rung{
			invokeRung("activate the control, which opens what it represents"),
			{Mechanism: MechDoubleClick, Detail: "double click the item", available: needsBounds},
			refuseRung(),
		}
	case directorapi.SemanticChoose:
		return []Rung{
			pattern(PatternSelectionItem, "choose the option through its SelectionItem pattern"),
			invokeRung("activate the option, which is how a menu entry is chosen"),
			clickRung("click the option"),
			refuseRung(),
		}
	case directorapi.SemanticShowContextMenu:
		// The keyboard rung comes FIRST even though a right click is the gesture
		// everybody pictures: Shift+F10 goes to the focused control with no geometry
		// involved, so it cannot open the context menu of whatever happens to be
		// underneath. It needs focus, which the lowering arranges.
		return []Rung{
			{Mechanism: MechKeyboard, Chord: "shift+f10",
				Detail:    "focus the control and press Shift+F10",
				available: func(t Target) (bool, string) { return needsPattern(PatternInvoke)(t) }},
			{Mechanism: MechRightClick, Detail: "right click the control", available: needsBounds},
			refuseRung(),
		}
	case directorapi.SemanticScrollHere:
		// One rung and a refusal. Wheel notches would scroll SOMETHING by an amount
		// nobody chose, and "scroll to the item" is not satisfied by "the view moved".
		return []Rung{
			pattern(PatternScrollItem, "ask the container to bring the item into view"),
			refuseRung(),
		}

	// ── selection and clipboard ──────────────────────────────────────────────

	case directorapi.SemanticSelectAll:
		return []Rung{keyRung("ctrl+a", "select everything in the focused control"), refuseRung()}
	case directorapi.SemanticCopy:
		return []Rung{keyRung("ctrl+c", "copy the selection"), refuseRung()}
	case directorapi.SemanticCut:
		return []Rung{keyRung("ctrl+x", "cut the selection"), refuseRung()}
	case directorapi.SemanticPaste:
		return []Rung{keyRung("ctrl+v", "paste the clipboard"), refuseRung()}

	// ── history ──────────────────────────────────────────────────────────────

	case directorapi.SemanticUndo:
		return []Rung{keyRung("ctrl+z", "undo the last change"), refuseRung()}
	case directorapi.SemanticRedo:
		// Ctrl+Y is the other convention and there is no way to tell from outside which
		// an application uses. Ctrl+Shift+Z is the more widely honoured of the two, and
		// the verification catches the case where nothing happened — which is the whole
		// reason a guess is tolerable here and not elsewhere.
		return []Rung{keyRung("ctrl+shift+z", "redo the last undone change"), refuseRung()}

	// ── navigation ───────────────────────────────────────────────────────────

	case directorapi.SemanticBack:
		return []Rung{keyRung("alt+left", "go back in the application's history"), refuseRung()}
	case directorapi.SemanticForward:
		return []Rung{keyRung("alt+right", "go forward in the application's history"), refuseRung()}
	case directorapi.SemanticNext:
		return []Rung{keyRung("ctrl+tab", "move to the next item"), refuseRung()}
	case directorapi.SemanticPrevious:
		return []Rung{keyRung("ctrl+shift+tab", "move to the previous item"), refuseRung()}
	case directorapi.SemanticRefresh:
		return []Rung{keyRung("f5", "reload the current view"), refuseRung()}

	// ── commitment ───────────────────────────────────────────────────────────
	// Each prefers INVOKING a named control when the user named one — "confirm" with
	// the OK button resolved is an invoke, not a guess about which key the dialog
	// treats as its default.

	case directorapi.SemanticSubmit:
		return []Rung{
			invokeRung("activate the submit control"),
			clickRung("click the submit control"),
			keyRung("enter", "press Enter, which commits the focused field"),
			refuseRung(),
		}
	case directorapi.SemanticConfirm:
		return []Rung{
			invokeRung("activate the confirming control"),
			clickRung("click the confirming control"),
			keyRung("enter", "press Enter, which activates a dialog's default button"),
			refuseRung(),
		}
	case directorapi.SemanticCancel:
		return []Rung{
			invokeRung("activate the cancelling control"),
			clickRung("click the cancelling control"),
			keyRung("escape", "press Escape, which cancels a dialog"),
			refuseRung(),
		}
	case directorapi.SemanticDismiss:
		return []Rung{
			invokeRung("activate the dismissing control"),
			keyRung("escape", "press Escape, which dismisses a transient popup"),
			refuseRung(),
		}
	case directorapi.SemanticClose:
		return []Rung{
			invokeRung("activate the close control"),
			clickRung("click the close control"),
			keyRung("ctrl+w", "close the current tab or document"),
			refuseRung(),
		}

	// ── window state ─────────────────────────────────────────────────────────

	case directorapi.SemanticMaximize:
		return []Rung{
			{Mechanism: MechWindowState, Detail: "ask the window manager to maximize it", available: needsWindow},
			refuseRung(),
		}
	case directorapi.SemanticMinimize:
		return []Rung{
			{Mechanism: MechWindowState, Detail: "ask the window manager to minimize it", available: needsWindow},
			refuseRung(),
		}
	case directorapi.SemanticRestore:
		return []Rung{
			{Mechanism: MechWindowState, Detail: "ask the window manager to restore it", available: needsWindow},
			refuseRung(),
		}

	// ── pinning ──────────────────────────────────────────────────────────────
	// Honestly thin. There is no pin pattern: pinning is a toggle on some controls, a
	// context-menu entry on most, and nothing at all on the rest. The ladder offers
	// what genuinely exists and refuses otherwise, rather than inventing a right-click
	// plus a guess at a menu item's label.

	case directorapi.SemanticPin, directorapi.SemanticUnpin:
		return []Rung{
			pattern(PatternToggle, "flip the pin control through its Toggle pattern"),
			invokeRung("activate the pin control"),
			refuseRung(),
		}
	}

	return []Rung{refuseRung()}
}

// WindowState is the state a window-level verb asks for.
//
// Exposed because the lowering needs it and because `director actions` prints it. The
// bool distinguishes "not a window verb" from a state that happens to be empty.
func WindowState(kind directorapi.SemanticActionKind) (directorapi.WindowState, bool) {
	switch kind {
	case directorapi.SemanticMaximize:
		return directorapi.WindowMaximized, true
	case directorapi.SemanticMinimize:
		return directorapi.WindowMinimized, true
	case directorapi.SemanticRestore:
		return directorapi.WindowNormal, true
	}
	return "", false
}

// Describe renders a whole ladder for `director actions`.
func Describe(kind directorapi.SemanticActionKind) []string {
	rungs := Ladder(kind)
	out := make([]string, 0, len(rungs))
	for i, r := range rungs {
		line := fmt.Sprintf("%d. %s", i+1, r.Detail)
		if r.Chord != "" {
			line += " (" + r.Chord + ")"
		}
		out = append(out, line)
	}
	return out
}
