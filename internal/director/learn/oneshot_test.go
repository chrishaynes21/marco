package learn_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/learn"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// ONE demonstration is the normal Learn path, and rehearsal is the confirmation.
//
// # What changed, and why these tests are not the old ones renamed
//
// Every learn test in this package built `CandidateAssessment{Verdict: CandidateConsistent}` with
// NO reasons — a shape `AssessCandidate` cannot produce after one demonstration, because it always
// adds `single_demonstration_only`. So the suite proved the coordinator handled an assessment that
// never occurs, and the assessment that DOES occur went to `needs_another_example` every time.
//
// The tests below use the real shape.

// realFirstAssessment is what AssessCandidate actually returns for one clean demonstration.
func realFirstAssessment() *observe.CandidateAssessment {
	return &observe.CandidateAssessment{
		Verdict: observe.CandidateConsistent,
		Reasons: []observe.AssessmentReason{observe.ReasonSingleDemonstration},
	}
}

// TestOneCleanDemonstrationOffersToTryRatherThanAskingAgain is the correction itself.
//
// Restoring the mandatory second demonstration must fail this.
func TestOneCleanDemonstrationOffersToTryRatherThanAskingAgain(t *testing.T) {
	c, _, p := learnToDemonstration(t, goodCandidate(), realFirstAssessment())

	s := c.Advance(context.Background())
	if s.Phase != learn.ReadyToRehearse {
		t.Fatalf("after ONE clean demonstration the phase is %q, want %q.\nThe user showed "+
			"Marco the thing they asked it to learn, and Marco asked them to do it again — "+
			"not because anything was unclear, but because there had only been one of them.",
			s.Phase, learn.ReadyToRehearse)
	}
	if s.Examples != 1 {
		t.Errorf("examples = %d, want 1", s.Examples)
	}
	if p.calls != 3 {
		t.Errorf("%d passes ran, want 3 (establish, discover, demonstrate) — a fourth means "+
			"a second demonstration was still being watched for", p.calls)
	}
	if len(s.Uncertain) != 0 {
		t.Errorf("nothing was unreadable, but the session reports %v as uncertain", s.Uncertain)
	}
	assertNoBackstageLeak(t, s.Say())
}

// The offer is phrased as an offer, and it is the thing that closes the remaining uncertainty.
func TestTheOfferToTryIsWhatResolvesTheSingleDemonstration(t *testing.T) {
	c, _, _ := learnToDemonstration(t, goodCandidate(), realFirstAssessment())
	s := c.Advance(context.Background())

	say := strings.ToLower(s.Say())
	if !strings.Contains(say, "try") {
		t.Errorf("Marco says %q; at this point it should be offering to try it", s.Say())
	}
	// And it must NOT be asking for another demonstration in the same breath.
	if strings.Contains(say, "show me") || strings.Contains(say, "again") {
		t.Errorf("Marco says %q, which still asks for another example", s.Say())
	}
}

// One demonstration plus a SUCCESSFUL rehearsal reaches a saved play, with no second human
// demonstration anywhere in the run.
//
// This is the whole contract in one test: observed once, tried once, learned.
func TestOneDemonstrationAndOneRehearsalIsEnoughToLearn(t *testing.T) {
	tail := newTail()
	tail.unnamed = []string{"subj_start", "subj_end"}

	c, _, p := learnToDemonstration(t, goodCandidate(), realFirstAssessment())
	c.WithTail(tail).WithPlayName("Downloads", "Open").WithRehearsal(rehearseInWalk)

	ctx := context.Background()
	s := c.Advance(ctx)
	for i := 0; i < 20 && !s.Phase.Settled(); i++ {
		answerEverything(tail, s.Phase)
		s = c.Advance(ctx)
	}

	if s.Phase != learn.Complete {
		t.Fatalf("phase is %q (%s), want %q\n%s",
			s.Phase, s.Refusal, learn.Complete, strings.Join(s.Watch(), "\n"))
	}
	if !s.Learned() {
		t.Fatal("the session says complete and has no durable artifact")
	}
	if s.Examples != 1 {
		t.Errorf("%d demonstrations were required, want 1", s.Examples)
	}
	if tail.rehearsals != 1 {
		t.Errorf("%d rehearsals ran, want exactly 1 — the rehearsal IS the confirmation, and "+
			"it has to actually happen", tail.rehearsals)
	}
	if len(tail.saves) != 1 {
		t.Fatalf("%d saves, want 1", len(tail.saves))
	}
	// Three passes: establish, discover, demonstrate. A fourth would be a second example.
	if p.calls != 3 {
		t.Errorf("%d observation passes, want 3; the user was asked to perform the route "+
			"more than once", p.calls)
	}
}

