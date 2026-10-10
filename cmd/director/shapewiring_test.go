package main

import (
	"context"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// GEOMETRY THE DETECTOR COULD NOT NAME, THROUGH THE PRODUCTION PATH.
//
// Experiment-024 is the reason this exists. A player navigated a real Trackmania menu with a
// controller for four minutes; every press was observed and the menu never was, because the
// detector returns ~70 boxes a frame on that game and nearly all of them come back classed
// `text` with nothing readable inside. Those were discarded before selection.Measure could ask
// how any of them was painted.
//
// These enter through the composition production uses:
//
//	painted capture → detector of unreadable text → newShadowProvider
//	  → providers.Collector → liveSampler.Sample → ShadowSample.Shapes → renderShadow
//
// Nothing here calls a helper to see what it returns. Three separate mechanisms in this
// repository have been written, unit-tested and left uncalled; the tests that would have
// caught it are the ones that enter through the constructor production enters through.

// ── the fixture ───────────────────────────────────────────────────────────────

// unreadableMenuFixture is nine rows of text nobody can read, which is the measured case.
//
// No `button`, no `menu`, no readable word anywhere: if this fixture produced one named
// control the tests below would pass on evidence the repair did not provide.
func unreadableMenuFixture() []vision.Detection {
	var out []vision.Detection
	for i := range 9 {
		y := 300 + i*60
		out = append(out, vision.Detection{
			Class: "text", Bounds: image.Rect(700, y, 1100, y+50),
			// The live band, 0.16–0.19 against a 0.15 floor. Chosen rather than a
			// comfortable 0.9: a repair that only works on confident detections does
			// not work on the screen it was built for.
			Confidence: 0.17,
		})
	}
	return out
}

// markedRow is which row the painted capture lights up, so a reading has something to find.
const markedRow = 3

// paintedFixtureCapture is a frame with a ground and one bright row.
//
// A separate capture from fixtureCapture, which paints a nearly black image: a region measuring
// {0,0} is indistinguishable from a region nobody measured, by design, so on that frame every
// shape would arrive unshaded and the selection half of this would be untestable.
type paintedFixtureCapture struct{ generation uint64 }

func (c paintedFixtureCapture) CaptureWindow(_ context.Context,
	w directorapi.Window) (capture.Image, error) {

	img := image.NewRGBA(image.Rect(0, 0, w.Bounds.Width, w.Bounds.Height))
	for y := range w.Bounds.Height {
		for x := range w.Bounds.Width {
			img.SetRGBA(x, y, color.RGBA{40, 40, 40, 255})
		}
	}
	lit := unreadableMenuFixture()[markedRow].Bounds
	for y := lit.Min.Y; y < lit.Max.Y; y++ {
		for x := lit.Min.X; x < lit.Max.X; x++ {
			img.SetRGBA(x, y, color.RGBA{215, 215, 215, 255})
		}
	}
	return capture.Image{
		Image: img, Bounds: w.Bounds, Scale: 1, CapturedAt: time.Now(),
		WindowID: w.ID, Application: w.Application, WindowBoundsAtCapture: w.Bounds,
		Target: &directorapi.TargetProvenance{
			Application: w.Application, ProcessID: 7, WindowGeneration: c.generation,
		},
	}, nil
}

// ── the wiring ────────────────────────────────────────────────────────────────

// The experiment's provider keeps what it cannot read. The authoritative one does not.
//
// Both halves in one test on purpose: the repair was authorised on the condition that the
// authoritative provider's text acceptance did not change, and a test that checked only the
// first half would pass on a day somebody set the flag in vision.New.
//
// Deleting `inner.KeepUnreadableShapes = true` in newShadowProvider must fail this.
func TestTheExperimentKeepsGeometryItCannotRead(t *testing.T) {
	rt := &Runtime{engine: stubEngine{}}
	prov := newShadowProvider(fixtureDetector{results: unreadableMenuFixture()},
		paintedFixtureCapture{generation: 1}, rt.activeWindow, nil, time.Nanosecond)

	inner, ok := prov.Inner().(*vision.Provider)
	if !ok {
		t.Fatalf("the experiment's inner provider is %T, not a vision provider", prov.Inner())
	}
	if !inner.KeepUnreadableShapes {
		t.Error("the EXPERIMENT's provider discards the geometry of text it cannot read. " +
			"On the measured game that is ~70 rectangles a frame, and the selection " +
			"reader needs exactly those rectangles and none of their words")
	}

	// And the authoritative one, built by the same constructor production uses for it.
	plain := vision.New(fixtureDetector{results: unreadableMenuFixture()},
		paintedFixtureCapture{generation: 1}, rt.activeWindow)
	if plain.KeepUnreadableShapes {
		t.Error("vision.New returns a provider that keeps unreadable geometry. The " +
			"authoritative provider's text acceptance was to be left exactly as it " +
			"was, and this is the default every provider in the tree inherits")
	}
}

// The sample carries it. One mechanism per layer is not enough: the provider can keep geometry
// all day with nothing copying it onto the value that reaches the session.
//
// Deleting `sh.Shapes = shadowShapes(...)` in liveSampler.Sample must fail this.
func TestTheSampleCarriesGeometryTheDetectorCouldNotName(t *testing.T) {
	rt := visionShadowRuntime(t, unreadableMenuFixture(), nil,
		paintedFixtureCapture{generation: 1})
	sample := sampleWithVision(t, rt)

	if sample.Shadow == nil {
		t.Fatal("no shadow record, so geometry has nowhere to ride")
	}
	if sample.Shadow.Detections != 0 {
		t.Fatalf("the fixture produced %d admitted detections; it is supposed to produce "+
			"only text nobody can read, and this test would prove nothing",
			sample.Shadow.Detections)
	}
	shapes := sample.Shadow.Shapes
	if len(shapes) != 9 {
		t.Fatalf("the sample carries %d shapes from 9 unreadable rows, want 9.\nThe "+
			"detector found them, refused them as evidence, and nothing carried the "+
			"rectangles — which is the whole of what Experiment-024 measured", len(shapes))
	}
	var shaded, bright int
	for _, sh := range shapes {
		r := sh.Region
		if r.X < 0 || r.Y < 0 || r.X+r.Width > 1.0001 || r.Y+r.Height > 1.0001 {
			t.Errorf("shape region %+v is not window-relative and normalised; nothing "+
				"downstream of the sampler may hold a screen coordinate", r)
		}
		if sh.Shaded() {
			shaded++
		}
		if len(sh.Shade) == 2 && sh.Shade[0] > 150 {
			bright++
		}
	}
	if shaded != 9 {
		t.Errorf("%d of 9 shapes carry a measurement; without one a rectangle cannot say "+
			"whether it holds the mark", shaded)
	}
	if bright != 1 {
		t.Errorf("%d shapes read as brightly painted, want exactly 1 — the row the "+
			"capture lit. The shading is measured where the pixels are, and if it does "+
			"not survive to here the fast path has nothing to compare", bright)
	}
}

// ── the report ────────────────────────────────────────────────────────────────

// unnamedListSession is a session that watched a mark move down a list of nameless rectangles.
func unnamedListSession(t *testing.T) observe.ShadowTotals {
	t.Helper()
	var totals observe.ShadowTotals
	seq := 0
	for _, m := range []int{-1, -1, -1, -1, -1, -1,
		0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3} {
		seq++
		var shapes []observe.ShadowShape
		for i := range 6 {
			shade := []float64{60, 72}
			if i == m {
				shade = []float64{205, 238}
			}
			shapes = append(shapes, observe.ShadowShape{
				Region: observe.Region{
					X: 0.10, Y: 0.40 + float64(i)*0.06, Width: 0.40, Height: 0.05,
				},
				Confidence: 0.17, Shade: shade,
			})
		}
		totals.Observe(observe.Sample{
			Sequence: seq, WindowGeneration: 1,
			Timestamp: time.Unix(1760000000, 0).Add(time.Duration(seq) * 500 * time.Millisecond),
			Shadow: &observe.ShadowSample{
				Detector: "screenparser", Ran: true, TargetProven: true,
				Shapes: shapes, LatencyMS: 1042,
			},
		})
	}
	if totals.Interface.Status != observe.InterfaceProvisional {
		t.Fatalf("precondition: no list identified (%s)", totals.Interface.Because)
	}
	return totals
}

// The report counts unnamed geometry APART from detections.
//
// The warning this repair was authorised under: a region being detected is not proof that it is
// a menu entry. One number mixing named structure with nameless rectangles is how a screen of
// twenty buttons and a screen of seventy nameless boxes become indistinguishable in a report —
// and they call for opposite conclusions.
//
// Deleting the geometry line in renderShadow must fail this.
func TestTheReportCountsUnnamedGeometryApart(t *testing.T) {
	var b strings.Builder
	renderShadow(&b, unnamedListSession(t))
	out := b.String()

	if !strings.Contains(out, "geometry ") {
		t.Errorf("the report never mentions the geometry it kept without a name:\n%s", out)
	}
	if !strings.Contains(out, "WITHOUT a name") {
		t.Error("the geometry line does not say the rectangles have no name")
	}
	if !strings.Contains(out, "not a menu entry") {
		t.Error("the report presents nameless rectangles without the caution that a " +
			"rectangle is not a menu entry. A reader who takes the count at face value " +
			"has been told something the evidence does not support")
	}
	// And the detections line must still be honest about there being none.
	if !strings.Contains(out, "detections   0 total") {
		t.Errorf("a session whose every box was nameless reports detections:\n%s", out)
	}
}

// And the belief says its entries have no names, in the place a person reads.
//
// Deleting the FromShapes line in renderInterface must fail this.
func TestTheReportSaysWhenAListHasNoNames(t *testing.T) {
	var b strings.Builder
	renderShadow(&b, unnamedListSession(t))
	out := b.String()

	if !strings.Contains(out, "geometry nobody could name") {
		t.Errorf("the interface panel claims a list of entries without saying that none "+
			"of them has a name:\n%s", out)
	}
	if !strings.Contains(out, "ALL of them") {
		t.Error("a list where every entry is nameless does not say so. 'some of them' " +
			"and 'all of them' are different claims about how much is known")
	}
}
