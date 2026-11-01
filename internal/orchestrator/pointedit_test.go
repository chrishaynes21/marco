package orchestrator

// MOVING A PRESS, AND EVERY SHAPE THAT MUST NOT BE MOVED.
//
// The fixture is the owner's own recording, reduced: a click measured inside an Ark window, with
// ADR-250's five terms on it. The window origin in it is NOT zero, which is the whole of what
// these can check — a press at the window's origin makes the offset and the screen point the same
// number, so every arithmetic mistake passes.
//
// See [[ADR-286-a-press-can-be-moved]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// aMeasuredPress is a click recorded inside a window, with the window 1100, 1000 from the corner
// of the screen — so the offset and the screen point are different numbers and stay that way.
//
// REDUCED FROM THE OWNER'S OWN ROUTINE. The terms are the ones `cloud-pull-stone.marco` carries:
//
//	the p1 is a Point with X 342, Y 294, RelX 342, RelY 294, RelTo "window",
//	                       WinW 1920, WinH 1080, Dpi 96, Process "arkascended.exe".
//
// with the window moved off the corner, which the real one was not.
func aMeasuredPress() macroir.Step {
	return macroir.Step{
		Kind: macroir.StepClick, Button: "right", Key: "ctrl", Count: 1,
		X: 1442, Y: 1294, RelX: 342, RelY: 294, WinRel: true,
		RelTo: "window", WinW: 1920, WinH: 1080, Dpi: 96,
		Process: "arkascended.exe", Window: "ARK: Survival Ascended",
	}
}

// aScreenPress is a click that was never measured against a window.
func aScreenPress() macroir.Step {
	return macroir.Step{Kind: macroir.StepClick, Button: "left", X: 1620, Y: 980}
}

// THE OFFSET IS WHAT MOVES, AND THE SCREEN POINT MOVES WITH IT OUT OF THE SAME MEASUREMENT.
func TestMovingAPressInsideItsWindowMovesTheScreenPointWithIt(t *testing.T) {
	steps := []macroir.Step{aMeasuredPress()}
	p := PointIn(steps, 0)
	if !p.Is || !p.Movable || !p.InWindow {
		t.Fatalf("a measured press is not offered as movable in its window: %+v", p)
	}
	if p.X != 342 || p.Y != 294 {
		t.Fatalf("the pair offered is %d, %d — the OFFSET is the editable half",
			p.X, p.Y)
	}

	out, said, err := movedPoint(steps, 0, 350, 300)
	if err != nil {
		t.Fatalf("moving it refused: %v", err)
	}
	if out.RelX != 350 || out.RelY != 300 {
		t.Errorf("the offset is %d, %d", out.RelX, out.RelY)
	}
	// THE ORIGIN IS 1100, 1000 — RECOVERED, NOT GUESSED. 1442-342 and 1294-294.
	if out.X != 1450 || out.Y != 1300 {
		t.Fatalf("the screen point is %d, %d, and the window this was measured in "+
			"starts at 1100, 1000 — so moving the offset to 350, 300 puts the "+
			"press at 1450, 1300", out.X, out.Y)
	}
	for _, want := range []string{"350, 300 in the window", "1450, 1300 on the screen",
		"342, 294", "1442, 1294"} {
		if !strings.Contains(said, want) {
			t.Errorf("the sentence does not say %q: %q", want, said)
		}
	}
}

// AND EVERYTHING ELSE ON THE STEP SURVIVES, INCLUDING EVERY TERM THE GUARD IS MADE OF.
//
// A press that was checked against its window before an edit is checked against it after. The
// terms are compared one at a time rather than by comparing the whole step, because the whole
// step is what CHANGED — and a test that compared it would have to spell the new coordinates,
// which is the thing being checked two lines up.
func TestMovingAPressKeepsTheButtonTheModifiersAndEveryGuardTerm(t *testing.T) {
	was := aMeasuredPress()
	out, _, err := movedPoint([]macroir.Step{was}, 0, 1, 2)
	if err != nil {
		t.Fatalf("moving it refused: %v", err)
	}
	for _, c := range []struct {
		what      string
		got, want any
	}{
		{"the kind", out.Kind, was.Kind},
		{"the button", out.Button, was.Button},
		{"the modifier", out.Key, was.Key},
		{"the count", out.Count, was.Count},
		{"that it is measured in a window", out.WinRel, was.WinRel},
		{"what it is measured against", out.RelTo, was.RelTo},
		{"the window width", out.WinW, was.WinW},
		{"the window height", out.WinH, was.WinH},
		{"the scale", out.Dpi, was.Dpi},
		{"the program", out.Process, was.Process},
		{"the window title", out.Window, was.Window},
	} {
		if c.got != c.want {
			t.Errorf("moving the press lost %s: %v, want %v", c.what, c.got, c.want)
		}
	}
}

