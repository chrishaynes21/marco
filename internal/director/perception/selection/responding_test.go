package selection_test

import (
	"image"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// WHICH OF TWO REAL LISTS IS THE ONE BEING USED.
//
// Measured on 864 frames of a real game's pause menu: the menu qualifies as a list handing a
// mark between its members, and so does the track-information panel beside it. Both are real
// lists. Geometry cannot separate them and never will — one of them is simply not the one the
// person is navigating, and nothing about how it is drawn says so.
//
// What separates them is that one of them RESPONDS. These tests pin that rule, and in
// particular pin the thing that makes it perception rather than inference from intent: the
// input labels a transfer the shading already proved, and can never create one.

const (
	// One scaled row height, and the geometry of two lists far apart.
	respRowH  = 40
	respPitch = 60
)

// listRows builds `n` evenly spaced rows for one group at a horizontal offset.
func listRows(group, n, x0, y0 int) []selection.Row {
	out := make([]selection.Row, 0, n)
	for i := range n {
		out = append(out, selection.Row{
			Bounds: image.Rect(x0, y0+i*respPitch, x0+300, y0+i*respPitch+respRowH),
			Group:  group,
		})
	}
	return out
}

// shadesWithMark returns one shade per row, with `marked` standing well clear of the rest.
//
// A real measurement is two numbers per row; these are chosen so the marked row clears
// SpatialK robust deviations and MinAbsolute luminance, and the unmarked rows do not.
func shadesWithMark(n, marked int) []selection.Shade {
	out := make([]selection.Shade, n)
	for i := range n {
		out[i] = selection.Shade{60, 90}
		if i == marked {
			out[i] = selection.Shade{210, 240}
		}
	}
	return out
}

// twoLists feeds a tracker two lists at once, each with its own mark, and says per observation
// how many navigation inputs were seen.
//
// One call per observation, both lists on every frame, which is the measured situation: the
// panel does not go away while the menu is used.
type step struct {
	markA, markB int // -1 for no mark in that list
	navs         int
}

func feedTwoLists(t *testing.T, steps []step) *selection.Tracker {
	t.Helper()
	tr := selection.NewTracker()
	for _, s := range steps {
		rowsA := listRows(0, 5, 100, 100)
		rowsB := listRows(1, 5, 900, 100)
		shA := shadesWithMark(5, s.markA)
		shB := shadesWithMark(5, s.markB)
		tr.ObserveNavigated(append(rowsA, rowsB...), append(shA, shB...), s.navs)
	}
	return tr
}

// walk builds steps where list A's mark moves only on observations that carry input, and list
// B's moves on its own schedule.
func walk(n int) []step {
	var out []step
	a, b := 0, 0
	for i := range n {
		// Baselines first: a member's level is learned only from frames in which it was
		// NOT marked, so nothing can be judged until every row has been quiet a few
		// times. Leaving this out makes every early frame unreadable and the test
		// measures the warm-up instead of the rule.
		navs := 0
		if i >= 8 && i%3 == 0 {
			a = (a + 1) % 5
			navs = 1
		}
		if i >= 8 && i%4 == 0 {
			b = (b + 1) % 5
		}
		out = append(out, step{markA: a, markB: b, navs: navs})
	}
	return out
}

// The list whose transfers followed navigation is the active one.
//
// Deleting the `navs` argument from the watch's ObserveNavigated call, or the attribution in
// Tracker.update, must fail this.
func TestTheActiveListIsTheOneThatRespondsToNavigation(t *testing.T) {
	tr := feedTwoLists(t, walk(60))

	ev := tr.Evidence()
	var a, b selection.Evidence
	for _, e := range ev {
		// List A sits at x=100, list B at x=900.
		if e.Bounds.Min.X < 500 {
			a = e
		} else {
			b = e
		}
	}
	if !a.Qualifies || !b.Qualifies {
		t.Fatalf("the fixture needs BOTH lists to qualify, or it is not testing "+
			"disambiguation at all.\n  A: %s qualifies=%v\n  B: %s qualifies=%v",
			a, a.Qualifies, b, b.Qualifies)
	}
	if a.Prompted <= b.Prompted {
		t.Fatalf("list A's transfers were supposed to follow input and B's not: "+
			"A prompted %d of %d, B prompted %d of %d",
			a.Prompted, a.Transfers, b.Prompted, b.Transfers)
	}

	_, why, ok := tr.Active()
	if !ok {
		t.Fatalf("two qualifying lists and only one of them responds, and the tracker "+
			"still abstains: %s", why)
	}
	if !strings.Contains(why, "responds to navigation") {
		t.Errorf("the reason does not say what decided it: %q", why)
	}
	bounds, _, _ := tr.Active()
	if bounds.Min.X >= 500 {
		t.Errorf("the tracker named the list at x=%d, which is the one whose mark moved "+
			"on its own schedule", bounds.Min.X)
	}
}

// Two lists that BOTH respond stay ambiguous.
//
// The case that must not be resolved. Picking between two lists that both answer navigation is
// guessing, and a guess that happens to be right teaches nothing about the next one.
func TestTwoListsThatBothRespondStayAmbiguous(t *testing.T) {
	var steps []step
	a, b := 0, 0
	for i := range 60 {
		navs := 0
		if i >= 8 && i%3 == 0 {
			a = (a + 1) % 5
			b = (b + 1) % 5
			navs = 1
		}
		steps = append(steps, step{markA: a, markB: b, navs: navs})
	}
	tr := feedTwoLists(t, steps)

	qualifying := 0
	for _, e := range tr.Evidence() {
		if e.Qualifies {
			qualifying++
		}
	}
	if qualifying < 2 {
		t.Fatalf("only %d list(s) qualified, so this is not the ambiguous case", qualifying)
	}
	if _, why, ok := tr.Active(); ok {
		t.Errorf("two lists both responded to navigation and the tracker named one "+
			"anyway: %s", why)
	} else if !strings.Contains(why, "not determined") {
		t.Errorf("the abstention does not say the evidence was insufficient: %q", why)
	}
}

// With no input observed anywhere, two qualifying lists stay ambiguous.
//
// The regression this must not break: a session with no input source, or a person who pressed
// nothing, must not have its silence read as "nobody navigated, so pick either". It is the
// behaviour that existed before attribution, preserved.
func TestWithNoInputObservedTwoListsStayAmbiguous(t *testing.T) {
	steps := walk(60)
	for i := range steps {
		steps[i].navs = 0
	}
	tr := feedTwoLists(t, steps)

	qualifying := 0
	for _, e := range tr.Evidence() {
		if e.Qualifies {
			qualifying++
		}
		if e.Prompted != 0 {
			t.Errorf("a transfer was attributed to input on a run where none was "+
				"observed: %s", e)
		}
	}
	if qualifying < 2 {
		t.Fatalf("only %d list(s) qualified; the fixture is wrong", qualifying)
	}
	if _, _, ok := tr.Active(); ok {
		t.Error("with no input evidence at all the tracker picked a list. Silence about " +
			"what the person did is not evidence about which list they were using")
	}
}

// INPUT CANNOT MANUFACTURE A VISUAL OUTCOME.
//
// The invariant, stated as a test. A list whose mark never moves cannot be named the active one
// however much navigation is observed, because there is nothing for the input to be attributed
// to. Every counter the rule reads is an increment of a move the shading proved.
func TestNavigationAloneNamesNothing(t *testing.T) {
	// Both lists hold still. The person navigates throughout.
	var steps []step
	for range 60 {
		steps = append(steps, step{markA: 2, markB: 4, navs: 1})
	}
	tr := feedTwoLists(t, steps)

	for _, e := range tr.Evidence() {
		if e.Transfers != 0 {
			t.Errorf("a transfer was counted on a list whose mark never moved: %s", e)
		}
		if e.Prompted != 0 {
			t.Errorf("input was attributed to a transfer that did not happen: %s", e)
		}
		if e.Qualifies {
			t.Errorf("a list that never moved its mark qualifies: %s", e)
		}
	}
	if _, why, ok := tr.Active(); ok {
		t.Errorf("sixty observations of navigation and no visual change, and the tracker "+
			"named a list: %s", why)
	}
}

// A press that lands between two observations is still attributed to the move it caused.
//
// NavWindow exists for this: the press and the repaint fall in the same observation or in
// consecutive ones depending on where the cadence lands, and a window of one observation would
// miss a real response about half the time.
func TestAResponseOneObservationLateIsStillAttributed(t *testing.T) {
	// Input on observation i, mark moves on i+1. Nothing else moves.
	var steps []step
	a := 0
	for i := range 60 {
		navs := 0
		if i >= 8 && i%4 == 0 {
			navs = 1
		}
		if i >= 9 && i%4 == 1 {
			a = (a + 1) % 5
		}
		steps = append(steps, step{markA: a, markB: 3, navs: navs})
	}
	tr := feedTwoLists(t, steps)

	for _, e := range tr.Evidence() {
		if e.Bounds.Min.X >= 500 {
			continue
		}
		if e.Transfers == 0 {
			t.Fatalf("the fixture's mark never moved: %s", e)
		}
		if e.Prompted == 0 {
			t.Errorf("%d transfers, all one observation after a press, and none was "+
				"attributed: %s. NavWindow is meant to cover exactly this",
				e.Transfers, e)
		}
	}
}
