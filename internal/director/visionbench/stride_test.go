package visionbench_test

import (
	"context"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// Does the 8px grid cost real recall, and would a finer one be worth it?
//
// Raised as a known gap by the classical milestone and deliberately not acted on: an 8px scan
// cannot resolve a 16px control at a non-multiple-of-8 offset, because the control merges
// into the surrounding colour run. Whether that matters is a measurement on FULL-RESOLUTION
// frames, and the legacy corpus is crops.
//
// Two questions, kept apart: what does stride cost in recall on synthetic controls whose
// offsets are known, and what does it cost in time on a real 1920x1080 screen.
//
//	MARCO_SHADOW_FRAME=/path/to.png go test ./internal/director/visionbench/ -run Stride -v

// TestStrideRecallAgainstKnownOffsets is the controlled half: controls placed at every
// offset relative to the grid, so the miss rate is exactly attributable.
func TestStrideRecallAgainstKnownOffsets(t *testing.T) {
	// Eight 100x18 controls, each shifted one more pixel off the 8px grid. A real menu
	// row is about this size at 1080p, and its vertical position is arbitrary.
	build := func() (image.Image, []image.Rectangle) {
		img := scene(1920, 1080, dark)
		var want []image.Rectangle
		for i := range 8 {
			y := 200 + i*90 + i // +i walks the control off the grid
			fill(img, 400, y, 100, 18, light)
			want = append(want, image.Rect(400, y, 500, y+18))
		}
		return img, want
	}
	img, want := build()

	type row struct {
		name    string
		stride  int
		offsets []int
	}
	rows := []row{
		{"stride 8 (shipping)", 8, nil},
		{"stride 4", 4, nil},
		{"stride 8, offsets 0/4", 8, []int{0, 4}},
	}

	fmt.Printf("\n%-22s %10s %10s %12s\n", "CONFIGURATION", "FOUND", "DETECTIONS", "TIME")
	found := map[string]int{}
	for _, r := range rows {
		c := visionbench.NewClassical()
		c.Stride, c.Offsets = r.stride, r.offsets
		start := time.Now()
		got, err := c.Detect(context.Background(), img)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("%s: %v", r.name, err)
		}
		hit := 0
		for _, w := range want {
			for _, d := range got {
				if overlaps(d.Bounds, w) {
					hit++
					break
				}
			}
		}
		found[r.name] = hit
		fmt.Printf("%-22s %8d/%d %10d %12s\n", r.name, hit, len(want), len(got),
			elapsed.Round(time.Microsecond))
	}

	// # What this measured, which is not what was assumed
	//
	// The classical milestone recorded "the 8px grid may miss a 16px control at an
	// unfavourable offset" and named a finer stride as the obvious fix. Measured, a finer
	// stride is the WORST option of the three: it finds nothing at all, and costs three
	// times the time.
	//
	// The reason is an interaction nobody had looked for. MinSide is a floor in PIXELS
	// applied to `cells x stride`, so at stride 4 an 18px control resolves to 4 cells =
	// 16px and is refused, while at stride 8 the same control over-merges with its
	// surroundings into a blob big enough to pass. Half of stride 8's apparent recall is
	// that accident.
	//
	// Multi-offset finds every control, at two thirds of stride 4's cost, because it
	// changes WHERE the grid lands rather than how fine it is. That is the answer to the
	// open question, and it is the opposite of the intuition.
	if found["stride 8, offsets 0/4"] <= found["stride 8 (shipping)"] {
		t.Errorf("multi-offset found %d controls against single-offset %d; sampling the "+
			"grid at two origins is the measured fix for the quantisation gap",
			found["stride 8, offsets 0/4"], found["stride 8 (shipping)"])
	}
	if found["stride 4"] > found["stride 8, offsets 0/4"] {
		t.Errorf("a finer stride (%d) beat multi-offset (%d); the recommendation in the "+
			"experiment record no longer holds",
			found["stride 4"], found["stride 8, offsets 0/4"])
	}
}

func overlaps(a, b image.Rectangle) bool {
	i := a.Intersect(b)
	if i.Empty() {
		return false
	}
	return float64(i.Dx()*i.Dy())/float64(b.Dx()*b.Dy()) > 0.3
}

// TestStrideCostOnARealScreen is the latency half, on evidence the corpus cannot provide.
func TestStrideCostOnARealScreen(t *testing.T) {
	path := os.Getenv("MARCO_SHADOW_FRAME")
	if path == "" {
		t.Skip("set MARCO_SHADOW_FRAME to a full-resolution capture")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("cannot open the frame: %v", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}
	b := img.Bounds()

	fmt.Printf("\n%-22s %12s %12s %12s\n", "CONFIGURATION", "CANDIDATES", "KEPT", "MEDIAN")
	for _, r := range []struct {
		name    string
		stride  int
		offsets []int
	}{
		{"stride 8 (shipping)", 8, nil},
		{"stride 4", 4, nil},
		{"stride 8, offsets 0/4", 8, []int{0, 4}},
	} {
		c := visionbench.NewClassical()
		c.Stride, c.Offsets = r.stride, r.offsets
		// Median of a few runs: a single timing on a desktop under load is noise.
		var best time.Duration
		var kept int
		for i := range 5 {
			start := time.Now()
			got, err := c.Detect(context.Background(), img)
			el := time.Since(start)
			if err != nil {
				t.Fatalf("%s: %v", r.name, err)
			}
			if i == 0 || el < best {
				best = el
			}
			kept = len(got)
		}
		cands := len(c.Explain(img))
		fmt.Printf("%-22s %12d %12d %12s   (%dx%d)\n",
			r.name, cands, kept, best.Round(time.Microsecond), b.Dx(), b.Dy())
	}
}
