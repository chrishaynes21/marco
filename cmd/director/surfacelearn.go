package main

// One question, one genuine press, one attributed crossing, one durable fact.
//
// # Where each half comes from, and why neither is new
//
// [[ADR-206-a-demonstrated-surface-is-one-fact-about-one-target]] built the reasoning and left
// two wires hanging: nothing supplied the durable place, and nothing supplied a confirmation that
// the demonstrated press had done anything. The owner's requirement was to connect them to what
// exists rather than to build either:
//
//	Connect an outstanding surface clarification to genuine user input from the existing
//	navsource observation path. … Connect the existing transition correlator to demonstration
//	confirmation. Do not let a caller simply assert ConfirmationAttributed without an actual
//	attributable transition.
//
// Both already run, continuously, in ambient watching:
//
//	THE PRESS       `navsource` admits a hand's press, refuses an injected one, refuses one on
//	                Marco's own surface and refuses one it cannot place in the window. The
//	                session runner banks it stamped with the screen state it was performed ON.
//	                `ambientObserver.drain` is where that log is read, and `offerPresses` below
//	                is one call at that point.
//	THE CROSSING    `ambientObserver` detects a screen change, asks whether a PERSON or Marco
//	                caused it, and attributes the human actions filed against the screen being
//	                left. `offerCrossing` below is one call at that point.
//
// So there is no second input recorder and no second correlator. What is added is a reader of
// both, and a question for them to be about.
//
// # The place, from the resolver that already answers it
//
// Every ambient look carries `observe.Place`, resolved "through the one resolver". Its `Subject`
// is the durable subject id when Marco recognises the screen and empty when it does not. That is
// the Place a question is bound to and the Place a target is stored under — no second screen
// identity, which is the thing `semanticmemory`'s whole design refuses.
//
// A screen Marco does not recognise durably therefore gets its question ASKED and cannot have the
// answer STORED. That is the honest shape: the person is not at fault for Marco not recognising
// the screen, and a target with no place could never be found again.
//
// # The licence
//
// A person asked Marco to click something, Marco asked them where, and they pointed. That is the
// human semantic event the durable writes here hang off — the same shape `promotion` keeps for
// retrospective Learn, and the same sentence: what is licensed is this operation, now, on the
// part of that evidence the person just pointed at. The licence is an object rather than a
// comment for the reason that file gives, and `learnLicence` below is the only place one is made.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/surface"
)

// surfaceLearner holds at most one outstanding surface question and the stores that can answer
// it.
//
// ONE AT A TIME, replaced rather than queued. Two open questions would mean a press has two
// things it might be answering, and nothing could tell which — the same reason a crossing
// attributed to two actions confirms nothing.
type surfaceLearner struct {
	mu      sync.Mutex
	pending *surface.Pending
	// learned is the last outcome, for the report and for `director explain`.
	learned []surfaceLesson
	// targets, surfaces and places are the durable halves. Nil on a Director whose memory
	// could not be read, which asks the question and stores nothing.
	targets  observe.TargetStore
	surfaces observe.SurfaceStore
	recall   func(application string, sig observe.StructureSignature) observe.Recollection
	now      func() time.Time
	// handoff is what became of every press offered here. A measurement only: see
	// surfacehandoff.go for the live run that made its absence the problem.
	handoff pressHandoff
	// awaiting is a clarification that was raised in an observation session which had
	// already ended, and so has not been opened as a question yet.
	awaiting *awaitingQuestion
}

// awaitingQuestion is a clarification raised but not yet established.
//
// # Why a request cannot open its own question
//
// A request makes ambient watching STAND ASIDE: its session is cancelled so that nothing the
// Director does is folded into evidence about what the person did, and the supervisor starts a
// fresh one afterwards. That is deliberate. But the question was stamped from the last ambient
// look, which belongs to the session just cancelled — so every press the person then made
// arrived in the NEXT session and was refused, correctly, by a rule that exists because screen
// state ids are session-local counters.
//
// Measured live: fourteen presses seen, fifty-four offers refused, every one of them saying the
// reading was from observe_2 and the question was asked in observe_1.
//
// So this holds the INTENT, and nothing executable. What it carries is the request, the
// application and the durable Place — the one identity that survives a session, because it is
// the only one that is not a session-local counter. No question, no options, no screen state:
// those have to come from a fresh reading in the session a press can actually arrive in, or the
// stale question would simply be wearing a new label.
type awaitingQuestion struct {
	Request     string
	Application string
	Place       string
	// RaisedIn is the session that had already ended, kept for the account.
	RaisedIn observe.SessionID
	At       time.Time
	// Expires bounds the wait.
	//
	// An intent is waiting for ONE thing: the supervisor to resume and read the
	// screen again, which it does within a glance of standing aside. So this is not
	// a person's deadline and is not generous. It exists because the screen the
	// person asked about can disappear while nothing is watching — they close the
	// window, a game takes the foreground, the observer never comes back — and an
	// intent with no bound would sit there until the next request happened to
	// replace it, then establish a question about a screen that had been gone for
	// an hour.
	Expires time.Time
}

