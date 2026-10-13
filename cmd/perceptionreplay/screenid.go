package main

// Can a screen Marco was shown be recognised again from what was saved about it?
//
// # Why this is not another arrangement experiment
//
// [[Experiment-029-binding-a-screen-across-two-independent-folds]] measured the arrangement
// relation binding the WRONG screen at 0.886 while binding right ones at 0.556, with no floor
// between them. Tuning that relation is not the work. The question is whether some other
// evidence, already in the tree, separates these screens where counts of rectangles in nine
// boxes do not.
//
// So five measures are computed over the same folds and the same looks, and judged the same way:
//
//	arrangement   counts of proposals per 3x3 cell            the production relation, as baseline
//	shape         the same counts as PROPORTIONS              exists, deliberately not the rule,
//	                                                          never put to this use
//	palette       one colour histogram of the whole frame     internal/screen, reused
//	grid          a colour histogram PER 3x3 cell, averaged   palette plus where it is
//	detector      ScreenParser classes, counted               the recorded detections, reused
//
// # What is measured, and why it is not a threshold
//
// Not "does measure X bind screen Y", which invites a floor chosen to make the answer come out.
// For each screen of fold A, every screen of fold B is scored and the list is sorted. Two
// numbers decide everything:
//
//   - whether the ORACLE's counterpart is top of that list, and
//   - the MARGIN between it and the best wrong answer.
//
// A measure where every correct counterpart outranks every wrong one by a visible margin has a
// floor waiting to be derived. A measure where the margins overlap has none, and no threshold
// will produce one — which is precisely what the arrangement relation's failure looked like.
//
// Fold A's descriptors are built from fold A's looks and fold B's from fold B's, so nothing is
// ever compared with itself, and the oracle is used ONLY to score the answers, never to produce
// them. Session screen ids are labels in the output and are not evidence: every measure is
// computed from pixels or proposals.

