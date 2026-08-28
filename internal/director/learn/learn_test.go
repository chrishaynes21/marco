package learn_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/learn"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// Orchestration, tested deterministically.
//
// These do NOT test Learn. Learn needs a person, because the whole point of the
// injected-input exclusion is that Marco's own keystrokes can never become evidence about how
// somebody works. What they test is the coordinator: that it waits for the evidence it says it
// waits for, arms through the one path that exists, refuses for the reason it names, and creates
// nothing it is not allowed to create.
//
// The fakes below supply session RESULTS — the ordinary output of the ordinary runner. They
// cannot supply a subject, a transition, a candidate or an assessment that the production types
// would not also have produced, because they are the production types.

// ── the doubles ───────────────────────────────────────────────────────────────

const (
	startSubject = "subj_start"
	endSubject   = "subj_end"
	elsewhere    = "subj_elsewhere"
	app          = "explorer"
)

// scriptedPasses hands back one prepared Result per Observe call, in order.
type scriptedPasses struct {
	results []observesession.Result
	errs    []error
	calls   int
	// durations records what each pass was asked to watch for.
	durations []time.Duration
	// onPass runs before returning, so a test can mutate memory the way a real session would.
	onPass func(n int)
	// finished counts Finish calls: the person saying the demonstration is over.
	finished int
}

func (p *scriptedPasses) Observe(_ context.Context, d time.Duration) (
	observesession.Result, error) {

	n := p.calls
	p.calls++
	p.durations = append(p.durations, d)
	if p.onPass != nil {
		p.onPass(n)
	}
	if n < len(p.errs) && p.errs[n] != nil {
		return observesession.Result{}, p.errs[n]
	}
	if n < len(p.results) {
		return p.results[n], nil
	}
	return observesession.Result{}, errors.New("the script ran out of passes")
}

// fakeMemory is durable knowledge with the two behaviours Learn depends on: it resolves the
// current place, and it refuses a learning request for a route it does not hold.
type fakeMemory struct {
	observe.Memory
	here     observe.Recollection
	edges    map[observe.RelationshipRef]int
	requests []request
	refuse   error
}

type request struct {
	ref observe.RelationshipRef
	req observe.LearningRequest
}

func newMemory() *fakeMemory {
	return &fakeMemory{
		here:  observe.Recollection{Verdict: observe.MatchSame, Subject: observe.RememberedSubject{ID: startSubject}},
		edges: map[observe.RelationshipRef]int{},
	}
}

func (m *fakeMemory) Recall(string, observe.StructureSignature) observe.Recollection {
	return m.here
}

func (m *fakeMemory) Topology(string) observe.Topology {
	var rels []observe.RememberedRelationship
	for ref, n := range m.edges {
		rels = append(rels, observe.RememberedRelationship{
			From: ref.From, To: ref.To, Application: app, Observations: n,
		})
	}
	return observe.Topology{Relationships: rels}
}

func (m *fakeMemory) RememberLearning(_ string, ref observe.RelationshipRef,
	req observe.LearningRequest) error {

	if m.refuse != nil && req.Status == observe.LearningPending {
		return m.refuse
	}
	m.requests = append(m.requests, request{ref: ref, req: req})
	return nil
}

// placedResult is a session that ended somewhere the identity path can describe.
func placedResult(id observe.SessionID) observesession.Result {
	return observesession.Result{
		Session: observe.Session{ID: id, Application: app},
		Stats: observesession.Stats{
			SamplesTaken: 12,
			Shadow: observe.ShadowTotals{
				CurrentState: "state_1",
				States: []observe.ScreenState{{
					ID: "state_1", Episodes: 3, TermObservations: 2,
					Roles: map[string]int{"button": 4, "list": 1},
				}},
			},
		},
	}
}

// ── A. the start has to be established before anything else happens ───────────

func TestLearnWaitsForTheStartBeforeItSaysGoAhead(t *testing.T) {
	m := newMemory()
	p := &scriptedPasses{results: []observesession.Result{placedResult("observe_1")}}
	c := learn.New("open downloads", p, m, learn.Bounds{Dwell: time.Second, Watch: time.Second})

	// A fresh session WAITS. It used to begin by establishing a start immediately, which is
	// only right when a person has already chosen what they are looking at — and the way a
	// session actually begins is somebody pressing a button in Marco, which puts Marco in
	// front. See learn.WaitingForDemonstration.
	if got := c.Session().Phase; got != learn.WaitingForDemonstration {
		t.Fatalf("a fresh learn session is in %q, want %q", got, learn.WaitingForDemonstration)
	}
	if say := c.Session().Say(); strings.TrimSpace(say) == "" {
		t.Error("a waiting session says nothing at all; the person is looking at it")
	}
	s := c.Advance(context.Background())
	if s.Phase != learn.ReadyForDemo {
		t.Fatalf("after establishing, phase is %q, want %q", s.Phase, learn.ReadyForDemo)
	}
	if s.Start != startSubject {
		t.Errorf("start is %q, want the subject the identity path resolved (%q)",
			s.Start, startSubject)
	}
	if p.calls != 1 {
		t.Errorf("%d passes ran to establish the start, want 1", p.calls)
	}
}

