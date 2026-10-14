package main

// Precomputing a recording's proposals once, and keeping the two reasons a look has none apart.
//
// # Why `Missing` is not "the proposer found nothing"
//
// `fastobserve.Look.Missing` means NO PIXELS WERE OBTAINED — a failed capture, a minimised
// window. It is not the same as a look whose pixels arrived and contained nothing the proposer
// could segment, and three replay harnesses had been setting it for both.
//
// That conflation produced a wrong diagnosis, which is how it was found. Look 643 of
// `rl-freeplay-1` was reported as a capture gap by [[Experiment-037-why-the-binding-disappeared]];
// the frame is on disk at full resolution, the recorder marked it `front: target` with no drop,
// and it decodes — to a completely black screen the game presented mid-transition. The capture
// worked perfectly. The proposer correctly found nothing in a black frame.
//
// So the two are separated here, once, for every harness that walks a recording.

import (
	"image"
	"path/filepath"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
)

// proposeAll runs the production proposer over every frame.
//
// `readable[i]` is false only when the pixels could not be obtained at all. A readable frame with
// no proposals is an ordinary thing — a loading screen, a fade, a black frame — and the observer
// should see it as a look that saw nothing rather than as one nobody could take.
func proposeAll(dir string, frames []frameRecord) (regions [][]observe.Region, readable []bool) {
	regions = make([][]observe.Region, len(frames))
	readable = make([]bool, len(frames))
	opt := structure.DefaultOptions()
	for i, fr := range frames {
		img, err := readPNG(filepath.Join(dir, filepath.FromSlash(fr.File)))
		if err != nil {
			continue
		}
		readable[i] = true
		res := structure.Propose(img, opt)
		rects := make([]image.Rectangle, 0, len(res.Candidates))
		for _, c := range res.Candidates {
			rects = append(rects, c.Bounds)
		}
		regions[i] = observe.NormaliseRects(img.Bounds(), rects)
	}
	return regions, readable
}
