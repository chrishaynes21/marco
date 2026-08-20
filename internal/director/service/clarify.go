package service

import (
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Clarification sessions, and the routing decision a front-end makes.
//
// The division of labour here is the milestone's architectural rule made concrete:
//
//   - the front-end captures speech and ROUTES it — is this a control phrase, an
//     answer to a pending question, or a new request;
//   - the Director INTERPRETS it — which candidate "the second one" means, and what
//     to do about it.
//
// Routing is a decision about which request to send. Interpretation is a decision
// about the desktop. A front-end that did the second would be choosing candidates
// itself, which is exactly what it must not do.

// ClarificationCandidate is one option offered to the user.
//
// Deliberately thin: an index, a short label and a role. Enough for a person to
// choose between them, and not enough for a front-end to reason about the screen.
type ClarificationCandidate struct {
	// Index is the 1-based position, which is what "the second one" refers to.
	Index int `json:"index"`
	// ID is the element id at the time the question was asked. Recorded for the
	// record, never used to act: by the time an answer arrives the screen may have
	// changed, so the answer is resolved afresh.
	ID    string `json:"id,omitempty"`
	Label string `json:"label"`
	Role  string `json:"role,omitempty"`
	// Application and WindowTitle let a user answer "the one in Notepad" — a
	// distinction two identically-labelled controls in different windows cannot be
	// made any other way.
}

// ClarificationPayload is a CLARIFICATION_REQUIRED event.
type ClarificationPayload struct {
	CommandID CommandID `json:"command_id"`
	// Phrase is the original request, so an answer can re-run it.
	Phrase     string                   `json:"phrase"`
	Question   string                   `json:"question"`
	Candidates []ClarificationCandidate `json:"candidates"`
	AskedAt    time.Time                `json:"asked_at"`

	// Program fields, set when the question interrupted a SEQUENCE. They are what
	// lets a client say "step 2 of 4" instead of just asking, and what proves to a
	// reader that the completed steps are not about to be re-run.
	//
	// Empty for a single-step request, which is how a client tells the two apart.
	ProgramID string `json:"program_id,omitempty"`
	StepID    string `json:"step_id,omitempty"`
	StepIndex int    `json:"step_index,omitempty"`
	StepCount int    `json:"step_count,omitempty"`
	// CompletedSteps is how many steps already verified. Stated separately from
	// StepIndex because they can differ: a best-effort step counts as completed
	// without having been proved.
	CompletedSteps int `json:"completed_steps,omitempty"`

	// EventID identifies THIS offer. An ordinal belongs to one offered list in one
	// observed world, so an answer carrying a different id was about a different
	// question and must not be applied to this one.
	//
	// Empty for a question that is not part of a collection iteration, where the
	// command id already distinguishes offers.
	EventID string `json:"event_id,omitempty"`
	// CollectionName and Iteration locate the question inside a bounded set, so a
	// client can say "iteration 3, 2 already verified".
	CollectionName string `json:"collection_name,omitempty"`
	Iteration      int    `json:"iteration_index,omitempty"`
	CompletedItems int    `json:"completed_items,omitempty"`
}

// Program reports whether this question interrupted a sequence.
func (c ClarificationPayload) Program() bool { return c.ProgramID != "" }

// ClarifyPayload answers a pending question.
type ClarifyPayload struct {
	// CommandID must match the question being answered. An answer that arrives for a
	// different command is refused rather than applied to whatever is pending now:
	// the user answered a question that is no longer on screen.
	CommandID CommandID `json:"command_id"`
	Response  string    `json:"response"`
	// EventID is the offer this answer is about. Validated EXPLICITLY rather than
	// inferred: without it, an answer written for one contender list would be applied
	// to whichever list happens to be pending when it arrives — the same failure as a
	// stale ordinal, reached through the transport instead of through the world.
	//
	// Empty from an older client, which stays compatible: the check below only rejects
	// a MISMATCH, never an absence.
	EventID string `json:"event_id,omitempty"`
}

// pendingClarification is a question awaiting an answer.
type pendingClarification struct {
	mu      sync.RWMutex
	payload *ClarificationPayload
}

// Set records a question.
func (p *pendingClarification) Set(c ClarificationPayload) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.payload = &c
}

// Clear forgets any pending question.
func (p *pendingClarification) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.payload = nil
}

// Get returns the pending question, if any.
func (p *pendingClarification) Get() (ClarificationPayload, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.payload == nil {
		return ClarificationPayload{}, false
	}
	return *p.payload, true
}

// ── phrase routing ────────────────────────────────────────────────────────────

// RouteKind is what a front-end should do with a recognised phrase.
type RouteKind string

const (
	// RouteCancel sends CANCEL_ACTIVE. A bare "stop" never reaches the planner.
	RouteCancel RouteKind = "cancel"
	// RouteClarify answers a pending question.
	RouteClarify RouteKind = "clarify"
	// RouteExecute submits a new request.
	RouteExecute RouteKind = "execute"
)

// PhraseRoute is the routing decision for one phrase.
type PhraseRoute struct {
	Kind RouteKind
	// CommandID is the question being answered, for RouteClarify.
	CommandID CommandID
	Phrase    string
	// Why explains the decision, for the front-end's status line and for logs.
	Why string
}

// RoutePhrase decides what to do with a recognised phrase, given what the service
// is currently doing.
//
// Pure, and therefore testable without a service or a desktop. The order of the
// checks is the whole of it:
//
//  1. A control phrase always cancels — including while a question is pending, so a
//     user who changes their mind mid-clarification is not stuck answering it.
//  2. A pending question captures the next phrase as its answer, so "the second one"
//     is not resolved against the screen as a control called "the second one".
//  3. Everything else is a new request.
func RoutePhrase(phrase string, status StatusPayload) PhraseRoute {
	if intent.IsControlPhrase(phrase) {
		return PhraseRoute{
			Kind: RouteCancel, Phrase: phrase,
			Why: "a control phrase — cancels rather than planning",
		}
	}
	if status.Clarification != nil {
		return PhraseRoute{
			Kind: RouteClarify, CommandID: status.Clarification.CommandID, Phrase: phrase,
			Why: "answering the pending question for " + string(status.Clarification.CommandID),
		}
	}
	return PhraseRoute{Kind: RouteExecute, Phrase: phrase, Why: "a new request"}
}

// ReadTextPayload asks for one OCR pass.
type ReadTextPayload struct {
	// Region narrows the read, in canonical desktop coordinates. Nil reads the whole
	// active window.
	Region *directorapi.Rect `json:"region,omitempty"`
}
