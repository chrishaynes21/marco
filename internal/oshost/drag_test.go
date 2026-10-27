package oshost

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// fakePointer is the machine as a log, so a claim about a GESTURE can be made about an order of
// events rather than about a summary of them. Which is the whole reason `dragGesture` moved out
// of the function that calls SendInput: the three defects it had were all about WHICH event was
// sent WHEN, and not one of them was reachable while the order lived next to the syscall.
type fakePointer struct {
	events []string
	// refuse names the events, by position (1-based), that the machine will not take —
	// a refused press, a refused move partway and a refused release are three different
	// outcomes and the old code could not tell any of them from success.
	//
	// ONE EVENT AT A TIME AND NOT "FROM THE NTH ONWARDS", which this started as. A
	// machine refusing everything from the fourth event refuses the RELEASE too, and a
	// release that fails produces an error of its own — so a test about a refused move
	// still saw a failure and a released button whether or not the refused move was
	// noticed at all. It passed with the fix mutated out.
	refuse map[int]bool
	// stopAfter cancels the run after the nth event, which is how a stop lands DURING the
	// travel rather than before it or after it.
	stopAfter int
	cancel    context.CancelFunc
}

func (f *fakePointer) took(kind string, x, y int) bool {
	f.events = append(f.events, fmt.Sprintf("%s %d,%d", kind, x, y))
	if f.stopAfter > 0 && len(f.events) == f.stopAfter && f.cancel != nil {
		f.cancel()
	}
	return !f.refuse[len(f.events)]
}

// moves is every point the travel actually reached, which is what says whether a refused move
// ENDED the travel or was shrugged off.
func (f *fakePointer) moves() []string {
	var out []string
	for _, e := range f.events {
		if strings.HasPrefix(e, "move ") {
			out = append(out, e)
		}
	}
	return out
}

func (f *fakePointer) moveTo(x, y int) bool                  { return f.took("move", x, y) }
func (f *fakePointer) press(b string, x, y int) bool         { return f.took(b+" down", x, y) }
func (f *fakePointer) release(b string, x, y int) bool       { return f.took(b+" up", x, y) }
func (f *fakePointer) settle(context.Context, time.Duration) {}

// last is the final event, which for a drag is the one that decides where the thing being
// dragged was DROPPED.
func (f *fakePointer) last() string {
	if len(f.events) == 0 {
		return ""
	}
	return f.events[len(f.events)-1]
}

// TestAStoppedDragLetsGoWhereThePointerIs is the first defect, and it is about a place.
//
// The old loop read the context at the top and broke out of it — and the two lines after the
// loop then sent the landing move and the release anyway. Stopping a drag therefore let go of the
// button, which is right, at the DESTINATION, which is not: the thing being dragged was dropped
// where the routine was going rather than where the pointer had got to when somebody pressed
// Escape. A stop that still delivers the gesture's whole point is not a stop.
func TestAStoppedDragLetsGoWhereThePointerIs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Stopped after the aim, the press and three moves, so the pointer is somewhere in
	// the middle of a long travel and that somewhere is not either end.
	p := &fakePointer{stopAfter: 5, cancel: cancel}
	err := dragGesture(ctx, p, "left", 0, 0, 240, 0, dragHost{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a stopped drag reported %v, want a cancellation", err)
	}
	where := p.events[len(p.events)-2] // the last move before the release
	if got, want := p.last(), strings.Replace(where, "move", "left up", 1); got != want {
		t.Errorf("the drag was stopped after %q and released with %q; it must let go "+
			"WHERE THE POINTER IS (%q), because a release at the destination is a "+
			"drop and a drop is the one thing a stopped drag must not do.\nevents: %v",
			where, got, want, p.events)
	}
	for _, e := range p.events {
		if e == "move 240,0" {
			t.Errorf("the drag was stopped and still sent its landing move to the "+
				"destination.\nevents: %v", p.events)
		}
	}
}

