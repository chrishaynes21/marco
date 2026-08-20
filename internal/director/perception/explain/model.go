// Package explain is the Director's account of how it came to believe something.
//
// Fusion is the one place in the Director where information is deliberately destroyed.
// Several accounts of a control become one element; the losing accounts leave no trace
// in the result; a confidence of 0.90 arrives as a bare number with no derivation. All
// of that is correct behaviour, and all of it is opaque — an element cannot tell you
// about the observation that lost to it, or why it kept the role it did.
//
// This package is the vocabulary for saying it out loud:
//
//	Why does this element exist?
//	Which observations created it, and which were considered and refused?
//	Which one became primary, and why did it win?
//	Why this role, this label, this confidence?
//	Why did fusion keep these observations SEPARATE?
//
// Two rules govern everything here.
//
// EVERY ANSWER COMES FROM RECORDED EVIDENCE. An explanation is derived from the
// observation cycle and the trace fusion left behind, never reconstructed by guessing
// at what the code probably did. An explanation that could be wrong is worse than none:
// it would be trusted.
//
// EXPLANATION IS A DIAGNOSTICS LAYER AND NOTHING ELSE. No planner, verifier, policy
// engine, replay path or action graph node may consult it, and none can — nothing
// outside perception imports this package, which is enforced by
// internal/director/perception_boundary_test.go. If explanations could change what the
// Director does, they would stop being a description of what it did.
package explain

