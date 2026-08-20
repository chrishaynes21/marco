package observesession_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// Scheduling, bounds and target loss, on a fake clock.
//
// Every question worth asking here is about time — what happens when a cycle overruns its
// interval, how long a lost window is waited for, whether cancellation stops the next sample
// — and answering them against the wall clock would make the suite slow and flaky while
// proving less. The clock advances only when the runner asks it to.

// fakeClock runs virtual time. Waits complete instantly and the clock jumps forward.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.advance(d)
	ch := make(chan time.Time, 1)
	ch <- c.Now()
	return ch
}

// steadyTarget always resolves.
type steadyTarget struct {
	ref   windowref.Ref
	calls int
	mu    sync.Mutex
}

func (t *steadyTarget) Acquire(context.Context, windowref.Selector) (windowref.Ref, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	return t.ref, nil
}

// scriptedTarget fails for a stretch, then recovers.
type scriptedTarget struct {
	mu        sync.Mutex
	calls     int
	failFrom  int
	failUntil int
	ref       windowref.Ref
	after     windowref.Ref
}

func (t *scriptedTarget) Acquire(context.Context, windowref.Selector) (windowref.Ref, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	if t.calls >= t.failFrom && t.calls <= t.failUntil {
		return windowref.Ref{}, errors.New("no window belonging to testgame is currently available")
	}
	if t.calls > t.failUntil && t.after.Handle != 0 {
		return t.after, nil
	}
	return t.ref, nil
}

// countingSampler produces a trivial sample and optionally consumes clock time.
type countingSampler struct {
	mu         sync.Mutex
	calls      int
	labelCalls int
	cost       time.Duration
	clock      *fakeClock
	failEvery  int
	failAll    bool
}

func (s *countingSampler) Sample(_ context.Context, req observesession.SampleRequest) (observe.Sample, error) {
	s.mu.Lock()
	s.calls++
	n := s.calls
	if req.ReadLabels {
		s.labelCalls++
	}
	s.mu.Unlock()

	if s.cost > 0 && s.clock != nil {
		s.clock.advance(s.cost)
	}
	if s.failAll || (s.failEvery > 0 && n%s.failEvery == 0) {
		return observe.Sample{}, errors.New("capture produced no frame")
	}
	return observe.Sample{
		WindowGeneration: req.Window.Generation,
		Entities: []observe.EntitySnapshot{{
			Identity: "thing", Role: "icon", Confidence: 0.9,
			Region: observe.Region{X: 0.8, Y: 0.8, Width: 0.1, Height: 0.1},
		}},
	}, nil
}

// recordingEvents keeps what was published.
type recordingEvents struct {
	mu     sync.Mutex
	events []observesession.Event
}

func (e *recordingEvents) Publish(ev observesession.Event) {
	e.mu.Lock()
	e.events = append(e.events, ev)
	e.mu.Unlock()
}

func (e *recordingEvents) kinds() []observesession.EventKind {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]observesession.EventKind, 0, len(e.events))
	for _, ev := range e.events {
		out = append(out, ev.Kind)
	}
	return out
}

func (e *recordingEvents) has(k observesession.EventKind) bool {
	for _, got := range e.kinds() {
		if got == k {
			return true
		}
	}
	return false
}

func ref(generation uint64) windowref.Ref {
	return windowref.Ref{
		ID: "hwnd:100", Handle: 100, ProcessID: 7, Application: "testgame",
		Generation: generation,
	}
}

func config() observesession.Config {
	b := observe.DefaultBounds()
	b.Duration = 10 * time.Second
	b.Interval = observe.MinInterval
	return observesession.Config{
		ID: "observe_1", Selector: windowref.Selector{Application: "testgame"}, Bounds: b,
	}
}

// ── the ordinary run ──────────────────────────────────────────────────────────

func TestASessionRunsToItsDurationAndIsComplete(t *testing.T) {
	clock := newClock()
	target := &steadyTarget{ref: ref(1)}
	sampler := &countingSampler{clock: clock}
	events := &recordingEvents{}

	got, err := observesession.New(clock, target, sampler, events).
		Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Session.State != observe.Completed {
		t.Fatalf("state = %q (%s), want completed", got.Session.State, got.Session.Reason)
	}
	if !got.Complete() {
		t.Error("a session that ran its duration is not reported complete")
	}
	if got.Stats.SamplesTaken == 0 {
		t.Fatal("no samples were taken")
	}
	if !events.has(observesession.SessionStarted) || !events.has(observesession.SessionCompleted) {
		t.Errorf("lifecycle events missing: %v", events.kinds())
	}
}

