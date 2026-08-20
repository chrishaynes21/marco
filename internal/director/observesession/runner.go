// Package observesession runs a passive observation session against a live target.
//
// The pure core in internal/director/observe knows how to ANALYSE a timeline and nothing
// about where samples come from. This package supplies the missing half — time, a target, a
// way to take one sample, somewhere to send events — and owns exactly one hard problem:
// deciding when a sample may be taken at all.
//
// # It orchestrates; it does not perceive
//
// Every dependency arrives as a narrow interface this package cannot construct. There is no
// capture code here, no provider, no fusion, no platform call. That is what lets the
// boundary test hold for the runner as well as for the core: a package that receives a
// Sampler can only take samples, and a package that could BUILD one could eventually build
// something else.
//
// # A sample exists only when ownership is proven
//
// Before every frame the target is revalidated, and a failure produces no sample rather than
// a sample with a caveat. The reason is the stale-capture incident: real pixels from the
// wrong window are worse than no pixels, because they are indistinguishable from evidence
// until somebody notices the game has been closed for ten minutes.
package observesession

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// Clock is time, injected so the scheduler can be tested without sleeping.
//
// Correctness tests that wait on wall-clock time are slow, flaky, and prove less than they
// appear to; a fake clock makes "what happens when a cycle overruns its interval" an
// ordinary assertion.
type Clock interface {
	Now() time.Time
	// After fires once after d. The scheduler waits on this and on cancellation.
	After(d time.Duration) <-chan time.Time
}

// Target resolves and revalidates the selected window.
//
// One method, called before every sample. Whatever it returns must have been checked
// against the live desktop at the moment of the call — this package trusts it completely
// and has no way to check its work, which is why the implementation lives next to the
// window tracker that does the checking.
type Target interface {
	Acquire(ctx context.Context, s windowref.Selector) (windowref.Ref, error)
}

// SampleRequest tells the sampler what to do for one frame.
type SampleRequest struct {
	// Window is the reference just validated. A sampler must capture THIS window.
	Window windowref.Ref
	// Sequence is the sample number, from 1.
	Sequence int
	// ReadLabels asks for the expensive scoped-OCR pass. False on most samples: label
	// reading measured ~230ms per control live, and re-reading unchanged text every
	// frame would spend the whole budget learning nothing.
	ReadLabels bool
}

// Sampler turns a validated window into one safe semantic snapshot.
//
// The entire perception chain — capture, providers, fusion, safe conversion — sits behind
// this. Deliberately: the runner must not be able to look at a pixel, and a sampler that
// returned raw frames would make that impossible to guarantee.
type Sampler interface {
	Sample(ctx context.Context, req SampleRequest) (observe.Sample, error)
}

// Events receives lifecycle notifications.
//
// Fire-and-forget. A slow or broken listener must never stall a session, so the runner
// never blocks on delivery and never inspects a return value.
type Events interface {
	Publish(Event)
}

// NopEvents discards everything.
type NopEvents struct{}

func (NopEvents) Publish(Event) {}

// EventKind is the closed vocabulary of session events.
type EventKind string

const (
	SessionStarted    EventKind = "observation_session_started"
	SampleCompleted   EventKind = "observation_sample_completed"
	SampleSkipped     EventKind = "observation_sample_skipped"
	TargetUnavailable EventKind = "observation_target_unavailable"
	TargetReacquired  EventKind = "observation_target_reacquired"
	SessionCancelled  EventKind = "observation_session_cancelled"
	SessionCompleted  EventKind = "observation_session_completed"
	SessionFailed     EventKind = "observation_session_failed"
)

// Event is one safe lifecycle notification.
//
// Closed fields, no arbitrary map. A generic payload bag is how private text escapes: it
// carries whatever somebody put in it, and nothing type-checks what that was.
type Event struct {
	Kind       EventKind         `json:"kind"`
	SessionID  observe.SessionID `json:"session_id"`
	At         time.Time         `json:"at"`
	Sequence   int               `json:"sequence,omitempty"`
	Generation uint64            `json:"generation,omitempty"`
	State      observe.State     `json:"state,omitempty"`
	// Reason is a safe sentence, never a label or a window title.
	Reason string `json:"reason,omitempty"`
}

// Config is everything one session needs.
type Config struct {
	ID       observe.SessionID
	Selector windowref.Selector
	Bounds   observe.Bounds
	// Thresholds and Insights tune the pure analysis.
	Thresholds observe.Thresholds
	Insights   observe.InsightThresholds
	// Hypotheses tunes the discovery-side interpretation.
	Hypotheses observe.HypothesisThresholds
	// ProposalPolicy bounds when Marco may interrupt with a question.
	ProposalPolicy observe.ProposalThresholds
	// LabelEvery is how many samples pass between scoped-OCR passes while the scene is
	// stable. 1 reads every frame, which the measured cost makes unwise.
	LabelEvery int
	// MaxLabelPasses caps scoped OCR for the whole session.
	MaxLabelPasses int
	// Episode is what the CALLER declares about why this session is running.
	Episode
}

// Episode is what one caller declares about the session it asked for.
//
// Both fields are set by LEARN and by nothing else, and they are together in one type so that
// stays visible: they are the two consequences of "a person explicitly asked to be watched doing
// something", and a caller that wanted one without the other would be claiming to be a learn
// session without being one.
type Episode struct {
	// SameEpisode says this session belongs to an episode whose corroboration has already
	// been counted, so its transitions fold their evidence and claim no further independent
	// sighting.
	//
	// A learn attempt runs several bounded passes back to back in one sitting; counting each
	// of them as an independent session would let one explicit learn satisfy a threshold that
	// exists to mean real-world recurrence.
	SameEpisode bool
	// EstablishPlaces licenses this session to make the place the user is standing on durably
	// recognisable — its IDENTITY, and no judgement about what it means.
	//
	// # Why an explicit learn may do this and passive observation may not
	//
	// Because `learn "…"` IS the human semantic event. Until now a durable subject appeared
	// only when somebody answered a question Marco had invented, so Learn could not begin
	// until the user had happened to settle an incidental "is this a menu?" — and which
	// question Marco raised was not theirs to choose. Observed live: the same application in
	// two sessions asked about the screen once and about a group inside it once, and only the
	// first would have unblocked Learn.
	//
	// It persists ZERO semantic judgements. The subject carries an empty interpretation list,
	// which every reader already handles: `Effective()` is `none`, `RecalledValidation`
	// returns nothing, and the edge report says "recognised, nothing known about what it is".
	//
	// It is bounded: at most ONE place per pass, only where the signature could ever be
	// matched again, only where memory does not already recognise it, and refused at the
	// store's existing subject bound. See observe.PlaceToEstablish.
	EstablishPlaces bool
	// PermissionExpected says the person is WAITING to be asked whether Marco may try it.
	//
	// # Why an explicit learn may have this slot and passive observation may not
	//
	// The interruption budget is one open question, and it is right: passive observation is
	// Marco interrupting somebody who is busy, and a queue of questions is a queue of
	// interruptions. Question kinds are reviewed understanding-first and permission-LAST, so
	// an incidental "is this a menu?" takes the slot and the rehearsal question is refused
	// with `another_question_open`.
	//
	// Under Learn that reasoning inverts. Somebody typed what they wanted, pressed Start,
	// demonstrated it and is sitting in front of the panel waiting to be asked — the question
	// is not an interruption, it is the thing they asked for. Observed live three runs
	// running: "I think I got it. Want me to try?" with no question behind it and no way
	// forward, while the one open slot held a question about a group nobody had asked about.
	//
	// It buys exactly ONE extra slot, and recency decides who gets it — so the route just
	// demonstrated is asked about and every other route is still bounded. MaxProposals is
	// untouched, so the ledger is still bounded too, and nothing here authorises anything: a
	// rehearsal still happens only when the person says yes.
	PermissionExpected bool
}

// Sensible defaults for the expensive parts.
const (
	DefaultLabelEvery     = 6
	DefaultMaxLabelPasses = 60
	// MaxConsecutiveFailures ends a session whose sampler keeps failing. Without it a
	// broken provider would produce a full-length session of nothing.
	MaxConsecutiveFailures = 10
)

// Result is a finished session.
type Result struct {
	Session  observe.Session
	Findings observe.Findings
	Insights []observe.Insight
	// Hypotheses are the cautious interpretations of the DISCOVERY evidence — recurring
	// screens, their structures, and the navigation observed between them.
	//
	// Held apart from Insights, which read the authoritative entity timeline. Two
	// generators over two bodies of evidence: one asks what the fused world contained, this
	// one asks what the screens the player moved between might be. Merging them would put a
	// shadow-derived guess in a list a reader takes as belief-adjacent.
	Hypotheses []observe.Hypothesis
	// Places is what this session did about making where the user is standing durably
	// recognisable, and the CLOSED reason when it did nothing.
	//
	// Reported on every session, licensed or not. A learn attempt that cannot establish a start
	// has to be able to say which of the half-dozen reasons applied, and "not_licensed" on an
	// ordinary session is the sentence that says passive observation still persists nothing.
	Places observe.PlaceEstablishment
	// Relationships is what this session's transitions did to the durable topology: how many
	// had both endpoints recognised, how many stayed session-local, and what the store made
	// of the rest.
	//
	// Counts rather than a list of edges. An unchanged edge is not news, and a report that
	// printed every one of them each session would bury the one that was new.
	Relationships observe.RelationshipReport
	// Learning is every remembered relationship, judged against the invitation policy —
	// eligible or not, and the CLOSED reasons why not.
	//
	// Carried because silence is the hard case. "Marco did not offer to learn anything" has a
	// dozen explanations — too few sessions, no navigation evidence, all of it
	// context-admitted, already declined, another question open — and without this a reader
	// cannot tell which, nor whether the policy is working.
	Learning []observe.LearningAssessment
	// Assessment is what Marco concludes from the demonstration, judged against what it
	// remembers NOW. Nil when there was no demonstration.
	//
	// Derived rather than stored: the same observation becomes more verifiable as memory
	// improves, and a verdict written into the record would freeze a judgement made when
	// Marco knew less. See [[ADR-021-a-judgement-is-recomputed-not-recorded]].
	Assessment *observe.CandidateAssessment
	// FollowUps is every route with a demonstration, judged for whether another example
	// would help — and the CLOSED reasons when Marco decided not to ask.
	//
	// Silence is the hard case: a user who agreed to show Marco something and then heard
	// nothing cannot otherwise tell whether Marco is satisfied, stuck, or broken.
	FollowUps []observe.FollowUpReport
	// Rehearsals is every demonstrated route judged for whether Marco may ASK to try it —
	// and the closed reasons when it may not.
	Rehearsals []observe.RehearsalJudgement
	// Demonstration is the bounded capture this session watched, nil when none was approved.
	//
	// Present whether it completed or not: an incomplete demonstration's REASON is the useful
	// part, and a report that showed only successes would leave a user wondering why nothing
	// happened after they said yes.
	// RouteWalk is the ordered durable edges this demonstration traced, first crossing
	// first — the sequence a review has to follow. The edges are reusable knowledge on
	// their own; this says which of them, in what order, THIS demonstration is evidence for.
	RouteWalk     []observe.RelationshipRef
	Demonstration *observe.ProcedureCandidate
	// Watched is why the pass that WATCHED a demonstration did not produce one from what it
	// saw, empty when it did or when the session was never licensed to.
	//
	// Carried because the alternative is silence: the runner declines, Learn falls back to the
	// armed capture, and a person is told "that example did not finish" about an example
	// nobody ever tried to build. Three separate diagnoses in this subsystem have been lost
	// exactly this way — the reason existed one layer down and nothing carried it up.
	Watched observe.DiscoveryRefusal
	// Proposals is what Marco asked the user about this session, and what came back.
	//
	// Carried on the terminal Result because an answer usually arrives AFTER the session
	// has ended — a three-minute session finishes long before somebody reads the question.
	Proposals observe.ProposalLedger
	Stats     Stats
}

// Complete reports whether the session ran its course.
func (r Result) Complete() bool { return r.Session.State.Succeeded() }

