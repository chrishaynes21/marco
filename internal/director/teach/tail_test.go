package teach_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
)

// The tail: from "want me to try it?" to a play on disk.
//
// Every step here is somebody else's decision. The stub below is the ordinary lifecycle with its
// answers scripted — it grants when the user grants, completes when the rehearsal completes,
// demands a name when lowering demands one, and writes when the save writes. What is under test
// is whether the coordinator FOLLOWS it honestly: no shortcut, no overstated claim, and nothing
// called learned that was not written down.

// ── the lifecycle stub ────────────────────────────────────────────────────────

type stubTail struct {
	// granted is the user's answer to the rehearsal question.
	granted bool
	// question is what will be handed back when one is asked for.
	questions map[observe.AskKind]teach.Question
	// attempt and attemptErr are what a rehearsal produces.
	attempt    teach.Attempt
	attemptErr error
	// unnamed is the queue of screens lowering will demand names for, consumed one per
	// `named` call. Empty means lowering is eligible.
	unnamed []string
	// loweringErr, saved and saveErr are the rest.
	loweringErr error
	saved       teach.Saved
	saveErr     error

	// what actually happened, for assertions.
	rehearsals int
	saves      []savedCall
	loweredFor []observe.RelationshipRef
}

type savedCall struct {
	route       observe.RelationshipRef
	actor, verb string
}

func newTail() *stubTail {
	return &stubTail{
		questions: map[observe.AskKind]teach.Question{
			observe.AskRehearse:   {ID: "q_rehearse", SessionID: "observe_3"},
			observe.AskNameScreen: {ID: "q_name", SessionID: "observe_3"},
		},
		attempt: teach.Attempt{Attempted: true, Completed: true, Terminal: "arrived", Live: true},
		saved:   teach.Saved{Name: "downloads-open", Saved: true, Source: "use screen.\n"},
	}
}

func (s *stubTail) Question(_ observe.RelationshipRef, kind observe.AskKind) (
	teach.Question, bool) {

	q, ok := s.questions[kind]
	return q, ok
}

func (s *stubTail) Granted(observe.RelationshipRef) bool { return s.granted }

func (s *stubTail) Rehearse(context.Context) (teach.Attempt, error) {
	s.rehearsals++
	return s.attempt, s.attemptErr
}

func (s *stubTail) Lowering(route observe.RelationshipRef) (teach.Readiness, error) {
	s.loweredFor = append(s.loweredFor, route)
	if s.loweringErr != nil {
		return teach.Readiness{}, s.loweringErr
	}
	if len(s.unnamed) > 0 {
		return teach.Readiness{Unnamed: s.unnamed[0],
			Refusals: []string{"screen_unnamed"}}, nil
	}
	return teach.Readiness{Eligible: true, Source: "use screen.\n"}, nil
}

// name simulates the user answering the naming question through the ordinary path.
func (s *stubTail) name() {
	if len(s.unnamed) > 0 {
		s.unnamed = s.unnamed[1:]
	}
}

func (s *stubTail) Save(route observe.RelationshipRef, actor, verb string) (teach.Saved, error) {
	s.saves = append(s.saves, savedCall{route: route, actor: actor, verb: verb})
	if s.saveErr != nil {
		return teach.Saved{}, s.saveErr
	}
	return s.saved, nil
}

// ── driving one whole teach attempt ───────────────────────────────────────────

// taught drives a coordinator from the top through the tail, letting the test answer the
// questions at the moments a person would.
func taught(t *testing.T, tail *stubTail, answer func(*stubTail, teach.Phase)) teach.Session {
	t.Helper()
	c, _, _ := teachToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	c.WithTail(tail).WithPlayName("Downloads", "Open")

	ctx := context.Background()
	s := c.Advance(ctx) // the demonstration pass, then the assessment
	for i := 0; i < 20 && !s.Phase.Settled(); i++ {
		if answer != nil {
			answer(tail, s.Phase)
		}
		s = c.Advance(ctx)
	}
	return s
}

// answerEverything is a cooperative user: yes to the rehearsal, a name whenever asked.
func answerEverything(tail *stubTail, p teach.Phase) {
	switch p {
	case teach.ReadyToRehearse:
		tail.granted = true
	case teach.Naming:
		tail.name()
	}
}

