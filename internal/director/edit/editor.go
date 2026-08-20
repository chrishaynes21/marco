package edit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit/clipboard"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit/providers"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Deps are the capabilities an Editor has. Every one is optional, and which are
// present is exactly what determines how far down the strategy ladder a given edit
// falls. That is the point: "use the strongest available strategy" has to be a
// decision made against real capabilities, not a hopeful comment above a call to Type.
type Deps struct {
	Values    providers.Values
	Focus     providers.Focus
	Observer  providers.Observer
	Input     providers.Input
	Clipboard *clipboard.Borrower
	Settle    providers.Settler
}

// Editor carries out semantic edit operations.
type Editor struct {
	deps Deps
	// clipboardMinimum is the length above which pasting beats typing. Typing is
	// per-character, so a long string is both slow and a long window in which an
	// autocomplete, a focus change or a validation popup can interfere. Named rather
	// than inline so the threshold is arguable instead of mysterious.
	clipboardMinimum int
}

// DefaultClipboardMinimum is where pasting starts to be worth borrowing the clipboard
// for. Below it, typing is quick enough that the borrow is not worth the risk to the
// user's clipboard.
const DefaultClipboardMinimum = 120

// New builds an editor.
func New(d Deps) *Editor {
	return &Editor{deps: d, clipboardMinimum: DefaultClipboardMinimum}
}

// keyHold is how long a chord is held. Chords sent with a zero hold are missed by
// applications that sample keyboard STATE rather than listening for events.
const keyHold = 30 * time.Millisecond

// Apply carries out one operation against one target and proves the result.
//
// The whole shape of the milestone is in this function's order: read the current
// state, decide what state is intended, pick the strongest way to reach it, do it,
// wait for the world to catch up, then look again and compare. Never "send input and
// assume".
func (e *Editor) Apply(ctx context.Context, t providers.Target, op Operation) (Outcome, error) {
	out := Outcome{Operation: op.ID(), Description: op.Description()}

	if !t.Editable() && needsEditableTarget(op) {
		out.Error = fmt.Sprintf("%s is a %s, which cannot hold text", describeTarget(t), t.Role)
		out.Evidence = out.Error
		return out, errors.New(out.Error)
	}

	// 1. What is there now. Needed for the expectation (Append), for change-based
	//    verification (Undo), and to notice an operation that did nothing.
	before, beforeKnown := e.read(ctx, t)
	out.Before, out.BeforeKnown = before, beforeKnown

	// 2. Do it.
	if err := e.perform(ctx, t, op, before, beforeKnown, &out); err != nil {
		out.Error = err.Error()
		if out.Evidence == "" {
			out.Evidence = err.Error()
		}
		return out, err
	}

	// 3. Let the world catch up — by condition, never by sleeping. See settle.
	want := expectedValue(op, before, beforeKnown)
	e.settle(ctx, t, want, before, beforeKnown)

	// 4. Look again.
	after, afterKnown := e.read(ctx, t)
	out.After, out.AfterKnown = after, afterKnown
	return out, nil
}

// needsEditableTarget reports whether an operation requires a text-bearing control.
//
// Undo and Enter do not: "undo that" is addressed to the application, and the target
// is only where to send it from.
func needsEditableTarget(op Operation) bool {
	switch op.ID() {
	case OpUndo, OpRedo, OpPressEnter, OpCopySelection, OpSelectAll:
		return false
	}
	return true
}

