//go:build cgo

package main

import (
	"encoding/json"
	"fmt"
	"os"
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
//
// TWO RECOGNISERS HEAR THE SAME AUDIO (phrases.go): the open one, which drives the live partials
// and decides where a phrase ends, and one restricted to the widget's phrase list. A Final is the
// two merged by timing. Both are created together and fed every buffer, so their word times share
// one clock; the list is changed with SetGrm, which keeps it (measured: times run on across SetGrm
// and Reset).
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
	rec.SetWords(1)

	known := func(w string) bool { return model.FindWord(w) >= 0 }
	list := &phraseFile{path: os.Getenv("MARCO_VOICE_PHRASES")}
	phrases, _ := list.changed()
	grammar, set := grammarOf(phrases, known)
	grm, err := vosk.NewRecognizerGrm(model, float64(sampleRate), grammar)
	if err != nil {
		return fmt.Errorf("phrase-list recognizer: %w", err)
	}
	defer grm.Free()
	grm.SetWords(1)
	mlogI("voice: phrase list", "entries", len(set))
	var d *decoder
	refresh := func() {
		if ps, ok := list.changed(); ok {
			g, s := grammarOf(ps, known)
			grm.SetGrm(g)
			d.set = s
			mlogI("voice: phrase list changed", "entries", len(s))
		}
	}

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

	d = &decoder{open: rec, list: grm, set: set, send: send, refresh: refresh, lastChange: time.Now()}
	for buf := range frames {
		d.accept(buf)
	}
	return nil
}

// decoder is the two recognisers hearing one stream: the microphone's, or a test's audio.
type decoder struct {
	open, list *vosk.VoskRecognizer
	// set is the phrases the list holds whole: only those correct what the open one heard.
	set     listedPhrases
	send    func(ev, data string)
	refresh func()
	// listed is what the phrase-list recogniser finished since the last Final: it can end a
	// phrase before the open one does.
	listed     []word
	last       string
	lastChange time.Time
}

func (d *decoder) final(open []word) {
	all := append(d.listed, resultWords(d.list.FinalResult())...)
	d.listed = nil
	if t := merge(open, whole(all, d.set)); t != "" {
		d.send("Final", t)
	}
	if d.refresh != nil {
		d.refresh() // between phrases, never inside one
	}
}

func (d *decoder) accept(buf []byte) {
	if d.list.AcceptWaveform(buf) != 0 {
		d.listed = append(d.listed, resultWords(d.list.Result())...)
	}
	// Vosk's own end-of-utterance (silence) detection.
	if d.open.AcceptWaveform(buf) != 0 {
		d.final(resultWords(d.open.Result()))
		d.last = ""
		return
	}
	p := jsonField(d.open.PartialResult(), "partial")
	switch {
	case p != d.last:
		d.last = p
		d.lastChange = time.Now()
		if p != "" {
			d.send("Partial", p)
		}
	case p != "" && time.Since(d.lastChange) > endpointTimeout:
		// Vosk didn't endpoint but the words have been stable — finalize it
		// ourselves so a pause always completes the phrase.
		d.final(resultWords(d.open.FinalResult()))
		d.last = ""
	}
}

// flush ends whatever is still being heard, as a pause would.
func (d *decoder) flush() { d.final(resultWords(d.open.FinalResult())) }

const sampleRate = 16000

func jsonField(s, field string) string {
	var m map[string]any
	if json.Unmarshal([]byte(s), &m) != nil {
		return ""
	}
	v, _ := m[field].(string)
	return v
}
