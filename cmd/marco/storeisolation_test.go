package main

// NO TEST IN THIS PACKAGE WRITES INTO THE OWNER'S REAL STORE.
//
// # The leak this closes, which is the second of its kind
//
// `cmd/director` already has this file, and its comment ends: *the next thing to leak that way
// might not be a record.* The next thing to leak that way was a record, from here.
//
// A routine's run now claims a reference and writes its account under `marcopaths.Runs()`, which
// resolves to the real store when nothing has said otherwise. Several tests in this package drive
// `runInvocation` end to end with a registry in a `t.TempDir()` and no `MARCO_HOME` — they were
// right about everything they were testing and wrong about where they were standing — so
// `go test ./cmd/marco/` left three run records in the directory the owner keeps their work in,
// every run.
//
// Nothing was destroyed: a record is a file the executor is entitled to write, and the owner's own
// runs sit beside them. But a suite that writes into somebody's store has no business doing so,
// and this is the second time the same shape has appeared — which is why the comparison these
// tests are judged by now lives in one place (`marcopaths.SameStore`) rather than in each guard.
//
// So the package runs in a temporary store unless something has already named one. A test that
// wants its own still names it with `t.Setenv`, which overrides this for that test and puts it
// back afterwards; a test that does not care no longer gets the real one by default. Child
// processes inherit it, so a test that spawns `director` or `marco` is isolated too.

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
)

// scriptedChildLines and scriptedChildCode are how a test tells this binary to be somebody
// else's child instead of running tests.
//
// # Why the child is this binary
//
// Because a test that needs a process printing a scripted transcript needs a program, and every
// other way of getting one reads something the machine happened to have: `sh` is Git Bash's and
// is absent under `gate.ps1` and on a clean Windows box, `cmd.exe` moves the same assumption to
// another platform, and `go run` needs the network on a cold module cache. The test binary is a
// program that certainly exists, because it is the one asking.
//
// The pattern is `os/exec`'s own: re-execute with an environment variable the entry point looks
// for, print, and EXIT " ' + M + ' before the testing framework prints a `PASS` nobody asked for, which
// is what makes the transcript exactly the scripted lines.
const (
	scriptedChildLines = "MARCO_TEST_SCRIPTED_LINES"
	scriptedChildCode  = "MARCO_TEST_SCRIPTED_CODE"
)

// scriptedChildMode is this binary being a child rather than a test run.
//
// Checked FIRST in `TestMain`, before the temporary store below: a child is not a test run and
// has no business making a store, and the parent has already put one in its environment anyway.
func scriptedChildMode() bool {
	lines, ok := os.LookupEnv(scriptedChildLines)
	if !ok {
		return false
	}
	for _, l := range strings.Split(lines, "\n") {
		if l != "" {
			fmt.Println(l)
		}
	}
	code, err := strconv.Atoi(os.Getenv(scriptedChildCode))
	if err != nil {
		code = 0
	}
	os.Exit(code)
	return true // unreachable; the compiler wants it
}

func TestMain(m *testing.M) {
	// THE CHILD MODE FIRST. See scriptedChildMode.
	scriptedChildMode()
	if os.Getenv(marcopaths.EnvHome) != "" {
		// SOMETHING ALREADY NAMED ONE. A dogfood store, a harness, a person
		// debugging one test against a store they prepared: all of those are
		// deliberate, and overriding them here would silently move the ground under
		// a test that was aimed somewhere on purpose.
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "marco-tests-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco tests: no temporary store: %v\n", err)
		os.Exit(1)
	}
	if err := os.Setenv(marcopaths.EnvHome, dir); err != nil {
		fmt.Fprintf(os.Stderr, "marco tests: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// AND THE ISOLATION IS ASSERTED, so it cannot be removed without a test saying so.
//
// Both halves, because they are two different ways of standing in the real store: no `MARCO_HOME`
// at all, which is how this leaked; and a `MARCO_HOME` pointing AT the default location, which is
// the same directory by a longer route.
func TestTheseTestsDoNotRunAgainstTheRealStore(t *testing.T) {
	home := os.Getenv(marcopaths.EnvHome)
	if strings.TrimSpace(home) == "" {
		t.Fatal("these tests run with no MARCO_HOME, so every play they perform " +
			"writes its record into the real store — see TestMain above")
	}
	if marcopaths.SameStore(home, marcopaths.Default()) {
		t.Errorf("these tests write into %s, which is the real store", home)
	}
}

// AND A RUN RECORD GOES WHERE THIS PACKAGE IS STANDING.
//
// The isolation above is only worth anything if the thing that leaked actually follows it. This
// asserts the two agree: the place a record would be written is inside the temporary store, not
// beside the owner's routines.
func TestARunRecordWouldBeWrittenInsideTheTemporaryStore(t *testing.T) {
	home := os.Getenv(marcopaths.EnvHome)
	if !strings.HasPrefix(marcopaths.Runs(), home) {
		t.Errorf("a record would go to %s, and this package's store is %s",
			marcopaths.Runs(), home)
	}
}
