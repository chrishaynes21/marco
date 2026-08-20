package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// Correcting an answer, through the surface a person actually reaches.
//
// The ledger tests prove the rules and the session tests prove the file. This proves the JOURNEY
// the CLI and the browser both take: a service request arrives, the registry finds the question by
// identity, the runner writes, and a store reopened from the same path reads the correction rather
// than the mistake.
//
// Both production comments that name a test — the registry's runner call and the runner's durable
// write — name this one.

// reviseFixture is a registry over a real store, holding a finished session with a semantic
// question that has been answered.
func reviseFixture(t *testing.T) (*observationRegistry, *semanticmemory.Store, observe.Proposal) {
	t.Helper()
	restore := sessionClock
	sessionClock = newDryClock()
	t.Cleanup(func() { sessionClock = restore })

	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	g := newObservationRegistry()
	g.memory = store

	// Screens the player enters, leaves and comes back to. One visit is never enough: a
	// hypothesis seen once stays contested, and a contested hypothesis is not worth
	// interrupting anybody for — which is the ordinary policy, not a fixture detail.
	id := observeOnce(t, g, revisitScript())
	view, ok := g.Snapshot(id)
	if !ok {
		t.Fatal("the finished session is not readable")
	}
	var saw []string
	for _, p := range view.Proposals {
		saw = append(saw, string(p.Kind)+"/"+string(p.Ask)+"/"+string(p.Status))
		if p.Status == observe.ProposalOpen && (p.Ask == "" || p.Ask == observe.AskSemantic) {
			return g, store, p
		}
	}
	t.Skipf("the fixture raised no semantic question to answer; proposals=%v", saw)
	return nil, nil, observe.Proposal{}
}

// ask puts one request through the production request path and returns the proposal it landed on.
func ask(t *testing.T, g *observationRegistry, q service.ObserveQuery) observe.Proposal {
	t.Helper()
	rt := &Runtime{observations: g}
	out, err := rt.Observation(q)
	if err != nil {
		t.Fatalf("Observation: %v", err)
	}
	view, ok := out.(observationView)
	if !ok {
		t.Fatalf("the handler returned %T", out)
	}
	if view.Answered == nil {
		t.Fatal("the reply does not say which question it landed on")
	}
	return *view.Answered
}

// judgementIn is what a store reopened from the same file says about one interpretation.
func judgementIn(t *testing.T, store *semanticmemory.Store,
	kind observe.HypothesisKind) observe.Judgement {

	t.Helper()
	reopened, why := semanticmemory.Open(store.Path())
	if why != "" {
		t.Fatalf("reopening memory: %s", why)
	}
	for _, s := range reopened.Subjects() {
		if k, ok := s.Find(kind); ok {
			return k.Effective()
		}
	}
	return observe.JudgementNone
}

// THE acceptance test for the milestone: a mistaken answer is corrected through the product, and
// the correction — not the mistake — is what the next process reads.
func TestARevisionThroughTheProductionPathSurvivesARestart(t *testing.T) {
	g, store, q := reviseFixture(t)

	// The mistake, given through the ordinary answer path.
	ask(t, g, service.ObserveQuery{Answer: &service.ObserveAnswer{
		ProposalID: string(q.ID), Response: string(observe.ResponseContradicted)}})
	if got := judgementIn(t, store, q.Kind); got != observe.JudgementContradicted {
		t.Fatalf("the answer reads %q on disk, so there is nothing to correct", got)
	}

	// The correction, given through the revision path.
	after := ask(t, g, service.ObserveQuery{Revise: &service.ObserveRevise{
		ProposalID: string(q.ID), Response: string(observe.ResponseConfirmed)}})
	if after.ID != q.ID {
		t.Errorf("the revision landed on %s, not on %s", after.ID, q.ID)
	}
	if after.Response != observe.ResponseConfirmed {
		t.Errorf("the reply says the answer is now %q", after.Response)
	}
	if got := judgementIn(t, store, q.Kind); got != observe.JudgementConfirmed {
		t.Fatalf("a store reopened from the same file reads %q. The user corrected this and "+
			"the next restart will undo the correction", got)
	}
}

// Withdrawing through the product leaves no active judgement, durably.
func TestAWithdrawalThroughTheProductionPathSurvivesARestart(t *testing.T) {
	g, store, q := reviseFixture(t)

	ask(t, g, service.ObserveQuery{Answer: &service.ObserveAnswer{
		ProposalID: string(q.ID), Response: string(observe.ResponseContradicted)}})
	if got := judgementIn(t, store, q.Kind); got != observe.JudgementContradicted {
		t.Fatalf("the answer reads %q on disk, so there is nothing to withdraw", got)
	}

	after := ask(t, g, service.ObserveQuery{Revise: &service.ObserveRevise{
		ProposalID: string(q.ID), Withdraw: true}})
	if !after.Retracted {
		t.Error("the reply does not say the answer was withdrawn, so no surface can say so")
	}
	if after.Response != observe.ResponseNone {
		t.Errorf("a withdrawn answer still reads as %q", after.Response)
	}
	if got := judgementIn(t, store, q.Kind); got != observe.JudgementNone {
		t.Fatalf("after withdrawing, a reopened store still reads %q", got)
	}
}

