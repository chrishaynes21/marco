package main

// WHAT A PERSON IS TOLD THEY CAN DO WITH WHAT THEY DEMONSTRATED.
//
// `director sequences` said a pixel routine CANNOT BE RUN and sent the person to `reacquire`,
// which measures whether a control can be found again and presses nothing. True when written —
// `runnable` asks whether the ORDINARY execution path can act, and a pixel routine carries no
// verb — and false from the moment `perform-workflow` learned to ground a saved control from
// pixels, press it and verify the outcome. Marco was reporting the thing it had just learned to
// do as impossible, about three routines on the owner's own machine that a person had
// demonstrated by hand.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// unscoped is a routine from before workflows recorded their application.
func unscoped(a workflowstore.Artifact) workflowstore.Artifact {
	a.Application = ""
	return a
}

// withoutOutcomes is a routine from before a step recorded what its action should produce.
func withoutOutcomes(a workflowstore.Artifact) workflowstore.Artifact {
	steps := make([]workflowstore.Step, len(a.Steps))
	copy(steps, a.Steps)
	for i := range steps {
		steps[i].Outcome = nil
	}
	a.Steps = steps
	return a
}

// TestAScopedPixelRoutineIsOfferedToThePixelExecutionPath.
func TestAScopedPixelRoutineIsOfferedToThePixelExecutionPath(t *testing.T) {
	said := strings.Join(howToRun(aNamedDemonstration("graphics tab")), "\n")

	if !strings.Contains(said, "perform-workflow --name \"graphics tab\"") {
		t.Errorf("a scoped pixel play was not offered to the command that can run "+
			"it:\n%s", said)
	}
	if strings.Contains(said, "reacquire") {
		t.Errorf("a runnable play was sent to a measurement command:\n%s", said)
	}
	if strings.Contains(said, "cannot be verified") {
		t.Errorf("a play carrying an outcome expectation was described as "+
			"unverifiable:\n%s", said)
	}
}

// TestAnUnscopedRoutineIsNotOfferedToACommandThatWillRefuseIt.
//
// The three Ark routines on the owner's machine are in this case: demonstrated by hand, before
// scope existed. An unscoped artifact is valid for NO application, so offering it to
// `perform-workflow` would send somebody to a refusal — and the application identity cannot be
// invented here, because it has to come from a window that was observed.
func TestAnUnscopedRoutineIsNotOfferedToACommandThatWillRefuseIt(t *testing.T) {
	a := unscoped(aNamedDemonstration("ark menu 2"))
	said := strings.Join(howToRun(a), "\n")

	if strings.Contains(said, "perform-workflow") {
		t.Errorf("an unscoped play was offered to the execution path, which will "+
			"refuse it:\n%s", said)
	}
	if !strings.Contains(said, "UNSCOPED") {
		t.Errorf("the refusal does not say what is missing:\n%s", said)
	}
	if !strings.Contains(said, "demonstrate-visual \"ark menu 2\"") {
		t.Errorf("the person is not told how to make it runnable:\n%s", said)
	}
	// AND THE GATE AGREES WITH THE ADVICE. If this ever starts allowing an unscoped
	// artifact, the advice above is wrong and this test is why somebody finds out.
	if ok, _ := a.ScopeAllows("arkascended"); ok {
		t.Error("an unscoped play is valid for an application, so the advice not " +
			"to run it is wrong")
	}
}

// TestARoutineWithNoOutcomeSaysItCannotBeVerified.
//
// Performed and not verified is a different offer from performed and verified, and a person
// about to drive their desktop should be told which one they are getting. The routines saved
// before today are all in this case too.
func TestARoutineWithNoOutcomeSaysItCannotBeVerified(t *testing.T) {
	said := strings.Join(howToRun(withoutOutcomes(aNamedDemonstration("ark menu 2"))), "\n")

	if !strings.Contains(said, "perform-workflow") {
		t.Errorf("a scoped play with no outcome is still runnable:\n%s", said)
	}
	if !strings.Contains(said, "cannot be verified") {
		t.Errorf("a play with nothing to verify against was offered as though it "+
			"would be verified:\n%s", said)
	}
}

// TestAVerbRoutineStillGoesToTheOrdinaryPath, which is what `runnable` has always meant.
func TestAVerbRoutineStillGoesToTheOrdinaryPath(t *testing.T) {
	a := aNamedDemonstration("rename this file")
	steps := make([]workflowstore.Step, len(a.Steps))
	copy(steps, a.Steps)
	steps[0].Verb = "click"
	a.Steps = steps

	said := strings.Join(howToRun(a), "\n")
	if !strings.Contains(said, "director execute \"run rename this file\"") {
		t.Errorf("a play with verbs was not offered to the ordinary path:\n%s", said)
	}
	if strings.Contains(said, "perform-workflow") {
		t.Errorf("a play with verbs was sent to the pixel path as well:\n%s", said)
	}
}

// TestARoutineWithNeitherSaysSoRatherThanOfferingSomething.
func TestARoutineWithNeitherSaysSoRatherThanOfferingSomething(t *testing.T) {
	a := aNamedDemonstration("nothing usable")
	a.Controls = nil
	steps := make([]workflowstore.Step, len(a.Steps))
	copy(steps, a.Steps)
	steps[0].Control = ""
	a.Steps = steps

	said := strings.Join(howToRun(a), "\n")
	if strings.Contains(said, "perform-workflow") || strings.Contains(said, "execute") {
		t.Errorf("a play with neither a verb nor visual evidence was offered "+
			"somewhere:\n%s", said)
	}
	if !strings.Contains(said, "nothing to run") {
		t.Errorf("it does not say there is nothing to do:\n%s", said)
	}
}