// Stats are the measured costs and losses.
type Stats struct {
	SamplesTaken   int           `json:"samples_taken"`
	SamplesSkipped int           `json:"samples_skipped"`
	SamplesLate    int           `json:"samples_late"`
	LabelPasses    int           `json:"label_passes"`
	SamplerErrors  int           `json:"sampler_errors"`
	TargetLosses   int           `json:"target_losses"`
	Generations    []uint64      `json:"generations,omitempty"`
	Elapsed        time.Duration `json:"elapsed"`

	// Shadow is the accumulated findings of an EXPERIMENTAL provider, when one ran.
	//
	// Reporting only: nothing here influenced the session.s authoritative analysis, and
	// nothing in the session.s authoritative analysis was derived from it.
	Shadow observe.ShadowTotals `json:"shadow,omitzero"`

	// ProvenanceRefusals counts SAMPLES in which at least one target-scoped provider
	// could not prove its evidence described the pinned generation.
	//
	// Counted per sample rather than per observation: the question a reader is asking is
	// "did the guard ever fire during this session", and a single stale accessibility
	// walk carrying two thousand observations would otherwise dwarf every other number.
	ProvenanceRefusals int `json:"provenance_refusals"`
	// ProvenanceQuarantined is the total evidence refused across the session.
	ProvenanceQuarantined int `json:"provenance_quarantined,omitempty"`
	// RefusedProviders names each provider that was refused at least once, with the
	// reason it gave the first time. Bounded by the provider count, not by sample count.
	RefusedProviders map[string]string `json:"refused_providers,omitempty"`
	// ProvenProviders maps each provider that proved its target to the generation it
	// proved. The positive half, and the one that answers the question a targeted session
	// exists to ask: did every contributing source describe the SAME live window?
	//
	// More than one generation for one provider across a session is not itself a fault —
	// a window legitimately changes generation on restart — but it means the session
	// spans two windows and its evidence must not be read as one continuous view.
	ProvenProviders map[string][]uint64 `json:"proven_providers,omitempty"`
}

// foldProvenance folds one sample's guard verdicts into the session totals.
//
// Reads the sample's own provider summaries rather than re-deriving anything: the verdict
// has one implementation, and a second opinion computed here would eventually disagree with
// the engine that actually decided.
func (s *Stats) foldProvenance(sample observe.Sample) {
	refusedThisSample := false
	for _, p := range sample.Providers {
		// A provider with no outcome (State empty) predates the guard or reported
		// nothing; it is not a refusal, and counting it as one would make every
		// untargeted cycle look like a failure.
		if p.State == "" || p.Global {
			continue
		}
		if p.Proven {
			if p.Observations > 0 {
				s.ProvenProviders = appendProven(s.ProvenProviders, p.Name, p.Expected)
			}
			continue
		}
		// A REFUSAL requires evidence to refuse.
		//
		// This is the same rule Cycle.Admitted applies, and it is repeated here because
		// the first live run got it wrong: a refiner that produces nothing and an OCR
		// engine that is not installed both fail to prove a target, and counting them
		// reported "91 of 91 samples refused evidence (0 observations quarantined)" —
		// alarming, prominent, and false. A diagnostic that disagrees with the engine it
		// describes is worse than no diagnostic.
		if p.Quarantined == 0 {
			continue
		}
		refusedThisSample = true
		s.ProvenanceQuarantined += p.Quarantined
		if s.RefusedProviders == nil {
			s.RefusedProviders = map[string]string{}
		}
		if _, seen := s.RefusedProviders[p.Name]; !seen {
			s.RefusedProviders[p.Name] = p.Reason
		}
	}
	if refusedThisSample {
		s.ProvenanceRefusals++
	}
}

// appendProven records a generation a provider proved, without duplicates.
func appendProven(m map[string][]uint64, name string, gen uint64) map[string][]uint64 {
	if m == nil {
		m = map[string][]uint64{}
	}
	for _, got := range m[name] {
		if got == gen {
			return m
		}
	}
	m[name] = append(m[name], gen)
	return m
}

// Runner performs one session.
type Runner struct {
	clock   Clock
	target  Target
	sampler Sampler
	events  Events

	mu       sync.RWMutex
	session  observe.Session
	stats    Stats
	analyzer *observe.Analyzer
	// live folds each sample's analysis into a streamable event log, so findings are
	// visible WHILE the session runs rather than only in its Result. It derives nothing
	// the analyzer has not already concluded; removing it changes no analysis and no
	// outcome, only what a front-end can show mid-session.
	live *observe.LiveRecorder
	// proposals is what Marco has asked the user this session and what came back.
	//
	// Session-scoped and mutable while the session runs, because an answer may arrive at
	// any moment through the service — including long after the screen it was about has
	// gone. Guarded by mu like everything else here.
	proposals observe.ProposalLedger
	// policy bounds when a question may be put. Held so Respond and the sampling loop
	// agree on it without threading Config through both.
	policy observe.ProposalThresholds
	// memory is durable semantic knowledge, nil when none is wired.
	//
	// An interface supplied by the composition root: this package never opens a file, and
	// a runner that could would be a runner that could write one.
	memory observe.Memory
	// places persists a place's identity, nil when none is wired.
	//
	// Held SEPARATELY from memory even though one store implements both, for the same reason
	// candidates is separate: the two answer different questions, and a field typed as
	// observe.PlaceStore cannot record a judgement whatever it is assigned.
	places observe.PlaceStore
	// learningPolicy decides when an observed habit has earned an invitation.
	learningPolicy observe.LearningThresholds
	// capture is the demonstration this session is watching, nil when none was approved.
	//
	// ONE at a time, and never resumed across sessions: a demonstration is a bounded thing
	// somebody agreed to give, and carrying an unfinished one into the next run would be
	// watching without being asked again.
	capture *observe.Capture
	// captureBounds structurally limit what one demonstration may watch.
	captureBounds observe.CaptureBounds
	// candidates is where a completed demonstration goes, nil when none is wired.
	candidates observe.CandidateStore
	// targets is the Repertoire: where a demonstrated semantic target becomes durable.
	//
	// Beside the candidate store rather than inside it, because they answer different
	// questions — one remembers what was DONE, the other what it was done TO — and a
	// Director wired for one and not the other should degrade honestly rather than crash.
	targets observe.TargetStore
	// grant is the rehearsal authorization this session issued, nil when none.
	//
	// EPHEMERAL by construction: there is no marshalling of it anywhere and no path that
	// could restore one, so a new Director begins with no authority whatever happened before.
	grant *observe.RehearsalGrant
	// epoch identifies this runner, so an attempt id from a previous one could never be
	// mistaken for one from this.
	epoch string
	// captureSequence is which observation of the route this capture is producing: 1 for the
	// first approved demonstration, 2 for the follow-up.
	captureSequence int
	// armAttempted records that this session has already looked for an approved demonstration.
	//
	// One look per session. The lookup reads the durable topology, which takes the store's lock
	// and may touch a file, and the answer cannot change part-way through: the selector is pinned
	// to one window of one process for the whole run.
	armAttempted bool
	// learning is the most recent judgement of every remembered relationship — eligible or
	// not, and WHY not. Carried so a report can explain silence: "Marco did not ask" has a
	// dozen explanations and a reader cannot otherwise tell them apart.
	learning []observe.LearningAssessment
	// authorization is why the most recent yes to a rehearsal created no grant, empty when
	// it created one. Silence here was a real failure mode: see AuthorizationRefusal.
	authorization AuthorizationRefusal
	// authorizationFor is the route that refusal was about.
	//
	// Carried because a refusal without its subject is a refusal about whatever the reader
	// happens to ask. A sequential edge review asks per leg, and a reason recorded while
	// answering leg 2 read as leg 1.s reason — a wrong explanation being strictly worse than
	// none, since a reader trusts it.
	authorizationFor observe.RelationshipRef
}

// New returns a runner over its dependencies.
// WithMemory installs durable semantic memory.
//
// Optional and nil-safe: a Director with no memory asks every session as though it were the
// first, which is exactly the behaviour that existed before this milestone.
func (r *Runner) WithMemory(m observe.Memory) *Runner {
	r.memory = m
	r.learningPolicy = observe.DefaultLearningThresholds()
	return r
}

// WithPlaces installs somewhere a place's identity can be made durable.
//
// Optional and nil-safe: a Director without one recognises only what a person has already answered
// a question about, which is exactly the behaviour that existed before this milestone — and the
// behaviour that made an explicit `learn "…"` impossible to start from a cold application.
//
// Separate from WithMemory although the composition root passes the same store to both, because a
// runner that reached the place store through its memory field would be a runner in which the
// narrow interface bought nothing.
func (r *Runner) WithPlaces(p observe.PlaceStore) *Runner {
	r.places = p
	return r
}

// WithCandidates installs somewhere for a completed demonstration to be kept.
//
// Optional and nil-safe. A Director without one still watches an approved demonstration and
// still reports what it saw; the candidate simply does not survive the session.
func (r *Runner) WithCandidates(c observe.CandidateStore) *Runner {
	r.candidates = c
	r.captureBounds = observe.DefaultCaptureBounds()
	return r
}

// WithTargets wires the Repertoire, so a demonstrated semantic target can become durable.
//
// Separate from WithCandidates because a Director may legitimately have one and not the other,
// and a session that cannot remember targets should still be able to remember demonstrations.
func (r *Runner) WithTargets(t observe.TargetStore) *Runner {
	r.targets = t
	return r
}

// WithCaptureBounds overrides what one demonstration may watch.
//
// Exported so a test can reach a bound without scripting sixty keypresses. Production uses the
// defaults.
func (r *Runner) WithCaptureBounds(b observe.CaptureBounds) *Runner {
	r.captureBounds = b
	return r
}

// Capture reports the demonstration this session is watching, nil when none.
//
// A bounded, safe view: counts and closed vocabulary. There are no raw keys to withhold, because
// none were ever observed.
func (r *Runner) Capture() *observe.CaptureView {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.capture.View()
}

// WithEpoch names this runner, for attempt identity.
//
// Supplied by the composition root rather than read from a clock here: `time.Now().UnixNano()`
// is nanosecond-typed and not nanosecond-resolved, and this repository has already had one flaky
// test caused by two runtimes reading the same instant.
func (r *Runner) WithEpoch(e string) *Runner {
	r.epoch = e
	return r
}

// CancelCapture stops the demonstration at the user's request.
func (r *Runner) CancelCapture() bool {
	// Cancelling withdraws any rehearsal authorization too. A grant is permission for a
	// future experiment, and a user who just said stop has not left it standing.
	r.RevokeRehearsal()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.capture == nil || r.capture.State.Settled() {
		return false
	}
	r.capture.Cancel()
	return true
}

// WithLearningPolicy overrides when an observed habit earns an invitation.
//
// Exported so a test can exercise the policy's boundaries without scripting weeks of sessions.
// Production uses the defaults.
func (r *Runner) WithLearningPolicy(t observe.LearningThresholds) *Runner {
	r.learningPolicy = t
	return r
}

func New(clock Clock, target Target, sampler Sampler, events Events) *Runner {
	if events == nil {
		events = NopEvents{}
	}
	return &Runner{clock: clock, target: target, sampler: sampler, events: events}
}

// Snapshot is the responsive view of a running session.
//
// Taken under a read lock held for a few field copies and nothing else. The whole point is
// that `director status` answers while a detector is running, so this must never wait on
// anything that touches a screen.
func (r *Runner) Snapshot() (observe.Session, Stats) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.session, r.stats
}

// LiveEvents returns analysis events after a cursor, with the numbers that make loss
// detectable.
//
// Read lock only: a client may poll this while a sample is being taken, which is the whole
// point of a live feed. Nothing here observes.
func (r *Runner) LiveEvents(cursor uint64, limit int) (events []observe.LiveEvent,
	newest, oldest uint64) {

	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.live == nil {
		return nil, 0, 0
	}
	return r.live.Since(cursor, limit), r.live.Newest(), r.live.Oldest()
}

// Respond records the user's answer to a question this session asked.
//
// THE response-consumption call site. Keyed on the proposal's own identity, never on whatever
// is on screen now: the user may answer minutes later, after the screen has changed and the
// state that prompted the question has been renumbered or has gone entirely. The answer belongs
// to the question that was put.
//
// Does not block, does not touch perception, and does not end the session. A question is a
// question; answering it changes what Marco believes and nothing about what it is doing.
func (r *Runner) Respond(id observe.ProposalID, resp observe.UserResponse) (observe.Proposal, bool) {
	r.mu.Lock()
	p, ok := r.proposals.Respond(id, resp, r.stats.SamplesTaken)
	r.mu.Unlock()
	if !ok {
		return p, ok
	}
	r.ApplyAnswer("", p, resp)
	return p, ok
}

