package main

// WHEN A SUPERVISED RUN BEGINS, AND WHO DECIDES.
//
//	Goal: make a supervised Marco trial possible without guessing when capture or execution
//	starts. […] An explicit Start interaction after the user has prepared the trial.
//
// `--delay` is a guess about when somebody will be ready. Every live trial in this project was
// either too short — the foreground was still the terminal when discovery ran, so a workflow
// scoped to a game resolved `code` — or long enough that it was spent waiting.

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
)

func TestAStartKeyBeginsTheRun(t *testing.T) {
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	go func() {
		time.Sleep(20 * time.Millisecond)
		press(navsource.KeyF7, true)
		press(navsource.KeyF7, false)
	}()
	if got := started(context.Background(), src, 10*time.Second); got != startPressed {
		t.Errorf("start state %v, want the run to begin when the key is pressed", got)
	}
}

// TestTheStopKeyCancelsBeforeAnythingIsObserved.
//
// Nothing has been acquired, authorised or pressed at this point, so a stop here is a
// cancellation rather than a refusal of a question — and the caller returns without building
// anything that could send input.
func TestTheStopKeyCancelsBeforeAnythingIsObserved(t *testing.T) {
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	go func() {
		time.Sleep(20 * time.Millisecond)
		press(navsource.KeyF9, true)
		press(navsource.KeyF9, false)
	}()
	if got := started(context.Background(), src, 10*time.Second); got != startCancelled {
		t.Errorf("start state %v, want cancellation", got)
	}
}

// TestARefusalKeyAnswersNothingBeforeTheRun.
//
// No question is open yet. Treating F8 as a cancellation would make the three keys mean one
// thing before the run and another during it, and the person holding them cannot see which
// phase they are in — which is the whole problem this slice exists to fix.
func TestARefusalKeyAnswersNothingBeforeTheRun(t *testing.T) {
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	go func() {
		time.Sleep(10 * time.Millisecond)
		press(navsource.KeyF8, true)
		press(navsource.KeyF8, false)
		// AND THEN THE REAL ANSWER, which must still be heard.
		time.Sleep(10 * time.Millisecond)
		press(navsource.KeyF7, true)
		press(navsource.KeyF7, false)
	}()
	if got := started(context.Background(), src, 10*time.Second); got != startPressed {
		t.Errorf("start state %v — a refusal key before the run ended the wait", got)
	}
}

// TestAnUnattendedRunStartsByItself, because a forgotten terminal must still stop by itself.
func TestAnUnattendedRunStartsByItself(t *testing.T) {
	src, _ := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	start := time.Now()
	if got := started(context.Background(), src, 30*time.Millisecond); got != startTimedOut {
		t.Errorf("start state %v, want the delay to start it unattended", got)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("the ceiling took %s to fire", took)
	}
}

// TestCancellingTheContextCancelsTheStart, which is Ctrl-C during the wait.
func TestCancellingTheContextCancelsTheStart(t *testing.T) {
	src, _ := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := started(ctx, src, time.Hour); got != startCancelled {
		t.Errorf("start state %v, want cancellation", got)
	}
}
