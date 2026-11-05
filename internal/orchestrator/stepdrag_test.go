package orchestrator

// A CLICK CAN BECOME A DRAG, AND A DRAG'S ENDS CAN MOVE — with the button, the held keys around
// it and the window check all kept. See [[ADR-314-drags-can-be-edited]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

func inAWindow(s macroir.Step) macroir.Step {
	s.WinRel, s.RelTo, s.WinW, s.WinH, s.Dpi, s.Process = true, "window", 1280, 720, 96,
		"notepad.exe"
	return s
}

// checkedClickSteps: the window comes forward, a right-click measured 100,80 inside a window whose
// origin was at 400,320 on the screen, a wait.
func checkedClickSteps() []macroir.Step {
	return []macroir.Step{
		{Kind: macroir.StepActivate, Text: "notepad"},
		inAWindow(macroir.Step{Kind: macroir.StepClick, Button: "right", X: 500, Y: 400,
			RelX: 100, RelY: 80}),
		{Kind: macroir.StepWait, Ms: 100},
	}
}

func checkedDragSteps() []macroir.Step {
	return []macroir.Step{
		{Kind: macroir.StepActivate, Text: "notepad"},
		inAWindow(macroir.Step{Kind: macroir.StepDrag, Button: "left", X: 500, Y: 400,
			X2: 700, Y2: 450, RelX: 100, RelY: 80, ToRelX: 300, ToRelY: 130,
			Steps: []macroir.Step{{Kind: macroir.StepMove, X: 600, Y: 420}}}),
	}
}

// A CHECKED CLICK BECOMES A DRAG IN THE SAME WINDOW, still checked, with its button.
func TestACheckedClickBecomesACheckedDragInTheSameWindow(t *testing.T) {
	out, says, why := editedFrom(checkedClickSteps(),
		StepEdit{Op: EditToDrag, At: 1, X: 100, Y: 80, X2: 300, Y2: 200})
	if why != "" {
		t.Fatalf("it refused: %s", why)
	}
	d := out[1]
	if d.Kind != macroir.StepDrag || d.Button != "right" {
		t.Fatalf("the click became %+v", d)
	}
	if !d.WinRel || d.RelTo != "window" || d.WinW != 1280 || d.WinH != 720 || d.Dpi != 96 ||
		d.Process != "notepad.exe" {
		t.Errorf("the drag lost its window check: %+v", d)
	}
	if d.RelX != 100 || d.RelY != 80 || d.ToRelX != 300 || d.ToRelY != 200 {
		t.Errorf("the drag's ends in the window are %d,%d to %d,%d", d.RelX, d.RelY,
			d.ToRelX, d.ToRelY)
	}
	// THE SCREEN POINTS FOLLOW FROM THE ORIGIN THE CLICK WAS MEASURED AT (400, 320).
	if d.X != 500 || d.Y != 400 || d.X2 != 700 || d.Y2 != 520 {
		t.Errorf("the drag's screen points are %d,%d to %d,%d", d.X, d.Y, d.X2, d.Y2)
	}
	if len(says) != 1 || !strings.Contains(says[0], "right-button drag") ||
		!strings.Contains(says[0], "in the window") {
		t.Errorf("the review does not say what it becomes and in which basis: %v", says)
	}
}

// AN UNCHECKED CLICK BECOMES A DRAG ON THE SCREEN, and says so.
func TestAnUncheckedClickBecomesADragOnTheScreen(t *testing.T) {
	steps := []macroir.Step{{Kind: macroir.StepClick, X: 10, Y: 20}}
	out, says, why := editedFrom(steps, StepEdit{Op: EditToDrag, At: 0, X: 10, Y: 20, X2: 90, Y2: 20})
	if why != "" {
		t.Fatalf("it refused: %s", why)
	}
	if out[0].Kind != macroir.StepDrag || out[0].WinRel || out[0].X2 != 90 {
		t.Errorf("the click became %+v", out[0])
	}
	if !strings.Contains(says[0], "on the screen") || !strings.Contains(says[0], "left-button") {
		t.Errorf("the review says %v", says)
	}
}

