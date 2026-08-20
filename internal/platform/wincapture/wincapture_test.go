package wincapture

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The capture boundary refuses what it cannot attribute.
//
// Defence in depth: the tracker upstream validates before anything gets here, and this
// checks again anyway, because the window can close between the two and because a future
// caller may simply forget. Every test below asserts a REFUSAL, and none of them reaches
// the screen — the point is that the decision is made before a camera is pointed anywhere.

func window(id string, r directorapi.Rect, app string) directorapi.Window {
	return directorapi.Window{ID: directorapi.WindowID(id), Bounds: r, Application: app}
}

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

// oldBounds is the rectangle from the incident: where Rocket League used to be, on the
// monitor left of the primary.
var oldBounds = rect(-1920, 0, 1920, 1080)

func TestADestroyedWindowIsNotPhotographedAtItsOldBounds(t *testing.T) {
	// The incident, at the layer that used to permit it. The lookup fails because the
	// window is gone; the old code took that as "cannot refresh" and captured at the
	// caller's remembered rectangle.
	asked := 0
	c := New()
	c.Bounds = func(directorapi.WindowID) (directorapi.Rect, bool) {
		asked++
		return directorapi.Rect{}, false
	}

	_, err := c.CaptureWindow(context.Background(),
		window("hwnd:661516", oldBounds, "rocketleague"))

	if err == nil {
		t.Fatal("a destroyed window was photographed; this is the incident")
	}
	if asked == 0 {
		t.Fatal("the capture never asked whether the window still exists")
	}
	if !strings.Contains(err.Error(), "no longer exists") {
		t.Errorf("error %q does not say the window is gone", err)
	}
}

func TestWithoutALivenessLookupNothingIsPhotographed(t *testing.T) {
	// The hook used to be optional, and that optionality was the bug. A capture with no
	// way to check liveness has no way to be honest, so it refuses.
	c := New()

	if _, err := c.CaptureWindow(context.Background(),
		window("hwnd:1", rect(0, 0, 100, 100), "app")); err == nil {
		t.Fatal("a window was photographed with no liveness lookup wired")
	}
}

func TestTheLiveBoundsAreUsedNotTheCallersMemory(t *testing.T) {
	// A window that moved since the caller last looked must be photographed where it IS.
	moved := rect(0, 0, 4, 4)
	c := New()
	c.Bounds = func(directorapi.WindowID) (directorapi.Rect, bool) { return moved, true }

	img, err := c.CaptureWindow(context.Background(),
		window("hwnd:1", oldBounds, "app"))
	if err != nil {
		t.Skipf("no capturable screen in this environment: %v", err)
	}
	if img.Bounds == oldBounds {
		t.Fatal("the caller's remembered bounds were used")
	}
	if img.Bounds != moved {
		t.Fatalf("bounds = %v, want the live %v", img.Bounds, moved)
	}
}

func TestAWindowOwnedBySomebodyElseIsRefused(t *testing.T) {
	// Handle recycling: the number is valid, the window is real, and the pixels belong
	// to a different program.
	c := New()
	c.Bounds = func(directorapi.WindowID) (directorapi.Rect, bool) { return rect(0, 0, 4, 4), true }
	c.Owner = func(directorapi.WindowID) (string, bool) { return "calculator", true }

	_, err := c.CaptureWindow(context.Background(),
		window("hwnd:100", rect(0, 0, 4, 4), "rocketleague"))
	if err == nil {
		t.Fatal("another application's pixels were captured as rocketleague's")
	}
	if !strings.Contains(err.Error(), "calculator") {
		t.Errorf("error %q does not name the actual owner", err)
	}
}

func TestAWindowThatClosesDuringCaptureProducesNoFrame(t *testing.T) {
	// The race the second check exists for: valid at the check, gone by the time the
	// pixels were read. The image is real; its attribution is not.
	calls := 0
	c := New()
	c.Bounds = func(directorapi.WindowID) (directorapi.Rect, bool) {
		calls++
		if calls == 1 {
			return rect(0, 0, 4, 4), true
		}
		return directorapi.Rect{}, false
	}

	_, err := c.CaptureWindow(context.Background(), window("hwnd:1", rect(0, 0, 4, 4), "app"))
	if err == nil {
		t.Fatal("a frame survived the window closing mid-capture")
	}
	if !strings.Contains(err.Error(), "window_changed_during_capture") {
		t.Errorf("error %q does not report the race by name", err)
	}
}

func TestAWindowThatChangesHandsDuringCaptureProducesNoFrame(t *testing.T) {
	owners := []string{"rocketleague", "calculator"}
	at := 0
	c := New()
	c.Bounds = func(directorapi.WindowID) (directorapi.Rect, bool) { return rect(0, 0, 4, 4), true }
	c.Owner = func(directorapi.WindowID) (string, bool) {
		owner := owners[min(at, len(owners)-1)]
		at++
		return owner, true
	}

	_, err := c.CaptureWindow(context.Background(),
		window("hwnd:1", rect(0, 0, 4, 4), "rocketleague"))
	if err == nil {
		t.Fatal("a frame survived the window changing owner mid-capture")
	}
	if !strings.Contains(err.Error(), "window_changed_during_capture") {
		t.Errorf("error %q does not report the race by name", err)
	}
}

func TestAnEmptyLiveRectangleIsRefused(t *testing.T) {
	c := New()
	c.Bounds = func(directorapi.WindowID) (directorapi.Rect, bool) {
		return directorapi.Rect{}, true // exists, but reports nothing usable
	}

	if _, err := c.CaptureWindow(context.Background(),
		window("hwnd:1", rect(0, 0, 100, 100), "app")); err == nil {
		t.Fatal("a window reporting no size was photographed")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
