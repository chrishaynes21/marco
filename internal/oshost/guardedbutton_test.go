package oshost

// WHICH BUTTON WENT DOWN, WHERE, AND WHAT STOPS IT.
//
// The saved route is checked in `internal/codegen`: the button is in the Point, beside the place,
// and the press is one sentence a chain can stop at. This file is the other end — what the
// MACHINE was asked for. A route can carry the right button and the right offset and still press
// the wrong thing, because between the file and the press there is a window to find, a
// measurement to compare, a stop that may have been raised, and a button name to translate into
// two flags.
//
// Everything from the recording to the press is production here: the simplifier, the generator,
// the registry, the compiler, the host and its resolver. The recorder and the desktop are stood
// in for, which is the only way a window can be made to change size between two instants.
//
// # The one that is not about right and middle at all
//
// `mouseButtonFlags` answers LEFT for any name it does not recognise. So a route asking for a
// button that does not exist — a hand-edited file, a typo, a term from some later recorder — sent
// a left click at the target, which in most applications is a different act entirely: one opens
// a menu and the other chooses something in it. "Could not tell which button" is not "left", and
// the cases below are where that is held.
//
// See [[ADR-260-a-press-is-a-button-and-a-place]].

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// pressesWithButtons is a recording of one press per button named, each in the game's window and
// each at its own offset, followed by the trip to Marco's window to press Stop.
//
// The offsets are the ones `guardedrun_test.go` uses, so a press can be told apart by where it
// landed as well as by which button it used — which is the point of the crossing case below.
func pressesWithButtons(buttons ...string) []recorder.RecordedEvent {
	at := []struct{ x, y, rx, ry int }{
		{842, 274, 722, 210},
		{496, 283, secondRelX, secondRelY},
	}
	out := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: when(0)},
	}
	for i, button := range buttons {
		p := at[i%len(at)]
		ms := 100 + i*800
		out = append(out,
			recorder.RecordedEvent{Kind: recorder.EvClick, Button: button,
				Down: true, X: p.x, Y: p.y, App: "arkascended", PID: 4812,
				T: when(ms), WinRel: true, RelX: p.rx, RelY: p.ry,
				RelTo: "window", WinW: 1920, WinH: 1080, Dpi: 144,
				Process: "arkascended.exe"},
			recorder.RecordedEvent{Kind: recorder.EvClick, Button: button,
				Down: false, X: p.x, Y: p.y, App: "arkascended",
				PID: 4812, T: when(ms + 5)})
	}
	return append(out,
		recorder.RecordedEvent{Kind: recorder.EvAppSwitch, KeyName: "studio", T: when(3000)},
		recorder.RecordedEvent{Kind: recorder.EvMove, X: 1500, Y: 900, T: when(3100)},
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: true,
			X: 1511, Y: 913, App: "studio", PID: 7001, T: when(3200)},
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: false,
			X: 1511, Y: 913, App: "studio", PID: 7001, T: when(3205)})
}

// A DEMONSTRATED RIGHT OR MIDDLE PRESS REACHES THE MACHINE AS THAT BUTTON, WHERE THE WINDOW IS.
//
// Both halves in one claim, because either alone is satisfied by something wrong. The right
// BUTTON at the recorded desktop coordinate is a press that ignored the window it was measured
// in; the right PLACE with a left button is the press this slice exists to stop being.
func TestADemonstratedNonLeftPressArrivesAsThatButtonWhereTheWindowIsNow(t *testing.T) {
	for _, button := range []string{"right", "middle"} {
		t.Run(button, func(t *testing.T) {
			_, at := savedFrom(t, orchestrator.ScopeAppAnywhere,
				pressesWithButtons(button))

			rec, h, out := played(t, at)

			if got := rec.clicks; len(got) != 1 || got[0] != button {
				t.Fatalf("the machine was asked for %v and the "+
					"demonstration was a %s press\n%s", got, button, out)
			}
			if len(rec.moves) != 1 || rec.moves[0] != firstPress {
				t.Fatalf("the %s press landed at %v, want the live window's "+
					"origin plus the demonstrated offset (%v)\n%s",
					button, rec.moves, firstPress, out)
			}
			if undone := h.Unperformed(); len(undone) != 0 {
				t.Fatalf("the run reports %v", undone)
			}
		})
	}
}

// AND THE BUTTON IS NOT CROSSED WITH THE TARGET.
//
// Two presses, two buttons, two places, one routine. Each assertion above passes on a run that
// pressed both presses with the first one's button, or both at the first one's point — and a
// generator that declared one Point and reused it, or a host that read the button once, would do
// exactly that. The order is part of it: a press is a button AND a place, and this is the only
// case here where getting either of them from the wrong step is visible.
func TestTheButtonIsNotCrossedWithTheTarget(t *testing.T) {
	_, at := savedFrom(t, orchestrator.ScopeAppAnywhere,
		pressesWithButtons("right", "middle"))

	rec, h, out := played(t, at)

	if got := strings.Join(rec.calls, ", "); got != "click right, click middle" {
		t.Fatalf("the machine was asked for %q, and the demonstration was a right "+
			"press and then a middle one\n%s", got, out)
	}
	if len(rec.moves) != 2 || rec.moves[0] != firstPress || rec.moves[1] != secondPress {
		t.Fatalf("the two presses landed at %v, want %v then %v\n%s", rec.moves,
			firstPress, secondPress, out)
	}
	if undone := h.Unperformed(); len(undone) != 0 {
		t.Fatalf("the run reports %v", undone)
	}
}

