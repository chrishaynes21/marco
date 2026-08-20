package rehearse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/production"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// One move, then look at the board again.
//
// # What changes from the dry attempt, and what does not
//
// Nothing above the host changes. The same `Attempt`, the same claim, the same `LowerStep`, the
// same `marcoexec.Operation`, the same legal Marco, the same compiler and runtime. What this file
// adds is everything that must be true AROUND one real input:
//
//	establish where Marco actually is, by looking          (never an argument)
//	compare that with what the grant authorized
//	claim the grant
//	re-check the target has not moved under us
//	emit ONE step
//	wait for the interface to stop changing
//	look again, at a FRESH observation
//	classify
//	stop
//
// # Two things this file is careful about
//
// **A refusal before input is not a result.** Nothing here produces a RehearsalResult unless the
// program actually reached a host. "Marco declined to try" and "Marco tried and it went wrong"
// are different facts about the world, and a reader who cannot tell them apart cannot audit
// anything.
//
// **The host accepting an input means the input was emitted.** It does not mean the application
// responded, and it is never evidence that the step worked. Only a fresh observation is.
//
// See [[ADR-024-a-dry-step-is-not-evidence]] and [[ADR-023-rehearsal-is-attempt-scoped-authority]].

// ── what happened, once something happened ────────────────────────────────────

// Outcome is the closed vocabulary of how one attempted step turned out.
//
// Every value here means input WAS emitted. There is no "refused" among them: a refusal is an
// error and produces no result at all.
type Outcome string

const (
	// DirectlyVerified is the only outcome that says the step did what it was meant to.
	//
	// The expected remembered subject was RESOLVED afterwards. Not "the screen changed" —
	// a screen that changed into something else is `wrong_state`, and treating change as
	// success is how a procedure gets promoted for going somewhere nobody asked for.
	DirectlyVerified Outcome = "directly_verified"
	// ProgressUnobservable is a step whose result Marco was never going to be able to see,
	// where reality nonetheless stayed inside the permitted containment: same target, same
	// application, same screen, nothing contradictory.
	//
	// It does NOT mean the selection moved correctly. It means Marco tried the step and
	// containment held. An EXPECTED property of the step — contrast Unobservable.
	ProgressUnobservable Outcome = "progress_unobservable"
	// WrongState is a different remembered subject, resolved clearly.
	WrongState Outcome = "wrong_state"
	// TargetMoved is the window identity changing under the attempt.
	TargetMoved Outcome = "target_moved"
	// TargetUnavailable is the window going away, or becoming impossible to observe.
	TargetUnavailable Outcome = "target_unavailable"
	// Ambiguous is perception that cannot separate several plausible subjects. Marco does
	// not pick the nearest one.
	Ambiguous Outcome = "ambiguous"
	// Unobservable is a RUNTIME failure to inspect the outcome — perception did not run,
	// the screen never settled, or the attempt was interrupted mid-observation.
	//
	// Deliberately distinct from ProgressUnobservable. One is a property of the step and was
	// known before Marco tried; the other is Marco failing to look. Collapsing them would
	// let every broken observation read as a contained success.
	Unobservable Outcome = "unobservable"
	// Unrecognised is a screen Marco COULD read and does not remember.
	//
	// Split from Unobservable, which used to carry both. They are opposite facts: one is
	// Marco failing to look, the other is Marco looking and finding somewhere new. A live
	// rehearsal reported "unobservable" while the window was in front and perfectly legible,
	// which sent the diagnosis chasing perception for an hour when the real answer was that
	// the keys had landed somewhere it had never been shown.
	Unrecognised Outcome = "unrecognised"
	// WindowBehind is the watched window falling out of the desktop's foreground mid-route.
	//
	// Only ever attached to the synthetic stop-step `stop` records: input has no address,
	// so a window that is not in front cannot be acted on, and the attempt ends where it
	// stood rather than typing into whatever took its place.
	WindowBehind Outcome = "window_not_in_front"
	// InputFailed is the host or the runtime refusing part-way.
	//
	// A RESULT rather than a refusal, and the choice is deliberate: by the time the program
	// reached the host, Marco can no longer claim nothing was sent. Some of the run may have
	// landed. Saying "refused" would be a claim about the world that is not available.
	InputFailed Outcome = "input_failed"
)

// SettleOutcome is what the wait after the input concluded.
type SettleOutcome string

const (
	// SettleStable is the screen holding still long enough to be worth reading.
	SettleStable SettleOutcome = "stable"
	// SettleChanging is an interface still moving when the bound ran out — a spinner, an
	// animation, a load. Honest, and not a longer wait.
	SettleChanging SettleOutcome = "still_changing"
	// SettleTargetLost is the window going away while waiting.
	SettleTargetLost SettleOutcome = "target_lost"
	// SettleInterrupted is cancellation arriving during the wait.
	SettleInterrupted SettleOutcome = "interrupted"
)

// TargetOutcome is what happened to the window across the attempt.
type TargetOutcome string

const (
	TargetHeld     TargetOutcome = "held"
	TargetChanged  TargetOutcome = "changed"
	TargetLost     TargetOutcome = "lost"
	TargetUnproven TargetOutcome = "unproven"
)

