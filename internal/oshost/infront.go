package oshost

// THE WINDOW IN FRONT, BOUND FOR A GUARDED ONLY-IN RUN (S9, ADR-306).
//
// A guarded press is checked against a BOUND window (`confirmWindow`), and an "Only in" routine
// has no `Activate` to bind one, so outside Studio every such press refused. Now the intake binds
// the window that is in front at admission: read once, verified, and bound, with NOTHING RAISED
// and NOTHING SEARCHED FOR. It is `GiveWindow` without the raise.
//
// The window in front becomes THIS RUN'S target. It need not be the window the routine was
// recorded in: a sibling window of the same program that is in front at admission is the one
// bound, and every press then checks it against the recorded size, scale and program. A sibling
// that comes to the front after this is not the bound handle and is refused at the next press.

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// foregroundNow is the window in front and who owns it. A variable so a test can state it.
var foregroundNow = func() (uintptr, uint32) {
	hwnd, pid, _ := winctx.ForegroundProcess()
	return hwnd, pid
}

// InFront is the window in front, as one reading: its handle, its owner and its program.
type InFront struct {
	HWnd    uintptr
	PID     uint32
	Program string
}

// ReadInFront reads the window in front once. Program is "" when it cannot be read.
func (h *Host) ReadInFront() InFront {
	hwnd, pid := foregroundNow()
	if hwnd == 0 || pid == 0 {
		return InFront{HWnd: hwnd, PID: pid}
	}
	return InFront{HWnd: hwnd, PID: pid, Program: imageOf(pid)}
}

// BindInFront binds the window that reading named, after verifying it is still that window and
// still in front. It raises nothing and searches for nothing, and it does not set `activated`
// (see GiveWindow: that flag changes where legacy window-relative clicks resolve).
//
// Raising, searching or binding a different window must fail
// TestAGuardedOnlyInRunIsBoundToTheWindowInFront (cmd/marco).
func (h *Host) BindInFront(w InFront) error {
	if w.HWnd == 0 || w.PID == 0 || w.Program == "" {
		return fmt.Errorf("Marco could not tell which window is in front, so there is no " +
			"window to check this play's presses against — nothing was performed")
	}
	f := windowFacts(w.HWnd)
	switch {
	case !f.Alive:
		return fmt.Errorf("the %s window in front closed before the run began — nothing "+
			"was performed", w.Program)
	case f.PID != w.PID:
		return fmt.Errorf("the %s window in front now belongs to another process — nothing "+
			"was performed", w.Program)
	case !f.Front:
		return fmt.Errorf("the %s window is no longer in front — nothing was performed",
			w.Program)
	}
	h.mu.Lock()
	h.bound, h.boundApp, h.boundPID = w.HWnd, w.Program, w.PID
	h.originSet = false
	h.mu.Unlock()
	return nil
}
