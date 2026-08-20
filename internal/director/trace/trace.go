// Package trace records what a command is doing, phase by phase, with timings.
//
// The rules it exists to enforce:
//
//	Slow is diagnosable. Stuck is bounded.
//	A timeout names what timed out and never implies the action did not happen.
//
// The motivating incident was a command that appeared to take three minutes. Nothing
// in the system could say which part of it was slow, because durations were only ever
// visible as the total. A trace that has to be reconstructed from log ordering after
// the fact is not a trace: it cannot see a phase that never finished, which is exactly
// the case worth seeing.
//
// So timing is recorded by the code that PERFORMS the phase, a started phase is always
// closed, and the phase that was active when a deadline expired is part of the error.
package trace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CommandPhase names one stage of a command. A closed vocabulary: a free-form phase
// string would let a new call site invent a name nothing else knows how to report on,
// which is how "unnamed phase" comes back.
type CommandPhase string

const (
	PhaseRoutePhrase        CommandPhase = "route_phrase"
	PhaseProgramValidation  CommandPhase = "program_validation"
	PhaseObserve            CommandPhase = "observe"
	PhaseResolve            CommandPhase = "resolve"
	PhasePolicy             CommandPhase = "policy"
	PhaseLower              CommandPhase = "lower"
	PhaseCompile            CommandPhase = "compile"
	PhaseRuntimeExecute     CommandPhase = "runtime_execute"
	PhaseSettle             CommandPhase = "settle"
	PhaseVerify             CommandPhase = "verify"
	PhaseClarificationBuild CommandPhase = "clarification_build"
	PhaseClarificationStore CommandPhase = "clarification_store"
	PhaseClarificationEmit  CommandPhase = "clarification_emit"
	PhaseProgramPause       CommandPhase = "program_pause"
	PhaseProgramResume      CommandPhase = "program_resume"
	// PhaseCaptureValue and PhaseValueRead are the data-flow phases. They record the
	// name, kind, visibility and byte length of a value — never its content, which is
	// why they can be traced at all.
	PhaseCaptureValue CommandPhase = "capture_value"
	PhaseValueRead    CommandPhase = "value_read"
	// PhaseProgramEnd is where program-local values are discarded. Traced because the
	// lifetime rule is a promise, and a promise nobody can observe being kept is
	// indistinguishable from one that is not.
	PhaseProgramEnd    CommandPhase = "program_end"
	PhaseResponseWrite CommandPhase = "response_write"
)

// PhaseState is how a phase ended, or that it has not.
type PhaseState string

const (
	// StateRunning: started and not yet closed. A phase left in this state after the
	// command finished is a bug in the tracing, and the trace shows it rather than
	// hiding it.
	StateRunning   PhaseState = "RUNNING"
	StateCompleted PhaseState = "COMPLETED"
	StateFailed    PhaseState = "FAILED"
	// StateTimedOut: the phase's deadline expired. Distinct from FAILED because the
	// action may well have happened — see ErrPhaseTimeout.
	StateTimedOut PhaseState = "TIMED_OUT"
	// StateCancelled: the user or a stop asked. Distinct from TIMED_OUT because
	// nothing was wrong.
	StateCancelled PhaseState = "CANCELLED"
)

// PhaseRecord is one phase of one command.
type PhaseRecord struct {
	Phase     CommandPhase  `json:"phase"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   *time.Time    `json:"ended_at,omitempty"`
	Duration  time.Duration `json:"duration"`
	State     PhaseState    `json:"state"`
	// Reason explains a non-COMPLETED end, empty otherwise.
	Reason string `json:"reason,omitempty"`
	// Deadline is what bounded this phase, zero when unbounded.
	Deadline time.Duration `json:"deadline,omitempty"`

	// Step position, zero outside a program.
	StepIndex int    `json:"step_index,omitempty"`
	StepCount int    `json:"step_count,omitempty"`
	StepID    string `json:"step_id,omitempty"`
}

// Elapsed is how long the phase has run, or ran.
func (p PhaseRecord) Elapsed() time.Duration {
	if p.EndedAt != nil {
		return p.Duration
	}
	return time.Since(p.StartedAt)
}

// Metadata is the per-phase context a caller supplies.
//
// Deliberately tiny, and deliberately carries no payload. A trace that copied
// observation graphs or screen text would be both expensive on the hot path and a
// place private content could leak into a diagnostic dump.
type Metadata struct {
	StepIndex int
	StepCount int
	StepID    string
	Deadline  time.Duration
}

// Trace is one command's phases, in order.
type Trace struct {
	CommandID string     `json:"command_id"`
	Phrase    string     `json:"phrase,omitempty"`
	ProgramID string     `json:"program_id,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	// Terminal is the command's final state, empty while running.
	Terminal string `json:"terminal,omitempty"`

	mu     sync.Mutex
	phases []PhaseRecord
	// events are the value data-flow records. Separate from phases because they
	// describe DATA rather than time: a phase answers "how long did resolving take?",
	// an event answers "what was captured, and who read it?". See dataflow.go.
	events []ValueEvent
	// ParentCommand links a clarification answer back to the command that asked, so a
	// resumed program reads as one story rather than two unrelated ones.
	ParentCommand string `json:"parent_command,omitempty"`
}

