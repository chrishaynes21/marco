package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The rules of the surface a person corrects their own judgements through.
//
// The wiring tests prove the journey. These prove what the journey is not allowed to do: reach a
// judgement nobody gave, or land on a record other than the one it was told about.

// fakeStore is durable knowledge, as this surface reaches it.
type fakeStore struct{ subjects []observe.RememberedSubject }

func (f *fakeStore) Subjects() []observe.RememberedSubject { return f.subjects }

func (f *fakeStore) Subject(id string) (observe.RememberedSubject, bool) {
	for _, s := range f.subjects {
		if s.ID == id {
			return s, true
		}
	}
	return observe.RememberedSubject{}, false
}

// Remember matches an existing subject by its structure, exactly as the real store does — so a
// write against the wrong signature lands on the wrong record here too, rather than being hidden.
func (f *fakeStore) Remember(application string, sig observe.StructureSignature,
	k observe.SemanticKnowledge) error {

	for i := range f.subjects {
		if observe.CompareStructure(sig, f.subjects[i].Structure) != observe.MatchSame {
			continue
		}
		for j := range f.subjects[i].Knowledge {
			if f.subjects[i].Knowledge[j].Kind == k.Kind {
				f.subjects[i].Knowledge[j] = k
				return nil
			}
		}
		f.subjects[i].Knowledge = append(f.subjects[i].Knowledge, k)
		return nil
	}
	f.subjects = append(f.subjects, observe.RememberedSubject{
		ID: "subj_new", Application: application, Structure: sig,
		Knowledge: []observe.SemanticKnowledge{k},
	})
	return nil
}

// remembered is one subject with a discriminating structure of its own.
func remembered(id string, terms []observe.InterfaceTerm,
	ks ...observe.SemanticKnowledge) observe.RememberedSubject {

	return observe.RememberedSubject{
		ID: id, Application: "testgame", Knowledge: ks, Sessions: 2,
		Structure: observe.StructureSignature{
			Subject: observe.SubjectState, Roles: map[string]int{"button": len(terms) + 4},
			Members: len(terms) + 4, Terms: terms, TermsKnown: true,
		},
	}
}

func said(kind observe.HypothesisKind, status observe.KnowledgeStatus) observe.SemanticKnowledge {
	return observe.SemanticKnowledge{Kind: kind, Status: status, Evidence: "ev", Answered: 1}
}

// Only what a person actually SAID is listed. A guess is not a statement.
func TestOnlyIntentionalJudgementsAreListed(t *testing.T) {
	subjects := []observe.RememberedSubject{
		remembered("subj_a", []observe.InterfaceTerm{observe.TermSettings},
			said(observe.PossibleSettingsLikeState, observe.KnowledgeConfirmed),
			// None of these is something you told Marco.
			said(observe.PossibleMenuLikeState, observe.KnowledgeObserved),
			said(observe.PossibleTextEntryState, observe.KnowledgeDeclined),
			said(observe.PossibleChoiceGroup, observe.KnowledgeRetracted)),
	}
	known := observe.WhatIsKnown(subjects, nil)
	if len(known) != 1 {
		t.Fatalf("listed %d judgements, want 1: %+v", len(known), known)
	}
	if known[0].Kind != observe.PossibleSettingsLikeState {
		t.Errorf("listed %s", known[0].Kind)
	}
	if known[0].Judgement != observe.JudgementConfirmed {
		t.Errorf("the one answer reads as %q", known[0].Judgement)
	}
}

// Locatability comes from what was RECOGNISED, and is never assumed.
func TestAJudgementIsLocatableOnlyWhenSomethingRecognisedIt(t *testing.T) {
	subjects := []observe.RememberedSubject{
		remembered("subj_a", []observe.InterfaceTerm{observe.TermSettings},
			said(observe.PossibleSettingsLikeState, observe.KnowledgeConfirmed)),
		remembered("subj_b", []observe.InterfaceTerm{observe.TermControls},
			said(observe.PossibleSettingsLikeState, observe.KnowledgeContradicted)),
	}
	known := observe.WhatIsKnown(subjects, map[string]bool{"subj_a": true})
	if len(known) != 2 {
		t.Fatalf("listed %d judgements, want 2", len(known))
	}
	for _, k := range known {
		want := k.Subject == "subj_a"
		if k.Locatable != want {
			t.Errorf("%s: locatable=%v, want %v. Being remembered and being findable are "+
				"different things", k.Subject, k.Locatable, want)
		}
	}
}

