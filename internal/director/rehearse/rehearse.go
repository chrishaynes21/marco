// Package rehearse turns one authorized rehearsal step into the exact thing Marco would send,
// and stops there.
//
// # Where this sits
//
//	RehearsalGrant          the user said yes to one attempt          (observe)
//	  → Attempt          the grant is claimed and scope is fixed   (here)
//	  → one StepPlan        exactly one step, chosen and checked      (here)
//	  → marcoexec.Operation the Director's typed effect               (marcoexec)
//	  → legal Marco source  compiled by Marco's own compiler          (marcoexec)
//	  → runtime.Host        the last thing before a real computer
//
// Everything from `marcoexec.Operation` down is the path a live rehearsal will use unchanged.
// The ONLY difference in this milestone is which Host is installed at the bottom: a recorder
// instead of the OS. That is deliberate and it is the whole design — a dry run that took a
// shortcut past the compiler would prove the shortcut works.
//
// # What this package may not do
//
// It holds no host, opens no window, reads no screen and imports nothing that can. The runner it
// is given is an interface (`directorapi.MarcoRunner`); whoever constructs one decides what is at
// the other end, and `cmd/director` is the only place that decides.
//
// See [[ADR-023-rehearsal-is-attempt-scoped-authority]] and [[Marco-Boundary]].
package rehearse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/production"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// hasPoint reports whether a step's intents include a pointer press.
func hasPoint(in []observe.NavIntent) bool {
	for _, i := range in {
		if i == observe.NavPoint {
			return true
		}
	}
	return false
}

// ── why an attempt was refused ────────────────────────────────────────────────

// Refusal is the CLOSED vocabulary of why nothing was produced.
//
// Every value here means the same thing about the world: **zero effects reached the boundary**.
// There is no partial state and no "produced some of it", which is why the vocabulary describes
// the check that failed rather than how far it got.
type Refusal string

const (
	// RefusalNoGrant is the default and the important one: no authorization at all.
	RefusalNoGrant Refusal = "no_grant"
	// RefusalGrantSpent is a grant already used. One attempt means one.
	RefusalGrantSpent Refusal = "grant_spent"
	// RefusalGrantRevoked is a grant withdrawn by cancellation.
	RefusalGrantRevoked Refusal = "grant_revoked"
	// RefusalGrantExpired is a permission older than its own duration bound.
	RefusalGrantExpired Refusal = "grant_expired"
	// RefusalApplicationMismatch, RefusalSourceMismatch and RefusalEvidenceMismatch are the
	// scope checks, all three performed by the grant itself at claim time.
	RefusalApplicationMismatch Refusal = "application_mismatch"
	RefusalSourceMismatch      Refusal = "source_mismatch"
	RefusalEvidenceMismatch    Refusal = "evidence_mismatch"
	// RefusalRelationshipMismatch is a scope naming a different route.
	RefusalRelationshipMismatch Refusal = "relationship_mismatch"
	// RefusalNoSuchStep is a position the plan does not have.
	RefusalNoSuchStep Refusal = "no_such_step"
	// RefusalStepUnverifiable is a step Marco could not check afterwards. It never reaches
	// the boundary, dry or otherwise.
	RefusalStepUnverifiable Refusal = "step_unverifiable"
	// RefusalInputBound is a step whose whole ordered run does not fit the remaining budget.
	//
	// Refused BEFORE the first input rather than truncated at the bound, because half of
	// `down, down, confirm` is not a smaller version of the procedure — it is a different one,
	// and it leaves the interface somewhere no demonstration ever went.
	RefusalInputBound Refusal = "input_bound_exceeded"
	// RefusalUnobservableBound is a step that would extend an unseeable run past what the
	// authorized plan contains.
	RefusalUnobservableBound Refusal = "unobservable_bound_exceeded"
	// RefusalAttemptTerminal is an attempt that has already produced its one step or ended.
	RefusalAttemptTerminal Refusal = "attempt_already_finished"
	// RefusalCancelled is an attempt cancelled before its step was lowered.
	RefusalCancelled Refusal = "cancelled"
	// RefusalLowering is a step the Director could not express in legal Marco, or that the
	// compiler rejected. Fail-closed: nothing ran.
	RefusalLowering Refusal = "cannot_express"
	// RefusalControlNotFound is a named control the live window is not offering right now.
	// Raised before anything is emitted: an invocation with no element is not an action,
	// and guessing a different control would be acting on something nobody demonstrated.
	RefusalControlNotFound Refusal = "control_not_found"
)

