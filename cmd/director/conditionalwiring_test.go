package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/shadow"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The admission context, entered through the production sampler.
//
// # Why this is not a unit test of MenuLike
//
// MenuLike is a pure function and its own tests already pin its behaviour. What they cannot show
// is that the Director ever CALLS it, or that the answer ever reaches the producer that needs
// it. Without that link the whole milestone is inert: the predicate would be correct, its tests
// green, and every W the player pressed would still be refused.
//
// That is the failure this repository has now shipped four times, so this test drives the real
// `liveSampler.Sample` over the real collector, with a shadow provider whose detections are the
// only thing that decides the answer — and then presses an ambiguous key through the real
// producer to see whether the context arrived.
//
// Mutation: delete the SetScreenContext call from Sample and this must fail.

// fakeShadowProvider is a shadow-only provider that reports exactly the detections it is given.
//
// It implements TargetedProvider so it can prove which window generation it observed — an
// outcome that cannot prove its target is refused, and would leave the sampler's shadow record
// unproven and the context unset for a reason that has nothing to do with what is being tested.
type fakeShadowProvider struct{ regions []directorapi.Rect }

func (fakeShadowProvider) Name() string                  { return "screenparser" }
func (fakeShadowProvider) Sources() []observation.Source { return []observation.Source{"vision"} }
func (fakeShadowProvider) ShadowOnly()                   {}

func (f fakeShadowProvider) Observe(context.Context,
	observation.Request) ([]observation.Observation, error) {
	return nil, nil
}

func (f fakeShadowProvider) ObserveTargeted(_ context.Context,
	req observation.Request) observation.ProviderOutcome {

	out := observation.ProviderOutcome{
		Source: "vision", State: observation.StateContributed,
		StartedAt: time.Now(), FinishedAt: time.Now(),
	}
	if req.Target != nil {
		// Proven: observed exactly what was asked for. Anything else and the outcome is
		// refused before it reaches the assessment under test.
		out.ExpectedTarget, out.ObservedTarget = *req.Target, *req.Target
	}
	for i, r := range f.regions {
		out.Observations = append(out.Observations, observation.NewElement(directorapi.Observation{
			ID: directorapi.ObservationID(string(rune('a' + i))), Source: "vision",
			Role: directorapi.RoleButton, Bounds: r, Confidence: 0.5,
			Timestamp: time.Now(),
		}))
	}
	return out
}

// shadowRuntime builds a Director whose shadow detector reports the given controls.
func shadowRuntime(t *testing.T, controls int) (*Runtime, func(uint16, bool)) {
	t.Helper()
	// shadowDetectorName reads this; without it the sampler builds no shadow record at all
	// and there is nothing to assess.
	t.Setenv("MARCO_SHADOW_VISION", "screenparser")

	var rects []directorapi.Rect
	for i := 0; i < controls; i++ {
		rects = append(rects, directorapi.Rect{
			X: 800, Y: 400 + i*80, Width: 330, Height: 70,
		})
	}

	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { src.Close() })
	return &Runtime{
		navSource:    src,
		collector:    providers.NewCollector(fakeShadowProvider{regions: rects}),
		engine:       stubEngine{},
		shadowVision: &shadow.Provider{},
	}, press
}

// pressAmbiguous fires W and waits for the classifier to reach a verdict.
func pressAmbiguous(t *testing.T, press func(uint16, bool), src *navsource.Source) {
	t.Helper()
	before := src.Stats().Received
	press(navsource.KeyW, true)
	press(navsource.KeyW, false)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if src.Stats().Received >= before+2 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("the synthetic keypress never reached the classifier")
}

