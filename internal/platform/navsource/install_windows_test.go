//go:build windows

package navsource

import (
	"strings"
	"testing"
)

// WHAT TWO HOOK OUTCOMES MEAN FOR THE PRODUCER, held apart from the platform call.
//
// No test can make Windows refuse a hook, so the call itself stays unexercised; this holds the
// rule that reads its result. A mutation run showed that rule could be deleted outright with the
// whole suite still green, which is the same shape as the defect it exists to prevent — a
// producer that reports success because nothing checked.
func TestWhatTwoHookOutcomesMeanForTheProducer(t *testing.T) {
	if err := installVerdict("", ""); err != nil {
		t.Errorf("both hooks installed and the producer was refused: %v", err)
	}
	if err := installVerdict("no keyboard hook", ""); err != nil {
		t.Errorf("a producer that still watches the mouse was refused outright: %v\n"+
			"Losing one device degrades what can be correlated, not what can be "+
			"seen, and Stats names which half is missing.", err)
	}
	if err := installVerdict("", "no mouse hook"); err != nil {
		t.Errorf("a producer that still watches the keyboard was refused outright: %v", err)
	}
	err := installVerdict("no keyboard hook", "no mouse hook")
	if err == nil {
		t.Fatal("a producer with NEITHER hook reported itself available.\nNothing " +
			"downstream can then tell a silent desktop from a missing hook, which " +
			"is the defect ADR-212 exists for.")
	}
	for _, want := range []string{"no keyboard hook", "no mouse hook"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}
}
