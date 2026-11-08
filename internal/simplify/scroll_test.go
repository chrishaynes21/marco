package simplify

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// wheel is a turn of the wheel at x,y, ms after the start.
func wheel(delta int, horizontal bool, x, y, ms int) recorder.RecordedEvent {
	return recorder.RecordedEvent{Kind: recorder.EvScroll, Delta: delta, Horizontal: horizontal,
		X: x, Y: y, T: at(ms)}
}

// A RUN OF NOTCHES IS ONE STEP, AND A PAUSE OR A CHANGE OF WAY IS ANOTHER: three notches down in
// quick succession are `scroll down 3`; a notch up after them is its own step; a horizontal tilt
// is left or right; and a notch after a long pause is a second turn, not a fourth notch.
//
// Each notch its own step, or the direction lost, must fail this.
func TestARunOfNotchesIsOneScroll(t *testing.T) {
	steps := simplified(t, []recorder.RecordedEvent{
		wheel(-120, false, 400, 300, 100), wheel(-120, false, 401, 300, 140),
		wheel(-120, false, 400, 301, 180),
		wheel(120, false, 400, 300, 220),
		wheel(-120, true, 400, 300, 260),
		wheel(-240, false, 400, 300, 1500), // after a pause: a second turn, of two notches
	}, DefaultOptions())
	var got []string
	for _, s := range steps {
		if s.Kind == macroir.StepScroll {
			got = append(got, s.Wheel+" "+string(rune('0'+s.Count)))
		}
	}
	want := []string{"down 3", "up 1", "left 1", "down 2"}
	if len(got) != len(want) {
		t.Fatalf("the wheel came out as %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the wheel came out as %v, want %v", got, want)
		}
	}
}

// CTRL HELD ACROSS A TURN OF THE WHEEL IS A HOLD AROUND THE SCROLL, as it is around a click:
// ctrl+wheel is zoom in most things, and a game binds it too. Without it the modifier would be
// dropped as having modified nothing.
//
// Not counting a scroll as an action a modifier is held across must fail this.
func TestCtrlHeldAcrossTheWheelIsAHold(t *testing.T) {
	got := shapeOf(simplified(t, aModifierAround([]recorder.RecordedEvent{
		wheel(120, false, 10, 20, 400)}, "ctrl"), DefaultOptions()))
	if want := "keydown(ctrl) scroll keyup(ctrl)"; got != want {
		t.Errorf("ctrl+wheel came out as %q, want %q", got, want)
	}
}