// ── B. no capture is armed before the user has been told to go ────────────────

func TestNoRequestIsWrittenBeforeTheUserHasDemonstratedAnything(t *testing.T) {
	m := newMemory()
	p := &scriptedPasses{results: []observesession.Result{placedResult("observe_1")}}
	c := learn.New("open downloads", p, m, learn.Bounds{Dwell: time.Second, Watch: time.Second})
	c.Advance(context.Background())

	if len(m.requests) != 0 {
		t.Fatalf("a learning request was written before the user was asked to demonstrate: %+v",
			m.requests)
	}
	if say := c.Session().Say(); !strings.Contains(strings.ToLower(say), "show me") {
		t.Errorf("after the start is established Marco says %q; it should invite the "+
			"demonstration", say)
	}
}

// ── C. a start that cannot be established is refused, and says why ────────────

// GOAL-CENTRIC: an unrecognisable start no longer ends the attempt. The capability being
// learned is the destination; the start was only ever one known way in. What is honestly
// lost is route evidence, and the session says so in its diagnostics rather than refusing.
func TestAnUnrecognisableStartStillWatches(t *testing.T) {
	m := newMemory()
	m.here = observe.Recollection{Verdict: observe.MatchDifferent}
	p := &scriptedPasses{results: []observesession.Result{placedResult("observe_1")}}
	c := learn.New("open downloads", p, m, learn.Bounds{Dwell: time.Second, Watch: time.Second})

	s := c.Advance(context.Background())
	if s.Phase != learn.ReadyForDemo {
		t.Fatalf("got phase %q (%s), want %q — a person should be able to say \"learn "+
			"this\" from wherever they happen to be standing", s.Phase, s.Refusal,
			learn.ReadyForDemo)
	}
	if s.Start != "" {
		t.Errorf("start = %q; an unestablished start must not be invented", s.Start)
	}
	assertNoBackstageLeak(t, s.Say())
}

// ── D. one complete demonstration, end to end through the coordinator ─────────

// learnToDemonstration drives a coordinator to the point where an example has been judged.
func learnToDemonstration(t *testing.T, d *observe.ProcedureCandidate,
	a *observe.CandidateAssessment) (*learn.Coordinator, *fakeMemory, *scriptedPasses) {

	t.Helper()
	m := newMemory()
	route := observe.RelationshipRef{From: startSubject, To: endSubject}

	discovery := placedResult("observe_2")
	demo := placedResult("observe_3")
	demo.Demonstration, demo.Assessment = d, a

	p := &scriptedPasses{
		results: []observesession.Result{placedResult("observe_1"), discovery, demo},
		onPass: func(n int) {
			// The DISCOVERY pass is the one that makes the edge durable, exactly as the
			// runner's own end-of-session write does.
			if n == 1 {
				m.edges[route] = m.edges[route] + 1
			}
		},
	}
	c := learn.New("open downloads", p, m,
		learn.Bounds{Dwell: time.Second, Watch: 5 * time.Second})
	c.Advance(context.Background()) // establish
	c.Advance(context.Background()) // discover
	return c, m, p
}

func goodCandidate() *observe.ProcedureCandidate {
	return &observe.ProcedureCandidate{
		Relationship: observe.RelationshipRef{From: startSubject, To: endSubject},
		Application:  app, Complete: true, Reason: observe.ReasonArrived,
		Start: observe.Checkpoint{Subject: startSubject, Verdict: observe.MatchSame},
		Steps: []observe.DemonstrationStep{{
			Intents: []observe.NavIntent{observe.NavDown, observe.NavConfirm},
			Arrived: observe.Checkpoint{Subject: endSubject, Verdict: observe.MatchSame},
		}},
		Events: 2, Checkpoints: 2,
	}
}

func TestOneCompleteDemonstrationReachesTheRehearsalQuestion(t *testing.T) {
	c, m, p := learnToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})

	s := c.Session()
	if s.Phase != learn.NeedsAnotherExample {
		t.Fatalf("after discovery the phase is %q, want %q", s.Phase, learn.NeedsAnotherExample)
	}
	if s.Route.From != startSubject || s.Route.To != endSubject {
		t.Fatalf("discovered route is %+v, want %s → %s", s.Route, startSubject, endSubject)
	}
	// THE arming, and it is the ordinary pending request.
	if len(m.requests) != 1 || m.requests[0].req.Status != observe.LearningPending {
		t.Fatalf("requests after discovery are %+v; want exactly one pending learning request",
			m.requests)
	}
	if m.requests[0].ref != s.Route {
		t.Errorf("the request was written for %+v, not the discovered route %+v",
			m.requests[0].ref, s.Route)
	}

	s = c.Advance(context.Background()) // the demonstration pass
	if s.Phase != learn.ReadyToRehearse {
		t.Fatalf("after a supported demonstration the phase is %q, want %q",
			s.Phase, learn.ReadyToRehearse)
	}
	if s.Examples != 1 {
		t.Errorf("examples = %d, want 1", s.Examples)
	}
	if p.calls != 3 {
		t.Errorf("%d passes ran, want 3 (establish, discover, demonstrate)", p.calls)
	}
	assertNoBackstageLeak(t, s.Say())
}