// HowLongAnIntentWaitsToBeEstablished bounds a deferred clarification.
//
// Thirty seconds: two orders of magnitude more than the supervisor needs to resume, and short
// enough that a clarification which outlived its screen is refused rather than honoured.
const HowLongAnIntentWaitsToBeEstablished = 30 * time.Second

// surfaceLesson is what became of one question, for a person reading the account.
type surfaceLesson struct {
	At      time.Time
	Request string
	// Outcome is one of: asked, settled, undiscriminating, elsewhere, learned, unconfirmed,
	// lapsed, replaced, cancelled.
	Outcome string
	Detail  string
}

// MaxSurfaceLessons bounds the account.
const MaxSurfaceLessons = 16

func newSurfaceLearner(mem *semanticStores, now func() time.Time) *surfaceLearner {
	l := &surfaceLearner{now: now}
	if mem != nil {
		l.targets, l.surfaces, l.recall = mem.targets, mem.surfaces, mem.recall
	}
	return l
}

// semanticStores is the narrow view of durable memory this needs.
//
// Three read/write capabilities and nothing else: no relationships, no goals, no judgements, no
// candidates. The same narrowing `observe.TargetStore` and `observe.SurfaceStore` make, applied
// to the composition root so that what is handed over is visible in one place.
type semanticStores struct {
	targets  observe.TargetStore
	surfaces observe.SurfaceStore
	recall   func(application string, sig observe.StructureSignature) observe.Recollection
}

// learnLicence is the permission the durable writes below spend.
//
// THE ONLY PLACE ONE IS MADE on this path. A person asked Marco to act, Marco asked them where,
// and they pointed at it — the explicit human semantic event the three permissions hang off.
//
// `NameActivatedTargets` is the one that matters and it is the privacy-carrying one: the durable
// target is identified by the word on the control, which is admitted only for the one control the
// person's own input landed on. Nothing else observed on that screen becomes a target.
func learnLicence() observesession.Licence { return observesession.LearnLicence() }

// ── the question ──────────────────────────────────────────────────────────────

// Asked opens a question about one finished request, replacing whatever was open.
//
// Called from the one door every request already passes through, so there is no second place the
// lifecycle can be forgotten. A request that raised no question CLOSES the outstanding one: the
// person asked for something else, and a press meant for the new instruction must not answer the
// old one.
func (l *surfaceLearner) Asked(out execute.Outcome, application, place string,
	session observe.SessionID, state observe.ScreenStateID, active observe.SessionID) {

	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock()
	q := out.SurfaceQuestion
	if q == nil || !q.Open() {
		// A DIFFERENT REQUEST ENDS THE OLD QUESTION. Not a lapse and not a refusal —
		// the person moved on, and the press they make next belongs to what they just
		// asked for.
		if l.pending != nil && l.pending.Request != out.Request {
			l.pending.Close("the person asked for something else instead")
			l.note("replaced", l.pending.Request, "a later request replaced it")
			l.pending = nil
		}
		// AND A DEFERRED ONE GOES WITH IT. An intent that was never established is
		// still a thing the person asked for, and a later request replaces it for
		// exactly the reason it replaces an open question.
		if l.awaiting != nil && l.awaiting.Request != out.Request {
			l.note("abandoned", l.awaiting.Request,
				"a later request replaced it before it could be "+
					"established")
			l.awaiting = nil
		}
		return
	}
	if l.pending != nil {
		l.pending.Close("a newer question replaced it")
		l.note("replaced", l.pending.Request, "a newer question replaced it")
	}
	l.awaiting = nil
	// THE SESSION A PRESS WILL ARRIVE IN IS NOT THE ONE THIS WOULD BE STAMPED WITH.
	//
	// The request that raised this cancelled the observation session it was observed in,
	// so the stamp available here is of a session that has ended. A question carrying it
	// can never be answered: every press the person makes afterwards belongs to the
	// session that replaced it. So the intent is held and the question is not opened.
	// See awaitingQuestion, and Establish below for the only thing that opens one.
	if session == "" || session != active {
		l.awaiting = &awaitingQuestion{Request: out.Request, Application: application,
			Place: place, RaisedIn: session, At: now,
			Expires: now.Add(HowLongAnIntentWaitsToBeEstablished)}
		l.note("deferred", out.Request, fmt.Sprintf(
			"the session this would be stamped with (%s) is not "+
				"the one a press will arrive in (%s), so the question "+
				"waits for a fresh reading",
			sessionOrNone(session), sessionOrNone(active)))
		return
	}
	l.pending = surface.NewPending(*q, out.Request, application, place, q.WindowID,
		session, state, now)
	detail := q.Asked
	if place == "" {
		// SAID OUT LOUD, because it changes what answering is worth. The question is
		// still asked; the answer cannot be stored.
		detail += " (this screen has no durable identity yet, so an answer cannot be " +
			"remembered)"
	}
	l.note("asked", out.Request, detail)
}

