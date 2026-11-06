package main

// VOICE IN THE WIDGET, from a TRANSCRIPT FIXTURE (voice.go).
//
// testdata/voice-transcript.jsonl is what voice.exe writes on its stdout after its own wake-word
// gating: heartbeats, partials, finished phrases. A stand-in process replays it, so the real
// start, the real pipe and the real forwarder run; nothing opens a microphone and nothing runs
// marco.exe (on this surface that performs real input). That the engine turns a forwarded Final
// into RunVoice is internal/driver's TestTheWidgetsOwnVoiceEventsReachRunVoice, over the real
// overlay.marco.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
)

const voiceFixture = "testdata/voice-transcript.jsonl"

// standInVoice makes startVoice start this test binary as voice.exe, replaying the fixture and
// ending with stderr and an exit code, and returns the environment the real one would have had.
func standInVoice(t *testing.T, stderr string, code int) func() []string {
	t.Helper()
	dir := t.TempDir()
	exe := filepath.Join(dir, "voice.exe")
	if err := os.WriteFile(exe, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "model"), 0o755); err != nil {
		t.Fatal(err)
	}
	fixture, _ := filepath.Abs(voiceFixture)
	t.Setenv("MARCO_VOICE_BIN", exe)
	t.Setenv("MARCO_VOSK_MODEL", "")
	t.Setenv("MARCO_NO_VOICE", "")
	var mu sync.Mutex
	var env []string
	prev, settle := startVoiceProcess, voiceSettle
	voiceSettle = 30 * time.Millisecond
	startVoiceProcess = func(cmd *exec.Cmd) error {
		mu.Lock()
		env = append([]string{"ARGS=" + strings.Join(cmd.Args[1:], " ")}, cmd.Env...)
		mu.Unlock()
		cmd.Path = os.Args[0]
		cmd.Args = []string{os.Args[0], "-test.run=TestVoiceStandInHelper"}
		cmd.Env = append(cmd.Env, "MARCO_VOICE_STANDIN="+fixture,
			"MARCO_VOICE_STANDIN_STDERR="+stderr, fmt.Sprintf("MARCO_VOICE_STANDIN_CODE=%d", code))
		return cmd.Start()
	}
	t.Cleanup(func() {
		startVoiceProcess, voiceSettle = prev, settle
		setVoicePhase(nil, voiceNotStarted, "", "")
	})
	return func() []string { mu.Lock(); defer mu.Unlock(); return env }
}

// TestVoiceStandInHelper is not a test. It is the body of the stand-in voice.exe: it writes the
// fixture as the recogniser would, pausing so the widget sees it listening, then ends.
func TestVoiceStandInHelper(t *testing.T) {
	path := os.Getenv("MARCO_VOICE_STANDIN")
	if path == "" {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		os.Exit(9)
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fmt.Println(sc.Text())
	}
	if os.Getenv("MARCO_VOICE_STANDIN_HOLD") != "" {
		time.Sleep(60 * time.Second) // a recogniser somebody else started, still listening
	}
	time.Sleep(150 * time.Millisecond)
	if s := os.Getenv("MARCO_VOICE_STANDIN_STDERR"); s != "" {
		fmt.Fprintln(os.Stderr, "[12:00:00][INFO] voice - dispatching command phrase=cloud pull stone")
		fmt.Fprintln(os.Stderr, s)
	}
	var code int
	fmt.Sscan(os.Getenv("MARCO_VOICE_STANDIN_CODE"), &code)
	os.Exit(code)
}

