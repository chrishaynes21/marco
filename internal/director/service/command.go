package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Command identity and lifecycle.
//
// A COMMAND is one user request — "repeat that ten times". An ACTION is one thing
// done to the desktop. They are deliberately different objects with different
// stores: that repeat is one command and up to ten action graph nodes, and
// collapsing them would make "how did that go?" unanswerable without counting.
//
// The action graph is the durable record of what happened to the computer. Command
// results are the record of what was ASKED, how far it got, and why it stopped.

// CommandID identifies one execution session.
type CommandID string

// CommandState is where a command is in its life.
type CommandState string

const (
	CommandPending    CommandState = "pending"
	CommandRunning    CommandState = "running"
	CommandCompleted  CommandState = "completed"
	CommandUnverified CommandState = "unverified"
	CommandFailed     CommandState = "failed"
	CommandCancelled  CommandState = "cancelled"
	CommandBlocked    CommandState = "blocked"
	// CommandTimedOut: a phase exceeded its deadline. Distinct from failed because
	// the action may well have happened — a timeout is the absence of confirmation,
	// not evidence that nothing occurred, and it must never invite a retry.
	CommandTimedOut CommandState = "timed_out"
	// CommandInternalError: the Director reached a state it cannot explain. Named so
	// an invariant violation is visible rather than rendered as an ordinary failure.
	CommandInternalError CommandState = "internal_error"
)

// Terminal reports whether the command has finished.
func (s CommandState) Terminal() bool {
	return s != CommandPending && s != CommandRunning
}

// ActiveCommand is the command currently in flight.
type ActiveCommand struct {
	ID        CommandID
	RequestID string
	Phrase    string
	StartedAt time.Time
	State     CommandState

	// Iteration and Total track a replay's position, so a status query during a
	// long repeat can say "4 of 10" rather than "busy".
	Iteration int
	Total     int

	// LastActionNode is the newest action graph node this command has produced.
	LastActionNode string

	cancel context.CancelFunc
}

// CommandResult is the record of one finished request.
type CommandResult struct {
	CommandID   CommandID    `json:"command_id"`
	Phrase      string       `json:"phrase"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	State       CommandState `json:"state"`
	// CompletedActions is how many desktop actions the command actually performed.
	CompletedActions int `json:"completed_actions"`
	// LastActionNode links the session to the semantic history it produced.
	LastActionNode string `json:"last_action_node,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

// Duration is how long the command took, or how long it has been running.
func (r CommandResult) Duration() time.Duration {
	if r.CompletedAt == nil {
		return time.Since(r.StartedAt)
	}
	return r.CompletedAt.Sub(r.StartedAt)
}

// ErrBusy is returned when a mutating command arrives while one is running.
type ErrBusy struct{ Active ActiveCommand }

func (e *ErrBusy) Error() string {
	// The step position matters more than the clock. "step 3 of 5" tells the user the
	// command is making progress; "running since 15:04:02" leaves them wondering
	// whether it has hung.
	if e.Active.Total > 1 {
		return fmt.Sprintf("BUSY\ncurrent program: %q\nstep %d of %d",
			e.Active.Phrase, e.Active.Iteration, e.Active.Total)
	}
	return fmt.Sprintf("BUSY\ncurrent command: %q, running since %s",
		e.Active.Phrase, e.Active.StartedAt.Format("15:04:05"))
}

// Registry owns command lifecycle: what is running, what may start, what happened.
//
// Exactly one MUTATING command may run at a time. Everything else — status,
// history, cancellation — stays available throughout, because a status query that
// blocked until a ten-iteration replay finished would be useless precisely when it
// is most wanted, and a cancel that had to wait its turn would never arrive.
type Registry struct {
	mu       sync.RWMutex
	active   *ActiveCommand
	results  []CommandResult
	capacity int
	seq      int
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{capacity: 100} }

// Begin claims the mutating slot and returns the command plus a context that
// cancellation will cut.
//
// The context is derived from the SERVICE's lifetime, not from the client
// connection. A client that hangs up must not stop work already under way: a
// dropped connection is a client that stopped listening, not a user who changed
// their mind, and treating the two the same would make every network hiccup an
// interruption.
func (r *Registry) Begin(serviceCtx context.Context, requestID, phrase string) (*ActiveCommand, context.Context, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active != nil && !r.active.State.Terminal() {
		snapshot := *r.active
		return nil, nil, &ErrBusy{Active: snapshot}
	}

	r.seq++
	ctx, cancel := context.WithCancel(serviceCtx)
	cmd := &ActiveCommand{
		ID:        CommandID(fmt.Sprintf("cmd_%d_%d", time.Now().Unix(), r.seq)),
		RequestID: requestID,
		Phrase:    phrase,
		StartedAt: time.Now(),
		State:     CommandRunning,
		cancel:    cancel,
	}
	r.active = cmd
	return cmd, ctx, nil
}

// Progress updates the running command's position, for status queries.
func (r *Registry) Progress(id CommandID, iteration, total int, node string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active == nil || r.active.ID != id {
		return
	}
	if iteration > 0 {
		r.active.Iteration = iteration
	}
	if total > 0 {
		r.active.Total = total
	}
	if node != "" {
		r.active.LastActionNode = node
	}
}

// Finish releases the mutating slot and records the result.
func (r *Registry) Finish(id CommandID, state CommandState, completedActions int, reason string) CommandResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	result := CommandResult{
		CommandID: id, State: state, CompletedAt: &now,
		CompletedActions: completedActions, Reason: reason,
	}
	if r.active != nil && r.active.ID == id {
		result.Phrase = r.active.Phrase
		result.StartedAt = r.active.StartedAt
		result.LastActionNode = r.active.LastActionNode
		r.active.cancel() // release the context's resources
		r.active = nil
	}

	r.results = append(r.results, result)
	if len(r.results) > r.capacity {
		r.results = r.results[len(r.results)-r.capacity:]
	}
	return result
}

