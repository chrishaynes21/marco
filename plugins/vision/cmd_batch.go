package main

// `vision batch <manifest.txt> <out.jsonl>` — run the detector over many files in ONE process.
//
// # Why this exists beside `detect`
//
// `detect` is the spike: one screenshot, an annotated copy, a table to read. Replaying a
// nine-hundred-frame recording through the detector with it costs nine hundred model loads, and
// the model is 97MB. This is the same detector, loaded once, over a list.
//
// It writes JSONL rather than printing, because the consumer is
// `perceptionreplay -onnx <file>`: the offline integration needs recorded detections it can
// replay at a simulated delay, and re-running inference inside the replay would make a
// measurement of the observer into a measurement of the model's throughput.
//
// Nothing here captures a screen. The manifest names files that already exist.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// batchFrame is one frame's detections, as the replay reads them back.
type batchFrame struct {
	File    string           `json:"file"`
	Width   int              `json:"width"`
	Height  int              `json:"height"`
	MS      float64          `json:"ms"`
	Error   string           `json:"error,omitempty"`
	Element []batchDetection `json:"elements,omitempty"`
}

type batchDetection struct {
	Label string  `json:"label"`
	Score float64 `json:"score"`
	X     int     `json:"x"`
	Y     int     `json:"y"`
	W     int     `json:"w"`
	H     int     `json:"h"`
}

func runBatch(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: vision batch <manifest.txt> <out.jsonl>")
		return 2
	}
	manifest, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "manifest:", err)
		return 1
	}
	var files []string
	for _, line := range strings.Split(string(manifest), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "manifest names no files")
		return 1
	}

	loadStart := time.Now()
	det := newDetector()
	load := time.Since(loadStart)
	if !det.Ready() {
		fmt.Fprintln(os.Stderr, "no model loaded — build with -tags onnxvision and set "+
			"$MARCO_VISION_MODEL + $MARCO_ONNXRUNTIME, then retry")
		return 1
	}

	out, err := os.Create(args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "create:", err)
		return 1
	}
	defer out.Close()
	w := bufio.NewWriter(out)
	defer w.Flush()
	enc := json.NewEncoder(w)

	fmt.Printf("model load %v; %d frame(s)\n", load.Round(time.Millisecond), len(files))
	var total time.Duration
	var failed, detections int
	for i, path := range files {
		fr := batchFrame{File: path}
		// A FRAME THAT CANNOT BE READ IS RECORDED AS SUCH rather than skipped. The
		// replay matches these back to looks by position, and a silently shorter
		// file would shift every later frame's detections onto the wrong look.
		raw, err := os.ReadFile(path)
		if err != nil {
			fr.Error = err.Error()
		} else if img, derr := decodeRGBA(raw); derr != nil {
			fr.Error = derr.Error()
		} else {
			fr.Width, fr.Height = img.Bounds().Dx(), img.Bounds().Dy()
			t0 := time.Now()
			els, derr := det.Detect(img)
			d := time.Since(t0)
			total += d
			fr.MS = float64(d.Microseconds()) / 1000
			if derr != nil {
				fr.Error = derr.Error()
			}
			for _, e := range els {
				fr.Element = append(fr.Element, batchDetection{
					Label: e.Label, Score: float64(e.Score),
					X: e.Box.Min.X, Y: e.Box.Min.Y,
					W: e.Box.Dx(), H: e.Box.Dy(),
				})
			}
			detections += len(els)
		}
		if fr.Error != "" {
			failed++
		}
		if err := enc.Encode(fr); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			return 1
		}
		if (i+1)%50 == 0 {
			fmt.Printf("  %d/%d\n", i+1, len(files))
			_ = w.Flush()
		}
	}
	mean := time.Duration(0)
	if n := len(files) - failed; n > 0 {
		mean = total / time.Duration(n)
	}
	fmt.Printf("%d frame(s), %d detection(s), %d failed; mean inference %v\n",
		len(files), detections, failed, mean.Round(time.Millisecond))
	return 0
}
