package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
)

// NOTHING HEARD REACHES THE LOG (quiet.go). A spoken line carrying a unique value goes through
// each place the widget writes about a request — the intake, the child's own echo of it (the
// error path: Director's "heard:", a refusal quoting the value), the run's ending, a resolve that
// could not be asked, a learn that failed — and neither the phrase nor the value is in the log.
// The log still says what happened: the shape of the request, the argument's name, the outcome.
//
// Logging any of them raw again must fail this.
func TestNothingHeardReachesTheLog(t *testing.T) {
	const value = "zq7stone9f3k"
	phrase := "cloud pull " + value
	var logged strings.Builder
	var mu sync.Mutex
	prevW, prevLvl := mlogW, mlogLvl
	mlogInit()
	mlogW, mlogLvl = lockedWriter{&mu, &logged}, mlogDebug
	t.Cleanup(func() { mlogW, mlogLvl = prevW, prevLvl })
	log := func() string { mu.Lock(); defer mu.Unlock(); return logged.String() }

	// The intake.
	done := make(chan struct{}, 1)
	restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
		done <- struct{}{}
		return childRun{result: "performed", route: "cloud pull"}
	})
	dispatchIntake(newModel(), phrase, invoke.SourceSpoken)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the intake never ran")
	}
	restore()

	// The child's echo, and the run's ending with no route announced. The test binary is the
	// child; the phrase is its first bare word, so the flags after it are not parsed.
	t.Setenv("MARCO_BIN", os.Args[0])
	t.Setenv("MARCO_OVERLAY_ECHO", "heard: "+phrase+"\nrefused: resource "+value+
		" is not one of the values this routine takes\n[result] refused\n")
	runRecordTracked(newModel(), phrase, false,
		"-test.run=TestOverlayEchoHelper", phrase, "--arg=resource="+value)

	// A resolve and a learn that cannot start.
	t.Setenv("MARCO_BIN", filepath.Join(t.TempDir(), "no-such-marco.exe"))
	t.Setenv("MARCO_OVERLAY_ECHO", "")
	runResolveChild(phrase)
	startLearn(newModel(), phrase)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(log(), "learn failed") {
		time.Sleep(10 * time.Millisecond)
	}

	out := log()
	for _, said := range []string{value, phrase} {
		if strings.Contains(strings.ToLower(out), strings.ToLower(said)) {
			t.Errorf("%q reached the log:\n%s", said, out)
		}
	}
	for _, want := range []string{"overlay - intake", "phrase=‹3 words›", "--source=spoken",
		"heard: ‹3 words›", "refused: resource ‹1 words›", "outcome=refused",
		"resolve said nothing", "learn failed"} {
		if !strings.Contains(out, want) {
			t.Errorf("the log does not say %q:\n%s", want, out)
		}
	}
}

// THE RECOGNISER ITSELF LOGS NO TRANSCRIPT. voice.exe's own log is on its stderr — the console
// of anything that pipes it, and in the widget a stream read only for its last error line. Its
// demo says "hello there" unaddressed and then "say hello" after the wake word, and at debug
// neither is in its log; the command still reaches stdout. Built without cgo, as the demo.
//
// Logging the phrase in plugins/voice again must fail this.
func TestTheRecogniserLogsNoTranscript(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "voice.exe")
	build := exec.Command("go", "build", "-o", exe, ".")
	build.Dir = filepath.Join("..", "voice")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building plugins/voice: %v\n%s", err, out)
	}
	run := exec.Command(exe, "--demo")
	run.Env = append(os.Environ(), "MARCO_LOG=debug", "MARCO_VOICE_WAKE=computer")
	var stderr strings.Builder
	run.Stderr = &stderr
	stdout, err := run.Output()
	if err != nil {
		t.Fatalf("voice --demo: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(string(stdout), `"event":"Final","data":"say hello"`) {
		t.Fatalf("the demo's command never reached stdout:\n%s", stdout)
	}
	log := stderr.String()
	if !strings.Contains(log, "dispatching command") || !strings.Contains(log, "words=2") {
		t.Errorf("the recogniser's log does not say it dispatched a command:\n%s", log)
	}
	if strings.Contains(log, "hello") {
		t.Errorf("what was heard is in the recogniser's log:\n%s", log)
	}
}

// What a child's argument list looks like in the log, and what its echo loses.
func TestTheLogKeepsWhatRanAndLosesWhatWasSaid(t *testing.T) {
	got := argsShown([]string{"do", "--source=typed", "--play=cloud-pull", "--app=game-b",
		"--focus", "--expect=v1", "--arg=resource=stone", "cloud pull stone"})
	want := "[do --source=typed --play=cloud-pull --app=game-b --focus --expect=v1 --arg=resource=… ‹3 words›]"
	if got != want {
		t.Errorf("argsShown:\n got %s\nwant %s", got, want)
	}
	r := redactionFor([]string{"do", "--arg=resource=stone", "Cloud Pull Stone"})
	if s := r.apply("heard: cloud pull stone — stone? Stones are elsewhere"); s !=
		"heard: ‹3 words› — ‹1 words›? Stones are elsewhere" {
		t.Errorf("redacted: %q", s)
	}
	// THE NARRATION A SESSION FORWARDS IS WHAT ITS ECHO LOSES: writeVoicePhrase is the one way a
	// narrated phrase reaches the child, so it is where the redaction learns it.
	narrationQuiet.reset("open my bank")
	writeVoicePhrase("type zq7secret now")
	if s := narrationQuiet.apply(`typed "zq7secret now" into open my bank`); strings.Contains(s, "zq7secret") ||
		strings.Contains(s, "open my bank") {
		t.Errorf("a forwarded narration phrase is still in its echo: %q", s)
	}
	narrationQuiet.reset()
	var g growingRedaction
	g.reset("my login")
	g.add("type hunter2 now")
	if s := g.apply(`step 2: typed "hunter2 now" into my login`); strings.Contains(s, "hunter2") ||
		strings.Contains(s, "my login") {
		t.Errorf("narration echo kept what was said: %q", s)
	}
}
