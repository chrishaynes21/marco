package providers

import (
	"context"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Proving what the accessibility walk actually read.
//
// # The race this exists for
//
//	generation 7 validated → UIA snapshot begins → the window is replaced
//	→ generation 8 becomes current → the snapshot returns
//
// The returned tree describes a window that is no longer the target. Nothing in the request
// can reveal that, because the request was correct when it was made. Only re-reading the
// platform AFTER the snapshot can.
//
// So the observed target is established from two pieces of evidence the request cannot
// supply: what the BRIDGE says it walked, and what the PLATFORM says that window is now.

// TargetResolver turns a window the platform reported into Director provenance.
//
// Deliberately the narrowest thing the provider needs. It does not enumerate windows, does
// not select a target, and cannot acquire one — a provider that could choose its own target
// would be answering a different question from the one the session asked.
//
// The second return is false when the observed window cannot be resolved to the Director's
// current target at all. That is NOT an error: a bridge that fell back to the foreground
// window has answered honestly about a window the Director is not tracking, and "unknown"
// is the correct provenance for it.
type TargetResolver interface {
	ResolveObserved(ctx context.Context, window directorapi.WindowID,
		application string) (directorapi.TargetProvenance, bool)
}

// WithTargetResolver lets the accessibility provider prove what it observed.
//
// Optional. Without it the provider still works for ordinary command perception, where no
// target is pinned and nothing has to be proven — but it cannot contribute to a targeted
// cycle, because it cannot establish provenance. That is the intended failure mode: a
// provider earns contribution rights by proving what it saw.
func (a *Accessibility) WithTargetResolver(r TargetResolver) *Accessibility {
	a.resolver = r
	return a
}

var _ observation.TargetedProvider = (*Accessibility)(nil)

// ObserveTargeted collects and reports what it can prove about the result.
//
// The ObservedTarget never comes from req.Target. It is resolved from the window the bridge
// reports it walked, re-validated against the platform as it is now.
func (a *Accessibility) ObserveTargeted(ctx context.Context,
	req observation.Request) observation.ProviderOutcome {

	out := observation.ProviderOutcome{
		Source:    directorapi.SourceAccessibility,
		Scope:     directorapi.ScopeTarget,
		StartedAt: time.Now(),
	}
	if req.Target != nil {
		out.ExpectedTarget = *req.Target
	}
	defer func() { out.FinishedAt = time.Now() }()

	scope := directorapi.WindowID("")
	if req.Window != nil {
		scope = *req.Window
	}

	snap, err := a.src.Snapshot(ctx, scope)
	if err != nil {
		out.State, out.Reason = observation.StateFailed, err.Error()
		return out
	}

	obs := a.observationsFrom(snap)

	// A truncated walk still degrades the cycle, whatever the provenance verdict turns
	// out to be. Carried here rather than at each return below, so a path added later
	// cannot forget it.
	if snap.Partial {
		out.Incomplete = snap.Reason
		if out.Incomplete == "" {
			out.Incomplete = "the accessibility walk was cut short"
		}
	}

	// Untargeted cycle: ordinary command perception, nothing pinned, nothing to prove.
	// The state still reflects reality so diagnostics stay honest.
	if req.Target == nil || !req.Target.Known() {
		out.State = stateFor(len(obs), snap)
		out.Observations = obs
		return out
	}

	// Targeted: the observed window has to be established, not assumed.
	if a.resolver == nil {
		out.State = observation.StateProvenanceMismatch
		out.Reason = "this provider cannot establish which window it observed, so its " +
			"evidence cannot be attributed to the target"
		out.Observations = obs
		return out
	}
	if snap.ObservedWindow == "" {
		// The bridge could not say what it walked. Unproven, not "probably right".
		out.State = observation.StateProvenanceMismatch
		out.Reason = "the accessibility source did not report which window it read"
		out.Observations = obs
		return out
	}

	observed, ok := a.resolver.ResolveObserved(ctx, snap.ObservedWindow, snap.ObservedApp)
	if !ok {
		out.State = observation.StateTargetChanged
		out.Reason = "the window the accessibility source read is not the Director's " +
			"current target; it was replaced or the source fell back to another window"
		out.Observations = obs
		return out
	}
	out.ObservedTarget = observed

	// The comparison itself lives on the outcome — one canonical implementation, so a
	// second opinion cannot drift from it.
	if !out.TargetProven() {
		out.State = observation.StateTargetChanged
		out.Reason = "the target was replaced while the accessibility snapshot was in flight"
		// Observations are retained for diagnostics and are not usable: Usable()
		// requires TargetProven(), which is false here.
		out.Observations = obs
		return out
	}

	out.State = stateFor(len(obs), snap)
	out.Observations = obs
	return out
}

// stateFor distinguishes an empty answer from an unreadable one.
//
// Zero observations is the same number either way, and the difference is what a person
// debugging "why can't it see this?" most needs: EMPTY means the target was read and holds
// nothing addressable, UNOBSERVABLE means the platform could not see into it at all.
func stateFor(n int, snap directorapi.AccessibilitySnapshot) observation.ProviderState {
	if n > 0 {
		return observation.StateContributed
	}
	// A partial walk that produced nothing did not establish structure — it was cut
	// short, refused, or timed out. That is not the same as looking and finding none.
	if snap.Partial {
		return observation.StateUnobservable
	}
	return observation.StateEmpty
}
