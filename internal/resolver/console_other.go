//go:build !windows

package resolver

import "os/exec"

// Quiet is a no-op where a child does not get a console window of its own.
func Quiet(*exec.Cmd) {}
