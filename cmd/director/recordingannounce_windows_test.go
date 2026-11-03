//go:build windows

package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/recordingnow"
)

// Windows-only: announcing needs this process's start time, which only Windows can read.

// A learn session is announced from its start until its capture ends.
//
// Deleting the announcement in learnSession.start, or the endRecording in finish, must fail this.
func TestALearnSessionIsAnnouncedUntilItsCaptureEnds(t *testing.T) {
	home := statedHome(t)
	tc := &learnSession{}
	if _, err := tc.start("open downloads", &oneTeachPass{}, &recognisingMemory{}, nil,
		&countingGround{}, windowref.Selector{EphemeralID: "window_1"}, "Downloads",
		"Open"); err != nil {
		t.Fatalf("starting: %v", err)
	}
	defer func() { _, _ = tc.stop() }()
	if _, st := recordingnow.InProgress(home); st != recordingnow.Recording {
		t.Fatalf("a running learn session reads as %v, not announced", st)
	}
	if _, err := tc.finish(); err != nil {
		t.Fatal(err)
	}
	if _, st := recordingnow.InProgress(home); st != recordingnow.None {
		t.Errorf("after the capture ended the session still reads as %v", st)
	}
}
