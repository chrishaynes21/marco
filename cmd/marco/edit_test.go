package main

import (
	"strconv"
	"strings"
	"testing"
)

const editSample = `// Auto-generated. Edit freely.
use os.

the Foo is an actor.
this can Run.
this's Run does...
    the p1 is a Point with X 10, Y 20, RelX 110, RelY 220.
    the p2 is a Point with X 30, Y 40.
    do OS's Click with p1.
    do OS's Sleep with 50.
    do OS's Find with a1...
        when ok?
            do OS's Click with that.
        or?
            do OS's Click with p1.
    do OS's Sleep with 50.
    do OS's Move with p2.
    do OS's Sleep with 50.
    this is ok!
`

// parseSteps shows the TOP-LEVEL action/wait sequence — actions as labels, Sleeps as editable
// waits — and hides declarations and the nested arms of a Find block.
func TestEditParseSteps(t *testing.T) {
	e := &editor{src: editSample}
	steps := e.parseSteps()
	var kinds []string
	for _, s := range steps {
		kinds = append(kinds, s.Kind)
	}
	// Click, wait, Find (one top-level action — its nested clicks are hidden), wait, Type, wait.
	want := "action wait action wait action wait"
	if got := strings.Join(kinds, " "); got != want {
		t.Fatalf("step kinds = %q, want %q (%+v)", got, want, steps)
	}
	// The three waits carry the original 50ms.
	for _, s := range steps {
		if s.Kind == "wait" && s.Ms != 50 {
			t.Errorf("wait ms = %d, want 50", s.Ms)
		}
	}
	// The nested `do OS's Click with that.` must NOT appear as its own step.
	if len(steps) != 6 {
		t.Fatalf("got %d steps, want 6 (nested Find arms should be hidden)", len(steps))
	}
	// The Click (step 0) carries p1's coords; the Move (step 4) carries p2's.
	if steps[0].Point != "p1" || steps[0].X != 10 || steps[0].Y != 20 {
		t.Errorf("click step = %+v, want point p1 at (10,20)", steps[0])
	}
	if steps[4].Point != "p2" || steps[4].X != 30 || steps[4].Y != 40 {
		t.Errorf("move step = %+v, want point p2 at (30,40)", steps[4])
	}
}

// applyPoints rewrites a Point's X,Y and shifts its window-relative RelX,RelY by the same delta;
// a point without Rel just gets its X,Y; unedited points are untouched.
func TestEditApplyPoints(t *testing.T) {
	out := applyPoints(editSample, map[string][2]int{
		"p1": {100, 200}, // was (10,20) with Rel (110,220) → delta (+90,+180)
	})
	if !strings.Contains(out, "the p1 is a Point with X 100, Y 200, RelX 200, RelY 400.") {
		t.Errorf("p1 not rewritten with shifted Rel:\n%s", out)
	}
	if !strings.Contains(out, "the p2 is a Point with X 30, Y 40.") {
		t.Errorf("unedited p2 should be untouched:\n%s", out)
	}
	// A point with no Rel keeps just X,Y.
	out2 := applyPoints(editSample, map[string][2]int{"p2": {7, 8}})
	if !strings.Contains(out2, "the p2 is a Point with X 7, Y 8.") {
		t.Errorf("p2 (no Rel) not rewritten:\n%s", out2)
	}
}

// rebuild is the editor's save engine: it edits a wait (by line), deletes a step (by line),
// and converts a click to a drag (by line) — all in one pass, keyed by source line so a delete
// doesn't disturb the other edits.
func TestEditRebuild(t *testing.T) {
	e := &editor{src: editSample}
	steps := e.parseSteps()
	var firstWait, clickP1, moveP2 = -1, -1, -1
	for _, s := range steps {
		if s.Kind == "wait" && firstWait < 0 {
			firstWait = s.Line
		}
		if s.Point == "p1" {
			clickP1 = s.Line
		}
		if s.Point == "p2" {
			moveP2 = s.Line
		}
	}
	out := e.rebuild(saveReq{
		Waits:   map[string]int{strconv.Itoa(firstWait): 1500},
		Deletes: []int{moveP2},                                                // delete the Move step
		Drags:   map[string][4]int{strconv.Itoa(clickP1): {10, 20, 300, 400}}, // Click p1 → drag
	})
	if !strings.Contains(out, "do OS's Sleep with 1500.") {
		t.Errorf("wait not updated by line:\n%s", out)
	}
	if strings.Contains(out, "do OS's Move with p2.") {
		t.Errorf("Move step should be deleted:\n%s", out)
	}
	if !strings.Contains(out, "do OS's Drag with drag1.") ||
		!strings.Contains(out, "Drag with FromX 10, FromY 20, ToX 300, ToY 400") {
		t.Errorf("top-level Click p1 not converted to a drag:\n%s", out)
	}
	// The other two Sleeps keep their original 50 (only the first was edited).
	if strings.Count(out, "Sleep with 50.") != 2 {
		t.Errorf("unedited sleeps changed:\n%s", out)
	}
}
