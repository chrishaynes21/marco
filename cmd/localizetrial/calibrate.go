package main

// CALIBRATING A GUARD FROM THE DEMONSTRATION, ON BOTH SIDES.
//
//	localizetrial -calibrate <dir of frames> -click x,y -region wxh [-band n]
//
// The recorder already watches a control over several looks while somebody demonstrates it. That
// is enough to decide what its template is worth, and a threshold decided there means something
// a constant cannot: it is about THIS control, at THIS capture's scale.
//
// # Why both sides, and not just how well it matches itself
//
//	positive self-matches alone can make the guard too permissive, so evaluate them against
//	nearby non-targets as well
//
// Exactly right, and the reason is visible in the numbers below: a control can match itself at
// 0.99 across every look and still have a lookalike two rows up that matches at 0.99 too. A
// threshold placed from the self-matches alone would accept both, and the guard would be
// confident about a control it cannot distinguish. So the number this prints is a SEPARATION —
// the worst the control scores against itself, minus the best anything else nearby scores — and
// a guard is only justified when that is positive, with the threshold placed inside the gap.
//
// A control with no separation is not a control to refuse at run time. It is one to SAY SO ABOUT
// while the person is still standing there, which is where the recorder already is.

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/localizetrial"
)

// calibrate measures a demonstrated control across every frame in a directory.
func calibrate(dir string, click, keep image.Point, band, by int) error {
	paths, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil {
		return err
	}
	sort.Strings(paths)
	if len(paths) < 2 {
		return fmt.Errorf("%s holds %d PNGs; a calibration needs several looks "+
			"at the same control", dir, len(paths))
	}

	first, err := loadRGBA(paths[0])
	if err != nil {
		return err
	}
	if by > 1 {
		// THE SAME PIXELS AT THE RESOLUTION A PREVIEW KEEPS. The click and
		// the region move with them, so what is measured is one control at
		// two scales and not two different questions.
		first = shrink(first, by)
		click = image.Pt(click.X/by, click.Y/by)
		keep = image.Pt(keep.X/by, keep.Y/by)
		band /= by
	}
	taught := localizetrial.Look{Client: first, On: first.Bounds(), Scale: 1}
	target := localizetrial.Demonstrate(taught, click,
		localizetrial.Instance{Panel: "real", Button: "demonstrated"}, keep)

	// WHAT COUNTS AS SOMEWHERE ELSE IS A FACT ABOUT THE CONTROL, not a constant.
	//
	// The fixture's 12px was half of its own small button. At full resolution a
	// 340px-wide template has a peak hundreds of pixels broad, and excluding only
	// twelve of them reports the SHOULDER OF THE SAME MATCH as the nearest rival —
	// which understates the hazard by measuring the control against itself. Half
	// the region in each direction is the smallest offset that lands elsewhere.
	apart := image.Pt(keep.X/2, keep.Y/2)
	fmt.Printf("CALIBRATION FROM THE DEMONSTRATION\n  frames     %d in %s\n"+
		"  size       %v\n  click      %v\n  region     %v  (%dx%d)\n"+
		"  band       %dpx around the click, which is where a guard could be "+
		"confused\n\n", len(paths), dir, first.Bounds().Size(), click,
		target.Region(), keep.X, keep.Y, band)

	fmt.Printf("  %-44s %8s %8s %8s\n", "FRAME", "itself", "nearest", "gap")
	worstSelf, bestOther := math.Inf(1), math.Inf(-1)
	var otherAt image.Point
	for _, p := range paths {
		img, err := loadRGBA(p)
		if err != nil {
			return err
		}
		if by > 1 {
			img = shrink(img, by)
		}
		if img.Bounds() != first.Bounds() {
			return fmt.Errorf("%s is %v and %s is %v; these are not looks at "+
				"one window", p, img.Bounds().Size(), paths[0],
				first.Bounds().Size())
		}
		look := localizetrial.Look{Client: img, On: img.Bounds(), Scale: 1}
		self := localizetrial.ScoreAt(target, look, click)

		// THE BEST THING NEARBY THAT IS NOT THE CONTROL. Searched in a band
		// around the demonstrated point, which is where the guard's own
		// tolerance could ever put it — not the whole screen, which at this
		// resolution would cost more than it tells.
		other, at := localizetrial.BestOther(target, look, click, band, apart)
		worstSelf = math.Min(worstSelf, self)
		if other > bestOther {
			bestOther, otherAt = other, at
		}
		fmt.Printf("  %-44s %8.4f %8.4f %8.4f\n", filepath.Base(p), self,
			other, self-other)
	}

	gap := worstSelf - bestOther
	fmt.Printf("\n  worst it ever matched itself      %.4f\n"+
		"  best anything else nearby ever    %.4f  at %v, %d,%d from the "+
		"click\n  SEPARATION                        %.4f\n\n", worstSelf,
		bestOther, otherAt, otherAt.X-click.X, otherAt.Y-click.Y, gap)

	switch {
	case gap <= 0:
		fmt.Printf("  NOT GUARDABLE BY APPEARANCE AT THIS REGION. Something " +
			"nearby matches this\n  control's own template as well as the " +
			"control does, across these looks.\n  No threshold separates " +
			"them, and a guard that picked one would be guessing.\n  The " +
			"answer is to say so while the person is still demonstrating — " +
			"a larger\n  region including what makes it different, or a " +
			"different control.\n")
		return nil
	case gap < 0.05:
		fmt.Printf("  THIN. A threshold fits, and only just: %.4f of room. "+
			"Worth saying out loud\n  at recording time rather than "+
			"discovering at run time.\n", gap)
	}
	accept := (worstSelf + bestOther) / 2
	fmt.Printf("  A THRESHOLD FOR THIS CONTROL, placed in the gap:  %.4f\n"+
		"  (the frozen fixture constant is %.2f, which is %s)\n",
		accept, localizetrial.Frozen.Accept, compare(worstSelf, bestOther))
	return nil
}

