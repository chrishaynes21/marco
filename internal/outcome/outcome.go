// Package outcome is the one vocabulary for what became of an invocation.
//
// # Six words, and they are a closed set
//
// Marco reports what happened in exactly six words. The set is small on purpose: a front end that
// can render six things renders all of them, and a person who learns six words has learned the
// whole vocabulary. Adding a seventh is a product decision, not a convenience.
//
//	performed    it was done, and — where a Play could be checked — checked
//	clarify      Marco asked something and is waiting to be told
//	refused      Marco declined, or was not permitted
//	unavailable  it was never delivered; NOTHING tried it
//	cancelled    somebody stopped it
//	failed       it was tried and it went wrong
//
// The distinctions that earn their place are `refused` versus `failed` — declining is not the same
// as breaking — and `unavailable` versus both, because "nothing tried this" is the only outcome
// after which offering to learn the thing makes any sense.
//
// # Why this is a package rather than a constant in cmd/marco
//
// Because it was in cmd/marco, and the overlay had its own copy.
//
// `cmd/marco` printed `[result] performed`. `plugins/overlay` matched the literal `"[result] "` and
// kept a parallel set of six lowercase words to compare against. They agreed, and nothing made
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
// [ResultPrefix] and the six words go over a pipe between processes. They are matched, not read.
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
	// Failed is it having been tried, and gone wrong.
	Failed Outcome = "failed"
)

// All is the closed set, in the order a person would meet them.
//
// Exported so a test can walk every word rather than naming three of them and trusting the rest,
// and so a surface can prove it renders all six.
var All = []Outcome{Performed, Clarify, Refused, Unavailable, Cancelled, Failed}

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
	default:
		return ExitFailed
	}
}

// Valid reports whether s is one of the six.
func Valid(s string) bool { _, ok := Parse(s); return ok }

// Parse reads one of the six words. It is deliberately exact: no trimming of surrounding rubbish,
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
// It requires the WHOLE line after the prefix to be one of the six. A line that begins with the
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