// StepRecord is what ONE attempted step did.
//
// A step owns no authorization — the attempt does — so nothing here is permission for anything.
// It is a report: what was sent, what Marco could have checked, and what it found when it looked.
//
// # What it is not
//
// It is not verification of the procedure. One step is one step: `ProcedureCandidate.Verified`
// stays false however this comes out, no Marco is generated, and no capability is promoted. It is
// not durable either — this milestone carries it in the session response and defers folding it
// into the learning loop, because evidence that feeds an assessment needs an ADR of its own.
//
// # What it may hold
//
// Ids, counts, closed vocabulary and semantic intents that were already approved by the
// observation boundary. No raw keys, no text, no titles, no screenshots, no OCR.
type StepRecord struct {
	// Attempt is the claimed authorization's identity.
	Attempt string
	// Application, Relationship, Source and Position say which step of which route.
	Application  string
	Relationship observe.RelationshipRef
	Source       string
	Position     int
	// Intents is what was sent, in order.
	Intents []observe.NavIntent
	// Expect is the remembered subject the step should have produced; Verification is how
	// well it could ever have been checked.
	Expect       string
	Verification observe.StepVerifiability
	// Outcome is the classification. Observed is the subject perception resolved
	// afterwards, empty when nothing resolved.
	Outcome  Outcome
	Observed string
	// Target and Settle are the two lifecycle facts a reader needs to judge the outcome.
	Target TargetOutcome
	Settle SettleOutcome
	// Cancelled says the user stopped the attempt after input had been emitted. It never
	// erases the fact that input occurred.
	Cancelled bool
	// Emitted is what the host was asked for, as the boundary recorded it. Empty when the
	// composition root installed a host that does not record.
	Emitted []string
	// Program is the legal Marco that was compiled and run, kept for inspection.
	Program string
	// StartedAt and Duration bound the attempt for diagnostics.
	StartedAt time.Time
	Duration  time.Duration
	// Detail is the host's OWN sentence when a step failed — which target, which provider,
	// what refused. Empty on every step that did not fail.
	//
	// # Why this one field may carry free text
	//
	// The rest of this record is closed vocabulary and subject ids on purpose, and that rule
	// is not relaxed. This is the same deliberate exception learn.Attempt.Detail already
	// makes, for the same reason: `input_failed` says a step did not land and says nothing
	// about WHY, and the difference between "the host could not find the target", "the window
	// went away" and "the provider errored" is the whole of what a person needs. It is
	// diagnostic, never durable — nothing writes it to the store — and no decision reads it.
	Detail string
}

// Tried reports that input was emitted. True for every result, by construction.
func (r StepRecord) Tried() bool { return r.Outcome != "" }

// Verified reports the one outcome that says this STEP did what it was meant to.
//
// Named on a step and never on a candidate. A procedure with four steps is not verified by one of
// them going right, and a prefix is not a route.
func (r StepRecord) Verified() bool { return r.Outcome == DirectlyVerified }

// ── the live runner ───────────────────────────────────────────────────────────

// Recogniser is the ONE thing a rehearsal needs from semantic memory: what is this screen?
//
// Narrowed to a single read on purpose. `observe.Memory` can remember subjects, relationships,
// learning requests and demonstrations; a rehearsal may do none of those, and taking the whole
// interface would make "a rehearsal never writes to memory" a rule somebody has to remember
// instead of a fact about the type. `*semanticmemory.Store` satisfies this already.
type Recogniser interface {
	Recall(application string, sig observe.StructureSignature) observe.Recollection
}

// Live rehearses one authorized step against a real application.
//
// # Why the runner is nil by default
//
// Because being able to act must be a decision somebody made. `Live` is constructed with
// perception and memory — neither of which can affect anything — and is INCAPABLE of emitting
// until a composition root hands it a `directorapi.MarcoRunner` through WithActuator. A package
// that defaulted to a real host would make every test one mistake away from typing into whatever
// was in front.
type Live struct {
	clock   observesession.Clock
	target  observesession.Target
	sampler observesession.Sampler
	memory  Recogniser
	th      observe.HypothesisThresholds

	actuator directorapi.MarcoRunner
	recorder Recorder
	// real says the actuator reaches a computer.
	//
	// Load-bearing for HONESTY rather than for safety. A dry run emits into a notebook, so the
	// application does not move — and classifying that would report "the screen became A, which
	// is not what that step was for", which reads as a failed step when in fact nothing was
	// sent. Marco does not get to say what came of an action it did not take.
	real bool
	// inFront reports whether the watched window currently leads the desktop.
	//
	// Emitted input has no address: whatever window is in front receives it, and nothing on
	// this path targets a window. So a real attempt against a window that is NOT in front
	// does not act on the application it was authorized for — it types into the terminal the
	// user just said yes in, and then honestly reports that the screen did not respond. That
	// happened live, and it reads exactly like "the rehearsal never fired".
	//
	// Supplied by the composition root, because only the platform can answer it. A runner
	// nobody gave the question to behaves as it always has — the stub platforms cannot ask —
	// and the Windows root always wires it.
	inFront func(windowref.Ref) bool
	// theater puts on a production for the one step shape that needs it — a demonstrated
	// pointer press. Nil means this Director has no Theater, and a point step refuses before
	// emitting rather than reaching for something else.
	theater production.Producer
}

// NewLive builds a rehearsal runner that cannot yet act.
func NewLive(clock observesession.Clock, target observesession.Target,
	sampler observesession.Sampler, memory Recogniser) *Live {

	return &Live{clock: clock, target: target, sampler: sampler, memory: memory,
		th: observe.DefaultHypothesisThresholds()}
}

// WithActuator makes this runner capable of emitting. The composition root's decision.
//
// `real` says whether what is at the other end is a computer. It changes nothing about what is
// sent — the same program goes to the same interface either way — and everything about what Marco
// is entitled to conclude afterwards.
func (l *Live) WithActuator(runner directorapi.MarcoRunner, rec Recorder, real bool) *Live {
	l.actuator, l.recorder, l.real = runner, rec, real
	return l
}

// WithTheater installs the Theater this Director's rehearsals put their productions on.
//
// The composition root's decision, exactly as the runner is: this package holds no host and may
// not build one. A Live without a Theater cannot press a named control and says so.
func (l *Live) WithTheater(p production.Producer) *Live {
	l.theater = p
	return l
}

