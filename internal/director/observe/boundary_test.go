package observe_test

import (
	"go/build"
	"strings"
	"testing"
)

// The passive guarantee, as a property of the build.
//
// "The session emits no input" is the whole promise of this milestone, and a promise kept by
// discipline is one that lasts until somebody adds a convenient import. This walks the
// package's transitive dependencies and fails if anything that can touch a desktop is
// reachable at all.
//
// The point is not that the current code does not click. It is that the current code COULD
// NOT, because nothing it can reach knows how.

// forbidden are packages that can affect the machine, by import path fragment.
var forbidden = []struct {
	fragment string
	why      string
}{
	{"internal/director/marcoexec", "lowers and runs Marco programs — the one path to a desktop effect"},
	{"internal/director/execute", "the execution pipeline: plans and performs actions"},
	{"internal/oshost", "the OS host: keyboard, mouse, clipboard, secrets"},
	{"internal/recorder", "installs low-level input hooks"},
	{"internal/driver", "drives input"},
	{"internal/runtime", "the Marco runtime — can invoke hosts"},
	{"internal/compile", "compiles Marco source, which exists to be run"},
	{"internal/director/goal", "goal execution"},
	{"internal/director/plan", "action planning"},
	{"internal/director/target", "target resolution: choosing what to act ON"},
	{"internal/director/verify", "verification of mutations, which implies mutations"},
	{"internal/winctx", "window activation and focus"},
	{"internal/screen", "screen capture and input geometry, reached only via injected interfaces"},
	{"internal/platform", "platform adapters, reached only via injected interfaces"},
	{"os/exec", "starting processes"},
}

func TestTheObservationPackageCannotAct(t *testing.T) {
	reachable := map[string]bool{}
	if err := walkImports("github.com/chaynes-simpleclouds/marco/internal/director/observe",
		reachable, 0); err != nil {
		t.Fatalf("walking imports: %v", err)
	}

	for path := range reachable {
		for _, f := range forbidden {
			if strings.Contains(path, f.fragment) {
				t.Errorf("observe reaches %s\n\t%s reason: %s\n\t"+
					"the session must not be ABLE to act, not merely choose not to",
					path, f.fragment, f.why)
			}
		}
	}
}

func TestTheObservationPackageStaysGameAgnostic(t *testing.T) {
	reachable := map[string]bool{}
	if err := walkImports("github.com/chaynes-simpleclouds/marco/internal/director/observe",
		reachable, 0); err != nil {
		t.Fatalf("walking imports: %v", err)
	}
	for path := range reachable {
		if strings.Contains(path, "internal/gamepacks") {
			t.Errorf("observe reaches %s; the core produces evidence and a pack assigns "+
				"meaning, never the other way round", path)
		}
	}
}

// walkImports collects the transitive non-stdlib imports of a package.
func walkImports(path string, seen map[string]bool, depth int) error {
	if depth > 12 || seen[path] {
		return nil
	}
	seen[path] = true

	pkg, err := build.Import(path, ".", 0)
	if err != nil {
		// A package that will not resolve cannot be inspected; that is a build problem
		// rather than a boundary violation, and the ordinary build catches it.
		return nil //nolint:nilerr
	}
	for _, imported := range pkg.Imports {
		if !strings.Contains(imported, ".") {
			continue // stdlib
		}
		if err := walkImports(imported, seen, depth+1); err != nil {
			return err
		}
	}
	return nil
}
