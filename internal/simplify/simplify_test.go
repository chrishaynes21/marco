package simplify

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// TestArgKeyPlaceholder: tapping the reserved arg key (F9) during a demo drops a
// positional {{N}} Type step (Nth tap → {{N}}) and the key itself never leaks into
// the macro as a press.
func TestArgKeyPlaceholder(t *testing.T) {
	events := []recorder.RecordedEvent{
		key("h", true, 0), key("h", false, 20),
		key("f9", true, 100), key("f9", false, 150), // arg 1
		key("f9", true, 300), key("f9", false, 350), // arg 2
	}
	steps := Simplify(events, DefaultOptions())
	var typed []string
	for _, s := range steps {
		if s.Kind == macroir.StepType {
			typed = append(typed, s.Text)
		}
		if s.Kind == macroir.StepKey && s.Key == "f9" {
			t.Fatalf("arg key leaked as a key press: %+v", steps)
		}
	}
	joined := strings.Join(typed, "|")
	if !strings.Contains(joined, "{{1}}") || !strings.Contains(joined, "{{2}}") {
		t.Fatalf("arg key not converted to {{1}}/{{2}}; Type steps: %v\nall: %+v", typed, steps)
	}
}

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

func appSwitch(app string, ms int) recorder.RecordedEvent {
	return recorder.RecordedEvent{Kind: recorder.EvAppSwitch, KeyName: app, T: at(ms)}
}

// TestAppSwitchDropsNavClicks is the taskbar-click scenario: you start teaching
// in one app, click the taskbar to switch to another (a brittle fixed-coordinate
// click — and on a left/upper monitor, negative coords), then type. The recorded
// route should focus the destination app robustly and NOT replay the navigation
// click. The leading "code" activate collapses into the switch to notepad.
func TestAppSwitchDropsNavClicks(t *testing.T) {
	evs := []recorder.RecordedEvent{
		appSwitch("code", 0),
		click("left", true, -287, 739, 100), // taskbar click on a secondary monitor
		click("left", false, -287, 739, 150),
		appSwitch("notepad", 300),
		key("h", true, 400), key("h", false, 402),
		key("i", true, 410), key("i", false, 412),
	}
	got := Simplify(evs, DefaultOptions())
	want := []macroir.Step{
		{Kind: macroir.StepActivate, Text: "notepad"},
		{Kind: macroir.StepType, Text: "hi"},
	}
	assertSteps(t, got, want)
}

// TestInAppClickSurvivesSwitch checks that a legitimate click made *in* an app is
// kept; only the gesture immediately before a switch (the navigation) is dropped,
// and a switch-away at the very end (nothing after it) is discarded.
func TestInAppClickSurvivesSwitch(t *testing.T) {
	evs := []recorder.RecordedEvent{
		appSwitch("notepad", 0),
		key("h", true, 10), key("h", false, 12),
		key("i", true, 20), key("i", false, 22),
		click("left", true, 50, 60, 100), // a real in-app click (e.g. Save)
		click("left", false, 50, 60, 150),
		click("left", true, -300, 740, 200), // taskbar click → switch
		click("left", false, -300, 740, 250),
		appSwitch("chrome", 400), // switch away at the end with nothing after
	}
	got := Simplify(evs, DefaultOptions())
	want := []macroir.Step{
		{Kind: macroir.StepActivate, Text: "notepad"},
		{Kind: macroir.StepType, Text: "hi"},
		{Kind: macroir.StepWait, Ms: 80},
		{Kind: macroir.StepClick, X: 50, Y: 60, Button: "left"},
	}
	assertSteps(t, got, want)
}

func rhythmOpts() Options {
	o := DefaultOptions()
	o.TypingRhythmMs = DefaultTypingRhythmMs
	return o
}