// TestADragDispatchedAfterAStopPressesNothing is the second defect: there was no entrance check.
//
// Nothing read the context before the press, so a drag reached with the run already stopped
// pressed the button and dragged regardless, and found the cancellation at the top of the travel
// loop — by which time the press had happened. `sendChord` needed exactly this and has it.
func TestADragDispatchedAfterAStopPressesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := &fakePointer{}
	if err := dragGesture(ctx, p, "left", 10, 10, 90, 90, dragHost{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("a drag dispatched after a stop reported %v, want a cancellation", err)
	}
	if len(p.events) != 0 {
		t.Errorf("a drag dispatched after the run had stopped sent %v; a stopped run "+
			"presses NOTHING, and a press is the event that cannot be taken back",
			p.events)
	}
}

// TestADragTheMachineRefusesIsNotReportedAsDone is the third defect, at the gesture.
//
// The send closure ignored what SendInput returned and the function returned only the context's
// error, so a refused press, a refused move and a refused release were indistinguishable from a
// gesture that worked. Each of the three is asked for separately here, because "the drag failed"
// is not the useful part: WHICH event the machine would not take is what tells a person whether
// anything moved and whether a button is still down.
func TestADragTheMachineRefusesIsNotReportedAsDone(t *testing.T) {
	t.Run("a refused press", func(t *testing.T) {
		p := &fakePointer{refuse: map[int]bool{2: true}} // the aim lands, not the press
		err := dragGesture(context.Background(), p, "left", 5, 5, 50, 50, dragHost{})
		if err == nil {
			t.Fatal("the machine refused the press and the gesture reported success")
		}
		// AND NO RELEASE FOR A BUTTON THAT IS NOT DOWN: a rejected press leaves
		// nothing to undo, and releasing anyway would report two failures for one.
		for _, e := range p.events {
			if strings.Contains(e, "up") {
				t.Errorf("the press was refused and the gesture released "+
					"anyway.\nevents: %v", p.events)
			}
		}
	})

	t.Run("a refused move partway", func(t *testing.T) {
		// The FOURTH event: the aim, the press, one move that lands, then one the
		// machine will not take. Only that one, so nothing else in the gesture can
		// produce the failure this is asking about.
		p := &fakePointer{refuse: map[int]bool{4: true}}
		err := dragGesture(context.Background(), p, "left", 0, 0, 240, 0, dragHost{})
		if err == nil {
			t.Fatal("the machine refused a move mid-drag and the gesture reported " +
				"success")
		}
		// AND THE TRAVEL ENDS THERE. A gesture that did not go where it said it
		// went must not carry on through a machine that is refusing input and
		// finish by dropping something somewhere nobody chose. Three moves were
		// asked for and no more: the aim, the one that landed, the one that did
		// not.
		want := []string{"move 0,0", "move 10,0", "move 20,0"}
		if got := p.moves(); !slices.Equal(got, want) {
			t.Errorf("a move was refused mid-drag and the gesture asked for %v; "+
				"the travel ends where the machine stopped taking it, so "+
				"the moves are %v and nothing after them", got, want)
		}
		// AND THE BUTTON STILL COMES UP, where the pointer actually got to — it is
		// DOWN, and down is the state somebody is left with.
		if p.last() != "left up 10,0" {
			t.Errorf("a move was refused mid-drag and the last event was %q, want "+
				"the release at the last point the pointer reached.\nevents: "+
				"%v", p.last(), p.events)
		}
	})

	t.Run("a refused release", func(t *testing.T) {
		// The release is the LAST of the gesture's events, and it is the only one
		// refused here: refusing the landing move as well would leave the sentence
		// naming the destination for the wrong reason, which is how this assertion
		// first passed while checking nothing.
		p := &fakePointer{refuse: map[int]bool{dragSteps + 3: true}}
		err := dragGesture(context.Background(), p, "left", 0, 0, 240, 0, dragHost{})
		if !errors.Is(err, errButtonNotReleased) {
			t.Fatalf("the machine refused the release and the gesture reported %v; "+
				"a button left down is the worst outcome this gesture has and "+
				"the one it most has to say out loud", err)
		}
		if !strings.Contains(err.Error(), "240,0") {
			t.Errorf("the sentence for a stuck button does not say where it was "+
				"last seen: %v", err)
		}
	})
}

