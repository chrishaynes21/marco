package main

import (
	"context"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
)

// The navigation producer, entered through the production session path.
//
// The rule this test obeys, and the reason it is written the way it is: it drives RAW VIRTUAL
// KEY CODES through the real source. A test that put observe.InputEvent values straight onto a
// ShadowSample would prove the plumbing downstream of the boundary and nothing about the
// boundary itself — and would have passed just as happily during the entire period there was no
// producer at all, which is the failure this repository keeps rediscovering.

// navSampler is the production shape: it holds a session-scoped subscription and drains it onto
// the sample, exactly as cmd/director's liveSampler does.
type navSampler struct {
	sub   *navsource.Subscription
	calls int
	// menuFrom is the sample index at which the screen becomes menu-like.
	menuFrom int
	// press, when set, is fired on the sample BEFORE the screen changes — which is where
	// a real keypress lands: between two observations, not during one.
	press func(uint16, bool)
	// detach mirrors the composition root's session-end hook.
	detached bool
}

// pressBetweenSamples fires a key and waits for the worker to classify it, so the next sample
// drains it. Real play has ~3.5s between observations and no such race; a fake clock has none.
func pressBetweenSamples(press func(uint16, bool), sub *navsource.Subscription, code uint16) {
	if press == nil {
		return
	}
	press(code, true)
	press(code, false)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && sub.Peek() == 0 {
		time.Sleep(time.Millisecond)
	}
}

func (s *navSampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	s.calls++
	hud := observe.ShadowRegion{
		Role: "icon", Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}
	regions := []observe.ShadowRegion{hud}
	if s.calls >= s.menuFrom {
		for _, y := range []float64{0.437, 0.480, 0.520, 0.562} {
			regions = append(regions, observe.ShadowRegion{
				Role: "button", Nameable: true,
				Region: observe.Region{X: 0.414, Y: y, Width: 0.172, Height: 0.036},
			})
		}
	}
	sh := &observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true,
		Detections: len(regions), Regions: regions, LatencyMS: 860,
	}
	if s.sub != nil {
		sh.Inputs = s.sub.Drain()
	}
	// The player acts BETWEEN observations: the press lands after this sample was taken
	// and before the next one, which is the sample that first shows the menu.
	if s.calls == s.menuFrom-1 {
		pressBetweenSamples(s.press, s.sub, navsource.KeyEscape)
	}
	return observe.Sample{Shadow: sh}, nil
}

func (s *navSampler) detachNavigation() { s.detached = true }

// THE producer wiring test. A real keypress becomes an attributed graph edge.
func TestARealKeypressBecomesAnAttributedTransitionEdge(t *testing.T) {
	src, press := navsource.NewSynthetic()
	defer src.Close()

	sub := src.Open(time.Now())
	// The player presses Escape between two observations. A RAW virtual-key code goes
	// in; the closed vocabulary is the only thing that may come out.
	sampler := &navSampler{sub: sub, menuFrom: 3, press: press}

	got, err := observesession.New(newNavClock(), navTarget{}, sampler,
		observesession.NopEvents{}).Run(context.Background(), navConfig())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	sh := got.Stats.Shadow
	if !sh.Observed() {
		t.Fatal("no shadow observations; this test can say nothing about navigation")
	}

	var attributed int
	for _, tr := range sh.Transitions {
		attributed += tr.Preceded[observe.NavPause]
	}
	if attributed == 0 {
		t.Fatalf("a real Escape keypress travelled through the producer and no state "+
			"transition records it. The navigation source is not reaching the discovery "+
			"path. transitions=%v", sh.Transitions)
	}
}

// waitForIntent lets the classifier worker catch up without draining the buffer.
func waitForIntent(t *testing.T, sub *navsource.Subscription) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := sub.Peek(); got > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("the synthetic keypress never became a navigation intent")
}