// Part 27 — the whole lifecycle, through production domain paths only.
func TestOneWholeTeachAttemptEndsWithAPlayOnDisk(t *testing.T) {
	tail := newTail()
	tail.unnamed = []string{"subj_start", "subj_end"} // both endpoints need a name
	s := taught(t, tail, answerEverything)

	if s.Phase != teach.Complete {
		t.Fatalf("phase is %q (%s), want %q\n%s",
			s.Phase, s.Refusal, teach.Complete, strings.Join(s.Watch(), "\n"))
	}
	if !s.Learned() {
		t.Fatal("the session says complete and has no durable artifact")
	}
	if tail.rehearsals != 1 {
		t.Errorf("%d rehearsals ran, want exactly 1", tail.rehearsals)
	}
	if s.Named != 2 {
		t.Errorf("%d screens were named, want 2 (both endpoints)", s.Named)
	}
	if len(tail.saves) != 1 {
		t.Fatalf("%d saves, want 1", len(tail.saves))
	}
	got := tail.saves[0]
	if got.route != s.Route {
		t.Errorf("saved route %+v, want the taught route %+v", got.route, s.Route)
	}
	if got.actor != "Downloads" || got.verb != "Open" {
		t.Errorf("saved as %q/%q, want the validated pair Downloads/Open", got.actor, got.verb)
	}
	if say := s.Say(); !strings.Contains(say, "open downloads") {
		t.Errorf("completion says %q; it should name what the user asked for", say)
	}
	// The judgement was recomputed after each name rather than remembered.
	if len(tail.loweredFor) < 3 {
		t.Errorf("lowering was consulted %d times; it must be recomputed after each name, "+
			"never cached", len(tail.loweredFor))
	}
	assertNoBackstageLeak(t, s.Say())
}

// Part 14 — nothing may claim completion before a durable artifact exists.
func TestCompletionIsNeverClaimedWithoutASavedPlay(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(*stubTail)
		want teach.Refusal
	}{
		{"the save failed", func(s *stubTail) { s.saveErr = errors.New("disk full") },
			teach.SaveFailed},
		{"the save wrote nothing", func(s *stubTail) { s.saved = teach.Saved{Saved: false} },
			teach.SaveFailed},
		{"lowering refused", func(s *stubTail) {
			s.loweringErr = errors.New("nothing is ready to be written down")
		}, teach.NotLowerable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tail := newTail()
			tc.set(tail)
			s := taught(t, tail, answerEverything)
			if s.Phase == teach.Complete || s.Learned() {
				t.Fatalf("Teach claimed completion: %+v", s.Saved)
			}
			if s.Refusal != tc.want {
				t.Fatalf("refusal is %q, want %s", s.Refusal, tc.want)
			}
			assertNoBackstageLeak(t, s.Say())
		})
	}
}

// Part 2 — silence never authorises.
func TestSilenceNeverAuthorisesARehearsal(t *testing.T) {
	tail := newTail() // granted stays false
	clock := &fakeClock{at: time.Unix(1_700_000_000, 0)}

	c, _, _ := teachToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	c.WithTail(tail).WithPlayName("Downloads", "Open").WithClock(clock.now)

	s := c.Advance(context.Background())
	if s.Phase != teach.ReadyToRehearse {
		t.Fatalf("phase is %q, want %q", s.Phase, teach.ReadyToRehearse)
	}
	// And the question the user has to answer travels with it. "Want me to try it once?"
	// with no reply address is a prompt nobody can act on.
	if s.Question == nil || s.Question.ID != "q_rehearse" {
		t.Fatalf("the rehearsal phase carries %+v, want the rehearsal question", s.Question)
	}
	// Polling repeatedly is not an answer.
	for i := 0; i < 10; i++ {
		s = c.Advance(context.Background())
		if s.Phase == teach.Rehearsing || tail.rehearsals > 0 {
			t.Fatal("a rehearsal began without anybody saying yes")
		}
	}
	// And it eventually gives up rather than holding a window open forever — as an
	// UNANSWERED question, not as a decline. Silence is not a decision, and reporting it as
	// one puts words in somebody.s mouth. See RehearsalNotStarted for the third case.
	clock.at = clock.at.Add(2 * teach.DefaultBounds().Answer)
	s = c.Advance(context.Background())
	if s.Phase != teach.Refused || s.Refusal != teach.AnswerTimedOut {
		t.Fatalf("got %q/%q, want refused/%s", s.Phase, s.Refusal, teach.AnswerTimedOut)
	}
	if tail.rehearsals != 0 {
		t.Fatalf("%d rehearsals ran with no authority", tail.rehearsals)
	}
}