// ── E. the assessment decides when another example is wanted ──────────────────

// Another example is asked for when something was UNREADABLE — never merely because there has
// been one of them.
//
// The reason here is `transient_checkpoint_unverifiable`: a screen along the route with no durable
// identity, which a second pass genuinely can corroborate. `single_demonstration_only` used to
// stand here and no longer blocks anything; see [[ADR-051-one-demonstration-and-an-attempt]].
func TestAnotherExampleIsAskedForWhenSomethingWasUnreadable(t *testing.T) {
	c, _, _ := learnToDemonstration(t, goodCandidate(), &observe.CandidateAssessment{
		Verdict: observe.CandidateConsistent,
		Reasons: []observe.AssessmentReason{observe.ReasonTransientCheckpoint},
	})
	s := c.Advance(context.Background())
	if s.Phase != learn.NeedsAnotherExample {
		t.Fatalf("phase is %q, want %q — a screen on the route had no durable identity",
			s.Phase, learn.NeedsAnotherExample)
	}
	if say := s.Say(); !strings.Contains(strings.ToLower(say), "again") {
		t.Errorf("Marco says %q; it should ask to see the unclear part again", say)
	}
	// And it must say WHAT was unclear, not just that something was.
	if say := s.Say(); !strings.Contains(strings.ToLower(say), "recognise") {
		t.Errorf("Marco says %q, which does not name the uncertainty. Asking somebody to "+
			"repeat themselves without saying what was unclear makes them guess which part "+
			"to do differently.", say)
	}
}

func TestLearnStopsAskingForExamplesAtTheBound(t *testing.T) {
	needsMore := &observe.CandidateAssessment{
		Verdict: observe.CandidateConsistent,
		Reasons: []observe.AssessmentReason{observe.ReasonTransientCheckpoint},
	}
	c, _, p := learnToDemonstration(t, goodCandidate(), needsMore)
	second := placedResult("observe_4")
	second.Demonstration, second.Assessment = goodCandidate(), needsMore
	p.results = append(p.results, second)

	c.Advance(context.Background()) // example 1
	s := c.Advance(context.Background())
	if s.Examples != learn.MaxExamples {
		t.Fatalf("examples = %d, want the bound %d", s.Examples, learn.MaxExamples)
	}
	if s.Phase != learn.Refused || s.Refusal != learn.ExamplesExhausted {
		t.Fatalf("got %q/%q, want refused/%s", s.Phase, s.Refusal, learn.ExamplesExhausted)
	}
}

// ── F. disagreement is the assessment's word, not Learn's ─────────────────────

func TestDisagreeingDemonstrationsAreNotResolvedByLearn(t *testing.T) {
	c, _, _ := learnToDemonstration(t, goodCandidate(), &observe.CandidateAssessment{
		Verdict: observe.CandidateAmbiguous,
		Reasons: []observe.AssessmentReason{observe.ReasonDemonstrationsDisagree},
	})
	s := c.Advance(context.Background())
	// Learn neither picks a winner nor averages the two routes. The comparison already
	// happened, in the assessment, and its word stands.
	if s.Phase != learn.Refused || s.Refusal != learn.DemonstrationsDisagree {
		t.Fatalf("got %q/%q, want refused/%s", s.Phase, s.Refusal, learn.DemonstrationsDisagree)
	}
	if s.Demonstration == nil {
		t.Fatal("the demonstration was discarded; the evidence must survive the disagreement")
	}
	assertNoBackstageLeak(t, s.Say())
}

// An assessment that did not come out consistent never reaches a question about ACTING.
func TestAnInconsistentAssessmentNeverReachesTheRehearsalQuestion(t *testing.T) {
	c, _, _ := learnToDemonstration(t, goodCandidate(), &observe.CandidateAssessment{
		Verdict: observe.CandidateInvalid,
		Reasons: []observe.AssessmentReason{observe.ReasonStartUnverifiable},
	})
	s := c.Advance(context.Background())
	if s.Phase == learn.ReadyToRehearse {
		t.Fatal("Marco offered to try something its own assessment called invalid")
	}
	if s.Refusal != learn.EvidenceInsufficient {
		t.Fatalf("refusal is %q, want %s", s.Refusal, learn.EvidenceInsufficient)
	}
}

// ── G/H. rehearsal and naming are handed to the questions that already exist ──

func TestLearnCreatesNoAuthorityOfItsOwn(t *testing.T) {
	c, m, _ := learnToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	s := c.Advance(context.Background())

	if s.Phase != learn.ReadyToRehearse {
		t.Fatalf("phase is %q, want %q", s.Phase, learn.ReadyToRehearse)
	}
	if !s.Phase.Waiting() {
		t.Error("ready_to_rehearse must be a WAITING phase: Learn may not advance past it " +
			"on a timer, because only the user can grant permission to act")
	}
	// With no lifecycle wired, advancing refuses honestly rather than proceeding. A
	// coordinator that could walk itself into rehearsing would be authority Learn invented.
	after := c.Advance(context.Background())
	if after.Phase == learn.Rehearsing {
		t.Fatal("Learn walked itself into a rehearsal with nothing behind it")
	}
	if after.Refusal != learn.NoTail {
		t.Errorf("a coordinator with no tail refused with %q, want %s",
			after.Refusal, learn.NoTail)
	}
	// The ONLY durable thing Learn ever wrote is a request to be shown something.
	for _, r := range m.requests {
		switch r.req.Status {
		case observe.LearningPending, observe.LearningDeclined:
		default:
			t.Errorf("Learn wrote a %q request; it may only ask to be shown, or withdraw",
				r.req.Status)
		}
	}
}

