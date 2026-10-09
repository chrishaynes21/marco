//go:build !windows

package padsource

// The non-Windows backend reads nothing, and says so.
//
// Inert rather than faked, for navsource's reason: a stub that produced plausible controller
// input would make every correlation on a non-Windows build a fabrication, and the only value
// this evidence has is that it came from something that really happened.

type stubBackend struct{}

func newBackend() backend { return stubBackend{} }

func (stubBackend) poll() padSnapshot { return padSnapshot{} }

func (stubBackend) unavailable() string {
	return "controller observation needs XInput, which this platform build does not provide"
}
