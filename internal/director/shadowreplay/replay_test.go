package shadowreplay_test

import (
	"math"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/shadowreplay"
)

func reg(x, y, w, h float64) observe.Region {
	return observe.Region{X: x, Y: y, Width: w, Height: h}
}

func det(role string, x, y, w, h float64) observe.ShadowRegion {
	return observe.ShadowRegion{
		Role: role, Region: reg(x, y, w, h), Confidence: 0.4, Nameable: role == "button",
	}
}

func infer(index int, regions ...observe.ShadowRegion) shadowreplay.Inference {
	return shadowreplay.Inference{Index: index, Regions: regions}
}

// production folds the same inferences through the SHIPPING tracker.
func production(infs []shadowreplay.Inference) observe.ShadowTotals {
	var totals observe.ShadowTotals
	for _, in := range infs {
		totals.Add(observe.ShadowSample{
			Detector: "screenparser", Ran: true, TargetProven: true, Regions: in.Regions,
		})
	}
	return totals
}

// The load-bearing test. Every number this package reports is only meaningful if replaying at
// the production policy reproduces production tracking exactly — otherwise the diagnosis is
// measuring a second implementation's opinion of the bug.
func TestTheMirrorReproducesProductionTracking(t *testing.T) {
	cases := map[string][]shadowreplay.Inference{
		"a still control": {
			infer(1, det("button", 0.41, 0.437, 0.17, 0.035)),
			infer(2, det("button", 0.41, 0.437, 0.17, 0.035)),
			infer(3, det("button", 0.41, 0.438, 0.17, 0.035)),
		},
		"four adjacent rows jittering": {
			infer(1, rows(0)...), infer(2, rows(0.002)...),
			infer(3, rows(-0.001)...), infer(4, rows(0.001)...),
		},
		"a control that really goes away and returns": {
			infer(1, det("button", 0.4, 0.4, 0.2, 0.05)),
			infer(2), infer(3), infer(4),
			infer(5, det("button", 0.4, 0.4, 0.2, 0.05)),
		},
		"a wandering false positive": {
			infer(1, det("button", 0.05, 0.05, 0.1, 0.05)),
			infer(2, det("button", 0.40, 0.30, 0.1, 0.05)),
			infer(3, det("button", 0.75, 0.60, 0.1, 0.05)),
		},
		"mixed roles overlapping": {
			infer(1, det("button", 0.4, 0.4, 0.2, 0.05), det("icon", 0.4, 0.4, 0.2, 0.05)),
			infer(2, det("button", 0.4, 0.4, 0.2, 0.05), det("icon", 0.4, 0.4, 0.2, 0.05)),
		},
		"steady drift": drift(6, 0.012),
	}

	for name, infs := range cases {
		t.Run(name, func(t *testing.T) {
			want := production(infs).Tracks
			got := shadowreplay.Run(infs, shadowreplay.Production()).Tracks

			if len(got) != len(want) {
				t.Fatalf("mirror made %d tracks, production %d — the replay is not "+
					"describing the tracker under test", len(got), len(want))
			}
			for i := range want {
				w, g := want[i], got[i]
				if g.ID != w.ID || g.Seen != w.Seen || g.Episodes != w.Episodes ||
					g.Eligible != w.Eligible {
					t.Errorf("track %d: mirror %s seen=%d elig=%d ep=%d; production "+
						"%s seen=%d elig=%d ep=%d", i,
						g.ID, g.Seen, g.Eligible, g.Episodes,
						w.ID, w.Seen, w.Eligible, w.Episodes)
				}
				if math.Abs(g.MeanIoU-w.MeanIoU) > 1e-9 {
					t.Errorf("track %s mean IoU %.6f, production %.6f",
						g.ID, g.MeanIoU, w.MeanIoU)
				}
			}
		})
	}
}

func rows(dy float64) []observe.ShadowRegion {
	return []observe.ShadowRegion{
		det("button", 0.41, 0.437+dy, 0.17, 0.035),
		det("button", 0.41, 0.479+dy, 0.17, 0.035),
		det("button", 0.41, 0.521+dy, 0.17, 0.035),
		det("button", 0.41, 0.563+dy, 0.17, 0.035),
	}
}

