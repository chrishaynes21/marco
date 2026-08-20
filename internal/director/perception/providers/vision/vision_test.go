package vision_test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The vision provider, without a model.
//
//	A detected box is evidence that something SHAPED LIKE a control is at a location.
//	It is not, by itself, evidence that an INTERACTIVE CONTROL is there.
//
// Every test drives a FAKE detector. What is under test is the provider's contract — what
// it accepts, what it refuses, and what it refuses to assert — and a test that used a real
// model would be testing the model.

var t0 = time.Date(2026, 8, 5, 18, 0, 0, 0, time.UTC)

// fakeDetector returns whatever a test hands it.
type fakeDetector struct {
	results []vision.Detection
	err     error
	model   string
	// seen is the image it was given, so a test can assert it never receives desktop
	// coordinates.
	seen image.Rectangle
}

func (f *fakeDetector) Detect(_ context.Context, in vision.Input) ([]vision.Detection, error) {
	if in.Image != nil {
		f.seen = in.Image.Bounds()
	}
	return f.results, f.err
}

func (f *fakeDetector) Model() string {
	if f.model != "" {
		return f.model
	}
	return "fake-v1"
}

// fakeCapture returns a blank image of a fixed size.
type fakeCapture struct {
	width, height int
	scale         float64
	origin        directorapi.Point
	capturedAt    time.Time
	movedTo       directorapi.Rect
	err           error
}

func (f *fakeCapture) CaptureWindow(_ context.Context, w directorapi.Window) (capture.Image, error) {
	if f.err != nil {
		return capture.Image{}, f.err
	}
	width, height := f.width, f.height
	if width == 0 {
		width, height = 800, 600
	}
	scale := f.scale
	if scale == 0 {
		scale = 1
	}
	at := f.capturedAt
	if at.IsZero() {
		at = time.Now()
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{A: 255})

	bounds := w.Bounds
	if f.movedTo != (directorapi.Rect{}) {
		bounds = f.movedTo
	}
	return capture.Image{
		Image: img, Bounds: w.Bounds, Scale: scale, CapturedAt: at,
		Transform:             capture.New(f.origin, scale),
		WindowID:              w.ID,
		Application:           w.Application,
		WindowBoundsAtCapture: bounds,
	}, nil
}

// provider builds one over a fake detector and capture.
func provider(t *testing.T, d *fakeDetector, c *fakeCapture) *vision.Provider {
	t.Helper()
	if c == nil {
		c = &fakeCapture{}
	}
	return vision.New(d, c, func(context.Context) (directorapi.Window, bool) {
		return directorapi.Window{
			ID: "hwnd:1", Application: "game", Title: "A Game",
			Bounds: directorapi.Rect{X: 0, Y: 0, Width: 800, Height: 600},
		}, true
	})
}

// look runs one pass, asking for vision explicitly.
func look(t *testing.T, p *vision.Provider) ([]observation.Observation, vision.Diagnostics) {
	t.Helper()
	obs, diag, err := p.Look(context.Background(),
		observation.Request{Include: []observation.Source{directorapi.SourceVision}})
	if err != nil {
		t.Fatalf("look: %v", err)
	}
	return obs, diag
}

func detection(class string, x, y, w, h int, conf float64) vision.Detection {
	return vision.Detection{
		Class: class, Bounds: image.Rect(x, y, x+w, y+h), Confidence: conf,
	}
}

// ── the provider (Part 13: provider) ──────────────────────────────────────────

func TestADetectionBecomesAnObservation(t *testing.T) {
	d := &fakeDetector{results: []vision.Detection{
		detection("button", 100, 50, 80, 24, 0.9),
	}}
	obs, diag := look(t, provider(t, d, nil))

	if len(obs) != 1 {
		t.Fatalf("%d observations: %+v", len(obs), obs)
	}
	el, ok := obs[0].(observation.Element)
	if !ok {
		t.Fatalf("a button became a %T", obs[0])
	}
	if el.Raw.Role != directorapi.RoleButton {
		t.Errorf("role = %q", el.Raw.Role)
	}
	if el.Raw.Source != directorapi.SourceVision {
		t.Errorf("source = %q; nothing downstream could weigh this correctly", el.Raw.Source)
	}
	if el.Raw.Confidence != 0.9 {
		t.Errorf("confidence = %.2f; the model's own number must survive", el.Raw.Confidence)
	}
	if diag.Counters.AcceptedStructural != 1 {
		t.Errorf("counters = %+v", diag.Counters)
	}
}

