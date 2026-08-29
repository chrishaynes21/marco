package main

import (
	"fmt"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// IS THIS GROUP A PATH, OR A ROW OF THINGS?
//
// 37K reduced destination naming to one ambiguity: a selected navigable item with no trail
// containing it means opposite things on two real Windows Settings screens.
//
//	Settings Home        selected `Home`                 -> Home IS the destination
//	Printers at 850px    selected `Bluetooth & devices`  -> the destination is its CHILD,
//	                                                        `Printers & scanners`, whose
//	                                                        breadcrumb ancestor collapsed
//	                                                        into an overflow control
//
// 37L asked whether anything else Marco has could tell them apart. Two candidates were measured
// and both are closed. The fixtures below are the measurements, transcribed from
// `director name-probe --deep` on 2026-08-29.

const pathWin = directorapi.WindowID("win_settings")

// node builds one element of a Settings-shaped tree.
func node(id, parent string, role directorapi.ElementRole, label string) *directorapi.Element {
	e := &directorapi.Element{
		ID: directorapi.ElementID(id), WindowID: pathWin, Role: role, Label: label,
		Bounds:     directorapi.Rect{X: 10, Y: 10, Width: 40, Height: 20},
		Enabled:    true,
		Visible:    true,
		Confidence: 1,
	}
	if parent != "" {
		p := directorapi.ElementID(parent)
		e.ParentID = &p
	}
	return e
}

// settingsFrame is the ancestry every Settings screen shares, measured identically on Home and on
// Printers at 850px:
//
//	selected list_item … in [group(-) pane(-) window(-) unknown(-) window(Settings) window(Settings)]
//
// The rail's selected item hangs off `rail`; the header's buttons hang off `header`, which sits
// outside the content pane.
func settingsFrame() map[directorapi.ElementID]*directorapi.Element {
	out := map[directorapi.ElementID]*directorapi.Element{}
	add := func(e *directorapi.Element) { out[e.ID] = e }
	add(node("win", "", directorapi.RoleWindow, "Settings"))
	add(node("frame", "win", directorapi.RoleUnknown, ""))
	add(node("inner", "frame", directorapi.RoleWindow, ""))
	add(node("content", "inner", directorapi.RolePane, ""))
	add(node("rail", "content", directorapi.RoleGroup, ""))
	add(node("header", "frame", directorapi.RoleGroup, ""))
	return out
}

func evidenceFor(world map[directorapi.ElementID]*directorapi.Element) []observe.PlaceNameEvidence {
	return placeNameEvidence(directorapi.WorldState{Elements: world})
}

// THE HIERARCHY ABOVE THE SELECTION IS THE SAME ON BOTH SCREENS.
//
// The first candidate 37J proposed was the accessibility hierarchy: perhaps what CONTAINS a claim
// says whether it names where you are or the region you are in.
//
// Measured, it does not. `director name-probe --deep` reports the identical chain for both:
//
//	Home @1500     selected list_item "Home"                in [group pane window unknown window window]
//	Printers @850  selected list_item "Bluetooth & devices" in [group pane window unknown window window]
//
// Byte-identical. So this test builds both screens over one frame and asserts that the evidence
// production hands the naming rule is indistinguishable apart from the word — which is why no rule
// reading that evidence can separate them, and why 37L changed nothing.
func TestTheHierarchyAboveTheSelectionDoesNotSayWhetherItIsTheDestination(t *testing.T) {
	home := settingsFrame()
	home[directorapi.ElementID("sel")] = node("sel", "rail", directorapi.RoleListItem, "Home")
	home["sel"].Selected = true

	printers := settingsFrame()
	printers[directorapi.ElementID("sel")] = node("sel", "rail",
		directorapi.RoleListItem, "Bluetooth & devices")
	printers["sel"].Selected = true
	// The header at 850px: the breadcrumb ancestor has become an overflow control, so no
	// group holds the selected word and the trail lookup finds nothing.
	printers[directorapi.ElementID("more")] = node("more", "header", directorapi.RoleButton, "More")
	printers[directorapi.ElementID("leaf")] = node("leaf", "header",
		directorapi.RoleButton, "Printers & scanners")

	shape := func(in []observe.PlaceNameEvidence) []string {
		var out []string
		for _, e := range in {
			out = append(out, fmt.Sprintf("role=%s selected=%v value=%v trail=%d",
				e.Role, e.Selected, e.InsideValueChooser, len(e.Trail)))
		}
		return out
	}
	h, p := shape(evidenceFor(home)), shape(evidenceFor(printers))
	if len(h) != 1 || len(p) != 1 || h[0] != p[0] {
		t.Fatalf("the two screens produce different evidence shapes:\n  home     %v\n  printers %v\n"+
			"If they now differ, the hierarchy HAS started saying which is which and the "+
			"37K ambiguity may be closable — say what changed.", h, p)
	}

	// And the rule therefore answers the same way about both, which is right on one and wrong
	// on the other.
	if got := observe.AdmittedPlaceName(evidenceFor(home)); got != "Home" {
		t.Errorf("Home is called %q, want Home", got)
	}
	if got := observe.AdmittedPlaceName(evidenceFor(printers)); got != "Bluetooth & devices" {
		t.Errorf("the Printers-at-850 shape yields %q; the measured current answer is the "+
			"section name, and this test exists to say so out loud", got)
	}
}

// A PAGE PUBLISHES ITS OWN CHILDREN, SO A KNOWN EDGE TO A VISIBLE LABEL PROVES NOTHING.
//
// The second candidate was the graph: if the selected Place has a remembered edge to a label that
// is currently visible, perhaps the visible label is the destination and the selection is only the
// section.
//
// It is unsafe, and Settings Home is the case that kills it. Measured, the rail on Home publishes
// every one of its children as a visible list item:
//
//	Home, System, Bluetooth & devices, Network & internet, Personalization, Apps, Accounts,
//	Time & language, Gaming, Accessibility, Privacy & security, Windows Update, Sound, Display…
//
// A navigation rail is a list of places you COULD go. It is present on every wide page, so the
// precondition of that rule — "a visible label the selected Place has an edge to" — is satisfied
// everywhere, and the rule would demote every parent to a section, starting with the one screen
// whose name is currently correct.
//
// This test states the fact that makes it unsafe rather than implementing the rejected rule: on a
// Home-shaped screen, the children are right there beside the selection, indistinguishable to
// anything that only knows they are visible.
func TestAPagePublishesItsOwnChildrenSoVisibilityIsNotAPath(t *testing.T) {
	home := settingsFrame()
	home[directorapi.ElementID("sel")] = node("sel", "rail", directorapi.RoleListItem, "Home")
	home["sel"].Selected = true
	children := []string{"System", "Bluetooth & devices", "Network & internet", "Personalization"}
	for i, label := range children {
		id := fmt.Sprintf("rail-%d", i)
		home[directorapi.ElementID(id)] = node(id, "rail", directorapi.RoleListItem, label)
	}

	world := directorapi.WorldState{Elements: home}
	visible := map[string]bool{}
	for _, el := range world.Elements {
		if el != nil && el.Visible && !el.Offscreen && el.Label != "" {
			visible[el.Label] = true
		}
	}
	for _, label := range children {
		if !visible[label] {
			t.Fatalf("%q is not visible on the Home-shaped screen; the fixture no longer "+
				"describes what a navigation rail does", label)
		}
	}

	// The naming rule is not fooled, because it never looks at unselected labels — and that
	// restraint is exactly what a topology rule would have to give up.
	if got := observe.AdmittedPlaceName(evidenceFor(home)); got != "Home" {
		t.Errorf("Home is called %q with its children beside it, want Home.\n"+
			"A rule that read a remembered edge from Home to any of %v as evidence that the "+
			"child is the destination would rename this screen after whichever section Marco "+
			"happened to have learned an edge to.", got, children)
	}
}

// AND MEMORY STAYS OUT OF THE PRODUCER.
//
// The boundary 37L exists to protect: memory may help INTERPRET current evidence and may never
// BECOME current evidence. `placeNameEvidence` reads one fused world and nothing else — no store,
// no topology, no goal, no play, no remembered name.
//
// Adding any of those here would let a screen be named after something that is not on it.
func TestTheNameProducerReadsOnlyTheCurrentWorld(t *testing.T) {
	code := withoutComments(mustReadSource(t, "observewiring.go"))
	// The producer is one function; check the whole file cannot reach memory from it.
	for _, forbidden := range []string{
		"semanticmemory.", "Topology(", "Relationships(", "SubjectNamed(", "Goals(",
		"placesKnown(", "RememberedSubject",
	} {
		if containsAll(code, "func placeNameEvidence(", forbidden) {
			t.Errorf("observewiring.go reaches for %s.\nThe producer of what a screen is "+
				"called must read the current world and nothing else: a remembered Place is "+
				"not a visible Place, and a remembered label is not a visible label.",
				forbidden)
		}
	}
}
