//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW: run the console child without giving it a console.
const createNoWindow = 0x08000000

// hideConsole stops a child Director from owning a window.
//
// # The failure this fixes
//
// `director` is a console executable, so starting one on Windows creates a console window — and
// a new console window TAKES THE FOREGROUND. Studio starts three of them: the backend service,
// a run, and a recording. The owner, on pressing Record:
//
//	Every time I click record, a new shell window is also launched.
//
// A black rectangle on the desktop is the visible half. The half that matters is the focus: a
// recording is a person demonstrating something IN THEIR GAME, and Studio's own `raise` has just
// brought that game forward. A console appearing a moment later puts itself in front of it, so
// the first seconds of the recording watch a window that is no longer there — and the recorder
// captures a covered window's pixels, refuses presses it cannot attribute to the target, and
// reads nothing on the screen the person is about to click.
//
// `internal/bridgehost` found the same thing in the Phase-0 live acceptance and fixed it the
// same way; the comment there names the measurement. This is a second copy rather than an import
// because `plugins/studio` is its own module and cannot reach the engine's internal packages.
//
// Stderr and stdout still reach the parent: CREATE_NO_WINDOW suppresses the console, not the
// inherited handles, so the transcript Studio reads off the child is unaffected — which is what
// `TestAHiddenConsoleStillHandsBackWhatTheChildSaid` holds.
func hideConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
