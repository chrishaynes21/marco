package visionbench_test

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// ScreenParser against Corpus V2, scored by the real ScoreV2 implementation.
//
// Inference runs the exported ONNX artifact and writes normalised detections to JSON; every
// metric below is computed by the same Go code that scored Classical CV. What is NOT yet in
// place is the Go plugin transport — see the experiment record. That is a wiring gap and it is
// stated rather than papered over.
//
//	MARCO_SP=1 go test ./internal/director/visionbench/ -run ScreenParser -v

// The calibration / evaluation split, frozen BEFORE any threshold was chosen.
//
// Split by SEQUENCE, never by frame: adjacent frames of one sequence are near-duplicates, and
// splitting between them would let the threshold be tuned on essentially the same evidence it
// is later judged on.
var (
	calibrationSeqs = map[string]bool{"freeplay-static": true, "pause-stable": true}
	// Everything else is held out: freeplay-camera-motion, pause-open, pause-close.
)

type spDump struct {
	Model    string  `json:"model"`
	Conf     float64 `json:"conf"`
	MedianMs float64 `json:"median_ms"`
	P95Ms    float64 `json:"p95_ms"`
	Frames   map[string][]struct {
		Class      string  `json:"class"`
		Confidence float64 `json:"confidence"`
		Bounds     struct {
			X, Y, Width, Height float64
		} `json:"bounds"`
	} `json:"frames"`
}

// screenParserClass maps ScreenParser's 55-class ontology onto Marco's closed vocabulary.
//
// Explicit and total: everything lands on a vision class or on unknown. No ambiguous class is
// promoted to a nameable role to improve a score — that is exactly the inflation
// NameablePrecision exists to catch.
var screenParserClass = map[string]string{
	// Nameable controls.
	"Button": "button", "Utility Button": "button",
	"Menu": "menu", "ContextMenu": "menu", "DockMenu": "menu",
	"EditMenu": "menu", "PopUp Menu": "menu",
	"Tab": "tab", "Tab Bar": "tab",
	"Checkbox": "checkbox", "Radiobox": "radio",
	"List Item": "menu_item",
	// Inputs.
	"Text Input": "field", "Search Field": "field", "Search Bar": "field",
	"Select": "field", "Picker": "field", "Date-Time picker": "field",
	// Proportional indicators.
	"Progress bar": "bar", "Slider": "bar", "Rating Indicator": "bar",
	// Containers.
	"Window": "panel", "Screen": "panel", "Side Bar": "panel", "Toolbar": "panel",
	"Navigation Bar": "panel", "Status Bar": "panel", "Alert": "panel",
	"Notification": "panel", "Tooltip": "panel", "Bottom navigation": "panel",
	"Card": "panel", "Column/Browser": "panel", "Table": "panel", "List": "panel",
	"Carousel": "panel", "Chart": "panel", "Scroll": "panel",
	// Text.
	"Text": "text", "Heading": "text", "Code snippet": "text", "Link": "text",
	"Badge": "text", "Breadcrumb": "text", "Pagination": "text", "Page control": "text",
	// Pictorial.
	"Image": "image", "App Icon": "icon", "File Icon": "icon",
	"Logo": "icon", "Avatar": "icon", "Video": "image",
	// Two-state.
	"Switch": "checkbox", "Toggles": "checkbox", "Steppers": "unknown",
	"Calendar": "panel",
}

func loadDump(t *testing.T, path string) spDump {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no detections at %s: %v", path, err)
	}
	var d spDump
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return d
}

// toDetections converts one dump into benchmark detections for the chosen frames.
func toDetections(d spDump, bounds map[string]image.Rectangle,
	keep func(seq string) bool, truths []visionbench.FrameTruth) map[string][]visionbench.Detection {

	// Keyed by SEQUENCE-scoped identity: the dump is keyed by basename, and four
	// boundary frames belong to two sequences each. Iterating the TRUTH rather than the
	// dump is what makes each sequence get its own copy of a shared frame.
	out := map[string][]visionbench.Detection{}
	for _, ft := range truths {
		if !keep(ft.Sequence) {
			continue
		}
		dets, ok := d.Frames[ft.Frame]
		if !ok {
			continue
		}
		fb, ok := bounds[ft.Key()]
		if !ok {
			continue
		}
		var list []visionbench.Detection
		for _, x := range dets {
			cls, known := screenParserClass[x.Class]
			if !known {
				cls = "unknown"
			}
			r := visionbench.NormRect{X: x.Bounds.X, Y: x.Bounds.Y,
				W: x.Bounds.Width, H: x.Bounds.Height}
			list = append(list, visionbench.Detection{
				Label: cls, Confidence: x.Confidence, Bounds: r.Pixels(fb),
			})
		}
		out[ft.Key()] = list
	}
	return out
}

func subsetTruth(truths []visionbench.FrameTruth, keep func(string) bool) []visionbench.FrameTruth {
	var out []visionbench.FrameTruth
	for _, ft := range truths {
		if keep(ft.Sequence) {
			out = append(out, ft)
		}
	}
	return out
}

