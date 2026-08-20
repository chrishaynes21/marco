package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Changing your mind, and the line between that and pressing a button twice.
//
// The rule this milestone exists to fix: durable means Marco remembers an intentional judgement
// after a restart. It does not mean somebody loses the right to correct themselves. Both halves
// are tested here, because weakening the first while fixing the second would be worse than the
// bug.

// ── the effective judgement is one function ───────────────────────────────────

func TestEffectiveJudgementIsTheOnlyReadingOfAnAnswer(t *testing.T) {
	for _, tc := range []struct {
		status observe.KnowledgeStatus
		want   observe.Judgement
	}{
		{observe.KnowledgeConfirmed, observe.JudgementConfirmed},
		{observe.KnowledgeContradicted, observe.JudgementContradicted},
		// None of these is a person's live position, and each is none for its own reason.
		{observe.KnowledgeObserved, observe.JudgementNone},  // nobody was ever asked
		{observe.KnowledgeDeclined, observe.JudgementNone},  // "I can't tell"
		{observe.KnowledgeRetracted, observe.JudgementNone}, // "don't use my answer"
	} {
		k := observe.SemanticKnowledge{Status: tc.status}
		if got := k.Effective(); got != tc.want {
			t.Errorf("%q reads as %q, want %q", tc.status, got, tc.want)
		}
		if k.Active() != (tc.want != observe.JudgementNone) {
			t.Errorf("%q: Active disagrees with Effective", tc.status)
		}
	}
}

// ── answering stays one-shot; revising is a different verb ────────────────────

func TestAnsweringStaysOneShotAndRevisingIsExplicit(t *testing.T) {
	l, id := answered(t, observe.ResponseContradicted)

	// The protection that was worth keeping: a double submit, a stale panel or a replayed
	// request cannot overwrite what somebody said.
	if _, ok := l.Respond(id, observe.ResponseConfirmed, 2); ok {
		t.Fatal("an answered question was answered again; a second click is not a revision")
	}
	if p := find(t, l, id); p.Response != observe.ResponseContradicted {
		t.Fatalf("the first answer was overwritten: %q", p.Response)
	}

	// And the right that was missing.
	if _, ok := l.Revise(id, observe.ResponseConfirmed, 3); !ok {
		t.Fatal("an explicit revision was refused")
	}
	p := find(t, l, id)
	if p.Response != observe.ResponseConfirmed || p.Status != observe.ProposalAnswered {
		t.Fatalf("after revising, the record reads %q/%q", p.Response, p.Status)
	}
	if p.Retracted {
		t.Error("revising to an answer left the proposal marked withdrawn")
	}
}

// A question nobody has answered cannot be revised. Accepting it would make revision a second
// way to answer, which is the door Respond deliberately closes.
func TestAnUnansweredQuestionCannotBeRevised(t *testing.T) {
	l, id := asked(t)
	if _, ok := l.Revise(id, observe.ResponseConfirmed, 1); ok {
		t.Fatal("a question that was never answered was revised")
	}
	if _, ok := l.Retract(id, 1); ok {
		t.Fatal("a question that was never answered was withdrawn")
	}
	if p := find(t, l, id); p.Status != observe.ProposalOpen {
		t.Fatalf("the open question was disturbed: %q", p.Status)
	}
}

// ── withdrawing ───────────────────────────────────────────────────────────────

func TestWithdrawingLeavesNoActiveJudgementAndSaysSo(t *testing.T) {
	for _, first := range []observe.UserResponse{
		observe.ResponseConfirmed, observe.ResponseContradicted} {

		l, id := answered(t, first)
		if _, ok := l.Retract(id, 5); !ok {
			t.Fatalf("withdrawing a %q was refused", first)
		}
		p := find(t, l, id)
		if !p.Retracted {
			t.Error("a withdrawn answer is not marked as withdrawn")
		}
		if p.Response != observe.ResponseNone {
			t.Errorf("a withdrawn answer still reads as %q", p.Response)
		}
		// Reconsiderable on the ordinary rule — when the evidence changes shape — rather
		// than immediately. Somebody who just said "ignore that" has not asked to be asked
		// again this second.
		if p.Status != observe.ProposalDeclined {
			t.Errorf("a withdrawn answer left status %q", p.Status)
		}
		// And what becomes durable is a WITHDRAWAL, not an absence: an absence would be
		// undone by the next restart.
		if got := observe.KnowledgeStatusForRevision(p); got != observe.KnowledgeRetracted {
			t.Errorf("a withdrawal stores %q, want %q", got, observe.KnowledgeRetracted)
		}
	}
}

