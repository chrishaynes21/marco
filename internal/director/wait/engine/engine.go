// Package engine runs the observe → evaluate → wait loop.
//
// The loop, and the reason each step is where it is:
//
//	Observe    — gather fresh evidence. A wait exists to do this; without it the
//	             loop would be a sleep with extra steps.
//	Evaluate   — answer the condition from that evidence.
//	Satisfied? — stop. Not "probably", not "it has been long enough".
//	Wait       — pause, then observe again. The pause is a POLL INTERVAL, not a
//	             prediction: it says how often to look, never how long the thing takes.
//
// The one duration in this package is the interval between looks, and it is not
// evidence of anything. Nothing here concludes that a condition holds because time
// passed.
package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/wait/conditions"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/evaluation"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/history"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Observer produces a fresh world. The same shape the execution pipeline uses, so a
// wait observes through exactly the path a command does — providers, fusion, identity.
type Observer func(ctx context.Context) (directorapi.WorldState, error)

// Options bound a wait.
type Options struct {
	// Timeout is the hard bound. There is no unbounded wait: a condition that never
	// becomes true would otherwise hang the Director forever, and a user who asked it
	// to wait for a dialog that will never appear deserves an answer.
	Timeout time.Duration

	// PollInterval is how often to look. NOT a prediction about how long anything
	// takes — the loop's answer comes from the observation, never from the interval
	// having elapsed.
	PollInterval time.Duration

	// StableObservations is how many CONSECUTIVE satisfied evaluations are required.
	//
	// One is not enough for anything that can flicker. An animation is momentarily
	// identical between frames; a page part-way through loading is quiet between
	// paints. Requiring a run is the difference between "it has stopped" and "it
	// happened not to move while I looked".
	StableObservations int

	// CancelOnUnknown stops the wait the first time the condition cannot be answered.
	//
	// Off by default, because Unknown is usually transient — an application still
	// starting up, a tree not yet published. It is worth turning on when the caller
	// would rather fail fast than poll a window it will never be able to read.
	CancelOnUnknown bool
}

// Defaults are conservative on purpose.
//
// The timeout is long enough for a slow page and short enough that a wrong condition
// does not strand a user; the interval is short enough to feel responsive and long
// enough that polling does not become the load it is waiting on.
func Defaults() Options {
	return Options{
		Timeout:            30 * time.Second,
		PollInterval:       120 * time.Millisecond,
		StableObservations: 2,
	}
}

// withDefaults fills the zero fields.
func (o Options) withDefaults() Options {
	d := Defaults()
	if o.Timeout <= 0 {
		o.Timeout = d.Timeout
	}
	if o.PollInterval <= 0 {
		o.PollInterval = d.PollInterval
	}
	if o.StableObservations <= 0 {
		o.StableObservations = d.StableObservations
	}
	return o
}

// Result is what a wait concluded.
type Result struct {
	Condition   conditions.ID `json:"condition"`
	Description string        `json:"description"`

	Final evaluation.Result `json:"final"`
	// Iterations is how many times the condition was evaluated.
	Iterations int `json:"iterations"`
	// ObservationCycles is how many fresh worlds were gathered. Usually equal to
	// Iterations; lower when observation failed and the previous world was reused.
	ObservationCycles int           `json:"observation_cycles"`
	Duration          time.Duration `json:"duration"`

	History []evaluation.Evaluation `json:"history,omitempty"`
}

// Satisfied is the headline answer.
func (r Result) Satisfied() bool { return r.Final.State == evaluation.Satisfied }

// Summary is one line for a person.
func (r Result) Summary() string {
	return fmt.Sprintf("%s after %d observation(s) in %s: %s",
		r.Final.State, r.ObservationCycles, r.Duration.Round(time.Millisecond),
		r.Final.Explanation)
}

// Engine waits for conditions.
type Engine interface {
	Wait(ctx context.Context, condition conditions.Condition, opts Options) (Result, error)
}