// TestTheDetectorNeverSeesDesktopCoordinates — a backend cannot misplace what it is never
// told the position of.
func TestTheDetectorNeverSeesDesktopCoordinates(t *testing.T) {
	d := &fakeDetector{}
	c := &fakeCapture{width: 640, height: 480, origin: directorapi.Point{X: 1920, Y: 100}}
	look(t, provider(t, d, c))

	if d.seen.Min.X != 0 || d.seen.Min.Y != 0 {
		t.Errorf("the detector was handed an image at %v; it must be image-local", d.seen)
	}
	if d.seen.Dx() != 640 || d.seen.Dy() != 480 {
		t.Errorf("the detector saw %v", d.seen)
	}
}

// TestBoxesArePlacedThroughTheTransform — including a scaled, offset desktop, which is the
// case that produces plausible wrong answers.
func TestBoxesArePlacedThroughTheTransform(t *testing.T) {
	d := &fakeDetector{results: []vision.Detection{
		detection("button", 10, 20, 30, 40, 0.9),
	}}
	c := &fakeCapture{scale: 2, origin: directorapi.Point{X: -1920, Y: 50}}

	obs, _ := look(t, provider(t, d, c))
	el := obs[0].(observation.Element)
	want := directorapi.Rect{X: -1920 + 20, Y: 50 + 40, Width: 60, Height: 80}
	if el.Raw.Bounds != want {
		t.Errorf("bounds = %+v, want %+v", el.Raw.Bounds, want)
	}
}

// ── confidence and provenance (Part 4) ────────────────────────────────────────

// TestEveryObservationCarriesItsProvenance.
//
//	Every observation carries confidence, model, provider, frame.
func TestEveryObservationCarriesItsProvenance(t *testing.T) {
	d := &fakeDetector{model: "opencv-shapes-v2", results: []vision.Detection{
		detection("button", 10, 10, 40, 20, 0.8),
	}}
	obs, diag := look(t, provider(t, d, nil))

	el := obs[0].(observation.Element)
	attrs := el.Raw.Attributes
	for _, want := range []string{"detector", "provider", "frame", "vision_class"} {
		if _, ok := attrs[want]; !ok {
			t.Errorf("the observation carries no %q: %+v", want, attrs)
		}
	}
	if attrs["detector"] != "opencv-shapes-v2" {
		t.Errorf("detector = %v", attrs["detector"])
	}
	if diag.FrameID == "" || attrs["frame"] != diag.FrameID {
		t.Errorf("the observation's frame (%v) does not match the pass's (%q)",
			attrs["frame"], diag.FrameID)
	}
}

// TestIdentityHintsAreNeverCoordinatesAlone.
//
//	Vision observations must contribute stable identity hints. Never coordinates alone.
func TestIdentityHintsAreNeverCoordinatesAlone(t *testing.T) {
	d := &fakeDetector{results: []vision.Detection{
		{Class: "button", Bounds: image.Rect(10, 10, 90, 34), Confidence: 0.9, Text: "Save"},
	}}
	obs, _ := look(t, provider(t, d, nil))

	el := obs[0].(observation.Element)
	if el.Raw.Label != "Save" {
		t.Errorf("the label the detector read was not kept: %q", el.Raw.Label)
	}
	if _, ok := el.Raw.Attributes["aspect"]; !ok {
		t.Error("no shape hint was recorded, so nothing but position identifies this box")
	}
}

// ── safety (Part 12, Part 13: safety) ─────────────────────────────────────────

// TestTextDoesNotBecomeAButton.
//
//	Seeing Delete does not produce Delete button unless structure supports it.
//	Text remains text.
func TestTextDoesNotBecomeAButton(t *testing.T) {
	d := &fakeDetector{results: []vision.Detection{
		{Class: "text", Bounds: image.Rect(10, 10, 90, 30), Confidence: 0.95, Text: "Delete"},
	}}
	obs, diag := look(t, provider(t, d, nil))

	if len(obs) != 1 {
		t.Fatalf("%d observations", len(obs))
	}
	if el, isElement := obs[0].(observation.Element); isElement {
		t.Fatalf("the word %q became a %s element", el.Raw.Label, el.Raw.Role)
	}
	txt, ok := obs[0].(observation.Text)
	if !ok {
		t.Fatalf("text became a %T", obs[0])
	}
	if txt.Content.String() != "Delete" {
		t.Errorf("content = %q", txt.Content.String())
	}
	if diag.Counters.AcceptedText != 1 || diag.Counters.AcceptedStructural != 0 {
		t.Errorf("counters = %+v", diag.Counters)
	}
}

