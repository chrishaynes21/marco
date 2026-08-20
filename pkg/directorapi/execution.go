package directorapi

import (
	"context"
	"time"
)

// DirectorExecutor performs one semantic action against the real desktop.
//
// It is the narrow waist between deciding and doing. Everything above it —
// perception, resolution, planning, policy — produces an Action; everything below it
// is Marco's existing execution machinery, reached through directorapi.Actuator and
// never reimplemented here. There is deliberately no mouse or keyboard code anywhere
// in the Director.
//
// An executor does NOT verify. It reports what it did and whether the underlying
// call succeeded; whether that had the intended effect is a separate question,
// answered by comparing world states afterwards. Conflating the two is how a system
// comes to believe that a click which "succeeded" actually pressed something.
type DirectorExecutor interface {
	Execute(ctx context.Context, action Action) (ExecutionOutcome, error)
}

// ExecutionOutcome is what an executor reports about a single action.
type ExecutionOutcome struct {
	// Performed is whether the underlying call went through. It is NOT a claim that
	// the action had its intended effect.
	Performed bool `json:"performed"`
	// Detail is a human phrase for the explanation trace ("mouse click sent").
	Detail string `json:"detail,omitempty"`
	// Point is where a positional action landed, recorded as evidence of what was
	// done — never as something to replay.
	Point *Point `json:"point,omitempty"`
	// Duration is how long the call took.
	Duration time.Duration `json:"duration"`
	// Error is the failure detail when Performed is false.
	Error string `json:"error,omitempty"`

	// Marco is the exact source that was compiled and run, redacted for operations
	// carrying credential material.
	//
	// Diagnostic and export material — never the semantic key. The Action is what a
	// repeat re-resolves against; this is the record of how it reached the computer
	// on the day, which is a different question and answers "why did that go wrong".
	Marco string `json:"marco,omitempty"`
	// Capabilities are the act-and-capability pairs the source invoked.
	Capabilities []string `json:"capabilities,omitempty"`
	// CompileStatus and RuntimeStatus are kept APART because they mean opposite
	// things: a compile failure means the desktop was never touched, a runtime
	// failure means an act ran and refused. Collapsing them loses the distinction
	// that decides whether retrying is safe.
	CompileStatus string `json:"compile_status,omitempty"`
	RuntimeStatus string `json:"runtime_status,omitempty"`
}

// WorldStateSummary is a compact digest of a snapshot, small enough to keep in
// history for every action ever taken.
//
// Full snapshots are large — a browser window is thousands of elements — and
// retaining them all would make history unusable within minutes. A summary keeps
// what before/after comparison and explanation actually need: what was in front,
// what had focus, how much was there, and how good the evidence was.
type WorldStateSummary struct {
	Timestamp time.Time `json:"timestamp"`

	Application  string   `json:"application,omitempty"`
	WindowID     WindowID `json:"window_id,omitempty"`
	WindowTitle  string   `json:"window_title,omitempty"`
	WindowBefore *Rect    `json:"window_bounds,omitempty"`
	MonitorID    string   `json:"monitor_id,omitempty"`

	// FocusedElement is what held keyboard focus, and FocusedLabel its name. Focus
	// change is the single most reliable piece of evidence that a click did
	// something, so it is kept explicitly rather than derived later.
	FocusedElement *ElementID `json:"focused_element,omitempty"`
	FocusedLabel   string     `json:"focused_label,omitempty"`

	ElementCount int `json:"element_count"`
	// WindowCount is how many top-level windows existed. A dialog appearing is a
	// window count change, which is how modal interruptions are noticed.
	WindowCount int `json:"window_count"`

	Confidence WorldConfidence `json:"confidence"`

	// Fingerprint is a cheap digest of the element set. Two snapshots with the same
	// fingerprint describe the same screen, which is what makes "nothing changed"
	// answerable without keeping either snapshot.
	Fingerprint string `json:"fingerprint"`
}

