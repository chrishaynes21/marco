package visualstate

import (
	"fmt"
	"image"
	"math"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Appearance analysis, kept deliberately small.
//
// The instruction this file follows most closely: IF A STATE CANNOT BE DETECTED
// RELIABLY, RETURN NO OBSERVATION RATHER THAN GUESSING. A missing observation costs the
// Director a piece of state it never had; a wrong one puts a confident falsehood into
// fusion, where it will be attached to a real control and believed.
//
// So there is no universal state classifier here and no attempt at one. There are two
// things a downscaled colour grid can genuinely tell you about a UI region — whether it
// is uniformly distinct from its surroundings (highlight), and whether it is uniformly
// washed out (disabled) — plus the change detection that is the point of the package.
// Everything else returns nothing.

// Detection thresholds. Provisional, and named so a disagreement moves a number rather
// than adding a special case.
const (
	// highlightContrast is how far a region's mean colour must sit from its
	// surroundings before "this looks highlighted" is worth saying.
	highlightContrast = 0.12
	// highlightUniformity is how consistent the region's own colour must be. A
	// selection highlight is a flat wash; a region that is merely BUSY is not
	// highlighted, however far its average sits from anything.
	highlightUniformity = 0.10
	// desaturation is how little colour variation a region may have before it looks
	// greyed out.
	desaturation = 0.05
	// minAnalysisCells guards against concluding anything from a region so small that
	// its grid is a handful of cells.
	minAnalysisCells = 16
)

// analyse looks at one captured region and reports what it can honestly say.
func (p *Provider) analyse(snap Snapshot, req Request) []observation.Observation {
	if len(req.Kinds) == 0 {
		// No kinds requested: this was a change-detection capture, and a lone snapshot
		// is not a change. The caller compares it with another.
		return nil
	}

	var out []observation.Observation
	for _, want := range req.Kinds {
		obs, ok := p.detect(want, snap, req)
		if !ok {
			continue // could not tell; say nothing
		}
		out = append(out, obs)
	}
	return out
}

// detect attempts one state kind. The bool is whether it could be established at all.
func (p *Provider) detect(kind observation.VisualStateKind, snap Snapshot,
	req Request) (observation.VisualState, bool) {

	if snap.Image == nil || len(snap.Fingerprint.Cells) < minAnalysisCells {
		return observation.VisualState{}, false
	}

	switch kind {
	case observation.VisualSelected:
		score, ok := highlighted(snap)
		if !ok {
			return observation.VisualState{}, false
		}
		return p.stateObservation(kind, snap, req, score,
			"the region is a flat wash of colour distinct from its surroundings, "+
				"which is what a selection highlight looks like"), true

	case observation.VisualDisabledAppearance:
		score, ok := washedOut(snap)
		if !ok {
			return observation.VisualState{}, false
		}
		return p.stateObservation(kind, snap, req, score,
			"the region has almost no colour variation, which is what a greyed-out "+
				"control looks like"), true
	}

	// Everything else — checked, pressed, expanded, loading, progress — needs either a
	// before image, a template, or semantics this package deliberately does not have.
	// Returning nothing is the correct answer, and a far better one than a guess that
	// fusion would attach to a real control.
	return observation.VisualState{}, false
}

// stateObservation builds an appearance observation.
func (p *Provider) stateObservation(kind observation.VisualStateKind, snap Snapshot,
	req Request, score float64, why string) observation.VisualState {

	return observation.VisualState{
		ObservationID: mintID(),
		CycleID:       req.Cycle,
		ProviderID:    p.Name(),
		VisualKind:    kind,
		From:          directorapi.SourceVision,
		At:            snap.At,
		Box:           snap.Region,
		Score:         score,
		WindowID:      snap.Window,
		ApplicationID: snap.Application,
		TargetHint:    req.Target,
		Metadata:      map[string]string{"reason": why},
	}
}

// highlighted reports whether a region looks like a selection highlight.
//
// Two conditions, both necessary. The region's own colour must be UNIFORM — a
// highlight is a flat wash, and a region full of varied content is not highlighted
// however striking its average. And that uniform colour must CONTRAST with the border
// ring just outside it, because "uniform" alone describes every empty area on screen.
func highlighted(snap Snapshot) (float64, bool) {
	f := snap.Fingerprint
	if f.GridW < 4 || f.GridH < 4 {
		return 0, false
	}

	inner, innerVar := meanAndSpread(f, 1, 1, f.GridW-1, f.GridH-1)
	if innerVar > highlightUniformity {
		return 0, false // too varied to be a wash
	}
	edge, _ := ringMean(f)
	contrast := cellDistance(inner, edge)
	if contrast < highlightContrast {
		return 0, false
	}

	// Confidence grows with contrast and falls with internal variation, and is capped
	// well below certainty: this is an inference from averaged colour, and it should
	// never outrank a structural source that actually knows.
	score := 0.55 + contrast
	if score > 0.9 {
		score = 0.9
	}
	return score, true
}

// washedOut reports whether a region looks greyed out.
func washedOut(snap Snapshot) (float64, bool) {
	f := snap.Fingerprint
	if f.GridW < 4 || f.GridH < 4 {
		return 0, false
	}
	_, spread := meanAndSpread(f, 0, 0, f.GridW, f.GridH)
	if spread > desaturation {
		return 0, false
	}
	// A flat region might be disabled, or might be blank. Reported at low confidence
	// precisely because those look identical from here — fusion will only attach it to
	// an element whose role permits it, and will not let it overrule a structural
	// enabled flag.
	return 0.6, true
}

// meanAndSpread is a sub-grid's mean colour and how much its cells vary from it.
func meanAndSpread(f Fingerprint, x0, y0, x1, y1 int) ([3]float64, float64) {
	var sum [3]float64
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := f.Cells[y*f.GridW+x]
			sum[0] += c[0]
			sum[1] += c[1]
			sum[2] += c[2]
			n++
		}
	}
	if n == 0 {
		return [3]float64{}, 0
	}
	mean := [3]float64{sum[0] / float64(n), sum[1] / float64(n), sum[2] / float64(n)}

	spread := 0.0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			spread += cellDistance(f.Cells[y*f.GridW+x], mean)
		}
	}
	return mean, spread / float64(n)
}

