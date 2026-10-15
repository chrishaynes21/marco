// Package workflowrun advances a saved workflow one grounded action at a time.
//
// # What it is
//
// The missing arrow. `workflowstore` can reload a workflow somebody demonstrated, ground one of
// its screens against the observation in front of it, re-acquire a control from the file's own
// readings, and report that a step is PLANNABLE — a point and an expectation. Up to now nothing
// could take that point and press it, and `workflowstore`'s own import walk keeps it that way on
// purpose: a saved workflow that could lower itself would be an executable file in the strongest
// sense.
//
// So the coordination lives here, outside the thing being coordinated, and it is small by
// intent. It holds no measure, captures nothing, asks nobody anything, and cannot build a host.
// What it does is sequence five operations somebody else performs, in an order that cannot be
// rearranged:
//
//	ground the screen   →  somebody looking, or an inference from somebody looking
//	re-acquire          →  the file's own readings against this look
//	authorise           →  somebody's permission, for this step, in this observation
//	act                 →  legal Marco, through the one path to an effect
//	establish           →  what the NEXT observation says about the destination
//
// # Why every fact about the present is passed in
//
// For the same reason `workflowstore.Now` is: this package can see no screen. A coordinator that
// captured its own pixels would be a second perception path that could disagree with the one the
// plan was made against, and the disagreement would be invisible — the plan would be about one
// screen and the press about another.
//
// # Why it cannot act by default
//
// `Runner` is built with an artifact and eyes, both of which are incapable of emitting, and stays
// incapable until a composition root hands it a `*marcoexec.Executor`. That is the same shape
// `rehearse.Live` has and it exists for the same reason: a package that defaulted to a real host
// would make every test one mistake away from clicking on somebody's desktop.
//
// # What it refuses, and why each refusal is its own word
//
//   - nobody could see — a failed capture or a minimised window is not a screen and never a
//     destination, so nothing is established from it and nothing is pressed on it.
//   - the target was covered — a capture by window can return the pixels of whatever is in
//     front, under the target's own name, so a look taken while something else led the desktop
//     is not evidence about the application.
//   - the target does not lead the desktop NOW — asked again immediately before the press,
//     because a click carries no address. It lands wherever the pointer is, in whatever window
//     is in front at that instant, and a look taken 100ms earlier is not that instant. An edit
//     resolved against Notepad once executed into a Discord message box; nothing detected it,
//     because input does not fail when it lands somewhere else, it succeeds.
//   - nobody authorised this — and an authorisation is for one step in one observation.
//   - the authorisation was given about another screen — see `Authorisation`.
//   - somebody stopped the run — checked again immediately before the press, so stopping
//     during the look that planned an action prevents that action rather than the next one.
//
// # And the one it will not make
//
// Arriving where a step expected is not proof that the step's action caused it. `Report` carries
// `workflowstore.Arrival` unchanged, including `Independent` and the always-false `Verified`, so
// no caller of this package can print an arrival and imply a success.
package workflowrun

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// DefaultPatience is how many observations one step is given before the run stops safely.
//
// Forty, which is four seconds at the ten-looks-a-second cadence
// [[ADR-175-no-settling-rule-survives-a-four-second-cadence]] established. A step is retried
// rather than failed because a control mid-animation is the ordinary case and a transient
// re-acquisition miss is not a reason to abandon a workflow — and a bound exists at all because
// a coordinator with infinite patience is a loop that never reports anything.
const DefaultPatience = 40

// DefaultDwell is how long the pointer rests on a control before the button goes down.
//
// # Why a click is not one event
//
// The Windows host sends an absolute move, a button-down and a button-up in ONE SendInput
// batch, so they arrive with nothing between them. That is deliberate and right for a desktop
// control: one batch cannot be interleaved with anything else, and the down and up carry their
// own position so the press cannot land at a stale spot.
//
// It is wrong for an interface that tracks the pointer. A game menu decides what is under the
// cursor on its own frame tick and lights it up then; a button-down arriving in the same batch
// as the move is a press on a control the interface has not yet noticed the pointer is over.
// Nothing registers, nothing moves, and the press reads as a click that missed.
//
// Measured, not assumed: a supervised live run put a real left click in the middle of Rocket
// League's PLAY button and the menu did not change. The point was right. The timing was not.
//
// 250ms is a starting value for the experiment, not a law. It is several frames at any refresh
// rate a game runs at, and short enough that the gap it opens — see the re-check after it — is
// a quarter of a second rather than a pause somebody could walk away during.
const DefaultDwell = 250 * time.Millisecond

// Point is a press position in DESKTOP coordinates.
//
// Its own type rather than `observe.PointerAt`, because that one is normalised to the window and
// this one is not, and confusing the two produces a click at the top-left corner of the desktop
// that looks exactly like a working program.
type Point struct {
	X, Y int
}

// Sighting is everything the coordinator is told about one observation.
type Sighting struct {
	// Screen is the screen hypothesis standing now, session-local.
	Screen observe.VisualScreenID
	// Epoch is the observation generation and Generation the window generation. Both are
	// carried, both are compared, and neither is inferred here — see
	// [[ADR-186-a-person-can-say-where-they-are]].
	Epoch      uint64
	Generation uint64
	// Window is which window was looked at, and Bounds its rectangle in desktop
	// coordinates.
	//
	// Bounds is load-bearing rather than informational: a plannable action's point is
	// normalised to the WINDOW, and the only thing that turns it into a press is this
	// rectangle. A coordinator that assumed the window filled the screen would click
	// somewhere else on every machine where it did not.
	Window directorapi.WindowID
	Bounds directorapi.Rect
	// Front says the TARGET was the window in front when the pixels were taken.
	Front bool
	// Missing says no pixels were obtained at all: the capture itself failed.
	Missing bool
	// Unobserved says pixels arrived and no screen could be made of them — a covered
	// window, a blank frame, a screen the observer will not name.
	//
	// SEPARATE FROM Missing, and the separation is not pedantry. Folding the two meant a
	// run whose window was covered for every one of 250 observations reported "no pixels
	// were obtained" 250 times, and the captures had in fact all succeeded. One of those
	// sentences sends somebody to look at the capture path and the other sends them to
	// look at what is in front of the application, and they are different afternoons.
	Unobserved bool
	// Outcomes is what re-acquisition said about this look, from the caller's own
	// candidate set. Passed through to `workflowstore` untouched.
	Outcomes []observe.Reacquisition
	// Candidates is every region the sources proposed on this look and Words every
	// word read on it, both normalised to the window, and both passed through
	// untouched as well.
	//
	// They are what lets readiness ask whether a geometric match IS the control it
	// was matched to. This package neither looks nor judges: a coordinator that
	// decided what a region said would be deciding its own click target.
	Candidates []observe.Region
	Words      []observe.ReadWord
	// At is when the look was taken.
	At time.Time
}

