//go:build windows

package winctx

import (
	"sync"
	"syscall"
	"unsafe"
)

// Window liveness: the questions that must be asked of the operating system rather than
// of a cache.
//
// A handle is valid for exactly as long as the window exists. Windows then reuses the
// number. Nothing in the Director could previously tell a live window from a destroyed one
// whose rectangle it happened to remember — which is how a capture ended up reading another
// monitor and calling the result Rocket League.

var (
	procIsWindow           = user32.NewProc("IsWindow")
	procGetWindowTextLenW  = user32.NewProc("GetWindowTextLengthW")
	procGetExitCodeProcess = kernel32.NewProc("GetExitCodeProcess")
	procQueryFullProcName  = kernel32.NewProc("QueryFullProcessImageNameW")
)

const stillActive = 259

// IsWindow reports whether the handle currently identifies a window.
//
// The single cheapest defence against the whole class of stale-handle bugs, and the one
// nothing was asking.
func IsWindow(hwnd uintptr) bool {
	r, _, _ := procIsWindow.Call(hwnd)
	return r != 0
}

// WindowProcessID returns the process that owns a window, or 0.
//
// What makes handle recycling detectable: the same number owned by a different process is
// a different window, and no amount of geometry checking would notice.
func WindowProcessID(hwnd uintptr) uint32 {
	var pid uint32
	procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid
}

// ProcessAlive reports whether a process currently exists and has not exited.
//
// Opening the process is not enough on its own: a handle to an exited process still opens
// while anything holds a reference to it, and reports its exit code rather than failing.
func ProcessAlive(pid uint32) bool {
	if pid == 0 {
		return false
	}
	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return false
	}
	defer procCloseHandle.Call(h)

	var code uint32
	if ok, _, _ := procGetExitCodeProcess.Call(h, uintptr(unsafe.Pointer(&code))); ok == 0 {
		// The process exists but will not answer. Unknown, and unknown is not alive —
		// the safe direction for a predicate that authorises a screen capture.
		return false
	}
	return code == stillActive
}

// ProcessImage returns a process's executable name, lowercased and without its path.
//
// The application identity that survives a restart. A new instance of the same program has
// a new process ID and a new window, and only this is the same.
func ProcessImage(pid uint32) string {
	if pid == 0 {
		return ""
	}
	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)

	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	r, _, _ := procQueryFullProcName.Call(h, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r == 0 || size == 0 {
		return ""
	}
	full := syscall.UTF16ToString(buf[:size])
	return baseName(full)
}

// baseName strips a path and the .exe suffix, lowercased.
func baseName(path string) string {
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '\\' || path[i] == '/' {
			start = i + 1
		}
	}
	name := path[start:]
	if n := len(name); n > 4 {
		if suffix := name[n-4:]; suffix == ".exe" || suffix == ".EXE" {
			name = name[:n-4]
		}
	}
	return lower(name)
}

func lower(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'A' && c <= 'Z' {
			out[i] = c + ('a' - 'A')
		}
	}
	return string(out)
}

// LiveWindow is one top-level window as the platform currently reports it.
type LiveWindow struct {
	Handle     uintptr
	ProcessID  uint32
	Image      string // executable name, lowercased, no path or extension
	Title      string
	Bounds     Rect
	Visible    bool
	Minimized  bool
	Foreground bool
	OnScreen   bool // the bounds intersect some current monitor
}

// LookUpWindow answers what a handle currently is.
//
// ok is false when no window has that handle — the answer that was missing.
func LookUpWindow(hwnd uintptr) (LiveWindow, bool) {
	if hwnd == 0 || !IsWindow(hwnd) {
		return LiveWindow{}, false
	}
	pid := WindowProcessID(hwnd)
	minimized, _, visible := WindowStyleState(hwnd)
	bounds, _ := WindowBounds(hwnd)
	fg, _, _ := procGetForegroundWindow.Call()

	return LiveWindow{
		Handle: hwnd, ProcessID: pid, Image: ProcessImage(pid),
		Title: windowTitle(hwnd), Bounds: bounds,
		Visible: visible, Minimized: minimized,
		Foreground: fg == hwnd,
		OnScreen:   onScreen(bounds),
	}, true
}

// The window-enumeration callback, created ONCE — see the note on monitorsCallback.
//
// This one is what turned a slow leak into a crash: it was built per call, and each call
// then invoked onScreen per window, which built ANOTHER callback per window through
// Monitors. A single listing of 45 windows consumed 46 entries of a table Go never frees,
// and a session sampling every two seconds killed the service in under three minutes.
var (
	liveOnce   sync.Once
	liveCB     uintptr
	liveMu     sync.Mutex
	liveAccum  []LiveWindow
	liveScreen []Monitor
	liveFg     uintptr
)

func liveWindowsCallback() uintptr {
	liveOnce.Do(func() {
		liveCB = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
			if vis, _, _ := procIsWindowVisible.Call(hwnd); vis == 0 {
				return 1
			}
			bounds, ok := WindowBounds(hwnd)
			if !ok || bounds.W <= 0 || bounds.H <= 0 {
				return 1
			}
			pid := WindowProcessID(hwnd)
			minimized, _, visible := WindowStyleState(hwnd)
			liveAccum = append(liveAccum, LiveWindow{
				Handle: hwnd, ProcessID: pid, Image: ProcessImage(pid),
				Title: windowTitle(hwnd), Bounds: bounds,
				Visible: visible, Minimized: minimized,
				Foreground: liveFg == hwnd,
				// The monitors were read ONCE for this enumeration. Asking per window
				// is what exhausted the callback table.
				OnScreen: intersectsAny(bounds, liveScreen),
			})
			return 1
		})
	})
	return liveCB
}

// LiveWindows enumerates the current visible top-level windows.
//
// Enumerated fresh every call, deliberately. A cached list is precisely the thing that
// cannot be trusted here — but the display layout within ONE enumeration is read once,
// because it cannot change halfway through.
func LiveWindows() []LiveWindow {
	cb := liveWindowsCallback()
	monitors, _ := Monitors()
	fg, _, _ := procGetForegroundWindow.Call()

	liveMu.Lock()
	defer liveMu.Unlock()
	liveAccum, liveScreen, liveFg = nil, monitors, fg

	procEnumWindows.Call(cb, 0)

	out := make([]LiveWindow, len(liveAccum))
	copy(out, liveAccum)
	liveAccum, liveScreen = nil, nil
	return out
}

// intersectsAny reports whether a rectangle overlaps any of the given monitors.
func intersectsAny(r Rect, monitors []Monitor) bool {
	if r.W <= 0 || r.H <= 0 {
		return false
	}
	for _, m := range monitors {
		if intersects(r, m.Bounds) {
			return true
		}
	}
	return false
}

// windowTitle reads a window's caption.
func windowTitle(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLenW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}

// onScreen reports whether a rectangle intersects any current monitor.
//
// Windows parks minimized windows at (-32000,-32000), and a capture there returns whatever
// is nowhere. A negative origin on its own is perfectly ordinary — a monitor to the left of
// the primary has one — so this asks about intersection rather than sign.
func onScreen(r Rect) bool {
	if r.W <= 0 || r.H <= 0 {
		return false
	}
	mons, err := Monitors()
	if err != nil || len(mons) == 0 {
		return false
	}
	for _, m := range mons {
		if intersects(r, m.Bounds) {
			return true
		}
	}
	return false
}

func intersects(a, b Rect) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}
