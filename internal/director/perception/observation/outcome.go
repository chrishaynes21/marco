package observation

import (
	"context"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// What a provider did, as distinct from what it was asked to do.
//
// # Why this type exists at all
//
// An earlier design stamped each observation with the target from the REQUEST and had
// fusion compare that against the request's own expectation. Both sides were copies of one
// value, so they matched by construction, and the guard passed precisely the stale evidence
// it existed to reject. The race it was meant to catch:
//
//	generation 7 validated → provider begins → window replaced
//	→ generation 8 current → provider returns generation-7 evidence
//
// is invisible to that comparison.
//
// So intent and evidence are now separate values with separate origins:
//
//	ExpectedTarget  what the Director INTENDED this provider to observe
//	ObservedTarget  what the provider can PROVE its evidence belongs to
//
// Fusion trusts neither implicitly. It compares them.
//
// # Why observations stay immutable
//
// The outcome owns collection provenance; the observations inside it remain the read-only
// records they always were. Nothing acquires provenance after the fact — provenance that
// could be assigned later is metadata, and metadata cannot be evidence about collection.

// ProviderState is the closed vocabulary of how a provider fared.
//
// Deliberately more than "how many observations". Zero observations is the same number for
// a provider that read the target and found nothing, one that could not see into it, and
// one that was never installed — and those want three different things done about them.
type ProviderState string

const (
	// StateContributed: the provider observed the target and produced evidence.
	StateContributed ProviderState = "contributed"
	// StateEmpty: the provider observed the CORRECT target and found nothing. A real
	// answer, not a failure.
	StateEmpty ProviderState = "empty"
	// StateUnobservable: the provider reached the target but could not see into it —
	// an application exposing no accessible tree, a protected window.
	StateUnobservable ProviderState = "unobservable"
	// StateUnavailable: the provider itself is not usable — no bridge, no engine, not
	// installed. Says nothing about the target.
	StateUnavailable ProviderState = "unavailable"
	// StateTargetChanged: the target was replaced while the provider was working, so
	// whatever it collected describes a window that is no longer current.
	StateTargetChanged ProviderState = "target_changed"
	// StateProvenanceMismatch: the provider produced evidence but could not prove it
	// belongs to the expected target.
	StateProvenanceMismatch ProviderState = "provenance_mismatch"
	// StateFailed: the provider errored.
	StateFailed ProviderState = "failed"
	// StateTimedOut: the provider exceeded its budget.
	StateTimedOut ProviderState = "timed_out"
	// StateNotRequested: the provider is opt-in and this request did not ask for it.
	//
	// Says nothing about the provider and nothing about the target — it did not look. Held
	// apart from StateEmpty and StateUnavailable, which is the whole point: an opt-in
	// provider that silently reported "empty" would look like a detector that examined the
	// screen and found no controls, and one that reported "unavailable" would send somebody
	// to reinstall a plugin that is working perfectly.
	//
	// It exists because vision's opt-in was enforced on Observe and not on ObserveTargeted,
	// so every ordinary cycle ran an unrequested screen capture and reported the resulting
	// window failure as a degradation. The guard now needs somewhere honest to land.
	StateNotRequested ProviderState = "not_requested"
)

// Contributing reports whether this state offers evidence worth considering.
//
// Only StateContributed does. StateEmpty is a legitimate answer and contributes nothing by
// definition; every other state means the evidence must not be used, whether or not any
// arrived.
func (s ProviderState) Contributing() bool { return s == StateContributed }

// ProviderOutcome is one provider's record for one cycle.
type ProviderOutcome struct {
	Source Source        `json:"source"`
	State  ProviderState `json:"state"`

	// Scope must be DECLARED. Global scope is never inferred from missing provenance —
	// that inference is exactly how a target-scoped provider that forgot to establish
	// its target would slip past the guard.
	Scope directorapi.ObservationScope `json:"scope,omitempty"`

	// ExpectedTarget is intent: what the Director asked this provider to observe.
	ExpectedTarget directorapi.TargetProvenance `json:"expected_target,omitzero"`
	// ObservedTarget is evidence: what the provider established its result belongs to.
	//
	// It must come from a different path than ExpectedTarget. A provider that copies
	// the expectation here has established nothing, and the same-request trap test
	// exists to catch exactly that.
	ObservedTarget directorapi.TargetProvenance `json:"observed_target,omitzero"`

	Observations []Observation `json:"-"`

	StartedAt  time.Time `json:"started_at,omitzero"`
	FinishedAt time.Time `json:"finished_at,omitzero"`

	// Reason explains a non-contributing state in words a person can act on.
	Reason string `json:"reason,omitempty"`

	// Incomplete is why CONTRIBUTED evidence is nonetheless not exhaustive — a node cap,
	// a depth limit, an application that refused part of its tree.
	//
	// Separate from Reason because it is not a failure and must not read as one. The
	// evidence is real and is used; what it cannot support is the negative conclusion. A
	// truncated walk that found no Save button has not established that there isn't one,
	// and collapsing that into "contributed" is how "I could not read this application"
	// starts looking like "the button is not there".
	Incomplete string `json:"incomplete,omitempty"`
}

// Global reports whether this outcome's evidence is desktop-wide.
//
// Global providers are exempt from the target guard because their evidence is not
// window-scoped: monitor topology does not belong to a window and cannot go stale when one
// is replaced.
func (o ProviderOutcome) Global() bool {
	return o.Scope == directorapi.ScopeGlobal
}

// TargetProven reports whether this outcome's evidence may enter belief.
//
// The rule, stated once:
//
//   - a GLOBAL outcome is always eligible — it makes no window claim to be wrong about;
//   - a target-scoped outcome must have an observed target that is Known AND Matches the
//     expectation;
//   - an UNKNOWN observed target is refused. "Could not establish provenance" is not
//     "probably fine", and treating it as agreement would let any provider bypass the
//     guard by declining to answer.
//
// The expectation being unknown means this was not a targeted cycle, and the guard does not
// apply — an ordinary command cycle has no pinned target to be stale relative to.
func (o ProviderOutcome) TargetProven() bool {
	if o.Global() {
		return true
	}
	if !o.ExpectedTarget.Known() {
		return true // untargeted cycle: nothing to compare against
	}
	if !o.ObservedTarget.Known() {
		return false // fail-safe: unproven is not proven
	}
	return o.ObservedTarget.Matches(o.ExpectedTarget)
}

// Usable reports whether fusion should take this outcome's observations.
func (o ProviderOutcome) Usable() bool {
	return o.State.Contributing() && o.TargetProven()
}

// TargetedProvider is a Provider that can establish what it actually observed.
//
// Optional, following the EnvironmentProvider and Refiner pattern. A provider that does not
// implement it is wrapped by the collector into an outcome with no observed target — which
// is refused on a targeted cycle, by design. Opting in is how a provider earns the right to
// contribute to a pinned session.
type TargetedProvider interface {
	Provider
	// ObserveTargeted collects and reports what it can prove about the result.
	//
	// The returned outcome's ObservedTarget must be established from the provider's own
	// post-collection evidence, never copied from req.Target.
	ObserveTargeted(ctx context.Context, req Request) ProviderOutcome
}

// ShadowProvider is a Provider whose evidence may be recorded but never believed.
//
// # Why a marker interface rather than a flag on the outcome
//
// Because a flag can be forgotten, and a type cannot. The guarantee this milestone needs is
// that experimental perception cannot reach belief EVEN IF someone later edits fusion, adds a
// code path, or copies an outcome between slices. A boolean on ProviderOutcome would put that
// guarantee in the hands of every future reader of the struct.
//
// So the split is made of the strongest thing available: shadow outcomes are collected into a
// DIFFERENT FIELD of the cycle. `Cycle.Observations` and `Cycle.Outcomes` — the only two things
// `Admitted()` reads — never contain them. Fusion is not asked to remember which provider is
// experimental, because fusion is never shown one.
//
// # What a shadow provider still owes
//
// Everything else. It runs against the same request, establishes the same target provenance,
// and produces a normal ProviderOutcome — being experimental is not a reason to collect
// untrustworthy evidence. An experiment comparing shadow evidence against belief is worthless
// if the shadow side cannot say which window it looked at.
type ShadowProvider interface {
	Provider
	// ShadowOnly is a marker. It has no arguments and no return value on purpose: there is
	// no per-call, per-request or per-configuration way to become authoritative.
	ShadowOnly()
}

// IsShadow reports whether a provider's evidence must be kept out of belief.
func IsShadow(p Provider) bool {
	_, ok := p.(ShadowProvider)
	return ok
}
