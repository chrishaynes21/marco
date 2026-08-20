package orchestrator

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The door between knowing which play you mean and being allowed to perform it.
//
// # Why this file exists
//
// Because until now there was no door. `Do` resolved a phrase to a route and called `Run` in the
// same breath — one function, no seam, nothing that could hold "I believe this is the play you
// mean" without also performing it.
//
// That was fine while every play was one a person had written or recorded by demonstration: they
// asked for it, so they authorized it. It stops being fine the moment a play exists that MARCO
// wrote, from behaviour it watched, because the user asking for it by name is not the same as the
// user having read what it does.
//
// So the seam is general. Every route goes through it, authored and taught included, and the
// policy is what differs — not the path.
//
// See [[ADR-029-resolution-is-not-permission]].

// Resolved is a play the system believes the user meant, held WITHOUT performing it.
//
// The type that makes "resolution is not authority" structural: it can be constructed, inspected,
// logged and refused, and there is nothing on it that runs anything. `Deps.Run` takes a Route, and
// getting one out of here means going through a decision.
type Resolved struct {
	// Route is which play, in which scope.
	Route routes.Route
	// Phrase is what the user actually said, for the sentence they are shown.
	Phrase string
	// Kind is where the play came from: authored, taught, or learned.
	Kind routes.Kind
	// Provenance is the state of its origin record. Only `intact` means the file is still
	// the artifact Director verified.
	Provenance routes.OriginState
}

// Learned reports a play Marco wrote from behaviour it watched, still matching its provenance.
//
// The ONE case that gets a different policy. A learned play whose source has been edited is not
// this — it is an ordinary play that remembers where it started, and it is treated as one.
func (r Resolved) Learned() bool {
	return r.Kind == routes.KindLearned && r.Provenance.Verified()
}

// Verdict is what the authority seam decided. Discrete, and never a score.
type Verdict string

const (
	// Allowed is an ordinary play the user asked for. Authored and taught plays are this:
	// somebody wrote them, and asking for one by name has always been enough.
	Allowed Verdict = "allowed"
	// NeedsConfirmation is a learned play whose provenance is intact.
	//
	// Not because it is dangerous, but because the user has never read it. Marco wrote this
	// one from something it watched; "you asked for it" is weaker evidence of consent when
	// the thing asked for was composed by the thing being asked.
	NeedsConfirmation Verdict = "needs_confirmation"
	// Declined is the user saying no. NOT the same as Marco believing the play is unsafe, and
	// not the same as not knowing which play was meant.
	Declined Verdict = "declined"
	// Refused is Marco declining on its own account.
	Refused Verdict = "refused"
)

// Decision is the verdict plus the reason a person is told.
type Decision struct {
	Verdict Verdict
	// Reason is the closed vocabulary. Empty when allowed outright.
	Reason string
	// Sentence is what the user reads. Backstage words never appear here.
	Sentence string
}

// Allow reports whether execution may proceed.
func (d Decision) Allow() bool { return d.Verdict == Allowed }

// Reasons, in the closed vocabulary.
const (
	// ReasonOrdinary is an authored or taught play: the user's own writing.
	ReasonOrdinary = "ordinary_play"
	// ReasonLearnedFirstUse is a learned play the user has not confirmed this session.
	ReasonLearnedFirstUse = "learned_play"
	// ReasonEditedSinceLearned is a learned play whose source no longer matches its
	// provenance. It is treated as ordinary — the file is still perfectly good Marco, and
	// refusing it would be refusing the user's own edit.
	ReasonEditedSinceLearned = "edited_since_learned"
	// ReasonUserDeclined is the user saying no to this invocation.
	ReasonUserDeclined = "user_declined"
)

// Gate decides whether a resolved play may be performed now.
//
// An interface so the decision is somebody's to make rather than this package's to assume, and so
// a test can hold the seam open and look at it.
type Gate interface {
	Allow(Resolved) Decision
}

