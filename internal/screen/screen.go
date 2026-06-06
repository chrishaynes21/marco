// Package screen is the platform "read the screen" capability: pixel reads and
// template image search (find-an-image-on-screen, like AHK's ImageSearch). The
// matcher is pure Go and unit-testable; capture is per-OS (screen_windows.go),
// so this is a first-class native automation capability, Windows first with
// macOS/Linux additive behind the same interface.
package screen

import (
	"errors"
	"image"
	"image/draw"
	_ "image/jpeg" // decode jpeg templates
	_ "image/png"  // decode png templates
	"os"
)

// ErrUnsupported is returned by New on platforms without a capture backend yet.
var ErrUnsupported = errors.New("screen: not supported on this platform")

// Region is a screen rectangle; the zero value means the whole virtual screen.
type Region struct{ X1, Y1, X2, Y2 int }

// Empty reports whether the region is unset (full screen).
func (r Region) Empty() bool { return r == Region{} }

// Match is the result of a Find: the center of the located image (absolute
// screen coords) and whether it was found.
type Match struct {
	X, Y  int
	Found bool
}

// Screen reads the display.
type Screen interface {
	// Pixel returns the RGB color (0xRRGGBB) at an absolute screen coordinate.
	Pixel(x, y int) (uint32, error)
	// Find searches the region (or the whole screen) for the template image,
	// allowing per-channel color difference up to tolerance. Returns the center
	// of the first match.
	Find(templatePath string, region Region, tolerance int) (Match, error)
}

// LoadTemplate decodes a PNG/JPEG template into an *image.RGBA.
func LoadTemplate(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return toRGBA(img), nil
}

func toRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

// match scans scr for tmpl, allowing per-channel difference up to tol. Returns
// the center of the first (top-left-most) match in scr's coordinate space.
// Pure and OS-agnostic — the unit-tested core of Find.
func match(scr, tmpl *image.RGBA, tol int) Match {
	sw, sh := scr.Rect.Dx(), scr.Rect.Dy()
	tw, th := tmpl.Rect.Dx(), tmpl.Rect.Dy()
	if tw == 0 || th == 0 || tw > sw || th > sh {
		return Match{}
	}
	t0r, t0g, t0b := at(tmpl, 0, 0)
	for y := 0; y <= sh-th; y++ {
		for x := 0; x <= sw-tw; x++ {
			// Quick reject on the template's top-left pixel.
			sr, sg, sb := at(scr, x, y)
			if !within(sr, sg, sb, t0r, t0g, t0b, tol) {
				continue
			}
			if fullMatch(scr, tmpl, x, y, tol) {
				return Match{X: x + tw/2, Y: y + th/2, Found: true}
			}
		}
	}
	return Match{}
}

func fullMatch(scr, tmpl *image.RGBA, ox, oy, tol int) bool {
	tw, th := tmpl.Rect.Dx(), tmpl.Rect.Dy()
	for ty := 0; ty < th; ty++ {
		for tx := 0; tx < tw; tx++ {
			tr, tg, tb := at(tmpl, tx, ty)
			sr, sg, sb := at(scr, ox+tx, oy+ty)
			if !within(sr, sg, sb, tr, tg, tb, tol) {
				return false
			}
		}
	}
	return true
}

// at returns the RGB of a pixel at the image-local coordinate.
func at(img *image.RGBA, x, y int) (r, g, b int) {
	i := img.PixOffset(img.Rect.Min.X+x, img.Rect.Min.Y+y)
	return int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
}

func within(r1, g1, b1, r2, g2, b2, tol int) bool {
	return iabs(r1-r2) <= tol && iabs(g1-g2) <= tol && iabs(b1-b2) <= tol
}

func iabs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
