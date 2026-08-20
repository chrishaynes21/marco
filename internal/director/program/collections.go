package program

import (
	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// CollectionResume is a clarification answer waiting to be applied to one paused
// collection member.
//
//	A paused collection retains progress, not stale members.
//	Clarification resumes one semantic member; it never restarts the collection.
//
// Everything here is either a POSITION or a NARROWING. There is no resolved element, no
// window handle and no member list, because none of those would still be true when the
// answer arrives: the user was thinking while the screen carried on. What survives a
// pause is the query (on the collection), the processed ledger (on the collection
// environment) and the two fields below.
//
// The ordinal is the one that needs care. It refers to the contender list of ONE
// clarification event and nothing else — not to the collection's ordering, not to a
// future resolution, and never to the stored query. It is applied once, to one member,
// and discarded.
type CollectionResume struct {
	// Ledger identifies which collection's iteration was paused.
	Ledger string
	// Iteration is the REAL position, so a resumed program reports "[3/5]" rather than
	// starting its count again. Inferring it from how many members remain would be
	// wrong the moment the set changed while the user was reading.
	Iteration int

	// Ordinal picks the Nth contender of the clarification event that paused this
	// member. 0 means the answer narrowed by role alone.
	Ordinal int
	// Role narrows to a kind of control ("the tab", "the button").
	Role directorapi.ElementRole

	// EventID is the clarification this answer belongs to. An answer that arrives after
	// a fresh question replaced the old one must not be applied to the new contender
	// list — the same failure as a stale ordinal, reached a different way.
	EventID string
	// Offered is the ordered contender fingerprint the user was shown. The ordinal is
	// applied only when this still describes what is there — see collections.CompareMembership.
	Offered collections.MembershipFingerprint
}

// Applies reports whether this answer is for the given collection.
func (r *CollectionResume) Applies(ledger string) bool {
	return r != nil && r.Ledger == ledger
}

// Narrow applies the answer to one member's query.
//
// Applied to the MEMBER's query rather than to the collection's, which is the whole
// distinction: the collection still means what it meant, and only this one member's
// resolution is narrowed. Writing the ordinal into the stored query would make every
// future iteration pick the same contender.
func (r *CollectionResume) Narrow(q *directorapi.ElementQuery) {
	if r == nil || q == nil {
		return
	}
	if r.Role != "" {
		q.Role = r.Role
	}
	if r.Ordinal > 0 {
		q.Ordinal = r.Ordinal
	}
}
