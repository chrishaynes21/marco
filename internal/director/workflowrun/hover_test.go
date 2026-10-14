package workflowrun_test

// Did the control notice the pointer.
//
// The question exists because a live run could not answer it. A real click went to the middle of
// Rocket League's PLAY button, the menu did not change, and nothing in the record could separate
// "the point was wrong" from "the point was right and the interface ignored it". The probe is
// the only evidence available for that: the control's own pixels, while the pointer is resting on
// it and before anything is pressed.
//
// What these hold is WHEN it is asked and that it NEVER decides anything.

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

// probe records what it was asked and when, against the programs that had reached Marco by then.
type probe struct {
	asked  []observe.Region
	at     []int
	answer bool
	why    string
}

func (p *probe) ask(g *rig) func(context.Context, observe.Region) (bool, string) {
	return func(ctx context.Context, where observe.Region) (bool, string) {
		p.asked = append(p.asked, where)
		p.at = append(p.at, len(g.book.programs))
		return p.answer, p.why
	}
}

// THE CONTROL IS ASKED WHILE THE POINTER IS ON IT AND NOTHING HAS BEEN PRESSED.
//
// One program sent at the moment of asking — the move — and two by the end. Any other count
// means the probe looked at a control that had already been clicked, or at one the pointer had
// not reached, and in both cases what it measured is not what it claims to measure.
func TestTheControlIsAskedWhileThePointerRestsOnItAndBeforeThePress(t *testing.T) {
	g := newRig(t)
	p := &probe{answer: true, why: "its pixels changed by 144.1/255 over 4488 px"}
	g.run.WithDwell(time.Millisecond).WithHover(p.ask(g))

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if len(p.asked) != 1 {
		t.Fatalf("the control was asked %d time(s) for one action", len(p.asked))
	}
	if p.at[0] != 1 {
		t.Errorf("the probe was asked with %d program(s) sent; want 1, the move alone",
			p.at[0])
	}
	if len(g.book.programs) != 2 {
		t.Fatalf("%d program(s) reached Marco, want 2", len(g.book.programs))
	}
	// THE PIXELS MEASURED ARE THE PIXELS PRESSED.
	//
	// Not the generous match and not the window: the part the file and the look agree on,
	// which is the rectangle the point is the middle of. A probe over a wider region
	// averages in background that no click could have changed, and one over a narrower
	// region can miss the reaction entirely -- either way it reports about somewhere other
	// than where the button went down.
	in := p.asked[0]
	if in.Width <= 0 || in.Height <= 0 {
		t.Fatalf("the probe was given no region: %+v", in)
	}
	near := func(a, b float64) bool { return a-b < 0.0001 && b-a < 0.0001 }
	if !near(in.X+in.Width/2, got.Intent.Where.X+got.Intent.Where.Width/2) ||
		!near(in.Y+in.Height/2, got.Intent.Where.Y+got.Intent.Where.Height/2) {
		t.Errorf("the probe measured %+v and the press targeted %+v",
			in, got.Intent.Where)
	}
	// AND THE ANSWER IS CARRIED, in the words of whoever measured it.
	if !got.Hovered {
		t.Error("the control reacted and the attempt does not say so")
	}
	if got.HoverBecause != p.why {
		t.Errorf("the attempt says %q and the probe said %q", got.HoverBecause, p.why)
	}
	rep := g.run.Report()
	if rep.Hovered != 1 {
		t.Errorf("the report counts %d reaction(s) for one", rep.Hovered)
	}
	if rep.Watched != 1 {
		t.Errorf("the report says %d action(s) were looked at, and one was",
			rep.Watched)
	}
}

// A CONTROL THAT DID NOT REACT IS STILL PRESSED.
//
// A diagnosis, never a gate. Plenty of real controls do not light up, and a run that refused on
// this would be Marco deciding from pixels that it knows what an interface looks like — on the
// same evidence that, measured, read 0.0/255 for a reason that had nothing to do with the
// control.
func TestAControlThatDidNotReactIsStillPressedAndTheReportSaysSo(t *testing.T) {
	g := newRig(t)
	p := &probe{answer: false, why: "changed by 0.0/255 over 4080 px, below the 8 that counts"}
	g.run.WithDwell(time.Millisecond).WithHover(p.ask(g))

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("a control that did not react was not pressed: %s — %s",
			got.Outcome, got.Because)
	}
	if g.book.clicks() != 1 {
		t.Errorf("%d press(es) reached Marco, want 1", g.book.clicks())
	}
	if got.Hovered {
		t.Error("the attempt claims a reaction the probe refused")
	}
	if got.HoverBecause != p.why {
		t.Errorf("the attempt says %q and the probe said %q", got.HoverBecause, p.why)
	}
	// AND IT IS NOT COUNTED AS ONE THAT DID.
	rep := g.run.Report()
	if rep.Hovered != 0 {
		t.Errorf("the report counts %d reaction(s) for an action that saw none",
			rep.Hovered)
	}
	// AND IT WAS LOOKED AT, which is the whole difference from the run above.
	if rep.Watched != 1 {
		t.Errorf("the report says %d action(s) were looked at, and one was",
			rep.Watched)
	}
}

// AND NOBODY IS ASKED WHEN THERE WAS NO MOMENT TO ASK IN.
//
// With no dwell the move and the press are one batch, so there is no instant at which the
// pointer is on the control and the button is still up. A probe answered then would be
// describing pixels from before the pointer arrived or after the click landed, and reporting
// either as "the control reacted to the pointer" is a false claim, not a weak one.
func TestWithNoDwellNothingIsAskedAboutTheControlsPixels(t *testing.T) {
	g := newRig(t)
	p := &probe{answer: true, why: "SIMULATED"}
	g.run.WithDwell(0).WithHover(p.ask(g))

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if len(p.asked) != 0 {
		t.Errorf("the control was asked %d time(s) with no dwell to ask in", len(p.asked))
	}
	if got.Hovered || got.HoverBecause != "" {
		t.Errorf("the attempt reports a reaction nobody measured: %v %q",
			got.Hovered, got.HoverBecause)
	}
}

// AND A RUN WITH NO PROBE SAYS NOTHING ABOUT REACTIONS, rather than reporting none.
//
// "No control reacted" and "nobody looked" are different findings and the second one must not
// read as the first.
func TestARunWithNoProbeClaimsNothingAboutReactions(t *testing.T) {
	g := newRig(t)
	g.run.WithDwell(time.Millisecond)

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if got.HoverBecause != "" {
		t.Errorf("an unmeasured action explains a reaction: %q", got.HoverBecause)
	}
	rep := g.run.Report()
	if rep.Hovered != 0 {
		t.Errorf("the report counts %d reaction(s) with no probe installed", rep.Hovered)
	}
	// AND NOTHING WAS LOOKED AT EITHER. Two numbers, and this is the one that separates
	// them: a run that counted an unwatched action as watched would report "0 of 3
	// controls reacted", which is a finding about three controls nobody measured.
	if rep.Watched != 0 {
		t.Errorf("the report says %d action(s) were looked at with no probe installed",
			rep.Watched)
	}
}
