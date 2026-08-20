package fusion

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/explain"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Conservative text fusion: the first cross-source rules the Director has.
//
// The architectural rule, in one line:
//
//	Accessibility may establish STRUCTURE.
//	OCR may establish VISIBLE TEXT.
//	Fusion decides whether the evidence describes the same entity.
//	Only structural evidence may establish ACTIONABILITY.
//
// Everything here follows from the asymmetry in that last sentence. Text evidence can
// improve what an element is CALLED and can never change what it IS. Concretely, and
// enforced by construction rather than by discipline:
//
//   - text observations never seed an element, so OCR cannot invent a control;
//   - text never sets or changes Role, so it cannot make anything clickable;
//   - text never changes Bounds, so it cannot move a click target onto the glyphs;
//   - text never changes state flags, so it cannot make a disabled control look usable.
//
// The only field text may write is Label, and only under the named conditions below.
// That is a deliberately small opening, and it is the whole benefit: an anonymous
// button becomes findable by the word printed on it, which is what a user would call
// it anyway.
//
// A worked example of why the restraint matters. Discord exposes eight anonymous panes
// and no operable controls. OCR reads hundreds of words from it. A permissive fusion
// would turn that into hundreds of clickable "controls" and report the application as
// richly actionable — replacing a visible failure ("I cannot see into this") with an
// invisible one ("I am confidently wrong about this"). Coverage may rise. Actionability
// must not.

// TextFusionOutcome is what became of one text observation.
type TextFusionOutcome string

const (
	// TextReinforcedLabel: the element already had this label; the text agrees.
	TextReinforcedLabel TextFusionOutcome = "reinforced_label"
	// TextFilledMissingLabel: the element had no usable label and now has one.
	TextFilledMissingLabel TextFusionOutcome = "filled_missing_label"
	// TextSupportingEvidence: attached as provenance without changing any field.
	TextSupportingEvidence TextFusionOutcome = "supporting_evidence"
	// TextStandalone: no structural element to attach to. Stays in the observation
	// graph; see the note on standalone text below.
	TextStandalone TextFusionOutcome = "standalone_text"
	// TextRejectedConflict: the element has a structural label saying something else.
	TextRejectedConflict TextFusionOutcome = "rejected_conflict"
	// TextRejectedGeometry: the text is not inside or overlapping any element.
	TextRejectedGeometry TextFusionOutcome = "rejected_geometry"
	// TextRejectedAmbiguous: more than one element could plausibly own it.
	TextRejectedAmbiguous TextFusionOutcome = "rejected_ambiguous"
	// TextRejectedStale: the text was read from a materially different moment.
	TextRejectedStale TextFusionOutcome = "rejected_stale"
	// TextRejectedScope: different window or application.
	TextRejectedScope TextFusionOutcome = "rejected_scope"
)

// TextDecision is one text observation's fate, with the rule that decided it.
type TextDecision struct {
	Observation directorapi.ObservationReference `json:"observation"`
	Text        string                           `json:"text"`
	Outcome     TextFusionOutcome                `json:"outcome"`
	Rule        string                           `json:"rule"`
	Reason      string                           `json:"reason"`
	// Element is what it attached to, when it attached to anything.
	Element directorapi.ElementID `json:"element,omitempty"`
	// Candidates are the elements that could have owned it, for the ambiguous case.
	Candidates []string `json:"candidates,omitempty"`
	// Containment is how much of the text box lies inside the element, 0..1.
	Containment float64 `json:"containment,omitempty"`

	// cluster is which cluster it attached to. Unexported because it is an
	// implementation detail of one run — the durable answer is Element, which is
	// back-filled once identity exists.
	cluster int
}

