package workflowrun_test

// The pointer arrives before the button goes down.
//
// Measured, not assumed. A supervised live run put a real left click in the middle of Rocket
// League's PLAY button and the menu did not change: the Windows host sends the absolute move,
// the button-down and the button-up in ONE SendInput batch, and an interface that decides what
// is under the cursor on its own frame tick had not yet noticed the pointer was there.
//
// So an action is a move, a rest, and a press. What these hold is that it is still ONE action —
// one authorisation, one step, one entry in the budget — and that the gap the rest opens is
// guarded like every other gap in this package.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

// THE POINTER IS MOVED TO THE CONTROL, AND THEN THE CONTROL IS PRESSED.
//
// Two programs, in that order, both naming the same point. A run that sent the click alone is
// the run that was measured failing against a real game.
func TestThePointerIsMovedToTheControlBeforeItIsPressed(t *testing.T) {
	g := newRig(t)
	g.run.WithDwell(time.Millisecond)

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if !got.Prepared {
		t.Error("the attempt does not report that the pointer was placed first")
	}
	if len(g.book.programs) != 2 {
		t.Fatalf("%d program(s) reached Marco, want 2 (a move and a click):\n%s",
			len(g.book.programs), strings.Join(g.book.programs, "\n---\n"))
	}
	move, click := g.book.programs[0], g.book.programs[1]
	if !strings.Contains(move, "do OS's Move with at.") {
		t.Errorf("the first program is not a move:\n%s", move)
	}
	if strings.Contains(move, "Click") {
		t.Errorf("the first program clicks:\n%s", move)
	}
	if !strings.Contains(click, "do OS's Click with at.") {
		t.Errorf("the second program is not a click:\n%s", click)
	}
	// THE SAME POINT IN BOTH, or the pointer rests somewhere the press does not land.
	for _, p := range []string{move, click} {
		if !strings.Contains(p, "the at is a Point with X 550, Y 310.") {
			t.Errorf("the programs do not agree on the point:\n%s", p)
		}
	}
}

