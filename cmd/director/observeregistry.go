package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// One passive observation session at a time, owned by the service.
//
// One, because a second would contend for the screen with the first and neither could
// attribute what it saw. Starting another is refused with the active id rather than queued:
// a queued session would start at a moment nobody chose, describing whatever happened to be
// happening then.
//
// # The lock is never held over the work
//
// Everything slow — validation, capture, detection, OCR, fusion, analysis — happens outside
// this lock, which is only ever taken to copy a few fields. That is what makes
// `director status` answer while a detector is running, and it is the property the barrier
// test exists to hold.

// MaxRetainedSessions is how many finished sessions stay readable.
//
// IN MEMORY ONLY for this milestone: a service restart loses them, and the diagnostics say
// so rather than implying a store that does not exist.
const MaxRetainedSessions = 10

// observationRegistry owns the active session and the recent finished ones.
type observationRegistry struct {
	mu sync.RWMutex

	active   *observesession.Runner
	activeID observe.SessionID
	cancel   context.CancelFunc
	// last is the most recent runner, kept after its session retires.
	//
	// It holds one thing that matters past the end of a session: the ephemeral rehearsal
	// grant, if the user gave one. Kept in memory only, never written, and replaced by the
	// next session — so a restart begins with no authority, which is the point.
	last *observesession.Runner
	// lastTarget and lastSampler are the perception the last session used.
	//
	// Kept so a rehearsal looks through the SAME eyes: the window it validates and the
	// detector it reads are the ones that produced the evidence being rehearsed, not a second
	// set that might disagree about which window is which.
	lastTarget  observesession.Target
	lastSampler observesession.Sampler

	// done is closed when the active session retires, so a caller that needs the session's
	// conclusion before deciding what to do next can wait for it instead of polling.
	done chan struct{}

	nextID   int
	finished []observesession.Result

	// memory is durable semantic knowledge, shared by every session this service runs.
	//
	// Owned here rather than per-session because it OUTLIVES a session: that is the whole
	// point. Nil-safe — a Director whose memory could not be read still observes, still
	// asks, and simply does not recognise.
	memory observe.Memory
}

func newObservationRegistry() *observationRegistry { return &observationRegistry{} }

// withMemory installs durable semantic memory for every session this registry runs.
func (g *observationRegistry) withMemory(m observe.Memory) *observationRegistry {
	g.memory = m
	return g
}

// memoryUnavailable explains why cross-session recognition cannot run, empty when it can.
//
// Asked of the store rather than assumed from nil-ness: a store that exists and could not read
// its file is a different situation from no store at all, and the reader deserves the real
// reason in both cases.
func (g *observationRegistry) memoryUnavailable() string {
	type reporter interface{ Unavailable() string }
	if r, ok := g.memory.(reporter); ok {
		return r.Unavailable()
	}
	if g.memory == nil {
		return "no semantic memory is wired, so nothing can be recognised across sessions"
	}
	return ""
}

// Start begins a session, or reports that one is already running.
//
// Returns as soon as the session is under way. The work happens on its own goroutine, so a
// three-minute observation does not hold a client connection open for three minutes.
func (g *observationRegistry) Start(target observesession.Target, sampler observesession.Sampler,
	events observesession.Events, selector windowref.Selector,
	bounds observe.Bounds) (observe.SessionID, error) {

	// An ordinary session is its own episode, corroborates on its own, and establishes no
	// place. Only teaching says otherwise, and it says so through RunPass.
	//
	// The ZERO value is what makes that safe: a caller has to name the licence to get it, and
	// there is exactly one that does.
	return g.start(target, sampler, events, selector, bounds, observesession.Episode{})
}

func (g *observationRegistry) start(target observesession.Target, sampler observesession.Sampler,
	events observesession.Events, selector windowref.Selector,
	bounds observe.Bounds, episode observesession.Episode) (observe.SessionID, error) {

	if err := selector.Validate(); err != nil {
		return "", err
	}
	normalised, err := bounds.Normalise()
	if err != nil {
		return "", err
	}

	g.mu.Lock()
	if g.active != nil {
		id := g.activeID
		g.mu.Unlock()
		return "", fmt.Errorf(
			"observation session %s is already running; cancel it before starting another "+
				"(two sessions would contend for the screen and neither could attribute "+
				"what it saw)", id)
	}
	g.nextID++
	id := observe.SessionID(fmt.Sprintf("observe_%d", g.nextID))
	// The demonstration licence travels to the sampler, because the sampler is where the
	// admitted-label gate for semantic targets is applied. Set from the episode the CALLER
	// declared — the same declaration that licenses establishing places — so a passive
	// session structurally cannot widen what a label may ride on.
	if ls, ok := sampler.(*liveSampler); ok {
		ls.demonstration = episode.EstablishPlaces
	}
	runner := observesession.New(sessionClock, target, sampler, events).WithMemory(g.memory)
	// Somewhere for an approved demonstration to be kept. The same store, reached through the
	// narrow read/write interface the analysis core declares — a candidate has no other route
	// into durable storage and no route at all into anything executable.
	if store, ok := g.memory.(observe.CandidateStore); ok {
		runner = runner.WithCandidates(store)
	}
	// The REPERTOIRE: somewhere a demonstrated semantic target can become durable. The same
	// store once more, through an interface that can write a target's identity and nothing
	// else — no judgement, no relationship, no authority.
	//
	// Deleting this must fail TestADemonstratedTargetBecomesDurable.
	if store, ok := g.memory.(observe.TargetStore); ok {
		runner = runner.WithTargets(store)
	}
	// Somewhere a place's IDENTITY can be made durable. The same store again, through an
	// interface that has no SemanticKnowledge in it at all — so the thing a teach session uses
	// to remember where you were standing structurally cannot claim you said anything about it.
	//
	// Deleting this must fail TestTeachingEstablishesTheStartThroughTheProductionRegistry.
	if store, ok := g.memory.(observe.PlaceStore); ok {
		runner = runner.WithPlaces(store)
	}
	// The runner's identity, for attempt ids. A counter beside it in the analysis core does
	// the rest — a timestamp alone is not enough on Windows, as runtimeEpochCounter records.
	runner = runner.WithEpoch(fmt.Sprintf("s%d", g.nextID))
	ctx, cancel := context.WithCancel(context.Background())
	g.active, g.activeID, g.cancel = runner, id, cancel
	g.done = make(chan struct{})
	// AND kept past the end of the session, because a rehearsal happens AFTER observing —
	// Marco cannot try something while it is passively watching somebody else. The runner is
	// where a yes lands and where the one ephemeral grant lives; retiring it on `finish` would
	// mean the only answer that can create authority could never be given in time.
	// A new session SUPERSEDES any outstanding authorization, and withdraws it rather than
	// letting it lapse quietly. A permission given about one screen, on evidence from one
	// session, has no business being spendable after the user has gone and done something
	// else — and "revoked" is a state a reader can see, where "unreachable" is not.
	g.last.RevokeRehearsal()
	g.last, g.lastTarget, g.lastSampler = runner, target, sampler
	g.mu.Unlock()

	go func() {
		defer cancel()
		// Navigation retention is SESSION-BOUNDED. A Director that kept accumulating the
		// player's intents whenever it happened to be running would be keeping a record of
		// their keyboard for no session's benefit, so the subscription is detached the
		// moment the session ends — however it ends.
		defer func() {
			if d, ok := sampler.(interface{ detachNavigation() }); ok {
				d.detachNavigation()
			}
		}()
		result, err := runner.Run(ctx, observesession.Config{
			ID: id, Selector: selector, Bounds: normalised, Episode: episode,
		})
		if err != nil {
			// A session that never started still becomes a readable record, because
			// "it failed to start" and "it was never asked for" are different answers.
			result = observesession.Result{Session: observe.Session{
				ID: id, Selector: selector, Bounds: normalised,
				State: observe.Failed, Reason: err.Error(),
			}}
		}
		g.finish(result)
	}()

	return id, nil
}

