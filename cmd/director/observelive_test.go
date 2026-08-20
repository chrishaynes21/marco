package main

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Which session's findings are current.
//
// These drive the REAL registry rather than a fake response model. The defect this milestone
// fixes was invisible to a response-model test: the protocol distinguished Available from
// Active perfectly, and the producer simply never emitted the combination — so a completed
// session's evidence became unreachable the moment it finished.

// finishedResult builds a terminal session result as the runner would file it.
func finishedResult(id observe.SessionID, state observe.State, reason string,
	samples int, started time.Time) observesession.Result {

	ended := started.Add(time.Duration(samples) * 500 * time.Millisecond)
	return observesession.Result{
		Session: observe.Session{
			ID: id, State: state, Reason: reason,
			StartedAt: started, EndedAt: &ended, Application: "testapp",
		},
		Stats: observesession.Stats{SamplesTaken: samples},
		Findings: observe.Findings{
			Samples: samples,
			Stable: []observe.Stability{{
				Identity: "d1", Role: "button", SamplesSeen: samples,
				SamplesTotal: samples, PresenceRatio: 1,
				Label: observe.SafeLabel{Text: "Save"},
			}},
		},
		Insights: []observe.Insight{{
			Kind: observe.PossibleMenu, Confidence: 0.7,
			SupportingEntities: []observe.Digest{"d1"},
			Validation:         "open and close the panel deliberately and re-run",
		}},
	}
}

// registryWith returns a runtime whose registry holds the given finished sessions.
func registryWith(t *testing.T, results ...observesession.Result) *Runtime {
	t.Helper()
	rt := testRuntime(t)
	rt.observations = newObservationRegistry()
	for _, res := range results {
		rt.observations.finish(res)
	}
	return rt
}

// Nothing has ever run. Not the same as a session that found nothing.
func TestLiveAnalysisUnavailableWhenNoSessionHasRun(t *testing.T) {
	rt := registryWith(t)
	got := rt.LiveAnalysis(service.LiveAnalysisPayload{})

	if got.Available || got.Active {
		t.Fatalf("want unavailable and inactive, got %+v", got)
	}
	if got.Summary.SessionID != "" {
		t.Error("an unavailable analysis named a session")
	}
}

// The defect. A completed session's evidence must survive its completion.
func TestCompletedSessionRemainsAvailable(t *testing.T) {
	start := time.Now().Add(-3 * time.Minute)
	rt := registryWith(t, finishedResult("observe_1", observe.Completed, "", 52, start))

	got := rt.LiveAnalysis(service.LiveAnalysisPayload{})
	if !got.Available {
		t.Fatal("a completed session became unreachable — completion must freeze " +
			"evidence, not erase it")
	}
	if got.Active {
		t.Error("a finished session reported itself active")
	}
	s := got.Summary
	if !s.Complete || !s.Terminal {
		t.Errorf("completion flags wrong: complete=%v terminal=%v", s.Complete, s.Terminal)
	}
	if s.SessionID != "observe_1" || s.Samples != 52 {
		t.Errorf("session identity lost: %+v", s)
	}
	if s.EndedAt == nil {
		t.Error("a terminal session carries no end time")
	}
	if len(s.Stable) != 1 || len(s.Hypotheses) != 1 {
		t.Errorf("the final findings did not survive: %d stable, %d hypotheses",
			len(s.Stable), len(s.Hypotheses))
	}
}

// Cancelled and failed sessions stopped, but they did not complete. Calling partial
// evidence a completed report licenses conclusions the sample size does not support.
func TestTerminalButNotCompleteSessionsAreNotCalledComplete(t *testing.T) {
	cases := []struct {
		state  observe.State
		reason string
	}{
		{observe.Cancelled, "stopped on request"},
		{observe.Failed, "the sampler produced nothing ten times in a row"},
		{observe.TargetUnavailable, "the window went away and could not be reacquired"},
	}
	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			rt := registryWith(t,
				finishedResult("observe_x", tc.state, tc.reason, 31, time.Now()))

			got := rt.LiveAnalysis(service.LiveAnalysisPayload{})
			if !got.Available {
				t.Fatal("a terminal session's evidence was discarded")
			}
			s := got.Summary
			if s.Complete {
				t.Errorf("%s reported itself complete", tc.state)
			}
			if !s.Terminal {
				t.Errorf("%s did not report itself terminal", tc.state)
			}
			if s.TerminalReason != tc.reason {
				t.Errorf("terminal reason = %q, want %q", s.TerminalReason, tc.reason)
			}
			if s.State != tc.state {
				t.Errorf("state = %q, want %q", s.State, tc.state)
			}
		})
	}
}

