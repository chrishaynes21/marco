// Package fusion merges raw observations into the elements the Director plans
// against.
//
// Several sources look at the same desktop and describe the same objects
// differently. Accessibility says there is a button labelled "Save" at
// (900,700,120,40). OCR says the word "Save" appears at (920,710,70,20). A detector
// says something button-shaped is at (900,700,120,40). Those are three accounts of
// ONE thing, and the Director has to plan against the thing, not the accounts.
//
// Two rules shape everything here.
//
// STRUCTURE OUTRANKS PIXELS. When sources disagree, the one higher on the evidence
// ladder wins the field (see directorapi.SourceRank). Accessibility knows a button's
// real hit box and its enabled state; OCR knows where some glyphs are. Letting an
// OCR box overwrite an accessibility bound would move the click target to the text
// inside the control, which is usually still inside it — and occasionally is not.
//
// EVIDENCE IS NEVER DESTROYED. Fusion produces an element and KEEPS the observations
// that produced it. That is what makes "why did you click that?" answerable after
// the fact, and it is the difference between a system you can debug and one you can
// only re-run.
package fusion

import (
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/explain"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Fused is one element plus the identity and evidence needed to place it in a tree
// and explain it. The Element has no ID yet — stable identity is assigned afterwards
// by the identity matcher below, which needs to compare against the previous snapshot.
type Fused struct {
	Element *directorapi.Element
	// NativeID is the winning observation's own identifier (a UIA RuntimeId, a DOM
	// id). Empty when no contributing source had one.
	NativeID string
	// ParentNativeID links to the parent's NativeID, so the tree can be rebuilt once
	// IDs exist.
	ParentNativeID string
	// Observations is the cluster this element was fused from, strongest source
	// first. Retained as provenance, not as a debugging afterthought.
	Observations []directorapi.Observation
}

// Fuse merges observations into elements, without cycle provenance. Used where the
// evidence has no cycle — recorded fixtures, and tests of clustering itself.
func Fuse(obs []directorapi.Observation) []Fused {
	fused, _ := cluster(obs, "", nil)
	return fused
}

// cluster merges observations into elements, recording what disagreed.
//
// The result is deterministic: observations are processed strongest-source-first and
// ties are broken by input order, so the same input always produces the same
// clustering. Fixture-based testing depends on that.
// rec, when non-nil, captures the decisions that produced each cluster. It is nil on
// the hot path: every command observes, and paying for an explanation nobody asked for
// on every cycle is how a diagnostics layer becomes a performance regression. Because
// clustering is a pure, deterministic function of its input, an explanation produced by
// re-running it later is identical to one recorded at the time.
func cluster(obs []directorapi.Observation, cycle observation.CycleID, rec *recorder) ([]Fused, []Conflict) {
	if len(obs) == 0 {
		return nil, nil
	}

	// Strongest source first, original order within a source. Processing this way
	// means a cluster's first member is always its most authoritative account, so
	// weaker observations attach to a structured anchor rather than seeding clusters
	// of their own that a structured observation then has to be merged into.
	idx := make([]int, len(obs))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		ra := directorapi.SourceRank(obs[idx[a]].Source)
		rb := directorapi.SourceRank(obs[idx[b]].Source)
		if ra != rb {
			return ra > rb
		}
		return idx[a] < idx[b]
	})

	var clusters [][]directorapi.Observation
	for _, i := range idx {
		o := obs[i]
		best, bestScore := -1, mergeThreshold
		var verdicts []clusterVerdict
		for ci, c := range clusters {
			v := scoreAgainst(c, o, rec != nil)
			if rec != nil {
				verdicts = append(verdicts, v)
			}
			if v.score > bestScore {
				best, bestScore = ci, v.score
			}
		}
		if best >= 0 {
			clusters[best] = append(clusters[best], o)
			// Guarded rather than relying on the recorder's nil-safety: Go evaluates
			// arguments before the call, so verdicts[best] would panic on the hot path
			// where verdicts is never populated. A nil-safe method is not a nil-safe
			// call site.
			if rec != nil {
				rec.merged(best, o, cycle, verdicts[best])
				// Everywhere else it was refused is recorded too, but only where it
				// came close enough to be a decision rather than a non-event.
				rec.refusedElsewhere(o, cycle, verdicts, best)
			}
			continue
		}
		clusters = append(clusters, []directorapi.Observation{o})
		rec.seeded(len(clusters)-1, o, cycle)
		rec.refusedElsewhere(o, cycle, verdicts, -1)
	}

	out := make([]Fused, 0, len(clusters))
	var conflicts []Conflict
	for ci, c := range clusters {
		f, cf := build(c, cycle, rec, ci)
		out = append(out, f)
		conflicts = append(conflicts, cf...)
	}
	return out, conflicts
}

