package main

// THE TWO BINARIES, FROM THIS REVISION, TESTED TOGETHER.
//
// # Why the fakes are not enough
//
// Everything in demonstrated_test.go substitutes the spawn, which is right for the questions it
// asks — what the door decided, what the runner was asked for, what each outcome means to a
// person. None of it can fail when the two halves disagree, and there are exactly three ways
// they can:
//
//  1. A FLAG. The front end passes `--hotkeys` and `--reading`; a runner that renamed one would
//     exit 2 on a usage refusal, and a fake spawn would go on reporting success forever.
//  2. THE REFERENCE FORMAT. The outcome a person is told is read off the line the runner prints.
//     Both sides now go through `runref`, and this is what proves they do.
//  3. THE VOCABULARY. The runner's seven words are mapped to Marco's; a word added on one side
//     and not the other would arrive as an unknown.
//
// Those are the failures a front end cannot see, because each one looks exactly like a run that
// did not work. So this builds `director.exe` from the working tree and drives the production
// path into it.
//
// # And it sends nothing
//
// The routine is scoped to an application that does not exist on this machine, nothing is
// authorised — the runner's questions are answerable only by a real function key, which this
// process cannot send and must not simulate — and the run is ended by the word "stop", raised the
// way `marco do stop` raises it. Zero actions, asserted from the run's own record.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
)

// theRunnerBuiltFromThisTree builds `director` and returns the path to it.
//
// From the TREE, not from anything installed: the point is that the front end and the runtime
// agree at this revision. A `director.exe` lying around from last week would make the three
// drifts above invisible, which is the whole thing being tested.
func theRunnerBuiltFromThisTree(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "director")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	build := exec.Command("go", "build", "-o", out,
		"github.com/chaynes-simpleclouds/marco/cmd/director")
	build.Dir = repoRoot(t)
	if b, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the runner from this tree: %v\n%s", err, b)
	}
	return out
}

// repoRoot walks up from this package to the module root, so `go build` runs inside the module
// whatever directory the test was started from.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("no go.mod above the working directory, so there is no module to build")
	return ""
}

// aRunRecordIn reads the one record the run left behind.
//
// Read as JSON by a program that is not the one that wrote it, which is what a run reference is
// FOR. The fields are the record's documented ones; a test that reached for the writer's own
// struct would prove only that a package can read itself.
func aRunRecordIn(t *testing.T, home string) struct {
	Outcome string `json:"outcome"`
	Sent    int    `json:"actions_sent"`
	Live    bool   `json:"live"`
	Name    string `json:"workflow"`
} {
	t.Helper()
	var rec struct {
		Outcome string `json:"outcome"`
		Sent    int    `json:"actions_sent"`
		Live    bool   `json:"live"`
		Name    string `json:"workflow"`
	}
	paths, err := filepath.Glob(filepath.Join(home, "runs", "*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("%d record(s) in %s (%v); the reference the person was given "+
			"retrieves exactly one attempt", len(paths), home, err)
	}
	b, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatalf("the record is not readable by another program: %v\n%s", err, b)
	}
	return rec
}

