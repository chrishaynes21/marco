package service

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Terminal rendering.
//
// A command that ends must SAY something. The failure this file exists to make
// impossible was real and user-visible: an ambiguous outcome fell through every branch
// with an empty state and an empty message, and the CLI rendered it as a bare
//
//	:
//
// which tells the user nothing at all — not what happened, not whether the desktop
// changed, not what to do next. An empty explanation is worse than a wrong one,
// because a wrong one can at least be argued with.
//
// So there is ONE function that produces a terminal reason, it switches exhaustively,
// and it has no silent default. TestEveryTerminalStateRendersSomething enumerates the
// states, because Go cannot enforce exhaustiveness for a string-typed enum.

// TerminalStates are every state a command can finish in.
//
// Listed here rather than derived, so the test that enumerates them fails loudly when
// a new state is added without a reason to go with it.
var TerminalStates = []CommandState{
	CommandCompleted,
	CommandUnverified,
	CommandFailed,
	CommandCancelled,
	CommandBlocked,
	CommandTimedOut,
	CommandInternalError,
}

// TerminalReason renders a non-empty explanation for a finished command.
//
// The outcome's own message wins when it has one — it was written by the code that
// knows what happened. This supplies the words when it did not, which is exactly the
// case that used to render as nothing.
func TerminalReason(state CommandState, out execute.Outcome) string {
	if msg := strings.TrimSpace(out.Message); msg != "" {
		return msg
	}

	// A timeout names its phase. "Timed out" without one is the unnamed-phase problem
	// in a different disguise, and the phase is the whole diagnostic value.
	if to, ok := trace.Timeout(errOf(out)); ok {
		return to.Error()
	}

	switch state {
	case CommandCompleted:
		return "done — the action was performed and verified"
	case CommandUnverified:
		return "the action was performed but could not be confirmed; it is NOT being repeated, " +
			"because repeating it could apply it twice"
	case CommandFailed:
		if out.Error != "" {
			return out.Error
		}
		return failureReason(out)
	case CommandCancelled:
		return "cancelled — it stopped at the next safe point"
	case CommandBlocked:
		return blockedReason(out)
	case CommandTimedOut:
		// The phase name lives in the error text, produced by ErrPhaseTimeout.Error().
		// By the time it reaches here it has crossed a struct boundary as a string, so
		// errors.As cannot recover the type — the text is the carrier.
		if out.Error != "" {
			return out.Error + "; the action may or may not have taken effect, " +
				"so it is not being repeated"
		}
		return "timed out in an unnamed phase, which is itself a bug; the action may or " +
			"may not have taken effect, so it is not being repeated"
	case CommandInternalError:
		return "internal: the Director reached a state it does not have an explanation for"
	}
	// Not a silent default. An unrecognised state is a bug in this file, and saying so
	// is more useful than inventing a plausible sentence for it.
	return fmt.Sprintf("internal: no terminal explanation is defined for state %q", state)
}

// failureReason explains a failure that carried no message of its own.
func failureReason(out execute.Outcome) string {
	if out.Resolution != nil {
		if err := out.Resolution.Consistent(); err != nil {
			return "internal: " + err.Error()
		}
		if out.Resolution.Explanation != "" {
			return out.Resolution.Explanation
		}
		switch out.Resolution.Status {
		case directorapi.ResolutionAbsent:
			return "nothing matching that is present in the observed window"
		case directorapi.ResolutionUnobservable:
			return "the application could not be observed well enough to answer, " +
				"which is not evidence that the target is absent"
		}
	}
	return "the request could not be completed, and no more specific reason was recorded"
}

// blockedReason explains a command waiting on the user.
func blockedReason(out execute.Outcome) string {
	if out.Resolution != nil {
		// An AMBIGUOUS resolution that produced no question is an invariant violation,
		// not a user-facing condition. Named as such so the regression is visible
		// rather than dressed up as advice to rephrase.
		if err := out.Resolution.Consistent(); err != nil {
			return "internal: " + err.Error()
		}
		if n := out.Resolution.ContenderCount(); n >= directorapi.MinContenders {
			return fmt.Sprintf("waiting for an answer — %d controls match about equally well", n)
		}
	}
	return "waiting for an answer"
}

// errOf recovers an error value from an outcome, for timeout classification.
func errOf(out execute.Outcome) error {
	if out.Error == "" {
		return nil
	}
	return fmt.Errorf("%s", out.Error)
}
