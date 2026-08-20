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
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Vision-derived structure acquiring a safe, generic, nameable semantic path — on an
// interface where accessibility says nothing at all.
//
// # What these tests enter through
//
// The production composition, from the picture onward:
//
//	fake capture → registered detector → newShadowProvider → providers.Collector
//	  → liveSampler.Sample → ShadowSample.Semantic → ShadowTotals → Hypotheses
//	    → SignatureOf
//
// Nothing here builds a provider of its own, and nothing calls a helper directly to see what
// it returns. That is deliberate and it is this repository's most expensive recurring lesson:
// three separate mechanisms have been written, unit-tested and left uncalled, and the tests
// that would have caught it were the ones that entered through the constructor production uses.
// See [[Wiring-Tests]].
//
// # The invariant every one of these is about
//
//	Text may enrich structure. Text may not create structure.
//
// Vision finds a thing, the thing's role decides whether it may be named, only then is text
// read, and only closed-vocabulary concepts survive. Reversing any pair of those steps produces
// a system that hallucinates buttons out of words, and one of these tests fails for each.

// ── the fixture ───────────────────────────────────────────────────────────────

// accessibilityPoorFixture is one screen of an application that exposes no accessibility tree:
// four rendered controls, one heading, one icon and one panel, as a detector reports them.
//
// The geometry is a plain vertical menu. The labels are ordinary interface words — the point of
// the milestone is that they are GENERIC, so nothing here names a game, a mode or a feature that
// only one application has.
func accessibilityPoorFixture() (dets []vision.Detection, text map[image.Rectangle]string) {
	text = map[image.Rectangle]string{}
	add := func(class string, r image.Rectangle, words string) {
		dets = append(dets, vision.Detection{Class: class, Bounds: r, Confidence: 0.7})
		if words != "" {
			text[r] = words
		}
	}
	// Deliberately different widths. The reader is keyed on the SIZE of the crop it is
	// handed, because that is all a scoped reader can see — so identical boxes would be
	// indistinguishable to the fixture, and three of the four labels would silently vanish.
	add("text", image.Rect(700, 300, 1104, 350), "CONTROLLER SETTINGS")
	add("button", image.Rect(700, 400, 1100, 450), "SETTINGS")
	add("button", image.Rect(700, 470, 1096, 520), "CONTROLS")
	add("button", image.Rect(700, 540, 1092, 590), "AUDIO")
	add("button", image.Rect(700, 610, 1088, 660), "BACK")
	// Structural and unsayable. An icon's contents on a desktop are a document title or a
	// contact, so it is never read however plainly it is written.
	add("icon", image.Rect(100, 900, 180, 980), "SETTINGS")
	add("panel", image.Rect(650, 280, 1150, 700), "CONTROLLER SETTINGS")
	return dets, text
}

// boxReader answers a crop with whatever the fixture wrote at the box of that SIZE.
//
// Keyed on size rather than position because a reader is handed a crop and cannot see where it
// came from — which is the property that makes scoped reading trustworthy, and is asserted
// directly by TestTheReaderIsNeverHandedTheWholeFrame in the vision package.
type boxReader struct {
	byArea map[int]string
	calls  int
	sizes  []image.Point
	// dropWord goes unread on one pass in dropEvery, so an intermittently unreadable
	// control can be measured apart from a permanently unreadable one. They are different
	// facts about an engine and only one of them threatens identity.
	dropWord  string
	dropEvery int
	seen      map[string]int
}

func newBoxReader(text map[image.Rectangle]string, upscale int) *boxReader {
	r := &boxReader{byArea: map[int]string{}}
	for box, words := range text {
		r.byArea[box.Dx()*upscale*box.Dy()*upscale] = words
	}
	return r
}

