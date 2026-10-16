package main

// `director try-it-out` — the live demonstration, operated rather than typed.
//
// # Why this exists when `demonstrate-surface` already does the work
//
// Because handing somebody six commands to paste is not the product. The owner's correction, in
// their words:
//
//	Do not hand me terminal commands and ask me to execute them. You are responsible for
//	operating the test harness in my local development environment. … The goal is for you to
//	test Marco's behavior, not operate Marco's development infrastructure.
//
// So this is a FRONT END and deliberately nothing else. Every phase delegates to the function
// `demonstrate-surface` already calls, in the same order, behind the same guard: there is no
// second harness here, and no copy of anything that could drift from the first. What it adds is
// the two things an operator needs and a person typing did not.
//
// THE SANDBOX, established by the command rather than by an environment variable somebody has to
// remember to set. The same `resetGuard`, the same disposable store, one less thing to get wrong
// and nothing a person has to be told.
//
// ONE CHECKPOINT LINE PER PHASE, in a closed vocabulary, at the start of its own line. An
// operator that reads prose to decide what happened is an operator that will eventually read it
// wrong. The checkpoint is derived from the exit code the phase already returns — it restates
// nothing and can therefore disagree with nothing.
//
// # What it does not do
//
// It does not decide to run. Each phase is a separate invocation, and the two that involve a
// person are gated exactly as they were: the demonstration phase is started only once somebody
// has said they are ready, and the one live click still needs `-authorise-one-click` on its own
// command line. Nothing here loosens that, nothing here retries, and the three facts a
// demonstration establishes are still reported apart by the phase that establishes them.
//
// Enforced by TestTryItOutPressesNothingWithoutItsOwnAuthorisation and
// TestEveryRefusalEndsInExactlyOneCheckpoint.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// The closed vocabulary of checkpoints.
//
// The owner's names, unchanged, because the whole value of a checkpoint is that the operator and
// the harness mean the same thing by it:
//
//	READY → OBSERVING → DEMONSTRATION NEEDED → LEARNED → READY TO EXECUTE → RESULT
//
// BLOCKED is the seventh and is what any of them becomes when the phase it belongs to refused.
// RESULT is the one that prints either way: a click that did not do what was hoped still has a
// result, and calling that BLOCKED would be hiding an outcome behind a refusal.
const (
	checkReady       = "READY"
	checkObserving   = "OBSERVING"
	checkDemoNeeded  = "DEMONSTRATION NEEDED"
	checkLearned     = "LEARNED"
	checkReadyToExec = "READY TO EXECUTE"
	checkResult      = "RESULT"
	checkCleaned     = "CLEANED"
	checkBlocked     = "BLOCKED"
)

// checkpoint prints one checkpoint, on its own line, with nothing after it on that line.
func checkpoint(out io.Writer, name string) {
	fmt.Fprintf(out, "\nCHECKPOINT: %s\n", name)
}

// atCheckpoint turns a phase's exit code into its checkpoint.
//
// The phase decides; this only names the decision. A phase that returns zero reached the
// checkpoint it was for, and anything else is BLOCKED — which is why no caller here inspects
// output to work out what happened.
func atCheckpoint(out io.Writer, code int, reached string) int {
	if code == 0 {
		checkpoint(out, reached)
	} else {
		checkpoint(out, checkBlocked)
	}
	return code
}

// tryItOutHome is the disposable store every phase runs against.
//
// Under the OS temp directory, which `resetGuard` accepts and which the real store is not. It is
// set in the PROCESS ENVIRONMENT because that is where the whole Director reads it from —
// `semanticMemoryPath`, `openGraph`, `enablePixelOnly` and `reset-test-state` all consult
// `$MARCO_HOME`, and setting it for one of them would be a Director that learns into one store
// and discards another.
//
// An existing `MARCO_HOME` is left exactly as it is. Somebody who set one meant it, and if what
// they meant was the real store then `resetGuard` refuses the whole experiment — which is the
// answer, not an inconvenience to route around.
func tryItOutHome() (string, error) {
	if home := os.Getenv("MARCO_HOME"); home != "" {
		return home, nil
	}
	home := filepath.Join(os.TempDir(), "marco-try-it-out")
	if err := os.MkdirAll(home, 0o755); err != nil {
		return "", fmt.Errorf("the disposable store could not be created: %w", err)
	}
	if err := os.Setenv("MARCO_HOME", home); err != nil {
		return "", fmt.Errorf("the disposable store could not be pointed at: %w", err)
	}
	return home, nil
}