// mergeThreshold is the score two observations must reach to be treated as the same
// object. Set high on purpose: wrongly merging two adjacent controls creates an
// element that is neither of them, with bounds spanning both — and a click on it
// lands between them. Wrongly SPLITTING one object into two is far less harmful,
// because ranking then simply sees two candidates for the same target.
const mergeThreshold = 0.55

// clusterVerdict is how an observation fared against one cluster, and why.
type clusterVerdict struct {
	score float64
	// rule names what settled it, stable and machine-readable.
	rule string
	// reason is the sentence a person reads.
	reason string
	// against is the cluster member the verdict was reached against.
	against directorapi.Observation
	// plausible is whether the pair were spatially related at all. A refusal between
	// two things in different places is not a decision worth recording — it is the
	// absence of one, and recording every such pair would bury the handful that matter.
	plausible bool
}

// scoreAgainst is how well o matches an existing cluster: the best score against any
// member, or 0 if o may not join at all.
//
// explaining widens what is computed rather than what is decided. The verdict's score
// is identical either way; the extra work is only establishing whether a refusal was a
// near miss, which the hot path has no use for.
func scoreAgainst(cluster []directorapi.Observation, o directorapi.Observation, explaining bool) clusterVerdict {
	best := clusterVerdict{}
	for _, m := range cluster {
		// Two observations from the SAME source in one snapshot are different
		// objects, always. A source enumerates the desktop once; if it reported two
		// nodes, it means two nodes. Merging them would collapse a list into a row.
		if m.Source == o.Source {
			v := clusterVerdict{
				rule:    "same_source",
				reason:  "the same source reported both, and one enumeration of the desktop cannot corroborate itself",
				against: m,
			}
			if explaining {
				// Only interesting where they overlap: two nodes of one tree in the
				// same place is a real question ("is this a button inside a button?"),
				// two nodes in different places is not.
				v.plausible = geometryScore(m.Bounds, o.Bounds) > 0
			}
			return v
		}
		s, rule, reason := pairVerdict(m, o)
		if s > best.score || best.rule == "" {
			best = clusterVerdict{
				score: s, rule: rule, reason: reason, against: m,
				plausible: s > 0 || geometryScore(m.Bounds, o.Bounds) > 0,
			}
		}
	}
	return best
}

// pairScore rates how likely two observations are to describe the same object, 0..1.
func pairScore(a, b directorapi.Observation) float64 {
	s, _, _ := pairVerdict(a, b)
	return s
}

// pairVerdict is pairScore plus the name of the rule that settled it.
//
// One function rather than two so the explanation cannot drift from the decision. A
// separate "explain why these did not merge" routine would be a second implementation
// of the merge rules, and the first time they disagreed the explanation would be
// confidently wrong — which is worse than having none.
func pairVerdict(a, b directorapi.Observation) (float64, string, string) {
	// Different windows are different objects. This is a hard gate rather than a
	// weighted signal: two windows can overlap on screen, and geometry alone would
	// happily merge a dialog's OK button with whatever sits behind it.
	if a.WindowID != "" && b.WindowID != "" && a.WindowID != b.WindowID {
		return 0, "window_mismatch",
			"they are in different windows, and windows overlap on screen"
	}
	if !rolesCompatible(a.Role, b.Role) {
		return 0, "role_conflict", "a " + string(a.Role) + " and a " + string(b.Role) +
			" in the same place are two controls, not one seen twice"
	}

	geo := geometryScore(a.Bounds, b.Bounds)
	if geo == 0 {
		// No spatial relationship at all. Nothing else can rescue that: two things in
		// different places are two things, however similar their text.
		return 0, "no_overlap", "their bounds are unrelated, and two things in different places are two things"
	}
	lbl := labelScore(a, b)

	// Geometry carries the weight because it is the one signal every source
	// produces. A label agreement is a strong confirmation on top, and a label
	// CONFLICT is disqualifying — two different words in the same place are a
	// control and its neighbour, not one object seen twice.
	if lbl < 0 {
		return 0, "label_conflict",
			"they read as different words in the same place: " + firstText(a) + " and " + firstText(b)
	}

	score := 0.7*geo + 0.3*lbl
	rule, reason := "bounds_overlap", "their bounds coincide"
	if lbl > 0 {
		rule, reason = "bounds_and_label", "their bounds coincide and they agree on the text"
	}
	if score <= mergeThreshold {
		return score, "below_threshold",
			reason + ", but not closely enough to be certain they are one object"
	}
	return score, rule, reason
}

