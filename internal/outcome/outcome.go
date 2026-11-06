// Package outcome is the one vocabulary for what became of an invocation.
//
// # Seven words, and they are a closed set
//
// Marco reports what happened in exactly seven words. The set is small on purpose: a front end
// that can render seven things renders all of them, and a person who learns seven words has
// learned the whole vocabulary. Adding an eighth is a product decision, not a convenience.
//
//	performed    it was done, and — where a Play could be checked — checked
//	clarify      Marco asked something and is waiting to be told
//	refused      Marco declined, or was not permitted
//	unavailable  it was never delivered; NOTHING tried it
//	cancelled    somebody stopped it
//	unverified   Marco delivered it and could not confirm it arrived
//	failed       it was tried and it went wrong
//
// The distinctions that earn their place are `refused` versus `failed` — declining is not the same
// as breaking — `unavailable` versus both, because "nothing tried this" is the only outcome after
// which offering to learn the thing makes any sense, and `unverified` versus `failed`, which was
// the seventh and is the subject of [[ADR-135-a-delivered-action-is-not-a-failed-one]].
//
// # Why `unverified` was worth a seventh word
//
// Because the vocabulary could already say "I will not claim that happened" and had no way to say
// "it happened and I cannot prove it", so the two shared `failed` — and they call for opposite
// responses. `failed` invites a retry. Retrying a request Marco already delivered acts on the
// world a second time, and the actions worth verifying are exactly the ones worth not doing
// twice. Measured: `select Beta` put the click on the list item, the item reported itself
// selected, no independent evidence agreed, and Marco correctly declined `performed` — then told
// the person it had failed.
//
// # Why this is a package rather than a constant in cmd/marco
//
// Because it was in cmd/marco, and the overlay had its own copy.
//
// `cmd/marco` printed `[result] performed`. `plugins/overlay` matched the literal `"[result] "` and
// kept a parallel set of the same lowercase words to compare against. They agreed, and nothing made
// them agree: the two ship as SEPARATE GO MODULES, so either could have been edited alone and the
// build would have stayed green. The comment on the old constant said so out loud — "nothing would
// fail to compile if either drifted" — which is an accurate description of a duplicate, written
// beside the duplicate, in place of removing it.
//
// The control centre made it three. It never read the line at all: it spawned the child with
// `.Start()`, returned `{"ok":true}` before anything had happened, and rendered "running…" for
// ever. So one run could be reported three ways by three surfaces, and only one of them was
// looking at what actually occurred.
//
// One package, imported by all three, is the structural version of the promise. The overlay is a
// separate module but it may import the engine's internal packages, so nothing here costs it
// anything.
//
// # This is protocol, not user text
//
// [ResultPrefix] and the seven words go over a pipe between processes. They are matched, not read.
// The sentences a person actually sees are built from them by whichever surface is doing the
// telling — the HUD says "ran: open the test", the control centre says something else — and those
// sentences are presentation, which is why they are not here.
package outcome

import (
	"fmt"
	"io"
	"strings"
)

// Outcome is what became of one invocation.
type Outcome string

const (
	// Performed is the thing happening. For a Play whose arrival could be checked, it means
	// checked — not merely attempted.
	Performed Outcome = "performed"
	// Clarify is Marco having asked something and waiting to be told.
	Clarify Outcome = "clarify"
	// Refused is Marco declining, or not being permitted: the authority door said no, a guard
	// did not recognise the place, an edge would not verify.
	Refused Outcome = "refused"
	// Unavailable is the request never having been delivered. NOTHING tried it.
	Unavailable Outcome = "unavailable"
	// Cancelled is somebody having stopped it.
	Cancelled Outcome = "cancelled"
	// Unverified is the request having been DELIVERED, with its arrival unconfirmed.
	//
	// Not a failure and not a success: the attempt reached the target and the evidence afterwards
	// was not enough to say it took. A surface should say so plainly and must NOT invite a retry —
	// see the package note above.
	Unverified Outcome = "unverified"
	// Failed is it having been tried, and gone wrong.
	//
	// # A GUARD'S REFUSAL ARRIVES HERE, AND THAT IS A KNOWN LIMITATION
	//
	// A routine whose press could not be checked against the window it was demonstrated
	// in REFUSES rather than pressing — which is the guard working exactly as intended,
	// and is a decision rather than a malfunction. It reaches a person as `failed`,
	// because `cmd/marco` maps every execution error onto this word and the guard's
	// refusal is an execution error.
	//
	// So is a secret that could not be looked up, a routine that stopped at an action
	// which did not happen, and a key the route-end sweep could not release. Each of them
	// is "it having been tried, and gone wrong" in the sense this word is defined in, and
	// none of them is what a person means by `failed`.
	//
	// What carries the distinction is the REASON, and it is carried everywhere: the
	// sentence is printed on the run's own output, stored in the record's `because`, and
	// shown on the card. Those sentences say what refused and what it lacked — "the window
	// is a different size", "that secret is not set" — which is the information a person
	// acts on. The word is the coarse channel and the reason is the precise one.
	//
	// RECLASSIFYING IS A SEPARATE CHANGE and deliberately not made here. The seven words
	// are a closed set that every surface and every stored record is written against, and
	// a run that used to say `failed` saying `refused` changes what a front end does with
	// it, what an exit code means, and what every record already on disk means by
	// comparison. That is worth doing on purpose, with the surfaces in hand, and not as a
	// side effect of a release. See
	// [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].
	Failed Outcome = "failed"
)

