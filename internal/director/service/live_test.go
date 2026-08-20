package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The live-findings carrier, end to end through a real socket.
//
// Producer-only tests would not catch what actually breaks here: a field that marshals but
// does not unmarshal, an omitempty that erases a meaningful false, or a privacy guarantee
// that holds in the analyser and not on the wire.

// everyLiveKind builds one event of each of the seven kinds, populated.
func everyLiveKind() []observe.LiveEvent {
	base := []struct {
		kind observe.LiveKind
		fill func(*observe.LiveEvent)
	}{
		{observe.EntityBecameStable, func(e *observe.LiveEvent) {
			e.Entity, e.Role, e.Labelled = "d1", "button", true
			e.PresenceRatio, e.SamplesSeen = 0.9, 18
		}},
		{observe.EntityBecameUnstable, func(e *observe.LiveEvent) { e.Entity = "d2" }},
		{observe.TransitionDetected, func(e *observe.LiveEvent) {
			e.Transition, e.TransitionKind = "t1", observe.MenuLikeAppeared
			e.Observed = "a menu-like structure appeared"
		}},
		{observe.HypothesisCreated, func(e *observe.LiveEvent) {
			e.Hypothesis, e.Concept, e.State = "h1", observe.PossibleMenu, observe.StateActive
			e.Confidence, e.ConfidenceBand = 0.78, 7
			e.SupportingEntities = []observe.Digest{"d1", "d2"}
			e.SupportingTransitions = []observe.TransitionID{"t1"}
			e.Contradictions = []string{"only observed once"}
			e.RecommendedValidation = "open and close the same menu again"
			e.SupportingSamples = 20
		}},
		{observe.HypothesisUpdated, func(e *observe.LiveEvent) {
			e.Hypothesis, e.Concept, e.State = "h1", observe.PossibleMenu, observe.StateActive
			e.ConfidenceBand = 8
			e.Contradictions = []string{"the labels were not all present in every sample"}
		}},
		{observe.HypothesisWithdrawn, func(e *observe.LiveEvent) {
			e.Hypothesis, e.Concept, e.State = "h1", observe.PossibleMenu, observe.StateWithdrawn
		}},
		{observe.ValidationRecommended, func(e *observe.LiveEvent) {
			e.Hypothesis, e.Concept = "h1", observe.PossibleMenu
			e.RecommendedValidation = "open and close the same menu again"
		}},
	}
	out := make([]observe.LiveEvent, 0, len(base))
	for i, b := range base {
		e := observe.LiveEvent{
			Kind: b.kind, SessionID: "observe_7",
			Sequence: uint64(i + 1), At: time.Unix(int64(i), 0),
			Sample: i + 1,
		}
		b.fill(&e)
		out = append(out, e)
	}
	return out
}

// Every kind must survive marshal, socket, unmarshal — with its evidence intact.
func TestEveryLiveEventKindSurvivesTheRoundTrip(t *testing.T) {
	rt := newFakeRuntime()
	sent := everyLiveKind()
	rt.obsEvents = ObservationEventsResponse{
		Available: true, Active: true, SessionID: "observe_7",
		ServiceGeneration: "svc-1", Events: sent, Newest: 7, Oldest: 1,
	}
	_, dir := serve(t, rt)

	got, err := dial(t, dir).ObservationEvents(ObservationEventsPayload{})
	if err != nil {
		t.Fatalf("ObservationEvents: %v", err)
	}
	if len(got.Events) != len(sent) {
		t.Fatalf("want %d events, got %d", len(sent), len(got.Events))
	}
	if got.SessionID != "observe_7" || got.ServiceGeneration != "svc-1" {
		t.Errorf("generations lost: %+v", got)
	}

	for i, want := range sent {
		g := got.Events[i]
		if g.Kind != want.Kind {
			t.Errorf("[%d] kind = %q, want %q (ordering or identity lost)", i, g.Kind, want.Kind)
		}
		if g.Sequence != want.Sequence {
			t.Errorf("[%d] sequence = %d, want %d", i, g.Sequence, want.Sequence)
		}
		if g.SessionID != want.SessionID {
			t.Errorf("[%d] session id lost", i)
		}
		if g.State != want.State {
			t.Errorf("[%d] hypothesis state = %q, want %q", i, g.State, want.State)
		}
		if len(g.Contradictions) != len(want.Contradictions) {
			t.Errorf("[%d] contradictions dropped: %v", i, g.Contradictions)
		}
		if g.RecommendedValidation != want.RecommendedValidation {
			t.Errorf("[%d] validation dropped", i)
		}
		if len(g.SupportingEntities) != len(want.SupportingEntities) {
			t.Errorf("[%d] supporting entities dropped", i)
		}
	}
}

