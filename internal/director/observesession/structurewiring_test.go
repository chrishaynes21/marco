package observesession_test

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// The live blocker, reduced.
//
// Watch reported this against Chrome and VS Code:
//
//	accessibility  687 obs
//	fusion         687 obs -> 685 elements
//	observation    0 screens   0 transitions
//
// Hundreds of fused elements, no screen. This file is the smallest thing that reproduces
// that shape, and it is deliberately written BEFORE the fix so the fix can be shown to
// change it.
//
// The shape is not exotic. It is what an ordinary application looks like: an accessibility
// tree that fusion turned into elements, and NO structural detector — which is the default
// deployment, because the detector is opt-in behind $MARCO_SHADOW_VISION and costs ~1.25 GB.

// accessibleSampler is an application with a real accessibility tree and no detector.
//
// Two screens alternating, exactly like alternatingSampler — the SAME evidence, arriving
// as fused entities instead of as detector regions. That is the whole experiment: if
// segmentation is about structure, these two samplers must produce the same number of
// screens, and only the provenance should differ.
type accessibleSampler struct{ calls int }

func (s *accessibleSampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	s.calls++
	entities := []observe.EntitySnapshot{{
		Identity: "hud", Role: "icon", Confidence: 0.9,
		Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}}
	if s.calls%2 == 1 {
		for i, y := range []float64{0.437, 0.480, 0.520, 0.562} {
			entities = append(entities, observe.EntitySnapshot{
				Identity: observe.Digest(string(rune('a' + i))), Role: "button",
				Confidence: 0.9, Actionable: true,
				Region: observe.Region{X: 0.414, Y: y, Width: 0.172, Height: 0.036},
			})
		}
	}
	// No Shadow field at all. This is what the default Director produces.
	//
	// Structure is set the way the live sampler sets it — see cmd/director's
	// `fusedStructure`, and TestFusedStructureReachesTheScreenModel for the proof that
	// production actually fills it. It is supplied here rather than derived from Entities
	// because deriving it would put a provenance decision in the analysis core: whether a
	// cycle proved its target is knowable only where the provider outcomes are, and a core
	// that guessed would segment from evidence fusion had refused.
	return observe.Sample{Entities: entities, Structure: structureOf(entities)}, nil
}

// structureOf mirrors what the composition root hands a live session.
func structureOf(entities []observe.EntitySnapshot) observe.StructuralView {
	out := observe.StructuralView{Source: observe.StructureFused}
	for _, e := range entities {
		out.Regions = append(out.Regions, observe.ShadowRegion{
			Role: string(e.Role), Region: e.Region, Confidence: e.Confidence,
		})
	}
	return out
}

// THE regression. An application with an accessibility tree and no detector must produce
// screens.
//
// Before the fix this reported 0. Screen segmentation read `Sample.Shadow.Regions` and
// nothing else, so the entire screen model — and therefore recognition, relationships,
// learning, rehearsal and plays — was reachable only through an opt-in experiment.
func TestAnAccessibleApplicationProducesScreens(t *testing.T) {
	got, err := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&accessibleSampler{}, &recordingEvents{}).
		Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	sh := got.Stats.Shadow

	if len(sh.States) < 2 {
		t.Fatalf("an application alternating between two structurally different screens "+
			"produced %d screen state(s).\n"+
			"It supplied %d fused elements per sample and no structural detector, which is "+
			"the DEFAULT deployment — so with this result no ordinary application can ever "+
			"be recognised, related, learned from or rehearsed.",
			len(sh.States), 5)
	}
	if len(sh.Transitions) == 0 {
		t.Error("no transitions across a session that alternated between two screens")
	}
	if sh.CurrentState == "" || sh.CurrentState == observe.ScreenStateUnknown {
		t.Errorf("the session ended without knowing which screen it was on: %q",
			sh.CurrentState)
	}
}

// skippingDetectorSampler is the shape a Director with BOTH sources actually produces.
//
// The detector's cadence gate declines most slots — it costs ~0.9s an inference — so a
// typical sample carries a shadow record with `Ran: false` beside a perfectly good fused
// composition. The screen model has to be built from the composition on those slots, which
// is nearly all of them.
//
// Found by mutation: removing the screen-model publish from the experiment's fold path
// survived every test, because every test either had a detector on every slot or had no
// detector at all. Production has neither shape.
type skippingDetectorSampler struct{ calls int }

