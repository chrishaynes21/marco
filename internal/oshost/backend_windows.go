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
	return 5 * time.Millisecond
}

// chordHold is how long a key combo's modifiers are held around the key tap, so the
// target registers it (Alt+Tab, Ctrl+Shift+Esc, …). Override with $MARCO_CHORD_HOLD_MS.
var chordHold = envDurationMs("MARCO_CHORD_HOLD_MS", 40)

// tapHold is the small linger between a plain key's down and up, so a fast game/app
// registers the press instead of dropping an instant tap. Override $MARCO_KEY_HOLD_MS.
var tapHold = envDurationMs("MARCO_KEY_HOLD_MS", 25)

func envDurationMs(name string, def int) time.Duration {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return time.Duration(def) * time.Millisecond
}

// sleepCtx waits for d, returning early if ctx is canceled (Esc/panic-stop).
func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}

func (winBackend) key(ctx context.Context, name string, hold time.Duration) error {
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
	// A chord (held modifiers) gets a small hold so the app registers it — a combo
	// like Alt+Tab needs the modifier settled before the key and the combo held a
	// beat, or it's missed. A plain key still gets a SMALL linger (tapHold) so a fast
	// game/app registers the press — an instant down+up is what they drop.
	chord := len(keys) > 1
	last := keys[len(keys)-1]
	// How long the key/combo stays down before release. An explicit hold (from
	// `Key with Key …, Hold …`) overrides the default linger — this is what lets
	// Alt+Tab stay held long enough to commit the switch against a fullscreen app.
	holdDur := hold
	if holdDur <= 0 {
		if chord {
			holdDur = chordHold
		} else {
			holdDur = tapHold
		}
	}
	if chord {
		sleepCtx(ctx, chordHold) // let the modifier(s) settle
	}
	okDown := sendKeyDown(last.vk, last.scan, last.ext)
	sleepCtx(ctx, holdDur) // hold the key/combo before releasing
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

// keyDown presses and HOLDS a single key (no release) — the start of a hold. keyUp
// releases it. Single key only (a held combo isn't a recorded pattern).
func (winBackend) keyDown(ctx context.Context, name string) error {
	vk, scan, ext := resolveKey(name)
	if vk == 0 && scan == 0 {
		return fmt.Errorf("unknown key: %q", name)
	}
	if !sendKeyDown(vk, scan, ext) {
		return fmt.Errorf("SendInput rejected key %q", name)
	}
	return ctx.Err()
}

func (winBackend) keyUp(ctx context.Context, name string) error {
	vk, scan, ext := resolveKey(name)
	if vk == 0 && scan == 0 {
		return fmt.Errorf("unknown key: %q", name)
	}
	if !sendKeyUp(vk, scan, ext) {
		return fmt.Errorf("SendInput rejected key %q", name)
	}
	return ctx.Err()
}

// splitChord splits a key spec on '+' into non-empty parts. A bare "+" (the plus
// key itself) falls back to the whole string so it isn't lost.
func splitChord(name string) []string {
	var out []string
	for p := range strings.SplitSeq(name, "+") {
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

// clickAt positions the cursor and clicks in ONE atomic SendInput batch (an absolute
// move, then button down, then up — every event carrying the absolute target). Going
// through one SendInput call means no other input interleaves and the down/up specify
// their own position, so the click can't land at a stale spot the way a separate
// SetCursorPos-then-click can race. Coordinates are normalised over the VIRTUAL desktop,
// so it's correct on any monitor (including one with a negative origin).
func (winBackend) clickAt(ctx context.Context, button string, x, y int) error {
	down, up := mouseButtonFlags(button)
	vx, vy, vw, vh := virtualDesk()
	nx, ny := normAbsolute(x, y, vx, vy, vw, vh)
	const absDesk = _MOUSEEVENTF_ABSOLUTE | _MOUSEEVENTF_VIRTUALDESK
	inps := []inputT{
		mouseInputT(nx, ny, 0, _MOUSEEVENTF_MOVE|absDesk),
		mouseInputT(nx, ny, 0, down|absDesk),
		mouseInputT(nx, ny, 0, up|absDesk),
	}
	if !sendInputs(inps) {
		return fmt.Errorf("SendInput rejected %s click at (%d,%d)", button, x, y)
	}
	return ctx.Err()
}

// drag performs a REAL button-held drag: move to the start, press the button, GLIDE to the end
// as a trail of injected MOVE events (so an app/game registers the drag motion, not a teleport),
// then release. Absolute coordinates over the virtual desktop, like clickAt.
func (winBackend) drag(ctx context.Context, button string, x1, y1, x2, y2 int) error {
	down, up := mouseButtonFlags(button)
	vx, vy, vw, vh := virtualDesk()
	const absDesk = _MOUSEEVENTF_ABSOLUTE | _MOUSEEVENTF_VIRTUALDESK
	send := func(x, y int, flags uint32) {
		nx, ny := normAbsolute(x, y, vx, vy, vw, vh)
		sendInputs([]inputT{mouseInputT(nx, ny, 0, flags)})
	}
	send(x1, y1, _MOUSEEVENTF_MOVE|absDesk) // to start
	send(x1, y1, down|absDesk)              // press
	const steps = 24
	for i := 1; i < steps; i++ {
		if ctx.Err() != nil {
			break
		}
		t := float64(i) / float64(steps)
		send(x1+int(float64(x2-x1)*t), y1+int(float64(y2-y1)*t), _MOUSEEVENTF_MOVE|absDesk)
		time.Sleep(8 * time.Millisecond) // let the drag motion register between steps
	}
	send(x2, y2, _MOUSEEVENTF_MOVE|absDesk) // land exactly on the end
	send(x2, y2, up|absDesk)                // release
	return ctx.Err()
}

// move sends a real injected mouse-MOVE event (SendInput), not a bare SetCursorPos.
// SetCursorPos only repositions the pointer; many apps — and ESPECIALLY games reading raw
// input — only fire their hover/highlight on an actual movement event, so a hover-settle
// or wiggle done with SetCursorPos never lights the button up. An absolute MOUSEEVENTF_MOVE
// over the virtual desktop is the same gesture clickAt uses, so it's correct on any
// monitor; it falls back to SetCursorPos if SendInput is rejected.
func (winBackend) move(ctx context.Context, x, y int) error {
	vx, vy, vw, vh := virtualDesk()
	nx, ny := normAbsolute(x, y, vx, vy, vw, vh)
	const absDesk = _MOUSEEVENTF_ABSOLUTE | _MOUSEEVENTF_VIRTUALDESK
	if !sendInputs([]inputT{mouseInputT(nx, ny, 0, _MOUSEEVENTF_MOVE|absDesk)}) {
		procSetCursorPos.Call(uintptr(x), uintptr(y))
	}
	return ctx.Err()
}

// virtualDesk returns the virtual screen's origin and size (all monitors). The origin
// is negative when a monitor sits left of / above the primary; int32 first so a negative
// metric isn't read as a huge uintptr.
func virtualDesk() (x, y, w, h int) {
	vx, _, _ := procGetSystemMetrics.Call(_SM_XVIRTUALSCREEN)
	vy, _, _ := procGetSystemMetrics.Call(_SM_YVIRTUALSCREEN)
	vw, _, _ := procGetSystemMetrics.Call(_SM_CXVIRTUALSCREEN)
	vh, _, _ := procGetSystemMetrics.Call(_SM_CYVIRTUALSCREEN)
	return int(int32(vx)), int(int32(vy)), int(int32(vw)), int(int32(vh))
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
	procGetSystemMetrics     = user32.NewProc("GetSystemMetrics")
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

	_MOUSEEVENTF_MOVE        = 0x0001
	_MOUSEEVENTF_LEFTDOWN    = 0x0002
	_MOUSEEVENTF_LEFTUP      = 0x0004
	_MOUSEEVENTF_RIGHTDOWN   = 0x0008
	_MOUSEEVENTF_RIGHTUP     = 0x0010
	_MOUSEEVENTF_MIDDLEDOWN  = 0x0020
	_MOUSEEVENTF_MIDDLEUP    = 0x0040
	_MOUSEEVENTF_VIRTUALDESK = 0x4000
	_MOUSEEVENTF_ABSOLUTE    = 0x8000

	// Virtual-screen metrics (GetSystemMetrics) — span ALL monitors; the origin is
	// negative when a monitor sits left of / above the primary.
	_SM_XVIRTUALSCREEN  = 76
	_SM_YVIRTUALSCREEN  = 77
	_SM_CXVIRTUALSCREEN = 78
	_SM_CYVIRTUALSCREEN = 79
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
	inp := mouseInputT(dx, dy, mouseData, flags)
	return sendInput(&inp)
}

// mouseInputT builds an INPUT for a single mouse event (dx,dy are absolute 0..65535
// when flags carry MOUSEEVENTF_ABSOLUTE, else a relative delta).
func mouseInputT(dx, dy int32, mouseData uint32, flags uint32) inputT {
	var inp inputT
	inp.inputType = _INPUT_MOUSE
	mi := (*mouseInput)(unsafe.Pointer(&inp.union[0]))
	mi.dx = dx
	mi.dy = dy
	mi.mouseData = mouseData
	mi.dwFlags = flags
	return inp
}

// sendInputs dispatches a batch of INPUTs in a SINGLE SendInput call, so no other
// (user or injected) input interleaves between them — the move+down+up of a click stay
// atomic and ordered. Reports whether every event was accepted.
func sendInputs(inps []inputT) bool {
	if len(inps) == 0 {
		return true
	}
	n, _, _ := procSendInput.Call(uintptr(len(inps)), uintptr(unsafe.Pointer(&inps[0])), unsafe.Sizeof(inps[0]))
	return int(n) == len(inps)
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
