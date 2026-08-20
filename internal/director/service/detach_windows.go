//go:build windows

package service

import (
	"os/exec"
	"syscall"
)

// configureDetached starts the service in its own process group and without a
// console window.
//
// Without this the service inherits the client's console, so closing the shell that
// happened to start it would take the Director down with it — and a service that
// dies when an unrelated window closes is not a service.
func configureDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008 | // DETACHED_PROCESS
			0x00000200, // CREATE_NEW_PROCESS_GROUP
	}
}
