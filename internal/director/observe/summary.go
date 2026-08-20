package observe

import (
	"sort"
	"time"
)

// The bounded, wire-shaped view of a live analysis.
//
// # Why a narrow DTO rather than shipping Findings
//
// Findings holds every entity the session has ever seen, stable and unstable alike. A real
// session produced 139 unstable entities in three minutes, and a HUD refreshing twice a
// second cannot carry that — nor should it, because a list of 139 anonymous flickers is not
// information. It is the same fact 139 times.
//
// So the unstable side becomes COUNTS and the stable side is bounded. Both report their
// true totals, so a bounded view is never mistaken for the whole picture.
//
// # This summarises; it does not decide
//
// Every number here is copied or counted from what the Analyzer already concluded. Nothing
// re-derives stability, re-scores confidence, or forms a hypothesis. The bounds are the only
// judgement, and they are declared rather than implied.

// SummaryBounds caps what crosses the wire.
type SummaryBounds struct {
	MaxStable      int
	MaxHypotheses  int
	MaxTransitions int
}

// DefaultSummaryBounds are sized for a HUD, not for an audit.
//
// Deliberately small: the full evidence is available from the session report when somebody
// wants it, and a live panel that carried everything would spend its budget on rows nobody
// reads.
func DefaultSummaryBounds() SummaryBounds {
	return SummaryBounds{MaxStable: 24, MaxHypotheses: 12, MaxTransitions: 12}
}

// StableEntity is one thing the session came to trust.
type StableEntity struct {
	// Identity is a safe digest. Never an ElementID, a handle or a native id.
	Identity Digest `json:"identity"`
	Role     string `json:"role"`
	// Label is already privacy-classified — a withheld one carries a digest and a
	// length and no text. Classification happened before this struct existed.
	Label SafeLabel `json:"label"`

	PresenceRatio float64 `json:"presence_ratio"`
	SamplesSeen   int     `json:"samples_seen"`
	SamplesTotal  int     `json:"samples_total"`
	// ConfidenceBand is the quantised weakest confidence, so a HUD shows a band rather
	// than implying precision the evidence does not have.
	ConfidenceBand int `json:"confidence_band"`
	// LastSeenMS is how long ago it was last observed.
	LastSeenMS int64 `json:"last_seen_ms"`
	// Flickers is how often it vanished and returned — the signal that separates a
	// thing being closed and reopened from a detector struggling.
	Flickers int `json:"flickers"`
}

// UnreliableSummary is the aggregate of what did NOT hold still.
//
// Counts only. The individual identities are exactly the data that is both enormous and
// useless: knowing that one specific anonymous icon flickered tells a viewer nothing, and
// knowing that 131 of them did tells them the detector is struggling.
type UnreliableSummary struct {
	// Entities is how many failed the stability threshold.
	Entities int `json:"entities"`
	// Anonymous is how many carried no readable label at all.
	Anonymous int `json:"anonymous"`
	// Withheld is how many had a label that privacy classification would not release.
	Withheld int `json:"withheld"`
	// Flickering is how many came and went more than the threshold tolerates.
	Flickering int `json:"flickering"`
	// DroppedEntities and DroppedTransitions are what retention discarded. Reported
	// because silence here reads as "nothing more happened".
	DroppedEntities    int `json:"dropped_entities,omitempty"`
	DroppedTransitions int `json:"dropped_transitions,omitempty"`
}

