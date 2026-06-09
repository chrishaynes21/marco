//go:build windows

// Windows backend: real keyboard/mouse/screen via Win32 SendInput & GDI.
// Ported from the original MacroMarco engine's platform/windows backend, trimmed
// to the primitives the OS host needs and made self-contained (no external
// platform/sentence deps).
package oshost

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func newBackend() backend { return &winBackend{} }

type winBackend struct{}

// typeCadence is the delay between characters when typing a string. Sending
// keystrokes back-to-back makes some apps/games drop characters; a small gap
// lets each WM_CHAR land. Override with $MARCO_TYPE_CADENCE_MS (0 = blast).
var typeCadence = typeCadenceFromEnv()

func typeCadenceFromEnv() time.Duration {
	if v := os.Getenv("MARCO_TYPE_CADENCE_MS"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return 12 * time.Millisecond
}

func (winBackend) key(ctx context.Context, name string) error {
	// A chord like "ctrl+c" or "ctrl+shift+esc": all parts but the last are
	// modifiers held down while the last key is tapped, then released in reverse.
	// A plain "c" is just a one-element chord.
	parts := splitChord(name)
	if len(parts) == 0 {
		return fmt.Errorf("empty key")
	}
	type resolved struct {
		vk, scan uint16
		ext      bool
	}
	keys := make([]resolved, len(parts))
	for i, p := range parts {
		vk, scan, ext := resolveKey(p)
		if vk == 0 && scan == 0 {
			return fmt.Errorf("unknown key: %q", p)
		}
		keys[i] = resolved{vk, scan, ext}
	}
	// Hold modifiers down.
	for i := 0; i < len(keys)-1; i++ {
		if !sendKeyDown(keys[i].vk, keys[i].scan, keys[i].ext) {
			return fmt.Errorf("SendInput rejected key %q", parts[i])
		}
	}
	last := keys[len(keys)-1]
	okDown := sendKeyDown(last.vk, last.scan, last.ext)
	okUp := sendKeyUp(last.vk, last.scan, last.ext)
	// Release modifiers in reverse order, even if the tap was rejected.
	for i := len(keys) - 2; i >= 0; i-- {
		sendKeyUp(keys[i].vk, keys[i].scan, keys[i].ext)
	}
	if !okDown || !okUp {
		return fmt.Errorf("SendInput rejected key %q", name)
	}
	return ctx.Err()
}

// splitChord splits a key spec on '+' into non-empty parts. A bare "+" (the plus
// key itself) falls back to the whole string so it isn't lost.
func splitChord(name string) []string {
	var out []string
	for _, p := range strings.Split(name, "+") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 && strings.TrimSpace(name) != "" {
		out = []string{strings.TrimSpace(name)}
	}
	return out
}

func (winBackend) typeText(ctx context.Context, text string) error {
	for i, ch := range text {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !sendUnicodeChar(ch) {
			return fmt.Errorf("SendInput rejected character %q", string(ch))
		}
		// Pace the keystrokes so fast apps/games don't drop characters.
		if typeCadence > 0 && i < len(text)-1 {
			select {
			case <-time.After(typeCadence):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}

func (winBackend) click(ctx context.Context, button string) error {
	down, up := mouseButtonFlags(button)
	if !sendMouseInput(0, 0, 0, down) || !sendMouseInput(0, 0, 0, up) {
		return fmt.Errorf("SendInput rejected %s click", button)
	}
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

// inputT mirrors the Win32 INPUT struct exactly. On x64 that is: a 4-byte type,
// 4 bytes of padding so the union is 8-byte aligned, then the union (large enough
// for MOUSEINPUT, the bigger arm, at 32 bytes). Total 40 bytes — SendInput's
// cbSize MUST equal this or the call fails and injects nothing.
type inputT struct {
	inputType uint32
	_         uint32
	union     [32]byte
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

// sendInput dispatches one INPUT and reports whether the OS accepted it (the
// return value is the number of events inserted; 0 means it was blocked or the
// struct size was wrong).
func sendInput(inp *inputT) bool {
	n, _, _ := procSendInput.Call(1, uintptr(unsafe.Pointer(inp)), unsafe.Sizeof(*inp))
	return n == 1
}

func sendKeyDown(vk, scan uint16, extended bool) bool {
	var inp inputT
	inp.inputType = _INPUT_KEYBOARD
	kb := (*keybdInput)(unsafe.Pointer(&inp.union[0]))
	kb.wVk = vk
	kb.wScan = scan
	if extended {
		kb.dwFlags = _KEYEVENTF_EXTENDEDKEY
	}
	return sendInput(&inp)
}

func sendKeyUp(vk, scan uint16, extended bool) bool {
	var inp inputT
	inp.inputType = _INPUT_KEYBOARD
	kb := (*keybdInput)(unsafe.Pointer(&inp.union[0]))
	kb.wVk = vk
	kb.wScan = scan
	kb.dwFlags = _KEYEVENTF_KEYUP
	if extended {
		kb.dwFlags |= _KEYEVENTF_EXTENDEDKEY
	}
	return sendInput(&inp)
}

func sendUnicodeChar(ch rune) bool {
	var inp inputT
	inp.inputType = _INPUT_KEYBOARD
	kb := (*keybdInput)(unsafe.Pointer(&inp.union[0]))
	kb.wScan = uint16(ch)
	kb.dwFlags = _KEYEVENTF_UNICODE
	okDown := sendInput(&inp)
	kb.dwFlags = _KEYEVENTF_UNICODE | _KEYEVENTF_KEYUP
	okUp := sendInput(&inp)
	return okDown && okUp
}

func sendMouseInput(dx, dy int32, mouseData uint32, flags uint32) bool {
	var inp inputT
	inp.inputType = _INPUT_MOUSE
	mi := (*mouseInput)(unsafe.Pointer(&inp.union[0]))
	mi.dx = dx
	mi.dy = dy
	mi.mouseData = mouseData
	mi.dwFlags = flags
	return sendInput(&inp)
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