// sessionOrNone names a session for a person reading the account.
func sessionOrNone(id observe.SessionID) string {
	if id == "" {
		return "none"
	}
	return string(id)
}

// Awaiting is the clarification waiting to be established, nil when none is.
func (l *surfaceLearner) Awaiting() *awaitingQuestion {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.awaiting == nil {
		return nil
	}
	c := *l.awaiting
	return &c
}

// Establish opens the deferred question from a FRESH reading, in the session a press can arrive
// in, and refuses every way the screen might have moved on since the request.
//
// # What it will not do
//
// It will not relabel the stale question. The session is not rewritten and the screen state is
// not copied across: both are session-local counters, so carrying either would disguise a stale
// question as a fresh one — which is the thing the session rule exists to prevent. The question
// handed in here is re-derived by the caller from its own reading of the screen, and everything
// this checks is checked against that reading.
//
// The durable Place is the one identity that may cross, because it is the only one that is not
// session-local. If it has changed, the person is somewhere else and the clarification is void.
//
// Returns whether a question was opened and, either way, the sentence saying why.
func (l *surfaceLearner) Establish(q surface.Question, application, place string,
	session observe.SessionID, state observe.ScreenStateID, active observe.SessionID) (bool, string) {

	if l == nil {
		return false, "there is no learner"
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.awaiting
	if w == nil {
		return false, "no clarification was waiting to be established"
	}
	if !q.Open() {
		l.note("refused", w.Request, "the fresh reading raises no answerable question, "+
			"so there is nothing to establish")
		l.awaiting = nil
		return false, "the fresh reading raises no answerable question"
	}
	// THE READING HAS TO BE THE ONE A PRESS WILL ARRIVE IN. Establishing from a session
	// that has already ended would reproduce the defect this whole path exists to fix.
	if session == "" || session != active {
		return false, fmt.Sprintf("the reading offered (%s) is not the session a press "+
			"will arrive in (%s)", sessionOrNone(session), sessionOrNone(active))
	}
	if state == "" {
		return false, "the reading placed no screen state, so a press could not be tied " +
			"to the screen the question is about"
	}
	if !strings.EqualFold(application, w.Application) {
		l.note("refused", w.Request, fmt.Sprintf("the clarification was raised about %q "+
			"and the screen in front is %q", w.Application, application))
		l.awaiting = nil
		return false, fmt.Sprintf("the application changed from %q to %q", w.Application,
			application)
	}
	// AND THE PLACE, which is the one identity that survives a session.
	if place != w.Place {
		l.note("refused", w.Request, fmt.Sprintf("the clarification was raised at place "+
			"%q and the screen in front is %q, so the person is somewhere else",
			placeOrNone(w.Place), placeOrNone(place)))
		l.awaiting = nil
		return false, fmt.Sprintf("the place changed from %s to %s", placeOrNone(w.Place),
			placeOrNone(place))
	}
	now := l.clock()
	if l.pending != nil {
		l.pending.Close("a newer question replaced it")
		l.note("replaced", l.pending.Request, "a newer question replaced it")
	}
	l.pending = surface.NewPending(q, w.Request, application, place, q.WindowID,
		session, state, now)
	l.awaiting = nil
	detail := q.Asked
	if place == "" {
		detail += " (this screen has no durable identity yet, so an answer cannot be " +
			"remembered)"
	}
	// THE GEOMETRY OF THE QUESTION THAT ACTUALLY OPENED, in the account.
	//
	// Not the geometry the request produced: that came from a different reading of a
	// screen in a session that has ended, and telling somebody to press there would be
	// the stale question giving directions.
	if where, ok := q.WhereToPress(); ok {
		detail += fmt.Sprintf(" — a press answers it at %v, whose middle is (%d, %d)",
			where, where.X+where.Width/2, where.Y+where.Height/2)
	}
	l.note("established", w.Request, detail)
	return true, fmt.Sprintf("established in %s on %s", session, state)
}

// placeOrNone names a durable place for a person reading the account.
func placeOrNone(p string) string {
	if p == "" {
		return "none"
	}
	return p
}

// Cancelled closes the outstanding question, if any.
func (l *surfaceLearner) Cancelled(why string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.pending == nil {
		return
	}
	l.pending.Close(why)
	l.note("cancelled", l.pending.Request, why)
	l.pending = nil
}

// ── the press ─────────────────────────────────────────────────────────────────

// offerPresses reads one ambient look's input log as a possible answer.
//
// Called from `ambientObserver.drain`, where the admitted input log is already in scope and
// already carries the screen state each event was performed on. Nothing is observed here and no
// event is consumed: the same log goes on to `coalesce` exactly as before, so the demonstration
// path cannot take an event away from the correlator that has to confirm it.
//
// Deleting this call must fail TestAGenuinePressSettlesAnOpenQuestion.
func (l *surfaceLearner) offerPresses(look ambientLook) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	// EVERY GATE BELOW IS COUNTED AND NONE OF THEM MOVES. Four of them used to return in
	// silence, which is how an admitted press came to vanish between the producer and the
	// question with nothing to read afterwards. See surfacehandoff.go.
	l.handoff.Offers++
	if !look.OK {
		l.handoff.LooksRefused++
		return
	}
	// COUNTED BEFORE THE QUESTION IS CHECKED, so that "a press arrived while nothing was
	// pending" is a fact rather than an absence. Reading the log is free of consequence:
	// nothing is consumed here and the same log goes on to the correlator.
	presses := 0
	for _, in := range look.Inputs {
		if in.Event.Intent == observe.NavPoint {
			presses++
		} else {
			l.handoff.NotAPress++
		}
	}
	l.handoff.PressesSeen += presses
	if l.pending == nil {
		l.handoff.NoQuestion++
		if presses > 0 {
			l.handoff.note("%d press(es) were in the log and no question was open",
				presses)
		}
		return
	}
	now := l.clock()
	if ok, why := l.pending.Open(now); !ok {
		l.handoff.Lapsed++
		l.handoff.note("lapsed: %s", why)
		l.note("lapsed", l.pending.Request, why)
		l.pending = nil
		return
	}
	// THE SESSION HAS TO BE THE ONE THE QUESTION WAS ASKED IN.
	//
	// Screen state ids are session-local counters, so `state_2` in two sessions are
	// unrelated screens — the same fact `drain` drops its whole cursor over. A press from
	// another session cannot be matched to this question's screen at all.
	if look.Session != l.pending.Session {
		l.handoff.OtherSession++
		l.handoff.note("the reading is from session %s and the question was asked in "+
			"%s, so no press in it can be matched to the question's screen",
			look.Session, l.pending.Session)
		return
	}
	if !strings.EqualFold(look.Application, l.pending.Application) {
		l.handoff.OtherApplication++
		l.handoff.note("the reading is of %q and the question is about %q",
			look.Application, l.pending.Application)
		return
	}
	for _, in := range look.Inputs {
		if in.Event.Intent != observe.NavPoint {
			continue
		}
		a, settled, why := l.pending.Press(in, now)
		switch {
		case settled:
			l.handoff.Settled++
			l.note("settled", l.pending.Request, why)
		case a.Outcome == surface.OutcomeUndiscriminating:
			// THE CASE THE WHOLE RULE EXISTS FOR, and the person is told, so they
			// can aim again rather than wonder why nothing happened.
			l.handoff.Undiscriminating++
			l.note("undiscriminating", l.pending.Request, why)
		case a.Outcome == surface.OutcomeElsewhere:
			l.handoff.Elsewhere++
			l.note("elsewhere", l.pending.Request, why)
		default:
			// THE SILENT ONE, and the reason this whole record exists.
			//
			// `Pending.Press` returns a zero Answer with a sentence when the press
			// was made on a different screen state than the question is about, and
			// a zero Outcome matches no case above. The press was refused for a
			// real reason and the account said nothing at all.
			//
			// Still no note on the lesson list: this is not an outcome the learner
			// acts on, and adding one would change what `Lessons` means to every
			// reader of it. It is recorded here, where measurements live.
			l.handoff.Unaccounted++
			l.handoff.note("refused with no outcome the learner acts on: %s", why)
		}
	}
}

