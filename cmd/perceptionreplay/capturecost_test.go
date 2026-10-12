package main

import (
	"image"
	"image/color"
	"testing"
	"time"
)

// A reduced observation keeps every Nth pixel, and nothing else.
//
// Nearest-neighbour on purpose: an averaging downscale costs more and blurs the very edges the
// proposer looks for. Measured, the cheap reduction keeps all seventeen click targets on a
// proposed region down to 480x270 — so the expensive one is a cost nobody has a reason to pay.
//
// Deleting the stride, or taking the wrong pixel, must fail this.
func TestAReducedObservationKeepsEveryNthPixel(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 8, 4))
	for y := range 4 {
		for x := range 8 {
			// A value that encodes its own position, so a wrong pixel is visible —
			// and a NON-OPAQUE alpha, so a reduction that copied the source's
			// alpha instead of writing 255 is visible too. A capture is always
			// opaque; a reduced observation that was not would read every shade
			// wrong, and the shading is what the selection reader runs on.
			src.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 9, A: 7})
		}
	}
	got := halve(src, 2)
	if b := got.Bounds(); b.Dx() != 4 || b.Dy() != 2 {
		t.Fatalf("halving an 8x4 image gave %v, want 4x2", b)
	}
	// Each destination pixel is the TOP-LEFT of its block: (0,0), (2,0), (4,0)...
	for y := range 2 {
		for x := range 4 {
			c := got.RGBAAt(x, y)
			if c.R != uint8(x*2) || c.G != uint8(y*2) {
				t.Errorf("pixel %d,%d came from %d,%d, want %d,%d",
					x, y, c.R, c.G, x*2, y*2)
			}
			if c.A != 255 {
				t.Errorf("pixel %d,%d has alpha %d; a reduced observation must "+
					"be opaque or every shade reads wrong", x, y, c.A)
			}
		}
	}
	// A factor of one is the image itself, with no copy and no cost.
	if halve(src, 1) != src {
		t.Error("halving by one copied the image; the /1 row of the table must measure " +
			"no resize at all")
	}
	if halve(src, 0) != src || halve(src, -3) != src {
		t.Error("a factor below one reduced something")
	}
}

// The stage report is percentiles, not a mean.
//
//	Measure median, p95, and worst-case latency, not just average throughput.
//
// A cadence has to survive its p95; an average hides the frame that arrived late, and a frame
// that arrives late is the one that makes a press land in the wrong slot.
//
// Deleting the sort must fail this.
func TestAStageIsReportedAsPercentilesAndNotAMean(t *testing.T) {
	var s stageCost
	// Nine fast samples and one very slow one. The mean is 11ms; the median is 2.
	for range 9 {
		s.add(2 * time.Millisecond)
	}
	s.add(92 * time.Millisecond)

	if m := s.at(50); m < 1.9 || m > 2.1 {
		t.Errorf("the median of nine 2ms samples and one 92ms is %.2f, want 2. A mean "+
			"would be 11 and would describe no frame that happened", m)
	}
	if w := s.at(100); w < 91 || w > 93 {
		t.Errorf("the worst sample reads %.2f, want 92: the slow frame is the one a "+
			"cadence has to survive", w)
	}
	// The samples arrive in time order and the percentiles must not depend on it.
	var rev stageCost
	rev.add(92 * time.Millisecond)
	for range 9 {
		rev.add(2 * time.Millisecond)
	}
	if a, b := s.at(50), rev.at(50); a != b {
		t.Errorf("the same samples in a different order give medians %.2f and %.2f", a, b)
	}
	// The worst case is the one the ORDER can hide: an unsorted read of the reversed
	// samples finds 2ms at the last index and reports the slowest frame as the fastest.
	if w := rev.at(100); w < 91 || w > 93 {
		t.Errorf("the worst sample of the reversed run reads %.2f, want 92. Taking a "+
			"percentile without sorting reports whatever arrived last", w)
	}
	// And an empty stage is zero rather than a panic: a run where every frame failed to
	// decode reports nothing, and must not take the report down with it.
	var none stageCost
	if none.at(50) != 0 || none.at(95) != 0 || none.at(100) != 0 {
		t.Error("an unmeasured stage did not report zero")
	}
	if none.line() == "" {
		t.Error("an unmeasured stage printed nothing at all")
	}
}

// Anything that is not already RGBA becomes RGBA, because that is what a capture produces.
//
// The recorded frames decode as whatever PNG says; a live capture is always RGBA. Measuring the
// proposer on an NRGBA image and then running it on RGBA would be measuring a conversion
// nobody pays in production.
func TestAnImageBecomesTheKindACaptureProduces(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	got := toRGBA(src)
	if got.RGBAAt(0, 0).R != 10 || got.RGBAAt(0, 0).B != 30 {
		t.Errorf("converting an NRGBA pixel gave %+v", got.RGBAAt(0, 0))
	}
	// An image that is ALREADY RGBA is returned as itself, so the measurement does not
	// include a copy production never makes.
	rgba := image.NewRGBA(image.Rect(0, 0, 2, 1))
	if toRGBA(rgba) != rgba {
		t.Error("an RGBA image was copied; the proposer rows would then include a " +
			"conversion a live capture never pays")
	}
}
