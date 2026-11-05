package main

// A STEP IS ADDED THROUGH THE STEP PANEL'S OWN PATH: StepEditing, ReviewStepEdit, SaveStepEdit.
// See [[ADR-312-a-step-can-be-added]].

import (
	"strings"
	"testing"
)

func TestAStepIsAddedThroughTheStepPanel(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	e := a.StepEditing(r.ID)
	last := e.Rows[len(e.Rows)-1].At
	v := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{
		{Op: "insert", At: 0, Kind: "key", Key: "F5"},
		{Op: "insert", At: last, Kind: "type", Text: "done"},
	})
	if v.Why != "" || v.Token == "" {
		t.Fatalf("the review was not prepared: %q", v.Why)
	}
	if len(v.Rows) != len(e.Rows)+2 {
		t.Errorf("the reviewed list has %d steps, %d before and two added", len(v.Rows),
			len(e.Rows))
	}
	if done := a.SaveStepEdit(v.Token); !done.Saved || done.Partial {
		t.Fatalf("the additions were not saved: %+v", done)
	}
	src := sourceOnDisk(t, a, r)
	if !strings.Contains(src, `Key with "f5"`) || !strings.Contains(src, `Type with "done"`) {
		t.Errorf("the file does not do what was added:\n%s", src)
	}
	if again := a.StepEditing(r.ID); again.Why != "" || len(again.Rows) != len(e.Rows)+2 {
		t.Errorf("after the additions the panel lists %d steps, or refuses: %q",
			len(again.Rows), again.Why)
	}
}

// AND ONE PUT BEFORE THE WINDOW COMES FORWARD IS A CORRECTION, NOTHING WRITTEN.
func TestAStepBeforeTheWindowComesForwardIsRefusedByTheStepPanel(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	e := a.StepEditing(r.ID)
	before := sourceOnDisk(t, a, r)
	v := a.ReviewStepEdit(r.ID, e.Basis,
		[]StepChange{{Op: "insert", At: -1, Kind: "key", Key: "a"}})
	if v.Why == "" || !v.Correct {
		t.Fatalf("a press before the window comes forward was not refused as a "+
			"correction: %+v", v)
	}
	if !strings.Contains(v.Why, "forward") {
		t.Errorf("it was refused, saying %q", v.Why)
	}
	if sourceOnDisk(t, a, r) != before {
		t.Error("something was written")
	}
}
