//go:build windows

package recorder

import (
	"strings"
	"testing"
	"time"
	"unsafe"
)

// WHICH WINDOW IS BEING RECORDED, AND WHAT DECIDES IT.
//
// The window is GIVEN now. The first version latched it — the first foreground window that was
// neither the surface the recording was started from nor its process became the target — and
// that is not a choice: whichever window comes forward first is whatever Windows does next, so
// a terminal, a notification toast or an updater became the thing being recorded. Choosing is
// `orchestrator.Pickable` and is tested there; what is tested HERE is that membership is
// decided from one reading of the desktop containing both halves of the identity.
//
// The one desktop read is a seam (`attributionNow`), which is the pattern
// `statedmode_windows_test.go` set in this package: a rule about WHICH window cannot be
// exercised against the one window a test process has.

const (
	chosenWindow  uintptr = 0x3333
	chosenProcess uint32  = 7001
)

// aDesktopWhere installs one reading of the desktop and restores the real one.
func aDesktopWhere(t *testing.T, hwnd uintptr, pid uint32, readable bool) {
	t.Helper()
	was := attributionNow
	t.Cleanup(func() { attributionNow = was })
	attributionNow = func() whose { return whose{hwnd: hwnd, pid: pid, ok: readable} }
}

// aRecordingOfTheChosenWindow is the package state `Start` leaves behind for a confined
// recording of one chosen window.
func aRecordingOfTheChosenWindow(t *testing.T) {
	t.Helper()
	aCleanPackage(t)
	confined.Store(true)
	gone.Store(false)
	targetHWnd.Store(chosenWindow)
	targetPID.Store(chosenProcess)
	targetApp.Store("arkascended")
	recMu.Lock()
	bound = NewBoundary()
	recMu.Unlock()
}

// ── MEMBERSHIP IS THE HANDLE AND THE OWNER, OUT OF ONE READING ───────────────

func TestOnlyTheChosenWindowIsAdmitted(t *testing.T) {
	aRecordingOfTheChosenWindow(t)

	aDesktopWhere(t, chosenWindow, chosenProcess, true)
	if !attributionNow().admits() {
		t.Fatal("the chosen window was not admitted, so nothing would ever be recorded")
	}

	// ANOTHER WINDOW OF THE SAME APPLICATION. Two windows of one program satisfy every
	// comparison of process id, geometry and program name and are two different windows,
	// which is why the handle decides. This is the case a "fall back to the process"
	// shortcut admits.
	aDesktopWhere(t, 0x4444, chosenProcess, true)
	if attributionNow().admits() {
		t.Error("another window of the same application was admitted, so a " +
			"demonstration in one browser window records what happened in the other")
	}

	// AND THE SAME HANDLE UNDER A DIFFERENT OWNER. Windows hands a handle back out when
	// a window closes, so the number alone is not an identity: without the owner, a
	// recording carries on admitting input from whatever got the handle next.
	aDesktopWhere(t, chosenWindow, 9999, true)
	if attributionNow().admits() {
		t.Error("the chosen window's handle was admitted under a different owner, " +
			"which is a different window with the same number")
	}

	// AND ATTRIBUTION THAT COULD NOT BE READ IS UNKNOWN, NOT MEMBERSHIP.
	for _, c := range []struct {
		why      string
		hwnd     uintptr
		pid      uint32
		readable bool
	}{
		{"the foreground could not be read", 0, 0, false},
		{"there is no foreground window", 0, chosenProcess, true},
		{"the owner could not be read", chosenWindow, 0, true},
	} {
		aDesktopWhere(t, c.hwnd, c.pid, c.readable)
		if attributionNow().admits() {
			t.Errorf("input was admitted when %s", c.why)
		}
	}

	// AND NOTHING AT ALL ONCE THE WINDOW HAS GONE, however the handle answers now.
	aDesktopWhere(t, chosenWindow, chosenProcess, true)
	gone.Store(true)
	if attributionNow().admits() {
		t.Error("input was admitted after the chosen window stopped existing")
	}
}

