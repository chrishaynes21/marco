package main

import (
	"fmt"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
)

// Traces reports the recent command phase traces.
func (r *Runtime) Traces(n int) []*trace.Trace {
	if r.traces == nil {
		return nil
	}
	return r.traces.Recent(n)
}

// TraceFor returns one command's trace.
func (r *Runtime) TraceFor(id string) *trace.Trace {
	if r.traces == nil {
		return nil
	}
	if id == "" || id == "last" {
		t, _ := r.traces.Last()
		return t
	}
	t, _ := r.traces.Get(id)
	return t
}

// beginTrace starts a trace for one command and attaches it to the pipeline.
//
// Attached under the same lock that serialises Handle, so the pipeline never has two
// traces at once. The trace is filed into history IMMEDIATELY rather than on
// completion — a command that is still running is exactly the one a status query
// wants to see, and a history that only holds finished commands cannot answer "what is
// it doing right now?".
func (r *Runtime) beginTrace(phrase string) *trace.Trace {
	return r.beginTraceLinked(phrase, "")
}

// beginTraceLinked starts a trace that CONTINUES another command.
//
// A clarification answer is a separate request over the wire — its own command id, its
// own connection — so it gets its own trace. Linking it back is what stops the pair
// reading as two unrelated commands: without the link, a reader sees a program that
// paused and, separately, a command that clicked something, with nothing joining them.
func (r *Runtime) beginTraceLinked(phrase, parent string) *trace.Trace {
	t := trace.New(nextTraceID(), phrase)
	t.ParentCommand = parent
	r.traces.Add(t)
	r.pipeline.Trace = t
	return t
}

// lastTraceID is the trace a resumed command continues.
func (r *Runtime) lastTraceID() string {
	if r.traces == nil {
		return ""
	}
	if t, ok := r.traces.Last(); ok {
		return t.CommandID
	}
	return ""
}

// finishTrace closes the trace and detaches it.
func (r *Runtime) finishTrace(t *trace.Trace, terminal string) {
	t.Finish(terminal)
	r.pipeline.Trace = nil
}

// traceSeq numbers traces within a service lifetime.
//
// Guarded because Handle and RunOperation both begin traces, and the registry permits
// only one MUTATING command at a time — but a diagnostic path could still race it.
var (
	traceMu  sync.Mutex
	traceSeq int
)

func nextTraceID() string {
	traceMu.Lock()
	defer traceMu.Unlock()
	traceSeq++
	return fmt.Sprintf("trace_%d", traceSeq)
}