func (r *boxReader) ReadLabel(_ context.Context, img image.Image) ([]vision.TextSpan, error) {
	r.calls++
	b := img.Bounds()
	r.sizes = append(r.sizes, image.Pt(b.Dx(), b.Dy()))
	words, ok := r.byArea[b.Dx()*b.Dy()]
	if !ok {
		return nil, nil
	}
	if r.dropEvery > 0 && words == r.dropWord {
		if r.seen == nil {
			r.seen = map[string]int{}
		}
		r.seen[words]++
		if r.seen[words]%r.dropEvery == 0 {
			return nil, nil
		}
	}
	return []vision.TextSpan{{Text: words, Confidence: 0.92}}, nil
}

// fixtureDetector reports the fixture's detections, whatever it is shown.
type fixtureDetector struct{ results []vision.Detection }

func (d fixtureDetector) Detect(context.Context, vision.Input) ([]vision.Detection, error) {
	return d.results, nil
}
func (fixtureDetector) Model() string { return "fixture" }

// fixtureCapture returns a blank frame that PROVES which window generation it came from.
//
// The provenance is the whole reason this is not a one-line stub. A frame that cannot say what
// it is a picture of is refused before any of this milestone's behaviour runs, and a test that
// silently hit that path would pass for the wrong reason.
type fixtureCapture struct {
	generation uint64
	moved      bool
}

func (c fixtureCapture) CaptureWindow(_ context.Context, w directorapi.Window) (capture.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, w.Bounds.Width, w.Bounds.Height))
	for y := 0; y < w.Bounds.Height; y += 97 {
		img.Set(0, y, color.White)
	}
	bounds := w.Bounds
	if c.moved {
		bounds.X += 40
	}
	return capture.Image{
		Image: img, Bounds: w.Bounds, Scale: 1, CapturedAt: time.Now(),
		WindowID: w.ID, Application: w.Application, WindowBoundsAtCapture: bounds,
		Target: &directorapi.TargetProvenance{
			Application: w.Application, ProcessID: 7, WindowGeneration: c.generation,
		},
	}, nil
}

// visionShadowRuntime is a Director whose ONLY evidence source is the shadow detector.
//
// stubEngine fuses nothing, so the authoritative world is empty and sample.Entities is empty:
// accessibility contributes no element, no role and no label. That is the load-bearing property
// of this fixture. If accessibility supplied anything here, a term could have come from it and
// the test would prove nothing about the vision path.
func visionShadowRuntime(t *testing.T, dets []vision.Detection,
	reader vision.LabelReader, cap capture.WindowCapture) *Runtime {

	t.Helper()
	// shadowDetectorName reads this; without it the sampler builds no shadow record at all.
	t.Setenv("MARCO_SHADOW_VISION", "screenparser")

	rt := &Runtime{engine: stubEngine{}}
	prov := newShadowProvider(fixtureDetector{results: dets}, cap, rt.activeWindow, reader,
		time.Nanosecond)
	rt.collector = providers.NewCollector(prov)
	rt.shadowVision = prov
	rt.pinnedWindow = &windowRef1
	return rt
}

// sampleWithVision runs one production label pass.
func sampleWithVision(t *testing.T, rt *Runtime) observe.Sample {
	t.Helper()
	live := rt.newObservationSampler(sessionClock).(*liveSampler)
	req := sampleRequest()
	req.ReadLabels = true
	sample, err := live.Sample(context.Background(), req)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	return sample
}

func termSet(sem observe.SemanticEvidence) map[observe.InterfaceTerm]bool {
	out := map[observe.InterfaceTerm]bool{}
	for _, term := range sem.Terms {
		out[term] = true
	}
	return out
}

// ── PART 8: accessibility must not be required ────────────────────────────────