// compare says whether the fixture constant would have worked for this control.
//
// Judged against the GAP and not against the derived midpoint: a constant anywhere strictly
// inside the gap separates this control from its neighbours, and where in the gap it sits is not
// the question. Outside it, which side it falls on says which failure it would have produced.
func compare(worstSelf, bestOther float64) string {
	switch c := localizetrial.Frozen.Accept; {
	case c <= bestOther:
		return "TOO PERMISSIVE — it would accept the thing next to it"
	case c >= worstSelf:
		return "TOO STRICT — it would refuse this control"
	}
	return "inside the gap here, which is luck and not derivation"
}

func mustPoint(s, what string) image.Point {
	var x, y int
	if _, err := fmt.Sscanf(s, "%d,%d", &x, &y); err != nil {
		fmt.Fprintf(os.Stderr, "localizetrial: %s needs x,y\n", what)
		os.Exit(2)
	}
	return image.Pt(x, y)
}

func mustSize(s, what string) image.Point {
	var w, h int
	if _, err := fmt.Sscanf(s, "%dx%d", &w, &h); err != nil {
		fmt.Fprintf(os.Stderr, "localizetrial: %s needs wxh\n", what)
		os.Exit(2)
	}
	return image.Pt(w, h)
}

// shrink is a box-filter downscale by an integer factor.
//
// For asking one question and only one: what does the SAME control, in the SAME pixels, measure
// at the resolution a preview is kept at? Studio's preview path saves a 640x360 picture of a
// 1920x1080 window, and whether that is evidence a guard can be calibrated from is not a thing
// to assume in either direction.
func shrink(src *image.RGBA, by int) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx()/by, b.Dy()/by))
	for y := out.Bounds().Min.Y; y < out.Bounds().Max.Y; y++ {
		for x := out.Bounds().Min.X; x < out.Bounds().Max.X; x++ {
			var r, g, bl, n int
			for dy := 0; dy < by; dy++ {
				for dx := 0; dx < by; dx++ {
					c := src.RGBAAt(x*by+dx, y*by+dy)
					r, g, bl, n = r+int(c.R), g+int(c.G),
						bl+int(c.B), n+1
				}
			}
			out.SetRGBA(x, y, color.RGBA{uint8(r / n), uint8(g / n),
				uint8(bl / n), 0xff})
		}
	}
	return out
}
