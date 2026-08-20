package visionbench_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// A lens on what the classical detector considered and why it refused things.
//
// Not an assertion — a listing. The heuristics in this milestone come from looking at the
// real output on the real corpus, and a threshold invented before that is a guess wearing a
// benchmark's clothes.
//
//	MARCO_CLASSICAL_DUMP=1 go test ./internal/director/visionbench/ -run ClassicalDump -v
func TestClassicalDump(t *testing.T) {
	if os.Getenv("MARCO_CLASSICAL_DUMP") == "" {
		t.Skip("set MARCO_CLASSICAL_DUMP=1 to list candidates")
	}
	fixture, _, err := visionbench.LoadFixture("../../../fixtures/vision/rocketleague")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}
	c := visionbench.NewClassical()
	totals := map[visionbench.Rejection]int{}
	kept := 0

	for _, f := range fixture.Frames {
		reports := c.Explain(f.Image)
		fb := f.Image.Bounds()
		fmt.Printf("\n=== %s  (%dx%d)  %d candidates\n", f.Name, fb.Dx(), fb.Dy(), len(reports))
		for _, r := range reports {
			totals[r.Rejected]++
			if r.Rejected == visionbench.RejectNone {
				kept++
			}
			verdict := string(r.Rejected)
			if verdict == "" {
				verdict = "KEPT:" + r.Role
			}
			b := r.Bounds
			fmt.Printf("  %4d,%4d %4dx%-4d border=%.2f peers=%d(r%d/c%d) area%%=%4.1f asp=%5.2f  %s\n",
				b.Min.X, b.Min.Y, b.Dx(), b.Dy(),
				r.Features.BorderContinuity, r.Features.AlignedPeers,
				r.Features.RowMembers, r.Features.ColMembers,
				r.Features.AreaRatio*100, r.Features.Aspect, verdict)
		}
	}
	fmt.Printf("\n=== TOTALS: %d kept\n", kept)
	for why, n := range totals {
		if why != visionbench.RejectNone {
			fmt.Printf("  %-20s %d\n", why, n)
		}
	}
}