// finish retires the active session into the bounded finished list.
func (g *observationRegistry) finish(result observesession.Result) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active, g.activeID, g.cancel = nil, "", nil
	g.finished = append(g.finished, result)
	if len(g.finished) > MaxRetainedSessions {
		g.finished = g.finished[len(g.finished)-MaxRetainedSessions:]
	}
	if g.done != nil {
		close(g.done)
	}
}

// RunPass starts one session and returns its Result when it ends.
//
// The SYNCHRONOUS face of Start, and it adds no capability: the same registry, the same runner,
// the same one-at-a-time rule, the same bounds. It exists because a teaching session is a
// sequence of bounded passes in which each conclusion decides what the user is asked next, and
// polling a status view for that would be reading a summary where the evidence is available.
//
// Cancelling ctx cancels the session and still waits for its record — the evidence collected
// before the cancel is kept, and a caller that walked away from a running detector would leave
// the registry believing a session was active.
func (g *observationRegistry) RunPass(ctx context.Context, target observesession.Target,
	sampler observesession.Sampler, events observesession.Events,
	selector windowref.Selector, bounds observe.Bounds,
	episode observesession.Episode) (observesession.Result, error) {

	id, err := g.start(target, sampler, events, selector, bounds, episode)
	if err != nil {
		return observesession.Result{}, err
	}
	g.mu.RLock()
	done := g.done
	g.mu.RUnlock()

	select {
	case <-ctx.Done():
		_ = g.Cancel(id)
		<-done
	case <-done:
	}

	g.mu.RLock()
	defer g.mu.RUnlock()
	for i := len(g.finished) - 1; i >= 0; i-- {
		if g.finished[i].Session.ID == id {
			return g.finished[i], nil
		}
	}
	return observesession.Result{}, fmt.Errorf(
		"observation session %s ended without leaving a record", id)
}

// Cancel stops the active session if the id matches.
func (g *observationRegistry) Cancel(id observe.SessionID) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active == nil {
		return fmt.Errorf("no observation session is running")
	}
	if id != "" && id != g.activeID {
		return fmt.Errorf("%s is not the running session (%s is)", id, g.activeID)
	}
	g.cancel()
	return nil
}

// ActiveID is the running session's id, empty when none.
func (g *observationRegistry) ActiveID() observe.SessionID {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.activeID
}

// ObservedApplication is which application is under observation, empty when none.
//
// Used to refuse Director-initiated mutations against a window somebody is passively
// watching: an action the Director took is not evidence about how a person plays.
func (g *observationRegistry) ObservedApplication() string {
	g.mu.RLock()
	runner, id := g.active, g.activeID
	g.mu.RUnlock()
	if runner == nil || id == "" {
		return ""
	}
	session, _ := runner.Snapshot()
	return session.Application
}

// Snapshot returns one session's current state, active or finished.
func (g *observationRegistry) Snapshot(id observe.SessionID) (observationView, bool) {
	g.mu.RLock()
	runner, activeID := g.active, g.activeID
	finished := make([]observesession.Result, len(g.finished))
	copy(finished, g.finished)
	g.mu.RUnlock()

	if runner != nil && (id == "" || id == activeID) {
		session, stats := runner.Snapshot()
		ledger := runner.Proposals()
		// An ACTIVE session generates from the evidence so far. "What is Marco learning
		// while I play" is the question this view exists to answer, and hypotheses that
		// only appeared after the session ended would not answer it.
		v := viewOf(session, stats, nil, nil,
			ledger.Annotate(
				observe.Hypotheses(stats.Shadow, observe.DefaultHypothesisThresholds())),
			true)
		v.Proposals = ledger.Proposals
		v.MemoryUnavailable = g.memoryUnavailable()
		// THE live demonstration view. Read from the runner rather than from a snapshot,
		// because the whole point is that `director status` answers WHILE a demonstration is
		// being watched. Counts and closed vocabulary; there are no raw keys to withhold.
		v.Capturing = runner.Capture()
		return v, true
	}
	for i := len(finished) - 1; i >= 0; i-- {
		r := finished[i]
		if id == "" || r.Session.ID == id {
			// A FINISHED session reports what the terminal Result recorded, so the
			// generator call in the runner is load-bearing for every completed session
			// rather than duplicated here.
			v := viewOf(r.Session, r.Stats, &r.Findings, r.Insights, r.Hypotheses, false)
			v.Proposals = r.Proposals.Proposals
			v.Relationships = r.Relationships
			v.Topology = g.topologyFor(r)
			v.Learning = g.learningViews(r)
			if r.Demonstration != nil {
				v.Demonstration = r.Demonstration.Describe()
			}
			// RECOMPUTED, not read back. A judgement stored beside the observation would
			// still say "unverifiable" long after the user had told Marco what that screen
			// is — see [[ADR-021-a-judgement-is-recomputed-not-recorded]].
			if r.Demonstration != nil && g.memory != nil {
				corr := observe.Corroboration{}
				if store, ok := g.memory.(observe.CandidateStore); ok {
					for _, other := range store.Candidates(r.Session.Application) {
						if other.Relationship == r.Demonstration.Relationship &&
							other.Sequence != r.Demonstration.Sequence {
							corr = observe.Corroboration{Compared: true,
								Agreement: observe.CompareCandidates(
									*r.Demonstration, other)}
						}
					}
				}
				a := observe.AssessCandidate(*r.Demonstration,
					g.memory.Topology(r.Session.Application),
					observe.DefaultCaptureBounds(), corr)
				// AND the rehearsal evidence, folded rather than read back. `verified` is
				// true only while a completed attempt still matches this demonstration
				// and memory still knows both endpoints — see
				// [[ADR-026-verification-is-derived-from-a-completed-rehearsal]].
				if store, ok := g.memory.(observe.RehearsalStore); ok {
					top := g.memory.Topology(r.Session.Application)
					if j, known := g.judgeNow(r.Session.Application,
						r.Demonstration.Relationship); known {
						a = a.WithRehearsal(*r.Demonstration, j.Digest, top,
							store.Rehearsals(r.Session.Application))
					}
				}
				v.Assessment = observe.DescribeAssessment(a)
			}
			for _, f := range r.FollowUps {
				v.FollowUps = append(v.FollowUps,
					observe.DescribeFollowUp(f.Assessment, f.Judgement))
			}
			if g.memory != nil {
				top := g.memory.Topology(r.Session.Application)
				for _, j := range r.Rehearsals {
					v.Rehearsals = append(v.Rehearsals,
						observe.DescribeRehearsal(j, top))
				}
			}
			if gr := g.last.Grant(); gr != nil && gr.Active() {
				v.Authorization = gr.Describe()
			}
			v.MemoryUnavailable = g.memoryUnavailable()
			return v, true
		}
	}
	return observationView{}, false
}

