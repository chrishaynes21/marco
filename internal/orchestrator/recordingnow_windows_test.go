//go:build windows

package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/recordingnow"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Windows-only: announcing a recording needs this process's start time, which only the Windows
// side can read. Elsewhere a recording is refused, which TestAnUnannouncedRecordingDoesNotStart
// holds on every platform.

// checkingRecorder records what the shared state said at the instant its hooks went on.
type checkingRecorder struct {
	fakeRecorder
	home    string
	atStart recordingnow.State
	started bool
}

func (c *checkingRecorder) Start() error {
	_, c.atStart = recordingnow.InProgress(c.home)
	c.started = true
	return c.fakeRecorder.Start()
}

func typedA() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvKey, KeyName: "a", Down: true, T: at(0)},
		{Kind: recorder.EvKey, KeyName: "a", Down: false, T: at(5)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(500)},
	}
}

// Announced from before the hooks go on until Finish.
//
// Moving the Mark after Start, or deleting the Clear in Finish, must fail this.
func TestARecordingIsAnnouncedBeforeItsHooksGoOn(t *testing.T) {
	home := t.TempDir()
	rec := &checkingRecorder{fakeRecorder: fakeRecorder{events: typedA()}, home: home}
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Rec: rec, Out: &strings.Builder{},
		StopKey: "esc", RecordingHome: home}
	c, err := d.Begin("type a")
	if err != nil {
		t.Fatal(err)
	}
	if rec.atStart != recordingnow.Recording {
		t.Fatalf("the hooks went on while the recording read as %v, not announced", rec.atStart)
	}
	c.Finish()
	if m, st := recordingnow.InProgress(home); st != recordingnow.None {
		t.Fatalf("a finished recording reads as %v %+v", st, m)
	}
}

// A recorder that fails to start leaves no announcement behind.
func TestARecorderThatFailsToStartLeavesNoAnnouncement(t *testing.T) {
	home := t.TempDir()
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Rec: failingRecorder{},
		Out: &strings.Builder{}, StopKey: "esc", RecordingHome: home}
	if _, err := d.Begin("x"); err == nil {
		t.Fatal("a recorder that failed to start began a recording")
	}
	if _, st := recordingnow.InProgress(home); st != recordingnow.None {
		t.Errorf("a recording that never started reads as %v", st)
	}
	if e, _ := os.ReadDir(filepath.Join(home, "recording")); len(e) != 0 {
		t.Errorf("left %v", e)
	}
}
