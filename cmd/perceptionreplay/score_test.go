package main

import "testing"

// The scorer, on a trace small enough to read.
//
// The real corpus holds the floors, and it cannot hold these: every assertion there is about a
// number that came out of 864 frames of somebody's game, so a counter that stops firing
// altogether reads as an improvement. A case the counter must fire on has to be written down.

// scoreAnn is a nine-row labelled menu, evenly spaced, with one interval per row.
//
// The spacing matters: the tolerance everything here is scored at is half the LABELLED pitch,
// so a fixture with an irregular pitch would be testing the arithmetic of labelledPitch rather
// than the scoring.
func scoreAnn() *annotationFile {
	a := &annotationFile{Session: "fixture"}
	for i := range 9 {
		y := 0.100 + float64(i)*0.050
		a.Intervals = append(a.Intervals, annotationInterval{
			From: i * 10, To: i*10 + 9, State: "selected", Y: y,
		})
	}
	return a
}

// A naming scored against a region as tall as its list is reported as such.
//
// THE MEASUREMENT TRAP, as a test rather than as a comment. A previous version of this score
// was handed the identified LIST's envelope instead of the named entry's region; the list
// spanned three quarters of the screen, so every labelled row on the frame was inside it and
// the measurement read 154 of 155 for a reader that was mostly wrong.
//
// The corpus test asserts the counter is zero, which a deleted counter also satisfies. This is
// the other half: a trace that SHOULD trip it.
func TestANamingAsTallAsItsListIsReported(t *testing.T) {
	ann := scoreAnn()
	// One frame, naming a "row" that is the whole list: top 0.075, height 0.450.
	s := scoreSelection([]frameObservation{{
		Index: 0, Status: "provisional", Selected: 0,
		ListTop: 0.075, ListBottom: 0.525,
		NamedY: 0.100, NamedHeight: 0.450,
	}}, ann)
	if s.Named != 1 {
		t.Fatalf("the fixture produced %d naming(s), want 1; nothing below was scored",
			s.Named)
	}
	if s.NamedAsTallAsList != 1 {
		t.Errorf("a naming scored against a region 0.450 tall inside a list 0.450 tall "+
			"was not reported (%d). An entry is not half a list, and a score "+
			"computed against the envelope is how a reader that cannot tell "+
			"neighbours apart reads as 99%% accurate", s.NamedAsTallAsList)
	}
	// And an entry-sized region in the same list does not trip it.
	ok := scoreSelection([]frameObservation{{
		Index: 0, Status: "provisional", Selected: 0,
		ListTop: 0.075, ListBottom: 0.525,
		NamedY: 0.100, NamedHeight: 0.040,
	}}, ann)
	if ok.NamedAsTallAsList != 0 {
		t.Errorf("an entry-sized region tripped the check (%d), so it is not a check "+
			"about envelopes", ok.NamedAsTallAsList)
	}
	if ok.Right != 1 {
		t.Errorf("an entry-sized region on the labelled row scored %d of 1 right",
			ok.Right)
	}
}

// Coverage counts the labelled frames and nothing else.
//
// The annotation marks the blend frames `uncertain` on purpose, and those are the frames a
// reader is expected to refuse. Counting one as missed coverage charges the reader for its
// correct answer; counting one as a naming credits it for a row the labels do not claim.
func TestOnlyLabelledFramesAreScored(t *testing.T) {
	ann := scoreAnn()
	ann.Intervals = append(ann.Intervals, annotationInterval{
		From: 500, To: 509, State: "uncertain",
	})
	trace := []frameObservation{
		{Index: 0, Status: "provisional", Selected: 0,
			ListTop: 0.075, ListBottom: 0.525, NamedY: 0.100, NamedHeight: 0.040},
		// An unlabelled frame, past every interval.
		{Index: 900, Status: "provisional", Selected: 0,
			ListTop: 0.075, ListBottom: 0.525, NamedY: 0.100, NamedHeight: 0.040},
		// And an `uncertain` one, which is labelled and not `selected`.
		{Index: 505, Status: "provisional", Selected: 0,
			ListTop: 0.075, ListBottom: 0.525, NamedY: 0.100, NamedHeight: 0.040},
	}
	s := scoreSelection(trace, ann)
	if s.Labelled != 1 {
		t.Errorf("%d of 3 frames were scored; one carries a `selected` label, one "+
			"carries none and one is `uncertain`", s.Labelled)
	}
	if s.Named != 1 {
		t.Errorf("%d naming(s) were counted against one labelled frame", s.Named)
	}
}

