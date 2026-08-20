package collections

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Membership drift across a clarification pause.
//
//	An ordinal belongs to one offered list in one observed world.
//	A fresh world may invalidate an old answer.
//
// The failure this prevents is small to describe and easy to cause. The user is offered:
//
//	1. New tab
//	2. New window
//
// They think about it. While they think, a "New folder" appears at the top. They say
// "the first one". A system that applied the ordinal to whatever list it now has would
// click New folder — a control the user never saw offered, chosen by an answer they gave
// about something else.
//
// So an ordinal is never applied because the fresh list is long enough. It is applied
// only when the position it referred to still means what it meant, which is what the
// fingerprint below establishes.

// MembershipFingerprint is a safe record of what was offered.
//
// DIGESTS, never members. It exists for one comparison — did the ordered contender
// semantics survive? — and carries nothing that could be used to act on a member
// directly. No labels, no ids, no coordinates.
type MembershipFingerprint struct {
	QueryDigest       string   `json:"query_digest"`
	OrderedKeyDigests []string `json:"ordered_key_digests"`
	MatchedCount      int      `json:"matched_count"`
}

// Fingerprint records the ordered membership at the moment a clarification was offered.
func Fingerprint(q Query, members []Member) MembershipFingerprint {
	f := MembershipFingerprint{
		QueryDigest:  Digest(q.Describe()),
		MatchedCount: len(members),
	}
	for _, m := range members {
		// Computed when absent rather than skipped. A member that has not been through
		// Resolve still has an identity, and a fingerprint that silently dropped it
		// would compare as unchanged against anything.
		key := m.Key
		if key == "" {
			key = SemanticKey(m, q)
		}
		f.OrderedKeyDigests = append(f.OrderedKeyDigests, Digest(key))
	}
	return f
}

// FingerprintCandidates records the ordered CONTENDERS a clarification offered.
//
// Contenders are what an ordinal indexes, and they are not the same list as the
// collection's members: the members are the set being iterated, the contenders are the
// several controls one member's own resolution could not choose between.
func FingerprintCandidates(q Query, keys []string) MembershipFingerprint {
	f := MembershipFingerprint{
		QueryDigest:  Digest(q.Describe()),
		MatchedCount: len(keys),
	}
	for _, k := range keys {
		f.OrderedKeyDigests = append(f.OrderedKeyDigests, Digest(k))
	}
	return f
}

// DriftKind is how membership changed between an offer and an answer.
type DriftKind string

const (
	// DriftUnchanged: the same members in the same order.
	DriftUnchanged DriftKind = "unchanged"
	// DriftChosenPresent: the set moved, but the chosen contender is still there and
	// still identifiable.
	DriftChosenPresent DriftKind = "chosen_member_present"
	// DriftChosenDisappeared: the contender the user picked is gone.
	DriftChosenDisappeared DriftKind = "chosen_member_disappeared"
	// DriftNewContender: something new is in the list. The old ordinal is meaningless.
	DriftNewContender DriftKind = "new_contender_appeared"
	// DriftOrderChanged: the same members, in a different order.
	DriftOrderChanged DriftKind = "order_changed"
	// DriftEmpty: nothing matches now. A fact, not a failure to see.
	DriftEmpty DriftKind = "collection_empty"
	// DriftUnobservable: the world stopped answering. Never reported as empty.
	DriftUnobservable DriftKind = "collection_unobservable"
	// DriftIdentityUncertain: the members cannot be told apart well enough to say.
	DriftIdentityUncertain DriftKind = "identity_uncertain"
)

// Resumable reports whether an ordinal answer may still be applied.
//
// Only two kinds permit it, and both mean the same thing: the position the user chose
// still refers to the control they were shown. Everything else requires a fresh
// question, because answering it would be putting a choice in the user's mouth.
func (d DriftKind) Resumable() bool {
	return d == DriftUnchanged || d == DriftChosenPresent
}

