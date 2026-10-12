package observe

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// The banked moment belongs to ONE row.
//
// In package, because the claim is about a lookup's contract rather than about an outcome. The
// only guard left for a stale sighting is settlingOf's precondition: an earlier version also
// cleared the banked moment wherever the identified list changed, and the mutation gate showed
// those resets changed no outcome, because a move is only ever emitted on a look that found the
// same list as the look before it.
//
// That makes the precondition the whole of the protection, and a precondition no caller can
// currently violate is exactly the kind of thing that rots. So it is asserted directly: hand
// settlingOf a row other than the one banked and it must say it knows nothing, rather than
// handing back another row's moment and another row's answer about the window.
//
// Deleting the `row != w.settlingRow` test in settlingOf must fail this.
func TestABankedMomentBelongsToOneRow(t *testing.T) {
	w := NewInterfaceWatch()
	at := time.Unix(1760000000, 0)

	// Nothing banked: every row is unknown, and row 0 is not a special case.
	for _, row := range []int{-1, 0, 3} {
		if _, _, known := w.settlingOf(row); known {
			t.Errorf("row %d has a banked moment before anything was observed", row)
		}
	}

	// AND AN UNDATED SIGHTING IS NOT BANKED, which is the invariant settlingOf rests on.
	//
	// It used to test for a zero time itself. The test moved to noteSettling because a
	// moment with no time is not a moment and storing one only to reject it later is two
	// answers to one question — so a banked entry now implies settlingAt is set, and
	// that implication is what this asserts. A session with no clock really happens: it
	// is the fixture that makes the press ledger report direction_unknown.
	//
	// Deleting the at.IsZero() refusal in noteSettling must fail this.
	w.noteSettling(selection.Result{
		Outcome: selection.Insufficient, Row: -1, Provisional: 2, ProvisionalEntry: 2, ProvisionalN: 1,
	}, time.Time{})
	if gotAt, _, known := w.settlingOf(2); known {
		t.Errorf("a sighting with no clock was banked and reads back as known (%s). "+
			"Without a timestamp there is no moment, and settlingOf no longer "+
			"checks for one because nothing is supposed to store it", gotAt)
	}

	// A press opens the window, then row 2 is pointed at and not published.
	w.Navigated(at, 1)
	w.noteSettling(selection.Result{
		Outcome: selection.Insufficient, Row: -1, Provisional: 2, ProvisionalEntry: 2, ProvisionalN: 1,
	}, at)

	got, prompted, known := w.settlingOf(2)
	if !known {
		t.Fatal("row 2 was pointed at and has no banked moment, so this test asserts " +
			"nothing about the row that follows")
	}
	if !got.Equal(at) {
		t.Errorf("row 2's moment is %s, want %s", got, at)
	}
	if !prompted {
		t.Error("row 2 was pointed at with the response window open and the banked " +
			"answer says it was not")
	}

	// EVERY OTHER ROW IS UNKNOWN, including the one next to it and row 0.
	for _, row := range []int{0, 1, 3, 4} {
		gotAt, gotPrompted, gotKnown := w.settlingOf(row)
		if gotKnown {
			t.Errorf("row %d was handed row 2's moment (%s, prompted=%v). A move "+
				"dated from a sighting of a different row is dated to whatever "+
				"was happening when some other entry was about to be marked, "+
				"and judged against the window as it stood then",
				row, gotAt, gotPrompted)
		}
	}
}

