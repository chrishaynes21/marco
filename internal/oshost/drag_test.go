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
	err := dragGesture(ctx, p, "left", 0, 0, 240, 0)
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
	if err := dragGesture(ctx, p, "left", 10, 10, 90, 90); !errors.Is(err, context.Canceled) {
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
		err := dragGesture(context.Background(), p, "left", 5, 5, 50, 50)
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
		err := dragGesture(context.Background(), p, "left", 0, 0, 240, 0)
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
		err := dragGesture(context.Background(), p, "left", 0, 0, 240, 0)
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
	err := dragGesture(ctx, p, "left", 0, 0, 240, 0)
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
	if err := dragGesture(context.Background(), p, "right", 10, 10, 250, 90); err != nil {
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
