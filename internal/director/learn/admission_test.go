package learn

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// THE FAST-LEARN ADMISSION RULE, tested where it is decided.
//
// # Why this is an internal test and not a lifecycle one
//
// Because the guard is DEFENSIVE, and that was measured rather than assumed. Deleting it entirely
// — admitting every edge regardless of evidence — left the whole suite green, because the
// coordinator refuses a demonstration that is not `CandidateConsistent` further upstream
// (`evidence_insufficient`) and diverts one with anything `Blocking()` to NeedsAnotherExample. So
// no lifecycle test can reach `admitObserved` with bad evidence to prove it says no.
//
// A guard nothing can reach is still worth keeping — it is the statement of what admission MEANS,
// and the upstream refusals could move — but it must not be mistaken for one the lifecycle tests
// are holding. This holds it, directly, and says so.
//
// Deleting the verdict or Blocking check must fail this.
func TestAdmissionNeedsCleanEvidence(t *testing.T) {
	edge := EdgeReview{Route: observe.RelationshipRef{From: "a", To: "b"}}
	for _, c := range []struct {
		name string
		a    *observe.CandidateAssessment
		want bool
	}{
		{"a clean demonstration", &observe.CandidateAssessment{
			Verdict: observe.CandidateConsistent,
			Reasons: []observe.AssessmentReason{observe.ReasonSingleDemonstration},
		}, true},
		{"no assessment at all", nil, false},
		{"evidence with a gap", &observe.CandidateAssessment{
			Verdict: observe.CandidateInsufficient,
		}, false},
		{"a demonstration with no clear shape", &observe.CandidateAssessment{
			Verdict: observe.CandidateAmbiguous,
		}, false},
		{"nothing Marco can recognise", &observe.CandidateAssessment{
			Verdict: observe.CandidateInvalid,
		}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			c2 := &Coordinator{}
			c2.s.Assessment = c.a
			e := edge
			if got := c2.admissible(&e); got != c.want {
				t.Errorf("admissible = %v, want %v. Fast is not reckless: only a clean "+
					"demonstration may be kept without Marco checking it itself.", got, c.want)
			}
		})
	}
}

// A blocking reason means another EXAMPLE is needed, and trying would not settle it.
//
// The third arm of the split, and the one that keeps Fast Learn honest: an uncertainty a rehearsal
// could confirm is not the same as one only more evidence can close.
func TestABlockingReasonIsNotAdmissible(t *testing.T) {
	// A reason that needs another EXAMPLE rather than an attempt. Chosen by the same two
	// predicates Blocking() folds, so this cannot drift from the rule it is testing.
	blocking := observe.ReasonIncompleteDemonstration
	if !blocking.ResolvableByDemonstration() || blocking.ConfirmableByRehearsal() {
		t.Fatalf("%q is no longer a blocking reason; pick another for this test", blocking)
	}
	c := &Coordinator{}
	c.s.Assessment = &observe.CandidateAssessment{
		Verdict: observe.CandidateConsistent,
		Reasons: []observe.AssessmentReason{blocking},
	}
	e := EdgeReview{Route: observe.RelationshipRef{From: "a", To: "b"}}
	if c.admissible(&e) {
		t.Errorf("a demonstration blocked by %q was admitted. Blocking means another example "+
			"is needed; keeping it anyway is exactly the confidence Fast Learn must not fake.",
			blocking)
	}
}
