package simplify

// A MODIFIER HELD ACROSS A POINTER ACTION IS A HOLD, AND ONE HELD AROUND A KEY IS STILL A CHORD.
//
// # The defect these were written for
//
// Demonstrate Ctrl, then a click, then release Ctrl, and what came out was A PLAIN CLICK. The
// modifier was tracked for chord-folding, found no key to fold into, and was discarded — so the
// routine performed a different act from the one that was shown, and read as a faithful recording
// of it. Ctrl+click and click are different in most applications: one adds to a selection, one
// replaces it.
//
// # Why there are two halves to check and not one
//
// `markHolds` decides WHICH presses are holds, and `lower` decides what gets written down. The
// modifier branch in `lower` used to run before the hold markers were read and `continue` past
// them, so teaching the classifier about modifiers on its own would have changed nothing at all —
// the marks would have been right and still swallowed. Both are exercised here through the real
// `Simplify`, which is the only way to tell a fix from half of one.
//
// # And the case that has to keep working
//
// `ctrl+shift+a` is ONE step. The modifiers around a key fold into it and the chord presses and
// releases them itself, so a chord whose modifiers were ALSO written as holds would press Ctrl
// twice and release it in the middle of its own expansion. That is why a modifier is a hold only
// when it spans a pointer action, and why the duration rule that applies to an ordinary key
// deliberately does not apply here.
//
// See [[ADR-258-a-chord-is-a-promise-to-let-go]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// shapeOf is the steps as one line, for a case to be asserted and a failure to be read.
//
// Kind, plus the key where there is one. Waits are dropped: every case here is about what the
// gesture became and none is about its timing, and a shape carrying rounded waits would have to
// be re-asserted every time an unrelated default moved.
func shapeOf(steps []macroir.Step) string {
	var parts []string
	for _, s := range steps {
		if s.Kind == macroir.StepWait {
			continue
		}
		d := string(s.Kind)
		if s.Key != "" {
			d += "(" + s.Key + ")"
		}
		parts = append(parts, d)
	}
	return strings.Join(parts, " ")
}

// aModifierAround builds the demonstration: modifiers down, something in the middle, modifiers up
// in reverse. `middle` supplies the events between them.
func aModifierAround(middle []recorder.RecordedEvent, mods ...string) []recorder.RecordedEvent {
	var ev []recorder.RecordedEvent
	for i, m := range mods {
		ev = append(ev, key(m, true, 100+i*20))
	}
	ev = append(ev, middle...)
	for i := len(mods) - 1; i >= 0; i-- {
		ev = append(ev, key(mods[i], false, 700+(len(mods)-1-i)*20))
	}
	return ev
}

// aClickInTheMiddle is a press and release, well inside any modifier around it.
func aClickInTheMiddle() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{click("left", true, 10, 20, 400),
		click("left", false, 10, 20, 440)}
}

func TestAModifierHeldAcrossAClickBecomesAHold(t *testing.T) {
	for _, c := range []struct {
		name string
		mods []string
		want string
	}{
		{"ctrl+click", []string{"ctrl"}, "keydown(ctrl) click keyup(ctrl)"},
		{"shift+click", []string{"shift"}, "keydown(shift) click keyup(shift)"},
		{"alt+click", []string{"alt"}, "keydown(alt) click keyup(alt)"},
		// BOTH OF THEM, AND LET GO OF IN REVERSE. The order is the gesture: a
		// routine that released Ctrl before Shift would be undoing its own hold in
		// the wrong order, which is the thing `sendChord` is careful about for a
		// chord and which a pair of holds has to get right by itself.
		{"ctrl+shift+click", []string{"ctrl", "shift"},
			"keydown(ctrl) keydown(shift) click keyup(shift) keyup(ctrl)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := shapeOf(simplified(t, aModifierAround(aClickInTheMiddle(),
				c.mods...), DefaultOptions()))
			if got != c.want {
				t.Errorf("a demonstrated %s came out as:\n  %s\nwant:\n  %s",
					c.name, got, c.want)
			}
		})
	}
}

// AND A PLAIN CLICK IS STILL A PLAIN CLICK.
//
// The control for the case above. Without it, a change that wrote a hold around every click would
// pass all four.
func TestAClickWithNothingHeldIsStillJustAClick(t *testing.T) {
	if got := shapeOf(simplified(t, aClickInTheMiddle(), DefaultOptions())); got != "click" {
		t.Errorf("a plain click came out as %q", got)
	}
}