// ── the discovery pass IS the demonstration ───────────────────────────────────

// twoPlaceMemory recognises the start and the destination as different subjects, which the shared
// fakeMemory cannot do — it answers every Recall with the same recollection, so on that fixture the
// one-shot path correctly refuses and the armed capture takes over.
type twoPlaceMemory struct {
	observe.Memory
	edges    map[observe.RelationshipRef]int
	requests int
}

func (m *twoPlaceMemory) Recall(_ string, sig observe.StructureSignature) observe.Recollection {
	id := startSubject
	if sig.Roles["button"] == 9 {
		id = endSubject
	}
	return observe.Recollection{
		Verdict: observe.MatchSame,
		Subject: observe.RememberedSubject{ID: id, Application: app},
	}
}

func (m *twoPlaceMemory) Topology(string) observe.Topology {
	top := observe.Topology{Subjects: map[string]observe.RememberedSubject{
		startSubject: {ID: startSubject, Application: app},
		endSubject:   {ID: endSubject, Application: app},
	}}
	for ref, n := range m.edges {
		top.Relationships = append(top.Relationships, observe.RememberedRelationship{
			From: ref.From, To: ref.To, Application: app, Observations: n, Sessions: 1,
		})
	}
	return top
}

func (m *twoPlaceMemory) RememberLearning(string, observe.RelationshipRef,
	observe.LearningRequest) error {

	m.requests++
	return nil
}

// watchedTheRoute is a discovery pass that saw the user move from the start to the destination,
// with the order of what they pressed intact.
func watchedTheRoute() observesession.Result {
	res := placedResult("observe_2")
	res.Stats.Shadow = observe.ShadowTotals{
		CurrentState: "state_2",
		States: []observe.ScreenState{
			{ID: "state_1", Episodes: 3, TermObservations: 2,
				Roles: map[string]int{"button": 4, "list": 1}},
			{ID: "state_2", Episodes: 3, TermObservations: 2,
				Roles: map[string]int{"button": 9, "list": 1}},
		},
		Transitions: []observe.ScreenTransition{{
			From: "state_1", To: "state_2", Count: 1,
			Preceded: map[observe.NavIntent]int{observe.NavConfirm: 1},
			Sequences: []observe.TargetedSequence{{
				Intents: []observe.NavIntent{
					observe.NavDown, observe.NavDown, observe.NavConfirm,
				},
				Count: 1,
			}},
		}},
	}
	// What the RUNNER now produces for a licensed pass that watched a clean route: the
	// ordinary candidate, already stored and already assessed by the session that saw it.
	res.Demonstration = &observe.ProcedureCandidate{
		Relationship: observe.RelationshipRef{From: startSubject, To: endSubject},
		Application:  app,
		Start:        observe.Checkpoint{Subject: startSubject, Verdict: observe.MatchSame},
		Steps: []observe.DemonstrationStep{{
			Intents: []observe.NavIntent{
				observe.NavDown, observe.NavDown, observe.NavConfirm,
			},
			Arrived: observe.Checkpoint{Subject: endSubject, Verdict: observe.MatchSame},
		}},
		Complete: true, Reason: observe.ReasonArrived, Events: 3, Checkpoints: 2,
	}
	res.Assessment = realFirstAssessment()
	return res
}