// drift is one control sliding steadily, the way a menu animating in behaves.
func drift(n int, step float64) []shadowreplay.Inference {
	var out []shadowreplay.Inference
	for i := 0; i < n; i++ {
		out = append(out, infer(i, det("button", 0.41, 0.437+float64(i)*step, 0.17, 0.035)))
	}
	return out
}

// THE mechanism, demonstrated on production code rather than argued about.
//
// A control whose consecutive overlap never once drops below the match threshold is still
// fragmented, because production scores every candidate against the FIRST box it ever saw
// (observe.ShadowTrack.record updates every statistic except Reference). Steady motion of a
// third of the element's height per inference keeps neighbour-to-neighbour overlap near 0.49
// and drives overlap-with-first to zero.
func TestFrozenReferenceGeometryFragmentsASteadilyMovingControl(t *testing.T) {
	infs := drift(6, 0.012)

	// First: consecutive overlap is comfortably ABOVE the bar at every single step.
	for i := 1; i < len(infs); i++ {
		o := shadowreplay.IoU(infs[i-1].Regions[0].Region, infs[i].Regions[0].Region)
		if o < observe.TrackMatchIoU {
			t.Fatalf("step %d consecutive IoU %.3f is already under the %.2f threshold; "+
				"this fixture cannot isolate reference drift", i, o, observe.TrackMatchIoU)
		}
	}

	tracks := production(infs).Tracks
	if len(tracks) < 2 {
		t.Fatalf("production made %d track(s) — if a steadily moving control survives, "+
			"reference drift is not a live failure mode and this diagnosis is wrong",
			len(tracks))
	}

	// And the fix that the measurement implies, measured rather than assumed.
	p := shadowreplay.Production()
	p.Reference = shadowreplay.ReferencePrevious
	if got := len(shadowreplay.Run(infs, p).Tracks); got != 1 {
		t.Errorf("matching against the previous detection gave %d tracks, want 1", got)
	}
}

// The counterpart. A control that sits still is tracked correctly by the frozen reference —
// which is why fixed HUD elements never exposed this and menu rows did.
func TestAStationaryControlIsUnaffectedByTheReferencePolicy(t *testing.T) {
	var infs []shadowreplay.Inference
	for i := 0; i < 6; i++ {
		infs = append(infs, infer(i, det("icon", 0.02, 0.85, 0.12, 0.06)))
	}
	for _, pol := range []shadowreplay.ReferencePolicy{
		shadowreplay.ReferenceFrozenFirst,
		shadowreplay.ReferencePrevious,
		shadowreplay.ReferenceMean,
	} {
		p := shadowreplay.Production()
		p.Reference = pol
		if got := len(shadowreplay.Run(infs, p).Tracks); got != 1 {
			t.Errorf("policy %s gave %d tracks for a stationary element, want 1", pol, got)
		}
	}
}

// Loosening the threshold is the OTHER candidate fix, and it must be shown to have a cost.
// A threshold low enough to absorb drift also merges neighbouring menu rows, which is a worse
// failure than fragmentation: a capability aimed at one row would act on another.
func TestALowerThresholdMergesAdjacentRows(t *testing.T) {
	infs := []shadowreplay.Inference{
		infer(1, rows(0)...), infer(2, rows(0.021)...), infer(3, rows(0.042)...),
	}
	p := shadowreplay.Production()
	p.MatchIoU = 0.10
	res := shadowreplay.Run(infs, p)

	swapped := 0
	for _, a := range res.Assignments {
		if !a.NewTrack && len(a.Candidates) > 1 {
			swapped++
		}
	}
	if swapped == 0 {
		t.Error("no detection had competing candidate tracks at IoU 0.10; this fixture is " +
			"not exercising the merge risk it exists to demonstrate")
	}
}