// AND THE ADMISSION AND THE EVENT COME FROM THE SAME READING.
//
// # The defect this is about
//
// A click used to ask `clickAttribution` for the window it landed in, and the gate asked the
// foreground AGAIN for itself. Between two reads the foreground can change — that is what a
// click on another window DOES — so an event could be attributed to one window and admitted on
// the strength of another. A key was worse: it was tagged with the app poller's last sample and
// admitted on a third reading.
//
// So this hands the hook a desktop that answers DIFFERENTLY EVERY TIME IT IS ASKED. One reading
// per edge means the recorded window is the window that was admitted, whatever the second answer
// would have been.
func TestOneReadingDecidesBothTheAdmissionAndTheEvent(t *testing.T) {
	aRecordingOfTheChosenWindow(t)
	wasShape := clickShape
	t.Cleanup(func() { clickShape = wasShape })
	clickShape = func(uintptr) (int, int, int, int, int, bool) {
		return 0, 0, 1920, 1080, 96, true
	}
	recMu.Lock()
	recBuf, recPub = nil, make(chan RecordedEvent, 16)
	recMu.Unlock()

	// A DESKTOP THAT FLIPS ON EVERY CALL: the chosen window first, somewhere else next.
	was := attributionNow
	t.Cleanup(func() { attributionNow = was })
	asked := 0
	attributionNow = func() whose {
		asked++
		if asked%2 == 1 {
			return whose{hwnd: chosenWindow, pid: chosenProcess, ok: true}
		}
		return whose{hwnd: 0x9999, pid: 4242, ok: true}
	}

	recordClickDown("left", 842, 274)
	if asked != 1 {
		t.Errorf("the press read the desktop %d times, so the gate and the event "+
			"can describe two different windows", asked)
	}
	recMu.Lock()
	buf := recBuf
	recMu.Unlock()
	var clicks []RecordedEvent
	for _, ev := range buf {
		if ev.Kind == EvClick {
			clicks = append(clicks, ev)
		}
	}
	if len(clicks) != 1 {
		t.Fatalf("the recording holds %d clicks, not 1: %+v", len(clicks), buf)
	}
	// THE RECORDED WINDOW IS THE ADMITTED WINDOW.
	if clicks[0].HWnd != chosenWindow || clicks[0].PID != chosenProcess {
		t.Errorf("the admitted click was attributed to %#x/%d, not to the window "+
			"that was admitted", clicks[0].HWnd, clicks[0].PID)
	}
	// AND THE NEXT EDGE, WHICH THIS DESKTOP SAYS IS ELSEWHERE, IS NOT RECORDED.
	//
	// The CLICKS are counted and not the events: the release reads as another window, so
	// the boundary moves and leaves an EvLeft — and a crossing, because the button went
	// down in the window and came up somewhere else, which is precisely a half-gesture.
	// Those markers are the rule working. A release in the demonstration would not be.
	recordClickUp("left", 842, 274)
	recMu.Lock()
	after := 0
	for _, ev := range recBuf {
		if ev.Kind == EvClick {
			after++
		}
	}
	recMu.Unlock()
	if after != 1 {
		t.Errorf("the recording holds %d clicks after an edge that read as another "+
			"window, not the 1 that was admitted", after)
	}
}

// AND AN ADMITTED EVENT CARRIES THE CHOSEN WINDOW'S NAME, not a sample of the foreground.
//
// `foreground()` is the app-switch poller's last reading, on a 120ms interval, so a key pressed
// inside that interval after a switch was labelled with the application the person had just
// LEFT. The chosen window's name cannot be wrong about the chosen window, and it is read once,
// at Start, off the hook thread.
func TestAnAdmittedEventIsTaggedWithTheChosenWindowsApplication(t *testing.T) {
	aRecordingOfTheChosenWindow(t)
	foreApp.Store("chrome") // the poller, lagging and wrong
	if got := appOfTarget(); got != "arkascended" {
		t.Errorf("an admitted event would be tagged %q, which is the application "+
			"the person had left", got)
	}
	// AND AN UNCONFINED RECORDING STILL READS THE SAMPLE, which is what every recording
	// did before this and the only answer available when no window was chosen.
	confined.Store(false)
	if got := appOfTarget(); got != "chrome" {
		t.Errorf("a recording of the whole desktop tagged an event %q instead of "+
			"the foreground sample", got)
	}
}

