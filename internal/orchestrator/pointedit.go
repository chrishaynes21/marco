package orchestrator

// A PRESS CAN BE MOVED, AND THE TWO NUMBERS BESIDE IT STAY TRUE.
//
// # What a press knows about where it is
//
// A click recorded inside a window is written down TWICE: as a point on the screen (`X`, `Y`) and
// as an offset inside that window (`RelX`, `RelY`), with the window's own terms beside them —
// `RelTo`, `WinW`, `WinH`, `Dpi`, `Process`. Their presence IS the opt-in for ADR-250's guarded
// contract: a run checks the window it is about to press in against those terms and refuses when
// they disagree.
//
// # Why editing one number is possible at all
//
// Because the two were measured AT THE SAME INSTANT. So the window's origin is recoverable —
// `(X-RelX, Y-RelY)` — and moving the offset can move the screen point with it, out of the same
// measurement. Nothing is invented, no new reading is taken, and the window terms still describe
// the window they always described.
//
// Without that, editing either number makes the other one a lie: a routine whose offset says one
// place and whose screen point says another is a routine that presses one of them and checks the
// other.
//
// # What is refused, and why each one
//
//	A CLICK FOUND BY ITS PICTURE   `Template`, `Color`, `AnchorText` or `AnchorVision` — the
//	                               coordinate is a position INSIDE the picture, so moving it aims
//	                               the picture somewhere else. It is a position in a thing that
//	                               is looked for, not a place.
//	A DRAG                         two points and one rectangle. Out of this slice by decision,
//	                               and `simplify` already refuses a drag whose ends disagree
//	                               about the window.
//	AN INCOMPLETE MEASUREMENT      a step that says it was measured in a window and does not
//	                               carry the terms that say WHICH window. Completing it here
//	                               would be manufacturing a guarded point out of a reading
//	                               nobody took.
//	AN OFFSET OUTSIDE THE WINDOW   an offset the window could not contain is not a place in it.
//
// And a step that carries no `WinRel` is edited by its SCREEN point, unchanged in kind: a press
// that was never measured against a window does not acquire a window by being moved.
//
// EVERYTHING ELSE ON THE STEP IS KEPT. The button, the modifiers, the count, the wait and every
// guard term travel untouched, because the operation copies the step and sets four numbers at
// most. A press that was checked against its window before an edit is checked against it after.
//
// See [[ADR-286-a-press-can-be-moved]].

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// MovablePoint is what a surface needs to offer the edit, and why it cannot.
//
// Returned for EVERY step, so a panel never works out for itself which rows have a position: a
// surface that decided would be free to offer a box the validator then refuses, which is a
// control whose refusal arrives after somebody has typed into it.
type MovablePoint struct {
	// Is says this step presses somewhere at all.
	Is bool
	// Movable says its position can be changed, and Why says what stops it when it
	// cannot. A step with `Is` and no `Movable` is shown with the reason ON THE ROW.
	Movable bool
	Why     string
	// InWindow says the editable pair is the offset INSIDE the window rather than the
	// point on the screen. The two survive different things and a person needs to know
	// which they are looking at: an offset still means the same place after the window
	// moves, and a screen point does not.
	InWindow bool
	// X and Y are the pair a surface offers, in whichever basis `InWindow` names.
	X, Y int
}

// THERE IS NO `Terms` HERE ANY MORE, and the two steps that took it away are worth
// keeping together.
//
// It began as `left · of 1280×800, 96 dpi, notepad.exe` beside every movable press,
// so that nobody had to find out afterwards whether moving a press also changed what
// it was checked against. [[ADR-289-the-panel-says-less-and-play-is-bigger]] cut it to
// the button alone: the promise is the same promise on every row, so it is made once,
// in Help's `measured` section.
//
// What was left was the button — and the button was never missing. A step is written
// as a sentence (`left-click at 120, 90 in the window`) and the row splits that
// sentence into an action column and a value column, so `Left-click` was already at
// the front of the row while `left` trailed the numbers at the end of it. One fact in
// two places, with the near-invisible one in the column a person scans.
//
// So the action column is where the button lives, and the surface needs nothing from
// here to put it there. See [[ADR-291-the-press-row-says-what-it-does]].