// AND A WINDOW OF ANOTHER SIZE REFUSES IT, SENDING NOTHING AT ALL.
//
// The same guard the left press has, asserted for these because "the press is one sentence now"
// is a claim about the generator and this is the consequence somebody is relying on: a right
// press into a window that is not the one it was shown opens a menu at an offset into something
// else. Nothing is sent, the account says which term disagreed, and the routine does not go on.
func TestANonLeftPressIntoAWindowOfAnotherSizeSendsNothing(t *testing.T) {
	_, at := savedFrom(t, orchestrator.ScopeAppAnywhere, pressesWithButtons("right"))

	was := playedOnTheDesktop
	t.Cleanup(func() { playedOnTheDesktop = was })
	playedOnTheDesktop = with(sameWindowMoved, func(f *winctx.WindowFacts) { f.W = 1600 })
	rec, h, out := played(t, at)

	if len(rec.clicks) != 0 || len(rec.moves) != 0 {
		t.Fatalf("the window was 1600 wide and the demonstration was made in one "+
			"1920 wide, and input was sent anyway: clicks=%v moves=%v\n%s",
			rec.clicks, rec.moves, out)
	}
	undone := h.Unperformed()
	if len(undone) != 1 || !strings.Contains(undone[0], "1600") {
		t.Fatalf("the run's account of what it did not do is %v, and it does not "+
			"name the window that was a different size", undone)
	}
	saysItStopped(t, out)
}

// AND ONE THE MACHINE WOULD NOT DELIVER STOPS THE ROUTINE.
//
// A guard that passes and a backend that then rejects the input are the same outcome from where
// a person is standing: the press did not happen. So the second press must never be asked for —
// a context menu that never opened has no entry to choose, and the pixel that entry would occupy
// belongs to whatever is there instead.
func TestANonLeftPressTheMachineWouldNotDeliverStopsTheRoutine(t *testing.T) {
	_, at := savedFrom(t, orchestrator.ScopeAppAnywhere,
		pressesWithButtons("right", "right"))

	rec, h, out := playedRefusingFrom(t, at, 1)

	// ONE ATTEMPT AND NO MORE, counted as SENDS rather than as clicks: a send the
	// machine rejected delivers nothing, so the list of clicks is empty either way and
	// an assertion on it would pass just as well for a run that tried both presses.
	if rec.sends != 1 {
		t.Fatalf("the machine refused the first press and %d send(s) were "+
			"attempted, so the routine went on to press after a press that "+
			"did not happen\n%s", rec.sends, out)
	}
	if len(rec.clicks) != 0 {
		t.Fatalf("the machine rejected the input and the backend recorded %v as "+
			"delivered\n%s", rec.clicks, out)
	}
	if len(h.Unperformed()) != 1 {
		t.Fatalf("the run reports %v", h.Unperformed())
	}
	saysItStopped(t, out)
}

// aPressWithButton is a guarded Point naming one button, as the generator writes one.
func aPressWithButton(button string) runtime.Value {
	s := aGuardedPress()
	s.Put("Button", runtime.Text(button))
	return runtime.SetVal(s)
}

// A BUTTON NOTHING CAN PRESS IS REFUSED, AND NOT PRESSED AS A LEFT-CLICK.
//
// `mouseButtonFlags` defaults to left, so every one of these used to be a left press at a
// correctly resolved, correctly guarded point: the guard did its whole job and the act was the
// wrong act. A route cannot be demonstrated into this state — the recorder names three buttons —
// which is exactly why it has to be held here: the shapes that reach it are a hand-edited file, a
// typo, and a term from some later recorder, and none of those arrives with a test.
//
// Both input shapes, because both name a button. The Point is the guarded form; the bare text is
// `do OS's Click with "right"`, which every legacy route uses.
func TestAPressAskingForAButtonNothingCanPressSendsNothing(t *testing.T) {
	for _, c := range []struct {
		what  string
		input runtime.Value
	}{
		{"a Point whose button is a typo", aPressWithButton("rigth")},
		{"a Point naming a button this cannot press", aPressWithButton("x1")},
		{"a legacy click whose button is a typo", runtime.Text("rigth")},
	} {
		t.Run(c.what, func(t *testing.T) {
			onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
			h, rec := aPreparedRun(t)

			st, v, _ := call(h, "Click", c.input)
			if st != "failed" {
				t.Fatalf("it answered %q, so a button nothing can press was "+
					"performed as something", st)
			}
			if len(rec.clicks) != 0 || len(rec.moves) != 0 {
				t.Fatalf("input was sent for a button nothing can press: "+
					"clicks=%v moves=%v", rec.clicks, rec.moves)
			}
			// AND THE REFUSAL NAMES THE VALUE, because the thing to change is in
			// a file and nothing else will say which word in it.
			said := ""
			if v.IsError() {
				said = v.AsError().Message
			}
			if !strings.Contains(said, "rigth") && !strings.Contains(said, "x1") {
				t.Errorf("the refusal is %q and does not name the button "+
					"that was asked for", said)
			}
			// AND IT IS IN THE ACCOUNT THE COMMAND READS. Without this the step
			// fails, the chain's arm ends the routine, and `marco do` reports
			// success about a routine that pressed nothing.
			if undone := h.Unperformed(); len(undone) != 1 {
				t.Errorf("the run's account of what it did not do is %v", undone)
			}
		})
	}
}

