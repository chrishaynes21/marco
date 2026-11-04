//go:build !windows

package gate

import "fmt"

// Exited cannot be answered here: there is no start time to identify the process by (see
// StartedAt), so nothing here can say that the process being waited for is the one named.
func Exited(id Identity) (<-chan struct{}, error) {
	return nil, fmt.Errorf("this platform cannot identify %s, so its exit cannot be "+
		"waited for", id)
}
