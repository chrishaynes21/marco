package directorapi

import "strings"

// Semantic actions: the vocabulary the Director's reasoning lowers INTO.
//
// The governing rule:
//
//	Marco owns mechanics. Director owns semantics.
//
// The gap this closes is narrow and specific. The Director could already perceive
// durable targets and execute verified Marco, but its action words were so few that
// ordinary requests decomposed into artificial sequences: "expand the folder" became
// "click the little triangle at (921,381)", which is not what the user said, cannot be
// verified as an expansion, and replays as a coordinate.
//
// A semantic action names WHAT THE USER MEANT. It carries no keyboard shortcut, no
// pattern name and no coordinate — those are implementations, chosen later against the
// control's real capabilities by the ladder in internal/director/uiact. A kind that
// encoded "ctrl+z" would have thrown that choice away before anyone could make it, in
// exactly the way the editing milestone found for text.
//
// Why a separate enum from ActionType. ActionType classifies the SHAPE of a plan step,
// and each of its values has a concrete struct behind it (ClickAction, TypeAction…).
// Thirty more ActionTypes with no structs would break that correspondence and force
// every switch in the system to grow thirty arms. One SemanticAction carrying a
// SemanticActionKind keeps the plan shape stable and puts the vocabulary where it can
// grow — which it will, because this list is not the last word on what a person can
// mean.

// SemanticActionKind names one thing a user can mean.
//
// String-typed, like ActionType and for the same reason: plans serialise readably, and
// a skill can contribute a verb without a Go enum change.
type SemanticActionKind string

const (
	// ── Element-directed structural actions ──────────────────────────────────
	// Each names a change to ONE control, and each is verified by that control's own
	// state rather than by "something on screen moved".

	// SemanticInvoke activates a control — the semantic form of pressing it. Distinct
	// from a click, which is one possible implementation: a button with an Invoke
	// pattern is pressed without the pointer going anywhere near it.
	SemanticInvoke SemanticActionKind = "invoke"
	// SemanticSelect makes a control the selected one among its siblings — a tab, a
	// row, a list item. NOT the same as invoking it: selecting a file highlights it,
	// invoking it opens it, and conflating them is how "select the file" deletes an
	// afternoon.
	SemanticSelect SemanticActionKind = "select"
	// SemanticDeselect removes a control from the selection.
	SemanticDeselect SemanticActionKind = "deselect"
	// SemanticExpand opens a collapsible control — a tree node, a disclosure, a
	// combo box's list.
	SemanticExpand SemanticActionKind = "expand"
	// SemanticCollapse closes one.
	SemanticCollapse SemanticActionKind = "collapse"
	// SemanticToggle flips a two-state control WITHOUT knowing which state it is in.
	// Kept distinct from Check/Uncheck because it is a different request: "toggle the
	// sidebar" is satisfied by either outcome, and "check the box" is not.
	SemanticToggle SemanticActionKind = "toggle"
	// SemanticCheck puts a checkbox into its checked state, and is already satisfied
	// if it is there. The idempotence is the point — a Toggle used for this would
	// UNCHECK an already-checked box, which is the opposite of what was asked.
	SemanticCheck SemanticActionKind = "check"
	// SemanticUncheck is its complement.
	SemanticUncheck SemanticActionKind = "uncheck"
	// SemanticChoose picks one option from an enumerated set — a menu entry, a combo
	// box option, the second search result.
	SemanticChoose SemanticActionKind = "choose"
	// SemanticOpen opens what a control represents: a folder, a file, a document.
	// Frequently an invoke, sometimes a double-click, and the difference is the
	// application's business rather than the user's.
	SemanticOpen SemanticActionKind = "open"
	// SemanticClose closes what a control represents — a tab, a document, a panel.
	// NOT SemanticDismiss: closing a tab is ordinary, dismissing a dialog answers a
	// question that was being asked.
	SemanticClose SemanticActionKind = "close"
	// SemanticShowContextMenu opens a control's context menu.
	SemanticShowContextMenu SemanticActionKind = "show_context_menu"
	// SemanticScrollHere brings a control into view. The one action whose success is
	// entirely about visibility, and the reason an offscreen element is a legitimate
	// target rather than a missing one.
	SemanticScrollHere SemanticActionKind = "scroll_here"

	// ── Selection and clipboard ──────────────────────────────────────────────

	// SemanticSelectAll selects everything in the focused context.
	SemanticSelectAll SemanticActionKind = "select_all"
	// SemanticCopy copies the selection.
	SemanticCopy SemanticActionKind = "copy"
	// SemanticCut cuts it. Destructive in a way Copy is not: the source loses it.
	SemanticCut SemanticActionKind = "cut"
	// SemanticPaste pastes the clipboard.
	SemanticPaste SemanticActionKind = "paste"

	// ── History ──────────────────────────────────────────────────────────────

	// SemanticUndo reverses the last change.
	SemanticUndo SemanticActionKind = "undo"
	// SemanticRedo reapplies it.
	SemanticRedo SemanticActionKind = "redo"

	// ── Navigation ───────────────────────────────────────────────────────────

	// SemanticBack goes back in whatever history the application keeps.
	SemanticBack SemanticActionKind = "back"
	// SemanticForward goes forward.
	SemanticForward SemanticActionKind = "forward"
	// SemanticNext moves to the next item in an ordered set.
	SemanticNext SemanticActionKind = "next"
	// SemanticPrevious moves to the previous one.
	SemanticPrevious SemanticActionKind = "previous"
	// SemanticRefresh reloads the current view.
	SemanticRefresh SemanticActionKind = "refresh"

	// ── Commitment ───────────────────────────────────────────────────────────
	// The three that decide something. All are high risk by default: see Risk.

	// SemanticSubmit commits a form or a field.
	SemanticSubmit SemanticActionKind = "submit"
	// SemanticConfirm answers an affirmative prompt — OK, Yes, Accept.
	SemanticConfirm SemanticActionKind = "confirm"
	// SemanticCancel answers a negative one. Distinct from Dismiss: cancelling
	// ANSWERS the dialog, and an application may act on the answer.
	SemanticCancel SemanticActionKind = "cancel"
	// SemanticDismiss makes a transient thing go away — a popup, a toast, a
	// notification — without answering anything.
	SemanticDismiss SemanticActionKind = "dismiss"

	// ── Window state ─────────────────────────────────────────────────────────

	SemanticMaximize SemanticActionKind = "maximize"
	SemanticMinimize SemanticActionKind = "minimize"
	SemanticRestore  SemanticActionKind = "restore"

	// ── Pinning ──────────────────────────────────────────────────────────────

	// SemanticPin and SemanticUnpin pin a control (a tab, a taskbar item, a panel).
	SemanticPin   SemanticActionKind = "pin"
	SemanticUnpin SemanticActionKind = "unpin"
)

