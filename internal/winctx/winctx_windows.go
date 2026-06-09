//go:build windows

package winctx

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	psapi    = syscall.NewLazyDLL("psapi.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	procGetForegroundWindow  = user32.NewProc("GetForegroundWindow")
	procGetWindowRect        = user32.NewProc("GetWindowRect")
	procGetCursorPos         = user32.NewProc("GetCursorPos")
	procSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	procShowWindow           = user32.NewProc("ShowWindow")
	procIsWindowVisible      = user32.NewProc("IsWindowVisible")
	procEnumWindows          = user32.NewProc("EnumWindows")
	procGetWindowThreadPID   = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput    = user32.NewProc("AttachThreadInput")
	procBringWindowToTop     = user32.NewProc("BringWindowToTop")
	procSystemParametersInfo = user32.NewProc("SystemParametersInfoW")
	procGetCurrentThreadId   = kernel32.NewProc("GetCurrentThreadId")
	procOpenProcess          = kernel32.NewProc("OpenProcess")
	procCloseHandle          = kernel32.NewProc("CloseHandle")
	procGetModuleFileNameExW = psapi.NewProc("GetModuleFileNameExW")
	procShellExecuteW        = shell32.NewProc("ShellExecuteW")
)

const (
	swRestore                      = 9
	processQueryLimitedInformation = 0x1000

	spiSetForegroundLockTimeout = 0x2001
)

// rect mirrors Win32 RECT (left, top, right, bottom as LONGs).
type rect struct{ left, top, right, bottom int32 }

// point mirrors Win32 POINT.
type point struct{ x, y int32 }

// CursorPos returns the cursor's position in virtual-desktop pixels (the process
// is per-monitor DPI aware, so these are physical and match SetCursorPos). Used by
// voice-teach to record a click "where the cursor is" when you say "click this".
func CursorPos() (x, y int) {
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return int(p.x), int(p.y)
}

// ForegroundOrigin returns the top-left corner of the current foreground window in
// virtual-desktop pixels, and ok=false if there's no foreground window. The
// recorder uses it to store clicks relative to the active window, and the OS host
// to resolve them back — so a click lands at the same spot inside the window
// wherever Windows has placed it (any monitor, any position).
func ForegroundOrigin() (left, top int, ok bool) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return 0, 0, false
	}
	var r rect
	if ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ret == 0 {
		return 0, 0, false
	}
	return int(r.left), int(r.top), true
}

// Active returns the foreground app's short name (lowercase basename, no .exe),
// or "" if it can't be determined.
func Active() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}
	return appName(exeOf(hwnd))
}

// Activate brings a visible top-level window of the named app to the front. If
// no such window exists, it launches the app (via the shell) and waits briefly
// for its window to appear — so a context-aware route opens its app when it
// isn't already running.
func Activate(name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return fmt.Errorf("activate: empty app name")
	}
	found := findWindow(name)
	if found == 0 {
		// Not running — try to launch it, then wait for its window.
		if !launch(name) {
			return fmt.Errorf("activate: %q is not running and could not be launched", name)
		}
		for i := 0; i < 20 && found == 0; i++ {
			time.Sleep(150 * time.Millisecond)
			found = findWindow(name)
		}
		if found == 0 {
			return nil // launched, but no matching window yet — good enough
		}
	}
	forceForeground(found)
	// SetForegroundWindow is asynchronous: it returns before the window is actually
	// frontmost and ready for input, so a click/keystroke that fires immediately can
	// land on the previous window. Wait until our target really is the foreground
	// window (up to ~150ms), then settle briefly so it can paint and accept input.
	for i := 0; i < 15; i++ {
		if fg, _, _ := procGetForegroundWindow.Call(); fg == found {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	return nil
}

// forceForeground brings hwnd to the foreground reliably, even when this
// process didn't receive the last input event (e.g. marco was spawned hidden
// by a background overlay). A bare SetForegroundWindow is refused by Windows in
// that case, so we clear the foreground-lock timeout and attach our input queue
// to both the current foreground thread and the target thread for the duration
// of the call — the documented way around the foreground-activation lock.
func forceForeground(hwnd uintptr) {
	procShowWindow.Call(hwnd, swRestore)

	// Best-effort: remove the lock delay so SetForegroundWindow isn't deferred.
	procSystemParametersInfo.Call(spiSetForegroundLockTimeout, 0, 0, 0)

	fg, _, _ := procGetForegroundWindow.Call()
	if fg == hwnd {
		return
	}

	cur, _, _ := procGetCurrentThreadId.Call()
	fgThread, _, _ := procGetWindowThreadPID.Call(fg, 0)
	tgtThread, _, _ := procGetWindowThreadPID.Call(hwnd, 0)

	if fgThread != 0 && fgThread != cur {
		procAttachThreadInput.Call(cur, fgThread, 1)
		defer procAttachThreadInput.Call(cur, fgThread, 0)
	}
	if tgtThread != 0 && tgtThread != cur && tgtThread != fgThread {
		procAttachThreadInput.Call(cur, tgtThread, 1)
		defer procAttachThreadInput.Call(cur, tgtThread, 0)
	}

	procBringWindowToTop.Call(hwnd)
	procShowWindow.Call(hwnd, swRestore)
	procSetForegroundWindow.Call(hwnd)
}

// findWindow returns a visible top-level window whose owning app matches name,
// or 0.
func findWindow(name string) uintptr {
	var found uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if vis, _, _ := procIsWindowVisible.Call(hwnd); vis == 0 {
			return 1
		}
		if strings.Contains(appName(exeOf(hwnd)), name) {
			found = hwnd
			return 0
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return found
}

// Launch starts an app, file, or URL via the shell (ShellExecute) — no existing
// window required. Accepts an app name (chrome, notepad), an .exe path, or a URL
// like steam://rungameid/… .
func Launch(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("launch: empty target")
	}
	if shellOpen(target) {
		return nil
	}
	return fmt.Errorf("launch: could not start %q", target)
}

// launch is the Activate fallback: try the bare name, then name.exe.
func launch(name string) bool {
	return shellOpen(name) || shellOpen(name+".exe")
}

// shellOpen runs ShellExecuteW("open", target); a return value > 32 is success.
func shellOpen(target string) bool {
	verb, _ := syscall.UTF16PtrFromString("open")
	file, _ := syscall.UTF16PtrFromString(target)
	r, _, _ := procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)), 0, 0, swRestore)
	return r > 32
}

// exeOf returns the full executable path owning a window.
func exeOf(hwnd uintptr) string {
	var pid uint32
	procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}
	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)
	buf := make([]uint16, 260)
	procGetModuleFileNameExW.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), 260)
	return syscall.UTF16ToString(buf)
}
