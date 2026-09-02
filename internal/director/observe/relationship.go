package observe

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Remembering that two subjects are CONNECTED, without claiming to know how to travel between
// them.
//
// # The claim, stated once
//
// A remembered relationship says exactly one thing:
//
//	the subject Marco recognises as A was observed becoming the subject it recognises as B.
//
// It may carry evidence about what navigation was seen around that change. It does NOT say the
// navigation caused it, that performing the navigation would reproduce it, or that Marco knows
// how to get from A to B. Those are three separate, larger claims and none of them is made here.
//
// The word for the durable object is therefore `observed_transition`, and the naming is
// deliberate: `procedure`, `route`, `action` and `binding` all imply something executable, and a
// type named for what it is not yet is a type somebody will eventually execute.
//
// # Why this is a durable object at all
//
// Screen transitions already exist, per session, keyed on `state_3 → state_8`. Those references
// are counters minted by this session's segmenter — the next run renumbers everything, so a
// session-local edge cannot corroborate anything a previous session saw. What survives a restart
// is a RememberedSubject, and an edge between two of those is the first thing Marco can
// accumulate across days rather than across minutes.
//
// # Both ends, or nothing
//
// A durable edge is written only when BOTH endpoints resolve to remembered subjects with
// established identity. `candidate`, `insufficient` and `different` are all refusals: a wrong
// durable edge is a claim about a screen Marco cannot actually recognise, and it would survive
// every session that could have contradicted it. A missed edge costs one session's evidence.
//
// The resolution is done by the semantic-memory identity layer — `Memory.Recall`, which is
// `CompareStructure` — and nothing here matches subjects itself. A second matcher with its own
// tolerances is a second answer to "is this the same screen", and the two would diverge.

// RelationshipEvidence is one session's bounded aggregate for one edge.
//
// A projection of ScreenTransition onto the durable side: the same categories, because they were
// chosen for the same reason and collapsing any of them would lose the thing that keeps this
// honest. See the field notes on ScreenTransition.
type RelationshipEvidence struct {
	// Observations is how many times the change was seen.
	Observations int `json:"observations"`
	// Preceded counts how often each navigation intent was seen before the change.
	//
	// CORRELATION. `confirm: 4` beside `observations: 5` says the intent was seen before four
	// of the five, not that it caused any of them.
	Preceded map[NavIntent]int `json:"preceded,omitempty"`
	// Unattributed counts changes with no navigation observed before them at all.
	//
	// The control evidence, and the reason it is stored rather than implied by subtraction:
	// an edge seen ten times with `confirm` before three of them is mostly a change that
	// happens on its own, and a representation that dropped this number would read as
	// "confirm, 3" with nothing to weigh it against.
	Unattributed int `json:"unattributed,omitempty"`
	// ConditionalOnly counts attributed observations whose navigation came ENTIRELY from keys
	// that are only navigation in context — see [[ADR-013-navigation-is-meaning-not-keys]].
	//
	// Weaker evidence, kept weaker across the durable boundary. W before a change while the
	// screen looked like a set of choices is real and is also "walk forwards"; folding it in
	// with the unambiguous observations would let a session of somebody walking around
	// produce the same confident edge as one of somebody using a menu.
	ConditionalOnly int `json:"conditional_only,omitempty"`
	// Bridged counts observations recovered across an unplaceable sample rather than seen as
	// one continuous change.
	//
	// Kept so the claim stays visible wherever the edge is read. The adjacency is the same
	// adjacency — A really did become B — but the evidence for it was assembled from two
	// halves either side of a frame nobody could place, and a reader weighing this edge is
	// entitled to know that. See bridge.go for the five conditions that must hold.
	Bridged int `json:"bridged,omitempty"`
	// Sequences are the ORDERED runs seen before the change, one entry per distinct order.
	//
	// Preceded answers "which intents"; this answers "in what order", and they are different
	// questions. `down, down, confirm` and `confirm, down, down` produce identical Preceded
	// maps and describe two different interactions.
	//
	// An observed order is NOT a procedure. Nothing here says these are the steps to reach B,
	// and the field is deliberately plural and counted rather than singular and canonical:
	// three different runs before the same change is evidence that there is no one way.
	Sequences []NavSequence `json:"sequences,omitempty"`
}