// THE production wiring test. A screen of choices makes an ambiguous key navigation.
func TestTheProductionSamplerPushesTheAdmissionContext(t *testing.T) {
	rt, press := shadowRuntime(t, 5)
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	// Before any observation, the Director has no idea what is on screen, so W is movement.
	pressAmbiguous(t, press, rt.navSource)
	if n := rt.navSource.Stats().Classified; n != 0 {
		t.Fatalf("%d intent(s) were classified before any screen had been observed", n)
	}

	// One real sampling cycle: collector → fusion → buildSample → shadow record → assessment.
	sample, err := live.Sample(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if sample.Shadow == nil || !sample.Shadow.Ran || !sample.Shadow.TargetProven {
		t.Fatalf("the sampler produced no usable shadow record (%+v); this test cannot say "+
			"anything about the assessment made from one", sample.Shadow)
	}
	if !observe.MenuLike(sample.Shadow.Regions) {
		t.Fatalf("the fixture's %d controls did not read as a set of choices; the scenario "+
			"is wrong, not the wiring", len(sample.Shadow.Regions))
	}

	// Now the same key means navigation.
	pressAmbiguous(t, press, rt.navSource)
	st := rt.navSource.Stats()
	if st.Classified == 0 {
		t.Fatal("after observing a screen of choices, an ambiguous key was still refused. " +
			"The sampler is not handing its assessment to the producer, so every W, A, S and " +
			"D the player presses is discarded and the transition graph stays unattributed")
	}
	if st.Conditional != st.Classified {
		t.Errorf("classified %d intents of which %d marked conditional; every one of these "+
			"was admitted on context and must say so", st.Classified, st.Conditional)
	}
}

// A screen that is not a set of choices must NOT license admission.
//
// The other half, and the one that keeps the relaxation honest: the same wiring, the same
// producer, a screen with one control — and the key stays movement.
func TestASparseScreenDoesNotLicenseAmbiguousKeys(t *testing.T) {
	rt, press := shadowRuntime(t, 1)
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	if _, err := live.Sample(context.Background(), sampleRequest()); err != nil {
		t.Fatalf("Sample: %v", err)
	}
	pressAmbiguous(t, press, rt.navSource)

	st := rt.navSource.Stats()
	if st.Classified != 0 {
		t.Errorf("%d intent(s) were admitted after observing a screen with a single control. "+
			"Somebody walking around is being recorded as somebody navigating", st.Classified)
	}
	if st.Ignored[navsource.ReasonAmbiguous] == 0 {
		t.Error("the refusal was not counted as ambiguous")
	}
}

// The assessment flips OFF when the screen stops being a set of choices.
//
// This is the mechanism freshness actually rests on, exercised through the production sampler
// rather than by calling SetScreenContext directly: leaving a menu must stop admission at the
// next observation, not when a timer expires.
func TestTheProductionSamplerFlipsAdmissionOffWhenChoicesLeaveTheScreen(t *testing.T) {
	rt, press := shadowRuntime(t, 5)
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	if _, err := live.Sample(context.Background(), sampleRequest()); err != nil {
		t.Fatalf("Sample: %v", err)
	}
	pressAmbiguous(t, press, rt.navSource)
	if rt.navSource.Stats().Classified == 0 {
		t.Fatal("the menu-like observation did not license admission; nothing to flip off")
	}
	admitted := rt.navSource.Stats().Classified

	// The player leaves. The next observation sees a bare screen.
	live.rt.collector = providers.NewCollector(fakeShadowProvider{
		regions: []directorapi.Rect{{X: 10, Y: 10, Width: 40, Height: 40}},
	})
	if _, err := live.Sample(context.Background(), sampleRequest()); err != nil {
		t.Fatalf("Sample: %v", err)
	}
	pressAmbiguous(t, press, rt.navSource)

	if got := rt.navSource.Stats().Classified; got != admitted {
		t.Errorf("%d further intent(s) were admitted after the choices left the screen; "+
			"admission did not flip off", got-admitted)
	}
}

// A skipped inference must not flip admission off.
//
// Unknown is not false ([[ADR-006-unknown-is-not-false]]). The detector sitting a slot out is not
// evidence the menu closed, and treating it as such would lose admission for an interval every
// time the cadence gate declined — which it did for 13 of 65 slots in the live session.
func TestASkippedInferenceLeavesTheAssessmentStanding(t *testing.T) {
	rt, press := shadowRuntime(t, 5)
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	if _, err := live.Sample(context.Background(), sampleRequest()); err != nil {
		t.Fatalf("Sample: %v", err)
	}
	pressAmbiguous(t, press, rt.navSource)
	admitted := rt.navSource.Stats().Classified
	if admitted == 0 {
		t.Fatal("the menu-like observation did not license admission")
	}

	// A slot the detector declined: no provider evidence at all.
	live.rt.collector = providers.NewCollector()
	if _, err := live.Sample(context.Background(), sampleRequest()); err != nil {
		t.Fatalf("Sample: %v", err)
	}
	pressAmbiguous(t, press, rt.navSource)

	if got := rt.navSource.Stats().Classified; got <= admitted {
		t.Error("a skipped inference turned admission off. Nothing observed the menu " +
			"closing; the previous assessment should stand until something does")
	}
}

// ── replay ────────────────────────────────────────────────────────────────────

// Context-admitted evidence must survive a trace and replay to the same interpretation.
//
// The parity property, extended to the weaker class of evidence. A trace that dropped the
// `Conditional` bit would replay an edge resting entirely on judgement as one resting on
// unambiguous navigation — turning a contested hypothesis into a supported one, offline, with no
// indication anything had changed.
func TestConditionalEvidenceSurvivesTheTraceAndReplaysIdentically(t *testing.T) {
	// A session where every attribution is context-admitted, plus one that is not, so both
	// the contested and the caveated paths are exercised.
	samples := conditionalSession()

	var live observe.ShadowTotals
	for _, s := range samples {
		live.Add(s)
	}
	liveHypotheses := observe.Hypotheses(live, observe.DefaultHypothesisThresholds())

	path := filepath.Join(t.TempDir(), "trace.jsonl")
	tr := &shadowTrace{path: path}
	for _, s := range samples {
		sample := s
		tr.record(&sample, 1)
	}
	slots, err := loadTrace(path)
	if err != nil {
		t.Fatalf("loadTrace: %v", err)
	}

	var replayed observe.ShadowTotals
	for _, s := range slots {
		replayed.Add(sampleFromSlot(s))
	}

	// The bit itself made the round trip.
	var sawConditional bool
	for _, s := range slots {
		for _, in := range s.Inputs {
			if in.Conditional {
				sawConditional = true
			}
		}
	}
	if !sawConditional {
		t.Fatal("no context-admitted intent survived the trace; a replayed session would " +
			"read judgement-based evidence as unambiguous navigation")
	}

	// And so did every conclusion drawn from it.
	if len(replayed.Transitions) != len(live.Transitions) {
		t.Fatalf("live produced %d edges, replay %d", len(live.Transitions),
			len(replayed.Transitions))
	}
	for _, want := range live.Transitions {
		got, ok := edgeFor(replayed, want.From, want.To)
		if !ok {
			t.Errorf("edge %s→%s missing from the replay", want.From, want.To)
			continue
		}
		if got.ConditionalOnly != want.ConditionalOnly {
			t.Errorf("edge %s→%s: live counted %d context-admitted observation(s), replay %d",
				want.From, want.To, want.ConditionalOnly, got.ConditionalOnly)
		}
	}

	replayedHypotheses := observe.Hypotheses(replayed, observe.DefaultHypothesisThresholds())
	if len(replayedHypotheses) != len(liveHypotheses) {
		t.Fatalf("live produced %d hypotheses, replay %d", len(liveHypotheses),
			len(replayedHypotheses))
	}
	for i := range liveHypotheses {
		l, r := liveHypotheses[i], replayedHypotheses[i]
		if l.Kind != r.Kind || l.Status != r.Status {
			t.Errorf("hypothesis %d: live %s/%s, replay %s/%s",
				i, l.Kind, l.Status, r.Kind, r.Status)
		}
		if len(l.Contradictions) != len(r.Contradictions) {
			t.Errorf("hypothesis %d (%s): live has %d contradiction(s), replay %d",
				i, l.Kind, len(l.Contradictions), len(r.Contradictions))
		}
	}
}

// conditionalSession is a scripted run whose menu is entered with a context-admitted key.
func conditionalSession() []observe.ShadowSample {
	hud := []observe.ShadowRegion{{
		Role: "icon", Confidence: 0.5,
		Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}}
	panel := append([]observe.ShadowRegion{}, hud...)
	for _, y := range []float64{0.437, 0.480, 0.520, 0.562} {
		panel = append(panel, observe.ShadowRegion{
			Role: "button", Nameable: true, Confidence: 0.45,
			Region: observe.Region{X: 0.414, Y: y, Width: 0.172, Height: 0.036},
		})
	}
	inference := func(regions []observe.ShadowRegion, in []observe.InputEvent) observe.ShadowSample {
		s := observe.ShadowSample{
			Detector: "screenparser", Ran: true, TargetProven: true, LatencyMS: 850,
			Regions: regions, Detections: len(regions), Roles: map[string]int{},
			Inputs: in,
		}
		for _, r := range regions {
			s.Roles[r.Role]++
			if r.Nameable {
				s.Nameable++
			}
		}
		return s
	}
	condUp := []observe.InputEvent{{Intent: observe.NavUp, Conditional: true}}

	var out []observe.ShadowSample
	out = append(out, inference(hud, nil), inference(hud, nil))
	for i := 0; i < 4; i++ {
		out = append(out,
			inference(panel, condUp), inference(panel, nil),
			inference(hud, nil), inference(hud, nil),
		)
	}
	return out
}
