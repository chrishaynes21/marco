// Package match is template matching over a picture: one correlation, one search, no policy.
//
// # What it is for
//
// A demonstration keeps a small picture of the control somebody clicked. Finding that control
// again — in a later look, or in a look taken a moment after the press — is this package's only
// job. It decides nothing: whether a score is good enough, where it is allowed to look and what
// to do when two places score alike are all somebody else's rules, and keeping them out of here
// is what lets the same arithmetic serve a recorder calibrating a control and an experiment
// measuring whether that calibration means anything.
//
// # Why zero-mean normalised cross-correlation
//
// Because the same control in two looks is rarely the same pixels. A menu drawn over a world
// that has moved, a banner that changed, a frame captured a hundred milliseconds later — all of
// them shift the brightness of a region slightly, and a plain difference would rank a dim copy
// of the right control below a bright patch of nothing. Zero-mean normalised cross-correlation
// is the ordinary answer and has one number a person can reason about: a score in [-1, 1] where
// 1 is "these are the same pixels".
//
// Measured on real game frames at 1920x1080: a control matches itself at 1.0000 across nine
// looks, and the sibling button forty-five pixels below it matches at 0.5962. See
// [[Experiment-097-a-translucent-menu-is-not-what-defeats-a-local-template]].
package match

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"sync"
)

// Gray is a picture as grey levels, which is what everything here works on.
//
// Rec. 601 luma, the ordinary one. Colour is dropped on purpose: two looks at one control differ
// in their colour far more than in their shape — a highlight, a hover, a background showing
// through — and a match that insisted on colour would refuse a control for being selected.
// THE ORIGIN IS KEPT. A Gray of a sub-rectangle has that rectangle as its bounds, not a
// zero-based copy of it, so every coordinate everywhere in this package is a coordinate in the
// picture it came from. A recorder that keeps a BAND around a control rather than a whole frame
// can then hand that band over and have it mean what it says, and a position in a result is a
// position in the window — which is the only kind that can be turned back into a place to press.
func Gray(src image.Image, r image.Rectangle) *image.Gray {
	r = r.Intersect(src.Bounds())
	out := image.NewGray(r)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := color.RGBAModel.Convert(src.At(x, y)).(color.RGBA)
			v := (299*int(c.R) + 587*int(c.G) + 114*int(c.B)) / 1000
			out.SetGray(x, y, color.Gray{Y: clamp(v)})
		}
	}
	return out
}

// Grid is a score for every top-left position in a rectangle.
//
// Dense rather than a map: a search over a 1920x1080 window scores a million and a half
// positions, and a map of those costs more than the arithmetic does.
type Grid struct {
	Over image.Rectangle
	At   []float64
}

// Get is the score at one position. Positions outside the grid are not scores and panic.
func (g Grid) Get(p image.Point) float64 {
	return g.At[(p.Y-g.Over.Min.Y)*g.Over.Dx()+(p.X-g.Over.Min.X)]
}

// Best is the highest score in the grid and where it is.
func (g Grid) Best() (image.Point, float64) {
	at, top := image.Point{}, math.Inf(-1)
	for y := g.Over.Min.Y; y < g.Over.Max.Y; y++ {
		for x := g.Over.Min.X; x < g.Over.Max.X; x++ {
			if s := g.Get(image.Pt(x, y)); s > top {
				at, top = image.Pt(x, y), s
			}
		}
	}
	return at, top
}

// BestAwayFrom is the highest score at least `apart` from a position, in both directions.
//
// # What counts as somewhere else is a fact about the control
//
// Positions a pixel apart are one match seen twice, not two things. How far apart two positions
// have to be before they are different places depends on how big the template is: a 340-pixel
// template has a peak hundreds of pixels broad, and excluding only a dozen of them reports the
// SHOULDER OF THE SAME MATCH as the nearest rival — which understates the hazard by measuring a
// control against itself. Half the template in each direction is the smallest offset that lands
// somewhere else, and callers that have a template pass exactly that.
func (g Grid) BestAwayFrom(of image.Point, apart image.Point) (image.Point, float64) {
	at, top := image.Point{}, math.Inf(-1)
	for y := g.Over.Min.Y; y < g.Over.Max.Y; y++ {
		for x := g.Over.Min.X; x < g.Over.Max.X; x++ {
			d := image.Pt(x, y).Sub(of)
			if abs(d.X) < apart.X && abs(d.Y) < apart.Y {
				continue
			}
			if s := g.Get(image.Pt(x, y)); s > top {
				at, top = image.Pt(x, y), s
			}
		}
	}
	return at, top
}

