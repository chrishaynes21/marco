package observesession_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// Live findings must be readable WHILE a session runs, not only in its Result.
//
// The behaviour these protect is the whole milestone: a person watching a HUD should see
// evidence accumulate, and a client that arrives late or loses events should be able to
// rebuild what it ought to be showing without inferring anything.

// watchingSampler reports a steady scene and lets a test observe mid-session.
//
// It signals after a chosen sample so the test can poll the runner's live surfaces at a
// deterministic point rather than sleeping and hoping.
type watchingSampler struct {
	mu      sync.Mutex
	calls   int
	at      int
	reached chan struct{}
	once    sync.Once
}

func (s *watchingSampler) Sample(_ context.Context,
	req observesession.SampleRequest) (observe.Sample, error) {

	s.mu.Lock()
	s.calls++
	n := s.calls
	s.mu.Unlock()

	if n >= s.at {
		s.once.Do(func() { close(s.reached) })
	}
	return observe.Sample{
		WindowGeneration: req.Window.Generation,
		Entities: []observe.EntitySnapshot{
			{Identity: "hud", Role: "icon", Confidence: 0.9,
				Region: observe.Region{X: 0.8, Y: 0.8, Width: 0.1, Height: 0.1}},
			{Identity: "panel", Role: "panel", Confidence: 0.9,
				Region: observe.Region{X: 0.1, Y: 0.1, Width: 0.4, Height: 0.4}},
		},
	}, nil
}

func liveRef() windowref.Ref {
	return windowref.Ref{ID: "hwnd:100", Handle: 100, ProcessID: 7,
		Application: "testgame", Generation: 1}
}

func liveConfig() observesession.Config {
	b := observe.DefaultBounds()
	b.Duration = 10 * time.Second
	b.Interval = observe.MinInterval
	return observesession.Config{
		ID: "observe_live", Selector: windowref.Selector{Application: "testgame"}, Bounds: b,
	}
}

// The core claim: findings are visible before the session ends.
func TestLiveEventsAreAvailableWhileTheSessionRuns(t *testing.T) {
	clock := newClock()
	sampler := &watchingSampler{at: 12, reached: make(chan struct{})}
	r := observesession.New(clock, &steadyTarget{ref: liveRef()}, sampler,
		observesession.NopEvents{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan observesession.Result, 1)
	go func() {
		res, _ := r.Run(ctx, liveConfig())
		done <- res
	}()

	select {
	case <-sampler.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("the sampler never reached the observation point")
	}

	// Poll the live surfaces MID-SESSION. Nothing here waits on the session finishing.
	events, newest, _ := r.LiveEvents(0, 0)
	if len(events) == 0 || newest == 0 {
		t.Fatalf("no live events mid-session (newest=%d) — findings are still end-only",
			newest)
	}
	analysis := r.LiveAnalysis(observe.DefaultInsightThresholds())
	if analysis.Samples == 0 {
		t.Error("the live analysis reported no samples mid-session")
	}
	if analysis.SessionID != "observe_live" {
		t.Errorf("session id = %q, want observe_live", analysis.SessionID)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the session did not stop after cancellation")
	}
}

// A cursor must let a client resume without replaying what it already rendered.
func TestLiveCursorResumesWithoutReplay(t *testing.T) {
	clock := newClock()
	sampler := &watchingSampler{at: 12, reached: make(chan struct{})}
	r := observesession.New(clock, &steadyTarget{ref: liveRef()}, sampler,
		observesession.NopEvents{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { _, _ = r.Run(ctx, liveConfig()); close(done) }()

	select {
	case <-sampler.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("sampler never reached the observation point")
	}

	_, newest, _ := r.LiveEvents(0, 0)
	again, _, _ := r.LiveEvents(newest, 0)
	for _, e := range again {
		if e.Sequence <= newest {
			t.Fatalf("event %d replayed below the cursor %d", e.Sequence, newest)
		}
	}

	cancel()
	<-done
}

// The rebuild path: a client with no history gets the current analysis, plus the cursor to
// resume from — so it never has to infer the events it missed.
func TestLiveAnalysisCarriesTheCursorToResumeFrom(t *testing.T) {
	clock := newClock()
	sampler := &watchingSampler{at: 12, reached: make(chan struct{})}
	r := observesession.New(clock, &steadyTarget{ref: liveRef()}, sampler,
		observesession.NopEvents{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { _, _ = r.Run(ctx, liveConfig()); close(done) }()

	select {
	case <-sampler.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("sampler never reached the observation point")
	}

	analysis := r.LiveAnalysis(observe.DefaultInsightThresholds())
	if analysis.Cursor == 0 {
		t.Fatal("the analysis snapshot carries no cursor; a client cannot resume from it")
	}
	// Resuming from the snapshot's cursor must not replay anything it already contained.
	after, _, _ := r.LiveEvents(analysis.Cursor, 0)
	for _, e := range after {
		if e.Sequence <= analysis.Cursor {
			t.Fatalf("event %d replayed below the snapshot cursor %d",
				e.Sequence, analysis.Cursor)
		}
	}

	cancel()
	<-done
}

// Polling the live feed must not wait on a sample. This blocks INSIDE the sampler and
// proves the read surfaces still answer — no shared lock is held across the slow work.
func TestLiveSurfacesAnswerWhileASampleIsBlocked(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{})
	var once sync.Once

	blocking := samplerFunc(func(_ context.Context,
		req observesession.SampleRequest) (observe.Sample, error) {

		once.Do(func() { close(entered) })
		<-release
		return observe.Sample{
			WindowGeneration: req.Window.Generation,
			Entities: []observe.EntitySnapshot{{Identity: "x", Role: "icon",
				Confidence: 0.9, Region: observe.Region{X: 0.1, Y: 0.1, Width: 0.1, Height: 0.1}}},
		}, nil
	})

	r := observesession.New(newClock(), &steadyTarget{ref: liveRef()}, blocking,
		observesession.NopEvents{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { _, _ = r.Run(ctx, liveConfig()); close(done) }()

	<-entered // a sample is in flight and will not return until released

	answered := make(chan struct{})
	go func() {
		r.Snapshot()
		r.LiveEvents(0, 0)
		r.LiveAnalysis(observe.DefaultInsightThresholds())
		close(answered)
	}()

	select {
	case <-answered:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("the live surfaces blocked behind an in-flight sample — a lock is held " +
			"across the slow work")
	}

	close(release)
	cancel()
	<-done
}
