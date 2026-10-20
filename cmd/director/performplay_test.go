package main

// RUNNING A SAVED ACT BY NAME, AND REFUSING BEFORE ANYTHING IS ACQUIRED.
//
// `--play` is the named production entry: select a saved Act, preflight it, perform it under
// supervision, and cite the run. Everything it does after preflight is `perform-workflow`'s own
// path, which has its own tests — what is asserted here is the part that is new, and the part a
// person's safety rests on: a play that cannot be performed is refused BEFORE the window is
// acquired, before a screen is named, before anybody is asked, and before anything is pressed.
//
// None of these tests can press anything: every one of them stops before a runner exists. That is
// the assertion, not a limitation of the harness.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aSavedAct writes a routine into a temporary store and a play beside it in a temporary routes
// tree, through the production save path, and points this process at both.
func aSavedAct(t *testing.T) (workflowstore.Artifact, routes.Route) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	t.Setenv("MARCO_ROUTES", filepath.Join(home, "routes"))

	art := aPlayableRoutine()
	// THE STORE THE COMMAND WILL READ, which is `configDir()` — and that honours
	// $MARCO_HOME, set above. Opening it here rather than naming a path keeps the test
	// and the command looking in the same place.
	store, err := demo.Open(configDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}
	src, binds, err := demonstrated.Compose(art, "an act", []demonstrated.Named{
		{Position: 0, Name: "the graphics tab"},
		{Position: 1, Name: "apply changes"},
	}, time.Now(), marcoChecks{})
	if err != nil {
		t.Fatal(err)
	}
	rt := routes.Route{Slug: routes.Slug(art.Name)}
	if err := demonstrated.Save(routesHere(), rt, src, binds, time.Now()); err != nil {
		t.Fatal(err)
	}
	return art, rt
}

// aPlayableRoutine is two demonstrated interactions, enough to compose an Act from.
func aPlayableRoutine() workflowstore.Artifact {
	control := func(label string, y float64) workflowstore.Control {
		return workflowstore.Control{Label: label,
			Readings: []observe.Region{{X: 0.41, Y: y, Width: 0.07, Height: 0.03}}}
	}
	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: "ark graphics live",
		SavedAt:     time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
		Application: "ArkAscended.exe",
		Screens: []workflowstore.Screen{
			{Label: "screen_a"}, {Label: "screen_b"}},
		Controls: []workflowstore.Control{
			control("control_1", 0.24), control("control_2", 0.29)},
		Steps: []workflowstore.Step{
			{Position: 0, From: "screen_a", To: "screen_b", Control: "control_1",
				Cause: observe.CauseClick, Seen: 2, Outcome: []string{"bloom"}},
			{Position: 1, From: "screen_b", To: "screen_b", Control: "control_2",
				Cause: observe.CauseClick, Seen: 2, Outcome: []string{"master"}},
		},
	}
}

