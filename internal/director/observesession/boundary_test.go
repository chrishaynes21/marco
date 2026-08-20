package observesession_test

import (
	"go/build"
	"strings"
	"testing"
)

// The passive guarantee, extended to the layer that actually runs.
//
// The pure core proves it cannot act. That is worth little if the package ORCHESTRATING it
// can, since the orchestrator is where a capture call would naturally be reached for. It
// takes everything by interface for exactly this reason, and this holds the line.

var forbidden = []struct{ fragment, why string }{
	{"internal/director/marcoexec", "lowers and runs Marco programs"},
	{"internal/director/execute", "the execution pipeline"},
	{"internal/oshost", "keyboard, mouse, clipboard, secrets"},
	{"internal/recorder", "installs low-level input hooks"},
	{"internal/driver", "drives input"},
	{"internal/runtime", "the Marco runtime — can invoke hosts"},
	{"internal/compile", "compiles Marco source, which exists to be run"},
	{"internal/director/goal", "goal execution"},
	{"internal/director/plan", "action planning"},
	{"internal/director/target", "choosing what to act ON"},
	{"internal/winctx", "window activation and focus"},
	{"internal/screen", "screen capture, which arrives through an injected Sampler"},
	{"internal/platform", "platform adapters, which arrive through injected interfaces"},
	{"os/exec", "starting processes"},
	{"internal/gamepacks", "a pack assigns meaning; the runner produces evidence"},
}

func TestTheRunnerCannotAct(t *testing.T) {
	reachable := map[string]bool{}
	walk("github.com/chaynes-simpleclouds/marco/internal/director/observesession", reachable, 0)

	for path := range reachable {
		for _, f := range forbidden {
			if strings.Contains(path, f.fragment) {
				t.Errorf("observesession reaches %s\n\treason it is forbidden: %s\n\t"+
					"the runner orchestrates; it must not be able to perceive or act directly",
					path, f.why)
			}
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