// New starts a trace.
func New(commandID, phrase string) *Trace {
	return &Trace{CommandID: commandID, Phrase: phrase, StartedAt: time.Now()}
}

// maxPhases bounds one trace. A command that somehow produced thousands of phases is a
// bug, and holding them all would turn a diagnostic into a leak.
const maxPhases = 500

// begin opens a phase and returns its index.
func (t *Trace) begin(phase CommandPhase, meta Metadata) int {
	if t == nil {
		return -1
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.phases) >= maxPhases {
		return -1
	}
	t.phases = append(t.phases, PhaseRecord{
		Phase: phase, StartedAt: time.Now(), State: StateRunning,
		StepIndex: meta.StepIndex, StepCount: meta.StepCount,
		StepID: meta.StepID, Deadline: meta.Deadline,
	})
	return len(t.phases) - 1
}

// end closes a phase exactly once.
func (t *Trace) end(i int, state PhaseState, reason string) {
	if t == nil || i < 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if i >= len(t.phases) || t.phases[i].EndedAt != nil {
		return
	}
	now := time.Now()
	t.phases[i].EndedAt = &now
	t.phases[i].Duration = now.Sub(t.phases[i].StartedAt)
	t.phases[i].State = state
	t.phases[i].Reason = reason
}

// Phases returns a copy of the records.
//
// A copy, taken under the lock and returned without it, because status reads this
// while a command is running. Handing out the live slice would make every reader a
// participant in the command's locking.
func (t *Trace) Phases() []PhaseRecord {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]PhaseRecord, len(t.phases))
	copy(out, t.phases)
	return out
}

