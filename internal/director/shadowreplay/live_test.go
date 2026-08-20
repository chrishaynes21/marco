package shadowreplay_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/shadowreplay"
)

func slot(n int, at int64, outcome string, regions ...observe.ShadowRegion) shadowreplay.Slot {
	return shadowreplay.Slot{N: n, AtMS: at, Outcome: outcome, Regions: regions}
}

// Part 7's discriminator, and the reason clustering exists at all.
//
// "26 detections became 24 tracks" has two opposite readings. Either the same four rows were
// seen repeatedly and tracking lost them, or the detector produced twenty-four genuinely
// different boxes and tracking was right. The clustering must tell those apart, or the live
// analysis cannot conclude anything.
func TestClusteringSeparatesLostIdentityFromGenuinelyDifferentBoxes(t *testing.T) {
	menu := func() []observe.ShadowRegion {
		return []observe.ShadowRegion{
			det("button", 0.414, 0.437, 0.172, 0.035),
			det("button", 0.414, 0.479, 0.172, 0.035),
			det("button", 0.414, 0.521, 0.172, 0.035),
			det("button", 0.414, 0.563, 0.172, 0.035),
		}
	}

	t.Run("four rows seen six times", func(t *testing.T) {
		var infs []shadowreplay.Inference
		for i := 0; i < 6; i++ {
			infs = append(infs, infer(i, menu()...))
		}
		res := shadowreplay.Run(infs, shadowreplay.Production())
		regions := shadowreplay.Cluster(infs, "button", res)

		if len(regions) != 4 {
			t.Fatalf("%d apparent regions for four stacked rows, want 4", len(regions))
		}
		for _, r := range regions {
			if !r.Recurring() {
				t.Errorf("region at y=%.3f seen in %d inference(s); it should recur",
					r.Mean.Y, r.Inferences)
			}
			if r.Detections != 6 {
				t.Errorf("region at y=%.3f has %d detections, want 6", r.Mean.Y, r.Detections)
			}
			if len(r.Tracks) != 1 {
				t.Errorf("region at y=%.3f spans %d tracks — a recurring place split",
					r.Mean.Y, len(r.Tracks))
			}
		}
	})

	t.Run("twenty-four scattered one-offs", func(t *testing.T) {
		var infs []shadowreplay.Inference
		for i := 0; i < 24; i++ {
			// Every box somewhere else entirely: this is what a detector firing on moving
			// game content looks like, and none of it is a recurring structure.
			x := 0.04 * float64(i%12)
			y := 0.30 + 0.05*float64(i/12)
			infs = append(infs, infer(i, det("button", x, y, 0.03, 0.02)))
		}
		res := shadowreplay.Run(infs, shadowreplay.Production())
		regions := shadowreplay.Cluster(infs, "button", res)

		if len(regions) < 20 {
			t.Fatalf("%d apparent regions for 24 disjoint boxes; clustering is merging "+
				"unrelated detections and would report lost identity that never existed",
				len(regions))
		}
		var recurring int
		for _, r := range regions {
			if r.Recurring() {
				recurring++
			}
		}
		if recurring > 2 {
			t.Errorf("%d regions marked recurring; one-off boxes must not look like "+
				"structure", recurring)
		}
	})
}

// Cadence comes from recorded times, never from the configured interval.
func TestCadenceIsMeasuredFromRealTimestamps(t *testing.T) {
	slots := []shadowreplay.Slot{
		slot(1, 0, "valid"),
		slot(2, 2000, "skipped"),
		slot(3, 4100, "valid"),
		slot(4, 6000, "failed"),
		slot(5, 8000, "unproven"),
		slot(6, 12500, "valid"), // a long gap, because the detector was busy
	}
	c := shadowreplay.MeasureCadence(slots)

	if c.Valid != 3 || c.Skipped != 1 || c.Failed != 1 || c.Unproven != 1 {
		t.Errorf("valid=%d skipped=%d failed=%d unproven=%d, want 3/1/1/1",
			c.Valid, c.Skipped, c.Failed, c.Unproven)
	}
	if c.MaxGapMS != 8400 {
		t.Errorf("max gap %dms, want 8400 — the worst gap is what decides whether a "+
			"fifteen-second menu could be observed at all", c.MaxGapMS)
	}
	if c.SpanMS != 12500 {
		t.Errorf("span %dms, want 12500", c.SpanMS)
	}
}

// Only valid slots may become inferences. A skipped slot rendered as an empty inference would
// tell the tracker every element had vanished.
func TestOnlyValidSlotsBecomeInferences(t *testing.T) {
	button := det("button", 0.41, 0.44, 0.17, 0.04)
	slots := []shadowreplay.Slot{
		slot(1, 0, "valid", button),
		slot(2, 2000, "skipped"),
		slot(3, 4000, "failed"),
		slot(4, 6000, "unproven"),
		slot(5, 8000, "valid", button),
	}
	infs := shadowreplay.InferencesFrom(slots)
	if len(infs) != 2 {
		t.Fatalf("%d inference(s) from 5 slots, want 2", len(infs))
	}
	res := shadowreplay.Run(infs, shadowreplay.Production())
	if len(res.Tracks) != 1 {
		t.Fatalf("%d tracks, want 1", len(res.Tracks))
	}
	if res.Tracks[0].Episodes != 1 {
		t.Errorf("episodes = %d, want 1 — slots that carried no evidence were counted as "+
			"the element going away", res.Tracks[0].Episodes)
	}
}

// Part 14. "24 tracks" must arrive with a reason attached, and the reasons must distinguish
// the cases whose fixes are opposites.
func TestEveryNewTrackCarriesWhyItWasMinted(t *testing.T) {
	infs := []shadowreplay.Inference{
		// Two rows appear: neither has any prior track of its role.
		infer(0, det("button", 0.414, 0.437, 0.172, 0.035)),
		// A box somewhere else entirely: tracks exist, none overlaps.
		infer(1, det("button", 0.10, 0.80, 0.05, 0.03)),
	}
	res := shadowreplay.Run(infs, shadowreplay.Production())

	want := map[int]string{0: shadowreplay.ReasonFirstOfRole, 1: shadowreplay.ReasonNoCompatibleTrack}
	for _, a := range res.Assignments {
		if !a.NewTrack {
			continue
		}
		if w, ok := want[a.Inference]; ok && a.Reason != w {
			t.Errorf("inference %d minted a track for %q, want %q", a.Inference, a.Reason, w)
		}
	}

	// And greedy competition is named as such, not folded into "nothing matched".
	rival := []shadowreplay.Inference{
		infer(0, det("button", 0.414, 0.437, 0.172, 0.035)),
		// Two detections now overlap that one track; the weaker must say it lost a rival.
		infer(1,
			det("button", 0.414, 0.437, 0.172, 0.035),
			det("button", 0.414, 0.445, 0.172, 0.035)),
	}
	r2 := shadowreplay.Run(rival, shadowreplay.Production())
	var found bool
	for _, a := range r2.Assignments {
		if a.NewTrack && a.Reason == shadowreplay.ReasonTrackAlreadyAssigned {
			found = true
		}
	}
	if !found {
		t.Error("a detection that overlapped an already-taken track did not report " +
			"competition; assignment failures would be invisible")
	}
}
