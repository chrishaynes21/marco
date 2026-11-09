package orchestrator

// A STEP CAN BE REPEATED: a key press or a block by its count, or complete steps as a new block.
//
// Through the same prepare/review/save contract as every other step edit, and preserved by the
// next edit and by a rebuild under today's rules, which renders the stored account rather than the
// demonstration. See [[ADR-313-steps-can-be-repeated]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// unguardedKeys is a routine of plain key presses with waits between, brought forward first.
func unguardedKeys() []macroir.Step {
	return []macroir.Step{
		{Kind: macroir.StepActivate, Text: "notepad"},
		{Kind: macroir.StepKey, Key: "a", Count: 1},
		{Kind: macroir.StepWait, Ms: 50},
		{Kind: macroir.StepKey, Key: "b", Count: 1},
		{Kind: macroir.StepWait, Ms: 60},
	}
}

// ── A COUNT ──────────────────────────────────────────────────────────────────

func TestAKeyPressCanHappenMoreTimes(t *testing.T) {
	out, says, why := editedFrom(typingSteps(), StepEdit{Op: EditRepeat, At: 2, Count: 3})
	if why != "" {
		t.Fatalf("it refused: %s", why)
	}
	if out[2].Count != 3 || out[2].Key != "enter" || len(out) != 4 {
		t.Errorf("the press is now %+v in %d steps", out[2], len(out))
	}
	if len(says) != 1 || !strings.Contains(says[0], "3 times") ||
		!strings.Contains(says[0], "once") {
		t.Errorf("the review says %v", says)
	}
}

func TestACountIsRefusedWhereItWouldNotBeACompleteStep(t *testing.T) {
	for _, c := range []struct {
		what  string
		steps []macroir.Step
		edit  StepEdit
		says  string
	}{
		{"half of a hold", heldSteps(), StepEdit{Op: EditRepeat, At: 1, Count: 2},
			"half a hold cannot repeat"},
		{"a step with no count", typingSteps(), StepEdit{Op: EditRepeat, At: 1, Count: 2},
			"does not repeat"},
		{"none", typingSteps(), StepEdit{Op: EditRepeat, At: 2, Count: 0}, "remove it instead"},
		{"too many", typingSteps(), StepEdit{Op: EditRepeat, At: 2, Count: 1000},
			"more than 999"},
		{"the same", typingSteps(), StepEdit{Op: EditRepeat, At: 2, Count: 1}, "already happens once"},
	} {
		t.Run(c.what, func(t *testing.T) {
			out, _, why := editedFrom(c.steps, c.edit)
			if why == "" {
				t.Fatalf("it was accepted: %+v", out)
			}
			if !strings.Contains(why, c.says) {
				t.Errorf("it was refused, saying %q", why)
			}
		})
	}
}

func TestARepeatAndARemovalOfOneStepAreRefusedInEitherOrder(t *testing.T) {
	a, b := StepEdit{Op: EditRepeat, At: 2, Count: 2}, StepEdit{Op: EditRemove, At: 2}
	for _, order := range [][]StepEdit{{a, b}, {b, a}} {
		if _, _, why := editedFrom(typingSteps(), order...); !strings.Contains(why,
			"nothing was changed") {
			t.Errorf("%s then %s: refused saying %q", order[0].Op, order[1].Op, why)
		}
	}
}

// ── A BLOCK ──────────────────────────────────────────────────────────────────

func TestCompleteStepsCanBeRepeatedAsOneBlock(t *testing.T) {
	out, says, why := editedFrom(unguardedKeys(), StepEdit{Op: EditLoop, At: 1, Upto: 4, Count: 3})
	if why != "" {
		t.Fatalf("it refused: %s", why)
	}
	if len(out) != 2 || out[1].Kind != macroir.StepLoop || out[1].Count != 3 ||
		len(out[1].Steps) != 4 || out[1].Steps[0].Key != "a" || out[1].Steps[3].Ms != 60 {
		t.Errorf("the list is %+v", out)
	}
	if len(says) != 1 || !strings.Contains(says[0], "steps 2 to 5 happen 3 times") {
		t.Errorf("the review says %v", says)
	}
	// A HOLD WHOLE INSIDE THE BLOCK IS A COMPLETE STEP.
	held := append(heldSteps()[:4:4], macroir.Step{Kind: macroir.StepWait, Ms: 5})
	if _, _, why := editedFrom(held, StepEdit{Op: EditLoop, At: 1, Upto: 3, Count: 2}); why != "" {
		t.Errorf("a block holding a whole hold was refused: %s", why)
	}
}

