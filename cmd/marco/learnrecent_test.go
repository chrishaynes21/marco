package main

import (
	"errors"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// LEARNING THE RECENT PAST DOES NOT START A DIRECTOR.
//
// # Why this is sharper than the same rule for `observe status`
//
// A Director that has just been started has watched NOTHING. The only answer it could give is "I
// don't know what you just did" — and it would have started a background process to say so. If
// nothing is running then nothing was watching, and that is the honest answer on its own.
//
// The FLAG is what is asserted, through the one dialler this command uses, for the reason
// TestOnlyAskingMarcoToWatchMayStartADirector gives: pointing the binary somewhere that does not
// exist makes an autostart fail and leaves no endpoint, so such a test passes whether or not the
// autostart was attempted.
//
// Deleting the false must fail this.
func TestLearningTheRecentPastDoesNotStartADirector(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())

	var asked []bool
	restore := learnRecentDial
	learnRecentDial = func(autoStart bool) (*service.Client, error) {
		asked = append(asked, autoStart)
		return nil, errors.New("nothing running")
	}
	t.Cleanup(func() { learnRecentDial = restore })

	if code := runLearnRecent("open mouse settings"); code != 1 {
		t.Errorf("exited %d with nothing running; the person is owed the sentence that "+
			"says why, and a Director that was never watching cannot answer", code)
	}
	if len(asked) != 1 {
		t.Fatalf("dialled %d times", len(asked))
	}
	if asked[0] {
		t.Error("asking Marco to learn what it just watched started a Director. One that " +
			"has just started has watched nothing, so the process would exist only to " +
			"say it does not know.")
	}
}
