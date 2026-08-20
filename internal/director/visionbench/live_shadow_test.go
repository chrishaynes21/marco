package visionbench_test

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// Shadowing the classical detector over a real screen.
//
// The detector is benchmark-only by construction — cmd/director/shadow_test.go fails if it
// ever reaches a runtime composition — so it cannot be validated through the live perception
// path. This runs it over a frame captured outside the Director instead, which is the only
// honest way to see what it does on a real screen without promoting it.
//
// The image is never committed: it is a full desktop capture and would carry whatever
// happened to be on screen. Path comes from the environment for that reason.
//
//	MARCO_SHADOW_FRAME=/path/to.png go test ./internal/director/visionbench/ -run LiveShadow -v
func TestLiveShadowOverARealScreen(t *testing.T) {
	path := os.Getenv("MARCO_SHADOW_FRAME")
	if path == "" {
		t.Skip("set MARCO_SHADOW_FRAME to a captured frame")
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

	for _, tc := range []struct {
		name string
		ab   visionbench.Ablations
	}{
		{"baseline", visionbench.Ablations{NoSizeFilter: true, NoSceneSizeFilter: true}},
		{"tuned", visionbench.Ablations{}},
	} {
		c := visionbench.NewClassical()
		c.Ablations = tc.ab
		reports := c.Explain(img)
		kept, byRole := 0, map[string]int{}
		byReason := map[visionbench.Rejection]int{}
		for _, r := range reports {
			if r.Rejected == visionbench.RejectNone {
				kept++
				byRole[r.Role]++
			} else {
				byReason[r.Rejected]++
			}
		}
		fmt.Printf("%-9s candidates=%d kept=%d roles=%v rejected=%v\n",
			tc.name, len(reports), kept, byRole, byReason)
	}
}
