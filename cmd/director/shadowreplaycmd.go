package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/shadowreplay"
	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// Diagnosing track fragmentation against the frozen corpus instead of a live game.
//
// # Why the corpus answers this and a live trace mostly cannot
//
// The question is why a VISIBLE button fails to become one track. Answering it needs to count
// the inferences where the button was on screen and nothing was detected — and a live trace
// cannot produce those, because nothing in a live session knows where the button was. The
// corpus does: every pause frame carries a per-identity box, so a miss is countable.
//
// What this cannot measure is cadence. Corpus frames are consecutive captures, not 2s samples,
// so consecutive overlap here is an UPPER BOUND on what a live 2s session would see. That
// asymmetry is the useful direction: geometry that already fails on dense frames fails harder
// when sampled sparsely, so a failure found here is real. Only a clean result would need the
// live run to confirm.
//
// Detector recall carries over unchanged — whether the model sees a button does not depend on
// how often it is asked.

// replayFrame is one cached frame of detections, so the 97MB model is loaded once.
type replayFrame struct {
	Sequence string                 `json:"sequence"`
	Frame    string                 `json:"frame"`
	Index    int                    `json:"index"`
	Regions  []observe.ShadowRegion `json:"regions"`
}

func runShadowReplay(args []string) int {
	fs := flag.NewFlagSet("shadow-replay", flag.ContinueOnError)
	root := fs.String("corpus", filepath.Join("fixtures", "vision", "v2", "rocketleague"),
		"corpus directory")
	cache := fs.String("detections", "", "cache detections here and reuse them")
	seqs := fs.String("sequences", "pause-open,pause-stable,pause-close",
		"comma-separated sequences to analyse")
	control := fs.String("control", "freeplay-static,freeplay-camera-motion",
		"sequences used as the stable-element control")
	sweep := fs.Bool("sweep", false, "replay at other thresholds and reference policies")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	all, err := loadReplayDetections(*root, *seqs+","+*control, *cache)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Println("ScreenParser button-track diagnosis — offline corpus replay")
	fmt.Println("conf=0.15  size=1280  NMS=0.45  match IoU=" +
		fmt.Sprintf("%.2f", observe.TrackMatchIoU) + "  reference=frozen-first")
	fmt.Println()

	for _, s := range splitStrings(*seqs) {
		reportSequence(*root, s, all[s], *sweep, false)
	}
	fmt.Println("─── control: elements that do not move ───")
	for _, s := range splitStrings(*control) {
		reportSequence(*root, s, all[s], false, true)
	}
	return 0
}

// loadReplayDetections runs ScreenParser over every frame, or reuses a cache.
func loadReplayDetections(root, seqs, cache string) (map[string][]replayFrame, error) {
	out := map[string][]replayFrame{}
	if cache != "" {
		if raw, err := os.ReadFile(cache); err == nil {
			var frames []replayFrame
			if err := json.Unmarshal(raw, &frames); err != nil {
				return nil, fmt.Errorf("reading cached detections: %w", err)
			}
			for _, f := range frames {
				out[f.Sequence] = append(out[f.Sequence], f)
			}
			fmt.Fprintf(os.Stderr, "reusing %d cached frames from %s\n", len(frames), cache)
			return out, nil
		}
	}

	backend := newScreenParserBackend()
	if state, reason := backend.Status(); state != "available" {
		// Reported, never swallowed. "The detector could not start" must not reach a report
		// as "the detector found nothing" — that confusion has cost this project a milestone.
		return nil, fmt.Errorf("ScreenParser is unavailable: %s", reason)
	}

	var flat []replayFrame
	for _, s := range splitStrings(seqs) {
		dir := filepath.Join(root, s)
		truths, err := visionbench.LoadTruth(dir)
		if err != nil {
			return nil, err
		}
		for _, t := range truths {
			path := filepath.Join(dir, t.Frame+".png")
			img, err := readPNG(path)
			if err != nil {
				// A declared frame with no image is a corpus gap, and the run must say so
				// rather than quietly analysing fewer frames than it claims.
				fmt.Fprintf(os.Stderr, "  %s/%s: no image on disk, skipped\n", s, t.Frame)
				continue
			}
			regions, err := detectShadowRegions(backend, img)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", t.Frame, err)
			}
			f := replayFrame{Sequence: s, Frame: t.Frame, Index: t.Index, Regions: regions}
			flat = append(flat, f)
			out[s] = append(out[s], f)
			fmt.Fprintf(os.Stderr, "  %s/%s: %d regions\n", s, t.Frame, len(regions))
		}
	}
	if cache != "" {
		if raw, err := json.MarshalIndent(flat, "", "  "); err == nil {
			_ = os.WriteFile(cache, raw, 0o644)
		}
	}
	return out, nil
}

