package main

// A CLICK IS MADE A DRAG, AND A DRAG'S ENDS MOVE, THROUGH THE STEP PANEL. See
// [[ADR-314-drags-can-be-edited]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// aMeasuredClickIntoNotepad is one click measured in full inside notepad's window, then esc.
func aMeasuredClickIntoNotepad() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 640, Y: 480, App: "notepad",
			PID: gamePID, WinRel: true, RelTo: "window", RelX: 40, RelY: 80, WinW: 1280,
			WinH: 720, Dpi: 96, Process: "notepad.exe", T: at(700)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 640, Y: 480, App: "notepad",
			PID: gamePID, T: at(710)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(1500), App: "notepad"},
	}
}

// A CLICK WHOSE OFFSET IN ITS WINDOW WAS NEVER MEASURED IS NOT OFFERED AS A DRAG: it could only
// become one on the screen, which would turn a checked press into an unchecked one.
func TestAClickThatCannotStayCheckedIsNotOfferedAsADrag(t *testing.T) {
	a := aStudioLibrary(t, "shift click", heldShiftIntoNotepad())
	for _, row := range a.StepEditing(a.Routines()[0].ID).Rows {
		if row.ToDrag {
			t.Errorf("a click with no measured offset is offered as a drag: %+v", row)
		}
	}
}

func TestACheckedClickIsMadeACheckedDragThroughTheStepPanel(t *testing.T) {
	a := aStudioLibrary(t, "click it", aMeasuredClickIntoNotepad())
	r := a.Routines()[0]
	e := a.StepEditing(r.ID)
	var click StepRow
	for _, row := range e.Rows {
		if row.ToDrag {
			click = row
		}
	}
	if !click.ToDrag || !click.InWindow {
		t.Fatalf("the measured click is not offered as a drag in its window: %+v", e.Rows)
	}
	v := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "todrag", At: click.At,
		X: click.X, Y: click.Y, X2: click.X + 30, Y2: click.Y + 10}})
	if v.Why != "" || v.Token == "" {
		t.Fatalf("the drag was not prepared: %q", v.Why)
	}
	if !strings.Contains(strings.Join(v.Doing, " "), "in the window") {
		t.Errorf("the review does not say the drag's basis: %v", v.Doing)
	}
	if done := a.SaveStepEdit(v.Token); !done.Saved {
		t.Fatalf("the drag was not saved: %+v", done)
	}
	if src := sourceOnDisk(t, a, r); !strings.Contains(src, "a Drag with") ||
		!strings.Contains(src, "FromRelX") || !strings.Contains(src, "notepad") {
		t.Errorf("the file's drag is not checked against the click's window:\n%s", src)
	}
	// AND THE NEXT LISTING OFFERS ITS TWO ENDS, IN THE WINDOW, AND MOVES THEM.
	e = a.StepEditing(r.ID)
	var drag StepRow
	for _, row := range e.Rows {
		if row.Drag {
			drag = row
		}
	}
	if !drag.DragMovable || !drag.InWindow || drag.X2 != click.X+30 || drag.Point {
		t.Fatalf("the drag is listed as %+v", drag)
	}
	v = a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "drag", At: drag.At,
		X: drag.X, Y: drag.Y, X2: drag.X2 + 5, Y2: drag.Y2}})
	if v.Why != "" {
		t.Fatalf("moving the drag's ends was refused: %q", v.Why)
	}
	if done := a.SaveStepEdit(v.Token); !done.Saved {
		t.Fatalf("moving the drag's ends was not saved: %+v", done)
	}
	for _, row := range a.StepEditing(r.ID).Rows {
		if row.Drag && row.X2 != click.X+35 {
			t.Errorf("after moving its end the drag ends at %d", row.X2)
		}
	}
}