// ── I. the requested name is held and never becomes a screen name ─────────────

func TestTheRequestedNameIsNeverUsedAsAScreenName(t *testing.T) {
	c, m, _ := learnToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	s := c.Advance(context.Background())

	if s.Name != "open downloads" {
		t.Fatalf("the requested name is %q, want %q", s.Name, "open downloads")
	}
	if s.Route.From == s.Name || s.Route.To == s.Name || s.Start == s.Name {
		t.Fatal("the behaviour name was used as a screen identity")
	}
	for _, r := range m.requests {
		if r.ref.From == s.Name || r.ref.To == s.Name {
			t.Fatal("the behaviour name reached a durable relationship endpoint")
		}
	}
}

func TestAnUnusableNameIsRefusedBeforeAnybodyIsAskedToDemonstrate(t *testing.T) {
	m := newMemory()
	p := &scriptedPasses{results: []observesession.Result{placedResult("observe_1")}}
	c := learn.New("   ", p, m, learn.DefaultBounds())

	if s := c.Session(); s.Phase != learn.Refused || s.Refusal != learn.NameNotUsable {
		t.Fatalf("got %q/%q, want refused/%s", s.Phase, s.Refusal, learn.NameNotUsable)
	}
	c.Advance(context.Background())
	if p.calls != 0 {
		t.Errorf("%d observation passes ran for a name Marco cannot use; want 0", p.calls)
	}
}

// ── J. cancellation, at every phase ───────────────────────────────────────────

func TestCancellingAtAnyPhaseLeavesNothingBehind(t *testing.T) {
	route := observe.RelationshipRef{From: startSubject, To: endSubject}

	build := func() (*learn.Coordinator, *fakeMemory, *scriptedPasses) {
		m := newMemory()
		demo := placedResult("observe_3")
		demo.Demonstration = goodCandidate()
		demo.Assessment = &observe.CandidateAssessment{Verdict: observe.CandidateConsistent}
		p := &scriptedPasses{
			results: []observesession.Result{
				placedResult("observe_1"), placedResult("observe_2"), demo},
			onPass: func(n int) {
				if n == 1 {
					m.edges[route] = m.edges[route] + 1
				}
			},
		}
		return learn.New("open downloads", p, m,
			learn.Bounds{Dwell: time.Second, Watch: time.Second}), m, p
	}

	for _, steps := range []int{0, 1, 2, 3} {
		c, m, p := build()
		for i := 0; i < steps; i++ {
			c.Advance(context.Background())
		}
		phaseBefore := c.Session().Phase
		c.Cancel()
		s := c.Session()
		if s.Phase != learn.Cancelled {
			t.Fatalf("cancelling from %q left phase %q", phaseBefore, s.Phase)
		}
		// Advancing after a cancel must run nothing.
		callsBefore := p.calls
		c.Advance(context.Background())
		if p.calls != callsBefore {
			t.Errorf("a cancelled session from %q ran another pass", phaseBefore)
		}
		// No request may be left PENDING, or an ordinary session later would arm a capture
		// for something nobody is learning any more.
		for _, r := range m.requests {
			if r.req.Status == observe.LearningPending && !withdrawn(m, r.ref) {
				t.Errorf("cancelling from %q left a pending request for %+v",
					phaseBefore, r.ref)
			}
		}
		if say := s.Say(); !strings.Contains(strings.ToLower(say), "stopped") {
			t.Errorf("a cancelled session says %q", say)
		}
	}
}

// withdrawn reports whether a later request retracted this route.
func withdrawn(m *fakeMemory, ref observe.RelationshipRef) bool {
	for i := len(m.requests) - 1; i >= 0; i-- {
		if m.requests[i].ref == ref {
			return m.requests[i].req.Status != observe.LearningPending
		}
	}
	return false
}

// ── K. nothing Learn does can admit input as evidence ─────────────────────────

