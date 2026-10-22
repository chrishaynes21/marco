package localizetrial

// THE TWO METHODS, WHICH DIFFER IN ONE THING ONLY: WHERE THEY LOOK.
//
// Same template, same correlation, same threshold, same ambiguity rule. One searches the whole
// client area; the other searches the neighbourhood the demonstrated click predicts. Everything
// else is held equal on purpose — a comparison where the methods also differ in their scoring
// would not be able to say which difference produced the result.
//
// # What a demonstration stores
//
//	the client area it was demonstrated in     (reference geometry)
//	the click, relative to that client area    (which instance was meant)
//	a small picture around the click           (what the control looks like)
//
// The point is AUTHORITATIVE ABOUT WHICH INSTANCE WAS TAUGHT. It is not a claim that the same
// instance still occupies that location, which is why a stored point alone is not enough and why
// the template is checked in the neighbourhood rather than simply clicked.
//
// # Why normalised correlation
//
// Because the fixture's noise and banner change the brightness of a region slightly, and a plain
// difference would rank a dim copy of the right control below a bright patch of nothing.
// Zero-mean normalised cross-correlation is the ordinary answer to that and has one parameter a
// person can reason about: a score in [-1, 1] where 1 is "these are the same pixels".

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"strconv"
	"sync"
)

// Target is what a demonstration stored about one control.
type Target struct {
	// Client is the client area's size at demonstration time, and Scale its display
	// scale. Both are part of the geometry a run has to find unchanged: the same
	// client-relative point in a client of another size is a different place.
	Client image.Point
	Scale  float64
	// At is the click, in client coordinates.
	At image.Point
	// Patch is the local region: the control's appearance as grey levels.
	Patch *image.Gray
	// Offset is where the patch's top-left sits relative to `At`, so a match at a
	// position can be turned back into a point to press.
	Offset image.Point
	// Shows is the instance that was demonstrated. The fixture's bookkeeping, kept
	// beside the record so a result can say whether it chose the right one; nothing
	// in either method reads it.
	Shows Instance
}

// Demonstrate stores a target from one look and one click.
//
// The patch is `region` wide and tall, centred on the click and clipped to the client area. This
// is the only place a region size is applied, so the frozen parameter is applied once.
func Demonstrate(l Look, at image.Point, shows Instance, region image.Point) Target {
	half := image.Pt(region.X/2, region.Y/2)
	r := image.Rectangle{Min: at.Sub(half), Max: at.Sub(half).Add(region)}
	r = r.Intersect(l.Client.Bounds())
	return Target{
		Client: l.Client.Bounds().Size(), Scale: l.Scale, At: at,
		Patch: grayOf(l.Client, r), Offset: r.Min.Sub(at), Shows: shows,
	}
}

// Rule is every parameter of the method, frozen before evaluation. See protocol.go.
type Rule struct {
	// Region is the local region kept at demonstration time.
	Region image.Point
	// Tolerance is how far from the predicted point the control may have moved and
	// still be the same control. Chosen from measured variation in the setup
	// examples, never widened to rescue an evaluation case.
	Tolerance int
	// Accept is the correlation a match needs to be believed at all.
	Accept float64
	// Margin decides ambiguity: a second peak, at least `Apart` pixels away from the
	// best one, scoring within this of it means the evidence does not distinguish
	// them and the answer is a refusal.
	Margin float64
	Apart  int
}

// Verdict is what a method answered.
type Verdict struct {
	// Chose is the point it would press, in client coordinates.
	Chose image.Point
	// Found says it answered at all. False is a refusal, and Why says which one.
	Found bool
	Why   string
	// Score is the best correlation it saw, and Runner the best one far enough away
	// from it to count as a rival.
	Score, Runner float64
}

// Whole is the baseline: match the template anywhere in the client area.
//
// It knows what the control looks like and nothing about where it was. With two instances that
// look alike it has two equally good answers and no way to prefer either — which is the thing
// the experiment is here to measure rather than to assert.
func Whole(t Target, l Look, r Rule) Verdict {
	area := l.Client.Bounds()
	return decide(t, l, r, area, "anywhere in the window")
}

