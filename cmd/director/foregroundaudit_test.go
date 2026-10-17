package main

// Nothing reaches out and moves somebody's desktop except through a guarded door.
//
// # The hole in the host abstraction
//
// Every desktop EFFECT is supposed to go through the Theater and its host, which is what makes
// `DryRunHost` a safety property rather than a promise: it logs the call and returns ok, so a dry
// runtime cannot type or click.
//
// Foregrounding was outside that abstraction. `winctx.Activate` and `winctx.ActivateTitle` were
// called straight from `cmd/director`, so a runtime built with `DryRunHost` could still raise
// windows on a person's screen. Found by reading, during the Alpha 1 evaluation: with an empty
// store no Place resolves, so `disambiguateWindow` would have walked every `applicationframehost`
// window in turn — Settings, XBOX, Realtek Audio Console — raising each one, during an evaluation
// whose whole premise was that it touched nothing.
//
// # Why this walks the tree rather than naming the sites
//
// The same reason `internal/platform/navsource/pump_test.go` does: a list of call sites is a list
// somebody forgets to extend. The next person who needs to bring a window forward will reach for
// `winctx.ActivateTitle`, because that is what it is called, and nothing about the name says
// "this bypasses the host".
//
// So the rule is structural: in this package, foregrounding is `Runtime.raiseWindow` and the
// package-var seam the experiment suite substitutes. Anything else is a new hole.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// theOnlyPlacesThatMayForeground are the lines allowed to name a foregrounding call directly.
//
// Three, each for a stated reason, and each is itself guarded or substitutable:
//
//   - `raiseWindow` is THE door. It refuses when the runtime cannot perform real input.
//   - the package-var seam (`activateWindow`, `activateTitle`) exists so the experiment suite can
//     run without touching the machine it runs on; production reads these and never assigns them.
//   - `restoreDesktop` goes through that seam and takes `mayActivate`, so a dry run restores
//     nothing — because a dry run moved nothing.
var theOnlyPlacesThatMayForeground = []string{
	"return winctx.ActivateTitle(title)", // inside raiseWindow
	"activateWindow  = winctx.Activate",
	"activateTitle   = winctx.ActivateTitle",
}

// A DRY RUNTIME MOVES NOBODY'S DESKTOP.
//
// Read from the source, because what is held is the ABSENCE of a call. A behavioural test would
// need a desktop to be moved in order to notice that it was, which is the thing this exists to
// prevent happening on the machine running the suite.
func TestADryRuntimeMovesNobodysDesktop(t *testing.T) {
	for _, rel := range goFilesIn(t, ".") {
		src := readFileForAudit(t, rel)
		for i, line := range strings.Split(src, "\n") {
			code := line
			if c := strings.Index(code, "//"); c >= 0 {
				code = code[:c]
			}
			if !strings.Contains(code, "winctx.Activate") {
				continue
			}
			allowed := false
			for _, ok := range theOnlyPlacesThatMayForeground {
				if strings.Contains(strings.TrimSpace(code), ok) {
					allowed = true
				}
			}
			if !allowed {
				t.Errorf("%s:%d foregrounds a window outside the guarded door:\n"+
					"    %s\n"+
					"Foregrounding is a desktop mutation and sits OUTSIDE the host "+
					"abstraction, so a runtime built with DryRunHost can still "+
					"move somebody's screen through it. Use Runtime.raiseWindow, "+
					"which refuses when the runtime cannot perform real input.",
					rel, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// AND THE DOOR ITSELF REFUSES, which is the behaviour the rule above is protecting.
func TestRaiseWindowRefusesWhenTheRuntimeCannotPerformRealInput(t *testing.T) {
	dry := &Runtime{performsRealInput: false}
	err := dry.raiseWindow("Settings")
	if err == nil {
		t.Fatal("a dry runtime raised a window.\nThe evaluation that found this would " +
			"have walked every applicationframehost window in turn, raising each, " +
			"while reporting that it touched nothing.")
	}
	if !strings.Contains(err.Error(), "cannot perform real input") {
		t.Errorf("the refusal does not say why: %v", err)
	}
	// AND A WET ONE DOES NOT REFUSE FOR THIS REASON. It may still fail — there is no
	// window called this — but not with the dry refusal, which would mean the guard had
	// swallowed the live path too.
	wet := &Runtime{performsRealInput: true}
	if err := wet.raiseWindow("no such window exists with this title"); err != nil &&
		strings.Contains(err.Error(), "cannot perform real input") {
		t.Errorf("a runtime that CAN perform real input was refused as dry: %v", err)
	}
}

// goFilesIn is every non-test Go file in a directory.
func goFilesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	if len(out) == 0 {
		t.Fatalf("no source files found in %s, so this test is guarding nothing", dir)
	}
	return out
}

// readFileForAudit reads one source file.
func readFileForAudit(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(rel)
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(b)
}
