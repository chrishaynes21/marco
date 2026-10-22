package workflow_test

// A STEP CARRIES THE POINT THE PERSON CLICKED.
//
// Not the middle of what a proposer drew around that click, and not conditional on a proposer
// having drawn anything at all. The distinction is the whole of the field: a proposal is
// generous on purpose — a live look once matched PLAY with a region running to the window's left
// edge, whose middle landed three pixels inside a button starting at 62 — so the centre of a
// saved rectangle is a different point from the one somebody pressed, and on an off-centre click
// it can be a different control.
//
// It is DEMONSTRATION EVIDENCE here and nothing more. Nothing on the recognition path reads it;
// it becomes something to press only under the fixed-layout licence, in a window of the size it
// was demonstrated in. See [[ADR-248-replaying-a-demonstrated-layout]].

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
)

// THE CLICK IS TAKEN FROM THE PRESS, AND NOT FROM THE REGION AROUND IT.
func TestAStepRecordsThePointThePersonClicked(t *testing.T) {
	clicked := observe.PointerAt{X: 0.1400, Y: 0.1300}
	// A SPRAWLING PROPOSAL whose middle is nowhere near the click.
	sprawl := observe.Region{X: 0.10, Y: 0.10, Width: 0.40, Height: 0.06}

	ev := observe.ClickEvidence{
		Steps: []observe.RouteStep{{From: "visual_1", To: "visual_2",
			Cause: observe.CauseClick, Target: "target_1", Count: 1,
			Looks: []int{60}}},
		Observations: []observe.ClickObservation{
			{Look: 60, Screen: "visual_1", Target: "target_1", At: clicked},
		},
	}
	reg := []observe.TargetRecord{{ID: "target_1", Screen: "visual_1",
		Where: []observe.Region{sprawl}, Presses: 1}}

	d := workflow.Learn("ark graphics", []observe.VisualScreenID{"visual_1", "visual_2"},
		ev, reg)
	if len(d.Steps) != 1 {
		t.Fatalf("%d step(s) learned", len(d.Steps))
	}
	got := d.Steps[0]
	if got.At == nil {
		t.Fatal("the step records no click at all, so a replay would have " +
			"nothing of the person's to repeat")
	}
	if *got.At != clicked {
		t.Fatalf("the step records %+v and the person clicked %+v",
			*got.At, clicked)
	}
	// AND IT IS NOT THE MIDDLE OF THE PROPOSAL, asserted rather than assumed so
	// the fixture cannot stop posing its question.
	middle := observe.PointerAt{
		X: sprawl.X + sprawl.Width/2, Y: sprawl.Y + sprawl.Height/2}
	if *got.At == middle {
		t.Error("the step records the middle of the proposed region, which is " +
			"the segmenter's idea of the control and not the person's click")
	}
}

// AND A PRESS THE PROPOSER FOUND NOTHING UNDER STILL RECORDS WHERE IT WAS.
//
// On a menu no recogniser can name, the segmenter's silence is the ordinary case rather than the
// exception — and that press is exactly the one geometry replay exists to repeat.
func TestAClickWithNoProposalStillRecordsItsPoint(t *testing.T) {
	clicked := observe.PointerAt{X: 0.4239, Y: 0.2574}
	ev := observe.ClickEvidence{
		Steps: []observe.RouteStep{{From: "visual_1", To: "visual_2",
			Cause: observe.CauseClick, Target: "target_1", Count: 1,
			Looks: []int{126}}},
		Observations: []observe.ClickObservation{
			{Look: 126, Screen: "visual_1", Target: "target_1", At: clicked},
		},
	}
	// NO REGISTRY AT ALL: nothing was ever proposed for this target.
	d := workflow.Learn("ark graphics", []observe.VisualScreenID{"visual_1", "visual_2"},
		ev, nil)
	if len(d.Steps) != 1 {
		t.Fatalf("%d step(s) learned", len(d.Steps))
	}
	if len(d.Steps[0].Readings) != 0 {
		t.Fatal("the fixture has readings and is not posing the question")
	}
	if d.Steps[0].At == nil || *d.Steps[0].At != clicked {
		t.Fatalf("a press with no proposal under it recorded %v as its point; "+
			"a proposal must not be what makes a click savable",
			d.Steps[0].At)
	}
}

// AND A CONTROL PRESSED TWICE GIVES EACH STEP ITS OWN CLICK.
//
// Readings merge across the steps that used one control, because two readings are two pieces of
// evidence about one thing. Clicks do not: two presses are two facts, and a step that took the
// other one's point would press where somebody pressed at a different moment in the procedure.
func TestEachStepKeepsItsOwnClick(t *testing.T) {
	first := observe.PointerAt{X: 0.20, Y: 0.30}
	second := observe.PointerAt{X: 0.60, Y: 0.70}
	ev := observe.ClickEvidence{
		Steps: []observe.RouteStep{
			{From: "visual_1", To: "visual_2", Cause: observe.CauseClick,
				Target: "target_1", Count: 1, Looks: []int{10}},
			{From: "visual_2", To: "visual_3", Cause: observe.CauseClick,
				Target: "target_1", Count: 1, Looks: []int{40}},
		},
		Observations: []observe.ClickObservation{
			{Look: 10, Screen: "visual_1", Target: "target_1", At: first},
			{Look: 40, Screen: "visual_2", Target: "target_1", At: second},
		},
	}
	d := workflow.Learn("twice", []observe.VisualScreenID{
		"visual_1", "visual_2", "visual_3"}, ev, nil)
	if len(d.Steps) != 2 {
		t.Fatalf("%d step(s) learned, of two demonstrated", len(d.Steps))
	}
	if d.Steps[0].At == nil || *d.Steps[0].At != first {
		t.Errorf("the first step's click is %v, and it was made at %+v",
			d.Steps[0].At, first)
	}
	if d.Steps[1].At == nil || *d.Steps[1].At != second {
		t.Errorf("the second step's click is %v, and it was made at %+v; a "+
			"control pressed twice is two presses and a step that took the "+
			"other one's point would press at the wrong moment in the "+
			"procedure", d.Steps[1].At, second)
	}
}

// AND A STEP THAT WAS NOT A PRESS HAS NO CLICK.
//
// A screen that changed by itself has nothing to repeat, and a point on it would be a coordinate
// with no act behind it.
func TestAStepThatWasNotAPressHasNoClick(t *testing.T) {
	ev := observe.ClickEvidence{
		Steps: []observe.RouteStep{{From: "visual_1", To: "visual_2",
			Cause: observe.CauseAutomatic, Count: 1}},
		Observations: []observe.ClickObservation{
			{Look: 5, Screen: "visual_1", At: observe.PointerAt{X: 0.5, Y: 0.5}},
		},
	}
	d := workflow.Learn("it opened by itself",
		[]observe.VisualScreenID{"visual_1", "visual_2"}, ev, nil)
	if len(d.Steps) != 1 {
		t.Fatalf("%d step(s) learned", len(d.Steps))
	}
	if d.Steps[0].At != nil {
		t.Errorf("a step the screen took by itself carries a click at %+v",
			*d.Steps[0].At)
	}
}
