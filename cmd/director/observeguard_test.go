package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Passive means passive.
//
// A session exists to describe how a PERSON uses an application. An action the Director
// took is not evidence about that, and a timeline that silently mixed the two would leave a
// later reader unable to tell which transitions were the player's.

func TestNoObservationMeansNoRefusal(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry()}
	if _, refused := rt.refuseWhileObserved(); refused {
		t.Fatal("actions were refused with no session running")
	}
}

func TestActionsAreRefusedWhileAWindowIsObserved(t *testing.T) {
	registry := newObservationRegistry()
	// Stand an active session up directly: the guard reads the registry, and starting a
	// real one would need a desktop.
	registry.activeID = "observe_3"
	rt := &Runtime{observations: registry}

	outcome, refused := rt.refuseWhileObserved()
	if !refused {
		t.Fatal("an action was allowed against a window under passive observation")
	}
	if outcome.Status != directorapi.ResultBlocked {
		t.Errorf("status = %q, want blocked", outcome.Status)
	}
	if !strings.Contains(outcome.Message, "observe_3") {
		t.Errorf("the refusal %q does not name the session", outcome.Message)
	}
	if !strings.Contains(outcome.Message, "cancel-observation") {
		t.Errorf("the refusal %q does not say how to proceed", outcome.Message)
	}
}

func TestTheGuardIsAbsentWithNoRegistry(t *testing.T) {
	// A Director built without observation must behave exactly as it did before.
	rt := &Runtime{}
	if _, refused := rt.refuseWhileObserved(); refused {
		t.Fatal("a Director with no registry refused an action")
	}
}

// A DEMONSTRATION OFFERS THE PLACE ITS NAME.
//
// The seam this proves is the one that was nearly wired to the wrong carrier. `ShadowSample` reads
// as the vision experiment's record and is not: the accessibility path attaches its semantic
// evidence to the same structure, which is how `terms` reach a durable Place on a Director with no
// detector configured. Wiring a place name one layer up would have compiled, read well, and fired
// only when the experiment was on.
//
// Deleting the AdmittedPlaceName call in the sampler must fail this.
func TestADemonstrationOffersThePlaceItsName(t *testing.T) {
	nav := &directorapi.Element{
		ID: "e1", Role: directorapi.RoleListItem, Label: "Home",
		Confidence: 1, Selected: true, Visible: true,
	}
	chooser := &directorapi.Element{
		ID: "e2", Role: directorapi.RoleComboBox, Label: "Color mode",
		Confidence: 1, Visible: true,
	}
	parent := chooser.ID
	value := &directorapi.Element{
		ID: "e3", Role: directorapi.RoleListItem, Label: "Dark", ParentID: &parent,
		Confidence: 1, Selected: true, Visible: true,
	}
	world := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{
		nav.ID: nav, chooser.ID: chooser, value.ID: value,
	}}

	evidence := placeNameEvidence(world)
	if len(evidence) != 2 {
		t.Fatalf("%d selected item(s) read from the world, want 2: %+v", len(evidence), evidence)
	}
	if got := observe.AdmittedPlaceName(evidence); got != "Home" {
		t.Errorf("the Place is called %q, want Home", got)
	}
}

// A SELECTED VALUE IS NOT OFFERED AS A PLACE NAME.
//
// Settings Home reports two selected items — `Home` in the navigation pane and `Dark` inside the
// Color-mode combo box. The walk up the parent chain is what tells them apart, and without it the
// rule sees two different answers and names the Place nothing at all.
func TestASelectedValueIsNotOfferedAsAPlaceName(t *testing.T) {
	chooser := &directorapi.Element{
		ID: "e2", Role: directorapi.RoleComboBox, Label: "Color mode", Visible: true,
	}
	parent := chooser.ID
	value := &directorapi.Element{
		ID: "e3", Role: directorapi.RoleListItem, Label: "Dark", ParentID: &parent,
		Confidence: 1, Selected: true, Visible: true,
	}
	got := placeNameEvidence(directorapi.WorldState{
		Elements: map[directorapi.ElementID]*directorapi.Element{
			chooser.ID: chooser, value.ID: value,
		},
	})
	if len(got) != 1 {
		t.Fatalf("%d piece(s) of evidence, want 1", len(got))
	}
	if !got[0].InsideValueChooser {
		t.Error("a selected item inside a combo box was offered as a destination. It says " +
			"what a setting is set to, not where anybody is.")
	}
}

