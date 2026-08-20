package visionbench_test

import (
	"context"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// The v2 corpus, measured. Full-resolution, ordered, with declared ground truth.
//
// Everything the classical milestone could only guess at is answerable here, and the answers
// are allowed to contradict the synthetic ones — a finding on 1920x1080 game frames beats a
// finding on a drawn rectangle.
//
//	MARCO_V2=1 go test ./internal/director/visionbench/ -run V2 -v

const v2Root = "../../../fixtures/vision/v2/rocketleague"

// loadV2 reads every sequence: frames, their bounds, and their truth.
func loadV2(t *testing.T) ([]visionbench.FrameTruth, map[string]image.Image, map[string]image.Rectangle) {
	t.Helper()
	dirs, err := os.ReadDir(v2Root)
	if err != nil {
		t.Skipf("no v2 corpus: %v", err)
	}
	var truths []visionbench.FrameTruth
	images := map[string]image.Image{}
	bounds := map[string]image.Rectangle{}

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		dir := filepath.Join(v2Root, d.Name())
		seq, err := visionbench.LoadTruth(dir)
		if err != nil {
			t.Fatalf("%s: %v", d.Name(), err)
		}
		truths = append(truths, seq...)
		paths, _ := filepath.Glob(filepath.Join(dir, "*.png"))
		sort.Strings(paths)
		for _, p := range paths {
			f, err := os.Open(p)
			if err != nil {
				t.Fatal(err)
			}
			img, _, err := image.Decode(f)
			_ = f.Close()
			if err != nil {
				t.Fatal(err)
			}
			name := filepath.Base(p)
			name = name[:len(name)-len(".png")]
			// SEQUENCE-scoped, matching FrameTruth.Key(): four boundary frames appear in
			// two sequences each, and a basename key silently aliased them.
			key := d.Name() + "/" + name
			images[key] = img
			bounds[key] = img.Bounds()
		}
	}
	if len(truths) == 0 {
		t.Skip("the v2 corpus is empty")
	}
	return truths, images, bounds
}

