package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// F7: narration starts only in a window it was given, so the engine never asks "Which window" on
// the stdin narration is written into. With no window, nothing starts and phrases go back to being
// commands.
//
// Starting without --window, or leaving the session flag set, must fail this.
func TestNarrationStartsInAWindowItWasGiven(t *testing.T) {
	asked := marcoAnswers(t, "Narrate \"x\"\n", 0)
	prev := windowNow
	t.Cleanup(func() { windowNow = prev })

	windowNow = func() string { return "4660" }
	voiceActive.Store(true)
	h := newModel()
	if err := runNarrateLearn(h, "cloud pull"); err != nil {
		t.Fatal(err)
	}
	if got := asked(); got != "learn --narrate --window 4660 cloud pull" {
		t.Errorf("narration was started as %q", got)
	}

	argv := os.Getenv("MARCO_OVERLAY_ARGV")
	_ = os.Remove(argv)
	windowNow = func() string { return "" }
	voiceActive.Store(true)
	if err := runNarrateLearn(h, "cloud pull"); !errors.Is(err, errNoWindowToNarrate) {
		t.Errorf("with no window: %v", err)
	}
	if _, err := os.Stat(argv); err == nil {
		t.Error("a child was started with no window")
	}
	if voiceActive.Load() {
		t.Error("the session flag was left set, so every later phrase would be narration for nothing")
	}
	if !strings.Contains(errNoWindowToNarrate.Error(), "bring the window forward") {
		t.Error("the refusal does not say what to do")
	}
}
