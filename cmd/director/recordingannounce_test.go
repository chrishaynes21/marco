package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// statedHome points the announcement at a home of the test's own.
func statedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	prev := recordingHome
	recordingHome = func() string { return home }
	t.Cleanup(func() { recordingHome = prev })
	return home
}

// Every Director entrance that captures a demonstration announces it BEFORE its hooks go on.
//
// Read from the source because two of the three install real hooks and cannot be entered without
// a desktop; the learn session, which can be, is entered below.
func TestDirectorRecordingEntrancesAnnounceBeforeTheirHooks(t *testing.T) {
	for _, tc := range []struct{ file, hooks string }{
		{"visualsession.go", "navsource.New()"},
		{"capturegamesession.go", "navsource.New()"},
		{"learnsessionwiring.go", "learn.New(name"},
	} {
		raw, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		a, h := strings.Index(src, "announceRecording("), strings.Index(src, tc.hooks)
		if a < 0 || h < 0 || a > h {
			t.Errorf("%s does not announce its recording before %s", tc.file, tc.hooks)
		}
		if !strings.Contains(src[a:h], "if !announced") {
			t.Errorf("%s does not refuse to start when the announcement fails", tc.file)
		}
	}
}

// A learn session that cannot be announced does not start.
func TestAnUnannouncedLearnSessionDoesNotStart(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "home")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := recordingHome
	recordingHome = func() string { return notADir }
	t.Cleanup(func() { recordingHome = prev })
	tc := &learnSession{}
	if _, err := tc.start("open downloads", &oneTeachPass{}, &recognisingMemory{}, nil,
		&countingGround{}, windowref.Selector{EphemeralID: "window_1"}, "Downloads",
		"Open"); err == nil {
		_, _ = tc.stop()
		t.Fatal("a learn session nothing was told about started")
	}
	if tc.running() {
		t.Error("the refused session is running")
	}
}