// Local is the method under test: predict the point from the client geometry, then match the
// template only in the neighbourhood of the prediction.
//
// The geometry is checked FIRST and the search does not happen at all if it fails. That order is
// the rule, not an optimisation: a client of another size is not this layout, and a method that
// went looking anyway would be searching a screen it has no reason to believe anything about.
func Local(t Target, l Look, r Rule) Verdict {
	if got := l.Client.Bounds().Size(); got != t.Client {
		return Verdict{Why: "the window is " + size(got) + " and this was " +
			"demonstrated in " + size(t.Client) + ", which is a " +
			"different layout rather than the same one moved"}
	}
	if l.Scale != t.Scale {
		return Verdict{Why: "the display scale has changed since this was " +
			"demonstrated"}
	}
	// THE NEIGHBOURHOOD THE CLICK PREDICTS, and nothing outside it. No fallback
	// search: an answer found elsewhere would be a different instance of the same
	// control, which is precisely the mistake being measured.
	near := image.Rectangle{
		Min: t.At.Add(t.Offset).Sub(image.Pt(r.Tolerance, r.Tolerance)),
		Max: t.At.Add(t.Offset).Add(image.Pt(r.Tolerance+1, r.Tolerance+1)),
	}
	return decide(t, l, r, near.Add(image.Pt(0, 0)), "within "+
		itoa(r.Tolerance)+"px of where it was demonstrated")
}

// decide runs the correlation over the top-left positions in `where` and applies the rule.
//
// `where` is a rectangle of TOP-LEFT POSITIONS for the patch, not of pixels to look at: the
// window the patch covers extends past it by the patch's size, which is what lets the local
// search see a control that has moved by the tolerance.
func decide(t Target, l Look, r Rule, where image.Rectangle, scope string) Verdict {
	grey := grayOf(l.Client, l.Client.Bounds())
	pw, ph := t.Patch.Bounds().Dx(), t.Patch.Bounds().Dy()
	limit := image.Rect(0, 0, grey.Bounds().Dx()-pw+1, grey.Bounds().Dy()-ph+1)
	where = where.Intersect(limit)
	if where.Empty() {
		return Verdict{Why: "there is nowhere in " + scope + " that the " +
			"control could even fit"}
	}
	scores := correlate(grey, t.Patch, where)

	best, bestScore := scores.best()
	// THE BEST RIVAL IS THE BEST SCORE FAR ENOUGH AWAY TO BE A DIFFERENT THING.
	// Positions one pixel apart are the same match seen twice; `Apart` is what
	// separates "the peak is broad" from "there are two of these".
	runner := scores.bestAwayFrom(best, r.Apart)
	v := Verdict{Score: bestScore, Runner: runner}
	switch {
	case bestScore < r.Accept:
		v.Why = "nothing " + scope + " looks enough like the control that was " +
			"demonstrated"
		return v
	case runner >= bestScore-r.Margin:
		v.Why = "more than one thing " + scope + " looks like the control " +
			"that was demonstrated, and nothing here says which was meant"
		return v
	}
	v.Found = true
	// BACK TO A POINT TO PRESS: the match is a top-left, and what was stored is
	// where the click sat inside the patch.
	v.Chose = best.Sub(t.Offset)
	return v
}

// grid is a score for every top-left position in a rectangle.
//
// Dense rather than a map: the whole-window condition scores two hundred thousand positions per
// case and a map of those was most of the run time and all of the allocation.
type grid struct {
	Over image.Rectangle
	At   []float64
}

func (g grid) get(p image.Point) float64 {
	return g.At[(p.Y-g.Over.Min.Y)*g.Over.Dx()+(p.X-g.Over.Min.X)]
}

// best is the highest score and where it was.
func (g grid) best() (image.Point, float64) {
	at, top := image.Point{}, math.Inf(-1)
	for y := g.Over.Min.Y; y < g.Over.Max.Y; y++ {
		for x := g.Over.Min.X; x < g.Over.Max.X; x++ {
			if s := g.get(image.Pt(x, y)); s > top {
				at, top = image.Pt(x, y), s
			}
		}
	}
	return at, top
}

// bestAwayFrom is the highest score at least `apart` from a position in both directions.
//
// THE BEST RIVAL IS THE BEST SCORE FAR ENOUGH AWAY TO BE A DIFFERENT THING. Positions a pixel
// apart are one match seen twice; `apart` is what separates "the peak is broad" from "there are
// two of these".
func (g grid) bestAwayFrom(of image.Point, apart int) float64 {
	top := math.Inf(-1)
	for y := g.Over.Min.Y; y < g.Over.Max.Y; y++ {
		for x := g.Over.Min.X; x < g.Over.Max.X; x++ {
			d := image.Pt(x, y).Sub(of)
			if abs(d.X) < apart && abs(d.Y) < apart {
				continue
			}
			if s := g.get(image.Pt(x, y)); s > top {
				top = s
			}
		}
	}
	return top
}