func TestTheFrameCapIsHonoured(t *testing.T) {
	clock := newClock()
	cfg := config()
	cfg.Bounds.Duration = 15 * time.Minute // far more time than frames
	cfg.Bounds.MaxFrames = 7
	sampler := &countingSampler{clock: clock}

	got, err := observesession.New(clock, &steadyTarget{ref: ref(1)}, sampler, nil).
		Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Stats.SamplesTaken != 7 {
		t.Fatalf("%d samples taken, cap was 7", got.Stats.SamplesTaken)
	}
	if got.Session.State != observe.Completed {
		t.Errorf("state = %q, want completed — the frame cap is a normal end", got.Session.State)
	}
}

func TestOutOfRangeBoundsAreRefusedBeforeAnythingRuns(t *testing.T) {
	clock := newClock()
	target := &steadyTarget{ref: ref(1)}
	cfg := config()
	cfg.Bounds.Duration = 5 * time.Hour

	if _, err := observesession.New(clock, target, &countingSampler{}, nil).
		Run(context.Background(), cfg); err == nil {
		t.Fatal("a five-hour session was accepted")
	}
	if target.calls != 0 {
		t.Error("the target was touched before the bounds were checked")
	}
}

func TestASelectorlessSessionIsRefused(t *testing.T) {
	cfg := config()
	cfg.Selector = windowref.Selector{}
	if _, err := observesession.New(newClock(), &steadyTarget{}, &countingSampler{}, nil).
		Run(context.Background(), cfg); err == nil {
		t.Fatal("a session with no explicit target was accepted")
	}
}

// ── scheduling ────────────────────────────────────────────────────────────────

func TestASlowCycleDoesNotQueueABacklog(t *testing.T) {
	// Sleeping for the interval AFTER the work drifts by the cost of the work every
	// cycle. Queueing the missed slots instead would run captures back to back and
	// describe a burst that never happened.
	clock := newClock()
	cfg := config()
	cfg.Bounds.Duration = 5 * time.Second
	cfg.Bounds.Interval = 200 * time.Millisecond
	// Each cycle costs three intervals.
	sampler := &countingSampler{clock: clock, cost: 600 * time.Millisecond}

	got, err := observesession.New(clock, &steadyTarget{ref: ref(1)}, sampler, nil).
		Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Five seconds of 800ms cycles is about six samples, not twenty-five.
	if got.Stats.SamplesTaken > 10 {
		t.Fatalf("%d samples in %s at %s per cycle — slots were queued rather than skipped",
			got.Stats.SamplesTaken, cfg.Bounds.Duration, 800*time.Millisecond)
	}
	if got.Stats.SamplesLate == 0 {
		t.Error("cycles overran their interval but no lateness was recorded; " +
			"an unreported overrun reads as a session that kept up")
	}
}

func TestOnlyOneSampleRunsAtATime(t *testing.T) {
	// The sampler asserts it is never re-entered. Overlapping captures would fight over
	// the screen and produce frames nobody could attribute to a moment.
	clock := newClock()
	var inFlight int32
	var mu sync.Mutex
	overlapped := false

	s := samplerFunc(func(_ context.Context, req observesession.SampleRequest) (observe.Sample, error) {
		mu.Lock()
		inFlight++
		if inFlight > 1 {
			overlapped = true
		}
		mu.Unlock()
		clock.advance(300 * time.Millisecond)
		mu.Lock()
		inFlight--
		mu.Unlock()
		return observe.Sample{WindowGeneration: req.Window.Generation}, nil
	})

	cfg := config()
	cfg.Bounds.Duration = 6 * time.Second
	if _, err := observesession.New(clock, &steadyTarget{ref: ref(1)}, s, nil).
		Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if overlapped {
		t.Fatal("two sample pipelines ran at once")
	}
}

// ── target loss ───────────────────────────────────────────────────────────────

