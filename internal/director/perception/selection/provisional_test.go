package selection_test

import (
	"image"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// What a provisional sighting is allowed to be, and what it must never become.
//
// A change of winner is not published until a second frame agrees, which refuses every
// one-frame blend and is correct. The cost is not in the answer but in the TIME the answer
// carries: a consumer that records when it was told has recorded the confirmation. So the
// reader now says which row THIS frame pointed at, beside an answer that still says nothing.
//
// Both halves of that have to be held by a test. The field must carry the sighting, and the
// sighting must never reach the published reading — because a blend names the row the
// selection is LEAVING, so a provisional row is wrong about as often as it is right.

// shadeRows builds a frame-sized set of rows and the shades to go with them.
//
// Five rows, differing from each other: a group of identical rows has almost no spread, and
// the measured consequence is recorded in the reader's MinSpread comment.
func shadeRows(mark int) ([]selection.Row, []selection.Shade) {
	rows := make([]selection.Row, 5)
	shades := make([]selection.Shade, 5)
	for i := range rows {
		rows[i] = selection.Row{
			Bounds: image.Rect(10, 10+i*40, 210, 44+i*40),
			Group:  1,
		}
		b := 50 + float64(i)*9
		shades[i] = selection.Shade{b, b + 30}
		if i == mark {
			shades[i] = selection.Shade{b + 150, b + 180}
		}
	}
	return rows, shades
}

// read is one frame through the reader.
func lookAt(r *selection.Reader, mark int) selection.Result {
	rows, shades := shadeRows(mark)
	return r.ReadShades(rows, shades)
}

// settle gives every row enough unselected observations for its own level to mean anything.
func quieten(r *selection.Reader) {
	for range 14 {
		lookAt(r, 0)
	}
}

// A provisional sighting never becomes a reading.
//
// THE SAFETY PROPERTY, stated over a whole walk rather than one frame: on every frame, either
// the reader published a row or it named a provisional one, never both. A frame that published
// a row has nothing provisional about it, and a frame that named a provisional row published
// nothing — which is what makes the field impossible to regress coverage or accuracy with,
// because the published fields are computed exactly as they were before it existed.
func TestAProvisionalSightingNeverBecomesAReading(t *testing.T) {
	r := selection.New()
	quieten(r)

	both, provisional, published := 0, 0, 0
	mark := 0
	for range 40 {
		mark = (mark + 1) % 5
		for range 3 {
			res := lookAt(r, mark)
			switch {
			case res.Provisional >= 0 && res.Outcome == selection.Selected:
				both++
			case res.Provisional >= 0:
				provisional++
				if res.Row != -1 {
					t.Errorf("a frame named provisional row %d and also reported "+
						"row %d. A provisional sighting is not a reading",
						res.Provisional, res.Row)
				}
				if res.ProvisionalN <= 0 {
					t.Errorf("provisional row %d is reported with %d frame(s) of "+
						"agreement; a sighting nothing has agreed with is not a "+
						"sighting", res.Provisional, res.ProvisionalN)
				}
			case res.Outcome == selection.Selected:
				published++
			}
		}
	}
	if both > 0 {
		t.Errorf("%d frame(s) published a row AND named a provisional one. The published "+
			"reading is what every existing consumer sees, and a frame that does both "+
			"has made a provisional sighting visible as a selection", both)
	}
	if provisional == 0 {
		t.Fatal("no frame named a provisional row across forty moves, so this test " +
			"asserts nothing. Either the field is never set or the walk never moved")
	}
	if published == 0 {
		t.Fatal("no frame published a row, so the walk was never read at all and the " +
			"separation above is between two empty sets")
	}
	t.Logf("%d provisional sighting(s), %d published reading(s), %d frames doing both",
		provisional, published, both)
}

// A provisional row that is never confirmed is never published.
//
// The one-frame artefact, as its own case: a row is pointed at for a single frame and the mark
// goes back where it was. ConfirmFrames exists for exactly this — the measured blends all last
// one frame, and one of them was a bar flickering to a neighbour and back with no input near
// it. The sighting is REPORTED, because a consumer may want to know the pixels wobbled, and it
// is never asserted.
func TestAOneFrameSightingIsReportedAndNeverAsserted(t *testing.T) {
	r := selection.New()
	quieten(r)
	// Settle a real selection first, so there is something for the flicker to depart from.
	for range 4 {
		lookAt(r, 2)
	}
	if res := lookAt(r, 2); res.Outcome != selection.Selected || res.Row != 2 {
		t.Fatalf("row 2 was never settled, so a flicker away from it proves nothing: "+
			"%s row %d (%s)", res.Outcome, res.Row, res.Why)
	}

	flick := lookAt(r, 3)
	if flick.Outcome == selection.Selected && flick.Row == 3 {
		t.Fatal("one frame pointing at row 3 published row 3. Every measured transition " +
			"blends for exactly one frame and the blend names the wrong row")
	}
	back := lookAt(r, 2)
	if back.Outcome != selection.Selected || back.Row != 2 {
		t.Errorf("after a one-frame flicker the reader says %s row %d (%s); the mark "+
			"never left row 2", back.Outcome, back.Row, back.Why)
	}
	if back.Provisional >= 0 {
		t.Errorf("the frame that returned to row 2 still names provisional row %d. "+
			"The row being published is not provisional", back.Provisional)
	}
}

// A different row resets the sighting, so a blend cannot lend its moment to what follows.
//
// THE ARITHMETIC THE SAFE USE RESTS ON. A consumer keeps the moment a provisional row APPEARED
// and reads it only once that same row is published. That is only safe if a blend — which
// names a different row — cannot pass its moment along: so a new provisional row must report
// its own first frame of agreement rather than inheriting the count from the row before it.
func TestABlendDoesNotLendItsMomentToTheRowThatFollowsIt(t *testing.T) {
	r := selection.New()
	quieten(r)
	for range 4 {
		lookAt(r, 0)
	}

	// One frame on row 3 — the departing-row blend — then the real arrival on row 1.
	first := lookAt(r, 3)
	if first.Provisional != 3 {
		t.Fatalf("the blend frame names provisional row %d, want 3; without it there is "+
			"no moment for the next row to inherit and this test asserts nothing (%s)",
			first.Provisional, first.Why)
	}
	second := lookAt(r, 1)
	if second.Provisional != 1 {
		t.Fatalf("after the blend, a frame pointing at row 1 names provisional row %d",
			second.Provisional)
	}
	if second.ProvisionalN != 1 {
		t.Errorf("row 1's first sighting reports %d frame(s) of agreement, want 1. A new "+
			"row inheriting the previous row's count would date its arrival from the "+
			"blend — which named a different row — and a consumer keeping the moment "+
			"would credit the move to whatever was happening then",
			second.ProvisionalN)
	}
}

// Nothing provisional is reported on a frame that could not be read at all.
//
// Because Provisional is an INDEX, and the zero value of an index is a valid row. A frame that
// says nothing must say nothing in this field too, or every unreadable frame would claim row 0
// was settling.
func TestAnUnreadableFrameNamesNoProvisionalRow(t *testing.T) {
	r := selection.New()
	if res := r.ReadShades(nil, nil); res.Provisional != -1 {
		t.Errorf("a frame with no candidate rows names provisional row %d, want -1. "+
			"Row 0 is a real row with a real entry on it", res.Provisional)
	}
	// And a frame where every row looks alike, which is the ordinary unreadable frame.
	rows := make([]selection.Row, 5)
	shades := make([]selection.Shade, 5)
	for i := range rows {
		rows[i] = selection.Row{Bounds: image.Rect(10, 10+i*40, 210, 44+i*40), Group: 1}
		shades[i] = selection.Shade{80, 110}
	}
	res := r.ReadShades(rows, shades)
	if res.Outcome == selection.Selected {
		t.Fatalf("five identical rows read as %s row %d, so this frame is not the "+
			"unreadable one it is meant to be", res.Outcome, res.Row)
	}
	if res.Provisional != -1 {
		t.Errorf("an unreadable frame names provisional row %d, want -1", res.Provisional)
	}
}

// rowsNamed builds `n` rows with entry identities, with one of them marked.
//
// `drop` removes the row at that position, which is what a detector does when it misses one.
// The identities of the rows that remain do not change, which is the whole point.
func rowsNamed(n, mark, drop int) ([]selection.Row, []selection.Shade) {
	var rows []selection.Row
	var shades []selection.Shade
	for i := range n {
		if i == drop {
			continue
		}
		base := 50 + float64(i)*9
		sh := selection.Shade{base, base + 30}
		if i == mark {
			sh = selection.Shade{base + 150, base + 180}
		}
		rows = append(rows, selection.Row{
			Bounds: image.Rect(100, 100+i*120, 400, 100+i*120+80),
			Group:  0, ID: i + 1,
		})
		shades = append(shades, sh)
	}
	return rows, shades
}

// A reader told nothing of entries still drops its history when the row count changes.
//
// THE OLD CONTRACT, KEPT EXACTLY WHERE NOTHING CAN NAME A ROW. A reader whose rows carry
// identities files each row's history under its own name, so a frame in which some other row
// was missing does not disturb it. A reader handed rows with no identities has only their
// positions to file under, and a position does not survive a count change — the rows below the
// one that went missing are all at different indices, and comparing them against a history
// measured at those indices is comparing them against other rows.
//
// `cmd/director selection-read` is that caller: it reads one frame of a screen with no tracker
// behind it, so there is nothing to name the rows. [[ADR-165-an-entry-is-the-same-entry-when-nothing-says-otherwise]].
//
// Keeping the history across a count change in the unnamed case must fail this.
func TestAReaderToldNothingOfEntriesStillDropsItsHistoryOnACountChange(t *testing.T) {
	r := selection.New()
	// Five unnamed rows, with the mark settled on the third.
	plain := func(n, mark int) ([]selection.Row, []selection.Shade) {
		rows, shades := rowsNamed(n, mark, -1)
		for i := range rows {
			rows[i].ID = 0
		}
		return rows, shades
	}
	for range 14 {
		rows, shades := plain(5, 0)
		r.ReadShades(rows, shades)
	}
	var settled selection.Result
	for range 4 {
		rows, shades := plain(5, 2)
		settled = r.ReadShades(rows, shades)
	}
	if settled.Outcome != selection.Selected {
		t.Fatalf("nothing was settled on five unnamed rows, so a count change proves "+
			"nothing: %s (%s)", settled.Outcome, settled.Why)
	}
	if settled.Entry != 0 {
		t.Errorf("the reading names entry %d on rows that carry no identities",
			settled.Entry)
	}

	// FOUR ROWS NOW. Nothing says which four, so nothing the reader knows applies.
	rows, shades := plain(4, 2)
	got := r.ReadShades(rows, shades)
	if got.Outcome == selection.Selected && got.Why == settled.Why {
		t.Errorf("the reader answered %q on four unnamed rows with a history measured "+
			"on five. Without an identity there is nothing to say the third of four "+
			"is the third of five", got.Why)
	}
}

// A claim about an entry that has gone is not pointed at a row.
//
// The reader holds a settled claim across a look it could not read, because a detector blinks
// and an unreadable frame is not evidence that the selection moved. The claim is an ENTRY, and
// holding it means pointing a caller at the row that entry is — so when that entry is not among
// this frame's rows there is no row to point at. Pointing at the position it used to occupy
// would name whichever row has moved into that slot, which is the renumbering this whole
// decision exists to stop.
//
// Deleting the `held < 0` branch in settle must fail this.
func TestAClaimAboutAnEntryThatHasGoneIsNotPointedAtARow(t *testing.T) {
	r := selection.New()
	for range 14 {
		rows, shades := rowsNamed(5, 0, -1)
		r.ReadShades(rows, shades)
	}
	var settled selection.Result
	for range 4 {
		rows, shades := rowsNamed(5, 4, -1)
		settled = r.ReadShades(rows, shades)
	}
	if settled.Outcome != selection.Selected || settled.Entry != 5 {
		t.Fatalf("entry 5 was never settled, so its going missing proves nothing: "+
			"%s entry %d (%s)", settled.Outcome, settled.Entry, settled.Why)
	}

	// THE ENTRY IT IS CLAIMING GOES MISSING, and nothing is marked among what is left.
	rows, shades := rowsNamed(5, -1, 4)
	got := r.ReadShades(rows, shades)
	if got.Outcome == selection.Selected {
		t.Errorf("the reader is still claiming a row (%d, entry %d) after the entry it "+
			"had settled on stopped being among the rows it was given: %s",
			got.Row, got.Entry, got.Why)
	}
	if got.Row >= 0 {
		t.Errorf("the reader points at row %d with no entry to point at", got.Row)
	}
	// AND IT SAYS WHICH REFUSAL THIS IS. "Unreadable for too long" and "the entry is not
	// here any more" are the same outcome and different facts, and the second is the one
	// a person chasing a lost reading needs: the frame was perfectly readable and the row
	// it was about has gone.
	if !strings.Contains(got.Why, "not among this frame's rows") {
		t.Errorf("the refusal reads %q, which does not say that the entry it was "+
			"claiming is no longer among the rows it was given", got.Why)
	}
}
