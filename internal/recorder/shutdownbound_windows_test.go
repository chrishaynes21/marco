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