// The widget starts its own recogniser and forwards exactly its Voice partials and finals, in
// order, with the wake phrase it was told; it says it is listening while it runs, and why it
// stopped when it ends.
//
// Not starting it (the launcher-only pipe this replaced) or forwarding a heartbeat must fail this.
func TestTheWidgetStartsItsOwnMicrophone(t *testing.T) {
	envOf := standInVoice(t, "voice: open mic: no capture device", 1)
	// The wake phrase comes from the widget's own setting, as overlay.cmd used to read it with
	// `marco wake`; nothing in this environment names it, so only the overlay can pass it on.
	settings := filepath.Join(t.TempDir(), "overlay.json")
	if err := os.WriteFile(settings, []byte(`{"wake":"computer"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MARCO_OVERLAY_CONFIG", settings)
	t.Setenv("MARCO_VOICE_WAKE", "")
	voiceEnabled.Store(true)
	h := newModel()
	var mu sync.Mutex
	var got []string
	listening := false
	done := make(chan struct{})
	go func() {
		startVoice(h, func(e event) {
			mu.Lock()
			got = append(got, e.Feed+" "+e.Event+" "+fmt.Sprint(e.Data))
			mu.Unlock()
		})
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !listening {
		listening = strings.HasPrefix(voiceSays(), "voice listening")
		time.Sleep(5 * time.Millisecond)
	}
	if !listening {
		t.Errorf("a running recogniser was never said to be listening: %q", voiceSays())
	}
	<-done
	want := []string{"Voice Partial listening...", "Voice Partial open", "Voice Final open social",
		"Voice Partial listening...", "Voice Partial bring up the", "Voice Final bring up the social menu",
		"Voice Partial listening...", "Voice Final cloud pull stone",
		"Voice Partial listening...", "Voice Final pull some stone from the cloud"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("forwarded:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	env := strings.Join(envOf(), "\n")
	if !strings.Contains(env, "MARCO_VOICE_WAKE=computer") || !strings.Contains(env, "ARGS=--model ") {
		t.Errorf("the recogniser was not given its wake phrase and model:\n%s", env)
	}
	if s := voiceSays(); !strings.HasPrefix(s, "voice unavailable: no microphone could be opened") {
		t.Errorf("an ended recogniser says %q", s)
	}
	if s := voiceSays(); strings.Contains(s, "cloud pull stone") {
		t.Errorf("what was said reached the voice sentence: %q", s)
	}
	if v := voiceShort(true); v != "on · unavailable" {
		t.Errorf("the settings line reads %q", v)
	}
}

// Each way voice cannot start is said, in a person's words, without starting anything.
func TestVoiceSaysWhyItCannotListen(t *testing.T) {
	t.Cleanup(func() { setVoicePhase(nil, voiceNotStarted, "", "") })
	started := false
	prev := startVoiceProcess
	startVoiceProcess = func(*exec.Cmd) error { started = true; return nil }
	t.Cleanup(func() { startVoiceProcess = prev })

	dir := t.TempDir()
	marco := filepath.Join(dir, "marco.exe")
	os.WriteFile(marco, nil, 0o755)
	t.Setenv("MARCO_BIN", marco)
	t.Setenv("MARCO_VOICE_BIN", "")
	t.Setenv("MARCO_VOSK_MODEL", "")

	t.Setenv("MARCO_NO_VOICE", "1")
	startVoice(newModel(), func(event) {})
	if s := voiceSays(); !strings.Contains(s, "-NoVoice") {
		t.Errorf("voice left out says %q", s)
	}
	t.Setenv("MARCO_NO_VOICE", "")
	startVoice(newModel(), func(event) {})
	if s := voiceSays(); !strings.Contains(s, "voice isn't installed") ||
		!strings.Contains(s, filepath.Join(dir, "plugins", "voice", "voice.exe")) {
		t.Errorf("no voice.exe beside marco says %q", s)
	}
	os.MkdirAll(filepath.Join(dir, "plugins", "voice"), 0o755)
	os.WriteFile(filepath.Join(dir, "plugins", "voice", "voice.exe"), nil, 0o755)
	startVoice(newModel(), func(event) {})
	if s := voiceSays(); !strings.Contains(s, "no speech model at") {
		t.Errorf("no model says %q", s)
	}
	if started {
		t.Error("a recogniser was started without one to start")
	}
	for said, want := range map[string]string{
		"voice: built without cgo — rebuild":                                            "built without the recogniser",
		"voice: load Vosk model \"m\": boom":                                            "speech model could not be loaded",
		"ERROR (VoskAPI:Model():model.cc:122) Folder 'm' does not contain model files.": "speech model could not be loaded",
		"": "voice stopped",
	} {
		if got := voiceEndedWhy(nil, said); !strings.Contains(got, want) {
			t.Errorf("%q ended as %q, want %q", said, got, want)
		}
	}
	tail := &lastLine{}
	fmt.Fprintln(tail, "voice: open mic: no device")
	fmt.Fprintln(tail, "[12:00:00][INFO] voice - dispatching command phrase=open my bank")
	if s := tail.String(); s != "voice: open mic: no device" {
		t.Errorf("a log line quoting speech was kept as the reason: %q", s)
	}
	// AND LIBVOSK'S OWN LINE, which it prints before crashing on a model it cannot read.
	vosk := &lastLine{}
	fmt.Fprintln(vosk, "ERROR (VoskAPI:Model():model.cc:122) Folder 'm' does not contain model files.")
	fmt.Fprintln(vosk, "Exception 0xc0000005 0x1 0x8")
	if got := voiceEndedWhy(nil, vosk.String()); !strings.Contains(got, "speech model could not be loaded") {
		t.Errorf("a model libvosk cannot read ended as %q", got)
	}
}

// A muted widget says it is muted, and how to unmute, whatever the microphone is doing.
func TestVoiceSaysWhenItIsMuted(t *testing.T) {
	t.Cleanup(func() { setVoicePhase(nil, voiceNotStarted, "", ""); voiceEnabled.Store(true) })
	setVoicePhase(nil, voiceListening, "", "marco")
	voiceEnabled.Store(false)
	if s := voiceSays(); !strings.HasPrefix(s, "voice off") || !strings.Contains(s, "voice on") {
		t.Errorf("muted says %q", s)
	}
	if v := voiceShort(false); v != "off" {
		t.Errorf("muted settings line %q", v)
	}
	voiceEnabled.Store(true)
	if s := voiceSays(); s != `voice listening — say "marco", then the command` {
		t.Errorf("listening says %q", s)
	}
}

// spokenFinals is every finished phrase in the fixture, in order.
func spokenFinals(t *testing.T) []string {
	t.Helper()
	f, err := os.Open(voiceFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	forwardVoice(f, func(e event) {
		if e.Event == "Final" {
			out = append(out, fmt.Sprint(e.Data))
		}
	})
	return out
}

// THE THREE SPOKEN REQUESTS, through the one intake, from the fixture: an exact name runs as
// words, a paraphrase is offered the routine Director named, and a line with a value — said, or
// proposed by Director from a paraphrase — opens the same review a typed one gets, and runs
// nothing until Enter.
//
// Giving voice its own parser, or sending a spoken line with a value straight to the intake, must
// fail this.
func TestASpokenLineWithAValueIsReviewedLikeATypedOne(t *testing.T) {
	finals := spokenFinals(t)
	if len(finals) != 4 {
		t.Fatalf("the fixture's finals: %q", finals)
	}
	exact, paraphrase, withValue, proposal := finals[0], finals[1], finals[2], finals[3]
	pull := aCloudPull(withValue, "stone")
	proposed := aCloudPull("cloud pull resource:stone", "stone")
	proposed.Parts[0].Said = "(read out of what you said)"
	engineReads(t, map[string]lineReading{
		exact:         {Line: exact, Kind: "routine", Slug: "open-social", Name: "open social", Runnable: true},
		paraphrase:    {Line: paraphrase, Kind: "none"},
		withValue:     pull,
		proposal:      {Line: proposal, Kind: "none"},
		proposed.Line: proposed,
	})
	var mu sync.Mutex
	var runs []string
	restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
		mu.Lock()
		runs = append(runs, strings.Join(args, " "))
		mu.Unlock()
		if strings.HasPrefix(args[len(args)-1], "--") || args[len(args)-1] == exact {
			return childRun{result: "performed", route: "open social"}
		}
		return childRun{result: string(outcome.Unavailable)} // nothing took the words
	})
	defer restore()
	ran := func() []string {
		time.Sleep(80 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), runs...)
	}
	resetRuns := func() { mu.Lock(); runs = nil; mu.Unlock() }
	voiceEnabled.Store(true)
	spoken := func(h *model, phrase string) {
		dispatch(h, request{Action: "RunVoice", Input: phrase}) // what overlay.marco's Final reaches
	}
	t.Cleanup(func() { promptAsk.Store(false); takePendingRoutine(); takePendingLearn() })

	t.Run("an exact name", func(t *testing.T) {
		resetRuns()
		spoken(newModel(), exact)
		if got := ran(); len(got) != 1 || got[0] != "do --source="+string(invoke.SourceSpoken)+" "+exact {
			t.Errorf("a spoken exact name ran %q", got)
		}
	})

	t.Run("a paraphrase", func(t *testing.T) {
		resetRuns()
		var r resolved
		r.Answer.Kind, r.Answer.ID, r.Answer.Slug, r.Answer.Name = "run", "open-social", "open-social", "open social"
		answering(t, r, "")
		h := newModel()
		spoken(h, paraphrase)
		got := ran()
		if len(got) != 1 || got[0] != "do --source="+string(invoke.SourceSpoken)+" "+paraphrase {
			t.Errorf("a spoken paraphrase reached the intake as %q", got)
		}
		if !promptAsk.Load() {
			t.Fatal("the routine Director named was not offered")
		}
		if o := takePendingRoutine(); o.slug != "open-social" {
			t.Errorf("offered %+v", o)
		}
		promptAsk.Store(false)
	})

	t.Run("a value, said", func(t *testing.T) {
		resetRuns()
		h := newModel()
		spoken(h, withValue)
		if got := ran(); len(got) != 0 {
			t.Fatalf("a spoken line with a value ran unread: %q", got)
		}
		s := h.snapshot()
		if !s.editing || s.input != withValue || !s.hasReading || s.status != "Enter runs it" {
			t.Fatalf("not open for review: editing %v, input %q, reading %v, status %q",
				s.editing, s.input, s.hasReading, s.status)
		}
		time.Sleep(readDwell + 50*time.Millisecond)
		h.takeInput()
		h.setEditing(false)
		typedEnter(h, withValue)
		if got := ran(); len(got) != 1 || got[0] != "do --source=typed --play=cloud-pull --app=game-b --expect=v1 --arg=resource=stone" {
			t.Errorf("Enter on the spoken line's read-back ran %q", got)
		}
	})

	t.Run("a value, proposed from a paraphrase", func(t *testing.T) {
		resetRuns()
		var r resolved
		r.Answer.Kind, r.Answer.ID, r.Answer.App, r.Answer.Slug, r.Answer.Name =
			"run", "game-b/context/cloud-pull", "game-b", "cloud-pull", "cloud pull"
		r.Answer.Values = append(r.Answer.Values, struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}{"resource", "stone"})
		r.Answer.SourceAt = "v1"
		answering(t, r, "")
		h := newModel()
		spoken(h, proposal)
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && !h.snapshot().editing {
			time.Sleep(5 * time.Millisecond)
		}
		s := h.snapshot()
		if !s.editing || s.input != proposed.Line || promptAsk.Load() {
			t.Fatalf("the proposal is not open for review: editing %v, input %q", s.editing, s.input)
		}
		if got := ran(); len(got) != 1 {
			t.Errorf("more than the intake ran before review: %q", got)
		}
	})
}

// ONE RECOGNISER PER MICROPHONE. A voice.exe from the same place already running — what an
// overlay.cmd from an older setup.ps1 leaves, piping it into marco serve — is found in the REAL
// process table, the widget does not start a second, and it says why and which process. The image
// path is compared as Windows compares paths.
//
// Starting the widget's own recogniser regardless must fail this.
func TestTheWidgetDoesNotStartASecondRecogniser(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "voice.exe")
	self, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, self, 0o755); err != nil {
		t.Fatal(err)
	}
	os.Mkdir(filepath.Join(dir, "model"), 0o755)
	fixture, _ := filepath.Abs(voiceFixture)
	other := exec.Command(exe, "-test.run=TestVoiceStandInHelper")
	other.Env = append(os.Environ(), "MARCO_VOICE_STANDIN="+fixture, "MARCO_VOICE_STANDIN_HOLD=1")
	other.Stdout = io.Discard
	if err := other.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Process.Kill(); _ = other.Wait() })

	t.Setenv("MARCO_VOICE_BIN", exe)
	t.Setenv("MARCO_VOSK_MODEL", "")
	t.Setenv("MARCO_NO_VOICE", "")
	started := false
	prev := startVoiceProcess
	// An error, so a start that should not have happened ends here rather than waiting on a pipe
	// from a process that never ran.
	startVoiceProcess = func(*exec.Cmd) error { started = true; return errors.New("not started in this test") }
	t.Cleanup(func() { startVoiceProcess = prev; setVoicePhase(nil, voiceNotStarted, "", "") })

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := recogniserRunning(exe); ok {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	startVoice(newModel(), func(event) {})
	if started {
		t.Fatal("a second recogniser was started beside the one already listening")
	}
	s := voiceSays()
	if !strings.Contains(s, "already running") || !strings.Contains(s, fmt.Sprint(other.Process.Pid)) ||
		!strings.Contains(s, "setup.ps1") {
		t.Errorf("the widget says %q", s)
	}
	if v := voiceShort(true); v != "on · another recogniser" {
		t.Errorf("the settings line reads %q", v)
	}
	// AND NOT A NAMESAKE: a voice.exe somewhere else is not this microphone's.
	if _, ok := recogniserRunning(filepath.Join(t.TempDir(), "voice.exe")); ok {
		t.Error("a voice.exe at another path was taken for this one")
	}
}

// EVERY LAUNCHER GETS VOICE FROM THE WIDGET, AND NONE STARTS A SECOND RECOGNISER. main starts it
// (main runs the window, so this reads its source, as the installer's stack is checked), and
// neither overlay.cmd nor the setup.ps1 that writes it pipes voice.exe into marco serve any more:
// two recognisers on one microphone would run every spoken command twice.
func TestEveryLauncherGetsVoiceFromTheWidget(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	starts := false
	for _, line := range strings.Split(string(src), "\n") {
		starts = starts || strings.HasPrefix(strings.TrimSpace(line), "go startVoice(m, emit)")
	}
	if !starts {
		t.Error("the overlay's main no longer starts the recogniser")
	}
	for _, launcher := range []string{"../../overlay.cmd", "../../setup.ps1"} {
		b, err := os.ReadFile(launcher)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, "voice.exe") && strings.Contains(line, "|") &&
				!strings.HasPrefix(strings.TrimSpace(line), "#") {
				t.Errorf("%s still pipes a recogniser into marco serve: %s", launcher, strings.TrimSpace(line))
			}
		}
	}
}
