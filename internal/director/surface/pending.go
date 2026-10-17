package surface

// One outstanding question, and every way it stops being about what it was about.
//
// # Why a lifecycle and not a flag
//
// Because a question left open is permission for the next press anybody makes to be read as an
// answer, and the owner named what that has to survive:
//
//	Bind each clarification to one request and one place. Handle cancellation, timeout, stale
//	observations, foreground changes, competing input, and replacement requests without leaking
//	the demonstration into another request.
//
// `workflowrun.Authorisation` is the closest existing shape and the reason this is not a boolean:
// a session flag granted while looking at the Play menu is still set four screens later, against
// a re-acquired control nobody has seen, in a window something else has covered. So this names
// the request, names the place, carries the observation it was asked in, and expires.
//
// # What it cannot do
//
// It holds no store, performs nothing, and observes nothing. A press arrives as a value from the
// existing navigation producer and a crossing arrives as a value from the existing correlator;
// this decides what they mean for one question and produces either a refusal or a record for
// somebody else to write.

import (
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// HowLongAQuestionStaysOpen is how long a person has to answer before the question lapses.
//
// # Why there is a bound at all
//
// Because an open question is the only state in which a press is read as an answer, and a person
// who was asked and then went to lunch has not answered anything. Without an expiry the next
// press of the afternoon would settle a question about a screen that is no longer there.
//
// Ninety seconds: long enough to read a sentence, look at the screen, and aim deliberately —
// which is what is being asked for, and is slower than clicking. Short enough that the window in
// which an unrelated press can be misread is one a person would still remember being asked in.
//
// It is one of three independent reasons a press is refused — the place, the screen state and the
// window all have to match as well — so a generous bound here is not the only thing standing
// between an unrelated click and a durable fact.
const HowLongAQuestionStaysOpen = 90 * time.Second

// Pending is one surface question, bound to the request and the place it was asked about.
type Pending struct {
	// Question is what was asked, with the candidate surfaces as they were then.
	Question Question
	// Request is the instruction that raised it, verbatim, so a second instruction cannot
	// be answered by a press meant for this one.
	Request string
	// Application and Place are the scope the answer will be stored under. A question with
	// no Place can still be ASKED and answered — the person is not at fault for Marco not
	// recognising the screen — and `Learned` refuses to make a record of it, because a
	// target with no place has no durable identity to hang one on.
	Application string
	Place       string
	// WindowID is the window the candidates are in.
	WindowID directorapi.WindowID
	// Session and State are the observation this was asked in.
	//
	// The screen state is the load-bearing one: the correlator files every press against
	// the session-local state it was performed ON, so requiring a press to carry this state
	// is what ties the press to the screen the question was about rather than to whatever
	// is in front afterwards. Session is carried with it because state ids are session-local
	// counters and `state_2` in two sessions are unrelated screens.
	Session observe.SessionID
	State   observe.ScreenStateID
	// Asked and Expires bound it.
	Asked   time.Time
	Expires time.Time

	// settled is the press that answered it, and settledOn which option that was.
	settled   *Answer
	pressedAt time.Time
	// contested says two presses settled on two different surfaces, so which one the
	// person meant is not something this knows.
	contested bool
	// closed says the question is over: cancelled, answered and learned, or abandoned.
	closed string
}

// NewPending opens a question about one request, giving the person the full bound to answer it.
func NewPending(q Question, request, application, place string, window directorapi.WindowID,
	session observe.SessionID, state observe.ScreenStateID, now time.Time) *Pending {

	return NewPendingUntil(q, request, application, place, window, session, state, now,
		now.Add(HowLongAQuestionStaysOpen))
}

// NewPendingUntil opens a question that lapses at a deadline the caller already holds.
//
// # Why the deadline is ever somebody else's to supply
//
// Because one TEACHING INTERACTION can outlive several observation sessions. Ambient watching
// runs bounded sessions, so the session a question was asked in ends while the person is still
// reading the sentence; the clarification is then derived again from a fresh reading in the next
// session, which is a NEW question about the same instruction.
//
// That re-derivation must not hand the person another ninety seconds. If it did, the only bound
// on a learning interaction would be reset by the very thing that was meant to be bounded, and a
// request could stay answerable all afternoon by rolling over often enough. So the deadline is
// set once, when the interaction first opens a question, and carried.
//
// `Asked` is still the moment THIS question was asked, because that is what the lapse sentence
// below is about and what a reader of one question needs; the deadline is the interaction's.
//
// Enforced by cmd/director TestTheOriginalDeadlineSurvivesEveryRollover.
func NewPendingUntil(q Question, request, application, place string,
	window directorapi.WindowID, session observe.SessionID, state observe.ScreenStateID,
	now, expires time.Time) *Pending {

	return &Pending{
		Question: q, Request: request, Application: application, Place: place,
		WindowID: window, Session: session, State: state,
		Asked: now, Expires: expires,
	}
}

// Open reports whether a press could still answer this, and why not.
func (p *Pending) Open(now time.Time) (bool, string) {
	switch {
	case p == nil:
		return false, "no question is open"
	case p.closed != "":
		return false, p.closed
	case p.contested:
		return false, "two presses settled on two different surfaces, so which one was " +
			"meant is not recoverable"
	case !now.Before(p.Expires):
		return false, fmt.Sprintf("the question was asked %s ago and has lapsed",
			now.Sub(p.Asked).Round(time.Second))
	}
	return true, ""
}

// Close ends the question, saying why.
//
// Cancellation, a replacement request, and a learned answer all come through here, so there is
// one place a question stops being answerable and one sentence explaining it.
func (p *Pending) Close(why string) {
	if p == nil || p.closed != "" {
		return
	}
	if why == "" {
		why = "the question was closed"
	}
	p.closed = why
}

// Closed reports why the question is over, empty while it is live.
func (p *Pending) Closed() string {
	if p == nil {
		return "no question is open"
	}
	return p.closed
}

// Settled is the answer so far, nil until a press told the surfaces apart.
func (p *Pending) Settled() *Answer {
	if p == nil {
		return nil
	}
	return p.settled
}

// Press offers one observed press as an answer, and says what it did.
//
// # Every way a press is not an answer to THIS question
//
// Each is a separate clause with its own sentence, because they send a person somewhere
// different: a press on the wrong screen means they moved, a press in another session means the
// observation rolled over, a press outside every candidate means the question was about the wrong
// thing, and a press inside several means they aimed at the overlap.
//
// What this does NOT have to check is whether the press came from a hand, was inside the watched
// window, or landed on Marco's own surface — `navsource` refuses all three before an event is
// ever admitted, and re-checking them here would be a second copy of a policy that lives at the
// boundary.
func (p *Pending) Press(in observe.AttributedInput, now time.Time) (Answer, bool, string) {
	if ok, why := p.Open(now); !ok {
		return Answer{}, false, why
	}
	if in.Event.Intent != observe.NavPoint {
		return Answer{}, false, fmt.Sprintf("%q is not a press", in.Event.Intent)
	}
	// THE SCREEN IT WAS PERFORMED ON, which is the whole of the correlation.
	//
	// Not the screen in front now: an event drained a second later may have been banked
	// before the person's own press changed the screen, so the current state is very often
	// the DESTINATION. The correlator stamps every event with the state it was performed on
	// for exactly this reason, and this is that stamp being used.
	if in.State != p.State {
		return Answer{}, false, fmt.Sprintf("the press was made on %s and this question "+
			"is about %s, so the screen had already moved", in.State, p.State)
	}
	a := p.Question.Answer(in.Event.Where)
	if a.Outcome != OutcomeSettled {
		return a, false, a.Because
	}
	// A SECOND PRESS THAT SETTLES SOMEWHERE ELSE CONTESTS THE FIRST.
	//
	// Two presses on two different surfaces are two things somebody did and this cannot say
	// which they meant — the same refusal `TransitionAttributed` makes of a crossing
	// attributed to two actions, and for the same reason. A second press on the SAME surface
	// is the same answer again and changes nothing.
	if p.settled != nil && p.settled.Settled != nil && a.Settled != nil &&
		p.settled.Settled.ElementID != a.Settled.ElementID {
		p.contested = true
		return a, false, fmt.Sprintf("an earlier press settled on %s and this one "+
			"settles on %s, so which was meant is not recoverable",
			p.settled.Settled.ElementID, a.Settled.ElementID)
	}
	if p.settled == nil {
		p.settled, p.pressedAt = &a, now
	}
	return a, true, a.Because
}

// Learned turns an attributed crossing into the durable fact the press established.
//
// # The three facts, kept apart
//
// A press observed is not a transition attributed and neither is an intended outcome verified.
// This is the second: the caller has a crossing in hand and `observe.TransitionAttributed` is the
// only thing that can turn one into a confirmation able to aim anything. Nothing here can assert
// a level, which is the whole point of that type.
//
// # What it refuses
//
// A question nobody answered, an answer that did not tell the surfaces apart, a crossing in
// another application, and a question with no durable place — the last because a semantic target
// is `{label, kind, place}` and one with no place could never be found again, so a record of it
// would be a saved aiming decision nothing can ever look up.
func (p *Pending) Learned(target, by string, w observe.TransitionWitness) (
	observe.DemonstratedSurface, error) {

	switch {
	case p == nil:
		return observe.DemonstratedSurface{}, fmt.Errorf("surface: no question is open")
	case p.contested:
		return observe.DemonstratedSurface{}, fmt.Errorf("surface: two presses settled " +
			"on two different surfaces, so nothing was established")
	case p.settled == nil:
		return observe.DemonstratedSurface{}, fmt.Errorf("surface: nothing has settled " +
			"this question, so there is nothing to remember")
	case p.Place == "":
		return observe.DemonstratedSurface{}, fmt.Errorf("surface: this screen has no " +
			"durable identity, so a target on it has none either and a record of " +
			"it could never be found again")
	case w.Application == "" || !equalFold(w.Application, p.Application):
		return observe.DemonstratedSurface{}, fmt.Errorf("surface: the crossing is in "+
			"%q and the question was about %q", w.Application, p.Application)
	}
	// THE CONFIRMATION, through the one door. Every way the crossing means nothing is
	// refused there rather than here, so there is one statement of what counts.
	confirmed, err := observe.TransitionAttributed(w)
	if err != nil {
		return observe.DemonstratedSurface{}, err
	}
	return p.Question.Demonstrated(*p.settled, Given{
		Application: p.Application, Target: target, By: by,
		At: w.At, Confirmed: confirmed,
	})
}

// Unconfirmed is what the press established and nothing confirmed, for the record and the report.
//
// A real thing that happened, and NOT usable to aim: `PressWasObserved` cannot produce a
// sufficient confirmation, so the record this returns is refused by `Usable` wherever it is read.
// It exists so a person who demonstrated something and whose screen did not visibly change is
// told what Marco saw rather than told nothing.
func (p *Pending) Unconfirmed(target, by string, now time.Time) (
	observe.DemonstratedSurface, error) {

	if p == nil || p.settled == nil {
		return observe.DemonstratedSurface{}, fmt.Errorf("surface: nothing has settled " +
			"this question")
	}
	confirmed, err := observe.PressWasObserved(p.settled.Because)
	if err != nil {
		return observe.DemonstratedSurface{}, err
	}
	return p.Question.Demonstrated(*p.settled, Given{
		Application: p.Application, Target: target, By: by,
		At: now, Confirmed: confirmed,
	})
}

// equalFold is strings.EqualFold, kept local so this file imports no more than it needs.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
