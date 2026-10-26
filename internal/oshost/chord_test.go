package oshost

// WHAT A CHORD OWES THE COMPUTER IT WAS SENT TO.
//
// Every case here is about the same promise: a modifier this process pressed is held down on
// somebody's machine until this process lets go of it, and nothing else will. So the three
// questions are what was sent, in what order, and what was left down — and the last one is the
// one the old code could get wrong while reporting success.
//
// No desktop, no `SendInput`, no build tag: `sendChord` is written against a machine that takes
// one key event at a time, so a refused modifier, a stop mid-flight and a release the machine
// will not take are all things a test can arrange. None of them were reachable while the whole
// sequence sat inside the function that calls the OS, which is why two of the three defects were
// silent and the third had never fired.
//
// See [[ADR-258-a-chord-is-a-promise-to-let-go]].

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// aKeyboard is a machine that takes one key event at a time and remembers everything.
//
// It can refuse a named key's press or its release, and it can have the run STOPPED while it is
// settling — which is what a person pressing Escape mid-chord looks like from in here.
type aKeyboard struct {
	events     []string
	refuseDown map[string]bool
	refuseUp   map[string]bool
	// stopOnSettle cancels the run during the nth settle (1-based, 0 for never), the way a
	// real stop arrives: the wait ends early and the sequence has to notice.
	stopOnSettle int
	settles      int
	stop         context.CancelFunc
	waited       []time.Duration
	// chordErr is what the production sequence answered, for a case that drives it from
	// above and has no other way to see the error the host turned into an account.
	chordErr error
}

func (k *aKeyboard) down(s keySpec) bool {
	if k.refuseDown[s.name] {
		k.events = append(k.events, "refused-down:"+s.name)
		return false
	}
	k.events = append(k.events, "down:"+s.name)
	return true
}

func (k *aKeyboard) up(s keySpec) bool {
	if k.refuseUp[s.name] {
		k.events = append(k.events, "refused-up:"+s.name)
		return false
	}
	k.events = append(k.events, "up:"+s.name)
	return true
}

func (k *aKeyboard) settle(ctx context.Context, d time.Duration) {
	k.settles++
	k.waited = append(k.waited, d)
	k.events = append(k.events, "settle")
	if k.stopOnSettle <= 0 || k.settles != k.stopOnSettle || k.stop == nil {
		return
	}
	k.stop()
	// AND THEN IT WAITS FOR THE STOP TO ARRIVE, because from in here it is not instant.
	//
	// The real `settle` waits — that is what it is — and a stop reaches a host call by
	// travelling: the runtime watches the entry context from a goroutine of its own and
	// cancels the frame tree when it fires. A stand-in that cancelled and returned at once
	// was asking the sequence about a context that had not been cancelled yet, which looked
	// exactly like a sequence that ignores stops. It cost an afternoon's debugging of
	// working code.
	//
	// Bounded, so a test that is wrong about the arrangement fails rather than hangs. In
	// the cases that cancel the context directly this returns at once.
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
	}
}

// stillDown is what the computer is left holding: every key pressed and not released.
//
// Computed from the events rather than asked of the stand-in, because this is the question the
// production code answers wrongly when it answers it at all — a count kept by the machine would
// be a second account of the same thing, agreeing with itself.
func (k *aKeyboard) stillDown() []string {
	var held []string
	for _, e := range k.events {
		name, ok := strings.CutPrefix(e, "down:")
		if ok {
			held = append(held, name)
			continue
		}
		name, ok = strings.CutPrefix(e, "up:")
		if !ok {
			continue
		}
		for i := len(held) - 1; i >= 0; i-- {
			if held[i] == name {
				held = append(held[:i], held[i+1:]...)
				break
			}
		}
	}
	return held
}

