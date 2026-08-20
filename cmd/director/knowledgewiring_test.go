package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// What Marco Knows, and the door it opens that nothing else could.
//
// Every other correction path reaches an answer through the question that produced it. That works
// until the subject stops being recognisable — and then the answer is uncorrectable forever, which
// is the exact state the real Explorer record is in. These tests run that scenario: a person
// answers, the service restarts, nothing recognises the subject any more, and they can still see
// what they said and take it back.
//
// Nothing here types a command or touches a file. Every step is a service request.

// knows puts one durable-knowledge request through the production request path.
func knows(t *testing.T, g *observationRegistry, k service.ObserveKnows) []observe.KnownJudgement {
	t.Helper()
	rt := &Runtime{observations: g}
	out, err := rt.Observation(service.ObserveQuery{Knows: &k})
	if err != nil {
		t.Fatalf("Observation: %v", err)
	}
	view, ok := out.(knowledgeView)
	if !ok {
		t.Fatalf("the handler returned %T", out)
	}
	return view.Known
}

// restarted is a new registry over the same file, holding no sessions.
//
// What a service restart actually is, as far as this surface is concerned: the durable records
// survive and every live question is gone.
func restarted(t *testing.T, store *semanticmemory.Store) *observationRegistry {
	t.Helper()
	reopened, why := semanticmemory.Open(store.Path())
	if why != "" {
		t.Fatalf("reopening memory: %s", why)
	}
	g := newObservationRegistry()
	g.memory = reopened
	return g
}

// only returns the single judgement the fixture produced, so a test can name it.
func only(t *testing.T, known []observe.KnownJudgement) observe.KnownJudgement {
	t.Helper()
	if len(known) != 1 {
		t.Fatalf("expected exactly one judgement, got %d: %+v", len(known), known)
	}
	return known[0]
}

// THE acceptance test for the milestone. A judgement nothing can recall any more is still
// inspectable, still correctable, and stays corrected across a restart.
//
// Deleting the registry.s RetractKnown call must fail this. (ReviseKnown has its own holder,
// TestChangingAnAnswerInTheProductSurvivesARestart — this one never changes an answer, only
// takes it back, so neutering Revise leaves it green.)
func TestAJudgementNothingRecognisesIsStillCorrectableInTheProduct(t *testing.T) {
	g, store, q := reviseFixture(t)
	ask(t, g, service.ObserveQuery{Answer: &service.ObserveAnswer{
		ProposalID: string(q.ID), Response: string(observe.ResponseContradicted)}})

	// The service restarts. No session holds the question now, so no revision path that
	// needs one exists any more.
	after := restarted(t, store)
	said := only(t, knows(t, after, service.ObserveKnows{}))

	if said.Judgement != observe.JudgementContradicted {
		t.Fatalf("the surface reads the answer as %q", said.Judgement)
	}
	if said.Locatable {
		t.Error("a subject no session has recognised is offered as something Marco can " +
			"point at. Remembering a judgement is not the same as being able to find it")
	}
	if !strings.Contains(said.Said, "NOT") {
		t.Errorf("a person cannot tell from %q that they said no", said.Said)
	}
	if said.Subject == "" || said.Kind == "" {
		t.Fatal("the surface cannot name what it would correct")
	}

	// They take it back, through the surface.
	left := knows(t, after, service.ObserveKnows{
		Subject: said.Subject, Kind: string(said.Kind), Withdraw: true})
	if len(left) != 0 {
		t.Fatalf("a withdrawn answer is still listed as something you told Marco: %+v", left)
	}

	// And it stays withdrawn. This is the whole claim: durable means the correction survives,
	// not that the mistake does.
	if again := knows(t, restarted(t, store), service.ObserveKnows{}); len(again) != 0 {
		t.Fatalf("the withdrawn answer came back after a restart: %+v", again)
	}
}

// Changing an answer through the surface changes what a later process reads.
func TestChangingAnAnswerInTheProductSurvivesARestart(t *testing.T) {
	g, store, q := reviseFixture(t)
	ask(t, g, service.ObserveQuery{Answer: &service.ObserveAnswer{
		ProposalID: string(q.ID), Response: string(observe.ResponseContradicted)}})

	after := restarted(t, store)
	said := only(t, knows(t, after, service.ObserveKnows{}))
	changed := only(t, knows(t, after, service.ObserveKnows{
		Subject: said.Subject, Kind: string(said.Kind),
		Response: string(observe.ResponseConfirmed)}))

	if changed.Judgement != observe.JudgementConfirmed {
		t.Fatalf("after changing the answer the surface reads %q", changed.Judgement)
	}
	if strings.Contains(changed.Said, "NOT") {
		t.Errorf("the sentence still reads as a refusal: %q", changed.Said)
	}
	// The subject is the same one. A correction that moved to another record would be
	// answering a different question.
	if changed.Subject != said.Subject {
		t.Errorf("the correction landed on %s, not on %s", changed.Subject, said.Subject)
	}

	if durable := only(t, knows(t, restarted(t, store), service.ObserveKnows{})); durable.Judgement != observe.JudgementConfirmed {
		t.Fatalf("a restart reads %q; the correction was lost", durable.Judgement)
	}
}

