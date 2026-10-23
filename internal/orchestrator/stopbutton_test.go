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

// A FIXTURE THAT CANNOT HAPPEN WAS DELETED HERE, and it is worth saying why.
//
// It claimed to cover "a trip too quick for the poller to see": no app switch recorded, and the
// stopping click tagged `App: "studio"`. That state does not exist. The app tag IS the poller's
// last sample, and the switch event is emitted from the same sample — so a click tagged "studio"
// means the poller HAD sampled Marco, which means the switch is there. Absent switch and
// "studio" tag cannot both be true.
//
// So the test passed against a world that cannot occur, while the race it named went uncovered:
// the poller still reading the game, no switch, and the stopping click tagged with the game.
// Both of the name rules read that one stale sample, so neither could ever have caught it.
// `TestAStopInsideTheSampleIntervalIsStillCut` below is that case, and it is answered by the
// process that owned the foreground window at the instant of the click instead.

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

// ── THE RACE BOTH NAME RULES MISS ───────────────────────────────────────────────

// gamePID and studioPID stand for two real processes. The numbers mean nothing; that they
// DIFFER is the whole content.
const (
	gamePID   uint32 = 4812
	studioPID uint32 = 9140
)

// aStopPressedInsideTheSampleInterval is the case the app-name rules cannot see.
//
// The poller sampled the game at the start and never samples again: no switch into Marco is
// recorded, and the click that presses Stop is tagged with the app the person LEFT, because the
// tag is the poller's last sample. Both name rules therefore read "arkascended" and keep it.
//
// Only the process that owned the foreground window AT THE INSTANT of the click tells the truth
// here, and that is read in the hook rather than sampled.
func aStopPressedInsideTheSampleInterval() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(105)},
		// ── MARCO COMES FORWARD AND IS CLICKED, INSIDE ONE SAMPLE INTERVAL ──
		//
		// No EvAppSwitch: the poller has not run. App still says the game: the tag
		// IS the poller's last sample. Only PID knows where the click landed.
		{Kind: recorder.EvMove, X: 1500, Y: 900, T: at(3100)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: "arkascended", PID: studioPID, T: at(3200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: "arkascended", PID: studioPID, T: at(3205)},
	}
}

// THE CLICK THAT STOPPED THE RECORDING IS CUT EVEN WHEN THE POLLER NEVER SAW MARCO.
//
// The failure this replaces was not a missing rule but two rules reading one source: the
// "too fast for the poller" fallback used the poller's own last sample. A fixture that tagged
// the stopping click "studio" asserted the rule worked in the one state the race cannot
// produce.
func TestAStopInsideTheSampleIntervalIsStillCut(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	rec := &heldRecorder{events: aStopPressedInsideTheSampleInterval()}
	press := make(chan struct{})

	var out bytes.Buffer
	d := Deps{
		Reg: reg, Rec: rec, Out: &out,
		Owner: "studio", OwnerPID: studioPID, StopWhen: press,
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		close(press)
	}()
	if err := d.LearnAuto("ark graphics"); err != nil {
		t.Fatalf("recording: %v", err)
	}
	// FILED UNDER THE GAME, which is also the assertion that the tail is gone: the
	// scope comes from the recording's last app switch.
	raw, err := os.ReadFile(reg.Path(routes.Route{App: "arkascended",
		Slug: "ark-graphics"}))
	if err != nil {
		t.Fatalf("the routine is not filed under the app it was demonstrated "+
			"in: %v (said: %s)", err, out.String())
	}
	src := string(raw)

	if !strings.Contains(src, "842") || !strings.Contains(src, "274") {
		t.Fatalf("the demonstrated click is missing:\n%s", src)
	}
	if strings.Contains(src, "1511") || strings.Contains(src, "913") {
		t.Errorf("the click that pressed Stop is in the routine. The poller never "+
			"sampled Marco, so the switch is absent and the click's app tag "+
			"names the game — every rule that reads the poller keeps it:\n%s",
			src)
	}
}

