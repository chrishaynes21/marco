package providers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/internal/fixtures"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The evidence side of the boundary.
//
// What is being protected here is not that providers work — that was true before — but
// that they now do LESS. A provider returns observations and nothing else: no element
// ids, no world, no decision about what two reports mean together. The tests that
// matter most are the ones asserting a provider's silence.

// fakeAccessibility replays a recorded snapshot, or an error.
type fakeAccessibility struct {
	snap   directorapi.AccessibilitySnapshot
	err    error
	calls  int
	scopes []directorapi.WindowID
}

func (f *fakeAccessibility) Snapshot(_ context.Context, scope directorapi.WindowID) (directorapi.AccessibilitySnapshot, error) {
	f.calls++
	f.scopes = append(f.scopes, scope)
	return f.snap, f.err
}

// fakeWindows is a window source that records what it was asked to do.
type fakeWindows struct {
	monitors  []directorapi.Monitor
	monErr    error
	enriched  int
	enrichAll func([]directorapi.Window) []directorapi.Window
}

func (f *fakeWindows) Monitors(context.Context) ([]directorapi.Monitor, error) {
	return f.monitors, f.monErr
}

func (f *fakeWindows) Enrich(w []directorapi.Window) []directorapi.Window {
	f.enriched++
	if f.enrichAll != nil {
		return f.enrichAll(w)
	}
	return w
}

// desktop turns a fixture into the snapshot an accessibility source would return.
func desktop(t *testing.T, name string) directorapi.AccessibilitySnapshot {
	t.Helper()
	d := fixtures.Load(t, name)
	return directorapi.AccessibilitySnapshot{
		Timestamp:    time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC),
		Observations: d.Observations,
		Windows:      []directorapi.Window{d.Window},
		Partial:      d.Partial,
		Reason:       d.Reason,
	}
}

// ── 1. accessibility observations ─────────────────────────────────────────────

func TestAccessibilityEmitsObservationsAndNotElements(t *testing.T) {
	snap := desktop(t, "save-dialog")
	p := NewAccessibility(&fakeAccessibility{snap: snap})

	obs, err := p.Observe(context.Background(), observation.Request{})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}

	// Every recorded control arrives as element evidence, plus the window and the
	// application it belongs to.
	elements := observation.Elements(obs)
	if len(elements) != len(snap.Observations) {
		t.Errorf("%d element observations, want %d", len(elements), len(snap.Observations))
	}
	if got := len(observation.Windows(obs)); got != 1 {
		t.Errorf("%d window observations, want 1", got)
	}
	if _, ok := observation.ActiveApplication(obs); !ok {
		t.Error("no application observation — nothing would know which app is in front")
	}

	// The interface every kind satisfies, and the thing fusion actually reads.
	for _, o := range obs {
		if o.ID() == "" {
			t.Errorf("an observation has no id: %#v", o)
		}
		if o.Source() != directorapi.SourceAccessibility {
			t.Errorf("source = %q, want accessibility", o.Source())
		}
		if o.Timestamp().IsZero() {
			t.Errorf("observation %s has no timestamp", o.ID())
		}
		if o.Kind() == "" {
			t.Errorf("observation %s has no kind", o.ID())
		}
	}
}

func TestAScopedRequestReachesTheSourceAndAnIgnoredRegionIsNotPretendedAway(t *testing.T) {
	src := &fakeAccessibility{snap: desktop(t, "save-dialog")}
	p := NewAccessibility(src)

	win := directorapi.WindowID("w-42")
	region := directorapi.Rect{X: 0, Y: 0, Width: 100, Height: 100}
	if _, err := p.Observe(context.Background(), observation.Request{
		Window: &win, Region: &region,
	}); err != nil {
		t.Fatalf("Observe: %v", err)
	}

	// The window scope is honoured — it is the one narrowing a tree walk can express.
	if len(src.scopes) != 1 || src.scopes[0] != win {
		t.Errorf("scopes = %v, want the request's window to reach the source", src.scopes)
	}
	// The region is not. A tree walk enumerates objects and has no rectangle to
	// restrict itself to; the point of the test is that the provider does not quietly
	// filter by bounds and let a caller believe it had scoped the work.
	obs, _ := p.Observe(context.Background(), observation.Request{Region: &region})
	if len(observation.Elements(obs)) == 0 {
		t.Error("a region narrowed the result — accessibility cannot honour Region and must not pretend to")
	}
}

// ── 2. provider failure ───────────────────────────────────────────────────────

