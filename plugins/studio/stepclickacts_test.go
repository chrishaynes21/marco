package main

// A CLICK IS ADDED THROUGH THE STEP PANEL, IN AN EXPLICITLY CHOSEN BASIS. See
// [[ADR-315-a-click-can-be-added]].

import (
	"strings"
	"testing"
)

// A ROUTINE MEASURED IN A WINDOW OFFERS THAT WINDOW, AND NO SCREEN POINT.
func TestAClickIsAddedInsideTheRoutinesWindowThroughTheStepPanel(t *testing.T) {
	a := aStudioLibrary(t, "click it", aMeasuredClickIntoNotepad())
	r := a.Routines()[0]
	e := a.StepEditing(r.ID)
	if len(e.ClickBases) != 1 || e.ScreenClicks || e.ClickSays != "" {
		t.Fatalf("a routine measured in one window offers %+v, screen %v, saying %q",
			e.ClickBases, e.ScreenClicks, e.ClickSays)
	}
	b := e.ClickBases[0]
	if b.W != 1280 || b.H != 720 || !strings.Contains(b.Label, "notepad.exe") {
		t.Errorf("the basis is offered as %+v", b)
	}
	v := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "insert", At: b.At, Kind: "click",
		Button: "right", BasisAt: b.At, X: 200, Y: 100}})
	if v.Why != "" || v.Token == "" {
		t.Fatalf("the click was not prepared: %q", v.Why)
	}
	if !strings.Contains(strings.Join(v.Doing, " "), "right-click at 200, 100 in the window") {
		t.Errorf("the review does not say the action and its basis: %v", v.Doing)
	}
	if done := a.SaveStepEdit(v.Token); !done.Saved {
		t.Fatalf("the click was not saved: %+v", done)
	}
	if src := sourceOnDisk(t, a, r); !strings.Contains(src, "RelX 200") {
		t.Errorf("the file has no click at 200 in the window:\n%s", src)
	}
	// AND A SCREEN POINT IS NOT A FALLBACK HERE.
	e = a.StepEditing(r.ID)
	v = a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "insert", At: b.At, Kind: "click",
		Screen: true, X: 5, Y: 5}})
	if v.Why == "" || !v.Correct || !strings.Contains(v.Why, "checks each press") {
		t.Errorf("a screen click in a checked routine was not refused as a correction: %+v", v)
	}
}

// A ROUTINE THAT CHECKS NOTHING OFFERS A SCREEN POINT, AND SAYS IT HAS NO WINDOW.
func TestAScreenClickIsAddedToAnUncheckedRoutineThroughTheStepPanel(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	e := a.StepEditing(r.ID)
	if len(e.ClickBases) != 0 || !e.ScreenClicks || !strings.Contains(e.ClickSays,
		"only at a point on the screen") {
		t.Fatalf("an unchecked routine offers %+v, screen %v, saying %q", e.ClickBases,
			e.ScreenClicks, e.ClickSays)
	}
	v := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "insert", At: 0, Kind: "click",
		Screen: true, X: 33, Y: 44}})
	if v.Why != "" {
		t.Fatalf("the screen click was refused: %q", v.Why)
	}
	if !strings.Contains(strings.Join(v.Doing, " "), "on the screen") {
		t.Errorf("the review does not say the basis: %v", v.Doing)
	}
}

// A CHECKED ROUTINE WITH NO FULLY MEASURED PRESS OFFERS NEITHER, AND SAYS WHY.
func TestACheckedRoutineWithNoMeasuredWindowSaysNoClickCanBeAdded(t *testing.T) {
	a := aStudioLibrary(t, "shift click", heldShiftIntoNotepad())
	e := a.StepEditing(a.Routines()[0].ID)
	if len(e.ClickBases) != 0 || e.ScreenClicks || !strings.Contains(e.ClickSays,
		"cannot be added here") {
		t.Errorf("it offers %+v, screen %v, saying %q", e.ClickBases, e.ScreenClicks,
			e.ClickSays)
	}
}
