//go:build windows

package winctx

// A RAISE SAYS WHETHER THE WINDOW ACTUALLY CAME FORWARD.
//
// `SetForegroundWindow` answers that the REQUEST was accepted, which is a different claim: the
// foreground lock refuses it outright to a process that does not already own the foreground, and
// the measured consequence of believing it was a run that waited thirty seconds for a window
// nobody had brought forward.
//
// It matters more than it looks. A guarded press asks for itself whether its window is in front
// and refuses; an UNGUARDED one asks nothing, so the handoff is the only thing between a routine
// that types and somebody's editor. A `Raise` that reported success whatever happened would put
// that back.
//
// The readings are behind variables because the case under test cannot be arranged: whether
// Windows grants a foreground handoff depends on which process owns the foreground at that
// instant, which depends on what the person at the machine is doing. See the note beside them.
//
// See [[ADR-272-the-window-a-run-acts-in-is-chosen-and-carried]].

import (
	"strings"
	"testing"
)

// aDesktopThatAnswers stands the two readings a raise makes in front of it: who owns the window,
// and who is leading after the ask.
//
// `leads` is read fresh every time, so a test can describe a foreground that arrives LATE — which
// is the ordinary case and the one an impatient check would refuse.
func aDesktopThatAnswers(t *testing.T, owner uint32, leads func() uint32) *int {
	t.Helper()
	wasForce, wasLeading, wasOwner, wasWait := forceTo, leadingNow, ownerOf, waitABit
	t.Cleanup(func() {
		forceTo, leadingNow, ownerOf, waitABit = wasForce, wasLeading, wasOwner, wasWait
	})
	asked := 0
	forceTo = func(uintptr) { asked++ }
	leadingNow = leads
	ownerOf = func(uintptr) uint32 { return owner }
	// NO WAITING. The real one sleeps up to half a second per try; what is under test is
	// the decision, and a test that took five seconds to make it would be measuring the
	// sleep.
	waitABit = func() {}
	return &asked
}

// A HANDOFF WINDOWS REFUSES IS REPORTED AS A FAILURE, NAMING WHAT IS STILL LEADING.
func TestARaiseSaysWhetherTheWindowCameForward(t *testing.T) {
	asked := aDesktopThatAnswers(t, 4242, func() uint32 { return 99 })

	err := Raise(0x1234)
	if err == nil {
		t.Fatal("a raise that did not change the foreground reported success, so a " +
			"routine that types would type into whatever stayed in front")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("the failure does not say what is still leading: %v", err)
	}
	if *asked == 0 {
		t.Error("the raise was never attempted")
	}

	// AND THE CONTROL: A HANDOFF THAT WORKS IS SUCCESS. Without this the rule above is
	// satisfied by failing every raise, which would refuse every run.
	asked = aDesktopThatAnswers(t, 4242, func() uint32 { return 4242 })
	if err := Raise(0x1234); err != nil {
		t.Errorf("a window that is leading was reported as not raised: %v", err)
	}
	if *asked == 0 {
		t.Error("the raise was never attempted")
	}
}

// AND A FOREGROUND THAT ARRIVES LATE IS NOT A FAILURE.
//
// The change is not synchronous with the call: a full-screen game has to process the activation,
// and asking once gets the answer from before — which would refuse a raise that was about to
// work, and refusing a run that would have been fine is the other way to get this wrong.
func TestARaiseWaitsForAForegroundThatArrivesLate(t *testing.T) {
	tries := 0
	aDesktopThatAnswers(t, 4242, func() uint32 {
		tries++
		if tries < 4 {
			return 99
		}
		return 4242
	})

	if err := Raise(0x1234); err != nil {
		t.Errorf("a foreground that arrived on the fourth look was reported as "+
			"refused: %v", err)
	}
}

// AND THE QUESTION IS ABOUT THE PROGRAM, NOT THE HANDLE.
//
// An application brought forward does not necessarily leave THAT window in front: a game can
// activate a child window, a launcher can hand over to the window it was waiting for. What a
// person means by "it came forward" is that their application is in front — and the guarded
// boundary asks the stricter question for itself, at the instant of the press.
func TestARaiseIsAboutTheProgramAndNotTheHandle(t *testing.T) {
	// THE OWNER IS LEADING AND SOME OTHER WINDOW OF IT IS IN FRONT, which is what
	// `leadingNow` reporting the owner's pid means.
	aDesktopThatAnswers(t, 4242, func() uint32 { return 4242 })

	if err := Raise(0x1234); err != nil {
		t.Errorf("a program that is leading through another of its own windows was "+
			"reported as not raised: %v", err)
	}
}

// AND A WINDOW THAT WILL NOT SAY WHO OWNS IT IS NOT RAISED.
//
// Unreadable is not matching. Without the owner there is nothing to compare the foreground with,
// so the answer "it came forward" could not be given honestly either way.
func TestARaiseOfAWindowWithNoOwnerIsRefused(t *testing.T) {
	asked := aDesktopThatAnswers(t, 0, func() uint32 { return 4242 })

	err := Raise(0x1234)
	if err == nil {
		t.Fatal("a window that would not say who owns it was reported as raised")
	}
	if !strings.Contains(err.Error(), "which process owns it") {
		t.Errorf("the failure reads: %v", err)
	}
	if *asked != 0 {
		t.Errorf("the foreground was asked to move for a window nothing could be "+
			"checked about (%d)", *asked)
	}
	// AND NO WINDOW AT ALL IS THE SAME ANSWER.
	if err := Raise(0); err == nil {
		t.Error("raising nothing was reported as success")
	}
}
