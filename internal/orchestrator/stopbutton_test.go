package orchestrator

// ENDING A RECORDING IS NOT PART OF THE RECORDING.
//
// A recording has always been ended by a gesture — F12 — and a key is not a place: pressing it
// leaves keypresses at the end of the stream and `stripTrailingStop` drops them. A BUTTON is a
// place. To press it a person brings Marco's window forward and clicks inside it, and the hooks
// record all of that exactly as faithfully as they record the demonstration. Left in, every
// replay of the saved routine would raise Marco and press Stop.
//
// So the rule these hold: a recording ended by the button contains what the person demonstrated
// and nothing about how they stopped. And the gesture path is left exactly as it was, because
// routines recorded by it must keep behaving as they do.

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// heldRecorder publishes its events and then KEEPS THE STREAM OPEN, the way a real recorder does
// while somebody is still demonstrating.
//
// `fakeRecorder` closes its channel, which ends `waitForStop` by itself — so it could not pose
// this question: a recording that stops because the stream ran out is not a recording that was
// stopped by anything.
type heldRecorder struct {
	events []recorder.RecordedEvent
	ch     chan recorder.RecordedEvent
}

func (h *heldRecorder) Start() error {
	h.ch = make(chan recorder.RecordedEvent, len(h.events)+1)
	for _, e := range h.events {
		h.ch <- e
	}
	return nil
}
func (h *heldRecorder) Stop() []recorder.RecordedEvent        { return h.events }
func (h *heldRecorder) Events() <-chan recorder.RecordedEvent { return h.ch }

// aDemonstrationEndedByTheButton is one click in a game, then the trip to Marco's window to press
// Stop: the switch the poller sees, the moves on the way, and the click on the button itself.
func aDemonstrationEndedByTheButton() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", T: at(105)},
		// ── AND NOW THEY GO AND PRESS STOP ──
		{Kind: recorder.EvAppSwitch, KeyName: "studio", T: at(3000)},
		{Kind: recorder.EvMove, X: 1500, Y: 900, T: at(3100)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: "studio", T: at(3200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: "studio", T: at(3205)},
	}
}

// learnByButton records the given stream, ends it with the button, and returns the saved Marco.
func learnByButton(t *testing.T, events []recorder.RecordedEvent) string {
	t.Helper()
	t.Setenv("MARCO_CV", "off") // no anchors: this is about the event tail
	reg := routes.Registry{Dir: t.TempDir()}
	rec := &heldRecorder{events: events}
	press := make(chan struct{})

	var out bytes.Buffer
	d := Deps{
		Reg: reg, Rec: rec, Out: &out,
		Owner:    "studio",
		StopWhen: press,
	}
	// THE BUTTON, pressed once the stream is in. Closed rather than sent on, so the
	// race in `waitForStop` resolves the same way every run.
	go func() {
		time.Sleep(10 * time.Millisecond)
		close(press)
	}()
	if err := d.LearnAuto("ark graphics"); err != nil {
		t.Fatalf("recording: %v", err)
	}
	raw, err := os.ReadFile(reg.Path(routes.Route{App: "arkascended",
		Slug: "ark-graphics"}))
	if err != nil {
		t.Fatalf("loading the saved route: %v (said: %s)", err, out.String())
	}
	return string(raw)
}

// THE CLICK THAT STOPPED THE RECORDING IS NOT IN THE MACRO.
//
// Asserted on the SAVED MARCO rather than on the event slice, because that file is the thing
// that executes. A strip that worked on the events and a codegen that reached the coordinates
// another way would pass a test of the slice and still press Stop on replay.
func TestTheClickThatStoppedTheRecordingIsNotInTheMacro(t *testing.T) {
	src := learnByButton(t, aDemonstrationEndedByTheButton())

	// WHAT WAS DEMONSTRATED IS THERE — and this assertion is load-bearing twice.
	//
	// It says the strip did not overreach. It also pins the ORDER the strip runs
	// in: `simplify.foldActivates` drops the step immediately before an app switch
	// as the gesture that performed it, and the last demonstrated click is exactly
	// there — immediately before the switch to Marco's window. Strip the tail
	// AFTER simplifying and that click is gone, counted as the taskbar press that
	// brought Marco forward. So the tail has to go before simplify ever sees it,
	// and this is what says so.
	if !strings.Contains(src, "842") || !strings.Contains(src, "274") {
		t.Fatalf("the demonstrated click is missing, so this proves nothing "+
			"about the tail:\n%s", src)
	}
	// AND THE PRESS OF STOP IS NOT.
	for _, gone := range []string{"1511", "913"} {
		if strings.Contains(src, gone) {
			t.Errorf("the saved routine contains %s — the click that pressed "+
				"Stop became a step, so every replay presses it again:\n%s",
				gone, src)
		}
	}
	// AND NEITHER IS THE TRIP TO MARCO'S OWN WINDOW.
	//
	// Named separately because it fails separately: an app switch becomes an
	// Activate, and a replay that raised Marco would be wrong even if it then
	// clicked nothing.
	if strings.Contains(strings.ToLower(src), "studio") {
		t.Errorf("the saved routine names Marco's own window, so a replay would "+
			"raise it:\n%s", src)
	}
}