// AND SILENCE MEANS LEFT, which is every Point ever saved.
//
// The other side of the case above, and it has to be asserted beside it: a check that refused
// what it did not recognise would refuse the empty string too, and every routine in the owner's
// library would stop pressing.
func TestAPressThatSaysNothingAboutAButtonIsALeftPress(t *testing.T) {
	onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
	h, rec := aPreparedRun(t)

	st, _, _ := call(h, "Click", runtime.SetVal(aGuardedPress()))
	if st != "ok" {
		t.Fatalf("a point that says nothing about a button was refused: %q", st)
	}
	if len(rec.clicks) != 1 || rec.clicks[0] != "left" {
		t.Fatalf("the machine was asked for %v, want one left press", rec.clicks)
	}
}

// A STOPPED RUN PRESSES NOTHING, AND DOES NOT INTERROGATE THE DESKTOP EITHER.
//
// The press had no stop check at all: the hold has one, the keystroke has one and the drag has
// two, and a click dispatched after a run was cancelled went to the machine — the backend returns
// the cancellation AFTER the send, so it was reported as a press that failed, having happened.
//
// The second half is why the check is the first thing: everything after it reads a live desktop
// and takes time, so a stopped run that got past here would be told its window had changed, which
// is a finding about somebody's application manufactured by a stop they raised themselves.
func TestAStoppedRunDoesNotPressAndDoesNotAskTheDesktop(t *testing.T) {
	asked := 0
	wasFacts, wasImage := windowFacts, imageOf
	t.Cleanup(func() { windowFacts, imageOf = wasFacts, wasImage })
	windowFacts = func(hwnd uintptr) winctx.WindowFacts {
		asked++
		return sameWindowMoved
	}
	imageOf = func(uint32) string { return "arkascended.exe" }
	h, rec := aPreparedRun(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	st, _, _ := h.Invoke(runtime.HostCall{Action: "Click",
		Input: aPressWithButton("right"), Ctx: ctx})

	if st != "failed" {
		t.Errorf("a press dispatched after the run stopped answered %q", st)
	}
	if len(rec.clicks) != 0 || len(rec.moves) != 0 {
		t.Fatalf("a stopped run sent input: clicks=%v moves=%v", rec.clicks,
			rec.moves)
	}
	if asked != 0 {
		t.Errorf("a stopped run asked the desktop %d question(s) about a window it "+
			"was never going to press in", asked)
	}
}

// AND THE STOP IS READ AGAIN AFTER THE WINDOW WAS CHECKED.
//
// A tested mechanism is not a tested moment. The check above holds for a stop raised BEFORE the
// press was dispatched; this one is about the gap the check itself opens. Confirming the window
// reads a rectangle, a scale and a program name off a live desktop — measured at hundreds of
// milliseconds for a drag's equivalent — and a stop raised while that ran found nothing between
// its own answer and the button going down.
//
// So the stop arrives DURING the confirmation, which nothing but a stand-in desktop can arrange.
func TestAStopRaisedWhileTheWindowWasBeingCheckedPressesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	wasFacts, wasImage := windowFacts, imageOf
	t.Cleanup(func() { windowFacts, imageOf = wasFacts, wasImage })
	// THE STOP IS RAISED BY THE ANSWER ITSELF, and the answer is a HEALTHY window: the
	// press is refused for the stop and not for anything about the desktop.
	windowFacts = func(hwnd uintptr) winctx.WindowFacts {
		cancel()
		return sameWindowMoved
	}
	imageOf = func(uint32) string { return "arkascended.exe" }
	h, rec := aPreparedRun(t)

	st, _, _ := h.Invoke(runtime.HostCall{Action: "Click",
		Input: aPressWithButton("right"), Ctx: ctx})

	if st != "failed" {
		t.Errorf("a stop raised while the window was being checked answered %q", st)
	}
	if len(rec.clicks) != 0 || len(rec.moves) != 0 {
		t.Fatalf("a stop raised while the window was being checked still pressed: "+
			"clicks=%v moves=%v", rec.clicks, rec.moves)
	}
}
