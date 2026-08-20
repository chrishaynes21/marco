package visionbench_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// What each heuristic actually contributes, measured one at a time.
//
// The rule this milestone runs on: a heuristic that adds complexity and changes nothing gets
// removed, and one that changes things in the wrong direction gets removed faster. Both
// happened here — see the experiment record.
//
//	MARCO_CLASSICAL_ABLATE=1 go test ./internal/director/visionbench/ -run ClassicalAblation -v
func TestClassicalAblation(t *testing.T) {
	if os.Getenv("MARCO_CLASSICAL_ABLATE") == "" {
		t.Skip("set MARCO_CLASSICAL_ABLATE=1 to run the ablation sweep")
	}
	fixture, _, err := visionbench.LoadFixture("../../../fixtures/vision/rocketleague")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}

	cases := []struct {
		name string
		ab   visionbench.Ablations
	}{
		{"tuned (shipping)", visionbench.Ablations{}},
		{"no alignment role", visionbench.Ablations{NoAlignmentEvidence: true}},
		{"no size filter", visionbench.Ablations{NoSizeFilter: true}},
		{"baseline (nothing on)", visionbench.Ablations{
			NoSizeFilter: true, NoAlignmentEvidence: true}},
	}

	fmt.Printf("\n%-24s %6s %6s %8s %8s %8s %8s %7s\n",
		"CONFIGURATION", "DETS", "STABLE", "STRUCT%", "NAME%", "FALSE%", "PERSIST%", "SCORE")
	for _, tc := range cases {
		c := visionbench.NewClassical()
		c.Ablations = tc.ab
		reg := visionbench.NewRegistry()
		reg.Register(c)
		results := visionbench.Run(context.Background(), reg, fixture,
			visionbench.DefaultThresholds())
		m := results[0].Metrics
		s := visionbench.Score(m, visionbench.DefaultWeights())
		fmt.Printf("%-24s %6d %6d %7.0f%% %7.0f%% %7.0f%% %7.0f%% %7.1f\n",
			tc.name, m.Detections, m.StableEntities,
			m.StructuralCoverage*100, m.NameableCoverage*100,
			m.FalseStructureRate*100, m.Persistence*100, s.Total)
	}
}
