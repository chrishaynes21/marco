package main

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
)

// WHERE THE CROSSINGS THAT BECAME NOTHING WENT.
//
// `relationships seen` counts only the crossings that became candidates. Every other outcome —
// a leg nobody acted on, one carrying several actions, an end Marco can neither recognise nor
// compare — read as though it had not happened at all.
//
// Measured on an accessibility-blind application: three settled screens, ten moves noticed, zero
// candidates, and nothing anywhere saying which door they left by. The candidate ledger held
// evidence from three other applications and none from that one, so the promotion gates were
// never even reached and every question about them was unanswerable.
//
// These hold each production return path to recording its own outcome. A reporting layer that
// re-asked the questions would be a second implementation of the policy, free to agree with the
// code while disagreeing with the run — the same rule the escalation note follows.

// outcomes totals what the observer recorded, across applications.
func outcomes(a *ambientObserver) map[string]int {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := map[string]int{}
	for _, by := range a.crossings {
		for outcome, n := range by {
			out[outcome] += n
		}
	}
	return out
}

// crossingStep is one crossing with the parts each test needs to spoil.
func crossingStep(did []ambient.Act, from, to string, fromShape, toShape *ambient.Shape,
	by ambient.Source) ambient.Step {

	return ambient.Step{
		From: from, To: to, FromShape: fromShape, ToShape: toShape,
		Application: recentApp, By: by, At: time.Now(), Did: did,
	}
}

func oneAct() []ambient.Act {
	return []ambient.Act{{Kind: ambient.Activate,
		Target: ambient.Target{Role: "button", Label: "Bluetooth & devices"}}}
}

// EVERY DOOR OUT OF `noticed` SAYS WHICH ONE IT WAS.
func TestEveryCrossingOutcomeIsRecorded(t *testing.T) {
	for _, tc := range []struct {
		what string
		step ambient.Step
		want string
	}{
		{
			"a crossing that becomes a candidate",
			crossingStep(oneAct(), "seen_state_1", "seen_state_2", homeShape(), btShape(),
				ambient.ByHuman),
			crossingCandidate,
		},
		{
			"Marco's own work, which is never evidence of a habit",
			crossingStep(oneAct(), "seen_state_1", "seen_state_2", homeShape(), btShape(),
				ambient.ByMarco),
			crossingNotHuman,
		},
		{
			"a screen that changed with nothing attributed to it",
			crossingStep(nil, "seen_state_1", "seen_state_2", homeShape(), btShape(),
				ambient.ByHuman),
			crossingNoAction,
		},
		{
			"a leg carrying several actions",
			crossingStep(append(oneAct(), oneAct()...), "seen_state_1", "seen_state_2",
				homeShape(), btShape(), ambient.ByHuman),
			crossingManyActions,
		},
		{
			"a source that is neither a known Place nor a comparable shape",
			crossingStep(oneAct(), "", "seen_state_2", nil, btShape(), ambient.ByHuman),
			crossingSourceUnusable,
		},
		{
			"a destination that is neither",
			crossingStep(oneAct(), "seen_state_1", "", homeShape(), nil, ambient.ByHuman),
			crossingDestinationUnusable,
		},
	} {
		rt, _ := learningRuntime(t)
		a := rt.ambient()
		a.noticed(tc.step, true)

		got := outcomes(a)
		if got[tc.want] != 1 {
			t.Errorf("%s recorded %v, want one %q.\nA crossing that leaves by an "+
				"unrecorded door is indistinguishable from one that never happened, "+
				"which is the silence this exists to end.", tc.what, got, tc.want)
		}
		if total := len(got); total != 1 {
			t.Errorf("%s recorded %d outcomes, want exactly 1: only the first gate "+
				"production actually reached may be counted", tc.what, total)
		}
	}
}

// AND THE COUNTS ARE KEYED BY APPLICATION, WITH NOTHING OFF THE SCREEN IN THEM.
func TestCrossingOutcomesCarryOnlyAnApplicationAndACount(t *testing.T) {
	rt, _ := learningRuntime(t)
	a := rt.ambient()
	a.noticed(crossingStep(oneAct(), "seen_state_1", "seen_state_2", homeShape(), btShape(),
		ambient.ByHuman), true)

	a.mu.Lock()
	byApp := a.crossings
	a.mu.Unlock()
	if len(byApp) != 1 {
		t.Fatalf("%d application(s) recorded, want 1", len(byApp))
	}
	for app, by := range byApp {
		if app != recentApp {
			t.Errorf("recorded against %q, want %q", app, recentApp)
		}
		for outcome := range by {
			// The outcomes are Marco's own closed vocabulary. A control's name, a
			// screen's name or a coordinate appearing here would make a diagnostic the
			// place those leak from.
			switch outcome {
			case crossingCandidate, crossingNoAction, crossingManyActions,
				crossingSourceUnusable, crossingDestinationUnusable, crossingNotHuman:
			default:
				t.Errorf("recorded an outcome %q that is not in the closed vocabulary",
					outcome)
			}
		}
	}
}

// THE REPORT ADDS UP WHAT PRODUCTION RECORDED AND DECIDES NOTHING.
func TestTheCrossingReportDoesNotReAskTheQuestions(t *testing.T) {
	src := mustReadSource(t, "serve.go")
	for _, forbidden := range []string{"ByHuman", "len(s.Did)", "Subject ==", "Shape == nil"} {
		if containsAll(src, forbidden) {
			t.Errorf("the status report reaches %s. It must total what the production "+
				"return sites recorded, never re-decide which of them applied — the two "+
				"would then be free to disagree about the run they describe.", forbidden)
		}
	}
	// And it totals outcomes it was never told about, so a new one is not silently dropped
	// by the reporting layer while production records it faithfully.
	got := crossingTotals(map[string]map[string]int{
		"a": {"something_new": 2, crossingCandidate: 1},
		"b": {"something_new": 3},
	})
	if got["something_new"] != 5 || got[crossingCandidate] != 1 {
		t.Errorf("totals %v lost an outcome the reporter did not recognise", got)
	}
}

// THE REPORT PRINTS OUTCOMES IT WAS NEVER TOLD ABOUT.
//
// Both funnels are rendered through one function, and it holds only an ORDER, never a
// membership list. A reporting layer with a closed list would silently omit a future production
// outcome — production would record it faithfully and nobody would ever see it, which is the
// exact failure these funnels were built to end.
func TestTheFunnelReportDoesNotHoldAClosedList(t *testing.T) {
	out := captureStdout(t, func() {
		printFunnel(map[string]int{
			crossingCandidate:        3,
			"an_outcome_added_later": 7,
			crossingNoAction:         5,
		}, crossingOrder)
	})
	for _, want := range []string{crossingCandidate, crossingNoAction, "an_outcome_added_later"} {
		if !strings.Contains(out, want) {
			t.Errorf("the report dropped %q:\n%s", want, out)
		}
	}
	// The known ones lead, in the order production asks them, so the funnel reads as a funnel.
	if strings.Index(out, crossingCandidate) > strings.Index(out, "an_outcome_added_later") {
		t.Errorf("an unrecognised outcome was printed before the known ones:\n%s", out)
	}
	// AND A ZERO IS NOT PRINTED. A line that is always there and always zero is noise a
	// reader learns to skip, which is how the one that matters gets skipped too.
	quiet := captureStdout(t, func() {
		printFunnel(map[string]int{crossingCandidate: 0}, crossingOrder)
	})
	if strings.TrimSpace(quiet) != "" {
		t.Errorf("a zero outcome was printed:\n%s", quiet)
	}
}