// TestAStoppedDragSaysBothWhatStoppedAndWhatIsStuck keeps the chord's rule for the drag: a
// cancellation whose release ALSO failed has to read as both, or one of the two answers is lost —
// the run would read as failed rather than stopped, or a button held down on somebody's desktop
// would go unsaid.
func TestAStoppedDragSaysBothWhatStoppedAndWhatIsStuck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &fakePointer{stopAfter: 5, cancel: cancel, refuse: map[int]bool{6: true}}
	err := dragGesture(ctx, p, "left", 0, 0, 240, 0, dragHost{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("a stopped drag whose release was refused does not read as stopped: %v",
			err)
	}
	if !errors.Is(err, errButtonNotReleased) {
		t.Errorf("a stopped drag whose release was refused does not say the button is "+
			"stuck: %v", err)
	}
}

// TestTheGestureSendsTheWholeTravel is the ordinary case, and it is here so the three tests above
// are read as being about exits rather than about a gesture that no longer happens.
func TestTheGestureSendsTheWholeTravel(t *testing.T) {
	p := &fakePointer{}
	if err := dragGesture(context.Background(), p, "right", 10, 10, 250, 90, dragHost{}); err != nil {
		t.Fatalf("an ordinary drag reported %v", err)
	}
	if p.events[0] != "move 10,10" || p.events[1] != "right down 10,10" {
		t.Errorf("a drag must aim and then press at the start: %v", p.events[:2])
	}
	if p.last() != "right up 250,90" {
		t.Errorf("an uninterrupted drag releases AT THE DESTINATION, which is the "+
			"whole gesture; it released with %q", p.last())
	}
	if n := len(p.events); n != dragSteps+3 {
		t.Errorf("the gesture sent %d events, want %d: the aim, the press, %d moves "+
			"along the way, the landing move and the release", n, dragSteps+3,
			dragSteps-1)
	}
}

// ── and the same three, from where a routine stands ──

// TestAStoppedRunDragsNothing is the entrance check where it holds for every backend.
//
// In the host rather than only in the gesture, for the reason `doKeyDown` gives: a claim about
// Windows is a claim nothing can check, and "a stopped run presses nothing" has to be a claim
// about the host. This is the one a test can drive.
func TestAStoppedRunDragsNothing(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := runtime.NewSet()
	for k, v := range map[string]int{"FromX": 10, "FromY": 20, "ToX": 300, "ToY": 400} {
		s.Put(k, runtime.Number(float64(v)))
	}
	st, _, _ := h.Invoke(runtime.HostCall{Action: "Drag", Input: runtime.SetVal(s), Ctx: ctx})
	if st != "failed" {
		t.Errorf("a drag dispatched after the run stopped reported %q", st)
	}
	if len(rec.drags) != 0 {
		t.Errorf("a drag dispatched after the run stopped reached the machine: %v",
			rec.drags)
	}
}

// TestADragTheMachineRefusedIsInTheAccount is the third defect where it is read from.
//
// `doDrag` reported through `ok`, which turns an error into a failed step and records it nowhere —
// so a drag the machine refused was absent from [Host.Unperformed], and a command that reads that
// account to decide what to say about a run read an empty one and said the run was fine. It is
// counted as ASKED FOR either way, which is the honest pair: the run asked, and this says it did
// not happen.
func TestADragTheMachineRefusedIsInTheAccount(t *testing.T) {
	rec := &recBackend{refuseFrom: 1}
	h := &Host{b: rec}
	s := runtime.NewSet()
	for k, v := range map[string]int{"FromX": 10, "FromY": 20, "ToX": 300, "ToY": 400} {
		s.Put(k, runtime.Number(float64(v)))
	}
	if st, _, _ := call(h, "Drag", runtime.SetVal(s)); st != "failed" {
		t.Fatalf("a refused drag reported %q", st)
	}
	undone := h.Unperformed()
	if len(undone) != 1 || !strings.Contains(undone[0], "dragging from 10,20 to 300,400") {
		t.Errorf("a drag the machine refused is not in the account of what did not "+
			"happen: %v", undone)
	}
	if got := h.InputActions(); got != 1 {
		t.Errorf("the run asked for %d input actions, want 1 — a refused drag is "+
			"still a drag somebody asked for", got)
	}
}