// Empty reports whether there is nothing here worth remembering.
func (e RelationshipEvidence) Empty() bool { return e.Observations == 0 }

// Attributed is how many observations had any navigation before them.
func (e RelationshipEvidence) Attributed() int {
	if e.Unattributed > e.Observations {
		return 0
	}
	return e.Observations - e.Unattributed
}

// StrongOnly reports whether every attributed observation rested on context-admitted keys.
//
// The question a reader has to be able to ask before treating a navigation association as
// meaning anything: "was any of this unambiguous?". False here does not make the evidence
// worthless — it makes it weak, and weak evidence presented as strong is the failure ADR-013
// exists to prevent.
func (e RelationshipEvidence) ConditionalEvidenceOnly() bool {
	return e.Attributed() > 0 && e.ConditionalOnly >= e.Attributed()
}

// MaxDurableSequences bounds how many distinct orders one durable edge remembers.
//
// Six, matching MaxSequencesPerTransition. A relationship observed a thousand times with a
// different incidental run each time is a relationship with no characteristic run, and storing
// the thousandth variant would say less than the counter reporting that variants were dropped.
const MaxDurableSequences = MaxSequencesPerTransition

// MaxDurableIntents bounds how many distinct intents one durable edge remembers.
const MaxDurableIntents = MaxIntentsPerTransition

// RememberedRelationship is one durable observed transition between two remembered subjects.
//
// Endpoints are RememberedSubject IDs, never session-local state references. Direction is part
// of identity: A→B and B→A are different observations about the world, and a menu that opens on
// `confirm` and closes on `back` would otherwise collapse into one edge that appears to respond
// to both.
type RememberedRelationship struct {
	// From and To are RememberedSubject.ID values.
	From string `json:"from"`
	To   string `json:"to"`
	// Application namespaces the edge, inherited from its endpoints rather than decided
	// here. Two applications presenting structurally identical screens are not assumed to
	// mean the same thing by them, and an edge across that boundary would assume it twice.
	Application string `json:"application"`
	// Observations is the running total across every session.
	Observations int `json:"observations"`
	// Sessions is how many separate sessions have seen this edge.
	//
	// Held apart from Observations, and discretely rather than as a ratio. Twenty
	// observations in one sitting and the same edge on five different days are different
	// kinds of evidence, and only the second has survived a restart, a different window
	// generation and whatever else changed in between.
	Sessions int `json:"sessions,omitempty"`

	Preceded        map[NavIntent]int `json:"preceded,omitempty"`
	Unattributed    int               `json:"unattributed,omitempty"`
	ConditionalOnly int               `json:"conditional_only,omitempty"`
	Sequences       []NavSequence     `json:"sequences,omitempty"`
	// DroppedSequences counts distinct orders refused at the bound, so a capped set never
	// reads as a complete one.
	DroppedSequences int `json:"dropped_sequences,omitempty"`
	// FollowUp is what the user said when asked for a SECOND example, nil when never asked.
	//
	// Held apart from Learning rather than overwriting it: the first request was fulfilled
	// and that is part of the lineage, and reusing one field would make "asked once"
	// indistinguishable from "asked twice".
	FollowUp *LearningRequest `json:"follow_up,omitempty"`
	// Learning is what the user said when asked whether Marco should learn this, nil when
	// they have never been asked.
	//
	// A PREFERENCE, held beside the observation and never mixed into it. A refusal does not
	// reduce Observations, does not touch the navigation evidence, and does not mean the
	// transition did not happen — see learning.go.
	Learning *LearningRequest `json:"learning,omitempty"`
}

// Evidence returns the aggregate as one value.
func (r RememberedRelationship) Evidence() RelationshipEvidence {
	return RelationshipEvidence{
		Observations: r.Observations, Preceded: r.Preceded,
		Unattributed: r.Unattributed, ConditionalOnly: r.ConditionalOnly,
		Sequences: r.Sequences,
	}
}

