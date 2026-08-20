// Package teach is the human-facing orchestration of Marco's existing learning chain.
//
// # What it is
//
// A COORDINATOR. It owns no evidence, no identity, no judgement and no authority. Every fact it
// reports was produced by the ordinary passive machinery — observation, durable relationships,
// the demonstration capture, the candidate assessment — and every question it raises is a
// question the proposal ledger already knows how to ask.
//
// It exists because that machinery is complete and unreachable: a person who wants to teach
// Marco something has, until now, had to wait for Marco to notice a habit, offer to learn it, and
// then be shown. Teach reverses the timing without touching the evidence model.
//
// # The two-pass shape, and why it is not a shortcut
//
// The demonstration capture is a CONFIRMATION mechanism. It is armed for a named relationship
// A → B, waits until the user is standing on A, and completes when B is established from current
// evidence. That is exactly the wrong shape for "show me something new", because B is unknown at
// the moment a capture would have to be armed.
//
// Rather than give the capture an open destination — new semantics in the one model that
// assessment, rehearsal and the wrong-destination guard all rest on — Teach lets the FIRST pass
// be ordinary observation:
//
//	establish A          → the canonical identity path, evidence-driven
//	"go ahead"           → passive discovery; the user does the thing once
//	a durable A → B edge → now the route is known
//	"once more"          → the EXISTING pending-learning request arms the EXISTING capture
//	capture completes    → the EXISTING assessment, unchanged
//
// The arming is not a new door. `RememberLearning(..., LearningPending)` is precisely what a
// user's "yes" to "shall I learn this?" writes today, and `teach "..."` IS that yes — given in
// advance, about a route the user chose by demonstrating it. The store refuses a request for a
// relationship it does not hold, so a route whose endpoints never became durable subjects cannot
// be armed at all. Parts of the refusal matrix are therefore enforced by the store rather than by
// a check here that somebody could remove.
//
// It costs the user one extra run. Existing policy wants two examples anyway.
//
// # What Teach may not do
//
//   - It creates no authority. Teaching is permission to observe a bounded session, nothing more.
//     Rehearsal still needs its own explicit yes, through the ledger, as it always did.
//   - It supplies no input. There is no path from here to anything that can press a key, which
//     the package boundary test holds — so "Marco taught itself" is not reachable by mistake.
//   - It retains no text. The only string it holds that came from a person is the name they asked
//     for, and that is theirs.
package teach

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// Phase is the coarse orchestration state, and NOTHING else.
//
// It is not authority, not evidence, and not a judgement. Every real decision is owned by the
// object that already owns it — the capture's state, the assessment's verdict, the ledger's
// proposal — and the phase only says which of them Teach is currently waiting on.
type Phase string

const (
	// WaitingForDemonstration is waiting for there to be something to watch that is not
	// Marco.
	//
	// # Why a session begins here rather than by looking
	//
	// Because pressing "Start Learning" necessarily brings Marco to the front. A session that
	// resolved its target the instant it was asked would pin Marco's own control surface,
	// fingerprint it as the place the task starts from, and then watch a window the person
	// was about to leave. Requiring them to put the application in front FIRST does not fix
	// it — the Start button is in Marco, so the last thing they touch before the session
	// begins is always Marco.
	//
	// So the session waits. The person goes to the application in the ordinary way, and the
	// demonstration begins when there is a real subject to watch. Nothing is captured here
	// and nothing is decided.
	WaitingForDemonstration Phase = "waiting_for_demonstration"
	// EstablishingStart is waiting for the current place to become a durable subject.
	EstablishingStart Phase = "establishing_start"
	// ReadyForDemo has a start and is about to watch.
	ReadyForDemo Phase = "ready_for_demo"
	// Capturing is watching — the discovery pass, or the demonstration.
	Capturing Phase = "capturing"
	// EstablishingDestination is deciding whether where the user ended up is recognisable.
	EstablishingDestination Phase = "establishing_destination"
	// Evaluating is reading the assessment of a completed demonstration.
	Evaluating Phase = "evaluating"
	// NeedsAnotherExample is waiting for the user to show the route again.
	NeedsAnotherExample Phase = "needs_another_example"
	// ReadyToRehearse has an open rehearsal question the user must answer themselves.
	ReadyToRehearse Phase = "ready_to_rehearse"
	// Rehearsing is the authorised attempt, running.
	Rehearsing Phase = "rehearsing"
	// WaitingForStart holds a granted rehearsal until Marco can see the screen it begins on.
	//
	// The permission has been given and NOT spent. See notReadyYet.
	WaitingForStart Phase = "waiting_for_start"
	// ReviewingEdges owns the ordered review of a demonstration.s required edges.
	//
	// # Why it is re-entrant
	//
	// A demonstration of A → B → C is two reusable edges, and BOTH have to reach a terminal
	// review state before the episode can finish. The lifecycle used to run one rehearsal and
	// advance to Naming, so the second edge was never offered and a two-hop task could not be
	// taught in one sitting — the person had to demonstrate the first leg again, separately.
	//
	// This phase selects one unresolved edge, drives it through the ordinary rehearsal
	// question and attempt, consumes the result, and comes back for the next. It is the only
	// state that may advance the episode past the review, and it does so only when every
	// required edge is terminal.
	ReviewingEdges Phase = "reviewing_edges"
	// Naming has an open screen-naming question.
	Naming Phase = "naming"
	// Lowering is where a play would be written, once the questions above are settled.
	Lowering Phase = "lowering"
	// Complete, Refused and Cancelled are the three ways a teach session ends.
	Complete  Phase = "complete"
	Refused   Phase = "refused"
	Cancelled Phase = "cancelled"
)

// Settled reports whether the teach session is over.
func (p Phase) Settled() bool {
	return p == Complete || p == Refused || p == Cancelled
}

// Waiting reports whether Teach is blocked on the user answering an existing question.
//
// The distinction matters to whatever is driving: a waiting session must NOT be advanced on a
// timer, because nothing has changed and advancing would consume a pass for nothing.
func (p Phase) Waiting() bool {
	return p == ReadyToRehearse || p == ReviewingEdges || p == Naming
}

// Refusal is the CLOSED vocabulary of why a teach session stopped.
//
// Discrete, like every other refusal in this repository. There is no teaching score: either
// Marco can say what it saw or it cannot.
type Refusal string

const (
	// NoObservation is a pass that produced nothing to read — the window went, or the
	// sampler failed. Distinct from every "I looked and could not tell" below.
	NoObservation Refusal = "no_observation"
	// NoSubject is a session that never found anything to watch but Marco itself.
	//
	// Distinct from NoObservation, which is a pass that ran and produced nothing: this one
	// never started, because the person never went to an application. It is the honest
	// reading of "I pressed Start and then did not demonstrate anything".
	NoSubject Refusal = "no_subject"
	// NothingChanged is a discovery pass in which the screen never moved at all.
	NothingChanged Refusal = "nothing_changed"
	// DestinationNotRecognised is a screen that changed to somewhere Marco cannot recognise
	// again later. Deliberately NOT the same as NothingChanged.
	DestinationNotRecognised Refusal = "destination_not_recognised"
	// SeveralRoutes is several durable routes in one pass, none of which ends where the
	// person stopped — so Marco cannot tell which outcome the demonstration was for.
	//
	// `start_not_recognised` and `left_the_start` used to sit beside this. Both were the
	// route-centric conflation — refusing a whole attempt because the demonstration did not
	// anchor to the dwelled-on start — and both are gone: the destination is the goal, and
	// where a demonstration began is evidence, never a gate.
	SeveralRoutes Refusal = "several_routes"
	// RouteNotRemembered is the store refusing to hold a request for this route.
	RouteNotRemembered Refusal = "route_not_remembered"
	// NotArmed is a demonstration pass in which no capture ran. The mutation target: if the
	// arming stops working, this is what the user sees, and it is never silent.
	NotArmed Refusal = "not_armed"
	// DemonstrationIncomplete carries the capture's own reason.
	DemonstrationIncomplete Refusal = "demonstration_incomplete"
	// RequiresTextEntry is the standing policy: what the user types is not observed, so a
	// procedure that depends on it cannot be reproduced.
	RequiresTextEntry Refusal = "requires_text_entry"
	// ActionNotAttributed is "I saw where you ended up, but not what you did".
	ActionNotAttributed Refusal = "action_not_attributed"
	// NotAssessable is a demonstration no assessment came back for.
	NotAssessable Refusal = "not_assessable"
	// DemonstrationsDisagree is two examples of one route that are not the same procedure.
	// Teach does not pick a winner and does not average them.
	DemonstrationsDisagree Refusal = "demonstrations_disagree"
	// EvidenceInsufficient is an assessment that did not come out consistent and that no
	// further demonstration would resolve.
	EvidenceInsufficient Refusal = "evidence_insufficient"
	// ApplicationChanged is a pass that watched somewhere other than where it began.
	ApplicationChanged Refusal = "application_changed"
	// NameNotUsable is a requested play name Marco cannot write down.
	NameNotUsable Refusal = "name_not_usable"
	// GoalNotRemembered is the durable store refusing to bind this name to this outcome —
	// most often a name that already means reaching somewhere else.
	GoalNotRemembered Refusal = "goal_not_remembered"
	// ExamplesExhausted is the bound on how many times Teach will ask for one more.
	ExamplesExhausted Refusal = "examples_exhausted"
	// RehearsalDeclined is the user saying no, or not now, to trying it once.
	RehearsalDeclined Refusal = "rehearsal_declined"
	// RehearsalRefused is Marco declining to act BEFORE emitting anything.
	RehearsalRefused Refusal = "rehearsal_refused"
	// RehearsalNotStarted is the Audience saying YES and Marco never obtaining authority.
	//
	// # Why this is its own outcome
	//
	// It used to be reported as `rehearsal_declined` — "Alright, I won.t try it" — which
	// tells somebody who just consented that they refused. Live, twice, and the internal
	// reason at the time already read "a yes was given and created no authority".
	//
	// Consent and authority are different facts. The Audience owns the first and Marco owns
	// the second, and a failure of Marco.s half may never be reported as a decision of
	// theirs. See ADR-077.
	RehearsalNotStarted Refusal = "rehearsal_not_started"
	// RehearsalFailed is an attempt that ran and did not complete.
	RehearsalFailed Refusal = "rehearsal_failed"
	// NotLowerable is a play the lowering judgement will not write down.
	NotLowerable Refusal = "not_lowerable"
	// NameRefused is a screen name the durable store would not accept.
	NameRefused Refusal = "name_refused"
	// SaveFailed is a play that could not be written.
	SaveFailed Refusal = "save_failed"
	// PlayNotRegistered is a play written down that nothing can ask for.
	//
	// Saved and registered are different places on purpose — `<app>/learned/` is invisible to
	// the resolver, which is what makes the two impossible to confuse. Completion claims the
	// Audience can ask for the play later, so completion requires both.
	PlayNotRegistered Refusal = "play_not_registered"
	// AnswerTimedOut is a question nobody answered inside the bound.
	AnswerTimedOut Refusal = "answer_timed_out"
	// NoTail is a Director with no lifecycle behind it. Only reachable in a partial wiring,
	// and named rather than silent.
	NoTail Refusal = "no_tail"
)

