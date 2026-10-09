package main

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Can somebody watching a live session READ the answer?
//
// The belief is on the wire already — `director observation-session --json` carries the whole
// of ShadowTotals. This is about the other surface: the text report a person reads while they
// navigate. A belief nothing renders is a belief nobody checks, and the one failure mode this
// repository keeps repeating is a complete subsystem that production never shows.
//
// Driven through ShadowTotals.Observe at the LIVE shape — one inference per four samples,
// because that is the ratio the shadow cadence produces against the default interval.

func interfaceMenuSample(seq int, rows, marked int) observe.Sample {
	var regions []observe.ShadowRegion
	for i := range rows {
		shade := []float64{60, 72}
		if i == marked {
			shade = []float64{205, 238}
		}
		regions = append(regions, observe.ShadowRegion{
			Role: "button",
			Region: observe.Region{
				X: 0.10, Y: 0.40 + float64(i)*0.06, Width: 0.40, Height: 0.05,
			},
			Shade: shade,
		})
	}
	return observe.Sample{
		Sequence:         seq,
		Timestamp:        time.Unix(1760000000, 0).Add(time.Duration(seq) * 500 * time.Millisecond),
		WindowGeneration: 1,
		Shadow: &observe.ShadowSample{
			// TargetProven, because an inference that cannot say which window its
			// pixels came from has not described this screen. A fixture without it
			// models a look production refuses.
			Detector: "screenparser", Ran: true, TargetProven: true, Regions: regions,
			Detections: len(regions), LatencyMS: 900,
		},
	}
}

func interfaceSkipped(seq int) observe.Sample {
	return observe.Sample{
		Sequence:         seq,
		Timestamp:        time.Unix(1760000000, 0).Add(time.Duration(seq) * 500 * time.Millisecond),
		WindowGeneration: 1,
		Shadow:           &observe.ShadowSample{Detector: "screenparser"},
	}
}

// navigatedMenu is a session that watched a mark move down a list at the live cadence.
func navigatedMenu(t *testing.T) observe.ShadowTotals {
	t.Helper()
	var totals observe.ShadowTotals
	seq := 0
	for _, m := range []int{-1, -1, -1, -1, -1, -1,
		0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3} {
		seq++
		totals.Observe(interfaceMenuSample(seq, 6, m))
		for range 3 {
			seq++
			totals.Observe(interfaceSkipped(seq))
		}
	}
	if totals.Interface.Status != observe.InterfaceProvisional {
		t.Fatalf("precondition: no list was identified (%s)", totals.Interface.Because)
	}
	return totals
}

func TestTheSessionReportSaysWhatItIsLookingAt(t *testing.T) {
	var b strings.Builder
	renderShadow(&b, navigatedMenu(t))
	out := b.String()

	for _, want := range []string{
		// what interface am I looking at
		"interface    provisional",
		"a list at",
		// and the thing a reader must not be allowed to miss
		"NOT known to be all of it",
		// what is selected
		"is selected",
		// what changed since the last observation
		"since the last look:",
		// and the evidence under it
		"because:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the session report cannot say %q:\n%s", want, out)
		}
	}
}

func TestTheSessionReportSaysWhenTheAnswerChanged(t *testing.T) {
	var b strings.Builder
	renderShadow(&b, navigatedMenu(t))
	out := b.String()
	if !strings.Contains(out, "moment(s) where the answer changed") {
		t.Fatalf("the report cannot say when its answer changed:\n%s", out)
	}
	// A journal of changes carries the look each one happened at, so a reader polling at
	// their own rate still sees the ones they were not watching for.
	if !strings.Contains(out, "sample ") {
		t.Errorf("the moments carry no sample numbers:\n%s", out)
	}
}

func TestTheReportStatesTheBoundaryItActuallyHolds(t *testing.T) {
	// This evidence may inform a hypothesis and may inform a proposed action — it already
	// does: observe.Hypotheses is derived from these totals. What it may never do is
	// authorize an effect or stand as proof that one happened.
	//
	// Printed because the previous wording ("cannot reach belief, planning, policy or
	// input") was wider than the rule the code holds, and a reader who believed it would
	// conclude that Marco's perception is unusable by the agent that owns it.
	var b strings.Builder
	renderShadow(&b, navigatedMenu(t))
	out := b.String()
	for _, want := range []string{
		"NONE over belief",
		"MAY inform hypotheses and proposed actions",
		"never authorize",
		"prove one happened",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not state %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "cannot reach belief, planning") {
		t.Error("the report still claims this evidence cannot reach planning, which is " +
			"false in this build: a plan reads the relationships these hypotheses support")
	}
}

func TestAStaleBeliefSaysSoInTheReport(t *testing.T) {
	// The live case this exists for: the cadence gate holds the detector back, nothing is
	// observed, and the report must not keep presenting the belief as freshly seen.
	totals := navigatedMenu(t)
	seq := 100
	for range 3 {
		seq++
		totals.Observe(interfaceSkipped(seq))
	}
	var b strings.Builder
	renderShadow(&b, totals)
	out := b.String()
	if !strings.Contains(out, "STALE") {
		t.Fatalf("an unconfirmed belief is reported as though it was just seen:\n%s", out)
	}
	if !strings.Contains(out, "interface    provisional") {
		t.Errorf("a look nobody took withdrew the belief in the report:\n%s", out)
	}
}
