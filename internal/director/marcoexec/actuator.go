package marcoexec

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The Executor IS the Director's actuator, its focuser, its value provider and its
// clipboard. Every one of those interfaces is served by lowering a typed Operation
// into Marco and compiling it.
//
// That is the whole milestone in four assertions: there is no other implementation
// left for the Director to reach, so "Director execution code must not invoke desktop
// hosts directly" is enforced by there being nothing to invoke.
var (
	_ directorapi.Actuator  = (*Executor)(nil)
	_ directorapi.Clipboard = (*Executor)(nil)
)

// DefaultMaxNodes matches uiaclient's bound, so a value written through a Marco
// program walks the same tree a snapshot does.
const DefaultMaxNodes = 1500

// ── directorapi.Actuator ──────────────────────────────────────────────────────

// Click clicks at an absolute point.
func (e *Executor) Click(ctx context.Context, at directorapi.Point, button directorapi.MouseButton, count int) error {
	_, err := e.Do(ctx, Operation{
		Kind: KindClick, At: Point{X: at.X, Y: at.Y},
		Button: string(button), Count: count,
	})
	return err
}

// Move moves the pointer without clicking.
func (e *Executor) Move(ctx context.Context, to directorapi.Point) error {
	_, err := e.Do(ctx, Operation{Kind: KindMove, At: Point{X: to.X, Y: to.Y}})
	return err
}

// Drag performs a press-hold-release drag.
//
// One capability, not three. OS's Drag carries both endpoints and the button, so the
// press, the glide and the release stay atomic inside the host — there is no window
// between them for a window to move or a foreground to change. That is also why
// cancellation is checked BEFORE the drag and not during it: Marco has no safe way to
// abandon a drag with the button down, and releasing at an arbitrary point would drop
// whatever was being dragged somewhere nobody chose.
func (e *Executor) Drag(ctx context.Context, from, to directorapi.Point, button directorapi.MouseButton) error {
	_, err := e.Do(ctx, Operation{
		Kind: KindDrag,
		From: Point{X: from.X, Y: from.Y}, To: Point{X: to.X, Y: to.Y},
		Button: string(button),
	})
	return err
}

// Key presses a key or chord, with no named destination.
//
// Reaches whatever holds the foreground, and therefore CANNOT be checked: the guard
// is given an empty window and passes trivially. Kept because directorapi.Actuator
// requires it, and safe only for input that is genuinely addressed to the desktop
// rather than to a control.
//
// Anything editing a control must use KeyIn. See providers.Input, whose signature
// makes that the only reachable option.
func (e *Executor) Key(ctx context.Context, chord string, hold time.Duration) error {
	return e.KeyIn(ctx, "", chord, hold)
}

// KeyIn presses a key or chord INTO a named window.
//
// The window is what lets the foreground guard do its job. Without it a Ctrl+C
// resolved against Notepad executes happily while Discord is in front, reports success,
// and copies nothing — the exact failure that made selected-text capture impossible.
func (e *Executor) KeyIn(ctx context.Context, window directorapi.WindowID, chord string,
	hold time.Duration) error {

	op := Operation{Kind: KindKey, Chord: chord, Window: string(window)}
	if hold > 0 {
		op.Hold = int(hold / time.Millisecond)
	}
	_, err := e.Do(ctx, op)
	return err
}

// Type enters literal text with no named destination. See Key.
func (e *Executor) Type(ctx context.Context, text string) error {
	return e.TypeIn(ctx, "", text)
}

// TypeIn enters literal text INTO a named window. Never used for credentials — see
// TypeSecret.
func (e *Executor) TypeIn(ctx context.Context, window directorapi.WindowID, text string) error {
	_, err := e.Do(ctx, Operation{Kind: KindType, Text: text, Window: string(window)})
	return err
}

// TypeSecret enters a stored credential by NAME.
//
// The value is fetched inside Marco's host from the OS credential store and typed
// there. It is never returned here, never lowered into source, never logged, and never
// present in a plan, an action record or a model prompt. Routing credentials through
// this capability rather than through Type is the entire reason it exists.
func (e *Executor) TypeSecret(ctx context.Context, ref string) error {
	_, err := e.Do(ctx, Operation{Kind: KindTypeSecret, SecretRef: ref})
	return err
}

// Activate focuses a named application, launching it if it is not running.
//
// Deliberately NOT the same as Launch, and deliberately not a fallback for it. OS's
// Activate is Marco's "focus this app, starting it if need be"; Launch starts
// something regardless. Collapsing them would mean a failed activation quietly
// starting a second copy of an application the user already had open.
func (e *Executor) Activate(ctx context.Context, application string) error {
	_, err := e.Do(ctx, Operation{Kind: KindActivate, App: application})
	return err
}

// Launch starts an application, path or URL through the shell.
func (e *Executor) Launch(ctx context.Context, target string) error {
	_, err := e.Do(ctx, Operation{Kind: KindLaunch, Target: target})
	return err
}

// MoveWindow moves and resizes a window.
func (e *Executor) MoveWindow(ctx context.Context, window directorapi.WindowID, to directorapi.Rect) error {
	_, err := e.Do(ctx, Operation{
		Kind: KindMoveWindow, Window: string(window),
		Bounds: Rect{X: to.X, Y: to.Y, W: to.Width, H: to.Height},
	})
	return err
}

// SetWindowState applies a symbolic window state.
func (e *Executor) SetWindowState(ctx context.Context, window directorapi.WindowID, state directorapi.WindowState) error {
	_, err := e.Do(ctx, Operation{
		Kind: KindWindowState, Window: string(window), State: string(state),
	})
	return err
}