// Answer records the user's response to a question a session asked.
//
// # Why both an active runner and a finished result
//
// Because answering a finished session is the ORDINARY case, not an edge case. A session runs
// for minutes; the question appears in a report the user reads afterwards. A loop that only
// accepted answers while the session was still running would be unusable for the interaction it
// exists to support.
//
// The answer is bound to the proposal id in both paths. Nothing here looks at what is currently
// on screen — see observe.ProposalLedger.Respond.
func (g *observationRegistry) Answer(id observe.SessionID, proposal observe.ProposalID,
	resp observe.UserResponse) (observe.Proposal, bool) {

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.active != nil && (id == "" || id == g.activeID) {
		return g.active.Respond(proposal, resp)
	}
	for i := len(g.finished) - 1; i >= 0; i-- {
		if id != "" && g.finished[i].Session.ID != id {
			continue
		}
		// Stored in place: the ledger on a finished Result is the durable record, and a
		// copy answered and discarded would lose the only ground truth this system gets.
		p, ok := g.finished[i].Proposals.Respond(proposal, resp,
			g.finished[i].Stats.SamplesTaken)
		if !ok {
			continue
		}
		// The hypotheses on a finished Result are a snapshot, so they are re-annotated
		// here; otherwise the answer would be recorded and invisible in the report.
		// AND the semantic half, which the ledger above does not do.
		//
		// Every question this system asks is asked at the END of a session — the review runs
		// on the terminal Result — so by the time anybody can answer one, `g.active` is
		// already nil and the branch above is the ONLY branch. The ledger is the record of
		// what was said; `Runner.Respond` is what makes it mean something: it writes the
		// approval into semantic memory, arms a demonstration, or turns a yes into a grant.
		//
		// Without this line an answer is remembered and inert. A user could say "yes, learn
		// that" every session forever and Marco would never once arm a capture, because the
		// pending request it looks for is written here or nowhere.
		if g.last != nil {
			// AND IT REACHES A RUNNER THAT NEVER RAISED IT.
			//
			// `g.last` is the newest runner, and a teach episode runs bounded passes back
			// to back — so the newest runner is routinely a session that STARTED AFTER the
			// question, with a ledger that has never held it. `Respond` then finds nothing
			// and returns, and the yes is recorded and inert: no grant, and no refusal
			// either, because the code that writes refusals never runs.
			//
			// Measured live: the proposal answered `confirmed` against evidence
			// 9f4a56779b4f2389, the judgement eligible with that exact digest, and no
			// authority at all — `step 1 of 2 — trying`, indefinitely.
			//
			// The proposal is in hand here, so hand it over.
			//
			// Deleting the ApplyAnswer fallback must fail
			// TestAYesReachesTheRunnerEvenWhenANewerSessionHasStarted.
			if _, known := g.last.Respond(proposal, resp); !known {
				// THE APPLICATION THE QUESTION WAS ASKED ABOUT, from the session that
				// raised it — never from whichever runner is newest.
				//
				// `g.last` is reassigned the moment a pass STARTS, before its first
				// sample sets the application. A yes arriving in that window was
				// judged against an empty application, found no candidate for the
				// route, and created no authority — and the review.s answer clock
				// then reported the timeout to the person as "Alright — I won.t try
				// it", blaming them for saying yes.
				//
				// Deleting the application argument must fail
				// TestAYesIsJudgedAgainstItsOwnApplication.
				g.last.ApplyAnswer(g.finished[i].Session.Application, p, resp)
			}
		}
		g.finished[i].Hypotheses = g.finished[i].Proposals.Annotate(g.finished[i].Hypotheses)
		// EVERY COPY OF THE QUESTION IS ANSWERED, because it is one question.
		//
		// A proposal's identity is what it is ABOUT, so the same id in three sessions is the
		// same question asked by three passes of one episode — see Runtime.asking. Settling
		// only the copy that happened to be found leaves the others open, and the panel
		// re-offers a question the person has already answered, forever.
		//
		// Deleting this must fail TestAnAnswerSettlesEveryCopyOfTheQuestion.
		for j := range g.finished {
			if j == i {
				continue
			}
			if _, also := g.finished[j].Proposals.Respond(proposal, resp,
				g.finished[j].Stats.SamplesTaken); also {

				g.finished[j].Hypotheses =
					g.finished[j].Proposals.Annotate(g.finished[j].Hypotheses)
			}
		}
		return p, true
	}
	return observe.Proposal{}, false
}