func TestARevisionStoresTheNewAnswerAndNotTheOld(t *testing.T) {
	l, id := answered(t, observe.ResponseContradicted)
	if _, ok := l.Revise(id, observe.ResponseConfirmed, 4); !ok {
		t.Fatal("revising was refused")
	}
	p := find(t, l, id)
	if got := observe.KnowledgeStatusForRevision(p); got != observe.KnowledgeConfirmed {
		t.Fatalf("a revision to yes stores %q", got)
	}
	// And back again. A correction is not one-way.
	if _, ok := l.Revise(id, observe.ResponseContradicted, 5); !ok {
		t.Fatal("revising a second time was refused")
	}
	if got := observe.KnowledgeStatusForRevision(find(t, l, id)); got !=
		observe.KnowledgeContradicted {
		t.Fatalf("a revision back to no stores %q", got)
	}
}

// "Not sure" is not "no". It supplies no evidence in either direction.
func TestNotSureSuppliesNoEvidenceEitherWay(t *testing.T) {
	l, id := asked(t)
	if _, ok := l.Respond(id, observe.ResponseDeclined, 1); !ok {
		t.Fatal("declining was refused")
	}
	p := find(t, l, id)
	if p.Status != observe.ProposalDeclined {
		t.Fatalf("a decline left status %q", p.Status)
	}
	k := observe.SemanticKnowledge{Status: observe.KnowledgeStatusFor(p.Response)}
	if k.Effective() != observe.JudgementNone {
		t.Fatalf("a decline reads as %q; it is neither a yes nor a no", k.Effective())
	}
	if k.Answered != 0 {
		t.Error("a decline was counted as an answer")
	}
}

// ── a revision binds to the proposition, not to whatever is current ───────────

func TestARevisionTouchesOnlyTheQuestionItNames(t *testing.T) {
	l := &observe.ProposalLedger{}
	// One at a time: the interruption budget allows one open question, so the first is
	// answered before the second is put. Two propositions, same application.
	a := askOne(t, l, roleHypothesis("button", 24))
	if _, ok := l.Respond(a, observe.ResponseContradicted, 1); !ok {
		t.Fatal("answering the first was refused")
	}
	b := askOne(t, l, roleHypothesis("list_item", 9))
	if a == b {
		t.Fatal("two different propositions produced one question identity")
	}
	if _, ok := l.Respond(b, observe.ResponseContradicted, 2); !ok {
		t.Fatal("answering the second was refused")
	}
	if _, ok := l.Revise(a, observe.ResponseConfirmed, 2); !ok {
		t.Fatal("revising the first was refused")
	}
	if p := find(t, l, b); p.Response != observe.ResponseContradicted {
		t.Fatalf("revising one question changed another: %q is now %q", b, p.Response)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// roleHypothesis is a supported reading the ordinary policy will ask about.
//
// Built the same way proposal_test.go builds one, so these tests exercise questions the real
// policy put rather than proposals appended by hand.
func roleHypothesis(role string, n int) observe.Hypothesis {
	return observe.Hypothesis{
		Kind:   observe.PossibleChoiceGroup,
		Status: observe.StatusSupported,
		Subject: observe.Subject{
			Kind: observe.SubjectGroup, Ref: role,
			Fingerprint: observe.Fingerprint{
				Roles: map[string]int{role: n}, Members: n, Recurrence: 3,
			},
		},
		Episodes: 3,
		Observed: "controls presented together as a set",
		Support: []observe.Evidence{
			{Source: observe.FromStructure, Statement: "controls presented as a set"},
			{Source: observe.FromRecurrence, Statement: "recurred three separate times"},
		},
		Validation: "interact with one and watch the others",
	}
}

func askOne(t *testing.T, l *observe.ProposalLedger, h observe.Hypothesis) observe.ProposalID {
	t.Helper()
	l.Refresh([]observe.Hypothesis{h}, 1, observe.DefaultProposalThresholds())
	id := observe.ProposalIdentity(h)
	for _, p := range l.Proposals {
		if p.ID == id {
			return id
		}
	}
	t.Skipf("the policy did not put this question, so there is nothing to answer")
	return id
}

func asked(t *testing.T) (*observe.ProposalLedger, observe.ProposalID) {
	t.Helper()
	l := &observe.ProposalLedger{}
	return l, askOne(t, l, roleHypothesis("button", 24))
}

func answered(t *testing.T, r observe.UserResponse) (*observe.ProposalLedger, observe.ProposalID) {
	t.Helper()
	l, id := asked(t)
	if _, ok := l.Respond(id, r, 1); !ok {
		t.Fatalf("answering %q was refused", r)
	}
	return l, id
}

func find(t *testing.T, l *observe.ProposalLedger, id observe.ProposalID) observe.Proposal {
	t.Helper()
	for _, p := range l.Proposals {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("no proposal called %s", id)
	return observe.Proposal{}
}
