package selection_test

import (
	"image"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// RESPONDING IS A SHARE, NOT A TALLY.
//
// # The measurement that forced this
//
// 864 frames of a real pause menu, with the response window applied once at its stated bound:
//
//	the menu    3 of its 7 moves attributed    43%
//	the panel   5 of its 31 moves attributed   16%   <-- won on COUNT
//
// 31 presses over 57 seconds and a 400ms window cover about 22% of the session, so a list that
// moves on its own schedule collects attributed moves at about that rate whatever it is doing.
// The panel's 16% is at or below pure coincidence; the menu's 43% is twice it. Comparing tallies
// picked the panel, and the belief then named rows in the wrong list — confidently, and with
// every counter in the report looking healthy.
//
// A tally rewards whichever list moves most. A share asks the question that was meant.

// sharesWalk makes list A move only when something was pressed, and list B move constantly.
//
// Some presses move nothing in A. That is ordinary — a person at the bottom of a menu pressing
// down again — and it is what lets B's TALLY of attributed moves exceed A's while its SHARE
// stays near coincidence. Without it the two tally equally and the test cannot tell the rules
// apart.
func sharesWalk(n int) []step {
	var out []step
	a, b := 0, 0
	for i := range n {
		nav := false
		// Baselines first: a member's level is learned only from frames in which it was
		// not marked, so nothing is judged until every row has been quiet a few times.
		if i >= 10 && i%6 == 0 {
			a = (a + 1) % 5
			nav = true
		}
		if i >= 10 && i%5 == 0 {
			nav = true
		}
		if i >= 10 && i%2 == 0 {
			b = (b + 1) % 5
		}
		out = append(out, step{markA: a, markB: b, navRecent: nav})
	}
	return out
}

// The list that answered a larger SHARE of its own moves is the active one, even when a busier
// list has more attributions.
//
// Changing `moreResponsive` back to comparing prompted counts, or dropping the cross-multiplied
// margin in `responding`, must fail this.
func TestTheMostResponsiveListWinsNotTheBusiest(t *testing.T) {
	tr := feedTwoLists(t, sharesWalk(120))

	var a, b selection.Evidence
	for _, e := range tr.Evidence() {
		if e.Bounds.Min.X < 500 {
			a = e
		} else {
			b = e
		}
	}
	if a.Prompted == 0 || b.Prompted == 0 {
		t.Fatalf("the fixture attributed a %d/%d and b %d/%d; both need attributions for "+
			"there to be a comparison", a.Prompted, a.Transfers, b.Prompted, b.Transfers)
	}
	// THE PRECONDITION THIS TEST RESTS ON. It only distinguishes the two rules while the
	// busy list would WIN on a tally, so if the fixture stops producing that, the test has
	// to say so rather than pass.
	if b.Prompted <= a.Prompted {
		t.Fatalf("the busy list has %d attributions against the sparse list's %d, so a "+
			"tally would not have picked it and this proves nothing",
			b.Prompted, a.Prompted)
	}
	if a.Prompted*b.Transfers <= b.Prompted*a.Transfers {
		t.Fatalf("the sparse list does not lead on share either (%d/%d against %d/%d)",
			a.Prompted, a.Transfers, b.Prompted, b.Transfers)
	}

	live, why, ok := tr.Active()
	if !ok {
		t.Fatalf("no list was named: %s", why)
	}
	if live.Min.X >= 500 {
		t.Errorf("the busiest list was named rather than the most responsive one: "+
			"%d of %d against %d of %d. %s",
			b.Prompted, b.Transfers, a.Prompted, a.Transfers, why)
	}
	t.Logf("named the list with %d of %d (%.0f%%) over the one with %d of %d (%.0f%%)",
		a.Prompted, a.Transfers, 100*float64(a.Prompted)/float64(a.Transfers),
		b.Prompted, b.Transfers, 100*float64(b.Prompted)/float64(b.Transfers))
}

// Two lists that answer the same share of their own moves stay ambiguous.
//
// The case that must NOT be resolved, preserved across the change from tallies to shares. A
// rule that picks between two equally responsive lists is guessing, and a guess that happens
// to be right teaches nothing about when to trust the next one.
func TestTwoEquallyResponsiveListsStayAmbiguous(t *testing.T) {
	// Both lists move on exactly the observations that carry navigation, so both answer
	// every move they make.
	var steps []step
	a, b := 0, 0
	for i := range 120 {
		nav := false
		if i >= 10 && i%4 == 0 {
			a, b = (a+1)%5, (b+1)%5
			nav = true
		}
		steps = append(steps, step{markA: a, markB: b, navRecent: nav})
	}
	tr := feedTwoLists(t, steps)

	var ea, eb selection.Evidence
	qualifying := 0
	for _, e := range tr.Evidence() {
		if e.Qualifies {
			qualifying++
		}
		if e.Bounds.Min.X < 500 {
			ea = e
		} else {
			eb = e
		}
	}
	if qualifying < 2 {
		t.Fatalf("only %d list qualified (a %v, b %v), so there is no ambiguity here to "+
			"leave unresolved", qualifying, ea, eb)
	}
	_, why, ok := tr.Active()
	if ok {
		t.Errorf("a list was named although both answered every move they made "+
			"(a %d/%d, b %d/%d): %s", ea.Prompted, ea.Transfers,
			eb.Prompted, eb.Transfers, why)
	}
	if !strings.Contains(why, "does not single one of them out") {
		t.Errorf("abstained for the wrong reason. Two lists both responding must be "+
			"reported as such, not as nothing having been seen: %s", why)
	}
}

// ── a list whose styling drifts down the page ─────────────────────────────────

// A long list is still one list when each row differs slightly from the one above it.
//
// # The measurement
//
// A real pause menu's panel is drawn slanted: each entry's left edge sits about half a per cent
// of the frame further left than the one above. Adjacent rows are well inside the alignment
// tolerance and the sixth row is not — so comparing every candidate member to the SEED made
// the test stricter the longer the run got. Nine entries, every one proposed, every one
// surviving deduplication, and the run stopped at four.
//
// Deleting the change from `alike(boxes[i], ...)` to `alike(boxes[prev], ...)` in run() must
// fail this.
func TestALongListWhoseStyleDriftsIsStillOneList(t *testing.T) {
	const (
		rows  = 9
		h     = 40
		pitch = 60
		// Per row, a fifth of a row height. Five rows of it exceeds the alignment
		// tolerance of half a row height; one row of it is nothing.
		drift = h / 5
	)
	var boxes []image.Rectangle
	for i := range rows {
		x := 500 - i*drift
		// WIDTHS THAT DIFFER, alternating by more than stackTolerance. Without this
		// every pair is alike on WIDTH alone and alike() returns before it ever reaches
		// the alignment test — so the drift is never consulted and the test passes
		// against the very rule it exists to pin. A real menu's rows are the extents of
		// words of different lengths; identical widths are a drawn fixture's artefact.
		w := 200 + (i%2)*120
		boxes = append(boxes, image.Rect(x, 100+i*pitch, x+w, 100+i*pitch+h))
	}
	grouped := selection.GroupRows(boxes)

	size := map[int]int{}
	for _, r := range grouped {
		size[r.Group]++
	}
	best := 0
	for _, n := range size {
		if n > best {
			best = n
		}
	}
	if best < rows {
		t.Errorf("a %d-row list whose left edge drifts %d per row grouped as %v. Each row "+
			"is well within tolerance of the row above it; only the distance from the "+
			"FIRST row exceeds it, and a list is a chain of neighbours",
			rows, drift, size)
	}
	t.Logf("%d rows with %d drift per row grouped as %v", rows, drift, size)
}

// And a chain of neighbours does not drift into something of a different size.
//
// The seed keeps one veto, on height, because height is what makes a row a row. Without it a
// neighbourly chain is free to walk from a menu entry to a panel one gentle step at a time.
func TestAChainOfNeighboursCannotDriftIntoADifferentScale(t *testing.T) {
	const pitch = 60
	var boxes []image.Rectangle
	h := 40
	for i := range 9 {
		// Each row a quarter taller than the last: every adjacent pair is inside the
		// 0.35 tolerance of each other, and the ninth is nothing like the first.
		//
		// CENTRED ON A FIXED BEAT. Growing the height downwards from a fixed top moves
		// each centre as well, which breaks the pitch and makes this test pass because
		// no run forms at all — which is how the first version of it passed against the
		// mutant it exists to catch.
		centre := 300 + i*pitch
		boxes = append(boxes, image.Rect(500, centre-h/2, 800, centre+h/2))
		h = h * 5 / 4
	}
	grouped := selection.GroupRows(boxes)
	size := map[int]int{}
	for _, r := range grouped {
		size[r.Group]++
	}
	// THE FLOOR IS THE MEASUREMENT. With the seed's height veto the chain stops at three;
	// without it, it reaches five, walking from a 40-tall row to a 97-tall one. Nine is
	// what it would reach if nothing stopped it at all, so asserting only "fewer than
	// nine" passes against the mutant and proves nothing — which is how the first version
	// of this test survived its own gate.
	const maxChain = 3
	for g, n := range size {
		if n > maxChain {
			t.Errorf("group %d took %d boxes, whose heights run from 40 to %d. With the "+
				"seed's height veto the chain stops at %d; a longer one has walked "+
				"from a row to something several times its size one gentle step at a "+
				"time", g, n, h, maxChain)
		}
	}
	t.Logf("heights 40..%d grouped as %v", h, size)
}