// TestADragStoppedWhileItAimsDoesNotPress is the gap between the entrance check and the press.
//
// One check at the top of the gesture is not enough on its own, and this is why: the aim is a
// send, a send takes time, and a stop arriving while the machine accepts the aim found nothing
// between it and the button going down. The press is the event that cannot be taken back, so the
// check has to sit against the press rather than against the function. The machine here cancels
// the run as it takes the aim, which is the narrowest arrangement of that instant.
func TestADragStoppedWhileItAimsDoesNotPress(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &fakePointer{stopAfter: 1, cancel: cancel}
	if err := dragGesture(ctx, p, "left", 10, 10, 90, 90, dragHost{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("a drag stopped during its aim reported %v, want a cancellation", err)
	}
	if len(p.events) != 1 || p.events[0] != "move 10,10" {
		t.Errorf("a drag stopped while it aimed sent %v; the aim had been taken, so "+
			"the aim is all there should be", p.events)
	}
	for _, e := range p.events {
		if strings.Contains(e, "down") || strings.Contains(e, "up") {
			t.Errorf("a stop arrived while the aim was being taken and the gesture "+
				"pressed anyway: %v", p.events)
		}
	}
}

// ── and the account a cancellation cannot discard ──

// aBackendWithARealDrag is the recording backend with the PRODUCTION gesture underneath its
// drags.
//
// `recBackend.drag` records four numbers and returns, which is right for every test about what
// was asked for. This one runs `dragGesture` instead, against a machine the case supplies — so
// the aim, the press, the travel and the release all happen in order, with a machine that can
// refuse a release and a stop that can arrive in the middle.
type aBackendWithARealDrag struct {
	*recBackend
	pointer *fakePointer
}

func (b aBackendWithARealDrag) drag(ctx context.Context, button string, x1, y1, x2, y2 int,
	host dragHost) error {

	b.recBackend.sawDrag(button, x1, y1, x2, y2)
	return dragGesture(ctx, b.pointer, button, x1, y1, x2, y2, host)
}

// aDrag is the Drag set a routine's `do OS's Drag with …` arrives as.
func aDrag(fx, fy, tx, ty int) runtime.Value {
	s := runtime.NewSet()
	for k, v := range map[string]int{"FromX": fx, "FromY": fy, "ToX": tx, "ToY": ty} {
		s.Put(k, runtime.Number(float64(v)))
	}
	return runtime.SetVal(s)
}

// draggedWithARealGesture performs one drag through the PRODUCTION host and the production
// gesture, with the machine arranged by the caller, and hands back the host to ask.
func draggedWithARealGesture(t *testing.T, p *fakePointer) (*Host, string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	p.cancel = cancel
	h := &Host{b: aBackendWithARealDrag{recBackend: &recBackend{}, pointer: p}}
	st, _, _ := h.Invoke(runtime.HostCall{Action: "Drag", Input: aDrag(10, 20, 250, 20),
		Ctx: ctx})
	return h, st
}

// TestARunStoppedMidDragKeepsTheStuckButtonWhereACancellationCannotDiscardIt is the warning
// channel, extended to the device that needs it most.
//
// `Host.NotReleased` is the account a stopped run cannot throw away: `performOnePlay` discards a
// cancelled run's error on purpose, because a stop is not a failure, and the half of that error
// which is STILL HAPPENING has to outlive the discarding. It recognised stuck keys only — so a
// drag stopped mid-travel whose release the machine refused reported `cancelled` with nothing
// beside it, while the button stayed down dragging whatever the pointer passed over. Through the
// production `Invoke` and the production gesture, with only the machine substituted.
func TestARunStoppedMidDragKeepsTheStuckButtonWhereACancellationCannotDiscardIt(t *testing.T) {
	// Stopped as the fourth move is taken, and the release that follows refused.
	h, st := draggedWithARealGesture(t, &fakePointer{stopAfter: 6,
		refuse: map[int]bool{7: true}})

	warn := h.NotReleased()
	switch {
	case warn == "":
		t.Fatalf("a run stopped mid-drag whose button the machine would not release "+
			"says nothing in the one account a cancellation cannot discard; the "+
			"step reported %q and that is thrown away", st)
	case !strings.Contains(warn, "left mouse button"):
		t.Errorf("the account says %q and never names what is held down", warn)
	case !strings.Contains(warn, "may still be held down"):
		t.Errorf("the account says %q and does not say the computer is still holding "+
			"it, which is the part somebody has to act on", warn)
	case !strings.Contains(warn, "dragging from 10,20 to 250,20"):
		t.Errorf("the account says %q and does not say what was being done when it "+
			"happened", warn)
	}
}

// AND A STOP THAT LET GO OF THE BUTTON SAYS NOTHING.
//
// The control, and the one that makes the case above mean something: a host that complained after
// every stopped drag would satisfy it and would also tell somebody their mouse was stuck every
// time they pressed Escape.
func TestAStoppedDragThatReleasedTheButtonLeavesNothingInTheAccount(t *testing.T) {
	h, _ := draggedWithARealGesture(t, &fakePointer{stopAfter: 6})

	if warn := h.NotReleased(); warn != "" {
		t.Errorf("a drag that was stopped and DID let go of its button left %q in the "+
			"account of what is still held down", warn)
	}
}

// TestADragRefusedAtTheLastInstantPressesNothing is the window's half of the gap the aim opens.
//
// The aim is an injected pointer move, and a pointer move changes applications. So the gesture
// asks once more, after aiming and immediately before the press — and a refusal there has to
// stop the press, which is the event that cannot be taken back. Asserted on what the machine was
// asked for: the aim, and nothing else.
func TestADragRefusedAtTheLastInstantPressesNothing(t *testing.T) {
	p := &fakePointer{}
	refused := errors.New("the window moved between this drag being aimed and the " +
		"button going down")
	err := dragGesture(context.Background(), p, "left", 10, 10, 90, 90,
		dragHost{confirm: func() error { return refused }})
	if !errors.Is(err, refused) {
		t.Fatalf("the gesture reported %v, want the refusal it was given", err)
	}
	if len(p.events) != 1 || p.events[0] != "move 10,10" {
		t.Errorf("the gesture sent %v; the aim had happened when the window was "+
			"re-read, so the aim is all there should be", p.events)
	}
	for _, e := range p.events {
		if strings.Contains(e, "down") || strings.Contains(e, "up") {
			t.Errorf("the window was refused immediately before the press and "+
				"the gesture pressed anyway: %v", p.events)
		}
	}
}

// AND NOTHING IS SENT BETWEEN THAT ANSWER AND THE PRESS.
//
// The property the whole check rests on, and it is about an ORDER rather than a value: a check
// followed by an injected event is a check made before that event. The machine records when it
// was asked, so the claim is that the aim came first and the press came next, with the question
// in between and nothing else.
func TestNothingIsSentBetweenTheWindowCheckAndThePress(t *testing.T) {
	p := &fakePointer{}
	var atAsk int
	err := dragGesture(context.Background(), p, "left", 10, 10, 90, 90,
		dragHost{confirm: func() error { atAsk = len(p.events); return nil }})
	if err != nil {
		t.Fatalf("an ordinary guarded drag reported %v", err)
	}
	if atAsk != 1 {
		t.Errorf("the window was re-read after %d event(s), want 1 — it has to "+
			"come AFTER the aim, which is the event that can change the "+
			"application", atAsk)
	}
	if len(p.events) <= atAsk || !strings.Contains(p.events[atAsk], "down") {
		t.Errorf("the event after the window was re-read is %v, want the press — "+
			"anything in between is input sent on an answer that predates it",
			p.events[atAsk:])
	}
}

// AND A STOP THAT ARRIVES WHILE THE WINDOW IS BEING READ STILL PRESSES NOTHING.
//
// The third instant in this gap, and the one the first cut got wrong. The window check is a READ
// and sends nothing, so it was allowed to sit between the stop check and the press — but it reads
// a rectangle, a scale and a process name off a live desktop, and a stop arriving while it does
// that found nothing between its own answer and the button going down. The window was still
// there, the callback said so, and the press went out on a run that had already been stopped.
//
// The machine here cancels the run from inside the confirmation and then AGREES, which is the
// narrowest arrangement of it: the only thing that can refuse has said yes, so nothing but a
// final read of the stop stands between the answer and the press.
func TestADragStoppedWhileTheWindowIsCheckedDoesNotPress(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &fakePointer{}
	err := dragGesture(ctx, p, "left", 10, 10, 90, 90, dragHost{
		confirm: func() error { cancel(); return nil },
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a drag stopped while its window was being read reported %v, want "+
			"a cancellation", err)
	}
	if len(p.events) != 1 || p.events[0] != "move 10,10" {
		t.Errorf("the gesture sent %v; the aim had been taken when the stop arrived, "+
			"so the aim is all there should be", p.events)
	}
	for _, e := range p.events {
		if strings.Contains(e, "down") || strings.Contains(e, "up") {
			t.Errorf("a stop arrived while the window was being confirmed and the "+
				"gesture pressed anyway: %v", p.events)
		}
	}
}

// AND A RUN THAT HAS ALREADY STOPPED ASKS THE DESKTOP NOTHING.
//
// The other side of the same gap, and the reason the stop is read BEFORE the window and not only
// after it. Somebody who presses Escape mid-route is owed "it was stopped" and not "that window
// changed size": a gesture that confirmed first would answer with whichever of the two it
// happened to find, and a desktop read is the slower of the two questions to ask on a run that
// has nothing left to do.
func TestADragStoppedBeforeItAimedAsksTheDesktopNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &fakePointer{stopAfter: 1, cancel: cancel} // stopped as the aim is taken
	var asked int
	err := dragGesture(ctx, p, "left", 10, 10, 90, 90, dragHost{
		confirm: func() error { asked++; return nil },
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a drag stopped during its aim reported %v, want a cancellation", err)
	}
	if asked != 0 {
		t.Errorf("the gesture read the window %d time(s) on a run that had already "+
			"been stopped — a window complaint is the wrong answer to give "+
			"somebody who pressed Escape", asked)
	}
}

// ── and the exits that are not returns ──

// aPointerThatPanics is a machine that stops working the way a real failure does: not by
// returning false, which the gesture already handles, but by unwinding the stack out from under
// it. `at` is the event number it happens on.
type aPointerThatPanics struct {
	*fakePointer
	at int
}

func (p *aPointerThatPanics) check() {
	if len(p.events) >= p.at {
		panic("the machine came apart")
	}
}

func (p *aPointerThatPanics) moveTo(x, y int) bool {
	p.check()
	return p.fakePointer.moveTo(x, y)
}

func (p *aPointerThatPanics) press(b string, x, y int) bool {
	p.check()
	return p.fakePointer.press(b, x, y)
}

// A PANIC BEFORE THE BUTTON WENT DOWN REPORTS NOTHING AND RELEASES NOTHING.
//
// The case that makes the state machine four states instead of two. A slot armed on the way INTO
// the gesture — which is what "write the sentence first, clear it on a clean release" would be —
// tells somebody a mouse button may be held down on their computer after a panic during the AIM,
// when nothing was ever pressed. A warning that fires when nothing happened is worse than none:
// it is the thing that teaches people to ignore the warning.
func TestAPanicBeforeTheButtonWentDownSaysNothing(t *testing.T) {
	p := &aPointerThatPanics{fakePointer: &fakePointer{}, at: 1} // the aim itself
	var said []error
	func() {
		defer func() { _ = recover() }()
		dragGesture(context.Background(), p, "left", 10, 10, 90, 90,
			dragHost{stuck: func(err error) { said = append(said, err) }})
	}()

	if len(said) != 0 {
		t.Errorf("a panic before anything was pressed reported %v — nothing was "+
			"down, so there is nothing to say", said)
	}
	for _, e := range p.events {
		if strings.Contains(e, "up") {
			t.Errorf("a release was sent for a button that never went down: %v",
				p.events)
		}
	}
}

// AND A PANIC DURING THE TRAVEL LETS GO, WHERE THE POINTER GOT TO.
//
// The cleanup-success case. A panic unwinding past the gesture runs its deferred code, so the
// button comes up on the way out — no ledger, no split backend, nothing outside this function
// involved, because the function is still on the stack while the unwinding happens. And it
// releases at the last point the machine accepted, for the same reason a stop does: a release at
// the destination is a drop.
func TestAPanicDuringTheTravelStillLetsGo(t *testing.T) {
	// The aim, the press, two moves, then the machine comes apart.
	p := &aPointerThatPanics{fakePointer: &fakePointer{}, at: 5}
	var said []error
	func() {
		defer func() { _ = recover() }()
		dragGesture(context.Background(), p, "left", 0, 0, 240, 0,
			dragHost{stuck: func(err error) { said = append(said, err) }})
	}()

	// Asserted against the last move the machine ACCEPTED rather than a hardcoded point,
	// for the reason the stopped-drag case gives: the claim is about where the pointer
	// got to, and a number copied from a passing run is a claim about the interpolation.
	where := p.events[len(p.events)-2]
	if want := strings.Replace(where, "move", "left up", 1); p.last() != want {
		t.Errorf("the gesture came apart after %q and released with %q, want %q — "+
			"the release goes WHERE THE POINTER IS, because a release at the "+
			"destination is a drop.\nevents: %v", where, p.last(), want,
			p.events)
	}
	// AND NOTHING IS REPORTED, because the release worked. The account exists for a
	// computer left in a changed state, and this one was not.
	if len(said) != 0 {
		t.Errorf("the button came up and the run was told it may be stuck: %v", said)
	}
}

// AND A RELEASE THE MACHINE REFUSES DURING THAT UNWINDING IS SAID OUT LOUD.
//
// The cleanup-failure case, and the one the callback exists for. A panic skips every return, so
// the gesture's own error reaches nobody — `doDrag` never runs again and neither does `delivered`.
// The sentence has to be written from inside the gesture, by code that is still on the stack.
func TestAPanicWhoseReleaseIsRefusedSaysSo(t *testing.T) {
	// The aim, the press and three moves, then the machine comes apart on the fourth —
	// so the deferred release is the sixth event, and that is the one refused.
	p := &aPointerThatPanics{
		fakePointer: &fakePointer{refuse: map[int]bool{6: true}},
		at:          5,
	}
	var said []error
	func() {
		defer func() { _ = recover() }()
		dragGesture(context.Background(), p, "left", 0, 0, 240, 0,
			dragHost{stuck: func(err error) { said = append(said, err) }})
	}()

	if len(said) != 1 {
		t.Fatalf("a panic that left the button down reported %d sentence(s), want "+
			"1 — the gesture's return value reaches nobody on this path", said)
	}
	if !errors.Is(said[0], errButtonNotReleased) {
		t.Errorf("what was reported is not a stuck button: %v", said[0])
	}
	if !strings.Contains(said[0].Error(), "30,0") {
		t.Errorf("the sentence does not say where the button was last seen (%v), "+
			"which is the last point the machine accepted: %v", said[0],
			p.events)
	}
}

// AND AN ORDINARY GESTURE RELEASES ONCE.
//
// The control for all three above, and for the deferred release in particular: a defer that ran
// unconditionally would send a second release on every successful drag — a button that is not
// down, a second refusal, and a sentence naming two failures for one.
func TestAnOrdinaryDragReleasesOnce(t *testing.T) {
	p := &fakePointer{}
	var said []error
	if err := dragGesture(context.Background(), p, "left", 0, 0, 240, 0,
		dragHost{stuck: func(err error) { said = append(said, err) }}); err != nil {
		t.Fatalf("an ordinary drag reported %v", err)
	}
	ups := 0
	for _, e := range p.events {
		if strings.Contains(e, "up") {
			ups++
		}
	}
	if ups != 1 {
		t.Errorf("the gesture sent %d release(s), want 1: %v", ups, p.events)
	}
	if len(said) != 0 {
		t.Errorf("an ordinary drag reported %v", said)
	}
}