// Usable reports whether anything may be concluded from this look, and why not.
func (s Sighting) Usable() (bool, string) {
	switch {
	case s.Missing:
		return false, "no pixels were obtained, so this is not a screen and nothing " +
			"can be established from it"
	case !s.Front:
		return false, "the target was not the window in front, so these pixels may " +
			"belong to whatever is covering it and are not evidence about the " +
			"application"
	case s.Unobserved:
		return false, "pixels arrived and no screen could be made of them, which is " +
			"what a covered or blank window looks like from here"
	case s.Screen == "":
		return false, "no screen hypothesis is standing, so there is nothing to " +
			"ground a step against"
	case s.Bounds.Width <= 0 || s.Bounds.Height <= 0:
		return false, "the window has no rectangle, so a point normalised to it " +
			"cannot become a press"
	}
	return true, ""
}

// Eyes supplies the next observation.
//
// One call, one look. The coordinator does not own a cadence: a live caller already has one and
// calls `Advance` from inside it, and `Run` exists for a caller that would rather be driven.
type Eyes interface {
	Look(ctx context.Context) (Sighting, error)
}

// Grounder runs the establishment path over a fresh observation.
//
// Handed the session so it can record what it established, and nothing is read back except
// through `workflowstore`'s own accessors — so this cannot invent a binding the establishment
// contract would not have made.
//
// It is an interface because establishing a screen is a person's answer or an inference from
// one, and [[Experiment-036-recognising-a-screen-you-were-shown]] measured why the coordinator
// may not do it itself: similarity ranks a present screen correctly six times out of six and
// nominates an absent one just as confidently, so what is missing is not a better measure.
type Grounder interface {
	Establish(ctx context.Context, s *workflowstore.SessionBinding, at Sighting) error
}

// Regrounder is asked where a control is, when the file cannot say.
//
// # Why this is somebody else's job
//
// Four of the nine controls in the measured artifact are bands across the window. They
// re-acquire perfectly and locate nothing, and `Region.Locates` refuses them — which is right,
// and leaves a step that can never be performed however long it is given.
//
// The repair is not arithmetic. It needs the live screen, candidates from whatever sources are
// running, and whatever evidence distinguishes one of them — which is everything this package
// deliberately cannot see. So it is an interface, like the grounder and the granter, and for the
// same reason: a coordinator that could decide where a control is from the inside would be
// choosing its own click target.
//
// Returning false is a complete answer and means the step stays refused. The region it returns
// is used for THIS SESSION only; nothing here writes to the artifact.
//
// The stage is passed because the two reasons for asking are different findings and a repair
// that described itself as the wrong one would send its reader to the wrong place. See
// `wantsReground` for which stages qualify and when.
type Regrounder interface {
	Reground(ctx context.Context, step int, control string, stage workflowstore.StepStage,
		at Sighting) (observe.Region, string, bool)
}

// Outcome is what one call to Advance did.
type Outcome string

const (
	// OutcomeUnobserved: nobody could see. Nothing established, nothing pressed.
	OutcomeUnobserved Outcome = "unobserved"
	// OutcomeWaiting: the step is not ready yet and has patience left.
	OutcomeWaiting Outcome = "waiting"
	// OutcomeArrived: the step's destination is established in this observation, so the
	// walk moved on. NOT a claim that the step's action caused it.
	OutcomeArrived Outcome = "destination established"
	// OutcomePressed: a program was lowered, compiled and run.
	OutcomePressed Outcome = "pressed"
	// OutcomeRefused: the coordinator would not act. `Because` says which refusal.
	OutcomeRefused Outcome = "refused"
	// OutcomeFailed: Marco was asked and did not complete. The desktop may or may not
	// have been touched — `Result.Status` is the only thing that knows, and a compile
	// failure means it was not.
	OutcomeFailed Outcome = "failed"
	// OutcomeStopped: somebody stopped the run, or the context ended.
	OutcomeStopped Outcome = "stopped"
	// OutcomeComplete: there is no step left to attempt.
	OutcomeComplete Outcome = "complete"
)

// Attempt is one observation's worth of work.
type Attempt struct {
	Step    int
	Outcome Outcome
	// Stage is how far the step got against this observation, in `workflowstore`'s own
	// words, or empty when the step was never examined.
	Stage workflowstore.StepStage
	// Because is why, in a sentence somebody could act on.
	Because string
	// Result is what Marco did, set only when a program was actually run.
	Result *marcoexec.Result
	// Arrival is what this observation said about a step's destination, set only when one
	// was asked about.
	Arrival *workflowstore.Arrival
	// Intent is what was authorised, set only when somebody was asked.
	Intent *Intent
	// Real says the executor this ran through reaches a computer.
	//
	// Carried on the attempt rather than only on the runner, because a report read later
	// has to be able to tell a press from a note in a notebook, and
	// [[Experiment-038-how-far-a-reloaded-workflow-gets]] is exactly the kind of record
	// that gets quoted without its preamble.
	Real bool
	// Hovered says the control's own pixels changed while the pointer rested on it, and
	// HoverBecause is how that was measured. Both empty when nobody looked.
	Hovered      bool
	HoverBecause string
	// Prepared says the pointer was moved to the control and rested there
	// before the press. False means the click was sent on its own, which is
	// what a desktop control wants and what a game menu ignores.
	Prepared bool
	// PointerArrived says the pointer was MEASURED where the move put it, and
	// PointerBecause is that measurement. Both empty when nobody could ask —
	// which is not the same as the pointer being there.
	PointerArrived bool
	PointerBecause string
}

// Acted reports whether a program was run through Marco.
func (a Attempt) Acted() bool { return a.Result != nil }

