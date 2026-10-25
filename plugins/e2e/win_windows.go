package main

// THE FEW THINGS THIS HARNESS ASKS WINDOWS DIRECTLY.
//
// All of them are READS or ARRANGEMENT, and the distinction is the whole of why this file is
// allowed to exist:
//
//	a read         — how big is the screen, is this a remote session, where is that window,
//	                 which window is in front, what is it called
//	arrangement    — put that window there, at that size
//	INPUT          — not here, and not anywhere in this harness. See boundary_test.go.
//
// The harness may put the Save button there; only Marco may press Save. Moving a window is
// putting the button somewhere, which is the thing a moved-window scenario is ABOUT — and it is
// not a press, a keystroke or a pointer movement. There is no SendInput, mouse_event, keybd_event
// or SetCursorPos in this module, and a test holds that, because a harness that could perform the
// action under test could make any scenario pass by doing the work itself and the failure would
// look exactly like a success.
//
// Windows is asked by hand rather than through `internal/winctx` for the three things that
// package does not expose — a system metric, a window belonging to a known process, and a window
// move. Everything winctx DOES answer is asked of winctx, because those answers are the ones
// Marco itself acts on and a harness with a second opinion about where a window is would be
// comparing its own arithmetic with Marco's.

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	procGetSystemMetrics       = user32.NewProc("GetSystemMetrics")
	procGetForegroundWindow    = user32.NewProc("GetForegroundWindow")
	procEnumWindows            = user32.NewProc("EnumWindows")
	procGetWindowThreadProcess = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible        = user32.NewProc("IsWindowVisible")
	procGetWindowTextW         = user32.NewProc("GetWindowTextW")
	procSetWindowPos           = user32.NewProc("SetWindowPos")
	procGetWindow              = user32.NewProc("GetWindow")
	procGetClientRect          = user32.NewProc("GetClientRect")
	procClientToScreen         = user32.NewProc("ClientToScreen")
	gdi32                      = syscall.NewLazyDLL("gdi32.dll")
	procGetDeviceCaps          = gdi32.NewProc("GetDeviceCaps")
	procGetDC                  = user32.NewProc("GetDC")
	procReleaseDC              = user32.NewProc("ReleaseDC")
	procPostMessageW           = user32.NewProc("PostMessageW")
	smRemoteSession            = 0x1000
	smCXScreen, smCYScreen     = 0, 1
	gwOwner                    = 4
	swpNoZOrder, swpNoActivate = uintptr(0x0004), uintptr(0x0010)
	logPixelsX                 = 88
)

func systemMetric(i int) int {
	v, _, _ := procGetSystemMetrics.Call(uintptr(i))
	return int(int32(v))
}

// screenDPI is the display's reported scale, in dots per inch at the desktop.
//
// Read because it decides whether a coordinate means the same thing to two processes. A harness
// that seeds a screen coordinate measured in one process and replays it from another is assuming
// both are talking about the same pixels, and on a scaled display with a process that has not
// said it understands scaling, they are not.
func screenDPI() int {
	dc, _, _ := procGetDC.Call(0)
	if dc == 0 {
		return 0
	}
	defer procReleaseDC.Call(0, dc)
	v, _, _ := procGetDeviceCaps.Call(dc, uintptr(logPixelsX))
	return int(int32(v))
}

func foregroundWindow() uintptr {
	h, _, _ := procGetForegroundWindow.Call()
	return h
}