// Learn READS the candidate the session produced and asks for nothing more.
//
// The coordinator half of the one-shot path. Building it here is what ADR-054 removed: a candidate
// made after the session ended never reaches the store, so `AskRehearse` is never raised and the
// grant it waits for cannot exist. Learn's job is to notice the demonstration and move on.
//
// The production-path gate is TestOneWatchedPassProducesACandidateAndAsksToTry, in observesession.
func TestTheDiscoveryPassBecomesTheDemonstrationWithoutAskingAgain(t *testing.T) {
	m := &twoPlaceMemory{edges: map[observe.RelationshipRef]int{}}
	route := observe.RelationshipRef{From: startSubject, To: endSubject}

	start := placedResult("observe_1")
	p := &scriptedPasses{
		results: []observesession.Result{start, watchedTheRoute()},
		onPass: func(n int) {
			if n == 1 {
				m.edges[route] = m.edges[route] + 1
			}
		},
	}
	c := learn.New("open downloads", p, m,
		learn.Bounds{Dwell: time.Second, Watch: 5 * time.Second})

	ctx := context.Background()
	c.Advance(ctx)      // establish the start
	s := c.Advance(ctx) // discover — and, now, demonstrate

	if s.Phase != learn.ReadyToRehearse {
		t.Fatalf("after ONE performance the phase is %q, want %q\n%s",
			s.Phase, learn.ReadyToRehearse, strings.Join(s.Watch(), "\n"))
	}
	if s.Examples != 1 {
		t.Errorf("examples = %d, want 1", s.Examples)
	}
	if p.calls != 2 {
		t.Errorf("%d observation passes ran, want 2 (establish, watch). A third is the armed "+
			"capture asking the person to perform the same route again.", p.calls)
	}
	if s.Demonstration == nil {
		t.Fatal("no demonstration record was built from the pass that watched one")
	}
	// It is the ORDINARY candidate, carrying what was actually observed.
	got := s.Demonstration
	if got.Relationship != route || got.Start.Subject != startSubject {
		t.Errorf("candidate is %+v on %+v", got.Start, got.Relationship)
	}
	want := []observe.NavIntent{observe.NavDown, observe.NavDown, observe.NavConfirm}
	if len(got.Steps) != 1 || !reflect.DeepEqual(got.Steps[0].Intents, want) {
		t.Errorf("steps = %+v, want one step of %v", got.Steps, want)
	}
	assertNoBackstageLeak(t, s.Say())
}

// When the pass cannot support a candidate, the armed capture is still there.
//
// The one-shot path is an ADDED route, not a replacement: a discovery pass with no attributed
// navigation must fall back rather than refuse the whole attempt.
func TestWhenTheWatchedPassCannotSupportACandidateTheCaptureStillRuns(t *testing.T) {
	m := &twoPlaceMemory{edges: map[observe.RelationshipRef]int{}}
	route := observe.RelationshipRef{From: startSubject, To: endSubject}

	silent := watchedTheRoute()
	silent.Stats.Shadow.Transitions[0].Sequences = nil // saw the change, saw no navigation
	// The runner refuses to build one from that, so the result carries none — which is the
	// signal Learn falls back on.
	silent.Demonstration, silent.Assessment = nil, nil

	p := &scriptedPasses{
		results: []observesession.Result{placedResult("observe_1"), silent},
		onPass: func(n int) {
			if n == 1 {
				m.edges[route] = m.edges[route] + 1
			}
		},
	}
	c := learn.New("open downloads", p, m,
		learn.Bounds{Dwell: time.Second, Watch: 5 * time.Second})

	ctx := context.Background()
	c.Advance(ctx)
	s := c.Advance(ctx)

	if s.Phase != learn.NeedsAnotherExample {
		t.Fatalf("phase is %q, want %q — with nothing attributed the armed capture has to "+
			"take over", s.Phase, learn.NeedsAnotherExample)
	}
	if s.Examples != 0 {
		t.Errorf("examples = %d, want 0; nothing was captured", s.Examples)
	}
	// And the person is told the truth about it: this is not a failure, it is the next step.
	if say := strings.ToLower(s.Say()); strings.Contains(say, "wasn't clear enough") {
		t.Errorf("Marco says %q about a discovery pass that worked", s.Say())
	}
}