// Runner advances one saved workflow.
//
// One Runner serves one run: it holds the cursor, the step waiting on a destination, and the
// tally. Not safe for concurrent use, and nothing needs it to be — a run is a sequence by
// definition.
type Runner struct {
	art      workflowstore.Artifact
	eyes     Eyes
	grounder Grounder
	granter  Granter

	// exec is the ONE path to an effect. Nil means this runner cannot act, which is what
	// it is built as.
	exec *marcoexec.Executor
	// real says what is at the other end of exec is a computer.
	//
	// Load-bearing for HONESTY rather than for safety. A dry run presses into a notebook,
	// so the application does not move — and reporting "the destination was not
	// established" for that reads as a failed step when in fact nothing was sent. Marco
	// does not get to say what came of an action it did not take.
	real bool
	// confirm is the platform's answer, at the instant of the press, to "is the world
	// still the one this action was planned against".
	//
	// Handed the sighting the plan was made from, so it can compare rather than report.
	// Two questions live in it and both have to be asked HERE rather than read off that
	// sighting:
	//
	//   - does the target lead the desktop NOW. A click carries no address, so it lands
	//     in whatever window is in front at that instant.
	//   - has the observation moved. The look that planned this is seconds old by the
	//     time somebody has read a prompt and answered it, and in those seconds the
	//     screen can change — which makes the point a coordinate on a screen that is no
	//     longer there.
	//
	// The second is not hypothetical and is the reason this takes an argument. A person
	// authorising a press is an unbounded pause in the middle of the one sequence that
	// must not have one, and an authorisation stamped with the observation it was granted
	// in only catches the case where somebody ELSE noticed.
	//
	// Nil means the question cannot be asked, and a runner that cannot ask it REFUSES to
	// act rather than assuming: an unverifiable precondition on an operation that clicks
	// wherever the pointer is, is not a precondition.
	confirm func(ctx context.Context, planned Sighting) (bool, string)

	patience int
	// dwell is how long the pointer rests on the control before the click. Zero sends the
	// click alone, which is what every desktop control wants and what a game menu ignores.
	dwell time.Duration
	// regrounder is asked where a control is when the file's own reading locates
	// nothing, and regrounded is what it has answered, for this session only.
	regrounder Regrounder
	regrounded map[string]observe.Region
	// repairs is what was re-grounded and on whose evidence, in order.
	repairs []Repair
	// pointer answers WHERE THE POINTER ACTUALLY IS, in desktop coordinates, at the
	// instant it is asked.
	//
	// [[Experiment-045-the-pointer-that-would-not-leave]] is why this is a gate and not a
	// diagnostic. A move was lowered into legal Marco, the runtime reported it COMPLETED,
	// and 400ms later the operating system said the cursor was 306px right and 452px down
	// from where it had been sent. The press that followed was aimed at a control the
	// pointer had never been on.
	//
	// A move that reports completed and did not happen is otherwise indistinguishable
	// from a control that did not react, and the run that cannot tell them apart presses
	// anyway. So the question is asked before the button goes down, and a pointer that is
	// somewhere else FAILS the action rather than being noted beside it.
	//
	// Returning false means nobody could ask, and nobody-could-ask is not an answer: the
	// press goes ahead, exactly as a look with no words still permits one. What is
	// refused is a measurement that DISAGREES.
	pointer func(ctx context.Context) (x, y int, ok bool)
	// hover is asked, after the pointer has moved and rested, whether the control's own
	// pixels changed.
	//
	// Separate from `confirm`, which asks whether the WORLD still holds. This asks a much
	// narrower question — did this control react to the pointer being on it — and the
	// answer is the difference between "the click missed" and "the click was ignored".
	// Nil means nobody is looking, which is not the same as nothing happening and is
	// reported as such.
	hover func(ctx context.Context, where observe.Region) (bool, string)
	// actedAt is the observation an action was performed in, and changed says a later
	// observation differed from it.
	//
	// Here because "did anything happen at all" is a different question from "did the
	// expected destination arrive", and a run that can only answer the second reports a
	// game that ignored the click and a game that responded in a way nobody recognised
	// as the same thing.
	actedAt Sighting
	changed bool
	// budget is how many actions this run may ever send, and sent is how many it has.
	//
	// Enforced HERE rather than by the caller, because a caller that counted presses
	// would be counting what it was told afterwards. The coordinator is the only thing
	// that knows an action is about to be sent, and the last moment a limit can be a
	// limit is before the one press that exceeds it.
	//
	// A spent budget is a refusal and not a completion: the run stops safely and says how
	// far it got, because stopping after one deliberate action is the ordinary shape of a
	// supervised test and must not read as a workflow that finished.
	//
	// Negative means unbounded, which is the default and is what every offline caller
	// wants. See WithBudget.
	budget int
	sent   int
	// cursor is the step being attempted.
	cursor int
	// acted is the step a program was run for and whose destination is not yet
	// established, or -1.
	acted int
	// looks is how many observations the current cursor has consumed.
	looks int

	// now is the clock, so a test can watch a lapsed authorisation be refused without
	// sleeping. Nil is time.Now.
	now func() time.Time

	stopped atomic.Bool
	// attempts is every Attempt, in order.
	attempts []Attempt
	// stage records the refusal the walk last saw, for Progress.
	lastStage   workflowstore.StepStage
	lastBecause string
}

// New builds a runner that CANNOT act.
//
// Deliberately: being able to press something is a decision somebody makes, and it is made by
// calling WithExecutor. Everything this is constructed with — an artifact, eyes, a grounder, a
// granter — is incapable of emitting.
func New(art workflowstore.Artifact, eyes Eyes, g Grounder, a Granter) *Runner {
	return &Runner{
		art: art, eyes: eyes, grounder: g, granter: a,
		patience: DefaultPatience, acted: -1, budget: -1, dwell: DefaultDwell,
	}
}

// WithExecutor makes this runner capable of acting. The composition root's decision.
//
// `real` says whether what is at the other end is a computer. It changes nothing about what is
// sent — the same program goes through the same interface either way — and everything about what
// may be concluded afterwards.
func (r *Runner) WithExecutor(e *marcoexec.Executor, real bool) *Runner {
	r.exec, r.real = e, real
	return r
}

// WithConfirm installs the platform's re-observation of the world at the instant of the press.
//
// Supplied by the composition root because only the platform can answer it. Deleting the call
// must fail TestARunnerThatCannotRecheckTheWorldRefusesToAct, and passing nil must fail it too —
// the two are different mistakes and a source-level check can only see the first.
func (r *Runner) WithConfirm(f func(ctx context.Context, planned Sighting) (bool, string)) *Runner {
	r.confirm = f
	return r
}

// WithGrounder replaces who establishes a screen. For a caller that changes its mind about
// whether a person is answering, and for tests that need an observation nobody grounded.
func (r *Runner) WithGrounder(g Grounder) *Runner {
	r.grounder = g
	return r
}

// WithClock replaces the clock an authorisation is checked against. For tests.
func (r *Runner) WithClock(f func() time.Time) *Runner {
	r.now = f
	return r
}

// WithPatience sets how many observations one step is given before the run stops safely.
func (r *Runner) WithPatience(n int) *Runner {
	if n > 0 {
		r.patience = n
	}
	return r
}

// WithDwell sets how long the pointer rests on a control before the click. Zero sends the click
// on its own, with no separate move.
func (r *Runner) WithDwell(d time.Duration) *Runner {
	if d >= 0 {
		r.dwell = d
	}
	return r
}

// Dwell is how long the pointer rests before the click.
func (r *Runner) Dwell() time.Duration { return r.dwell }

// Changed reports whether any observation after an action differed from the one it was
// performed in. It is evidence that something happened, and not evidence of what.
func (r *Runner) Changed() bool { return r.changed }

// Repair is a control re-grounded for this session, and the evidence for it.
//
// Reported rather than applied. The artifact is not touched: a workflow corrected in place by the
// run that was using it would make the next run's evidence depend on this one's guess, and the
// provenance would be gone by the time anybody looked.
type Repair struct {
	Step    int
	Control string
	Where   observe.Region
	// Because is the evidence, in the words of whoever established it.
	Because string
}

// WithRegrounder installs the question "where is this control, when the file cannot say".
func (r *Runner) WithRegrounder(g Regrounder) *Runner {
	r.regrounder = g
	return r
}

// Repairs is every control re-grounded in this session.
func (r *Runner) Repairs() []Repair { return r.repairs }

