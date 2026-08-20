package providers

import (
	"context"
	"errors"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Accessibility must PROVE what it read, not assume it.
//
// The defining property: changing what the platform actually observed must change
// ObservedTarget without changing the request. An implementation that copies req.Target
// fails every test whose expected and observed values differ.

// fakeSource returns a scripted snapshot, optionally after running a hook — which is how
// the race test replaces the window while the snapshot is "in flight".
type fakeSource struct {
	snap     directorapi.AccessibilitySnapshot
	err      error
	duringFn func()
}

func (f *fakeSource) Snapshot(context.Context,
	directorapi.WindowID) (directorapi.AccessibilitySnapshot, error) {

	if f.duringFn != nil {
		f.duringFn()
	}
	return f.snap, f.err
}

// fakeResolver answers what a window IS now, from a table the test controls.
//
// This stands in for re-reading the platform: the test mutates it to make the world change
// under the provider, exactly as a real window replacement would.
type fakeResolver struct {
	byWindow map[directorapi.WindowID]directorapi.TargetProvenance
}

func (f *fakeResolver) ResolveObserved(_ context.Context, w directorapi.WindowID,
	_ string) (directorapi.TargetProvenance, bool) {

	got, ok := f.byWindow[w]
	return got, ok
}

func snapshotWith(window directorapi.WindowID, app string, n int) directorapi.AccessibilitySnapshot {
	snap := directorapi.AccessibilitySnapshot{ObservedWindow: window, ObservedApp: app}
	for i := range n {
		snap.Observations = append(snap.Observations, directorapi.Observation{
			ID: directorapi.ObservationID(string(rune('a' + i))), Role: directorapi.RoleButton,
		})
	}
	return snap
}

func prov(app string, pid uint32, gen uint64) directorapi.TargetProvenance {
	return directorapi.TargetProvenance{Application: app, ProcessID: pid, WindowGeneration: gen}
}

func targetedRequest(t directorapi.TargetProvenance, w directorapi.WindowID) observation.Request {
	return observation.Request{Window: &w, Target: &t}
}

// providerWith builds an accessibility provider over a scripted platform.
func providerWith(snap directorapi.AccessibilitySnapshot,
	table map[directorapi.WindowID]directorapi.TargetProvenance) *Accessibility {

	return NewAccessibility(&fakeSource{snap: snap}).
		WithTargetResolver(&fakeResolver{byWindow: table})
}

// ── the happy path ────────────────────────────────────────────────────────────

func TestAccessibilityProvesAMatchingTarget(t *testing.T) {
	expected := prov("code", 4242, 7)
	a := providerWith(snapshotWith("hwnd:100", "code", 3),
		map[directorapi.WindowID]directorapi.TargetProvenance{"hwnd:100": expected})

	got := a.ObserveTargeted(context.Background(), targetedRequest(expected, "hwnd:100"))

	if got.State != observation.StateContributed {
		t.Fatalf("state = %q, want contributed (%s)", got.State, got.Reason)
	}
	if !got.Usable() {
		t.Fatal("proven, contributing evidence was not usable")
	}
	if got.ObservedTarget != expected {
		t.Errorf("observed = %+v, want %+v", got.ObservedTarget, expected)
	}
	if len(got.Observations) != 3 {
		t.Errorf("observations = %d, want 3", len(got.Observations))
	}
	if got.Scope != directorapi.ScopeTarget {
		t.Errorf("scope = %q, want target-scoped", got.Scope)
	}
}

// ── the trap ──────────────────────────────────────────────────────────────────

// THE test for this milestone. The platform proves the walk read generation 8 while the
// request expected 7. An implementation that copies req.Target cannot fail this.
func TestObservedTargetComesFromThePlatformNotTheRequest(t *testing.T) {
	expected := prov("code", 4242, 7)
	// The bridge walked the window it was asked for — but that handle is now a
	// DIFFERENT generation, which only re-reading the platform reveals.
	a := providerWith(snapshotWith("hwnd:100", "code", 5),
		map[directorapi.WindowID]directorapi.TargetProvenance{
			"hwnd:100": prov("code", 4242, 8),
		})

	got := a.ObserveTargeted(context.Background(), targetedRequest(expected, "hwnd:100"))

	if got.ObservedTarget == got.ExpectedTarget {
		t.Fatal("ObservedTarget equals ExpectedTarget — it was copied from the request " +
			"rather than established from platform evidence")
	}
	if got.ObservedTarget.WindowGeneration != 8 {
		t.Errorf("observed generation = %d, want the platform's 8",
			got.ObservedTarget.WindowGeneration)
	}
	if got.State != observation.StateTargetChanged {
		t.Errorf("state = %q, want target_changed", got.State)
	}
	if got.TargetProven() || got.Usable() {
		t.Fatal("evidence from a replaced window was accepted")
	}
}

// The same property, stated as sensitivity: changing only the PLATFORM must change the
// outcome. The request is byte-identical across both halves.
func TestChangingOnlyThePlatformChangesTheOutcome(t *testing.T) {
	expected := prov("code", 4242, 7)
	req := targetedRequest(expected, "hwnd:100")

	same := providerWith(snapshotWith("hwnd:100", "code", 2),
		map[directorapi.WindowID]directorapi.TargetProvenance{"hwnd:100": expected})
	moved := providerWith(snapshotWith("hwnd:100", "code", 2),
		map[directorapi.WindowID]directorapi.TargetProvenance{
			"hwnd:100": prov("code", 4242, 9),
		})

	if !same.ObserveTargeted(context.Background(), req).Usable() {
		t.Fatal("an unchanged platform refused usable evidence")
	}
	if moved.ObserveTargeted(context.Background(), req).Usable() {
		t.Fatal("a changed platform still produced usable evidence — the provider is " +
			"not reading the platform at all")
	}
}

// ── the race, with a barrier ──────────────────────────────────────────────────

// generation 7 validated → snapshot begins → window replaced → snapshot returns.
//
// No sleeps: the replacement happens inside the snapshot call, which is exactly when the
// real race occurs.
func TestTargetReplacedDuringTheSnapshotIsRefused(t *testing.T) {
	expected := prov("code", 4242, 7)
	table := map[directorapi.WindowID]directorapi.TargetProvenance{"hwnd:100": expected}
	resolver := &fakeResolver{byWindow: table}

	src := &fakeSource{snap: snapshotWith("hwnd:100", "code", 9)}
	// The window is replaced WHILE the snapshot is in flight.
	src.duringFn = func() { table["hwnd:100"] = prov("code", 4242, 8) }

	a := NewAccessibility(src).WithTargetResolver(resolver)
	got := a.ObserveTargeted(context.Background(), targetedRequest(expected, "hwnd:100"))

	if got.State != observation.StateTargetChanged {
		t.Fatalf("state = %q, want target_changed (%s)", got.State, got.Reason)
	}
	if got.Usable() {
		t.Fatal("a snapshot taken across a window replacement was accepted")
	}
	if got.Reason == "" {
		t.Error("no reason given for the refusal")
	}
	// Evidence is retained for diagnostics; it is refused by Usable(), not by being
	// thrown away — a diagnostic that showed nothing could not explain the refusal.
	if len(got.Observations) == 0 {
		t.Error("observations were discarded rather than marked unusable")
	}
}

// ── recycled handle ───────────────────────────────────────────────────────────

// The same raw handle, a different owning process. This must fail independently of the
// generation check.
func TestRecycledHandleWithADifferentProcessIsRefused(t *testing.T) {
	expected := prov("code", 100, 7)
	a := providerWith(snapshotWith("hwnd:100", "code", 4),
		map[directorapi.WindowID]directorapi.TargetProvenance{
			// Same generation number, different process: a recycled handle.
			"hwnd:100": prov("code", 999, 7),
		})

	got := a.ObserveTargeted(context.Background(), targetedRequest(expected, "hwnd:100"))
	if got.Usable() {
		t.Fatal("a handle owned by a different process was accepted")
	}
}

func TestDifferentApplicationIsRefused(t *testing.T) {
	expected := prov("code", 100, 7)
	a := providerWith(snapshotWith("hwnd:100", "chrome", 4),
		map[directorapi.WindowID]directorapi.TargetProvenance{
			"hwnd:100": prov("chrome", 100, 7),
		})

	if a.ObserveTargeted(context.Background(), targetedRequest(expected, "hwnd:100")).Usable() {
		t.Fatal("evidence from a different application was accepted")
	}
}

// ── unproven provenance ───────────────────────────────────────────────────────

// The bridge fell back to a window the Director is not tracking.
func TestUnresolvableObservedWindowIsRefused(t *testing.T) {
	expected := prov("code", 4242, 7)
	a := providerWith(snapshotWith("hwnd:999", "terminal", 40),
		map[directorapi.WindowID]directorapi.TargetProvenance{"hwnd:100": expected})

	got := a.ObserveTargeted(context.Background(), targetedRequest(expected, "hwnd:100"))
	if got.State != observation.StateTargetChanged || got.Usable() {
		t.Fatalf("a fallback to an untracked window was accepted: %+v", got)
	}
}

// A bridge that cannot say what it read has established nothing.
func TestSnapshotWithNoObservedWindowIsRefused(t *testing.T) {
	expected := prov("code", 4242, 7)
	a := providerWith(snapshotWith("", "", 12),
		map[directorapi.WindowID]directorapi.TargetProvenance{"hwnd:100": expected})

	got := a.ObserveTargeted(context.Background(), targetedRequest(expected, "hwnd:100"))
	if got.State != observation.StateProvenanceMismatch || got.Usable() {
		t.Fatalf("a snapshot that named no window was accepted: %+v", got)
	}
}

// A provider with no resolver cannot prove anything, so it must not contribute to a
// targeted cycle. This is the missing-opt-in gate.
func TestWithoutAResolverATargetedCycleIsRefused(t *testing.T) {
	expected := prov("code", 4242, 7)
	a := NewAccessibility(&fakeSource{snap: snapshotWith("hwnd:100", "code", 5)})

	got := a.ObserveTargeted(context.Background(), targetedRequest(expected, "hwnd:100"))
	if got.Usable() {
		t.Fatal("a provider that cannot establish provenance contributed to a targeted cycle")
	}
	if got.State != observation.StateProvenanceMismatch {
		t.Errorf("state = %q, want provenance_mismatch", got.State)
	}
}

// ── ordinary command perception ───────────────────────────────────────────────

// An untargeted cycle pins nothing, so there is nothing to prove and nothing to refuse.
func TestUntargetedCycleStillContributes(t *testing.T) {
	a := NewAccessibility(&fakeSource{snap: snapshotWith("hwnd:100", "code", 4)})

	got := a.ObserveTargeted(context.Background(), observation.Request{})
	if !got.Usable() {
		t.Fatalf("ordinary command perception was refused: %+v", got)
	}
	if got.State != observation.StateContributed {
		t.Errorf("state = %q, want contributed", got.State)
	}
}

// ── empty versus unobservable ─────────────────────────────────────────────────

// Zero observations means different things, and the count alone cannot tell them apart.
func TestEmptyAndUnobservableAreDistinct(t *testing.T) {
	expected := prov("code", 4242, 7)
	table := map[directorapi.WindowID]directorapi.TargetProvenance{"hwnd:100": expected}

	// Read the right target, found nothing addressable.
	empty := providerWith(snapshotWith("hwnd:100", "code", 0), table).
		ObserveTargeted(context.Background(), targetedRequest(expected, "hwnd:100"))
	if empty.State != observation.StateEmpty {
		t.Errorf("state = %q, want empty", empty.State)
	}

	// The walk was cut short, so no structure was established at all.
	cut := snapshotWith("hwnd:100", "code", 0)
	cut.Partial, cut.Reason = true, "the walk hit the node cap"
	unobservable := providerWith(cut, table).
		ObserveTargeted(context.Background(), targetedRequest(expected, "hwnd:100"))
	if unobservable.State != observation.StateUnobservable {
		t.Errorf("state = %q, want unobservable", unobservable.State)
	}
}

func TestSourceFailureIsReportedAsFailed(t *testing.T) {
	a := NewAccessibility(&fakeSource{err: errors.New("bridge is not running")}).
		WithTargetResolver(&fakeResolver{})

	got := a.ObserveTargeted(context.Background(),
		targetedRequest(prov("code", 1, 7), "hwnd:100"))
	if got.State != observation.StateFailed {
		t.Fatalf("state = %q, want failed", got.State)
	}
	if got.Usable() {
		t.Error("a failed provider contributed")
	}
}
