package main

// HOW IT WENT, IN A SENTENCE SOMEBODY DID NOT HAVE TO LEARN.
//
// # Why a sentence and not the word
//
// A run ends in one of seven words, and they are exactly right for a record and wrong for a
// person. `unconfirmed` is the one that makes the case: it means Marco pressed the control and
// could not get evidence either way, which is neither a success nor a failure and is the single
// most surprising thing a first run can do. Shown as a grey tag reading `unconfirmed`, it reads
// as jargon for "something went wrong".
//
// So the word stays — it is the outcome, it goes in the record, and a reader who knows it should
// see it — and beside it goes what it means for the thing the person asked for.
//
// # Why this is Go and not the page
//
// Because it is the part that can be wrong. There are seven outcomes and a run with none yet,
// and a page with a switch over them has no way to say which ones it covers; this has a test
// that walks all eight. The page renders what it is given.

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
)

// saidOf is what happened, for a person who pressed Play.
//
// `act` is the Act's name, `outcome` the run's own word, `reason` its own sentence about why.
// The reason is appended rather than paraphrased: it comes from the thing that decided, and a
// surface rewriting it would be putting its own words on somebody else's finding.
func saidOf(act string, outcome runref.Outcome, reason string) string {
	named := "it"
	if act != "" {
		named = fmt.Sprintf("%q", act)
	}
	var said string
	switch outcome {
	case "":
		return ""
	case runref.Verified:
		// THE ONLY ENDING THAT IS UNAMBIGUOUSLY GOOD, and it is worth saying what
		// the evidence was: a change that was absent before, present after, stood
		// still, and came from somewhere other than the control that was pressed.
		said = fmt.Sprintf("It worked. Marco performed %s and the screen changed "+
			"the way it did when you showed it.", named)
	case runref.Contradicted:
		// PRESSED, AND THE SCREEN DISAGREED. The press happened — that is the half
		// a person has to know first, because something on their machine moved.
		said = fmt.Sprintf("Marco performed %s and the screen did NOT change the "+
			"way it did when you showed it.", named)
	case runref.Unconfirmed:
		said = fmt.Sprintf("Marco performed %s and could not tell whether it "+
			"worked.", named)
	case runref.Refused:
		// NOTHING HAPPENED, which is the other half a person has to know first.
		said = fmt.Sprintf("Marco did not perform %s, and nothing was pressed.",
			named)
	case runref.Cancelled:
		said = "You stopped it."
	case runref.Failed:
		said = fmt.Sprintf("Marco could not finish %s.", named)
	case runref.Started:
		// A RECORD THAT ONLY SAYS IT BEGAN. Reachable: the run reserves its
		// reference and writes `started` before anything else, so a child killed
		// between the two leaves this.
		said = fmt.Sprintf("Marco started %s and never said how it ended.", named)
	default:
		// AN OUTCOME THIS DOES NOT KNOW, said as itself rather than swallowed. The
		// vocabulary is closed today and a surface that silently showed nothing for
		// a new word would hide exactly the run worth looking at.
		said = fmt.Sprintf("Marco ended %s: %s.", named, outcome)
	}
	if reason != "" {
		// THE RUN'S OWN SENTENCE, CARRIED AND NOT PARAPHRASED — and made to read
		// like one beside the sentence above it. The runner writes its reasons in
		// lower case because they usually follow a colon; here one follows a full
		// stop, and "worked. no words were read" reads as a transcription error
		// rather than as a finding. The words are untouched.
		said += " " + strings.ToUpper(reason[:1]) + reason[1:]
	}
	return said
}
