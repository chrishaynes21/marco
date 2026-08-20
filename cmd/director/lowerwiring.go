package main

import (
	"context"
	"errors"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// loweringHistory keeps the recent lowered operations for `director lower` and the
// execution trace.
//
// Bounded, and diagnostics only — nothing plans from it. Source stored here is already
// redacted by marcoexec.Redact before it arrives, so a credential name cannot reach
// this slice even if the recorder is later reused somewhere less careful.
type loweringHistory struct {
	mu      sync.Mutex
	entries []marcoexec.Result
}

// loweringLimit is how many results are kept. The generated source is held, so this is
// also the bound on how long a typed value lingers in memory.
const loweringLimit = 50

func (h *loweringHistory) record(r marcoexec.Result) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = append(h.entries, r)
	if len(h.entries) > loweringLimit {
		h.entries = h.entries[len(h.entries)-loweringLimit:]
	}
}

func (h *loweringHistory) recent() []marcoexec.Result {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]marcoexec.Result, len(h.entries))
	copy(out, h.entries)
	return out
}

// recordLowering files one lowered operation.
func (r *Runtime) recordLowering(res marcoexec.Result) {
	if r.lowerings == nil {
		return
	}
	r.lowerings.record(res)
}

// Lowerings reports the recent lowered operations, source included.
func (r *Runtime) Lowerings() []marcoexec.Result {
	if r.lowerings == nil {
		return nil
	}
	return r.lowerings.recent()
}

// activateWindow returns a function that brings a window's application to the front,
// used by the foreground guard's one repair attempt.
//
// It goes through the executor, so the repair is itself a compiled Marco program —
// there is no privileged side door for "just activate it quickly". The window is
// mapped to its application first because OS's Activate names an APP, which is the
// vocabulary Marco actually has.
func activateApp(e *marcoexec.Executor) func(context.Context, string) error {
	return func(ctx context.Context, app string) error { return e.Activate(ctx, app) }
}

// observeForGuard is the guard's window onto the world.
//
// A method rather than the observe closure directly, because the closure is built
// later in NewRuntime than the guard is attached. Indirecting through the Runtime lets
// the guard be wired first and still see the real pipeline — and it stays the SAME
// pipeline, so the guard checks the world the planner reasoned over.
func (r *Runtime) observeForGuard(ctx context.Context) (directorapi.WorldState, error) {
	if r.pipeline == nil || r.pipeline.Observe == nil {
		return directorapi.WorldState{}, errNoPipeline
	}
	return r.pipeline.Observe(ctx)
}

// errNoPipeline is returned before the pipeline exists. The guard treats an error as
// "cannot confirm", which refuses — the safe direction.
var errNoPipeline = errors.New("the observation pipeline is not built yet")

// Last returns the most recently lowered operation.
//
// Safe as "the one this action caused" because the pipeline runs a single action at a
// time under a lock, so nothing else can have lowered anything in between.
func (h *loweringHistory) Last() (marcoexec.Result, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.entries) == 0 {
		return marcoexec.Result{}, false
	}
	return h.entries[len(h.entries)-1], true
}

// RunOperation executes one operation through the ordinary path.
//
// Serialised with the same lock as Handle: the executor's foreground guard observes,
// and the pipeline holds a stateful world builder whose element identity depends on
// snapshots arriving in order.
func (r *Runtime) RunOperation(ctx context.Context, op marcoexec.Operation) marcoexec.Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	res, _ := r.effects.Do(ctx, op)
	return res
}