// A CLICK THAT CANNOT BE A CHECKED DRAG IS REFUSED — never made an unchecked one.
func TestAClickThatCannotStayCheckedIsNotMadeADrag(t *testing.T) {
	incomplete := checkedClickSteps()
	incomplete[1].RelTo = "" // says it was measured in a window, and not which
	pictured := checkedClickSteps()
	pictured[1].Template = []byte{1}
	double := checkedClickSteps()
	double[1].Count = 2
	for _, c := range []struct {
		what  string
		steps []macroir.Step
		edit  StepEdit
		says  string
	}{
		{"an incomplete measurement", incomplete,
			StepEdit{Op: EditToDrag, At: 1, X: 100, Y: 80, X2: 200, Y2: 80}, "does not say what"},
		{"found by its picture", pictured,
			StepEdit{Op: EditToDrag, At: 1, X: 100, Y: 80, X2: 200, Y2: 80}, "picture"},
		{"an end outside the window", checkedClickSteps(),
			StepEdit{Op: EditToDrag, At: 1, X: 100, Y: 80, X2: 1280, Y2: 80}, "outside it"},
		{"no movement", checkedClickSteps(),
			StepEdit{Op: EditToDrag, At: 1, X: 100, Y: 80, X2: 100, Y2: 80}, "somewhere other"},
		{"a double click", double,
			StepEdit{Op: EditToDrag, At: 1, X: 100, Y: 80, X2: 200, Y2: 80}, "presses 2 times"},
		{"not a click", checkedClickSteps(),
			StepEdit{Op: EditToDrag, At: 2, X: 1, Y: 1, X2: 2, Y2: 2}, "is not a click"},
	} {
		t.Run(c.what, func(t *testing.T) {
			out, _, why := editedFrom(c.steps, c.edit)
			if why == "" {
				t.Fatalf("it was accepted: %+v", out)
			}
			if !strings.Contains(why, c.says) {
				t.Errorf("it was refused, saying %q", why)
			}
		})
	}
}

// A DRAG'S ENDS MOVE TOGETHER, IN ITS WINDOW; ITS OLD PATH GOES; MOVING A PRESS NEVER MOVES ONE END.
func TestADragsEndsMoveTogetherAndOnlyTogether(t *testing.T) {
	out, says, why := editedFrom(checkedDragSteps(),
		StepEdit{Op: EditDrag, At: 1, X: 50, Y: 60, X2: 400, Y2: 300})
	if why != "" {
		t.Fatalf("it refused: %s", why)
	}
	d := out[1]
	if d.RelX != 50 || d.RelY != 60 || d.ToRelX != 400 || d.ToRelY != 300 ||
		d.X != 450 || d.Y != 380 || d.X2 != 800 || d.Y2 != 620 {
		t.Errorf("the drag is now %+v", d)
	}
	if d.Button != "left" || d.Process != "notepad.exe" || !d.WinRel {
		t.Errorf("the drag lost its button or its window: %+v", d)
	}
	if len(d.Steps) != 0 {
		t.Errorf("the path between the old ends was kept beside ends it no longer joins: %+v",
			d.Steps)
	}
	if !strings.Contains(says[0], "in the window") {
		t.Errorf("the review says %v", says)
	}
	if _, _, why := editedFrom(checkedDragSteps(),
		StepEdit{Op: EditPoint, At: 1, X: 10, Y: 10}); !strings.Contains(why, "changed together") {
		t.Errorf("moving a press moved one end of a drag, or was refused saying %q", why)
	}
}

func TestADragAndAnotherChangeToOneStepAreRefusedInEitherOrder(t *testing.T) {
	for _, c := range []struct {
		what  string
		steps []macroir.Step
		a, b  StepEdit
	}{
		{"made a drag and moved", checkedClickSteps(),
			StepEdit{Op: EditToDrag, At: 1, X: 100, Y: 80, X2: 200, Y2: 80},
			StepEdit{Op: EditPoint, At: 1, X: 5, Y: 5}},
		{"made a drag and removed", checkedClickSteps(),
			StepEdit{Op: EditToDrag, At: 1, X: 100, Y: 80, X2: 200, Y2: 80},
			StepEdit{Op: EditRemove, At: 1}},
		{"dragged and removed", checkedDragSteps(),
			StepEdit{Op: EditDrag, At: 1, X: 5, Y: 5, X2: 50, Y2: 50},
			StepEdit{Op: EditRemove, At: 1}},
	} {
		for _, order := range [][]StepEdit{{c.a, c.b}, {c.b, c.a}} {
			if _, _, why := editedFrom(c.steps, order...); !strings.Contains(why,
				"nothing was changed") {
				t.Errorf("%s, %s first: refused saying %q", c.what, order[0].Op, why)
			}
		}
	}
}