// THE positive test of this milestone.
//
// Accessibility supplies nothing. The detector supplies structure. Scoped OCR reads inside the
// structure it supplied. Closed-vocabulary terms come out, and reach the durable signature that
// cross-session identity is made of.
//
// Mutation 1 (delete inner.Reader in newShadowProvider), mutation 2 (delete Class.Nameable from
// the reading gate), mutation 7 (drop terms before SignatureOf) and mutation 8 (delete the
// association call in shadowSampleFor) each fail here.
func TestVisionStructureBecomesSemanticWithoutAccessibility(t *testing.T) {
	dets, text := accessibilityPoorFixture()
	reader := newBoxReader(text, vision.DefaultLabelThresholds().Upscale)
	rt := visionShadowRuntime(t, dets, reader, fixtureCapture{generation: 1})

	sample := sampleWithVision(t, rt)

	if len(sample.Entities) != 0 {
		t.Fatalf("the fixture produced %d authoritative entities; accessibility is "+
			"participating and this test would prove nothing", len(sample.Entities))
	}
	if sample.Shadow == nil {
		t.Fatal("no shadow record, so semantic evidence has nowhere to ride")
	}
	sem := sample.Shadow.Semantic
	if !sem.Observed {
		t.Fatal("the detector's own boxes were read and the sample reports that nothing was " +
			"read. Terms stay UNKNOWN forever and the state's term ratio has no denominator")
	}
	got := termSet(sem)
	for _, want := range []observe.InterfaceTerm{
		observe.TermSettings, observe.TermControls, observe.TermAudio, observe.TermBack,
	} {
		if !got[want] {
			t.Errorf("term %q never appeared; terms were %v. Vision-derived structure has no "+
				"semantic path and an accessibility-poor application can still never be "+
				"recognised", want, sem.Terms)
		}
	}

	// And the rest of the chain, through the real accumulator, to the value semantic memory
	// compares. A term that reaches the sample and dies before the signature is a term that
	// changes nothing.
	var totals observe.ShadowTotals
	for i := 0; i < 8; i++ {
		s := *sample.Shadow
		s.Regions = menuRegions()
		s.Detections = len(s.Regions)
		s.Roles = map[string]int{"button": 4, "icon": 1}
		s.Nameable = 4
		totals.Add(s)
		totals.Add(gameplayShadow())
	}
	var carried bool
	for _, h := range observe.Hypotheses(totals, observe.DefaultHypothesisThresholds()) {
		sig := observe.SignatureOf(h)
		if sig.TermsKnown && len(sig.Terms) > 0 {
			carried = true
		}
	}
	if !carried {
		t.Fatal("no term read from the detector's own structure reached a durable signature. " +
			"Cross-session identity on an accessibility-poor surface still has no discriminator")
	}
}

// ── PART 9: OCR cannot synthesize structure ───────────────────────────────────

// Text without structure creates nothing.
//
// Mutation 3 (let a read text region become an element) fails here. The screen-level concept
// may survive — a heading genuinely says something about the screen — but no control appears,
// and nothing acquires a role.
func TestReadTextNeverBecomesAControl(t *testing.T) {
	dets := []vision.Detection{
		{Class: "text", Bounds: image.Rect(700, 300, 1100, 350), Confidence: 0.8},
	}
	reader := newBoxReader(map[image.Rectangle]string{
		image.Rect(700, 300, 1100, 350): "SETTINGS",
	}, vision.DefaultLabelThresholds().Upscale)
	rt := visionShadowRuntime(t, dets, reader, fixtureCapture{generation: 1})

	sample := sampleWithVision(t, rt)
	if sample.Shadow == nil {
		t.Fatal("no shadow record")
	}
	// The concept is admissible evidence about the screen.
	if !termSet(sample.Shadow.Semantic)[observe.TermSettings] {
		t.Errorf("a read heading produced no screen-level concept; terms %v",
			sample.Shadow.Semantic.Terms)
	}
	// A control is not.
	if n := sample.Shadow.Detections; n != 0 {
		t.Fatalf("reading the word SETTINGS produced %d structural detections. OCR has been "+
			"allowed to synthesize structure, which is the one thing this layer exists to "+
			"refuse", n)
	}
	if sample.Shadow.Nameable != 0 || len(sample.Shadow.Roles) != 0 {
		t.Fatalf("a text region acquired a role: nameable=%d roles=%v",
			sample.Shadow.Nameable, sample.Shadow.Roles)
	}
}