// ApplyAnswer gives an answer its MEANING, for a proposal this runner may not have raised.
//
// # Why that separation has to exist
//
// Answering is two acts: recording what was said, and acting on it — writing an approval into
// memory, arming a capture, or turning a yes into authority. The second is the runner's, because
// the runner owns the store, the bounds and the one ephemeral grant.
//
// `Respond` does both, and could only do the second for a question in ITS OWN ledger. The comment
// that justified that said every question is asked at the end of a session, so by the time anybody
// can answer, no session is running and the newest runner is the one that asked.
//
// Learn breaks that assumption completely. A learn episode runs bounded passes back to back, so
// the newest runner is routinely a session that started AFTER the question was raised and whose
// ledger has never held it. Measured live, on Home → Bluetooth → Mouse:
//
//	the proposal:  answered, confirmed, evidence 9f4a56779b4f2389
//	the judgement: eligible, digest 9f4a56779b4f2389, inputs 1
//	the authority: none
//	step 1 of 2: Home → Bluetooth — trying, forever
//
// The yes was not refused. It was recorded in the finished session's ledger, handed to a runner
// that had never heard of it, and dropped — with no refusal written anywhere, because the code
// that writes refusals never ran. A yes that creates nothing must at least say why, and this one
// could not even do that.
//
// Deleting the `ApplyAnswer` fallback in observationRegistry.Answer must fail
// TestAYesReachesTheRunnerEvenWhenANewerSessionHasStarted.
func (r *Runner) ApplyAnswer(application string, p observe.Proposal, resp observe.UserResponse) {
	r.mu.RLock()
	memory := r.memory
	if application == "" {
		// The caller could not say, so fall back to this session.s own — which is right
		// for a question this runner raised, and is all `Respond` ever needs.
		application = r.session.Application
	}
	r.mu.RUnlock()
	if memory == nil {
		// A YES WITH NOWHERE TO GO STILL SAYS SO.
		//
		// The forbidden middle is `response: yes, consequence: none, refusal: none`. Every
		// other failure in this path records a reason and this one returned in silence, so
		// the one configuration where nothing can possibly work was also the one that
		// explained nothing.
		//
		// Deleting this must fail TestAYesWithNoStoreStillRecordsWhyItCreatedNothing.
		if p.Ask == observe.AskRehearse && p.Relationship != nil {
			r.noteAuthorization(AuthorizationNoStore, *p.Relationship)
		}
		return
	}
	// WHAT the answer meant is decided by the question's TYPE, never by its wording.
	//
	// The two `no`s are opposites. `no` to "is this a settings screen?" says Marco's
	// interpretation is wrong and becomes a durable contradiction. `no` to "shall I learn
	// this?" says the user does not want it learned and touches no evidence at all — the
	// transition still happened, five times, across three sessions.
	//
	// Routing on p.Ask rather than on p.Kind or on the sentence is what keeps those apart.
	// Deleting this branch must fail TestRefusingToLearnDoesNotContradictTheRelationship.
	switch p.Ask {
	case observe.AskLearnRelationship:
		r.recordLearningAnswer(memory, application, p, resp)
		return
	case observe.AskRehearse:
		// THE only yes that creates authority. Every other kind lands somewhere that cannot
		// authorise input, which is what makes "a previous yes to learn this" harmless.
		r.authorizeRehearsal(application, p, resp)
		if resp != observe.ResponseConfirmed {
			r.recordRehearsalAnswer(memory, application, p, resp)
		}
		return
	case observe.AskSecondDemonstration:
		// A THIRD meaning for the same three words. `no` here declines to demonstrate
		// again — it does not withdraw the original request to learn, does not
		// invalidate the first demonstration, and contradicts nothing semantic.
		r.recordFollowUpAnswer(memory, application, p, resp)
		return
	}
	// THE memory write, and it happens HERE because this is a semantic EVENT.
	//
	// Not per sample, not per hypothesis refresh: a person settling a question is the only
	// thing worth making durable, which is what keeps the store bounded by subjects rather
	// than by observations.
	//
	// Written outside the lock: the store does file I/O, and holding the session lock
	// across a write would stall sampling behind a disk. A failure is not fatal — the
	// answer still applies to this session — and the store reports it.
	//
	// Deleting this call must fail TestAConfirmedSubjectIsRecognisedInALaterSession.
	_ = memory.Remember(application, observe.SignatureOf(observe.Hypothesis{
		Kind: p.Kind, Subject: p.Subject,
	}), observe.SemanticKnowledge{
		Kind: p.Kind, Status: observe.KnowledgeStatusFor(resp), Evidence: p.Evidence,
		Support: p.Support, Contradictions: p.Contradictions,
		Answered: answeredCount(resp),
	})
}

// recordLearningAnswer stores what the user said about learning one relationship.
//
// # What each answer means, and what none of them do
//
//	yes      → a PENDING request. Marco is willing to be shown, and can do nothing yet.
//	no       → a refusal. A PREFERENCE about learning, not a claim about the observation.
//	not now  → suppressed until the relationship's evidence changes shape.
//
// None of the three creates a procedure, a capability, an action or a route, and none of them
// starts a recorder. There is no registry here that could hold one — obtaining a demonstration
// is a separate workflow with its own consent, because it may need to observe more than passive
// discovery does. A yes that silently widened what Marco watches would be the worst possible
// reading of an invitation.
//
// Bound to the proposal's OWN relationship, which is why the reference travels on the proposal
// rather than being looked up from what is currently on screen: the answer usually arrives after
// the user has left both screens.
func (r *Runner) recordLearningAnswer(memory observe.Memory, application string,
	p observe.Proposal, resp observe.UserResponse) {

	if p.Relationship == nil {
		return
	}
	var status observe.LearningStatus
	switch resp {
	case observe.ResponseConfirmed:
		status = observe.LearningPending
	case observe.ResponseContradicted:
		status = observe.LearningRefused
	case observe.ResponseDeclined:
		status = observe.LearningDeclined
	default:
		return
	}
	// Written outside the session lock: the store does file I/O. A failure is not fatal —
	// the answer still applies to this session's ledger — and the store reports it.
	_ = memory.RememberLearning(application, *p.Relationship, observe.LearningRequest{
		Status: status, Evidence: p.Evidence,
	})
}

// answeredCount is 1 for a real answer and 0 for a decline, which settles nothing.
func answeredCount(r observe.UserResponse) int {
	if r.Answered() {
		return 1
	}
	return 0
}

// Proposals returns a snapshot of what has been asked and answered.
func (r *Runner) Proposals() observe.ProposalLedger {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.proposals.Clone()
}

// LiveAnalysis is the CURRENT state of the analysis, for a client that has no history.
//
// Events alone are not enough when the overlay opens midway through a session, reconnects,
// or hits a retention gap: it has to be able to rebuild what it should be showing without
// inferring the events it missed. This is that rebuild, and it is the analyzer's own result
// types rather than a second summary.
func (r *Runner) LiveAnalysis(t observe.InsightThresholds) observe.LiveAnalysis {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.analyzer == nil {
		return observe.LiveAnalysis{}
	}
	findings := r.analyzer.Findings()
	out := observe.LiveAnalysis{
		SessionID: r.session.ID,
		State:     r.session.State,
		Samples:   r.stats.SamplesTaken,
		StartedAt: r.session.StartedAt,
		EndedAt:   r.session.EndedAt,
		Reason:    r.session.Reason,
		Findings:  findings,
		Insights:  observe.Insights(findings, t),
		// The live view generates from the evidence accumulated SO FAR, which is the
		// point: a hypothesis that only appears once a session ends cannot answer "what is
		// Marco learning while I play". Annotated with anything the user has already said.
		Hypotheses: r.proposals.Annotate(
			observe.Hypotheses(r.stats.Shadow, observe.DefaultHypothesisThresholds())),
	}
	if r.live != nil {
		out.Cursor = r.live.Newest()
		out.Oldest = r.live.Oldest()
		out.WithdrawnHypotheses = r.live.WithdrawnCount()
	}
	return out
}

