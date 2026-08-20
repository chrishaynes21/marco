package visionbench_test

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// The deterministic detector, at the properties that matter.
//
// Every test here corresponds to something measured on the reference corpus rather than to
// something the method might in principle do. The corpus's finding was blunt: 71% of the
// detector's output landed on frames declared to contain no interface, and one normalised
// size rule removes three quarters of that without losing a single true detection.

// scene paints a background and returns a canvas to draw interface onto.
func scene(w, h int, bg color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, bg)
		}
	}
	return img
}

func fill(img *image.RGBA, x0, y0, w, h int, c color.RGBA) {
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			img.Set(x, y, c)
		}
	}
}

var (
	dark  = color.RGBA{R: 20, G: 22, B: 30, A: 255}
	light = color.RGBA{R: 200, G: 205, B: 215, A: 255}
	mid   = color.RGBA{R: 110, G: 115, B: 125, A: 255}
)

func detect(t *testing.T, c *visionbench.Classical, img image.Image) []visionbench.Detection {
	t.Helper()
	got, err := c.Detect(context.Background(), img)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	return got
}

// ── candidate generation ──────────────────────────────────────────────────────

func TestAnObviousPanelIsFound(t *testing.T) {
	img := scene(640, 480, dark)
	fill(img, 100, 100, 300, 200, light)

	got := detect(t, visionbench.NewClassical(), img)
	if len(got) == 0 {
		t.Fatal("a 300x200 panel on a plain background was not found at all")
	}
}

func TestAnEmptyFrameDoesNotBecomeOneEnormousPanel(t *testing.T) {
	// The measured failure, in miniature: a plain field should yield nothing. On the real
	// corpus the equivalent frame produced 42 detections before the size rule.
	img := scene(640, 480, dark)
	if got := detect(t, visionbench.NewClassical(), img); len(got) != 0 {
		t.Fatalf("%d detections on a frame containing nothing", len(got))
	}
}

// TestSpeckledTextureIsIndistinguishableFromControls records the CEILING of this method.
//
// This is the milestone's central result, written as a test so it cannot quietly stop being
// true. Arena texture — many small patches of a slightly different colour — is the population
// that made up 65 of the corpus's 92 detections, and geometry alone cannot remove it.
//
// The reason is not a missing heuristic. A 24x24 uniform square IS a checkbox and IS a patch
// of texture, and at a size threshold safe for real controls at 1080p (see minNormSide) both
// survive. The only threshold that removes the texture also removes the checkbox. Rectangle
// geometry has no further information to appeal to.
//
// So this test asserts that the specks ARE kept. If a future change makes it fail, either
// something genuinely new is being measured — colour context, temporal stability, a learned
// classifier — or a threshold has been over-fitted to a corpus again.
func TestSpeckledTextureIsIndistinguishableFromControls(t *testing.T) {
	img := scene(960, 540, dark)
	for i := range 40 {
		fill(img, 20+(i%8)*110, 20+(i/8)*100, 24, 24, mid)
	}

	specks := 0
	for _, d := range detect(t, visionbench.NewClassical(), img) {
		if d.Bounds.Dx() <= 48 && d.Bounds.Dy() <= 48 {
			specks++
		}
	}
	if specks == 0 {
		t.Fatal("no texture speck survived, which means the size threshold is back above " +
			"the size of a real control — the over-fit this test exists to catch")
	}
}

// ── the size rule, and that it is normalised ──────────────────────────────────

func TestTheSizeRuleIsNormalisedNotPixelBased(t *testing.T) {
	// The same control, at two resolutions, must be treated the same way. A fixed pixel
	// floor tuned at 1080p silently changes meaning on a 720p screen or a crop, and the
	// corpus contains frames from 240x240 to 960x540.
	small := scene(320, 240, dark)
	fill(small, 40, 40, 60, 40, light)

	large := scene(1280, 960, dark)
	fill(large, 160, 160, 240, 160, light) // 4x the frame, 4x the control

	c := visionbench.NewClassical()
	gotSmall := len(detect(t, c, small))
	gotLarge := len(detect(t, c, large))
	if gotSmall == 0 || gotLarge == 0 {
		t.Fatalf("proportionally identical controls were not both found: small=%d large=%d",
			gotSmall, gotLarge)
	}
}

func TestDisablingTheSizeRuleRestoresTheNoise(t *testing.T) {
	// The ablation switch has to actually ablate, or the measured table in the classifier
	// documentation is unverifiable.
	img := scene(960, 540, dark)
	for i := range 40 {
		fill(img, 20+(i%8)*110, 20+(i/8)*100, 24, 24, mid)
	}
	c := visionbench.NewClassical()
	c.Ablations = visionbench.Ablations{NoSizeFilter: true}

	if got := detect(t, c, img); len(got) < 20 {
		t.Errorf("%d detections with the size rule disabled; it is not the rule doing "+
			"the work the ablation table credits it with", len(got))
	}
}

// ── roles ─────────────────────────────────────────────────────────────────────

func TestALongThinRegionIsABarNotAButton(t *testing.T) {
	img := scene(400, 300, dark)
	fill(img, 40, 24, 304, 24, light) // grid-aligned; aspect ~12.7

	for _, d := range detect(t, visionbench.NewClassical(), img) {
		if d.Bounds.Dx() > 200 && d.Label != "bar" {
			t.Errorf("a long thin strip was labelled %q, want bar", d.Label)
		}
	}
}

