package providers

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The collector is where a provider is ASKED to prove what it observed.
//
// The unit rules are tested on the outcome and the guard is tested in fusion. What is left,
// and what these cover, is the wiring between them — which is exactly where this mechanism
// spent its previous life being complete and inert: every type existed, every rule was
// tested, and nothing ever called any of it.

// plainProvider implements Provider and nothing more. It cannot prove anything, which is the
// point of it.
type plainProvider struct {
	name string
	obs  []observation.Observation
	err  error
}

func (p *plainProvider) Name() string { return p.name }
func (p *plainProvider) Sources() []observation.Source {
	return []observation.Source{directorapi.SourceOCR}
}
func (p *plainProvider) Observe(context.Context, observation.Request) ([]observation.Observation, error) {
	return p.obs, p.err
}

func someObservations(n int) []observation.Observation {
	var out []observation.Observation
	for i := range n {
		out = append(out, observation.NewElement(directorapi.Observation{
			ID:     directorapi.ObservationID(string(rune('a' + i))),
			Source: directorapi.SourceOCR,
			Role:   directorapi.RoleButton,
		}))
	}
	return out
}

func pinnedTo(gen uint64) observation.Request {
	return observation.Request{
		Target: &directorapi.TargetProvenance{
			Application: "notepad", ProcessID: 7, WindowGeneration: gen,
		},
	}
}

// ── every provider gets an outcome ────────────────────────────────────────────

func TestEveryProviderThatRunsProducesAnOutcome(t *testing.T) {
	// The choke point. If a provider can run without producing an outcome, then on a
	// targeted cycle its evidence either bypasses the guard or vanishes unexplained, and
	// which of those it is would depend on code elsewhere.
	c := NewCollector(
		NewAccessibility(&fakeAccessibility{snap: desktop(t, "save-dialog")}),
		&plainProvider{name: "plain", obs: someObservations(2)},
	)
	cycle := c.Collect(context.Background(), observation.Request{})

	if len(cycle.Outcomes) != 2 {
		t.Fatalf("%d outcomes for 2 providers that ran", len(cycle.Outcomes))
	}
	for _, o := range cycle.Outcomes {
		if o.Source == "" {
			t.Errorf("an outcome names no source: %+v", o)
		}
		if o.State == "" {
			t.Errorf("outcome for %s has no state", o.Source)
		}
	}
}

func TestAProviderThatCannotProveItsTargetIsRefusedOnATargetedCycle(t *testing.T) {
	// A plain Provider has no way to say which window it read. On an ordinary cycle that
	// costs it nothing; on a pinned one it must not contribute, because "I did not say"
	// is not "I agree".
	c := NewCollector(&plainProvider{name: "plain", obs: someObservations(3)})
	cycle := c.Collect(context.Background(), pinnedTo(5))

	admitted, refused := cycle.Admitted()
	if len(admitted) != 0 {
		t.Fatalf("%d observations admitted from a provider that established no target",
			len(admitted))
	}
	if len(refused) != 1 {
		t.Fatalf("%d refusals recorded, want the plain provider's evidence accounted for",
			len(refused))
	}
}

func TestTheSameProviderContributesFreelyOnAnUntargetedCycle(t *testing.T) {
	// The other side of the same rule, and the reason the guard is not simply "refuse
	// providers that cannot prove things": on an ordinary command there is nothing to
	// prove, and requiring proof would disable OCR and every future simple source.
	c := NewCollector(&plainProvider{name: "plain", obs: someObservations(3)})
	cycle := c.Collect(context.Background(), observation.Request{})

	admitted, refused := cycle.Admitted()
	if len(admitted) != 3 {
		t.Fatalf("%d observations admitted on an untargeted cycle, want 3", len(admitted))
	}
	if len(refused) != 0 {
		t.Errorf("%d refusals on a cycle with no target to be stale relative to", len(refused))
	}
}

// ── the race, through the real collector ──────────────────────────────────────