func TestLearnHasNoWayToSupplyNavigationEvidence(t *testing.T) {
	// A demonstration that reached its destination with NO attributed navigation is the
	// shape an injected-input session produces: the screen moved, and nothing the person did
	// was admitted. Learn must report that specific gap and must not invent a step.
	silent := goodCandidate()
	silent.Steps = []observe.DemonstrationStep{{
		Arrived: observe.Checkpoint{Subject: endSubject, Verdict: observe.MatchSame},
	}}
	silent.Events = 0

	c, _, _ := learnToDemonstration(t, silent,
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	s := c.Advance(context.Background())

	if s.Phase != learn.Refused || s.Refusal != learn.ActionNotAttributed {
		t.Fatalf("got %q/%q, want refused/%s", s.Phase, s.Refusal, learn.ActionNotAttributed)
	}
	if say := s.Say(); !strings.Contains(say, "couldn't tell what you did") {
		t.Errorf("the refusal reads %q; it must distinguish 'I could not see what you did' "+
			"from 'I could not see the screen change'", say)
	}
	if s.Say() == learn.NothingChanged.Say() {
		t.Error("the two silences read identically")
	}
}

// ── L. Learn neither verifies nor stores nor runs anything ────────────────────

func TestLearnNeverMarksAnythingVerified(t *testing.T) {
	c, _, _ := learnToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	s := c.Advance(context.Background())
	if s.Demonstration != nil && s.Demonstration.Verified {
		t.Error("Learn set Verified on a demonstration")
	}
	if s.Assessment != nil && s.Assessment.Verified {
		t.Error("Learn set Verified on an assessment")
	}
}

// ── the refusal matrix ────────────────────────────────────────────────────────

func TestEveryRefusalIsReadableAndNamesADistinctSituation(t *testing.T) {
	all := []learn.Refusal{
		learn.NoObservation, learn.NothingChanged,
		learn.DestinationNotRecognised, learn.SeveralRoutes,
		learn.RouteNotRemembered, learn.NotArmed, learn.DemonstrationIncomplete,
		learn.RequiresTextEntry, learn.ActionNotAttributed, learn.NotAssessable,
		learn.ApplicationChanged, learn.NameNotUsable, learn.GoalNotRemembered,
		learn.ExamplesExhausted,
		learn.DemonstrationsDisagree, learn.EvidenceInsufficient,
	}
	seen := map[string]learn.Refusal{}
	for _, r := range all {
		say := r.Say()
		if say == "" || say == "I've stopped." {
			t.Errorf("%s has no explanation of its own", r)
		}
		if prev, dup := seen[say]; dup {
			t.Errorf("%s and %s say the same thing: %q", prev, r, say)
		}
		seen[say] = r
		assertNoBackstageLeak(t, say)
	}
}

func TestTheDiscoveryPassRefusesEachWayItCan(t *testing.T) {
	route := observe.RelationshipRef{From: startSubject, To: endSubject}
	second := observe.RelationshipRef{From: startSubject, To: elsewhere}

	cases := []struct {
		name string
		// grow is what became durable during the discovery pass.
		grow []observe.RelationshipRef
		// local is how many transitions stayed session-local.
		local int
		// refuseStore makes the store reject the pending request.
		refuseStore bool
		application string
		want        learn.Refusal
	}{
		{name: "nothing moved", want: learn.NothingChanged},
		{name: "moved somewhere unrecognisable", local: 3,
			want: learn.DestinationNotRecognised},
		{name: "two ways out at once, neither ending where the person stopped",
			grow: []observe.RelationshipRef{route, second}, want: learn.SeveralRoutes},
		{name: "the store will not hold it", grow: []observe.RelationshipRef{route},
			refuseStore: true, want: learn.RouteNotRemembered},
		{name: "we left the application", grow: []observe.RelationshipRef{route},
			application: "notepad", want: learn.ApplicationChanged},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMemory()
			if tc.refuseStore {
				m.refuse = errors.New("no remembered relationship")
			}
			discovery := placedResult("observe_2")
			discovery.Relationships.SessionLocal = tc.local
			if tc.application != "" {
				discovery.Session.Application = tc.application
			}
			grow := tc.grow
			p := &scriptedPasses{
				results: []observesession.Result{placedResult("observe_1"), discovery},
				onPass: func(n int) {
					if n == 1 {
						for _, ref := range grow {
							m.edges[ref] = m.edges[ref] + 1
						}
					}
				},
			}
			c := learn.New("open downloads", p, m,
				learn.Bounds{Dwell: time.Second, Watch: time.Second})
			c.Advance(context.Background())
			s := c.Advance(context.Background())
			if s.Phase != learn.Refused || s.Refusal != tc.want {
				t.Fatalf("got %q/%q, want refused/%s", s.Phase, s.Refusal, tc.want)
			}
			assertNoBackstageLeak(t, s.Say())
		})
	}
}

func TestTheDemonstrationPassRefusesEachWayItCan(t *testing.T) {
	typed := goodCandidate()
	typed.Steps[0].RequiresTextEntry = true

	stopped := goodCandidate()
	stopped.Complete, stopped.Reason = false, observe.ReasonDestinationMismatch

	cases := []struct {
		name string
		d    *observe.ProcedureCandidate
		a    *observe.CandidateAssessment
		want learn.Refusal
	}{
		{name: "no capture ran", d: nil, want: learn.NotArmed},
		{name: "the capture did not finish", d: stopped,
			want: learn.DemonstrationIncomplete},
		{name: "it needed typing", d: typed, want: learn.RequiresTextEntry},
		{name: "nothing judged it", d: goodCandidate(), a: nil, want: learn.NotAssessable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _, _ := learnToDemonstration(t, tc.d, tc.a)
			s := c.Advance(context.Background())
			if s.Phase != learn.Refused || s.Refusal != tc.want {
				t.Fatalf("got %q/%q, want refused/%s", s.Phase, s.Refusal, tc.want)
			}
			assertNoBackstageLeak(t, s.Say())
		})
	}
}

