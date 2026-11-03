//go:build windows

package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/recordingnow"
)

// Windows-only: announcing a recording needs this process's start time.

// A RECORDING IN THIS WINDOW IS ANNOUNCED TO THE REST OF MARCO, so the widget's shortcuts are held
// off while somebody demonstrates here — through StartRecording itself, in this window's store.
//
// Deleting `RecordingHome: a.home` in beginRecording must fail this.
func TestARecordingInStudioIsAnnouncedToTheWidget(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	a.recording = func() recorder.Recorder { return &heldRecorder{} }
	if why := a.StartRecording("save draft", ""); why != "" {
		t.Fatalf("StartRecording: %s", why)
	}
	m, st := recordingnow.InProgress(a.home)
	if st != recordingnow.Recording || m.What != "save draft" {
		t.Fatalf("a Studio recording was not announced in its store: %+v %v", m, st)
	}
	a.StopRecording()
	if _, st := recordingnow.InProgress(a.home); st != recordingnow.None {
		t.Error("the recording is still announced after Stop")
	}
}