// WithPointer installs the question "where is the pointer, actually".
//
// Asked after the move and the rest, before the press, and unlike the hover probe it IS a gate.
// The difference is what each one measures: a control that does not light up may still take the
// click, and Marco does not get to decide what an interface looks like — but a pointer that is
// not where Marco put it means the move did not happen, whatever the runtime reported, and a
// press aimed through it lands somewhere nobody chose.
//
// Not installed means the question cannot be asked, and the press goes ahead. See `pointer`.
func (r *Runner) WithPointer(f func(ctx context.Context) (x, y int, ok bool)) *Runner {
	r.pointer = f
	return r
}

// WithHover installs the question "did this control react to the pointer".
//
// Asked after the move and the rest, before the press. It is a diagnosis and never a gate: a
// control that does not light up may still take the click, and refusing on that would be Marco
// deciding it knows what an interface looks like.
func (r *Runner) WithHover(f func(ctx context.Context, where observe.Region) (bool, string)) *Runner {
	r.hover = f
	return r
}

// WithBudget sets how many actions this run may send in total. Negative is unbounded.
//
// The first supervised live test of anything that can click wants exactly one press, and
// "exactly one" has to be a property of the thing doing the pressing. A caller that stopped the
// run on seeing a press reported would be acting on a report of something that already happened.
func (r *Runner) WithBudget(n int) *Runner {
	r.budget = n
	return r
}

// Budget is how many actions this run may send, and Sent how many it has.
func (r *Runner) Budget() int { return r.budget }

// Sent reports how many actions have been lowered, compiled and run.
func (r *Runner) Sent() int { return r.sent }

// Stop ends the run.
//
// Safe to call from another goroutine, and checked again immediately before the press — so
// stopping during the observation that planned an action prevents THAT action, not the one after
// it. A stop that only took effect at the next step would be the one thing a person pressing
// stop is trying to prevent.
func (r *Runner) Stop() { r.stopped.Store(true) }

// Stopped reports whether somebody stopped this run.
func (r *Runner) Stopped() bool { return r.stopped.Load() }

// Acts reports whether this runner has any way to perform an action at all.
//
// Exists so a composition root can ASSERT it wired the executor it meant to. WithExecutor
// returns the same runner, so it is easy to build one and forget — which silently produces a
// walk that refuses every step and reads exactly like a workflow that could not be grounded.
func (r *Runner) Acts() bool { return r != nil && r.exec != nil }

// Real reports whether what this runner acts through reaches a computer.
func (r *Runner) Real() bool { return r != nil && r.real }

// Cursor is the step being attempted.
func (r *Runner) Cursor() int { return r.cursor }

