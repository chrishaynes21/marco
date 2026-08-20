package providers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Shadow perception may watch. It may not decide.
//
// These tests are the whole safety argument for running an experimental detector beside live
// perception. They are written against the STRUCTURE rather than against behaviour: the claim
// is not "fusion currently ignores shadow evidence", it is "fusion is never handed shadow
// evidence", which is a property no future edit to fusion can quietly undo.

// probe is a provider that reports whatever it is told to.
type probe struct {
	name   string
	obs    []observation.Observation
	err    error
	target directorapi.TargetProvenance
}

func (p *probe) Name() string                        { return p.name }
func (p *probe) Sources() []observation.Source       { return []observation.Source{directorapi.SourceVision} }
func (p *probe) Contributes(observation.Source) bool { return true }

func (p *probe) Observe(context.Context, observation.Request) ([]observation.Observation, error) {
	return p.obs, p.err
}

func (p *probe) ObserveTargeted(_ context.Context,
	req observation.Request) observation.ProviderOutcome {

	out := observation.ProviderOutcome{
		Source:         directorapi.SourceVision,
		State:          observation.StateContributed,
		Observations:   p.obs,
		ObservedTarget: p.target,
	}
	if req.Target != nil {
		out.ExpectedTarget = *req.Target
	}
	if len(p.obs) == 0 {
		out.State = observation.StateEmpty
	}
	if p.err != nil {
		out.State = observation.StateFailed
		out.Reason = p.err.Error()
	}
	return out
}

// shadowProbe is a probe that has declared itself experimental.
type shadowProbe struct{ probe }

func (s *shadowProbe) ShadowOnly() {}

var _ observation.ShadowProvider = (*shadowProbe)(nil)

func obs(id string) observation.Observation {
	return observation.NewElement(directorapi.Observation{
		ID:     directorapi.ObservationID(id),
		Source: directorapi.SourceVision,
		Label:  id,
	})
}

func targetedRequest(gen uint64) observation.Request {
	t := directorapi.TargetProvenance{Application: "rocketleague", ProcessID: 4242, WindowGeneration: gen}
	return observation.Request{Target: &t, Include: []observation.Source{directorapi.SourceVision}}
}

// Part 20. The load-bearing one: shadow evidence never reaches the collection fusion reads.
func TestShadowEvidenceNeverReachesAdmittedObservations(t *testing.T) {
	target := directorapi.TargetProvenance{Application: "rocketleague", ProcessID: 4242, WindowGeneration: 8}
	authoritative := &probe{name: "authoritative", obs: []observation.Observation{obs("real")},
		target: target}
	shadow := &shadowProbe{probe{name: "screenparser",
		obs: []observation.Observation{obs("shadow-a"), obs("shadow-b")}, target: target}}

	c := providers.NewCollector(authoritative, shadow)
	cycle := c.Collect(context.Background(), targetedRequest(8))

	admitted, _ := cycle.Admitted()
	if len(admitted) != 1 || admitted[0].ID() != "real" {
		t.Fatalf("Admitted() returned %d observations %v; shadow evidence must not be "+
			"admissible under any circumstances", len(admitted), names(admitted))
	}
	// Not merely absent from Admitted — absent from the fields Admitted READS. This is the
	// difference between fusion choosing to ignore it and fusion never seeing it.
	if len(cycle.Observations) != 1 {
		t.Errorf("Cycle.Observations holds %d entries %v; shadow evidence leaked into the "+
			"authoritative evidence record", len(cycle.Observations), names(cycle.Observations))
	}
	for _, o := range cycle.Outcomes {
		if o.Source == directorapi.SourceVision && len(o.Observations) == 2 {
			t.Error("a shadow outcome appeared in Cycle.Outcomes, which Admitted() iterates")
		}
	}
	if len(cycle.Shadow) != 1 || len(cycle.Shadow[0].Observations) != 2 {
		t.Fatalf("shadow evidence was not recorded: %d outcomes", len(cycle.Shadow))
	}
}

