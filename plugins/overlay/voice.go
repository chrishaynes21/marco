package main

// THE WIDGET STARTS ITS OWN MICROPHONE.
//
// # The defect
//
// Voice reached the widget only through overlay.cmd, which piped `voice.exe` into `marco
// serve`'s standard input. The two other ways the widget is started — Studio's Open widget
// (internal/widgetstack) and the installed `Marco.exe --overlay` — run the same `marco serve`
// with nothing in front of it, so their widget had a voice toggle, a wake phrase setting and a
// `Voice` feed, and nothing ever spoke into it. Nothing said so either: a widget nobody pipes a
// microphone into looks exactly like one listening to a quiet room.
//
// # The fix is to move the start, not to write a second path
//
// The overlay is already a bridge that pushes feed events to the engine (Commands, Hotkeys).
// It now starts `voice.exe` itself and forwards that process's own `Voice` lines, unchanged,
// onto the same stdout, so they arrive at overlay.marco's `when Voice reads Partial/Final`
// exactly as the pipe delivered them, and a finished phrase comes back as RunVoice through the
// one intake. No phrase is read here: this file moves lines and reports whether it can.
//
// Every launcher starts the overlay, so every launcher now has voice, and the pipe is gone from
// overlay.cmd (two recognisers on one microphone would run every command twice).
//
// # It says which state it is in
//
// voiceSays is the one sentence: listening for the wake phrase, muted, starting, or unavailable
// and why — not installed, no recogniser compiled in, no model, no microphone, a library missing.
// It is logged when it changes, it is the settings panel's voice line, and it answers the
// `voice` toggle.

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
	"github.com/shirou/gopsutil/v3/process"
)

// voicePhase is where the microphone is.
type voicePhase int

const (
	voiceNotStarted voicePhase = iota
	voiceStarting
	voiceListening
	voiceUnavailable
	voiceElsewhere // another voice.exe from the same place is already listening; why holds its pid
)

var (
	voiceStMu  sync.Mutex
	voicePh    voicePhase
	voiceWhy   string // why it is unavailable, in a person's words
	voiceWake  string // the phrase it is listening for
	voiceNoted string // the last sentence logged, so a change is logged once
)

// voiceSettle is how long the recogniser must have stayed up to be called listening. Every
// failure voice.exe has — no recogniser, no model, no microphone, a missing library — ends the
// process while it loads, inside this. A var so a test need not wait.
var voiceSettle = 4 * time.Second

// startVoiceProcess starts the recogniser. A seam: a test starts a stand-in.
var startVoiceProcess = func(cmd *exec.Cmd) error { return cmd.Start() }

// voiceFound is where the recogniser and its model are, or why there is none.
type voiceFound struct {
	exe, model string
	why        string
}

// findVoice looks for the recogniser beside marco.exe, at plugins\voice\voice.exe with its model
// in plugins\voice\model — where setup.ps1 -Voice builds it in a development tree, and the same
// relative place an installation keeps the accessibility bridge (plugins\uia\uia.exe).
// $MARCO_VOICE_BIN names it outright; $MARCO_VOSK_MODEL names the model, as voice.exe reads it.
// $MARCO_NO_VOICE=1 is setup.ps1 -NoVoice: the microphone is not started at all.
func findVoice() voiceFound {
	if on := strings.TrimSpace(os.Getenv("MARCO_NO_VOICE")); on != "" && on != "0" {
		return voiceFound{why: "voice was left out when Marco was set up (-NoVoice)"}
	}
	exe := strings.TrimSpace(os.Getenv("MARCO_VOICE_BIN"))
	root := ""
	if m, err := exec.LookPath(marcoBin()); err == nil {
		if abs, err := filepath.Abs(m); err == nil {
			root = filepath.Dir(abs)
		}
	}
	if exe == "" {
		if root == "" {
			return voiceFound{why: "marco.exe was not found, so voice.exe could not be looked for beside it"}
		}
		exe = filepath.Join(root, "plugins", "voice", "voice.exe")
	}
	if _, err := os.Stat(exe); err != nil {
		// The installer ships it (plugins/marco-app `staged`, ADR-319), so an installation lands
		// here only when something removed it.
		return voiceFound{why: "voice isn't installed (no " + exe + ") — reinstall Marco, or in a " +
			"development tree run setup.ps1 -Voice"}
	}
	model := strings.TrimSpace(os.Getenv("MARCO_VOSK_MODEL"))
	if model == "" {
		model = filepath.Join(filepath.Dir(exe), "model")
	}
	if _, err := os.Stat(model); err != nil {
		return voiceFound{exe: exe, why: "no speech model at " + model + " — run setup.ps1 -Voice"}
	}
	return voiceFound{exe: exe, model: model}
}

