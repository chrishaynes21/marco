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
	procSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	procShowWindow           = user32.NewProc("ShowWindow")
	procIsWindowVisible      = user32.NewProc("IsWindowVisible")
	procEnumWindows          = user32.NewProc("EnumWindows")
	procGetWindowThreadPID   = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess          = kernel32.NewProc("OpenProcess")
	procCloseHandle          = kernel32.NewProc("CloseHandle")
	procGetModuleFileNameExW = psapi.NewProc("GetModuleFileNameExW")
	procShellExecuteW        = shell32.NewProc("ShellExecuteW")
)

const (
	swRestore                      = 9
	processQueryLimitedInformation = 0x1000
)

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
	procShowWindow.Call(found, swRestore)
	procSetForegroundWindow.Call(found)
	return nil
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
