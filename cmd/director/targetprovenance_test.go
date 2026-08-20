package main

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The live resolver: the one place a provider's claim meets the real desktop.
//
// Everything above this is tested against fakes, and rightly — the race cannot be staged on
// a real desktop on demand. What cannot be tested against a fake is whether the production
// wiring resolves against the tracker at all, which is precisely the failure this subsystem
// has already had once: a complete, correct, fully unit-tested mechanism that nothing called.

// resolverDesktop is the smallest Platform a tracker needs.
type resolverDesktop struct {
	windows map[uintptr]windowref.Candidate
	dead    map[uint32]bool
}

func newResolverDesktop() *resolverDesktop {
	return &resolverDesktop{
		windows: map[uintptr]windowref.Candidate{}, dead: map[uint32]bool{},
	}
}

func (d *resolverDesktop) open(handle uintptr, pid uint32, app string) windowref.Candidate {
	c := windowref.Candidate{
		ID:          directorapi.WindowID(windowIDOf(handle)),
		Handle:      handle,
		ProcessID:   pid,
		Application: app,
		Title:       app,
		Bounds:      directorapi.Rect{X: 0, Y: 0, Width: 800, Height: 600},
		Visible:     true,
		OnScreen:    true,
	}
	d.windows[handle] = c
	return c
}

func (d *resolverDesktop) close(handle uintptr) { delete(d.windows, handle) }

func (d *resolverDesktop) Live(_ context.Context, h uintptr) (windowref.Candidate, bool) {
	c, ok := d.windows[h]
	return c, ok
}

func (d *resolverDesktop) ProcessAlive(_ context.Context, pid uint32) bool { return !d.dead[pid] }

func (d *resolverDesktop) Candidates(_ context.Context, app string) []windowref.Candidate {
	var out []windowref.Candidate
	for h := uintptr(0); h < 1000; h++ {
		if c, ok := d.windows[h]; ok && c.Application == app {
			out = append(out, c)
		}
	}
	return out
}

// windowIDOf renders a handle the way the Director does.
func windowIDOf(h uintptr) string {
	const digits = "0123456789"
	if h == 0 {
		return "hwnd:0"
	}
	var b []byte
	for ; h > 0; h /= 10 {
		b = append([]byte{digits[h%10]}, b...)
	}
	return "hwnd:" + string(b)
}

func liveTrackerOn(t *testing.T, d *resolverDesktop, app string) (*windowref.Tracker, windowref.Ref) {
	t.Helper()
	tr := windowref.NewTracker(d)
	v := tr.Acquire(context.Background(), app)
	if !v.State.OK() {
		t.Fatalf("the fixture could not acquire its own window: %s", v.Reason)
	}
	return tr, v.Ref
}

// ── the trap ──────────────────────────────────────────────────────────────────

func TestExpectedAndObservedComeFromDifferentPaths(t *testing.T) {
	// The defect that made the first version of this guard useless: both sides were
	// copies of one value, so they matched by construction and the guard passed exactly
	// the stale evidence it existed to reject.
	//
	// Here the two are built from genuinely different sources — the expectation from the
	// reference the runner validated, the observation from re-reading the platform — and
	// the test forces them apart by changing the desktop between the two calls. An
	// implementation that copied the expectation would report agreement.
	d := newResolverDesktop()
	d.open(100, 42, "notepad")
	tracker, validated := liveTrackerOn(t, d, "notepad")

	expected := expectedTarget(validated)
	if expected == nil || expected.WindowGeneration == 0 {
		t.Fatalf("the expectation carries no generation: %+v", expected)
	}

	// The window goes away after the runner validated it and before the resolver is
	// consulted — the shape of the in-flight replacement.
	d.close(100)

	resolver := newTargetResolver(tracker)
	observed, ok := resolver.ResolveObserved(context.Background(), validated.ID, "notepad")
	if ok {
		t.Fatalf("the resolver attributed evidence to a window that is gone: %+v", observed)
	}
	// And the guard's own rule agrees, which is what closes the loop: an unknown observed
	// target never matches a known expectation.
	if observed.Matches(*expected) {
		t.Error("an unestablished provenance matched the expectation")
	}
}

func TestTheResolverProvesAWindowThatIsStillItself(t *testing.T) {
	// The control. A guard that refused everything would satisfy the test above and make
	// every targeted session blind.
	d := newResolverDesktop()
	d.open(100, 42, "notepad")
	tracker, validated := liveTrackerOn(t, d, "notepad")

	resolver := newTargetResolver(tracker)
	observed, ok := resolver.ResolveObserved(context.Background(), validated.ID, "notepad")
	if !ok {
		t.Fatal("an unchanged window could not be proven")
	}
	if !observed.Matches(*expectedTarget(validated)) {
		t.Errorf("observed %+v does not match the expectation for the same window", observed)
	}
}

// ── the fallback a bridge performs on its own ─────────────────────────────────

func TestEvidenceFromADifferentWindowIsNotAttributedToTheTarget(t *testing.T) {
	// What an accessibility bridge falling back to the foreground window looks like from
	// here: an honest answer about a window nobody asked about. It must not be attributed
	// to the target merely because the target is what was requested.
	d := newResolverDesktop()
	d.open(100, 42, "notepad")
	d.open(200, 99, "calculator")
	tracker, _ := liveTrackerOn(t, d, "notepad")

	resolver := newTargetResolver(tracker)
	if _, ok := resolver.ResolveObserved(context.Background(),
		directorapi.WindowID(windowIDOf(200)), "calculator"); ok {
		t.Fatal("evidence read from another window was attributed to the tracked target")
	}
}

func TestAnApplicationDisagreementIsRefused(t *testing.T) {
	// A window id that matches while the application does not is the recycled-handle
	// signature, and the whole reason window identity is generational.
	d := newResolverDesktop()
	d.open(100, 42, "notepad")
	tracker, validated := liveTrackerOn(t, d, "notepad")

	resolver := newTargetResolver(tracker)
	if _, ok := resolver.ResolveObserved(context.Background(),
		validated.ID, "calculator"); ok {
		t.Fatal("a window claimed by two different applications resolved cleanly")
	}
}

// ── a Director with nothing to track ──────────────────────────────────────────

func TestADirectorWithNoTrackerProvesNothingRatherThanEverything(t *testing.T) {
	// Nil is a real configuration. It must fail closed: unprovable, therefore refused on
	// a targeted cycle, rather than quietly behaving like a proven one.
	if r := newTargetResolver(nil); r != nil {
		t.Fatalf("a Director with no window tracker produced a resolver: %#v", r)
	}
}

func TestExpectedTargetOfAnEmptyReferenceIsNoExpectation(t *testing.T) {
	// No window, no claim. An expectation invented from a zero reference would be a
	// generation of 0 that every unproven provider would then match.
	if got := expectedTarget(windowref.Ref{}); got != nil {
		t.Fatalf("a zero reference produced an expectation: %+v", got)
	}
}