// Active returns the phase currently running, if any.
//
// The last one still open, because phases nest: an observe inside a step inside a
// program. The innermost is what the command is actually doing.
func (t *Trace) Active() (PhaseRecord, bool) {
	if t == nil {
		return PhaseRecord{}, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := len(t.phases) - 1; i >= 0; i-- {
		if t.phases[i].EndedAt == nil {
			return t.phases[i], true
		}
	}
	return PhaseRecord{}, false
}

// Finish closes the trace with a terminal state, and closes any phase left open.
//
// The sweep matters: a phase still RUNNING when the command ended is either a tracing
// bug or a goroutine that outlived its command, and both are worth seeing. Leaving
// them open would make Active() report a phase forever.
func (t *Trace) Finish(terminal string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	now := time.Now()
	for i := range t.phases {
		if t.phases[i].EndedAt == nil {
			t.phases[i].EndedAt = &now
			t.phases[i].Duration = now.Sub(t.phases[i].StartedAt)
			t.phases[i].State = StateFailed
			t.phases[i].Reason = "the command ended while this phase was still open"
		}
	}
	t.EndedAt = &now
	t.Terminal = terminal
	t.mu.Unlock()
}

// Total is how long the command has taken.
func (t *Trace) Total() time.Duration {
	if t == nil {
		return 0
	}
	if t.EndedAt != nil {
		return t.EndedAt.Sub(t.StartedAt)
	}
	return time.Since(t.StartedAt)
}

// ErrPhaseTimeout is a phase that exceeded its deadline.
//
// It names the phase, because "timed out" without one is the unnamed-phase problem
// wearing a different hat. It is deliberately NOT a failure: a mutating operation that
// did not confirm in time may well have happened, and treating it as "did not happen"
// is how a system retries something it already did.
type ErrPhaseTimeout struct {
	Phase    CommandPhase
	Deadline time.Duration
	Elapsed  time.Duration
}

func (e *ErrPhaseTimeout) Error() string {
	return fmt.Sprintf("TIMED_OUT during %s after %s (deadline %s)",
		e.Phase, e.Elapsed.Round(time.Millisecond), e.Deadline)
}

// Timeout reports whether an error is a phase timeout, and which phase.
func Timeout(err error) (*ErrPhaseTimeout, bool) {
	var t *ErrPhaseTimeout
	ok := errors.As(err, &t)
	return t, ok
}

// Do runs one phase, recording its timing and applying its deadline.
//
// The single source of timing truth: the code that performs the phase is the code that
// records it, so there is no window in which a phase is happening and untracked. The
// close is deferred, so a panic or an early return cannot leave a phase RUNNING.
//
// Classification order matters. A cancelled context that ALSO passed its deadline is
// reported as cancelled, because the user asking to stop is the more informative fact
// and because reporting it as a timeout would invent a fault that did not occur.
func Do(ctx context.Context, t *Trace, phase CommandPhase, meta Metadata,
	fn func(context.Context) error) error {

	i := t.begin(phase, meta)
	started := time.Now()

	runCtx := ctx
	var cancel context.CancelFunc
	if meta.Deadline > 0 {
		runCtx, cancel = context.WithTimeout(ctx, meta.Deadline)
		defer cancel()
	}

	err := fn(runCtx)

	switch {
	case err == nil:
		t.end(i, StateCompleted, "")
		return nil

	case ctx.Err() != nil:
		// The OUTER context, not the phase's. A cancelled command is cancelled
		// whatever the phase's own deadline did.
		t.end(i, StateCancelled, ctx.Err().Error())
		return err

	case meta.Deadline > 0 && runCtx.Err() == context.DeadlineExceeded:
		to := &ErrPhaseTimeout{Phase: phase, Deadline: meta.Deadline, Elapsed: time.Since(started)}
		t.end(i, StateTimedOut, to.Error())
		// Wrapped, not replaced: the underlying error often says what the operation
		// was doing when the deadline hit, and that is the useful half.
		return fmt.Errorf("%w: %v", to, err)

	default:
		t.end(i, StateFailed, err.Error())
		return err
	}
}

// Mark records a phase that has no operation to bound — a state transition such as a
// pause, which happens instantaneously and is worth seeing in the sequence.
func Mark(t *Trace, phase CommandPhase, meta Metadata, reason string) {
	i := t.begin(phase, meta)
	t.end(i, StateCompleted, reason)
}

// wireTrace is the serialisable shape of a Trace.
//
// A Trace holds its phases behind a mutex, so the slice is unexported and JSON cannot
// see it — which is how a trace crossed the service boundary carrying a total and no
// phases at all. Marshalling through a snapshot fixes that AND keeps the guarantee
// that nobody reads the live slice: the copy is taken under the lock, and the encoder
// only ever sees the copy.
type wireTrace struct {
	CommandID     string        `json:"command_id"`
	Phrase        string        `json:"phrase,omitempty"`
	ProgramID     string        `json:"program_id,omitempty"`
	ParentCommand string        `json:"parent_command,omitempty"`
	StartedAt     time.Time     `json:"started_at"`
	EndedAt       *time.Time    `json:"ended_at,omitempty"`
	Terminal      string        `json:"terminal,omitempty"`
	Phases        []PhaseRecord `json:"phases"`
	// Events are the value data-flow records. Carried across the boundary for the
	// same reason the phases are: a trace that arrived without them would look like a
	// command that moved no data.
	Events []ValueEvent `json:"value_events,omitempty"`
}

// MarshalJSON snapshots the trace, phases included.
func (t *Trace) MarshalJSON() ([]byte, error) {
	if t == nil {
		return []byte("null"), nil
	}
	w := wireTrace{
		CommandID: t.CommandID, Phrase: t.Phrase, ProgramID: t.ProgramID,
		ParentCommand: t.ParentCommand, StartedAt: t.StartedAt,
		EndedAt: t.EndedAt, Terminal: t.Terminal, Phases: t.Phases(),
		Events: t.ValueEvents(),
	}
	return json.Marshal(w)
}

// UnmarshalJSON restores a trace on the client side.
func (t *Trace) UnmarshalJSON(b []byte) error {
	var w wireTrace
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	t.CommandID, t.Phrase, t.ProgramID = w.CommandID, w.Phrase, w.ProgramID
	t.ParentCommand, t.StartedAt, t.EndedAt = w.ParentCommand, w.StartedAt, w.EndedAt
	t.Terminal = w.Terminal
	t.mu.Lock()
	t.phases = w.Phases
	// The events have to be restored too, and forgetting them is invisible: a client
	// decodes the trace and re-encodes it to print, so events that survive the wire but
	// not the decode vanish silently between the service and the terminal. That is
	// exactly how they came to be missing from `director trace last --json` while every
	// unit test on the producing side passed.
	t.events = w.Events
	t.mu.Unlock()
	return nil
}