// Advance does one observation's worth of work.
//
// The whole coordinator. Called once per look by a caller that owns the cadence, and the order
// of what it does is the guarantee:
//
//	stopped?  →  usable?  →  ground  →  expire  →  destination  →  readiness
//	          →  authorise  →  stopped?  →  foreground  →  lower + run
//
// Everything before "lower + run" can refuse, and refusing at any of them means nothing was
// sent.
func (r *Runner) Advance(ctx context.Context, s *workflowstore.SessionBinding,
	at Sighting) Attempt {

	a := Attempt{Step: r.cursor, Real: r.real}

	// ── 0. HAS SOMEBODY STOPPED ──
	if stopped, why := r.halted(ctx); stopped {
		a.Outcome, a.Because = OutcomeStopped, why
		return r.record(a)
	}
	if r.cursor >= len(r.art.Steps) {
		a.Outcome = OutcomeComplete
		a.Because = fmt.Sprintf("every one of the %d step(s) was reached",
			len(r.art.Steps))
		return r.record(a)
	}

	// ── 1. IS THERE ANYTHING TO CONCLUDE FROM THIS LOOK ──
	//
	// Before establishment, not after. A look nobody could take must not ground a screen,
	// because a binding made from no pixels would then be live for an observation in which
	// nothing was seen — and every refusal downstream reads off that binding.
	if ok, why := at.Usable(); !ok {
		a.Outcome, a.Because = OutcomeUnobserved, why
		return r.record(a)
	}

	// ── 2. SOMEBODY ELSE GROUNDS THE SCREEN ──
	if r.grounder != nil {
		if err := r.grounder.Establish(ctx, s, at); err != nil {
			a.Outcome = OutcomeWaiting
			a.Because = "the screen could not be established: " + err.Error()
			r.looks++
			if r.looks > r.patience {
				return r.record(r.impatient(a, workflowstore.StepScreenUnestablished))
			}
			return r.record(a)
		}
	}
	// AND WHATEVER THIS OBSERVATION HAS OUTRUN IS DROPPED. Not aged, not carried
	// forward on the grounds that the screen looks the same.
	s.Observed(at.Epoch, at.Generation)

	now := workflowstore.Now{
		Screen: at.Screen, Epoch: at.Epoch, Generation: at.Generation,
		Outcomes:   at.Outcomes,
		Candidates: at.Candidates, Words: at.Words,
	}

	// ── 3. WHAT DOES THIS OBSERVATION SAY ABOUT THE CURRENT STEP'S DESTINATION ──
	//
	// Asked FIRST, and for the step the walk is on rather than for a remembered one. It is
	// the only thing that advances the cursor, which makes one sentence true of both kinds
	// of step this subsystem has: a press moves on by arriving, and so does a wait.
	//
	// It establishes nothing. The grounder above has already run the establishment path
	// over this observation and this reads the result — see `workflowstore.DestinationAt`.
	arrival := r.art.DestinationAt(r.cursor, s, now)
	if arrival.Established() {
		a.Arrival = &arrival
		// A DESTINATION THAT ARRIVED IS A SCREEN THAT CHANGED.
		//
		// Recorded here as well as in the waiting branch below, because an arrival
		// on the very next observation never reaches that branch — and the first
		// successful live navigation reported "the application did not visibly
		// respond" directly above the line saying its destination was established.
		if r.acted == r.cursor && (at.Screen != r.actedAt.Screen ||
			at.Epoch != r.actedAt.Epoch) {
			r.changed = true
		}
		a.Outcome, a.Because = OutcomeArrived, arrival.Because
		r.advance()
		return r.record(a)
	}

	// ── 4. A STEP ALREADY ACTED ON IS NEVER ACTED ON AGAIN ──
	//
	// Only waited for. The press went out; whether it worked is the destination's question
	// and asking it again is free, while pressing again is not — a second click on a menu
	// that did respond is a click on whatever replaced the control.
	if r.acted == r.cursor {
		a.Arrival = &arrival
		// DID ANYTHING HAPPEN AT ALL. A separate question from whether the expected
		// destination arrived, and the one that separates an interface which ignored
		// the press from one that responded in a way nobody recognised.
		if at.Screen != r.actedAt.Screen || at.Epoch != r.actedAt.Epoch {
			r.changed = true
		}
		r.looks++
		if r.looks > r.patience {
			a.Outcome = OutcomeRefused
			a.Stage = workflowstore.StepScreenUnestablished
			a.Because = fmt.Sprintf("the action for step %d was performed and its "+
				"destination %s was not established within %d observation(s): "+
				"%s. The press was sent and nothing has grounded the screen it "+
				"was supposed to produce, which is neither a completed step nor "+
				"a failed one", r.cursor, arrival.Destination, r.patience,
				arrival.Because)
			r.lastStage, r.lastBecause = a.Stage, a.Because
			return r.record(a)
		}
		a.Outcome = OutcomeWaiting
		a.Because = fmt.Sprintf("step %d was acted on and %s is not established yet: %s",
			r.cursor, arrival.Destination, arrival.Because)
		return r.record(a)
	}

	// ── 5. HOW FAR DOES THE STEP GET AGAINST THIS OBSERVATION ──
	//
	// `Authorised` is false here, deliberately and always. Asking first with no permission
	// is how the coordinator discovers that permission is the ONLY thing missing, which is
	// the one moment a person can usefully be asked — everything else about the step is
	// settled and can be described to them.
	now.Reground = r.regrounded
	ready := r.art.StepReadinessAt(r.cursor, s, now)

	// ── 5a. THE FILE CANNOT SAY WHERE THIS CONTROL IS ──
	//
	// Asked once per control and only here, where the refusal actually happens. A step
	// refused for a reason that is not about where its control is stays refused: nobody
	// looks for a different rectangle because a person has not yet said which control was
	// meant, or because this run may not send another action.
	if r.regrounder != nil && r.wantsReground(ready.Stage) {
		if _, done := r.regrounded[r.art.Steps[r.cursor].Control]; !done {
			control := r.art.Steps[r.cursor].Control
			if where, why, ok := r.regrounder.Reground(ctx, r.cursor, control,
				ready.Stage, at); ok {
				if r.regrounded == nil {
					r.regrounded = map[string]observe.Region{}
				}
				r.regrounded[control] = where
				r.repairs = append(r.repairs, Repair{
					Step: r.cursor, Control: control,
					Where: where, Because: why,
				})
				now.Reground = r.regrounded
				ready = r.art.StepReadinessAt(r.cursor, s, now)
			}
		}
	}
	a.Stage, a.Because = ready.Stage, ready.Because

	// ── 6. SOMEBODY IS ASKED, FOR THIS STEP, IN THIS OBSERVATION ──
	//
	// Unless this run has already sent everything it may. Asking first and refusing after
	// would put a question to a person whose answer could not be used, which is how a
	// person learns that answering does not matter.
	if ready.Stage == workflowstore.StepUnauthorised {
		if over, why := r.overBudget(); over {
			a.Outcome = OutcomeRefused
			a.Because = fmt.Sprintf("%s, so nobody was asked about step %d",
				why, r.cursor)
			r.lastStage, r.lastBecause = ready.Stage, a.Because
			return r.record(a)
		}
	}
	if ready.Stage == workflowstore.StepUnauthorised {
		in := r.intentOf(ready, at)
		a.Intent = &in
		auth, granted := r.ask(ctx, in)
		if !granted {
			a.Outcome = OutcomeRefused
			a.Because = fmt.Sprintf("nobody authorised step %d. %s", r.cursor,
				ready.Because)
			r.lastStage, r.lastBecause = ready.Stage, a.Because
			return r.record(a)
		}
		if ok, why := auth.Spendable(r.cursor, at, r.clock()); !ok {
			a.Outcome = OutcomeRefused
			a.Because = "the authorisation cannot be spent here: " + why
			r.lastStage, r.lastBecause = ready.Stage, a.Because
			return r.record(a)
		}
		// THE SAME OBSERVATION, re-examined with the permission in hand. Not a second
		// look: a fresh one would be a different screen and the plan would be about
		// the old one.
		now.Authorised = true
		ready = r.art.StepReadinessAt(r.cursor, s, now)
		a.Stage, a.Because = ready.Stage, ready.Because
	}

	// ── 7. A WAIT ADVANCES BY ARRIVING, WHICH STEP 3 ALREADY ASKED ──
	if ready.Stage == workflowstore.StepNotRepeatable {
		r.looks++
		if r.looks > r.patience {
			return r.record(r.impatient(a, ready.Stage))
		}
		a.Outcome = OutcomeWaiting
		a.Because = fmt.Sprintf("%s. %s is not established yet: %s", ready.Because,
			arrival.Destination, arrival.Because)
		return r.record(a)
	}

	// ── 8. ANY OTHER REFUSAL IS RETRIED, THEN THE RUN STOPS SAFELY ──
	//
	// Retried because a control mid-animation is the ordinary case and this package may not
	// turn a transient miss into an abandoned workflow. Bounded because a refusal that
	// never resolves is the commonest one there is.
	if !ready.Ready() {
		r.looks++
		if r.looks > r.patience {
			return r.record(r.impatient(a, ready.Stage))
		}
		a.Outcome = OutcomeWaiting
		return r.record(a)
	}

	// ── 9. PLANNABLE IS NOT PERMISSION TO SEND, AND THESE ARE WHY ──
	return r.record(r.press(ctx, a, ready, at))
}

