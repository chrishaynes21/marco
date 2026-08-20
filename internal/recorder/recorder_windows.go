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
// flag: teaching records plain coordinates + timings unless CV is explicitly turned on
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
)

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
}

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
	// Wake the blocking pump so it can unhook what it installed — only the installing
	// thread may do that, and it is asleep in GetMessage until something arrives.
	<-r.ready
	procPostThreadMessageW.Call(uintptr(r.threadID.Load()), wmQuit, 0, 0)
	<-r.stopped // hook pump unhooked — no more mouseProc, so no more capture sends
	<-r.pollStopped
	close(captureReqs) // safe now: nothing sends after the hooks are gone
	<-r.captureStopped // let queued templates finish writing back before we read recBuf
	recActive.Store(false)
	recMu.Lock()
	out := recBuf
	recBuf = nil
	recMu.Unlock()
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
				if anchorKey != "" && name == anchorKey && !stopKey.Has(name) {
					if down {
						anchorArmed.Store(true) // tap to start; the next click ends it
					}
					return 1 // suppress — never reaches the app
				}
				record(RecordedEvent{Kind: EvKey, VK: vk, KeyName: name, Down: down, T: time.Now()})
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
	armed := anchorArmed.Load()
	if armed || (captureAnchors && (button == "left" || cvKitchenSink)) {
		select {
		case captureReqs <- captureReq{idx: idx, x: x, y: y, armed: armed}:
		default:
		}
	}
	if armed {
		anchorArmed.Store(false) // consume the one-shot arm
	}
}

// newClick builds an EvClick event. On a press it also records the click's offset
// from the foreground window's top-left (WinRel), so codegen can emit a click that
// follows the active window to whatever monitor/position it's on — the portable
// default. GetForegroundWindow + GetWindowRect are cheap, safe to call in the hook.
func newClick(button string, down bool, x, y int) RecordedEvent {
	ev := RecordedEvent{Kind: EvClick, Button: button, Down: down, X: x, Y: y, T: time.Now()}
	if down {
		if left, top, ok := winctx.ForegroundOrigin(); ok {
			ev.RelX, ev.RelY, ev.WinRel = x-left, y-top, true
		}
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
