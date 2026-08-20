package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/shadow"
	visionclient "github.com/chaynes-simpleclouds/marco/internal/platform/visionclient"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Turning an experimental detector on, deliberately and never by accident.
//
// # Why this is opt-in at the process level
//
// ScreenParser costs ~1.25 GB resident and ~0.9s per inference. A Director that loaded it
// because it happened to be installed would make every user of every command pay for an
// experiment they did not ask for — and would do it invisibly, since a model that loads
// successfully produces no message.
//
// So nothing here runs unless asked. With the variable unset there is no model, no ONNX
// Runtime session, no plugin process and no memory: `newShadowVision` returns nil before it
// touches anything expensive, and the collector is built without it.
//
// # Why an environment variable rather than a flag
//
// `observe-game` is a client command; it talks to a Director service whose providers were
// built at startup. A per-command flag would have to be plumbed through the service protocol
// and would let a client change the process's memory profile mid-session, which is not a power
// a client should have. The activation surface can become a flag once the experiment has
// earned a permanent place; until then the honest surface is one a user sets when they start
// the service they are experimenting with.

// shadowVisionEnv names the detector to run in shadow, empty for none.
const shadowVisionEnv = "MARCO_SHADOW_VISION"

// shadowCadenceEnv overrides the shadow sampling cadence.
const shadowCadenceEnv = "MARCO_SHADOW_CADENCE"

// shadowMaxLabels and shadowMaxScreenTexts bound one experimental label pass. See the note at
// the assignment for the arithmetic.
const (
	shadowMaxLabels      = 6
	shadowMaxScreenTexts = 2
)

// shadowVisionRequested reports which shadow detector was asked for.
func shadowVisionRequested() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv(shadowVisionEnv)))
}

// shadowCadence is how often a shadow inference may start.
func shadowCadence() time.Duration {
	if v := strings.TrimSpace(os.Getenv(shadowCadenceEnv)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return shadow.DefaultCadence
}

// newShadowVision builds the shadow provider, or returns nil with the reason it could not.
//
// The reason is carried rather than swallowed, the same way the authoritative vision detector
// carries its own. "You asked for a shadow detector and it did not start" must never look like
// "the experiment ran and found nothing" — that confusion has cost this project two
// milestones already, and a shadow session that silently observed nothing would produce a
// confident report of zero information gain.
func newShadowVision(cap capture.WindowCapture,
	active func(context.Context) (directorapi.Window, bool),
	reader vision.LabelReader) (*shadow.Provider, *bridgehost.Host, string) {

	want := shadowVisionRequested()
	if want == "" {
		// The common path. Nothing loaded, nothing allocated, no reason to report.
		return nil, nil, ""
	}
	if want != "screenparser" {
		return nil, nil, fmt.Sprintf("$%s=%q is not a known shadow detector (screenparser)",
			shadowVisionEnv, want)
	}

	model := screenParserModel()
	if model == "" {
		return nil, nil, "no ScreenParser model — set $MARCO_SCREENPARSER_MODEL to " +
			"screenparser-1280.onnx"
	}
	if _, err := os.Stat(model); err != nil {
		return nil, nil, "the ScreenParser model is not at " + model
	}
	if os.Getenv("MARCO_ONNXRUNTIME") == "" {
		return nil, nil, "$MARCO_ONNXRUNTIME is not set — the plugin loads the ONNX " +
			"Runtime shared library dynamically and cannot find it"
	}

	// The shadow bridge's configuration goes to THAT CHILD ONLY.
	//
	// It was process-wide `os.Setenv` first, and that was a real defect caught in the first
	// live start: a bridge host launches its child on first USE, so both the authoritative
	// and the shadow plugin spawned after the setters ran, and both inherited ScreenParser.
	// The experiment had silently become the authoritative detector — two vision processes,
	// same model, no error anywhere. Per-child environment is what makes that impossible;
	// see bridgehost.Host.WithEnv.
	//
	// The values themselves are the ones FROZEN on the calibration split and validated on
	// held-out evidence. A shadow session at different settings would not be observing the
	// detector the benchmark approved.
	bridge := defaultVisionBridge()
	if bridge == "" {
		return nil, nil, "no vision plugin found — build plugins/vision and set " +
			"$DIRECTOR_VISION to vision.exe"
	}
	host := bridgehost.New(bridge).WithEnv(
		"MARCO_VISION_MODEL="+model,
		"MARCO_VISION_SIZE=1280",
		"MARCO_VISION_CONF=0.15",
		"MARCO_VISION_IOU=0.45",
	)
	detector := visionclient.New(host)

	return newShadowProvider(detector, cap, active, reader, shadowCadence()), host, ""
}

// newShadowProvider composes the experiment: a detector, a capture source, a label reader and a
// cadence gate.
//
// Split out of newShadowVision so the composition can be entered with a fake detector and a
// fake reader. Everything above this line is about FINDING a model on disk and starting a
// plugin; everything below is the arrangement that decides what the experiment can see and say,
// and that is the part a wiring test has to be able to reach. Nothing here is duplicated by the
// test — the test calls this function.
func newShadowProvider(detector vision.Detector, cap capture.WindowCapture,
	active func(context.Context) (directorapi.Window, bool),
	reader vision.LabelReader, cadence time.Duration) *shadow.Provider {

	// An ordinary vision provider — the SAME type authoritative vision uses, over its own
	// detector. It therefore inherits capture provenance, frame attribution and the
	// expected/observed target split for free, which is what makes its evidence comparable
	// with belief rather than merely adjacent to it.
	inner := vision.New(detector, cap, active)
	inner.ProviderName = "screenparser"
	// Judged at the floors it was calibrated at. Confidence scales are not comparable
	// between models, and running ScreenParser behind another detector's floors is the
	// mistake that made a working model look like a failing one, twice.
	inner.Thresholds.MinConfidence = 0.15
	inner.Thresholds.MinStructuralConfidence = 0.15
	// Reading the words inside its own boxes, on a much tighter budget than the
	// authoritative detector's.
	//
	// The arithmetic decides this, not taste. A scoped reading measured 230–730ms and they
	// are serial, so the default ceiling of 24 would spend 6–17 seconds inside a cycle the
	// shadow cadence expects to finish in two. Six is what fits a label pass without the
	// experiment becoming the thing the machine is doing, and the pass runs only when the
	// session's own label budget allows it — roughly one sample in six.
	//
	// A cap that drops readings is reported, never silent: LabelsSkipped and
	// LabelsUnsayable are on the diagnostic precisely so "the detector found nothing
	// nameable" cannot be confused with "we stopped looking".
	inner.Reader = reader
	inner.Labels.MaxLabels = shadowMaxLabels
	inner.Labels.MaxScreenTexts = shadowMaxScreenTexts

	return shadow.NewProvider(inner, cadence)
}
