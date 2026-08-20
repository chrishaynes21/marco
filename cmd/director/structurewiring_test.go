package main

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The composition root's half of the screen fix.
//
// `observesession` proves that a sample carrying an observed composition produces screens.
// This proves the other half: that a live sample actually carries one. Between them there is
// no gap a Director could fall into, which is the failure mode this repository keeps finding
// — a mechanism that exists, is tested, and is never handed anything.

// THE wiring test. A live sample carries the fused composition.
//
// Deleting `sample.Structure = fusedStructure(...)` from buildSample returns the Director to
// having no screen model unless the experimental detector is switched on — which is the live
// blocker, and which produces no error anywhere.
func TestFusedStructureReachesTheScreenModel(t *testing.T) {
	rt := &Runtime{collector: &providers.Collector{}, engine: labelledEngine{}}
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	sample, err := live.Sample(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}

	structure := observe.StructureOf(sample)
	if !structure.Observed() {
		t.Fatalf("a sample from a cycle that fused an element carries no observed "+
			"composition: %q\n"+
			"With this, screen segmentation never runs and no application can be "+
			"recognised, related or learned from.", structure.Why)
	}
	if structure.Source != observe.StructureFused {
		t.Errorf("the composition came from %q, want the authoritative fused world",
			structure.Source)
	}
	if len(structure.Regions) == 0 {
		t.Fatal("the fused element did not become a structural region")
	}
	r := structure.Regions[0]
	if r.Role == "" {
		t.Error("a structural region with no role contributes nothing to a signature")
	}
	// Window-relative and normalised. A signature built from desktop pixels would change
	// the moment the window moved.
	if r.Region.X < 0 || r.Region.X > 1 || r.Region.Width <= 0 || r.Region.Width > 1 {
		t.Errorf("region %+v is not window-relative and normalised", r.Region)
	}
}