func TestABlockIsRefusedWhereItWouldSplitOrBreakSomething(t *testing.T) {
	guarded := []macroir.Step{
		{Kind: macroir.StepActivate, Text: "notepad"},
		{Kind: macroir.StepClick, Button: "left", X: 500, Y: 400, WinRel: true, RelTo: "window",
			RelX: 100, RelY: 80, WinW: 1280, WinH: 720, Dpi: 96, Process: "notepad.exe"},
		{Kind: macroir.StepWait, Ms: 100},
	}
	for _, c := range []struct {
		what  string
		steps []macroir.Step
		edit  StepEdit
		says  string
	}{
		{"a split hold", heldSteps(), StepEdit{Op: EditLoop, At: 1, Upto: 2, Count: 2},
			"would split the hold"},
		{"the window coming forward", unguardedKeys(),
			StepEdit{Op: EditLoop, At: 0, Upto: 2, Count: 2}, "brings notepad forward"},
		{"once", unguardedKeys(), StepEdit{Op: EditLoop, At: 1, Upto: 2, Count: 1},
			"at least twice"},
		{"a play whose presses are checked", guarded,
			StepEdit{Op: EditLoop, At: 1, Upto: 2, Count: 2}, "checks each press"},
		{"backwards", unguardedKeys(), StepEdit{Op: EditLoop, At: 3, Upto: 1, Count: 2},
			"no steps 4 to 2"},
	} {
		t.Run(c.what, func(t *testing.T) {
			out, _, why := editedFrom(c.steps, c.edit)
			if why == "" {
				t.Fatalf("it was accepted: %+v", out)
			}
			if !strings.Contains(why, c.says) {
				t.Errorf("it was refused, saying %q", why)
			}
		})
	}
	// AND IT IS REVIEWED ON ITS OWN.
	if _, _, why := editedFrom(unguardedKeys(), StepEdit{Op: EditLoop, At: 1, Upto: 2, Count: 2},
		StepEdit{Op: EditWait, At: 4, To: 9}); !strings.Contains(why, "on its own") {
		t.Errorf("a block beside another change was accepted, or refused saying %q", why)
	}
}

// ── THROUGH SAVE, THE NEXT EDIT, AND A REBUILD UNDER TODAY'S RULES ────────────

// A GUARDED ROUTINE'S KEY, THREE TIMES: three checked presses, still three after a rebuild.
func TestARepeatedKeyIsWhatTheRoutineDoesAndSurvivesARebuild(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	at := keyStepAt(t, d, rt, "esc")
	editing(t, d, rt, StepEdit{Op: EditRepeat, At: at, Count: 3})
	if got := countCalls(whatItAsked(t, d, rt, 0), "OS's Key with esc"); got != 3 {
		t.Errorf("the play pressed esc %d times, not 3", got)
	}
	p, err := d.Prepare(rt, RebuildCurrent)
	if err != nil || p.Refused != "" {
		t.Fatalf("the rebuild was refused: %v %q", err, p.Refused)
	}
	if strings.Count(p.Now, `Key with "esc"`) != 3 {
		t.Errorf("a rebuild under today's rules would not press esc three times:\n%s", p.Now)
	}
	// AND THE NEXT EDIT SEES THE COUNT, rather than one press.
	e, err := d.PrepareEdit(rt, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s := e.Steps()[at]; s.Kind != macroir.StepKey || s.Count != 3 {
		t.Errorf("the next edit sees %+v", s)
	}
}

// AN UNGUARDED ROUTINE'S KEYS AS A BLOCK, TWICE: run twice, kept by a rebuild and the next edit.
func TestARepeatedBlockIsWhatTheRoutineDoesAndSurvivesARebuild(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)
	editing(t, d, rt, StepEdit{Op: EditLoop, At: 2, Upto: 10, Count: 2})
	if got := countCalls(whatItAsked(t, d, rt, 0), "OS's Key with m"); got != 2 {
		t.Errorf("the block's first key was pressed %d times, not 2", got)
	}
	p, err := d.Prepare(rt, RebuildCurrent)
	if err != nil || p.Refused != "" {
		t.Fatalf("the rebuild was refused: %v %q", err, p.Refused)
	}
	if !strings.Contains(p.Now, "repeat 2 times") {
		t.Errorf("a rebuild under today's rules would lose the block:\n%s", p.Now)
	}
	// AND THE BLOCK'S COUNT CAN BE CHANGED BY THE NEXT EDIT.
	editing(t, d, rt, StepEdit{Op: EditRepeat, At: 2, Count: 4})
	if got := countCalls(whatItAsked(t, d, rt, 0), "OS's Key with m"); got != 4 {
		t.Errorf("after changing the block's count the key was pressed %d times, not 4", got)
	}
}

// AND A LETTER PRESSED THREE TIMES IS THREE LETTERS WHEN IT IS LATER DECLARED TYPED.
func TestARepeatedLetterIsThatManyLettersWhenLaterDeclaredTyped(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)
	editing(t, d, rt, StepEdit{Op: EditRepeat, At: keyStepAt(t, d, rt, "a"), Count: 3})
	editing(t, d, rt, StepEdit{Op: EditTyped, At: 2, Upto: 10})
	if calls := whatItAsked(t, d, rt, 0); !hasCall(calls, "OS's Type with maaarco") {
		t.Errorf("the typed run is not what the keys press: %v", calls)
	}
}

func countCalls(calls []string, call string) int {
	n := 0
	for _, c := range calls {
		if c == call {
			n++
		}
	}
	return n
}
