//go:build windows

package main

// BRINGING THE APPLICATION FORWARD, FROM THE PROCESS THAT MAY.
//
// # The foreground lock, which is the whole reason this is here
//
// Windows refuses `SetForegroundWindow` to a process that does not already own the foreground.
// It is not negotiable and it is not a permission that can be asked for. Measured on this
// machine from a harness trying to raise a window from outside: the call returned false and the
// foreground did not move.
//
// Studio owns the foreground at exactly one instant — the person just clicked a button in it —
// and that is when this is called. The supervised runner, a child process nobody is looking at,
// could never do this, which is why it does not try.
//
// # What this deliberately does not do
//
// There are known ways to get around the lock: attaching to another thread's input queue,
// minimising and restoring somebody else's window, or synthesising an Alt keypress so the
// process looks like it received the last input. The last of those is the one that matters here:
// it is simulated human input, Marco's whole provenance rule is that injected presses are not a
// person's, and a product that faked a keystroke to steal focus would be breaking that rule to
// make a window appear. So this calls the two honest functions, and if Windows says no, it says
// no — the run then pauses saying it cannot see the application, which is true and actionable.
//
// It also does not move or resize anything. The demonstration recorded no window geometry — the
// control regions are fractions of whatever the window was, and the arrangement is a coarse cell
// summary — so there is no size to restore, and resizing a game makes it re-lay-out its own
// interface until the fractions no longer land on the control.

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	// ShowWindow with SW_RESTORE, for a window that is minimised or maximised-hidden:
	// raising a minimised window to the foreground gives you a foreground window with
	// no pixels, which the capture path then reads as an unusable look.
	procShowWindow = user32.NewProc("ShowWindow")
	// SetForegroundWindow, which is the call the lock applies to.
	procSetForeground = user32.NewProc("SetForegroundWindow")
	// IsIconic, so the restore is only attempted on a window that needs it. Restoring a
	// window that is already up can change a maximised window's size, and the point of
	// this file is to touch the person's arrangement as little as possible.
	procIsIconic = user32.NewProc("IsIconic")
	// AND THE THREE THAT ANSWER "IS IT ACTUALLY IN FRONT NOW". SetForegroundWindow's
	// own return value is not that answer: it reports that the request was accepted,
	// and the measured consequence of believing it was a run that waited thirty
	// seconds for a window nobody had brought forward.
	procGetForeground   = user32.NewProc("GetForegroundWindow")
	procWindowThreadPID = user32.NewProc("GetWindowThreadProcessId")
)

// swRestore shows a window in its previous size and position, activating it.
const swRestore = 9

// raiseWindow brings one window forward and reports whether the application is now in front.
//
// `leading` is what it looks like to the person: the application owning `pid` is the foreground
// application. `checked` says this machine could tell — true here, false in the stub, so a
// surface can distinguish "it is not in front" from "nobody could say".
//
// # Why the window's own handle is not what is compared
//
// Because an application brought forward does not necessarily leave THAT window in front: a
// game can activate a child window, a launcher can hand over to the window it was waiting for.
// What a person means by "it came forward" is that their application is in front, which is the
// process, and it is also what Marco's own foreground guard asks at the instant of a press.
//
// # And why it waits
//
// The foreground change is not synchronous with the call. A full-screen game has to process the
// activation, and asking immediately gets the answer from before — which would refuse a raise
// that was about to work. Half a second, in short steps, returning the moment it is true.
func raiseWindow(h uintptr, pid uint32) (leading, checked bool) {
	l, _, c := raiseWindowTraced(h, pid)
	return l, c
}