func TestAWindowReplacedMidCollectionDoesNotReachTheAdmittedSet(t *testing.T) {
	// The whole mechanism, end to end, at the only moment it matters.
	//
	//	generation 5 validated → the walk begins → the window is replaced
	//	→ generation 6 is current → the walk returns generation-5 evidence
	//
	// The request was correct when it was made. Only the post-collection comparison can
	// reveal this, which is why the resolver is consulted after the snapshot returns.
	resolver := &fakeResolver{byWindow: map[directorapi.WindowID]directorapi.TargetProvenance{
		"win:1": prov("notepad", 7, 5),
	}}
	src := &fakeSource{
		snap: snapshotWith("win:1", "notepad", 4),
		duringFn: func() {
			// The window is replaced while the walk is in flight.
			resolver.byWindow["win:1"] = prov("notepad", 7, 6)
		},
	}

	c := NewCollector(NewAccessibility(src).WithTargetResolver(resolver))
	cycle := c.Collect(context.Background(), pinnedTo(5))

	admitted, refused := cycle.Admitted()
	if len(admitted) != 0 {
		t.Fatalf("%d observations from a window replaced mid-walk were admitted", len(admitted))
	}
	if len(refused) != 1 {
		t.Fatalf("%d refusals, want the stale walk accounted for", len(refused))
	}
	if got := refused[0].ObservedTarget.WindowGeneration; got != 6 {
		t.Errorf("observed generation %d, want 6 — the observed target must come from the "+
			"platform after the walk, not from the request", got)
	}
	if refused[0].ExpectedTarget.WindowGeneration != 5 {
		t.Errorf("expected generation %d, want the 5 the request pinned",
			refused[0].ExpectedTarget.WindowGeneration)
	}
	// Evidence is retained for diagnostics even though it may not be believed. Discarding
	// it would leave nobody able to say what the stale window had looked like.
	if len(refused[0].Observations) == 0 {
		t.Error("the refused evidence was discarded rather than quarantined")
	}
}

func TestAnUnreplacedWindowContributesThroughTheSamePath(t *testing.T) {
	// The control. Without it the test above passes under an implementation that refuses
	// everything, which would be a far worse bug than the one being prevented.
	resolver := &fakeResolver{byWindow: map[directorapi.WindowID]directorapi.TargetProvenance{
		"win:1": prov("notepad", 7, 5),
	}}
	c := NewCollector(NewAccessibility(&fakeSource{
		snap: snapshotWith("win:1", "notepad", 4),
	}).WithTargetResolver(resolver))

	cycle := c.Collect(context.Background(), pinnedTo(5))

	admitted, refused := cycle.Admitted()
	if len(refused) != 0 {
		t.Fatalf("%d refusals on a cycle where nothing changed: %+v", len(refused), refused)
	}
	if len(admitted) != 4 {
		t.Fatalf("%d observations admitted, want the 4 the walk produced", len(admitted))
	}
}

// ── degradation is not lost on the targeted path ──────────────────────────────

func TestATruncatedWalkStillDegradesTheCycleWhenTargeted(t *testing.T) {
	// Regression. Routing providers through ObserveTargeted moved the partial-walk
	// signal off the error return it used to travel on, and a truncated walk briefly
	// became indistinguishable from a complete one — which is precisely the confusion
	// "I could not read this application" versus "the button is not there".
	snap := snapshotWith("win:1", "notepad", 2)
	snap.Partial = true
	snap.Reason = "node cap reached"

	resolver := &fakeResolver{byWindow: map[directorapi.WindowID]directorapi.TargetProvenance{
		"win:1": prov("notepad", 7, 5),
	}}
	c := NewCollector(NewAccessibility(&fakeSource{snap: snap}).WithTargetResolver(resolver))
	cycle := c.Collect(context.Background(), pinnedTo(5))

	if len(cycle.Failures) != 1 {
		t.Fatalf("%d failures, want the truncation recorded even on a targeted cycle",
			len(cycle.Failures))
	}
	if cycle.Failures[0].Reason != "node cap reached" {
		t.Errorf("reason = %q", cycle.Failures[0].Reason)
	}
	// And the evidence is still real and still usable.
	admitted, _ := cycle.Admitted()
	if len(admitted) != 2 {
		t.Errorf("%d observations admitted; a partial walk's evidence is real", len(admitted))
	}
}

func TestAProvenanceRefusalIsNotAlsoRecordedAsACollectorFailure(t *testing.T) {
	// Admissibility is fusion's to decide and to report. If the collector recorded it as
	// well, the same refusal would appear twice in the world's Degraded list, from two
	// components that would then have to be kept in agreement forever.
	resolver := &fakeResolver{byWindow: map[directorapi.WindowID]directorapi.TargetProvenance{
		"win:1": prov("notepad", 7, 9),
	}}
	c := NewCollector(NewAccessibility(&fakeSource{
		snap: snapshotWith("win:1", "notepad", 3),
	}).WithTargetResolver(resolver))

	cycle := c.Collect(context.Background(), pinnedTo(5))

	if len(cycle.Failures) != 0 {
		t.Errorf("the collector recorded a provenance refusal as a failure: %+v",
			cycle.Failures)
	}
	if _, refused := cycle.Admitted(); len(refused) != 1 {
		t.Error("the refusal was not recorded anywhere")
	}
}
