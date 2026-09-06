package main

import "testing"

// BOTH VISION PATHS RUN ONE CALIBRATION.
//
// # The silent failure this closes
//
// `screenparser-1280.onnx` was approved at a frozen size, confidence and IOU. The shadow detector
// has always handed those to its child. The authoritative one never did, and the child defaulted:
//
//	vision inference: Got invalid dimensions for input: images
//	  index: 2 Got: 640 Expected: 1280
//
// A working detector, a present model and a compatible runtime, producing a result that read as
// "the detector found nothing". Two constructors configuring one model differently.
//
// Letting either path fall back to the plugin's defaults must fail this.
func TestBothVisionPathsRunOneCalibration(t *testing.T) {
	env := screenParserCalibration("model.onnx")
	want := map[string]bool{
		"MARCO_VISION_MODEL=model.onnx": true,
		"MARCO_VISION_SIZE=1280":        true,
		"MARCO_VISION_CONF=0.15":        true,
		"MARCO_VISION_IOU=0.45":         true,
	}
	if len(env) != len(want) {
		t.Fatalf("the calibration is %v", env)
	}
	for _, kv := range env {
		if !want[kv] {
			t.Errorf("the calibration carries %q, which the benchmark did not approve", kv)
		}
	}
	// AND THE CHILD ENVIRONMENT IS THAT CALIBRATION plus wherever the runtime is. A path
	// that assembled its own size or confidence would be running a detector nobody measured.
	child := visionChildEnv("model.onnx")
	for _, kv := range env {
		found := false
		for _, c := range child {
			if c == kv {
				found = true
			}
		}
		if !found {
			t.Errorf("the child environment is missing %q", kv)
		}
	}
}