// Run performs the session and returns when it ends.
//
// Cancelling ctx stops it: no further sample begins, the evidence already collected is
// kept, and the session is Cancelled rather than Completed — a distinction that matters
// because only Completed licenses reading conclusions from the sample size.
func (r *Runner) Run(ctx context.Context, cfg Config) (Result, error) {
	bounds, err := cfg.Bounds.Normalise()
	if err != nil {
		return Result{}, err
	}
	if err := cfg.Selector.Validate(); err != nil {
		return Result{}, err
	}
	if cfg.LabelEvery <= 0 {
		cfg.LabelEvery = DefaultLabelEvery
	}
	if cfg.MaxLabelPasses <= 0 {
		cfg.MaxLabelPasses = DefaultMaxLabelPasses
	}
	thresholds := cfg.Thresholds
	if thresholds.MinSamples == 0 {
		thresholds = observe.DefaultThresholds()
	}
	insightThresholds := cfg.Insights
	if insightThresholds.MinPanelLabels == 0 {
		insightThresholds = observe.DefaultInsightThresholds()
	}
	hypothesisThresholds := cfg.Hypotheses
	if hypothesisThresholds.MinEpisodes == 0 {
		hypothesisThresholds = observe.DefaultHypothesisThresholds()
	}
	proposalPolicy := cfg.ProposalPolicy
	if proposalPolicy.MaxOpen == 0 {
		proposalPolicy = observe.DefaultProposalThresholds()
	}
	// The resolved thresholds go back onto cfg, which is what the per-sample path reads.
	//
	// Without this the sampling loop uses the caller's ZERO values while the terminal
	// Result uses the defaults — so a session's live feed and its final report would be
	// computed at different settings. That was already true of Insights; it is fixed here
	// rather than reproduced, because a second consumer reading unresolved thresholds is
	// how the discrepancy would have become load-bearing.
	cfg.Thresholds, cfg.Insights = thresholds, insightThresholds
	cfg.Hypotheses, cfg.ProposalPolicy = hypothesisThresholds, proposalPolicy

	start := r.clock.Now()
	r.mu.Lock()
	r.analyzer = observe.NewAnalyzer(thresholds, bounds)
	r.live = observe.NewLiveRecorder(cfg.ID, observe.DefaultMaterialThresholds())
	r.proposals = observe.ProposalLedger{}
	r.policy = proposalPolicy
	r.session = observe.Session{
		ID: cfg.ID, Selector: cfg.Selector, Bounds: bounds,
		StartedAt: start, State: observe.Observing,
	}
	r.stats = Stats{}
	// Beside the other per-run resets, and deliberately untested: the registry builds a new
	// Runner per pass, and `capture` is not reset here either, so a second Run on one runner is
	// not a shape production has. It is here so this line does not become the one that was
	// forgotten if that ever changes.
	r.armAttempted = false
	r.mu.Unlock()

	r.publish(Event{Kind: SessionStarted, SessionID: cfg.ID, At: start, State: observe.Observing})

	deadline := start.Add(bounds.Duration)
	// The arming used to be here, on cfg.Selector.Application, and that was wrong: see
	// armForApplication. It happens on the first sample instead, still before anything reaches
	// the capture.
	final, reason := r.loop(ctx, cfg, bounds, deadline)

	end := r.clock.Now()
	r.mu.Lock()
	r.session.State = final
	r.session.Reason = reason
	r.session.EndedAt = &end
	r.stats.Elapsed = end.Sub(start)
	findings := r.analyzer.Findings()
	session := r.session
	stats := r.stats
	// A deep copy: the ledger keeps accepting answers after this snapshot is taken, and a
	// shared backing array would let a late answer mutate a Result that has already been
	// handed out.
	proposals := r.proposals.Clone()
	r.mu.Unlock()

	kind := SessionCompleted
	switch final {
	case observe.Cancelled:
		kind = SessionCancelled
	case observe.Failed, observe.TargetUnavailable:
		kind = SessionFailed
	}
	r.publish(Event{Kind: kind, SessionID: cfg.ID, At: end, State: final, Reason: reason})

	// THE hypothesis call site, computed once and used twice: for the Result, and to name
	// the endpoints of this session's transitions.
	hypotheses := proposals.Annotate(observe.Hypotheses(stats.Shadow, hypothesisThresholds))

	// THE place-establishment call site. Once per session, here, and nowhere else.
	//
	// # Why immediately before the relationships and not after
	//
	// Because an edge is only written when BOTH endpoints resolve to remembered subjects. The
	// destination of a learn pass is a place the user has just walked to and Marco has never
	// been told anything about; establishing it after the topology was folded would reject the
	// one edge the whole attempt exists to record, and the user would be told their destination
	// was not recognised while it sat in the store.
	//
	// # Why session end and not per sample
	//
	// The same reason the relationships are here. The current state is a conclusion the
	// segmenter reaches over the whole pass, and a per-sample write would establish every
	// transition frame the user walked through — which is the unbounded persistence this
	// mechanism is careful not to be.
	//
	// Deleting this call must fail TestTeachingEstablishesTheStartThroughTheProductionPath;
	// deleting the licence check inside it must fail TestAnOrdinarySessionEstablishesNoPlace.
	places := r.establishPlace(session.Application, stats, cfg)

	// THE durable relationship call site. Once per session, here, and nowhere else.
	//
	// # Why session end and not per sample
	//
	// Three reasons, and each of them on its own would be enough. The transition tally GROWS
	// while the session runs, so an edge written per sample would be written with an
	// incomplete count and then rewritten with a slightly larger one, forever. `Sessions` is
	// a count of independent corroborations and only a finished session is one. And the store
	// writes its whole file atomically, so a batch of edges is one write where n edges would
	// be n.
	//
	// # Why after everything else
	//
	// Because both halves have to be complete: the transitions are this session's own
	// evidence, and the endpoints are named by consulting memory — in that order.
	// [[ADR-018-a-remembered-relationship-is-adjacency-not-a-route]] states the direction
	// this arrow may not point, and TestMemoryDoesNotManufactureATransition holds it.
	//
	// Deleting this call must fail TestARelationshipIsCorroboratedByALaterSession.
	relationships, grew := r.rememberRelationships(session.Application, stats, hypotheses,
		cfg.SameEpisode)

	// THE learning-proposal call site. Once per session, at the end, and after the durable
	// topology has been updated with what this session contributed.
	//
	// # Why the end and not per sample
	//
	// Two reasons, and the second is the one that matters.
	//
	// The evidence is only complete here: this session's own transitions were folded into the
	// store one line above, so an edge that has just earned its third independent
	// corroboration can be offered immediately rather than next time.
	//
	// And it makes the PRIORITY structural rather than a race. Semantic understanding comes
	// before behaviour learning — if Marco is still asking what a screen is, it has no
	// business asking whether to learn a route involving it — and a per-sample invitation
	// would win the single interruption slot simply by being ready on sample one, before any
	// hypothesis had accumulated enough recurrence to be worth asking about. Running here
	// gives the semantic side the whole session to claim the budget, and an invitation that
	// finds it spent says so with `another_question_open`.
	//
	// An invitation raised at session end is still perfectly answerable. That is the ORDINARY
	// case for every question this system asks — a three-minute session finishes long before
	// somebody reads what it wanted to know — and it is also the kinder moment to ask
	// somebody whether they would like to show you something.
	//
	// Deleting this call must fail TestACorroboratedRelationshipEarnsALearningQuestion.
	// THE candidate-consumption call site. An unfinished demonstration ends here with its
	// reason; a complete one becomes durable evidence.
	candidate := r.finishCapture(session.Application)
	// THE one-shot candidate, and it is HERE for one reason: everything downstream of a
	// demonstration reads the candidate store, and the questions that grant authority are
	// raised a few lines below. A candidate built after the session ended is invisible to all
	// of it — which is exactly what happened, live: Learn reached "want me to try?" and
	// waited for a grant that could never be created, because no `AskRehearse` proposal
	// existed to answer. See [[ADR-054-the-one-shot-candidate-belongs-to-the-session]].
	//
	// Same store, same assessment, same review, same proposal machinery as the armed capture.
	// Nothing here creates authority; it makes a candidate visible to the thing that asks for
	// it, and a person still answers.
	r.mu.RLock()
	walkMemory := r.memory
	r.mu.RUnlock()
	var watched observe.DiscoveryRefusal
	if candidate == nil {
		candidate, watched = r.watchedDemonstration(session.Application, stats, grew,
			cfg.Episode, hypothesisThresholds)
	}
	// THE assessment call site. One per session, on the demonstration that just finished,
	// against the topology as it stands after this session contributed to it.
	//
	// Deleting this line must fail TestACompletedDemonstrationIsAssessedOnTheProductionPath.
	assessment := r.assessCandidate(session.Application, candidate)
	// THE follow-up call site. After the assessment, sharing the same interruption budget, so
	// semantic questions and first-time learning invitations both get first claim.
	followUps := r.reviewFollowUp(session.Application)
	// Last of the question kinds, deliberately: understanding, then learning, then another
	// example, and only then permission to try.
	rehearsals := r.reviewRehearsal(session.Application, cfg.Episode, candidate)
	proposals = r.Proposals()
	learning := r.reviewLearning(session.Application, stats.SamplesTaken)
	// Re-snapshot: the review may have opened a question, and the Result must carry it.
	proposals = r.Proposals()

	return Result{
		Session:  session,
		Findings: findings,
		Insights: observe.Insights(findings, insightThresholds),
		// THE hypothesis call site. Every session that ends — completed, cancelled or
		// failed — passes through here exactly once.
		//
		// Deliberately on the terminal Result rather than beside the accumulator: a
		// hypothesis is a read over the whole session's evidence, and generating one per
		// sample would let an interpretation exist before the recurrence that justifies it.
		//
		// Deleting this line must fail a test that enters through the runner; see
		// TestTheProductionSessionPathGeneratesHypotheses. This is the fourth mechanism in
		// this subsystem to need such a test, and the third to have been written without
		// one — see [[Wiring-Tests]].
		//
		// Annotated with whatever the user has said, so the terminal report can show
		// "you confirmed this" beside the observations rather than only the observations.
		// Deleting the Annotate call must fail TestAConfirmedHypothesisIsValidatedInTheResult.
		Hypotheses:    hypotheses,
		Places:        places,
		Relationships: relationships,
		Learning:      learning,
		// THE ORDER, from the session.s own walk. Not from the store, not from the
		// candidate set, not from a map — see observe.DemonstratedWalk.
		RouteWalk:     observe.DemonstratedWalk(stats.Shadow, session.Application, walkMemory, hypothesisThresholds, grew),
		Demonstration: candidate,
		Watched:       watched,
		Assessment:    assessment,
		FollowUps:     followUps,
		Rehearsals:    rehearsals,
		Proposals:     proposals,
		Stats:         stats,
	}, nil
}

// establishPlace makes the place the user is standing on durably recognisable, when this session
// was licensed to and the evidence allows it.
//
// It persists an IDENTITY and nothing else. There is no SemanticKnowledge in this function, no
// path from here to one, and the field it writes through is typed so there could not be: a place
// established here carries an empty interpretation list, and every reader of a subject already
// treats that as "recognised, nothing known about what it is".
//
// Returns the account rather than a bool, because each refusal means something different to
// somebody trying to show Marco something.
func (r *Runner) establishPlace(application string, stats Stats,
	cfg Config) observe.PlaceEstablishment {

	if !cfg.EstablishPlaces {
		return observe.PlaceEstablishment{Reason: observe.PlaceNotLicensed}
	}
	r.mu.RLock()
	memory, places := r.memory, r.places
	r.mu.RUnlock()
	if memory == nil || places == nil {
		return observe.PlaceEstablishment{Licensed: true, Reason: observe.PlaceNoMemory}
	}
	// EVERY place this pass settled on, not only the one it ended at. A person
	// demonstrating a route walks THROUGH places, and an intermediate one that never
	// becomes durable leaves the edges either side of it with an unresolvable endpoint —
	// measured live, and the reason a captured, attributed, semantically-resolved
	// demonstration still could not be learned. See observe.PlacesToEstablish.
	//
	// Deleting the loop, or narrowing it back to the current state, must fail
	// TestEveryPlaceOnTheRouteBecomesDurable.
	candidates, refusal := observe.PlacesToEstablish(stats.Shadow, application, memory,
		cfg.Hypotheses)
	out := observe.PlaceEstablishment{Licensed: true, Reason: refusal}
	for _, c := range candidates {
		// Outside the session lock: the store does file I/O, and a failure is not fatal —
		// the session's own evidence is unaffected and the report says what could not be
		// written.
		id, err := places.EstablishPlace(application, c.Signature)
		if err != nil || id == "" {
			if c.Current {
				out.Reason = observe.PlaceNotWritten
			}
			continue
		}
		if c.Current {
			out.Subject, out.Reason = id, ""
			continue
		}
		out.Also = append(out.Also, id)
	}
	// AND WHAT ANY OF THEM APPEAR TO BE CALLED — a sweep of its own, over every state this
	// session saw rather than over the ones it just created.
	//
	// A Place is established the first time Marco can recognise it; a name settles by
	// recurrence. The two almost never coincide, so writing the name inside the loop above
	// meant a Place established on pass one and named on pass three never got the name: by
	// then there was nothing left to establish and the loop did not run for it.
	//
	// Identity does not depend on any of this. A failure here loses a word, not a Place.
	//
	// Deleting this must fail TestAKnownPlaceStillGainsItsName.
	if namer, ok := places.(observe.PlaceNamer); ok {
		for subject, name := range observe.PlaceNamesToRecord(
			stats.Shadow, application, memory, cfg.Hypotheses) {

			_ = namer.ObserveSemanticName(application, subject, name, observe.FromStructure)
		}
	}
	return out
}

// rememberRelationships makes this session's observed transitions durable, where both of their
// endpoints are subjects memory recognises.
//
// Returns what happened, for the session report. A session whose transitions all stayed
// session-local is a perfectly ordinary session — most are, early on — and the report says so
// rather than being silent, because "no durable edges" has two explanations and only one of them
// is about the screen.
func (r *Runner) rememberRelationships(application string, stats Stats,
	hypotheses []observe.Hypothesis, sameEpisode bool) (observe.RelationshipReport,
	[]observe.RelationshipRef) {

	r.mu.RLock()
	memory := r.memory
	r.mu.RUnlock()

	// What this session knows about whether it held ONE view of ONE window throughout.
	//
	// Read from figures the session already keeps rather than derived: a window replaced
	// part-way through, or a target that went away and came back, means an unsettled interval
	// could be hiding an entire restart — and no bound on its length would make bridging one
	// honest. The analysis core cannot observe either, so the session states them.
	//
	// Deleting this must fail TestABrokenObservationIsNotBridged.
	continuity := observe.Continuity{
		Generations: len(stats.Generations), TargetLosses: stats.TargetLosses,
	}
	obs, report := observe.RelationshipsFrom(stats.Shadow, hypotheses, application, memory,
		continuity)
	if memory == nil || len(obs) == 0 {
		return report, nil
	}
	// An EPISODE already counted folds its evidence and claims no further independent
	// corroboration — see observe.RelationshipObservation.SameEpisode. Stamped here rather
	// than inside the store, because whether two sessions are one episode is a fact about
	// WHY they were run, and the store has no way to know it.
	//
	// Deleting this loop must fail TestATeachingEpisodeCorroboratesOnce.
	for i := range obs {
		obs[i].SameEpisode = sameEpisode
	}
	// Outside the session lock: the store does file I/O, and a failure is not fatal — the
	// session's own evidence is unaffected and the report says what could not be written.
	update, err := memory.RememberRelationships(application, obs)
	report.Created, report.Corroborated, report.Rejected =
		update.Created, update.Corroborated, update.Rejected
	if err != nil {
		report.Unavailable = err.Error()
		return report, nil
	}
	// WHICH edges this session contributed, for the one-shot candidate. Taken from the
	// observations that were actually written rather than recomputed, so the candidate can
	// only ever describe a route the store now holds.
	grew := make([]observe.RelationshipRef, 0, len(obs))
	for _, o := range obs {
		grew = append(grew, observe.RelationshipRef{From: o.From, To: o.To})
	}
	return report, grew
}

