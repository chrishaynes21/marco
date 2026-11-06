package main

// WHAT THE RECOGNISER IS TOLD TO EXPECT.
//
// voice.exe runs a second recogniser restricted to a phrase list, and corrects what the open one
// heard wherever a whole listed phrase was said (plugins/voice/phrases.go): "cloud poll stone"
// becomes "cloud pull stone", and a value or a free request keeps the open recogniser's words.
// This writes that list — the routine names, the widget's own words, the wake phrase, the stop
// phrases and the "why" questions — and keeps it current as routines are learned, renamed or
// forgotten. voice.exe re-reads it between phrases when it changes.
//
// Names only: nothing anybody said is in it. See [[ADR-323-the-recogniser-is-told-the-routine-names]].

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/lastattempt"
)

// voicePhrasesEvery is how often the list is rebuilt: one `marco routes` start.
var voicePhrasesEvery = 15 * time.Second

// voiceRoutineNames is where the routine names come from. A seam, so a test names its own.
var voiceRoutineNames = listRoutes

// voicePhrasesPath is the file voice.exe is pointed at, through $MARCO_VOICE_PHRASES in ITS
// environment. A variable so a test points it somewhere of its own without putting it in the
// widget's environment, where the child would inherit it whether or not it was passed.
var voicePhrasesPath = func() string {
	if p := strings.TrimSpace(os.Getenv("MARCO_VOICE_PHRASES")); p != "" {
		return p
	}
	return filepath.Join(os.TempDir(), "marco-voice-phrases.json")
}

// voicePhrases is the whole list, sorted and without repeats.
func voicePhrases(wake string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	add(wake)
	for _, n := range voiceRoutineNames() {
		add(n)
	}
	for w := range byWord {
		add(w)
	}
	for _, p := range intent.ControlPhrases() {
		add(p)
	}
	for _, p := range lastattempt.Phrasings() {
		add(p)
	}
	sort.Strings(out)
	return out
}

// writeVoicePhrases writes the list when it differs from what is there, atomically, so voice.exe
// never reads half of one. Reports whether it wrote.
func writeVoicePhrases(path, wake string) bool {
	b, err := json.Marshal(voicePhrases(wake))
	if err != nil {
		return false
	}
	if old, err := os.ReadFile(path); err == nil && string(old) == string(b) {
		return false
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		mlogW2("overlay: the voice phrase list could not be written", "err", err)
		return false
	}
	if err := os.Rename(tmp, path); err != nil {
		mlogW2("overlay: the voice phrase list could not be written", "err", err)
		_ = os.Remove(tmp)
		return false
	}
	mlogI("overlay: voice phrase list written", "entries", strings.Count(string(b), ",")+1)
	return true
}

// keepVoicePhrases rewrites the list every voicePhrasesEvery until stop is closed.
func keepVoicePhrases(path, wake string, stop <-chan struct{}) {
	t := time.NewTicker(voicePhrasesEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			writeVoicePhrases(path, wake)
		}
	}
}
