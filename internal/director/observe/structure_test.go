package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// What a screen is made of, and what it is not.
//
// These are about the CONTRACT rather than the wiring: which evidence segmentation is
// entitled to act on, and what a missing answer means. The wiring — that production actually
// hands it any of this — is proved in observesession and cmd/director.

// K. Fusion cannot silently drop the evidence segmentation needs.
//
// A composition is role and window-relative region. Both survive the narrowing, or the
// signature is built from something else and screen identity means something else.
func TestTheCompositionKeepsExactlyWhatASignatureReads(t *testing.T) {
	regions := []observe.ShadowRegion{
		{Role: "button", Region: observe.Region{X: 0.4, Y: 0.4, Width: 0.1, Height: 0.05}},
		{Role: "icon", Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10}},
	}
	sig := observe.NewScreenSignature(regions)
	if sig.Total != 2 {
		t.Fatalf("a two-region composition signed as %d", sig.Total)
	}
	if sig.Roles["button"] != 1 || sig.Roles["icon"] != 1 {
		t.Errorf("role composition lost: %v", sig.Roles)
	}
	if len(sig.Cells) != 2 {
		t.Errorf("arrangement lost: %v", sig.Cells)
	}

	// And a region with no role contributes nothing, so an accessibility tree full of
	// structureless nodes cannot inflate a signature.
	quiet := observe.NewScreenSignature([]observe.ShadowRegion{{Role: ""}})
	if quiet.Total != 0 {
		t.Errorf("a roleless region signed as %d", quiet.Total)
	}
}

// L. Screen identity survives harmless variation.
//
// A control that shifts a few pixels, a confidence that moves, a nameable flag that changes
// when a label happens to be read — none of these is a different screen. If any of them were,
// every application would produce a new state every second and nothing could ever recur.
func TestScreenIdentitySurvivesHarmlessVariation(t *testing.T) {
	var g observe.ScreenSegmenter

	base := menuRows(0.437)
	first := g.Observe(1, base, nil, observe.SemanticEvidence{})

	// The same screen, jittered: sub-cell movement, different confidences, a label read
	// this time and not last.
	jittered := menuRows(0.4404)
	for i := range jittered {
		jittered[i].Confidence = 0.91
		jittered[i].Nameable = true
	}
	again := g.Observe(2, jittered, nil, observe.SemanticEvidence{})

	if first != again {
		t.Fatalf("a few pixels of jitter made a new screen: %s then %s.\n"+
			"Every application would mint a state per sample and nothing could recur",
			first, again)
	}
	if first == observe.ScreenStateUnknown {
		t.Fatal("the first sighting of a four-row menu was unplaceable")
	}
}

// M. A meaningful structural change produces a different screen, and a transition.
//
// The other half of L, and it has to be asserted beside it: a segmenter that never changes
// its mind is as useless as one that changes it constantly, and only the pair says the
// threshold is anywhere sensible.
func TestAMeaningfulChangeProducesATransition(t *testing.T) {
	var g observe.ScreenSegmenter

	menuState := g.Observe(1, menuRows(0.437), nil, observe.SemanticEvidence{})
	sparse := []observe.ShadowRegion{
		{Role: "icon", Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10}},
	}
	// Twice: the first sighting of a composition a known screen already explains is held
	// rather than judged, and recurrence promotes it. That is the segmenter's own rule and
	// this test must not pretend otherwise.
	g.Observe(2, sparse, nil, observe.SemanticEvidence{})
	other := g.Observe(3, sparse, nil, observe.SemanticEvidence{})

	if other == menuState {
		t.Fatalf("a four-row menu and a bare HUD were read as the same screen (%s)", menuState)
	}
	if len(g.Transitions()) == 0 {
		t.Error("no transition was recorded between two different screens")
	}
}

// A source that did not look mints nothing.
//
// The rule the whole fix rests on: "observed and empty" is a screen, "not observed" is not.
// Collapsing them would let a Director with no working provider invent one universal screen
// and place every track in it.
func TestAnUnobservedCompositionIsNotAnEmptyScreen(t *testing.T) {
	if (observe.StructuralView{}).Observed() {
		t.Fatal("a view nothing supplied reported itself as observed")
	}
	empty := observe.StructuralView{Source: observe.StructureFused}
	if !empty.Observed() {
		t.Fatal("a source that looked and found nothing reported itself as not having looked")
	}
}

