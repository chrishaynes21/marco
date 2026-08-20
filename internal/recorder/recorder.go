// Package recorder captures real OS input (keys, clicks, mouse moves) with
// timing, so a user can demonstrate a macro. The captured event stream is
// OS-agnostic (this file); the actual hooks are a per-OS platform capability
// (recorder_windows.go etc.), so the recorder is one of the swappable backends
// behind a small interface — Windows first, macOS/Linux additive.
package recorder

import (
	"errors"
	"time"
)

// ErrUnsupported is returned by New on platforms without a recorder backend yet.
var ErrUnsupported = errors.New("recorder: not supported on this platform")

// EventKind distinguishes the captured input events.
type EventKind int

const (
	EvKey EventKind = iota
	EvClick
	EvMove
	EvAppSwitch // foreground application changed; KeyName carries the new app name
)

// RecordedEvent is one captured input event with a wall-clock timestamp.
type RecordedEvent struct {
	Kind    EventKind
	VK      uint16    // virtual key code (EvKey)
	KeyName string    // human key name, lowercased (EvKey); app name (EvAppSwitch)
	Down    bool      // press vs release (EvKey / EvClick)
	Button  string    // "left" | "right" | "middle" (EvClick)
	X, Y    int       // screen coordinates (EvClick / EvMove)
	T       time.Time // capture time
	// Image, on an EvClick down, is a PNG of the distinctive region the user
	// clicked — captured live so codegen can match it on screen later (robust to
	// the UI moving). Empty when the area wasn't distinctive or capture failed.
	Image []byte
	// RelX, RelY are the click's offset from the top-left of the foreground window
	// at click time, set when WinRel is true (EvClick down). They let codegen emit a
	// window-relative click that lands at the same spot inside the window wherever it
	// is on screen — the default that makes routes portable across monitors/machines.
	RelX, RelY int
	WinRel     bool
	// Color, on an EvClick down with an explicit (armed) anchor, is the "0xRRGGBB"
	// pixel under the click, read from the captured frame. It becomes the anchor's
	// secondary resolver — confirm the screen by that one pixel when the template
	// match is marginal. Empty when there was no anchor capture.
	Color string
	// Window, on an EvClick down, is the title of the foreground window at click time.
	// It becomes the anchor's CONTEXT check — confirm the route is acting on the right
	// window of an app that opens several (Steam's library vs friends vs store), so a
	// match in the wrong window can't produce a confident click. Empty when unknown.
	Window string
	// ClickX, ClickY, on an EvClick down with an Image, are the click's position WITHIN
	// that Image (its own 0-origin pixel space). Learn-time OCR uses it to read the
	// button under the click rather than every word in the crop. (0,0) when no anchor
	// capture (no Image) — the OCR step then falls back to the template centre.
	ClickX, ClickY int
}

// Recorder captures input until stopped. Implementations install OS hooks.
type Recorder interface {
	// Start installs the hooks and begins capturing. Returns ErrUnsupported on
	// platforms with no backend.
	Start() error
	// Stop uninstalls the hooks and returns the ordered events captured.
	Stop() []RecordedEvent
	// Events streams events live (used to detect the stop hotkey).
	Events() <-chan RecordedEvent
}