func TestNothingIsCalledAButtonWithoutControlLikeProportions(t *testing.T) {
	// The conservative asymmetry: calling a panel a button invites the Director to
	// believe it can be clicked, and that error is not symmetric with the reverse.
	img := scene(400, 300, dark)
	fill(img, 40, 40, 200, 200, light) // square, not control-shaped

	for _, d := range detect(t, visionbench.NewClassical(), img) {
		if d.Label == "button" {
			t.Errorf("a 200x200 square was called a button at %v", d.Bounds)
		}
	}
}

func TestClassicalNeverProducesText(t *testing.T) {
	// Privacy: this detector reads no pixels as characters and must never claim to. Text
	// and its withholding rules belong to scoped OCR.
	img := scene(400, 300, dark)
	fill(img, 40, 40, 160, 40, light)

	for _, d := range detect(t, visionbench.NewClassical(), img) {
		if d.Text != "" {
			t.Errorf("the classical detector produced text %q", d.Text)
		}
	}
}

// ── determinism ───────────────────────────────────────────────────────────────

func TestTheSameFrameAlwaysProducesIdenticalDetections(t *testing.T) {
	// Determinism is this backend's structural advantage over a learned model, and it is
	// easy to lose: an earlier version grouped alignment peers in map order and produced
	// two different scores for one fixture.
	img := scene(800, 600, dark)
	fill(img, 100, 100, 200, 48, light)
	fill(img, 100, 200, 200, 48, light)
	fill(img, 400, 100, 240, 300, mid)

	c := visionbench.NewClassical()
	first := detect(t, c, img)
	for i := range 20 {
		again := detect(t, c, img)
		if len(again) != len(first) {
			t.Fatalf("run %d produced %d detections, first produced %d",
				i, len(again), len(first))
		}
		for j := range first {
			if again[j] != first[j] {
				t.Fatalf("run %d differs at %d: %+v vs %+v", i, j, again[j], first[j])
			}
		}
	}
}

func TestTheRegionCapKeepsTheLargestRegions(t *testing.T) {
	// The output order is load-bearing exactly when the cap binds: the ceiling drops
	// whatever sorted last, and the documented intent is that the biggest regions — the
	// likeliest real interface — survive. Nothing tested this, and a mutation that
	// reordered the output passed the whole suite because the reference corpus never
	// reaches the cap.
	img := scene(1200, 900, dark)
	// One unmistakable panel, plus more medium regions than the cap allows.
	fill(img, 500, 400, 400, 300, light)
	for i := range 30 {
		fill(img, 10+(i%6)*190, 10+(i/6)*70, 120, 56, mid)
	}

	c := visionbench.NewClassical()
	c.MaxRegions = 5
	got := detect(t, c, img)
	if len(got) != 5 {
		t.Fatalf("%d detections with a cap of 5", len(got))
	}
	for i := 1; i < len(got); i++ {
		prev := got[i-1].Bounds.Dx() * got[i-1].Bounds.Dy()
		cur := got[i].Bounds.Dx() * got[i].Bounds.Dy()
		if cur > prev {
			t.Fatalf("detection %d (%d px) is larger than %d (%d px); the cap is not "+
				"keeping the largest regions", i, cur, i-1, prev)
		}
	}
}

// ── explanations ──────────────────────────────────────────────────────────────

func TestEveryRefusedCandidateSaysWhy(t *testing.T) {
	// A benchmark that cannot say why it dropped a rectangle cannot be tuned by anybody,
	// which is how this detector reached 71% false positives without anyone noticing
	// which rule was responsible.
	// A large frame with specks below the control-size floor. Sized against the frame,
	// deliberately: 32px is 1.3% of a 2400px width, under the 1.5% threshold, while still
	// being large enough for the scan to propose as a candidate at all.
	img := scene(2400, 1400, dark)
	for i := range 30 {
		fill(img, 40+(i%6)*380, 40+(i/6)*260, 32, 32, mid)
	}
	reports := visionbench.NewClassical().Explain(img)
	if len(reports) == 0 {
		t.Fatal("Explain reported no candidates at all")
	}
	refused := 0
	for _, r := range reports {
		if r.Rejected == visionbench.RejectNone {
			continue
		}
		refused++
		if r.Rejected != visionbench.RejectTooSmall {
			t.Errorf("unexpected rejection %q — the vocabulary is meant to be closed",
				r.Rejected)
		}
	}
	if refused == 0 {
		t.Error("nothing was refused on a frame of pure texture")
	}
}

func TestExplainAgreesWithDetect(t *testing.T) {
	// Two paths through one pipeline. If they drift, every tuning decision made from
	// Explain is being made about a detector that is not the one running.
	img := scene(800, 600, dark)
	fill(img, 100, 100, 200, 48, light)
	fill(img, 100, 200, 200, 48, light)

	c := visionbench.NewClassical()
	kept := 0
	for _, r := range c.Explain(img) {
		if r.Rejected == visionbench.RejectNone {
			kept++
		}
	}
	if got := len(detect(t, c, img)); got != kept {
		t.Errorf("Detect returned %d detections, Explain kept %d", got, kept)
	}
}
