package main

// WHEN A DEMONSTRATION SESSION ENDS, AND WHEN IT MUST NOT.
//
// `demonstrate-visual` offers "Press ENTER when you are done", so a person is not held to a
// clock. The reader for that ENTER ended the session on any error — EOF included — and EOF is
// what a shell with no console gives you immediately.
//
// Measured on the first live human demonstration this project attempted: the session closed
// before it began and reported `0 screen(s) seen, words offered 0`, which reads as a perception
// failure and is a closed stdin. Two of the owner's clicks into a live game were spent on it
// before anybody looked at the reader.

import (
	"io"
	"strings"
	"testing"
	"time"
)

// closes reports whether the channel closed within a moment.
func closes(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	case <-time.After(250 * time.Millisecond):
		return false
	}
}

// TestASessionNobodyCanEndRunsToItsCeiling is the defect.
func TestASessionNobodyCanEndRunsToItsCeiling(t *testing.T) {
	// A READER AT EOF, which is what a shell with no console hands over.
	if closes(endsWhenSomebodySaysSo(strings.NewReader(""))) {
		t.Fatal("a session with nobody to press ENTER ended immediately; the " +
			"demonstration is over before the person has done anything, and the " +
			"report calls it a perception failure")
	}
	// AND A READER THAT NEVER ANSWERS, which is a terminal nobody is sitting at.
	r, w := io.Pipe()
	defer func() { _ = w.Close() }()
	if closes(endsWhenSomebodySaysSo(r)) {
		t.Error("a session ended although nobody had said anything")
	}
}

// TestSomebodySayingSoStillEndsIt, because the whole point is not to hold them to a clock.
func TestSomebodySayingSoStillEndsIt(t *testing.T) {
	if !closes(endsWhenSomebodySaysSo(strings.NewReader("\n"))) {
		t.Error("ENTER did not end the session, so a person is held to --seconds " +
			"whether they are finished or not")
	}
	// AND A LINE WITH SOMETHING ON IT IS STILL A LINE.
	if !closes(endsWhenSomebodySaysSo(strings.NewReader("done\n"))) {
		t.Error("a typed line did not end the session")
	}
}