// wakePhrase is what the recogniser listens for: $MARCO_VOICE_WAKE when a launcher set it, else
// the widget's saved setting, which is what overlay.cmd read into it before.
func wakePhrase() string {
	if w := strings.TrimSpace(os.Getenv("MARCO_VOICE_WAKE")); w != "" {
		return w
	}
	if w := strings.TrimSpace(widgetconfig.Load().Wake); w != "" {
		return w
	}
	return "marco"
}

// startVoice starts the recogniser and forwards what it hears, for the life of the overlay. It
// returns once the recogniser has ended, or at once when there is none.
func startVoice(h *model, emit func(event)) {
	found := findVoice()
	wake := wakePhrase()
	if found.why != "" {
		setVoicePhase(h, voiceUnavailable, found.why, wake)
		return
	}
	// ONE RECOGNISER PER MICROPHONE. An overlay.cmd written by an older setup.ps1 still pipes
	// this same voice.exe into marco serve, and its lines reach overlay.marco through serve's
	// stdin; a second one started here would run every spoken command twice. That pipe starts
	// voice.exe before serve has even compiled the program that starts this widget, so it is
	// already running when this asks. The widget then listens through it and says so.
	if pid, ok := recogniserRunning(found.exe); ok {
		setVoicePhase(h, voiceElsewhere, strconv.Itoa(pid), wake)
		return
	}
	// THE ROUTINE NAMES, BEFORE IT STARTS, and kept current while it runs (voicephrases.go).
	phrases := voicePhrasesPath()
	writeVoicePhrases(phrases, wake)
	stopPhrases := make(chan struct{})
	defer close(stopPhrases)
	go keepVoicePhrases(phrases, wake, stopPhrases)
	cmd := exec.Command(found.exe, "--model", found.model)
	cmd.Env = append(os.Environ(), "MARCO_VOICE_WAKE="+wake, "MARCO_VOICE_PHRASES="+phrases)
	out, err := cmd.StdoutPipe()
	if err != nil {
		setVoicePhase(h, voiceUnavailable, "voice.exe could not be connected: "+err.Error(), wake)
		return
	}
	tail := &lastLine{}
	cmd.Stderr = tail // kept in memory for the reason, never logged: it can quote speech
	if err := startVoiceProcess(cmd); err != nil {
		setVoicePhase(h, voiceUnavailable, "voice.exe could not be started: "+err.Error(), wake)
		return
	}
	setVoicePhase(h, voiceStarting, "", wake)
	settled := time.AfterFunc(voiceSettle, func() { setVoicePhaseIf(h, voiceStarting, voiceListening, wake) })
	forwardVoice(out, emit)
	err = cmd.Wait()
	settled.Stop()
	setVoicePhase(h, voiceUnavailable, voiceEndedWhy(err, tail.String()), wake)
}

