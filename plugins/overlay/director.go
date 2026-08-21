package main

import (
	"os"
	"strconv"
	"strings"
)

// What is left of the overlay's involvement in semantic desktop control, and its
// smallness is now the whole point.
//
// # What used to be here
//
// A second dispatcher. A spoken phrase ran `marco director "<phrase>"` from this file
// while a typed one ran `marco do "<phrase>"` from acts.go, so the same words meant
// different things depending on how they arrived — and the two paths shared nothing, so
// nothing could hold them level. Both now go through intake.go, and everything that made
// this a dispatcher went with them.
//
// Two properties that lived here are preserved on the way out, deliberately:
//
//   - Dispatch is asynchronous. A replay runs for minutes and a spoken "stop" arrives as
//     a second phrase while the first is still going; a blocked act handler could not hear
//     it. See dispatchIntake.
//   - A phrase is never silently dropped. Every phrase reaches a child and every child's
//     outcome reaches the history.
//
// And one bug went with them: while a spoken Director phrase ran, this file's bare
// exec.Command registered nothing, so `isRunning()` was false and the leader key opened
// the command line instead of cancelling. Routing everything through streamChild fixes it
// structurally — the same registration that makes a typed play cancellable now covers a
// spoken one, because there is only one spawn.

// directorEnabled reads the kill switch.
//
// MARCO_DIRECTOR=off no longer gates dispatch — the engine's intake decides whether a
// request is for Director, and a front end second-guessing that would be a second router.
// What it still gates is the watch panel (watch.go): with no Director expected, polling
// for a playbill is only a stream of failed dials.
func directorEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_DIRECTOR"))) {
	case "off", "0", "false", "no":
		return false
	}
	return true
}

// directorStatusLine turns one line of a running command into a short HUD status, and
// says whether the line was one it recognises.
//
// # Why it reports "not mine" rather than echoing anything
//
// Every child's output now flows through one streamer, including an ordinary play's find/
// click log. Statusing every line would make the status bar flicker through a play's
// internals; statusing only the Director's rendered events keeps it answering the question
// a person actually glances at it for, which is "what is it doing NOW".
//
// The Director's own wording is used verbatim wherever it exists. Re-describing its result
// here would mean maintaining a second vocabulary that could drift from the first.
//
// # What it must never say is DIRECTOR
//
// The two lines this file writes itself used to read "director: open the settings" and
// "director asked — answer it", and both of them land on the status line of an always-
// visible window. A person who has learned one play is not required to know that this
// product contains a thing called a Director, what it does, or why it is the one asking;
// the word explains nothing to them and it is the whole of what those two lines taught.
//
// The register is pkg/playbill's, because that is where the product's own account of what
// is happening already lives: MARCO HAS A QUESTION is the exact headline the Watch panel
// and the always-visible hint show for the same state (pkg/playbill/narrate.go), so the
// status line now agrees with the two surfaces beside it instead of naming a component.
// The question itself is carried through when the engine sent one — "answer it" without
// saying what is being asked is a demand, not information.
//
// Deleting the CLARIFICATION_REQUIRED arm must fail TestAQuestionShowsInTheStatusLine, and
// putting a backstage word back on either line must fail
// TestTheStatusLineNeverNamesTheDirector.
// questionStatus is the ONE spelling of "Marco is waiting on you" on the status line.
//
// It is a constant because the wording is load-bearing twice: this file writes it, and
// model.setStatus colours the panel by it — a question is the listening colour, never the
// error colour, because nothing has gone wrong. Those two used to agree by both containing
// the word "director", so rewording the line silently turned a question grey. A shared
// constant is the only version of that agreement a compiler can keep.
//
// The words are pkg/playbill's headline for the same state, so the status line, the Watch
// panel and the always-visible hint all say it the same way.
const questionStatus = "Marco has a question"

func directorStatusLine(line string) (string, bool) {
	s := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(s, "heard: "):
		return "heard " + strconv.Quote(strings.TrimPrefix(s, "heard: ")), true
	case strings.HasPrefix(s, "CLARIFICATION_REQUIRED"):
		q := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(s, "CLARIFICATION_REQUIRED"), ":"))
		if q == "" {
			return questionStatus + " — answer it", true
		}
		return questionStatus + " — " + q, true
	case isProgressLine(s):
		return s, true // "[3/5] iteration 3"
	}
	return "", false
}

// isProgressLine matches the Director's "[3/5] …" step counter, and nothing else that
// happens to start with a bracket.
func isProgressLine(s string) bool {
	if !strings.HasPrefix(s, "[") {
		return false
	}
	end := strings.IndexByte(s, ']')
	if end < 2 {
		return false
	}
	inner := s[1:end]
	if !strings.Contains(inner, "/") {
		return false
	}
	for _, r := range inner {
		if (r < '0' || r > '9') && r != '/' {
			return false
		}
	}
	return true
}