// correlate is zero-mean normalised cross-correlation of the patch at every position in `where`.
//
// Spread across the available cores, because the whole-window condition searches every position
// in the client area and the experiment runs a hundred and ten cases. Nothing about the
// arithmetic changes with the number of workers, and the two conditions call this same function
// with different rectangles — which is the only difference between them.
func correlate(in, patch *image.Gray, where image.Rectangle) grid {
	pw, ph := patch.Bounds().Dx(), patch.Bounds().Dy()
	n := float64(pw * ph)
	var pmean float64
	for y := 0; y < ph; y++ {
		for x := 0; x < pw; x++ {
			pmean += float64(patch.GrayAt(x, y).Y)
		}
	}
	pmean /= n
	var pvar float64
	pd := make([]float64, 0, pw*ph)
	for y := 0; y < ph; y++ {
		for x := 0; x < pw; x++ {
			d := float64(patch.GrayAt(x, y).Y) - pmean
			pd = append(pd, d)
			pvar += d * d
		}
	}
	pnorm := math.Sqrt(pvar)

	out := grid{Over: where, At: make([]float64, where.Dx()*where.Dy())}
	workers := runtime.NumCPU()
	if workers > where.Dy() {
		workers = where.Dy()
	}
	if workers < 1 {
		workers = 1
	}
	var wg sync.WaitGroup
	rows := make(chan int, where.Dy())
	for y := where.Min.Y; y < where.Max.Y; y++ {
		rows <- y
	}
	close(rows)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// EACH WORKER OWNS WHOLE ROWS, so no two of them ever
			// write the same element and the slice needs no lock.
			for y := range rows {
				base := (y - where.Min.Y) * where.Dx()
				for x := where.Min.X; x < where.Max.X; x++ {
					out.At[base+x-where.Min.X] = at(in, pd,
						pnorm, x, y, pw, ph)
				}
			}
		}()
	}
	wg.Wait()
	return out
}

// at is the correlation of the patch with the window whose top-left is (x, y).
func at(in *image.Gray, pd []float64, pnorm float64, x, y, pw, ph int) float64 {
	n := float64(pw * ph)
	var mean float64
	for j := 0; j < ph; j++ {
		row := in.PixOffset(x, y+j)
		for i := 0; i < pw; i++ {
			mean += float64(in.Pix[row+i])
		}
	}
	mean /= n
	var num, wvar float64
	k := 0
	for j := 0; j < ph; j++ {
		row := in.PixOffset(x, y+j)
		for i := 0; i < pw; i++ {
			d := float64(in.Pix[row+i]) - mean
			num += d * pd[k]
			wvar += d * d
			k++
		}
	}
	// A FLAT WINDOW CORRELATES WITH NOTHING. Zero variance on either side means
	// there is no shape to compare, and the honest score is zero rather than a
	// division by zero that would rank it above everything.
	if wvar == 0 || pnorm == 0 {
		return 0
	}
	return num / (math.Sqrt(wvar) * pnorm)
}

func grayOf(src *image.RGBA, r image.Rectangle) *image.Gray {
	r = r.Intersect(src.Bounds())
	out := image.NewGray(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := src.RGBAAt(x, y)
			// Rec. 601 luma, the ordinary one.
			v := (299*int(c.R) + 587*int(c.G) + 114*int(c.B)) / 1000
			out.SetGray(x-r.Min.X, y-r.Min.Y, grayLevel(v))
		}
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// grayLevel is a luma value as a Gray colour.
func grayLevel(v int) color.Gray { return color.Gray{Y: clamp8(v)} }

// size is a client area written the way a refusal says it.
func size(p image.Point) string { return itoa(p.X) + "x" + itoa(p.Y) }

func itoa(v int) string { return strconv.Itoa(v) }

// Region is the rectangle the local region covers, in client coordinates.
//
// For drawing the evidence: what was kept is a fact about the record and a reader of a picture
// should be able to see it rather than take it from a comment.
func (t Target) Region() image.Rectangle {
	min := t.At.Add(t.Offset)
	return image.Rectangle{Min: min, Max: min.Add(t.Patch.Bounds().Size())}
}
