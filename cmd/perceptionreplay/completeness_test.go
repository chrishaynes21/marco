package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The funnel asks about the list's OWN column, not about a height anywhere on the screen.
//
// # The measurement that forced this
//
// The annotation gives a selected row's HEIGHT and nothing else, so "something was found at
// that height" is satisfied by anything at that height anywhere. Measured that way, the
// labelled row looked grouped on 86% of frames and a member of the tracked list on 10%, and
// the whole gap was an artefact of the question: on frame 273 of the Trackmania corpus the
// labelled row at y 0.439 shares its height with a six-row stack of its own at x 0.000 width
// 0.099, while the menu being tracked is at x 0.107 width 0.348.
//
// Asked in the list's own column the same funnel reads 38% and 10%, and the missing rung is
// grouping rather than detection. [[ADR-166-a-list-shares-an-edge-and-a-partial-row-keeps-one]].
//
// The column is the LIST'S, taken from its members, so nothing here assumes where a list is.
//
// Deleting the horizontal term from coversRow must fail this.
func TestTheFunnelAsksAboutTheListsOwnColumn(t *testing.T) {
	// A list's column, and a row of it.
	const x0, x1 = 0.10, 0.45
	mine := observe.Region{X: 0.10, Y: 0.40, Width: 0.35, Height: 0.05}
	// Something else entirely, at the same height, in another column.
	elsewhere := observe.Region{X: 0.70, Y: 0.40, Width: 0.20, Height: 0.05}

	if !coversRow([]observe.Region{mine}, 0.425, x0, x1) {
		t.Error("a row of the list, at the labelled height and in the list's own " +
			"column, was not found")
	}
	if coversRow([]observe.Region{elsewhere}, 0.425, x0, x1) {
		t.Error("something at the labelled height in a different column counted as " +
			"this list's row. A height is not a place")
	}
	// AND A BAND THAT MERELY CROSSES THE LIST IS NOT ONE OF ITS ROWS. The share is of the
	// row's OWN width, so a narrow row inside a wide list belongs and a wide one does not.
	across := observe.Region{X: 0.0, Y: 0.40, Width: 1.0, Height: 0.05}
	if coversRow([]observe.Region{across}, 0.425, x0, x1) {
		t.Error("a band spanning the whole screen counted as a row of this list")
	}
	// A NARROW ROW INSIDE THE COLUMN DOES belong: a partially found row is still one.
	part := observe.Region{X: 0.30, Y: 0.40, Width: 0.12, Height: 0.05}
	if !coversRow([]observe.Region{part}, 0.425, x0, x1) {
		t.Error("a narrow row wholly inside the list's column was refused; a row the " +
			"detector found only part of is the row it is part of")
	}
}

// The replay says WHERE a selected row was lost, and the three places are counted apart.
//
// A proposal covering the row, a stack holding it, and the list owning it are three different
// claims, and a frame that cannot name the row failed at exactly one of them. Reporting a
// single "coverage" number cannot say which, and the three want opposite work: a detector, a
// grouper, or a candidate's membership.
//
// The split and the drop are counted apart for the same reason. A row in a stack holding NONE
// of the list's members means the list is one piece of a menu that grouping cut up; a row in
// the SAME stack that the list never took means membership refused it. On the Trackmania corpus
// those read 109 and 4, which is what said the repair belonged in grouping.
//
// Deleting either tally in scoreSelection must fail this.
func TestTheReplayReportsWhereTheSelectedRowWasLost(t *testing.T) {
	ann := scoreAnn() // nine rows, y 0.100 to 0.500, pitch 0.050
	const y = 0.300   // the fifth labelled row
	trace := []frameObservation{
		// Lost at grouping: a shape in the column, in no stack.
		{Index: 40, Status: "provisional", Selected: -1, Labelled: true, LabelledY: y,
			RowShaped: true, ListMembers: 6,
			ListTop: 0.35, ListBottom: 0.60},
		// Lost to a SPLIT: in a stack that holds none of the list's members.
		{Index: 41, Status: "provisional", Selected: -1, Labelled: true, LabelledY: y,
			RowShaped: true, RowGrouped: true, RowStack: 3, RowStackHolds: 0,
			ListMembers: 6, ListTop: 0.35, ListBottom: 0.60},
		// Lost to MEMBERSHIP: in the same stack as the list's members, never taken.
		{Index: 42, Status: "provisional", Selected: -1, Labelled: true, LabelledY: y,
			RowShaped: true, RowGrouped: true, RowStack: 7, RowStackHolds: 6,
			ListMembers: 6, ListTop: 0.35, ListBottom: 0.60},
		// Not lost at all, and named: the row is a member and inside the extent.
		{Index: 43, Status: "provisional", Selected: 2, Labelled: true, LabelledY: y,
			RowShaped: true, RowGrouped: true, RowStack: 9, RowStackHolds: 6,
			RowIsMember: true, ListMembers: 9,
			ListTop: 0.25, ListBottom: 0.60, NamedY: y, NamedHeight: 0.04},
		// Named while the row was NOT a member: the claim could not have been right.
		{Index: 44, Status: "provisional", Selected: 0, Labelled: true, LabelledY: y,
			RowShaped: true, RowGrouped: true, RowStack: 7, RowStackHolds: 6,
			ListMembers: 6, ListTop: 0.35, ListBottom: 0.60,
			NamedY: 0.375, NamedHeight: 0.04},
	}
	sc := scoreSelection(trace, ann)

	if sc.Shaped != 5 {
		t.Errorf("%d frame(s) had a shape at the labelled row's height in the list's "+
			"column, want 5", sc.Shaped)
	}
	if sc.Grouped != 4 {
		t.Errorf("%d frame(s) had it in a stack, want 4: one of the five had a shape "+
			"and no stack", sc.Grouped)
	}
	if sc.Member != 1 {
		t.Errorf("%d frame(s) had it as a member of the list, want 1", sc.Member)
	}
	if sc.Split != 1 {
		t.Errorf("%d frame(s) were a split — the row in a stack holding none of the "+
			"list's members — want 1", sc.Split)
	}
	if sc.Dropped != 2 {
		t.Errorf("%d frame(s) were a drop — the row in the list's own stack and not "+
			"taken — want 2", sc.Dropped)
	}
	if sc.WhileNamed != 1 || sc.WhileNotMember != 1 {
		t.Errorf("namings over a present row %d and over an absent one %d, want 1 and 1",
			sc.WhileNamed, sc.WhileNotMember)
	}
	if sc.NotMemberWrong != 1 || sc.NotMemberRight != 0 {
		t.Errorf("the naming made over an absent row scored %d wrong and %d right, "+
			"want 1 and 0: the row it named is not the row the labels claim",
			sc.NotMemberWrong, sc.NotMemberRight)
	}
	if n := sc.MembersSeen[6]; n != 4 {
		t.Errorf("the list was believed to have 6 rows on %d frame(s), want 4: %v",
			n, sc.MembersSeen)
	}
}

