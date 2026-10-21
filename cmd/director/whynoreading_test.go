package main

// WHY THERE IS NO READING, WHICH USED TO BE ONE SENTENCE OVER THREE SITUATIONS.
//
// The owner's first live Act refused twice in a row on the screen it starts on, saying "nothing
// has been read on it yet" both times, for four seconds each. That sentence is true of a source
// that has not answered yet, of a source whose every answer arrives about a screen that has
// already changed, and of a look whose pixels were never the application's — and the remedy is
// different in all three. One of them is fixed by waiting, one by holding still, and one by
// clicking the game's own window.

import (
	"strings"
	"testing"
)

func TestEveryWayAScreenCanGoUnreadHasItsOwnSentence(t *testing.T) {
	for _, c := range []struct {
		what     string
		unusable string
		wordless int
		stale    int
		says     string
	}{
		{"nothing has come back yet", "", 0, 0, "nothing has been read on it yet"},
		{"the pixels were not the application's",
			"the target was not the window in front", 0, 0,
			"not the window in front"},
		{"the words came back empty", "", 2, 0, "found none"},
		{"every reading arrived about an older screen", "", 0, 7,
			"7 reading(s)"},
		// AND THE ONE A PERSON CAN ACT ON COMES FIRST. A look that was not
		// evidence about the application explains the empty reading AND the stale
		// ones; saying either of those instead would send somebody to hold still
		// in front of a window that is not the one being read.
		{"all of them at once", "the target was not the window in front", 3, 9,
			"not the window in front"},
	} {
		got := whyNoReading(c.unusable, c.wordless, c.stale)
		if !strings.Contains(got, c.says) {
			t.Errorf("%s: says %q, which does not mention %q",
				c.what, got, c.says)
		}
	}
}

// AND A RUN WHOSE READINGS KEEP ARRIVING STALE IS TOLD THAT WAITING IS NOT THE ANSWER, because
// it is the one of these situations that patience cannot reach — and Studio now gives a reading
// run fifteen seconds of it.
func TestAScreenThatKeepsMovingSaysWaitingWillNotHelp(t *testing.T) {
	got := whyNoReading("", 0, 4)
	for _, want := range []string{"keeps moving", "waiting longer will not change that"} {
		if !strings.Contains(got, want) {
			t.Errorf("the sentence for stale readings does not say %q: %s", want, got)
		}
	}
}