// Describe renders the drift for a person, in the words the user needs.
func (d DriftKind) Describe() string {
	switch d {
	case DriftUnchanged, DriftChosenPresent:
		return ""
	case DriftChosenDisappeared:
		return "That item is no longer present."
	case DriftNewContender:
		return "The choices have changed since you were asked."
	case DriftOrderChanged:
		return "The choices are in a different order than when you were asked."
	case DriftEmpty:
		return "The collection is now empty."
	case DriftUnobservable:
		return "Director can no longer observe this collection."
	case DriftIdentityUncertain:
		return "The collection changed and member identity is no longer reliable " +
			"enough to resume safely."
	}
	return string(d)
}

// CompareMembership classifies what happened between an offer and an answer.
//
// chosen is the 1-based ordinal the user gave, 0 when they narrowed by role instead.
//
// The comparison is on DIGESTS in ORDER, which is exactly the property an ordinal
// depends on. Comparing counts would accept the New-folder case; comparing sets
// unordered would accept a reordering, and "the first one" would pick a different
// control than the one that was first.
func CompareMembership(offered MembershipFingerprint, current MembershipFingerprint,
	chosen int, observable bool) DriftKind {

	if !observable {
		return DriftUnobservable
	}
	if current.MatchedCount == 0 {
		return DriftEmpty
	}
	if offered.QueryDigest != current.QueryDigest {
		// The question itself changed. Nothing about the old answer can be trusted.
		return DriftIdentityUncertain
	}
	if equalDigests(offered.OrderedKeyDigests, current.OrderedKeyDigests) {
		return DriftUnchanged
	}

	// The user picked a position. Its meaning survives only if that exact position
	// still holds the same contender — not if the list merely still has enough items.
	if chosen > 0 {
		if chosen > len(current.OrderedKeyDigests) {
			return DriftChosenDisappeared
		}
		want := offered.at(chosen)
		if want == "" {
			return DriftIdentityUncertain
		}
		if current.at(chosen) == want {
			// Same contender, same position. The answer still means what it meant,
			// even though other entries moved.
			return DriftChosenPresent
		}
		if !contains(current.OrderedKeyDigests, want) {
			return DriftChosenDisappeared
		}
		// Still present, but somewhere else. The ordinal no longer points at it, and
		// applying it would select whatever took its place.
		if len(current.OrderedKeyDigests) > len(offered.OrderedKeyDigests) {
			return DriftNewContender
		}
		return DriftOrderChanged
	}

	// No ordinal: the answer narrowed by role, which does not depend on position.
	if len(current.OrderedKeyDigests) > len(offered.OrderedKeyDigests) {
		return DriftNewContender
	}
	return DriftOrderChanged
}

// at reads the digest at a 1-based position.
func (f MembershipFingerprint) at(ordinal int) string {
	if ordinal < 1 || ordinal > len(f.OrderedKeyDigests) {
		return ""
	}
	return f.OrderedKeyDigests[ordinal-1]
}

func equalDigests(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// ClarificationEventID identifies one offered choice.
//
// An answer belongs to ONE event. Without an id, a response arriving after a fresh
// question had replaced the old one would be applied to the new contender list — the
// same failure as a stale ordinal, reached a different way.
type ClarificationEventID string

// NewEventID mints an id from the offer's content, so the same offer in the same world
// produces the same id and a changed offer produces a different one.
func NewEventID(collection string, iteration int, f MembershipFingerprint) ClarificationEventID {
	h := sha256.New()
	h.Write([]byte(collection))
	h.Write([]byte{0})
	h.Write([]byte(strings.Join(f.OrderedKeyDigests, ",")))
	h.Write([]byte{0})
	h.Write([]byte(f.QueryDigest))
	return ClarificationEventID("clarification_" +
		hex.EncodeToString(h.Sum(nil))[:8] + "_" + itoa(iteration))
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