// WithForeground installs the platform's answer to "is this window in front".
//
// Checked only for REAL attempts: a dry run emits into a notebook and the desktop's
// foreground is none of its business.
func (l *Live) WithForeground(f func(windowref.Ref) bool) *Live {
	l.inFront = f
	return l
}

// behind reports whether real input emitted right now would land somewhere other than the
// watched window.
func (l *Live) behind(ref windowref.Ref) bool {
	return l.real && l.inFront != nil && !l.inFront(ref)
}

// The bounds of the two observation windows.
//
// Both are counts of observations rather than durations, because an observation is the evidence
// and a duration is a guess about how long evidence takes. `establishSamples` is how long Marco
// looks before it is willing to say where it is; `settleSamples` bounds the wait afterwards.
const (
	establishSamples = 6
	settleSamples    = 8
	// settleStableRun is how many consecutive unchanged observations count as settled.
	settleStableRun = 3
	// sampleGap is the pause between observations, taken on the injected clock.
	sampleGap = 120 * time.Millisecond
)

// ── the whole attempt ─────────────────────────────────────────────────────────

// Terminal is how one whole attempt ended.
//
// Distinct from a step's Outcome because they answer different questions. A step's outcome says
// what one action did; this says what became of the route. The only value that means the learned
// procedure survived its conversation with reality is CompletedRoute.
type Terminal string

const (
	// CompletedRoute is every authorized step taken, every observable checkpoint verified,
	// and the destination DIRECTLY verified. The only outcome that could ever support
	// verifying a candidate.
	CompletedRoute Terminal = "completed_route"
	// StoppedAtStep is a step that came out any way other than one permitting continuation.
	// The step's own Outcome says which.
	StoppedAtStep Terminal = "stopped_at_step"
	// EndedUnverified is every step taken without a refusal, but the route did not finish on
	// a directly-verified destination. A rehearsal cannot succeed on containment.
	EndedUnverified Terminal = "ended_unverified"
	// CancelledAttempt is the user stopping it. Whatever was already sent stays recorded.
	CancelledAttempt Terminal = "cancelled"
	// BoundsExceeded is an attempt that ran out of steps, inputs or time.
	BoundsExceeded Terminal = "bounds_exceeded"
	// NothingSent is a dry attempt: lowered, offered to a recorder, and nothing tried.
	NothingSent Terminal = "nothing_sent"
)

// RehearsalResult is what one whole authorized attempt did.
//
// # What it may support, and what it may not
//
// A CompletedRoute is the ONLY evidence in this system that could ever justify calling a candidate
// verified — and even then the justification is derived, not stamped: see
// [[ADR-026-verification-is-derived-from-a-completed-rehearsal]]. A prefix does not verify a
// route, a contained step does not verify a step, and a dry run verifies nothing at all.
//
// # What it may hold
//
// Ids, counts, closed vocabulary and semantic intents already approved by the observation
// boundary. No raw keys, no text, no titles, no screenshots, no OCR, and nothing executable — the
// steps name MEANINGS, and reproducing them means lowering again through marcoexec.
type RehearsalResult struct {
	// Attempt is the claimed authorization's identity.
	Attempt string
	// Application, Relationship, Source and Destination are the fixed scope.
	Application  string
	Relationship observe.RelationshipRef
	Source       string
	Destination  string
	// Evidence is the candidate digest this attempt was authorized against.
	//
	// Load-bearing for anything that reads a stored result later: it says WHICH version of the
	// demonstration was rehearsed, so a result cannot vouch for a candidate that has since
	// been revised.
	Evidence string
	// Live says a real host was installed. False means nothing reached a computer.
	Live bool
	// Steps is every step that was attempted, in order.
	Steps []StepRecord
	// Terminal is how the whole attempt ended.
	Terminal Terminal
	// Inputs and StepsTaken are the bounds consumed.
	Inputs     int
	StepsTaken int
	// Planned is how many steps the authorized plan contained.
	Planned int
	// StartedAt and Duration bound the attempt.
	StartedAt time.Time
	Duration  time.Duration
}

// Completed reports the one terminal outcome that says the whole route survived.
//
// Everything a later milestone needs to ask before lowering a learned procedure, in one method —
// and deliberately requiring `Live`, because a recorder proves only that Marco knows what it would
// have sent.
func (r RehearsalResult) Completed() bool {
	return r.Live && r.Terminal == CompletedRoute
}

// Emitted reports that input reached a host at all.
func (r RehearsalResult) Emitted() bool {
	for _, s := range r.Steps {
		if len(s.Emitted) > 0 || s.Tried() {
			return true
		}
	}
	return false
}

