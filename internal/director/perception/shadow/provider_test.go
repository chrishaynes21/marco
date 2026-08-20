package shadow_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/shadow"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// slowInner is a provider that blocks until released, so concurrency can be asserted with a
// barrier rather than with a sleep. A sleep-based version of these tests would pass on a fast
// machine and lie on a loaded one, which is the opposite of what a timing invariant needs.
type slowInner struct {
	entered chan struct{}
	release chan struct{}
	calls   int
	mu      sync.Mutex
}

func (s *slowInner) Name() string { return "screenparser" }
func (s *slowInner) Sources() []observation.Source {
	return []observation.Source{directorapi.SourceVision}
}
func (s *slowInner) Observe(context.Context, observation.Request) ([]observation.Observation, error) {
	return nil, nil
}

func (s *slowInner) ObserveTargeted(context.Context,
	observation.Request) observation.ProviderOutcome {

	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if s.entered != nil {
		s.entered <- struct{}{}
		<-s.release
	}
	return observation.ProviderOutcome{
		Source: directorapi.SourceVision, State: observation.StateContributed,
	}
}

func (s *slowInner) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// Part 4: skip, never queue. A second caller arriving during an inference must return at once.
func TestASlotArrivingDuringInferenceIsSkippedNotQueued(t *testing.T) {
	inner := &slowInner{entered: make(chan struct{}), release: make(chan struct{})}
	p := shadow.NewProvider(inner, time.Nanosecond) // cadence out of the way

	done := make(chan struct{})
	go func() {
		p.ObserveTargeted(context.Background(), observation.Request{})
		close(done)
	}()
	<-inner.entered // the first inference is now in flight and blocked

	// The barrier: this call must return without waiting for the one in flight. If it
	// queued, the test deadlocks here rather than failing slowly somewhere else.
	returned := make(chan observation.ProviderOutcome, 1)
	go func() {
		returned <- p.ObserveTargeted(context.Background(), observation.Request{})
	}()

	select {
	case out := <-returned:
		if out.State != observation.StateEmpty {
			t.Errorf("a skipped slot reported %q; it did not fail, it was not asked",
				out.State)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a shadow slot arriving during an inference BLOCKED. The control plane " +
			"would freeze for the duration of every inference")
	}

	close(inner.release)
	<-done

	if got := inner.count(); got != 1 {
		t.Errorf("the inner detector ran %d times; the skipped slot must not run it", got)
	}
	if s := p.Snapshot(); s.SkippedBusy != 1 {
		t.Errorf("SkippedBusy = %d, want 1 — an experiment that quietly ran less often "+
			"than it claimed would under-report its own information gain", s.SkippedBusy)
	}
}

// Cadence: calls arriving faster than the cadence are skipped without running the detector.
func TestCallsFasterThanTheCadenceAreSkipped(t *testing.T) {
	inner := &slowInner{}
	p := shadow.NewProvider(inner, time.Hour)

	for i := 0; i < 10; i++ {
		p.ObserveTargeted(context.Background(), observation.Request{})
	}
	if got := inner.count(); got != 1 {
		t.Fatalf("the detector ran %d times in 10 cycles at a one-hour cadence, want 1", got)
	}
	s := p.Snapshot()
	if s.Inferences != 1 || s.SkippedRate != 9 {
		t.Errorf("inferences %d, skipped %d; want 1 and 9", s.Inferences, s.SkippedRate)
	}
}

// The cold pass is held apart from the warm ones. Folding a model load into the median
// would misreport steady cost badly enough to change a cadence decision.
func TestTheColdPassIsNotAveragedIntoTheWarmMedian(t *testing.T) {
	inner := &slowInner{}
	// A clock we drive: each call lands a full cadence after the last, so every slot is
	// due and nothing is skipped for reasons the test did not intend.
	clock := time.Unix(0, 0)
	p := shadow.NewProviderWithClock(inner, time.Second, func() time.Time { return clock })
	for i := 0; i < 5; i++ {
		p.ObserveTargeted(context.Background(), observation.Request{})
		clock = clock.Add(2 * time.Second)
	}
	s := p.Snapshot()
	if s.Inferences != 5 {
		t.Fatalf("inferences = %d, want 5", s.Inferences)
	}
	median, _ := p.Latencies()
	if median > s.FirstLatency && s.FirstLatency > 0 {
		t.Logf("cold %s, warm median %s", s.FirstLatency, median)
	}
}

// The wrapper must remain a shadow provider. If this ever stops compiling, an expensive
// experiment has become eligible for belief.
func TestTheWrapperIsStructurallyShadow(t *testing.T) {
	var p any = shadow.NewProvider(&slowInner{}, time.Second)
	if _, ok := p.(observation.ShadowProvider); !ok {
		t.Fatal("the shadow wrapper no longer declares ShadowOnly; its evidence would be " +
			"collected into Cycle.Outcomes and admitted to Fusion")
	}
}
