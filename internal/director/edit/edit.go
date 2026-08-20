// Package edit makes text manipulation a semantic capability rather than a stream of
// keystrokes.
//
// The governing rule:
//
//	The user asked the Director to change TEXT, not to press keys.
//	The Director plans text state. Marco executes the chosen editing primitive.
//	Verification proves the resulting STATE, not the emitted input.
//
// Characters are not intent. "Type hello into the search box" is a statement about what
// the box should CONTAIN; the keystrokes are one way of getting there, and frequently
// the worst one. Typing sends characters through the keyboard layout, the IME, and
// whatever the application does with each keystroke — autocomplete that rewrites the
// field on the third letter, validation that rejects an intermediate state, a dropdown
// that steals focus. A ValuePattern SetValue has none of those failure modes because
// there is no intermediate state for anything to react to.
//
// So the planner emits SetText and the execution layer picks the strongest
// implementation available on THIS control, recording which and why. A fallback that
// happened silently would be indistinguishable from a preference.
package edit

import (
	"fmt"
	"strings"
)

// OperationID names an edit operation, stable and machine-readable.
type OperationID string

const (
	OpSetText          OperationID = "set_text"
	OpAppendText       OperationID = "append_text"
	OpInsertText       OperationID = "insert_text"
	OpReplaceSelection OperationID = "replace_selection"
	OpClearText        OperationID = "clear_text"
	OpCopySelection    OperationID = "copy_selection"
	OpPasteClipboard   OperationID = "paste_clipboard"
	OpSelectAll        OperationID = "select_all"
	OpUndo             OperationID = "undo"
	OpRedo             OperationID = "redo"
	OpPressEnter       OperationID = "press_enter"
)

// Operation is a semantic edit: a statement about the text state the user wants.
//
// Deliberately tiny, and deliberately not a keyboard macro. Nothing here names a key, a
// modifier or a layout — an Operation that could say "Ctrl+A" would let the planner
// start making decisions that belong to the execution layer, where the control's actual
// capabilities are known.
type Operation interface {
	ID() OperationID
	Description() string
}

// TextOperation carries the text an operation is about.
type TextOperation interface {
	Operation
	Text() string
}

// SetText replaces a field's entire contents.
type SetText struct{ Value string }

func (o SetText) ID() OperationID { return OpSetText }
func (o SetText) Text() string    { return o.Value }
func (o SetText) Description() string {
	return fmt.Sprintf("set the value to %q", truncate(o.Value, 40))
}

// AppendText adds to the end of what is already there.
//
// Genuinely different from SetText, not sugar for it: appending requires READING the
// current value first, and a planner that expressed it as SetText(old+new) would be
// making a decision about the old value at planning time — before the field was
// observed, and possibly wrong by the time it acted.
type AppendText struct{ Value string }

func (o AppendText) ID() OperationID { return OpAppendText }
func (o AppendText) Text() string    { return o.Value }
func (o AppendText) Description() string {
	return fmt.Sprintf("append %q", truncate(o.Value, 40))
}

// InsertText inserts at the caret without disturbing the rest.
type InsertText struct{ Value string }

func (o InsertText) ID() OperationID { return OpInsertText }
func (o InsertText) Text() string    { return o.Value }
func (o InsertText) Description() string {
	return fmt.Sprintf("insert %q at the caret", truncate(o.Value, 40))
}

// ReplaceSelection replaces whatever is selected.
type ReplaceSelection struct{ Value string }

func (o ReplaceSelection) ID() OperationID { return OpReplaceSelection }
func (o ReplaceSelection) Text() string    { return o.Value }
func (o ReplaceSelection) Description() string {
	return fmt.Sprintf("replace the selection with %q", truncate(o.Value, 40))
}

// ClearText empties a field.
type ClearText struct{}

func (o ClearText) ID() OperationID     { return OpClearText }
func (o ClearText) Description() string { return "clear the field" }

// CopySelection copies the selection to the clipboard.
//
// The one operation that is SUPPOSED to change the clipboard. Everywhere else the
// clipboard is borrowed and restored; here the user asked for it, so it is left alone.
type CopySelection struct{}

func (o CopySelection) ID() OperationID     { return OpCopySelection }
func (o CopySelection) Description() string { return "copy the selection" }

// PasteClipboard pastes whatever is on the clipboard.
type PasteClipboard struct{}

func (o PasteClipboard) ID() OperationID     { return OpPasteClipboard }
func (o PasteClipboard) Description() string { return "paste the clipboard" }

// SelectAll selects a field's whole contents.
type SelectAll struct{}