func TestAPassThatProducedNothingIsRefusedRatherThanBelieved(t *testing.T) {
	m := newMemory()
	p := &scriptedPasses{errs: []error{errors.New("the window went away")}}
	c := learn.New("open downloads", p, m, learn.DefaultBounds())
	s := c.Advance(context.Background())
	if s.Phase != learn.Refused || s.Refusal != learn.NoObservation {
		t.Fatalf("got %q/%q, want refused/%s", s.Phase, s.Refusal, learn.NoObservation)
	}

	m2 := newMemory()
	empty := placedResult("observe_1")
	empty.Stats.SamplesTaken = 0
	p2 := &scriptedPasses{results: []observesession.Result{empty}}
	c2 := learn.New("open downloads", p2, m2, learn.DefaultBounds())
	if s := c2.Advance(context.Background()); s.Refusal != learn.NoObservation {
		t.Fatalf("a session with no samples refused with %q, want %s",
			s.Refusal, learn.NoObservation)
	}
}

// ── the two readings ──────────────────────────────────────────────────────────

// backstage is the vocabulary a person must never be shown.
var backstage = []string{
	"subj_", "state_", "fingerprint", "recurrence", "signature", "digest",
	"structural group", "verdict", "members", "topology", "hypothesis",
}

func assertNoBackstageLeak(t *testing.T, line string) {
	t.Helper()
	low := strings.ToLower(line)
	for _, w := range backstage {
		if strings.Contains(low, w) {
			t.Errorf("a Normal-mode line leaks %q: %q", w, line)
		}
	}
}

func TestNormalModeNeverNamesDirectorsBackstage(t *testing.T) {
	c, _, _ := learnToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	s := c.Advance(context.Background())
	for _, p := range []learn.Phase{
		learn.EstablishingStart, learn.ReadyForDemo, learn.Capturing,
		learn.EstablishingDestination, learn.Evaluating, learn.NeedsAnotherExample,
		learn.ReadyToRehearse, learn.Naming, learn.Lowering, learn.Complete,
		learn.Refused, learn.Cancelled,
	} {
		v := s
		v.Phase = p
		if v.Say() == "" {
			t.Errorf("phase %q has nothing to say", p)
		}
		assertNoBackstageLeak(t, v.Say())
	}
}

func TestWatchModeShowsTheEvidenceUnderneath(t *testing.T) {
	c, _, _ := learnToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	s := c.Advance(context.Background())
	panel := strings.Join(s.Watch(), "\n")
	for _, want := range []string{"LEARNING", startSubject, endSubject, "open downloads"} {
		if !strings.Contains(panel, want) {
			t.Errorf("the Watch panel does not mention %q:\n%s", want, panel)
		}
	}
}

// ── bounds ────────────────────────────────────────────────────────────────────

func TestEachPassIsAskedForTheLengthItsQuestionNeeds(t *testing.T) {
	c, _, p := learnToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	c.Advance(context.Background())
	if len(p.durations) != 3 {
		t.Fatalf("%d passes, want 3", len(p.durations))
	}
	if p.durations[0] >= p.durations[1] {
		t.Errorf("the establishing pass watched for %s and the demonstration for %s; "+
			"holding still is a shorter question than showing something",
			p.durations[0], p.durations[1])
	}
}

// ── the goal-centric shape ────────────────────────────────────────────────────

// goalMemory is fakeMemory plus the optional GoalStore, so the write can be observed.
type goalMemory struct {
	*fakeMemory
	goals []observe.Goal
	deny  error
}

func (m *goalMemory) RememberGoal(_ string, g observe.Goal) error {
	if m.deny != nil {
		return m.deny
	}
	m.goals = append(m.goals, g)
	return nil
}

func (m *goalMemory) Goals(string) []observe.Goal { return m.goals }

// A demonstrated route that did not begin where the dwell pass happened to settle is a
// route, not a refusal. The destination is the goal; where the person came from is evidence.
func TestARouteFromSomewhereElseIsStillLearned(t *testing.T) {
	stray := observe.RelationshipRef{From: elsewhere, To: endSubject}
	m := newMemory()
	discovery := placedResult("observe_2")
	p := &scriptedPasses{
		results: []observesession.Result{placedResult("observe_1"), discovery},
		onPass: func(n int) {
			if n == 1 {
				m.edges[stray] = m.edges[stray] + 1
			}
		},
	}
	c := learn.New("open downloads", p, m, learn.Bounds{Dwell: time.Second, Watch: time.Second})
	c.Advance(context.Background())
	s := c.Advance(context.Background())
	if s.Phase == learn.Refused {
		t.Fatalf("refused (%s): a demonstration that began somewhere other than the "+
			"dwelled-on start was thrown away", s.Refusal)
	}
	if s.Route != stray {
		t.Fatalf("route = %v, want %v", s.Route, stray)
	}
}

