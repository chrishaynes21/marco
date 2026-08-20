// Package wincapture takes pictures of windows for the Director's OCR provider.
//
// It is the only place that knows both how to capture pixels and how the Director's
// canonical coordinate space works, which is why the conversion between them lives
// here and is stated once. Everything above sees a CapturedImage that already knows
// where it came from.
package wincapture

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/screen"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Capture photographs windows and regions.
type Capture struct {
	// Bounds looks up a window's CURRENT bounds, so a capture can tell whether the
	// window moved while it was being taken, and refuse when it no longer exists.
	//
	// REQUIRED for CaptureWindow. It used to be optional, and the optionality is what
	// made the Rocket League incident possible: when the lookup failed the capture fell
	// back to the caller's remembered rectangle, so a destroyed window was photographed
	// at the coordinates it used to occupy — on a different monitor, as it turned out,
	// where the detector found 169 real elements that every diagnostic then attributed
	// to the closed game.
	//
	// A failed lookup is now a refusal. See the note on CaptureWindow.
	Bounds func(directorapi.WindowID) (directorapi.Rect, bool)
	// Owner reports which application currently owns a window, for the ownership check
	// immediately before and after the pixels are read. Optional: without it the
	// liveness and geometry checks still run, and they are what caught the incident.
	Owner func(directorapi.WindowID) (string, bool)
}

// New returns a capture source.
func New() *Capture { return &Capture{} }

var (
	_ ocr.WindowCapture = (*Capture)(nil)
	_ ocr.RegionCapture = (*Capture)(nil)
)

// CaptureWindow photographs one window.
//
// The window's bounds are read BEFORE and AFTER the capture where possible. A window
// that moved in between produces an image whose pixels came from one place and whose
// transform describes another — and every word read from it would be placed,
// confidently, on whatever is now at those coordinates. The provider refuses such a
// capture; this is what gives it the evidence to do so.
func (c *Capture) CaptureWindow(ctx context.Context, w directorapi.Window) (ocr.CapturedImage, error) {
	if err := ctx.Err(); err != nil {
		return ocr.CapturedImage{}, err
	}
	r := w.Bounds
	if r.Width <= 0 || r.Height <= 0 {
		return ocr.CapturedImage{}, fmt.Errorf("wincapture: window %s has no usable bounds %v", w.ID, r)
	}

	// The window must be there NOW, and the rectangle must come from the platform.
	//
	// No fallback. The caller's Bounds are a memory of where the window was when
	// somebody last looked, and a memory is not a place to point a camera. If the
	// lookup cannot answer, the window is gone or unreadable and there is no honest
	// frame to return — no pixels beats confidently attributed pixels from the wrong
	// window.
	if c.Bounds == nil {
		return ocr.CapturedImage{}, fmt.Errorf(
			"wincapture: refusing to photograph %s without a live bounds lookup; "+
				"cached bounds are not evidence that the window still exists", w.ID)
	}
	live, ok := c.Bounds(w.ID)
	if !ok {
		return ocr.CapturedImage{}, fmt.Errorf(
			"wincapture: the %s window no longer exists, so there is nothing to photograph "+
				"(its last known bounds %v are not a substitute)", w.ID, r)
	}
	if live.Width <= 0 || live.Height <= 0 {
		return ocr.CapturedImage{}, fmt.Errorf(
			"wincapture: the %s window reports no usable bounds %v", w.ID, live)
	}
	r = live

	// Ownership, before and after. The window can close between the check and the
	// capture — a race this cannot prevent, only detect — so it is asked twice and the
	// frame is discarded if the answer changed.
	ownerBefore, ownerKnown := c.owner(w.ID)
	if ownerKnown && w.Application != "" && !strings.EqualFold(ownerBefore, w.Application) {
		return ocr.CapturedImage{}, fmt.Errorf(
			"wincapture: %s is owned by %q now, not %q — refusing to attribute its pixels",
			w.ID, ownerBefore, w.Application)
	}

	img, err := screen.CaptureRegion(r.X, r.Y, r.Width, r.Height)
	if err != nil {
		return ocr.CapturedImage{}, fmt.Errorf("wincapture: capturing %s: %w", w.ID, err)
	}
	at := time.Now()

	after, stillThere := c.Bounds(w.ID)
	if !stillThere {
		return ocr.CapturedImage{}, fmt.Errorf(
			"wincapture: window_changed_during_capture: %s closed while it was being "+
				"photographed; the pixels cannot be attributed to it", w.ID)
	}
	if ownerAfter, known := c.owner(w.ID); known && ownerKnown && !strings.EqualFold(ownerAfter, ownerBefore) {
		return ocr.CapturedImage{}, fmt.Errorf(
			"wincapture: window_changed_during_capture: %s changed hands from %q to %q "+
				"while it was being photographed", w.ID, ownerBefore, ownerAfter)
	}

	out := ocr.CapturedImage{
		Image:  img,
		Bounds: r,
		// The capture is of the whole window rectangle including its frame, so the
		// image's origin IS the window's origin. A future content-only capture would
		// set this to the client-area inset and everything downstream would follow.
		ContentOrigin: directorapi.Point{},
		// One captured pixel is one desktop pixel: screen.CaptureRegion works in
		// physical pixels and the Director's canonical space is physical too. The
		// field is explicit rather than assumed so a per-monitor-DPI capture path can
		// change it in one place — and so that anyone reading a misplaced box can see
		// immediately what scale was believed.
		Scale:      1,
		CapturedAt: at,
		// Negative origins are ordinary: a monitor to the left of the primary has
		// negative X, and the transform carries that through untouched.
		Transform:             ocr.NewTransform(directorapi.Point{X: r.X, Y: r.Y}, 1),
		WindowID:              w.ID,
		Application:           w.Application,
		WindowBoundsAtCapture: after,
	}
	return out, nil
}

// CaptureRegion photographs a rectangle of the desktop, in canonical coordinates.
func (c *Capture) CaptureRegion(ctx context.Context, r directorapi.Rect) (ocr.CapturedImage, error) {
	if err := ctx.Err(); err != nil {
		return ocr.CapturedImage{}, err
	}
	if r.Width <= 0 || r.Height <= 0 {
		return ocr.CapturedImage{}, fmt.Errorf("wincapture: region %v has no area", r)
	}
	img, err := screen.CaptureRegion(r.X, r.Y, r.Width, r.Height)
	if err != nil {
		return ocr.CapturedImage{}, fmt.Errorf("wincapture: capturing region %v: %w", r, err)
	}
	return ocr.CapturedImage{
		Image:      img,
		Bounds:     r,
		Scale:      1,
		CapturedAt: time.Now(),
		Transform:  ocr.NewTransform(directorapi.Point{X: r.X, Y: r.Y}, 1),
	}, nil
}

// owner reports the application currently owning a window, when that can be determined.
//
// A separate method so the nil check lives in one place: the hook is optional, and a
// capture without it still has the liveness and geometry checks, which are what caught the
// incident this file's comments describe.
func (c *Capture) owner(id directorapi.WindowID) (string, bool) {
	if c.Owner == nil {
		return "", false
	}
	return c.Owner(id)
}
