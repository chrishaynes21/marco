package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// THE LOOP WAS NOT OPEN. IT WAS STUCK OPEN.
//
// # What 37F found
//
// 37E left one question outstanding: EscalationOf could decline an expensive sensor but nothing
// ever acquired one because it said yes. The expectation was a missing acquisition path.
//
// There was no missing path. `observation.Request.Include` already opts extra sensors into an
// ordinary cycle, `WithVision` and `WithPixels` already exist for exactly this, `rt.vision` and
// `rt.ocr` are already AUTHORITATIVE providers in the production collector — no ShadowOnly
// marker, so their evidence reaches fusion and therefore reaches SufficiencyOf — and the
// session sampler already asked for vision.
//
// It asked on EVERY sample. Measured on a healthy Settings page with the detector configured:
//
//	accessibility alone        66ms    155 elements   sufficient
//	accessibility + vision    940ms    176 elements   sufficient
//
// Fourteen times the cost for the same verdict. 37E's gate reached the shadow provider and not
// the authoritative one, so the loop that looked open was running continuously in the one place
// nobody was gating.
//
// These tests hold the request to the decision.

// stubRegistry answers placeHere with whatever this test needs.
type stubPlace struct{ place observe.Place }

func (s stubPlace) Recall(string, observe.StructureSignature) observe.Recollection {
	return observe.Recollection{}
}

// A sufficient reading does not ask for the visual pass.
func TestASufficientSampleDoesNotAskForPixels(t *testing.T) {
	rt := &Runtime{}
	s := &liveSampler{rt: rt, labels: true}
	req := observesession.SampleRequest{
		Window:     windowref.Ref{ID: directorapi.WindowID("hwnd:1")},
		ReadLabels: true,
	}

	// With nothing known, the pass is kept — ignorance is not a decline.
	if got := s.request(req); !got.Includes(directorapi.SourceVision) {
		t.Error("a Director that knows nothing about the current reading declined the " +
			"visual pass.\nThat is Marco not knowing, not Marco knowing it has enough, " +
			"and a sampler that stops looking when it is unsure sees less the less it " +
			"knows.")
	}

	// And it is the shared decision doing the work, not a local rule: the two cases where
	// "not sufficient" and "worth buying" disagree must both decline.
	incomplete := observe.Sufficiency{State: observe.Incomplete,
		Reason: observe.ReasonClientAreaUnpopulated}
	if observe.EscalationOf(observe.NeedWatching, incomplete, canSayWhichState, 0).Worth() {
		t.Error("background watching of an incomplete reading was told to spend")
	}
	sufficient := observe.Sufficiency{State: observe.Sufficient,
		Reason: observe.ReasonContentReached}
	if observe.EscalationOf(observe.NeedAnswer, sufficient, canSayWhichState, 0).Worth() {
		t.Error("a sufficient reading was told to spend; the sampler would then buy a " +
			"940ms pass for a verdict it already had")
	}
}

// The request is built from the shared decision rather than a rule of its own.
//
// A sampler that wrote `if !sufficient { vision }` would spend on the first incomplete frame of
// every navigation and forever on a game window. Both of those are the policy's business and
// neither is visible from here.
func TestTheSamplerAsksThePolicyForItsPixels(t *testing.T) {
	src := mustReadSource(t, "observewiring.go")
	if !containsAll(src, "s.rt.moreEvidenceIsWorthBuying()", "observation.WithVision(nil)") {
		t.Error("observewiring.go no longer decides the visual pass through the shared " +
			"escalation wiring.\nA sampler with its own rule about when to spend is a " +
			"second sensor policy, and it is the one that runs on every sample.")
	}
	// And the gate must sit ABOVE both requests, or WithPixels escapes it.
	pixels := indexOf(src, "observation.WithPixels(nil)")
	gate := indexOf(src, "s.rt.moreEvidenceIsWorthBuying()")
	if gate < 0 || pixels < 0 || gate > pixels {
		t.Error("the pixels request is not inside the escalation gate; the expensive " +
			"combined pass would still run unconditionally")
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
