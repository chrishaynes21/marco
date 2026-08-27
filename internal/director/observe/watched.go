package observe

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// What watching has SEEN repeatedly, before Marco has decided to know it.
//
// # Why this is a third kind of record and not a relationship
//
// The durable topology is what Marco KNOWS: an edge in it is plannable, and a planner handed one
// will walk it. Ambient watching produces something weaker and earlier — "I have seen somebody do
// this, twice, and both times it worked out the same way" — and the whole safety of ambient
// learning rests on those two not being the same thing.
//
// Collapsing them would make the first sighting of anything immediately plannable, which is how a
// system comes to confidently do something it saw once by accident. So a WatchedEdge accumulates
// on its own, is judged by its own policy, and becomes a relationship only by passing through the
// same admission boundary an explicit Learn passes through.
//
// # It is a SUMMARY, and that is the privacy boundary
//
// Counts, times, a semantic identity and — for a screen Marco cannot yet name — the structural
// signature it was seen as. No screenshot, no accessibility tree, no pointer trajectory, no typed
// text, no coordinate that means anything off the window it came from. This is the first thing in
// the ambient path that survives a restart, and it is bounded and semantic for exactly that
// reason.
//
// See [[ADR-095-repeated-observation-may-become-knowledge]].

// MaxWatchedEdges bounds how many candidates are kept, across all applications.
//
// Somebody using a computer meets thousands of controls, and almost every one of them is a thing
// they did once. The bound is on DISTINCT relationships and the eviction is deterministic: see
// WeakerThan for the order, which drops what has been seen least and longest ago before anything
// close to promotion.
const MaxWatchedEdges = 512

// WatchedEnd is one end of a watched relationship.
//
// Either Marco recognises the screen, in which case it has a durable Subject and nothing else is
// needed — carrying a second description beside an identity is the start of a duplicate — or it
// does not, and what is kept is the structure and the word the interface put on it, which is
// exactly what a licensed operation would need to establish it later.
type WatchedEnd struct {
	// Subject is the RememberedSubject id, empty when Marco does not recognise the screen.
	Subject string `json:"subject,omitempty"`
	// Shape is what an unrecognised screen was seen AS: role counts, members, normalised
	// envelope, closed interface vocabulary. No raw text, no coordinate that means anything
	// off this window, no title, no screenshot. It is the same value the place store keys an
	// identity on, carried whole so a promotion establishes the screen that was seen rather
	// than a near-duplicate of it.
	Shape *StructureSignature `json:"shape,omitempty"`
	// Called is what the screen APPEARS to be called, when a name settled by recurrence.
	//
	// The one word of somebody's display kept here, and it is one that already crosses under
	// no licence at all: AdmittedPlaceName is unconditional and applies the same shape filter
	// every other admitted text passes. It is kept because a Place established without a name
	// is a Place a later Learn has to interrupt somebody about — see
	// [[ADR-076-a-place-may-say-what-it-appears-to-be-called]].
	Called string `json:"called,omitempty"`
}

// WatchedEdge is repeated evidence about one semantic relationship.
type WatchedEdge struct {
	// ID is a stable, content-derived handle. It is NOT the identity test — matching a new
	// sighting to a candidate compares structure, exactly as CompareStructure is the identity
	// test for a subject and its content-derived id is not.
	ID          string `json:"id"`
	Application string `json:"application"`
	// From and To are the two ends of the relationship.
	From WatchedEnd `json:"from"`
	To   WatchedEnd `json:"to"`
	// Kind is what the person did, and Target what they did it to. A closed vocabulary and an
	// admitted name; see the ambient package for both.
	Kind   string `json:"kind"`
	Target string `json:"target,omitempty"`
	// Role is the sort of control the target was, when it was resolved.
	Role string `json:"role,omitempty"`

	// Seen is every crossing of this edge; Occasions is how many of them were INDEPENDENT.
	//
	// Held apart, and the distinction is the whole of what makes repetition mean anything.
	// Somebody flicking back and forth between two pages produces twenty crossings in a
	// minute and has shown Marco one thing; the same route on two afternoons is two
	// occasions. See ambient.Independent for the rule.
	Seen      int `json:"seen"`
	Occasions int `json:"occasions"`
	// Contradicted counts crossings that began at the same screen, by the same action on the
	// same control, and arrived somewhere ELSE.
	//
	// Counted rather than resolved. A majority is not an answer here: the same button leading
	// to two different places means Marco does not understand the screen, and promoting the
	// more frequent one would be a coin toss dressed as knowledge.
	Contradicted int `json:"contradicted,omitempty"`

	First time.Time `json:"first"`
	Last  time.Time `json:"last"`
	// Promoted is when this became durable knowledge, zero while it has not.
	//
	// Kept after promotion rather than deleted, so further sightings strengthen this record
	// instead of starting a second one beside knowledge that already exists.
	Promoted time.Time `json:"promoted,omitzero"`
}

