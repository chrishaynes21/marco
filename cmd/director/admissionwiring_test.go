package main

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// WHERE THE DETECTOR'S OUTPUT WENT, THROUGH THE PRODUCTION PATH.
//
// Experiment-024 is the reason this exists, and it is a different defect from the one
// shapewiring_test.go holds. That one is about geometry being discarded. This one is about the
// session being unable to SAY so: its record carried only what survived admission, so "the
// detector found nothing" and "the detector found seventy boxes and refused every one" were
// the same report. The run's central conclusion had to come from a separate `director vision`
// reading taken afterwards — a different sample of a different screen at a different moment.
//
// The fixture is the measured case: nine rows of text with nothing readable inside. Zero
// accepted, nine refused, every refusal the same kind. A session that cannot distinguish that
// from a blank screen cannot be used to judge whether ADR-154 reached this path.

// The sample carries it, and the totals accumulate it, through the sampler production uses.
//
// Both layers in one test deliberately: the wiring can attach the counters to every sample
// while nothing folds them into the session, which is the shape of eight recorded cases in
// this repository of complete code that was never invoked.
//
// Deleting `sh.Admission = shadowAdmission(...)` in liveSampler.Sample must fail this, and so
// must deleting `t.Admission.Add(s.Admission)` in ShadowTotals.Observe.
func TestTheSessionSaysWhereTheDetectorsOutputWent(t *testing.T) {
	rt := visionShadowRuntime(t, unreadableMenuFixture(), nil,
		paintedFixtureCapture{generation: 1})
	sample := sampleWithVision(t, rt)

	if sample.Shadow == nil {
		t.Fatal("no shadow record, so the admission account has nowhere to ride")
	}
	a := sample.Shadow.Admission
	if a.Proposed != 9 {
		t.Fatalf("the sample says %d detections were proposed, from a fixture of 9.\n"+
			"Without the denominator, the %d that survived cannot be read as a share "+
			"of anything — which is exactly what made Experiment-024's finding rest on "+
			"evidence the run did not contain", a.Proposed, sample.Shadow.Detections)
	}
	if a.Accepted != 0 {
		t.Errorf("%d of the fixture's unreadable rows were accepted; the fixture is "+
			"supposed to produce nothing admissible and this test would prove nothing",
			a.Accepted)
	}
	if a.Unreadable != 9 {
		t.Errorf("%d refusals were 'nothing readable inside', want 9. That is the "+
			"dominant refusal on a game, and the one whose geometry ADR-154 keeps — a "+
			"report that cannot name it cannot say whether the repair is working",
			a.Unreadable)
	}
	if a.Refused() != 9 {
		t.Errorf("%d refused of 9 proposed; proposed must account for accepted plus "+
			"every refusal, or the shares do not add up", a.Refused())
	}

	// AND THE SESSION ACCUMULATES IT. Folded through Observe, which is the call site the
	// live cycle uses, rather than by adding the structs up here.
	var totals observe.ShadowTotals
	for i := range 3 {
		s := sample
		s.Sequence = i + 1
		s.Timestamp = time.Unix(1760000000, 0).Add(time.Duration(i) * time.Second)
		totals.Observe(s)
	}
	if totals.Admission.Proposed != 27 {
		t.Errorf("three looks at nine proposals total %d on the session, want 27: the "+
			"sample carries the account and nothing folds it in",
			totals.Admission.Proposed)
	}
	if totals.Admission.Unreadable != 27 {
		t.Errorf("the session totals %d unreadable refusals over three looks, want 27",
			totals.Admission.Unreadable)
	}
}

// refusedEverythingSession is a session whose detector proposed plenty and kept none.
func refusedEverythingSession() observe.ShadowTotals {
	var totals observe.ShadowTotals
	for i := range 4 {
		totals.Observe(observe.Sample{
			Sequence: i + 1, WindowGeneration: 1,
			Timestamp: time.Unix(1760000000, 0).Add(time.Duration(i) * time.Second),
			Shadow: &observe.ShadowSample{
				Detector: "screenparser", Ran: true, TargetProven: true, LatencyMS: 1042,
				Admission: observe.ShadowAdmission{
					Proposed: 19, Unreadable: 18, Confidence: 1,
				},
			},
		})
	}
	return totals
}

// The report says it, in the place a person reads.
//
// A reader who has "detections 0 total" and nothing else draws the wrong conclusion from a
// true number, and it is the wrong conclusion in the most expensive direction: that the screen
// was empty, when the detector was looking at a full one and refusing all of it.
//
// Deleting the admission block in renderShadow must fail this.
func TestTheReportSaysWhereTheDetectorsOutputWent(t *testing.T) {
	var b strings.Builder
	renderShadow(&b, refusedEverythingSession())
	out := b.String()

	if !strings.Contains(out, "admission ") {
		t.Fatalf("the report never says what became of the detector's output:\n%s", out)
	}
	for _, want := range []string{"76 proposed", "0 accepted", "76 refused"} {
		if !strings.Contains(out, want) {
			t.Errorf("the admission line is missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "nothing readable inside") {
		t.Error("the report counts refusals without saying what kind. One number for " +
			"every reason is how a vocabulary gap and an unreadable screen became the " +
			"same finding once already — see Counters.RejectedClass")
	}
	// And the honest zero above it is still printed, because the pair is the point.
	if !strings.Contains(out, "detections   0 total") {
		t.Errorf("a session that admitted nothing no longer reports it:\n%s", out)
	}

	// A session nobody asked a detector prints no refusal rate at all: zero proposals is
	// not a refusal of anything, and a rate invented from it would be a finding.
	var quiet strings.Builder
	renderShadow(&quiet, observe.ShadowTotals{Detector: "screenparser", Inferences: 1})
	if strings.Contains(quiet.String(), "admission ") {
		t.Errorf("a session whose detector proposed nothing reports an admission "+
			"account:\n%s", quiet.String())
	}
}
