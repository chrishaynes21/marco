package main

// MOVING A PRESS FROM THE PANEL, AGAINST A REAL LIBRARY.
//
// The rules are `internal/orchestrator`'s and are held there. What is asked here is this
// surface's own promises: that the row says where the press is and in which basis, that the
// review shows the moved point before anything is written, and that what lands on disk still
// carries every term the guard is made of.
//
// THE GUARD TERMS ARE THE POINT OF THE LAST ONE. `clickingInNotepad` records a press that asks to
// be checked against the window it was made in; a move that dropped one of those terms would
// produce a routine that looks right in the panel and refuses every press.
//
// See [[ADR-286-a-press-can-be-moved]].

import (
	"strings"
	"testing"
)

// aRoutineWithAMeasuredPress is a saved routine whose click carries the five window terms.
func aRoutineWithAMeasuredPress(t *testing.T) (*Acts, Routine) {
	t.Helper()
	a := aStudioLibrary(t, "press the thing", clickingInNotepad())
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routines", len(rs))
	}
	for _, want := range []string{"RelTo", "WinW 1280", "Dpi 96", "notepad.exe"} {
		if !strings.Contains(rs[0].Source, want) {
			t.Fatalf("the fixture's press is not measured in a window (%q):\n%s",
				want, rs[0].Source)
		}
	}
	return a, rs[0]
}

// THE ROW OFFERS THE OFFSET, THE REVIEW SHOWS BOTH NUMBERS MOVED, AND THE GUARD SURVIVES.
func TestAMeasuredPressCanBeMovedAndKeepsWhatItIsCheckedAgainst(t *testing.T) {
	a, r := aRoutineWithAMeasuredPress(t)
	e := a.StepEditing(r.ID)
	if e.Why != "" {
		t.Fatalf("the routine cannot be edited: %s", e.Why)
	}
	at := -1
	for _, row := range e.Rows {
		if row.Point {
			at = row.At
		}
	}
	if at < 0 {
		t.Fatalf("no row presses anywhere: %+v", e.Rows)
	}
	row := e.Rows[at]
	if !row.Movable || !row.InWindow {
		t.Fatalf("the press is not offered as movable inside its window: %+v", row)
	}
	if row.X != 120 || row.Y != 90 {
		t.Fatalf("the pair offered is %d, %d, and the OFFSET recorded was 120, 90",
			row.X, row.Y)
	}
	for _, want := range []string{"left", "1280×800", "96 dpi", "notepad.exe"} {
		if !strings.Contains(row.Terms, want) {
			t.Errorf("the row does not show %q beside the numbers, which is what "+
				"the edit keeps: %q", want, row.Terms)
		}
	}

	p := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{
		{Op: "point", At: at, X: 200, Y: 150}})
	if p.Why != "" || p.Refused != "" {
		t.Fatalf("moving the press refused: %s %s", p.Why, p.Refused)
	}
	if !p.Savable {
		t.Fatalf("the review produced nothing to save: %+v", p)
	}
	// THE OFFSET MOVED BY 80, 60, SO THE SCREEN POINT DID TOO: 400, 300 → 480, 360.
	for _, want := range []string{"RelX 200", "RelY 150", "X 480", "Y 360"} {
		if !strings.Contains(p.Now, want) {
			t.Errorf("the version offered does not say %q:\n%s", want, p.Now)
		}
	}
	// ── AND NOTHING IS WRITTEN UNTIL IT IS ──
	if strings.Contains(a.Routines()[0].Source, "RelX 200") {
		t.Fatal("reviewing the move wrote to the library")
	}

	if done := a.SaveStepEdit(p.Token); done.Why != "" || !done.Saved {
		t.Fatalf("saving the move refused: %+v", done)
	}
	after := a.Routines()[0].Source
	// ── EVERY TERM THE GUARD IS MADE OF IS STILL THERE ──
	for _, want := range []string{"RelX 200", "RelY 150", "X 480", "Y 360",
		`RelTo "window"`, "WinW 1280", "WinH 800", "Dpi 96", `Process "notepad.exe"`} {
		if !strings.Contains(after, want) {
			t.Errorf("the saved routine lost %q, so a press that was checked "+
				"against its window is not any more:\n%s", want, after)
		}
	}
	// AND IT CAN BE EDITED AGAIN, which is what the step account being rewritten buys.
	if again := a.StepEditing(a.Routines()[0].ID); again.Why != "" {
		t.Errorf("the moved routine cannot be edited again: %s", again.Why)
	}
}

// AND A PRESS THE PANEL WILL NOT MOVE SAYS WHY, ON ITS ROW.
//
// Through the real contract rather than a stubbed row: the refusal a person reads and the
// refusal the validator gives have to be one sentence, or the row offers what the press then
// declines.
func TestAPressThatCannotBeMovedSaysSoOnItsRow(t *testing.T) {
	a, r := aRoutineWithAMeasuredPress(t)
	e := a.StepEditing(r.ID)
	at := -1
	for _, row := range e.Rows {
		if row.Point {
			at = row.At
		}
	}
	if at < 0 {
		t.Fatal("no row presses anywhere")
	}
	// AN OFFSET OUTSIDE THE WINDOW IT WAS MEASURED IN.
	p := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{
		{Op: "point", At: at, X: 5000, Y: 5000}})
	if p.Why == "" {
		t.Fatal("a press was moved outside the window it is checked against")
	}
	if !strings.Contains(p.Why, "outside it") {
		t.Errorf("the refusal does not say why: %q", p.Why)
	}
	// A STEP THAT PRESSES NOWHERE.
	wait := -1
	for _, row := range e.Rows {
		if row.Wait {
			wait = row.At
		}
	}
	if wait >= 0 {
		q := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{
			{Op: "point", At: wait, X: 1, Y: 2}})
		if q.Why == "" {
			t.Error("a wait was given a position")
		}
	}
	// AND NOTHING WAS WRITTEN BY EITHER.
	if !strings.Contains(a.Routines()[0].Source, "RelX 120") {
		t.Error("a refused move changed the routine")
	}
}

// AND A BASIS THE LIST NO LONGER HAS IS REFUSED, FOR A MOVE AS FOR EVERY OTHER CHANGE.
//
// Positions alone are not consent. A move names a step by its row number, and between the list
// being drawn and the button being pressed anything can write this routine.
func TestAMoveCountedAgainstAListThatHasMovedIsRefused(t *testing.T) {
	a, r := aRoutineWithAMeasuredPress(t)
	e := a.StepEditing(r.ID)
	at := -1
	for _, row := range e.Rows {
		if row.Point {
			at = row.At
		}
	}
	if at < 0 {
		t.Fatal("no row presses anywhere")
	}
	// THE ROUTINE IS WRITTEN SINCE, through the panel itself.
	first := a.ReviewStepEdit(r.ID, e.Basis, []StepChange{
		{Op: "point", At: at, X: 200, Y: 150}})
	if done := a.SaveStepEdit(first.Token); !done.Saved {
		t.Fatalf("the first move refused: %+v", done)
	}
	// AND THE OLD BASIS IS OFFERED AGAIN.
	stale := a.ReviewStepEdit(a.Routines()[0].ID, e.Basis, []StepChange{
		{Op: "point", At: at, X: 300, Y: 250}})
	if stale.Why == "" {
		t.Fatal("a move was applied against a list that had moved under it")
	}
	if !strings.Contains(a.Routines()[0].Source, "RelX 200") {
		t.Error("the refused move changed the routine")
	}
}
