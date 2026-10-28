//go:build windows

package stopsignal

// A STOP THAT COULD NOT BE RAISED SAYS SO, AND SAYS WHAT IT MEANS.
//
// # The investigation this records
//
// Three desktop scenarios carry a note saying `Raise` "has been seen to fail with Access is
// denied", recorded as unexplained. It is explained: `os.Rename` over the generation file fails
// with exactly that whenever any process holds the file open, and a watcher polling the
// generation holds it open every 100ms to read it. The stop is therefore most likely to fail at
// precisely the moment it matters — when something is listening for one.
//
// Reproduced here deterministically by holding the file open, which is what a watcher does.
//
// # What this test is for
//
// The WORDS. "Access is denied" sends somebody to look at permissions for a transient sharing
// conflict, and both entrances surface whatever this returns: `marco stop` logs it, Studio's Stop
// button puts it on the page. Nothing is retried — that is a policy about how hard to try to stop
// somebody's computer and is not a decision for an error path.
//
// See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].

import (
	"os"
	"strings"
	"testing"
)

func TestAStopThatCouldNotBeRaisedSaysWhatItMeans(t *testing.T) {
	home := t.TempDir()
	if err := Raise(home); err != nil {
		t.Fatalf("the first raise, with nothing holding anything: %v", err)
	}
	was := Generation(home)

	// HELD OPEN, THE WAY A WATCHER READING IT HOLDS IT.
	f, err := os.Open(Path(home))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	err = Raise(home)
	if err == nil {
		t.Skip("this platform replaces a file that is held open, so the failure this " +
			"is about does not arise here")
	}
	for _, says := range []string{
		"was not raised",              // what did not happen
		"has been asked to stop",      // and what that means
		"while something has it open", // why, in terms of the machine
		"Press stop again",            // and what to do
	} {
		if !strings.Contains(err.Error(), says) {
			t.Errorf("the refusal does not say %q: %v", says, err)
		}
	}
	// AND IT IS NOT A PERMISSIONS PROBLEM, which is what the bare error reads as.
	if strings.Contains(err.Error(), "permission") {
		t.Errorf("the refusal talks about permissions, which is the wrong place to "+
			"send somebody: %v", err)
	}
	// AND THE GENERATION DID NOT MOVE, so nothing is fooled into thinking a stop went
	// out. This is the half that matters to a watcher: a raise that failed must not
	// leave a number that looks like a stop.
	if now := Generation(home); now != was {
		t.Errorf("the generation went %d → %d for a raise that failed, so a watcher "+
			"would act on a stop nobody managed to send", was, now)
	}
	// AND NO TEMPORARY FILE IS LEFT BEHIND.
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != FileName {
			t.Errorf("a failed raise left %s behind in the store", e.Name())
		}
	}
	// AND WITH NOTHING HOLDING IT, IT WORKS — so the above is about the conflict and
	// not about a Raise that has stopped working.
	f.Close()
	if err := Raise(home); err != nil {
		t.Fatalf("a raise with nothing holding the file: %v", err)
	}
	if Generation(home) <= was {
		t.Error("the generation did not rise after a successful raise")
	}
}
