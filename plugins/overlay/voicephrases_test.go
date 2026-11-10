package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/skills"
)

// THE RECOGNISER IS TOLD THE ROUTINE NAMES (ADR-323). Started, voice.exe is pointed at a list that
// already holds them, the wake phrase, stop, the widget's own words and the "why" questions — and
// a routine learned while it runs reaches the list without a restart.
//
// Not passing the path, not writing the list before the start, or not keeping it current must
// fail this.
func TestTheRecogniserIsToldTheRoutineNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phrases.json")
	prevPath := voicePhrasesPath
	voicePhrasesPath = func() string { return path }
	t.Cleanup(func() { voicePhrasesPath = prevPath })
	t.Setenv("MARCO_VOICE_WAKE", "computer")
	names := []string{"Cloud Pull", "open social"}
	prevNames, prevEvery := voiceRoutineNames, voicePhrasesEvery
	voiceRoutineNames = func() []string { return names }
	voicePhrasesEvery = 20 * time.Millisecond
	t.Cleanup(func() { voiceRoutineNames, voicePhrasesEvery = prevNames, prevEvery })
	windows := []skills.Window{{Image: "discord", Title: "#raids | Clan - Discord"},
		{Image: "signal", Title: "Alex Rivera"}}
	prevWindows := voiceOpenWindows
	voiceOpenWindows = func() []skills.Window { return windows }
	t.Cleanup(func() { voiceOpenWindows = prevWindows })

	envOf := standInVoice(t, "", 0)
	read := func() []string {
		var ps []string
		b, _ := os.ReadFile(path)
		_ = json.Unmarshal(b, &ps)
		return ps
	}
	var atStart []string
	prevStart := startVoiceProcess
	startVoiceProcess = func(cmd *exec.Cmd) error { atStart = read(); return prevStart(cmd) }
	t.Cleanup(func() { startVoiceProcess = prevStart })

	done := make(chan struct{})
	go func() { startVoice(newModel(), func(event) {}); close(done) }()
	time.Sleep(60 * time.Millisecond)
	names = append(names, "enter freeplay")                                  // learned while it listens
	windows = append(windows, skills.Window{Image: "steam", Title: "Steam"}) // opened while it listens
	time.Sleep(120 * time.Millisecond)
	later := read()
	<-done

	if !strings.Contains(strings.Join(envOf(), "\n"), "MARCO_VOICE_PHRASES="+path) {
		t.Error("voice.exe was not pointed at the phrase list")
	}
	has := func(ps []string, p string) bool {
		for _, x := range ps {
			if x == p {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"cloud pull", "open social", "computer", "stop", "why didn't that work", "learn"} {
		if !has(atStart, want) {
			t.Errorf("the list at the start has no %q: %q", want, atStart)
		}
	}
	if !has(later, "enter freeplay") {
		t.Errorf("a play learned while voice listens did not reach the list: %q", later)
	}
	// ADR-344: the words of a focus and the open programs' own names, and no person out of a title.
	for _, want := range []string{"focus", "switch to", "discord", "signal"} {
		if !has(atStart, want) {
			t.Errorf("the list at the start has no %q: %q", want, atStart)
		}
	}
	if has(atStart, "alex rivera") {
		t.Errorf("a window title that is not an application's name reached the list: %q", atStart)
	}
	if !has(later, "steam") {
		t.Errorf("a program opened while voice listens did not reach the list: %q", later)
	}
}
