package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// The daemon's own wiring.
//
//	The Director daemon installs a working Confirmer.
//	The execution pipeline used by production matches the pipeline exercised by unit
//	tests.
//
// The previous milestone left `Runtime.SetConfirmer` as the installation point and the
// daemon installed nothing, so every action needing agreement was BLOCKED. That was safe
// and useless, and it meant the production path and the tested path diverged at exactly
// the gate that matters. These are the regressions that stop it happening again.
//
// They construct the real Runtime — not a stand-in — against a temporary MARCO_HOME, and
// they observe nothing and execute nothing: constructing a Director does not touch the
// desktop, which is what makes this checkable in CI.

// testRuntime builds the production Runtime against a scratch home.
//
// The accessibility bridge path is deliberately one that does not exist. Nothing here
// observes, so the bridge is never spawned; a test that needed it would be a live test.
func testRuntime(t *testing.T) *Runtime {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)

	g, err := actiongraph.OpenFile(filepath.Join(home, "action-graph.json"))
	if err != nil {
		t.Fatalf("opening a scratch graph: %v", err)
	}
	rt, err := NewRuntime(filepath.Join(home, "no-such-bridge.exe"), 500, true, g)
	if err != nil {
		t.Fatalf("constructing the Director: %v", err)
	}
	return rt
}

// writeFile makes a scratch file. Only ever under t.TempDir().
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

// TestTheDaemonInstallsAConfirmer is the milestone's first acceptance criterion.
func TestTheDaemonInstallsAConfirmer(t *testing.T) {
	rt := testRuntime(t)

	if rt.Confirmations() == nil {
		t.Fatal("the runtime has no confirmation broker, so the service cannot put a " +
			"question to anyone and every action needing one is blocked")
	}
	if rt.pipeline.Confirmer == nil {
		t.Fatal("the pipeline has no Confirmer, so the production path differs from the " +
			"tested path at exactly the gate that matters")
	}
	if rt.pipeline.Confirmer != rt.Confirmations() {
		t.Fatal("the pipeline's Confirmer is not the broker the service publishes " +
			"through, so a question would be asked and never delivered")
	}
}

// TestTheDaemonInstallsAResourceInspector — verification that reads the result back.
func TestTheDaemonInstallsAResourceInspector(t *testing.T) {
	rt := testRuntime(t)
	if rt.pipeline.Resources == nil {
		t.Fatal("the pipeline cannot read a renamed file back, so a rename would be " +
			"verified only by the screen having changed — which the wrong file changing " +
			"also satisfies")
	}
}

// TestTheDaemonSatisfiesTheServiceRuntime — the compile-time claim, asserted as a test so
// a missing method reads as a failure rather than as a build error somewhere else.
func TestTheDaemonSatisfiesTheServiceRuntime(t *testing.T) {
	rt := testRuntime(t)
	var _ service.Runtime = rt
	if rt.Confirmations() == nil {
		t.Fatal("a service.Runtime that reports no broker tells the server this Director " +
			"cannot ask, which blocks rather than allows — but it is not the wiring the " +
			"daemon is supposed to have")
	}
}

// TestSetConfirmerCanReplaceButNotBypass — a front-end may install a better prompt, and
// passing nil disables confirmation, which BLOCKS.
func TestSetConfirmerCanReplaceButNotBypass(t *testing.T) {
	rt := testRuntime(t)
	other := service.NewConfirmationBroker()

	rt.SetConfirmer(other)
	if rt.pipeline.Confirmer != other {
		t.Fatal("SetConfirmer did not replace the confirmer")
	}

	rt.SetConfirmer(nil)
	if rt.pipeline.Confirmer != nil {
		t.Fatal("SetConfirmer(nil) left a confirmer installed")
	}
	// Nil is "cannot ask", which the execution layer maps to unavailable → blocked.
	// There is deliberately no value that means "assume yes".
}

// ── the filesystem inspector ──────────────────────────────────────────────────

// TestTheInspectorDistinguishesAbsentFromUnreadable.
//
// A missing file is EVIDENCE about a rename; a path that could not be answered is the
// absence of evidence. Reporting the second as the first is how a verification passes
// because it could not look.
func TestTheInspectorDistinguishesAbsentFromUnreadable(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "Alpha.txt")
	if err := writeFile(present, "hello"); err != nil {
		t.Fatalf("preparing: %v", err)
	}

	insp := osResources{}

	got, known := insp.Inspect(present)
	if !known || !got.Exists {
		t.Fatalf("a file that is there was reported as %+v (known=%v)", got, known)
	}
	if got.ContentDigest == "" {
		t.Error("no content digest, so a replacement carrying the right name could not " +
			"be told from a rename")
	}

	missing, known := insp.Inspect(filepath.Join(dir, "NotThere.txt"))
	if !known {
		t.Fatal("an absent file was reported as unanswerable; that is evidence, not a gap")
	}
	if missing.Exists {
		t.Fatal("an absent file was reported as present")
	}

	if _, known := insp.Inspect(""); known {
		t.Error("an empty path was answered")
	}
}

// TestTheInspectorDigestsContentNotNames — two files with the same name and different
// contents must not look alike.
func TestTheInspectorDigestsContentNotNames(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	if err := writeFile(filepath.Join(a, "x.txt"), "one"); err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if err := writeFile(filepath.Join(b, "x.txt"), "two"); err != nil {
		t.Fatalf("preparing: %v", err)
	}

	insp := osResources{}
	first, _ := insp.Inspect(filepath.Join(a, "x.txt"))
	second, _ := insp.Inspect(filepath.Join(b, "x.txt"))

	if first.ContentDigest == second.ContentDigest {
		t.Fatal("two files with the same name and different contents digest the same")
	}

	same, _ := insp.Inspect(filepath.Join(a, "x.txt"))
	if same.ContentDigest != first.ContentDigest {
		t.Fatal("the same file digested differently twice")
	}
}