// Scroll has no Marco capability.
//
// Reported as unsupported rather than approximated. Substituting a sequence of key
// presses or a drag would be a different action with different effects, silently
// performed — the hidden fallback this whole design exists to prevent. When Marco
// gains a scroll primitive this becomes an ordinary lowering.
func (e *Executor) Scroll(ctx context.Context, at directorapi.Point, delta int, horizontal bool) error {
	return fmt.Errorf("marcoexec: Marco has no scroll capability, and it will not be approximated " +
		"with keys or a drag; add OS's Scroll to os.marco and its host first")
}

// ── focus and control values ──────────────────────────────────────────────────

// Focus moves keyboard focus structurally, by asking the application.
//
// Never by clicking: a click ACTIVATES, and a click into a document to "focus it" can
// land on a hyperlink and navigate away.
func (e *Executor) Focus(ctx context.Context, window directorapi.WindowID, nativeID string) error {
	_, err := e.Do(ctx, Operation{
		Kind: KindFocus, Window: string(window), Element: nativeID, MaxNodes: DefaultMaxNodes,
	})
	return err
}

// SetValue writes a control's value through its native API.
//
// Returns what the control HOLDS afterwards, read from the capability's own return —
// not the value it was asked to hold. A field with a length cap accepts the write and
// keeps something else, and returning the asked-for value would make verification a
// tautology.
func (e *Executor) SetValue(ctx context.Context, window directorapi.WindowID, nativeID, value string) (string, error) {
	res, err := e.Do(ctx, Operation{
		Kind: KindSetValue, Window: string(window), Element: nativeID,
		Value: value, MaxNodes: DefaultMaxNodes,
	})
	if err != nil {
		// A refusal — the control has no writable value API — must stay
		// distinguishable from a fault: the caller falls back on the first and stops
		// on the second.
		if reason, unsupported := unsupportedReason(res, "Accessibility's SetValue"); unsupported {
			return "", &directorapi.ValueUnsupportedError{Reason: reason}
		}
		return "", err
	}
	if v, ok := res.Returned["Accessibility's SetValue"]; ok {
		if got, ok := v.Field("Value"); ok {
			return got, nil
		}
	}
	return "", nil
}

// GetValue reads a control's current value.
//
// The bool separates "this control has no value" from "its value is empty". A label
// has no value; an emptied search box does, and it is "". Verifying a clear depends
// entirely on telling those apart.
func (e *Executor) GetValue(ctx context.Context, window directorapi.WindowID, nativeID string) (string, bool, error) {
	res, err := e.Do(ctx, Operation{
		Kind: KindGetValue, Window: string(window), Element: nativeID, MaxNodes: DefaultMaxNodes,
	})
	if err != nil {
		if _, unsupported := unsupportedReason(res, "Accessibility's GetValue"); unsupported {
			return "", false, nil // no value API — nothing to read, not an error
		}
		return "", false, err
	}
	if v, ok := res.Returned["Accessibility's GetValue"]; ok {
		if got, ok := v.Field("Value"); ok {
			return got, true, nil
		}
	}
	return "", false, nil
}

// ── directorapi.Clipboard ─────────────────────────────────────────────────────

// Read reads the clipboard.
func (e *Executor) Read(ctx context.Context) (directorapi.ClipboardContents, error) {
	res, err := e.Do(ctx, Operation{Kind: KindClipboardGet})
	if err != nil {
		return directorapi.ClipboardContents{}, err
	}
	v, ok := res.Returned["OS's ClipboardGet"]
	if !ok {
		return directorapi.ClipboardContents{}, fmt.Errorf("marcoexec: the clipboard read returned nothing")
	}
	text, _ := v.Field("Text")
	isText, _ := v.Field("IsText")
	empty, _ := v.Field("Empty")
	return directorapi.ClipboardContents{
		Text: text, IsText: isText == "true", Empty: empty == "true",
	}, nil
}

// Write puts text on the clipboard.
func (e *Executor) Write(ctx context.Context, text string) error {
	_, err := e.Do(ctx, Operation{Kind: KindClipboardSet, Text: text})
	return err
}

// unsupportedReason reports whether a failure was a capability REFUSING rather than
// erroring, and why.
//
// The bridge answers "unsupported: …" for a control with no writable value API. That
// is an answer, not a fault: it tells the caller to fall back. Collapsing the two
// would turn every read-only field into an outage, and every outage into a burst of
// typing into whatever happened to have focus.
func unsupportedReason(res Result, key string) (string, bool) {
	msg := res.Diagnostic
	if v, ok := res.Returned[key]; ok && v.Error != "" {
		msg = v.Error
	}
	if i := strings.Index(msg, "unsupported:"); i >= 0 {
		return strings.TrimSpace(msg[i+len("unsupported:"):]), true
	}
	return "", false
}

// Unsupported reports whether an operation was refused because the control lacks the pattern.
//
// # Why this is exported and unsupportedReason is not
//
// Because the `unsupported:` contract has a second reader. The host attaches that prefix — its own
// comment says why: "so the Director can fall back and RECORD that it did" — and until now the
// only caller was SetValue, which handles its own fallback privately. A press has the same need
// and must not restate the contract: a second copy of "how do I recognise a capability refusing"
// is a second thing to keep in agreement with a C# file.
//
// The distinction it draws is the load-bearing one. A control that does not implement a pattern
// can be pressed another way; a control that vanished, or a bridge that died, cannot, and
// retrying that under a different pattern would be Marco pressing a control repeatedly until
// something happened.
//
// Deleting this must fail TestAPressFallsBackToThePatternTheControlImplements.
func Unsupported(res Result, kind Kind) (string, bool) {
	key, ok := controlKinds[kind]
	if !ok {
		return "", false
	}
	return unsupportedReason(res, key)
}
