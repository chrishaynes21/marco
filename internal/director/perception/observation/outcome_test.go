package observation

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The guard exists to catch evidence from a window that has since been replaced.
//
// The abandoned design could not: it stamped observations with the request's target and
// compared that against the request's own expectation, so both sides were copies of one
// value and matched by construction. These tests are written so that implementation would
// fail them.

func target(app string, pid uint32, gen uint64) directorapi.TargetProvenance {
	return directorapi.TargetProvenance{
		Application: app, ProcessID: pid, WindowGeneration: gen,
	}
}

func outcome(expected, observed directorapi.TargetProvenance) ProviderOutcome {
	return ProviderOutcome{
		Source: directorapi.SourceAccessibility, State: StateContributed,
		ExpectedTarget: expected, ObservedTarget: observed,
	}
}

// ── the same-request trap ─────────────────────────────────────────────────────

// THE regression test for the abandoned design.
//
// An implementation that sets ObservedTarget = ExpectedTarget passes every other test in
// this file. This one constructs the race directly: the expectation is generation 7 and the
// platform proves the evidence came from generation 8.
func TestEvidenceFromAReplacedWindowIsRefused(t *testing.T) {
	expected := target("code", 4242, 7)
	// What the provider actually established, from its own post-collection evidence.
	observed := target("code", 4242, 8)

	o := outcome(expected, observed)
	if o.TargetProven() {
		t.Fatal("evidence from generation 8 was accepted against an expectation of " +
			"generation 7 — the guard is comparing a value with itself")
	}
	if o.Usable() {
		t.Error("stale evidence would enter clustering")
	}
}

// The trap, stated as the property rather than one instance: for ANY expectation, copying
// it into the observed field must not be what makes the guard pass.
func TestCopyingTheExpectationProvesNothing(t *testing.T) {
	for _, gen := range []uint64{1, 7, 8, 4242} {
		expected := target("code", 100, gen)

		// The abandoned implementation, in one line.
		copied := outcome(expected, expected)
		if !copied.TargetProven() {
			t.Fatalf("gen %d: a copied target should still pass — this test is not "+
				"the one that catches it", gen)
		}
		// ...which is exactly why the trap above must exist. Assert the guard is
		// SENSITIVE to a difference, so a copy is not indistinguishable from proof.
		drifted := outcome(expected, target("code", 100, gen+1))
		if drifted.TargetProven() {
			t.Fatalf("gen %d: the guard cannot tell a replaced window from the "+
				"expected one", gen)
		}
	}
}

// ── unknown provenance ────────────────────────────────────────────────────────

// "Could not establish provenance" is not "probably fine". Otherwise any provider could
// bypass the guard by declining to answer.
func TestUnknownObservedTargetIsRefused(t *testing.T) {
	o := outcome(target("code", 4242, 7), directorapi.TargetProvenance{})
	if o.TargetProven() {
		t.Fatal("a provider that established nothing was treated as having agreed")
	}
}

// An untargeted cycle has no pinned target, so there is nothing to be stale relative to.
func TestUntargetedCycleAppliesNoGuard(t *testing.T) {
	o := outcome(directorapi.TargetProvenance{}, directorapi.TargetProvenance{})
	if !o.TargetProven() {
		t.Error("an ordinary command cycle was refused for having no target")
	}
}

// ── global scope ──────────────────────────────────────────────────────────────

// Monitor topology does not belong to a window and cannot go stale when one is replaced.
func TestGlobalOutcomesAreExemt(t *testing.T) {
	o := outcome(target("code", 4242, 7), directorapi.TargetProvenance{})
	o.Scope = directorapi.ScopeGlobal
	if !o.TargetProven() {
		t.Fatal("global evidence was refused for lacking a window generation")
	}
}

// Global scope must be DECLARED. Inferring it from missing provenance is how a
// target-scoped provider that failed to establish its target would slip through.
func TestMissingScopeIsTargetScopedNotGlobal(t *testing.T) {
	o := outcome(target("code", 4242, 7), directorapi.TargetProvenance{})
	if o.Global() {
		t.Fatal("an undeclared scope defaulted to global")
	}
	if o.TargetProven() {
		t.Error("an undeclared scope bypassed the guard")
	}
}

// ── match semantics ───────────────────────────────────────────────────────────

func TestGenerationMustAgree(t *testing.T) {
	if target("code", 1, 7).Matches(target("code", 1, 8)) {
		t.Error("different generations matched")
	}
	if !target("code", 1, 7).Matches(target("code", 1, 7)) {
		t.Error("identical provenance did not match")
	}
}

// A recycled handle: same generation number would be a coincidence, but the process
// differs and that is decisive.
func TestKnownProcessMismatchIsRefused(t *testing.T) {
	if target("code", 100, 7).Matches(target("code", 200, 7)) {
		t.Fatal("evidence from a different process was accepted — this is the " +
			"recycled-handle case")
	}
}

// A provider that cannot see a process id has not contradicted anything by omitting it.
func TestUnknownProcessIsNotAContradiction(t *testing.T) {
	if !target("code", 0, 7).Matches(target("code", 100, 7)) {
		t.Error("an omitted process id was treated as a disagreement")
	}
	if !target("code", 100, 7).Matches(target("code", 0, 7)) {
		t.Error("an omitted expectation process id was treated as a disagreement")
	}
}

func TestApplicationMismatchIsRefused(t *testing.T) {
	if target("chrome", 100, 7).Matches(target("code", 100, 7)) {
		t.Error("evidence from a different application was accepted")
	}
	// Case is not meaningful in an application key.
	if !target("Code", 100, 7).Matches(target("code", 100, 7)) {
		t.Error("application comparison was case-sensitive")
	}
}

// ── state semantics ───────────────────────────────────────────────────────────

// Zero observations means three different things, and they must not collapse.
func TestNonContributingStatesDoNotOfferEvidence(t *testing.T) {
	for _, s := range []ProviderState{
		StateEmpty, StateUnobservable, StateUnavailable, StateTargetChanged,
		StateProvenanceMismatch, StateFailed, StateTimedOut,
	} {
		if s.Contributing() {
			t.Errorf("%s reported itself as contributing evidence", s)
		}
	}
	if !StateContributed.Contributing() {
		t.Error("contributed does not contribute")
	}
}

// A provider whose target checks out but which failed still contributes nothing.
func TestProvenGuardDoesNotRescueAFailedProvider(t *testing.T) {
	o := outcome(target("code", 1, 7), target("code", 1, 7))
	o.State = StateFailed
	if !o.TargetProven() {
		t.Error("provenance should still be proven; the failure is separate")
	}
	if o.Usable() {
		t.Fatal("a failed provider's evidence was taken because its target matched")
	}
}

// And the converse: a provider that contributed but cannot prove its target is refused.
func TestContributingDoesNotRescueUnprovenProvenance(t *testing.T) {
	o := outcome(target("code", 1, 7), target("code", 1, 8))
	if o.State != StateContributed {
		t.Fatal("precondition")
	}
	if o.Usable() {
		t.Fatal("contributed evidence with a mismatched target was taken")
	}
}
