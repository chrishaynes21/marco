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

// The report says how many lists have been retired, and the belief names the one it is about.
//
// Both are wiring, and both are things no caller can work out for itself. A population being
// pruned and one that was never crowded look identical from a count of the survivors; and an
// extent is not an identity, so "is this still the list I was looking at" cannot be answered
// from `Where`. [[ADR-164-a-list-is-the-same-list-when-its-rows-are]].
//
// Entered through NewInterfaceWatch and the Observe door a session uses, because a report built
// by reaching past the production constructor has been built eight times in this repository and
// never once invoked.
//
// Deleting either assignment must fail this.
func TestTheReportSaysWhatWasRetiredAndWhichListTheBeliefIsAbout(t *testing.T) {
	w := observe.NewInterfaceWatch()
	base := time.Unix(1760000000, 0)
	mark, seq := 0, 0
	look := func(both bool) {
		seq++
		at := base.Add(time.Duration(seq) * 100 * time.Millisecond)
		if seq >= 10 && seq%3 == 0 {
			mark = (mark + 1) % 5
		}
		lk := lookOfTwoLists(seq, at, mark, 3, 1)
		if !both {
			// THE SECOND LIST GONE. It has its own left edge, over past the
			// middle of the screen, so dropping it is a filter on position
			// rather than a count.
			var kept []observe.ShadowShape
			for _, sh := range lk.Shapes {
				if sh.Region.X < 0.5 {
					kept = append(kept, sh)
				}
			}
			lk.Shapes = kept
		}
		w.Observe(lk)
	}
	for range 120 {
		look(true)
	}
	if w.RetiredCandidates() != 0 {
		t.Fatalf("%d candidate(s) retired while every list was still on the screen",
			w.RetiredCandidates())
	}
	b := w.Belief()
	if b.Status != observe.InterfaceProvisional {
		t.Fatalf("no list was identified, so the belief names nothing: %s", b.Because)
	}
	if b.ListID == 0 {
		t.Error("the belief names no list. Whether this is still the list a caller was " +
			"looking at is the tracker's answer to give, and a caller that has to " +
			"infer it from the published extent gets 'no' every time a row is " +
			"found a pixel lower")
	}
	id := b.ListID
	var tracked bool
	for _, c := range w.Candidates() {
		if c.ID == id {
			tracked = true
		}
	}
	if !tracked {
		t.Errorf("the belief is about list %d and no candidate in the report has that "+
			"name, so the two cannot be read together", id)
	}

	// THE SECOND LIST GOES AWAY. lookOfTwoLists drops it when asked for fewer rows than
	// it takes to be a stack, so nothing finds it again and it ages out.
	for range 40 {
		look(false)
	}
	if w.RetiredCandidates() == 0 {
		t.Errorf("nothing was retired after 40 observations in which one of the two "+
			"lists was never found again. A tracked list that is gone keeps its "+
			"counters, publishes its extent, and is the population this rule had "+
			"to bound: %s", w.Belief().Because)
	}
	t.Logf("belief about list %d, %d candidate(s) now tracked, %d retired",
		id, len(w.Candidates()), w.RetiredCandidates())
}
