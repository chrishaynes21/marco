//go:build windows

package screen

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

func New() Screen { return winScreen{} }

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBmp = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject        = gdi32.NewProc("SelectObject")
	procBitBlt              = gdi32.NewProc("BitBlt")
	procGetDIBits           = gdi32.NewProc("GetDIBits")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
	procDeleteDC            = gdi32.NewProc("DeleteDC")
	procGetPixel            = gdi32.NewProc("GetPixel")

	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
)

// dpiPerMonitorAwareV2 is DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 ((HANDLE)-4).
const dpiPerMonitorAwareV2 = ^uintptr(3)

// init makes the process per-monitor DPI aware so screen capture and cursor
// coordinates are real (physical) pixels and stay consistent across monitors
// with different scaling. Without it Windows hands back DPI-virtualized
// coordinates and downscaled captures, which breaks clicks and image matching on
// scaled or secondary (e.g. left) monitors. oshost imports this package, so every
// macro process (do/teach/marco-macros) gets it. Must run before any GetDC; an
// init does (before main). Re-teach image templates after this change if older
// ones were captured while DPI-unaware.
func init() {
	if r, _, _ := procSetProcessDpiAwarenessContext.Call(dpiPerMonitorAwareV2); r == 0 {
		procSetProcessDPIAware.Call() // Windows 8.1 / older fallback (system-DPI aware)
	}
}

const (
	// Virtual-screen metrics span ALL monitors (the origin can be negative when a
	// monitor sits left of / above the primary), so capture and Find work across
	// the whole desktop regardless of layout.
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCxVirtualScreen = 78
	smCyVirtualScreen = 79
	smCxScreen        = 0 // primary monitor width
	smCyScreen        = 1 // primary monitor height
	srcCopy           = 0x00CC0020
	biRGB             = 0
	dibRGB            = 0
)

// PrimarySize returns the primary monitor's pixel dimensions (physical pixels —
// the process is per-monitor DPI aware). Returns 0,0 if unavailable. Used to size
// the recorder's click templates relative to the screen.
func PrimarySize() (w, h int) {
	cx, _, _ := procGetSystemMetrics.Call(smCxScreen)
	cy, _, _ := procGetSystemMetrics.Call(smCyScreen)
	return int(int32(cx)), int(int32(cy))
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type winScreen struct{}

func (winScreen) Pixel(x, y int) (uint32, error) {
	hdc, _, _ := procGetDC.Call(0)
	if hdc == 0 {
		return 0, fmt.Errorf("screen: GetDC failed")
	}
	defer procReleaseDC.Call(0, hdc)
	c, _, _ := procGetPixel.Call(hdc, uintptr(x), uintptr(y))
	r := c & 0xFF
	g := (c >> 8) & 0xFF
	b := (c >> 16) & 0xFF
	return uint32(r<<16 | g<<8 | b), nil
}

// regionRect resolves a Region to an absolute (x,y,w,h) capture rectangle — the whole
// virtual desktop (origin may be negative) when the Region is empty.
func regionRect(region Region) (x, y, w, h int, err error) {
	x, y, w, h = region.X1, region.Y1, region.X2-region.X1, region.Y2-region.Y1
	if region.Empty() {
		// int32 first so a negative metric isn't read as a huge uintptr.
		vx, _, _ := procGetSystemMetrics.Call(smXVirtualScreen)
		vy, _, _ := procGetSystemMetrics.Call(smYVirtualScreen)
		vw, _, _ := procGetSystemMetrics.Call(smCxVirtualScreen)
		vh, _, _ := procGetSystemMetrics.Call(smCyVirtualScreen)
		x, y, w, h = int(int32(vx)), int(int32(vy)), int(int32(vw)), int(int32(vh))
	}
	if w <= 0 || h <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("screen: empty search region")
	}
	return x, y, w, h, nil
}