// PointIn is what can be done with the position of one step.
func PointIn(steps []macroir.Step, at int) MovablePoint {
	if at < 0 || at >= len(steps) {
		return MovablePoint{}
	}
	s := steps[at]
	switch s.Kind {
	case macroir.StepClick, macroir.StepMove:
	case macroir.StepDrag:
		// A GESTURE WITH TWO ENDS, and one box for one pair would be a control
		// that edits half of it.
		return MovablePoint{Is: true, Why: "a drag has two ends, and they are " +
			"changed together"}
	default:
		return MovablePoint{}
	}
	out := MovablePoint{Is: true}
	if why := foundByItsPicture(s); why != "" {
		out.Why = why
		return out
	}
	if !s.WinRel {
		out.Movable, out.X, out.Y = true, s.X, s.Y
		return out
	}
	if why := wholeMeasurement(s); why != "" {
		out.Why = why
		return out
	}
	out.Movable, out.InWindow, out.X, out.Y = true, true, s.RelX, s.RelY
	return out
}

// MovableDrag is what can be done with the two ends of a drag, as PointIn is for one press.
//
// ITS OWN READING, NOT POINTIN'S, so that moving a press can never move one end of a drag: a drag
// has two ends and they are changed together, by EditDrag.
type MovableDrag struct {
	Is, Movable bool
	Why         string
	// InWindow says the pairs offered are offsets inside the window the drag was measured
	// in; otherwise they are points on the screen.
	InWindow     bool
	X, Y, X2, Y2 int
}

// DragIn is what can be done with the drag at `at`, if it is one.
func DragIn(steps []macroir.Step, at int) MovableDrag {
	if at < 0 || at >= len(steps) || steps[at].Kind != macroir.StepDrag {
		return MovableDrag{}
	}
	s := steps[at]
	out := MovableDrag{Is: true}
	if why := foundByItsPicture(s); why != "" {
		out.Why = why
		return out
	}
	if !s.WinRel {
		out.Movable, out.X, out.Y, out.X2, out.Y2 = true, s.X, s.Y, s.X2, s.Y2
		return out
	}
	if why := wholeMeasurement(s); why != "" {
		out.Why = why
		return out
	}
	out.Movable, out.InWindow = true, true
	out.X, out.Y, out.X2, out.Y2 = s.RelX, s.RelY, s.ToRelX, s.ToRelY
	return out
}

// dragged is `s` as a drag from x,y to x2,y2 in the basis `inWindow` names, and the two ends in
// words, or why it cannot be.
//
// A COPY OF THE STEP, so its button and every term its window check is made of are carried; only
// the four positions and their screen points change. The screen points move with the offsets out
// of the press's own reading — where the window was at the instant it was measured — so no new
// measurement is taken and none is invented. The recorded path between the ends describes the old
// gesture and is dropped, not kept beside ends it no longer joins.
func dragged(s macroir.Step, at int, inWindow bool, x, y, x2, y2 int) (macroir.Step, string, error) {
	if x == x2 && y == y2 {
		return macroir.Step{}, "", fmt.Errorf("a drag at step %d has to end somewhere other "+
			"than where it starts", at+1)
	}
	if !inWindow {
		s.Kind, s.X, s.Y, s.X2, s.Y2, s.Steps = macroir.StepDrag, x, y, x2, y2, nil
		return s, fmt.Sprintf("from %d, %d to %d, %d on the screen", x, y, x2, y2), nil
	}
	for _, p := range [][2]int{{x, y}, {x2, y2}} {
		if p[0] < 0 || p[1] < 0 || p[0] >= s.WinW || p[1] >= s.WinH {
			return macroir.Step{}, "", fmt.Errorf("step %d was measured in a window %d by "+
				"%d, and %d, %d is outside it — both ends of a drag have to be in the "+
				"window it is checked against", at+1, s.WinW, s.WinH, p[0], p[1])
		}
	}
	originX, originY := s.X-s.RelX, s.Y-s.RelY
	s.Kind, s.Steps = macroir.StepDrag, nil
	s.RelX, s.RelY, s.ToRelX, s.ToRelY = x, y, x2, y2
	s.X, s.Y, s.X2, s.Y2 = originX+x, originY+y, originX+x2, originY+y2
	return s, fmt.Sprintf("from %d, %d to %d, %d in the window (%d, %d to %d, %d on the "+
		"screen)", x, y, x2, y2, s.X, s.Y, s.X2, s.Y2), nil
}