// NAMING BOTH WAYS OF CHOOSING WHAT TO PERFORM IS A USAGE REFUSAL.
//
// Exit 2 rather than 1, because the command was used wrongly rather than pointed at something
// that cannot be performed — and the two send somebody to different places.
func TestPlayAndNameTogetherIsAUsageRefusal(t *testing.T) {
	art, _ := aSavedAct(t)
	if code := runPerformWorkflow([]string{
		"--play", art.Name, "--name", art.Name, "--seconds", "2"}); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

// A PLAY NOBODY SAVED IS REFUSED, AND NOT SEARCHED FOR.
func TestAnUnsavedPlayIsRefusedBeforeAnythingIsAcquired(t *testing.T) {
	aSavedAct(t)
	if code := runPerformWorkflow([]string{
		"--play", "something nobody demonstrated", "--seconds", "2"}); code == 0 {
		t.Fatal("a play nobody saved returned 0")
	}
}

// A SAVED PLAY WHOSE BINDINGS NO LONGER DESCRIBE IT IS REFUSED BY PREFLIGHT.
//
// The binding record is edited and the play left alone, which is the case a digest cannot catch
// on its own — and the run must stop at preflight, with no window acquired and nothing grounded.
func TestAPlayWhoseBindingsNoLongerDescribeItIsRefusedByPreflight(t *testing.T) {
	art, rt := aSavedAct(t)
	src, binds, err := demonstrated.Load(routesHere(), rt)
	if err != nil {
		t.Fatal(err)
	}
	// THE SECOND BINDING REMOVED, THE PLAY AND ITS DIGEST UNTOUCHED.
	binds.Scenes = binds.Scenes[:1]
	raw, err := binds.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(demonstrated.BindingsPath(routesHere(), rt),
		raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := demonstrated.Load(routesHere(), rt); err != nil {
		t.Fatal(err)
	}
	if code := runPerformWorkflow([]string{
		"--play", art.Name, "--seconds", "2"}); code == 0 {
		t.Fatal("a play whose bindings no longer describe it returned 0")
	}
	// AND THE PLAY ITSELF IS UNTOUCHED, because a refusal does not repair anything.
	back, _, _ := demonstrated.Load(routesHere(), rt)
	if back != src {
		t.Fatal("the play was changed by a run that refused it")
	}
}

// AND A PLAY AIMED AT AN APPLICATION IT WAS NOT DEMONSTRATED AGAINST IS REFUSED.
//
// The scope gate is `perform-workflow`'s and it is reached unchanged through `--play`: the
// artifact a play performs comes from its own bindings, so the gate compares the acquired window
// against THAT routine's application. `notepad` stands in for whatever else is on the machine —
// the test must not depend on a game being installed — and the refusal may be the gate or the
// window not being found at all. Both are refusals before any input; what must never happen is a
// run that proceeds.
func TestAPlayAimedAtAnotherApplicationIsRefused(t *testing.T) {
	art, _ := aSavedAct(t)
	code := runPerformWorkflow([]string{
		"--play", art.Name, "--application", "notepad", "--seconds", "2"})
	if code == 0 {
		t.Fatal("a play aimed at another application returned 0")
	}
	// AND NOT A USAGE REFUSAL, which is what makes this a test of the path rather than
	// of the flags. Exit 2 would mean the command never got as far as the play: the
	// first version of this test did exactly that — `--play` alone did not satisfy "name
	// what to perform", so it refused before preflight and the test passed for the wrong
	// reason. Observed now: `no current window matches application notepad`, which is
	// window acquisition, which is past preflight.
	if code == 2 {
		t.Fatal("the run refused on usage, so it never reached the play at all")
	}
}

// AND THE FLAG IS DOCUMENTED WHERE A PERSON LOOKS FOR IT.
func TestThePlayFlagSaysWhatItDoes(t *testing.T) {
	src := readSource(t, "performworkflow.go")
	if !strings.Contains(src, `fs.String("play"`) {
		t.Fatal("--play is not declared on perform-workflow")
	}
	for _, want := range []string{"bindings are resolved", "rather than from --name"} {
		if !strings.Contains(src, want) {
			t.Errorf("the flag's help does not say %q", want)
		}
	}
}

// --play AND --step TOGETHER ARE REFUSED BEFORE PREPARATION.
//
// `--step` extracts one step into a one-step artifact IN MEMORY, which renumbers it to position 0.
// A play's bindings name ORIGINAL positions and nothing renumbers them — so `--play X --step 1`
// left the play's first Scene bound to position 0 while position 0 now held what used to be
// interaction 1. The sentence a person reads would have pressed the other control, with every gate
// satisfied and nothing at all to notice.
//
// Exit 2, and before anything is prepared: no store opened, no window acquired, no screen named,
// nobody asked, nothing pressed.
func TestPlayWithStepIsRefusedOnUsage(t *testing.T) {
	art, _ := aSavedAct(t)
	for _, step := range []string{"0", "1"} {
		code := runPerformWorkflow([]string{
			"--play", art.Name, "--step", step, "--seconds", "2"})
		if code != 2 {
			t.Fatalf("--play --step %s exited %d, want 2 (a usage refusal)",
				step, code)
		}
	}
	// AND --step ALONE STILL WORKS, which is what keeps this a refusal of the
	// combination rather than of the flag.
	if code := runPerformWorkflow([]string{
		"--name", art.Name, "--step", "1", "--application", "notepad",
		"--seconds", "2"}); code == 2 {
		t.Fatal("--step on its own became a usage refusal")
	}
}
