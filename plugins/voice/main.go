// Command voice is the overlay's voice layer: it captures the microphone, runs
// offline speech recognition (Vosk — cross-platform, no cloud), and emits the
// transcript as Marco feed events on stdout, one JSON object per line:
//
//	{"feed":"Voice","event":"Partial","data":"open ch"}   // live, as you speak
//	{"feed":"Voice","event":"Final","data":"open chrome"}  // a finished phrase
//
// Pipe it into a served program (the same event seam globals/ahk-hotkeys use):
//
//	voice --model vosk-model-small-en-us | marco serve --host Overlay=bridge:overlay plugins/overlay/overlay.marco
//
// overlay.marco turns Partial into a live preview (Overlay's Heard) and Final
// into a command (Overlay's Run).
//
// The real recognizer needs cgo + libvosk + an audio backend (see README); build
// it with CGO_ENABLED=1. Without cgo it still builds, and `--demo` emits a canned
// phrase so the pipe can be exercised end-to-end.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type event struct {
	Feed  string `json:"feed"`
	Event string `json:"event"`
	Data  string `json:"data"`
}

func main() {
	demo := flag.Bool("demo", false, "emit a canned phrase instead of using the mic (no cgo/model needed)")
	model := flag.String("model", envOr("MARCO_VOSK_MODEL", "model"), "path to the Vosk model directory")
	wake := flag.String("wake", envOr("MARCO_VOICE_WAKE", "computer"),
		"assistant wake word: say it to activate, then speak the command (\"\" or \"off\" = always listen)")
	flag.Parse()

	w := strings.ToLower(strings.TrimSpace(*wake))

	// narrateLock is a file the overlay creates while a narrate-learn session is
	// live. When it exists the voice plugin re-arms after each command phrase, so
	// the user never needs to repeat the wake word mid-narration.
	narrateLock := envOr("MARCO_NARRATE_LOCK", filepath.Join(os.TempDir(), "marco-narrate.lock"))
	inNarrateMode := func() bool {
		_, err := os.Stat(narrateLock)
		return err == nil
	}

	enc := json.NewEncoder(os.Stdout)
	var encMu sync.Mutex
	// write serializes stdout and reports a broken pipe — when serve exits (the
	// HUD's `exit`), the read end closes and this errors, our cue to shut down.
	write := func(e event) bool {
		encMu.Lock()
		defer encMu.Unlock()
		return enc.Encode(e) == nil
	}
	emit := func(ev, data string) { write(event{Feed: "Voice", Event: ev, Data: data}) }

	// Heartbeat: the mic loop only writes on speech, so without this voice would
	// never notice serve dying and would keep the `voice | marco serve` pipeline
	// (and the overlay.cmd batch) alive forever. A periodic no-op write detects
	// the closed pipe and exits. "Heartbeat" has no listener, so it's ignored.
	go func() {
		for {
			time.Sleep(time.Second)
			if !write(event{Feed: "Voice", Event: "Heartbeat"}) {
				os.Exit(0)
			}
		}
	}()

	// Two-phase activation: say the wake word ("computer") to ARM — the HUD shows
	// "listening..." — then the NEXT phrase is the command. Saying "computer <cmd>"
	// in one breath also works (one-shot). No wake gating if w is ""/"off".
	armed := false
	var armedAt time.Time
	const armWindow = 8 * time.Second

	// listening() gates whether speech is treated as a command. In narrate-learn it's
	// always on — the session stays live with no wake word and no arm-window timeout,
	// so you can pause to read/navigate a menu and keep narrating. Otherwise it's the
	// usual armed-within-the-window check.
	listening := func() bool { return inNarrateMode() || (armed && time.Since(armedAt) <= armWindow) }
	send := func(ev, data string) {
		if w == "" || w == "off" {
			emit(ev, data) // no activation: stream everything
			return
		}
		if ev == "Partial" {
			// Stay quiet until activated — only show the live transcript while
			// listening, so idle/background speech never lands in the HUD.
			if listening() {
				emit("Partial", data)
			} else {
				mlogD("voice: partial suppressed (not armed)", "data", data)
			}
			return
		}
		if ev != "Final" {
			emit(ev, data)
			return
		}
		t := strings.ToLower(strings.TrimSpace(data))
		// Armed: this phrase is the command (a leading wake word is fine too).
		if listening() {
			if inNarrateMode() {
				// Narrate-learn is active: re-arm immediately so the user doesn't
				// need to repeat the wake word between phrases. The narrate child
				// handles "done" / "cancel" and the overlay removes the lock file.
				mlogD("voice: narrate mode re-arm", "phrase", t)
				armedAt = time.Now()
				emit("Partial", "listening...")
			} else {
				armed = false
			}
			if after, ok := strings.CutPrefix(t, w); ok {
				t = strings.TrimSpace(strings.TrimLeft(after, " ,"))
			}
			if t != "" {
				mlogI("voice: dispatching command", "phrase", t)
				emit("Final", t)
			}
			return
		}
		armed = false
		// Not armed: only the wake word does anything; everything else is ignored
		// (not shown), so the HUD isn't fed random speech.
		if after, ok := strings.CutPrefix(t, w); ok {
			if cmd := strings.TrimSpace(strings.TrimLeft(after, " ,")); cmd != "" {
				mlogI("voice: one-shot command", "phrase", cmd)
				emit("Final", cmd) // one-shot: "computer login to facebook"
			} else {
				mlogI("voice: armed", "wake", w)
				armed, armedAt = true, time.Now() // just wake word → wait for the command
				emit("Partial", "listening...")
			}
		} else {
			mlogD("voice: ignored (not armed, no wake word)", "phrase", t)
		}
	}

	if *demo {
		runDemo(send)
		return
	}
	if err := runVosk(send, *model); err != nil {
		fmt.Fprintln(os.Stderr, "voice:", err)
		os.Exit(1)
	}
}

// runDemo exercises the two-phase pipe without a mic or model: wake word arms,
// then a command phrase runs.
func runDemo(send func(ev, data string)) {
	// Unaddressed speech first — must be suppressed.
	send("Partial", "hello there")
	send("Final", "hello there")
	time.Sleep(300 * time.Millisecond)
	// Wake word arms, then command.
	send("Partial", "computer")
	time.Sleep(400 * time.Millisecond)
	send("Final", "computer") // arms → "listening..."
	time.Sleep(400 * time.Millisecond)
	for _, p := range []string{"say", "say hello"} {
		send("Partial", p)
		time.Sleep(400 * time.Millisecond)
	}
	send("Final", "say hello") // runs
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
