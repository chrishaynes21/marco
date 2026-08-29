package main

import (
	"strings"
	"testing"
)

// THE PROBE ASKS PRODUCTION. IT DOES NOT PARSE.
//
// A naming probe exists to answer "why did Marco call this screen that", and a probe with a rule
// of its own answers a question about itself. Worse, it would agree with production everywhere
// except the one place somebody was using it to look.
//
// So it calls `placeNameEvidence` — the production producer, which is where selection, parentage
// and the value-chooser walk live — and `observe.ExplainPlaceName`, the one rule. Everything it
// prints about a NAME comes back from those two.
//
// `Navigable` is deliberately not on the forbidden list: the probe reads it to report the
// PRESENTATION, which is a different question, and the test below requires it for that.
func TestTheNameProbeUsesTheProductionNamingPath(t *testing.T) {
	src := mustReadSource(t, "nameprobe.go")
	if !containsAll(src, "placeNameEvidence(world)", "observe.ExplainPlaceName(") {
		t.Error("nameprobe.go no longer reaches the production producer and the production " +
			"rule.\nA probe that derived a name itself would be a second naming policy, and " +
			"the one it is used to debug would be the other one.")
	}
	code := withoutComments(src)
	for _, forbidden := range []string{
		"safeLabelText", "InsideValueChooser", "MaxTargetLabelLength",
		"LevelDestination", "LevelSection", "e.Trail",
	} {
		if strings.Contains(code, forbidden) {
			t.Errorf("nameprobe.go reaches for %s. Deciding what a name means is the rule's "+
				"job; this file collects a world and prints what the rule made of it.",
				forbidden)
		}
	}
	// And it is reachable, or the measurement it exists for cannot be taken.
	if !strings.Contains(mustReadSource(t, "main.go"), `case "name-probe":`) {
		t.Error("`director name-probe` is not registered in main.go")
	}
}

// THE PRESENTATION IS REPORTED FROM THE INTERFACE, NOT FROM THE WINDOW.
//
// Windows moves its responsive breakpoints with DPI and font scaling, so a pixel width describes
// one machine — and 37J's acceptance turned on being able to say the navigation was gone rather
// than that the window was narrow. A label like `Open Navigation` would be equally wrong in a
// different direction: it names one operating system in one language.
//
// What the probe reports instead is whether anything on screen claims to be the selected
// destination, which is the same evidence the naming rule consumes, so the two lines explain
// each other.
func TestTheProbeDoesNotDecidePresentationFromGeometryOrLanguage(t *testing.T) {
	code := withoutComments(mustReadSource(t, "nameprobe.go"))
	for _, forbidden := range []string{
		"Open Navigation", "Expand search", "Width <", "Width >", "Bounds.Width >",
	} {
		if strings.Contains(code, forbidden) {
			t.Errorf("nameprobe.go decides something from %q. The presentation has to be read "+
				"from what the interface exposes, or the measurement describes this machine.",
				forbidden)
		}
	}
	if !strings.Contains(code, "el.Selected") || !strings.Contains(code, "Navigable()") {
		t.Error("navigationState no longer reads whether anything reports itself as the " +
			"selected destination, which is the whole of what it claims to report")
	}
}
