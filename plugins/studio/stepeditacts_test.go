package main

// THE BACKEND BEHIND THE STEP EDITOR, WITH A REAL LIBRARY.
//
// The page test next door renders the window against a stub, which answers whether the branches in
// the page take. This asks the other half: whether the `Acts` methods the page calls offer what can
// be done, refuse what cannot, and never let the page state a change against a list it was not
// shown.
//
// The contract itself — the basis, the step list the saved source is the render of, the
// whole-gesture removal, the compile gate, the route's lock, the three digests re-read, the account
// written last — is held in `internal/orchestrator` and these do not restate it. What is asked
// here is this surface's own promises.
//
// See [[ADR-273-a-routine-writes-down-what-it-does]].

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// pausedTypingIntoNotepad presses two keys with a real pause between them, so the step list has a
// wait in it that the generator writes down.
//
// Nothing in it checks the window it was made in, which keeps it out of the one-scope corner and
// makes the fixture the same shape as the move tests' one.
func pausedTypingIntoNotepad() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "a", Down: true, T: at(600), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "a", Down: false, T: at(610), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "b", Down: true, T: at(2600), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "b", Down: false, T: at(2610), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(3200), App: "notepad"},
	}
}

// heldShiftIntoNotepad holds shift across a press and lets it go, so the step list carries the one
// gesture that is two steps.
func heldShiftIntoNotepad() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "shift", Down: true, T: at(200), App: "notepad"},
		// HELD ACROSS A PRESS THAT IS NOT A KEY, which is what `simplify` writes down
		// as a keydown and a later keyup: shift across another key folds into one
		// chord instead, and a chord is one step with nothing to pair.
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 640, Y: 480,
			App: "notepad", PID: gamePID, RelTo: "window", RelX: 40, RelY: 80,
			WinW: 1920, WinH: 1080, Dpi: 96, Process: "notepad.exe",
			T: at(700)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 640, Y: 480,
			App: "notepad", PID: gamePID, T: at(710)},
		{Kind: recorder.EvKey, KeyName: "shift", Down: false, T: at(1400), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(2000), App: "notepad"},
	}
}

// anEditableRoutineInStudio is one routine with a wait in it, in a library this window reads.
func anEditableRoutineInStudio(t *testing.T) (*Acts, Routine) {
	t.Helper()
	a := aStudioLibrary(t, "type two letters", pausedTypingIntoNotepad())
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d plays, want the one saved", len(rs))
	}
	return a, rs[0]
}

// aWaitRow is the first row the panel offers a duration on, which is the one every change below
// is made to.
func aWaitRow(t *testing.T, e StepEditing) StepRow {
	t.Helper()
	for _, r := range e.Rows {
		if r.Wait {
			return r
		}
	}
	t.Fatalf("no row in this fixture offers a duration: %+v", e.Rows)
	return StepRow{}
}

// THE PANEL LISTS EVERY STEP, WITH WHAT CAN BE DONE TO EACH.
//
// Every one of them, in order, because the positions a change names mean something only against
// the whole list. A wait offers a duration; everything else offers removal and nothing more.
func TestTheStepPanelListsEveryStepWithWhatCanBeDoneToIt(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	e := a.StepEditing(r.ID)
	if e.Why != "" {
		t.Fatalf("the panel refused a play saved by the production path: %s", e.Why)
	}
	if !e.Editable || len(e.Rows) == 0 {
		t.Fatalf("nothing was offered: %+v", e)
	}
	if e.Basis == "" {
		t.Error("the list came with no basis, so nothing could be asked against it")
	}
	// THE POSITIONS ARE THE LIST'S OWN, in order and with none missing.
	for i, row := range e.Rows {
		if row.At != i {
			t.Errorf("row %d says it is step %d", i, row.At)
		}
		if strings.TrimSpace(row.Says) == "" {
			t.Errorf("step %d has no words", i+1)
		}
	}
	waits := 0
	for _, row := range e.Rows {
		if row.Wait {
			waits++
			if row.Ms <= 0 {
				t.Errorf("step %d offers a duration of %dms", row.At+1, row.Ms)
			}
		}
	}
	if waits == 0 {
		t.Error("no row offers a duration, so this fixture says nothing about the " +
			"control that changes one")
	}
}

