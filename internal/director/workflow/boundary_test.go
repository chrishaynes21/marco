package workflow_test

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A LEARNED WORKFLOW CANNOT ACT AND CANNOT REMEMBER.
//
// The same two promises `fastobserve` makes, and they matter more here. A workflow is the most
// promotable-looking thing this tier has produced: it is literally a saved sequence of actions
// with the evidence to resolve each one. A draft that could lower itself would be an autonomous
// actuator with a reading interface.
//
// So the refusal is a property of the build. The point is not that a draft does not press
// anything — it is that it COULD NOT, because nothing it can reach knows how.

var forbidden = []struct {
	fragment string
	why      string
}{
	{"internal/director/marcoexec", "lowers and runs Marco programs — the one path to a " +
		"desktop effect. A plan is handed OUT to a caller that owns this; the plan " +
		"must not be able to take the last step itself"},
	{"internal/director/execute", "the execution pipeline: plans and performs actions"},
	{"internal/oshost", "the OS host: keyboard, mouse, clipboard, secrets"},
	{"internal/recorder", "installs low-level input hooks"},
	{"internal/driver", "drives input"},
	{"internal/runtime", "the Marco runtime — can invoke hosts"},
	{"internal/compile", "compiles Marco source, which exists to be run"},
	{"internal/director/goal", "goal execution"},
	{"internal/director/actiongraph", "the durable action graph: permanent Places and edges"},
	{"internal/director/semanticmemory", "the durable store, where permanent identities live"},
	{"internal/winctx", "window activation and focus"},
	{"os/exec", "starting processes"},
}

func TestAWorkflowCannotActOrRemember(t *testing.T) {
	reachable := map[string]bool{}
	if err := walkImports(
		"github.com/chaynes-simpleclouds/marco/internal/director/workflow",
		reachable, 0); err != nil {
		t.Fatalf("walking imports: %v", err)
	}
	// MODULE-LOCAL PATHS ONLY, for the reason fastobserve's copy of this records: Go's own
	// standard library contains `internal/runtime/sys`, and a bare fragment match against
	// `internal/runtime` catches it while saying nothing about Marco's runtime.
	const module = "github.com/chaynes-simpleclouds/marco/"
	for path := range reachable {
		if !strings.HasPrefix(path, module) {
			continue
		}
		for _, f := range forbidden {
			if strings.Contains(path, f.fragment) {
				t.Errorf("workflow reaches %s\n\t%s — %s\n\ta learned draft "+
					"must not be ABLE to perform itself",
					path, f.fragment, f.why)
			}
		}
	}
}

// AND IT NAMES NOTHING THAT MAKES A DRAFT DURABLE OR A CLICK REAL.
//
// The import walk catches a package; this catches a call. `observe` is reachable on purpose and
// `observe` also holds the place establisher, so the one package this depends on is the one it
// could use to cross the line.
func TestAWorkflowNamesNothingDurableOrExecutable(t *testing.T) {
	src, err := os.ReadFile("workflow.go")
	if err != nil {
		t.Fatalf("reading the workflow: %v", err)
	}
	body := string(src)
	for _, name := range []string{
		"EstablishPlace", "StructureSignature", "NewScreenSignature",
		"RememberedRelationship", "DurablePlace", "Hypothesis",
	} {
		if strings.Contains(body, name) {
			t.Errorf("workflow names %q. A draft refers to screens identified by a "+
				"count of rectangles in nine boxes; the durable store takes a "+
				"composition of roles a provider vouched for, and a saved "+
				"workflow is not a licence to confuse them", name)
		}
	}
	for _, name := range []string{
		"SetForegroundWindow", "SendInput", "mouse_event", "keybd_event", "syscall",
		"os.WriteFile", "os.Create",
	} {
		if strings.Contains(body, name) {
			t.Errorf("workflow names %q; it records what was demonstrated and "+
				"computes what would be done", name)
		}
	}
}

// THE TIER GREW TO INCLUDE THIS PACKAGE, AND THAT IS VISIBLE FROM BOTH SIDES.
//
// `observe`'s TestAVisualScreenCannotReachDurableMemory walks internal/ and fails if the visual
// names appear outside an allowlist. This package uses them, so it is in that allowlist — the
// fourth addition, recorded in [[ADR-181-a-demonstration-marco-can-keep]]. If somebody removes
// it, that test fails and names this file; if somebody adds a package without saying so, this
// one does.
func TestThisPackageIsPartOfTheSessionLocalTier(t *testing.T) {
	path := filepath.Join("..", "observe", "arrangement_test.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the durable-boundary test: %v", err)
	}
	if !strings.Contains(string(src), "workflow") {
		t.Error("observe's durable-boundary allowlist does not name workflow, so " +
			"either that test is failing or the tier grew without anybody saying so")
	}
}

// walkImports collects a package's transitive dependencies.
func walkImports(path string, seen map[string]bool, depth int) error {
	if depth > 24 || seen[path] {
		return nil
	}
	seen[path] = true
	pkg, err := build.Import(path, "", 0)
	if err != nil {
		// A package that does not build on this platform cannot be reached on it.
		return nil
	}
	for _, imp := range pkg.Imports {
		if !strings.Contains(imp, "/") || strings.HasPrefix(imp, "golang.org/") {
			continue
		}
		if err := walkImports(imp, seen, depth+1); err != nil {
			return err
		}
	}
	return nil
}