// press performs a plannable step, after the checks that can only be made at the last moment.
func (r *Runner) press(ctx context.Context, a Attempt, ready workflowstore.StepReadiness,
	at Sighting) Attempt {

	// THE BUDGET, ASKED FIRST.
	//
	// Before the executor, before stop, before the world: a run that may not send another
	// action has nothing to check, and asking the cheapest unconditional question first
	// means a spent budget cannot be reported as some other refusal that happened to come
	// up on the way.
	// UNREACHABLE WHILE THE CHECK BEFORE THE QUESTION STANDS, and kept anyway.
	//
	// Every path to here passes through the authorisation refusal in Advance, which asks
	// the same question first so that nobody is asked for permission that could not be
	// used. A mutation run confirms it: disabling this one changes no test, because the
	// earlier one catches every case.
	//
	// It stays because this is where `sent` is incremented, and a limit whose only check
	// sits in a different function is a limit the next reordering of that function
	// removes without anything failing. Written down rather than deleted so that the day
	// the order changes, this is the line that was already in the right place — the same
	// reason Report.Verified prints its unreachable branch instead of assuming it.
	if over, why := r.overBudget(); over {
		a.Outcome = OutcomeRefused
		a.Because = fmt.Sprintf("%s; step %d was plannable and nothing was sent",
			why, r.cursor)
		r.lastStage, r.lastBecause = ready.Stage, a.Because
		return a
	}

	if !r.Acts() {
		a.Outcome = OutcomeRefused
		a.Because = "this runner has no executor, so it has no way to perform an " +
			"action at all; the step was plannable and nothing was sent"
		r.lastStage, r.lastBecause = ready.Stage, a.Because
		return a
	}

	// STOPPED, ASKED AGAIN. The look that planned this took time; a person who pressed
	// stop during it is asking for THIS press not to happen.
	if stopped, why := r.halted(ctx); stopped {
		a.Outcome, a.Because = OutcomeStopped, why
		r.lastStage, r.lastBecause = ready.Stage, why
		return a
	}

	// AND THE WORLD, RE-OBSERVED.
	//
	// `Sighting.Front` was true when the pixels were taken and says nothing about the
	// instant of the press — the user alt-tabs, a notification steals focus, an installer
	// appears, and somebody reading an authorisation prompt takes seconds during which the
	// screen can change. `marcoexec` does not cover this: its guard is for operations that
	// NAME a window, and a click names a coordinate.
	if r.confirm == nil {
		a.Outcome = OutcomeRefused
		a.Because = "nobody installed the question about the world, so whether this " +
			"click would land in the target, on the screen it was planned " +
			"against, cannot be confirmed; an unverifiable precondition on an " +
			"operation that clicks wherever the pointer is, is not a precondition"
		r.lastStage, r.lastBecause = ready.Stage, a.Because
		return a
	}
	if ok, why := r.confirm(ctx, at); !ok {
		a.Outcome = OutcomeRefused
		a.Because = "the world is no longer the one this action was planned against, " +
			"so nothing was sent: " + why
		r.lastStage, r.lastBecause = ready.Stage, a.Because
		return a
	}

	// THE POINT, IN DESKTOP COORDINATES. See Sighting.Bounds.
	p := Desktop(ready.Action.At, at.Bounds)
	point := marcoexec.Point{X: p.X, Y: p.Y}

	// ── THE POINTER ARRIVES BEFORE THE BUTTON GOES DOWN ──
	//
	// Two operations rather than one program containing both, and the gap between them is
	// the reason. A single program would put the move, the wait and the click inside one
	// run of the Marco runtime, where this package can check nothing; splitting them keeps
	// the dwell HERE, where a stop can land in it and the world can be asked again on the
	// other side of it. Both go out under the one authorisation somebody gave for this
	// step: moving the pointer to a control and pressing it are one act, and nobody is
	// asked twice for it.
	if r.dwell > 0 {
		mv := marcoexec.Operation{Kind: marcoexec.KindMove, At: point}
		mres, merr := r.exec.Do(ctx, mv)
		if merr != nil || mres.Status != marcoexec.StatusCompleted {
			a.Result = &mres
			a.Outcome = OutcomeFailed
			a.Because = "the pointer could not be moved to the control, so nothing " +
				"was pressed: " + mres.Summary()
			r.lastStage, r.lastBecause = ready.Stage, a.Because
			return a
		}
		a.Prepared = true

		// THE INTERFACE IS GIVEN TIME TO NOTICE, and a stop lands in it.
		select {
		case <-time.After(r.dwell):
		case <-ctx.Done():
		}
		if stopped, why := r.halted(ctx); stopped {
			a.Outcome = OutcomeStopped
			a.Because = "the pointer was moved to the control and the run was " +
				"stopped before the press: " + why
			r.lastStage, r.lastBecause = ready.Stage, a.Because
			return a
		}

		// DID THE CONTROL NOTICE. Asked here because this is the only moment it can
		// be: the pointer is on it and nothing has been pressed yet.
		if r.hover != nil {
			lit, why := r.hover(ctx, ready.Action.Where)
			a.Hovered, a.HoverBecause = lit, why
		}

		// AND WHETHER THE POINTER IS WHERE IT WAS SENT, which the hover probe
		// cannot answer: unchanged pixels mean "the control ignored the pointer"
		// and "the pointer is somewhere else" equally, and those are different
		// findings with different repairs. See `pointer`.
		if r.pointer != nil {
			if px, py, ok := r.pointer(ctx); ok {
				off := away(px-p.X, py-p.Y)
				a.PointerArrived = off <= pointerSlack
				if a.PointerArrived {
					a.PointerBecause = fmt.Sprintf("the operating "+
						"system puts the pointer at (%d,%d), "+
						"where it was sent", px, py)
				} else {
					a.PointerBecause = fmt.Sprintf("the pointer was "+
						"sent to (%d,%d) and the operating system "+
						"puts it at (%d,%d), %d px away",
						p.X, p.Y, px, py, off)
					a.Outcome = OutcomeFailed
					a.Because = "the move was reported completed and " +
						"did not happen (" + a.PointerBecause +
						"), so nothing was pressed: a click through " +
						"a pointer nobody placed lands somewhere " +
						"nobody chose"
					r.lastStage, r.lastBecause = ready.Stage, a.Because
					return a
				}
			}
		}

		// AND THE WORLD AGAIN, because the dwell is a gap like any other.
		if ok, why := r.confirm(ctx, at); !ok {
			a.Outcome = OutcomeRefused
			a.Because = "the pointer was moved to the control and the world " +
				"changed before the press, so nothing was clicked: " + why
			r.lastStage, r.lastBecause = ready.Stage, a.Because
			return a
		}
	}

	op := marcoexec.Operation{
		Kind:  marcoexec.KindClick,
		Count: 1,
		At:    point,
	}
	res, err := r.exec.Do(ctx, op)
	a.Result = &res
	switch {
	case err == nil && res.Status == marcoexec.StatusCompleted:
		a.Outcome = OutcomePressed
		a.Because = fmt.Sprintf("step %d was lowered into legal Marco, compiled and "+
			"run at (%d,%d). %s is what the workflow says follows; whether it "+
			"does is the next observation's answer", r.cursor, p.X, p.Y,
			ready.Expect)
		// NOT cursor++. The step moves on by ARRIVING, and until it does this
		// records that a press went out so nothing presses again.
		r.acted = r.cursor
		r.looks = 0
		r.sent++
		r.actedAt = at
	case res.Status == marcoexec.StatusCancelled:
		a.Outcome, a.Because = OutcomeStopped, res.Summary()
		r.lastStage, r.lastBecause = ready.Stage, a.Because
	default:
		a.Outcome, a.Because = OutcomeFailed, res.Summary()
		r.lastStage, r.lastBecause = ready.Stage, a.Because
		// A compile failure means the desktop was never touched; a runtime failure
		// means it ran and refused. Both stop the run, and `Result.Status` is the
		// only thing that can tell a reader which happened.
	}
	return a
}

// pointerSlack is how far from the point it was sent to the pointer may be measured and still
// count as having arrived.
//
// Four pixels, and the number is a rounding allowance rather than a tolerance. An injected
// absolute move is normalised to 65535 steps across the whole virtual desktop, so on the widest
// desk this machine has seen — two 1920-wide monitors, 3840 px — one step is under a tenth of a
// pixel. Nothing legitimate lands four pixels out. The defect this exists to catch was 306 px
// right and 452 px down, and a number this small is deliberate: a slack wide enough to absorb a
// real disagreement would turn the gate back into the diagnostic it replaces.
const pointerSlack = 4

// away is the Chebyshev distance from the origin — the longer axis, the same measure
// `oshost.glide` steps along, so a miss is reported in the units the move was made in.
func away(dx, dy int) int {
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dy > dx {
		return dy
	}
	return dx
}

// Desktop turns a point normalised to a window into one on the desktop.
//
// Exported because it is arithmetic a caller showing somebody where a press would go needs too,
// and two statements of it would be two chances to get it wrong.
func Desktop(at observe.PointerAt, in directorapi.Rect) Point {
	return Point{
		X: in.X + int(at.X*float64(in.Width)),
		Y: in.Y + int(at.Y*float64(in.Height)),
	}
}

