package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// THE TEACHING FLOW, END TO END — the commands a person runs and what they are told.

// A PIXEL ROUTINE IS NOT OFFERED AS RUNNABLE.
//
// Found while verifying the teaching walkthrough end to end on a real pixel artifact: both the
// listing and the detail view printed *Run one with: director execute "run enter free play"*
// under a routine whose steps carry visual evidence and no verb. `execute` can do nothing with
// it, so the suggestion sends somebody to a confusing failure at step three of a walkthrough
// that had worked until then.
func TestAPixelRoutineIsNotOfferedAsRunnable(t *testing.T) {
	pixel := workflowstore.Artifact{
		Version: workflowstore.Version, Name: "enter free play",
		Screens: []workflowstore.Screen{{Label: "screen_a"}, {Label: "screen_b"}},
		Controls: []workflowstore.Control{{
			Label:    "control_1",
			Readings: []observe.Region{{X: 0.4, Y: 0.5, Width: 0.1, Height: 0.02}},
		}},
		Steps: []workflowstore.Step{{
			Position: 0, From: "screen_a", To: "screen_b",
			Control: "control_1", Cause: "click", Seen: 1,
		}},
	}
	if runnable(pixel) {
		t.Error("a play whose steps have no verb was offered as runnable")
	}
	// AND A ROUTINE WITH NO STEPS IS NOT RUNNABLE EITHER. Offering `execute` for one is
	// the same confusing failure, reached by a shorter route.
	if runnable(workflowstore.Artifact{Version: workflowstore.Version, Name: "empty"}) {
		t.Error("a play with no steps at all was offered as runnable")
	}
	shown := renderSequence(pixel)
	if strings.Contains(shown, "director execute") {
		t.Errorf("the detail view offers to execute a pixel play:\n%s", shown)
	}
	// IT CANNOT BE RUN, AND THE REASON CHANGED. This asserted "CANNOT BE RUN YET", which
	// was about carrying no verb — true of every pixel routine when it was written, and
	// false once `perform-workflow` could ground a saved control from pixels and press it.
	// This fixture is UNSCOPED, which is the reason it cannot be run now: an artifact
	// saved before workflows recorded their application is valid for no application.
	// See howToRun and [[ADR-239-running-what-you-demonstrated-by-the-name-you-gave-it]].
	if !strings.Contains(shown, "CANNOT BE RUN AS SAVED") {
		t.Errorf("the detail view does not say it cannot run:\n%s", shown)
	}
	if !strings.Contains(shown, "UNSCOPED") {
		t.Errorf("the detail view does not say WHY it cannot run:\n%s", shown)
	}
	// AND IT IS NOT SENT TO THE COMMAND THAT WOULD REFUSE IT.
	if strings.Contains(shown, "perform-workflow") {
		t.Errorf("an unscoped play is offered to the execution path:\n%s", shown)
	}
	if !strings.Contains(shown, "director reacquire") {
		t.Errorf("the detail view does not say what CAN be done with it — measuring "+
			"the controls needs no scope:\n%s", shown)
	}
	// AND A CONTROL WITH ONLY VISUAL EVIDENCE SAYS SO. "the focused control" was the
	// fallback for all of them, which is the opposite claim: one is resolved at run time
	// by asking what has focus, the other by comparing rectangles.
	if strings.Contains(shown, "the focused control") {
		t.Errorf("a control found by sight is described as the focused control:\n%s", shown)
	}
	if !strings.Contains(shown, "by sight only") {
		t.Errorf("the detail view does not say the control is found by sight:\n%s", shown)
	}

	// AND A DESKTOP ROUTINE IS STILL RUNNABLE, which is the other half: this must not
	// have become a rule that refuses everything.
	desktop := workflowstore.Artifact{
		Version: workflowstore.Version, Name: "my settings tour",
		Controls: []workflowstore.Control{{Label: "control_1", Text: []string{"Gaming"}}},
		Steps: []workflowstore.Step{
			{Position: 0, Control: "control_1", Verb: "open", Says: `open "Gaming"`},
		},
	}
	if !runnable(desktop) {
		t.Error("a play with a verb per step was refused as unrunnable")
	}
	if shown := renderSequence(desktop); strings.Contains(shown, "CANNOT BE RUN") {
		t.Errorf("a desktop play is reported as unrunnable:\n%s", shown)
	}
}

// AND LISTING SAVED SEQUENCES WORKS WITHOUT THE SERVICE.
//
// `demonstrate-visual` and `reacquire` both read and write the store directly, so a person
// teaching Marco a game menu had to start a daemon for the one command in the middle that only
// lists files. A saved sequence is read fresh from disk on every call and the daemon holds no
// copy, so the two answers are the same bytes.
func TestListingSavedSequencesWorksWithoutTheService(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)

	// NOTHING SAVED is the ordinary answer, not an error about a missing service.
	got, code := savedSequences()
	if code != 0 {
		t.Fatalf("listing an empty store failed with %d", code)
	}
	if len(got) != 0 {
		t.Errorf("an empty store listed %d sequence(s)", len(got))
	}

	store, err := demo.Open(configDir())
	if err != nil {
		t.Fatal(err)
	}
	a := workflowstore.Artifact{
		Version: workflowstore.Version, Name: "enter free play",
		Controls: []workflowstore.Control{{Label: "control_1", Text: []string{"PLAY"}}},
		Steps:    []workflowstore.Step{{Position: 0, Control: "control_1", Verb: "open"}},
	}
	if err := store.SaveSequence(a); err != nil {
		t.Fatal(err)
	}
	got, code = savedSequences()
	if code != 0 {
		t.Fatalf("listing failed with %d", code)
	}
	if len(got) != 1 || got[0].Name != "enter free play" {
		t.Errorf("the listing is %+v", got)
	}
}
