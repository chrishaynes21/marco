package providers

import (
	"context"
	"errors"
	"image"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A frame must be attributed to the window it is a picture OF, established after the grab.
//
// The capture backend's own before/after check compares application NAMES, so a window of
// the same application being replaced passes it. These tests are written against that
// specific blind spot.

// scriptedCapture returns a frame, optionally running a hook mid-capture — which is how the
// race test replaces the window while the shutter is open.
type scriptedCapture struct {
	img      capture.Image
	err      error
	duringFn func()
}

func (s *scriptedCapture) CaptureWindow(context.Context,
	directorapi.Window) (capture.Image, error) {

	if s.duringFn != nil {
		s.duringFn()
	}
	return s.img, s.err
}

func frame(window directorapi.WindowID, app string) capture.Image {
	return capture.Image{
		Image:       image.NewRGBA(image.Rect(0, 0, 4, 4)),
		WindowID:    window,
		Application: app,
		CapturedAt:  time.Now(),
	}
}

func provenance(app string, pid uint32, gen uint64) directorapi.TargetProvenance {
	return directorapi.TargetProvenance{Application: app, ProcessID: pid, WindowGeneration: gen}
}

func TestAFrameCarriesTheGenerationItWasCapturedFrom(t *testing.T) {
	want := provenance("code", 4242, 7)
	c := NewProvenCapture(
		&scriptedCapture{img: frame("hwnd:100", "code")},
		&fakeResolver{byWindow: map[directorapi.WindowID]directorapi.TargetProvenance{
			"hwnd:100": want,
		}})

	img, err := c.CaptureWindow(context.Background(), directorapi.Window{ID: "hwnd:100"})
	if err != nil {
		t.Fatalf("CaptureWindow: %v", err)
	}
	if img.Target == nil {
		t.Fatal("the frame carries no provenance, so nothing downstream can attribute it")
	}
	if *img.Target != want {
		t.Errorf("target = %+v, want %+v", *img.Target, want)
	}
}

// The blind spot, directly: the same application, a replaced window. The capture backend's
// name comparison passes this; the generation must not.
func TestAReplacedWindowOfTheSameApplicationIsDetected(t *testing.T) {
	table := map[directorapi.WindowID]directorapi.TargetProvenance{
		"hwnd:100": provenance("code", 4242, 7),
	}
	resolver := &fakeResolver{byWindow: table}
	inner := &scriptedCapture{img: frame("hwnd:100", "code")}
	// The window is replaced WHILE the shutter is open. Same application, same handle
	// string — only the generation moves.
	inner.duringFn = func() { table["hwnd:100"] = provenance("code", 4242, 8) }

	img, err := NewProvenCapture(inner, resolver).
		CaptureWindow(context.Background(), directorapi.Window{ID: "hwnd:100"})
	if err != nil {
		t.Fatalf("CaptureWindow: %v", err)
	}
	if img.Target == nil {
		t.Fatal("no provenance established")
	}
	if img.Target.WindowGeneration != 8 {
		t.Fatalf("generation = %d, want the post-capture 8 — provenance was taken "+
			"before the grab, or copied from the request",
			img.Target.WindowGeneration)
	}
	if img.Target.Application != "code" {
		t.Errorf("application changed unexpectedly: %+v", img.Target)
	}
}

// Provenance is resolved from the FRAME's window, not the requested one. Attributing a
// frame to what was asked for is the mistake the whole design exists to avoid.
func TestProvenanceFollowsTheFrameNotTheRequest(t *testing.T) {
	// The backend fell back to a different window than the one requested.
	inner := &scriptedCapture{img: frame("hwnd:999", "terminal")}
	c := NewProvenCapture(inner, &fakeResolver{
		byWindow: map[directorapi.WindowID]directorapi.TargetProvenance{
			"hwnd:100": provenance("code", 4242, 7),
			"hwnd:999": provenance("terminal", 77, 3),
		}})

	img, _ := c.CaptureWindow(context.Background(), directorapi.Window{
		ID: "hwnd:100", Application: "code",
	})
	if img.Target == nil {
		t.Fatal("no provenance established")
	}
	if img.Target.Application != "terminal" {
		t.Fatalf("the frame was attributed to the REQUESTED window (%+v) rather than "+
			"the one actually photographed", *img.Target)
	}
}

// An unresolvable window leaves the frame unproven, which is not the same as fine.
func TestAnUnresolvableWindowLeavesTheFrameUnproven(t *testing.T) {
	c := NewProvenCapture(
		&scriptedCapture{img: frame("hwnd:404", "gone")},
		&fakeResolver{byWindow: map[directorapi.WindowID]directorapi.TargetProvenance{}})

	img, _ := c.CaptureWindow(context.Background(), directorapi.Window{ID: "hwnd:404"})
	if img.Target != nil {
		t.Errorf("an unresolvable window produced provenance: %+v", *img.Target)
	}
}

// A Director with no window tracker can still capture; it just cannot claim what it saw.
func TestWithoutAResolverFramesAreUnproven(t *testing.T) {
	img, err := NewProvenCapture(&scriptedCapture{img: frame("hwnd:100", "code")}, nil).
		CaptureWindow(context.Background(), directorapi.Window{ID: "hwnd:100"})
	if err != nil {
		t.Fatalf("CaptureWindow: %v", err)
	}
	if img.Target != nil {
		t.Error("provenance was invented without a resolver")
	}
}

// A failed capture is passed through untouched — there is nothing to attribute.
func TestACaptureFailureIsNotDecorated(t *testing.T) {
	want := errors.New("window_changed_during_capture")
	_, err := NewProvenCapture(&scriptedCapture{err: want}, &fakeResolver{}).
		CaptureWindow(context.Background(), directorapi.Window{ID: "hwnd:100"})
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want the capture's own error", err)
	}
}
