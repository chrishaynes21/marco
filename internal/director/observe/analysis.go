package observe

import (
	"fmt"
	"sort"
	"time"
)

// What a timeline can and cannot tell you.
//
// One frame says a circular thing sits in the lower right. Two hundred frames say it is
// there while the player is driving and gone while a menu is up, and that its interior
// changes constantly. The first is a detection; the second is worth building a rule on.
//
// Everything here is deterministic and runs on retained samples, so a session's conclusions
// can be recomputed from a fixture with no game, no detector and no OCR.

// Kind is the closed vocabulary of transitions.
//
// Closed on purpose. Free-form event names become a dozen spellings of the same thing, and
// nothing downstream can switch on them.
type Kind string

const (
	EntityAppeared     Kind = "entity_appeared"
	EntityDisappeared  Kind = "entity_disappeared"
	EntityLabelChanged Kind = "entity_label_changed"
	EntityStateChanged Kind = "entity_state_changed"
	EntityMoved        Kind = "entity_moved"

	GridAppeared     Kind = "grid_appeared"
	GridDisappeared  Kind = "grid_disappeared"
	GridShapeChanged Kind = "grid_shape_changed"

	MenuLikeAppeared    Kind = "menu_like_structure_appeared"
	MenuLikeDisappeared Kind = "menu_like_structure_disappeared"
	HUDLikeAppeared     Kind = "hud_like_structure_appeared"
	HUDLikeDisappeared  Kind = "hud_like_structure_disappeared"

	SceneBecameUnstable Kind = "scene_became_unstable"
	SceneBecameStable   Kind = "scene_became_stable"

	WindowGenerationChanged Kind = "window_generation_changed"
	TargetLost              Kind = "target_unavailable"
	TargetReacquired        Kind = "target_reacquired"
)

// TransitionID identifies one transition within a session.
type TransitionID string

// Transition is one meaningful change, with the evidence for it.
type Transition struct {
	ID       TransitionID `json:"id"`
	Kind     Kind         `json:"kind"`
	At       time.Time    `json:"at"`
	Sequence int          `json:"sequence"`
	Identity Digest       `json:"identity,omitempty"`
	Before   string       `json:"before,omitempty"`
	After    string       `json:"after,omitempty"`
	// Confidence is how sure the session is that this happened at all, which is not the
	// same as how sure it is what it means.
	Confidence float64 `json:"confidence"`
	// Sources names the providers whose evidence supports it.
	Sources []string `json:"sources,omitempty"`
	// Reason is a safe sentence. Never plaintext that failed label classification.
	Reason string `json:"reason"`
}

