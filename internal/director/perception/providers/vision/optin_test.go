package vision_test

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Vision is opt-in on BOTH doors.
//
// It was opt-in on one. `Observe` checked `req.Includes(SourceVision)` and documented the
// rule — "a caller that has not thought about whether it wants a screen capture does not get
// one" — and `ObserveTargeted` went straight to the detector. The collector prefers
// `ObserveTargeted` for any provider that implements it, so from the moment vision became a
// TargetedProvider every ordinary perception cycle ran a pass nobody asked for.
//
// Two consequences, and the second is why it went unnoticed for so long:
//
//  1. a screen capture per cycle on any Director with the vision plugin configured;
//  2. on a Director whose world had no focused window yet, "no window to look at" in every
//     live diagnosis — which reads as a targeting fault and is not one.

// countingWindow records how often anything asked which window to look at.
//
// The cheapest possible witness that a pass began. Asking for the window is the FIRST thing
// Look does after checking the detector exists, so a counter here fires before any capture
// and cannot be satisfied by a pass that failed for some later reason.
type countingWindow struct {
	calls  int
	window directorapi.Window
	ok     bool
}

func (c *countingWindow) active(context.Context) (directorapi.Window, bool) {
	c.calls++
	return c.window, c.ok
}

// refusingCapture fails any capture, loudly. Nothing here should ever reach it.
type refusingCapture struct{ calls int }

func (r *refusingCapture) CaptureWindow(context.Context, directorapi.Window) (capture.Image, error) {
	r.calls++
	return capture.Image{}, errNoCapture
}

type captureError struct{}

func (captureError) Error() string { return "the test refuses to capture the screen" }

var errNoCapture = captureError{}

// stubDetector is present and never used.
type stubDetector struct{ calls int }

func (d *stubDetector) Detect(context.Context, vision.Input) ([]vision.Detection, error) {
	d.calls++
	return nil, nil
}
func (d *stubDetector) Model() string { return "stub" }

func newProvider() (*vision.Provider, *countingWindow, *refusingCapture, *stubDetector) {
	win := &countingWindow{
		window: directorapi.Window{ID: "hwnd:1", Application: "testapp",
			Bounds: directorapi.Rect{Width: 1920, Height: 1080}},
		ok: true,
	}
	cap := &refusingCapture{}
	det := &stubDetector{}
	return vision.New(det, cap, win.active), win, cap, det
}

// THE regression. An ordinary cycle must not run a vision pass.
//
// `observation.Request{}` is what `Runtime`'s observe closure sends on every command, every
// wait poll and every diagnostic read. Before this fix it captured the screen.
func TestAnOrdinaryCycleDoesNotRunAVisionPass(t *testing.T) {
	p, win, cap, det := newProvider()

	out := p.ObserveTargeted(context.Background(), observation.Request{})

	if win.calls != 0 {
		t.Errorf("an unrequested cycle asked which window to look at %d time(s)", win.calls)
	}
	if cap.calls != 0 {
		t.Fatalf("an unrequested cycle CAPTURED THE SCREEN %d time(s). Every ordinary "+
			"command would take a screenshot nobody asked for", cap.calls)
	}
	if det.calls != 0 {
		t.Errorf("an unrequested cycle ran the detector %d time(s)", det.calls)
	}

	// And it says what happened. Not-requested is neither empty nor unavailable: one would
	// read as "the detector looked and found no controls", the other as "reinstall the
	// plugin", and both would send somebody somewhere useless.
	if out.State != observation.StateNotRequested {
		t.Errorf("an unrequested pass reported state %q", out.State)
	}
	if out.Reason != "" {
		t.Errorf("an unrequested pass reported a reason %q, so it would land in the "+
			"world's Degraded list and read as a fault", out.Reason)
	}
	if len(out.Observations) != 0 {
		t.Error("an unrequested pass produced observations")
	}
}

// A request that ASKS for vision still gets it. The opt-in is a gate, not a wall.
func TestARequestedCycleStillRunsAVisionPass(t *testing.T) {
	p, win, cap, _ := newProvider()

	out := p.ObserveTargeted(context.Background(), observation.WithVision(nil))

	if win.calls == 0 {
		t.Fatal("a requested pass never asked which window to look at")
	}
	if cap.calls == 0 {
		t.Fatal("a requested pass never tried to capture")
	}
	// The stub capture refuses, so this is a failure — which is the point: the pass RAN.
	if out.State == observation.StateNotRequested {
		t.Error("a requested pass reported itself as not requested")
	}
}

// The two doors agree. Whichever one the collector picks, the same rule applies.
func TestBothDoorsHonourTheSameOptIn(t *testing.T) {
	p, _, cap, _ := newProvider()

	obs, err := p.Observe(context.Background(), observation.Request{})
	if len(obs) != 0 || err != nil {
		t.Errorf("Observe on an unrequested cycle returned %d obs, err=%v", len(obs), err)
	}
	if cap.calls != 0 {
		t.Fatalf("Observe captured on an unrequested cycle")
	}

	p.ObserveTargeted(context.Background(), observation.Request{})
	if cap.calls != 0 {
		t.Fatalf("ObserveTargeted captured on an unrequested cycle; the two doors disagree")
	}
}