// Part 4 — a rehearsal that did not complete never becomes a play.
func TestARehearsalThatDidNotCompleteNeverBecomesAPlay(t *testing.T) {
	for _, tc := range []struct {
		name    string
		attempt teach.Attempt
		err     error
		want    teach.Refusal
	}{
		// A refusal waiting cannot fix. `source_unrecognised` used to stand here and no
		// longer refuses at all: the grant is unspent, so Marco waits for the start.
		// [[ADR-055-an-authorised-rehearsal-waits-for-its-start]]
		{name: "refused before acting",
			attempt: teach.Attempt{Attempted: false, Refusal: "no_actuator"},
			want:    teach.RehearsalRefused},
		{name: "ended somewhere else",
			attempt: teach.Attempt{Attempted: true, Completed: false, Terminal: "wrong_state"},
			want:    teach.RehearsalFailed},
		{name: "could not be verified",
			attempt: teach.Attempt{Attempted: true, Completed: false, Terminal: "unobservable"},
			want:    teach.RehearsalFailed},
		{name: "the attempt errored", err: errors.New("the window went away"),
			want: teach.RehearsalFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tail := newTail()
			tail.attempt, tail.attemptErr = tc.attempt, tc.err
			s := taught(t, tail, answerEverything)
			if s.Refusal != tc.want {
				t.Fatalf("got %q, want %s", s.Refusal, tc.want)
			}
			if len(tail.saves) != 0 {
				t.Fatal("a failed rehearsal still reached the save path")
			}
			if s.Learned() {
				t.Fatal("a failed rehearsal was reported as learned")
			}
			assertNoBackstageLeak(t, s.Say())
		})
	}
}

// Part 6 — the behaviour name is never offered as a screen name.
func TestTheBehaviourNameIsNeverOfferedAsAScreenName(t *testing.T) {
	tail := newTail()
	tail.unnamed = []string{"subj_start"}
	s := taught(t, tail, answerEverything)

	if s.Phase != teach.Complete {
		t.Fatalf("phase is %q (%s)", s.Phase, s.Refusal)
	}
	// Teach has no method that could name a screen: the Tail interface has none, which is the
	// structural half. The behavioural half is that the requested phrase went only to Save.
	if len(tail.saves) != 1 {
		t.Fatalf("%d saves", len(tail.saves))
	}
	if tail.saves[0].actor == s.Name || tail.saves[0].verb == s.Name {
		t.Error("the raw requested phrase was used verbatim as a play name half")
	}
}

// Part 16 — Teach never runs what it just learned.
func TestASuccessfulTeachDoesNotRunThePlay(t *testing.T) {
	tail := newTail()
	s := taught(t, tail, answerEverything)
	if s.Phase != teach.Complete {
		t.Fatalf("phase is %q (%s)", s.Phase, s.Refusal)
	}
	// Exactly one attempt ever emitted input, and it was the authorised rehearsal.
	if tail.rehearsals != 1 {
		t.Errorf("%d attempts emitted input after a successful teach, want 1 (the rehearsal)",
			tail.rehearsals)
	}
	// Registration is the store's decision, not Teach's, and Teach reports it rather than
	// asking for it.
	if s.Saved.Registered {
		t.Error("Teach registered the play; saving and registering are two permissions")
	}
}

