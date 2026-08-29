package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/shadowreplay"
	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// WHAT DOES A PIXEL DETECTOR ADD TO PERCEPTION THAT ALREADY WORKS?
//
// 37B answered a narrower question — is ScreenParser better than the current detector on game
// frames — and got a clear yes. That answer does not transfer. A game has no accessibility
// tree, so ANY detector beats nothing. The desktop is the opposite case: accessibility is
// present, rich, labelled and legally attributed, and the honest question is whether a
// detector adds anything on top of it.
//
// This command puts both readings of the SAME captured moment side by side and counts. It
// decides nothing and admits nothing; the decision belongs to the experiment record.
//
// Three counts carry the whole result:
//
//	MATCHED        a detection that lands on an element production already knows about.
//	               Confirmation, not addition. This is the number that will be large.
//	DETECTOR ONLY  a detection with no production element under it. The candidate additions —
//	               and the only place ADDITIVE value can come from.
//	PRODUCTION ONLY an element with no detection on it. Not a failure: production sees
//	               structure that has no visual boundary at all.
//
// The trap this is built to avoid is treating DETECTOR ONLY as value. A box drawn around a
// gradient, a shadow, or the whitespace between two controls is also DETECTOR ONLY. So the
// count is reported alongside what production believed at that spot, and the experiment record
// reads the candidates rather than the ratio.