// spec is the resolved form of a chord, named part by part. The numbers are arbitrary here and
// are only carried so a failure can be told which key it was about.
func spec(names ...string) []keySpec {
	out := make([]keySpec, len(names))
	for i, n := range names {
		out[i] = keySpec{name: n, vk: uint16(i + 1), scan: uint16(i + 1)}
	}
	return out
}

const (
	aSettle = 40 * time.Millisecond
	aTap    = 25 * time.Millisecond
)

// AN ORDINARY CHORD, WHICH IS THE CONTROL EVERY OTHER CASE IS READ AGAINST.
//
// Without it, a sequence that pressed nothing at all would satisfy every assertion below about
// what was left down.
func TestAChordHoldsItsModifiersAndLetsThemGoInReverse(t *testing.T) {
	k := &aKeyboard{}
	if err := sendChord(context.Background(), k, "ctrl+shift+esc",
		spec("ctrl", "shift", "esc"), 0, aSettle, aTap); err != nil {
		t.Fatalf("an ordinary chord: %v", err)
	}
	want := []string{"down:ctrl", "down:shift", "settle", "down:esc", "settle",
		"up:esc", "up:shift", "up:ctrl"}
	if got := strings.Join(k.events, " "); got != strings.Join(want, " ") {
		t.Errorf("the machine was given\n  %s\nand should have been given\n  %s", got,
			strings.Join(want, " "))
	}
	if held := k.stillDown(); len(held) != 0 {
		t.Errorf("%v is still held down on this computer", held)
	}
}

// AND A PLAIN KEY IS A CHORD OF ONE, with the linger a fast application needs.
//
// The default differs — a plain key gets the tap linger and a combo gets the settle — so the two
// are asserted apart. An instant down-and-up is what a game drops.
func TestAPlainKeyIsPressedWithALingerAndNothingElse(t *testing.T) {
	k := &aKeyboard{}
	if err := sendChord(context.Background(), k, "escape", spec("escape"),
		0, aSettle, aTap); err != nil {
		t.Fatalf("a plain key: %v", err)
	}
	want := "down:escape settle up:escape"
	if got := strings.Join(k.events, " "); got != want {
		t.Errorf("the machine was given %q, want %q", got, want)
	}
	if len(k.waited) != 1 || k.waited[0] != aTap {
		t.Errorf("a plain key was held for %v, want the tap linger %v", k.waited, aTap)
	}
}

// AND AN EXPLICIT HOLD WINS, which is what lets Alt+Tab commit against a fullscreen app.
func TestAnExplicitHoldIsHowLongTheComboStaysDown(t *testing.T) {
	k := &aKeyboard{}
	const asked = 300 * time.Millisecond
	if err := sendChord(context.Background(), k, "alt+tab", spec("alt", "tab"),
		asked, aSettle, aTap); err != nil {
		t.Fatalf("a held chord: %v", err)
	}
	// Two waits: the modifier settling, then the combo being held.
	if len(k.waited) != 2 || k.waited[0] != aSettle || k.waited[1] != asked {
		t.Errorf("the waits were %v, want the settle %v then the asked-for hold %v",
			k.waited, aSettle, asked)
	}
}

// A MODIFIER THE MACHINE REFUSES DOES NOT LEAVE THE ONE BEFORE IT HELD DOWN.
//
// The first defect, and the one with a consequence somebody has to clear up by hand. `ctrl+shift+esc`
// with the Shift refused used to return from the middle of the loop with Ctrl down — so the
// routine reported a keystroke that did not happen, and left the computer in a state where
// everything typed afterwards was a different keystroke. The report was the smaller half.
func TestARefusedModifierReleasesTheOnesAlreadyHeld(t *testing.T) {
	k := &aKeyboard{refuseDown: map[string]bool{"shift": true}}
	err := sendChord(context.Background(), k, "ctrl+shift+esc",
		spec("ctrl", "shift", "esc"), 0, aSettle, aTap)
	if err == nil {
		t.Fatal("a chord the machine refused reported success")
	}
	if held := k.stillDown(); len(held) != 0 {
		t.Errorf("%v is still held down after a refused chord, which is the defect: "+
			"the keystroke did not happen AND the computer was left modified", held)
	}
	// AND THE KEY IS NEVER TAPPED. A chord whose modifiers are not all down is not that
	// chord, and sending the bare key would be sending a different keystroke.
	for _, e := range k.events {
		if e == "down:esc" {
			t.Errorf("the key was tapped without its modifiers: %v", k.events)
		}
	}
	if !strings.Contains(err.Error(), `"shift"`) ||
		!strings.Contains(err.Error(), `"ctrl+shift+esc"`) {
		t.Errorf("the failure says %q; it should name the part the machine refused "+
			"and the chord it was part of", err)
	}
}

