package observesession_test

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// A shadow experiment that runs but cannot report what it saw is still unwired.
//
// This is the regression for a defect that reached a live game session: the shadow
// accumulator and its renderer existed, were unit-tested, and had NO production call site.
// ScreenParser ran for five minutes and 149 CPU-seconds against Rocket League, produced
// evidence every cycle, and the session result discarded all of it.
//
// So this test deliberately does NOT call the accumulator. It enters through the runner —
// the production entry — supplies samples the way a real sampler does, and asserts the
// experiment's findings survive into the terminal Result. Deleting the one production call
// must fail it. A test that called ShadowTotals.Add directly would have passed throughout the
// entire period the feature was dead.

// shadowSampler produces samples carrying experimental findings, as a real sampler does.
type shadowSampler struct {
	clock *fakeClock
	cost  interface{ String() string }
}

func (s *shadowSampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	return observe.Sample{
		Shadow: &observe.ShadowSample{
			Detector:     "screenparser",
			Ran:          true,
			TargetProven: true,
			Detections:   7,
			Nameable:     4,
			Unknown:      1,
			Roles:        map[string]int{"button": 4, "text": 2},
			LatencyMS:    880,
			Comparison: observe.ShadowComparison{
				Agreed: 1, ShadowOnly: 6, ShadowOnlyNameable: 4,
			},
		},
	}, nil
}

// THE test. Enter through the runner; assert the terminal Result carries the experiment.
func TestTheSessionResultCarriesShadowFindings(t *testing.T) {
	clock := newClock()
	got, err := observesession.New(clock, &steadyTarget{ref: ref(1)},
		&shadowSampler{clock: clock}, &recordingEvents{}).
		Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Stats.SamplesTaken == 0 {
		t.Fatal("no samples were taken; this test cannot say anything about shadow wiring")
	}

	sh := got.Stats.Shadow
	if !sh.Observed() {
		t.Fatal("the session ran samples carrying shadow findings and the terminal Result " +
			"reports none. The experiment is unwired: it runs, produces evidence, and the " +
			"session discards it — exactly the failure this test exists for")
	}
	if sh.Detector != "screenparser" {
		t.Errorf("detector = %q, want screenparser", sh.Detector)
	}
	if sh.Inferences != got.Stats.SamplesTaken {
		t.Errorf("inferences %d against %d samples taken; every sample carried one",
			sh.Inferences, got.Stats.SamplesTaken)
	}
	// Aggregation, not just presence: a wiring that recorded the last sample only would
	// pass a presence check and misreport a whole session.
	if sh.Detections != 7*sh.Inferences {
		t.Errorf("detections %d over %d inferences, want %d — findings are not accumulating",
			sh.Detections, sh.Inferences, 7*sh.Inferences)
	}
	if sh.Comparison.ShadowOnly != 6*sh.Inferences {
		t.Errorf("shadow-only %d, want %d", sh.Comparison.ShadowOnly, 6*sh.Inferences)
	}
	if sh.Comparison.ShadowOnlyNameable != 4*sh.Inferences {
		t.Errorf("shadow-only-nameable %d, want %d — this is the product metric and it "+
			"must not be the number that gets dropped",
			sh.Comparison.ShadowOnlyNameable, 4*sh.Inferences)
	}
	if sh.Roles["button"] != 4*sh.Inferences {
		t.Errorf("role distribution not accumulated: %v", sh.Roles)
	}
	if sh.MedianMS != 880 {
		t.Errorf("median latency %dms, want 880", sh.MedianMS)
	}
}

// A session with no experiment must report none — not an experiment that saw nothing.
func TestASessionWithNoShadowReportsNoneRatherThanZero(t *testing.T) {
	clock := newClock()
	got, err := observesession.New(clock, &steadyTarget{ref: ref(1)},
		&countingSampler{clock: clock}, &recordingEvents{}).
		Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Stats.Shadow.Observed() {
		t.Fatal("a session with no experimental provider reported shadow observations")
	}
	if got.Stats.Shadow.Opportunities != 0 {
		t.Errorf("opportunities = %d; a Director with no shadow provider must not look "+
			"like one whose experiment never got a slot", got.Stats.Shadow.Opportunities)
	}
}

// A skipped cadence slot is not a failure and not an inference.
func TestSkippedShadowSlotsAreCountedApartFromInferences(t *testing.T) {
	var totals observe.ShadowTotals
	totals.Add(observe.ShadowSample{Detector: "screenparser", Ran: false})
	totals.Add(observe.ShadowSample{Detector: "screenparser", Ran: true, TargetProven: true})

	if totals.Inferences != 1 || totals.Skipped != 1 || totals.Opportunities != 2 {
		t.Fatalf("inferences %d, skipped %d, opportunities %d; want 1, 1, 2",
			totals.Inferences, totals.Skipped, totals.Opportunities)
	}
	if totals.Failures != 0 {
		t.Error("a skipped slot was counted as a failure; the cadence would read as an " +
			"error rate")
	}
}

// Evidence from a superseded generation is a provenance failure, never a UI disagreement.
func TestProvenanceFailuresAreNotDisagreements(t *testing.T) {
	var totals observe.ShadowTotals
	totals.Add(observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: false,
		Detections: 5,
		Comparison: observe.ShadowComparison{Uncomparable: 5},
	})
	if totals.ProvenanceFailures != 1 {
		t.Errorf("ProvenanceFailures = %d, want 1", totals.ProvenanceFailures)
	}
	if totals.Comparison.ShadowOnly != 0 || totals.Comparison.AuthoritativeOnly != 0 {
		t.Error("evidence about a different window generation was counted as a " +
			"disagreement about the interface")
	}
	if totals.Comparison.Uncomparable != 5 {
		t.Errorf("Uncomparable = %d, want 5", totals.Comparison.Uncomparable)
	}
}