// Rehearse attempts an authorized candidate one step at a time, looking between every two.
//
// # The order, and why it is this order
//
//  1. LOOK. Establish the window and the current subject from perception alone.
//  2. COMPARE. Application, subject and route against the grant. Any mismatch aborts, and
//     the grant is NOT spent: Marco never got as far as being able to act.
//  3. CLAIM. From here the permission is gone whatever happens next, because from here an
//     input becomes possible. See [[ADR-023-rehearsal-is-attempt-scoped-authority]].
//  4. Then, for each step, and never twice in a row:
//     RE-CHECK the target — between looking and acting the user may have alt-tabbed;
//     EMIT one step;
//     SETTLE and look again at a FRESH observation;
//     CLASSIFY, record, and ask reality whether there may be another.
//
// The loop is bounded by the attempt, not by this function: `Attempt.Observed` is the only way out
// of `acted`, so a bug here refuses rather than acting twice.
//
// Returns a result only when the attempt got as far as being able to act. Everything before that
// returns an error, because "Marco declined to try" and "Marco tried and it went wrong" are
// different facts about the world.
func (l *Live) Rehearse(ctx context.Context, g *observe.RehearsalGrant,
	j observe.RehearsalJudgement, selector windowref.Selector, from int) (
	RehearsalResult, error) {

	if l == nil || l.sampler == nil || l.target == nil || l.memory == nil {
		return RehearsalResult{}, refuse(RefusalNoActuator, "this runner cannot observe")
	}
	// EXPLICIT live enablement. Not a default, not a fallback, not inferred from a grant.
	if l.actuator == nil {
		return RehearsalResult{}, refuse(RefusalNoActuator,
			"no actuator is wired; a rehearsal runner cannot obtain one for itself")
	}
	if g == nil {
		return RehearsalResult{}, refuse(RefusalNoGrant, "nothing has authorized this")
	}
	// The authorization first, and cheaply. Looking at a screen for two seconds to discover
	// that permission was withdrawn before we started is a worse answer to the same question.
	switch g.State() {
	case observe.GrantConsumed:
		return RehearsalResult{}, refuse(RefusalGrantSpent, "one authorization permits one attempt")
	case observe.GrantRevoked:
		return RehearsalResult{}, refuse(RefusalGrantRevoked, "this authorization was withdrawn")
	}
	return l.Perform(ctx, g, j, selector, from)
}

