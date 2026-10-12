package selection_test

import (
	"image"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// A tracked candidate is one LIST, because a list is the same list when its rows are.
//
// # The defect this replaced, measured
//
// Candidates used to be matched across frames by `overlap`, which divides the intersection by
// the SMALLER area. That is right in grouping and for a stated reason - a row contains its
// label, so the label scores 1.0 against the row and is discarded as a duplicate - and it meant
// here that ANY group nested inside a tracked candidate's extent scored 1.0 and was taken for
// the same list. The candidate's bounds were then overwritten by the nested extent.
//
// Over 864 real frames of a game's pause menu, every candidate that ever qualified had a hull
// - the union of every extent it had held - spanning the WHOLE SCREEN, and had changed area by
// more than a factor of two between 37 and 127 times:
//
//	qualified   rehomed 37, 46, 101, 127     hull 0.000..1.000 in every case
//	refused     rehomed 0, 0, 0, 5, 6, 14, 87, 108
//
// So drifting was what EARNED a candidate its verdict. A bucket sees "a different member
// marked" whenever it absorbs a different group, which reads as a transfer among several
// holders - and every counter the decision reads was a sum over unrelated rectangles.
//
// # Three repairs were measured. Two were rejected and are not in the tree
//
// Scored on the same corpus against the labelled list's own EXTENT - "correct list" means at
// least six of the nine labelled rows inside it and no more than 2.5 labelled pitches taller,
// so a full-screen rectangle fails it:
//
//	                          correct list   coverage   right   reader alone
//	the bucket rule            45 of 829     135        43      43 of 46 (93%)
//	intersection over union     6 of 829      86        44      44 of 51 (86%)
//	a scale guard on overlap     0 of 829     269        87      87 of 89 (98%)
//	ROW CORRESPONDENCE        266 of 829     109        31      31 of 40 (78%)
//
// Intersection over union fragments the real menu, whose grouping genuinely varies between
// three and seven rows frame to frame - three rows of a six-row list score 0.33 - and it
// identified NOTHING at one sampling stride in four. The scale guard improved coverage,
// accuracy AND the reader's conditional score while taking correct-list identification to
// zero. Both are recorded in [[ADR-164-a-list-is-the-same-list-when-its-rows-are]] so a future
// repair has to confront what they cost, and the number to beat is correct-list identification
// rather than coverage, because both rejected repairs improved coverage.

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

// A small group nested inside a tracked list is NOT that list.
//
// The sliver sits entirely inside the tracked extent, so under the old rule it scored 1.0 and
// became the list. Its rows are a fraction of a member's height from each other and most of a
// member's height from every member, so as ROWS they are not this list's rows.
//
// Reverting Tracker.match to `overlap(c.bounds, ext) >= CandidateMatch` must fail this.
func TestANestedGroupIsNotTheListAroundIt(t *testing.T) {
	tr := selection.NewTracker()
	big := rowsAt(0, 5, 100, 100, 600)
	for i := range 12 {
		tr.ObserveShades(big, shadesWithMark(5, i%5))
	}
	if len(tr.Evidence()) != 1 {
		t.Fatalf("%d candidate(s) from one list, want 1", len(tr.Evidence()))
	}
	first := tr.Evidence()[0]

	// A SLIVER INSIDE IT, arriving on its own because the detector found only those rows
	// this frame. Nested completely: 400..460 sits inside 100..600.
	small := rowsAt(0, 3, 120, 400, 460)
	for i := range 12 {
		tr.ObserveShades(small, shadesWithMark(3, i%3))
	}

	ev := tr.Evidence()
	if len(ev) != 2 {
		t.Fatalf("%d candidate(s) after a sliver nested inside a tracked list, want 2: "+
			"the sliver is not that list and folding it in is the defect this "+
			"replaced.\n%v", len(ev), ev)
	}
	for _, e := range ev {
		if e.Rehomed != 0 {
			t.Errorf("a candidate reports changing area by more than double %d "+
				"time(s). Neither of these two ever did: %s at %v, hull %v",
				e.Rehomed, e, e.Bounds, e.Hull)
		}
		if e.Hull != e.Bounds {
			t.Errorf("a candidate's hull %v is larger than its extent %v, so it has "+
				"held more than one rectangle", e.Hull, e.Bounds)
		}
	}
	var wide, thin int
	for _, e := range ev {
		if e.Bounds.Dy() > 200 {
			wide++
		} else {
			thin++
		}
	}
	if wide != 1 || thin != 1 {
		t.Errorf("want one candidate the size of the list and one the size of the "+
			"sliver, got %d and %d: %v", wide, thin, ev)
	}
	// AND THE LIST KEPT ITS OWN EVIDENCE. The sliver's twelve looks are not the list's.
	for _, e := range ev {
		if e.Bounds.Dy() > 200 && e.Frames != first.Frames {
			t.Errorf("the list was seen on %d frames and now reports %d: the sliver's "+
				"looks have been counted as the list's", first.Frames, e.Frames)
		}
	}
	t.Logf("two candidates: %v and %v", ev[0].Bounds, ev[1].Bounds)
}

// A frame that shows only PART of a list is still that list.
//
// The asymmetry in Tracker.match, stated as a test. A detector routinely finds three rows of a
// six-row menu, and on the real corpus the menu's grouping varies between three and seven rows
// frame to frame - so a rule that needed most of the CANDIDATE's rows back would fragment the
// one list that matters. Intersection over union is that rule, and it identified the right list
// on 6 of 829 frames against 266.
//
// Raising the share to be of the candidate's members rather than the frame's rows must fail this.
func TestAFrameShowingPartOfAListIsStillThatList(t *testing.T) {
	tr := selection.NewTracker()
	whole := rowsAt(0, 6, 100, 100, 700)
	for i := range 12 {
		tr.ObserveShades(whole, shadesWithMark(6, i%6))
	}
	if n := len(tr.Evidence()); n != 1 {
		t.Fatalf("%d candidates from one list, want 1", n)
	}
	seen := tr.Evidence()[0].Frames

	// HALF OF IT, the same three rows in the same places, which is what a dropout looks
	// like. Built by taking the whole list's rows rather than by re-deriving them, so the
	// fixture cannot drift away from the list it is a part of.
	part := whole[:3]
	for i := range 8 {
		tr.ObserveShades(part, shadesWithMark(3, i%3))
	}
	ev := tr.Evidence()
	if len(ev) != 1 {
		t.Fatalf("%d candidates after a dropout showing three of six rows, want 1: a "+
			"frame that shows part of a list is that list, and a rule that says "+
			"otherwise fragments every real menu.\n%v", len(ev), ev)
	}
	if ev[0].Frames != seen+8 {
		t.Errorf("the candidate was seen on %d frames and the eight dropout frames "+
			"took it to %d, want %d", seen, ev[0].Frames, seen+8)
	}
	if ev[0].Rows != 6 {
		t.Errorf("the candidate reports %d members after a dropout, want the 6 it "+
			"has: a list does not lose entries because one look missed them",
			ev[0].Rows)
	}
	// AND THE PUBLISHED EXTENT IS STILL THE LIST'S, not the dropout's. Callers restrict
	// the reader to rows inside it, so an extent that shrinks to whatever was found this
	// time hands the reader three rows of a six-row list and it can read none of them:
	// three siblings have no middle to deviate from. Measured, with the extent still
	// coming from the frame's group, 263 of 315 published frames were unreadable.
	if got, want := ev[0].Bounds, extentOf(whole); got != want {
		t.Errorf("the published extent after a dropout is %v, want the whole list's "+
			"%v. The members are the identity, so their extent is the list's",
			got, want)
	}
	// AND A FRAME THAT BRINGS A STRANGER is still the same list, as long as most of it
	// is known rows. Three of the list plus one row it has never seen: a share of the
	// CANDIDATE's members would refuse this, and that rule identified the right list on
	// 6 of 829 real frames against 312.
	withStranger := append([]selection.Row{}, part...)
	withStranger = append(withStranger, selection.Row{
		Bounds: image.Rect(100, 2000, 400, 2080), Group: 0,
	})
	for i := range 8 {
		tr.ObserveShades(withStranger, shadesWithMark(4, i%4))
	}
	if n := len(tr.Evidence()); n != 1 {
		t.Errorf("%d candidates after a frame of three known rows and one stranger, "+
			"want 1: three quarters of the frame is this list.\n%v", n,
			tr.Evidence())
	}
}

// extentOf is the rectangle a fixture's rows occupy, which is what the candidate publishes.
func extentOf(rows []selection.Row) image.Rectangle {
	out := rows[0].Bounds
	for _, r := range rows[1:] {
		out = out.Union(r.Bounds)
	}
	return out
}

// A stack that merely CONTAINS a list is not that list.
//
// The share is of this frame's rows, and this is the case that says why it has to be. A frame of
// twelve rows spanning the screen, five of which are a tracked list's, is a different structure
// that happens to include it — and folding it in is how a candidate became a bucket whose hull
// spanned the whole screen. The other direction is a detector dropout and must still match; see
// TestAFrameShowingPartOfAListIsStillThatList.
//
// Deleting the share from Tracker.match must fail this.
func TestAStackThatMerelyContainsAListIsNotThatList(t *testing.T) {
	tr := selection.NewTracker()
	list := rowsAt(0, 5, 100, 300, 500)
	for i := range 12 {
		tr.ObserveShades(list, shadesWithMark(5, i%5))
	}
	if n := len(tr.Evidence()); n != 1 {
		t.Fatalf("%d candidates from one list, want 1", n)
	}
	was := tr.Evidence()[0]

	// TWELVE ROWS DOWN THE SCREEN at the same pitch, five of which ARE the list's rows.
	// Built by taking the list's own rows so the five cannot drift away from it.
	big := append([]selection.Row{}, list...)
	for i := range 7 {
		y := 600 + i*40
		big = append(big, selection.Row{
			Bounds: image.Rect(100, y, 400, y+38), Group: 0,
		})
	}
	for i := range 12 {
		tr.ObserveShades(big, shadesWithMark(len(big), i%len(big)))
	}
	ev := tr.Evidence()
	if len(ev) != 2 {
		t.Fatalf("%d candidate(s) after a twelve-row stack containing the five-row "+
			"list, want 2.\n%v", len(ev), ev)
	}
	for _, e := range ev {
		if e.Hull != e.Bounds {
			t.Errorf("a candidate's hull %v is larger than its extent %v, so the "+
				"stack was folded into the list and dragged it", e.Hull, e.Bounds)
		}
	}
	for _, e := range ev {
		if e.Rows == was.Rows && e.Bounds != was.Bounds {
			t.Errorf("the list's extent moved from %v to %v", was.Bounds, e.Bounds)
		}
	}
}

// Three rows in one member's place are one row found again, not three.
//
// A detector returns a row, and its label, and an icon beside it, as separate rectangles in the
// same place. Counted as three correspondences they are most of a small frame and it matches
// anything; counted as one they are one row of this list seen again.
//
// Deleting the distinctness from `corresponding` must fail this.
func TestThreeRowsInOneMembersPlaceAreOneRowFoundAgain(t *testing.T) {
	tr := selection.NewTracker()
	list := rowsAt(0, 5, 100, 100, 600)
	for i := range 12 {
		tr.ObserveShades(list, shadesWithMark(5, i%5))
	}
	if n := len(tr.Evidence()); n != 1 {
		t.Fatalf("%d candidates from one list, want 1", n)
	}

	// ONE ROW PROPOSED THREE TIMES, at slightly different extents, which is what a
	// detector does when it cannot decide where a row ends. All three are the same row as
	// member 2 — within half their own height of it — so a tally makes this three of three
	// rows found again, a perfect correspondence to a list it shares ONE row with.
	m := list[2].Bounds
	crowd := []selection.Row{
		{Bounds: m, Group: 0},
		{Bounds: m.Add(image.Pt(0, 10)), Group: 0},
		{Bounds: m.Add(image.Pt(0, 20)), Group: 0},
	}
	for i := range 12 {
		tr.ObserveShades(crowd, shadesWithMark(3, i%3))
	}
	ev := tr.Evidence()
	if len(ev) != 2 {
		t.Errorf("%d candidate(s) after one row arrived proposed three times, want 2: "+
			"all three are one of this list's rows found again, and one row is not "+
			"a correspondence with a five-row list.\n%v", len(ev), ev)
	}
}

// The MOST corresponding candidate takes a frame, not the first one over the bar.
//
// The old rule returned the first extent that overlapped enough, which made the answer depend on
// the order candidates happened to be created in — and that order is frame order, which is the
// sampling cadence. The two candidates here overlap in height and are offset sideways just
// enough that neither is the other's rows, and the frame shares width with both. Three of its
// four rows are the first candidate's and all four are the second's.
//
// Taking the first candidate over the bar instead of the best must fail this.
func TestTheMostCorrespondingCandidateTakesAFrame(t *testing.T) {
	rowsAtX := func(x0 int, ys ...int) []selection.Row {
		out := make([]selection.Row, 0, len(ys))
		for _, y := range ys {
			out = append(out, selection.Row{
				Bounds: image.Rect(x0, y, x0+300, y+80), Group: 0,
			})
		}
		return out
	}
	tr := selection.NewTracker()
	// FIRST, a three-row list on the left.
	three := rowsAtX(100, 100, 200, 300)
	for i := range 12 {
		tr.ObserveShades(three, shadesWithMark(3, i%3))
	}
	// THEN a four-row list at the same heights, far enough right that no row of it shares
	// half its width with a row of the first.
	four := rowsAtX(350, 100, 200, 300, 400)
	for i := range 12 {
		tr.ObserveShades(four, shadesWithMark(4, i%4))
	}
	ev := tr.Evidence()
	if len(ev) != 2 {
		t.Fatalf("%d candidate(s) for two lists, want 2: the fixture needs both, and "+
			"the three-row one created FIRST.\n%v", len(ev), ev)
	}
	var sawThree, sawFour int
	for _, e := range ev {
		switch e.Rows {
		case 3:
			sawThree = e.Frames
		case 4:
			sawFour = e.Frames
		}
	}
	if sawThree == 0 || sawFour == 0 {
		t.Fatalf("want a three-row and a four-row candidate, got %v", ev)
	}

	// A FRAME BETWEEN THEM, sharing more than half its width with rows of both. Three of
	// its four rows are the three-row list's; all four are the four-row list's.
	between := rowsAtX(225, 100, 200, 300, 400)
	for i := range 10 {
		tr.ObserveShades(between, shadesWithMark(4, i%4))
	}
	for _, e := range tr.Evidence() {
		switch e.Rows {
		case 3:
			if e.Frames != sawThree {
				t.Errorf("the three-row list took the frame: seen on %d frames, "+
					"was %d. Four of the frame's rows are the four-row "+
					"list's and three are this one's, so the better "+
					"correspondence has to win or the answer depends on "+
					"which list was created first", e.Frames, sawThree)
			}
		case 4:
			if e.Frames != sawFour+10 {
				t.Errorf("the four-row list was seen on %d frames, want %d: it "+
					"corresponds to every row of the frame", e.Frames,
					sawFour+10)
			}
		}
	}
}

// A row nested inside a member is not that member, however near its centre.
//
// The scale-free half of `sameRow`, stated as a test. Both tolerances are fractions of the
// SHORTER of the two rectangles, so a small rectangle inside a tall member is a different row
// even when it sits well within the member's own bounds. Measured against the LONGER instead,
// three slivers nested one inside each of three members read as those three members and the list
// becomes whatever is drawn on top of it.
//
// Changing RowMatch's denominator to the taller of the two must fail this.
func TestASliverInsideEachRowIsNotThoseRows(t *testing.T) {
	tr := selection.NewTracker()
	list := rowsAt(0, 5, 100, 100, 600)
	for i := range 12 {
		tr.ObserveShades(list, shadesWithMark(5, i%5))
	}
	if n := len(tr.Evidence()); n != 1 {
		t.Fatalf("%d candidates from one list, want 1", n)
	}

	// THREE SMALL ROWS, one inside each of three members, each offset from its member's
	// centre by more than its OWN height and far less than the member's.
	var fine []selection.Row
	for i := range 3 {
		m := list[i].Bounds
		y := m.Min.Y + 70
		fine = append(fine, selection.Row{
			Bounds: image.Rect(m.Min.X+10, y, m.Min.X+290, y+10), Group: 0,
		})
	}
	for i := range 12 {
		tr.ObserveShades(fine, shadesWithMark(3, i%3))
	}
	ev := tr.Evidence()
	if len(ev) != 2 {
		t.Errorf("%d candidate(s) after three slivers nested one inside each of three "+
			"members, want 2.\n%v", len(ev), ev)
	}
	for _, e := range ev {
		if e.Rehomed != 0 {
			t.Errorf("a candidate changed area by more than double %d time(s): the "+
				"slivers were taken for the rows they sit inside", e.Rehomed)
		}
	}
}

// A row that only LOOSELY matches a member does not move it.
//
// Attribution and geometry are deliberately different rules. Attribution is tolerant because a
// selected row is redrawn and the row whose shade matters is the one least like its member;
// geometry is strict because a member that moves to wherever its nearest row was drags the list
// after it. Measured with the tolerant rule moving members: published extents grew to twelve and
// thirteen entries where the menu has nine, and the reader's conditional accuracy fell from 93%
// to 51%. The bucket re-formed one member at a time.
//
// Using memberAt instead of rowAt for the geometry update must fail this.
func TestARowThatOnlyLooselyMatchesAMemberDoesNotMoveIt(t *testing.T) {
	tr := selection.NewTracker()
	list := rowsAt(0, 5, 100, 100, 600)
	for i := range 12 {
		tr.ObserveShades(list, shadesWithMark(5, i%5))
	}
	if n := len(tr.Evidence()); n != 1 {
		t.Fatalf("%d candidates from one list, want 1", n)
	}
	was := tr.Evidence()[0].Bounds

	// THREE OF THE LIST'S OWN ROWS, so the frame is this list, plus two rows 80 pixels
	// below where the last two members are. Eighty is well inside a quarter of the
	// list's height, which is what attribution allows, and well outside half a row's
	// height, which is what being the same row allows.
	mixed := append([]selection.Row{}, list[:3]...)
	for _, k := range list[3:] {
		b := k.Bounds
		mixed = append(mixed, selection.Row{
			Bounds: b.Add(image.Pt(0, 80)), Group: 0,
		})
	}
	for i := range 12 {
		tr.ObserveShades(mixed, shadesWithMark(5, i%5))
	}
	ev := tr.Evidence()
	var list5 *selection.Evidence
	for i := range ev {
		if ev[i].Rows == 5 && ev[i].Bounds.Min.Y <= was.Min.Y {
			list5 = &ev[i]
		}
	}
	if list5 == nil {
		t.Fatalf("the five-row list is no longer tracked.\n%v", ev)
	}
	if list5.Bounds.Max.Y > was.Max.Y {
		t.Errorf("the list's extent grew from %v to %v. Two rows 80 pixels below its "+
			"last two members are not those members, and letting them move the "+
			"members drags the list after whatever is drawn near it",
			was, list5.Bounds)
	}
	if list5.Rehomed != 0 {
		t.Errorf("the list changed area by more than double %d time(s)", list5.Rehomed)
	}
}

// A candidate is never shrunk below a stack; one that is not a list dies whole.
//
// Members age out, and a list that keeps losing them would otherwise dwindle to two rows and
// carry on being consulted. Two rows cannot show a mark transferring at all, so what is left is
// not a list and the honest end for it is to stop being found and be retired.
//
// Deleting the minStack term from forgetColdMembers must fail this.
func TestACandidateIsNeverShrunkBelowAStack(t *testing.T) {
	tr := selection.NewTracker()
	list := rowsAt(0, 4, 100, 100, 500)
	for i := range 12 {
		tr.ObserveShades(list, shadesWithMark(4, i%4))
	}
	if n := tr.Evidence()[0].Rows; n != 4 {
		t.Fatalf("the candidate has %d members, want 4", n)
	}

	// TWO OF ITS FOUR ROWS, plus one stranger, for longer than a member may go unfound.
	// Two of three rows is a share this list still wins, so it keeps being matched while
	// two of its members are never seen again.
	thin := []selection.Row{list[0], list[1], {
		Bounds: image.Rect(100, 900, 400, 980), Group: 0,
	}}
	for i := range selection.MaxUnmatched + 4 {
		tr.ObserveShades(thin, shadesWithMark(3, i%3))
	}
	for _, e := range tr.Evidence() {
		if e.Rows < 3 {
			t.Errorf("a candidate is down to %d member(s) at %v. Fewer than a stack "+
				"cannot show a mark transferring, so what is left is not a list "+
				"and must be retired whole rather than kept", e.Rows, e.Bounds)
		}
	}
}

// A list beside another at the same heights is a different list.
//
// Row correspondence compares centres, and two columns of a screen have the same centres all
// the way down. So it compares widths too, and this is the case that says why.
//
// Deleting the horizontal term from sameRow must fail this.
func TestAListBesideAnotherAtTheSameHeightsIsNotIt(t *testing.T) {
	tr := selection.NewTracker()
	left := rowsAt(0, 5, 100, 100, 600)
	right := rowsAt(0, 5, 900, 100, 600)
	for i := range 12 {
		tr.ObserveShades(left, shadesWithMark(5, i%5))
	}
	for i := range 12 {
		tr.ObserveShades(right, shadesWithMark(5, i%5))
	}
	ev := tr.Evidence()
	if len(ev) != 2 {
		t.Fatalf("%d candidate(s) for two lists at the same heights in different "+
			"columns, want 2.\n%v", len(ev), ev)
	}
	if ev[0].Bounds.Min.X == ev[1].Bounds.Min.X {
		t.Errorf("both candidates are at x %d, so they are not the two columns",
			ev[0].Bounds.Min.X)
	}
}

// A candidate nothing finds again is retired, and its evidence goes with it.
//
// THE LIFECYCLE HALF OF THE REPAIR. Matching by extent kept the population down by absorbing
// almost everything into a few buckets, which is the defect; matching by rows refuses far more,
// and an experiment with a stricter rule accumulated candidates until the replay slowed several
// times over. So a rule that refuses more has to say what becomes of what it refused.
//
// The evidence going with it is the part the graph depends on: a later list at that place starts
// from nothing and serves its full MinObservations, rather than inheriting counters measured
// from a list that is gone.
//
// Deleting the retirement in Tracker.age must fail this.
func TestACandidateNothingFindsAgainIsRetiredWithItsEvidence(t *testing.T) {
	tr := selection.NewTracker()
	gone := rowsAt(0, 5, 100, 100, 600)
	for i := range 20 {
		tr.ObserveNavigated(gone, shadesWithMark(5, i%5), true)
	}
	if n := len(tr.Evidence()); n != 1 {
		t.Fatalf("%d candidates from one list, want 1", n)
	}
	if !tr.Evidence()[0].Qualifies {
		t.Fatalf("the list did not qualify, so retiring it proves nothing: %s",
			tr.Evidence()[0])
	}
	if tr.Retired() != 0 {
		t.Fatalf("%d candidate(s) retired before anything stopped being found",
			tr.Retired())
	}

	// SOMETHING ELSE ENTIRELY, for longer than a candidate may go unmatched.
	other := rowsAt(0, 4, 900, 1200, 1600)
	for i := range selection.MaxUnmatched + 2 {
		tr.ObserveNavigated(other, shadesWithMark(4, i%4), true)
	}
	if tr.Retired() != 1 {
		t.Errorf("%d candidate(s) retired after one went %d observations without being "+
			"found again, want 1", tr.Retired(), selection.MaxUnmatched+2)
	}
	for _, e := range tr.Evidence() {
		if e.Bounds.Min.X == 100 {
			t.Errorf("the first list is still tracked after %d observations in which "+
				"nothing found it: %s", selection.MaxUnmatched+2, e)
		}
	}

	// AND IT COMES BACK A STRANGER. Its old counters are not waiting for it, and neither
	// is its name: a name derived from how many candidates are currently tracked would be
	// handed out again the moment one retires, and then "the same list as before" is a
	// question two different lists answer yes to.
	named := map[int]bool{}
	for _, e := range tr.Evidence() {
		named[e.ID] = true
	}
	back := rowsAt(0, 5, 100, 100, 600)
	tr.ObserveNavigated(back, shadesWithMark(5, 0), true)
	for _, e := range tr.Evidence() {
		if e.Bounds.Min.X == 100 && named[e.ID] {
			t.Errorf("the list came back named %d, which is a name already in use. A "+
				"retired candidate's name must not be handed out again", e.ID)
		}
	}
	for _, e := range tr.Evidence() {
		if e.Bounds.Min.X != 100 {
			continue
		}
		if e.Frames != 1 {
			t.Errorf("the list came back and reports %d frames, so a retired "+
				"candidate's evidence was waiting to be inherited by the next "+
				"list at that place: %s", e.Frames, e)
		}
		if e.Qualifies {
			t.Errorf("the list came back and qualifies on its first look: %s", e)
		}
	}
}

// A member nothing finds again is forgotten, and the list stops claiming where it was.
//
// A member that is never found again still decides things: its rectangle is part of the extent
// the belief publishes, so callers keep restricting the reader to rows there, and its baseline
// is still consulted. A list that scrolled away from a row would otherwise stay anchored to it.
//
// Deleting the call to forgetColdMembers must fail this.
func TestAMemberNothingFindsAgainIsForgotten(t *testing.T) {
	tr := selection.NewTracker()
	six := rowsAt(0, 6, 100, 100, 700)
	for i := range 12 {
		tr.ObserveShades(six, shadesWithMark(6, i%6))
	}
	was := tr.Evidence()[0]
	if was.Rows != 6 {
		t.Fatalf("the candidate has %d members, want 6", was.Rows)
	}

	// THE TOP ROW AND THE BOTTOM ROW GO AWAY and stay away; the middle four keep being
	// found. Taking one from the TOP is deliberate: forgetting a member renumbers every
	// member after it, so a fixture that only ever drops the last one renumbers nothing
	// and says nothing about the indices that have to follow.
	part := six[1:5]
	// AND THE MARK IS HELD STILL THROUGHOUT, which is what makes the renumbering visible.
	// The forgetting happens on one particular observation inside this stretch, and `last`
	// is a position in the member slice set on the observation before it. Left unmoved, it
	// then names a different row than the one the mark is actually on, and the very next
	// look reads a transfer — exactly ONE of them, which is why a count taken after the
	// stretch cannot see it and this one is taken across it.
	tr.ObserveShades(part, shadesWithMark(4, 1))
	moves := tr.Evidence()[0].Transfers
	for range selection.MaxUnmatched + 4 {
		tr.ObserveShades(part, shadesWithMark(4, 1))
	}
	ev := tr.Evidence()
	if len(ev) != 1 {
		t.Fatalf("%d candidates, want 1: four of six rows is most of this frame's rows "+
			"found again, so it is the same list.\n%v", len(ev), ev)
	}
	if ev[0].Rows != 4 {
		t.Errorf("the candidate still reports %d members after %d observations in which "+
			"two of them were never found, want 4", ev[0].Rows,
			selection.MaxUnmatched+4)
	}
	if ev[0].Bounds.Max.Y >= was.Bounds.Max.Y {
		t.Errorf("the published extent still reaches y %d, where the two forgotten rows "+
			"were; it ended at y %d before they went away. A list that keeps "+
			"claiming ground it no longer occupies hands the reader rows from there",
			ev[0].Bounds.Max.Y, was.Bounds.Max.Y)
	}
	// AND THE INDICES WENT WITH THEM. `last` and `holders` are positions in the member
	// slice, so forgetting a member renumbers everything after it. Left alone, this
	// frame's member is compared against a different row than the one the mark was last
	// seen in — and the mark is still where it was, so what comes out is a transfer
	// nothing moved: a false transition, offered to the graph.
	if ev[0].Holders > ev[0].Rows {
		t.Errorf("%d distinct members have held the mark in a list with %d members, so "+
			"the holders are positions in a slice that has been renumbered under "+
			"them", ev[0].Holders, ev[0].Rows)
	}
	if got := ev[0].Transfers; got != moves {
		t.Errorf("%d transfer(s) were recorded, up from %d, with the mark held still on "+
			"one row across the observation that forgot two members. A transfer "+
			"nothing moved is what a stale member index produces, and it is offered "+
			"to the graph as a move between rows the mark never made", got, moves)
	}
}

// A candidate is named, and the name is never reused.
//
// Callers were deriving "is this still the list I was looking at" from the published extent, and
// an extent is not an identity: a list whose rows are re-found a pixel lower publishes a
// different one. Measured on 864 real frames, that comparison retained ONE move out of 31
// presses. See observe.TestAListThatDriftedIsStillTheSameList.
//
// Deleting the id assignment in Tracker.update must fail this.
func TestACandidateIsNamedAndTheNameIsNotReused(t *testing.T) {
	tr := selection.NewTracker()
	one := rowsAt(0, 5, 100, 100, 600)
	for i := range 12 {
		tr.ObserveShades(one, shadesWithMark(5, i%5))
	}
	id := tr.Evidence()[0].ID
	if id == 0 {
		t.Fatal("a tracked candidate has no name")
	}
	// A DRIFT of less than half a row: the same list, found a little lower.
	drift := rowsAt(0, 5, 100, 120, 620)
	for i := range 6 {
		tr.ObserveShades(drift, shadesWithMark(5, i%5))
	}
	ev := tr.Evidence()
	if len(ev) != 1 {
		t.Fatalf("%d candidates after a drift of a fifth of a row, want 1.\n%v",
			len(ev), ev)
	}
	if ev[0].ID != id {
		t.Errorf("the list was named %d and is now %d after drifting a fifth of a row. "+
			"Its rows are all still the same rows, so it is the same list",
			id, ev[0].ID)
	}
	if ev[0].Bounds == tr.Evidence()[0].Bounds && ev[0].Bounds.Min.Y == 100 {
		t.Errorf("the extent did not move with the rows, so this says nothing about an " +
			"identity surviving an extent that moves")
	}

	// AND A SECOND LIST GETS A SECOND NAME.
	other := rowsAt(0, 5, 900, 100, 600)
	for i := range 12 {
		tr.ObserveShades(other, shadesWithMark(5, i%5))
	}
	names := map[int]bool{}
	for _, e := range tr.Evidence() {
		if names[e.ID] {
			t.Errorf("two candidates share the name %d", e.ID)
		}
		names[e.ID] = true
	}
	if len(names) != 2 {
		t.Errorf("%d distinct names for 2 candidates", len(names))
	}
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
