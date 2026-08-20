package vision_test

import (
	"context"
	"errors"
	"image"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Reading the words inside a detected control.
//
// The fixture is a live Rocket League pause menu — four buttons the detector found at
// 0.59–0.64 with the default threshold, and the labels an OCR engine read inside each one.
// See fixtures/rocketleague/pause.json.
//
// The measurement that produced this design is in the package doc: whole-frame OCR read one
// string of the twelve on that screen, tiling read 151 mostly-hallucinated ones, and reading
// inside the detected boxes read four of four exactly.

// lookText runs a pass that is WILLING TO PAY for text.
//
// Scoped reading is opt-in per cycle, and `SourceOCR` in the request is the permission — the
// same flag the observation session sets on the label passes its budget allows. A pass asking
// only for vision gets boxes and no readings, which is what TestAVisionPassWithoutTheTextBudget
// holds.
func lookText(t *testing.T, p *vision.Provider) ([]observation.Observation, vision.Diagnostics) {
	t.Helper()
	obs, diag, err := p.Look(context.Background(), observation.Request{
		Include: []observation.Source{directorapi.SourceVision, directorapi.SourceOCR},
	})
	if err != nil {
		t.Fatalf("look: %v", err)
	}
	return obs, diag
}

// pauseMenuDetections are the four menu buttons at the geometry the real model reported.
//
// The CLASS is not the shipping model's. It reported all four as `icon`, which is the finding
// [[docs/subsystems/Vision]] records as the blocker: a detector whose whole vocabulary is
// unsayable can never name anything, however well an engine reads inside its boxes. These
// tests are about the reading mechanism — cropping, budget, confidence, shape — so they use
// the class a detector with a real vocabulary emits. That the shipping one does not is held
// separately, by TestAnUnsayableRoleIsNeverRead.
var pauseMenuDetections = []vision.Detection{
	{Class: "button", Confidence: 0.61, Bounds: image.Rect(791, 468, 1131, 515)},
	{Class: "button", Confidence: 0.64, Bounds: image.Rect(790, 512, 1132, 559)},
	{Class: "button", Confidence: 0.64, Bounds: image.Rect(786, 558, 1132, 604)},
	{Class: "button", Confidence: 0.59, Bounds: image.Rect(782, 603, 1132, 649)},
}

// scriptedReader answers by which box it was handed, keyed by the crop's size.
//
// A reader is given a CROP, so it cannot see where the crop came from — which is the point,
// and also why this keys on dimensions. It records what it was asked to read so a test can
// assert the provider cropped rather than handing over the whole frame.
type scriptedReader struct {
	byArea map[int][]vision.TextSpan
	err    error
	sizes  []image.Point
	calls  int
}

func (s *scriptedReader) ReadLabel(_ context.Context, img image.Image) ([]vision.TextSpan, error) {
	s.calls++
	if img != nil {
		s.sizes = append(s.sizes, image.Pt(img.Bounds().Dx(), img.Bounds().Dy()))
	}
	if s.err != nil {
		return nil, s.err
	}
	if img == nil {
		return nil, nil
	}
	return s.byArea[img.Bounds().Dx()*img.Bounds().Dy()], nil
}

func span(text string, confidence float64, r image.Rectangle) vision.TextSpan {
	return vision.TextSpan{Text: text, Confidence: confidence, Bounds: r}
}

// pauseReader answers every crop with one of the real labels, in order of arrival.
type pauseReader struct {
	labels []string
	at     int
	sizes  []image.Point
}

func (p *pauseReader) ReadLabel(_ context.Context, img image.Image) ([]vision.TextSpan, error) {
	if img != nil {
		p.sizes = append(p.sizes, image.Pt(img.Bounds().Dx(), img.Bounds().Dy()))
	}
	if p.at >= len(p.labels) {
		return nil, nil
	}
	text := p.labels[p.at]
	p.at++
	return []vision.TextSpan{span(text, 0.9, image.Rect(0, 0, 100, 20))}, nil
}

func labelledProvider(t *testing.T, r vision.LabelReader, d *fakeDetector) *vision.Provider {
	t.Helper()
	p := provider(t, d, &fakeCapture{width: 1920, height: 1080})
	p.Reader = r
	return p
}

func TestDetectedControlsAreNamedByReadingInsideThem(t *testing.T) {
	reader := &pauseReader{labels: []string{
		"RESUME GAME", "CHANGE MODE/MATCH", "SETTINGS", "EXIT TO MAIN MENU",
	}}
	obs, diag := lookText(t, labelledProvider(t,
		reader, &fakeDetector{results: pauseMenuDetections}))

	got := map[string]bool{}
	for _, o := range obs {
		el, ok := o.(observation.Element)
		if !ok || el.Raw.Attributes["vision_class"] == "grid" {
			continue
		}
		if el.Raw.Label != "" {
			got[el.Raw.Label] = true
		}
	}
	for _, want := range reader.labels {
		if !got[want] {
			t.Errorf("no control was named %q; got %v", want, got)
		}
	}
	if diag.Counters.LabelsRead != 4 {
		t.Errorf("LabelsRead = %d, want 4", diag.Counters.LabelsRead)
	}
}

func TestTheReaderSeesACropNotTheWholeFrame(t *testing.T) {
	// The entire finding this feature rests on: an engine pointed at the whole frame
	// reads almost nothing, and pointed at arena texture it hallucinates. If the provider
	// ever hands over the full picture, the labels become fiction.
	reader := &pauseReader{}
	_, _ = lookText(t, labelledProvider(t, reader, &fakeDetector{results: pauseMenuDetections}))

	if len(reader.sizes) != len(pauseMenuDetections) {
		t.Fatalf("the reader was called %d times for %d detections",
			len(reader.sizes), len(pauseMenuDetections))
	}
	for _, s := range reader.sizes {
		if s.X >= 1920 || s.Y >= 1080 {
			t.Fatalf("the reader was handed a %dx%d image — that is the frame, not a control", s.X, s.Y)
		}
	}
	// Largest box first: 350x46, at the default 3x upscale. The order matters because
	// MaxLabels drops the tail, and the biggest controls are the likeliest to carry a
	// name worth having.
	if first := reader.sizes[0]; first.X != 350*3 || first.Y != 46*3 {
		t.Errorf("first crop %v, want the largest button enlarged 3x (1050x138)", first)
	}
}

func TestReadingIsBoundedSoOneCycleCannotRunAway(t *testing.T) {
	// Each reading is a serial round trip to an OCR engine. Measured live, 39 boxes cost
	// 9.0 seconds — which is why there is a ceiling, and why what it drops is counted
	// rather than silently omitted.
	many := make([]vision.Detection, 0, 40)
	for i := 0; i < 40; i++ {
		y := 10 + i*20
		many = append(many, vision.Detection{
			Class: "button", Confidence: 0.7, Bounds: image.Rect(10, y, 210, y+18),
		})
	}
	reader := &pauseReader{}
	p := labelledProvider(t, reader, &fakeDetector{results: many})
	_, diag := lookText(t, p)

	if reader.at > p.Labels.MaxLabels {
		t.Fatalf("the reader was called %d times, past the ceiling of %d",
			reader.at, p.Labels.MaxLabels)
	}
	if diag.Counters.LabelsSkipped == 0 {
		t.Fatal("boxes were dropped but LabelsSkipped stayed 0; a silent cap reads as " +
			"\"nothing to find\" when it means \"we stopped looking\"")
	}
}

func TestBoxesTooSmallForAWordAreNotRead(t *testing.T) {
	// A 12x12 icon costs the same round trip as a button and cannot hold a name.
	reader := &pauseReader{labels: []string{"SETTINGS"}}
	_, diag := lookText(t, labelledProvider(t, reader, &fakeDetector{results: []vision.Detection{
		{Class: "button", Confidence: 0.7, Bounds: image.Rect(10, 10, 22, 22)},
	}}))

	if reader.at != 0 {
		t.Fatalf("a 12x12 box was sent to the reader")
	}
	if diag.Counters.LabelsSkipped != 1 {
		t.Errorf("LabelsSkipped = %d, want 1", diag.Counters.LabelsSkipped)
	}
}

func TestAnUnreadableControlStaysUnnamedRatherThanGuessed(t *testing.T) {
	// The live frame's stylised name plate read "Qovisivre ys". A control named that is
	// worse than an unnamed one, because it is addressable.
	reader := &scriptedReader{byArea: map[int][]vision.TextSpan{}}
	obs, diag := lookText(t, labelledProvider(t, reader, &fakeDetector{
		results: []vision.Detection{{Class: "button", Confidence: 0.7, Bounds: image.Rect(10, 10, 200, 60)}},
	}))

	for _, o := range obs {
		el, ok := o.(observation.Element)
		if !ok {
			continue
		}
		if el.Raw.Label != "" {
			t.Fatalf("an unreadable control was named %q", el.Raw.Label)
		}
		if _, named := el.Raw.Attributes["label"]; named {
			t.Fatal("an unreadable control carries a label attribute")
		}
	}
	if diag.Counters.LabelsRead != 0 {
		t.Errorf("LabelsRead = %d, want 0", diag.Counters.LabelsRead)
	}
	if diag.Counters.LabelsUnreadable != 1 {
		t.Errorf("LabelsUnreadable = %d, want 1", diag.Counters.LabelsUnreadable)
	}
}

// oneDetection is a single structural box for the single-label tests.
var oneDetection = []vision.Detection{
	{Class: "button", Confidence: 0.7, Bounds: image.Rect(10, 10, 200, 60)},
}

// nameOf runs one pass and returns whatever the control ended up called.
func nameOf(t *testing.T, spans []vision.TextSpan) string {
	t.Helper()
	reader := &scriptedReader{byArea: map[int][]vision.TextSpan{(190 * 3) * (50 * 3): spans}}
	obs, _ := lookText(t, labelledProvider(t, reader, &fakeDetector{results: oneDetection}))
	for _, o := range obs {
		if el, ok := o.(observation.Element); ok && el.Raw.Label != "" {
			return el.Raw.Label
		}
	}
	return ""
}

func TestSymbolSoupIsRefusedWhateverTheEngineClaims(t *testing.T) {
	// Verbatim from the live tiling run over arena texture. Confidence is deliberately
	// HIGH here: shape alone has to be enough, because an engine reading texture is not
	// always modest about it.
	for _, garbage := range []string{"Sty 4;", "{= =", "»)  (ee i", "~~ A", "|", "\\ Sea"} {
		if got := nameOf(t, []vision.TextSpan{span(garbage, 0.95, image.Rect(0, 0, 90, 20))}); got != "" {
			t.Errorf("symbol soup %q was accepted as the name %q", garbage, got)
		}
	}
}

func TestLetterShapedNonsenseIsRefusedByConfidenceNotShape(t *testing.T) {
	// "Qovisivre ys" is what the live frame's stylised name plate read as. It is all
	// letters, so no shape rule can reject it — and pretending one could, by bolting on a
	// dictionary, would refuse every application whose vocabulary nobody anticipated.
	//
	// Confidence is what rejects it. Both halves of that are worth pinning: unsure is
	// refused, and sure is NOT, because the filter must not quietly become a shape rule
	// that happens to work on this one string.
	const nonsense = "Qovisivre ys"

	// "j Qe" is the same category and was misfiled as symbol soup while these tests were
	// being written: it is three letters and a space, and passes every shape rule there
	// is. Kept here as the second witness that shape is not the mechanism.
	for _, unsure := range []string{nonsense, "j Qe", "itirne", "Feasts"} {
		if got := nameOf(t, []vision.TextSpan{span(unsure, 0.30, image.Rect(0, 0, 90, 20))}); got != "" {
			t.Errorf("an unsure reading of %q was accepted as the name %q", unsure, got)
		}
	}
	if got := nameOf(t, []vision.TextSpan{span(nonsense, 0.95, image.Rect(0, 0, 90, 20))}); got != nonsense {
		t.Errorf("a confident letter-shaped reading was refused (%q); "+
			"shape cannot tell this from a product name, and only confidence may reject it", got)
	}
}

func TestBorderMarksArriveAsTheirOwnSpansAndAreDropped(t *testing.T) {
	// Running the real engine over the real button produced "| RESUME GAME ," — the panel
	// edge and a highlight, read as characters. That is how it looks in tesseract's
	// single-LINE mode; the Director asks for words, and a border mark is a word of its
	// own that the engine is not confident about.
	//
	// So the mechanism is span confidence, not string surgery. Trimming the ends was
	// written and then reverted: it rescues "»)  (ee i" as "ee i", which is exactly the
	// garbage the shape filter exists to stop.
	got := nameOf(t, []vision.TextSpan{
		span("|", 0.11, image.Rect(0, 0, 6, 20)),
		span("RESUME", 0.93, image.Rect(10, 0, 60, 20)),
		span("GAME", 0.95, image.Rect(65, 0, 105, 20)),
		span(",", 0.09, image.Rect(110, 12, 116, 20)),
	})
	if got != "RESUME GAME" {
		t.Errorf("got %q, want %q — the border marks are low-confidence spans", got, "RESUME GAME")
	}
}

func TestRealLabelsSurviveTheFilter(t *testing.T) {
	// The filter must not be so strict that it throws away the labels it exists to keep.
	for _, real := range []string{
		"RESUME GAME", "CHANGE MODE/MATCH", "SETTINGS", "EXIT TO MAIN MENU",
		"Save & Close", "OK", "Don't Save", "50%", "Item 3",
	} {
		reader := &pauseReader{labels: []string{real}}
		obs, _ := lookText(t, labelledProvider(t, reader, &fakeDetector{
			results: []vision.Detection{{Class: "button", Confidence: 0.7, Bounds: image.Rect(10, 10, 200, 60)}},
		}))
		named := ""
		for _, o := range obs {
			if el, ok := o.(observation.Element); ok && el.Raw.Label != "" {
				named = el.Raw.Label
			}
		}
		if named != real {
			t.Errorf("the real label %q came through as %q", real, named)
		}
	}
}

func TestALabelIsOneObservationNotTwo(t *testing.T) {
	// The same lesson grid positions taught: a name is a property OF a control. Emitted
	// separately it would be a second same-source observation at identical geometry, and
	// fusion never merges those — two elements per button, each holding half.
	reader := &pauseReader{labels: []string{
		"RESUME GAME", "CHANGE MODE/MATCH", "SETTINGS", "EXIT TO MAIN MENU",
	}}
	obs, _ := lookText(t, labelledProvider(t, reader, &fakeDetector{results: pauseMenuDetections}))

	elements := 0
	for _, o := range obs {
		if el, ok := o.(observation.Element); ok && el.Raw.Attributes["vision_class"] != "grid" {
			elements++
		}
	}
	if elements != len(pauseMenuDetections) {
		t.Fatalf("%d elements for %d detections; a label became a second observation",
			elements, len(pauseMenuDetections))
	}
	for _, o := range obs {
		if _, ok := o.(observation.Text); ok {
			t.Fatal("the reader produced a Text observation; a label belongs to its control")
		}
	}
}

func TestAReaderThatFailsLeavesThePassIntact(t *testing.T) {
	// A reader is optional. Losing it costs names, never evidence.
	reader := &scriptedReader{err: errors.New("the OCR plugin is not installed")}
	obs, diag := lookText(t, labelledProvider(t, reader, &fakeDetector{results: pauseMenuDetections}))

	if diag.Error != "" {
		t.Errorf("a failing reader failed the pass: %q", diag.Error)
	}
	if diag.Counters.Accepted != len(pauseMenuDetections) {
		t.Errorf("accepted %d, want %d — the boxes are still evidence",
			diag.Counters.Accepted, len(pauseMenuDetections))
	}
	if len(obs) == 0 {
		t.Fatal("a failing reader produced no observations at all")
	}
	if diag.Counters.LabelsUnreadable != len(pauseMenuDetections) {
		t.Errorf("LabelsUnreadable = %d, want %d",
			diag.Counters.LabelsUnreadable, len(pauseMenuDetections))
	}
}

func TestNoReaderIsTheOrdinaryCase(t *testing.T) {
	// A build with a detector and no OCR must behave exactly as it did before labels
	// existed — no calls, no names, no counters.
	obs, diag := look(t, provider(t, &fakeDetector{results: pauseMenuDetections},
		&fakeCapture{width: 1920, height: 1080}))

	if diag.Counters.LabelsRead != 0 || diag.Counters.LabelsUnreadable != 0 {
		t.Errorf("counters moved with no reader: %+v", diag.Counters)
	}
	for _, o := range obs {
		if el, ok := o.(observation.Element); ok && el.Raw.Label != "" {
			t.Fatalf("a control was named %q with no reader wired", el.Raw.Label)
		}
	}
}

func TestTheWeakestSpanCarriesTheLabelsConfidence(t *testing.T) {
	// A label is right only if every part of it was read right. Averaging would let one
	// confident word carry two illegible ones.
	reader := &scriptedReader{byArea: map[int][]vision.TextSpan{
		(190 * 3) * (50 * 3): {
			span("CHANGE", 0.95, image.Rect(0, 0, 60, 20)),
			span("MODE", 0.52, image.Rect(65, 0, 110, 20)),
		},
	}}
	obs, _ := lookText(t, labelledProvider(t, reader, &fakeDetector{
		results: []vision.Detection{{Class: "button", Confidence: 0.7, Bounds: image.Rect(10, 10, 200, 60)}},
	}))

	for _, o := range obs {
		el, ok := o.(observation.Element)
		if !ok || el.Raw.Label == "" {
			continue
		}
		if el.Raw.Label != "CHANGE MODE" {
			t.Fatalf("label = %q, want the spans joined in reading order", el.Raw.Label)
		}
		got, _ := el.Raw.Attributes["label_confidence"].(float64)
		if got != 0.52 {
			t.Fatalf("label_confidence = %v, want the weakest span's 0.52", got)
		}
		return
	}
	t.Fatal("no label was produced")
}

func TestALabelSaysWhereItCameFrom(t *testing.T) {
	// Provenance: a name read out of pixels must never be mistaken for one an
	// accessibility tree reported.
	reader := &pauseReader{labels: []string{"SETTINGS"}}
	obs, _ := lookText(t, labelledProvider(t, reader, &fakeDetector{
		results: []vision.Detection{{Class: "button", Confidence: 0.7, Bounds: image.Rect(10, 10, 200, 60)}},
	}))
	for _, o := range obs {
		el, ok := o.(observation.Element)
		if !ok || el.Raw.Label == "" {
			continue
		}
		if el.Raw.Attributes["label_source"] != "reader" {
			t.Fatalf("label_source = %v, want \"reader\"", el.Raw.Attributes["label_source"])
		}
		return
	}
	t.Fatal("no label was produced")
}