// Withdrawing takes back the answer and nothing else.
func TestWithdrawingLeavesTheSubjectAndEverythingElseAlone(t *testing.T) {
	g, store, q := reviseFixture(t)
	ask(t, g, service.ObserveQuery{Answer: &service.ObserveAnswer{
		ProposalID: string(q.ID), Response: string(observe.ResponseContradicted)}})

	before := restarted(t, store).knowledgeMustExist(t)
	said := only(t, knows(t, restarted(t, store), service.ObserveKnows{}))

	after := restarted(t, store)
	knows(t, after, service.ObserveKnows{
		Subject: said.Subject, Kind: string(said.Kind), Withdraw: true})

	got := restarted(t, store).knowledgeMustExist(t)
	if len(got) != len(before) {
		t.Fatalf("withdrawing an answer changed how many subjects are remembered, %d → %d",
			len(before), len(got))
	}
	for i, was := range before {
		if got[i].ID != was.ID {
			t.Errorf("subject %d changed identity", i)
		}
		if got[i].Called != was.Called {
			t.Errorf("withdrawing changed what a subject is CALLED: %q → %q",
				was.Called, got[i].Called)
		}
		if got[i].Sessions != was.Sessions {
			t.Errorf("withdrawing changed the visit count: %d → %d",
				was.Sessions, got[i].Sessions)
		}
		if len(got[i].Knowledge) != len(was.Knowledge) {
			t.Errorf("withdrawing one answer changed how many interpretations are held: "+
				"%d → %d", len(was.Knowledge), len(got[i].Knowledge))
		}
	}
}

// A judgement nobody gave cannot be revised, and a surface cannot invent one.
func TestOnlySomethingYouAnsweredCanBeChanged(t *testing.T) {
	g, store, _ := reviseFixture(t)
	_ = g
	after := restarted(t, store)
	rt := &Runtime{observations: after}

	// Nothing was answered, so nothing is listed and nothing is changeable.
	if listed := knows(t, after, service.ObserveKnows{}); len(listed) != 0 {
		t.Fatalf("an unanswered observation is listed as something you told Marco: %+v", listed)
	}
	for _, k := range []service.ObserveKnows{
		{Subject: "subj_nothing", Kind: string(observe.PossibleChoiceGroup), Withdraw: true},
		{Subject: "subj_nothing", Kind: string(observe.PossibleChoiceGroup),
			Response: string(observe.ResponseConfirmed)},
	} {
		if _, err := rt.Observation(service.ObserveQuery{Knows: &k}); err == nil {
			t.Errorf("%+v was accepted against a subject nobody has answered anything about", k)
		}
	}
}

// The closed vocabulary stays closed on this path too.
func TestAKnowledgeCorrectionOutsideTheClosedVocabularyIsRefused(t *testing.T) {
	g, store, q := reviseFixture(t)
	ask(t, g, service.ObserveQuery{Answer: &service.ObserveAnswer{
		ProposalID: string(q.ID), Response: string(observe.ResponseContradicted)}})

	after := restarted(t, store)
	said := only(t, knows(t, after, service.ObserveKnows{}))
	rt := &Runtime{observations: after}
	if _, err := rt.Observation(service.ObserveQuery{Knows: &service.ObserveKnows{
		Subject: said.Subject, Kind: string(said.Kind), Response: "maybe"}}); err == nil {
		t.Fatal("\"maybe\" was accepted as a correction")
	}
}

// knowledgeMustExist is the durable record, for the tests that check what survived.
func (g *observationRegistry) knowledgeMustExist(t *testing.T) []observe.RememberedSubject {
	t.Helper()
	store, ok := g.knowledgeStore()
	if !ok {
		t.Fatal("the registry holds no durable store")
	}
	return store.Subjects()
}

// Locatable means something RECOGNISED it, not merely that a record mentions it.
//
// Mutation Y6 — treating any proposal that names a durable subject as proof Marco can find it —
// survived until this existed. The difference matters: a recalled question is one Marco matched
// against what is on screen now; a proposal that merely carries an id is not.
func TestNamingASubjectIsNotTheSameAsRecognisingIt(t *testing.T) {
	g, store, q := reviseFixture(t)
	ask(t, g, service.ObserveQuery{Answer: &service.ObserveAnswer{
		ProposalID: string(q.ID), Response: string(observe.ResponseContradicted)}})

	after := restarted(t, store)
	subject := only(t, knows(t, after, service.ObserveKnows{})).Subject

	// A finished session that NAMES the subject without having recognised it.
	after.finished = append(after.finished, observesession.Result{
		Session: observe.Session{ID: "observe_x", State: observe.Completed,
			Application: "testgame"},
		Proposals: observe.ProposalLedger{Proposals: []observe.Proposal{{
			ID: "q_mentions", Kind: observe.PossibleSettingsLikeState,
			Recognised: false, RecognisedAs: subject,
		}}},
	})
	if said := only(t, knows(t, after, service.ObserveKnows{})); said.Locatable {
		t.Error("a subject that was named but never recognised is offered as something " +
			"Marco can point at")
	}

	// And when a session really did recognise it, it is.
	after.finished[len(after.finished)-1].Proposals.Proposals[0].Recognised = true
	if said := only(t, knows(t, after, service.ObserveKnows{})); !said.Locatable {
		t.Error("a subject a session recognised is not offered as locatable")
	}
}
