package observesession_test

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// The goal-centric decomposition, through the production runner.
//
// A demonstration A → B → C is TWO reusable pieces of route knowledge, never one monolithic
// macro: each leg becomes its own candidate for its own edge, and the leg that arrived where
// the person stopped is the one the learn tail carries forward. Nothing here infers that
// B is required for C — the edges stand alone, and a later X → C coexists with both.

// threeSubjects remembers screens a, b and c, so every leg of the walk can resolve.
func threeSubjects(t *testing.T, store *semanticmemory.Store) (a, b, c string) {
	t.Helper()
	for _, sig := range []observe.StructureSignature{aSignature(), bSignature(), cSignature()} {
		if err := store.Remember("testgame", sig, observe.SemanticKnowledge{
			Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
		}); err != nil {
			t.Fatalf("seeding a subject: %v", err)
		}
	}
	for _, s := range store.Subjects() {
		if len(s.Structure.Terms) == 0 {
			continue
		}
		switch s.Structure.Terms[0] {
		case observe.TermControls:
			a = s.ID
		case observe.TermAudio:
			b = s.ID
		case observe.TermInvite:
			c = s.ID
		}
	}
	if a == "" || b == "" || c == "" {
		t.Fatalf("could not identify the three seeded subjects")
	}
	return a, b, c
}

// twoLegWalk is a person walking a → b → c, pausing briefly on b the way anybody does.
func twoLegWalk() []demoFrame {
	var out []demoFrame
	out = append(out, hold("a", 4)...)
	out = append(out, press("b", observe.NavDown, observe.NavConfirm))
	out = append(out, hold("b", 3)...)
	out = append(out, press("c", observe.NavDown, observe.NavDown, observe.NavConfirm))
	out = append(out, hold("c", 8)...)
	return out
}

func TestAMultiLegDemonstrationDecomposesIntoReusableEdges(t *testing.T) {
	store := memoryAt(t, t.TempDir())
	a, b, c := threeSubjects(t, store)

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: twoLegWalk()}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	res, err := r.Run(context.Background(), licensed())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Both legs are durable route knowledge now.
	top := store.Topology("testgame")
	edges := map[observe.RelationshipRef]bool{}
	for _, rel := range top.Relationships {
		edges[observe.RelationshipRef{From: rel.From, To: rel.To}] = true
	}
	if !edges[observe.RelationshipRef{From: a, To: b}] ||
		!edges[observe.RelationshipRef{From: b, To: c}] {
		t.Fatalf("the walk left edges %v; want both a→b and b→c durable", edges)
	}

	// One candidate PER edge — the decomposition itself. A single record spanning a→c would
	// be the monolithic macro this milestone removes.
	stored := store.Candidates(res.Session.Application)
	byEdge := map[observe.RelationshipRef]int{}
	for _, cand := range stored {
		byEdge[cand.Relationship]++
		if cand.Relationship.From == a && cand.Relationship.To == c {
			t.Fatalf("a monolithic a→c candidate was stored: %+v", cand)
		}
	}
	if byEdge[observe.RelationshipRef{From: a, To: b}] != 1 ||
		byEdge[observe.RelationshipRef{From: b, To: c}] != 1 {
		t.Fatalf("candidates per edge = %v; want one for a→b and one for b→c", byEdge)
	}

	// The TERMINAL leg — the one that arrived where the person stopped — is the one the
	// learn tail carries forward.
	if res.Demonstration == nil {
		t.Fatalf("no terminal demonstration was reported (refusal %q)", res.Watched)
	}
	if got := res.Demonstration.Relationship; got.From != b || got.To != c {
		t.Fatalf("the terminal leg is %+v, want %s → %s", got, b, c)
	}

	// And every attributed press of the whole walk is still in the raw record — capture
	// first, interpret second, whatever the decomposition made of it.
	presses := 0
	for _, e := range res.Stats.Shadow.InputLog.Events {
		if e.Event.Intent != "" {
			presses++
		}
	}
	if presses != 5 {
		t.Errorf("the input log holds %d events, want the 5 the person made", presses)
	}
}

// A click-based demonstration, through the production runner: the click resolved to a named
// control, so the candidate carries the target, the assessment holds together, and the
// rehearsal plan aims at the control by name. The realistic shape of "Learn 'open Mouse
// settings'" — one ordinary human click.
func TestAClickDemonstrationBecomesAnAimableCandidate(t *testing.T) {
	store := memoryAt(t, t.TempDir())
	threeSubjects(t, store)

	click := demoFrame{screen: "b",
		inputs:  []observe.NavIntent{observe.NavPoint},
		targets: []observe.SemanticTarget{{Role: "list_item", Label: "Mouse"}},
	}
	var script []demoFrame
	script = append(script, hold("a", 4)...)
	script = append(script, click)
	script = append(script, hold("b", 8)...)

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: script}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	res, err := r.Run(context.Background(), licensed())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Demonstration == nil {
		t.Fatalf("no demonstration was built (refusal %q)", res.Watched)
	}
	step := res.Demonstration.Steps[0]
	if len(step.Intents) != 1 || step.Intents[0] != observe.NavPoint {
		t.Fatalf("the step records %v", step.Intents)
	}
	if got := step.TargetAt(0); got.Label != "Mouse" || got.Role != "list_item" {
		t.Fatalf("the click's target did not survive into the candidate: %+v", got)
	}
	if res.Assessment == nil {
		t.Fatal("no assessment")
	}
	for _, reason := range res.Assessment.Reasons {
		if reason == observe.ReasonUnresolvedPointer {
			t.Fatalf("a RESOLVED click was refused as an unresolved pointer: %v",
				res.Assessment.Reasons)
		}
	}
	if res.Assessment.Verdict != observe.CandidateConsistent {
		t.Fatalf("verdict = %q (%v); a clean one-click demonstration should hold together",
			res.Assessment.Verdict, res.Assessment.Reasons)
	}
	// And the rehearsal judgement carries the aim, so an attempt can invoke the control by
	// name rather than refuse the whole route.
	top := store.Topology(res.Session.Application)
	j := observe.JudgeRehearsal(*res.Demonstration, *res.Assessment, top,
		res.Session.Application)
	if !j.Eligible {
		t.Fatalf("the click route is not rehearsable: %v", j.Refusals)
	}
	if len(j.Plan) != 1 || len(j.Plan[0].Targets) != 1 ||
		j.Plan[0].Targets[0].Label != "Mouse" {
		t.Fatalf("the plan lost the aim: %+v", j.Plan)
	}
}
