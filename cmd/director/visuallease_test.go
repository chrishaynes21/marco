package main

import (
	"context"
	"errors"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/visualstate"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A SENSOR SITTING OUT IS NOT A THING DISAPPEARING.
//
// `SetActionables` replaces, so the candidate set a press resolves against is rebuilt from each
// acquisition's world — and the detector is bought once per settled screen state. Every
// following acquisition therefore offered the accessibility window alone, not because anything
// observed the pixel-derived controls disappearing but because the only sensor that can see
// them did not run.
//
// The lease is justified in the present tense, never from age: the same rectangle is
// photographed again and the fingerprints compared. These tests hold that line from both sides
// — that continuity keeps the evidence, and that every discontinuity drops it.

// sceneCapture serves scripted pictures, so a test can say "the same screen" or "a different
// one" without a desktop.
type sceneCapture struct {
	images []image.Image
	err    error
	calls  int
}

func (c *sceneCapture) CaptureRegion(ctx context.Context, r directorapi.Rect) (ocr.CapturedImage, error) {
	if c.err != nil {
		return ocr.CapturedImage{}, c.err
	}
	i := c.calls
	c.calls++
	if i >= len(c.images) {
		i = len(c.images) - 1
	}
	if i < 0 {
		return ocr.CapturedImage{}, errors.New("no scripted image")
	}
	return ocr.CapturedImage{
		Image: c.images[i], Bounds: r, CapturedAt: time.Now(),
		Transform: ocr.NewTransform(directorapi.Point{X: r.X, Y: r.Y}, 1),
	}, nil
}

func flat(shade uint8) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: shade, G: shade, B: shade, A: 255})
		}
	}
	return img
}

var leaseWindow = directorapi.Window{
	ID: "hwnd:1", Application: "epicgameslauncher",
	Bounds: directorapi.Rect{X: 0, Y: 0, Width: 64, Height: 64},
}

// pixelSeen is a world holding one pixel-derived control and the focused window over it.
func pixelSeen() directorapi.WorldState {
	return directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{
		"w": {Role: directorapi.RoleWindow, Visible: true, Focused: true, Confidence: 0.9,
			Bounds: directorapi.Rect{X: 0, Y: 0, Width: 64, Height: 64}},
		"lib": {Role: directorapi.RoleMenu, Label: "Library", Visible: true, Confidence: 0.9,
			Bounds:     directorapi.Rect{X: 4, Y: 8, Width: 20, Height: 4},
			Provenance: pixelProvenance()},
	}}
}

// blind is the same screen read by accessibility alone: the window, and nothing inside it.
func blind() directorapi.WorldState {
	return directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{
		"w": {Role: directorapi.RoleWindow, Visible: true, Focused: true, Confidence: 0.9,
			Bounds: directorapi.Rect{X: 0, Y: 0, Width: 64, Height: 64}},
	}}
}

func pixelProvenance() directorapi.Provenance {
	return directorapi.Provenance{Sources: []directorapi.ObservationReference{
		{Source: directorapi.SourceVision, Observation: "o1"},
	}}
}

// leaseSampler is a sampler whose visual provider serves the scripted scenes.
func leaseSampler(t *testing.T, scenes ...image.Image) (*liveSampler, *sceneCapture) {
	t.Helper()
	s, _, _ := samplerWithNav(t)
	cap := &sceneCapture{images: scenes}
	s.rt.visual = visualstate.New(cap)
	return s, cap
}

// offered reports the roles currently on offer, and which of them are leased.
func offered(s *liveSampler) map[string]bool {
	out := map[string]bool{}
	for _, a := range s.rt.navSource.Actionables() {
		out[a.Role] = a.Leased
	}
	return out
}

// STILL-CURRENT VISUAL EVIDENCE SURVIVES ACQUISITIONS THAT DID NOT LOOK WITH THAT SENSOR.
func TestVisualEvidenceSurvivesAnAcquisitionThatDidNotUseThatSensor(t *testing.T) {
	same := flat(80)
	s, _ := leaseSampler(t, same, same, same, same, same, same)
	ctx := context.Background()

	s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())
	if _, ok := offered(s)["menu"]; !ok {
		t.Fatal("the acquisition that saw the menu did not offer it")
	}

	// FOUR acquisitions that never ran the detector. The scene is unchanged each time, so
	// the evidence is still current and the menu stays on offer.
	for i := 0; i < 4; i++ {
		s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())
		leased, ok := offered(s)["menu"]
		if !ok {
			t.Fatalf("acquisition %d dropped the menu. Accessibility not seeing it is not "+
				"evidence that it went away — nothing looked with the sensor that can.", i+1)
		}
		if !leased {
			t.Errorf("acquisition %d offered the menu without marking it as leased; a "+
				"reader cannot tell which evidence explained a click", i+1)
		}
	}
	if s.lease == nil {
		t.Error("the lease was dropped while the scene was demonstrably unchanged")
	}
}