// Error is a refusal carrying the sentence a person reads.
type Error struct {
	Refusal Refusal
	Detail  string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return string(e.Refusal)
	}
	return string(e.Refusal) + ": " + e.Detail
}

func refuse(r Refusal, format string, args ...any) error {
	return &Error{Refusal: r, Detail: fmt.Sprintf(format, args...)}
}

// RefusalOf reports the closed reason behind an error, if it has one.
func RefusalOf(err error) (Refusal, bool) {
	var e *Error
	if err == nil {
		return "", false
	}
	if as, ok := err.(*Error); ok {
		e = as
	}
	if e == nil {
		return "", false
	}
	return e.Refusal, true
}

// ── the facts an attempt must be given ────────────────────────────────────────

// Scope is what is true NOW, supplied by whoever is arranging the attempt.
//
// Deliberately an argument rather than something this package works out. Establishing where the
// interface actually is means looking at a screen, and looking at a screen is the next
// milestone's job; taking it as input means the scope check can be proved deterministically
// before anything can look at anything.
//
// The grant is the authority. This is the claim being made against it, and every field is
// compared rather than trusted.
type Scope struct {
	// Application is which program is in front.
	Application string
	// Source is the remembered subject the interface is believed to be showing.
	Source string
	// Relationship is the route the caller believes it is attempting.
	Relationship observe.RelationshipRef
	// Evidence is the rehearsal digest of the candidate as it stands now, recomputed by the
	// caller from live memory — never carried over from when the question was asked.
	Evidence string
}

// ── the attempt ───────────────────────────────────────────────────────────────

// AttemptState is where one attempt has got to.
//
// # The whole point of this type
//
// The transition graph makes ACT → ACT impossible. A step may be lowered only from `open`, and
// lowering moves the attempt to `acted`, which nothing but an OBSERVATION can leave. Authority
// therefore cannot outrun observation by construction rather than by an orchestrator remembering
// to look — and an orchestrator that forgot would be refused rather than typing twice.
//
//	open --lower--> acted --observe--> open          the only loop
//	  |               |                  |
//	  +---------------+------------------+--> finished | cancelled
type AttemptState string

const (
	// AttemptOpen has claimed its grant and may lower ONE step.
	AttemptOpen AttemptState = "open"
	// AttemptActed has emitted a step and is waiting to be told what came of it.
	//
	// The state that makes the invariant structural: nothing can be lowered from here.
	AttemptActed AttemptState = "acted"
	// AttemptFinished reached a terminal outcome. Terminal.
	AttemptFinished AttemptState = "finished"
	// AttemptCancelled was stopped. Terminal.
	AttemptCancelled AttemptState = "cancelled"
)

// Attempt is one claimed authorization, fixed to one scope.
//
// # Why the grant is already spent by the time this exists
//
// `BeginAttempt` claims the grant FIRST and builds everything else afterwards. If construction
// then fails, the grant stays spent — it is not handed back. That is the point: the alternative
// is claim, fail, retry with the same permission, which turns "one attempt" into "as many
// attempts as it takes to get past the setup".
//
// The user said yes to one try. A try that fell over during preparation was still the try.
type Attempt struct {
	// ID is the grant's own attempt identity. Not printed in ordinary output — see Describe.
	ID string
	// Application, Source, Destination and Relationship are the fixed scope.
	Application  string
	Source       string
	Destination  string
	Relationship observe.RelationshipRef
	// Evidence is the digest this attempt is authorized against.
	Evidence string
	// Plan is the authorized plan. Read-only here; this package never re-judges it.
	Plan []observe.StepPlan

	remainingInputs int
	maxUnobservable int
	maxSteps        int
	taken           int
	state           AttemptState
}