// MaxExamples bounds how many demonstrations one teach session will ask for.
//
// Two. Existing policy wants a second example to corroborate the first; a third is a session that
// has stopped converging, and asking indefinitely is how a teaching flow becomes a chore.
const MaxExamples = 2

// Bounds are how long each kind of pass watches.
//
// Separate numbers because they are separate questions. Establishing a place needs a handful of
// observations of somebody holding still; watching a demonstration has to allow for a person
// finding what they are doing.
type Bounds struct {
	// Dwell is the establishing pass: "hold still a moment".
	Dwell time.Duration
	// Watch is a discovery or demonstration pass.
	Watch time.Duration
	// Answer is how long a question may go unanswered before the attempt gives up.
	//
	// A teach session is ephemeral and holds a window under observation; one left open
	// overnight waiting for somebody to type a screen name is a session nobody meant to
	// leave running. Giving up is not a refusal of the evidence — everything durable that was
	// already written stays written.
	Answer time.Duration
}

// DefaultBounds are the conservative defaults.
func DefaultBounds() Bounds {
	return Bounds{Dwell: 6 * time.Second, Watch: 45 * time.Second, Answer: 10 * time.Minute}
}

// MaxNameRounds bounds how many screens one play will ask the user to name.
//
// Three. A route has two endpoints; the third is slack for a judgement that finds another demand
// after a recompute. A coordinator that asked forever would be a coordinator with a bug nobody
// could see.
const MaxNameRounds = 3

// Passes runs one bounded observation pass against the window Teach was started for.
//
// The ENTIRE perception and session chain sits behind this. Teach cannot start a session of its
// own, choose a window, take a sample or read a pixel — which is what lets the boundary test hold
// for the coordinator as well as for the runner.
type Passes interface {
	Observe(ctx context.Context, d time.Duration) (observesession.Result, error)
	// Finish ends the pass that is running NOW, keeping everything it has seen.
	//
	// Distinct from cancelling the context, which abandons the attempt. This is the person
	// saying "I have finished showing you" — see Coordinator.Finish — so the evidence is the
	// point of it and must survive. A no-op when no pass is running.
	Finish()
	// AwaitSubject blocks until there is something to watch that is not Marco itself, and
	// fixes it as the window this attempt is about.
	//
	// The coordinator cannot choose a window, look at one, or tell one program from another,
	// and it must not learn how — so the whole of "wait for the person to go to the
	// application" lives on this side of the seam. All the coordinator knows is that until
	// this returns there is nothing worth watching. See WaitingForDemonstration.
	//
	// Returns an error only when it can never succeed; a caller's cancelled context is
	// reported as ctx.Err().
	AwaitSubject(ctx context.Context) error
}

// Grounding turns a screen Teach has just decided about into somewhere on the display.
//
// # Why it is injected rather than done here
//
// The coordinator has the semantics — WHICH screen, and at which moment — and none of the things
// the answer also depends on: the window rectangle the measurement was normalised against, whether
// that rectangle can still be trusted, which display it is on. Those live where the sampler and the
// platform live, and reaching for them from here would put a second copy of the conversion inside
// the one package that must not be able to touch the screen at all.
//
// So Teach says which screen it means and gets back where that currently is, in the ordinary
// window-relative form. It still cannot draw anything, and the boundary test still holds.
//
// # Failure is a returned referent, never an error
//
// There is nothing for a teach session to DO about not being able to point. The referent carries
// its own typed reason, the surface reads it, and the session continues exactly as it would have.
type Grounding interface {
	Ground(t observe.ShadowTotals, application string, state observe.ScreenStateID,
		role observe.ReferentRole) observe.VisualReferent
}

// Session is what a teach attempt looks like from outside.
//
// EPHEMERAL. Nothing here is written anywhere, and a restart ends the attempt: a half-finished
// demonstration resumed days later would be watching somebody who never agreed to it again.
type Session struct {
	// Name is what the user asked to call the behaviour. Their words, held and not
	// interpreted; Director still has to discover what actually happened. It is also the
	// name the durable GOAL is remembered under, verbatim.
	Name string
	// Actor and Verb are the two halves of the sentence a saved play would become, derived
	// from Name by the caller (or given as flags) and validated there.
	//
	// Carried so a surface can say what the play WILL be called before anybody demonstrates
	// anything. A phrase welded into an identifier silently is a developer identifier
	// wearing the user's words; said out loud, it is a name they can correct.
	Actor, Verb string
	// Application is what the first pass turned out to be watching.
	Application string
	Phase       Phase
	Refusal     Refusal
	// Start is the place established before the user was told to go ahead.
	Start string
	// Route is the edge under review right now, empty until discovery has one.
	//
	// For a multi-edge demonstration this MOVES: ReviewingEdges points it at each required
	// edge in turn, so every layer below — the question, the grant, the attempt — goes on
	// working in terms of one route at a time and needed no notion of a sequence.
	Route observe.RelationshipRef
	// Edges is the ordered review of what this demonstration requires, empty for a session
	// that has not got that far. See EdgeReview.
	Edges []EdgeReview
	// StartState and DestinationState are the screens the two decisions were made ON.
	//
	// PINNED at the moment of the decision and never recomputed. They are what makes grounding
	// answerable later — "show me the start again" has to mean the screen the start was
	// established on, not wherever the user is standing when they ask.
	StartState, DestinationState observe.ScreenStateID
	// StartReferent and DestinationReferent are where those screens were, when they were
	// decided. Nil until grounding was attempted; a referent that cannot point carries its own
	// reason and is kept, because "I decided this and cannot show you it" is worth saying.
	StartReferent, DestinationReferent *observe.VisualReferent
	// Examples is how many demonstrations have been captured.
	Examples int
	// Input is what the navigation producer did during the pass that watched.
	//
	// Carried because "I couldn't tell what you did" has three completely different causes —
	// nothing was watching, events arrived and were all discarded, or the classifier fell
	// behind — and the refusal alone reads as the first while usually meaning one of the
	// others. Counts and closed reasons; there is no key in here to leak.
	Input observe.InputStats
	// Uncertain is what specifically was unreadable, when another example is being asked for.
	//
	// Empty on the normal path, because the normal path does not ask. A person who IS asked is
	// owed the part that was unclear rather than a request to do the whole thing again — the
	// difference between "I lost track of one transition" and "show me again".
	Uncertain []observe.AssessmentReason
	// Demonstration and Assessment are the ordinary objects, unmodified.
	Demonstration *observe.ProcedureCandidate
	Assessment    *observe.CandidateAssessment
	// Question is the open proposal the user must answer, nil when there is none.
	//
	// An ADDRESS, not the question: the proposal owns its own wording and Teach does not
	// paraphrase it.
	Question *Question
	// SessionID is the observation session the question belongs to, so a caller can route an
	// answer to the command that already accepts one.
	SessionID observe.SessionID
	// Attempt is what the rehearsal did, nil until one has run.
	Attempt *Attempt
	// Readiness is the most recent lowering judgement, nil until one was asked for.
	Readiness *Readiness
	// Saved is the durable artifact, nil until one exists. NOTHING may claim completion
	// without this: a play Marco says it learned and did not write down is a lie a person
	// discovers tomorrow.
	Saved *Saved
	// Named counts how many screens the user has named for this play.
	Named int
	// Diagnostics is the developer-facing account. Never shown in Normal mode: it names
	// durable subject ids, and a subject id on a user's screen is a leak of Director's
	// backstage into the play.
	Diagnostics []string
}

// Learned reports whether a durable play exists. The ONLY basis for saying so.
func (s Session) Learned() bool { return s.Saved != nil && s.Saved.Saved }