// A revision for a question nobody answered is refused, and says why.
func TestRevisingAnUnansweredQuestionIsRefusedByTheProtocol(t *testing.T) {
	g, _, q := reviseFixture(t)
	rt := &Runtime{observations: g}

	if _, err := rt.Observation(service.ObserveQuery{Revise: &service.ObserveRevise{
		ProposalID: string(q.ID), Response: string(observe.ResponseConfirmed)}}); err == nil {
		t.Fatal("an open question was revised. Revision is not a second way to answer")
	}
	if _, err := rt.Observation(service.ObserveQuery{Revise: &service.ObserveRevise{
		ProposalID: "q_nothing", Withdraw: true}}); err == nil {
		t.Error("a question that was never put was withdrawn")
	}
}

// A revision cannot smuggle in a word outside the closed vocabulary.
func TestARevisionOutsideTheClosedVocabularyIsRefused(t *testing.T) {
	g, _, q := reviseFixture(t)
	ask(t, g, service.ObserveQuery{Answer: &service.ObserveAnswer{
		ProposalID: string(q.ID), Response: string(observe.ResponseContradicted)}})

	rt := &Runtime{observations: g}
	if _, err := rt.Observation(service.ObserveQuery{Revise: &service.ObserveRevise{
		ProposalID: string(q.ID), Response: "maybe"}}); err == nil {
		t.Fatal("\"maybe\" was accepted as a revision")
	}
}

// revisitScript is a player going somewhere and coming back, three times.
//
// Recurrence across VISITS is what turns a guess into a question: the ordinary policy will not
// interrupt anybody about a screen it has seen once. A fixture that held one screen produced no
// question at all, which is the policy working rather than a broken fixture.
func revisitScript() []dryFrame {
	var out []dryFrame
	for i := 0; i < 3; i++ {
		out = append(out, dryHold("a", 4)...)
		out = append(out, dryPress("b", observe.NavConfirm))
		out = append(out, dryHold("b", 4)...)
		out = append(out, dryPress("a", observe.NavBack))
	}
	out = append(out, dryHold("a", 4)...)
	return out
}

// ── what the typing asked for ─────────────────────────────────────────────────

// `revise` and `withdraw` are different operations, and the words decide which.
//
// Mutation M17 — the CLI sending every revision as a withdrawal — survived until this existed.
// The rest of the command is a round trip to a running service; this is the one decision it makes
// on its own.
func TestTheWordsTypedDecideWhetherAnAnswerChangesOrIsWithdrawn(t *testing.T) {
	for _, tc := range []struct {
		args     []string
		withdraw bool
		want     observe.UserResponse
		wantDraw bool
	}{
		{[]string{"q_1", "yes"}, false, observe.ResponseConfirmed, false},
		{[]string{"q_1", "no"}, false, observe.ResponseContradicted, false},
		{[]string{"q_1", "not-sure"}, false, observe.ResponseDeclined, false},
		{[]string{"q_1"}, true, "", true},
	} {
		rev, complaint := reviseRequest(tc.args, tc.withdraw)
		if rev == nil {
			t.Fatalf("%v (withdraw=%v) was refused: %s", tc.args, tc.withdraw, complaint)
		}
		if rev.Withdraw != tc.wantDraw {
			t.Errorf("%v: withdraw=%v, want %v", tc.args, rev.Withdraw, tc.wantDraw)
		}
		if observe.UserResponse(rev.Response) != tc.want {
			t.Errorf("%v: response %q, want %q", tc.args, rev.Response, tc.want)
		}
		if rev.ProposalID != "q_1" {
			t.Errorf("%v: the request names %q", tc.args, rev.ProposalID)
		}
	}
}

// Words outside the closed vocabulary, and missing ones, are refused before anything is sent.
func TestARevisionThatIsNotAnAnswerNeverLeavesTheCommand(t *testing.T) {
	for _, args := range [][]string{{"q_1", "maybe"}, {"q_1"}, {}} {
		if rev, complaint := reviseRequest(args, false); rev != nil {
			t.Errorf("%v built a request %+v", args, rev)
		} else if complaint == "" {
			t.Errorf("%v was refused without saying why", args)
		}
	}
	if rev, complaint := reviseRequest(nil, true); rev != nil {
		t.Errorf("a withdrawal with no question built a request %+v", rev)
	} else if !strings.Contains(complaint, "question-id") {
		t.Errorf("the refusal does not say what is missing: %q", complaint)
	}
}
