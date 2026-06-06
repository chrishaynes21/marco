//go:build windows

// Windows backend: real keyboard/mouse/screen via Win32 SendInput & GDI.
// Ported from the original MacroMarco engine's platform/windows backend, trimmed
// to the primitives the OS host needs and made self-contained (no external
// platform/sentence deps).
package oshost

import (
	"context"
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

func newBackend() backend { return &winBackend{} }

type winBackend struct{}

func (winBackend) key(ctx context.Context, name string) error {
	vk, scan, extended := resolveKey(name)
	if vk == 0 && scan == 0 {
		return fmt.Errorf("unknown key: %q", name)
	}
	sendKeyDown(vk, scan, extended)
	sendKeyUp(vk, scan, extended)
	return ctx.Err()
}

func (winBackend) typeText(ctx context.Context, text string) error {
	for _, ch := range text {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		sendUnicodeChar(ch)
	}
	return nil
}

func (winBackend) click(ctx context.Context, button string) error {
	down, up := mouseButtonFlags(button)
	sendMouseInput(0, 0, 0, down)
	sendMouseInput(0, 0, 0, up)
	return ctx.Err()
}

func (winBackend) move(ctx context.Context, x, y int) error {
	procSetCursorPos.Call(uintptr(x), uintptr(y))
	return ctx.Err()
}

func (winBackend) color(ctx context.Context, x, y int) (uint32, error) {
	hdc, _, _ := procGetDC.Call(0)
	col, _, _ := procGetPixel.Call(hdc, uintptr(x), uintptr(y))
	procReleaseDC.Call(0, hdc)
	b := (col >> 16) & 0xFF
	g := (col >> 8) & 0xFF
	r := col & 0xFF
	return uint32((r << 16) | (g << 8) | b), ctx.Err()
}

func (winBackend) activeExe(ctx context.Context) (string, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	var pid uint32
	procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return "", ctx.Err()
	}
	const queryLimited = 0x1000
	hProc, _, _ := procOpenProcess.Call(queryLimited, 0, uintptr(pid))
	if hProc == 0 {
		return "", ctx.Err()
	}
	defer procCloseHandle.Call(hProc)
	buf := make([]uint16, 260)
	procGetModuleFileNameExW.Call(hProc, 0, uintptr(unsafe.Pointer(&buf[0])), 260)
	return syscall.UTF16ToString(buf), ctx.Err()
}

// ── Win32 bindings ──

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	psapi    = syscall.NewLazyDLL("psapi.dll")

	procSendInput            = user32.NewProc("SendInput")
	procSetCursorPos         = user32.NewProc("SetCursorPos")
	procGetForegroundWindow  = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadPID   = user32.NewProc("GetWindowThreadProcessId")
	procGetDC                = user32.NewProc("GetDC")
	procReleaseDC            = user32.NewProc("ReleaseDC")
	procMapVirtualKeyW       = user32.NewProc("MapVirtualKeyW")
	procGetPixel             = gdi32.NewProc("GetPixel")
	procOpenProcess          = kernel32.NewProc("OpenProcess")
	procCloseHandle          = kernel32.NewProc("CloseHandle")
	procGetModuleFileNameExW = psapi.NewProc("GetModuleFileNameExW")
)

const (
	_INPUT_KEYBOARD = 1
	_INPUT_MOUSE    = 0

	_KEYEVENTF_EXTENDEDKEY = 0x0001
	_KEYEVENTF_KEYUP       = 0x0002
	_KEYEVENTF_UNICODE     = 0x0004

	_MOUSEEVENTF_LEFTDOWN   = 0x0002
	_MOUSEEVENTF_LEFTUP     = 0x0004
	_MOUSEEVENTF_RIGHTDOWN  = 0x0008
	_MOUSEEVENTF_RIGHTUP    = 0x0010
	_MOUSEEVENTF_MIDDLEDOWN = 0x0020
	_MOUSEEVENTF_MIDDLEUP   = 0x0040
)

type inputT struct {
	inputType uint32
	padding   [40]byte // union of KEYBDINPUT / MOUSEINPUT — 40 bytes covers both on x64
}

type keybdInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

type mouseInput struct {
	dx          int32
	dy          int32
	mouseData   uint32
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

func sendKeyDown(vk, scan uint16, extended bool) {
	var inp inputT
	inp.inputType = _INPUT_KEYBOARD
	kb := (*keybdInput)(unsafe.Pointer(&inp.padding[0]))
	kb.wVk = vk
	kb.wScan = scan
	if extended {
		kb.dwFlags = _KEYEVENTF_EXTENDEDKEY
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

func sendKeyUp(vk, scan uint16, extended bool) {
	var inp inputT
	inp.inputType = _INPUT_KEYBOARD
	kb := (*keybdInput)(unsafe.Pointer(&inp.padding[0]))
	kb.wVk = vk
	kb.wScan = scan
	kb.dwFlags = _KEYEVENTF_KEYUP
	if extended {
		kb.dwFlags |= _KEYEVENTF_EXTENDEDKEY
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

func sendUnicodeChar(ch rune) {
	var inp inputT
	inp.inputType = _INPUT_KEYBOARD
	kb := (*keybdInput)(unsafe.Pointer(&inp.padding[0]))
	kb.wScan = uint16(ch)
	kb.dwFlags = _KEYEVENTF_UNICODE
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
	kb.dwFlags = _KEYEVENTF_UNICODE | _KEYEVENTF_KEYUP
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

func sendMouseInput(dx, dy int32, mouseData uint32, flags uint32) {
	var inp inputT
	inp.inputType = _INPUT_MOUSE
	mi := (*mouseInput)(unsafe.Pointer(&inp.padding[0]))
	mi.dx = dx
	mi.dy = dy
	mi.mouseData = mouseData
	mi.dwFlags = flags
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

func mouseButtonFlags(button string) (down, up uint32) {
	switch strings.ToLower(button) {
	case "right":
		return _MOUSEEVENTF_RIGHTDOWN, _MOUSEEVENTF_RIGHTUP
	case "middle":
		return _MOUSEEVENTF_MIDDLEDOWN, _MOUSEEVENTF_MIDDLEUP
	default:
		return _MOUSEEVENTF_LEFTDOWN, _MOUSEEVENTF_LEFTUP
	}
}

func mapVirtualKey(vk uint16) uint16 {
	const mapvkVKToVSC = 0
	ret, _, _ := procMapVirtualKeyW.Call(uintptr(vk), mapvkVKToVSC)
	return uint16(ret)
}
