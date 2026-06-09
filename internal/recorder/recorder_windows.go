//go:build windows

package recorder

import (
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

// clickTemplateRadius is the half-size of the square patch captured around a
// left-click, so codegen can later locate the target by image. The patch is
// centered on the click, so locating it at run time and clicking its center
// reproduces the exact click — the click position is inferred from the image.
//
// Default: a patch ~1/8 of the screen wide (radius = primary-width / 16), so it
// takes in the target PLUS enough surrounding context (neighbouring controls,
// panel borders, labels) to be distinctive and locatable when the UI moves, and
// scales with resolution. A small patch is often a flat, non-distinctive interior
// that matches everywhere and falls back to a fixed coordinate. The score-based
// matcher tolerates the mouse cursor at the centre — a small fraction of the area.
// Tune with $MARCO_CLICK_RADIUS (pixels; the patch is 2× this), which overrides
// the screen-relative default.
var clickTemplateRadius = clickRadiusFromEnv()

// captureAnchors gates image-template capture on clicks. Default OFF: recordings
// use plain coordinates (points) — exact and predictable for the same layout. Set
// $MARCO_ANCHORS=1 to also capture a template per click so codegen emits an
// image-find click that survives the UI moving, at the cost of match accuracy.
var captureAnchors = os.Getenv("MARCO_ANCHORS") != ""

func clickRadiusFromEnv() int {
	if v := os.Getenv("MARCO_CLICK_RADIUS"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 8 && n <= 600 {
			return n
		}
	}
	if w, _ := screen.PrimarySize(); w > 0 {
		return w / 16 // patch side = w/8 ≈ an eighth of the screen across
	}
	return 120 // fallback (~1/8 of a 1080p width) when metrics are unavailable
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
	done           chan struct{}
	stopped        chan struct{} // hook pump finished
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
	idx  int
	x, y int
}

// captureReqs carries click-template work from the hook thread to the worker.
// Buffered so a burst of clicks never blocks the hook; over-full just drops the
// template (the click degrades to a plain coordinate), never the click itself.
var captureReqs chan captureReq

func (r *winRecorder) Start() error {
	recMu.Lock()
	recBuf = nil
	recPub = make(chan RecordedEvent, 1024)
	recMu.Unlock()
	lastMoveNanos.Store(0)
	recActive.Store(true)

	r.done = make(chan struct{})
	r.stopped = make(chan struct{})
	r.pollStopped = make(chan struct{})
	r.captureStopped = make(chan struct{})
	captureReqs = make(chan captureReq, 256)

	// Click-template capture worker: snapshots the patch around each left-click
	// off the hook thread and writes the PNG back onto the recorded event, so the
	// synchronous LL hooks return instantly (see captureReq). Drains on Stop.
	go func() {
		defer close(r.captureStopped)
		for req := range captureReqs {
			img := captureClickTemplate(req.x, req.y)
			if img == nil {
				continue
			}
			recMu.Lock()
			if req.idx >= 0 && req.idx < len(recBuf) {
				recBuf[req.idx].Image = img
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

		kbCB := syscall.NewCallback(keyboardProc)
		msCB := syscall.NewCallback(mouseProc)
		kbHook, _, _ := procSetWindowsHookExW.Call(whKeyboardLL, kbCB, 0, 0)
		msHook, _, _ := procSetWindowsHookExW.Call(whMouseLL, msCB, 0, 0)

		var m msg
		for {
			select {
			case <-r.done:
				if kbHook != 0 {
					procUnhookWindowsHookEx.Call(kbHook)
				}
				if msHook != 0 {
					procUnhookWindowsHookEx.Call(msHook)
				}
				return
			default:
			}
			ret, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, pmRemove)
			if ret != 0 {
				procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
				procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
			} else {
				procSleep.Call(1)
			}
		}
	}()
	return nil
}

func (r *winRecorder) Stop() []RecordedEvent {
	if r.done == nil {
		return nil
	}
	close(r.done)
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
				record(RecordedEvent{Kind: EvKey, VK: vk, KeyName: vkToName(vk), Down: down, T: time.Now()})
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
				// Record the click now (fast) and capture its template off-thread —
				// never BitBlt inside the hook (see captureReq). A full capture queue
				// just drops the template; the click is already recorded.
				idx := recordIndexed(newClick("left", true, x, y))
				if captureAnchors {
					select {
					case captureReqs <- captureReq{idx: idx, x: x, y: y}:
					default:
					}
				}
			case wmLButtonUp:
				record(newClick("left", false, x, y))
			case wmRButtonDown:
				record(newClick("right", true, x, y))
			case wmRButtonUp:
				record(newClick("right", false, x, y))
			case wmMButtonDown:
				record(newClick("middle", true, x, y))
			case wmMButtonUp:
				record(newClick("middle", false, x, y))
			}
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
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
	}
	return ev
}

// captureClickTemplate grabs a generous fixed patch around a click (see
// clickTemplateRadius) and returns it as PNG bytes, or nil for a near-uniform area
// (blank background, not worth matching) so the click stays a plain coordinate.
// No auto-trim: the patch is saved as-is — predictable, and the user can crop the
// PNG beside the route for a tighter match. Best-effort: any failure returns nil.
func captureClickTemplate(x, y int) []byte {
	// Capture centered on the actual click — coords may be negative on a monitor
	// left of / above the primary; the virtual-desktop capture handles that.
	r := clickTemplateRadius
	img, err := screen.CaptureRegion(x-r, y-r, r*2, r*2)
	if err != nil || img == nil || !screen.Distinctive(img) {
		return nil
	}
	data, err := screen.EncodePNG(img)
	if err != nil {
		return nil
	}
	return data
}
