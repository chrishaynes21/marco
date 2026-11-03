//go:build windows

package recordingnow

import (
	"errors"
	"syscall"
	"time"
)

const (
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
)

// processState is whether pid is a running process and when it started.
//
// GONE ONLY WHEN CONFIRMED. OpenProcess refusing with ERROR_INVALID_PARAMETER is "no such
// process"; an exit code other than STILL_ACTIVE is a process that has ended. Any other failure —
// access denied, an unreadable time — is `unsure`, never gone, because gone is what lets a marker
// be swept and a shortcut through.
func processState(pid uint32) (liveness, time.Time) {
	if pid == 0 {
		return gone, time.Time{}
	}
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil {
		// ERROR_INVALID_PARAMETER (87) is what OpenProcess says of a pid with no process.
		if errors.Is(err, syscall.Errno(87)) {
			return gone, time.Time{}
		}
		return unsure, time.Time{}
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return unsure, time.Time{}
	}
	if code != stillActive {
		return gone, time.Time{}
	}
	var created, exited, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return unsure, time.Time{}
	}
	return alive, time.Unix(0, created.Nanoseconds())
}
