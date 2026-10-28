//go:build !windows

package gate

import "errors"

// Snapshot is not available off Windows, and says so rather than answering nothing.
//
// An empty list would read as "nothing survived", which is the one wrong answer: the gate's whole
// point is that silence and absence are different. A caller gets an error and reports that the
// question could not be asked here.
func Snapshot() ([]Process, error) {
	return nil, errors.New("this platform cannot be asked what is running")
}
