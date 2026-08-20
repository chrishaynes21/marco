package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
)

// Collection events across the REAL protocol boundary.
//
//	Runtime truth and client-visible truth must be identical across the protocol
//	boundary.
//
// Producer-side tests are not enough, and this package knows that from experience: the
// captured-values milestone had events that marshalled correctly, crossed the wire
// correctly, and were then silently dropped by the client's decode — invisible, with
// every producing-side test green. The only way to catch that class of bug is to run
// the whole path and compare.
//
// So this test does not check that the runtime EMITS events. It checks that what a
// client ends up holding is what the runtime had.

// collectionRun is one realistic execution: capture, policy, a verified member, a
// pause, drift, a resume, a completion and cleanup.
//
// Built by hand rather than by running a pipeline, because the subject here is the
// TRANSPORT. A fixture that depended on execution would fail for reasons that have
// nothing to do with serialisation.
func collectionRun() []trace.ValueEvent {
	return []trace.ValueEvent{
		{Kind: trace.EventCollectionCaptureStarted, ProgramID: "action_310", StepID: "s1",
			Collection: "tabs", CollectionKind: "targets", QuerySummary: "tab in code"},
		{Kind: trace.EventCollectionCaptureCompleted, ProgramID: "action_310", StepID: "s1",
			Collection: "tabs", MatchedCount: 5, Outcome: "ok"},
		{Kind: trace.EventCollectionBound, ProgramID: "action_310", StepID: "s1",
			Collection: "tabs", CollectionKind: "targets", MatchedCount: 5, Limit: 10},
		{Kind: trace.EventCollectionPolicyStarted, ProgramID: "action_310", StepID: "s2",
			Collection: "tabs", Operation: "focus"},
		{Kind: trace.EventCollectionPolicyCompleted, ProgramID: "action_310", StepID: "s2",
			Collection: "tabs", Operation: "focus", MatchedCount: 5,
			Outcome: "permitted", Reason: "focus on 5 items is low risk and reversible"},
		{Kind: trace.EventIterationStarted, ProgramID: "action_310", StepID: "s2",
			Collection: "tabs", Iteration: 1, MemberDigest: "18d4aa01bb22"},
		{Kind: trace.EventIterationResolved, ProgramID: "action_310", StepID: "s2",
			Collection: "tabs", Iteration: 1, MemberDigest: "18d4aa01bb22"},
		{Kind: trace.EventIterationCompleted, ProgramID: "action_310", StepID: "s2",
			Collection: "tabs", Iteration: 1, CompletedCount: 1,
			MemberDigest: "18d4aa01bb22", Progress: "member_state_changed", Outcome: "verified"},
		{Kind: trace.EventCollectionPaused, ProgramID: "action_310", StepID: "s2",
			Collection: "tabs", Iteration: 2, CompletedCount: 1, MatchedCount: 5,
			EventID: "clarification_9f2a1c04_2", Outcome: "awaiting_clarification"},
		{Kind: trace.EventCollectionMembershipChanged, ProgramID: "action_310", StepID: "s2",
			Collection: "tabs", Iteration: 2, CompletedCount: 1,
			ChangeKind: "new_contender_appeared", EventID: "clarification_9f2a1c04_2",
			OldCount: 2, MatchedCount: 3,
			Reason: "The choices have changed since you were asked."},
		{Kind: trace.EventCollectionResumed, ProgramID: "action_310", StepID: "s2",
			Collection: "tabs", Iteration: 2, CompletedCount: 1},
		{Kind: trace.EventIterationFailed, ProgramID: "action_310", StepID: "s2",
			Collection: "tabs", Iteration: 2, CompletedCount: 1,
			MemberDigest: "c0921de4ff10", Progress: "member_unchanged",
			Outcome: "no_progress",
			Reason:  "Iteration stopped: no verified progress for the current member."},
		{Kind: trace.EventCollectionCompleted, ProgramID: "action_310", StepID: "s2",
			Collection: "tabs", MatchedCount: 5, CompletedCount: 1, Limit: 10,
			Outcome: "stopped"},
		{Kind: trace.EventCollectionCleared, ProgramID: "action_310",
			MatchedCount: 1, TerminalState: "failed",
			Reason: "the program reached a terminal state"},
	}
}