// Text fusion thresholds. Named and provisional, like the provider's.
const (
	// minContainment is how much of a text box must lie inside an element before the
	// text can be said to belong to it.
	//
	// High, because the alternative errs in the dangerous direction. Text that merely
	// TOUCHES a button is usually the label of the control beside it, and attaching it
	// would rename a button after its neighbour — producing an element that is
	// findable by the wrong word, with full confidence and no visible defect.
	minContainment = 0.80

	// ambiguityMargin: when a second element contains the text nearly as well as the
	// best one, the text belongs to neither as far as fusion is concerned. Nested
	// containers make this the common case, not a corner one.
	ambiguityMargin = 0.15

	// maxTextAge is how far apart a text observation and its cycle may be. Text read
	// from a screen that has since changed is not weaker evidence; it is evidence
	// about a different screen.
	maxTextAge = 3 * time.Second

	// labelAgreementBonus and labelFillBonus are the bounded contributions text makes
	// to an element's confidence. Small: agreement between a strong source and a weak
	// one is worth something, and nowhere near what a second strong source would be.
	labelAgreementBonus = 0.04
	labelFillBonus      = 0.02
	// labelConflictPenalty reduces confidence in the LABEL when sources disagree,
	// without reducing confidence that the element exists — two sources disagreeing
	// about what a control is called still both agree there is a control there.
	labelConflictPenalty = 0.25
)

// fuseText attaches text evidence to already-clustered elements.
//
// Runs AFTER clustering, never as part of it, and that ordering is the safety property:
// clustering is what creates elements, so text that is not present during clustering
// cannot create one. There is no code path by which an OCR observation becomes a
// control.
func fuseText(fused []Fused, texts []observation.Text, cycleID observation.CycleID,
	cycleAt time.Time, rec *recorder) []TextDecision {

	if len(texts) == 0 {
		return nil
	}

	// Deterministic order. Text arrives in engine order, which is stable, but a caller
	// assembling a cycle from several sources need not preserve it — and two runs
	// producing different labels for the same screen would be the worst possible
	// property for something whose whole job is to be explainable.
	ordered := append([]observation.Text(nil), texts...)
	sort.SliceStable(ordered, func(a, b int) bool {
		return ordered[a].ObservationID < ordered[b].ObservationID
	})

	decisions := make([]TextDecision, 0, len(ordered))
	for _, t := range ordered {
		decisions = append(decisions, fuseOneText(fused, t, cycleID, cycleAt, rec))
	}
	return decisions
}