// A published look leaves the banked moment standing.
//
// The look that confirms a row is the look AFTER the one that proposed it, and on that look the
// reader reports nothing provisional — it has published instead. Clearing on a -1 would throw
// the moment away exactly when noteMove is about to read it, which is the whole mechanism.
//
// In package for the same reason as above: the end-to-end test
// TestAMoveSaysWhenThePixelsMovedAndNotOnlyWhenItWasBelieved holds the outcome, and this holds
// the reason, in one assertion with nothing else in it.
func TestAPublishedLookLeavesTheBankedMomentStanding(t *testing.T) {
	w := NewInterfaceWatch()
	at := time.Unix(1760000000, 0)
	w.noteSettling(selection.Result{
		Outcome: selection.Insufficient, Row: -1, Provisional: 2, ProvisionalEntry: 2, ProvisionalN: 1,
	}, at)
	// The next look publishes row 2 and reports nothing provisional.
	w.noteSettling(selection.Result{
		Outcome: selection.Selected, Row: 2, Provisional: -1, Entry: 2,
	}, at.Add(100*time.Millisecond))

	got, _, known := w.settlingOf(2)
	if !known {
		t.Fatal("the look that published row 2 cleared the moment row 2 was first " +
			"pointed at. The confirmation is always the later look, so clearing " +
			"there leaves every move dated from its confirmation again")
	}
	if !got.Equal(at) {
		t.Errorf("row 2's moment moved to %s; the pixels first pointed at it at %s",
			got, at)
	}
}

// An undated move is judged by the window as it stands, which is what every move used to be.
//
// # Why this is in package, and why it is not a redundant test
//
// Because no public path reaches it. A move is emitted when the published row differs from the
// row the mark was last seen on, and the only way the reader publishes a NEW row is by pending
// it first — which banks a moment. Measured: 2 of 2 observations on the real corpus and 27 of
// 27 on a scripted walk were dated. The mutation gate confirmed it from the other side, by
// replacing the fallback with `false` and finding no test that cared.
//
// So the fallback is a CONTRACT rather than a branch anybody takes: it says what happens if a
// reader ever publishes a row without settling it, and the answer has to be the behaviour every
// move had before FirstSeen existed, or a reader change would silently stop attributing
// presses. A contract no caller can currently violate is exactly the kind of thing that rots,
// so it is stated here directly.
//
// Deleting the `prompted := w.navDebt > 0` fallback in noteMove must fail this.
func TestAnUndatedMoveIsJudgedByTheWindowAsItStands(t *testing.T) {
	list := Region{X: 0.05, Y: 0.10, Width: 0.30, Height: 0.60}
	// A belief about one list, with the mark on a given entry. Status must be provisional
	// or noteMove drops the held sighting instead of comparing against it.
	// The entry identities are what noteMove compares, and ListEntries is the list's own
	// count rather than this frame's — both are what a move is recorded against now, so a
	// belief built by hand has to carry them or there is no move to assert anything about.
	belief := func(sel int, at time.Time) InterfaceBelief {
		return InterfaceBelief{
			Status: InterfaceProvisional, Where: list, Entries: 5,
			ListEntries: 5, SelectedEntry: sel + 1, SelectedRank: sel,
			Selected: sel, At: at, Sample: 1,
		}
	}
	at := time.Unix(1760000000, 0)

	for _, tc := range []struct {
		name     string
		press    bool
		prompted bool
	}{
		{"a press is still in the window", true, true},
		{"nothing was pressed", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := NewInterfaceWatch()
			if tc.press {
				w.Navigated(at, 1)
			}
			// Two settles over the SAME list, with no reading in between to bank a
			// moment: the mark is seen on entry 1 and then on entry 2.
			w.noteMove(InterfaceBelief{}, belief(1, at))
			w.noteMove(belief(1, at), belief(2, at.Add(50*time.Millisecond)))

			moves, _ := w.TakeSelectionMoves()
			if len(moves) != 1 {
				t.Fatalf("%d move(s) from two sightings in one list, want 1", len(moves))
			}
			m := moves[0]
			if !m.FirstSeen.IsZero() {
				t.Fatalf("the move is dated %s, so nothing here exercised the "+
					"fallback", m.FirstSeen)
			}
			if m.Prompted != tc.prompted {
				t.Errorf("an undated move with press=%v came out prompted=%v, "+
					"want %v. Undated means the sighting could not be "+
					"placed, and the honest answer is then the one every "+
					"move gave before it could be",
					tc.press, m.Prompted, tc.prompted)
			}
		})
	}
}