// ── WHAT WAS ALREADY HELD WHEN THE RECORDING BEGAN ───────────────────────────

// CTRL HELD BEFORE RECORD IS PRESSED MAKES A TARGET CLICK UNREPRESENTABLE.
//
// The press is one this recording will never see, so `Boundary` started empty and believed
// nothing was held: a ctrl+click performed in the chosen window was admitted and written down
// as an ordinary `click`, which is a different demonstration. It is the same defect as entering
// the window with Ctrl down, one step earlier, and the boundary could not see it because the
// press happened before there was a boundary.
func TestSomethingHeldBeforeTheRecordingBeganRefusesTheRecording(t *testing.T) {
	aCleanPackage(t)
	was := alreadyHeld
	t.Cleanup(func() { alreadyHeld = was })
	alreadyHeld = func() []string { return []string{"ctrl"} }

	r := &winRecorder{}
	r.Only(OneWindow, Target{HWnd: chosenWindow, PID: chosenProcess})
	r.beginRecording()
	recMu.Lock()
	b := bound
	recMu.Unlock()
	if b == nil {
		t.Fatal("a confined recording raised no boundary")
	}
	// A CLICK IN THE CHOSEN WINDOW, WITH CTRL STILL DOWN FROM BEFORE.
	if got := b.Pressed("the left button", true, true, false); got != Admit {
		t.Fatalf("a click in the chosen window was %v", got)
	}
	why := b.Crossing()
	if why == "" {
		t.Fatal("a ctrl+click whose ctrl was held before recording started saved as " +
			"an ordinary click, which is a different demonstration")
	}
	if !strings.Contains(why, "ctrl") {
		t.Errorf("the reason does not name what was held: %q", why)
	}
}

// AND ITS RELEASE IS NOT RECORDED AS A KEYSTROKE NOBODY MADE.
func TestTheReleaseOfSomethingHeldBeforeTheRecordingIsDropped(t *testing.T) {
	b := NewBoundary()
	b.HeldBefore([]string{"ctrl", "q"})
	if _, ok := b.Arrived(true, time.Now()); !ok {
		t.Fatal("arriving in the window was not reported")
	}
	for _, id := range []string{"ctrl", "q"} {
		if got := b.Pressed(id, false, true, false); got != Drop {
			t.Errorf("the release of %q, held since before the recording, was %v",
				id, got)
		}
	}
	// AND ONCE THEY ARE ALL RELEASED THE RECORDING IS LIVE AGAIN: the seeding is a
	// starting state, not a permanent refusal.
	if got := b.Pressed("a", true, true, false); got != Admit {
		t.Errorf("a key after the held ones were released was %v", got)
	}
}

// AND AN EMPTY KEYBOARD SEEDS NOTHING, which is the control: a seeding that always refused
// would pass every case above and no recording would ever be saveable.
func TestNothingHeldAtTheStartRefusesNothing(t *testing.T) {
	aCleanPackage(t)
	was := alreadyHeld
	t.Cleanup(func() { alreadyHeld = was })
	alreadyHeld = func() []string { return nil }

	r := &winRecorder{}
	r.Only(OneWindow, Target{HWnd: chosenWindow, PID: chosenProcess})
	r.beginRecording()
	recMu.Lock()
	b := bound
	recMu.Unlock()
	b.Arrived(true, time.Now())
	b.Pressed("the left button", true, true, false)
	b.Pressed("the left button", false, true, false)
	if why := b.Crossing(); why != "" {
		t.Errorf("an ordinary click on an idle keyboard refused the recording: %q", why)
	}
}

