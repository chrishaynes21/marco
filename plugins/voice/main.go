// Command voice is the overlay's voice layer: it captures the microphone, runs
// offline speech recognition (Vosk — cross-platform, no cloud), and emits the
// transcript as Marco feed events on stdout, one JSON object per line:
//
//	{"feed":"Voice","event":"Partial","data":"open ch"}   // live, as you speak
//	{"feed":"Voice","event":"Final","data":"open chrome"}  // a finished phrase
//
// Pipe it into a served program (the same event seam globals/ahk-hotkeys use):
//
//	voice --model vosk-model-small-en-us | marco serve --host Overlay=bridge:overlay programs/overlay.marco
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
	wake := flag.String("wake", envOr("MARCO_VOICE_WAKE", "marco"),
		"assistant wake word: say it to activate, then speak the command (\"\" or \"off\" = always listen)")
	flag.Parse()

	w := strings.ToLower(strings.TrimSpace(*wake))
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

	// Two-phase activation: say the wake word ("marco") to ARM — the HUD shows
	// "listening..." — then the NEXT phrase is the command. Saying "marco <cmd>"
	// in one breath also works (one-shot). No wake gating if w is ""/"off".
	armed := false
	var armedAt time.Time
	const armWindow = 8 * time.Second

	listening := func() bool { return armed && time.Since(armedAt) <= armWindow }
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
			armed = false
			if after, ok := strings.CutPrefix(t, w); ok {
				t = strings.TrimSpace(strings.TrimLeft(after, " ,"))
			}
			if t != "" {
				emit("Final", t)
			}
			return
		}
		armed = false
		// Not armed: only the wake word does anything; everything else is ignored
		// (not shown), so the HUD isn't fed random speech.
		if after, ok := strings.CutPrefix(t, w); ok {
			if cmd := strings.TrimSpace(strings.TrimLeft(after, " ,")); cmd != "" {
				emit("Final", cmd) // one-shot: "marco login to facebook"
			} else {
				armed, armedAt = true, time.Now() // just "marco" → wait for the command
				emit("Partial", "listening...")
			}
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
	// Unaddressed speech first — must be suppressed (nothing reaches the HUD).
	send("Partial", "hello there")
	send("Final", "hello there")
	time.Sleep(300 * time.Millisecond)
	// Activate, then command.
	send("Partial", "marco")
	time.Sleep(400 * time.Millisecond)
	send("Final", "marco") // arms → "listening..."
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
