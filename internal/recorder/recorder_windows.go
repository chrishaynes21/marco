//go:build windows

package recorder

import (
	"fmt"
	"image"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/screen"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// clickTemplateRadius is the half-size of the FALLBACK patch captured around a left-click
// — used only when button recognition can't isolate a control at the click (a flat panel,
// a freeform canvas). The normal path captures the whole desktop and crops to the detected
// button (screen.AutoCropAt), re-centring the anchor on it; the fallback patch is centred
// on the click, so locating it and clicking its centre reproduces the click. Tune with
// $MARCO_CLICK_RADIUS (pixels; the patch is 2× this).
var clickTemplateRadius = clickRadiusFromEnv()

// captureAnchors gates per-click anchor capture. Default OFF — the CV anchor stack (match a
// moving control by image/colour/edge) isn't reliable enough yet, so it's behind a feature
// flag: learning records plain coordinates + timings unless CV is explicitly turned on
// ($MARCO_CV=on/max, or $MARCO_ANCHORS=1/on). See anchorsEnabled.
var captureAnchors = anchorsEnabled()

// cvKitchenSink is the MARCO_CV=max mode — the one switch that throws EVERY signal at
// EVERY click for a maximally robust (and maximally A/B-testable) capture: anchor every
// button (not just left), and capture even a non-distinctive patch so the OCR/Vision
// resolvers can still label it. Run-time scoring stays conservative (it won't move off
// the recorded point without strong, corroborated evidence), so "kitchen sink" means more
// signals captured, never less-safe clicking.
var cvKitchenSink = cvMode() == "max"

// cvMode reads the MARCO_CV master switch: "max" → the kitchen sink (above); "off" →
// plain coordinates only; "" → leave the individual knobs (MARCO_ANCHORS, MARCO_OCR, …)
// at their defaults. setup.ps1 -CV / -NoCV flip it so the whole CV stack toggles at once.
func cvMode() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_CV"))) {
	case "max", "1", "on", "all", "true", "yes":
		return "max"
	case "0", "off", "false", "no", "none":
		return "off"
	default:
		return ""
	}
}

func anchorsEnabled() bool {
	switch cvMode() {
	case "max":
		return true // CV explicitly on → anchors on, regardless of MARCO_ANCHORS
	case "off":
		return false // CV explicitly off → plain coordinates
	}
	// CV is a feature flag that's OFF BY DEFAULT (the anchor stack isn't reliable yet). Anchors
	// turn on only when explicitly asked via MARCO_ANCHORS.
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_ANCHORS"))) {
	case "1", "on", "true", "yes":
		return true
	}
	return false
}

func clickRadiusFromEnv() int {
	if v := os.Getenv("MARCO_CLICK_RADIUS"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 8 && n <= 1200 {
			return n
		}
	}
	// Half-size of the fallback patch (radius 100 → ~200px) used when no button is cleanly
	// detected at the click; the normal path crops to the recognised button instead. Kept
	// generous so the click-centred fallback is still distinctive. Override $MARCO_CLICK_RADIUS.
	return 100
}

// New returns the Windows recorder (global low-level keyboard + mouse hooks).
func New() Recorder { return &winRecorder{} }

// Package-level capture state — the hook callbacks are C callbacks and can only
// reach package state. Only one recorder is active at a time.
var (
	recMu         sync.Mutex
	recBuf        []RecordedEvent
	recPub        chan RecordedEvent
	recActive     atomic.Bool
	lastMoveNanos atomic.Int64
	// foreApp is the app poller's last sample, published so the HOOK can tag an event
	// with the application it landed in without making a process query of its own.
	// The hooks are synchronous and shared with the whole desktop; a lookup there is
	// how Windows comes to drop them. An empty value means no sample yet, which is a
	// real answer and is never guessed at.
	foreApp atomic.Value
)

// foreground returns the app poller's last sample, or "" before the first one.
func foreground() string {
	if v, ok := foreApp.Load().(string); ok {
		return v
	}
	return ""
}

const moveThrottle = 8 * time.Millisecond

// appPollInterval is how often the recorder samples the foreground app to detect
// switches (clicking a taskbar icon, Alt-Tab, etc.). Switches become EvAppSwitch
// events so simplify can turn "however you got to the app" into a robust
// Activate, instead of recording brittle navigation clicks.
const appPollInterval = 120 * time.Millisecond