// ── THE RECORDING'S OWN SHAPE ────────────────────────────────────────────────

// A CONFINED RECORDING OPENS OUTSIDE THE WINDOW, said in the buffer.
//
// It is what makes the walk to the chosen window an absence like any other, so
// `orchestrator.inWindowClock` takes it out of the clock by the rule it already has instead of
// a second rule about the beginning.
func TestAConfinedRecordingBeginsWithAnAbsence(t *testing.T) {
	aCleanPackage(t)
	r := &winRecorder{}
	r.Only(OneWindow, Target{HWnd: chosenWindow, PID: chosenProcess})
	r.beginRecording()
	recMu.Lock()
	buf, b := recBuf, bound
	recMu.Unlock()
	if len(buf) != 1 || buf[0].Kind != EvLeft {
		t.Fatalf("a confined recording did not open outside the window: %+v", buf)
	}
	if b == nil {
		t.Error("the crossing state was not raised for a confined recording")
	}
	// AND A RECORDING OF THE WHOLE DESKTOP OPENS WITH NOTHING, which is what it always
	// did: there is no window to be outside of.
	aCleanPackage(t)
	d := &winRecorder{}
	d.Only(WholeDesktop, Target{})
	d.beginRecording()
	recMu.Lock()
	buf, b = recBuf, bound
	recMu.Unlock()
	if len(buf) != 0 {
		t.Errorf("a recording of the whole desktop opened with %+v", buf)
	}
	if b != nil {
		t.Error("a recording of the whole desktop has crossing state, so a gesture " +
			"between two applications would be refused")
	}
}

// AN APPLICATION SWITCH THAT HAPPENED WHILE THE PERSON WAS AWAY IS NOT THIS RECORDING'S.
//
// The poller writes STRAIGHT TO THE BUFFER, so it is a second door into the recording and the
// gate has to be on it too. `simplify` turns an EvAppSwitch into an Activate, so without this a
// routine demonstrated in a game would begin by raising the browser somebody checked.
func TestAnApplicationSwitchFromElsewhereIsNotRecorded(t *testing.T) {
	aRecordingOfTheChosenWindow(t)
	r := &winRecorder{watch: OneWindow}
	r.beginRecording()
	confined.Store(true)
	targetHWnd.Store(chosenWindow)
	targetPID.Store(chosenProcess)
	// THE WHOLE IDENTITY, because `beginRecording` resolved the name of this
	// recorder's own zero target and the poller records the CHOSEN window's
	// application rather than its sample. See
	// TestAConfinedApplicationSwitchNamesTheChosenWindow.
	targetApp.Store("arkascended")
	recMu.Lock()
	recBuf = nil // the opening absence is not what this is about
	bound = NewBoundary()
	recMu.Unlock()

	aDesktopWhere(t, 0x9999, 4242, true) // away
	if got := r.sampled("chrome", "", time.Now()); got != "" {
		t.Errorf("a switch to %q while away was recorded as this demonstration's", got)
	}
	recMu.Lock()
	switches := 0
	for _, ev := range recBuf {
		if ev.Kind == EvAppSwitch {
			switches++
		}
	}
	recMu.Unlock()
	if switches != 0 {
		t.Errorf("%d app switches from elsewhere are in the recording", switches)
	}
	// AND THE SWITCH TO THE CHOSEN WINDOW IS RECORDED, which is the control — and the
	// reason `last` is left alone above: the switch back must not look like a repeat.
	aDesktopWhere(t, chosenWindow, chosenProcess, true)
	if got := r.sampled("arkascended", "", time.Now()); got != "arkascended" {
		t.Fatalf("the switch to the chosen window was %q", got)
	}
	recMu.Lock()
	found := false
	for _, ev := range recBuf {
		if ev.Kind == EvAppSwitch && ev.KeyName == "arkascended" {
			found = true
		}
	}
	recMu.Unlock()
	if !found {
		t.Error("the switch to the chosen window was not recorded")
	}
}

