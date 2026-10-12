package observe_test

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A session can say which candidate qualified first and how much each one has wandered.
//
// # Why these two numbers had to reach the report
//
// Because a wrong active list cannot be diagnosed without them, and the final table actively
// misleads. A candidate's bounds are overwritten by every frame's extent, so `Where` shows
// wherever it ended up — on the real corpus the band that produced 138 of 185 selection
// readings does not appear in the candidate table at all, because by the end that candidate had
// moved on. And the tracker publishes a sole qualifier with no comparison to anything, so the
// ORDER candidates cross the bar in is the decision; after the fact the counters cannot say who
// was first.
//
// With them: on 864 real frames the bucket that won qualified at observation 15 and the real
// menu at 162, and every candidate that ever qualified had a hull spanning the whole screen.
//
// This is a WIRING test and enters through the production constructor, because the two fields
// are read from `selection.Evidence` in one place and a report that silently carried zeroes
// would read as "nothing wandered".
//
// Deleting the QualifiedAt/Hull/Rehomed assignment in InterfaceWatch.Candidates must fail this.
func TestTheReportSaysWhichCandidateQualifiedFirstAndHowFarItWandered(t *testing.T) {
	w := observe.NewInterfaceWatch()
	base := time.Unix(1760000000, 0)
	mark := 0
	for i := range 120 {
		at := base.Add(time.Duration(i) * 100 * time.Millisecond)
		navs := 0
		if i >= 10 && i%3 == 0 {
			mark = (mark + 1) % 5
			navs = 1
		}
		w.Observe(lookOfTwoLists(i+1, at, mark, 3, navs))
	}

	cands := w.Candidates()
	if len(cands) == 0 {
		t.Fatalf("no candidate was tracked: %s", w.Belief().Because)
	}
	var qualified int
	for _, c := range cands {
		if c.Hull.Height <= 0 {
			t.Errorf("a candidate reports an empty hull; every candidate has held at "+
				"least one extent, and a zero here reads as a list that never "+
				"moved: %+v", c.Where)
		}
		if c.Hull.Height+1e-9 < c.Where.Height {
			t.Errorf("a candidate's hull %v is shorter than its current extent %v; "+
				"the hull is the union of every extent it has held",
				c.Hull, c.Where)
		}
		if !c.Qualifies {
			continue
		}
		qualified++
		if c.QualifiedAt <= 0 {
			t.Errorf("a qualifying candidate reports qualifying at observation %d. "+
				"Which candidate got there FIRST is the decision whenever one "+
				"qualifies alone, and nothing else in the report says it",
				c.QualifiedAt)
		}
		if c.QualifiedAt > c.Frames {
			t.Errorf("a candidate seen on %d frame(s) reports qualifying at "+
				"observation %d", c.Frames, c.QualifiedAt)
		}
	}
	if qualified == 0 {
		t.Fatalf("no candidate qualified over 120 observations of two lists handing "+
			"their marks around, so nothing above was checked: %s",
			w.Belief().Because)
	}
}
