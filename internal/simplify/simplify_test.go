package simplify

import (
	"reflect"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func at(ms int) time.Time { return base.Add(time.Duration(ms) * time.Millisecond) }

func key(name string, down bool, ms int) recorder.RecordedEvent {
	return recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: name, Down: down, T: at(ms)}
}
func click(btn string, down bool, x, y, ms int) recorder.RecordedEvent {
	return recorder.RecordedEvent{Kind: recorder.EvClick, Button: btn, Down: down, X: x, Y: y, T: at(ms)}
}
func move(x, y, ms int) recorder.RecordedEvent {
	return recorder.RecordedEvent{Kind: recorder.EvMove, X: x, Y: y, T: at(ms)}
}

func TestClicksAndWait(t *testing.T) {
	evs := []recorder.RecordedEvent{
		click("left", true, 100, 200, 0), click("left", false, 100, 200, 5),
		click("left", true, 400, 500, 350), click("left", false, 400, 500, 355),
	}
	got := Simplify(evs, DefaultOptions())
	want := []macroir.Step{
		{Kind: macroir.StepClick, X: 100, Y: 200, Button: "left"},
		{Kind: macroir.StepWait, Ms: 350},
		{Kind: macroir.StepClick, X: 400, Y: 500, Button: "left"},
	}
	assertSteps(t, got, want)
}

func TestUniformKeyCoalesces(t *testing.T) {
	// game-style "e" spam → key e count 3, not type "eee"
	var evs []recorder.RecordedEvent
	for i := 0; i < 3; i++ {
		evs = append(evs, key("e", true, i*10), key("e", false, i*10+2))
	}
	got := Simplify(evs, DefaultOptions())
	want := []macroir.Step{{Kind: macroir.StepKey, Key: "e", Count: 3, Text: "e"}}
	assertSteps(t, got, want)
}

func TestVariedKeysBecomeTyping(t *testing.T) {
	evs := []recorder.RecordedEvent{
		key("h", true, 0), key("h", false, 2),
		key("i", true, 8), key("i", false, 10),
	}
	got := Simplify(evs, DefaultOptions())
	want := []macroir.Step{{Kind: macroir.StepType, Text: "hi"}}
	assertSteps(t, got, want)
}

func TestNamedKeyStaysKey(t *testing.T) {
	evs := []recorder.RecordedEvent{key("enter", true, 0), key("enter", false, 2)}
	got := Simplify(evs, DefaultOptions())
	want := []macroir.Step{{Kind: macroir.StepKey, Key: "enter", Count: 1}}
	assertSteps(t, got, want)
}

func TestDragDetected(t *testing.T) {
	evs := []recorder.RecordedEvent{
		click("left", true, 10, 10, 0),
		move(50, 50, 20), move(100, 100, 40),
		click("left", false, 100, 100, 60),
	}
	got := Simplify(evs, DefaultOptions())
	want := []macroir.Step{{Kind: macroir.StepDrag, X: 10, Y: 10, X2: 100, Y2: 100, Button: "left"}}
	assertSteps(t, got, want)
}

func TestHoverMovesDiscarded(t *testing.T) {
	evs := []recorder.RecordedEvent{
		move(10, 10, 0), move(200, 200, 20),
		click("left", true, 200, 200, 40), click("left", false, 200, 200, 45),
	}
	got := Simplify(evs, DefaultOptions())
	want := []macroir.Step{{Kind: macroir.StepClick, X: 200, Y: 200, Button: "left"}}
	assertSteps(t, got, want)
}

func TestCycleFold(t *testing.T) {
	// [click A; wait 100; click B] repeated 3 times → one loop of count 3.
	// Reps are tightly adjacent (inter-rep gap 15ms < MinWait, dropped) so the
	// 3-step block tiles cleanly.
	var evs []recorder.RecordedEvent
	for i := 0; i < 3; i++ {
		t0 := i * 115
		evs = append(evs,
			click("left", true, 0, 0, t0), click("left", false, 0, 0, t0+5),
			click("left", true, 9, 9, t0+100), click("left", false, 9, 9, t0+105),
		)
	}
	got := Simplify(evs, DefaultOptions())
	if len(got) != 1 || got[0].Kind != macroir.StepLoop || got[0].Count != 3 {
		t.Fatalf("expected one loop count 3, got %+v", got)
	}
	body := got[0].Steps
	want := []macroir.Step{
		{Kind: macroir.StepClick, X: 0, Y: 0, Button: "left"},
		{Kind: macroir.StepWait, Ms: 100},
		{Kind: macroir.StepClick, X: 9, Y: 9, Button: "left"},
	}
	assertSteps(t, body, want)
}

func TestNearRepeatNotFolded(t *testing.T) {
	// Differing coords must NOT fold.
	var evs []recorder.RecordedEvent
	coords := []int{0, 50, 99}
	for i, c := range coords {
		t0 := i * 200
		evs = append(evs, click("left", true, c, c, t0), click("left", false, c, c, t0+5))
	}
	got := Simplify(evs, DefaultOptions())
	if len(got) != 5 { // click, wait, click, wait, click
		t.Fatalf("expected 5 steps (no fold), got %d: %+v", len(got), got)
	}
}

func TestShiftedPlaceholderRecords(t *testing.T) {
	// Typing {{pw}} = shift+[ shift+[ p w shift+] shift+] must record the braces.
	evs := []recorder.RecordedEvent{
		key("shift", true, 0),
		key("[", true, 5), key("[", true, 10),
		key("shift", false, 15),
		key("p", true, 20), key("w", true, 25),
		key("shift", true, 30),
		key("]", true, 35), key("]", true, 40),
		key("shift", false, 45),
	}
	got := Simplify(evs, DefaultOptions())
	want := []macroir.Step{{Kind: macroir.StepType, Text: "{{pw}}"}}
	assertSteps(t, got, want)
}

func assertSteps(t *testing.T, got, want []macroir.Step) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("steps mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}