// watchedDemonstration builds the demonstration from the pass that WATCHED it, and puts it where
// every later stage already looks.
//
// # Why it lives here
//
// A demonstration is not finished when the evidence exists; it is finished when the candidate is
// in the store, because that is what `assessCandidate` judges and what `reviewRehearsal` turns
// into the question a person answers. Authority comes from answering that question and from
// nothing else.
//
// Built in the Learn COORDINATOR first, and that was wrong in a way only a live run showed:
// the coordinator runs after the session has ended, so the candidate appeared minutes after the
// only stage that could have raised `AskRehearse`. Learn reached "want me to try?" and waited
// for a grant nobody could give. See [[ADR-054-the-one-shot-candidate-belongs-to-the-session]].
//
// # What it is allowed to do
//
// Store one candidate. It creates no proposal, no grant and no authority, and it cannot: the
// review below is the only thing that asks, and a person is the only thing that answers.
//
// # What it refuses
//
//   - Unlicensed sessions. A pass nobody asked for does not produce demonstrations, however clean
//     its evidence — that is the whole difference between watching and being shown.
//   - Anything `CandidateFromDiscovery` refuses for the TERMINAL leg, which falls through to
//     the armed capture unchanged.
//
// It no longer refuses a multi-edge demonstration. A → B → C decomposes into one candidate
// per grown edge — each a reusable piece of route evidence — and the leg that arrived where
// the person stopped is the one the learn tail carries forward. That is the goal-centric
// correction: the demonstration is evidence, the destination is the capability, and no leg
// of the way there is welded to any other.
func (r *Runner) watchedDemonstration(application string, stats Stats,
	grew []observe.RelationshipRef, ep Episode,
	th observe.HypothesisThresholds) (*observe.ProcedureCandidate, observe.DiscoveryRefusal) {

	if !ep.EstablishPlaces {
		return nil, ""
	}
	if len(grew) == 0 {
		return nil, observe.DiscoveryNoTransition
	}
	r.mu.RLock()
	memory, store := r.memory, r.candidates
	r.mu.RUnlock()
	if memory == nil || store == nil {
		return nil, observe.DiscoveryNoMemory
	}
	// EVERY grown edge contributes, one candidate each. A → B → C is two reusable pieces of
	// route evidence, never one monolithic macro — the decomposition the goal-centric model
	// rests on. An edge whose evidence does not support a candidate is skipped, and skipping
	// it does not cost the others: knowledge of the legs that COULD be read survives whatever
	// happened to the legs that could not.
	var terminal *observe.ProcedureCandidate
	var terminalWhy observe.DiscoveryRefusal
	built := 0
	for _, ref := range grew {
		d := observe.CandidateFromDiscovery(stats.Shadow, ref, application, memory, th)
		if !d.Built || !d.Candidate.Complete {
			if d.EndsAtCurrent || len(grew) == 1 {
				terminalWhy = d.Refusal
				if d.Refusal == "" {
					terminalWhy = observe.DiscoveryNoTransition
				}
			}
			continue
		}
		// THE storage call, and it is the same one the armed capture makes. A candidate
		// that is not here is invisible to the assessment, to the review, and therefore to
		// the question.
		if err := store.RememberCandidate(application, d.Candidate); err != nil {
			if d.EndsAtCurrent || len(grew) == 1 {
				terminalWhy = observe.DiscoveryNotStored
			}
			continue
		}
		// THE durable TARGETS, from the same demonstration and under the same licence.
		//
		// A person pressed something they could see and Marco could name. That control is a
		// semantic thing in the world — it has a name, a sort, and a place it lives — and
		// remembering it is what lets a saved play refer to it later without naming the
		// provider that happened to find it.
		//
		// Here rather than beside the places, because a target is grounded in a place and a
		// place has to exist first; and gated on the same episode licence, because the
		// person who demonstrated this is the only reason any of it may be kept.
		//
		// Deleting this must fail TestADemonstratedTargetBecomesDurable.
		r.rememberTargets(application, d.Candidate)
		built++
		// The TERMINAL leg — the one arriving where the person stopped — is the candidate
		// the learn tail carries forward. A single-edge demonstration is its own
		// terminal, whether or not the session still shows its destination.
		if d.EndsAtCurrent || (len(grew) == 1 && terminal == nil) {
			copied := d.Candidate
			terminal = &copied
		}
	}
	switch {
	case terminal != nil:
		return terminal, ""
	case built > 0:
		// Legs were learned and none of them ends where the person is standing now, so
		// Marco cannot say which outcome the demonstration was FOR. The edge knowledge is
		// already in the store; only the one-shot tail falls back to the armed capture.
		return nil, observe.DiscoveryAmbiguousRoute
	case terminalWhy != "":
		return nil, terminalWhy
	default:
		return nil, observe.DiscoveryNoTransition
	}
}

// loop is the sampling schedule. Returns the terminal state and its reason.
func (r *Runner) loop(ctx context.Context, cfg Config, bounds observe.Bounds,
	deadline time.Time) (observe.State, string) {

	// Scheduled against INTENDED times rather than "sleep(interval) after the work".
	// Sleeping after a 700ms capture on a 500ms interval drifts by 700ms every cycle, so
	// a three-minute session would take five and nobody would be told.
	next := r.clock.Now()
	sequence := 0
	consecutiveFailures := 0
	lostSince := time.Time{}

	for {
		if err := ctx.Err(); err != nil {
			return observe.Cancelled, "the session was cancelled"
		}
		now := r.clock.Now()
		if !now.Before(deadline) {
			return observe.Completed, ""
		}
		r.mu.RLock()
		taken := r.stats.SamplesTaken
		r.mu.RUnlock()
		if taken >= bounds.MaxFrames {
			return observe.Completed, ""
		}

		// Wait for the next scheduled slot.
		if wait := next.Sub(now); wait > 0 {
			select {
			case <-ctx.Done():
				return observe.Cancelled, "the session was cancelled"
			case <-r.clock.After(wait):
			}
			if err := ctx.Err(); err != nil {
				return observe.Cancelled, "the session was cancelled"
			}
		}

		sequence++
		state, reason, ok := r.oneSample(ctx, cfg, bounds, sequence, &lostSince, &consecutiveFailures)
		if !ok {
			return state, reason
		}
		// A LICENSED PASS DOES NOT DECIDE THAT THE PERSON IS FINISHED.
		//
		// It used to: one settled screen different from the one it opened on, plus about
		// two seconds of stillness, and the pass ended. That reads a PAUSE as an ending,
		// and the pause between legs of a walk is exactly when somebody is looking for the
		// next thing to click.
		//
		// Measured on the run that could not learn Home to Bluetooth to Mouse: the
		// demonstration pass ended 3.6 seconds into a 45 second budget, having taken 8
		// samples, and no session in the whole episode ever contained the Mouse
		// composition. The second edge was not lost in review, in ownership or in naming —
		// its destination was never observed, because the pass had already stopped
		// watching. The Audience.s own words: "it didn.t let me continue".
		//
		// A previous attempt at this raised the dwell from "the first settled screen" to
		// two seconds, with a comment describing this very failure. Raising it again would
		// be guessing at how long a person takes to find a row.
		//
		// There is no guess to make. The panel says "Press Stop Learning when you are
		// done", and Stop ends the pass through Finish, keeping everything it saw. The
		// person already has a way to say they have finished, and it is the only
		// trustworthy one — so a pass ends when they say so, or when its budget runs out.
		//
		// The cost is that a one-hop demonstration runs its full length unless the person
		// presses Stop. That is a wait; truncating a walk is a wrong answer.
		//
		// Deleting this comment.s consequence — reinstating an arrival-based end — must
		// fail TestALicensedPassWatchesTheWholeWalk.

		// Advance the schedule, skipping slots that have already passed rather than
		// queueing them. A queued backlog would run captures back to back and describe
		// a burst that never happened.
		interval := bounds.Interval
		now = r.clock.Now()
		next = next.Add(interval)
		if next.Before(now) {
			missed := int(now.Sub(next)/interval) + 1
			r.mu.Lock()
			r.stats.SamplesLate += missed
			r.mu.Unlock()
			next = now.Add(interval)
		}
	}
}

// oneSample validates the target, takes a sample, and folds it in.
//
// Returns ok=false with a terminal state when the session should stop.
func (r *Runner) oneSample(ctx context.Context, cfg Config, bounds observe.Bounds,
	sequence int, lostSince *time.Time, consecutiveFailures *int) (observe.State, string, bool) {

	ref, err := r.target.Acquire(ctx, cfg.Selector)
	if err != nil {
		return r.handleTargetLoss(ctx, cfg, bounds, sequence, lostSince, err)
	}
	if !lostSince.IsZero() {
		// Back after an absence.
		*lostSince = time.Time{}
		r.setState(observe.Observing, "")
		r.analyzerNote(observe.Transition{
			Kind: observe.TargetReacquired, At: r.clock.Now(), Sequence: sequence,
			Confidence: 1,
			Reason: "the selected window became available again; evidence resumes here " +
				"and may belong to a different window generation",
		})
		r.publish(Event{
			Kind: TargetReacquired, SessionID: cfg.ID, At: r.clock.Now(),
			Sequence: sequence, Generation: ref.Generation, State: observe.Observing,
		})
	}

	readLabels := r.shouldReadLabels(cfg, sequence)
	sample, err := r.sampler.Sample(ctx, SampleRequest{
		Window: ref, Sequence: sequence, ReadLabels: readLabels,
	})
	if err != nil {
		*consecutiveFailures++
		r.mu.Lock()
		r.stats.SamplerErrors++
		r.stats.SamplesSkipped++
		r.mu.Unlock()
		r.publish(Event{
			Kind: SampleSkipped, SessionID: cfg.ID, At: r.clock.Now(), Sequence: sequence,
			Reason: "no sample was produced for this slot",
		})
		if *consecutiveFailures >= MaxConsecutiveFailures {
			return observe.Failed, fmt.Sprintf(
				"%d samples in a row produced nothing; the session was stopped rather than "+
					"run its full length collecting silence", *consecutiveFailures), false
		}
		return observe.Observing, "", true
	}
	*consecutiveFailures = 0

	if readLabels {
		r.mu.Lock()
		r.stats.LabelPasses++
		r.mu.Unlock()
	}

	sample.Sequence = sequence
	if sample.Timestamp.IsZero() {
		sample.Timestamp = r.clock.Now()
	}
	if sample.WindowGeneration == 0 {
		sample.WindowGeneration = ref.Generation
	}

	// THE arming, and it is here rather than before the loop because until a window reference
	// resolves the session does not know what it is watching. Above the fold below, so the very
	// first cycle still reaches the capture and a user already standing where the request begins
	// is seen — which is the property the old placement was reaching for.
	//
	// Deleting this must fail TestAWindowChosenWithoutNamingItsApplicationStillArms.
	r.armForApplication(ref.Application)

	r.mu.Lock()
	r.analyzer.Observe(sample)
	r.stats.SamplesTaken++
	r.stats.Generations = appendGeneration(r.stats.Generations, sample.WindowGeneration)
	r.session.Application = ref.Application
	r.session.Generations = r.stats.Generations
	r.stats.foldProvenance(sample)
	// The screen composition, and the experiment folded in beside the evidence rather than
	// into it.
	//
	// THE call site. Every successful sample passes through here exactly once, which is
	// why it is the only place the screen model and the shadow findings are accumulated —
	// the previous attempt put the accumulator somewhere production never reached, and a
	// five-minute live session discarded every finding it produced. Deleting this line must
	// fail a test that enters through the runner; see
	// TestDeletingTheShadowAccumulationCallIsCaught and
	// TestAnAccessibleApplicationProducesScreens.
	r.stats.foldStructure(sample)
	// THE demonstration feed. Every successful sample passes through here exactly once, on the
	// same accumulated evidence everything else reads — so a capture cannot observe a world
	// the rest of the session did not.
	r.observeCapture(sample, cfg.Hypotheses)
	// Fold the analysis the Analyzer just updated into the live feed. Nothing is
	// re-derived: Findings and Insights are the same pure reads the final Result uses,
	// and the recorder only reports what CHANGED. Under the same lock, because a client
	// polling the feed arrives on another goroutine while this runs.
	//
	// Findings() is computed ONCE and shared — it sorts, and calling it per consumer
	// would pay that cost twice a sample for the life of the session. Both it and
	// Insights are bounded and pure CPU: no capture, no provider, no I/O happens while
	// this lock is held.
	if r.live != nil {
		findings := r.analyzer.Findings()
		r.live.Observe(sequence, findings,
			observe.Insights(findings, cfg.Insights), sample.Timestamp)
	}
	// THE proposal call site. Every successful sample passes through here exactly once.
	//
	// One call doing two things on purpose: it attaches whatever the user has already said
	// about each hypothesis, and it asks about any that have newly earned a question. They
	// are two halves of one idea, and splitting them would create a second call site that
	// could be forgotten independently — which is how this subsystem has now shipped four
	// mechanisms nothing invoked.
	//
	// Asking is a pure read over evidence already accumulated: no capture, no provider, no
	// I/O, and nothing here blocks waiting for an answer. Perception continues while a
	// question is open, and the answer arrives later through the service.
	//
	// Deleting this line must fail TestTheProductionSessionPathProposesQuestions.
	hypotheses := observe.Hypotheses(r.stats.Shadow, cfg.Hypotheses)
	// THE memory read. What earlier sessions already established about evidence like this,
	// seeded into the ledger as questions already answered — so a subject the user
	// confirmed last week is not asked about again today.
	//
	// It runs BEFORE the proposal policy, deliberately: recall must be able to suppress a
	// question, and a policy that had already decided to ask would only be able to withdraw
	// one. Deleting this line must fail TestAConfirmedSubjectIsRecognisedInALaterSession.
	r.proposals.RecallFrom(hypotheses, r.session.Application, r.memory)
	r.proposals.Refresh(hypotheses, sequence, r.policy)
	r.mu.Unlock()

	r.publish(Event{
		Kind: SampleCompleted, SessionID: cfg.ID, At: sample.Timestamp,
		Sequence: sequence, Generation: sample.WindowGeneration, State: observe.Observing,
	})
	return observe.Observing, "", true
}

