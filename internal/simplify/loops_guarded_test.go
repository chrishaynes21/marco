package simplify

// A PRESS THAT ASKED TO BE CHECKED IS NEVER FOLDED INTO A REPEAT.
//
// `repeat 3 times... do OS's Click with p1.` says: press it, then press it again, then again. A
// press that checks the window it was recorded in can REFUSE, and a refusal has to stop the
// routine — which the generated route says by giving every such press its own capability that
// calls the next only when its own press went through. A repeat has no way to say that: the body
// would run again after a press that did not happen.
//
// So three identical guarded presses stay three steps. It costs a longer file and buys the one
// property the whole contract is about.

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// fourIdenticalPresses is the same button pressed four times — the shape `foldLoops` exists
// for, with `measured` deciding whether each press carries the window it was made in.
func fourIdenticalPresses(measured bool) []recorder.RecordedEvent {
	at := func(ms int) time.Time {
		return time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC).
			Add(time.Duration(ms) * time.Millisecond)
	}
	var out []recorder.RecordedEvent
	for i := 0; i < 4; i++ {
		down := recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left",
			Down: true, X: 842, Y: 274, T: at(i * 400)}
		if measured {
			down.WinRel, down.RelX, down.RelY = true, 722, 210
			down.RelTo = macroir.RelToWindow
			down.WinW, down.WinH, down.Dpi = 1920, 1080, 144
			down.Process = "arkascended.exe"
		}
		up := down
		up.Down, up.T = false, at(i*400+5)
		out = append(out, down, up)
	}
	return out
}

func countKinds(steps []macroir.Step) (clicks, loops int) {
	for _, s := range steps {
		switch s.Kind {
		case macroir.StepClick:
			clicks++
		case macroir.StepLoop:
			loops++
		}
	}
	return clicks, loops
}

func TestGuardedPressesAreNotFoldedIntoARepeat(t *testing.T) {
	steps := Simplify(fourIdenticalPresses(true), Options{FoldLoops: true, MinLoopReps: 2})

	clicks, loops := countKinds(steps)
	if loops != 0 {
		t.Errorf("%d repeat(s) over presses that each have to be allowed: %+v",
			loops, steps)
	}
	if clicks != 4 {
		t.Errorf("%d press(es) survived of four: %+v", clicks, steps)
	}
}

// AND AN UNMEASURED REPEAT IS STILL FOLDED.
//
// The control, and it is what keeps the rule confined: a routine whose presses asked to be
// checked against nothing reads the way it always did, repeats included.
func TestUnmeasuredPressesAreStillFolded(t *testing.T) {
	steps := Simplify(fourIdenticalPresses(false), Options{FoldLoops: true, MinLoopReps: 2})

	_, loops := countKinds(steps)
	if loops != 1 {
		t.Errorf("%d repeat(s) over four identical presses that check nothing, "+
			"want 1: %+v", loops, steps)
	}
}
