package main

import (
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/outcome"
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
//
// # Why the six words are no longer written down in this file
//
// They used to be. This file held its own `outcome` type, its own six constants and its own
// copy of the literal `"[result] "`, matched by hand against cmd/marco's — across a module
// boundary, where no compiler can look. The comment beside them said it out loud: nothing
// would fail to compile if either drifted. That is an accurate description of a duplicate,
// written next to the duplicate, in place of removing it. And it was not two copies but
// three, because the control centre had neither and simply reported `{"ok":true}` before
// anything had run.
//
// [outcome] is now the one vocabulary, imported by all three. The overlay is a separate Go
// module but it may import the engine's internal packages, so this costs nothing.
//
// WHAT STAYED HERE IS THE PHRASING. "ran: X", "asked about: X — answer it", "refused: X"
// are the HUD's own sentences and belong to the HUD; the control centre says the same six
// things in its own words. What moved is the vocabulary and the wire literal — the parts
// that have to be identical — and not the presentation, which must not be.

// routePrefix says WHICH play resolved. Deliberately separate from [outcome.ResultPrefix]:
// `[route] ` answers "what did these words become", `[result] ` answers "what happened to
// it". The learn offer needs both facts and cannot be derived from either alone.
//
// It is the engine's constant itself, not a copy of it, and that is the whole of what this
// line does — a compiler now owns the agreement between the HUD and [outcome]. The comment
// that used to stand here said the opposite ("still a hand-matched literal … nothing links
// the two at compile time") directly above the line that links them; it described the file
// as it was before internal/outcome existed. Asserting the VALUE cannot tell the two
// arrangements apart, so the link is held structurally by
// TestTheRoutePrefixIsTheEnginesConstantItself.
//
// What is STILL hand-matched is the PRODUCER: cmd/marco/intake.go prints the tag with a
// literal format string, in the root module, where nothing compares it to anything.
// TestTheRoutePrefixIsStillPinnedByHand reads that source across the boundary and is the
// only defence there is.
const routePrefix = outcome.RoutePrefix

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
// An unrecognised word is NOT rendered as itself: [outcome.Parse] refuses it, and a front
// end that invented a seventh state from a drifted engine would be describing something
// nobody defined.
//
// Deleting the `[result] ` read must fail TestTheSixOutcomesComeFromTheWire.
func (r childRun) outcome() outcome.Outcome {
	if o, ok := outcome.Parse(strings.TrimSpace(r.result)); ok {
		return o
	}
	switch {
	case r.killed:
		return outcome.Cancelled
	case r.err != nil:
		return outcome.Failed
	}
	return outcome.Performed
}

// offersLearn is the ONE condition under which an unknown command becomes an offer to
// record a demonstration: nothing took the request at all.
//
// Both halves are load-bearing. `unavailable` alone is not enough — a resolved play whose
// bridge could not be reached is unavailable and already exists, so offering to learn it
// would invite somebody to learn a play Marco already learned. And a Director that RAN and
// failed is not an unknown command: answering "I could not do that" with "shall I learn
// it?" is a non-sequitur about something the person just watched go wrong.
//
// Deleting either half must fail TestTheLearnOfferNeedsBothHalves.
func (r childRun) offersLearn() bool {
	return r.outcome() == outcome.Unavailable && r.route == ""
}

// statusLine is the one-line HUD status for an outcome.
//
// This is PRESENTATION, and it is the HUD's alone. The six words are protocol; these six
// sentences are the ambient surface's voice, sitting on the same line that says "running:
// X" a moment earlier, which is why they read as a continuation of it rather than as six
// nouns. Another surface rendering the same six words says something else, and should.
//
// Every one of the six must produce its own sentence — a missing arm would collapse two
// genuinely different endings onto one line, which is the whole defect this vocabulary
// removed. Enforced by TestTheHudRendersEveryOutcomeInTheSet.
func statusLine(o outcome.Outcome, disp string) string {
	switch o {
	case outcome.Performed:
		return "ran: " + disp
	case outcome.Clarify:
		return "asked about: " + disp + " — answer it"
	case outcome.Refused:
		return "refused: " + disp
	case outcome.Unavailable:
		return "unavailable: " + disp
	case outcome.Cancelled:
		return "cancelled: " + disp
	default:
		return "failed: " + disp
	}
}
