package selection_test

import (
	"image"
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
