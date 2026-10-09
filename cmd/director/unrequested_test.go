package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A PROVIDER NOBODY ASKED DID NOT RUN.
//
// # What it looked like instead
//
// Vision is opt-in on its own source and returns StateNotRequested immediately: no
// observations, no provenance, no timing. Every one of those absences was read further down as
// a finding about the SCREEN. The outcome arrived at the sample as Ran=true with
// TargetProven=false — an inference that happened and could not say which window it described
// — so one live Trackmania run reported:
//
//	inferences   122 of 297 slots
//	provenance   116 inference(s) could not prove they observed the session's window
//
// on a session that performed six inferences and failed provenance on none of them. The
// interface belief, fed the same samples, reported "no shaded regions were proposed this look"
// — an empty screen — for three minutes in front of a menu.
//
// The cause is fixed in shadow.Provider, which now asks its own detector for its own sources.
// This is the line that stops the appearance being manufacturable at all, from any cause: a
// request that carries no pixel source is a fact about the request.

func TestADetectorNobodyAskedIsNotAnInference(t *testing.T) {
	frame := directorapi.Rect{Width: 1920, Height: 1080}
	unrequested := observation.ProviderOutcome{
		Source: directorapi.SourceVision,
		State:  observation.StateNotRequested,
	}

	got := shadowSampleFor("screenparser", "",
		observation.Cycle{Shadow: []observation.ProviderOutcome{unrequested}}, frame)
	if got == nil {
		t.Fatal("no sample at all; a slot that was offered must be reported")
	}
	if got.Ran {
		t.Errorf("a detector nobody asked was recorded as having RUN.\nIt returns in 0ms "+
			"having looked at nothing: counting it inflates inferences, latency and the "+
			"belief's own look count.\n%+v", got)
	}
	if got.TargetProven {
		t.Errorf("a detector nobody asked was recorded as having proven its target")
	}
	if !got.Unrequested {
		t.Errorf("the sample does not say the detector was never asked: %+v", got)
	}

	// And through the totals, which is where a reader meets the number.
	var totals observe.ShadowTotals
	totals.Add(*got)
	if totals.Inferences != 0 {
		t.Errorf("inferences = %d, want 0", totals.Inferences)
	}
	if totals.ProvenanceFailures != 0 {
		t.Errorf("provenance failures = %d, want 0.\nNothing failed to prove anything; "+
			"nothing was asked.", totals.ProvenanceFailures)
	}
	if totals.Unrequested != 1 {
		t.Errorf("unrequested = %d, want 1", totals.Unrequested)
	}
}

// And the report says so, in the panel where the misleading numbers lived.
//
// Not a cosmetic check. The two numbers directly below this line — latency and detections —
// are per-inference, and they were being divided by a count that was twenty times too high.
// A reader who cannot see how many slots were never asked cannot read either of them.
func TestTheReportSaysHowManySlotsWereNeverAsked(t *testing.T) {
	totals := navigatedMenu(t)
	for i := 0; i < 4; i++ {
		totals.Add(observe.ShadowSample{Detector: "screenparser", Unrequested: true})
	}

	var b strings.Builder
	renderShadow(&b, totals)
	out := b.String()

	if !strings.Contains(out, "NEVER ASKED") {
		t.Errorf("the report does not say any slot was never asked:\n%s", out)
	}
	if !strings.Contains(out, "no pixel source") {
		t.Errorf("the report does not say WHY the detector was not asked:\n%s", out)
	}
	// It must not be reported as the cadence pacing itself, which is the gate working.
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if strings.Contains(l, "NEVER ASKED") && strings.Contains(l, "cadence") {
			t.Errorf("a slot nobody asked is reported as cadence pacing: %q", l)
		}
	}
}

// A session where this happened for the WHOLE run must say so rather than reporting a silent
// cadence. That was the live shape: every slot offered, none taken, and a report that read as
// if the experiment were merely sampling slowly.
func TestASessionNobodyAskedSaysSoRatherThanSayingCadence(t *testing.T) {
	var totals observe.ShadowTotals
	for i := 0; i < 20; i++ {
		totals.Add(observe.ShadowSample{Detector: "screenparser", Unrequested: true})
	}

	var b strings.Builder
	renderShadow(&b, totals)
	out := b.String()

	if !strings.Contains(out, "never ran") {
		t.Fatalf("a session with no inferences did not say so:\n%s", out)
	}
	if !strings.Contains(out, "never asked") {
		t.Errorf("a session whose detector was never asked does not say that; a reader is "+
			"left to assume the cadence:\n%s", out)
	}
}

// And the per-slot trace, which is the instrument that found this in the first place.
//
// $MARCO_SHADOW_TRACE exists to answer "why was there no evidence", and its outcome set is
// closed on purpose. A skipped slot it cannot attribute answers its own question wrongly: on
// the live run every unasked slot was indistinguishable from the cadence, and the only reason
// the cause was findable at all was that the latency column read 0ms.
func TestTheTraceSaysASkippedSlotWasNeverAsked(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "slots.jsonl")
	tr := &shadowTrace{path: path}

	tr.record(&observe.ShadowSample{Detector: "screenparser"}, 1)
	tr.record(&observe.ShadowSample{Detector: "screenparser", Unrequested: true}, 1)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no trace written: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("%d slots traced, want 2", len(lines))
	}

	var paced, unasked struct {
		Outcome string `json:"outcome"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &paced); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &unasked); err != nil {
		t.Fatal(err)
	}

	if paced.Outcome != "skipped" || unasked.Outcome != "skipped" {
		t.Errorf("outcomes %q and %q; both are slots that produced no inference",
			paced.Outcome, unasked.Outcome)
	}
	if paced.Reason != "" {
		t.Errorf("a cadence skip carries a reason it does not need: %q", paced.Reason)
	}
	if !strings.Contains(unasked.Reason, "never asked") {
		t.Errorf("a slot nobody asked traced as %q — indistinguishable from the cadence, "+
			"which is the confusion that cost a whole live run", unasked.Reason)
	}
}