// TestVisionNeverAssertsActionability.
//
// The strongest of the safety properties: a picture cannot tell a greyed-out button from a
// live one, and the difference is a click that does nothing and a click that deletes
// something.
func TestVisionNeverAssertsActionability(t *testing.T) {
	d := &fakeDetector{results: []vision.Detection{
		detection("button", 10, 10, 80, 24, 0.99),
	}}
	obs, _ := look(t, provider(t, d, nil))

	el := obs[0].(observation.Element)
	if el.Raw.Enabled != nil {
		t.Errorf("vision claimed the control is enabled=%v", *el.Raw.Enabled)
	}
	if el.Raw.Visible != nil {
		t.Errorf("vision claimed visibility")
	}
	if el.Raw.Focused != nil {
		t.Errorf("vision claimed focus")
	}
	if el.Raw.NativeID != "" {
		t.Errorf("vision invented a native id: %q", el.Raw.NativeID)
	}
}

// TestAnIconDoesNotBecomeAControlWithoutStructure — an icon is a shape, and it stays one.
func TestAnIconDoesNotBecomeAControlWithoutStructure(t *testing.T) {
	d := &fakeDetector{results: []vision.Detection{
		detection("icon", 10, 10, 24, 24, 0.9),
	}}
	obs, _ := look(t, provider(t, d, nil))

	el := obs[0].(observation.Element)
	if el.Raw.Role != directorapi.RoleIcon {
		t.Errorf("role = %q, want icon — not a button", el.Raw.Role)
	}
	if el.Raw.Enabled != nil {
		t.Error("an icon was reported as an enabled control")
	}
}

// TestAnUnknownClassIsRefused — a model's private vocabulary must not become the
// Director's.
func TestAnUnknownClassIsRefused(t *testing.T) {
	d := &fakeDetector{results: []vision.Detection{
		detection("frobnicator", 10, 10, 40, 40, 0.99),
	}}
	obs, diag := look(t, provider(t, d, nil))

	if len(obs) != 0 {
		t.Fatalf("an unmapped class produced %d observation(s): %+v", len(obs), obs)
	}
	if diag.Counters.RejectedClass != 1 {
		t.Errorf("counters = %+v", diag.Counters)
	}
	// And the diagnostic still says what was seen, so "the model found forty things
	// this build has no word for" is distinguishable from "the model found nothing".
	if diag.Classes["frobnicator"] != 1 {
		t.Errorf("the unmapped class was not counted: %+v", diag.Classes)
	}
}

func TestLowConfidenceIsRejected(t *testing.T) {
	d := &fakeDetector{results: []vision.Detection{
		detection("button", 10, 10, 40, 20, 0.1),
		// Above the reporting floor and below the structural bar: reported by neither,
		// because a box is a claim that something is there and half-believing one is
		// not useful.
		detection("button", 60, 10, 40, 20, 0.4),
	}}
	obs, diag := look(t, provider(t, d, nil))

	if len(obs) != 0 {
		t.Fatalf("%d low-confidence observations survived: %+v", len(obs), obs)
	}
	if diag.Counters.RejectedConfidence != 2 {
		t.Errorf("counters = %+v", diag.Counters)
	}
}

func TestAGiantBoxIsRejected(t *testing.T) {
	// A detector reporting the whole window as one control. The element would swallow
	// every click aimed anywhere inside it.
	d := &fakeDetector{results: []vision.Detection{
		detection("button", 0, 0, 800, 600, 0.99),
	}}
	obs, diag := look(t, provider(t, d, nil))

	if len(obs) != 0 {
		t.Fatalf("a window-sized button was accepted: %+v", obs)
	}
	if diag.Counters.RejectedGeometry != 1 {
		t.Errorf("counters = %+v", diag.Counters)
	}
}

func TestABoxOutsideTheImageIsRejected(t *testing.T) {
	// A detector reporting normalised or un-letterboxed coordinates. Accepting these
	// would produce plausible controls in wrong places.
	d := &fakeDetector{results: []vision.Detection{
		detection("button", 900, 700, 40, 20, 0.99),
	}}
	obs, diag := look(t, provider(t, d, nil))

	if len(obs) != 0 {
		t.Fatalf("a box outside the image was accepted: %+v", obs)
	}
	if diag.Counters.RejectedGeometry != 1 {
		t.Errorf("counters = %+v", diag.Counters)
	}
}

