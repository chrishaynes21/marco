//go:build windows

package gate

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

const synchronize = 0x00100000

// Exited is closed when one identified process instance has exited.
//
// # Why on a handle, and why the identity is checked on it
//
// For the reason End is: a pid is a number Windows hands back out. The handle is opened once, the
// start time is read from THAT handle, and the wait is on THAT handle, so the thing waited for is
// the kernel object that was identified and not whatever holds the number later. A process that
// cannot be opened, or that is no longer the instance named, is an error: the caller knows only
// that it cannot wait for it, which is not the same as knowing it has gone.
//
// A process that has already exited but is still referenced elsewhere opens fine and answers its
// start time, so the wait returns at once, which is the right answer.
func Exited(id Identity) (<-chan struct{}, error) {
	if !id.Known() {
		return nil, fmt.Errorf("%s, so its exit cannot be waited for", id)
	}
	h, _, err := procOpenProcess.Call(synchronize|queryLimitedInformation, 0, uintptr(id.PID))
	if h == 0 {
		return nil, fmt.Errorf("pid %d could not be opened, so its exit cannot be waited "+
			"for: %v", id.PID, err)
	}
	var created, exited, kernel, user syscall.Filetime
	if r, _, err := procGetProcessTimes.Call(h,
		uintptr(unsafe.Pointer(&created)), uintptr(unsafe.Pointer(&exited)),
		uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); r == 0 {
		syscall.CloseHandle(syscall.Handle(h))
		return nil, fmt.Errorf("pid %d would not say when it started: %v", id.PID, err)
	}
	if at := time.Unix(0, created.Nanoseconds()); !at.Equal(id.Since) {
		syscall.CloseHandle(syscall.Handle(h))
		return nil, fmt.Errorf("pid %d is no longer %s: it started %s", id.PID, id,
			at.Format("2006-01-02 15:04:05.000"))
	}
	done := make(chan struct{})
	go func() {
		defer syscall.CloseHandle(syscall.Handle(h))
		_, _ = syscall.WaitForSingleObject(syscall.Handle(h), syscall.INFINITE)
		close(done)
	}()
	return done, nil
}
