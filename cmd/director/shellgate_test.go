package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A window nobody can see into is not a screen.
//
// # The live evidence
//
// A backgrounded application can stop exposing its interior entirely. Measured on Discord, one
// window, minutes apart:
//
//	active        hundreds of structures — button=138 group=182 text=156 …
//	backgrounded  pane=7 window=1, coverage 0.00, nothing operable
//
// Marco was minting durable places from the second one. A live learn established a "screen" of
// five controls in a 44-pixel strip — the husk of Settings after the person clicked away — and
// remembered it. Two junk subjects went into the store that way in one afternoon, and the START
// they poisoned is what `left_the_start` was complaining about.
//
// # Why the existing signal and not a new one
//
// `WorldConfidence.Shallow` already means "well observed but never seen into", in those words, and
// was written for exactly this shape — it names Electron and browsers as the signature. `Blind`
// already means nothing here can be operated. The condition needed no new detector, only a caller.
//
// BOTH are required, and that is what protects a genuinely simple application: `Actionability` is
// a SHARE rather than a count, so a two-button dialog scores as highly as a full toolbar. Small is
// not shallow; empty is.

// collapsed is what a backgrounded application's world looks like: well observed, nothing inside.
func collapsed() directorapi.WorldConfidence {
	return directorapi.WorldConfidence{
		ObservationQuality: 0.90, Coverage: 0.00, Actionability: 0.00,
		IdentityDurability: 0.25, Freshness: 1,
	}
}

// shellEntities is the husk: containers, no content. Roles and extents are all present, which is
// why a filter over the entities alone could never have caught this.
func shellEntities() []observe.EntitySnapshot {
	out := make([]observe.EntitySnapshot, 0, 8)
	for i := range 7 {
		out = append(out, observe.EntitySnapshot{
			Role: directorapi.RolePane, Confidence: 0.9,
			Region: observe.Region{
				X: 0, Y: float64(i) * 0.1, Width: 1, Height: 0.1,
			},
		})
	}
	return append(out, observe.EntitySnapshot{
		Role: directorapi.RoleWindow, Confidence: 0.9,
		Region: observe.Region{X: 0, Y: 0, Width: 1, Height: 1},
	})
}

func liveFrame() directorapi.Rect { return directorapi.Rect{Width: 1920, Height: 1080} }

// THE gate. Deleting it must fail this.
func TestACollapsedShellIsNotAComposition(t *testing.T) {
	v := fusedStructure(shellEntities(), provenCycle(), liveFrame(), collapsed())

	if v.Observed() {
		t.Fatalf("a backgrounded window's husk was admitted as a composition of %d region(s).\n"+
			"Everything downstream treats an observed composition as a screen: it segments, it "+
			"can be promoted, and an explicit learn will make it a durable place. Marco did "+
			"exactly that live and remembered a 44-pixel strip forever.", len(v.Regions))
	}
	if v.Why == "" {
		t.Error("the refusal carries no reason; a reader cannot tell it from 'nothing looked'")
	}
}

// The SAME structure, in a window Marco can see into, is admitted.
//
// The discriminator is perception quality, not the shape of the tree — so this must pass with the
// identical entities. Without it the gate could be a filter on panes, which would reject real
// container-heavy applications.
func TestTheSameStructureIsAdmittedWhenTheWindowIsHealthy(t *testing.T) {
	v := fusedStructure(shellEntities(), provenCycle(), liveFrame(), seenInto())
	if !v.Observed() {
		t.Fatalf("a healthy window was refused: %s.\nThe gate is about whether Marco could see "+
			"into the window, not about how many containers the tree has.", v.Why)
	}
	if len(v.Regions) != 8 {
		t.Errorf("%d region(s), want all 8", len(v.Regions))
	}
}

// A genuinely simple application is not rejected for being small.
//
// The false positive that would matter most. `Actionability` is a share, so a two-control dialog
// scores as highly as a toolbar; only a window with NOTHING operable is blind. A small app with
// real controls stays admissible however little of the window it covers.
func TestASmallApplicationWithRealControlsIsStillAScreen(t *testing.T) {
	sparse := directorapi.WorldConfidence{
		ObservationQuality: 0.90,
		Coverage:           0.10, // barely covers the window — shallow on its own
		Actionability:      1.00, // but everything in it works
		Freshness:          1,
	}
	if !sparse.Shallow() {
		t.Fatal("the fixture is not shallow, so it does not test what it claims")
	}
	entities := []observe.EntitySnapshot{{
		Role: directorapi.RoleButton, Confidence: 0.9,
		Region: observe.Region{X: 0.4, Y: 0.4, Width: 0.1, Height: 0.05},
	}, {
		Role: directorapi.RoleButton, Confidence: 0.9,
		Region: observe.Region{X: 0.4, Y: 0.5, Width: 0.1, Height: 0.05},
	}}
	v := fusedStructure(entities, provenCycle(), liveFrame(), sparse)
	if !v.Observed() {
		t.Fatalf("a two-button application was refused as a shell: %s.\nBeing small is not the "+
			"same as being closed, and rejecting it would make simple software impossible to learn in.",
			v.Why)
	}
}

// A blind-but-well-covered window is admitted too. Both conditions are required.
//
// A screen can legitimately have nothing operable on it — a results page, a progress view — and
// that is a real place somebody navigates to. Only blindness TOGETHER with never having been seen
// into describes a husk.
func TestAReadOnlyScreenIsStillAScreen(t *testing.T) {
	readOnly := directorapi.WorldConfidence{
		ObservationQuality: 0.90, Coverage: 0.80, Actionability: 0.00, Freshness: 1,
	}
	if !readOnly.Blind() {
		t.Fatal("the fixture is not blind, so it does not test what it claims")
	}
	if v := fusedStructure(shellEntities(), provenCycle(), liveFrame(), readOnly); !v.Observed() {
		t.Errorf("a screen with nothing to press was refused as a shell: %s", v.Why)
	}
}

// A window that collapsed to NOTHING is refused as a shell, not admitted as a sparse screen.
//
// The distinction the gate has to make, at its hardest. A proven cycle reporting no structure is
// the legitimately-empty case and IS a screen — that is deliberate, and predates this. But an
// application that closed up entirely reports the same zero, and calling that "an empty screen"
// would mint a durable place for every backgrounded window on the machine.
//
// Perception quality is what separates them, and it is the only thing that can.
func TestAWindowThatCollapsedToNothingIsNotAnEmptyScreen(t *testing.T) {
	if v := fusedStructure(nil, provenCycle(), liveFrame(), collapsed()); v.Observed() {
		t.Error("a window that closed up entirely was admitted as an empty screen; every " +
			"backgrounded application on the machine is now a place Marco could remember")
	}
	// And the legitimately-empty screen still is one, so the gate did not swallow both.
	if v := fusedStructure(nil, provenCycle(), liveFrame(), seenInto()); !v.Observed() {
		t.Errorf("a proven look at a genuinely empty screen stopped being an observation: %s",
			v.Why)
	}
}

// provenCycle is a cycle in which a provider looked at the pinned window and proved it.
//
// The composition question is then entirely about what was on the screen, which is what these
// tests are for.
func provenCycle() observation.Cycle {
	target := &directorapi.TargetProvenance{Application: "testgame", WindowGeneration: 7}
	return observation.Cycle{
		Request: observation.Request{Target: target},
		Outcomes: []observation.ProviderOutcome{{
			Source: directorapi.SourceAccessibility, Scope: directorapi.ScopeTarget,
			State: observation.StateEmpty, ExpectedTarget: *target, ObservedTarget: *target,
		}},
	}
}