// fuseOneText decides what becomes of a single text observation.
func fuseOneText(fused []Fused, t observation.Text, cycleID observation.CycleID,
	cycleAt time.Time, rec *recorder) TextDecision {

	d := TextDecision{
		Observation: observation.Reference(t, cycleID),
		Text:        t.Content.Raw,
	}

	// Rule 3: compatible observation period. Checked first because a stale reading is
	// disqualified regardless of where it landed, and checking geometry first would
	// mean computing overlaps against a screen that no longer exists.
	if !cycleAt.IsZero() && !t.At.IsZero() {
		if age := cycleAt.Sub(t.At); age > maxTextAge || age < -maxTextAge {
			d.Outcome, d.Rule = TextRejectedStale, "stale_text"
			d.Reason = fmt.Sprintf("read %s from this cycle's moment, beyond the %s window — "+
				"text from a screen that has since changed is evidence about a different screen",
				absDuration(age).Round(time.Millisecond), maxTextAge)
			return d
		}
	}

	// No structure at all. Distinguished from a window mismatch below because they are
	// different findings: "this application exposes nothing" is the Discord case, and
	// "this text belongs to another window" is a scoping error. Reporting the first as
	// the second would hide the one worth acting on.
	if len(fused) == 0 {
		d.Outcome, d.Rule = TextStandalone, "no_structure"
		d.Reason = "nothing structural was observed at all, so there is nothing this text " +
			"could be the name of — it stays evidence"
		return d
	}

	// Rules 1 and 2: same application, same window. Hard gates. The same word appears
	// in many windows, and geometry alone would happily attach a background window's
	// text to whatever overlaps it in front.
	candidates, scoped := textCandidates(fused, t)
	if !scoped {
		d.Outcome, d.Rule = TextRejectedScope, "scope_mismatch"
		d.Reason = "no element in the same window and application — the same word in two " +
			"windows is two different words"
		return d
	}

	// Rule 4: contained by, or substantially overlapping, a structural element.
	if len(candidates) == 0 {
		d.Outcome, d.Rule = TextStandalone, "no_container"
		d.Reason = "no structural element contains this text, so it stays evidence: visible " +
			"text with nothing structural under it is not a control"
		return d
	}

	best := candidates[0]
	if rival, ok := ambiguousRival(candidates); ok {
		// Two elements could equally own this text. Choosing arbitrarily would label a
		// control after its neighbour half the time, and the half it got wrong would be
		// invisible.
		d.Outcome, d.Rule = TextRejectedAmbiguous, "ambiguous_container"
		d.Containment = best.containment
		for _, c := range []textCandidate{best, rival} {
			d.Candidates = append(d.Candidates,
				fmt.Sprintf("%s(%.2f, %dpx²)", c.fused.Element.ID, c.containment,
					c.fused.Element.Bounds.Area()))
		}
		d.Reason = "two comparably-sized elements contain this text equally well, so it " +
			"belongs to neither as far as belief is concerned"
		return d
	}

	el := best.fused.Element
	// The element has no ID yet: text fusion runs BEFORE identity is assigned, so that a
	// label filled from OCR is part of what makes the element recognisable next cycle.
	// The id is back-filled by the caller once it exists.
	d.cluster, d.Containment = best.index, best.containment

	// Rules 5 and 6: what the element's own label says.
	existing := observation.Normalize(el.Label)
	incoming := t.Content.Comparable

	switch {
	case existing == "":
		// The opening this whole feature exists for: a structurally real, actionable
		// control that nothing can name. The role, the bounds and the actionability all
		// stay exactly as accessibility reported them; only the label is filled.
		el.Label = t.Content.Raw
		attachText(el, t, cycleID)
		adjustConfidence(el, labelFillBonus)
		el.LabelConfidence = clamp(labelConfidence(el, t.Score))
		d.Outcome, d.Rule = TextFilledMissingLabel, "empty_structural_label"
		d.Reason = "the element is structurally real but had no label; the contained text " +
			"names it. Role and actionability are unchanged and still come from accessibility"
		rec.textFilled(best.index, d)

	case labelsAgree(existing, incoming):
		el.Provenance.Add(t.Reference(cycleID))
		addSource(el, directorapi.SourceOCR)
		adjustConfidence(el, labelAgreementBonus)
		el.LabelConfidence = clamp(labelConfidence(el, t.Score))
		d.Outcome, d.Rule = TextReinforcedLabel, "label_agreement"
		d.Reason = "an independent source read the same words in the same place"
		rec.textReinforced(best.index, d)

	default:
		// Rule 6: stronger structural evidence exists and says something else. The
		// structural label is KEPT — silently overwriting it with pixels would be the
		// single most dangerous thing this file could do, since "Delete" read as
		// "Cancel" is a click nobody intended.
		el.Provenance.Add(t.Reference(cycleID))
		addSource(el, directorapi.SourceOCR)
		el.LabelConfidence = clamp(labelConfidence(el, t.Score) - labelConflictPenalty)
		d.Outcome, d.Rule = TextRejectedConflict, "label_conflict"
		d.Reason = fmt.Sprintf("the structural label %q is kept; OCR read %q in the same "+
			"place. Confidence in the LABEL is reduced, confidence that the element exists is not",
			el.Label, t.Content.Raw)
		rec.textConflict(best.index, d)
	}
	return d
}

// ambiguousRival reports whether a second candidate could equally own the text.
//
// NESTING IS NOT AMBIGUITY, and conflating the two is the mistake this function exists
// to avoid. Containment measures how much of the text lies inside the element, so every
// ancestor of the real owner scores exactly 1.0 as well: a word on a button is equally
// inside the button, the toolbar, the pane and the window. Measured live against VS
// Code, treating that as a tie rejected all 217 text observations — which looks like
// caution and is really just blindness.
//
// A word is printed ON the innermost control that contains it, which is the same rule
// that resolves "this" in "click this" to the button rather than the dialog. So the
// smallest container wins, and ambiguity means something narrower: two candidates that
// contain the text equally well AND are comparably sized — siblings, not ancestors.
func ambiguousRival(sorted []textCandidate) (textCandidate, bool) {
	if len(sorted) < 2 {
		return textCandidate{}, false
	}
	best := sorted[0]
	bestArea := best.fused.Element.Bounds.Area()

	for _, c := range sorted[1:] {
		if c.containment <= best.containment-ambiguityMargin {
			continue // materially worse containment; not a rival
		}
		area := c.fused.Element.Bounds.Area()
		if bestArea <= 0 || area <= 0 {
			continue
		}
		// Comparable in size, so neither is plainly inside the other.
		if float64(area) <= float64(bestArea)*nestingRatio {
			return c, true
		}
	}
	return textCandidate{}, false
}

