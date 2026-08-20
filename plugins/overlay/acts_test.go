package main

import (
	"strings"
	"testing"
)

// TestNoPlayMatchesPrefix pins the engine's unknown-command error prefix.
//
// The producers live in ANOTHER Go module — cmd/marco/panicstop.go and
// cmd/marco/bind.go in the repo root — so no compiler, and no cross-module test,
// notices when one side is reworded. The consequence of a silent drift is quiet:
// the raw engine error is logged to the HUD alongside the learn offer instead of
// being suppressed. This constant and theirs must be changed together.
func TestNoPlayMatchesPrefix(t *testing.T) {
	if noPlayMatches != "no play matches " {
		t.Fatalf("prefix drifted from the engine's literal: %q", noPlayMatches)
	}
	// The match is a prefix test against a whole error line, so a missing trailing
	// space would match lines the engine never meant to be suppressed.
	if !strings.HasSuffix(noPlayMatches, " ") {
		t.Fatalf("prefix must end in a space: %q", noPlayMatches)
	}
	line := noPlayMatches + `"open the thing" — try: marco learn open the thing`
	if !strings.HasPrefix(line, noPlayMatches) || !strings.Contains(line, "marco learn") {
		t.Fatalf("suppression condition no longer matches a producer line: %q", line)
	}
}
