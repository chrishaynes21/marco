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
	// Terms are the machine facts beside it — the button, and the window it was measured
	// against when there is one. Shown dim; they are what the edit KEEPS.
	Terms string
}

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
		return MovablePoint{Is: true, Why: "a drag has two ends — moving one of " +
			"them is not something this offers yet"}
	default:
		return MovablePoint{}
	}
	out := MovablePoint{Is: true, Terms: pointTerms(s)}
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
		return "it is found by its picture, so the position is a place inside that " +
			"picture rather than a place on the screen"
	case s.AnchorVision != "":
		return "it is found by what Marco can see there, so the position is a place " +
			"inside what it looks for"
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

// pointTerms are the machine facts beside a press: what it presses with, and the window it is
// checked against when there is one.
//
// SHOWN BECAUSE THEY ARE WHAT THE EDIT KEEPS. A row that showed two editable numbers and nothing
// else would leave somebody to find out afterwards whether moving a press also changed what it
// was checked against.
func pointTerms(s macroir.Step) string {
	button := s.Button
	if button == "" && s.Kind == macroir.StepClick {
		button = "left"
	}
	if !s.WinRel || s.WinW <= 0 || s.WinH <= 0 {
		return button
	}
	where := fmt.Sprintf("of %d×%d", s.WinW, s.WinH)
	if s.Dpi > 0 {
		where += fmt.Sprintf(", %d dpi", s.Dpi)
	}
	if s.Process != "" {
		where += ", " + s.Process
	}
	if button == "" {
		return where
	}
	return button + " · " + where
}