// A judgement nobody gave cannot be revised. Accepting one would make this a second way to
// answer — reachable without ever having been asked.
func TestOnlyASettledJudgementCanBeCorrected(t *testing.T) {
	for _, status := range []observe.KnowledgeStatus{
		observe.KnowledgeObserved,  // Marco guessed; nobody was asked
		observe.KnowledgeDeclined,  // "I can't tell" is not an answer to change
		observe.KnowledgeRetracted, // already taken back
	} {
		store := &fakeStore{subjects: []observe.RememberedSubject{
			remembered("subj_a", []observe.InterfaceTerm{observe.TermSettings},
				said(observe.PossibleSettingsLikeState, status))}}

		if err := observe.ReviseKnown(store, "subj_a",
			observe.PossibleSettingsLikeState, observe.ResponseConfirmed); err == nil {
			t.Errorf("%q was revised, but nobody had answered it", status)
		}
		if err := observe.RetractKnown(store, "subj_a",
			observe.PossibleSettingsLikeState); err == nil {
			t.Errorf("%q was withdrawn, but there was no answer to withdraw", status)
		}
		if got := store.subjects[0].Knowledge[0].Status; got != status {
			t.Errorf("the record changed anyway: %q → %q", status, got)
		}
	}
}

// A correction lands on the judgement it names, and on nothing else.
func TestACorrectionLandsOnTheSubjectItNames(t *testing.T) {
	store := &fakeStore{subjects: []observe.RememberedSubject{
		remembered("subj_a", []observe.InterfaceTerm{observe.TermSettings},
			said(observe.PossibleSettingsLikeState, observe.KnowledgeContradicted)),
		remembered("subj_b", []observe.InterfaceTerm{observe.TermControls},
			said(observe.PossibleSettingsLikeState, observe.KnowledgeContradicted)),
	}}
	if err := observe.ReviseKnown(store, "subj_b",
		observe.PossibleSettingsLikeState, observe.ResponseConfirmed); err != nil {
		t.Fatalf("revising: %v", err)
	}
	if len(store.subjects) != 2 {
		t.Fatalf("a correction changed how many subjects exist: %d", len(store.subjects))
	}
	for _, s := range store.subjects {
		want := observe.JudgementContradicted
		if s.ID == "subj_b" {
			want = observe.JudgementConfirmed
		}
		if got := s.Knowledge[0].Effective(); got != want {
			t.Errorf("%s now reads %q, want %q. A correction reached a record it was not "+
				"told about", s.ID, got, want)
		}
	}
}

// Withdrawing one interpretation leaves the subject's others alone.
func TestWithdrawingOneJudgementLeavesTheSubjectsOthers(t *testing.T) {
	store := &fakeStore{subjects: []observe.RememberedSubject{
		remembered("subj_a", []observe.InterfaceTerm{observe.TermSettings},
			said(observe.PossibleSettingsLikeState, observe.KnowledgeConfirmed),
			said(observe.PossibleMenuLikeState, observe.KnowledgeConfirmed))}}

	if err := observe.RetractKnown(store, "subj_a",
		observe.PossibleSettingsLikeState); err != nil {
		t.Fatalf("withdrawing: %v", err)
	}
	s := store.subjects[0]
	if len(s.Knowledge) != 2 {
		t.Fatalf("the subject holds %d interpretations after one was withdrawn", len(s.Knowledge))
	}
	if got, _ := s.Find(observe.PossibleSettingsLikeState); got.Status != observe.KnowledgeRetracted {
		t.Errorf("the withdrawn interpretation reads %q; a withdrawal has to be durable, or "+
			"the next restart undoes it", got.Status)
	}
	if got, _ := s.Find(observe.PossibleMenuLikeState); got.Effective() !=
		observe.JudgementConfirmed {
		t.Errorf("the untouched interpretation now reads %q", got.Effective())
	}
}

// The sentence a person reads back says which way they answered.
func TestTheSentenceSaysWhichWayYouAnswered(t *testing.T) {
	for _, kind := range []observe.HypothesisKind{
		observe.PossibleChoiceGroup, observe.PossibleMenuLikeState,
		observe.PossibleSettingsLikeState, observe.PossibleTextEntryState,
		observe.PossibleReversiblePlace, observe.PossibleTransitionAction,
		observe.PossibleSelectionSequence,
	} {
		yes := observe.SaidAbout(kind, observe.JudgementConfirmed)
		no := observe.SaidAbout(kind, observe.JudgementContradicted)
		if yes == no {
			t.Errorf("%s reads the same whether you said yes or no: %q", kind, yes)
		}
		if yes == "" || no == "" {
			t.Errorf("%s has no sentence", kind)
		}
	}
}
