package recorder

import (
	"testing"
	"time"
)

// A RECORDING THAT IS STOPPED THE INSTANT IT STARTED STILL STOPS.
//
// # What used to happen instead
//
// `Stop` wakes the hook pump by posting WM_QUIT to the thread that installed the hooks, because
// only that thread may remove them and it is asleep in `GetMessage` until a message arrives. But a
// Windows thread has no message queue until it first calls a message function, and the pump
// publishes its thread id BEFORE installing anything, deliberately, so that a Stop racing a Start
// still has somewhere to aim. In the window between those two moments the post is refused — and a
// single unchecked post is simply lost. `GetMessage` then waits for a message that will never
// come, and `Stop`, waiting for the pump to finish, waits with it. Forever.
//
// `withPanicStop` starts a recording to watch for the stop key and stops it when the play ends, so
// EVERY real-input run goes through this, and a short play is exactly the shape that posts into
// the window. `cmd/marco` hit its 600-second test limit on it twice.
//
// # What this test can and cannot say
//
// The window is microseconds wide and scheduling-dependent, so a green run here is not a proof
// that it has gone: this is the only BEHAVIOURAL shape that can see the defect at all, and the
// structural claim — that the post sits in a retry loop at every site that wakes a pump — is held
// by `internal/platform/navsource` `TestAPumpWakeupIsRetriedRatherThanPostedOnce`, which does not
// depend on catching a race.
//
// What it does say is sharper than "it did not hang". `Stop` is bounded now, so a lost wakeup no
// longer blocks forever — it falls through to its own deadline after five seconds, reports that
// the hooks are still installed, and returns. That is a different failure from the hang and it
// must not pass as a success, so the DURATION is the assertion: a pump that was woken answers in
// milliseconds, and anything near the bound means the message never reached it.
func TestAStopRightAfterAStartDoesNotHang(t *testing.T) {
	t.Cleanup(func() { recActive.Store(false) })
	for i := range 20 {
		r := &winRecorder{}
		if err := r.Start(); err != nil {
			t.Fatalf("cycle %d: starting a recording: %v", i, err)
		}
		began := time.Now()
		done := make(chan struct{})
		go func() {
			r.Stop()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			// Unbounded is the old defect itself. Fatal rather than Error: the pump of
			// this cycle is still asleep with its hooks installed, and nineteen more
			// cycles would leave nineteen more.
			t.Fatalf("cycle %d: Stop has not returned after 30s. The pump was never "+
				"woken and nothing bounds the wait", i)
		}
		if took := time.Since(began); took > 2*time.Second {
			t.Fatalf("cycle %d: Stop took %v. The wakeup was not delivered and it fell "+
				"through to its own deadline, so the hooks it installed are still on "+
				"this desktop — a quieter version of the same defect", i, took)
		}
	}
}