// ── PART 10: unsayable structure stays unsayable ──────────────────────────────

// An icon that plainly says SETTINGS is still an icon.
//
// Mutation 4 (make every vision structural role nameable) fails here — and it fails on BOTH
// counts, because widening the allowlist both spends the reader on an icon and lets the
// classifier release what it found.
func TestAnUnsayableRoleIsNeitherReadNorNamed(t *testing.T) {
	dets := []vision.Detection{
		{Class: "icon", Bounds: image.Rect(100, 900, 300, 980), Confidence: 0.8},
		{Class: "panel", Bounds: image.Rect(650, 280, 1150, 700), Confidence: 0.8},
		{Class: "bar", Bounds: image.Rect(100, 100, 500, 140), Confidence: 0.8},
	}
	text := map[image.Rectangle]string{}
	for _, d := range dets {
		text[d.Bounds] = "SETTINGS"
	}
	reader := newBoxReader(text, vision.DefaultLabelThresholds().Upscale)
	rt := visionShadowRuntime(t, dets, reader, fixtureCapture{generation: 1})

	sample := sampleWithVision(t, rt)

	if reader.calls != 0 {
		t.Errorf("the reader was called %d times for three unsayable regions. Nameability "+
			"decides what is READ, not only what is kept — reading them costs a round trip "+
			"each and puts text nobody may use into this process", reader.calls)
	}
	if sample.Shadow == nil {
		t.Fatal("no shadow record")
	}
	if sample.Shadow.Detections != 3 {
		t.Errorf("the three regions are structural and should still be reported: got %d",
			sample.Shadow.Detections)
	}
	if sample.Shadow.Nameable != 0 {
		t.Errorf("an unsayable role was counted nameable")
	}
	if len(sample.Shadow.Semantic.Terms) != 0 {
		t.Fatalf("an icon, a panel and a progress bar produced the terms %v. The word decided "+
			"nameability instead of the structure, which is exactly backwards",
			sample.Shadow.Semantic.Terms)
	}
	if sample.Shadow.Semantic.Observed {
		t.Error("nothing was read and the sample claims perception looked")
	}
}

// ── PART 12: ambiguous association is refused, not resolved ───────────────────

// Two overlapping controls compete for the same words, so neither gets them.
//
// The failure this prevents is a confidently MISNAMED control: a request that says "click
// Settings" resolving to the button next to Settings. Nothing about iteration order or centre
// distance is evidence about which one the word belongs to.
func TestOverlappingControlsDoNotTakeEachOthersWords(t *testing.T) {
	a := image.Rect(700, 400, 1100, 500)
	b := image.Rect(700, 440, 1100, 540) // 60% of each, and neither contains the other
	dets := []vision.Detection{
		{Class: "button", Bounds: a, Confidence: 0.8},
		{Class: "button", Bounds: b, Confidence: 0.8},
	}
	reader := newBoxReader(map[image.Rectangle]string{a: "SETTINGS", b: "SETTINGS"},
		vision.DefaultLabelThresholds().Upscale)
	rt := visionShadowRuntime(t, dets, reader, fixtureCapture{generation: 1})

	sample := sampleWithVision(t, rt)
	if reader.calls != 0 {
		t.Errorf("an unattributable region was still read %d times", reader.calls)
	}
	if sample.Shadow != nil && len(sample.Shadow.Semantic.Terms) != 0 {
		t.Fatalf("ambiguous text became the terms %v. Association was resolved by guessing",
			sample.Shadow.Semantic.Terms)
	}
	// Stacked controls that merely touch are NOT ambiguous — that is the ordinary case and
	// refusing it would name nothing on any real menu.
	dets2, text2 := accessibilityPoorFixture()
	rt2 := visionShadowRuntime(t, dets2,
		newBoxReader(text2, vision.DefaultLabelThresholds().Upscale),
		fixtureCapture{generation: 1})
	if s := sampleWithVision(t, rt2); s.Shadow == nil || len(s.Shadow.Semantic.Terms) == 0 {
		t.Fatal("a plain vertical menu was treated as ambiguous; the overlap rule is too strict")
	}
}

