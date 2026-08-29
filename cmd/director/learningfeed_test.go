package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// THE FEED IS FED BY THE STORE, AND BY NOTHING ELSE.
//
// 37E, 37F, 37G and 37H each found a correct mechanism wired to nothing or wired to the wrong
// caller. A learning feed is the worst possible place for that to happen again: it makes a claim
// directly to a person — "I learned this, go and try it" — and a feed wired to an intention rather
// than to a commit would be confidently wrong at exactly the moment somebody trusted it.
func TestTheFeedIsFedByTheStoresOwnCommits(t *testing.T) {
	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening: %s", why)
	}
	rt := &Runtime{}
	rt.watchLearning(store)

	sig := observe.StructureSignature{
		Subject: observe.SubjectState, TermsKnown: true,
		Terms: []observe.InterfaceTerm{observe.TermControls},
		Roles: map[string]int{"button": 4, "slider": 2},
	}
	id, err := store.EstablishPlace("settings", sig)
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}

	view := rt.LearningSince(service.ObserveLearning{})
	if len(view.Events) != 1 {
		t.Fatalf("the Director reports %d changes after one Place was committed: %+v\n"+
			"Nothing between the store and this feed may decide what counts as learning.",
			len(view.Events), view.Events)
	}
	got := view.Events[0]
	if got.Change != "learned" || got.Kind != "place" {
		t.Errorf("the event is %s/%s, want learned/place", got.Change, got.Kind)
	}
	if view.Newest == 0 {
		t.Error("the cursor did not advance, so a follower would print the same event forever")
	}
	// The description renders the subject, and says plainly that it has no name — which is a
	// real outcome on a collapsed presentation and the thing this dogfood session is watching
	// for. Hiding it would hide the finding.
	if !strings.Contains(got.Description, "unnamed") && !strings.Contains(got.Description, id[:8]) {
		t.Errorf("an unnamed Place rendered as %q; it should say so and show its subject",
			got.Description)
	}

	// AND THE CURSOR IS HONEST. Asking again from where we left off reports nothing, rather
	// than replaying what somebody has already been told.
	if again := rt.LearningSince(service.ObserveLearning{After: view.Newest}); len(again.Events) != 0 {
		t.Errorf("%d change(s) replayed after the cursor advanced", len(again.Events))
	}
}

// A NAME ARRIVING IS RENDERED FROM THE STORE AS IT IS NOW.
//
// A Place is established on one pass and named on a later one. If the feed had captured the name
// at the moment of the write it would say `[unnamed]` forever about a Place that is now perfectly
// well named — and a person reading back through the session would be told something false.
func TestAPlaceNamedLaterRendersWithItsName(t *testing.T) {
	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening: %s", why)
	}
	rt := &Runtime{}
	rt.watchLearning(store)

	sig := observe.StructureSignature{
		Subject: observe.SubjectState, TermsKnown: true,
		Terms: []observe.InterfaceTerm{observe.TermAudio},
		Roles: map[string]int{"button": 9, "list": 3},
	}
	id, err := store.EstablishPlace("settings", sig)
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	if err := store.ObserveSemanticName("settings", id, "Mouse", observe.FromStructure); err != nil {
		t.Fatalf("naming: %v", err)
	}

	view := rt.LearningSince(service.ObserveLearning{})
	if len(view.Events) != 2 {
		t.Fatalf("want the establishment and the naming, got %d: %+v", len(view.Events), view.Events)
	}
	// The FIRST event — recorded before the name existed — now renders with it.
	if !strings.Contains(view.Events[0].Description, "Mouse") {
		t.Errorf("the Place's own event still renders as %q after it was named; names are "+
			"meant to be resolved when somebody looks, not when the write happened",
			view.Events[0].Description)
	}
	if view.Events[1].Change != "named" {
		t.Errorf("the second event is %q, want named", view.Events[1].Change)
	}
}

// THE FEED IS WIRED WHERE THE STORE IS OPENED.
//
// A Director with durable memory and no feed is the failure this project has now made four times
// in a row: the mechanism is right and nothing calls it. There is one place the store comes into
// existence, and the feed is attached there.
func TestTheDirectorWiresItsFeedWhereItOpensItsMemory(t *testing.T) {
	src := mustReadSource(t, "runtime.go")
	if !containsAll(src, "semanticmemory.Open(semanticMemoryPath())", "rt.watchLearning(") {
		t.Error("runtime.go opens the semantic store without attaching the learning feed.\n" +
			"Then `marco observe --follow` prints nothing, forever, and looks like a Marco " +
			"that never learns anything.")
	}
}