// An unknown kind must arrive intact rather than being silently dropped or coerced — a
// client on an older build has to be able to say "I do not render this" rather than
// pretending it never happened.
func TestUnknownEventKindSurvivesAndIsIdentifiable(t *testing.T) {
	rt := newFakeRuntime()
	rt.obsEvents = ObservationEventsResponse{
		Available: true, Newest: 1, Oldest: 1,
		Events: []observe.LiveEvent{{
			Kind: observe.LiveKind("observation_something_new"), Sequence: 1,
		}},
	}
	_, dir := serve(t, rt)

	got, err := dial(t, dir).ObservationEvents(ObservationEventsPayload{})
	if err != nil {
		t.Fatalf("ObservationEvents: %v", err)
	}
	if len(got.Events) != 1 {
		t.Fatalf("an unknown kind was dropped in transit")
	}
	if got.Events[0].Kind.Known() {
		t.Error("an unrecognised kind reported itself known")
	}
}

func TestLiveAnalysisSummarySurvivesTheRoundTrip(t *testing.T) {
	rt := newFakeRuntime()
	rt.liveAnalysis = LiveAnalysisResponse{
		Available: true, Active: true, ServiceGeneration: "svc-1",
		Summary: observe.LiveSummary{
			Active: true, SessionID: "observe_7", Samples: 42, FreshnessMS: 800,
			StableTotal: 3,
			Stable: []observe.StableEntity{{
				Identity: "d1", Role: "button",
				Label:         observe.SafeLabel{Text: "Save"},
				PresenceRatio: 0.9, SamplesSeen: 18, SamplesTotal: 20,
				ConfidenceBand: 8, LastSeenMS: 1200,
			}},
			Unreliable: observe.UnreliableSummary{
				Entities: 131, Anonymous: 120, Withheld: 9, Flickering: 14,
			},
			Hypotheses: []observe.LiveHypothesis{{
				ID: "h1", Kind: observe.PossibleMenu, State: observe.StateActive,
				Confidence: 0.78, ConfidenceBand: 7, SupportingSamples: 20,
				Contradictions:        []string{"only observed once"},
				RecommendedValidation: "open and close the same menu again",
				Observed:              "4 readable labels shared the upper-left across the session",
			}},
			HypothesesTotal: 1, WithdrawnHypotheses: 2, Cursor: 19,
		},
	}
	_, dir := serve(t, rt)

	got, err := dial(t, dir).LiveAnalysis(LiveAnalysisPayload{})
	if err != nil {
		t.Fatalf("LiveAnalysis: %v", err)
	}
	s := got.Summary
	if !got.Available || !got.Active || s.Samples != 42 || s.Cursor != 19 {
		t.Fatalf("envelope did not survive: %+v", got)
	}
	if len(s.Stable) != 1 || s.Stable[0].Label.Text != "Save" {
		t.Errorf("stable evidence did not survive: %+v", s.Stable)
	}
	if s.Unreliable.Entities != 131 || s.Unreliable.Withheld != 9 {
		t.Errorf("unreliable aggregate did not survive: %+v", s.Unreliable)
	}
	if len(s.Hypotheses) != 1 {
		t.Fatalf("hypotheses dropped")
	}
	h := s.Hypotheses[0]
	if h.State != observe.StateActive || h.ConfidenceBand != 7 {
		t.Errorf("hypothesis state or band lost: %+v", h)
	}
	if len(h.Contradictions) != 1 || h.RecommendedValidation == "" {
		t.Errorf("uncertainty stripped in transit: %+v", h)
	}
	if s.WithdrawnHypotheses != 2 {
		t.Errorf("withdrawn count lost — active hypotheses alone read as though nothing was wrong")
	}
}