// AND `ctrl+shift+a` IS STILL ONE CHORD.
//
// The case that must not change, and the reason a modifier is a hold only when it spans a POINTER
// action. If the modifiers around a key also became holds, this routine would press Ctrl, press
// Ctrl again inside the chord, release it there, and then release a key that was already up.
func TestModifiersAroundAKeyAreStillOneChord(t *testing.T) {
	ev := aModifierAround([]recorder.RecordedEvent{key("a", true, 300), key("a", false, 340)},
		"ctrl", "shift")
	got := shapeOf(simplified(t, ev, DefaultOptions()))
	if got != "key(ctrl+shift+a)" {
		t.Errorf("ctrl+shift+a came out as:\n  %s\nwant one chord step", got)
	}
}

// AND A MODIFIER HELD A LONG TIME OVER NOTHING IS STILL NOTHING.
//
// The duration rule belongs to ordinary keys: a key held half a second is a charge-hold even with
// nothing between its edges. A modifier is different — held over nothing it modified nothing —
// and a person resting a finger on Shift while they think about what to demonstrate should not
// acquire a stray hold in their routine.
func TestAModifierHeldOverNothingIsNotAHold(t *testing.T) {
	ev := []recorder.RecordedEvent{key("shift", true, 100), key("shift", false, 3000)}
	if got := shapeOf(simplified(t, ev, DefaultOptions())); got != "" {
		t.Errorf("a modifier held for three seconds over nothing came out as %q", got)
	}
}

// AND A MODIFIER HELD ACROSS A KEY *AND THEN* A CLICK IS ONE HOLD, PRESSED ONCE.
//
// The mixed case, and the one with two ways to go wrong at the same time. Ctrl spans the click,
// so it is a hold; the `a` inside it must NOT be expanded into `ctrl+a`, because that chord would
// press Ctrl a second time and RELEASE IT before the click — leaving the click unmodified while
// every step looked correct.
//
// It must also not carry Text. Text is what lets a run of keys coalesce into a `Type` step, and
// `Type` sends characters rather than keystrokes, so `a` under a held Ctrl would arrive as the
// letter instead of as Ctrl+A — after a fold that happens two passes later and that nothing at
// this point can see.
func TestAModifierAcrossAKeyAndThenAClickIsPressedOnce(t *testing.T) {
	ev := aModifierAround([]recorder.RecordedEvent{
		key("a", true, 200), key("a", false, 240),
		click("left", true, 10, 20, 500), click("left", false, 10, 20, 540),
	}, "ctrl")
	steps := simplified(t, ev, DefaultOptions())
	if got := shapeOf(steps); got != "keydown(ctrl) key(a) click keyup(ctrl)" {
		t.Errorf("Ctrl held across a keypress and then a click came out as:\n  %s\n"+
			"want keydown(ctrl) key(a) click keyup(ctrl)", got)
	}
	for _, s := range steps {
		switch {
		case s.Kind == macroir.StepKey && strings.Contains(s.Key, "+"):
			t.Errorf("the key under the hold was expanded into the chord %q, which "+
				"presses the modifier again and releases it before the click",
				s.Key)
		case s.Kind == macroir.StepKey && s.Text != "":
			t.Errorf("the key under the hold carries Text %q, so it can coalesce "+
				"into a Type step — and Type sends the character, not the "+
				"keystroke, so it would arrive unmodified", s.Text)
		case s.Kind == macroir.StepType:
			t.Errorf("the key under the hold became a Type step (%q)", s.Text)
		}
	}
}

// AND SHIFT FOR A CAPITAL LETTER IS STILL TYPING.
//
// The other side of the case above, and the reason the hold rule is about pointer actions rather
// than about modifiers in general. Shift held over a letter is how a capital is typed; turning it
// into a hold would make every demonstrated capital letter a three-step gesture.
func TestShiftForACapitalLetterIsStillTyping(t *testing.T) {
	ev := aModifierAround([]recorder.RecordedEvent{key("a", true, 200), key("a", false, 240)},
		"shift")
	steps := simplified(t, ev, DefaultOptions())
	if got := shapeOf(steps); got != "key(a)" {
		t.Errorf("a shifted letter came out as %q", got)
	}
	if len(steps) != 1 || steps[0].Text != "A" {
		t.Errorf("a shifted letter lost its capital: %+v", steps)
	}
}

// ── AND WHAT CANNOT BE WRITTEN DOWN IS REFUSED, WITH A REASON ──────────────────────────
//
// Three ways a modifier around a pointer action cannot be represented, all of them about not
// knowing WHEN it was held. Each is refused rather than quietly dropped, because dropping it is
// the original defect: a routine that does something else and looks right.
//
// Each case has a control beside it where the same shape is harmless, because the refusals must
// not fire on a recording that lost nothing — a stray modifier release before anything happened
// modified nothing, and refusing it would refuse a recording for no reason.

