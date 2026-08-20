package collections

import (
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Iteration progress classification.
//
//	Iteration advances only when the current member produced verified semantic
//	progress.
//
// This layer EXPLAINS verification; it never replaces it. The verifier has already
// decided whether the action can be proved, and it requires positive evidence to say
// yes — so "verified" already means something happened. What a bulk operation needs on
// top of that is WHICH something, because the answers differ in what they imply about
// continuing:
//
//   - a member that disappeared is normal, and is what "close every window" looks like
//     when it works;
//   - a member whose state changed is normal, and is what "focus every item" looks like;
//   - a member that is still there, unchanged, is the dangerous one. Repeating the
//     operation on it would be a loop applying the same ineffective action fifty times,
//     and that is the case this classification exists to catch by name.
//
// Everything here is derived from evidence the verifier already gathered. No extra
// observation, no coordinates, and no guessing from what the operation was supposed to
// do — only from what was actually seen.

// ProgressKind is what sort of verified progress an iteration produced.
//
// A CLOSED vocabulary. A free-form string would let a new call site invent a
// classification nothing downstream knows how to reason about, which is how a
// "probably fine" would eventually be treated as progress.
type ProgressKind string

const (
	// ProgressMemberCompleted: the postcondition verified, without a more specific
	// story to tell.
	ProgressMemberCompleted ProgressKind = "member_completed"
	// ProgressMemberRemoved: the member existed before and is gone now. Normal, and
	// what a successful "close every window" produces.
	ProgressMemberRemoved ProgressKind = "member_removed"
	// ProgressMemberStateChanged: the member remains and a relevant state changed —
	// focus acquired, selection changed, value changed.
	ProgressMemberStateChanged ProgressKind = "member_state_changed"
	// ProgressMemberUnchanged: the member remains with no verified progress. STOPS.
	ProgressMemberUnchanged ProgressKind = "member_unchanged"
	// ProgressCollectionReordered: the order moved but identity is still trustworthy.
	ProgressCollectionReordered ProgressKind = "collection_reordered"
	// ProgressCollectionUnobservable: the world stopped answering. Never "empty".
	ProgressCollectionUnobservable ProgressKind = "collection_unobservable"
)

// Advances reports whether this classification permits moving to the next member.
//
// Only three do. Unchanged is the important exclusion: the operation ran, the world did
// not move, and doing it again would be a loop.
func (k ProgressKind) Advances() bool {
	switch k {
	case ProgressMemberCompleted, ProgressMemberRemoved, ProgressMemberStateChanged:
		return true
	}
	return false
}

// Describe renders the classification for a person.
func (k ProgressKind) Describe() string {
	switch k {
	case ProgressMemberCompleted:
		return "the operation verified"
	case ProgressMemberRemoved:
		return "the member is no longer present"
	case ProgressMemberStateChanged:
		return "the member's state changed"
	case ProgressMemberUnchanged:
		return "the member is unchanged"
	case ProgressCollectionReordered:
		return "the collection reordered"
	case ProgressCollectionUnobservable:
		return "the collection can no longer be observed"
	}
	return string(k)
}

// Evidence is what the classifier reasons from.
//
// All of it comes from the verifier and from the collection resolution that follows.
// There is deliberately no Bounds and no Point: classifying progress from coordinates
// would call a repainted list "changed" and a scrolled one "removed".
type Evidence struct {
	// Verified is the verifier's own verdict. Authoritative.
	Verified bool
	// Inconclusive says the verifier could not tell, which is not the same as no.
	Inconclusive bool
	// Kinds are the evidence kinds the verifier recorded.
	Kinds []string
	// Observable reports whether the collection could still be read afterwards.
	Observable bool
	// MemberPresent reports whether this member is still in the resolved membership.
	MemberPresent bool
	// OrderChanged reports that the membership sequence moved.
	OrderChanged bool
}

// removalEvidence are the verifier's ways of saying the target went away.
var removalEvidence = map[string]bool{
	"target_gone": true,
}

// noChangeEvidence is the verifier saying it looked and nothing moved.
var noChangeEvidence = map[string]bool{
	"nothing_changed": true, "focus_unchanged": true,
}

// stateChangeEvidence are the verifier's ways of saying the target itself changed.
var stateChangeEvidence = map[string]bool{
	"focus_on_target": true, "focus_changed": true, "target_state_changed": true,
	"value_matches": true, "value_changed": true, "menu_opened": true,
}

// ClassifyProgress decides what verified progress an iteration produced.
//
// The precedence is fixed and total, so two conflicting terminal classifications cannot
// both be reached:
//
//	collection unobservable
//	→ member removed
//	→ explicit no-change evidence
//	→ verified member state change
//	→ verified semantic completion
//	→ collection reordered
//	→ member unchanged (the residue)
//
// Unobservable comes first because it invalidates every other reading: a world that
// stopped answering cannot tell us the member is gone. Removal comes before state
// change because a member that no longer exists has no state to have changed. Unchanged
// is LAST, as the residue — the answer when nothing positive was established.
func ClassifyProgress(e Evidence) ProgressKind {
	if !e.Observable {
		return ProgressCollectionUnobservable
	}
	if !e.MemberPresent && hasAny(e.Kinds, removalEvidence) {
		return ProgressMemberRemoved
	}
	if !e.MemberPresent && e.Verified {
		// Gone, and the action verified. The verifier did not name removal, but the
		// member is not there and the operation proved itself — reporting it as removed
		// is the only reading that fits both facts.
		return ProgressMemberRemoved
	}
	// EXPLICIT no-change evidence is decisive for a loop, whatever the single-action
	// verdict was. The verifier may be lenient about one action — a best-effort
	// operation can pass without proving much — but a loop that advanced past a member
	// the verifier had positively observed to be unchanged would leave it eligible
	// again and apply the same ineffective operation until the limit.
	//
	// Placed after removal so a member that vanished is still reported as removed.
	if hasAny(e.Kinds, noChangeEvidence) {
		return ProgressMemberUnchanged
	}
	if e.Verified && hasAny(e.Kinds, stateChangeEvidence) {
		return ProgressMemberStateChanged
	}
	if e.Verified {
		return ProgressMemberCompleted
	}
	if e.OrderChanged && !e.Inconclusive {
		return ProgressCollectionReordered
	}
	return ProgressMemberUnchanged
}

// hasAny reports whether any of the kinds is in the set.
func hasAny(kinds []string, set map[string]bool) bool {
	for _, k := range kinds {
		if set[strings.ToLower(k)] {
			return true
		}
	}
	return false
}

// EvidenceKinds extracts the kinds from a verification result.
func EvidenceKinds(v directorapi.VerificationResult) []string {
	out := make([]string, 0, len(v.Evidence))
	for _, e := range v.Evidence {
		out = append(out, e.Kind)
	}
	return out
}
