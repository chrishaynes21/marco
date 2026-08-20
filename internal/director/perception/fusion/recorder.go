package fusion

import (
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/explain"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The recorder is how clustering says what it did.
//
// It hangs off the SAME code path that makes the decisions rather than reimplementing
// them, which is the only way an explanation can be trusted. A separate "work out why
// these did not merge" routine would be a second copy of the merge rules, and the first
// time the two disagreed the explanation would be confidently wrong — a worse outcome
// than having no explanation at all.
//
// Every method is nil-safe. That is what lets the hot path pass nil and pay nothing:
// no branch at each call site, no partially-populated structures, and no risk that the
// uninstrumented and instrumented runs diverge because one of them forgot a guard.

// recorder captures merge decisions per cluster.
type recorder struct {
	// merges and rejects are indexed by cluster position, which is stable within one
	// clustering run because clusters are only ever appended to.
	merges  map[int][]explain.MergeDecision
	rejects map[int][]explain.RejectedObservation
	fields  map[int][]explain.FieldChoice
	// confidence is the derivation of each cluster's confidence.
	confidence map[int]explain.ConfidenceExplanation
}

func newRecorder() *recorder {
	return &recorder{
		merges:     map[int][]explain.MergeDecision{},
		rejects:    map[int][]explain.RejectedObservation{},
		fields:     map[int][]explain.FieldChoice{},
		confidence: map[int]explain.ConfidenceExplanation{},
	}
}

// seeded records an observation that started a cluster, having nothing to join.
func (r *recorder) seeded(ci int, o directorapi.Observation, cycle observation.CycleID) {
	if r == nil {
		return
	}
	r.merges[ci] = append(r.merges[ci], explain.MergeDecision{
		Rule:    "seeded",
		Outcome: explain.MergeSeeded,
		Reason: "no existing element matched, so this observation became one — " +
			"splitting an object in two costs a duplicate candidate, where merging two " +
			"objects creates an element that is neither",
	})
	_ = o
	_ = cycle
}

// merged records an observation joining a cluster.
func (r *recorder) merged(ci int, o directorapi.Observation, cycle observation.CycleID, v clusterVerdict) {
	if r == nil {
		return
	}
	against := v.against.Reference(string(cycle))
	r.merges[ci] = append(r.merges[ci], explain.MergeDecision{
		Rule:    v.rule,
		Outcome: explain.MergeAccepted,
		Reason:  v.reason,
		Against: &against,
		Score:   v.score,
	})
	_ = o
}

// refusedElsewhere records the clusters that considered o and did not take it.
//
// Only the near misses. A refusal between two observations with no spatial relationship
// is not a decision — it is the absence of one — and with a few hundred observations
// there are tens of thousands of those. Recording them all would bury the handful that
// explain a duplicate element under noise nobody can read.
func (r *recorder) refusedElsewhere(o directorapi.Observation, cycle observation.CycleID,
	verdicts []clusterVerdict, taken int) {

	if r == nil {
		return
	}
	ref := o.Reference(string(cycle))
	for ci, v := range verdicts {
		if ci == taken || !v.plausible {
			continue
		}
		r.rejects[ci] = append(r.rejects[ci], explain.RejectedObservation{
			Observation: ref,
			Rule:        v.rule,
			Reason:      v.reason,
			Label:       firstText(o),
			Score:       v.score,
		})
	}
}

// field records which observation supplied one field of an element.
func (r *recorder) field(ci int, choice explain.FieldChoice) {
	if r == nil {
		return
	}
	r.fields[ci] = append(r.fields[ci], choice)
}

// overrule notes that a field's value was contested and held.
func (r *recorder) overrule(ci int, field, by string) {
	if r == nil {
		return
	}
	for i := range r.fields[ci] {
		if r.fields[ci][i].Field == field {
			r.fields[ci][i].Overruled = append(r.fields[ci][i].Overruled, by)
			return
		}
	}
}

// setConfidence records a cluster's confidence derivation.
func (r *recorder) setConfidence(ci int, c explain.ConfidenceExplanation) {
	if r == nil {
		return
	}
	r.confidence[ci] = c
}

// ── text fusion ───────────────────────────────────────────────────────────────

// text records what became of one text observation, against the element it concerned.
//
// Recorded on the ELEMENT's slot rather than in a flat list because the question a
// reader has is "why is this button called that", and answering it should not require
// scanning every text decision in the cycle.
func (r *recorder) text(ci int, d TextDecision, outcome explain.MergeOutcome, kind string) {
	if r == nil {
		return
	}
	against := d.Observation
	r.merges[ci] = append(r.merges[ci], explain.MergeDecision{
		Rule:    d.Rule,
		Outcome: outcome,
		Reason:  d.Reason,
		Against: &against,
		Score:   d.Containment,
	})
	if kind != "" {
		r.field(ci, explain.FieldChoice{
			Field: "label", Value: d.Text, From: against, Reason: kind,
		})
	}
}

func (r *recorder) textFilled(ci int, d TextDecision) {
	r.text(ci, d, explain.MergeAccepted,
		"filled from contained OCR text — the element had no structural label. "+
			"Role and actionability still come from accessibility alone")
}

func (r *recorder) textReinforced(ci int, d TextDecision) {
	r.text(ci, d, explain.MergeAccepted, "")
}

func (r *recorder) textConflict(ci int, d TextDecision) {
	if r == nil {
		return
	}
	r.text(ci, d, explain.MergeRejected, "")
	// Also recorded as a refusal, so it appears under "considered and refused" where a
	// reader looking for a disagreement will actually look.
	r.rejects[ci] = append(r.rejects[ci], explain.RejectedObservation{
		Observation: d.Observation,
		Rule:        d.Rule,
		Reason:      d.Reason,
		Label:       d.Text,
		Score:       d.Containment,
	})
}

// visual records what became of one visual observation.
func (r *recorder) visual(ci int, d VisualDecision, outcome explain.MergeOutcome) {
	if r == nil {
		return
	}
	against := d.Observation
	r.merges[ci] = append(r.merges[ci], explain.MergeDecision{
		Rule: d.Rule, Outcome: outcome, Reason: d.Reason, Against: &against,
	})
	if outcome == explain.MergeRejected {
		r.rejects[ci] = append(r.rejects[ci], explain.RejectedObservation{
			Observation: d.Observation,
			Rule:        d.Rule,
			Reason:      d.Reason,
			Label:       string(d.VisualKind),
		})
	}
	if outcome == explain.MergeAccepted && d.Flag != "" {
		r.field(ci, explain.FieldChoice{
			Field: "state:" + d.Flag,
			Value: string(d.VisualKind),
			From:  against,
			Reason: "appearance " + string(d.Outcome) +
				" — the role and everything that follows from it still come from accessibility",
		})
	}
}
