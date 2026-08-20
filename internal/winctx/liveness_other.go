//go:build !windows

package winctx

// Window liveness, on platforms with no backend yet.
//
// Everything answers "no" rather than "yes", which is the safe direction: the callers all
// use these to decide whether a screen capture may be attributed to an application, and an
// unimplemented platform must refuse rather than wave it through. See liveness.go for what
// these mean.

// LiveWindow is one top-level window as the platform currently reports it.
type LiveWindow struct {
	Handle     uintptr
	ProcessID  uint32
	Image      string
	Title      string
	Bounds     Rect
	Visible    bool
	Minimized  bool
	Foreground bool
	OnScreen   bool
}

func IsWindow(uintptr) bool                   { return false }
func WindowProcessID(uintptr) uint32          { return 0 }
func ProcessAlive(uint32) bool                { return false }
func ProcessImage(uint32) string              { return "" }
func LookUpWindow(uintptr) (LiveWindow, bool) { return LiveWindow{}, false }
func LiveWindows() []LiveWindow               { return nil }
