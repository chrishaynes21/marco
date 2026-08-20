package main

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

// runDetect is the `vision detect <in.png> [out.png]` debug command: run the detector over
// a SCREENSHOT FILE (no live capture), print a table of what it found, and write an
// annotated copy with a coloured box per element. It's the spike tool — point it at a few
// real screenshots to see whether a candidate model detects YOUR UIs before wiring it into
// the learn pipeline. Works in any build: the default (null) detector reports "no model", the
// `-tags onnxvision` build runs the real one.
func runDetect(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: vision detect <in.png> [out.png]")
		return 2
	}
	inPath := args[0]
	outPath := strings.TrimSuffix(inPath, filepath.Ext(inPath)) + "-detected.png"
	if len(args) > 1 {
		outPath = args[1]
	}

	raw, err := os.ReadFile(inPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		return 1
	}
	img, err := decodeRGBA(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	det := newDetector()
	if !det.Ready() {
		fmt.Fprintln(os.Stderr, "no model loaded — build with -tags onnxvision and set "+
			"$MARCO_VISION_MODEL + $MARCO_ONNXRUNTIME, then retry")
		return 1
	}
	els, err := det.Detect(img)
	if err != nil {
		fmt.Fprintln(os.Stderr, "detect:", err)
		return 1
	}
	if len(els) == 0 {
		fmt.Println("no elements detected (try lowering $MARCO_VISION_CONF)")
		return 0
	}

	// Table of detections, and a legend mapping each class to its box colour.
	fmt.Printf("%d element(s) in %s:\n", len(els), inPath)
	seen := map[string]bool{}
	for i, e := range els {
		fmt.Printf("  [%2d] %-14s score %.2f  box (%d,%d  %dx%d)\n",
			i, e.Label, e.Score, e.Box.Min.X, e.Box.Min.Y, e.Box.Dx(), e.Box.Dy())
		seen[e.Label] = true
	}
	fmt.Println("legend:")
	for label := range seen {
		c := colorFor(label)
		fmt.Printf("  %-14s = RGB(%d,%d,%d)\n", label, c.R, c.G, c.B)
	}

	// Annotate a copy and write it out.
	out := image.NewRGBA(img.Bounds())
	copy(out.Pix, img.Pix)
	for _, e := range els {
		drawRect(out, e.Box, colorFor(e.Label), 3)
	}
	data, err := screen.EncodePNG(out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode:", err)
		return 1
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		return 1
	}
	fmt.Printf("annotated → %s\n", outPath)
	return 0
}

// palette is the box-colour cycle; a class maps to a stable entry by name hash so the same
// label is always the same colour across runs and screenshots.
var palette = []color.RGBA{
	{255, 64, 64, 255},  // red
	{64, 200, 64, 255},  // green
	{64, 128, 255, 255}, // blue
	{255, 196, 0, 255},  // amber
	{200, 64, 255, 255}, // purple
	{0, 200, 200, 255},  // cyan
	{255, 128, 0, 255},  // orange
}

func colorFor(label string) color.RGBA {
	h := fnv.New32a()
	_, _ = h.Write([]byte(label))
	return palette[int(h.Sum32())%len(palette)]
}

// drawRect outlines r on img with a thick-px border of colour c (clipped to bounds).
func drawRect(img *image.RGBA, r image.Rectangle, c color.RGBA, thick int) {
	r = r.Intersect(img.Bounds())
	for t := 0; t < thick; t++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			setPix(img, x, r.Min.Y+t, c)
			setPix(img, x, r.Max.Y-1-t, c)
		}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			setPix(img, r.Min.X+t, y, c)
			setPix(img, r.Max.X-1-t, y, c)
		}
	}
}

func setPix(img *image.RGBA, x, y int, c color.RGBA) {
	if (image.Point{X: x, Y: y}).In(img.Bounds()) {
		img.SetRGBA(x, y, c)
	}
}