// detectShadowRegions reproduces the shadow provider's acceptance exactly.
//
// Same class table, same structural rule, same floors, same nameability allowlist. Written
// against the SAME functions production uses rather than copies of them, so a change to the
// vocabulary cannot make this replay silently disagree with the thing it is describing.
func detectShadowRegions(b *screenParserBackend, img image.Image) ([]observe.ShadowRegion, error) {
	dets, err := b.Detect(context.Background(), img)
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	area := float64(bounds.Dx() * bounds.Dy())
	var out []observe.ShadowRegion
	for _, d := range dets {
		c, ok := vision.ClassOf(d.Label)
		if !ok || !c.Structural() {
			// Non-structural detections never become Elements, and only Elements reach the
			// tracker — see shadowRegions in shadowdiag.go.
			continue
		}
		if d.Confidence < 0.15 {
			continue
		}
		w, h := d.Bounds.Dx(), d.Bounds.Dy()
		if w < 6 || h < 6 {
			continue
		}
		if area > 0 && float64(w*h)/area > 0.9 {
			continue
		}
		role := string(c.Role())
		out = append(out, observe.ShadowRegion{
			Role:       role,
			Confidence: d.Confidence,
			Nameable:   nameableRole(c.Role()),
			Region: observe.Region{
				X:      float64(d.Bounds.Min.X-bounds.Min.X) / float64(bounds.Dx()),
				Y:      float64(d.Bounds.Min.Y-bounds.Min.Y) / float64(bounds.Dy()),
				Width:  float64(w) / float64(bounds.Dx()),
				Height: float64(h) / float64(bounds.Dy()),
			},
		})
	}
	return out, nil
}

func reportSequence(root, seq string, frames []replayFrame, sweep, controlOnly bool) {
	if len(frames) == 0 {
		fmt.Printf("%s: no frames\n\n", seq)
		return
	}
	truths, err := visionbench.LoadTruth(filepath.Join(root, seq))
	if err != nil {
		fmt.Printf("%s: %v\n\n", seq, err)
		return
	}

	sort.SliceStable(frames, func(a, b int) bool { return frames[a].Index < frames[b].Index })
	infs := make([]shadowreplay.Inference, 0, len(frames))
	for _, f := range frames {
		infs = append(infs, shadowreplay.Inference{
			Index: f.Index, Frame: f.Frame, Regions: f.Regions,
		})
	}

	anchors := anchorsFrom(truths, controlOnly)
	res := shadowreplay.Run(infs, shadowreplay.Production())
	reports := shadowreplay.Analyse(infs, anchors, res)

	fmt.Printf("=== %s — %d frames, %d valid inferences, %d tracks ===\n",
		seq, len(truths), res.Inferences, len(res.Tracks))
	fmt.Printf("%-18s %-6s %4s %4s %4s %4s %4s  %5s  %6s %6s %6s\n",
		"element", "role", "elig", "det", "mat", "frag", "miss",
		"trks", "recall", "consIoU", "refIoU")
	for _, e := range reports {
		fmt.Printf("%-18s %-6s %4d %4d %4d %4d %4d  %5d  %6.2f %6.2f %6.2f\n",
			e.Identity, e.Role, e.Eligible, e.Detected, e.Matched, e.Fragmented, e.Missed,
			len(e.Tracks), e.Recall(), e.MedianConsecutiveIoU, e.MedianRefIoU)
	}

	// The fragmentation events, with the two numbers that classify them.
	var frag int
	for _, e := range reports {
		for _, op := range e.Opportunities {
			if op.Outcome != shadowreplay.DetectedFragmented {
				continue
			}
			if frag == 0 {
				fmt.Println("\n  fragmentation events (why the expected track lost):")
			}
			frag++
			verdict := "unstable localisation"
			if op.PrevIoU >= observe.TrackMatchIoU && op.RefIoU < observe.TrackMatchIoU {
				verdict = "REFERENCE DRIFT — previous overlap was fine"
			} else if op.PrevIoU < observe.TrackMatchIoU && op.RefIoU < observe.TrackMatchIoU {
				verdict = "geometry moved too far between inferences"
			} else if op.RefIoU >= observe.TrackMatchIoU {
				verdict = "ASSIGNMENT — reference overlap cleared the bar, another detection won"
			}
			fmt.Printf("    %-18s inference %2d  prevIoU %.2f  refIoU %.2f  %s → %s  [%s]\n",
				op.Identity, op.Inference, op.PrevIoU, op.RefIoU, op.Expected, op.Track, verdict)
		}
	}
	if frag == 0 {
		fmt.Println("\n  no fragmentation events")
	}

	if sweep {
		reportSweep(infs, anchors)
	}
	fmt.Println()
}