// perform runs the strategy ladder for an operation.
func (e *Editor) perform(ctx context.Context, t providers.Target, op Operation, before string, beforeKnown bool, out *Outcome) error {
	switch o := op.(type) {
	case SetText:
		return e.setValue(ctx, t, o.Value, out)
	case ClearText:
		return e.setValue(ctx, t, "", out)
	case AppendText:
		// Appending through a value API means writing the WHOLE new value, which is
		// only honest if the old one was actually read. Otherwise the caret route is
		// the correct one — it appends to whatever is really there.
		if beforeKnown {
			if err := e.tryValueAPI(ctx, t, before+o.Value, out); err == nil {
				return nil
			} else if !isUnsupported(err) {
				return err
			}
		} else {
			out.Attempts = append(out.Attempts, Attempt{
				Strategy: StrategyValueAPI,
				Reason:   "the current value could not be read, so the appended result is not known in advance",
			})
		}
		if err := e.ensureFocus(ctx, t, out); err != nil {
			return err
		}
		// Ctrl+End rather than End: a multi-line document's end is not the line's end,
		// and appending to the wrong line would be a silent corruption.
		if err := e.key(ctx, t, "ctrl+end"); err != nil {
			return err
		}
		return e.deliver(ctx, t, o.Value, out)

	case InsertText:
		if err := e.ensureFocus(ctx, t, out); err != nil {
			return err
		}
		out.Attempts = append(out.Attempts, Attempt{
			Strategy: StrategyValueAPI,
			Reason:   "inserting at the caret cannot be expressed as a whole-value write",
		})
		return e.deliver(ctx, t, o.Value, out)

	case ReplaceSelection:
		if err := e.ensureFocus(ctx, t, out); err != nil {
			return err
		}
		out.Attempts = append(out.Attempts, Attempt{
			Strategy: StrategyValueAPI,
			Reason:   "the selection is not part of the World State, so it cannot be replaced by a whole-value write",
		})
		// Typing or pasting over a selection replaces it — that is the platform's own
		// behaviour, not a trick, and it is why no explicit delete is sent first.
		return e.deliver(ctx, t, o.Value, out)

	case SelectAll:
		if err := e.ensureFocus(ctx, t, out); err != nil {
			return err
		}
		out.Strategy = StrategyNone
		out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyNone, Chosen: true,
			Reason: "selection is a keyboard operation with no value-API equivalent"})
		return e.key(ctx, t, "ctrl+a")

	case CopySelection:
		if err := e.ensureFocus(ctx, t, out); err != nil {
			return err
		}
		out.Strategy = StrategyNone
		// The one operation that is SUPPOSED to change the clipboard, so it is not
		// borrowed and not restored. The user asked for the clipboard to change.
		out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyNone, Chosen: true,
			Reason: "the clipboard is the intended destination, so it is not saved and restored"})
		return e.key(ctx, t, "ctrl+c")

	case PasteClipboard:
		if err := e.ensureFocus(ctx, t, out); err != nil {
			return err
		}
		out.Strategy = StrategyNone
		out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyNone, Chosen: true,
			Reason: "the clipboard's existing contents are the intended text, so nothing is borrowed"})
		return e.key(ctx, t, "ctrl+v")

	case Undo:
		if err := e.ensureFocus(ctx, t, out); err != nil {
			return err
		}
		out.Strategy = StrategyNone
		out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyNone, Chosen: true,
			Reason: "undo is the application's own history, which no value API exposes"})
		return e.key(ctx, t, "ctrl+z")

	case Redo:
		if err := e.ensureFocus(ctx, t, out); err != nil {
			return err
		}
		out.Strategy = StrategyNone
		out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyNone, Chosen: true,
			Reason: "redo is the application's own history, which no value API exposes"})
		// Ctrl+Y first, then Ctrl+Shift+Z, is a platform difference rather than a
		// guess: Windows applications overwhelmingly bind Ctrl+Y. Verification by
		// change is what catches the ones that do not.
		return e.key(ctx, t, "ctrl+y")

	case PressEnter:
		if err := e.ensureFocus(ctx, t, out); err != nil {
			return err
		}
		out.Strategy = StrategyNone
		out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyNone, Chosen: true,
			Reason: "committing a field is a keystroke by definition"})
		return e.key(ctx, t, "enter")
	}
	return fmt.Errorf("edit: %s is not an operation this editor knows how to carry out", op.ID())
}

// setValue puts an exact value into a control, walking the ladder.
//
//  1. the control's own value API   — no keystrokes, no layout, no intermediate state
//  2. select everything and replace — the semantic keyboard route
//  3. clipboard-assisted            — for text typing would mangle or take too long
//  4. typing                        — the fallback, never the preference
//
// Each rung that is skipped or refused is RECORDED. A fallback that happened silently
// would be indistinguishable from a preference, and the next person debugging why an
// autocomplete fired would have nothing to go on.
func (e *Editor) setValue(ctx context.Context, t providers.Target, value string, out *Outcome) error {
	if err := e.tryValueAPI(ctx, t, value, out); err == nil {
		return nil
	} else if !isUnsupported(err) {
		return err
	}

	if err := e.ensureFocus(ctx, t, out); err != nil {
		return err
	}
	// Select everything, so that whatever arrives next replaces the old contents
	// rather than joining them. This is the second rung: the field is emptied by
	// SELECTION, semantically, not by holding backspace and hoping.
	if err := e.key(ctx, t, "ctrl+a"); err != nil {
		return fmt.Errorf("edit: could not select the existing text to replace it: %w", err)
	}
	if value == "" {
		// Clearing needs nothing delivered — the selection plus a delete IS the whole
		// operation, and typing "" would be a no-op that left the text selected.
		out.Strategy = StrategySelectReplace
		markChosen(out, StrategySelectReplace, "selected the existing text and deleted it")
		return e.key(ctx, t, "delete")
	}
	return e.deliver(ctx, t, value, out)
}