// handleTargetLoss decides whether a missing target ends the session.
//
// Never falls back to the foreground window, and never reuses the last known bounds. The
// only two honest outcomes are "wait a little longer" and "stop and say so".
func (r *Runner) handleTargetLoss(ctx context.Context, cfg Config, bounds observe.Bounds,
	sequence int, lostSince *time.Time, cause error) (observe.State, string, bool) {

	now := r.clock.Now()
	if lostSince.IsZero() {
		*lostSince = now
		r.mu.Lock()
		r.stats.TargetLosses++
		r.mu.Unlock()
		r.setState(observe.TargetUnavailable, cause.Error())
		r.analyzerNote(observe.Transition{
			Kind: observe.TargetLost, At: now, Sequence: sequence, Confidence: 1,
			Reason: "the selected window became unavailable; no frame was taken and no " +
				"other window was substituted",
		})
		r.publish(Event{
			Kind: TargetUnavailable, SessionID: cfg.ID, At: now, Sequence: sequence,
			State: observe.TargetUnavailable, Reason: cause.Error(),
		})
	}

	r.mu.Lock()
	r.stats.SamplesSkipped++
	r.mu.Unlock()

	if now.Sub(*lostSince) >= bounds.ReacquireWindow {
		return observe.TargetUnavailable, fmt.Sprintf(
			"the selected window did not come back within %s (%v); the evidence collected "+
				"before it went is kept, and the session is NOT complete",
			bounds.ReacquireWindow, cause), false
	}
	if err := ctx.Err(); err != nil {
		return observe.Cancelled, "the session was cancelled", false
	}
	return observe.TargetUnavailable, "", true
}

// shouldReadLabels decides whether this sample pays for scoped OCR.
//
// The first sample always reads, so a session has labels from the start; after that only
// every LabelEvery-th, and never past the session cap. Measured live: 39 regions cost 9.0
// seconds unbounded, 3.7s with the per-frame ceiling — far too much to spend on every
// frame of a three-minute session.
func (r *Runner) shouldReadLabels(cfg Config, sequence int) bool {
	r.mu.RLock()
	passes := r.stats.LabelPasses
	r.mu.RUnlock()
	if passes >= cfg.MaxLabelPasses {
		return false
	}
	return sequence == 1 || sequence%cfg.LabelEvery == 0
}

func (r *Runner) setState(s observe.State, reason string) {
	r.mu.Lock()
	r.session.State = s
	r.session.Reason = reason
	r.mu.Unlock()
}

func (r *Runner) analyzerNote(t observe.Transition) {
	r.mu.Lock()
	r.analyzer.Note(t)
	r.mu.Unlock()
}

func (r *Runner) publish(e Event) {
	if r.events != nil {
		r.events.Publish(e)
	}
}

// appendGeneration records a generation the first time it is seen.
func appendGeneration(seen []uint64, g uint64) []uint64 {
	if g == 0 {
		return seen
	}
	for _, s := range seen {
		if s == g {
			return seen
		}
	}
	return append(seen, g)
}

// ErrBusy is returned when a session is already running.
var ErrBusy = errors.New("a passive observation session is already running")

// foldStructure accumulates one sample's screen composition and the experiment's findings.
//
// NOT gated on the shadow field. It was, and that was the defect: a Director with no
// structural detector produces samples with no shadow record, this returned immediately, and
// the session therefore had no screen model — for every application, familiar or not. The
// composition is on the sample either way; which source described it is StructureOf's
// decision, not this function's.
func (s *Stats) foldStructure(sample observe.Sample) { s.Shadow.Observe(sample) }

// reviewLearning offers to learn the habits that have earned an invitation.
//
// Reads the durable topology and puts at most one question, sharing the same interruption budget
// every other question uses. Returns a judgement of EVERY remembered edge, eligible or not, so a
// report can explain silence — see Result.Learning.
func (r *Runner) reviewLearning(application string, inference int) []observe.LearningAssessment {
	r.mu.Lock()
	memory, policy, learningPolicy := r.memory, r.policy, r.learningPolicy
	r.mu.Unlock()
	if memory == nil {
		return nil
	}
	// The store read happens OUTSIDE the session lock: it takes the store's own lock and may
	// touch a file, and holding both would order two locks for no reason.
	top := memory.Topology(application)
	if len(top.Relationships) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.proposals.ReviewRelationships(top, inference, policy, learningPolicy)
}

// armForApplication arms the approved demonstration once the session knows what it is watching.
//
// # Why the application cannot come from the selector
//
// `cfg.Selector` names a WINDOW — by ephemeral id, by title, by process — and only the
// `--application` form happens to carry the name the durable topology is keyed by. Arming from the
// selector therefore armed nothing at all for a session that chose its window any other way,
// including `--window-id` and the foreground one a person actually uses. `Topology("")` holds no
// relationships, so no request was ever pending, no capture was ever created, and Learn told the
// user "I wasn't watching for your example" after watching them demonstrate it four times.
//
// Invisible to the whole suite because every fixture builds `Selector{Application: "testgame"}`.
// The resolved name has always been available one line further down, as `ref.Application`.
//
// One attempt per session, because the lookup reads the store and the answer cannot change: the
// selector is pinned to one window of one process for the run.
func (r *Runner) armForApplication(application string) {
	if application == "" {
		return
	}
	r.mu.Lock()
	if r.armAttempted {
		r.mu.Unlock()
		return
	}
	r.armAttempted = true
	r.mu.Unlock()
	// OUTSIDE the session lock, for the reason reviewLearning gives: the store takes its own
	// lock and may touch a file.
	r.armCapture(application)
}

// armCapture arms a demonstration for an approved learning request, if one is waiting.
//
// THE authorisation gate. A capture exists only when memory holds a relationship whose learning
// request is PENDING — a person having answered yes to a question Marco put. There is no other
// constructor and no other caller, which is what makes "no orphan observation becomes a
// demonstration" a property of the wiring rather than of a check somebody could remove.
//
// One at a time. A second approved request waits for the next session rather than being captured
// alongside: two demonstrations watched at once would share one stream of navigation, and there
// is no honest way to say which press belonged to which.
//
// Deleting this call must fail TestAnApprovedDemonstrationIsCapturedEndToEnd.
func (r *Runner) armCapture(application string) {
	r.mu.RLock()
	memory, existing := r.memory, r.capture
	r.mu.RUnlock()
	if memory == nil || existing != nil {
		return
	}
	top := memory.Topology(application)
	// A FOLLOW-UP outranks a first demonstration, because it is the one the user was most
	// recently asked for and agreed to. Sequence travels with the capture so the candidate it
	// produces knows which observation of the route it is.
	sequence := 1
	pending := observe.PendingFollowUp(top)
	if len(pending) > 0 {
		sequence = 2
	} else {
		pending = observe.PendingLearning(top)
	}
	if len(pending) == 0 {
		return
	}
	bounds := r.captureBounds
	if bounds.MaxEvents == 0 {
		bounds = observe.DefaultCaptureBounds()
	}
	r.mu.Lock()
	if r.capture == nil {
		r.capture = observe.NewCapture(application, pending[0], bounds)
		r.captureSequence = sequence
	}
	r.mu.Unlock()
}

// observeCapture feeds one observation cycle to a running demonstration.
//
// Called under the session lock from the per-sample choke point, because it reads the same
// accumulated shadow evidence everything else there does. It is pure CPU over bounded data: no
// capture, no provider, no I/O.
//
// The subject resolution asks MEMORY where the user is right now. The learning request says the
// demonstration is about A → B, which is a claim about history; whether the user is standing on A
// at this moment is a question only this cycle can answer, and assuming it because the request
// said so is the stale-ordinal mistake in another costume.
func (r *Runner) observeCapture(sample observe.Sample, th observe.HypothesisThresholds) {
	if r.capture == nil || r.capture.State.Settled() {
		return
	}
	in := observe.CaptureInput{}
	if sh := sample.Shadow; sh != nil {
		in.Ran = sh.Ran && sh.TargetProven
		in.Inputs = sh.Inputs
	}
	if in.Ran {
		// THE resolution, and it is `observe.PlaceNow` rather than a chain assembled here.
		// The Learn coordinator asks the same question before it says "go ahead", and two
		// derivations of "what screen is this" would eventually give two answers.
		p := observe.PlaceNow(r.stats.Shadow, r.session.Application, r.memory, th)
		in.Placed, in.Structure, in.EditableFields = p.Placed, p.Structure, p.EditableFields
		in.Verdict, in.Subject = p.Verdict, p.Subject
	}
	r.capture.Observe(in)
}

// finishCapture ends an unfinished demonstration and keeps a completed one.
//
// THE candidate-consumption call site. A demonstration that completed becomes durable evidence; an
// unfinished one becomes an incomplete record with its reason and is never carried forward.
func (r *Runner) finishCapture(application string) *observe.ProcedureCandidate {
	r.mu.Lock()
	capture, store, memory := r.capture, r.candidates, r.memory
	if capture != nil {
		capture.EndSession()
	}
	r.mu.Unlock()
	if capture == nil {
		return nil
	}
	c, ok := capture.Candidate()
	if !ok {
		return nil
	}
	// Only a COMPLETE demonstration is kept. An incomplete one is reported — its reason is
	// the useful part — and never stored as evidence of how anything is done.
	c.Sequence = r.captureSequence
	if c.Sequence == 0 {
		c.Sequence = 1
	}
	if c.Complete && store != nil {
		_ = store.RememberCandidate(application, c)
		// THE durable TARGETS, from the other licensed path.
		//
		// An armed capture runs because the person answered "yes, learn this" to a
		// question about a route they had already been observed taking — an explicit
		// licence exactly as a learn pass is. The two paths produce candidates in
		// different ways and both are demonstrations, so both make the things the person
		// pressed durable. Establishing on only one would mean a target existed or did
		// not depending on which door the demonstration came through.
		//
		// Deleting this must fail TestADemonstratedTargetBecomesDurable.
		r.rememberTargets(application, c)
		// THE thing that stops the collection loop: a request that stayed pending would
		// arm a capture in every later session, forever and without asking again.
		if memory != nil {
			_ = memory.FulfilLearning(application, c.Relationship, c.Sequence)
		}
	}
	return &c
}

// assessCandidate judges the demonstration this session watched.
//
// Against the CURRENT topology, read from memory at the moment of judging rather than from
// anything the candidate carries. That dependency is the point: the same demonstration assessed
// tomorrow, after the user has told Marco what one of its screens is, produces a better verdict
// without a single new observation.
//
// Returns nil when there was no demonstration, so a report can tell "nothing was watched" from
// "something was watched and could not be judged".
func (r *Runner) assessCandidate(application string,
	c *observe.ProcedureCandidate) *observe.CandidateAssessment {

	if c == nil {
		return nil
	}
	r.mu.RLock()
	memory, bounds := r.memory, r.captureBounds
	r.mu.RUnlock()
	if memory == nil {
		return nil
	}
	// Corroborated where a second approved demonstration of the same route exists. The
	// dependency is a parameter rather than a lookup inside the judgement, so the same
	// candidate and the same inputs always give the same verdict.
	a := observe.AssessCandidate(*c, memory.Topology(application), bounds,
		r.corroborationFor(application, c))
	return &a
}

// AssessStored judges every demonstration memory holds for this application.
//
// The recomputation surface. A caller that wants to know what Marco makes of its demonstrations
// TODAY asks for it and gets a fresh judgement; nothing durable holds a stale one.
func AssessStored(application string, memory observe.Memory, store observe.CandidateStore,
	bounds observe.CaptureBounds) []observe.CandidateAssessment {

	if memory == nil || store == nil {
		return nil
	}
	top := memory.Topology(application)
	var out []observe.CandidateAssessment
	for _, c := range store.Candidates(application) {
		corr := observe.Corroboration{}
		for _, other := range store.Candidates(application) {
			if other.Relationship == c.Relationship && other.Sequence != c.Sequence {
				corr = observe.Corroboration{
					Compared: true, Agreement: observe.CompareCandidates(c, other),
				}
			}
		}
		out = append(out, observe.AssessCandidate(c, top, bounds, corr))
	}
	return out
}