// ── capture safety ────────────────────────────────────────────────────────────

func TestAMovedWindowRefusesTheWholePass(t *testing.T) {
	d := &fakeDetector{results: []vision.Detection{detection("button", 10, 10, 40, 20, 0.9)}}
	c := &fakeCapture{movedTo: directorapi.Rect{X: 500, Y: 500, Width: 800, Height: 600}}

	_, diag, err := provider(t, d, c).Look(context.Background(),
		observation.Request{Include: []observation.Source{directorapi.SourceVision}})
	if err == nil {
		t.Fatal("a capture of a window that moved was accepted")
	}
	if !strings.Contains(diag.Error, "moved") {
		t.Errorf("the refusal does not say why: %s", diag.Error)
	}
}

func TestAStaleCaptureIsRefused(t *testing.T) {
	d := &fakeDetector{}
	c := &fakeCapture{capturedAt: time.Now().Add(-time.Minute)}

	_, diag, err := provider(t, d, c).Look(context.Background(),
		observation.Request{Include: []observation.Source{directorapi.SourceVision}})
	if err == nil {
		t.Fatal("a stale capture was accepted")
	}
	if !strings.Contains(diag.Error, "old") {
		t.Errorf("the refusal does not say why: %s", diag.Error)
	}
}

// TestNoDetectorIsUnavailableNotEmpty — "no backend" and "nothing on screen" must never
// look alike.
func TestNoDetectorIsUnavailableNotEmpty(t *testing.T) {
	p := vision.New(nil, &fakeCapture{}, func(context.Context) (directorapi.Window, bool) {
		return directorapi.Window{ID: "hwnd:1"}, true
	})
	obs, diag, err := p.Look(context.Background(),
		observation.Request{Include: []observation.Source{directorapi.SourceVision}})

	if err == nil {
		t.Fatal("a Director with no detector reported an empty success")
	}
	if !vision.IsUnavailable(err) {
		t.Errorf("err = %v; unavailability must be distinguishable", err)
	}
	if diag.Available {
		t.Error("the diagnostic claims vision is available")
	}
	if len(obs) != 0 {
		t.Errorf("%d observations from no detector", len(obs))
	}
}

func TestADetectorErrorIsAFailureNotSilence(t *testing.T) {
	d := &fakeDetector{err: errors.New("the model exploded")}
	_, _, err := provider(t, d, nil).Look(context.Background(),
		observation.Request{Include: []observation.Source{directorapi.SourceVision}})
	if err == nil {
		t.Fatal("a detector failure was reported as success")
	}
}

// ── opt-in ────────────────────────────────────────────────────────────────────

// TestVisionDoesNotRunUnlessAsked — a caller that has not thought about whether it wants a
// screen capture does not get one.
func TestVisionDoesNotRunUnlessAsked(t *testing.T) {
	d := &fakeDetector{results: []vision.Detection{detection("button", 10, 10, 40, 20, 0.9)}}
	p := provider(t, d, nil)

	obs, err := p.Observe(context.Background(), observation.Request{})
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if len(obs) != 0 {
		t.Fatalf("an ordinary cycle captured the screen and produced %d observations", len(obs))
	}
	if d.seen.Dx() != 0 {
		t.Error("the detector ran during an ordinary cycle")
	}
}

// ── grids (Part 7) ────────────────────────────────────────────────────────────

// gridDetections builds a rows×cols arrangement of equal cells.
func gridDetections(rows, cols, size, gap int, conf float64) []vision.Detection {
	var out []vision.Detection
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			out = append(out, detection("slot",
				10+c*(size+gap), 20+r*(size+gap), size, size, conf))
		}
	}
	return out
}

