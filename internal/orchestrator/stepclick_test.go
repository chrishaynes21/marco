package orchestrator

// A CLICK CAN BE ADDED: inside a window the routine was measured in, or at a screen point in a
// routine that checks nothing — each basis chosen explicitly, never one standing in for the other.
// See [[ADR-315-a-click-can-be-added]].

import (
	"bytes"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// A CLICK ADDED INSIDE A MEASURED WINDOW REUSES THAT MEASUREMENT AND NOTHING ELSE.
func TestAClickIsAddedInsideAWindowTheRoutineWasMeasuredIn(t *testing.T) {
	steps := checkedClickSteps()
	steps[1].AnchorClickX = 7 // a term of HOW the basis press was made, never carried
	out, says, why := editedFrom(steps, StepEdit{Op: EditInsert, At: 1, Kind: "click",
		Button: "right", BasisAt: 1, X: 200, Y: 100})
	if why != "" {
		t.Fatalf("it refused: %s", why)
	}
	c := out[2]
	if c.Kind != macroir.StepClick || c.Button != "right" || !c.WinRel {
		t.Fatalf("the added step is %+v", c)
	}
	if c.RelTo != "window" || c.Process != "notepad.exe" || c.WinW != 1280 || c.WinH != 720 ||
		c.Dpi != 96 || c.RelX != 200 || c.RelY != 100 {
		t.Errorf("the click does not carry the basis's measurement and its own offset: %+v", c)
	}
	// THE SCREEN POINT FROM THE ORIGIN THE BASIS PRESS WAS MEASURED AT (400, 320).
	if c.X != 600 || c.Y != 420 {
		t.Errorf("the click's screen point is %d, %d", c.X, c.Y)
	}
	if c.AnchorClickX != 0 || len(c.Template) != 0 || c.AnchorText != "" || c.AnchorVision != "" ||
		c.Color != "" {
		t.Errorf("the added click copied a locator from its basis: %+v", c)
	}
	if len(says) != 1 || !strings.Contains(says[0], "right-click at 200, 100 in the window "+
		"step 2 was measured in") || !strings.Contains(says[0], "600, 420 on the screen") {
		t.Errorf("the review does not say the action and its basis: %v", says)
	}
}

// A WINDOW-RELATIVE CLICK IS REFUSED WHERE THE BASIS DOES NOT HOLD IT.
func TestAWindowClickIsRefusedWhereItsBasisDoesNotHoldIt(t *testing.T) {
	anchoredOnly := checkedClickSteps()
	anchoredOnly[1].AnchorText = "Save"
	for _, c := range []struct {
		what  string
		steps []macroir.Step
		edit  StepEdit
		says  string
	}{
		{"outside the window", checkedClickSteps(), StepEdit{Op: EditInsert, At: 1,
			Kind: "click", BasisAt: 1, X: 1280, Y: 10}, "outside it"},
		{"a basis that is not a measured press", checkedClickSteps(), StepEdit{Op: EditInsert,
			At: 1, Kind: "click", BasisAt: 0, X: 10, Y: 10}, "not a press this play was measured"},
		{"no measured window at all", typingSteps(), StepEdit{Op: EditInsert, At: 1,
			Kind: "click", BasisAt: 1, X: 10, Y: 10}, "no press in this play was measured"},
		{"only a press found by its words", anchoredOnly, StepEdit{Op: EditInsert, At: 1,
			Kind: "click", BasisAt: 1, X: 10, Y: 10}, "no press in this play was measured"},
		{"not a mouse button", checkedClickSteps(), StepEdit{Op: EditInsert, At: 1,
			Kind: "click", Button: "fourth", BasisAt: 1, X: 10, Y: 10}, "not a mouse button"},
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

// A SCREEN CLICK IS FOR A ROUTINE THAT CHECKS NOTHING, AND NEVER A FALLBACK IN ONE THAT DOES.
func TestAScreenClickIsAddedOnlyToARoutineThatChecksNothing(t *testing.T) {
	out, says, why := editedFrom(typingSteps(), StepEdit{Op: EditInsert, At: 1, Kind: "click",
		Screen: true, X: 30, Y: 40})
	if why != "" {
		t.Fatalf("a screen click in a play that checks nothing was refused: %s", why)
	}
	if c := out[2]; c.Kind != macroir.StepClick || c.WinRel || c.RelTo != "" || c.X != 30 ||
		c.Button != "left" {
		t.Errorf("the added click is %+v", c)
	}
	if !strings.Contains(says[0], "left-click at 30, 40 on the screen") {
		t.Errorf("the review says %v", says)
	}
	_, _, why = editedFrom(checkedClickSteps(), StepEdit{Op: EditInsert, At: 1, Kind: "click",
		Screen: true, X: 30, Y: 40})
	if !strings.Contains(why, "checks each press against its window") {
		t.Errorf("a screen click in a checked play was accepted, or refused saying %q", why)
	}
}

// ONE BASIS PER WINDOW AND ORIGIN, from presses measured in full.
func TestTheBasesAreTheDistinctWindowsTheRoutineWasMeasuredIn(t *testing.T) {
	steps := checkedClickSteps()
	same := steps[1]
	same.RelX, same.X = 300, 700 // same window, same origin
	moved := steps[1]
	moved.X = 900 // same offset, the window was somewhere else
	steps = append(steps, same, moved)
	bases := ClickBases(steps)
	if len(bases) != 2 || bases[0].At != 1 || bases[1].At != 4 {
		t.Errorf("the bases are %+v", bases)
	}
	if len(ClickBases(typingSteps())) != 0 {
		t.Error("a play with no measured press offers a basis")
	}
}

// ── THROUGH SAVE, THE DRIVER, THE NEXT EDIT AND A REBUILD ────────────────────

// ADDED INSIDE THE WINDOW OF ONE OF ITS OWN PRESSES: performed checked against that window, the
// demonstration byte-for-byte as it was, the click in the stored account, kept by a rebuild.
func TestAClickAddedInsideAWindowIsWhatTheRoutineDoes(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	demo, ok := d.Reg.LoadRecording(rt)
	if !ok {
		t.Fatal("the fixture has no demonstration")
	}
	basis := firstStepOf(t, d, rt, macroir.StepClick)
	before := len(whatItAsked(t, d, rt, 0))
	editing(t, d, rt, StepEdit{Op: EditInsert, At: basis, Kind: "click", BasisAt: basis,
		X: 15, Y: 25})
	calls := whatItAsked(t, d, rt, 0)
	if len(calls) != before+1 {
		t.Errorf("the play made %d calls, %d before and one added", len(calls), before)
	}
	added := ""
	for _, c := range calls {
		if strings.HasPrefix(c, "OS's Click with") && strings.Contains(c, "RelX=15") {
			added = c
		}
	}
	if added == "" || !strings.Contains(added, "arkascended") {
		t.Errorf("no click was performed at 15, 25 checked against the play's window: %v",
			calls)
	}
	if after, _ := d.Reg.LoadRecording(rt); !bytes.Equal(after, demo) {
		t.Error("adding a click changed the demonstration")
	}
	steps := mustSteps(t, d, rt)
	if s := steps[basis+1]; s.Kind != macroir.StepClick || s.RelX != 15 || s.RelY != 25 {
		t.Errorf("the next edit does not see the added click: %+v", s)
	}
	rb, err := d.Prepare(rt, RebuildCurrent)
	if err != nil || rb.Refused != "" {
		t.Fatalf("the rebuild was refused: %v %q", err, rb.Refused)
	}
	if !strings.Contains(rb.Now, "RelX 15") {
		t.Errorf("a rebuild under today's rules would lose the added click:\n%s", rb.Now)
	}
}

// ADDED AT A SCREEN POINT IN A ROUTINE THAT CHECKS NOTHING: performed there.
func TestAScreenClickAddedToAnUncheckedRoutineIsWhatItDoes(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)
	editing(t, d, rt, StepEdit{Op: EditInsert, At: 2, Kind: "click", Screen: true, X: 11, Y: 22})
	found := false
	for _, c := range whatItAsked(t, d, rt, 0) {
		if strings.HasPrefix(c, "OS's Click with") && strings.Contains(c, "X=11") &&
			strings.Contains(c, "Y=22") {
			found = true
		}
	}
	if !found {
		t.Error("the screen click was not performed at 11, 22")
	}
}

// AND A SIDE BUTTON CAN BE CHOSEN for an added click, as the recorder can now record one: every
// button macroir.MouseButtons names is accepted here, the side buttons among them.
//
// Refusing back or forward here must fail this.
func TestAnAddedClickCanUseEveryButtonMarcoPresses(t *testing.T) {
	for _, b := range macroir.MouseButtons() {
		out, _, why := editedFrom(checkedClickSteps(), StepEdit{Op: EditInsert, At: 1,
			Kind: "click", Button: b, BasisAt: 1, X: 200, Y: 100})
		if why != "" {
			t.Errorf("an added %s click was refused: %s", b, why)
			continue
		}
		if out[2].Button != b && !(b == "left" && out[2].Button == "") {
			t.Errorf("an added %s click came out as the %q button", b, out[2].Button)
		}
	}
	if len(macroir.MouseButtons()) != 5 {
		t.Errorf("Marco presses %v; the side buttons are missing", macroir.MouseButtons())
	}
}

// A TURN OF THE WHEEL SENDS INPUT, so a recording of nothing but a scroll is worth saving.
//
// Leaving the scroll out of what counts must fail this.
func TestAScrollIsAPressableStep(t *testing.T) {
	d := &Draft{Steps: []macroir.Step{{Kind: macroir.StepScroll, X: 1, Y: 2, Wheel: "down", Count: 1}}}
	if n := d.Pressable(); n != 1 {
		t.Errorf("a draft that scrolls counts %d steps that send input, want 1", n)
	}
}