// Positions is the rectangle of top-left positions a template can occupy in a picture.
//
// Its own function because getting it wrong is a panic rather than a wrong answer, and three
// callers needed it.
func Positions(in, template image.Rectangle) image.Rectangle {
	return image.Rect(in.Min.X, in.Min.Y, in.Max.X-template.Dx()+1,
		in.Max.Y-template.Dy()+1)
}

// Search scores the template at every top-left position in `where`.
//
// `where` is clipped to the positions the template can actually occupy. An empty result means
// there was nowhere it could fit, which is a real answer and not an error: a template wider than
// the picture it is looked for in is a window that has changed shape.
func Search(in, template *image.Gray, where image.Rectangle) Grid {
	where = where.Intersect(Positions(in.Bounds(), template.Bounds()))
	if where.Empty() {
		return Grid{}
	}
	pw, ph := template.Bounds().Dx(), template.Bounds().Dy()
	n := float64(pw * ph)

	tb := template.Bounds()
	var mean float64
	for y := tb.Min.Y; y < tb.Max.Y; y++ {
		for x := tb.Min.X; x < tb.Max.X; x++ {
			mean += float64(template.GrayAt(x, y).Y)
		}
	}
	mean /= n
	diff := make([]float64, 0, pw*ph)
	var variance float64
	for y := tb.Min.Y; y < tb.Max.Y; y++ {
		for x := tb.Min.X; x < tb.Max.X; x++ {
			d := float64(template.GrayAt(x, y).Y) - mean
			diff = append(diff, d)
			variance += d * d
		}
	}
	norm := math.Sqrt(variance)

	out := Grid{Over: where, At: make([]float64, where.Dx()*where.Dy())}
	rows := make(chan int, where.Dy())
	for y := where.Min.Y; y < where.Max.Y; y++ {
		rows <- y
	}
	close(rows)
	workers := runtime.NumCPU()
	if workers > where.Dy() {
		workers = where.Dy()
	}
	if workers < 1 {
		workers = 1
	}
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// EACH WORKER OWNS WHOLE ROWS, so no two of them write the
			// same element and the slice needs no lock.
			for y := range rows {
				base := (y - where.Min.Y) * where.Dx()
				for x := where.Min.X; x < where.Max.X; x++ {
					out.At[base+x-where.Min.X] = at(in, diff,
						norm, x, y, pw, ph)
				}
			}
		}()
	}
	wg.Wait()
	return out
}

// At is the score of the template against the window whose top-left is `top`.
//
// For the one question that needs no search: how well does this control match where it is
// supposed to be. Returns 0 when the template cannot sit there at all, which is the same answer
// a flat window gets and for the same reason — there is nothing to compare.
func At(in, template *image.Gray, top image.Point) float64 {
	if !top.In(Positions(in.Bounds(), template.Bounds())) {
		return 0
	}
	g := Search(in, template, image.Rectangle{Min: top, Max: top.Add(image.Pt(1, 1))})
	if len(g.At) == 0 {
		return 0
	}
	return g.Get(top)
}

// at is one correlation. The caller has checked that the window fits.
func at(in *image.Gray, diff []float64, norm float64, x, y, pw, ph int) float64 {
	n := float64(pw * ph)
	var mean float64
	for j := 0; j < ph; j++ {
		row := in.PixOffset(x, y+j)
		for i := 0; i < pw; i++ {
			mean += float64(in.Pix[row+i])
		}
	}
	mean /= n
	var num, variance float64
	k := 0
	for j := 0; j < ph; j++ {
		row := in.PixOffset(x, y+j)
		for i := 0; i < pw; i++ {
			d := float64(in.Pix[row+i]) - mean
			num += d * diff[k]
			variance += d * d
			k++
		}
	}
	// A FLAT WINDOW CORRELATES WITH NOTHING. Zero variance on either side means
	// there is no shape to compare, and the honest score is zero rather than a
	// division by zero that would rank it above everything.
	if variance == 0 || norm == 0 {
		return 0
	}
	return num / (math.Sqrt(variance) * norm)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func clamp(v int) uint8 {
	switch {
	case v < 0:
		return 0
	case v > 255:
		return 255
	}
	return uint8(v)
}
