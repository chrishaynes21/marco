package orchestrator

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// fakeRecorder replays a canned event stream, so the whole teach pipeline
// (record → simplify → codegen → save → run) is testable with no OS hooks.
type fakeRecorder struct {
	events []recorder.RecordedEvent
	ch     chan recorder.RecordedEvent
}

func (f *fakeRecorder) Start() error {
	f.ch = make(chan recorder.RecordedEvent, len(f.events))
	for _, e := range f.events {
		f.ch <- e
	}
	close(f.ch)
	return nil
}
func (f *fakeRecorder) Stop() []recorder.RecordedEvent        { return f.events }
func (f *fakeRecorder) Events() <-chan recorder.RecordedEvent { return f.ch }

func at(ms int) time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(ms) * time.Millisecond)
}

func osSource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "programs", "os.marco"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestTeachThenRun(t *testing.T) {
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir, OS: osSource(t)}

	// Demonstration: click (100,200), wait ~350ms, click (400,500), then Esc.
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 100, Y: 200, T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 100, Y: 200, T: at(5)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 400, Y: 500, T: at(350)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 400, Y: 500, T: at(355)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(800)}, // stop gesture
	}

	var out bytes.Buffer
	d := Deps{Reg: reg, Rec: &fakeRecorder{events: events}, In: strings.NewReader("y\n"), Out: &out}

	// First Do: unknown → teaches, saves, and runs under dryrun.
	if err := d.Do("open chest"); err != nil {
		t.Fatalf("Do(teach): %v\n%s", err, out.String())
	}
	if !reg.Has("open chest") {
		t.Fatal("route was not saved")
	}
	// The Esc stop gesture must not appear as a step in the saved route.
	saved, _ := os.ReadFile(reg.Path("open chest"))
	if strings.Contains(string(saved), `Key with "esc"`) {
		t.Fatalf("stop key leaked into route:\n%s", saved)
	}
	if !strings.Contains(string(saved), "do OS's Click with p1.") ||
		!strings.Contains(string(saved), "do OS's Sleep with 350.") {
		t.Fatalf("route missing expected steps:\n%s", saved)
	}
	// It also ran (dryrun host logs the calls + the done line).
	if !strings.Contains(out.String(), "[dryrun] OS's Click") ||
		!strings.Contains(out.String(), "open chest: done") {
		t.Fatalf("route did not run; output:\n%s", out.String())
	}

	// Second Do: now known → runs directly without teaching.
	var out2 bytes.Buffer
	d2 := Deps{Reg: reg, Rec: &fakeRecorder{}, Out: &out2}
	if err := d2.Do("open chest"); err != nil {
		t.Fatalf("Do(run): %v", err)
	}
	if strings.Contains(out2.String(), "I don't know") {
		t.Fatalf("second Do re-taught instead of running:\n%s", out2.String())
	}
	if !strings.Contains(out2.String(), "open chest: done") {
		t.Fatalf("second Do did not run:\n%s", out2.String())
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"login to facebook":     "login-to-facebook",
		"Start Sea of Thieves!": "start-sea-of-thieves",
		"  spaced  out  ":       "spaced-out",
	}
	for in, want := range cases {
		if got := routes.Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}
