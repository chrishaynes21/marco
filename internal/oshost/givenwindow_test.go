package oshost

// THE WINDOW A RUN WAS GIVEN IS THE WINDOW IT ACTS IN, OR THE RUN REFUSES.
//
// `winctx.Bind` is given a program NAME and takes the first visible window of it, which is
// whichever `EnumWindows` reaches first. A surface that had already chosen a window — raised it,
// shown it to somebody, written it down — then had its choice silently replaced by that one, and
// the guarded boundary next door could not see the difference: two windows of one program agree on
// the program, the size and the scale, and the owner check compares the bound window against
// itself.
//
// So the window travels, and these are the four things that can be wrong with one by the time it
// arrives. Every case is driven through the `windowFacts`, `imageOf` and `raiseWindow` seams,
// because a window that closes, changes owner or changes program between two instants cannot be
// arranged on a real desktop on purpose — the same reason `guardedpress_test.go` gives for the
// first two.
//
// See [[ADR-272-the-window-a-run-acts-in-is-chosen-and-carried]].

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// theChosenWindow and theSiblingWindow are two windows of ONE program, which is the case all of
// this is about. The numbers mean nothing; that the run acts in the first and never the second
// does.
const (
	theChosenWindow  uintptr = 0x3310
	theSiblingWindow uintptr = 0x3320
)

// twoWindowsOfOneProgram is a desktop with both of them up, identical in every way a check can
// see and owned by two different processes.
func twoWindowsOfOneProgram(hwnd uintptr) winctx.WindowFacts {
	switch hwnd {
	case theChosenWindow:
		return winctx.WindowFacts{Alive: true, Front: true, PID: 900,
			Left: 0, Top: 0, W: 1280, H: 800, Dpi: 96}
	case theSiblingWindow:
		return winctx.WindowFacts{Alive: true, Front: true, PID: 901,
			Left: 100, Top: 100, W: 1280, H: 800, Dpi: 96}
	}
	return winctx.WindowFacts{}
}

// withTheDesktop stands a desktop in front of the host, names the program every process owns, and
// records which handles were raised.
func withTheDesktop(t *testing.T, facts func(uintptr) winctx.WindowFacts,
	program string) *[]uintptr {
	t.Helper()
	wasFacts, wasImage, wasRaise := windowFacts, imageOf, raiseWindow
	t.Cleanup(func() { windowFacts, imageOf, raiseWindow = wasFacts, wasImage, wasRaise })
	windowFacts = facts
	imageOf = func(uint32) string { return program }
	var raised []uintptr
	raiseWindow = func(h uintptr) error {
		raised = append(raised, h)
		return nil
	}
	return &raised
}

// A GIVEN WINDOW IS BOUND, AND ACTIVATE NEVER REPLACES IT WITH A SIBLING.
//
// The positive claim and the one the whole slice exists for. `Activate` is asked for the program,
// and the program has two windows; with one given, the search that would have chosen between them
// does not happen at all.
func TestAGivenWindowIsBoundAndNeverReplacedBySibling(t *testing.T) {
	raised := withTheDesktop(t, twoWindowsOfOneProgram, "notepad.exe")
	// AND `bindWindow` IS MADE TO ANSWER WITH THE OTHER ONE, which is what it would
	// really do: it takes the first visible window of the program. If the given window is
	// honoured, this is never called; if it is not, the sibling is bound and the
	// assertions below say which.
	wasBind := bindWindow
	t.Cleanup(func() { bindWindow = wasBind })
	searched := 0
	bindWindow = func(string) (uintptr, error) {
		searched++
		return theSiblingWindow, nil
	}

	h := &Host{b: &recBackend{}}
	is, err := h.GiveWindow(theChosenWindow, 900)
	if err != nil {
		t.Fatalf("the window could not be given: %v", err)
	}
	if is != "notepad.exe" {
		t.Errorf("the program that owns it is reported as %q", is)
	}
	// IT IS BOUND BEFORE A STEP RUNS, so a guarded press has a window to be checked
	// against even in a route that activates nothing.
	if hwnd, app, pid := h.binding(); hwnd != theChosenWindow || pid != 900 {
		t.Errorf("the run bound hwnd %#x owned by %d (%q), want %#x owned by 900",
			hwnd, pid, app, theChosenWindow)
	}
	if len(*raised) != 1 || (*raised)[0] != theChosenWindow {
		t.Errorf("the handles raised were %#x, want [%#x]", *raised, theChosenWindow)
	}

	// AND NOW THE ROUTE'S OWN ACTIVATE.
	st, val, err := h.Invoke(anActivate("notepad"))
	if err != nil || st != "ok" {
		t.Fatalf("activating the given window answered %q (%v): %v", st, val, err)
	}
	if searched != 0 {
		t.Errorf("the run looked for a window of the program %d time(s) although it "+
			"had been given one — that search is what chooses between two "+
			"windows of one program, and it is the defect this closes", searched)
	}
	if hwnd, _, pid := h.binding(); hwnd != theChosenWindow || pid != 900 {
		t.Errorf("after Activate the run is bound to hwnd %#x owned by %d, want "+
			"%#x owned by 900 — the sibling window was substituted", hwnd, pid,
			theChosenWindow)
	}
}

