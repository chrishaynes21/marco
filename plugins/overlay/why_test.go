package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/lastattempt"
)

// "WHY DIDN'T THAT WORK?", SAID OR TYPED, IS ANSWERED FROM THE RECORD AND IS NOT A REQUEST. The
// answer's first sentence is the status and every sentence is in the log; nothing reaches the
// intake. A request that merely starts with "why" is still a request.
//
// Removing the arm in dispatch (the question then goes to the intake as words) must fail this.
func TestWhyDidntThatWorkIsAnsweredFromTheRecord(t *testing.T) {
	var mu sync.Mutex
	var intake []string
	restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
		mu.Lock()
		intake = append(intake, strings.Join(args, " "))
		mu.Unlock()
		return childRun{result: "performed"}
	})
	defer restore()
	prev := askLast
	asked := 0
	askLast = func() (lastAnswer, string) {
		mu.Lock()
		asked++
		mu.Unlock()
		return lastAnswer{Known: true, Says: []string{"“cloud pull” was refused before anything ran (just now).",
			"Why: no value was given for \"resource\"", "Next: say it again with the value"}}, ""
	}
	t.Cleanup(func() { askLast = prev })
	voiceEnabled.Store(true)

	for _, c := range []struct{ action, said string }{
		{"RunVoice", "why didn't that work"}, {"Run", "why"}, {"Run", "What happened?"},
	} {
		h := newModel()
		dispatch(h, request{Action: c.action, Input: c.said})
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && !strings.Contains(h.snapshot().status, "refused") {
			time.Sleep(5 * time.Millisecond)
		}
		s := h.snapshot()
		if !strings.HasPrefix(s.status, "“cloud pull” was refused") {
			t.Errorf("%s %q: status %q", c.action, c.said, s.status)
		}
		if !strings.Contains(strings.Join(s.logs, "\n"), "Next: say it again") {
			t.Errorf("%s %q: the log lacks the next step: %q", c.action, c.said, s.logs)
		}
	}
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if asked != 3 || len(intake) != 0 {
		t.Errorf("asked the record %d times; the intake got %q", asked, intake)
	}
	intake = nil
	mu.Unlock()

	dispatch(newModel(), request{Action: "Run", Input: "why is the sky blue"})
	time.Sleep(80 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(intake) != 1 || asked != 3 {
		t.Errorf("an ordinary request starting with why: intake %q, record asked %d times", intake, asked)
	}
}

// AND THE WIDGET READS THE REAL `marco last`: a marco built from this tree, a record in a Marco
// home of the test's own, and the sentences come back. Built to a temporary path, never over the
// tree's marco.exe.
func TestTheWidgetReadsWhatMarcoWroteDown(t *testing.T) {
	marco := filepath.Join(t.TempDir(), "marco.exe")
	build := exec.Command("go", "build", "-o", marco, "./cmd/marco")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building cmd/marco: %v\n%s", err, out)
	}
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	t.Setenv("MARCO_BIN", marco)
	if err := lastattempt.Write(home, lastattempt.Attempt{At: time.Now().Format(time.RFC3339),
		Routine: "cloud pull", Outcome: "refused", Stage: lastattempt.StageValues,
		Because: "no value was given for \"resource\"", Next: "say it again with the value"}); err != nil {
		t.Fatal(err)
	}
	a, failed := readLastChild()
	if failed != "" || !a.Known || len(a.Says) != 3 ||
		!strings.HasPrefix(a.Says[0], "“cloud pull” was refused before anything ran") {
		t.Errorf("read %+v (%s)", a, failed)
	}
	os.Remove(lastattempt.Path(home))
	if a, _ := readLastChild(); a.Known || !strings.Contains(strings.Join(a.Says, " "), "Nothing has been asked for yet") {
		t.Errorf("with no record: %+v", a)
	}
}
