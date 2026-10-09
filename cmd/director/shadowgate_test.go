package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/shadow"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Can the experiment be run on a surface whose primary reading claims to be sufficient?
//
// # Why this switch exists
//
// The purchase policy declines an inference on a positive statement that the current reading
// suffices. That is correct and measured for a desktop. On a game it is the wrong answer to
// the right question: accessibility exposes a window and a pane, SufficiencyOf reports content
// reached, and the only sensor that can see the menu is declined for the rest of the epoch.
//
// Measured live on Trackmania through the production observation loop: ONE inference in 78
// looks, and the report called the other 77 cadence skips.
//
// The policy is not changed here. This is the switch that lets the experiment be RUN on such a
// surface so the decision can be made on evidence rather than on argument.

func TestTheExperimentIsGatedUnlessTheSwitchIsSet(t *testing.T) {
	t.Setenv(shadowUngatedEnv, "")
	if (&Runtime{}).shadowGate() == nil {
		t.Fatal("the purchase policy is off by default; a desktop session would spend a " +
			"second an inference on readings that already answer the question")
	}
	if note := shadowGateNote(true); note == "" || strings.Contains(note, "UNGATED") {
		t.Errorf("a gated run describes itself as %q", note)
	}
}

func TestTheSwitchRemovesTheGateAndSaysSo(t *testing.T) {
	t.Setenv(shadowUngatedEnv, "1")
	if (&Runtime{}).shadowGate() != nil {
		t.Fatal("the switch was set and the experiment is still behind the purchase policy")
	}
	note := shadowGateNote(false)
	if !strings.Contains(note, "UNGATED") {
		t.Errorf("an ungated run must say so wherever its numbers go, got %q", note)
	}
	if !strings.Contains(note, shadowUngatedEnv) {
		t.Errorf("the note does not name the switch that would cause it: %q", note)
	}
	// Off words mean off. A switch whose "false" turns it on is worse than no switch.
	for _, off := range []string{"0", "no", "false", "off", ""} {
		t.Setenv(shadowUngatedEnv, off)
		if (&Runtime{}).shadowGate() == nil {
			t.Errorf("$%s=%q removed the gate", shadowUngatedEnv, off)
		}
	}
}

func TestTheReportSaysWhichPurchasePolicyItRanBehind(t *testing.T) {
	// A run whose policy nobody recorded is a measurement of an unknown system: the same
	// detector on the same screen differs by two orders of magnitude between the two.
	var b strings.Builder
	totals := navigatedMenu(t)
	totals.Gate = "declines a slot when the primary reading claimed to be sufficient"
	renderShadow(&b, totals)
	if out := b.String(); !strings.Contains(out, "gate         declines a slot") {
		t.Errorf("the report does not name the purchase policy:\n%s", out)
	}
}

func TestADeclinedSlotIsNotReportedAsCadencePacing(t *testing.T) {
	// The whole failure mode in one assertion. Every slot declined, nothing ran, and the
	// report used to say "all skipped by cadence" — which sends a reader to tune a cadence
	// that was never involved.
	var totals observe.ShadowTotals
	for seq := 1; seq <= 20; seq++ {
		totals.Add(observe.ShadowSample{
			Detector: "screenparser", Declined: true,
			Gate: "declines a slot when the primary reading claimed to be sufficient",
		})
	}
	var b strings.Builder
	renderShadow(&b, totals)
	out := b.String()
	if strings.Contains(out, "skipped by cadence") {
		t.Errorf("declined slots are reported as cadence pacing:\n%s", out)
	}
	if !strings.Contains(out, "declined because the primary reading claimed to be sufficient") {
		t.Errorf("the report does not say why nothing ran:\n%s", out)
	}
	if totals.Declined != 20 {
		t.Errorf("declined slots counted %d of 20", totals.Declined)
	}
}

func TestACadenceSkipIsNotReportedAsADecline(t *testing.T) {
	// And the other way round, so the fix cannot be "say declined for everything".
	var totals observe.ShadowTotals
	for seq := 1; seq <= 20; seq++ {
		totals.Add(observe.ShadowSample{Detector: "screenparser"})
	}
	if totals.Declined != 0 {
		t.Fatalf("%d paced slots were counted as declined", totals.Declined)
	}
	var b strings.Builder
	renderShadow(&b, totals)
	if out := b.String(); strings.Contains(out, "declined because") {
		t.Errorf("paced slots are reported as declined:\n%s", out)
	}
}

