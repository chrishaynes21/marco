package visionbench_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// Generating the v2 ground truth from measured region positions.
//
// A generator rather than 39 hand-written files, because the interesting structures are the
// SAME across a sequence — a pause menu does not move while it is open, and the boost meter
// sits in the same corner all session. Writing that out by hand would invite exactly the
// transcription errors the schema validator exists to catch.
//
// Positions were measured off the captured frames. They are coarse, which is what the
// benchmark wants: MatchIoU is 0.35 because a detector that puts a slightly loose box around
// a button has found the button.
//
//	MARCO_WRITE_TRUTH=1 go test ./internal/director/visionbench/ -run GenerateV2Truth -v
func TestGenerateV2Truth(t *testing.T) {
	if os.Getenv("MARCO_WRITE_TRUTH") == "" {
		t.Skip("set MARCO_WRITE_TRUTH=1 to regenerate the v2 ground truth")
	}
	root := "../../../fixtures/vision/v2/rocketleague"

	// The boost meter: bottom-right, screen-relative, present in every gameplay frame and
	// in the pause frames behind the menu. This is the element the camera-motion sequence
	// exists to test — it must stay put while the entire arena moves.
	boost := visionbench.TruthRegion{
		Kind: visionbench.TruthMeter, Identity: "hud_boost",
		Bounds: visionbench.NormRect{X: 0.862, Y: 0.750, W: 0.104, H: 0.180},
	}

	// Arena texture: the brick wall band and the field surface. Broad, representative
	// negatives rather than exhaustive segmentation — enough to score whether the detector
	// mistakes scene structure for interface.
	arena := []visionbench.NegativeRegion{
		{Kind: visionbench.NegScenery,
			Bounds: visionbench.NormRect{X: 0.00, Y: 0.55, W: 0.42, H: 0.40}},
		{Kind: visionbench.NegGeometry,
			Bounds: visionbench.NormRect{X: 0.00, Y: 0.00, W: 0.55, H: 0.35}},
		{Kind: visionbench.NegGeometry,
			Bounds: visionbench.NormRect{X: 0.62, Y: 0.00, W: 0.38, H: 0.42}},
	}

	// The pause menu, measured once: a panel, a title, and four stacked controls of
	// identical width at regular vertical spacing.
	menu := []visionbench.TruthRegion{
		{Kind: visionbench.TruthPanel, Identity: "pause_panel",
			Bounds: visionbench.NormRect{X: 0.396, Y: 0.360, W: 0.208, H: 0.262}},
		{Kind: visionbench.TruthTextRegion, Identity: "pause_title",
			Bounds: visionbench.NormRect{X: 0.455, Y: 0.383, W: 0.090, H: 0.040}},
		{Kind: visionbench.TruthButton, Identity: "menu_resume",
			Bounds: visionbench.NormRect{X: 0.414, Y: 0.437, W: 0.172, H: 0.035}},
		{Kind: visionbench.TruthButton, Identity: "menu_change_mode",
			Bounds: visionbench.NormRect{X: 0.414, Y: 0.479, W: 0.172, H: 0.035}},
		{Kind: visionbench.TruthButton, Identity: "menu_settings",
			Bounds: visionbench.NormRect{X: 0.414, Y: 0.521, W: 0.172, H: 0.035}},
		{Kind: visionbench.TruthButton, Identity: "menu_exit",
			Bounds: visionbench.NormRect{X: 0.414, Y: 0.563, W: 0.172, H: 0.035}},
	}
	// # Two corrections the sanitisation pass forced, and they are corrections
	//
	// The bottom band of every pause frame is now a flat mask — see sanitize_test.go. Two
	// annotations had to go with it:
	//
	//   - `player_card` and `media_overlay` described real interface that no longer exists
	//     in the frame. An annotation claiming a region that sanitisation removed would make
	//     the corpus lie, and every detector would be scored against a phantom.
	//
	//   - `hud_boost` was annotated on pause frames and should never have been. Rocket
	//     League HIDES the HUD while paused; the boost meter is simply not there. That was
	//     an annotation error, found by looking at a frame rather than by any test, and it
	//     was inflating the truth-region count on 21 of 39 frames.
	//
	// The masked band itself is annotated as NOTHING — neither interface nor declared
	// scenery. A detection landing on it is `unmatched`, so it is neither credited nor
	// punished, which is the honest treatment of a region this experiment created.
	gameplay := []visionbench.TruthRegion{boost}
	paused := append([]visionbench.TruthRegion{}, menu...)

	specs := []struct {
		dir     string
		regions func(i, n int) []visionbench.TruthRegion
	}{
		{"freeplay-static", func(int, int) []visionbench.TruthRegion { return gameplay }},
		{"freeplay-camera-motion", func(int, int) []visionbench.TruthRegion { return gameplay }},
		// The transition sequences: the menu is absent at the start of open and at the end
		// of close. Frame 0 of pause-open is gameplay; the menu has arrived by frame 3.
		{"pause-open", func(i, n int) []visionbench.TruthRegion {
			if i < 3 {
				return gameplay
			}
			return paused
		}},
		{"pause-stable", func(int, int) []visionbench.TruthRegion { return paused }},
		{"pause-close", func(i, n int) []visionbench.TruthRegion {
			if i < 2 {
				return paused
			}
			return gameplay
		}},
	}

	total := 0
	for _, spec := range specs {
		dir := filepath.Join(root, spec.dir)
		paths, err := filepath.Glob(filepath.Join(dir, "*.png"))
		if err != nil || len(paths) == 0 {
			t.Fatalf("%s: no frames", dir)
		}
		sort.Strings(paths)

		truths := make([]visionbench.FrameTruth, 0, len(paths))
		for i, p := range paths {
			name := strings.TrimSuffix(filepath.Base(p), ".png")
			ft := visionbench.FrameTruth{
				Schema:           visionbench.GroundTruthSchema,
				Frame:            name,
				Sequence:         spec.dir,
				Index:            i,
				InterfacePresent: true,
				Regions:          spec.regions(i, len(paths)),
				NegativeRegions:  arena,
			}
			// Every pause-cycle frame was privacy-masked below y=0.80, INCLUDING the ones
			// that are gameplay. Declaring the band means the boost meter is no longer
			// claimed where it was painted over, and a detector is not punished for finding
			// the flat rectangle sanitisation left behind.
			if masked[spec.dir] {
				ft.IgnoreRegions = []visionbench.NormRect{{X: 0, Y: 0.80, W: 1, H: 0.20}}
				ft.Regions = withoutMasked(ft.Regions)
			}
			truths = append(truths, ft)
		}
		raw, err := json.MarshalIndent(truths, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "ground-truth.json"), append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		total += len(truths)
		fmt.Printf("  %-26s %d frames annotated\n", spec.dir, len(truths))
	}
	fmt.Printf("\n%d frames total\n", total)

	// Generated truth goes through the same validator as hand-written truth. A generator
	// that could emit something the loader refuses is a generator nobody can trust.
	for _, spec := range specs {
		if _, err := visionbench.LoadTruth(filepath.Join(root, spec.dir)); err != nil {
			t.Errorf("%s: %v", spec.dir, err)
		}
	}
}

// masked names the sequences whose frames carry a privacy mask.
//
// All three pause sequences, because the mask was applied to the whole pause-cycle capture —
// including its gameplay frames, whose boost meter it destroyed.
var masked = map[string]bool{"pause-open": true, "pause-stable": true, "pause-close": true}

// withoutMasked drops annotations the mask destroyed.
//
// Only `hud_boost` survives the earlier corrections, and only on masked sequences: it sits at
// y 0.750-0.930, more than half of it under the band.
func withoutMasked(in []visionbench.TruthRegion) []visionbench.TruthRegion {
	out := make([]visionbench.TruthRegion, 0, len(in))
	for _, r := range in {
		if r.Identity == "hud_boost" {
			continue
		}
		out = append(out, r)
	}
	return out
}
