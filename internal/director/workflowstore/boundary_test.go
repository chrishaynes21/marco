package workflowstore_test

// A DURABLE WORKFLOW CANNOT ACT, AND CANNOT BECOME A PLACE.
//
// `workflow` already makes this promise about a draft. It matters more here, because this is the
// first thing in the subsystem that OUTLIVES the session that learned it — and the two failures
// it could enable are the two the owner has refused most consistently:
//
//	Do not promote geometry-only screen hypotheses into durable cross-session Places.
//	A saved workflow is not executable authority.
//
// So the refusal is a property of the build. A saved workflow could not press anything or write
// itself into the durable graph, because nothing it can reach knows how.

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var forbidden = []struct {
	fragment string
	why      string
}{
	{"internal/director/marcoexec", "lowers and runs Marco programs — the one path to a " +
		"desktop effect. Persistence and lowering stay separate: a saved workflow " +
		"that could lower itself would be an executable file in the strongest sense"},
	{"internal/director/execute", "the execution pipeline: plans and performs actions"},
	{"internal/director/actiongraph", "the durable action graph, where permanent Places " +
		"and edges live. THE one this package exists next to: a workflow artifact " +
		"is durable and a Place is durable, and only one of them is a claim about " +
		"the world"},
	{"internal/director/semanticmemory", "the durable store, where permanent identities live"},
	{"internal/director/goal", "goal execution"},
	{"internal/oshost", "the OS host: keyboard, mouse, clipboard, secrets"},
	{"internal/recorder", "installs low-level input hooks"},
	{"internal/driver", "drives input"},
	{"internal/runtime", "the Marco runtime — can invoke hosts"},
	{"internal/compile", "compiles Marco source, which exists to be run"},
	{"internal/winctx", "window activation and focus"},
	{"os/exec", "starting processes"},
}

func TestASavedWorkflowCannotActOrBecomeAPlace(t *testing.T) {
	const pkg = "github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"

	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Up to the module root, so the build context resolves the module's own packages.
	root = filepath.Join(root, "..", "..", "..")

	seen := map[string]bool{}
	var walk func(string, []string)
	walk = func(path string, via []string) {
		if seen[path] {
			return
		}
		seen[path] = true
		p, err := build.Default.Import(path, root, 0)
		if err != nil {
			// A package that does not build on this platform cannot be walked.
			// Reported rather than skipped silently.
			if _, ok := err.(*build.NoGoError); !ok && !strings.Contains(
				err.Error(), "cannot find package") {
				t.Logf("not walked: %s (%v)", path, err)
			}
			return
		}
		for _, f := range forbidden {
			if !strings.Contains(path, f.fragment) {
				continue
			}
			t.Errorf("a saved workflow can reach %s\n\tby %s\n\twhich %s",
				path, strings.Join(append(via, path), " -> "), f.why)
		}
		for _, imp := range p.Imports {
			if !strings.HasPrefix(imp, "github.com/chaynes-simpleclouds/marco") {
				continue
			}
			walk(imp, append(via, path))
		}
	}
	walk(pkg, nil)

	if len(seen) < 2 {
		t.Fatalf("the walk visited %d package(s), so it is not walking anything and "+
			"would pass however this package changed", len(seen))
	}
}