func TestAFailedProviderDegradesTheCycleRatherThanEmptyingIt(t *testing.T) {
	broken := &fakeAccessibility{err: errors.New("the bridge is not running")}
	c := NewCollector(NewAccessibility(broken), NewWindowSystem(&fakeWindows{}))

	cycle := c.Collect(context.Background(), observation.Request{})

	if len(cycle.Observations) != 0 {
		t.Errorf("%d observations from a failed provider", len(cycle.Observations))
	}
	if len(cycle.Failures) != 1 {
		t.Fatalf("%d failures recorded, want 1 — a provider that failed must not leave a gap",
			len(cycle.Failures))
	}
	if cycle.Failures[0].Source != directorapi.SourceAccessibility {
		t.Errorf("failure blamed %q", cycle.Failures[0].Source)
	}
	if cycle.Failures[0].Reason == "" {
		t.Error("the failure carries no reason")
	}
	// The cycle is still a cycle. "I could not see" is an observable state, not a
	// transport error a caller should retry.
	if cycle.ID == "" || cycle.CompletedAt.IsZero() {
		t.Error("a failed cycle must still be a complete, identified cycle")
	}
}

func TestAPartialWalkKeepsItsEvidenceAndStillReportsDegradation(t *testing.T) {
	// The distinction that keeps "I could not read this application" from looking like
	// "the button is not there". Both would otherwise arrive as a short element list.
	snap := desktop(t, "save-dialog")
	snap.Partial = true
	snap.Reason = "node cap reached"

	c := NewCollector(NewAccessibility(&fakeAccessibility{snap: snap}))
	cycle := c.Collect(context.Background(), observation.Request{})

	if len(observation.Elements(cycle.Observations)) == 0 {
		t.Fatal("a partial walk's evidence was discarded — it is real evidence")
	}
	if len(cycle.Failures) != 1 {
		t.Fatalf("%d failures, want the truncation recorded", len(cycle.Failures))
	}
	if cycle.Failures[0].Reason != "node cap reached" {
		t.Errorf("reason = %q", cycle.Failures[0].Reason)
	}
}

// ── 3. empty and duplicate evidence ───────────────────────────────────────────

func TestAnEmptyObservationSetIsACycleAndNotAnError(t *testing.T) {
	c := NewCollector(NewAccessibility(&fakeAccessibility{}))
	cycle := c.Collect(context.Background(), observation.Request{})

	if len(cycle.Observations) != 0 {
		t.Errorf("%d observations from an empty desktop", len(cycle.Observations))
	}
	if len(cycle.Failures) != 0 {
		t.Errorf("an empty desktop is not a failure: %v", cycle.Failures)
	}
	if cycle.ID == "" {
		t.Error("an empty cycle still needs an id")
	}
}

func TestObservationsWithoutIdsAreGivenDistinctOnes(t *testing.T) {
	// Provenance is by observation id. Two observations sharing one would make an
	// element's evidence ambiguous, and Provenance.Add would silently drop the second.
	snap := directorapi.AccessibilitySnapshot{
		Timestamp: time.Now(),
		Observations: []directorapi.Observation{
			{Source: directorapi.SourceAccessibility, Label: "One"},
			{Source: directorapi.SourceAccessibility, Label: "Two"},
		},
	}
	p := NewAccessibility(&fakeAccessibility{snap: snap})
	obs, _ := p.Observe(context.Background(), observation.Request{})

	seen := map[directorapi.ObservationID]bool{}
	for _, o := range obs {
		if o.ID() == "" {
			t.Fatal("an observation was left without an id")
		}
		if seen[o.ID()] {
			t.Fatalf("duplicate observation id %q", o.ID())
		}
		seen[o.ID()] = true
	}
}

// ── 4. observation cycles ─────────────────────────────────────────────────────

func TestACycleIsTimedIdentifiedAndSelfDescribing(t *testing.T) {
	c := NewCollector(
		NewAccessibility(&fakeAccessibility{snap: desktop(t, "save-dialog")}),
		NewWindowSystem(&fakeWindows{monitors: []directorapi.Monitor{{ID: "m1"}}}),
	)
	cycle := c.Collect(context.Background(), observation.Request{})

	if cycle.ID == "" {
		t.Error("the cycle has no id")
	}
	if cycle.StartedAt.IsZero() || cycle.CompletedAt.IsZero() {
		t.Error("the cycle is not timed")
	}
	if cycle.CompletedAt.Before(cycle.StartedAt) {
		t.Error("the cycle completed before it started")
	}
	// The world is stamped with the START, not the completion: a snapshot is only as
	// fresh as its oldest evidence.
	if !cycle.Timestamp().Equal(cycle.StartedAt) {
		t.Errorf("Timestamp() = %v, want the start %v", cycle.Timestamp(), cycle.StartedAt)
	}
	if len(cycle.Environment.Monitors) != 1 {
		t.Errorf("%d monitors, want the environment collected", len(cycle.Environment.Monitors))
	}

	// Two cycles are two cycles, even from identical evidence.
	second := c.Collect(context.Background(), observation.Request{})
	if second.ID == cycle.ID {
		t.Error("two cycles share an id")
	}
}

