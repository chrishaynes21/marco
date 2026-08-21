package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// `reach` through the production wiring: goals resolve, plans run over VERIFIED edges from
// where the person was last seen, and knowledge never becomes authority on the way through.

func reachOver(t *testing.T, g *observationRegistry, name string) service.ReachView {
	t.Helper()
	rt := &Runtime{observations: g}
	v, err := rt.Reach(service.ObserveReach{Name: name})
	if err != nil {
		t.Fatalf("reach: %v", err)
	}
	return v
}

func rememberGoalFor(t *testing.T, g *observationRegistry, name, subject string) {
	t.Helper()
	store, ok := g.memory.(*semanticmemory.Store)
	if !ok {
		t.Fatal("the fixture registry holds no production store")
	}
	if err := store.RememberGoal("testgame", observe.Goal{Name: name,
		Subject: subject}); err != nil {
		t.Fatalf("remembering the goal: %v", err)
	}
}

// A verified edge from where the person stands is a one-step plan — and the plan is FROM
// there, never from wherever the original demonstration began.
func TestReachPlansOverTheVerifiedEdgeFromWhereThePersonStands(t *testing.T) {
	g := verifiedRegistry(t)
	grant := g.last.Grant()
	rememberGoalFor(t, g, "open the audio page", grant.Destination)

	v := reachOver(t, g, "open the audio page")
	if v.Refusal != "" {
		t.Fatalf("refused %q (%s); current=%q", v.Refusal, v.Say, v.Current)
	}
	if len(v.Steps) != 1 || v.Steps[0].From != grant.Source ||
		v.Steps[0].To != grant.Destination {
		t.Fatalf("plan = %+v, want the one verified edge %s → %s",
			v.Steps, grant.Source, grant.Destination)
	}
}

// AN OBSERVED EDGE CAN BE PLANNED OVER, and planning still creates no authority.
//
// # This test asserted the opposite until Roadmap 35B
//
// It read: "an unrehearsed edge entered a plan … only a completed rehearsal earns an edge that
// place." That was right while Learn could not finish WITHOUT rehearsing every edge — the only way
// an edge could exist unrehearsed was for something to have gone wrong partway through.
//
// Fast Learn removed that ceremony, so a clean demonstration now produces edges Marco understands
// perfectly well and has never walked. A planner that refused them would refuse to plan over the
// knowledge Learn had just acquired, and "I learned that" would be followed by "I don't know how".
//
// # What did NOT change, and is asserted here because it is the whole safety argument
//
// Planning is not permission — PlanToGoal says so at its own definition. The three things that
// stand between a plan and an effect are all downstream: the authority door mints a grant per
// invocation, the foreground must lead before input is emitted, and every edge is positively
// verified as it is walked. A read of the plan spends none of them, which is what the grant
// assertion below holds.
//
// Deleting the observational arm of plannableEdges must fail this.
func TestAnObservedEdgeCanBePlannedOver(t *testing.T) {
	g := authorizedRegistry(t) // demonstrated cleanly, never rehearsed by Marco
	grant := g.last.Grant()
	rememberGoalFor(t, g, "open the audio page", grant.Destination)

	v := reachOver(t, g, "open the audio page")
	if v.Refusal != "" {
		t.Fatalf("refused %q (%s). A route the person demonstrated cleanly is a route Marco "+
			"knows; refusing to plan over it leaves Learn unable to produce anything runnable.",
			v.Refusal, v.Say)
	}
	if len(v.Steps) != 1 {
		t.Fatalf("plan = %+v, want the one demonstrated edge", v.Steps)
	}
	// AND THE READ CONSUMED NOTHING. Knowing a way is not being allowed to walk it.
	if got := g.last.Grant(); got == nil || !got.Active() {
		t.Error("a read of the plan touched the rehearsal grant. Planning must create and " +
			"consume no authority whatsoever.")
	}
}

// Listing outcomes is a read of the goal records, nothing more.
func TestReachListsLearnedOutcomes(t *testing.T) {
	g := verifiedRegistry(t)
	grant := g.last.Grant()
	rememberGoalFor(t, g, "open the audio page", grant.Destination)

	v := reachOver(t, g, "")
	if len(v.Outcomes) != 1 || v.Outcomes[0].Name != "open the audio page" {
		t.Fatalf("outcomes = %+v", v.Outcomes)
	}
}
