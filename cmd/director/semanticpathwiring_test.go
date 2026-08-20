package main

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The whole semantic-discriminator path, from the request that asks for pixels to the signature
// semantic memory matches on.
//
// Each link is one line of production code and each has been, at some point in this repository,
// the line nobody called. So the chain is asserted end to end rather than in pieces:
//
//	request asks for OCR → labels arrive → classified to closed vocabulary → raw text dies
//	→ terms reach the subject fingerprint → TermsKnown is set → the durable signature carries it

// TestTheSamplerAsksForOCROnALabelPass is mutation 1's target.
//
// Scoped text reading is expensive and opt-in, so it happens only when the runner's budget allows
// — and that gate is exactly the kind of thing that can be silently removed, after which no term
// ever appears again and every report reads like an application with no readable text.
func TestTheSamplerAsksForOCROnALabelPass(t *testing.T) {
	rt := &Runtime{collector: &providers.Collector{}, engine: labelledEngine{}}
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	req := sampleRequest()
	req.ReadLabels = true
	asked := live.request(req)
	if !asked.Includes(directorapi.SourceOCR) {
		t.Fatalf("a label pass asked for %v; OCR is not among them, so the text provider "+
			"will never run and no interface concept can ever be classified", asked.Include)
	}

	// And the target is pinned, so whatever OCR reads can prove which window it came from.
	// Text lifted from a replaced window must never reach durable semantic identity.
	if asked.Window == nil || *asked.Window == "" {
		t.Error("the request does not pin a window; OCR evidence could not prove its target")
	}
	if asked.Target == nil {
		t.Error("the request carries no expected target, so the provenance guard has nothing " +
			"to compare an OCR reading against")
	}
}

