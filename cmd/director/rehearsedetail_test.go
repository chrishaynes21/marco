package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
)

// The host's reason survives all the way from the runner to the panel.
//
// # Why this test and not two field assertions
//
// Because a field that exists at both ends and is not carried between them is the failure this
// repository keeps finding — three recorded cases of complete code that nothing invoked, and one
// caught earlier in this same session. `Detail` is set in the live runner and read by the Learn
// panel, and between them sit two conversions that would compile perfectly while dropping it.
//
// Deleting `Detail: s.Detail` from either conversion must fail this.
func TestTheHostsReasonSurvivesToTheSurface(t *testing.T) {
	const said = `theater: target_not_found: no control called "Mouse"`

	// What the LIVE RUNNER produced.
	result := rehearse.RehearsalResult{
		Steps: []rehearse.StepRecord{{
			Position: 1, Outcome: rehearse.InputFailed,
			Expect: "subj_543793ccc326", Detail: said,
		}},
	}

	// → the service view, through the production conversion.
	view := rehearsalViewOf(result, true)
	if len(view.Steps) != 1 {
		t.Fatalf("%d step(s) in the view", len(view.Steps))
	}
	if view.Steps[0].Detail != said {
		t.Fatalf("the service view carries %q, want the host's own sentence.\nEverything "+
			"downstream can only report input_failed, which is a kind of problem and "+
			"not a problem.", view.Steps[0].Detail)
	}

	// → the teaching attempt, through the production conversion.
	steps := attemptSteps(view.Steps)
	if len(steps) != 1 {
		t.Fatalf("%d step(s) reached teaching", len(steps))
	}
	if steps[0].Detail != said {
		t.Fatalf("teaching's step carries %q; the panel reads this one", steps[0].Detail)
	}

	// → and into what a person reads.
	a := teach.Attempt{Attempted: true, Steps: steps}
	reading := strings.Join(attemptDetail(&a), "\n")
	if !strings.Contains(reading, "target_not_found") {
		t.Errorf("the reading does not say what the host refused:\n%s", reading)
	}
}
