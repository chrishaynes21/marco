package workflowstore_test

// GEOMETRY REPLAY OF A PRESS THAT CHANGED A PANEL AND NOT THE SCREEN.
//
// This is the press the whole route exists for, and it was the one case the layout tests did not
// cover. Ark's GRAPHICS tab replaces the panel under a persistent tab bar: the screen model sees
// no change, both ends of the step are the same screen label, and the step is finished by the
// words that appeared rather than by arriving anywhere.
//
// On the owner's recording it is also where all the good evidence is — GRAPHICS and SOUND each
// match themselves at 0.99999 against a rival at 0.35, while the one step that DID change screen
// was a proposal covering the whole window at 0.73. So "the step with the best evidence in the
// project" and "a step whose screens are the same label" are the same step, and a route that
// handled only transitions would refuse exactly the press worth replaying.
//
// See [[ADR-248-replaying-a-demonstrated-layout]].

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aPanelPressOnAMenu is Ark's GRAPHICS press: one click on the settings screen that changed no
// screen, finished by the words it made appear, in a window of a recorded size.
func aPanelPressOnAMenu(t *testing.T, clicked observe.PointerAt,
	seen ...observe.Region) workflowstore.Artifact {

	t.Helper()
	at := clicked
	d := workflow.Draft{Name: "ark graphics", Steps: []workflow.Step{{
		// BOTH ENDS THE SAME SCREEN, which is what makes this a panel press.
		From: "visual_1", To: "visual_1", Target: "target_2",
		Cause: observe.CauseClick, Seen: 1,
		Outcome:  []string{"bloom", "foliage", "quality", "shadows", "textures"},
		Readings: seen, At: &at,
	}}}
	a, err := workflowstore.Save(d, map[observe.VisualScreenID]observe.VisualArrangement{
		"visual_1": {Cells: map[string]int{"0,0": 9}, Total: 9},
	}, saved)
	if err != nil {
		t.Fatal(err)
	}
	a.Window = &workflowstore.WindowGeometry{Width: 1920, Height: 1080}
	return a
}

// THE PANEL PRESS IS PLANNED, AND IT PRESSES THE PERSON'S CLICK.
func TestAPanelPressIsReplayedAtItsDemonstratedPoint(t *testing.T) {
	// THE OWNER'S OWN NUMBERS: 842,274 of 1920x1080, inside a tab the proposer read
	// as 781,253 to 933,296. The click is left of that rectangle's middle.
	clicked := observe.PointerAt{X: 0.43854166666666666, Y: 0.2537037037037037}
	tab := rect(781.0/1920, 253.0/1080, 152.0/1920, 43.0/1080)

	a := aPanelPressOnAMenu(t, clicked, tab)
	s, now := replaying(t)

	got := a.StepReadinessAt(0, s, now)
	if !got.Ready() {
		t.Fatalf("the press this route exists for is %q: %s", got.Stage, got.Because)
	}
	if got.Action == nil {
		t.Fatal("ready, with nothing to press")
	}
	if got.Action.At != clicked {
		t.Fatalf("the replay would press %+v and the person pressed %+v",
			got.Action.At, clicked)
	}
	// AND NOT THE MIDDLE OF THE TAB, which on this recording is 20px to the right.
	middle := observe.PointerAt{X: tab.X + tab.Width/2, Y: tab.Y + tab.Height/2}
	if got.Action.At == middle {
		t.Error("the replay presses the middle of the rectangle the proposer " +
			"drew, which is not where anybody clicked")
	}
	// AND IT SAYS WHAT IT IS RESTING ON, so no report can call this recognition.
	if got.Action.Support != workflow.SupportDemonstratedLayout {
		t.Errorf("the action rests on %q rather than the demonstrated layout",
			got.Action.Support)
	}
}

// AND A CHANGED WINDOW REFUSES IT, same as any other step.
//
// Asserted on this step specifically because its screens are equal at both ends: an
// implementation that reached the geometry check through a transition's arrival logic could pass
// every transition test and skip the check entirely here.
func TestAPanelPressInAnotherWindowIsRefused(t *testing.T) {
	a := aPanelPressOnAMenu(t, observe.PointerAt{X: 0.4385, Y: 0.2537})
	s, now := replaying(t)
	now.Layout.Window = workflowstore.WindowGeometry{Width: 1280, Height: 720}

	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepLayoutChanged {
		t.Fatalf("a panel press in a 1280x720 window is %q, demonstrated at "+
			"1920x1080: %s", got.Stage, got.Because)
	}
	if got.Action != nil {
		t.Errorf("a changed window was given %+v to press", got.Action.At)
	}
}

// AND A PANEL PRESS WITH NO RECORDED CLICK IS REFUSED, NOT CENTRED.
//
// Every routine recorded before the panel loop kept its point is in this position. The refusal
// has to name the missing click rather than fall back to the tab's middle, which is available
// here and is the wrong point.
func TestAPanelPressWithNoClickIsRefusedRatherThanCentred(t *testing.T) {
	tab := rect(781.0/1920, 253.0/1080, 152.0/1920, 43.0/1080)
	a := aPanelPressOnAMenu(t, observe.PointerAt{X: 0.4385, Y: 0.2537}, tab)
	// THE SHAPE THE OLD RECORDER LEFT: readings kept, click dropped.
	a.Steps[0].At = nil
	s, now := replaying(t)

	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepControlUnlocated {
		t.Fatalf("a panel press with no recorded click is %q: %s",
			got.Stage, got.Because)
	}
	if got.Action != nil {
		t.Fatalf("a point was invented for a press that recorded none: %+v",
			got.Action.At)
	}
}