// Coordinator drives one teach attempt.
//
// Not safe for concurrent Advance; the caller serialises, exactly as the observation registry
// serialises sessions. Cancel is safe from another goroutine because cancelling is the one thing
// that has to work while a pass is in flight.
type Coordinator struct {
	passes Passes
	memory observe.Memory
	tail   Tail
	ground Grounding
	th     observe.HypothesisThresholds
	bounds Bounds
	// actor and verb are the two halves of the sentence a saved play becomes. Supplied
	// already validated by the caller, because the naming rules live where Marco is written
	// and the coordinator must not be able to reach that far.
	actor, verb string
	// now is the clock the answer bound is measured against, injected so a test does not wait.
	now func() time.Time
	// waitingSince is when the current question was first put.
	waitingSince time.Time

	s Session
	// cancelled is set by Cancel and checked at every phase boundary, so a cancel that lands
	// mid-pass takes effect the moment the pass returns rather than being overwritten by it.
	cancelled bool
	// finished is set by Finish: the person has said the demonstration is over. Kept apart
	// from `cancelled` because they are opposite instructions — one abandons the evidence
	// and one is the reason the evidence exists.
	finished bool
	// topology is the durable edge counts as they stood before the discovery pass.
	//
	// The diff against this is how Teach knows WHICH route the user just demonstrated. It
	// reads counts the store already keeps; it derives no relationship of its own.
	topology map[observe.RelationshipRef]int
}

// New starts a teach attempt for a name the user supplied.
//
// The name is checked here so an unusable one fails before anybody is asked to demonstrate
// anything. It is BOUND to nothing until a play is written — a failed attempt must not leave the
// user's word attached to something Marco could not learn.
func New(name string, passes Passes, memory observe.Memory, b Bounds) *Coordinator {
	if b.Dwell <= 0 || b.Watch <= 0 {
		b = DefaultBounds()
	}
	if b.Answer <= 0 {
		b.Answer = DefaultBounds().Answer
	}
	c := &Coordinator{
		passes: passes, memory: memory, bounds: b, now: time.Now,
		th: observe.DefaultHypothesisThresholds(),
		s:  Session{Name: strings.TrimSpace(name), Phase: WaitingForDemonstration},
	}
	if err := CheckName(name); err != nil {
		c.refuse(NameNotUsable, err.Error())
	}
	return c
}

// WithTail installs the rest of the lifecycle.
//
// Optional and nil-safe in the sense that its absence is REPORTED rather than hidden: a
// coordinator with no tail reaches `ready_to_rehearse` and then refuses with `no_tail`, which is
// a partial wiring somebody can see.
func (c *Coordinator) WithTail(t Tail) *Coordinator { c.tail = t; return c }

// WithGrounding installs the thing that can say where a screen currently is.
//
// Optional, and its absence is silent rather than reported — unlike the tail. A Director that
// cannot point can still teach perfectly well; it simply cannot show the user what it decided, and
// the surface says so from the missing referent rather than from a refusal here.
func (c *Coordinator) WithGrounding(g Grounding) *Coordinator { c.ground = g; return c }

// WithPlayName supplies the two halves of the sentence a saved play becomes.
//
// `do Downloads's Open …` — an actor and a verb. Both are validated against Marco's naming rules
// by the CALLER, where those rules live; the coordinator only carries them and only uses them at
// save time. A teach attempt that fails must not leave the user's word attached to anything.
func (c *Coordinator) WithPlayName(actor, verb string) *Coordinator {
	c.actor, c.verb = actor, verb
	c.s.Actor, c.s.Verb = actor, verb
	return c
}

// Finish is the person saying their demonstration is over.
//
// # Why this is not Cancel, and not a shorter timer
//
// A demonstration used to end when a clock ran out — `Bounds.Watch`, forty-five seconds — or when
// the person happened to hold still long enough to look finished. Both are guesses about
// something the person knows for certain, and both fail in the two directions that matter: a
// careful demonstration is cut off mid-route, and a quick one leaves the person sitting in front
// of a window doing nothing, wondering whether Marco noticed.
//
// "I am done" is real semantic evidence and the only reliable source of it is the person. So Stop
// ENDS ADMISSION and keeps every single thing already captured — the in-flight pass is finished
// rather than abandoned, which the observation layer already supports because a cancelled session
// keeps its evidence and is recorded as incomplete.
//
// What it deliberately does not do: discard the capture, require the person to have returned to
// where they started, require a circular route, or require them to sit still first. It also does
// not skip anything downstream. The pass returns as usual and the ordinary pipeline runs — the
// destination is established, the demonstration is built, the candidate is assessed — because the
// alternative would be a second, shorter implementation of everything after capture.
func (c *Coordinator) Finish() {
	if c.s.Phase.Settled() || c.finished {
		return
	}
	c.finished = true
	c.note("you said that was the whole demonstration")
	if c.passes != nil {
		c.passes.Finish()
	}
}

// Finished reports whether the person has said the demonstration is over.
func (c *Coordinator) Finished() bool { return c.finished }

// WithClock replaces the clock the answer bound is measured against.
func (c *Coordinator) WithClock(now func() time.Time) *Coordinator { c.now = now; return c }

// Session returns the current state.
func (c *Coordinator) Session() Session { return c.s }

// Cancel ends the attempt.
//
// Nothing partial survives it: no candidate is stored, no request is left pending that would arm
// a capture in a later session, and no play is written. The in-flight pass is stopped by the
// caller cancelling the context it passed to Advance — this only makes the decision durable for
// the phases that come after.
func (c *Coordinator) Cancel() {
	c.cancelled = true
	if c.s.Phase.Settled() {
		return
	}
	c.s.Phase, c.s.Question = Cancelled, nil
	c.note("cancelled by the user")
	// A pending request is withdrawn, or the next ordinary session would arm a capture for a
	// route nobody is teaching any more.
	c.withdraw()
}

// Advance performs the next step and returns the session.
//
// One step per call, and a call may block for the length of a pass. A phase that is Waiting is
// not advanced by time — the user has to answer a question first — and calling anyway is a no-op
// rather than a wasted pass.
func (c *Coordinator) Advance(ctx context.Context) Session {
	if c.cancelled || c.s.Phase.Settled() {
		return c.s
	}
	switch c.s.Phase {
	case WaitingForDemonstration, EstablishingStart:
		c.establishStart(ctx)
	case ReadyForDemo:
		c.discover(ctx)
	case NeedsAnotherExample:
		c.demonstrate(ctx)
	case Evaluating:
		c.evaluate()
	case ReadyToRehearse:
		// The entry condition hands over as soon as there is an ordered route to walk.
		// ReviewingEdges is the only state that may finish the review.
		if len(c.s.Edges) > 0 {
			c.enter(ReviewingEdges)
			c.reviewEdges()
			break
		}
		c.awaitGrant()
	case ReviewingEdges:
		c.reviewEdges()
	case Rehearsing:
		c.rehearse(ctx)
	case WaitingForStart:
		// The same call, again. The grant is intact and its own expiry is the bound, so
		// retrying costs nothing and asserts nothing: if the world is still not ready this
		// lands back here, and if the permission has run out the refusal comes from the
		// authority rather than from a timer of Teach's own.
		c.rehearse(ctx)
	case Naming:
		c.awaitName()
	case Lowering:
		c.lower()
	}
	if c.cancelled && !c.s.Phase.Settled() {
		c.s.Phase = Cancelled
		c.withdraw()
	}
	return c.s
}

// ── the phases ────────────────────────────────────────────────────────────────

// awaitSubject holds the session until there is something to watch that is not Marco.
//
// Nothing is captured and nothing is decided here — see WaitingForDemonstration. The wait itself
// is the platform's, behind Passes.AwaitSubject, so the coordinator still cannot look at a window
// or tell one program from another.
//
// A PRECONDITION of the first real step rather than a step of its own, so it costs no cycle when
// there was never anything to wait for: a session started against a named window, or one begun
// while the person was already in their application, goes straight on to establishing the start.
func (c *Coordinator) awaitSubject(ctx context.Context) bool {
	if c.passes == nil {
		c.refuse(NoObservation, "this Director has nothing that can watch a window")
		return false
	}
	if err := c.passes.AwaitSubject(ctx); err != nil {
		if c.cancelled || ctx.Err() != nil {
			// The person stopped waiting, or the attempt was cancelled. Not a refusal
			// about the evidence, and Advance's own cancellation handling says so.
			return false
		}
		c.refuse(NoSubject, err.Error())
		return false
	}
	c.s.Phase = EstablishingStart
	return true
}