// Skips are absence-free here too: an inference that did not run must not be materialised.
func TestAMissingInferenceIsNotADetectorMiss(t *testing.T) {
	infs := []shadowreplay.Inference{
		infer(0, det("button", 0.41, 0.437, 0.17, 0.035)),
		infer(2, det("button", 0.41, 0.437, 0.17, 0.035)),
	}
	anchor := shadowreplay.Anchor{
		Identity: "menu_resume", Role: "button",
		Boxes: map[int]observe.Region{
			0: reg(0.414, 0.437, 0.172, 0.035),
			1: reg(0.414, 0.437, 0.172, 0.035), // no inference covered this frame
			2: reg(0.414, 0.437, 0.172, 0.035),
		},
	}
	res := shadowreplay.Run(infs, shadowreplay.Production())
	rep := shadowreplay.Analyse(infs, []shadowreplay.Anchor{anchor}, res)[0]

	if rep.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", rep.Skipped)
	}
	if rep.Missed != 0 {
		t.Errorf("missed = %d, want 0 — a slot with no inference is unknown, never a "+
			"detector failure", rep.Missed)
	}
	if rep.Detected != 2 || rep.Eligible != 3 {
		t.Errorf("detected %d of %d eligible, want 2 of 3", rep.Detected, rep.Eligible)
	}
}

// Recall is measured against visibility, and a real miss is reported as one.
func TestARealDetectorMissIsCounted(t *testing.T) {
	infs := []shadowreplay.Inference{
		infer(0, det("button", 0.41, 0.437, 0.17, 0.035)),
		infer(1), // inference ran, found nothing
		infer(2, det("button", 0.41, 0.437, 0.17, 0.035)),
	}
	anchor := shadowreplay.Anchor{
		Identity: "menu_resume", Role: "button",
		Boxes: map[int]observe.Region{
			0: reg(0.414, 0.437, 0.172, 0.035),
			1: reg(0.414, 0.437, 0.172, 0.035),
			2: reg(0.414, 0.437, 0.172, 0.035),
		},
	}
	res := shadowreplay.Run(infs, shadowreplay.Production())
	rep := shadowreplay.Analyse(infs, []shadowreplay.Anchor{anchor}, res)[0]

	if rep.Missed != 1 || rep.Detected != 2 {
		t.Errorf("detected %d missed %d, want 2 and 1", rep.Detected, rep.Missed)
	}
	if got := rep.Recall(); math.Abs(got-2.0/3.0) > 1e-9 {
		t.Errorf("recall %.3f, want 0.667", got)
	}
	// One tolerated miss must not fragment the element.
	if rep.Fragmented != 0 {
		t.Errorf("fragmented = %d, want 0", rep.Fragmented)
	}
	if len(rep.Tracks) != 1 {
		t.Errorf("%d tracks, want 1", len(rep.Tracks))
	}
}

// Fragmentation is reported WITH the two numbers that say why, or it cannot be classified.
func TestFragmentationRecordsPreviousAndReferenceOverlap(t *testing.T) {
	infs := drift(6, 0.012)
	boxes := map[int]observe.Region{}
	for i, in := range infs {
		boxes[i] = in.Regions[0].Region
	}
	anchor := shadowreplay.Anchor{Identity: "menu_row", Role: "button", Boxes: boxes}

	res := shadowreplay.Run(infs, shadowreplay.Production())
	rep := shadowreplay.Analyse(infs, []shadowreplay.Anchor{anchor}, res)[0]

	if rep.Fragmented == 0 {
		t.Fatal("no fragmentation recorded for a control production splits")
	}
	if rep.Recall() != 1 {
		t.Fatalf("recall %.2f — detector recall must be perfect here so that only "+
			"tracking can be blamed", rep.Recall())
	}
	var checked bool
	for _, op := range rep.Opportunities {
		if op.Outcome != shadowreplay.DetectedFragmented {
			continue
		}
		checked = true
		if op.PrevIoU < observe.TrackMatchIoU {
			t.Errorf("inference %d: previous-detection IoU %.3f is below the threshold; "+
				"this is unstable localisation, not reference drift", op.Inference, op.PrevIoU)
		}
		if op.RefIoU >= observe.TrackMatchIoU {
			t.Errorf("inference %d: reference IoU %.3f cleared the threshold yet the track "+
				"still broke", op.Inference, op.RefIoU)
		}
	}
	if !checked {
		t.Fatal("no fragmented opportunity carried its overlap numbers")
	}
	if rep.MedianConsecutiveIoU < observe.TrackMatchIoU {
		t.Errorf("median consecutive IoU %.3f", rep.MedianConsecutiveIoU)
	}
	if rep.MedianRefIoU >= rep.MedianConsecutiveIoU {
		t.Errorf("reference overlap (%.3f) is not below consecutive overlap (%.3f); the "+
			"drift signature is absent", rep.MedianRefIoU, rep.MedianConsecutiveIoU)
	}
}
