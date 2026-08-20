package observe

import (
	"fmt"
	"sort"
	"strings"
)

// Measuring what a detector is WORTH, rather than how much it says.
//
// The obvious number is detections per frame, and it is close to useless. A live
// three-minute session against a real game produced 52 samples, one stable entity, and more
// than a hundred anonymous ones that flickered in and out — a detector working hard and
// telling nobody anything. Counting boxes would have scored that highly.
//
// So these metrics ask the questions the rest of the Director actually depends on:
//
//	does anything STAY?              stable entities, presence
//	can anything be NAMED?           structural roles, safe-label opportunity
//	is it the SAME thing next frame? flicker, spatial consistency
//	do changes MEAN anything?        transition utility
//
// A model that produces fewer believable structures beats one that produces more anonymous
// boxes, and that ordering is what these numbers have to express.
//
// # What is a proxy and what is measured
//
// Several of these are honest proxies rather than ground truth, and are named as such in
// their comments. Nothing here is a labelled dataset, so "false structure" cannot be
// measured directly — what CAN be measured is how much of what a detector called structure
// failed to persist, which is the same signal from the other side.

// Metrics is one backend's semantic usefulness over one session.
type Metrics struct {
	// Backend and Model identify what produced the evidence.
	Backend string `json:"backend"`
	Model   string `json:"model,omitempty"`

	Samples int `json:"samples"`

	// StableEntities survived the stability thresholds; Unstable did not. The RATIO
	// between them matters more than either alone.
	StableEntities   int `json:"stable_entities"`
	UnstableEntities int `json:"unstable_entities"`

	// AnonymousRatio is the share of tracked entities carrying no readable label at all.
	// A detector whose output is entirely anonymous can support geometry and nothing
	// else — no request can name what it found.
	AnonymousRatio float64 `json:"anonymous_ratio"`

	// StructuralRoleCoverage is the share of entities whose role establishes STRUCTURE
	// rather than decoration. Structure is what fusion can attach text to and what
	// policy can reason about; an icon is neither.
	StructuralRoleCoverage float64 `json:"structural_role_coverage"`

	// SafeLabelOpportunity is the share of entities whose role makes them eligible to
	// hold plaintext under the privacy rules. This is the number that decides whether a
	// session can produce readable names at all.
	SafeLabelOpportunity float64 `json:"safe_label_opportunity"`

	// FlickerRate is appearances-after-absence per observation. High flicker with high
	// presence is the signature of a detector struggling rather than an interface
	// changing.
	FlickerRate float64 `json:"flicker_rate"`

	// Transitions is how many were recorded at all. Carried so a verdict can tell "the
	// changes were meaningless" from "nothing changed", which are opposite findings.
	Transitions int `json:"transitions"`

	// TransitionUtility is the share of transitions that describe something an
	// interpreter could use — a structure appearing, a grid forming, a label changing —
	// as opposed to boxes drifting. A session of pure movement has learned nothing.
	TransitionUtility float64 `json:"transition_utility"`

	// UnstableStructureRatio is how much of what the detector called STRUCTURE failed to
	// persist. A proxy for false structure: without ground truth nobody can say a box
	// was wrong, but a panel that exists in a third of frames was not a panel.
	UnstableStructureRatio float64 `json:"unstable_structure_ratio"`

	// RolesSeen counts each role the backend produced, so a reader can see the
	// vocabulary rather than infer it.
	RolesSeen map[string]int `json:"roles_seen,omitempty"`

	// Dropped is evidence discarded to stay bounded — itself a finding, since a scene
	// too noisy to track is a fact about the detector.
	DroppedEntities    int `json:"dropped_entities,omitempty"`
	DroppedTransitions int `json:"dropped_transitions,omitempty"`
}

// structuralRoles establish interface structure.
//
// Wider than the plaintext allowlist deliberately: a pane is real structure that fusion can
// attach things to, but its "label" is whatever text sits inside it, so it establishes
// structure without earning plaintext. The two questions are different and the two sets
// should not be conflated.
var structuralRoles = map[string]bool{
	"button": true, "menu": true, "menu_item": true, "tab": true, "tab_list": true,
	"checkbox": true, "radio": true, "dialog": true, "pane": true, "group": true,
	"list": true, "list_item": true, "table": true, "slider": true, "progress_bar": true,
	"combo_box": true, "text_box": true, "edit": true, "toolbar": true,
}

// meaningfulTransitions describe something an interpreter could act on knowing.
var meaningfulTransitions = map[Kind]bool{
	EntityAppeared: true, EntityDisappeared: true, EntityLabelChanged: true,
	EntityStateChanged: true, GridAppeared: true, GridDisappeared: true,
	GridShapeChanged: true, MenuLikeAppeared: true, MenuLikeDisappeared: true,
	HUDLikeAppeared: true, HUDLikeDisappeared: true,
	WindowGenerationChanged: true, TargetLost: true, TargetReacquired: true,
}

