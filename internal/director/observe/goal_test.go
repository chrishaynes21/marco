package observe_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The goal-centric correction, held as tests. A goal is a destination; the demonstrated
// route is evidence for one way in; planning composes whatever known edges exist from
// wherever the person currently is.

func topologyOf(edges ...observe.RelationshipRef) observe.Topology {
	top := observe.Topology{Subjects: map[string]observe.RememberedSubject{}}
	for _, e := range edges {
		top.Subjects[e.From] = observe.RememberedSubject{ID: e.From}
		top.Subjects[e.To] = observe.RememberedSubject{ID: e.To}
		top.Relationships = append(top.Relationships,
			observe.RememberedRelationship{From: e.From, To: e.To})
	}
	return top
}

func edge(from, to string) observe.RelationshipRef {
	return observe.RelationshipRef{From: from, To: to}
}

// A goal cannot carry a required start, structurally: the type has no field one could go
// in, under any of the names a start might wear. One demonstration beginning at A proves
// nothing about A being required, and a record that COULD hold a start would eventually be
// read as one.
func TestGoalHasNoRequiredStart(t *testing.T) {
	forbidden := []string{"start", "from", "source", "route", "origin", "steps", "waypoint"}
	typ := reflect.TypeFor[observe.Goal]()
	for i := range typ.NumField() {
		name := strings.ToLower(typ.Field(i).Name)
		for _, f := range forbidden {
			if strings.Contains(name, f) {
				t.Errorf("Goal carries a field %q; a goal is a destination and must have "+
					"nowhere to put a %s", typ.Field(i).Name, f)
			}
		}
	}
}

// Already standing on B: the edge B→C is the whole plan. The demonstration C was learned from
// began at A, and A appears nowhere.
func TestCurrentBCanUseBToC(t *testing.T) {
	top := topologyOf(edge("subj_a", "subj_b"), edge("subj_b", "subj_c"))
	p := observe.PlanToGoal("subj_c", "subj_b", top, nil)
	if p.Refusal != "" || p.Satisfied {
		t.Fatalf("plan refused (%q) or claimed satisfaction", p.Refusal)
	}
	want := []observe.RelationshipRef{edge("subj_b", "subj_c")}
	if !reflect.DeepEqual(p.Steps, want) {
		t.Fatalf("plan from B is %v, want %v — the original START leaked into the plan",
			p.Steps, want)
	}
}

// Knowledge composes: X→B was learned some other day, B→C from the demonstration, and a
// person standing on X gets X→B→C without anybody ever demonstrating that whole chain.
func TestCurrentXCanComposeXToBToC(t *testing.T) {
	top := topologyOf(
		edge("subj_a", "subj_b"), edge("subj_b", "subj_c"), edge("subj_x", "subj_b"))
	p := observe.PlanToGoal("subj_c", "subj_x", top, nil)
	want := []observe.RelationshipRef{edge("subj_x", "subj_b"), edge("subj_b", "subj_c")}
	if !reflect.DeepEqual(p.Steps, want) {
		t.Fatalf("plan from X is %v, want %v", p.Steps, want)
	}
}

// Already there is already there.
func TestCurrentCIsAlreadySatisfied(t *testing.T) {
	top := topologyOf(edge("subj_a", "subj_b"), edge("subj_b", "subj_c"))
	p := observe.PlanToGoal("subj_c", "subj_c", top, nil)
	if !p.Satisfied || len(p.Steps) != 0 || p.Refusal != "" {
		t.Fatalf("standing on the goal produced %+v; want satisfied, no steps, no refusal", p)
	}
}

// No known chain reaches the goal: the refusal is honest and closed, never a guess and
// never a walk back to the demonstration's original start.
func TestUnknownRouteToCRefusesHonestly(t *testing.T) {
	top := topologyOf(edge("subj_a", "subj_b"), edge("subj_b", "subj_c"))
	top.Subjects["subj_island"] = observe.RememberedSubject{ID: "subj_island"}
	p := observe.PlanToGoal("subj_c", "subj_island", top, nil)
	if p.Refusal != observe.PlanNoKnownRoute {
		t.Fatalf("refusal = %q, want %q", p.Refusal, observe.PlanNoKnownRoute)
	}
	if len(p.Steps) != 0 {
		t.Fatalf("a refused plan carries steps: %v", p.Steps)
	}
	if !strings.Contains(p.Refusal.Say(), "don't yet know how to get there") {
		t.Errorf("the refusal sentence hedges: %q", p.Refusal.Say())
	}
}

// One demonstration A→B→C proves the route succeeded; it does not make B required. When a
// direct A→C edge is later learned, planning from A takes it — and neither plan ever puts
// B in the way of the other.
func TestOneDemonstrationDoesNotMakeAWaypointRequired(t *testing.T) {
	top := topologyOf(
		edge("subj_a", "subj_b"), edge("subj_b", "subj_c"), edge("subj_a", "subj_c"))
	p := observe.PlanToGoal("subj_c", "subj_a", top, nil)
	want := []observe.RelationshipRef{edge("subj_a", "subj_c")}
	if !reflect.DeepEqual(p.Steps, want) {
		t.Fatalf("plan from A is %v, want the direct %v — the first demonstration's "+
			"waypoint was baked into the goal", p.Steps, want)
	}
}

// The usable predicate is the caller's authority question, and the planner honours it: an
// edge the predicate refuses is not routed through, however short the chain it would make.
func TestThePlannerRoutesOnlyOverUsableEdges(t *testing.T) {
	top := topologyOf(
		edge("subj_a", "subj_c"), edge("subj_a", "subj_b"), edge("subj_b", "subj_c"))
	verified := map[observe.RelationshipRef]bool{
		edge("subj_a", "subj_b"): true, edge("subj_b", "subj_c"): true,
	}
	p := observe.PlanToGoal("subj_c", "subj_a", top,
		func(r observe.RelationshipRef) bool { return verified[r] })
	want := []observe.RelationshipRef{edge("subj_a", "subj_b"), edge("subj_b", "subj_c")}
	if !reflect.DeepEqual(p.Steps, want) {
		t.Fatalf("plan is %v, want %v — an edge the predicate refused was routed through",
			p.Steps, want)
	}
}

// An unknown position and an unknown goal are different refusals, said apart.
func TestPlanRefusalsAreDiscrete(t *testing.T) {
	top := topologyOf(edge("subj_a", "subj_b"))
	if p := observe.PlanToGoal("subj_gone", "subj_a", top, nil); p.Refusal != observe.PlanGoalUnknown {
		t.Errorf("unknown goal refused as %q", p.Refusal)
	}
	if p := observe.PlanToGoal("subj_b", "", top, nil); p.Refusal != observe.PlanPositionUnknown {
		t.Errorf("empty position refused as %q", p.Refusal)
	}
	if p := observe.PlanToGoal("subj_b", "subj_gone", top, nil); p.Refusal != observe.PlanPositionUnknown {
		t.Errorf("forgotten position refused as %q", p.Refusal)
	}
}
