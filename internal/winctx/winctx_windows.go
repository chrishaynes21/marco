//go:build windows

package winctx

import (
	"fmt"
	"os"
	"strconv"
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
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetWindowRect        = user32.NewProc("GetWindowRect")
	procGetCursorPos         = user32.NewProc("GetCursorPos")
	procSetCursorPos         = user32.NewProc("SetCursorPos")
	procSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	procShowWindow           = user32.NewProc("ShowWindow")
	procIsIconic             = user32.NewProc("IsIconic")
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
	swShow                         = 5 // activate + show in current size/position (preserves maximized)
	swRestore                      = 9 // activate + restore (un-minimizes AND un-maximizes)
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

// SetCursorPos moves the cursor to virtual-desktop (x, y). Used to put the pointer
// back where the user left it after a route runs, so a menu macro doesn't leave it
// parked on the last click.
func SetCursorPos(x, y int) {
	procSetCursorPos.Call(uintptr(x), uintptr(y))
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

// ForegroundInfo is ForegroundOrigin plus the foreground window's app name, read from
// the SAME window handle — so a diagnostic can report WHICH window a window-relative
// click resolved its origin against (the "why it clicked there" when it's wrong).
func ForegroundInfo() (left, top int, name string, ok bool) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return 0, 0, "", false
	}
	var r rect
	if ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ret == 0 {
		return 0, 0, "", false
	}
	return int(r.left), int(r.top), appName(exeOf(hwnd)), true
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

// ForegroundTitle returns the title-bar text of the current foreground window — the human
// WINDOW name (e.g. "Friends List" vs "Steam"), used to confirm a route is acting on the
// right window of an app that can open several. "" if it can't be read.
func ForegroundTitle() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}
	buf := make([]uint16, 512)
	n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:n])
}

// Activate brings a visible top-level window of the named app to the front. If no
// such window exists it launches the app and WAITS for its window to actually open
// (a launcher or game can be slow to cold-start) — failing if it never does, so the
// route stops instead of clicking into the wrong window. Once frontmost it waits for
// the window to stop resizing, so a fullscreen/maximize transition finishes before
// the route's clicks land (otherwise they hit the pre-fullscreen layout).
func Activate(name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return fmt.Errorf("activate: empty app name")
	}
	found := findWindow(name)
	if found == 0 {
		// Not running — launch it, then wait (generously) for its window to open.
		if !launch(name) {
			return fmt.Errorf("activate: %q is not running and could not be launched", name)
		}
		deadline := time.Now().Add(launchTimeout())
		for found == 0 && time.Now().Before(deadline) {
			time.Sleep(250 * time.Millisecond)
			found = findWindow(name)
		}
		if found == 0 {
			return fmt.Errorf("activate: %q did not open within the launch timeout", name)
		}
	}
	// Remember what we're switching away from, so Restore can return there (e.g. go
	// back to your game after muting Discord). Only when we're actually changing
	// windows, so Restore never points at the target itself.
	if cur, _, _ := procGetForegroundWindow.Call(); cur != 0 && cur != found {
		prevForeground = cur
	}
	forceForeground(found)
	// SetForegroundWindow is asynchronous: it returns before the window is actually
	// frontmost. Wait until our target really is the foreground window (up to ~150ms).
	for range 15 {
		if fg, _, _ := procGetForegroundWindow.Call(); fg == found {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Then wait for it to settle — poll the window rect until it stops changing, so a
	// fullscreen/maximize transition (or a still-laying-out launcher) is done before
	// the route clicks. Returns quickly for an already-stable window.
	waitWindowStable(found)
	return nil
}

// prevForeground is the window that was in front before the most recent Activate
// switch — Restore returns to it. Per-process (a route runs in its own `marco do`).
var prevForeground uintptr

// RestorePrevious re-focuses the window that was in front before the last Activate,
// so a route can act in another app and then return you to where you were (mute
// Discord from a game, then go back). No-op if nothing was switched away from.
func RestorePrevious() error {
	if prevForeground == 0 {
		return nil
	}
	forceForeground(prevForeground)
	return nil
}

// launchTimeout is how long Activate waits for a freshly launched app's window to
// open. Generous by default — cold-starting Epic/Steam/a game can take many seconds.
// Override with $MARCO_LAUNCH_TIMEOUT_MS.
func launchTimeout() time.Duration {
	if v := os.Getenv("MARCO_LAUNCH_TIMEOUT_MS"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return 20 * time.Second
}

// waitWindowStable blocks until hwnd's rectangle is unchanged across a few polls (it
// finished a fullscreen/maximize/restore transition), or a short cap elapses. This
// is what stops a route from clicking before a window has finished going fullscreen.
func waitWindowStable(hwnd uintptr) {
	var last rect
	stable := 0
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var r rect
		if ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ret == 0 {
			return
		}
		if r == last {
			if stable++; stable >= 3 { // ~120ms unchanged → settled
				return
			}
		} else {
			stable, last = 0, r
		}
		time.Sleep(40 * time.Millisecond)
	}
}

// forceForeground brings hwnd to the foreground reliably, even when this
// process didn't receive the last input event (e.g. marco was spawned hidden
// by a background overlay). A bare SetForegroundWindow is refused by Windows in
// that case, so we clear the foreground-lock timeout and attach our input queue
// to both the current foreground thread and the target thread for the duration
// of the call — the documented way around the foreground-activation lock.
func forceForeground(hwnd uintptr) {
	restoreIfMinimized(hwnd)

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
	restoreIfMinimized(hwnd)
	procSetForegroundWindow.Call(hwnd)
}

// restoreIfMinimized un-minimizes a minimized window so it can come forward, but
// leaves a maximized/fullscreen window untouched. SW_RESTORE un-maximizes a
// maximized window too, which pulls fullscreen apps (games, the Xbox app) out of
// fullscreen — IsIconic is true only when the window is actually minimized, so this
// restores only that case.
func restoreIfMinimized(hwnd uintptr) {
	if iconic, _, _ := procIsIconic.Call(hwnd); iconic != 0 {
		procShowWindow.Call(hwnd, swRestore)
	}
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
// nCmdShow is SW_SHOW (not SW_RESTORE): when the target is already running, the
// shell re-activates the existing window, and SW_SHOW leaves its size alone —
// SW_RESTORE would un-maximize a maximized app (a game, the Xbox app). A freshly
// launched app is still shown and activated.
func shellOpen(target string) bool {
	verb, _ := syscall.UTF16PtrFromString("open")
	file, _ := syscall.UTF16PtrFromString(target)
	r, _, _ := procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)), 0, 0, swShow)
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
