package main

// WHAT AN ACT'S RESULT IS, WHEN THE ACT IS NOT THE WHOLE DEMONSTRATION.
//
// An Act may legitimately perform a SUBSET of a longer demonstration, or its interactions in
// another order. The artifact cursor cannot describe either: it reaches the end of the artifact or
// it does not, and for a one-interaction Act over a three-step routine it never does.
//
// So a run that did exactly what its play said — every sentence verified — was recorded as
// REFUSED and exited 1, because `rep.Progress.Complete()` was false. The person's routine worked
// and Marco said it had declined.
//
// These drive the production coordinator, the production verifier and the production executor
// over a notebook where the mouse would be, through the real runtime reading the play's own
// bytes. No test here emits desktop input.

import (
	"context"
	"io"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

// anActOver runs a play composed of just the named interactions of the two-tap routine, in that
// order, and returns the account and the runner's own report.
func anActOver(t *testing.T, picked []demonstrated.Named,
	world func() *dpStage) (*demonstrated.Account, bool, int) {

	t.Helper()
	art := dpATwoTapPanel()
	now := dpLooked
	s := dpSave(t, art, picked)
	src, binds := s.dpReload(t)
	plan, err := demonstrated.Preflight(src, binds, dpAStore{art.Name: art}, marcoChecks{})
	if err != nil {
		t.Fatalf("the Act does not preflight: %v", err)
	}
	run, bind, _, _, book := dpARun(t, art, &now)
	st := world()
	st.book, st.now = book, &now
	acc, _ := performDemonstratedPlay(context.Background(), dpTheRun, plan,
		demonstrated.NewSupervised(run, bind, st.look, 240), nil, io.Discard)
	return acc, run.Report().Progress.Complete(), book.clicks()
}

// A ONE-INTERACTION ACT OVER A TWO-INTERACTION DEMONSTRATION IS VERIFIED WHEN IT VERIFIES.
//
// And the cursor says otherwise, which is the point: the two disagree, and the one that is right
// about an ACT is the account.
func TestASubsetActThatVerifiesIsRecordedAsVerified(t *testing.T) {
	acc, cursorComplete, clicks := anActOver(t,
		[]demonstrated.Named{{Position: 0, Name: "the graphics tab"}},
		func() *dpStage { return dpAPanel(dpPanelWords(dpFirstOutcome()...)) })

	if got := outcomeOfAct(acc, false); got != RunVerified {
		t.Fatalf("a subset Act that verified everything it asked for was recorded "+
			"%q: %+v", got, acc.Asked())
	}
	if clicks != 1 {
		t.Fatalf("%d press(es) went out, want 1", clicks)
	}
	// THE DISAGREEMENT, ASSERTED. If the cursor were complete this test would pass
	// whichever classifier was used, and would be proving nothing.
	if cursorComplete {
		t.Fatal("the artifact cursor completed, so this fixture no longer " +
			"distinguishes the account from the cursor")
	}
	if got := outcomeOfRun(runReportOf(t, acc), false); got == RunVerified {
		t.Fatal("the cursor-based classifier agrees here, so the two cannot be told " +
			"apart by this test")
	}
}

// AND A REORDERED ACT TOO, which the cursor can describe even less.
func TestAReorderedActThatVerifiesIsRecordedAsVerified(t *testing.T) {
	acc, cursorComplete, clicks := anActOver(t, []demonstrated.Named{
		{Position: 1, Name: "apply changes"},
		{Position: 0, Name: "the graphics tab"},
	}, func() *dpStage {
		return dpAPanel(dpPanelWords(dpSecondOutcome()...),
			dpPanelWords(dpFirstOutcome()...))
	})

	if got := outcomeOfAct(acc, false); got != RunVerified {
		t.Fatalf("a reordered Act that verified both interactions was recorded %q: %+v",
			got, acc.Asked())
	}
	if clicks != 2 {
		t.Fatalf("%d press(es) went out, want 2", clicks)
	}
	if cursorComplete {
		t.Fatal("the artifact cursor completed; this fixture no longer distinguishes")
	}
}

// AND PARTIAL EXECUTION NEVER BECOMES VERIFIED.
//
// The limit that keeps the rule above from being a way to call anything a success: the same
// two-interaction Act, with the first interaction's outcome withheld, must not be verified by
// either classifier.
func TestAPartialActIsNeverVerified(t *testing.T) {
	acc, _, clicks := anActOver(t, dpTheNames(), func() *dpStage {
		s := dpAPanel(nil, nil)
		s.limit = 120
		return s
	})
	if got := outcomeOfAct(acc, false); got == RunVerified {
		t.Fatalf("an Act whose first interaction was never verified was recorded "+
			"%q: %+v", got, acc.Asked())
	}
	if clicks != 1 {
		t.Fatalf("%d press(es) went out, want exactly 1", clicks)
	}
}

// AND A STOP OUTRANKS THE ACCOUNT, because somebody chose it.
func TestAStoppedActIsRecordedAsCancelledWhateverElseItDid(t *testing.T) {
	acc, _, _ := anActOver(t,
		[]demonstrated.Named{{Position: 0, Name: "the graphics tab"}},
		func() *dpStage { return dpAPanel(dpPanelWords(dpFirstOutcome()...)) })
	if got := outcomeOfAct(acc, true); got != RunCancelled {
		t.Fatalf("a stopped run was recorded %q", got)
	}
}

// runReportOf is an empty report with the account's own press count, for the comparison above.
//
// Deliberately thin: what the second assertion in the subset test needs is "what would the
// cursor-based classifier have said", and the answer rests on `Progress.Complete()` being false,
// which an empty report already is.
func runReportOf(t *testing.T, acc *demonstrated.Account) workflowrun.Report {
	t.Helper()
	var rep workflowrun.Report
	for _, k := range acc.Asked() {
		if k.Verified() {
			rep.VerifiedOK++
		}
	}
	return rep
}