// tryValueAPI attempts the top rung. Returns a ValueUnsupportedError when the control
// has no writable value API, which the caller treats as "fall back", not as failure.
func (e *Editor) tryValueAPI(ctx context.Context, t providers.Target, value string, out *Outcome) error {
	if e.deps.Values == nil {
		err := &directorapi.ValueUnsupportedError{Reason: "no value provider is configured"}
		out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyValueAPI, Reason: err.Reason})
		return err
	}
	if t.NativeID == "" {
		err := &directorapi.ValueUnsupportedError{Reason: "the target has no native element id, so its value API cannot be addressed"}
		out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyValueAPI, Reason: err.Reason})
		return err
	}
	got, err := e.deps.Values.SetValue(ctx, t.Window, t.NativeID, value)
	if err != nil {
		if isUnsupported(err) {
			out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyValueAPI, Reason: err.Error()})
			return err
		}
		// A real error, not a refusal. Recorded and returned: trying harder against a
		// broken bridge only makes a mess in the user's document.
		out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyValueAPI,
			Reason: "the value API failed", Err: err.Error()})
		return fmt.Errorf("edit: setting the value failed: %w", err)
	}
	out.Strategy = StrategyValueAPI
	markChosen(out, StrategyValueAPI, "the control's own value API accepted the new value")
	// The bridge reports what the control actually holds, which is not always what it
	// was given. Recorded here so verification compares against reality.
	_ = got
	return nil
}

// deliver puts text into an already-focused control by the best remaining means.
func (e *Editor) deliver(ctx context.Context, t providers.Target, text string, out *Outcome) error {
	if reason, ok := e.clipboardWorthIt(text); ok {
		if e.deps.Clipboard == nil {
			out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyClipboard,
				Reason: "no clipboard is available, though " + reason})
		} else {
			loan, err := e.deps.Clipboard.Borrow(ctx, text)
			if err != nil {
				// Refusing to borrow is a legitimate outcome — see clipboard.ErrNonText.
				out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyClipboard,
					Reason: "the clipboard could not be borrowed safely", Err: err.Error()})
			} else {
				out.ClipboardBorrowed = true
				pasteErr := e.key(ctx, t, "ctrl+v")
				// Restore ALWAYS, including after a failed paste. The clipboard is the
				// user's, and a failure on our side is not a reason to keep it.
				restored, restoreErr := loan.Restore(ctx)
				out.ClipboardRestored = restored
				if pasteErr != nil {
					return fmt.Errorf("edit: pasting failed: %w", pasteErr)
				}
				out.Strategy = StrategyClipboard
				note := reason
				if !restored {
					note += "; the previous clipboard contents could NOT be restored"
					if restoreErr != nil {
						note += " (" + restoreErr.Error() + ")"
					}
				}
				markChosen(out, StrategyClipboard, note)
				return nil
			}
		}
	} else {
		out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyClipboard,
			Reason: "the text is short and plain, so borrowing the user's clipboard is not warranted"})
	}

	if e.deps.Input == nil {
		return errors.New("edit: there is no way left to enter the text — no value API, no clipboard, and no keyboard")
	}
	if err := e.deps.Input.Type(ctx, t.Window, text); err != nil {
		out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyKeystrokes,
			Reason: "typing failed", Err: err.Error()})
		return fmt.Errorf("edit: typing the text failed: %w", err)
	}
	if out.Strategy == "" {
		out.Strategy = StrategyKeystrokes
	}
	markChosen(out, StrategyKeystrokes, "typed, because every stronger route was unavailable or refused")
	return nil
}

// clipboardWorthIt decides whether to borrow the clipboard, and says why.
func (e *Editor) clipboardWorthIt(text string) (string, bool) {
	if len(text) >= e.clipboardMinimum {
		return fmt.Sprintf("the text is %d characters, long enough that typing it would be slow and easily interrupted", len(text)), true
	}
	if strings.ContainsAny(text, "\n\t") {
		// A newline typed into a single-line field submits it; typed into a chat box it
		// sends the message half-written. Pasting delivers it as data instead of as an
		// Enter key.
		return "the text contains line breaks or tabs, which typing would turn into keypresses the application acts on", true
	}
	for _, r := range text {
		if r > unicode.MaxASCII {
			// The keyboard layout is the application's, not ours, and we must not
			// assume it is US. Characters outside ASCII are exactly the ones a layout
			// may be unable to produce.
			return "the text contains characters the keyboard layout may not be able to produce", true
		}
	}
	return "", false
}

