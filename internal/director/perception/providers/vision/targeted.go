package vision

import (
	"context"
	"errors"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Vision as an evidence producer.
//
// The detector knows nothing about window identity and must not: it is handed pixels and
// returns boxes. What those pixels were a picture OF is established by the capture layer,
// after the grab — see providers.ProvenCapture.
//
// So this method's whole job is to report the frame's provenance faithfully, and to map
// what actually happened onto the provider-state vocabulary without flattening the
// distinctions that matter. In particular a capture-ownership failure is never reported as
// "empty": one means the pixels could not be attributed, the other means they were read and
// held nothing.

var _ observation.TargetedProvider = (*Provider)(nil)

// ObserveTargeted runs one vision pass and reports what it can prove about the result.
//
// ObservedTarget comes from the FRAME, which the capture layer established after the
// picture was taken. It is never copied from req.Target: the request says what was intended,
// and a detector that returned boxes from a window replaced mid-capture would satisfy that
// intent perfectly while describing something else.
func (p *Provider) ObserveTargeted(ctx context.Context,
	req observation.Request) observation.ProviderOutcome {

	out := observation.ProviderOutcome{
		Source:    directorapi.SourceVision,
		Scope:     directorapi.ScopeTarget,
		StartedAt: time.Now(),
	}
	if req.Target != nil {
		out.ExpectedTarget = *req.Target
	}

	// THE OPT-IN, and it belongs here as much as it belongs on Observe.
	//
	// Vision looks at NOTHING unless the request asks for it by name. That is the trigger
	// policy, and this method used to bypass it: the collector prefers ObserveTargeted for
	// any provider that implements it, so once vision became a TargetedProvider every
	// ordinary perception cycle ran an unrequested pass. On a Director with the vision
	// plugin configured that is a screen capture per cycle that nobody asked for; on one
	// without a resolvable window it is the "no window to look at" degradation that made
	// every live diagnosis look like a targeting fault.
	//
	// Not-asked-for is NOT-RUN, and not-run is not a failure: the outcome carries no
	// observations, no error and no reason, so nothing appears in Degraded and nothing
	// pretends the detector saw an empty screen.
	if !req.Includes(directorapi.SourceVision) {
		out.State = observation.StateNotRequested
		out.FinishedAt = out.StartedAt
		return out
	}

	obs, diag, err := p.Look(ctx, req)
	out.FinishedAt = time.Now()
	out.Observations = obs

	// The frame's own provenance, whatever came of the pass. Recorded even on a refusal
	// so a diagnostic can say WHICH window the refused pixels belonged to.
	if t := p.lastFrameTarget(); t != nil {
		out.ObservedTarget = *t
	}

	if err != nil {
		out.State, out.Reason = visionFailureState(err, diag), failureReason(diag, err)
		return out
	}

	// Untargeted cycle: ordinary command perception pins nothing.
	if req.Target == nil || !req.Target.Known() {
		out.State = visionSuccessState(len(obs))
		return out
	}

	if !out.ObservedTarget.Known() {
		out.State = observation.StateProvenanceMismatch
		out.Reason = "the capture could not be attributed to a tracked window, so these " +
			"detections cannot be shown to describe the target"
		return out
	}
	if !out.TargetProven() {
		out.State = observation.StateTargetChanged
		out.Reason = "the target was replaced while the frame was being captured; these " +
			"detections describe the window that was there before"
		// Retained, unusable. Usable() refuses them — a diagnostic that showed nothing
		// could not explain what was rejected or why.
		return out
	}

	out.State = visionSuccessState(len(obs))
	return out
}

// visionSuccessState separates a read that found nothing from one that found something.
//
// Note what is NOT here: unobservable. A vision pass that returns no boxes has looked at
// the pixels and found no controls, which is exactly EMPTY. Reaching for "unobservable"
// because a frame was uninteresting would make the word mean nothing on the provider that
// genuinely needs it.
func visionSuccessState(n int) observation.ProviderState {
	if n > 0 {
		return observation.StateContributed
	}
	return observation.StateEmpty
}

// visionFailureState classifies a failed pass.
//
// A capture-ownership failure must never become StateEmpty. "The pixels could not be
// attributed" and "there was nothing in the picture" are different facts, and collapsing
// them is how a targeting defect hides as an uninteresting screen.
func visionFailureState(err error, diag Diagnostics) observation.ProviderState {
	var unavailable *Unavailable
	if errors.As(err, &unavailable) {
		return observation.StateUnavailable
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return observation.StateTimedOut
	}
	if !diag.Available {
		return observation.StateUnavailable
	}
	// A stale or unattributable capture is a target problem, not a detector one.
	if diag.Counters.RejectedStaleCapture > 0 {
		return observation.StateTargetChanged
	}
	return observation.StateFailed
}

func failureReason(diag Diagnostics, err error) string {
	if diag.Error != "" {
		return diag.Error
	}
	if diag.Unavailable != "" {
		return diag.Unavailable
	}
	return err.Error()
}

// lastFrameTarget is the provenance of the most recent frame this provider captured.
func (p *Provider) lastFrameTarget() *directorapi.TargetProvenance {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.frameTarget
}
