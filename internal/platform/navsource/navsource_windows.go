//go:build windows

package navsource

import (
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// The Windows low-level keyboard hook, doing as close to nothing as a hook can.
//
// The recorder in internal/recorder already runs this pattern and its comments explain why:
// Windows silently drops a hook that overruns LowLevelHooksTimeout, and a dropped keyboard hook
// takes the F12 stop key down with it. That failure is invisible — no error, no log, the key
// simply stops working — which is why the constraint is honoured structurally here rather than
// measured.
//
// This is a SEPARATE hook from the recorder's, and that is correct rather than a duplication:
// they live in different processes (marco.exe during a demonstration, director.exe during an
// observation session). Windows supports multiple low-level hooks from different processes; the
// rule that matters is per-callback latency, not global uniqueness. Nothing here suppresses an
// event — every keystroke reaches the game exactly as it would have.

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadId  = kernel32.NewProc("GetCurrentThreadId")
)

const (
	whKeyboardLL  = 13
	whMouseLL     = 14
	wmKeyDown     = 0x0100
	wmKeyUp       = 0x0101
	wmSysKeyDown  = 0x0104
	wmSysKeyUp    = 0x0105
	wmQuit        = 0x0012
	llkhfInjected = 0x00000010
	// llmhfInjected is LLMHF_INJECTED, the mouse struct's own injected flag.
	llmhfInjected = 0x00000001

	// The only two pointer messages this package knows.
	//
	// Deliberately NOT WM_MOUSEMOVE. A movement stream is a pointer trail — a continuous
	// record of where somebody's hand went — and it is on the list of things the durable
	// record may never contain. The way to guarantee that is to never receive it: there is
	// no branch below that could admit a move, so no configuration, no flag and no later
	// edit can turn one on without adding a case somebody has to write on purpose.
	//
	// Button-UP is absent for the same reason key-up is: emitting on both edges would
	// double every press, and a release carries no meaning the press did not.
	wmLButtonDown = 0x0201
	wmRButtonDown = 0x0204
)

type kbdllHookStruct struct {
	vkCode      uint32
	scanCode    uint32
	flags       uint32
	time        uint32
	dwExtraInfo uintptr
}

// msllHookStruct is MSLLHOOKSTRUCT. Only `pt` and `flags` are read.
type msllHookStruct struct {
	pt          struct{ x, y int32 }
	mouseData   uint32
	flags       uint32
	time        uint32
	dwExtraInfo uintptr
}

type msg struct {
	hwnd   uintptr
	msg    uint32
	wParam uintptr
	lParam uintptr
	time   uint32
	pt     struct{ x, y int32 }
}

// The callback is a C function pointer and cannot carry a receiver, so the active offer
// function is package state. Guarded by an atomic because it is read from the hook thread and
// written from an ordinary one.
//
// There is no drop counter here. Drops are counted by the Source inside offer, because a
// package-level one would be process-global and two Sources would report each other's
// backpressure as their own.
var (
	hookOffer  atomic.Pointer[func(rawEvent) bool]
	hookActive atomic.Bool
)

type winBackend struct {
	once    sync.Once
	done    chan struct{}
	stopped chan struct{}
	// ready closes once the pump has published its thread id, so stop has somewhere to
	// post WM_QUIT. A blocking pump cannot poll for shutdown.
	ready    chan struct{}
	threadID atomic.Uint32
}

func newBackend() backend { return &winBackend{} }

func (b *winBackend) unavailable() string { return "" }

func (b *winBackend) start(offer func(rawEvent) bool) error {
	b.done = make(chan struct{})
	b.stopped = make(chan struct{})
	b.ready = make(chan struct{})
	hookOffer.Store(&offer)
	hookActive.Store(true)

	go func() {
		// The hook must be installed on a thread with a message pump, and must be
		// unhooked from the same thread that installed it.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(b.stopped)

		// Published BEFORE the hooks exist, so a stop racing a start still has a thread to
		// wake and cannot leave a hook installed with nobody able to remove it.
		tid, _, _ := procGetCurrentThreadId.Call()
		b.threadID.Store(uint32(tid))
		close(b.ready)

		// Two hooks, one thread, one pump.
		//
		// They must share the thread that installs them and be unhooked from it, and a
		// second pump would be a second latency budget to blow. Each callback does the
		// same near-nothing: read the struct, offer, chain.
		//
		// A failed mouse hook is not fatal. Keyboard navigation is the older and more
		// load-bearing evidence, and losing pointer presses degrades what Marco can
		// correlate rather than what it can see.
		kbCB := syscall.NewCallback(keyboardProc)
		hook, _, _ := procSetWindowsHookExW.Call(whKeyboardLL, kbCB, 0, 0)
		mouseCB := syscall.NewCallback(mouseProc)
		mouseHook, _, _ := procSetWindowsHookExW.Call(whMouseLL, mouseCB, 0, 0)

		// A BLOCKING pump, and the blocking is the whole point.
		//
		// Windows delivers a low-level hook callback to the thread that installed the hook, and
		// it can only do that while that thread is waiting on messages. GetMessage waits in the
		// kernel in exactly that state, so the callback runs the moment an event arrives.
		//
		// This used to be PeekMessage in a loop with Sleep(1) between polls, which looks
		// equivalent and is not: the thread then spends nearly all of its time inside Sleep
		// rather than waiting on messages, so every mouse move and every keystroke ON THE WHOLE
		// DESKTOP waits for it to come back round. Sleep(1) is not a millisecond either — the
		// default timer resolution is about 15.6ms, so each nap is up to a full quantum.
		//
		// Reported live as a heavy cursor and keys that hang, on both devices at once, which is
		// the signature of this rather than of a slow callback: the two hooks share this one
		// thread, so they stall together.
		//
		// It never showed up as a broken hook because it is not one. Windows only unhooks past
		// LowLevelHooksTimeout (300ms); below that it simply adds the latency and carries on,
		// so the "callbacks must return fast or the hooks get dropped" rule this package is
		// careful about was satisfied the entire time. plugins/overlay has always used
		// GetMessage; this is that pump, and nothing more.
		var m msg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			// 0 is WM_QUIT — the shutdown below. -1 is an error, and continuing would spin.
			if r == 0 || int32(r) == -1 {
				break
			}
		}
		if hook != 0 {
			procUnhookWindowsHookEx.Call(hook)
		}
		if mouseHook != 0 {
			procUnhookWindowsHookEx.Call(mouseHook)
		}
	}()
	return nil
}

