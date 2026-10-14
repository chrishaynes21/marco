package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// WHERE THE DEMONSTRATION AND THE PRESENT AGREE.
//
// The numbers in the first case are the measured ones. A live look matched Rocket League's PLAY
// button with a region running from the window's left edge to 132px in; the saved reading was
// 26..192. The button itself is 62..370. The middle of the match was x 66, three pixels inside
// the button's border, and the click did nothing. The middle of the agreed part is x 79, which
// is the first letter of PLAY.
func TestTheAgreedPartIsBoundedByTheSmallerOfTheTwo(t *testing.T) {
	const w = 1920.0
	found := observe.Region{X: 0, Y: 0.4241, Width: 0.0688, Height: 0.0407}
	saved := observe.Region{X: 0.013541, Y: 0.433333, Width: 0.086458, Height: 0.037037}

	got, ok := observe.Agreed(found, []observe.Region{saved})
	if !ok {
		t.Fatal("two regions that overlap were reported as not agreeing")
	}
	if x := round(got.X * w); x != 26 {
		t.Errorf("the agreed part starts at %d, want 26 — the saved reading's edge", x)
	}
	if x := round((got.X + got.Width) * w); x != 132 {
		t.Errorf("the agreed part ends at %d, want 132 — the match's edge", x)
	}
	mid := round((got.X + got.Width/2) * w)
	if mid != 79 {
		t.Errorf("the point is x %d, want 79", mid)
	}
	// AND IT IS INSIDE THE BUTTON, which the middle of the match was not.
	if mid < 62 || mid > 370 {
		t.Errorf("x %d is outside the measured button (62..370)", mid)
	}
}

// NO OVERLAP IS A CONTRADICTION, and a contradiction is not a point.
func TestRegionsThatDoNotTouchDoNotAgree(t *testing.T) {
	a := observe.Region{X: 0.1, Y: 0.1, Width: 0.1, Height: 0.1}
	b := observe.Region{X: 0.5, Y: 0.5, Width: 0.1, Height: 0.1}
	if got, ok := observe.Agreed(a, []observe.Region{b}); ok {
		t.Fatalf("two regions that do not touch agreed on %v", got)
	}
	// Touching at an edge is not overlapping either: a shared boundary encloses nothing.
	edge := observe.Region{X: 0.2, Y: 0.1, Width: 0.1, Height: 0.1}
	if _, ok := observe.Agreed(a, []observe.Region{edge}); ok {
		t.Error("regions sharing only an edge were reported as agreeing")
	}
}

// AND THE LARGEST AGREEMENT WINS, when a control was read more than once.
//
// Several readings are several looks at the same control, and the one that agrees most with what
// is in front of Marco now is the one with the most evidence behind the point it gives.
func TestTheReadingThatAgreesMostIsTheOneUsed(t *testing.T) {
	found := observe.Region{X: 0.1, Y: 0.1, Width: 0.4, Height: 0.1}
	saved := []observe.Region{
		{X: 0.45, Y: 0.1, Width: 0.1, Height: 0.1},  // a slice at the far end
		{X: 0.12, Y: 0.1, Width: 0.3, Height: 0.1},  // most of it
		{X: 0.9, Y: 0.9, Width: 0.05, Height: 0.05}, // nowhere near
	}
	got, ok := observe.Agreed(found, saved)
	if !ok {
		t.Fatal("nothing agreed")
	}
	if got.X < 0.11 || got.X > 0.13 {
		t.Errorf("the agreed part starts at %.3f, want the larger overlap at 0.12",
			got.X)
	}
	if got.Width < 0.25 {
		t.Errorf("the agreed part is %.3f wide, want the larger overlap", got.Width)
	}
}

// AND A CONTROL WITH NO SAVED READINGS IS NOT AGREED WITH, rather than agreed with trivially.
func TestNoSavedReadingsIsNoAgreement(t *testing.T) {
	found := observe.Region{X: 0.1, Y: 0.1, Width: 0.1, Height: 0.1}
	if _, ok := observe.Agreed(found, nil); ok {
		t.Error("a control with no readings agreed with the first thing it was shown")
	}
}

// round is how the runner turns a fraction into a pixel, so the test measures what it measures.
func round(f float64) int { return int(f + 0.5) }