// establishStart waits for the current place to become a durable subject.
//
// EVIDENCE, not a sleep. The pass runs for its bounded length and then the canonical identity
// path is asked where the user is; a place that has not become recognisable in that time is
// refused honestly rather than assumed.
func (c *Coordinator) establishStart(ctx context.Context) {
	// THE gate, not a step: there is nothing to establish a start ON until the person is
	// somewhere that is not Marco. See WaitingForDemonstration.
	if c.s.Phase == WaitingForDemonstration && !c.awaitSubject(ctx) {
		return
	}
	res, ok := c.pass(ctx, c.bounds.Dwell)
	if !ok {
		return
	}
	c.s.Application = res.Session.Application
	// THE identity call, and it is the same function the demonstration capture uses every
	// cycle. Teach constructs no subject and matches no structure.
	p := observe.PlaceNow(res.Stats.Shadow, c.s.Application, c.memory, c.th)
	c.topology = c.edgeCounts()
	c.s.Phase = ReadyForDemo
	if !p.Established() {
		// GOAL-CENTRIC: an unrecognisable start no longer ends the attempt. The capability
		// being learned is the DESTINATION; the start was only ever one known way in, and a
		// person should be able to say "learn this" from wherever they happen to be
		// standing. What is honestly lost is route evidence — an edge needs both endpoints
		// durable, so a demonstration from an unestablished start may teach the goal and no
		// route to it yet — and the account below is what lets a reader see exactly that,
		// rather than a refusal that used to discard the whole attempt.
		c.s.Start = ""
		c.note(fmt.Sprintf(
			"the starting place did not become durable (placed=%v verdict=%q licensed=%v "+
				"established=%q reason=%q); watching anyway — the destination is the goal",
			p.Placed, p.Verdict, res.Places.Licensed, res.Places.Subject, res.Places.Reason))
		return
	}
	c.s.Start = p.Subject
	// The screen the decision was made ON, pinned now. Recomputing it later would resolve
	// against wherever the user had wandered to, and the highlight would then confirm whatever
	// they were looking at rather than what Marco decided.
	c.s.StartState = res.Stats.Shadow.CurrentState
	c.note("start established as " + p.Subject)
	// LAST, and deliberately after the phase has already moved. Grounding is a PICTURE of a
	// decision that has already been made: it cannot establish the identity, cannot refuse the
	// session, and cannot advance it. Nothing below this line is allowed to read its result.
	c.s.StartReferent = c.showing(res, c.s.StartState, observe.ReferentTeachStart)
}

// discover watches one ordinary pass and reads the route out of the durable topology.
//
// # Why the topology rather than this session's transitions
//
// Because the durable topology is the thing that had to survive. An edge appears there only when
// BOTH endpoints resolved to remembered subjects — which is the same requirement a learned play's
// guards will have to meet later. Reading anything weaker here would let Teach accept a route it
// could never afterwards write down, and the user would find that out at the end.
func (c *Coordinator) discover(ctx context.Context) {
	c.s.Phase = Capturing
	res, ok := c.pass(ctx, c.bounds.Watch)
	if !ok {
		return
	}
	// What the navigation producer managed during the pass that watched, carried BEFORE any
	// refusal below can return.
	//
	// It used to be set only on the success path, which is precisely backwards: the input
	// account is what separates "you pressed nothing", "nothing was listening" and "your
	// clicks named no control", and every one of those ends in a refusal. A live run
	// refused `destination_not_recognised` with the pointer counters sitting unread in the
	// result — the diagnosis existed one layer down and nothing carried it up, for the
	// fifth time in this subsystem.
	c.s.Input = res.Stats.Shadow.Input
	if !strings.EqualFold(res.Session.Application, c.s.Application) &&
		res.Session.Application != "" {
		c.refuse(ApplicationChanged, "the pass ended on "+res.Session.Application+
			", not "+c.s.Application)
		return
	}
	c.s.Phase = EstablishingDestination

	grew := c.grownEdges()
	if len(grew) == 0 {
		// The two silences are different, and the report says which.
		switch {
		case res.Relationships.SessionLocal > 0:
			// The causes travel with the count. Which of them it is decides whether this
			// is a recognition problem, a self-loop, or a change that crossed a frame
			// nobody could place — and the bare count sent all three to the same sentence.
			c.refuse(DestinationNotRecognised, fmt.Sprintf(
				"%d transition(s) were seen and none had two recognisable endpoints: %s",
				res.Relationships.SessionLocal, res.Relationships.Why()))
		default:
			c.refuse(NothingChanged, "no screen change was observed during the pass")
		}
		return
	}
	// GOAL-CENTRIC route selection. The route the tail carries forward is the TERMINAL leg —
	// the edge that arrived where the person stopped — and the runner already identified it
	// when it built the one-shot demonstration. It is NOT "the edge that began at the
	// start": a demonstration is evidence about reaching the destination, and requiring it
	// to begin where the dwell pass happened to establish was the route-centric conflation
	// this milestone removes. Every other grown edge is already durable route knowledge in
	// its own right, whatever happens to this session's tail.
	c.recordRequiredEdges(res)
	switch {
	case res.Demonstration != nil:
		c.s.Route = res.Demonstration.Relationship
	case len(grew) == 1:
		c.s.Route = grew[0]
	default:
		// Several legs grew and none of them ends where the person is standing, so Marco
		// cannot say which outcome the demonstration was FOR. The edges themselves are
		// already remembered; only the naming of a goal has to stop here.
		c.refuse(SeveralRoutes, fmt.Sprintf(
			"%d routes became durable and none of them ends where you stopped; Marco "+
				"cannot tell which outcome you meant", len(grew)))
		return
	}

	// THE arming, and it is the ORDINARY pending-learning request — the same durable object a
	// user's yes to "shall I learn this?" writes. The store refuses a route it does not hold,
	// which is what makes "a place Marco cannot recognise cannot be taught" a property of the
	// store rather than of a check here.
	//
	// SKIPPED when the session already produced a candidate. A request left pending after a
	// one-shot demonstration succeeded would arm a capture in the next ordinary session —
	// watching somebody who was never asked again — which is the failure `FulfilLearning`
	// exists to prevent, reached from the other side.
	//
	// Deleting this call must fail TestTeachArmsTheExistingCaptureForTheDiscoveredRoute.
	if res.Demonstration == nil {
		if err := c.memory.RememberLearning(c.s.Application, c.s.Route,
			observe.LearningRequest{Status: observe.LearningPending}); err != nil {
			c.refuse(RouteNotRemembered, err.Error())
			return
		}
	}
	// THE goal write. What was learned is the OUTCOME, in the person's own words, bound to
	// the destination subject — never to the route, never to the start. It happens here,
	// the moment the destination is known, because it is knowledge about what the person
	// wants and owes nothing to whether the tail's rehearsal later succeeds: a goal with no
	// verified route yet is exactly the honest state "I know what you want to reach, but I
	// don't yet know how to get there from here" describes.
	//
	// Probed as an optional interface so every Memory fake keeps compiling; the production
	// store implements it. Deleting this write must fail
	// TestTeachingRecordsTheDestinationAsAGoal.
	if gs, ok := c.memory.(observe.GoalStore); ok {
		if err := gs.RememberGoal(c.s.Application, observe.Goal{
			Name: c.s.Name, Subject: c.s.Route.To,
		}); err != nil {
			// One name, one outcome. A conflict is the person's to resolve, and burying it
			// would leave them with a goal that silently means something else.
			c.refuse(GoalNotRemembered, err.Error())
			return
		}
		c.note("goal remembered: " + quote(c.s.Name) + " reaches " + c.s.Route.To)
	}

	c.s.Phase = NeedsAnotherExample
	c.s.DestinationState = res.Stats.Shadow.CurrentState
	c.note("route discovered: " + c.s.Route.From + " → " + c.s.Route.To)
	// Same rule as the start: after the route is durable, after the capture is armed, after
	// the phase has moved. A destination that could not be pointed at is still the destination.
	c.s.DestinationReferent = c.showing(res, c.s.DestinationState,
		observe.ReferentTeachDestination)

	// THE one-shot path, READ rather than built. The pass that just ran watched the
	// demonstration and the RUNNER made the candidate from it — in the session, beside the
	// store it goes into and the review that turns it into the question a person answers.
	//
	// Teach built it here once, and that was the defect: a candidate constructed after the
	// session ended is invisible to the only stage that can raise `AskRehearse`, so teaching
	// reached "want me to try?" and waited for a grant nobody could give.
	// See [[ADR-054-the-one-shot-candidate-belongs-to-the-session]].
	if res.Watched != "" {
		// The runner declined to build one from what it watched. Said out loud: the
		// armed capture is about to take over, and if THAT also fails the person would
		// otherwise be told "that example did not finish" about an example nobody tried
		// to build.
		c.note("the watched pass produced no demonstration: " + string(res.Watched))
	}
	if res.Demonstration != nil {
		c.s.Demonstration, c.s.Assessment = res.Demonstration, res.Assessment
		c.recordRequiredEdges(res)
		c.s.Examples++
		c.s.Phase = Evaluating
		c.note("the pass that watched it produced the demonstration")
		c.evaluate()
	}
}

// showing asks where a screen currently is, and can do nothing else.
//
// # The property, and the reason it is a separate function
//
// It returns a value. It has no path to the phase, the refusal, the route or the question, so a
// grounding that fails cannot establish an identity, change an assessment, advance the session or
// grant authority — not because those are checked afterwards, but because there is nothing here
// that could reach them. Every caller assigns the result to one field and reads it no further.
//
// A Director with no grounding wired gets nil, which the surfaces render as "I can't show you"
// rather than as an error. Teaching does not depend on being able to point.
//
// Deleting the nil guard, or letting a caller branch on the result, must fail
// TestGroundingFailureDoesNotChangeAnythingAboutTheTeachSession.
func (c *Coordinator) showing(res observesession.Result, state observe.ScreenStateID,
	role observe.ReferentRole) *observe.VisualReferent {

	if c.ground == nil {
		return nil
	}
	v := c.ground.Ground(res.Stats.Shadow, c.s.Application, state, role)
	return &v
}