func (winScreen) Find(templatePath string, region Region, tolerance int) (Match, error) {
	tmpl, err := LoadTemplate(templatePath)
	if err != nil {
		return Match{}, err
	}
	x, y, w, h, err := regionRect(region)
	if err != nil {
		return Match{}, err
	}
	scr, err := capture(x, y, w, h)
	if err != nil {
		return Match{}, err
	}
	m := matchMultiScale(scr, tmpl, tolerance)
	if m.Found {
		m.X += x // capture-relative → absolute screen coords
		m.Y += y
	}
	return m, nil
}

func (winScreen) FindEdge(templatePath string, region Region, tolerance int) (Match, error) {
	tmpl, err := LoadTemplate(templatePath)
	if err != nil {
		return Match{}, err
	}
	x, y, w, h, err := regionRect(region)
	if err != nil {
		return Match{}, err
	}
	scr, err := capture(x, y, w, h)
	if err != nil {
		return Match{}, err
	}
	// Match the template's OUTLINE against the screen via a distance transform: only the
	// shape lines up (so a recoloured/re-themed button resolves) and it tolerates anti-
	// aliasing / a small shift. tolerance is unused here — edge slack is fixed.
	_ = tolerance
	m := findEdgeMultiScale(scr, tmpl)
	if m.Found {
		m.X += x
		m.Y += y
	}
	return m, nil
}

// CaptureVirtual grabs the WHOLE virtual desktop (all monitors) and returns the frame
// plus its absolute top-left origin — negative when a monitor sits left of / above the
// primary. The recorder uses it to capture a click target big enough to contain the whole
// button (then crops to the button), so a control wider than a fixed patch is recognised
// rather than clipped to a fragment. Map an absolute point into the frame by subtracting
// the origin. The frame's own bounds are 0-origin.
func CaptureVirtual() (img *image.RGBA, originX, originY int, err error) {
	x, y, w, h, err := regionRect(Region{})
	if err != nil {
		return nil, 0, 0, err
	}
	img, err = capture(x, y, w, h)
	return img, x, y, err
}

// CaptureRegion grabs a screen rectangle as an *image.RGBA (absolute screen
// coords). Used by the recorder to snapshot a click target for image matching.
func CaptureRegion(x, y, w, h int) (*image.RGBA, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("screen: empty capture region")
	}
	return capture(x, y, w, h)
}

// capture grabs a screen rectangle into an *image.RGBA via GDI BitBlt+GetDIBits.
func capture(x, y, w, h int) (*image.RGBA, error) {
	hScreen, _, _ := procGetDC.Call(0)
	if hScreen == 0 {
		return nil, fmt.Errorf("screen: GetDC failed")
	}
	defer procReleaseDC.Call(0, hScreen)

	hMem, _, _ := procCreateCompatibleDC.Call(hScreen)
	if hMem == 0 {
		return nil, fmt.Errorf("screen: CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(hMem)

	hBmp, _, _ := procCreateCompatibleBmp.Call(hScreen, uintptr(w), uintptr(h))
	if hBmp == 0 {
		return nil, fmt.Errorf("screen: CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(hBmp)

	procSelectObject.Call(hMem, hBmp)
	procBitBlt.Call(hMem, 0, 0, uintptr(w), uintptr(h), hScreen, uintptr(x), uintptr(y), srcCopy)

	bmi := bitmapInfoHeader{
		Size:        40,
		Width:       int32(w),
		Height:      -int32(h), // negative → top-down rows
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
	}
	buf := make([]byte, w*h*4)
	ret, _, _ := procGetDIBits.Call(hMem, hBmp, 0, uintptr(h),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bmi)), dibRGB)
	if ret == 0 {
		return nil, fmt.Errorf("screen: GetDIBits failed")
	}

	// DIB is BGRA; convert to RGBA.
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i+3 < len(buf); i += 4 {
		img.Pix[i+0] = buf[i+2] // R
		img.Pix[i+1] = buf[i+1] // G
		img.Pix[i+2] = buf[i+0] // B
		img.Pix[i+3] = 255
	}
	return img, nil
}
