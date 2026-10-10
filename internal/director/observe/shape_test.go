package observe_test

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A LIST NOBODY COULD NAME IS STILL A LIST, AND SAYS SO.
//
// Experiment-024's finding, as a test: on a real game menu the detector returns rectangles it
// cannot read, and the question "which of these holds the mark" never needed the words. These
// tests fold unnamed geometry through the same ShadowTotals.Observe the production path uses.
//
// The warning they also hold, in the words it was given in: *a region being detected is not
// proof that it is a menu entry … preserving 70 unreadable text regions could increase
// ambiguity rather than solve it.* So every belief built this way has to say what it is made
// of, and unnamed geometry must not quietly become named structure anywhere.

// shapeSample is one look at a list of n unnamed rows with one of them marked.
//
// NO regions at all — this is the measured case, where every box the detector returned came
// back classed `text` with nothing readable in it.
func shapeSample(seq, n, marked int) observe.Sample {
	var shapes []observe.ShadowShape
	for i := range n {
		shade := []float64{60, 72}
		if i == marked {
			shade = []float64{205, 238}
		}
		shapes = append(shapes, observe.ShadowShape{
			Region: observe.Region{
				X: 0.10, Y: 0.40 + float64(i)*0.06, Width: 0.40, Height: 0.05,
			},
			Confidence: 0.17, // the live band: 0.16–0.19 against a 0.15 floor
			Shade:      shade,
		})
	}
	return observe.Sample{
		Sequence:         seq,
		Timestamp:        sampleEpoch.Add(time.Duration(seq) * 500 * time.Millisecond),
		WindowGeneration: 1,
		Shadow: &observe.ShadowSample{
			Detector: "test", Ran: true, TargetProven: true, Shapes: shapes,
		},
	}
}

// The whole point: a menu made only of rectangles can be watched, and the mark on it read.
//
// Deleting look.Shapes in interfaceLookFrom must fail this.
func TestAListOfUnnamedRowsCanStillBeWatched(t *testing.T) {
	var totals observe.ShadowTotals
	// The mark moves between members, which is what earns a list its identity — a static
	// row of boxes is not evidence of anything being chosen from. Held for three looks
	// each, the same pattern the named path's fixtures use: the reader asks whether a mark
	// has settled, and a mark that moves every look has not.
	marks := []int{-1, -1, -1, -1, -1, -1,
		0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3}
	for i, m := range marks {
		totals.Observe(shapeSample(i+1, 6, m))
	}

	b := totals.Interface
	if b.Status != observe.InterfaceProvisional {
		t.Fatalf("interface %q after %d looks at a list of unnamed rows with a moving "+
			"mark, want provisional.\nbecause: %s", b.Status, len(marks), b.Because)
	}
	if b.Selected < 0 {
		t.Errorf("no entry read (%s); the reader never needed the words", b.Reading)
	}
	if b.Entries == 0 {
		t.Fatal("no entries")
	}
	if b.FromShapes != b.Entries {
		t.Errorf("FromShapes = %d of %d entries; every one of them came from geometry "+
			"nobody could name, and a belief that cannot say so is the ambiguity this "+
			"channel was warned about", b.FromShapes, b.Entries)
	}
	if !strings.Contains(b.Because, "nobody could name") {
		t.Errorf("the belief's reason does not say its entries are unnamed:\n  %s",
			b.Because)
	}
	if b.Complete {
		t.Error("a belief built from unnamed geometry claims its entries are all of them")
	}
}

