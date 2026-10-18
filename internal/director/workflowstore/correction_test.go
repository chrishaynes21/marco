package workflowstore_test

// A MISTAKE CAN BE TAKEN OUT OF THE PROCEDURE WITHOUT BEING TAKEN OFF THE RECORD.
//
//	The raw evidence must remain unchanged after correction. The learned procedure must
//	reflect the correction.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

func aThreeStepRoutine() workflowstore.Artifact {
	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: "ark graphics",
		SavedAt: time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC),
		// THE SCREENS AND CONTROLS A STEP MAY NAME. `Marshal` refuses a step naming a
		// screen the artifact does not describe, which is how a correction that removed
		// a screen's last step would be caught — it does not, deliberately: a screen
		// with no step is still evidence of where the routine goes.
		Screens: []workflowstore.Screen{
			{Label: "menu"}, {Label: "settings"}, {Label: "graphics"},
		},
		Controls: []workflowstore.Control{
			{Label: "control_1", Readings: []observe.Region{
				{X: 0.4, Y: 0.4, Width: 0.1, Height: 0.05}}},
			{Label: "control_2", Readings: []observe.Region{
				{X: 0.4, Y: 0.5, Width: 0.1, Height: 0.05}}},
			{Label: "control_3", Readings: []observe.Region{
				{X: 0.410, Y: 0.241, Width: 0.072, Height: 0.028}}},
		},
		Steps: []workflowstore.Step{
			{Position: 0, From: "menu", To: "settings", Control: "control_1",
				Cause: observe.CauseClick, Seen: 1},
			{Position: 1, From: "settings", To: "settings", Control: "control_2",
				Cause: observe.CauseClick, Seen: 1},
			{Position: 2, From: "settings", To: "graphics", Control: "control_3",
				Cause: observe.CauseClick, Seen: 2},
		},
	}
}

// THE CORRECTION REMOVES THE STEP AND KEEPS IT.
func TestAnAccidentalStepComesOutOfTheProcedureAndStaysOnTheRecord(t *testing.T) {
	a := aThreeStepRoutine()
	at := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)

	out, err := a.Dropping(2, "that click was accidental", at)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Steps) != 2 {
		t.Fatalf("%d step(s) after dropping one of three", len(out.Steps))
	}
	for _, s := range out.Steps {
		if s.Control == "control_2" {
			t.Error("the corrected procedure still presses the control the person " +
				"said was a mistake")
		}
	}
	// AND THE STEP ITSELF IS STILL THERE, with their words and the moment.
	if len(out.Corrections) != 1 {
		t.Fatalf("%d correction(s) recorded", len(out.Corrections))
	}
	c := out.Corrections[0]
	if c.Dropped != 2 || c.Step.Control != "control_2" {
		t.Errorf("the correction records step %d / %q", c.Dropped, c.Step.Control)
	}
	if c.Said != "that click was accidental" {
		t.Errorf("the person's words came back as %q", c.Said)
	}
	if !c.At.Equal(at) {
		t.Errorf("the correction is stamped %v", c.At)
	}
	// AND THE LINK TO THE DEMONSTRATION SURVIVES, which is the whole point: a correction
	// that restamped SavedAt would orphan the raw record it is a correction OF.
	if !out.SavedAt.Equal(a.SavedAt) {
		t.Errorf("the correction restamped the demonstration: %v, was %v",
			out.SavedAt, a.SavedAt)
	}
	if !out.CorrectedAt.Equal(at) {
		t.Errorf("the correction is not dated separately: %v", out.CorrectedAt)
	}
	e := aDemonstration(a.Name, a.SavedAt)
	if !e.Describes(out) {
		t.Error("the raw demonstration no longer describes the corrected procedure")
	}
	// AND THE ORIGINAL IS UNTOUCHED, because the caller holds what is on disk.
	if len(a.Steps) != 3 || len(a.Corrections) != 0 {
		t.Errorf("the artifact passed in was modified: %d step(s), %d correction(s)",
			len(a.Steps), len(a.Corrections))
	}
}