// geometryScore rates the spatial relationship between two bounds.
//
// It uses two measures because sources report different things. Coincident boxes
// (accessibility and a detector describing the same control) score on IoU. A
// contained box (an OCR word inside its button) scores on coverage, which IoU rates
// near zero — the case a single symmetric measure would always miss.
func geometryScore(a, b directorapi.Rect) float64 {
	if a.Empty() || b.Empty() {
		// One source knows the object exists but not where. Position cannot confirm
		// or deny, and guessing here would merge on label alone.
		return 0
	}
	if iou := a.IoU(b); iou >= 0.5 {
		return iou
	}
	// Containment: the smaller box lies (almost) wholly inside the larger one.
	inner, outer := a, b
	if a.Area() > b.Area() {
		inner, outer = b, a
	}
	if cov := outer.Covers(inner); cov >= 0.8 {
		// Scaled below a strong IoU: containment is good evidence, but a button also
		// "contains" a smaller button drawn inside it.
		return 0.5 + 0.3*cov
	}
	return 0
}

// labelScore rates textual agreement: 1 for a match, 0 when one side is silent, and
// -1 for a genuine conflict (which pairScore treats as disqualifying).
func labelScore(a, b directorapi.Observation) float64 {
	ta, tb := textOf(a), textOf(b)
	if ta == "" || tb == "" {
		return 0 // silence is not disagreement
	}
	if ta == tb {
		return 1
	}
	// Sources trim and decorate differently: an accessibility label of "Save" against
	// an OCR read of "Save..." or "&Save" is agreement, not conflict.
	if strings.Contains(ta, tb) || strings.Contains(tb, ta) {
		return 0.8
	}
	return -1
}

// textOf is an observation's comparable text, normalised. Label is preferred over
// Text because a source that assigned a label is making a claim about what the text
// belongs to, where OCR is only reporting glyphs.
func textOf(o directorapi.Observation) string {
	s := o.Label
	if s == "" {
		s = o.Text
	}
	return normalise(s)
}

// normalise is the ONE definition of comparable text, shared with the evidence layer.
//
// Clustering, identity matching and text fusion all have to agree about what makes two
// labels the same. When each had its own copy they were identical by luck and one
// comment; the moment OCR arrived they would have diverged, and an element that merged
// one moment would have failed to match the next for reasons no diagnostic could show.
func normalise(s string) string { return observation.Normalize(s) }

// rolesCompatible reports whether two roles could describe the same object.
//
// Sources disagree about role legitimately: OCR calls everything text, a detector
// may see a "button" where accessibility knows a menu item. So an unknown or
// generic-text role is compatible with anything, and two DIFFERENT specific roles
// are not — a button and a checkbox in the same place are two controls.
func rolesCompatible(a, b directorapi.ElementRole) bool {
	if a == b {
		return true
	}
	if generic(a) || generic(b) {
		return true
	}
	// Roles that routinely disagree between sources for the same object.
	pairs := [][2]directorapi.ElementRole{
		{directorapi.RoleButton, directorapi.RoleMenuItem},
		{directorapi.RoleButton, directorapi.RoleIcon},
		{directorapi.RoleButton, directorapi.RoleToggle},
		{directorapi.RoleListItem, directorapi.RoleRow},
		{directorapi.RoleImage, directorapi.RoleIcon},
		{directorapi.RoleTab, directorapi.RoleListItem},
	}
	for _, p := range pairs {
		if (a == p[0] && b == p[1]) || (a == p[1] && b == p[0]) {
			return true
		}
	}
	return false
}