func TestHistoryIsBoundedAndCountsWhatItDropped(t *testing.T) {
	h := observation.NewHistory(3)
	for i := 0; i < 10; i++ {
		h.Record(observation.Cycle{
			ID:           observation.CycleID(string(rune('a' + i))),
			Observations: []observation.Observation{observation.NewElement(directorapi.Observation{ID: "x"})},
		})
	}

	recent := h.Recent()
	if len(recent) != 3 {
		t.Fatalf("%d cycles retained, want the bound of 3", len(recent))
	}
	// Newest first, and the oldest seven are gone — observations are ephemeral, and an
	// unbounded history is a leak that grows with uptime.
	if recent[0].ID != "j" || recent[2].ID != "h" {
		t.Errorf("retained %v, want the newest three newest-first", []observation.CycleID{
			recent[0].ID, recent[1].ID, recent[2].ID,
		})
	}
	// Without the total, a service that has observed all day looks like one that just
	// started.
	if h.Total() != 10 {
		t.Errorf("Total() = %d, want 10", h.Total())
	}
	if latest, ok := h.Latest(); !ok || latest.ID != "j" {
		t.Errorf("Latest() = %v", latest.ID)
	}
}

// ── 5. provider isolation ─────────────────────────────────────────────────────

func TestOneProviderCannotSuppressAnother(t *testing.T) {
	// Isolation is the property that makes adding a source safe. A new provider that
	// throws, hangs on its own error path, or returns nothing must cost the Director
	// that source and nothing else.
	broken := NewAccessibility(&fakeAccessibility{err: errors.New("boom")})
	working := NewAccessibility(&fakeAccessibility{snap: desktop(t, "save-dialog")})

	c := NewCollector(broken, working)
	cycle := c.Collect(context.Background(), observation.Request{})

	if len(observation.Elements(cycle.Observations)) == 0 {
		t.Error("a failing provider suppressed a working one")
	}
	if len(cycle.Failures) != 1 {
		t.Errorf("%d failures, want exactly the broken one", len(cycle.Failures))
	}
}

func TestASourceFilteredRequestSkipsTheProvidersItDidNotAskFor(t *testing.T) {
	// How an expensive source stays opt-in. A vision pass on every cycle would be
	// unusable, so a provider that was not asked for must not be started at all.
	acc := &fakeAccessibility{snap: desktop(t, "save-dialog")}
	win := &fakeWindows{}
	c := NewCollector(NewAccessibility(acc), NewWindowSystem(win))

	c.Collect(context.Background(), observation.Request{
		Sources: []observation.Source{directorapi.SourceWindowSystem},
	})

	if acc.calls != 0 {
		t.Errorf("accessibility ran %d times for a window-system-only request", acc.calls)
	}
}

// ── 6. no additional desktop work ─────────────────────────────────────────────

func TestACycleCostsExactlyTheSamePlatformCallsAsBefore(t *testing.T) {
	// The milestone's explicit performance requirement, pinned rather than asserted in
	// prose. Before the pipeline existed, one observation was one accessibility
	// snapshot, one window enrichment and one monitor query. It still is — the
	// providers reorganised who makes those calls, not how many are made.
	//
	// Worth a test because the shape invites regression: an extra Enrich per window,
	// or monitors fetched per provider, would be invisible in every other test and
	// would show up live as a Director that got slower for no reason anyone could name.
	acc := &fakeAccessibility{snap: desktop(t, "save-dialog")}
	win := &fakeWindows{}
	c := NewCollector(NewAccessibility(acc), NewWindowSystem(win))

	c.Collect(context.Background(), observation.Request{})

	if acc.calls != 1 {
		t.Errorf("%d accessibility snapshots per cycle, want 1", acc.calls)
	}
	if win.enriched != 1 {
		t.Errorf("%d window enrichments per cycle, want 1 batched call", win.enriched)
	}
}

func TestTheWindowSystemRefinesWindowEvidenceWithoutProducingAny(t *testing.T) {
	acc := &fakeAccessibility{snap: desktop(t, "save-dialog")}
	win := &fakeWindows{
		enrichAll: func(ws []directorapi.Window) []directorapi.Window {
			for i := range ws {
				ws[i].Bounds = directorapi.Rect{X: 10, Y: 20, Width: 300, Height: 200}
			}
			return ws
		},
	}
	c := NewCollector(NewAccessibility(acc), NewWindowSystem(win))
	cycle := c.Collect(context.Background(), observation.Request{})

	if win.enriched != 1 {
		t.Errorf("Enrich called %d times, want exactly one batched call", win.enriched)
	}
	windows := observation.Windows(cycle.Observations)
	if len(windows) != 1 {
		t.Fatalf("%d windows", len(windows))
	}
	if windows[0].Bounds.Width != 300 {
		t.Errorf("the refinement did not land: %+v", windows[0].Bounds)
	}
	// A refiner produces nothing of its own. If it ever does, it is a Provider.
	alone := NewCollector(NewWindowSystem(win)).Collect(context.Background(), observation.Request{})
	if len(alone.Observations) != 0 {
		t.Errorf("the window system produced %d observations on its own", len(alone.Observations))
	}
}