// intentOf describes what somebody is being asked to authorise.
func (r *Runner) intentOf(ready workflowstore.StepReadiness, at Sighting) Intent {
	in := Intent{
		Step: r.cursor, Expect: ready.Expect,
		Epoch: at.Epoch, Generation: at.Generation, Window: at.Window,
		// ON WHAT EVIDENCE, carried through untouched. Readiness decided it; a
		// coordinator that recomputed the kind of grounding here would be a second
		// account of one fact, and this subsystem's recorded failure mode is two
		// statements of one fact drifting apart.
		Grounding: ready.Grounding, Said: ready.Identity.Said,
	}
	if r.cursor < len(r.art.Steps) {
		in.Control = r.art.Steps[r.cursor].Control
		in.Screen = r.art.Steps[r.cursor].From
	}
	// Found rather than Action, because at the moment somebody is asked the step is NOT
	// plannable — authorisation is the last refusal, so `Action` is still nil. Asking a
	// person to permit a click without telling them where it lands is not an explicit
	// authorisation, and it was this test that found it.
	if ready.Found != nil {
		in.At = Desktop(*ready.Found, at.Bounds)
	}
	// THE REGION IS KNOWN BEFORE THE STEP IS PLANNABLE, like the point is. At the moment
	// somebody is asked, authorisation is the last refusal and Action is still nil — so
	// the outcome is consulted only when there is nothing better.
	//
	// Action.Where WINS where it exists, and that is not a preference. It is the region
	// the point was actually taken from: narrowed by agreement, or replaced outright by a
	// re-grounding. A live run re-grounded control_3 to the FREE PLAY label, pressed the
	// right point, and printed the saved full-window rectangle as its target — because
	// this loop ran afterwards and overwrote the good answer with the matched one.
	for _, o := range at.Outcomes {
		if string(o.Target) == in.Control && o.Actionable() {
			in.Where = o.Where
		}
	}
	if ready.Action != nil {
		in.Because = ready.Action.Because
		in.Where = ready.Action.Where
	}
	return in
}

// overBudget reports whether this run has sent every action it may, and says so.
//
// One statement, asked twice. Before the question, so nobody is asked for permission that could
// not be used; and again before the press, because that is where `sent` is incremented and a
// limit whose check sits anywhere but beside the thing it limits is a limit somebody can route
// around. Deleting either call has its own test.
func (r *Runner) overBudget() (bool, string) {
	if r.budget < 0 || r.sent < r.budget {
		return false, ""
	}
	return true, fmt.Sprintf("this run may send %d action(s) and has sent %d",
		r.budget, r.sent)
}

// ask puts the question to the granter, or refuses when there is nobody to ask.
func (r *Runner) ask(ctx context.Context, in Intent) (Authorisation, bool) {
	if r.granter == nil {
		return Authorisation{}, false
	}
	return r.granter.Authorise(ctx, in)
}

// halted reports whether the run may not continue.
func (r *Runner) halted(ctx context.Context) (bool, string) {
	if r.stopped.Load() {
		return true, "somebody stopped this run"
	}
	if err := ctx.Err(); err != nil {
		return true, "the run was cancelled: " + err.Error()
	}
	return false, ""
}

// wantsReground says whether this refusal is one that looking at the screen could answer, and
// whether now is the moment to look.
//
// # Two refusals, and only one of them is immediate
//
// `StepControlUnlocated` is the file naming nowhere: a band across the window re-acquires
// perfectly on every look and locates nothing. No amount of waiting changes that, so it is asked
// the first time it happens.
//
// `StepControlUnresolved` is the control not being found on this look, and that is the one the
// original rule excluded on purpose — a control mid-animation, or briefly covered by something
// passing over it, is refused exactly this way and resolves itself a look later. Replacing its
// target with whatever happened to be visible during the gap would break a step that works.
//
// # What changed, and why the exclusion survives it
//
// A menu MOVES. Measured live: Rocket League's main menu is bottom-anchored, and signed out it
// loses two entries, so PLAY sits 95px lower than the demonstration recorded it. The recorded
// rectangle still locates something — the arena behind the menu — so the step is refused
// unresolved, forever, and the repair that exists for precisely this was never asked.
//
// The patience window is what separates the two. A control that is covered comes back within it;
// a control that has moved is refused on every look of it. So an unresolved control is asked
// about ONCE, on the last observation before the run would stop anyway — where the alternative is
// not a working target but no target at all, and the exclusion's reasoning no longer applies.
func (r *Runner) wantsReground(stage workflowstore.StepStage) bool {
	switch stage {
	case workflowstore.StepControlUnlocated:
		return true
	case workflowstore.StepControlContradicted:
		// AT ONCE, and this is the one that needs no window at all. The evidence is
		// positive and already in hand: something on THIS look is named as this
		// control is, and the thing that was matched is not it. Waiting could only
		// produce the same answer more slowly, and the repair is being asked to
		// weigh a candidate that has already been seen.
		return true
	case workflowstore.StepControlUnresolved:
		// THE LAST LOOK, counted the way section 8 counts it: `looks` is
		// incremented there and compared after, so the final observation of the
		// window is the one where it has not yet been incremented past patience.
		return r.looks >= r.patience
	default:
		return false
	}
}

// impatient turns a refusal that has run out of observations into the end of the walk.
func (r *Runner) impatient(a Attempt, stage workflowstore.StepStage) Attempt {
	a.Outcome = OutcomeRefused
	a.Stage = stage
	a.Because = fmt.Sprintf("step %d was refused for %d observation(s) and the run "+
		"stopped rather than waiting indefinitely: %s", r.cursor, r.patience, a.Because)
	r.lastStage, r.lastBecause = stage, a.Because
	return a
}

// advance moves to the next step and forgets everything about this one.
func (r *Runner) advance() {
	r.cursor++
	r.acted = -1
	r.looks = 0
}

// record keeps the attempt and returns it.
func (r *Runner) record(a Attempt) Attempt {
	r.attempts = append(r.attempts, a)
	return a
}

