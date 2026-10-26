package oshost

// A CHORD STOPPED BETWEEN ITS MODIFIERS AND ITS KEY, THROUGH THE PRODUCTION HOST.
//
// `chord_test.go` holds the sequence against a machine it supplies. This holds the same thing one
// layer up, where the pieces are the real ones: a demonstration taken through the production
// recorder, simplifier, generator, registry and compiler, performed by `oshost.Host` with the
// context cancelled the way a stop cancels it — and the only stand-in is the machine itself,
// because the real one sends real input.
//
// What that adds, and what `sendChord`'s own tests cannot say:
//
//   - the chord reaches the host as ONE step. `simplify` folds held modifiers into
//     `ctrl+shift+a`, and a demonstration that came out as four keystrokes would make every
//     assertion below true of something that is not a chord.
//   - the step AFTER it does not run. A routine does not go on pressing after something it was
//     shown did not happen, and a stop is the clearest case of that.
//   - the host's own ACCOUNT keeps the sentence. `Unperformed` is what a command reads and what a
//     front end is shown; a warning that reached neither would be a warning nobody gets.
//   - and the error that comes out is recognisable as BOTH a stop and a key nobody released,
//     which is what lets the command keep the second while discarding the first.
//
// See [[ADR-258-a-chord-is-a-promise-to-let-go]].

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// aChordBetweenTwoPressesInTheGame is a demonstration with a shortcut in the middle of it.
//
// The shape matters: a press, the chord, and then ANOTHER press, so the question "did anything
// after the stop happen" has something to be about. Modifiers down, the key inside them, then the
// modifiers up in reverse — exactly as the hooks would have left it, with the key released well
// inside `simplify`'s hold threshold so it stays a chord rather than becoming a hold.
func aChordBetweenTwoPressesInTheGame() []recorder.RecordedEvent {
	press := func(x, y, rx, ry, at int) []recorder.RecordedEvent {
		return []recorder.RecordedEvent{
			{Kind: recorder.EvClick, Button: "left", Down: true, X: x, Y: y,
				App: "arkascended", PID: 4812, T: when(at), WinRel: true,
				RelX: rx, RelY: ry, RelTo: "window", WinW: 1920, WinH: 1080,
				Dpi: 144, Process: "arkascended.exe"},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: x, Y: y,
				App: "arkascended", PID: 4812, T: when(at + 40)},
		}
	}
	key := func(name string, down bool, at int) recorder.RecordedEvent {
		return recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: name, Down: down,
			App: "arkascended", PID: 4812, T: when(at)}
	}
	s := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: when(0)},
	}
	s = append(s, press(900, 300, 100, 200, 100)...)
	s = append(s,
		key("ctrl", true, 1000), key("shift", true, 1020),
		key("a", true, 1060), key("a", false, 1100),
		key("shift", false, 1140), key("ctrl", false, 1160))
	s = append(s, press(950, 350, 150, 250, 1600)...)
	// AND THE TRIP TO MARCO'S WINDOW TO PRESS STOP, which the strip at the end of a
	// recording walks back through. Without it this is a recording of somebody who never
	// finished.
	s = append(s,
		recorder.RecordedEvent{Kind: recorder.EvMove, X: 1500, Y: 900, T: when(2100)},
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: true,
			X: 1511, Y: 913, App: "studio", PID: 7001, T: when(2200)},
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: false,
			X: 1511, Y: 913, App: "studio", PID: 7001, T: when(2205)})
	return s
}

// aBackendWithARealChord is the recording backend with the PRODUCTION sequence underneath its
// keystrokes.
//
// `recBackend.key` records a name and returns, which is right for every test about what was
// asked for. This one runs `sendChord` instead, against a machine the case supplies — so the
// modifier presses, the settle, the tap and the releases all happen for real, in order, with a
// desktop that can refuse a release and a stop that can arrive in the middle.
type aBackendWithARealChord struct {
	*recBackend
	keyboard *aKeyboard
}

func (b aBackendWithARealChord) key(ctx context.Context, name string, hold time.Duration) error {
	b.recBackend.sawKey(name, hold)
	err := sendChord(ctx, b.keyboard, name, spec(strings.Split(name, "+")...),
		hold, aSettle, aTap)
	b.keyboard.chordErr = err
	return err
}

// playedWithARealChord performs a saved routine against the production host, with the chord
// sequence real and the machine arranged by the caller.
func playedWithARealChord(t *testing.T, at string, k *aKeyboard) (*recBackend, *Host, string,
	error) {

	t.Helper()
	onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
	wasBind, wasLog := bindWindow, mlog.Default
	t.Cleanup(func() { bindWindow, mlog.Default = wasBind, wasLog })
	bindWindow = func(string) (uintptr, error) { return theGameWindow, nil }
	var said bytes.Buffer
	mlog.Default = mlog.New(&said, mlog.LevelDebug)

	rec := &recBackend{}
	h := &Host{b: aBackendWithARealChord{recBackend: rec, keyboard: k}}
	// THE CONTEXT THE STOP ARRIVES ON. `k.stop` is handed the cancel so the machine can
	// stop the run from inside its own settle, which is where a stop lands when a chord is
	// in flight: the modifiers are down and the key has not been pressed.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	k.stop = cancel

	src, err := os.ReadFile(at)
	if err != nil {
		t.Fatalf("reading the saved routine: %v", err)
	}
	var out bytes.Buffer
	runErr := driver.RunSourceWithHostsCtx(ctx, string(src), ".", at, &out,
		map[string]runtime.Host{"*": h})
	return rec, h, out.String() + "\n" + said.String(), runErr
}