func (s *skippingDetectorSampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	s.calls++
	entities := []observe.EntitySnapshot{{
		Identity: "hud", Role: "icon",
		Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}}
	if s.calls%2 == 1 {
		for i, y := range []float64{0.437, 0.480, 0.520, 0.562} {
			entities = append(entities, observe.EntitySnapshot{
				Identity: observe.Digest(string(rune('a' + i))), Role: "button",
				Region: observe.Region{X: 0.414, Y: y, Width: 0.172, Height: 0.036},
			})
		}
	}
	return observe.Sample{
		Entities:  entities,
		Structure: structureOf(entities),
		// The detector was configured and sat this slot out. Every slot, which is the
		// worst case and close to the real one.
		Shadow: &observe.ShadowSample{Detector: "screenparser", Ran: false},
	}, nil
}

// A detector that skips its slots must not take the screen model down with it.
//
// The two failure modes are independent: the experiment declining a slot says nothing about
// whether the window had a composition, and the accessibility tree that described it is
// still right there on the sample.
func TestASkippedDetectorSlotStillSegmentsTheFusedComposition(t *testing.T) {
	got, err := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&skippingDetectorSampler{}, &recordingEvents{}).
		Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	sh := got.Stats.Shadow

	if len(sh.States) < 2 {
		t.Fatalf("a Director whose detector skipped every slot produced %d screen(s), "+
			"while its accessibility tree described two. The experiment's cadence took "+
			"the screen model with it", len(sh.States))
	}
	if sh.Structure != observe.StructureFused {
		t.Errorf("the screens were segmented from %q", sh.Structure)
	}
	// And the experiment's own accounting is still honest about what IT did.
	if sh.Inferences != 0 {
		t.Errorf("a detector that ran nothing reported %d inferences", sh.Inferences)
	}
	if sh.Skipped == 0 {
		t.Error("a detector that skipped every slot reported no skips")
	}
}

// unobservedSampler is a session in which nothing described the window.
//
// A validated target, a sample that arrives, and no source that could say what was on it —
// the shape of a Director whose only provider could not prove its target.
type unobservedSampler struct{}

func (unobservedSampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	return observe.Sample{
		Structure: observe.StructuralView{Why: "no provider proved it observed this window"},
	}, nil
}

// Unknown must not become a screen.
//
// The rule the whole fix rests on. An empty composition from a source that LOOKED is the
// sparse screen — real, recurring, and minted. An empty composition from a source that did
// not look is silence, and minting a state from silence would give every track a screen to
// be absent from and every session a universal state.
//
// Found by mutation: deleting the gate in ShadowTracker.Observe survived, because nothing
// drove a session whose composition was unobserved.
func TestASessionThatObservedNothingMintsNoScreen(t *testing.T) {
	got, err := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		unobservedSampler{}, &recordingEvents{}).
		Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	sh := got.Stats.Shadow

	if len(sh.States) != 0 {
		t.Fatalf("a session in which nothing described the window minted %d screen(s). "+
			"Marco would place every track in a screen it invented out of silence",
			len(sh.States))
	}
	if sh.CurrentState != "" && sh.CurrentState != observe.ScreenStateUnknown {
		t.Errorf("it also decided it was ON one: %q", sh.CurrentState)
	}
	if sh.StructureWhy == "" {
		t.Error("and it cannot say why, so a person sees only 'no screens'")
	}
}

// The control for the test above: a source that LOOKED and found nothing does mint a screen.
//
// Without this, the assertion above would hold for a segmenter that had simply stopped
// working, and a genuinely sparse application would silently lose its screen.
func TestASourceThatLookedAndFoundNothingStillMintsAScreen(t *testing.T) {
	got, err := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		emptyButObservedSampler{}, &recordingEvents{}).
		Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got.Stats.Shadow.States) == 0 {
		t.Fatal("a provider that proved its target and found no structure produced no " +
			"screen; a sparse application could never be observed at all")
	}
}

type emptyButObservedSampler struct{}

func (emptyButObservedSampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	return observe.Sample{
		Structure: observe.StructuralView{Source: observe.StructureFused},
	}, nil
}

// The two provenances must agree about the STRUCTURE.
//
// The same two screens, once through a detector and once through an accessibility tree,
// must segment the same way. If they do not, segmentation is keyed on something about the
// provider rather than on something about the interface — which is the defect this
// milestone is about, in a different disguise.
func TestDetectorAndAccessibilityStructureSegmentAlike(t *testing.T) {
	run := func(s observesession.Sampler) observesession.Result {
		t.Helper()
		got, err := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, s,
			&recordingEvents{}).Run(context.Background(), config())
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return got
	}

	detector := run(&alternatingSampler{}).Stats.Shadow
	accessible := run(&accessibleSampler{}).Stats.Shadow

	if len(detector.States) != len(accessible.States) {
		t.Errorf("the same two screens segmented into %d states from a detector and %d "+
			"from an accessibility tree", len(detector.States), len(accessible.States))
	}
}
