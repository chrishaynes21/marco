package main

// EVERY SURFACE THIS HARNESS NAVIGATES TO HOLDS THE CONTROL IT WAITS FOR.
//
// # The defect this closes
//
// `stage.go` and `scenario_window.go` asked for the `scenes` surface and waited for `capBtn`, and
// then chose a routine card. That worked while Studio's recording card and its routine list were
// one surface. They are not: the three demonstrate-and-watch tools were given their own place, so
// `rail-scenes` shows `learnName`/`learnApp`/`learnBtn` and `capBtn` moved under `rail-routines`
// with the cards.
//
// EVERY REGISTERED SCENARIO THEN FAILED, identically, in **stage 6 of 6** — twenty minutes into a
// gate run, and only with a screen. The sentence it produced was *the scenes surface did not come
// up: capBtn never became ready*, which is a harness describing a window that was working.
//
// This asks the same question of the page, in the suites, with no screen and no Studio: for each
// `(surface, control)` pair the harness uses, does `rail-<surface>` exist, and is that control
// inside THAT view's section?
//
// It is the `cardID` guard's argument one step further. That one holds the harness's copy of an
// id-building rule against the page's; this holds the harness's idea of WHERE A CONTROL IS. Both
// are two copies of one fact, which is the shape this repository keeps finding.
//
// See [[Alpha-candidate-2026-10-29]].

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// harnessSurfaces is every `(surface, control)` pair this harness waits on.
//
// Written down rather than discovered, and HELD AGAINST THE SOURCE below: a list that drifted
// from the calls would make this test pass about pairs nobody uses. The alternative — parsing Go
// for `surface(` calls — is a second parser to maintain for four lines of table.
var harnessSurfaces = []struct{ surface, control string }{
	{"routines", "capBtn"},
}

// AND THE PAIRS ARE THE ONES THE HARNESS ACTUALLY USES.
//
// Two copies of one fact is what this file is about, so it may not introduce another. The table
// above is checked against the calls in the sources that make them.
func TestTheSurfaceTableIsThePairsTheHarnessUses(t *testing.T) {
	calls := regexp.MustCompile(`surface\("([a-z]+)",\s*"([A-Za-z0-9]+)"\)`)
	seen := map[string]bool{}
	for _, f := range []string{"stage.go", "scenario_window.go", "studio.go"} {
		for _, m := range calls.FindAllStringSubmatch(read(t, f), -1) {
			seen[m[1]+"/"+m[2]] = true
		}
	}
	if len(seen) == 0 {
		t.Fatal("no `surface(\"x\", \"y\")` call was found in the harness, so this " +
			"test is checking a table against nothing")
	}
	listed := map[string]bool{}
	for _, p := range harnessSurfaces {
		listed[p.surface+"/"+p.control] = true
	}
	for pair := range seen {
		if !listed[pair] {
			t.Errorf("the harness navigates to %q and nothing here checks that the "+
				"control is on that surface — add it to harnessSurfaces", pair)
		}
	}
	for pair := range listed {
		if !seen[pair] {
			t.Errorf("harnessSurfaces lists %q and no call uses it, so this file "+
				"is asserting about a pair nobody navigates to", pair)
		}
	}
}

// AND EACH PAIR IS A REAL VIEW HOLDING A REAL CONTROL.
//
// The claim the scenarios rest on, asked of the page: the rail button exists, the view exists,
// and the control the harness waits for is inside that view rather than another one.
func TestEveryHarnessSurfaceHoldsTheControlItWaitsFor(t *testing.T) {
	page := read(t, filepath.Join("..", "studio", "frontend", "index.html"))
	for _, p := range harnessSurfaces {
		if why := whySurfaceWrong(page, p.surface, p.control); why != "" {
			t.Error(why)
		}
	}
}