// ── THROUGH SAVE, THE PRODUCTION DRIVER, THE NEXT EDIT AND A REBUILD ─────────

// A CHECKED CLICK MADE A DRAG IS A CHECKED DRAG WHEN IT RUNS, its ends can be moved by the next
// edit, and a rebuild under today's rules keeps it.
func TestAClickMadeADragIsACheckedDragThroughEverythingAfter(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	at := firstStepOf(t, d, rt, macroir.StepClick)
	p := PointIn(mustSteps(t, d, rt), at)
	if !p.InWindow {
		t.Fatalf("the fixture's first click is not measured in a window: %+v", p)
	}
	editing(t, d, rt, StepEdit{Op: EditToDrag, At: at, X: p.X, Y: p.Y, X2: p.X + 40, Y2: p.Y})
	drag := findCall(whatItAsked(t, d, rt, 0), "OS's Drag with")
	if drag == "" || !strings.Contains(drag, "FromRelX") || !strings.Contains(drag, "arkascended") {
		t.Errorf("the routine's drag is not checked against the click's window: %q", drag)
	}
	// THE NEXT EDIT OFFERS ITS ENDS, IN THE WINDOW, AND MOVES THEM.
	dr := DragIn(mustSteps(t, d, rt), at)
	if !dr.Movable || !dr.InWindow || dr.X2 != p.X+40 {
		t.Fatalf("the next edit offers the drag as %+v", dr)
	}
	editing(t, d, rt, StepEdit{Op: EditDrag, At: at, X: dr.X, Y: dr.Y, X2: dr.X2 + 10, Y2: dr.Y2})
	if got := DragIn(mustSteps(t, d, rt), at); got.X2 != p.X+50 {
		t.Errorf("after moving its end the drag ends at %d in the window", got.X2)
	}
	rb, err := d.Prepare(rt, RebuildCurrent)
	if err != nil || rb.Refused != "" {
		t.Fatalf("the rebuild was refused: %v %q", err, rb.Refused)
	}
	if !strings.Contains(rb.Now, "a Drag with") || !strings.Contains(rb.Now, "ToRelX") {
		t.Errorf("a rebuild under today's rules would lose the checked drag:\n%s", rb.Now)
	}
}

// A HELD KEY AROUND THE CLICK IS STILL AROUND THE DRAG.
func TestAKeyHeldAroundAClickIsHeldAroundItsDrag(t *testing.T) {
	d, rt := anEditableRoutine(t, "shift click", aHeldKeyDemonstration())
	at := firstStepOf(t, d, rt, macroir.StepClick)
	p := PointIn(mustSteps(t, d, rt), at)
	editing(t, d, rt, StepEdit{Op: EditToDrag, At: at, X: p.X, Y: p.Y, X2: p.X + 20, Y2: p.Y + 20})
	calls := whatItAsked(t, d, rt, 0)
	down, drag, up := -1, -1, -1
	for i, c := range calls {
		switch {
		case c == "OS's KeyDown with shift" && down < 0:
			down = i
		case strings.HasPrefix(c, "OS's Drag with") && drag < 0:
			drag = i
		case c == "OS's KeyUp with shift" && up < 0:
			up = i
		}
	}
	if !(down >= 0 && down < drag && drag < up) {
		t.Errorf("shift is not held around the drag: %v", calls)
	}
}

// mustSteps is the routine's step list as the next edit would see it.
func mustSteps(t *testing.T, d Deps, rt routes.Route) []macroir.Step {
	t.Helper()
	e, err := d.PrepareEdit(rt, nil)
	if err != nil {
		t.Fatal(err)
	}
	return e.Steps()
}

// firstStepOf is the position of the routine's first step of `kind`.
func firstStepOf(t *testing.T, d Deps, rt routes.Route, kind macroir.StepKind) int {
	t.Helper()
	for i, s := range mustSteps(t, d, rt) {
		if s.Kind == kind {
			return i
		}
	}
	t.Fatalf("the routine has no %s step", kind)
	return -1
}

func findCall(calls []string, prefix string) string {
	for _, c := range calls {
		if strings.HasPrefix(c, prefix) {
			return c
		}
	}
	return ""
}