// demonstrate watches the pass in which the armed capture records the example.
//
// Teach arms nothing here and observes nothing here. The runner finds the pending request the
// discovery step wrote and does exactly what it does for a request the user answered yes to.
func (c *Coordinator) demonstrate(ctx context.Context) {
	c.s.Phase = Capturing
	res, ok := c.pass(ctx, c.bounds.Watch)
	if !ok {
		return
	}
	c.s.SessionID = res.Session.ID
	if res.Demonstration == nil {
		// The arming did not take. Never silent: a teaching flow that quietly watched
		// nothing would be indistinguishable from one that worked.
		c.refuse(NotArmed, "no demonstration was captured; the pending request for "+
			c.s.Route.From+" → "+c.s.Route.To+" did not arm a capture")
		return
	}
	c.s.Demonstration = res.Demonstration
	c.s.Assessment = res.Assessment
	c.s.Examples++
	c.s.Phase = Evaluating
	c.evaluate()
}

// evaluate reads the ordinary assessment and decides what to ask for next.
//
// It implements no scoring. Every branch below is a question the assessment already answered;
// Teach only chooses which sentence a person hears.
func (c *Coordinator) evaluate() {
	d := c.s.Demonstration
	if d == nil {
		c.refuse(NotAssessable, "there is no demonstration to judge")
		return
	}
	if !d.Complete {
		c.refuse(DemonstrationIncomplete, "the capture ended: "+string(d.Reason))
		return
	}
	// The typed-text boundary, restated where the user can see it. Marco deliberately does not
	// retain what is typed, so a procedure that depends on it cannot be reproduced — and
	// pretending otherwise would produce a play that fails the first time it is run.
	for _, s := range d.Steps {
		if s.RequiresTextEntry {
			c.refuse(RequiresTextEntry,
				"a step crossed a screen offering somewhere to type")
			return
		}
	}
	// Part 15's distinction, and it is a real one: knowing where somebody ended up is not
	// knowing what they did. Most desktop software is mouse-driven and a pointer press is
	// observed but not yet nameable, so this is the refusal a live session is most likely to
	// meet — and it must not read as "I could not see the screen change".
	presses := 0
	for _, s := range d.Steps {
		presses += len(s.Intents)
	}
	if presses == 0 {
		c.refuse(ActionNotAttributed, "the destination was reached with no navigation "+
			"attributed to the person")
		return
	}
	a := c.s.Assessment
	if a == nil {
		c.refuse(NotAssessable, "the demonstration produced no assessment")
		return
	}
	// ONE demonstration is the normal path. Another is asked for only when something was
	// UNREADABLE — never merely because there has been one of them.
	//
	// `Blocking()` rather than `NeedsAnotherDemonstration()`: the latter answers "would another
	// example reduce uncertainty", which is true of `single_demonstration_only` and always will
	// be. The question here is narrower and is the one that should cost a person their time:
	// must this be closed before Marco may offer to try? See
	// [[ADR-051-one-demonstration-and-an-attempt]].
	if blocking := a.Blocking(); len(blocking) > 0 {
		if c.s.Examples >= MaxExamples {
			c.refuse(ExamplesExhausted, fmt.Sprintf(
				"%d example(s) still left %v open", c.s.Examples, blocking))
			return
		}
		c.s.Phase = NeedsAnotherExample
		// The SPECIFIC uncertainty, not the whole assessment. A person asked for more
		// evidence is owed the part that was unclear.
		c.s.Uncertain = blocking
		c.note("another example would close: " + reasonList(blocking))
		return
	}
	c.s.Uncertain = nil
	// The VERDICT is the assessment's word and Teach does not overrule it. An ambiguous or
	// invalid demonstration that another example would not fix is refused here rather than
	// carried forward, because the next thing Teach would otherwise say is "want me to try
	// it?" — which is a question about acting, asked on evidence that did not hold up.
	if a.Verdict != observe.CandidateConsistent {
		if hasReason(a, observe.ReasonDemonstrationsDisagree) {
			c.refuse(DemonstrationsDisagree,
				"the examples were compared and did not describe the same procedure")
			return
		}
		c.refuse(EvidenceInsufficient, string(a.Verdict)+": "+reasonsOf(a))
		return
	}
	// Everything beyond here needs something only the user can give — permission to try, or a
	// word for a screen — and Teach creates neither. It hands over to the questions that
	// already exist.
	c.note("assessment: " + string(a.Verdict) + " " + reasonsOf(a))
	// READY TO REHEARSE remains the entry condition. What happens next depends on whether
	// this demonstration has an ordered route to review: one edge behaves exactly as it
	// always did, and several are handed to the phase that owns a sequence.
	c.enter(ReadyToRehearse)
	// Pick the question up straight away, so the moment a person is told "want me to try
	// it?" they can also be told how to say yes. Guarded rather than refusing here: whether
	// this Director has a lifecycle behind it is the NEXT step's problem to report, and
	// reporting it during the assessment would attach the wrong reason to the wrong phase.
	if c.tail != nil {
		c.awaitGrant()
	}
}

// ── the tail ──────────────────────────────────────────────────────────────────

// awaitGrant waits for the user to answer the EXISTING rehearsal question.
//
// It creates no question, answers none, and has no way to. Authority appears here only because
// somebody said yes through the ledger — silence, a decline, a not-now and a malformed answer all
// leave `Granted` false, which is the fail-closed reading this system already proved.
func (c *Coordinator) awaitGrant() {
	if !c.haveTail() {
		return
	}
	if q, ok := c.tail.Question(c.s.Route, observe.AskRehearse); ok {
		c.s.Question, c.s.SessionID = &q, questionSession(q, c.s.SessionID)
	} else if d, ok := c.tail.(QuestionDiagnoser); ok {
		// NO QUESTION IS A SILENCE WITH CAUSES, and the tail knows which.
		//
		// Live, twice: the phase said "waiting for permission", the sentence said "Want me
		// to try?", and there was no proposal to answer because the single-question budget
		// had gone to another route. Nothing said so, in any surface, for as long as the
		// session lasted.
		//
		// Recorded once rather than every cycle — awaitGrant runs on each Advance, and a
		// diagnostic repeated fifty times is a diagnostic nobody reads.
		//
		// Deleting this must fail TestNoQuestionSaysWhyThereIsNoQuestion.
		if why := d.QuestionRefusal(c.s.Route, observe.AskRehearse); why != "" {
			note := "no rehearsal question: " + why
			if len(c.s.Diagnostics) == 0 ||
				c.s.Diagnostics[len(c.s.Diagnostics)-1] != note {
				c.note(note)
			}
		}
	}
	if c.tail.Granted(c.s.Route) {
		c.note("the user authorised one rehearsal")
		c.s.Question = nil
		c.enter(Rehearsing)
		return
	}
	// A yes that created nothing is a different silence from no answer at all, and it used
	// to be indistinguishable: every cause was a silent return, the person had answered, and
	// the timeout below blamed them for it. The reason is carried into the diagnostics and
	// into the refusal a timeout produces.
	why := ""
	if d, ok := c.tail.(GrantDiagnoser); ok {
		why = d.GrantRefusal(c.s.Route)
	}
	if why != "" && (len(c.s.Diagnostics) == 0 ||
		c.s.Diagnostics[len(c.s.Diagnostics)-1] != "a yes created no authority: "+why) {
		c.note("a yes created no authority: " + why)
	}
	// THE OUTCOME FOLLOWS WHAT THE AUDIENCE ACTUALLY DECIDED.
	//
	// Every one of these used to end as `rehearsal_declined` — "Alright, I won.t try it" —
	// whatever had happened. Somebody who said YES was told they had refused, twice live,
	// while the diagnostic beside it already read "a yes was given and created no authority".
	//
	// Consent is the Audience.s and authority is Marco.s. A timeout is an observation about
	// what happened AFTER the question, and it may not reinterpret the answer.
	//
	// Deleting the branch must fail TestAYesIsNeverReportedAsADecline.
	answer, settled := observe.ResponseNone, false
	if a, ok := c.tail.(RehearsalAnswer); ok {
		answer, settled = a.AnswerToRehearsal(c.s.Route)
	}
	consented := settled && answer == observe.ResponseConfirmed
	// A GRANT REFUSAL IS ITSELF EVIDENCE OF CONSENT: it reports why the most recent YES
	// created no authority, and is empty when none was given.
	if why != "" {
		consented = true
	}
	switch {
	case consented:
		said := "you said yes and it created no authority"
		if why != "" {
			said = "you said yes and it created no authority (" + why + ")"
		}
		c.timeOut(RehearsalNotStarted, said)
	case settled:
		c.timeOut(RehearsalDeclined, "the answer to this was not a yes")
	default:
		c.timeOut(AnswerTimedOut,
			"nobody answered within "+c.bounds.Answer.String())
	}
}

// GrantDiagnoser is the optional half of Tail.Granted: why the most recent yes created no
// authority, empty when it created one or when nobody has answered.
//
// Optional so a Tail that cannot know — every test stub, and any composition that does not
// track it — keeps exactly the behaviour it had. A tail that CAN know owes the person the
// reason, because "nobody authorised a rehearsal" is false when somebody did and it silently
// failed.
type GrantDiagnoser interface {
	GrantRefusal(route observe.RelationshipRef) string
}