// movedDrag is the drag at `at` with both ends moved, and the review's sentence.
func movedDrag(steps []macroir.Step, at, x, y, x2, y2 int) (macroir.Step, string, error) {
	d := DragIn(steps, at)
	switch {
	case !d.Is:
		return macroir.Step{}, "", fmt.Errorf("step %d is not a drag, so it has no two "+
			"ends to change", at+1)
	case !d.Movable:
		return macroir.Step{}, "", fmt.Errorf("step %d cannot be moved: %s", at+1, d.Why)
	case x == d.X && y == d.Y && x2 == d.X2 && y2 == d.Y2:
		return macroir.Step{}, "", fmt.Errorf("step %d already drags from %d, %d to %d, %d",
			at+1, x, y, x2, y2)
	}
	s, where, err := dragged(steps[at], at, d.InWindow, x, y, x2, y2)
	if err != nil {
		return macroir.Step{}, "", err
	}
	return s, fmt.Sprintf("the drag at step %d now goes %s", at+1, where), nil
}

// clickToDrag is the click at `at` made a drag from x,y to x2,y2, and the review's sentence.
//
// A GUARDED CLICK STAYS GUARDED. The drag is measured against the window the click was measured
// against, with both ends inside it. A click whose measurement cannot be moved — found by its
// picture, or missing a term of its window — is refused with that reason rather than turned into
// a drag on the screen, which would be a checked press becoming an unchecked one with nothing said.
func clickToDrag(steps []macroir.Step, at, x, y, x2, y2 int) (macroir.Step, string, error) {
	if at < 0 || at >= len(steps) || steps[at].Kind != macroir.StepClick {
		return macroir.Step{}, "", fmt.Errorf("step %d is not a click, so it cannot become "+
			"a drag", at+1)
	}
	p := PointIn(steps, at)
	if !p.Movable {
		return macroir.Step{}, "", fmt.Errorf("step %d cannot become a drag: %s", at+1, p.Why)
	}
	if steps[at].Count > 1 {
		return macroir.Step{}, "", fmt.Errorf("step %d presses %d times, and a drag is one "+
			"press held and moved", at+1, steps[at].Count)
	}
	s, where, err := dragged(steps[at], at, p.InWindow, x, y, x2, y2)
	if err != nil {
		return macroir.Step{}, "", err
	}
	btn := s.Button
	if btn == "" {
		btn = "left"
	}
	return s, fmt.Sprintf("the click at step %d becomes a %s-button drag %s", at+1, btn,
		where), nil
}

