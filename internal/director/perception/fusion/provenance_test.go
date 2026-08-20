package fusion

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The target provenance guard, at the boundary where it decides something.
//
// The unit rules live on ProviderOutcome and are tested there. What is tested HERE is the
// only thing that ultimately matters: that evidence which cannot prove it describes the
// live target does not become belief, and that its disappearance is accounted for rather
// than silent.

// targetedCycle builds a cycle pinned to a window generation, carrying one provider's
// outcome. The outcome's observations are the only route into belief, which is the property
// under test.
func targetedCycle(t *testing.T, name string, expected uint64,
	outcomes ...observation.ProviderOutcome) observation.Cycle {

	t.Helper()
	c := cycleOf(t, name)
	c.Request.Target = &directorapi.TargetProvenance{
		Application: "notepad", ProcessID: 4242, WindowGeneration: expected,
	}
	c.Outcomes = outcomes
	return c
}

// outcomeOver wraps a cycle's own observations in an outcome claiming a given generation.
func outcomeOver(c observation.Cycle, observedGen uint64) observation.ProviderOutcome {
	return observation.ProviderOutcome{
		Source: directorapi.SourceAccessibility,
		State:  observation.StateContributed,
		Scope:  directorapi.ScopeTarget,
		ExpectedTarget: directorapi.TargetProvenance{
			Application: "notepad", ProcessID: 4242, WindowGeneration: 8,
		},
		ObservedTarget: directorapi.TargetProvenance{
			Application: "notepad", ProcessID: 4242, WindowGeneration: observedGen,
		},
		Observations: c.Observations,
	}
}

// ── the race the guard exists for ─────────────────────────────────────────────

func TestEvidenceFromAReplacedWindowNeverBecomesBelief(t *testing.T) {
	// generation 8 validated → provider begins → the window is replaced → the provider
	// returns evidence about generation 7. The request was correct when it was made, so
	// nothing in the request can reveal this. Only the comparison can.
	base := cycleOf(t, "save-dialog")
	if len(observation.Elements(base.Observations)) == 0 {
		t.Fatal("the fixture carries no element observations; the test proves nothing")
	}
	cycle := targetedCycle(t, "save-dialog", 8, outcomeOver(base, 7))

	w, report, err := NewEngine().Fuse(cycle)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if len(w.Elements) != 0 {
		t.Fatalf("%d elements were believed from evidence about a window that no longer "+
			"exists", len(w.Elements))
	}
	// And it must not look like an empty desktop.
	if len(w.Degraded) == 0 {
		t.Error("the world reports no degradation; a silently empty world reads as " +
			"'the button is not there' when the truth is 'I could not attribute what I saw'")
	}
	if report.Provenance.Refused != len(base.Observations) {
		t.Errorf("report accounts for %d refused observations, want %d",
			report.Provenance.Refused, len(base.Observations))
	}
	if len(report.Provenance.Providers) != 1 {
		t.Fatalf("%d refusals named in the report", len(report.Provenance.Providers))
	}
	r := report.Provenance.Providers[0]
	if r.Expected.WindowGeneration != 8 || r.Observed.WindowGeneration != 7 {
		t.Errorf("the report shows expected %d / observed %d; a reader needs both numbers "+
			"to believe the verdict", r.Expected.WindowGeneration, r.Observed.WindowGeneration)
	}
	if r.Reason == "" {
		t.Error("the refusal carries no reason")
	}
}

func TestMatchingProvenanceIsBelievedNormally(t *testing.T) {
	// The other half, and the one that keeps the guard honest: a guard that refused
	// everything would pass the test above and be useless.
	base := cycleOf(t, "save-dialog")
	want := len(observation.Elements(base.Observations))
	cycle := targetedCycle(t, "save-dialog", 8, outcomeOver(base, 8))

	w, report, err := NewEngine().Fuse(cycle)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if len(w.Elements) != want {
		t.Fatalf("%d elements from %d proven element observations", len(w.Elements), want)
	}
	if report.Provenance.Refused != 0 {
		t.Errorf("%d observations refused from a provider that proved its target",
			report.Provenance.Refused)
	}
	if !report.Provenance.Targeted || report.Provenance.Generation != 8 {
		t.Errorf("the report does not record that the guard ran against generation 8: %+v",
			report.Provenance)
	}
}

