package fastobserve_test

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// THE TWO PROMISES OF THIS PACKAGE, AS PROPERTIES OF THE BUILD.
//
// It observes, and it does not act, and it does not remember. Promises kept by discipline last
// until somebody adds a convenient import, so these walk the dependency graph and the source
// instead of trusting the current code.
//
// The point is not that the observer does not click or store. It is that it COULD NOT, because
// nothing it can reach knows how.

// forbidden are packages a passive visual observer must not be able to reach.
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
	{"internal/director/semanticmemory", "the durable store. A visual screen is a count of " +
		"rectangles in nine boxes, and this is where permanent identities live"},
	{"os/exec", "starting processes"},
}

func TestTheFastObserverCannotActOrRemember(t *testing.T) {
	reachable := map[string]bool{}
	if err := walkImports(
		"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve",
		reachable, 0); err != nil {
		t.Fatalf("walking imports: %v", err)
	}
	// MODULE-LOCAL PATHS ONLY. Go's own standard library has `internal/runtime/sys` and
	// `internal/runtime/syscall/windows` in it, and a fragment match against a bare
	// `internal/runtime` catches those — which says nothing about whether this package
	// can reach MARCO's runtime. The first version of this test failed on exactly that.
	const module = "github.com/chaynes-simpleclouds/marco/"
	for path := range reachable {
		if !strings.HasPrefix(path, module) {
			continue
		}
		for _, f := range forbidden {
			if strings.Contains(path, f.fragment) {
				t.Errorf("fastobserve reaches %s\n\t%s — %s\n\t"+
					"a passive observer must not be ABLE to act or to "+
					"remember, not merely choose not to",
					path, f.fragment, f.why)
			}
		}
	}
}

// AND IT NAMES NOTHING THAT MAKES A SCREEN DURABLE.
//
// The import walk catches a package. This catches a call: `observe` is reachable on purpose —
// the arrangement and the correlator live there — and `observe` also holds the place
// establisher and the durable signature. So the one package this depends on is also the one it
// could use to cross the line.
//
//	Keep visual screen identities session-local. Do not promote geometry-only identities to
//	durable graph Places.
func TestTheFastObserverNamesNothingDurable(t *testing.T) {
	src, err := os.ReadFile("fastobserve.go")
	if err != nil {
		t.Fatalf("reading the observer: %v", err)
	}
	body := string(src)
	for _, forbidden := range []string{
		"EstablishPlace", "StructureSignature", "NewScreenSignature",
		"RememberedRelationship", "DurablePlace", "Hypothesis",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the observer names %q. A visual screen is a count of "+
				"rectangles in nine boxes; the durable store takes a "+
				"composition of roles a provider vouched for, and promoting one "+
				"into the other is a decision somebody takes on purpose",
				forbidden)
		}
	}
	// It must not be able to raise, focus or press anything either, which the import
	// walk covers for packages and this covers for syscalls somebody might add directly.
	for _, forbidden := range []string{
		"SetForegroundWindow", "SendInput", "mouse_event", "keybd_event", "syscall",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the observer names %q; it folds geometry and publishes "+
				"hypotheses", forbidden)
		}
	}
}

// The visual names are still held to their own files inside internal/, and this package is
// counted as part of that tier on purpose.
//
// `observe`'s own TestAVisualScreenCannotReachDurableMemory walks internal/ and fails if the
// visual names appear outside an allowlist. This package uses them, so it is IN that allowlist —
// a decision, taken once, recorded in [[ADR-178-the-fast-observer]], and the next addition
// should be as hard. This test exists so that the allowlist's growth is visible from both sides:
// if somebody removes this package from it, that test fails and names this file.
func TestThisPackageIsPartOfTheSessionLocalTier(t *testing.T) {
	// The allowlist lives in observe's test. Read it and check this package is named.
	path := filepath.Join("..", "observe", "arrangement_test.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the durable-boundary test: %v", err)
	}
	if !strings.Contains(string(src), "fastobserve") {
		t.Error("observe's durable-boundary allowlist does not name fastobserve, so " +
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
