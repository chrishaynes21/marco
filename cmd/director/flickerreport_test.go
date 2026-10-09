package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Can a reader tell forty transitions from a busy player apart from forty from a shaky
// detector?
//
// The live run printed `state_transitions: 40, every one unattributed` and nothing else. The
// number was correct and unreadable: the two causes have nothing in common except the count,
// and only the gate's own account distinguishes them.

func TestTheSessionReportsWhatTheFlickerGateRefused(t *testing.T) {
	// A session shaped like the live one: a handful of real changes, a pile of refusals.
	var totals observe.ShadowTotals
	a, b := flickerScreens()
	seq := []observe.ShadowSample{
		shadowOf(a), shadowOf(a), shadowOf(b), shadowOf(a), shadowOf(a),
		shadowOf(b), shadowOf(b), shadowOf(b),
	}
	for _, s := range seq {
		totals.Add(s)
	}

	if totals.Flicker.Proposed == 0 {
		t.Fatal("the gate's account never reached the totals; nothing below is about the " +
			"report")
	}

	var sb strings.Builder
	renderShadow(&sb, totals)
	out := sb.String()

	if !strings.Contains(out, "refused as fluctuation") {
		t.Errorf("the report does not say the gate refused anything:\n%s", out)
	}
	if !strings.Contains(out, "consecutive look") {
		t.Errorf("the report does not state the rule its numbers were produced under:\n%s",
			out)
	}
}

// AND IT PRINTS THE MISS DIRECTION, which is the unflattering half.
func TestTheReportPrintsTheGatesOwnErrorRate(t *testing.T) {
	var sb strings.Builder
	renderFlicker(&sb, observe.FlickerStats{
		Proposed: 10, Confirmed: 2, Rejected: 8, RejectedWithInput: 3,
		Dropouts: 2, Window: 2,
	}, "", 0, 0)
	out := sb.String()

	if !strings.Contains(out, "may have been REAL changes") {
		t.Errorf("the report hides the refusals that navigation preceded:\n%s\n"+
			"A gate that printed only the fluctuations it removed would be reporting "+
			"its successes.", out)
	}
	if !strings.Contains(out, "gate's own error rate") {
		t.Errorf("the report does not name those as the gate's error:\n%s", out)
	}
	if !strings.Contains(out, "without inventing a journey") {
		t.Errorf("the report does not report the dropouts it stepped over:\n%s", out)
	}
}

// And a change in progress is visible rather than the screen looking settled.
func TestTheReportSaysWhenTheScreenIsChangingRightNow(t *testing.T) {
	var sb strings.Builder
	renderFlicker(&sb, observe.FlickerStats{Proposed: 1, Window: 2}, "state_4", 1, 1)
	out := sb.String()
	if !strings.Contains(out, "CHANGING NOW") {
		t.Errorf("a screen mid-change reads as settled:\n%s", out)
	}
	if !strings.Contains(out, "state_4") {
		t.Errorf("the report does not name the candidate:\n%s", out)
	}
}

// A settled session with nothing refused prints nothing, so the panel stays readable.
func TestASettledSessionPrintsNoFlickerPanel(t *testing.T) {
	var sb strings.Builder
	renderFlicker(&sb, observe.FlickerStats{}, "", 0, 0)
	if sb.String() != "" {
		t.Errorf("a session with no proposals printed a panel:\n%s", sb.String())
	}
}

// ── fixtures ───────────────────────────────────────────────────────────────────

func flickerScreens() (a, b []observe.ShadowRegion) {
	rows := func(y0 float64) []observe.ShadowRegion {
		var out []observe.ShadowRegion
		for i := range 4 {
			out = append(out, observe.ShadowRegion{
				Role: "button",
				Region: observe.Region{
					X: 0.4, Y: y0 + float64(i)*0.042, Width: 0.17, Height: 0.036,
				},
			})
		}
		return out
	}
	return rows(0.10), rows(0.60)
}

func shadowOf(regions []observe.ShadowRegion) observe.ShadowSample {
	return observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true,
		Regions: regions, Detections: len(regions), LatencyMS: 900,
	}
}
