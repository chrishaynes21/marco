// Package runref is the run reference: the one line a supervised run prints about itself, and
// the seven words it can end in.
//
// # Why this is a package and not a format string
//
// The reference exists to be RETRIEVED — that is its whole job, and
// [[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]] is mostly about making it
// honest. It is printed by `director perform-workflow` and read by whatever asked for the run:
// a person pasting it into a report, and now a front end that spawned the runner and has to tell
// somebody what happened.
//
// Two processes, one line. Written twice it would be two formats and two vocabularies, and the
// drift would be silent in the worst possible place: a front end that recognised a slightly older
// shape would report every run as "I could not tell", which is indistinguishable from a run that
// could not tell. So the shape and the words live here, the runner prints through it, and the
// front end parses through it.
//
// # What it is not
//
// Not the record. The record is a file with counts and sentences in it and belongs to the command
// that writes it. This is the REFERENCE to that file: an id, where it went, and how it ended.
package runref

import (
	"fmt"
	"strings"
)

// Outcome is how a supervised run ended, in the ways a person asks about.
//
// Seven and not two. "It did not work" covers a refusal before anything was pressed, a press
// whose outcome contradicted the expectation, and a press nobody could confirm either way — and
// those send somebody to three different places. Cancellation is separate because a run somebody
// stopped is not a run that failed, a startup failure is separate because nothing was attempted,
// and `Started` is the one that says this command has a bug.
type Outcome string

const (
	// Cancelled: a person stopped it. Nothing is implied about the application.
	Cancelled Outcome = "cancelled"
	// Refused: Marco declined before sending input, or stopped without arriving.
	Refused Outcome = "refused"
	// Verified: an action was performed and its outcome was verified.
	Verified Outcome = "verified"
	// Unconfirmed: an action was performed and its outcome could not be decided.
	Unconfirmed Outcome = "unconfirmed"
	// Contradicted: an action was performed and the evidence disagreed with it.
	Contradicted Outcome = "contradicted"
	// Failed: the run could not start — no window, no artifact, a refused scope.
	Failed Outcome = "failed"
	// Started: reserved and not finalised. A record that still says this is an attempt
	// that left by a path which wrote no outcome — which is a defect in the command and
	// not a thing that happened to the application.
	Started Outcome = "started"
)

// Every is the whole vocabulary, so a consumer that sweeps it cannot quietly stop covering one.
var Every = []Outcome{Cancelled, Refused, Verified, Unconfirmed, Contradicted, Failed, Started}

// Known reports whether a word is one of the seven.
//
// Used by the parser, which is the reason it matters: a parenthesised word at the end of a line
// is the outcome only if it IS an outcome. Otherwise it is part of a path, or a sentence, or
// whatever else a future line carries, and a parser that assumed would invent outcomes.
func Known(word string) bool {
	for _, o := range Every {
		if string(o) == word {
			return true
		}
	}
	return false
}

// unwritten is what the line says instead of a path when the record could not be written.
//
// A reference to a record that does not exist is still worth printing — the id is in the frames
// directory's name — but it must not read like a path.
const unwritten = "(its record could not be written)"

// Cite is the line a person copies into a bug report, printed before anything happens.
//
// Leading spaces because it is printed inside an indented block; `path` empty means the record
// could not be written.
func Cite(id, path string) string {
	if path == "" {
		return fmt.Sprintf("  run %s   %s", id, unwritten)
	}
	return fmt.Sprintf("  run %s   %s", id, path)
}

// Ended is the same line with the outcome on it, printed when the run is over.
func Ended(id, path string, out Outcome) string {
	return fmt.Sprintf("%s   (%s)", Cite(id, path), out)
}

// Reference is one parsed reference line.
type Reference struct {
	ID   string
	Path string
	// Outcome is empty on the opening citation, which is printed before there is one.
	Outcome Outcome
}

// Parse reads a reference line back, and reports whether the line was one.
//
// # Why it is not Fields()
//
// A record's path is a filesystem path on somebody's own machine and may well have a space in
// it — `C:\Users\A Person\AppData\Roaming\marco\runs\…` is ordinary. So the id is taken up to
// the first space and everything after it is the path, with the outcome split off the end only
// when the parenthesised word at the end IS one of the seven.
func Parse(line string) (Reference, bool) {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "run ") {
		return Reference{}, false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(s, "run "))
	id := rest
	if i := strings.IndexAny(rest, " \t"); i >= 0 {
		id, rest = rest[:i], strings.TrimSpace(rest[i:])
	} else {
		rest = ""
	}
	if id == "" {
		return Reference{}, false
	}
	r := Reference{ID: id}
	// THE OUTCOME, IF THE LAST BRACKET HOLDS ONE.
	if strings.HasSuffix(rest, ")") {
		if i := strings.LastIndex(rest, "("); i >= 0 {
			if word := rest[i+1 : len(rest)-1]; Known(word) {
				r.Outcome = Outcome(word)
				rest = strings.TrimSpace(rest[:i])
			}
		}
	}
	if rest != unwritten {
		r.Path = rest
	}
	return r, true
}
