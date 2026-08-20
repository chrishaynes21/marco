package teach

import (
	"context"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The rest of the learning lifecycle, as the coordinator sees it.
//
// # Why this is an interface of plain values
//
// Because everything behind it can act. Rehearsal spends a grant and drives real input; lowering
// compiles Marco; saving writes a file. The coordinator must be able to FOLLOW that lifecycle
// without being able to reach any of it — the boundary test proves it cannot import `marcoexec`,
// `rehearse`, `execute` or a platform package, and this is how it stays true while Teach still
// walks the whole path.
//
// Every method here is a question about state somebody else owns. None of them decides anything:
// `Granted` reports whether the USER authorised a rehearsal, `Rehearse` runs the state machine
// that already exists, `Lowering` recomputes the judgement that is recomputed everywhere else,
// and `Save` calls the one persistence path. The coordinator's whole contribution is choosing
// which sentence a person hears and in what order.

// Tail is the lifecycle beyond the demonstration.
type Tail interface {
	// Question returns the open proposal for this route, when Marco has put one.
	//
	// Never CREATES one. Proposals are raised by the machinery that judged the evidence, and
	// a coordinator that could raise its own would be a second question system.
	Question(route observe.RelationshipRef, kind observe.AskKind) (Question, bool)
	// Granted reports whether an explicit answer has authorised a rehearsal of this route.
	//
	// The fail-closed reading: silence, a decline, a not-now and a malformed answer all
	// leave this false, because none of them is somebody saying yes.
	Granted(route observe.RelationshipRef) bool
	// Rehearse runs the authorised attempt and reports what happened.
	Rehearse(ctx context.Context) (Attempt, error)
	// Lowering recomputes whether the play may be written down, and what it still needs.
	Lowering(route observe.RelationshipRef) (Readiness, error)
	// Save writes the play under the chosen names, through the one persistence path.
	Save(route observe.RelationshipRef, actor, verb string) (Saved, error)
}

// Question is one open proposal, addressed so a person can answer it.
//
// The ids travel because the user answers through the commands that already accept answers.
// Nothing here is the question's content: the proposal owns its own wording.
type Question struct {
	ID        observe.ProposalID
	SessionID observe.SessionID
	// Screen is the durable subject a naming question is about. Diagnostics only — it is a
	// subject id, and a subject id never reaches a person.
	Screen string
}

// Attempt is what one rehearsal did.
//
// A projection of the ordinary result, not a second record. `Attempted` false with a `Refusal` is
// Marco declining BEFORE input, and it is a different fact from trying and failing — a reader who
// cannot tell them apart cannot audit anything.
type Attempt struct {
	Attempted bool
	Completed bool
	// Terminal is why the attempt ended, from the rehearsal's closed vocabulary.
	Terminal string
	// Refusal is why Marco declined before acting, from the rehearsal's closed vocabulary.
	Refusal string
	// Live says real input was emitted.
	Live bool
	// Steps is what each step of the attempt did: what it emitted, what it expected, and what
	// it actually observed.
	//
	// Watch only. Without it a failed rehearsal reports `stopped_at_step` and nothing else —
	// true, and useless: "it did not go as expected" with no way to see WHAT it saw.
	Steps []AttemptStep
	// Detail is the refusal's own sentence — WHICH window, WHICH provider, what failed.
	//
	// Carried because the closed vocabulary answers "what kind of problem" and a person
	// debugging a live run needs "which one". `source_unobservable` was reported five times
	// in a row with no way to tell a stale window reference from a provider that errored.
	// Watch only; Normal never shows it.
	Detail string
}

// Readiness is the lowering judgement, projected.
type Readiness struct {
	Eligible bool
	// Refusals is the closed lowering vocabulary.
	Refusals []string
	// Unnamed is the durable subject that still needs a name, empty when none does.
	//
	// ONE at a time, because the judgement offers one at a time: naming the source, then
	// recomputing and finding the destination still missing, is the ordinary shape. There is
	// no queue here and there must not be one — a queue would be a second copy of a demand
	// the judgement already makes.
	Unnamed string
	// Source is the generated Marco, present only when eligible. Diagnostics: Normal mode
	// does not print a program at somebody.
	Source string
}

// Saved is what the persistence path did.
type Saved struct {
	// Name is the slug the play was written under.
	Name string
	// Saved says a durable artifact exists. Nothing may claim completion without it.
	Saved bool
	// Registered says a later request can find it. Separate on purpose: saving is "keep
	// this", registering is "and let me ask for it", and they are two permissions.
	Registered bool
	Source     string
}

// AttemptStep is one step of a rehearsal, as the surfaces may see it.
//
// Closed vocabulary and subject ids only — the same rule the rest of this package follows. No
// keys, no labels, no coordinates.
type AttemptStep struct {
	Step     int
	Intents  []string
	Expected string
	Observed string
	Outcome  string
	// Detail is what the host said when this step failed. Diagnostic only, empty on a
	// step that worked. See rehearse.StepRecord.Detail for why one field carries free
	// text where the rest of the record is closed vocabulary.
	Detail string
}

// RehearsalOfferer is a tail that can put ONE required edge of a demonstration to the Audience.
//
// # Why the coordinator may ask for this, when it may not raise a question itself
//
// It does not raise one. It names an edge it is entitled to review — a leg of the demonstration
// the Audience explicitly asked Marco to learn — and the ordinary proposal machinery judges the
// evidence and decides whether there is a question to ask. The coordinator gains no ability to
// invent a question; it gains the ability to say WHICH edge is next.
//
// # Why it exists at all
//
// Teaching gets one exempt rehearsal question, and the terminal leg claims it. Every other leg is
// refused `another_question_open`, so a demonstration of A to B to C could only ever have one of
// its two legs reviewed — and the review lifecycle would then wait forever for a question that
// nobody was going to raise. Observed live on Settings Home to Bluetooth to Mouse.
//
// # What it is not
//
// It is not a wider interruption budget. Passive observation is untouched. The allowance is one
// edge, of this demonstration, in this episode, while that edge is the one under review — and it
// creates room to ASK, never permission to act. Each edge still needs its own explicit yes.
//
// Optional, so every Tail fake keeps compiling and a Director without one simply reviews the leg
// it already has a question for.
type RehearsalOfferer interface {
	// OfferRehearsal asks for this route to be put to the Audience, if the evidence allows.
	//
	// Returns nil whether or not a question resulted: whether the evidence supports asking is
	// the proposal machinery's judgement, not this caller's, and a route it declines to ask
	// about is an ordinary answer rather than a failure.
	OfferRehearsal(route observe.RelationshipRef) error
}

// RehearsalAnswer is a tail that can say whether a rehearsal question has been ANSWERED.
//
// # Why "no question is open" was not enough
//
// A review has to tell two silences apart. One is "nobody has been asked yet, because something
// else holds the interruption slot" — temporary, and the leg is still owed its turn. The other is
// "somebody was asked and the answer was not a yes" — terminal, and the next leg is owed its turn
// instead. Both look identical through `Question`, which reports only what is open right now.
//
// Reading them as the same thing wrote off a real step: live, a screen-naming question held the
// slot while step 1 of 2 was under review, so step 1 was marked unresolved before the Audience had
// been asked anything at all, and the review moved on to step 2.
//
// An answer is a fact the proposal record already holds. This exposes it rather than inferring it
// from an absence.
//
// # And why it reports WHAT was said, not merely that something was
//
// The first version answered a bool, and the review read "answered, and no authority exists yet"
// as "the answer was not a yes". A yes is two facts arriving separately — the proposal gains a
// response, and a grant is created — and observing them in that order is ordinary. Live, somebody
// pressed Yes and read back:
//
//	step 1 of 2 … — unresolved (the answer to this step was not a yes)
//
// about the step they had just said yes to. Absence of authority is not a denial; it is the state
// between the two halves of a yes.
//
// So the answer travels in the proposal's own closed vocabulary and the review decides on what was
// actually said. A `confirmed` never ends a leg here — the grant, the rehearsal, or a grant
// diagnosis does.
//
// Optional, so every Tail fake keeps compiling. Without it a review never concludes that a leg was
// declined by silence — which is the fail-closed direction: a leg stays under review rather than
// being written off unasked.
type RehearsalAnswer interface {
	// AnswerToRehearsal is what the Audience said about this route, and whether the question
	// is SETTLED — put to somebody and no longer open.
	//
	// # Why settled rather than answered
	//
	// A question can close without a response. A retraction sets `Response` back to none,
	// marks the proposal retracted and closes it — so "has it been answered" reports NO for
	// a question that will never be asked again, because the proposal machinery will not
	// re-raise a closed one unless the evidence changes shape.
	//
	// Live, that stalled a review permanently: step 1 sat on "waiting for your answer" with
	// no question anywhere in any session, no grant, and no control the person could press.
	// The response still travels, so a review can say what happened; `ResponseNone` with
	// settled true is "you took that back".
	AnswerToRehearsal(route observe.RelationshipRef) (observe.UserResponse, bool)
}

// RouteSaver is a tail that can write down a whole ordered route rather than one edge.
//
// # Why the walk has to travel
//
// A demonstration of A → B → C is kept as two reusable edges, and each lowers to its own play.
// Saving through `Save` names one edge, so the artifact began in the middle of the behaviour the
// Audience taught and refused its own entry condition when asked from the start.
//
// The episode is the only thing that knows the ordered walk and which of its legs verified, so it
// is the only thing that can ask for the route. Optional, so every Tail fake keeps compiling and a
// Director without one saves the single edge it always did.
type RouteSaver interface {
	// SaveRoute writes the play for this ordered walk, under the chosen names.
	SaveRoute(walk []observe.RelationshipRef, actor, verb string) (Saved, error)
}
