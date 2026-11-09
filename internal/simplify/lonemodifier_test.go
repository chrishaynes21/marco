package simplify

// A MODIFIER PRESSED WITH NOTHING UNDER IT IS A KEY OF ITS OWN.
//
// The slide melee that found it: W held, Shift tapped (sprint), Ctrl tapped (slide), the forward
// button (melee). The recording had all of it; the saved routine went from W straight to the click,
// because a modifier was only ever state for the next key's chord, and nothing came before it was
// let go of. See ADR-339.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// The events of the owner's recording, at its own timings, relative to the first.
func aSlideMelee() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		key("w", true, 0),
		key("shift", true, 200), key("shift", false, 366),
		key("ctrl", true, 561), key("ctrl", false, 729),
		click("forward", true, 300, 572, 806), move(302, 572, 827), move(304, 572, 854),
		click("forward", false, 304, 572, 984),
		key("d", true, 1203), key("d", false, 1209),
		key("w", false, 1213),
	}
}

// Dropping a lone modifier, either tap, or putting them out of order, must fail this.
func TestASlideMeleeKeepsItsShiftAndItsCtrl(t *testing.T) {
	got := shapeOf(simplified(t, aSlideMelee(), DefaultOptions()))
	want := "keydown(w) key(shift) key(ctrl) click key(d) keyup(w)"
	if got != want {
		t.Fatalf("the slide melee came out as:\n  %s\nwant:\n  %s", got, want)
	}
}

// A MODIFIER HELD ALONE IS HELD ONCE, however many key-repeat downs Windows sent while it was.
// Counting a repeat as a press of its own must fail this, as must counting it as another key.
func TestAHeldModifierWithItsRepeatsIsOneHold(t *testing.T) {
	ev := []recorder.RecordedEvent{key("shift", true, 100)}
	for ms := 600; ms < 2000; ms += 33 {
		ev = append(ev, key("shift", true, ms))
	}
	ev = append(ev, key("shift", false, 2100))
	if got := shapeOf(simplified(t, ev, DefaultOptions())); got != "keydown(shift) keyup(shift)" {
		t.Fatalf("shift held two seconds came out as %q", got)
	}
}

// AND A MODIFIER UNDER WHICH SOMETHING HAPPENED IS WHAT IT WAS: Ctrl+S is one chord, with no
// extra Ctrl pressed before it.
func TestAModifierUsedByAKeyIsNotAlsoPressedAlone(t *testing.T) {
	ev := []recorder.RecordedEvent{key("ctrl", true, 100), key("s", true, 150),
		key("s", false, 200), key("ctrl", false, 250)}
	if got := shapeOf(simplified(t, ev, DefaultOptions())); got != "key(ctrl+s)" {
		t.Fatalf("ctrl+s came out as %q", got)
	}
}

// CTRL AND SHIFT TAPPED TOGETHER WITH NOTHING ELSE ARE ONE COMBO, named in chord order whichever
// went down first. Dropping it, or pressing them as two separate taps, must fail this.
func TestCtrlAndShiftTappedTogetherAreOneCombo(t *testing.T) {
	ev := []recorder.RecordedEvent{key("shift", true, 100), key("ctrl", true, 120),
		key("shift", false, 220), key("ctrl", false, 240)}
	if got := shapeOf(simplified(t, ev, DefaultOptions())); got != "key(ctrl+shift)" {
		t.Fatalf("ctrl and shift tapped together came out as %q", got)
	}
}

// AND HELD TOGETHER THEY ARE EACH HELD, pressed and let go of in the order they were.
func TestCtrlAndShiftHeldTogetherAreEachHeld(t *testing.T) {
	ev := []recorder.RecordedEvent{key("shift", true, 100), key("ctrl", true, 150),
		key("ctrl", false, 1200), key("shift", false, 1300)}
	got := shapeOf(simplified(t, ev, DefaultOptions()))
	if want := "keydown(shift) keydown(ctrl) keyup(ctrl) keyup(shift)"; got != want {
		t.Fatalf("ctrl and shift held together came out as:\n  %s\nwant:\n  %s", got, want)
	}
}

// A KEY HELD AROUND THE TAP IS NOT A KEY PRESSED UNDER IT, even when Windows repeats that key's
// down while the Shift is tapped: W held to run forward, Shift tapped to sprint.
func TestARepeatOfAKeyHeldAroundAShiftTapIsNotAKeyUnderIt(t *testing.T) {
	ev := []recorder.RecordedEvent{key("w", true, 0), key("shift", true, 600),
		key("w", true, 620), key("w", true, 653), key("shift", false, 700), key("w", false, 1500)}
	// The repeats themselves are written as more presses of W, which is how a held key is
	// replayed with its repeats; what this asks is only that the Shift survives them.
	got := shapeOf(simplified(t, ev, DefaultOptions()))
	if !strings.HasPrefix(got, "keydown(w) key(shift)") || !strings.HasSuffix(got, "keyup(w)") {
		t.Fatalf("W held around a Shift tap came out as %q", got)
	}
}

// AND A COMBO WITH A KEY UNDER IT IS STILL THAT KEY'S CHORD.
func TestCtrlShiftWithAKeyUnderItIsStillOneChord(t *testing.T) {
	ev := []recorder.RecordedEvent{key("ctrl", true, 100), key("shift", true, 120),
		key("esc", true, 150), key("esc", false, 200), key("shift", false, 220), key("ctrl", false, 240)}
	if got := shapeOf(simplified(t, ev, DefaultOptions())); got != "key(ctrl+shift+esc)" {
		t.Fatalf("ctrl+shift+esc came out as %q", got)
	}
}

// A GROUP THAT MODIFIED SOMETHING DOES NOT SPOIL THE NEXT ONE: Ctrl+S, then Shift tapped alone.
func TestATapAfterAChordIsStillATap(t *testing.T) {
	ev := []recorder.RecordedEvent{key("ctrl", true, 100), key("s", true, 150),
		key("s", false, 200), key("ctrl", false, 250), key("shift", true, 600), key("shift", false, 700)}
	if got := shapeOf(simplified(t, ev, DefaultOptions())); got != "key(ctrl+s) key(shift)" {
		t.Fatalf("ctrl+s then a shift tap came out as %q", got)
	}
}