// And the note reaches the sample, through the production sampler.
//
// The call site it needs is one line in liveSampler.Sample, beside the label budget. This
// repository has eight recorded cases of a complete mechanism nothing invoked, and a
// configuration note that never travels is the same defect in its mildest form: every number
// in the report stays correct and nothing says which system produced it.
func TestTheGateNoteTravelsWithTheSample(t *testing.T) {
	rt := visionShadowRuntime(t, nil, nil, fixtureCapture{generation: 1})

	// Nothing has attached a policy yet, which is also what a composition root that forgot
	// the line looks like. The report must say so rather than describe the default.
	if got := sampleWithVision(t, rt).Shadow.Gate; !strings.Contains(got, "UNGATED") {
		t.Errorf("a provider with no policy attached reported its policy as %q", got)
	}
	rt.attachShadowGate()
	got := sampleWithVision(t, rt).Shadow.Gate
	if got != shadowGateNote(true) {
		t.Errorf("a gated provider reported its policy as %q", got)
	}
}

// And the switch decides whether the purchase policy is attached to the provider at all.
//
// The sentence in the report and the behaviour of the detector are two different claims, and
// only the second is the reason the switch exists. Probed with a SENTINEL in the slot the
// policy would take: if attachShadowGate puts the policy there, the sentinel is never asked
// again. Asserting on whether an inference ran would prove nothing here — a bare Runtime has
// nothing watching, so the real policy answers "ignorance buys a look" and the detector runs
// either way.
func TestTheSwitchDecidesWhetherThePolicyIsAttached(t *testing.T) {
	probe := func(ungated string) bool {
		t.Setenv(shadowUngatedEnv, ungated)
		rt := &Runtime{engine: stubEngine{}}
		rt.shadowVision = newShadowProvider(&countingDetector{},
			fixtureCapture{generation: 1}, rt.activeWindow, nil, time.Nanosecond)
		rt.pinnedWindow = &windowRef1
		asked := false
		rt.shadowVision.WhenWorthIt(func() bool { asked = true; return true })
		rt.attachShadowGate()
		rt.shadowVision.ObserveTargeted(context.Background(), observation.Request{
			Sources: []directorapi.ObservationSource{directorapi.SourceVision},
		})
		return asked
	}
	if probe("") {
		t.Error("by default the purchase policy was never attached: the experiment would " +
			"run on every cadence slot of every desktop session")
	}
	if !probe("1") {
		t.Error("the switch was set and the purchase policy was attached anyway; " +
			"$MARCO_SHADOW_UNGATED does nothing")
	}
}

// countingDetector counts the inferences that actually started.
type countingDetector struct{ calls int }

func (d *countingDetector) Detect(context.Context, vision.Input) ([]vision.Detection, error) {
	d.calls++
	return nil, nil
}
func (d *countingDetector) Model() string { return "counting" }

// And the reason survives the trip from the provider onto the sample.
//
// The provider knows WHY it sat out; the sample is where a session can read it. The join is
// one string comparison over a free-text reason, which is exactly the kind that rots silently
// when somebody rewords the sentence at either end.
func TestTheSampleRecordsWhyTheSlotWasNotTaken(t *testing.T) {
	declined := observation.ProviderOutcome{
		Source: directorapi.SourceVision, State: observation.StateEmpty,
		Reason: "shadow slot skipped: " + shadow.DeclinedReason,
	}
	paced := observation.ProviderOutcome{
		Source: directorapi.SourceVision, State: observation.StateEmpty,
		Reason: "shadow slot skipped: cadence 1s",
	}
	frame := directorapi.Rect{Width: 1920, Height: 1080}

	got := shadowSampleFor("screenparser", "",
		observation.Cycle{Shadow: []observation.ProviderOutcome{declined}}, frame)
	if got == nil || !got.Declined {
		t.Errorf("a declined slot reached the sample as %+v", got)
	}
	got = shadowSampleFor("screenparser", "",
		observation.Cycle{Shadow: []observation.ProviderOutcome{paced}}, frame)
	if got == nil || got.Declined {
		t.Errorf("a paced slot reached the sample as declined: %+v", got)
	}
}
