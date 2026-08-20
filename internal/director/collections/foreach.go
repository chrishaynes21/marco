package collections

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Iteration.
//
//	Iteration advances only after the current member is verified.
//
// ForEach is a DIRECTOR construct, not a Marco one. Marco executes one deterministic
// member action at a time and knows nothing about the set; the Director owns the loop
// because between members it must observe, re-resolve, evaluate policy and verify —
// none of which a language-level loop could do without becoming the Director.
//
// There is deliberately no `while`, no nesting and no parallelism. Each of those
// changes the loop from "a bounded set, one at a time" into something whose extent
// cannot be shown to a person before it runs.

// ForEach applies one semantic operation to every member of a collection.
type ForEach struct {
	// Collection names a captured collection. Empty when the collection is inline.
	Collection string `json:"collection,omitempty"`
	// Inline is the anonymous collection for a phrase that names its members directly
	// — "click every selected item" — rather than referring to a captured one.
	//
	// Anonymous rather than auto-named, because naming it would put a collection in the
	// program's namespace that the user never asked for and could then collide with one
	// they did.
	Inline *Collection `json:"inline,omitempty"`
	// Operation is the per-member template: the verb and parameters applied to the
	// current member. Its TARGET is filled in per iteration and is never part of the
	// template — that is the whole point.
	Operation directorapi.Intent `json:"operation"`
	// Limit bounds the iteration independently of the collection's own limit.
	Limit int `json:"limit"`
}

// Validate checks a ForEach is bounded and coherent.
func (f ForEach) Validate() error {
	if f.Collection == "" && f.Inline == nil {
		return fmt.Errorf("collections: an iteration needs something to iterate over")
	}
	if f.Collection != "" && f.Inline != nil {
		return fmt.Errorf("collections: an iteration cannot be over both a named and an " +
			"inline collection")
	}
	if f.Inline != nil {
		if err := f.Inline.Validate(); err != nil {
			return err
		}
	}
	if f.Limit <= 0 {
		return fmt.Errorf("collections: an iteration must be bounded")
	}
	if f.Limit > MaximumIterations {
		return fmt.Errorf("collections: %d iterations exceeds the maximum of %d",
			f.Limit, MaximumIterations)
	}
	if f.Operation.Verb == "" {
		return fmt.Errorf("collections: an iteration needs an operation to apply")
	}
	return nil
}

// Describe names the iteration for a person.
func (f ForEach) Describe() string {
	over := f.Collection
	if f.Inline != nil {
		over = f.Inline.Query.Describe()
	}
	return fmt.Sprintf("apply %s to %s", f.Operation.Verb, over)
}

// IterationState is how one member's turn ended.
type IterationState string

const (
	IterationVerified   IterationState = "verified"
	IterationFailed     IterationState = "failed"
	IterationUnverified IterationState = "unverified"
	IterationUnsafe     IterationState = "unsafe"
	IterationAmbiguous  IterationState = "ambiguous"
	IterationCancelled  IterationState = "cancelled"
	// IterationNoProgress: the operation ran and the world did not change in any way
	// that shows it took effect. Distinct from failed, and it STOPS: repeating an
	// operation that already appears to do nothing is how a loop applies it fifty times.
	IterationNoProgress IterationState = "no_progress"
	// IterationIdentityUncertain: the member cannot be told apart from its siblings
	// well enough to guarantee it is not processed twice.
	IterationIdentityUncertain IterationState = "identity_uncertain"
)

// Terminal reports whether this state stops the collection.
//
// Everything except a verified iteration stops. That is the conjunction rule the whole
// program model rests on, applied one level down: a collection that continued past a
// failure would act on later members in a world the failed member was supposed to have
// produced.
func (s IterationState) Terminal() bool { return s != IterationVerified }

// IterationResult is one member's turn.
type IterationResult struct {
	Index  int            `json:"index"`
	Member Summary        `json:"member"`
	State  IterationState `json:"state"`
	// ActionNode is the durable node this iteration produced, empty when it made none.
	ActionNode string `json:"action_node,omitempty"`
	Reason     string `json:"reason,omitempty"`
	// Progress explains WHAT verified progress this member produced. It explains the
	// verification rather than replacing it — see collections.ClassifyProgress.
	Progress ProgressKind `json:"progress_kind,omitempty"`
}

// CollectionState is how a whole iteration ended.
type CollectionState string

const (
	CollectionCompleted CollectionState = "completed"
	CollectionStopped   CollectionState = "stopped"
	CollectionEmpty     CollectionState = "empty"
	CollectionRefused   CollectionState = "refused"
	// CollectionAwaitingClarification is a PAUSE, not a stop. The program is alive,
	// the processed ledger is intact, and an answer resumes the same member.
	CollectionAwaitingClarification CollectionState = "awaiting_clarification"
	// CollectionUnobservable: the world could not answer. Never reported as empty.
	CollectionUnobservable CollectionState = "unobservable"
)

// Result is the whole iteration's account.
type Result struct {
	CollectionName string            `json:"collection_name,omitempty"`
	Query          string            `json:"query"`
	State          CollectionState   `json:"state"`
	Completed      int               `json:"completed"`
	Attempted      int               `json:"attempted"`
	Matched        int               `json:"matched"`
	Limit          int               `json:"limit"`
	Iterations     []IterationResult `json:"iterations"`
	// Policy is the collection-level decision. Present even on a refusal — especially
	// on a refusal, since that is when a reader needs to see what was decided and why.
	Policy *BulkDecision `json:"policy,omitempty"`
	// PausedAt is the REAL iteration a clarification stopped at, so a resume continues
	// from it rather than counting again. Inferring it from how many members remain
	// would be wrong the moment the set changed while the user was reading.
	PausedAt int `json:"paused_at,omitempty"`
	// Offered is the ordered contender fingerprint shown at the pause, and EventID
	// identifies that offer. Digests only — see MembershipFingerprint. Together they
	// are what lets a later answer be checked against what the user actually saw.
	Offered MembershipFingerprint `json:"offered,omitempty"`
	EventID string                `json:"event_id,omitempty"`
	Reason  string                `json:"reason,omitempty"`
}

// Summarise renders the result for a person.
//
// "8 of 10 verified" is never reported as success. A partial collection is a STOPPED
// one, and saying so is what stops a caller treating it as done.
func (r Result) Summarise() string {
	switch r.State {
	case CollectionCompleted:
		return fmt.Sprintf("%d of %d member(s) verified", r.Completed, r.Attempted)
	case CollectionEmpty:
		return r.Reason
	case CollectionAwaitingClarification:
		return r.Reason
	}
	return fmt.Sprintf("Stopped after %d of %d: %s", r.Completed, r.Matched, r.Reason)
}