// AND A PRESS THAT WAS NEVER MEASURED AGAINST A WINDOW IS MOVED BY ITS SCREEN POINT —
// AND DOES NOT ACQUIRE A WINDOW BY BEING MOVED.
func TestMovingAnUnmeasuredPressMovesTheScreenPointAndInventsNoWindow(t *testing.T) {
	steps := []macroir.Step{aScreenPress()}
	p := PointIn(steps, 0)
	if !p.Movable || p.InWindow {
		t.Fatalf("an unmeasured press is not offered as a point on the screen: %+v", p)
	}
	if p.X != 1620 || p.Y != 980 {
		t.Fatalf("the pair offered is %d, %d", p.X, p.Y)
	}
	out, said, err := movedPoint(steps, 0, 400, 300)
	if err != nil {
		t.Fatalf("moving it refused: %v", err)
	}
	if out.X != 400 || out.Y != 300 {
		t.Errorf("the screen point is %d, %d", out.X, out.Y)
	}
	if !strings.Contains(said, "on the screen") {
		t.Errorf("the sentence does not say which pair moved: %q", said)
	}
	// ── AND NO GUARDED POINT WAS MANUFACTURED ──
	//
	// A press that acquired `WinRel` and a size here would be checked at run time
	// against a window this code invented, and would refuse every press in a window
	// that did not happen to match.
	if out.WinRel || out.RelTo != "" || out.WinW != 0 || out.WinH != 0 ||
		out.Dpi != 0 || out.Process != "" || out.RelX != 0 || out.RelY != 0 {
		t.Fatalf("moving an unmeasured press gave it a window to be checked "+
			"against: %+v", out)
	}
}

// ── AND THE SHAPES THAT REFUSE ────────────────────────────────────────────────

// A PRESS FOUND BY LOOKING IS NOT A PRESS AT A PLACE.
func TestAPressFoundByItsPictureCannotBeMoved(t *testing.T) {
	for _, c := range []struct {
		what string
		step macroir.Step
		says string
	}{
		{"a picture", macroir.Step{Kind: macroir.StepClick, X: 1, Y: 2,
			Template: []byte{1, 2, 3}, AnchorClickX: 4, AnchorClickY: 5},
			"picture"},
		{"what Marco can see", macroir.Step{Kind: macroir.StepClick, X: 1, Y: 2,
			AnchorVision: "the Options button"}, "see"},
		{"the words beside it", macroir.Step{Kind: macroir.StepClick, X: 1, Y: 2,
			AnchorText: "Options"}, "words"},
		{"its colour", macroir.Step{Kind: macroir.StepClick, X: 1, Y: 2,
			Color: "#ff8800"}, "colour"},
	} {
		steps := []macroir.Step{c.step}
		p := PointIn(steps, 0)
		if p.Movable {
			t.Errorf("a press found by %s was offered as movable", c.what)
		}
		if !strings.Contains(p.Why, c.says) {
			t.Errorf("the reason for %s does not say so: %q", c.what, p.Why)
		}
		if _, _, err := movedPoint(steps, 0, 9, 9); err == nil {
			t.Errorf("a press found by %s was moved", c.what)
		}
	}
}

// AND A DRAG HAS TWO ENDS, SO IT IS NOT OFFERED AT ALL.
func TestADragIsNotMovedByThisAndSaysSo(t *testing.T) {
	steps := []macroir.Step{{Kind: macroir.StepDrag, X: 1, Y: 2, X2: 3, Y2: 4}}
	p := PointIn(steps, 0)
	if p.Movable {
		t.Fatal("a drag was offered as a single movable point")
	}
	if !p.Is || !strings.Contains(p.Why, "two ends") {
		t.Errorf("a drag does not say why it is not offered: %+v", p)
	}
	if _, _, err := movedPoint(steps, 0, 9, 9); err == nil {
		t.Fatal("a drag was moved")
	}
}

// AND A MEASUREMENT WITH A TERM MISSING IS REFUSED RATHER THAN COMPLETED.
//
// Filling one in here would be manufacturing a guarded point out of a reading nobody took: the
// routine would then be checked at run time against a window this code invented.
func TestAnIncompleteMeasurementIsRefusedAndNotCompleted(t *testing.T) {
	for _, c := range []struct {
		what string
		lose func(*macroir.Step)
		says string
	}{
		{"what it was measured against", func(s *macroir.Step) { s.RelTo = "" },
			"does not say what"},
		{"the width", func(s *macroir.Step) { s.WinW = 0 }, "size"},
		{"the height", func(s *macroir.Step) { s.WinH = 0 }, "size"},
		{"the scale", func(s *macroir.Step) { s.Dpi = 0 }, "scale"},
		{"the program", func(s *macroir.Step) { s.Process = "" }, "program"},
	} {
		s := aMeasuredPress()
		c.lose(&s)
		steps := []macroir.Step{s}
		p := PointIn(steps, 0)
		if p.Movable {
			t.Errorf("a press missing %s was offered as movable", c.what)
		}
		if !strings.Contains(p.Why, c.says) {
			t.Errorf("the reason for a press missing %s is %q", c.what, p.Why)
		}
		out, _, err := movedPoint(steps, 0, 10, 10)
		if err == nil {
			t.Errorf("a press missing %s was moved to %d, %d", c.what, out.X, out.Y)
		}
	}
}

