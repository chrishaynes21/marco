package oshost

// A KEYSTROKE THE MACHINE WOULD NOT TAKE, THROUGH THE WHOLE CHAIN.
//
// `internal/codegen` holds what the generator writes and what the runtime does with it, against a
// host that answers however a case needs. This holds the other half, which that cannot: a real
// recording, taken through the production recorder, simplifier, generator, registry, compiler and
// THIS host, with the machine rejecting the keystroke the way Windows really does — an elevated
// window owning the foreground, UIPI, a game with raw input and a lower-integrity sender.
//
// Two claims, and the second is the one a status cannot make. The routine stops, and the run's own
// ACCOUNT keeps what the machine said, because that account is what `marco do` turns into a failed
// command and what Studio reads back as the reason. A key failure used to stop nothing and be
// recorded nowhere: the status said failed, the chain had no reason to look at it, and the
// account a command reads was empty — so the command reported success about a routine that had
// not run.
//
// See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// aKeyBetweenTwoPressesInTheGame is the owner's own routine as the hooks take it: the application
// comes forward, a press in it, Escape, another press in it, and then the trip to Marco's window
// to press Stop.
//
// Escape is released 50ms later and well before the press that follows, because a key still held
// when something else happens is a HOLD and becomes a different pair of steps — `simplify`'s
// threshold is 500ms and its other rule is "released before the next action".
func aKeyBetweenTwoPressesInTheGame() []recorder.RecordedEvent {
	press := func(x, y, rx, ry, at int) []recorder.RecordedEvent {
		return []recorder.RecordedEvent{
			{Kind: recorder.EvClick, Button: "left", Down: true, X: x, Y: y,
				App: "arkascended", PID: 4812, T: when(at), WinRel: true,
				RelX: rx, RelY: ry, RelTo: "window", WinW: 1920, WinH: 1080,
				Dpi: 144, Process: "arkascended.exe"},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: x, Y: y,
				App: "arkascended", PID: 4812, T: when(at + 5)},
		}
	}
	s := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: when(0)},
	}
	s = append(s, press(842, 274, 722, 210, 100)...)
	s = append(s,
		recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "escape", Down: true,
			T: when(400)},
		recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "escape", Down: false,
			T: when(450)})
	s = append(s, press(496, 283, secondRelX, secondRelY, 900)...)
	// ── AND NOW THEY GO AND PRESS STOP ──
	s = append(s,
		recorder.RecordedEvent{Kind: recorder.EvAppSwitch, KeyName: "studio",
			T: when(3000)},
		recorder.RecordedEvent{Kind: recorder.EvMove, X: 1500, Y: 900, T: when(3100)},
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: true,
			X: 1511, Y: 913, App: "studio", PID: 7001, T: when(3200)},
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: false,
			X: 1511, Y: 913, App: "studio", PID: 7001, T: when(3205)})
	return s
}

// A KEYSTROKE THE MACHINE WILL NOT DELIVER STOPS THE ROUTINE, AND THE RUN SAYS WHY.
//
// Everything about this case is right except the delivery: the application came forward, the
// first press passed every check and landed, and the keystroke is one the recording really
// contains. The machine then refuses it — and what follows it was demonstrated against a screen
// that Escape was supposed to change, so none of it may happen.
func TestAKeyTheMachineWillNotDeliverStopsTheRoutine(t *testing.T) {
	src, at := savedFrom(t, orchestrator.ScopeAppAnywhere, aKeyBetweenTwoPressesInTheGame())

	// THE KEY IS A LINK IN THE SAVED ROUTE. Without this the rest of the test passes on
	// a routine that stopped for some other reason, or never had the key at all.
	if !strings.Contains(src, `do OS's Key with "escape"...`) {
		t.Fatalf("the saved routine does not stop at its keystroke:\n%s", src)
	}

	// The sends are: the first press, then the key. Refusing from the second means the
	// press went through and the keystroke did not.
	rec, h, out := playedRefusingFrom(t, at, 2)

	if len(rec.moves) != 1 || rec.moves[0] != firstPress {
		t.Fatalf("the first press landed at %v, want just %v\n%s", rec.moves,
			firstPress, out)
	}
	if len(rec.keys) != 1 || rec.keys[0] != "escape" {
		t.Errorf("the machine was asked for %v, want one escape\n%s", rec.keys, out)
	}
	// NOTHING AFTER IT. The second press is never attempted, which is the whole claim:
	// a routine does not go on pressing after something it was shown did not happen.
	if len(rec.clicks) != 1 {
		t.Errorf("%d press(es) were sent; the keystroke before the second one did "+
			"not happen, so there should have been one\n%s", len(rec.clicks), out)
	}
	if rec.sends != 2 {
		t.Errorf("the machine was asked to send input %d time(s), want 2 — the "+
			"press, and the keystroke it refused\n%s", rec.sends, out)
	}
	// AND THE RUN IS UNSUCCESSFUL, WITH THE MACHINE'S OWN WORDS KEPT. A command reads
	// this list and nothing else, so a sentence that is only in the log is a run that
	// reports success.
	undone := h.Unperformed()
	if len(undone) != 1 {
		t.Fatalf("the run reports %d thing(s) that did not happen: %v — a command "+
			"reading this would report success\n%s", len(undone), undone, out)
	}
	if !strings.Contains(undone[0], "the escape key did not happen") ||
		!strings.Contains(undone[0], errTheMachineSaysNo.Error()) {
		t.Errorf("the run's answer is %q; it should name the keystroke and keep what "+
			"the machine said", undone[0])
	}
	saysItStopped(t, out)
}

// AND THE SAME ROUTINE DOES ALL OF IT WHEN THE MACHINE TAKES THE KEY.
//
// The control. Without it the case above passes on a routine that refuses everything, or on one
// whose keystroke never reached the host at all.
func TestAKeyTheMachineTakesLetsTheRoutineGoOn(t *testing.T) {
	_, at := savedFrom(t, orchestrator.ScopeAppAnywhere, aKeyBetweenTwoPressesInTheGame())

	rec, h, out := played(t, at)

	want := [][2]int{firstPress, secondPress}
	if len(rec.moves) != 2 || rec.moves[0] != want[0] || rec.moves[1] != want[1] {
		t.Errorf("the presses landed at %v, want %v\n%s", rec.moves, want, out)
	}
	if len(rec.keys) != 1 || rec.keys[0] != "escape" {
		t.Errorf("the machine was asked for %v, want one escape\n%s", rec.keys, out)
	}
	if undone := h.Unperformed(); len(undone) != 0 {
		t.Errorf("the run reports %v, having done everything it was shown", undone)
	}
	if !strings.Contains(out, "ark graphics: done") {
		t.Errorf("the routine does not report that it finished:\n%s", out)
	}
}
