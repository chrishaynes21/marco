package visionbench

import (
	"image"
	"sort"
)

// Deterministic structural features, and why each one is here.
//
// Every feature below earned its place from the measured failure population on the Rocket
// League corpus, not from a list of things one could compute. The corpus says:
//
//	65 of 92 detections (71%) landed on frames the manifest declares to contain NO
//	interface structure at all, and almost every one of them is a near-minimum-size
//	rectangle of uniform colour lying in arena scenery.
//
// So the question a feature has to help answer is narrow: what distinguishes a control from
// a patch of texture that happens to be one colour? Two things do, and they are the two
// computed here.
//
// # What is deliberately NOT computed
//
// Nested-duplicate suppression. The measured nesting depth across the whole corpus is ZERO,
// because the grid scan consumes cells as it grows a region and can therefore never emit one
// rectangle inside another. A suppression pass would be decorative logic that never fires —
// see the ablation table in the experiment record. If candidate generation is ever replaced
// by connected components, this becomes necessary and not before.
//
// Scene-sized rejection, for the same reason: the largest candidate in the corpus occupies
// 7.9% of its frame. There is nothing to reject.

// candidate is one rectangle before classification, with the evidence about it.
type candidate struct {
	// cell coordinates, so features can be computed against the sampled grid rather than
	// re-reading pixels.
	x0, y0, w, h int
	bounds       image.Rectangle
	colour       [3]int32
	feat         features
}

// features are bounded, deterministic, and computed from the grid the scan already built.
type features struct {
	// NormW and NormH are the candidate's size as a fraction of the frame, so a rule
	// tuned at 1080p still means the same thing at 720p or on a cropped fixture.
	NormW, NormH float64
	// Aspect is width over height.
	Aspect float64
	// AreaRatio is the fraction of the frame covered.
	AreaRatio float64

	// BorderContinuity is the fraction of the rectangle's perimeter that has a genuine
	// colour change just outside it, 0..1.
	//
	// The single most discriminating feature measured. A control is bounded on every side
	// — that is what makes it a control rather than a region — whereas a uniform patch of
	// arena texture is bounded only where the scan happened to stop, which is typically
	// two edges out of four. Cheap: it reads the ring of grid cells around the candidate,
	// which the scan has already sampled.
	BorderContinuity float64

	// AlignedPeers is how many other candidates share this one's width or height AND sit
	// in the same row or column with regular spacing.
	//
	// Interface is regular; scenery is not. Two 80x24 rectangles at the same x, 40px
	// apart, are a menu. One 24x24 rectangle alone in a textured field is a coincidence,
	// and the corpus is full of the latter.
	AlignedPeers int
	// RowMembers and ColMembers are the group sizes this candidate belongs to, kept
	// separately so a classifier can tell a horizontal toolbar from a vertical menu.
	RowMembers, ColMembers int
}

// computeFeatures fills in every candidate's features, including the group evidence that
// can only be known once all candidates exist.
func computeFeatures(cands []candidate, cell [][3]int32, cols, rows int,
	frame image.Rectangle, tolerance uint8) {

	fw, fh := float64(frame.Dx()), float64(frame.Dy())
	for i := range cands {
		c := &cands[i]
		r := c.bounds
		if fw > 0 && fh > 0 {
			c.feat.NormW = float64(r.Dx()) / fw
			c.feat.NormH = float64(r.Dy()) / fh
			c.feat.AreaRatio = float64(r.Dx()*r.Dy()) / (fw * fh)
		}
		if r.Dy() > 0 {
			c.feat.Aspect = float64(r.Dx()) / float64(r.Dy())
		}
		c.feat.BorderContinuity = borderContinuity(*c, cell, cols, rows, tolerance)
	}
	assignAlignment(cands)
}

// borderContinuity measures how much of the perimeter has a real edge outside it.
//
// Walks the ring of cells immediately outside the candidate and counts those whose colour
// differs from the candidate's own beyond tolerance. A cell outside the frame counts as an
// edge: the screen boundary genuinely bounds a control, and HUD elements sit against it.
func borderContinuity(c candidate, cell [][3]int32, cols, rows int, tolerance uint8) float64 {
	t := int32(tolerance)
	differs := func(x, y int) bool {
		if x < 0 || y < 0 || x >= cols || y >= rows {
			return true // the frame edge is a real boundary
		}
		o := cell[y*cols+x]
		return abs32(o[0]-c.colour[0]) > t || abs32(o[1]-c.colour[1]) > t ||
			abs32(o[2]-c.colour[2]) > t
	}

	total, edged := 0, 0
	for x := c.x0; x < c.x0+c.w; x++ {
		total += 2
		if differs(x, c.y0-1) {
			edged++
		}
		if differs(x, c.y0+c.h) {
			edged++
		}
	}
	for y := c.y0; y < c.y0+c.h; y++ {
		total += 2
		if differs(c.x0-1, y) {
			edged++
		}
		if differs(c.x0+c.w, y) {
			edged++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(edged) / float64(total)
}

// assignAlignment records how regularly each candidate sits beside its peers.
//
// Deterministic by construction: candidates are sorted into a stable order before grouping,
// so the same frame always produces the same groups and therefore the same classifications.
// An earlier version grouped in map order and produced two different scores for one fixture.
func assignAlignment(cands []candidate) {
	if len(cands) < 2 {
		return
	}
	order := make([]int, len(cands))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		ca, cb := cands[order[a]].bounds, cands[order[b]].bounds
		if ca.Min.X != cb.Min.X {
			return ca.Min.X < cb.Min.X
		}
		return ca.Min.Y < cb.Min.Y
	})

	for _, i := range order {
		ci := cands[i].bounds
		cols, rowsN := 0, 0
		for _, j := range order {
			if i == j {
				continue
			}
			cj := cands[j].bounds
			// A COLUMN: same left edge, same width, stacked vertically. The shape of a
			// menu, which is the structure the corpus's pause frames actually contain.
			if near(ci.Min.X, cj.Min.X, alignTolerance) &&
				near(ci.Dx(), cj.Dx(), sizeTolerance) {
				cols++
			}
			// A ROW: same top edge, same height, side by side. A toolbar.
			if near(ci.Min.Y, cj.Min.Y, alignTolerance) &&
				near(ci.Dy(), cj.Dy(), sizeTolerance) {
				rowsN++
			}
		}
		cands[i].feat.ColMembers = cols
		cands[i].feat.RowMembers = rowsN
		cands[i].feat.AlignedPeers = maxInt(cols, rowsN)
	}
}

// alignTolerance and sizeTolerance are in pixels.
//
// Both are one grid step (8px) plus a little: interface elements line up exactly, and the
// grid scan quantises their edges to the step, so anything further apart than one cell was
// not aligned to begin with. Tighter would make alignment depend on where the scan happened
// to land; looser would call two unrelated texture patches a menu.
const (
	alignTolerance = 12
	sizeTolerance  = 12
)

func near(a, b, tol int) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