// Perform walks one edge and verifies every step of it, against an authority already established.
//
// # Why this sits BELOW rehearsal
//
// Rehearsal is one caller. A learned play being performed because the Audience asked for it by name
// is another, and the two differ only in how the authority was obtained — a yes to a question Marco
// raised, or an explicit instruction naming the behaviour. Everything after that is identical: look
// at the Stage, refuse if it is not where the edge begins, take one step at a time, and verify
// after each.
//
// Factoring it here is what stops the second caller reimplementing the walk. A second walker would
// be a second set of answers to "did that step work", and the whole verification story rests on
// there being one.
//
// The authority is also the BUDGET — `BeginAttempt` binds the attempt to its input and duration
// bounds and consumes it. So a caller that performs without one is not a caller with different
// authority semantics; it is a caller with none, and this cannot be reached without one.
//
// Deleting the shared call from Rehearse must fail TestRehearsalAndExecutionShareOneWalker.
func (l *Live) Perform(ctx context.Context, g *observe.RehearsalGrant,
	j observe.RehearsalJudgement, selector windowref.Selector, from int) (
	RehearsalResult, error) {

	if l == nil || l.sampler == nil || l.target == nil || l.memory == nil {
		return RehearsalResult{}, refuse(RefusalNoActuator, "this runner cannot observe")
	}
	if l.actuator == nil {
		return RehearsalResult{}, refuse(RefusalNoActuator,
			"no actuator is wired; a runner cannot obtain one for itself")
	}
	if g == nil {
		return RehearsalResult{}, refuse(RefusalNoGrant, "nothing has authorized this")
	}
	started := l.clock.Now()

	// ── 1 & 2: look, then compare. No input is possible anywhere in here. ──
	ref, subject, err := l.establish(ctx, selector, g.Application)
	if err != nil {
		return RehearsalResult{}, err
	}
	if subject != g.Source {
		return RehearsalResult{}, refuse(RefusalSourceMismatch,
			"authorized to start from one screen, and Marco is looking at a different one")
	}
	// THE foreground gate, and it sits BEFORE the claim on purpose. Nothing has been spent,
	// so the honest response upstream is to wait — the person answered yes in some other
	// window, and the watched one comes forward the moment they click back into it. See
	// notReadyYet in the Learn coordinator and [[ADR-055-an-authorised-rehearsal-waits-for-its-start]].
	if l.behind(ref) {
		return RehearsalResult{}, refuse(RefusalWindowBehind,
			"the watched window is not in front, so input would land somewhere else")
	}

	// ── 3: THE claim. Past this line the permission is spent whatever happens. ──
	//
	// The digest comes from the RECOMPUTED judgement, never from the grant. Taking it from the
	// grant would compare the authorization against itself and always agree, which is a check
	// that cannot fail and therefore is not one.
	scope := Scope{Application: g.Application, Source: subject,
		Relationship: j.Relationship, Evidence: j.Digest}
	attempt, err := BeginAttempt(g, j, scope, l.clock.Now())
	if err != nil {
		return RehearsalResult{}, err
	}

	out := RehearsalResult{
		Attempt: attempt.ID, Application: g.Application, Relationship: g.Relationship,
		Source: subject, Destination: g.Destination, Evidence: j.Digest, Live: l.real,
		Planned: len(j.Plan), StartedAt: started,
	}
	if from <= 0 {
		from = 1
	}

	for position := from; position <= len(j.Plan); position++ {
		// ── the final guard, as late as it can possibly be, for EVERY step. ──
		//
		// Not once at the start. Generation being stable before step 1 says nothing about
		// step 4, and this is the race that matters: verify the screen, the user alt-tabs,
		// the input lands in their email.
		// A guard that fails BEFORE anything has been sent is a refusal, not a result:
		// Marco never got as far as trying. Once a step has gone out it becomes a result,
		// because the record of what already happened must not be thrown away.
		again, err := l.target.Acquire(ctx, selector)
		if err != nil {
			if out.StepsTaken == 0 {
				attempt.Cancel()
				return RehearsalResult{}, refuse(RefusalTargetLost,
					"the window went away: %s", err)
			}
			return l.stop(attempt, out, StoppedAtStep, TargetUnavailable, started), nil
		}
		if !sameWindow(ref, again) {
			if out.StepsTaken == 0 {
				attempt.Cancel()
				return RehearsalResult{}, refuse(RefusalTargetMoved,
					"the window changed between checking the screen and acting on it")
			}
			return l.stop(attempt, out, StoppedAtStep, TargetMoved, started), nil
		}
		// Re-asked before EVERY step, like the guards above it, and for the same race: the
		// window was in front for step 1, the user clicked their email, and step 2 would
		// land there. Once something has been sent this is a result, not a refusal — the
		// record of what already happened must not be thrown away.
		if l.behind(again) {
			if out.StepsTaken == 0 {
				attempt.Cancel()
				return RehearsalResult{}, refuse(RefusalWindowBehind,
					"the watched window fell behind before anything was sent")
			}
			return l.stop(attempt, out, StoppedAtStep, WindowBehind, started), nil
		}
		if ctx.Err() != nil {
			if out.StepsTaken == 0 {
				attempt.Cancel()
				return RehearsalResult{}, refuse(RefusalCancelled, "%s", ctx.Err())
			}
			return l.stop(attempt, out, CancelledAttempt, "", started), nil
		}

		// ── ONE step. The same lowering the dry attempt used. ──
		//
		// The record is built BEFORE the step rather than from its result, because a
		// production verifies itself and writes its finding here — see settled. The fields
		// it needs are the plan's, which are known now; everything the emission adds is
		// merged below and the two sets are disjoint.
		plan := j.Plan[position-1]
		rec := StepRecord{
			Attempt: attempt.ID, Relationship: g.Relationship, Position: plan.Position,
			// The scope travels ON the record because the record is what classifies the
			// outcome: observeOutcome recalls against rec.Application, and memory is
			// application-namespaced. These were left empty once, and the consequence was
			// not a compile error or a wrong field in a report — it was every live
			// rehearsal recalling against the EMPTY application, matching nothing, and
			// ending `stopped_at_step` with `unrecognised` on software Marco knew
			// perfectly well. Deleting either assignment must fail
			// TestALiveStepIsClassifiedAgainstItsOwnApplication.
			Application: g.Application, Source: subject,
			Expect: plan.Expect, Verification: plan.Verifiability, Target: TargetHeld,
		}
		// THE DIRECTOR'S OWN VERIFICATION, lent to the production rather than kept back.
		//
		// Only for a real attempt: a dry run reached a notebook, so there is nothing to
		// look at and a verifier would be inventing an observation. The Theater is handed
		// nil there and honestly reports the production unverified.
		check := &settled{l: l, selector: selector, before: again, rec: &rec}
		stage := Stage{Produce: l.theater}
		if l.real {
			stage.Verify = check
		}
		emission, emitErr := attempt.LowerStep(ctx, position, l.actuator, l.recorder,
			again, stage)
		rec.Position = emission.Position
		rec.Intents, rec.Program, rec.Emitted = emission.Intents, emission.Program,
			emission.Emitted
		if emission.Expect != "" {
			rec.Expect, rec.Verification = emission.Expect, emission.Verification
		}
		if emitErr != nil {
			if !emission.Reached {
				// A refusal INSIDE lowering — a bound, a classification, a cancelled
				// context. Nothing reached the host, so nothing is claimed about the
				// world; the attempt simply stops here.
				if position == from {
					return RehearsalResult{}, emitErr
				}
				out.Terminal = terminalFor(emitErr)
				return l.finish(attempt, out, started), nil
			}
			// WHAT THE HOST ACTUALLY SAID. `input_failed` names the KIND of problem and
			// nothing about which one — the reason exists here, in emitErr, and used to
			// be dropped on the floor.
			//
			// Live, this reported "step 1: input_failed — expected subj_…, saw nothing"
			// and there was no way to tell a target the host could not find from a
			// window that had gone from a provider that errored. Every reporting gap
			// found in this session has had the same shape: the reason existed one layer
			// down and nothing carried it.
			//
			// Deleting this must fail TestAFailedInputSaysWhatTheHostSaid.
			rec.Outcome, rec.Settle = InputFailed, SettleInterrupted
			rec.Detail = emitErr.Error()
			out.Steps = append(out.Steps, rec)
			out.Terminal = StoppedAtStep
			return l.finish(attempt, out, started), nil
		}
		out.Inputs += len(emission.Intents)
		out.StepsTaken++

		if !l.real {
			// Nothing reached a computer, so there is nothing to look at and nothing to
			// conclude. A dry attempt stops after one step — sequencing past it would be
			// pretending an application had responded.
			out.Steps = append(out.Steps, rec)
			out.Terminal = NothingSent
			return l.finish(attempt, out, started), nil
		}

		// ── settle, look at something NEW, classify. ──
		//
		// Unless the production already did it. A press goes through the Theater and the
		// verifier above IS this call, made from inside it; looking again would read a
		// screen that has already settled and answer a question that has been answered.
		//
		// Asked of the verifier rather than of the report: a producer that claimed to have
		// verified without asking would otherwise leave the step with no outcome at all.
		//
		// Deleting the guard must fail TestAVerifiedProductionIsNotObservedTwice.
		if !check.ran {
			l.observeOutcome(ctx, selector, ref, &rec)
		}
		out.Steps = append(out.Steps, rec)

		// ── and ask reality whether there may be another. ──
		last := position == len(j.Plan)
		if !attempt.Observed(mayContinue(rec) && !last) {
			out.Terminal = terminalAfter(rec, last)
			return l.finish(attempt, out, started), nil
		}
	}
	// Every planned step was taken and every one permitted continuation, but the loop ran out
	// before a last step could settle the route. Reached only when `from` skipped past the end.
	out.Terminal = EndedUnverified
	return l.finish(attempt, out, started), nil
}