// runTryItOut is `director try-it-out`.
func runTryItOut(args []string) int { return tryItOut(os.Stdout, args) }

// tryItOut is the command with its output named.
//
// Split from the command for the reason every harness in this tree is: the flags are not
// the thing worth testing and the refusals are.
func tryItOut(out io.Writer, args []string) int {
	fs := flag.NewFlagSet("try-it-out", flag.ExitOnError)
	prepare := fs.Bool("prepare", false,
		"establish the disposable store and report every precondition, interacting with nothing")
	observe := fs.Int("observe", 0,
		"watch for this many seconds and report whether a press from a hand arrives; "+
			"started only when a person has said they are ready")
	demonstrate := fs.Bool("demonstrate", false,
		"ask the live screen, then watch a person answer; keeps only what an attributed "+
			"crossing confirms")
	replay := fs.Bool("prepare-replay", false,
		"collect a fresh Stage and report what the saved relation picks out of it")
	exec := fs.Bool("execute", false, "perform exactly one guarded click")
	authorise := fs.Bool("authorise-one-click", false,
		"authorise the single click -execute would perform")
	clean := fs.Bool("clean", false, "empty the disposable store")
	phrase := fs.String("instruction", "",
		"the instruction being tested, e.g. \"click Bluetooth & devices\"")
	seconds := fs.Int("seconds", 120,
		"how long the demonstration phase waits for a person; generous on purpose, "+
			"because the person is told to click only once the question is open")
	bridgeFlag := fs.String("accessibility", defaultBridge(), "path to the accessibility bridge")
	maxNodes := fs.Int("max-nodes", 4000, "node ceiling per snapshot")
	_ = fs.Parse(flagsFirst(args))

	// ONE PHASE PER RUN. The same rule `demonstrate-surface` holds, for the same reason:
	// separating what learns from what presses is a safety property, and a front end that
	// chained them would be the thing that property exists to prevent.
	chosen := 0
	for _, on := range []bool{*prepare, *observe > 0, *demonstrate, *replay, *exec, *clean} {
		if on {
			chosen++
		}
	}
	if chosen != 1 {
		return tryItOutUsage(out)
	}

	home, err := tryItOutHome()
	if err != nil {
		fmt.Fprintf(out, "STOP: %v\n", err)
		checkpoint(out, checkBlocked)
		return 1
	}
	fmt.Fprintf(out, "disposable store: %s\n", home)
	switch {
	case *clean:
		// The checkpoint prints either way: a store that was already empty is a
		// clean store, and `reset-test-state` says so itself.
		code := runReset(nil)
		return atCheckpoint(out, code, checkCleaned)
	case *demonstrate:
		// DEMONSTRATION NEEDED is printed by the phase, at the moment the question
		// is actually open — not here, where it would be a guess about when.
		return atCheckpoint(out, runSurfaceLearn(out, *phrase, *bridgeFlag, *maxNodes,
			watchFor(*seconds)), checkLearned)
	case *replay:
		return atCheckpoint(out, runSurfaceReground(out, *phrase, *bridgeFlag, *maxNodes),
			checkReadyToExec)
	case *exec:
		code := runSurfaceExecute(out, *phrase, *bridgeFlag, *maxNodes, *authorise)
		// RESULT, whatever happened. See the vocabulary above.
		checkpoint(out, checkResult)
		return code
	case *observe > 0:
		// OBSERVING is printed by the watch itself, for the same reason
		// DEMONSTRATION NEEDED is.
		return atCheckpoint(out, surfacePreconditions(out, "", *bridgeFlag, *maxNodes, *observe),
			checkReady)
	}
	return atCheckpoint(out, surfacePreconditions(out, *phrase, *bridgeFlag, *maxNodes, 0),
		checkReady)
}

// tryItOutUsage names the phases in the order they happen.
func tryItOutUsage(out io.Writer) int {
	fmt.Fprintln(out, "director try-it-out: choose exactly one phase")
	fmt.Fprintln(out, "  -prepare           preconditions and the live Stage; interacts with nothing")
	fmt.Fprintln(out, "  -observe N         watch N seconds for a press from a hand")
	fmt.Fprintln(out, "  -demonstrate       ask, watch a person answer, keep what was confirmed")
	fmt.Fprintln(out, "  -prepare-replay    a fresh Stage, and what the saved relation picks out")
	fmt.Fprintln(out, "  -execute           one guarded click, with -authorise-one-click")
	fmt.Fprintln(out, "  -clean             empty the disposable store")
	return 2
}
