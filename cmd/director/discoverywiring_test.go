package main

import (
	"context"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// WHERE A SESSION'S GEOMETRY COMES FROM, THROUGH THE PRODUCTION PATH.
//
// Every rectangle in Marco used to come from one learned detector in a plugin, and `go build`
// in that plugin produces a detector that detects nothing. Measured on a real menu, the
// consequence was that the list was present as groupable structure on about ten looks in a
// hundred, so nothing downstream ever accumulated enough evidence to name an entry.
//
// The default is now to propose structure from the captured frame, with the detector kept as a
// fallback. These hold that the switch is real, reversible, and uses the SAME capture — a second
// screenshot pipeline is the thing this was explicitly not to become.

// drawnMenuCapture paints a frame with a panel and five rows, one of them lit.
//
// A DRAWN frame and not a fixture of detections, because the subject here is the path that reads
// pixels. The existing vision fixtures paint a ground and one row on purpose, which is right for
// testing the detector's channel and useless for testing this one.
type drawnMenuCapture struct{ lit int }

func (c drawnMenuCapture) CaptureWindow(_ context.Context,
	w directorapi.Window) (capture.Image, error) {

	img := image.NewRGBA(image.Rect(0, 0, w.Bounds.Width, w.Bounds.Height))
	for y := range w.Bounds.Height {
		for x := range w.Bounds.Width {
			img.SetRGBA(x, y, color.RGBA{26, 28, 34, 255})
		}
	}
	for row := range 5 {
		top := 60 + row*48
		shade := color.RGBA{74, 78, 88, 255}
		if row == c.lit {
			shade = color.RGBA{226, 228, 234, 255}
		}
		for y := top; y < top+34 && y < w.Bounds.Height; y++ {
			for x := 70; x < 330 && x < w.Bounds.Width; x++ {
				img.SetRGBA(x, y, shade)
			}
		}
	}
	return capture.Image{
		Image: img, Bounds: w.Bounds, Scale: 1, CapturedAt: time.Now(),
		WindowID: w.ID, Application: w.Application, WindowBoundsAtCapture: w.Bounds,
		Target: &directorapi.TargetProvenance{
			Application: w.Application, ProcessID: 7, WindowGeneration: 1,
		},
	}, nil
}

// blindDetector is a detector that finds nothing, which is what the default build produces.
type blindDetector struct{}

func (blindDetector) Detect(context.Context, vision.Input) ([]vision.Detection, error) {
	return nil, nil
}

// Pixels are the default discovery path, and a session gets geometry from a blind detector.
//
// THE POINT OF THE WHOLE CHANGE, stated as one assertion: the detector finds nothing, and the
// sample still carries the menu. Before this, a Director with an unbuilt ONNX backend could not
// propose a rectangle from a screenshot at all.
//
// Deleting `inner.Discovery = vision.DiscoveryFromEnv()` in newShadowProvider, or the
// proposeStructure call in Provider.Observe, must fail this.
func TestPixelsAreTheDefaultDiscoveryPath(t *testing.T) {
	// No MARCO_SHADOW_DISCOVERY: the default is what is under test.
	t.Setenv("MARCO_SHADOW_VISION", "screenparser")
	rt := &Runtime{engine: stubEngine{}}
	prov := newShadowProvider(blindDetector{}, drawnMenuCapture{lit: 2},
		rt.activeWindow, nil, time.Nanosecond)
	rt.collector = providers.NewCollector(prov)
	rt.shadowVision = prov
	rt.pinnedWindow = &windowRef1

	sample := sampleWithVision(t, rt)
	if sample.Shadow == nil {
		t.Fatal("no shadow record")
	}
	if sample.Shadow.Detections != 0 {
		t.Fatalf("the detector found %d things; this fixture's detector is blind and the "+
			"test proves nothing if it is not", sample.Shadow.Detections)
	}
	if len(sample.Shadow.Shapes) == 0 {
		t.Fatal("a drawn five-row menu, a blind detector, and the sample carries no " +
			"geometry at all. That is the state this change exists to end: with no " +
			"model there was no rectangle, and every stage downstream was starved")
	}
	// AND THE GEOMETRY IS MEASURABLE, which is what makes it selection evidence rather
	// than a rectangle. Unshaded shapes are dropped before grouping.
	shaded := 0
	for _, sh := range sample.Shadow.Shapes {
		if sh.Shaded() {
			shaded++
		}
	}
	if shaded == 0 {
		t.Errorf("%d shapes and none of them measurable; every candidate built from "+
			"these would be permanently quiet", len(sample.Shadow.Shapes))
	}
	t.Logf("blind detector, drawn menu: %d shapes, %d measurable",
		len(sample.Shadow.Shapes), shaded)
}

// The fallback is reachable, and it is the old behaviour exactly.
//
// Reversibility is a condition of this change, not a nicety: a perception regression has to be
// bisectable without a rebuild, and a run has to be reproducible after the default moved.
func TestTheModelRemainsAvailableAsAFallback(t *testing.T) {
	t.Setenv("MARCO_SHADOW_VISION", "screenparser")
	t.Setenv("MARCO_SHADOW_DISCOVERY", "model")
	rt := &Runtime{engine: stubEngine{}}
	prov := newShadowProvider(blindDetector{}, drawnMenuCapture{lit: 2},
		rt.activeWindow, nil, time.Nanosecond)
	rt.collector = providers.NewCollector(prov)
	rt.shadowVision = prov
	rt.pinnedWindow = &windowRef1

	sample := sampleWithVision(t, rt)
	if sample.Shadow == nil {
		t.Fatal("no shadow record")
	}
	if len(sample.Shadow.Shapes) != 0 {
		t.Errorf("in model mode a blind detector produced %d shapes. The fallback is "+
			"supposed to be the behaviour that existed before the proposer, where a "+
			"detector that finds nothing yields nothing", len(sample.Shadow.Shapes))
	}

	// And an unrecognised value falls back to the default rather than refusing to start:
	// a perception path must not fail over a typo in a diagnostic switch.
	t.Setenv("MARCO_SHADOW_DISCOVERY", "pixles")
	if got := vision.DiscoveryFromEnv(); got != vision.DiscoveryPixels {
		t.Errorf("a misspelled mode selected %q; it must fall back to the default", got)
	}
}

// The pass reports what the proposer cost, every time.
//
// The cadence argument — 37ms of proposal against a 976ms median inference — is only honest if
// the number is measured per pass rather than quoted from a benchmark. It is also the number
// that would reveal the proposer becoming expensive on some screen nobody tested.
func TestThePassReportsWhatDiscoveryCost(t *testing.T) {
	t.Setenv("MARCO_SHADOW_VISION", "screenparser")
	rt := &Runtime{engine: stubEngine{}}
	prov := newShadowProvider(blindDetector{}, drawnMenuCapture{lit: 1},
		rt.activeWindow, nil, time.Nanosecond)
	rt.collector = providers.NewCollector(prov)
	rt.shadowVision = prov
	rt.pinnedWindow = &windowRef1
	_ = sampleWithVision(t, rt)

	inner, ok := prov.Inner().(*vision.Provider)
	if !ok {
		t.Fatalf("the experiment's inner provider is %T", prov.Inner())
	}
	diag := inner.LastDiagnostics()
	if diag.StructureShapes == 0 {
		t.Errorf("the pass reports no proposed structure on a drawn menu, so a reader " +
			"cannot tell the pixel path ran from a screen with nothing on it")
	}
	if diag.Timings.Structure <= 0 {
		t.Error("the pass reports no cost for the proposer. The choice of discovery path " +
			"is a cadence decision, and the two timings beside each other ARE the " +
			"decision")
	}
	t.Logf("proposer: %d shapes in %s (detector: %s)",
		diag.StructureShapes, diag.Timings.Structure, diag.Timings.Detect)
}

// Model is part of the Detector interface: a backend says what it is, and "nothing" is a real
// answer that the default build gives.
func (blindDetector) Model() string { return "blind-test-detector" }

// In pixels mode the detector's own discarded geometry is NOT kept as well.
//
// The gate exists for a measured reason: on the default path the proposer has already described
// the frame, and keeping the detector's rectangles too hands grouping two proposals for every
// row. Two boxes a few pixels apart define a pitch of nearly zero, which is measured to split a
// six-row list into one partial stack and three strays — so a screen that was discovered
// perfectly produces no candidate the tracker will consider.
//
// A blind detector cannot show this, so this one finds nine unreadable rows.
//
// Deleting `p.Discovery.keepsModelGeometry()` from either keepShape gate in
// Provider.observations must fail this.
func TestPixelsModeDoesNotAlsoKeepTheDetectorsGeometry(t *testing.T) {
	t.Setenv("MARCO_SHADOW_VISION", "screenparser")
	rt := &Runtime{engine: stubEngine{}}
	prov := newShadowProvider(fixtureDetector{results: unreadableMenuFixture()},
		drawnMenuCapture{lit: 2}, rt.activeWindow, nil, time.Nanosecond)
	rt.collector = providers.NewCollector(prov)
	rt.shadowVision = prov
	rt.pinnedWindow = &windowRef1

	sample := sampleWithVision(t, rt)
	if sample.Shadow == nil {
		t.Fatal("no shadow record")
	}
	inner, ok := prov.Inner().(*vision.Provider)
	if !ok {
		t.Fatalf("inner provider is %T", prov.Inner())
	}
	diag := inner.LastDiagnostics()
	if diag.StructureShapes == 0 {
		t.Fatal("the proposer kept nothing, so this test cannot tell the two sources apart")
	}
	if diag.Counters.RejectedUnreadable == 0 {
		t.Fatal("the detector's nine unreadable rows were not refused, so there is no " +
			"second source of geometry to exclude and this proves nothing")
	}
	if got := len(sample.Shadow.Shapes); got != diag.StructureShapes {
		t.Errorf("the sample carries %d shapes and the proposer kept %d. The difference "+
			"is the detector's refused rectangles arriving as well, which gives "+
			"grouping two proposals per row and splits the list it just found",
			got, diag.StructureShapes)
	}
}