// mayContinue reports whether one step's outcome permits Marco to take another.
//
// Two values out of eight, and the second is the subtle one. `progress_unobservable` permits
// continuing because containment held — the same window, the same application, the same screen the
// step started on — and because the CANDIDATE declared before the attempt that this step was one
// Marco could never see the result of. It is never inferred afterwards from a failure to detect
// change; that path produces `wrong_state` or `unobservable`, and both stop.
func mayContinue(r StepRecord) bool {
	switch r.Outcome {
	case DirectlyVerified:
		return true
	case ProgressUnobservable:
		return r.Verification == observe.ProgressUnobservable && r.Target == TargetHeld
	}
	return false
}

// terminalAfter is how the attempt ended, given the step that ended it.
func terminalAfter(r StepRecord, last bool) Terminal {
	if r.Cancelled {
		return CancelledAttempt
	}
	if !mayContinue(r) {
		return StoppedAtStep
	}
	// The last step, and it permitted continuation. Only a DIRECTLY verified destination
	// completes a route: a rehearsal cannot succeed on containment, because containment says
	// nothing about having arrived.
	if last && r.Outcome == DirectlyVerified {
		return CompletedRoute
	}
	return EndedUnverified
}

// terminalFor maps a lowering refusal onto how the attempt ended.
func terminalFor(err error) Terminal {
	switch r, _ := RefusalOf(err); r {
	case RefusalCancelled:
		return CancelledAttempt
	case RefusalInputBound, RefusalUnobservableBound, RefusalAttemptTerminal:
		return BoundsExceeded
	}
	return StoppedAtStep
}

// stop ends an attempt that could not take the step it was about to.
//
// Distinct from `finish` only in that it records a step which was never emitted, so a reader can
// see WHERE the attempt stopped rather than only that it did.
func (l *Live) stop(a *Attempt, out RehearsalResult, t Terminal, why Outcome,
	started time.Time) RehearsalResult {

	if why != "" {
		out.Steps = append(out.Steps, StepRecord{
			Attempt: a.ID, Relationship: out.Relationship, Position: a.StepsTaken() + 1,
			Outcome: why, Target: TargetChanged,
		})
	}
	out.Terminal = t
	return l.finish(a, out, started)
}

// finish closes the attempt and stamps the duration.
func (l *Live) finish(a *Attempt, out RehearsalResult, started time.Time) RehearsalResult {
	a.Finish()
	out.Duration = l.clock.Now().Sub(started)
	return out
}

// Describe renders a whole attempt for a person.
//
// The last line is the boundary of the claim, and it is different for a completed route than for
// anything else — because a completed route is the first thing in this system that has earned a
// stronger sentence, and everything else must not borrow it.
func (r RehearsalResult) Describe() []string {
	out := []string{"rehearsal attempt"}
	out = append(out, "  path: "+r.Relationship.From+" → "+r.Relationship.To)
	if !r.Live {
		out = append(out, "  nothing reached the computer: this ran against a recording host")
	}
	for _, s := range r.Steps {
		for _, line := range s.Describe()[1:] {
			out = append(out, line)
		}
	}
	out = append(out, fmt.Sprintf("  %d of %d step(s) attempted, %d input(s)",
		r.StepsTaken, r.Planned, r.Inputs))
	switch {
	case r.Completed():
		out = append(out, "  the whole route ran and ended where it was meant to")
	case r.Terminal == NothingSent:
		out = append(out, "  this is what WOULD be sent. Marco did not try it")
	case r.Terminal == CancelledAttempt:
		out = append(out, "  you stopped it; whatever had already gone out stays on the record")
	case r.Terminal == BoundsExceeded:
		out = append(out, "  it ran out of the room it was given")
	default:
		out = append(out, "  it stopped before the end of the route")
	}
	if !r.Completed() {
		out = append(out, "  nothing was learned and nothing was saved")
	}
	return out
}

// establish answers "where is Marco, right now" from perception alone.
//
// Not an argument, and not the grant's opinion. The grant says the user demonstrated A → B, which
// is a claim about history; whether the interface is showing A at this moment is a question only
// looking can answer — the same rule the demonstration capture already follows, and the same
// stale-ordinal mistake this repository has made before.
func (l *Live) establish(ctx context.Context, selector windowref.Selector, application string) (
	windowref.Ref, string, error) {

	ref, totals, err := l.watch(ctx, selector, establishSamples, nil)
	if err != nil {
		return ref, "", err
	}
	if !strings.EqualFold(ref.Application, application) {
		return ref, "", refuse(RefusalApplicationMismatch,
			"authorized for %s, and %s is in front", application, ref.Application)
	}
	// THE ONE CURRENT-PLACE ANSWER. `observe.PlaceNow` projects the settled evidence and
	// resolves it against durable memory; this asks it rather than doing both again.
	//
	// It used to derive the signature and recall it here, and Sight, Learn and the outcome
	// classifier each did the same in their own words. They agreed — all four went through
	// `SignatureOfState` and `Recall` — but four copies of "where are we" is four places for
	// them to stop agreeing, and a rehearsal that answered it differently from the panel the
	// person is reading would be unexplainable from the outside.
	//
	// Deleting this — resolving the source separately — must fail
	// TestARehearsalAsksTheSameQuestionSightDoes.
	p := observe.PlaceNow(totals, application, l.memory, l.th)
	switch {
	case !p.Placed:
		return ref, "", refuse(RefusalSourceUnobservable,
			"Marco could not make out what is on screen well enough to say where it is")
	case p.Verdict == observe.MatchCandidate:
		return ref, "", refuse(RefusalSourceAmbiguous,
			"this screen resembles more than one Marco remembers")
	case !p.Established():
		return ref, "", refuse(RefusalSourceUnrecognised,
			"Marco does not recognise the screen it is looking at")
	}
	return ref, p.Subject, nil
}