// Measure computes a backend's semantic usefulness from a session's findings.
//
// Deterministic and pure, so the same fixture always produces the same report and two
// backends can be compared without either of them being present.
func Measure(backend, model string, f Findings) Metrics {
	m := Metrics{
		Backend: backend, Model: model, Samples: f.Samples,
		StableEntities: len(f.Stable), UnstableEntities: len(f.Unstable),
		RolesSeen:          map[string]int{},
		DroppedEntities:    f.DroppedEntities,
		DroppedTransitions: f.DroppedTransitions,
	}

	all := make([]Stability, 0, len(f.Stable)+len(f.Unstable))
	all = append(all, f.Stable...)
	all = append(all, f.Unstable...)
	if len(all) == 0 {
		return m
	}

	anonymous, structural, eligible := 0, 0, 0
	flickers, observations := 0, 0
	unstableStructural, totalStructural := 0, 0

	for _, s := range all {
		role := baseRole(s.Role)
		m.RolesSeen[role]++

		if s.Label.Empty() {
			anonymous++
		}
		if structuralRoles[role] {
			structural++
			totalStructural++
		}
		if roleEligibleForPlaintext(role) {
			eligible++
		}
		flickers += s.Flickers
		observations += s.SamplesSeen
	}
	for _, s := range f.Unstable {
		if structuralRoles[baseRole(s.Role)] {
			unstableStructural++
		}
	}

	n := float64(len(all))
	m.AnonymousRatio = float64(anonymous) / n
	m.StructuralRoleCoverage = float64(structural) / n
	m.SafeLabelOpportunity = float64(eligible) / n
	if observations > 0 {
		m.FlickerRate = float64(flickers) / float64(observations)
	}
	if totalStructural > 0 {
		m.UnstableStructureRatio = float64(unstableStructural) / float64(totalStructural)
	}

	m.Transitions = len(f.Transitions)
	if len(f.Transitions) > 0 {
		useful := 0
		for _, t := range f.Transitions {
			if meaningfulTransitions[t.Kind] {
				useful++
			}
		}
		m.TransitionUtility = float64(useful) / float64(len(f.Transitions))
	}
	return m
}

// baseRole strips the shape suffix a grid's role carries ("grid 2x3" → "grid").
func baseRole(role string) string {
	if i := strings.IndexByte(role, ' '); i > 0 {
		return role[:i]
	}
	return role
}

// roleEligibleForPlaintext mirrors the privacy allowlist.
//
// Deliberately duplicated as a STRING check rather than importing the typed set, because
// this measures what a backend could offer rather than deciding what may be stored. If the
// two ever disagree the metric is wrong, not the policy — which is the right way round.
func roleEligibleForPlaintext(role string) bool {
	switch role {
	case "button", "menu_item", "menu", "tab", "checkbox", "radio":
		return true
	}
	return false
}

// Thresholds a backend must clear to be a usable default.
//
// PROVISIONAL, and set from the one real measurement available rather than from taste: the
// baseline detector scored far below all of them on a live game, so these are "clearly
// better than what plainly does not work" rather than "known to be sufficient". They should
// move the moment a second real backend is measured.
type SelectionThresholds struct {
	MinStableEntities       int     `json:"min_stable_entities"`
	MaxAnonymousRatio       float64 `json:"max_anonymous_ratio"`
	MinStructuralCoverage   float64 `json:"min_structural_role_coverage"`
	MinSafeLabelOpportunity float64 `json:"min_safe_label_opportunity"`
	MaxFlickerRate          float64 `json:"max_flicker_rate"`
	MinTransitionUtility    float64 `json:"min_transition_utility"`
	MaxUnstableStructure    float64 `json:"max_unstable_structure_ratio"`
}

// DefaultSelectionThresholds are the provisional bar.
func DefaultSelectionThresholds() SelectionThresholds {
	return SelectionThresholds{
		MinStableEntities:       5,
		MaxAnonymousRatio:       0.60,
		MinStructuralCoverage:   0.25,
		MinSafeLabelOpportunity: 0.15,
		MaxFlickerRate:          0.15,
		MinTransitionUtility:    0.35,
		MaxUnstableStructure:    0.50,
	}
}

// Verdict is whether a backend clears the bar, and what it failed on.
type Verdict struct {
	Backend  string   `json:"backend"`
	Suitable bool     `json:"suitable"`
	Failures []string `json:"failures,omitempty"`
}

