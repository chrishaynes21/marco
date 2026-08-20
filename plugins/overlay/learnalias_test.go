package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// `teach <name>` still starts a demonstration.
//
// # Why an alias in the overlay needs its own test
//
// LEARN, TEACH and DO are three different words vertically, and the product settled on LEARN for
// the person acting while Marco watches. `teach` survives as an undocumented alias for the muscle
// memory the product shipped with — somebody who has typed it for months types it again — and an
// alias nothing exercises is an alias that goes away in the next tidy-up with nothing to say so.
//
// The CLI verb has its own guard in cmd/marco (TestTheLearnVerbAnswersToItsOldName). This is a
// DIFFERENT surface: a separate Go module, its own dispatch table, and a word typed into the HUD
// rather than onto a command line. An audit found this half untested.
//
// What is asserted is that the word reaches the LEARN ARM and not the intake: if the alias were
// dropped, "teach open downloads" would fall through to the desktop-intent path and be sent to
// Director as a request to interpret — which reads, to the person, as Marco having forgotten how
// to learn.
//
// Deleting the "teach" arm in the dispatch must fail this.
func TestTheLearnWordAnswersToItsOldName(t *testing.T) {
	for _, word := range []string{"learn", "teach"} {
		t.Run(word, func(t *testing.T) {
			// NOTHING MAY SPAWN marco.exe HERE: on this surface it performs real input,
			// and `learn` would start the recorder. Pointing MARCO_BIN at a path that
			// cannot exist makes the child fail to start instead.
			t.Setenv("MARCO_BIN", filepath.Join(t.TempDir(), "no-such-marco.exe"))
			restore := stubIntakeChild(t, func(_ *model, name string, _ bool, args ...string) childRun {
				t.Errorf("%q was sent to the intake as desktop intent (%q); the alias no "+
					"longer starts a demonstration", word, args)
				return childRun{}
			})
			defer restore()

			h := newModel()
			got := dispatch(h, request{Action: "Run", Input: word + " open downloads"})
			if got.Status != "ok" {
				t.Fatalf("%q was refused: %+v", word, got)
			}

			// The HUD went into a recording session, naming what is being learned. Both
			// happen synchronously in startLearn, before the child is spawned.
			s := h.snapshot()
			if !s.learnSession {
				t.Fatalf("%q did not open a learn session", word)
			}
			if !strings.Contains(strings.Join(s.logs, "\n"), `recording "open downloads"`) {
				t.Errorf("the HUD never said what it was recording: %q", s.logs)
			}

			// And the NAME is what follows the verb, not the whole phrase.
			if strings.Contains(strings.Join(s.logs, "\n"), `recording "`+word) {
				t.Errorf("the verb was kept as part of the name: %q", s.logs)
			}

			// Let the failed child finish so the next subtest starts clean.
			waitLearnDone(t, h)
		})
	}
}

// A bare `teach` is the same refusal a bare `learn` is: a demonstration needs a name.
func TestTheOldNameStillNeedsAName(t *testing.T) {
	restore := stubIntakeChild(t, func(_ *model, name string, _ bool, args ...string) childRun {
		t.Errorf("a nameless demonstration reached the intake: %q", args)
		return childRun{}
	})
	defer restore()
	if got := dispatch(newModel(), request{Action: "Run", Input: "teach"}); got.Status == "ok" {
		t.Fatalf("`teach` with no name was accepted: %+v", got)
	}
}

// waitLearnDone waits for the background learn goroutine to give the session back.
func waitLearnDone(t *testing.T, h *model) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !h.snapshot().learnSession {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the learn session never ended; the child was not the stub")
}