// corroborationFor compares this candidate with the OTHER approved demonstration of the same
// route, when one exists.
//
// THE comparison call site. It uses `observe.CompareCandidates` and implements nothing of its
// own — a second comparison would be a second answer to "are these the same procedure", and the
// two would drift.
//
// Deleting this call must fail TestASecondDemonstrationIsComparedWithTheFirst.
func (r *Runner) corroborationFor(application string,
	c *observe.ProcedureCandidate) observe.Corroboration {

	if c == nil || !c.Complete {
		return observe.Corroboration{}
	}
	r.mu.RLock()
	store := r.candidates
	r.mu.RUnlock()
	if store == nil {
		return observe.Corroboration{}
	}
	for _, other := range store.Candidates(application) {
		if other.Relationship != c.Relationship || other.Sequence == c.Sequence {
			continue
		}
		return observe.Corroboration{
			Compared: true, Agreement: observe.CompareCandidates(*c, other),
		}
	}
	return observe.Corroboration{}
}

// reviewFollowUp asks for a second demonstration, where the assessment says one would help.
//
// # Why this reads the STORE rather than only this session's candidate
//
// Because the gap usually becomes visible in a session that captured nothing. The first
// demonstration was given days ago; today's session simply observed the application and can
// still notice that its judgement of that old demonstration has an opening a second example
// would close.
//
// Runs after the learning review and shares the same interruption budget, so the ordering is
// semantic questions, then first-time learning invitations, then follow-ups. If Marco is still
// asking what a screen IS it has no business asking for a second demonstration of a route
// through it, and a session with an open question says `another_question_open` rather than
// asking twice.
//
// Deleting this call must fail TestAFollowUpIsRequestedWhenAnotherExampleWouldHelp.
func (r *Runner) reviewFollowUp(application string) []observe.FollowUpReport {
	r.mu.RLock()
	memory, store, bounds, policy := r.memory, r.candidates, r.captureBounds, r.policy
	r.mu.RUnlock()
	if memory == nil || store == nil {
		return nil
	}
	if bounds.MaxEvents == 0 {
		bounds = observe.DefaultCaptureBounds()
	}
	top := memory.Topology(application)

	// One report per ROUTE, judged on its first demonstration. A follow-up is about whether
	// another example would help, and the candidate that raised the question is candidate 1.
	byRoute, routes := routesByRecency(store.Candidates(application))

	r.mu.Lock()
	defer r.mu.Unlock()
	var out []observe.FollowUpReport
	for _, ref := range routes {
		out = append(out, r.proposals.ReviewFollowUp(
			ref, byRoute[ref], top, bounds, policy))
	}
	return out
}

// recordFollowUpAnswer stores what the user said when asked for another example.
//
//	yes      → a second bounded demonstration request, on the SAME route and lineage.
//	no       → a preference. Candidate 1, the relationship and every assessment stay exactly
//	           as they were: declining to demonstrate something again says nothing about
//	           whether the transition happens.
//	not now  → suppressed until Marco's JUDGEMENT changes shape.
//
// None of the three verifies anything, promotes anything, or creates anything that can run.
func (r *Runner) recordFollowUpAnswer(memory observe.Memory, application string,
	p observe.Proposal, resp observe.UserResponse) {

	if p.Relationship == nil {
		return
	}
	var status observe.LearningStatus
	switch resp {
	case observe.ResponseConfirmed:
		status = observe.LearningPending
	case observe.ResponseContradicted:
		status = observe.LearningRefused
	case observe.ResponseDeclined:
		status = observe.LearningDeclined
	default:
		return
	}
	_ = memory.RememberFollowUp(application, *p.Relationship, observe.LearningRequest{
		Status: status, Evidence: p.Evidence,
	})
}