// movedPoint is the step with its position changed, and what that change is in words.
//
// It copies the step and sets at most four numbers. Everything else — the button, the key, the
// count, the duration, and every term the guarded contract is made of — is carried by the copy,
// which is why this is written as a copy rather than as a new `macroir.Step`.
func movedPoint(steps []macroir.Step, at, x, y int) (macroir.Step, string, error) {
	if at < 0 || at >= len(steps) {
		return macroir.Step{}, "", fmt.Errorf("there is no step %d in a routine of %d "+
			"steps, so nothing was changed — look at it again", at+1, len(steps))
	}
	p := PointIn(steps, at)
	switch {
	case !p.Is:
		return macroir.Step{}, "", fmt.Errorf("step %d does not press anywhere, so "+
			"it has no position to change", at+1)
	case !p.Movable:
		return macroir.Step{}, "", fmt.Errorf("step %d cannot be moved: %s",
			at+1, p.Why)
	}
	s := steps[at]
	if !p.InWindow {
		if x == s.X && y == s.Y {
			return macroir.Step{}, "", fmt.Errorf("step %d is already at %d, %d on "+
				"the screen", at+1, x, y)
		}
		was := fmt.Sprintf("%d, %d", s.X, s.Y)
		s.X, s.Y = x, y
		return s, fmt.Sprintf("the press at step %d moves to %d, %d on the screen "+
			"(it was %s)", at+1, x, y, was), nil
	}
	// ── AN OFFSET THE WINDOW COULD NOT CONTAIN IS NOT A PLACE IN IT ──
	//
	// Checked against the size this press was MEASURED against, which is the size the
	// guard will compare at run time. A negative offset is outside on the other side.
	if x < 0 || y < 0 || x >= s.WinW || y >= s.WinH {
		return macroir.Step{}, "", fmt.Errorf("step %d was measured in a window "+
			"%d by %d, and %d, %d is outside it — a press has to be somewhere "+
			"in the window it is checked against", at+1, s.WinW, s.WinH, x, y)
	}
	if x == s.RelX && y == s.RelY {
		return macroir.Step{}, "", fmt.Errorf("step %d is already at %d, %d in the "+
			"window", at+1, x, y)
	}
	// ── THE ORIGIN, OUT OF THE MEASUREMENT ITSELF ──
	//
	// Both numbers were read at ONE instant, so their difference is where the window was
	// at that instant. The screen point moves with the offset out of that same reading:
	// no new measurement is taken and none is guessed.
	originX, originY := s.X-s.RelX, s.Y-s.RelY
	wasRel := fmt.Sprintf("%d, %d", s.RelX, s.RelY)
	wasAbs := fmt.Sprintf("%d, %d", s.X, s.Y)
	s.RelX, s.RelY = x, y
	s.X, s.Y = originX+x, originY+y
	return s, fmt.Sprintf("the press at step %d moves to %d, %d in the window (it was "+
		"%s), and with it to %d, %d on the screen (it was %s)",
		at+1, x, y, wasRel, s.X, s.Y, wasAbs), nil
}

// foundByItsPicture is why this press cannot be moved, when it is found by looking rather than by
// going to a place.
//
// The coordinate on such a step is a position INSIDE the thing being looked for —
// `AnchorClickX`/`AnchorClickY` is exactly that — so moving it does not move the press, it aims
// the picture at somewhere else. A control that offered it would be offering to break the one
// kind of press that survives a window moving.
func foundByItsPicture(s macroir.Step) string {
	switch {
	case len(s.Template) > 0:
		return "found by its picture: the position is inside that picture, not on " +
			"the screen"
	case s.AnchorVision != "":
		return "found by what Marco can see there: the position is inside what it " +
			"looks for"
	case s.AnchorText != "":
		return "it is found by the words beside it, so the position is a place " +
			"relative to those words"
	case s.Color != "":
		return "it is found by its colour, so the position is a place inside what " +
			"it looks for"
	}
	return ""
}

// wholeMeasurement is why a press that says it was measured in a window cannot be moved inside
// one: it does not carry the terms that say WHICH window.
//
// THE TERMS ARE THE OPT-IN. A run checks the program, the size and the scale before it presses,
// and their presence is what asks for that check. Filling one in here to make the arithmetic work
// would be manufacturing a guarded point out of a reading nobody took — the routine would then be
// checked against a window this code invented. Refused instead, with the missing term named.
func wholeMeasurement(s macroir.Step) string {
	switch {
	case s.RelTo == "":
		return "it says it was measured inside something and does not say what"
	case s.WinW <= 0 || s.WinH <= 0:
		return "the size of the window it was measured in was not recorded, so there " +
			"is nothing to place an offset against"
	case s.Dpi <= 0:
		return "the scale of the window it was measured in was not recorded"
	case s.Process == "":
		return "the program the window belonged to was not recorded"
	}
	return ""
}
