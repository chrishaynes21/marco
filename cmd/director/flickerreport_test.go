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
	// The wording changed with ADR-153: `state_unknown` is no longer a destination, so a
	// dropout is no longer a refused proposal to it. The number means the same thing to a
	// reader — how much of the session Marco could not read — and the line now says that
	// directly instead of describing the refusal of a journey nobody can propose.
	if !strings.Contains(out, "carried as uncertainty") {
		t.Errorf("the report does not report the looks it could not place:\n%s", out)
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

// A SESSION THAT STOPPED KNOWING WHERE THE PLAYER WAS HAS TO SAY SO.
//
// `state_unknown` used to be a destination, so a blackout appeared in the transition table and
// a reader could see it there. It cannot be one any more — [[ADR-153-a-blackout-is-not-a-destination]] — which means this
// report is the ONLY place the event appears at all. A session that lost track would otherwise
// be indistinguishable from one that never did.
//
// Deleting the expiry lines in renderFlicker must fail this.
func TestTheReportSaysWhenItStoppedKnowingWhereYouAre(t *testing.T) {
	var b strings.Builder
	renderFlicker(&b, observe.FlickerStats{
		Proposed: 4, Confirmed: 2, Rejected: 2, Window: 2,
		Dropouts: 9, Expired: 2, LongestBlackout: 6, ChangesLostToBlackout: 1,
	}, "", 0, 0)
	out := b.String()

	if !strings.Contains(out, "STOPPED KNOWING") {
		t.Errorf("a session that twice gave up on knowing which screen was in front of "+
			"it does not say so:\n%s", out)
	}
	if !strings.Contains(out, "longest blackout 6") {
		t.Errorf("the report does not say how long the worst blackout was; expiry alone "+
			"cannot tell four seconds from four minutes:\n%s", out)
	}
	if !strings.Contains(out, "no adjacency is recorded") {
		t.Error("the report reports an expiry without saying what was NOT recorded. A " +
			"reader has to be able to tell this from a change into nowhere, which is " +
			"exactly what it used to be")
	}
	// And the detector's loss is not presented as the gate's.
	if !strings.Contains(out, "still being confirmed when the screen went dark") {
		t.Errorf("a candidate lost to a blackout is not reported:\n%s", out)
	}
	if strings.Contains(out, "gate's own error rate") {
		t.Error("a session with no refusals that navigation preceded printed the gate's " +
			"error rate anyway; a change nobody could judge is the detector's loss")
	}
}
