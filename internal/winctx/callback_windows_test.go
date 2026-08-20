//go:build windows

package winctx

import "testing"

// The callback table is finite and Go never frees it.
//
// syscall.NewCallback allocates from a fixed process-wide table. A callback built per call
// is therefore a permanent leak, and this one killed a running service:
//
//	fatal error: too many callback functions
//	  winctx.Monitors()  →  winctx.onScreen()  →  winctx.LiveWindows()
//
// Monitors had been leaking quietly for a long time because it was rarely called. Then
// LiveWindows started calling onScreen per window, which called Monitors per window, and a
// passive observation session sampling every two seconds exhausted the table in under three
// minutes.
//
// A few thousand iterations is well past the limit, so a regression crashes the test binary
// outright rather than failing an assertion. That is the correct severity: this is not a
// wrong answer, it is a process that dies.

func TestEnumerationDoesNotLeakCallbacks(t *testing.T) {
	for i := 0; i < 3000; i++ {
		if _, err := Monitors(); err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
	}
}

func TestLiveWindowsDoesNotLeakCallbacks(t *testing.T) {
	// The exact path that crashed: enumeration, with an on-screen test per window.
	for i := 0; i < 1500; i++ {
		_ = LiveWindows()
	}
}