// whySurfaceWrong is the rule itself, over one page and one pair.
//
// A FUNCTION SO IT CAN BE EXERCISED AGAINST A BROKEN PAGE. Mutations that deleted either half of
// this survived while it was inline: with the tree correct, removing a check changes nothing, and
// a guard rule cannot be falsified from a tree that satisfies it. The same answer the unix-shell
// guard in `internal/wiringcheck` came to.
//
// Returns "" when the harness's pair is one the page can honour.
func whySurfaceWrong(page, surface, control string) string {
	rail := fmt.Sprintf(`id="rail-%s"`, surface)
	if !strings.Contains(page, rail) {
		return fmt.Sprintf("the harness presses %s and the page has no such control",
			rail)
	}
	view, ok := sectionHolding(page, fmt.Sprintf(`id="%s"`, control))
	switch {
	case !ok:
		return fmt.Sprintf("the harness waits for %q on the %s surface and the page "+
			"has no control with that id at all", control, surface)
	case view != "view-"+surface:
		// THE EXACT SENTENCE THE SCENARIOS DIED OF, twenty minutes earlier and
		// without a screen.
		return fmt.Sprintf("the harness waits for %q after pressing rail-%s, and %q "+
			"is inside %q, so every scenario reports that the %s surface did "+
			"not come up — about a window that is working", control, surface,
			control, view, surface)
	}
	return ""
}

// AND THE RULE IS HELD WITH THE PAGES IT IS ABOUT.
//
// Three ways a pair can be wrong and one way it can be right, over pages crafted here. Without
// this, deleting either check above passes: the tree is correct, so a checker that answers
// "nothing wrong" to everything agrees with it.
func TestTheSurfaceRuleCatchesEachWayAPairCanBeWrong(t *testing.T) {
	const page = `<nav><button id="rail-routines">Routines</button>` +
		`<button id="rail-scenes">Scenes</button></nav>` +
		`<section id="view-routines"><button id="capBtn">Record</button></section>` +
		`<section id="view-scenes"><button id="learnBtn">Learn</button></section>`

	if why := whySurfaceWrong(page, "routines", "capBtn"); why != "" {
		t.Errorf("a pair the page honours was reported wrong: %s", why)
	}
	for _, c := range []struct {
		surface, control, says string
	}{
		// THE DEFECT ITSELF: the control exists, on another surface.
		{"scenes", "capBtn", "is inside"},
		// A SURFACE THE PAGE HAS NO RAIL FOR.
		{"history", "capBtn", "no such control"},
		// A CONTROL THE PAGE DOES NOT HAVE AT ALL.
		{"routines", "goneBtn", "no control with that id"},
	} {
		why := whySurfaceWrong(page, c.surface, c.control)
		if why == "" {
			t.Errorf("waiting for %q on the %s surface was accepted", c.control,
				c.surface)
			continue
		}
		if !strings.Contains(why, c.says) {
			t.Errorf("the refusal for %s/%s reads %q, which does not say %q",
				c.surface, c.control, why, c.says)
		}
	}
}

// sectionHolding is the `<section id="…">` an attribute sits inside, by position.
//
// Positional rather than structural on purpose: the page is one file of flat sections, and a real
// HTML parser here would be a dependency this module does not have for a question this simple.
// It reports the LAST section opened before the attribute, which is that section as long as
// sections do not nest — and `TestTheViewsAreFlatSections` holds that.
func sectionHolding(page, attr string) (string, bool) {
	at := strings.Index(page, attr)
	if at < 0 {
		return "", false
	}
	ids := regexp.MustCompile(`<section id="([^"]+)"`).FindAllStringSubmatch(page[:at], -1)
	if len(ids) == 0 {
		return "", false
	}
	return ids[len(ids)-1][1], true
}

// AND THE SECTIONS DO NOT NEST, which is what makes the reading above correct.
//
// A nested `<section>` would make "the last one opened" the inner one, and a control in the outer
// section would be reported as being in the inner — a wrong answer that looks like a right one.
func TestTheViewsAreFlatSections(t *testing.T) {
	page := read(t, filepath.Join("..", "studio", "frontend", "index.html"))
	depth, worst := 0, 0
	for _, tok := range regexp.MustCompile(`</?section`).FindAllString(page, -1) {
		if tok == "<section" {
			depth++
			if depth > worst {
				worst = depth
			}
		} else {
			depth--
		}
	}
	if worst > 1 {
		t.Errorf("the page nests sections %d deep, so reading a control's view as "+
			"the last section opened before it gives the inner one", worst)
	}
	if depth != 0 {
		t.Errorf("the page opens and closes an unequal number of sections (%d left "+
			"open), so nothing here can say which view a control is in", depth)
	}
}
