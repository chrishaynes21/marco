package main

// Does what a control LOOKS like add anything to where it is?
//
// # The one case this is allowed to examine
//
// [[Experiment-034-two-controls-and-why-each-was-lost]] separated two failures. One was evidence
// coverage and is fixed. The other is `control_6`: a 170x14 label whose 352x34 button contains
// it, mutually centre-containing, so no proposal can tell the two apart and the matcher refuses
// on every look. Geometry has said all it can.
//
// So the question is narrow: **where the geometric matcher ALREADY refuses for indistinctness,
// does appearance separate the control from its container?** Appearance may not create a
// candidate, may not rescue an unobservable result, and may not weaken a refusal. It is examined
// only on the looks geometry has already given up on.
//
// # How a claim about appearance is kept honest
//
// The hard part of an experiment like this is that it is trivially easy to confirm. A descriptor
// compared against the thing it was computed from will always agree.
//
// So every comparison here is CROSS-FOLD — the reference crops come from fold A's looks and the
// test crops from fold B's — and the verdict appearance gives is scored against a ground truth it
// never sees: which of the two stored rectangles the proposal actually overlaps more, by area.
// Appearance is right when it recovers what overlap already knew, and the interesting number is
// how often it does that on the looks where mutual centre containment could not.
//
// # And one thing a correct answer still would not establish
//
// `control_6`'s stored reading is a LABEL. Appearance agreeing that a proposal is that label does
// not say the label is the clickable control, and the button around it is the thing a person
// pressed. A descriptor that resolved the ambiguity would tell Marco which rectangle it is
// looking at; it would not promote a run of text into a button.

