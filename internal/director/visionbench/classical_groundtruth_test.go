package visionbench_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// Precision against DECLARED ground truth, which is the one thing this corpus can measure.
//
// # Why this exists beside false_structure_rate
//
// The benchmark's false-structure metric is a temporal proxy: a structural detection that
// fails to persist across frames was probably not structure. That is a good proxy for a
// SEQUENCE. This corpus is not one — it is six heterogeneous crops of different scenes at
// different resolutions (960x540 down to 240x240), so "persisted across frames" mostly
// measures whether two unrelated rectangles landed in the same coarse ninth.
//
// The consequence is severe and was only visible once measured: emitting MORE junk produces
// more accidental identity collisions, so persistence RISES and the score improves. Under
// that metric the best possible detector on this corpus is the one that filters nothing.
//
// The manifest, however, states outright which frames contain no interface. A detection on
// one of those is a false positive, full stop, with no inference. That is what is counted
// here — and it moves in the opposite direction to the score, which is the finding.
//
//	MARCO_CLASSICAL_ABLATE=1 go test ./internal/director/visionbench/ -run GroundTruth -v
func TestClassicalGroundTruthPrecision(t *testing.T) {
	if os.Getenv("MARCO_CLASSICAL_ABLATE") == "" {
		t.Skip("set MARCO_CLASSICAL_ABLATE=1 to run the ground-truth sweep")
	}
	fixture, manifest, err := visionbench.LoadFixture("../../../fixtures/vision/rocketleague")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}

	// Frames the manifest declares to hold no interface. Everything found there is wrong.
	empty := map[string]bool{}
	for _, f := range manifest.Frames {
		if f.Expected == "contains_no_ui_structure" {
			empty[f.ID] = true
		}
	}
	if len(empty) == 0 {
		t.Skip("this corpus declares no empty frames, so precision cannot be grounded")
	}

	cases := []struct {
		name string
		ab   visionbench.Ablations
	}{
		{"baseline (no heuristics)", visionbench.Ablations{NoSizeFilter: true, NoAlignmentEvidence: true}},
		{"size filter only", visionbench.Ablations{NoAlignmentEvidence: true}},
		{"size + alignment role", visionbench.Ablations{}},
	}

	fmt.Printf("\n%-26s %8s %8s %8s %10s %10s\n",
		"CONFIGURATION", "ON EMPTY", "ON UI", "TOTAL", "FALSE-POS%", "UI KEPT")
	for _, tc := range cases {
		c := visionbench.NewClassical()
		c.Ablations = tc.ab
		onEmpty, onUI := 0, 0
		for _, f := range fixture.Frames {
			dets, err := c.Detect(context.Background(), f.Image)
			if err != nil {
				t.Fatalf("%s: %v", f.Name, err)
			}
			if empty[f.Name] {
				onEmpty += len(dets)
			} else {
				onUI += len(dets)
			}
		}
		total := onEmpty + onUI
		rate := 0.0
		if total > 0 {
			rate = float64(onEmpty) / float64(total) * 100
		}
		fmt.Printf("%-26s %8d %8d %8d %9.0f%% %10d\n",
			tc.name, onEmpty, onUI, total, rate, onUI)
	}
}
