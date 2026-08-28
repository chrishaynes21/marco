package main

import (
	"os"
	"strings"
	"testing"
)

// THE BENCHMARK BASELINE IS NOT THE CHALLENGER.
//
// # What was measured, and why it made every comparison meaningless
//
// A bridge host launches its child on FIRST USE, which is during the run — after every backend
// has been constructed. So configuring the challenger with process-wide `os.Setenv` reached the
// baseline's plugin too: both children spawned inheriting ScreenParser's model and settings, and
// the report printed two rows with byte-identical numbers under two different names.
//
// Measured on `fixtures/vision/v2/rocketleague`, 39 frames: `current` and `screenparser` both
// reported 145 detections, 80 TP, 9 FP and ScoreV2 66.8. With the environment isolated per child,
// `current` correctly reports UNAVAILABLE — no model is configured for the shipping detector —
// and only the challenger produces numbers.
//
// This is the SAME defect `newShadowVision` records having been caught in the first live start.
// It was fixed there and not here, which is the shape a fix takes when it is applied to the
// symptom's location rather than to every caller of the pattern.
//
// # Why this reads source
//
// The behaviour needs a model, an ONNX Runtime and a corpus, none of which CI has. What can be
// checked anywhere is that the configuration does not escape into the process — and that is the
// whole of the defect: one call to `os.Setenv` in a backend constructor is enough to bring it
// back, silently, with the report still printing two confident rows.
//
// Deleting WithEnv, or reintroducing a process-wide setter, must fail this.
func TestTheBenchmarkBaselineIsNotTheChallenger(t *testing.T) {
	src, err := os.ReadFile("benchscreenparser.go")
	if err != nil {
		t.Fatalf("reading the challenger backend: %v", err)
	}
	text := string(src)

	// NO PROCESS-WIDE CONFIGURATION. The detector settings must reach one child only.
	if strings.Contains(text, "os.Setenv(") {
		t.Error("the challenger backend sets a process-wide environment variable. A bridge " +
			"host spawns its child on first use, so this reaches the BASELINE's plugin " +
			"too — and the benchmark then compares one model against itself.")
	}
	// AND IT CONFIGURES ITS OWN CHILD.
	if !strings.Contains(text, "WithEnv(") {
		t.Error("the challenger backend no longer configures its own child; whatever it " +
			"reads its settings from is shared with every other backend")
	}
	for _, want := range []string{
		"MARCO_VISION_MODEL=", "MARCO_VISION_SIZE=1280",
		"MARCO_VISION_CONF=0.15", "MARCO_VISION_IOU=0.45",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the challenger's frozen setting %q is not passed to its child", want)
		}
	}

	// AND THE BASELINE STILL BUILDS ITS OWN DETECTOR from the ordinary wiring, so "current"
	// in a report means what actually ships rather than an approximation of it.
	base, err := os.ReadFile("benchcurrent.go")
	if err != nil {
		t.Fatalf("reading the baseline backend: %v", err)
	}
	if !strings.Contains(string(base), "newVisionDetector(defaultVisionBridge())") {
		t.Error("the baseline no longer reaches the plugin the Director runs")
	}
	if strings.Contains(string(base), "MARCO_VISION_MODEL") {
		t.Error("the baseline configures the shared vision model, which is the same leak " +
			"from the other side")
	}
}