// The destination becomes a durable GOAL in the person's own words, the moment it is known —
// whatever later happens to the rehearsal tail.
func TestLearningRecordsTheDestinationAsAGoal(t *testing.T) {
	route := observe.RelationshipRef{From: startSubject, To: endSubject}
	m := &goalMemory{fakeMemory: newMemory()}
	discovery := placedResult("observe_2")
	p := &scriptedPasses{
		results: []observesession.Result{placedResult("observe_1"), discovery},
		onPass: func(n int) {
			if n == 1 {
				m.edges[route] = m.edges[route] + 1
			}
		},
	}
	c := learn.New("open downloads", p, m, learn.Bounds{Dwell: time.Second, Watch: time.Second})
	c.Advance(context.Background())
	s := c.Advance(context.Background())
	if s.Phase == learn.Refused {
		t.Fatalf("refused: %s", s.Refusal)
	}
	if len(m.goals) != 1 {
		t.Fatalf("%d goal(s) recorded, want 1", len(m.goals))
	}
	g := m.goals[0]
	if g.Name != "open downloads" || g.Subject != endSubject {
		t.Fatalf("goal = %+v; want the person's words bound to the destination", g)
	}
}

// A name the store will not bind — one name already meaning another outcome — is the
// person's to resolve, and the refusal says so rather than burying it.
func TestAGoalNameConflictRefusesHonestly(t *testing.T) {
	route := observe.RelationshipRef{From: startSubject, To: endSubject}
	m := &goalMemory{fakeMemory: newMemory(),
		deny: errors.New("\"open downloads\" already names reaching subj_other")}
	discovery := placedResult("observe_2")
	p := &scriptedPasses{
		results: []observesession.Result{placedResult("observe_1"), discovery},
		onPass: func(n int) {
			if n == 1 {
				m.edges[route] = m.edges[route] + 1
			}
		},
	}
	c := learn.New("open downloads", p, m, learn.Bounds{Dwell: time.Second, Watch: time.Second})
	c.Advance(context.Background())
	s := c.Advance(context.Background())
	if s.Phase != learn.Refused || s.Refusal != learn.GoalNotRemembered {
		t.Fatalf("got %q/%q, want refused/%s", s.Phase, s.Refusal, learn.GoalNotRemembered)
	}
	assertNoBackstageLeak(t, s.Say())
}

// Finish records that the person ended the demonstration.
//
// A scripted pass returns whenever it is called, so there is nothing to interrupt; what matters
// to the tests is that the coordinator asked, which is the seam a live pass acts on.
func (p *scriptedPasses) Finish() { p.finished++ }

// AwaitSubject returns at once: a scripted pass has a window by construction.
func (p *scriptedPasses) AwaitSubject(context.Context) error { return nil }

// ── the account: sentences a person reads, particulars kept beside them ───────

// A durable subject id is a FACT, and never a part of a sentence.
//
// "start established as subj_7f3a" was one string. It was composed here, travelled through the
// projection untouched, and was rendered to a person in the red block of the Learn panel — a
// durable subject id on the Audience's screen. Every surface showing it was rendering it
// faithfully; there was nothing in the value to tell them which half was backstage.
//
// The facts were KEPT. `subject`, and on the unrecognised-start path `placed`, `verdict`,
// `licensed`, `established` and `reason`, are how that failure is told apart from the four other
// things that look like it. What is asserted here is the SEPARATION: the id is reachable as a
// particular and absent from the sentence.
func TestASubjectIdIsAFactAndNeverPartOfASentence(t *testing.T) {
	establish := func(t *testing.T, m *fakeMemory, res observesession.Result) learn.Session {
		t.Helper()
		p := &scriptedPasses{results: []observesession.Result{res}}
		c := learn.New("open downloads", p, m,
			learn.Bounds{Dwell: time.Second, Watch: time.Second})
		return c.Advance(context.Background())
	}
	recognised := placedResult("observe_1")
	// A pass that DID make a place durable, over a recall that does not match it: the
	// unrecognised-start account reports both, and `established` is an id.
	unrecognised := placedResult("observe_1")
	unrecognised.Places = observe.PlaceEstablishment{Licensed: true, Subject: elsewhere}

	// Both paths through establishStart put an id in the account: the start that WAS
	// recognised carries `subject`, and the one that was not carries `established`.
	for _, tc := range []struct {
		name   string
		memory func() *fakeMemory
		result observesession.Result
		fact   string
	}{
		{"recognised", newMemory, recognised, "subject"},
		{"unrecognised", func() *fakeMemory {
			m := newMemory()
			// Placed somewhere durable, and not matched to it: the account says so, and
			// says which subject it was measured against.
			m.here = observe.Recollection{
				Verdict: observe.MatchDifferent,
				Subject: observe.RememberedSubject{ID: startSubject},
			}
			return m
		}, unrecognised, "established"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := establish(t, tc.memory(), tc.result)
			notes := s.Notes()
			if len(notes) == 0 {
				t.Fatal("the coordinator gave no account of establishing a start")
			}
			for _, n := range notes {
				if strings.Contains(n.Say, "subj_") {
					t.Errorf("a sentence carries a durable subject id: %q.\nA surface "+
						"cannot un-mix a mixed string, and three surfaces trying would "+
						"be three regexes over Marco's own prose.", n.Say)
				}
			}
			// And the id is still there for whoever is allowed to see it.
			found := ""
			for _, n := range notes {
				for _, f := range n.Facts {
					if f.Name == tc.fact {
						found = f.Value
					}
				}
			}
			if found == "" {
				t.Errorf("no note carries a %q fact. The particulars are how this is told "+
					"apart from the other things that look like it, and losing them to "+
					"make a sentence tidy costs more than the leak did:\n%+v",
					tc.fact, notes)
			}
		})
	}
}

