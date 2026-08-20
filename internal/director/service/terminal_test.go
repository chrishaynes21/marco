package service

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The empty-terminal-output bug, as tests.
//
// A user ran an ambiguous request and the CLI printed a bare ":" — no state, no
// message, nothing. It happened because an outcome fell through every branch with an
// empty message and nothing downstream supplied one. These tests make that
// unreachable by construction rather than by care.

func TestEveryTerminalStateRendersSomething(t *testing.T) {
	// Go cannot enforce exhaustiveness for a string-typed enum, so the enumeration
	// lives in TerminalStates and this test walks it. Adding a state without a reason
	// fails here rather than in front of a user.
	for _, state := range TerminalStates {
		t.Run(string(state), func(t *testing.T) {
			for _, out := range []execute.Outcome{
				{},                                      // nothing at all
				{Status: directorapi.ResultFailed},      // a status and no message
				{Error: "something went wrong"},         // an error and no message
				{Resolution: &directorapi.Resolution{}}, // an empty resolution
			} {
				got := TerminalReason(state, out)
				if strings.TrimSpace(got) == "" {
					t.Fatalf("state %q with outcome %+v rendered nothing", state, out)
				}
				if got == ":" {
					t.Fatalf("state %q rendered the bare colon this test exists to prevent", state)
				}
			}
		})
	}
}

func TestAnUnknownTerminalStateSaysSoExplicitly(t *testing.T) {
	// Not a silent default. A state nobody wrote a reason for is a bug in this file,
	// and inventing a plausible sentence for it would hide that.
	got := TerminalReason(CommandState("reticulating"), execute.Outcome{})
	if !strings.Contains(got, "internal") {
		t.Fatalf("reason = %q, want it to name an internal problem", got)
	}
	if !strings.Contains(got, "reticulating") {
		t.Fatalf("reason = %q, want it to name the unrecognised state", got)
	}
}

func TestTheOutcomesOwnMessageWins(t *testing.T) {
	// The code that knows what happened wrote it. This function supplies words only
	// when there are none.
	out := execute.Outcome{Message: "the control now contains \"hello\""}
	if got := TerminalReason(CommandCompleted, out); got != out.Message {
		t.Fatalf("reason = %q, want the outcome's own message", got)
	}
}

func TestAnAmbiguousOutcomeWithNoQuestionIsReportedAsInternal(t *testing.T) {
	// The invariant violation that produced the bare ":". An AMBIGUOUS resolution
	// carrying fewer than two contenders cannot be turned into a question, and saying
	// "rephrase it" would hide a Director bug behind advice to the user.
	out := execute.Outcome{
		Resolution: &directorapi.Resolution{
			Status:     directorapi.ResolutionAmbiguous,
			Candidates: []directorapi.TargetCandidate{{Label: "only one"}},
		},
	}
	got := TerminalReason(CommandBlocked, out)
	if !strings.Contains(got, "internal") {
		t.Fatalf("reason = %q, want an internal error naming the broken promise", got)
	}
	if !strings.Contains(got, "AMBIGUOUS") {
		t.Fatalf("reason = %q, want it to name the status that broke its promise", got)
	}
}

func TestATimeoutReasonNamesItsPhase(t *testing.T) {
	// "Timed out" without a phase is the unnamed-phase problem wearing a different
	// hat. The phase is the whole diagnostic value.
	out := execute.Outcome{Error: "TIMED_OUT during observe after 15s (deadline 15s)"}
	got := TerminalReason(CommandTimedOut, out)
	if !strings.Contains(got, "observe") {
		t.Fatalf("reason = %q, want it to name the phase", got)
	}
}

func TestAnUnverifiedOutcomeSaysItIsNotBeingRepeated(t *testing.T) {
	// Failure to receive confirmation is not proof the action did not happen. The
	// wording has to carry that, or a reader will assume it is safe to try again.
	got := TerminalReason(CommandUnverified, execute.Outcome{})
	if !strings.Contains(strings.ToLower(got), "not being repeated") {
		t.Fatalf("reason = %q, want it to say the action is not being repeated", got)
	}
}