// AND A CONFINED RECORDING'S APPLICATION SWITCH NAMES THE CHOSEN WINDOW, NOT THE SAMPLE.
//
// # The other half of the same race
//
// The case above is the one where the person is still away when the admission snapshot is
// taken. This is the one where they are not: the sample came back naming Chrome, they clicked
// back to the game, and by the time `attributionNow` reads the desktop the foreground IS the
// game. So the reading admits — correctly, they ARE in the window — and the string written
// beside it is still `chrome`. `simplify` turns that into an Activate for the browser and
// `Keep` files the routine under it, off one sample taken 120ms before the gate agreed to it.
func TestAConfinedApplicationSwitchNamesTheChosenWindow(t *testing.T) {
	aRecordingOfTheChosenWindow(t)
	r := &winRecorder{watch: OneWindow}
	r.session = recSession.Load()
	recMu.Lock()
	recBuf, recPub = nil, make(chan RecordedEvent, 16)
	recMu.Unlock()

	// THE SAMPLE IS STALE AND THE SNAPSHOT IS THE TARGET, which is the race.
	aDesktopWhere(t, chosenWindow, chosenProcess, true)
	if got := r.sampled("chrome", "", time.Now()); got != "arkascended" {
		t.Errorf("the poller remembers %q as the application it recorded", got)
	}
	if got := appSwitchesIn(t); len(got) != 1 || got[0] != "arkascended" {
		t.Fatalf("the confined recording holds the app switches %q, so a foreign "+
			"Activate would be raised on every run", got)
	}
	// AND A SECOND SAMPLE ADDS NOTHING, because there is only one window to name: the
	// poller's `last` already holds it.
	if got := r.sampled("chrome", "arkascended", time.Now()); got != "arkascended" {
		t.Errorf("the poller remembers %q after a second stale sample", got)
	}
	if got := appSwitchesIn(t); len(got) != 1 {
		t.Errorf("a second sample added an app switch: %q", got)
	}

	// AND A CHOSEN WINDOW WHOSE IMAGE COULD NOT BE READ RECORDS NO SWITCH AT ALL,
	// rather than falling back to the sample — which is the same fallback `appOfTarget`
	// makes for a TAG and must not make for a STEP. Unknown is not an application.
	targetApp.Store("")
	recMu.Lock()
	recBuf = nil
	recMu.Unlock()
	if got := r.sampled("chrome", "", time.Now()); got != "" {
		t.Errorf("a target with no readable image recorded a switch to %q", got)
	}
	if got := appSwitchesIn(t); len(got) != 0 {
		t.Errorf("a target with no readable image recorded %q", got)
	}

	// AND AN UNCONFINED RECORDING STILL RECORDS THE SAMPLE, which is what every
	// recording there was did, and the only answer available with no window chosen.
	confined.Store(false)
	if got := r.sampled("chrome", "", time.Now()); got != "chrome" {
		t.Errorf("a recording of the whole desktop recorded %q, not its sample", got)
	}
	if got := appSwitchesIn(t); len(got) != 1 || got[0] != "chrome" {
		t.Errorf("a recording of the whole desktop holds %q", got)
	}
}

// appSwitchesIn is the applications the recording in flight has switched to, in order.
func appSwitchesIn(t *testing.T) []string {
	t.Helper()
	recMu.Lock()
	defer recMu.Unlock()
	var out []string
	for _, ev := range recBuf {
		if ev.Kind == EvAppSwitch {
			out = append(out, ev.KeyName)
		}
	}
	return out
}

