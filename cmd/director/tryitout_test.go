package main

// THE FRONT END MUST NOT BE A WAY ROUND THE GATES.
//
// `try-it-out` exists so an assistant can operate the harness instead of dictating commands. The
// risk in that is obvious and worth naming: a front end built for convenience is exactly where a
// gate gets quietly dropped, because dropping it makes the convenient thing work.
//
// So what is asserted here is the two gates that matter — the one live click still needs its own
// authorisation, and every phase that refuses says so in exactly one checkpoint — plus the thing
// an operator depends on, which is that the checkpoint vocabulary is closed.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// countCheckpoints reports the checkpoint lines in some output, in order.
func countCheckpoints(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), "CHECKPOINT: "); ok {
			out = append(out, rest)
		}
	}
	return out
}

// TRY-IT-OUT PRESSES NOTHING WITHOUT ITS OWN AUTHORISATION.
//
// The owner's limit, carried across the new front end unchanged:
//
//	Do not execute a learned action without separate explicit authorization.
//
// `-execute` alone must refuse, and the refusal must come BEFORE anything is built: no runtime,
// no live host, nothing observed. That is why this test can run at all — a front end that had
// started building a live Director before checking would need a desktop to prove it refused.
func TestTryItOutPressesNothingWithoutItsOwnAuthorisation(t *testing.T) {
	t.Setenv("MARCO_HOME", filepath.Join(t.TempDir(), "sandbox"))

	var buf bytes.Buffer
	code := tryItOut(&buf, []string{"-execute", "-instruction", "click Bluetooth & devices"})
	got := buf.String()

	if code == 0 {
		t.Errorf("an unauthorised execute succeeded:\n%s", got)
	}
	if !strings.Contains(got, "-authorise-one-click") {
		t.Errorf("the refusal does not name the authorisation it needs:\n%s", got)
	}
	// AND IT IS STILL A RESULT. A phase that was asked to press and refused has an
	// outcome; calling it BLOCKED would file a decision as a malfunction.
	if marks := countCheckpoints(got); len(marks) != 1 || marks[0] != checkResult {
		t.Errorf("checkpoints %v, want exactly [%s]", marks, checkResult)
	}
	// AND NOTHING WAS OBSERVED OR BUILT on the way to that refusal.
	for _, leaked := range []string{"PIXEL-ONLY TEST", "runtime:", "foreground"} {
		if strings.Contains(got, leaked) {
			t.Errorf("the refusal came after %q, so something was built before the "+
				"authorisation was checked:\n%s", leaked, got)
		}
	}
}

// EVERY REFUSAL ENDS IN EXACTLY ONE CHECKPOINT.
//
// An operator reads the checkpoint, not the prose, so two of them is an ambiguous outcome and
// none is a hang. One per phase, always, and from the phase's own exit code.
//
// The phases here are the ones that refuse before touching a desktop. `-prepare` and `-observe`
// build a runtime and look at the real screen, so they are not reachable from a unit test and are
// covered by the live run instead; what makes that acceptable is that they go through the same
// `atCheckpoint` as these.
func TestEveryRefusalEndsInExactlyOneCheckpoint(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"an execute with no authorisation", []string{"-execute", "-instruction", "click x"},
			checkResult},
		{"a demonstration with no instruction", []string{"-demonstrate"}, checkBlocked},
		{"a replay with no instruction", []string{"-prepare-replay"}, checkBlocked},
		{"emptying the disposable store", []string{"-clean"}, checkCleaned},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MARCO_HOME", filepath.Join(t.TempDir(), "sandbox"))
			var buf bytes.Buffer
			tryItOut(&buf, tc.args)
			marks := countCheckpoints(buf.String())
			if len(marks) != 1 {
				t.Fatalf("%d checkpoints %v, want one:\n%s", len(marks), marks,
					buf.String())
			}
			if marks[0] != tc.want {
				t.Errorf("checkpoint %q, want %q:\n%s", marks[0], tc.want,
					buf.String())
			}
		})
	}
}

