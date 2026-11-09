package main

// RECORDING A ROUTINE AGAIN, FROM THE ROUTINE.
//
// # What this is for
//
// Re-recording is what somebody testing a routine does all afternoon. The menu moved, a press
// landed wrong, the pauses were too tight — so you do it again. Until now that meant reading the
// name off the card, typing it back into the box at the top of the page exactly, and remembering
// which application the first one was saved for. Get the name wrong by a space and you have two
// routines; get the application wrong and you have two more.
//
// So the routine itself offers it. The name comes from the routine, the application is remembered,
// and the review says which routine this is a second recording of.
//
// # And it carries no authority whatsoever
//
// This is the whole of the design. `again` is a LABEL, not a licence. Nothing here can replace
// anything: the save goes through the same `keep` every other recording goes through, which asks
// what is at the target, refuses a collision, and offers *Save as* or *Replace that routine by
// name*. Pressing Save after Record again does exactly what pressing Save does after any other
// recording — it refuses, and names what is there.
//
// That is not caution for its own sake. [[ADR-253-the-claim-is-shared-and-the-record-is-not]]'s
// predecessor exists because `Registry.Save` is a plain file write, so a recording kept under a
// name already in the library replaced what was there — the source, and the demonstration beside
// it, with nothing asked and nothing said. The likeliest casualty was the routine somebody was
// comparing against. A *Record again* button that then replaced on Save would have reintroduced
// exactly that loss through a door marked convenience, and the person pressing it would have had
// more reason than anybody to expect it.
//
// # The one thing it does notice
//
// Whether the demonstration was made in the same application. The identity a save writes is built
// from the application that was OBSERVED, so a re-recording made in a different application is a
// different route — no collision, no refusal, and a second routine quietly saved beside the one
// somebody set out to replace. Nothing is lost and nothing is wrong, and it is not what they
// asked for, so the review says so.
//
// See [[ADR-256-recording-a-routine-again]].

import (
	"fmt"
	"strings"
)

// reRecording is the saved routine a recording was started from.
//
// Three facts and no behaviour. The identity is what a replacement would have to name, the name is
// what the window shows and what the recording is called, and the application is what makes a
// mismatch noticeable at review.
type reRecording struct {
	// id is the route's whole identity — `<app>/<scope>/<slug>`. The identity rather
	// than the name, because `settings` can be saved for two applications under two
	// scopes and a window holding the word would offer to replace whichever it found
	// first. See Routine.ID.
	id string
	// name is what that routine calls itself, which is what the new recording is called.
	name string
	// app is the application it is saved for, or "" for a routine that is about no
	// application. Kept to compare against what the new demonstration is observed in.
	app string
	// scope is where it works, in the product's own three words — the same vocabulary
	// the review's select offers.
	//
	// Remembered so the review opens on the scope the routine already has, rather than
	// on the first option in a list. A re-recording silently rescoped is a routine that
	// works somewhere else than the one it replaced, which is the kind of change nobody
	// notices until it refuses every press.
	scope string
}

// RecordAgain starts a second recording of one saved routine.
//
// The identity is passed in and looked up HERE, now, rather than taken on trust: a routine can be
// deleted between the page drawing a card and somebody pressing the button on it, and a recording
// started as a second version of something that is gone would be a window promising a replacement
// it could not offer.
func (a *Acts) RecordAgain(id, window string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "say which play to record again"
	}
	// THE ROUTINE AS IT IS RIGHT NOW, through the listing the page chose from, so what
	// is remembered is one answer about one route and not a second derivation of it.
	//
	// ONE LISTING, AND THE MATCH ITSELF. This took the index out of the listing it had
	// matched in and applied it to a SECOND `a.Routines()` — which reads the library
	// again. A routine added, removed or renamed between the two reads moves everything
	// after it, so the index that had identified the right route identified a namesake
	// one position along, or nothing at all: a recording started under another routine's
	// name and application, or a panic out of the end of a shorter slice. The same shape
	// as PlayRoutine's own finding — a check that is correct about the wrong subject —
	// and the same remedy, which is to keep what was matched.
	var found *Routine
	for _, r := range a.Routines() {
		if r.ID == id {
			got := r
			found = &got
			break
		}
	}
	if found == nil {
		return fmt.Sprintf("there is no play %q any more — it has gone since "+
			"this page was drawn. Choose one from the list below.", id)
	}
	if strings.TrimSpace(found.Name) == "" {
		// A ROUTINE WHOSE NAME CANNOT BE READ. `NameIn` falls back to the slug with
		// its dashes read as spaces, so this is a file whose header is gone AND
		// whose slug is empty — which is not a thing a recording can be called.
		return "that play does not say what it is called, so a new recording " +
			"cannot be named after it. Record it under a name of your own."
	}
	return a.beginRecording(found.Name, window, &reRecording{
		id: found.ID, name: found.Name, app: found.App, scope: found.Scope,
	})
}

// againElsewhere is the sentence for a re-recording made in a different application, or "".
//
// Compared only when BOTH are named. A routine saved for no application is about no application,
// and a demonstration in which no application was observed has nothing to compare — in neither
// case is there a mismatch to report, and inventing one would put a warning on the ordinary case.
func againElsewhere(g *reRecording, observed string) string {
	if g == nil || g.app == "" || observed == "" || strings.EqualFold(g.app, observed) {
		return ""
	}
	return fmt.Sprintf("This was recorded in %s, and %q is saved for %s. Saving it "+
		"will make a SECOND play rather than a new version of that one, and "+
		"nothing will refuse it: which play a save writes is decided by the "+
		"application it was recorded in. Record it again in %s, or keep it under "+
		"a name of its own.", observed, g.name, g.app, g.app)
}