// raiseWindowTraced is the same raise, saying what it saw at every step.
//
// # Why the trace exists
//
// Three desktop scenarios failed with "Windows would not bring %s to the front", and that
// sentence is what this function concludes " — not what it observed. All it establishes is that the
// target process was not leading after the bounded wait. It does NOT establish that the foreground
// lock rejected the request, which is one of several things that sentence invites a reader to
// assume: the call may have been accepted and the window may have handed on to something else, the
// window may have been restored and not activated, the process may have had a second window
// leading, or Studio may not have owned the foreground to give away in the first place.
//
// So this records the four facts a reader needs and nothing concludes for them: who was leading
// before the ask (which is whether Studio had anything to hand on), the window and owner asked
// for, what `SetForegroundWindow` ITSELF returned together with the error code behind it, and who
// was leading at each look through the wait.
//
// It changes no behaviour. The waiting, the restore and the comparison are exactly as they were:
// nothing is retried, nothing waits longer, and no key is synthesised to get around anything.
func raiseWindowTraced(h uintptr, pid uint32) (leading bool, trace raiseTrace, checked bool) {
	trace.Asked, trace.AskedPID = h, pid
	if h == 0 {
		trace.Why = "no window was given"
		return false, trace, true
	}
	trace.LeadingBefore, trace.LeadingPIDBefore = leadingNow()
	if iconic, _, _ := procIsIconic.Call(h); iconic != 0 {
		trace.Restored = true
		procShowWindow.Call(h, uintptr(swRestore))
	}
	// THE CALL'S OWN ANSWER, AND THE ERROR BEHIND IT. `SetForegroundWindow` returns
	// non-zero when the request was accepted. A zero with no error code is the lock
	// declining; a zero WITH one is something else, and the two have been read as the
	// same thing.
	ok, _, err := procSetForeground.Call(h)
	trace.SetForegroundReturned = ok != 0
	if err != nil {
		trace.SetForegroundError = err.Error()
	}
	for i := 0; i < 10; i++ {
		hwnd, owner := leadingNow()
		trace.Looks = append(trace.Looks, raiseLook{At: i, Hwnd: hwnd, PID: owner})
		if owner == pid {
			trace.LeadingAfter, trace.LeadingPIDAfter = hwnd, owner
			return true, trace, true
		}
		time.Sleep(50 * time.Millisecond)
	}
	hwnd, owner := leadingNow()
	trace.Looks = append(trace.Looks, raiseLook{At: 10, Hwnd: hwnd, PID: owner})
	trace.LeadingAfter, trace.LeadingPIDAfter = hwnd, owner
	return owner == pid, trace, true
}

// raiseTrace is what one attempt to hand the foreground on actually observed.
//
// Every field is a READING, and none of them is a conclusion. `Why` is set only where this
// function declines before asking anything.
type raiseTrace struct {
	Asked    uintptr `json:"asked_hwnd"`
	AskedPID uint32  `json:"asked_pid"`
	// Who was leading before the ask " — which is whether the process doing the asking had
	// the foreground to hand on at all.
	LeadingBefore    uintptr `json:"leading_before_hwnd"`
	LeadingPIDBefore uint32  `json:"leading_before_pid"`
	// Whether the window had to be restored first.
	Restored bool `json:"restored,omitempty"`
	// What the call itself answered, and the error code behind it.
	SetForegroundReturned bool   `json:"setforeground_returned"`
	SetForegroundError    string `json:"setforeground_error,omitempty"`
	// Every look through the existing wait, in order.
	Looks []raiseLook `json:"looks"`
	// Who was leading when the wait ended.
	LeadingAfter    uintptr `json:"leading_after_hwnd"`
	LeadingPIDAfter uint32  `json:"leading_after_pid"`
	Why             string  `json:"why,omitempty"`
}

// raiseLook is one reading of the foreground during the wait.
type raiseLook struct {
	At   int     `json:"look"`
	Hwnd uintptr `json:"hwnd"`
	PID  uint32  `json:"pid"`
}

// leadingNow is the foreground window and its owner, or zeroes when there is none.
func leadingNow() (uintptr, uint32) {
	front, _, _ := procGetForeground.Call()
	if front == 0 {
		return 0, 0
	}
	var pid uint32
	procWindowThreadPID.Call(front, uintptr(unsafe.Pointer(&pid)))
	return front, pid
}