// The developer-facing lines are exactly the account, rendered.
//
// Two lists appended to in different places drift. These two are appended to in one statement,
// from one value, which is the only reason Session.Diagnostics can be documented as the RENDERING
// of Session.Account rather than as a second account of the same run — and the reason a reader of
// either can trust the other.
func TestTheDiagnosticLinesAreExactlyTheAccountRendered(t *testing.T) {
	m := newMemory()
	m.here = observe.Recollection{
		Verdict: observe.MatchDifferent,
		Subject: observe.RememberedSubject{ID: startSubject},
	}
	p := &scriptedPasses{results: []observesession.Result{placedResult("observe_1")}}
	c := learn.New("open downloads", p, m, learn.Bounds{Dwell: time.Second, Watch: time.Second})
	s := c.Advance(context.Background())

	if len(s.Account) == 0 {
		t.Fatal("the session carries no account at all")
	}
	if len(s.Diagnostics) != len(s.Account) {
		t.Fatalf("%d diagnostic line(s) against %d note(s). One list is being written "+
			"somewhere the other is not, and a reader of either can no longer trust it:\n"+
			"%v\n%+v", len(s.Diagnostics), len(s.Account), s.Diagnostics, s.Account)
	}
	for i, n := range s.Account {
		if s.Diagnostics[i] != n.Line() {
			t.Errorf("diagnostic %d is %q, and the note at the same index renders as %q",
				i, s.Diagnostics[i], n.Line())
		}
	}
	// The rendering is not empty of the particulars: a note WITH facts must show them, or
	// "Diagnostics is the rendering of Account" is true only because both lost the same half.
	carried := false
	for i, n := range s.Account {
		if len(n.Facts) == 0 {
			continue
		}
		carried = true
		for _, f := range n.Facts {
			if !strings.Contains(s.Diagnostics[i], f.Name+"="+f.Value) {
				t.Errorf("the developer line %q dropped the particular %s=%s",
					s.Diagnostics[i], f.Name, f.Value)
			}
		}
	}
	if !carried {
		t.Error("no note in this account carries particulars, so this test proved nothing " +
			"about the rendering; the unrecognised-start path is supposed to record five")
	}
}

// REUSING A NAME SAYS WHAT IT USED TO MEAN.
//
// # The silence this closes
//
// A name that already means somewhere else is REBOUND by the store rather than refused, for a
// reason measured live on 2026-08-17: a goal left behind by a failed learn held its name hostage,
// so refusing punished the person for Marco's own earlier failure. That rule stands.
//
// What was missing is the saying. Marco finished with "I learned X. You can ask me to do it
// later." and nothing about X having stopped meaning what it meant — so somebody would believe
// they had two commands, and the old one would silently be the new one.
//
// Deleting the observe.ReboundFrom read, or the Complete-phase sentence, must fail this.
func TestReusingANameSaysWhatItUsedToMean(t *testing.T) {
	route := observe.RelationshipRef{From: startSubject, To: endSubject}
	m := &goalMemory{fakeMemory: newMemory()}
	// The name already means somewhere else, the way it would after an earlier learn.
	m.goals = []observe.Goal{{Name: "open downloads", Subject: "subj_somewhere_else"}}

	p := &scriptedPasses{
		results: []observesession.Result{placedResult("observe_1"), placedResult("observe_2")},
		onPass: func(n int) {
			if n == 1 {
				m.edges[route] = m.edges[route] + 1
			}
		},
	}
	c := learn.New("open downloads", p, m, learn.Bounds{Dwell: time.Second, Watch: time.Second})
	c.Advance(context.Background())
	s := c.Advance(context.Background())
	if s.Phase == learn.Refused {
		t.Fatalf("refused: %s", s.Refusal)
	}
	if s.ReboundFrom != "subj_somewhere_else" {
		t.Fatalf("the session says the name used to mean %q; it meant subj_somewhere_else",
			s.ReboundFrom)
	}

	// AND THE PERSON IS TOLD, at the phase where they are told what was learned.
	done := s
	done.Phase = learn.Complete
	said := done.Say()
	if !strings.Contains(said, "used to mean") {
		t.Errorf("what the person reads does not say the name changed meaning: %q", said)
	}
	// AND NOT WHERE, because Normal never names a subject id.
	if strings.Contains(said, "subj_") {
		t.Errorf("the Normal line names a subject id: %q", said)
	}

	// A FIRST USE SAYS NOTHING, so the sentence stays meaningful.
	fresh := done
	fresh.ReboundFrom = ""
	if plain := fresh.Say(); strings.Contains(plain, "used to mean") {
		t.Errorf("an ordinary learn claims a name changed meaning: %q", plain)
	}
}
