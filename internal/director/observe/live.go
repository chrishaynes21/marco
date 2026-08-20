package observe

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Live streaming of what a session has learned SO FAR.
//
// # This publishes analysis; it does not perform any
//
// Everything here reads Findings and Insights — which the Analyzer already accumulates
// sample by sample — and reports what CHANGED since the last time it looked. It computes no
// stability, no transitions and no hypotheses. Deleting this file would remove the live
// feed and alter no analysis, no execution and no result.
//
// That matters because the alternative was tempting and wrong: a front-end could poll the
// session and diff two snapshots itself. Then every client would own a slightly different
// idea of when a hypothesis "changed", and the Director would no longer be the authority on
// its own findings.
//
// # Why the events carry no labels
//
// A structural label is allowed in the clear by the classifier, and the session SNAPSHOT
// carries SafeLabel for exactly that reason. The event stream does not: it is the part most
// likely to be logged, forwarded, or left on screen for hours, and a count of readable
// labels is all a HUD needs to say "four control-shaped labels". Nothing here can leak a
// name because nothing here has a field to put one in.
//
// # Nothing here asserts
//
// A hypothesis is published with its contradictions and its validation step, or it is not
// published. The rendering may abbreviate; the payload may not.

// LiveKind is the closed vocabulary of live analysis events.
//
// Closed, and checked: an unknown kind is a programming error rather than a string that
// reaches a client. A front-end renders these and nothing else.
type LiveKind string

const (
	EntityBecameStable    LiveKind = "observation_entity_became_stable"
	EntityBecameUnstable  LiveKind = "observation_entity_became_unstable"
	TransitionDetected    LiveKind = "observation_transition_detected"
	HypothesisCreated     LiveKind = "observation_hypothesis_created"
	HypothesisUpdated     LiveKind = "observation_hypothesis_updated"
	HypothesisWithdrawn   LiveKind = "observation_hypothesis_withdrawn"
	ValidationRecommended LiveKind = "observation_validation_recommended"
)

// liveKinds is every kind, for validation and for tests that must cover all of them.
var liveKinds = []LiveKind{
	EntityBecameStable, EntityBecameUnstable, TransitionDetected,
	HypothesisCreated, HypothesisUpdated, HypothesisWithdrawn, ValidationRecommended,
}

// LiveKinds returns the vocabulary.
func LiveKinds() []LiveKind { return append([]LiveKind{}, liveKinds...) }

// Known reports whether a kind is one this build emits.
func (k LiveKind) Known() bool {
	for _, known := range liveKinds {
		if known == k {
			return true
		}
	}
	return false
}

// HypothesisState is where a candidate stands.
//
// Withdrawn is a state rather than a deletion: a hypothesis that quietly vanished from a
// HUD would leave the viewer believing it still held.
type HypothesisState string

const (
	StateActive    HypothesisState = "active"
	StateWithdrawn HypothesisState = "withdrawn"
)

// LiveEvent is one thing the session learned, ready to render.
//
// Closed fields, no arbitrary map — the same rule the session's lifecycle Event follows. A
// generic payload bag carries whatever somebody put in it and nothing type-checks what that
// was.
type LiveEvent struct {
	Kind      LiveKind  `json:"kind"`
	SessionID SessionID `json:"session_id"`
	// Sequence is monotonic within a session and never reused, so a client polling with
	// a cursor can tell "nothing happened" from "I missed some".
	Sequence uint64    `json:"sequence"`
	At       time.Time `json:"at"`
	// Sample is which observation sample produced this.
	Sample int `json:"sample,omitempty"`

	// ── entity events ──
	// Entity is a safe digest, never an ElementID or a handle.
	Entity Digest `json:"entity,omitempty"`
	// Role is a structural kind, which is not screen content.
	Role string `json:"role,omitempty"`
	// Labelled says whether it carried a READABLE label — not what the label said.
	Labelled      bool    `json:"labelled,omitempty"`
	PresenceRatio float64 `json:"presence_ratio,omitempty"`
	SamplesSeen   int     `json:"samples_seen,omitempty"`

	// ── transition events ──
	Transition     TransitionID `json:"transition,omitempty"`
	TransitionKind Kind         `json:"transition_kind,omitempty"`

	// ── hypothesis events ──
	// Hypothesis is session-local and derived from the concept and its anchoring
	// evidence digest. It identifies nothing outside this session.
	Hypothesis string          `json:"hypothesis,omitempty"`
	Concept    Concept         `json:"concept,omitempty"`
	State      HypothesisState `json:"state,omitempty"`
	Confidence float64         `json:"confidence,omitempty"`
	// ConfidenceBand is the quantised band the confidence falls in. It is what
	// "materially changed" is judged on, and publishing it lets a client see the same
	// thing the deduplicator did.
	ConfidenceBand int `json:"confidence_band,omitempty"`

	SupportingSamples     int            `json:"supporting_samples,omitempty"`
	SupportingEntities    []Digest       `json:"supporting_entities,omitempty"`
	SupportingTransitions []TransitionID `json:"supporting_transitions,omitempty"`

	// Contradictions are the analyzer's own safe sentences — generated from counts and
	// ratios, never read from the screen. Always carried when any exist: a hypothesis
	// shipped with only its supporting evidence is advocacy.
	Contradictions []string `json:"contradictions,omitempty"`
	// RecommendedValidation is what a person could do next to settle it.
	RecommendedValidation string `json:"recommended_validation,omitempty"`
	// Observed is the plain evidence sentence, kept separate from the guess.
	Observed string `json:"observed,omitempty"`
}

