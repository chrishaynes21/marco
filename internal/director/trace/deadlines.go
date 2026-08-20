package trace

import (
	"sync"
	"time"
)

// Deadlines bound the phases that can block on something outside the Director:
// a subprocess, the accessibility tree, the screen, the Marco runtime.
//
// Every value here is PROVISIONAL. They were chosen from measured behaviour on one
// machine — observation runs at ~0.15s, a two-step program completes in ~1.5s, a
// semantic settle satisfies in ~0.5s — and set an order of magnitude above that, so
// they catch a stall without failing a slow-but-working machine. They are not
// tuned, and a value that starts firing in normal use is evidence about the system
// rather than a number to raise reflexively.
//
// Deliberately not uniform. An observation that takes fifteen seconds is broken; a
// Marco runtime execution that takes fifteen seconds may be a legitimately slow
// application. Giving them the same bound would either fail the second or fail to
// catch the first.
type Deadlines struct {
	Observe           time.Duration
	Resolve           time.Duration
	Policy            time.Duration
	Lower             time.Duration
	Compile           time.Duration
	RuntimeExecute    time.Duration
	Settle            time.Duration
	Verify            time.Duration
	ClarificationEmit time.Duration
	ResponseWrite     time.Duration
}

// DefaultDeadlines are the provisional production bounds.
func DefaultDeadlines() Deadlines {
	return Deadlines{
		// An accessibility walk of a large application (Chrome at ~2200 elements) is
		// the worst measured case and lands well under a second. Fifteen means the
		// bridge is wedged, not that the tree is big.
		Observe: 15 * time.Second,
		// Resolution is pure computation over an already-observed world. It cannot
		// legitimately take seconds.
		Resolve: 5 * time.Second,
		Policy:  5 * time.Second,
		// Lowering is string building; compiling is the Marco front end over a program
		// of a dozen lines. Both are milliseconds in practice.
		Lower:   5 * time.Second,
		Compile: 10 * time.Second,
		// The one that must be generous. This is a real application receiving real
		// input: a slow Electron app, a remote desktop, a machine under load. Thirty
		// seconds is long enough that firing means something is genuinely stuck.
		RuntimeExecute: 30 * time.Second,
		// Settling already stops as soon as its condition holds; this is the ceiling
		// for the case where it never does. Above the wait engine's own 30s default so
		// the inner bound reports first and names the condition.
		Settle: 35 * time.Second,
		Verify: 15 * time.Second,
		// Control-plane work. A client that cannot take its event in five seconds has
		// gone away, and the command must not be held up by it.
		ClarificationEmit: 5 * time.Second,
		ResponseWrite:     10 * time.Second,
	}
}

// For returns the deadline for a phase, zero when the phase is unbounded.
//
// Unbounded is correct for the phases that are pure bookkeeping — routing a phrase,
// validating a program, marking a pause. They cannot block on anything, and giving
// them a deadline would add a timer to every command for no possible benefit.
func (d Deadlines) For(p CommandPhase) time.Duration {
	switch p {
	case PhaseObserve:
		return d.Observe
	case PhaseResolve:
		return d.Resolve
	case PhasePolicy:
		return d.Policy
	case PhaseLower:
		return d.Lower
	case PhaseCompile:
		return d.Compile
	case PhaseRuntimeExecute:
		return d.RuntimeExecute
	case PhaseSettle:
		return d.Settle
	case PhaseVerify:
		return d.Verify
	case PhaseClarificationEmit:
		return d.ClarificationEmit
	case PhaseResponseWrite:
		return d.ResponseWrite
	}
	return 0
}

// Meta builds the metadata for a phase, with its deadline filled in.
func (d Deadlines) Meta(p CommandPhase) Metadata {
	return Metadata{Deadline: d.For(p)}
}

// StepMeta is Meta with a step's position attached.
func (d Deadlines) StepMeta(p CommandPhase, index, count int, stepID string) Metadata {
	return Metadata{
		Deadline: d.For(p), StepIndex: index, StepCount: count, StepID: stepID,
	}
}

// History holds the recent command traces.
//
// Bounded, and holding only timings and states — no observation graphs, no screen
// text, no payloads. That is what keeps tracing cheap enough to leave on by default,
// which it has to be: instrumentation you have to turn on is never on when the thing
// you needed it for happens.
type History struct {
	mu     sync.Mutex
	traces []*Trace
	limit  int
	byID   map[string]*Trace
}

// DefaultHistoryLimit is how many command traces are kept.
const DefaultHistoryLimit = 25

// NewHistory returns a bounded history.
func NewHistory() *History {
	return &History{limit: DefaultHistoryLimit, byID: map[string]*Trace{}}
}

// Add files a trace, evicting the oldest when full.
func (h *History) Add(t *Trace) {
	if h == nil || t == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.traces = append(h.traces, t)
	h.byID[t.CommandID] = t
	for len(h.traces) > h.limit {
		delete(h.byID, h.traces[0].CommandID)
		h.traces = h.traces[1:]
	}
}

// Last returns the most recent trace.
func (h *History) Last() (*Trace, bool) {
	if h == nil {
		return nil, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.traces) == 0 {
		return nil, false
	}
	return h.traces[len(h.traces)-1], true
}

// Get returns a trace by command id.
func (h *History) Get(id string) (*Trace, bool) {
	if h == nil {
		return nil, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	t, ok := h.byID[id]
	return t, ok
}

// Recent returns up to n traces, newest last.
func (h *History) Recent(n int) []*Trace {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if n <= 0 || n > len(h.traces) {
		n = len(h.traces)
	}
	out := make([]*Trace, n)
	copy(out, h.traces[len(h.traces)-n:])
	return out
}