// ProposeScreenName puts the naming question, and makes it visible where questions are read.
//
// Two ledgers, for the same reason Answer writes to two: the RUNNER's ledger is what
// `Runner.NameScreen` looks the question up in, and the finished Result's ledger is what the
// report renders from. A question in only the first is unanswerable because nobody can see it; a
// question in only the second is visible and cannot be answered.
func (g *observationRegistry) ProposeScreenName(application, subject string) (
	observe.Proposal, bool) {

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.memory == nil {
		return observe.Proposal{}, false
	}
	id := observe.ScreenNameProposalIdentity(application, subject)
	// ALREADY ASKED anywhere is already asked. The naming question outlives the session that
	// happened to raise it — a user may play three more times before reading the report — so
	// the search is over every session record, not over the newest one.
	for i := range g.finished {
		for _, p := range g.finished[i].Proposals.Proposals {
			if p.ID == id {
				return p, false
			}
		}
	}
	// It belongs to the newest record for the application it is about, because that is where
	// a reader of that application's report will find it.
	target := -1
	for i := len(g.finished) - 1; i >= 0; i-- {
		if g.finished[i].Session.Application == application {
			target = i
			break
		}
	}
	if target < 0 {
		return observe.Proposal{}, false
	}
	top := g.memory.Topology(application)
	p, asked := g.finished[target].Proposals.ReviewScreenName(
		observe.SubjectRef{Application: application, ID: subject}, top,
		observe.DefaultProposalThresholds())
	return p, asked
}

// AnswerName records a user-supplied screen name against the question that asked for it.
//
// Answering a FINISHED session is the ordinary case here even more than elsewhere: the question
// is raised by the learned-play lifecycle, long after any observation session ended, and the
// durable subject exists independently of both. Requiring the user to keep the screen open while
// naming it would defeat the entire point of durable identity.
func (g *observationRegistry) AnswerName(id observe.SessionID, proposal observe.ProposalID,
	name observe.ScreenName) (observe.Proposal, error) {

	g.mu.Lock()
	defer g.mu.Unlock()

	// The question is found by IDENTITY, across every session record. The session it was
	// raised in is bookkeeping; the screen it is about is the thing that has to survive.
	found := -1
	var q observe.Proposal
	for i := len(g.finished) - 1; i >= 0; i-- {
		if id != "" && g.finished[i].Session.ID != id {
			continue
		}
		if p, ok := g.finished[i].Proposals.NamingQuestion(proposal); ok {
			found, q = i, p
			break
		}
	}
	if found < 0 {
		return observe.Proposal{}, fmt.Errorf(
			"no open question called %s is waiting for a screen name", proposal)
	}
	namer, ok := g.memory.(observe.ScreenNamer)
	if !ok {
		return observe.Proposal{}, fmt.Errorf("this Director cannot remember screen names")
	}
	// THE durable write, and it is bound to the subject the QUESTION named — never to what
	// is in front of anybody now, and never to the application that happens to be current.
	// By the time somebody types a name they have usually moved on, often to another program.
	//
	// Deleting this call must fail TestNamingAScreenBindsToTheSubjectThatWasAskedAbout.
	if err := namer.NameSubject(q.Screen.Application, q.Screen.ID, name); err != nil {
		// The question stays OPEN. A refused name is not an answered question, and closing
		// it would take away the only prompt that lets the user try again.
		return observe.Proposal{}, err
	}
	settled, _ := g.finished[found].Proposals.SettleName(proposal, name,
		g.finished[found].Stats.SamplesTaken)
	return settled, nil
}

// List summarises every session this service still remembers.
func (g *observationRegistry) List() []observationView {
	g.mu.RLock()
	runner := g.active
	finished := make([]observesession.Result, len(g.finished))
	copy(finished, g.finished)
	g.mu.RUnlock()

	var out []observationView
	if runner != nil {
		session, stats := runner.Snapshot()
		out = append(out, viewOf(session, stats, nil, nil, nil, true))
	}
	for i := len(finished) - 1; i >= 0; i-- {
		r := finished[i]
		out = append(out, viewOf(r.Session, r.Stats, nil, nil, nil, false))
	}
	return out
}

// observationView is the safe outward shape of a session.
//
// No handles, no labels, no titles. A status line is read by whoever is looking at the
// terminal, which during a game session is not necessarily only the player.
type observationView struct {
	ID          observe.SessionID    `json:"id"`
	State       observe.State        `json:"state"`
	Selector    string               `json:"selector"`
	Application string               `json:"application,omitempty"`
	Generations []uint64             `json:"generations,omitempty"`
	StartedAt   time.Time            `json:"started_at"`
	Elapsed     time.Duration        `json:"elapsed"`
	Duration    time.Duration        `json:"duration"`
	Interval    time.Duration        `json:"interval"`
	Samples     int                  `json:"samples"`
	Skipped     int                  `json:"skipped"`
	Late        int                  `json:"late,omitempty"`
	LabelPasses int                  `json:"label_passes,omitempty"`
	Active      bool                 `json:"active"`
	Complete    bool                 `json:"complete"`
	Reason      string               `json:"reason,omitempty"`
	Findings    *observe.Findings    `json:"findings,omitempty"`
	Insights    []observe.Insight    `json:"insights,omitempty"`
	Hypotheses  []observe.Hypothesis `json:"hypotheses,omitempty"`
	Proposals   []observe.Proposal   `json:"proposals,omitempty"`
	// Answered echoes the proposal an answer was just recorded against, so a caller can
	// see WHICH question it landed on rather than trusting that it landed on the right one.
	Answered *observe.Proposal `json:"answered,omitempty"`
	// MemoryUnavailable explains why cross-session recognition could not run, empty when it
	// could. Reported so an absence of recognition is never mistaken for novelty.
	MemoryUnavailable string `json:"memory_unavailable,omitempty"`
	// Relationships is what this session's transitions did to the durable topology.
	//
	// Counts, not a list of edges: an unchanged edge is not news, and a report that printed
	// every one of them each session would bury the one that was new.
	Relationships observe.RelationshipReport `json:"relationships,omitzero"`
	// Topology is the durable edges whose endpoints are BOTH subjects this session saw, each
	// already rendered for a person. Bounded — see renderTopology.
	Topology []topologyView `json:"topology,omitempty"`
	// Learning is every remembered edge judged against the invitation policy, already
	// rendered. Bounded — see learningViews.
	Learning []learningView `json:"learning,omitempty"`
	// Demonstration is the bounded capture this session watched, already rendered. Present
	// whether it completed or not: an incomplete one's reason is the useful part.
	Demonstration []string `json:"demonstration,omitempty"`
	// Capturing is the live view while a demonstration is in progress.
	Capturing *observe.CaptureView `json:"capturing,omitempty"`
	// Assessment is what Marco concludes from the demonstration, recomputed against what it
	// remembers now — never a stored verdict.
	Assessment []string `json:"assessment,omitempty"`
	// FollowUps is every demonstrated route judged for whether another example would help,
	// already rendered — including the closed reasons when Marco decided not to ask.
	FollowUps [][]string `json:"follow_ups,omitempty"`
	// Rehearsals is every demonstrated route judged for whether Marco may ask to TRY it,
	// already rendered — including the closed reasons when it may not.
	Rehearsals [][]string `json:"rehearsals,omitempty"`
	// Authorization is the outstanding permission, if the user gave one.
	//
	// A DESCRIPTION of it, never the grant: a view is serialised to JSON and handed around,
	// and authority that can be marshalled is authority that can be replayed.
	Authorization []string             `json:"authorization,omitempty"`
	Stats         observesession.Stats `json:"stats"`
}

