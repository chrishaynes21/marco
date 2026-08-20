package observesession_test

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// Asking whether Marco should learn a habit, and what each answer does.
//
// Entered through the real persisted relationship path: a store on disk holding a corroborated
// edge between two remembered subjects, a real runner, the real proposal ledger, the real
// `Respond` path. Nothing here constructs a proposal or reaches into the ledger to plant one.
//
// The invariant every test below defends:
//
//	yes buys a pending REQUEST, and nothing else. No procedure, no capability, no action,
//	no route, and no recorder.

// ── fixtures ──────────────────────────────────────────────────────────────────

// seedRelationship puts a corroborated edge into a store on disk.
//
// Through the production writes — `Remember` for the subjects, `RememberRelationships` for the
// edge, once per simulated session so `Sessions` counts real corroborations. Nothing is written
// by hand: an edge assembled by a test would not be one the policy has to accept.
func seedRelationship(t *testing.T, dir string, sessions int,
	ev observe.RelationshipEvidence) (string, string) {

	t.Helper()
	return seedRelationshipIn(t, memoryAt(t, dir), sessions, ev)
}

// seedRelationshipIn is seedRelationship against a store the caller already holds.
//
// Two Store handles over one file is not a configuration production ever has — a Director owns
// exactly one — and the last writer wins with its whole-file snapshot. A test that wants to add
// evidence WHILE a runner is holding the store has to share the handle, or it is testing the
// clobber rather than the thing it meant to.
func seedRelationshipIn(t *testing.T, store *semanticmemory.Store, sessions int,
	ev observe.RelationshipEvidence) (string, string) {

	t.Helper()
	for _, sig := range []observe.StructureSignature{aSignature(), bSignature()} {
		if err := store.Remember("testgame", sig, observe.SemanticKnowledge{
			Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
		}); err != nil {
			t.Fatalf("seeding a subject: %v", err)
		}
	}
	subjects := store.Subjects()
	var from, to string
	for _, s := range subjects {
		if len(s.Structure.Terms) > 0 && s.Structure.Terms[0] == observe.TermAudio {
			to = s.ID
		} else {
			from = s.ID
		}
	}
	if from == "" || to == "" {
		t.Fatalf("could not identify the two seeded subjects: %+v", subjects)
	}
	// One call per session, which is what the production path does — that is the ONLY way
	// Sessions becomes a count of independent corroborations rather than a second tally.
	for i := 0; i < sessions; i++ {
		if _, err := store.RememberRelationships("testgame",
			[]observe.RelationshipObservation{{From: from, To: to, Evidence: ev}}); err != nil {
			t.Fatalf("seeding the relationship: %v", err)
		}
	}
	return from, to
}

// strongEvidence is one session's worth of a habit Marco could plausibly learn.
func strongEvidence() observe.RelationshipEvidence {
	return observe.RelationshipEvidence{
		Observations: 2,
		Preceded:     map[observe.NavIntent]int{observe.NavConfirm: 2},
		Sequences: []observe.NavSequence{{
			Intents: []observe.NavIntent{observe.NavDown, observe.NavConfirm}, Count: 2,
		}},
	}
}

// runOver runs one ordinary session against a store and returns the runner and its result.
//
// The sampler is the static one: what is on screen is irrelevant to a learning proposal, which
// is judged entirely on what memory already holds. That is itself the point of Part 19 — the
// question does not require either endpoint to be visible.
func runOver(t *testing.T, store observe.Memory) (*observesession.Runner, observesession.Result) {
	t.Helper()
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, &staticSampler{},
		&recordingEvents{}).WithMemory(store)
	got, err := r.Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return r, got
}

// learningQuestion finds the open invitation, or reports that there is none.
func learningQuestion(r *observesession.Runner) (observe.Proposal, bool) {
	for _, p := range r.Proposals().Open() {
		if p.Ask == observe.AskLearnRelationship {
			return p, true
		}
	}
	return observe.Proposal{}, false
}