// THE SAMPLER NAMES THE PLACE ONLY UNDER THE LICENCE.
//
// Entered through the sampler's own method rather than the free function, because what is being
// proved is that PRODUCTION reads the world this way — a free function proves the rule and not
// that anything calls it.
//
// This test used to be TestTheSamplerNamesThePlaceOnlyUnderTheLicence and asserted that a passive
// sampler read NOTHING. Roadmap 35A inverted that: naming is perception, and perception does not
// need a licence. The licence moved to where the name is written down. So the claim here becomes
// the stronger one — the sampler reads the same name either way, and the ONLY difference an
// acquisition context makes is what becomes durable, which `TestAPassiveSessionWritesNoPlaceName`
// holds at the store.
func TestTheSamplerNamesThePlaceWhoeverIsWatching(t *testing.T) {
	nav := &directorapi.Element{
		ID: "e1", Role: directorapi.RoleListItem, Label: "Bluetooth & devices",
		Confidence: 1, Selected: true, Visible: true,
	}
	world := directorapi.WorldState{
		Elements: map[directorapi.ElementID]*directorapi.Element{nav.ID: nav},
	}

	learnSession := &liveSampler{demonstration: true}
	if got := learnSession.placeName(world); got != "Bluetooth & devices" {
		t.Errorf("a demonstration read the Place as %q", got)
	}
	passive := &liveSampler{}
	if got := passive.placeName(world); got != "Bluetooth & devices" {
		t.Errorf("a passive sampler read the Place as %q — observation answers what is on screen "+
			"regardless of whether anybody is teaching Marco anything", got)
	}
}

// THE TRAIL IS THE SIBLINGS CONTAINING THE SELECTED WORD.
//
// Self-identifying, so nothing here names an application. Measured on the Settings Mouse page:
// two sibling buttons under one parent, `Bluetooth & devices` and `Mouse`, while the rail reports
// `Bluetooth & devices` selected. The group holding the selected word IS the trail.
//
// No geometry: the fused world is a map with no order, and this reads membership only.
func TestTheTrailIsTheSiblingsContainingTheSelectedWord(t *testing.T) {
	crumbs := directorapi.ElementID("crumbs")
	rail := directorapi.ElementID("rail")
	b := func(id, label string, parent directorapi.ElementID) *directorapi.Element {
		p := parent
		return &directorapi.Element{
			ID: directorapi.ElementID(id), Role: directorapi.RoleButton, Label: label,
			ParentID: &p, Visible: true, Confidence: 1,
		}
	}
	world := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{
		"c1": b("c1", "Bluetooth & devices", crumbs),
		"c2": b("c2", "Mouse", crumbs),
		// Buttons elsewhere in the tree are not this trail.
		"x1": b("x1", "Back", rail),
		"x2": b("x2", "Close", rail),
	}}

	got := trailContaining(world.Elements, "Bluetooth & devices")
	if len(got) != 2 {
		t.Fatalf("the trail is %v, want the two siblings that share a parent", got)
	}
	seen := map[string]bool{}
	for _, g := range got {
		seen[g] = true
	}
	if !seen["Bluetooth & devices"] || !seen["Mouse"] {
		t.Errorf("the trail is %v, want both breadcrumb entries", got)
	}
	// A word in no trail has none — Chrome and Discord name a Place from the selection alone.
	if got := trailContaining(world.Elements, "Direct Messages"); got != nil {
		t.Errorf("a word that appears in no trail reported %v", got)
	}
}
