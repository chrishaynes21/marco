package main

// A STEP IS REPEATED THROUGH THE STEP PANEL'S OWN PATH. See [[ADR-313-steps-can-be-repeated]].

import (
	"strings"
	"testing"
)

// A KEY PRESS'S COUNT IS OFFERED, CHANGED AND SAVED; A HOLD'S HALVES OFFER NONE.
func TestAKeyPressesCountIsChangedThroughTheStepPanel(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	e := a.StepEditing(r.ID)
	row := rowWithKey(t, e, "esc")
	if !row.Repeats || row.Count != 1 {
		t.Fatalf("a key press is not offered a count of one: %+v", row)
	}
	v := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "repeat", At: row.At, Count: 3}})
	if v.Why != "" || v.Token == "" {
		t.Fatalf("the review was not prepared: %q", v.Why)
	}
	if done := a.SaveStepEdit(v.Token); !done.Saved {
		t.Fatalf("the count was not saved: %+v", done)
	}
	if src := sourceOnDisk(t, a, r); !strings.Contains(src, "repeat 3 times") &&
		strings.Count(src, `Key with "esc"`) != 3 {
		t.Errorf("the file does not press esc three times:\n%s", src)
	}
	again := a.StepEditing(r.ID)
	if got := rowWithKey(t, again, "esc"); got.Count != 3 {
		t.Errorf("after the change the panel offers a count of %d", got.Count)
	}
}

// A HOLD'S HALVES HAVE NO COUNT, AND A ROUTINE WHOSE PRESSES ARE CHECKED SAYS WHY THERE IS NO BLOCK.
func TestAGuardedRoutineOffersNoBlockAndSaysWhy(t *testing.T) {
	a := aStudioLibrary(t, "shift click", heldShiftIntoNotepad())
	r := a.Routines()[0]
	e := a.StepEditing(r.ID)
	for _, row := range e.Rows {
		if row.Holds && row.Repeats {
			t.Errorf("half of a hold is offered a count: %+v", row)
		}
	}
	if e.Blocks || !strings.Contains(e.BlocksSays, "checks each press") {
		t.Errorf("a routine whose presses are checked offers blocks %v, saying %q",
			e.Blocks, e.BlocksSays)
	}
}

// AN UNCHECKED ROUTINE'S STEPS ARE REPEATED AS A BLOCK, SAVED, AND LISTED AS ONE.
func TestStepsAreRepeatedAsABlockThroughTheStepPanel(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	e := a.StepEditing(r.ID)
	if !e.Blocks || e.BlocksSays != "" {
		t.Fatalf("a routine whose presses are not checked offers no block: %v %q",
			e.Blocks, e.BlocksSays)
	}
	from, to := rowWithKey(t, e, "a").At, rowWithKey(t, e, "b").At
	v := a.ReviewStepEdit(r.ID, e.Basis,
		[]StepChange{{Op: "loop", At: from, Upto: to, Count: 2}})
	if v.Why != "" || v.Token == "" {
		t.Fatalf("the block was not prepared: %q", v.Why)
	}
	if done := a.SaveStepEdit(v.Token); !done.Saved {
		t.Fatalf("the block was not saved: %+v", done)
	}
	if src := sourceOnDisk(t, a, r); !strings.Contains(src, "repeat 2 times") {
		t.Errorf("the file has no block:\n%s", src)
	}
	listed := a.StepEditing(r.ID)
	found := false
	for _, row := range listed.Rows {
		if strings.HasPrefix(row.Says, "repeat 2 times: ") && row.Repeats && row.Count == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("the block is not listed as one row that says what it repeats: %+v",
			listed.Rows)
	}
}