// Fold merges one session's evidence into this record, bounded.
//
// Additive on counts and set-union on the categorical parts. The bounds are applied here rather
// than by the caller so that every path into the store obeys them.
func (r *RememberedRelationship) Fold(e RelationshipEvidence) {
	r.Observations += e.Observations
	r.Unattributed += e.Unattributed
	r.ConditionalOnly += e.ConditionalOnly
	for intent, n := range e.Preceded {
		// Admission, not trust. The underlying type is a string, so what keeps arbitrary
		// text out of a durable file is checking membership rather than declaring a type.
		if !intent.Known() {
			continue
		}
		if r.Preceded == nil {
			r.Preceded = map[NavIntent]int{}
		}
		if _, seen := r.Preceded[intent]; !seen && len(r.Preceded) >= MaxDurableIntents {
			continue
		}
		r.Preceded[intent] += n
	}
	for _, seq := range e.Sequences {
		r.foldSequence(seq)
	}
	sort.Slice(r.Sequences, func(i, j int) bool {
		if r.Sequences[i].Count != r.Sequences[j].Count {
			return r.Sequences[i].Count > r.Sequences[j].Count
		}
		return joinSequence(r.Sequences[i].Intents) < joinSequence(r.Sequences[j].Intents)
	})
}

func (r *RememberedRelationship) foldSequence(seq NavSequence) {
	intents := admissibleIntents(seq.Intents)
	if len(intents) == 0 {
		return
	}
	for i := range r.Sequences {
		if r.Sequences[i].Equal(intents) {
			r.Sequences[i].Count += seq.Count
			return
		}
	}
	if len(r.Sequences) >= MaxDurableSequences {
		// Counted, never silent. A capped set of runs that reported nothing would read as
		// "these are the ways it happens" when it means "these are the first six".
		r.DroppedSequences++
		return
	}
	r.Sequences = append(r.Sequences, NavSequence{Intents: intents, Count: seq.Count})
}

// admissibleIntents filters a run to the closed vocabulary and bounds its length.
func admissibleIntents(in []NavIntent) []NavIntent {
	out := make([]NavIntent, 0, len(in))
	for _, i := range in {
		if !i.Known() {
			continue
		}
		out = append(out, i)
		if len(out) >= MaxSequenceLength {
			break
		}
	}
	return out
}

func joinSequence(in []NavIntent) string {
	out := ""
	for _, i := range in {
		out += string(i) + ">"
	}
	return out
}

// RelationshipObservation is one session's evidence for one edge, with its endpoints already
// resolved against durable memory.
//
// The endpoints are IDs rather than signatures because resolution has already happened, at the
// one place that is entitled to do it. A store handed signatures would have to match them again,
// and a second matching site is a second set of tolerances.
type RelationshipObservation struct {
	From     string
	To       string
	Evidence RelationshipEvidence
	// SameEpisode says this session is part of an episode already counted, so the evidence
	// folds but no further independent corroboration is claimed.
	//
	// `Sessions` means "how many separate times has this been seen" — separate sittings,
	// separate window generations, separate days. Learn runs several bounded passes back
	// to back in one sitting, and counting each of them would let one explicit learn attempt
	// satisfy a threshold that exists to mean real-world recurrence. It would be Marco
	// manufacturing its own corroboration.
	//
	// The zero value counts, so every existing caller is unchanged and a caller that forgets
	// this field cannot accidentally stop corroborating.
	SameEpisode bool
	// Routed says this crossing arrives WITH route evidence: a demonstration naming what was
	// pressed is written alongside it.
	//
	// It changes nothing that is stored. It decides only what a learning feed is entitled to
	// SAY about the write — "learned the way here" against "watched you go here" — and those
	// are different claims. See semanticmemory.Saw.
	//
	// The zero value is the cautious one on purpose: a caller that forgets this field
	// understates what Marco knows, which is the direction a wrong answer should fall.
	Routed bool
}

