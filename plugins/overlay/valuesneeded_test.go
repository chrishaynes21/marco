package main

// THE OVERLAY'S COMMAND LINE CANNOT REVIEW A PROPOSAL, AND THE EXPLANATION REACHES THE HUD.
//
// This surface is the one the voice path arrives through as well: a transcript becomes a phrase in
// the same box, and the same child is spawned. So what the overlay shows for a routine that needs
// text and was given none is what somebody speaking gets too.
//
// The explanation itself is the engine's, from `cmd/marco`'s one door — there is no second copy of
// the rule up here, which is the point. What this file establishes is that the sentence is not
// swallowed on the way: a refusal the engine prints and the overlay drops is a surface going
// quiet, which is the behaviour R8 exists to prevent.
//
// Through the PRODUCTION streamer and a real child, as next door: `MARCO_BIN` points at this test
// binary, because on this surface running marco.exe would perform real input.
//
// See [[ADR-277-a-request-may-propose-the-text-a-routine-types]].

import (
	"strings"
	"testing"
)

// theEngineRefusal is what `marco do` prints for a routine that needs text, as it prints it.
//
// Copied rather than imported: this is a separate module and cannot reach `cmd/marco`. A copy that
// drifts is the known cost, and `TestTheEngineStillSaysWhatThisExpects` in `cmd/marco` is what
// holds the two together — it asserts the same sentence against the real door.
const theEngineRefusal = `"write a note": this play types text that has to be supplied, ` +
	`and no value was given for "message". This request needs text values reviewed before ` +
	`running. Open it in Studio, or supply the value with the request` + "\n"

// THE EXPLANATION IS SHOWN, WHOLE, AND NOTHING IS SWALLOWED.
func TestTheHudShowsWhereToReviewAValueItCannotAskFor(t *testing.T) {
	h := newModel()
	if r := runEchoChild(t, h, theEngineRefusal); r.err != nil {
		t.Fatalf("the child failed: %v", r.err)
	}
	lines := strings.Join(said(h), "\n")
	for _, want := range []string{
		"This request needs text values reviewed before running",
		"Open it in Studio",
		"supply the value with the request",
		// AND WHICH ARGUMENT, because "it needs text" without a name is not
		// actionable either.
		`"message"`,
	} {
		if !strings.Contains(lines, want) {
			t.Errorf("the HUD does not carry %q:\n%s", want, lines)
		}
	}
}

// AND AN ORDINARY RUN GAINS NOTHING.
//
// The control. A HUD that printed the review clause beside every result would be telling somebody
// to go and check text for routines that take none — which is most of them.
func TestTheHudSaysNothingAboutReviewingAnOrdinaryRun(t *testing.T) {
	h := newModel()
	const ordinary = "[route] open settings\n[result] performed\n"
	if r := runEchoChild(t, h, ordinary); r.err != nil {
		t.Fatalf("the child failed: %v", r.err)
	}
	lines := strings.Join(said(h), "\n")
	for _, never := range []string{"reviewed before running", "Open it in Studio"} {
		if strings.Contains(lines, never) {
			t.Errorf("an ordinary run was told to review text (%q):\n%s", never, lines)
		}
	}
}