// ── PART 13: provenance ───────────────────────────────────────────────────────

// Evidence from a window generation the cycle was not about must not become semantic.
//
// Mutation 6 (drop the TargetProven gate in shadowSampleFor) fails here. The consequence of
// getting this wrong is larger for a term than for a box: a box is counted this session, a term
// reaches cross-session identity and outlives the mistake.
func TestSemanticEvidenceIsRefusedFromAStaleTarget(t *testing.T) {
	dets, text := accessibilityPoorFixture()
	reader := newBoxReader(text, vision.DefaultLabelThresholds().Upscale)
	// The request expects generation 1 (sampleRequest); the frame proves generation 8.
	rt := visionShadowRuntime(t, dets, reader, fixtureCapture{generation: 8})

	sample := sampleWithVision(t, rt)
	if sample.Shadow == nil {
		t.Fatal("no shadow record")
	}
	if sample.Shadow.TargetProven {
		t.Fatal("a frame from generation 8 was accepted against an expected generation 1; " +
			"the provenance guard is not running and this test cannot mean anything")
	}
	if len(sample.Shadow.Semantic.Terms) != 0 || sample.Shadow.Semantic.Observed {
		t.Fatalf("semantic evidence survived an unproven target: %+v. Terms read off a "+
			"replaced window would reach durable identity", sample.Shadow.Semantic)
	}
}

// ── PART 14: unknown is not empty, on this path too ───────────────────────────

// A pass with no text budget looked at nothing, and must say so.
//
// Mutation 5 (always report Observed) fails here. Without the distinction, a cycle that never
// paid for a reading is indistinguishable from a screen with no interface concepts — and the
// matcher then reads a remembered subject's terms against it as a positive DISAGREEMENT.
func TestAVisionPassWithoutTheTextBudgetReportsUnknownNotEmpty(t *testing.T) {
	dets, text := accessibilityPoorFixture()
	reader := newBoxReader(text, vision.DefaultLabelThresholds().Upscale)
	rt := visionShadowRuntime(t, dets, reader, fixtureCapture{generation: 1})

	live := rt.newObservationSampler(sessionClock).(*liveSampler)
	req := sampleRequest()
	req.ReadLabels = false // no budget this cycle
	sample, err := live.Sample(context.Background(), req)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}

	if reader.calls != 0 {
		t.Errorf("scoped reading ran %d times on a cycle that did not ask for text; the "+
			"budget is not a budget", reader.calls)
	}
	if sample.Shadow != nil && sample.Shadow.Semantic.Observed {
		t.Fatal("a cycle that read nothing claims perception looked and found no concepts")
	}
	// The structure is still reported. Not reading is not not seeing.
	if sample.Shadow == nil || sample.Shadow.Detections == 0 {
		t.Fatal("declining to read text also lost the structure")
	}
}

// ── PART 6/7: the privacy surfaces stay separate ──────────────────────────────