// generic reports whether a role carries no real claim about what the object is.
//
// ONE definition, and it lives in directorapi because the durable place signature asks the same
// question about the same roles — see ElementRole.Generic. Two copies would let an element be
// given its kind under one rule and counted under another.
func generic(r directorapi.ElementRole) bool { return r.Generic() }

// build turns a cluster into an element. The cluster is already strongest-source
// first, so "the first observation that supplies a field" is exactly "the most
// authoritative source that knows it".
//
// Where a weaker source DISAGREES rather than merely being later, the disagreement is
// recorded as a Conflict. Silently discarding it would be the tempting thing and the
// wrong one: two sources naming the same control differently is the single most
// informative signal there is about a perception bug, and it is invisible from the
// finished element, which shows only the winner.
func build(cluster []directorapi.Observation, cycle observation.CycleID,
	rec *recorder, ci int) (Fused, []Conflict) {

	primary := cluster[0]
	el := &directorapi.Element{
		WindowID: primary.WindowID,
		Role:     primary.Role,
	}

	f := Fused{Element: el, Observations: cluster}
	var conflicts []Conflict
	note := func(field string, winner, loser directorapi.Observation, won, lost string) {
		conflicts = append(conflicts, Conflict{
			Field:       field,
			Winner:      winner.Reference(string(cycle)),
			Loser:       loser.Reference(string(cycle)),
			WinnerValue: won,
			LoserValue:  lost,
		})
		rec.overrule(ci, field, string(loser.Source)+" said "+lost)
	}
	// chose records which observation supplied a field. The reason is phrased from the
	// rule that actually applied — first non-empty from the strongest source — rather
	// than restated per field, because that IS the rule and there is only one.
	chose := func(field, value string, o directorapi.Observation, why string) {
		rec.field(ci, explain.FieldChoice{
			Field: field, Value: value,
			From:   o.Reference(string(cycle)),
			Reason: why,
		})
	}
	// Who supplied each contested field, so a conflict can name the observation it
	// actually lost to rather than just the cluster's primary.
	//
	// Seeded with the primary for the fields the element was CONSTRUCTED with. Without
	// this a role conflict would name an empty observation as the winner: the loop
	// below only records a supplier when it sets the field, and the primary's role was
	// already in place before the loop began.
	from := map[string]directorapi.Observation{}
	if !generic(primary.Role) {
		from["role"] = primary
	}

	// Which state flags have already been claimed by a source. Tracked here rather
	// than on the Element because "has anyone reported this yet" is a fact about the
	// fusion in progress, not a property of the finished element.
	claimed := map[string]bool{}

	for _, o := range cluster {
		if el.Role == "" || el.Role == directorapi.RoleUnknown {
			if !generic(o.Role) {
				el.Role = o.Role
				from["role"] = o
				chose("role", string(o.Role), o,
					"the first source to make a specific claim about what this is; "+
						"a generic role is not a claim")
			}
		} else if !generic(o.Role) && o.Role != el.Role {
			note("role", from["role"], o, string(el.Role), string(o.Role))
		}

		if el.Label == "" {
			if o.Label != "" {
				el.Label = o.Label
				from["label"] = o
				chose("label", o.Label, o,
					"the strongest source that assigned a label — a label is a claim about "+
						"what the text belongs to, where raw text is only glyphs")
			} else if o.Text != "" {
				el.Label = o.Text
				from["label"] = o
				chose("label", o.Text, o,
					"no source assigned a label, so the recognised text was used instead")
			}
		} else if t := textOf(o); t != "" && t != normalise(el.Label) {
			// Normalised, so "&Save" against "Save" is agreement. What reaches here is
			// two sources reading genuinely different words in one place — which is
			// either a merge that should not have happened or a source misreading.
			note("label", from["label"], o, el.Label, firstText(o))
		}

		if el.Value == "" {
			el.Value = o.Value
			if o.Value != "" {
				from["value"] = o
			}
		} else if o.Value != "" && o.Value != el.Value {
			note("value", from["value"], o, el.Value, o.Value)
		}

		if el.Description == "" {
			el.Description = o.Description
		}
		if el.Bounds.Empty() {
			el.Bounds = o.Bounds
			from["bounds"] = o
			chose("bounds", rectText(o.Bounds), o,
				"structure outranks pixels: the strongest source's box is the control's "+
					"real hit area, where a weaker one may be the text drawn inside it")
		}
		if el.WindowID == "" {
			el.WindowID = o.WindowID
		}
		if f.NativeID == "" && o.NativeID != "" {
			f.NativeID = o.NativeID
			f.ParentNativeID = o.ParentNativeID
			// Also recorded on the element itself. Acting on an element sometimes
			// means asking its SOURCE to do something — focusing, for instance, which
			// no input event can express — and the source knows it only by the id it
			// originally reported. Without this the Director could identify an
			// element perfectly and still have no way to name it back.
			if el.Attributes == nil {
				el.Attributes = map[string]any{}
			}
			el.Attributes["native_id"] = o.NativeID
		}

		// State is taken from the first source that actually REPORTED it. A nil here
		// means the source could not know — OCR cannot see an enabled flag — and
		// treating that silence as "false" would hide usable targets.
		applyState(el, o, claimed)

		// Provenance. Every element records the evidence it was believed into
		// existence from — the whole point of separating the two.
		el.Provenance.Add(o.Reference(string(cycle)))
		if !el.HasSource(o.Source) {
			el.Sources = append(el.Sources, o.Source)
		}

		// The object the control STANDS FOR, from the first source that knew.
		//
		// First rather than strongest, and the two are the same thing here: the sources
		// are visited strongest-first, and only a source that positively established an
		// identity reports one at all. A later source cannot overwrite it, because two
		// sources disagreeing about which file a control is would be a disagreement to
		// refuse rather than to resolve by rank — and the second one having nothing to
		// say (the ordinary case) must not erase the first one's answer.
		if el.Resource == nil && o.Resource.Known() {
			el.Resource = o.Resource.Clone()
		}
		// The application's own account of what this control is, under the same rule and
		// for the same reason: first source that KNEW wins, and a source with nothing to
		// say cannot erase an answer another one established.
		if el.Entity == nil && o.Entity.Known() {
			el.Entity = o.Entity.Clone()
		}

		for k, v := range o.Attributes {
			if el.Attributes == nil {
				el.Attributes = map[string]any{}
			}
			if _, exists := el.Attributes[k]; !exists {
				el.Attributes[k] = v
			}
		}
	}

	// Defaults for what nobody could report. Enabled and Visible default to TRUE
	// because a purely visual observation cannot see either, and defaulting to false
	// would make every OCR-only element invisible to targeting. The doubt is carried
	// by Confidence and by Sources, which policy consults before anything
	// destructive — not by pretending the element is unusable.
	if !anyReported(cluster, func(o directorapi.Observation) *bool { return o.Enabled }) {
		el.Enabled = true
	}
	if !anyReported(cluster, func(o directorapi.Observation) *bool { return o.Visible }) {
		el.Visible = !el.Bounds.Empty()
	}

	conf, derivation := clusterConfidence(cluster)
	el.Confidence = conf
	rec.setConfidence(ci, derivation)
	return f, conflicts
}

