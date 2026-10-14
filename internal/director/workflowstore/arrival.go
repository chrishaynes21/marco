package workflowstore

// Did the screen become the one the step was going to?
//
// # Why this is called destination established and not action verified
//
// [[Experiment-038-how-far-a-reloaded-workflow-gets]] found that `workflow.Verify` cannot answer
// this for a reloaded workflow: it compares screen IDENTITIES, and a saved destination is an
// artifact LABEL that nothing has bound, because arriving is what would bind it.
//
// The fix is to ask the establishment path the same question it already answers — is this screen
// the saved `screen_c` — one observation later. That is a real and useful answer, and it is
// **not** proof that an action worked. The owner drew the line and it is the whole of this file:
//
//	Keep action verified separate and unavailable unless independent post-action evidence
//	supports that stronger claim.
//
// # The circularity this exists to label
//
// A destination can be established two ways, and they are not equally good evidence HERE even
// though they are elsewhere:
//
//   - **confirmed** — somebody looked at the screen and said it is `screen_c`. Independent of
//     the workflow entirely: the person did not consult it.
//   - **inferred** — the workflow demonstrates a step into `screen_c` and the arrangement
//     agrees. The expectation came from the same workflow whose step is being checked, so the
//     only independent part is the arrangement agreement — and
//     [[Experiment-036-recognising-a-screen-you-were-shown]] measured that arrangement agreement
//     nominates an absent screen just as confidently as a present one.
//
// So an inferred arrival is weaker than a confirmed one, and `Independent` is the field that
// says so. A report that collapsed the two would be letting a workflow confirm its own
// expectations.

import "fmt"

// Arrival is what the observation AFTER an action boundary says about a step's destination.
type Arrival struct {
	Step int
	// Destination is the step's saved destination label.
	Destination string
	// Ground is how it came to be established, or empty when it did not.
	Ground Ground
	// Depth is how many inferences from a person's answer, for an inferred arrival.
	Depth   int
	Because string
}

// Established reports whether the saved destination is grounded in this observation.
func (a Arrival) Established() bool { return a.Ground != "" }

// Independent reports whether the evidence came from outside the workflow being checked.
//
// Only a person's confirmation does. An inference consults the workflow's own demonstrated
// transition to decide what to expect, so it cannot corroborate that workflow's step without
// arguing in a circle.
func (a Arrival) Independent() bool { return a.Ground == GroundConfirmed }

// Verified reports whether the ACTION was independently verified, and why not.
//
// It is always false, and deliberately a method rather than a field somebody could set. There is
// no post-action evidence in this system at all: nothing has been executed, and even once
// something is, arriving where you meant to is not the same as the press having been the cause.
// The day that changes, this returns something else and every caller already prints the reason.
func (a Arrival) Verified() (bool, string) {
	return false, "nothing was executed, so there is no post-action evidence; and " +
		"arriving where a step expected is not by itself proof that the step's " +
		"action caused it"
}

// DestinationAt asks what the current observation says about a step's destination.
//
// It establishes NOTHING. The caller has already run the establishment path over this
// observation — confirming or inferring as the evidence allowed — and this reads the result.
// Thresholds, expiry and the three outcomes are exactly as they were.
func (a Artifact) DestinationAt(step int, s *SessionBinding, now Now) Arrival {
	r := Arrival{Step: step}
	if step < 0 || step >= len(a.Steps) {
		r.Because = fmt.Sprintf("step %d is not in a workflow of %d", step, len(a.Steps))
		return r
	}
	st := a.Steps[step]
	r.Destination = st.To
	if st.To == "" {
		r.Because = "the step has no recorded destination, so there is nothing an " +
			"arrival could be checked against"
		return r
	}
	bound, ok := s.Established(st.To, now.Epoch, now.Generation)
	switch {
	case !ok:
		r.Because = fmt.Sprintf("%s is not established in this observation; the screen "+
			"standing is %s and nothing has grounded it as the destination",
			st.To, now.Screen)
		return r
	case bound.Screen != now.Screen:
		r.Because = fmt.Sprintf("%s is established as %s and the screen standing is %s",
			st.To, bound.Screen, now.Screen)
		return r
	}
	r.Ground, r.Depth = bound.Ground, bound.Depth
	if r.Independent() {
		r.Because = fmt.Sprintf("%s was confirmed as %s by %s, who did not consult the "+
			"workflow to decide", st.To, bound.Screen, bound.By)
		return r
	}
	r.Because = fmt.Sprintf("%s was inferred as %s at depth %d — the workflow said to "+
		"expect it and the arrangement agreed. The expectation came from the same "+
		"workflow this step belongs to, so this is weaker than somebody looking",
		st.To, bound.Screen, bound.Depth)
	return r
}