// Evidence is one observation supporting or contradicting a verification verdict.
type Evidence struct {
	// Kind names what was checked: "focus_changed", "element_appeared",
	// "window_bounds_changed", "target_gone", "nothing_changed".
	Kind string `json:"kind"`
	// Observed is whether it held.
	Observed bool `json:"observed"`
	// Detail describes what was seen, for the explanation trace.
	Detail string `json:"detail,omitempty"`
	// Weight is how much this contributes to confidence, 0..1. Not all evidence is
	// equal: focus moving to the element that was clicked is near-proof, while the
	// element count merely changing is suggestive.
	Weight float64 `json:"weight"`
	// Source is which source the evidence came from.
	Source ObservationSource `json:"source,omitempty"`
}

// ActionID identifies one executed action.
type ActionID string

// ActionStatus is how an executed action ended.
type ActionStatus string

const (
	ActionPending    ActionStatus = "pending"
	ActionRunning    ActionStatus = "running"
	ActionSucceeded  ActionStatus = "succeeded"
	ActionFailed     ActionStatus = "failed"
	ActionUnverified ActionStatus = "unverified" // performed, but the result couldn't be confirmed
	ActionCancelled  ActionStatus = "cancelled"
	ActionBlocked    ActionStatus = "blocked" // refused by policy
)

// ActionRecord is the durable, structured history of one executed action.
//
// It is written whether the action succeeded or not, because a failure is at least
// as interesting as a success, and it holds enough to support the things later
// milestones need without them being built yet:
//
//   - "do that again" needs the SEMANTIC action and the query that resolved it, so
//     the target can be found afresh rather than replayed at old coordinates;
//   - "undo the last action" needs the before-state and a reversing action;
//   - "repeat that" needs both, plus the verification that says it worked.
//
// None of those commands exist yet. The record is shaped for them now because
// history that was not captured cannot be reconstructed later.
type ActionRecord struct {
	ID ActionID `json:"id"`

	// RequestedIntent is the user's own words.
	RequestedIntent string `json:"requested_intent,omitempty"`

	// Action is the semantic action — what was meant, not where the mouse went.
	Action Action `json:"action"`

	// SettleWait is how the Director waited after acting, when it waited on a
	// CONDITION rather than a duration. Recorded because "waited 350ms" and "waited
	// until the region stopped changing, which took two looks" are different claims,
	// and only the second is evidence of anything.
	SettleWait string `json:"settle_wait,omitempty"`

	// VisualChange summarises what the target REGION did, when a watcher was wired.
	// Evidence about the screen rather than the tree, kept because the two disagree in
	// exactly the case that matters: a page mid-navigation has an unchanged tree, so
	// "nothing happened" and "everything is happening" look identical from the
	// structural side.
	//
	// A short string, not a fingerprint. The action graph is durable and append-only,
	// and a grid of colours stored forever would be a large unreadable artifact about
	// pixels nobody can see again. What survives is the conclusion and why.
	VisualChange string `json:"visual_change,omitempty"`

	// Target is what it resolved to this time, including the candidates that lost
	// and why. Re-resolving later starts from Target.Query, not Target.Point.
	Target ResolvedTarget `json:"target"`

	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	Before WorldStateSummary `json:"before"`
	After  WorldStateSummary `json:"after"`

	Verification VerificationResult `json:"verification"`

	Success       bool   `json:"success"`
	FailureReason string `json:"failure_reason,omitempty"`

	Status ActionStatus `json:"status"`

	// Attempts is how many times this action was executed, including retries. A
	// value above 1 means verification failed and the pipeline re-observed,
	// re-resolved and tried again.
	Attempts int `json:"attempts"`

	// Reversible reports whether UndoAction can put this back, and UndoAction is
	// the action that would. For a window move that is a move to the captured
	// original bounds — which is why the before-summary keeps them.
	Reversible bool   `json:"reversible"`
	UndoAction Action `json:"undo_action,omitempty"`

	// Policy is the decision that permitted or refused this action.
	Policy *PolicyDecision `json:"policy,omitempty"`

	// Execution is what the executor reported doing.
	Execution ExecutionOutcome `json:"execution"`
}

// Succeeded reports whether the action completed and verified.
func (r *ActionRecord) Succeeded() bool { return r != nil && r.Success }

// Duration is how long the action took, or 0 if it never completed.
func (r *ActionRecord) Duration() time.Duration {
	if r == nil || r.CompletedAt == nil {
		return 0
	}
	return r.CompletedAt.Sub(r.StartedAt)
}