// runBackend detects over every frame and reports metrics plus median latency.
func runBackend(t *testing.T, c *visionbench.Classical, truths []visionbench.FrameTruth,
	images map[string]image.Image, bounds map[string]image.Rectangle) (
	visionbench.TruthMetrics, time.Duration) {

	t.Helper()
	byFrame := map[string][]visionbench.Detection{}
	var lat []time.Duration
	for _, ft := range truths {
		img, ok := images[ft.Key()]
		if !ok {
			continue
		}
		start := time.Now()
		dets, err := c.Detect(context.Background(), img)
		lat = append(lat, time.Since(start))
		if err != nil {
			t.Fatalf("%s: %v", ft.Frame, err)
		}
		byFrame[ft.Key()] = dets
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	median := time.Duration(0)
	if len(lat) > 0 {
		median = lat[len(lat)/2]
	}
	return visionbench.EvaluateTruth(byFrame, bounds, truths), median
}

func header() {
	fmt.Printf("\n%-30s %6s %6s %6s %6s %6s %6s %6s %6s %8s %7s\n",
		"CONFIGURATION", "DETS", "PREC", "RECALL", "N-PREC", "N-REC", "T-PREC", "T-REC",
		"OCR-P", "MEDIAN", "SCOREV2")
}

func report(name string, m visionbench.TruthMetrics, median time.Duration) {
	s, ok := visionbench.ScoreV2(m, median, visionbench.DefaultWeightsV2())
	score := "n/a"
	if ok {
		score = fmt.Sprintf("%.1f", s.Total)
	}
	fmt.Printf("   %-27s tp=%d fp=%d unmatched=%d truth=%d matched=%d\n",
		name, m.TruePos, m.FalsePos, m.Unmatched, m.TruthRegions, m.Matched)
	fmt.Printf("%-30s %6d %5.0f%% %5.0f%% %5.0f%% %5.0f%% %5.0f%% %5.0f%% %5.0f%% %8s %7s\n",
		name, m.Detections, m.Precision*100, m.Recall*100,
		m.NameablePrecision*100, m.NameableRecall*100,
		m.TemporalPrecision*100, m.TemporalRecall*100, m.OCRPrecision*100,
		median.Round(time.Microsecond), score)
}

// TestV2SizeFloor reruns the size-floor question on real full-frame geometry.
//
// The classical milestone could not settle it: 0.045 scored well on crops and rejected 79 of
// 83 candidates on a real screen, and 0.015 was chosen as the value safe for a real control
// with no corpus able to confirm it. This corpus can.
func TestV2SizeFloor(t *testing.T) {
	if os.Getenv("MARCO_V2") == "" {
		t.Skip("set MARCO_V2=1")
	}
	truths, images, bounds := loadV2(t)
	header()
	for _, tc := range []struct {
		name string
		ab   visionbench.Ablations
	}{
		{"no size filter", visionbench.Ablations{NoSizeFilter: true}},
		{"shipping (0.015)", visionbench.Ablations{}},
	} {
		c := visionbench.NewClassical()
		c.Ablations = tc.ab
		m, med := runBackend(t, c, truths, images, bounds)
		report(tc.name, m, med)
	}
}

// TestV2Stride reruns the stride sweep on real evidence.
//
// The synthetic result was stride 8 → 4 of 8 controls, stride 4 → 0 of 8, multi-offset → 8 of
// 8. Whether that survives on real game frames is exactly the kind of thing a synthetic test
// cannot promise.
func TestV2Stride(t *testing.T) {
	if os.Getenv("MARCO_V2") == "" {
		t.Skip("set MARCO_V2=1")
	}
	truths, images, bounds := loadV2(t)
	header()
	for _, tc := range []struct {
		name    string
		stride  int
		offsets []int
	}{
		{"stride 8 (shipping)", 8, nil},
		{"stride 4", 4, nil},
		{"stride 8, offsets 0/4", 8, []int{0, 4}},
	} {
		c := visionbench.NewClassical()
		c.Stride, c.Offsets = tc.stride, tc.offsets
		m, med := runBackend(t, c, truths, images, bounds)
		report(tc.name, m, med)
	}
}

// TestV2PathologicalBackends checks ScoreV2 ranks known-bad detectors sensibly on REAL
// evidence, not only on the synthetic sequences it was designed against.
func TestV2PathologicalBackends(t *testing.T) {
	if os.Getenv("MARCO_V2") == "" {
		t.Skip("set MARCO_V2=1")
	}
	truths, _, bounds := loadV2(t)

	// A box spammer: a grid of rectangles everywhere, all called buttons. High coverage,
	// no idea what anything is.
	spam := map[string][]visionbench.Detection{}
	// A perfect-but-narrow detector: the pause panel only, correctly.
	narrow := map[string][]visionbench.Detection{}
	for _, ft := range truths {
		fb, ok := bounds[ft.Key()]
		if !ok {
			continue
		}
		var s []visionbench.Detection
		for gx := range 8 {
			for gy := range 5 {
				r := visionbench.NormRect{
					X: float64(gx) * 0.125, Y: float64(gy) * 0.2, W: 0.12, H: 0.19,
				}
				s = append(s, visionbench.Detection{Label: "button", Bounds: r.Pixels(fb)})
			}
		}
		spam[ft.Key()] = s
		for _, r := range ft.Regions {
			if r.Identity == "pause_panel" {
				narrow[ft.Key()] = []visionbench.Detection{
					{Label: "panel", Bounds: r.Bounds.Pixels(fb)},
				}
			}
		}
	}

	sm := visionbench.EvaluateTruth(spam, bounds, truths)
	nm := visionbench.EvaluateTruth(narrow, bounds, truths)
	ss, _ := visionbench.ScoreV2(sm, 0, visionbench.DefaultWeightsV2())
	ns, _ := visionbench.ScoreV2(nm, 0, visionbench.DefaultWeightsV2())

	header()
	report("box spammer", sm, 0)
	report("narrow but correct", nm, 0)

	if sm.NameablePrecision >= 0.5 {
		t.Errorf("the box spammer kept %.0f%% nameable precision on real frames",
			sm.NameablePrecision*100)
	}
	if ss.Total >= ns.Total {
		t.Errorf("the box spammer scored %.1f against a narrow correct detector's %.1f",
			ss.Total, ns.Total)
	}
}
