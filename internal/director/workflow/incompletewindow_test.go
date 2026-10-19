package workflow_test

// AN INTERRUPTED OUTCOME WINDOW MUST NOT BECOME A SAVED EXPECTATION.
//
// Found by review of the live-demonstration fix, which carried the screen-scoping correction
// across from the run side and left this behind. `LocalOutcome.Verified` asked for ONE reading
// on each side, so a window of one reading was persistent by vacuity — every word it carried
// appeared on all one of its readings — and `outcomeOf` and `panelSteps` both turn a verified
// outcome into the saved step's expectation.
//
// These drive the whole path the defect reaches: the correlator, `workflow.Learn`, and the
// artifact `workflowstore.SaveWith` writes from the draft. Asserting only on the observation
// helper would leave the consumer untested, and the consumer is where the harm is.

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// theTab is the control the press lands in, and theBar the proposal it is distinguished from.
func theTab() observe.Region {
	return observe.Region{X: 0.40, Y: 0.24, Width: 0.08, Height: 0.03}
}

func theBar() observe.Region {
	return observe.Region{X: 0.03, Y: 0.23, Width: 0.94, Height: 0.05}
}

// panelSaying is one reading of the panel below the bar, where an outcome may be read.
func panelSaying(words ...string) []observe.ReadWord {
	out := make([]observe.ReadWord, 0, len(words))
	for i, w := range words {
		out = append(out, observe.ReadWord{Text: w, Where: observe.Region{
			X: 0.10, Y: 0.40 + float64(i)*0.03, Width: 0.20, Height: 0.02}})
	}
	return out
}

// savedExpectation is what a demonstration would persist for its one clicked step.
//
// Through `Learn` and `SaveWith`, because that is the path: a verified outcome becomes
// `Step.Outcome`, and `Step.Outcome` is what a later run is verified against.
func savedExpectation(t *testing.T, c *observe.ClickTargets,
	walk []observe.VisualScreenID) []string {

	t.Helper()
	d := workflow.Learn("a demonstration", walk, c.Evidence(), c.Registry())
	a, err := workflowstore.SaveWith(d, nil, nil, time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("saving the draft: %v", err)
	}
	for _, st := range a.Steps {
		if st.Control != "" {
			return st.Outcome
		}
	}
	return nil
}

// TestAWatchCutShortByAThirdScreenSavesNoExpectation is the reported sequence, exactly.
//
//	five before-readings on A, a press, three readings of B of which two are skipped and one
//	is kept carrying five new independent words, then C — which closes the watch.
//
// The one B reading qualified as verified despite never establishing persistence.
func TestAWatchCutShortByAThirdScreenSavesNoExpectation(t *testing.T) {
	c := &observe.ClickTargets{}
	for i := 0; i < 5; i++ {
		c.Reading("screen_a", panelSaying("resolution", "window", "mode"))
	}
	c.Press(10, observe.PointerAt{X: 0.44, Y: 0.25}, "screen_a",
		observe.Proposed([]observe.Region{theBar(), theTab()}, 1, 0, 0))
	// THREE READINGS OF B: two skipped as mid-response, ONE kept — and it carries five
	// independent words, which is the whole threshold.
	for i := 0; i < 3; i++ {
		c.Reading("screen_b", panelSaying("presets", "textures", "shadows",
			"bloom", "foliage"))
	}
	// AND THEN SOMEWHERE ELSE, which closes the watch on one reading.
	for i := 0; i < 6; i++ {
		c.Reading("screen_c", panelSaying("inventory", "crafting", "engrams"))
	}
	c.Arrived(12, observe.VisualTransition{From: "screen_a", To: "screen_b"})

	outs := c.Evidence().Outcomes
	if len(outs) != 1 {
		t.Fatalf("%d outcome(s) for one press", len(outs))
	}
	if outs[0].Verified() {
		t.Errorf("a window cut short at %d reading(s) verified: %v — %s",
			outs[0].AfterLooks, outs[0].Appeared, outs[0].Why())
	}
	if got := savedExpectation(t, c, []observe.VisualScreenID{"screen_a", "screen_b"}); len(got) > 0 {
		t.Fatalf("an interrupted window became the saved expectation: %v", got)
	}
}

// TestASessionEndingJustAfterARestartSavesNoExpectation.
//
// The other way a window is cut short, and it needs no third screen: a transition lands inside
// the settled window, the window starts again from it, and the session ends one reading later.
// One reading, five words, and persistence never asked.
func TestASessionEndingJustAfterARestartSavesNoExpectation(t *testing.T) {
	c := &observe.ClickTargets{}
	was := panelSaying("resolution", "window", "mode")
	for i := 0; i < 5; i++ {
		c.Reading("screen_a", was)
	}
	c.Press(10, observe.PointerAt{X: 0.44, Y: 0.25}, "screen_a",
		observe.Proposed([]observe.Region{theBar(), theTab()}, 1, 0, 0))
	// THE OLD PANEL PAST THE SKIP, so the window begins to fill with it...
	for i := 0; i < 5; i++ {
		c.Reading("screen_a", was)
	}
	// ...THEN THE TRANSITION, which restarts the window — and nothing after it.
	c.Reading("screen_a", panelSaying("presets", "textures", "shadows", "bloom", "foliage"))
	c.Arrived(12, observe.VisualTransition{From: "screen_a", To: "screen_b"})

	outs := c.Evidence().Outcomes
	if len(outs) != 1 {
		t.Fatalf("%d outcome(s) for one press", len(outs))
	}
	if outs[0].Verified() {
		t.Errorf("a window holding one reading after a restart verified: %v — %s",
			outs[0].Appeared, outs[0].Why())
	}
	if got := savedExpectation(t, c, []observe.VisualScreenID{"screen_a", "screen_b"}); len(got) > 0 {
		t.Fatalf("a restarted-then-ended window became the saved expectation: %v", got)
	}
}

// TestAFullWindowAcrossAGenuineScreenChangeStillSaves, which is the capability this must not cost.
//
// A press that changes the screen records its outcome once enough fresh readings of where it
// led have accumulated. That is the live case the screen-scoping fix exists for, and a rule
// that rejected interrupted windows by rejecting changed screens would undo it.
func TestAFullWindowAcrossAGenuineScreenChangeStillSaves(t *testing.T) {
	c := &observe.ClickTargets{}
	for i := 0; i < 5; i++ {
		c.Reading("screen_a", panelSaying("resolution", "window", "mode"))
	}
	c.Press(10, observe.PointerAt{X: 0.44, Y: 0.25}, "screen_a",
		observe.Proposed([]observe.Region{theBar(), theTab()}, 1, 0, 0))
	for i := 0; i < 8; i++ {
		c.Reading("screen_b", panelSaying("presets", "textures", "shadows",
			"bloom", "foliage", "distance"))
	}
	c.Arrived(12, observe.VisualTransition{From: "screen_a", To: "screen_b"})

	outs := c.Evidence().Outcomes
	if len(outs) != 1 || !outs[0].Verified() {
		t.Fatalf("a full window across a screen change did not verify: %+v", outs)
	}
	got := savedExpectation(t, c, []observe.VisualScreenID{"screen_a", "screen_b"})
	if len(got) < observe.OutcomeWords {
		t.Fatalf("the expectation saved is %v, want the panel the press produced", got)
	}
}