// The replay says what a list did not show, and whether a claim was made anyway.
//
// # Why this is reported rather than acted on
//
// A selection claim made while the list has a row this look did not show is a claim that could
// not have been right if the mark was on that row. Withholding those was built and measured:
// accuracy 96% to 98% and confidently wrong readings 13 to 7, against retained graph moves 13
// to ONE — because a move is recorded between two readings of one list and the withheld looks
// fall between them. On the same corpus 65 namings were made while a row was unseen and 6 of
// them were wrong, so the rule costs 59 right answers and 12 moves to remove 6 errors.
// [[ADR-167-a-slow-gate-and-two-rules-that-measured-worse]].
//
// So the numbers are reported and the next rule has to beat them. That is what these counters
// are for, and a report that cannot separate "the reader misread a list it was given" from "the
// list was missing the row" cannot tell whether a change helped.
//
// Deleting either tally in scoreSelection must fail this.
func TestTheReplayReportsWhatAListDidNotShow(t *testing.T) {
	ann := scoreAnn() // nine rows, y 0.100 to 0.500, pitch 0.050
	const y = 0.300   // the fifth labelled row
	trace := []frameObservation{
		// A complete list, read correctly.
		{Index: 40, Status: "provisional", Selected: 2, Labelled: true, LabelledY: y,
			RowShaped: true, RowGrouped: true, RowIsMember: true,
			ListMembers: 9, ListRows: 9, MissingRows: 0,
			ListTop: 0.05, ListBottom: 0.55, NamedY: y, NamedHeight: 0.04},
		// An incomplete list, read correctly anyway: the mark was on a row that WAS
		// shown, so the claim happened to be safe.
		{Index: 41, Status: "provisional", Selected: 2, Labelled: true, LabelledY: y,
			RowShaped: true, RowGrouped: true, RowIsMember: true,
			ListMembers: 9, ListRows: 9, MissingRows: 2,
			ListTop: 0.05, ListBottom: 0.55, NamedY: y, NamedHeight: 0.04},
		// An incomplete list, read WRONGLY: the claim names a row the labels put
		// elsewhere, and two of the list's rows were not shown.
		{Index: 42, Status: "provisional", Selected: 0, Labelled: true, LabelledY: y,
			RowShaped: true, RowGrouped: true, RowIsMember: true,
			ListMembers: 9, ListRows: 9, MissingRows: 3,
			ListTop: 0.05, ListBottom: 0.55, NamedY: 0.15, NamedHeight: 0.04},
		// And a frame with a list and no reading, which belongs in the histogram and
		// in neither naming tally.
		{Index: 43, Status: "provisional", Selected: -1, Labelled: true, LabelledY: y,
			RowShaped: true, ListMembers: 9, ListRows: 9, MissingRows: 1,
			ListTop: 0.05, ListBottom: 0.55},
	}
	sc := scoreSelection(trace, ann)

	if sc.WhileIncomplete != 2 {
		t.Errorf("%d naming(s) were made while a row of the list was unseen, want 2",
			sc.WhileIncomplete)
	}
	if sc.WhileIncompleteWrong != 1 {
		t.Errorf("%d of those were wrong, want 1. A claim that could not have been "+
			"right and one that happened to be are the same risk and different "+
			"outcomes, and the rule to come has to beat both numbers",
			sc.WhileIncompleteWrong)
	}
	want := map[int]int{0: 1, 1: 1, 2: 1, 3: 1}
	for n, c := range want {
		if sc.MissingSeen[n] != c {
			t.Errorf("%d row(s) unseen on %d frame(s), want %d: %v",
				n, sc.MissingSeen[n], c, sc.MissingSeen)
		}
	}
}
