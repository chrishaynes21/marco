//go:build cgo

package main

import (
	"encoding/json"
	"fmt"
	"time"

	vosk "github.com/alphacep/vosk-api/go"
	"github.com/gen2brain/malgo"
)

// endpointTimeout finalizes a stable partial when Vosk's own silence detection
// doesn't fire (some models/mics don't endpoint reliably).
const endpointTimeout = 1200 * time.Millisecond

// runVosk captures the mic (malgo / miniaudio) at 16 kHz mono and streams it
// through Vosk, emitting Partial transcripts live and a Final on each phrase end.
// Needs libvosk on the link path and a model directory (see README).
func runVosk(send func(ev, data string), modelPath string) error {
	vosk.SetLogLevel(-1)
	model, err := vosk.NewModel(modelPath)
	if err != nil {
		return fmt.Errorf("load Vosk model %q: %w (download one — see README)", modelPath, err)
	}
	defer model.Free()
	rec, err := vosk.NewRecognizer(model, float64(sampleRate))
	if err != nil {
		return fmt.Errorf("recognizer: %w", err)
	}
	defer rec.Free()

	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(string) {})
	if err != nil {
		return fmt.Errorf("audio context: %w", err)
	}
	defer ctx.Free()

	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = 1
	cfg.SampleRate = sampleRate

	frames := make(chan []byte, 32)
	onRecv := func(_, in []byte, _ uint32) {
		buf := make([]byte, len(in))
		copy(buf, in)
		select {
		case frames <- buf:
		default: // drop if recognition is briefly behind
		}
	}
	device, err := malgo.InitDevice(ctx.Context, cfg, malgo.DeviceCallbacks{Data: onRecv})
	if err != nil {
		return fmt.Errorf("open mic: %w", err)
	}
	defer device.Uninit()
	if err := device.Start(); err != nil {
		return fmt.Errorf("start mic: %w", err)
	}

	last := ""
	lastChange := time.Now()
	for buf := range frames {
		// Vosk's own end-of-utterance (silence) detection.
		if rec.AcceptWaveform(buf) != 0 {
			if t := jsonField(rec.Result(), "text"); t != "" {
				send("Final", t)
			}
			last = ""
			continue
		}
		p := jsonField(rec.PartialResult(), "partial")
		switch {
		case p != last:
			last = p
			lastChange = time.Now()
			if p != "" {
				send("Partial", p)
			}
		case p != "" && time.Since(lastChange) > endpointTimeout:
			// Vosk didn't endpoint but the words have been stable — finalize it
			// ourselves so a pause always completes the phrase.
			send("Final", p)
			rec.Result() // reset the recognizer for the next phrase
			last = ""
		}
	}
	return nil
}

const sampleRate = 16000

func jsonField(s, field string) string {
	var m map[string]any
	if json.Unmarshal([]byte(s), &m) != nil {
		return ""
	}
	v, _ := m[field].(string)
	return v
}
