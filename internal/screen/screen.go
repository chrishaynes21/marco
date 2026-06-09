// Package screen is the platform "read the screen" capability: pixel reads and
// template image search (find-an-image-on-screen, like AHK's ImageSearch). The
// matcher is pure Go and unit-testable; capture is per-OS (screen_windows.go),
// so this is a first-class native automation capability, Windows first with
// macOS/Linux additive behind the same interface.
package screen

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	_ "image/jpeg" // decode jpeg templates
	"image/png"
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

// EncodePNG encodes an image as PNG bytes — used to persist a captured click
// target as a route template. OS-agnostic.
func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DistinctiveTolerance is the per-channel color difference under which two
// pixels count as "the same" when judging whether a patch is distinctive.
const DistinctiveTolerance = 12

// Distinctive reports whether a captured patch has enough visual variety to be
// worth matching by image — i.e. it isn't a near-uniform area (blank canvas,
// flat background) that would match almost anywhere on screen. A click on such
// an area is better left as a fixed coordinate. The test: at least 15% of pixels
// differ from the top-left pixel by more than DistinctiveTolerance on a channel.
func Distinctive(img *image.RGBA) bool {
	if img == nil {
		return false
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	total := w * h
	if total == 0 {
		return false
	}
	r0, g0, b0 := at(img, 0, 0)
	differing := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b := at(img, x, y)
			if !within(r, g, b, r0, g0, b0, DistinctiveTolerance) {
				differing++
			}
		}
	}
	return differing*100 >= total*15
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

// MatchThreshold is the fraction of a template's pixels that must be within
// tolerance for a location to count as a match. Below 1.0 on purpose: requiring
// EVERY pixel match is far too brittle — a mouse cursor, tooltip, focus glow, or
// one anti-aliased edge over the target makes an exact match fail. Accepting ~88%
// tolerates that incidental overlay while still being specific to the target.
const MatchThreshold = 0.88

// match scans scr for tmpl, allowing per-channel difference up to tol, and
// returns the center of the BEST-scoring location whose matching-pixel fraction
// is at least MatchThreshold. Pure and OS-agnostic — the unit-tested core of Find.
func match(scr, tmpl *image.RGBA, tol int) Match {
	sw, sh := scr.Rect.Dx(), scr.Rect.Dy()
	tw, th := tmpl.Rect.Dx(), tmpl.Rect.Dy()
	if tw == 0 || th == 0 || tw > sw || th > sh {
		return Match{}
	}
	total := tw * th
	maxMiss := total - int(float64(total)*MatchThreshold) // mismatches allowed
	bestMiss := maxMiss + 1
	best := Match{}
	for y := 0; y <= sh-th; y++ {
		for x := 0; x <= sw-tw; x++ {
			// Cheap pre-filter: 5 spread sample points must mostly agree, so most
			// positions are rejected without the full per-pixel scan (and it
			// doesn't hinge on any single pixel, unlike a top-left-only reject).
			if !preFilter(scr, tmpl, x, y, tol) {
				continue
			}
			miss := countMismatch(scr, tmpl, x, y, tol, bestMiss-1)
			if miss < bestMiss {
				bestMiss = miss
				best = Match{X: x + tw/2, Y: y + th/2, Found: true}
				if miss == 0 {
					return best // perfect; can't do better
				}
			}
		}
	}
	return best
}

// preFilter samples 5 points (corners + center) of the template at (ox,oy) and
// requires at least 3 within tolerance — a fast rejection that tolerates a couple
// of off pixels (a cursor sitting on a corner, say).
func preFilter(scr, tmpl *image.RGBA, ox, oy, tol int) bool {
	tw, th := tmpl.Rect.Dx(), tmpl.Rect.Dy()
	pts := [5][2]int{{0, 0}, {tw - 1, 0}, {0, th - 1}, {tw - 1, th - 1}, {tw / 2, th / 2}}
	hits := 0
	for _, p := range pts {
		tr, tg, tb := at(tmpl, p[0], p[1])
		sr, sg, sb := at(scr, ox+p[0], oy+p[1])
		if within(sr, sg, sb, tr, tg, tb, tol) {
			hits++
		}
	}
	return hits >= 3
}

// countMismatch returns how many template pixels at (ox,oy) differ by more than
// tol, aborting early once it exceeds limit (so hopeless positions bail fast).
func countMismatch(scr, tmpl *image.RGBA, ox, oy, tol, limit int) int {
	tw, th := tmpl.Rect.Dx(), tmpl.Rect.Dy()
	miss := 0
	for ty := 0; ty < th; ty++ {
		for tx := 0; tx < tw; tx++ {
			tr, tg, tb := at(tmpl, tx, ty)
			sr, sg, sb := at(scr, ox+tx, oy+ty)
			if !within(sr, sg, sb, tr, tg, tb, tol) {
				miss++
				if miss > limit {
					return miss
				}
			}
		}
	}
	return miss
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