func TestAModifierLeftDownOverAClickIsRefused(t *testing.T) {
	ev := []recorder.RecordedEvent{key("ctrl", true, 100),
		click("left", true, 10, 20, 400), click("left", false, 10, 20, 440)}
	_, err := Simplify(ev, DefaultOptions())
	if err == nil {
		t.Fatal("a recording that presses Ctrl, clicks, and never lets go was " +
			"accepted — so a hold with no release, or a lost modifier, reaches a " +
			"routine")
	}
	for _, want := range []string{"ctrl", "never released", "down on the desktop"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %s", want, err)
		}
	}
	// THE CONTROL: the same unreleased modifier with nothing under it is fine. It
	// modified nothing, so nothing was lost.
	if _, err := Simplify([]recorder.RecordedEvent{key("ctrl", true, 100)},
		DefaultOptions()); err != nil {
		t.Errorf("an unreleased modifier over nothing was refused: %v", err)
	}
}

func TestAModifierAlreadyHeldWhenRecordingBeganIsRefused(t *testing.T) {
	ev := []recorder.RecordedEvent{
		click("left", true, 100, 20, 100), click("left", false, 100, 20, 140),
		key("ctrl", false, 400)}
	_, err := Simplify(ev, DefaultOptions())
	if err == nil {
		t.Fatal("a recording whose only evidence of Ctrl is its RELEASE, after a " +
			"click, was accepted — so a click made under a modifier comes out plain")
	}
	for _, want := range []string{"ctrl", "already held", "recording began"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %s", want, err)
		}
	}
	// THE CONTROL: the release before anything happened. Nothing was pressed under it.
	after := []recorder.RecordedEvent{key("ctrl", false, 100),
		click("left", true, 10, 20, 400), click("left", false, 10, 20, 440)}
	if _, err := Simplify(after, DefaultOptions()); err != nil {
		t.Errorf("a stray modifier release before any press was refused: %v", err)
	}
}

func TestAModifierLetGoOfInTheMiddleOfAClickIsRefused(t *testing.T) {
	ev := []recorder.RecordedEvent{key("ctrl", true, 100),
		click("left", true, 10, 20, 400),
		key("ctrl", false, 420),
		click("left", false, 10, 20, 440)}
	_, err := Simplify(ev, DefaultOptions())
	if err == nil {
		t.Fatal("Ctrl released between the mouse going down and coming up was " +
			"accepted; holding it across the press and letting go before it are " +
			"different gestures and the recording does not say which")
	}
	for _, want := range []string{"ctrl", "middle of a press", "until after the click"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %s", want, err)
		}
	}
	// AND THE OTHER END OF THE SAME AMBIGUITY: pressed in the middle of a press.
	//
	// The application read the keyboard when the button went down, so the press was
	// unmodified — and a hold written around it would claim a gesture that did not
	// happen. Refused for the same reason and in the same words as the case above,
	// and together the two are why it cannot matter which edge of a click
	// spansAPointerAction tests: every recording where the two edges disagree is
	// refused before the classification is reached.
	late := []recorder.RecordedEvent{
		click("left", true, 10, 20, 100),
		key("ctrl", true, 120),
		click("left", false, 10, 20, 140),
		key("ctrl", false, 400)}
	if _, err := Simplify(late, DefaultOptions()); err == nil {
		t.Error("Ctrl pressed between the mouse going down and coming up was " +
			"accepted, so a plain press came out wearing a modifier it never had")
	}

	// THE CONTROL: let go of after the click, which is the ordinary gesture.
	if _, err := Simplify(aModifierAround(aClickInTheMiddle(), "ctrl"),
		DefaultOptions()); err != nil {
		t.Errorf("an ordinary Ctrl+click was refused: %v", err)
	}
}

// AND A REFUSED RECORDING STILL COMES BACK WITH ITS STEPS.
//
// So review can show what was understood of it. The person performed the gesture and is the only
// one who can perform it again differently; "nothing happened" would be a worse answer than the
// steps plus the reason they cannot be kept. Nothing downstream may save them while the reason is
// set, which `Save` holds.
func TestARefusedRecordingStillSaysWhatItUnderstood(t *testing.T) {
	ev := []recorder.RecordedEvent{appSwitch("fixture", 0), key("ctrl", true, 100),
		click("left", true, 10, 20, 400), click("left", false, 10, 20, 440)}
	steps, err := Simplify(ev, DefaultOptions())
	if err == nil {
		t.Fatal("this recording should have been refused")
	}
	if len(steps) == 0 {
		t.Error("a refused recording came back with no steps, so review can only " +
			"say that something was wrong and not what was seen")
	}
}