// Waiter is the default Engine. Exported because the composition root holds one to
// expose the wait in flight through `director wait` — an interface value could not.
type Waiter struct {
	observe Observer
	// stop is the Director's own cancellation signal, polled alongside the context.
	// Both exist because they arrive by different routes: a context is cancelled
	// in-process, while a spoken "stop" reaches a different process entirely.
	stop func() bool
	// now is injectable so tests are deterministic.
	now func() time.Time
	// active is the wait currently running, for `director wait`.
	active *Active
}

// New builds an engine over an observer.
func New(observe Observer) *Waiter {
	return &Waiter{observe: observe, now: time.Now, active: NewActive()}
}

// WithStopCheck adds the Director's cancellation poll.
func (e *Waiter) WithStopCheck(stop func() bool) *Waiter {
	e.stop = stop
	return e
}

// Active exposes the wait in flight, for diagnostics.
func (e *Waiter) Active() *Active { return e.active }

var _ Engine = (*Waiter)(nil)

// Wait runs the loop until the condition is answered or the bound is reached.
func (e *Waiter) Wait(ctx context.Context, condition conditions.Condition, opts Options) (Result, error) {
	opts = opts.withDefaults()
	started := e.now()
	rec := history.New(0)

	out := Result{
		Condition:   condition.ID(),
		Description: condition.Description(),
	}
	e.active.begin(condition, opts, started)
	defer e.active.end()

	deadline := started.Add(opts.Timeout)
	consecutive := 0
	var world directorapi.WorldState
	var haveWorld bool

	for {
		// Cancellation BEFORE observing. Checked at every boundary rather than once at
		// the top, because a wait can be long and a user who has asked it to stop
		// should not have to sit through one more capture.
		if res, stopped := e.cancelled(ctx); stopped {
			return e.finish(out, rec, res, started), nil
		}
		if e.now().After(deadline) {
			return e.finish(out, rec, e.timedOut(rec, opts), started), nil
		}

		// ── Observe ───────────────────────────────────────────────────────────
		fresh, err := e.observe(ctx)
		if err == nil {
			world, haveWorld = fresh, true
			out.ObservationCycles++
		} else if !haveWorld {
			// Never managed to observe. UNKNOWN, and recorded as such: a wait that
			// could not look has learned nothing, and reporting that as "the condition
			// is not met" would be a claim about a screen it never saw.
			rec.Record(evaluation.Evaluation{
				Timestamp: e.now(),
				Result: evaluation.Unknowable(
					"could not observe the screen: " + err.Error()),
			})
			out.Iterations++
			if opts.CancelOnUnknown {
				return e.finish(out, rec, mustLatest(rec), started), nil
			}
			if !e.sleep(ctx, opts.PollInterval) {
				return e.finish(out, rec, cancelledResult(), started), nil
			}
			continue
		}

		// Cancellation AFTER observing. An observation can take hundreds of
		// milliseconds against a warm editor; a stop that arrived during it should not
		// be answered by evaluating and acting on the result.
		if res, stopped := e.cancelled(ctx); stopped {
			return e.finish(out, rec, res, started), nil
		}

		// ── Evaluate ──────────────────────────────────────────────────────────
		result := evaluate(condition, world)
		out.Iterations++
		ev := evaluation.Evaluation{Timestamp: e.now(), Result: result}
		rec.Record(ev)
		e.active.record(ev)

		switch result.State {
		case evaluation.Satisfied:
			consecutive++
			if consecutive >= opts.StableObservations {
				return e.finish(out, rec, result, started), nil
			}
		case evaluation.Unknown:
			// Unknown does NOT reset the run and does not count toward it. It is the
			// absence of an answer: a condition that was satisfied, then unobservable,
			// then satisfied has not been falsified — but nor has it been confirmed
			// twice.
			if opts.CancelOnUnknown {
				return e.finish(out, rec, result, started), nil
			}
		default:
			// Unsatisfied resets the run. This is what makes stability a RUN rather
			// than a tally: two quiet looks either side of a change are not stability.
			consecutive = 0
		}

		if e.now().After(deadline) {
			return e.finish(out, rec, e.timedOut(rec, opts), started), nil
		}
		// ── Wait, then look again ─────────────────────────────────────────────
		if !e.sleep(ctx, opts.PollInterval) {
			return e.finish(out, rec, cancelledResult(), started), nil
		}
	}
}

