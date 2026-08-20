package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// "I know enough to write this play, but I do not know what you call the screen where it starts."
//
// The whole milestone in one sentence, and every test here is about the sentence actually being
// said, the answer actually arriving, and the answer landing on the RIGHT screen — which is
// almost never the one in front of the user by the time they type it.
//
// Nothing here calls NameSubject, constructs a Proposal, or invokes the naming helper directly.
// Everything enters through `Runtime.LearnedPlay` and `Runtime.Observation`, which are the two
// doors the product has.

// unnamedRegistry is the fixture the milestone exists for: verified, and with no name for its
// starting screen.
//
// Identical to verifiedRegistry except for the one thing under test. If the naming step were
// quietly unnecessary, this would lower anyway and half these tests would pass vacuously — so
// TestAVerifiedPlayThatCannotSayWhereItStartsAsks checks it does not.
func unnamedRegistry(t *testing.T) (*observationRegistry, string) {
	t.Helper()
	g := authorizedRegistry(t)
	grant := g.last.Grant()
	if grant == nil {
		t.Fatal("the fixture holds no authorization")
	}
	j, ok := g.judgeNow("testgame", grant.Relationship)
	if !ok {
		t.Fatal("no judgement for the authorized route")
	}
	g.rememberRehearsal("testgame", j, rehearse.RehearsalResult{
		Relationship: grant.Relationship, Source: grant.Source,
		Destination: grant.Destination, Evidence: j.Digest, Live: true,
		Terminal: rehearse.CompletedRoute, StepsTaken: 1, Inputs: 1,
		Steps: []rehearse.StepRecord{{Outcome: rehearse.DirectlyVerified}},
	})
	return g, grant.Source
}

// otherApp is a window belonging to a DIFFERENT program.
//
// A separate target rather than a selector argument, because the selector is what is asked for
// and the Ref is what was found — and it is the Ref that decides which application a session is
// about. A fixture that only changed the request would report `testgame` anyway and every
// cross-application claim built on it would be vacuous. It was: the first version of this file
// switched the selector, and the mutation that binds an answer to the application in front
// survived, because nothing had actually switched.
type otherApp struct{ name string }

func (o otherApp) Acquire(context.Context, windowref.Selector) (windowref.Ref, error) {
	return windowref.Ref{
		ID: "hwnd:200", Handle: 200, ProcessID: 9, Application: o.name, Generation: 1,
	}, nil
}

// observeOnceIn runs one session against a named application, so "what is in front" can move.
func observeOnceIn(t *testing.T, g *observationRegistry, application string,
	script []dryFrame) observe.SessionID {

	t.Helper()
	id, err := g.Start(otherApp{name: application}, &drySampler{script: script},
		observesession.NopEvents{},
		windowref.Selector{Application: application}, dryBounds())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for g.ActiveID() != "" {
		if time.Now().After(deadline) {
			t.Fatal("the session never finished")
		}
		time.Sleep(time.Millisecond)
	}
	return id
}

// lower runs the production lowering read, which is also the naming trigger.
func lower(t *testing.T, g *observationRegistry) service.LearnedView {
	t.Helper()
	rt := &Runtime{observations: g}
	out, err := rt.LearnedPlay(service.LearnedQuery{Application: "testgame"})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	return out
}

// namingQuestion finds the open naming question in what a reader would see.
//
// Across every session record, because the question outlives the session that raised it — which
// is the whole claim under test.
func namingQuestion(t *testing.T, g *observationRegistry) (observe.Proposal, bool) {
	t.Helper()
	g.mu.RLock()
	defer g.mu.RUnlock()
	for i := len(g.finished) - 1; i >= 0; i-- {
		for _, p := range g.finished[i].Proposals.Proposals {
			if p.Ask == observe.AskNameScreen && p.Status == observe.ProposalOpen {
				return p, true
			}
		}
	}
	return observe.Proposal{}, false
}

// answerName puts a name in through the real user-facing request path.
func answerName(t *testing.T, g *observationRegistry, id observe.ProposalID, raw string) error {
	t.Helper()
	rt := &Runtime{observations: g}
	_, err := rt.Observation(service.ObserveQuery{
		Name: &service.ObserveScreenName{ProposalID: string(id), Name: raw},
	})
	return err
}