// refusalsFor collects the reasons one edge was not proposed.
func refusalsFor(res observesession.Result) map[observe.LearningRefusal]bool {
	out := map[observe.LearningRefusal]bool{}
	for _, a := range res.Learning {
		for _, r := range a.Refusals {
			out[r] = true
		}
	}
	return out
}

func relationshipIn(t *testing.T, dir, from, to string) observe.RememberedRelationship {
	t.Helper()
	for _, rel := range memoryAt(t, dir).Relationships() {
		if rel.From == from && rel.To == to {
			return rel
		}
	}
	t.Fatalf("no remembered relationship %s → %s", from, to)
	return observe.RememberedRelationship{}
}

// ── PART 22 A / PART 24: the production path ──────────────────────────────────

// THE production test. A corroborated habit earns an invitation, and a yes records a request.
//
// Deleting the ReviewRelationships call from the sampling loop must fail this.
func TestACorroboratedRelationshipEarnsALearningQuestion(t *testing.T) {
	dir := t.TempDir()
	from, to := seedRelationship(t, dir, 3, strongEvidence())

	runner, res := runOver(t, memoryAt(t, dir))
	p, ok := learningQuestion(runner)
	if !ok {
		t.Fatalf("no learning question was put about an edge seen %d time(s) across %d "+
			"session(s) with unambiguous navigation. Refusals: %v",
			relationshipIn(t, dir, from, to).Observations,
			relationshipIn(t, dir, from, to).Sessions, refusalsFor(res))
	}

	// It is about the DURABLE edge, not about a session-local state or the current screen.
	if p.Relationship == nil || p.Relationship.From != from || p.Relationship.To != to {
		t.Fatalf("the question refers to %+v, not to the remembered edge %s → %s",
			p.Relationship, from, to)
	}
	if p.Ask != observe.AskLearnRelationship {
		t.Errorf("ask kind = %q; a learning question typed as a semantic one would have its "+
			"answer interpreted as a correction", p.Ask)
	}
	// The wording invites, and never claims.
	lower := strings.ToLower(p.Question)
	if !strings.Contains(lower, "learn") {
		t.Errorf("the question does not offer to learn anything: %q", p.Question)
	}
	for _, claim := range []string{"i can ", "i know how", "shall i do", "i'll do"} {
		if strings.Contains(lower, claim) {
			t.Errorf("the question claims an ability Marco does not have (%q): %q",
				claim, p.Question)
		}
	}
	// And it does not put a subject id in front of a person.
	if strings.Contains(p.Question, from) || strings.Contains(p.Question, to) {
		t.Errorf("the question shows a remembered-subject id: %q", p.Question)
	}

	// ── yes, through the real response path ──
	if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}
	rel := relationshipIn(t, dir, from, to)
	if !rel.Learning.Pending() {
		t.Fatalf("a yes left the relationship's learning request as %+v; the user's "+
			"willingness was not recorded", rel.Learning)
	}
	if rel.Learning.Sessions == 0 || rel.Learning.Observations == 0 {
		t.Error("the request carries no snapshot of what was true when it was made")
	}
	// The evidence itself is untouched by an answer about learning.
	if rel.Observations != 6 || rel.Sessions != 3 {
		t.Errorf("answering changed the observation record: %d observations, %d sessions",
			rel.Observations, rel.Sessions)
	}
}

// A pending request stops the question coming back.
func TestAPendingLearningRequestSuppressesTheQuestion(t *testing.T) {
	dir := t.TempDir()
	from, to := seedRelationship(t, dir, 3, strongEvidence())

	runner, _ := runOver(t, memoryAt(t, dir))
	p, ok := learningQuestion(runner)
	if !ok {
		t.Fatal("no question to answer")
	}
	if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}

	// A later session over the same store must not ask again.
	later, res := runOver(t, memoryAt(t, dir))
	if _, ok := learningQuestion(later); ok {
		t.Error("Marco asked to learn something it has already been told to learn")
	}
	if !refusalsFor(res)[observe.RefusalLearningPending] {
		t.Errorf("the report does not explain the silence: %v", refusalsFor(res))
	}
	_ = from
	_ = to
}

