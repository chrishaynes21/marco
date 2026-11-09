package main

// EVERY ATTEMPT LEAVES A RECORD, INCLUDING THE ONES THAT NEVER STARTED.
//
// These drive `runPerformWorkflow` — the production orchestration — rather than the helpers it
// calls. That distinction is the whole reason this file exists: `started`, `countdown` and
// `saveRun` were each tested on their own and all three passed, while the production path
// returned before any of them could write anything. The attempts somebody is most likely to ask
// about — cancelled at the prompt, a window that could not be acquired, a scope that refused —
// were the attempts with no reference and no record.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/runs"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
)

// aSavedRoutine puts a named pixel routine in a temporary home and returns its name.
func aSavedRoutine(t *testing.T, name string) {
	t.Helper()
	store, err := demo.Open(configDir())
	if err != nil {
		t.Fatalf("opening the store: %v", err)
	}
	if err := store.SaveSequence(aNamedDemonstration(name)); err != nil {
		t.Fatalf("saving %q: %v", name, err)
	}
}

// theOnlyRecord reads the single run record a test produced.
func theOnlyRecord(t *testing.T) RunRecord {
	t.Helper()
	dir := runs.Dir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("no run records were written at all: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("%d run record(s), want exactly one", len(entries))
	}
	b, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var rec RunRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatalf("the record is not readable: %v", err)
	}
	return rec
}

// TestAWindowThatCannotBeAcquiredIsRecordedAsFailed.
//
// A startup failure, driven through the real command. `RunFailed` was declared and never
// produced before this: nothing on a startup path reached a classifier.
func TestAWindowThatCannotBeAcquiredIsRecordedAsFailed(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	aSavedRoutine(t, "a play")

	code := runPerformWorkflow([]string{
		"--application", "nosuchapplicationanywhere",
		"--name", "a play", "--seconds", "5",
	})
	if code == 0 {
		t.Error("a run against an application with no window reported success")
	}
	rec := theOnlyRecord(t)
	if rec.Outcome != RunFailed {
		t.Errorf("outcome %q, want %q", rec.Outcome, RunFailed)
	}
	if mustSay(t, rec) != 0 {
		t.Errorf("%d action(s) sent by a run that never acquired a window", mustSay(t, rec))
	}
	if rec.Workflow != "a play" {
		t.Errorf("the record does not name the play: %+v", rec)
	}
	// AND THE REFERENCE RETRIEVES IT, which is the whole point of printing one.
	if _, err := os.Stat(runPath(rec.Run)); err != nil {
		t.Errorf("the cited record is not where the reference says: %v", err)
	}
}

// TestAStartPromptCancellationIsRecordedAsCancelled.
//
// The F9 path, through the production command: a synthetic key source stands in for the hook,
// the stop key is pressed while the start prompt is open, and the attempt must be recorded as
// cancelled with nothing sent. This is the early return that used to leave nothing behind.
func TestAStartPromptCancellationIsRecordedAsCancelled(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	aSavedRoutine(t, "a play")

	// A SYNTHETIC HOOK IN PLACE OF THE DESKTOP'S, so the production command can be
	// driven through its own start prompt.
	was := newKeySource
	t.Cleanup(func() { newKeySource = was })
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	newKeySource = func() (*navsource.Source, string) { return src, "" }
	go func() {
		time.Sleep(30 * time.Millisecond)
		press(navsource.KeyF9, true)
		press(navsource.KeyF9, false)
	}()

	code := runPerformWorkflow([]string{
		"--application", "nosuchapplicationanywhere",
		"--name", "a play", "--hotkeys", "--delay", "10s", "--seconds", "5",
	})
	if code == 0 {
		t.Error("a cancelled run reported success")
	}
	rec := theOnlyRecord(t)
	if rec.Outcome != RunCancelled {
		t.Errorf("outcome %q, want %q — a run somebody stopped is not a run that "+
			"failed", rec.Outcome, RunCancelled)
	}
	if mustSay(t, rec) != 0 {
		t.Errorf("%d action(s) sent by a run cancelled at the start prompt", mustSay(t, rec))
	}
}

// TestAStopSaidAnywhereReachesASupervisedRun.
//
// # The condition
//
// Three things can stop this command, and until now only two of them could. Ctrl-C is a
// terminal's stop and F9 is a desktop's; the one a person actually uses is the word. Typed into
// the overlay or spoken, "stop" reaches `marco`'s one stop, which raises the local generation
// every running Play watches and tells the Director service to drop what it is holding — and a
// supervised run is neither of those. It is a command, started by a front end, with no terminal
// to interrupt.
//
// So the moment the product could start a run from the widget, it could start one it could not
// stop by the only means the widget offers. That is the wrong way round for a path whose whole
// design is that a person stays in charge of every action.
//
// Nothing new is raised and no second broadcast exists: this watches the same generation in the
// same home that `marco do stop` already raises. And because a stop arrives as a cancelled
// context, the classifier already reads it as cancellation rather than failure.
func TestAStopSaidAnywhereReachesASupervisedRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	aSavedRoutine(t, "a play")

	// A HOOK THAT NEVER ANSWERS, so the start prompt would wait out its own patience
	// and the only thing that can end this run is the word.
	was := newKeySource
	t.Cleanup(func() { newKeySource = was })
	src, _ := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	newKeySource = func() (*navsource.Source, string) { return src, "" }

	// AND THE WORD, FROM SOMEWHERE ELSE ENTIRELY. This is what `marco do stop` does,
	// in the same home, through the same one definition.
	go func() {
		time.Sleep(50 * time.Millisecond)
		if err := stopsignal.Raise(home); err != nil {
			t.Errorf("the stop could not be published: %v", err)
		}
	}()

	began := time.Now()
	code := runPerformWorkflow([]string{
		"--application", "nosuchapplicationanywhere",
		"--name", "a play", "--hotkeys", "--delay", "30s", "--seconds", "5",
	})
	took := time.Since(began)

	if code == 0 {
		t.Error("a stopped run reported success")
	}
	// AND IT STOPPED WHEN IT WAS TOLD, not when its own patience ran out. The delay
	// alone is thirty seconds; a run that waited that out heard nothing.
	if took > 10*time.Second {
		t.Errorf("the run took %s to hear a stop, so the word did not reach it", took)
	}
	rec := theOnlyRecord(t)
	if rec.Outcome != RunCancelled {
		t.Errorf("outcome %q, want %q — a run somebody stopped is not a run that "+
			"failed", rec.Outcome, RunCancelled)
	}
	if mustSay(t, rec) != 0 {
		t.Errorf("%d action(s) sent by a run that was stopped", mustSay(t, rec))
	}
}