func TestALostTargetProducesNoSampleAndNoSubstitute(t *testing.T) {
	clock := newClock()
	cfg := config()
	cfg.Bounds.Duration = 10 * time.Second
	cfg.Bounds.ReacquireWindow = 1 * time.Second
	target := &scriptedTarget{failFrom: 3, failUntil: 1 << 30, ref: ref(1)}
	sampler := &countingSampler{clock: clock}
	events := &recordingEvents{}

	got, err := observesession.New(clock, target, sampler, events).
		Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Session.State != observe.TargetUnavailable {
		t.Fatalf("state = %q, want target_unavailable", got.Session.State)
	}
	if got.Complete() {
		t.Fatal("a session that lost its target reports as complete")
	}
	if !strings.Contains(got.Session.Reason, "NOT complete") {
		t.Errorf("the reason %q does not say the session is incomplete", got.Session.Reason)
	}
	// Only the two samples taken before the loss.
	if sampler.calls != 2 {
		t.Errorf("the sampler ran %d times; it must not run without a validated target",
			sampler.calls)
	}
	if got.Stats.SamplesTaken != 2 {
		t.Errorf("%d samples retained, want the 2 taken before the loss", got.Stats.SamplesTaken)
	}
	if !events.has(observesession.TargetUnavailable) {
		t.Errorf("no target-unavailable event: %v", events.kinds())
	}
}

func TestATargetThatComesBackIsReacquiredAndTheSeamIsMarked(t *testing.T) {
	clock := newClock()
	cfg := config()
	cfg.Bounds.Duration = 10 * time.Second
	cfg.Bounds.ReacquireWindow = 5 * time.Second
	// Gone for three calls, then back as a NEW generation, as a restart would be.
	target := &scriptedTarget{failFrom: 3, failUntil: 5, ref: ref(1), after: ref(2)}
	events := &recordingEvents{}

	got, err := observesession.New(clock, target, &countingSampler{clock: clock}, events).
		Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Session.State != observe.Completed {
		t.Fatalf("state = %q (%s), want completed after a successful reacquisition",
			got.Session.State, got.Session.Reason)
	}
	if !events.has(observesession.TargetReacquired) {
		t.Errorf("no reacquisition event: %v", events.kinds())
	}
	if len(got.Stats.Generations) < 2 {
		t.Errorf("generations = %v, want both the old and the new", got.Stats.Generations)
	}

	seams := 0
	for _, tr := range got.Findings.Transitions {
		switch tr.Kind {
		case observe.TargetLost, observe.TargetReacquired, observe.WindowGenerationChanged:
			seams++
		}
	}
	if seams == 0 {
		t.Fatal("the timeline has no seam; evidence from two generations would read as continuous")
	}
}

// ── cancellation ──────────────────────────────────────────────────────────────

