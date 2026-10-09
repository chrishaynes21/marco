package observe_test

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// THREE WAYS TO SEE NOTHING, AND ONLY ONE OF THEM IS ABOUT THE SCREEN.
//
// The detector found nothing; the detector was never asked; the detector looked and cannot say
// which window it looked at. A belief that folds the last two in reports an EMPTY SCREEN on
// evidence that says nothing about the screen at all — which is
// [[ADR-006-unknown-is-not-false]] in its most expensive form, because the answer is confident
// and wrong rather than absent.
//
// Both of the added cases came from one live run and neither is reachable in replay: a replay
// hands every look a frame whose provenance was settled when it was recorded, and nothing in a
// replay ever declines to ask. Live, on Trackmania, 116 of 122 looks were a detector nobody
// asked, and the session reported "no shaded regions were proposed this look" — an empty
// screen — for three minutes in front of a menu.

// sampleUnrequested is a slot that was TAKEN and whose detector was never asked, because the
// request carried no pixel source.
func sampleUnrequested(seq int) observe.Sample {
	return observe.Sample{
		Sequence:         seq,
		Timestamp:        sampleEpoch.Add(time.Duration(seq) * 500 * time.Millisecond),
		WindowGeneration: 1,
		Shadow:           &observe.ShadowSample{Detector: "test", Unrequested: true},
	}
}

// sampleUnproven is a look the detector took whose pixels it cannot attribute to this window.
// It carries a full menu's worth of regions on purpose: the question is whether the belief
// refuses evidence it cannot place, not whether it refuses evidence that is empty.
func sampleUnproven(seq int) observe.Sample {
	s := menuSample(seq, 6, 2)
	s.Shadow.TargetProven = false
	return s
}

func TestADetectorNobodyAskedIsNotAnEmptyScreen(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	liveCadence(&totals, &seq, 1, []int{-1, -1, -1, -1, -1, -1,
		0, 0, 0, 1, 1, 1, 2, 2, 2})

	settled := totals.Interface
	if settled.Status != observe.InterfaceProvisional {
		t.Fatalf("precondition: no list was identified (%s)", settled.Because)
	}

	// Now the purchase policy stops asking for pixels. Nothing about the screen changed.
	for range 3 {
		seq++
		totals.Observe(sampleUnrequested(seq))
	}

	b := totals.Interface
	if b.Status != settled.Status || b.Selected != settled.Selected {
		t.Errorf("the belief moved from %s/%d to %s/%d because nobody asked the detector.\n"+
			"A slot where the detector was never asked is evidence about the REQUEST, not "+
			"about the screen.\nbecause: %s",
			settled.Status, settled.Selected, b.Status, b.Selected, b.Because)
	}
	if !b.Stale {
		t.Errorf("the belief is not marked stale after %d looks nobody took", 3)
	}
	if b.Unconfirmed != 3 {
		t.Errorf("unconfirmed = %d, want 3", b.Unconfirmed)
	}
	if !containsAll(b.Because, "never asked") {
		t.Errorf("the belief does not say WHY it is unconfirmed: %q.\nIt must not read as "+
			"the cadence pacing itself — that gate working and this gate starving the "+
			"experiment have opposite remedies.", b.Because)
	}
	if totals.Unrequested != 3 {
		t.Errorf("totals counted %d unrequested slots, want 3", totals.Unrequested)
	}
	if totals.Inferences != 15 {
		t.Errorf("inferences = %d, want 15; a detector nobody asked was counted as one "+
			"that ran", totals.Inferences)
	}
}