func viewOf(s observe.Session, stats observesession.Stats,
	f *observe.Findings, insights []observe.Insight,
	hypotheses []observe.Hypothesis, active bool) observationView {

	elapsed := stats.Elapsed
	if active {
		elapsed = s.Elapsed(time.Now())
	}
	return observationView{
		ID: s.ID, State: s.State, Selector: s.Selector.Describe(),
		Application: s.Application, Generations: s.Generations,
		StartedAt: s.StartedAt, Elapsed: elapsed,
		Duration: s.Bounds.Duration, Interval: s.Bounds.Interval,
		Samples: stats.SamplesTaken, Skipped: stats.SamplesSkipped,
		Late: stats.SamplesLate, LabelPasses: stats.LabelPasses,
		Active: active, Complete: s.State.Succeeded(), Reason: s.Reason,
		Findings: f, Insights: insights, Hypotheses: hypotheses, Stats: stats,
	}
}

// ── the service surface ───────────────────────────────────────────────────────

// StartObservation begins a bounded passive session and returns at once.
func (r *Runtime) StartObservation(p service.ObservePayload) (service.ObserveStarted, error) {
	if r.observations == nil {
		return service.ObserveStarted{}, fmt.Errorf("this Director has no observation registry")
	}
	bounds := observe.DefaultBounds()
	if p.Duration != 0 {
		bounds.Duration = p.Duration
	}
	if p.Interval != 0 {
		bounds.Interval = p.Interval
	}
	normalised, err := bounds.Normalise()
	if err != nil {
		return service.ObserveStarted{}, err
	}
	// NOBODY SAID WHICH WINDOW. Marco chooses one, rather than making a person run
	// `director windows`, read a table of ephemeral ids and type one back — which is the
	// product asking somebody to do the Director's job.
	//
	// Resolved ONCE, here, and immediately pinned to an ephemeral id. The session that follows
	// is bound to that one window of that one process for its whole life: focus stops being an
	// input the moment this line returns, which is the stale-capture invariant intact.
	target := p.Target
	if target.Zero() {
		chosen, err := r.currentContext(context.Background())
		if err != nil {
			return service.ObserveStarted{}, err
		}
		target = chosen
	}
	id, err := r.observations.Start(
		r.newObservationTarget(), r.newObservationSampler(sessionClock),
		observesession.NopEvents{}, target, normalised)
	if err != nil {
		return service.ObserveStarted{}, err
	}
	return service.ObserveStarted{
		ID: string(id), Target: target.Describe(),
		Duration: normalised.Duration, Interval: normalised.Interval,
		State: string(observe.Observing),
	}, nil
}

