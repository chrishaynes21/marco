package teach_test

import (
	"go/build"
	"strings"
	"testing"
)

// The passive guarantee, extended to the layer that ASKS a person to do something.
//
// Teach is the first thing in this system that addresses the user directly and then watches. That
// makes it the most natural place for somebody to eventually reach for "…and now just press the
// key yourself to check". This holds the line structurally: the coordinator cannot reach anything
// that presses a key, opens a window, starts a process or captures a screen, so the only evidence
// it can ever have is a session Result somebody else produced.
//
// It is also what makes the injected-input exclusion safe under Teach. That exclusion lives in
// the platform navigation source, several layers below; Teach cannot reach it to weaken it, and
// cannot reach an input API to route around it.

var forbidden = []struct{ fragment, why string }{
	{"internal/director/marcoexec", "lowers and runs Marco programs"},
	{"internal/director/execute", "the execution pipeline"},
	{"internal/director/rehearse", "spends a rehearsal grant"},
	{"internal/oshost", "keyboard, mouse, clipboard, secrets"},
	{"internal/recorder", "installs low-level input hooks"},
	{"internal/driver", "drives input"},
	{"internal/runtime", "the Marco runtime — can invoke hosts"},
	{"internal/compile", "compiles Marco source, which exists to be run"},
	{"internal/director/goal", "goal execution"},
	{"internal/director/plan", "action planning"},
	{"internal/director/target", "choosing what to act ON"},
	{"internal/platform", "platform adapters, including the input source whose " +
		"injected-input exclusion Teach must not be able to reach"},
	{"internal/winctx", "window activation and focus"},
	{"internal/screen", "screen capture"},
	{"os/exec", "starting processes"},
}

func TestTeachCannotAct(t *testing.T) {
	reachable := map[string]bool{}
	walk("github.com/chaynes-simpleclouds/marco/internal/director/teach", reachable, 0)

	for path := range reachable {
		for _, f := range forbidden {
			if strings.Contains(path, f.fragment) {
				t.Errorf("teach reaches %s\n\treason it is forbidden: %s\n\t"+
					"Teach asks a person to demonstrate; it must never be able to "+
					"demonstrate anything itself", path, f.why)
			}
		}
	}
}

// Teach must not be able to write a file either. Its whole durable footprint is one learning
// request through the semantic-memory interface, and a package that could open a file could keep
// a demonstration log — which is on the list of things no durable record may contain.
func TestTeachCannotOpenAFile(t *testing.T) {
	pkg, err := build.Import(
		"github.com/chaynes-simpleclouds/marco/internal/director/teach", ".", 0)
	if err != nil {
		t.Fatalf("importing the teach package: %v", err)
	}
	for _, imported := range pkg.Imports {
		switch imported {
		case "os", "io/ioutil", "path/filepath", "net", "net/http":
			t.Errorf("teach imports %q; it orchestrates and must persist nothing of its own",
				imported)
		}
	}
}

func walk(path string, seen map[string]bool, depth int) {
	if depth > 12 || seen[path] {
		return
	}
	seen[path] = true
	pkg, err := build.Import(path, ".", 0)
	if err != nil {
		return
	}
	for _, imported := range pkg.Imports {
		if strings.Contains(imported, ".") {
			walk(imported, seen, depth+1)
		}
	}
}
