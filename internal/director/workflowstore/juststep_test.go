package workflowstore_test

// ONE DEMONSTRATED ACTION, EXTRACTED, WITH ITS STARTING STATE INTACT.
//
//	Do not simply drop step zero and accidentally retain the wrong initial-state assumption.
//
// The trap is specific. A step's `From` is the screen a run must be standing on, and dropping
// the steps before it does not change that — so an extraction that kept the original first
// screen, or dropped the screen the step actually starts on, would describe a route nobody
// demonstrated. Both show up here.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// theArkWalk is the shape the owner's capture learned: a menu press that changed the screen,
// then two panel presses that did not. [[Experiment-088-four-things-between-a-demonstration-and-a-click]].
func theArkWalk() workflowstore.Artifact {
	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: "ark graphics",
		SavedAt:     time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC),
		Application: workflowstore.NormaliseScope("arkascended"),
		Screens: []workflowstore.Screen{
			{Label: "screen_a", Arrangement: observe.VisualArrangement{
				Total: 166, Cells: map[string]int{"r@0,0": 32}}},
			{Label: "screen_b", Arrangement: observe.VisualArrangement{
				Total: 358, Cells: map[string]int{"r@0,0": 65}}},
		},
		Controls: []workflowstore.Control{
			{Label: "control_1", Readings: []observe.Region{
				{X: 0.50, Y: 0.41, Width: 0.04, Height: 0.01}}},
			{Label: "control_2", Readings: []observe.Region{
				{X: 0.30, Y: 0.24, Width: 0.07, Height: 0.03}}},
			{Label: "control_3", Readings: []observe.Region{
				{X: 0.410, Y: 0.241, Width: 0.072, Height: 0.028}},
				Containers: []observe.Region{
					{X: 0.027, Y: 0.230, Width: 0.946, Height: 0.05}}},
		},
		Steps: []workflowstore.Step{
			{Position: 0, From: "screen_a", To: "screen_b", Control: "control_1",
				Cause: observe.CauseClick, Seen: 1},
			{Position: 1, From: "screen_b", To: "screen_b", Control: "control_2",
				Cause: observe.CauseClick, Seen: 1,
				Outcome: []string{"resolution", "upscaler", "window", "vsync",
					"mode"}},
			{Position: 2, From: "screen_b", To: "screen_b", Control: "control_3",
				Cause: observe.CauseClick, Seen: 1,
				Outcome: []string{"bloom", "foliage", "presets", "shadows",
					"textures"}},
		},
		Clarifications: []workflowstore.Answer{},
	}
}