import (
	"fmt"
	"image"
	"image/draw"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

// descriptor is one region's appearance, with where it came from.
type descriptor struct {
	look int
	rect observe.Region
	hist screen.Histogram
	ok   bool
}

// describeCrop lifts a normalised rectangle out of a frame and describes it.
//
// Refuses a crop too small to describe: a 2x2 patch has a histogram, and it is noise with the
// shape of evidence.
func describeCrop(img image.Image, r observe.Region, look int) descriptor {
	b := img.Bounds()
	rect := image.Rect(
		b.Min.X+int(r.X*float64(b.Dx())),
		b.Min.Y+int(r.Y*float64(b.Dy())),
		b.Min.X+int((r.X+r.Width)*float64(b.Dx())),
		b.Min.Y+int((r.Y+r.Height)*float64(b.Dy())),
	).Intersect(b)
	// A CROP TOO SMALL TO DESCRIBE IS REFUSED. A 2x2 patch has a histogram, and it is
	// noise with the shape of evidence.
	if rect.Dx() < 4 || rect.Dy() < 4 {
		return descriptor{look: look, rect: r}
	}
	out := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(out, out.Bounds(), img, rect.Min, draw.Src)
	return descriptor{look: look, rect: r, hist: screen.ColorHistogram(out), ok: true}
}

// similarity is histogram intersection, or -1 when either side could not be described.
func similarity(a, b descriptor) float64 {
	if !a.ok || !b.ok {
		return -1
	}
	return screen.HistIntersection(a.hist, b.hist)
}

// ambiguousCase is one look on which geometry refused a control for indistinctness.
type ambiguousCase struct {
	look int
	// cand is the proposal that agreed with the reading AND with a container.
	cand observe.Region
	// simReading and simContainer are its appearance against the fold-A references,
	// or -1 when either side could not be described.
	simReading, simContainer float64
}

// appearanceTrial is one control's whole account.
type appearanceTrial struct {
	id        observe.VisualTargetID
	screen    observe.VisualScreenID
	reading   observe.Region
	container observe.Region
	cases     []ambiguousCase

	// refScreen is the fold-A screen the references were cropped from, and refLooks
	// how many looks it stood for. Printed, because a reference taken from the
	// wrong screen produces a confident number about nothing.
	refScreen observe.VisualScreenID
	refLooks  int
	// refReading and refContainer are the reference crops, from fold A.
	refReading, refContainer []descriptor
}

// stability is the spread of one region's descriptor across the looks it was cropped from.
//
// The number that says whether a descriptor means anything at all. A control whose appearance
// changes more between two looks of its own screen than it differs from its container has no
// identity to contribute, however well one pair of crops happens to score.
func stability(d []descriptor) (lo, hi, mean float64, n int) {
	lo = 1
	var sum float64
	for i := range d {
		for j := i + 1; j < len(d); j++ {
			s := similarity(d[i], d[j])
			if s < 0 {
				continue
			}
			n++
			sum += s
			if s < lo {
				lo = s
			}
			if s > hi {
				hi = s
			}
		}
	}
	if n == 0 {
		return 0, 0, 0, 0
	}
	return lo, hi, sum / float64(n), n
}

// truthByOverlap is which stored rectangle a proposal really is, by area overlap.
//
// The ground truth appearance is scored against, and it is deliberately a DIFFERENT question
// from the one the matcher asks: mutual centre containment holds for both rectangles at once,
// which is the whole problem, while overlap always prefers one.
func (t appearanceTrial) truthByOverlap(r observe.Region) string {
	if iou(r, t.reading) >= iou(r, t.container) {
		return "reading"
	}
	return "container"
}

// printAppearance reports whether appearance separated anything.
func printAppearance(trials []*appearanceTrial, neighbours []neighbourPair) {
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  DOES APPEARANCE SEPARATE A CONTROL FROM ITS CONTAINER?\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("  Only on looks where the geometric matcher ALREADY refused for\n")
	fmt.Printf("  indistinctness. References are cropped from fold A, tests from fold B,\n")
	fmt.Printf("  and the verdict is scored against which rectangle the proposal really\n")
	fmt.Printf("  overlaps more — a ground truth the descriptor never sees.\n")

	for _, t := range trials {
		fmt.Printf("\n  %s\n", t.id)
		fmt.Printf("    reading   %.4f,%.4f %.4fx%.4f\n",
			t.reading.X, t.reading.Y, t.reading.Width, t.reading.Height)
		fmt.Printf("    container %.4f,%.4f %.4fx%.4f   (overlap %.3f)\n",
			t.container.X, t.container.Y, t.container.Width, t.container.Height,
			iou(t.reading, t.container))
		if len(t.cases) == 0 {
			fmt.Printf("    no look refused this control for indistinctness\n")
			continue
		}

		fmt.Printf("    references from %s (%d look(s) in fold A)\n",
			t.refScreen, t.refLooks)
		// STABILITY FIRST. A descriptor that is not stable cannot discriminate.
		for _, r := range []struct {
			name string
			d    []descriptor
		}{{"reading", t.refReading}, {"container", t.refContainer}} {
			lo, hi, mean, n := stability(r.d)
			if n == 0 {
				fmt.Printf("    %-9s stability: not enough crops\n", r.name)
				continue
			}
			fmt.Printf("    %-9s stability across %d fold-A crop(s): "+
				"%.3f-%.3f, mean %.3f\n", r.name, len(r.d), lo, hi, mean)
			if r.name == "reading" {
				// THE SHAPE OF THE INSTABILITY, not only its range. A
				// descriptor that steps between two values is reading a
				// STATE — a control that is sometimes selected — while one
				// that drifts is reading noise. Different problems, and only
				// one of them has an obvious fix.
				fmt.Printf("      against the first crop:")
				for k := 1; k < len(r.d) && k < 10; k++ {
					fmt.Printf(" %.2f", similarity(r.d[0], r.d[k]))
				}
				fmt.Printf("\n")
			}
		}

		// THE SEPARATION ITSELF.
		var agreed, disagreed, undecided int
		var margins []float64
		for _, c := range t.cases {
			sr, sc := c.simReading, c.simContainer
			if sr < 0 || sc < 0 {
				undecided++
				continue
			}
			margins = append(margins, sr-sc)
			said := "container"
			if sr > sc {
				said = "reading"
			}
			if sr == sc {
				undecided++
				continue
			}
			if said == t.truthByOverlap(c.cand) {
				agreed++
			} else {
				disagreed++
			}
		}
		sort.Float64s(margins)
		fmt.Printf("    %d indistinct look(s): appearance agreed with overlap on %d, "+
			"disagreed on %d, could not say on %d\n",
			len(t.cases), agreed, disagreed, undecided)
		if len(margins) > 0 {
			fmt.Printf("    margin (reading - container): min %+.3f  median %+.3f  "+
				"max %+.3f\n", margins[0], margins[len(margins)/2],
				margins[len(margins)-1])
			if margins[0] > 0 || margins[len(margins)-1] < 0 {
				fmt.Printf("    -> SEPARATED: every margin has the same sign\n")
			} else {
				fmt.Printf("    -> NOT SEPARATED: the margin changes sign, so no\n")
				fmt.Printf("       threshold divides these cases\n")
			}
		}
	}

	// ── THE NEGATIVE CASE ──
	if len(neighbours) > 0 {
		fmt.Printf("\n  %s\n", repeat("-", 74))
		fmt.Printf("  AND WHAT IT MUST NOT SEPARATE: NEIGHBOURING CONTROLS\n")
		fmt.Printf("  %s\n", repeat("-", 74))
		fmt.Printf("  Rows of one menu are the same button art with different words on\n")
		fmt.Printf("  them. If a colour histogram calls two of them different, it is\n")
		fmt.Printf("  reading noise; if it calls them the same, it cannot be used to\n")
		fmt.Printf("  choose between them. Either way it is a limit worth knowing.\n\n")
		var sims []float64
		for _, n := range neighbours {
			sims = append(sims, n.sim)
		}
		sort.Float64s(sims)
		fmt.Printf("    %d neighbouring pair(s): min %.3f  median %.3f  max %.3f\n",
			len(sims), sims[0], sims[len(sims)/2], sims[len(sims)-1])
	}
}

// neighbourPair is two different controls on one screen, compared.
type neighbourPair struct {
	a, b observe.Region
	sim  float64
}