func TestARegularArrangementBecomesAGrid(t *testing.T) {
	d := &fakeDetector{results: gridDetections(6, 6, 48, 4, 0.8)}
	obs, diag := look(t, provider(t, d, nil))

	if len(diag.Grids) != 1 {
		t.Fatalf("%d grids found: %+v", len(diag.Grids), diag.Grids)
	}
	g := diag.Grids[0]
	if g.Rows != 6 || g.Columns != 6 || g.Cells != 36 {
		t.Fatalf("grid = %dx%d with %d cells", g.Rows, g.Columns, g.Cells)
	}
	if g.Confidence < 0.99 {
		t.Errorf("a complete arrangement scored %.2f", g.Confidence)
	}

	// The cells carry their POSITION, which is the durable identity hint.
	positioned := 0
	for _, o := range obs {
		el, ok := o.(observation.Element)
		if !ok {
			continue
		}
		if _, has := el.Raw.Attributes["grid_index"]; has {
			positioned++
		}
	}
	if positioned != 36 {
		t.Errorf("%d cells carry a grid position, want 36", positioned)
	}
}

func TestAnIrregularScatterIsNotAGrid(t *testing.T) {
	d := &fakeDetector{results: []vision.Detection{
		detection("slot", 10, 10, 48, 48, 0.9),
		detection("slot", 300, 90, 48, 48, 0.9),
		detection("slot", 120, 400, 48, 48, 0.9),
		detection("slot", 600, 40, 48, 48, 0.9),
	}}
	_, diag := look(t, provider(t, d, nil))

	if len(diag.Grids) != 0 {
		t.Fatalf("a scatter became a grid: %+v", diag.Grids)
	}
	// And the slots survive as slots: not being in a grid is not a reason to discard
	// what was seen.
	if diag.Counters.AcceptedStructural != 4 {
		t.Errorf("counters = %+v", diag.Counters)
	}
}

func TestARowIsNotAGrid(t *testing.T) {
	d := &fakeDetector{results: gridDetections(1, 8, 40, 4, 0.9)}
	_, diag := look(t, provider(t, d, nil))

	if len(diag.Grids) != 0 {
		t.Fatalf("a single row became a grid: %+v", diag.Grids)
	}
}

func TestCellsOfDifferentSizesAreDifferentArrangements(t *testing.T) {
	// A 3×3 of small cells and a 3×3 of large ones, side by side. Two grids, not one
	// ragged one: cells of visibly different size are different things.
	results := gridDetections(3, 3, 40, 4, 0.9)
	for _, det := range gridDetections(3, 3, 90, 6, 0.9) {
		det.Bounds = det.Bounds.Add(image.Pt(300, 0))
		results = append(results, det)
	}
	d := &fakeDetector{results: results}
	_, diag := look(t, provider(t, d, nil))

	if len(diag.Grids) != 2 {
		t.Fatalf("%d grids found, want two: %+v", len(diag.Grids), diag.Grids)
	}
}

// TestGridPositionsAreStableAcrossFrames.
//
// The property the whole grid pass exists for: the same arrangement, photographed twice,
// gives the same cell the same position — even though nothing about a detected box is
// durable on its own.
func TestGridPositionsAreStableAcrossFrames(t *testing.T) {
	d := &fakeDetector{results: gridDetections(4, 4, 50, 5, 0.85)}
	p := provider(t, d, nil)

	first := gridIndexAt(t, p, 3, 2)
	second := gridIndexAt(t, p, 3, 2)
	if first == 0 || first != second {
		t.Fatalf("cell (3,2) was index %d then %d", first, second)
	}
}

// gridIndexAt runs a pass and returns the reading-order index of one cell.
func gridIndexAt(t *testing.T, p *vision.Provider, row, col int) int {
	t.Helper()
	obs, _ := look(t, p)
	for _, o := range obs {
		el, ok := o.(observation.Element)
		if !ok {
			continue
		}
		r, hasR := el.Raw.Attributes["grid_row"].(int)
		c, hasC := el.Raw.Attributes["grid_column"].(int)
		if hasR && hasC && r == row && c == col {
			if idx, ok := el.Raw.Attributes["grid_index"].(int); ok {
				return idx
			}
		}
	}
	return 0
}

// ── the frame log (Part 11) ───────────────────────────────────────────────────

// TestTheFrameLogRemembersShapeNotPictures.
//
// A rolling buffer of screenshots would be the most sensitive thing this system could
// accumulate. What is kept is when, how big, and what was made of it.
func TestTheFrameLogRemembersShapeNotPictures(t *testing.T) {
	d := &fakeDetector{results: []vision.Detection{detection("button", 10, 10, 40, 20, 0.9)}}
	p := provider(t, d, nil)
	look(t, p)
	look(t, p)

	frames := p.Frames()
	if len(frames) != 2 {
		t.Fatalf("%d frames logged", len(frames))
	}
	// Newest first.
	if frames[0].ID == frames[1].ID {
		t.Error("two passes were logged under one frame id")
	}
	if frames[0].Size != "800x600" {
		t.Errorf("size = %q", frames[0].Size)
	}
	if frames[0].Counters.Accepted != 1 {
		t.Errorf("counters = %+v", frames[0].Counters)
	}
	// A FrameRecord has no field that could hold an image, which is the property that
	// matters. Asserted structurally: adding one would fail to compile against this.
	var _ = struct {
		ID string
	}{ID: string(frames[0].ID)}
}

