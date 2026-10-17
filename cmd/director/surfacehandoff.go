package main

// What became of every press offered to an open surface question.
//
// # The run this exists for
//
// A live demonstration ended with three facts printed — observed press false, attributed
// transition false, intended outcome not implemented — and no account of why. The press had been
// made: the page it was made on navigated, independently of Marco, and a separate measurement
// later showed the same desktop delivering presses into `navsource` and placing them inside the
// watched window. So the press was admitted and then disappeared, and nothing said where.
//
// It disappeared in `offerPresses`, which had four ways to drop one without a word: a look from
// another observation session, a look from another application, a press whose screen state is not
// the question's, and an outcome its switch had no case for. The fourth is the quiet one, because
// `Pending.Press` returns a ZERO Answer when the states disagree — and a zero `Outcome` matches
// neither OutcomeUndiscriminating nor OutcomeElsewhere.
//
// This record is the answer to "which of those four". It changes no decision: every gate still
// refuses exactly what it refused before, and nothing here extends a question's life, relaxes an
// identity or invents a fallback.
//
// # What it may hold
//
// Counts, and the producer's own sentences about identity and placement. No key, no typed
// content, no pointer position — the identities are a session-local counter, an application name
// and an element id, all three of which already appear in the account this prints beside.

import (
	"fmt"
	"strings"
)

// MaxHandoffNotes bounds the per-press account.
//
// Small, because the question is "which gate" and not "every press of the afternoon": a
// demonstration is one press and a handful of looks, and a session-long log would be a different
// thing with different privacy questions. Overflow keeps the FIRST notes rather than the last —
// the first time a press was dropped is the one that explains the demonstration.
const MaxHandoffNotes = 24

// pressHandoff is what the learner saw, per gate.
//
// COUNTS ARE PER OFFER, not per distinct press. `offerPresses` is handed the session's whole
// admitted input log on every look, so one press is offered again on each subsequent look and
// counted again here. That is reported rather than deduplicated, because deduplicating would mean
// a cursor, a cursor would change which presses the learner considers, and this is a measurement
// that must not change the thing it measures.
type pressHandoff struct {
	// Offers is calls to offerPresses, and Looks the ones carrying a usable reading.
	Offers       int
	LooksRefused int
	// NoQuestion is offers that arrived with nothing pending.
	NoQuestion int
	// PressesSeen is pointer presses found in the log across all offers.
	PressesSeen int
	// The four gates, each of which used to return in silence.
	Lapsed           int
	OtherSession     int
	OtherApplication int
	// Rolled is learning interactions carried from one observation session to the next.
	//
	// Not a refusal. It is counted here because it is the OUTCOME of the OtherSession
	// refusals above and of the supervisor ending a session, and a reader looking at a
	// column of refused presses needs to see that the interaction went on.
	Rolled int
	// NotAPress is a logged input that is not a pointer press, which is ordinary.
	NotAPress int
	// The three outcomes the learner acts on, and the one it had no case for.
	Settled          int
	Undiscriminating int
	Elsewhere        int
	Unaccounted      int
	Notes            []string
}

// note records one sentence, keeping the earliest.
func (h *pressHandoff) note(format string, args ...any) {
	if len(h.Notes) >= MaxHandoffNotes {
		return
	}
	h.Notes = append(h.Notes, fmt.Sprintf(format, args...))
}

// anything reports whether a press was ever offered at all.
func (h pressHandoff) anything() bool { return h.PressesSeen > 0 }

// handoffLines is the account in words, separated from collecting it.
//
// Separated for the reason every report in this tree is: collecting needs a desktop and a person,
// and the words are the whole value. A reader has to be able to tell "nobody pressed anything"
// from "a press arrived and was refused for this reason", because those send them to different
// places — the first to the person and the second to the code.
func handoffLines(h pressHandoff) string {
	out := "\nTHE HANDOFF from admitted input to the open question:\n"
	out += fmt.Sprintf("  offers ................... %d (%d carried no usable reading)\n",
		h.Offers, h.LooksRefused)
	out += fmt.Sprintf("  presses seen in the log .. %d (counted once per offer, so one "+
		"press recurs)\n", h.PressesSeen)
	out += fmt.Sprintf("  nothing was pending ...... %d\n", h.NoQuestion)
	out += fmt.Sprintf("  question had lapsed ...... %d\n", h.Lapsed)
	out += fmt.Sprintf("  another session .......... %d\n", h.OtherSession)
	out += fmt.Sprintf("  another application ...... %d\n", h.OtherApplication)
	out += fmt.Sprintf("  carried to a new session . %d\n", h.Rolled)
	out += fmt.Sprintf("  settled .................. %d\n", h.Settled)
	out += fmt.Sprintf("  undiscriminating ......... %d\n", h.Undiscriminating)
	out += fmt.Sprintf("  elsewhere ................ %d\n", h.Elsewhere)
	out += fmt.Sprintf("  refused for another reason %d\n", h.Unaccounted)
	if len(h.Notes) > 0 {
		out += "  what each refusal said:\n"
		for _, n := range h.Notes {
			out += "    " + n + "\n"
		}
	}
	switch {
	case h.Offers == 0:
		out += "  READ THIS AS: the learner was never offered a reading at all, which " +
			"is about the observer and not about the press.\n"
	case !h.anything():
		out += "  READ THIS AS: no pointer press was ever in the log, so nothing was " +
			"refused — either none was made inside the watched window, or it never " +
			"reached the producer. The producer's own counters say which.\n"
	case h.Settled == 0:
		out += "  READ THIS AS: a press WAS offered and every one of them was refused. " +
			"The reasons above are the answer, and none of them is the person's " +
			"aim unless it says undiscriminating.\n"
	}
	return strings.TrimRight(out, "\n") + "\n"
}
