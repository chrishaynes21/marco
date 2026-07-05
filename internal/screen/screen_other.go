//go:build !windows

package screen

import "image"

// New returns a screen stub on platforms without a capture backend yet
// (macOS: CGDisplay; Linux: X11/SHM — future work).
func New() Screen { return stub{} }

// CaptureRegion is unsupported without a backend; callers (e.g. the recorder)
// treat the error as "no template" and fall back to coordinate clicks.
func CaptureRegion(x, y, w, h int) (*image.RGBA, error) { return nil, ErrUnsupported }

// CaptureVirtual is unsupported without a backend.
func CaptureVirtual() (*image.RGBA, int, int, error) { return nil, 0, 0, ErrUnsupported }

// PrimarySize is unknown without a backend.
func PrimarySize() (w, h int) { return 0, 0 }

type stub struct{}

func (stub) Pixel(int, int) (uint32, error)              { return 0, ErrUnsupported }
func (stub) Find(string, Region, int) (Match, error)     { return Match{}, ErrUnsupported }
func (stub) FindEdge(string, Region, int) (Match, error) { return Match{}, ErrUnsupported }