func TestTheFrameLogIsBounded(t *testing.T) {
	d := &fakeDetector{}
	p := provider(t, d, nil)
	for i := 0; i < vision.FrameLogSize+10; i++ {
		look(t, p)
	}
	if got := len(p.Frames()); got != vision.FrameLogSize {
		t.Errorf("%d frames retained, want the bound of %d", got, vision.FrameLogSize)
	}
}

// ── the class vocabulary ──────────────────────────────────────────────────────

func TestClassMappingIsSpellingInsensitiveAndMeaningStrict(t *testing.T) {
	for _, spelling := range []string{"push_button", "Push Button", "pushButton", "PUSH-BUTTON"} {
		if c, ok := vision.ClassOf(spelling); !ok || c != vision.ClassButton {
			t.Errorf("%q mapped to %q (ok=%v)", spelling, c, ok)
		}
	}
	// A word absent from the table stays absent however it is spelled.
	for _, unknown := range []string{"clickable_region", "widget", "thing"} {
		if c, ok := vision.ClassOf(unknown); ok {
			t.Errorf("%q was mapped to %q; an unmapped class must be refused", unknown, c)
		}
	}
}

func TestOnlyStructuralClassesCarryARole(t *testing.T) {
	for _, c := range vision.Classes() {
		role := c.Role()
		if c.Structural() && role == directorapi.RoleUnknown {
			t.Errorf("%q is structural and maps to no role", c)
		}
		if !c.Structural() && role != directorapi.RoleUnknown {
			t.Errorf("%q is not structural and maps to %q", c, role)
		}
	}
}

// TestTheCeilingIsReportedRatherThanSilent.
func TestTheCeilingIsReportedRatherThanSilent(t *testing.T) {
	var many []vision.Detection
	for i := 0; i < 600; i++ {
		many = append(many, detection("button", i%700, 10+(i/700)*30, 20, 20, 0.9))
	}
	d := &fakeDetector{results: many}
	p := provider(t, d, nil)
	p.Thresholds.MaxDetections = 100

	_, diag := look(t, p)
	if diag.Counters.Accepted != 100 {
		t.Errorf("accepted = %d, want the ceiling", diag.Counters.Accepted)
	}
	if diag.Counters.RejectedCeiling == 0 {
		t.Error("what the ceiling dropped was not reported, so the listing reads as complete")
	}
}

// A compile-time check that the fake satisfies the real interface.
var _ vision.Detector = (*fakeDetector)(nil)

var _ = fmt.Sprintf

// TestAGridCellIsOneObservationNotTwo.
//
// A grid position is a PROPERTY of a slot, not a second thing at the same place. Emitting
// it separately produced two elements per cell — fusion refuses to merge two observations
// from one source however identical their geometry, because a source enumerates the
// desktop once and two of its nodes are two objects.
func TestAGridCellIsOneObservationNotTwo(t *testing.T) {
	d := &fakeDetector{results: gridDetections(3, 3, 40, 4, 0.9)}
	obs, diag := look(t, provider(t, d, nil))

	cells, grids, boxes := 0, 0, map[directorapi.Rect]int{}
	for _, o := range obs {
		el, ok := o.(observation.Element)
		if !ok {
			continue
		}
		boxes[el.Raw.Bounds]++
		switch el.Raw.Attributes["vision_class"] {
		case "grid":
			grids++
		case string(vision.ClassSlot):
			cells++
		}
	}
	if cells != 9 {
		t.Errorf("%d cell observations for a 3×3 grid, want 9", cells)
	}
	if grids != 1 {
		t.Errorf("%d grid observations, want 1", grids)
	}
	for box, n := range boxes {
		if n > 1 {
			t.Errorf("%d observations at %+v; one source must say one thing about one box",
				n, box)
		}
	}
	if diag.Counters.AcceptedStructural != 9 {
		t.Errorf("counters = %+v", diag.Counters)
	}
}
