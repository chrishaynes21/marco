//go:build !windows

package recorder

// New returns a recorder stub on platforms without an input-capture backend yet
// (macOS/Linux are future work: CGEventTap, evdev/libinput). Start reports
// ErrUnsupported so callers degrade gracefully.
func New() Recorder { return stubRecorder{} }

type stubRecorder struct{}

func (stubRecorder) Start() error                 { return ErrUnsupported }
func (stubRecorder) Stop() []RecordedEvent        { return nil }
func (stubRecorder) Events() <-chan RecordedEvent { return nil }

// Capture is stated and ignored, because there is nothing here to turn off.
//
// A stub records nothing at all, so a mode would be a setting on a recorder with no pictures to
// suppress. It is here so this backend satisfies `Stated` and a caller does not have to ask which
// platform it is on before stating what it wants — see recorder.Mode.
func (stubRecorder) Capture(Mode) {}