// All is the closed set, in the order a person would meet them.
//
// Exported so a test can walk every word rather than naming three of them and trusting the rest,
// and so a surface can prove it renders all of them.
var All = []Outcome{Performed, Clarify, Refused, Unavailable, Cancelled, Unverified, Failed}

// Process exit codes.
//
// 0 and 3 keep the meanings they had before there was a vocabulary — 3 has always meant "never
// delivered, a caller may reasonably try something else", and the overlay already read it that way.
// The rest are distinct so that a front end which can see ONLY an exit code still cannot mistake a
// refusal for a success. That is the property worth having: the failure mode being guarded against
// is a surface reporting "ran" for something that did not.
const (
	ExitPerformed   = 0
	ExitFailed      = 1
	ExitUnavailable = 3
	ExitClarify     = 4
	ExitRefused     = 5
	ExitCancelled   = 6
	// ExitUnverified is numbered last because it was added last, and it is distinct from
	// ExitFailed for the same reason the word is: a caller reading only the code must not retry.
	ExitUnverified = 7
)

// Exit is the process code for an outcome.
func (o Outcome) Exit() int {
	switch o {
	case Performed:
		return ExitPerformed
	case Unavailable:
		return ExitUnavailable
	case Clarify:
		return ExitClarify
	case Refused:
		return ExitRefused
	case Cancelled:
		return ExitCancelled
	case Unverified:
		return ExitUnverified
	default:
		return ExitFailed
	}
}

// Valid reports whether s is one of the seven.
func Valid(s string) bool { _, ok := Parse(s); return ok }

// Parse reads one of the seven words. It is deliberately exact: no trimming of surrounding rubbish,
// no case folding, no prefix matching. A surface that received something else received something
// else, and guessing which word was meant is how a refusal becomes a success.
func Parse(s string) (Outcome, bool) {
	for _, o := range All {
		if string(o) == s {
			return o, true
		}
	}
	return "", false
}

// ResultPrefix is the wire line a front end reads to learn what happened.
//
// It sits beside `[route] `, which says WHICH Play a loose phrase became. Both are protocol
// between processes rather than anything a person is meant to read.
//
// Changing this literal must fail TestTheWireLineIsOneSharedLiteral.
const ResultPrefix = "[result] "

// Announce publishes an outcome for whatever is reading this process's stdout.
func Announce(w io.Writer, o Outcome) { fmt.Fprintf(w, "%s%s\n", ResultPrefix, o) }

// Line renders the wire line without writing it, for a test or a buffer.
func Line(o Outcome) string { return ResultPrefix + string(o) }

// FromLine reads a single line of a child's output and reports the outcome it announces.
//
// Returns false for every line that is not one — which is almost all of them, since a child's
// stdout also carries the play's own logging. A caller streams its child's output through this and
// keeps the last true answer.
//
// It requires the WHOLE line after the prefix to be one of the seven. A line that begins with the
// prefix and continues with something unrecognised is not a partial result to be salvaged; it is a
// disagreement between two builds, and the honest reading of it is "no result was announced".
func FromLine(line string) (Outcome, bool) {
	rest, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n"), ResultPrefix)
	if !ok {
		return "", false
	}
	return Parse(strings.TrimSpace(rest))
}

// RoutePrefix is the engine's other wire line: WHICH Play a loose phrase turned out to be.
//
// It is a LABEL, not a verdict — nothing derives behaviour from it — but it crosses the same
// module boundary [ResultPrefix] does, and it was written out by hand in three separate places:
// the engine that prints it, the HUD that reads it, and the control centre that reads it. Three
// copies of a literal that no compiler compares is the same arrangement this package was created
// to end, so it lives here too.
//
// Changing this literal must fail TestTheWireLineIsOneSharedLiteral.
const RoutePrefix = "[route] "

// RouteFromLine reads the play name off a `[route] ` line, if that is what this line is.
func RouteFromLine(line string) (string, bool) {
	rest, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n"), RoutePrefix)
	if !ok {
		return "", false
	}
	name := strings.TrimSpace(rest)
	return name, name != ""
}

// ProposedPrefix is the engine's third wire line: a SAVED ROUTINE Director's resolver proposed
// for words nothing matched exactly, offered for review instead of reading the words against the
// screen. What follows it is one JSON object, the same answer `marco resolve --json` prints. It
// carries proposed values, so a front end consumes it and does not echo it into a log.
//
// Changing this literal must fail TestTheWireLineIsOneSharedLiteral.
const ProposedPrefix = "[proposed] "

// RanPrefix is the engine's fourth wire line: WHAT a routine was run with, printed beside
// `[route] ` — its name and the values it was given, in the routine's own order, never a value the
// credential store answers. The route line is the name alone; a front end showing a person what
// ran ("cloud pull resource: stone") reads this one. It carries values, so a front end consumes it
// and does not echo it into a log.
//
// Changing this literal must fail TestTheWireLineIsOneSharedLiteral.
const RanPrefix = "[ran] "
