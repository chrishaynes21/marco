package service

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// How many things were done to the person's computer.
//
// # Measured, on the task that earned it
//
//	marco do "add milk, eggs and bread, then change milk to oat milk"
//	[1/2] add milk, eggs and bread — verified
//	[2/2] change milk to oat milk — verified
//	COMPLETED: 2 of 2 steps verified
//	(1 action performed)
//
// Two edits landed in the document and the report said one. The outcome above it was right, so
// this is not a false success — it is the number that exists so somebody can tell what happened
// without reading a trace, and it did not count a program at all.
//
// The old arms were a replay's completed count and "this Outcome performed one action". A
// program's actions are in its STEPS, and a collection's are in its iterations, so both fell
// through to zero or to the single trailing record.

func performed() *directorapi.ActionRecord {
	r := &directorapi.ActionRecord{}
	r.Execution.Performed = true
	return r
}

func TestEveryStepOfAProgramIsCounted(t *testing.T) {
	out := execute.Outcome{Program: &execute.ProgramOutcome{
		Completed: 2,
		Steps: []execute.Outcome{
			{Record: performed()},
			{Record: performed()},
		},
	}}
	if got := completedActions(out); got != 2 {
		t.Errorf("a two-step program reported %d actions; the person is told how much was "+
			"done to their computer by this number alone", got)
	}
}

// TestOnlyTheStepsThatRanAreCounted — a program that stopped did not do the rest of itself.
//
// Counting Program.Steps rather than Program.Program.Steps is the whole of this: the planned
// steps are what was intended, and the only honest answer is what ran.
func TestOnlyTheStepsThatRanAreCounted(t *testing.T) {
	out := execute.Outcome{Program: &execute.ProgramOutcome{
		Steps: []execute.Outcome{
			{Record: performed()},
			{}, // resolved nothing, performed nothing
		},
	}}
	if got := completedActions(out); got != 1 {
		t.Errorf("got %d, want 1 — a step that performed nothing was counted", got)
	}
}

// TestAStepThatIsItselfASequenceIsCounted keeps the count honest one level down.
//
// A step's Outcome can be a replay or a program of its own, so the count recurses. A flat sum
// of "did this step have a record" would report one for a step that replayed six actions.
func TestAStepThatIsItselfASequenceIsCounted(t *testing.T) {
	out := execute.Outcome{Program: &execute.ProgramOutcome{
		Steps: []execute.Outcome{
			{Record: performed()},
			{Replay: &execute.ReplayOutcome{Completed: 3}},
		},
	}}
	if got := completedActions(out); got != 4 {
		t.Errorf("got %d, want 4 — a replay inside a program was counted as one action", got)
	}
}

// TestACollectionReportsItsIterations — "click every checkbox" is many actions in one command.
func TestACollectionReportsItsIterations(t *testing.T) {
	out := execute.Outcome{Collection: &collections.Result{Completed: 5, Attempted: 5}}
	if got := completedActions(out); got != 5 {
		t.Errorf("a collection of five reported %d actions", got)
	}
}

// TestASingleActionStillCountsAsOne is the case that always worked, kept so widening the
// function cannot narrow it.
func TestASingleActionStillCountsAsOne(t *testing.T) {
	if got := completedActions(execute.Outcome{Record: performed()}); got != 1 {
		t.Errorf("a single performed action reported %d", got)
	}
	var none directorapi.ActionRecord
	if got := completedActions(execute.Outcome{Record: &none}); got != 0 {
		t.Errorf("an action that did not perform reported %d", got)
	}
}
