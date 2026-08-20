package observation

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// CycleID identifies one round of observation.
type CycleID string

// Cycle is everything the providers reported for one moment.
//
// Observations are EPHEMERAL and the World State is persistent. A cycle is the unit of
// that ephemerality: evidence belongs to exactly one cycle, and when the cycle is
// forgotten the evidence goes with it. What survives is the belief fusion built from
// it, plus the provenance references that name what the evidence WAS.
//
// Keeping observations indefinitely would be the easy mistake and a bad one. They are
// large — a warm VS Code cycle is well over two thousand of them — and they describe a
// desktop that has since moved on. A stale observation is not weaker evidence; it is
// evidence about a different screen.
type Cycle struct {
	ID          CycleID
	StartedAt   time.Time
	CompletedAt time.Time

	// Request is what was asked for. Recorded so a diagnostic can distinguish "no OCR
	// evidence because none was requested" from "none because it failed".
	Request Request

	Observations []Observation

	// Outcomes is what each provider did, with the provenance it established.
	//
	// The complete record, INCLUDING providers whose evidence was refused — a cycle that
	// dropped a refused provider would be indistinguishable from one where it never ran,
	// and "its evidence described a window that is no longer current" is the single most
	// useful sentence a person debugging an empty world can read.
	//
	// Observations above stays the full evidence record for the same reason. What may be
	// BELIEVED is a narrower question, and Admitted is where it is answered.
	Outcomes []ProviderOutcome

	// Shadow is what experimental providers reported. Recorded, never believed.
	//
	// A SEPARATE FIELD, and that is the entire safety argument. Admitted() reads Outcomes
	// and Observations; it does not read this, and it cannot be made to by any value a
	// shadow provider returns. There is no flag to check, no state to get wrong, and no way
	// for a future edit to fusion to start believing an experiment — the evidence is simply
	// not in the collection fusion is handed.
	//
	// Outcomes carry full provenance all the same. An experiment that cannot say which
	// window generation it looked at cannot honestly be compared with belief.
	Shadow []ProviderOutcome

	// Failures are the providers that were expected and did not deliver. A cycle with
	// failures is still usable; it just cannot be read as "the Save button does not
	// exist". Fusion carries these into the world's Degraded list, which is what keeps
	// "I could not see" from being mistaken for "it is not there".
	Failures []directorapi.SourceFailure

	// Environment is the frame the evidence sits in rather than evidence about
	// objects: where the pointer is, what the screens are, what is selected. Kept out
	// of Observations because none of it is a claim any source could be wrong about in
	// the way a misread label is wrong, and modelling it as evidence would give fusion
	// four more kinds to reason about for no gain.
	Environment Environment
}

// Environment is the ambient state of one observation cycle.
type Environment struct {
	Monitors  []directorapi.Monitor
	Cursor    directorapi.CursorState
	Selection *directorapi.SelectionState
	Clipboard *directorapi.ClipboardState
}

// Duration is how long the cycle took.
func (c Cycle) Duration() time.Duration {
	if c.CompletedAt.IsZero() || c.StartedAt.IsZero() {
		return 0
	}
	return c.CompletedAt.Sub(c.StartedAt)
}

// Timestamp is when observation began. Fusion stamps the world with this rather than
// with the completion time: a snapshot is only as fresh as its OLDEST evidence, and
// claiming otherwise would let a slow provider make a stale world look current.
func (c Cycle) Timestamp() time.Time {
	if c.StartedAt.IsZero() {
		return c.CompletedAt
	}
	return c.StartedAt
}

// Targeted reports whether this cycle pinned a live window generation.
//
// An ordinary command cycle does not. It looks at whatever is in front, has no generation
// to be stale relative to, and the guard below correctly does not apply to it.
func (c Cycle) Targeted() bool {
	return c.Request.Target != nil && c.Request.Target.Known()
}

