package main

// A STEP'S TEXT AND ITS KEY ARE CHANGED THROUGH THE STEP PANEL'S OWN PATH.
//
// StepEditing lists the row with its text or key, ReviewStepEdit prepares the change and writes
// nothing, SaveStepEdit writes it by the review's token — the same three calls every other step
// change uses. See [[ADR-311-a-steps-text-and-key-can-be-changed]].

import (
	"strings"
	"testing"
)

func rowWithKey(t *testing.T, e StepEditing, key string) StepRow {
	t.Helper()
	for _, r := range e.Rows {
		if r.Keyed && r.Key == key {
			return r
		}
	}
	t.Fatalf("no row offers the key %q: %+v", key, e.Rows)
	return StepRow{}
}

// A PRESS'S KEY IS OFFERED, CHANGED, REVIEWED AND SAVED.
func TestAPressesKeyIsChangedThroughTheStepPanel(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	e := a.StepEditing(r.ID)
	row := rowWithKey(t, e, "esc")
	if row.Holds {
		t.Error("a press is offered as half of a hold")
	}
	v := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "key", At: row.At, Key: "Enter"}})
	if v.Why != "" || v.Token == "" {
		t.Fatalf("the review was not prepared: %q", v.Why)
	}
	if !strings.Contains(v.Now, `"enter"`) || strings.Contains(v.Now, `"esc"`) {
		t.Errorf("the review would write:\n%s", v.Now)
	}
	if src := sourceOnDisk(t, a, r); !strings.Contains(src, `"esc"`) {
		t.Fatal("the review wrote something")
	}
	if done := a.SaveStepEdit(v.Token); done.Why != "" || !done.Saved || done.Partial {
		t.Fatalf("the reviewed change was not saved: %+v", done)
	}
	src := sourceOnDisk(t, a, r)
	if !strings.Contains(src, `"enter"`) || strings.Contains(src, `"esc"`) {
		t.Errorf("the file says:\n%s", src)
	}
	// AND THE PANEL LISTS IT AGAIN, which is the stored account having been written too.
	if again := a.StepEditing(r.ID); again.Why != "" {
		t.Errorf("after the change the panel refuses the play: %s", again.Why)
	}
}

// AND A KEY MARCO CANNOT PRESS IS REFUSED AT REVIEW, WITH NOTHING WRITTEN.
func TestAKeyMarcoCannotPressIsRefusedByTheStepPanel(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	e := a.StepEditing(r.ID)
	row := rowWithKey(t, e, "esc")
	before := sourceOnDisk(t, a, r)
	v := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "key", At: row.At, Key: "hyper"}})
	if v.Why == "" || v.Token != "" {
		t.Fatalf("a key Marco cannot press was prepared for saving: %+v", v)
	}
	if !strings.Contains(v.Why, "not a key Marco can press") {
		t.Errorf("it was refused, saying %q", v.Why)
	}
	if sourceOnDisk(t, a, r) != before {
		t.Error("something was written")
	}
	// AND IT IS A CORRECTION, NOT A STALE LIST: the page keeps what was typed for this one.
	if !v.Correct {
		t.Error("a key Marco cannot press is reported like a list that is no longer " +
			"current, so the page would throw away everything typed")
	}
	// WHERE A STALE LIST IS NOT, and the page lists the steps again.
	stale := a.ReviewStepEdit(r.ID, "a-basis-nobody-was-shown",
		[]StepChange{{Op: "key", At: row.At, Key: "tab"}})
	if stale.Why == "" || stale.Correct {
		t.Errorf("a change against a list that was never shown is reported as a "+
			"correction: %+v", stale)
	}
}

// A HELD KEY IS OFFERED AS ONE KEY ON BOTH HALVES, AND CHANGING ONE CHANGES BOTH.
func TestAHeldKeyIsChangedWholeThroughTheStepPanel(t *testing.T) {
	a := aStudioLibrary(t, "shift click", heldShiftIntoNotepad())
	r := a.Routines()[0]
	e := a.StepEditing(r.ID)
	var halves []StepRow
	for _, row := range e.Rows {
		if row.Keyed && row.Key == "shift" {
			halves = append(halves, row)
		}
	}
	if len(halves) != 2 || !halves[0].Holds || !halves[1].Holds {
		t.Fatalf("the hold is not offered as two halves of one key: %+v", halves)
	}
	v := a.ReviewStepEdit(r.ID, e.Basis,
		[]StepChange{{Op: "key", At: halves[0].At, Key: "ctrl"}})
	if v.Why != "" {
		t.Fatalf("the review was refused: %q", v.Why)
	}
	if strings.Contains(v.Now, `"shift"`) {
		t.Errorf("one half of the hold still uses shift:\n%s", v.Now)
	}
	if !strings.Contains(strings.Join(v.Doing, " "), "two halves of one hold") {
		t.Errorf("the review does not say both halves change: %v", v.Doing)
	}
}

// THE TEXT A STEP TYPES IS OFFERED, CHANGED, REVIEWED AND SAVED.
func TestAStepsTextIsChangedThroughTheStepPanel(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	// THE TWO LETTERS ARE KEY PRESSES UNTIL THEY ARE SAID TO BE TYPED TEXT, which is
	// itself a step change and is saved first.
	e := a.StepEditing(r.ID)
	from, to := rowWithKey(t, e, "a").At, rowWithKey(t, e, "b").At
	typed := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "typed", At: from, Upto: to}})
	if typed.Why != "" {
		t.Fatalf("declaring the letters typed text was refused: %q", typed.Why)
	}
	if done := a.SaveStepEdit(typed.Token); !done.Saved {
		t.Fatalf("declaring them typed text was not saved: %+v", done)
	}
	e = a.StepEditing(r.ID)
	at := -1
	for _, row := range e.Rows {
		if row.Types && row.Text == "ab" {
			at = row.At
		}
	}
	if at < 0 {
		t.Fatalf("no row offers the typed text: %+v", e.Rows)
	}
	v := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "text", At: at, Text: "hello "}})
	if v.Why != "" || v.Token == "" {
		t.Fatalf("the review was not prepared: %q", v.Why)
	}
	if done := a.SaveStepEdit(v.Token); !done.Saved {
		t.Fatalf("the new text was not saved: %+v", done)
	}
	src := sourceOnDisk(t, a, r)
	if !strings.Contains(src, `"hello "`) {
		t.Errorf("the file does not type the new text, trailing space and all:\n%s", src)
	}
}
