package directorapi

import (
	"context"
	"time"
)

// Director is the whole system behind one entry point. A front-end — the CLI, the
// overlay, a voice layer, a service — hands it a Request and gets back a Result it
// can render, without knowing anything about world models, plans or providers.
//
// Handle is expected to be the ONLY method that matters for a long time. Keeping
// the surface this small is what allows the Director to move in-process, out of
// process, or behind a network boundary without any caller changing.
type Director interface {
	Handle(ctx context.Context, request Request) (Result, error)
}

// Request is one thing asked of the Director.
type Request struct {
	// Input is what the user said or typed.
	Input string `json:"input"`
	// SessionID groups requests into one conversation, so "it" and "that" resolve
	// against the right history when several front-ends share a Director.
	SessionID string `json:"session_id,omitempty"`
	// Confirm answers a pending confirmation rather than starting new work. When
	// set, Input is ignored.
	Confirm *ConfirmationResponse `json:"confirm,omitempty"`
	// DryRun plans, validates and explains without executing — the safe way to see
	// what the Director would do.
	DryRun bool `json:"dry_run,omitempty"`
	// Source identifies the front-end ("cli", "overlay", "voice"), recorded with the
	// action and available to policy: a voice command in a noisy room is not the
	// same evidence of intent as a typed one.
	Source string `json:"source,omitempty"`
}

// ConfirmationResponse answers a Clarification.
type ConfirmationResponse struct {
	// Approved is the yes/no answer.
	Approved bool `json:"approved"`
	// ChosenEntityID selects one of the offered options, when the question was a
	// choice between candidates.
	ChosenEntityID string `json:"chosen_entity_id,omitempty"`
}

// ResultStatus is how a request ended.
type ResultStatus string

const (
	// ResultDone means the work completed and verified.
	ResultDone ResultStatus = "done"
	// ResultPartial means some steps ran and then stopped. The records say exactly
	// how far it got — a partial result is reported, never hidden behind an error.
	ResultPartial ResultStatus = "partial"
	// ResultNeedsConfirmation means a plan is ready and waiting on the user.
	ResultNeedsConfirmation ResultStatus = "needs_confirmation"
	// ResultNeedsClarification means the request was ambiguous.
	ResultNeedsClarification ResultStatus = "needs_clarification"
	// ResultBlocked means policy refused.
	ResultBlocked ResultStatus = "blocked"
	// ResultFailed means it could not be done, with the reason.
	ResultFailed ResultStatus = "failed"
	// ResultAnswered means a query was answered with no action taken.
	ResultAnswered ResultStatus = "answered"
	// ResultCancelled means the user stopped it.
	ResultCancelled ResultStatus = "cancelled"
	// ResultNothingToDo means the input asked for nothing.
	ResultNothingToDo ResultStatus = "nothing_to_do"
)

// Result is the Director's structured answer to a Request.
//
// It carries the reasoning, not just the outcome. Every field after Message exists
// so a caller can show — or a developer can audit — what the Director believed, what
// it considered, what it chose and why. A Result that says only "failed" would make
// the system unfixable in exactly the situations that matter.
type Result struct {
	Status ResultStatus `json:"status"`
	// Message is a short sentence for the user.
	Message string `json:"message"`
	// Question is set when the status needs an answer.
	Question *Clarification `json:"question,omitempty"`

	Intent *Intent `json:"intent,omitempty"`
	Plan   *Plan   `json:"plan,omitempty"`
	// Execution is the per-step outcome, present whenever anything ran.
	Execution *ExecutionResult `json:"execution,omitempty"`

	// Explanation is the decision trace: what was on screen, which targets were
	// considered, why one won.
	Explanation *Explanation `json:"explanation,omitempty"`

	// Error is the failure detail for ResultFailed.
	Error string `json:"error,omitempty"`

	Duration time.Duration `json:"duration"`
}

// Explanation is the inspectable record of one decision.
//
// This is the shape behind the Director's debug output: intent, the candidates it
// weighed with their scores and sources, the one it selected, and the reason. It is
// produced whether or not anyone asks for it, because the moment you need it is
// after something went wrong.
//
// Sensitive values are redacted before an Explanation is built — no credentials, no
// password-field contents, no clipboard text.
type Explanation struct {
	// Request is what the user asked.
	Request string `json:"request"`
	// WorldSummary is a one-line description of what the Director believed was on
	// screen ("Notepad — Save As dialog, 14 elements, sources: accessibility").
	WorldSummary string `json:"world_summary,omitempty"`
	// Candidates are the targets considered, best first.
	Candidates []TargetCandidate `json:"candidates,omitempty"`
	// Selected is the chosen target.
	Selected *ResolvedTarget `json:"selected,omitempty"`
	// Reason is why it was selected.
	Reason string `json:"reason,omitempty"`
	// PolicyDecisions are the policy verdicts applied.
	PolicyDecisions []PolicyDecision `json:"policy_decisions,omitempty"`
	// Degraded lists sources that were unavailable, so a weak decision can be
	// attributed to missing evidence rather than bad judgement.
	Degraded []SourceFailure `json:"degraded,omitempty"`
	// Notes are additional lines the pipeline chose to record.
	Notes []string `json:"notes,omitempty"`
}
