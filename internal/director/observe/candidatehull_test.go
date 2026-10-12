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

// The reader keeps a row's history when another row goes missing.
//
// # THE CENTRAL PROPERTY of entry identity, and the measurement that forced it
//
// A reader's only evidence about a row is how it compares with its own past, and its past has
// to be filed under something. It was filed under the row's POSITION in the slice it was handed,
// and a position does not survive a frame: a detector that finds six rows and then five has
// renumbered every row below the one it lost, so the whole history had to be thrown away. The
// code said so plainly — "a changed row count means the caller is no longer describing the same
// thing" — and that was the honest answer while nothing could say which row was which.
//
// Measured on 864 real frames of one menu, the count took nine different values and changed on
// most frames. The reader said "no row stands out from its siblings or from its own unselected
// level" on 279 of the 312 frames where it was handed the right list, and named an entry on 33.
// Filing the history under the row's ENTRY instead took that to 100 named and 78 correct, with
// the reader right 78 times out of 78 when the labelled row was inside the list it was given.
// [[ADR-165-an-entry-is-the-same-entry-when-nothing-says-otherwise]].
//
// Entered through NewInterfaceWatch and the Observe door, because the thing being tested is
// that the identities reach the reader at all.
//
// Deleting the ActiveEntries loop in foldLook must fail this.
func TestTheReaderKeepsARowsHistoryWhenAnotherRowGoesMissing(t *testing.T) {
	w := observe.NewInterfaceWatch()
	base := time.Unix(1760000000, 0)
	mark, seq := 0, 0
	walking := true
	// topGone drops the FIRST row of the left-hand list, which is the case stated plainly:
	// three entries are detected, then the top one is not, and every remaining entry is at
	// a different position in what the reader is handed while being the same entry.
	//
	// The first row and not a middle one, so the rows that remain are still the regular
	// stack grouping requires — a gap in the middle is a different situation and would
	// test grouping rather than the reader.
	look := func(topGone bool) {
		seq++
		at := base.Add(time.Duration(seq) * 100 * time.Millisecond)
		if walking && seq >= 10 && seq%3 == 0 {
			mark = (mark + 1) % 5
		}
		lk := lookOfTwoLists(seq, at, mark, 3, 1)
		if topGone {
			var kept []observe.ShadowShape
			var dropped bool
			for _, sh := range lk.Shapes {
				if !dropped && sh.Region.X < 0.5 {
					dropped = true
					continue
				}
				kept = append(kept, sh)
			}
			lk.Shapes = kept
		}
		w.Observe(lk)
	}
	for range 120 {
		look(false)
	}
	// AND THE MARK PARKED on a row that is not the one about to go missing, so what is
	// being tested is a history surviving a renumbering and not a mark being hidden.
	walking = false
	mark = 2
	for range 6 {
		look(false)
	}
	b := w.Belief()
	if b.Status != observe.InterfaceProvisional || b.Selected < 0 {
		t.Fatalf("nothing was read with every row present, so dropping one proves "+
			"nothing: %s / %s", b.Status, b.Reading)
	}
	entry, rank, entries := b.SelectedEntry, b.SelectedRank, b.Entries
	if rank == 0 {
		t.Fatalf("the mark is on the top entry, which is the one about to go missing, "+
			"so this would test a hidden mark rather than a renumbering (entry %d)",
			entry)
	}
	if entry == 0 {
		t.Fatal("the reading names no entry, so there is no identity for a history to " +
			"be filed under")
	}

	// THE TOP ROW GOES MISSING. The mark is on the third entry and stays there; every row
	// the reader is handed is one position further up than it was.
	for range 6 {
		look(true)
	}
	b = w.Belief()
	if b.Entries >= entries {
		t.Fatalf("the reader was handed %d rows and then %d, so no row went missing and "+
			"this test asserts nothing", entries, b.Entries)
	}
	// AND THE LIST STILL SAYS HOW MANY ROWS IT HAS, which is not how many were found.
	if b.ListEntries != entries {
		t.Errorf("the list reports %d entries while one of its rows is undetected; it "+
			"has %d, and only the detector saw fewer", b.ListEntries, entries)
	}
	if b.SelectedEntry != entry {
		t.Errorf("the mark was on entry %d and the belief now says %d, with nothing "+
			"moving but a row elsewhere in the list going undetected. An entry is "+
			"the same entry when nothing says otherwise", entry, b.SelectedEntry)
	}
	if b.Selected < 0 {
		t.Errorf("the reading was lost when a row the mark is not on went missing (%s). "+
			"That is a history filed under a position rather than an entry: every "+
			"row below the missing one is at a different index, so nothing the "+
			"reader knows applies to it any more", b.Reading)
	}
	// AND A MOVE SEEN WHILE A ROW IS MISSING IS RECORDED AGAINST THE LIST, not against
	// what the detector found. "From entry 2 to entry 3 of a 5-entry list" is a claim a
	// person can check; the same move recorded as "of 4" describes a list that was never
	// on the screen, and two readings taken while the count flickers between 4 and 5 are
	// dropped rather than joined.
	// THE COUNT FLICKERS, because the detector finds the top row on some looks and not
	// others — which is what it does — while the LIST still has the rows it has. A rule
	// that compares the count it was handed drops the held sighting on every other look
	// and retains nothing.
	walking = true
	for i := range 36 {
		look(i%2 == 0)
	}
	b = w.Belief()
	if b.ListEntries != entries {
		t.Errorf("the list reports %d entries with one of its rows undetected; it has "+
			"%d, and the one the detector found is a different number", b.ListEntries,
			entries)
	}
	moves, _ := w.TakeSelectionMoves()
	if len(moves) == 0 {
		t.Errorf("the mark moved %d times while the detector found the top row on some "+
			"looks and not others, and not one move was retained. A count that "+
			"changes with the detector drops the held sighting on every other "+
			"look", 36/3)
	}
	for _, m := range moves {
		if m.Entries != entries {
			t.Errorf("a move was recorded in a %d-entry list; the list has %d entries "+
				"and only the detector saw fewer", m.Entries, entries)
		}
	}
	t.Logf("entry %d at rank %d survived %d rows becoming %d; %d move(s) retained, all "+
		"in a %d-entry list", entry, rank, entries, b.Entries, len(moves), entries)
}