import (
	"image"
	"path/filepath"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

// screenEvidence is everything saved about one screen, by every measure.
type screenEvidence struct {
	id    observe.VisualScreenID
	truth observe.VisualScreenID
	looks int

	arrangement observe.VisualArrangement
	// palette and grid are per-look, kept as sets rather than averaged: a screen that
	// looks two ways is two palettes, and a mean of them is a third that matches neither.
	palette []screen.Histogram
	grid    [][]screen.Histogram
	// classes counts the detector's classes over the looks sampled.
	classes map[string]int
	// proposals is the per-look proposal count, for the sensitivity report.
	proposals []int
}

// gridCells is the same 3x3 the arrangement uses, so `grid` is the palette asked the question
// the arrangement asks: not just what colours, but where.
const gridCells = 3

// describeScreen samples a screen's looks and builds every descriptor.
func describeScreen(dir string, frames []frameRecord, regions [][]observe.Region,
	dets map[int][]observe.Candidate, f fold, id observe.VisualScreenID,
	truth observe.VisualScreenID, sample int) screenEvidence {

	e := screenEvidence{id: id, truth: truth, classes: map[string]int{}}
	var looks []int
	for look, sc := range f.at {
		if sc == id {
			looks = append(looks, look)
		}
	}
	sort.Ints(looks)
	e.looks = len(looks)
	e.arrangement = f.arr[id]
	if len(looks) == 0 {
		return e
	}
	step := 1
	if len(looks) > sample {
		step = len(looks) / sample
	}
	for i := 0; i < len(looks) && len(e.palette) < sample; i += step {
		look := looks[i]
		img, err := readPNG(filepath.Join(dir, filepath.FromSlash(frames[look].File)))
		if err != nil {
			continue
		}
		e.palette = append(e.palette, wholeFrame(img))
		e.grid = append(e.grid, gridOf(img))
		e.proposals = append(e.proposals, len(regions[look]))
		for _, c := range dets[look+1] {
			if c.Class != "" {
				e.classes[c.Class]++
			}
		}
	}
	return e
}

// wholeFrame is one histogram of everything.
func wholeFrame(img image.Image) screen.Histogram {
	return describeCrop(img, observe.Region{Width: 1, Height: 1}, 0).hist
}

// gridOf is one histogram per cell of the same 3x3 the arrangement counts over.
func gridOf(img image.Image) []screen.Histogram {
	out := make([]screen.Histogram, 0, gridCells*gridCells)
	for gy := range gridCells {
		for gx := range gridCells {
			out = append(out, describeCrop(img, observe.Region{
				X: float64(gx) / gridCells, Y: float64(gy) / gridCells,
				Width: 1.0 / gridCells, Height: 1.0 / gridCells,
			}, 0).hist)
		}
	}
	return out
}

// measureScore is one measure's similarity between two screens' evidence.
//
// Every set-valued measure takes the BEST pair, deliberately generously: a screen that looks two
// ways should be recognised from either, and a measure that cannot manage that under the kindest
// reading has failed on its own terms.
func measureScore(measure string, a, b screenEvidence) float64 {
	switch measure {
	case "arrangement":
		return observe.ArrangementSimilarity(a.arrangement, b.arrangement)
	case "shape":
		return observe.ArrangementShapeSimilarity(a.arrangement, b.arrangement)
	case "palette":
		best := 0.0
		for _, x := range a.palette {
			for _, y := range b.palette {
				if s := screen.HistIntersection(x, y); s > best {
					best = s
				}
			}
		}
		return best
	case "grid":
		best := 0.0
		for _, x := range a.grid {
			for _, y := range b.grid {
				if len(x) != len(y) {
					continue
				}
				sum := 0.0
				for i := range x {
					sum += screen.HistIntersection(x[i], y[i])
				}
				if s := sum / float64(len(x)); s > best {
					best = s
				}
			}
		}
		return best
	case "detector":
		return countSimilarity(a.classes, b.classes)
	}
	return 0
}

// countSimilarity is histogram intersection over named counts.
func countSimilarity(a, b map[string]int) float64 {
	sa, sb := 0, 0
	for _, v := range a {
		sa += v
	}
	for _, v := range b {
		sb += v
	}
	if sa == 0 || sb == 0 {
		return 0
	}
	inter := 0.0
	for k, v := range a {
		x := float64(v) / float64(sa)
		y := float64(b[k]) / float64(sb)
		if y < x {
			x = y
		}
		inter += x
	}
	return inter
}

// recognition is how one measure did on one screen of fold A.
type recognition struct {
	from screenEvidence
	// ranked is every fold-B screen, strongest first.
	ranked []scored
	// correctAt is where the oracle's counterpart came, or -1 when fold B never saw it.
	correctAt int
	// margin is the correct score minus the best wrong one. Negative means something
	// wrong outranked the right answer.
	margin         float64
	hasCounterpart bool
}

type scored struct {
	to  screenEvidence
	sim float64
}

// recognise ranks fold B's screens for one of fold A's, by one measure.
func recognise(measure string, from screenEvidence, to []screenEvidence) recognition {
	r := recognition{from: from, correctAt: -1}
	for _, t := range to {
		r.ranked = append(r.ranked, scored{to: t, sim: measureScore(measure, from, t)})
	}
	sort.SliceStable(r.ranked, func(i, j int) bool { return r.ranked[i].sim > r.ranked[j].sim })

	var correct, bestWrong float64
	bestWrong = -1
	for i, s := range r.ranked {
		same := s.to.truth == from.truth && from.truth != ""
		if same {
			r.hasCounterpart = true
			if r.correctAt < 0 {
				r.correctAt, correct = i, s.sim
			}
			continue
		}
		if s.sim > bestWrong {
			bestWrong = s.sim
		}
	}
	if r.hasCounterpart && bestWrong >= 0 {
		r.margin = correct - bestWrong
	}
	return r
}
