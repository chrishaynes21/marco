package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractBins unpacks the embedded stack to a temp dir and checks every staged file lands
// WHERE the table says, then confirms a same-version re-run is a no-op.
//
// Driven by `staged` rather than by a second list, because a hand-written expectation here would
// be a third place to forget the accessibility provider. It runs only in a packed tree; the
// invariant that survives a clean clone is TestThePackagedLayoutIsWhatMarcoLooksFor in cmd/marco,
// which reads this file as text and needs no staged assets at all.
func TestExtractBins(t *testing.T) {
	// A tree is either packed to the current list or it is not one this test can speak
	// about. Staleness is the ordinary case — an assets/ dir packed before an asset was
	// added — and pack.ps1 is where a missing asset is an error, because that is where
	// somebody can fix it before it ships.
	for _, s := range staged {
		if _, err := assets.ReadFile("assets/" + s.asset); err != nil {
			t.Skipf("assets/%s is not staged in this tree; run pack.ps1 to exercise "+
				"extraction", s.asset)
		}
	}
	dir := t.TempDir()
	if err := extractBins(dir, "v1"); err != nil {
		t.Fatal(err)
	}
	for _, s := range staged {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(s.dest))); err != nil {
			t.Errorf("missing %s: %v", s.dest, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".version")); err != nil {
		t.Errorf("missing .version: %v", err)
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