// BeginAttempt claims a grant and fixes one attempt to one scope.
//
// THE claim point, and the only one. Nothing downstream of here can be reached without an
// authorization that was live, in scope, unexpired and unspent at the moment of this call.
//
// Order matters and is load-bearing: scope is compared, then the grant is consumed, and only then
// does an attempt exist at all. A caller cannot get a Attempt without spending a grant, and
// cannot spend a grant without matching the scope.
func BeginAttempt(g *observe.RehearsalGrant, j observe.RehearsalJudgement, s Scope,
	now time.Time) (*Attempt, error) {

	if g == nil {
		return nil, refuse(RefusalNoGrant, "nothing has authorized this")
	}
	// The cheap, unambiguous mismatches first, so the reason a person is given names the
	// thing that was actually wrong rather than whatever the grant checked first.
	if !strings.EqualFold(s.Application, g.Application) {
		return nil, refuse(RefusalApplicationMismatch,
			"authorized for %s, and %s is in front", g.Application, s.Application)
	}
	if s.Source != g.Source {
		return nil, refuse(RefusalSourceMismatch,
			"authorized to start from one screen, and this is a different one")
	}
	if s.Relationship != g.Relationship {
		return nil, refuse(RefusalRelationshipMismatch, "this authorization is for another route")
	}
	if s.Evidence != g.Evidence {
		return nil, refuse(RefusalEvidenceMismatch,
			"the demonstration this was authorized against has changed since")
	}
	switch g.State() {
	case observe.GrantConsumed:
		return nil, refuse(RefusalGrantSpent, "one authorization permits one attempt")
	case observe.GrantRevoked:
		return nil, refuse(RefusalGrantRevoked, "this authorization was withdrawn")
	}

	// ── THE claim. Everything above is a question; this is the moment the permission
	// ── is spent, and it happens before a single semantic input can exist.
	if err := g.BeginAttempt(s.Application, s.Source, s.Evidence, now); err != nil {
		// The grant's own checks, which are the authority. Reached when something moved
		// between the comparisons above and this line — a concurrent cancellation, or the
		// clock crossing the duration bound.
		if g.State() == observe.GrantRevoked {
			return nil, refuse(RefusalGrantRevoked, "this authorization was withdrawn")
		}
		if !now.IsZero() && now.Sub(g.IssuedAt) > g.MaxDuration {
			return nil, refuse(RefusalGrantExpired,
				"this permission was given %s ago", now.Sub(g.IssuedAt).Round(time.Second))
		}
		return nil, refuse(RefusalGrantSpent, "%s", err)
	}

	return &Attempt{
		ID:              g.ID,
		Application:     g.Application,
		Source:          g.Source,
		Destination:     g.Destination,
		Relationship:    g.Relationship,
		Evidence:        g.Evidence,
		Plan:            append([]observe.StepPlan{}, j.Plan...),
		remainingInputs: g.MaxInputs,
		maxSteps:        len(j.Plan),
		maxUnobservable: g.MaxUnobservable,
		state:           AttemptOpen,
	}, nil
}

// State reports where the attempt has got to.
func (a *Attempt) State() AttemptState {
	if a == nil {
		return AttemptCancelled
	}
	return a.state
}

// Cancel ends the attempt without producing anything.
//
// Idempotent, and it never un-finishes a lowered attempt: a record of what would have been sent
// is not withdrawn by a later change of mind, because it was already true.
func (a *Attempt) Cancel() {
	if a == nil || a.state != AttemptOpen {
		return
	}
	a.state = AttemptCancelled
}

// ── what one dry step produced ────────────────────────────────────────────────

// EmissionStatus is what happened, in the only two words this milestone is entitled to.
type EmissionStatus string

const (
	// EmissionSent means the whole path ran and a recorder holds what the host was asked
	// for. NOT "success", NOT "verified", NOT "rehearsed": no key was pressed, nothing was
	// observed afterwards, and the world is exactly as it was.
	EmissionSent EmissionStatus = "sent_to_host"
	// EmissionRefused means nothing reached the boundary.
	EmissionRefused EmissionStatus = "refused"
)

