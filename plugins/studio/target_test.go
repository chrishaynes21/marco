package main

// WHICH WINDOW A ROUTINE WOULD BE PLAYED INTO, ASKED OF THE BACKEND.
//
// The page's half is `targetpage_test.go`, rendered. This is the other half: what the window
// OFFERS, and what it refuses at the press. A greyed-out button proves nothing about a backend
// that would have obliged, and a refusing backend proves nothing about a button that offers it.
//
// The desktop is STATED in every case. A live reading makes these pass or fail on what the owner
// happens to have open, which is the trap `chosenwindow_test.go` records at length — and on a
// machine with no desktop at all it would reduce to asserting that a list is a list.
//
// See [[ADR-272-the-window-a-run-acts-in-is-chosen-and-carried]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// twoWindowsOfArk is a desktop with two windows of one program and one of another.
//
// The third is what makes the filtering a claim: a routine about `arkascended` offered a Discord
// window would be offering a press its own guard refuses.
func twoWindowsOfArk(t *testing.T) {
	t.Helper()
	orchestrator.WindowsForTest(t, []winctx.LiveWindow{
		{Handle: 0x1010, ProcessID: 1010, Image: "arkascended", Title: "first",
			Visible: true, OnScreen: true},
		{Handle: 0x2020, ProcessID: 2020, Image: "arkascended", Title: "second",
			Visible: true, OnScreen: true},
		{Handle: 0x3030, ProcessID: 3030, Image: "discord", Title: "chat",
			Visible: true, OnScreen: true},
	})
	orchestrator.ForegroundForTest(t, 0x1010, true)
}

// TWO WINDOWS OF ONE PROGRAM ARE OFFERED AND NEITHER IS CHOSEN FOR SOMEBODY.
//
// The case the whole slice is about. Choosing between two windows of one program is not this
// window's decision: they are two Explorer folders, two browser profiles, two copies of a game,
// and nothing here can tell which one somebody meant. A surface that picked the biggest or the
// first would be making that decision silently, which is what both halves of the old path did.
func TestTwoWindowsOfOneProgramAreOfferedAndNeitherIsChosen(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	twoWindowsOfArk(t)
	r, found := routineCalled(a.Routines(), "ark graphics")
	if !found {
		t.Fatalf("it is not listed: %+v", a.Routines())
	}

	into := a.PlaybackTarget(r.ID)
	if into.Why != "" {
		t.Fatalf("a desktop with two of its windows on it offered nowhere to "+
			"press: %+v", into)
	}
	if len(into.Windows) != 2 {
		t.Fatalf("%d window(s) offered, want the two of that program and not the "+
			"one of another: %+v", len(into.Windows), into.Windows)
	}
	if into.Chosen != "" {
		t.Errorf("a window was chosen out of two: %q", into.Chosen)
	}
	if !strings.Contains(into.Choose, "2 windows") {
		t.Errorf("nothing asks for a choice: %q", into.Choose)
	}
	// AND EACH ROW SAYS WHAT CHOOSING *IT* HANDS OVER, naming its own window. One
	// sentence for the list named whichever window the backend had selected, so choosing
	// the other left the right words about the wrong window.
	for _, w := range into.Windows {
		if !strings.Contains(w.Says, w.Label) {
			t.Errorf("the row for %q says %q, which is about another window",
				w.Label, w.Says)
		}
		if !strings.Contains(w.Says, "checked against that window") {
			t.Errorf("a guarded play's row does not say its presses are "+
				"checked: %q", w.Says)
		}
	}
	// AND PLAYING WITHOUT CHOOSING IS REFUSED rather than resolved to the first.
	withAMarcoBeside(t)
	lifted := 0
	a.lifting = func(uintptr, uint32) (bool, bool) { lifted++; return true, true }
	why := a.PlayRoutine(r.ID, "")
	if why == "" {
		t.Fatal("a play was played into one of two windows with nobody having " +
			"chosen which")
	}
	if !strings.Contains(why, "choose") {
		t.Errorf("the refusal reads: %s", why)
	}
	if lifted != 0 {
		t.Errorf("a window was brought forward for a refused Play (%d)", lifted)
	}
}

