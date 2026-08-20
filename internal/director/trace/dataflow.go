package trace

import (
	"time"
)

// Value data-flow events.
//
//	A value may be observable as metadata while it lives, but its contents remain
//	governed by its visibility.
//
// These events are SAFE BY CONSTRUCTION rather than safe by redaction, and that is the
// central decision in this file. ValueEvent has no field capable of holding content —
// no Text, no Value, no any — so there is nothing to scrub at a boundary and nothing a
// future caller can accidentally put content into. A struct with a string payload would
// have to be audited at every emit site forever; this one cannot leak whatever anyone
// does with it.
//
// They are emitted by the code that PERFORMS each operation, never reconstructed
// afterwards from history. A reconstruction would be a second account of what happened,
// derived from the first, and the two would disagree exactly when it mattered — when
// something went wrong.

// ValueEventKind names one point in a value's life.
type ValueEventKind string

const (
	// EventCaptureStarted: before the source is read. No value data exists yet.
	EventCaptureStarted ValueEventKind = "capture_value_started"
	// EventCaptureCompleted: the capture's result is known, success or not.
	EventCaptureCompleted ValueEventKind = "capture_value_completed"
	// EventValueBound: Bind succeeded. A refused duplicate does not emit this.
	EventValueBound ValueEventKind = "value_bound"
	// EventValueRead: a later step retrieved ${name} from the environment.
	EventValueRead ValueEventKind = "value_read"
	// EventValueConsumed: the resulting operation received the value and proceeded.
	EventValueConsumed ValueEventKind = "value_consumed"
	// EventEnvironmentCleared: the program's values were discarded. Exactly once.
	EventEnvironmentCleared ValueEventKind = "value_environment_cleared"
)

// ValueEvent is one safe, structured data-flow record.
//
// One struct with a Kind discriminator rather than six types. The fields overlap
// heavily, six near-identical types would drift, and — the deciding reason — a single
// type means a single place to be sure no content field ever appears.
type ValueEvent struct {
	Kind ValueEventKind `json:"kind"`
	At   time.Time      `json:"at"`

	ProgramID string `json:"program_id,omitempty"`
	StepID    string `json:"step_id,omitempty"`
	StepIndex int    `json:"step_index,omitempty"`
	StepCount int    `json:"step_count,omitempty"`

	// Name is the value's name. User-chosen and already visible in the program they
	// typed, so it is safe to report — it is a label, not content.
	Name string `json:"value_name,omitempty"`
	// ValueKind, Visibility and ByteLength describe the value without being it. The
	// length is the most useful safe fact there is: it distinguishes an empty capture
	// from a full one without revealing either.
	ValueKind  string `json:"value_kind,omitempty"`
	Visibility string `json:"visibility,omitempty"`
	ByteLength int    `json:"byte_length,omitempty"`

	SourceKind  string `json:"source_kind,omitempty"`
	CaptureKind string `json:"capture_kind,omitempty"`
	Verified    bool   `json:"verified,omitempty"`

	// Outcome is how the operation ended; Reason explains it in safe prose.
	Outcome string `json:"outcome,omitempty"`
	Reason  string `json:"safe_reason,omitempty"`

	// Consumption detail.
	Operation              string `json:"operation_kind,omitempty"`
	DestinationRole        string `json:"destination_role,omitempty"`
	DestinationApplication string `json:"destination_application,omitempty"`
	ExpectedInputType      string `json:"expected_input_type,omitempty"`
	Compatibility          string `json:"compatibility_result,omitempty"`

	// Cleanup detail.
	ValueCount    int    `json:"value_count,omitempty"`
	TerminalState string `json:"terminal_state,omitempty"`

	// Collection detail. Counts and positions — never a member.s label, which is why
	// MemberDigest is a digest.
	Collection     string `json:"collection_name,omitempty"`
	CollectionKind string `json:"collection_kind,omitempty"`
	QuerySummary   string `json:"query_summary,omitempty"`
	MatchedCount   int    `json:"matched_count,omitempty"`
	CompletedCount int    `json:"completed_count,omitempty"`
	Iteration      int    `json:"iteration_index,omitempty"`
	Limit          int    `json:"iteration_limit,omitempty"`
	MemberDigest   string `json:"member_digest,omitempty"`
	// Progress is the iteration.s progress classification, from verification evidence.
	Progress string `json:"progress_kind,omitempty"`
	// ChangeKind and OldCount describe a membership drift between a pause and a
	// resume. Counts and a closed vocabulary — never a member.
	ChangeKind string `json:"change_kind,omitempty"`
	OldCount   int    `json:"old_count,omitempty"`
	EventID    string `json:"event_id,omitempty"`
}

// maxValueEvents bounds one command's data-flow record.
//
// A program captures at most 20 values across at most 10 steps, so a run that produced
// more events than this is a loop rather than a program. Bounded for the same reason
// every other diagnostic here is: a diagnostic that can grow without limit is a way to
// exhaust memory from the outside.
const maxValueEvents = 256

// Emit records one data-flow event.
//
// Safe on a nil Trace, like everything else here: tracing is optional, and a caller
// should never have to check before reporting what it did.
func (t *Trace) Emit(e ValueEvent) {
	if t == nil {
		return
	}
	if e.At.IsZero() {
		e.At = time.Now()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.events) >= maxValueEvents {
		return
	}
	t.events = append(t.events, e)
}

// ValueEvents returns the data-flow events, as a copy.
//
// A copy so a reader can serialise it after releasing the lock — the same discipline
// the environment snapshot follows, and for the same reason.
func (t *Trace) ValueEvents() []ValueEvent {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]ValueEvent(nil), t.events...)
}

// CountEvents reports how many events of a kind were emitted.
//
// Exists for the "exactly once" tests. Cleanup runs through deferred paths that can
// plausibly be reached twice, and "emitted exactly once" is a claim worth being able to
// check rather than assert.
func (t *Trace) CountEvents(kind ValueEventKind) int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, e := range t.events {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

// Collection lifecycle events.
//
// Carried on the same ValueEvent type, and that reuse is the point: the type has no
// field capable of holding content, so a collection event cannot leak a member's label
// however it is populated. The fields below are all counts, names and outcomes.
const (
	EventCollectionCaptureStarted   ValueEventKind = "collection_capture_started"
	EventCollectionCaptureCompleted ValueEventKind = "collection_capture_completed"
	EventCollectionBound            ValueEventKind = "collection_bound"
	EventCollectionPolicyStarted    ValueEventKind = "collection_policy_started"
	EventCollectionPolicyCompleted  ValueEventKind = "collection_policy_completed"
	EventIterationStarted           ValueEventKind = "iteration_started"
	EventIterationResolved          ValueEventKind = "iteration_resolved"
	EventIterationCompleted         ValueEventKind = "iteration_completed"
	EventIterationFailed            ValueEventKind = "iteration_failed"
	EventCollectionPaused           ValueEventKind = "collection_paused"
	EventCollectionResumed          ValueEventKind = "collection_resumed"
	EventCollectionCompleted        ValueEventKind = "collection_completed"
	EventCollectionCleared          ValueEventKind = "collection_cleared"
	// EventCollectionMembershipChanged records that the set moved between a pause and
	// the answer, which is what decides whether an old ordinal still means anything.
	EventCollectionMembershipChanged ValueEventKind = "collection_membership_changed"
)
