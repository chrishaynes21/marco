package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// doAsProduct is `marco do` as the product runs it.
//
// Every test that used to call `dispatchDo` calls this instead, and the difference is the whole
// point of the "prove wiring by deleting it" rule: `dispatchDo` stopped being what `marco do`
// calls, and a suite full of tests saying it was would have gone on passing about a function
// nothing entered. This enters through `runInvocation`, which `runAssistantDo` also enters.
//
// It stubs only the ONE thing a unit test must not really do: ask the developer's own running
// Director whether it is mid-question. Everything else is production code.
func doAsProduct(t *testing.T, d orchestrator.Deps, text string,
	named map[string]string, positional []string) (Outcome, error) {

	t.Helper()
	noDirector(t)
	return runInvocation(d, invoke.Request{
		Text: text, Source: invoke.SourceCLI,
		Named: named, Positional: positional,
	})
}

// noPendingQuestion keeps a test off the real socket.
//
// `pendingQuestion` dials the Director without starting it. In a test that is both slow and
// wrong: it would read $MARCO_HOME from the developer's machine and could reach a live service.
func noPendingQuestion(t *testing.T) {
	t.Helper()
	prev := pendingQuestion
	pendingQuestion = func() bool { return false }
	t.Cleanup(func() { pendingQuestion = prev })
}

// noDirector keeps a test off the real service entirely.
//
// Without it a unit test dials — and AUTO-STARTS — the Director on whatever machine runs the
// suite. That is slow, flaky, and capable of driving a real mouse. Observed while building this
// phase: a test that merely asked for an unknown command reached a live service and got a real
// verdict back, because the intake's Director fallback is production code and does what it says.
//
// Substituting is not weakening. `submitPhrase` and `stopWhatIsRunning` are the two places the
// intake talks to the service, and a test that wants to prove ROUTING does not want to prove the
// socket as well — the socket has its own tests.
func noDirector(t *testing.T) {
	t.Helper()
	noPendingQuestion(t)
	prevSubmit, prevStop := submitPhrase, stopWhatIsRunning
	submitPhrase = func(string, bool) int { return exitUnavailable }
	stopWhatIsRunning = func(bool) int { return exitUnavailable }
	t.Cleanup(func() { submitPhrase, stopWhatIsRunning = prevSubmit, prevStop })
}
