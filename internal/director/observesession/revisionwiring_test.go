package observesession_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Changing your mind has to survive the file.
//
// `observe`'s own revision tests prove the ledger's rules. These prove the JOURNEY: a person
// answers, corrects themselves, the process ends, and a later session reads the correction rather
// than the mistake. A revision that lived only in the session that made it would be undone by the
// next restart — which is the same failure durability exists to prevent, pointed the other way.
//
// Both tests run a real session through the production constructor, answer through the runner, and
// reopen the store on disk. Nothing here reaches into storage.

// THE revision test. Named in two production comments as the thing their deletion must fail.
func TestARevisedAnswerIsWhatALaterSessionRecalls(t *testing.T) {
	dir := t.TempDir()

	// ── Session A: asked, answered wrongly, corrected ──
	first, _ := sessionOver(t, memoryAt(t, dir), settingsSession())
	asked := questionAbout(t, first, observe.PossibleSettingsLikeState)
	if _, ok := first.Respond(asked.ID, observe.ResponseContradicted); !ok {
		t.Fatal("session A refused the answer")
	}
	if _, ok := first.Revise(asked.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("session A refused an explicit revision")
	}

	// ── Session B: a new store handle over the same file, a renumbered sampler ──
	second, resultB := sessionOver(t, memoryAt(t, dir), renumberedSettingsSession())

	var found bool
	for _, p := range second.Proposals().Proposals {
		if p.ID != asked.ID {
			continue
		}
		found = true
		if p.Response != observe.ResponseConfirmed {
			t.Errorf("session B recalled %q. The user corrected this before the restart and "+
				"the correction was lost", p.Response)
		}
		if p.Status != observe.ProposalAnswered {
			t.Errorf("a revised answer came back as %q", p.Status)
		}
		if p.Retracted {
			t.Error("a revision to an answer came back marked withdrawn")
		}
	}
	if !found {
		t.Fatal("session B recalled nothing about a question session A answered")
	}

	// And what reaches the rest of the system is the correction, not the mistake.
	for _, h := range resultB.Hypotheses {
		if h.Kind != observe.PossibleSettingsLikeState || h.UserValidation == nil {
			continue
		}
		if h.UserValidation.Response != observe.ResponseConfirmed {
			t.Errorf("the hypothesis carries %q, so everything downstream still believes "+
				"the answer the user took back", h.UserValidation.Response)
		}
	}
}

// A withdrawal is itself durable: the old answer does not come back on restart.
func TestAWithdrawnAnswerDoesNotComeBackAfterARestart(t *testing.T) {
	for _, first := range []observe.UserResponse{
		observe.ResponseConfirmed, observe.ResponseContradicted} {

		dir := t.TempDir()
		a, _ := sessionOver(t, memoryAt(t, dir), settingsSession())
		asked := questionAbout(t, a, observe.PossibleSettingsLikeState)
		if _, ok := a.Respond(asked.ID, first); !ok {
			t.Fatalf("session A refused %q", first)
		}
		if _, ok := a.Retract(asked.ID); !ok {
			t.Fatalf("withdrawing a %q was refused", first)
		}

		b, resultB := sessionOver(t, memoryAt(t, dir), renumberedSettingsSession())
		for _, p := range b.Proposals().Proposals {
			if p.ID != asked.ID {
				continue
			}
			if p.Response != observe.ResponseNone {
				t.Errorf("after withdrawing a %q, a later session recalled %q",
					first, p.Response)
			}
			if !p.Retracted {
				t.Errorf("after withdrawing a %q, the later session does not know it was "+
					"withdrawn, so it cannot say so", first)
			}
			if p.Status == observe.ProposalAnswered {
				t.Errorf("a withdrawn answer came back as answered")
			}
		}
		// Nothing downstream inherits a judgement the user took back.
		for _, h := range resultB.Hypotheses {
			if h.Kind != observe.PossibleSettingsLikeState || h.UserValidation == nil {
				continue
			}
			t.Errorf("a hypothesis carries %q after the answer was withdrawn",
				h.UserValidation.Response)
		}
	}
}

// Withdrawing a judgement does not withdraw what was seen.
//
// A person taking back an answer is making a claim about their own reliability, not about the
// application. The subject, its structure and its visits are none of revision's business.
func TestARevisionLeavesWhatWasObservedAlone(t *testing.T) {
	dir := t.TempDir()
	a, _ := sessionOver(t, memoryAt(t, dir), settingsSession())
	asked := questionAbout(t, a, observe.PossibleSettingsLikeState)
	if _, ok := a.Respond(asked.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("session A refused the answer")
	}
	before := memoryAt(t, dir).Subjects()
	if len(before) == 0 {
		t.Fatal("nothing was stored")
	}

	if _, ok := a.Retract(asked.ID); !ok {
		t.Fatal("withdrawing was refused")
	}
	after := memoryAt(t, dir).Subjects()

	if len(after) != len(before) {
		t.Fatalf("withdrawing an answer changed the number of remembered subjects, %d → %d",
			len(before), len(after))
	}
	for i, was := range before {
		got := after[i]
		if got.ID != was.ID {
			t.Errorf("subject %d changed identity when an answer was withdrawn", i)
		}
		if got.Called != was.Called {
			t.Errorf("withdrawing an answer changed what the subject is CALLED: %q → %q",
				was.Called, got.Called)
		}
		if got.Sessions != was.Sessions {
			t.Errorf("withdrawing an answer changed the visit count: %d → %d",
				was.Sessions, got.Sessions)
		}
		if len(got.Knowledge) != len(was.Knowledge) {
			t.Errorf("withdrawing one answer changed how many interpretations are held: "+
				"%d → %d", len(was.Knowledge), len(got.Knowledge))
		}
	}
}

// A revision writes against the question's OWN proposition, not against whatever else is known.
func TestARevisionWritesAgainstTheQuestionItNames(t *testing.T) {
	dir := t.TempDir()
	a, _ := sessionOver(t, memoryAt(t, dir), settingsSession())

	open := a.Proposals().Open()
	var target, other observe.Proposal
	for _, p := range open {
		if p.Ask != "" && p.Ask != observe.AskSemantic {
			continue
		}
		if target.ID == "" {
			target = p
			continue
		}
		if p.Kind != target.Kind && other.ID == "" {
			other = p
		}
	}
	if target.ID == "" || other.ID == "" {
		t.Skip("this fixture did not produce two semantic questions to tell apart")
	}
	if _, ok := a.Respond(target.ID, observe.ResponseContradicted); !ok {
		t.Fatal("answering the target was refused")
	}
	if _, ok := a.Respond(other.ID, observe.ResponseContradicted); !ok {
		t.Fatal("answering the other was refused")
	}
	if _, ok := a.Revise(target.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("revising was refused")
	}

	for _, s := range memoryAt(t, dir).Subjects() {
		for _, k := range s.Knowledge {
			switch k.Kind {
			case target.Kind:
				if k.Effective() != observe.JudgementConfirmed {
					t.Errorf("the revised interpretation %s reads %q durably",
						k.Kind, k.Effective())
				}
			case other.Kind:
				if k.Effective() != observe.JudgementContradicted {
					t.Errorf("revising %s changed what is known about %s: now %q",
						target.Kind, k.Kind, k.Effective())
				}
			}
		}
	}
}