// Nothing readable reaches the durable record, whatever the detector read.
//
// A control's name may be kept in the clear for the length of a session, because a structural
// role vouches for it. A durable semantic term may not be text at all. These are different
// surfaces and this asserts on the SHAPE of what crosses, so no care at the call sites is being
// relied on.
func TestNoReadableTextLeavesTheVisionSemanticBoundary(t *testing.T) {
	dets, text := accessibilityPoorFixture()
	// A person's name, on a control whose role IS nameable, and inside a text region.
	text[image.Rect(700, 610, 1100, 660)] = "Dave Wilson"
	text[image.Rect(700, 300, 1100, 350)] = "xX SomePlayer Xx"
	reader := newBoxReader(text, vision.DefaultLabelThresholds().Upscale)
	rt := visionShadowRuntime(t, dets, reader, fixtureCapture{generation: 1})

	sample := sampleWithVision(t, rt)
	if sample.Shadow == nil {
		t.Fatal("no shadow record")
	}
	for _, term := range sample.Shadow.Semantic.Terms {
		if !term.Known() {
			t.Errorf("term %q is not from the closed vocabulary", term)
		}
		if strings.Contains(strings.ToLower(string(term)), "dave") ||
			strings.Contains(strings.ToLower(string(term)), "player") {
			t.Errorf("read text reached the semantic record as %q", term)
		}
	}
	// And the shadow record carries no text field at all — there is nowhere to put one.
	if sample.Shadow.Semantic.EditableFields != 0 {
		t.Errorf("a vision-only pass reported %d editable fields; a detector cannot know that",
			sample.Shadow.Semantic.EditableFields)
	}
}

// ── PART 21: the budget is visible ────────────────────────────────────────────

// A cap that drops readings says so, and says which kind of drop it was.
//
// "The detector found nothing nameable", "the regions were too small" and "we stopped looking"
// are three different findings, and a report that cannot separate them will be read as the
// first one every time.
func TestTheLabelBudgetIsReportedRatherThanSilent(t *testing.T) {
	var dets []vision.Detection
	text := map[image.Rectangle]string{}
	for i := 0; i < 20; i++ {
		r := image.Rect(700, 100+i*40, 1100, 130+i*40)
		dets = append(dets, vision.Detection{Class: "button", Bounds: r, Confidence: 0.8})
		text[r] = "SETTINGS"
	}
	// Two unsayable regions and one too small to hold a word.
	dets = append(dets,
		vision.Detection{Class: "icon", Bounds: image.Rect(10, 10, 90, 90), Confidence: 0.8},
		vision.Detection{Class: "panel", Bounds: image.Rect(10, 200, 400, 600), Confidence: 0.8},
		vision.Detection{Class: "button", Bounds: image.Rect(10, 700, 30, 712), Confidence: 0.8},
	)
	reader := newBoxReader(text, vision.DefaultLabelThresholds().Upscale)
	rt := visionShadowRuntime(t, dets, reader, fixtureCapture{generation: 1})

	sampleWithVision(t, rt)

	if reader.calls > shadowMaxLabels {
		t.Fatalf("the reader ran %d times against a ceiling of %d; one cycle can now cost "+
			"%d serial OCR round trips", reader.calls, shadowMaxLabels, reader.calls)
	}
	diag := shadowLabelBudget(rt.shadowVision)
	if diag.Skipped == 0 {
		t.Error("readings were dropped and LabelsSkipped stayed 0; a silent cap reads as " +
			"\"nothing to find here\"")
	}
	if diag.Unsayable != 2 {
		t.Errorf("LabelsUnsayable = %d, want 2 — the number that says where a detector's "+
			"usefulness stops", diag.Unsayable)
	}
	if diag.Read == 0 {
		t.Error("the budget dropped everything")
	}
}

// menuRegions is a four-control menu with one HUD icon, in window-relative geometry.
func menuRegions() []observe.ShadowRegion {
	out := []observe.ShadowRegion{{
		Role: "icon", Confidence: 0.5,
		Region: observe.Region{X: 0.05, Y: 0.83, Width: 0.04, Height: 0.07},
	}}
	for _, y := range []float64{0.370, 0.435, 0.500, 0.565} {
		out = append(out, observe.ShadowRegion{
			Role: "button", Nameable: true, Confidence: 0.7,
			Region: observe.Region{X: 0.364, Y: y, Width: 0.208, Height: 0.046},
		})
	}
	return out
}

var windowRef1 = sampleRequest().Window
