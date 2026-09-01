package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// Observe is where somebody watches Marco build a map of their computer.
//
// The dogfood finding: a person could not tell what Marco thought the screen was called, what it
// had just discovered, how Places related, or what any of it would let Marco do. These hold the
// three claims the map makes and the one it must never make.

// THE MARKER COMES FROM PERCEPTION, AND NEVER FROM MEMORY.
//
// A remembered Place is not a visible Place. A "you are here" that moved because Marco recalled
// something would be telling somebody where they are on the strength of where they once were —
// and a map is exactly the surface where that mistake is invisible, because it looks like
// knowledge.
//
// Unknown is a valid, ordinary answer and the map says it rather than guessing.
//
// Deleting the perception source must fail this.
func TestTheMapMarkerComesFromPerception(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	// A GRAPH WITH PLACES IN IT, and nothing watching.
	from, to := establishTwo(t, store)
	if err := store.RememberWatched(watchedFor(from, to, "Mouse", 2)); err != nil {
		t.Fatalf("recording: %v", err)
	}
	v := rt.Map(service.ObserveMap{Application: recentApp})
	if v.Here != "" || v.HereKnown {
		t.Fatalf("the map placed the marker on %q with nothing watching. Memory is not "+
			"perception, and a map that says where you are from what it remembers is "+
			"the most convincing way to be wrong.", v.Here)
	}
	if v.HereWords != observe.Unnamed {
		t.Errorf("an unknown current place reads as %q", v.HereWords)
	}
	// AND THE NEIGHBOURHOOD IS EMPTY WITHOUT A MARKER, rather than the whole graph. A map
	// with no "you are here" drawn over every place Marco has ever seen answers a question
	// nobody asked.
	if len(v.Places) != 0 || len(v.Edges) != 0 {
		t.Errorf("the map drew %d place(s) and %d edge(s) around a place it cannot find",
			len(v.Places), len(v.Edges))
	}
	// The whole graph is still reported as counts, so growth is visible without drawing it.
	if v.KnownPlaces == 0 {
		t.Error("the map reports no places at all, so somebody could not watch it grow")
	}
}

// THE MAP IS THE CANONICAL GRAPH, NAMED BY THE CANONICAL FUNCTION.
//
// Places, connections and words all come from semantic memory through `observe.PlaceWords`. A map
// that named a screen itself would be a second opinion about what a place is called, which is
// what two dogfood sessions were spent removing.
//
// Deleting the topology read, or naming a place here, must fail this.
func TestTheMapDrawsTheCanonicalGraph(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	here := watchNow(t, g, store, "testgame")
	other, err := store.EstablishPlace("testgame",
		screenLike(9, observe.TermSettings, observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	if _, err := store.RememberRelationships("testgame", []observe.RelationshipObservation{{
		From: here, To: other, Evidence: observe.RelationshipEvidence{Observations: 1},
	}}); err != nil {
		t.Fatalf("remembering the relationship: %v", err)
	}

	v := rt.Map(service.ObserveMap{Application: "testgame"})
	if v.Here != here {
		t.Fatalf("the map places you at %q, want %q", v.Here, here)
	}
	if !v.HereKnown {
		t.Error("the map does not report that it recognises where you are")
	}
	if len(v.Edges) != 1 {
		t.Fatalf("%d connection(s) on the map, want the one in the graph", len(v.Edges))
	}
	e := v.Edges[0]
	if e.From != here || e.To != other {
		t.Errorf("the connection runs %s -> %s", e.From, e.To)
	}
	for what, got := range map[string]string{"from": e.FromWords, "to": e.ToWords} {
		if strings.TrimSpace(got) == "" || strings.Contains(got, "subj_") {
			t.Errorf("the map says the %s of a connection is %q. A subject id is not a "+
				"name, and the map may not invent one.", what, got)
		}
	}
	// AND THE PLACES EITHER SIDE ARE ON IT, with the marker on exactly one.
	marked := 0
	for _, p := range v.Places {
		if p.Here {
			marked++
		}
	}
	if len(v.Places) != 2 || marked != 1 {
		t.Errorf("%d place(s) on the map with %d marked as here, want 2 and 1",
			len(v.Places), marked)
	}
}

// AND REACHABILITY IS THE PLANNER'S ANSWER, NOT THE PICTURE'S.
//
// A destination is offered because `observe.PlanToGoal` produced a route over the canonical graph
// with the canonical eligibility — the same call `Reach` and `PerformGoal` make. A map that drew a
// line because two places happen to be connected would promise something the planner does not
// support.
//
// Deleting the planner call must fail this.
func TestReachabilityComesFromThePlanner(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	here := watchNow(t, g, store, "testgame")
	other, err := store.EstablishPlace("testgame",
		screenLike(9, observe.TermSettings, observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	if _, err := store.RememberRelationships("testgame", []observe.RelationshipObservation{{
		From: here, To: other, Evidence: observe.RelationshipEvidence{Observations: 1},
	}}); err != nil {
		t.Fatalf("remembering: %v", err)
	}

	// A CONNECTION IS NOT A ROUTE. The relationship exists and no candidate demonstrates it,
	// so the planner refuses it — and the map must refuse it too rather than drawing a line
	// because two places touch.
	v := rt.Map(service.ObserveMap{Application: "testgame"})
	if len(v.Edges) == 0 {
		t.Fatal("the fixture drew no connection, so this proves nothing")
	}
	for _, x := range v.Reachable {
		if x.Subject == other {
			t.Fatalf("the map offers %q as reachable on the strength of a connection the "+
				"planner will not use. A picture is not a promise.", x.Words)
		}
	}
}

// THE DIRECTOR ANSWERS WITH ITS MAP, THROUGH THE OBSERVATION DOOR.
//
// A wiring test, because this repository has shipped a complete mechanism reached by nothing six
// times. It enters where a client enters — `Observation` with a Map request — rather than calling
// `Runtime.Map`, which would prove the map can be built and not that anything asks for it.
//
// Deleting the case from the dispatch must fail this.
func TestTheDirectorAnswersWithItsMap(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	if _, _, err := establishTwoIn(t, store); err != nil {
		t.Fatalf("establishing: %v", err)
	}

	out, err := rt.Observation(service.ObserveQuery{
		Map: &service.ObserveMap{Application: recentApp}})
	if err != nil {
		t.Fatalf("asking for the map: %v", err)
	}
	v, ok := out.(service.MapView)
	if !ok {
		t.Fatalf("the observation door answered a map request with %T", out)
	}
	if v.Application != recentApp {
		t.Errorf("the map is about %q", v.Application)
	}
	if v.KnownPlaces == 0 {
		t.Error("the map came back knowing nothing about a store with places in it")
	}
}

// establishTwoIn is establishTwo, reporting its error instead of failing, for a caller that wants
// to say something else about it.
func establishTwoIn(t *testing.T, store interface {
	EstablishPlace(application string, sig observe.StructureSignature) (string, error)
}) (string, string, error) {
	t.Helper()
	from, err := store.EstablishPlace(recentApp, screenLike(4, observe.TermSettings))
	if err != nil {
		return "", "", err
	}
	to, err := store.EstablishPlace(recentApp,
		screenLike(7, observe.TermSettings, observe.TermAudio))
	return from, to, err
}