// rehearse runs the EXISTING rehearsal state machine and reports what it did.
//
// Teach performs nothing. It does not choose steps, does not retry, does not recover, and cannot
// reach anything that emits input — the boundary test holds that. What it does is read the result
// and refuse to overstate it: a rehearsal that did not complete does not become a play.
func (c *Coordinator) rehearse(ctx context.Context) {
	if !c.haveTail() {
		return
	}
	a, err := c.tail.Rehearse(ctx)
	if c.cancelled {
		return
	}
	if err != nil {
		c.refuse(RehearsalFailed, err.Error())
		return
	}
	c.s.Attempt = &a
	switch {
	case !a.Attempted && notReadyYet(a.Refusal):
		// THE PATIENT CASE. Marco could not see the starting screen, or was looking at a
		// different one. Nothing was emitted and — this is the part that matters — the grant
		// was never claimed: `BeginAttempt` compares scope BEFORE it spends, and these
		// refusals happen earlier still.
		//
		// So the permission is intact and the only honest thing to do is wait for the world.
		// The demonstration capture has behaved this way since it was written: armed, and
		// patient until the user is standing on the start. A rehearsal that fired once on
		// the instant of approval and gave up made a person choreograph their way back to a
		// screen at a second they could not predict — which is test scaffolding, not a
		// product. See [[ADR-055-an-authorised-rehearsal-waits-for-its-start]].
		//
		// Bounded by the grant's own expiry, not by anything new here.
		c.s.Phase = WaitingForStart
		// RECORDED ONCE PER REASON, not once per cycle.
		//
		// A patient rehearsal re-attempts on every Advance and refuses for the same reason
		// every time. Live, that wrote `waiting for the start: window_not_in_front` ten
		// times in a row and buried the lines that actually explained the run — the route,
		// the grant, and the one sentence saying what the attempt refused. A diagnostic
		// repeated ten times is a diagnostic nobody reads.
		//
		// Deleting this must fail TestARepeatedWaitIsRecordedOnce.
		note := "waiting for the start: " + a.Refusal
		if len(c.s.Diagnostics) == 0 || c.s.Diagnostics[len(c.s.Diagnostics)-1] != note {
			c.note(note)
		}
	case !a.Attempted:
		// Marco declined BEFORE emitting anything, for a reason waiting will not change.
		// A different fact from trying and failing, and the vocabulary keeps them apart.
		//
		// It ends THIS EDGE, not the episode. A route whose second leg cannot be tried is
		// partial — the first leg is still verified and still durable — and saying so is
		// more use than refusing the whole demonstration.
		if c.resolveEdge(EdgeRefused, "declined before acting: "+a.Refusal) {
			return
		}
		c.refuse(RehearsalRefused, "declined before acting: "+a.Refusal)
	case !a.Completed:
		if c.resolveEdge(EdgeRefused, "the attempt ended: "+a.Terminal) {
			return
		}
		c.refuse(RehearsalFailed, "the attempt ended: "+a.Terminal)
	default:
		c.note("rehearsal completed (" + a.Terminal + ")")
		// BACK TO THE REVIEW, not on to the play. A demonstration of A → B → C has a
		// second leg waiting, and advancing here is exactly what left it unoffered.
		// reviewEdges is the only thing that may finish the review.
		if c.resolveEdge(EdgeVerified, "") {
			return
		}
		c.enter(Lowering)
	}
}

// notReadyYet reports whether a pre-input refusal is about the WORLD rather than about the
// authorisation or about Marco.
//
// A CLOSED set, and every member has the same two properties: nothing was emitted, and the grant
// was not claimed — `BeginAttempt` compares scope before it spends, and these are raised before it
// is reached. Waiting is therefore free, and it is the only response that does not throw away a
// permission the person already gave.
//
// Everything else is terminal on purpose. `no_actuator` is a fact about how this Director was
// built and no amount of waiting fixes it; a spent, revoked, expired or mismatched grant is the
// authority saying no, and retrying against it would be a loop asking the same forbidden question.
func notReadyYet(refusal string) bool {
	switch refusal {
	case "source_unobservable", "source_unrecognised", "source_ambiguous",
		"target_lost", "target_moved":
		return true
	case "window_not_in_front":
		// The watched window exists, holds the right screen, and is simply not in front —
		// almost always because the person's yes was typed somewhere else, and the window
		// comes forward the moment they click back into it. Raised before the grant is
		// claimed, like everything in this set, so waiting spends nothing.
		return true
	case "source_mismatch":
		// The person is standing on a screen Marco knows, and it is not the one the route
		// begins at. That is the SAME situation as `source_unrecognised` — somewhere else —
		// and it was refusing terminally only because it is raised by a different check.
		//
		// Distinct from the mismatches below it in `BeginAttempt`, which stay terminal:
		// application, relationship and evidence mismatches mean the grant is for something
		// else entirely and no amount of walking around will make it fit. A source mismatch
		// is a person two clicks away from the start, which is the ordinary case.
		//
		// Safe for the same reason as the rest: scope is compared BEFORE the grant is
		// claimed, so nothing has been spent.
		return true
	}
	return false
}

// lower recomputes whether the play may be written down, and asks for a name when it may not.
//
// The judgement is the ORDINARY one, recomputed from durable memory on every call. Teach sets no
// flag, promotes nothing, and does not remember a previous answer — a play that was lowerable a
// minute ago may not be now, and asking again is cheaper than being wrong.
func (c *Coordinator) lower() {
	if !c.haveTail() {
		return
	}
	r, err := c.tail.Lowering(c.s.Route)
	if err != nil {
		c.refuse(NotLowerable, err.Error())
		return
	}
	c.s.Readiness = &r
	switch {
	case r.Eligible:
		c.save()
	case r.Unnamed != "":
		if c.s.Named >= MaxNameRounds {
			c.refuse(NotLowerable, "still unnamed after "+itoa(c.s.Named)+" name(s)")
			return
		}
		if q, ok := c.tail.Question(c.s.Route, observe.AskNameScreen); ok {
			c.s.Question, c.s.SessionID = &q, questionSession(q, c.s.SessionID)
		}
		c.note("waiting for a name for " + r.Unnamed)
		c.enter(Naming)
	default:
		c.refuse(NotLowerable, strings.Join(r.Refusals, ", "))
	}
}

// awaitName waits for the user's own word, then recomputes.
//
// It watches the JUDGEMENT rather than the answer: when the screen it was waiting on is no longer
// the one lowering demands, a name landed. That is deliberate — there is no local queue and no
// second copy of "which screen still needs naming", so a name answered through any surface at all
// moves this forward.
func (c *Coordinator) awaitName() {
	if !c.haveTail() {
		return
	}
	waitingOn := ""
	if c.s.Readiness != nil {
		waitingOn = c.s.Readiness.Unnamed
	}
	r, err := c.tail.Lowering(c.s.Route)
	if err != nil {
		c.refuse(NotLowerable, err.Error())
		return
	}
	if r.Unnamed == waitingOn && !r.Eligible {
		c.timeOut(NameRefused, "no name arrived within "+c.bounds.Answer.String())
		return
	}
	c.s.Named++
	c.s.Question, c.s.Readiness = nil, &r
	c.note("a name landed; recomputing")
	c.enter(Lowering)
	c.lower()
}

// save writes the play, and ONLY then may anything say it was learned.
func (c *Coordinator) save() {
	// THE WHOLE ROUTE the Audience demonstrated, not the leg the episode happens to be
	// pointing at. `Session.Route` is the edge under review; the behaviour is every verified
	// edge in walk order, and saving one of them writes down a play that begins in the middle.
	//
	// Deleting the walk must fail TestSavingAMultiEdgeRouteWritesTheWholeWalk.
	s, err := c.saveWalk()
	if err != nil {
		c.refuse(SaveFailed, err.Error())
		return
	}
	if !s.Saved {
		c.refuse(SaveFailed, "the play was not written")
		return
	}
	// SAVED IS NOT ASKABLE. The completion sentence says "you can ask me to do it later",
	// and a play that is written down but not registered lives where the resolver cannot see
	// it — `marco routes` reports "No routes yet" for a capability Marco just claimed.
	//
	// The artifact is kept: it is readable, editable and correct, and deleting it to hide a
	// registration failure would destroy work to make a message tidy. What is refused is the
	// CLAIM.
	//
	// Deleting this must fail TestAPlayIsNotCalledAskableUntilItIsRegistered.
	if !s.Registered {
		c.s.Saved = &s
		why := "the play was written down as " + s.Name + " and nothing can ask for it yet"
		// WHY, when the persistence path said why. A dead end a person can act on — "that
		// name is already taken; rename it or remove the other one" — is a different thing
		// from one they cannot.
		if s.Reason != "" {
			why += ": " + s.Reason
		}
		c.refuse(PlayNotRegistered, why)
		return
	}
	c.s.Saved = &s
	c.s.Phase, c.s.Question = Complete, nil
	c.note("saved as " + s.Name + " (registered=" + boolWord(s.Registered) + ")")
}

// enter moves to a phase and restarts the answer clock if it is one that waits.
func (c *Coordinator) enter(p Phase) {
	c.s.Phase = p
	if p.Waiting() {
		c.waitingSince = c.now()
	}
}

// timeOut refuses a wait that has gone on past the bound, and does nothing before then.
func (c *Coordinator) timeOut(r Refusal, why string) {
	if c.waitingSince.IsZero() {
		c.waitingSince = c.now()
		return
	}
	if c.now().Sub(c.waitingSince) >= c.bounds.Answer {
		c.refuse(r, why)
	}
}

