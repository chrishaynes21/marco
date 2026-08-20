package referent_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/referent"
)

// The arithmetic, with the origin never assumed.
//
// Every test here uses a window that is NOT at (0,0) and NOT square. Both are deliberate: a square
// window at the origin makes the two commonest coordinate mistakes — mapping X against the height,
// and forgetting to add the window's own position — completely invisible.

func frame(x, y, w, h int) referent.Frame {
	return referent.Frame{X: x, Y: y, Width: w, Height: h}
}

// A region maps to the desktop rectangle it describes.
func TestARegionMapsOntoItsWindow(t *testing.T) {
	f := frame(100, 200, 1000, 800)
	got := referent.Map([]referent.Norm{{X: 0.10, Y: 0.20, Width: 0.30, Height: 0.40}}, f, f)
	if !got.Drawable() {
		t.Fatalf("nothing to draw: %q", got.Reason)
	}
	want := referent.Box{X: 100 + 100, Y: 200 + 160, Width: 300, Height: 320}
	if got.Boxes[0] != want {
		t.Fatalf("mapped to %v, want %v", got.Boxes[0], want)
	}
}

// X uses the width and Y uses the height, and swapping them is visible.
//
// The window is deliberately 1000×800: with a square one, mapping X against the height would
// produce the right answer and the mistake would ship.
func TestXUsesTheWidthAndYUsesTheHeight(t *testing.T) {
	f := frame(0, 0, 1000, 800)
	got := referent.Map([]referent.Norm{{X: 0.5, Y: 0.5, Width: 0.1, Height: 0.1}}, f, f)
	if !got.Drawable() {
		t.Fatalf("nothing to draw: %q", got.Reason)
	}
	if got.Boxes[0].X != 500 {
		t.Errorf("x mapped to %d, want 500 (half of the WIDTH)", got.Boxes[0].X)
	}
	if got.Boxes[0].Y != 400 {
		t.Errorf("y mapped to %d, want 400 (half of the HEIGHT)", got.Boxes[0].Y)
	}
	if got.Boxes[0].Width != 100 || got.Boxes[0].Height != 80 {
		t.Errorf("size mapped to %dx%d, want 100x80",
			got.Boxes[0].Width, got.Boxes[0].Height)
	}
}

// A monitor to the left, or above, has a negative origin. Nothing special happens.
func TestANegativeDesktopOriginNeedsNoSpecialCase(t *testing.T) {
	for _, f := range []referent.Frame{
		frame(-1920, 0, 800, 600),     // the monitor to the left
		frame(0, -1080, 800, 600),     // the monitor above
		frame(-1920, -1080, 800, 600), // above and to the left
		frame(2560, 140, 800, 600),    // the ordinary second monitor, to the right
	} {
		got := referent.Map([]referent.Norm{{X: 0.25, Y: 0.5, Width: 0.5, Height: 0.25}}, f, f)
		if !got.Drawable() {
			t.Fatalf("%v: nothing to draw: %q", f, got.Reason)
		}
		want := referent.Box{
			X: f.X + 200, Y: f.Y + 300, Width: 400, Height: 150,
		}
		if got.Boxes[0] != want {
			t.Errorf("%v mapped to %v, want %v. A monitor is not assumed to start at "+
				"(0,0), and clamping one that does not is how a highlight lands on the "+
				"wrong screen", f, got.Boxes[0], want)
		}
	}
}

// Every region gets its own box, in order.
func TestEveryRegionKeepsItsOwnBox(t *testing.T) {
	f := frame(-300, 50, 400, 1000)
	in := []referent.Norm{
		{X: 0.1, Y: 0.0, Width: 0.8, Height: 0.05},
		{X: 0.1, Y: 0.1, Width: 0.8, Height: 0.05},
		{X: 0.1, Y: 0.2, Width: 0.8, Height: 0.05},
	}
	got := referent.Map(in, f, f)
	if len(got.Boxes) != len(in) {
		t.Fatalf("%d regions produced %d box(es); a set of choices is not one rectangle",
			len(in), len(got.Boxes))
	}
	for i := 1; i < len(got.Boxes); i++ {
		if got.Boxes[i].Y <= got.Boxes[i-1].Y {
			t.Errorf("box %d is not below box %d; the order was not preserved", i, i-1)
		}
	}
}

// A window that has moved or resized refuses, rather than drawing where it used to be.
func TestAMovedWindowRefusesRatherThanGuessing(t *testing.T) {
	at := frame(100, 200, 1000, 800)
	regions := []referent.Norm{{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2}}

	for _, now := range []referent.Frame{
		frame(140, 200, 1000, 800), // moved
		frame(100, 200, 900, 800),  // resized
		frame(100, 260, 1000, 700), // both
	} {
		got := referent.Map(regions, at, now)
		if got.Drawable() {
			t.Errorf("%v → %v drew anyway, at %v. The regions are proportions of a "+
				"rectangle that is no longer there", at, now, got.Boxes)
		}
		if got.Reason != referent.WindowMoved {
			t.Errorf("%v → %v refused for %q", at, now, got.Reason)
		}
	}
	// And the unchanged case still draws, so the guard is not simply refusing everything.
	if !referent.Map(regions, at, at).Drawable() {
		t.Error("an unmoved window refused")
	}
}