// eachWindow walks every top-level window, visible or not.
func eachWindow(visit func(hwnd uintptr) bool) {
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if !visit(hwnd) {
			return 0
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
}

func pidOf(hwnd uintptr) uint32 {
	var pid uint32
	procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid
}

func visible(hwnd uintptr) bool {
	v, _, _ := procIsWindowVisible.Call(hwnd)
	return v != 0
}

func titleOf(hwnd uintptr) string {
	buf := make([]uint16, 512)
	n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// windowOf is the visible, unowned top-level window of one process.
//
// BY PROCESS AND NOT BY NAME, which is the point. This harness starts its own Studio and its own
// fixture, and the machine it runs on may well have the owner's Studio open beside them — so
// "the window called Marco Studio" is ambiguous in exactly the case that matters, and the window
// belonging to the process this harness spawned never is.
//
// Owned windows are skipped: a dialog or a tooltip belongs to the same process and is not the
// window somebody means.
func windowOf(pid uint32) (uintptr, bool) {
	var found uintptr
	eachWindow(func(hwnd uintptr) bool {
		if pidOf(hwnd) != pid || !visible(hwnd) {
			return true
		}
		if owner, _, _ := procGetWindow.Call(hwnd, uintptr(gwOwner)); owner != 0 {
			return true
		}
		if titleOf(hwnd) == "" {
			return true
		}
		found = hwnd
		return false
	})
	return found, found != 0
}

// strays is every visible window with this title that does NOT belong to one of these processes.
//
// For refusing rather than for tidying up. A fixture somebody left running from an earlier
// session writes to the same oracle names and answers to the same application name, and the
// harness does not know whose it is — ending somebody else's process to make room is not a thing
// a test should do.
func strays(title string, mine ...uint32) []string {
	var out []string
	eachWindow(func(hwnd uintptr) bool {
		if !visible(hwnd) || titleOf(hwnd) != title {
			return true
		}
		pid := pidOf(hwnd)
		for _, m := range mine {
			if pid == m {
				return true
			}
		}
		out = append(out, fmt.Sprintf("%s (pid %d)", title, pid))
		return true
	})
	return out
}

// clientOrigin is where a window's client area starts on screen, and how big it is.
func clientOrigin(hwnd uintptr) (x, y, w, h int, ok bool) {
	var r struct{ Left, Top, Right, Bottom int32 }
	if ret, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ret == 0 {
		return 0, 0, 0, 0, false
	}
	var p struct{ X, Y int32 }
	if ret, _, _ := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&p))); ret == 0 {
		return 0, 0, 0, 0, false
	}
	return int(p.X), int(p.Y), int(r.Right - r.Left), int(r.Bottom - r.Top), true
}

// place moves and resizes a window, without raising it or taking the foreground.
//
// ARRANGEMENT. A moved-window scenario has to move the window, and this is the only way to do
// that which is not a person dragging a title bar. It deliberately does not activate: a harness
// that took the foreground as a side effect of arranging the furniture would be changing the one
// condition Marco's own guard is about to check.
func place(hwnd uintptr, x, y, w, h int) error {
	ret, _, err := procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(w),
		uintptr(h), swpNoZOrder|swpNoActivate)
	if ret == 0 {
		return fmt.Errorf("moving the window to %d,%d %dx%d: %v", x, y, w, h, err)
	}
	return nil
}

// closeWindow asks a window to close, the way its own close button does.
//
// ASKS. `WM_CLOSE` runs the application's shutdown — which for Studio means stopping the Director
// it started and letting the trial journal have its last word — where killing the process does
// not. Every caller here has a bound on it and kills afterwards, because a harness that waited
// forever for a window to go would hang a run that had already produced its result.
func closeWindow(hwnd uintptr) {
	const wmClose = 0x0010
	procPostMessageW.Call(hwnd, wmClose, 0, 0)
}

// childOf is the process id of this process's child with the given image name.
//
// BY PARENT, which is the whole reason this exists. A scenario that killed `marco.exe` by name
// would end a run the OWNER had started if they happened to be performing something at that
// moment — and the harness would then be reporting about a process it did not own, having
// interfered with somebody's work to do it. The runner this run may kill is the one its own
// Studio spawned.
//
// Refuses an ambiguous answer rather than picking one: two children of one name means the
// question did not identify a process.
func childOf(parent uint32, image string) (uint32, bool) {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, false
	}
	defer syscall.CloseHandle(snap)
	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	found, n := uint32(0), 0
	for err = syscall.Process32First(snap, &e); err == nil; err = syscall.Process32Next(snap, &e) {
		if e.ParentProcessID != parent {
			continue
		}
		if !strings.EqualFold(syscall.UTF16ToString(e.ExeFile[:]), image) {
			continue
		}
		found, n = e.ProcessID, n+1
	}
	return found, n == 1
}

// killPid ends one process, by the id a caller has already established is theirs to end.
func killPid(pid uint32) error {
	p, err := os.FindProcess(int(pid))
	if err != nil {
		return fmt.Errorf("pid %d: %w", pid, err)
	}
	if err := p.Kill(); err != nil {
		return fmt.Errorf("killing pid %d: %w", pid, err)
	}
	return nil
}
