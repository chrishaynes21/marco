package demo_test

// WHAT A PIXEL ROUTINE IS TOLD WHEN THE ORDINARY PATH CANNOT RUN IT.
//
// Reported live by the owner, who asked the overlay to run a routine they had demonstrated an
// hour earlier and was told Marco only understands click, focus, move and text editing. The
// routine was fine. The ordinary execution path acts on VERBS, a demonstration from pixels
// carries none, and the refusal named the limits of the path instead of what the routine can do.
//
// The same shape as the defect [[ADR-239-running-what-you-demonstrated-by-the-name-you-gave-it]]
// closed in `director sequences`: the product denying a capability it has.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aPixelRoutine is what `demonstrate-visual` saves: visual evidence, and no verb anywhere.
func aPixelRoutine(name string) workflowstore.Artifact {
	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: name,
		SavedAt:     time.Date(2026, 10, 18, 22, 13, 0, 0, time.UTC),
		Application: workflowstore.NormaliseScope("arkascended"),
		Screens: []workflowstore.Screen{
			{Label: "screen_a"}, {Label: "screen_b"}},
		Controls: []workflowstore.Control{{
			Label: "control_1",
			Readings: []observe.Region{
				{X: 0.027, Y: 0.229, Width: 0.946, Height: 0.050}},
			Text: []string{"AUDIO GRAPHICS"}}},
		Steps: []workflowstore.Step{
			// THE FIRST STEP IS THE SCREEN CHANGING ON ITS OWN, which is what the
			// owner's routine actually began with and is why the refusal landed on
			// step 0 rather than on the step that was clicked.
			{Position: 0, From: "screen_a", To: "screen_b", Cause: observe.CauseAutomatic,
				Seen: 1},
			{Position: 1, From: "screen_b", To: "screen_b", Control: "control_1",
				Cause: observe.CauseClick, Seen: 1},
		},
	}
}

// TestAPixelRoutineIsToldHowItCanBeRun.
func TestAPixelRoutineIsToldHowItCanBeRun(t *testing.T) {
	_, err := demo.Directives(aPixelRoutine("ark graphics live"))
	if err == nil {
		t.Fatal("the ordinary execution path accepted a routine with no verbs")
	}
	said := err.Error()
	// IT SAYS WHAT CAN BE DONE, naming a command that exists and the routine by the
	// name the person gave it.
	if !strings.Contains(said, "perform-workflow --name \"ark graphics live\"") {
		t.Errorf("the refusal does not say how to run it:\n%s", said)
	}
	if !strings.Contains(said, "demonstrated from pixels") {
		t.Errorf("the refusal does not say why this path cannot:\n%s", said)
	}
	// AND IT DOES NOT BLAME THE ROUTINE. "says nothing to do" is true of a recorded
	// transition and false of a demonstration somebody made with their hand.
	if strings.Contains(said, "says nothing to do") {
		t.Errorf("a demonstrated routine was described as saying nothing to do:\n%s",
			said)
	}
}

// TestARoutineWithNoVisualEvidenceEitherStillSaysSo.
//
// The other half, kept: a step that is a recorded transition and carries no visual target is
// not runnable by sight either, and must not be offered to a command that would refuse it.
func TestARoutineWithNoVisualEvidenceEitherStillSaysSo(t *testing.T) {
	a := aPixelRoutine("a bare walk")
	a.Controls = nil
	steps := make([]workflowstore.Step, len(a.Steps))
	copy(steps, a.Steps)
	steps[1].Control = ""
	a.Steps = steps

	_, err := demo.Directives(a)
	if err == nil {
		t.Fatal("a routine with neither verbs nor visual evidence was accepted")
	}
	if strings.Contains(err.Error(), "perform-workflow") {
		t.Errorf("a routine with no visual target was sent to the pixel path:\n%s",
			err)
	}
	if !strings.Contains(err.Error(), "says nothing to do") {
		t.Errorf("the honest refusal for a recorded transition was lost:\n%s", err)
	}
}
