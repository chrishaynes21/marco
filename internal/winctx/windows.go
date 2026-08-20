//go:build windows

package winctx

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Window and monitor geometry: the capabilities a Director needs to answer "which
// windows exist, where are they, and which screen is each one on", and to move one.
//
// Marco's macro engine never needed these — a recorded macro replays clicks and
// never reasons about layout — so they are additive here rather than a change to
// anything existing. Windows-only, with stubs elsewhere, like the rest of winctx.

var (
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procMonitorFromWindow   = user32.NewProc("MonitorFromWindow")
	procGetDpiForMonitor    = lazyShcore("GetDpiForMonitor")
	procGetWindowLongPtrW   = user32.NewProc("GetWindowLongPtrW")
	procGetPropW            = user32.NewProc("GetPropW")
)

func lazyShcore(name string) *syscall.LazyProc {
	return syscall.NewLazyDLL("shcore.dll").NewProc(name)
}

const (
	swpNoZOrder     = 0x0004
	swpNoActivate   = 0x0010
	swpFrameChanged = 0x0020

	monitorDefaultToNearest = 0x00000002

	gwlStyle   = ^uintptr(0) - 15 // GWL_STYLE (-16) as an unsigned index
	wsMaximize = 0x01000000
	wsMinimize = 0x20000000
	wsVisible  = 0x10000000

	mdtEffectiveDPI = 0
)

// Rect is a screen rectangle in virtual-desktop pixels. Position may be negative:
// a monitor placed left of or above the primary has a negative origin, and clamping
// that away would move every window on it to the wrong screen.
type Rect struct{ X, Y, W, H int }

// Monitor is one physical display.
type Monitor struct {
	ID      string
	Bounds  Rect
	Work    Rect // excludes the taskbar and other reserved edges
	Primary bool
	Scale   float64
}

// The monitor-enumeration callback, created ONCE.
//
// syscall.NewCallback allocates from a fixed process-wide table that Go never frees, so a
// callback built per call is a permanent leak. This one was leaking quietly for a long time
// because monitors are rarely enumerated — and then a passive observation session, which
// checks whether each window is on screen for every window of every sample, exhausted the
// table and killed the service outright:
//
//	fatal error: too many callback functions
//
// One callback, created on first use, with the accumulator reached through package state
// under a mutex. Not elegant — a callback cannot close over per-call state if it is to be
// created once — but the alternative is a process that dies after a few thousand calls.
var (
	monitorsOnce  sync.Once
	monitorsCB    uintptr
	monitorsMu    sync.Mutex
	monitorsAccum []Monitor
)

func monitorsCallback() uintptr {
	monitorsOnce.Do(func() {
		monitorsCB = syscall.NewCallback(func(hMonitor, hdc, lprc, data uintptr) uintptr {
			var mi struct {
				cbSize    uint32
				rcMonitor rect
				rcWork    rect
				dwFlags   uint32
			}
			mi.cbSize = uint32(unsafe.Sizeof(mi))
			if ret, _, _ := procGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&mi))); ret == 0 {
				return 1 // keep enumerating; one unreadable monitor is not fatal
			}
			monitorsAccum = append(monitorsAccum, Monitor{
				ID:      fmt.Sprintf("monitor:%d", hMonitor),
				Bounds:  fromRect(mi.rcMonitor),
				Work:    fromRect(mi.rcWork),
				Primary: mi.dwFlags&1 != 0, // MONITORINFOF_PRIMARY
				Scale:   monitorScale(hMonitor),
			})
			return 1
		})
	})
	return monitorsCB
}

// Monitors returns the current display layout.
func Monitors() ([]Monitor, error) {
	cb := monitorsCallback()

	monitorsMu.Lock()
	defer monitorsMu.Unlock()
	monitorsAccum = nil

	if ret, _, err := procEnumDisplayMonitors.Call(0, 0, cb, 0); ret == 0 {
		return nil, fmt.Errorf("winctx: enumerating monitors: %w", err)
	}
	out := make([]Monitor, len(monitorsAccum))
	copy(out, monitorsAccum)
	monitorsAccum = nil
	return out, nil
}

