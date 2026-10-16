//go:build corpus

package main

// THE SURFACE HYPOTHESIS, AGAINST FIVE RECORDED INTERFACES.
//
// `internal/director/surface` reasons about whether an explicitly requested click has a
// defensible place to land, and executes nothing. This runs it over the Stage the real collector
// and the real fusion engine build from real recorded pixels, on five unrelated applications,
// and asserts the properties the owner named:
//
//	Trackmania SETTINGS   the text-sized Prop against the 444x56 menu row
//	Trackmania background the 48%-of-window region must refuse
//	Explorer toolbar      must not become defensible on its geometry
//	Settings rows         whether independent structure supports a surface
//	Browser controls      competing candidates
//	Paint                 decorative slivers must not become actionable
//
// Nothing here presses anything. The hypothesis is a value; the test reads it.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/surface"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// hypothesisFor runs the evaluator the way Director would, on a Stage built from a recording.
func hypothesisFor(t *testing.T, w directorapi.WorldState, words string) surface.Hypothesis {
	t.Helper()
	h := surface.New().Evaluate(&w, "click "+words, "click",
		directorapi.ElementQuery{Text: words})
	t.Logf("── %q → %s", "click "+words, h.Decision)
	t.Logf("   %s", h.Explanation)
	if h.Unsettled != "" {
		t.Logf("   unsettled: %s", h.Unsettled)
	}
	if h.Identity != nil {
		t.Logf("   identity %s %q %v score %.3f sources %v rejected=%q",
			h.Identity.ElementID, h.Identity.Label, h.Identity.Bounds,
			h.Identity.Score, h.Identity.Sources, h.Identity.Rejected)
	}
	for _, c := range h.Candidates {
		e := c.Evidence
		verdict := "DEFENSIBLE"
		if !c.Defensible() {
			verdict = "refused: " + c.Rejected
		}
		t.Logf("   %s %v point %v  cov %.3f anc=%v self=%v obj=%v loc=%v "+
			"area %.1f%% kind=%s fill=%.2f parts=%d sib=%d/%d named-inside=%d",
			c.ElementID, c.Bounds, c.Point, e.Containment, e.Ancestor, e.IsIdentity,
			e.DescribesAnObject, e.Locates, 100*e.AreaShare, e.StructureKind,
			e.Fill, e.Parts, e.Siblings, e.DistinctTexts, len(e.OtherNamed))
		t.Logf("      %s", verdict)
	}
	return h
}

// THE CASE THE MILESTONE IS ABOUT.
func TestTheSurfaceHypothesisOnARecordedGameMenu(t *testing.T) {
	frame := loadFrame(t, filepath.Join(tmRecording, "frames", "tm-menu-steps-0100.png"))
	world, _ := stageFrom(t, frame, true)

	h := hypothesisFor(t, world, "settings")

	if h.Identity == nil {
		t.Fatal("SETTINGS was not identified at all; the measurement says it is on " +
			"this Stage")
	}
	// WHATEVER IT DECIDES, IT MAY NOT HAVE CHOSEN A SURFACE THAT STANDS FOR THE SCREEN.
	for _, c := range h.Candidates {
		if c.Defensible() && c.Evidence.AreaShare > 0.25 {
			t.Errorf("a surface covering %.0f%% of the window is defensible: %s %v",
				100*c.Evidence.AreaShare, c.ElementID, c.Bounds)
		}
	}
	if h.Chosen != nil {
		t.Logf("CHOSEN %s %v, press at %v", h.Chosen.ElementID, h.Chosen.Bounds,
			h.Chosen.Point)
		if h.Chosen.Evidence.AreaShare > 0.25 {
			t.Error("the chosen surface stands for a quarter of the window")
		}
	}

	// AND THE OVERSIZED BACKGROUND, ASKED FOR DIRECTLY. It carries a name of its own on
	// this frame — OPPONENTS, at 0.99 confidence — which makes it the strongest reading of
	// the safety test: a NAMED 48% region is more dangerous than a nameless one, because
	// every rule keyed on having a name admits it.
	big := hypothesisFor(t, world, "opponents")
	if big.Decision == surface.DecisionDefensible {
		t.Errorf("the oversized background produced a defensible surface: %+v",
			big.Chosen)
	}
}