// reviewRehearsal asks whether Marco may try a candidate itself, where the evidence allows.
//
// THE judgement→proposal call site. Once per session, after every other question kind has had
// its chance at the interruption budget: understanding first, then learning, then another
// example, and only last permission to act.
//
// Deleting this call must fail TestARehearsalIsProposedOnlyWhenTheEvidenceAllows.
func (r *Runner) reviewRehearsal(application string, ep Episode,
	demonstrated *observe.ProcedureCandidate) []observe.RehearsalJudgement {

	r.mu.RLock()
	memory, store, bounds, policy := r.memory, r.candidates, r.captureBounds, r.policy
	grant := r.grant
	r.mu.RUnlock()
	// AUTHORISED means authorised for THE ROUTE BEING JUDGED, not "some authority exists".
	//
	// A grant names its route. Reading it as a blanket "already authorized" let one
	// outstanding permission veto the question about every OTHER route — which deadlocked a
	// sequential edge review live: an unspendable grant for leg 2 meant leg 1 could never be
	// asked about, so the review sat on "trying" indefinitely.
	//
	// Deleting the relationship comparison must fail
	// TestAGrantForOneRouteDoesNotSilenceTheQuestionAboutAnother.
	authorised := func(ref observe.RelationshipRef) bool {
		return grant != nil && grant.Active() && grant.Relationship == ref
	}
	invited := ep.PermissionExpected
	// ONE EXTRA SLOT, when the person is waiting to be asked.
	//
	// Not a wider budget: one more open question than usual, claimed by the route this
	// session actually DEMONSTRATED. Every other route still meets the ordinary bound, and
	// MaxProposals is untouched.
	//
	// See Episode.PermissionExpected for why Learn may have this and observation may not.
	//
	// Deleting this must fail TestAnInvitedRehearsalQuestionIsNotBlockedByAnotherQuestion.
	if memory == nil || store == nil {
		return nil
	}
	if bounds.MaxEvents == 0 {
		bounds = observe.DefaultCaptureBounds()
	}
	top := memory.Topology(application)

	byRoute, routes := routesByRecency(store.Candidates(application))
	// THE EXEMPTION FOLLOWS THE DEMONSTRATION, not the candidate store's ordering.
	//
	// # The live failure
	//
	// A learn called "Open Mouse Settings" demonstrated `subj_543793ccc326 →
	// subj_61ffd6bc8602` — one step, one click, and the session's own conclusion names that
	// candidate. The panel then sat on "I think I got it, but I can't ask you for permission
	// right now" with ZERO questions open, which is the signature of an exemption that went
	// somewhere else.
	//
	// It had. `routesByRecency` orders by the index of each route's newest candidate in the
	// store, and the pass that watched the demonstration also recorded four incidental
	// transitions between screens the person merely passed through. One of those was appended
	// after the demonstrated one, so it sorted first and took the only exempt slot; the route
	// the person had actually asked about was reviewed third and refused
	// `another_question_open`. Every other run in this session's chain hit the same wall.
	//
	// Recency was a proxy for "the one just demonstrated" and the proxy is simply wrong when a
	// pass sees more than one transition — which, on a real desktop, is the normal case rather
	// than the exception. The session already KNOWS which candidate it demonstrated; it is
	// three lines above the call site. So it is asked rather than guessed at.
	//
	// Deleting this must fail TestTheExemptionFollowsTheDemonstratedRoute.
	if demonstrated != nil {
		routes = demonstratedFirst(routes, demonstrated.Relationship)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	var out []observe.RehearsalJudgement
	// THE QUESTION THE PERSON ASKED FOR IS NOT RATIONED.
	//
	// # Why a widened budget was not enough
	//
	// The first attempt at this gave Learn ONE extra open question. It was still refused,
	// live, in a completely clean sandbox: Learn runs several bounded passes and the
	// incidental semantic questions accumulate — three were open by the time rehearsal was
	// reviewed, against a budget of two. Rationing the question somebody explicitly asked for
	// against questions they did not means it loses whenever the screen was interesting.
	//
	// So the invitation EXEMPTS rather than widens, and it is spent on exactly one route: the
	// one this session demonstrated, hoisted to the front above. Every other route meets the
	// ordinary bound, MaxProposals still bounds the ledger, one proposal per route is still
	// enforced by identity, and nothing here authorises anything — a rehearsal still happens
	// only when the person says yes.
	//
	// See Episode.PermissionExpected for why Learn may do this and observation may not.
	//
	// Deleting the exemption must fail TestAnInvitedRehearsalQuestionIsNotRationed.
	spent := false
	for _, ref := range routes {
		first, corr := firstAndCorroboration(byRoute[ref])
		if first == nil {
			continue
		}
		// THE POLICY IS NORMALISED BEFORE IT IS WIDENED.
		//
		// ReviewRehearsal replaces a wholly-zero policy with the defaults at its top, and
		// the registry never sets ProposalPolicy — so `policy` here is the zero value in
		// production. The exemption used to be `th.MaxOpen = th.MaxProposals`, which on a
		// zero policy is `0 = 0`: still wholly zero, so ReviewRehearsal substituted the
		// defaults and put the budget of one straight back.
		//
		// The exemption erased itself, silently, and only in production — every test that
		// passed a real policy exempted correctly, which is why four live runs ended at
		// "I can't ask you for permission right now" while the tests stayed green.
		//
		// Deleting the normalisation must fail TestTheExemptionSurvivesAZeroPolicy.
		th := policy
		if th.MaxOpen == 0 && th.MaxProposals == 0 {
			th = observe.DefaultProposalThresholds()
		}
		if invited && !spent {
			th.MaxOpen = th.MaxProposals
			spent = true
		}
		// The assessment stays the owner of "what does this evidence support". The
		// judgement only asks the rehearsal-specific question on top of it.
		a := observe.AssessCandidate(*first, top, bounds, corr)
		j := observe.JudgeRehearsal(*first, a, top, application)
		out = append(out, r.proposals.ReviewRehearsal(j, top, authorised(ref), th))
	}
	return out
}

// firstAndCorroboration picks the demonstration a judgement is about and what the other said.
func firstAndCorroboration(candidates []observe.ProcedureCandidate) (
	*observe.ProcedureCandidate, observe.Corroboration) {

	var first *observe.ProcedureCandidate
	for i := range candidates {
		if candidates[i].Sequence >= 2 {
			continue
		}
		if first == nil || candidates[i].Sequence < first.Sequence {
			first = &candidates[i]
		}
	}
	if first == nil {
		return nil, observe.Corroboration{}
	}
	for i := range candidates {
		if candidates[i].Sequence >= 2 {
			return first, observe.Corroboration{
				Compared:  true,
				Agreement: observe.CompareCandidates(*first, candidates[i]),
			}
		}
	}
	return first, observe.Corroboration{}
}

// authorizeRehearsal turns a yes into exactly one ephemeral grant.
//
// THE yes→grant call site, and the only one. Every other proposal kind's yes lands elsewhere and
// creates nothing that could authorise input — which is the whole reason `Proposal.Ask` is typed.
//
// The judgement is RECOMPUTED here rather than trusted from when the question was asked. Between
// the question and the answer the candidate may have gained a second demonstration that disagrees
// with the first, or a subject the route depends on may have been contradicted; authorising
// against evidence Director no longer believes would be authorising something the user never saw.
// The digest comparison is what catches it.
//
// Deleting this must fail TestSayingYesToARehearsalCreatesExactlyOneGrant.
func (r *Runner) authorizeRehearsal(application string, p observe.Proposal,
	resp observe.UserResponse) {

	if p.Relationship == nil || resp != observe.ResponseConfirmed {
		return
	}
	r.mu.RLock()
	memory, store, bounds, epoch := r.memory, r.candidates, r.captureBounds, r.epoch
	existing := r.grant
	r.mu.RUnlock()
	if memory == nil || store == nil {
		r.noteAuthorization(AuthorizationNoStore, *p.Relationship)
		return
	}
	// ONE ACTIVE AUTHORITY AT A TIME, and a yes about a DIFFERENT route supersedes it.
	//
	// A second yes about the same route is still refused: it queues no second attempt, which
	// is what this rule has always been for.
	//
	// # Why a different route may take the slot
	//
	// A sequential edge review walks a demonstrated route leg by leg, and the ordinary
	// machinery may already have asked about a LATER leg — so a person can quite reasonably
	// say yes to leg 2 while the review is still on leg 1. Live, on Home → Bluetooth → Mouse:
	//
	//	an active grant for Bluetooth → Mouse, unusable (`source_mismatch`, Marco was at Mouse)
	//	step 1 of 2: Home → Bluetooth — trying, forever
	//
	// Nothing could ever release it. Asking about leg 1 was refused `already_authorized`, and
	// a yes to leg 1 would have been refused here — so the leg that had to go first could
	// neither be asked about nor authorised while an authority nobody could spend sat in the
	// slot.
	//
	// Superseding is what the person meant. They are looking at a question about this route
	// and saying yes to it; the older permission is for something Marco is not doing. It is
	// still exactly one authority, and it still came from an explicit yes about the thing it
	// authorises — which is the whole of what this slot protects.
	//
	// Deleting the relationship comparison must fail
	// TestAYesAboutAnotherRouteSupersedesAnUnspendableGrant.
	if existing != nil && existing.Active() && existing.Relationship == *p.Relationship {
		r.noteAuthorization(AuthorizationAlreadyGranted, *p.Relationship)
		return
	}
	if bounds.MaxEvents == 0 {
		bounds = observe.DefaultCaptureBounds()
	}

	top := memory.Topology(application)
	var candidates []observe.ProcedureCandidate
	for _, c := range store.Candidates(application) {
		// THE proposal's own route, never the newest one. An answer belongs to the question
		// that was put, and the user may well be looking at something else by now.
		if c.Relationship == *p.Relationship {
			candidates = append(candidates, c)
		}
	}
	first, corr := firstAndCorroboration(candidates)
	if first == nil {
		r.noteAuthorization(AuthorizationNoCandidate, *p.Relationship)
		return
	}
	a := observe.AssessCandidate(*first, top, bounds, corr)
	j := observe.JudgeRehearsal(*first, a, top, application)
	if !j.Eligible || j.Digest != p.Evidence {
		// Fail closed. The evidence moved between the question and the answer, so the yes
		// was given about something that no longer exists. Marco asks again.
		r.noteAuthorization(AuthorizationEvidenceMoved, *p.Relationship)
		return
	}
	grant, err := observe.NewRehearsalGrant(epoch, j, r.clock.Now())
	if err != nil {
		r.noteAuthorization(AuthorizationEvidenceMoved, *p.Relationship)
		return
	}
	r.mu.Lock()
	// Re-checked under the write lock, and by the SAME rule as above: two concurrent yeses
	// about one route must not both install, and a yes about another route supersedes.
	if r.grant == nil || !r.grant.Active() || r.grant.Relationship != *p.Relationship {
		r.grant = grant
	}
	r.authorization, r.authorizationFor = "", observe.RelationshipRef{}
	r.mu.Unlock()
}

// AuthorizationRefusal is the CLOSED vocabulary of why a person's yes created no authority.
//
// Every one of these used to be a silent return. The consequence was the worst kind of
// failure this system can produce: the person answers "yes, try it", nothing whatever
// happens, and ten minutes later Learn gives up with `rehearsal_declined` — a sentence
// that blames the person for the silence. A yes that creates nothing must say why.
type AuthorizationRefusal string

const (
	// AuthorizationNoStore — no memory or candidate store is wired, so there is nothing to
	// authorise an attempt against.
	AuthorizationNoStore AuthorizationRefusal = "no_store"
	// AuthorizationAlreadyGranted — an earlier yes is still outstanding. One active
	// authority at a time, and a second yes does not queue a second attempt.
	AuthorizationAlreadyGranted AuthorizationRefusal = "already_granted"
	// AuthorizationNoCandidate — no first demonstration of the route is in the store.
	AuthorizationNoCandidate AuthorizationRefusal = "no_candidate"
	// AuthorizationEvidenceMoved — the recomputed judgement no longer matches what the
	// question showed. Fail closed; Marco asks again.
	AuthorizationEvidenceMoved AuthorizationRefusal = "evidence_moved"
)

// noteAuthorization records why the most recent yes created nothing.
func (r *Runner) noteAuthorization(why AuthorizationRefusal, about observe.RelationshipRef) {
	r.mu.Lock()
	r.authorization, r.authorizationFor = why, about
	r.mu.Unlock()
}

// AuthorizationRefused reports why the most recent yes to a rehearsal created no grant,
// empty when it created one — or when nobody has said yes at all.
func (r *Runner) AuthorizationRefused() AuthorizationRefusal {
	if r == nil {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.authorization
}

// AuthorizationRefusedFor reports why the most recent yes ABOUT THIS ROUTE created no grant.
//
// Empty for any other route, which is the point: a sequential edge review asks one leg at a time,
// and a reason recorded while answering a different leg is not this leg.s reason. Reporting it as
// such told a reader something confident and false.
//
// Deleting the route comparison must fail TestAGrantRefusalIsAboutTheRouteItWasRecordedFor.
func (r *Runner) AuthorizationRefusedFor(route observe.RelationshipRef) AuthorizationRefusal {
	if r == nil {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.authorizationFor != route {
		return ""
	}
	return r.authorization
}

// Grant returns the active rehearsal authorization, nil when there is none.
//
// A read, and the only way out of the runner. Whatever consumes it later has to be handed it —
// there is no lookup, no registry and no durable copy.
func (r *Runner) Grant() *observe.RehearsalGrant {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.grant
}

// RevokeRehearsal withdraws the authorization.
//
// Called by cancellation and by session end. Fail-closed: a grant that outlived the session that
// issued it would be authority nobody is watching.
func (r *Runner) RevokeRehearsal() {
	if r == nil {
		return
	}
	r.mu.Lock()
	g := r.grant
	r.mu.Unlock()
	g.Revoke()
}

// recordRehearsalAnswer stores a no or a not-now to a rehearsal question.
//
//	no       → Marco does not try this. A PREFERENCE about acting, and nothing else: the
//	           candidate stands, the relationship stands, the demonstrations stand, and no
//	           semantic knowledge is contradicted. Declining to let something be attempted
//	           says nothing about whether it would have worked.
//	not now  → suppressed until the JUDGEMENT changes shape.
//
// Neither creates a grant, and neither is durable beyond the ledger — a rehearsal question is
// about this evidence in this session, and a refusal recorded forever would outlive the
// candidate it was about.
func (r *Runner) recordRehearsalAnswer(memory observe.Memory, application string,
	p observe.Proposal, resp observe.UserResponse) {

	// Deliberately a no-op beyond the ledger the proposal already updated.
	//
	// The ledger holds the answer, the suppression and the material-change rule, and that is
	// the whole of what a refusal to rehearse means. Writing it into semantic memory would be
	// recording a preference about ACTING as though it were something learned about the
	// application, which is the confusion this milestone exists to prevent.
	_, _, _, _ = memory, application, p, resp
}

// Revise replaces a settled answer, and Retract withdraws one.
//
// THE revision call sites, and they are deliberately separate verbs from `Respond`. Answering
// stays one-shot — that is what stops a double click, a stale panel or a replayed request from
// overwriting what somebody said — and changing your mind is something a caller has to mean.
//
// Both write through the SAME durable path an answer takes, so there is one place that decides
// what a judgement means and one record that holds it. Nothing here touches perception: the
// subject, its structure, its visits, its name and every relationship it takes part in are
// exactly as they were.
//
// Only SEMANTIC questions are revisable here. A learning invitation, a follow-up request and a
// rehearsal permission are answers about what Marco may DO, they create or withhold authority,
// and quietly re-running one of those from a revision would be the one place this milestone could
// widen what it is allowed to touch. They are refused.
func (r *Runner) Revise(id observe.ProposalID, resp observe.UserResponse) (observe.Proposal, bool) {
	return r.revise(id, func(l *observe.ProposalLedger, inference int) (observe.Proposal, bool) {
		return l.Revise(id, resp, inference)
	})
}

// Retract withdraws a previous answer.
func (r *Runner) Retract(id observe.ProposalID) (observe.Proposal, bool) {
	return r.revise(id, func(l *observe.ProposalLedger, inference int) (observe.Proposal, bool) {
		return l.Retract(id, inference)
	})
}

func (r *Runner) revise(id observe.ProposalID,
	apply func(*observe.ProposalLedger, int) (observe.Proposal, bool)) (observe.Proposal, bool) {

	r.mu.Lock()
	p, ok := apply(&r.proposals, r.stats.SamplesTaken)
	memory, application := r.memory, r.session.Application
	r.mu.Unlock()
	if !ok || memory == nil {
		return p, ok
	}
	if p.Ask != "" && p.Ask != observe.AskSemantic {
		// Not ours to revise. See the note above: these are answers about what Marco may
		// DO, and revising one here would reach past what this operation is for.
		return observe.Proposal{}, false
	}
	// THE durable write, and it is the same one an answer makes. A revision that wrote
	// somewhere else would be a second record, and the two would disagree.
	//
	// Deleting this call must fail TestARevisedAnswerIsWhatALaterSessionRecalls.
	_ = memory.Remember(application, observe.SignatureOf(observe.Hypothesis{
		Kind: p.Kind, Subject: p.Subject,
	}), observe.SemanticKnowledge{
		Kind: p.Kind, Status: observe.KnowledgeStatusForRevision(p), Evidence: p.Evidence,
		Support: p.Support, Contradictions: p.Contradictions,
	})
	return p, ok
}

// settledNow reports whether the named state has stopped changing shape, per this session's model.
func (r *Runner) settledNow(id observe.ScreenStateID) bool {
	for _, st := range r.stats.Shadow.States {
		if st.ID == id {
			return st.Settled
		}
	}
	return false
}

// rememberTargets makes the semantic things a demonstration acted on durable.
//
// # Why this is safe to do unconditionally here
//
// Because it is only reachable from watchedDemonstration, which has already refused every session
// without an explicit Learn licence. A target exists because a person asked Marco to learn and then
// pressed something; there is no path to this function from passive observation.
//
// # What it may keep, and what it may not
//
// The word on the control, the sort of thing it is, and the place it lives — nothing else. The
// signature type has nowhere to put a runtime id, a node handle or a rectangle, so a future edit
// that wanted to persist one would have to widen the durable identity in a visible way.
//
// Provenance is recorded as ACCESSIBILITY because that is the way of knowing that produced the
// label today. It is read by nothing that decides how to act later; see
// observe.EvidenceSource.Authoritative.
//
// Failures are not fatal. The demonstration is already stored, and a target that could not be
// written costs the play a way of referring to it rather than costing the person their evidence.
func (r *Runner) rememberTargets(application string, c observe.ProcedureCandidate) {
	r.mu.RLock()
	targets := r.targets
	r.mu.RUnlock()
	if targets == nil {
		return
	}
	for _, sig := range observe.TargetsDemonstrated(c) {
		_, _ = targets.RememberTarget(application, sig, observe.FromAccessible)
	}
}

// routesByRecency groups candidates by route, most recently demonstrated first.
//
// # Why the order decides whether anybody is asked at all
//
// The interruption budget is ONE open question (observe.DefaultProposalThresholds). Every route
// with a stored candidate is reviewed in a single pass against the same ledger, so the first
// eligible route takes the slot and every route after it is refused for budget. Order here is not
// presentation — it decides which question gets asked, and therefore whether the person in front
// of the panel is asked about the thing they just did.
//
// It used to be lexicographic by subject id: deterministic, and meaningless. The winner was
// whichever route happened to start from the lowest-sorting hash, and it stayed the winner
// forever. Live, with five stored routes, somebody demonstrated a new one, watched Marco say "I
// think I got it. Want me to try?" and could never be asked — the slot had gone to a route they
// had shown it earlier, and nothing on any surface said so.
//
// Store order is append order, so a later index is a more recently demonstrated route. Recency is
// just as deterministic and actually means something: what somebody just demonstrated is what
// they are waiting on. The old ordering breaks ties so the sort stays total.
//
// Changing this to sort by id must fail TestTheRouteJustDemonstratedGetsTheQuestion.
func routesByRecency(candidates []observe.ProcedureCandidate) (
	map[observe.RelationshipRef][]observe.ProcedureCandidate, []observe.RelationshipRef) {

	byRoute := map[observe.RelationshipRef][]observe.ProcedureCandidate{}
	newest := map[observe.RelationshipRef]int{}
	for i, c := range candidates {
		byRoute[c.Relationship] = append(byRoute[c.Relationship], c)
		newest[c.Relationship] = i
	}
	routes := make([]observe.RelationshipRef, 0, len(byRoute))
	for ref := range byRoute {
		routes = append(routes, ref)
	}
	sort.Slice(routes, func(i, j int) bool {
		if newest[routes[i]] != newest[routes[j]] {
			return newest[routes[i]] > newest[routes[j]]
		}
		if routes[i].From != routes[j].From {
			return routes[i].From < routes[j].From
		}
		return routes[i].To < routes[j].To
	})
	return byRoute, routes
}

// demonstratedFirst hoists one route to the front, keeping the rest in order.
//
// Used for the invited-rehearsal exemption, which is spent on the first route reviewed. Recency
// used to decide that and was a bad proxy: a pass that watches a demonstration also records every
// incidental transition the person passed through on the way, and any one of those can be newer
// than the route they actually asked about.
//
// A route the store has no candidate for is left alone rather than inserted — the exemption is a
// slot in a review, not a way to review something that has nothing behind it.
func demonstratedFirst(routes []observe.RelationshipRef,
	ref observe.RelationshipRef) []observe.RelationshipRef {

	for i, r := range routes {
		if r != ref {
			continue
		}
		out := make([]observe.RelationshipRef, 0, len(routes))
		out = append(out, r)
		out = append(out, routes[:i]...)
		return append(out, routes[i+1:]...)
	}
	return routes
}