// StepEmission is the inert record this milestone exists to produce.
//
// It is an engineering artefact and nothing else. It is not evidence, it does not count towards
// anything, and no part of the learning loop reads it — see [[ADR-024-a-dry-step-is-not-evidence]].
type StepEmission struct {
	// Attempt is the claimed authorization's identity.
	Attempt string
	// Relationship, Position and Intents say which step, and what it would send in order.
	Relationship observe.RelationshipRef
	Position     int
	Intents      []observe.NavIntent
	// Verification is the step's classification, carried through lowering UNCHANGED.
	//
	// A `progress_unobservable` step that arrived here as `directly_verifiable` would be a
	// claim that an action establishes something it cannot, which is the exact collapse the
	// three-valued vocabulary exists to prevent.
	Verification observe.StepVerifiability
	// Expect is what the screen must show afterwards: for a directly-verifiable step the
	// subject it ARRIVES at, for an unobservable one the subject it must REMAIN on.
	//
	// Carried, and NOT CHECKED. Checking it means observing, and nothing has been observed —
	// this milestone only proves the expectation survives the trip.
	Expect string
	// Status is `would_emit` or `refused`.
	Status EmissionStatus
	// Refusal is why not, empty when the step was lowered.
	Refusal Refusal
	// Program is the legal Marco the Director produced, kept for inspection.
	Program string
	// Reached says the program was handed to the runner.
	//
	// The line between "nothing was sent" and "something may have been". A lowering that
	// failed before this is a refusal; one that failed after it is a RESULT, because by then
	// part of the run may already have landed and claiming otherwise would be a claim about
	// the world that is not available.
	Reached bool
	// Emitted is what the host was actually asked for, in order, as the host recorded it.
	Emitted []string
	// And NOTHING about the result. No Verified, no Outcome, no Checked: an emission is what
	// Marco sent, and whether it worked belongs to the StepRecord.
	//
	// A production that verifies itself was briefly reported here, so the caller would know
	// not to look again. It is asked of the caller's own verifier instead — whether the step
	// has been observed is a fact about that object, and a producer that claimed to have
	// verified without asking would otherwise leave the step with no outcome at all.
	// See TestAStepEmissionClaimsNothing and TestAnUnverifiedProductionIsStillObserved.
}

// Recorder is whatever holds what the host was asked for.
//
// An interface so this package never has to know what is at the bottom. The recording host
// satisfies it; a live host does not, and could not be handed here by accident.
type Recorder interface {
	// Lines returns each call the host received, in order.
	Lines() []string
}

// LowerStep produces exactly one step of the authorized plan and stops.
//
// # The order, which is the milestone
//
//  1. every precheck, with zero effects if any fails
//  2. the Director's typed Operation
//  3. legal Marco, through marcoexec — the one path to a desktop effect
//  4. the compiler and the runtime
//  5. whatever Host was installed
//
// Step 1 finishing before step 5 begins is the property Part 13 asks for: a bound violation is
// discovered before the first input exists, not after the first one has been sent.
//
// # One step
//
// There is no loop here and no continuation. The attempt becomes terminal, and a second step
// needs a second authorization — which does not exist, because a grant permits one attempt.
// Stage is what a caller brings to a production: the Theater that can put one on, and the
// verification the caller is able to lend it.
//
// # Why both travel together and neither is held
//
// The Director does not own a Theater. It is handed a `production.Producer` by whoever composed
// it, exactly as it is handed a `MarcoRunner` — that is the boundary this package's import test
// enforces, and the reason a rehearsal cannot act by itself.
//
// The VERIFIER is the caller's, not the stage's. A dry run brings none, because nothing reached a
// computer and there is nothing to look at. A live rehearsal brings the Director's own settled
// observation, which is the only verification in this system that can tell "the screen is
// somewhere else" from "the screen had not finished painting".
type Stage struct {
	// Produce puts a production on. Nil means this Director has no Theater, and a press
	// refuses before emitting rather than reaching for something else.
	Produce production.Producer
	// Verify is what will check the result, or nil when the caller cannot check.
	Verify production.Verifier
}

