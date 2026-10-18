package workflowstore_test

// A VERIFIED OUTCOME IS THE EXPECTED ONE, NOT ANY CHANGE.
//
// The durable half of [[Experiment-087-the-panel-changed-and-the-verifier-could-say-so]]. The
// four terms live in `observe`; what is asserted here is what makes this a verifier rather than
// a change detector: the fresh evidence has to OVERLAP what the demonstration recorded.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// theGraphicsStep is one step that pressed a tab and changed the panel under it.
//
// Ark's geometry, measured: the GRAPHICS tab at 7.2%x2.8% inside a 94.6%x5.0% tab bar, and the
// outcome words are the ones the capture actually produced.
func theGraphicsStep() workflowstore.Artifact {
	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: "ark graphics",
		SavedAt: time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC),
		Screens: []workflowstore.Screen{{Label: "screen_a"}},
		Controls: []workflowstore.Control{{Label: "control_1",
			Readings: []observe.Region{
				{X: 0.410, Y: 0.241, Width: 0.072, Height: 0.028}},
			Containers: []observe.Region{
				{X: 0.027, Y: 0.230, Width: 0.946, Height: 0.050}}}},
		Steps: []workflowstore.Step{{
			Position: 0, From: "screen_a", To: "screen_a", Control: "control_1",
			Cause: observe.CauseClick, Seen: 1,
			Outcome: []string{"bloom", "blur", "brightness", "decals", "distance",
				"foliage", "presets", "shadows", "textures"},
		}},
	}
}

// panel is N readings of some words below the tab bar.
func panel(n int, words ...string) [][]observe.ReadWord {
	var read []observe.ReadWord
	for i, w := range words {
		read = append(read, observe.ReadWord{Text: w, Where: observe.Region{
			X: 0.10, Y: 0.35 + float64(i)*0.03, Width: 0.20, Height: 0.025}})
	}
	out := make([][]observe.ReadWord, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, read)
	}
	return out
}

// THE EXPECTED PANEL ARRIVES AND THE OUTCOME HOLDS.
func TestAStepsOwnOutcomeIsWhatVerifiesIt(t *testing.T) {
	a := theGraphicsStep()
	before := panel(5, "resolution", "window", "mode", "upscaler", "vsync")
	after := panel(5, "presets", "textures", "shadows", "bloom", "foliage", "distance")

	held, why := a.OutcomeHeld(0, before, after, 54, 0)
	if !held {
		t.Fatalf("the demonstrated panel arrived and the outcome did not hold: %s", why)
	}
	if !strings.Contains(why, "where the control is not") {
		t.Errorf("the reason does not say what was checked: %s", why)
	}
}

// A DIFFERENT PANEL IS A CHANGE AND NOT THE EXPECTED DESTINATION.
//
// The term that makes this a verifier. A wrong tab produces plenty of words that appeared and
// stayed, and accepting those would be verifying that something happened.
func TestADifferentPanelIsNotTheExpectedDestination(t *testing.T) {
	a := theGraphicsStep()
	before := panel(5, "presets", "textures", "shadows", "bloom", "foliage")
	// THE AUDIO TAB'S PANEL: five words, all persistent, none of them expected.
	after := panel(5, "master", "music", "voice", "ambient", "sfx", "volume")

	held, why := a.OutcomeHeld(0, before, after, 54, 0)
	if held {
		t.Fatalf("a different panel verified the step's outcome: %s", why)
	}
	if !strings.Contains(why, "what appeared instead was") {
		t.Errorf("the refusal does not say what DID appear: %s", why)
	}
	// AND IT NAMES HOW MANY OF THE EXPECTED WORDS IT SAW, which is zero.
	if !strings.Contains(why, "0 of the 9") {
		t.Errorf("the refusal does not count the expected words: %s", why)
	}
}

// A SCREEN THAT CHANGES BY ITSELF IS REFUSED BEFORE ITS EVIDENCE IS READ.
func TestAScreenThatMovesByItselfRefusesTheOutcome(t *testing.T) {
	a := theGraphicsStep()
	before := panel(5, "resolution", "window", "mode")
	after := panel(5, "presets", "textures", "shadows", "bloom", "foliage", "distance")

	// THE SAME EVIDENCE that verified above, on a screen measured to move by itself.
	held, why := a.OutcomeHeld(0, before, after, 46, 3)
	if held {
		t.Fatalf("a screen that moved in 3 of 46 quiet windows verified an outcome: %s",
			why)
	}
	if !strings.Contains(why, "changed by itself in 3 of 46") {
		t.Errorf("the refusal does not say which measurement refused it: %s", why)
	}
}

