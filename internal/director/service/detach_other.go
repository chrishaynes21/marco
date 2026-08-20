//go:build !windows

package service

import "os/exec"

// configureDetached is a no-op off Windows; Setsid would go here.
func configureDetached(cmd *exec.Cmd) {}