// RelationshipReport is what happened to this session's transitions.
//
// Bounded counts rather than a list of edges: a session result is read by a person, and an
// unchanged edge is not news. See the rendering note in the observation view.
type RelationshipReport struct {
	// Durable is how many of this session's transitions had both endpoints recognised.
	Durable int `json:"durable,omitempty"`
	// SessionLocal is how many did not, and therefore stayed evidence about this session
	// only. Reported because a session with no durable edges has two very different
	// explanations — nothing transitioned, or nothing was recognised — and this separates
	// them.
	SessionLocal int `json:"session_local,omitempty"`
	// Created and Corroborated are what the store did with the durable ones.
	Created      int `json:"created,omitempty"`
	Corroborated int `json:"corroborated,omitempty"`
	// Rejected is how many the store refused — a dangling endpoint, or the bound.
	Rejected int `json:"rejected,omitempty"`
	// Unavailable explains why nothing was written, when nothing could be.
	Unavailable string `json:"unavailable,omitempty"`
	// SessionLocalCauses is WHY each session-local transition stayed local, by closed cause.
	//
	// `SessionLocal` alone is a bare count with four different explanations, and the
	// difference decides whether anything is wrong. A live learn attempt refused with
	// "2 transition(s) were seen and none had two recognisable endpoints" and no surface
	// could say whether the screens were unrecognised, identical, or separated by a frame
	// that could not be placed at all. Bounded by the vocabulary, not by the session.
	SessionLocalCauses map[SessionLocalCause]int `json:"session_local_causes,omitempty"`
	// Bridged is how many adjacencies were recovered across one unsettled interval.
	Bridged int `json:"bridged,omitempty"`
	// BridgeRefusals is why a bridge was NOT taken, by closed reason. Silence is the hard
	// case here too: a reader seeing no bridge cannot otherwise tell "there was nothing to
	// bridge" from "there was, and the evidence did not support it".
	BridgeRefusals map[BridgeRefusal]int `json:"bridge_refusals,omitempty"`
}

// Why renders the causes behind a session that produced no durable edge.
//
// The counts are already here and were already closed vocabulary; what was missing was any surface
// that said them out loud. A refusal reading "4 transition(s) were seen and none had two
// recognisable endpoints" is a count of a silence, and the four explanations behind it call for
// four different responses — so the count alone sent every one of them to the same place.
//
// Sorted, so two readings of the same session produce the same sentence.
func (r RelationshipReport) Why() string {
	var parts []string
	for _, c := range sortedKeys(r.SessionLocalCauses) {
		parts = append(parts, string(c)+"="+strconv.Itoa(r.SessionLocalCauses[c]))
	}
	for _, b := range sortedKeys(r.BridgeRefusals) {
		parts = append(parts, "bridge:"+string(b)+"="+strconv.Itoa(r.BridgeRefusals[b]))
	}
	if r.Bridged > 0 {
		parts = append(parts, "bridged="+strconv.Itoa(r.Bridged))
	}
	if len(parts) == 0 {
		return "no cause recorded"
	}
	return strings.Join(parts, " ")
}

// sortedKeys orders a closed-vocabulary tally so its rendering is stable.
func sortedKeys[K ~string, V any](m map[K]V) []K {
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(a, b int) bool { return out[a] < out[b] })
	return out
}

// SessionLocalCause is the CLOSED vocabulary of why one transition produced no durable edge.
//
// Four, and they are genuinely different problems. Two of them say a screen was not recognised
// and point at identity; one says two session-local screens turned out to be the same durable
// place and is ordinary; and the fourth says the change passed through something that was never
// a screen at all, which is a fact about the SAMPLING rather than about either endpoint.
type SessionLocalCause string

const (
	// SourceUnresolved — the screen the change came FROM is not a durable subject.
	SourceUnresolved SessionLocalCause = "source_unresolved"
	// DestinationUnresolved — the screen the change went TO is not a durable subject.
	DestinationUnresolved SessionLocalCause = "destination_unresolved"
	// BothUnresolved — neither end is. Kept apart from the two above because a session in
	// which nothing at all is recognised is a different situation from one where half is.
	BothUnresolved SessionLocalCause = "both_unresolved"
	// SameSubject — both ends resolved, to the SAME durable place. Not a failure: it is a
	// screen Marco cannot tell apart from itself, and a self-loop would claim more.
	SameSubject SessionLocalCause = "same_subject"
)

// noteSessionLocal records one transition's cause. Bounded by the vocabulary above.
func (r *RelationshipReport) noteSessionLocal(cause SessionLocalCause) {
	r.SessionLocal++
	if r.SessionLocalCauses == nil {
		r.SessionLocalCauses = map[SessionLocalCause]int{}
	}
	r.SessionLocalCauses[cause]++
}