// Assess judges one backend against the thresholds.
//
// Reports EVERY failure rather than the first. A backend that misses one number by a little
// is a different proposition from one that misses six, and a reader deciding what to try
// next needs to see which.
func Assess(m Metrics, t SelectionThresholds) Verdict {
	v := Verdict{Backend: m.Backend, Suitable: true}
	fail := func(format string, args ...any) {
		v.Suitable = false
		v.Failures = append(v.Failures, fmt.Sprintf(format, args...))
	}

	if m.StableEntities < t.MinStableEntities {
		fail("only %d stable entities (want %d+): almost nothing persisted, so there is "+
			"nothing durable to build a rule on", m.StableEntities, t.MinStableEntities)
	}
	if m.AnonymousRatio > t.MaxAnonymousRatio {
		fail("%.0f%% of entities are anonymous (want under %.0f%%): geometry without names, "+
			"so no request can refer to what was found",
			m.AnonymousRatio*100, t.MaxAnonymousRatio*100)
	}
	if m.StructuralRoleCoverage < t.MinStructuralCoverage {
		fail("%.0f%% structural roles (want %.0f%%+): the output is decoration rather than "+
			"interface, and fusion has nothing to attach text to",
			m.StructuralRoleCoverage*100, t.MinStructuralCoverage*100)
	}
	if m.SafeLabelOpportunity < t.MinSafeLabelOpportunity {
		fail("%.0f%% of entities could hold a plaintext label (want %.0f%%+): under the "+
			"privacy rules a session would produce almost no readable names",
			m.SafeLabelOpportunity*100, t.MinSafeLabelOpportunity*100)
	}
	if m.FlickerRate > t.MaxFlickerRate {
		fail("flicker %.2f per observation (want under %.2f): identities do not survive "+
			"between frames, so temporal analysis has nothing to track",
			m.FlickerRate, t.MaxFlickerRate)
	}
	// Only judged when something CHANGED. A session in which nothing happened has no
	// transitions, and scoring that as "the transitions were useless" confuses an
	// absence of change with a failure to describe one.
	if m.Transitions > 0 && m.TransitionUtility < t.MinTransitionUtility {
		fail("%.0f%% of transitions are meaningful (want %.0f%%+): mostly boxes drifting",
			m.TransitionUtility*100, t.MinTransitionUtility*100)
	}
	if m.UnstableStructureRatio > t.MaxUnstableStructure {
		fail("%.0f%% of claimed structure did not persist (want under %.0f%%): a panel that "+
			"exists in a third of frames was not a panel",
			m.UnstableStructureRatio*100, t.MaxUnstableStructure*100)
	}
	return v
}

// Compare renders several backends side by side.
//
// A table, because the whole point is relative judgement: no single number here means
// anything on its own, and the decision is always "which of these".
func Compare(all []Metrics, t SelectionThresholds) string {
	if len(all) == 0 {
		return "No backends were measured.\n"
	}
	var b strings.Builder

	b.WriteString("Semantic usefulness — higher is better except where marked\n\n")
	fmt.Fprintf(&b, "  %-26s", "METRIC")
	for _, m := range all {
		fmt.Fprintf(&b, " %14s", truncateTo(m.Backend, 14))
	}
	b.WriteString("\n")

	row := func(name string, get func(Metrics) string) {
		fmt.Fprintf(&b, "  %-26s", name)
		for _, m := range all {
			fmt.Fprintf(&b, " %14s", get(m))
		}
		b.WriteString("\n")
	}
	row("samples", func(m Metrics) string { return fmt.Sprint(m.Samples) })
	row("stable entities", func(m Metrics) string { return fmt.Sprint(m.StableEntities) })
	row("unstable entities", func(m Metrics) string { return fmt.Sprint(m.UnstableEntities) })
	row("anonymous ratio (lower)", func(m Metrics) string { return pct(m.AnonymousRatio) })
	row("structural roles", func(m Metrics) string { return pct(m.StructuralRoleCoverage) })
	row("safe-label opportunity", func(m Metrics) string { return pct(m.SafeLabelOpportunity) })
	row("flicker (lower)", func(m Metrics) string { return fmt.Sprintf("%.2f", m.FlickerRate) })
	row("transition utility", func(m Metrics) string { return pct(m.TransitionUtility) })
	row("unstable structure (lower)", func(m Metrics) string { return pct(m.UnstableStructureRatio) })

	b.WriteString("\nVerdicts\n")
	for _, m := range all {
		v := Assess(m, t)
		if v.Suitable {
			fmt.Fprintf(&b, "\n  %s — clears every threshold\n", m.Backend)
			continue
		}
		fmt.Fprintf(&b, "\n  %s — NOT suitable as a default\n", m.Backend)
		for _, f := range v.Failures {
			fmt.Fprintf(&b, "     • %s\n", f)
		}
	}

	b.WriteString("\nVocabulary produced\n")
	for _, m := range all {
		fmt.Fprintf(&b, "  %-14s", truncateTo(m.Backend, 14))
		names := make([]string, 0, len(m.RolesSeen))
		for r := range m.RolesSeen {
			names = append(names, r)
		}
		sort.Strings(names)
		if len(names) == 0 {
			b.WriteString(" (nothing)")
		}
		for _, r := range names {
			fmt.Fprintf(&b, " %s=%d", r, m.RolesSeen[r])
		}
		b.WriteString("\n")
	}
	return b.String()
}

func pct(f float64) string { return fmt.Sprintf("%.0f%%", f*100) }

func truncateTo(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
