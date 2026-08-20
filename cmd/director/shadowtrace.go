package main

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
)

// A bounded, opt-in trace of what the shadow detector reported, slot by slot.
//
// # Why this exists
//
// Aggregates cannot answer "why does one visible button become four tracks". The counts say
// 26 button detections became 24 tracks; they cannot say whether that is because the menu was
// only visible for two inferences, because the boxes moved too far to associate, or because
// two neighbouring rows fought over one reference and both lost.
//
// Those are different bugs with different fixes, and choosing between them by reasoning about
// totals is how a project ends up accelerating a detector whose problem was arithmetic.
//
// # Why EVERY slot is recorded, not only the ones that produced evidence
//
// The first version wrote a line only for valid inferences, and that made the two questions it
// existed to separate unanswerable. A cadence measurement needs the slots where nothing ran —
// the difference between "the detector was asked six times during a fifteen-second menu" and
// "it was asked twice because it was still busy" IS the cadence question, and a trace of
// successes alone reports both as the same two lines.
//
// The same applies to failures and to unproven targets. A slot that could not prove which
// window it observed is not a slot that saw nothing, and a trace that omits it invites exactly
// the reading this project has already made twice: infrastructure silence read as model
// behaviour.
//
// # Why it is opt-in and bounded
//
// A per-slot trace is unbounded history, which is what normal tracking refuses to keep. It
// writes only when $MARCO_SHADOW_TRACE names a file, caps what it retains, and holds no
// labels, no pixels and no absolute coordinates — the regions it records are the same
// window-relative normalised geometry the tracker itself matches on.
type shadowTrace struct {
	mu   sync.Mutex
	path string
	n    int
}

// maxTracedSlots bounds a diagnostic run. A 60s session at the observation cadence is on the
// order of a hundred slots; this leaves room for a long one without becoming a log file.
const maxTracedSlots = 2000

var traceOnce sync.Once
var tracer *shadowTrace

// shadowTracer returns the trace sink, or nil when tracing was not requested.
func shadowTracer() *shadowTrace {
	traceOnce.Do(func() {
		if p := os.Getenv("MARCO_SHADOW_TRACE"); p != "" {
			tracer = &shadowTrace{path: p}
		}
	})
	return tracer
}

// Slot outcomes. A closed set, because "why was there no evidence" is the question the trace
// is for and free-form status text cannot be counted.
const (
	slotValid    = "valid"    // an inference ran and proved its target
	slotSkipped  = "skipped"  // the cadence gate declined the slot; NOT a failure
	slotFailed   = "failed"   // an inference was attempted and could not complete
	slotUnproven = "unproven" // it ran, but cannot show which window it describes
)

// tracedSlot is one shadow opportunity, whatever became of it.
type tracedSlot struct {
	N       int    `json:"n"`
	AtMS    int64  `json:"at_ms"`
	Outcome string `json:"outcome"`
	// Generation is the window lifecycle epoch the evidence belongs to. Geometry from two
	// generations is geometry about two different worlds, and comparing it would manufacture
	// movement out of a window that was replaced.
	Generation uint64 `json:"generation,omitempty"`
	LatencyMS  int64  `json:"latency_ms,omitempty"`
	Detections int    `json:"detections,omitempty"`
	// Reason is Marco's own diagnostic text for a slot that produced nothing.
	//
	// Marco-authored only. It must never be built from a window title, a detector label or
	// OCR output; the privacy rule does not relax because a field is called a reason.
	Reason  string                 `json:"reason,omitempty"`
	Regions []observe.ShadowRegion `json:"regions,omitempty"`
	// Inputs is the navigation observed during this slot: closed-vocabulary intents,
	// session-relative times, and no key identity — see internal/director/observe/input.go.
	//
	// Recorded on every slot including the skipped ones, because input evidence and screen
	// evidence fail independently. Without it a replayed trace attributes nothing, and the
	// production-versus-replay agreement this trace exists to establish would be measuring
	// the recorder rather than the tracker.
	Inputs []observe.InputEvent `json:"inputs,omitempty"`
	// InputStats is the producer's cumulative counters, so a replay can tell a session
	// where nobody pressed anything from one where nothing was listening.
	InputStats *observe.InputStats `json:"input_stats,omitempty"`
	// Semantic is what the readable interface said about itself, as closed-vocabulary
	// terms. No text: the vocabulary is the whole of what survives the OCR boundary.
	Semantic observe.SemanticEvidence `json:"semantic,omitzero"`
}

// maxReason keeps a diagnostic string from becoming a payload.
const maxReason = 200

var traceStart = time.Now()

// record appends one slot. Silent on error: a diagnostic must never be able to affect the
// session it is observing.
func (t *shadowTrace) record(s *observe.ShadowSample, generation uint64) {
	if t == nil || s == nil || s.Detector == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.n >= maxTracedSlots {
		return
	}
	t.n++

	slot := tracedSlot{
		N: t.n, AtMS: time.Since(traceStart).Milliseconds(),
		Generation: generation, LatencyMS: s.LatencyMS,
		// Before the outcome switch, deliberately: the navigation is real whatever became
		// of the screen evidence, and three of the four outcomes below return early.
		Inputs: s.Inputs, InputStats: s.InputStats, Semantic: s.Semantic,
	}
	switch {
	case s.Unavailable != "":
		slot.Outcome, slot.Reason = slotFailed, truncateReason(s.Unavailable, maxReason)
	case !s.Ran:
		slot.Outcome = slotSkipped
	case !s.TargetProven:
		slot.Outcome = slotUnproven
	default:
		slot.Outcome = slotValid
		slot.Detections = s.Detections
		slot.Regions = s.Regions
	}

	line, err := json.Marshal(slot)
	if err != nil {
		return
	}
	f, err := os.OpenFile(t.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(line, '\n'))
}

// truncateReason keeps a diagnostic string from becoming a payload.
func truncateReason(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// shadowGeneration is the window epoch the shadow evidence proved it observed.
//
// Taken from the OBSERVED side, not the expected one: intent is what the Director asked for,
// and a trace that recorded intent would say the generation was stable in exactly the case
// where the provider failed to establish it.
func shadowGeneration(cycle observation.Cycle) uint64 {
	if len(cycle.Shadow) == 0 {
		return 0
	}
	return cycle.Shadow[0].ObservedTarget.WindowGeneration
}
