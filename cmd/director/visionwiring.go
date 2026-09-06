package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/platform/visionclient"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Wiring the vision provider.
//
// Same shape as the OCR wiring next door, and deliberately so: a detector is a subprocess
// with heavy dependencies, reached over the bridge, and absent on most machines. What
// differs is only which plugin is launched.
//
// A Director with no detector is the ORDINARY case and behaves exactly as it did before
// this existed — the provider is opt-in, so a cycle that does not ask for vision never
// notices, and one that does asks and is told plainly that no detector is configured.

// defaultVisionBridge locates the vision plugin.
//
// $DIRECTOR_VISION first, then beside the executable, then the source tree. The same
// search the OCR bridge uses, for the same reason: Marco's binaries ship together, and a
// development tree has them where they were built.
func defaultVisionBridge() string {
	if p := os.Getenv("DIRECTOR_VISION"); p != "" {
		return p
	}
	if p := os.Getenv("MARCO_VISION"); p != "" {
		return p
	}
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "vision.exe"))
	}
	candidates = append(candidates, filepath.Join("plugins", "vision", "vision.exe"))
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// newVisionDetector builds the detector, reporting why when it cannot.
//
// The unavailability REASON is carried rather than swallowed. "No detector is installed"
// and "this window has nothing in it" are different findings, and a diagnostic that showed
// an empty result for the first would send a user looking for a model that was never the
// problem.
func newVisionDetector(bridgePath string) (vision.Detector, *bridgehost.Host, string) {
	if bridgePath == "" {
		return nil, nil, "no vision plugin found — build plugins/vision and set " +
			"$DIRECTOR_VISION to vision.exe"
	}
	if _, err := os.Stat(bridgePath); err != nil {
		return nil, nil, "the vision plugin is not at " + bridgePath
	}
	// THE MODEL AND ITS CALIBRATION, handed to the child rather than left to its defaults.
	//
	// This path never did, and inherited 640 against a 1280 model — a working detector, a
	// present model and a compatible runtime producing a silent failure that read as "the
	// detector found nothing". The shadow path has always passed these. See
	// screenParserCalibration for why there is now one source.
	model := screenParserModel()
	if model == "" {
		return nil, nil, "no ScreenParser model — set $MARCO_SCREENPARSER_MODEL to " +
			"screenparser-1280.onnx"
	}
	if _, err := os.Stat(model); err != nil {
		return nil, nil, "the ScreenParser model is not at " + model
	}
	if defaultONNXRuntime() == "" {
		return nil, nil, "no ONNX Runtime found — the plugin loads it dynamically; " +
			"vendor it under tools/onnxruntime or set $MARCO_ONNXRUNTIME"
	}
	host := bridgehost.New(bridgePath).WithEnv(visionChildEnv(model)...)
	return visionclient.New(host), host, ""
}

// newVisionProvider builds the perception provider over a detector and the shared capture.
func newVisionProvider(det vision.Detector, cap capture.WindowCapture,
	active func(context.Context) (directorapi.Window, bool)) *vision.Provider {

	return vision.New(det, cap, active)
}

// ── the calibration a ScreenParser child runs under ───────────────────────────

// screenParserCalibration is the configuration `screenparser-1280.onnx` was approved at.
//
// # One source, because two would eventually disagree
//
// These values were FROZEN on the calibration split and validated on held-out evidence. The
// shadow detector has always passed them to its child; the authoritative one never did, and
// inherited whatever the plugin defaulted to — 640 against a 1280 model:
//
//	vision inference: Got invalid dimensions for input: images
//	  index: 2 Got: 640 Expected: 1280
//
// A working detector, a present model, a compatible runtime, and a silent failure that read as
// "the detector found nothing". Two constructors configuring one model differently is the defect;
// this is the one place it is written down.
//
// Deleting this and letting the child default must fail TestBothVisionPathsRunOneCalibration.
func screenParserCalibration(model string) []string {
	return []string{
		"MARCO_VISION_MODEL=" + model,
		"MARCO_VISION_SIZE=1280",
		"MARCO_VISION_CONF=0.15",
		"MARCO_VISION_IOU=0.45",
	}
}

// defaultONNXRuntime is the shared library the vision plugin loads.
//
// # Why this is chosen rather than found
//
// The binding requests a specific ONNX Runtime API version, and a runtime that is too old refuses
// with a message that never reaches a person:
//
//	The requested API version [28] is not available, only API versions [1, 26] are supported
//	in this build. Current ORT Version is: 1.26.0
//
// There were two copies in the tree — a stale 1.26 beside the plugin and the 1.28 the repository
// vendors under tools/ — and the plugin loaded whichever the environment happened to name. So the
// vendored one is preferred explicitly, and $MARCO_ONNXRUNTIME still wins for anybody pointing at
// their own.
func defaultONNXRuntime() string {
	if p := os.Getenv("MARCO_ONNXRUNTIME"); p != "" {
		return p
	}
	candidates := []string{
		filepath.Join("tools", "onnxruntime", "onnxruntime-win-x64-1.28.0", "lib",
			"onnxruntime.dll"),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "tools",
			"onnxruntime", "onnxruntime-win-x64-1.28.0", "lib", "onnxruntime.dll"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// visionChildEnv is everything one ScreenParser child needs, in one place.
//
// Per-child rather than process-wide, for the reason shadowwiring records: a bridge host launches
// its child on first USE, so `os.Setenv` would have handed the authoritative detector the
// experiment's configuration and nobody would have seen it happen.
func visionChildEnv(model string) []string {
	env := screenParserCalibration(model)
	if rt := defaultONNXRuntime(); rt != "" {
		env = append(env, "MARCO_ONNXRUNTIME="+rt)
	}
	return env
}
