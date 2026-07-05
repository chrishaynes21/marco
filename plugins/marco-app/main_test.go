package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractBins unpacks the embedded stack to a temp dir and checks every binary lands, then
// confirms a same-version re-run is a no-op. (Runs only after pack.ps1 has staged assets/.)
func TestExtractBins(t *testing.T) {
	dir := t.TempDir()
	if err := extractBins(dir, "v1"); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"marco.exe", "marco-macros.exe", "overlay.exe", "overlay.marco", ".version"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("missing %s: %v", n, err)
		}
	}
	if err := extractBins(dir, "v1"); err != nil { // same version → skip, no error
		t.Fatal(err)
	}
}

// TestSeedHello writes the starter hello route into the global scope.
func TestSeedHello(t *testing.T) {
	dir := t.TempDir()
	if err := seedHello(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "global", "hello.marco")); err != nil {
		t.Errorf("hello route not seeded: %v", err)
	}
}