// The detector's gate is the original one, unchanged.
//
// Three conditions, and the detector's NAME is not among them. Adding a fourth refusal here
// would be this milestone quietly narrowing the rule it set out to widen.
func TestTheDetectorGateIsUnchanged(t *testing.T) {
	ran := &observe.ShadowSample{Detector: "screenparser", Ran: true, TargetProven: true}
	if !ran.Structure().Observed() {
		t.Error("a detector that ran and proved its target was refused")
	}
	unnamed := &observe.ShadowSample{Ran: true, TargetProven: true}
	if !unnamed.Structure().Observed() {
		t.Error("a record that ran and proved its target was refused for having no name")
	}

	for name, s := range map[string]*observe.ShadowSample{
		"skipped":     {Detector: "screenparser", Ran: false, TargetProven: true},
		"unproven":    {Detector: "screenparser", Ran: true, TargetProven: false},
		"unavailable": {Detector: "screenparser", Ran: true, TargetProven: true, Unavailable: "no model"},
		"nil":         nil,
	} {
		v := s.Structure()
		if v.Observed() {
			t.Errorf("%s: a detector that did not deliver was read as an observation", name)
		}
		if v.Why == "" {
			t.Errorf("%s: the refusal is silent", name)
		}
	}
}

// C. The authoritative composition wins, and the experiment cannot displace it.
//
// A screen identity reaches durable memory, decides what a track is measured against, and
// becomes the first line of a play. Letting an out-of-fusion experiment move it would be the
// influence the shadow boundary exists to prevent.
func TestTheExperimentCannotDisplaceTheAuthoritativeComposition(t *testing.T) {
	fused := observe.StructuralView{
		Source:  observe.StructureFused,
		Regions: []observe.ShadowRegion{{Role: "button", Region: observe.Region{Width: 0.1, Height: 0.1}}},
	}
	sample := observe.Sample{
		Structure: fused,
		Shadow: &observe.ShadowSample{
			Detector: "screenparser", Ran: true, TargetProven: true,
			Regions: []observe.ShadowRegion{
				{Role: "panel", Region: observe.Region{Width: 0.9, Height: 0.9}},
				{Role: "panel", Region: observe.Region{Width: 0.8, Height: 0.8}},
			},
		},
	}
	got := observe.StructureOf(sample)
	if got.Source != observe.StructureFused {
		t.Fatalf("the experiment displaced the authoritative composition: %q", got.Source)
	}
	if len(got.Regions) != 1 {
		t.Errorf("the experiment's regions were added to the authoritative ones: %d",
			len(got.Regions))
	}
}

// B / I. The detector alone can produce a screen where fusion sees nothing.
//
// The surface the detector was built for. Fusion looking and finding no structure is a real
// answer, and it must not veto the one source that has an answer.
func TestTheDetectorAnswersWhereFusionSeesNoStructure(t *testing.T) {
	sample := observe.Sample{
		Structure: observe.StructuralView{Source: observe.StructureFused},
		Shadow: &observe.ShadowSample{
			Detector: "screenparser", Ran: true, TargetProven: true,
			Regions: []observe.ShadowRegion{{Role: "button",
				Region: observe.Region{Width: 0.1, Height: 0.1}}},
		},
	}
	got := observe.StructureOf(sample)
	if got.Source != observe.StructureDetector || len(got.Regions) != 1 {
		t.Fatalf("a game with no accessibility structure got %+v", got)
	}
}

// Both silent is silent, and it says why.
func TestBothSourcesSilentReportsWhy(t *testing.T) {
	got := observe.StructureOf(observe.Sample{})
	if got.Observed() {
		t.Fatal("a sample nothing observed produced a composition")
	}
	if got.Why == "" {
		t.Error("the silence has no explanation, so 'no screens' is all anybody would see")
	}
}

func menuRows(top float64) []observe.ShadowRegion {
	out := []observe.ShadowRegion{
		{Role: "icon", Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10}},
	}
	for i := 0; i < 4; i++ {
		out = append(out, observe.ShadowRegion{
			Role: "button", Confidence: 0.4,
			Region: observe.Region{
				X: 0.414, Y: top + float64(i)*0.042, Width: 0.172, Height: 0.036},
		})
	}
	return out
}
