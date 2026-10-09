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

// TestTheActiveInterfaceIsDecidedByEvidenceNotByPlace guards the line this milestone turns on.
//
// The tracker must not learn where menus tend to be. If a coordinate, a game's name or the
// annotation's own answer reached it, "identify the active interface" would collapse into
// looking the answer up, and the measurement would mean nothing.
func TestTheActiveInterfaceIsDecidedByEvidenceNotByPlace(t *testing.T) {
	b, err := os.ReadFile("../../internal/director/perception/selection/active.go")
	if err != nil {
		t.Fatalf("reading the tracker: %v", err)
	}
	body := string(b)
	for _, forbidden := range []string{
		"0.439", "0.494", "0.550", "0.606", "0.661", "0.717", "0.772", "0.828", "0.896",
		"0.4545", "trackmania", "rocketleague", "annotation", "RESUME", "EXIT",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("active.go contains %q — the active interface must be decided by "+
				"what was observed, not by where a menu was last time", forbidden)
		}
	}
	// And the controller log must not reach it either: an input is evidence about a
	// player, not about what is drawn, and letting it label visual state would make the
	// measurement circular.
	for _, forbidden := range []string{"InputEvent", "NavIntent", "padsource", "input.jsonl"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("active.go references %q — controller observations must not "+
				"manufacture visual selection labels", forbidden)
		}
	}
}

// TestAShadedRegionKeepsItsShadingOnTheWayToASession is the other half of that link.
//
// The provider measures; this reads the measurement into the region a session sees. Between
// them is a map of `any`, which is exactly the kind of join that fails silently: a renamed
// key, a float that arrived as something else, and every interface hypothesis downstream
// believes nothing for a reason nobody can see.
func TestAShadedRegionKeepsItsShadingOnTheWayToASession(t *testing.T) {
	got := shadeOf(map[string]any{"shade_mean": 61.5, "shade_top": 204.0, "label": "x"})
	if len(got) != 2 || got[0] != 61.5 || got[1] != 204.0 {
		t.Fatalf("the measurement did not survive the attribute map: %v", got)
	}
	// A region nobody measured and a region measured as black must not look alike: a
	// zero would read as the darkest thing on screen.
	for _, attrs := range []map[string]any{
		nil,
		{},
		{"shade_mean": 61.5}, // half a measurement is not one
		{"shade_mean": "61.5", "shade_top": "204"}, // arrived as text
	} {
		if got := shadeOf(attrs); got != nil {
			t.Errorf("%v should yield no measurement, got %v", attrs, got)
		}
	}
}
