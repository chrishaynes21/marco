//go:build windows

package navsource

import (
	"runtime"
	"testing"
	"time"
)

// The hook LIFECYCLE, against the real OS.
//
// No fake Windows. These install genuine global low-level hooks and take them down again, because
// the thing worth proving is exactly the part a fake would stub out: that a blocking pump can be
// woken, that the thread which installed a hook is the one that removes it, and that doing it
// repeatedly leaves nothing behind.
//
// A leaked hook is invisible from inside the process, which is why it is worth a test rather than
// a habit. What a leak COSTS on a live desktop is not established here and is not claimed: see the
// retraction in the hook-pump note. This asserts only that shutdown is complete.

// closeMustBePrompt is the bound on Close.
//
// Well under the backend's own 2s fallback, so a Close that returns "in time" cannot have got
// there by timing out. Generous enough not to flake on a loaded machine.
const closeMustBePrompt = 1500 * time.Millisecond

// TestClosingASourceWakesTheBlockingPump is the wakeup, measured.
//
// A blocking GetMessage returns when a message arrives and at no other time. Delete the posted
// WM_QUIT and this does not hang — it quietly falls back to the timeout, leaving the hooks
// installed for the life of the process, which is the failure that must not be silent.
func TestClosingASourceWakesTheBlockingPump(t *testing.T) {
	s, why := New()
	if why != "" {
		t.Skipf("no navigation source on this machine: %s", why)
	}
	start := time.Now()
	s.Close()
	if took := time.Since(start); took > closeMustBePrompt {
		t.Fatalf("Close took %s. The pump was not woken — it timed out, and on that path the "+
			"hooks are still installed with the thread that owns them gone", took)
	}
	if hookActive.Load() {
		t.Error("the hooks are still marked active after Close")
	}
}

// TestRepeatedStartStopLeavesNothingBehind is the accumulation test.
//
// Each cycle installs two global hooks and a locked OS thread. If shutdown is incomplete in any
// way — the pump not woken, the unhook skipped, the thread not returning — the goroutine count
// climbs and the hooks stay registered with no thread able to remove them. What that costs a live
// desktop is unmeasured; that it is wrong is not in question.
func TestRepeatedStartStopLeavesNothingBehind(t *testing.T) {
	first, why := New()
	if why != "" {
		t.Skipf("no navigation source on this machine: %s", why)
	}
	first.Close()
	settle()
	baseline := runtime.NumGoroutine()

	const cycles = 5
	for i := 0; i < cycles; i++ {
		s, why := New()
		if why != "" {
			t.Fatalf("cycle %d: %s", i, why)
		}
		start := time.Now()
		s.Close()
		if took := time.Since(start); took > closeMustBePrompt {
			t.Fatalf("cycle %d: Close took %s", i, took)
		}
	}
	settle()

	// Slack for the runtime's own workers, not for a leak: a leak is one pump goroutine and
	// one classifier per cycle, which at five cycles is comfortably outside this.
	if grew := runtime.NumGoroutine() - baseline; grew > 2 {
		t.Fatalf("%d goroutine(s) survived %d start/stop cycles. Each leaked pump is a locked "+
			"OS thread holding two global input hooks that nothing can now remove",
			grew, cycles)
	}
}

// TestASourceThatWasNeverStartedClosesCleanly holds the race the ready gate exists for.
//
// Close may arrive before the pump has published its thread id. It must not block forever waiting
// for a thread that is still starting, and must not skip the unhook for one that already has.
func TestASourceThatWasNeverStartedClosesCleanly(t *testing.T) {
	s, why := New()
	if why != "" {
		t.Skipf("no navigation source on this machine: %s", why)
	}
	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(closeMustBePrompt * 2):
		t.Fatal("Close blocked. A stop racing a start must not wait on a thread id that " +
			"may never be published")
	}
	// Twice is not an error. Close is idempotent, and a second one must not hang either.
	s.Close()
}

// settle gives retiring goroutines a moment to actually retire.
func settle() {
	for i := 0; i < 20; i++ {
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
	}
}
