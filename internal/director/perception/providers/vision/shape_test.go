package vision_test

import (
	"context"
	"image"
	"reflect"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// THE GEOMETRY OF WHAT NOBODY COULD READ.
//
// Experiment-024 watched a player navigate a real Trackmania menu with a controller. Every
// press was observed and the menu never was: the detector returns ~70 boxes a frame on that
// game's screens, nearly all classed `text`, and a `text` box whose words nobody could read was
// discarded before selection.Measure could ask how it was painted.
//
// These tests hold both halves of the repair: the geometry survives on the experiment's
// instance, and nothing about the authoritative one changed.

// rows reports one detection per rectangle, with the class and text it is given.
type rows struct {
	at    []image.Rectangle
	class string
	text  string
	conf  float64
}

func (d *rows) Detect(context.Context, vision.Input) ([]vision.Detection, error) {
	conf := d.conf
	if conf == 0 {
		conf = 0.9
	}
	out := make([]vision.Detection, 0, len(d.at))
	for _, r := range d.at {
		out = append(out, vision.Detection{
			Class: d.class, Bounds: r, Confidence: conf, Text: d.text,
		})
	}
	return out, nil
}
func (d *rows) Model() string { return "rows" }

func menuBand() image.Rectangle { return image.Rect(40, 100, 360, 140) }

// menuRows is nine rectangles down the frame, one of which is the painted band.
func menuRows() []image.Rectangle {
	var out []image.Rectangle
	for i := range 9 {
		y := 60 + i*20
		out = append(out, image.Rect(40, y, 360, y+18))
	}
	out[2] = menuBand()
	return out
}

func shapeProvider(t *testing.T, d vision.Detector, keep bool) *vision.Provider {
	t.Helper()
	win := &countingWindow{
		window: directorapi.Window{ID: "hwnd:1", Application: "testapp",
			Bounds: directorapi.Rect{Width: 400, Height: 300}},
		ok: true,
	}
	p := vision.New(d, &paintedCapture{band: menuBand()}, win.active)
	p.KeepUnreadableShapes = keep
	return p
}

func shapeLook(t *testing.T, p *vision.Provider) observation.ProviderOutcome {
	t.Helper()
	return p.ObserveTargeted(context.Background(), observation.Request{
		Sources: []directorapi.ObservationSource{directorapi.SourceVision},
	})
}

// The case the repair exists for: text regions the detector could not read itself.
//
// Nine rows arrive, none of them readable. Before this, nine rejections and nothing to show
// for them. After it, nine rectangles with shading — and still nine rejections, because what
// was refused as EVIDENCE has not changed at all.
func TestAnUnreadableTextRegionKeepsItsGeometry(t *testing.T) {
	p := shapeProvider(t, &rows{at: menuRows(), class: "text"}, true)
	out := shapeLook(t, p)

	d := p.LastDiagnostics()
	if len(d.Shapes) != 9 {
		t.Fatalf("kept %d shapes from 9 unreadable text rows, want 9", len(d.Shapes))
	}
	if d.Counters.RejectedUnreadable != 9 {
		t.Errorf("RejectedUnreadable = %d, want 9: keeping the geometry must not change "+
			"what was refused as evidence", d.Counters.RejectedUnreadable)
	}
	if len(out.Observations) != 0 {
		t.Errorf("%d observation(s) from unreadable text; a kept shape must never become "+
			"evidence", len(out.Observations))
	}
	var measured int
	for _, sh := range d.Shapes {
		if len(sh.Shade) == 2 {
			measured++
		}
		if sh.Read {
			t.Error("a shape from an unreadable region says it was read")
		}
	}
	if measured != 9 {
		t.Errorf("%d of 9 shapes carry shading; without it the selection reader has "+
			"nothing to work on and the whole repair is pointless", measured)
	}
}

// The other half, and the condition the repair was authorised under: the authoritative
// provider's text acceptance is untouched.
//
// Same detector, same frame, flag unset — which is how every provider in the tree is
// constructed except the experiment's one.
func TestTheAuthoritativeProviderStillDiscardsUnreadableText(t *testing.T) {
	p := shapeProvider(t, &rows{at: menuRows(), class: "text"}, false)
	out := shapeLook(t, p)

	d := p.LastDiagnostics()
	if len(d.Shapes) != 0 {
		t.Errorf("an instance with KeepUnreadableShapes unset kept %d shapes; the "+
			"authoritative path must behave exactly as it did", len(d.Shapes))
	}
	if d.Counters.RejectedUnreadable != 9 {
		t.Errorf("RejectedUnreadable = %d, want 9", d.Counters.RejectedUnreadable)
	}
	if len(out.Observations) != 0 {
		t.Errorf("%d observation(s); unchanged means none", len(out.Observations))
	}
}

// A kept shape is not a control, and the type is what stops it becoming one.
//
// The strongest statement available here: there is no path from a shape into the observation
// stream, so there is no path from one into fusion, World State, or actionability. A test that
// only counted observations would pass on a day somebody added a Role field to Shape; this one
// asserts the absence of anywhere to put it.
func TestAKeptShapeCarriesNothingThatCouldNameAControl(t *testing.T) {
	p := shapeProvider(t, &rows{at: menuRows(), class: "text"}, true)
	shapeLook(t, p)
	shapes := p.LastDiagnostics().Shapes
	if len(shapes) == 0 {
		t.Fatal("nothing kept, so nothing to check")
	}
	// Reflection over the type, deliberately: the rule is about what the type can hold,
	// not about what one instance happens to hold today.
	forbidden := []string{"Role", "Label", "Text", "Name", "NativeID", "ID", "Actionable"}
	tn := reflectFieldNames(shapes[0])
	for _, f := range forbidden {
		for _, have := range tn {
			if have == f {
				t.Errorf("vision.Shape has a %s field. A shape is a rectangle and how "+
					"it is painted; anything that could name a control belongs on an "+
					"Element, which has to earn its way past the structural gate", f)
			}
		}
	}
}

// An image region is not kept, and this is a deliberate asymmetry.
//
// A `text` box is somewhere something WROTE something, so a row of them at uniform spacing is
// an argument about arrangement. An `image` box has no rows in it by construction, and
// measuring one would invite exactly the reading the whole design refuses: that a rectangle is
// a menu entry because something was found there.
func TestAnImageRegionIsNotKept(t *testing.T) {
	p := shapeProvider(t, &rows{at: menuRows(), class: "image"}, true)
	shapeLook(t, p)
	if n := len(p.LastDiagnostics().Shapes); n != 0 {
		t.Errorf("kept %d shapes from image regions, want 0", n)
	}
}

// Only VALID geometry is kept. A box below the confidence floor was never evidence about
// anything and must not become geometry about anything either — the gates above the
// non-structural branch are the whole reason a shape can be called valid at all.
func TestGeometryThatFailedTheFloorsIsNotKept(t *testing.T) {
	p := shapeProvider(t, &rows{at: menuRows(), class: "text", conf: 0.01}, true)
	shapeLook(t, p)
	if n := len(p.LastDiagnostics().Shapes); n != 0 {
		t.Errorf("kept %d shapes from detections below the confidence floor, want 0", n)
	}
}

// A region the detector DID read is kept too, and says so.
//
// The channel must not appear and disappear with the label budget. On the measured session
// that budget bought a reading on roughly one sample in six, and geometry that existed only on
// those samples would be a cadence rather than a signal — so the shape is recorded either way
// and `Read` carries the difference.
func TestReadableTextKeepsItsGeometryAndSaysItWasRead(t *testing.T) {
	p := shapeProvider(t, &rows{at: menuRows(), class: "text", text: "RESUME"}, true)
	out := shapeLook(t, p)

	shapes := p.LastDiagnostics().Shapes
	if len(shapes) != 9 {
		t.Fatalf("kept %d shapes, want 9", len(shapes))
	}
	for _, sh := range shapes {
		if !sh.Read {
			t.Error("a shape from a region the detector read says it was not read")
			break
		}
	}
	// And it is still accepted as text evidence, exactly as before: this changes what is
	// kept, never what is admitted.
	var texts int
	for _, o := range out.Observations {
		if _, ok := o.(observation.Text); ok {
			texts++
		}
	}
	if texts != 9 {
		t.Errorf("%d text observation(s) from 9 readable rows; keeping geometry must not "+
			"change what the provider admits", texts)
	}
}

// The bound exists and holds. A pass past it is a pass whose geometry is already incomparable
// with any other's, so there is nothing to lose — but an unbounded slice on a per-cycle
// diagnostic is how a diagnostic becomes a payload.
func TestKeptGeometryIsBounded(t *testing.T) {
	var many []image.Rectangle
	for i := range 400 {
		y := (i % 140) * 2
		many = append(many, image.Rect(40+(i/140)*60, y, 90+(i/140)*60, y+12))
	}
	p := shapeProvider(t, &rows{at: many, class: "text"}, true)
	shapeLook(t, p)
	n := len(p.LastDiagnostics().Shapes)
	if n == 0 {
		t.Fatal("nothing kept from 400 detections")
	}
	if n > 256 {
		t.Errorf("kept %d shapes; the per-pass bound is 256 and an unbounded per-cycle "+
			"diagnostic is how a diagnostic becomes a payload", n)
	}
}

// reflectFieldNames lists a struct's exported field names.
func reflectFieldNames(v any) []string {
	t := reflect.TypeOf(v)
	out := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		out = append(out, t.Field(i).Name)
	}
	return out
}
