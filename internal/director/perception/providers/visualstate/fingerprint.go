// Package visualstate observes how bounded regions of the screen LOOK, and whether
// they changed.
//
// The third and weakest kind of evidence, after structure and text. The rule it exists
// under, stated once:
//
//	Visual evidence may tell the Director how something looks and whether it changed.
//	It may not decide that something is a control, or that it is safe to act upon.
//
// Nothing here can express "this is a button". There is no kind for it, no role field,
// no actionability. What there is: a claim that a region changed, or that a control
// whose ROLE ALREADY PERMITS a state appears to be in it. Both are useful and neither
// can invent a target.
//
// The motivating case is verification rather than perception. Clicking Back in Chrome
// navigates the page, but the navigation does not finish within a settle delay — so
// structural verification fails, and a naive retry clicks Back a second time and sends
// the browser two pages back. Watching the pixels answers the question structure could
// not: something IS happening, so do not do it again.
package visualstate

import (
	"errors"
	"fmt"
	"image"
	"math"
)

// Fingerprinter reduces an image to something comparable, and compares two.
//
// An interface so the reduction can be replaced — a perceptual hash, a structural
// similarity measure, eventually something learned — without any caller learning what
// changed. The default is deliberately simple and deterministic: no model, no training
// data, no dependency, and the same answer every time for the same pixels.
type Fingerprinter interface {
	Fingerprint(img image.Image) (Fingerprint, error)
	Compare(before, after Fingerprint) ChangeResult
}

// ChangeKind is how much two fingerprints differ.
type ChangeKind string

const (
	// ChangeIdentical: nothing at all differs. The only state that permits retrying a
	// non-idempotent action, because it is the only one that proves nothing happened.
	ChangeIdentical ChangeKind = "identical"
	// ChangeMinor: a difference too small or too local to mean anything — antialiasing,
	// a caret blink, a clock digit, the cursor passing through.
	ChangeMinor ChangeKind = "minor_change"
	// ChangeMeaningful: enough of the region is different that something happened.
	ChangeMeaningful ChangeKind = "meaningful_change"
	// ChangeStillChanging: the region was different from the one before AND is
	// different again — an animation, a page loading, a menu sliding open. Never a
	// reason to retry; always a reason to wait and look again.
	ChangeStillChanging ChangeKind = "still_changing"
)

// Meaningful reports whether the change is evidence that something happened.
func (c ChangeKind) Meaningful() bool {
	return c == ChangeMeaningful || c == ChangeStillChanging
}

// ChangeResult is one comparison.
type ChangeResult struct {
	Kind ChangeKind `json:"kind"`
	// ChangedCells is the fraction of the grid that differs materially, 0..1. The
	// headline number: a cursor crossing a button moves one or two cells, where a menu
	// opening moves most of them.
	ChangedCells float64 `json:"changed_cells"`
	// MaxCellDelta is the largest single-cell difference, 0..1. Distinguishes a small
	// region changing completely from a large region shifting imperceptibly.
	MaxCellDelta float64 `json:"max_cell_delta"`
	// Reason is the sentence a person reads.
	Reason string `json:"reason"`
}

// Fingerprint is a region reduced to a comparable summary.
//
// A grid of average colours rather than the pixels. Downscaling is what makes the
// comparison ROBUST rather than merely fast: at this resolution antialiasing and
// sub-pixel text rendering average away, while anything a person would notice — a
// highlight, a menu, a page — moves whole cells.
type Fingerprint struct {
	// Cells are grid-cell average colours, row-major, each 0..1 per channel.
	Cells [][3]float64 `json:"-"`
	// Grid is the grid dimensions used.
	GridW int `json:"grid_w,omitempty"`
	GridH int `json:"grid_h,omitempty"`
	// Width and Height are the source region's pixel dimensions. Compared before
	// anything else: two fingerprints of differently-sized regions describe different
	// things, and averaging over a different area would make them falsely comparable.
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
	// Digest is a short stable string, for logging and for cheap equality.
	Digest string `json:"digest,omitempty"`
}

// Empty reports whether the fingerprint describes nothing.
func (f Fingerprint) Empty() bool { return len(f.Cells) == 0 }

// Thresholds. Named, configurable, and PROVISIONAL — chosen by watching real UI
// transitions, not derived from anything.
type Thresholds struct {
	// Grid is how many cells across and down. Coarse enough that text antialiasing
	// averages out, fine enough that a menu opening in a corner is not diluted to
	// nothing by the rest of the region.
	Grid int
	// CellDelta is how different one cell's average colour must be before that cell
	// counts as changed, 0..1.
	//
	// The single most important number here. Too low and a caret blink reads as a page
	// navigation; too high and a checkbox ticking reads as nothing happening. The
	// second error is the dangerous one — it permits a retry — so this errs low.
	CellDelta float64
	// MinorCells is the fraction of changed cells below which a change is dismissed as
	// noise. A cursor crossing a button, a blinking caret and a ticking clock all move
	// a cell or two out of hundreds.
	MinorCells float64
	// MeaningfulCells is the fraction at which a change is taken as something having
	// happened.
	MeaningfulCells float64
}

// DefaultThresholds are the provisional defaults.
func DefaultThresholds() Thresholds {
	return Thresholds{
		Grid:            24,
		CellDelta:       0.06,
		MinorCells:      0.02,
		MeaningfulCells: 0.05,
	}
}

// GridFingerprinter is the default, deterministic implementation.
type GridFingerprinter struct {
	Thresholds Thresholds
}

// NewFingerprinter returns the default fingerprinter.
func NewFingerprinter() *GridFingerprinter {
	return &GridFingerprinter{Thresholds: DefaultThresholds()}
}

var _ Fingerprinter = (*GridFingerprinter)(nil)

