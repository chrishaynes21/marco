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

// shadowUngatedEnv runs the experiment on its cadence even where the primary reading claims to
// be sufficient.
//
// # Why this exists
//
// The purchase policy declines an inference on a positive statement that the current reading
// suffices, which is correct and measured for a desktop: ScreenParser added nothing to a
// healthy Settings page and cost ~1s a frame to add it. On a GAME it is the wrong answer to
// the right question. Accessibility exposes a window and a pane, `SufficiencyOf` reports that
// as content reached, and the detector — the only sensor that can see the menu at all — is
// declined for the rest of the epoch. Measured live on Trackmania: ONE inference in 78 looks.
//
// So the two questions are not the same question. "Did accessibility describe this window"
// cannot answer "can anything see which entry is highlighted", and
// [[ADR-102-a-detector-earns-its-place-where-perception-fails]] is about the ABSENCE of better
// evidence, which a claim of structural sufficiency does not establish on a surface that has
// no accessibility tree to be sufficient about.
//
// Changing that policy is a product decision with a measured cost, and it is not made here.
// This is the switch that lets the experiment be RUN on such a surface so the decision can be
// made on evidence — opt-in, off by default, and reported on every sample so no number can be
// read without knowing which policy produced it.
//
// # It removed one of two gates, and that is the lesson
//
// The policy is asked in TWO places, and this variable reaches only one of them. The other is
// the observation request's own source set — `observewiring.go` asks the same policy whether to
// put vision in `Include`, and the vision provider is opt-in on exactly that. So with this set,
// the provider claimed its slot, called its detector, and the detector returned in 0ms having
// been asked for nothing: 116 of 122 counted inferences on the first live run, every one of
// them then reported as an inference that could not prove its window. The report said UNGATED
// throughout, which is the worst thing a switch can say while being wrong.
//
// The repair is in shadow.Provider, which now asks its own detector for its own sources rather
// than inheriting a decision about the authoritative sensors — see ADR-149. What belongs HERE
// is the warning: a switch named after a policy must account for every place that policy is
// consulted, and "the gate is removed" is a claim about a system, not about a line.
const shadowUngatedEnv = "MARCO_SHADOW_UNGATED"

// shadowUngated reports whether the experiment was asked to ignore the purchase policy.
func shadowUngated() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(shadowUngatedEnv))) {
	case "", "0", "no", "false", "off":
		return false
	}
	return true
}

// attachShadowGate puts the experiment behind the purchase policy, unless asked not to.
//
// A method rather than three lines inside newRuntime so the decision can be entered by a test
// holding a real shadow provider. Without that, the one line that makes $MARCO_SHADOW_UNGATED
// mean anything would be the kind nothing checks: everything still builds, every number is
// still correct, and the switch quietly does nothing.
//
// Deleting this must fail TestTheSwitchDecidesWhetherThePolicyIsAttached. Deleting its CALL
// from NewRuntime is not caught by anything — see ADR-148; what catches it instead is that
// the session report then says UNGATED on every ordinary desktop session.
func (r *Runtime) attachShadowGate() {
	if r == nil || r.shadowVision == nil {
		return
	}
	if gate := r.shadowGate(); gate != nil {
		r.shadowVision.WhenWorthIt(gate)
	}
}

// shadowGate is the purchase policy the experiment should run behind, or nil for none.
func (r *Runtime) shadowGate() func() bool {
	if shadowUngated() {
		return nil
	}
	return func() bool { return r.moreEvidenceIsWorthBuying().Buy }
}

// shadowGateNote describes the policy a provider is ACTUALLY running behind.
//
// Taken from the provider rather than from the environment on purpose. Asking the environment
// describes the configuration somebody intended; asking the provider describes the system that
// produced the numbers, and when one line at the composition root goes missing those are
// different systems. This way the report says UNGATED on an ordinary desktop session, loudly,
// instead of quietly attributing an ungated run to the default policy.
func shadowGateNote(gated bool) string {
	if gated {
		return "declines a slot when the primary reading claims to be sufficient"
	}
	return "UNGATED — it runs on its cadence even where the primary reading claims to be " +
		"sufficient (set by $" + shadowUngatedEnv + ", or by nothing having attached a policy)"
}

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
	// The runtime is CHOSEN rather than required from the environment. It used to demand
	// $MARCO_ONNXRUNTIME, so a repository that vendors a compatible runtime still refused to
	// start the detector until somebody exported a path by hand. See defaultONNXRuntime.
	if defaultONNXRuntime() == "" {
		return nil, nil, "no ONNX Runtime found — the plugin loads it dynamically; " +
			"vendor it under tools/onnxruntime or set $MARCO_ONNXRUNTIME"
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
	host := bridgehost.New(bridge).WithEnv(visionChildEnv(model)...)
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
	// AND THE GEOMETRY OF WHAT IT COULD NOT READ.
	//
	// On the EXPERIMENT'S instance only. The authoritative provider is a different object
	// with different thresholds and this flag unset, so its text acceptance and everything
	// fusion does with it are exactly as they were — which is the condition this repair was
	// authorised under.
	//
	// What it buys, measured in Experiment-024: on a real Trackmania menu the detector
	// returns ~70 boxes a frame, ~all of them `text`, and a `text` box nobody could read was
	// discarded before selection.Measure could ask how it was shaded. The session therefore
	// could not name a list across 150 seconds of a player navigating one.
	//
	// It cannot reach belief. Shapes travel on the provider's Diagnostics, never in the
	// observation stream, and this provider is additionally wrapped in a shadow.Provider
	// whose evidence the collector routes to Cycle.Shadow.
	//
	// Deleting this must fail TestTheExperimentKeepsGeometryItCannotRead.
	inner.KeepUnreadableShapes = true

	return shadow.NewProvider(inner, cadence)
}