// AND THE NAME RULES ARE NOT WHAT SAVES IT.
//
// The same stream with the owner NAME withheld. If this passes on the process boundary alone,
// then that boundary is doing the work rather than riding on a name rule that happened to match.
func TestTheProcessBoundaryHoldsWithNoOwnerName(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	rec := &heldRecorder{events: aStopPressedInsideTheSampleInterval()}
	press := make(chan struct{})

	var out bytes.Buffer
	d := Deps{
		Reg: reg, Rec: rec, Out: &out,
		OwnerPID: studioPID, StopWhen: press, // no Owner name at all
	}
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
		t.Fatalf("loading: %v (said: %s)", err, out.String())
	}
	if src := string(raw); strings.Contains(src, "1511") {
		t.Errorf("with no owner name, the stopping click survived — so the "+
			"process boundary is not carrying this on its own:\n%s", src)
	}
}

// AND A MOVE BETWEEN TWO DEMONSTRATED CLICKS IS NOT EATEN.
//
// The process rule looks past a trailing move, because a move carries no attribution of its own.
// It must look past it only to something the owner owns: a move that sits between two of the
// demonstration's own events is part of the demonstration's tail, not the operator's.
func TestAMoveBetweenDemonstratedClicksSurvivesTheProcessRule(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(105)},
		{Kind: recorder.EvMove, X: 900, Y: 300, T: at(200)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 496, Y: 283,
			App: "arkascended", PID: gamePID, T: at(300)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 496, Y: 283,
			App: "arkascended", PID: gamePID, T: at(305)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: "arkascended", PID: studioPID, T: at(3200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: "arkascended", PID: studioPID, T: at(3205)},
	}
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	press := make(chan struct{})
	var out bytes.Buffer
	d := Deps{Reg: reg, Rec: &heldRecorder{events: events}, Out: &out,
		Owner: "studio", OwnerPID: studioPID, StopWhen: press}
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
		t.Fatalf("loading: %v (said: %s)", err, out.String())
	}
	src := string(raw)
	for _, want := range []string{"842", "496"} {
		if !strings.Contains(src, want) {
			t.Errorf("the demonstration is missing %s — the process rule walked "+
				"past a move into events it does not own:\n%s", want, src)
		}
	}
	if strings.Contains(src, "1511") {
		t.Errorf("the stopping click survived:\n%s", src)
	}
}

// AND A MOVE BETWEEN TWO OF THE OPERATOR'S OWN CLICKS IS CUT WITH THEM.
//
// The ordinary way a person presses a button in a window that is not in front: click the window
// to focus it, move to the button, click the button. That is two clicks in Marco with a move
// between them, and the move carries no attribution of its own — so the rule has to look past it
// to the event behind it, and keep going when that event is the operator's too.
//
// Without the lookahead the walk stops at the move, and the first of the two clicks stays in the
// routine: a replay would click into Marco's window at whatever is there now.
func TestAMoveBetweenTwoOperatorClicksIsCutWithThem(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(105)},
		// ── CLICK MARCO TO FOCUS IT, inside the sample interval ──
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1200, Y: 400,
			App: "arkascended", PID: studioPID, T: at(3000)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1200, Y: 400,
			App: "arkascended", PID: studioPID, T: at(3005)},
		// ── MOVE TO THE BUTTON ──
		{Kind: recorder.EvMove, X: 1450, Y: 880, T: at(3100)},
		// ── AND PRESS IT ──
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: "arkascended", PID: studioPID, T: at(3200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: "arkascended", PID: studioPID, T: at(3205)},
	}
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	press := make(chan struct{})
	var out bytes.Buffer
	d := Deps{Reg: reg, Rec: &heldRecorder{events: events}, Out: &out,
		Owner: "studio", OwnerPID: studioPID, StopWhen: press}
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
		t.Fatalf("loading: %v (said: %s)", err, out.String())
	}
	src := string(raw)
	if !strings.Contains(src, "842") {
		t.Fatalf("the demonstrated click is missing:\n%s", src)
	}
	// BOTH of the operator's clicks, named separately: the one that pressed the
	// button, and the one that focused the window on the way to it.
	for _, gone := range []string{"1511", "1200"} {
		if strings.Contains(src, gone) {
			t.Errorf("the routine contains %s, one of the clicks that ended the "+
				"recording:\n%s", gone, src)
		}
	}
}