// settled is the Director's verification, in the shape the Theater can be handed.
//
// # Why the adapter is this thin
//
// Because there is only one verification in this system and it already exists. `observeOutcome`
// waits for the screen to stop moving, reads an identity off it, recalls that against durable
// memory and classifies the result into the closed outcome vocabulary — including the distinction
// between "you are somewhere else" and "I looked too early", which nothing simpler can make.
//
// `production.Verifier` asks for a subject and a boolean, which is less than that. So this
// records the full finding on the step it belongs to and answers the narrower question from it.
// The alternative — the Theater verifying for itself — would be a second answer to the question
// this system asks most carefully, which is the whole of what Roadmap 34E is removing.
//
// Bound to ONE step. It is built inside the loop and discarded with it; there is no way to verify
// a later step against an earlier step's record.
type settled struct {
	l        *Live
	selector windowref.Selector
	before   windowref.Ref
	rec      *StepRecord
	// ran says this verification actually happened. The loop reads it to decide whether it
	// still has to look.
	//
	// Its OWN record, deliberately, rather than a flag on the report. Whether the step has
	// been observed is a fact about this object; taking the Theater's word for it would mean
	// a producer that reported "verified" without asking left the step with no outcome at
	// all — which is exactly what a fake doing precisely that revealed.
	ran bool
}

func (s *settled) Verify(ctx context.Context, _ production.Request) (string, bool) {
	s.ran = true
	s.l.observeOutcome(ctx, s.selector, s.before, s.rec)
	// DIRECTLY VERIFIED and nothing else. Containment holding is a reason to carry on, not
	// evidence that a press did what it was supposed to do, and the Theater must not be told
	// a production succeeded on it.
	return s.rec.Observed, s.rec.Outcome == DirectlyVerified
}

// observeOutcome settles, takes a FRESH observation, and classifies what it finds.
//
// Freshness is structural rather than checked: the totals this reads are created here, after the
// input, and every sample in them was taken after it. There is no path by which a pre-action
// observation could reach the classification, because none of them is in scope.
func (l *Live) observeOutcome(ctx context.Context, selector windowref.Selector,
	before windowref.Ref, out *StepRecord) {

	stable := false
	ref, totals, err := l.watch(ctx, selector, settleSamples, &stable)
	switch {
	case err != nil && ctx.Err() != nil:
		out.Settle, out.Cancelled, out.Outcome = SettleInterrupted, true, Unobservable
		return
	case err != nil:
		out.Settle, out.Target, out.Outcome = SettleTargetLost, TargetLost, TargetUnavailable
		return
	}
	if !sameWindow(before, ref) {
		// The window is a different window. Marco does not reacquire and carry on — a
		// future rehearsal may begin a new attempt, and this one is over.
		out.Settle, out.Target, out.Outcome = SettleStable, TargetChanged, TargetMoved
		return
	}
	out.Target = TargetHeld
	if stable {
		out.Settle = SettleStable
	} else {
		out.Settle = SettleChanging
	}

	// The SAME current-place answer the source check used, and the one Sight renders.
	p := observe.PlaceNow(totals, out.Application, l.memory, l.th)
	if !p.Placed {
		// Input was emitted and Marco cannot tell what came of it. A RUNTIME failure to
		// look, never the step's own unobservability.
		out.Outcome = Unobservable
		return
	}
	if p.Verdict == observe.MatchCandidate {
		out.Outcome = Ambiguous
		return
	}
	if !p.Established() {
		// Read perfectly well, and nowhere Marco knows. NOT a failure to look.
		out.Outcome = Unrecognised
		return
	}
	out.Observed = p.Subject

	classifyOutcome(out)
}

// classifyOutcome decides what a step's observation means, given what was expected.
//
// Split out so the decision can be driven directly. It is a pure reading of the record — no
// window, no sampler, no memory — and it is where "the screen was still moving" has to be told
// from "the screen is somewhere else", which is a distinction a live rehearsal cannot be relied
// on to reproduce on demand.
func classifyOutcome(out *StepRecord) {
	switch out.Verification {
	case observe.ProgressUnobservable:
		// Containment, checked rather than assumed: the same target, the same application,
		// and the same screen it started on. Anything else contradicts the one promise a
		// progress-unobservable step makes.
		if out.Observed == out.Expect {
			out.Outcome = ProgressUnobservable
			return
		}
		// The same rule for a contained step: containment that appears broken on a screen
		// still painting has not been shown to be broken. See below.
		if out.Settle == SettleChanging {
			out.Outcome = Unobservable
			return
		}
		out.Outcome = WrongState
	default:
		// THE specific expected subject. Not "something changed" — a screen that became a
		// different remembered screen is wrong_state, and calling change success is how a
		// procedure gets promoted for going somewhere nobody asked for.
		if out.Observed == out.Expect {
			out.Outcome = DirectlyVerified
			return
		}
		// A DISAGREEMENT READ OFF A SCREEN THAT WAS STILL MOVING IS NOT A DISAGREEMENT.
		//
		// # The live failure
		//
		// A rehearsal recognised Home, clicked through to Bluetooth, and reported
		// `wrong_state — expected subj_543793ccc326, saw subj_892a4cc30f41`. Those two
		// subjects are the same Settings page: one read after it had finished painting and
		// one read part-way through, four controls short.
		//
		// Marco had already noticed. The settle wait ran out, the record says
		// `still_changing`, and the classification went ahead and compared identities
		// anyway — turning "I looked too early" into "you went somewhere else". Worse, an
		// identity read mid-render is how the twin subjects got minted in the first place,
		// so this both produced the confusion and then reported it as the person's fault.
		//
		// The Outcome vocabulary already said so: Unobservable is documented as covering
		// "the screen never settled". This makes the code honour its own contract.
		//
		// # Why only the disagreement is withheld
		//
		// A reading that AGREES early is still agreement — the screen reached the expected
		// place and kept moving, which is an ordinary page finishing its work. A reading
		// that disagrees early is evidence of nothing: the screen had not finished being
		// the thing it was going to be.
		//
		// Deleting this must fail TestAnUnsettledScreenIsNotAWrongState.
		if out.Settle == SettleChanging {
			out.Outcome = Unobservable
			return
		}
		out.Outcome = WrongState
	}
}