// rectText renders bounds for an explanation.
func rectText(r directorapi.Rect) string {
	return "(" + itoa(r.X) + "," + itoa(r.Y) + " " + itoa(r.Width) + "x" + itoa(r.Height) + ")"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// firstText is the text an observation actually carries, un-normalised, for reporting.
func firstText(o directorapi.Observation) string {
	if o.Label != "" {
		return o.Label
	}
	return o.Text
}

// applyState copies the reported state flags from o. First reporter wins, and since
// the cluster is ordered strongest-source-first, that is the most authoritative one.
//
// It also RECORDS which source claimed each flag, in StateEvidence. That record is what
// lets a later visual pass tell "structure said false" from "structure said nothing" —
// two situations a bare boolean cannot distinguish, and which call for opposite
// behaviour: the first must not be overridden by appearance, the second is exactly the
// gap appearance may fill.
func applyState(el *directorapi.Element, o directorapi.Observation, claimed map[string]bool) {
	claim := func(name string, reported *bool, dst *bool) {
		if reported == nil || claimed[name] {
			return
		}
		*dst = *reported
		claimed[name] = true
		if el.StateEvidence == nil {
			el.StateEvidence = map[string]directorapi.StateFact{}
		}
		el.StateEvidence[name] = directorapi.StateFact{
			Value: *reported, Source: o.Source, Confidence: 1, Observation: o.ID,
		}
	}
	claim("enabled", o.Enabled, &el.Enabled)
	claim("visible", o.Visible, &el.Visible)
	claim("focused", o.Focused, &el.Focused)
	claim("selected", o.Selected, &el.Selected)

	if off, ok := o.Attributes["offscreen"].(bool); ok && off {
		el.Offscreen = true
	}
}

func anyReported(cluster []directorapi.Observation, get func(directorapi.Observation) *bool) bool {
	for _, o := range cluster {
		if get(o) != nil {
			return true
		}
	}
	return false
}

// Per-source base confidence: how much the Director trusts a lone observation from
// this source. These are the ladder expressed as numbers — SourceRank gives the
// total order, this gives the magnitudes that arithmetic needs.
func baseConfidence(s directorapi.ObservationSource) float64 {
	switch s {
	case directorapi.SourceNative:
		return 0.98
	case directorapi.SourceDOM:
		return 0.95
	case directorapi.SourceAccessibility:
		return 0.90
	case directorapi.SourceWindowSystem:
		return 0.90
	case directorapi.SourcePlugin:
		return 0.70
	case directorapi.SourceOCR:
		return 0.60
	case directorapi.SourceVision:
		return 0.50
	case directorapi.SourceModel:
		return 0.40
	}
	return 0.30
}

// corroborationWeight is how much of the remaining doubt an additional independent
// source can remove.
const corroborationWeight = 0.7

// maxConfidence caps fusion's output below 1. The Director is reasoning about a
// live desktop from a snapshot that is already milliseconds stale; certainty is not
// available, and a 1.0 would let a threshold comparison elsewhere read as proof.
const maxConfidence = 0.99

// clusterConfidence combines a cluster's evidence.
//
// The model is deliberately simple and monotone: start from the strongest source's
// base confidence (scaled by that observation's own certainty), then let each
// additional INDEPENDENT source remove a fraction of the remaining doubt. More
// corroboration always helps and never hurts; no amount of weak corroboration
// reaches the confidence of one strong source alone.
// It also returns its own derivation, term by term. A confidence of 0.90 from one
// accessibility observation and a 0.90 from three weak sources agreeing are different
// findings that a threshold comparison downstream cannot tell apart, and the number
// alone gives a reader no way to check the arithmetic or the reasoning.
func clusterConfidence(cluster []directorapi.Observation) (float64, explain.ConfidenceExplanation) {
	best := 0.0
	var strongest directorapi.Observation
	for _, o := range cluster {
		if c := weighted(o); c > best {
			best, strongest = c, o
		}
	}

	derivation := explain.ConfidenceExplanation{Base: best, Total: best}
	if strongest.Source != "" {
		derivation.Contributions = append(derivation.Contributions, explain.ConfidenceContribution{
			Source: string(strongest.Source),
			Delta:  0,
			Reason: "base trust in " + string(strongest.Source) + " (" +
				pct(baseConfidence(strongest.Source)) + "), scaled by its own certainty in this " +
				"observation (" + pct(selfCertainty(strongest)) + ")",
		})
	}

	conf := best
	for _, o := range cluster {
		c := weighted(o)
		if c == best {
			best = -1 // consume the strongest exactly once
			continue
		}
		delta := (1 - conf) * corroborationWeight * c
		conf += delta
		derivation.Add(string(o.Source), delta,
			"an independent "+string(o.Source)+" observation agreeing, removing "+
				pct(corroborationWeight)+" of the remaining doubt")
	}
	if conf > maxConfidence {
		derivation.Add("cap", maxConfidence-conf,
			"capped below 1: the Director reasons about a live desktop from a snapshot "+
				"that is already stale, and certainty is not available")
		return maxConfidence, derivation
	}
	derivation.Total = conf
	return conf, derivation
}

// weighted is one observation's contribution: the source's base trust scaled by the
// source's own reported certainty in this particular observation.
func weighted(o directorapi.Observation) float64 {
	return baseConfidence(o.Source) * selfCertainty(o)
}

// selfCertainty is how sure the source said it was, clamped. A source that reports no
// per-item confidence is not thereby uncertain — accessibility never scores its nodes.
func selfCertainty(o directorapi.Observation) float64 {
	c := o.Confidence
	if c <= 0 {
		return 1
	}
	if c > 1 {
		return 1
	}
	return c
}

// pct renders a 0..1 value as a percentage, for explanation prose.
func pct(v float64) string {
	return itoa(int(v*100+0.5)) + "%"
}