// Unnamed geometry does not become named structure. The two counts stay apart all the way
// out, because one screen of twenty named buttons and one of seventy nameless rectangles call
// for opposite conclusions and a single number cannot carry both.
func TestUnnamedGeometryIsNotCountedAsADetection(t *testing.T) {
	var totals observe.ShadowTotals
	for i := range 4 {
		totals.Observe(shapeSample(i+1, 6, i%6))
	}
	if totals.Detections != 0 {
		t.Errorf("detections = %d from four looks that produced only shapes; a rectangle "+
			"nobody could name is not a detection", totals.Detections)
	}
	if totals.Shapes != 24 {
		t.Errorf("shapes = %d, want 24 (4 looks × 6 rows)", totals.Shapes)
	}
	if totals.ShapesShaded != 24 {
		t.Errorf("shapes_shaded = %d, want 24", totals.ShapesShaded)
	}
	if len(totals.Roles) != 0 {
		t.Errorf("roles = %v; unnamed geometry has no role to be counted under",
			totals.Roles)
	}
}

// And it does not change what SCREEN the session thinks it is on.
//
// Deliberate, and the sharpest form of the ambiguity warning. A screen state is a composition
// of named roles; letting seventy nameless rectangles into it would make every frame a new
// composition, destroy screen identity, and produce a transition per look — which is precisely
// the failure the confirmation gate was built to stop, arriving by a different door.
func TestUnnamedGeometryDoesNotChangeTheScreenComposition(t *testing.T) {
	var withShapes, without observe.ShadowTotals
	for i := range 6 {
		s := shapeSample(i+1, 6, i%6)
		withShapes.Observe(s)

		bare := s
		stripped := *s.Shadow
		stripped.Shapes = nil
		bare.Shadow = &stripped
		without.Observe(bare)
	}
	if got, want := len(withShapes.States), len(without.States); got != want {
		t.Errorf("%d screen state(s) with shapes against %d without; shapes must not "+
			"reach segmentation", got, want)
	}
	if got, want := len(withShapes.Crossings), len(without.Crossings); got != want {
		t.Errorf("%d crossing(s) with shapes against %d without", got, want)
	}
}

// A rectangle nobody measured is not a dark row.
//
// The same rule the named path already follows, and it is the one that decides whether a fast
// read may be trusted at all: "nobody measured this" and "this is the darkest thing on screen"
// are different answers, and defaulting the first to zero makes them the same.
func TestAnUnmeasuredShapeIsNotADarkRow(t *testing.T) {
	var totals observe.ShadowTotals
	for i := range 12 {
		s := shapeSample(i+1, 6, i%6)
		// Strip the shading from every row but keep the geometry: a pass where the
		// pixels could not be measured.
		stripped := *s.Shadow
		rows := make([]observe.ShadowShape, 0, len(stripped.Shapes))
		for _, sh := range stripped.Shapes {
			sh.Shade = nil
			rows = append(rows, sh)
		}
		stripped.Shapes = rows
		s.Shadow = &stripped
		totals.Observe(s)
	}
	if totals.Shapes != 72 {
		t.Errorf("shapes = %d, want 72: unmeasured geometry is still geometry and is "+
			"still reported", totals.Shapes)
	}
	if totals.ShapesShaded != 0 {
		t.Errorf("shapes_shaded = %d, want 0", totals.ShapesShaded)
	}
	if b := totals.Interface; b.Selected >= 0 {
		t.Errorf("entry %d read as selected from rows nobody measured (%s)",
			b.Selected, b.Reading)
	}
}

// Named structure and unnamed geometry from the SAME look are one observation of the screen.
//
// Handing them to the belief as two looks would make a single frame read as two observations,
// which is the arithmetic that turned one flickering inference into two transitions.
func TestNamedAndUnnamedGeometryFromOneLookAreOneLook(t *testing.T) {
	var totals observe.ShadowTotals
	for i := range 5 {
		s := menuSample(i+1, 6, i%6)
		mixed := *s.Shadow
		mixed.Shapes = shapeSample(i+1, 3, 0).Shadow.Shapes
		s.Shadow = &mixed
		totals.Observe(s)
	}
	if got := totals.Interface.Observations; got != 5 {
		t.Errorf("the belief counted %d looks from 5 samples; a sample is one look at the "+
			"screen however many kinds of evidence it carried", got)
	}
}