func TestCancellationStopsTheNextSampleAndKeepsTheEvidence(t *testing.T) {
	clock := newClock()
	ctx, cancel := context.WithCancel(context.Background())
	cfg := config()
	cfg.Bounds.Duration = 30 * time.Second

	var taken int
	s := samplerFunc(func(_ context.Context, req observesession.SampleRequest) (observe.Sample, error) {
		taken++
		if taken == 3 {
			cancel()
		}
		clock.advance(50 * time.Millisecond)
		return observe.Sample{
			WindowGeneration: req.Window.Generation,
			Entities: []observe.EntitySnapshot{{
				Identity: "thing", Role: "icon", Confidence: 0.9,
			}},
		}, nil
	})
	events := &recordingEvents{}

	got, err := observesession.New(clock, &steadyTarget{ref: ref(1)}, s, events).Run(ctx, cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Session.State != observe.Cancelled {
		t.Fatalf("state = %q, want cancelled", got.Session.State)
	}
	if got.Complete() {
		t.Fatal("a cancelled session reports as complete")
	}
	if taken > 4 {
		t.Errorf("%d samples ran after cancellation was requested at 3", taken)
	}
	if got.Stats.SamplesTaken < 3 {
		t.Errorf("evidence was discarded on cancellation: %d samples", got.Stats.SamplesTaken)
	}
	if !events.has(observesession.SessionCancelled) {
		t.Errorf("no cancellation event: %v", events.kinds())
	}
}

// ── failure ───────────────────────────────────────────────────────────────────

func TestARelentlesslyFailingSamplerEndsTheSession(t *testing.T) {
	// Without this a broken provider produces a full-length session of silence, which
	// looks exactly like a game with nothing on screen.
	clock := newClock()
	cfg := config()
	cfg.Bounds.Duration = 60 * time.Second

	got, err := observesession.New(clock, &steadyTarget{ref: ref(1)},
		&countingSampler{clock: clock, failAll: true}, nil).Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Session.State != observe.Failed {
		t.Fatalf("state = %q, want failed", got.Session.State)
	}
	if got.Stats.SamplerErrors == 0 {
		t.Error("sampler errors were not counted")
	}
	if !strings.Contains(got.Session.Reason, "silence") {
		t.Errorf("the reason %q does not explain why it stopped early", got.Session.Reason)
	}
}

func TestAnOccasionalFailureIsSurvived(t *testing.T) {
	clock := newClock()
	cfg := config()
	cfg.Bounds.Duration = 5 * time.Second

	got, err := observesession.New(clock, &steadyTarget{ref: ref(1)},
		&countingSampler{clock: clock, failEvery: 3}, nil).Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Session.State != observe.Completed {
		t.Fatalf("state = %q, want completed — one failure in three is not a broken session",
			got.Session.State)
	}
	if got.Stats.SamplesSkipped == 0 {
		t.Error("skipped samples were not counted")
	}
}

// ── OCR budget ────────────────────────────────────────────────────────────────

func TestLabelReadingIsRareAndCapped(t *testing.T) {
	// Measured live: 39 regions cost 9.0 seconds. Reading every frame of a three-minute
	// session would spend the entire budget re-reading text that had not changed.
	clock := newClock()
	cfg := config()
	cfg.Bounds.Duration = 20 * time.Second
	cfg.LabelEvery = 5
	cfg.MaxLabelPasses = 4
	sampler := &countingSampler{clock: clock}

	got, err := observesession.New(clock, &steadyTarget{ref: ref(1)}, sampler, nil).
		Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sampler.labelCalls > 4 {
		t.Fatalf("%d label passes, cap was 4", sampler.labelCalls)
	}
	if sampler.labelCalls == 0 {
		t.Fatal("no label pass ever ran; a session would have no readable names at all")
	}
	if sampler.labelCalls >= sampler.calls {
		t.Errorf("labels were read on %d of %d samples; the expensive pass must be the "+
			"exception", sampler.labelCalls, sampler.calls)
	}
	if got.Stats.LabelPasses != sampler.labelCalls {
		t.Errorf("reported %d label passes, actually ran %d",
			got.Stats.LabelPasses, sampler.labelCalls)
	}
}

func TestTheFirstSampleAlwaysReadsLabels(t *testing.T) {
	// So a session has names from the start rather than after the first multiple.
	clock := newClock()
	cfg := config()
	cfg.Bounds.Duration = 5 * time.Second
	cfg.Bounds.MaxFrames = 1
	cfg.LabelEvery = 50

	var readFirst bool
	s := samplerFunc(func(_ context.Context, req observesession.SampleRequest) (observe.Sample, error) {
		if req.Sequence == 1 {
			readFirst = req.ReadLabels
		}
		return observe.Sample{WindowGeneration: req.Window.Generation}, nil
	})
	if _, err := observesession.New(clock, &steadyTarget{ref: ref(1)}, s, nil).
		Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !readFirst {
		t.Fatal("the first sample did not read labels")
	}
}

// ── responsiveness ────────────────────────────────────────────────────────────

func TestStatusAnswersWhileASampleIsRunning(t *testing.T) {
	// `director status` must answer while a detector runs. The sampler blocks until the
	// test has read a snapshot; if Snapshot took the same lock the sampling path holds,
	// this deadlocks rather than fails, so the timeout is the assertion.
	clock := newClock()
	entered := make(chan struct{})
	release := make(chan struct{})

	s := samplerFunc(func(_ context.Context, req observesession.SampleRequest) (observe.Sample, error) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return observe.Sample{WindowGeneration: req.Window.Generation}, nil
	})

	cfg := config()
	cfg.Bounds.Duration = 5 * time.Second
	cfg.Bounds.MaxFrames = 1
	runner := observesession.New(clock, &steadyTarget{ref: ref(1)}, s, nil)

	done := make(chan struct{})
	go func() {
		_, _ = runner.Run(context.Background(), cfg)
		close(done)
	}()

	<-entered
	answered := make(chan observe.State, 1)
	go func() {
		session, _ := runner.Snapshot()
		answered <- session.State
	}()

	select {
	case state := <-answered:
		if state != observe.Observing {
			t.Errorf("status reported %q mid-sample, want observing", state)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("status did not answer while a sample was in flight; the sampling path " +
			"is holding a lock the control plane needs")
	}
	close(release)
	<-done
}

// samplerFunc adapts a function to the Sampler interface.
type samplerFunc func(context.Context, observesession.SampleRequest) (observe.Sample, error)

func (f samplerFunc) Sample(ctx context.Context, r observesession.SampleRequest) (observe.Sample, error) {
	return f(ctx, r)
}
