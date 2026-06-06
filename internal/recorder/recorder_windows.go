//go:build windows

package recorder

import (
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

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

type winRecorder struct {
	done    chan struct{}
	stopped chan struct{}
}

func (r *winRecorder) Start() error {
	recMu.Lock()
	recBuf = nil
	recPub = make(chan RecordedEvent, 1024)
	recMu.Unlock()
	lastMoveNanos.Store(0)
	recActive.Store(true)

	r.done = make(chan struct{})
	r.stopped = make(chan struct{})

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
	<-r.stopped
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
func record(ev RecordedEvent) {
	recMu.Lock()
	recBuf = append(recBuf, ev)
	pub := recPub
	recMu.Unlock()
	if pub != nil {
		select {
		case pub <- ev:
		default:
		}
	}
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
				record(RecordedEvent{Kind: EvClick, Button: "left", Down: true, X: x, Y: y, T: time.Now()})
			case wmLButtonUp:
				record(RecordedEvent{Kind: EvClick, Button: "left", Down: false, X: x, Y: y, T: time.Now()})
			case wmRButtonDown:
				record(RecordedEvent{Kind: EvClick, Button: "right", Down: true, X: x, Y: y, T: time.Now()})
			case wmRButtonUp:
				record(RecordedEvent{Kind: EvClick, Button: "right", Down: false, X: x, Y: y, T: time.Now()})
			case wmMButtonDown:
				record(RecordedEvent{Kind: EvClick, Button: "middle", Down: true, X: x, Y: y, T: time.Now()})
			case wmMButtonUp:
				record(RecordedEvent{Kind: EvClick, Button: "middle", Down: false, X: x, Y: y, T: time.Now()})
			}
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}
