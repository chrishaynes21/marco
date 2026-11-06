//go:build cgo && vosk

package main

// THE REAL MODEL, both recognisers, on speech synthesised by Windows (System.Speech) — no
// microphone, nobody's voice. Behind -tags vosk because the model and libvosk are not in this
// repository (setup.ps1 -Voice puts them in plugins/voice). Windows only.
//
//	CGO_ENABLED=1 CGO_CFLAGS=-I. CGO_LDFLAGS="-L. -lvosk" go test -tags vosk .

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	vosk "github.com/alphacep/vosk-api/go"
)

func synthesise(t *testing.T, dir string, phrases []string) []string {
	t.Helper()
	var ps strings.Builder
	ps.WriteString("Add-Type -AssemblyName System.Speech; ")
	ps.WriteString("$f = New-Object System.Speech.AudioFormat.SpeechAudioFormatInfo(16000, [System.Speech.AudioFormat.AudioBitsPerSample]::Sixteen, [System.Speech.AudioFormat.AudioChannel]::Mono); ")
	var files []string
	for i, p := range phrases {
		f := filepath.Join(dir, "p"+string(rune('a'+i))+".wav")
		files = append(files, f)
		ps.WriteString("$v = New-Object System.Speech.Synthesis.SpeechSynthesizer; $v.SetOutputToWaveFile('" + f + "', $f); $v.Speak('" + strings.ReplaceAll(p, "'", "''") + "'); $v.Dispose(); ")
	}
	if out, err := exec.Command("powershell", "-NoProfile", "-Command", ps.String()).CombinedOutput(); err != nil {
		t.Fatalf("synthesising speech: %v\n%s", err, out)
	}
	return files
}

func decodeFile(t *testing.T, m *vosk.VoskModel, phrases []string, file string) []string {
	t.Helper()
	open, err := vosk.NewRecognizer(m, sampleRate)
	if err != nil {
		t.Fatal(err)
	}
	defer open.Free()
	open.SetWords(1)
	g, set := grammarOf(phrases, func(s string) bool { return m.FindWord(s) >= 0 })
	list, err := vosk.NewRecognizerGrm(m, sampleRate, g)
	if err != nil {
		t.Fatal(err)
	}
	defer list.Free()
	list.SetWords(1)
	var finals []string
	d := &decoder{open: open, list: list, set: set, send: func(ev, s string) {
		if ev == "Final" {
			finals = append(finals, s)
		}
	}}
	wav, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	pcm := wav[44:]
	for i := 0; i < len(pcm); i += 3200 {
		d.accept(pcm[i:min(i+3200, len(pcm))])
	}
	d.flush()
	return finals
}

func TestTheRealModelCorrectsARoutineNameAndKeepsTheRest(t *testing.T) {
	m, err := vosk.NewModel("model")
	if err != nil {
		t.Fatalf("no model in plugins/voice/model (setup.ps1 -Voice): %v", err)
	}
	defer m.Free()
	vosk.SetLogLevel(-1)
	said := []string{"cloud pull stone", "cloud pull metal", "open social", "open the settings menu please",
		"computer cloud pull black pearl"}
	files := synthesise(t, t.TempDir(), said)
	list := []string{"computer", "cloud pull", "open social", "stop", "cancel", "why didn't that work"}
	for i, f := range files {
		with := strings.Join(decodeFile(t, m, list, f), " | ")
		without := strings.Join(decodeFile(t, m, nil, f), " | ")
		t.Logf("%q: with the list %q, without %q", said[i], with, without)
		// A VALUE IS NOT IN THE LIST, so the open recogniser's word stands: "metal" is "medal" from
		// the small model with or without it. The name must be right, and nothing the list does not
		// hold may change.
		ok := with == said[i]
		if said[i] == "cloud pull metal" {
			ok = strings.HasPrefix(with, "cloud pull ")
		}
		if !ok {
			t.Errorf("%q was heard as %q with the phrase list", said[i], with)
		}
	}
}