func TestAnUnprovenLookIsNotAnEmptyScreen(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	liveCadence(&totals, &seq, 1, []int{-1, -1, -1, -1, -1, -1,
		0, 0, 0, 1, 1, 1, 2, 2, 2})

	settled := totals.Interface
	if settled.Status != observe.InterfaceProvisional {
		t.Fatalf("precondition: no list was identified (%s)", settled.Because)
	}

	for range 3 {
		seq++
		totals.Observe(sampleUnproven(seq))
	}

	b := totals.Interface
	if !b.Stale || b.Unconfirmed != 3 {
		t.Errorf("after three looks whose window could not be established the belief is "+
			"stale=%v unconfirmed=%d, want true/3.\nPixels that cannot be attributed to "+
			"this window are not evidence about this window — in either direction.\n"+
			"because: %s", b.Stale, b.Unconfirmed, b.Because)
	}
	if !containsAll(b.Because, "which window") {
		t.Errorf("the belief does not say its looks could not be attributed: %q", b.Because)
	}
}

// And the far more dangerous direction: unproven pixels must not be ADOPTED either.
//
// The regions on an unproven look are real detections of a real window — just possibly not
// this one. Folding them in would move the selection to wherever the other window's mark is
// and report it as this interface's, with full confidence and a sample number to cite.
func TestUnprovenPixelsDoNotMoveTheSelection(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	liveCadence(&totals, &seq, 1, []int{-1, -1, -1, -1, -1, -1,
		0, 0, 0, 1, 1, 1, 2, 2, 2})
	settled := totals.Interface
	if settled.Status != observe.InterfaceProvisional {
		t.Fatalf("precondition: no list was identified (%s)", settled.Because)
	}

	// Three looks that would move the mark three rows down, if they were believed.
	for _, m := range []int{3, 3, 3} {
		seq++
		s := menuSample(seq, 6, m)
		s.Shadow.TargetProven = false
		totals.Observe(s)
	}

	if got := totals.Interface.Selected; got != settled.Selected {
		t.Errorf("the selection moved from %d to %d on evidence that cannot be shown to "+
			"describe this window", settled.Selected, got)
	}
}

// A detector nobody asked is not the cadence pacing itself, and the totals must not read as
// if it were. Both are skips; only one of them is the gate working.
func TestANeverAskedSlotIsNotACadenceSkip(t *testing.T) {
	var totals observe.ShadowTotals
	for i := 1; i <= 4; i++ {
		totals.Observe(sampleSkipped(i))
	}
	for i := 5; i <= 9; i++ {
		totals.Observe(sampleUnrequested(i))
	}

	if totals.Skipped != 9 {
		t.Errorf("skipped = %d, want 9: both are slots that produced no inference",
			totals.Skipped)
	}
	if totals.Unrequested != 5 {
		t.Errorf("unrequested = %d, want 5", totals.Unrequested)
	}
	if totals.Inferences != 0 {
		t.Errorf("inferences = %d, want 0", totals.Inferences)
	}
	if totals.ProvenanceFailures != 0 {
		t.Errorf("provenance failures = %d, want 0.\nA detector that was never asked did "+
			"not fail to prove anything — it was never given a window to prove.",
			totals.ProvenanceFailures)
	}
}

// The STRUCTURAL view says it too, because that is the surface a diagnostic reads.
//
// Same distinction, different consumer: "the detector paced itself" is the gate working and
// "nobody asked the detector" is the experiment switched off by a decision made elsewhere.
// A diagnostic that reports the first for the second sends a reader to the wrong layer.
func TestTheStructuralViewSaysWhichKindOfAbsence(t *testing.T) {
	paced := observe.ShadowSample{Detector: "screenparser"}
	unasked := observe.ShadowSample{Detector: "screenparser", Unrequested: true}

	if got := paced.Structure().Why; !containsAll(got, "sat this frame out") {
		t.Errorf("a cadence skip reads as %q", got)
	}
	got := unasked.Structure().Why
	if !containsAll(got, "never asked", "no pixel source") {
		t.Errorf("a detector nobody asked reads as %q.\nIt must not read as the cadence "+
			"pacing itself; the two have opposite remedies.", got)
	}
	if containsAll(got, "sat this frame out") {
		t.Errorf("a detector nobody asked is reported as having sat the frame out: %q", got)
	}
}
