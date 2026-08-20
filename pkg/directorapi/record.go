package directorapi

import "time"

// ResolvedTarget is a reference the resolver has settled on, with the reasoning that
// settled it. Recorded on every action so a selection can be defended and audited
// long after the world has moved on.
type ResolvedTarget struct {
	// ElementID is the chosen element, when the target is an element.
	ElementID ElementID `json:"element_id,omitempty"`
	// WindowID is the chosen window, when the target is a window.
	WindowID WindowID `json:"window_id,omitempty"`
	// Point is the resolved click point at the time of resolution. Kept as EVIDENCE
	// of where the action went, never as the thing a repeat replays — a repeat
	// re-resolves unless the reference was coordinate-locked.
	Point Point `json:"point"`

	// Role and Label describe what was chosen, so history stays readable after the
	// element itself is long gone.
	Role  ElementRole `json:"role,omitempty"`
	Label string      `json:"label,omitempty"`

	// NativeID is the id the SOURCE knows this element by (a UIA RuntimeId, a DOM
	// id). Some actions are performed by asking the source rather than by input —
	// focusing is one, since no mouse event can move focus without also activating —
	// and the source knows the element only by its own name for it.
	NativeID string `json:"native_id,omitempty"`

	Confidence float64 `json:"confidence"`
	// Explanation is why this candidate won, in one sentence.
	Explanation string `json:"explanation,omitempty"`
	// Alternatives are the runners-up with their scores — what makes "why not the
	// other one?" answerable, and what a clarification prompt offers the user.
	Alternatives []TargetCandidate `json:"alternatives,omitempty"`
	// Query is the query that produced this target, retained so a repeat can
	// re-resolve semantically rather than replaying a coordinate.
	Query *ElementQuery `json:"query,omitempty"`
}

// TargetCandidate is one considered candidate — chosen or rejected — with the score
// breakdown that ranked it. The breakdown is the point: a bare score is not an
// explanation.
type TargetCandidate struct {
	ElementID ElementID   `json:"element_id"`
	Role      ElementRole `json:"role,omitempty"`
	Label     string      `json:"label,omitempty"`
	Score     float64     `json:"score"`
	// Signals is the per-signal contribution ("text": 1.0, "inert": 0.35), mirroring
	// the way Marco's anchor scorer already logs its candidate breakdowns.
	Signals map[string]float64  `json:"signals,omitempty"`
	Sources []ObservationSource `json:"sources,omitempty"`
	// Rejected, when set, is why this candidate was excluded outright rather than
	// merely out-scored ("disabled", "not an interactive control", "not visible").
	Rejected string `json:"rejected,omitempty"`
}

// VerificationResult is the verdict on whether an action had its intended effect.
//
// Confidence matters as much as Success: "focus moved to the element I clicked" is a
// far stronger yes than "the number of elements changed", and a caller deciding
// whether to retry should treat them differently.
type VerificationResult struct {
	Success    bool    `json:"success"`
	Confidence float64 `json:"confidence"`
	// Evidence is what was checked and what was found — several independent checks
	// where possible, since any single one can be fooled.
	Evidence []Evidence `json:"evidence,omitempty"`
	// Reason explains the verdict in the user's terms.
	Reason string `json:"reason,omitempty"`
	// Inconclusive marks the case where nothing could be checked at all — genuinely
	// different from a failure, and the reason ActionUnverified exists. Reporting
	// "I clicked it but could not confirm" beats claiming either outcome.
	Inconclusive bool `json:"inconclusive,omitempty"`
}

// ExecutionResult is the outcome of running a plan.
//
// It is deliberately a PARTIAL result: a plan that failed at step 3 returns the
// records for steps 1–2, the failure, and what the Director understood about it.
// "Return partial, inspectable failures rather than hiding errors" is a rule, and a
// bare error would throw away the evidence needed to recover or explain.
type ExecutionResult struct {
	PlanID PlanID `json:"plan_id"`
	// Records is one record per executed step, in order.
	Records []ActionRecord `json:"records,omitempty"`
	// Completed is true when every non-optional step ran and the plan's success
	// conditions held.
	Completed bool `json:"completed"`
	// StoppedAtStep is the index of the step that stopped the plan, -1 if none did.
	StoppedAtStep int `json:"stopped_at_step"`
	// Verification is the plan-level verdict.
	Verification *VerificationResult `json:"verification,omitempty"`
	// Recovery lists recovery attempts that were made, in order.
	Recovery []RecoveryAttempt `json:"recovery,omitempty"`
	// Cancelled is true when the user or a caller stopped the run.
	Cancelled bool   `json:"cancelled,omitempty"`
	Error     string `json:"error,omitempty"`
}

// RecoveryStrategy names one rung of the recovery ladder.
//
// Only RecoveryReobserve is implemented so far, on purpose. Repeating an action that
// just failed is the least likely thing to work and the most likely to do damage, so
// the first and only recovery is to look again and re-resolve — the target usually
// moved rather than vanished. The rest are named because the ladder's ORDER is a
// design decision worth fixing early, even before the rungs exist.
type RecoveryStrategy string

const (
	// RecoveryReobserve re-observes and re-resolves the target, then retries once.
	RecoveryReobserve RecoveryStrategy = "reobserve"
	// RecoveryEquivalent looks for a semantically equivalent element.
	RecoveryEquivalent RecoveryStrategy = "equivalent"
	// RecoveryDismissDialog handles an unexpected dialog that intercepted the action.
	RecoveryDismissDialog RecoveryStrategy = "dismiss_dialog"
	// RecoveryRestoreFocus re-focuses the window that lost focus mid-plan.
	RecoveryRestoreFocus RecoveryStrategy = "restore_focus"
	// RecoveryScrollIntoView scrolls an offscreen target into view.
	RecoveryScrollIntoView RecoveryStrategy = "scroll_into_view"
	// RecoveryAlternateMethod tries another way to do the same thing.
	RecoveryAlternateMethod RecoveryStrategy = "alternate_method"
	// RecoverySkill hands off to a skill's own recovery handler.
	RecoverySkill RecoveryStrategy = "skill"
	// RecoveryAskUser gives up and asks. The last rung, and a legitimate outcome.
	RecoveryAskUser RecoveryStrategy = "ask_user"
)

// RecoveryAttempt records one rung that was tried and what came of it.
type RecoveryAttempt struct {
	Strategy  RecoveryStrategy `json:"strategy"`
	StepIndex int              `json:"step_index"`
	Succeeded bool             `json:"succeeded"`
	Detail    string           `json:"detail,omitempty"`
	At        time.Time        `json:"at"`
}
