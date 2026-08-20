package observe

import (
	"fmt"
	"sort"
)

// Turning a timeline into questions worth asking, and never into answers.
//
// A hypothesis here is a suggestion about where to look next, carrying its own support, its
// own contradictions, and a concrete way to test it. That last part is what keeps it a
// hypothesis: "this may be a resource meter, and you could find out by using some of the
// resource and watching whether it changes" invites verification. "Boost meter detected"
// forecloses it.
//
// Nothing in this file names a game. The evidence is geometric and temporal — a thing that
// is present while other things change, a panel that appears with several readable labels
// inside it — and what any of that MEANS belongs to a capability pack that can cite it.

// Insight is one candidate concept, with everything needed to doubt it.
type Insight struct {
	// Kind is drawn from a closed vocabulary, all of which are hedged: nothing here
	// asserts, and the strings themselves say so.
	Kind Concept `json:"kind"`
	// Confidence is how well the evidence supports the SHAPE, not how sure the meaning
	// is. There is no field for that, deliberately.
	Confidence float64 `json:"confidence"`

	SupportingSamples     int            `json:"supporting_samples"`
	SupportingEntities    []Digest       `json:"supporting_entities,omitempty"`
	SupportingTransitions []TransitionID `json:"supporting_transitions,omitempty"`

	// Contradictions are the reasons to doubt it, and are never empty when any exist —
	// a hypothesis presented with only its supporting evidence is advocacy.
	Contradictions []string `json:"contradictions,omitempty"`
	// Validation is what a person could do next to settle it. Required.
	Validation string `json:"validation"`
	// Observed is the plain evidence sentence this rests on, so a reader can see the
	// finding and the guess separately.
	Observed string `json:"observed"`
}

// Concept is the closed vocabulary of candidate concepts.
//
// Every value begins "possible". That is not timidity; it is the type telling the truth
// about what a timeline can establish.
type Concept string

const (
	PossibleMenu          Concept = "possible_menu_like_panel"
	PossiblePersistentHUD Concept = "possible_persistent_hud_element"
	PossibleChangingMeter Concept = "possible_changing_meter_or_counter"
	PossibleGrid          Concept = "possible_item_grid"
	PossibleModeChange    Concept = "possible_mode_change"
	PossibleTransientOver Concept = "possible_transient_overlay"
)

// InsightThresholds bound hypothesis generation.
type InsightThresholds struct {
	// MinPanelLabels is how many readable labels must share a region before it is
	// worth calling menu-like.
	MinPanelLabels int
	// MinPersistence is the presence ratio at which something counts as always there.
	MinPersistence float64
	// MinChanges is how many changes make a region interesting rather than noisy.
	MinChanges int
	// MaxCandidates bounds the output.
	MaxCandidates int
}

// DefaultInsightThresholds are the provisional defaults.
func DefaultInsightThresholds() InsightThresholds {
	return InsightThresholds{
		MinPanelLabels: 3,
		MinPersistence: 0.85,
		MinChanges:     3,
		MaxCandidates:  DefaultMaxHypotheses,
	}
}

// Insights derives candidate concepts from findings.
//
// Deterministic: the same findings always produce the same candidates in the same order, so
// a fixture replays exactly and a change in output means a change in evidence.
func Insights(f Findings, t InsightThresholds) []Insight {
	var out []Insight

	out = append(out, menuLike(f, t)...)
	out = append(out, persistent(f, t)...)
	out = append(out, changing(f, t)...)
	out = append(out, grids(f, t)...)
	out = append(out, modeChanges(f, t)...)

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		return out[i].Kind < out[j].Kind
	})
	if t.MaxCandidates > 0 && len(out) > t.MaxCandidates {
		out = out[:t.MaxCandidates]
	}
	return out
}

// menuLike looks for several readable labels sharing one part of the screen.
//
// The shape of a menu, without the word. Four labelled controls stacked in one place is a
// panel whatever the application is; that it might be a PAUSE menu is a pack's guess, and
// this does not make it.
func menuLike(f Findings, t InsightThresholds) []Insight {
	byArea := map[string][]Stability{}
	for _, s := range f.Stable {
		if s.Label.Empty() || s.Label.Text == "" {
			continue
		}
		byArea[describeRegion(s.Region)] = append(byArea[describeRegion(s.Region)], s)
	}

	areas := make([]string, 0, len(byArea))
	for a := range byArea {
		areas = append(areas, a)
	}
	sort.Strings(areas)

	var out []Insight
	for _, area := range areas {
		group := byArea[area]
		if len(group) < t.MinPanelLabels {
			continue
		}
		ids := make([]Digest, 0, len(group))
		lowest := 1.0
		for _, s := range group {
			ids = append(ids, s.Identity)
			if s.PresenceRatio < lowest {
				lowest = s.PresenceRatio
			}
		}
		in := Insight{
			Kind:              PossibleMenu,
			Confidence:        clamp01(0.4 + 0.1*float64(len(group))),
			SupportingSamples: f.Samples, SupportingEntities: ids,
			Observed: fmt.Sprintf("%d readable labels shared %s across the session",
				len(group), area),
			Validation: "open and close the panel deliberately and re-run; a menu should " +
				"appear and disappear together with its labels",
		}
		if lowest < 0.95 {
			in.Contradictions = append(in.Contradictions,
				fmt.Sprintf("the labels were not all present in every sample (weakest %.0f%%)",
					lowest*100))
		}
		out = append(out, in)
	}
	return out
}