// Stability is how reliably one entity was present.
type Stability struct {
	Identity Digest    `json:"identity"`
	Role     string    `json:"role"`
	Label    SafeLabel `json:"label"`
	Region   Region    `json:"region"`

	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`

	SamplesSeen   int     `json:"samples_seen"`
	SamplesTotal  int     `json:"samples_total"`
	PresenceRatio float64 `json:"presence_ratio"`

	ConfidenceMin float64 `json:"confidence_min"`
	ConfidenceMax float64 `json:"confidence_max"`

	StateChanges int `json:"state_changes"`
	// Occurrences is how many times this identity was seen in total, which exceeds
	// SamplesSeen when several indistinguishable things share it — a row of identical
	// unnamed icons, say. The difference is what tells one control from nine.
	Occurrences int `json:"occurrences,omitempty"`
	// Flickers counts how often it vanished and came back. A high count with a high
	// presence ratio is the signature of an unreliable detection rather than a thing
	// that keeps being closed and reopened.
	Flickers int `json:"flickers"`
}

// Thresholds decide what counts as stable, and are reported alongside the findings.
//
// Reported because a reader cannot judge "stable" without knowing what it meant, and
// because the numbers are provisional — they came from one live session and should move.
type Thresholds struct {
	// MinSamples is the fewest observations before anything may be called stable.
	MinSamples int `json:"min_samples"`
	// MinPresence is the share of samples an entity must appear in.
	MinPresence float64 `json:"min_presence"`
	// MinConfidence is the floor its weakest observation must clear.
	MinConfidence float64 `json:"min_confidence"`
	// MaxFlickerRatio is how much coming-and-going is tolerated before presence is
	// called unreliable rather than intermittent.
	MaxFlickerRatio float64 `json:"max_flicker_ratio"`
	// RegionTolerance is how far a region may drift and still be the same place.
	RegionTolerance float64 `json:"region_tolerance"`
}

// DefaultThresholds are the provisional defaults.
func DefaultThresholds() Thresholds {
	return Thresholds{
		MinSamples:      8,
		MinPresence:     0.6,
		MinConfidence:   0.4,
		MaxFlickerRatio: 0.35,
		RegionTolerance: 0.02,
	}
}

// Stable reports whether this entity earned the word.
//
// Deliberately strict about the difference between "usually there" and "detected
// inconsistently". An entity seen in 70% of samples across forty separate appearances is
// not a stable feature of the interface; it is a detector struggling.
func (s Stability) Stable(t Thresholds) bool {
	if s.SamplesSeen < t.MinSamples {
		return false
	}
	if s.PresenceRatio < t.MinPresence {
		return false
	}
	if s.ConfidenceMin < t.MinConfidence {
		return false
	}
	if s.SamplesSeen > 0 && float64(s.Flickers)/float64(s.SamplesSeen) > t.MaxFlickerRatio {
		return false
	}
	return true
}

// Analyzer accumulates evidence across samples.
//
// Bounded everywhere: entities are capped, transitions are capped, and the caps are
// reported. An unbounded map keyed by "whatever the detector called it" is how a long
// session against a noisy detector eats a machine.
type Analyzer struct {
	thresholds Thresholds
	bounds     Bounds

	samples     int
	entities    map[Digest]*Stability
	present     map[Digest]bool
	grids       map[Digest]*Stability
	transitions []Transition
	nextID      int
	generation  uint64
	// dropped counts evidence discarded to stay inside the caps.
	droppedEntities    int
	droppedTransitions int
}

// NewAnalyzer returns an analyzer.
func NewAnalyzer(t Thresholds, b Bounds) *Analyzer {
	return &Analyzer{
		thresholds: t, bounds: b,
		entities: map[Digest]*Stability{},
		present:  map[Digest]bool{},
		grids:    map[Digest]*Stability{},
	}
}

// maxTrackedEntities bounds how many distinct things a session will follow.
//
// A detector that produces a new spurious identity every frame would otherwise grow this
// map without limit. When the cap is hit new identities are dropped and counted, which is
// itself a finding: it means the scene was not stable enough to track.
const maxTrackedEntities = 2000

// Observe folds one sample into the accumulated evidence.
func (a *Analyzer) Observe(s Sample) {
	a.samples++

	if a.generation != 0 && s.WindowGeneration != a.generation {
		a.record(Transition{
			Kind: WindowGenerationChanged, At: s.Timestamp, Sequence: s.Sequence,
			Before:     fmt.Sprintf("generation %d", a.generation),
			After:      fmt.Sprintf("generation %d", s.WindowGeneration),
			Confidence: 1,
			Reason: "the window was replaced; evidence either side of this belongs to " +
				"different windows and must not be read as one continuous view",
		})
	}
	a.generation = s.WindowGeneration

	// One count per identity per SAMPLE, not per occurrence.
	//
	// Found by running it: a live session reported "present in 925% of samples". Identity
	// is role + label + coarse quadrant, so a screen with nine unnamed icons in one
	// quadrant collapses them into a single identity — which is intended, since they are
	// indistinguishable — but counting each occurrence made presence a multiple of the
	// sample count and every ratio meaningless.
	//
	// Occurrences are kept separately: several things sharing an identity is itself worth
	// knowing, and it is exactly what distinguishes "one persistent control" from "a row
	// of nine identical ones".
	seen := map[Digest]bool{}
	for _, e := range s.Entities {
		first := !seen[e.Identity]
		seen[e.Identity] = true
		a.foldEntity(e, s, first)
	}
	for _, g := range s.Grids {
		first := !seen[g.Identity]
		seen[g.Identity] = true
		a.foldGrid(g, s, first)
	}

	// Anything present last time and absent now.
	for id := range a.present {
		if seen[id] {
			continue
		}
		if st, ok := a.entities[id]; ok {
			st.Flickers++
			a.record(Transition{
				Kind: EntityDisappeared, At: s.Timestamp, Sequence: s.Sequence,
				Identity: id, Before: describeEntity(st), Confidence: 0.7,
				Reason: "an element that had been present was not observed in this sample",
			})
		} else if st, ok := a.grids[id]; ok {
			st.Flickers++
			a.record(Transition{
				Kind: GridDisappeared, At: s.Timestamp, Sequence: s.Sequence,
				Identity: id, Before: describeEntity(st), Confidence: 0.7,
				Reason: "a grid that had been present was not observed in this sample",
			})
		}
	}
	a.present = seen

	// Totals move for everything tracked, so presence ratios stay comparable.
	for _, st := range a.entities {
		st.SamplesTotal = a.samples
		st.PresenceRatio = float64(st.SamplesSeen) / float64(st.SamplesTotal)
	}
	for _, st := range a.grids {
		st.SamplesTotal = a.samples
		st.PresenceRatio = float64(st.SamplesSeen) / float64(st.SamplesTotal)
	}
}

func (a *Analyzer) foldEntity(e EntitySnapshot, s Sample, firstInSample bool) {
	st, known := a.entities[e.Identity]
	if !known {
		if len(a.entities) >= maxTrackedEntities {
			a.droppedEntities++
			return
		}
		st = &Stability{
			Identity: e.Identity, Role: string(e.Role), Label: e.Label,
			Region: e.Region, FirstSeen: s.Timestamp,
			ConfidenceMin: e.Confidence, ConfidenceMax: e.Confidence,
		}
		a.entities[e.Identity] = st
		a.record(Transition{
			Kind: EntityAppeared, At: s.Timestamp, Sequence: s.Sequence,
			Identity: e.Identity, After: describeEntity(st),
			Confidence: e.Confidence, Sources: e.Sources,
			Reason: "an element was observed that had not been seen before",
		})
	} else if !a.present[e.Identity] {
		// Back after an absence.
		st.Flickers++
	}

	if known {
		if st.Label.Digest != e.Label.Digest {
			a.record(Transition{
				Kind: EntityLabelChanged, At: s.Timestamp, Sequence: s.Sequence,
				Identity: e.Identity,
				Before:   st.Label.Describe(), After: e.Label.Describe(),
				Confidence: e.Label.Confidence,
				Reason:     "the text inside an element changed",
			})
			st.Label = e.Label
		}
		if st.Region.nearlyEqual(e.Region, a.thresholds.RegionTolerance) {
			// Jitter, not movement. Deliberately silent.
		} else {
			a.record(Transition{
				Kind: EntityMoved, At: s.Timestamp, Sequence: s.Sequence,
				Identity: e.Identity,
				Before:   describeRegion(st.Region), After: describeRegion(e.Region),
				Confidence: e.Confidence,
				Reason:     "an element moved further than rendering jitter accounts for",
			})
			st.Region = e.Region
			st.StateChanges++
		}
	}

	st.LastSeen = s.Timestamp
	st.Occurrences++
	if firstInSample {
		st.SamplesSeen++
	}
	if e.Confidence < st.ConfidenceMin {
		st.ConfidenceMin = e.Confidence
	}
	if e.Confidence > st.ConfidenceMax {
		st.ConfidenceMax = e.Confidence
	}
}

func (a *Analyzer) foldGrid(g GridSnapshot, s Sample, firstInSample bool) {
	st, known := a.grids[g.Identity]
	if !known {
		if len(a.grids) >= maxTrackedEntities {
			a.droppedEntities++
			return
		}
		st = &Stability{
			Identity: g.Identity,
			Role:     fmt.Sprintf("grid %dx%d", g.Rows, g.Columns),
			Region:   g.Region, FirstSeen: s.Timestamp,
			ConfidenceMin: g.Fill, ConfidenceMax: g.Fill,
		}
		a.grids[g.Identity] = st
		a.record(Transition{
			Kind: GridAppeared, At: s.Timestamp, Sequence: s.Sequence,
			Identity:   g.Identity,
			After:      fmt.Sprintf("%dx%d grid of %d cells", g.Rows, g.Columns, g.Cells),
			Confidence: g.Fill,
			Reason:     "a regular arrangement of similar elements was observed",
		})
	}
	st.LastSeen = s.Timestamp
	st.Occurrences++
	if firstInSample {
		st.SamplesSeen++
	}
	if g.Fill < st.ConfidenceMin {
		st.ConfidenceMin = g.Fill
	}
	if g.Fill > st.ConfidenceMax {
		st.ConfidenceMax = g.Fill
	}
}

// Note records a transition the loop itself observed, such as losing the target.
func (a *Analyzer) Note(t Transition) { a.record(t) }

func (a *Analyzer) record(t Transition) {
	if len(a.transitions) >= a.bounds.MaxTransitions {
		a.droppedTransitions++
		return
	}
	a.nextID++
	t.ID = TransitionID(fmt.Sprintf("t%d", a.nextID))
	a.transitions = append(a.transitions, t)
}

// Findings is everything a session established, before any interpretation.
type Findings struct {
	Samples     int          `json:"samples"`
	Thresholds  Thresholds   `json:"thresholds"`
	Stable      []Stability  `json:"stable"`
	Unstable    []Stability  `json:"unstable"`
	Transitions []Transition `json:"transitions"`
	// Dropped says what was discarded to stay bounded. Reported because silence here
	// would read as "nothing more happened".
	DroppedEntities    int `json:"dropped_entities,omitempty"`
	DroppedTransitions int `json:"dropped_transitions,omitempty"`
}

// Findings returns the deterministic evidence summary.
func (a *Analyzer) Findings() Findings {
	f := Findings{
		Samples: a.samples, Thresholds: a.thresholds,
		Transitions:        a.transitions,
		DroppedEntities:    a.droppedEntities,
		DroppedTransitions: a.droppedTransitions,
	}
	all := make([]*Stability, 0, len(a.entities)+len(a.grids))
	for _, st := range a.entities {
		all = append(all, st)
	}
	for _, st := range a.grids {
		all = append(all, st)
	}
	// Sorted for determinism: a fixture must replay identically.
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].PresenceRatio != all[j].PresenceRatio {
			return all[i].PresenceRatio > all[j].PresenceRatio
		}
		return all[i].Identity < all[j].Identity
	})
	for _, st := range all {
		if st.Stable(a.thresholds) {
			f.Stable = append(f.Stable, *st)
		} else {
			f.Unstable = append(f.Unstable, *st)
		}
	}
	return f
}

func describeEntity(s *Stability) string {
	if s.Label.Empty() {
		return fmt.Sprintf("%s at %s", s.Role, describeRegion(s.Region))
	}
	return fmt.Sprintf("%s %s at %s", s.Role, s.Label.Describe(), describeRegion(s.Region))
}

// describeRegion names a region in words rather than numbers.
//
// "lower right" survives a resolution change and a window move; "1643,831" does not, and is
// also a small fact about somebody's monitor.
func describeRegion(r Region) string {
	vertical := []string{"upper", "middle", "lower"}[clampQ(int(r.Y*3))]
	horizontal := []string{"left", "centre", "right"}[clampQ(int(r.X*3))]
	if vertical == "middle" && horizontal == "centre" {
		return "the centre"
	}
	return "the " + vertical + " " + horizontal
}