// causeOf names why a transition with these resolutions produced nothing durable.
func causeOf(okFrom, okTo bool, from, to string) SessionLocalCause {
	switch {
	case !okFrom && !okTo:
		return BothUnresolved
	case !okFrom:
		return SourceUnresolved
	case !okTo:
		return DestinationUnresolved
	}
	_ = from
	_ = to
	return SameSubject
}

// RelationshipUpdate is what a store did with one batch.
type RelationshipUpdate struct {
	Created      int
	Corroborated int
	Rejected     int
}

// RelationshipsFrom pairs this session's screen transitions with the durable subjects their
// endpoints resolved to.
//
// # Current evidence first, memory second
//
// The transitions are read from THIS session's accumulated evidence, and memory is consulted
// only to name their endpoints. Nothing here can produce a transition: a remembered edge does
// not put A→B into the current session, and if the current session saw no change between two
// screens then no amount of history makes one appear. The arrow points one way, and
// TestMemoryDoesNotManufactureATransition holds it open.
//
// # One adjacency may be recovered across an unplaceable sample
//
// `c` is what the session knows about whether its view was continuous, and it is required rather
// than optional: bridging an unsettled interval is only honest when nothing else could have
// happened during it, and this package cannot observe a window replacement or a target loss. The
// zero value is the conservative reading — no generations recorded, no losses — and every
// condition is checked in bridge.go.
//
// Returns the edges worth making durable and an account of what happened to the rest.
func RelationshipsFrom(t ShadowTotals, hs []Hypothesis, application string, m Memory,
	c Continuity) ([]RelationshipObservation, RelationshipReport) {

	var report RelationshipReport
	if len(t.Transitions) == 0 {
		return nil, report
	}
	if m == nil {
		report.SessionLocal = len(t.Transitions)
		report.Unavailable = "no durable memory is wired, so this session's transitions " +
			"stay session-local"
		return nil, report
	}

	// One signature per state, from the hypotheses this session already generated. Not
	// re-derived: the signature a relationship's endpoint is resolved by must be the same
	// value the subject itself would be stored under, and deriving it twice is how two
	// answers to one question appear.
	signatures := map[ScreenStateID]StructureSignature{}
	for _, h := range hs {
		if h.Subject.Kind != SubjectState {
			continue
		}
		id := ScreenStateID(h.Subject.Ref)
		if _, seen := signatures[id]; seen {
			continue
		}
		signatures[id] = SignatureOf(h)
	}

	// Resolution is cached per state: a session with twenty transitions over five screens
	// would otherwise walk the whole store twenty times for the same answer.
	resolved := map[ScreenStateID]string{}
	resolve := func(id ScreenStateID) (string, bool) {
		if got, seen := resolved[id]; seen {
			return got, got != ""
		}
		sig, ok := signatures[id]
		if !ok {
			resolved[id] = ""
			return "", false
		}
		rec := m.Recall(application, sig)
		// ESTABLISHED, not "close enough". `candidate` means the structure agrees and
		// nothing distinctive confirms it, and `insufficient` means several subjects fit
		// equally — building a durable edge on either would attach a claim about one
		// screen to whichever happened to look similar.
		if !rec.Verdict.Established() || rec.Subject.ID == "" {
			resolved[id] = ""
			return "", false
		}
		resolved[id] = rec.Subject.ID
		return rec.Subject.ID, true
	}

	var out []RelationshipObservation
	for _, tr := range t.Transitions {
		from, okFrom := resolve(tr.From)
		to, okTo := resolve(tr.To)
		if !okFrom || !okTo || from == to {
			// from == to is not an edge between two subjects. Two session-local states
			// that resolve to ONE remembered subject describe a screen Marco cannot tell
			// apart from itself, and a self-loop would say something this layer has no
			// evidence for.
			report.noteSessionLocal(causeOf(okFrom, okTo, from, to))
			continue
		}
		report.Durable++
		out = append(out, RelationshipObservation{
			From: from, To: to,
			Evidence: RelationshipEvidence{
				Observations: tr.Count,
				Preceded:     copyIntents(tr.Preceded),
				Unattributed: tr.Unattributed, ConditionalOnly: tr.ConditionalOnly,
				// STRIPPED to plain runs at the durable boundary, structurally: the
				// store's sequence type has nowhere to put a target, so a label read
				// off a screen cannot outlive the session by riding an edge.
				Sequences: PlainSequences(tr.Sequences),
			},
		})
	}
	// THE bridge. A change that crossed one unplaceable sample is still one change — and a
	// walk that crossed several is several changes, not an ambiguity. See bridge.go.
	for _, b := range bridgeUnsettled(t, resolve, c, &report) {
		report.Durable++
		out = append(out, b)
	}
	// Deterministic order, so two identical sessions write identical files.
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out, report
}

