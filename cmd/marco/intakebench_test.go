package main

import (
	"testing"
	"time"
)

// What asking "is Director mid-question" costs when Director is not running.
//
// It is on the path of every invocation that carries words, so it had better be cheap when the
// answer is trivially no. It connects WITHOUT auto-starting, so an absent service is one failed
// dial and not a twenty-second service start.
func TestThePendingCheckIsCheapWhenNothingIsRunning(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir()) // no endpoint file: the service is not running
	const n = 20
	start := time.Now()
	for i := 0; i < n; i++ {
		if directorPending() {
			t.Fatal("a temp home reported a pending question")
		}
	}
	each := time.Since(start) / n
	t.Logf("directorPending with no service: %v per call", each)
	// Generous, because this is a guard against a REGRESSION — an auto-start, a retry loop, or
	// a dial timeout landing on this path — not a performance target. Twenty seconds is what
	// auto-starting would cost; two milliseconds is what the absent-endpoint path costs.
	if each > 250*time.Millisecond {
		t.Fatalf("the pending check costs %v per invocation with no Director running; "+
			"something on this path is dialling, retrying or starting a service", each)
	}
}