// A CORRECTED PROCEDURE IS CORRECT FOR A READER THAT KNOWS NOTHING ABOUT CORRECTIONS.
//
// This is the test that justifies not bumping the artifact version. Version 3's rule is that a
// reader ignoring a new field would perform something different from what the file describes —
// and it does not apply, because the dropped step is GONE from `Steps`. If a correction ever
// became something a performer has to consult, this fails, and the version has to move.
func TestACorrectedProcedureIsCorrectForAReaderThatIgnoresCorrections(t *testing.T) {
	out, err := aThreeStepRoutine().Dropping(2, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	body, err := out.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := workflowstore.Unmarshal(body)
	if err != nil {
		t.Fatal(err)
	}
	// THE VERSION DID NOT MOVE, so every artifact already on disk still loads.
	if back.Version != workflowstore.Version {
		t.Errorf("a corrected artifact is version %d and this build writes %d",
			back.Version, workflowstore.Version)
	}
	// AND THE STEPS ALONE EXPRESS THE CORRECTION.
	for _, s := range back.Steps {
		if s.Control == "control_2" {
			t.Fatal("the corrected step is still in Steps, so a reader that " +
				"ignores Corrections would perform it — this needs a version bump")
		}
	}
	if len(back.Steps) != 2 {
		t.Errorf("%d step(s) came back of 2", len(back.Steps))
	}
}

// "REDO THE LAST STEP" IS A STEP NUMBER THE PERSON DOES NOT HAVE TO COUNT.
func TestTheLastStepCanBeDroppedWithoutCountingTheSteps(t *testing.T) {
	a := aThreeStepRoutine()
	out, err := a.Dropping(len(a.Steps), "redo the last step", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Steps) != 2 || out.Steps[1].Control != "control_2" {
		t.Errorf("dropping the last step left %d step(s) ending at %q",
			len(out.Steps), out.Steps[len(out.Steps)-1].Control)
	}
	if out.Corrections[0].Step.Control != "control_3" {
		t.Errorf("the correction kept %q", out.Corrections[0].Step.Control)
	}
}

// A STEP NOBODY HAS IS REFUSED WITH ITS OWN SENTENCE, never clamped.
//
// "Ignore step 4" about a three-step routine is a person describing something Marco should go
// and look at. Rounding it down to step 3 would remove a step they did not mean and look like
// a working feature.
func TestAStepThatDoesNotExistIsRefusedRatherThanRoundedDown(t *testing.T) {
	a := aThreeStepRoutine()
	for _, n := range []int{0, -1, 4, 99} {
		out, err := a.Dropping(n, "", time.Now())
		if err == nil {
			t.Errorf("dropping step %d was accepted and left %d step(s)",
				n, len(out.Steps))
			continue
		}
		if !strings.Contains(err.Error(), "has 3") {
			t.Errorf("the refusal for step %d does not say how many there are: %v",
				n, err)
		}
	}
	// AND NOTHING CHANGED.
	if len(a.Steps) != 3 {
		t.Errorf("a refused correction changed the artifact")
	}
}

// TWO CORRECTIONS ARE TWO CORRECTIONS, and the second numbers the steps as they now are.
//
// The numbering has to be what the person last SAW, because that is what they are referring
// to. A correction that kept the original numbering would remove a different step the second
// time and be impossible to predict.
func TestASecondCorrectionNumbersTheStepsAsTheyNowAre(t *testing.T) {
	first, err := aThreeStepRoutine().Dropping(1, "wrong click", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	second, err := first.Dropping(1, "and that one", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Steps) != 1 || second.Steps[0].Control != "control_3" {
		t.Fatalf("%d step(s) left, ending at %q", len(second.Steps),
			second.Steps[0].Control)
	}
	if len(second.Corrections) != 2 {
		t.Fatalf("%d correction(s) after two", len(second.Corrections))
	}
	// THE TWO ARE DIFFERENT STEPS, which is what proves the renumbering happened.
	if second.Corrections[0].Step.Control == second.Corrections[1].Step.Control {
		t.Errorf("both corrections removed %q", second.Corrections[0].Step.Control)
	}
	// AND THE FIRST CORRECTION SURVIVED THE SECOND.
	if second.Corrections[0].Said != "wrong click" {
		t.Errorf("the first correction now reads %q", second.Corrections[0].Said)
	}
	if !first.Corrected() || len(first.Corrections) != 1 {
		t.Error("the artifact the second correction was made from was modified")
	}
}

// AND A REASON LONG ENOUGH TO BE A DOCUMENT IS BOUNDED, like every other free text here.
func TestAReasonIsBoundedLikeEveryOtherFreeTextField(t *testing.T) {
	long := strings.Repeat("why ", 400)
	out, err := aThreeStepRoutine().Dropping(1, long, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(out.Corrections[0].Said)); n != workflowstore.MaxCorrectionWords {
		t.Errorf("a %d-rune reason was stored as %d runes, bound is %d",
			len([]rune(long)), n, workflowstore.MaxCorrectionWords)
	}
}

// ── THE LABELS THE RAW RECORD BORROWS FROM THE ROUTINE ──

// AN AUTOMATIC TRANSITION MINTS NO CONTROL, SO IT MUST NOT CONSUME A NUMBER EITHER.
//
// `ControlLabels` exists so the raw demonstration beside a routine can record each press by the
// routine's own label rather than by a session identity. It is only worth having if it allocates
// the SAME labels `SaveWith` does — a record that called the second control `control_3` while
// the routine called it `control_2` would point at the wrong entry, in a file whose whole job is
// to be readable beside the other one.
//
// A mutation run found this unasserted: skipping the targetless step could be deleted and
// everything still passed.
func TestAnAutomaticTransitionDoesNotConsumeAControlNumber(t *testing.T) {
	d := workflow.Draft{Name: "ark graphics", Steps: []workflow.Step{
		{From: "visual_1", To: "visual_2", Cause: observe.CauseClick, Target: "target_1",
			Seen: 1},
		// THE SCREEN CHANGING BY ITSELF, between two clicks. It has no target, so it
		// is no control, and the click after it is the SECOND control.
		{From: "visual_2", To: "visual_3", Cause: observe.CauseAutomatic, Seen: 1},
		{From: "visual_3", To: "visual_4", Cause: observe.CauseClick, Target: "target_9",
			Seen: 2},
	}}
	got := workflowstore.ControlLabels(d)
	if len(got) != 2 {
		t.Fatalf("%d control label(s) for two clicked steps: %v", len(got), got)
	}
	if got["target_1"] != "control_1" {
		t.Errorf("the first control is %q", got["target_1"])
	}
	if got["target_9"] != "control_2" {
		t.Errorf("the second control is %q, and the routine will call it control_2 — "+
			"a record that disagrees points at the wrong entry", got["target_9"])
	}

	// AND IT IS THE SAME ALLOCATION THE ARTIFACT MAKES, which is the only thing that
	// makes the function worth having. Asserted against the saved artifact rather than
	// against a second copy of the rule.
	// WITH NO ARRANGEMENTS, which `SaveWith` allows: a screen with none is saved as a
	// screen nothing can bind, and this test is about the LABELS. Passing arrangements
	// here would also name a session-local visual type in a file the durable-memory
	// walk reads, which is a guard worth not widening for a convenience.
	a, err := workflowstore.SaveWith(d, nil, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	clicked := []string{}
	for _, s := range a.Steps {
		if s.Control != "" {
			clicked = append(clicked, s.Control)
		}
	}
	if len(clicked) != 2 || clicked[0] != got["target_1"] || clicked[1] != got["target_9"] {
		t.Errorf("the artifact labels its controls %v and ControlLabels says %v",
			clicked, got)
	}

	// AND A DRAFT WITH NO CLICKED STEP HAS NO CONTROLS, rather than one for the walk.
	none := workflowstore.ControlLabels(workflow.Draft{Name: "keys only",
		Steps: []workflow.Step{{From: "a", To: "b", Cause: observe.CauseUnknown, Seen: 1}}})
	if len(none) != 0 {
		t.Errorf("a draft with nothing pressed has %d control label(s): %v",
			len(none), none)
	}
}