// A region that does not lie inside its own window is refused, never clamped.
func TestARegionOutsideItsWindowIsRefusedNotClamped(t *testing.T) {
	f := frame(0, 0, 1000, 800)
	// The shape of the real Explorer record: a group whose stored geometry sits mostly
	// above the window it was normalised against.
	for _, r := range []referent.Norm{
		{X: 0.04, Y: -1.73, Width: 0.11, Height: 1.30},
		{X: 0.9, Y: 0.1, Width: 0.5, Height: 0.1},
		{X: 0.1, Y: 0.95, Width: 0.1, Height: 0.5},
	} {
		got := referent.Map([]referent.Norm{r}, f, f)
		if got.Drawable() {
			t.Errorf("%v was clamped into the window and drawn at %v. That rectangle is "+
				"inside the window and describes nothing that was measured", r, got.Boxes)
		}
		if got.Reason != referent.OffFrame {
			t.Errorf("%v refused for %q", r, got.Reason)
		}
	}
}

// Nothing to place, or nothing with area, produces nothing to draw.
func TestThereIsAlwaysAReasonAndNeverAnApproximation(t *testing.T) {
	f := frame(10, 20, 640, 480)
	for _, tc := range []struct {
		name    string
		regions []referent.Norm
		at, now referent.Frame
		want    referent.Unmappable
	}{
		{"no window", []referent.Norm{{X: 0, Y: 0, Width: 1, Height: 1}},
			referent.Frame{}, referent.Frame{}, referent.NoWindow},
		{"no regions", nil, f, f, referent.NoWindow},
		{"no area", []referent.Norm{{X: 0.1, Y: 0.1}}, f, f, referent.Degenerate},
	} {
		got := referent.Map(tc.regions, tc.at, tc.now)
		if got.Drawable() {
			t.Errorf("%s: drew %v", tc.name, got.Boxes)
		}
		if got.Reason != tc.want {
			t.Errorf("%s: reason %q, want %q", tc.name, got.Reason, tc.want)
		}
		if got.Reason.Say() == "" {
			t.Errorf("%s: the refusal has no reading", tc.name)
		}
	}
	if referent.Mappable.Say() != "" {
		t.Error("a successful mapping produced an apology")
	}
	if (referent.Mapping{Reason: referent.Mappable}).Drawable() {
		t.Error("a mapping with no boxes says it is drawable")
	}
}

// A box lands on the nearest pixel, not the one below it.
//
// Truncation loses up to a pixel on every edge in the same direction, so an outline sits
// consistently up and to the left of the control and is consistently short — visibly off on a
// small control, and exactly the kind of near-miss a person reads as Marco meaning something else.
func TestARegionRoundsToTheNearestPixel(t *testing.T) {
	left := referent.Map([]referent.Norm{{X: 0.335, Y: 0.335, Width: 0.33, Height: 0.33}},
		frame(-1000, 0, 999, 999), frame(-1000, 0, 999, 999))
	right := referent.Map([]referent.Norm{{X: 0.335, Y: 0.335, Width: 0.33, Height: 0.33}},
		frame(1000, 0, 999, 999), frame(1000, 0, 999, 999))
	if !left.Drawable() || !right.Drawable() {
		t.Fatal("one side produced nothing")
	}
	if left.Boxes[0].X-(-1000) != right.Boxes[0].X-1000 {
		t.Errorf("the same region rounded to offset %d on the left and %d on the right",
			left.Boxes[0].X+1000, right.Boxes[0].X-1000)
	}
	if left.Boxes[0].Width != right.Boxes[0].Width {
		t.Errorf("widths differ across the origin: %d vs %d",
			left.Boxes[0].Width, right.Boxes[0].Width)
	}
}

// The nearest pixel, in both position and size.
func TestNearestPixelInPositionAndSize(t *testing.T) {
	f := frame(0, 0, 999, 999)
	got := referent.Map([]referent.Norm{{X: 0.335, Y: 0.335, Width: 0.335, Height: 0.335}}, f, f)
	if !got.Drawable() {
		t.Fatalf("nothing to draw: %q", got.Reason)
	}
	// 0.335 x 999 = 334.665 — nearest is 335, truncation gives 334.
	want := referent.Box{X: 335, Y: 335, Width: 335, Height: 335}
	if got.Boxes[0] != want {
		t.Fatalf("mapped to %v, want %v. Truncating puts every edge up to a pixel short, "+
			"always in the same direction", got.Boxes[0], want)
	}
}