// THE EXTRACTED STEP CARRIES THE SCREEN IT ACTUALLY STARTS ON, NOT THE ROUTINE'S FIRST ONE.
func TestExtractingOneStepKeepsItsOwnStartingScreen(t *testing.T) {
	a := theArkWalk()
	one, err := a.JustStep(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(one.Steps) != 1 {
		t.Fatalf("%d step(s) in a one-step extraction", len(one.Steps))
	}
	st := one.Steps[0]
	if st.Control != "control_3" {
		t.Fatalf("the extraction presses %q", st.Control)
	}
	if st.Position != 0 {
		t.Errorf("the only step is at position %d", st.Position)
	}
	// THE STARTING SCREEN IS screen_b, which is where step 2 begins — NOT screen_a,
	// which is where the routine began and which this file must not imply.
	if st.From != "screen_b" {
		t.Errorf("the extracted step starts on %q", st.From)
	}
	if len(one.Screens) != 1 || one.Screens[0].Label != "screen_b" {
		var got []string
		for _, s := range one.Screens {
			got = append(got, s.Label)
		}
		t.Fatalf("the extraction carries screens %v; it needs screen_b and must "+
			"not carry screen_a, which no step of it visits", got)
	}
	// AND THAT SCREEN'S EVIDENCE TRAVELS, or nothing could establish the starting
	// state and the step would be unplannable.
	if !one.Screens[0].Arrangement.Observed() {
		t.Error("the starting screen carries no arrangement, so nothing could " +
			"establish that the run is in the right place")
	}
	// AND ONLY THE CONTROL IT PRESSES.
	if len(one.Controls) != 1 || one.Controls[0].Label != "control_3" {
		t.Fatalf("the extraction carries %d control(s)", len(one.Controls))
	}
	if len(one.Controls[0].Readings) == 0 || len(one.Controls[0].Containers) == 0 {
		t.Error("the action-site evidence did not travel: a control with no readings " +
			"cannot be re-acquired, and with no containers the outcome has no " +
			"region it may not be read from")
	}
	// AND THE OUTCOME EXPECTATION, which is the only way a same-screen step finishes.
	if len(st.Outcome) != 5 {
		t.Errorf("the outcome expectation is %v", st.Outcome)
	}
}

// AND THE PROVENANCE TRAVELS: scope, instant, and a name that says what it is.
func TestAnExtractionKeepsItsScopeAndItsLinkToTheDemonstration(t *testing.T) {
	a := theArkWalk()
	one, err := a.JustStep(2)
	if err != nil {
		t.Fatal(err)
	}
	// THE SAME APPLICATION. Narrowing a routine cannot widen where it may act, and an
	// extraction that lost its scope would be runnable nowhere.
	if ok, why := one.ScopeAllows("arkascended"); !ok {
		t.Errorf("the extraction is not valid for the application it was shown in: %s",
			why)
	}
	if ok, _ := one.ScopeAllows("solasta"); ok {
		t.Error("the extraction is valid for an application it was not shown in")
	}
	// THE SAME INSTANT, so the raw demonstration is still findable beside it.
	if !one.SavedAt.Equal(a.SavedAt) {
		t.Errorf("the extraction is stamped %v and the demonstration %v",
			one.SavedAt, a.SavedAt)
	}
	// AND A NAME THAT DISTINGUISHES IT, because two files under one name is how
	// somebody runs the wrong one.
	if one.Name == a.Name {
		t.Error("the extraction has the same name as the routine it came from")
	}
	if !strings.Contains(one.Name, "step 2") {
		t.Errorf("the name does not say which step it is: %q", one.Name)
	}
	// AND IT SAYS THE EARLIER STEPS ARE NOT IMPLIED.
	if len(one.Notes) == 0 ||
		!strings.Contains(strings.Join(one.Notes, " "), "are not in this file") {
		t.Errorf("the extraction does not say that the steps before it are absent: %v",
			one.Notes)
	}
	// AND IT IS AN ORDINARY ARTIFACT: it marshals, and it loads.
	body, err := one.Marshal()
	if err != nil {
		t.Fatalf("the extraction is not a valid artifact: %v", err)
	}
	back, err := workflowstore.Unmarshal(body)
	if err != nil {
		t.Fatalf("the extraction does not load: %v", err)
	}
	if len(back.Steps) != 1 || back.Steps[0].Control != "control_3" {
		t.Errorf("the reloaded extraction has %d step(s)", len(back.Steps))
	}
	// AND NO PRESS COORDINATE APPEARED, which no extraction may add.
	if strings.Contains(string(body), `"at"`) {
		t.Errorf("the extraction carries a press coordinate:\n%s", body)
	}
}

// AND A STEP WHOSE TWO ENDS DIFFER CARRIES BOTH SCREENS.
func TestExtractingAStepThatChangesScreenCarriesBothEnds(t *testing.T) {
	one, err := theArkWalk().JustStep(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(one.Screens) != 2 {
		var got []string
		for _, s := range one.Screens {
			got = append(got, s.Label)
		}
		t.Fatalf("a step from screen_a to screen_b carries screens %v", got)
	}
	if _, err := one.Marshal(); err != nil {
		t.Fatalf("the extraction does not marshal: %v", err)
	}
}

// AND A STEP NOBODY HAS IS REFUSED.
func TestExtractingAStepThatDoesNotExistIsRefused(t *testing.T) {
	a := theArkWalk()
	for _, n := range []int{-1, 3, 99} {
		if _, err := a.JustStep(n); err == nil {
			t.Errorf("step %d was extracted from a workflow of 3", n)
		} else if !strings.Contains(err.Error(), "which has 3") {
			t.Errorf("the refusal does not say how many there are: %v", err)
		}
	}
}
