package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Carrying out something Marco has learned.
//
// # What this vertical is
//
// A learned play was demonstrated once, verified edge by edge, written down as legal Marco,
// registered and resolvable — and nothing in the system would walk it. `marco do` has no
// perception and refused at the play's own first line; `director execute` ignored the play and did
// a one-shot semantic lookup; `director reach` could only plan, and planned from where a finished
// session happened to end.
//
// So execution is its own path, above the same walker rehearsal uses, entered under a different
// authority. These hold the parts that were measured wrong live.

// A COLD PROCESS FINDS ITS WINDOW ON THE DESKTOP.
//
// # Why this is the difference between durable and cached
//
// Both the application and the window used to come from the newest finished session. That makes a
// restart look durable while actually meaning "Marco can run this if it happens to have observed
// the application in THIS process" — a warm cache wearing durability's clothes.
//
// After a restart there are no sessions. The Audience's phrase names a goal; durable memory knows
// which application that goal lives in; the desktop knows where that application's window is. None
// of that needs history.
//
// Deleting the search across applications must fail this. The live branch of performSelector does
// NOT — measured: this fixture is cold, so history is empty either way and the refusal is
// unchanged. TestTheWindowComesFromTheDesktopNotAPreviousSession is the one that holds it.
func TestAColdProcessFindsItsWindowOnTheDesktop(t *testing.T) {
	dir := t.TempDir()
	store, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	home, err := store.EstablishPlace("settings", namedPlace(observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	if err := store.RememberGoal("settings", observe.Goal{
		Name: "Open Mouse Settings", Application: "settings", Subject: home, Demonstrations: 1,
	}); err != nil {
		t.Fatalf("remembering the goal: %v", err)
	}

	// A COLD registry: nothing observed, no sessions, exactly as after a restart.
	rt := &Runtime{observations: newObservationRegistry().withMemory(store)}
	if id := rt.observations.ActiveID(); id != "" {
		t.Fatal("the fixture is not cold")
	}

	// The goal is found with no application supplied and no session to ask.
	goals, ok := any(store).(observe.GoalStore)
	if !ok {
		t.Fatal("the store keeps no goals")
	}
	apps := rt.applicationsWithGoals(store, goals, "")
	var found bool
	for _, a := range apps {
		if a == "settings" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a cold process cannot tell which application %q belongs to (saw %v). It "+
			"then asks the newest session, of which there are none, and a restarted Marco "+
			"can run nothing it learned.", "Open Mouse Settings", apps)
	}

	// And performing reports an honest refusal rather than "nothing has been observed yet":
	// the goal is known, the desktop is what it is.
	v, err := rt.PerformGoal(context.Background(), service.PerformQuery{Name: "Open Mouse Settings"})
	if err != nil {
		t.Fatalf("performing: %v", err)
	}
	if strings.Contains(strings.ToLower(v.Say), "haven't learned") {
		t.Errorf("a cold process says it never learned the outcome: %q", v.Say)
	}
	if v.Application != "settings" {
		t.Errorf("the outcome was attributed to %q, want settings", v.Application)
	}
}

// AN UNKNOWN OUTCOME IS REFUSED, NOT ATTEMPTED.
func TestAnUnlearnedOutcomeIsRefused(t *testing.T) {
	dir := t.TempDir()
	store, _ := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	rt := &Runtime{observations: newObservationRegistry().withMemory(store)}

	v, err := rt.PerformGoal(context.Background(), service.PerformQuery{Name: "Do Something Marco Never Learned"})
	if err != nil {
		t.Fatalf("performing: %v", err)
	}
	if v.Refusal != "not_learned" {
		t.Errorf("an unlearned outcome refused with %q, want not_learned", v.Refusal)
	}
	if len(v.Steps) != 0 {
		t.Errorf("%d step(s) were attempted for something Marco never learned", len(v.Steps))
	}
}

// settingsInFront is a desktop with the learned application in front, and nothing else.
func settingsInFront() *fakeDesktop {
	return &fakeDesktop{front: windowref.Candidate{
		ID: "window_9", Handle: 0x9999, ProcessID: 7512, Application: "settings",
		Title: "Settings", Foreground: true, Visible: true, OnScreen: true,
		Bounds: directorapi.Rect{Width: 1600, Height: 900},
	}}
}

// THE WINDOW COMES FROM THE DESKTOP, NOT FROM A PREVIOUS SESSION.
//
// # Why the precedence matters rather than the outcome
//
// Both answers can be non-empty at once — a Director that has observed the application before AND
// a live desktop — and taking the historical one makes restart durability fake. It quietly means
// "Marco can run this if it happened to observe the application in this process".
//
// So this seeds BOTH: a finished session carrying one window, and a live desktop showing another.
// The live one must win. The historical one remains available for when the desktop cannot be read,
// which is history being used as history.
//
// Deleting the live branch of performSelector must fail this.
func TestTheWindowComesFromTheDesktopNotAPreviousSession(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry()}
	rt.winPlatform = settingsInFront()
	rt.winDirectory = windowref.NewDirectory()

	// History says one thing…
	rt.observations.finished = []observesession.Result{{
		Session: observe.Session{
			ID: "observe_1", Application: "settings",
			Selector: windowref.Selector{EphemeralID: "window_from_history"},
		},
	}}

	got := rt.performSelector(context.Background(), "settings")
	if got.EphemeralID == "window_from_history" {
		t.Fatal("the window came from a finished session while the desktop could answer. " +
			"That makes a restart look durable when it is really a warm cache.")
	}
	if got.Validate() != nil {
		t.Errorf("no usable window was derived from the desktop: %+v", got)
	}

	// …and with no desktop at all, history is an honest fallback rather than nothing.
	rt.winPlatform = nil
	if fell := rt.performSelector(context.Background(), "settings"); fell.EphemeralID != "window_from_history" {
		t.Errorf("with no desktop to read, the known window was not used as history: %+v", fell)
	}
}