// The newest retained session wins, so a HUD shows the session just finished rather than
// one from ten minutes ago.
func TestNewestTerminalSessionIsSelected(t *testing.T) {
	base := time.Now().Add(-10 * time.Minute)
	rt := registryWith(t,
		finishedResult("observe_1", observe.Completed, "", 20, base),
		finishedResult("observe_2", observe.Completed, "", 40, base.Add(5*time.Minute)),
		finishedResult("observe_3", observe.Cancelled, "stopped on request", 9,
			base.Add(8*time.Minute)),
	)

	got := rt.LiveAnalysis(service.LiveAnalysisPayload{})
	if got.Summary.SessionID != "observe_3" {
		t.Errorf("session = %q, want the newest (observe_3)", got.Summary.SessionID)
	}
	if got.Summary.Complete {
		t.Error("the newest session was cancelled and must not read as complete")
	}
}

// Naming an older session by id must reach it, so a client holding an id does not silently
// get a different session's findings.
func TestAnOlderSessionCanBeAddressedByID(t *testing.T) {
	base := time.Now().Add(-10 * time.Minute)
	rt := registryWith(t,
		finishedResult("observe_1", observe.Completed, "", 20, base),
		finishedResult("observe_2", observe.Completed, "", 40, base.Add(5*time.Minute)),
	)

	got := rt.LiveAnalysis(service.LiveAnalysisPayload{SessionID: "observe_1"})
	if got.Summary.SessionID != "observe_1" {
		t.Errorf("session = %q, want observe_1", got.Summary.SessionID)
	}
	if got.Summary.Samples != 20 {
		t.Errorf("samples = %d, want the addressed session's 20", got.Summary.Samples)
	}
}

// An unknown id must not silently fall through to whatever is newest.
func TestAnUnknownSessionIDIsUnavailableRatherThanSubstituted(t *testing.T) {
	rt := registryWith(t,
		finishedResult("observe_1", observe.Completed, "", 20, time.Now()))

	got := rt.LiveAnalysis(service.LiveAnalysisPayload{SessionID: "observe_999"})
	if got.Available {
		t.Errorf("an unknown id returned session %q instead of nothing",
			got.Summary.SessionID)
	}
}

// A service with only in-memory sessions has nothing to report after a restart, and must
// say so rather than implying a session is still around.
func TestAFreshRuntimeReportsUnavailableHonestly(t *testing.T) {
	rt := testRuntime(t)
	rt.observations = newObservationRegistry()

	got := rt.LiveAnalysis(service.LiveAnalysisPayload{})
	if got.Available {
		t.Error("a fresh runtime claimed an analysis it does not have")
	}
	if got.ServiceGeneration == "" {
		t.Error("no service generation: a client cannot detect the restart")
	}
}

// A finished session has no live event recorder, and reporting a stale cursor would invite
// a client to poll a feed that no longer exists.
func TestCompletedSessionCarriesNoResumableCursor(t *testing.T) {
	rt := registryWith(t,
		finishedResult("observe_1", observe.Completed, "", 52, time.Now()))

	got := rt.LiveAnalysis(service.LiveAnalysisPayload{})
	if got.Summary.Cursor != 0 {
		t.Errorf("cursor = %d; a retired session has no feed to resume",
			got.Summary.Cursor)
	}

	events := rt.ObservationEvents(service.ObservationEventsPayload{})
	if events.Available {
		t.Error("events reported available for a session whose recorder has retired")
	}
}

// Privacy classification happened upstream; the summary must carry the classifier's verdict
// rather than re-deciding it.
func TestFinalSummaryPreservesLabelClassification(t *testing.T) {
	res := finishedResult("observe_1", observe.Completed, "", 20, time.Now())
	res.Findings.Stable = append(res.Findings.Stable, observe.Stability{
		Identity: "d2", Role: "icon", SamplesSeen: 20, SamplesTotal: 20, PresenceRatio: 1,
		Label: observe.SafeLabel{Digest: "ff5d43925d81", Length: 12, Redacted: true},
	})
	rt := registryWith(t, res)

	got := rt.LiveAnalysis(service.LiveAnalysisPayload{})
	var withheld, readable int
	for _, e := range got.Summary.Stable {
		if e.Label.Redacted {
			withheld++
			if e.Label.Text != "" {
				t.Errorf("a withheld label carried text: %+v", e.Label)
			}
			continue
		}
		readable++
	}
	if withheld != 1 || readable != 1 {
		t.Errorf("want one withheld and one readable label, got %d and %d",
			withheld, readable)
	}
}

// ── target pinning ────────────────────────────────────────────────────────────

