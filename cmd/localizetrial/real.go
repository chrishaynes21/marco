package main

// THE SAME METHOD, POINTED AT REAL PIXELS.
//
//	localizetrial -real <taught.png> -then <later.png> -click x,y
//
// The fixture answered the question on a layout built for it. This asks the next one on a
// layout nobody built: a real game menu, captured twice with different things happening behind
// it. It demonstrates a click at a point in the first image and asks both methods to find it in
// the second, printing every candidate peak so a reader can see what the method had to choose
// between rather than being told the conclusion.
//
// Nothing is committed. The images are whoever's they are, they stay where they are, and this
// reads them.
//
// # What this cannot be
//
// Not a second evaluation. Two images are two cases, and the frozen parameters were calibrated
// against an opaque fixture — whether they transfer is exactly the question, and the numbers
// below are the measurement rather than a pass or a fail.

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/localizetrial"
)

// probeReal runs one demonstration in `taught` and locates it in `later`.
func probeReal(taughtPath, laterPath string, click image.Point, keep image.Point) error {
	taught, err := loadRGBA(taughtPath)
	if err != nil {
		return err
	}
	later, err := loadRGBA(laterPath)
	if err != nil {
		return err
	}
	if taught.Bounds() != later.Bounds() {
		return fmt.Errorf("%s is %v and %s is %v; these are not two looks at "+
			"one window", taughtPath, taught.Bounds().Size(), laterPath,
			later.Bounds().Size())
	}

	before := localizetrial.Look{Client: taught, On: taught.Bounds(), Scale: 1}
	after := localizetrial.Look{Client: later, On: later.Bounds(), Scale: 1}
	target := localizetrial.Demonstrate(before, click,
		localizetrial.Instance{Panel: "real", Button: "demonstrated"}, keep)

	fmt.Printf("REAL PIXELS\n  taught in  %s\n  located in %s\n"+
		"  click      %v\n  region     %v\n  frozen     accept %.2f, "+
		"tolerance %dpx, margin %.2f\n\n", taughtPath, laterPath, click,
		target.Region(), localizetrial.Frozen.Accept,
		localizetrial.Frozen.Tolerance, localizetrial.Frozen.Margin)

	// ── WHAT THE TEMPLATE SCORES WHERE IT WAS TAUGHT, IN THE LATER LOOK ──
	//
	// The number that decides whether the frozen threshold transfers at all. A
	// translucent control over a world that has moved is the same control with
	// different pixels behind it, and this says how different.
	same := localizetrial.ScoreAt(target, after, click)
	fmt.Printf("  the demonstrated position, in the later look:  %.4f%s\n",
		same, verdictOn(same))

	// ── AND EVERY OTHER PEAK IN THE WINDOW, WHICH IS WHAT IT HAD TO BEAT ──
	peaks := localizetrial.Peaks(target, after, 12)
	fmt.Printf("\n  the ten best matches anywhere in the window:\n")
	sort.SliceStable(peaks, func(i, j int) bool { return peaks[i].Score > peaks[j].Score })
	for i, p := range peaks {
		if i >= 10 {
			break
		}
		d := p.Click.Sub(click)
		mark := ""
		if abs(d.X) <= localizetrial.Frozen.Tolerance &&
			abs(d.Y) <= localizetrial.Frozen.Tolerance {
			mark = "   <- the demonstrated one"
		}
		fmt.Printf("    %.4f  at %-12v %4d,%-4d from the click%s\n",
			p.Score, p.Click, d.X, d.Y, mark)
	}

	fmt.Printf("\n  WHAT EACH METHOD DOES\n")
	for _, m := range localizetrial.Methods() {
		v := m.Run(target, after, localizetrial.Frozen)
		if v.Found {
			d := v.Chose.Sub(click)
			fmt.Printf("    %-14s chose %v, %d,%d from the click "+
				"(best %.4f, next %.4f)\n", m.Name, v.Chose,
				d.X, d.Y, v.Score, v.Runner)
			continue
		}
		fmt.Printf("    %-14s refused: %s (best %.4f, next %.4f)\n",
			m.Name, v.Why, v.Score, v.Runner)
	}
	return nil
}

func verdictOn(s float64) string {
	if s >= localizetrial.Frozen.Accept {
		return "   (above the frozen threshold)"
	}
	return "   (BELOW the frozen threshold — calibrated on an opaque fixture)"
}

func loadRGBA(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	out := image.NewRGBA(src.Bounds())
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			out.Set(x, y, src.At(x, y))
		}
	}
	return out, nil
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
