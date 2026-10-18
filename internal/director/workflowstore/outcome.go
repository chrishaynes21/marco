package workflowstore

// DID THE ACTION DO WHAT IT DID WHEN IT WAS DEMONSTRATED?
//
// # Why `Reached` cannot answer this
//
// Every one of its three terms is WHOLE-SCREEN: the destination's arrangement against this
// look, the source's against it, and the destination's saved words against a fresh reading.
// [[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]] measured a case where that
// is unanswerable — Ark's settings menu has two panels under one persistent tab bar, so the
// panel changes completely, the screen does not, and a step has the same label at both ends.
// There is no second arrangement to compare and no second vocabulary to corroborate.
//
// The owner's instruction was to fix the verification rather than the segmentation:
//
//	Do not require a new global screen ID for every meaningful UI change. […] Do not count any
//	pixel difference as success. The verifier needs positive evidence of the expected
//	destination.
//
// # What a step remembers, and what it has to see again
//
// `Step.Outcome` is the words a demonstration read SOMEWHERE OTHER THAN the control it pressed,
// which appeared when the control was pressed and stayed. At replay the same question is asked
// of fresh readings, and the answer has to OVERLAP the saved expectation — which is what makes
// this positive evidence of the EXPECTED destination rather than evidence that something
// changed.
//
// The four terms and the measurement behind each are in `observe.OutcomeBetween` and
// `observe.Stationary`. This file is the durable half: it holds the expectation, it supplies
// the excluded region from the control the step already saved, and it refuses with a sentence.
//
// # And it is still not proof that the action caused it
//
// `Arrival.Verified` stays false and still says why. A verified outcome is much better evidence
// than an arrival — it is a change that was absent before, present after, persistent, and from
// outside the control that was pressed — and it is still not a causal claim. Nothing else
// changed on the screen is not something this can know.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// controlNamed is the saved control carrying one label.
func (a Artifact) controlNamed(label string) (Control, bool) {
	for _, c := range a.Controls {
		if c.Label == label {
			return c, true
		}
	}
	return Control{}, false
}

// OutcomeExcluded is the region a step's outcome may not be read from.
//
// The control's own readings AND the containers that were not it — every rectangle the
// demonstration recorded as having held the press. The action site is approved from inside
// these, so a word read there is that evidence again rather than corroboration of it.
//
// Taken from what the artifact already saved. No new geometry: `Readings` and `Containers`
// have been on a control since it existed, kept for the reacquisition question, and they are
// exactly the right answer to this one.
func (a Artifact) OutcomeExcluded(step int) []observe.Region {
	if step < 0 || step >= len(a.Steps) {
		return nil
	}
	c, ok := a.controlNamed(a.Steps[step].Control)
	if !ok {
		return nil
	}
	out := make([]observe.Region, 0, len(c.Readings)+len(c.Containers))
	out = append(out, c.Readings...)
	out = append(out, c.Containers...)
	return out
}

// OutcomeHeld asks whether a step's demonstrated outcome is standing now.
//
// `before` is the readings taken of this screen before Marco acted and `after` the readings
// since, one per look, oldest first — the caller's to supply, because this package cannot see.
// `quiet` and `moved` are `observe.Stationary` over windows with no action in them: a screen
// that produces an outcome by itself is refused rather than guessed at.
//
// Returns whether it held and a sentence either way. The sentence is the point: a refusal that
// says "the evidence does not agree" sends somebody nowhere, and every refusal here names which
// term failed and on what.
func (a Artifact) OutcomeHeld(step int, before, after [][]observe.ReadWord,
	quiet, moved int) (bool, string) {

	if step < 0 || step >= len(a.Steps) {
		return false, fmt.Sprintf("step %d is not in a workflow of %d", step,
			len(a.Steps))
	}
	want := a.Steps[step].Outcome
	if len(want) == 0 {
		return false, "this step was demonstrated without an outcome, so there is " +
			"nothing it could be checked against — which is the honest answer for " +
			"a step learned before outcomes were recorded, or one whose screen " +
			"could not be read at the time"
	}
	// ── THE SCREEN HAS TO HOLD STILL, or nothing it does means anything ──
	//
	// Asked FIRST, because it decides whether the rest of the evidence is worth
	// reading. Measured: Rocket League's main menu produced an outcome in 2 to 12 of
	// every 25 to 64 windows with no input in them, and Ark's settings panel in 0 of 54.
	if quiet > 0 && moved > 0 {
		return false, fmt.Sprintf("this screen changed by itself in %d of %d window(s) "+
			"with no action in them, so an outcome on it is not evidence that "+
			"anything was caused", moved, quiet)
	}
	got := observe.OutcomeBetween(a.OutcomeExcluded(step), before, after)
	if got.BeforeLooks == 0 || got.AfterLooks == 0 {
		return false, fmt.Sprintf("nothing was read on one side of the action — %d "+
			"reading(s) before and %d after — so the question was not asked",
			got.BeforeLooks, got.AfterLooks)
	}
	// ── AND THE FRESH EVIDENCE HAS TO BE THE EXPECTED ONE ──
	//
	// The overlap, not the count. A panel that changed to something ELSE produces plenty
	// of words that appeared and stayed, and accepting those would be verifying that
	// something happened rather than that the right thing happened.
	kept := overlap(want, got.Appeared)
	if len(kept) < observe.OutcomeWords {
		return false, fmt.Sprintf("%d of the %d word(s) this step expected appeared and "+
			"stayed, and it takes %d: %s",
			len(kept), len(want), observe.OutcomeWords, saying(kept, got))
	}
	return true, fmt.Sprintf("%d of the %d word(s) this step expected appeared where the "+
		"control is not and stayed for %d reading(s): %s",
		len(kept), len(want), got.AfterLooks, strings.Join(kept, " "))
}

// saying is what a refusal has to offer: what DID appear, when the expected words did not.
func saying(kept []string, got observe.LocalOutcome) string {
	switch {
	case len(kept) > 0:
		return "only " + strings.Join(kept, " ")
	case len(got.Appeared) > 0:
		return "what appeared instead was " + strings.Join(got.Appeared, " ")
	case len(got.Transient) > 0:
		return "everything that appeared was transient"
	default:
		return "nothing elsewhere on the window appeared and stayed"
	}
}

// overlap is the words present in both lists, sorted.
func overlap(want, got []string) []string {
	in := map[string]bool{}
	for _, w := range want {
		in[w] = true
	}
	var out []string
	for _, g := range got {
		if in[g] {
			out = append(out, g)
		}
	}
	sort.Strings(out)
	return out
}
