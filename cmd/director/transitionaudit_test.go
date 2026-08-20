package main

import (
	"context"
	"image"
	"os"
	"sort"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// Where a detector's mistimed transition claims actually fall.
//
// The aggregate says `pause-close` earned ten mistimed frames. That number cannot distinguish
// two very different behaviours:
//
//	a one-frame lag       the menu is released on the next frame — a shutter effect, and
//	                      arguably correct given a fading menu
//	sustained staleness   the menu is still being reported four frames after it closed
//
// The first is a latency characteristic; the second would disqualify the detector from ever
// driving belief. A percentage cannot tell them apart, and guessing which one a number means is
// how "stale menu persistence" survived two milestones as an unmeasured assumption.
//
// Gated because it needs the real ONNX runtime and the real model:
//
//	MARCO_TRANSITION_AUDIT=1 MARCO_ONNXRUNTIME=… MARCO_VISION=… go test ./cmd/director -run TransitionAudit -v
func TestTransitionAuditPerFrame(t *testing.T) {
	if os.Getenv("MARCO_TRANSITION_AUDIT") == "" {
		t.Skip("set MARCO_TRANSITION_AUDIT=1 with a real ONNX runtime to audit transitions")
	}
	c := loadCorpus(t)
	backend := newScreenParserBackend()
	if status, reason := backend.Status(); status != "available" {
		t.Skipf("screenparser unavailable: %s", reason)
	}

	for _, seq := range c.SequenceNames() {
		mode := c.Modes[seq]
		frameCount := len(c.Subset(func(s string) bool { return s == seq }))
		changing := false
		for _, tr := range mode.Tracks {
			if tr.Frames() < frameCount {
				changing = true
			}
		}
		if !changing {
			continue
		}
		t.Run(seq, func(t *testing.T) {
			frames := c.Subset(func(s string) bool { return s == seq })
			sort.SliceStable(frames, func(i, j int) bool {
				return frames[i].Index < frames[j].Index
			})

			// Where each transitioning identity lives, taken from a frame that declares it.
			where := map[string]visionbench.NormRect{}
			declared := map[string]map[int]bool{}
			for _, ft := range frames {
				for _, r := range ft.Regions {
					id := r.Identity
					if id == "" {
						continue
					}
					where[id] = r.Bounds
					if declared[id] == nil {
						declared[id] = map[int]bool{}
					}
					declared[id][ft.Index] = true
				}
			}
			ids := make([]string, 0, len(where))
			for id := range where {
				ids = append(ids, id)
			}
			sort.Strings(ids)

			t.Logf("%s — %s", seq, mode.Shape(frameCount))
			for _, ft := range frames {
				fb := c.Bounds[ft.Key()]
				dets, err := backend.Detect(context.Background(), imageFor(t, c, ft))
				if err != nil {
					t.Fatalf("frame %s: %v", ft.Key(), err)
				}
				var found []string
				for _, id := range ids {
					want := where[id].Pixels(fb)
					for _, d := range dets {
						if overlap(d.Bounds, want) >= visionbench.MatchIoU {
							found = append(found, id)
							break
						}
					}
				}
				phase := "absent "
				if declaredAt(declared, ids, ft.Index) {
					phase = "PRESENT"
				}
				t.Logf("  frame %d  %s  %2d detections  %d/%d identities found: %v",
					ft.Index, phase, len(dets), len(found), len(ids), found)
			}
		})
	}
}

func declaredAt(declared map[string]map[int]bool, ids []string, index int) bool {
	for _, id := range ids {
		if declared[id][index] {
			return true
		}
	}
	return false
}

func imageFor(t *testing.T, c visionbench.V2Corpus, ft visionbench.FrameTruth) image.Image {
	t.Helper()
	for _, f := range c.Fixture.Frames {
		if f.Name == ft.Key() {
			return f.Image
		}
	}
	t.Fatalf("no image for %s", ft.Key())
	return nil
}

// overlap is intersection over union. Duplicated from the scorer rather than exported from it:
// a diagnostic must not be able to change what the benchmark measures.
func overlap(a, b image.Rectangle) float64 {
	i := a.Intersect(b)
	if i.Empty() {
		return 0
	}
	ia := float64(i.Dx() * i.Dy())
	ua := float64(a.Dx()*a.Dy()) + float64(b.Dx()*b.Dy()) - ia
	if ua <= 0 {
		return 0
	}
	return ia / ua
}
