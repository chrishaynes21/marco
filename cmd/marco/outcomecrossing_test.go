package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
)

// TestTheDirectorsVerdictSurvivesTheCrossing is the property the whole crossing exists for.
//
// The Director distinguishes nine states a command can be in. `marco do` told a person two of
// them. Everything else — delivered-but-unconfirmed, cancelled, blocked by policy, and the
// question Marco had just asked — arrived as the single word `failed`.
//
// It is a round trip on purpose. `directorSubmit` returns a process exit code, and
// outcomeOfDirectorExit reads that code back into the word a person sees, so the two halves can
// only be checked together: a state that maps to a truthful code and then back to the wrong word
// is still a lie, and so is a state whose code collides with another state's.
func TestTheDirectorsVerdictSurvivesTheCrossing(t *testing.T) {
	for _, tc := range []struct {
		state service.CommandState
		want  outcome.Outcome
		why   string
	}{
		{service.CommandCompleted, outcome.Performed,
			"it ran and arrival was verified"},
		{service.CommandUnverified, outcome.Unverified,
			"Marco delivered it and could not confirm it arrived — NOT that it went wrong"},
		{service.CommandCancelled, outcome.Cancelled,
			"somebody stopped it; nothing went wrong"},
		{service.CommandBlocked, outcome.Refused,
			"a door said no, which is not a thing breaking"},
		{service.CommandFailed, outcome.Failed,
			"it was tried and it went wrong"},
		{service.CommandTimedOut, outcome.Failed,
			"it was tried and never ended"},
		{service.CommandInternalError, outcome.Failed,
			"Marco broke"},
	} {
		t.Run(string(tc.state), func(t *testing.T) {
			code := exitOfCommandState(tc.state)
			if got := outcomeOfDirectorExit(code); got != tc.want {
				t.Errorf("the Director said %q and a person is told %q; want %q\nbecause %s",
					tc.state, got, tc.want, tc.why)
			}
		})
	}
}

// TestNoTwoVerdictsShareAnExitCode is why the round trip above can be trusted.
//
// Without it every case could pass by accident: map two states to the same code and the inverse
// has to guess, and the guess is right for one of them.
func TestNoTwoVerdictsShareAnExitCode(t *testing.T) {
	// The three that genuinely ARE one word to a person. Everything else must be distinct.
	sameWord := map[service.CommandState]bool{
		service.CommandTimedOut: true, service.CommandInternalError: true,
	}
	seen := map[int]service.CommandState{}
	for _, s := range []service.CommandState{
		service.CommandCompleted, service.CommandUnverified, service.CommandCancelled,
		service.CommandBlocked, service.CommandFailed,
	} {
		code := exitOfCommandState(s)
		if prev, dup := seen[code]; dup && !sameWord[s] {
			t.Errorf("%q and %q both exit %d, so a front end that sees only the code cannot tell them apart",
				prev, s, code)
		}
		seen[code] = s
	}
}

// TestADeliveredActionIsNeverReportedAsFailed is the safety half of the seventh word.
//
// Measured live: `select Beta` reached the list item, the click landed, and no independent
// evidence confirmed the application had taken it. Marco rightly declined to say `performed`.
// Saying `failed` instead is the other false claim — and it is the dangerous one, because the
// obvious response to "that failed" is to do it again, and doing a delivered action again is a
// second real effect in the world.
func TestADeliveredActionIsNeverReportedAsFailed(t *testing.T) {
	got := outcomeOfDirectorExit(exitOfCommandState(service.CommandUnverified))
	if got == outcome.Failed {
		t.Fatal("an action Marco delivered but could not confirm reads as `failed`, " +
			"which invites a retry that would act on the world a second time")
	}
	if got == outcome.Performed {
		t.Fatal("an unconfirmed action reads as `performed`, which is the claim it exists to refuse")
	}
}

// TestAQuestionIsNotAFailure covers the clarification path.
//
// Marco asks "which Save did you mean?", prints the candidates, and then announces `failed` —
// about a request it is still waiting to be told about. `clarify` is in the vocabulary for
// exactly this and nothing reached it.
func TestAQuestionIsNotAFailure(t *testing.T) {
	if got := outcomeOfDirectorExit(outcome.ExitClarify); got != outcome.Clarify {
		t.Fatalf("a pending question reads as %q; want %q", got, outcome.Clarify)
	}
}

// TestTheVocabularyHasOneDefinition is the duplicate this package's existence was meant to end.
//
// internal/outcome says so out loud — "One package, imported by all three" — and the overlay's
// own note records what was still outstanding: "What is STILL hand-matched is the PRODUCER:
// cmd/marco/intake.go". So the engine that PRINTS the word kept its own copy of the set, in the
// one place a drift would be invisible, because nothing compared the two.
func TestTheVocabularyHasOneDefinition(t *testing.T) {
	for _, o := range outcome.All {
		if _, ok := outcome.Parse(string(o)); !ok {
			t.Fatalf("%q is in All and Parse refuses it", o)
		}
	}
	// Announce and Exit must come from the shared set, not from a local redefinition: if
	// cmd/marco still had its own type these would be different values of different types and
	// this assignment would not compile.
	var _ outcome.Outcome = OutcomePerformed
	var _ outcome.Outcome = OutcomeUnverified
	if OutcomeFailed.Exit() == OutcomeUnverified.Exit() {
		t.Fatal("`failed` and `unverified` share an exit code")
	}
}