// AND A HELD KEY SAYS ON ITS OWN ROW THAT REMOVING IT REMOVES ITS OTHER HALF.
//
// Before the press rather than in the review afterwards: that is a removal of two steps, and
// somebody pressing one row is owed the second one while they can still decide not to.
func TestTheStepPanelSaysWhichRemovalsTakeTwoSteps(t *testing.T) {
	a := aStudioLibrary(t, "hold shift", heldShiftIntoNotepad())
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d plays", len(rs))
	}
	e := a.StepEditing(rs[0].ID)
	if e.Why != "" {
		t.Fatalf("the panel refused: %s", e.Why)
	}
	paired := 0
	for _, row := range e.Rows {
		if !row.Pairs {
			continue
		}
		paired++
		if row.PairsAt == row.At {
			t.Errorf("step %d says it pairs with itself", row.At+1)
		}
		if !strings.Contains(row.PairSays, "shift") {
			t.Errorf("step %d does not name the key it is half of: %q",
				row.At+1, row.PairSays)
		}
		if !strings.Contains(row.PairSays, "two halves") {
			t.Errorf("step %d does not say removing it removes two: %q",
				row.At+1, row.PairSays)
		}
	}
	if paired != 2 {
		t.Errorf("%d rows report a partner; a held key is two steps and both of them "+
			"have one:\n%+v", paired, e.Rows)
	}
}

// A CHANGE WITHOUT THE LIST IT WAS READ OFF IS REFUSED.
//
// A caller with no basis has not been shown the steps its positions refer to. Refused here rather
// than passed through, because the backend cannot tell whether a caller had a list on a screen and
// this can.
func TestTheStepPanelRefusesChangesWithoutTheListTheyWereReadOff(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	e := a.StepEditing(r.ID)
	row := aWaitRow(t, e)

	v := a.ReviewStepEdit(r.ID, "", []StepChange{{Op: "wait", At: row.At, To: 800}})
	if v.Why == "" {
		t.Fatal("a change was reviewed against no list at all")
	}
	if !strings.Contains(v.Why, "list the steps before") {
		t.Errorf("the refusal does not say what is missing: %q", v.Why)
	}
	if v.Token != "" {
		t.Error("a review was prepared anyway")
	}
	// AND NOTHING WAS ASKED IS ITS OWN ANSWER, not a review of no operations.
	if none := a.ReviewStepEdit(r.ID, e.Basis, nil); none.Why == "" {
		t.Error("a review with no changes in it was prepared")
	}
}

// AND A ROUTINE WRITTEN SINCE ITS STEPS WERE LISTED REFUSES THE CHANGES COUNTED AGAINST THEM.
//
// THE CASE THIS RULE EXISTS FOR, and it is not the same as a change after the review. Between the
// panel listing the steps and somebody pressing Review, another writer can insert or remove one —
// a re-recording, a hand edit, a second window's edit — and every position after it moves. Row 3
// of a list nobody saw is somebody's intention applied to a different step, and the result is a
// perfectly good routine that does the wrong thing.
//
// Nothing here can translate those positions, so nothing tries.
func TestChangesAreRefusedWhenTheRoutineWasWrittenSinceItsStepsWereListed(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	e := a.StepEditing(r.ID)
	row := aWaitRow(t, e)
	asked := []StepChange{{Op: "wait", At: row.At, To: 800}}

	// ── ANOTHER WRITER, BETWEEN THE LISTING AND THE REVIEW ──
	//
	// The same name demonstrated again, with a press at the front, so every position
	// in the list this panel is holding has moved by two.
	rt := r.route()
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	longer := append([]recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "z", Down: true, T: at(300), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "z", Down: false, T: at(310), App: "notepad"},
	}, pausedTypingIntoNotepad()[1:]...)
	dr := d.DraftOf("type two letters", longer, "notepad", "a second recording",
		orchestrator.Unstated)
	if _, err := d.Save(dr, orchestrator.ScopeAppAnywhere, "notepad"); err != nil {
		t.Fatalf("the second recording could not be saved: %v", err)
	}

	v := a.ReviewStepEdit(r.ID, e.Basis, asked)
	if v.Why == "" {
		t.Fatalf("the changes were applied to a list nobody was shown:\n%s", v.Now)
	}
	if !strings.Contains(v.Why, "has been written since its steps were listed") {
		t.Errorf("the refusal does not say what happened: %q", v.Why)
	}
	if v.Token != "" {
		t.Error("a review was prepared anyway")
	}
	// AND THE SAME CHANGES AGAINST THE LIST AS IT NOW STANDS ARE FINE, which is what
	// makes this a refusal about the basis and not about the operations.
	again := a.StepEditing(r.ID)
	if again.Basis == e.Basis {
		t.Fatal("the basis did not move, so this test asks nothing")
	}
	ok := a.ReviewStepEdit(r.ID, again.Basis, []StepChange{
		{Op: "wait", At: aWaitRow(t, again).At, To: 800}})
	if ok.Why != "" {
		t.Errorf("the changes were refused against the list they were read off: %q",
			ok.Why)
	}
	if !ok.Savable {
		t.Errorf("that review cannot be saved: refused %q", ok.Refused)
	}
	// AND THE ROUTINE IS UNTOUCHED BY EITHER OF THEM, because reviewing writes nothing.
	if _, st := a.reg().Steps(rt); st != routes.StepsNone {
		t.Errorf("reviewing wrote a step list (%q)", st)
	}
}

