package fusion

import (
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Report is what fusion did with one cycle's evidence.
//
// Diagnostics, not control flow: nothing plans differently because of a report. It
// exists because fusion is the one place in the Director where information is
// deliberately DESTROYED — several accounts of a control become one element, and the
// losing accounts leave no trace in the result. That is the correct behaviour and it
// is also, without this, completely opaque. A report is how "why is there one Save
// button when two sources reported one each" and "why are there two" become
// answerable questions.
type Report struct {
	Cycle observation.CycleID `json:"cycle,omitempty"`

	// ObservationCount is the evidence that went in — all kinds, not just elements.
	ObservationCount int `json:"observation_count"`
	// ElementCount is the belief that came out.
	ElementCount int `json:"element_count"`
	// Merged is how many observations were absorbed into an element that another
	// observation had already established. With one source it is zero by definition:
	// a source enumerating the desktop once cannot corroborate itself. It becomes the
	// headline number the day a second source exists — a merged count of zero across
	// a thousand OCR observations would mean OCR was seeing a different screen.
	Merged int `json:"merged"`
	// Rejected is evidence that produced no element and reinforced none: it was
	// neither believed nor used. Not an error — a text observation with nothing to
	// attach to is exactly what a page of body text looks like — but a large rejected
	// count against a small element count means a source is not landing.
	Rejected int `json:"rejected"`

	Conflicts []Conflict `json:"conflicts,omitempty"`

	// ByKind and BySource break the input down, so a missing provider shows up as a
	// zero rather than as an unexplained shortfall in the total.
	ByKind   map[observation.Kind]int   `json:"by_kind,omitempty"`
	BySource map[observation.Source]int `json:"by_source,omitempty"`

	// Degraded lists the sources that were expected and did not report.
	Degraded []directorapi.SourceFailure `json:"degraded,omitempty"`

	// Text is what became of the text evidence, if any arrived. Reported separately
	// from Merged because text NEVER creates an element and merging counts do not
	// describe it: the interesting numbers are how many labels it filled and how many
	// it was refused for.
	Text TextSummary `json:"text"`

	// Visual is what became of the appearance evidence, if any arrived. Like Text it is
	// reported separately because it never creates an element — the interesting numbers
	// are the states it filled and the conflicts it lost.
	Visual VisualSummary `json:"visual"`

	// Provenance accounts for evidence the target guard refused.
	//
	// Reported even when nothing was refused, because "the guard ran and admitted
	// everything" and "the guard did not run" are different facts about a cycle and only
	// one of them is reassuring.
	Provenance ProvenanceSummary `json:"provenance"`

	// Duration is how long fusion itself took, excluding observation. Watched because
	// this milestone promised to add no desktop work: if fusion time grows while
	// observation time does not, the cost is in here.
	Duration time.Duration `json:"duration"`
}

// ProvenanceSummary is what the target guard did with one cycle.
//
// # Why this is in the report at all
//
// Because a guard that silently drops evidence is indistinguishable from a desktop with
// nothing on it. The whole value of refusing stale evidence comes from being able to say
// afterwards that it was refused and why — otherwise the Director has merely exchanged a
// confident wrong answer for a confident empty one, which is not obviously an improvement.
type ProvenanceSummary struct {
	// Targeted is whether this cycle pinned a generation. False means the guard did not
	// apply, which is normal for an ordinary command.
	Targeted bool `json:"targeted"`
	// Generation is the window generation the cycle expected its evidence to describe.
	Generation uint64 `json:"generation,omitempty"`

	// Admitted and Refused count OBSERVATIONS, not providers: one refused provider can
	// take two thousand observations with it, and the provider count would make that look
	// like a rounding error.
	Admitted int `json:"admitted"`
	Refused  int `json:"refused"`

	// Providers names each refusal, so the answer to "why is the world empty" is one line
	// of a report rather than an afternoon.
	Providers []ProvenanceRefusal `json:"providers,omitempty"`
}

// ProvenanceRefusal is one provider's evidence, refused.
type ProvenanceRefusal struct {
	Source observation.Source        `json:"source"`
	State  observation.ProviderState `json:"state"`
	Reason string                    `json:"reason,omitempty"`
	// Expected and Observed are kept apart in the report exactly as they are kept apart
	// in the outcome. A reader has to be able to see BOTH numbers to believe the verdict.
	Expected directorapi.TargetProvenance `json:"expected,omitzero"`
	Observed directorapi.TargetProvenance `json:"observed,omitzero"`
	// Lost is how many observations this refusal cost.
	Lost int `json:"lost"`
}

// summariseProvenance accounts for the guard's decisions on one cycle.
func summariseProvenance(cycle observation.Cycle,
	refused []observation.ProviderOutcome, admitted int) ProvenanceSummary {

	s := ProvenanceSummary{Targeted: cycle.Targeted(), Admitted: admitted}
	if s.Targeted {
		s.Generation = cycle.Request.Target.WindowGeneration
	}
	for _, o := range refused {
		s.Refused += len(o.Observations)
		s.Providers = append(s.Providers, ProvenanceRefusal{
			Source:   o.Source,
			State:    o.State,
			Reason:   refusalReason(o),
			Expected: o.ExpectedTarget,
			Observed: o.ObservedTarget,
			Lost:     len(o.Observations),
		})
	}
	return s
}

// Conflict is two sources disagreeing about one field of one object.
//
// Not an error. Sources disagree legitimately and constantly: a toolkit reports
// "&Save" where OCR reads "Save", accessibility calls something a button that a
// detector calls an icon. The ladder resolves it, and this records that it did — which
// is the difference between a resolution you can audit and one you have to take on
// faith.
type Conflict struct {
	// Field is what was disputed: "role", "label", "value".
	Field string `json:"field"`
	// Element is the element the dispute was about, filled in after identity has been
	// assigned — conflicts are detected before the element has an id.
	Element directorapi.ElementID `json:"element,omitempty"`

	Winner directorapi.ObservationReference `json:"winner"`
	Loser  directorapi.ObservationReference `json:"loser"`

	WinnerValue string `json:"winner_value,omitempty"`
	LoserValue  string `json:"loser_value,omitempty"`
}

// ── merge candidacy ───────────────────────────────────────────────────────────

// MergeCandidate is one piece of evidence weighed against an element that already
// exists.
//
// The type names the decision fusion will have to make the moment a second source
// exists: does this observation REINFORCE the belief already held, or is it about a
// different object? Today clustering answers a narrower version of that question —
// observation against observation, within a single cycle — because with one source
// there is never an existing element to reinforce.
//
// The criteria the fuller version will weigh, in roughly descending strength:
//
//   - SHARED WINDOW. A hard gate, not a signal. Two windows overlap on screen and
//     geometry alone would merge a dialog's OK button with whatever sits behind it.
//   - MATCHING ROLE. Compatible rather than equal: OCR calls everything text and a
//     detector may see a button where accessibility knows a menu item.
//   - BOUNDS OVERLAP. The one signal every source produces. Coincident boxes score on
//     IoU; a contained box — an OCR word inside its button — scores on coverage, which
//     IoU rates near zero.
//   - MATCHING LABEL. Strong confirmation on top of geometry, and a genuine conflict
//     is disqualifying: two different words in one place are two controls.
//   - SHARED PARENT. Cheap and decisive where both sources expose a tree. OCR and
//     vision never will, which is why it cannot be relied on.
//   - OBSERVATION AGE. Evidence from an older cycle describes a screen that has since
//     moved. A slow provider's box should not be merged into a fresh element as though
//     it were seen at the same instant.
//   - PROVIDER RELIABILITY. Not the ladder, which is a fixed order over sources, but
//     the OBSERVED behaviour of this provider on this application — the thing that
//     would let the Director learn that a particular app's accessibility labels are
//     unreliable.
//
// Deliberately not implemented beyond what one source needs. Speculative merge
// heuristics tuned against no second source would be tuned against nothing.
type MergeCandidate struct {
	Observation observation.Observation
	// ExistingElement is the belief this evidence might reinforce. Nil when the
	// evidence is being weighed against other evidence rather than against a belief.
	ExistingElement *directorapi.Element
}

// Score rates how likely the candidate is to be about the existing element, 0..1.
//
// Zero when there is no existing element to reinforce, which is every call today. It
// is here so the call site exists and is exercised, rather than being invented later
// alongside the heuristics it feeds.
func (m MergeCandidate) Score() float64 {
	if m.ExistingElement == nil || m.Observation == nil {
		return 0
	}
	el, ok := m.Observation.(observation.Element)
	if !ok {
		// Only element evidence can be weighed this way today. Text and vision
		// evidence needs the reinforcement path, which does not exist yet — and
		// scoring it with the element rules would merge a read word into the control
		// behind it on geometry alone.
		return 0
	}
	against := directorapi.Observation{
		Source:   directorapi.SourceAccessibility,
		WindowID: m.ExistingElement.WindowID,
		Role:     m.ExistingElement.Role,
		Label:    m.ExistingElement.Label,
		Bounds:   m.ExistingElement.Bounds,
	}
	if el.Raw.Source == against.Source {
		// The same-source rule: one enumeration of the desktop cannot corroborate
		// itself, and merging two of its nodes would collapse a list into a row.
		return 0
	}
	return pairScore(against, el.Raw)
}

// TextSummary is what one cycle's text evidence achieved.
//
// The shape is chosen to make the safety property READABLE rather than merely true. A
// reader should be able to see at a glance that text filled some labels and created no
// controls — "standalone" is text that stayed evidence, and there is deliberately no
// counter for "text that became an element", because that number cannot be anything
// but zero.
type TextSummary struct {
	Observations int `json:"observations"`
	FilledLabel  int `json:"filled_label"`
	Reinforced   int `json:"reinforced"`
	Supporting   int `json:"supporting"`
	// Standalone is text with no structural element under it. It stays in the
	// observation graph and becomes no part of belief.
	Standalone        int `json:"standalone"`
	RejectedConflict  int `json:"rejected_conflict"`
	RejectedGeometry  int `json:"rejected_geometry"`
	RejectedAmbiguous int `json:"rejected_ambiguous"`
	RejectedStale     int `json:"rejected_stale"`
	RejectedScope     int `json:"rejected_scope"`

	// Decisions is the per-observation detail, bounded for display by the caller.
	Decisions []TextDecision `json:"decisions,omitempty"`
}

// Any reports whether any text evidence arrived at all.
func (t TextSummary) Any() bool { return t.Observations > 0 }

// summariseText tallies the decisions.
func summariseText(decisions []TextDecision) TextSummary {
	s := TextSummary{Observations: len(decisions), Decisions: decisions}
	for _, d := range decisions {
		switch d.Outcome {
		case TextFilledMissingLabel:
			s.FilledLabel++
		case TextReinforcedLabel:
			s.Reinforced++
		case TextSupportingEvidence:
			s.Supporting++
		case TextStandalone:
			s.Standalone++
		case TextRejectedConflict:
			s.RejectedConflict++
		case TextRejectedGeometry:
			s.RejectedGeometry++
		case TextRejectedAmbiguous:
			s.RejectedAmbiguous++
		case TextRejectedStale:
			s.RejectedStale++
		case TextRejectedScope:
			s.RejectedScope++
		}
	}
	return s
}
