package oshost

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// aScrollRecorded is a recording of three notches down in the game's window, at the offset the
// first press in pressesWithButtons uses, followed by the trip to Marco's window to press Stop.
func aScrollRecorded() []recorder.RecordedEvent {
	out := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: when(0)},
	}
	for i := range 3 {
		out = append(out, recorder.RecordedEvent{Kind: recorder.EvScroll, Delta: -120,
			X: 842, Y: 274, App: "arkascended", PID: 4812, T: when(100 + i*40),
			WinRel: true, RelX: 722, RelY: 210, RelTo: "window", WinW: 1920, WinH: 1080,
			Dpi: 144, Process: "arkascended.exe"})
	}
	return append(out,
		recorder.RecordedEvent{Kind: recorder.EvAppSwitch, KeyName: "studio", T: when(3000)},
		recorder.RecordedEvent{Kind: recorder.EvMove, X: 1500, Y: 900, T: when(3100)},
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: true,
			X: 1511, Y: 913, App: "studio", PID: 7001, T: when(3200)},
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: false,
			X: 1511, Y: 913, App: "studio", PID: 7001, T: when(3205)})
}

// A DEMONSTRATED SCROLL TURNS THE WHEEL THE WAY IT WAS TURNED, AS MANY NOTCHES, WHERE THE WINDOW
// IS NOW. Everything from the recording to the turn is production: the simplifier, the generator,
// the registry, the compiler and the host. The window has moved since, so a turn at the recorded
// desktop coordinate would be a turn over something else.
func TestADemonstratedScrollTurnsWhereTheWindowIsNow(t *testing.T) {
	src, at := savedFrom(t, orchestrator.ScopeAppAnywhere, aScrollRecorded())
	if !strings.Contains(src, `Scroll "down", Notches 3`) {
		t.Fatalf("the saved routine does not say the turn:\n%s", src)
	}
	rec, h, out := played(t, at)
	want := fmt.Sprintf("down 3 at %d,%d", firstPress[0], firstPress[1])
	if len(rec.scrolls) != 1 || rec.scrolls[0] != want {
		t.Fatalf("the machine was asked for %v, want [%s]\n%s", rec.scrolls, want, out)
	}
	if undone := h.Unperformed(); len(undone) != 0 {
		t.Fatalf("the run reports %v", undone)
	}
}

// AND INTO A WINDOW OF ANOTHER SIZE IT TURNS NOTHING, as a press does not press.
func TestAScrollIntoAWindowOfAnotherSizeTurnsNothing(t *testing.T) {
	_, at := savedFrom(t, orchestrator.ScopeAppAnywhere, aScrollRecorded())
	was := playedOnTheDesktop
	t.Cleanup(func() { playedOnTheDesktop = was })
	playedOnTheDesktop = with(sameWindowMoved, func(f *winctx.WindowFacts) { f.W = 1600 })
	rec, h, out := played(t, at)
	if len(rec.scrolls) != 0 {
		t.Errorf("the wheel turned in a window of another size: %v\n%s", rec.scrolls, out)
	}
	if undone := h.Unperformed(); len(undone) != 1 || !strings.Contains(undone[0], "1600") {
		t.Errorf("the account of what did not happen is %v", undone)
	}
}

// A SCROLL THAT DOES NOT SAY WHICH WAY IS REFUSED, AND TURNS NOTHING.
func TestAScrollThatDoesNotSayWhichWayTurnsNothing(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	s := runtime.NewSet()
	s.Put("X", runtime.Number(10))
	s.Put("Y", runtime.Number(20))
	s.Put("Scroll", runtime.Text("sideways"))
	if st, _, _ := call(h, "Scroll", runtime.SetVal(s)); st != "failed" || len(rec.scrolls) != 0 {
		t.Errorf("a scroll sideways answered %q and turned %v", st, rec.scrolls)
	}
}

// AND A SCROLL THE MACHINE WOULD NOT TAKE STOPS THE ROUTINE: it is a link in the chain like a
// press, so the press after it is never asked for. Whatever the wheel was meant to bring into
// view is not there, and the press after it would land on whatever is.
//
// A scroll that is not a link (an ordinary step the routine carries on past) must fail this.
func TestAScrollTheMachineWouldNotTakeStopsTheRoutine(t *testing.T) {
	ev := pressesWithButtons("right")
	scroll := aScrollRecorded()[1:4] // its three notches, before the press
	ev = append(ev[:1], append(scroll, ev[1:]...)...)
	_, at := savedFrom(t, orchestrator.ScopeAppAnywhere, ev)

	rec, h, out := playedRefusingFrom(t, at, 1)
	if rec.sends != 1 || len(rec.clicks) != 0 {
		t.Fatalf("the machine refused the scroll and %d send(s) were attempted, pressing "+
			"%v, so the routine went on after a scroll that did not happen\n%s",
			rec.sends, rec.clicks, out)
	}
	if len(h.Unperformed()) != 1 {
		t.Fatalf("the run reports %v", h.Unperformed())
	}
	saysItStopped(t, out)
}