// SAVING TAKES THE REVIEW'S TOKEN AND NOTHING ELSE.
//
// Not the routine's identity, which two reviews of one routine share, and no part of the source:
// the three digests the review was bound to are in this window, and a page that handed the
// rendered source back could hand back something else.
func TestAStepEditIsSavedByItsReviewsTokenAlone(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	e := a.StepEditing(r.ID)
	row := aWaitRow(t, e)
	v := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "wait", At: row.At, To: 800}})
	if v.Why != "" || v.Token == "" {
		t.Fatalf("the review was not prepared: %q", v.Why)
	}
	if !strings.Contains(v.Now, "Sleep with 800") {
		t.Errorf("the review does not show the change it would write:\n%s", v.Now)
	}
	if v.Was == v.Now || !v.Changes {
		t.Error("the review reports no difference between the two sources")
	}
	if len(v.Doing) == 0 {
		t.Error("the review does not say what it would do")
	}

	// A TOKEN THIS WINDOW NEVER MINTED WRITES NOTHING.
	if done := a.SaveStepEdit("tok-made-up"); done.Why == "" || done.Saved {
		t.Errorf("a token nobody prepared was saved with: %+v", done)
	}
	if src := sourceOnDisk(t, a, r); strings.Contains(src, "Sleep with 800") {
		t.Error("something was written")
	}

	done := a.SaveStepEdit(v.Token)
	if done.Why != "" || !done.Saved || done.Partial {
		t.Fatalf("the reviewed edit was not saved: %+v", done)
	}
	if !strings.Contains(done.Says, "one change to its steps") {
		t.Errorf("the outcome does not say what it did: %q", done.Says)
	}
	if !strings.Contains(sourceOnDisk(t, a, r), "Sleep with 800") {
		t.Error("the edit did not reach the file")
	}
	// AND IT IS SPENT, IN THIS WINDOW'S OWN WORDS.
	//
	// A MUTATION FOUND THIS. The assertion used to be that a second press was refused,
	// and dropping the review changed nothing: the contract underneath refuses a moment
	// later anyway, because the source has moved since the review was prepared. So what
	// is asked is which layer answered — this window letting go of a spent review, rather
	// than a revalidation catching it on the way to the disk.
	again := a.SaveStepEdit(v.Token)
	if again.Why == "" || again.Saved {
		t.Fatalf("the same review was saved twice: %+v", again)
	}
	if !strings.Contains(again.Why, "no longer the one this window holds") {
		t.Errorf("the second press was refused by something other than this window "+
			"letting go of a spent review: %q", again.Why)
	}
}