// nameTheDestination answers whatever naming question is outstanding next, through the real path.
//
// A learned play needs TWO names — where it starts and where it expects to finish — and Marco
// asks for one at a time. Recomputing the lowering is what surfaces the second; nothing here
// queues anything.
func nameTheDestination(t *testing.T, g *observationRegistry) {
	t.Helper()
	lower(t, g)
	q, ok := namingQuestion(t, g)
	if !ok {
		t.Fatal("the destination was never asked about")
	}
	if err := answerName(t, g, q.ID, "the audio page"); err != nil {
		t.Fatalf("naming the destination: %v", err)
	}
}

// ── the demand ────────────────────────────────────────────────────────────────

// A verified play that cannot say where it starts makes Marco ask what the screen is called.
//
// THE wiring test. Deleting the trigger in LearnedPlay must fail this — no unit test of
// ReviewScreenName can, because the production path is the thing that was missing.
func TestAVerifiedPlayThatCannotSayWhereItStartsAsks(t *testing.T) {
	g, source := unnamedRegistry(t)

	// Before anything asks, the play genuinely cannot be written down, and says why.
	v := lower(t, g)
	var sawUnnamed bool
	for _, p := range v.Plays {
		if p.Eligible {
			t.Fatalf("an unnamed starting screen was written down anyway:\n%s", p.Source)
		}
		for _, r := range p.Refusals {
			if r == string(observe.RefusalScreenUnnamed) {
				sawUnnamed = true
			}
		}
	}
	if !sawUnnamed {
		t.Fatal("the lowering did not refuse for want of a screen name, so this fixture " +
			"proves nothing")
	}

	q, ok := namingQuestion(t, g)
	if !ok {
		t.Fatal("Marco knew it needed a name and never asked for one")
	}
	// It is about the screen the ROUTE starts on, by durable identity.
	if q.Screen == nil {
		t.Fatal("the question carries no screen; an answer could only land on a guess")
	}
	if q.Screen.ID != source {
		t.Errorf("the question is about %q, not the screen the route starts on (%q)",
			q.Screen.ID, source)
	}
	if q.Screen.Application != "testgame" {
		t.Errorf("the question is scoped to %q", q.Screen.Application)
	}
	// And it reads like a question, not like a record.
	for _, leak := range []string{"subj_", "state_", q.Screen.ID, "screen_unnamed", "digest"} {
		if leak != "" && strings.Contains(q.Question, leak) {
			t.Errorf("the question shows %q to the user: %s", leak, q.Question)
		}
	}
	if !strings.Contains(strings.ToLower(q.Question), "what do you call") {
		t.Errorf("the question does not ask what the screen is called: %s", q.Question)
	}
}

