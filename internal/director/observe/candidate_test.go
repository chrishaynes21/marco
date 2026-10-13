package observe_test

// MERGING TWO SOURCES' PROPOSALS WITHOUT INVENTING A DOUBT OR LOSING A RECTANGLE.
//
// Two cues now propose over one frame. The pixel proposer offers everything it segments; the
// learned detector offers a handful of controls, late. Both go into the set a press is judged
// against, and the only decision this file holds is what happens when they describe the same
// rectangle — which is both the best case and, measured, the one that nearly discarded the
// capability the second source was added for.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

func rect(x, y, w, h float64) observe.Region {
	return observe.Region{X: x / 1920, Y: y / 1080, Width: w / 1920, Height: h / 1080}
}

// AGREEMENT IS NOT AMBIGUITY.
//
// `Press` calls a press ambiguous when more than one candidate contains it, and that is the
// right rule for one source. With two, the case where both describe the same control is the
// strongest evidence available — and keeping both copies would report it as a doubt, so a
// corroborated control would look less certain than an uncorroborated one.
func TestAgreementBetweenSourcesIsNotAmbiguity(t *testing.T) {
	button := rect(794, 607, 331, 38)
	pixels := observe.Proposed([]observe.Region{button}, 10, 1, 1)
	// The detector's box for the same button, a pixel out, which is what agreement
	// actually looks like.
	slow := observe.Detected([]observe.Region{rect(795, 608, 330, 37)},
		[]string{"button"}, []float64{0.45}, 10, 1, 1)

	if got := observe.Merge(pixels, slow); len(got) != 1 {
		t.Fatalf("%d candidate(s) from two accounts of one button, want 1: %+v",
			len(got), got)
	}
}

// THE MORE SPECIFIC READING SURVIVES, AND THIS IS THE CASE THAT ALMOST WENT WRONG.
//
// `sameRect` is mutual centre containment: threshold-free, and deliberately loose enough to
// tolerate a proposer that found the same row slightly larger on a later frame. It is loose
// enough to hold between a large aggregate and a real control sitting inside it — here a
// stadium-crowd slab and a menu tile that differ 2.2x in area.
//
// The first version of Merge kept the PIXEL copy of a duplicate, so that a stored reading never
// depended on inference timing. Under that rule this pair deletes the detector's tile, the slab
// wins the press, and every admission counter reports success. The capability would disappear
// without a single failure anywhere.
//
// Both orders are checked, because "whichever arrived first" is not a rule.
func TestTheMoreSpecificReadingSurvivesADuplicate(t *testing.T) {
	// Shifted from the measured geometry until the two ARE sameRect — which is the
	// near miss, a few pixels away from the real frame.
	slab := rect(0, 476, 880, 230)
	tile := rect(430, 527, 360, 226)

	pixels := observe.Proposed([]observe.Region{slab}, 10, 1, 1)
	slow := observe.Detected([]observe.Region{tile}, []string{"image"},
		[]float64{0.42}, 10, 1, 1)

	got := observe.Merge(pixels, slow)
	if len(got) != 1 {
		t.Fatalf("%d candidate(s), want 1 — the fixture is not exercising a duplicate "+
			"(are these still sameRect?): %+v", len(got), got)
	}
	if got[0].Width*got[0].Height != tile.Width*tile.Height {
		t.Errorf("the surviving candidate is the LARGER of the pair; the control " +
			"inside the aggregate is the one a press should find")
	}
	if got[0].From != directorapi.SourceVision {
		t.Errorf("the survivor claims to be a %s proposal", got[0].From)
	}
}