// nestingRatio is how much bigger a rival container may be before it is treated as an
// ANCESTOR rather than a peer. A control's own box and its parent's differ by far more
// than this in every real toolkit; two adjacent buttons differ by far less.
const nestingRatio = 1.5

// textCandidate is one element a text observation might belong to.
type textCandidate struct {
	index       int
	fused       Fused
	containment float64
}

// textCandidates finds the elements that could own this text, best containment first.
//
// The second return reports whether ANY element shared the text's window and
// application — which distinguishes "there is nothing structural here" from "there is
// structure, and this text is not inside any of it".
func textCandidates(fused []Fused, t observation.Text) ([]textCandidate, bool) {
	var out []textCandidate
	scoped := false

	for i, f := range fused {
		el := f.Element
		if el.WindowID != "" && t.WindowID != "" && el.WindowID != t.WindowID {
			continue
		}
		scoped = true
		if el.Bounds.Empty() || t.Box.Empty() {
			continue
		}
		// Containment of the TEXT by the ELEMENT, not intersection-over-union. A word
		// inside a button covers a small fraction of it, which IoU rates near zero —
		// the exact relationship that matters here.
		cov := el.Bounds.Covers(t.Box)
		if cov < minContainment {
			continue
		}
		out = append(out, textCandidate{index: i, fused: f, containment: cov})
	}

	// Smallest container first among equals: the pointer is over a pane, a group and a
	// button at once, and the button is the one the text is printed on.
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
	return out, scoped
}

// labelsAgree reports whether two normalised labels describe the same thing.
//
// Containment either way, because OCR splits labels at glyph boundaries: a button
// labelled "Save As" may be read as two words, and the first of them agrees with the
// element rather than contradicting it.
func labelsAgree(existing, incoming string) bool {
	if existing == "" || incoming == "" {
		return false
	}
	if existing == incoming {
		return true
	}
	return strings.Contains(existing, incoming) || strings.Contains(incoming, existing)
}

// attachText records text evidence on an element without touching structure.
func attachText(el *directorapi.Element, t observation.Text, cycleID observation.CycleID) {
	el.Provenance.Add(t.Reference(cycleID))
	addSource(el, directorapi.SourceOCR)
}

func addSource(el *directorapi.Element, s directorapi.ObservationSource) {
	if !el.HasSource(s) {
		el.Sources = append(el.Sources, s)
	}
}

// adjustConfidence applies a BOUNDED contribution to existence confidence.
//
// Bounded and small, never additive in the naive sense: 0.9 + 0.8 is not 1.7, and a
// model where corroboration could push past certainty would let a threshold downstream
// read agreement as proof. The cap is fusion's existing one.
func adjustConfidence(el *directorapi.Element, delta float64) {
	el.Confidence = clamp(el.Confidence + delta*(1-el.Confidence)/(1-maxConfidence+0.01))
	if el.Confidence > maxConfidence {
		el.Confidence = maxConfidence
	}
}