// Asking again and again is not asking again.
func TestTheNamingQuestionIsAskedOnce(t *testing.T) {
	g, _ := unnamedRegistry(t)
	for i := 0; i < 4; i++ {
		lower(t, g)
	}
	view, _ := g.Snapshot("")
	n := 0
	for _, p := range view.Proposals {
		if p.Ask == observe.AskNameScreen {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("four lowering attempts produced %d naming questions", n)
	}
}

// A later session does not re-ask a question that is already outstanding.
//
// The case the in-ledger check cannot cover. Every new session gets its own record, so a
// suppression that only looked at the record the question would be filed under would ask again
// every time the user played — the exact nagging every other question kind is arranged to avoid.
func TestPlayingAgainDoesNotReAskForTheSameScreen(t *testing.T) {
	g, _ := unnamedRegistry(t)
	lower(t, g)
	first, ok := namingQuestion(t, g)
	if !ok {
		t.Fatal("nothing asked")
	}

	// The user plays twice more and reads the report afterwards, as people do.
	observeOnce(t, g, dryHold("a", 6))
	lower(t, g)
	observeOnce(t, g, dryHold("b", 6))
	lower(t, g)

	g.mu.RLock()
	defer g.mu.RUnlock()
	var found []observe.ProposalID
	for i := range g.finished {
		for _, p := range g.finished[i].Proposals.Proposals {
			if p.Ask == observe.AskNameScreen {
				found = append(found, p.ID)
			}
		}
	}
	if len(found) != 1 {
		t.Fatalf("three lowering attempts across three sessions produced %d naming "+
			"questions (%v)", len(found), found)
	}
	if found[0] != first.ID {
		t.Errorf("the question changed identity to %s; it is about the same screen", found[0])
	}
}

// A screen that already has a name is not asked about.
func TestANamedScreenIsNotAskedAbout(t *testing.T) {
	g := verifiedRegistry(t) // names the source screen
	lower(t, g)
	if q, ok := namingQuestion(t, g); ok {
		t.Fatalf("Marco asked for a name it already had: %s", q.Question)
	}
}

// ── PART 4: the delayed answer ────────────────────────────────────────────────

// The answer lands on the screen the question was about, not on whatever is current.
//
// THE load-bearing test. Between the question and the answer the user goes somewhere else — which
// is the ordinary case, because the question appears in a report they read afterwards.
func TestNamingAScreenBindsToTheSubjectThatWasAskedAbout(t *testing.T) {
	g, source := unnamedRegistry(t)
	lower(t, g)
	q, ok := namingQuestion(t, g)
	if !ok {
		t.Fatal("nothing asked")
	}

	store := g.memory.(*semanticmemory.Store)
	// The user moves on. A later session leaves a DIFFERENT screen as the most recent thing
	// this Director saw — everything ambient now points somewhere else.
	other := otherSubject(t, store, source)
	observeOnce(t, g, dryHold("b", 6))

	if err := answerName(t, g, q.ID, "the pause menu"); err != nil {
		t.Fatalf("answering: %v", err)
	}

	// The screen that was asked about is named.
	got, ok := store.SubjectNamed("testgame", "the pause menu")
	if !ok {
		t.Fatal("no screen is called the pause menu")
	}
	if got.ID != source {
		t.Fatalf("the name landed on %q, not on the screen the question was about (%q)",
			got.ID, source)
	}
	// And the one the user happened to be looking at is untouched.
	for _, s := range store.Subjects() {
		if s.ID == other && s.Called != "" {
			t.Fatalf("the screen the user was on when they answered was named %q", s.Called)
		}
	}

	// And the judgement is recomputed from memory, not patched: the entry condition is now
	// the user's own words. (The play still cannot be written down — it needs a name for the
	// screen it finishes on too, which is TestTheNamingLifecycleUnblocksTheLearnedPlay.)
	// The question that was answered stops being asked; the one that is now outstanding, if
	// any, is about a DIFFERENT screen.
	if q2, still := namingQuestion(t, g); still && q2.ID == q.ID {
		t.Error("the naming question is still open after it was answered")
	}
}

// otherSubject is any remembered subject that is not the one given.
func otherSubject(t *testing.T, store *semanticmemory.Store, not string) string {
	t.Helper()
	for _, s := range store.Subjects() {
		if s.ID != not {
			return s.ID
		}
	}
	t.Fatal("the fixture holds only one subject, so this proves nothing")
	return ""
}

// ── PART 5: across an application change ──────────────────────────────────────

// Switching application does not redirect the answer.
//
// The strongest form of the same claim. A name is scoped to one program, so an answer that
// wandered into the application in front at answer time would put the user's word in a namespace
// where they never meant it — and `Screen's Showing` would then refuse forever in the one place
// it was supposed to work.
func TestAnAnswerDoesNotFollowTheApplicationInFront(t *testing.T) {
	g, source := unnamedRegistry(t)
	lower(t, g)
	q, _ := namingQuestion(t, g)

	// Another program entirely is now what this Director has most recently seen.
	observeOnceIn(t, g, "someothergame", dryHold("b", 6))

	if err := answerName(t, g, q.ID, "the pause menu"); err != nil {
		t.Fatalf("answering: %v", err)
	}

	store := g.memory.(*semanticmemory.Store)
	got, ok := store.SubjectNamed("testgame", "the pause menu")
	if !ok || got.ID != source {
		t.Fatalf("the name did not reach the original application's screen (got %+v)", got)
	}
	if _, leaked := store.SubjectNamed("someothergame", "the pause menu"); leaked {
		t.Fatal("the other application gained the name; ambient application redirected the " +
			"answer, which is exactly what durable identity exists to prevent")
	}
}

// ── PART 8: an answer that is not a name ──────────────────────────────────────

// An invalid answer changes nothing and leaves the question open.
//
// Closing a question the user did not successfully answer would remove the only prompt that lets
// them try again.
func TestAnInvalidNameChangesNothingAndTheQuestionStaysOpen(t *testing.T) {
	for _, tc := range []struct{ name, in string }{
		{"nothing at all", ""},
		{"only spaces", "   "},
		{"longer than a name", strings.Repeat("x", observe.MaxScreenNameLength+1)},
		{"a newline", "the pause\nmenu"},
		{"a control character", "the pause\x00menu"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, source := unnamedRegistry(t)
			lower(t, g)
			q, _ := namingQuestion(t, g)

			if err := answerName(t, g, q.ID, tc.in); err == nil {
				t.Fatalf("%q was accepted as a screen name", tc.in)
			}

			store := g.memory.(*semanticmemory.Store)
			for _, s := range store.Subjects() {
				if s.Called != "" {
					t.Fatalf("a screen was named %q by a refused answer", s.Called)
				}
			}
			if _, still := namingQuestion(t, g); !still {
				t.Fatal("the question was closed by an answer that did not work")
			}
			// And the artifact is still blocked, for the same stated reason.
			for _, p := range lower(t, g).Plays {
				if p.Eligible {
					t.Fatal("the play became writable after a refused answer")
				}
			}
			_ = source
		})
	}
}