// LiveHypothesis is one candidate interpretation, with its uncertainty attached.
//
// Contradictions and RecommendedValidation are carried, never optional. A hypothesis that
// reached a screen without them would be an assertion.
type LiveHypothesis struct {
	ID         string          `json:"id"`
	Kind       Concept         `json:"kind"`
	State      HypothesisState `json:"state"`
	Confidence float64         `json:"confidence"`
	// ConfidenceBand is what a HUD should render. The raw confidence is carried for
	// diagnostics, but a percentage to two decimal places would imply a precision the
	// analysis does not claim.
	ConfidenceBand int `json:"confidence_band"`

	SupportingSamples     int            `json:"supporting_samples"`
	SupportingEntities    []Digest       `json:"supporting_entities,omitempty"`
	SupportingTransitions []TransitionID `json:"supporting_transitions,omitempty"`

	Contradictions        []string `json:"contradictions,omitempty"`
	RecommendedValidation string   `json:"recommended_validation,omitempty"`
	Observed              string   `json:"observed,omitempty"`
}

// LiveTransition is one recent change, in the analyzer's own closed vocabulary.
type LiveTransition struct {
	ID   TransitionID `json:"id"`
	Kind Kind         `json:"kind"`
	At   time.Time    `json:"at"`
	// Identity is a safe digest where the transition concerned one entity.
	Identity   Digest  `json:"identity,omitempty"`
	Confidence float64 `json:"confidence"`
	// Reason is the analyzer's own safe sentence. Never plaintext that failed
	// classification, and never rewritten here.
	Reason string `json:"reason,omitempty"`
}

