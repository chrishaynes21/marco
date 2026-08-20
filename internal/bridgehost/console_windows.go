//go:build windows

package bridgehost

import (
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW: run the console child without giving it a console.
const createNoWindow = 0x08000000

// hideConsole stops a bridge child from owning a window.
//
// # The failure this fixes
//
// A bridge is a PROTOCOL child — JSON over stdin/stdout, logs on stderr — and has no business
// appearing on the desktop. On Windows it is a console executable, so starting it created a
// console window, and a new console window TAKES THE FOREGROUND.
//
// Measured in the Phase-0 live acceptance, on the cold path only. Asking for a learned play with
// VS Code in front brought Settings forward correctly, then the Director's accessibility bridge
// started for the first time, its console appeared, and the fresh look that followed found
// `WindowsTerminal` where Settings had just been:
//
//	nothing has observed applicationframehost, so there is no window to look at
//
// A warm Director never showed it, because the bridge was already running and no window appeared
// mid-performance. That is the shape of a defect only a cold start can find — and the reason
// [[ADR-078-a-learned-play-is-performed-by-the-director]] insists cold start is the test of
// durability.
//
// Stderr still reaches the parent: CREATE_NO_WINDOW suppresses the console, not the inherited
// handles, so the bridge's logs are unaffected.
func hideConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