// Recognised reports whether Marco already has an identity for this end.
func (e WatchedEnd) Recognised() bool { return e.Subject != "" }

// Describable reports whether this end can be resolved at promotion time — either because Marco
// already recognises it, or because the structure it was seen as would let a licensed operation
// establish it.
func (e WatchedEnd) Describable() bool {
	if e.Recognised() {
		return true
	}
	return e.Shape != nil && e.Shape.Discriminating()
}

// Known reports whether both endpoints already have durable identities.
func (w WatchedEdge) Known() bool { return w.From.Recognised() && w.To.Recognised() }

// Describable reports whether both endpoints can be resolved at promotion time.
func (w WatchedEdge) Describable() bool {
	return w.From.Describable() && w.To.Describable()
}

// WeakerThan orders two candidates for eviction, weakest first.
//
// # Deterministic, and the order is a judgement about what is worth keeping
//
// A candidate already promoted is the most valuable thing here — it is the provenance of durable
// knowledge — and goes last. Then the ones closest to promotion by independent occasions, because
// discarding those loses the most work. Contradicted candidates go early: they are evidence Marco
// does not understand the screen, and re-accumulating them costs one more sighting.
//
// Ties break on last-seen and then on id, so eviction never depends on map order. A store that
// dropped whichever candidate a range happened to reach first would forget different things on
// two runs over the same evidence.
func (w WatchedEdge) WeakerThan(other WatchedEdge) bool {
	wp, op := !w.Promoted.IsZero(), !other.Promoted.IsZero()
	if wp != op {
		return op
	}
	wc, oc := w.Contradicted > 0, other.Contradicted > 0
	if wc != oc {
		return wc
	}
	if w.Occasions != other.Occasions {
		return w.Occasions < other.Occasions
	}
	if w.Seen != other.Seen {
		return w.Seen < other.Seen
	}
	if !w.Last.Equal(other.Last) {
		return w.Last.Before(other.Last)
	}
	return w.ID < other.ID
}

// WatchedID is a stable handle for one candidate.
//
// Content-derived from the semantic identity and NOT from anything a sighting happens to carry:
// no timestamp, no counter, no session, no coordinate. Two sightings of the same relationship on
// two different days produce the same id, which is what makes cross-session evidence add up.
//
// The endpoints contribute their durable id where they have one and their structure where they do
// not, so a screen that becomes recognised part-way through changes the candidate's id — and the
// fold, which matches on structure rather than on this, carries the evidence across.
func WatchedID(application string, from, to WatchedEnd, kind, target string) string {
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0})
		}
	}
	write(strings.ToLower(strings.TrimSpace(application)), kind,
		strings.ToLower(strings.TrimSpace(target)))
	write(from.digest(), to.digest())
	return "watched_" + hex.EncodeToString(h.Sum(nil))[:12]
}

// digest describes one end of a candidate for the id.
//
// The NAME is deliberately absent from it. What a screen appears to be called is perception's
// current reading and can change between two sightings of the same screen; folding it into the
// handle would make the second sighting a different candidate, and the evidence would never add
// up. Identity is structure, here as everywhere.
func (e WatchedEnd) digest() string {
	if e.Recognised() {
		return "subject:" + e.Subject
	}
	if e.Shape == nil {
		return "unknown"
	}
	roles := make([]string, 0, len(e.Shape.Roles))
	for r, n := range e.Shape.Roles {
		roles = append(roles, fmt.Sprintf("%s:%d", r, n))
	}
	sort.Strings(roles)
	terms := make([]string, 0, len(e.Shape.Terms))
	for _, t := range e.Shape.Terms {
		terms = append(terms, string(t))
	}
	sort.Strings(terms)
	return fmt.Sprintf("shape:%s|%d|%s|%s", e.Shape.Subject, e.Shape.Members,
		strings.Join(roles, ","), strings.Join(terms, ","))
}

// WatchedStore is where candidate evidence lives.
//
// Narrower than Memory in the direction that matters: it can summarise what watching has seen and
// nothing else. There is no SemanticKnowledge here, no relationship, no goal and no authority, so
// a caller holding one cannot make anything plannable — which is the whole point of candidate
// evidence being a separate record.
type WatchedStore interface {
	// RememberWatched writes one candidate summary, replacing any it already holds under the
	// same id. It never appends: repeated evidence about one relationship is one record with
	// bigger numbers on it.
	RememberWatched(e WatchedEdge) error
	// Watched returns every candidate for one application, in a stable order.
	Watched(application string) []WatchedEdge
}