// ── PART 22 B: sessions, not raw observations ─────────────────────────────────

// Ten observations in one sitting are not three sessions.
//
// The rule most likely to be quietly traded away for "but there is plenty of evidence". A habit
// seen ten times in one long fiddle with a menu has survived nothing; four transitions on three
// separate days have survived a restart, a new window generation, and whatever else changed.
func TestManyObservationsInOneSessionDoNotEarnAQuestion(t *testing.T) {
	dir := t.TempDir()
	seedRelationship(t, dir, 1, observe.RelationshipEvidence{
		Observations: 10,
		Preceded:     map[observe.NavIntent]int{observe.NavConfirm: 10},
		Sequences: []observe.NavSequence{{
			Intents: []observe.NavIntent{observe.NavConfirm}, Count: 10,
		}},
	})

	runner, res := runOver(t, memoryAt(t, dir))
	if p, ok := learningQuestion(runner); ok {
		t.Fatalf("ten observations in ONE session earned an invitation: %q", p.Question)
	}
	if !refusalsFor(res)[observe.RefusalInsufficientSessions] {
		t.Errorf("refused for %v, not for the session count", refusalsFor(res))
	}

	// And the contrast: fewer observations, across enough sessions, IS proposed.
	other := t.TempDir()
	seedRelationship(t, other, 3, observe.RelationshipEvidence{
		Observations: 2,
		Preceded:     map[observe.NavIntent]int{observe.NavConfirm: 2},
	})
	fewer, res2 := runOver(t, memoryAt(t, other))
	if _, ok := learningQuestion(fewer); !ok {
		t.Errorf("6 observations across 3 sessions did not earn an invitation that 10 in "+
			"one did not either; the policy rejects everything. Refusals: %v",
			refusalsFor(res2))
	}
}

// ── PART 22 C/E: navigation evidence must exist ───────────────────────────────

// A change nothing precedes is not something the user does.
func TestATransitionWithNoNavigationEvidenceIsNotProposed(t *testing.T) {
	dir := t.TempDir()
	seedRelationship(t, dir, 4, observe.RelationshipEvidence{
		Observations: 5, Unattributed: 5,
	})

	runner, res := runOver(t, memoryAt(t, dir))
	if p, ok := learningQuestion(runner); ok {
		t.Fatalf("Marco offered to learn how the user does something it never saw them do: "+
			"%q", p.Question)
	}
	got := refusalsFor(res)
	if !got[observe.RefusalNavigationTooWeak] || !got[observe.RefusalTooMuchUnattributed] {
		t.Errorf("refused for %v; both the missing navigation and the unattributed majority "+
			"should be named", got)
	}
}

// More observations do not rescue mostly-unattributed evidence.
//
// 20 observations with `confirm` before 3 is weaker than 6 with `confirm` before 6, and a policy
// that let raw volume dominate would get that backwards.
func TestVolumeDoesNotRescueMostlyUnattributedEvidence(t *testing.T) {
	dir := t.TempDir()
	seedRelationship(t, dir, 4, observe.RelationshipEvidence{
		Observations: 20,
		Preceded:     map[observe.NavIntent]int{observe.NavConfirm: 3},
		Unattributed: 17,
	})

	runner, res := runOver(t, memoryAt(t, dir))
	if p, ok := learningQuestion(runner); ok {
		t.Fatalf("80 observations, 68 of them with nothing before them, earned an "+
			"invitation: %q", p.Question)
	}
	if !refusalsFor(res)[observe.RefusalTooMuchUnattributed] {
		t.Errorf("refused for %v, not for the unattributed majority", refusalsFor(res))
	}
}

// ── PART 22 D: context-admitted evidence is not enough ────────────────────────