// Every scopeable provider in an observation cycle must be pointed at the window the
// runner validated — not at whatever happens to be in front.
//
// This defect survived three milestones because the intent was written in a comment
// beside the code that did not honour it. It is an assertion now.
func TestTheSamplerPinsAccessibilityToTheValidatedWindow(t *testing.T) {
	s := &liveSampler{labels: true}
	ref := windowref.Ref{
		ID: "hwnd:661516", Handle: 661516, ProcessID: 4242,
		Application: "code", Generation: 7,
	}

	for _, tc := range []struct {
		name       string
		readLabels bool
	}{
		{"vision only", false},
		{"vision and labels", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := s.request(observesession.SampleRequest{
				Window: ref, Sequence: 1, ReadLabels: tc.readLabels,
			})
			if got.Window == nil {
				t.Fatal("the request names no window, so accessibility falls back to " +
					"the FOREGROUND window and the session observes the wrong application")
			}
			if *got.Window != ref.ID {
				t.Errorf("window = %q, want the validated %q", *got.Window, ref.ID)
			}
			// A region is a different narrowing and is correctly absent: a tree walk
			// has no notion of a rectangle.
			if got.Region != nil {
				t.Errorf("a region was set on a tree-walking cycle: %+v", got.Region)
			}
		})
	}
}

// The window must not quietly override which providers run.
func TestPinningDoesNotChangeWhichProvidersRun(t *testing.T) {
	ref := windowref.Ref{ID: "hwnd:1", Application: "code", Generation: 1}

	cheap := (&liveSampler{labels: true}).request(
		observesession.SampleRequest{Window: ref, ReadLabels: false})
	if cheap.Includes(directorapi.SourceOCR) {
		t.Error("a non-label sample included OCR, which is the expensive pass")
	}
	if !cheap.Includes(directorapi.SourceVision) {
		t.Error("vision should run on every sample")
	}

	full := (&liveSampler{labels: true}).request(
		observesession.SampleRequest{Window: ref, ReadLabels: true})
	if !full.Includes(directorapi.SourceOCR) || !full.Includes(directorapi.SourceVision) {
		t.Error("a label sample should include both pixel passes")
	}

	// Accessibility is never in Include: it is a default source and must stay in the
	// cycle. A request that had to opt into it would silently drop structure.
	for _, r := range []observation.Request{cheap, full} {
		if !r.Wants(directorapi.SourceAccessibility) {
			t.Error("accessibility fell out of the cycle")
		}
	}
}

// The generation must be pinned too, and for the same reason the window is.
//
// Window and Target answer different questions. Window says WHERE to look and is satisfied
// by any window bearing that id; Target says which live generation the answer is allowed to
// describe, and is not satisfied by a replacement. Setting only the first is what leaves the
// in-flight replacement race undetectable — and an unset field on this exact function is
// what cost three milestones the last time.
func TestTheSamplerPinsTheGenerationAndNotOnlyTheWindow(t *testing.T) {
	ref := windowref.Ref{
		ID: "hwnd:661516", Handle: 661516, ProcessID: 4242,
		Application: "code", Generation: 7,
	}
	got := (&liveSampler{labels: true}).request(
		observesession.SampleRequest{Window: ref, Sequence: 1})

	if got.Target == nil {
		t.Fatal("the request pins no target generation, so every provider's evidence is " +
			"unguarded: a window replaced mid-cycle would be fused without complaint")
	}
	if got.Target.WindowGeneration != ref.Generation {
		t.Errorf("generation = %d, want the validated %d",
			got.Target.WindowGeneration, ref.Generation)
	}
	if got.Target.Application != ref.Application || got.Target.ProcessID != ref.ProcessID {
		t.Errorf("target = %+v, want it built from the validated reference", got.Target)
	}
	// The expectation must be recognisable AS an expectation. A target that is not Known
	// makes the guard skip the cycle entirely, which fails open.
	if !got.Target.Known() {
		t.Error("the pinned target is not Known, so the guard would treat this as an " +
			"untargeted cycle and admit everything")
	}
}

// A Director that cannot read text must still pin the window.
func TestPinningAppliesEvenWithoutLabelReading(t *testing.T) {
	got := (&liveSampler{labels: false}).request(observesession.SampleRequest{
		Window:     windowref.Ref{ID: "hwnd:99", Application: "code"},
		ReadLabels: true, // asked for, but this Director has no OCR
	})
	if got.Window == nil || *got.Window != "hwnd:99" {
		t.Fatalf("window pinning was lost when label reading was unavailable: %+v", got.Window)
	}
	if got.Includes(directorapi.SourceOCR) {
		t.Error("OCR was requested from a Director that cannot read text")
	}
}