// AND AN UNATTRIBUTED RELEASE DOES NOT HIDE THE PRESS BEHIND IT.
//
// Defence in depth for a hole that has been opened twice. The builder now stamps both edges of a
// click and `recorder` holds that with its own test against the production builder — but this
// strip is what consumes the stream, and it should not be the only thing standing between a
// stamping mistake and a routine that presses Stop on replay.
//
// So the tail here is the one the OLD builder produced: the press of the Stop button carries its
// owner and the release carries nothing. Walking back from the end meets the release first, and
// a strip that could not place it would stop there and keep both edges.
func TestAnUnattributedReleaseDoesNotHideThePressBehindIt(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(105)},
		// ── THE STOP PRESS, STAMPED ON THE PRESS ONLY ──
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: "arkascended", PID: studioPID, T: at(3200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: "arkascended", T: at(3205)}, // no owner at all
	}
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	press := make(chan struct{})
	var out bytes.Buffer
	d := Deps{Reg: reg, Rec: &heldRecorder{events: events}, Out: &out,
		Owner: "studio", OwnerPID: studioPID, StopWhen: press}
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
		t.Fatalf("the routine is not filed under the app it was demonstrated "+
			"in: %v (said: %s)", err, out.String())
	}
	src := string(raw)
	if !strings.Contains(src, "842") {
		t.Fatalf("the demonstrated click is missing:\n%s", src)
	}
	if strings.Contains(src, "1511") {
		t.Errorf("an unattributed release stopped the walk, so the Stop press "+
			"stayed in the routine:\n%s", src)
	}
}

// A STALE NAME MUST NOT DELETE A CLICK THE EVENT ITSELF PLACES IN THE GAME.
//
// The sequence, and every step of it is ordinary:
//
//  1. glance at Marco, so the poller samples it;
//  2. go back to the game and click BEFORE the next sample — the click carries the game's
//     process and Marco's stale name;
//  3. return to Marco and press Stop.
//
// The process rule gets this right: it removes the stopping click and keeps the game click. The
// name rule then removed that game click anyway, because its tag said Marco. Two rules, each
// correct on its own, and the second undoing the first.
//
// Known event-time ownership outranks a sampled name in BOTH directions — not only to condemn
// an event the sample excuses, but to spare one the sample accuses.
func TestAStaleNameDoesNotDeleteAClickTheEventPlacesInTheGame(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(105)},
		// A KEY, to shield the click behind it from
		// `simplify.foldActivates` — which drops the one step immediately
		// before an app switch as the gesture that performed it. That rule
		// predates this work; the shield keeps this test about precedence
		// rather than about that rule.
		{Kind: recorder.EvKey, KeyName: "esc", Down: true,
			App: "arkascended", T: at(800)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: false,
			App: "arkascended", T: at(805)},
		// ── 1. A GLANCE AT MARCO. The poller sees it. ──
		{Kind: recorder.EvAppSwitch, KeyName: "studio", T: at(1000)},
		// ── 2. BACK TO THE GAME, AND A CLICK BEFORE THE NEXT SAMPLE ──
		//
		// The process says the game, because that is read at the instant of the
		// click. The name still says Marco, because that is the last sample.
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 496, Y: 283,
			App: "studio", PID: gamePID, T: at(1500)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 496, Y: 283,
			App: "studio", PID: gamePID, T: at(1505)},
		// ── 3. BACK TO MARCO AND STOP ──
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: "studio", PID: studioPID, T: at(3200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: "studio", PID: studioPID, T: at(3205)},
	}
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	press := make(chan struct{})
	var out bytes.Buffer
	d := Deps{Reg: reg, Rec: &heldRecorder{events: events}, Out: &out,
		Owner: "studio", OwnerPID: studioPID, StopWhen: press}
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
		t.Fatalf("the routine is not filed under the app it was demonstrated "+
			"in: %v (said: %s)", err, out.String())
	}
	src := string(raw)

	// THE CLICK THE SAMPLE ACCUSED, which the event itself places in the game.
	if !strings.Contains(src, "496") || !strings.Contains(src, "283") {
		t.Errorf("the click at 496,283 is gone. Its process said the game and "+
			"its stale tag said Marco, and the tag won:\n%s", src)
	}
	if !strings.Contains(src, "842") {
		t.Errorf("the first demonstrated click is gone too:\n%s", src)
	}
	if strings.Contains(src, "1511") {
		t.Errorf("the click that pressed Stop survived:\n%s", src)
	}
}