// TestScreenParserCalibration sweeps confidence on the CALIBRATION sequences only.
func TestScreenParserCalibration(t *testing.T) {
	if os.Getenv("MARCO_SP") == "" {
		t.Skip("set MARCO_SP=1")
	}
	truths, _, bounds := loadV2(t)
	cal := subsetTruth(truths, func(s string) bool { return calibrationSeqs[s] })

	fmt.Printf("\nCALIBRATION (freeplay-static + pause-stable, %d frames)\n", len(cal))
	fmt.Printf("%-6s %6s %7s %7s %8s %8s %5s %5s\n",
		"CONF", "DETS", "PREC", "RECALL", "N-PREC", "N-REC", "TP", "FP")
	for _, c := range []string{"0.15", "0.25", "0.35", "0.50"} {
		d := loadDump(t, "../../../tools/vision-export/dets-"+c+".json")
		byFrame := toDetections(d, bounds, func(s string) bool { return calibrationSeqs[s] }, truths)
		m := visionbench.EvaluateTruth(byFrame, bounds, cal)
		fmt.Printf("%-6s %6d %6.0f%% %6.0f%% %7.0f%% %7.0f%% %5d %5d\n",
			c, m.Detections, m.Precision*100, m.Recall*100,
			m.NameablePrecision*100, m.NameableRecall*100, m.TruePos, m.FalsePos)
	}
}

// TestScreenParserHeldOut runs ONCE at the frozen threshold on evidence never used to choose it.
func TestScreenParserHeldOut(t *testing.T) {
	if os.Getenv("MARCO_SP") == "" {
		t.Skip("set MARCO_SP=1")
	}
	conf := os.Getenv("MARCO_SP_CONF")
	if conf == "" {
		t.Skip("set MARCO_SP_CONF to the frozen threshold")
	}
	truths, _, bounds := loadV2(t)
	heldOut := func(s string) bool { return !calibrationSeqs[s] }
	eval := subsetTruth(truths, heldOut)

	d := loadDump(t, "../../../tools/vision-export/dets-"+conf+".json")
	byFrame := toDetections(d, bounds, heldOut, truths)
	m := visionbench.EvaluateTruth(byFrame, bounds, eval)
	median := time.Duration(d.MedianMs) * time.Millisecond
	s, _ := visionbench.ScoreV2(m, median, visionbench.DefaultWeightsV2())

	fmt.Printf("\nHELD OUT — camera-motion + pause-open + pause-close, %d frames, conf=%s\n",
		len(eval), conf)
	fmt.Printf("  detections            %d\n", m.Detections)
	fmt.Printf("  TP / FP / unmatched   %d / %d / %d\n", m.TruePos, m.FalsePos, m.Unmatched)
	fmt.Printf("  truth regions/matched %d / %d\n", m.TruthRegions, m.Matched)
	fmt.Printf("  structural  P / R     %.0f%% / %.0f%%\n", m.Precision*100, m.Recall*100)
	fmt.Printf("  nameable    P / R     %.0f%% / %.0f%%\n",
		m.NameablePrecision*100, m.NameableRecall*100)
	fmt.Printf("  temporal    P / R     %.0f%% / %.0f%%\n",
		m.TemporalPrecision*100, m.TemporalRecall*100)
	fmt.Printf("  OCR-region  P / R     %.0f%% / %.0f%%\n",
		m.OCRPrecision*100, m.OCRRecall*100)
	fmt.Printf("  median / p95          %.0fms / %.0fms\n", d.MedianMs, d.P95Ms)
	fmt.Printf("  ScoreV2               %.1f\n", s.Total)
}

// TestScreenParserBySequence breaks the held-out result down, because "mediocre overall" and
// "excellent on menus, blind on HUD" are different findings and only one is actionable.
func TestScreenParserBySequence(t *testing.T) {
	if os.Getenv("MARCO_SP") == "" {
		t.Skip("set MARCO_SP=1")
	}
	conf := os.Getenv("MARCO_SP_CONF")
	if conf == "" {
		t.Skip("set MARCO_SP_CONF")
	}
	truths, _, bounds := loadV2(t)
	d := loadDump(t, "../../../tools/vision-export/dets-"+conf+".json")

	seqs := map[string]bool{}
	for _, ft := range truths {
		seqs[ft.Sequence] = true
	}
	names := make([]string, 0, len(seqs))
	for s := range seqs {
		names = append(names, s)
	}
	sort.Strings(names)

	fmt.Printf("\n%-26s %5s %5s %5s %7s %7s %7s %7s\n",
		"SEQUENCE", "DETS", "TP", "FP", "PREC", "RECALL", "T-PREC", "T-REC")
	for _, name := range names {
		only := func(s string) bool { return s == name }
		sub := subsetTruth(truths, only)
		m := visionbench.EvaluateTruth(toDetections(d, bounds, only, truths), bounds, sub)
		tag := name
		if calibrationSeqs[name] {
			tag += " (cal)"
		}
		fmt.Printf("%-26s %5d %5d %5d %6.0f%% %6.0f%% %6.0f%% %6.0f%%\n",
			tag, m.Detections, m.TruePos, m.FalsePos,
			m.Precision*100, m.Recall*100, m.TemporalPrecision*100, m.TemporalRecall*100)
	}
}
