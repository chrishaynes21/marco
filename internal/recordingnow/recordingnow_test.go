package recordingnow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type proc struct {
	state liveness
	since time.Time
}

// stated makes processAt answer for a stated set of processes; any other pid is gone.
func stated(t *testing.T, ps map[uint32]proc) {
	t.Helper()
	prev := processAt
	processAt = func(pid uint32) (liveness, time.Time) {
		if p, ok := ps[pid]; ok {
			return p.state, p.since
		}
		return gone, time.Time{}
	}
	t.Cleanup(func() { processAt = prev })
}

var me = uint32(os.Getpid())

func TestARecordingIsInProgressUntilItIsCleared(t *testing.T) {
	home := t.TempDir()
	stated(t, map[uint32]proc{me: {alive, time.Unix(1000, 0)}})
	if _, st := InProgress(home); st != None {
		t.Fatalf("an empty home reads as %v, want None", st)
	}
	m, err := Mark(home, "save draft")
	if err != nil {
		t.Fatal(err)
	}
	got, st := InProgress(home)
	if st != Recording || got.What != "save draft" {
		t.Fatalf("a marked recording reads as %v %+v", st, got)
	}
	if !strings.Contains(got.Says(), `"save draft"`) {
		t.Errorf("the sentence does not name the recording: %s", got.Says())
	}
	Clear(m)
	if _, st := InProgress(home); st != None {
		t.Errorf("a cleared recording reads as %v", st)
	}
}

// A recording that cannot be announced does not start: Mark refuses and writes nothing.
func TestARecordingThatCannotBeAnnouncedIsRefused(t *testing.T) {
	home := t.TempDir()
	stated(t, map[uint32]proc{me: {unsure, time.Time{}}})
	if _, err := Mark(home, "x"); err == nil {
		t.Fatal("a process that could not be identified was announced")
	}
	if entries, _ := os.ReadDir(filepath.Join(home, "recording")); len(entries) != 0 {
		t.Errorf("a refused announcement left %v", entries)
	}
}

// Only a process CONFIRMED gone makes a marker stale; one that cannot be read is Unknown and kept.
func TestOnlyAConfirmedDeadRecorderIsStale(t *testing.T) {
	home := t.TempDir()
	stated(t, map[uint32]proc{me: {alive, time.Unix(1000, 0)}})
	if _, err := Mark(home, "x"); err != nil {
		t.Fatal(err)
	}
	markers := func() int {
		e, _ := os.ReadDir(filepath.Join(home, "recording"))
		return len(e)
	}

	stated(t, map[uint32]proc{me: {unsure, time.Time{}}})
	if _, st := InProgress(home); st != Unknown {
		t.Errorf("an unreadable recorder reads as %v, want Unknown", st)
	}
	if markers() != 1 {
		t.Error("a marker whose process could not be read was swept")
	}

	stated(t, map[uint32]proc{me: {alive, time.Unix(2000, 0)}}) // the pid was reused
	if _, st := InProgress(home); st != None {
		t.Errorf("a reused pid reads as %v, want None", st)
	}
	if markers() != 0 {
		t.Error("the stale marker was not swept")
	}

	stated(t, map[uint32]proc{me: {alive, time.Unix(1000, 0)}})
	if _, err := Mark(home, "y"); err != nil {
		t.Fatal(err)
	}
	stated(t, map[uint32]proc{}) // gone
	if _, st := InProgress(home); st != None {
		t.Errorf("a recorder that is gone reads as %v, want None", st)
	}
}

// An unreadable marker, or unreadable state, is Unknown — never "no recording".
func TestUnreadableStateIsUnknownNotAbsent(t *testing.T) {
	home := t.TempDir()
	d := filepath.Join(home, "recording")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "1-1.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, st := InProgress(home); st != Unknown {
		t.Errorf("a corrupt marker reads as %v, want Unknown", st)
	}
	if _, st := InProgress(""); st != Unknown {
		t.Errorf("no home reads as %v, want Unknown", st)
	}
	// A file where the directory should be cannot be listed.
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "recording"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, st := InProgress(other); st != Unknown {
		t.Errorf("an unlistable directory reads as %v, want Unknown", st)
	}
}

// No home stated, nothing written.
func TestNoHomeAnnouncesNothing(t *testing.T) {
	m, err := Mark("", "x")
	if err != nil || m.path != "" {
		t.Fatalf("Mark with no home: %+v %v", m, err)
	}
}
