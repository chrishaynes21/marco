package providers

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The semantic-perception side of the same invariant.
//
// Two independent pipelines carry windows: this one becomes WorldState, and the candidate listing
// becomes what a person can target. They are filtered separately on purpose — the invariant is
// "Marco's own surfaces never enter Marco's semantic desktop", and an invariant enforced in only
// one of two producers is enforced nowhere.

// ownedSource is a window source that knows which handles are Marco's.
type ownedSource struct{ owned map[directorapi.WindowID]bool }

func (ownedSource) Monitors(context.Context) ([]directorapi.Monitor, error) { return nil, nil }

// Enrich is the identity, and stays ONE-TO-ONE. See the note on withoutOwnedSurfaces: the caller
// maps these back by position, so a source that dropped one here would mislabel the rest.
func (ownedSource) Enrich(w []directorapi.Window) []directorapi.Window { return w }

func (o ownedSource) Owned(id directorapi.WindowID) bool { return o.owned[id] }

func windowObs(id, title string) observation.Window {
	return observation.Window{
		ObservationID: directorapi.ObservationID("win:" + id),
		From:          directorapi.SourceAccessibility,
		Detail:        directorapi.Window{ID: directorapi.WindowID(id), Title: title},
	}
}

// An owned surface never becomes part of the semantic world.
func TestAMarcoOwnedSurfaceNeverReachesTheEvidence(t *testing.T) {
	src := ownedSource{owned: map[directorapi.WindowID]bool{"hwnd:2": true}}
	obs := []observation.Observation{
		windowObs("hwnd:1", "Downloads"),
		windowObs("hwnd:2", "marco grounding probe"),
		windowObs("hwnd:3", "Marco - Marco Director - Google Chrome"),
	}
	got := withoutOwnedSurfaces(obs, src)
	if len(got) != 2 {
		t.Fatalf("%d observations survived, want 2", len(got))
	}
	for _, o := range got {
		if w, ok := o.(observation.Window); ok && w.Detail.ID == "hwnd:2" {
			t.Error("a Marco-owned surface reached the semantic world")
		}
	}
}

// THE positional regression. A, owned M, B — and A must still be A afterwards.
//
// This is the bug class that made filtering inside `Enrich` wrong: results are mapped back onto
// observations by index, so a dropped entry silently attaches one window's metadata to another.
// Keep this test even if the filter moves.
func TestFilteringAnOwnedSurfaceDoesNotShiftItsNeighbours(t *testing.T) {
	src := ownedSource{owned: map[directorapi.WindowID]bool{"hwnd:M": true}}
	got := withoutOwnedSurfaces([]observation.Observation{
		windowObs("hwnd:A", "A"),
		windowObs("hwnd:M", "ours"),
		windowObs("hwnd:B", "B"),
	}, src)

	if len(got) != 2 {
		t.Fatalf("%d survived, want 2", len(got))
	}
	for _, o := range got {
		w := o.(observation.Window)
		if string(w.Detail.ID) != "hwnd:"+w.Detail.Title {
			t.Errorf("%s carries %q; metadata shifted onto the wrong window",
				w.Detail.ID, w.Detail.Title)
		}
	}
}

// Non-window evidence from the same cycle is untouched.
//
// Ownership excludes the SURFACE, not whatever else happened to be observed alongside it.
func TestExcludingASurfaceKeepsTheRestOfTheCycle(t *testing.T) {
	src := ownedSource{owned: map[directorapi.WindowID]bool{"hwnd:2": true}}
	app := observation.Application{
		ObservationID: "app:1", Detail: directorapi.Application{ID: "explorer"}, Active: true,
	}
	got := withoutOwnedSurfaces([]observation.Observation{
		windowObs("hwnd:1", "Downloads"), windowObs("hwnd:2", "ours"), app,
	}, src)
	var sawApp bool
	for _, o := range got {
		if _, ok := o.(observation.Application); ok {
			sawApp = true
		}
	}
	if !sawApp {
		t.Error("unrelated evidence was removed because it shared a cycle with our surface")
	}
}

// A source that cannot answer ownership changes nothing.
func TestASourceThatCannotReportOwnershipRemovesNothing(t *testing.T) {
	obs := []observation.Observation{windowObs("hwnd:1", "a"), windowObs("hwnd:2", "b")}
	if got := withoutOwnedSurfaces(obs, blindSource{}); len(got) != len(obs) {
		t.Fatalf("a source with no ownership opinion removed %d", len(obs)-len(got))
	}
}

// blindSource implements WindowSource and nothing else.
type blindSource struct{}

func (blindSource) Monitors(context.Context) ([]directorapi.Monitor, error) { return nil, nil }
func (blindSource) Enrich(w []directorapi.Window) []directorapi.Window      { return w }

// THE wiring test: the exclusion is reached through the production refiner.
//
// The tests above prove the RULE. This proves it is plugged in — and it exists because the
// mutation that deletes the call from `Refine` survived everything else: a test that calls the
// filter directly cannot notice that nobody calls it.
func TestTheRefinerActuallyExcludesOwnedSurfaces(t *testing.T) {
	src := ownedSource{owned: map[directorapi.WindowID]bool{"hwnd:2": true}}
	ws := NewWindowSystem(src)

	got := ws.Refine([]observation.Observation{
		windowObs("hwnd:1", "Downloads"),
		windowObs("hwnd:2", "marco grounding probe"),
	})
	if len(got) != 1 {
		t.Fatalf("the refiner returned %d observations, want 1", len(got))
	}
	if w := got[0].(observation.Window); w.Detail.ID != "hwnd:1" {
		t.Errorf("the refiner kept %s", w.Detail.ID)
	}
}