// forwardVoice passes the recogniser's own Voice events on, as they are. A heartbeat or anything
// else is not the engine's business.
func forwardVoice(r io.Reader, emit func(event)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var ev struct {
			Feed  string `json:"feed"`
			Event string `json:"event"`
			Data  string `json:"data"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil || ev.Feed != "Voice" {
			continue
		}
		if ev.Event == "Partial" || ev.Event == "Final" {
			emit(event{Feed: "Voice", Event: ev.Event, Data: ev.Data})
		}
	}
}

// voiceEndedWhy is why the recogniser stopped, in a person's words.
func voiceEndedWhy(err error, said string) string {
	said = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(said), "voice:"))
	low := strings.ToLower(said)
	switch {
	case strings.Contains(low, "without cgo"):
		return "this voice.exe was built without the recogniser — run setup.ps1 -Voice with gcc installed"
	case strings.Contains(low, "vosk model"), strings.Contains(low, "does not contain model files"):
		return "the speech model could not be loaded — run setup.ps1 -Voice"
	case strings.Contains(low, "mic") || strings.Contains(low, "audio context"):
		return "no microphone could be opened (" + said + ")"
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && uint32(ee.ExitCode()) == 0xC0000135 {
		return "voice.exe could not start: a library it needs (libvosk.dll) is not beside it — run setup.ps1 -Voice"
	}
	if said != "" {
		return "voice stopped: " + said
	}
	if err != nil {
		return "voice stopped (" + err.Error() + ")"
	}
	return "voice stopped"
}

func setVoicePhaseIf(h *model, from, to voicePhase, wake string) {
	voiceStMu.Lock()
	if voicePh != from {
		voiceStMu.Unlock()
		return
	}
	voiceStMu.Unlock()
	setVoicePhase(h, to, "", wake)
}

func setVoicePhase(h *model, p voicePhase, why, wake string) {
	voiceStMu.Lock()
	voicePh, voiceWhy, voiceWake = p, why, wake
	voiceStMu.Unlock()
	noteVoice(h)
}

// noteVoice logs the voice sentence when it has changed.
func noteVoice(h *model) {
	s := voiceSays()
	voiceStMu.Lock()
	changed := s != voiceNoted
	voiceNoted = s
	voiceStMu.Unlock()
	if changed && h != nil {
		h.log(s)
	}
}

// voiceSays is the one sentence about voice: whether it is on, listening, or cannot be.
func voiceSays() string {
	voiceStMu.Lock()
	p, why, wake := voicePh, voiceWhy, voiceWake
	voiceStMu.Unlock()
	switch p {
	case voiceUnavailable:
		return "voice unavailable: " + why
	case voiceNotStarted:
		return "voice not started"
	case voiceElsewhere:
		return "voice is listening through a recogniser that was already running (voice.exe, process " +
			why + "), so the widget did not start a second on the same microphone — if an older " +
			"overlay.cmd started it, run setup.ps1 again so the widget starts its own"
	}
	if !voiceEnabled.Load() {
		return "voice off — type " + leaderShown() + "voice on to listen"
	}
	if p == voiceStarting {
		return "voice starting…"
	}
	return "voice listening — say \"" + wake + "\", then the command"
}

// voiceShort is the settings panel's voice value: the toggle, and what the microphone is doing.
func voiceShort(on bool) string {
	voiceStMu.Lock()
	p := voicePh
	voiceStMu.Unlock()
	switch {
	case p == voiceUnavailable:
		return onOff(on) + " · unavailable"
	case p == voiceElsewhere:
		return onOff(on) + " · another recogniser"
	case !on:
		return "off"
	case p == voiceStarting:
		return "on · starting"
	case p == voiceListening:
		return "on · listening"
	}
	return "on"
}

// lastLine keeps the last line voice.exe ended with — its `voice: <error>` line — bounded. Its
// log lines (`[15:04:05][INFO] voice - dispatching command phrase=…`) are on the same stream at
// the default level and quote what was said, so they are never kept.
type lastLine struct {
	mu   sync.Mutex
	buf  []byte
	last string
}

func (l *lastLine) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf = append(l.buf, p...)
	for {
		i := strings.IndexByte(string(l.buf), '\n')
		if i < 0 {
			break
		}
		if s := strings.TrimSpace(string(l.buf[:i])); isReason(s) {
			l.last = s
		}
		l.buf = l.buf[i+1:]
	}
	if len(l.buf) > 4096 {
		l.buf = l.buf[len(l.buf)-4096:]
	}
	return len(p), nil
}

func (l *lastLine) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if s := strings.TrimSpace(string(l.buf)); isReason(s) {
		return s
	}
	return l.last
}

// recogniserRunning is the pid of a voice.exe already running from exe, if there is one. A seam:
// a test answers for the process table.
var recogniserRunning = recogniserRunningFrom

// recogniserRunningFrom asks the process table for a process whose image is exe. The image, not
// the name: something else called voice.exe is not this microphone's recogniser.
func recogniserRunningFrom(exe string) (int, bool) {
	want, err := filepath.Abs(exe)
	if err != nil {
		return 0, false
	}
	pids, err := process.Pids()
	if err != nil {
		return 0, false
	}
	base := strings.ToLower(filepath.Base(want))
	for _, pid := range pids {
		if int(pid) == os.Getpid() {
			continue
		}
		p, err := process.NewProcess(pid)
		if err != nil {
			continue
		}
		if n, err := p.Name(); err != nil || strings.ToLower(n) != base {
			continue
		}
		if got, err := p.Exe(); err == nil && samePath(got, want) {
			return int(pid), true
		}
	}
	return 0, false
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// isReason is a line voice.exe or the recogniser it loads ends with when it cannot listen: its own
// `voice: <error>`, or Vosk's `ERROR (VoskAPI:…)` — which libvosk prints before it crashes on a
// model it cannot read, instead of returning the error. Neither quotes anything said.
func isReason(s string) bool {
	return strings.HasPrefix(s, "voice:") || strings.HasPrefix(s, "ERROR (VoskAPI")
}
