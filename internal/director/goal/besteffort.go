package goal

// Best-effort semantics.
//
//	A best-effort step may be skipped only when its applicability condition is
//	demonstrably false.
//
// The close-without-saving procedure's second step — choose the control that discards
// changes — applies only when a save prompt appeared, and whether it appeared depends on
// whether the document was dirty. The Director cannot know that in advance, and the
// program layer has no branches, so the step is best-effort.
//
// The danger is that "best effort" becomes "ignore whatever happened". These are NOT
// applicability failures and must never be reported as success:
//
//   - the target was found and the action failed;
//   - the target identification was ambiguous;
//   - verification failed;
//   - safety policy refused the action;
//   - capability execution began and then errored.
//
// Each of those is a real failure that happens to occur inside a step whose
// PRECONDITION was satisfied. Only the precondition being demonstrably false — no save
// prompt, nothing to discard — permits a skip.

// Applicability is why a best-effort step did or did not run.
type Applicability string

const (
	// NotApplicable: the condition the step exists for is demonstrably absent. No save
	// prompt appeared, so there was nothing to answer. The step is skipped and the
	// program continues, which is the whole point of best effort.
	NotApplicable Applicability = "not_applicable"
	// ApplicableSucceeded: the condition held and the step did its job.
	ApplicableSucceeded Applicability = "applicable_succeeded"
	// ApplicableFailed: the condition held and the step did NOT do its job. A real
	// failure. Best effort does not launder it.
	ApplicableFailed Applicability = "applicable_failed"
	// ApplicabilityUnknown: whether the condition held could not be established. NOT
	// success and NOT a skip — the honest third answer, and the one that keeps a
	// blind Director from reporting a clean run.
	ApplicabilityUnknown Applicability = "unknown"
)

// Tolerable reports whether this outcome lets the program continue.
//
// Only a demonstrable absence. Unknown is deliberately NOT tolerable: a step whose
// applicability could not be read might have been needed, and continuing past it would
// mean closing a window whose save prompt is still on screen.
func (a Applicability) Tolerable() bool { return a == NotApplicable || a == ApplicableSucceeded }

// Describe renders an applicability for a person.
func (a Applicability) Describe() string {
	switch a {
	case NotApplicable:
		return "did not apply — the condition it exists for was absent"
	case ApplicableSucceeded:
		return "applied and succeeded"
	case ApplicableFailed:
		return "applied and failed"
	case ApplicabilityUnknown:
		return "could not tell whether it applied"
	}
	return string(a)
}

// FailureKind classifies why a step did not do its job.
//
// The point of enumerating them is that only ONE of these is an applicability question.
// Everything else is a failure that a best-effort step must surface.
type FailureKind string

const (
	// FailureTargetAbsent: the control the step needs is not there. The only kind that
	// can mean "not applicable".
	FailureTargetAbsent FailureKind = "target_absent"
	// FailureTargetAmbiguous: several controls could be it. NOT an absence — something
	// is there, and the Director could not tell which. Acting would be a guess and
	// skipping would hide one.
	FailureTargetAmbiguous FailureKind = "target_ambiguous"
	// FailureActionFailed: the target was found and the action did not work.
	FailureActionFailed FailureKind = "action_failed"
	// FailureVerification: it ran and could not be proved to have worked.
	FailureVerification FailureKind = "verification_failed"
	// FailurePolicyRefused: safety refused it.
	FailurePolicyRefused FailureKind = "policy_refused"
	// FailureExecutionErrored: the capability started and errored.
	FailureExecutionErrored FailureKind = "execution_errored"
)

// PermitsSkip reports whether this failure means the step simply did not apply.
//
// Exactly one kind qualifies, and the list is exhaustive on purpose: a new failure kind
// added without thought defaults to NOT permitting a skip, which is the safe direction.
func (f FailureKind) PermitsSkip() bool { return f == FailureTargetAbsent }

// ClassifyBestEffort decides what a best-effort step's failure means.
//
// The single place the rule lives, so the executor cannot accidentally widen it. A
// target that is absent is an applicability answer; anything else is a failure wearing
// a best-effort label.
func ClassifyBestEffort(failure FailureKind, verified bool) Applicability {
	switch {
	case failure == "":
		if verified {
			return ApplicableSucceeded
		}
		// It ran, nothing failed outright, and nothing proved it worked. Unknown rather
		// than success: a step that cannot be verified has not been shown to have done
		// anything.
		return ApplicabilityUnknown
	case failure.PermitsSkip():
		return NotApplicable
	}
	return ApplicableFailed
}