// A RUN STOPPED MID-CHORD DOES NOT TAP, DOES NOT GO ON, AND SAYS WHAT IT LEFT HELD.
//
// All three in one case on purpose: they are one event from where a person is standing. They
// pressed Escape while a routine was holding Ctrl and Shift, and what they need to know is that
// the shortcut did not fire, the routine stopped, and Ctrl is still down.
func TestARunStoppedMidChordLeavesNothingPressedAndSaysWhatIsStuck(t *testing.T) {
	_, at := savedFrom(t, orchestrator.ScopeAppAnywhere, aChordBetweenTwoPressesInTheGame())
	src, err := os.ReadFile(at)
	if err != nil {
		t.Fatal(err)
	}
	// THE CHORD IS ONE STEP IN THE SAVED ROUTE. Without this the rest of the case is
	// about four keystrokes that happen to be next to each other.
	if !strings.Contains(string(src), `do OS's Key with "ctrl+shift+a"`) {
		t.Fatalf("the demonstration did not come out as a chord:\n%s", src)
	}

	k := &aKeyboard{
		// THE STOP ARRIVES DURING THE SETTLE — modifiers down, key not yet pressed.
		stopOnSettle: 1,
		// AND THE MACHINE WILL NOT LET GO OF CTRL, which is the state that outlives
		// the run and the whole reason a cancelled run may not discard its error.
		refuseUp: map[string]bool{"ctrl": true},
	}
	rec, h, out, runErr := playedWithARealChord(t, at, k)

	// NOTHING WAS TAPPED. A chord that was stopped did not happen, and a bare `a` would
	// be a different keystroke delivered to a window nobody asked.
	for _, e := range k.events {
		if e == "down:a" {
			t.Errorf("the key was tapped after the run was stopped: %v", k.events)
		}
	}
	// AND NOTHING AFTER IT. The second press was demonstrated against a screen the
	// shortcut was supposed to change.
	if len(rec.clicks) != 1 {
		t.Errorf("%d press(es) were sent; the run was stopped at the chord, so there "+
			"should have been the one before it\n%s", len(rec.clicks), out)
	}
	// AND EVERY KEY THAT WENT DOWN WAS AT LEAST ASKED TO COME UP, including the one the
	// machine refused: an implementation that stopped at the first failed release would
	// leave Shift down as well and report only Ctrl.
	for _, want := range []string{"up:shift", "refused-up:ctrl"} {
		if !strings.Contains(strings.Join(k.events, " "), want) {
			t.Errorf("the machine was never asked for %q: %v", want, k.events)
		}
	}
	// AND THE HOST'S OWN ACCOUNT KEEPS IT, which is what a command reads.
	undone := h.Unperformed()
	if len(undone) != 1 {
		t.Fatalf("the run reports %d thing(s) that did not happen: %v\n%s", len(undone),
			undone, out)
	}
	if !strings.Contains(undone[0], "ctrl+shift+a") ||
		!strings.Contains(undone[0], "would not release ctrl") {
		t.Errorf("the account says %q; it should name the chord and the key that may "+
			"still be held down", undone[0])
	}
	// AND THE ERROR IS BOTH THINGS AT ONCE. The command keeps the second while
	// discarding the first, and it can only do that if both are recognisable.
	if runErr != nil {
		t.Logf("the run also returned: %v", runErr)
	}
	stopped := k.chordErr
	switch {
	case stopped == nil:
		t.Fatal("the chord reported success after being stopped with a key held down")
	case !errors.Is(stopped, context.Canceled):
		t.Errorf("the chord answered %v, which no longer reads as a stop — a person "+
			"pressing Escape would become a routine that failed", stopped)
	case !errors.Is(stopped, errKeyNotReleased):
		t.Errorf("the chord answered %v, which is not recognisable as a key nobody "+
			"released — so a cancelled run has no way to keep that half of it",
			stopped)
	}
}

// AND A STOP THAT LET GO OF EVERYTHING SAYS NOTHING EXTRA.
//
// The control, and it is the one that keeps the case above honest: an implementation that
// complained about a stuck key after every stop would satisfy every assertion up there while
// telling people their keyboard is broken whenever they press Escape.
func TestARunStoppedMidChordThatReleasedEverythingReportsOnlyTheStop(t *testing.T) {
	_, at := savedFrom(t, orchestrator.ScopeAppAnywhere, aChordBetweenTwoPressesInTheGame())

	k := &aKeyboard{stopOnSettle: 1}
	rec, h, out, _ := playedWithARealChord(t, at, k)

	if len(rec.clicks) != 1 {
		t.Errorf("%d press(es) were sent; the run was stopped at the chord\n%s",
			len(rec.clicks), out)
	}
	if held := k.stillDown(); len(held) != 0 {
		t.Errorf("%v is still held down after a stop that should have released "+
			"everything", held)
	}
	if stopped := k.chordErr; !errors.Is(stopped, context.Canceled) {
		t.Errorf("the chord answered %v, want a stop", stopped)
	} else if errors.Is(stopped, errKeyNotReleased) {
		t.Errorf("the chord reported a key nobody released (%v) after letting go of "+
			"all of them", stopped)
	}
	// THE ACCOUNT STILL NAMES THE KEYSTROKE AS ONE THAT DID NOT HAPPEN, because it did
	// not — the routine was stopped before the tap. What it must not carry is a sentence
	// about a key that is down.
	for _, line := range h.Unperformed() {
		if strings.Contains(line, "would not release") {
			t.Errorf("the account mentions a stuck key after a clean stop: %q", line)
		}
	}
}