// Observation reads, lists or cancels sessions.
func (r *Runtime) Observation(q service.ObserveQuery) (any, error) {
	if r.observations == nil {
		return nil, fmt.Errorf("this Director has no observation registry")
	}
	switch {
	case q.Point != nil:
		// THE pointing call site, and the only one. Reads; writes nothing.
		return r.PointAt(context.Background(), *q.Point)
	case q.Reach != nil:
		// THE goal-planning call site, and the only one. A read over remembered goals and
		// the durable topology; it grants nothing and can reach nothing that acts.
		return r.Reach(*q.Reach)
	case q.Perform != nil:
		// THE learned-execution call site, and the only one. Unlike every other branch
		// here it ACTS — under an authority minted from this explicit request, bounded
		// exactly as a rehearsal.s is, through the one walker that verifies every step.
		return r.PerformGoal(*q.Perform)
	case q.Teach != nil:
		// THE teach call site, and the only one. It starts bounded observation passes
		// through this registry and creates no authority of its own — see
		// internal/director/teach.
		return r.Teaching(context.Background(), *q.Teach)
	case q.Learn != nil:
		// THE control-surface call site. Every verb on it turns into a request one of the
		// cases around it already serves; it creates no authority and owns no lifecycle.
		return r.Learn(context.Background(), *q.Learn)
	case q.Rehearse != nil:
		// THE rehearsal trigger, and the only one. Not a session, not a review, not a
		// timer: a grant is spent because somebody asked for it to be, in this request.
		return r.Rehearse(*q.Rehearse)
	case q.Name != nil:
		// THE conversion point, and the only one on this path: raw human text becomes a
		// ScreenName here, at the boundary where a person's typing arrives, and nowhere
		// deeper. Everything downstream carries the type, so a caller cannot substitute an
		// OCR line, an accessibility label or a window title by assigning it to the right
		// variable — see [[ADR-031-the-user-names-the-stage]].
		name, err := observe.UserSuppliedScreenName(q.Name.Name)
		if err != nil {
			return nil, err
		}
		p, err := r.observations.AnswerName(observe.SessionID(q.ID),
			observe.ProposalID(q.Name.ProposalID), name)
		if err != nil {
			return nil, err
		}
		view, _ := r.observations.Snapshot(observe.SessionID(q.ID))
		view.Answered = &p
		return view, nil
	case q.Knows != nil:
		// THE durable-knowledge call site, and the only one. It reaches a judgement by the
		// SUBJECT it is about rather than through a live question, because a subject nothing
		// can recognise any more has no question and never will — and its answer would
		// otherwise be uncorrectable forever. It creates no authority.
		k := *q.Knows
		if k.Subject == "" {
			return knowledgeView{Known: r.observations.Known()}, nil
		}
		kind := observe.HypothesisKind(k.Kind)
		var err error
		if k.Withdraw {
			err = r.observations.RetractKnown(k.Subject, kind)
		} else {
			resp := observe.UserResponse(k.Response)
			if !resp.Known() {
				return nil, fmt.Errorf(
					"%q is not an answer; say confirmed, contradicted or declined", k.Response)
			}
			err = r.observations.ReviseKnown(k.Subject, kind, resp)
		}
		if err != nil {
			return nil, err
		}
		// The list is re-read rather than patched, so what comes back is what is stored —
		// a surface that trusted its own edit would drift from the file it claims to show.
		return knowledgeView{Known: r.observations.Known()}, nil
	case q.Revise != nil:
		// THE revision call site. Explicit, and separate from Answer on purpose — see
		// ObserveRevise. It creates no authority and touches no perception.
		var p observe.Proposal
		var ok bool
		if q.Revise.Withdraw {
			p, ok = r.observations.Withdraw(observe.SessionID(q.ID),
				observe.ProposalID(q.Revise.ProposalID))
		} else {
			resp := observe.UserResponse(q.Revise.Response)
			if !resp.Known() {
				return nil, fmt.Errorf(
					"%q is not an answer; say confirmed, contradicted or declined",
					q.Revise.Response)
			}
			p, ok = r.observations.Revise(observe.SessionID(q.ID),
				observe.ProposalID(q.Revise.ProposalID), resp)
		}
		if !ok {
			return nil, fmt.Errorf(
				"nothing called %s has an answer to change; a question that was never "+
					"answered is answered, not revised", q.Revise.ProposalID)
		}
		view, _ := r.observations.Snapshot(observe.SessionID(q.ID))
		view.Answered = &p
		return view, nil
	case q.Answer != nil:
		// THE response-consumption call site. An unrecognised answer is refused rather
		// than coerced: the only defaults available are "yes" and "no", and both would be
		// putting words in the user's mouth.
		resp := observe.UserResponse(q.Answer.Response)
		if !resp.Known() {
			return nil, fmt.Errorf(
				"%q is not an answer; say confirmed, contradicted or declined",
				q.Answer.Response)
		}
		p, ok := r.observations.Answer(observe.SessionID(q.ID),
			observe.ProposalID(q.Answer.ProposalID), resp)
		if !ok {
			return nil, fmt.Errorf(
				"no open question called %s (it may already have been answered)",
				q.Answer.ProposalID)
		}
		view, _ := r.observations.Snapshot(observe.SessionID(q.ID))
		view.Answered = &p
		return view, nil
	case q.Cancel:
		if err := r.observations.Cancel(observe.SessionID(q.ID)); err != nil {
			return nil, err
		}
		view, _ := r.observations.Snapshot(observe.SessionID(q.ID))
		return view, nil
	case q.List:
		return r.observations.List(), nil
	default:
		view, ok := r.observations.Snapshot(observe.SessionID(q.ID))
		if !ok {
			if q.ID == "" {
				return nil, fmt.Errorf("no observation session has been run yet")
			}
			return nil, fmt.Errorf("no observation session called %s", q.ID)
		}
		if !q.Insights {
			view.Findings, view.Insights = nil, nil
		}
		return view, nil
	}
}

// refuseWhileObserved blocks Director actions against a window under passive observation.
//
// A passive session exists to describe how a PERSON uses an application. An action the
// Director took is not evidence about that, and evidence that silently mixes the two is
// worse than none — a later reader has no way to tell which transitions were the player's.
//
// Refusal rather than automatic pause-and-resume: pausing would leave a gap in the timeline
// that looks like a quiet moment, and resuming would stitch two halves together as though
// nothing had happened in between.
func (r *Runtime) refuseWhileObserved() (execute.Outcome, bool) {
	if r.observations == nil {
		return execute.Outcome{}, false
	}
	id := r.observations.ActiveID()
	if id == "" {
		return execute.Outcome{}, false
	}
	app := r.observations.ObservedApplication()
	if app == "" {
		app = "an application"
	}
	return execute.Outcome{
		Status:  directorapi.ResultBlocked,
		Request: "",
		Message: fmt.Sprintf(
			"%s is under passive observation by session %s. Director actions are refused "+
				"while it is being watched, because an action the Director took is not "+
				"evidence about how you play. Stop it first: director cancel-observation %s",
			app, id, id),
	}, true
}

// ── live findings ─────────────────────────────────────────────────────────────

// LiveAnalysis reports what a passive session has learned so far.
//
// Reads the analysis the session already accumulated. It starts nothing, samples nothing
// and attaches no provider, so a HUD may poll it at whatever rate it likes without becoming
// a participant in the thing it is describing.
func (r *Runtime) LiveAnalysis(p service.LiveAnalysisPayload) service.LiveAnalysisResponse {
	out := service.LiveAnalysisResponse{ServiceGeneration: r.epoch}
	if r.observations == nil {
		return out
	}
	analysis, active, ok := r.observations.analysis(observe.SessionID(p.SessionID))
	if !ok {
		// No session has ever run. Distinct from a session that found nothing, and a
		// client showing empty sections for the first would be inventing a result.
		return out
	}
	out.Available = true
	out.Active = active
	out.Summary = observe.Summarise(analysis, observe.DefaultSummaryBounds(), time.Now())
	out.Summary.Active = active
	return out
}

// ObservationEvents returns a session's live findings after a cursor.
//
// Gap is decided HERE, where both the client's cursor and what is still retained are known.
// Leaving that arithmetic to each client is how two front-ends come to disagree about
// whether anything was missed.
func (r *Runtime) ObservationEvents(
	p service.ObservationEventsPayload) service.ObservationEventsResponse {

	out := service.ObservationEventsResponse{ServiceGeneration: r.epoch}
	if r.observations == nil {
		return out
	}
	runner, active := r.observations.live(observe.SessionID(p.SessionID))
	if runner == nil {
		return out
	}
	session, _ := runner.Snapshot()
	events, newest, oldest := runner.LiveEvents(p.After, p.Limit)

	out.Available = true
	out.Active = active
	out.SessionID = session.ID
	out.Events = events
	out.Newest, out.Oldest = newest, oldest
	// A cursor that fell behind what is retained means events were lost. Only
	// meaningful once the log has rolled at all — oldest is 1 until then.
	out.Gap = p.After > 0 && oldest > p.After+1
	return out
}

