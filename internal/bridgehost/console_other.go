//go:build !windows

package bridgehost

import "os/exec"

// hideConsole does nothing off Windows.
//
// The problem it solves is a Windows one: starting a console executable there creates a console
// window, and a new window takes the foreground. Nothing equivalent happens on the other
// platforms, so the stub is the whole implementation rather than a gap waiting to be filled.
func hideConsole(*exec.Cmd) {}
