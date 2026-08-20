package providers

import (
	"context"
	"errors"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Collector runs the providers and assembles one observation cycle.
//
// It does not interpret anything it collects. Its whole responsibility is that a cycle
// is a faithful record of what the sources said, INCLUDING the ones that said nothing:
// a provider that failed leaves a SourceFailure, not a gap. That distinction is the
// one thing perception must never lose — "there is no Save button" and "I could not
// read this application" have to stay separable all the way to the confidence score a
// policy gate reads, and a collector that silently dropped a failed provider would
// merge them at the first step.
type Collector struct {
	providers []observation.Provider
}

// NewCollector returns a collector over the given providers, in priority order.
func NewCollector(p ...observation.Provider) *Collector {
	return &Collector{providers: p}
}

// Providers lists the registered providers, for diagnostics.
func (c *Collector) Providers() []observation.Provider {
	return append([]observation.Provider(nil), c.providers...)
}

// Collect runs one observation cycle.
//
// Providers run in order rather than concurrently. Concurrency would be free wall-clock
// today — there is one provider doing real work — and would immediately cost
// determinism: observations would arrive in a different order every cycle, and
// clustering breaks ties by input order. Fixture reproducibility is worth more than
// milliseconds until there is a second slow source to overlap with.
//
// It never returns an error. A cycle in which everything failed is a legitimate,
// fully-described state: no observations, every provider in Failures. Turning that
// into an error would let a caller retry it as though the desktop were unreachable,
// when the honest answer is that the Director looked and could not see.
func (c *Collector) Collect(ctx context.Context, req observation.Request) observation.Cycle {
	started := time.Now()
	cycle := observation.Cycle{
		ID:        observation.NewCycleID(started),
		StartedAt: started,
		Request:   req,
	}

	for _, p := range c.providers {
		if !wanted(req, p) {
			continue
		}
		// A provider that can prove what it observed is asked to. One that cannot is
		// still run, and still wrapped in an outcome — with no observed target, which
		// is refused on a targeted cycle. That is the intended shape: contributing to a
		// pinned session is earned by proving, not granted by being registered.
		out, err := run(ctx, p, req)

		// The branch. A shadow provider's evidence goes to a field fusion is never handed,
		// and goes there ALONE — not additionally into Outcomes or Observations. This is
		// the one place authoritative and experimental perception diverge, and it is here
		// rather than in fusion so that fusion needs no notion of an experiment at all.
		//
		// Its failures are not recorded as cycle Failures either: a world degraded because
		// an EXPERIMENT could not see would let a shadow provider suppress belief, which is
		// influence by another name.
		if observation.IsShadow(p) {
			if err != nil {
				out.Reason = shadowReason(out.Reason, err)
			}
			cycle.Shadow = append(cycle.Shadow, out)
			continue
		}

		cycle.Outcomes = append(cycle.Outcomes, out)
		cycle.Observations = append(cycle.Observations, out.Observations...)

		if err == nil {
			continue
		}
		var partial *Degraded
		if errors.As(err, &partial) {
			// Partial: the evidence collected is real and is kept. The degradation is
			// recorded alongside it.
			cycle.Failures = append(cycle.Failures, directorapi.SourceFailure{
				Source: partial.Source,
				Reason: partial.Reason,
			})
			continue
		}
		cycle.Failures = append(cycle.Failures, directorapi.SourceFailure{
			Source: primarySource(p),
			Reason: err.Error(),
		})
	}

	// Refinement, then environment. Both after every provider has reported, because a
	// refiner is defined over the whole cycle.
	for _, p := range c.providers {
		r, ok := p.(observation.Refiner)
		if !ok || !wanted(req, p) {
			continue
		}
		cycle.Observations = r.Refine(cycle.Observations)
	}
	for _, p := range c.providers {
		e, ok := p.(observation.EnvironmentProvider)
		if !ok || !wanted(req, p) {
			continue
		}
		merge(&cycle.Environment, e.Environment(ctx))
	}

	cycle.CompletedAt = time.Now()
	return cycle
}

// run asks a provider for evidence and for whatever it can prove about it.
//
// # Why an untargeted provider still gets an outcome
//
// So that "this provider established no target" is a recorded fact rather than an absence.
// A provider outside the TargetedProvider interface is wrapped here into an outcome whose
// ObservedTarget is zero — unknown, which the guard refuses on a targeted cycle and ignores
// on an ordinary one. Nothing is inferred on its behalf and nothing is assumed about it.
//
// The error is returned rather than folded into the outcome because the two answer different
// questions. State is what the provider concluded about the TARGET; the error is what went
// wrong in the CALL, and the collector's existing Degraded handling depends on seeing it.
func run(ctx context.Context, p observation.Provider,
	req observation.Request) (observation.ProviderOutcome, error) {

	if tp, ok := p.(observation.TargetedProvider); ok {
		out := tp.ObserveTargeted(ctx, req)
		if out.Source == "" {
			out.Source = primarySource(p)
		}
		// A provider that could not be used at all still has to reach the cycle's
		// Failures list, or a caller reading only Failures would see a clean cycle.
		//
		// PROVENANCE states are excluded here on purpose. "Your evidence does not
		// describe the target" is a conclusion about admissibility, and admissibility is
		// fusion's to decide and to report — recording it here as well would put the same
		// refusal in the world's Degraded list twice, from two components that would then
		// have to be kept in agreement forever.
		if unusable(out.State) && out.Reason != "" {
			return out, &Degraded{Source: out.Source, Reason: out.Reason}
		}
		// Contributed, but not exhaustively. The evidence is kept and the shortfall is
		// recorded beside it — the same bargain the plain path has always struck.
		if out.Incomplete != "" {
			return out, &Degraded{Source: out.Source, Reason: out.Incomplete}
		}
		return out, nil
	}

	obs, err := p.Observe(ctx, req)
	out := observation.ProviderOutcome{
		Source:       primarySource(p),
		State:        untargetedState(obs, err),
		Observations: obs,
	}
	if req.Target != nil {
		out.ExpectedTarget = *req.Target
	}
	if err != nil {
		out.Reason = err.Error()
	}
	return out, err
}

// unusable reports whether a state means the provider itself could not deliver, as opposed
// to delivering evidence that turned out not to be admissible.
//
// StateEmpty is absent because it is a real answer. The two provenance states are absent
// because they belong to fusion — see run.
func unusable(s observation.ProviderState) bool {
	switch s {
	case observation.StateFailed, observation.StateTimedOut,
		observation.StateUnavailable, observation.StateUnobservable:
		return true
	}
	return false
}

// untargetedState maps a plain provider's return onto the outcome vocabulary.
//
// Deliberately conservative. A plain Provider cannot distinguish "read it and found nothing"
// from "could not see into it" — that distinction is exactly what the richer interface
// exists to carry — so a silent success is reported as empty rather than as either.
func untargetedState(obs []observation.Observation, err error) observation.ProviderState {
	if err != nil {
		var partial *Degraded
		if errors.As(err, &partial) && len(obs) > 0 {
			return observation.StateContributed
		}
		return observation.StateFailed
	}
	if len(obs) > 0 {
		return observation.StateContributed
	}
	return observation.StateEmpty
}

// wanted reports whether p should run for this request.
func wanted(req observation.Request, p observation.Provider) bool {
	if len(req.Sources) == 0 {
		return true
	}
	for _, s := range p.Sources() {
		if req.Wants(s) {
			return true
		}
	}
	return false
}

// primarySource is the source to blame when a provider fails without naming one.
func primarySource(p observation.Provider) directorapi.ObservationSource {
	if s := p.Sources(); len(s) > 0 {
		return s[0]
	}
	return ""
}

// merge folds one provider's environment into the cycle's. First reporter wins, in
// provider order — the same strongest-first rule fusion applies to observations.
func merge(dst *observation.Environment, src observation.Environment) {
	if len(dst.Monitors) == 0 {
		dst.Monitors = src.Monitors
	}
	if dst.Cursor.Position == (directorapi.Point{}) && dst.Cursor.Over == nil {
		dst.Cursor = src.Cursor
	}
	if dst.Selection == nil {
		dst.Selection = src.Selection
	}
	if dst.Clipboard == nil {
		dst.Clipboard = src.Clipboard
	}
}

// shadowReason folds a shadow provider's call error into its outcome reason.
//
// Shadow failures never become cycle Failures, so the reason field is the only place the
// error survives. Losing it would leave a diagnostic unable to tell an experiment that found
// nothing from one that could not run — the same distinction the collector exists to keep for
// authoritative providers, owed equally to an experiment whose whole purpose is measurement.
func shadowReason(reason string, err error) string {
	if err == nil {
		return reason
	}
	if reason == "" {
		return err.Error()
	}
	return reason + "; " + err.Error()
}
