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
}

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
	session observe.SessionID, state observe.ScreenStateID) {

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
		return
	}
	if l.pending != nil {
		l.pending.Close("a newer question replaced it")
		l.note("replaced", l.pending.Request, "a newer question replaced it")
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
	if l == nil || !look.OK {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.pending == nil {
		return
	}
	now := l.clock()
	if ok, why := l.pending.Open(now); !ok {
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
		return
	}
	if !strings.EqualFold(look.Application, l.pending.Application) {
		return
	}
	for _, in := range look.Inputs {
		if in.Event.Intent != observe.NavPoint {
			continue
		}
		a, settled, why := l.pending.Press(in, now)
		switch {
		case settled:
			l.note("settled", l.pending.Request, why)
		case a.Outcome == surface.OutcomeUndiscriminating:
			// THE CASE THE WHOLE RULE EXISTS FOR, and the person is told, so they
			// can aim again rather than wonder why nothing happened.
			l.note("undiscriminating", l.pending.Request, why)
		case a.Outcome == surface.OutcomeElsewhere:
			l.note("elsewhere", l.pending.Request, why)
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
