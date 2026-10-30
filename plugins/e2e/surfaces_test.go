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
// This asks part of that question of the page, in the suites, with no screen and no Studio: for
// each `(surface, control)` pair the harness uses, does `rail-<surface>` exist, and is that
// control inside THAT view's section?
//
// # IT CHECKS CONTAINMENT, WHICH IS NOT VISIBILITY
//
// And the first version of the fix proved that the distinction matters: moving the pair from
// `scenes` to `routines` satisfied this guard completely, and the harness still waited for a
// control inside `captureCard`, which carries `hide` until somebody presses `Record something
// new`. An id in the document is not an element in the accessibility tree.
//
// So the claim here is deliberately narrow: the control is in the right SECTION. Whether it is on
// screen after the navigation and reveal a scenario actually performs is held by `plugins/studio`
// TestTheRecordingFormIsRevealedRatherThanWaitedFor, which renders the real page and presses the
// real controls. Both are needed: this one catches the drift that moved the control, and that one
// catches the assumption that finding it in the file means finding it on screen.
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
	{"routines", routinesProof},
}

// AND THE PAIRS ARE THE ONES THE HARNESS ACTUALLY USES.
//
// Two copies of one fact is what this file is about, so it may not introduce another. The table
// above is checked against the calls in the sources that make them.
func TestEveryNavigationWaitUsesTheNamedProof(t *testing.T) {
	// EVERY SITE, NOT JUST THE `surface` CALLS.
	//
	// The third wait was a bare `waitFor(…, "capBtn", present)` in `leaveRun`, and a
	// guard that read only `surface(` calls agreed with it completely: a mutation
	// pointing that one back at the hidden control survived. The sites are named here
	// as the EXPRESSIONS they must contain, which is how
	// `TestTheCardIdIsBuiltTheSameWayThePageBuildsIt` holds the page's own id rule.
	for _, c := range []struct{ file, expression string }{
		{"stage.go", `s.surface("routines", routinesProof)`},
		{"stage.go", `st.s.waitFor(10*time.Second, routinesProof, present)`},
		{"scenario_window.go", `s.surface("routines", routinesProof)`},
	} {
		if !strings.Contains(read(t, c.file), c.expression) {
			t.Errorf("%s no longer contains %s, so a navigation wait is naming its "+
				"own control and can drift from the one this file checks",
				c.file, c.expression)
		}
	}
	// AND NO SITE NAMES A CONTROL DIRECTLY. One place to be wrong, which is the whole
	// reason the constant exists: this has been wrong twice, and both times at more
	// than one site.
	literal := regexp.MustCompile(`surface\("[a-z]+",\s*"[A-Za-z0-9]+"\)`)
	for _, f := range []string{"stage.go", "scenario_window.go", "studio.go"} {
		if got := literal.FindString(read(t, f)); got != "" {
			t.Errorf("%s navigates with a literal control, %s — use the named proof "+
				"so there is one place for it to be wrong", f, got)
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
	case hiddenOnArrival(page, control):
		// THE SECOND WAY THIS WAS WRONG. `capBtn` is in the right section and
		// inside `captureCard`, which carries `hide` until somebody reveals the
		// recording form: an id in the document and not an element in the tree.
		return fmt.Sprintf("the harness waits for %q to prove the %s surface is up, "+
			"and the page has it inside a container that starts hidden — so the "+
			"id is in the document and not in the accessibility tree, and the "+
			"wait can only time out", control, surface)
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
		`<section id="view-routines">` +
		// A HIDDEN CONTAINER THAT CLOSES BEFORE THE CONTROL, which is what makes
		// the walk's POPPING load-bearing: a stack that never pops leaves this on
		// it and reports everything after it as hidden. A mutation removing the pop
		// survived until this was here, because the real page happens to have no
		// such container before `askBtn`.
		`<div class="hide" id="somethingElse"><span>x</span></div>` +
		`<div class="card" id="askCard">` +
		`<button id="askBtn">Find it</button></div>` +
		`<div class="card hide" id="captureCard">` +
		`<button id="capBtn">Record</button></div></section>` +
		`<section id="view-scenes"><button id="learnBtn">Learn</button></section>`

	if why := whySurfaceWrong(page, "routines", "askBtn"); why != "" {
		t.Errorf("a pair the page honours was reported wrong: %s", why)
	}
	for _, c := range []struct {
		surface, control, says string
	}{
		// THE FIRST DEFECT: the control exists, on another surface.
		{"scenes", "askBtn", "is inside"},
		// THE SECOND: the right section, inside a container that starts hidden.
		{"routines", "capBtn", "starts hidden"},
		// A SURFACE THE PAGE HAS NO RAIL FOR.
		{"history", "askBtn", "no such control"},
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

// hiddenOnArrival reports whether an element is inside a container, BELOW ITS VIEW, that the page
// starts hidden.
//
// A STACK, not a regular expression: what matters is an ANCESTOR carrying `hide`, and `capBtn`
// carries nothing itself. The page is well-formed and uses no self-closing divs, so tracking opens
// and closes is enough to know what encloses an id.
//
// # BELOW ITS VIEW, which is the whole of the distinction
//
// Every `<section id="view-...">` carries `hide`, because navigation is what reveals one — which
// is precisely what the harness does before it waits. Counting those would report every control in
// the window as hidden, and the first version of this did: it failed on `askBtn`, which the
// rendered page shows perfectly well. So the walk starts at the view's own section.
//
// It is the STATIC half of the question. A container revealed or hidden by script later is not
// visible here, which is why `plugins/studio` renders the real page and presses the real controls.
// This catches the shape that can be read off the file.
func hiddenOnArrival(page, id string) bool {
	at := strings.Index(page, fmt.Sprintf(`id="%s"`, id))
	if at < 0 {
		return false
	}
	// FROM THE VIEW DOWN. Everything above it is the window's frame, and the section
	// itself is hidden until somebody navigates to it.
	from := 0
	if opens := viewOpens.FindAllStringIndex(page[:at], -1); len(opens) > 0 {
		from = opens[len(opens)-1][1]
	}
	within := page[from:at]
	var stack []string
	for _, m := range boxTags.FindAllStringSubmatchIndex(within, -1) {
		if within[m[2]:m[3]] == "/" {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		stack = append(stack, within[m[6]:m[7]])
	}
	for _, attrs := range stack {
		for _, cls := range classAttr.FindAllStringSubmatch(attrs, -1) {
			for _, word := range strings.Fields(cls[1]) {
				if word == "hide" {
					return true
				}
			}
		}
	}
	return false
}

// The three expressions `hiddenOnArrival` reads the page with, compiled once.
var (
	viewOpens = regexp.MustCompile(`<section id="view-[^"]*"`)
	boxTags   = regexp.MustCompile(`<(/?)(div|section)\b([^>]*)>`)
	classAttr = regexp.MustCompile(`class="([^"]*)"`)
)

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