// throughProtocol sends a trace the way the service does and decodes it the way the
// client does — the same MarshalJSON, the same envelope, the same Decode.
func throughProtocol(t *testing.T, src *trace.Trace) *trace.Trace {
	t.Helper()
	// What the server puts on the wire.
	resp := NewResponse("req_1", ResponsePerception, src)
	wire, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("server marshal: %v", err)
	}
	// What the client reads off it.
	var envelope ResponseEnvelope
	if err := json.Unmarshal(wire, &envelope); err != nil {
		t.Fatalf("client unmarshal envelope: %v", err)
	}
	var got trace.Trace
	if err := envelope.Decode(&got); err != nil {
		t.Fatalf("client decode payload: %v", err)
	}
	return &got
}

func TestEveryCollectionEventSurvivesTheRealProtocol(t *testing.T) {
	src := trace.New("trace_7", "focus each item in tabs")
	src.ProgramID = "action_310"
	for _, e := range collectionRun() {
		src.Emit(e)
	}
	want := src.ValueEvents()

	got := throughProtocol(t, src).ValueEvents()

	if len(got) != len(want) {
		t.Fatalf("%d events survived, want %d — the client dropped some", len(got), len(want))
	}
	for i := range want {
		w, g := want[i], got[i]
		if g.Kind != w.Kind {
			t.Errorf("event %d kind = %s, want %s (order changed)", i, g.Kind, w.Kind)
		}
		if g.ProgramID != w.ProgramID {
			t.Errorf("event %d lost its program: %q", i, g.ProgramID)
		}
		if g.StepID != w.StepID {
			t.Errorf("event %d lost its step: %q", i, g.StepID)
		}
		if g.Collection != w.Collection {
			t.Errorf("event %d lost its collection: %q", i, g.Collection)
		}
		if g.Iteration != w.Iteration {
			t.Errorf("event %d iteration = %d, want %d", i, g.Iteration, w.Iteration)
		}
		if g.CompletedCount != w.CompletedCount {
			t.Errorf("event %d completed = %d, want %d", i, g.CompletedCount, w.CompletedCount)
		}
		if g.Progress != w.Progress {
			t.Errorf("event %d lost its progress classification: %q", i, g.Progress)
		}
		if g.EventID != w.EventID {
			t.Errorf("event %d lost its clarification event id: %q", i, g.EventID)
		}
		if g.MemberDigest != w.MemberDigest {
			t.Errorf("event %d lost its member digest: %q", i, g.MemberDigest)
		}
		if g.ChangeKind != w.ChangeKind {
			t.Errorf("event %d lost its drift kind: %q", i, g.ChangeKind)
		}
		if g.Reason != w.Reason {
			t.Errorf("event %d lost its reason: %q", i, g.Reason)
		}
		if g.Outcome != w.Outcome {
			t.Errorf("event %d lost its outcome: %q", i, g.Outcome)
		}
	}
}

func TestEveryCollectionEventKindIsExercisedByTheRoundTrip(t *testing.T) {
	// A round-trip test that silently stopped covering a kind would be worse than none:
	// it would report success for a vocabulary it no longer checks.
	seen := map[trace.ValueEventKind]bool{}
	for _, e := range collectionRun() {
		seen[e.Kind] = true
	}
	for _, kind := range []trace.ValueEventKind{
		trace.EventCollectionCaptureStarted,
		trace.EventCollectionCaptureCompleted,
		trace.EventCollectionBound,
		trace.EventCollectionPolicyStarted,
		trace.EventCollectionPolicyCompleted,
		trace.EventIterationStarted,
		trace.EventIterationResolved,
		trace.EventIterationCompleted,
		trace.EventIterationFailed,
		trace.EventCollectionPaused,
		trace.EventCollectionResumed,
		trace.EventCollectionMembershipChanged,
		trace.EventCollectionCompleted,
		trace.EventCollectionCleared,
	} {
		if !seen[kind] {
			t.Errorf("%s is never exercised by the round-trip fixture", kind)
		}
	}
}

