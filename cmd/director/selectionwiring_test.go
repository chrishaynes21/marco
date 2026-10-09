package main

import (
	"os"
	"strings"
	"testing"
)

// TestTheSelectionReaderIsReachable is the wiring claim for read-selection.
//
// This repository has eight recorded cases of complete, correct code that production never
// invoked, and a reader with its own passing unit tests and no way to run it over a real
// session would be the ninth. The package's tests prove the rules; this proves there is a
// door.
func TestTheSelectionReaderIsReachable(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading the dispatch: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, `case "read-selection":`) {
		t.Error("main.go dispatches no read-selection verb")
	}
	if !strings.Contains(s, "runReadSelection(os.Args[2:])") {
		t.Error("the read-selection verb does not call the command")
	}
}

// TestSuppliedRowsAreHarnessOnly holds the line the experiment's instructions drew.
//
// --rows exists so that "the reader cannot tell which row is chosen" can be told apart from
// "the band finder could not find the rows" — a distinction one accuracy number destroys. It
// is an operator's input to a diagnostic. If those coordinates ever reach the reader, the
// package stops being general and starts being a description of one menu.
func TestSuppliedRowsAreHarnessOnly(t *testing.T) {
	cmd, err := os.ReadFile("selectionread.go")
	if err != nil {
		t.Fatalf("reading the command: %v", err)
	}
	if !strings.Contains(string(cmd), "harness only") {
		t.Error("--rows must say in its own help that it is harness input")
	}

	// The reader's package may not contain a coordinate from either measured game.
	for _, f := range []string{
		"../../internal/director/perception/selection/selection.go",
		"../../internal/director/perception/selection/rows.go",
	} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		body := string(b)
		for _, forbidden := range []string{
			"0.439", "0.494", "0.550", "0.606", "0.661", "0.717", "0.772", "0.828", "0.896",
			"0.4545", "0.414", "menu_resume", "trackmania", "rocketleague",
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s contains %q — a measured game's geometry or vocabulary "+
					"must not reach the reader", f, forbidden)
			}
		}
	}
}