// Navigation that is only navigation in context does not earn an invitation.
//
// ADR-013's distinction does not stop at the durable boundary and does not stop here. W before a
// change while the screen looked like a set of choices is also "walk forwards", and a session of
// somebody walking around must not produce the same invitation as one of somebody using a menu.
func TestConditionalOnlyNavigationDoesNotEarnAnInvitation(t *testing.T) {
	dir := t.TempDir()
	seedRelationship(t, dir, 4, observe.RelationshipEvidence{
		Observations: 4,
		Preceded:     map[observe.NavIntent]int{observe.NavUp: 4},
		// Every attributed observation rested on context-admitted keys.
		ConditionalOnly: 4,
	})

	runner, res := runOver(t, memoryAt(t, dir))
	if p, ok := learningQuestion(runner); ok {
		t.Fatalf("an edge resting entirely on context-admitted keys earned an invitation: "+
			"%q", p.Question)
	}
	if !refusalsFor(res)[observe.RefusalConditionalOnly] {
		t.Errorf("refused for %v, not for the weaker evidence", refusalsFor(res))
	}
}

// ── PART 22 F: ordered-run consistency ────────────────────────────────────────

// A scatter of one-off runs is navigation ambiguity, not a habit.
func TestScatteredOrderedRunsBlockTheInvitation(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	// Four DIFFERENT runs, each seen once, and two further sessions that add observations
	// without repeating any of them. Seeding the same four every session would fold each to a
	// count of three, which is the opposite of the case under test.
	seedRelationshipIn(t, store, 1, observe.RelationshipEvidence{
		Observations: 4,
		Preceded:     map[observe.NavIntent]int{observe.NavConfirm: 4},
		Sequences: []observe.NavSequence{
			{Intents: []observe.NavIntent{observe.NavConfirm}, Count: 1},
			{Intents: []observe.NavIntent{observe.NavDown, observe.NavConfirm}, Count: 1},
			{Intents: []observe.NavIntent{observe.NavUp, observe.NavConfirm}, Count: 1},
			{Intents: []observe.NavIntent{observe.NavLeft, observe.NavConfirm}, Count: 1},
		},
	})
	seedRelationshipIn(t, store, 2, observe.RelationshipEvidence{
		Observations: 2,
		Preceded:     map[observe.NavIntent]int{observe.NavConfirm: 2},
	})

	runner, res := runOver(t, memoryAt(t, dir))
	if _, ok := learningQuestion(runner); ok {
		t.Fatal("four different one-off navigation runs before the same change earned an " +
			"invitation; there is no characteristic way the user does this")
	}
	if !refusalsFor(res)[observe.RefusalRunsInconsistent] {
		t.Errorf("refused for %v, not for the inconsistent runs", refusalsFor(res))
	}
}

// ── PART 22 H/I: identity ─────────────────────────────────────────────────────

// The question is asked once, however many sessions notice the same edge.
//
// Identity is the durable edge. If it were derived from anything session-local the user would be
// invited to learn the same habit every time they played, which is the nagging every part of
// this subsystem is arranged to prevent.
func TestTheSameEdgeIsNotProposedTwiceAcrossSessions(t *testing.T) {
	dir := t.TempDir()
	seedRelationship(t, dir, 3, strongEvidence())

	first, _ := runOver(t, memoryAt(t, dir))
	p1, ok := learningQuestion(first)
	if !ok {
		t.Fatal("no question in the first session")
	}
	if _, ok := first.Respond(p1.ID, observe.ResponseDeclined); !ok {
		t.Fatal("the decline was not recorded")
	}

	second, res := runOver(t, memoryAt(t, dir))
	if p2, ok := learningQuestion(second); ok {
		t.Errorf("a declined invitation came straight back in the next session: %q "+
			"(same id: %v)", p2.Question, p2.ID == p1.ID)
	}
	if !refusalsFor(res)[observe.RefusalAlreadyDeclined] {
		t.Errorf("refused for %v, not for the earlier decline", refusalsFor(res))
	}
}

// ── PART 22 J / PART 16: one question at a time, semantics first ──────────────