// AND A VISIT TO MARCO IN THE MIDDLE OF A DEMONSTRATION SURVIVES.
//
// The cut is at the LAST switch into Marco and only when the recording ends there. Somebody who
// alt-tabbed away and came back demonstrated everything after they came back, and a strip that
// walked past the first switch it found would delete it.
func TestAVisitToMarcoInTheMiddleIsNotCutAway(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", T: at(105)},
		// ── A KEY, AND THEN A LOOK AT MARCO MID-DEMONSTRATION ──
		//
		// The key is not decoration. `simplify.foldActivates` drops the one step
		// immediately before an app switch as the gesture that PERFORMED it — a
		// taskbar click is navigation, not demonstration — and a move will not
		// shield a click from that rule, because moves are folded away earlier in
		// the pipeline and the click becomes adjacent again. A key is not
		// navigation, so it survives and the click behind it survives with it.
		//
		// That rule predates this work and is not changed by it. What it costs is
		// worth stating: a click made immediately before alt-tabbing away is read
		// as the alt-tab and is lost. The tail strip is what keeps the same rule
		// from eating the LAST demonstrated action of every button-stopped
		// recording, which is always immediately before the switch to Marco.
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, App: "arkascended", T: at(800)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: false, App: "arkascended", T: at(805)},
		{Kind: recorder.EvAppSwitch, KeyName: "studio", T: at(1000)},
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(2000)},
		// ── AND MORE OF THE DEMONSTRATION, which must survive ──
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 496, Y: 283,
			App: "arkascended", T: at(2100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 496, Y: 283,
			App: "arkascended", T: at(2105)},
		// ── THEN STOP ──
		{Kind: recorder.EvAppSwitch, KeyName: "studio", T: at(4000)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: "studio", T: at(4100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: "studio", T: at(4105)},
	}
	src := learnByButton(t, events)

	for _, want := range []string{"842", "274", "496", "283"} {
		if !strings.Contains(src, want) {
			t.Errorf("the saved routine is missing %s: the tail strip ate part "+
				"of the demonstration:\n%s", want, src)
		}
	}
	if strings.Contains(src, "1511") {
		t.Errorf("the press of Stop is still in the routine:\n%s", src)
	}
}

// AND A TRIP TOO QUICK FOR THE POLLER TO SEE IS STILL CUT.
//
// The app switch is produced by a poller on an interval, not by the hooks, so a person who
// reaches Marco and clicks between two samples leaves a click in Marco with no switch in front
// of it. The app each event was tagged with is what catches that one.
func TestAStopPressedBetweenTwoPollsIsStillCut(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", T: at(105)},
		// NO SWITCH RECORDED — the poller never sampled it.
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: "studio", T: at(3200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: "studio", T: at(3205)},
	}
	src := learnByButton(t, events)

	if !strings.Contains(src, "842") {
		t.Fatalf("the demonstrated click is missing:\n%s", src)
	}
	if strings.Contains(src, "1511") {
		t.Errorf("a Stop press the poller never saw a switch for stayed in the "+
			"routine:\n%s", src)
	}
}

// AND THE GESTURE PATH IS LEFT EXACTLY AS IT WAS.
//
// The legacy test. The owner strip runs only for a recording the BUTTON ended, so a recording
// ended by the gesture keeps whatever it always kept — including, if it comes to it, a click
// somebody made in Marco's window. Changing that is a separate decision and this records that it
// was not taken quietly.
func TestAGestureEndedRecordingIsNotStripped(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	events := append(aDemonstrationEndedByTheButton(),
		recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "f12",
			Down: true, App: "studio", T: at(5000)})

	var out bytes.Buffer
	d := Deps{
		Reg: reg, Rec: &fakeRecorder{events: events}, Out: &out,
		// THE OWNER IS SET AND THE BUTTON IS NOT USED, which is the whole point:
		// what gates the strip is how the recording ENDED, not who was driving it.
		Owner: "studio",
	}
	if err := d.LearnAuto("ark graphics"); err != nil {
		t.Fatalf("recording: %v", err)
	}
	// SCOPED TO MARCO'S OWN WINDOW, which is where an unstripped tail leads: the
	// last app the recording switched to is the app it is filed under. Read from
	// there deliberately rather than worked around, because it is the clearest
	// statement of what the strip is for.
	raw, err := os.ReadFile(reg.Path(routes.Route{App: "studio",
		Slug: "ark-graphics"}))
	if err != nil {
		t.Fatalf("loading: %v (said: %s)", err, out.String())
	}
	src := string(raw)
	if !strings.Contains(src, "1511") {
		t.Errorf("a gesture-ended recording was stripped as though a button had "+
			"ended it; the two tails are different and only one of them is "+
			"known to have been the operator:\n%s", src)
	}
}

// AND THE BUTTON-STOPPED RECORDING IS FILED UNDER THE APP IT WAS MADE IN.
//
// The other half of the test above, and the one a person would notice first. Without the strip
// the recording's last app switch is into Marco, so the routine is filed under Marco and is
// offered only while Marco is in front — a routine for a game, scoped to the window you pressed
// Stop in. That is not a cosmetic difference: it decides whether the routine can ever be found.
func TestAButtonStoppedRecordingIsFiledUnderTheAppItWasMadeIn(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	rec := &heldRecorder{events: aDemonstrationEndedByTheButton()}
	press := make(chan struct{})
	var out bytes.Buffer
	d := Deps{Reg: reg, Rec: rec, Out: &out, Owner: "studio", StopWhen: press}
	go func() {
		time.Sleep(10 * time.Millisecond)
		close(press)
	}()
	if err := d.LearnAuto("ark graphics"); err != nil {
		t.Fatalf("recording: %v", err)
	}
	if _, err := os.Stat(reg.Path(routes.Route{App: "studio",
		Slug: "ark-graphics"})); err == nil {
		t.Fatal("the routine was filed under Marco's own window, so it is only " +
			"offered while Marco is in front")
	}
	if _, err := os.Stat(reg.Path(routes.Route{App: "arkascended",
		Slug: "ark-graphics"})); err != nil {
		t.Fatalf("the routine is not filed under the app it was demonstrated "+
			"in: %v (said: %s)", err, out.String())
	}
}