type winRecorder struct {
	done    chan struct{}
	stopped chan struct{} // hook pump finished
	// ready closes once the pump publishes its thread id; a blocking pump is woken by a
	// posted WM_QUIT rather than by a closed channel.
	ready          chan struct{}
	threadID       atomic.Uint32
	pollStopped    chan struct{} // app poller finished
	captureStopped chan struct{} // click-template capture worker finished
	// mode is what this recorder was TOLD to capture, or Inherited when nothing was.
	//
	// Per RECORDER rather than per package, because stating a mode is something a caller
	// does about one recording. `Start` publishes it to `plainCoordinates`, which is where
	// the hook callbacks can reach it — see the capture state below.
	mode Mode
}

// Capture states what this recorder captures, overriding what the process was launched with.
//
// See recorder.Mode. Called before Start; a recorder is reusable and the mode stays until it is
// stated again, which is the same lifetime the environment-read variables have.
func (r *winRecorder) Capture(m Mode) { r.mode = m }

// plainCoordinates is "the recording that is running was told to capture coordinates only".
//
// Package-level and atomic for the same reason every other piece of capture state here is: the
// hook callbacks are C callbacks and can reach nothing but package state. Written by `Start` from
// the recorder's own `mode`, so what a CALLER stated is what the hooks read — and read on every
// click rather than once at process start, which is the whole defect this closes.
var plainCoordinates atomic.Bool

// anchoring reports whether this recording captures a picture of what was pressed.
//
// `captureAnchors` is the environment, read at process start. `plainCoordinates` is a caller
// saying so about THIS recording, and it wins: a stated mode an inherited variable could
// overrule would not be a stated mode. Every point that would capture, crop, colour-sample or
// listen for the anchor key asks this one question, so a fourth such point cannot be added
// without answering it.
func anchoring() bool { return captureAnchors && !plainCoordinates.Load() }

// captureReq asks the off-thread worker to snapshot the patch around a click and
// write it back onto the already-recorded event at idx. The capture is done off
// the hook thread because BitBlt against the live screen can spike to tens of ms
// (worse while the GPU composites an overlay); doing it inline in the LL mouse
// hook would blow past Windows' LowLevelHooksTimeout, and Windows silently drops
// a hook that's too slow — taking the keyboard hook (and thus the F12 stop key)
// down with it.
type captureReq struct {
	idx   int
	x, y  int
	armed bool // explicit anchor (tap-then-click): capture the whole screen to crop
}

// captureReqs carries click-template work from the hook thread to the worker.
// Buffered so a burst of clicks never blocks the hook; over-full just drops the
// template (the click degrades to a plain coordinate), never the click itself.
var captureReqs chan captureReq

// anchorKey is the key you TAP to start an image anchor; the next left-click ends it
// — that click is captured as an image anchor. One-handed: tap, then click, no holding.
// "" disables it. Gated on captureAnchors: with CV off (the alpha default) we don't
// listen for it at all, so F12 stays a normal key that reaches the game.
var anchorKey = func() string {
	if !captureAnchors {
		return ""
	}
	return AnchorKey()
}()

// anchorArmed is set by a tap of the anchor key and consumed by the next click,
// which is then captured as a template — even when MARCO_ANCHORS is off.
var anchorArmed atomic.Bool

// stopKey is the recording's stop gesture, read from $MARCO_STOP_KEY (default F12).
// When the anchor key is the same key, stopping wins — the key isn't treated as an
// anchor, so the demo can still be ended (see keyboardProc).
var stopKey = ParseStopKey(os.Getenv("MARCO_STOP_KEY"))