// A semantic question in flight defers the learning question.
//
// If Marco is still asking what a screen IS, it has no business asking whether to learn a route
// involving it. The user would be agreeing to a workflow before agreeing on the vocabulary.
func TestASemanticQuestionTakesPriorityOverAnInvitation(t *testing.T) {
	dir := t.TempDir()
	seedRelationship(t, dir, 3, strongEvidence())

	// The discovery sampler produces unanswered semantic questions; the store holds an
	// eligible edge at the same time.
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, settingsSession(),
		&recordingEvents{}).WithMemory(memoryAt(t, dir))
	res, err := r.Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	open := r.Proposals().Open()
	if len(open) != 1 {
		t.Fatalf("%d questions are open at once; the interruption budget is not being "+
			"shared between the two kinds", len(open))
	}
	if open[0].Ask == observe.AskLearnRelationship {
		t.Error("Marco asked whether to learn a route while it was still asking what the " +
			"screens are")
	}
	if !refusalsFor(res)[observe.RefusalAnotherQuestionOpen] {
		t.Errorf("the invitation was withheld and the report does not say why: %v",
			refusalsFor(res))
	}
}

// ── PART 23: response semantics ───────────────────────────────────────────────

// Refusing to learn is a preference, not a correction.
//
// THE test that keeps the two `no`s apart. A semantic `no` says Marco's interpretation is wrong
// and becomes a durable contradiction; a learning `no` says the user does not want this learned
// and must leave every observation exactly where it was.
func TestRefusingToLearnDoesNotContradictTheRelationship(t *testing.T) {
	dir := t.TempDir()
	from, to := seedRelationship(t, dir, 3, strongEvidence())
	before := relationshipIn(t, dir, from, to)

	runner, _ := runOver(t, memoryAt(t, dir))
	p, ok := learningQuestion(runner)
	if !ok {
		t.Fatal("no question to answer")
	}
	if _, ok := runner.Respond(p.ID, observe.ResponseContradicted); !ok {
		t.Fatal("the answer was not recorded")
	}

	after := relationshipIn(t, dir, from, to)
	if after.Learning == nil || after.Learning.Status != observe.LearningRefused {
		t.Fatalf("a no left the learning request as %+v", after.Learning)
	}
	if after.Learning.Pending() {
		t.Error("a refusal created a pending learning request")
	}
	// EVERY piece of evidence is untouched. The transition still happened.
	if after.Observations != before.Observations || after.Sessions != before.Sessions ||
		after.Unattributed != before.Unattributed ||
		len(after.Preceded) != len(before.Preceded) ||
		len(after.Sequences) != len(before.Sequences) {
		t.Errorf("refusing to LEARN altered the observation record:\nbefore %+v\nafter  %+v",
			before, after)
	}
	// And nothing semantic was contradicted about either endpoint.
	for _, id := range []string{from, to} {
		s, ok := memoryAt(t, dir).Subject(id)
		if !ok {
			t.Fatalf("endpoint %s vanished", id)
		}
		for _, k := range s.Knowledge {
			if k.Status == observe.KnowledgeContradicted {
				t.Errorf("declining to learn a route contradicted the endpoint's meaning: "+
					"%s is now %s", k.Kind, k.Status)
			}
		}
	}
}

// Not-now is not no.
//
// A decline suppresses the question and creates no request; a refusal is durable. Collapsing
// them would turn "I'm busy" into "never ask me that again".
func TestDecliningToLearnIsNotRefusing(t *testing.T) {
	dir := t.TempDir()
	from, to := seedRelationship(t, dir, 3, strongEvidence())

	runner, _ := runOver(t, memoryAt(t, dir))
	p, ok := learningQuestion(runner)
	if !ok {
		t.Fatal("no question to answer")
	}
	if _, ok := runner.Respond(p.ID, observe.ResponseDeclined); !ok {
		t.Fatal("the decline was not recorded")
	}

	rel := relationshipIn(t, dir, from, to)
	if rel.Learning == nil || rel.Learning.Status != observe.LearningDeclined {
		t.Fatalf("a decline was stored as %+v", rel.Learning)
	}
	if rel.Learning.Status == observe.LearningRefused {
		t.Fatal("a decline became a refusal; 'I'm busy' has become 'no'")
	}
	if rel.Learning.Pending() {
		t.Error("a decline created a pending learning request")
	}
}