func TestTheClientCanReEncodeWithoutLosingMeaning(t *testing.T) {
	// The CLI decodes and then re-encodes to print --json. A field that survives the
	// first hop and not the second is invisible to every producer-side test.
	src := trace.New("trace_7", "focus each item in tabs")
	src.ProgramID = "action_310"
	for _, e := range collectionRun() {
		src.Emit(e)
	}

	decoded := throughProtocol(t, src)
	again, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("client re-marshal: %v", err)
	}
	for _, want := range []string{
		"collection_membership_changed", "collection_paused", "collection_resumed",
		"iteration_resolved", "member_state_changed", "member_unchanged",
		"clarification_9f2a1c04_2", "18d4aa01bb22", "new_contender_appeared",
	} {
		if !strings.Contains(string(again), want) {
			t.Errorf("re-encoding lost %q:\n%s", want, again)
		}
	}
}

func TestNoCollectionEventCarriesPrivateMemberData(t *testing.T) {
	// The event type has no field capable of holding a member label, so this is a
	// property rather than a habit — but it is asserted against the serialised form,
	// because that is what a bug report gets pasted into.
	src := trace.New("trace_7", "focus each item in tabs")
	for _, e := range collectionRun() {
		src.Emit(e)
	}
	raw, err := json.Marshal(throughProtocol(t, src).ValueEvents())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// The digests are present; the things they are digests OF are not.
	for _, forbidden := range []string{
		"element_id", "hwnd", "runtime_id", "bounds", "native_id",
		"Invoice", "Private", "@example.com",
	} {
		if strings.Contains(strings.ToLower(string(raw)), strings.ToLower(forbidden)) {
			t.Errorf("a collection event carried %q:\n%s", forbidden, raw)
		}
	}
}

func TestAnUnknownEventKindIsPreservedRatherThanDiscarded(t *testing.T) {
	// Forward compatibility. A newer service sending a kind this client has never heard
	// of must not have it silently vanish: the kind is a plain string on the wire, so
	// it survives as itself and a reader can see there was an event it does not
	// understand. Silently dropping it is the failure mode this whole file exists for.
	src := trace.New("trace_7", "focus each item in tabs")
	src.Emit(trace.ValueEvent{
		Kind:       trace.ValueEventKind("collection_something_new_in_a_later_build"),
		Collection: "tabs", Iteration: 1,
	})

	got := throughProtocol(t, src).ValueEvents()
	if len(got) != 1 {
		t.Fatalf("%d events survived, want 1 — an unknown kind was discarded", len(got))
	}
	if string(got[0].Kind) != "collection_something_new_in_a_later_build" {
		t.Fatalf("the unknown kind was rewritten to %q", got[0].Kind)
	}
	if got[0].Collection != "tabs" || got[0].Iteration != 1 {
		t.Fatalf("an unknown kind lost its known fields: %+v", got[0])
	}
}

// ── stale clarification rejection ─────────────────────────────────────────────

