package demonstrated_test

// HOW A RUN ENDED, WHEN THE SENTENCES ALONE CANNOT SAY.
//
// Every other test of this bridge drives the real runtime, which is right — but the case these
// cover cannot be produced deterministically there. Cancelling a run aborts the frame tree, so
// whether the second sentence is reached and refused, or never reached at all, depends on which
// of two cancellations lands first. Measured at roughly one run in six.
//
// The integration tests assert what is true in every ordering. These assert the arithmetic of the
// ordering that is hardest to produce on purpose: every interaction the play REACHED was
// verified, and it did not reach them all.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

func verified(step int) demonstrated.Ask {
	return demonstrated.Ask{Name: "an interaction", Step: step,
		Verdict: workflowrun.VerdictVerified, Decided: true}
}

// HALF A ROUTINE IS NOT A FINISHED ONE.
//
// The defect: a run cancelled as its first interaction finished never reaches the second
// sentence, so the host is never called about it, so every record there is says verified — and
// `Outcome` said `verified` about a two-interaction routine that performed one. Of everything
// this bridge could get wrong, telling somebody a routine completed when half of it ran is the
// worst, because nothing about it looks like a failure.
func TestARunThatDidNotReachEverySentenceIsNotVerified(t *testing.T) {
	a := demonstrated.NewAccount("r_1", 2)
	a.Note(verified(0))
	if got := a.Outcome(); got == runref.Verified {
		t.Fatalf("one verified interaction of two reported %q", got)
	}
	if got := a.Outcome(); got != runref.Unconfirmed {
		t.Fatalf("outcome = %q, want unconfirmed — nobody ever said anything about "+
			"the second interaction", got)
	}
	// AND A CANCELLED ONE SAYS SO, because "somebody stopped this" and "this could not
	// be checked" send a person to different places.
	a.Cut("the run was cancelled before every interaction was reached", true)
	if got := a.Outcome(); got != runref.Cancelled {
		t.Fatalf("outcome = %q, want cancelled", got)
	}
	// AND THE ONE-LINE ACCOUNT SAYS WHAT STOPPED IT, because "0 of 2 verified" without
	// a reason is a number somebody has to go and investigate. This sentence is what
	// reaches the durable record as its `because`.
	s := a.Short()
	if !strings.Contains(s, "1 of 2 interaction(s) verified") {
		t.Fatalf("the account does not say how far it got: %q", s)
	}
	if !strings.Contains(s, "cancelled before every interaction was reached") {
		t.Fatalf("the account does not say what stopped it: %q", s)
	}
}

// AND A RUN THAT DID REACH THEM ALL IS VERIFIED.
//
// The limit of the rule above, which is what keeps it from being an overcorrection.
func TestARunThatVerifiedEveryInteractionIsVerified(t *testing.T) {
	a := demonstrated.NewAccount("r_1", 2)
	a.Note(verified(0))
	a.Note(verified(1))
	if got := a.Outcome(); got != runref.Verified {
		t.Fatalf("outcome = %q, want verified", got)
	}
}

// A RUN THAT ASKED FOR NOTHING IS NOT A RUN THAT WORKED.
func TestARunThatAskedForNothingIsRefusedOrCancelled(t *testing.T) {
	a := demonstrated.NewAccount("r_1", 2)
	if got := a.Outcome(); got != runref.Refused {
		t.Fatalf("outcome = %q, want refused", got)
	}
	b := demonstrated.NewAccount("r_2", 2)
	b.Cut("cancelled before anything was attempted", true)
	if got := b.Outcome(); got != runref.Cancelled {
		t.Fatalf("outcome = %q, want cancelled", got)
	}
}

// AND A VERDICT OUTRANKS A REFUSAL, which is the ordering that cost a correction.
//
// The coordinator refuses to carry a walk forward BECAUSE of a verdict, so the attempt it stops
// on carries both. Reading the refusal first reported every contradicted press as `refused` —
// which says Marco pressed nothing, about a run that pressed.
func TestAVerdictIsReportedBeforeTheRefusalItCaused(t *testing.T) {
	a := demonstrated.NewAccount("r_1", 2)
	a.Note(demonstrated.Ask{Name: "an interaction", Step: 0,
		Verdict: workflowrun.VerdictContradicted, Decided: true,
		Refused: "refused at screen not established here — its outcome is contradicted"})
	if got := a.Outcome(); got != runref.Contradicted {
		t.Fatalf("outcome = %q, want contradicted", got)
	}
}

// A REFUSAL WITH NO VERDICT IS A REFUSAL, which is the other side of the same ordering: nothing
// was attempted, so there is nothing to have a verdict about.
func TestARefusalWithNoVerdictIsReportedAsRefused(t *testing.T) {
	a := demonstrated.NewAccount("r_1", 1)
	a.Note(demonstrated.Ask{Name: "an interaction", Step: 0,
		Refused: "the control could not be re-acquired"})
	if got := a.Outcome(); got != runref.Refused {
		t.Fatalf("outcome = %q, want refused", got)
	}
}
