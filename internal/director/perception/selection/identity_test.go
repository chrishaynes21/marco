package selection_test

import (
	"image"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// A tracked candidate is a BUCKET, not one list, and both known repairs are worse.
//
// # The defect, measured
//
// Candidates are matched across frames by `overlap`, which divides the intersection by the
// SMALLER area. That is right in grouping and for a stated reason — a row contains its label,
// so the label scores 1.0 against the row and is discarded as a duplicate — and it means here
// that ANY group nested inside a tracked candidate's extent scores 1.0 and is taken for the
// same list. The candidate's bounds are then overwritten by the nested extent.
//
// Over 864 real frames of a game's pause menu, every candidate that ever qualified had a hull
// — the union of every extent it had held — spanning the WHOLE SCREEN, and had changed area by
// more than a factor of two between 37 and 127 times:
//
//	qualified   rehomed 37, 46, 101, 127     hull 0.000..1.000 in every case
//	refused     rehomed 0, 0, 0, 5, 6, 14, 87, 108
//
// So drifting is what EARNS a candidate its verdict. A bucket sees "a different member marked"
// whenever it absorbs a different group, which reads as a transfer among several holders — and
// every counter the decision reads is a sum over unrelated rectangles.
//
// # Two repairs tried, both rejected on measurement
//
// Scored on the same corpus against the labelled list's own EXTENT — "correct list" means at
// least six of the nine labelled rows inside it and no more than 2.5 labelled pitches taller
// than the labelled list, so a full-screen rectangle fails it:
//
//	                        correct list   coverage   accuracy   reader alone
//	as it is                45 of 829      185        43 (23%)   43 of 46 (93%)
//	intersection over union  6 of 829       86        44 (51%)   44 of 51 (86%)
//	a scale guard on overlap  0 of 829      269        87 (32%)   87 of 89 (98%)
//
// Intersection over union fragments the real menu, whose grouping genuinely varies between
// three and seven rows frame to frame — three rows of a six-row list score 0.33 — and it
// identified NOTHING at all at one sampling stride in four. The scale guard improved coverage,
// accuracy AND the reader's conditional score while taking correct-list identification to
// zero: it publishes one drifting bucket continuously, and the responsive arm never fires. It
// also made candidate matching reject so much that the replay slowed several times over.
//
// Three metrics improving while the one that matters collapses is the whole reason list
// identity is scored as an identity. Neither repair is in the tree; this test asserts the
// DEFECT, so a future repair has to confront what these two cost.

// rowsAt builds `n` rows of one group spanning y0..y1, which is what makes an extent.
func rowsAt(group, n, x0, y0, y1 int) []selection.Row {
	out := make([]selection.Row, 0, n)
	h := (y1 - y0) / n
	for i := range n {
		out = append(out, selection.Row{
			Bounds: image.Rect(x0, y0+i*h, x0+300, y0+i*h+h-2),
			Group:  group,
		})
	}
	return out
}

// A small group nested inside a tracked list is folded into it, and should not be.
//
// THE DEFECT, ASSERTED. If this starts failing, a repair has landed — and the repair has to
// beat the table above on correct-list identification, not on coverage or accuracy, because
// both rejected attempts improved those.
func TestANestedGroupIsStillFoldedIntoTheListAroundIt(t *testing.T) {
	tr := selection.NewTracker()
	// A five-row list over most of the panel, watched long enough to be a candidate.
	big := rowsAt(0, 5, 100, 100, 600)
	for i := range 12 {
		tr.ObserveShades(big, shadesWithMark(5, i%5))
	}
	if len(tr.Evidence()) != 1 {
		t.Fatalf("%d candidate(s) from one list, want 1", len(tr.Evidence()))
	}
	// A PARTIALLY OVERLAPPING EXTENT NEXT, which drags the same candidate down the screen:
	// the intersection is more than half the smaller area, so it matches, and the hull has
	// to grow to hold both. Done before the sliver because the hull growing is what a
	// NESTED extent cannot demonstrate — the union of a rectangle with one inside it is
	// the outer rectangle, so a hull only ever assigned at creation would look right.
	down := rowsAt(0, 5, 100, 300, 800)
	for i := range 12 {
		tr.ObserveShades(down, shadesWithMark(5, i%5))
	}
	if n := len(tr.Evidence()); n != 1 {
		t.Fatalf("%d candidates after a partially overlapping extent, want 1", n)
	}
	if h := tr.Evidence()[0].Hull; h.Min.Y > 100 || h.Max.Y < 790 {
		t.Errorf("the candidate's hull is %v; it has held extents from y 100 to y 800, "+
			"so a hull that does not span both has stopped being the union of "+
			"every extent and is only the one it started with", h)
	}
	// THEN A SLIVER INSIDE IT, arriving on its own because the detector found only those
	// rows this frame. Nested completely: 400..460 sits inside 300..800, so it scores 1.0
	// against the tracked extent and is taken for the same list.
	small := rowsAt(0, 3, 120, 400, 460)
	for i := range 12 {
		tr.ObserveShades(small, shadesWithMark(3, i%3))
	}

	ev := tr.Evidence()
	if len(ev) != 1 {
		t.Fatalf("%d candidates tracked. The nesting defect is REPAIRED, which is good "+
			"news and makes this test wrong: delete it, and check the repair against "+
			"correct-list identification on the real corpus — 45 of 829 is the "+
			"number to beat, and two repairs that improved coverage and accuracy "+
			"took it to 6 and to 0.\n%v", len(ev), ev)
	}
	e := ev[0]
	if e.Rehomed == 0 {
		t.Errorf("the candidate reports no change of area, so the sliver did not drag " +
			"its bounds and this fixture no longer reaches the defect")
	}
	if e.Hull.Dy() <= e.Bounds.Dy() {
		t.Errorf("the candidate's hull %v is no larger than its extent %v, so nothing "+
			"recorded that it has held more than one rectangle. Hull and Rehomed "+
			"are the only way this is visible: Bounds is overwritten every frame "+
			"and shows wherever it ended up", e.Hull, e.Bounds)
	}
	// AND THE PUBLISHED EXTENT IS NOW THE SLIVER, which is the shape the defect takes on
	// a screen: the active list moves to a few pixels of nothing while its counters still
	// describe the list it used to be.
	if e.Bounds.Dy() > 200 {
		t.Errorf("the candidate's extent is still %v; the defect is that it becomes "+
			"the nested sliver", e.Bounds)
	}
	t.Logf("one candidate, extent %v, hull %v, rehomed %d, %d transfers among %d holders",
		e.Bounds, e.Hull, e.Rehomed, e.Transfers, e.Holders)
}

// A candidate that qualified records WHEN, so the order they crossed the bar in is readable.
//
// The sole-qualifier arm publishes without comparing anything, so on a session where it fires
// the order candidates qualify in IS the decision. After the fact the counters cannot say who
// was first — on the real corpus the bucket that won qualified at observation 15 and the real
// menu at 162, and nothing but this field says so.
//
// Deleting the qualifiedAt assignment in Tracker.update must fail this.
func TestACandidateRecordsWhenItQualified(t *testing.T) {
	tr := selection.NewTracker()
	for _, e := range tr.Evidence() {
		t.Fatalf("a fresh tracker reports a candidate: %s", e)
	}
	tr2 := feedOneList(t, oneWalk(60, true), true)
	var qualified int
	for _, e := range tr2.Evidence() {
		if !e.Qualifies {
			if e.QualifiedAt >= 0 {
				t.Errorf("a candidate that does not qualify reports qualifying at "+
					"observation %d", e.QualifiedAt)
			}
			continue
		}
		qualified++
		if e.QualifiedAt <= 0 {
			t.Errorf("a qualifying candidate reports qualifying at observation %d; "+
				"it cannot have qualified before it was ever observed",
				e.QualifiedAt)
		}
		if e.QualifiedAt > e.Frames {
			t.Errorf("a candidate seen on %d frame(s) reports qualifying at "+
				"observation %d", e.Frames, e.QualifiedAt)
		}
	}
	if qualified == 0 {
		t.Fatal("no candidate qualified, so nothing above was checked")
	}

	// AND IT IS THE FIRST QUALIFICATION, not the latest. Recorded once and never
	// rewritten, because the question it answers is which candidate got there first —
	// and a field overwritten on every later frame says only "it still qualifies", which
	// every counter already says.
	before := map[image.Rectangle]int{}
	for _, e := range tr2.Evidence() {
		if e.Qualifies {
			before[e.Bounds] = e.QualifiedAt
		}
	}
	for range 20 {
		tr2.ObserveNavigated(listRows(0, 5, 100, 100), shadesWithMark(5, 2), true)
	}
	for _, e := range tr2.Evidence() {
		was, ok := before[e.Bounds]
		if !ok || !e.Qualifies {
			continue
		}
		if e.QualifiedAt != was {
			t.Errorf("a candidate's qualification moved from observation %d to %d "+
				"after twenty more looks. It is the FIRST crossing that is the "+
				"decision, because the arm that publishes a sole qualifier "+
				"compares nothing", was, e.QualifiedAt)
		}
	}
}
