package execute

import (
	"context"

	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Phase instrumentation for the pipeline.
//
//	Slow is diagnosable. Stuck is bounded.
//
// Each helper wraps the code that PERFORMS a phase, so there is no window in which
// work is happening and untracked, and no reconstruction of durations from log order
// afterwards. A trace built that way cannot see a phase that never finished — which is
// the only case actually worth seeing.
//
// Every helper is a no-op when no trace is wired, so the pipeline behaves exactly as
// before without one.

// stepMeta carries the current step's position into a phase record, so a trace of a
// program says which step was slow rather than only that something was.
func (p *Pipeline) stepMeta(phase trace.CommandPhase) trace.Metadata {
	m := p.Deadlines.StepMeta(phase, p.stepIndex, p.stepCount, p.stepID)
	return m
}

// observeTraced observes with timing and a deadline.
//
// An observation that times out reports the world as UNAVAILABLE, never as empty. An
// empty world would be read downstream as "the target is not there", which is a claim
// about the application rather than about the Director — exactly the confusion
// ResolutionUnobservable exists to prevent.
func (p *Pipeline) observeTraced(ctx context.Context) (directorapi.WorldState, error) {
	var w directorapi.WorldState
	err := trace.Do(ctx, p.Trace, trace.PhaseObserve, p.stepMeta(trace.PhaseObserve),
		func(ctx context.Context) error {
			var err error
			w, err = p.Observe(ctx)
			return err
		})
	return w, err
}

// markPhase records an instantaneous transition.
func (p *Pipeline) markPhase(phase trace.CommandPhase, reason string) {
	trace.Mark(p.Trace, phase, p.stepMeta(phase), reason)
}

// emitValue records one data-flow event, filling in where in the program it happened.
//
// The step position is added HERE rather than at each call site, so an emit cannot
// forget it and so the six emit points stay about the data-flow fact they are
// reporting. The event type has no field that can hold content — see trace.ValueEvent —
// which is what makes this safe to call from anywhere without a second thought.
func (p *Pipeline) emitValue(e trace.ValueEvent) {
	e.StepIndex, e.StepCount, e.StepID = p.stepIndex, p.stepCount, p.stepID
	// Only filled in when the caller did not already say. The cleanup event knows its
	// program directly — it runs after the last step, when the pipeline's step fields
	// no longer describe anything — and overwriting it from the trace would blank it.
	if e.ProgramID == "" && p.Trace != nil {
		e.ProgramID = p.Trace.ProgramID
	}
	p.Trace.Emit(e)
}
