package visionbench

import (
	"context"
	"fmt"
	"image"
	"sort"
)

// A deterministic structure detector, as the second backend.
//
// Its purpose is twofold. It proves the harness takes an arbitrary backend — this one is
// pure Go, has no model, no weights and no plugin — and it is a genuine candidate in its own
// right: interface chrome is rectangular, and the thing the learned detector conspicuously
// failed to produce on real games was RECTANGLES it could call panels and buttons.
//
// It is not expected to win. It is expected to be measurable, which is the whole point of
// building a benchmark before choosing a model.
//
// # What it looks for
//
// Uniform axis-aligned rectangles: runs of similar colour with straight edges, which is what
// a panel, a button, a bar and a slot all are. It has no idea what any of them mean, and
// says so by labelling everything with the most conservative structural word that fits.

// # The pipeline
//
//	frame → rectangle candidates → deterministic features → classification → detections
//
// Generation and classification are separate stages because conflating them made every
// rejection unexplainable, and this detector's measured problem is entirely about what it
// should be rejecting: 71% of its output on the reference corpus landed on frames the
// manifest declares to contain no interface at all.

// Classical is a deterministic rectangle detector.
type Classical struct {
	// MinSide is the smallest edge worth reporting, in pixels, at candidate-generation
	// time. Classification applies a NORMALISED floor on top of this; this one only
	// bounds the scan's own cost.
	MinSide int
	// Tolerance is how much colour may vary inside one region, per channel.
	Tolerance uint8
	// MaxRegions bounds the output so a noisy frame cannot produce thousands.
	MaxRegions int
	// Ablations disable individual heuristics, for measuring what each contributes.
	// The zero value is the full detector.
	Ablations Ablations

	// Stride is the grid sampling step in pixels. Zero means the default.
	//
	// Exposed to be MEASURED rather than argued about. An 8px grid cannot resolve a 16px
	// control sitting at a non-multiple-of-8 offset — it merges into the background run —
	// and the obvious fix of halving the stride costs four times the sampling. Which
	// trade is right is a measurement on full-resolution frames, and until corpus v2
	// exists there was nothing to measure it on.
	Stride int
	// Offsets samples the grid several times at sub-stride shifts, unioning the results.
	//
	// The alternative to a finer stride: a 16px control missed at offset 0 is found at
	// offset 4, at twice the cost rather than four times. Empty means one pass at 0.
	Offsets []int

	// lastRejections counts why candidates were refused on the most recent frame, for
	// benchmark diagnostics. Not part of any detection and never observed by the
	// Director: the production path sees detections, and nothing else.
	lastRejections map[Rejection]int
	lastCandidates int
}

// DefaultStride is the grid sampling step, in pixels.
//
// 8, unchanged, and now a named constant so the sweep that questions it has something to
// vary. A full-pixel connected-components pass would find everything and would dominate the
// benchmark's own latency column, which is the trade this number represents.
const DefaultStride = 8

// NewClassical returns the detector with sensible bounds.
func NewClassical() *Classical {
	return &Classical{MinSide: 24, Tolerance: 18, MaxRegions: 120, Stride: DefaultStride}
}

// Explain runs the pipeline and reports every candidate with its features and verdict.
//
// The tuning surface. Detect returns what survived; this returns what was considered and
// why each one did or did not make it, which is the difference between tuning a detector
// and guessing at it. Benchmark and test only — nothing in the Director calls it.
func (c *Classical) Explain(frame image.Image) []CandidateReport {
	cands, cell, cols, rows, b := c.candidates(frame)
	if len(cands) == 0 {
		return nil
	}
	computeFeatures(cands, cell, cols, rows, b, c.Tolerance)

	out := make([]CandidateReport, 0, len(cands))
	for _, cand := range cands {
		role, why := classify(cand, c.Ablations)
		out = append(out, CandidateReport{
			Bounds: cand.bounds, Features: cand.feat, Role: role, Rejected: why,
		})
	}
	return out
}

// CandidateReport is one candidate's full account, for diagnostics.
type CandidateReport struct {
	Bounds   image.Rectangle
	Features features
	Role     string
	Rejected Rejection
}

