package workflowrun_test

// Is the pointer where it was sent.
//
// A different question from "did the control react", and the live run that forced it could not
// tell them apart. Marco moved the pointer to a menu item, the Marco runtime reported the move
// COMPLETED, and four hundred milliseconds later the operating system said the cursor was 452px
// away. The press went out anyway, at a control the pointer had never been on, and the record
// called it "the control did not react".
//
// So this one IS a gate, and what these hold is the line between the two: a measurement that
// DISAGREES stops the press; no measurement at all does not.

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

// hand answers where the pointer is, as an offset from wherever the press was aimed — which the
// granter has already been told, so the fixture never has to know the fixture's own arithmetic.
type hand struct {
	off   int
	lost  bool // nobody could ask
	asked int
	at    []int // programs that had reached Marco at each asking
}

func (h *hand) ask(g *rig) func(context.Context) (int, int, bool) {
	return func(context.Context) (int, int, bool) {
		h.asked++
		h.at = append(h.at, len(g.book.programs))
		if h.lost {
			return 0, 0, false
		}
		aimed := g.auth.asked[len(g.auth.asked)-1].At
		return aimed.X + h.off, aimed.Y + h.off, true
	}
}

// A POINTER SOMEWHERE ELSE FAILS THE ACTION, AND THE MOVE IS THE ONLY PROGRAM SENT.
//
// The count is the whole test. Two programs means the click went out through a pointer nobody
// placed, which is the defect, however the attempt describes itself afterwards.
func TestAPointerThatIsNotWhereItWasSentFailsTheActionBeforeThePress(t *testing.T) {
	g := newRig(t)
	h := &hand{off: 452}
	g.run.WithDwell(time.Millisecond).WithPointer(h.ask(g))

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomeFailed {
		t.Fatalf("a press through a displaced pointer was %s — %s",
			got.Outcome, got.Because)
	}
	if len(g.book.programs) != 1 {
		t.Fatalf("%d program(s) reached Marco; want 1, the move alone", len(g.book.programs))
	}
	if h.at[0] != 1 {
		t.Errorf("the pointer was asked about with %d program(s) sent; want 1", h.at[0])
	}
	if got.PointerArrived {
		t.Error("the pointer was 452 px away and the attempt says it arrived")
	}
	// THE MEASUREMENT IS CARRIED SEPARATELY FROM THE CONSEQUENCE. One says where the
	// pointer is, the other says what was done about it, and a report that only has the
	// second cannot be checked by somebody who doubts the first.
	if got.PointerBecause == "" {
		t.Error("nothing says where the pointer was measured")
	}
	if got.Because == got.PointerBecause {
		t.Error("the consequence and the measurement are the same sentence")
	}
	if got.Acted() {
		t.Error("the attempt claims Marco ran a program for the press")
	}
}

// AND A POINTER THAT ARRIVED IS PRESSED, with the measurement kept.
func TestAPointerThatArrivedIsPressedAndTheAttemptSaysWhereItWas(t *testing.T) {
	g := newRig(t)
	h := &hand{}
	g.run.WithDwell(time.Millisecond).WithPointer(h.ask(g))

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("a pointer that arrived was %s — %s", got.Outcome, got.Because)
	}
	if !got.PointerArrived {
		t.Error("the pointer was where it was sent and the attempt does not say so")
	}
	if got.PointerBecause == "" {
		t.Error("the attempt does not say where the pointer was measured")
	}
	if len(g.book.programs) != 2 {
		t.Fatalf("%d program(s) reached Marco; want 2", len(g.book.programs))
	}
}

// NOBODY COULD ASK IS NOT AN ANSWER.
//
// The same bound every other evidence gate in this system takes: missing evidence is not
// contradictory evidence. A platform whose cursor cannot be read must not become a platform
// where no step can be performed.
func TestAPointerNobodyCanAskAboutDoesNotStopThePress(t *testing.T) {
	g := newRig(t)
	h := &hand{lost: true}
	g.run.WithDwell(time.Millisecond).WithPointer(h.ask(g))

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("an unaskable pointer was %s — %s", got.Outcome, got.Because)
	}
	if h.asked != 1 {
		t.Errorf("the pointer was asked about %d time(s) for one action", h.asked)
	}
	// AND NOTHING IS CLAIMED. "Arrived" must stay false — the pointer was not measured
	// there, and a run that reported otherwise would be inventing the evidence this gate
	// exists to require.
	if got.PointerArrived {
		t.Error("nobody could say where the pointer was and the attempt says it arrived")
	}
	if got.PointerBecause != "" {
		t.Errorf("nobody measured anything and the attempt says %q", got.PointerBecause)
	}
}

// THE SLACK IS A ROUNDING ALLOWANCE, NOT A TOLERANCE.
//
// Pinned from both sides, because a number nothing holds drifts upwards every time a run misses
// by a little more — and a slack wide enough to absorb a real disagreement is the diagnostic
// this replaced.
func TestThePointerSlackIsFourPixelsAndNotFive(t *testing.T) {
	for _, c := range []struct {
		off  int
		want workflowrun.Outcome
	}{
		{0, workflowrun.OutcomePressed},
		{4, workflowrun.OutcomePressed},
		{5, workflowrun.OutcomeFailed},
		{306, workflowrun.OutcomeFailed},
	} {
		g := newRig(t)
		g.run.WithDwell(time.Millisecond).WithPointer((&hand{off: c.off}).ask(g))
		got := g.run.Advance(context.Background(), g.s, firstStep())
		if got.Outcome != c.want {
			t.Errorf("a pointer %d px away was %s, want %s — %s",
				c.off, got.Outcome, c.want, got.Because)
		}
	}
}

// WITHOUT A DWELL THERE IS NO MOVE, SO THERE IS NOTHING TO CHECK.
//
// A click with no dwell carries its own coordinates and never asks the pointer to go anywhere
// first. Asking where the cursor happens to be and failing the action on it would refuse every
// press on the one path where the pointer's position is genuinely irrelevant.
func TestThePointerIsNotAskedAboutWhenNothingMovedIt(t *testing.T) {
	g := newRig(t)
	h := &hand{off: 452}
	g.run.WithDwell(0).WithPointer(h.ask(g))

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("a click with no dwell was %s — %s", got.Outcome, got.Because)
	}
	if h.asked != 0 {
		t.Errorf("the pointer was asked about %d time(s) when nothing moved it", h.asked)
	}
	if got.Prepared {
		t.Error("no move was sent and the attempt says the pointer was prepared")
	}
}