// Navigation observed while the detector SKIPPED its slot must still be attributed.
//
// This is the production-path form of TestNavigationDuringASkippedSlotIsNotLost. At the real
// skip rate roughly a third of slots carry no screen evidence, and the keypress that opens a
// menu lands in one of them far more often than not.
func TestNavigationSurvivesASkippedDetectorSlotThroughTheProducer(t *testing.T) {
	src, press := navsource.NewSynthetic()
	defer src.Close()
	sub := src.Open(time.Now())

	sampler := &skippingNavSampler{sub: sub, skipAt: 2, menuFrom: 3, press: press}

	got, err := observesession.New(newNavClock(), navTarget{}, sampler,
		observesession.NopEvents{}).Run(context.Background(), navConfig())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var attributed int
	for _, tr := range got.Stats.Shadow.Transitions {
		attributed += tr.Preceded[observe.NavPause]
	}
	if attributed == 0 {
		t.Fatal("navigation delivered during a skipped detector slot was lost on the " +
			"production path")
	}
}

type skippingNavSampler struct {
	sub              *navsource.Subscription
	calls            int
	skipAt, menuFrom int
	press            func(uint16, bool)
}

func (s *skippingNavSampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	s.calls++
	sh := &observe.ShadowSample{Detector: "screenparser"}
	if s.sub != nil {
		sh.Inputs = s.sub.Drain()
	}
	if s.calls == s.skipAt {
		// The cadence gate declined this slot: no screen evidence, input still real.
		return observe.Sample{Shadow: sh}, nil
	}
	sh.Ran, sh.TargetProven, sh.LatencyMS = true, true, 860
	sh.Regions = []observe.ShadowRegion{{
		Role: "icon", Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}}
	if s.calls >= s.menuFrom {
		for _, y := range []float64{0.437, 0.480, 0.520, 0.562} {
			sh.Regions = append(sh.Regions, observe.ShadowRegion{
				Role: "button", Nameable: true,
				Region: observe.Region{X: 0.414, Y: y, Width: 0.172, Height: 0.036},
			})
		}
	}
	sh.Detections = len(sh.Regions)
	// The press lands in the slot the detector is about to SKIP.
	if s.calls == s.skipAt-1 {
		pressBetweenSamples(s.press, s.sub, navsource.KeyEscape)
	}
	return observe.Sample{Shadow: sh}, nil
}

// A transition nobody was seen to cause stays unattributed.
//
// The essential control. Without it every correlation would be unfalsifiable: a model that can
// only ever say "something preceded this" is not measuring anything.
func TestATransitionWithNoNavigationStaysUnattributed(t *testing.T) {
	src, _ := navsource.NewSynthetic()
	defer src.Close()
	sub := src.Open(time.Now())

	got, err := observesession.New(newNavClock(), navTarget{},
		&navSampler{sub: sub, menuFrom: 3}, observesession.NopEvents{}).
		Run(context.Background(), navConfig())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var unattributed, attributed int
	for _, tr := range got.Stats.Shadow.Transitions {
		unattributed += tr.Unattributed
		attributed += tr.Attributed()
	}
	if attributed != 0 {
		t.Errorf("%d transitions were attributed to navigation in a session with no "+
			"keypresses at all", attributed)
	}
	if unattributed == 0 {
		t.Error("a screen change with no input before it was not recorded as unattributed")
	}
}

// ── minimal session harness ───────────────────────────────────────────────────
//
// Redefined here rather than shared with internal/director's own tests, because
// TestDirectorImportsNoPlatformCode forbids the Director from importing a platform package —
// which is exactly right, and is why the producer wiring test belongs at the composition root
// where both halves are legitimately in scope.

type navClock struct {
	mu  sync.Mutex
	now time.Time
}

func newNavClock() *navClock {
	return &navClock{now: time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)}
}

func (c *navClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *navClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
	ch := make(chan time.Time, 1)
	ch <- c.Now()
	return ch
}

type navTarget struct{}

func (navTarget) Acquire(context.Context, windowref.Selector) (windowref.Ref, error) {
	return windowref.Ref{
		ID: "hwnd:100", Handle: 100, ProcessID: 7, Application: "testgame", Generation: 1,
	}, nil
}

func navConfig() observesession.Config {
	b := observe.DefaultBounds()
	b.Duration = 10 * time.Second
	b.Interval = observe.MinInterval
	return observesession.Config{
		ID: "observe_nav", Selector: windowref.Selector{Application: "testgame"}, Bounds: b,
	}
}
