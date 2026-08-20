//go:build !windows

package navsource

// The non-Windows source produces nothing, and says so.
//
// Inert rather than faked. A stub that emitted plausible navigation would make every
// correlation on a non-Windows build a fabrication, and the discovery loop's whole value is
// that its edges came from something that really happened.

type stubBackend struct{}

func newBackend() backend { return stubBackend{} }

func (stubBackend) unavailable() string {
	return "navigation observation needs a low-level keyboard hook, which this platform " +
		"build does not provide"
}

func (stubBackend) start(func(rawEvent) bool) error { return nil }
func (stubBackend) stop()                           {}