// desktopComparison is one sample, read twice.
type desktopComparison struct {
	ID     string `json:"id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	// Production is what fusion believed, on screen only. See redactdesktop.go for why
	// offscreen elements are not counted here.
	Production int `json:"production_elements"`
	// ProductionActionable is the subset production would let Marco act on. The comparison
	// that matters for actuation is against THIS, not against everything perceived.
	ProductionActionable int `json:"production_actionable"`
	Detections           int `json:"detector_detections"`
	Matched              int `json:"matched"`
	DetectorOnly         int `json:"detector_only"`
	// DetectorOnlyInsideKnown is the detector-only boxes whose CENTRE nonetheless falls
	// inside an element production already had. These are not additions: they are the icon
	// in a nav row, the toggle in a settings row, the chevron in a combo box — parts of a
	// thing already known, or the same thing boxed to a different convention.
	//
	// It exists because IoU alone is unfair to the detector AND flattering to it at once.
	// A detector boxes a radio button together with its label; accessibility boxes the 14px
	// dot. IoU says 0.00 and both counts go up, when in truth the two agree.
	DetectorOnlyInsideKnown int `json:"detector_only_inside_known"`
	ProductionOnly          int `json:"production_only"`
	// DetectorOnlyExamples is what a reader should look at before believing the count.
	DetectorOnlyExamples []detectorCandidate `json:"detector_only_examples,omitempty"`
	DetectMillis         int64               `json:"detect_millis"`
	// Unavailable says the detector could not run, so zero detections means "not asked",
	// never "found nothing". The `icon_detect` 0% is the reason this is a field.
	Unavailable string `json:"unavailable,omitempty"`
}

// detectorCandidate is a detection production had nothing under.
type detectorCandidate struct {
	Label      string         `json:"label"`
	Confidence float64        `json:"confidence"`
	Bounds     observe.Region `json:"bounds"`
	// NearestRole and NearestIoU say what production DID have nearby, which is how a
	// reader tells a genuine addition from a box drawn slightly off an element that was
	// already known.
	NearestRole  string  `json:"nearest_role,omitempty"`
	NearestLabel string  `json:"nearest_label,omitempty"`
	NearestIoU   float64 `json:"nearest_iou"`
}

// runCompareDesktopPerception is `director compare-desktop-perception --dir <corpus>`.
func runCompareDesktopPerception(args []string) int {
	fs := flag.NewFlagSet("compare-desktop-perception", flag.ExitOnError)
	dir := fs.String("dir", ".tmp/desktop-corpus-review", "the corpus directory")
	minIoU := fs.Float64("iou", 0.5, "overlap at which a detection counts as the same thing")
	examples := fs.Int("examples", 6, "detector-only candidates to record per sample")
	out := fs.String("out", "", "write the comparison here as JSON (default: alongside each sample)")
	_ = fs.Parse(flagsFirst(args))

	entries, err := os.ReadDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}

	backend := newScreenParserBackend()
	status, reason := backend.Status()
	fmt.Printf("detector: %s — %s\n", backend.Name(), backend.Model())
	if status != "available" {
		// Stated once, at the top, and then recorded on every row. A run that could not
		// load the model must not read as a run where the model found nothing.
		fmt.Printf("  UNAVAILABLE: %s\n", reason)
	}
	fmt.Println()

	var all []desktopComparison
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := compareOneSample(filepath.Join(*dir, e.Name()), backend, *minIoU, *examples)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s: %v\n", e.Name(), err)
			continue
		}
		all = append(all, c)
	}
	if len(all) == 0 {
		fmt.Fprintf(os.Stderr, "director: no samples in %s\n", *dir)
		return 1
	}

	printComparison(all)

	if *out != "" {
		b, err := json.MarshalIndent(all, "", " ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			return 1
		}
		fmt.Printf("\nwritten to %s\n", *out)
	}
	return 0
}

// compareOneSample runs the detector over one frame and counts against the reading.
func compareOneSample(dir string, backend visionbench.Backend, minIoU float64, examples int) (
	desktopComparison, error) {

	raw, err := os.ReadFile(filepath.Join(dir, "production.json"))
	if err != nil {
		return desktopComparison{}, err
	}
	var s desktopSample
	if err := json.Unmarshal(raw, &s); err != nil {
		return desktopComparison{}, err
	}

	c := desktopComparison{
		ID: s.ID, Width: s.Width, Height: s.Height, Production: len(s.Elements),
	}
	for _, e := range s.Elements {
		if e.Actionable {
			c.ProductionActionable++
		}
	}

	f, err := os.Open(filepath.Join(dir, s.ID+".png"))
	if err != nil {
		return c, err
	}
	frame, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		return c, fmt.Errorf("%s.png: %w", s.ID, err)
	}

	start := time.Now()
	dets, err := backend.Detect(context.Background(), frame)
	c.DetectMillis = time.Since(start).Milliseconds()
	if err != nil {
		c.Unavailable = err.Error()
		return c, nil
	}

	// EVERY BOX IN THE SAME SPACE. Production stores proportions of the window frame;
	// detections come back in pixels of the image. Converting the detections is what makes
	// the two comparable at all, and getting it backwards would produce a clean-looking zero.
	b := frame.Bounds()
	matchedProd := make([]bool, len(s.Elements))
	var candidates []detectorCandidate
	for _, d := range dets {
		if insideAnyRedaction(d.Bounds, b, s.Redactions) {
			// A detection on a black rectangle is an artifact of the privacy pass, not a
			// reading of the interface.
			continue
		}
		c.Detections++
		box := relativeToImage(d.Bounds, b)
		best, bestIdx := 0.0, -1
		for i, e := range s.Elements {
			if v := shadowreplay.IoU(box, e.Bounds); v > best {
				best, bestIdx = v, i
			}
		}
		if best >= minIoU {
			c.Matched++
			if bestIdx >= 0 {
				matchedProd[bestIdx] = true
			}
			continue
		}
		c.DetectorOnly++
		if containedInAny(box, s.Elements) {
			c.DetectorOnlyInsideKnown++
		}
		cand := detectorCandidate{
			Label: d.Label, Confidence: d.Confidence, Bounds: box, NearestIoU: best,
		}
		if bestIdx >= 0 {
			cand.NearestRole = s.Elements[bestIdx].Role
			cand.NearestLabel = s.Elements[bestIdx].Label
		}
		candidates = append(candidates, cand)
	}
	for _, m := range matchedProd {
		if !m {
			c.ProductionOnly++
		}
	}

	// The most confident candidates are the ones worth a person's attention.
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Confidence > candidates[j].Confidence
	})
	if len(candidates) > examples {
		candidates = candidates[:examples]
	}
	c.DetectorOnlyExamples = candidates
	return c, nil
}

// relativeToImage puts a detection in the same space as the reading.
func relativeToImage(r image.Rectangle, frame image.Rectangle) observe.Region {
	w, h := float64(frame.Dx()), float64(frame.Dy())
	if w == 0 || h == 0 {
		return observe.Region{}
	}
	return observe.Region{
		X:      float64(r.Min.X-frame.Min.X) / w,
		Y:      float64(r.Min.Y-frame.Min.Y) / h,
		Width:  float64(r.Dx()) / w,
		Height: float64(r.Dy()) / h,
	}
}

// insideAnyRedaction is whether a detection sits on a blacked-out rectangle.
func insideAnyRedaction(r image.Rectangle, frame image.Rectangle, reds []redaction) bool {
	box := relativeToImage(r, frame)
	for _, red := range reds {
		if shadowreplay.IoU(box, red.Region) > 0 {
			return true
		}
	}
	return false
}

// printComparison renders the table and the corpus totals.
func printComparison(all []desktopComparison) {
	fmt.Printf("%-24s %5s %5s %5s %7s %7s %7s %8s\n",
		"sample", "prod", "act", "dets", "matched", "det-only", "prod-only", "detect")
	fmt.Println(strings.Repeat("-", 85))
	var tp, ta, td, tm, tdo, tdi, tpo int
	var tms int64
	for _, c := range all {
		note := ""
		if c.Unavailable != "" {
			note = "  UNAVAILABLE"
		}
		fmt.Printf("%-24s %5d %5d %5d %7d %7d %7d %6dms%s\n",
			c.ID, c.Production, c.ProductionActionable, c.Detections,
			c.Matched, c.DetectorOnly, c.ProductionOnly, c.DetectMillis, note)
		tp += c.Production
		ta += c.ProductionActionable
		td += c.Detections
		tm += c.Matched
		tdo += c.DetectorOnly
		tdi += c.DetectorOnlyInsideKnown
		tpo += c.ProductionOnly
		tms += c.DetectMillis
	}
	fmt.Println(strings.Repeat("-", 85))
	fmt.Printf("%-24s %5d %5d %5d %7d %7d %7d %6dms\n",
		fmt.Sprintf("TOTAL (%d samples)", len(all)), tp, ta, td, tm, tdo, tpo, tms)

	if td > 0 {
		fmt.Printf("\n  of %d detections, %d (%.0f%%) land on something production already "+
			"knew about\n", td, tm, 100*float64(tm)/float64(td))
		fmt.Printf("  %d (%.0f%%) have no production element under them — the candidate "+
			"additions\n", tdo, 100*float64(tdo)/float64(td))
	}
	if tp > 0 {
		fmt.Printf("  of %d production elements, %d (%.0f%%) had no detection on them\n",
			tp, tpo, 100*float64(tpo)/float64(tp))
	}
	fmt.Printf("\n  A detector-only box is a CANDIDATE, not an addition. Read the examples in\n" +
		"  the JSON before believing any of them; a box on a gradient counts here too.\n")
}

// containedInAny is whether a detection's centre falls inside an element already perceived.
//
// The window element contains everything, so it is skipped: "inside the window" is true of
// every box on screen and says nothing about whether the thing was known.
func containedInAny(box observe.Region, els []desktopElement) bool {
	cx, cy := box.X+box.Width/2, box.Y+box.Height/2
	for _, e := range els {
		if e.Role == "window" || e.Role == "pane" {
			continue
		}
		if cx >= e.Bounds.X && cx <= e.Bounds.X+e.Bounds.Width &&
			cy >= e.Bounds.Y && cy <= e.Bounds.Y+e.Bounds.Height {
			return true
		}
	}
	return false
}