// AND AN UNATTRIBUTED EVENT IS NOT THE OWNER'S FOR MERELY FOLLOWING ONE OF THEIRS.
//
// The release fallback is matched to its OWN press: same button, same point, immediately behind
// it. A looser rule — anything unowned that follows an owner event — would delete whatever
// happened to be sitting there, which it has been shown nothing about.
//
// Each stream below ends with the owner's PRESS and then one unattributed event, because that is
// the only arrangement where the question arises: a release behind a release is two clicks, and
// the matching clause is never reached.
func TestAnUnownedEventAfterAnOwnerClickIsNotAssumedToBeTheirs(t *testing.T) {
	// THE DEMONSTRATION, then a click of Marco's whose release is missing.
	base := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(105)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1200, Y: 400,
			App: "arkascended", PID: studioPID, T: at(3000)},
	}
	click := func(button string, down bool, x, y int) recorder.RecordedEvent {
		return recorder.RecordedEvent{Kind: recorder.EvClick, Button: button,
			Down: down, X: x, Y: y, App: "arkascended", T: at(3005)}
	}

	for _, c := range []struct {
		what string
		ev   recorder.RecordedEvent
		// kept is how many of the five events survive. 3 means the
		// unattributed event and the owner's press behind it both went; 5
		// means the walk stopped and took nothing it could not place.
		kept int
	}{
		{"its own press, released", click("left", false, 1200, 400), 3},
		{"a release at another point", click("left", false, 500, 500), 5},
		{"a release of another button", click("right", false, 1200, 400), 5},
		{"a second press, not a release", click("left", true, 1200, 400), 5},
	} {
		t.Run(c.what, func(t *testing.T) {
			stream := append(append([]recorder.RecordedEvent(nil), base...), c.ev)
			got := stripTrailingOwner(stream, "studio", studioPID)
			if len(got) != c.kept {
				t.Fatalf("%d of %d event(s) kept, want %d", len(got),
					len(stream), c.kept)
			}
		})
	}
}

// AND NOTHING REMOVED IS AN ANSWER, NOT A SHORTFALL.
//
// Three of the four cases above remove nothing at all, which is the right way round: the strip
// deletes what it can show belongs to the operator, and an event it cannot place is not shown to
// belong to anybody. Guessing the other way deletes demonstrations.
//
// The cost is small because it is nearly unreachable — the builder stamps both edges of every
// click, so an unowned click means the foreground window could not be read at all.
func TestTheStripKeepsWhatItCannotPlace(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", T: at(105)}, // owner unknown
	}
	if got := stripTrailingOwner(events, "studio", studioPID); len(got) != 2 {
		t.Errorf("%d of 2 event(s) kept: a demonstration whose release could "+
			"not be attributed was deleted", len(got))
	}
}