// ── the crossing ──────────────────────────────────────────────────────────────

// offerCrossing reads one attributed screen change as the confirmation for a settled press.
//
// Called from the one place the ambient observer concludes a crossing, with the step it built —
// so the witness is the correlator's own account and not a second reading of it.
//
// Deleting this call must fail TestAPressIsLearnedOnlyAfterAnAttributedCrossing.
func (l *surfaceLearner) offerCrossing(step ambient.Step) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.pending == nil || l.pending.Settled() == nil {
		return
	}
	// HOW MANY HUMAN ACTIONS THIS CROSSING WAS ATTRIBUTED TO.
	//
	// Counted from the acts the correlator carried across, and only the activations: a
	// crossing that also carried a Back or a Menu is not one action, and
	// `observe.TransitionAttributed` refuses anything but exactly one.
	acts := 0
	for _, a := range step.Did {
		if a.Kind == ambient.Activate {
			acts++
		}
	}
	w := observe.TransitionWitness{
		Application: step.Application, From: step.From, To: step.To,
		ByHuman: step.By == ambient.ByHuman, Acts: acts, At: step.At,
	}
	l.learn(w)
}

// learn writes the durable fact, or says why it could not.
//
// Called with the lock held.
func (l *surfaceLearner) learn(w observe.TransitionWitness) {
	p := l.pending
	request := p.Request
	if l.targets == nil || l.surfaces == nil || l.recall == nil {
		l.note("unconfirmed", request, "this Director cannot remember things, so the "+
			"demonstration was seen and not kept")
		p.Close("there is nowhere to remember it")
		l.pending = nil
		return
	}
	// WHO THE TARGET IS, DURABLY — through the one signature and the one store.
	//
	// `RememberTarget` is idempotent by signature and refuses a target with no place, so a
	// question asked on a screen Marco does not recognise fails here with its own sentence
	// rather than being stored against nothing.
	sig := observe.TargetSignature(p.Place, p.Question.Identity.Label,
		observe.TargetKindOf(string(p.Question.Identity.Role)))
	if !learnLicence().NameActivatedTargets {
		l.note("unconfirmed", request, "this operation was not licensed to name the "+
			"control the person activated")
		p.Close("not licensed")
		l.pending = nil
		return
	}
	target, err := l.targets.RememberTarget(p.Application, sig, observe.FromAccessible)
	if err != nil {
		l.note("unconfirmed", request, "the target could not be remembered: "+err.Error())
		p.Close("the target could not be remembered")
		l.pending = nil
		return
	}
	// AND THE SURFACE, confirmed by the crossing and by nothing a caller said.
	d, err := p.Learned(target, "the person at the keyboard", w)
	if err != nil {
		// NOT A FAILURE OF THE DEMONSTRATION. The press was real and was seen; what is
		// missing is evidence that it did anything. Kept as that, so a person is told
		// what Marco saw — and `Usable` refuses it wherever it is read.
		l.keepUnconfirmed(target, request, err)
		return
	}
	if err := l.surfaces.RememberSurface(p.Application, d); err != nil {
		l.note("unconfirmed", request, "the surface could not be remembered: "+
			err.Error())
		p.Close("the surface could not be remembered")
		l.pending = nil
		return
	}
	l.note("learned", request, fmt.Sprintf("%s at depth %d for %q, confirmed because %s",
		d.Relation, d.Depth, p.Question.Identity.Label, d.Confirmed.Why()))
	p.Close("the demonstration was learned")
	l.pending = nil
}