func copyIntents(in map[NavIntent]int) map[NavIntent]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[NavIntent]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// DescribeRelationship renders one durable edge for a person.
//
// Semantic descriptions where the endpoints have them, ids only as a last resort — a person
// asked to compare two twelve-character digests is being asked to do the system's job. The
// causal line is printed explicitly and always, because the single most likely misreading of
// this record is that it says something about cause.
func DescribeRelationship(r RememberedRelationship, from, to RememberedSubject) []string {
	out := []string{
		"from: " + describeSubject(from, r.From),
		"to:   " + describeSubject(to, r.To),
	}
	out = append(out, fmt.Sprintf("observed transitions: %d", r.Observations))
	if r.Sessions > 0 {
		out = append(out, fmt.Sprintf("sessions: %d", r.Sessions))
	}
	intents := make([]NavIntent, 0, len(r.Preceded))
	for intent := range r.Preceded {
		intents = append(intents, intent)
	}
	sort.Slice(intents, func(i, j int) bool {
		if r.Preceded[intents[i]] != r.Preceded[intents[j]] {
			return r.Preceded[intents[i]] > r.Preceded[intents[j]]
		}
		return intents[i] < intents[j]
	})
	for _, intent := range intents {
		out = append(out, fmt.Sprintf("navigation seen before: %s %d/%d",
			string(intent), r.Preceded[intent], r.Observations))
	}
	if r.Unattributed > 0 {
		out = append(out, fmt.Sprintf("no navigation observed: %d/%d",
			r.Unattributed, r.Observations))
	}
	if r.Evidence().ConditionalEvidenceOnly() {
		out = append(out, "all of that navigation was context-admitted, which is weaker "+
			"evidence: those keys mean something else outside a menu")
	} else if r.ConditionalOnly > 0 {
		out = append(out, fmt.Sprintf("context-admitted (weaker) observations: %d",
			r.ConditionalOnly))
	}
	for _, seq := range r.Sequences {
		out = append(out, fmt.Sprintf("observed navigation sequence: %s (%d time(s))",
			describeIntents(seq.Intents), seq.Count))
	}
	if r.DroppedSequences > 0 {
		out = append(out, fmt.Sprintf("further distinct sequences were seen and not kept: %d",
			r.DroppedSequences))
	}
	out = append(out, "causal claim: none. This records that the change was observed and "+
		"what was seen around it, not that any of it caused the change or would reproduce it")
	return out
}

// describeSubject prefers what is KNOWN about a subject over its handle.
func describeSubject(s RememberedSubject, id string) string {
	if s.ID == "" {
		return id + " (no longer in memory)"
	}
	best, status := "", KnowledgeStatus("")
	for _, k := range s.Knowledge {
		// A confirmed interpretation outranks an observed one, and a contradiction is
		// reported as a contradiction rather than quietly dropped — the endpoint's
		// provenance is part of what the edge is worth.
		if best == "" || (status != KnowledgeConfirmed && k.Status == KnowledgeConfirmed) {
			best, status = string(k.Kind), k.Status
		}
	}
	if best == "" {
		return id + " (recognised, nothing known about what it is)"
	}
	return fmt.Sprintf("%s [%s] (%s)", best, status, id)
}

func describeIntents(in []NavIntent) string {
	out := ""
	for i, intent := range in {
		if i > 0 {
			out += " → "
		}
		out += string(intent)
	}
	return out
}
