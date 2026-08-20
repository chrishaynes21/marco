package fusion

import (
	"fmt"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/explain"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Visual state fusion: the third source, and the most restricted.
//
//	Accessibility may establish STRUCTURE and ACTIONABILITY.
//	OCR may establish VISIBLE TEXT.
//	Visual perception may establish APPEARANCE, STATE, and CHANGE.
//	Fusion decides whether that evidence belongs to an element that already exists.
//	Pixels alone must not create an actionable control.
//
// Text fusion could write one field — the label. Visual fusion can write STATE, and
// only state that the element's ROLE already permits: a checkbox may look checked, a
// tab may look selected, a button may look pressed. A pane may not look like a button,
// because there is no visual kind that says "button" and no path from any visual kind
// to Role or to actionability.
//
// The order of authority is the point. Structural state, when a source actually
// reported it, is what the Director believes. Visual state may fill a gap the structure
// left, or agree, or disagree — and disagreement is RECORDED rather than resolved in
// favour of the pixels, because a control that reports itself disabled and merely looks
// enabled is disabled.

// VisualOutcome is what became of one visual observation.
type VisualOutcome string

const (
	// VisualReinforcedState: structure said the same thing.
	VisualReinforcedState VisualOutcome = "reinforced_state"
	// VisualFilledState: structure reported nothing and appearance filled it in.
	VisualFilledState VisualOutcome = "filled_state"
	// VisualRecordedChange: change evidence about a region, attached to nothing.
	VisualRecordedChange VisualOutcome = "recorded_change"
	// VisualRejectedRole: the element's role does not permit this state.
	VisualRejectedRole VisualOutcome = "rejected_role"
	// VisualRejectedConflict: structure says otherwise, and structure wins.
	VisualRejectedConflict VisualOutcome = "rejected_conflict"
	// VisualRejectedGeometry: no element contains the region.
	VisualRejectedGeometry VisualOutcome = "rejected_geometry"
	// VisualRejectedAmbiguous: two comparable elements could own it.
	VisualRejectedAmbiguous VisualOutcome = "rejected_ambiguous"
	// VisualRejectedStale: read from a materially different moment.
	VisualRejectedStale VisualOutcome = "rejected_stale"
	// VisualRejectedScope: different window or application.
	VisualRejectedScope VisualOutcome = "rejected_scope"
)

// VisualDecision is one visual observation's fate.
type VisualDecision struct {
	Observation directorapi.ObservationReference `json:"observation"`
	VisualKind  observation.VisualStateKind      `json:"visual_kind"`
	Outcome     VisualOutcome                    `json:"outcome"`
	Rule        string                           `json:"rule"`
	Reason      string                           `json:"reason"`
	Element     directorapi.ElementID            `json:"element,omitempty"`
	// Flag is the state it wrote or would have written.
	Flag string `json:"flag,omitempty"`
	// Candidates are the elements that could have owned it, for the ambiguous case.
	Candidates []string `json:"candidates,omitempty"`

	cluster int
}

// maxVisualAge is how far apart a visual observation and its cycle may be.
//
// Tighter than text. A label read a second ago is almost certainly still the label; an
// appearance read a second ago may be a hover that has since ended, a spinner that has
// stopped, or a selection that has moved. Appearance is the most perishable evidence
// the Director has.
const maxVisualAge = 1500 * time.Millisecond

// roleAllowsState reports whether an element's role permits a visual state claim.
//
// The gate that keeps appearance from becoming semantics. "This looks highlighted" is a
// legitimate thing to say about a tab, a row or a list item, and a meaningless thing to
// say about a window — so on a window it is refused rather than recorded, and no amount
// of visual confidence changes that.
func roleAllowsState(role directorapi.ElementRole, kind observation.VisualStateKind) bool {
	switch kind {
	case observation.VisualSelected:
		switch role {
		case directorapi.RoleTab, directorapi.RoleListItem, directorapi.RoleRow,
			directorapi.RoleTreeItem, directorapi.RoleMenuItem, directorapi.RoleCell:
			return true
		}
	case observation.VisualChecked:
		// Only where CHECKEDNESS is a real property of the control. A checkbox looking
		// ticked is evidence; a button looking ticked is a misread.
		switch role {
		case directorapi.RoleCheckbox, directorapi.RoleRadio, directorapi.RoleMenuItem,
			directorapi.RoleToggle:
			return true
		}
	case observation.VisualPressed:
		switch role {
		case directorapi.RoleButton, directorapi.RoleToggle:
			return true
		}
	case observation.VisualExpanded, observation.VisualCollapsed:
		switch role {
		case directorapi.RoleTreeItem, directorapi.RoleMenuItem, directorapi.RoleGroup,
			directorapi.RoleComboBox:
			return true
		}
	case observation.VisualDisabledAppearance:
		// Any operable control can look disabled. A pane cannot: it was never operable,
		// so "it looks disabled" says nothing about it.
		return role.Clickable() || role == directorapi.RoleTextField
	case observation.VisualLoading, observation.VisualProgress:
		// Busy is the one state a container legitimately has — a pane can be loading in
		// a way a pane cannot be "pressed".
		switch role {
		case directorapi.RoleWindow, directorapi.RolePane, directorapi.RoleGroup,
			directorapi.RoleButton, directorapi.RoleProgressBar:
			return true
		}
	}
	return false
}

// fuseVisual attaches visual state evidence to already-clustered elements.
//
// Runs after clustering and after text fusion, and like text fusion it can only ever
// modify an element that already exists. There is no path from a visual observation to
// a new cluster, so pixels cannot create a control — which is the milestone's governing
// rule, enforced by the shape of the code rather than by a check that could be removed.
func fuseVisual(fused []Fused, visuals []observation.VisualState,
	cycleID observation.CycleID, cycleAt time.Time, rec *recorder) []VisualDecision {

	if len(visuals) == 0 {
		return nil
	}
	ordered := append([]observation.VisualState(nil), visuals...)
	sort.SliceStable(ordered, func(a, b int) bool {
		return ordered[a].ObservationID < ordered[b].ObservationID
	})

	out := make([]VisualDecision, 0, len(ordered))
	for _, v := range ordered {
		out = append(out, fuseOneVisual(fused, v, cycleID, cycleAt, rec))
	}
	return out
}

func fuseOneVisual(fused []Fused, v observation.VisualState, cycleID observation.CycleID,
	cycleAt time.Time, rec *recorder) VisualDecision {

	d := VisualDecision{
		Observation: v.Reference(cycleID),
		VisualKind:  v.VisualKind,
		cluster:     -1,
	}

	// Change evidence is about an EVENT, not about a control. It is kept as evidence
	// and attached to nothing: a region that changed is not a durable property of
	// anything, and writing it onto an element would make the element's state depend on
	// what happened to have moved recently.
	if v.VisualKind.AboutChange() {
		d.Outcome, d.Rule = VisualRecordedChange, "change_evidence"
		d.Reason = "a region changed, which is evidence about an event rather than about " +
			"a control — kept for verification, written onto nothing"
		return d
	}

	flag, value, ok := v.VisualKind.StateFlag()
	if !ok {
		d.Outcome, d.Rule = VisualRejectedRole, "unmapped_kind"
		d.Reason = "this visual kind maps to no state the Director models"
		return d
	}
	d.Flag = flag

	if !cycleAt.IsZero() && !v.At.IsZero() {
		if age := cycleAt.Sub(v.At); age > maxVisualAge || age < -maxVisualAge {
			d.Outcome, d.Rule = VisualRejectedStale, "stale_visual"
			d.Reason = fmt.Sprintf("read %s from this cycle's moment — appearance is the "+
				"most perishable evidence there is, and a hover that has since ended looks "+
				"exactly like one that has not", absDuration(age).Round(time.Millisecond))
			return d
		}
	}

	if len(fused) == 0 {
		d.Outcome, d.Rule = VisualRejectedGeometry, "no_structure"
		d.Reason = "nothing structural was observed, so there is nothing this appearance " +
			"could be the state of — pixels alone do not make a control"
		return d
	}

	candidates, scoped := visualCandidates(fused, v)
	if !scoped {
		d.Outcome, d.Rule = VisualRejectedScope, "scope_mismatch"
		d.Reason = "no element in the same window and application"
		return d
	}
	if len(candidates) == 0 {
		d.Outcome, d.Rule = VisualRejectedGeometry, "no_container"
		d.Reason = "no element of a compatible role overlaps this region"
		return d
	}
	if rival, ambiguous := ambiguousRival(candidates); ambiguous {
		d.Outcome, d.Rule = VisualRejectedAmbiguous, "ambiguous_container"
		for _, c := range []textCandidate{candidates[0], rival} {
			d.Candidates = append(d.Candidates, string(c.fused.Element.ID))
		}
		d.Reason = "two comparably-sized elements of compatible role overlap this region"
		return d
	}

	best := candidates[0]
	el := best.fused.Element
	d.cluster = best.index

	// Structural authority. A source that actually REPORTED this state is believed over
	// one that inferred it from averaged colour, and disagreement is recorded rather
	// than resolved toward the pixels.
	if prior, reported := el.StateEvidence[flag]; reported && prior.Source.Structured() {
		if prior.Value == value {
			prior.Confidence = reinforce(prior.Confidence, v.Score)
			el.StateEvidence[flag] = prior
			attachVisual(el, v, cycleID)
			d.Outcome, d.Rule = VisualReinforcedState, "state_agreement"
			d.Reason = fmt.Sprintf("appearance agrees with the structural %s state", flag)
			rec.visual(best.index, d, explain.MergeAccepted)
			return d
		}
		prior.Conflict = fmt.Sprintf("%s appeared %s", v.From, v.VisualKind)
		// Confidence in the STATE falls; nothing about the element's existence,
		// role or actionability is touched.
		prior.Confidence = clamp(prior.Confidence - visualConflictPenalty)
		el.StateEvidence[flag] = prior
		attachVisual(el, v, cycleID)

		d.Outcome, d.Rule = VisualRejectedConflict, "structural_state_wins"
		d.Reason = fmt.Sprintf(
			"structure reported %s=%v and it is kept; the region merely LOOKS %s. "+
				"Confidence in that state is reduced; the element's role and actionability "+
				"are untouched", flag, prior.Value, v.VisualKind)
		rec.visual(best.index, d, explain.MergeRejected)
		return d
	}

	// Nothing structural said anything. Appearance may fill the gap — recorded as
	// coming from vision, so nothing downstream can mistake it for a structural fact.
	if el.StateEvidence == nil {
		el.StateEvidence = map[string]directorapi.StateFact{}
	}
	el.StateEvidence[flag] = directorapi.StateFact{
		Value: value, Source: v.From, Confidence: v.Score, Observation: v.ObservationID,
	}
	// The one legacy field appearance may write, and only when nothing reported it.
	// Role, bounds, enabled and every capability stay exactly as structure left them.
	if flag == directorapi.StateSelected && value {
		el.Selected = true
	}
	attachVisual(el, v, cycleID)

	d.Outcome, d.Rule = VisualFilledState, "missing_structural_state"
	d.Reason = fmt.Sprintf("no source reported %s, and the region looks %s. "+
		"The role and everything that follows from it still come from accessibility",
		flag, v.VisualKind)
	rec.visual(best.index, d, explain.MergeAccepted)
	return d
}

// visualCandidates finds the elements a visual observation might describe.
//
// Narrower than the text version in one important way: an element whose ROLE does not
// permit the state is not a candidate at all. That is what stops "this looks
// highlighted" being recorded against the pane behind the tab.
func visualCandidates(fused []Fused, v observation.VisualState) ([]textCandidate, bool) {
	var out []textCandidate
	scoped := false

	for i, f := range fused {
		el := f.Element
		if el.WindowID != "" && v.WindowID != "" && el.WindowID != v.WindowID {
			continue
		}
		scoped = true
		if !roleAllowsState(el.Role, v.VisualKind) {
			continue
		}
		if el.Bounds.Empty() || v.Box.Empty() {
			continue
		}
		// Either direction: a watched region is usually LARGER than its target (it
		// includes a margin), where a state observation of a checkbox glyph is smaller.
		cov := el.Bounds.Covers(v.Box)
		if inv := v.Box.Covers(el.Bounds); inv > cov {
			cov = inv
		}
		if cov < minContainment {
			continue
		}
		out = append(out, textCandidate{index: i, fused: f, containment: cov})
	}

	sort.SliceStable(out, func(a, b int) bool {
		if out[a].containment != out[b].containment {
			return out[a].containment > out[b].containment
		}
		aa := out[a].fused.Element.Bounds.Area()
		bb := out[b].fused.Element.Bounds.Area()
		if aa != bb {
			return aa < bb
		}
		return out[a].fused.Element.ID < out[b].fused.Element.ID
	})

	// A hinted target wins outright when it is among the candidates. The provider was
	// looking AT that element, which is better information than geometry — but it is
	// still only a hint: an element the hint names that fails the role or geometry gate
	// is not promoted past them.
	if v.TargetHint != nil {
		for i, c := range out {
			if c.fused.Element.ID == *v.TargetHint && i != 0 {
				out[0], out[i] = out[i], out[0]
				break
			}
		}
	}
	return out, scoped
}

// visualConflictPenalty is how much a disagreement costs confidence in a STATE.
// Nothing else about the element is affected: a control that reports itself disabled
// and looks enabled is still a control, still in the same place, still that role.
const visualConflictPenalty = 0.2

// reinforce raises a state confidence by corroboration, bounded.
func reinforce(current, by float64) float64 {
	return clamp(current + (1-current)*0.5*by)
}

// attachVisual records the evidence on an element without touching structure.
func attachVisual(el *directorapi.Element, v observation.VisualState, cycle observation.CycleID) {
	el.Provenance.Add(v.Reference(cycle))
	addSource(el, v.From)
}

// nameVisualDecisions back-fills element ids once identity has been assigned.
func nameVisualDecisions(decisions []VisualDecision, fused []Fused) {
	for i := range decisions {
		if ci := decisions[i].cluster; ci >= 0 && ci < len(fused) {
			decisions[i].Element = fused[ci].Element.ID
		}
	}
}

// VisualSummary is what one cycle's visual evidence achieved.
//
// Shaped, like the text summary, so the safety property reads off the page. There is no
// counter for "controls created from appearance", because that number cannot be
// anything but zero.
type VisualSummary struct {
	Observations     int `json:"observations"`
	FilledState      int `json:"filled_state"`
	ReinforcedState  int `json:"reinforced_state"`
	RecordedChange   int `json:"recorded_change"`
	RejectedRole     int `json:"rejected_role"`
	RejectedConflict int `json:"rejected_conflict"`
	RejectedGeometry int `json:"rejected_geometry"`

	RejectedAmbiguous int              `json:"rejected_ambiguous"`
	RejectedStale     int              `json:"rejected_stale"`
	RejectedScope     int              `json:"rejected_scope"`
	Decisions         []VisualDecision `json:"decisions,omitempty"`
}

// Any reports whether any visual evidence arrived.
func (v VisualSummary) Any() bool { return v.Observations > 0 }

func summariseVisual(decisions []VisualDecision) VisualSummary {
	s := VisualSummary{Observations: len(decisions), Decisions: decisions}
	for _, d := range decisions {
		switch d.Outcome {
		case VisualFilledState:
			s.FilledState++
		case VisualReinforcedState:
			s.ReinforcedState++
		case VisualRecordedChange:
			s.RecordedChange++
		case VisualRejectedRole:
			s.RejectedRole++
		case VisualRejectedConflict:
			s.RejectedConflict++
		case VisualRejectedGeometry:
			s.RejectedGeometry++
		case VisualRejectedAmbiguous:
			s.RejectedAmbiguous++
		case VisualRejectedStale:
			s.RejectedStale++
		case VisualRejectedScope:
			s.RejectedScope++
		}
	}
	return s
}
