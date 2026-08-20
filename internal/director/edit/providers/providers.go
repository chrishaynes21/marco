// Package providers holds the capabilities an editor needs from the world.
//
// They are interfaces rather than concrete types for the usual reason — a fake in a
// test, a bridge in production — and for one specific reason: the editor's strategy
// ladder is defined by which of these are PRESENT. A missing ValueProvider is not an
// error, it is the reason the second rung exists. Making each capability separately
// optional is what lets "the strongest available strategy" be a real decision instead
// of a hopeful comment.
package providers

import (
	"context"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Target is the control being edited.
//
// Carries BOTH ids on purpose. Element is the Director's stable identity, which is
// what the plan, the verification record and the explanation all refer to. NativeID is
// what the accessibility provider knows the control by, and is the only thing a value
// API will accept. Keeping them apart is what stops a provider's dialect leaking into
// the Director's vocabulary.
type Target struct {
	Element  directorapi.ElementID
	NativeID string
	Window   directorapi.WindowID
	Role     directorapi.ElementRole
	Label    string
}

// Editable reports whether this target is the sort of thing text can be put into.
//
// A guard against a plan that would type into a label or a button. Not a permission
// check — that is policy's job — but a sanity check that target resolution produced
// something that can hold text at all.
//
// RoleUnknown counts. A control the accessibility provider could not classify is
// routinely a real edit surface behind a custom implementation, and refusing those
// would rule out a large share of the applications the Director is for. The safety
// this rule provides comes from what it EXCLUDES — buttons, labels, links, menu
// items — not from demanding a positive identification the tree often cannot give.
func (t Target) Editable() bool {
	return t.Role.TextEditable() || t.Role == directorapi.RoleUnknown
}

// Values sets and reads a control's value natively. Optional: many controls do not
// implement one, and many providers cannot reach it.
type Values interface {
	SetValue(ctx context.Context, window directorapi.WindowID, nativeID, value string) (string, error)
	GetValue(ctx context.Context, window directorapi.WindowID, nativeID string) (string, bool, error)
}

// Focus moves keyboard focus structurally.
//
// Structurally, meaning by asking the application — not by clicking. Clicking a
// control activates it, and a click that lands on a hyperlink inside a document to
// "focus the document" has navigated away. The rule the editor enforces is that no
// keystroke is ever emitted without focus having been established this way and then
// CONFIRMED; this interface is the establishing half.
type Focus interface {
	Focus(ctx context.Context, window directorapi.WindowID, nativeID string) error
}

// Observer re-reads the world so the editor can confirm focus and verify state.
//
// The editor never verifies by remembering what it sent. It verifies by looking.
type Observer interface {
	// Element returns the current belief about one element, false if it is gone.
	Element(ctx context.Context, window directorapi.WindowID, id directorapi.ElementID) (directorapi.Element, bool, error)
}

// Input is the keyboard, used only when everything better has been refused.
//
// Every method names the WINDOW the input is meant for, and that parameter is the
// whole point of this interface's shape. Keystrokes are delivered to whatever holds
// the foreground — SendInput has no notion of a destination — so an input call that
// does not say where it is going cannot be checked, and the foreground guard it
// should have passed through is silently skipped.
//
// That is not hypothetical. It is exactly how a keystroke path came to bypass the
// guard entirely: the window was known at the call site, the guard was wired, and the
// two were never connected, so `Confirm(ctx, "")` returned true every time and Ctrl+C
// went to whatever was in front. Making the window a required argument means the
// connection cannot be forgotten again.
type Input interface {
	Type(ctx context.Context, window directorapi.WindowID, text string) error
	Key(ctx context.Context, window directorapi.WindowID, chord string, hold time.Duration) error
}

// Settler waits for the world to reflect an edit.
//
// A wait, never a sleep. The editor asks "has the value changed yet?" and stops as
// soon as the answer is yes; a fixed delay would be both slower on a fast machine and
// wrong on a slow one. Optional — with no settler the editor verifies immediately,
// which is correct for a value API where the write already returned the new value.
type Settler interface {
	// Settle blocks until want returns true, the deadline passes, or ctx ends. The
	// bool reports whether the condition was actually satisfied, which is the
	// difference between "verified" and "timed out".
	Settle(ctx context.Context, want func(context.Context) bool) (bool, error)
}