// keepUnconfirmed stores what the press established without the confirmation it lacks.
//
// Called with the lock held. The record is refused by `Usable`, so it aims nothing; it exists
// because a demonstration nobody could confirm is a real thing that happened and discarding it
// silently would make the person show Marco again for nothing.
func (l *surfaceLearner) keepUnconfirmed(target, request string, why error) {
	p := l.pending
	d, derr := p.Unconfirmed(target, "the person at the keyboard", l.clock())
	if derr == nil && l.surfaces != nil {
		derr = l.surfaces.RememberSurface(p.Application, d)
	}
	detail := why.Error()
	if derr != nil {
		detail += "; and it could not be kept as an unconfirmed demonstration: " +
			derr.Error()
	} else {
		detail += "; kept as an unconfirmed demonstration, which cannot aim anything"
	}
	l.note("unconfirmed", request, detail)
	p.Close("nothing confirmed that the press did anything")
	l.pending = nil
}

// ── the account ───────────────────────────────────────────────────────────────

func (l *surfaceLearner) note(outcome, request, detail string) {
	l.learned = append(l.learned, surfaceLesson{
		At: l.clock(), Request: request, Outcome: outcome, Detail: detail,
	})
	if len(l.learned) > MaxSurfaceLessons {
		l.learned = l.learned[len(l.learned)-MaxSurfaceLessons:]
	}
}