// CANCELLING WRITES NOTHING, AND LETS GO OF THE REVIEW.
//
// So a second press of a button somebody already dismissed cannot save with it.
func TestCancellingAStepEditWritesNothingAndCannotBeSavedAfterwards(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	e := a.StepEditing(r.ID)
	row := aWaitRow(t, e)
	v := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "wait", At: row.At, To: 800}})
	if v.Token == "" {
		t.Fatalf("no review: %q", v.Why)
	}
	before := sourceOnDisk(t, a, r)

	a.CancelStepEdit(v.Token)
	if done := a.SaveStepEdit(v.Token); done.Why == "" || done.Saved {
		t.Errorf("a cancelled review was saved: %+v", done)
	}
	if sourceOnDisk(t, a, r) != before {
		t.Error("the play changed")
	}
	if a.reg().HasSteps(r.route()) {
		t.Error("a step list was written")
	}
}

// A SOURCE THAT LANDED WITHOUT ITS STEP LIST IS NEITHER A SUCCESS NOR A REFUSAL.
//
// The routine IS the edited one and it runs, because execution reads only the `.marco`; what
// failed is the explanation beside it, so the next edit of it will refuse until it has been
// rebuilt. Reported as the third answer rather than folded into either of the other two.
func TestAStepEditThatLandedWithoutItsStepListSaysSo(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	// ONE EDIT FIRST, so the step list exists, and then nothing can be written over it.
	e := a.StepEditing(r.ID)
	row := aWaitRow(t, e)
	first := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "wait", At: row.At, To: 700}})
	if done := a.SaveStepEdit(first.Token); !done.Saved {
		t.Fatalf("the first edit was not saved: %+v", done)
	}
	at := a.reg().StepsPath(r.route())
	if err := os.Chmod(at, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(at, 0o644) })

	second := a.StepEditing(r.ID)
	v := a.ReviewStepEdit(r.ID, second.Basis,
		[]StepChange{{Op: "wait", At: aWaitRow(t, second).At, To: 800}})
	if v.Token == "" {
		t.Fatalf("the second review was not prepared: %q", v.Why)
	}
	done := a.SaveStepEdit(v.Token)
	switch {
	case done.Why != "":
		t.Fatalf("it was reported as nothing written, which would tell somebody "+
			"their edit did not happen: %q", done.Why)
	case !done.Partial:
		t.Fatalf("it was reported as an ordinary save: %+v", done)
	case !done.Saved:
		t.Error("it does not say the play was saved, which it was")
	}
	if !strings.Contains(done.Says, "not written down beside it") {
		t.Errorf("the outcome does not say what did not land: %q", done.Says)
	}
	// THE ROUTINE IS THE EDITED ONE.
	if !strings.Contains(sourceOnDisk(t, a, r), "Sleep with 800") {
		t.Error("the edit did not reach the file")
	}
	// AND THE NEXT EDIT REFUSES, which is what somebody has to be told about.
	if next := a.StepEditing(r.ID); next.Why == "" {
		t.Error("the panel offered an edit of a play whose step list no longer " +
			"describes it")
	}
}

// AND NOTHING IS OFFERED WHILE A DEMONSTRATION IS BEING RECORDED.
//
// The recorder owns the library's attention, which is the rule every other write in this window
// obeys.
func TestTheStepPanelHoldsOffWhileRecording(t *testing.T) {
	a, r := anEditableRoutineInStudio(t)
	e := a.StepEditing(r.ID)
	row := aWaitRow(t, e)
	v := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{{Op: "wait", At: row.At, To: 800}})
	if v.Token == "" {
		t.Fatalf("no review: %q", v.Why)
	}

	a.mu.Lock()
	a.learn = &learning{name: "something else"}
	a.mu.Unlock()

	for _, c := range []struct {
		what string
		why  string
	}{
		{"listing the steps", a.StepEditing(r.ID).Why},
		{"reviewing a change", a.ReviewStepEdit(r.ID, e.Basis,
			[]StepChange{{Op: "remove", At: row.At}}).Why},
		{"saving one", a.SaveStepEdit(v.Token).Why},
	} {
		if !strings.Contains(c.why, "one thing at a time") {
			t.Errorf("%s during a recording said %q", c.what, c.why)
		}
	}
}

// sourceOnDisk is the routine as the library holds it now.
func sourceOnDisk(t *testing.T, a *Acts, r Routine) string {
	t.Helper()
	raw, err := os.ReadFile(a.reg().Path(r.route()))
	if err != nil {
		t.Fatalf("reading %s back: %v", r.ID, err)
	}
	return string(raw)
}
