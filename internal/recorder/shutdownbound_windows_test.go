//go:build windows

package recorder

// A SHUTDOWN THAT CANNOT BE COMPLETED STILL RETURNS, AND SAYS WHAT IT LEFT BEHIND.
//
// # What was seen
//
// `plugins/studio`'s own suite, under the load of its headless-browser tests, intermittently left
// a played routine unfinished: the child had run the program — its last transcript line was the
// routine's own log — and then said nothing for thirty seconds, with no outcome word and the
// window's one run slot still claimed. Roughly one run in six; in isolation it finishes in half a
// second.
//
// The pump wake in `Stop` has been bounded since the 600-second timeouts that found it. The two
// channel receives EITHER SIDE of it were not, and the second of them waits for the app-switch
// poller — whose one blocking call is a foreground-window read, which blocks when the window's
// owning thread is not pumping. A desktop full of browser windows coming and going is exactly
// that desktop.
//
// # What this test establishes, and what it does not
//
// That neither wait can wedge a caller: a recording whose hook thread never reported starting,
// and one whose poller never comes back, both let `Stop` return. It does NOT establish why the
// poller was late — nothing here does, and the message exists so the next occurrence says which
// wait expired rather than needing the paragraph above again.
//
// No hooks are installed. The channels are what `Stop` waits on, so a recorder with those
// channels and nothing else is the whole of what the bounds are about — and installing real
// low-level hooks in a test is not a thing this package may do.
//
// See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].

import (
	"strings"
	"testing"
	"time"
)

// aRecordingThatWillNotFinishShuttingDown is a recorder waiting on channels nobody closes.
//
// `ready` is given or withheld per case; `stopped` and `pollStopped` are never closed, which is
// the state a thread stuck in a syscall leaves them in.
func aRecordingThatWillNotFinishShuttingDown(readyNow bool) *winRecorder {
	r := &winRecorder{
		done:        make(chan struct{}),
		stopped:     make(chan struct{}),
		ready:       make(chan struct{}),
		pollStopped: make(chan struct{}),
	}
	if readyNow {
		close(r.ready)
	}
	return r
}

// A HOOK THREAD THAT NEVER REPORTED STARTING DOES NOT WEDGE THE SHUTDOWN.
func TestAHookThreadThatNeverStartedDoesNotWedgeTheShutdown(t *testing.T) {
	r := aRecordingThatWillNotFinishShuttingDown(false)
	done := make(chan struct{})
	go func() { defer close(done); r.Stop() }()

	// GENEROUSLY MORE THAN THE BOUND, and less than the thirty seconds that was the
	// symptom. A test that waited the bound exactly would fail on a slow machine for a
	// reason that is not the product's.
	select {
	case <-done:
	case <-time.After(hookShutdownBound + 20*time.Second):
		t.Fatal("Stop never returned for a recording whose hook thread never " +
			"reported that it had started, so a completed run reports nothing " +
			"at all and whoever spawned it waits for ever")
	}
	// AND A SECOND STOP IS A NO-OP rather than a second wait. `abandon` clears `done`,
	// which is what makes that true, and a caller that retries is a caller that would
	// otherwise pay the bound twice.
	again := make(chan struct{})
	go func() { defer close(again); r.Stop() }()
	select {
	case <-again:
	case <-time.After(2 * time.Second):
		t.Error("a second Stop waited again")
	}
}

// AND NEITHER DOES A POLLER THAT NEVER COMES BACK.
//
// The one the symptom points at. `ready` is closed and the pump still never answers, so this goes
// through the pump's own bound first and then through the poller's — which is the ordering a real
// shutdown has, and the reason the total is bounded rather than merely each part.
func TestAPollerThatNeverComesBackDoesNotWedgeTheShutdown(t *testing.T) {
	r := aRecordingThatWillNotFinishShuttingDown(true)
	done := make(chan struct{})
	started := time.Now()
	go func() { defer close(done); r.Stop() }()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("Stop never returned for a recording whose app-switch poller never " +
			"came back, which is the thirty-second silence a played routine " +
			"reported as nothing at all")
	}
	// AND IT DID NOT TAKE FOR EVER MINUS ONE. The bound is the point: a shutdown that
	// waits five minutes is a shutdown nobody distinguishes from a hang.
	if took := time.Since(started); took > 40*time.Second {
		t.Errorf("the shutdown took %s, which is not bounded by anything a person "+
			"would wait through", took)
	}
}

