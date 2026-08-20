package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/shadow"
)

// Activation must be explicit, and the default must cost nothing.
//
// The product consequence is concrete: ScreenParser is ~1.25 GB resident. A Director that
// loaded it because the model happened to be on disk would charge every user of every command
// for an experiment they did not ask for, and would do it silently — a model that loads
// successfully prints nothing.

func TestShadowVisionIsOffUnlessAsked(t *testing.T) {
	t.Setenv(shadowVisionEnv, "")
	p, host, reason := newShadowVision(nil, nil, nil)
	if p != nil {
		t.Error("a shadow provider was built without being requested")
	}
	if host != nil {
		t.Error("a plugin process was started without being requested — this is where the " +
			"1.25 GB comes from")
	}
	if reason != "" {
		t.Errorf("unrequested shadow reported %q; not asking for something is not a "+
			"failure to report", reason)
	}
}

// An unknown detector name is refused with a reason, never ignored. Silently running nothing
// because of a typo would produce a confident report of zero information gain.
func TestAnUnknownShadowDetectorIsRefusedLoudly(t *testing.T) {
	t.Setenv(shadowVisionEnv, "screenpasrer") // transposed
	p, _, reason := newShadowVision(nil, nil, nil)
	if p != nil {
		t.Fatal("a provider was built for an unknown detector name")
	}
	if !strings.Contains(reason, "not a known shadow detector") {
		t.Errorf("reason %q does not name the problem", reason)
	}
}

// Requested but unavailable must report WHY. "You asked for it and it did not start" must
// never be indistinguishable from "it ran and saw nothing" — that confusion has already cost
// this project two milestones.
func TestARequestedButUnavailableShadowExplainsItself(t *testing.T) {
	t.Setenv(shadowVisionEnv, "screenparser")
	t.Setenv("MARCO_SCREENPARSER_MODEL", "does-not-exist.onnx")
	p, _, reason := newShadowVision(nil, nil, nil)
	if p != nil {
		t.Fatal("a provider was built over a model that is not there")
	}
	if reason == "" {
		t.Fatal("an unavailable shadow detector reported no reason")
	}
	if !strings.Contains(reason, "does-not-exist.onnx") {
		t.Errorf("reason %q does not say which model was missing", reason)
	}
}

// The cadence is configurable but defaults to the measured-safe value.
func TestShadowCadenceDefaultsToTheMeasuredValue(t *testing.T) {
	t.Setenv(shadowCadenceEnv, "")
	if got := shadowCadence(); got != shadow.DefaultCadence {
		t.Errorf("cadence = %s, want %s", got, shadow.DefaultCadence)
	}
	t.Setenv(shadowCadenceEnv, "5s")
	if got := shadowCadence(); got.String() != "5s" {
		t.Errorf("cadence = %s, want 5s", got)
	}
	// Nonsense falls back rather than disabling the gate. A zero cadence would run the
	// detector on every cycle and saturate the machine the player is using.
	t.Setenv(shadowCadenceEnv, "banana")
	if got := shadowCadence(); got != shadow.DefaultCadence {
		t.Errorf("a malformed cadence gave %s; it must fall back, never disable the gate", got)
	}
}

// The runtime must register the shadow provider as a ShadowProvider. If it were ever
// registered as a plain provider, its evidence would land in Cycle.Outcomes and be admitted.
func TestTheRegisteredShadowProviderIsStructurallyShadow(t *testing.T) {
	var p observation.Provider = shadow.NewProvider(nil, 0)
	if !observation.IsShadow(p) {
		t.Fatal("the shadow provider is not recognised as one; the collector would route " +
			"its evidence into the collection Admitted() reads")
	}
}

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
		t.Skip("the ScreenParser model is not present; this test needs construction to complete")
	}
	t.Setenv("MARCO_SCREENPARSER_MODEL", model)

	// And a plugin path that resolves from the test working directory, for the same
	// reason: an unresolvable one returns before the code under test.
	plugin := filepath.Join("..", "..", "plugins", "vision", "vision.exe")
	if _, err := os.Stat(plugin); err != nil {
		t.Skip("the vision plugin is not built; this test needs construction to complete")
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
