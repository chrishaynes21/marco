package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Can a session answer, from its own current state, what it is looking at, what is selected,
// and what changed since the last observation?
//
// Driven through ShadowTotals.Observe, which is the fold call site the shadow evidence
// actually goes through — not a helper written for the test. Nothing here constructs the
// belief directly.

// menuSample builds one look at a list of n rows with one of them marked, as a detector would
// have reported it: window-relative normalised geometry and two shading numbers per region.
func menuSample(seq int, n, marked int) observe.Sample {
	var regions []observe.ShadowRegion
	for i := range n {
		shade := []float64{60, 72}
		if i == marked {
			shade = []float64{205, 238}
		}
		regions = append(regions, observe.ShadowRegion{
			Role: "button",
			Region: observe.Region{
				X: 0.10, Y: 0.40 + float64(i)*0.06, Width: 0.40, Height: 0.05,
			},
			Shade: shade,
		})
	}
	// Something else on screen that is not a list being chosen from: a fixed block.
	regions = append(regions, observe.ShadowRegion{
		Role:   "image",
		Region: observe.Region{X: 0.60, Y: 0.10, Width: 0.30, Height: 0.25},
		Shade:  []float64{90, 110},
	})
	return observe.Sample{
		Sequence:         seq,
		WindowGeneration: 1,
		Shadow:           &observe.ShadowSample{Detector: "test", Ran: true, Regions: regions},
	}
}

func TestASessionAnswersWhatItIsLookingAt(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	look := func(marked int) {
		seq++
		totals.Observe(menuSample(seq, 6, marked))
	}

	// Nothing selected at first, then a mark that moves — which is the only evidence
	// that distinguishes a list being used from a list that is merely present.
	for range 6 {
		look(-1)
	}
	for _, m := range []int{0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3} {
		look(m)
	}

	b := totals.Interface
	if b.Status != observe.InterfaceProvisional {
		t.Fatalf("what interface am I looking at? got %q, because: %s", b.Status, b.Because)
	}
	if b.Entries < 3 {
		t.Errorf("the belief should name the entries it found, got %d", b.Entries)
	}
	// The field that stops a reader counting. Grouping found seven of nine on the real
	// menu, and a belief reporting "a menu of seven" would be wrong undetectably.
	if b.Complete {
		t.Error("the entries are never known to be all of them")
	}
	if b.Selected < 0 {
		t.Errorf("what is selected? got no reading (%s)", b.Because)
	}
	if b.Because == "" {
		t.Error("a belief must be able to say what it rests on")
	}
	if b.Observations == 0 || b.After == 0 {
		t.Errorf("the belief should say how much looking it took: %d looks, identified after %d",
			b.Observations, b.After)
	}
}

func TestASessionAnswersWhatChangedSinceTheLastLook(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	look := func(marked int) {
		seq++
		totals.Observe(menuSample(seq, 6, marked))
	}
	for range 6 {
		look(-1)
	}
	for _, m := range []int{0, 0, 0, 1, 1, 1, 2, 2, 2} {
		look(m)
	}
	settled := totals.Interface.Selected

	// A look that changes nothing must say so, rather than leaving a caller to diff.
	look(2)
	if totals.Changed.SelectionMoved {
		t.Errorf("nothing moved, but the session reported a move: %s", totals.Changed.Why)
	}
	if totals.Changed.Why == "" {
		t.Error("a look that changed nothing must still say so")
	}

	// And a look that moves the selection must name both ends of the move.
	look(3)
	look(3)
	if !totals.Changed.SelectionMoved && totals.Interface.Selected == settled {
		t.Fatalf("the selection moved and the session did not notice: %s", totals.Changed.Why)
	}
	if totals.Changed.SelectionMoved {
		if totals.Changed.To == totals.Changed.From {
			t.Errorf("a move must have two ends, got %d to %d",
				totals.Changed.From, totals.Changed.To)
		}
		if totals.Changed.Why == "" {
			t.Error("a move must say so in words")
		}
	}
}

func TestASessionWithNoShadingBelievesNothing(t *testing.T) {
	// Regions a detector proposed but nobody measured. A zero shade would read as the
	// darkest thing on screen, so they are dropped and the session says it does not know.
	var totals observe.ShadowTotals
	for seq := 1; seq <= 12; seq++ {
		s := menuSample(seq, 6, 2)
		for i := range s.Shadow.Regions {
			s.Shadow.Regions[i].Shade = nil
		}
		totals.Observe(s)
	}
	if got := totals.Interface.Status; got != observe.InterfaceUnknown {
		t.Fatalf("unmeasured regions support no belief, got %q", got)
	}
	if totals.Interface.Selected >= 0 {
		t.Error("nothing can be selected in a list nobody could see")
	}
}

func TestASessionThatSawNoListSaysSoRatherThanGuessing(t *testing.T) {
	// A screen with regions on it that nobody is navigating. The honest answer is that
	// the active interface is not known — not the nearest-looking list.
	var totals observe.ShadowTotals
	for seq := 1; seq <= 20; seq++ {
		totals.Observe(menuSample(seq, 6, -1))
	}
	if got := totals.Interface.Status; got != observe.InterfaceUnknown {
		t.Fatalf("a list nobody navigates is not the active interface, got %q (%s)",
			got, totals.Interface.Because)
	}
	if totals.Interface.Because == "" {
		t.Error("an abstention must say why")
	}
}