// Cancel stops the running command, if there is one.
//
// It returns what was cancelled so the caller can say so. Cancelling nothing is not
// an error — "stop" when nothing is running is a reasonable thing for a person to
// say, and answering "there was nothing to stop" is more useful than a failure.
func (r *Registry) Cancel() (ActiveCommand, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active == nil || r.active.State.Terminal() {
		return ActiveCommand{}, false
	}
	snapshot := *r.active
	r.active.State = CommandCancelled
	r.active.cancel()
	return snapshot, true
}

// Active returns the running command, if any.
func (r *Registry) Active() (ActiveCommand, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.active == nil || r.active.State.Terminal() {
		return ActiveCommand{}, false
	}
	return *r.active, true
}

// Recent returns the most recent finished commands, newest first.
func (r *Registry) Recent(limit int) []CommandResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 || limit > len(r.results) {
		limit = len(r.results)
	}
	out := make([]CommandResult, 0, limit)
	for i := len(r.results) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, r.results[i])
	}
	return out
}

// Conversation is what a follow-up phrase resolves against.
//
// Deliberately tiny for this milestone. It exists so "do that again" means the same
// thing whichever client process asks — not to be a general memory. The action graph
// remains the durable source of semantic history; this is a pointer into it.
type Conversation struct {
	mu             sync.RWMutex
	lastCommandID  CommandID
	lastActionNode string
	lastPhrase     string
	updatedAt      time.Time
}

// NewConversation returns empty conversation state.
func NewConversation() *Conversation { return &Conversation{} }

// Record notes what the most recent command was.
func (c *Conversation) Record(id CommandID, phrase, node string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastCommandID = id
	c.lastPhrase = phrase
	if node != "" {
		c.lastActionNode = node
	}
	c.updatedAt = time.Now()
}

// Summary reports the current state.
func (c *Conversation) Summary() ConversationSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return ConversationSummary{
		LastPhrase:     c.lastPhrase,
		LastActionNode: c.lastActionNode,
		LastCommandID:  c.lastCommandID,
		UpdatedAt:      c.updatedAt,
	}
}