// TestTypingRhythmFolds: the default keeps per-letter keys + rhythm waits
// (faithful, and "ll" even folds into a loop); with TypingRhythmMs on, the whole
// varied run collapses into one clean Type step — and because rhythm-folding runs
// before loop-folding, the repeated "l" never becomes a loop.
func TestTypingRhythmFolds(t *testing.T) {
	// "hello" typed at human speed: gaps >MinWaitMs (so waits appear by default)
	// but <DefaultTypingRhythmMs (so they fold).
	evs := []recorder.RecordedEvent{
		key("h", true, 0), key("h", false, 2),
		key("e", true, 140), key("e", false, 142),
		key("l", true, 220), key("l", false, 222),
		key("l", true, 310), key("l", false, 312),
		key("o", true, 400), key("o", false, 402),
	}
	if faithful := Simplify(evs, DefaultOptions()); len(faithful) < 4 {
		t.Fatalf("expected faithful per-letter steps, got %d: %+v", len(faithful), faithful)
	}
	got := Simplify(evs, rhythmOpts())
	want := []macroir.Step{{Kind: macroir.StepType, Text: "hello"}}
	assertSteps(t, got, want)
}

// TestTypingRhythmKeepsUniformSpam: a uniform key run with gaps (game spam) is
// NOT folded into typing even with rhythm-folding on — its timing is preserved.
func TestTypingRhythmKeepsUniformSpam(t *testing.T) {
	evs := []recorder.RecordedEvent{
		key("e", true, 0), key("e", false, 2),
		key("e", true, 140), key("e", false, 142),
		key("e", true, 280), key("e", false, 282),
	}
	got := Simplify(evs, rhythmOpts())
	for _, s := range got {
		if s.Kind == macroir.StepType {
			t.Fatalf("uniform spam was wrongly folded into typing: %+v", got)
		}
	}
}

// TestAggressiveFoldsWordGap: a pause between words (here 320ms — longer than
// DefaultTypingRhythmMs, so the default rhythm fold leaves it) is folded away
// under AggressiveTypingRhythmMs, the value the explicit "simplify" paths use.
func TestAggressiveFoldsWordGap(t *testing.T) {
	evs := []recorder.RecordedEvent{
		key("h", true, 0), key("h", false, 2),
		key("i", true, 90), key("i", false, 92),
		key("space", true, 180), key("space", false, 182),
		// deliberate-looking 320ms gap between the two words
		key("y", true, 502), key("y", false, 504),
		key("o", true, 590), key("o", false, 592),
	}
	o := DefaultOptions()
	o.TypingRhythmMs = AggressiveTypingRhythmMs
	got := Simplify(evs, o)
	want := []macroir.Step{{Kind: macroir.StepType, Text: "hi yo"}}
	assertSteps(t, got, want)
}

// TestClickTemplateCarried: a captured patch on a click-down survives lowering
// onto the click step, so codegen can turn it into an image find. A drag (moves
// between down and up) does not carry one.
func TestClickTemplateCarried(t *testing.T) {
	patch := []byte("PNGBYTES")
	evs := []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 50, Y: 60, T: at(0), Image: patch},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 50, Y: 60, T: at(5)},
	}
	got := Simplify(evs, DefaultOptions())
	if len(got) != 1 || got[0].Kind != macroir.StepClick {
		t.Fatalf("expected one click, got %+v", got)
	}
	if string(got[0].Template) != string(patch) {
		t.Fatalf("click did not carry its template: %+v", got[0])
	}
}

// TestCtrlComboBecomesChord: Ctrl+C records as a single "ctrl+c" key chord, not
// as the letter "c" typed. The modifier is held across the key, not dropped.
func TestCtrlComboBecomesChord(t *testing.T) {
	evs := []recorder.RecordedEvent{
		key("ctrl", true, 0),
		key("c", true, 20), key("c", false, 25),
		key("ctrl", false, 40),
	}
	got := Simplify(evs, DefaultOptions())
	want := []macroir.Step{{Kind: macroir.StepKey, Key: "ctrl+c", Count: 1}}
	assertSteps(t, got, want)
}

// TestCtrlShiftCombo: multiple held modifiers compose in a stable order.
func TestCtrlShiftCombo(t *testing.T) {
	evs := []recorder.RecordedEvent{
		key("ctrl", true, 0),
		key("shift", true, 5),
		key("esc", true, 20), key("esc", false, 25),
		key("shift", false, 40),
		key("ctrl", false, 45),
	}
	got := Simplify(evs, DefaultOptions())
	want := []macroir.Step{{Kind: macroir.StepKey, Key: "ctrl+shift+esc", Count: 1}}
	assertSteps(t, got, want)
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
