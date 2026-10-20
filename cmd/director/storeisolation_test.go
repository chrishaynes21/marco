package main

// NO TEST IN THIS PACKAGE WRITES INTO THE OWNER'S REAL STORE.
//
// # The leak this closes
//
// `go test ./...` was leaving run records in the real store — three per suite run. They come
// from the tests that drive `runPerformWorkflow` end to end with a workflow file in a
// `t.TempDir()` and no MARCO_HOME: the command reserves its run reference and writes its record
// under `configDir()`, which resolves to the real store when nothing has said otherwise. The
// tests were right about everything they were testing and wrong about where they were standing.
//
// It was found by building Studio's run history, which reads the records a person's own runs
// left: the owner's five most recent runs were three runs of `notepad` and `director.test` from
// a suite, above their own. Nothing was destroyed — a record is a file the runner is entitled to
// write — but a suite that writes into the directory somebody keeps their work in has no
// business doing so, and the next thing to leak that way might not be a record.
//
// So the package runs in a temporary store unless something has already named one. A test that
// wants its own still names it with `t.Setenv`, which overrides this for that test and puts it
// back afterwards; a test that does not care no longer gets the real one by default. Child
// processes inherit it, so a test that spawns `director` is isolated too.

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("MARCO_HOME") != "" {
		// SOMETHING ALREADY NAMED ONE. A dogfood store, a harness, a person
		// debugging one test against a store they prepared: all of those are
		// deliberate, and overriding them here would silently move the ground
		// under a test that was aimed somewhere on purpose.
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "director-tests-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "director tests: no temporary store: %v\n", err)
		os.Exit(1)
	}
	if err := os.Setenv("MARCO_HOME", dir); err != nil {
		fmt.Fprintf(os.Stderr, "director tests: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// AND THE ISOLATION IS ASSERTED, so it cannot be removed without a test saying so.
//
// Through `resetGuard`, which is the hardest guard in this repository and already knows every
// way of spelling the real store: a slash, a letter case, a trailing separator, a `.` on the
// end. Whatever this package is pointed at has to pass the same test a destructive reset does.
func TestTheseTestsDoNotRunAgainstTheRealStore(t *testing.T) {
	home := os.Getenv("MARCO_HOME")
	if strings.TrimSpace(home) == "" {
		t.Fatal("these tests run with no MARCO_HOME, so every command they drive " +
			"writes into the real store — see TestMain above")
	}
	if why := resetGuard(home, defaultHome()); why != "" {
		t.Errorf("these tests write into %s, which is the real store: %s", home, why)
	}
}