// ── PART 9: a name somebody else already has ──────────────────────────────────

// Two screens in one application may not share a name, through the real path.
func TestADuplicateNameIsRefusedThroughTheAnswerPath(t *testing.T) {
	g, source := unnamedRegistry(t)
	store := g.memory.(*semanticmemory.Store)

	// Another screen in the same application already has the name.
	other := otherSubject(t, store, source)
	taken, err := observe.UserSuppliedScreenName("the pause menu")
	if err != nil {
		t.Fatalf("naming: %v", err)
	}
	if err := store.NameSubject("testgame", other, taken); err != nil {
		t.Fatalf("naming the first screen: %v", err)
	}

	lower(t, g)
	q, ok := namingQuestion(t, g)
	if !ok {
		t.Fatal("nothing asked")
	}
	if err := answerName(t, g, q.ID, "the pause menu"); err == nil {
		t.Fatal("two screens in one application took the same name; a play's first line " +
			"would be ambiguous")
	}

	// The source stays unnamed, the existing name is untouched, and the question stays open.
	for _, s := range store.Subjects() {
		if s.ID == source && s.Called != "" {
			t.Errorf("the second screen was named %q anyway", s.Called)
		}
		if s.ID == other && s.Called != "the pause menu" {
			t.Errorf("the screen that already had the name now says %q", s.Called)
		}
	}
	if _, still := namingQuestion(t, g); !still {
		t.Error("the question closed on a name that was refused")
	}

	// A different application may still use the word — the scope is the point.
	if got, ok := store.SubjectNamed("testgame", "the pause menu"); !ok || got.ID != other {
		t.Errorf("the original name stopped resolving (%+v)", got)
	}
}

// ── PART 6: the generic vocabulary stays closed ───────────────────────────────

// A naming question cannot be answered yes, no, or not-now.
func TestANamingQuestionIsNotAnsweredWithYesOrNo(t *testing.T) {
	g, _ := unnamedRegistry(t)
	lower(t, g)
	q, _ := namingQuestion(t, g)

	rt := &Runtime{observations: g}
	for _, resp := range []observe.UserResponse{
		observe.ResponseConfirmed, observe.ResponseContradicted, observe.ResponseDeclined,
	} {
		if _, err := rt.Observation(service.ObserveQuery{
			Answer: &service.ObserveAnswer{ProposalID: string(q.ID), Response: string(resp)},
		}); err == nil {
			t.Errorf("%q was accepted as an answer to \"what do you call this screen?\"", resp)
		}
	}
	if _, still := namingQuestion(t, g); !still {
		t.Fatal("a yes/no answer settled the naming question")
	}
}

// ── PART 10: the user can actually see it ─────────────────────────────────────

// The question reaches the report a person reads, and says how to answer it.
func TestTheNamingQuestionIsPutToTheReaderWithAWayToAnswer(t *testing.T) {
	g, _ := unnamedRegistry(t)
	lower(t, g)
	q, ok := namingQuestion(t, g)
	if !ok {
		t.Fatal("nothing asked")
	}

	view, _ := g.Snapshot("")
	out := renderProposalsFor(view.ID, view.Proposals)

	if !strings.Contains(out, "MARCO HAS A QUESTION") {
		t.Fatal("the report does not put the question to the reader at all")
	}
	if !strings.Contains(out, q.Question) {
		t.Error("the question sentence is missing from the report")
	}
	if !strings.Contains(out, "director name-screen") {
		t.Error("the report does not say how to give a name; a question nobody can see how " +
			"to answer is a statement")
	}
	// No identifiers in front of the user.
	if q.Screen != nil && strings.Contains(out, q.Screen.ID) {
		t.Error("the report shows the durable subject id to the user")
	}
}

