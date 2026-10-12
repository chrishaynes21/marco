package observe_test

import (
	"image"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// ONE STATEMENT OF THE NORMALISATION, AND IT HAS TO BE RIGHT.
//
// Two callers now: the offline replay and the live observer. They must agree exactly, because
// the whole point of the replay is that it measures what production does — four divisions
// written twice is this repository's worst defect class.
//
// Deleting the origin subtraction, or the area guard, must fail this.
func TestRectanglesAreNormalisedToTheImageTheyCameFrom(t *testing.T) {
	// A whole image at the origin: halves and quarters come out as halves and quarters.
	got := observe.NormaliseRects(image.Rect(0, 0, 400, 200), []image.Rectangle{
		image.Rect(0, 0, 200, 100),
		image.Rect(200, 100, 400, 200),
	})
	if len(got) != 2 {
		t.Fatalf("%d region(s) from 2 rectangles", len(got))
	}
	if got[0] != (observe.Region{X: 0, Y: 0, Width: 0.5, Height: 0.5}) {
		t.Errorf("the top-left quarter normalised to %+v", got[0])
	}
	if got[1] != (observe.Region{X: 0.5, Y: 0.5, Width: 0.5, Height: 0.5}) {
		t.Errorf("the bottom-right quarter normalised to %+v", got[1])
	}

	// A SUB-IMAGE DOES NOT START AT ZERO. A capture of a window on a second monitor, or
	// any cropped image, has a non-zero origin — and a normalisation that assumed
	// otherwise would place every region by however far the crop was offset, which looks
	// plausible and is wrong by a whole window.
	off := observe.NormaliseRects(image.Rect(1000, 500, 1400, 700), []image.Rectangle{
		image.Rect(1000, 500, 1200, 600),
	})
	if len(off) != 1 || off[0] != (observe.Region{X: 0, Y: 0, Width: 0.5, Height: 0.5}) {
		t.Errorf("a rectangle at a sub-image's own origin normalised to %+v, want the "+
			"top-left quarter. The image's origin must be subtracted", off)
	}

	// A rectangle with no area sits nowhere and is dropped rather than placed at a point.
	none := observe.NormaliseRects(image.Rect(0, 0, 100, 100), []image.Rectangle{
		image.Rect(10, 10, 10, 20), // no width
		image.Rect(10, 10, 20, 10), // no height
		image.Rect(10, 10, 20, 20), // real
	})
	if len(none) != 1 {
		t.Errorf("%d region(s) from one real rectangle and two area-less ones: %+v",
			len(none), none)
	}
	// An image with no area produces nothing rather than dividing by zero.
	if n := observe.NormaliseRects(image.Rect(0, 0, 0, 0),
		[]image.Rectangle{image.Rect(0, 0, 1, 1)}); n != nil {
		t.Errorf("an empty image produced %+v", n)
	}
}