// Rejections reports why candidates were refused on the last frame, newest call wins.
//
// Diagnostics only. It exists because tuning this detector without knowing which rule fired
// is guesswork, and the whole milestone is an argument against guesswork.
func (c *Classical) Rejections() (map[Rejection]int, int) {
	out := make(map[Rejection]int, len(c.lastRejections))
	for k, v := range c.lastRejections {
		out[k] = v
	}
	return out, c.lastCandidates
}

func (c *Classical) Name() string  { return "classical-cv" }
func (c *Classical) Model() string { return "deterministic rectangles" }

// Detect finds uniform rectangular regions.
//
// A coarse grid scan rather than a full connected-components pass: the frames are 1920x1080
// and a per-pixel flood fill would dominate the benchmark's own latency measurements, which
// would make the latency column meaningless. The grid is fine enough to find controls and
// coarse enough to run in milliseconds.
func (c *Classical) Detect(_ context.Context, frame image.Image) ([]Detection, error) {
	cands, cell, cols, rows, b := c.candidates(frame)
	if len(cands) == 0 {
		c.lastRejections, c.lastCandidates = map[Rejection]int{}, 0
		return nil, nil
	}

	// ── features, then classification ─────────────────────────────────────────
	computeFeatures(cands, cell, cols, rows, b, c.Tolerance)

	c.lastRejections = map[Rejection]int{}
	c.lastCandidates = len(cands)

	var out []Detection
	for _, cand := range cands {
		role, why := classify(cand, c.Ablations)
		if why != RejectNone {
			c.lastRejections[why]++
			continue
		}
		out = append(out, Detection{
			Label: role,
			// Fixed, and honest: this is geometry, not recognition. A varying
			// "confidence" would imply a belief the method does not hold.
			Confidence: 0.65,
			Bounds:     cand.bounds,
		})
	}

	// Largest first, then capped: if the ceiling has to drop something, the biggest
	// regions are the likeliest to be real interface.
	sort.SliceStable(out, func(i, j int) bool {
		return area(out[i].Bounds) > area(out[j].Bounds)
	})
	if len(out) > c.MaxRegions {
		out = out[:c.MaxRegions]
	}
	return out, nil
}

// candidates is the generation stage: every uniform rectangle the grid scan can find.
//
// It makes no judgement. Everything about whether a rectangle is a control is decided in
// classify, which is the separation that made rejections explainable at all.
func (c *Classical) candidates(frame image.Image) (
	[]candidate, [][3]int32, int, int, image.Rectangle) {

	if frame == nil {
		return nil, nil, 0, 0, image.Rectangle{}
	}
	b := frame.Bounds()
	if b.Dx() < c.MinSide || b.Dy() < c.MinSide {
		return nil, nil, 0, 0, b
	}

	step := c.Stride
	if step <= 0 {
		step = DefaultStride
	}

	// Multi-offset: run the scan at each sub-stride shift and union the results. A control
	// missed at offset 0 because it straddles the grid is found at offset 4, and the cost
	// is one extra pass rather than the four a halved stride would charge.
	//
	// The union is by candidate, and duplicates across offsets are expected — the same
	// panel will be found by every pass. They are dropped by position so downstream sees
	// one rectangle per thing, which is what a nested-suppression pass would otherwise
	// have to do later.
	if len(c.Offsets) > 1 {
		return c.multiOffset(frame, b, step)
	}

	origin := 0
	if len(c.Offsets) == 1 {
		origin = c.Offsets[0]
	}
	cols, rows := (b.Dx()-origin)/step, (b.Dy()-origin)/step
	if cols < 2 || rows < 2 {
		return nil, nil, 0, 0, b
	}
	b = image.Rect(b.Min.X+origin, b.Min.Y+origin, b.Max.X, b.Max.Y)

	// Sample once per cell, then group neighbouring cells of near-identical colour.
	cell := make([][3]int32, cols*rows)
	for y := range rows {
		for x := range cols {
			r, g, bl, _ := frame.At(b.Min.X+x*step, b.Min.Y+y*step).RGBA()
			cell[y*cols+x] = [3]int32{int32(r >> 8), int32(g >> 8), int32(bl >> 8)}
		}
	}

	seen := make([]bool, cols*rows)
	var cands []candidate
	for y := range rows {
		for x := range cols {
			if seen[y*cols+x] {
				continue
			}
			w, h := c.extent(cell, seen, cols, rows, x, y)
			if w*step < c.MinSide || h*step < c.MinSide {
				// Mark it consumed anyway, so the scan does not revisit a region it
				// has already rejected.
				markUsed(seen, cols, x, y, w, h)
				continue
			}
			markUsed(seen, cols, x, y, w, h)
			cands = append(cands, candidate{
				x0: x, y0: y, w: w, h: h,
				colour: cell[y*cols+x],
				bounds: image.Rect(
					b.Min.X+x*step, b.Min.Y+y*step,
					b.Min.X+(x+w)*step, b.Min.Y+(y+h)*step),
			})
		}
	}
	return cands, cell, cols, rows, b
}

