//go:build windows

package homelock

import (
	"syscall"
	"unsafe"
)

// The Windows backend: a named mutex, whose lifetime is the process's.
//
// # Why a mutex and not a file
//
// `CreateMutexW` returns a handle to a kernel object. The object lives while a handle to it is
// open, and Windows closes every handle a process holds when the process ends — cleanly, by
// crash, or by being killed. That is the whole property this package needs and the one a file
// cannot have: there is no stale mutex, no timeout to guess at, and no PID to distrust.
//
// `ERROR_ALREADY_EXISTS` is how somebody else's claim announces itself. Note that the handle is
// still returned in that case, and it is closed rather than kept: holding it would keep the
// object alive past the real owner's exit and turn a released claim into a permanent one.
//
// Ownership of the mutex is deliberately NOT taken (`bInitialOwner` false, no wait). This is used
// as a presence object, not as a critical section: nothing here blocks, and a Director that could
// not claim must say so at once rather than waiting for a process that may run for hours.

const caseInsensitiveFilesystem = true

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutx  = kernel32.NewProc("CreateMutexW")
	procCloseHandle = kernel32.NewProc("CloseHandle")
)

const errAlreadyExists = 183 // ERROR_ALREADY_EXISTS

type windowsClaim struct {
	handle uintptr
	name   string
}

func (c *windowsClaim) Name() string { return c.name }

func (c *windowsClaim) Release() {
	if c == nil || c.handle == 0 {
		return
	}
	_, _, _ = procCloseHandle.Call(c.handle)
	c.handle = 0
}

func claim(name string) (Claim, error) {
	wide, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	handle, _, callErr := procCreateMutx.Call(
		0, // no security attributes: this user's session, default ACL
		0, // bInitialOwner = false — see the note above
		uintptr(unsafe.Pointer(wide)),
	)
	if handle == 0 {
		return nil, callErr
	}
	if errno, ok := callErr.(syscall.Errno); ok && uintptr(errno) == errAlreadyExists {
		// SOMEBODY ELSE HAS IT, and the handle just opened is a second one on THEIR
		// object. Closing it is not optional: keeping it would hold the object open after
		// they exit, and the next Director would be refused by a ghost.
		_, _, _ = procCloseHandle.Call(handle)
		return nil, ErrHeld
	}
	return &windowsClaim{handle: handle, name: name}, nil
}