// A STEP DEMONSTRATED WITHOUT AN OUTCOME SAYS SO, rather than refusing as though it disagreed.
//
// Every artifact saved before outcomes existed is one. Missing evidence is not contradictory
// evidence, and a reader has to be able to tell them apart.
func TestAStepWithNoRecordedOutcomeIsUnaskedAndNotRefused(t *testing.T) {
	a := theGraphicsStep()
	a.Steps[0].Outcome = nil
	held, why := a.OutcomeHeld(0, panel(5, "a"), panel(5, "b"), 54, 0)
	if held {
		t.Error("a step with no recorded outcome verified one")
	}
	if !strings.Contains(why, "without an outcome") {
		t.Errorf("the reason reads as a disagreement rather than as an absence: %s", why)
	}
	if _, why := a.OutcomeHeld(7, nil, nil, 0, 0); !strings.Contains(why, "not in a workflow") {
		t.Errorf("a step nobody has is not refused as one: %s", why)
	}
}

// AND THE EXCLUDED REGION IS THE ONE THE ARTIFACT ALREADY SAVED.
//
// No new geometry: `Readings` and `Containers` have been on a control since it existed, kept
// for the reacquisition question, and they are the right answer to this one. A word read inside
// the tab bar cannot become an outcome, so the tab's own label cannot verify the tab's press.
func TestTheOutcomeCannotBeReadFromTheControlThatWasPressed(t *testing.T) {
	a := theGraphicsStep()
	if got := a.OutcomeExcluded(0); len(got) != 2 {
		t.Fatalf("%d excluded region(s), want the control's reading and its container",
			len(got))
	}
	// A PRESS WHOSE "OUTCOME" IS THE TAB BAR'S OWN LABELS, which is the circular case.
	inBar := func(n int, words ...string) [][]observe.ReadWord {
		var read []observe.ReadWord
		for i, w := range words {
			read = append(read, observe.ReadWord{Text: w, Where: observe.Region{
				X: 0.05 + float64(i)*0.09, Y: 0.245,
				Width: 0.07, Height: 0.02}})
		}
		out := make([][]observe.ReadWord, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, read)
		}
		return out
	}
	a.Steps[0].Outcome = []string{"presets", "textures", "shadows", "bloom", "foliage"}
	before := inBar(5, "audio", "video")
	after := inBar(5, "audio", "video", "presets", "textures", "shadows", "bloom", "foliage")

	held, why := a.OutcomeHeld(0, before, after, 54, 0)
	if held {
		t.Fatalf("words read INSIDE the pressed control verified its own outcome: %s",
			why)
	}
	if !strings.Contains(why, "0 of the 5") {
		t.Errorf("the refusal does not say that none of the expected words were seen "+
			"outside the control: %s", why)
	}
}

// AND THE SAVE CARRIES IT, asserted from THIS package.
//
// A mutation run found this unguarded from here: deleting the field from what `SaveWith` writes
// survived, because the only assertion lived in `workflow`'s tests and the gate runs one
// package at a time. A saved step with no outcome is a step a replay can never verify, and it
// would look exactly like a step demonstrated before outcomes existed.
func TestTheSaveCarriesTheDemonstratedOutcome(t *testing.T) {
	want := []string{"bloom", "foliage", "presets", "shadows", "textures"}
	d := workflow.Draft{Name: "ark graphics", Steps: []workflow.Step{{
		From: "visual_1", To: "visual_1", Cause: observe.CauseClick,
		Target: "target_1", Seen: 1, Outcome: want,
	}}}
	a, err := workflowstore.SaveWith(d, nil, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Steps) != 1 {
		t.Fatalf("%d step(s) saved", len(a.Steps))
	}
	if len(a.Steps[0].Outcome) != len(want) {
		t.Fatalf("the saved step's outcome is %v, want %v", a.Steps[0].Outcome, want)
	}
	// AND IT SURVIVES A ROUND TRIP, because a replay loads from bytes.
	body, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := workflowstore.Unmarshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Steps[0].Outcome) != len(want) {
		t.Errorf("the outcome came back as %v", back.Steps[0].Outcome)
	}
	// AND IT IS WORDS ONLY: no screen, no target, no rectangle, so the thing the
	// durable-memory walk exists to stop cannot travel inside an outcome.
	for _, forbidden := range []string{"visual_", "target_", "\"look\""} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("the saved outcome carries %q:\n%s", forbidden, body)
		}
	}
	// AND THE VERIFIER CAN USE WHAT THE SAVE KEPT, which is the join that matters.
	held, why := back.OutcomeHeld(0, panel(5, "resolution", "window"),
		panel(5, "presets", "textures", "shadows", "bloom", "foliage"), 54, 0)
	if !held {
		t.Errorf("a saved outcome could not be verified by fresh readings of it: %s",
			why)
	}
}