// multiOffset unions the scan across several sub-stride origins.
//
// Returns the FIRST offset's grid alongside the merged candidates, because features are
// computed against a grid and the passes have different ones. That is a real limitation and
// is stated rather than hidden: a candidate found only at offset 4 has its border measured
// against offset 0's samples, which is approximate. It is good enough for the recall question
// this exists to answer, and would not be good enough to ship without measuring.
func (c *Classical) multiOffset(frame image.Image, b image.Rectangle, step int) (
	[]candidate, [][3]int32, int, int, image.Rectangle) {

	var merged []candidate
	var baseCell [][3]int32
	var baseCols, baseRows int
	var baseBounds image.Rectangle
	seen := map[string]bool{}

	for i, off := range c.Offsets {
		pass := &Classical{
			MinSide: c.MinSide, Tolerance: c.Tolerance, MaxRegions: c.MaxRegions,
			Stride: step, Offsets: []int{off},
		}
		cands, cell, cols, rows, pb := pass.candidates(frame)
		if i == 0 {
			baseCell, baseCols, baseRows, baseBounds = cell, cols, rows, pb
		}
		for _, cand := range cands {
			// Deduplicate by position, tolerant to the sub-stride shift itself: the same
			// panel found at offsets 0 and 4 differs by up to 4px and is one panel.
			key := fmt.Sprintf("%d:%d:%d:%d",
				cand.bounds.Min.X/step, cand.bounds.Min.Y/step,
				cand.bounds.Dx()/step, cand.bounds.Dy()/step)
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, cand)
		}
	}
	// Stable order, so a multi-offset run is as deterministic as a single-offset one.
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].bounds.Min.Y != merged[j].bounds.Min.Y {
			return merged[i].bounds.Min.Y < merged[j].bounds.Min.Y
		}
		return merged[i].bounds.Min.X < merged[j].bounds.Min.X
	})
	return merged, baseCell, baseCols, baseRows, baseBounds
}

// extent grows a rectangle of near-uniform cells from a corner.
func (c *Classical) extent(cell [][3]int32, seen []bool, cols, rows, x0, y0 int) (int, int) {
	base := cell[y0*cols+x0]

	width := 0
	for x := x0; x < cols && c.near(cell[y0*cols+x], base) && !seen[y0*cols+x]; x++ {
		width++
	}
	if width == 0 {
		return 0, 0
	}
	height := 0
	for y := y0; y < rows; y++ {
		ok := true
		for x := x0; x < x0+width; x++ {
			if seen[y*cols+x] || !c.near(cell[y*cols+x], base) {
				ok = false
				break
			}
		}
		if !ok {
			break
		}
		height++
	}
	return width, height
}

func (c *Classical) near(a, b [3]int32) bool {
	t := int32(c.Tolerance)
	return abs32(a[0]-b[0]) <= t && abs32(a[1]-b[1]) <= t && abs32(a[2]-b[2]) <= t
}

func markUsed(seen []bool, cols, x0, y0, w, h int) {
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			if idx := y*cols + x; idx >= 0 && idx < len(seen) {
				seen[idx] = true
			}
		}
	}
}

func area(r image.Rectangle) int { return r.Dx() * r.Dy() }

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
