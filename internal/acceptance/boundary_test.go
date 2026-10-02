package acceptance_test

import (
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The rules that make an acceptance result mean something, held as tests rather than as prose.
//
// Both of these run in the ordinary suite — no gate, no desktop, no input. They are about what the
// CODE can reach, and the whole point of them is to fail on the day somebody takes a shortcut that
// would make every acceptance above meaningless while leaving them all green.

// MARCO CANNOT READ THE ORACLE.
//
// # Why this is the load-bearing test in the package
//
// The acceptance suite's claim is "Marco pressed Save, and here is independent proof". The proof is
// independent only because the fixture's state file is invisible to Marco. If anything Marco can
// reach knew about it, an acceptance proving "Marco clicked Save" would be proving that Marco can
// read a file — and it would still pass, which is what makes this worth a test rather than a
// comment.
//
// So: no production package may mention the oracle's name. The harness and the fixture may; nothing
// else may.
//
// Deleting this must be the thing that fails when somebody wires the oracle into perception to make
// a stubborn case pass.
func TestMarcoCannotReadTheOracle(t *testing.T) {
	root := repoRootFromTest(t)

	// Where the oracle is ALLOWED to be known. Two places: the application that writes it, and
	// the harness that reads it. Each is named with the reason it is here.
	allowed := map[string]string{
		"plugins/fixture/Fixture.cs":           "writes it",
		"internal/acceptance/acceptance.go":    "reads it, after Marco has finished",
		"internal/acceptance/report.go":        "renders it",
		"internal/acceptance/desktop_test.go":  "asserts on it",
		"internal/acceptance/boundary_test.go": "this test",
		"internal/acceptance/harness_test.go":  "exercises the harness",
	}

	var offenders []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".cs" {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		if strings.HasPrefix(rel, "_dl/") {
			return nil
		}
		if _, ok := allowed[rel]; ok {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if strings.Contains(string(b), "fixture-state") {
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("these files know about the acceptance oracle:\n  %s\n\n"+
			"The oracle is the harness's INDEPENDENT ground truth, and it is independent "+
			"only because Marco cannot see it. Anything Marco can reach that reads it turns "+
			"every acceptance in this package from `Marco pressed Save` into `Marco can "+
			"read a file` — and they would all still pass.",
			strings.Join(offenders, "\n  "))
	}
}

// THE HARNESS CANNOT PERFORM THE ACTION UNDER TEST.
//
//	THE HARNESS MAY PUT THE SAVE BUTTON THERE. ONLY MARCO MAY PRESS SAVE.
//
// # What this checks, and what it cannot
//
// It checks that the harness holds no input primitive: nothing that sends a key, moves a pointer,
// clicks, or reaches a platform package that could. A harness with its own SendInput would be able
// to make any acceptance pass by doing the work itself, and the failure would look exactly like a
// success.
//
// What it cannot check is intent — a harness could in principle drive input through a subprocess
// whose name it does not spell. That is why the list below is about CAPABILITY rather than about
// naming: there is no import here through which input exists, so there is nothing to misuse.
//
// `user32` and `SendInput` are deliberately in the list even though the fixture's own raise uses
// SetForegroundWindow: that call lives in the FIXTURE, inside its own process, which is the one
// place it can legally be — see Fixture.Raise.
func TestTheHarnessCannotPerformTheActionUnderTest(t *testing.T) {
	root := repoRootFromTest(t)
	harness := filepath.Join(root, "internal", "acceptance")

	forbidden := []string{
		"SendInput",       // injected input
		"mouse_event",     // its older spelling
		"keybd_event",     // and the keyboard's
		"SetCursorPos",    // moving a pointer
		"internal/oshost", // Marco's own input backend
		"internal/platform/recordhost",
		"internal/platform/theaterhost", // the actor layer that performs
		"internal/recorder",             // the hooks
		"internal/runtime",              // the Marco engine, which could run a program directly
		"internal/driver",
	}

	entries, err := os.ReadDir(harness)
	if err != nil {
		t.Fatalf("reading the harness: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".go" {
			continue
		}
		if e.Name() == "boundary_test.go" {
			continue // this file names them in order to forbid them
		}
		// THE CODE, NOT THE PROSE.
		//
		// Read whole, this test failed on its own package documentation, which explains the
		// measured SendInput finding in order to record WHY the harness runs on the real
		// desktop. A guard that cannot tell a capability from a sentence about a capability is
		// a guard that forbids explaining itself — met twice now in this repository, in
		// `interpret`'s import check and here.
		//
		// So the comments are discarded and what remains is what the compiler sees.
		source := codeOnly(t, filepath.Join(harness, e.Name()))
		for _, f := range forbidden {
			if strings.Contains(source, f) {
				t.Errorf("%s mentions %q.\n\n"+
					"The harness arranges the world and reads the result. It may launch an "+
					"application and ask that application to come forward; it may not send "+
					"input, resolve a control, or run a Marco program. An acceptance whose "+
					"harness could press the button proves nothing about Marco.",
					e.Name(), f)
			}
		}
	}
}

// repoRootFromTest walks up to the directory holding go.mod.
func repoRootFromTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not find the repository root")
	return ""
}

// codeOnly returns one Go file with every comment removed.
//
// Parsed and reprinted rather than filtered line by line, because a `//` inside a string literal
// is not a comment and a `/*` inside one is not the start of a block — and a guard that got that
// wrong would silently stop checking the file it was pointed at.
func codeOnly(t *testing.T, path string) string {
	t.Helper()
	fset := token.NewFileSet()
	// Mode 0: comments are not attached, so printing drops them.
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	var b strings.Builder
	if err := printer.Fprint(&b, fset, file); err != nil {
		t.Fatalf("printing %s: %v", path, err)
	}
	return b.String()
}