// clock is now. A method so a test can see a lapsed authorisation refused without sleeping.
func (r *Runner) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// Report is what the whole run did.
type Report struct {
	Workflow string
	// Progress distinguishes a completed workflow from one that stopped partway, in
	// `workflowstore`'s own words. A run interacts with something that changes after every
	// action, so stopping partway is the ordinary case and reporting it as either success
	// or failure is a lie in a different direction.
	Progress workflowstore.Progress
	Attempts []Attempt
	// Pressed is how many programs were run through Marco; Real says whether they reached
	// a computer.
	Pressed int
	Real    bool
	// Established is how many destinations were grounded in the observation after their
	// step, and Independent how many of those by somebody looking rather than by the
	// workflow's own expectation agreeing with an arrangement.
	//
	// Two numbers because collapsing them would let a workflow corroborate its own steps.
	// See `workflowstore.Arrival`.
	Established int
	Independent int
	// Budget is how many actions the run was allowed to send, negative for unbounded, and
	// Sent how many it did. Reported because "one action" is only a meaningful claim
	// beside the limit it was made under.
	Budget int
	Sent   int
	// Changed says an observation after an action differed from the one it was performed
	// in. Evidence that something happened; never evidence of what.
	Changed bool
	// Looks is how many observations the run was given, Usable how many it could
	// conclude anything from, and Unusable why the rest could not be used.
	//
	// Reported because a run that never saw its application is indistinguishable, in every
	// other number here, from a run that saw it perfectly and chose to do nothing. The
	// first supervised exploration session produced a report whose entire explanation was
	// an empty string, and this is what was missing from it.
	Looks    int
	Usable   int
	Unusable map[string]int
	// Repairs is every control re-grounded for this session, with its evidence. Reported
	// so a correction can be proposed afterwards by somebody who can weigh it; nothing
	// here writes one.
	Repairs []Repair
	// StepsPressed is how many distinct steps had an action performed for them, and
	// Unrecognised how many of those never had their destination established.
	//
	// THE PRODUCT NUMBER: goal reached versus goal recognised. An action that moved the
	// application somewhere Director could not name is a navigation that worked and a
	// recognition that did not, and the two have to be counted apart or the first is
	// filed as a failure of the second.
	//
	// Counted rather than subtracted. `Sent - Established` looks like the same number and
	// is not: a destination can be established with no press behind it -- the screen was
	// already standing when the step was examined -- so the difference can cross zero and
	// print as a negative count of unrecognised screens.
	StepsPressed int
	Unrecognised int
	// Hovered is how many actions saw the control react to the pointer resting on it, and
	// Watched how many were looked at. Two numbers, because nobody looking and nothing
	// happening are different findings.
	Hovered int
	Watched int
	// Dwell is how long the pointer rested on a control before each press.
	Dwell time.Duration
}

// Verified reports whether any ACTION was independently verified, and why not.
//
// Always false, and a method rather than a field for the same reason
// `workflowstore.Arrival.Verified` is: nothing may set it. The reason is taken from the arrival
// contract rather than restated, so there is one sentence about this in the codebase.
func (r Report) Verified() (bool, string) {
	if r.Sent == 0 {
		_, why := workflowstore.Arrival{}.Verified()
		return false, why
	}
	// A RUN THAT DID PERFORM SOMETHING says so, and then makes the same claim.
	//
	// Borrowing the arrival's whole sentence here would have this run report that nothing
	// was executed immediately below a line counting what it executed. The part that is
	// load-bearing is shared; the part that is a fact about this run is this run's.
	return false, fmt.Sprintf("%d action(s) were performed and no post-action evidence "+
		"is collected about any of them; and %s", r.Sent,
		workflowstore.ArrivalIsNotCausation)
}

// Report builds the record of the run so far.
func (r *Runner) Report() (out Report) {
	out = Report{
		Workflow: r.art.Name, Attempts: r.attempts, Real: r.real,
		Budget: r.budget, Sent: r.sent, Changed: r.changed, Dwell: r.dwell,
		Repairs: r.repairs,
		Progress: workflowstore.Progress{
			Steps: len(r.art.Steps), Reached: r.cursor, Stopped: -1,
			Stage: r.lastStage, Because: r.lastBecause, Acted: r.sent,
		},
	}
	if r.cursor < len(r.art.Steps) {
		out.Progress.Stopped = r.cursor
	}
	defer func() {
		// AN EMPTY EXPLANATION IS NOT AN EXPLANATION. A run whose looks were all
		// unusable never reached a refusal, so nothing set one — and the report
		// then said it stopped before the first step, for no stated reason.
		if out.Progress.Because != "" {
			return
		}
		if out.Usable > 0 {
			// NOTHING REFUSED IT. The walk was still going when the caller
			// stopped asking — its own deadline, usually — and a summary that
			// trails off after "at step N:" reads as a reason nobody wrote
			// down rather than as a run that simply ran out of time.
			out.Progress.Because = fmt.Sprintf("the run ended while step %d was "+
				"still being attempted; nothing refused it", r.cursor)
			return
		}
		why, most := "no observation could be used", 0
		for because, n := range out.Unusable {
			if n > most {
				why, most = because, n
			}
		}
		if out.Looks == 0 {
			out.Progress.Because = "the run was never given an observation at all"
			return
		}
		out.Progress.Because = fmt.Sprintf("not one of the %d observation(s) could "+
			"be used, so nothing was ever examined. The commonest reason, %d "+
			"time(s): %s", out.Looks, most, why)
	}()
	out.Unusable = map[string]int{}
	// WHICH STEPS WERE PRESSED, AND WHICH OF THOSE ARRIVED. Per step, because a step
	// presses once and is then asked about over however many later observations it
	// takes, so a run of attempts carries one press and many answers about it.
	pressed, arrived := map[int]bool{}, map[int]bool{}
	for _, a := range r.attempts {
		out.Looks++
		if a.Outcome == OutcomeUnobserved {
			out.Unusable[a.Because]++
		} else {
			out.Usable++
		}
		if a.Acted() && a.Result.Status == marcoexec.StatusCompleted {
			out.Pressed++
			pressed[a.Step] = true
		}
		if a.HoverBecause != "" {
			out.Watched++
			if a.Hovered {
				out.Hovered++
			}
		}
		if a.Arrival != nil && a.Arrival.Established() {
			out.Established++
			arrived[a.Step] = true
			if a.Arrival.Independent() {
				out.Independent++
			}
		}
	}
	for step := range pressed {
		out.StepsPressed++
		if !arrived[step] {
			out.Unrecognised++
		}
	}
	return out
}

// Run drives the walk from the eyes until it finishes, stops or runs out of looks.
//
// A thin loop, for a caller that would rather be driven than drive. A live caller that already
// owns a capture cadence calls Advance from inside it instead, and gets the same guarantees —
// there is nothing in here that Advance does not do.
//
// `looks` bounds the whole run. Zero means `len(steps) * patience`, which is the most a walk
// could legitimately need.
func (r *Runner) Run(ctx context.Context, s *workflowstore.SessionBinding, looks int) (
	Report, error) {

	if r.eyes == nil {
		return r.Report(), fmt.Errorf("workflowrun: this runner has no eyes, so it " +
			"cannot observe anything to act on")
	}
	if looks <= 0 {
		looks = len(r.art.Steps) * r.patience
	}
	for i := 0; i < looks; i++ {
		at, err := r.eyes.Look(ctx)
		if err != nil {
			r.lastBecause = "the next observation could not be taken: " + err.Error()
			return r.Report(), err
		}
		a := r.Advance(ctx, s, at)
		switch a.Outcome {
		case OutcomeComplete, OutcomeRefused, OutcomeFailed, OutcomeStopped:
			return r.Report(), nil
		}
	}
	r.lastStage = workflowstore.StepScreenUnestablished
	r.lastBecause = fmt.Sprintf("the run used its %d observation(s) without reaching "+
		"step %d", looks, r.cursor)
	return r.Report(), nil
}