// A WORKER HELD ACROSS A TIMED-OUT SHUTDOWN CANNOT REACH A LATER RECORDING.
//
// # What the finding was
//
// `Stop` is bounded, so a shutdown that cannot complete gives up and returns — and the app-switch
// poller and the capture worker are still running. The poller's one blocking call is a
// foreground-window read, which does not return while the window's owning thread is not pumping;
// the capture worker is doing screen work. Either can come back AFTER another recording has
// started, and both publish through this package's global state: an old poller's app switch
// appears in somebody else's demonstration as a step they never performed, and an old capture
// writes a picture into whatever event is at its index now.
//
// # Both halves of the answer
//
// The workers are fenced by SESSION, which is what this exercises: a late publish from an
// abandoned recording is dropped.
//
// And a further recording is REFUSED outright, because the hooks cannot be fenced from Go at all.
// They are C callbacks with no context of their own, they gate on `recActive`, and the next
// `Start` would set that true again — so an abandoned recording's hooks, still installed because
// the thread that owns them never answered, would feed the new one. A session number cannot reach
// a C callback, so the honest consequence of hooks that are still installed is that recording is
// over until the process restarts, which is what the shutdown's own message already said.
//
// See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].
func TestAWorkerHeldAcrossATimeoutCannotReachALaterRecording(t *testing.T) {
	// A CLEAN SLATE, because this is package state and a previous test may have wedged
	// it. Restored afterwards so the order of tests in this package cannot matter.
	wasWedged, wasSession := recWedged.Load(), recSession.Load()
	t.Cleanup(func() {
		recWedged.Store(wasWedged)
		recSession.Store(wasSession)
	})
	recWedged.Store(false)

	// THE ABANDONED RECORDING. Its session is taken the way Start takes one, and its
	// shutdown gives up: nothing ever closes `stopped` or `pollStopped`.
	old := aRecordingThatWillNotFinishShuttingDown(true)
	old.session = recSession.Add(1)
	done := make(chan struct{})
	go func() { defer close(done); old.Stop() }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("the shutdown never returned")
	}

	// A LATER RECORDING IS REFUSED, and says what to do about it.
	err := New().Start()
	if err == nil {
		t.Fatal("a second recording started after a shutdown that could not take its " +
			"hooks off, so the abandoned recording's hooks would feed it")
	}
	for _, says := range []string{"cannot record again", "could not be taken off",
		"open it again"} {
		if !strings.Contains(err.Error(), says) {
			t.Errorf("the refusal does not say %q: %v", says, err)
		}
	}

	// AND THE OLD WORKER'S LATE PUBLISH IS DROPPED EVEN SO.
	//
	// Asserted separately, because the refusal above is what protects a real process and
	// the fence is what makes the workers correct: a build that only refused would be one
	// line from being wrong again the day somebody allowed recording to resume.
	recSession.Add(1) // as a later Start would
	if old.mine() {
		t.Fatal("the abandoned recording still believes the global state is its own")
	}
	recMu.Lock()
	recBuf = []RecordedEvent{{Kind: EvKey, KeyName: "a"}}
	recMu.Unlock()
	// WHAT THE POLLER DOES WHEN ITS READ FINALLY RETURNS, in the one line that matters.
	if old.mine() {
		record(RecordedEvent{Kind: EvAppSwitch, KeyName: "somebody-elses-window"})
	}
	recMu.Lock()
	n, kind := len(recBuf), recBuf[0].Kind
	recMu.Unlock()
	if n != 1 || kind != EvKey {
		t.Errorf("a later recording's buffer holds %d event(s) starting with %v after "+
			"an abandoned poller came back", n, kind)
	}
	// AND A RECORDER OF THE CURRENT SESSION IS NOT FENCED OUT, or the check above is
	// satisfied by one that drops everything.
	current := &winRecorder{session: recSession.Load()}
	if !current.mine() {
		t.Error("the current recording is fenced out of its own state")
	}
}

// AN ABANDONED POLLER'S SAMPLE IS NOT RECORDED, AND A LIVE ONE'S IS.
//
// The decision the poller makes twice, asked directly. The goroutine that asks it cannot be
// started in a test — `Start` installs real low-level hooks — so the decision is its own function
// and this is what holds it. A mutation taking the session fence out of ONE of the two call sites
// is what made that necessary, and it survived a test that only exercised `mine`.
func TestAnAbandonedPollersSampleIsNotRecorded(t *testing.T) {
	was := recSession.Load()
	t.Cleanup(func() { recSession.Store(was) })

	live := &winRecorder{session: recSession.Add(1)}
	// THE FIRST SAMPLE: nothing recorded yet, so any application is a change.
	if !live.appSwitchToRecord("notepad", "") {
		t.Error("the first sample of a live recording is not recorded, so a " +
			"demonstration would not say which application it began in")
	}
	// AND A LATER ONE, only when it differs.
	if !live.appSwitchToRecord("chrome", "notepad") {
		t.Error("a change of application is not recorded")
	}
	if live.appSwitchToRecord("notepad", "notepad") {
		t.Error("the same application is recorded again, so a demonstration fills up " +
			"with switches to where it already was")
	}
	if live.appSwitchToRecord("", "notepad") {
		t.Error("a sample that found no application is recorded as a switch to none")
	}

	// THE ABANDONED ONE. Its session is no longer the current one, which is what a
	// later Start leaves behind — and its read may return at any time after that.
	old := &winRecorder{session: live.session}
	recSession.Add(1)
	for _, c := range []struct{ app, last string }{
		{"somebody-elses-window", ""},
		{"somebody-elses-window", "notepad"},
	} {
		if old.appSwitchToRecord(c.app, c.last) {
			t.Errorf("an abandoned poller's sample of %q is recorded, so an app "+
				"switch nobody performed appears in a later recording", c.app)
		}
	}
}