// Handoff is what became of every press offered to a question, copied under the lock.
//
// COPIED, because a reader prints this while ambient watching keeps offering looks — the same
// reason Lessons below copies.
func (l *surfaceLearner) Handoff() pressHandoff {
	if l == nil {
		return pressHandoff{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	h := l.handoff
	h.Notes = append([]string(nil), l.handoff.Notes...)
	return h
}

// Lessons is what became of the recent questions, newest last.
func (l *surfaceLearner) Lessons() []surfaceLesson {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]surfaceLesson{}, l.learned...)
}

// Open reports the outstanding question, for the report. Nil when none is.
func (l *surfaceLearner) Open() *surface.Pending {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.pending
}

func (l *surfaceLearner) clock() time.Time {
	if l.now != nil {
		return l.now()
	}
	return time.Now()
}

// activeObservationID is which observation session is running now, empty when none is.
//
// One accessor, because "which session will a press arrive in" is asked in three places and a
// second reading of the registry is a second place the answer could be wrong.
func (r *Runtime) activeObservationID() observe.SessionID {
	if r == nil || r.observations == nil {
		return ""
	}
	return r.observations.ActiveID()
}

// readyToEstablish is the cheap question asked of every reading: could this one establish the
// clarification that is waiting?
//
// CHEAP ON PURPOSE, because it is asked on the thread that drains the input log. A few field
// comparisons under the lock the learner already holds, and nothing that reads the screen: the
// observation an establishment needs is taken elsewhere, once, off this thread.
//
// It also expires an intent that waited too long, because this is the one place a reading and
// the clock meet. A mismatch of place or application is NOT a refusal here — the person may
// simply be somewhere else for a moment — so it reports not-yet and lets the expiry decide. The
// refusals that void a clarification outright live in Establish, where they are recorded.
func (l *surfaceLearner) readyToEstablish(look ambientLook, active observe.SessionID) (
	awaitingQuestion, bool) {

	if l == nil {
		return awaitingQuestion{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.awaiting == nil {
		return awaitingQuestion{}, false
	}
	now := l.clock()
	if !now.Before(l.awaiting.Expires) {
		l.note("expired", l.awaiting.Request, fmt.Sprintf("no reading in a session a press could arrive in appeared within %s, so the clarification is no longer about a screen anybody is looking at", HowLongAnIntentWaitsToBeEstablished))
		l.awaiting = nil
		return awaitingQuestion{}, false
	}
	// THE READING HAS TO BE ONE A PRESS COULD ARRIVE IN, which is the whole point.
	if !look.OK || look.Session == "" || look.Session != active || look.State == "" {
		return awaitingQuestion{}, false
	}
	// AND IT HAS TO BE A SESSION THAT REPLACED THE ONE THE REQUEST ENDED. The same
	// session would mean watching has not resumed yet and this is the dead reading again.
	if look.Session == l.awaiting.RaisedIn {
		return awaitingQuestion{}, false
	}
	if !strings.EqualFold(look.Application, l.awaiting.Application) {
		return awaitingQuestion{}, false
	}
	if look.Place.Subject != l.awaiting.Place {
		return awaitingQuestion{}, false
	}
	return *l.awaiting, true
}

// CouldNotObserve records an establishment attempt that could not read the screen.
//
// The intent is LEFT WAITING. A failed observation is not evidence that the screen changed, and
// the next reading may well succeed; what stops this repeating forever is the expiry above
// rather than a refusal invented here. See the standing rule that missing evidence is not
// contradictory evidence.
func (l *surfaceLearner) CouldNotObserve(request, why string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.awaiting == nil {
		return
	}
	l.note("unavailable", request, why)
}

// surfaceQuestionRaised is the one door a finished request passes through on its way to the
// learner.
//
// Extracted from Handle so the registry path can be driven by a test. A mutation run over
// [[ADR-213-a-question-belongs-to-the-session-that-can-answer-it]] left exactly one survivor:
// handing the look's own session over as the active one made the learner never defer, with the
// whole suite still green, because nothing could enter Handle with a live observation registry.
// This can be entered.
func (r *Runtime) surfaceQuestionRaised(out execute.Outcome) {
	if r == nil {
		return
	}
	place, session, state, _ := r.ambient().standing()
	// WHICH SESSION A PRESS WILL ACTUALLY ARRIVE IN, beside the one that look came from.
	//
	// They differ, always, for a request that raised a question: the request made ambient
	// watching stand aside, so the look belongs to a session that has just been cancelled
	// and the supervisor is about to start a fresh one. The learner holds the intent in
	// that case rather than opening a question nothing could answer.
	r.surfaceLearner().Asked(out, r.activeApplication(), place, session, state,
		r.activeObservationID())
}

// considerEstablishing hands a waiting clarification to one background establishment.
//
// # Where this is called from, and what it must not do there
//
// From the drain, which is the one place a reading and the admitted input log are both in hand.
// So it runs on the supervisor thread, and the only thing it may do there is compare a few
// fields: a Stage collection on this thread would stall ambient watching, which is the thread
// that has to notice what the person did. The observation therefore happens on its own
// goroutine, one at a time.
//
// SINGLE FLIGHT, not a loop. Readings arrive about ten times a second and an observation takes
// a hundred times longer than that, so without the flag every reading would start another one
// and they would queue behind the accessibility bridge until the intent expired. There is no
// polling here and no second observation loop: the trigger is a reading the supervisor was
// taking anyway.
func (r *Runtime) considerEstablishing(look ambientLook) {
	if r == nil {
		return
	}
	intent, ok := r.surfaceLearner().readyToEstablish(look, r.activeObservationID())
	if !ok {
		return
	}
	if !r.establishing.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer r.establishing.Store(false)
		r.establishDeferredQuestion(intent)
	}()
}

// establishDeferredQuestion takes one fresh reading and establishes the waiting clarification
// from it, or records why it could not.
//
// Through `pipeline.Observe`, which is the production observation and the same one the request
// itself used — not a second path with its own idea of what the screen looks like. The question
// is re-derived from that reading by the same surface evaluation the pipeline runs, and every
// check lives in Establish: a reading that is not the running session, a place that moved, an
// application that changed, an ambiguity that resolved itself. Nothing is carried across but the
// instruction and the durable Place.
//
// A reading that raises no question is handed to Establish as an empty one on purpose, so that
// the refusal is recorded in the one place refusals are recorded.
func (r *Runtime) establishDeferredQuestion(intent awaitingQuestion) {
	l := r.surfaceLearner()
	if r.pipeline == nil || r.pipeline.Observe == nil {
		l.CouldNotObserve(intent.Request, "there is no observation pipeline")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), deferredObservationTimeout)
	defer cancel()
	w, err := r.pipeline.Observe(ctx)
	if err != nil {
		l.CouldNotObserve(intent.Request, "the screen could not be read again: "+err.Error())
		return
	}
	q, _ := surfaceHypothesisFor(w, intent.Request).Question()
	// THE PLACE, THE SESSION AND THE STATE FROM THE READING, not from the intent.
	place, session, state, app := r.ambient().standing()
	l.Establish(q, app, place, session, state, r.activeObservationID())
}

// deferredObservationTimeout bounds the one observation an establishment takes.
//
// Twenty seconds, which is the ordinary request timeout order rather than a tight one: this
// observation competes for the accessibility bridge with the supervisor that triggered it, so a
// bound that assumed an idle machine would turn a busy one into a failed establishment.
const deferredObservationTimeout = 20 * time.Second
