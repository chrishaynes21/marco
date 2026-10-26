package orchestrator

// A RECORDING WHOSE GESTURE CANNOT BE WRITTEN DOWN IS NOT SAVED AS SOMETHING ELSE.
//
// # What the refusal is for
//
// Everything else in the simplifier degrades gracefully: an unreleased ordinary key becomes a
// tap, hover movement disappears, a wait is rounded. None of those changes what a routine DOES.
// Dropping a modifier does — Ctrl+click and click are different acts in most applications — so a
// routine that quietly lost the Ctrl performs a different gesture while reading as a faithful
// recording of the one that was shown.
//
// `simplify` is where that is detected and this is where it has to be ANSWERED. Two producers
// build a draft (`Capture.Finish` and `DraftOf`), both carry the reason, and `Save` is the single
// place a draft becomes a routine — so a refusal reaching the library would mean one of those
// three stopped holding, and nothing about the saved file would look wrong afterwards.
//
// See [[Demonstrations]] and [[ADR-258-a-chord-is-a-promise-to-let-go]].

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aCtrlLeftDownOverAClick is a demonstration that cannot be written down: Ctrl is pressed, a
// click is made under it, and the recording ends with Ctrl still down.
//
// A hold written from it would have no release and would leave Ctrl down on a real desktop after
// the routine finished; dropping it would perform a plain click. Neither is what was shown.
func aCtrlLeftDownOverAClick() []recorder.RecordedEvent {
	t0 := time.Now().Add(-time.Minute)
	at := func(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "ctrl", Down: true, App: "arkascended",
			PID: 4812, T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			RelTo: macroirRelToWindow, RelX: 722, RelY: 210, WinRel: true,
			WinW: 1920, WinH: 1080, Dpi: 144, Process: "arkascended.exe",
			App: "arkascended", PID: 4812, T: at(400)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: 4812, T: at(440)},
	}
}

// macroirRelToWindow is spelled out rather than imported, because this file needs one constant
// from macroir and nothing else from it.
const macroirRelToWindow = "window"

// THE DRAFT CARRIES THE REASON, AND SAVE REFUSES IT.
//
// Both halves in one test because neither is worth anything alone: a reason nobody reads is a
// comment, and a refusal with no reason is an error message somebody has to guess at.
func TestADraftWhoseGestureCannotBeWrittenDownIsRefused(t *testing.T) {
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	dr := d.DraftOf("ark graphics", aCtrlLeftDownOverAClick(), "arkascended",
		"the Stop button", Coordinates)

	if dr.Unrepresentable == "" {
		t.Fatal("the draft of a recording that leaves Ctrl down over a click says " +
			"nothing is wrong with it, so review would offer it and Save would " +
			"take it")
	}
	for _, want := range []string{"ctrl", "never released"} {
		if !strings.Contains(dr.Unrepresentable, want) {
			t.Errorf("the reason does not say %q: %s", want, dr.Unrepresentable)
		}
	}
	// AND THE STEPS ARE STILL THERE, so review can show what was understood. The person
	// performed the gesture and is the only one who can perform it again differently.
	if len(dr.Steps) == 0 {
		t.Error("a refused draft has no steps, so a window can only say that something " +
			"was wrong and not what was seen")
	}

	_, err := d.Save(dr, ScopeAppAnywhere, "arkascended")
	if err == nil {
		t.Fatal("Save wrote a routine for a gesture that cannot be written down " +
			"faithfully — so the library now holds a plain click where a " +
			"Ctrl+click was demonstrated, and nothing about the file looks wrong")
	}
	if !strings.Contains(err.Error(), "ctrl") {
		t.Errorf("the refusal from Save does not name the key: %v", err)
	}
	// AND NOTHING WAS WRITTEN. A refusal that had already saved would be a worse
	// outcome than no refusal at all, because the sentence would say the opposite of
	// what the library holds.
	if left := d.Reg.List(); len(left) != 0 {
		t.Errorf("the library holds %d routine(s) after a refused save: %v",
			len(left), left)
	}
}

// AND THE SAME DEMONSTRATION, DONE PROPERLY, SAVES.
//
// The control, and the one that matters most: this refusal sits in front of every save of every
// recording. A check that was wrong about the ordinary case would stop the product working while
// reading as a careful safeguard.
func TestAnOrdinaryCtrlClickStillSaves(t *testing.T) {
	ev := aCtrlLeftDownOverAClick()
	ev = append(ev, recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "ctrl",
		Down: false, App: "arkascended", PID: 4812,
		T: ev[len(ev)-1].T.Add(300 * time.Millisecond)})

	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	dr := d.DraftOf("ark graphics", ev, "arkascended", "the Stop button", Coordinates)
	if dr.Unrepresentable != "" {
		t.Fatalf("an ordinary Ctrl+click was refused: %s", dr.Unrepresentable)
	}
	rt, err := d.Save(dr, ScopeAppAnywhere, "arkascended")
	if err != nil {
		t.Fatalf("saving an ordinary Ctrl+click: %v", err)
	}
	src := sourceOf(t, d, rt)
	// AND IT SAVED AS THE GESTURE, which is what the whole correction is about.
	for _, want := range []string{`do OS's KeyDown with "ctrl"`, `do OS's KeyUp with "ctrl"`} {
		if !strings.Contains(src, want) {
			t.Errorf("the saved routine has no %s in it:\n%s", want, src)
		}
	}
}
