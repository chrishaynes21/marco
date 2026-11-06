package resolver

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// Quiet starts a plugin with no console window of its own.
//
// # Why
//
// The plugin is a console program, and Director — which asks it most — runs with no console. Windows
// gives a console child of a console-less parent a NEW window, so every resolution flashed a
// terminal on the owner's desktop the moment the widget asked about a misheard request. The
// bridge plugins were started this way for the same reason (internal/bridgehost); the resolver's
// were not. stdin and stdout are pipes either way, so nothing about the exchange changes.
func Quiet(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
