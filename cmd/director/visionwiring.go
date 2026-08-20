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
	host := bridgehost.New(bridgePath)
	return visionclient.New(host), host, ""
}

// newVisionProvider builds the perception provider over a detector and the shared capture.
func newVisionProvider(det vision.Detector, cap capture.WindowCapture,
	active func(context.Context) (directorapi.Window, bool)) *vision.Provider {

	return vision.New(det, cap, active)
}