// AND LEAVING AND COMING BACK WITHOUT TOUCHING ANYTHING MOVES THE BOUNDARY.
//
// # The defect this is about
//
// Markers were made by `observe`, which only runs when the hooks see something. So somebody who
// left the window, read a page and came back WITHOUT PRESSING ANYTHING produced no EvLeft and no
// EvEntered at all — and with no markers around the absence, `inWindowClock` had nothing to take
// out of the clock, so the routine waited the whole interval on every run. Exactly the defect
// the clock exists to prevent, arriving through the one path that generates no input.
func TestLeavingAndReturningWithNoInputStillMovesTheClock(t *testing.T) {
	aRecordingOfTheChosenWindow(t)
	r := &winRecorder{watch: OneWindow}
	r.session = recSession.Load()
	recMu.Lock()
	recBuf, recPub = nil, make(chan RecordedEvent, 16)
	recMu.Unlock()

	base := time.Now()
	// IN THE WINDOW, then away for thirty seconds, then back — and not one hook event.
	aDesktopWhere(t, chosenWindow, chosenProcess, true)
	r.sampled("arkascended", "", base)
	aDesktopWhere(t, 0x9999, 4242, true)
	r.sampled("chrome", "arkascended", base.Add(time.Second))
	aDesktopWhere(t, chosenWindow, chosenProcess, true)
	r.sampled("arkascended", "arkascended", base.Add(31*time.Second))

	recMu.Lock()
	buf := recBuf
	recMu.Unlock()
	var left, entered int
	for _, ev := range buf {
		switch ev.Kind {
		case EvLeft:
			left++
		case EvEntered:
			entered++
		}
	}
	if left != 1 || entered != 2 {
		t.Fatalf("an absence with no input in it produced %d EvLeft and %d "+
			"EvEntered, so the clock has nothing to take out: %+v", left, entered, buf)
	}
	// AND THE PAIR BRACKETS THE ABSENCE, which is what the clock reads.
	recMu.Lock()
	b := bound
	recMu.Unlock()
	away, absences := b.Away()
	if absences != 1 || away < 29*time.Second {
		t.Errorf("the absence was measured as %v over %d, not ~30s over 1",
			away, absences)
	}
}

// AND A SURFACE CAN ASK WHETHER THE CAPTURE IS PAUSED RIGHT NOW.
//
// Studio does not read the event channel, so it cannot see the markers; it asks on the poll that
// already draws the recording card. Before the person has ever arrived the answer is NOT paused:
// they have chosen a window and not gone to it yet, which is not a pause in a recording.
func TestPausedIsAnsweredFromTheBoundary(t *testing.T) {
	aRecordingOfTheChosenWindow(t)
	r := &winRecorder{watch: OneWindow}
	r.session = recSession.Load()

	if paused, _ := r.Paused(); paused {
		t.Error("a recording whose person has not arrived yet says it is paused")
	}
	recMu.Lock()
	bound.Arrived(true, time.Now())
	recMu.Unlock()
	if paused, _ := r.Paused(); paused {
		t.Error("a recording in its window says it is paused")
	}
	recMu.Lock()
	bound.Arrived(false, time.Now().Add(-3*time.Second))
	recMu.Unlock()
	paused, since := r.Paused()
	if !paused {
		t.Fatal("a recording whose person has left does not say it is paused")
	}
	if since < 2*time.Second {
		t.Errorf("it has been paused for %v, which is not how long ago they left",
			since)
	}
}

// AND AN OFF-TARGET PRESS ASKS FOR NO PICTURE OF WHAT IT LANDED ON.
//
// The clearest case of the whole rule: a screen capture of a window nobody chose is off-target
// PIXELS. `recordClickDown` reads the index `observe` gives it, and -1 means there is nothing to
// attach a picture to.
func TestAnOffTargetPressQueuesNoScreenCapture(t *testing.T) {
	aRecordingOfTheChosenWindow(t)
	wasAnchors, wasShape := captureAnchors, clickShape
	t.Cleanup(func() {
		captureAnchors, clickShape = wasAnchors, wasShape
		captureReqs = nil
	})
	captureAnchors = true
	plainCoordinates.Store(false)
	clickShape = func(uintptr) (int, int, int, int, int, bool) {
		return 0, 0, 1920, 1080, 96, true
	}
	captureReqs = make(chan captureReq, 4)

	aDesktopWhere(t, 0x9999, 4242, true) // a press somewhere else
	recordClickDown("left", 842, 274)
	select {
	case req := <-captureReqs:
		t.Fatalf("a picture of an off-target press was asked for, at index %d", req.idx)
	default:
	}
	// AND A PRESS IN THE CHOSEN WINDOW STILL ASKS FOR ONE, without which this would pass
	// against a recorder that had stopped capturing anything.
	aDesktopWhere(t, chosenWindow, chosenProcess, true)
	recordClickDown("left", 842, 274)
	select {
	case <-captureReqs:
	default:
		t.Error("a press in the chosen window asked for no picture, so the anchor " +
			"capture is gone for everybody")
	}
}