// haveTail refuses honestly when the lifecycle behind Teach was never wired.
func (c *Coordinator) haveTail() bool {
	if c.tail != nil {
		return true
	}
	c.refuse(NoTail, "this Director has no learning lifecycle wired behind teaching")
	return false
}

func questionSession(q Question, current observe.SessionID) observe.SessionID {
	if q.SessionID != "" {
		return q.SessionID
	}
	return current
}

func boolWord(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// ── the machinery underneath ──────────────────────────────────────────────────

// pass runs one bounded observation and reports whether the session may continue.
func (c *Coordinator) pass(ctx context.Context, d time.Duration) (observesession.Result, bool) {
	res, err := c.passes.Observe(ctx, d)
	if c.cancelled {
		return res, false
	}
	if err != nil {
		c.refuse(NoObservation, err.Error())
		return res, false
	}
	if res.Stats.SamplesTaken == 0 {
		c.refuse(NoObservation, "the pass produced no samples: "+res.Session.Reason)
		return res, false
	}
	return res, true
}

// edgeCounts snapshots the durable observation count of every edge in this application.
func (c *Coordinator) edgeCounts() map[observe.RelationshipRef]int {
	out := map[observe.RelationshipRef]int{}
	if c.memory == nil {
		return out
	}
	for _, rel := range c.memory.Topology(c.s.Application).Relationships {
		out[observe.RelationshipRef{From: rel.From, To: rel.To}] = rel.Observations
	}
	return out
}

// grownEdges is which durable routes gained observations during the discovery pass.
func (c *Coordinator) grownEdges() []observe.RelationshipRef {
	var out []observe.RelationshipRef
	for ref, n := range c.edgeCounts() {
		if n > c.topology[ref] {
			out = append(out, ref)
		}
	}
	// Deterministic, so a session with two candidate routes always refuses the same way.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && less(out[j], out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func less(a, b observe.RelationshipRef) bool {
	if a.From != b.From {
		return a.From < b.From
	}
	return a.To < b.To
}

// withdraw cancels a pending demonstration request this attempt created.
//
// Fail-closed. A request left pending would arm a capture in every later ordinary session, which
// is watching somebody who has not been asked — the exact failure `FulfilLearning` exists to
// prevent on the success path.
func (c *Coordinator) withdraw() {
	if c.memory == nil || c.s.Route.From == "" {
		return
	}
	_ = c.memory.RememberLearning(c.s.Application, c.s.Route, observe.LearningRequest{
		Status: observe.LearningDeclined,
	})
}

func (c *Coordinator) refuse(r Refusal, why string) {
	c.s.Phase, c.s.Refusal, c.s.Question = Refused, r, nil
	c.note(string(r) + ": " + why)
	c.withdraw()
}

func (c *Coordinator) note(s string) { c.s.Diagnostics = append(c.s.Diagnostics, s) }

func hasReason(a *observe.CandidateAssessment, want observe.AssessmentReason) bool {
	for _, r := range a.Reasons {
		if r == want {
			return true
		}
	}
	return false
}

func reasonsOf(a *observe.CandidateAssessment) string {
	if a == nil || len(a.Reasons) == 0 {
		return "(no reasons)"
	}
	return reasonList(a.Reasons)
}

// reasonList renders a subset of the reasons, for the cases that are about one part rather than
// the whole assessment.
func reasonList(rs []observe.AssessmentReason) string {
	if len(rs) == 0 {
		return "(no reasons)"
	}
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		parts = append(parts, string(r))
	}
	return strings.Join(parts, ", ")
}

// ── the one string a person writes ────────────────────────────────────────────

// MaxNameLength bounds a requested behaviour name.
const MaxNameLength = 60

// CheckName decides whether a requested behaviour name can be written down.
//
// Deliberately minimal. The AUTHORITATIVE validation happens where a play is saved, against the
// naming rules that already exist; this exists so that a name nobody could ever use is refused
// before the user is asked to demonstrate anything. It rejects nothing a save would accept.
func CheckName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return fmt.Errorf("a name is needed: what should Marco call this?")
	}
	if len([]rune(n)) > MaxNameLength {
		return fmt.Errorf("that name is %d characters; %d is the most Marco will write down",
			len([]rune(n)), MaxNameLength)
	}
	for _, r := range n {
		if !unicode.IsPrint(r) {
			return fmt.Errorf("that name contains something Marco cannot write down")
		}
	}
	return nil
}

// QuestionDiagnoser is the optional half of Tail.Question: why there is no question to ask.
//
// # Why this exists
//
// Because "no question" is a silence with several completely different causes, and the
// coordinator cannot tell them apart. The evidence may have earned no question; the question may
// have been asked already and answered; or — the one found live — the interruption budget may
// have gone to a different route entirely, so the person sitting in front of the panel waiting to
// be asked about the thing they just demonstrated never was.
//
// All three land in the same phase, with the same sentence on screen, forever. The tail knows
// which; the coordinator does not; nothing carried it across. Optional for the same reason
// GrantDiagnoser is: a Tail that cannot know keeps exactly the behaviour it had.
type QuestionDiagnoser interface {
	// QuestionRefusal says why no question of this kind is open for this route, in the
	// closed vocabulary the judgement already uses. Empty when one IS open, or when the
	// tail has nothing to say.
	QuestionRefusal(route observe.RelationshipRef, kind observe.AskKind) string
}

// ── the ordered review of a demonstration's edges ─────────────────────────────

// EdgeStatus is how far one required edge has got through its review.
//
// A CLOSED set with exactly one open value. An episode may only finish when every edge is
// terminal, so "still to do" has to be distinguishable from "done and not verified" — the
// distinction the old lifecycle could not make, because it never looked at a second edge.
type EdgeStatus string

const (
	// EdgePending has not been offered yet.
	EdgePending EdgeStatus = "pending"
	// EdgeOffered has an open rehearsal question, or a grant being spent.
	EdgeOffered EdgeStatus = "offered"
	// EdgeVerified rehearsed and arrived. The only status that makes a route executable.
	EdgeVerified EdgeStatus = "verified"
	// EdgeDeclined is the person saying no to this leg. Their answer, kept.
	EdgeDeclined EdgeStatus = "declined"
	// EdgeRefused is Marco unable to try it, with a reason.
	EdgeRefused EdgeStatus = "refused"
	// EdgeUnresolved is a leg that could not be reviewed in this episode — no question could
	// be raised for it, or the attempt could not be made. Terminal HERE without claiming
	// anything about the edge itself, which stays perfectly good durable knowledge.
	EdgeUnresolved EdgeStatus = "unresolved"
)

// Terminal reports whether this edge's review is over.
func (s EdgeStatus) Terminal() bool {
	return s == EdgeVerified || s == EdgeDeclined || s == EdgeRefused || s == EdgeUnresolved
}

// EdgeReview is one required edge of the demonstrated route, and how its review went.
type EdgeReview struct {
	Route  observe.RelationshipRef `json:"route"`
	Status EdgeStatus              `json:"status"`
	// Asked says this edge has already been put to the proposal machinery once. One request
	// per edge: asking again every cycle would be a loop, not a question.
	Asked bool `json:"-"`
	// Why is the reason behind a non-verified terminal status, for a person to read.
	Why string `json:"why,omitempty"`
}

// RouteStatus is what the demonstrated route as a whole amounts to.
type RouteStatus string

const (
	// RouteUnreviewed has edges still to look at.
	RouteUnreviewed RouteStatus = "unreviewed"
	// RouteVerified is every required edge verified — the route is executable end to end.
	RouteVerified RouteStatus = "verified"
	// RoutePartial is reviewed to the end with at least one edge not verified.
	//
	// The honest middle, and the one the old lifecycle could not say. "I learned the route,
	// but one step is not verified yet" is a different thing from success and a different
	// thing from failure, and the edges that DID verify are durable either way.
	RoutePartial RouteStatus = "partial"
)

// Status folds the edge reviews into what the route amounts to.
//
// A fold, deliberately: there is no route-level flag anybody could set out of step with the edges
// it is made of. Verified means every required edge verified — one unverified leg leaves the whole
// thing not executable from its start, however well the rest went.
func (s Session) Status() RouteStatus {
	if len(s.Edges) == 0 {
		return RouteUnreviewed
	}
	verified := 0
	for _, e := range s.Edges {
		if !e.Status.Terminal() {
			return RouteUnreviewed
		}
		if e.Status == EdgeVerified {
			verified++
		}
	}
	if verified == len(s.Edges) {
		return RouteVerified
	}
	return RoutePartial
}

// Verified is how many required edges are verified, and how many there are.
func (s Session) Verified() (int, int) {
	n := 0
	for _, e := range s.Edges {
		if e.Status == EdgeVerified {
			n++
		}
	}
	return n, len(s.Edges)
}

// nextEdge is the first required edge whose review is not over, in DEMONSTRATED order.
//
// First unresolved rather than any unresolved: the order is the walk, and after Marco rehearses
// A → B it is standing at B, which is exactly where B → C begins. Reviewing out of order would
// ask a person to walk back and forth to satisfy an ordering nobody chose.
func (c *Coordinator) nextEdge() (int, bool) {
	for i, e := range c.s.Edges {
		if !e.Status.Terminal() {
			return i, true
		}
	}
	return 0, false
}