// Admitted separates evidence that may become belief from evidence that may not.
//
// # The rule
//
// On an UNTARGETED cycle everything is admitted. There is no pinned generation, so there is
// no claim to be stale about, and applying a guard would reject every ordinary command.
//
// On a TARGETED cycle only observations belonging to a usable outcome are admitted. An
// observation reaches belief by being carried by a provider that proved what it observed —
// there is no other route, which is what makes the guard total rather than advisory.
//
// # Why a targeted cycle with no outcomes admits NOTHING
//
// It reads like an edge case and it is the fail-safe. The alternative — falling back to the
// flat Observations list when outcomes are absent — would mean any code path that pinned a
// target but did not collect outcomes silently bypassed the guard, and it would do so
// quietly, on the exact path where staleness is possible. A cycle that pins a target and
// cannot say which provider proved what has established nothing, and is treated that way.
//
// The refused outcomes are returned rather than discarded so a caller can degrade the world
// with them. Refused evidence that vanished without trace would turn "I could not attribute
// what I saw" into "there was nothing there", which is the confusion this whole subsystem
// exists to prevent.
func (c Cycle) Admitted() (admitted []Observation, refused []ProviderOutcome) {
	if !c.Targeted() {
		return c.Observations, nil
	}
	for _, o := range c.Outcomes {
		if o.Usable() {
			admitted = append(admitted, o.Observations...)
			continue
		}
		// Only outcomes that HELD evidence are reported as refused. A provider that was
		// unavailable or found nothing had nothing to lose, and listing it here would
		// bury the one case that matters in routine noise.
		if len(o.Observations) > 0 {
			refused = append(refused, o)
		}
	}
	return admitted, refused
}

// cycleSeq numbers cycles within this process.
var cycleSeq atomic.Int64

// NewCycleID mints an id. Monotonic within a process and prefixed with the start
// time, so cycles from one run sort in order and two runs' ids do not collide in a log.
func NewCycleID(at time.Time) CycleID {
	return CycleID(fmt.Sprintf("cyc_%d_%d", at.UnixMilli(), cycleSeq.Add(1)))
}

// ── bounded history ───────────────────────────────────────────────────────────

// DefaultHistory is how many recent cycles are retained for diagnostics.
//
// Five. Enough to see what changed across a click and its verification — observe,
// act, observe — and few enough that the memory cost is a rounding error against one
// live accessibility tree. The bound is the point: an unbounded history of a
// per-command observation is a leak that grows with uptime, and this service is meant
// to run for days.
const DefaultHistory = 5

// History retains the most recent cycles for diagnostics, and nothing else.
//
// Safe for concurrent use: the service observes on its command goroutine and answers
// diagnostics on connection goroutines.
type History struct {
	mu     sync.RWMutex
	limit  int
	cycles []Cycle
	// total counts every cycle ever recorded, including those dropped. Without it a
	// diagnostic showing five cycles cannot tell a freshly started service from one
	// that has been observing all day.
	total int
}

// NewHistory returns a history retaining at most limit cycles.
func NewHistory(limit int) *History {
	if limit <= 0 {
		limit = DefaultHistory
	}
	return &History{limit: limit}
}

// Record adds a cycle, dropping the oldest beyond the limit.
func (h *History) Record(c Cycle) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.total++
	h.cycles = append(h.cycles, c)
	if len(h.cycles) > h.limit {
		// Copy down rather than reslicing: reslicing keeps the dropped cycle's
		// observations reachable from the backing array, which is precisely the leak
		// the bound exists to prevent.
		n := copy(h.cycles, h.cycles[len(h.cycles)-h.limit:])
		for i := n; i < len(h.cycles); i++ {
			h.cycles[i] = Cycle{}
		}
		h.cycles = h.cycles[:n]
	}
}

// Recent returns the retained cycles, newest first.
func (h *History) Recent() []Cycle {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Cycle, 0, len(h.cycles))
	for i := len(h.cycles) - 1; i >= 0; i-- {
		out = append(out, h.cycles[i])
	}
	return out
}

// Latest returns the most recent cycle.
func (h *History) Latest() (Cycle, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.cycles) == 0 {
		return Cycle{}, false
	}
	return h.cycles[len(h.cycles)-1], true
}

// Total is how many cycles have been recorded since start, including dropped ones.
func (h *History) Total() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.total
}

// Limit is the retention bound.
func (h *History) Limit() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.limit
}