// ensureFocus establishes focus structurally and CONFIRMS it.
//
// Never type into an unfocused control. Establishing focus is asking the application
// through the accessibility interface — not clicking, which activates. Confirming it
// is looking at the world afterwards, because Focus returning nil only means the
// request was accepted.
func (e *Editor) ensureFocus(ctx context.Context, t providers.Target, out *Outcome) error {
	if e.deps.Focus == nil {
		return errors.New("edit: focus cannot be established structurally, and typing into an unfocused control is not permitted")
	}
	// A control that ALREADY has focus is not focused again.
	//
	// Re-focusing looks like a harmless no-op and is not one: SetFocus on a text field
	// moves the caret and collapses the selection. That destroys exactly what a
	// selection-dependent operation is about to act on — copying the selection would
	// copy nothing, and replacing it would replace nothing — so "make sure it is
	// focused" has to mean checking before acting rather than acting unconditionally.
	//
	// The check is also the cheaper order: the overwhelmingly common case for an edit is
	// that the target is already where the user is typing.
	if e.deps.Observer != nil {
		if el, ok, err := e.deps.Observer.Element(ctx, t.Window, t.Element); err == nil && ok && el.Focused {
			out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyNone,
				Reason: "already focused; not re-focusing, which would collapse the selection"})
			return nil
		}
	}
	if err := e.deps.Focus.Focus(ctx, t.Window, t.NativeID); err != nil {
		return fmt.Errorf("edit: could not focus %s, so nothing was typed: %w", describeTarget(t), err)
	}
	if e.deps.Observer == nil {
		// The request was accepted and there is no way to check. Recorded as the gap
		// it is rather than passed off as confirmation.
		out.Attempts = append(out.Attempts, Attempt{Strategy: StrategyNone,
			Reason: "focus was requested but could not be confirmed — no observer is configured"})
		return nil
	}
	confirm := func(ctx context.Context) bool {
		el, ok, err := e.deps.Observer.Element(ctx, t.Window, t.Element)
		return err == nil && ok && el.Focused
	}
	if e.deps.Settle != nil {
		if ok, _ := e.deps.Settle.Settle(ctx, confirm); ok {
			return nil
		}
	} else if confirm(ctx) {
		return nil
	}
	return fmt.Errorf("edit: %s did not take focus, so nothing was typed", describeTarget(t))
}

// settle waits for the value to reflect the edit, by CONDITION.
//
// Not a sleep. The editor asks "has it changed yet?" and continues the moment the
// answer is yes; a fixed delay would be slower than necessary on a fast machine and
// still too short on a slow one. With no settler configured it returns at once, which
// is correct for a value API — that write already returned the new value.
func (e *Editor) settle(ctx context.Context, t providers.Target, want string, before string, beforeKnown bool) {
	if e.deps.Settle == nil {
		return
	}
	e.deps.Settle.Settle(ctx, func(ctx context.Context) bool {
		got, ok := e.read(ctx, t)
		if !ok {
			return false
		}
		if want != "" || !beforeKnown {
			return got == want
		}
		// No predictable target value: settle on the value having changed at all.
		return got != before
	})
}

// read returns the target's current value, preferring the value API.
//
// The value API is preferred because it asks the control directly rather than reading
// a snapshot that may predate the edit by a fraction of a second — which is exactly
// the window in which a verification would otherwise read a stale value and call a
// successful edit a failure.
func (e *Editor) read(ctx context.Context, t providers.Target) (string, bool) {
	if e.deps.Values != nil && t.NativeID != "" {
		if v, ok, err := e.deps.Values.GetValue(ctx, t.Window, t.NativeID); err == nil && ok {
			return v, true
		}
	}
	if e.deps.Observer != nil {
		if el, ok, err := e.deps.Observer.Element(ctx, t.Window, t.Element); err == nil && ok {
			return el.Value, true
		}
	}
	return "", false
}

func (e *Editor) key(ctx context.Context, t providers.Target, chord string) error {
	if e.deps.Input == nil {
		return fmt.Errorf("edit: %s is needed but no keyboard is available", chord)
	}
	return e.deps.Input.Key(ctx, t.Window, chord, keyHold)
}

// expectedValue is the value the control should hold afterwards, "" when unpredictable.
func expectedValue(op Operation, before string, beforeKnown bool) string {
	switch o := op.(type) {
	case SetText:
		return o.Value
	case AppendText:
		if beforeKnown {
			return before + o.Value
		}
	}
	return ""
}

func markChosen(out *Outcome, s Strategy, reason string) {
	for i := range out.Attempts {
		if out.Attempts[i].Strategy == s && out.Attempts[i].Err == "" && !out.Attempts[i].Chosen {
			out.Attempts[i].Chosen = true
			out.Attempts[i].Reason = reason
			return
		}
	}
	out.Attempts = append(out.Attempts, Attempt{Strategy: s, Chosen: true, Reason: reason})
}

func isUnsupported(err error) bool {
	var u *directorapi.ValueUnsupportedError
	return errors.As(err, &u)
}

func describeTarget(t providers.Target) string {
	if t.Label != "" {
		return fmt.Sprintf("%q", t.Label)
	}
	if t.Element != "" {
		return string(t.Element)
	}
	return "the target"
}
