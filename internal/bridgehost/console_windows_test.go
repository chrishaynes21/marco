//go:build windows

package bridgehost

import (
	"os/exec"
	"syscall"
	"testing"
)

// A BRIDGE CHILD IS NOT ALLOWED A WINDOW.
//
// # The live failure
//
// Phase-0 acceptance, cold path only. A learned play was asked for with VS Code in front; the
// Director brought Settings forward correctly, then started its accessibility bridge for the first
// time. The bridge is a console executable, Windows gave it a console, and a new console window
// takes the foreground. The fresh look that decides where the Audience is standing then found
//
//	nothing has observed applicationframehost, so there is no window to look at
//
// about a desktop where Settings had been in front a moment earlier. A warm Director never showed
// it — the bridge was already running, so no window appeared mid-performance.
//
// Deleting either flag in hideConsole must fail this.
func TestABridgeChildIsGivenNoConsoleWindow(t *testing.T) {
	cmd := exec.Command("cmd.exe")
	hideConsole(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("the bridge child was started with no process attributes at all, so Windows " +
			"chose for it — and its choice is a console window that takes the foreground")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow is not set")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Errorf("CREATE_NO_WINDOW (0x%08x) is not set in creation flags 0x%08x; the bridge "+
			"gets a console, and a new console window steals the foreground from the "+
			"application the Director just brought forward",
			createNoWindow, cmd.SysProcAttr.CreationFlags)
	}
}

// AND IT DOES NOT TRAMPLE ATTRIBUTES SOMEBODY ELSE SET.
func TestHidingTheConsolePreservesOtherProcessAttributes(t *testing.T) {
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200} // CREATE_NEW_PROCESS_GROUP
	hideConsole(cmd)

	if cmd.SysProcAttr.CreationFlags&0x00000200 == 0 {
		t.Error("an existing creation flag was overwritten rather than added to")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Error("CREATE_NO_WINDOW was not added")
	}
}