// SemanticVocabulary is every kind, in the order this file declares them.
//
// One list rather than a scatter of switch arms, so `director actions` and the
// completeness tests read from the same place the ladder does. A kind that is in the
// vocabulary and has no ladder is a build-time hole the tests close.
var SemanticVocabulary = []SemanticActionKind{
	SemanticInvoke, SemanticSelect, SemanticDeselect, SemanticExpand, SemanticCollapse,
	SemanticToggle, SemanticCheck, SemanticUncheck, SemanticChoose, SemanticOpen,
	SemanticClose, SemanticShowContextMenu, SemanticScrollHere,
	SemanticSelectAll, SemanticCopy, SemanticCut, SemanticPaste,
	SemanticUndo, SemanticRedo,
	SemanticBack, SemanticForward, SemanticNext, SemanticPrevious, SemanticRefresh,
	SemanticSubmit, SemanticConfirm, SemanticCancel, SemanticDismiss,
	SemanticMaximize, SemanticMinimize, SemanticRestore,
	SemanticPin, SemanticUnpin,
}

// Known reports whether a kind is in the vocabulary.
//
// Used before planning and before lowering. An unknown verb is REFUSED rather than
// approximated with a click, for the reason the whole milestone exists: a click that
// stands in for an unimplemented verb does something, reports success, and is wrong.
func (k SemanticActionKind) Known() bool {
	for _, v := range SemanticVocabulary {
		if v == k {
			return true
		}
	}
	return false
}

// NeedsTarget reports whether this kind is meaningless without a control to act on.
//
// The division is not cosmetic: a kind that needs a target and has none must ask,
// while a kind that does not is addressed to the focused context and asking would be
// noise. "Undo" means the thing the user is in; "expand" means nothing at all until
// something is named.
func (k SemanticActionKind) NeedsTarget() bool {
	switch k {
	case SemanticInvoke, SemanticSelect, SemanticDeselect, SemanticExpand,
		SemanticCollapse, SemanticToggle, SemanticCheck, SemanticUncheck,
		SemanticChoose, SemanticOpen, SemanticShowContextMenu, SemanticScrollHere,
		SemanticPin, SemanticUnpin:
		return true
	}
	// Close is deliberately NOT here. "Close" with a target closes that tab; "close"
	// alone closes the active window, and both are things people say.
	return false
}

// WindowLevel reports whether this kind acts on a WINDOW rather than on a control.
//
// These lower through OS's WindowState rather than through the accessibility surface,
// and they are the reason the foreground guard treats window operations differently:
// minimising a window is not typing into it.
func (k SemanticActionKind) WindowLevel() bool {
	switch k {
	case SemanticMaximize, SemanticMinimize, SemanticRestore:
		return true
	}
	return false
}

// Describe renders the kind as a phrase for a person.
func (k SemanticActionKind) Describe() string {
	if s, ok := semanticPhrases[k]; ok {
		return s
	}
	return strings.ReplaceAll(string(k), "_", " ")
}

