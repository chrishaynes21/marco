package main

import (
	"os"
	"strings"
	"testing"
)

// The challenger is a benchmark backend and nothing else.
//
// A model under evaluation must not be able to influence a live decision — that would make
// the evaluation an experiment running on the user. The rule is easy to state and easy to
// break by accident, so it is checked against the source rather than trusted.

// runtimeFiles are the files that compose the LIVE Director.
var runtimeFiles = []string{
	"runtime.go", "observewiring.go", "observesnapshot.go", "observeregistry.go",
	"visionwiring.go", "ocrwiring.go", "gamewiring.go", "actionwiring.go",
	"editwiring.go", "lowerwiring.go", "tracewiring.go", "waitwiring.go",
	"visualwiring.go", "demowiring.go", "serve.go",
}

func TestTheChallengerIsNotInAnyRuntimeComposition(t *testing.T) {
	for _, name := range runtimeFiles {
		src, err := os.ReadFile(name)
		if err != nil {
			continue // not every file exists in every build
		}
		text := string(src)
		for _, forbidden := range []string{
			"GroundingDINO", "grounding-dino", "newChallenger",
		} {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s references %q; the challenger must reach the benchmark and "+
					"nothing else, or an experimental model could affect a live decision",
					name, forbidden)
			}
		}
	}
}

func TestTheChallengerIsConstructedInExactlyOnePlace(t *testing.T) {
	// One construction site means one thing to audit. Several would make "it is only a
	// benchmark backend" a claim nobody could check quickly.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var sites []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") ||
			strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(e.Name())
		if err != nil {
			continue
		}
		if strings.Contains(string(src), "visionbench.NewGroundingDINO(") {
			sites = append(sites, e.Name())
		}
	}
	if len(sites) != 1 {
		t.Fatalf("the challenger is constructed in %v; want exactly one place", sites)
	}
	if sites[0] != "benchcmd.go" {
		t.Errorf("constructed in %s, want the benchmark command", sites[0])
	}
}

func TestTheBenchmarkPackageIsNotImportedByTheRuntime(t *testing.T) {
	for _, name := range runtimeFiles {
		src, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		if strings.Contains(string(src), "director/visionbench") {
			t.Errorf("%s imports the benchmark package; benchmarking must not be reachable "+
				"from the live perception path", name)
		}
	}
}
