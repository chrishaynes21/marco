package rehearse

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// The rule for what a walk may hand forward, held where it is decided.
//
// `TestAVerifiedWalkReturnsTheStageItProved` drives the whole walker and holds the reachable
// half: a completed route hands back what it proved, and a walk that did not arrive hands back
// nothing. It cannot reach the other half. A route completes only when its last step came out
// DirectlyVerified, and that outcome is DEFINED as the observation matching the expectation — so
// there is no walk anywhere that produces a completed route whose two fields differ, and a
// mutation swapping them survives the whole suite. Measured.
//
// So it is held here, on the function, with a record no walker can produce. That is the point:
// the guard exists for a future in which a route may complete on something weaker, and the test
// has to be able to describe that future before it arrives.

func provedRecord(observed, expect string) StepRecord {
	return StepRecord{
		Position: 1, Outcome: DirectlyVerified, Observed: observed, Expect: expect,
	}
}

func TestOnlyACompletedWalkProvesAnything(t *testing.T) {
	ref := windowref.Ref{ID: "hwnd:100", Handle: 100, ProcessID: 7,
		Application: "testgame", Generation: 1}
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

	// THE PREMISE, or every case below passes for a function that returns nothing.
	got := provedBy(ref, CompletedRoute, provedRecord("subj_b", "subj_b"), now)
	if got.Subject != "subj_b" || got.From != EvidenceVerifiedOutcome || got.At != now {
		t.Fatalf("a completed route proved %+v", got)
	}

	// THE PROOF IS THE OBSERVATION, and this record is one no walk can currently produce.
	// If a route ever completes on something weaker than a directly verified last step,
	// reading `Expect` would hand the next edge a screen nothing ever resolved.
	if got := provedBy(ref, CompletedRoute, provedRecord("subj_c", "subj_b"), now); got.Subject != "subj_c" {
		t.Errorf("the walk landed on subj_c and proved %q — the proof came from the plan "+
			"rather than from what was seen", got.Subject)
	}

	for _, c := range []struct {
		name     string
		terminal Terminal
		rec      StepRecord
	}{
		{"stopped at a step", StoppedAtStep, provedRecord("subj_b", "subj_b")},
		{"ended unverified", EndedUnverified, provedRecord("subj_b", "subj_b")},
		{"cancelled", CancelledAttempt, provedRecord("subj_b", "subj_b")},
		{"nothing sent", NothingSent, provedRecord("subj_b", "subj_b")},
		{"out of room", BoundsExceeded, provedRecord("subj_b", "subj_b")},
		{"nothing was resolved", CompletedRoute, provedRecord("", "subj_b")},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := provedBy(ref, c.terminal, c.rec, now); got.Subject != "" {
				t.Fatalf("a walk that %s handed back %q as proof of where Marco is "+
					"standing", c.name, got.Subject)
			}
		})
	}
}