var semanticPhrases = map[SemanticActionKind]string{
	SemanticInvoke:          "activate",
	SemanticSelect:          "select",
	SemanticDeselect:        "deselect",
	SemanticExpand:          "expand",
	SemanticCollapse:        "collapse",
	SemanticToggle:          "toggle",
	SemanticCheck:           "check",
	SemanticUncheck:         "uncheck",
	SemanticChoose:          "choose",
	SemanticOpen:            "open",
	SemanticClose:           "close",
	SemanticShowContextMenu: "show the context menu for",
	SemanticScrollHere:      "scroll to",
	SemanticSelectAll:       "select everything",
	SemanticCopy:            "copy",
	SemanticCut:             "cut",
	SemanticPaste:           "paste",
	SemanticUndo:            "undo the last change",
	SemanticRedo:            "redo",
	SemanticBack:            "go back",
	SemanticForward:         "go forward",
	SemanticNext:            "go to the next one",
	SemanticPrevious:        "go to the previous one",
	SemanticRefresh:         "refresh",
	SemanticSubmit:          "submit",
	SemanticConfirm:         "confirm",
	SemanticCancel:          "cancel",
	SemanticDismiss:         "dismiss",
	SemanticMaximize:        "maximize",
	SemanticMinimize:        "minimize",
	SemanticRestore:         "restore",
	SemanticPin:             "pin",
	SemanticUnpin:           "unpin",
}

// Risk is the single-action risk this verb carries, BEFORE the target is known.
//
// Semantics, not mechanics — which is the whole point of classifying here rather than
// at the primitive. A click on "Delete" is not a low-risk click, and a policy that
// reasoned about the primitive would see a mouse event and wave it through. The verb
// the user said is better evidence of consequence than the input that carries it.
//
// The target still matters and is still consulted: policy raises this further for a
// destructive-looking label. This is the floor, not the verdict.
func (k SemanticActionKind) Risk() RiskLevel {
	switch k {
	case SemanticSubmit, SemanticConfirm:
		// Committing is the archetypal irreversible act: it sends, buys, posts or
		// deletes, and which one is not knowable from the verb.
		return RiskHigh
	case SemanticCut, SemanticPaste, SemanticClose, SemanticCancel, SemanticInvoke,
		SemanticOpen, SemanticChoose, SemanticUnpin, SemanticRedo:
		// Each does something the user would notice and might not be able to take
		// back. Cancel is here rather than with Dismiss because it ANSWERS a dialog,
		// and an application acts on the answer — "cancel" during an install is not a
		// no-op. Close can discard unsaved work.
		return RiskMedium
	}
	// Everything else changes what is shown, what is selected, or what has focus.
	// Reversible by doing the opposite, and observable the moment it happens.
	return RiskLow
}

// Reversible reports whether an ordinary opposite action undoes this one.
//
// Consulted by policy for bulk and by explain. Deliberately conservative: an action is
// reversible only when the Director can NAME the inverse, because "probably fine"
// reversibility is what makes a bulk operation feel safe while it is not.
func (k SemanticActionKind) Reversible() bool {
	switch k {
	case SemanticSelect, SemanticDeselect, SemanticExpand, SemanticCollapse,
		SemanticToggle, SemanticCheck, SemanticUncheck, SemanticSelectAll,
		SemanticCopy, SemanticUndo, SemanticBack, SemanticForward,
		SemanticNext, SemanticPrevious, SemanticScrollHere,
		SemanticMaximize, SemanticMinimize, SemanticRestore,
		SemanticPin, SemanticUnpin, SemanticShowContextMenu, SemanticDismiss:
		return true
	}
	return false
}

// SemanticAction is one semantic intention, targeted.
//
// The concrete Action the planner emits for every verb in the vocabulary. It holds the
// KIND and the TARGET and nothing about how the effect will be produced — the ladder
// decides that at execution time, against the control's real capabilities, and records
// what it chose and what it rejected.
type SemanticAction struct {
	Kind SemanticActionKind `json:"kind"`
	// Target is the control to act on. Zero for the kinds that address the focused
	// context (undo, refresh, back).
	Target ElementReference `json:"target,omitempty"`
	// Window names the window for a window-level kind. Empty means the active one.
	Window *WindowReference `json:"window,omitempty"`
	// Ordinal is which one, for Choose ("the second result"). 0 means unspecified.
	Ordinal int `json:"ordinal,omitempty"`
}

func (SemanticAction) ActionType() ActionType { return ActionSemantic }

// Describe renders the action for a log, a confirmation or a trace.
func (a SemanticAction) Describe() string {
	phrase := a.Kind.Describe()
	if a.Kind.WindowLevel() {
		if a.Window != nil && a.Window.Description != "" {
			return phrase + " " + a.Window.Description
		}
		return phrase + " the window"
	}
	if !a.Targeted() {
		return phrase
	}
	return phrase + " " + describeTarget(a.Target)
}

// Targeted reports whether this action carries a usable target.
func (a SemanticAction) Targeted() bool { return a.Target.Resolvable() }
