//go:build !windows

package winctx

// Stubs keeping the window/monitor geometry surface compiling off Windows. Marco's
// cross-platform-by-construction rule: every OS capability has a real backend and a
// stub, and the stub side must keep building.

// Rect is a screen rectangle in virtual-desktop pixels.
type Rect struct{ X, Y, W, H int }

// Monitor is one physical display.
type Monitor struct {
	ID      string
	Bounds  Rect
	Work    Rect
	Primary bool
	Scale   float64
}

func Monitors() ([]Monitor, error) { return nil, ErrUnsupported }

func MonitorOf(hwnd uintptr) (Monitor, bool) { return Monitor{}, false }

func WindowBounds(hwnd uintptr) (Rect, bool) { return Rect{}, false }

func WindowStyleState(hwnd uintptr) (minimized, maximized, visible bool) { return false, false, false }

func MoveWindow(hwnd uintptr, x, y, w, h int) error { return ErrUnsupported }

func SetWindowState(hwnd uintptr, state string) error { return ErrUnsupported }