// ringMean is the mean colour of the grid's outermost ring — the region's immediate
// surroundings, which is what an appearance has to contrast with to mean anything.
func ringMean(f Fingerprint) ([3]float64, int) {
	var sum [3]float64
	n := 0
	add := func(x, y int) {
		c := f.Cells[y*f.GridW+x]
		sum[0] += c[0]
		sum[1] += c[1]
		sum[2] += c[2]
		n++
	}
	for x := 0; x < f.GridW; x++ {
		add(x, 0)
		add(x, f.GridH-1)
	}
	for y := 1; y < f.GridH-1; y++ {
		add(0, y)
		add(f.GridW-1, y)
	}
	if n == 0 {
		return [3]float64{}, 0
	}
	return [3]float64{sum[0] / float64(n), sum[1] / float64(n), sum[2] / float64(n)}, n
}

// OverlayAppeared reports whether the change looks like something new drawn ON TOP —
// a menu, a dropdown, a dialog — rather than the region's contents changing in place.
//
// The signal is CONCENTRATION. A menu opening changes a contiguous block and leaves the
// rest alone; a page navigating changes everything. Distinguishing them matters because
// "a menu appeared" verifies a click on File, where "the page changed" does not.
func OverlayAppeared(before, after Fingerprint, th Thresholds) (bool, string) {
	if before.Empty() || after.Empty() ||
		before.GridW != after.GridW || before.GridH != after.GridH {
		return false, ""
	}

	changed := make([]bool, len(before.Cells))
	total := 0
	for i := range before.Cells {
		if cellDistance(before.Cells[i], after.Cells[i]) >= th.CellDelta {
			changed[i] = true
			total++
		}
	}
	if total == 0 {
		return false, ""
	}
	fraction := float64(total) / float64(len(before.Cells))
	// A change covering nearly everything is a repaint, not an overlay.
	if fraction > 0.75 {
		return false, ""
	}

	// The changed cells' bounding box. A contiguous block fills most of its own box;
	// scattered noise fills very little of a large one.
	minX, minY, maxX, maxY := before.GridW, before.GridH, -1, -1
	for i, c := range changed {
		if !c {
			continue
		}
		x, y := i%before.GridW, i/before.GridW
		minX, minY = minInt(minX, x), minInt(minY, y)
		maxX, maxY = maxInt(maxX, x), maxInt(maxY, y)
	}
	boxCells := (maxX - minX + 1) * (maxY - minY + 1)
	if boxCells <= 0 {
		return false, ""
	}
	density := float64(total) / float64(boxCells)
	if density < 0.6 {
		return false, "" // scattered, not a block
	}
	if fraction < th.MeaningfulCells {
		return false, "" // too small to be an overlay
	}
	return true, fmt.Sprintf(
		"a contiguous block covering %.0f%% of the region changed (%.0f%% dense), which "+
			"is what something drawn on top looks like rather than a repaint",
		fraction*100, density*100)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SubImage crops an image, used when a region capture returns more than was asked for.
func SubImage(img image.Image, r image.Rectangle) image.Image {
	type subImager interface {
		SubImage(image.Rectangle) image.Image
	}
	if s, ok := img.(subImager); ok {
		return s.SubImage(r)
	}
	return img
}

var _ = math.Abs
