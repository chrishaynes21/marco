//go:build !windows

package recorder

// New returns a recorder stub on platforms without an input-capture backend yet
// (macOS/Linux are future work: CGEventTap, evdev/libinput). Start reports
// ErrUnsupported so callers degrade gracefully.
func New() Recorder { return stubRecorder{} }

type stubRecorder struct{}

func (stubRecorder) Start() error                  { return ErrUnsupported }
func (stubRecorder) Stop() []RecordedEvent         { return nil }
func (stubRecorder) Events() <-chan RecordedEvent  { return nil }