// Authorize is the seam every invocation passes through.
//
// # What the policy is, and why it is this
//
// An authored or taught play runs. It always has: a person wrote it or demonstrated it, and asking
// for it by name is the whole of the permission model Marco has ever had. Changing that here would
// be changing something this milestone was not asked to change.
//
// A LEARNED play with intact provenance asks first, once per invocation. Marco composed it from
// behaviour it watched; the user has never read the file, and "they asked for it" is thinner
// consent than usual. The question is asked about THIS invocation and answered for this
// invocation — see [[ADR-029-resolution-is-not-permission]] on why the file does not carry a
// permanent `Trusted: true`.
//
// A learned play that has been EDITED is an ordinary play. It is no longer the artifact Director
// verified, so the learned policy has nothing to apply to; and refusing it would be refusing the
// user's own writing, which no other play suffers.
func Authorize(r Resolved, g Gate) Decision {
	if r.Kind == routes.KindLearned && r.Provenance == routes.OriginEdited {
		return Decision{Verdict: Allowed, Reason: ReasonEditedSinceLearned,
			Sentence: "You have changed this one since Marco wrote it, so it runs like " +
				"anything else you have written."}
	}
	if !r.Learned() {
		return Decision{Verdict: Allowed, Reason: ReasonOrdinary}
	}
	if g == nil {
		// No way to ask, so no way to be told yes. Fail closed: a Marco with no means of
		// putting the question must not answer it on the user's behalf.
		return Decision{Verdict: Refused, Reason: ReasonLearnedFirstUse,
			Sentence: "Marco learned this play by watching you, and there is no way to ask " +
				"you about it here."}
	}
	return g.Allow(r)
}

// Classify reads a resolved route's provenance.
//
// The only thing that decides what kind of play this is. Not the path, not the name, not where it
// happens to sit — the origin record beside it, whose digest either still describes the file or
// does not.
// The switch that used to live here is now `routes.Registry.KindOf`, unchanged in behaviour and
// moved so the PRODUCT LISTING reads the same one. A Plays surface that derived kind and
// provenance for itself would be a second account of the fact this door is guarding, and the two
// would disagree the first time a state was added — which is how a surface comes to call a play
// learned that this function calls authored.
//
// What stays here is this function's own job: turning that answer into a Resolved, with the
// phrase the person actually said.
//
// Deleting the KindOf call and re-deriving the switch must fail
// TestTheDoorAndTheListingAgreeAboutEveryProvenance.
func Classify(reg routes.Registry, rt routes.Route, phrase string) Resolved {
	kind, state := reg.KindOf(rt)
	return Resolved{Route: rt, Phrase: phrase, Kind: kind, Provenance: state}
}

// AskToRun is the sentence a person is shown before a learned play runs.
//
// Their words for what they asked for, and Marco's own plain account of where the play came from.
// No candidate, no digest, no rehearsal, no evidence — those are backstage, and a question nobody
// can answer is not a question.
func AskToRun(r Resolved) string {
	return fmt.Sprintf("Marco learned %q by watching you. Run it now?", r.Phrase)
}

// AskFirst is the ordinary gate: put the question to the person, and take their answer.
//
// It reuses the confirmation this repository already has — the one that asks before recording a
// demonstration and before overwriting a simplified route — rather than building a second question
// system. The question is the only new thing, and it is one sentence.
type AskFirst struct{ Deps Deps }

// Allow asks, and reports what was said.
//
// The answer is about THIS invocation. Nothing is written down, nothing is remembered, and asking
// again next time is the correct behaviour rather than a missing feature: a durable play is
// knowledge, and permission to use it now is not the same thing.
//
// # No explicit yes is not a yes
//
// This deliberately does NOT reuse `Deps.confirm`, whose empty-line-is-yes default is right for a
// prompt about the user's OWN just-demonstrated play ("Run it now?") and wrong for a play MARCO
// composed by watching. Here the rule is fail-closed: only an explicit `y`/`yes` runs it. An empty
// line, end-of-input, or anything unrecognised is a play that ran because nobody said no — and a
// learned play must never execute because nobody answered. Silence is never yes.
//
// The three outcomes are kept apart because they send the user to different places: an explicit
// `no` is the user declining THIS play; anything else is Marco refusing on its own account because
// it never got a yes — which is the more accurate word than pretending the user said no.
func (a AskFirst) Allow(r Resolved) Decision {
	if a.Deps.In == nil {
		// Nothing to ask with. Fail closed for a learned play, exactly as a nil gate does.
		return Decision{Verdict: Refused, Reason: ReasonLearnedFirstUse,
			Sentence: "Marco learned this play by watching you, and cannot ask you about it " +
				"from here."}
	}
	fmt.Fprint(a.Deps.Out, AskToRun(r)+" [y]es / [n]o: ")
	switch strings.TrimSpace(strings.ToLower(readLine(a.Deps.In))) {
	case "y", "yes":
		return Decision{Verdict: Allowed, Reason: ReasonLearnedFirstUse}
	case "n", "no":
		return Decision{Verdict: Declined, Reason: ReasonUserDeclined,
			Sentence: "Left alone."}
	default:
		// Empty line, EOF, or an answer that is not a plain yes/no. Not an explicit yes, so the
		// play does not run — and this is Marco's own refusal, not a claim the user said no.
		return Decision{Verdict: Refused, Reason: ReasonLearnedFirstUse,
			Sentence: "Marco needs a yes to run a play it learned by watching you, and did not " +
				"get one. Left alone."}
	}
}