// reportSweep replays the SAME detections under other policies. Nothing here changes
// production; it measures what a change would have been worth.
func reportSweep(infs []shadowreplay.Inference, anchors []shadowreplay.Anchor) {
	fmt.Println("\n  offline replay — threshold sweep (frozen-first reference):")
	fmt.Printf("    %-8s %6s %6s %s\n", "IoU", "tracks", "frag", "note")
	for _, th := range []float64{0.20, 0.25, 0.30, 0.35, 0.40, 0.50} {
		p := shadowreplay.Production()
		p.MatchIoU = th
		res := shadowreplay.Run(infs, p)
		reps := shadowreplay.Analyse(infs, anchors, res)
		note := ""
		if th == observe.TrackMatchIoU {
			note = "← production"
		}
		fmt.Printf("    %-8.2f %6d %6d %s\n", th, len(res.Tracks), totalFrag(reps), note)
	}

	fmt.Println("\n  offline replay — reference policy (production threshold):")
	fmt.Printf("    %-14s %6s %6s %6s %s\n", "policy", "tracks", "frag", "merges", "note")
	for _, pol := range []shadowreplay.ReferencePolicy{
		shadowreplay.ReferenceFrozenFirst,
		shadowreplay.ReferencePrevious,
		shadowreplay.ReferenceMean,
	} {
		p := shadowreplay.Production()
		p.Reference = pol
		res := shadowreplay.Run(infs, p)
		reps := shadowreplay.Analyse(infs, anchors, res)
		note := ""
		if pol == shadowreplay.ReferenceFrozenFirst {
			note = "← production"
		}
		fmt.Printf("    %-14s %6d %6d %6d %s\n",
			pol, len(res.Tracks), totalFrag(reps), sharedTracks(reps), note)
	}
}

// sharedTracks counts elements that ended up sharing a track with another element — the
// merge failure a looser policy risks, and the reason "fewer tracks" is not automatically
// better. A capability aimed at one menu row acting on another is worse than no capability.
func sharedTracks(reps []shadowreplay.ElementReport) int {
	owner := map[string]string{}
	var merged int
	for _, e := range reps {
		for _, id := range e.Tracks {
			if prev, ok := owner[id]; ok && prev != e.Identity {
				merged++
				continue
			}
			owner[id] = e.Identity
		}
	}
	return merged
}

func totalFrag(reps []shadowreplay.ElementReport) int {
	var n int
	for _, e := range reps {
		n += e.Fragmented
	}
	return n
}

// anchorsFrom turns ground truth into per-element opportunity anchors.
//
// The truth KIND is resolved through the same class table the detector's labels go through,
// so an element is only anchored when both sides have a word for it.
func anchorsFrom(truths []visionbench.FrameTruth, controlOnly bool) []shadowreplay.Anchor {
	byIdentity := map[string]*shadowreplay.Anchor{}
	var order []string
	for _, t := range truths {
		for _, r := range t.Regions {
			if r.Identity == "" {
				continue
			}
			c, ok := vision.ClassOf(string(r.Kind))
			if !ok || !c.Structural() {
				continue
			}
			role := string(c.Role())
			if controlOnly && role != "icon" {
				continue
			}
			a, seen := byIdentity[r.Identity]
			if !seen {
				a = &shadowreplay.Anchor{
					Identity: r.Identity, Role: role,
					Boxes: map[int]observe.Region{},
				}
				byIdentity[r.Identity] = a
				order = append(order, r.Identity)
			}
			a.Boxes[t.Index] = observe.Region{
				X: r.Bounds.X, Y: r.Bounds.Y, Width: r.Bounds.W, Height: r.Bounds.H,
			}
		}
	}
	out := make([]shadowreplay.Anchor, 0, len(order))
	for _, id := range order {
		out = append(out, *byIdentity[id])
	}
	return out
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// splitStrings splits a comma list, dropping empties so an omitted flag contributes nothing.
func splitStrings(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