func (a *Attempt) LowerStep(ctx context.Context, position int,
	runner directorapi.MarcoRunner, rec Recorder, window windowref.Ref,
	stage Stage) (StepEmission, error) {

	if a == nil {
		return StepEmission{Status: EmissionRefused, Refusal: RefusalNoGrant}, refuse(RefusalNoGrant,
			"there is no attempt")
	}
	out := StepEmission{Attempt: a.ID, Relationship: a.Relationship, Position: position,
		Status: EmissionRefused}

	fail := func(r Refusal, format string, args ...any) (StepEmission, error) {
		out.Refusal = r
		return out, refuse(r, format, args...)
	}

	switch a.state {
	case AttemptCancelled:
		return fail(RefusalCancelled, "this attempt was cancelled")
	case AttemptActed:
		return fail(RefusalAwaitingObservation,
			"the last step has not been observed yet; Marco looks before it acts again")
	case AttemptFinished:
		return fail(RefusalAttemptTerminal, "this attempt is over")
	}
	if position < 1 || position > len(a.Plan) {
		return fail(RefusalNoSuchStep, "the authorized plan has %d step(s)", len(a.Plan))
	}
	step := a.Plan[position-1]
	out.Position = step.Position
	out.Verification = step.Verifiability
	out.Expect = step.Expect
	out.Intents = append([]observe.NavIntent{}, step.Intents...)

	// ── the prechecks. Every one of them before anything can be produced. ──
	if step.Verifiability == observe.Unverifiable {
		return fail(RefusalStepUnverifiable,
			"Marco could not tell afterwards whether this step had worked")
	}
	if len(step.Intents) == 0 {
		return fail(RefusalNoSuchStep, "the step has nothing to send")
	}
	if len(step.Intents) > a.remainingInputs {
		// ATOMIC. Refused whole rather than truncated — see RefusalInputBound.
		return fail(RefusalInputBound,
			"this step sends %d input(s) and %d remain", len(step.Intents), a.remainingInputs)
	}
	if step.Verifiability == observe.ProgressUnobservable && a.maxUnobservable < 1 {
		return fail(RefusalUnobservableBound,
			"the authorized plan contains no step Marco cannot see the result of")
	}
	if err := ctx.Err(); err != nil {
		return fail(RefusalCancelled, "%s", err)
	}

	// ── the Director's typed effect. Meanings, never keys: the Director learned that
	// ── somebody confirmed, and which key that is belongs to the host. A pointer press
	// ── with a named target is the third shape: the Director learned that somebody
	// ── ACTIVATED a control called something, and it asks the application to activate
	// ── that control through its own accessibility pattern — no coordinate is aimed,
	// ── nothing can be obscured, and a control that cannot do it refuses out loud.
	if hasPoint(step.Intents) {
		// ── A PRESS IS A PRODUCTION. Handed whole to the Theater. ──
		//
		// It used to be lowered here: resolve the name to a live element id, build a
		// KindInvoke operation, walk a private fallback ladder. That was the second body.
		// A saved play asking for the same control went through the Theater, which knew
		// about selection items, scoped by window and refused an ambiguous name; a
		// rehearsal did none of it, and only one of the two ever learned anything.
		//
		// What crosses the boundary now is semantic and nothing else — a name, a kind, a
		// window. No element id, so nothing here can go stale between deciding and doing.
		//
		// Deleting this must fail TestALivePressIsPutOnByTheTheater.
		switch {
		case len(step.Intents) != 1:
			// A press sharing a leg with other input is a shape no demonstration in this
			// repository has produced; expressing it needs its own design, not a guess.
			return fail(RefusalLowering,
				"a pointer press shares this step with other input; Marco cannot say that")
		case len(step.Targets) == 0 || !step.Targets[0].Named():
			return fail(RefusalLowering,
				"a pointer press with no named control; there is nothing to aim at")
		case stage.Produce == nil:
			return fail(RefusalLowering,
				"this Director has no Theater, so it cannot press a named control")
		}
		req := production.Request{
			Target: production.Target{
				Name: step.Targets[0].Label,
				Kind: step.Targets[0].Role,
			},
			Window: string(window.ID),
			Expect: step.Expect,
		}
		// AUTHORITY for exactly this press, spendable once. The attempt has already
		// passed every bound above; this is what makes the permission unusable for
		// anything other than the target the plan authorised.
		report := stage.Produce.Perform(ctx, req, &pressGrant{target: req.Target.Name},
			stage.Verify)
		out.Program = report.Program
		// REACHED means part of the production may already have landed, which is the
		// line between a refusal and a result.
		//
		// Performed says an Actor acted. `perform_failed` is the other side of it: the
		// cast program RAN and the capability declined, so something did reach the
		// machine even though the Theater does not call that performed. Everything else
		// in the vocabulary — not permitted, target not found, ambiguous, no actor —
		// stopped before anything was sent, and reporting those as results would throw
		// away the distinction between "Marco declined to try" and "Marco tried".
		out.Reached = report.Performed || report.Refused == production.PerformFailed
		if !report.Performed {
			return fail(refusalFor(report.Refused), "%s", production.Describe(report))
		}
		// PERFORMED and not verified is not a failure here. The live loop verifies
		// afterwards with the full outcome vocabulary — see mayContinue — and a caller
		// that brought no verifier was never claiming otherwise.
	} else {
		op := marcoexec.Operation{Kind: marcoexec.KindNavigate}
		for _, in := range step.Intents {
			op.Intents = append(op.Intents, string(in))
		}
		program, err := marcoexec.Lower(op)
		if err != nil {
			return fail(RefusalLowering, "%s", err)
		}
		out.Program = program

		// ── and down the real path. marcoexec compiles before it runs, so a program
		// ── that cannot be expressed never reaches a host at all.
		out.Reached = true
		res, err := marcoexec.New(runner).Do(ctx, op)
		if err != nil {
			return fail(RefusalLowering, "%s", err)
		}
		if res.Failed() {
			return fail(RefusalLowering, "%s", res.Summary())
		}
	}

	a.remainingInputs -= len(step.Intents)
	if step.Verifiability == observe.ProgressUnobservable {
		a.maxUnobservable--
	}
	a.state = AttemptActed
	a.taken++
	out.Status = EmissionSent
	if rec != nil {
		out.Emitted = rec.Lines()
	}
	return out, nil
}