// AND A REFUSED KEY RELEASES THE MODIFIERS IT WAS ASKED TO GO WITH.
//
// This half always worked. It is here because the change that fixed the half above moved the
// release, and a release that moved too far would take this with it.
func TestARefusedKeyStillReleasesTheModifiers(t *testing.T) {
	k := &aKeyboard{refuseDown: map[string]bool{"c": true}}
	err := sendChord(context.Background(), k, "ctrl+c", spec("ctrl", "c"),
		0, aSettle, aTap)
	if err == nil {
		t.Fatal("a chord whose key the machine refused reported success")
	}
	if held := k.stillDown(); len(held) != 0 {
		t.Errorf("%v is still held down after the key was refused", held)
	}
	// AND NOTHING IS RELEASED THAT WAS NEVER DOWN. `SendInput` takes an event or it does
	// not, so a refused press left nothing to undo — and the release used to be sent
	// anyway, which produced a second refusal and a sentence naming two failures for one.
	for _, e := range k.events {
		if e == "up:c" || e == "refused-up:c" {
			t.Errorf("a key the machine would not press was released anyway: %v",
				k.events)
		}
	}
}

// A STOP WHILE THE MODIFIERS SETTLE DOES NOT SEND THE CHORD.
//
// The second defect, and the one that had never fired. `settle` returns early when the run is
// cancelled — and the next line pressed the key regardless, so Escape or the panic key, pressed
// in the one window where a chord was in flight, sent the chord. The whole purpose of a stop is
// that the thing in flight does not happen.
func TestAStopWhileTheModifiersSettleDoesNotTapTheKey(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	k := &aKeyboard{stopOnSettle: 1, stop: cancel}

	err := sendChord(ctx, k, "ctrl+c", spec("ctrl", "c"), 0, aSettle, aTap)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("a stopped chord answered %v; a stop has to stay a stop, or a person "+
			"pressing Escape becomes a routine that failed", err)
	}
	for _, e := range k.events {
		if e == "down:c" {
			t.Errorf("the key was tapped after the run was stopped: %v", k.events)
		}
	}
	// AND THE MODIFIER DOES NOT STAY DOWN BECAUSE OF THE STOP. Stopping a routine is the
	// one moment somebody is already reaching for the keyboard.
	if held := k.stillDown(); len(held) != 0 {
		t.Errorf("%v is still held down after the run was stopped", held)
	}
}

// AND A RUN ALREADY STOPPED HAS NOTHING SENT TO IT AT ALL.
//
// The modifiers used to go down first and the cancellation was noticed nowhere, so a chord
// reached after a stop pressed Ctrl, waited, and tapped.
func TestAChordSendsNothingToARunThatHasAlreadyStopped(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	k := &aKeyboard{}

	if err := sendChord(ctx, k, "ctrl+c", spec("ctrl", "c"), 0, aSettle,
		aTap); !errors.Is(err, context.Canceled) {
		t.Errorf("a chord sent to a stopped run answered %v", err)
	}
	if len(k.events) != 0 {
		t.Errorf("the machine was given %v by a run that had already stopped", k.events)
	}
}

