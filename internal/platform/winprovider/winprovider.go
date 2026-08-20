// Package winprovider reads window and monitor geometry through Marco's winctx.
//
// The Director needs the display layout to answer "the other monitor" and to turn
// "left half" into a rectangle. That is window-system information, not accessibility
// information: it is cheap, always available, and independent of whether the target
// application exposes anything at all — which matters, since the applications least
// willing to expose an accessibility tree still have windows that can be moved.
package winprovider

import (
	"context"
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/winctx"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Provider serves monitor and window geometry.
type Provider struct{}

// New returns a Provider.
func New() *Provider { return &Provider{} }

// Monitors returns the current display layout.
func (p *Provider) Monitors(ctx context.Context) ([]directorapi.Monitor, error) {
	mons, err := winctx.Monitors()
	if err != nil {
		return nil, fmt.Errorf("winprovider: %w", err)
	}
	out := make([]directorapi.Monitor, 0, len(mons))
	for _, m := range mons {
		out = append(out, directorapi.Monitor{
			ID:       m.ID,
			Bounds:   rect(m.Bounds),
			WorkArea: rect(m.Work),
			Primary:  m.Primary,
			Scale:    m.Scale,
		})
	}
	return out, nil
}

// Enrich fills in the geometry an accessibility snapshot cannot supply: which
// monitor each window is on, and its true minimized/maximized state.
//
// The accessibility provider reports a window's rectangle, but not the display
// layout it sits in — and a minimized window still reports a rectangle, an
// off-screen one, so its state has to be read from the window system rather than
// inferred from its position.
func (p *Provider) Enrich(windows []directorapi.Window) []directorapi.Window {
	mons, err := winctx.Monitors()
	if err != nil {
		return windows
	}
	out := make([]directorapi.Window, 0, len(windows))
	for _, w := range windows {
		if h, ok := ParseHandle(w.ID); ok {
			minimized, maximized, visible := winctx.WindowStyleState(h)
			w.Minimized, w.Maximized = minimized, maximized
			w.Visible = visible
			if b, ok := winctx.WindowBounds(h); ok && !minimized {
				w.Bounds = rect(b)
			}
		}
		if w.MonitorID == "" && !w.Bounds.Empty() {
			centre := w.Bounds.Center()
			for _, m := range mons {
				if rect(m.Bounds).Contains(centre) {
					w.MonitorID = m.ID
					break
				}
			}
		}
		out = append(out, w)
	}
	return out
}

// ParseHandle reads the "hwnd:<n>" form the Director uses for a WindowID.
func ParseHandle(id directorapi.WindowID) (uintptr, bool) {
	s := string(id)
	if len(s) < 6 || (s[:5] != "hwnd:" && s[:5] != "HWND:") {
		return 0, false
	}
	var n uint64
	for _, c := range s[5:] {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + uint64(c-'0')
	}
	if n == 0 {
		return 0, false
	}
	return uintptr(n), true
}

func rect(r winctx.Rect) directorapi.Rect {
	return directorapi.Rect{X: r.X, Y: r.Y, Width: r.W, Height: r.H}
}

// Owned reports whether a window is one of Marco's own presentation surfaces.
//
// Separate from Enrich on purpose, and the separation is load-bearing: `Enrich` is a ONE-TO-ONE
// transform whose results the caller maps back onto its observations by position. Dropping an
// entry there does not remove a window — it shifts every later one onto the wrong observation, so
// a filter written into `Enrich` would silently mislabel the rest of the desktop instead of
// excluding anything. That mistake was made and caught here; this method exists so it cannot be
// made again.
//
// Ownership is read from a window property the surface set on its own handle. See
// [directorapi.OwnedSurfaceProperty] for why it is a property rather than a title or a process
// name.
func (p *Provider) Owned(id directorapi.WindowID) bool {
	h, ok := ParseHandle(id)
	return ok && winctx.IsOwnedSurface(h)
}