// stop unhooks, by waking the pump rather than by polling a flag.
//
// A blocking GetMessage cannot notice a closed channel, so shutdown is a message: WM_QUIT posted
// to the pump's own thread, which is the only thing that may unhook what it installed.
func (b *winBackend) stop() {
	b.once.Do(func() {
		hookActive.Store(false)
		if b.done == nil {
			hookOffer.Store(nil)
			return
		}
		close(b.done)
		// The pump publishes its thread id before installing anything, so by the time a
		// caller can observe a running source there is somewhere to post to. Waiting is
		// bounded: a source that never got as far as a thread has nothing to unhook.
		select {
		case <-b.ready:
		case <-time.After(2 * time.Second):
			hookOffer.Store(nil)
			return
		}
		// RETRIED, and the wait is bounded, because the post can legitimately fail.
		//
		// A Windows thread has no message queue until it first calls a message function, and
		// the id is published before GetMessage runs so that a stop racing a start still has
		// somewhere to aim. That leaves a window in which PostThreadMessage lands nowhere —
		// and waiting unconditionally for a pump that was never woken hangs forever. It did:
		// the package's own suite ran 495s before being killed.
		//
		// Retrying costs nothing and closes the window; the outer bound means a pump that has
		// somehow already died cannot wedge a caller either.
		deadline := time.Now().Add(2 * time.Second)
		for {
			procPostThreadMessageW.Call(uintptr(b.threadID.Load()), wmQuit, 0, 0)
			select {
			case <-b.stopped:
				hookOffer.Store(nil)
				return
			case <-time.After(20 * time.Millisecond):
			}
			if time.Now().After(deadline) {
				break
			}
		}
		hookOffer.Store(nil)
	})
}

// keyboardProc is the WH_KEYBOARD_LL callback.
//
// Everything it does is visible in one screen: bounds-check, read the struct, offer, chain. No
// lock, no allocation, no map, no classification, no logging, no Director call. The key code it
// reads is passed straight into a bounded channel and the frame is gone.
//
// It NEVER suppresses. Returning 1 here would eat the user's keystroke, and a passive observer
// that changed what the game received would not be passive.
func keyboardProc(nCode int, wParam, lParam uintptr) uintptr {
	if nCode >= 0 && hookActive.Load() {
		if fn := hookOffer.Load(); fn != nil {
			kb := (*kbdllHookStruct)(unsafe.Pointer(lParam))
			// Injected events are Marco's own input, or another tool's. Attributing them
			// to the player would make the Director correlate its own actions with the
			// screen changes they caused and call it discovery.
			if kb.flags&llkhfInjected == 0 {
				down := wParam == wmKeyDown || wParam == wmSysKeyDown
				up := wParam == wmKeyUp || wParam == wmSysKeyUp
				if down || up {
					// The return value is deliberately unused: offer counts its own
					// refusals, and there is nothing this thread may do about one.
					(*fn)(rawEvent{code: uint16(kb.vkCode), down: down, at: time.Now()})
				}
			}
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

// mouseProc is the WH_MOUSE_LL callback.
//
// The same shape as keyboardProc and for the same reason: bounds-check, read the struct, offer,
// chain. No lock, no allocation, no window lookup, no normalisation. Where the press lands
// relative to the watched window is decided on the worker, because answering that question needs
// the OS, and asking the OS anything from a low-level hook is how the hook gets dropped.
//
// It NEVER suppresses. Returning 1 here would eat the user's click.
//
// Only two messages reach the offer. A move is not in the switch, so a pointer trail cannot be
// produced by this process even by mistake — see the note on wmLButtonDown.
func mouseProc(nCode int, wParam, lParam uintptr) uintptr {
	if nCode >= 0 && hookActive.Load() {
		switch wParam {
		case wmLButtonDown, wmRButtonDown:
			if fn := hookOffer.Load(); fn != nil {
				ms := (*msllHookStruct)(unsafe.Pointer(lParam))
				// Injected events are Marco's own input, or another tool's. Attributing
				// them to the user would make the Director correlate its own clicks with
				// the screen changes they caused and call it discovery.
				if ms.flags&llmhfInjected == 0 {
					(*fn)(rawEvent{
						pointer: true, down: true,
						x: ms.pt.x, y: ms.pt.y, at: time.Now(),
					})
				}
			}
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}
