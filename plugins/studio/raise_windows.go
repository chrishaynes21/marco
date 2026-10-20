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

import "syscall"

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
)

// swRestore shows a window in its previous size and position, activating it.
const swRestore = 9

// raiseWindow brings one window forward, and says whether Windows allowed it.
func raiseWindow(h uintptr) bool {
	if h == 0 {
		return false
	}
	if iconic, _, _ := procIsIconic.Call(h); iconic != 0 {
		procShowWindow.Call(h, uintptr(swRestore))
	}
	ok, _, _ := procSetForeground.Call(h)
	return ok != 0
}
