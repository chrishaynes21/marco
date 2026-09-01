package main

import (
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// Marco's map of the interface, as a person watches it being built.
//
// # What Observe is for
//
// Not a log. Not a recorder. Not a stream of Director internals. It is where somebody watches
// Marco build a map of their computer, and the dogfood finding that produced this file is that
// they could not tell what Marco currently thought the screen was called, what it had just
// discovered, how Places related to each other, or what any of it would let Marco do.
//
// So the primary object is the GRAPH, and this answers four questions about it:
//
//	WHERE AM I                what fresh perception says, and never memory
//	WHAT DOES MARCO KNOW      the places and connections around here
//	WHAT DID IT JUST FIND     one meaningful new fact
//	WHAT CAN IT REACH         through the canonical planner, from here
//
// # A read, and a projection
//
// It establishes nothing, plans nothing new and cannot act. Every field comes from somewhere that
// already exists: `hereFrom` for the current place, `Topology` for the graph, `PlanToGoal` for
// reachability, `observe.PlaceWords` for every word. Nothing here decides what a place is called,
// which is the whole reason the map and the rest of the product cannot disagree.
//
// # Graph, not workflow
//
// The map shows PLACES and the CONNECTIONS between them, because that is what Marco learns. It
// must never suggest that watching somebody walk somewhere recorded a macro: what was learned is
// that this action, from this screen, arrives at that one — and routes are composed from those
// afterwards. See [[ADR-056-a-goal-is-a-destination-not-a-route]].

// mapNeighbourhood is how far from the current place the map reaches by default.
//
// ONE STEP, either way. The question is "what does Marco know AROUND here", and a person standing
// on a screen can act on what leaves it and understand what led to it. Two steps is already a
// picture of somewhere else.
const mapNeighbourhood = 1

// Map is what Marco knows about where somebody is, for the surface that shows it.
func (r *Runtime) Map(q service.ObserveMap) service.MapView {
	application := strings.TrimSpace(q.Application)
	if application == "" {
		application = r.ambient().view().Application
	}
	out := service.MapView{Application: application}
	if application == "" {
		return out
	}
	memory, ok := r.durableMemory()
	if !ok {
		return out
	}
	top := memory.Topology(application)

	// WHERE THE PERSON IS, FROM PERCEPTION. Never from memory, and never from the graph: a
	// remembered Place is not a visible Place, and a marker that moved because Marco recalled
	// something would be the map telling somebody where they are on the strength of where
	// they once were.
	//
	// Deleting the perception source must fail TestTheMapMarkerComesFromPerception.
	here := r.placeNowIn(application)
	out.Here = here
	out.HereWords = observe.Unnamed
	if here != "" {
		out.HereWords = placeWordsIn(top, here)
		out.HereKnown = true
	}

	out.Places, out.Edges = mapAround(top, here, mapNeighbourhood)
	out.KnownPlaces, out.KnownEdges = len(top.Subjects), len(top.Relationships)
	if here != "" {
		out.Reachable = r.reachableFrom(application, top, here)
	}
	return out
}

// mapAround is the neighbourhood: the places within one step of here, and what connects them.
//
// # Why not the whole graph
//
// Because "what does Marco know around here" is a question a person can act on and "here is
// everything Marco has ever seen" is a picture. The whole topology is still reported as COUNTS, so
// somebody watching the map grow can see it grow without being shown all of it.
//
// With no current place the neighbourhood is empty rather than global: a map with no marker on it
// is not a map of where you are, and drawing the whole graph in that state would be answering a
// different question than the one that was asked.
//
// Deterministic order — by name, then by id — so two readings of one store draw the same map.
func mapAround(top observe.Topology, here string, depth int) ([]service.MapPlace,
	[]service.MapEdge) {

	if here == "" || depth < 1 {
		return nil, nil
	}
	near := map[string]bool{here: true}
	var edges []service.MapEdge
	for _, rel := range top.Relationships {
		if rel.From != here && rel.To != here {
			continue
		}
		near[rel.From], near[rel.To] = true, true
		edges = append(edges, service.MapEdge{
			From: rel.From, To: rel.To,
			FromWords: placeWordsIn(top, rel.From), ToWords: placeWordsIn(top, rel.To),
			// OBSERVED AND VERIFIED ARE DIFFERENT FACTS, and the map says which.
			// "Somebody did this once" and "Marco has done it and checked" are not the
			// same claim, and a surface that drew them alike would be inviting
			// confidence Marco has not earned.
			Observations: rel.Observations,
		})
	}
	places := make([]service.MapPlace, 0, len(near))
	for id := range near {
		s, held := top.Subjects[id]
		if !held {
			continue
		}
		places = append(places, service.MapPlace{
			Subject: id, Words: observe.PlaceWords(s),
			Describes: observe.DescribeStructure(s.Structure),
			Here:      id == here,
		})
	}
	sort.Slice(places, func(i, j int) bool {
		if places[i].Words != places[j].Words {
			return places[i].Words < places[j].Words
		}
		return places[i].Subject < places[j].Subject
	})
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromWords != edges[j].FromWords {
			return edges[i].FromWords < edges[j].FromWords
		}
		if edges[i].ToWords != edges[j].ToWords {
			return edges[i].ToWords < edges[j].ToWords
		}
		return edges[i].From < edges[j].From
	})
	return places, edges
}

// reachableFrom is where Marco believes it could get to from here, through the canonical planner.
//
// # The planner's answer, never the map's
//
// A destination appears because `observe.PlanToGoal` produced a route over the canonical graph
// with the canonical eligibility — the same call `Reach` and `PerformGoal` make. A map that drew a
// line because two places happen to be connected would be claiming reachability the planner does
// not support, which is the difference between a picture and a promise.
//
// Verified says every step of that route is one Marco has walked and checked. Unverified is
// ordinary and honest — "I know a way and I have not earned every step of it yet" — and the two
// are reported apart because they are different claims.
//
// Deleting the planner call must fail TestReachabilityComesFromThePlanner.
func (r *Runtime) reachableFrom(application string, top observe.Topology,
	here string) []service.MapReach {

	grade := r.plannableEdges(application, top)
	var out []service.MapReach
	for id, s := range top.Subjects {
		if id == here {
			continue
		}
		plan := observe.PlanToGoal(id, here, top, grade)
		if plan.Refusal != "" || plan.Satisfied || len(plan.Steps) == 0 {
			continue
		}
		verified := true
		for _, step := range plan.Steps {
			rank, ok := grade(step)
			if !ok || rank.Class != observe.ClassVerified {
				verified = false
				break
			}
		}
		out = append(out, service.MapReach{
			Subject: id, Words: observe.PlaceWords(s),
			Steps: len(plan.Steps), Verified: verified,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Steps != out[j].Steps {
			return out[i].Steps < out[j].Steps
		}
		if out[i].Words != out[j].Words {
			return out[i].Words < out[j].Words
		}
		return out[i].Subject < out[j].Subject
	})
	return out
}
