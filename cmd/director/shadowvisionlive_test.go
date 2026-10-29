//go:build vision

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The one shadow-detector test that needs a REAL model and a BUILT plugin, behind `-tags vision`.
//
// # Why it is not in the default suite
//
// It has to reach the end of construction to say anything, and construction stats the model file
// and the plugin binary and returns early without them. So it carried two `t.Skip`s -- and a skip
// is green. On a machine where `setup.ps1` had fetched the weights and `plugins/vision/build.cmd`
// had been run with gcc, it ran; in a clean checkout it skipped, and `cmd/gate` reported a
// finding, because a conditional skip is the one thing `gate-skips.txt` cannot express.
//
// The model is vendored under `tools/vision-export/weights/` and the plugin needs gcc, and both
// of those directories are ignored on purpose: they are fetched and built, not committed. So
// unlike a packed bundle, this IS something supplied from outside the repository, and an
// explicitly selected integration test is the right shape for it:
//
//	go test -tags vision ./cmd/director/
//
// # And under the tag it FAILS rather than skips
//
// Because selecting it is asking for it. A skip here would be the same silence one level up:
// somebody ran the command, nothing happened, and the output said `ok`.
//
// ONLY THIS TEST MOVED. `shadowwiring_test.go` holds five others -- the detector being off
// unless asked, an unknown one refused, a requested-but-unavailable one explaining itself, the
// cadence default, and the registered provider being structurally shadow -- and not one of them
// touches a model or a plugin. Tagging the file would have taken five claims out of the default
// suite, which is the quiet loss of coverage the desktop slice exists to prevent.
//
// See [[ADR-270-packaging-is-gated-on-a-clean-checkout]].

// The shadow detector must not configure the authoritative one.
//
// This is a regression test for a defect caught in the first live start, not a hypothetical.
// The shadow bridge was configured with process-wide os.Setenv; a bridge host launches its
// child on first USE rather than at construction; so both plugin children spawned after the
// setters ran and both inherited ScreenParser. Two vision processes, same model, no error —
// the experiment had silently become the authoritative detector.
func TestBuildingTheShadowDetectorDoesNotTouchTheProcessEnvironment(t *testing.T) {
	t.Setenv(shadowVisionEnv, "screenparser")
	t.Setenv("MARCO_ONNXRUNTIME", "anything-non-empty")
	// A model that REALLY EXISTS, so construction runs to completion. Pointing this at a
	// missing file made an earlier version of this test vacuous: it returned at the stat
	// check and never reached the code being tested, then passed against the defect.
	model := filepath.Join("..", "..", "tools", "vision-export", "weights", "screenparser-1280.onnx")
	if _, err := os.Stat(model); err != nil {
		t.Fatalf("the ScreenParser model is not at %s, and this test is behind "+
			"`-tags vision`, so running it is asking for it: construction "+
			"returns at the stat check without it and the test would say "+
			"nothing. Run `setup.ps1` to fetch the weights. (%v)", model, err)
	}
	t.Setenv("MARCO_SCREENPARSER_MODEL", model)

	// And a plugin path that resolves from the test working directory, for the same
	// reason: an unresolvable one returns before the code under test.
	plugin := filepath.Join("..", "..", "plugins", "vision", "vision.exe")
	if _, err := os.Stat(plugin); err != nil {
		t.Fatalf("the vision plugin is not at %s, and this test is behind "+
			"`-tags vision`, so running it is asking for it. Run "+
			"`plugins/vision/build.cmd`, which needs gcc. (%v)", plugin, err)
	}
	t.Setenv("DIRECTOR_VISION", plugin)

	// Whatever the authoritative side had configured must survive untouched.
	t.Setenv("MARCO_VISION_MODEL", "authoritative-model.onnx")
	t.Setenv("MARCO_VISION_CONF", "0.50")

	newShadowVision(nil, nil, nil)

	if got := os.Getenv("MARCO_VISION_MODEL"); got != "authoritative-model.onnx" {
		t.Errorf("MARCO_VISION_MODEL is now %q — building the shadow detector overwrote the "+
			"authoritative detector's configuration, and because bridges spawn lazily the "+
			"authoritative plugin would inherit it", got)
	}
	if got := os.Getenv("MARCO_VISION_CONF"); got != "0.50" {
		t.Errorf("MARCO_VISION_CONF is now %q, want 0.50", got)
	}
}