// LiveAnalysis is the current state of a running session's analysis.
//
// The rebuild path. A client that opened late, reconnected, or lost events to a retention
// rollover asks for this and renders it, rather than inferring what it missed — inference
// over a gap is how a HUD ends up confidently showing something that was withdrawn.
//
// Carries the analyzer's own result types. Nothing here is a second summary, and nothing
// recomputes: Findings and Insights are the same pure reads the final Result uses.
type LiveAnalysis struct {
	SessionID SessionID  `json:"session_id"`
	State     State      `json:"state"`
	Samples   int        `json:"samples"`
	StartedAt time.Time  `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	// Reason explains a non-completed ending.
	Reason string `json:"reason,omitempty"`

	Findings Findings  `json:"findings"`
	Insights []Insight `json:"insights,omitempty"`
	// Hypotheses are the discovery-side interpretations: what the recurring screens and
	// the navigation between them might mean. Recomputed on read, like Insights.
	Hypotheses []Hypothesis `json:"hypotheses,omitempty"`

	// Cursor is the newest event sequence at the moment this was taken, so a client can
	// resume the feed from exactly here with no gap and no replay.
	Cursor uint64 `json:"cursor"`
	// Oldest is the lowest sequence still retained.
	Oldest uint64 `json:"oldest"`
	// WithdrawnHypotheses is how many were published and later retracted. Reported
	// because a count of active hypotheses alone reads as though nothing was ever wrong.
	WithdrawnHypotheses int `json:"withdrawn_hypotheses"`
}

// MaterialThresholds decide when a change is worth publishing.
//
// Without these the feed restates the same hypothesis every sample — thirty times a minute,
// saying nothing new each time. The point of a live feed is that a line appearing MEANS
// something happened.
type MaterialThresholds struct {
	// ConfidenceBands is how many bands the 0..1 range is divided into. A confidence
	// that moves within one band is not news.
	ConfidenceBands int
	// MinSupportChange is retained for compatibility and is NOT used.
	//
	// It keyed materiality on the supporting-sample count, which several concepts
	// report as the session's total sample count — so it grew every half second and
	// republished unchanged hypotheses forever. Materiality is keyed on the evidence
	// SET instead; see `published.evidence`.
	MinSupportChange int
	// MaxEvents bounds the retained log.
	MaxEvents int
}

// DefaultMaterialThresholds are the provisional defaults.
//
// Ten bands means a hypothesis has to move a tenth of the range before it is mentioned
// again. Provisional like the analysis thresholds beside them — they came from reasoning
// about a 500ms sampling interval, not from a measured session.
func DefaultMaterialThresholds() MaterialThresholds {
	return MaterialThresholds{
		ConfidenceBands:  10,
		MinSupportChange: 3,
		MaxEvents:        512,
	}
}

// LiveRecorder folds successive analyses into an event log.
//
// Not safe for concurrent use; the caller serialises with the sampling loop it already
// owns. One writer, by construction.
type LiveRecorder struct {
	session   SessionID
	t         MaterialThresholds
	seq       uint64
	events    []LiveEvent
	oldestSeq uint64

	// stable is what was stable at the last fold, so a crossing is detectable. Entities
	// are NOT re-announced every sample — only the crossing is news.
	stable map[Digest]bool
	// transitions already published.
	seen map[TransitionID]bool
	// active is the last published form of each hypothesis, for material comparison.
	active map[string]published
	// withdrawn counts hypotheses that were active and no longer are.
	withdrawn int
}

// published is the last form of a hypothesis that reached a client.
type published struct {
	band int
	// evidence is a digest of the supporting entity and transition SET.
	//
	// The set, deliberately, and not the supporting-sample COUNT. Several concepts
	// report SupportingSamples as the session's total sample count, which grows every
	// half second whether or not anything new supports the hypothesis — so a rule keyed
	// on that count republishes an unchanged claim forever. A live session against VS
	// Code produced six identical "updated" events at the same confidence band before
	// this was keyed on evidence instead of time.
	evidence   string
	contra     string
	validation string
	concept    Concept
}

// NewLiveRecorder builds a recorder for one session.
func NewLiveRecorder(id SessionID, t MaterialThresholds) *LiveRecorder {
	if t.ConfidenceBands <= 0 {
		t.ConfidenceBands = DefaultMaterialThresholds().ConfidenceBands
	}
	if t.MaxEvents <= 0 {
		t.MaxEvents = DefaultMaterialThresholds().MaxEvents
	}
	return &LiveRecorder{
		session: id, t: t,
		stable: map[Digest]bool{},
		seen:   map[TransitionID]bool{},
		active: map[string]published{},
	}
}

// Observe folds one sample's accumulated analysis into the log.
//
// Called after the Analyzer has taken the sample, with the findings and insights it already
// produced. It re-derives nothing.
func (r *LiveRecorder) Observe(sample int, f Findings, insights []Insight, now time.Time) {
	r.foldStability(sample, f, now)
	r.foldTransitions(sample, f, now)
	r.foldHypotheses(sample, insights, now)
}

// foldStability emits only the CROSSINGS of the analyzer's own stability threshold.
func (r *LiveRecorder) foldStability(sample int, f Findings, now time.Time) {
	nowStable := make(map[Digest]bool, len(f.Stable))
	for _, s := range f.Stable {
		nowStable[s.Identity] = true
		if r.stable[s.Identity] {
			continue // already announced; presence alone is not news
		}
		r.emit(LiveEvent{
			Kind: EntityBecameStable, At: now, Sample: sample,
			Entity: s.Identity, Role: s.Role,
			Labelled:      !s.Label.Empty() && s.Label.Text != "",
			PresenceRatio: s.PresenceRatio, SamplesSeen: s.SamplesSeen,
			Observed: fmt.Sprintf("a %s was present in %d of %d samples",
				s.Role, s.SamplesSeen, s.SamplesTotal),
		})
	}
	// Anything that was stable and is not now has fallen below the threshold, or its
	// identity stopped being reliable. Either way the earlier claim no longer holds and
	// saying nothing would leave it standing.
	for id := range r.stable {
		if nowStable[id] {
			continue
		}
		r.emit(LiveEvent{
			Kind: EntityBecameUnstable, At: now, Sample: sample, Entity: id,
			Observed: "an entity that had been stable no longer meets the stability threshold",
		})
	}
	r.stable = nowStable
}

// foldTransitions publishes each transition once, in the analyzer's own vocabulary.
func (r *LiveRecorder) foldTransitions(sample int, f Findings, now time.Time) {
	for _, t := range f.Transitions {
		if r.seen[t.ID] {
			continue
		}
		r.seen[t.ID] = true
		r.emit(LiveEvent{
			Kind: TransitionDetected, At: now, Sample: sample,
			Transition: t.ID, TransitionKind: t.Kind, Entity: t.Identity,
			Confidence: t.Confidence,
			// The analyzer's own safe sentence, forwarded rather than rewritten.
			Observed: t.Reason,
		})
	}
}

// foldHypotheses emits creations, MATERIAL updates, and withdrawals.
func (r *LiveRecorder) foldHypotheses(sample int, insights []Insight, now time.Time) {
	seen := map[string]bool{}

	for _, in := range insights {
		id := hypothesisID(in)
		seen[id] = true
		band := r.band(in.Confidence)
		contra := joinContradictions(in.Contradictions)

		prev, known := r.active[id]
		next := published{
			band: band, evidence: evidenceKey(in), contra: contra,
			validation: in.Validation, concept: in.Kind,
		}

		switch {
		case !known:
			r.emit(r.hypothesisEvent(HypothesisCreated, id, in, band,
				StateActive, now, sample))
			// A validation step is worth its own line the first time, because it is
			// the one thing on the card a person can act on.
			if in.Validation != "" {
				r.emit(LiveEvent{
					Kind: ValidationRecommended, At: now, Sample: sample,
					Hypothesis: id, Concept: in.Kind,
					RecommendedValidation: in.Validation,
				})
			}
		case material(prev, next, r.t):
			r.emit(r.hypothesisEvent(HypothesisUpdated, id, in, band,
				StateActive, now, sample))
		default:
			continue // nothing material changed; saying so again would be noise
		}
		r.active[id] = next
	}

	// Withdrawal: the evidence no longer supports something previously published.
	for id, prev := range r.active {
		if seen[id] {
			continue
		}
		delete(r.active, id)
		r.withdrawn++
		r.emit(LiveEvent{
			Kind: HypothesisWithdrawn, At: now, Sample: sample,
			Hypothesis: id, Concept: prev.concept, State: StateWithdrawn,
			Observed: "the accumulated evidence no longer supports this",
		})
	}
}

func (r *LiveRecorder) hypothesisEvent(k LiveKind, id string, in Insight, band int,
	state HypothesisState, now time.Time, sample int) LiveEvent {

	return LiveEvent{
		Kind: k, At: now, Sample: sample,
		Hypothesis: id, Concept: in.Kind, State: state,
		Confidence: in.Confidence, ConfidenceBand: band,
		SupportingSamples:     in.SupportingSamples,
		SupportingEntities:    append([]Digest{}, in.SupportingEntities...),
		SupportingTransitions: append([]TransitionID{}, in.SupportingTransitions...),
		Contradictions:        append([]string{}, in.Contradictions...),
		RecommendedValidation: in.Validation,
		Observed:              in.Observed,
	}
}

// band quantises a confidence, so movement within a band is not published.
func (r *LiveRecorder) band(c float64) int {
	b := int(clamp01(c) * float64(r.t.ConfidenceBands))
	if b >= r.t.ConfidenceBands {
		b = r.t.ConfidenceBands - 1
	}
	return b
}

// material decides whether a change is worth a line.
//
// Every rule here is about the EVIDENCE, never about elapsed time. A hypothesis that says
// exactly the same thing about exactly the same evidence is not news however long it has
// been saying it.
func material(prev, next published, _ MaterialThresholds) bool {
	switch {
	case prev.band != next.band:
		return true
	case prev.evidence != next.evidence:
		return true
	case prev.contra != next.contra:
		return true
	case prev.validation != next.validation:
		return true
	}
	return false
}

// evidenceKey digests the supporting entity and transition set.
//
// Order-insensitive: the analyser's ordering is stable, but keying on order would turn a
// re-sort into a fabricated update.
func evidenceKey(in Insight) string {
	parts := make([]string, 0, len(in.SupportingEntities)+len(in.SupportingTransitions))
	for _, d := range in.SupportingEntities {
		parts = append(parts, "e:"+string(d))
	}
	for _, tr := range in.SupportingTransitions {
		parts = append(parts, "t:"+string(tr))
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

// hypothesisID is a session-local identity for one candidate.
//
// Concept plus its lowest supporting evidence digest. Deterministic, and stable for as long
// as that anchoring evidence survives — which is the honest boundary: a hypothesis whose
// anchor disappeared is not the same hypothesis, and it is withdrawn and re-created rather
// than silently re-pointed at different evidence.
func hypothesisID(in Insight) string {
	anchor := ""
	for _, d := range in.SupportingEntities {
		if anchor == "" || string(d) < anchor {
			anchor = string(d)
		}
	}
	if anchor == "" {
		return string(in.Kind)
	}
	return string(in.Kind) + ":" + anchor
}

func joinContradictions(c []string) string {
	if len(c) == 0 {
		return ""
	}
	sorted := append([]string{}, c...)
	sort.Strings(sorted)
	out := ""
	for _, s := range sorted {
		out += s + "\x00"
	}
	return out
}

func (r *LiveRecorder) emit(e LiveEvent) {
	r.seq++
	e.Sequence = r.seq
	e.SessionID = r.session
	r.events = append(r.events, e)
	if len(r.events) > r.t.MaxEvents {
		r.events = r.events[len(r.events)-r.t.MaxEvents:]
	}
	r.oldestSeq = r.events[0].Sequence
}

// Since returns events after a cursor, oldest first.
func (r *LiveRecorder) Since(cursor uint64, limit int) []LiveEvent {
	if limit <= 0 || limit > r.t.MaxEvents {
		limit = r.t.MaxEvents
	}
	out := make([]LiveEvent, 0, limit)
	for _, e := range r.events {
		if e.Sequence > cursor {
			out = append(out, e)
		}
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// Newest is the highest sequence issued; Oldest is the lowest still retained.
//
// Together they make loss detectable: an oldest greater than cursor+1 means the log rolled
// past the client. Dropped frames are acceptable; dropped events must be visible.
func (r *LiveRecorder) Newest() uint64 { return r.seq }
func (r *LiveRecorder) Oldest() uint64 { return r.oldestSeq }

// WithdrawnCount is how many hypotheses were published and later retracted.
func (r *LiveRecorder) WithdrawnCount() int { return r.withdrawn }
