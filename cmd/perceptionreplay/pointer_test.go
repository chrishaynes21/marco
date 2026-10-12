package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A MOUSE DEMONSTRATION IS A CONTRACT ACROSS TWO MAINS, AND IT WAS BROKEN AT BOTH ENDS.
//
// `director capture-game-session` writes input.jsonl; this program reads it back and hands the
// events to the same input path production uses. Neither can import the other, so they agree
// through JSON tag names — and both sides were taking `intent` and `at_ms` off a record that
// has carried a pointer position since the mouse half existed.
//
// Nothing errored. A recording of somebody navigating a menu with the mouse replayed as a run
// of `point` intents with no positions: a record that somebody clicked, and no record of what
// they clicked. That is the whole content of the demonstration, and it is also the half that
// looks fine in a log.
//
// So the line is written here in the writer's exact shape and replayed through the real path.

// writeInputs writes input.jsonl in `capture-game-session`'s layout.
//
// Hand-written rather than copied from a session, for the reason the frame fixture is: a
// fixture taken from one recording freezes that recording's accidents.
func writeInputs(t *testing.T, dir string, lines []string) {
	t.Helper()
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "input.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The input record reads the writer's tags, position included.
func TestTheInputRecordReadsTheWritersTags(t *testing.T) {
	const line = `{"intent":"point","at_ms":1480,"device":"mouse",` +
		`"where":{"x":0.42,"y":0.55},"observed":true}`
	var in inputRecord
	if err := json.Unmarshal([]byte(line), &in); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		got  any
		want any
	}{
		{"intent", in.Intent, "point"},
		{"at_ms", in.AtMS, int64(1480)},
		{"device", in.Device, "mouse"},
		{"where.x", in.Where.X, 0.42},
		{"where.y", in.Where.Y, 0.55},
	} {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s decoded as %v, want %v. A `where` that does not decode makes "+
				"every click in a mouse demonstration a placeless intent, and the "+
				"replay reports presses it cannot say anything about",
				c.name, c.got, c.want)
		}
	}
}

// A press reaches perception with its position, and the replay says what it landed on.
//
// The fixture draws five rows at a known place, so the two presses have known answers: one in
// the middle of the third row, one in the top-left corner where nothing is drawn. A replay
// that dropped the position would score both the same way, whatever that way is — which is
// why the background press is here rather than only the row one.
//
// Deleting the `Where` field from inputRecord, or the containment tally in replayFrames, must
// fail this.
func TestAPointerPressReachesPerceptionWithItsPosition(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "session")
	writeRecording(t, dir, 24)
	// The fixture's rows are x 60..420 of 480 and the third is y 152..192 of 360.
	writeInputs(t, dir, []string{
		`{"intent":"point","at_ms":200,"device":"mouse",` +
			`"where":{"x":0.5,"y":0.478},"observed":true}`,
		`{"intent":"point","at_ms":600,"device":"mouse",` +
			`"where":{"x":0.02,"y":0.02},"observed":true}`,
	})

	r, err := replayFrames(dir, 1, 0, false)
	if err != nil {
		t.Fatalf("replaying a recording with two presses: %v", err)
	}
	if r.PointsSeen != 2 {
		t.Fatalf("%d pointer press(es) reached the replay of 2. A press the input "+
			"path never sees is a demonstration with nothing in it", r.PointsSeen)
	}
	if r.PointsOnAShape != 1 {
		t.Errorf("%d press(es) landed on a proposed region, want 1: one was aimed at "+
			"the middle of a drawn row and one at bare background. Scoring both the "+
			"same way is what a dropped position looks like", r.PointsOnAShape)
	}
}

