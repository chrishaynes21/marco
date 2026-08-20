package demo

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Validation, before a demonstration is allowed to become anything.
//
//	Validate: all steps verified, no failed action, no unsafe action, no unresolved
//	clarification, no replay-only targets, no unresolved values, deterministic
//	ordering. Reject partial demonstrations.
//
// Distinct from Unsafe, and the two are not interchangeable. Safety asks whether this
// demonstration may EVER become a procedure — a credential entry may not, however cleanly
// it ran. Validation asks whether this demonstration DESCRIBES one: a task half performed,
// or performed with a question left hanging, is not a description of an outcome even when
// every part of it was harmless.
//
// Both refuse the whole demonstration. A validator that dropped the offending step would
// produce a procedure the user never demonstrated, from the parts of one they did.

// Validate reports whether a demonstration describes a complete, repeatable outcome.
//
// Returns the FIRST failure with its reason. One reason rather than a list, because they
// are not independent: a demonstration that stopped half way usually trips several, and
// the first one is the one that explains the rest.
func Validate(d *Demonstration) (string, bool) {
	if d == nil {
		return "there is no demonstration to validate", false
	}
	for _, check := range validators {
		if why, ok := check(d); !ok {
			return why, false
		}
	}
	return "", true
}

type validator func(*Demonstration) (string, bool)

var validators = []validator{
	deterministicOrdering,
	everyStepVerified,
	noUnresolvedClarification,
	noReplayOnlyTarget,
	noUnresolvedValue,
	notPartial,
}

// deterministicOrdering requires the steps to be a contiguous, ordered sequence.
//
// A procedure is an ORDER. Steps with a gap in them are a recording that lost something,
// and a procedure built from what survived would run a sequence nobody performed.
func deterministicOrdering(d *Demonstration) (string, bool) {
	for i, s := range d.Steps {
		if s.Index != i+1 {
			return fmt.Sprintf(
				"the recorded steps are not a contiguous sequence — step %d is numbered %d — "+
					"so the order they would run in is not the order they happened in",
				i+1, s.Index), false
		}
	}
	return "", true
}

// everyStepVerified requires each step to have proved its own effect.
func everyStepVerified(d *Demonstration) (string, bool) {
	for _, s := range d.Steps {
		if !s.Verified || s.Status != directorapi.ActionSucceeded {
			return fmt.Sprintf(
				"step %d (%s) ended %s rather than verified, so what it achieved is not "+
					"established and a procedure cannot claim it",
				s.Index, s.Describe(), s.Status), false
		}
	}
	return "", true
}

// noUnresolvedClarification refuses a demonstration in which the Director had to ask.
//
// The question it asked is exactly the question a learned procedure would face every time
// it ran, and the user's one-off answer is not an answer to it.
func noUnresolvedClarification(d *Demonstration) (string, bool) {
	for _, s := range d.Steps {
		if s.Clarified {
			return fmt.Sprintf(
				"step %d needed the user to say which control was meant. A procedure has to "+
					"resolve its own targets, and this one could not.", s.Index), false
		}
	}
	return "", true
}

// noReplayOnlyTarget refuses a step aimed at something with no durable description.
//
//	No replay-only targets.
//
// A step whose target is neither a semantic role, nor the object the user pointed at, nor
// the editor a previous step opened, nor a label, has nothing a future run could search
// for. It ran because a handle was resolved at that moment — which is precisely what a
// procedure may not carry.
func noReplayOnlyTarget(d *Demonstration) (string, bool) {
	for _, s := range d.Steps {
		t := s.Target
		if t.Role != "" || t.Deictic || t.DerivedEditor || t.Anaphoric || t.Label != "" {
			continue
		}
		return fmt.Sprintf(
			"step %d acted on something with no durable description — no semantic role, no "+
				"label, and not the object the user pointed at. A procedure cannot look for "+
				"it again.", s.Index), false
	}
	return "", true
}

// noUnresolvedValue refuses a step that consumed a program-local value.
//
// The value belonged to a program that has ended. A learned step reading it would resolve
// to nothing — and the value layer is emphatic that resurrecting the old plaintext is not
// the alternative.
func noUnresolvedValue(d *Demonstration) (string, bool) {
	for _, s := range d.Steps {
		if s.ValueRef != "" {
			return fmt.Sprintf(
				"step %d used the captured value ${%s}, which belonged to the program that "+
					"captured it. A procedure that read it later would find nothing.",
				s.Index, s.ValueRef), false
		}
	}
	return "", true
}

// notPartial requires the demonstration to be more than a fragment.
//
//	Reject partial demonstrations.
//
// Two steps is the floor: one action is a request the user could simply make, and calling
// it a procedure adds a name and nothing else.
func notPartial(d *Demonstration) (string, bool) {
	if len(d.Steps) < 2 {
		return fmt.Sprintf(
			"only %d step was recorded. A single action is a request, not a procedure — "+
				"ask for it directly rather than teaching it.", len(d.Steps)), false
	}
	return "", true
}