// AND A GIVEN WINDOW THAT HAS GONE IS REFUSED, WITH NOTHING BOUND.
//
// Refused rather than repaired: looking for another window of the same program is precisely the
// substitution this exists to stop, and a run that pressed into one would have been told it was
// pressing into the window somebody chose.
func TestAGivenWindowThatHasGoneIsRefused(t *testing.T) {
	withTheDesktop(t, func(uintptr) winctx.WindowFacts {
		return winctx.WindowFacts{} // NOT A WINDOW ANY MORE.
	}, "notepad.exe")

	h := &Host{b: &recBackend{}}
	_, err := h.GiveWindow(theChosenWindow, 900)
	if err == nil {
		t.Fatal("a run was given a window that is not there")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Errorf("the refusal reads %q", err)
	}
	if hwnd, _, _ := h.binding(); hwnd != 0 {
		t.Errorf("hwnd %#x was bound by a refused hand-over", hwnd)
	}
}

// AND ONE WHOSE HANDLE NOW BELONGS TO ANOTHER PROCESS IS REFUSED.
//
// A handle is not an identity. Windows hands the number back out, and the program NAME cannot see
// the difference: the new owner can be another copy of the same program, which is exactly the case
// a sibling window is. The owner is why both halves travel.
func TestAGivenWindowWhoseOwnerChangedIsRefused(t *testing.T) {
	withTheDesktop(t, func(hwnd uintptr) winctx.WindowFacts {
		return winctx.WindowFacts{Alive: true, Front: true, PID: 999,
			W: 1280, H: 800, Dpi: 96}
	}, "notepad.exe")

	h := &Host{b: &recBackend{}}
	_, err := h.GiveWindow(theChosenWindow, 900)
	if err == nil {
		t.Fatal("a run was given a handle that now belongs to another process")
	}
	for _, want := range []string{"900", "999"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name process %s: %q", want, err)
		}
	}
	if hwnd, _, _ := h.binding(); hwnd != 0 {
		t.Errorf("hwnd %#x was bound by a refused hand-over", hwnd)
	}
	// AND HALF AN IDENTITY IS NOT ONE EITHER.
	for _, half := range []struct {
		hwnd uintptr
		pid  uint32
	}{{theChosenWindow, 0}, {0, 900}} {
		if _, err := (&Host{b: &recBackend{}}).GiveWindow(half.hwnd, half.pid); err == nil {
			t.Errorf("a window was given as hwnd %#x owned by %d", half.hwnd,
				half.pid)
		}
	}
}

// AND ACTIVATING A GIVEN WINDOW THAT BELONGS TO ANOTHER PROGRAM IS REFUSED.
//
// A route that says `Activate "notepad"` while the run was handed a browser window is a surface
// that passed on the wrong window. Performing it against either would be performing something
// nobody asked for — and the search that would have "fixed" it is the thing being removed.
func TestActivatingAGivenWindowOfAnotherProgramIsRefused(t *testing.T) {
	raised := withTheDesktop(t, twoWindowsOfOneProgram, "chrome.exe")

	h := &Host{b: &recBackend{}}
	if _, err := h.GiveWindow(theChosenWindow, 900); err != nil {
		t.Fatalf("the window could not be given: %v", err)
	}
	st, val, err := h.Invoke(anActivate("notepad"))
	if err != nil {
		t.Fatalf("activating answered an error: %v", err)
	}
	if st == "ok" {
		t.Fatal("a route activated a window of a program it is not about")
	}
	said := fmt.Sprint(val)
	for _, want := range []string{"notepad", "chrome.exe"} {
		if !strings.Contains(said, want) {
			t.Errorf("the refusal does not name %s: %q", want, said)
		}
	}
	// AND IT IS KEPT BESIDE THE REFUSALS, which is how a route's own contract decides
	// whether the run was unsuccessful.
	if !strings.Contains(strings.Join(h.Unperformed(), "\n"), "could not be brought") {
		t.Errorf("the run does not record that nothing was brought forward: %v",
			h.Unperformed())
	}
	// THE RAISE FROM `GiveWindow` HAPPENED AND THE ONE FROM `Activate` DID NOT: a window
	// of the wrong program is not brought forward on the strength of a route step.
	if len(*raised) != 1 {
		t.Errorf("the handles raised were %#x, want only the one the hand-over "+
			"raised", *raised)
	}
	// AND THE PROGRAM A ROUTE ASKS FOR MATCHES ITS EXECUTABLE THE WAY `Activate` MATCHES.
	// `notepad` against `notepad.exe` is the ordinary case, and a stricter rule here
	// would refuse every route that worked by name.
	if !sameProgram("notepad.exe", "notepad") {
		t.Error("a route saying `notepad` does not match a window owned by " +
			"notepad.exe, so every route that worked by name would be refused")
	}
	if sameProgram("chrome.exe", "notepad") {
		t.Error("two different programs are treated as one")
	}
}

// anActivate is the call a route's `do OS's Activate with "<app>"` makes.
func anActivate(app string) runtime.HostCall {
	return runtime.HostCall{Action: "activate", Input: runtime.Text(app)}
}