// A CANDIDATE NOBODY ELSE PROPOSED IS SIMPLY ADDED, with its provenance intact.
func TestAnUnmatchedDetectionJoinsTheSetWithItsProvenance(t *testing.T) {
	pixels := observe.Proposed([]observe.Region{rect(0, 0, 100, 20)}, 7, 3, 9)
	slow := observe.Detected([]observe.Region{rect(900, 900, 100, 20)},
		[]string{"image"}, []float64{0.42}, 4, 3, 9)

	got := observe.Merge(pixels, slow)
	if len(got) != 2 {
		t.Fatalf("%d candidate(s), want 2", len(got))
	}
	var seen int
	for _, c := range got {
		switch c.From {
		case observe.SourceStructure:
			seen++
			if c.FromLook != 7 || c.Class != "" {
				t.Errorf("a pixel proposal carries look %d class %q",
					c.FromLook, c.Class)
			}
		case directorapi.SourceVision:
			seen++
			// THE LOOK IT WAS COMPUTED FROM, not the one it arrived on. The
			// distinction is the whole reason the field exists.
			if c.FromLook != 4 || c.Class != "image" || c.Score != 0.42 {
				t.Errorf("the detection lost its provenance: %+v", c)
			}
		}
	}
	if seen != 2 {
		t.Errorf("%d candidate(s) had a recognised source, want 2", seen)
	}
}

// SMALLEST FIRST, WHATEVER ORDER THE SOURCES ARRIVED IN.
//
// One recording must replay to the same answer twice, and a press's candidate list is ordered
// by area before it is truncated — so an order that depended on arrival would silently change
// which candidates survived the bound.
func TestTheMergedSetIsOrderedByAreaAndNotByArrival(t *testing.T) {
	// Far apart, so nothing here is a duplicate and ordering is the only thing
	// under test.
	big := rect(0, 0, 800, 600)
	mid := rect(1000, 0, 400, 300)
	small := rect(20, 700, 100, 50)

	a := observe.Merge(observe.Proposed([]observe.Region{big}, 1, 1, 1),
		observe.Detected([]observe.Region{small, mid}, nil, nil, 1, 1, 1))
	b := observe.Merge(observe.Proposed([]observe.Region{big}, 1, 1, 1),
		observe.Detected([]observe.Region{mid, small}, nil, nil, 1, 1, 1))

	for _, got := range [][]observe.Candidate{a, b} {
		if len(got) != 3 {
			t.Fatalf("%d candidate(s), want 3", len(got))
		}
		for i := 1; i < len(got); i++ {
			p, q := got[i-1], got[i]
			if p.Width*p.Height > q.Width*q.Height {
				t.Fatalf("candidate %d is larger than %d", i-1, i)
			}
		}
	}
}

// BOTH HALVES OF THE CONTEXT, tested on the rule itself.
//
// `Admissible` is the one statement of when a delayed result is still about now, and it is
// applied by two callers. Reaching it only through the observer hid half of it: a window change
// also moves the epoch there, so the window clause could be deleted with every observer test
// still green. A rule with two clauses is tested as a rule.
func TestAdmissibleNeedsBothTheWindowAndTheObservation(t *testing.T) {
	c := observe.Detected([]observe.Region{rect(0, 0, 10, 10)}, nil, nil, 1, 3, 9)[0]
	if !observe.Admissible(c, 3, 9) {
		t.Fatal("a candidate in its own context was refused")
	}
	if observe.Admissible(c, 4, 9) {
		t.Error("admitted on a window generation it was not computed under")
	}
	if observe.Admissible(c, 3, 10) {
		t.Error("admitted in an observation it was not computed in")
	}
	if observe.Admissible(c, 4, 10) {
		t.Error("admitted with neither half of its context intact")
	}
}

// Regions is how the purely geometric parts of this package stay that way.
func TestStrippingProvenanceLeavesTheRectangles(t *testing.T) {
	in := []observe.Region{rect(1, 2, 3, 4), rect(5, 6, 7, 8)}
	got := observe.Regions(observe.Proposed(in, 1, 1, 1))
	if len(got) != len(in) {
		t.Fatalf("%d region(s), want %d", len(got), len(in))
	}
	for i := range in {
		if got[i] != in[i] {
			t.Errorf("region %d came back as %+v, want %+v", i, got[i], in[i])
		}
	}
	if observe.Regions(nil) != nil {
		t.Error("stripping nothing produced something")
	}
}