// labelConfidence is how sure the Director is about what an element is CALLED, as
// distinct from whether it exists.
//
// Separate because the two genuinely differ: two sources disagreeing about a label
// still agree there is a control there. Keeping one number would force a choice between
// under-trusting the element and over-trusting its name.
func labelConfidence(el *directorapi.Element, textScore float64) float64 {
	base := el.Confidence
	if base <= 0 {
		base = 0.5
	}
	return base*0.7 + textScore*0.3
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > maxConfidence {
		return maxConfidence
	}
	return v
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// ── grouping ──────────────────────────────────────────────────────────────────

// maxWordGap is how far apart two words may sit and still be one phrase, as a
// multiple of the line height. Deliberately tight: joining across a wider gap
// concatenates a label with its neighbour, producing a phrase the screen does not
// contain.
const maxWordGap = 0.8

// GroupLine joins adjacent words the engine split, when and only when they are
// unambiguously one rendered phrase.
//
// Conservative on purpose. Aggressive concatenation is how "Save" beside "Cancel"
// becomes "Save Cancel" — a string that appears nowhere, matches nothing, and quietly
// makes both controls unfindable by their real names.
func GroupLine(words []observation.Text) []observation.Text {
	if len(words) < 2 {
		return words
	}
	ordered := append([]observation.Text(nil), words...)
	sort.SliceStable(ordered, func(a, b int) bool {
		if ordered[a].LineID != ordered[b].LineID {
			return ordered[a].LineID < ordered[b].LineID
		}
		if ordered[a].Box.X != ordered[b].Box.X {
			return ordered[a].Box.X < ordered[b].Box.X
		}
		return ordered[a].ObservationID < ordered[b].ObservationID
	})

	var out []observation.Text
	run := []observation.Text{ordered[0]}
	flush := func() {
		out = append(out, joinRun(run))
		run = nil
	}
	for _, w := range ordered[1:] {
		prev := run[len(run)-1]
		if !sameLine(prev, w) || gapTooWide(prev, w) {
			flush()
		}
		run = append(run, w)
	}
	flush()
	return out
}

// sameLine reports whether two words belong to one rendered line.
func sameLine(a, b observation.Text) bool {
	if a.LineID == "" || b.LineID == "" {
		return false // no line information: grouping is unavailable, not guessed
	}
	if a.LineID != b.LineID || a.WindowID != b.WindowID || a.From != b.From {
		return false
	}
	// Vertical alignment, in case an engine reuses line ids across columns.
	centreA := a.Box.Y + a.Box.Height/2
	centreB := b.Box.Y + b.Box.Height/2
	tolerance := maxInt(a.Box.Height, b.Box.Height) / 2
	return absInt(centreA-centreB) <= tolerance
}

// gapTooWide reports whether two words are too far apart to be one phrase.
func gapTooWide(a, b observation.Text) bool {
	gap := b.Box.X - (a.Box.X + a.Box.Width)
	if gap < 0 {
		gap = 0 // overlapping boxes are adjacent enough
	}
	height := maxInt(a.Box.Height, b.Box.Height)
	if height <= 0 {
		return true
	}
	return float64(gap) > maxWordGap*float64(height)
}

// joinRun merges a run of words into one text observation.
func joinRun(run []observation.Text) observation.Text {
	if len(run) == 1 {
		return run[0]
	}
	joined := run[0]
	var parts []string
	box := run[0].Box
	score := 0.0
	for _, w := range run {
		parts = append(parts, w.Content.Raw)
		box = unionRect(box, w.Box)
		score += w.Score
	}
	joined.Content = observation.NewText(strings.Join(parts, " "))
	joined.Box = box
	// The phrase is only as trustworthy as its average word: one confident word does
	// not vouch for the ones beside it.
	joined.Score = score / float64(len(run))
	joined.WordIndex = run[0].WordIndex
	if joined.Metadata == nil {
		joined.Metadata = map[string]string{}
	}
	joined.Metadata["grouped_from"] = fmt.Sprintf("%d words", len(run))
	return joined
}

func unionRect(a, b directorapi.Rect) directorapi.Rect {
	x0, y0 := minInt(a.X, b.X), minInt(a.Y, b.Y)
	x1, y1 := maxInt(a.X+a.Width, b.X+b.Width), maxInt(a.Y+a.Height, b.Y+b.Height)
	return directorapi.Rect{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Reference builds the provenance entry for a text observation.
func textReference(t observation.Text, cycle observation.CycleID) directorapi.ObservationReference {
	return observation.Reference(t, cycle)
}

var _ = textReference
var _ = explain.MergeAccepted

// nameTextDecisions back-fills the element ids once identity has been assigned.
//
// Necessary because of the ordering: text fusion runs before identity, so that a label
// filled from OCR is part of what makes an element recognisable next cycle. The
// decisions therefore know which CLUSTER they attached to and not yet which element
// that cluster became. Without this the diagnostics report every text decision against
// an empty id, which is exactly as useful as not reporting it.
func nameTextDecisions(decisions []TextDecision, fused []Fused) {
	for i := range decisions {
		switch decisions[i].Outcome {
		case TextFilledMissingLabel, TextReinforcedLabel,
			TextSupportingEvidence, TextRejectedConflict:
			if ci := decisions[i].cluster; ci >= 0 && ci < len(fused) {
				decisions[i].Element = fused[ci].Element.ID
			}
		}
	}
}