func TestAnAnswerToAnOldOfferCannotAffectANewerOne(t *testing.T) {
	// Membership drift replaces a pending question with a fresh one at the same
	// iteration of the same command. An answer written for the old contender list must
	// not be applied to the new one.
	//
	//	event A: 1 New tab     2 New window
	//	event B: 1 New folder  2 New tab
	//
	// "The first one" meant New tab. Applied to B it would select New folder.
	rt := newFakeRuntime()
	s, dir := serve(t, rt)
	c := dial(t, dir)

	// Event B is what is pending now.
	s.pending.Set(ClarificationPayload{
		CommandID: "cmd_1", Phrase: "focus each item in tabs",
		Question: "which New did you mean?", EventID: "clarification_B",
		CollectionName: "tabs", Iteration: 3, CompletedItems: 2,
		Candidates: []ClarificationCandidate{
			{Index: 1, Label: "New folder"}, {Index: 2, Label: "New tab"},
		},
	})

	// The answer carries event A's id.
	resp, err := c.roundTrip(RequestClarify, ClarifyPayload{
		CommandID: "cmd_1", Response: "the first one", EventID: "clarification_A",
	})
	if err != nil {
		t.Fatalf("clarify: %v", err)
	}
	if resp.Type != ResponseError {
		t.Fatalf("a stale answer was accepted: %s", resp.Type)
	}
	var errPayload ErrorPayload
	if err := resp.Decode(&errPayload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if errPayload.Message != "That clarification is no longer active." {
		t.Fatalf("message = %q", errPayload.Message)
	}

	// Event B is STILL pending: a rejected answer must not consume the question.
	still, ok := s.pending.Get()
	if !ok {
		t.Fatal("the pending question was cleared by a rejected answer")
	}
	if still.EventID != "clarification_B" {
		t.Fatalf("pending event = %q, want B", still.EventID)
	}
	// And nothing ran.
	if len(rt.phrases()) != 0 {
		t.Fatalf("%d commands ran under a stale answer", len(rt.phrases()))
	}
}

func TestAnAnswerForTheCurrentOfferResumesNormally(t *testing.T) {
	rt := newFakeRuntime()
	s, dir := serve(t, rt)
	c := dial(t, dir)

	s.pending.Set(ClarificationPayload{
		CommandID: "cmd_1", Phrase: "focus each item in tabs",
		Question: "which New did you mean?", EventID: "clarification_B",
		Candidates: []ClarificationCandidate{
			{Index: 1, Label: "New folder"}, {Index: 2, Label: "New tab"},
		},
	})

	resp, err := c.roundTrip(RequestClarify, ClarifyPayload{
		CommandID: "cmd_1", Response: "the second one", EventID: "clarification_B",
	})
	if err != nil {
		t.Fatalf("clarify: %v", err)
	}
	if resp.Type == ResponseError {
		var e ErrorPayload
		_ = resp.Decode(&e)
		t.Fatalf("a valid answer was rejected: %s", e.Message)
	}
}

func TestAnAnswerWithNoPendingQuestionIsNotADesktopCommand(t *testing.T) {
	// "The first one" searched for as a control label would either find nothing or,
	// far worse, find something.
	rt := newFakeRuntime()
	_, dir := serve(t, rt)
	c := dial(t, dir)

	resp, err := c.roundTrip(RequestClarify, ClarifyPayload{
		CommandID: "cmd_1", Response: "the first one", EventID: "clarification_A",
	})
	if err != nil {
		t.Fatalf("clarify: %v", err)
	}
	if resp.Type != ResponseError {
		t.Fatalf("an orphaned answer was executed: %s", resp.Type)
	}
	if len(rt.phrases()) != 0 {
		t.Fatal("an orphaned answer reached the desktop")
	}
}

func TestAnOlderClientWithoutAnEventIDStillWorks(t *testing.T) {
	// Compatibility: the check rejects a MISMATCH, never an absence.
	rt := newFakeRuntime()
	s, dir := serve(t, rt)
	c := dial(t, dir)

	s.pending.Set(ClarificationPayload{
		CommandID: "cmd_1", Phrase: "click Save", Question: "which Save?",
		EventID: "clarification_B",
		Candidates: []ClarificationCandidate{
			{Index: 1, Label: "Save"}, {Index: 2, Label: "Save As"},
		},
	})
	resp, err := c.roundTrip(RequestClarify, ClarifyPayload{
		CommandID: "cmd_1", Response: "the first one",
	})
	if err != nil {
		t.Fatalf("clarify: %v", err)
	}
	if resp.Type == ResponseError {
		var e ErrorPayload
		_ = resp.Decode(&e)
		t.Fatalf("an id-less answer was rejected: %s", e.Message)
	}
}