// ── fail-safe ─────────────────────────────────────────────────────────────────

func TestATargetedCycleWithNoOutcomesBelievesNothing(t *testing.T) {
	// The fail-safe, and the reason Admitted does not fall back to the flat observation
	// list. A path that pins a target but collects no outcomes has proven nothing about
	// anything, and must not be treated as having proven everything.
	cycle := cycleOf(t, "save-dialog")
	cycle.Request.Target = &directorapi.TargetProvenance{
		Application: "notepad", WindowGeneration: 3,
	}

	w, _, err := NewEngine().Fuse(cycle)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if len(w.Elements) != 0 {
		t.Fatalf("%d elements were believed on a targeted cycle where no provider "+
			"established what it observed", len(w.Elements))
	}
}

func TestAnUntargetedCycleIsUnaffectedByTheGuard(t *testing.T) {
	// Every ordinary command runs this path. If the guard reached it, the Director would
	// stop being able to see anything at all.
	cycle := cycleOf(t, "save-dialog")
	want := len(observation.Elements(cycle.Observations))

	w, report, err := NewEngine().Fuse(cycle)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if len(w.Elements) != want {
		t.Fatalf("%d elements on an untargeted cycle, want %d — the guard is applying "+
			"where there is no target to be stale relative to", len(w.Elements), want)
	}
	if report.Provenance.Targeted {
		t.Error("an untargeted cycle reports itself as guarded")
	}
}

func TestGlobalEvidenceSurvivesAGuardedCycle(t *testing.T) {
	// Monitor topology does not belong to a window and cannot go stale when one is
	// replaced. A guard that discarded it would break the desktop frame for every
	// targeted session.
	base := cycleOf(t, "save-dialog")
	global := observation.ProviderOutcome{
		Source:       directorapi.SourceWindowSystem,
		State:        observation.StateContributed,
		Scope:        directorapi.ScopeGlobal,
		Observations: base.Observations,
	}
	cycle := targetedCycle(t, "save-dialog", 8, global)

	w, _, err := NewEngine().Fuse(cycle)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if len(w.Elements) == 0 {
		t.Fatal("global evidence was refused by the target guard; it makes no window " +
			"claim that could be wrong")
	}
}

// ── the report accounts for itself ────────────────────────────────────────────

func TestRefusedEvidenceIsNotCountedAsFused(t *testing.T) {
	// A report that counted refused evidence as input would make the guard invisible in
	// exactly the numbers somebody would check it with.
	base := cycleOf(t, "save-dialog")
	cycle := targetedCycle(t, "save-dialog", 8, outcomeOver(base, 7))

	_, report, err := NewEngine().Fuse(cycle)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if report.ObservationCount != 0 {
		t.Errorf("the report claims %d observations went in; all of them were refused",
			report.ObservationCount)
	}
	if n := len(report.BySource); n != 0 {
		t.Errorf("BySource attributes %d sources on a cycle where nothing was admitted", n)
	}
}

func TestAProviderThatContributedNothingIsNotReportedAsRefused(t *testing.T) {
	// Signal-to-noise. A provider that was unavailable had no evidence to lose, and
	// listing it beside a genuine staleness refusal would bury the one line that matters.
	cycle := targetedCycle(t, "save-dialog", 8, observation.ProviderOutcome{
		Source: directorapi.SourceOCR,
		State:  observation.StateUnavailable,
		Reason: "no OCR engine is installed",
	})

	_, report, err := NewEngine().Fuse(cycle)
	if err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	if len(report.Provenance.Providers) != 0 {
		t.Errorf("a provider with no evidence was reported as a provenance refusal: %+v",
			report.Provenance.Providers)
	}
}