// evaluate answers a condition against a world.
//
// A condition that cannot be answered from a world at all — one that needs a capture
// and was given no sampler — reports Unknown rather than failing. That keeps "this
// Director has no capture layer" a degraded state rather than an error.
func evaluate(c conditions.Condition, w directorapi.WorldState) evaluation.Result {
	if wc, ok := c.(conditions.WorldCondition); ok {
		return wc.Evaluate(w)
	}
	return evaluation.Unknowable(
		"this condition cannot be evaluated against a world state alone")
}

// cancelled reports whether the wait should stop, from either signal.
func (e *Waiter) cancelled(ctx context.Context) (evaluation.Result, bool) {
	select {
	case <-ctx.Done():
		return cancelledResult(), true
	default:
	}
	if e.stop != nil && e.stop() {
		return cancelledResult(), true
	}
	return evaluation.Result{}, false
}

// cancelledResult is the answer for a stopped wait.
//
// CANCELLED, never TimedOut. Reporting a cancellation as a timeout would tell the user
// their interface was slow when in fact they asked the Director to stop — and would
// send whoever reads the log looking for a performance problem that does not exist.
func cancelledResult() evaluation.Result {
	return evaluation.Result{
		State:       evaluation.Cancelled,
		Explanation: "the wait was cancelled",
	}
}

// timedOut builds the timeout answer, carrying WHAT it kept seeing.
func (e *Waiter) timedOut(rec *history.Recorder, opts Options) evaluation.Result {
	last, ok := rec.Latest()
	base := fmt.Sprintf("the condition was not satisfied within %s", opts.Timeout)
	if !ok {
		return evaluation.Result{State: evaluation.TimedOut, Explanation: base}
	}

	// The last state is the diagnosis. Timing out while UNKNOWN means the Director
	// could not see the thing it was waiting for — a perception problem. Timing out
	// while UNSATISFIED means it saw, and the thing never happened. Collapsing them
	// would send someone debugging the wrong layer.
	summary := rec.Summary()
	detail := fmt.Sprintf("%s — the last %d evaluation(s) were %s: %s",
		base, summary[last.Result.State], last.Result.State, last.Result.Explanation)
	if summary[evaluation.Unknown] > 0 && last.Result.State == evaluation.Unknown {
		detail += " (this is blindness rather than a condition that never came true)"
	}
	return evaluation.Result{
		State:       evaluation.TimedOut,
		Confidence:  last.Result.Confidence,
		Evidence:    last.Result.Evidence,
		Explanation: detail,
	}
}

// sleep pauses between looks, returning false if cancelled.
//
// Cancellation is checked BEFORE and AFTER the pause, which is why this is a function
// rather than a bare time.After: the pause is the longest a wait is unresponsive, and
// it is where a stop most often arrives.
func (e *Waiter) sleep(ctx context.Context, d time.Duration) bool {
	if res, stopped := e.cancelled(ctx); stopped {
		_ = res
		return false
	}
	select {
	case <-time.After(d):
	case <-ctx.Done():
		return false
	}
	_, stopped := e.cancelled(ctx)
	return !stopped
}

func (e *Waiter) finish(out Result, rec *history.Recorder,
	final evaluation.Result, started time.Time) Result {

	out.Final = final
	out.Duration = e.now().Sub(started)
	out.History = rec.All()
	return out
}

func mustLatest(rec *history.Recorder) evaluation.Result {
	if e, ok := rec.Latest(); ok {
		return e.Result
	}
	return evaluation.Unknowable("nothing was evaluated")
}
