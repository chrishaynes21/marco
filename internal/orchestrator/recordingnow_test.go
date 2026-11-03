package orchestrator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

type failingRecorder struct{}

func (failingRecorder) Start() error                          { return errors.New("no hooks here") }
func (failingRecorder) Events() <-chan recorder.RecordedEvent { return nil }
func (failingRecorder) Stop() []recorder.RecordedEvent        { return nil }

// A recording that cannot be announced does not start: the hooks never go on.
//
// Ignoring Mark's error in Begin must fail this.
func TestAnUnannouncedRecordingDoesNotStart(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "home")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := &startFlag{}
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Rec: rec, Out: &strings.Builder{},
		StopKey: "esc", RecordingHome: notADir}
	if _, err := d.Begin("x"); err == nil {
		t.Fatal("a recording that could not be announced began")
	}
	if rec.started {
		t.Fatal("the hooks went on for a recording nothing was told about")
	}
}

type startFlag struct {
	fakeRecorder
	started bool
}

func (s *startFlag) Start() error { s.started = true; return s.fakeRecorder.Start() }