// ── PART 15: a decline comes back only when the claim changes shape ───────────

// Growing counts do not reopen a declined invitation; a change of shape does.
func TestADeclinedInvitationReturnsOnlyWhenTheEvidenceChangesShape(t *testing.T) {
	dir := t.TempDir()
	from, to := seedRelationship(t, dir, 3, strongEvidence())

	runner, _ := runOver(t, memoryAt(t, dir))
	p, ok := learningQuestion(runner)
	if !ok {
		t.Fatal("no question to answer")
	}
	if _, ok := runner.Respond(p.ID, observe.ResponseDeclined); !ok {
		t.Fatal("the decline was not recorded")
	}

	// MORE OF THE SAME. The counts grow; the shape does not.
	store := memoryAt(t, dir)
	if _, err := store.RememberRelationships("testgame",
		[]observe.RelationshipObservation{{
			From: from, To: to, Evidence: strongEvidence(),
		}}); err != nil {
		t.Fatalf("corroborating: %v", err)
	}
	quiet, _ := runOver(t, memoryAt(t, dir))
	if _, ok := learningQuestion(quiet); ok {
		t.Error("a declined invitation came back because the counts grew; that is the " +
			"nagging a decline exists to prevent")
	}

	// A NEW KIND of navigation evidence is a different claim.
	if _, err := memoryAt(t, dir).RememberRelationships("testgame",
		[]observe.RelationshipObservation{{
			From: from, To: to,
			Evidence: observe.RelationshipEvidence{
				Observations: 2,
				Preceded:     map[observe.NavIntent]int{observe.NavPause: 2},
			},
		}}); err != nil {
		t.Fatalf("corroborating: %v", err)
	}
	changed, _ := runOver(t, memoryAt(t, dir))
	if _, ok := learningQuestion(changed); !ok {
		t.Error("the evidence gained a new kind of navigation and the question did not " +
			"come back; a decline has become permanent")
	}
}

// ── PART 8: endpoint quality ──────────────────────────────────────────────────

// An endpoint whose every interpretation the user rejected is not a basis for an invitation.
func TestAnEndpointWhoseMeaningWasRejectedBlocksTheInvitation(t *testing.T) {
	dir := t.TempDir()
	from, _ := seedRelationship(t, dir, 3, strongEvidence())

	// The user says the settings-like reading of the FROM screen is wrong, and there is no
	// other interpretation of it on record.
	if err := memoryAt(t, dir).Remember("testgame", aSignature(), observe.SemanticKnowledge{
		Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeContradicted,
	}); err != nil {
		t.Fatalf("recording the contradiction: %v", err)
	}
	if s, ok := memoryAt(t, dir).Subject(from); !ok || len(s.Knowledge) != 1 {
		t.Fatalf("the fixture did not leave exactly one rejected interpretation: %+v", s)
	}

	runner, res := runOver(t, memoryAt(t, dir))
	if p, ok := learningQuestion(runner); ok {
		t.Fatalf("Marco offered to learn a route through a screen whose meaning the user "+
			"has rejected: %q", p.Question)
	}
	if !refusalsFor(res)[observe.RefusalEndpointUnresolved] {
		t.Errorf("refused for %v, not for the endpoint", refusalsFor(res))
	}
}