// TestTypingADemonstratedNameReachesTheRealRunner.
//
// The whole hop, in one test: the door, the argument list, a real child process built from this
// tree, the transcript coming back, the reference read out of it, the run's own word translated
// into Marco's — and a stop, said somewhere else entirely, ending it.
func TestTypingADemonstratedNameReachesTheRealRunner(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	aHomeHolding(t, aPixelDemonstration("ark graphics live"))
	// AND THE HOME THE DEMONSTRATION WENT INTO IS THE ONE THE RUNNER WILL READ.
	// `aHomeHolding` makes its own temporary home, so take the one it set.
	home = directorDir()
	t.Setenv("DIRECTOR_BIN", theRunnerBuiltFromThisTree(t))

	// THE TRANSCRIPT, CAPTURED WHERE A FRONT END WOULD READ IT.
	seen := &watchedBuffer{}
	wasOut := runnerTranscript
	t.Cleanup(func() { runnerTranscript = wasOut })
	runnerTranscript = seen

	// AND THE WORD "STOP", FROM A THIRD PLACE, ONCE THE RUN IS ASKING.
	//
	// This is what a person typing `stop` into the overlay does, through the one
	// definition both sides share. It waits for the prompt rather than for a duration,
	// and not for tidiness: `stopsignal.Watch` reads its baseline when the run starts, so
	// a stop raised before the child got that far would be ADOPTED as the baseline and
	// never heard. The prompt is printed after the watch is installed, which makes the
	// order a fact rather than a hope.
	go func() {
		if !seen.waitFor("press "+navsource.ConsentKeys[navsource.ConsentApprove],
			20*time.Second) {
			t.Error("the run never asked to be started, so there was nothing " +
				"to stop")
			return
		}
		if err := stopsignal.Raise(home); err != nil {
			t.Errorf("the stop could not be published: %v", err)
		}
	}()

	d := depsForIntake(t)
	began := time.Now()
	got, err := runInvocation(d, invoke.Request{
		Text: "ark graphics live", Source: invoke.SourceTyped})
	took := time.Since(began)
	if err != nil {
		t.Fatalf("the invocation returned an error: %v\n%s", err, seen.String())
	}

	// 1. IT WAS STOPPED, AND THE WORD FOR THAT IS MARCO'S OWN.
	if got != OutcomeCancelled {
		t.Errorf("the outcome is %q, want %q — the run was stopped, and a run "+
			"somebody stopped is not a run that failed\n%s",
			got, OutcomeCancelled, seen.String())
	}
	if took > 30*time.Second {
		t.Errorf("the run took %s to hear a stop", took)
	}

	// 2. THE TRANSCRIPT CAME BACK WHOLE — the reference a person pastes into a report,
	// and the prompt that says the run is waiting on a key. This is what the HUD's tail
	// shows.
	out := seen.String()
	for _, want := range []string{"run ", "press " + navsource.ConsentKeys[navsource.ConsentApprove]} {
		if !strings.Contains(out, want) {
			t.Errorf("the transcript does not carry %q:\n%s", want, out)
		}
	}
	// AND THE RUNNER UNDERSTOOD EVERY FLAG IT WAS GIVEN.
	//
	// This is the drift a fake spawn cannot see and the reason this test builds a real
	// one. `perform-workflow` parses with the error ignored, so a renamed flag does not
	// refuse the run — it carries on with the flag unset, which means a front end asking
	// for `--live` and `--hotkeys` would get a dry run that answers on a terminal nobody
	// has, and every assertion above would still pass.
	if strings.Contains(out, "not defined") || strings.Contains(out, "usage:") {
		t.Errorf("the runner did not understand what the front end asked for:\n%s", out)
	}

	// 3. AND THE RUN'S OWN RECORD AGREES, read as JSON by this process.
	rec := aRunRecordIn(t, home)
	if rec.Outcome != "cancelled" {
		t.Errorf("the record says %q", rec.Outcome)
	}
	if rec.Name != "ark graphics live" {
		t.Errorf("the record names the play %q", rec.Name)
	}
	// NOTHING WAS SENT, which is the one assertion that makes this test safe to run on
	// a desktop at all: no action was authorised, so none was performed.
	if rec.Sent != 0 {
		t.Fatalf("%d action(s) were sent by a run nobody authorised", rec.Sent)
	}
	// WHAT `live` IN THE RECORD IS NOT.
	//
	// It is `rep.Real` — whether a program actually reached a computer — and not whether
	// `--live` was asked for. On a run that sent nothing it is false, and that is correct:
	// the record describes what happened, not what was requested. Which flags were
	// requested is asserted where they are built, in
	// TestADemonstrationIsPerformedWithTheFlagsItNeeds, and that they were UNDERSTOOD is
	// the check above.
	if rec.Sent == 0 && rec.Live {
		t.Error("the record claims real input on a run that sent nothing")
	}
}

// watchedBuffer is a transcript another goroutine may read while it is being written.
//
// A bare bytes.Buffer is not that, and the race detector is right to say so: this test has to
// act on something the run has said while the run is still saying things.
type watchedBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (w *watchedBuffer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *watchedBuffer) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

// waitFor blocks until the transcript contains a line of text, or the patience runs out.
func (w *watchedBuffer) waitFor(text string, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if strings.Contains(w.String(), text) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// AND WHEN THE RUNNER DOES NOT UNDERSTAND SOMETHING, THE PERSON IS TOLD.
//
// The runner says a few things on stderr and every one of them is part of the account of what
// happened: a provider that is not installed, a record that could not be written, a flag it does
// not know. A front end that captured only stdout would show the half of the transcript that says
// what happened and lose the half that says why.
//
// It is also what gives the flag check in the test above its teeth. `perform-workflow` parses
// with the error ignored, so a renamed flag is not a refusal — it is a run that carries on with
// the flag unset, and the complaint about it goes to stderr. Asserting the absence of that
// complaint means nothing unless this proves its presence can be seen.
func TestWhatTheRunnerSaysOnStandardErrorReachesThePerson(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	t.Setenv("DIRECTOR_BIN", theRunnerBuiltFromThisTree(t))
	var seen watchedBuffer
	// A FLAG NO BUILD HAS EVER HAD, which is the shape of the drift this is about.
	if _, err := runDirectorCommand([]string{"perform-workflow",
		"--name", "nothing saved under this", "--notaflag"}, &seen); err != nil {
		t.Fatalf("the runner could not be run at all: %v", err)
	}
	if got := seen.String(); !strings.Contains(got, "not defined") {
		t.Errorf("the runner's complaint about a flag it does not know did not "+
			"reach the person:\n%s", got)
	}
}