// A SECOND INTERFACE, WHERE A BAND OF CHROME IS THE TEMPTING ANSWER.
func TestTheSurfaceHypothesisOnARecordedToolbar(t *testing.T) {
	frame := loadFrame(t, `../../.tmp/desktop-corpus-review/explorer-fixture-wide/`+
		`explorer-fixture-wide.png`)
	world, _ := stageFrom(t, frame, false)

	h := hypothesisFor(t, world, "details")
	for _, c := range h.Candidates {
		// The toolbar band is 1084x44 of a 1092-wide window. If it is ever defensible,
		// this logic is not ready — the owner's words.
		if c.Defensible() && c.Bounds.Width > 1000 {
			t.Errorf("the toolbar band is a defensible surface: %s %v",
				c.ElementID, c.Bounds)
		}
	}
	if h.Chosen != nil && h.Chosen.Bounds.Width > 1000 {
		t.Error("a band across the window was chosen as the place to press")
	}
}

// AND THREE MORE, FOR WHAT THEY SAY RATHER THAN FOR WHAT THEY ASSERT.
func TestTheSurfaceHypothesisOnThreeMoreInterfaces(t *testing.T) {
	for _, tc := range []struct {
		name  string
		path  string
		words []string
	}{
		{"settings", `../../.tmp/desktop-corpus-review/settings-mouse-wide/` +
			`settings-mouse-wide.png`,
			[]string{"bluetooth", "mouse", "pointer"}},
		{"browser", `../../.tmp/desktop-corpus-review/browser-fixture-wide/` +
			`browser-fixture-wide.png`,
			[]string{"storage", "about", "check"}},
		{"paint", `../../.tmp/37d/custom-canvas-paint/custom-canvas-paint.png`,
			[]string{"brush", "fill"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame := loadFrame(t, tc.path)
			world, _ := stageFrom(t, frame, false)
			for _, words := range tc.words {
				h := hypothesisFor(t, world, words)
				// THE ONE PROPERTY ASSERTED EVERYWHERE: a chosen surface is
				// never a band across the window and never a quarter of it.
				if h.Chosen == nil {
					continue
				}
				if h.Chosen.Evidence.AreaShare > 0.25 {
					t.Errorf("%s/%s: chose a surface covering %.0f%% of the "+
						"window", tc.name, words,
						100*h.Chosen.Evidence.AreaShare)
				}
				if !h.Chosen.Evidence.Locates {
					t.Errorf("%s/%s: chose a surface that locates nothing",
						tc.name, words)
				}
				if !h.Chosen.Evidence.DescribesAnObject {
					t.Errorf("%s/%s: chose a surface only read text accounts for",
						tc.name, words)
				}
			}
		})
	}
}

// AND THE HYPOTHESIS IS READ-ONLY, asserted rather than assumed.
//
// Nothing it looks at may come back changed. A Prop whose role or label the evaluator had
// quietly adjusted would be a perception change wearing a reasoning change's clothes, and the
// whole premise of [[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] is that
// the Stage claims nothing.
func TestTheSurfaceHypothesisChangesNothingItLooksAt(t *testing.T) {
	frame := loadFrame(t, filepath.Join(tmRecording, "frames", "tm-menu-steps-0100.png"))
	world, _ := stageFrom(t, frame, true)

	type snapshot struct {
		role  directorapi.ElementRole
		label string
		b     directorapi.Rect
		conf  float64
		srcs  int
	}
	before := map[directorapi.ElementID]snapshot{}
	for id, el := range world.Elements {
		before[id] = snapshot{el.Role, el.Label, el.Bounds, el.Confidence,
			len(el.Provenance.Sources)}
	}

	for _, words := range []string{"settings", "scoreboard", "opponents", "resume"} {
		surface.New().Evaluate(&world, "click "+words, "click",
			directorapi.ElementQuery{Text: words})
	}

	if len(world.Elements) != len(before) {
		t.Fatalf("the Stage went from %d Props to %d", len(before), len(world.Elements))
	}
	for id, el := range world.Elements {
		was := before[id]
		if el.Role != was.role || el.Label != was.label || el.Bounds != was.b ||
			el.Confidence != was.conf || len(el.Provenance.Sources) != was.srcs {
			t.Errorf("%s changed: role %q→%q label %q→%q bounds %v→%v "+
				"conf %.3f→%.3f sources %d→%d", id, was.role, el.Role,
				was.label, el.Label, was.b, el.Bounds, was.conf, el.Confidence,
				was.srcs, len(el.Provenance.Sources))
		}
	}
	_ = context.Background()
}
