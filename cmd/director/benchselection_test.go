package main

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// TestTheBenchReportsSelectionCoverageBesideEverySequence holds ADR-144's last claim: a
// coverage nothing prints is a coverage nobody consults.
//
// Selection coverage exists to stop a future selection score being read as a measurement over
// sequences that never said which entry was chosen. That only works if the report says so on
// the same line as the numbers — the lesson the temporal column already carries, where an
// aggregate quoted without its population let a 0/0 sequence read as a 0% detector failure for
// two milestones.
//
// Structural, for the same reason the bench's own numbers are not asserted here: the report
// runs a detector over 39 real frames and prints to stdout, so reaching it behaviourally means
// building an ONNX backend. This reads the printing site instead, which is what deleting the
// column would actually touch.
func TestTheBenchReportsSelectionCoverageBesideEverySequence(t *testing.T) {
	src, err := os.ReadFile("benchv2.go")
	if err != nil {
		t.Fatalf("reading the report's source: %v", err)
	}
	s := string(src)

	// The header, so the column is labelled rather than being a bare word in a row.
	if !strings.Contains(s, `"SEL"`) {
		t.Error("the per-sequence table has no SEL header; a coverage word printed " +
			"without a column name reads as part of SHAPE")
	}
	// The value, from the sequence's own declaration. This is the line a mutation deletes.
	if !strings.Contains(s, "corpus.Modes[seq].Selection()") {
		t.Error("the per-sequence row does not ask the sequence for its selection coverage")
	}
}

// TestUnannotatedCoveragePrintsAsAWordThatRefuses checks what that column actually says.
//
// The whole decision rests on `unannotated` being visibly different from a selection that
// exists and does not move. An empty cell, or a zero, would read as a measurement.
func TestUnannotatedCoveragePrintsAsAWordThatRefuses(t *testing.T) {
	cases := []struct {
		coverage visionbench.SelectionCoverage
		want     string
	}{
		{visionbench.SelectionUnannotated, "unannotated"},
		{visionbench.SelectionNone, "none-chosen"},
		{visionbench.SelectionStill, "still"},
		{visionbench.SelectionMoves, "moves"},
	}
	seen := map[string]bool{}
	for _, c := range cases {
		got := c.coverage.String()
		if got != c.want {
			t.Errorf("coverage %d prints %q, want %q", c.coverage, got, c.want)
		}
		if got == "" {
			t.Errorf("coverage %d prints nothing; an empty cell reads as a measurement", c.coverage)
		}
		if seen[got] {
			t.Errorf("two coverages print %q; the column must distinguish them", got)
		}
		seen[got] = true
	}
}