// live returns the runner for a session id, and whether it is the ACTIVE one.
//
// Only the active session has a live event recorder: a finished one's recorder retired with
// it. Its EVIDENCE survives in its Result — see analysis.
func (g *observationRegistry) live(id observe.SessionID) (*observesession.Runner, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.active != nil && (id == "" || id == g.activeID) {
		return g.active, true
	}
	return nil, false
}

// analysis resolves which session's findings are current.
//
// # Ordering rule
//
//  1. the ACTIVE session, if one is running (or if its id was asked for by name);
//  2. otherwise the MOST RECENT retained terminal session — newest last in the bounded
//     finished list;
//  3. otherwise nothing has ever run.
//
// An active session always wins over an older completed one, because "what is happening
// now" is what a live panel is for. Completion freezes the evidence rather than erasing it:
// a finished session's Findings and Insights are exactly what its Result carries, so
// nothing is recomputed and no second cache exists.
//
// The second return says whether the analysis came from a running session. A terminal one
// is still a valid source and must not be presented as live.
func (g *observationRegistry) analysis(id observe.SessionID) (observe.LiveAnalysis, bool, bool) {
	if runner, active := g.live(id); runner != nil {
		return runner.LiveAnalysis(observe.DefaultInsightThresholds()), active, true
	}

	g.mu.RLock()
	finished := make([]observesession.Result, len(g.finished))
	copy(finished, g.finished)
	g.mu.RUnlock()

	// Newest first. The list is append-ordered and bounded, so the last entry is the
	// most recent session this service still remembers.
	for i := len(finished) - 1; i >= 0; i-- {
		res := finished[i]
		if id != "" && res.Session.ID != id {
			continue
		}
		return observe.LiveAnalysis{
			SessionID: res.Session.ID,
			State:     res.Session.State,
			Samples:   res.Stats.SamplesTaken,
			StartedAt: res.Session.StartedAt,
			EndedAt:   res.Session.EndedAt,
			Reason:    res.Session.Reason,
			Findings:  res.Findings,
			Insights:  res.Insights,
			// No cursor: the live recorder retired with the session, so there is no
			// event stream to resume. Reporting a stale cursor would invite a client
			// to poll a feed that no longer exists.
		}, false, true
	}
	return observe.LiveAnalysis{}, false, false
}

// topologyView is one durable edge, already explained.
//
// Rendered here rather than handed over as a record, because the explanation is the point: a
// person cannot compare two twelve-character digests, and asking them to is asking them to do
// the system's job. See observe.DescribeRelationship.
type topologyView struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Lines []string `json:"lines"`
}

// MaxTopologyReported bounds how many edges one session report prints.
//
// A session that touched forty edges has nothing useful to say about any of them in a report a
// person reads. The store keeps them all; this shows the ones with the most corroboration.
const MaxTopologyReported = 8

// topologyFor renders the durable edges this session had something to do with.
//
// Only edges between subjects the session RESOLVED — the whole remembered topology of an
// application could be hundreds of edges, and a session report is about this session.
func (g *observationRegistry) topologyFor(r observesession.Result) []topologyView {
	// Asked of the store through a narrow interface, so a Director whose memory is a
	// different implementation — or absent — reports nothing rather than failing.
	type topology interface {
		Relationships() []observe.RememberedRelationship
		Subject(id string) (observe.RememberedSubject, bool)
	}
	store, ok := g.memory.(topology)
	if !ok || r.Relationships.Durable == 0 {
		return nil
	}
	rels := store.Relationships()
	sort.Slice(rels, func(i, j int) bool {
		if rels[i].Sessions != rels[j].Sessions {
			return rels[i].Sessions > rels[j].Sessions
		}
		return rels[i].Observations > rels[j].Observations
	})
	out := make([]topologyView, 0, MaxTopologyReported)
	for _, rel := range rels {
		if !strings.EqualFold(rel.Application, r.Session.Application) {
			continue
		}
		from, _ := store.Subject(rel.From)
		to, _ := store.Subject(rel.To)
		out = append(out, topologyView{
			From: rel.From, To: rel.To,
			Lines: observe.DescribeRelationship(rel, from, to),
		})
		if len(out) >= MaxTopologyReported {
			break
		}
	}
	return out
}

// learningView is one judged relationship, already explained.
type learningView struct {
	Eligible bool     `json:"eligible"`
	Question string   `json:"question,omitempty"`
	Lines    []string `json:"lines"`
	// Refusals is the CLOSED vocabulary, carried beside the rendered lines.
	//
	// The lines are a developer-facing account and they name durable subject ids; anything
	// that has to reach a person reads these instead. Without them the visibility layer had
	// nothing to translate and forwarded a rendered line, which put `subj_ad7cea89aecd` on
	// a Watch panel.
	Refusals []observe.LearningRefusal `json:"refusals,omitempty"`
}

// MaxLearningReported bounds how many judgements one session report prints.
//
// The ELIGIBLE ones first and always: an invitation Marco decided to put is the news, and the
// twenty edges it decided not to are the explanation for why there is only one.
const MaxLearningReported = 6

func (g *observationRegistry) learningViews(r observesession.Result) []learningView {
	if len(r.Learning) == 0 {
		return nil
	}
	type topology interface {
		Subject(id string) (observe.RememberedSubject, bool)
	}
	store, _ := g.memory.(topology)
	subject := func(id string) observe.RememberedSubject {
		if store == nil {
			return observe.RememberedSubject{}
		}
		s, _ := store.Subject(id)
		return s
	}

	ranked := append([]observe.LearningAssessment{}, r.Learning...)
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Eligible != ranked[j].Eligible {
			return ranked[i].Eligible
		}
		return ranked[i].Sessions > ranked[j].Sessions
	})
	out := make([]learningView, 0, MaxLearningReported)
	for _, a := range ranked {
		v := learningView{Eligible: a.Eligible, Refusals: a.Refusals,
			Lines: observe.DescribeLearningAssessment(
				a, subject(a.Relationship.From), subject(a.Relationship.To))}
		if a.Eligible {
			v.Question = a.Question
		}
		out = append(out, v)
		if len(out) >= MaxLearningReported {
			break
		}
	}
	return out
}