// ── what a person is told ─────────────────────────────────────────────────────

// Describe renders a dry step for a person.
//
// Two things are deliberately absent. There is no attempt id: the identity of a claimed
// authorization is a token for one experiment, and a token printed in ordinary status output ends
// up in scrollback, in a screenshot and in a bug report. And there is no key: the Director does
// not know one, and a line reading `Enter` would be this layer inventing a binding it has no
// standing to choose.
//
// The last line is the one that matters. Whatever else this says, nothing happened.
func (s StepEmission) Describe() []string {
	out := []string{"rehearsal dry run"}
	out = append(out, "  path: "+s.Relationship.From+" → "+s.Relationship.To)
	out = append(out, fmt.Sprintf("  step: %d", s.Position))
	if len(s.Intents) > 0 {
		words := make([]string, 0, len(s.Intents))
		for _, in := range s.Intents {
			words = append(words, string(in))
		}
		out = append(out, "  intent: "+strings.Join(words, ", "))
	}
	switch s.Verification {
	case observe.DirectlyVerifiable:
		out = append(out, "  verification: directly verifiable")
		if s.Expect != "" {
			out = append(out, "  expected afterwards: the screen Marco knows as "+s.Expect)
		}
	case observe.ProgressUnobservable:
		out = append(out, "  verification: progress unobservable")
		out = append(out, "  expected afterwards: still the screen it started on; Marco "+
			"cannot see what this moves")
	case observe.Unverifiable:
		out = append(out, "  verification: unverifiable")
	}
	switch s.Status {
	case EmissionSent:
		out = append(out, fmt.Sprintf("  effect: would emit %d call(s)", len(s.Emitted)))
		for i, line := range s.Emitted {
			out = append(out, fmt.Sprintf("    %d. %s", i+1, line))
		}
	default:
		out = append(out, "  effect: none")
		if s.Refusal != "" {
			out = append(out, "  refused: "+string(s.Refusal))
		}
	}
	out = append(out, "  real input: none. Nothing was sent and nothing was observed; "+
		"this is what WOULD be sent")
	return out
}

