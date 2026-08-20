package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
)

// Putting a confirmation to a person, over the wire.
//
//	nil confirmer → unavailable, and no action may execute after it.
//
// The execution layer has had a Confirmer interface and a full policy for a milestone,
// and the daemon has installed nothing — so every action that needed agreement was
// BLOCKED. That was the safe failure, and it was still a failure: the Director could not
// delete, print, or discard anything at all.
//
// This is the transport that closes it, and it is deliberately only a transport. It
// decides nothing: it takes the question the execution layer composed, sends it to
// whichever client is watching the command, blocks until an answer arrives, and returns
// the answer. Which questions get asked, what covers what, and what happens on a refusal
// are all decided in internal/director/execute and are not restated here.
//
// The three ways it can fail to get an answer are three different things, and the
// execution layer already distinguishes them:
//
//   - nobody is listening → an error → unavailable → BLOCKED;
//   - nobody answered in time → an error → unavailable → BLOCKED;
//   - the request was cancelled → the context is done → cancelled.
//
// None of them is a yes. There is no configuration in which this returns true without a
// person having said so.

// DefaultConfirmationTimeout bounds how long a question may stay open.
//
// Long enough for a person to read a prompt and decide, short enough that a command
// dispatched by voice at a machine nobody is watching does not hold the Director open
// indefinitely. Timing out BLOCKS, so the cost of it being too short is a refused action
// rather than an unattended one.
const DefaultConfirmationTimeout = 90 * time.Second

