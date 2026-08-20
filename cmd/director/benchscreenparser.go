package main

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// ScreenParser as a benchmark backend.
//
// # Why this is configuration and not a new engine
//
// The generic ONNX plugin absorbed a 1280px 55-class YOLO11 detector without a line of
// inference code: it already did dynamic runtime loading, grey-114 letterboxing, CHW float32
// normalisation, `[1, 4+nc, n]` channel-major decode, class-aware NMS and reverse-letterbox
// mapping, and it reads class names from the model's own embedded metadata.
//
// So ScreenParser is a MODEL plus five environment values, and this file is deliberately
// nothing but that plus availability reporting. If a future model needs `if model ==
// screenparser` anywhere in the tensor path, the seam has been broken and that is the thing to
// fix rather than to work around.
//
// # Why it exists at all rather than reusing `current`
//
// Because "the shipping detector" and "the challenger" must be separately nameable in a
// report. A benchmark whose two backends are the same binary reading different environment
// variables cannot say which one produced a number six months later.

// screenParserBackend runs ScreenParser through the ordinary vision plugin.
type screenParserBackend struct {
	detector vision.Detector
	model    string
	reason   string
}

var _ visionbench.Backend = (*screenParserBackend)(nil)

// newScreenParserBackend configures the vision bridge for ScreenParser.
//
// Availability is decided HERE and reported as a reason, never as an empty frame. An
// infrastructure failure that reached the metrics as "zero detections" would be recorded as
// model performance — the mistake `icon_detect`'s 0% already taught this project once.
func newScreenParserBackend() *screenParserBackend {
	b := &screenParserBackend{}

	model := screenParserModel()
	if model == "" {
		b.reason = "no ScreenParser model — set $MARCO_SCREENPARSER_MODEL to " +
			"screenparser-1280.onnx (see docs/director-vision-ui-detector-decision.md)"
		return b
	}
	if _, err := os.Stat(model); err != nil {
		b.reason = "the ScreenParser model is not at " + model
		return b
	}
	if os.Getenv("MARCO_ONNXRUNTIME") == "" {
		b.reason = "$MARCO_ONNXRUNTIME is not set — the plugin loads the ONNX Runtime " +
			"shared library dynamically and cannot find it"
		return b
	}
	b.model = model

	// The plugin reads its configuration from the environment it inherits. Set here rather
	// than left to the caller so a benchmark run is reproducible from the command alone.
	os.Setenv("MARCO_VISION_MODEL", model)
	os.Setenv("MARCO_VISION_SIZE", "1280") // ScreenParser's training resolution.
	os.Setenv("MARCO_VISION_CONF", "0.15") // Frozen on the calibration split. Do not sweep.
	os.Setenv("MARCO_VISION_IOU", "0.45")  // Matches the reference NMS.

	detector, _, reason := newVisionDetector(defaultVisionBridge())
	if detector == nil {
		b.reason = reason
		return b
	}
	b.detector = detector
	return b
}

// screenParserModel resolves the model path, preferring an explicit setting.
func screenParserModel() string {
	if p := os.Getenv("MARCO_SCREENPARSER_MODEL"); p != "" {
		return p
	}
	local := filepath.Join("tools", "vision-export", "weights", "screenparser-1280.onnx")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	return ""
}

func (s *screenParserBackend) Name() string { return "screenparser" }

// Model records enough to reproduce a run, because an environment variable is not history.
func (s *screenParserBackend) Model() string {
	if s.detector == nil {
		return "unavailable"
	}
	return fmt.Sprintf("ScreenParser YOLO11-L 1280px 55-class conf=0.15 (%s)",
		filepath.Base(s.model))
}

// Status reports whether this backend can run, and why not when it cannot.
func (s *screenParserBackend) Status() (string, string) {
	if s.detector != nil {
		return "available", ""
	}
	return "unavailable", s.reason
}

// Detect runs one frame through the Go plugin. No Python is involved.
func (s *screenParserBackend) Detect(ctx context.Context, frame image.Image) (
	[]visionbench.Detection, error) {

	if s.detector == nil {
		return nil, errUnavailable(s.reason)
	}
	results, err := s.detector.Detect(ctx, vision.Input{Image: frame})
	if err != nil {
		// Surfaced, never swallowed. A runtime that refused to start must not be
		// indistinguishable from a model that found nothing.
		return nil, err
	}
	out := make([]visionbench.Detection, 0, len(results))
	for _, r := range results {
		out = append(out, visionbench.Detection{
			Label: r.Class, Confidence: r.Confidence, Bounds: r.Bounds, Text: r.Text,
		})
	}
	return out, nil
}

// Acceptance declares the floors ScreenParser should be judged at.
//
// # Why a backend gets to move its own bar
//
// Confidence scales are not comparable between models, and judging one model at another's
// floors measures calibration rather than usefulness. Experiment-001 recorded this exactly:
// Grounding DINO returned 0.40 for a correct `menu` against a 0.50 structural floor, twelve of
// thirteen detections were discarded, and the report blamed the model.
//
// ScreenParser reproduced it. Its scores on real game frames run 0.22-0.47 — the four pause
// menu buttons it finds perfectly score 0.25 to 0.47 — against production floors of 0.35 and
// 0.50. Every detection was refused for being under a bar set for a different detector.
//
// So the floors move to the threshold that was FROZEN on the calibration split, and to nothing
// else. This is not tuning: 0.15 was chosen before any held-out evidence was seen and is the
// same number the Python reference used.
func (s *screenParserBackend) Acceptance(base visionbench.Thresholds) visionbench.Thresholds {
	base.MinConfidence = 0.15
	base.MinStructuralConfidence = 0.15
	return base
}
