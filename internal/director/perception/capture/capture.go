// Package capture is the boundary between "a picture of the screen" and everything that
// reads one.
//
// It holds no capture implementation. What it holds is the SHAPE a picture arrives in —
// where it came from, at what scale, when, and how to put a box read from it back onto the
// desktop — because every provider that reads pixels needs exactly that, and two copies of
// it would be two ways to get the same arithmetic wrong.
//
// # Why this is its own package
//
// It began inside the OCR provider, which was right while OCR was the only thing reading
// pixels. A second one — the vision provider — made the choice explicit: either duplicate
// the coordinate transform, or share it. Duplicating it would mean two implementations of
// the one calculation whose failure mode is silent and severe. A box misplaced by a wrong
// scale does not look like a bug; it looks like a slightly misaligned overlay, and it lands
// confidently on whatever is now at those coordinates.
//
// The OCR package keeps its names as aliases, so nothing that used them changed.
package capture

import (
	"context"
	"fmt"
	"image"
	"math"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// WindowCapture takes a picture of a window.
type WindowCapture interface {
	CaptureWindow(ctx context.Context, window directorapi.Window) (Image, error)
}

// RegionCapture takes a picture of a rectangle of the desktop. Optional: a capture
// implementation that can only do whole windows implements WindowCapture alone.
type RegionCapture interface {
	CaptureRegion(ctx context.Context, region directorapi.Rect) (Image, error)
}

// Image is a picture plus everything needed to say where it came from.
//
// The fields exist because getting any one of them wrong misplaces every observation
// derived from the image, and a misplaced observation that LOOKS right is the failure mode
// this whole layer has to avoid: it would merge into the wrong control and read as
// corroboration.
type Image struct {
	Image image.Image
	// Bounds is where the image came from, in canonical desktop coordinates. May be
	// negative: a second monitor to the left of the primary has negative X, and that
	// is ordinary rather than exceptional.
	Bounds directorapi.Rect
	// ContentOrigin is where the image's (0,0) sits within Bounds — non-zero when the
	// capture excludes a window's frame or shadow.
	ContentOrigin directorapi.Point
	// Scale is the DPI scaling of the captured surface, 1.0 at 96 DPI.
	Scale      float64
	CapturedAt time.Time
	// Transform converts image-local coordinates to canonical desktop ones.
	Transform Transform
	// WindowID and Application scope everything read from this image.
	WindowID    directorapi.WindowID
	Application string
	// WindowBoundsAtCapture is where the window was when the picture was taken.
	// Compared against the window's bounds afterwards to detect a window that MOVED
	// mid-capture, which would place every observation confidently in the wrong place.
	WindowBoundsAtCapture directorapi.Rect

	// Target is which validated window GENERATION these pixels belong to, established
	// AFTER the capture returned.
	//
	// The existing ownership check in the capture backend compares application NAMES
	// before and after the grab. That catches a window changing hands to a different
	// application; it does not catch a window of the SAME application being replaced —
	// generation 7 to generation 8 of one editor passes it unremarked, which is
	// precisely the race that matters here.
	//
	// So this is resolved separately, from the platform as it is once the pixels are
	// in hand. Nil means provenance could not be established, which is not the same as
	// "fine": everything downstream treats an absent target as unproven.
	Target *directorapi.TargetProvenance
}

// ── coordinate transform ──────────────────────────────────────────────────────

// Transform converts between coordinate spaces.
//
// Explicit rather than implied by arithmetic scattered through a provider. Mixed-DPI
// desktops and negative monitor origins are both ordinary, and both produce plausible
// wrong answers when the conversion is assumed rather than stated: a 150%-scaled window
// silently reports every box two-thirds of the way toward the origin, which looks like a
// slightly misaligned overlay rather than like a bug.
type Transform struct {
	SourceSpace string
	TargetSpace string

	ScaleX float64
	ScaleY float64

	OffsetX float64
	OffsetY float64
}

// Coordinate space names.
const (
	SpaceImage   = "image"
	SpaceDesktop = "desktop"
)

// Identity is the no-op transform.
func Identity() Transform {
	return Transform{
		SourceSpace: SpaceImage, TargetSpace: SpaceDesktop, ScaleX: 1, ScaleY: 1,
	}
}

// New maps an image onto a desktop rectangle at the given scale.
func New(origin directorapi.Point, scale float64) Transform {
	if scale <= 0 {
		scale = 1
	}
	return Transform{
		SourceSpace: SpaceImage, TargetSpace: SpaceDesktop,
		ScaleX: scale, ScaleY: scale,
		OffsetX: float64(origin.X), OffsetY: float64(origin.Y),
	}
}

// Apply converts an image-local rectangle into the target space.
//
// Rounds the edges rather than the position and size, so a box never loses or gains a
// pixel of width from where it happens to sit. Negative results are correct and preserved.
func (t Transform) Apply(r image.Rectangle) directorapi.Rect {
	sx, sy := t.ScaleX, t.ScaleY
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	x0 := int(math.Round(float64(r.Min.X)*sx + t.OffsetX))
	y0 := int(math.Round(float64(r.Min.Y)*sy + t.OffsetY))
	x1 := int(math.Round(float64(r.Max.X)*sx + t.OffsetX))
	y1 := int(math.Round(float64(r.Max.Y)*sy + t.OffsetY))
	return directorapi.Rect{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}

// Invert converts a desktop rectangle back into image-local coordinates, for cropping a
// region out of a window capture.
func (t Transform) Invert(r directorapi.Rect) image.Rectangle {
	sx, sy := t.ScaleX, t.ScaleY
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	x0 := int(math.Round((float64(r.X) - t.OffsetX) / sx))
	y0 := int(math.Round((float64(r.Y) - t.OffsetY) / sy))
	x1 := int(math.Round((float64(r.X+r.Width) - t.OffsetX) / sx))
	y1 := int(math.Round((float64(r.Y+r.Height) - t.OffsetY) / sy))
	return image.Rect(x0, y0, x1, y1)
}

// String describes the transform for diagnostics.
func (t Transform) String() string {
	return fmt.Sprintf("%s→%s scale %.2fx%.2f offset %+.0f%+.0f",
		t.SourceSpace, t.TargetSpace, t.ScaleX, t.ScaleY, t.OffsetX, t.OffsetY)
}

// Moved reports whether a window's bounds changed enough to invalidate a capture.
//
// Exact comparison, deliberately. A window that moved by one pixel moved; there is no
// tolerance that is obviously right, and a wrong tolerance produces confidently misplaced
// observations, which is the failure this check exists to prevent.
func Moved(now, atCapture directorapi.Rect) bool {
	if atCapture == (directorapi.Rect{}) {
		return false // the capture layer did not report it; nothing to compare
	}
	return now != (directorapi.Rect{}) && now != atCapture
}

// Crop returns the sub-image of r, using the fast path when the image supports it.
func Crop(img image.Image, r image.Rectangle) image.Image {
	type subImager interface {
		SubImage(image.Rectangle) image.Image
	}
	if s, ok := img.(subImager); ok {
		return s.SubImage(r)
	}
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dst.Set(x-r.Min.X, y-r.Min.Y, img.At(x, y))
		}
	}
	return dst
}
