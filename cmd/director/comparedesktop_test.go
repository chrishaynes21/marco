package main

import (
	"image"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// THE COMPARISON RESTS ENTIRELY ON THIS CONVERSION.
//
// Production stores boxes as proportions of the window frame; the detector returns pixels of
// the image. Every count in Experiment 016 — matched, detector-only, production-only — is
// the result of putting those two in one space and measuring overlap.
//
// Get it wrong and nothing complains. Divide x by the height instead of the width and every
// detection lands somewhere else on the frame, overlap collapses toward zero, and the report
// reads "the detector found 300 things production had never seen" in clean, confident columns.
// That is the exact shape of conclusion this project has been burned by before: a number that
// is wrong for an infrastructural reason and looks like a finding.
//
// The frames are all non-square on purpose here. A square fixture cannot tell width from
// height, and would let the mutation through.

func TestRelativeToImageUsesWidthForXAndHeightForY(t *testing.T) {
	frame := image.Rect(0, 0, 1000, 500) // deliberately not square

	got := relativeToImage(image.Rect(250, 250, 500, 375), frame)
	want := observe.Region{X: 0.25, Y: 0.5, Width: 0.25, Height: 0.25}
	if !nearRegion(got, want) {
		t.Errorf("relativeToImage = %+v, want %+v\n"+
			"x and width scale by the frame WIDTH; y and height by its HEIGHT. Crossing "+
			"them puts every detection somewhere else on the frame and the comparison "+
			"silently measures nothing.", got, want)
	}

	// The whole frame maps to the unit square, whatever its aspect.
	if got := relativeToImage(frame, frame); !nearRegion(got,
		observe.Region{X: 0, Y: 0, Width: 1, Height: 1}) {
		t.Errorf("the whole frame mapped to %+v, want the unit square", got)
	}

	// A frame not anchored at the origin still maps from its own corner. A screenshot
	// decoded with a non-zero Min is rare and not impossible, and treating its Min as 0
	// would offset every box by the corner.
	off := image.Rect(100, 200, 1100, 700)
	if got := relativeToImage(image.Rect(100, 200, 600, 450), off); !nearRegion(got,
		observe.Region{X: 0, Y: 0, Width: 0.5, Height: 0.5}) {
		t.Errorf("a box at the corner of an offset frame mapped to %+v, want the top-left "+
			"quarter", got)
	}

	// A degenerate frame returns nothing rather than dividing by zero.
	if got := relativeToImage(image.Rect(0, 0, 10, 10), image.Rect(0, 0, 0, 0)); got !=
		(observe.Region{}) {
		t.Errorf("an empty frame produced %+v, want the zero region", got)
	}
}

// A detection is "inside something already known" by its centre, and containers do not count.
//
// The window element spans the whole frame and a pane usually spans most of it, so counting
// them would make every detection on screen "already known" — a 100% that means nothing. The
// figure this feeds is load-bearing in Experiment 016, where all 302 detector-only boxes came
// back contained; that number is only worth reading because these two roles are excluded.
func TestContainedInAnyIgnoresTheContainers(t *testing.T) {
	els := []desktopElement{
		{ID: "w", Role: "window", Bounds: observe.Region{X: 0, Y: 0, Width: 1, Height: 1}},
		{ID: "p", Role: "pane", Bounds: observe.Region{X: 0, Y: 0, Width: 1, Height: 1}},
		{ID: "b", Role: "button", Bounds: observe.Region{X: 0.5, Y: 0.5, Width: 0.1, Height: 0.05}},
	}

	inButton := observe.Region{X: 0.52, Y: 0.51, Width: 0.02, Height: 0.02}
	if !containedInAny(inButton, els) {
		t.Error("a detection sitting inside a known button was not counted as contained")
	}

	// Inside the window and the pane, and nothing else. Must be false, or the containment
	// figure is just "is it on screen".
	elsewhere := observe.Region{X: 0.1, Y: 0.1, Width: 0.02, Height: 0.02}
	if containedInAny(elsewhere, els) {
		t.Error("a detection in empty space was counted as contained.\n" +
			"The window and pane roles span the whole frame; counting them makes every " +
			"detection 'already known' and the measurement says nothing at all.")
	}
}

func nearRegion(a, b observe.Region) bool {
	const eps = 1e-9
	d := func(x, y float64) bool { return x-y < eps && y-x < eps }
	return d(a.X, b.X) && d(a.Y, b.Y) && d(a.Width, b.Width) && d(a.Height, b.Height)
}