import (
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ElementExplanation is the complete account of one element.
type ElementExplanation struct {
	ElementID directorapi.ElementID `json:"element_id"`

	// Label and Role are carried so an explanation reads on its own, without the
	// caller having to hold the world open beside it.
	Label string                  `json:"label,omitempty"`
	Role  directorapi.ElementRole `json:"role,omitempty"`

	// PrimaryObservation is the account that won: the strongest source's report, which
	// supplied every field no earlier observation had already claimed.
	PrimaryObservation directorapi.ObservationReference `json:"primary_observation"`

	// Supporting are the other observations fused into this element — corroboration,
	// in the order they joined.
	Supporting []directorapi.ObservationReference `json:"supporting,omitempty"`

	// Rejected are observations that were CONSIDERED for this element and refused.
	// This is the half of fusion that leaves no trace in the result, and the half that
	// explains a duplicate element or a missing one.
	Rejected []RejectedObservation `json:"rejected,omitempty"`

	// MergeSteps are the decisions that produced this element, in the order they were
	// taken.
	MergeSteps []MergeDecision `json:"merge_steps,omitempty"`

	// Fields records which observation supplied each contested field and why — the
	// answer to "why this label" and "why this role".
	Fields []FieldChoice `json:"fields,omitempty"`

	IdentityReason IdentityExplanation   `json:"identity"`
	Confidence     ConfidenceExplanation `json:"confidence"`
}

// RejectedObservation is one observation that could have joined this element and did
// not.
type RejectedObservation struct {
	Observation directorapi.ObservationReference `json:"observation"`
	// Rule is the machine-readable name of what refused it.
	Rule string `json:"rule"`
	// Reason is the sentence a person reads.
	Reason string `json:"reason"`
	// Label is what the rejected observation called itself, when it said anything.
	// Carried because "a different control in the same place" is only diagnosable if
	// you can see which one.
	Label string `json:"label,omitempty"`
	// Score is how close it came, 0..1. A rejection at 0.54 against a threshold of
	// 0.55 is a very different finding from one at 0.
	Score float64 `json:"score"`
}

// MergeOutcome is what a merge decision concluded.
type MergeOutcome string

const (
	// MergeAccepted: the observation joined this element.
	MergeAccepted MergeOutcome = "accepted"
	// MergeRejected: the observation was considered and refused.
	MergeRejected MergeOutcome = "rejected"
	// MergeSeeded: the observation started this element, having nothing to join.
	MergeSeeded MergeOutcome = "seeded"
)

// MergeDecision is one decision that contributed to belief.
//
// Deliberately not a log of everything the clusterer touched. Two observations at
// opposite ends of the screen are not a decision — they are the absence of one, and
// recording every such pair would bury the handful that matter under thousands that do
// not. What is recorded is: every merge that happened, every seeding, and every
// refusal of an observation that was spatially plausible.
type MergeDecision struct {
	// Rule names the rule that settled it, machine-readable and stable.
	Rule    string       `json:"rule"`
	Outcome MergeOutcome `json:"outcome"`
	Reason  string       `json:"reason"`

	// Against is the other observation, absent for a seeding.
	Against *directorapi.ObservationReference `json:"against,omitempty"`
	// Score is the pair's match score where one was computed.
	Score float64 `json:"score,omitempty"`
}

// FieldChoice records which observation supplied one field of the element.
//
// The cluster is ordered strongest-source-first, so "the first observation that
// supplied a field" is exactly "the most authoritative source that knew it". This is
// where that becomes visible rather than implied.
type FieldChoice struct {
	Field string `json:"field"`
	Value string `json:"value"`
	// From is the observation that supplied it.
	From directorapi.ObservationReference `json:"from"`
	// Reason explains why that observation and not another.
	Reason string `json:"reason"`
	// Overruled lists the observations that offered a different value and lost.
	Overruled []string `json:"overruled,omitempty"`
}

// IdentityExplanation is how an element came to have the id it has.
//
// The quiet load-bearing question. "Click that again" works only because the button
// that exists now is recognisably the button that existed a moment ago, and when that
// fails it fails SILENTLY — a re-resolved target still looks like a successful lookup.
// This is how the failure becomes visible.
type IdentityExplanation struct {
	// Stable is whether this identity is expected to survive the UI being rebuilt —
	// whether the element has a unique authored id or a unique label, rather than only
	// a runtime handle that is reissued on recreation.
	Stable bool `json:"stable"`
	// MatchedPrevious is whether the identity was carried forward from the previous
	// cycle rather than minted fresh.
	MatchedPrevious bool `json:"matched_previous"`
	// Rule names the tier that decided it: "native_id", "automation_id", "structural",
	// or "new".
	Rule string `json:"rule"`
	// Reason is the sentence a person reads.
	Reason string `json:"reason"`
	// PreviousElement is the identity inherited, when one was.
	PreviousElement *directorapi.ElementID `json:"previous_element,omitempty"`
	// Score is the structural match score, when the structural tier decided it.
	Score float64 `json:"score,omitempty"`
}

// ConfidenceExplanation is how a confidence value was arrived at.
//
// Replaces an opaque number with its derivation. A 0.90 that came from one
// accessibility observation and a 0.90 that came from three weak sources agreeing are
// different findings, and a threshold comparison downstream cannot tell them apart.
type ConfidenceExplanation struct {
	// Base is the strongest source's contribution, before corroboration.
	Base float64 `json:"base"`
	// Contributions are what moved it from Base to Total, in order.
	Contributions []ConfidenceContribution `json:"contributions,omitempty"`
	// Total is the final value, and must equal Base plus the deltas.
	Total float64 `json:"total"`
}

// ConfidenceContribution is one term of the confidence calculation.
type ConfidenceContribution struct {
	Source string  `json:"source"`
	Delta  float64 `json:"delta"`
	Reason string  `json:"reason"`
}

// Add appends a contribution and advances the total.
func (c *ConfidenceExplanation) Add(source string, delta float64, reason string) {
	c.Contributions = append(c.Contributions, ConfidenceContribution{
		Source: source, Delta: delta, Reason: reason,
	})
	c.Total += delta
}

// Consistent reports whether the stated total matches its own derivation.
//
// Worth checking rather than assuming: an explanation whose arithmetic does not add up
// is actively misleading, and the failure mode is silent. The tolerance is for float
// accumulation, not for slack in the model.
func (c ConfidenceExplanation) Consistent() bool {
	sum := c.Base
	for _, contrib := range c.Contributions {
		sum += contrib.Delta
	}
	d := sum - c.Total
	return d < 1e-9 && d > -1e-9
}

// CycleExplanation is every element of one observation cycle, explained.
type CycleExplanation struct {
	Cycle string `json:"cycle"`
	// Elements are in a stable order — reading order within a window — so two runs
	// over the same cycle produce byte-identical output.
	Elements []ElementExplanation `json:"elements"`
	// Unexplained counts elements the trace could not account for. Should always be
	// zero; a non-zero value means the explanation layer has drifted from fusion, and
	// saying so is better than quietly omitting them.
	Unexplained int `json:"unexplained,omitempty"`
}

// Find returns the explanation for one element.
func (c CycleExplanation) Find(id directorapi.ElementID) (ElementExplanation, bool) {
	for _, e := range c.Elements {
		if e.ElementID == id {
			return e, true
		}
	}
	return ElementExplanation{}, false
}