// A silent frame is attributed to the half of coverage it belongs to.
//
// The two halves call for opposite work: no list identified is disambiguation, a list with no
// readable row is the reader. One counter for both would make them interchangeable, which is
// the mistake the whole file exists to undo one level up.
func TestASilentFrameSaysWhichHalfOfCoverageItIsIn(t *testing.T) {
	ann := scoreAnn()
	s := scoreSelection([]frameObservation{
		{Index: 0, Status: "provisional", Selected: -1},
		{Index: 10, Status: "ambiguous", Selected: -1},
		{Index: 20, Status: "unknown", Selected: -1},
	}, ann)
	if s.Named != 0 {
		t.Fatalf("%d naming(s) from three frames that named nothing", s.Named)
	}
	if s.Unread != 1 {
		t.Errorf("%d frame(s) are recorded as a list with no readable row; one of the "+
			"three had an identified list", s.Unread)
	}
	if s.NoList != 2 {
		t.Errorf("%d frame(s) are recorded as having no list; two of the three were "+
			"ambiguous or unknown", s.NoList)
	}
	if got := s.Named + s.NoList + s.Unread; got != s.Labelled {
		t.Errorf("three labelled frames split into %d; a frame in none of the three "+
			"has been lost", got)
	}
}

// A neighbour is a different failure from a stranger.
//
// Half the labelled pitch is exactly the distinction a navigation route is made of, so a
// reading one row out and a reading five rows out are not the same defect: one is a reader that
// cannot separate neighbours and the other is the wrong list entirely. The signed histogram is
// what tells them apart, and a spike in it is the tell.
func TestAWrongNamingIsPartitionedByHowWrongItIs(t *testing.T) {
	ann := scoreAnn()
	// Pitch is 0.050. Frame 0's labelled row is 0.100.
	s := scoreSelection([]frameObservation{
		// one row below: 0.150
		{Index: 0, Status: "provisional", Selected: 1,
			ListTop: 0.075, ListBottom: 0.525, NamedY: 0.150, NamedHeight: 0.040},
		// three rows below: 0.250
		{Index: 1, Status: "provisional", Selected: 3,
			ListTop: 0.075, ListBottom: 0.525, NamedY: 0.250, NamedHeight: 0.040},
		// and one outside the list being read entirely
		{Index: 2, Status: "provisional", Selected: 0,
			ListTop: 0.600, ListBottom: 0.800, NamedY: 0.620, NamedHeight: 0.040},
	}, ann)
	if s.Named != 3 || s.Right != 0 {
		t.Fatalf("the fixture scored %d naming(s), %d right; want 3 and 0",
			s.Named, s.Right)
	}
	if s.OffByOne != 1 {
		t.Errorf("%d naming(s) are one row out; one of the three is", s.OffByOne)
	}
	if s.OffByMore != 2 {
		t.Errorf("%d naming(s) are more than one row out; two of the three are",
			s.OffByMore)
	}
	if s.OutsideList != 1 {
		t.Errorf("%d naming(s) had the labelled row outside the list being read; one "+
			"of the three did, and no reading of that list could have been right",
			s.OutsideList)
	}
	// THE READER'S OWN SCORE excludes the one it could not have got right.
	if s.InsideNamed != 2 {
		t.Errorf("the reader's own score is computed over %d naming(s); two of the "+
			"three were made with the labelled row inside the list", s.InsideNamed)
	}
	if s.Offsets[1] != 1 || s.Offsets[3] != 1 {
		t.Errorf("the signed histogram reads %v; want one naming at +1 and one at +3. "+
			"A spread around zero is a reader that cannot tell neighbours apart "+
			"and a spike elsewhere is the wrong list, and the histogram is what "+
			"separates them", s.Offsets)
	}
}

// The confirmation lag is measured from the frame the labels put the new row in place.
//
// Not from a press, which is what the other latency measurement does. A press is not when the
// screen changed: press-to-belief is the game's repaint plus Marco's confirmation delay, and it
// cannot separate them. The annotation can, so this measures Marco's half alone.
func TestTheConfirmationLagIsMeasuredFromTheLabelledChange(t *testing.T) {
	ann := scoreAnn()
	// Row 0 is labelled over frames 0..9 and row 1 over 10..19. The belief names row 0
	// throughout, then agrees with row 1 at frame 13.
	var trace []frameObservation
	for i := range 20 {
		y := 0.100
		if i >= 13 {
			y = 0.150
		}
		trace = append(trace, frameObservation{
			Index: i, AtMS: int64(i) * 100, Status: "provisional", Selected: 0,
			ListTop: 0.075, ListBottom: 0.525, NamedY: y, NamedHeight: 0.040,
		})
	}
	confs := confirmations(trace, ann)
	if len(confs) == 0 {
		t.Fatal("no labelled change of row was found, so nothing was measured")
	}
	c := confs[0]
	if c.From != 0 || c.To != 1 {
		t.Errorf("the first change is %d->%d, want 0->1", c.From, c.To)
	}
	if c.Never {
		t.Fatal("the change was recorded as never named, though frame 13 named it")
	}
	if c.SeenAt != 13 {
		t.Errorf("the belief is recorded as agreeing at frame %d, want 13", c.SeenAt)
	}
	if c.MS != 300 {
		t.Errorf("the lag is %dms; frames 10 and 13 are 300ms apart on this fixture's "+
			"clock", c.MS)
	}
	if c.Wrong != 3 {
		t.Errorf("%d frame(s) in the gap named some other row; frames 10, 11 and 12 "+
			"named row 0 while the labels had moved to row 1. A gap spent naming "+
			"the row the selection has left is a wrong answer rather than a slow "+
			"one, and the graph cannot tell them apart from the moves alone",
			c.Wrong)
	}
}
