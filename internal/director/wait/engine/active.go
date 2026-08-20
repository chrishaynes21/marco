package engine

import (
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/wait/conditions"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/evaluation"
)

// The wait in flight, for `director wait`.
//
// A wait can run for tens of seconds, and during it the Director looks busy and says
// nothing. That is exactly when a user wants to know WHAT it is waiting for and whether
// it is making progress — "waiting for the dialog, 14 observations, still unsatisfied"
// is the difference between a considered pause and an apparent hang.
//
// Not history: this is what is happening NOW. When the wait ends it is cleared, because
// the durable answer is the wait's Result and keeping a finished wait here would make
// `director wait` report on something that is no longer running.

// Snapshot is what one active wait looks like from outside.
type Snapshot struct {
	Waiting     bool                   `json:"waiting"`
	Condition   conditions.ID          `json:"condition,omitempty"`
	Description string                 `json:"description,omitempty"`
	StartedAt   time.Time              `json:"started_at,omitempty"`
	Elapsed     time.Duration          `json:"elapsed,omitempty"`
	Timeout     time.Duration          `json:"timeout,omitempty"`
	Iterations  int                    `json:"iterations"`
	Latest      *evaluation.Evaluation `json:"latest,omitempty"`
	// Counts is how many evaluations landed in each state, which is what makes a stuck
	// wait diagnosable: all-unknown is a perception problem, all-unsatisfied is not.
	Counts map[evaluation.State]int `json:"counts,omitempty"`
}

// Active tracks the wait currently running.
type Active struct {
	mu sync.RWMutex

	waiting     bool
	condition   conditions.ID
	description string
	startedAt   time.Time
	timeout     time.Duration
	iterations  int
	latest      *evaluation.Evaluation
	counts      map[evaluation.State]int
}

// NewActive returns an empty tracker.
func NewActive() *Active { return &Active{counts: map[evaluation.State]int{}} }

func (a *Active) begin(c conditions.Condition, opts Options, at time.Time) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.waiting, a.condition, a.description = true, c.ID(), c.Description()
	a.startedAt, a.timeout = at, opts.Timeout
	a.iterations, a.latest = 0, nil
	a.counts = map[evaluation.State]int{}
}

func (a *Active) record(e evaluation.Evaluation) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.iterations++
	copied := e
	a.latest = &copied
	a.counts[e.Result.State]++
}

func (a *Active) end() {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.waiting = false
}

// Snapshot reports the wait in flight.
func (a *Active) Snapshot() Snapshot {
	if a == nil {
		return Snapshot{}
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.waiting {
		return Snapshot{}
	}
	out := Snapshot{
		Waiting: true, Condition: a.condition, Description: a.description,
		StartedAt: a.startedAt, Elapsed: time.Since(a.startedAt),
		Timeout: a.timeout, Iterations: a.iterations,
		Counts: map[evaluation.State]int{},
	}
	for k, v := range a.counts {
		out.Counts[k] = v
	}
	if a.latest != nil {
		copied := *a.latest
		out.Latest = &copied
	}
	return out
}