// A screen with no readable name is still a fine endpoint.
//
// The question says "another screen" and is perfectly answerable — the user is looking at their
// own memory of playing, not at Marco's record. Requiring a name would make the invitation
// impossible in exactly the applications it is most useful for.
func TestAnUnnamedEndpointStillEarnsAnAnswerableQuestion(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	// The destination is remembered only as a MENU-LIKE screen, observed and never confirmed,
	// so nothing about it may be said out loud.
	if err := store.Remember("testgame", aSignature(), observe.SemanticKnowledge{
		Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
	}); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if err := store.Remember("testgame", bSignature(), observe.SemanticKnowledge{
		Kind: observe.PossibleMenuLikeState, Status: observe.KnowledgeObserved,
	}); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	var from, to string
	for _, s := range store.Subjects() {
		if len(s.Structure.Terms) > 0 && s.Structure.Terms[0] == observe.TermAudio {
			to = s.ID
		} else {
			from = s.ID
		}
	}
	for i := 0; i < 3; i++ {
		if _, err := store.RememberRelationships("testgame",
			[]observe.RelationshipObservation{{
				From: from, To: to, Evidence: strongEvidence(),
			}}); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}

	runner, res := runOver(t, memoryAt(t, dir))
	p, ok := learningQuestion(runner)
	if !ok {
		t.Fatalf("an unnamed destination blocked the invitation entirely: %v", refusalsFor(res))
	}
	if strings.Contains(p.Question, to) || strings.Contains(p.Question, "menu") {
		t.Errorf("the question named a screen on observed-only evidence: %q", p.Question)
	}
	if !strings.Contains(strings.ToLower(p.Question), "another screen") {
		t.Errorf("the question does not read naturally with one unnamed end: %q", p.Question)
	}
}

// ── PART 24: no capability exists ─────────────────────────────────────────────

// A yes creates a request and nothing that can be run.
//
// The authority boundary, asserted on the SHAPE of what a yes produces rather than on the
// absence of a call somebody might add later. A learning request lives on a relationship record
// in semantic memory; there is no registry here it could be promoted into.
func TestAYesCreatesNoCapabilityProcedureOrAction(t *testing.T) {
	dir := t.TempDir()
	from, to := seedRelationship(t, dir, 3, strongEvidence())

	runner, _ := runOver(t, memoryAt(t, dir))
	p, ok := learningQuestion(runner)
	if !ok {
		t.Fatal("no question to answer")
	}
	if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}

	rel := relationshipIn(t, dir, from, to)
	if !rel.Learning.Pending() {
		t.Fatalf("the request is %+v", rel.Learning)
	}
	// What a pending request may hold: a status, a digest, and two counts. Nothing that
	// could be executed, and nothing that could name a key.
	req := *rel.Learning
	if req.Evidence == "" {
		t.Error("the request records no evidence digest, so nothing can tell later whether " +
			"the claim has changed since the user agreed")
	}
	// The store is the whole durable surface. Nothing else was created.
	store := memoryAt(t, dir)
	if n := len(store.Relationships()); n != 1 {
		t.Errorf("%d relationships exist after one yes", n)
	}
	if n := len(store.Subjects()); n != 2 {
		t.Errorf("%d subjects exist after one yes", n)
	}
	_ = to
}

// ── PART 21: silence is explained ─────────────────────────────────────────────