// A PHASE HAS TO BE CHOSEN, AND ONLY ONE.
//
// Not a checkpoint: a command line nobody can act on is a usage error, and printing BLOCKED for
// it would file "you typed it wrong" as "a precondition failed". The two are different reports
// and an operator reacts to them differently.
func TestTryItOutNeedsExactlyOnePhase(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"-prepare", "-demonstrate"},
		{"-execute", "-clean", "-authorise-one-click"},
	} {
		var buf bytes.Buffer
		if code := tryItOut(&buf, args); code != 2 {
			t.Errorf("%v exited %d, want a usage error", args, code)
		}
		if marks := countCheckpoints(buf.String()); len(marks) != 0 {
			t.Errorf("%v reached %v, but no phase ran", args, marks)
		}
	}
}

// THE DISPOSABLE STORE IS ESTABLISHED, AND AN EXISTING ONE IS LEFT ALONE.
//
// Both halves matter. The first is the whole point — nobody should have to set an environment
// variable for a test to be safe. The second is the guard: somebody who pointed MARCO_HOME
// somewhere meant it, and if what they meant was the real store then `resetGuard` refuses the
// experiment, which is the answer rather than something to route around.
func TestTheDisposableStoreIsEstablishedWithoutBeingAskedFor(t *testing.T) {
	mine := filepath.Join(t.TempDir(), "mine")
	t.Setenv("MARCO_HOME", mine)
	if home, err := tryItOutHome(); err != nil || home != mine {
		t.Errorf("tryItOutHome() = %q, %v; want the home that was already set", home, err)
	}

	t.Setenv("MARCO_HOME", "")
	home, err := tryItOutHome()
	if err != nil {
		t.Fatalf("establishing a disposable store: %v", err)
	}
	if home == "" || home == defaultHome() {
		t.Fatalf("the disposable store is %q", home)
	}
	// AND IT IS ONE THE EXPERIMENT WILL ACCEPT, through the same predicate the reset
	// command uses. A store `resetGuard` refuses is a store nothing can be discarded from.
	if why := resetGuard(home, defaultHome()); why != "" {
		t.Errorf("the store it established is not a sandbox: %s", why)
	}
	// AND THE WHOLE DIRECTOR READS IT FROM THERE, not just the caller.
	if got := semanticMemoryPath(); !strings.HasPrefix(got, home) {
		t.Errorf("durable memory is at %q, outside the disposable store %q", got, home)
	}
}

// EVERY CHECKPOINT IN THE VOCABULARY IS PRINTED SOMEWHERE.
//
// Two of them — OBSERVING and DEMONSTRATION NEEDED — are printed by the phase that reaches the
// moment, not by the front end, and neither moment is reachable without a desktop. So no unit test
// can watch them happen, and deleting either call would leave an operator waiting for a checkpoint
// that is never coming while the harness carried on working perfectly.
//
// This reads the package instead. It is the same discipline as the value-taking flag table: a
// property about the source, written down as a test, because the alternative is trusting that
// somebody will remember.
func TestEveryCheckpointIsPrintedSomewhere(t *testing.T) {
	names := map[string]string{
		"checkReady": checkReady, "checkObserving": checkObserving,
		"checkDemoNeeded": checkDemoNeeded, "checkLearned": checkLearned,
		"checkReadyToExec": checkReadyToExec, "checkResult": checkResult,
		"checkCleaned": checkCleaned, "checkBlocked": checkBlocked,
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package: %v", err)
	}
	printed := map[string]int{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		for ident := range names {
			// EVERY MENTION, because a print site is spelled three ways: the
			// checkpoint call itself, the name handed to atCheckpoint beside a
			// delegate, and the same thing after gofmt has wrapped the line.
			// Matching the call shape would be a test about formatting.
			printed[ident] += strings.Count(string(src), ident)
		}
	}
	for ident, text := range names {
		// ONE for the declaration, so a constant nothing uses counts one.
		if printed[ident] < 2 {
			t.Errorf("nothing prints CHECKPOINT: %s (%s), so an operator waiting "+
				"for it would wait forever while the harness worked", text, ident)
		}
	}
}