// AND IT IS STILL ONE ACTION.
//
// One authorisation, one step, one entry in the budget. A move that cost its own permission
// would ask somebody twice for one act; a move that counted against the budget would make
// --max-actions 1 send a move and no click.
func TestAMoveAndItsPressAreOneAction(t *testing.T) {
	g := newRig(t)
	g.run.WithDwell(time.Millisecond).WithBudget(1)

	if got := g.run.Advance(context.Background(), g.s, firstStep()); got.Outcome !=
		workflowrun.OutcomePressed {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if len(g.auth.asked) != 1 {
		t.Errorf("somebody was asked %d time(s) for one action", len(g.auth.asked))
	}
	if g.run.Sent() != 1 {
		t.Errorf("the run counted %d action(s) for one press", g.run.Sent())
	}
	if len(g.book.programs) != 2 {
		t.Errorf("%d program(s) reached Marco, want 2", len(g.book.programs))
	}
}

// AND ZERO DWELL SENDS THE CLICK ALONE.
//
// Kept reachable because it is right for a desktop control, where one atomic batch is better
// than two operations with a gap between them, and because it is what every earlier measurement
// of this subsystem was taken with.
func TestNoDwellSendsTheClickOnItsOwn(t *testing.T) {
	g := newRig(t)
	g.run.WithDwell(0)

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if got.Prepared {
		t.Error("the attempt claims the pointer was placed, with no dwell set")
	}
	if len(g.book.programs) != 1 {
		t.Fatalf("%d program(s) reached Marco, want 1", len(g.book.programs))
	}
	if strings.Contains(g.book.programs[0], "Move") {
		t.Errorf("a move was sent with no dwell:\n%s", g.book.programs[0])
	}
}

// ── the gap the rest opens ────────────────────────────────────────────────────

// A WORLD THAT MOVED DURING THE DWELL IS NOT CLICKED.
//
// The dwell is a gap like the one a person reading a prompt opens, and it gets the same
// treatment: the world is asked again on the other side of it. The pointer has already moved by
// then, which is why this refusal says so — somebody reading the report needs to know the
// cursor is sitting on a control that was never pressed.
func TestAWorldThatMovedDuringTheDwellIsNotClicked(t *testing.T) {
	g := newRig(t)
	g.run.WithDwell(time.Millisecond)
	// The fixture's confirm answers from g.moved, which the move itself flips.
	g.book.after = func() { g.moved = true }

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome == workflowrun.OutcomePressed {
		t.Fatal("a click was sent after the world moved during the dwell")
	}
	if len(g.book.programs) != 1 {
		t.Fatalf("%d program(s) reached Marco, want 1 (the move only)",
			len(g.book.programs))
	}
	if strings.Contains(g.book.programs[0], "Click") {
		t.Errorf("the one program sent was a click:\n%s", g.book.programs[0])
	}
	if !strings.Contains(got.Because, "pointer was moved") {
		t.Errorf("the refusal does not say the pointer had already moved: %s",
			got.Because)
	}
}

// AND A STOP DURING THE DWELL PREVENTS THE PRESS.
func TestAStopDuringTheDwellPreventsThePress(t *testing.T) {
	g := newRig(t)
	g.run.WithDwell(20 * time.Millisecond)
	g.book.after = func() { g.run.Stop() }

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomeStopped {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if len(g.book.programs) != 1 {
		t.Fatalf("%d program(s) reached Marco, want 1 (the move only)",
			len(g.book.programs))
	}
}

// ── did anything happen at all ────────────────────────────────────────────────

// AN OBSERVATION THAT DIFFERS FROM THE ONE THE ACTION HAPPENED IN IS A VISIBLE CHANGE.
//
// A separate question from whether the expected destination arrived, and the one the first live
// run could not answer: a report saying only "screen_b was not established" describes an
// interface that ignored the press and one that responded unrecognisably in the same words.
func TestAScreenThatChangedAfterTheActionIsReportedSeparately(t *testing.T) {
	g := newRig(t)
	g.run.WithDwell(time.Millisecond)
	ctx := context.Background()

	if got := g.run.Advance(ctx, g.s, firstStep()); got.Outcome !=
		workflowrun.OutcomePressed {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if g.run.Changed() {
		t.Error("the run reports a change before any later observation")
	}
	// A LATER OBSERVATION OF A DIFFERENT SCREEN, which nobody has grounded.
	g.run.WithGrounder(nil)
	g.run.Advance(ctx, g.s, on("now_3", 9, "control_2", rect(0.2, 0.3, 0.1, 0.04)))
	if !g.run.Changed() {
		t.Error("the screen changed after the action and the run does not say so")
	}
	if rep := g.run.Report(); !rep.Changed {
		t.Error("the report does not carry the change")
	}
}

// AND A SCREEN THAT DID NOT CHANGE IS NOT REPORTED AS ONE THAT DID.
//
// The measured case: a real click on the right control that the interface ignored.
func TestAScreenThatDidNotChangeIsNotReportedAsAChange(t *testing.T) {
	g := newRig(t)
	g.run.WithDwell(time.Millisecond)
	ctx := context.Background()

	g.run.Advance(ctx, g.s, firstStep())
	for i := 0; i < 3; i++ {
		g.run.Advance(ctx, g.s, firstStep())
	}
	if g.run.Changed() {
		t.Error("nothing changed and the run reports that something did")
	}
}

// ── what somebody is told they are authorising ────────────────────────────────

// THE QUESTION NAMES THE RECTANGLE, NOT ONLY THE POINT.
//
// A live run clicked (71,479) on one pass and (79,479) on the next, and the record could not say
// what either was the middle of — so "did it find the button, or a piece of the button" had no
// answer. A point near the left edge of a wide control and the middle of a narrow one are the
// same two numbers and completely different findings.
func TestTheQuestionNamesTheRegionThePointCameFrom(t *testing.T) {
	g := newRig(t)
	g.run.Advance(context.Background(), g.s, firstStep())
	if len(g.auth.asked) == 0 {
		t.Fatal("nobody was asked")
	}
	in := g.auth.asked[0]
	if in.Where.Width <= 0 || in.Where.Height <= 0 {
		t.Fatalf("the question carries no region: %+v", in.Where)
	}
	// THE POINT IS THE MIDDLE OF THE REGION IT NAMES, or the two lines disagree.
	midX := in.Where.X + in.Where.Width/2
	midY := in.Where.Y + in.Where.Height/2
	want := workflowrun.Desktop(observe.PointerAt{X: midX, Y: midY}, window)
	if in.At != want {
		t.Errorf("the point is %v and the middle of the region it names is %v",
			in.At, want)
	}
	got := in.Target(1920, 1080)
	for _, w := range []string{"the middle of a region", "px at"} {
		if !strings.Contains(got, w) {
			t.Errorf("want %q in: %s", w, got)
		}
	}
}

// AND A TARGET WITH NO REGION SAYS SO RATHER THAN PRINTING ZEROES.
func TestATargetWithNoRegionSaysSo(t *testing.T) {
	got := workflowrun.Intent{}.Target(1920, 1080)
	if !strings.Contains(got, "no region was recorded") {
		t.Errorf("an empty target reads as a real one: %s", got)
	}
}