// Fingerprint reduces an image to a grid of average colours.
//
// Averaging rather than sampling. A sampled grid would be cheaper and would miss
// exactly the thing that matters: a one-pixel-wide selection outline contributes
// nothing to a sample and a real amount to an average, and a selection outline is
// precisely the appearance change this package is for.
func (g *GridFingerprinter) Fingerprint(img image.Image) (Fingerprint, error) {
	if img == nil {
		return Fingerprint{}, errors.New("visualstate: no image to fingerprint")
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return Fingerprint{}, fmt.Errorf("visualstate: image has no area (%v)", b)
	}

	grid := g.thresholds().Grid
	if grid <= 0 {
		grid = DefaultThresholds().Grid
	}
	// A region smaller than the grid gets one cell per pixel rather than a grid of
	// mostly-empty cells: a 10x10 checkbox is a legitimate region to watch.
	gw, gh := grid, grid
	if b.Dx() < gw {
		gw = b.Dx()
	}
	if b.Dy() < gh {
		gh = b.Dy()
	}

	cells := make([][3]float64, gw*gh)
	counts := make([]int, gw*gh)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		cy := (y - b.Min.Y) * gh / b.Dy()
		for x := b.Min.X; x < b.Max.X; x++ {
			cx := (x - b.Min.X) * gw / b.Dx()
			r, gg, bb, _ := img.At(x, y).RGBA()
			i := cy*gw + cx
			cells[i][0] += float64(r >> 8)
			cells[i][1] += float64(gg >> 8)
			cells[i][2] += float64(bb >> 8)
			counts[i]++
		}
	}
	for i := range cells {
		if counts[i] == 0 {
			continue
		}
		n := float64(counts[i]) * 255
		cells[i][0] /= n
		cells[i][1] /= n
		cells[i][2] /= n
	}

	f := Fingerprint{
		Cells: cells, GridW: gw, GridH: gh,
		Width: b.Dx(), Height: b.Dy(),
	}
	f.Digest = digest(cells)
	return f, nil
}

// Compare classifies the difference between two fingerprints.
func (g *GridFingerprinter) Compare(before, after Fingerprint) ChangeResult {
	switch {
	case before.Empty() || after.Empty():
		// Unknown. Reported as MEANINGFUL, because every caller of this uses it to
		// decide whether repeating an action is safe, and the safe assumption when the
		// evidence is missing is that the action landed.
		return ChangeResult{
			Kind: ChangeMeaningful, ChangedCells: 1,
			Reason: "one of the regions could not be fingerprinted, so no comparison is " +
				"possible — treated as changed, because the alternative permits a retry",
		}
	case before.GridW != after.GridW || before.GridH != after.GridH,
		before.Width != after.Width || before.Height != after.Height:
		// The region resized. Comparing averages over different areas would produce a
		// number that means nothing; a window that resized has visibly changed anyway.
		return ChangeResult{
			Kind: ChangeMeaningful, ChangedCells: 1,
			Reason: fmt.Sprintf("the region changed size (%dx%d → %dx%d), which is itself "+
				"a visible change", before.Width, before.Height, after.Width, after.Height),
		}
	}

	th := g.thresholds()
	changed, maxDelta := 0, 0.0
	for i := range before.Cells {
		d := cellDistance(before.Cells[i], after.Cells[i])
		if d > maxDelta {
			maxDelta = d
		}
		if d >= th.CellDelta {
			changed++
		}
	}
	total := float64(len(before.Cells))
	if total == 0 {
		return ChangeResult{Kind: ChangeIdentical, Reason: "the region is empty"}
	}
	fraction := float64(changed) / total

	res := ChangeResult{ChangedCells: fraction, MaxCellDelta: maxDelta}
	switch {
	case changed == 0 && maxDelta == 0:
		res.Kind = ChangeIdentical
		res.Reason = "the region is pixel-for-pixel what it was"
	case fraction < th.MinorCells:
		res.Kind = ChangeMinor
		res.Reason = fmt.Sprintf("%.1f%% of the region differs — below the %.0f%% noise "+
			"floor, which is where a blinking caret, a moving cursor and a ticking clock live",
			fraction*100, th.MinorCells*100)
	case fraction < th.MeaningfulCells:
		res.Kind = ChangeMinor
		res.Reason = fmt.Sprintf("%.1f%% of the region differs — a change, but too small "+
			"to be confident something happened", fraction*100)
	default:
		res.Kind = ChangeMeaningful
		res.Reason = fmt.Sprintf("%.1f%% of the region differs", fraction*100)
	}
	return res
}

func (g *GridFingerprinter) thresholds() Thresholds {
	if g == nil || g.Thresholds.Grid == 0 {
		return DefaultThresholds()
	}
	return g.Thresholds
}

// cellDistance is how different two cell colours are, 0..1.
//
// Mean absolute difference across channels rather than Euclidean: it is linear in the
// thing a person perceives as "how different", and it does not let one saturated
// channel dominate the way a squared measure does.
func cellDistance(a, b [3]float64) float64 {
	d := math.Abs(a[0]-b[0]) + math.Abs(a[1]-b[1]) + math.Abs(a[2]-b[2])
	return d / 3
}

// digest is a short stable string for a cell grid.
//
// FNV-1a over quantised cells. Quantised so that a digest is stable across
// imperceptible differences — two captures of a static screen should produce the same
// string, or the digest would be useless for the equality check it exists for.
func digest(cells [][3]float64) string {
	const offset64 = 14695981039346656037
	const prime64 = 1099511628211

	h := uint64(offset64)
	for _, c := range cells {
		for _, v := range c {
			q := byte(math.Round(v * 63)) // 64 levels per channel
			h ^= uint64(q)
			h *= prime64
		}
	}
	return fmt.Sprintf("%016x", h)
}