// "No session has run" must not arrive looking like "a session found nothing".
func TestUnavailableLiveAnalysisIsDistinctFromEmptyFindings(t *testing.T) {
	rt := newFakeRuntime()
	rt.liveAnalysis = LiveAnalysisResponse{Available: false}
	_, dir := serve(t, rt)

	got, err := dial(t, dir).LiveAnalysis(LiveAnalysisPayload{})
	if err != nil {
		t.Fatalf("LiveAnalysis: %v", err)
	}
	if got.Available {
		t.Error("an unavailable analysis reported itself available")
	}
}

// A completed session is still worth showing, and must not be presented as live.
func TestCompletedSessionIsAvailableButNotActive(t *testing.T) {
	rt := newFakeRuntime()
	rt.liveAnalysis = LiveAnalysisResponse{
		Available: true, Active: false,
		Summary: observe.LiveSummary{SessionID: "observe_1", Samples: 60, Complete: true},
	}
	_, dir := serve(t, rt)

	got, _ := dial(t, dir).LiveAnalysis(LiveAnalysisPayload{})
	if !got.Available || got.Active {
		t.Fatalf("want available-but-not-active, got %+v", got)
	}
	if !got.Summary.Complete {
		t.Error("completeness lost — only a complete session licenses reading conclusions")
	}
}

func TestObservationCursorReturnsOnlyNewerEvents(t *testing.T) {
	rt := newFakeRuntime()
	rt.obsEvents = ObservationEventsResponse{
		Available: true, Newest: 3, Oldest: 1,
		Events: []observe.LiveEvent{
			{Kind: observe.EntityBecameStable, Sequence: 1},
			{Kind: observe.EntityBecameStable, Sequence: 2},
			{Kind: observe.EntityBecameStable, Sequence: 3},
		},
	}
	_, dir := serve(t, rt)

	got, err := dial(t, dir).ObservationEvents(ObservationEventsPayload{After: 2})
	if err != nil {
		t.Fatalf("ObservationEvents: %v", err)
	}
	if len(got.Events) != 1 || got.Events[0].Sequence != 3 {
		t.Fatalf("cursor not honoured: %+v", got.Events)
	}
}

// Gap is decided by the SERVER, so two clients cannot disagree about whether anything was
// missed.
func TestGapIsReportedByTheServer(t *testing.T) {
	rt := newFakeRuntime()
	rt.obsEvents = ObservationEventsResponse{Available: true, Newest: 40, Oldest: 12, Gap: true}
	_, dir := serve(t, rt)

	got, _ := dial(t, dir).ObservationEvents(ObservationEventsPayload{After: 2})
	if !got.Gap {
		t.Error("a retention gap was not reported")
	}
}

// Privacy, on the wire. The markers stand for the categories that must never appear.
func TestNoLivePayloadCarriesPrivateContent(t *testing.T) {
	markers := []string{
		"Chris Haynes", "blakeus", "PartyMember7", "hey what's up",
		"eu-west-3.server", "ghp_secretToken", "you have 3 notifications", "hunter2",
	}

	rt := newFakeRuntime()
	rt.liveAnalysis = LiveAnalysisResponse{
		Available: true, Active: true,
		Summary: observe.LiveSummary{
			SessionID: "observe_7",
			Stable: []observe.StableEntity{
				// A structural label the classifier released.
				{Identity: "d1", Role: "button", Label: observe.SafeLabel{Text: "Save"}},
				// One it withheld: digest and length only.
				{Identity: "d2", Role: "icon",
					Label: observe.SafeLabel{Digest: "ff5d43925d81", Length: 12, Redacted: true}},
			},
			Unreliable: observe.UnreliableSummary{Entities: 131, Anonymous: 120},
		},
	}
	rt.obsEvents = ObservationEventsResponse{Available: true, Events: everyLiveKind(), Newest: 7}
	_, dir := serve(t, rt)
	c := dial(t, dir)

	analysis, _ := c.LiveAnalysis(LiveAnalysisPayload{})
	events, _ := c.ObservationEvents(ObservationEventsPayload{})

	for _, payload := range []any{analysis, events} {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range markers {
			if strings.Contains(string(raw), m) {
				t.Errorf("a live payload leaked %q:\n%s", m, raw)
			}
		}
	}

	// And the safe structural label must still be usable, or the feed says nothing.
	if analysis.Summary.Stable[0].Label.Text != "Save" {
		t.Error("a safe structural label was lost; privacy must not cost every name")
	}
	if !analysis.Summary.Stable[1].Label.Redacted {
		t.Error("a withheld label arrived unredacted")
	}
}
