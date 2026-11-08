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
	// -- CAPTURE STATE, WHICH IS NOT INPUT AND NEVER REACHES A SAVED RECORDING --
	//
	// The five below are markers: they carry a time and, for EvCrossed and EvHeldAcross,
	// a sentence or a key, and no coordinate or window. They exist so the live channel can
	// tell a surface what the capture is DOING, and so `Finish` can transform the clock
	// and refuse a recording that crossed the boundary.
	//
	// `orchestrator.inWindowClock` consumes all five. `Draft.Events` contains NONE of
	// them, which is deliberate and is the whole compatibility answer: a stored recording
	// holds only the kinds it always held, with gaps that are already right, so there is
	// nothing in it for an older reader to interpret differently.
	EvLeft       // the chosen window stopped being the foreground; nothing is admitted now
	EvEntered    // the chosen window became the foreground; admission resumes
	EvCrossed    // a gesture crossed the boundary; KeyName carries why it cannot be saved
	EvTargetGone // the chosen window closed or changed owner; the recording ends
	// EvHeldAcross is a key that was down in the chosen window when the boundary was
	// crossed, and KeyName is the key — a FACT, not a verdict.
	//
	// # Why this is not EvCrossed
	//
	// Because the boundary cannot tell, at the instant it happens, which of two things it
	// is looking at. A key still held as the window is left can be a gesture the
	// demonstration depends on and whose other half happened somewhere else — or it can be
	// the Alt-Tab that somebody used to come back to Studio and press Stop. Those call for
	// opposite answers, and what separates them is what happens NEXT: nothing, or the rest
	// of a demonstration.
	//
	// So the boundary reports the key and `inWindowClock` decides, where the whole stream
	// is in front of it. It used to decide here, which made the one obvious way to end a
	// recording — go back to the window with the Stop button in it — refuse every recording
	// it ended. Appended rather than inserted so no other kind's number moves.
	EvHeldAcross
)

// Marker reports whether k is capture state rather than something the person did.
func (k EventKind) Marker() bool {
	switch k {
	case EvLeft, EvEntered, EvCrossed, EvTargetGone, EvHeldAcross:
		return true
	}
	return false
}

// RecordedEvent is one captured input event with a wall-clock timestamp.
type RecordedEvent struct {
	Kind    EventKind
	VK      uint16    // virtual key code (EvKey)
	KeyName string    // human key name, lowercased (EvKey); app name (EvAppSwitch)
	Down    bool      // press vs release (EvKey / EvClick)
	Button  string    // one of macroir.MouseButtons (EvClick)
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
	// WinX, WinY are where that window's outer top-left WAS, at the same instant.
	//
	// RECORDING EVIDENCE, AND NEVER A REPLAY TARGET. Nothing emits these into a route and
	// nothing resolves against them: a saved absolute origin is the thing the whole
	// window-relative design exists to avoid. They are here so a pass that lowers a
	// gesture spanning two edges can ask whether both edges are about the SAME rectangle
	// — a question an offset cannot answer, because an offset is what is left once the
	// origin has been subtracted away. Two offsets agree happily about a window that
	// moved between them and describe a gesture no static window can reproduce.
	WinX, WinY int
	// HWnd is the window handle the attribution named at the instant of the event, and
	// `PID` beside it is its owner then.
	//
	// RECORDING EVIDENCE, AND NEVER A REPLAY TARGET, for the same reason and one more: a
	// handle is only meaningful inside the session that issued it, and Windows reuses
	// them. What it is for is identity WITHIN one recording — whether the two edges of a
	// gesture happened in the same window. Geometry and a program name cannot answer
	// that: two windows of one executable, the same size at the same scale, satisfy every
	// comparison of those and are two different windows.
	HWnd uintptr
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
		// WHATEVER DECLARED A BASIS IS ABOUT A WINDOW, which is now both edges of a
		// click and not just the press.
		//
		// It read "only a press is about a window" while the release was measured not
		// at all. A drag's release is a point in a window exactly as much as its press
		// is, so both edges are measured and both are named — and the condition did
		// not have to change for that, because it was never about the edge. It asks
		// whether this event claimed a rectangle, which is the question.
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

// ── WHAT A RECORDING CAPTURES, WHEN SOMEBODY SAYS SO ──────────────────────────

// Mode is what a recording captures beside the coordinate and the clock.
//
// # Why a recorder has to be TOLD, and not left to read the environment
//
// `captureAnchors` and `cvKitchenSink` are package variables read ONCE, from `$MARCO_CV` and
// `$MARCO_ANCHORS`, when the process starts. That is right for a command: you set the switch and
// run `marco learn`, and the setting is the whole of what you asked for.
//
// It is wrong for a window that stays open. Studio inherits whatever launched it for the life of
// the window, and with anchors on a click whose cropped region is distinctive carries a picture —
// which `codegen.Unpropagatable` then refuses, because a press found by matching a picture cannot
// be stopped at. So the same demonstration SAVES OR IS REFUSED depending on a variable nobody in
// the window can see and the pixels that happened to be under the pointer. A person recording a
// routine twice gets two different answers and no way to tell why.
//
// So a caller that has a mode in mind states it, for the recording it is about to start, and what
// it states wins. A caller that states nothing is every caller there was: the overlay, `marco
// learn`, `director demonstrate-visual` and the CLI all keep reading the environment, because
// turning the switch on and demonstrating something is what the switch is FOR there.
type Mode int

const (
	// Inherited leaves it to $MARCO_CV / $MARCO_ANCHORS — the behaviour every caller
	// had before any of this, and the behaviour they keep.
	Inherited Mode = iota
	// Coordinates captures the coordinate and the clock and nothing else: no picture of
	// the control, no colour under it, and the anchor key stays an ordinary key that
	// reaches whatever is in front.
	//
	// It is not "CV off" spelled differently. `$MARCO_CV=off` is a setting of the
	// process; this is a statement about ONE recording, made by the thing that started
	// it, and it cannot be overridden by what the process was launched with.
	Coordinates
)

// Stated is a recorder that can be told what to capture.
//
// An interface, and an optional one, for the reason `inputCounter` is: the rule has to be
// exercisable against a recorder that is not the real one, and a backend with no notion of
// pictures to turn off (the stub) must not have to grow one. A caller asks, and a recorder that
// cannot answer is a recorder that captures what it always did.
type Stated interface {
	// Capture states the mode for the recording `Start` begins next.
	//
	// Before Start, because the hooks read it: a mode stated afterwards would be a mode
	// stated after the first press could already have been captured.
	Capture(Mode)
}
