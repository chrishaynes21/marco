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
)

const (
	smCxScreen = 0
	smCyScreen = 1
	srcCopy    = 0x00CC0020
	biRGB      = 0
	dibRGB     = 0
)

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

func (winScreen) Find(templatePath string, region Region, tolerance int) (Match, error) {
	tmpl, err := LoadTemplate(templatePath)
	if err != nil {
		return Match{}, err
	}
	x, y, w, h := region.X1, region.Y1, region.X2-region.X1, region.Y2-region.Y1
	if region.Empty() {
		cx, _, _ := procGetSystemMetrics.Call(smCxScreen)
		cy, _, _ := procGetSystemMetrics.Call(smCyScreen)
		x, y, w, h = 0, 0, int(cx), int(cy)
	}
	if w <= 0 || h <= 0 {
		return Match{}, fmt.Errorf("screen: empty search region")
	}
	scr, err := capture(x, y, w, h)
	if err != nil {
		return Match{}, err
	}
	m := match(scr, tmpl, tolerance)
	if m.Found {
		m.X += x // capture-relative → absolute screen coords
		m.Y += y
	}
	return m, nil
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
