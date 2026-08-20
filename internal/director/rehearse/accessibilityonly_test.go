package rehearse_test

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
)

// A rehearsal must be able to SEE an application that has no vision detector behind it.
//
// # Why this had never been tested, and why that mattered
//
// Every other test in this package builds its samples with a `screenparser` shadow detector — the
// vision experiment, switched on. Production runs with vision OFF. So the fixtures supplied a
// source that production does not have, and the one line that mattered was never exercised:
//
//	if sample.Shadow != nil { totals.Add(*sample.Shadow) }   // the VISION structure, only
//
// With vision off `sample.Shadow` is nil, nothing was folded, `CurrentState` stayed empty, and
// `SignatureOfState` refused. Every rehearsal against an accessibility-only application ended in
// `source_unobservable` and could not have done anything else. It was not intermittent and it was
// not the world being uncooperative: the rehearsal was reading a source that was never populated.
//
// Live, this cost four separate attempts across two days, each one diagnosed as something else,
// because "Marco could not make out the screen" is exactly what a stale window or a backgrounded
// application looks like.
//
// The session runner folds the same sample with `Observe`, which routes through `StructureOf` and
// prefers the fused world. There was never a reason for the two to differ.

// accessibleSample is what production actually produces: a fused composition, no shadow at all.
func accessibleSample(screen string) observe.Sample {
	s := liveSample(screen)
	if s.Shadow == nil {
		return s
	}
	// PRODUCTION'S shape, which is not "no shadow sample" but "a shadow sample that never
	// ran": the detector is configured and skips on cadence, so the record exists, carries
	// its name and its semantic reading, and has no regions of its own. All the structure
	// comes from the fused world.
	s.Structure = observe.StructuralView{
		Source: observe.StructureFused, Regions: s.Shadow.Regions,
	}
	s.Shadow = &observe.ShadowSample{
		Detector: s.Shadow.Detector, Ran: false, TargetProven: true,
		Semantic: s.Shadow.Semantic,
	}
	return s
}

// TestARehearsalCanSeeAnApplicationWithoutVision is the regression.
//
// Reverting `totals.Observe(sample)` to `totals.Add(*sample.Shadow)` must fail it.
func TestARehearsalCanSeeAnApplicationWithoutVision(t *testing.T) {
	w := newWorld("a", "b")
	w.sample = func(screen string, _ int) (observe.Sample, error) {
		return accessibleSample(screen), nil
	}

	j := livePlan()
	g := liveGrant(t, j)
	live := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).WithActuator(w, w, true)

	res, err := live.Rehearse(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1)
	if err != nil {
		t.Fatalf("a rehearsal against an application Marco can read perfectly well through "+
			"accessibility was refused: %v.\nThe screen was there, the structure was there, "+
			"and the rehearsal was looking at the vision detector — which is off.", err)
	}
	got := step1(t, res)
	if got.Outcome != rehearse.DirectlyVerified {
		t.Errorf("outcome = %q (observed %q), want the step to have been verified",
			got.Outcome, got.Observed)
	}
	if w.sent() != 1 {
		t.Errorf("%d program(s) reached the host, want 1", w.sent())
	}
}

// And a machine with NEITHER source still refuses, honestly.
//
// The control. The fix must be "read the authoritative structure too", not "assume a screen when
// nothing reported one" — acting blind is the thing this whole path exists to prevent.
func TestARehearsalWithNothingToSeeStillRefuses(t *testing.T) {
	w := newWorld("a", "b")
	w.sample = func(string, int) (observe.Sample, error) {
		return observe.Sample{}, nil // no fused structure, no shadow
	}

	j := livePlan()
	g := liveGrant(t, j)
	live := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).WithActuator(w, w, true)

	_, err := live.Rehearse(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1)
	if err == nil {
		t.Fatal("a rehearsal proceeded with nothing observed at all")
	}
	if reason, _ := rehearse.RefusalOf(err); reason != rehearse.RefusalSourceUnobservable {
		t.Errorf("refusal = %q, want %q", reason, rehearse.RefusalSourceUnobservable)
	}
	if w.sent() != 0 {
		t.Errorf("%d program(s) were emitted despite seeing nothing", w.sent())
	}
}

// "I could not see" and "I do not recognise this" are different facts.
//
// They shared one outcome, and the conflation cost real time: a live rehearsal reported
// `unobservable` while the window was in front and perfectly legible. The diagnosis went looking
// for a perception failure for an hour, when the truth was that the keys had landed on a screen
// Marco had never been shown.
//
// They also want opposite responses. A failure to look is Marco's problem; an unfamiliar screen is
// evidence about the route — it means the step went somewhere new.
func TestSeeingAnUnfamiliarScreenIsNotAFailureToSee(t *testing.T) {
	w := newWorld("a", "c") // ends on "c", which the memory does not remember
	w.sample = func(screen string, _ int) (observe.Sample, error) {
		return accessibleSample(screen), nil
	}

	j := livePlan()
	g := liveGrant(t, j)
	live := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).WithActuator(w, w, true)

	res, err := live.Rehearse(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	got := step1(t, res)
	if got.Outcome == rehearse.Unobservable {
		t.Fatal("landing on a screen Marco does not remember was reported as a failure to " +
			"observe.\nMarco read the screen perfectly well; it simply does not know the " +
			"place. Those send a reader looking in opposite directions.")
	}
	if got.Outcome != rehearse.Unrecognised && got.Outcome != rehearse.WrongState {
		t.Errorf("outcome = %q, want the step to say it ended somewhere unfamiliar", got.Outcome)
	}
}
