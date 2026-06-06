//go:build windows

package winctx

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	psapi    = syscall.NewLazyDLL("psapi.dll")

	procGetForegroundWindow  = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	procShowWindow           = user32.NewProc("ShowWindow")
	procIsWindowVisible      = user32.NewProc("IsWindowVisible")
	procEnumWindows          = user32.NewProc("EnumWindows")
	procGetWindowThreadPID   = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess          = kernel32.NewProc("OpenProcess")
	procCloseHandle          = kernel32.NewProc("CloseHandle")
	procGetModuleFileNameExW = psapi.NewProc("GetModuleFileNameExW")
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

// Activate brings a visible top-level window of the named app to the front.
func Activate(name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return fmt.Errorf("activate: empty app name")
	}
	var found uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if vis, _, _ := procIsWindowVisible.Call(hwnd); vis == 0 {
			return 1 // keep enumerating
		}
		if strings.Contains(appName(exeOf(hwnd)), name) {
			found = hwnd
			return 0 // stop
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	if found == 0 {
		return fmt.Errorf("activate: no window for %q", name)
	}
	procShowWindow.Call(found, swRestore)
	procSetForegroundWindow.Call(found)
	return nil
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