func (r *winRecorder) Start() error {
	recMu.Lock()
	recBuf = nil
	recPub = make(chan RecordedEvent, 1024)
	recMu.Unlock()
	lastMoveNanos.Store(0)
	recActive.Store(true)
	// WHAT THIS RECORDING CAPTURES, FROM THE CALLER THAT STARTED IT. Published before
	// the hooks go on, because after them the first press can already have been seen.
	plainCoordinates.Store(r.mode == Coordinates)
	// AND NOTHING IS LEFT ARMED FROM A RECORDING BEFORE THIS ONE. The arm is a one-shot
	// consumed by the next click, so an unconsumed one outlives the recording it was
	// tapped in — which would put a picture in the first click of a recording that asked
	// for none.
	anchorArmed.Store(false)

	r.done = make(chan struct{})
	r.stopped = make(chan struct{})
	r.ready = make(chan struct{})
	r.pollStopped = make(chan struct{})
	r.captureStopped = make(chan struct{})
	captureReqs = make(chan captureReq, 256)

	// Capture worker: snapshots the click target off the hook thread and writes the
	// PNG back onto the recorded event, so the synchronous LL hooks return instantly
	// (see captureReq). An explicit (armed) anchor grabs the WHOLE screen — you crop
	// it down to the target afterwards (keep the target centered, since the match
	// clicks the patch centre). Global MARCO_ANCHORS mode grabs the small auto-patch.
	// Drains on Stop.
	go func() {
		defer close(r.captureStopped)
		for req := range captureReqs {
			var img []byte
			var color string
			clickLX, clickLY := 0, 0
			newX, newY, recenter := req.x, req.y, false
			if req.armed {
				// Explicit anchor: keep the whole frame to crop AND read the pixel under
				// the click, so the anchor gets both a template and a colour resolver from
				// the one capture. The frame is the primary screen at origin (0,0), so the
				// click's position within it is just its absolute coordinate.
				if frame := captureFullFrameRGBA(); frame != nil {
					if data, err := screen.EncodePNG(frame); err == nil {
						img = data
						clickLX, clickLY = req.x, req.y
					}
					if col, ok := screen.ColorAt(frame, req.x, req.y); ok {
						color = fmt.Sprintf("0x%06X", col)
					}
				}
			} else if frame, ox, oy, err := screen.CaptureVirtual(); err == nil && frame != nil {
				// Auto-anchor: capture the WHOLE desktop so a button bigger than a fixed
				// patch fits, then crop to the button containing the click and re-centre the
				// recorded click on that button — the template's centre is the click target
				// the matcher resolves. A flat / non-distinctive spot drops the anchor (the
				// click stays a plain coordinate). ox,oy map the absolute click into the frame.
				lx, ly := req.x-ox, req.y-oy
				tmpl, cgx, cgy, clx, cly := screen.AutoCropAt(frame, lx, ly, clickTemplateRadius)
				if tmpl != nil && (cvKitchenSink || screen.Distinctive(tmpl)) {
					if col, ok := screen.ColorAt(frame, cgx, cgy); ok {
						color = fmt.Sprintf("0x%06X", col)
					}
					if data, encErr := screen.EncodePNG(tmpl); encErr == nil {
						img = data
						clickLX, clickLY = clx, cly
					}
					if nx, ny := cgx+ox, cgy+oy; nx != req.x || ny != req.y {
						newX, newY, recenter = nx, ny, true
					}
				}
			}
			if img == nil {
				continue
			}
			recMu.Lock()
			if req.idx >= 0 && req.idx < len(recBuf) {
				recBuf[req.idx].Image = img
				recBuf[req.idx].ClickX, recBuf[req.idx].ClickY = clickLX, clickLY
				if color != "" {
					recBuf[req.idx].Color = color
				}
				// Re-centre the recorded click on the button centre, shifting the
				// window-relative offset by the same delta so both stay consistent.
				if recenter {
					dx, dy := newX-recBuf[req.idx].X, newY-recBuf[req.idx].Y
					recBuf[req.idx].X, recBuf[req.idx].Y = newX, newY
					if recBuf[req.idx].WinRel {
						recBuf[req.idx].RelX += dx
						recBuf[req.idx].RelY += dy
					}
				}
			}
			recMu.Unlock()
		}
	}()

	// App-switch poller: sample the foreground app and emit EvAppSwitch on change
	// (including an initial one for the app the demo starts in). Runs off the hook
	// thread so it never stalls the synchronous LL hooks.
	go func() {
		defer close(r.pollStopped)
		last := ""
		if app := winctx.Active(); app != "" {
			last = app
			foreApp.Store(app)
			record(RecordedEvent{Kind: EvAppSwitch, KeyName: app, T: time.Now()})
		}
		t := time.NewTicker(appPollInterval)
		defer t.Stop()
		for {
			select {
			case <-r.done:
				return
			case <-t.C:
				if app := winctx.Active(); app != "" && app != last {
					last = app
					foreApp.Store(app)
					record(RecordedEvent{Kind: EvAppSwitch, KeyName: app, T: time.Now()})
				}
			}
		}
	}()

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(r.stopped)

		// Published before the hooks exist, so a Stop racing Start still has a thread to wake.
		tid, _, _ := procGetCurrentThreadId.Call()
		r.threadID.Store(uint32(tid))
		close(r.ready)

		kbCB := syscall.NewCallback(keyboardProc)
		msCB := syscall.NewCallback(mouseProc)
		kbHook, _, _ := procSetWindowsHookExW.Call(whKeyboardLL, kbCB, 0, 0)
		msHook, _, _ := procSetWindowsHookExW.Call(whMouseLL, msCB, 0, 0)

		// BLOCKING, for the reason set out in internal/platform/navsource: Windows can only
		// deliver a low-level hook callback while the installing thread waits on messages, and
		// PeekMessage with a Sleep between polls leaves it asleep instead — which puts up to a
		// scheduler quantum of latency on every keystroke and every mouse move on the whole
		// desktop. It is not a dropped hook and never looks like one; it just feels heavy.
		var m msg
		for {
			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if ret == 0 || int32(ret) == -1 {
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
		if kbHook != 0 {
			procUnhookWindowsHookEx.Call(kbHook)
		}
		if msHook != 0 {
			procUnhookWindowsHookEx.Call(msHook)
		}
	}()
	return nil
}

func (r *winRecorder) Stop() []RecordedEvent {
	if r.done == nil {
		return nil
	}
	close(r.done)
	// WAKE THE BLOCKING PUMP, AND KEEP KNOCKING UNTIL IT ANSWERS.
	//
	// Only the installing thread may unhook what it installed, and it is asleep in
	// GetMessage until a message arrives — so shutdown is a message rather than a flag.
	//
	// THE POST CAN LAND NOWHERE. A Windows thread has no message queue until it first
	// calls a message function, and PostThreadMessage to a thread without one fails. The
	// id is published before the hooks exist so that a Stop racing a Start still has
	// somewhere to aim, and that is the window: a single unchecked post into it is lost,
	// GetMessage waits forever for a message that will never come, and so does this.
	//
	// It is not hypothetical and it was not cheap. `cmd/marco` hit its 600-second limit
	// twice on this, both times recorded as an unexplained timeout with contention as the
	// guess — and the same defect had already been found and fixed in
	// `internal/platform/navsource`, whose own suite ran 495s before being killed. This
	// is that fix carried to the site that still had it. Every real-input run goes through
	// it: `withPanicStop` starts a recording to watch for the stop key and stops it when
	// the play ends, so a short play is exactly the shape that posts into the window.
	//
	// Bounded, because a pump that has somehow already died must not wedge the caller
	// either, and because a shutdown that cannot be completed is a thing to SAY.
	<-r.ready
	woken := false
	deadline := time.Now().Add(5 * time.Second)
	for {
		procPostThreadMessageW.Call(uintptr(r.threadID.Load()), wmQuit, 0, 0)
		select {
		case <-r.stopped: // hook pump unhooked — no more mouseProc, so no more capture sends
			woken = true
		case <-time.After(20 * time.Millisecond):
		}
		// The deadline is checked HERE and not in the select's case, where a `break`
		// would leave the select and go round the loop again forever.
		if woken || time.Now().After(deadline) {
			break
		}
	}
	<-r.pollStopped // watches r.done, so it finishes either way
	if !woken {
		// REPORTED, NOT RECOVERED, and deliberately not forced. The hooks can only be
		// removed by a thread that is not answering, and closing captureReqs while a
		// callback can still send into it would panic inside a hook — which is a worse
		// end to a run than a hook that outlives it. So: stop recording, say what is
		// left behind, and hand back what was captured.
		recActive.Store(false)
		fmt.Fprintf(os.Stderr, "\n[marco: this recording's keyboard and mouse hooks could "+
			"not be taken off — the thread that installed them did not answer. They stay "+
			"installed until this process exits. Nothing more is being recorded.]\n")
		r.done = nil
		recMu.Lock()
		out := recBuf
		recBuf = nil
		recMu.Unlock()
		resolveImages(out, winctx.ImageOf)
		return out
	}
	close(captureReqs) // safe now: nothing sends after the hooks are gone
	<-r.captureStopped // let queued templates finish writing back before we read recBuf
	recActive.Store(false)
	recMu.Lock()
	out := recBuf
	recBuf = nil
	recMu.Unlock()
	// AND THE PROCESS IMAGES, NOW THAT THERE IS NO HOOK LEFT TO SLOW DOWN.
	//
	// The hooks are unhooked by this point, so opening a process handle costs nobody
	// anything. Doing it here rather than in the callback is the whole reason the
	// process id is recorded at the press and the name is not. See resolveImages.
	resolveImages(out, winctx.ImageOf)
	r.done = nil
	return out
}

func (r *winRecorder) Events() <-chan RecordedEvent {
	recMu.Lock()
	defer recMu.Unlock()
	return recPub
}

// record appends an event and forwards it to the live channel (non-blocking).
func record(ev RecordedEvent) { recordIndexed(ev) }

// recordIndexed is record that also returns the event's index in recBuf, so the
// capture worker can later fill in a click's Image in place.
func recordIndexed(ev RecordedEvent) int {
	recMu.Lock()
	idx := len(recBuf)
	recBuf = append(recBuf, ev)
	pub := recPub
	recMu.Unlock()
	if pub != nil {
		select {
		case pub <- ev:
		default:
		}
	}
	return idx
}

// keyboardProc is the WH_KEYBOARD_LL callback. It records but never suppresses —
// the user's keystrokes must reach the app they're demonstrating in.
func keyboardProc(nCode int, wParam, lParam uintptr) uintptr {
	if nCode >= 0 && recActive.Load() {
		kb := (*kbdllHookStruct)(unsafe.Pointer(lParam))
		if kb.flags&llkhfInjected == 0 {
			down := wParam == wmKeyDown || wParam == wmSysKeyDown
			up := wParam == wmKeyUp || wParam == wmSysKeyUp
			if down || up {
				vk := uint16(kb.vkCode)
				name := vkToName(vk)
				// The anchor key is a recorder-only modifier: a TAP arms an anchor (the
				// next click is captured), so it's never recorded and is swallowed so the
				// app never sees it. Skip this when the anchor key is also the stop key —
				// then it must fall through to be recorded so the demo can be stopped.
				// AND THE ANCHOR KEY IS AN ORDINARY KEY IN A COORDINATE
				// RECORDING. Belt and braces, recorded as such: the gate
				// in `recordClickDown` already refuses to honour an arm in
				// this mode, so a mutation removing THIS one changes no
				// behaviour and the gate run for it is recorded as
				// unfalsifiable rather than as a kill. It stays because it
				// is the honest expression of the rule at the point where
				// the key is read, and because the two gates protect
				// different things if either ever moves.
				if anchoring() && anchorKey != "" && name == anchorKey && !stopKey.Has(name) {
					if down {
						anchorArmed.Store(true) // tap to start; the next click ends it
					}
					return 1 // suppress — never reaches the app
				}
				record(RecordedEvent{Kind: EvKey, VK: vk, KeyName: name,
					Down: down, T: time.Now(), App: foreground()})
			}
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

// mouseProc is the WH_MOUSE_LL callback. Moves are throttled in-callback to keep
// the synchronous hook from stalling.
func mouseProc(nCode int, wParam, lParam uintptr) uintptr {
	if nCode >= 0 && recActive.Load() {
		ms := (*msllHookStruct)(unsafe.Pointer(lParam))
		if ms.flags&llmhfInjected == 0 {
			x, y := int(ms.pt.x), int(ms.pt.y)
			switch wParam {
			case wmMouseMove:
				now := time.Now().UnixNano()
				if now-lastMoveNanos.Load() >= int64(moveThrottle) {
					lastMoveNanos.Store(now)
					record(RecordedEvent{Kind: EvMove, X: x, Y: y, T: time.Now()})
				}
			case wmLButtonDown:
				recordClickDown("left", x, y)
			case wmLButtonUp:
				record(newClick("left", false, x, y))
			case wmRButtonDown:
				recordClickDown("right", x, y)
			case wmRButtonUp:
				record(newClick("right", false, x, y))
			case wmMButtonDown:
				recordClickDown("middle", x, y)
			case wmMButtonUp:
				record(newClick("middle", false, x, y))
			}
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

// recordClickDown records a button-press click (fast) and captures its template
// off-thread — never BitBlt inside the hook (see captureReq) — when anchors are on
// globally (left clicks, or EVERY button in kitchen-sink mode) OR the anchor key armed
// this click (tap-then-click, any button). An armed click ends the anchor and disarms. A
// full capture queue just drops the template; the click itself is already recorded.
func recordClickDown(button string, x, y int) {
	idx := recordIndexed(newClick(button, true, x, y))
	// ── A STATED COORDINATE MODE TAKES THE ARM WITH IT, AND NOTHING ELSE CHANGES ──
	//
	// The arm is the one path that captures a picture of a click the global switch would
	// not have anchored — a tap of the anchor key makes the next click an anchor whatever
	// button it is. So a gate that asked only about `captureAnchors` would have left
	// exactly one way for a coordinate recording to come back carrying a picture.
	//
	// It is `plainCoordinates` here and not `anchoring()`, and the difference is a defect
	// this very mutation gate found: `anchoring()` is the conjunction, so using it would
	// have made an armed click do nothing whenever the switch was off, taking the
	// tap-then-click anchor away from every caller instead of from this recording. With
	// no mode stated, both lines below are what they were before, term for term.
	armed := !plainCoordinates.Load() && anchorArmed.Load()
	if armed || (anchoring() && (button == "left" || cvKitchenSink)) {
		select {
		case captureReqs <- captureReq{idx: idx, x: x, y: y, armed: armed}:
		default:
		}
	}
	if armed {
		anchorArmed.Store(false) // consume the one-shot arm
	}
}

// clickAttribution answers WHOSE a click is: the foreground window at this instant, the process
// owning it, and the app the poller last sampled.
//
// It hands back the HANDLE as well as the process because `clickShape` below needs the same
// window this answered about. Asking `GetForegroundWindow` twice on one click would be two reads
// of a thing that can change between them — and the second answer would then describe a window
// the first one did not attribute.
//
// A variable so a test can drive the production builder with a known desktop. Nothing else
// replaces it, and the hook calls it on every click edge.
var clickAttribution = func() (hwnd uintptr, pid uint32, app string) {
	hwnd, pid, _ = winctx.ForegroundProcess()
	return hwnd, pid, foreground()
}

// clickShape answers WHAT THAT WINDOW WAS LIKE: its origin, its outer size and its DPI.
//
// Separate from the attribution above because the two are asked at different times — the
// attribution on both click edges, this only on the press — and replaceable for the same reason:
// a real answer needs a real window on a real desktop, and what the builder does with the answer
// is what these tests are about.
var clickShape = winctx.WindowShape

// newClick builds an EvClick event.
//
// # Attribution is on BOTH edges, geometry is on the press only
//
// They are separated deliberately, and the separation is the whole of this comment. The offset
// from the window's top-left is about WHERE the click goes, and only a press has a where worth
// keeping — a release lands wherever the press did. Attribution is about WHOSE the click is, and
// both edges have an owner.
//
// Stamping the press alone is what a first version did, and it reopened the defect it was added
// to close. A button is activated on RELEASE, so the tail of a recording stopped by one ends
// with an edge carrying no owner at all; the strip walking back from the end meets that
// unattributed release, cannot place it, and stops — never reaching the correctly stamped press
// one event behind it. The fixtures stamped both edges, so they described a stream the hook does
// not produce and the strip looked fine.
//
// GetForegroundWindow, GetWindowRect and GetWindowThreadProcessId are all window-data reads:
// cheap, and safe to call on the hook thread. None of them opens a process handle.
func newClick(button string, down bool, x, y int) RecordedEvent {
	hwnd, pid, app := clickAttribution()
	ev := RecordedEvent{Kind: EvClick, Button: button, Down: down, X: x, Y: y,
		T: time.Now(), App: app, PID: pid, HWnd: uintptr(hwnd)}
	// THE WINDOW AS IT WAS AT THIS EDGE — where it is, how big it is on the outside, and
	// what DPI it is drawn at, from the window the attribution above already identified.
	// At the edge itself, because the edge is the only moment it can be read: anything
	// later is a sample, and a sample cannot answer about an instant it missed.
	//
	// The process IMAGE is deliberately not read here. That opens a process handle, which
	// is the work that makes Windows drop the hooks, so it is resolved from `PID` once the
	// capture is over. See `resolveImages`.
	//
	// ── BOTH EDGES, AND THAT IS WHAT A DRAG NEEDS ──
	//
	// The release used to be measured not at all: a release lands wherever the press did,
	// so for a CLICK it is about no rectangle of its own. A DRAG ends somewhere else, and
	// its destination is a point in a window just as much as its start is — so deriving
	// the destination's offset from the PRESS's rectangle would subtract a rectangle
	// measured at one instant from a point captured at another, with nothing able to tell
	// whether the window moved in between. That is an offset into a window as it WAS,
	// attributed to a point from after, and it would pass every check at replay.
	//
	// So the release is measured the same way, `simplify` requires the two edges to agree
	// about the window, and a gesture that spans two different rectangles is refused
	// rather than reconciled. The cost is one more `clickShape` per mouse event in the
	// hook: `GetWindowRect` plus `GetDpiForWindow`, window data Windows already holds, no
	// process handle. The same class of call the attribution above already makes on both
	// edges.
	//
	// ── THE CONTRACT IS DECLARED FOR EVERY EDGE, BEFORE ANY OF IT IS READ ──
	//
	// `RelTo` names the rectangle this edge is ABOUT: the one its offset is measured from
	// and the one it is checked against immediately before input. It is a statement of
	// what the edge MEANS, not one of the measurements, so it goes in whether or not the
	// measurements arrive.
	//
	// That ordering is the whole of this comment, because the other one was wrong.
	// Writing the basis only when every term arrived meant a press whose window would not
	// report its size, or whose scale Windows will not report (no `GetDpiForWindow` before
	// Windows 10 1607), or whose program had exited before the hooks came down, was saved
	// as an ORDINARY point — and an ordinary point is pressed at its recorded coordinate
	// having checked nothing. A failed measurement cannot be what makes the safeguards
	// optional: it means guarded replay is unavailable for that recording, and the
	// recording has to say so rather than quietly become something that runs.
	//
	// So: the basis always, each measurement only when it was taken, missing left
	// missing, and replay refuses until they are all there. Routes already on disk carry
	// no basis and are untouched.
	ev.RelTo = macroir.RelToWindow
	if left, top, w, h, dpi, ok := clickShape(hwnd); ok {
		ev.Dpi = dpi // 0 when Windows would not say, and 0 stays 0
		if w > 0 && h > 0 {
			ev.WinW, ev.WinH = w, h
			// AND WHERE THAT RECTANGLE WAS, which is not a replay target and is
			// never emitted into a route. It is here so `simplify` can ask
			// whether the two edges of a drag are about the SAME rectangle —
			// a question an offset cannot answer, because an offset is what is
			// left after the origin has been subtracted away.
			ev.WinX, ev.WinY = left, top
			// AND THE OFFSET IS A MEASUREMENT OF THAT WINDOW ONLY IF THE EDGE
			// WAS IN IT. A click that CHANGED the foreground looks like this
			// from inside the hook: the attribution names the window that was
			// in front at that instant and the click landed in the one it was
			// about to raise, so the subtraction produces a number that is not
			// an offset into anything.
			if x >= left && y >= top && x-left < w && y-top < h {
				ev.RelX, ev.RelY, ev.WinRel = x-left, y-top, true
			}
		}
	}
	if down {
		ev.Window = winctx.ForegroundTitle() // context: which window was clicked
	}
	return ev
}

// captureFullFrameRGBA grabs the whole primary screen — the anchor image for an
// explicit (armed) anchor, which you then crop down to the target (until cropped it
// won't match the changed live screen, so the click falls back to its recorded
// coordinate). The raw frame is returned (not yet PNG) so the worker can also read
// the pixel under the click for the colour resolver. Best-effort: nil on any failure.
func captureFullFrameRGBA() *image.RGBA {
	w, h := screen.PrimarySize()
	if w <= 0 || h <= 0 {
		return nil
	}
	img, err := screen.CaptureRegion(0, 0, w, h)
	if err != nil || img == nil {
		return nil
	}
	return img
}

// Available says whether this platform has an input-capture backend at all.
//
// A PLATFORM fact and nothing more. It does not say the hooks will install: that can only be
// known by installing them, which takes the keyboard and the mouse of the whole desktop, so a
// readiness check claiming it would be claiming something it had not done. See Start, and the
// note on `anchorKey` for what Windows does to a hook it does not like.
func Available() bool { return true }