// Changing what the shadow provider reports must change nothing that is believed.
//
// The experiment's whole premise is that its output varies; this proves the variation is
// confined. Same authoritative evidence, wildly different shadow evidence, identical admitted
// result — asserted by comparison rather than by inspection, so it holds for any future
// definition of "admitted".
func TestShadowOutputCannotChangeWhatIsBelieved(t *testing.T) {
	target := directorapi.TargetProvenance{Application: "rocketleague", ProcessID: 4242, WindowGeneration: 8}
	run := func(shadowObs []observation.Observation, shadowErr error) []observation.Observation {
		authoritative := &probe{name: "authoritative",
			obs: []observation.Observation{obs("real")}, target: target}
		shadow := &shadowProbe{probe{name: "screenparser",
			obs: shadowObs, err: shadowErr, target: target}}
		cycle := providers.NewCollector(authoritative, shadow).
			Collect(context.Background(), targetedRequest(8))
		admitted, _ := cycle.Admitted()
		return admitted
	}

	base := run(nil, nil)
	loud := run([]observation.Observation{obs("a"), obs("b"), obs("c")}, nil)
	broken := run(nil, errors.New("the ONNX runtime is not installed"))

	if len(base) != len(loud) || len(base) != len(broken) {
		t.Fatalf("admitted evidence changed with shadow output: none=%d loud=%d broken=%d",
			len(base), len(loud), len(broken))
	}
	for i := range base {
		if base[i].ID() != loud[i].ID() ||
			base[i].ID() != broken[i].ID() {
			t.Fatalf("admitted evidence differed at %d", i)
		}
	}
}

// A shadow provider that fails must not degrade the world.
//
// Suppressing belief is influence. A detector that could veto perception by crashing would be
// authoritative in the only direction that matters for safety.
func TestAFailingShadowProviderDoesNotDegradeTheWorld(t *testing.T) {
	target := directorapi.TargetProvenance{Application: "rocketleague", ProcessID: 4242, WindowGeneration: 8}
	authoritative := &probe{name: "authoritative",
		obs: []observation.Observation{obs("real")}, target: target}
	shadow := &shadowProbe{probe{name: "screenparser",
		err: errors.New("model failed to load"), target: target}}

	cycle := providers.NewCollector(authoritative, shadow).
		Collect(context.Background(), targetedRequest(8))

	if len(cycle.Failures) != 0 {
		t.Errorf("a shadow failure reached Cycle.Failures as %v; fusion carries failures "+
			"into the world's Degraded list, so this would let an experiment suppress belief",
			cycle.Failures)
	}
	// Diagnosable all the same — the reason must survive somewhere.
	if len(cycle.Shadow) != 1 || cycle.Shadow[0].Reason == "" {
		t.Error("the shadow failure was silently discarded; an experiment that cannot say " +
			"why it produced nothing cannot be measured")
	}
}

// Part 22. A shadow provider that observed a different window generation is UNCOMPARABLE,
// not a disagreement about UI. Provenance is not waived for being experimental.
func TestShadowProvenanceIsNotWaived(t *testing.T) {
	stale := directorapi.TargetProvenance{Application: "rocketleague", ProcessID: 4242, WindowGeneration: 7}
	shadow := &shadowProbe{probe{name: "screenparser",
		obs: []observation.Observation{obs("menu")}, target: stale}}

	cycle := providers.NewCollector(shadow).
		Collect(context.Background(), targetedRequest(8)) // authoritative is on generation 8

	if len(cycle.Shadow) != 1 {
		t.Fatalf("expected one shadow outcome, got %d", len(cycle.Shadow))
	}
	out := cycle.Shadow[0]
	if out.TargetProven() {
		t.Error("an outcome observed on generation 7 was reported as proving generation 8")
	}
	if out.Usable() {
		t.Error("a shadow outcome from a superseded generation reported itself usable")
	}
	if out.ExpectedTarget.WindowGeneration != 8 || out.ObservedTarget.WindowGeneration != 7 {
		t.Errorf("expected/observed collapsed: %d vs %d — the two must stay separable or "+
			"the mismatch cannot be detected at all",
			out.ExpectedTarget.WindowGeneration, out.ObservedTarget.WindowGeneration)
	}
}

// A shadow provider is still asked to prove its target — being experimental is not a reason
// to collect evidence nobody can trust.
func TestShadowProvidersAreStillAskedToProveTheirTarget(t *testing.T) {
	target := directorapi.TargetProvenance{Application: "rocketleague", ProcessID: 4242, WindowGeneration: 8}
	shadow := &shadowProbe{probe{name: "screenparser",
		obs: []observation.Observation{obs("menu")}, target: target}}

	cycle := providers.NewCollector(shadow).Collect(context.Background(), targetedRequest(8))
	if len(cycle.Shadow) != 1 {
		t.Fatalf("expected one shadow outcome, got %d", len(cycle.Shadow))
	}
	if !cycle.Shadow[0].TargetProven() {
		t.Fatal("a shadow provider that observed the pinned generation was not credited " +
			"with proving it; comparisons against belief would be uncomparable forever")
	}
}

func names(obs []observation.Observation) []string {
	out := make([]string, 0, len(obs))
	for _, o := range obs {
		out = append(out, string(o.ID()))
	}
	return out
}