// AND THE KEY HOOK ASKS, which is the one call site a test of `observe` cannot reach: the clicks
// go through `recordClickDown` and the keys go through `keyboardProc`, a C callback, so this
// builds the structure Windows passes and calls it.
func TestTheKeyHookDoesNotRecordKeysFromAnotherWindow(t *testing.T) {
	aRecordingOfTheChosenWindow(t)
	wasActive := recActive.Load()
	t.Cleanup(func() { recActive.Store(wasActive) })
	recActive.Store(true)
	recMu.Lock()
	recBuf, recPub = nil, make(chan RecordedEvent, 16)
	recMu.Unlock()

	press := func() {
		kb := kbdllHookStruct{vkCode: 0x41} // 'A'
		keyboardProc(0, wmKeyDown, uintptr(unsafe.Pointer(&kb)))
	}
	aDesktopWhere(t, 0x9999, 4242, true)
	press()
	recMu.Lock()
	n := len(recBuf)
	recMu.Unlock()
	if n != 0 {
		t.Errorf("the hook recorded %d events for a key pressed outside the chosen "+
			"window, so somebody's typing is in the demonstration", n)
	}
	// AND IN THE CHOSEN WINDOW, without which a hook that recorded nothing would pass.
	//
	// The poller's sample is set to the WRONG application first, because the other half
	// of this is what the hook tags the event with: `foreground()` is a 120ms sample, so
	// a key pressed inside that interval after a switch was labelled with the
	// application the person had just left. See appOfTarget.
	foreApp.Store("chrome")
	aDesktopWhere(t, chosenWindow, chosenProcess, true)
	press()
	recMu.Lock()
	var keys []RecordedEvent
	for _, ev := range recBuf {
		if ev.Kind == EvKey {
			keys = append(keys, ev)
		}
	}
	recMu.Unlock()
	if len(keys) != 1 {
		t.Fatalf("the hook recorded %d keys in the chosen window, not 1", len(keys))
	}
	if keys[0].App != "arkascended" {
		t.Errorf("the hook tagged an admitted key with %q, which is the application "+
			"the person had left rather than the window being recorded",
			keys[0].App)
	}
}

// AND THE CHOSEN WINDOW'S NAME IS RESOLVED WHEN THE RECORDING IS RAISED.
//
// Once, off the hook thread, because reading it opens a process handle — the work that makes
// Windows drop the hooks. Without it `appOfTarget` falls back to the poller's sample and every
// admitted event is tagged with whatever the foreground happened to be, which is the defect one
// layer down from the one above.
func TestTheChosenWindowsNameIsResolvedOnceWhenTheRecordingBegins(t *testing.T) {
	aCleanPackage(t)
	was := imageOfProcess
	t.Cleanup(func() { imageOfProcess = was })
	asked := 0
	imageOfProcess = func(pid uint32) string {
		asked++
		if pid != chosenProcess {
			t.Errorf("the name was asked of process %d, not the chosen window's "+
				"owner", pid)
		}
		return "arkascended"
	}

	r := &winRecorder{}
	r.Only(OneWindow, Target{HWnd: chosenWindow, PID: chosenProcess})
	r.beginRecording()
	confined.Store(true)
	if asked != 1 {
		t.Fatalf("the chosen window's name was asked for %d times, not once", asked)
	}
	foreApp.Store("chrome") // the poller, lagging and wrong
	if got := appOfTarget(); got != "arkascended" {
		t.Errorf("an admitted event would be tagged %q", got)
	}
}
