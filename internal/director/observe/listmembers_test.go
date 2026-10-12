package observe_test

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A session says which rows the active list believes it has, and what grouping made of a look.
//
// # Why these two exist
//
// Because every selection claim about a row the list does not contain is wrong before the
// reader is consulted, and nothing else in the report says which rows those are.
// `InterfaceBelief.Where` is their extent and `Entries` is how many rows the DETECTOR found
// inside it this frame; neither is the member set.
//
// Measured with them, over the 312 frames of the Trackmania corpus that published a list: the
// row the person was looking at was detected IN THE LIST'S OWN COLUMN on every one of them, was
// in a stack grouping made on 197, and was a member of the list on 85. Of the 112 lost in
// between, 109 were in a stack holding none of the list's members — the menu cut in half — and
// 4 were in its own stack. That is what said the repair belonged in grouping rather than in
// membership, and neither number is visible without these.
// [[ADR-166-a-list-shares-an-edge-and-a-partial-row-keeps-one]].
//
// Entered through NewInterfaceWatch and the Observe door, because a report assembled by
// reaching past the production constructor has been built eight times here and never invoked.
//
// Deleting either accessor's body, or the sort in Tracker.ActiveMembers, must fail this.
func TestASessionSaysWhichRowsTheListHasAndWhatGroupingMade(t *testing.T) {
	w := observe.NewInterfaceWatch()
	base := time.Unix(1760000000, 0)
	mark, seq := 0, 0
	var last observe.InterfaceLook
	for range 120 {
		seq++
		if seq >= 10 && seq%3 == 0 {
			mark = (mark + 1) % 5
		}
		last = lookOfTwoLists(seq, base.Add(time.Duration(seq)*100*time.Millisecond),
			mark, 3, 1)
		w.Observe(last)
	}
	if b := w.Belief(); b.Status != observe.InterfaceProvisional {
		t.Fatalf("no list was identified, so it has no members to report: %s", b.Because)
	}

	members := w.ActiveMembers()
	if len(members) == 0 {
		t.Fatal("the active list reports no rows at all. Every selection claim is about " +
			"one of them, so a report that cannot name them cannot say whether a " +
			"claim had a chance")
	}
	// IN ORDER DOWN THE SCREEN, because a rank is a place and the members are stored in
	// the order they were first seen.
	for i := 1; i < len(members); i++ {
		if members[i].Y < members[i-1].Y {
			t.Errorf("member %d is at y %.3f, above member %d at y %.3f; the rows are "+
				"meant to come in the order they appear", i, members[i].Y, i-1,
				members[i-1].Y)
		}
	}
	// AND THEY ARE ONE COLUMN. lookOfTwoLists draws two lists, one on each side; the
	// active one is a list and not a sample of the screen.
	x0, x1 := members[0].X, members[0].X+members[0].Width
	for _, m := range members {
		if m.X < x0 {
			x0 = m.X
		}
		if m.X+m.Width > x1 {
			x1 = m.X + m.Width
		}
	}
	if x1-x0 > 0.6 {
		t.Errorf("the list's rows span x %.3f..%.3f, which is most of the screen; the "+
			"two lists in this fixture are in different halves of it", x0, x1)
	}

	// AND GROUPING'S STACKS ARE REPORTED AS STACKS, which is what says whether a row the
	// list is missing is in a stack of its own.
	stacks := observe.GroupedStacksForLook(last)
	if len(stacks) < 2 {
		t.Fatalf("%d stack(s) reported for a look holding two lists; the two have their "+
			"own pitches and row heights and are meant to group apart", len(stacks))
	}
	var big int
	for _, st := range stacks {
		if len(st) > big {
			big = len(st)
		}
		if len(st) == 0 {
			t.Error("a stack with no rows was reported")
		}
	}
	if big < 3 {
		t.Errorf("the largest stack has %d rows; neither list grouped", big)
	}
	// AND A SESSION'S OWN TOTALS CARRY THEM, which is the door the replay comes in by.
	// An accessor nothing reports through is an accessor nothing uses.
	var totals observe.ShadowTotals
	for i := range 120 {
		m := 0
		if i >= 10 {
			m = (i / 3) % 5
		}
		totals.Observe(twoListSample(i+1,
			base.Add(time.Duration(i+1)*100*time.Millisecond),
			navSample{markA: m, markB: 3}))
	}
	if totals.Interface.Status != observe.InterfaceProvisional {
		t.Fatalf("the session identified no list: %s", totals.Interface.Because)
	}
	if len(totals.ActiveMembers) == 0 {
		t.Error("the session's totals report no rows for the active list. The replay " +
			"reads them from here, and a funnel that cannot say which rows the " +
			"list has cannot say whether a selection claim had a chance")
	}
	if totals.ActiveRows != len(totals.ActiveMembers) {
		t.Errorf("the totals say the list has %d rows and report %d of them",
			totals.ActiveRows, len(totals.ActiveMembers))
	}
	// AND THE CAUSES OF A LOST SIGHTING COME THROUGH THE SAME DOOR. Nothing is marked on
	// the looks below, so the reader holds its claim and then gives it up, and the gap
	// between sightings runs out. Each of those is a move that could not be recorded.
	for i := range 24 {
		totals.Observe(twoListSample(200+i,
			base.Add(time.Duration(200+i)*100*time.Millisecond),
			navSample{markA: -1, markB: 3}))
	}
	if len(totals.HeldSightingsDropped) == 0 {
		t.Error("the session's totals report no cause for a lost sighting after " +
			"twenty-four looks that read nothing. The replay reads them from here")
	}

	t.Logf("%d member(s) in x %.3f..%.3f; %d stack(s), largest %d; %d in the totals",
		len(members), x0, x1, len(stacks), big, len(totals.ActiveMembers))
}