func (o SelectAll) ID() OperationID     { return OpSelectAll }
func (o SelectAll) Description() string { return "select everything" }

// Undo reverses the last edit.
//
// A semantic operation, which is why it is verified against the PREVIOUS observed value
// rather than by confirming a key was pressed. An application with nothing to undo
// swallows the keystroke silently, and "Ctrl+Z was sent" would report that as success.
type Undo struct{}

func (o Undo) ID() OperationID     { return OpUndo }
func (o Undo) Description() string { return "undo the last edit" }

// Redo reapplies an undone edit.
type Redo struct{}

func (o Redo) ID() OperationID     { return OpRedo }
func (o Redo) Description() string { return "redo" }

// PressEnter commits a field.
//
// Named for what the user means — "submit this" — rather than for the key. It is the
// one operation whose implementation genuinely is a keystroke, and it is here so that
// the planner never has to reach for a key elsewhere.
type PressEnter struct{}

func (o PressEnter) ID() OperationID     { return OpPressEnter }
func (o PressEnter) Description() string { return "press Enter" }

// ── strategies ────────────────────────────────────────────────────────────────

// Strategy is HOW an operation was carried out.
type Strategy string

const (
	// StrategyValueAPI: the provider set the value directly. The strongest: no
	// keystrokes, no layout, no intermediate state for the application to react to.
	StrategyValueAPI Strategy = "value_api"
	// StrategySelectReplace: focus, select the existing value, replace it.
	StrategySelectReplace Strategy = "select_replace"
	// StrategyClipboard: put the text on the clipboard, paste, restore the clipboard.
	// Used only when typing would be wrong — long text, or characters the layout
	// cannot produce.
	StrategyClipboard Strategy = "clipboard"
	// StrategyKeystrokes: type it. The FALLBACK, never the preference.
	StrategyKeystrokes Strategy = "keystrokes"
	// StrategyNone: the operation needs no text transport at all (Undo, PressEnter).
	StrategyNone Strategy = "direct"
)

// Attempt is one strategy tried, and what came of it.
//
// Recorded even when it failed, because "the Director typed this" and "the Director
// tried to set the value, the control refused, and it typed this instead" are different
// events — and only the second explains why an autocomplete fired.
type Attempt struct {
	Strategy Strategy `json:"strategy"`
	// Chosen is whether this strategy was the one that ran to completion.
	Chosen bool `json:"chosen"`
	// Reason explains why it was skipped, refused, or selected. Every fallback is
	// explainable; a silent one is indistinguishable from a preference.
	Reason string `json:"reason"`
	// Err is set when the strategy was tried and failed.
	Err string `json:"error,omitempty"`
}

// Outcome is the full result of one edit.
type Outcome struct {
	Operation   OperationID `json:"operation"`
	Description string      `json:"description"`

	// Before and After are the field's value, when it could be read.
	Before string `json:"before"`
	After  string `json:"after"`
	// BeforeKnown and AfterKnown distinguish "" from "could not read". Verification
	// depends on the difference: an empty field and an unreadable one look the same in
	// a bare string, and only one of them proves a Clear worked.
	BeforeKnown bool `json:"before_known"`
	AfterKnown  bool `json:"after_known"`

	Attempts []Attempt `json:"attempts,omitempty"`
	Strategy Strategy  `json:"strategy"`

	// Verified is whether the resulting STATE was proved, not whether input was sent.
	Verified bool   `json:"verified"`
	Evidence string `json:"evidence"`
	// ClipboardRestored reports that a borrowed clipboard was put back. False when the
	// clipboard was never touched.
	ClipboardBorrowed bool   `json:"clipboard_borrowed,omitempty"`
	ClipboardRestored bool   `json:"clipboard_restored,omitempty"`
	Error             string `json:"error,omitempty"`
}

// Summary is one line for a person.
func (o Outcome) Summary() string {
	verdict := "unverified"
	if o.Verified {
		verdict = "verified"
	}
	return fmt.Sprintf("%s via %s — %s: %s", o.Operation, o.Strategy, verdict, o.Evidence)
}

// FallbackReason is why the strongest strategy was not used, empty when it was.
func (o Outcome) FallbackReason() string {
	if o.Strategy == StrategyValueAPI || o.Strategy == StrategyNone {
		return ""
	}
	var parts []string
	for _, a := range o.Attempts {
		if a.Chosen || a.Reason == "" {
			continue
		}
		parts = append(parts, string(a.Strategy)+": "+a.Reason)
	}
	return strings.Join(parts, "; ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