// A press on nested regions is counted as ambiguous rather than resolved here.
//
// The live pointer path picks the SMALLEST containing control, and that rule lives there. This
// program counts whether an answer was possible and does not invent a second copy of the rule
// — which is this repository's worst class of defect: two statements of one rule that drift.
func TestAPressInsideTwoRegionsIsCountedAsAmbiguous(t *testing.T) {
	// Binary fractions throughout, so the edges below are the edges and not a float's
	// idea of them: 0.4+0.2 is 0.6000000000000001, and a test that asserted an exact
	// boundary on those numbers would be measuring the arithmetic. The half-open rule
	// is exact where the arithmetic is, which is a real limit of it and not a claim
	// this test should paper over.
	shapes := []observe.ShadowShape{
		{Region: observe.Region{X: 0.125, Y: 0.125, Width: 0.75, Height: 0.75}},
		{Region: observe.Region{X: 0.5, Y: 0.375, Width: 0.25, Height: 0.125}},
	}
	if n := shapesUnder(shapes, observe.PointerAt{X: 0.625, Y: 0.4375}); n != 2 {
		t.Errorf("a press inside a row inside a panel is under %d region(s), want 2", n)
	}
	if n := shapesUnder(shapes, observe.PointerAt{X: 0.625, Y: 0.8}); n != 1 {
		t.Errorf("a press inside the panel alone is under %d region(s), want 1", n)
	}
	if n := shapesUnder(shapes, observe.PointerAt{X: 0.01, Y: 0.01}); n != 0 {
		t.Errorf("a press on bare background is under %d region(s), want 0", n)
	}
	// The right and bottom edges belong to whatever is beyond them, the same half-open
	// convention windowBounds.place uses — so a press on the boundary between two
	// stacked rows belongs to one of them and not to both.
	if n := shapesUnder(shapes, observe.PointerAt{X: 0.75, Y: 0.4375}); n != 1 {
		t.Errorf("a press on the inner region's right edge is under %d region(s), "+
			"want 1: the edge is outside", n)
	}
	if n := shapesUnder(shapes, observe.PointerAt{X: 0.625, Y: 0.5}); n != 1 {
		t.Errorf("a press on the inner region's bottom edge is under %d region(s), "+
			"want 1: it belongs to what is below it", n)
	}
	// AND THE LEFT AND TOP EDGES ARE INSIDE, which is the other half of the same
	// convention: a press on the first pixel of a row belongs to that row. The live
	// placement rule says the same thing — a press at a window's top-left corner
	// normalises to 0,0 and is inside it — and a row whose own leading edge rejected a
	// press would hand every such click to whatever encloses it.
	if n := shapesUnder(shapes, observe.PointerAt{X: 0.5, Y: 0.4375}); n != 2 {
		t.Errorf("a press on the inner region's left edge is under %d region(s), "+
			"want 2: the leading edge belongs to the region", n)
	}
	if n := shapesUnder(shapes, observe.PointerAt{X: 0.625, Y: 0.375}); n != 2 {
		t.Errorf("a press on the inner region's top edge is under %d region(s), "+
			"want 2: the leading edge belongs to the region", n)
	}
	// A region with no area is not a place. It needs no guard of its own — for zero
	// width the right-edge test is the negation of the left-edge one, so the half-open
	// rule already excludes every point — and it is asserted because that is a property
	// of the comparison rather than something written down anywhere.
	none := []observe.ShadowShape{{Region: observe.Region{X: 0.5, Y: 0.5}}}
	if n := shapesUnder(none, observe.PointerAt{X: 0.5, Y: 0.5}); n != 0 {
		t.Errorf("a zero-area region contained a press (%d)", n)
	}
}

// The three tallies count three different outcomes of one press.
//
// Deterministic, over hand-made geometry, because the drawn recording fixture happens to
// produce no nested proposals — so a fixture-only test of the ambiguity count would pass
// whether the count existed or not. A real frame proposes two hundred regions and nesting is
// ordinary there, which is exactly why the number has to be right before a session is read
// with it.
//
// Deleting any of the three increments must fail this.
func TestEachPressIsCountedForWhatItLandedOn(t *testing.T) {
	shapes := []observe.ShadowShape{
		// A panel, and one row inside it.
		{Region: observe.Region{X: 0.125, Y: 0.125, Width: 0.75, Height: 0.75}},
		{Region: observe.Region{X: 0.5, Y: 0.375, Width: 0.25, Height: 0.125}},
	}
	slot := []observe.InputEvent{
		// On the row inside the panel: an answer exists and needs choosing.
		{Intent: observe.NavPoint, Where: observe.PointerAt{X: 0.625, Y: 0.4375}},
		// On the panel alone: an answer exists and is unambiguous.
		{Intent: observe.NavPoint, Where: observe.PointerAt{X: 0.625, Y: 0.8}},
		// On bare background: no answer, and nothing to re-ground later.
		{Intent: observe.NavPoint, Where: observe.PointerAt{X: 0.01, Y: 0.01}},
		// Not a press at all, and must not be counted as one.
		{Intent: observe.NavDown},
	}
	var r frameResult
	scorePresses(slot, shapes, &r)

	if r.PointsOnAShape != 2 {
		t.Errorf("%d press(es) landed on a proposed region, want 2 of the three: one "+
			"was on bare background", r.PointsOnAShape)
	}
	if r.PointsAmbiguous != 1 {
		t.Errorf("%d press(es) were under more than one region, want 1. A press inside "+
			"a row inside a panel has an answer that needs choosing, and a tally "+
			"that cannot see that would report every nested click as settled",
			r.PointsAmbiguous)
	}
	// PointsSeen is counted where the intents are read, since a press the replay was
	// given but could not place still happened. Not touched here, and asserted as such
	// so the two sites cannot quietly become one.
	if r.PointsSeen != 0 {
		t.Errorf("scorePresses counted %d press(es) seen; that tally belongs to the "+
			"intent walk, where a press with no position is still a press",
			r.PointsSeen)
	}
	// A directional intent must not be scored as a click. Its Where is zero, which is a
	// real position in the top-left corner — a tally that skipped the intent check would
	// credit every arrow press to whatever sits there.
	var only frameResult
	scorePresses([]observe.InputEvent{{Intent: observe.NavDown}},
		[]observe.ShadowShape{{Region: observe.Region{X: 0, Y: 0, Width: 0.2, Height: 0.2}}},
		&only)
	if only.PointsOnAShape != 0 {
		t.Error("a directional press was scored as landing on a region. Its position " +
			"is zero, and zero is the top-left corner of the screen")
	}
}
