// Package recordhost is a Marco host that writes down what it was asked to do and does
// nothing.
//
// # What it is for
//
// The last thing between a compiled Marco program and a real computer is a runtime.Host. The
// Windows OS host presses keys; this one appends a line. Everything above it — the lowering, the
// encoder, the compiler, the runtime, the frame scheduler — is the SAME code, because the only
// thing that changed is which Host was installed.
//
// That is the whole design. A "dry run" that took a shortcut past the compiler would prove that
// the shortcut works. This proves that the real path works and that its final step was replaced
// with a notebook.
//
// # Why it is not runtime.DryRunHost
//
// The runtime's own dry host prints to the program's log and returns ok, which is right for
// `marco run --host dryrun` and useless for a test that must assert on the ORDER and EXACT
// content of what would have been sent. This one keeps a structured, ordered, bounded record and
// hands it back in Go.
package recordhost

import (
	"fmt"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// MaxCalls bounds the record.
//
// A host that grew without limit would be a memory leak reachable from a program, and the
// programs this exists for are one step long. Past the bound calls are counted and dropped, and
// Overflowed says so — silence would let a truncated record read as a complete one.
const MaxCalls = 256

// Call is one foreign invocation, as it arrived.
//
// The payload is kept as the runtime's own rendering rather than re-encoded here: the question a
// test asks is "what would the host have been given", and any re-encoding of the value would be
// this package's answer to that question instead of the runtime's.
type Call struct {
	// Act and Action are the capability, e.g. "OS" and "Navigate".
	Act, Action string
	// Input is the `with …` payload as the runtime renders it, empty when absent.
	Input string
}

// String is the form a person reads: `OS's Navigate with "down"`.
func (c Call) String() string {
	if c.Input == "" {
		return c.Act + "'s " + c.Action
	}
	return c.Act + "'s " + c.Action + " with " + c.Input
}

// Host records calls and performs none.
//
// Safe for concurrent use because the runtime's scheduler may run frames in parallel — see the
// multi-active scheduler work; a host that assumed one goroutine would be a data race waiting for
// a program with two active frames.
type Host struct {
	mu        sync.Mutex
	calls     []Call
	dropped   int
	overflowd bool
}

// New returns an empty recorder.
func New() *Host { return &Host{} }

// Invoke records the call and reports success without doing anything.
//
// Reporting `ok` is deliberate. A recorder that failed every call would make every program take
// its failure branch, and the thing under test — what the successful path would have emitted —
// would never be reached.
func (h *Host) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	rec := Call{Act: c.Act, Action: c.Action}
	if !c.Input.IsAbsent() {
		rec.Input = c.Input.String()
	}
	h.mu.Lock()
	if len(h.calls) < MaxCalls {
		h.calls = append(h.calls, rec)
	} else {
		h.dropped++
		h.overflowd = true
	}
	h.mu.Unlock()
	return "ok", runtime.Absent(), nil
}

// Calls returns what was asked for, in order.
func (h *Host) Calls() []Call {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]Call{}, h.calls...)
}

// Lines returns the readable form of each call, in order.
func (h *Host) Lines() []string {
	out := []string{}
	for _, c := range h.Calls() {
		out = append(out, c.String())
	}
	return out
}

// Overflowed reports that the bound was reached and how many calls were dropped.
func (h *Host) Overflowed() (int, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.dropped, h.overflowd
}

// Reset empties the record.
func (h *Host) Reset() {
	h.mu.Lock()
	h.calls, h.dropped, h.overflowd = nil, 0, false
	h.mu.Unlock()
}

// Describe renders the record for a person.
func (h *Host) Describe() []string {
	calls := h.Calls()
	if len(calls) == 0 {
		return []string{"nothing would be sent"}
	}
	out := make([]string, 0, len(calls)+1)
	out = append(out, fmt.Sprintf("%d call(s) would be sent, in this order:", len(calls)))
	for i, c := range calls {
		out = append(out, fmt.Sprintf("  %d. %s", i+1, c))
	}
	return out
}
