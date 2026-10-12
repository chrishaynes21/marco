package main

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// THE L0 RECORDING IS A CONTRACT TOO, AND ITS SILENT BREAK IS WORSE.
//
// `director capture-game-session` writes frames.jsonl, input.jsonl, session.json and a PNG per
// frame. This program reads them back and runs perception over the pixels. The two cannot
// import each other — both are mains — so they agree through JSON tag names and a file layout.
//
// A break here is worse than the L1 one. If `file` stops decoding, every frame fails to open,
// every failure is treated as a sanitised gap (which is correct behaviour for a missing frame),
// and the replay reports a recording of zero frames. Nothing errors. A perception change
// measured against that reports no regression because it reports nothing at all — and the
// recording that was supposed to prove it is 1.4GB of pixels nobody read.
//
// So: a recording is built here, from scratch, in the writer's layout, and replayed.

// writeRecording builds a minimal L0 recording: three frames with a mark that moves.
//
// Written by this test rather than copied from a session, for the reason the L1 golden line is
// hand-written: a fixture taken from one recording freezes that recording's accidents.
func writeRecording(t *testing.T, dir string, frames int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "frames"), 0o755); err != nil {
		t.Fatal(err)
	}
	type frameRec struct {
		Index            int    `json:"index"`
		File             string `json:"file"`
		AtMS             int64  `json:"at_ms"`
		Width            int    `json:"width"`
		Height           int    `json:"height"`
		WindowGeneration uint64 `json:"window_generation"`
	}
	fl, err := os.Create(filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer fl.Close()
	enc := json.NewEncoder(fl)

	const w, h = 480, 360
	for i := range frames {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				img.SetRGBA(x, y, color.RGBA{30, 30, 34, 255})
			}
		}
		// Five rows, one of them lit, and which one advances with the frame — so a
		// replay that reads the pixels sees a mark being handed along, and one that
		// silently reads nothing sees a blank session.
		for row := range 5 {
			r := image.Rect(60, 40+row*56, 420, 40+row*56+40)
			c := color.RGBA{70, 72, 80, 255}
			if row == i%5 {
				c = color.RGBA{225, 226, 232, 255}
			}
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					img.SetRGBA(x, y, c)
				}
			}
		}
		rel := filepath.ToSlash(filepath.Join("frames", "f-"+itoa(i)+".png"))
		f, err := os.Create(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		if err := enc.Encode(frameRec{
			Index: i, File: rel, AtMS: int64(i) * 66,
			Width: w, Height: h, WindowGeneration: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// A recording written in the writer's layout replays, and the pixels reach perception.
func TestAnL0RecordingReplaysThroughPerception(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "session")
	writeRecording(t, dir, 24)

	r, err := replayFrames(dir, 1, 0, false)
	if err != nil {
		t.Fatalf("replaying a freshly written recording: %v", err)
	}
	if r.Frames != 24 {
		t.Fatalf("reprocessed %d frames of 24. A `file` tag that no longer decodes makes "+
			"every frame unopenable, every miss look like a sanitised gap, and the "+
			"whole recording report as empty with no error anywhere", r.Frames)
	}
	if r.Proposed == 0 {
		t.Error("no structure was proposed from 24 frames of five drawn rows, so the " +
			"pixels did not reach the proposer")
	}
	if r.Grouped == 0 {
		t.Error("no frame produced a usable stack from five evenly spaced rows of one " +
			"size, so either the proposals or the normalisation into the look is wrong")
	}
	// AND THE ARRANGEMENT MEASUREMENT RAN. It is collected per frame inside the same
	// loop, so a replay that gathered no signatures reports no screens at all — and
	// that is indistinguishable in the output from a recording the cue could not
	// separate, which is the one conclusion it exists to support.
	if len(r.PixelClusters) == 0 {
		t.Error("the replay reports no pixel arrangements over 24 frames of drawn rows, " +
			"so the measurement in screens.go was never reached")
	}
	// AND THE STABILISED LAYER SAW EVERY LOOK. It is folded as the frames arrive, so a
	// replay that skipped the fold reports no screens — indistinguishable in the output
	// from a recording whose screen never changed, which is the one conclusion it is
	// there to support or refute.
	if r.VisualStability.Looks != 24 {
		t.Errorf("the stabiliser was offered %d look(s) of 24 frames replayed",
			r.VisualStability.Looks)
	}
	if len(r.VisualSeen) == 0 {
		t.Error("no session-local screen settled over 24 frames of five drawn rows, " +
			"which do not move")
	}
	// THE MARK MOVES in this fixture, so the belief has to get somewhere. This is the
	// assertion that the shading was measured FROM THE PIXELS: a replay that handed the
	// watch unshaded geometry would group these rows and report every candidate quiet.
	if r.Belief.Status == observe.InterfaceUnknown && r.Identified < 0 {
		t.Errorf("after 24 frames of a mark walking five rows the belief is still %q "+
			"(%s). The rows were found and grouped, so what did not arrive is the "+
			"shading", r.Belief.Status, r.Belief.Because)
	}
}

// A recording with metadata and no pixels says so, rather than reporting an empty session.
//
// The measured case: `tm-smoke2` on this machine has frames.jsonl and session.json claiming 75
// frames, and a frames directory that does not exist. Old L1 traces have no pixels either. A
// replay must never let "the pixels were never captured" read as "perception saw nothing".
func TestARecordingWithNoFramesIsNotAnEmptySession(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "frames.jsonl"), []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := replayFrames(dir, 1, 0, false); err == nil {
		t.Error("a recording with no frame entries replayed without complaint. " +
			"It must be an error: a silent zero-frame report is indistinguishable " +
			"from a perception failure")
	}
}

// The frame record's tags are what the writer writes.
//
// Checked by decoding a line in the writer's exact shape, since the writer's type lives in
// another main and cannot be imported.
func TestTheFrameRecordReadsTheWritersTags(t *testing.T) {
	const line = `{"index":36,"file":"frames/tm-0036.png","at_ms":2376,` +
		`"width":1920,"height":1080,"window_generation":3}`
	var fr frameRecord
	if err := json.Unmarshal([]byte(line), &fr); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		got  any
		want any
	}{
		{"index", fr.Index, 36},
		{"file", fr.File, "frames/tm-0036.png"},
		{"at_ms", fr.AtMS, int64(2376)},
		{"width", fr.Width, 1920},
		{"height", fr.Height, 1080},
		{"window_generation", fr.WindowGeneration, uint64(3)},
	} {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s decoded as %v, want %v", c.name, c.got, c.want)
		}
	}
	// And the index must be PRESERVED rather than renumbered, because a sanitised
	// recording keeps original indices on purpose: the annotation file and the input log
	// are both keyed to them, and renumbering would falsify when each frame happened.
	if fr.Index != 36 {
		t.Error("the frame index did not survive, so annotations and inputs cannot be " +
			"joined to frames")
	}
}