// Revise replaces a settled answer; Withdraw takes one back.
//
// The same two-ledger dance `Answer` does, and for the same reason: the RUNNER's ledger is what
// makes an answer mean something durably, and the finished Result's ledger is what every report
// renders from. A revision in only the first is invisible; one in only the second changes nothing.
//
// Deleting the runner call must fail TestARevisionThroughTheProductionPathSurvivesARestart.
func (g *observationRegistry) Revise(id observe.SessionID, proposal observe.ProposalID,
	resp observe.UserResponse) (observe.Proposal, bool) {

	return g.reviseBoth(id, proposal,
		func(r *observesession.Runner) (observe.Proposal, bool) { return r.Revise(proposal, resp) },
		func(l *observe.ProposalLedger, n int) (observe.Proposal, bool) {
			return l.Revise(proposal, resp, n)
		})
}

// Withdraw takes back a previous answer, leaving no active judgement.
func (g *observationRegistry) Withdraw(id observe.SessionID, proposal observe.ProposalID) (
	observe.Proposal, bool) {

	return g.reviseBoth(id, proposal,
		func(r *observesession.Runner) (observe.Proposal, bool) { return r.Retract(proposal) },
		func(l *observe.ProposalLedger, n int) (observe.Proposal, bool) {
			return l.Retract(proposal, n)
		})
}

func (g *observationRegistry) reviseBoth(id observe.SessionID, proposal observe.ProposalID,
	live func(*observesession.Runner) (observe.Proposal, bool),
	record func(*observe.ProposalLedger, int) (observe.Proposal, bool)) (observe.Proposal, bool) {

	g.mu.Lock()
	defer g.mu.Unlock()

	// The question is found by IDENTITY, across every session record. Which session raised it
	// is bookkeeping; the proposition it is about is the thing that has to survive — and by
	// the time somebody changes their mind they have usually restarted twice.
	var out observe.Proposal
	var ok bool
	for i := len(g.finished) - 1; i >= 0; i-- {
		if id != "" && g.finished[i].Session.ID != id {
			continue
		}
		if p, done := record(&g.finished[i].Proposals, g.finished[i].Stats.SamplesTaken); done {
			out, ok = p, true
			g.finished[i].Hypotheses = g.finished[i].Proposals.Annotate(g.finished[i].Hypotheses)
			break
		}
	}
	// THE durable write. Without this a revision is remembered for as long as the service
	// happens to stay up and is undone by the next restart.
	if g.last != nil {
		if p, done := live(g.last); done {
			out, ok = p, true
		}
	}
	return out, ok
}

// ── what a person has told Marco ──────────────────────────────────────────────

// Known is the canonical list of intentional semantic judgements.
//
// Two halves, and they answer different questions. `observe.WhatIsKnown` decides WHICH stored
// records count as something a person said — through the canonical effective reading, so a
// withdrawn answer stops appearing as a truth. This method supplies the other half: which of those
// subjects any recent session actually RECOGNISED, which is the only honest source for "can Marco
// point at this right now".
//
// Locatability deliberately does not come from stored geometry. Stored geometry is exactly what
// made the record this milestone exists because of unfindable, and a surface that consulted it
// would confidently offer to point at something nothing can see.
func (g *observationRegistry) Known() []observe.KnownJudgement {
	store, ok := g.knowledgeStore()
	if !ok {
		return nil
	}
	g.mu.Lock()
	recognised := map[string]bool{}
	for _, r := range g.finished {
		for _, p := range r.Proposals.Proposals {
			if p.Recognised && p.RecognisedAs != "" {
				recognised[p.RecognisedAs] = true
			}
		}
	}
	g.mu.Unlock()
	return observe.WhatIsKnown(store.Subjects(), recognised)
}

// ReviseKnown changes a durable judgement; RetractKnown withdraws one.
//
// Both go through the canonical verbs in `observe`, which enforce the same rule the ledger does —
// only a settled answer may be revised — and write through the same `Remember` a session answer
// writes through. Nothing here touches perception: the subject, its structure, its visits, its
// name and its other interpretations are untouched.
//
// Deleting either call must fail TestTheExplorerJudgementIsCorrectableWithoutATerminal.
func (g *observationRegistry) ReviseKnown(subject string, kind observe.HypothesisKind,
	resp observe.UserResponse) error {

	store, ok := g.knowledgeStore()
	if !ok {
		return fmt.Errorf("there is no durable memory to change")
	}
	return observe.ReviseKnown(store, subject, kind, resp)
}

// RetractKnown withdraws a durable judgement.
func (g *observationRegistry) RetractKnown(subject string, kind observe.HypothesisKind) error {
	store, ok := g.knowledgeStore()
	if !ok {
		return fmt.Errorf("there is no durable memory to change")
	}
	return observe.RetractKnown(store, subject, kind)
}

// knowledgeStore is the durable store, when the one in use can be read by subject.
//
// A narrowing rather than a widening of Memory: nothing reached through it can record a
// relationship, arm a demonstration or create authority.
func (g *observationRegistry) knowledgeStore() (observe.KnowledgeStore, bool) {
	g.mu.Lock()
	m := g.memory
	g.mu.Unlock()
	store, ok := m.(observe.KnowledgeStore)
	return store, ok && store != nil
}

// knowledgeView is what a surface receives when it asks what a person has told Marco.
//
// A view type of its own rather than a field on the observation view, because it is not about a
// session: these judgements outlive every session that produced them, and hanging them off one
// would make them look like this run's findings.
type knowledgeView struct {
	Known []observe.KnownJudgement `json:"known"`
}

// routesBeingAskedAbout is every route a finished session still has an OPEN rehearsal question for.
//
// Open, not merely present: an answered question is settled, and a route whose question was
// answered is free to be asked about again if the evidence ever changes. What must not happen is a
// second copy of a question somebody is looking at right now.
//
// See observesession.Config.AlreadyAsking for the failure this feeds.
func (g *observationRegistry) routesBeingAskedAbout() []observe.RelationshipRef {
	if g == nil {
		return nil
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []observe.RelationshipRef
	seen := map[observe.RelationshipRef]bool{}
	for i := range g.finished {
		for _, p := range g.finished[i].Proposals.Proposals {
			if p.Ask != observe.AskRehearse || p.Response != observe.ResponseNone {
				continue
			}
			if p.Relationship == nil || seen[*p.Relationship] {
				continue
			}
			seen[*p.Relationship] = true
			out = append(out, *p.Relationship)
		}
	}
	return out
}