// FAILING CLOSED: the rehearsal is not a formality.
//
// Skipping it, or saving after it fails, must be caught. Rehearsal carries the confidence a
// second demonstration used to carry, so a rehearsal that did not happen or did not work leaves
// exactly as little evidence as before.
func TestWithoutASuccessfulRehearsalOneDemonstrationLearnsNothing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		attempt learn.Attempt
		answer  func(*stubTail, learn.Phase)
	}{{
		name: "permission was never given",
		answer: func(tail *stubTail, p learn.Phase) {
			if p == learn.Naming {
				tail.name()
			}
		},
	}, {
		name:    "the rehearsal ended somewhere else",
		attempt: learn.Attempt{Attempted: true, Completed: false, Terminal: "wrong_state"},
		answer:  answerEverything,
	}, {
		name:    "the rehearsal could not be verified",
		attempt: learn.Attempt{Attempted: true, Completed: false, Terminal: "unobservable"},
		answer:  answerEverything,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			tail := newTail()
			tail.attempt = tc.attempt
			c, _, _ := learnToDemonstration(t, goodCandidate(), realFirstAssessment())
			c.WithTail(tail).WithPlayName("Downloads", "Open").WithRehearsal(rehearseInWalk)

			ctx := context.Background()
			s := c.Advance(ctx)
			for i := 0; i < 20 && !s.Phase.Settled(); i++ {
				tc.answer(tail, s.Phase)
				s = c.Advance(ctx)
			}
			if s.Learned() {
				t.Fatalf("a play was learned with no successful rehearsal behind it (%s).\n"+
					"One demonstration is the normal path ONLY because Marco proves it "+
					"understood by doing the thing; without that proof there is one "+
					"observation and nothing else.", tc.name)
			}
			if len(tail.saves) != 0 {
				t.Errorf("%d save(s) happened anyway", len(tail.saves))
			}
		})
	}
}

// Ambiguous evidence still fails closed — one-shot is not "accept whatever came first".
func TestOneShotDoesNotMeanAcceptAnything(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reasons []observe.AssessmentReason
		verdict observe.CandidateVerdict
		want    learn.Phase
	}{{
		name: "a run too long to read as deliberate",
		reasons: []observe.AssessmentReason{
			observe.ReasonSingleDemonstration, observe.ReasonAmbiguousRun,
		},
		verdict: observe.CandidateAmbiguous, want: learn.NeedsAnotherExample,
	}, {
		name: "a screen on the route with no identity",
		reasons: []observe.AssessmentReason{
			observe.ReasonSingleDemonstration, observe.ReasonTransientCheckpoint,
		},
		verdict: observe.CandidateInsufficient, want: learn.NeedsAnotherExample,
	}, {
		name: "the user doubled back",
		reasons: []observe.AssessmentReason{
			observe.ReasonSingleDemonstration, observe.ReasonBacktracking,
		},
		verdict: observe.CandidateAmbiguous, want: learn.NeedsAnotherExample,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			c, _, _ := learnToDemonstration(t, goodCandidate(),
				&observe.CandidateAssessment{Verdict: tc.verdict, Reasons: tc.reasons})
			s := c.Advance(context.Background())
			if s.Phase != tc.want {
				t.Fatalf("phase is %q, want %q — uncertain evidence reached the offer to try",
					s.Phase, tc.want)
			}
			if s.Phase == learn.NeedsAnotherExample && len(s.Uncertain) == 0 {
				t.Error("another example was asked for and nothing was named as uncertain")
			}
			// The single-demonstration reason must NOT be among what is asked about.
			for _, r := range s.Uncertain {
				if r == observe.ReasonSingleDemonstration {
					t.Errorf("the request for more evidence names %q, which is not "+
						"something a person can fix by repeating themselves", r)
				}
			}
		})
	}
}

// unclearAssessment is a demonstration Marco understood but could not fully check.
//
// The three-way split Fast Learn rests on, and this is the middle case:
//
//	CandidateConsistent, nothing blocking   admit it — the person showed Marco and it is clear
//	NOT consistent, nothing blocking        offer to TRY — the gap is one Marco could close itself
//	something blocking                      ask for another example — trying would not settle it
//
// `CandidateInsufficient` is "coherent evidence with a gap Marco cannot currently close", and
// `ReasonSingleDemonstration` is the one reason that is confirmable by rehearsal rather than by
// another demonstration. Together they are precisely a route worth offering to try.
func unclearAssessment() *observe.CandidateAssessment {
	return &observe.CandidateAssessment{
		Verdict: observe.CandidateInsufficient,
		Reasons: []observe.AssessmentReason{observe.ReasonSingleDemonstration},
	}
}
