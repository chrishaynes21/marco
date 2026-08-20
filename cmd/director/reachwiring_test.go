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

// A goal whose route has never survived a rehearsal yields NO plan — knowledge of the way
// is said honestly as known-but-unearned, and nothing here creates or consumes authority.
func TestAKnownGoalWithoutARehearsedRouteRefusesHonestly(t *testing.T) {
	g := authorizedRegistry(t) // demonstrated and granted, never rehearsed
	grant := g.last.Grant()
	rememberGoalFor(t, g, "open the audio page", grant.Destination)

	v := reachOver(t, g, "open the audio page")
	if len(v.Steps) != 0 {
		t.Fatalf("an unrehearsed edge entered a plan: %+v.\nA plan is a claim about what "+
			"Marco may rely on, and only a completed rehearsal earns an edge that place.",
			v.Steps)
	}
	if v.Refusal != string(observe.PlanNoKnownRoute) {
		t.Errorf("refusal = %q, want %q", v.Refusal, observe.PlanNoKnownRoute)
	}
	if !v.KnownUnverified {
		t.Error("the observed-but-unrehearsed way was not acknowledged; \"I don't know how\" " +
			"and \"I know how and haven't earned it\" invite different responses")
	}
	// And the read consumed nothing: the person's one grant is exactly as it was.
	if got := g.last.Grant(); got == nil || !got.Active() {
		t.Error("a read of the plan touched the rehearsal grant")
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
