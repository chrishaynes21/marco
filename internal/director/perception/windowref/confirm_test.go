package windowref_test

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Confirm asks a question and changes nothing.
//
// It exists for the provenance guard, which needs to know whether the window some evidence
// claims to describe is still the window being tracked. Acquire cannot answer that: it
// repairs what it finds broken, and a repair performed while checking would erase the very
// staleness being checked for.

func trackerOn(d *desktop, app string) (*windowref.Tracker, windowref.Ref) {
	t := windowref.NewTracker(d)
	v := t.Acquire(context.Background(), app)
	if !v.State.OK() {
		panic("the fixture could not acquire its own window: " + v.Reason)
	}
	return t, v.Ref
}

func TestConfirmReportsTheHeldReferenceWhenNothingChanged(t *testing.T) {
	d := newDesktop()
	d.open(100, 42, "notepad", directorapi.Rect{X: 0, Y: 0, Width: 800, Height: 600})
	tracker, held := trackerOn(d, "notepad")

	got, ok := tracker.Confirm(context.Background())
	if !ok {
		t.Fatal("a window that is still open and still owned did not confirm")
	}
	if got.ID != held.ID || got.Generation != held.Generation {
		t.Errorf("confirmed %s gen %d, want %s gen %d",
			got.ID, got.Generation, held.ID, held.Generation)
	}
}

func TestConfirmRefusesAWindowThatHasGone(t *testing.T) {
	// The case the guard is built on: evidence arrives claiming a window that has since
	// been destroyed. There is nothing to attribute it to.
	d := newDesktop()
	d.open(100, 42, "notepad", directorapi.Rect{X: 0, Y: 0, Width: 800, Height: 600})
	tracker, _ := trackerOn(d, "notepad")

	d.close(100)

	if _, ok := tracker.Confirm(context.Background()); ok {
		t.Fatal("a destroyed window confirmed; evidence about it would be attributed to " +
			"the live target")
	}
}

func TestConfirmRefusesARecycledHandle(t *testing.T) {
	// Same handle, different process. The signature of the incident this whole package
	// exists for, and the reason a handle is never an identity.
	d := newDesktop()
	d.open(100, 42, "notepad", directorapi.Rect{X: 0, Y: 0, Width: 800, Height: 600})
	tracker, _ := trackerOn(d, "notepad")

	d.close(100)
	d.open(100, 99, "calculator", directorapi.Rect{X: 0, Y: 0, Width: 400, Height: 300})

	if _, ok := tracker.Confirm(context.Background()); ok {
		t.Fatal("a handle now owned by another process confirmed as the tracked window")
	}
}

func TestConfirmDoesNotReacquireOrAdvanceTheGeneration(t *testing.T) {
	// The defining property, and the reason this is not just a call to Acquire.
	//
	// Acquire would notice the window had gone, find the application's new window, adopt
	// it, and report a healthy generation 2. Every one of those steps is correct when you
	// are about to capture and catastrophic when you are checking somebody's evidence:
	// the guard would ask "is this stale?", the act of asking would make it current, and
	// the answer would be no.
	d := newDesktop()
	d.open(100, 42, "notepad", directorapi.Rect{X: 0, Y: 0, Width: 800, Height: 600})
	tracker, _ := trackerOn(d, "notepad")
	before := tracker.Generation()

	// The window is replaced by another of the SAME application — the case a name
	// comparison cannot see.
	d.close(100)
	d.open(200, 42, "notepad", directorapi.Rect{X: 10, Y: 10, Width: 800, Height: 600})

	if _, ok := tracker.Confirm(context.Background()); ok {
		t.Fatal("Confirm silently adopted the replacement window")
	}
	if after := tracker.Generation(); after != before {
		t.Errorf("the generation moved %d → %d during a read-only check; the staleness "+
			"being tested for would disappear at the moment of testing", before, after)
	}
	if cur, held := tracker.Current(); !held || cur.Handle != 100 {
		t.Error("Confirm changed which reference is held")
	}
}

func TestConfirmRefusesWhenNothingIsHeld(t *testing.T) {
	// No target means no target to belong to. Not an error, and not agreement either.
	tracker := windowref.NewTracker(newDesktop())
	if _, ok := tracker.Confirm(context.Background()); ok {
		t.Fatal("a tracker holding nothing confirmed something")
	}
}