// ConfirmationPayload is a CONFIRMATION_REQUIRED event.
//
// The fields are what a person needs to answer: what will happen, to which object, how
// bad it would be to get wrong, and where the question came from. Composed by the
// execution layer — this type carries it, and adds only the id an answer must quote.
type ConfirmationPayload struct {
	// ID identifies THIS question. An answer carrying a different id was about a
	// different question and is refused rather than applied to whatever is pending now.
	ID string `json:"id"`
	// CommandID is the command that asked.
	CommandID CommandID `json:"command_id,omitempty"`

	// Scope is "goal", "action" or "replay".
	Scope string `json:"scope"`
	// Request is the user's own words.
	Request string `json:"request,omitempty"`
	// Action is the semantic action about to be performed.
	Action string `json:"action,omitempty"`
	// Target is the human-readable thing acted on, and Resource its backing identity —
	// a file path — when the binding established one.
	Target   string `json:"target,omitempty"`
	Resource string `json:"resource,omitempty"`
	// Effect is what it is expected to do; Reason is why agreement is needed.
	Effect string `json:"effect,omitempty"`
	Reason string `json:"reason,omitempty"`
	Risk   string `json:"risk,omitempty"`

	// Goal, Procedure and the step position are provenance, present when the action
	// came from a goal expansion.
	Goal      string `json:"goal,omitempty"`
	Procedure string `json:"procedure,omitempty"`
	StepIndex int    `json:"step_index,omitempty"`
	StepCount int    `json:"step_count,omitempty"`
	// Steps are the phrases of an expanded procedure, for a goal-scoped question.
	Steps []string `json:"steps,omitempty"`

	// TargetChanged and Changes report that the bound object was re-established under a
	// changed world since anything was last agreed to.
	TargetChanged bool     `json:"target_changed,omitempty"`
	Changes       []string `json:"changes,omitempty"`
	// ReplayOf is the recorded node being repeated, and StoredConfirmation what its
	// original run concluded — disclosed, never treated as permission.
	ReplayOf           string `json:"replay_of,omitempty"`
	StoredConfirmation string `json:"stored_confirmation,omitempty"`

	AskedAt   time.Time `json:"asked_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Question renders the payload as the sentence to put to a person.
func (c ConfirmationPayload) Question() string {
	subject := c.Target
	if c.Resource != "" {
		subject = c.Resource
	}
	out := c.Action
	if out == "" {
		out = c.Request
	}
	if subject != "" {
		out += " " + subject
	}
	if c.Effect != "" {
		out += " — " + c.Effect
	}
	return out + "?"
}

// ConfirmPayload answers a pending confirmation.
type ConfirmPayload struct {
	// ID must match the question. An answer for a question that is no longer open is
	// refused: the person answered something they can no longer see.
	ID string `json:"id"`
	// Approved is the answer. False is a refusal, which is a normal outcome.
	Approved bool `json:"approved"`
}

// ConfirmResultPayload reports whether an answer was accepted.
type ConfirmResultPayload struct {
	Accepted bool   `json:"accepted"`
	Message  string `json:"message,omitempty"`
}

// ConfirmationBroker asks a connected client and waits for the answer.
//
// Implements execute.Confirmer, which is what makes it installable through the runtime's
// existing SetConfirmer — no alternate path, no compile-time special case, and the same
// gate the regression suite already covers.
type ConfirmationBroker struct {
	// Timeout bounds one question. Zero means DefaultConfirmationTimeout.
	Timeout time.Duration
	// Now is injectable so a test can pin the timestamps.
	Now func() time.Time

	mu      sync.Mutex
	seq     int
	pending *ConfirmationPayload
	answer  chan bool
	// publish sends the question to whoever is watching the running command. Nil when
	// nothing is — which is "there is no way to ask", not "assume yes".
	publish func(ConfirmationPayload)
}

// NewConfirmationBroker returns a broker with the default timeout.
func NewConfirmationBroker() *ConfirmationBroker {
	return &ConfirmationBroker{Timeout: DefaultConfirmationTimeout, Now: time.Now}
}

var _ execute.Confirmer = (*ConfirmationBroker)(nil)

func (b *ConfirmationBroker) now() time.Time {
	if b.Now == nil {
		return time.Now()
	}
	return b.Now()
}

func (b *ConfirmationBroker) timeout() time.Duration {
	if b.Timeout > 0 {
		return b.Timeout
	}
	return DefaultConfirmationTimeout
}

// Watch installs the channel a question is published on, for the duration of one command.
//
// Returns the function that removes it again. Scoped to a command rather than to the
// server, because a question belongs to the request that asked it: publishing to a client
// that has since disconnected would leave the question open until it timed out.
func (b *ConfirmationBroker) Watch(id CommandID, publish func(ConfirmationPayload)) func() {
	b.mu.Lock()
	b.publish = func(p ConfirmationPayload) {
		p.CommandID = id
		publish(p)
	}
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		b.publish = nil
		// Any question still open is abandoned with the command that asked it. Leaving
		// it would let the NEXT command's answer arrive for the previous command's
		// question, which is the confirmation equivalent of a stale ordinal.
		b.discardLocked()
		b.mu.Unlock()
	}
}

// Confirm puts one question and blocks for the answer.
func (b *ConfirmationBroker) Confirm(ctx context.Context, req execute.ConfirmationRequest) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	b.mu.Lock()
	if b.publish == nil {
		b.mu.Unlock()
		return false, fmt.Errorf("nothing is listening for confirmations, so the question " +
			"could not be put")
	}
	if b.pending != nil {
		// One at a time. A second question while one is open would make an answer
		// ambiguous, and the execution path asks strictly sequentially — so this is a
		// programming error rather than a race to be tolerated.
		b.mu.Unlock()
		return false, fmt.Errorf("another confirmation is already open")
	}
	b.seq++
	deadline := b.timeout()
	payload := payloadFor(req)
	payload.ID = fmt.Sprintf("confirm-%d", b.seq)
	payload.AskedAt = b.now()
	payload.ExpiresAt = payload.AskedAt.Add(deadline)
	answer := make(chan bool, 1)
	b.pending, b.answer = &payload, answer
	publish := b.publish
	b.mu.Unlock()

	publish(payload)

	timer := time.NewTimer(deadline)
	defer timer.Stop()

	select {
	case ok := <-answer:
		b.discard(payload.ID)
		return ok, nil
	case <-ctx.Done():
		b.discard(payload.ID)
		// The context error, not a refusal: the execution layer maps a cancelled context
		// to "cancelled", which is what happened — the person did not decide.
		return false, ctx.Err()
	case <-timer.C:
		b.discard(payload.ID)
		return false, fmt.Errorf("no answer within %s", deadline)
	}
}

// Answer records a person's decision. It reports whether the answer was accepted.
func (b *ConfirmationBroker) Answer(id string, approved bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pending == nil {
		return fmt.Errorf("no confirmation is waiting for an answer")
	}
	if id != "" && id != b.pending.ID {
		return fmt.Errorf("that answer is for %q and the open question is %q", id, b.pending.ID)
	}
	select {
	case b.answer <- approved:
	default:
		return fmt.Errorf("that question has already been answered")
	}
	return nil
}

// Pending returns the open question, if any.
func (b *ConfirmationBroker) Pending() (ConfirmationPayload, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pending == nil {
		return ConfirmationPayload{}, false
	}
	return *b.pending, true
}

func (b *ConfirmationBroker) discard(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pending != nil && b.pending.ID == id {
		b.discardLocked()
	}
}

func (b *ConfirmationBroker) discardLocked() {
	b.pending, b.answer = nil, nil
}

// payloadFor turns the execution layer's question into the wire form.
//
// A projection and nothing more: every field comes from the request, and none is
// computed, defaulted or softened here. A transport that reworded a destructive prompt
// would be deciding what the user is agreeing to.
func payloadFor(req execute.ConfirmationRequest) ConfirmationPayload {
	p := ConfirmationPayload{
		Scope: string(req.Scope), Request: req.Request, Action: req.Action,
		Target: req.Target, Resource: req.Resource, Effect: req.Effect,
		Reason: req.Reason, Risk: string(req.Risk),
		Goal: string(req.Goal), Procedure: req.Procedure,
		StepIndex: req.StepIndex, StepCount: req.StepCount, Steps: req.Steps,
		TargetChanged: req.TargetChanged, Changes: req.Changes,
	}
	if req.Replay != nil {
		p.ReplayOf = req.Replay.SourceNode
		p.StoredConfirmation = req.Replay.StoredConfirmation
	}
	return p
}