// renderProposalsFor is the production renderer, over one session's questions.
func renderProposalsFor(session observe.SessionID, ps []observe.Proposal) string {
	var b strings.Builder
	renderProposals(&b, session, ps)
	return b.String()
}

// ── PART 16: the whole thing ──────────────────────────────────────────────────

// Verified, blocked, asked, answered, written down — through the production path, end to end.
func TestTheNamingLifecycleUnblocksTheLearnedPlay(t *testing.T) {
	g, _ := unnamedRegistry(t)

	// 1-4. The lifecycle tries to lower and cannot.
	if _, ok := namingQuestion(t, g); ok {
		t.Fatal("a question existed before anything needed one")
	}
	lower(t, g)

	// 5-6. It asked, and a reader can see it.
	q, ok := namingQuestion(t, g)
	if !ok {
		t.Fatal("nothing asked")
	}

	// 7-10. The user answers through the real surface — and then Marco asks the second
	// question, which only exists because recomputing the lowering found it.
	if err := answerName(t, g, q.ID, "the pause menu"); err != nil {
		t.Fatalf("answering: %v", err)
	}
	nameTheDestination(t, g)

	// 11. It persisted: a store reopened from the same file still knows.
	store := g.memory.(*semanticmemory.Store)
	reopened, why := semanticmemory.Open(store.Path())
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	if _, ok := reopened.SubjectNamed("testgame", "the pause menu"); !ok {
		t.Fatal("the name did not survive being written to disk")
	}

	// 12-14. Lowering recomputes, and the play carries the user's exact words inside the
	// corrected nested guard — no caller injected it and no string was replaced.
	play := onlyPlay(t, lower(t, g))
	for _, want := range []string{
		"use screen.",
		`    do Screen's Showing with "the pause menu"...`,
		"        when ok?",
		`            do OS's Navigate with "confirm".`,
		// AND where it expects to finish — the second name the user gave, read back from
		// durable memory. Success is inside this arm and nowhere else.
		`            do Screen's Showing with "the audio page"...`,
		"                when ok?",
		"                    this is ok!",
		"                or?",
		`                    this is failed with error "this play expected to finish on the audio page"!`,
		"        or?",
		`            this is failed with error "this play starts on the pause menu"!`,
	} {
		if !strings.Contains(play.Source, want) {
			t.Errorf("the play is missing %q:\n%s", want, play.Source)
		}
	}
	// The guard is STRUCTURAL: no effect sits outside the `when ok?` arm.
	for _, line := range strings.Split(play.Source, "\n") {
		if strings.Contains(line, "OS's Navigate") &&
			!strings.HasPrefix(line, "            ") {
			t.Fatalf("an effect sits outside the guard: %q\n%s", line, play.Source)
		}
	}
	// And it says nothing about how Marco came to know any of it.
	lowerSrc := strings.ToLower(play.Source)
	for _, leak := range []string{"subj_", "state_", "digest", "proposal", "question"} {
		if strings.Contains(lowerSrc, leak) {
			t.Errorf("the play mentions %q:\n%s", leak, play.Source)
		}
	}
}

// ── PART 21: naming cannot act ────────────────────────────────────────────────

// Asking for a name, and answering, perform nothing.
func TestNamingAScreenPerformsNothing(t *testing.T) {
	g, _ := unnamedRegistry(t)
	before := treeOf(t, ".")

	lower(t, g)
	q, _ := namingQuestion(t, g)
	if err := answerName(t, g, q.ID, "the pause menu"); err != nil {
		t.Fatalf("answering: %v", err)
	}
	nameTheDestination(t, g)
	play := onlyPlay(t, lower(t, g))
	if play.Source == "" {
		t.Fatal("nothing was generated")
	}

	// No route was written, registered or made resolvable by naming a screen. The working
	// tree is byte-unchanged.
	if got := treeOf(t, "."); len(got) != len(before) {
		t.Fatalf("naming a screen wrote to disk:\nbefore %v\nafter %v", before, got)
	}
	// And no authority was created by any of it. The fixture's grant is whatever the
	// rehearsal chain left; naming must not have touched it.
	if grant := g.last.Grant(); grant != nil && grant.Source != "" {
		if _, ok := namingQuestion(t, g); ok {
			t.Error("an unanswered naming question is somehow still open")
		}
	}
}
