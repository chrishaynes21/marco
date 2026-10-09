package selection_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// Everything here is drawn, not photographed.
//
// Deliberately: a reader tested only on the recording it was built from learns that
// recording. These rows are grey rectangles, and the properties being checked — the odd one
// out, a row judged against its own past, a flicker refused — are the ones that have to hold
// whatever an interface looks like.

const (
	rowH = 20
	sepH = 2
	colW = 200
)

// draw renders a stack of rows at the given levels, separated by rules.
func draw(levels []int) *image.RGBA {
	h := len(levels)*(rowH+sepH) + sepH
	img := image.NewRGBA(image.Rect(0, 0, colW, h))
	fill(img, img.Bounds(), 10)
	for i, l := range levels {
		top := sepH + i*(rowH+sepH)
		fill(img, image.Rect(0, top, colW, top+rowH), l)
	}
	return img
}

func fill(img *image.RGBA, r image.Rectangle, level int) {
	c := color.RGBA{uint8(level), uint8(level), uint8(level), 255}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

// rowsFor builds the candidate rows matching draw's geometry, all in one group.
func rowsFor(n int) []selection.Row {
	out := make([]selection.Row, n)
	for i := range out {
		top := sepH + i*(rowH+sepH)
		out[i] = selection.Row{Bounds: image.Rect(0, top, colW, top+rowH)}
	}
	return out
}

// settle reads the same picture repeatedly, which is what a held selection looks like.
func settle(t *testing.T, r *selection.Reader, levels []int, n int) selection.Result {
	t.Helper()
	img := draw(levels)
	rows := rowsFor(len(levels))
	var last selection.Result
	for range n {
		last = r.Read(img, rows)
	}
	return last
}

func TestTheOddRowOutIsTheChosenOne(t *testing.T) {
	r := selection.New()
	got := settle(t, r, []int{60, 60, 220, 60, 60}, 3)
	if got.Outcome != selection.Selected || got.Row != 2 {
		t.Fatalf("the row unlike its siblings is the chosen one, got %v row %d (%s)",
			got.Outcome, got.Row, got.Why)
	}
}

func TestADarkHighlightReadsTheSameAsABrightOne(t *testing.T) {
	// The measured games both mark a choice by making it brighter. Nothing here may
	// depend on that: a menu whose chosen row is a dark fill is just as ordinary, and a
	// reader that only looks upwards would report nothing on every frame of it.
	r := selection.New()
	got := settle(t, r, []int{200, 200, 30, 200, 200}, 3)
	if got.Outcome != selection.Selected || got.Row != 2 {
		t.Fatalf("a dark chosen row is still the odd one out, got %v row %d (%s)",
			got.Outcome, got.Row, got.Why)
	}
}

func TestARowWithNoSiblingsIsJudgedAgainstItself(t *testing.T) {
	// This is EXIT's situation, and the reason the temporal half exists. A row alone in
	// its group has nothing to be unlike, so the only evidence is that it is no longer
	// what it was.
	r := selection.New()
	rows := rowsFor(4)
	for i := range rows[:3] {
		rows[i].Group = 0
	}
	rows[3].Group = 1 // set apart, as a row below a gap would be

	quiet := draw([]int{60, 60, 60, 90})
	for range 6 {
		if got := r.Read(quiet, rows); got.Outcome == selection.Selected {
			t.Fatalf("nothing is chosen here, got row %d (%s)", got.Row, got.Why)
		}
	}
	// Now the lone row brightens and nothing else moves.
	lit := draw([]int{60, 60, 60, 150})
	var got selection.Result
	for range 3 {
		got = r.Read(lit, rows)
	}
	if got.Outcome != selection.Selected || got.Row != 3 {
		t.Fatalf("the lone row moved away from its own level, got %v row %d (%s)",
			got.Outcome, got.Row, got.Why)
	}
}

func TestASelectionDrawnOnlyOnTheTextIsStillFound(t *testing.T) {
	// The measured failure that earned the second feature. EXIT is chosen by its TEXT
	// getting brighter, and text is a small part of a wide row: a change obvious to a
	// person moves the row's mean by two or three units out of two hundred. A reader
	// describing a row by its average alone misses it on every frame, and — worse — keeps
	// confidently claiming whichever entry was chosen before.
	//
	// Here a tenth of the row brightens and the rest does not, so the mean barely moves
	// and the brightest quarter moves a lot.
	// The rows JITTER from frame to frame, and that is the point. Perfectly flat rows make
	// the spread floor do all the work, so a two-unit change in a row's average looks
	// enormous and the mean alone appears sufficient — which is exactly how an earlier
	// version of this test passed against a reader that had the second feature removed. A
	// real frame has a moving background behind a translucent panel, so a row's average
	// wanders by more than a text change moves it.
	r := selection.New()
	rows := rowsFor(4)
	rows[3].Group = 1

	frame := func(tick int, lit bool) *image.RGBA {
		base := []int{60, 60, 60, 60}
		for i := range base {
			base[i] += ((tick*7+i*5)%13 - 6) // deterministic wander, about +-6
		}
		img := draw(base)
		if lit {
			band := rows[3].Bounds
			fill(img, image.Rect(band.Min.X, band.Min.Y+6, band.Min.X+colW/10, band.Max.Y-6), 250)
		}
		return img
	}

	for tick := range 10 {
		r.Read(frame(tick, false), rows)
	}
	var got selection.Result
	for tick := 10; tick < 14; tick++ {
		got = r.Read(frame(tick, true), rows)
	}
	if got.Outcome != selection.Selected || got.Row != 3 {
		t.Fatalf("a selection drawn only on the text is still a selection, got %v row %d (%s)",
			got.Outcome, got.Row, got.Why)
	}
}

func TestASelectionSurvivesBeingHeld(t *testing.T) {
	// The failure this guards: measuring a row against its own RECENT average means a
	// held selection stops looking different from itself, and the reader quietly drops a
	// selection that never went anywhere. Forty frames is under three seconds.
	r := selection.New()
	rows := rowsFor(4)
	rows[3].Group = 1
	for range 6 {
		r.Read(draw([]int{60, 60, 60, 90}), rows)
	}
	lit := draw([]int{60, 60, 60, 150})
	var got selection.Result
	for range 40 {
		got = r.Read(lit, rows)
	}
	if got.Outcome != selection.Selected || got.Row != 3 {
		t.Fatalf("after 40 held frames the selection is still there, got %v row %d (%s)",
			got.Outcome, got.Row, got.Why)
	}
}

func TestAOneFrameFlickerIsNotASelection(t *testing.T) {
	// Measured in the real recording: the bar appeared on a neighbour for a single frame
	// and returned, with no input anywhere near it, and a per-frame rule scored it a
	// confident selection at a margin higher than many true ones.
	r := selection.New()
	rows := rowsFor(5)
	stable := draw([]int{60, 220, 60, 60, 60})
	for range 5 {
		r.Read(stable, rows)
	}
	flick := r.Read(draw([]int{60, 60, 220, 60, 60}), rows)
	if flick.Outcome == selection.Selected && flick.Row == 2 {
		t.Fatal("a single frame must not move the selection")
	}
	after := r.Read(stable, rows)
	if after.Outcome != selection.Selected || after.Row != 1 {
		t.Fatalf("the selection never left row 1, got %v row %d (%s)",
			after.Outcome, after.Row, after.Why)
	}
}

func TestAConfirmedChangeIsAccepted(t *testing.T) {
	// The other side of the previous test: refusing one frame must not mean refusing
	// change. A real move is asserted on the second frame that agrees.
	r := selection.New()
	rows := rowsFor(5)
	for range 5 {
		r.Read(draw([]int{60, 220, 60, 60, 60}), rows)
	}
	moved := draw([]int{60, 60, 220, 60, 60})
	first := r.Read(moved, rows)
	if first.Outcome == selection.Selected && first.Row == 2 {
		t.Fatal("one frame is not yet a confirmed change")
	}
	second := r.Read(moved, rows)
	if second.Outcome != selection.Selected || second.Row != 2 {
		t.Fatalf("a change confirmed by a second frame is accepted, got %v row %d (%s)",
			second.Outcome, second.Row, second.Why)
	}
}

func TestTwoRowsDeviatingTogetherIsUnreadable(t *testing.T) {
	// What a blend looks like from here: the departing entry still carries something and
	// the arriving one carries something, and neither claim is safe.
	r := selection.New()
	got := settle(t, r, []int{60, 220, 215, 60, 60}, 3)
	if got.Outcome == selection.Selected {
		t.Fatalf("two rows deviating together is not a selection, got row %d (%s)",
			got.Row, got.Why)
	}
}

func TestNoRecognisedCueIsNotAFindingThatNothingIsChosen(t *testing.T) {
	// The distinction the whole package turns on. A reader that has not recognised a cue
	// has learned nothing about whether something is chosen, and must not dress that up
	// as a finding. [[ADR-006-unknown-is-not-false]].
	r := selection.New()
	got := settle(t, r, []int{60, 60, 60, 60, 60}, 5)
	if got.Outcome == selection.NoneSelected {
		t.Fatal("an unrecognised cue is not evidence that nothing is chosen")
	}
	if got.Outcome != selection.Insufficient {
		t.Fatalf("want insufficient, got %v (%s)", got.Outcome, got.Why)
	}
}

func TestHoldingAPreviousSelectionIsBounded(t *testing.T) {
	// Temporal context, without carrying the old answer forever.
	r := selection.New()
	rows := rowsFor(5)
	for range 5 {
		r.Read(draw([]int{60, 220, 60, 60, 60}), rows)
	}
	flat := draw([]int{60, 60, 60, 60, 60})
	held := r.Read(flat, rows)
	if held.Outcome != selection.Selected || held.Row != 1 {
		t.Fatalf("one unreadable frame should not lose a settled selection, got %v (%s)",
			held.Outcome, held.Why)
	}
	// A FIXED number of frames, deliberately not derived from HoldFrames.
	//
	// An earlier version looped HoldFrames+2 times, so raising the constant also moved the
	// test's own goalposts: the bound could be inflated to a hundred thousand and this
	// still passed. A test that takes its threshold from the thing it is testing is not a
	// test. Thirty frames is two seconds at the measured capture rate, which is far longer
	// than any claim about a screen should survive without re-reading it.
	var last selection.Result
	for range 30 {
		last = r.Read(flat, rows)
	}
	if last.Outcome == selection.Selected {
		t.Fatalf("the old selection was carried too long, got row %d (%s)", last.Row, last.Why)
	}
	if selection.HoldFrames > 20 {
		t.Errorf("HoldFrames = %d; holding a reading for that long stops being temporal "+
			"context and becomes a claim about a screen nobody has looked at",
			selection.HoldFrames)
	}
}

func TestFindRowsFindsAStackWithoutBeingToldItsShape(t *testing.T) {
	img := draw([]int{60, 60, 220, 60, 60})
	rows := selection.FindRows(img, img.Bounds())
	if len(rows) < 4 {
		t.Fatalf("want about five bands, got %d", len(rows))
	}
	// Evenly spaced bands of equal height are siblings, so one group.
	for _, r := range rows {
		if r.Group != rows[0].Group {
			t.Fatalf("an even stack should be one group, got %d and %d",
				rows[0].Group, r.Group)
		}
	}
}

func TestADetachedRowIsNotTreatedAsASibling(t *testing.T) {
	// A row set off from the stack by a larger gap — which is exactly how the measured
	// menu draws its last entry — has no siblings, and saying so is what sends it down
	// the temporal path instead of being compared with rows it does not resemble.
	img := image.NewRGBA(image.Rect(0, 0, colW, 200))
	fill(img, img.Bounds(), 10)
	for i := range 4 {
		top := 2 + i*(rowH+sepH)
		fill(img, image.Rect(0, top, colW, top+rowH), 60)
	}
	fill(img, image.Rect(0, 150, colW, 150+rowH), 60) // detached, after a wide gap

	rows := selection.FindRows(img, img.Bounds())
	if len(rows) < 4 {
		t.Skipf("the band finder saw %d bands; grouping is only meaningful above four", len(rows))
	}
	groups := map[int]bool{}
	for _, r := range rows {
		groups[r.Group] = true
	}
	if len(groups) < 2 {
		t.Errorf("a row after a wide gap should start a new group, got %d group(s)", len(groups))
	}
}
