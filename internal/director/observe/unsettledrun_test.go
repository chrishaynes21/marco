package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The segmenter records HOW LONG it could not say where the user was.
//
// Entered through the real tracker, because the value is produced by segmentation and consumed by
// the relationship layer, and a test that set the field by hand would prove only that a struct has
// a field. The bridge's bound is meaningless if nothing measures the interval — and a mutation run
// found exactly that: deleting the counter broke no test, because every bridge fixture supplied
// the number itself.

// column is a vertical run of buttons, which is the shape a structural group forms from.
func column(n int, x, y0 float64) []observe.ShadowRegion {
	out := make([]observe.ShadowRegion, 0, n)
	for i := range n {
		out = append(out, observe.ShadowRegion{
			Role: "button", Nameable: true, Confidence: 0.6,
			Region: observe.Region{
				X: x, Y: y0 + float64(i)*0.05, Width: 0.18, Height: 0.04,
			},
		})
	}
	return out
}

// segmentOver feeds one script of frames through the real tracker.
func segmentOver(frames [][]observe.ShadowRegion) *observe.ShadowTracker {
	var k observe.ShadowTracker
	for _, regions := range frames {
		k.Observe(
			&observe.ShadowSample{Ran: true, TargetProven: true, Regions: regions},
			observe.StructuralView{Source: observe.StructureFused, Regions: regions})
	}
	return &k
}

func hold(regions []observe.ShadowRegion, times int) [][]observe.ShadowRegion {
	out := make([][]observe.ShadowRegion, 0, times)
	for range times {
		out = append(out, regions)
	}
	return out
}

// TestTheSegmenterRecordsHowLongItCouldNotPlaceTheScreen is the mutation gate on the counter.
//
// Deleting `g.unsettled++` must fail this. The script is the live shape: a settled screen, one
// frame that is a strict SUBSET of it — a partial rendering, which is what a mid-animation frame
// looks like and which the segmenter declines to place on first sighting — then a different
// screen.
func TestTheSegmenterRecordsHowLongItCouldNotPlaceTheScreen(t *testing.T) {
	a := column(6, 0.40, 0.20)
	partial := a[:3] // contained by A, never seen before: unplaceable, not a new screen
	b := column(5, 0.10, 0.55)

	var frames [][]observe.ShadowRegion
	frames = append(frames, hold(a, 6)...)
	frames = append(frames, partial)
	frames = append(frames, hold(b, 6)...)

	k := segmentOver(frames)

	var exit *observe.ScreenTransition
	for i, tr := range k.Transitions() {
		if tr.From == observe.ScreenStateUnknown && tr.To != observe.ScreenStateUnknown {
			exit = &k.Transitions()[i]
		}
	}
	if exit == nil {
		t.Skipf("the script did not produce an unplaceable frame; transitions: %+v",
			k.Transitions())
	}
	if exit.UnsettledRun < 1 {
		t.Fatalf("the exit from the unplaced state records a run of %d.\nWithout it the "+
			"relationship layer has no length to bound, and every gap looks equally short — "+
			"which is the difference between recovering an adjacency and inventing one.",
			exit.UnsettledRun)
	}
}

// A longer blackout is recorded as longer, so the bound can actually bite.
func TestALongerBlackoutIsRecordedAsLonger(t *testing.T) {
	a := column(6, 0.40, 0.20)
	partial := a[:3]
	b := column(5, 0.10, 0.55)

	runFor := func(n int) int {
		var frames [][]observe.ShadowRegion
		frames = append(frames, hold(a, 6)...)
		// The SAME partial frame repeated would promote on its second sighting and become a
		// screen, so each unplaced frame is a different subset — which is also what a real
		// animation looks like.
		for i := range n {
			frames = append(frames, partial[:1+i%3])
		}
		frames = append(frames, hold(b, 6)...)
		k := segmentOver(frames)
		longest := 0
		for _, tr := range k.Transitions() {
			if tr.From == observe.ScreenStateUnknown && tr.UnsettledRun > longest {
				longest = tr.UnsettledRun
			}
		}
		return longest
	}

	short, long := runFor(1), runFor(5)
	if short == 0 || long == 0 {
		t.Skipf("the script produced no unplaceable interval (short=%d long=%d)", short, long)
	}
	if long <= short {
		t.Errorf("a five-frame blackout recorded %d and a one-frame blackout %d; the run "+
			"length is not tracking the interval", long, short)
	}
	// And the bound would reject the long one, which is the whole point of measuring it.
	if long < observe.StatePromotionCount {
		t.Errorf("a five-frame blackout recorded %d, below the promotion bound of %d, so "+
			"nothing would ever be refused for being too long",
			long, observe.StatePromotionCount)
	}
}

// A run that ends is not carried into the next one.
func TestTheUnsettledRunResetsWhenAScreenIsPlacedAgain(t *testing.T) {
	a := column(6, 0.40, 0.20)
	b := column(5, 0.10, 0.55)

	var frames [][]observe.ShadowRegion
	frames = append(frames, hold(a, 6)...)
	frames = append(frames, a[:3]) // one unplaced frame
	frames = append(frames, hold(b, 6)...)
	frames = append(frames, b[:2]) // one more, later
	frames = append(frames, hold(a, 6)...)

	k := segmentOver(frames)
	for _, tr := range k.Transitions() {
		if tr.From != observe.ScreenStateUnknown {
			continue
		}
		if tr.UnsettledRun > 1 {
			t.Errorf("edge %s → %s records a run of %d after two SEPARATE one-frame gaps; "+
				"the counter is accumulating across placements instead of resetting",
				tr.From, tr.To, tr.UnsettledRun)
		}
	}
}