// AND AN OFFSET THE WINDOW COULD NOT CONTAIN IS NOT A PLACE IN IT.
func TestAnOffsetOutsideTheWindowIsRefused(t *testing.T) {
	steps := []macroir.Step{aMeasuredPress()}
	for _, c := range [][2]int{{-1, 10}, {10, -1}, {1920, 10}, {10, 1080}, {9999, 9999}} {
		if _, _, err := movedPoint(steps, 0, c[0], c[1]); err == nil {
			t.Errorf("%d, %d was accepted in a window 1920 by 1080", c[0], c[1])
		} else if !strings.Contains(err.Error(), "outside it") {
			t.Errorf("the refusal for %d, %d does not say why: %v", c[0], c[1], err)
		}
	}
	// AND THE EDGES ARE IN IT.
	for _, c := range [][2]int{{0, 0}, {1919, 1079}} {
		if _, _, err := movedPoint(steps, 0, c[0], c[1]); err != nil {
			t.Errorf("%d, %d is inside a window 1920 by 1080 and was refused: %v",
				c[0], c[1], err)
		}
	}
}

// AND A STEP THAT PRESSES NOWHERE, A POSITION IT ALREADY HAS, AND A STEP THAT IS NOT THERE.
func TestMovingWhatCannotBeMovedSaysWhichOfTheThreeItIs(t *testing.T) {
	steps := []macroir.Step{
		{Kind: macroir.StepWait, Ms: 250},
		aMeasuredPress(),
	}
	if p := PointIn(steps, 0); p.Is {
		t.Error("a wait was offered a position")
	}
	if _, _, err := movedPoint(steps, 0, 1, 2); err == nil ||
		!strings.Contains(err.Error(), "does not press anywhere") {
		t.Errorf("moving a wait said: %v", err)
	}
	if _, _, err := movedPoint(steps, 1, 342, 294); err == nil ||
		!strings.Contains(err.Error(), "already at") {
		t.Errorf("moving a press to where it already is said: %v", err)
	}
	for _, at := range []int{-1, 2, 99} {
		if _, _, err := movedPoint(steps, at, 1, 2); err == nil {
			t.Errorf("step %d was moved in a routine of 2 steps", at)
		}
	}
}

// ── AND THROUGH THE EDIT CONTRACT ─────────────────────────────────────────────

// A MOVE COLLECTS WITH THE OTHER CHANGES, AND REFUSES THE ONE IT CANNOT BE ASKED BESIDE.
//
// Unlike a typed-run declaration, this leaves the list the length it was — so every other
// position in the same review still means what it meant. What it cannot be asked beside is a
// REMOVAL of the same step, which is two answers to one question.
func TestAMoveCollectsWithOtherChangesAndRefusesARemovalOfTheSameStep(t *testing.T) {
	steps := []macroir.Step{
		{Kind: macroir.StepActivate, Text: "arkascended"},
		{Kind: macroir.StepWait, Ms: 250},
		aMeasuredPress(),
	}
	out, says, refused, _, err := applyEdits(steps, []StepEdit{
		{Op: EditPoint, At: 2, X: 350, Y: 300},
		{Op: EditWait, At: 1, To: 800},
	})
	if err != nil || refused != "" {
		t.Fatalf("a move beside a wait refused: %v %s", err, refused)
	}
	if len(out) != 3 {
		t.Fatalf("the list is %d steps long", len(out))
	}
	if out[2].RelX != 350 || out[2].X != 1450 || out[1].Ms != 800 {
		t.Fatalf("one of the two changes did not land: %+v", out)
	}
	if len(says) != 2 {
		t.Errorf("%d sentences for two changes: %v", len(says), says)
	}

	for _, pair := range [][]StepEdit{
		{{Op: EditPoint, At: 2, X: 350, Y: 300}, {Op: EditRemove, At: 2}},
		{{Op: EditRemove, At: 2}, {Op: EditPoint, At: 2, X: 350, Y: 300}},
		{{Op: EditPoint, At: 2, X: 350, Y: 300}, {Op: EditPoint, At: 2, X: 1, Y: 1}},
	} {
		if _, _, _, _, err := applyEdits(steps, pair); err == nil {
			t.Errorf("%v was applied", pair)
		}
	}
}
