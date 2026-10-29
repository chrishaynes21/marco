package main

// THE PARTS OF THE LIVE FIXTURE THAT ARE NOT ABOUT THE SCREEN.
//
// `groundbykey_test.go` is behind `-tags desktop` because its three tests establish that a
// screen can be named from the window actually in front of somebody. These four were in that
// file and are not about the screen at all: two assemble a saved demonstration out of regions
// somebody else supplied, and two are plumbing. They live here so the stated fixture
// (`anActOverAStatedWindow`) can use them in the default suite, and so the live fixture and the
// stated one cannot drift into saving differently shaped routines.
//
// See [[ADR-269-a-screen-free-gate-and-the-tests-that-need-the-screen]].

import (
	"context"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// theControlShapedRegion is the largest region shaped like a control, out of one look.
//
// NOT the largest region. Measured, on a live Ark window: the largest thing the proposer finds
// spans the full height of the window, and the coordinator refuses it — *the middle of it is
// the middle of the window and not the position of anything in it; this step has no point to
// press and no substitute for one will be invented*. That guard is right and the fixture was
// wrong: a demonstration's control is a control, so the region has to be bounded in both axes
// the way one is.
//
// Among those, the largest, because the largest thing is the likeliest to still be there on the
// next look — these tests are about the sequence, not about how little evidence a
// reacquisition can work from.
//
// Shared with the stated fixture so the two cannot save different shapes of routine. See
// `anActOverAStatedWindow`.
func theControlShapedRegion(regions []observe.Region) (observe.Region, bool) {
	best := observe.Region{}
	for _, r := range regions {
		if r.Width > 0.5 || r.Height > 0.25 || r.Width <= 0 || r.Height <= 0 {
			continue
		}
		if r.Width*r.Height > best.Width*best.Height {
			best = r
		}
	}
	return best, best.Width > 0
}

// anArtifactOver assembles the saved demonstration: one screen, one control, one click.
//
// Shared by the live fixture above and the stated one, which is the whole point of extracting
// it: the stated fixture exists so a test of Director's RUN LOGIC does not need the owner's
// screen, and it would be worthless if it saved a differently shaped routine from the one the
// live path saves.
func anArtifactOver(w directorapi.Window, regions []observe.Region,
	best observe.Region) workflowstore.Artifact {

	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: "the window in front",
		SavedAt:     time.Now().UTC(),
		Application: workflowstore.NormaliseScope(w.Application),
		Screens: []workflowstore.Screen{{Label: "screen_a",
			Arrangement: observe.ArrangeLook(regions)}},
		Controls: []workflowstore.Control{{Label: "control_1",
			Readings: []observe.Region{best}}},
		Steps: []workflowstore.Step{{
			Position: 0, From: "screen_a", To: "screen_a", Control: "control_1",
			Cause: observe.CauseClick, Seen: 1,
		}},
	}
}

// boundsOf is the proposer's rectangles, in the order it found them.
func boundsOf(res structure.Result) []image.Rectangle {
	out := make([]image.Rectangle, 0, len(res.Candidates))
	for _, c := range res.Candidates {
		out = append(out, c.Bounds)
	}
	return out
}

func mustOpenStore(t *testing.T) *demo.Store {
	t.Helper()
	store, err := demo.Open(configDir())
	if err != nil {
		t.Fatalf("opening the store: %v", err)
	}
	return store
}

// AND IT IS SHARED, WHICH IS WHY IT IS NOT IN THE TAGGED FILE.
//
// This was in `foregroundlive_test.go` and moved when that file went behind `-tags desktop`:
// `TestGeometryReplayTakesNoLookAtThePressBoundary` is a claim about Director and asks this
// same question, and it now answers it from the stated desktop rather than the owner's. A
// helper left in the tagged file would have taken that test with it.
// aBackgroundApplication finds a real application with a visible window that is NOT in front.
//
// Returns the image name, or skips. Deliberately not the foreground one: this test is about the
// state where the target has lost the desktop, and taking the foreground to create it would
// interrupt whoever is using the machine.
func aBackgroundApplication(t *testing.T, p *winprovider.Provider) string {
	t.Helper()
	all := p.AllCandidates(context.Background())
	if len(all) == 0 {
		t.Skip("no top-level windows on this machine, so there is no live window " +
			"state to check against")
	}
	front := map[string]bool{}
	for _, c := range all {
		if c.Foreground && c.Application != "" {
			front[strings.ToLower(c.Application)] = true
		}
	}
	for _, c := range all {
		img := strings.ToLower(strings.TrimSpace(c.Application))
		if img == "" || c.Minimized || !c.Visible || front[img] {
			continue
		}
		return img
	}
	t.Skip("every windowed application on this machine is in the foreground, so the " +
		"state this checks does not exist here")
	return ""
}
