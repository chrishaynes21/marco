package workflowrun_test

// THE COORDINATOR'S ONLY ROUTE TO AN EFFECT IS LEGAL MARCO.
//
// This package is the first thing in the subsystem that is ALLOWED to act, which makes it the
// one place where "nothing reaches the desktop except as legal Marco source"
// ([[ADR-005-legal-marco-only]]) stops being inherited and has to be established. Every other
// package here is safe because it cannot reach a host at all; this one is safe because the only
// thing it can reach is the compiler.
//
// So the test is in two halves, and the second is the half that matters. It is easy to write an
// import walk that forbids a list of packages and passes forever because the package it is
// guarding imports nothing interesting; a walk that also REQUIRES the legal path to be present
// fails if somebody removes the executor, which is the mistake that would leave the forbidden
// list guarding nothing.

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cannotReach are the packages that could emit input, or persist a belief, by another route.
var cannotReach = []struct{ fragment, why string }{
	{"internal/oshost", "the OS host: keyboard, mouse, clipboard, secrets. The one thing " +
		"this package exists to go AROUND rather than to"},
	{"internal/recorder", "installs low-level input hooks"},
	{"internal/driver", "drives Marco directly, skipping directorapi.MarcoRunner — which " +
		"is how a composition root's choice of host stops being a choice"},
	{"internal/runtime", "the Marco runtime: can invoke a host without compiling anything"},
	{"internal/compile", "compiling here would mean this package decides what is legal"},
	{"internal/winctx", "window activation and focus"},
	{"internal/screen", "screen capture. A coordinator that took its own pixels would be a " +
		"second perception path, and it could disagree with the one the plan was " +
		"made against"},
	{"internal/platform", "platform implementations are wired in, never imported"},
	{"internal/director/actiongraph", "the durable action graph. Running a workflow must " +
		"not promote anything into permanent memory"},
	{"internal/director/semanticmemory", "the durable store, where permanent identities live"},
	{"internal/production", "a Theater production: a second way to press something"},
	{"os/exec", "starting processes"},
}

// mustReach is the path that has to exist, or the list above is guarding nothing.
const mustReach = "internal/director/marcoexec"

func TestTheOnlyRouteToAnEffectIsLegalMarco(t *testing.T) {
	const pkg = "github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"

	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(root, "..", "..", "..")

	seen := map[string]bool{}
	reachedMarcoexec := false
	var walk func(string, []string)
	walk = func(path string, via []string) {
		if seen[path] {
			return
		}
		seen[path] = true
		p, err := build.Default.Import(path, root, 0)
		if err != nil {
			if _, ok := err.(*build.NoGoError); !ok && !strings.Contains(
				err.Error(), "cannot find package") {
				t.Logf("not walked: %s (%v)", path, err)
			}
			return
		}
		if strings.Contains(path, mustReach) {
			reachedMarcoexec = true
		}
		for _, f := range cannotReach {
			if !strings.Contains(path, f.fragment) {
				continue
			}
			t.Errorf("a workflow run can reach %s\n\tby %s\n\twhich %s",
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
	// THE HALF THAT KEEPS THE OTHER HALF HONEST.
	if !reachedMarcoexec {
		t.Fatalf("this package does not reach %s, so it has no way to act through legal "+
			"Marco at all — and the forbidden list above is then guarding a package "+
			"that could not emit anything anyway", mustReach)
	}
}
