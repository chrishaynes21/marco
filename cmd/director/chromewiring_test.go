package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The chrome classification survives the trip into the shadow sample.
//
// # Why this test is here and not beside the classifier
//
// Because this is the seam where the hierarchy dies. A ShadowRegion carries a role and a
// rectangle and no parent, so the identity projection could never work out for itself that a
// button belongs to a title bar. `shadowSampleFor` is the last place the accessibility hierarchy
// still exists, and if it stops asking, identity silently goes back to counting the frame's own
// buttons — which is exactly the bug that produced three families of twins.
//
// The classifier is proved in internal/director/perception/observation. What is proved HERE is
// that this binary calls it and carries the answer.
//
// Deleting the ChromeIn call, or the Chrome field on the region, must fail this.
func TestTheShadowSampleCarriesTheChromeClassification(t *testing.T) {
	obs := func(id, parent string, role directorapi.ElementRole, w, h int) observation.Observation {
		return observation.NewElement(directorapi.Observation{
			NativeID: id, ParentNativeID: parent, Role: role,
			Bounds: directorapi.Rect{Width: w, Height: h},
			Source: directorapi.SourceAccessibility,
		})
	}
	cycle := observation.Cycle{
		Shadow: []observation.ProviderOutcome{{
			State: observation.StateContributed,
			Observations: []observation.Observation{
				obs("w", "", directorapi.RoleWindow, 800, 600),
				obs("tb", "w", directorapi.RoleTitleBar, 800, 30),
				obs("cls", "tb", directorapi.RoleButton, 0, 0),
				obs("pane", "w", directorapi.RolePane, 800, 570),
				obs("btn", "pane", directorapi.RoleButton, 200, 40),
			},
		}},
	}
	frame := directorapi.Rect{Width: 800, Height: 600}

	s := shadowSampleFor("accessibility", "", cycle, frame)
	if s == nil {
		t.Fatal("no shadow sample was produced")
	}
	if len(s.Regions) == 0 {
		t.Fatal("the sample carries no regions, so this proves nothing")
	}
	var chrome, content int
	for _, r := range s.Regions {
		if r.Chrome {
			chrome++
		} else {
			content++
		}
	}
	if chrome == 0 {
		t.Error("nothing in the sample was classified as chrome.\nThe title bar and its " +
			"caption button are the window's own machinery, and if that answer does not " +
			"reach the region then place identity counts them again.")
	}
	if content == 0 {
		t.Error("everything was classified as chrome; the page has content")
	}
	// And the classification is the CANONICAL one, not a guess made here.
	want := observation.ChromeIn(cycle.Shadow[0].Observations)
	if len(want) != chrome {
		t.Errorf("the sample classified %d region(s) as chrome; the one classifier says %d",
			chrome, len(want))
	}
}

// Raw observation still contains the window's machinery.
//
// The correction is about what decides a PLACE, never about what Marco may see. A person asking
// Marco to press Close must still be able to, and a diagnostic must still be able to show it.
//
// Deleting chrome from the raw observation must fail this.
func TestChromeIsStillPresentInTheRawSample(t *testing.T) {
	obs := func(id, parent string, role directorapi.ElementRole) observation.Observation {
		return observation.NewElement(directorapi.Observation{
			NativeID: id, ParentNativeID: parent, Role: role,
			Bounds: directorapi.Rect{Width: 10, Height: 10},
			Source: directorapi.SourceAccessibility,
		})
	}
	cycle := observation.Cycle{
		Shadow: []observation.ProviderOutcome{{
			State: observation.StateContributed,
			Observations: []observation.Observation{
				obs("w", "", directorapi.RoleWindow),
				obs("tb", "w", directorapi.RoleTitleBar),
				obs("cls", "tb", directorapi.RoleButton),
			},
		}},
	}
	s := shadowSampleFor("accessibility", "", cycle,
		directorapi.Rect{Width: 800, Height: 600})

	var sawTitleBar, sawChromeButton bool
	for _, r := range s.Regions {
		if r.Role == string(directorapi.RoleTitleBar) {
			sawTitleBar = true
		}
		if r.Role == string(directorapi.RoleButton) && r.Chrome {
			sawChromeButton = true
		}
	}
	if !sawTitleBar || !sawChromeButton {
		t.Errorf("the window's machinery was removed from the sample "+
			"(title_bar=%v caption button=%v).\nChrome is labelled, never deleted: Sight "+
			"shows it and a play may still press it.", sawTitleBar, sawChromeButton)
	}
}
