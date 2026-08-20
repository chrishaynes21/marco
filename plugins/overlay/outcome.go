package main

import (
	"strings"
	"time"
)

// What became of one invocation, in the six words the engine announces.
//
// # Why an exit code was not enough
//
// The overlay used to derive three words — ok / canceled / failed — from a child's exit
// code, and three genuinely different things all came back as `ok`. A play the door
// DECLINED exited 0, because "you said no" is not an error. A play whose Screen guard
// refused caught its own failure and exited 0. So "Marco refused to do that" and "it
// worked" rendered identically in the history, which is the one place a person looks to
// find out whether the thing happened.
//
// The engine now says what happened on its own line and the overlay reads it. Deriving
// it here again would be a second result vocabulary, and the two would disagree.

// resultPrefix is the wire line the engine writes to say what became of an invocation.
//
// PRODUCED in the ROOT module by cmd/marco/intake.go (const resultPrefix, func announce).
// The overlay is a separate Go module, so nothing links the two literals at compile time
// and nothing fails to build when one side is reworded — the HUD would simply fall back
// to guessing from the exit code and start lying again, quietly.
//
// Changing it must fail TestResultPrefixAndVocabularyArePinned.
const resultPrefix = "[result] "

// routePrefix says WHICH play resolved. Deliberately unchanged and deliberately separate:
// `[route] ` answers "what did these words become", `[result] ` answers "what happened to
// it". The learn offer needs both facts and cannot be derived from either alone.
const routePrefix = "[route] "

// outcome is the engine's vocabulary, not a translation of it.
type outcome string

const (
	// outcomePerformed means it ran AND arrival was verified. Nothing else may claim it.
	outcomePerformed outcome = "performed"
	// outcomeClarify means Director asked something and is waiting to be told.
	outcomeClarify outcome = "clarify"
	// outcomeRefused means Marco declined or was not permitted — a door, a guard, an
	// edge that would not verify. Not an error, and not a success.
	outcomeRefused outcome = "refused"
	// outcomeUnavailable means nothing took the request. No play answered to it and
	// Director was never reached — the ONLY state in which "shall I learn this?" is an
	// honest thing to say.
	outcomeUnavailable outcome = "unavailable"
	// outcomeCancelled means somebody stopped it.
	outcomeCancelled outcome = "cancelled"
	// outcomeFailed means it was tried and it went wrong.
	outcomeFailed outcome = "failed"
)

// knownOutcome accepts only the six. An unrecognised word is NOT rendered as itself:
// a front end that invented a seventh state from a drifted engine would be describing
// something nobody defined.
func knownOutcome(s string) (outcome, bool) {
	switch outcome(strings.TrimSpace(s)) {
	case outcomePerformed:
		return outcomePerformed, true
	case outcomeClarify:
		return outcomeClarify, true
	case outcomeRefused:
		return outcomeRefused, true
	case outcomeUnavailable:
		return outcomeUnavailable, true
	case outcomeCancelled:
		return outcomeCancelled, true
	case outcomeFailed:
		return outcomeFailed, true
	}
	return "", false
}

// childRun is everything one spawned marco child reported about one invocation.
type childRun struct {
	// err is the process error, if any.
	err error
	// killed says THIS overlay killed the child (as opposed to it stopping by itself).
	killed bool
	// route is the play the engine announced on `[route] `; "" when nothing resolved.
	route string
	// result is the raw word from `[result] `; "" when the child never announced one —
	// every subcommand except the intake (`bind`, `forget`, `learn`, …).
	result string
	// dur is how long it took, for the history row.
	dur time.Duration
}

// outcome reads the engine's word, and only falls back where there is none to read.
//
// # The fallback is for the OTHER subcommands, not for the intake
//
// `marco bind` / `forget` / `rename` / `simplify` / `learn` announce no result line, and
// nothing is gained by teaching them one — they either did the thing or errored. The
// intake always announces, so a missing result there means the child died before it could
// speak, and "failed" is then the truthful reading of a non-zero exit.
//
// Deleting the `[result] ` read must fail TestTheSixOutcomesComeFromTheWire.
func (r childRun) outcome() outcome {
	if o, ok := knownOutcome(r.result); ok {
		return o
	}
	switch {
	case r.killed:
		return outcomeCancelled
	case r.err != nil:
		return outcomeFailed
	}
	return outcomePerformed
}

// offersTeach is the ONE condition under which an unknown command becomes an offer to
// record a demonstration: nothing took the request at all.
//
// Both halves are load-bearing. `unavailable` alone is not enough — a resolved play whose
// bridge could not be reached is unavailable and already exists, so offering to learn it
// would invite somebody to learn a play Marco already learned. And a Director that RAN and
// failed is not an unknown command: answering "I could not do that" with "shall I learn
// it?" is a non-sequitur about something the person just watched go wrong.
//
// Deleting either half must fail TestTheTeachOfferNeedsBothHalves.
func (r childRun) offersTeach() bool {
	return r.outcome() == outcomeUnavailable && r.route == ""
}

// status is the one-line HUD status for an outcome, in the engine's own words wherever
// they read as a sentence.
func (o outcome) status(disp string) string {
	switch o {
	case outcomePerformed:
		return "ran: " + disp
	case outcomeClarify:
		return "asked about: " + disp + " — answer it"
	case outcomeRefused:
		return "refused: " + disp
	case outcomeUnavailable:
		return "unavailable: " + disp
	case outcomeCancelled:
		return "cancelled: " + disp
	default:
		return "failed: " + disp
	}
}