// reviewEdges drives the demonstration's required edges, one at a time, in order.
//
// # The defect this closes
//
// A live two-hop teach — Settings Home → Bluetooth & devices → Mouse — produced both candidates
// correctly and rehearsed only one. Teaching gets a single exempt rehearsal question, the terminal
// leg claimed it, and when that rehearsal finished the lifecycle advanced straight to Naming. The
// first leg stayed `single_demonstration_only`, was never offered, and the episode ended. The goal
// was therefore unreachable from where the person started, and the only way forward was to
// demonstrate the first leg again as a separate teach.
//
// # Sequential, not parallel
//
// One edge is under review at a time. The alternative — opening a question per edge — turns Learn
// into a questionnaire and spends the interruption budget the rest of this system is careful
// about. So this points `c.s.Route` at one edge and lets the ordinary machinery underneath do
// exactly what it already did for a single-edge demonstration: raise the question, take the
// answer, spend the grant, run the attempt. Nothing below this function knows there is a sequence.
//
// # It is the only way past the review
//
// The episode advances when every required edge is terminal, and not before. An edge that could
// not be offered at all becomes `unresolved` rather than being skipped silently — a route with an
// unreviewed leg must never read as a learned one.
//
// Deleting the re-entry — advancing after the first edge — must fail
// TestBothDemonstratedEdgesAreOffered.
func (c *Coordinator) reviewEdges() {
	i, more := c.nextEdge()
	if !more {
		// Every required edge is terminal. What the route AMOUNTS to is a fold over them,
		// never a flag set here.
		done, total := c.s.Verified()
		c.note(fmt.Sprintf("route review complete: %d/%d verified (%s)",
			done, total, c.s.Status()))
		c.enter(Lowering)
		return
	}
	e := &c.s.Edges[i]
	// POINT THE EPISODE AT THIS EDGE. Every layer below works in terms of one route.
	c.s.Route = e.Route
	if e.Status == EdgePending {
		e.Status = EdgeOffered
		done, _ := c.s.Verified()
		c.note(fmt.Sprintf("reviewing step %d of %d (%d verified so far)",
			i+1, len(c.s.Edges), done))
	}
	if !c.haveTail() {
		return
	}
	// ASK FOR THIS EDGE TO BE PUT TO THE AUDIENCE, once, when nothing is asking yet.
	//
	// Teaching gets one exempt rehearsal question and the terminal leg claims it, so every
	// other required leg is refused `another_question_open` and this phase would wait for a
	// question nobody was going to raise. Observed live: Home to Bluetooth to Mouse reviewed
	// exactly one of its two legs.
	//
	// It names the edge; the proposal machinery still judges the evidence and decides whether
	// there is anything to ask. Requested only for the edge under review, only while it is
	// not terminal, and only when no question is already open for it — one at a time.
	//
	// Deleting this must fail TestTheSecondEdgeGetsItsOwnQuestion.
	answer, answered := observe.ResponseNone, false
	if a, ok := c.tail.(RehearsalAnswer); ok {
		answer, answered = a.AnswerToRehearsal(e.Route)
	}
	if o, ok := c.tail.(RehearsalOfferer); ok && !answered && !c.tail.Granted(e.Route) {
		if _, open := c.tail.Question(e.Route, observe.AskRehearse); !open {
			// RETRIED while this leg is still waiting to be asked about.
			//
			// The slot can be busy for a reason that has nothing to do with this route:
			// live, a screen-naming question held it, the offer produced no question,
			// and asking once meant the leg was never put to the Audience at all.
			//
			// Bounded by the two conditions above, which is what stops it being a busy
			// loop. A question exists — nothing to ask for. A yes exists — the asking is
			// over. Only the genuinely unasked leg tries again, and one proposal per
			// route is enforced by identity, so a repeat either raises the question that
			// could not be raised before or changes nothing.
			//
			// Deleting either guard must fail TestAnAnsweredEdgeIsNotOfferedAgain.
			first := !e.Asked
			e.Asked = true
			if err := o.OfferRehearsal(e.Route); err != nil && first {
				c.note("could not offer step " + fmt.Sprint(i+1) + ": " + err.Error())
			}
		}
	}
	c.awaitGrant()
	// AN ANSWER THAT CREATED NO PERMISSION ENDS THIS EDGE, not the episode.
	//
	// # Why this needs an actual reason and not merely a missing question
	//
	// It used to end the edge whenever no question was open, and that is wrong in the most
	// ordinary case there is. Live: a screen-naming question held the interruption slot, so
	// the offer for step 1 produced nothing YET, and step 1 was marked unresolved before the
	// person had been asked anything at all. The review then moved on to step 2 and the first
	// leg of the route was silently written off.
	//
	// "No question right now" is temporary. "Somebody answered and it was not a yes" is
	// terminal, and the two are the same absence through `Question`. So the leg ends only on
	// a POSITIVE signal — what the Audience actually said, or a grant diagnosis saying why a
	// yes created no authority. Absent both, the leg stays under review and is offered again:
	// waiting costs a cycle, writing off a step costs the step.
	//
	// # A yes that has not finished arriving is not a no
	//
	// Saying yes creates two facts — the proposal gains a response, and a grant is created —
	// and there is a moment where the first exists and the second does not. Treating "answered
	// and not granted" as a refusal turned somebody's Yes into
	// `unresolved (the answer to this step was not a yes)` about the step they had just
	// approved. So `confirmed` never ends a leg here; only a no, a not-now, or a diagnosed
	// failure of the yes itself does.
	//
	// Deleting the `answered` requirement must fail TestABusyQuestionSlotDoesNotWriteOffTheStep.
	// Ending the leg on a `confirmed` must fail TestAYesIsNotReadAsARefusalBeforeTheGrantExists.
	if c.s.Phase == ReviewingEdges && e.Status == EdgeOffered && e.Asked {
		if _, open := c.tail.Question(e.Route, observe.AskRehearse); !open &&
			!c.tail.Granted(e.Route) {

			switch {
			case answered && answer != observe.ResponseConfirmed:
				c.resolveEdge(EdgeDeclined, refusedRehearsal(answer))
			default:
				if d, ok := c.tail.(GrantDiagnoser); ok {
					if why := d.GrantRefusal(e.Route); why != "" {
						c.resolveEdge(EdgeUnresolved, why)
					}
				}
			}
		}
	}
}

// refusedRehearsal says, in a person's words, what their answer meant for this step.
//
// The proposal vocabulary keeps "no" and "not now" apart deliberately — one is a judgement about
// the route, the other a decision not to make one — and a review that flattened them would tell
// somebody who asked for time that they had rejected the step.
func refusedRehearsal(answer observe.UserResponse) string {
	switch answer {
	case observe.ResponseContradicted:
		return "you said no to trying this step"
	case observe.ResponseDeclined:
		return "you said not now"
	case observe.ResponseNone:
		// SETTLED WITH NO RESPONSE is a retraction: the question was put, an answer was
		// taken back, and the proposal machinery will not raise it again. Reporting it as
		// unasked left a review waiting forever on a question nobody would ask.
		return "that answer was taken back, so this step was not tried"
	}
	return "the answer to this step was not a yes"
}

// resolveEdge records how the edge under review turned out and returns to the review.
//
// Called from the rehearsal outcomes instead of advancing the episode. The edge under review is
// the one `c.s.Route` points at, which reviewEdges set.
func (c *Coordinator) resolveEdge(status EdgeStatus, why string) bool {
	for i := range c.s.Edges {
		if c.s.Edges[i].Route != c.s.Route {
			continue
		}
		c.s.Edges[i].Status, c.s.Edges[i].Why = status, why
		c.enter(ReviewingEdges)
		c.reviewEdges()
		return true
	}
	return false
}

// recordRequiredEdges takes the ordered route a demonstration walked.
//
// ONE implementation, called from both places a result carrying a demonstration is consumed.
// Two copies of "which edges does this episode owe a review to" would eventually be two answers,
// and the one that ran last would win silently.
//
// Every leg has to reach a terminal review state before the episode may finish. A walk that could
// not be ordered leaves this empty, and the lifecycle then behaves exactly as it always did — on
// the terminal leg alone, which is the honest fallback: an unordered set is not a sequence.
//
// Truncating it must fail TestAThreeEdgeDemonstrationReviewsAllThree.
func (c *Coordinator) recordRequiredEdges(res observesession.Result) {
	if len(res.RouteWalk) == 0 {
		return
	}
	c.s.Edges = nil
	for _, ref := range res.RouteWalk {
		c.s.Edges = append(c.s.Edges, EdgeReview{Route: ref, Status: EdgePending})
	}
}

// saveWalk writes the whole verified route where the tail can, and one edge where it cannot.
//
// Every required edge must have verified. A route with an unreviewed or refused leg is partial —
// `Session.Status` already says so — and writing it down as though it were the whole behaviour
// would be a play that claims more than was proven.
func (c *Coordinator) saveWalk() (Saved, error) {
	saver, ok := c.tail.(RouteSaver)
	if !ok || len(c.s.Edges) < 2 {
		return c.tail.Save(c.s.Route, c.actor, c.verb)
	}
	walk := make([]observe.RelationshipRef, 0, len(c.s.Edges))
	for _, e := range c.s.Edges {
		if e.Status != EdgeVerified {
			// Not the whole behaviour, so it is not saved as the whole behaviour. The
			// terminal edge alone is what the episode has actually proven.
			return c.tail.Save(c.s.Route, c.actor, c.verb)
		}
		walk = append(walk, e.Route)
	}
	return saver.SaveRoute(walk, c.actor, c.verb)
}