// persistent looks for things that were essentially always there.
func persistent(f Findings, t InsightThresholds) []Insight {
	var out []Insight
	for _, s := range f.Stable {
		if s.PresenceRatio < t.MinPersistence || s.StateChanges >= t.MinChanges {
			continue
		}
		in := Insight{
			Kind:               PossiblePersistentHUD,
			Confidence:         clamp01(s.PresenceRatio * 0.8),
			SupportingSamples:  s.SamplesSeen,
			SupportingEntities: []Digest{s.Identity},
			Observed: fmt.Sprintf("a %s was present in %.0f%% of samples, at %s",
				s.Role, s.PresenceRatio*100, describeRegion(s.Region)),
			Validation: "change game mode or open a full-screen menu; a HUD element " +
				"should disappear while a decoration of the scene may not",
		}
		if s.Label.Empty() {
			in.Contradictions = append(in.Contradictions,
				"it carried no readable label, so what it represents is unknown")
		}
		if s.ConfidenceMin < 0.5 {
			in.Contradictions = append(in.Contradictions,
				fmt.Sprintf("its weakest observation was only %.2f confident", s.ConfidenceMin))
		}
		out = append(out, in)
	}
	return out
}

// changing looks for something present throughout whose contents kept changing.
func changing(f Findings, t InsightThresholds) []Insight {
	var out []Insight
	for _, s := range f.Stable {
		if s.StateChanges < t.MinChanges && countLabelChanges(f, s.Identity) < t.MinChanges {
			continue
		}
		changes := s.StateChanges + countLabelChanges(f, s.Identity)
		out = append(out, Insight{
			Kind:                  PossibleChangingMeter,
			Confidence:            clamp01(0.35 + 0.05*float64(changes)),
			SupportingSamples:     s.SamplesSeen,
			SupportingEntities:    []Digest{s.Identity},
			SupportingTransitions: transitionsFor(f, s.Identity),
			Observed: fmt.Sprintf("a %s at %s stayed present and changed %d times",
				s.Role, describeRegion(s.Region), changes),
			Contradictions: []string{
				"a value that changes is not necessarily a resource; scene motion behind a " +
					"transparent element produces the same evidence",
			},
			Validation: "cause a known change deliberately — spend, collect or consume " +
				"something — and check this region changes at the same moment",
		})
	}
	return out
}

// grids reports repeated regular arrangements.
func grids(f Findings, t InsightThresholds) []Insight {
	var out []Insight
	for _, s := range f.Stable {
		if len(s.Role) < 4 || s.Role[:4] != "grid" {
			continue
		}
		out = append(out, Insight{
			Kind:               PossibleGrid,
			Confidence:         clamp01(s.PresenceRatio * 0.7),
			SupportingSamples:  s.SamplesSeen,
			SupportingEntities: []Digest{s.Identity},
			Observed: fmt.Sprintf("a %s was present in %.0f%% of samples at %s",
				s.Role, s.PresenceRatio*100, describeRegion(s.Region)),
			Contradictions: []string{
				"regular geometry is not by itself an inventory; toolbars and tile pickers " +
					"look identical to a detector",
			},
			Validation: "add or remove something from the collection this might represent " +
				"and check the cell count or contents change",
		})
	}
	return out
}

// modeChanges reports layouts that replaced one another.
func modeChanges(f Findings, t InsightThresholds) []Insight {
	appeared, disappeared := 0, 0
	var ids []TransitionID
	for _, tr := range f.Transitions {
		switch tr.Kind {
		case EntityAppeared, GridAppeared, MenuLikeAppeared:
			appeared++
			ids = append(ids, tr.ID)
		case EntityDisappeared, GridDisappeared, MenuLikeDisappeared:
			disappeared++
			ids = append(ids, tr.ID)
		}
	}
	if appeared < t.MinChanges || disappeared < t.MinChanges {
		return nil
	}
	if len(ids) > 12 {
		ids = ids[:12]
	}
	return []Insight{{
		Kind:                  PossibleModeChange,
		Confidence:            0.5,
		SupportingSamples:     f.Samples,
		SupportingTransitions: ids,
		Observed: fmt.Sprintf("%d elements appeared and %d disappeared during the session",
			appeared, disappeared),
		Contradictions: []string{
			"appearing and disappearing is also what an unreliable detector looks like; " +
				"check the unstable list before reading this as a change of screen",
		},
		Validation: "move between two screens deliberately and re-run; a real mode change " +
			"should produce the same pairing at the same moments",
	}}
}

func countLabelChanges(f Findings, id Digest) int {
	n := 0
	for _, t := range f.Transitions {
		if t.Identity == id && t.Kind == EntityLabelChanged {
			n++
		}
	}
	return n
}

func transitionsFor(f Findings, id Digest) []TransitionID {
	var out []TransitionID
	for _, t := range f.Transitions {
		if t.Identity == id {
			out = append(out, t.ID)
		}
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func clamp01(f float64) float64 {
	switch {
	case f < 0:
		return 0
	case f > 1:
		return 1
	}
	return f
}