// Refusals that belong to a LIVE attempt: reasons Marco never got as far as being able to act.
//
// Kept in the same closed vocabulary as the rest, and meaning the same thing — zero effects — but
// listed apart because every one of them is about the world rather than about the authorization.
const (
	// RefusalNoActuator is a runner that was never made capable of acting. The default.
	RefusalNoActuator Refusal = "no_actuator"
	// RefusalSourceUnobservable is perception that could not make out the screen at all.
	RefusalSourceUnobservable Refusal = "source_unobservable"
	// RefusalSourceUnrecognised is a screen Marco is looking at and does not remember.
	RefusalSourceUnrecognised Refusal = "source_unrecognised"
	// RefusalSourceAmbiguous is a screen resembling more than one remembered subject. Marco
	// does not pick the nearest.
	RefusalSourceAmbiguous Refusal = "source_ambiguous"
	// RefusalTargetLost is the window going away before anything was sent.
	RefusalTargetLost Refusal = "target_lost"
	// RefusalWindowBehind is the watched window not leading the desktop when a REAL
	// actuator was about to emit.
	//
	// Emitted input has no address — whatever is in front receives it — so acting now would
	// act on the wrong application and then honestly report that the right one never moved.
	// Raised before the grant is claimed, so waiting for the person to bring the window
	// forward costs nothing; the teach coordinator treats it as the patient case.
	RefusalWindowBehind Refusal = "window_not_in_front"
	// RefusalTargetMoved is the window changing between checking the screen and acting on
	// it — the race the final guard exists to close.
	RefusalTargetMoved Refusal = "target_moved"
)

// RefusalAwaitingObservation is a second step asked for before the first was looked at.
//
// The refusal that enforces the governing invariant. Reached only by a caller that tried to act
// twice in a row, which is the one thing the state machine exists to prevent.
const RefusalAwaitingObservation Refusal = "awaiting_observation"

// Observed tells the attempt what came of the step it just took, and reopens it if reality allows.
//
// THE only way out of `acted`. `mayContinue` is the orchestrator's reading of the world — not this
// type's, because classifying a screen needs perception and this package's Attempt has none.
//
// Returns whether another step may be lowered. False is terminal and stays terminal.
func (a *Attempt) Observed(mayContinue bool) bool {
	if a == nil || a.state != AttemptActed {
		return false
	}
	if !mayContinue || a.taken >= a.maxSteps || a.remainingInputs <= 0 {
		a.state = AttemptFinished
		return false
	}
	a.state = AttemptOpen
	return true
}

// Finish ends the attempt without another step. Idempotent; never resurrects a cancelled one.
func (a *Attempt) Finish() {
	if a == nil || a.state == AttemptCancelled || a.state == AttemptFinished {
		return
	}
	a.state = AttemptFinished
}

// StepsTaken is how many steps have been emitted.
func (a *Attempt) StepsTaken() int {
	if a == nil {
		return 0
	}
	return a.taken
}

// RemainingInputs is how much of the authorized input budget is left.
func (a *Attempt) RemainingInputs() int {
	if a == nil {
		return 0
	}
	return a.remainingInputs
}

// ── authority for one press ───────────────────────────────────────────────────

// pressGrant is permission to put on exactly one production, for exactly one target.
//
// # Why the Theater is handed something spendable rather than told "yes"
//
// Because a boolean is a claim the caller has to be trusted not to reuse. Every bound that makes
// a rehearsal safe — the input budget, the unobservable budget, one step then look — has already
// been checked by the time this is minted, and none of it is visible from inside the Theater. So
// what crosses the boundary is a permission that refuses a second claim and refuses a different
// target, and a Theater that retried would be told no by the authority rather than relied upon.
//
// It is minted per press and never stored. There is no way to hold one over to the next step.
type pressGrant struct {
	target string
	spent  bool
}

func (g *pressGrant) Claim(r production.Request) error {
	if g.spent {
		return fmt.Errorf("this step's permission has already been spent")
	}
	if r.Target.Name != g.target {
		return fmt.Errorf("permitted to press %q, asked for %q", g.target, r.Target.Name)
	}
	g.spent = true
	return nil
}

// refusalFor translates the production vocabulary into this package's.
//
// # Why two vocabularies exist at all
//
// Because they answer different questions. `production.Refusal` says what happened on the stage;
// `rehearse.Refusal` says what happened to an ATTEMPT, and it is what a rehearsal record and the
// UI are written against. Mapping is the price of the boundary, and it is a small price: the
// alternative is the Director importing the Theater's words, which is the coupling the whole of
// Roadmap 34E exists to remove.
//
// `target_not_found` and `target_ambiguous` both become control_not_found: from the attempt's
// point of view the named control was not there to press, and the sentence carries which it was.
func refusalFor(r production.Refusal) Refusal {
	switch r {
	case production.TargetNotFound, production.TargetAmbiguous:
		return RefusalControlNotFound
	case production.NotPermitted:
		return RefusalNoGrant
	}
	return RefusalLowering
}