// AND A CHANGED SCREEN DROPS IT IMMEDIATELY.
func TestAChangedSceneDropsTheVisualEvidence(t *testing.T) {
	s, _ := leaseSampler(t, flat(20), flat(230))
	ctx := context.Background()

	s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())

	if _, ok := offered(s)["menu"]; ok {
		t.Error("the menu stayed on offer after the picture materially changed. That is " +
			"memory asserting a Target that nothing currently sees.")
	}
	if s.lease != nil {
		t.Error("the lease survived a meaningful change")
	}
}

// EVERY DISCONTINUITY DROPS IT, AND EACH IS CHECKED BEFORE THE EXPENSIVE ONE.
func TestDiscontinuityDropsTheVisualEvidence(t *testing.T) {
	same := flat(80)
	elsewhere := leaseWindow
	elsewhere.ID = "hwnd:2"
	moved := leaseWindow
	moved.Bounds = directorapi.Rect{X: 40, Y: 40, Width: 64, Height: 64}

	for _, tc := range []struct {
		what   string
		window directorapi.Window
		gen    uint64
	}{
		{"a different window is in front", elsewhere, 1},
		{"the window was replaced", leaseWindow, 2},
		{"the window moved", moved, 1},
	} {
		s, _ := leaseSampler(t, same, same, same)
		ctx := context.Background()
		s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())
		s.pushActionables(ctx, blind(), tc.window, tc.gen, time.Now())

		if _, ok := offered(s)["menu"]; ok {
			t.Errorf("%s: the menu was still offered", tc.what)
		}
		if s.lease != nil {
			t.Errorf("%s: the lease survived", tc.what)
		}
	}
}

// A DEGRADED ACQUISITION DOES NOT PRESERVE IT.
//
// "The picture could not be taken" is not "the picture is the same". A lease that survived an
// unanswerable question would be the elapsed-time cache this design exists to refuse.
func TestAnUnreadableSceneDoesNotPreserveTheEvidence(t *testing.T) {
	s, cap := leaseSampler(t, flat(80), flat(80))
	ctx := context.Background()
	s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())
	if s.lease == nil {
		t.Fatal("no lease was taken from the acquisition that saw the menu")
	}

	cap.err = errors.New("the screen is locked")
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())

	if _, ok := offered(s)["menu"]; ok {
		t.Error("the menu was offered on evidence that could not be checked")
	}
	if s.lease != nil {
		t.Error("the lease survived an acquisition that could not photograph the scene")
	}
}

// ELAPSED TIME ALONE PRESERVES NOTHING, AND NEITHER DOES A SESSION BOUNDARY.
func TestTimeAloneCannotPreserveVisualEvidence(t *testing.T) {
	// A Director with no visual provider cannot establish continuity, so it cannot hold a
	// lease at all — rather than holding one on the strength of the clock.
	s, _, _ := samplerWithNav(t)
	ctx := context.Background()
	s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())
	if _, ok := offered(s)["menu"]; ok {
		t.Error("a Director that cannot photograph the scene offered leased evidence " +
			"anyway; the only thing supporting it would be elapsed time")
	}

	// AND A NEW SESSION STARTS WITH NOTHING. The lease is a field on the sampler, which
	// is per-session, so a stop and restart cannot carry it — asserted rather than assumed
	// because "it is obviously session-scoped" is how durable state gets introduced.
	fresh, _ := leaseSampler(t, flat(80))
	if fresh.lease != nil {
		t.Error("a newly constructed sampler already holds visual evidence")
	}
}

// THE EVIDENCE STAYS TRANSIENT NO MATTER HOW LONG IT SURVIVES.
//
// Ten cheap confirmations in a row must leave it exactly as provisional as one: still on the
// sampler, still nowhere near semantic memory, and still not a durable affordance.
func TestSurvivingManyAcquisitionsDoesNotMakeItDurable(t *testing.T) {
	same := flat(80)
	s, _ := leaseSampler(t, same, same, same, same, same, same, same, same, same, same, same, same)
	ctx := context.Background()
	s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())
	for i := 0; i < 10; i++ {
		s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())
	}
	if s.lease == nil {
		t.Fatal("the lease was dropped while the scene was unchanged")
	}
	// It is still the ORIGINAL observation being compared against, not a chain of
	// re-photographs that could drift a cell at a time into a different screen.
	if s.lease.at.IsZero() {
		t.Error("the lease lost the acquisition it came from")
	}
	if got := len(s.lease.items); got != 1 {
		t.Errorf("the lease holds %d items; it should hold what one acquisition saw and "+
			"never accumulate", got)
	}
}