// A RELEASE THE MACHINE WILL NOT TAKE IS REPORTED, AND SAYS WHAT IS STILL DOWN.
//
// The third defect, and the quietest: the reverse loop discarded every return value, so the one
// outcome a person most needs to hear about reported success. Nothing in the run record, nothing
// in the command's exit status, and a modifier held down on their desktop.
func TestAReleaseTheMachineWillNotTakeIsReported(t *testing.T) {
	k := &aKeyboard{refuseUp: map[string]bool{"ctrl": true}}
	err := sendChord(context.Background(), k, "ctrl+c", spec("ctrl", "c"),
		0, aSettle, aTap)
	if err == nil {
		t.Fatal("a chord that left Ctrl held down reported success, which is the " +
			"defect: the routine goes on and every keystroke after it is modified")
	}
	if !strings.Contains(err.Error(), "ctrl") ||
		!strings.Contains(err.Error(), "still be held down") {
		t.Errorf("the failure says %q; it should name the key and say that the "+
			"computer is still holding it", err)
	}
	// AND EVERY OTHER KEY WAS STILL LET GO OF. Reporting the first failure by leaving
	// the rest down would be the same defect with a sentence attached.
	if held := k.stillDown(); len(held) != 1 || held[0] != "ctrl" {
		t.Errorf("after a refused release the computer holds %v, and only the key the "+
			"machine refused should be stuck", held)
	}
}

// AND A STUCK MODIFIER IS WHAT A CALLER HEARS ABOUT FIRST.
//
// Two bad things at once: the keystroke was refused AND a modifier would not release. The
// refusal is something that did not happen; the stuck key is something that is still happening,
// to everything typed afterwards. So it leads, and the refusal is kept underneath it.
func TestAStuckModifierOutranksARefusedKeystroke(t *testing.T) {
	k := &aKeyboard{
		refuseDown: map[string]bool{"c": true},
		refuseUp:   map[string]bool{"ctrl": true},
	}
	err := sendChord(context.Background(), k, "ctrl+c", spec("ctrl", "c"),
		0, aSettle, aTap)
	if err == nil {
		t.Fatal("a chord that was refused AND left Ctrl down reported success")
	}
	if !strings.HasPrefix(err.Error(), "the machine would not release") {
		t.Errorf("the failure leads with %q; the key still held down is the half "+
			"somebody has to do something about", err)
	}
	if !strings.Contains(err.Error(), "refused") {
		t.Errorf("the failure says %q and drops the refused keystroke entirely", err)
	}
}

// AND A STOP KEEPS ITS IDENTITY EVEN WHEN IT LEAVES A KEY STUCK.
//
// Both at once, in the order that matters: a run stopped mid-chord must still read as stopped —
// otherwise pressing Escape turns into a routine that failed — and the stuck key must still be
// said out loud in the same sentence.
func TestAStopThatLeavesAKeyStuckIsStillAStopAndSaysSo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	k := &aKeyboard{
		stopOnSettle: 1,
		stop:         cancel,
		refuseUp:     map[string]bool{"ctrl": true},
	}
	err := sendChord(ctx, k, "ctrl+c", spec("ctrl", "c"), 0, aSettle, aTap)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("a stopped chord answered %v and no longer reads as a stop", err)
	}
	if !strings.Contains(err.Error(), "still be held down") {
		t.Errorf("the stop says %q and never mentions the key it left held down", err)
	}
}

// AND AN EMPTY CHORD IS REFUSED RATHER THAN SENT.
func TestAnEmptyChordIsNotAKeystroke(t *testing.T) {
	k := &aKeyboard{}
	if err := sendChord(context.Background(), k, "", nil, 0, aSettle, aTap); err == nil {
		t.Error("an empty key spec was accepted")
	}
	if len(k.events) != 0 {
		t.Errorf("the machine was given %v for an empty key spec", k.events)
	}
}
