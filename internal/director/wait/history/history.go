// Package history records what a wait saw while it waited.
//
// Bounded to ONE wait and never persisted. A wait's evaluations describe a screen that
// existed for a few hundred milliseconds; keeping them past the wait would be keeping
// an account of a world that no longer exists, and putting them in the action graph
// would make a durable record of something transient.
//
// What survives a wait is its RESULT — satisfied, timed out, cancelled — and the one
// evaluation that settled it. That is what an explanation needs.
package history

import (
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/wait/evaluation"
)

// Recorder collects the evaluations of one active wait.
//
// Safe for concurrent use: a wait runs on the command goroutine while `director wait`
// may ask what it is doing from a connection goroutine.
type Recorder struct {
	mu    sync.RWMutex
	items []evaluation.Evaluation
	// limit bounds a single wait's history. A wait polling every 100ms for its full
	// 30-second bound is 300 evaluations; a bound of a few hundred keeps that whole
	// while refusing to grow without limit if someone raises the timeout.
	limit int
}

// DefaultLimit is how many evaluations one wait retains.
const DefaultLimit = 500

// New returns a recorder.
func New(limit int) *Recorder {
	if limit <= 0 {
		limit = DefaultLimit
	}
	return &Recorder{limit: limit}
}

// Record adds one evaluation.
func (r *Recorder) Record(e evaluation.Evaluation) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, e)
	if len(r.items) > r.limit {
		// Drop from the FRONT. The recent evaluations are the ones that explain how a
		// wait ended; the early ones only say it had not finished yet.
		n := copy(r.items, r.items[len(r.items)-r.limit:])
		r.items = r.items[:n]
	}
}

// All returns the evaluations in order.
func (r *Recorder) All() []evaluation.Evaluation {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]evaluation.Evaluation(nil), r.items...)
}

// Latest returns the most recent evaluation.
func (r *Recorder) Latest() (evaluation.Evaluation, bool) {
	if r == nil {
		return evaluation.Evaluation{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.items) == 0 {
		return evaluation.Evaluation{}, false
	}
	return r.items[len(r.items)-1], true
}

// Len is how many evaluations were made.
func (r *Recorder) Len() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}

// Summary counts the states seen, so a timeout can say what it kept seeing.
//
// "It timed out while UNKNOWN" and "it timed out while UNSATISFIED" are different
// diagnoses: the first means the Director could not see the thing it was waiting for,
// which is a perception problem, and the second means it saw and the thing never
// happened, which is not.
func (r *Recorder) Summary() map[evaluation.State]int {
	out := map[evaluation.State]int{}
	for _, e := range r.All() {
		out[e.Result.State]++
	}
	return out
}
