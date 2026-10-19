package main

// THE WORLD IS LOOKED AT AGAIN BEFORE A CLICK, NOT REMEMBERED.
//
// The observation loop captures, proposes and observes once a tick, and then calls `Advance` —
// which BLOCKS while somebody is asked to authorise. Nothing looks during that wait, so asking
// the observer what is standing returned the stamps the plan was made from and the check
// compared the plan against itself.
//
// What that misses is the case it most needs to catch: an application changes a panel inside
// the same window, with the same geometry, still in front. Window validity and frontness are
// refreshed, all three terms agree, and the approved point is now on a different interface.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// TestAPanelThatChangedDuringAuthorisationRefusesThePress.
//
// Driven through `worldMoved`, which is where the decision lives, with the stamps a FRESH look
// would produce: same window, same application, same frontness, different screen. The press
// must be refused.
func TestAPanelThatChangedDuringAuthorisationRefusesThePress(t *testing.T) {
	planned := workflowrun.Sighting{
		Epoch: 7, Generation: 2, Screen: "visual_1",
		Window: "hwnd:1", Front: true,
	}
	// WHAT A LOOK TAKEN NOW SAYS: the panel changed, so the observer is standing on a
	// different screen. Everything else is identical, which is the whole difficulty.
	now := worldNow{
		Alive: true, State: "valid", Application: "arkascended",
		Generation: 2, Epoch: 7, Screen: "visual_2",
		Front: observe.FrontTarget,
	}
	ok, why := worldMoved("arkascended", workflowstore.NormaliseScope("arkascended"),
		now, planned)
	if ok {
		t.Fatal("a press was cleared onto a panel that changed while the person " +
			"was being asked to authorise it")
	}
	if why == "" {
		t.Error("the refusal does not say what moved")
	}
}

// TestAnUnchangedWorldStillClears, so the fresh look does not refuse everything.
//
// A check that refused on every look would be safe and useless, and nothing else in this file
// would notice: every other case here asserts a refusal.
func TestAnUnchangedWorldStillClears(t *testing.T) {
	planned := workflowrun.Sighting{
		Epoch: 7, Generation: 2, Screen: "visual_1",
		Window: "hwnd:1", Front: true,
	}
	now := worldNow{
		Alive: true, State: "valid", Application: "arkascended",
		Generation: 2, Epoch: 7, Screen: "visual_1",
		Front: observe.FrontTarget,
	}
	if ok, why := worldMoved("arkascended",
		workflowstore.NormaliseScope("arkascended"), now, planned); !ok {
		t.Fatalf("an unchanged world refused the press: %s", why)
	}
}
