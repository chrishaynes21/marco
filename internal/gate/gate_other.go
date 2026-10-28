//go:build !windows

package gate

import (
	"errors"
	"fmt"
	"time"
)

// Snapshot is not available off Windows, and says so rather than answering nothing.
//
// An empty list would read as "nothing survived", which is the one wrong answer: the gate's whole
// point is that silence and absence are different. A caller gets an error and reports that the
// question could not be asked here.
func Snapshot() ([]Process, error) {
	return nil, errors.New("this platform cannot be asked what is running")
}

// StartedAt cannot be answered here, which means nothing is identified and so nothing is ended.
//
// The same reasoning as Snapshot's, one step on: a pid with no start time is not an identity (see
// Identity), and the one thing that must never happen is a kill decided on a bare pid.
func StartedAt(pid uint32) (time.Time, error) {
	return time.Time{}, errors.New("this platform cannot be asked when a process started")
}

// End cannot be answered here either, and says so rather than killing by pid.
//
// A portable `os.FindProcess` + `Kill` is exactly the shape this change exists to remove: it
// takes a number and ends whatever now holds it. There is nothing here to check an identity
// against, so there is nothing here that may end a process.
func End(id Identity) error {
	return fmt.Errorf("this platform cannot verify that %s is still the process this "+
		"run established, so nothing was done to it", id)
}
