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
	// RelTo names what RelX,RelY are measured FROM, set with them. `macroir.RelToWindow`
	// — the window's OUTER top-left, frame included, because that is what the origin read
	// returns.
	//
	// Recorded on the EVENT and not named later by the pass that lowers it, although
	// there is one value today. A raw recording is kept so it can be simplified again,
	// possibly by a build whose recorder measures from somewhere else — and a stream
	// holding only `RelX 391` with the basis supplied at lowering time would be
	// reinterpreted by that build without anything to notice.
	RelTo string
	// WinW, WinH, Dpi are what the window the press landed in was like AT THE INSTANT of
	// the press: its outer dimensions, and its DPI as its own term.
	//
	// Read in the hook, from the same window handle as the origin and by the same class
	// of call — window data Windows already holds, no process handle opened. A press is
	// the only moment this can be read, because anything later is a sample and a sample
	// cannot answer about an instant it missed.
	//
	// Dpi is 0 when Windows would not say, which is not 96. A DPI change is not a size
	// change: an application that handles `WM_DPICHANGED` relayouts inside a rectangle
	// that never moved, so nothing about the dimensions would show it.
	WinW, WinH, Dpi int
	// Process is the file name of the executable that owned that window, lowercased.
	//
	// Resolved AFTER the capture, off the hook thread, from `PID` — which was read at the
	// instant of the event. Reading it in the hook would mean opening a process handle,
	// which is the work that makes Windows drop the hooks; reading it from the app
	// poller's sample would mean asking the one witness that cannot have seen the moment.
	// Empty when the process had already gone, which is a real answer.
	Process string
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
	// PID is the process that owned the foreground window AT THE MOMENT of the event,
	// read in the hook from two window-data calls. Zero when it could not be read.
	//
	// This is the authoritative answer to "which app is this", and `App` below is not: App
	// comes from a poller sampling the foreground on an interval, and a poller cannot
	// answer about an instant it has not sampled. Somebody who brings a window forward
	// and clicks inside it faster than the sample interval is attributed by App to the app
	// they LEFT — so a recording stopped by a button that quickly kept the click that
	// stopped it, with both of the rules meant to catch it reading the same stale sample.
	// Set on clicks, where it is needed and where a window call is already being made.
	PID uint32
	// App is the application the event landed in — the lowercased executable basename,
	// as EvAppSwitch reports it. Read from the app poller's own last sample rather than
	// looked up here, because this is set on the low-level hook thread and a process
	// query there is exactly the kind of work that makes Windows drop the hooks.
	//
	// It exists so a recording can be told apart from the act of ENDING a recording. A
	// person who stops by pressing a button in Marco's own window has clicked a real
	// button in a real window, and the hooks record it like any other click; without
	// knowing which app each event landed in, that click becomes a step and every replay
	// re-opens Marco and presses Stop. Empty when no sample has been taken yet.
	App string
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

// resolveImages fills in each press's `Process` from the `PID` read at the instant of that press.
//
// Called once, after the hooks are gone, with one lookup per DISTINCT process rather than one
// per click: a recording of a menu is thirty presses in one program, and thirty handle opens for
// one answer would be thirty chances to be told a different one.
//
// A process that has already exited cannot be named, and the name STAYS MISSING. Not filled from
// the app poller's last sample, which would put a guess where the point is to have measured
// something; and not taken as licence to drop the press's basis either, which was the first
// answer here and the wrong one — a press that gives up its basis is a press that runs at its
// recorded coordinate having checked nothing, so a failed lookup would have been quietly turning
// the safeguards off. Guarded replay is simply unavailable for that press until it is
// demonstrated again, and replay says exactly that.
//
// `look` is a parameter so this can be tested without a desktop. It is `winctx.ImageOf` in
// production and nothing else.
func resolveImages(events []RecordedEvent, look func(uint32) string) {
	seen := map[uint32]string{}
	for i := range events {
		pid := events[i].PID
		// ONLY A PRESS IS ABOUT A WINDOW. The release carries the same process —
		// both edges are stamped — but it is about no rectangle, so a name beside
		// it would describe a window nothing was measured in.
		if events[i].RelTo == "" {
			continue
		}
		name, asked := seen[pid]
		if !asked && pid != 0 {
			name = look(pid)
			seen[pid] = name
		}
		events[i].Process = name
	}
}