// Part 17 — cancelling anywhere in the tail leaves nothing behind.
func TestCancellingInTheTailLeavesNothingBehind(t *testing.T) {
	stopAt := []teach.Phase{teach.ReadyToRehearse, teach.Rehearsing, teach.Naming,
		teach.Lowering}

	for _, phase := range stopAt {
		t.Run(string(phase), func(t *testing.T) {
			tail := newTail()
			tail.unnamed = []string{"subj_start"}
			c, m, _ := teachToDemonstration(t, goodCandidate(),
				&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
			c.WithTail(tail).WithPlayName("Downloads", "Open")

			ctx := context.Background()
			s := c.Advance(ctx)
			for i := 0; i < 20 && !s.Phase.Settled() && s.Phase != phase; i++ {
				answerEverything(tail, s.Phase)
				s = c.Advance(ctx)
			}
			if s.Phase != phase {
				t.Skipf("this run did not reach %s (got %s)", phase, s.Phase)
			}
			c.Cancel()
			if got := c.Session().Phase; got != teach.Cancelled {
				t.Fatalf("cancelling at %s left phase %q", phase, got)
			}
			savesBefore := len(tail.saves)
			c.Advance(ctx)
			if len(tail.saves) != savesBefore {
				t.Error("a cancelled session still wrote a play")
			}
			if c.Session().Learned() {
				t.Error("a cancelled session claims it learned something")
			}
			if withdrawnPending(m) {
				t.Error("a pending demonstration request survived the cancel")
			}
		})
	}
}

func withdrawnPending(m *fakeMemory) bool {
	for i := len(m.requests) - 1; i >= 0; i-- {
		return m.requests[i].req.Status == observe.LearningPending
	}
	return false
}

// Part 26 — Watch shows the tail, Normal never leaks it.
func TestTheTailReadsTwoWays(t *testing.T) {
	tail := newTail()
	tail.unnamed = []string{"subj_start"}
	s := taught(t, tail, answerEverything)

	panel := strings.Join(s.Watch(), "\n")
	for _, want := range []string{"rehearsal:", "lowering:", "saved:", "use screen."} {
		if !strings.Contains(panel, want) {
			t.Errorf("the Watch panel does not show %q:\n%s", want, panel)
		}
	}
	// The generated program is developer-facing. Normal mode does not print code at somebody.
	for _, p := range []teach.Phase{teach.Rehearsing, teach.Lowering, teach.Complete} {
		v := s
		v.Phase = p
		if strings.Contains(v.Say(), "use screen") {
			t.Errorf("Normal mode printed the generated program in phase %q", p)
		}
		assertNoBackstageLeak(t, v.Say())
	}
}

// fakeClock is time a test can move.
type fakeClock struct{ at time.Time }

func (c *fakeClock) now() time.Time { return c.at }

// Part 21 — the rehearsal's own input can never come back as evidence.
//
// The exclusion itself lives in the platform navigation source, several layers below, and is
// tested there. What Teach must not do is create a NEW route around it, and there is exactly one
// way it could: watching while Marco acts. It does not — every observation pass is finished before
// the rehearsal is authorised, and nothing after the rehearsal opens another.
func TestARehearsalIsNeverWatchedByATeachPass(t *testing.T) {
	tail := newTail()
	tail.unnamed = []string{"subj_start"}

	c, _, passes := teachToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	c.WithTail(tail).WithPlayName("Downloads", "Open")

	ctx := context.Background()
	s := c.Advance(ctx) // the demonstration pass
	watchedBeforeTheTail := passes.calls
	if watchedBeforeTheTail == 0 {
		t.Fatal("no passes ran at all")
	}
	for i := 0; i < 20 && !s.Phase.Settled(); i++ {
		answerEverything(tail, s.Phase)
		if s.Phase == teach.Rehearsing && passes.calls != watchedBeforeTheTail {
			t.Fatal("an observation pass was running while Marco was rehearsing; " +
				"Marco's own input would be offered as evidence about a person")
		}
		s = c.Advance(ctx)
	}
	if s.Phase != teach.Complete {
		t.Fatalf("phase is %q (%s)", s.Phase, s.Refusal)
	}
	if passes.calls != watchedBeforeTheTail {
		t.Fatalf("%d observation pass(es) ran during the tail, want 0 — the whole tail "+
			"happens after watching has stopped", passes.calls-watchedBeforeTheTail)
	}
	if tail.rehearsals != 1 {
		t.Fatalf("%d rehearsals", tail.rehearsals)
	}
}

// M16's gap: `Learned` must read the ARTIFACT, never the phase.
//
// The phase is orchestration bookkeeping and a bug can set it. The saved play is a file on disk.
// A view that derived "learned" from the phase would tell somebody Marco had learned a play it
// had not written down, and they would find out tomorrow.
func TestLearnedIsReadFromTheArtifactAndNotFromThePhase(t *testing.T) {
	if (teach.Session{Phase: teach.Complete}).Learned() {
		t.Fatal("a session with no saved play reports that it learned one")
	}
	if (teach.Session{Phase: teach.Complete, Saved: &teach.Saved{Saved: false}}).Learned() {
		t.Fatal("a save that wrote nothing reports that it learned something")
	}
	if !(teach.Session{Phase: teach.Complete,
		Saved: &teach.Saved{Saved: true, Name: "x"}}).Learned() {
		t.Fatal("a saved play does not report as learned")
	}
	// And the other direction: an artifact without the phase is not completion either, but
	// the artifact is what is true — Learned answers "is there a play", nothing more.
	if !(teach.Session{Phase: teach.Lowering, Saved: &teach.Saved{Saved: true}}).Learned() {
		t.Fatal("Learned must answer whether a play exists, independently of the phase")
	}
}

// M17's gap: an unanswered naming question must not advance.
//
// The naming wait watches the JUDGEMENT — while lowering still demands the same screen, nothing
// has changed. A coordinator that advanced anyway would loop back into lowering, find the same
// demand, and eventually declare the play unwritable for a reason that was never true.
func TestAnUnansweredNamingQuestionDoesNotAdvance(t *testing.T) {
	tail := newTail()
	tail.unnamed = []string{"subj_start"} // and the user never answers
	clock := &fakeClock{at: time.Unix(1_700_000_000, 0)}

	c, _, _ := teachToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	c.WithTail(tail).WithPlayName("Downloads", "Open").WithClock(clock.now)

	ctx := context.Background()
	s := c.Advance(ctx)
	for i := 0; i < 6 && s.Phase != teach.Naming; i++ {
		if s.Phase == teach.ReadyToRehearse {
			tail.granted = true
		}
		s = c.Advance(ctx)
	}
	if s.Phase != teach.Naming {
		t.Fatalf("phase is %q (%s), want %q", s.Phase, s.Refusal, teach.Naming)
	}
	// The user has to be able to ANSWER it. A phase that says "what do you call this
	// screen?" and carries no question id is a prompt with no reply address.
	if s.Question == nil || s.Question.ID == "" {
		t.Fatal("the naming phase carries no question; Teach cannot invent one and must " +
			"surface the one the judgement raised")
	}
	if s.Question.ID != "q_name" {
		t.Errorf("the naming phase surfaced %q, want the naming question", s.Question.ID)
	}
	// Polling is not an answer, however often.
	for i := 0; i < 10; i++ {
		s = c.Advance(ctx)
		if s.Named != 0 {
			t.Fatalf("Teach counted %d names with nobody having answered", s.Named)
		}
		if len(tail.saves) != 0 {
			t.Fatal("a play was written while a screen was still unnamed")
		}
		if s.Phase.Settled() {
			t.Fatalf("the wait ended early as %q/%q", s.Phase, s.Refusal)
		}
	}
	// And it eventually gives up rather than holding the window open forever.
	clock.at = clock.at.Add(2 * teach.DefaultBounds().Answer)
	s = c.Advance(ctx)
	if s.Phase != teach.Refused || s.Refusal != teach.NameRefused {
		t.Fatalf("got %q/%q, want refused/%s", s.Phase, s.Refusal, teach.NameRefused)
	}
	if s.Learned() {
		t.Fatal("an unnamed play was reported as learned")
	}
}

// M34's gap: naming is bounded.
//
// A judgement that finds a new screen to name after every recompute would otherwise loop forever,
// holding a window under observation and asking a question that never ends. Three is a route's two
// endpoints plus slack; past that the honest answer is that this cannot be written down.
func TestTeachStopsAskingForNamesAtTheBound(t *testing.T) {
	tail := newTail()
	// A judgement that always wants one more, and a different one each time.
	round := 0
	tail.unnamed = []string{"subj_1"}
	naming := func(tl *stubTail, p teach.Phase) {
		switch p {
		case teach.ReadyToRehearse:
			tl.granted = true
		case teach.Naming:
			round++
			tl.unnamed = []string{"subj_" + itoa(round+1)}
		}
	}
	s := taught(t, tail, naming)

	if s.Phase != teach.Refused || s.Refusal != teach.NotLowerable {
		t.Fatalf("got %q/%q, want refused/%s", s.Phase, s.Refusal, teach.NotLowerable)
	}
	if s.Named > teach.MaxNameRounds {
		t.Fatalf("Teach asked for %d names; the bound is %d", s.Named, teach.MaxNameRounds)
	}
	if len(tail.saves) != 0 {
		t.Error("a play was written while a screen was still unnamed")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// A tail that can say WHY a yes created nothing lends that reason to the diagnostics and to
// the eventual refusal — because "nobody authorised a rehearsal" is false when somebody did
// and the authorization silently failed.
type diagnosingTail struct {
	*stubTail
	why string
}

func (d *diagnosingTail) GrantRefusal(observe.RelationshipRef) string { return d.why }

func TestAYesThatCreatedNoAuthorityIsSaidOutLoud(t *testing.T) {
	tail := &diagnosingTail{stubTail: newTail(), why: "evidence_moved"}
	clock := &fakeClock{at: time.Unix(1_700_000_000, 0)}

	c, _, _ := teachToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	c.WithTail(tail).WithPlayName("Downloads", "Open").WithClock(clock.now)

	s := c.Advance(context.Background())
	if s.Phase != teach.ReadyToRehearse {
		t.Fatalf("phase is %q, want %q", s.Phase, teach.ReadyToRehearse)
	}
	found := false
	for _, line := range s.Diagnostics {
		if strings.Contains(line, "evidence_moved") {
			found = true
		}
	}
	if !found {
		t.Errorf("the diagnostics never carry the authorization refusal:\n%v", s.Diagnostics)
	}
	clock.at = clock.at.Add(2 * teach.DefaultBounds().Answer)
	s = c.Advance(context.Background())
	// A YES THAT CREATED NO AUTHORITY IS NOT A DECLINE. This test.s own last assertion
	// always said so — "blames silence rather than the failed authorization" — and the
	// refusal kind now agrees with it.
	if s.Phase != teach.Refused || s.Refusal != teach.RehearsalNotStarted {
		t.Fatalf("got %q/%q, want refused/%s", s.Phase, s.Refusal, teach.RehearsalNotStarted)
	}
	last := s.Diagnostics[len(s.Diagnostics)-1]
	if !strings.Contains(last, "created no authority") || !strings.Contains(last, "evidence_moved") {
		t.Errorf("the terminal refusal blames silence rather than the failed authorization:\n%s",
			last)
	}
}

// unregisteringTail writes the play and cannot make it askable.
type unregisteringTail struct{ *stubTail }

func (u *unregisteringTail) Save(r observe.RelationshipRef, actor, verb string) (
	teach.Saved, error) {

	return teach.Saved{Name: "mousesettings", Saved: true, Registered: false}, nil
}

// A PLAY IS NOT CALLED ASKABLE UNTIL IT IS REGISTERED.
//
// Completion says "you can ask me to do it later". A play written down but not registered lives
// where the resolver cannot see it, and `marco routes` reports nothing — so the sentence is a
// promise the product does not keep.
//
// The artifact is KEPT. It is readable, editable and correct; deleting it to tidy a message would
// destroy work. What is refused is the claim.
//
// Deleting the Registered check must fail this.
func TestAPlayIsNotCalledAskableUntilItIsRegistered(t *testing.T) {
	inner := newTail()
	inner.granted = true // the Audience said yes; this test is about what SAVE claims
	tail := &unregisteringTail{stubTail: inner}
	c, _, _ := teachToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	c.WithTail(tail).WithPlayName("Mouse", "Open")

	s := c.Advance(context.Background())
	for i := 0; i < 30 && !s.Phase.Settled(); i++ {
		if s.Phase == teach.Naming {
			inner.name()
		}
		s = c.Advance(context.Background())
	}

	if s.Phase == teach.Complete {
		t.Fatal("Marco said the play was learned and askable. Nothing can ask for it: it is " +
			"saved somewhere route discovery never looks.")
	}
	if s.Refusal != teach.PlayNotRegistered {
		t.Errorf("the outcome is %q, want %q", s.Refusal, teach.PlayNotRegistered)
	}
	// The artifact survives the refusal.
	if s.Saved == nil || !s.Saved.Saved {
		t.Error("the written play was discarded to make the message tidy")
	}
}
