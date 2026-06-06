//go:build !windows

package screen

// New returns a screen stub on platforms without a capture backend yet
// (macOS: CGDisplay; Linux: X11/SHM — future work).
func New() Screen { return stub{} }

type stub struct{}

func (stub) Pixel(int, int) (uint32, error)          { return 0, ErrUnsupported }
func (stub) Find(string, Region, int) (Match, error) { return Match{}, ErrUnsupported }