// monitorScale reads a monitor's DPI scale factor, defaulting to 1.0 where the API
// is unavailable (pre-8.1) or fails.
func monitorScale(hMonitor uintptr) float64 {
	var dpiX, dpiY uint32
	ret, _, _ := procGetDpiForMonitor.Call(hMonitor, mdtEffectiveDPI,
		uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY)))
	if ret != 0 || dpiX == 0 {
		return 1.0
	}
	return float64(dpiX) / 96.0
}

// MonitorOf returns the monitor a window is mostly on.
func MonitorOf(hwnd uintptr) (Monitor, bool) {
	h, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if h == 0 {
		return Monitor{}, false
	}
	id := fmt.Sprintf("monitor:%d", h)
	mons, err := Monitors()
	if err != nil {
		return Monitor{}, false
	}
	for _, m := range mons {
		if m.ID == id {
			return m, true
		}
	}
	return Monitor{}, false
}

// WindowBounds returns a window's rectangle.
func WindowBounds(hwnd uintptr) (Rect, bool) {
	var r rect
	if ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ret == 0 {
		return Rect{}, false
	}
	return fromRect(r), true
}

// WindowStyleState reports whether a window is currently minimized, maximized and
// visible — read from its style bits rather than inferred from its rectangle, since
// a minimized window still reports a rectangle (an off-screen one).
func WindowStyleState(hwnd uintptr) (minimized, maximized, visible bool) {
	style, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlStyle)
	return style&wsMinimize != 0, style&wsMaximize != 0, style&wsVisible != 0
}

// MoveWindow repositions and resizes a window.
//
// It deliberately does NOT activate the window (SWP_NOACTIVATE). Moving something
// is not the same as switching to it, and stealing focus as a side effect would
// change what every subsequent observation is about — the Director would then be
// looking at a different window than the one the user was talking about.
//
// A maximized window is restored first: SetWindowPos on a maximized window is
// silently ignored by the window manager, which would look to the Director like a
// move that reported success and did nothing.
func MoveWindow(hwnd uintptr, x, y, w, h int) error {
	if hwnd == 0 {
		return fmt.Errorf("winctx: no window")
	}
	if _, maximized, _ := WindowStyleState(hwnd); maximized {
		procShowWindow.Call(hwnd, swRestore)
	}
	ret, _, err := procSetWindowPos.Call(hwnd, 0,
		uintptr(int32(x)), uintptr(int32(y)), uintptr(int32(w)), uintptr(int32(h)),
		swpNoZOrder|swpNoActivate|swpFrameChanged)
	if ret == 0 {
		return fmt.Errorf("winctx: moving window: %w", err)
	}
	return nil
}

// SetWindowState applies a symbolic window state.
func SetWindowState(hwnd uintptr, state string) error {
	if hwnd == 0 {
		return fmt.Errorf("winctx: no window")
	}
	var cmd uintptr
	switch state {
	case "maximized":
		cmd = 3 // SW_MAXIMIZE
	case "minimized":
		cmd = 6 // SW_MINIMIZE
	case "normal":
		cmd = swRestore
	default:
		return fmt.Errorf("winctx: unknown window state %q", state)
	}
	procShowWindow.Call(hwnd, cmd)
	return nil
}

func fromRect(r rect) Rect {
	return Rect{X: int(r.left), Y: int(r.top),
		W: int(r.right - r.left), H: int(r.bottom - r.top)}
}

// IsOwnedSurface reports whether a window is one of Marco's own presentation surfaces.
//
// Read from a window PROPERTY the surface set on itself — see
// [directorapi.OwnedSurfaceProperty] for why ownership is marked this way rather than by title or
// process name. A window that does not carry it is somebody else's, which is the safe direction:
// the cost of failing to exclude one of ours is contamination, and the cost of wrongly excluding
// somebody else's is a window the user cannot target.
func IsOwnedSurface(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	name, err := syscall.UTF16PtrFromString(directorapi.OwnedSurfaceProperty)
	if err != nil {
		return false
	}
	h, _, _ := procGetPropW.Call(hwnd, uintptr(unsafe.Pointer(name)))
	return h != 0
}