// Every remembered edge is judged and reported, whether or not it earns a question.
func TestEveryRememberedEdgeIsJudgedAndTheReasonsAreClosed(t *testing.T) {
	dir := t.TempDir()
	seedRelationship(t, dir, 1, observe.RelationshipEvidence{
		Observations: 1, Unattributed: 1,
	})

	_, res := runOver(t, memoryAt(t, dir))
	if len(res.Learning) != 1 {
		t.Fatalf("%d edge(s) judged, want 1", len(res.Learning))
	}
	a := res.Learning[0]
	if a.Eligible {
		t.Fatal("a single unattributed observation was judged eligible")
	}
	if len(a.Refusals) == 0 {
		t.Fatal("an edge was refused with no reason; 'Marco did not ask' is undebuggable")
	}
	known := map[observe.LearningRefusal]bool{
		observe.RefusalInsufficientSessions: true, observe.RefusalInsufficientObservations: true,
		observe.RefusalNavigationTooWeak: true, observe.RefusalTooMuchUnattributed: true,
		observe.RefusalConditionalOnly: true, observe.RefusalRunsInconsistent: true,
		observe.RefusalEndpointUnresolved: true, observe.RefusalAlreadyDeclined: true,
		observe.RefusalAlreadyRefused: true, observe.RefusalLearningPending: true,
		observe.RefusalAnotherQuestionOpen: true, observe.RefusalAlreadyAsked: true,
	}
	for _, r := range a.Refusals {
		if !known[r] {
			t.Errorf("refusal %q is not in the closed vocabulary", r)
		}
	}
	// And it can explain itself to a person, without claiming authority.
	lines := observe.DescribeLearningAssessment(a,
		observe.RememberedSubject{ID: a.Relationship.From},
		observe.RememberedSubject{ID: a.Relationship.To})
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "not asked:") {
		t.Errorf("the explanation does not say it was not asked:\n%s", joined)
	}
	if !strings.Contains(joined, "authority: none") {
		t.Errorf("the explanation does not disclaim authority:\n%s", joined)
	}
}

// ── PARTS 18/19: a delayed answer binds to the edge it was asked about ────────

// The answer follows the question, not whatever is best now.
//
// The user answers after leaving both screens, and after the store has grown a better-corroborated
// edge. The answer must still land on the relationship the question named.
func TestADelayedAnswerBindsToTheProposedRelationship(t *testing.T) {
	dir := t.TempDir()
	// ONE store handle throughout, as a Director has. Two handles over one file would let the
	// last whole-file write clobber the other, and this test would be measuring that.
	store := memoryAt(t, dir)
	from, to := seedRelationshipIn(t, store, 3, strongEvidence())

	runner, _ := runOver(t, store)
	p, ok := learningQuestion(runner)
	if !ok {
		t.Fatal("no question to answer")
	}

	// A SECOND, better-corroborated edge appears while the question is open.
	if err := store.Remember("testgame", cSignature(), observe.SemanticKnowledge{
		Kind: observe.PossibleMenuLikeState, Status: observe.KnowledgeConfirmed,
	}); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	var third string
	for _, s := range store.Subjects() {
		if len(s.Structure.Terms) > 0 && s.Structure.Terms[0] == observe.TermInvite {
			third = s.ID
		}
	}
	for i := 0; i < 9; i++ {
		if _, err := store.RememberRelationships("testgame",
			[]observe.RelationshipObservation{{
				From: from, To: third, Evidence: strongEvidence(),
			}}); err != nil {
			t.Fatalf("seeding the rival edge: %v", err)
		}
	}

	if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}
	if got := relationshipOf(t, store, from, to); !got.Learning.Pending() {
		t.Errorf("the answer did not land on the edge that was asked about: %+v", got.Learning)
	}
	if rival := relationshipOf(t, store, from, third); rival.Learning != nil {
		t.Errorf("the answer landed on the better-corroborated edge instead: %+v",
			rival.Learning)
	}
}

// ── the store refuses a request it cannot attach ──────────────────────────────

func TestALearningRequestForAnUnknownRelationshipIsRefused(t *testing.T) {
	store, why := semanticmemory.Open("")
	if why != "" {
		t.Fatalf("Open: %s", why)
	}
	err := store.RememberLearning("testgame",
		observe.RelationshipRef{From: "subj_a", To: "subj_b"},
		observe.LearningRequest{Status: observe.LearningPending})
	if err == nil {
		t.Fatal("a learning request was attached to a relationship the store does not hold")
	}
}

// relationshipOf reads one edge from a store the caller holds.
func relationshipOf(t *testing.T, store *semanticmemory.Store,
	from, to string) observe.RememberedRelationship {

	t.Helper()
	for _, rel := range store.Relationships() {
		if rel.From == from && rel.To == to {
			return rel
		}
	}
	t.Fatalf("no remembered relationship %s → %s", from, to)
	return observe.RememberedRelationship{}
}
