package ocr

import (
	"context"
	"image"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Deterministic doubles, for tests and for the runtime-unavailable state.
//
// These are test infrastructure that lives in the package rather than in a _test file
// because the composition root needs UnavailableEngine: when no OCR runtime can be
// found, the Director must still start, still observe, and still say clearly that OCR
// is unavailable. A nil engine would mean a nil check at every call site; an engine
// that returned empty success would mean every application looked textless.

// FakeEngine returns a fixed set of results. Deterministic by construction: the same
// engine returns the same results in the same order every time, which is what lets
// fusion tests assert on exact outcomes.
type FakeEngine struct {
	Results []Result
	Err     error
	// Delay simulates a slow engine, for timeout and cancellation tests.
	Delay time.Duration
	// Calls counts recognitions, so a test can assert the cache prevented one.
	Calls int
}

var _ Engine = (*FakeEngine)(nil)

func (f *FakeEngine) Recognize(ctx context.Context, _ ImageInput) ([]Result, error) {
	f.Calls++
	if f.Delay > 0 {
		// Cancellation must win over the delay, which is the property a timeout test
		// is actually checking: an engine that ignored ctx would hang the Director.
		select {
		case <-time.After(f.Delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.Err != nil {
		return nil, f.Err
	}
	return append([]Result(nil), f.Results...), nil
}

// UnavailableEngine is the engine used when no OCR runtime exists.
//
// It fails every call with an Unavailable error rather than returning nothing, so the
// distinction that matters most in this package — "OCR is not installed" versus "there
// is no text here" — survives all the way to the diagnostics a user reads.
type UnavailableEngine struct{ Reason string }

var _ Engine = UnavailableEngine{}

func (u UnavailableEngine) Recognize(context.Context, ImageInput) ([]Result, error) {
	reason := u.Reason
	if reason == "" {
		reason = "no OCR runtime was configured"
	}
	return nil, &Unavailable{Engine: "ocr", Reason: reason}
}

// FakeCapture returns a fixed image, for tests that need no desktop.
type FakeCapture struct {
	Img CapturedImage
	Err error
	// MoveWindowTo, when set, reports the window as having been somewhere else at
	// capture time — the stale-capture case.
	MoveWindowTo *directorapi.Rect
	Calls        int
}

var _ WindowCapture = (*FakeCapture)(nil)

func (f *FakeCapture) CaptureWindow(ctx context.Context, w directorapi.Window) (CapturedImage, error) {
	f.Calls++
	if err := ctx.Err(); err != nil {
		return CapturedImage{}, err
	}
	if f.Err != nil {
		return CapturedImage{}, f.Err
	}
	img := f.Img
	if img.Image == nil {
		img.Image = image.NewRGBA(image.Rect(0, 0, w.Bounds.Width, w.Bounds.Height))
	}
	if img.CapturedAt.IsZero() {
		img.CapturedAt = time.Now()
	}
	if img.Transform == (CoordinateTransform{}) {
		img.Transform = NewTransform(directorapi.Point{X: w.Bounds.X, Y: w.Bounds.Y}, 1)
	}
	img.WindowID, img.Application = w.ID, w.Application
	img.WindowBoundsAtCapture = w.Bounds
	if f.MoveWindowTo != nil {
		img.WindowBoundsAtCapture = *f.MoveWindowTo
	}
	return img, nil
}
