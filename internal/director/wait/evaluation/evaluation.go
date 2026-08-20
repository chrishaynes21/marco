// Package evaluation is the vocabulary for answering a condition.
//
// The distinction this package exists to protect is between UNSATISFIED and UNKNOWN.
//
//	Unsatisfied — the Director looked, and the thing is not so.
//	Unknown     — the Director could not look, so it does not know.
//
// Collapsing them is the mistake that makes waiting dangerous. "The Save button is not
// enabled" and "I cannot see into this application" are opposite findings: the first is
// a reason to keep waiting, and the second is a reason to stop and say so. A wait that
// treated blindness as a negative would poll a window it cannot read until it timed
// out, and then report that the condition never came true — which is a confident claim
// about something it never observed.
package evaluation

import (
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// State is the answer to a condition.
type State string

const (
	// Satisfied: the world provides sufficient evidence that the condition holds.
	Satisfied State = "satisfied"
	// Unsatisfied: the Director observed, and the condition does not hold. A reason to
	// keep waiting.
	Unsatisfied State = "unsatisfied"
	// Unknown: the Director could not observe what the condition asks about. NEVER a
	// synonym for false — see the package comment.
	Unknown State = "unknown"
	// Cancelled: the user stopped the wait. Distinct from TimedOut, always: a wait
	// that reported a cancellation as a timeout would tell the user their interface
	// was slow when in fact they had asked it to stop.
	Cancelled State = "cancelled"
	// TimedOut: the bound elapsed without the condition being satisfied. Reports what
	// the LAST evaluation was, because "it timed out while unknown" and "it timed out
	// while unsatisfied" are different diagnoses.
	TimedOut State = "timed_out"
)

// Terminal reports whether a wait should stop on this state.
func (s State) Terminal() bool {
	return s == Satisfied || s == Cancelled || s == TimedOut
}

// EvidenceReference points at the observation or element that answered the condition.
//
// Conditions are answered from EVIDENCE, and an answer that cannot say what it looked
// at is not checkable. This is what makes "why did the wait finish" a question with a
// structured answer rather than a shrug.
type EvidenceReference struct {
	// Kind names what was consulted: "element", "window", "text", "region",
	// "verification".
	Kind string `json:"kind"`
	// Element or Window, when the evidence is one of those.
	Element directorapi.ElementID `json:"element,omitempty"`
	Window  directorapi.WindowID  `json:"window,omitempty"`
	// Observation is the raw evidence, where the answer traces to one.
	Observation directorapi.ObservationID `json:"observation,omitempty"`
	// Detail is what was actually seen.
	Detail string `json:"detail,omitempty"`
}

// Result is one evaluation of one condition.
type Result struct {
	State State `json:"state"`
	// Confidence is how sure the Director is of this answer, 0..1.
	//
	// Separate from the state because an answer can be right and weak: a condition
	// satisfied by a visual inference is satisfied, and less certainly so than one
	// satisfied by a structural fact. A caller deciding whether to act on a wait's
	// result should be able to tell.
	Confidence float64             `json:"confidence"`
	Evidence   []EvidenceReference `json:"evidence,omitempty"`
	// Explanation is the sentence a person reads.
	Explanation string `json:"explanation"`
}

// Satisfy builds a satisfied result.
func Satisfy(confidence float64, explanation string, ev ...EvidenceReference) Result {
	return Result{State: Satisfied, Confidence: confidence, Explanation: explanation, Evidence: ev}
}

// Deny builds an unsatisfied result — the Director looked, and it is not so.
func Deny(confidence float64, explanation string, ev ...EvidenceReference) Result {
	return Result{State: Unsatisfied, Confidence: confidence, Explanation: explanation, Evidence: ev}
}

// Unknowable builds an unknown result — the Director could not look.
//
// The confidence is deliberately absent rather than zero-with-a-state: an unknown
// answer has no confidence to report, because there is no answer to be confident about.
func Unknowable(explanation string, ev ...EvidenceReference) Result {
	return Result{State: Unknown, Explanation: explanation, Evidence: ev}
}

// Evaluation is one evaluation with the moment it was made.
type Evaluation struct {
	Timestamp time.Time `json:"timestamp"`
	Result    Result    `json:"result"`
	// Cycle is the observation cycle the answer came from, so an evaluation can be
	// traced back to the evidence it read.
	Cycle string `json:"cycle,omitempty"`
}

// String renders an evaluation for a diagnostic line.
func (e Evaluation) String() string {
	return fmt.Sprintf("%-12s %.2f  %s", e.Result.State, e.Result.Confidence, e.Result.Explanation)
}