// LiveSummary is the whole bounded picture a client renders.
type LiveSummary struct {
	// Active says a session is running now. Distinct from a completed session's
	// summary, which is still worth showing and must not be presented as live.
	Active    bool      `json:"active"`
	SessionID SessionID `json:"session_id,omitempty"`
	State     State     `json:"state,omitempty"`
	Samples   int       `json:"samples"`
	// FreshnessMS is how old the newest evidence is.
	FreshnessMS int64 `json:"freshness_ms"`
	// Complete distinguishes a session that ran its course from one that was cancelled
	// or failed — only the first licenses reading conclusions from the sample size.
	Complete bool `json:"complete"`

	StartedAt time.Time  `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	// Terminal says the session has stopped. A terminal session is still a valid source
	// of analysis — completion freezes evidence, it does not erase it.
	Terminal bool `json:"terminal,omitempty"`
	// TerminalReason explains a non-completed ending in words a person can act on.
	// Empty for a session that ran its course.
	TerminalReason string `json:"terminal_reason,omitempty"`

	Stable          []StableEntity    `json:"stable,omitempty"`
	StableTotal     int               `json:"stable_total"`
	Unreliable      UnreliableSummary `json:"unreliable"`
	Hypotheses      []LiveHypothesis  `json:"hypotheses,omitempty"`
	HypothesesTotal int               `json:"hypotheses_total"`
	// WithdrawnHypotheses is how many were published and later retracted. Carried
	// because a list of active ones alone reads as though nothing was ever wrong.
	WithdrawnHypotheses int              `json:"withdrawn_hypotheses"`
	Transitions         []LiveTransition `json:"transitions,omitempty"`
	TransitionsTotal    int              `json:"transitions_total"`

	// Cursor is the newest event sequence at the moment this was taken, so a client can
	// resume the feed from exactly here with neither a gap nor a replay.
	Cursor uint64 `json:"cursor"`
	Oldest uint64 `json:"oldest"`

	Truncated bool `json:"truncated,omitempty"`
}

// Summarise converts a live analysis into the bounded wire view.
//
// Pure. Given the same analysis it produces the same summary, so a fixture replays exactly.
func Summarise(a LiveAnalysis, b SummaryBounds, now time.Time) LiveSummary {
	if b.MaxStable <= 0 {
		b = DefaultSummaryBounds()
	}
	out := LiveSummary{
		SessionID: a.SessionID, State: a.State, Samples: a.Samples,
		// Complete is Succeeded(), NOT Terminal(). A cancelled or failed session has
		// stopped and is still worth showing, but calling it complete would license
		// reading conclusions from a sample size that was cut short.
		Complete:            a.State.Succeeded(),
		Terminal:            a.State.Terminal(),
		TerminalReason:      a.Reason,
		StartedAt:           a.StartedAt,
		EndedAt:             a.EndedAt,
		WithdrawnHypotheses: a.WithdrawnHypotheses,
		Cursor:              a.Cursor, Oldest: a.Oldest,
	}
	f := a.Findings
	out.StableTotal = len(f.Stable)
	out.TransitionsTotal = len(f.Transitions)
	out.HypothesesTotal = len(a.Insights)

	newest := time.Time{}
	for _, s := range f.Stable {
		if s.LastSeen.After(newest) {
			newest = s.LastSeen
		}
	}
	if !newest.IsZero() {
		out.FreshnessMS = now.Sub(newest).Milliseconds()
	}

	// Stable, strongest first — the Analyzer's own ordering, not a re-ranking.
	for i, s := range f.Stable {
		if i >= b.MaxStable {
			out.Truncated = true
			break
		}
		e := StableEntity{
			Identity: s.Identity, Role: s.Role, Label: s.Label,
			PresenceRatio: s.PresenceRatio,
			SamplesSeen:   s.SamplesSeen, SamplesTotal: s.SamplesTotal,
			ConfidenceBand: bandOf(s.ConfidenceMin),
			Flickers:       s.Flickers,
		}
		if !s.LastSeen.IsZero() {
			e.LastSeenMS = now.Sub(s.LastSeen).Milliseconds()
		}
		out.Stable = append(out.Stable, e)
	}

	out.Unreliable = summariseUnreliable(f)

	for i, in := range a.Insights {
		if i >= b.MaxHypotheses {
			out.Truncated = true
			break
		}
		out.Hypotheses = append(out.Hypotheses, LiveHypothesis{
			ID: hypothesisID(in), Kind: in.Kind, State: StateActive,
			Confidence: in.Confidence, ConfidenceBand: bandOf(in.Confidence),
			SupportingSamples:     in.SupportingSamples,
			SupportingEntities:    append([]Digest{}, in.SupportingEntities...),
			SupportingTransitions: append([]TransitionID{}, in.SupportingTransitions...),
			Contradictions:        append([]string{}, in.Contradictions...),
			RecommendedValidation: in.Validation,
			Observed:              in.Observed,
		})
	}

	// Transitions, most recent first — what a feed wants is the latest, and a bound
	// taken from the front would show the oldest forever.
	recent := append([]Transition{}, f.Transitions...)
	sort.SliceStable(recent, func(i, j int) bool { return recent[i].Sequence > recent[j].Sequence })
	for i, t := range recent {
		if i >= b.MaxTransitions {
			out.Truncated = true
			break
		}
		out.Transitions = append(out.Transitions, LiveTransition{
			ID: t.ID, Kind: t.Kind, At: t.At, Identity: t.Identity,
			Confidence: t.Confidence, Reason: t.Reason,
		})
	}
	return out
}

// summariseUnreliable counts what did not hold still, rather than listing it.
func summariseUnreliable(f Findings) UnreliableSummary {
	u := UnreliableSummary{
		Entities:           len(f.Unstable),
		DroppedEntities:    f.DroppedEntities,
		DroppedTransitions: f.DroppedTransitions,
	}
	for _, s := range f.Unstable {
		switch {
		case s.Label.Empty():
			u.Anonymous++
		case s.Label.Redacted:
			u.Withheld++
		}
		if s.SamplesSeen > 0 &&
			float64(s.Flickers)/float64(s.SamplesSeen) > f.Thresholds.MaxFlickerRatio {
			u.Flickering++
		}
	}
	return u
}

// bandOf quantises a 0..1 confidence into ten bands.
//
// The same quantisation the live recorder uses to decide what is a material change, so a
// band shown on a HUD and a band that triggered an update mean the same thing.
func bandOf(c float64) int {
	b := int(clamp01(c) * 10)
	if b > 9 {
		b = 9
	}
	return b
}
