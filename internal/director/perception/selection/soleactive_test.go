package selection_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// A list that qualifies alone is not therefore the list being used.
//
// # The measurement that forced this rule
//
// [[ADR-158-the-active-list-is-the-one-that-responds]] compared responsiveness only when
// SEVERAL candidates qualified. With one, the tracker published it and asked nothing. Scored
// over 864 real frames against the labelled list's own extent — not against whether some
// candidate overlapped the selected row, which a full-screen rectangle satisfies:
//
//	published by the sole-qualifier arm    144 frames, 0 the right list, 144 wrong
//	published by the responsive arm         48 frames, 45 the right list,   3 wrong
//
// 139 of the session's 185 selection readings came from the first arm and **not one named the
// labelled row**. The second read 43 of its 46 correctly. So the defect was never in the
// selection reader, which is 93% accurate when handed a list containing the row; it was that
// the first candidate to behave like a list wins, and on a real screen that is a stack of
// unrelated rectangles that happens to flicker.
//
// These tests pin the repair and, as hard, the two things it must not break: a session with no
// input source must still name a list, and input must still never manufacture a visual outcome.

// feedOneList feeds a tracker a single five-row list, with its own mark and input per step.
//
// One list and not two, unlike feedTwoLists: the whole question here is what happens when only
// ONE candidate qualifies, so a second qualifying list would route the decision to the arm this
// file is not about.
func feedOneList(t *testing.T, steps []step, navigated bool) *selection.Tracker {
	t.Helper()
	tr := selection.NewTracker()
	for _, s := range steps {
		rows := listRows(0, 5, 100, 100)
		sh := shadesWithMark(5, s.markA)
		if navigated {
			tr.ObserveNavigated(rows, sh, s.navRecent)
		} else {
			// THE NO-INPUT DOOR, which is the one `cmd/director selection-read`
			// comes in by. Not the same as ObserveNavigated(.., false): that is a
			// caller saying "nothing was pressed just now", and this is a caller
			// that cannot see input at all.
			tr.ObserveShades(rows, sh)
		}
	}
	return tr
}

// oneWalk moves a single list's mark, with navigation observed on the moving steps or never.
func oneWalk(n int, navigated bool) []step {
	var out []step
	mark := 0
	for i := range n {
		// Baselines first: a member's level is learned only from frames in which it was
		// NOT marked, so nothing can be judged until every row has been quiet a few
		// times.
		nav := false
		if i >= 8 && i%3 == 0 {
			mark = (mark + 1) % 5
			nav = navigated
		}
		out = append(out, step{markA: mark, markB: -1, navRecent: nav})
	}
	return out
}

// A sole qualifying list is not the active one until navigation has reached it.
//
// The mark moves on its own schedule and the person navigates at other moments, so the list
// qualifies — exclusive marks, transfers, several holders — with nothing attributable. Before
// this rule it was published anyway, which is the 144-of-144 failure above.
//
// Deleting the `t.navSeen && c.prompted < MinTransfers` guard in Tracker.active must fail this.
func TestASoleQualifyingListIsNotActiveUntilNavigationReachesIt(t *testing.T) {
	// The mark moves every third observation; navigation is observed on the others, so
	// nothing the list does is ever attributable.
	var steps []step
	mark := 0
	for i := range 60 {
		nav := false
		if i >= 8 && i%3 == 0 {
			mark = (mark + 1) % 5
		} else if i >= 8 {
			nav = true
		}
		steps = append(steps, step{markA: mark, markB: -1, navRecent: nav})
	}
	tr := feedOneList(t, steps, true)

	var qualifying int
	for _, e := range tr.Evidence() {
		if e.Qualifies {
			qualifying++
			if e.Prompted >= 2 {
				t.Fatalf("the list has %d attributed transfer(s), so the fixture "+
					"does not reach the case it is for: %s", e.Prompted, e)
			}
		}
	}
	if qualifying != 1 {
		t.Fatalf("%d candidate(s) qualify, want exactly 1; with any other number the "+
			"decision goes to a different arm and this test asserts nothing",
			qualifying)
	}
	_, why, ok := tr.Active()
	if ok {
		t.Errorf("a list nothing was ever pressed at was named the active one: %s\n"+
			"Over 864 real frames this arm named the wrong list on 144 of 144 "+
			"frames and produced 139 readings, none of them right", why)
	}
	if !contains(why, "navigation has never reached it") {
		t.Errorf("the abstention says %q, which does not name what is missing. Five "+
			"different things produce an unidentified list and they call for "+
			"different responses", why)
	}
}

// A sole qualifying list that DOES respond is named.
//
// The other side of the guard, and the reason it is a requirement rather than a veto: a list
// somebody is demonstrably navigating is the list being used, and one qualifying candidate that
// responds needs no comparison because there is nothing to compare it with.
func TestASoleQualifyingListThatRespondsIsNamed(t *testing.T) {
	tr := feedOneList(t, oneWalk(60, true), true)
	bounds, why, ok := tr.Active()
	if !ok {
		t.Fatalf("a list whose mark moved while the person navigated was not named: %s",
			why)
	}
	if bounds.Empty() {
		t.Error("the active list has an empty extent")
	}
}

// With no input source at all, a sole qualifying list is still named.
//
// THE REGRESSION THIS MUST NOT CAUSE. `cmd/director selection-read` folds frames through
// `Tracker.Observe`, which cannot see input, and a caller that cannot see input must not have
// its silence read as "the person pressed nothing here". The rule is conditional on navigation
// having been observed at all, so where none ever was this arm behaves exactly as it did.
//
// Replacing the `t.navSeen &&` condition with an unconditional one must fail this.
func TestWithNoInputSourceASoleQualifyingListIsStillNamed(t *testing.T) {
	tr := feedOneList(t, oneWalk(60, false), false)
	var qualifying int
	for _, e := range tr.Evidence() {
		if e.Qualifies {
			qualifying++
		}
	}
	if qualifying != 1 {
		t.Fatalf("%d candidate(s) qualify, want exactly 1", qualifying)
	}
	if _, why, ok := tr.Active(); !ok {
		t.Errorf("a session with no input source could not name its only qualifying "+
			"list: %s\nA caller that cannot see input has not told us the person "+
			"pressed nothing", why)
	}
}

// Navigation still cannot manufacture an active list out of a list that never moves.
//
// The invariant, re-asserted on the arm that now reads input evidence. `Prompted` is an
// increment of a transfer the shading already proved, so a list whose mark never moves has
// nothing for an input to be attributed to however much is pressed — and the new requirement
// makes input NECESSARY without making it SUFFICIENT.
func TestTheSoleQualifierRuleStillRefusesAListThatNeverMoves(t *testing.T) {
	var steps []step
	for range 60 {
		steps = append(steps, step{markA: 2, markB: -1, navRecent: true})
	}
	tr := feedOneList(t, steps, true)
	if _, why, ok := tr.Active(); ok {
		t.Errorf("a list whose mark never moved was named the active one after sixty "+
			"observations of somebody navigating: %s", why)
	}
}

// contains is a substring test, spelled so the assertions above read as questions.
func contains(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