// AND THE OTHER PROGRAM'S WINDOW IS NOT OFFERED, NOR ACCEPTED.
//
// Two claims because the second is not the first: a surface can filter its list correctly and
// still accept whatever term arrives, and the term arrives from a page whose list is as old as its
// last ask.
func TestAWindowOfAnotherProgramIsNeitherOfferedNorAccepted(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	twoWindowsOfArk(t)
	r, _ := routineCalled(a.Routines(), "ark graphics")

	discord := orchestrator.Pickable{HWnd: 0x3030, PID: 3030}.Term()
	for _, w := range a.PlaybackTarget(r.ID).Windows {
		if w.ID == discord {
			t.Error("a play recorded in one program is offered another " +
				"program's window")
		}
	}
	withAMarcoBeside(t)
	lifted := 0
	a.lifting = func(uintptr, uint32) (bool, bool) { lifted++; return true, true }
	if why := a.PlayRoutine(r.ID, discord); why == "" {
		t.Fatal("a play was played into a window of a program it is not about")
	}
	if lifted != 0 {
		t.Errorf("a window was brought forward for a refused Play (%d)", lifted)
	}
}

// AND A WINDOW THAT HAS CLOSED SINCE THE PAGE DREW ITS LIST IS REFUSED.
//
// The term is resolved AGAIN at the press, out of the list as it is then. The list the page was
// showing is as old as its last ask, and between then and the click a window can close or its
// handle be handed back out — so a term naming nothing open is a refusal rather than a handle
// passed on to a child.
func TestAWindowThatHasClosedSinceTheListWasDrawnIsRefused(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	twoWindowsOfArk(t)
	r, _ := routineCalled(a.Routines(), "ark graphics")
	chosen := orchestrator.Pickable{HWnd: 0x2020, PID: 2020}.Term()

	// AND NOW IT IS GONE, which is what happens while somebody reads the panel.
	orchestrator.WindowsForTest(t, []winctx.LiveWindow{
		{Handle: 0x1010, ProcessID: 1010, Image: "arkascended", Title: "first",
			Visible: true, OnScreen: true},
	})
	withAMarcoBeside(t)
	lifted := 0
	a.lifting = func(uintptr, uint32) (bool, bool) { lifted++; return true, true }
	why := a.PlayRoutine(r.ID, chosen)
	if why == "" {
		t.Fatal("a play was played into a window that is no longer open, which " +
			"with a reused handle is a press in somebody else's window")
	}
	if !strings.Contains(why, "no longer open") {
		t.Errorf("the refusal reads: %s", why)
	}
	if lifted != 0 {
		t.Errorf("a window was brought forward for a refused Play (%d)", lifted)
	}
	// AND THE ONE THAT IS STILL THERE IS STILL PLAYABLE, so this is a refusal about one
	// window and not about the routine.
	if into := a.PlaybackTarget(r.ID); into.Chosen == "" {
		t.Errorf("the remaining window is not offered: %+v", into)
	}
}

// AND NOTHING IS BROUGHT FORWARD FOR A RUN THAT WAS NEVER GOING TO START.
//
// Raising is the first thing a Play does that touches somebody's machine: it brings a window to
// the front over whatever they were doing. A run already in flight, a recording in progress and a
// window that is closing are all decidable without that, so they are asked first — the ordering
// the Act path has carried in its own words since it was written.
func TestNothingIsBroughtForwardForARunThatCannotStart(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	aWindowToPlayInto(t, "arkascended")
	r, _ := routineCalled(a.Routines(), "ark graphics")
	into := a.PlaybackTarget(r.ID)
	if into.Chosen == "" {
		t.Fatalf("the fixture window is not offered: %+v", into)
	}
	// THE SLOT IS CLAIMED BY SOMETHING ELSE.
	if why := a.reserve(kindAct); why != "" {
		t.Fatal(why)
	}
	lifted := 0
	a.lifting = func(uintptr, uint32) (bool, bool) { lifted++; return true, true }

	withAMarcoBeside(t)
	why := a.PlayRoutine(r.ID, into.Chosen)
	if why == "" {
		t.Fatal("a play started while a run was already in flight")
	}
	if !strings.Contains(why, "one at a time") {
		t.Errorf("the refusal reads: %s", why)
	}
	if lifted != 0 {
		t.Errorf("a window was brought to the front over whatever the person was "+
			"doing, for a run that was refused anyway (%d)", lifted)
	}
}
