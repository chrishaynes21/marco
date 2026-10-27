package main

// AND A STOPPED RUN MAY NOT TAKE A HALF-FILLED FIELD AWAY WITH IT EITHER.
//
// The third device through the channel the chord slice built, and the first one that is not about
// something held down. A cancelled run discards its error on purpose — somebody pressed Escape,
// which is not a failure — so whatever is STILL TRUE afterwards has to be kept by the host in an
// account of its own and asked for separately. A typing stopped between two characters is exactly
// that shape: the field has part of a value in it, the step after it was written for the whole
// value, and the record said `cancelled` with nothing beside it.
//
// # Why it is a second account and not more of the first
//
// `NotReleased` means the computer has not finished doing something: a key is down, and every
// keystroke anybody types next arrives modified. Incomplete text is not that. Nothing is held,
// nothing afterwards is changed by it, and a person told their keyboard may be stuck would be
// told something untrue. So `PartlyTyped` is its own account with its own words, and this file is
// the command's half: that the sentence reaches the durable record and the run's own output while
// the outcome stays `cancelled`.
//
// The gesture itself — what was submitted, the uncertainty about one character, the release
// attempted after an accepted down, and a secret measured in no number at all — is held in
// `internal/oshost` by `TestAStoppedTypingIsKeptWhereACancelledRunCannotDiscardIt` and the cases
// beside it. The sentence comes from `oshost.PartlyTypedText`, the production constructor, rather
// than being retyped here.
//
// See [[ADR-261-typed-text-is-a-sequence-of-submissions]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/oshost"
)

// aHostStoppedMidText answers the way `oshost` does for a typing the stop caught between two
// characters: nothing is held down, and part of a value is in a field.
type aHostStoppedMidText struct {
	aHostStoppedMidChord
	// submitted is how far the typing got, and 0 with `total` 0 is the control: a stop
	// that caught the run between steps rather than inside a typing.
	submitted, total int
}

// NotReleased is EMPTY, and that is the point of this case.
//
// A typing is not a held key. If the only account a run had were this one, the choice would be
// between calling incomplete text a stuck key and saying nothing at all.
func (h *aHostStoppedMidText) NotReleased() string { return "" }

func (h *aHostStoppedMidText) PartlyTyped() string {
	if h.keys == 0 || h.total == 0 {
		return ""
	}
	return oshost.PartlyTypedText(h.submitted, h.total, false).Error()
}

// A RUN STOPPED MID-TEXT SAYS SO IN ITS RECORD, AND STILL READS AS STOPPED.
//
// Both halves, for the chord's reason: a record that called this `failed` would turn somebody
// pressing Escape into a routine that broke, and a record that said only `cancelled` is the
// defect.
func TestAStoppedRunRecordsTheTextItLeftHalfSubmitted(t *testing.T) {
	machine := &aHostStoppedMidText{submitted: 4, total: 11}
	out, m, said, raw := stoppedMidRunWith(t, &machine.aHostStoppedMidChord, machine)

	if out != OutcomeCancelled {
		t.Errorf("a run stopped mid-text reported %q; a stop is not a failure", out)
	}
	if m["outcome"] != string(OutcomeCancelled) {
		t.Errorf("the record says the run was %v, want cancelled:\n%s", m["outcome"], raw)
	}
	because, _ := m["because"].(string)
	switch {
	case because == "":
		t.Errorf("the record of a run that left part of a value in a field says only "+
			"%q, with nothing beside it — so the one account anybody reads "+
			"afterwards does not mention the field:\n%s", m["outcome"], raw)
	case !strings.Contains(because, "4 of 11"):
		t.Errorf("the record says %q and does not say how much went in", because)
	case !strings.Contains(because, "incomplete"):
		t.Errorf("the record says %q and does not say the field is unfinished, "+
			"which is the part somebody has to act on", because)
	}
	// AND IT IS SAID, not only written down. The window keeps the last thing the child
	// said before the outcome word, so a warning that reached the record and nothing
	// else would never reach the person standing there.
	if !strings.Contains(said, "4 of 11") {
		t.Errorf("the run said nothing about the text it left half submitted, so "+
			"nothing reading its output could show it:\n%s", said)
	}
	// AND IT IS NOT DESCRIBED AS A KEY LEFT DOWN. The two accounts are separate so that
	// each can be reported in its own words, and this is the half of that worth
	// asserting: a person whose field is half full must not be told their keyboard is
	// stuck.
	if strings.Contains(because, "held down") || strings.Contains(said, "held down") {
		t.Errorf("incomplete text is reported as something still held down: %q", because)
	}
}

// AND A STOP THAT CAUGHT NOTHING MID-TEXT ADDS NOTHING TO THE RECORD.
//
// The control, and the one that keeps the case above from being satisfied by a record that
// complains after every stop. Somebody who presses Escape between two steps must not be told a
// field is half full.
func TestAStoppedRunThatTypedNothingRecordsOnlyThat(t *testing.T) {
	machine := &aHostStoppedMidText{}
	out, m, said, raw := stoppedMidRunWith(t, &machine.aHostStoppedMidChord, machine)

	if out != OutcomeCancelled {
		t.Errorf("a stopped run reported %q", out)
	}
	if because, _ := m["because"].(string); because != "" {
		t.Errorf("a stop that caught no typing recorded %q beside it:\n%s", because, raw)
	}
	if strings.Contains(said, "incomplete") {
		t.Errorf("a clean stop told somebody a field was left incomplete:\n%s", said)
	}
}

// AND A SECRET SAYS NO NUMBER IN THE RECORD EITHER.
//
// The suppression has to hold at every layer it travels through, and this is the layer that
// writes a durable file somebody else can read later. A length is a fact about a password and a
// failure position is a prefix whose size somebody now knows.
func TestAStoppedSecretRecordsNoNumber(t *testing.T) {
	machine := &aHostStoppedMidSecret{}
	out, m, said, raw := stoppedMidRunWith(t, &machine.aHostStoppedMidChord, machine)

	if out != OutcomeCancelled {
		t.Errorf("a run stopped mid-secret reported %q", out)
	}
	because, _ := m["because"].(string)
	if because == "" {
		t.Fatalf("the record of a run that left part of a secret in a field says "+
			"nothing beside %q:\n%s", m["outcome"], raw)
	}
	if !strings.Contains(because, "secret") || !strings.Contains(because, "incomplete") {
		t.Errorf("the record says %q and does not say an unfinished secret is what "+
			"happened", because)
	}
	if strings.ContainsAny(because, "0123456789") {
		t.Errorf("the record says %q, and a number about a secret is a fact about "+
			"the value", because)
	}
	if strings.ContainsAny(firstLineAbout(said, "secret"), "0123456789") {
		t.Errorf("the run's own output says a number about a secret:\n%s", said)
	}
}

// aHostStoppedMidSecret is the same stop, inside a secret rather than a literal.
type aHostStoppedMidSecret struct{ aHostStoppedMidChord }

func (h *aHostStoppedMidSecret) NotReleased() string { return "" }

func (h *aHostStoppedMidSecret) PartlyTyped() string {
	if h.keys == 0 {
		return ""
	}
	return oshost.PartlyTypedSecret(false).Error()
}

// firstLineAbout is the line of output that mentions `what`, or "".
//
// The run's own output carries the record's JSON as well as its narrative, and the JSON holds a
// timestamp and a reference full of digits — so a test about digits has to read the sentence
// rather than the whole stream.
func firstLineAbout(said, what string) string {
	for _, line := range strings.Split(said, "\n") {
		if strings.Contains(line, what) && !strings.Contains(line, "{") {
			return line
		}
	}
	return ""
}
