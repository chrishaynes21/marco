package observesession_test

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// Screen-state segmentation, entered through the production path only.
//
// The same discipline as TestTheSessionResultCarriesShadowFindings, for the same reason: this
// subsystem has already shipped one mechanism that existed, was unit-tested, and was never
// called. A test that drove ShadowTracker directly would pass just as happily with the
// segmentation call deleted from ShadowTotals.Add, and the CLI would quietly report a session
// with no states as a session with nothing to say.
//
// So this enters at the runner, supplies samples the way a real sampler does, and asserts the
// terminal Result carries state-local evidence.

// alternatingSampler shows a four-row menu on odd samples and a sparse gameplay screen on
// even ones — the shape of the live Rocket League session, which is the case the whole
// state-relative correction exists for.
type alternatingSampler struct{ calls int }

func (s *alternatingSampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	s.calls++
	hud := observe.ShadowRegion{
		Role: "icon", Nameable: false, Confidence: 0.5,
		Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}
	regions := []observe.ShadowRegion{hud}
	if s.calls%2 == 1 {
		for i, y := range []float64{0.437, 0.480, 0.520, 0.562} {
			regions = append(regions, observe.ShadowRegion{
				Role: "button", Nameable: true, Confidence: 0.4 + float64(i)*0.01,
				Region: observe.Region{X: 0.414, Y: y, Width: 0.172, Height: 0.036},
			})
		}
	}
	return observe.Sample{
		Shadow: &observe.ShadowSample{
			Detector: "screenparser", Ran: true, TargetProven: true,
			Detections: len(regions), Regions: regions, LatencyMS: 860,
			Roles: map[string]int{"button": len(regions) - 1, "icon": 1},
		},
	}, nil
}

// THE test. Deleting the segmentation call from ShadowTotals.Add must fail this.
func TestTheProductionSessionPathSegmentsScreenStates(t *testing.T) {
	clock := newClock()
	got, err := observesession.New(clock, &steadyTarget{ref: ref(1)},
		&alternatingSampler{}, &recordingEvents{}).
		Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	sh := got.Stats.Shadow
	if !sh.Observed() {
		t.Fatal("no shadow observations; this test cannot say anything about segmentation")
	}

	if len(sh.States) < 2 {
		t.Fatalf("the session was segmented into %d screen state(s); a run alternating "+
			"between a four-row menu and a sparse screen must produce at least two. "+
			"Segmentation is not reachable from the production session path", len(sh.States))
	}
	if len(sh.Transitions) == 0 {
		t.Error("no state transitions recorded across an alternating session")
	}

	// The menu rows: state-locally perfect, globally about half. Both must be true, and the
	// difference between them is the entire milestone.
	var rows int
	for _, tr := range sh.Tracks {
		if tr.Role != "button" {
			continue
		}
		rows++
		st, ok := tr.PrimaryState()
		if !ok {
			t.Fatalf("button track %s carries no state evidence", tr.ID)
		}
		if st.PresenceRatio() != 1.0 {
			t.Errorf("%s: state-local presence %.2f (%d/%d), want 1.00 — the row was "+
				"detected in every inference its own screen was on screen",
				tr.ID, st.PresenceRatio(), st.Seen, st.Eligible)
		}
		if st.Shape != observe.ShapePersistent {
			t.Errorf("%s: state-local shape %q, want %q", tr.ID, st.Shape,
				observe.ShapePersistent)
		}
		if st.Eligible >= tr.Eligible {
			t.Errorf("%s: state-local eligible %d is not fewer than global %d — "+
				"eligibility is still counting every valid inference",
				tr.ID, st.Eligible, tr.Eligible)
		}
		if tr.PresenceRatio() >= 0.80 {
			t.Errorf("%s: global presence %.2f — this assertion only means something if "+
				"the global answer really is the unfavourable one", tr.ID, tr.PresenceRatio())
		}
	}
	if rows != 4 {
		t.Fatalf("%d button tracks, want 4 — one per menu row", rows)
	}

	// The four rows arrive and leave together, which is what makes them a structure.
	if len(sh.Groups) == 0 {
		t.Fatal("no structural group discovered from four rows that co-occur in one state")
	}
	g := sh.Groups[0]
	if len(g.Members) != 4 {
		t.Errorf("group %s has %d members, want 4", g.ID, len(g.Members))
	}
	if g.Uniformity < 0.80 {
		t.Errorf("group %s uniformity %.2f — an evenly stacked column should score high",
			g.ID, g.Uniformity)
	}
}

// Part 33. State segmentation is shadow-only and reaches nothing.
//
// The mutation guard the whole subsystem rests on. State IDs, state history and track-state
// association are experimental evidence; if any of it could move authoritative analysis, the
// experiment would have become the detector — which has already happened once in this project,
// silently, and cost a milestone.
func TestScreenStatesReachNothingAuthoritative(t *testing.T) {
	run := func(s observesession.Sampler) observesession.Result {
		t.Helper()
		got, err := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, s,
			&recordingEvents{}).Run(context.Background(), config())
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return got
	}

	withStates := run(&alternatingSampler{})
	withoutShadow := run(&countingSampler{})

	if len(withStates.Stats.Shadow.States) < 2 {
		t.Fatal("the shadow session produced no states; nothing to prove isolation about")
	}
	if withoutShadow.Stats.Shadow.Observed() {
		t.Fatal("the control session carried shadow evidence")
	}

	// A session whose shadow provider discovered a rich state model must not have produced
	// different authoritative findings from one that ran no experiment at all.
	if len(withStates.Findings.Stable) != 0 {
		t.Errorf("shadow-derived structure reached the authoritative findings: %d stable "+
			"elements from a sampler that admitted no authoritative evidence",
			len(withStates.Findings.Stable))
	}
	if withStates.Stats.ProvenanceRefusals != withoutShadow.Stats.ProvenanceRefusals {
		t.Error("the presence of a shadow state model changed the provenance verdict")
	}
}
