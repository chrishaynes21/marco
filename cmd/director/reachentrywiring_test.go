package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// The route diagnostic says which ENTRY each step was observed being taken from.
//
// # Why this test is in cmd and not beside the planner
//
// Because the planner's own test proves the plan carries approaches, and that is not the same
// claim as a person being able to see them. The whole point of retaining a selection is that it
// can be inspected later; a field nothing renders is a field that will quietly stop being
// filled, which this repository has eight recorded cases of.
//
// So this goes in through `Runtime.Reach` — the production query the CLI and the service both
// call — over the production store, with the evidence written by the production writer.
//
// Deleting the approach block in Runtime.Reach must fail this.
func TestTheDiagnosticSaysWhichEntryAStepWasTakenFrom(t *testing.T) {
	g := verifiedRegistry(t)
	grant := g.last.Grant()
	rememberGoalFor(t, g, "open the audio page", grant.Destination)
	store, ok := g.memory.(*semanticmemory.Store)
	if !ok {
		t.Fatal("the fixture registry holds no production store")
	}

	// ONE SESSION'S SELECTION EVIDENCE, through the store's own write rather than poked
	// into the record: the question is whether the whole chain carries it.
	update, err := store.RememberRelationships("testgame", []observe.RelationshipObservation{{
		From: grant.Source, To: grant.Destination,
		SameEpisode: true,
		Evidence: observe.RelationshipEvidence{
			Observations: 4,
			SelectedBefore: []observe.SelectedEntry{
				{Entry: 3, Entries: 9, Count: 3},
			},
			SelectedUnknown: 1,
			Moves: []observe.SelectionStep{
				{From: 1, To: 2, Entries: 9, Observations: 5, Prompted: 4},
				{From: 2, To: 3, Entries: 9, Observations: 3, Prompted: 3},
			},
		},
	}})
	if err != nil {
		t.Fatalf("writing the selection evidence: %v", err)
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
	if step.Entries != 9 || step.Entry != 3 {
		t.Fatalf("the step reads entry %d of %d; the edge was observed with entry 3 of 9 "+
			"selected. A route whose steps name only two screens each cannot be taught "+
			"as \"open the third entry\", however much was counted in a diagnostic",
			step.Entry, step.Entries)
	}
	if step.Support != 3 || step.Of != 3 {
		t.Errorf("the step reads %d of %d readable observation(s), want 3 of 3",
			step.Support, step.Of)
	}
	if step.Moves != 2 {
		t.Errorf("the step reports %d observed move(s) of the mark in that list, want 2. "+
			"Knowing which row a step goes from is worth nothing without something to "+
			"say about getting the mark there", step.Moves)
	}
	if step.SelectionRefusal != "" {
		t.Errorf("the step carries refusal %q as well as an entry; they are alternatives",
			step.SelectionRefusal)
	}
}

// A step with no selection evidence says so, and that is not a refusal of the route.
//
// Most screens do not change out of a list. The diagnostic has to report "no entry known" for
// those without it reading as a defect, or the surface becomes one people stop checking.
func TestAStepWithNoEntryEvidenceStillPlans(t *testing.T) {
	g := verifiedRegistry(t)
	grant := g.last.Grant()
	rememberGoalFor(t, g, "open the audio page", grant.Destination)

	v := reachOver(t, g, "open the audio page")
	if v.Refusal != "" || len(v.Steps) != 1 {
		t.Fatalf("the route was refused for want of selection evidence: %q, %+v",
			v.Refusal, v.Steps)
	}
	step := v.Steps[0]
	if step.Entries != 0 {
		t.Errorf("a step with no observed selection names entry %d of %d",
			step.Entry, step.Entries)
	}
	if step.SelectionRefusal != string(observe.ApproachUnknownEntry) {
		t.Errorf("the step's selection refusal is %q, want %q",
			step.SelectionRefusal, observe.ApproachUnknownEntry)
	}
}
