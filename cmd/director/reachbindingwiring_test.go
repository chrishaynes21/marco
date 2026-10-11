package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// The route diagnostic says which DIRECTIONS each step's moves were observed following.
//
// # Why this is in cmd and not beside the planner
//
// Because the planner's own test proves the plan carries the directions, and that is not the
// same claim as a person being able to see them. A route that names a row and cannot name a
// single control is non-executable, and saying so out loud is the point of this milestone — a
// field nothing renders is a field that will quietly stop being filled, which this repository
// has eight recorded cases of.
//
// So this goes in through `Runtime.Reach` — the production query the CLI and the service both
// call — over the production store, with the evidence written by the production writer.
//
// Deleting the Bound/Bindings block in Runtime.Reach must fail this.
func TestTheDiagnosticSaysWhichDirectionsAreKnown(t *testing.T) {
	g := verifiedRegistry(t)
	grant := g.last.Grant()
	rememberGoalFor(t, g, "open the audio page", grant.Destination)
	store, ok := g.memory.(*semanticmemory.Store)
	if !ok {
		t.Fatal("the fixture registry holds no production store")
	}

	// ONE SESSION'S DIRECTION EVIDENCE, through the store's own write rather than poked
	// into the record: the question is whether the whole chain carries it. Three moves,
	// one with repeated evidence behind one direction, one contradicted, one never
	// attributed — so the count the diagnostic prints has to be 1 and not 3.
	update, err := store.RememberRelationships("testgame", []observe.RelationshipObservation{{
		From: grant.Source, To: grant.Destination,
		SameEpisode: true,
		Evidence: observe.RelationshipEvidence{
			Observations: 4,
			SelectedBefore: []observe.SelectedEntry{
				{Entry: 3, Entries: 9, Count: 3},
			},
			Moves: []observe.SelectionStep{
				{From: 1, To: 2, Entries: 9, Observations: 5, Prompted: 5,
					Directions: []observe.DirectionCount{
						{Intent: observe.NavDown, Count: 5},
					}},
				{From: 2, To: 3, Entries: 9, Observations: 6, Prompted: 6,
					Directions: []observe.DirectionCount{
						{Intent: observe.NavDown, Count: 3},
						{Intent: observe.NavUp, Count: 3},
					}},
				{From: 3, To: 4, Entries: 9, Observations: 3},
			},
		},
	}})
	if err != nil {
		t.Fatalf("writing the direction evidence: %v", err)
	}
	if update.Corroborated+update.Created == 0 {
		t.Fatalf("the store wrote nothing: %+v", update)
	}

	v := reachOver(t, g, "open the audio page")
	if v.Refusal != "" {
		t.Fatalf("refused %q (%s)", v.Refusal, v.Say)
	}
	if len(v.Steps) != 1 {
		t.Fatalf("plan = %+v, want one step", v.Steps)
	}
	step := v.Steps[0]
	if step.Moves != 3 {
		t.Fatalf("the step reports %d observed move(s), want 3", step.Moves)
	}
	if step.Bound != 1 {
		t.Errorf("the step says %d of its 3 moves have a known direction, want 1: one is "+
			"contradicted and one was never attributed, and counting either would "+
			"claim a control nothing established", step.Bound)
	}
	if len(step.Bindings) != 3 {
		t.Fatalf("the step renders %d binding line(s), want one per observed move: %v",
			len(step.Bindings), step.Bindings)
	}
	joined := strings.Join(step.Bindings, "\n")
	for _, want := range []string{
		"repeated", "down: 5",
		string(observe.BindingContradicted),
		string(observe.BindingUnattributed),
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the rendered bindings never say %q, so a person cannot see what "+
				"Marco does and does not know how to press:\n%s", want, joined)
		}
	}
	t.Logf("step 1: %s → %s, %d move(s), %d bound\n  %s",
		step.From, step.To, step.Moves, step.Bound, joined)
}
