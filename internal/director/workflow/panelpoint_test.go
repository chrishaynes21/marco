package workflow_test

// A PRESS THAT CHANGED A PANEL AND NOT THE SCREEN KEEPS ITS CLICK TOO.
//
// Such a press is the ORDINARY case on the menus geometry replay exists for — a tab bar that
// stays put while the panel under it is replaced — and on the owner's Ark recording it is the
// majority of the procedure and all of the good evidence: GRAPHICS and SOUND each match
// themselves at 0.99999 against a rival at 0.35, while the one step that changed screen was a
// proposal covering the whole window.
//
// The first version of the demonstrated point reached only the walk steps. Panel steps are
// assembled by a second loop (`panelSteps`) and carried none, so every press with a verified
// local outcome was refused by the demonstrated-layout route with "a point will not be invented
// from the rectangle something drew around it" — about a press whose point had been observed and
// thrown away. Found on the owner's second recording, by reading it rather than the suite.
//
// Driven through `Learn`, which is what the recorder calls. A fixture built over `panelSteps`
// directly would have agreed with whichever loop it was written against, which is the shape of
// mistake this file already exists to catch. See [[ADR-248-replaying-a-demonstrated-layout]].

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
)

// arkPanelPress is the owner's GRAPHICS press, in the shape the observer files it: a press on
// the settings screen that changed no screen, with words that appeared and stood.
func arkPanelPress(clicked observe.PointerAt, look int) observe.ClickEvidence {
	return observe.ClickEvidence{
		// NO TRANSITION AT ALL, which is what made this press inert.
		Inert: []observe.ClickObservation{
			{Look: look, Screen: "visual_3", Target: "target_2", At: clicked},
		},
		Observations: []observe.ClickObservation{
			{Look: look, Screen: "visual_3", Target: "target_2", At: clicked},
		},
		Outcomes: []observe.LocalOutcome{{
			Target: "target_2", Screen: "visual_3", Look: look,
			Appeared:    []string{"bloom", "foliage", "quality", "shadows", "textures"},
			Settled:     true,
			BeforeLooks: 3, AfterLooks: 3,
		}},
	}
}

// THE PANEL PRESS CARRIES THE POINT IT WAS MADE AT.
func TestAPanelPressRecordsThePointThePersonClicked(t *testing.T) {
	clicked := observe.PointerAt{X: 0.4385, Y: 0.2537} // 842,274 of 1920x1080
	ev := arkPanelPress(clicked, 274)

	d := workflow.Learn("ark graphics", []observe.VisualScreenID{"visual_3"}, ev, nil)
	if len(d.Steps) != 1 {
		t.Fatalf("%d step(s) learned from one verified panel press", len(d.Steps))
	}
	got := d.Steps[0]
	// THE FIXTURE IS POSING THE QUESTION: this is the panel step and not a walk step.
	if got.From != got.To {
		t.Fatalf("the learned step goes %s to %s, so it is not the "+
			"changed-no-screen press this is about", got.From, got.To)
	}
	if got.At == nil {
		t.Fatal("a press that changed a panel records no click, so the " +
			"demonstrated-layout route refuses it for having no point — " +
			"about a press whose point was observed and dropped")
	}
	if *got.At != clicked {
		t.Fatalf("the step records %+v and the person clicked %+v",
			*got.At, clicked)
	}
}

// AND IT IS THE PRESS'S OWN POINT, NOT ANOTHER PRESS OF THE SAME CONTROL.
//
// The owner's recording presses GRAPHICS twice, once on the way in and once on the way back from
// SOUND. Two presses are two facts; a step that took the other's point would press where
// somebody pressed at a different moment in the procedure. This is the panel-loop half of
// TestEachStepKeepsItsOwnClick, which held the same line for walk steps.
func TestEachPanelPressKeepsItsOwnClick(t *testing.T) {
	first := observe.PointerAt{X: 0.4385, Y: 0.2537}
	again := observe.PointerAt{X: 0.4479, Y: 0.2611} // the same tab, pressed elsewhere
	ev := arkPanelPress(first, 274)
	ev.Inert = append(ev.Inert, observe.ClickObservation{
		Look: 402, Screen: "visual_3", Target: "target_2", At: again})
	ev.Observations = append(ev.Observations, observe.ClickObservation{
		Look: 402, Screen: "visual_3", Target: "target_2", At: again})

	d := workflow.Learn("ark graphics", []observe.VisualScreenID{"visual_3"}, ev, nil)
	if len(d.Steps) != 2 {
		t.Fatalf("%d step(s) learned, of two presses demonstrated", len(d.Steps))
	}
	if d.Steps[0].At == nil || *d.Steps[0].At != first {
		t.Errorf("the first press's click is %v and it was made at %+v",
			d.Steps[0].At, first)
	}
	if d.Steps[1].At == nil || *d.Steps[1].At != again {
		t.Errorf("the second press's click is %v and it was made at %+v; one "+
			"control pressed twice is two presses, and the second step "+
			"pressing the first point would act at the wrong moment",
			d.Steps[1].At, again)
	}
}

// AND A PANEL PRESS WITH NO PROPOSAL UNDER IT STILL KEEPS ITS POINT.
//
// Same requirement as the walk half, asserted separately because the two loops consult the
// registry at different moments: a proposal must not be what makes a click savable, and on a
// translucent menu the proposer's silence is the ordinary case.
func TestAPanelPressWithNoReadingsStillRecordsItsPoint(t *testing.T) {
	clicked := observe.PointerAt{X: 0.2583, Y: 0.2620}
	ev := arkPanelPress(clicked, 283)

	d := workflow.Learn("ark graphics", []observe.VisualScreenID{"visual_3"}, ev, nil)
	if len(d.Steps) != 1 {
		t.Fatalf("%d step(s) learned", len(d.Steps))
	}
	if len(d.Steps[0].Readings) != 0 {
		t.Fatal("the fixture has readings and is not posing the question")
	}
	if d.Steps[0].At == nil || *d.Steps[0].At != clicked {
		t.Fatalf("a panel press with no proposal under it recorded %v as its "+
			"point", d.Steps[0].At)
	}
}

// AND A PRESS WHOSE OUTCOME WAS NEVER VERIFIED IS NOT A STEP AT ALL.
//
// The guard either side of the fix: carrying a point must not have turned an inert press into a
// step. A click that did nothing observable is not part of a procedure, and most clicks are that.
func TestAnInertPressWithNoVerifiedOutcomeIsStillNotAStep(t *testing.T) {
	clicked := observe.PointerAt{X: 0.5, Y: 0.5}
	ev := arkPanelPress(clicked, 274)
	// TOO FEW WORDS TO BE AN OUTCOME, which is the only thing changed.
	ev.Outcomes[0].Appeared = []string{"back"}

	d := workflow.Learn("nothing happened", []observe.VisualScreenID{"visual_3"}, ev, nil)
	if len(d.Steps) != 0 {
		t.Fatalf("%d step(s) learned from a press with no verified outcome: %+v",
			len(d.Steps), d.Steps)
	}
}