// THE end-to-end path: a labelled element becomes a term on the durable signature.
//
// Mutation 2 (delete the SemanticEvidenceFrom call) and mutation 3 (drop terms out of the
// fingerprint) both fail here.
func TestLabelsBecomeTermsAndReachTheDurableSignature(t *testing.T) {
	rt := &Runtime{collector: &providers.Collector{}, engine: labelledEngine{}}
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	sample, err := live.Sample(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if sample.Shadow == nil {
		t.Fatal("no shadow record, so semantic evidence has nowhere to ride")
	}

	sem := sample.Shadow.Semantic
	if !sem.Observed {
		t.Fatal("the sample carries readable labels and reports that nothing was read. " +
			"Terms would be UNKNOWN forever, and the ratio denominator would stay zero")
	}
	var found bool
	for _, term := range sem.Terms {
		if term == observe.TermSettings {
			found = true
		}
	}
	if !found {
		t.Fatalf("a labelled button reading \"Settings\" produced terms %v. The classification "+
			"call is not on the production path", sem.Terms)
	}

	// Now the rest of the chain, through the real accumulator: terms must survive into a
	// state, into a hypothesis fingerprint, and into the signature memory matches on.
	var totals observe.ShadowTotals
	for i := 0; i < 8; i++ {
		s := *sample.Shadow
		s.Ran, s.TargetProven = true, true
		s.Regions = settingsRegions()
		s.Detections = len(s.Regions)
		s.Roles = map[string]int{"button": 4, "icon": 1}
		s.Nameable = 4
		totals.Add(s)
		totals.Add(gameplayShadow())
	}

	var carried bool
	for _, h := range observe.Hypotheses(totals, observe.DefaultHypothesisThresholds()) {
		sig := observe.SignatureOf(h)
		if !sig.TermsKnown {
			continue
		}
		for _, term := range sig.Terms {
			if term == observe.TermSettings {
				carried = true
			}
		}
	}
	if !carried {
		t.Fatal("the classified term never reached a durable structure signature. Semantic " +
			"memory would have no discriminator, every subject would be a candidate at best, " +
			"and nothing could ever be recognised across sessions")
	}
}

// THE privacy boundary: the words die at the classifier.
//
// Mutation: carry the label text alongside the term. This asserts on the SHAPE of what leaves
// the boundary, so no amount of care at the call sites is being relied on.
func TestRawLabelTextDiesAtTheClassificationBoundary(t *testing.T) {
	entities := []observe.EntitySnapshot{
		{Label: observe.SafeLabel{Text: "Settings", Digest: "d1"}},
		{Label: observe.SafeLabel{Text: "xX_SomePlayer_Xx", Digest: "d2"}},
		{Label: observe.SafeLabel{Text: "dave@example.com", Digest: "d3"}},
	}
	sem := observe.SemanticEvidenceFrom(entities)

	// The one recognisable concept survives; the personal text produced nothing.
	if len(sem.Terms) != 1 || sem.Terms[0] != observe.TermSettings {
		t.Fatalf("terms %v, want exactly [settings]", sem.Terms)
	}
	// And every term that leaves is from the closed vocabulary, by construction.
	for _, term := range sem.Terms {
		if !term.Known() {
			t.Errorf("term %q is not in the closed vocabulary", term)
		}
	}
	// A term is not a string field somebody could put a label in: the vocabulary is closed
	// and admission is checked again on the way into durable storage.
	sig := observe.StructureSignature{
		Terms: []observe.InterfaceTerm{"dave@example.com"}, TermsKnown: true,
	}
	for _, term := range sig.Terms {
		if term.Known() {
			t.Error("an arbitrary string passed as a known interface term")
		}
	}
	h := observe.Hypothesis{Subject: observe.Subject{Fingerprint: observe.Fingerprint{
		Terms: []observe.InterfaceTerm{"dave@example.com", observe.TermSettings},
	}}}
	got := observe.SignatureOf(h)
	for _, term := range got.Terms {
		if strings.Contains(string(term), "@") {
			t.Errorf("arbitrary text %q reached a durable signature; admission on the way "+
				"into memory is not being applied", term)
		}
	}
}

// Unavailable is not empty, on the production path.
//
// A sample with no readable labels must report terms UNKNOWN. If it reported an empty SET,
// cross-session matching would read a remembered subject's terms against it and conclude the
// screen DIFFERS — turning "Marco could not look" into "this is somewhere else".
func TestASampleWithNoReadableLabelsReportsUnknownNotEmpty(t *testing.T) {
	rt := &Runtime{collector: &providers.Collector{}, engine: stubEngine{}}
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	sample, err := live.Sample(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if sample.Shadow != nil && sample.Shadow.Semantic.Observed {
		t.Fatal("a sample with no readable labels claims perception looked and found nothing")
	}

	// And the consequence at the matcher: a remembered subject with terms must not be
	// declared DIFFERENT merely because this session could not read any.
	remembered := observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 4}, Members: 4,
		Terms: []observe.InterfaceTerm{observe.TermSettings}, TermsKnown: true,
	}
	current := observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 4}, Members: 4,
		TermsKnown: false,
	}
	if got := observe.CompareStructure(current, remembered); got == observe.MatchDifferent {
		t.Fatal("a session that could not read any text was told the screen DIFFERS from a " +
			"remembered one. A missing measurement became a positive disagreement, which is " +
			"the failure this representation exists to prevent")
	}
}

// settingsRegions is a four-control panel.
func settingsRegions() []observe.ShadowRegion {
	out := []observe.ShadowRegion{{
		Role: "icon", Confidence: 0.5,
		Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}}
	for _, y := range []float64{0.437, 0.480, 0.520, 0.562} {
		out = append(out, observe.ShadowRegion{
			Role: "button", Nameable: true, Confidence: 0.45,
			Region: observe.Region{X: 0.414, Y: y, Width: 0.172, Height: 0.036},
		})
	}
	return out
}

func gameplayShadow() observe.ShadowSample {
	return observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true, LatencyMS: 800,
		Regions: []observe.ShadowRegion{{
			Role: "icon", Confidence: 0.5,
			Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
		}},
		Detections: 1, Roles: map[string]int{"icon": 1},
	}
}

var _ = observation.Request{}