// watch takes a bounded run of observations of one window.
//
// One helper for both windows — establishing and settling — because they ask the same question of
// the same machinery, and two implementations would eventually disagree about what "the current
// screen" means.
//
// When `stable` is non-nil the run stops early once the screen state has held for
// settleStableRun consecutive observations, and reports whether it did. Settling by WATCHING
// rather than by sleeping: a condition is falsifiable where a duration is not, and a satisfied
// condition finishes the instant it becomes true.
func (l *Live) watch(ctx context.Context, selector windowref.Selector, n int, stable *bool) (
	windowref.Ref, observe.ShadowTotals, error) {

	var totals observe.ShadowTotals
	var ref windowref.Ref
	run := 0
	last := observe.ScreenStateID("")
	for i := 1; i <= n; i++ {
		if err := ctx.Err(); err != nil {
			return ref, totals, refuse(RefusalCancelled, "%s", err)
		}
		got, err := l.target.Acquire(ctx, selector)
		if err != nil {
			return ref, totals, refuse(RefusalTargetLost, "the window went away: %s", err)
		}
		if !ref.Zero() && !sameWindow(ref, got) {
			return got, totals, refuse(RefusalTargetMoved, "the window changed while watching")
		}
		ref = got
		sample, err := l.sampler.Sample(ctx, observesession.SampleRequest{
			Window: ref, Sequence: i, ReadLabels: true,
		})
		if err != nil {
			return ref, totals, refuse(RefusalSourceUnobservable, "%s", err)
		}
		// THE authoritative composition, the same way every other reader gets it.
		//
		// This was `totals.Add(*sample.Shadow)`, which folds the SHADOW sample — the vision
		// experiment's own structure — and nothing else. With vision off, and it is off by
		// default, `sample.Shadow` is nil, nothing was folded at all, `CurrentState` stayed
		// empty and `SignatureOfState` refused. A rehearsal against an accessibility-only
		// application therefore ended in `source_unobservable` every single time, and could
		// not have done anything else.
		//
		// `Observe` routes through `StructureOf`, which prefers the fused world and falls
		// back to the detector. It is the call the session runner makes, and there was never
		// a reason for the two to differ: "what is on this screen" has one answer, and this
		// file was reading a different source for it.
		totals.Observe(sample)
		if stable != nil {
			if totals.CurrentState != "" && totals.CurrentState == last {
				run++
				if run >= settleStableRun {
					*stable = true
					return ref, totals, nil
				}
			} else {
				run = 0
			}
			last = totals.CurrentState
		}
		if i < n {
			select {
			case <-l.clock.After(sampleGap):
			case <-ctx.Done():
				return ref, totals, refuse(RefusalCancelled, "%s", ctx.Err())
			}
		}
	}
	return ref, totals, nil
}

// sameWindow reports that two references name the same live window.
//
// Identity, process and generation — never the title, which changes while a window lives, and
// never the bounds, which change when it is moved. This is the comparison the window tracker
// already uses to tell a reacquired window from a recycled handle.
func sameWindow(a, b windowref.Ref) bool {
	return a.ID == b.ID && a.Handle == b.Handle && a.ProcessID == b.ProcessID &&
		a.Generation == b.Generation
}

// Describe renders a result for a person.
//
// Says what was sent and what came of it, in that order, and never prints a key or an attempt
// token. The last line is the boundary of the claim: one step is not a procedure.
func (r StepRecord) Describe() []string {
	words := make([]string, 0, len(r.Intents))
	for _, in := range r.Intents {
		words = append(words, string(in))
	}
	out := []string{
		"rehearsal attempt",
		"  path: " + r.Relationship.From + " → " + r.Relationship.To,
		fmt.Sprintf("  step: %d", r.Position),
		"  started on: " + r.Source,
		"  sent: " + strings.Join(words, ", "),
	}
	if r.Outcome == "" {
		out = append(out, "  result: none. Nothing reached the computer, so there is nothing "+
			"to conclude")
		out = append(out, "  this is what WOULD be sent. Marco did not try it")
		return out
	}
	switch r.Outcome {
	case DirectlyVerified:
		out = append(out, "  result: the screen Marco expected is the screen it found ("+
			r.Observed+")")
	case ProgressUnobservable:
		out = append(out, "  result: Marco cannot see what that moved, and everything it "+
			"can see is unchanged, which is as much as this step could ever show")
	case WrongState:
		out = append(out, "  result: the screen became "+r.Observed+
			", which is not what that step was for")
	case TargetMoved:
		out = append(out, "  result: the window changed; Marco stopped rather than "+
			"following it")
	case TargetUnavailable:
		out = append(out, "  result: the window went away")
	case Ambiguous:
		out = append(out, "  result: more than one remembered screen fits what is there now")
	case Unobservable:
		out = append(out, "  result: Marco could not see well enough afterwards to say")
	case InputFailed:
		out = append(out, "  result: the input did not go through")
	}
	out = append(out, "  settled: "+string(r.Settle), "  window: "+string(r.Target))
	if r.Cancelled {
		out = append(out, "  you stopped it after the input had gone out")
	}
	out = append(out, "  this proves at most one step. The procedure is not verified, "+
		"nothing was learned, and nothing was saved")
	return out
}
