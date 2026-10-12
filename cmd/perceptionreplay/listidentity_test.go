package main

import "testing"

// The list score, on traces small enough to read.
//
// The real corpus holds the floors and cannot hold these: every number there came out of 864
// frames of somebody's game, so a predicate that stops firing reads as an improvement. The
// cases each counter must FIRE on have to be written down.

// A full-screen candidate is not the labelled list, though it contains every labelled row.
//
// THE REASON THE SCORE IS AN IDENTITY AND NOT AN OVERLAP. "The labelled row is inside it" is
// satisfied by a rectangle covering the whole screen, and a measured candidate on this corpus
// does exactly that — y 0.059..1.000 with 21 "entries" in it. A route planned in that list
// names nothing a person could act on.
//
// Deleting the span test in scoreLists must fail this.
func TestAFullScreenCandidateIsNotTheLabelledList(t *testing.T) {
	ann := scoreAnn() // nine rows, 0.100 to 0.500, pitch 0.050
	sb := scoreLists([]frameObservation{{
		Index: 0, Reading: "one list hands a mark around: rows 21",
		Selected: 0, ListTop: 0.059, ListBottom: 1.000, Entries: 21,
	}}, ann)
	if sb.Labelled != 1 {
		t.Fatalf("%d labelled frame(s) scored, want 1", sb.Labelled)
	}
	if sb.Wrong != 1 {
		t.Errorf("a candidate spanning y 0.059..1.000 was scored as the labelled list "+
			"(correct %d, wrong %d, none %d). It contains all nine labelled rows "+
			"and is not a menu", sb.Correct, sb.Wrong, sb.None)
	}
}

// A list missing rows at one end is still the labelled list.
//
// The other side of the predicate, and it has to hold or the score measures a different defect.
// The menu on this corpus groups as six of its nine entries — a separately recorded shortfall
// in `stack` — and six rows of the menu IS the menu: the reader reads them correctly and a route
// planned in them names real entries. Demanding all nine would score that shortfall as a
// list-identification failure and hide the one being measured.
func TestAListMissingRowsAtOneEndIsStillTheLabelledList(t *testing.T) {
	ann := scoreAnn()
	// Rows 0.100..0.500. An envelope over the last six, 0.225..0.525.
	sb := scoreLists([]frameObservation{{
		Index: 0, Reading: "one of 4 qualifying lists responds to navigation",
		Selected: 0, ListTop: 0.225, ListBottom: 0.525, Entries: 6,
	}}, ann)
	if sb.Correct != 1 {
		t.Errorf("an envelope over six of the nine labelled rows was not scored as the "+
			"labelled list (correct %d, wrong %d, none %d)",
			sb.Correct, sb.Wrong, sb.None)
	}
}

// A sliver nowhere near the labelled list is wrong, and is attributed to the arm that published it.
//
// The table that locates the failure: each arm of the tracker's decision rests on different
// evidence, so an arm that is usually wrong names the rule to look at. Reading the arm off the
// belief's own prose is what makes that possible without a second channel that can disagree
// with it.
//
// Deleting a case from branchOf must fail this.
func TestAWrongListIsAttributedToTheArmThatPublishedIt(t *testing.T) {
	ann := scoreAnn()
	sb := scoreLists([]frameObservation{
		// The sliver, published by the arm that compares nothing.
		{Index: 0, Reading: "one list hands a mark around: rows 9",
			Selected: 0, ListTop: 0.700, ListBottom: 0.755, Entries: 9},
		// The menu, published by the arm that consults input evidence.
		{Index: 10, Reading: "one of 4 qualifying lists responds to navigation: 15 of 18",
			Selected: 0, ListTop: 0.075, ListBottom: 0.525, Entries: 9},
		// And an abstention.
		{Index: 20, Reading: "4 lists each hand a mark between their members, and what " +
			"the person pressed does not single one of them out",
			Selected: -1},
	}, ann)

	sole := sb.ByBranch[branchSole]
	resp := sb.ByBranch[branchResponsive]
	amb := sb.ByBranch[branchAmbiguous]
	if sole == nil || resp == nil || amb == nil {
		t.Fatalf("three beliefs with three different reasons were attributed to %d "+
			"arm(s); the prose is what tells them apart", len(sb.ByBranch))
	}
	if sole.Wrong != 1 || sole.Correct != 0 {
		t.Errorf("the sole-qualifier arm scored correct %d wrong %d on one sliver",
			sole.Correct, sole.Wrong)
	}
	if resp.Correct != 1 || resp.Wrong != 0 {
		t.Errorf("the responsive arm scored correct %d wrong %d on one menu",
			resp.Correct, resp.Wrong)
	}
	if amb.None != 1 {
		t.Errorf("the ambiguous arm scored %d frame(s) with no list, want 1", amb.None)
	}
	if got := sb.Correct + sb.Wrong + sb.None; got != sb.Labelled {
		t.Errorf("three labelled frames split into %d", got)
	}
}

// The timeline says when one candidate's extent drifted inside a single stretch.
//
// `moved` is the tell for temporal-identity churn, and it cannot be read off the final
// candidate table at all: a candidate's bounds are overwritten by every frame's extent, so by
// the end they are wherever they ended up. A stretch whose extent moves is one tracked "list"
// being several rectangles while the belief was published about it.
//
// Deleting the Moved assignment in scoreLists must fail this.
func TestTheTimelineSaysWhenAnExtentDriftedWithinOneStretch(t *testing.T) {
	ann := scoreAnn()
	const why = "one list hands a mark around: rows 5"
	sb := scoreLists([]frameObservation{
		{Index: 0, Reading: why, Selected: 0,
			ListTop: 0.700, ListBottom: 0.900, Entries: 5},
		// Still overlapping enough to be the same stretch, and not the same extent.
		{Index: 10, Reading: why, Selected: 0,
			ListTop: 0.720, ListBottom: 0.920, Entries: 5},
	}, ann)
	if len(sb.Runs) != 1 {
		t.Fatalf("%d stretch(es) from two overlapping extents under one arm, want 1: "+
			"%+v", len(sb.Runs), sb.Runs)
	}
	if !sb.Runs[0].Moved {
		t.Error("a stretch whose extent moved from 0.700..0.900 to 0.720..0.920 is " +
			"not marked as moved, so one candidate drifting and the belief " +
			"changing lists look alike")
	}

	// AND A JUMP ENDS THE STRETCH rather than being recorded as drift. The two are
	// different findings: one candidate's bounds moving, against the belief being
	// published about a different list.
	jump := scoreLists([]frameObservation{
		{Index: 0, Reading: why, Selected: 0,
			ListTop: 0.700, ListBottom: 0.900, Entries: 5},
		{Index: 10, Reading: why, Selected: 0,
			ListTop: 0.100, ListBottom: 0.300, Entries: 5},
	}, ann)
	if len(jump.Runs) != 2 {
		t.Errorf("%d stretch(es) from two extents that do not overlap at all, want 2",
			len(jump.Runs))
	}
}
