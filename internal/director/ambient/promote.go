package ambient

import (
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// When repeated watching may become knowing.
//
// # The progression this sits in the middle of
//
//	I SAW YOU CROSS IT ONCE          candidate evidence
//	IT WAS A CLEAN CROSSING          durable EdgeObserved knowledge   <- here
//	I SEE YOU CROSS IT AGAIN         the same edge, stronger
//	THE PERSON ASKS ME TO DO IT      ordinary authority
//	I DO IT                          Theater
//	I PROVE IT WORKED                EdgeVerified evidence
//
// This file owns exactly one step of that: deciding, from a summary and a policy, whether an
// observed relationship has earned a place in durable memory. It is PURE — no store, no clock it
// reads for itself, no I/O, and nothing reachable from here that could write anything.
//
// # Why the first policy is deliberately dull
//
// Because the failure mode is permanent. A play a person did not want can be deleted; a piece of
// semantic memory Marco is confident about quietly changes what it plans forever, and nobody
// asked for it. It is better for Marco to hold something back and say why than to learn an
// accident, so every condition below is binary, every refusal names itself, and there is no score
// anywhere. A number between nought and one is a way of avoiding a decision, and the decision
// here is the product.
//
// # What it is NOT
//
// It is not permission to act. A promoted edge is a thing Marco KNOWS, and knowing has never been
// authority in this system: the grant is minted per invocation at the ordinary door, the window
// must lead before any input is emitted, and every edge is verified as it is walked. See
// [[ADR-095-repeated-observation-may-become-knowledge]].

// DefaultTraversals is how many clean traversals the policy asks for before an edge is admitted.
//
// ONE. A clean human traversal of A --X--> B is already knowledge: the person went there, by
// pressing that, and arrived. Repeating it does not make the relationship more TRUE — it makes
// Marco more confident that it is reliable, which is a different question and belongs in the
// evidence rather than in whether the fact exists.
//
// # It was two, and that was the wrong model
//
// The first policy asked for two independent demonstrations, on the reasoning that "one is not
// evidence of a habit". That reasoning is right about HABITS and this is not a habit store: it is
// a graph of what the interface is. Waiting for a repetition to admit an edge is waiting for
// somebody to prove that a door they just walked through is still a door.
//
// Worse, it made a product out of the wait. Somebody had to perform an action, wait, and perform
// it again before Marco would admit what it had plainly seen — ceremony, in the one mode whose
// whole promise is that there is no ceremony. See ADR-095's correction.
//
// It remains configurable, so a deployment that wants corroboration before admission can ask for
// it, and so the refusal it produces stays reachable and tested.
const DefaultTraversals = 1

// There is deliberately no minimum time between traversals.
//
// # What the old sixty-second gap was for, and why it was answering the wrong question
//
// It existed so a thousand provider samples of one action could not read as a thousand
// demonstrations. That is a real hazard and it is handled twice already, upstream, where it
// belongs: duplicate input events are coalesced into one act, and a crossing is only recorded when
// the PLACE CHANGES. One traversal is one recorded step, whatever the desktop emitted.
//
// So the clock was measuring nothing that mattered — and it charged for it. A person who did a
// thing, went back, and did it again fifteen seconds later had performed two real traversals and
// was credited with one, because a timer said so. Semantic re-entry is what makes a second
// traversal second: to do A --X--> B again you have to get back to A, and getting back to A is
// itself a recorded step.
//
// What DOES still separate evidence is the watching session, and that is provenance rather than a
// threshold: see WatchedEdge.Sessions.

// Policy is the promotion rule's configuration.
//
// Held as a value and passed in, so the decision is a pure function of its arguments and a test
// can state the policy it is testing rather than depending on a default that may change.
type Policy struct {
	// Enabled is whether ambient promotion may make anything durable at all.
	//
	// SEPARATE FROM WATCHING, deliberately and structurally. "Marco is paying attention" and
	// "Marco is building permanent memory from what it sees" are two different things to
	// agree to, and a single switch that silently did both would turn `marco observe` into
	// something nobody said yes to. See ADR-095, and ADR-093 for the same argument about
	// watching itself.
	Enabled bool
	// Traversals is how many clean traversals are required before admission. Zero means the
	// default, which is one: a clean traversal is already knowledge.
	Traversals int
	// Freshness bounds how old the most recent evidence may be. Zero means no bound, which is
	// the first policy's choice: see the note on Stale.
	Freshness time.Duration
}

// traversals reads the policy with its default applied.
func (p Policy) traversals() int {
	if p.Traversals > 0 {
		return p.Traversals
	}
	return DefaultTraversals
}

// Verdict is what the policy concluded, from a closed vocabulary of three.
type Verdict string

const (
	// Promote is evidence that has earned durable memory.
	Promote Verdict = "promote"
	// Wait is evidence that may yet: nothing is wrong with it, there is not enough of it.
	Wait Verdict = "wait"
	// Never is evidence that cannot become knowledge as it stands, however often it recurs.
	//
	// Its own verdict rather than a kind of Wait, because the two mean opposite things to
	// everything that reads them: one is a candidate worth keeping and strengthening, the
	// other is one whose repetition will never help. Eviction and diagnostics both need to
	// tell them apart.
	Never Verdict = "never"
)

// Why is the closed vocabulary of reasons, so a refusal can be explained rather than counted.
type Why string

const (
	// WhyEnough is the affirmative case.
	WhyEnough Why = "enough_evidence"
	// WhyDisabled is ambient promotion switched off. Nothing is wrong with the evidence.
	WhyDisabled Why = "promotion_disabled"
	// WhyTooFew is fewer independent occasions than the policy asks for.
	WhyTooFew Why = "not_seen_often_enough"
	// WhyContradicted is the same action on the same control arriving somewhere else.
	//
	// It is Never rather than Wait on purpose. More of the same evidence does not resolve a
	// contradiction — it deepens it — and a policy that waited would accumulate forever
	// against a screen Marco does not understand. Explicit Learn is the way through, because
	// a person saying "this is what I mean" is information a repetition is not.
	WhyContradicted Why = "contradictory_evidence"
	// WhyUnnamedTarget is an activation on a control whose name was withheld.
	WhyUnnamedTarget Why = "control_not_named"
	// WhyUnknownPlace is an endpoint Marco can neither recognise nor describe well enough to
	// establish.
	WhyUnknownPlace Why = "unrecognised_screen"
	// WhyUnsupported is an action the durable representation has no sentence for.
	WhyUnsupported Why = "unsupported_action"
	// WhyStale is evidence too old for the policy's recency bound.
	WhyStale Why = "evidence_stale"
	// WhyAlready is a relationship this candidate has already been promoted into.
	WhyAlready Why = "already_known"
)

// Judgement is the decision and the reason for it.
type Judgement struct {
	Verdict Verdict
	Why     Why
	// Short is how many more independent occasions Wait is waiting for. Zero otherwise.
	Short int
}

// Judge decides whether one candidate has earned durable memory.
//
// # The order of the questions, and why it is the order
//
// The things that can never change go first — a contradiction, an unnameable control, a screen
// nothing can establish, an action the language cannot say — so a candidate that is structurally
// unpromotable is reported as such rather than as one occasion short forever. Only then does the
// count matter.
//
// `Enabled` is asked FIRST of all, before any of it. A switched-off policy has no opinion about
// somebody's evidence, and answering "not seen often enough" while switched off would be a
// sentence about the wrong thing.
//
// Deterministic in its arguments including the clock, which is passed rather than read.
func Judge(e observe.WatchedEdge, p Policy, now time.Time) Judgement {
	if !p.Enabled {
		return Judgement{Verdict: Wait, Why: WhyDisabled}
	}
	if !e.Promoted.IsZero() {
		return Judgement{Verdict: Never, Why: WhyAlready}
	}
	if e.Contradicted > 0 {
		return Judgement{Verdict: Never, Why: WhyContradicted}
	}
	if !ActionKind(e.Kind).Known() {
		return Judgement{Verdict: Never, Why: WhyUnsupported}
	}
	if !(Act{Kind: ActionKind(e.Kind), Target: Target{Role: e.Role, Label: e.Target}}).
		Representable() {
		return Judgement{Verdict: Never, Why: WhyUnnamedTarget}
	}
	if !e.Describable() {
		// NOT Never. A screen Marco cannot describe today may be one it recognises
		// tomorrow — memory improves, and the same evidence is re-judged every time. See
		// [[ADR-021-a-judgement-is-recomputed-not-recorded]], which is why nothing here is
		// written into the record.
		return Judgement{Verdict: Wait, Why: WhyUnknownPlace}
	}
	if p.Freshness > 0 && !e.Last.IsZero() && now.Sub(e.Last) > p.Freshness {
		return Judgement{Verdict: Wait, Why: WhyStale}
	}
	if want := p.traversals(); e.Seen < want {
		return Judgement{Verdict: Wait, Why: WhyTooFew, Short: want - e.Seen}
	}
	return Judgement{Verdict: Promote, Why: WhyEnough}
}

// Fold adds one observed step to a candidate summary and returns the result.
//
// # It is pure, and it decides nothing about promotion
//
// Given the same candidate, step and clock it returns the same summary, every time. Judging that
// summary is a separate function so that the two can be tested apart — and so that a policy
// change can never accidentally alter what was OBSERVED, which is a fact about the world rather
// than about Marco's rules.
//
// # Contradiction is recorded here rather than judged here
//
// A step that begins where this candidate begins, by the same action on the same control, and
// arrives somewhere ELSE, is counted against it. The caller finds that candidate — see the
// matching rule in the Director's ledger — and hands it here; this only knows how to add one.
func Fold(e observe.WatchedEdge, s Step, sameSession bool) observe.WatchedEdge {
	// ONE RECORDED CROSSING IS ONE TRAVERSAL. No clock, no threshold.
	//
	// The two things that could make that untrue are both handled upstream: duplicate provider
	// events are coalesced into one act, and a crossing is only recorded when the PLACE
	// CHANGES — so a screen sampled forty times produces no traversal at all, and one press
	// arriving as a burst produces one.
	e.Seen++
	if !sameSession || e.Sessions == 0 {
		// A DIFFERENT WATCHING SESSION is provenance rather than a threshold: it says this
		// relationship has been seen across a restart, a different window generation, very
		// often a different day. It strengthens confidence and gates nothing.
		e.Sessions++
	}
	if e.First.IsZero() {
		e.First = s.At
	}
	e.Last = s.At
	e.From = foldEnd(e.From, s.From, s.FromShape)
	e.To = foldEnd(e.To, s.To, s.ToShape)
	return e
}

// foldEnd updates one end of a candidate from one sighting.
//
// # A screen that has SINCE been recognised loses its description and gains its identity
//
// Memory improves between two sightings of the same thing — an explicit Learn somewhere else, a
// place established on another route — and a candidate that went on carrying the shape it was
// first seen as would ask a promotion to establish a place that already exists. Identity wins the
// moment there is one, and the description it replaces was only ever standing in for it.
//
// The reverse never happens: an end that has an identity does not go back to being a shape.
func foldEnd(end observe.WatchedEnd, key string, shape *Shape) observe.WatchedEnd {
	if Recognised(key) {
		return observe.WatchedEnd{Subject: key}
	}
	if end.Recognised() {
		return end
	}
	if shape == nil {
		return end
	}
	// THE FIRST DESCRIPTION IS KEPT, and a later reading does not replace it.
	//
	// Two readings of one screen differ in small ways — that is why the canonical matcher has
	// tolerances at all — and overwriting the description on every sighting would make the
	// candidate's own content move under it. Any one of them is as good as any other for
	// establishing the place, because the store matches with the same tolerance.
	//
	// A NAME is different and does get filled in: it settles by recurrence rather than being
	// read off one sample, so the first sighting often has none and a later one does.
	if end.Shape == nil {
		sig := shape.Signature
		end.Shape = &sig
	}
	if end.Called == "" {
		end.Called = shape.Called
	}
	return end
}

// Contradict records that the same beginning and the same action arrived somewhere else.
func Contradict(e observe.WatchedEdge, at time.Time) observe.WatchedEdge {
	e.Contradicted++
	e.Last = at
	return e
}

// Describe renders a judgement for a person, in their words rather than the vocabulary's.
//
// The refusals are the point of this: "I have seen this twice and I am not ready to remember it"
// and "that button leads to two different places" are different states, and a diagnostic that
// rendered both as "not learned" would be a diagnostic nobody could act on.
func Describe(j Judgement) string {
	switch j.Why {
	case WhyEnough:
		return "seen often enough, consistently, to remember"
	case WhyDisabled:
		return "watching only: learning from what I see is switched off"
	case WhyTooFew:
		if j.Short == 1 {
			return "seen once; I remember this kind of thing after I have seen it again"
		}
		return "not seen often enough yet"
	case WhyContradicted:
		return "that control has led somewhere different, so I don't understand the screen " +
			"well enough to remember this"
	case WhyUnnamedTarget:
		return "I couldn't read what the control was called, so I can't say what to press"
	case WhyUnknownPlace:
		return "one end of this is a screen I can't describe well enough to find again"
	case WhyUnsupported:
		return "I have no way to write down what you did there"
	case WhyStale:
		return "I last saw this too long ago to act on it now"
	case WhyAlready:
		return "I already know this"
	}
	return string(j.Why)
}