// N. The composition carries no content.
//
// A screen is role and place. If a label, a title or a text run could ride into segmentation
// then screen IDENTITY — which reaches durable memory — would depend on what a control said,
// and [[ADR-017-structure-earns-a-name-text-never-earns-structure]] would be inverted.
func TestTheCompositionCarriesNoScreenContent(t *testing.T) {
	rt := &Runtime{collector: &providers.Collector{}, engine: labelledEngine{}}
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	sample, err := live.Sample(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	// labelledEngine fuses an element whose label reads "Settings". The word must not be
	// anywhere in the structural view.
	for _, r := range observe.StructureOf(sample).Regions {
		if r.Role == "Settings" {
			t.Error("a label reached the composition as a role")
		}
	}
	// Structurally: there is no field on a region that could hold one.
	var region observe.ShadowRegion
	_ = region.Role
	_ = region.Region
	_ = region.Nameable
	_ = region.Confidence
}

// J. A cycle that proved nothing produces no composition.
//
// Wrong or stale targeting fails CLOSED. A frame nothing could attribute to the watched
// window must not become a screen — it would be a screen made of somebody else's window,
// and every track would then be judged present or absent against it.
// J. Wrong or stale targeting fails closed.
//
// Fusion refuses evidence whose provider could not prove which window it described, so an
// unproven cycle reaches here with NO entities. What has to be true then is that the empty
// result reads as "nothing is known", not as "the screen was empty" — because an empty
// screen is a real, recurring state and would be minted, and every track would then be
// judged absent from a screen made of somebody else's window.
func TestAnUnprovenCycleProducesNoComposition(t *testing.T) {
	frame := directorapi.Rect{Width: 1920, Height: 1080}
	target := &directorapi.TargetProvenance{Application: "testgame", WindowGeneration: 7}

	unproven := observation.Cycle{
		Request: observation.Request{Target: target},
		Outcomes: []observation.ProviderOutcome{{
			Source: directorapi.SourceAccessibility, Scope: directorapi.ScopeTarget,
			State: observation.StateProvenanceMismatch,
			// ExpectedTarget is set the way `providers.run` sets it in production; without
			// it TargetProven answers `true` for an untargeted cycle and the test would be
			// asserting against a shape production never produces.
			ExpectedTarget: *target,
		}},
	}
	v := fusedStructure(nil, unproven, frame, seenInto())
	if v.Observed() {
		t.Fatal("a cycle in which nothing proved its target produced an OBSERVED empty " +
			"composition. Marco would mint a screen out of a window it could not identify")
	}
	if v.Why == "" {
		t.Error("the refusal is silent, so a person would only see 'no screens'")
	}

	// The proven equivalent, which really did look and really did find nothing, IS
	// observed — otherwise the assertion above would hold for the wrong reason and a
	// genuinely sparse application would never get a screen.
	proven := unproven
	proven.Outcomes = []observation.ProviderOutcome{{
		Source: directorapi.SourceAccessibility, Scope: directorapi.ScopeTarget,
		State: observation.StateEmpty, ExpectedTarget: *target, ObservedTarget: *target,
	}}
	if v := fusedStructure(nil, proven, frame, seenInto()); !v.Observed() {
		t.Fatal("a provider that proved its target and found nothing structural was read " +
			"as not having looked; a sparse application could never have a screen")
	}
}

// A provider that was not asked to run has not looked.
//
// The distinction the vision opt-in created. An opt-out vision provider must not be able to
// stand in for "something observed this window" — it did not.
func TestANotRequestedProviderIsNotAnObservation(t *testing.T) {
	cycle := observation.Cycle{
		Request: observation.Request{Target: &directorapi.TargetProvenance{
			Application: "testgame", WindowGeneration: 7}},
		Outcomes: []observation.ProviderOutcome{{
			Source: directorapi.SourceVision, Scope: directorapi.ScopeTarget,
			State: observation.StateNotRequested,
		}},
	}
	if v := fusedStructure(nil, cycle, directorapi.Rect{Width: 100, Height: 100}, seenInto()); v.Observed() {
		t.Fatal("a provider that was never asked to run counted as having observed the window")
	}
}

// A window with no usable bounds produces no composition.
//
// Everything downstream is window-relative. Dividing by a zero frame would place every
// region at the origin and collapse every screen into one.
func TestAFrameWithNoBoundsProducesNoComposition(t *testing.T) {
	cycle := observation.Cycle{Outcomes: []observation.ProviderOutcome{{
		Source: directorapi.SourceAccessibility, State: observation.StateContributed}}}
	if v := fusedStructure(nil, cycle, directorapi.Rect{}, seenInto()); v.Observed() {
		t.Fatal("a window with no bounds produced a composition")
	}
}

// D. Elements alone are not a screen.
//
// Hundreds of fused elements with no extent or no role contribute nothing, so a Director
// cannot conclude "there are many elements, therefore there is a screen". The structure has
// to be structure.
func TestElementsWithoutStructureContributeNothing(t *testing.T) {
	frame := directorapi.Rect{Width: 1920, Height: 1080}
	cycle := observation.Cycle{Outcomes: []observation.ProviderOutcome{{
		Source: directorapi.SourceAccessibility, State: observation.StateContributed}}}

	var junk []observe.EntitySnapshot
	for i := 0; i < 500; i++ {
		junk = append(junk, observe.EntitySnapshot{
			Identity: observe.Digest("x"), Role: "", // no kind
			Region: observe.Region{X: 0.5, Y: 0.5},
		})
	}
	v := fusedStructure(junk, cycle, frame, seenInto())
	if !v.Observed() {
		t.Fatal("the cycle looked, so the view must be observed even if it is empty")
	}
	if len(v.Regions) != 0 {
		t.Errorf("%d roleless, extentless elements became structural regions", len(v.Regions))
	}
}

// ── the whole path, through the real registry ─────────────────────────────────

// E / F / G. An unfamiliar screen EXISTS without being recognised, named, or read.
//
// The success criterion of this milestone in one test. A Director that has never seen this
// application before, has no user-supplied name for anything, and read no text must still be
// able to say "I am looking at one coherent screen, and it changed".
func TestAnUnfamiliarScreenExistsWithoutRecognitionNameOrText(t *testing.T) {
	restore := sessionClock
	sessionClock = newDryClock()
	t.Cleanup(func() { sessionClock = restore })

	g := newObservationRegistry() // NO memory: nothing has ever been remembered
	id, err := g.Start(dryTarget{}, &accessibleOnlySampler{}, observesession.NopEvents{},
		windowref.Selector{Application: "testgame"}, dryBounds())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for g.ActiveID() != "" {
		if time.Now().After(deadline) {
			t.Fatal("the session never finished")
		}
		time.Sleep(time.Millisecond)
	}

	view, ok := g.Snapshot(id)
	if !ok {
		t.Fatal("the session vanished")
	}
	sh := view.Stats.Shadow

	if len(sh.States) < 2 {
		t.Fatalf("an unfamiliar application produced %d screen(s)", len(sh.States))
	}
	if len(sh.Transitions) == 0 {
		t.Error("no transition across a session that alternated between two screens")
	}
	// EXISTENCE, without any of the things that are not existence.
	if view.MemoryUnavailable == "" && len(view.Proposals) > 0 {
		for _, p := range view.Proposals {
			if p.Recognised {
				t.Error("a Director with no memory recognised something")
			}
		}
	}
	for _, st := range sh.States {
		if st.TermObservations != 0 {
			t.Errorf("state %s carries term evidence from a session that read no text", st.ID)
		}
	}
	if sh.Structure != observe.StructureFused {
		t.Errorf("the screens were segmented from %q", sh.Structure)
	}
}

// H. Vision being absent does not destroy an accessibility screen.
//
// The provider-independence half. An accessibility-only Director is the ordinary case, and it
// must be a fully capable one.
func TestMissingVisionDoesNotDestroyAnAccessibilityScreen(t *testing.T) {
	frame := directorapi.Rect{Width: 1920, Height: 1080}
	cycle := observation.Cycle{Outcomes: []observation.ProviderOutcome{
		{Source: directorapi.SourceAccessibility, State: observation.StateContributed},
		{Source: directorapi.SourceVision, State: observation.StateNotRequested},
	}}
	entities := []observe.EntitySnapshot{{
		Identity: "a", Role: "button",
		Region: observe.Region{X: 0.4, Y: 0.4, Width: 0.1, Height: 0.05},
	}}
	v := fusedStructure(entities, cycle, frame, seenInto())
	if !v.Observed() || len(v.Regions) != 1 {
		t.Fatalf("an accessibility-only cycle produced %+v", v)
	}
}

// I. Accessibility being absent does not destroy a detector screen.
//
// The other half, and the case the detector was built for: a game exposing no tree at all.
func TestMissingAccessibilityDoesNotDestroyADetectorScreen(t *testing.T) {
	sample := observe.Sample{
		// Fusion looked and found no structure — a real answer for a game.
		Structure: observe.StructuralView{Source: observe.StructureFused},
		Shadow: &observe.ShadowSample{
			Detector: "screenparser", Ran: true, TargetProven: true,
			Regions: []observe.ShadowRegion{{
				Role: "button", Region: observe.Region{X: 0.4, Y: 0.4, Width: 0.1, Height: 0.05},
			}},
		},
	}
	v := observe.StructureOf(sample)
	if v.Source != observe.StructureDetector {
		t.Fatalf("a game with no accessibility structure segmented from %q", v.Source)
	}
	if len(v.Regions) != 1 {
		t.Errorf("the detector's regions did not reach segmentation: %d", len(v.Regions))
	}
}

// C. Both providers present produce ONE composition, not two.
//
// The de-duplication guarantee. Fusion already merged the providers that describe the same
// thing; the experiment is deliberately outside fusion, so adding its boxes on top would
// double-count every control both sources found — and would let an experiment move screen
// identity, which is the influence the shadow boundary exists to prevent.
func TestBothProvidersPresentProduceOneComposition(t *testing.T) {
	sample := observe.Sample{
		Structure: observe.StructuralView{
			Source: observe.StructureFused,
			Regions: []observe.ShadowRegion{{
				Role: "button", Region: observe.Region{X: 0.4, Y: 0.4, Width: 0.1, Height: 0.05},
			}},
		},
		Shadow: &observe.ShadowSample{
			Detector: "screenparser", Ran: true, TargetProven: true,
			Regions: []observe.ShadowRegion{{
				// The SAME button, found again by the experiment.
				Role: "button", Region: observe.Region{X: 0.4, Y: 0.4, Width: 0.1, Height: 0.05},
			}},
		},
	}
	v := observe.StructureOf(sample)
	if v.Source != observe.StructureFused {
		t.Errorf("the experiment displaced the authoritative composition: %q", v.Source)
	}
	if len(v.Regions) != 1 {
		t.Fatalf("one button seen by two providers became %d regions", len(v.Regions))
	}
}

// ── fixtures ──────────────────────────────────────────────────────────────────

// accessibleOnlySampler is an ordinary application: an accessibility tree, no detector,
// no text.
type accessibleOnlySampler struct{ calls int }

func (s *accessibleOnlySampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	s.calls++
	regions := []observe.ShadowRegion{{
		Role: "icon", Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}}
	if s.calls%2 == 1 {
		for i, y := range []float64{0.437, 0.480, 0.520, 0.562} {
			regions = append(regions, observe.ShadowRegion{
				Role:   "button",
				Region: observe.Region{X: 0.414, Y: y, Width: 0.172, Height: 0.036},
			})
			_ = i
		}
	}
	return observe.Sample{
		Structure: observe.StructuralView{Source: observe.StructureFused, Regions: regions},
	}, nil
}

var _ = fusion.Report{}

// seenInto is a world Marco could actually see into: well observed, well covered, operable.
//
// The default for every structure test, because the interesting cases are about composition and
// not about perception quality. The shell gate has its own file.
func seenInto() directorapi.WorldConfidence {
	return directorapi.WorldConfidence{
		ObservationQuality: 0.90, Coverage: 0.80, Actionability: 0.90,
		IdentityDurability: 0.60, Freshness: 1,
	}
}

// A scroll bar's arrows are not part of the place.
//
// # Why this is asserted at buildSample and not at the classifier
//
// Because this is the seam where the hierarchy dies. `observation.Chrome` is proved where it
// lives, and `NewScreenSignature` is proved to skip what is marked; between them sits the one
// question neither can answer for itself — does the sample a live session produces still carry
// the answer. An `EntitySnapshot` has a role and a rectangle and no parent, so nothing
// downstream could ever work out for itself that a button is one of a scroll bar's arrows.
//
// The measured symptom of losing it: Windows Settings minted a fresh durable subject for the
// same page on nearly every visit, and three families of twins each differed from their named
// original by exactly the frame's own buttons. See [[ADR-062-a-scroll-bar-is-not-a-screen]].
//
// Deleting the Chrome classification in buildSample must fail this.
func TestAScrollBarsArrowsAreNotPartOfThePlace(t *testing.T) {
	const win = directorapi.WindowID("win_1")
	frame := directorapi.Rect{Width: 1920, Height: 1080}
	el := func(id, parent string, role directorapi.ElementRole, label string,
		x, y int) *directorapi.Element {

		e := &directorapi.Element{
			ID: directorapi.ElementID(id), WindowID: win, Role: role, Label: label,
			Bounds:  directorapi.Rect{X: x, Y: y, Width: 40, Height: 20},
			Enabled: true, Visible: true, Confidence: 0.9,
		}
		if parent != "" {
			p := directorapi.ElementID(parent)
			e.ParentID = &p
		}
		return e
	}
	world := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{
		"pane": el("pane", "", directorapi.RolePane, "", 0, 0),
		// THE PAGE.
		"save":   el("save", "pane", directorapi.RoleButton, "Save", 100, 100),
		"cancel": el("cancel", "pane", directorapi.RoleButton, "Cancel", 160, 100),
		// THE WINDOW'S OWN MACHINERY: a scroll bar, and the two buttons inside it. Roles
		// and hierarchy only — never the words "Line up", which are one operating system's
		// in one language.
		"bar":  el("bar", "pane", directorapi.RoleScrollBar, "", 1900, 0),
		"up":   el("up", "bar", directorapi.RoleButton, "Line up", 1900, 0),
		"down": el("down", "bar", directorapi.RoleButton, "Line down", 1900, 1060),
	}}

	sample := buildSample(world, observation.Cycle{},
		directorapi.Window{ID: win, Bounds: frame}, sampleRequest())

	want := map[string]bool{"Save": false, "Cancel": false, "Line up": true, "Line down": true}
	seen := map[string]bool{}
	for _, e := range sample.Entities {
		if e.Role != directorapi.RoleButton {
			continue
		}
		label := e.Label.Text
		w, known := want[label]
		if !known {
			continue
		}
		seen[label] = true
		if e.Chrome != w {
			t.Errorf("%q is carried as chrome=%v, want %v", label, e.Chrome, w)
		}
	}
	for label := range want {
		if !seen[label] {
			t.Fatalf("%q never reached the sample, so this proves nothing", label)
		}
	}

	// AND THE CONSEQUENCE, which is the only reason the flag exists. The page's identity
	// counts two buttons, not four; without the classification the same page looks like a
	// different place every time the scroll bar comes and goes.
	sig := observe.NewScreenSignature(observe.StructureOf(sample).Regions)
	if got := sig.Roles[string(directorapi.RoleButton)]; got != 2 {
		t.Fatalf("the place's identity counts %d button(s), want 2.\nThe scroll bar's own "+
			"arrows are being counted into what makes this screen this screen, so the same "+
			"page mints a new durable subject whenever the frame changes.", got)
	}
}
