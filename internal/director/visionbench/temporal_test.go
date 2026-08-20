package visionbench_test

import (
	"fmt"
	"image"
	"math"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// Temporal semantics for sequences that CHANGE.
//
// The defect these lock down: temporal truth had one rule — a track counted only if it appeared
// in at least half its sequence — and that rule made two mirror-image transitions score
// oppositely. `pause-open` (menu in 3 of 6 frames) qualified and scored 83%; `pause-close`
// (the same menu in 2 of 6) had zero qualifying tracks, and 0/0 was rendered as 0%. The
// benchmark reported a detector failure that never happened.
//
// The load-bearing test is TestMirroredTransitionsScoreIdentically. Everything else here
// protects one property it depends on.

const frameSide = 1000

// track is where the element under test lives, in normalised coordinates.
var track = visionbench.NormRect{X: 0.4, Y: 0.4, W: 0.2, H: 0.1}

// transitionCase builds a sequence from two presence patterns of equal length: what the truth
// declares, and what the detector found.
//
// Written as parallel strings so a test reads as the thing it is testing:
//
//	truth "..XXX"   element absent for two frames, then present for three
//	found "...XX"   detector picked it up one frame late
func transitionCase(t *testing.T, name string, truth, found string) (
	visionbench.TruthMetrics, visionbench.TruthMetrics) {

	t.Helper()
	if len(truth) != len(found) {
		t.Fatalf("%s: truth %q and found %q describe different numbers of frames",
			name, truth, found)
	}
	bounds := map[string]image.Rectangle{}
	byFrame := map[string][]visionbench.Detection{}
	var truths []visionbench.FrameTruth

	for i := range truth {
		ft := visionbench.FrameTruth{
			Schema: 1, Frame: fmt.Sprintf("f%02d", i), Sequence: name, Index: i,
			InterfacePresent: true,
		}
		// Exactly one identity, deliberately. A second always-present region would be a
		// static track averaged into the same sequence mean, and every assertion below
		// would then be reading a blend of two questions instead of the one under test.
		if truth[i] == 'X' {
			ft.Regions = append(ft.Regions, visionbench.TruthRegion{
				Kind: visionbench.TruthButton, Bounds: track, Identity: "subject",
			})
		}
		truths = append(truths, ft)

		fb := image.Rect(0, 0, frameSide, frameSide)
		bounds[ft.Key()] = fb
		var dets []visionbench.Detection
		if found[i] == 'X' {
			dets = append(dets, visionbench.Detection{
				Label: "button", Confidence: 0.9, Bounds: track.Pixels(fb),
			})
		}
		byFrame[ft.Key()] = dets
	}

	modes := map[string]visionbench.SequenceTruth{
		name: {
			Schema: visionbench.SequenceSchema, Sequence: name,
			Tracks: []visionbench.TrackTruth{{Identity: "subject", Present: spansOf(truth)}},
		},
	}
	// Both readings of the same evidence: the declared one, and the undeclared fallback —
	// which is the old static majority rule. Tests assert on the first and use the second to
	// show what the rule they replaced would have said.
	return visionbench.EvaluateTruthModes(byFrame, bounds, truths, modes),
		visionbench.EvaluateTruth(byFrame, bounds, truths)
}

// spansOf turns "XX..XX" into [[0,1],[4,5]].
//
// Derived from the same string that builds the annotations, deliberately: these tests are about
// SCORING, and a hand-written span list would just be a second place to make a typo. That the
// declaration and the annotations must agree is a separate property, checked by
// CheckAnnotations and its own tests.
func spansOf(pattern string) []visionbench.Span {
	var out []visionbench.Span
	start := -1
	for i := 0; i <= len(pattern); i++ {
		present := i < len(pattern) && pattern[i] == 'X'
		switch {
		case present && start < 0:
			start = i
		case !present && start >= 0:
			out = append(out, visionbench.Span{From: start, To: i - 1})
			start = -1
		}
	}
	return out
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.001 }

// Part 10. THE regression: an appearance and its time-reversed disappearance, with mirrored
// detector quality, must score the same. The old majority rule cannot pass this.
func TestMirroredTransitionsScoreIdentically(t *testing.T) {
	// Perfectly mirrored: identical detector competence, opposite direction of change.
	app, appOld := transitionCase(t, "appear",
		"..XX", "..XX")
	dis, disOld := transitionCase(t, "vanish",
		"XX..", "XX..")

	if !near(app.TemporalPrecision, dis.TemporalPrecision) ||
		!near(app.TemporalRecall, dis.TemporalRecall) {
		t.Fatalf("mirror-image transitions scored differently:\n"+
			"  appearance     P %.3f R %.3f\n  disappearance  P %.3f R %.3f",
			app.TemporalPrecision, app.TemporalRecall,
			dis.TemporalPrecision, dis.TemporalRecall)
	}
	if !near(app.TemporalRecall, 1) || !near(app.TemporalPrecision, 1) {
		t.Fatalf("a perfect transition detector scored P %.3f R %.3f, want 1.0/1.0",
			app.TemporalPrecision, app.TemporalRecall)
	}

	_, _ = appOld, disOld
}

// The old rule's defect, reproduced at the shape the real corpus actually has.
//
// `pause-open` holds its menu for 3 of 6 frames and `pause-close` for 2 of 6 — near-mirrors
// either side of a majority threshold. Same detector, same competence, opposite verdicts. This
// is the measurement that made a working detector look like it kept a closed menu alive.
func TestOldStaticRuleSplitTheRealCorpusShape(t *testing.T) {
	// Perfect detection in both, differing only in which side of frame 3 the menu sits.
	_, openOld := transitionCase(t, "pause-open",
		"...XXX", "...XXX")
	_, closeOld := transitionCase(t, "pause-close",
		"XX....", "XX....")
	if near(openOld.TemporalRecall, closeOld.TemporalRecall) {
		t.Fatalf("the static rule scored these alike (%.3f / %.3f) — this test has stopped "+
			"guarding the defect it was written for",
			openOld.TemporalRecall, closeOld.TemporalRecall)
	}
	if !near(openOld.TemporalRecall, 1) || !near(closeOld.TemporalRecall, 0) {
		t.Fatalf("expected the old rule to score 1.0 / 0.0, got %.3f / %.3f",
			openOld.TemporalRecall, closeOld.TemporalRecall)
	}
	if closeOld.PersistentTruthTracks != 0 {
		t.Fatalf("the old 0%% was expected to come from 0/0, but %d tracks qualified",
			closeOld.PersistentTruthTracks)
	}

	// Under the declared semantics both are perfect, because both detectors were.
	openNew, _ := transitionCase(t, "pause-open",
		"...XXX", "...XXX")
	closeNew, _ := transitionCase(t, "pause-close",
		"XX....", "XX....")
	if !near(openNew.TemporalRecall, 1) || !near(closeNew.TemporalRecall, 1) {
		t.Fatalf("corrected recall %.3f / %.3f, want 1.0 / 1.0",
			openNew.TemporalRecall, closeNew.TemporalRecall)
	}
}

// Part 11. A perfect detector scores perfectly wherever the transition falls. This is what
// stops another accidental threshold from quietly becoming semantics.
func TestBoundaryPlacementDoesNotChangeAPerfectScore(t *testing.T) {
	const n = 6
	for b := 1; b < n; b++ {
		truth := ""
		for i := 0; i < n; i++ {
			if i >= b {
				truth += "X"
			} else {
				truth += "."
			}
		}
		m, old := transitionCase(t, "appear", truth, truth)
		if !near(m.TemporalPrecision, 1) || !near(m.TemporalRecall, 1) {
			t.Errorf("boundary %d/%d: perfect detector scored P %.3f R %.3f, want 1.0/1.0",
				b, n, m.TemporalPrecision, m.TemporalRecall)
		}
		// The old rule collapsed once the present phase fell below half the sequence.
		if b > n/2 && !near(old.TemporalRecall, 0) {
			t.Errorf("boundary %d/%d: expected the static rule to fail here", b, n)
		}
	}
}

// Part 12.
func TestPerfectAppearance(t *testing.T) {
	m, _ := transitionCase(t, "appear", "..XXX", "..XXX")
	if !near(m.TemporalPrecision, 1) || !near(m.TemporalRecall, 1) {
		t.Fatalf("P %.3f R %.3f, want 1.0/1.0", m.TemporalPrecision, m.TemporalRecall)
	}
}

// Part 13. Claimed before it existed: precision falls, recall does not.
func TestEarlyFalseAppearance(t *testing.T) {
	m, _ := transitionCase(t, "appear", "..XX", "X.XX")
	if m.TemporalPrecision >= 1 {
		t.Errorf("claiming the element before it appeared did not cost precision: %.3f",
			m.TemporalPrecision)
	}
	if !near(m.TemporalRecall, 1) {
		t.Errorf("recall %.3f — an early claim must not be charged as a miss too",
			m.TemporalRecall)
	}
	if m.Mistimed != 1 {
		t.Errorf("Mistimed = %d, want 1", m.Mistimed)
	}
}

// Part 14. Late onset is not equivalent to perfect onset.
func TestLateAppearanceCostsRecall(t *testing.T) {
	late, _ := transitionCase(t, "appear", "..XXX", "...XX")
	exact, _ := transitionCase(t, "appear", "..XXX", "..XXX")
	if late.TemporalRecall >= exact.TemporalRecall {
		t.Fatalf("a one-frame-late onset scored recall %.3f against a perfect %.3f — "+
			"timing quality is not being measured", late.TemporalRecall, exact.TemporalRecall)
	}
	if !near(late.TemporalRecall, 2.0/3.0) {
		t.Errorf("recall %.3f, want 2/3 (found in 2 of 3 frames it was there)",
			late.TemporalRecall)
	}
}

// Part 15.
func TestPerfectDisappearance(t *testing.T) {
	m, _ := transitionCase(t, "vanish", "XX..", "XX..")
	if !near(m.TemporalPrecision, 1) || !near(m.TemporalRecall, 1) {
		t.Fatalf("P %.3f R %.3f, want 1.0/1.0", m.TemporalPrecision, m.TemporalRecall)
	}
}

// Part 16. The behaviour the project spent two milestones wrongly believing it had measured:
// a detector that keeps reporting a menu after the menu is gone.
func TestStaleDisappearanceCostsPrecision(t *testing.T) {
	stale, _ := transitionCase(t, "vanish", "XX..", "XXX.")
	clean, _ := transitionCase(t, "vanish", "XX..", "XX..")
	if stale.TemporalPrecision >= clean.TemporalPrecision {
		t.Fatalf("holding a departed element scored precision %.3f against a clean %.3f — "+
			"staleness is invisible to this metric", stale.TemporalPrecision,
			clean.TemporalPrecision)
	}
	// Asserted on the raw counts rather than the sequence mean, because the mean also
	// carries the persistent-false-structure measure and would be reporting two things.
	if stale.OnTime != 2 || stale.Mistimed != 1 {
		t.Errorf("OnTime %d, Mistimed %d; want 2 and 1 — two timely claims and one made "+
			"after the element was gone", stale.OnTime, stale.Mistimed)
	}
	if !near(stale.TemporalRecall, 1) {
		t.Errorf("recall %.3f — it did track the element while it was there",
			stale.TemporalRecall)
	}
}

// Part 17. Dropped it before it left.
func TestPrematureDisappearanceCostsRecall(t *testing.T) {
	m, _ := transitionCase(t, "vanish", "XXX.", "X...")
	if !near(m.TemporalRecall, 1.0/3.0) {
		t.Errorf("recall %.3f, want 1/3 (tracked 1 of the 3 frames it was there)",
			m.TemporalRecall)
	}
	// It never claimed the element when it was absent, so its timing was not wrong —
	// only incomplete. Charging precision here would punish one miss twice.
	if !near(m.TemporalPrecision, 1) {
		t.Errorf("precision %.3f, want 1.0", m.TemporalPrecision)
	}
}

// Part 18. Static sequences keep the persistence rule, and a static track inside a TRANSITION
// sequence keeps it too — a HUD element does not become a transition because a menu opened
// over it.
func TestStaticPersistenceSurvives(t *testing.T) {
	held, _ := transitionCase(t, "steady", "XXXX", "XXXX")
	if !near(held.TemporalRecall, 1) {
		t.Fatalf("a fully tracked static element scored recall %.3f", held.TemporalRecall)
	}
	if held.TransitionTracks != 0 {
		t.Fatalf("a static sequence produced %d transition tracks", held.TransitionTracks)
	}
	// The static rule is a MAJORITY rule and stays one: a detector that finds a persistent
	// element in most frames still counts, one that mostly loses it does not.
	half, _ := transitionCase(t, "steady", "XXXX", "X.X.")
	if !near(half.TemporalRecall, 1) {
		t.Errorf("static recall %.3f — the majority rule was weakened, not preserved",
			half.TemporalRecall)
	}
	lost, _ := transitionCase(t, "steady", "XXXX", "X...")
	if !near(lost.TemporalRecall, 0) {
		t.Errorf("a mostly-lost static element scored recall %.3f, want 0",
			lost.TemporalRecall)
	}
}

// A persistent identity inside a TRANSITION sequence keeps the static rule. The HUD does not
// become a transition because a menu opened over it.
func TestPersistentIdentityInsideATransitionSequence(t *testing.T) {
	hud := visionbench.NormRect{X: 0.02, Y: 0.85, W: 0.12, H: 0.06}
	var truths []visionbench.FrameTruth
	byFrame := map[string][]visionbench.Detection{}
	bounds := map[string]image.Rectangle{}
	fb := image.Rect(0, 0, frameSide, frameSide)

	// Menu arrives at frame 2; the HUD is there the whole time and is always found.
	for i := 0; i < 4; i++ {
		ft := visionbench.FrameTruth{
			Schema: 1, Frame: fmt.Sprintf("f%02d", i), Sequence: "appear", Index: i,
			InterfacePresent: true,
			Regions: []visionbench.TruthRegion{
				{Kind: visionbench.TruthBar, Bounds: hud, Identity: "hud"},
			},
		}
		dets := []visionbench.Detection{
			{Label: "bar", Confidence: 0.9, Bounds: hud.Pixels(fb)},
		}
		if i >= 2 {
			ft.Regions = append(ft.Regions, visionbench.TruthRegion{
				Kind: visionbench.TruthButton, Bounds: track, Identity: "menu",
			})
			dets = append(dets, visionbench.Detection{
				Label: "button", Confidence: 0.9, Bounds: track.Pixels(fb),
			})
		}
		truths = append(truths, ft)
		bounds[ft.Key()] = fb
		byFrame[ft.Key()] = dets
	}
	modes := map[string]visionbench.SequenceTruth{
		"appear": {Schema: visionbench.SequenceSchema, Sequence: "appear",
			Tracks: []visionbench.TrackTruth{
				{Identity: "hud", Present: spansOf("XXXX")},
				{Identity: "menu", Present: spansOf("..XX")},
			}},
	}
	m := visionbench.EvaluateTruthModes(byFrame, bounds, truths, modes)

	if m.TransitionTracks != 1 {
		t.Errorf("TransitionTracks = %d, want 1 — only the menu is transitioning",
			m.TransitionTracks)
	}
	if m.PersistentTruthTracks != 1 {
		t.Errorf("PersistentTruthTracks = %d, want 1 — the HUD is still a static track "+
			"even though the sequence around it changes", m.PersistentTruthTracks)
	}
	if !near(m.TemporalRecall, 1) || !near(m.TemporalPrecision, 1) {
		t.Errorf("P %.3f R %.3f, want 1.0/1.0", m.TemporalPrecision, m.TemporalRecall)
	}
}

// Part 22, transition half. The pathological detectors, judged by the corrected metric.
func TestTransitionPathologicalBackends(t *testing.T) {
	cases := []struct {
		name  string
		found string
		// wantWorseThan names the property the detector must fail.
		precisionBelow1 bool
		recallBelow1    bool
	}{
		{"always-on", "XXXX", true, false},
		{"never-on", "....", false, true},
		{"one-frame-late", ".XX.", false, true},
		{"one-frame-early", "XXX.", true, false},
	}
	// Truth: present in frames 0 and 1, gone from frame 2.
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, _ := transitionCase(t, "vanish",
				"XX..", c.found)
			if c.precisionBelow1 && m.TemporalPrecision >= 1 {
				t.Errorf("precision %.3f, expected a penalty", m.TemporalPrecision)
			}
			if c.recallBelow1 && m.TemporalRecall >= 1 {
				t.Errorf("recall %.3f, expected a penalty", m.TemporalRecall)
			}
		})
	}
	// A perfect detector must still beat all four.
	best, _ := transitionCase(t, "vanish", "XX..", "XX..")
	for _, c := range cases {
		m, _ := transitionCase(t, "vanish", "XX..", c.found)
		if m.TemporalPrecision+m.TemporalRecall >= best.TemporalPrecision+best.TemporalRecall {
			t.Errorf("%s scored P+R %.3f, at or above a perfect detector's %.3f", c.name,
				m.TemporalPrecision+m.TemporalRecall,
				best.TemporalPrecision+best.TemporalRecall)
		}
	}
}

// Part 20. Aggregation is a mean over SEQUENCES, so a long static run cannot swamp a short
// transition. Asserted by construction rather than by inspection of the mean.
func TestAggregateDoesNotWeightBySequenceLength(t *testing.T) {
	build := func(name string,
		truth, found string, into *[]visionbench.FrameTruth,
		byFrame map[string][]visionbench.Detection, bounds map[string]image.Rectangle) {

		for i := range truth {
			ft := visionbench.FrameTruth{
				Schema: 1, Frame: fmt.Sprintf("%s-%02d", name, i), Sequence: name,
				Index: i, InterfacePresent: true,
			}
			if truth[i] == 'X' {
				ft.Regions = append(ft.Regions, visionbench.TruthRegion{
					Kind: visionbench.TruthButton, Bounds: track, Identity: "subject",
				})
			}
			*into = append(*into, ft)
			fb := image.Rect(0, 0, frameSide, frameSide)
			bounds[ft.Key()] = fb
			if found[i] == 'X' {
				byFrame[ft.Key()] = []visionbench.Detection{
					{Label: "button", Confidence: 0.9, Bounds: track.Pixels(fb)},
				}
			} else {
				byFrame[ft.Key()] = nil
			}
		}
	}

	var truths []visionbench.FrameTruth
	byFrame := map[string][]visionbench.Detection{}
	bounds := map[string]image.Rectangle{}
	// A long, perfectly tracked static run...
	build("long-static",
		"XXXXXXXXXXXXXXXXXXXX", "XXXXXXXXXXXXXXXXXXXX", &truths, byFrame, bounds)
	// ...and a short transition the detector completely misses.
	build("short-vanish", "XX..", "....",
		&truths, byFrame, bounds)

	modes := map[string]visionbench.SequenceTruth{
		"long-static": {Schema: visionbench.SequenceSchema, Sequence: "long-static",
			Tracks: []visionbench.TrackTruth{
				{Identity: "subject", Present: spansOf("XXXXXXXXXXXXXXXXXXXX")},
			}},
		"short-vanish": {Schema: visionbench.SequenceSchema, Sequence: "short-vanish",
			Tracks: []visionbench.TrackTruth{
				{Identity: "subject", Present: spansOf("XX..")},
			}},
	}
	m := visionbench.EvaluateTruthModes(byFrame, bounds, truths, modes)

	// Two sequences, one perfect and one failed: the mean is a half, NOT 20/24 of the frames.
	if !near(m.TemporalRecall, 0.5) {
		t.Fatalf("temporal recall %.3f, want 0.5 — a 20-frame static sequence is "+
			"outvoting a 4-frame transition, which is the weighting Part 20 forbids",
			m.TemporalRecall)
	}
	if m.TemporalRecallSequences != 2 {
		t.Fatalf("mean taken over %d sequences, want 2", m.TemporalRecallSequences)
	}
}

// A sequence that offers no temporal evidence is excluded from the mean, not scored zero.
// Rendering "no opportunity" as 0% is the original defect in its purest form.
func TestNoTemporalOpportunityIsExcludedNotZero(t *testing.T) {
	var truths []visionbench.FrameTruth
	byFrame := map[string][]visionbench.Detection{}
	bounds := map[string]image.Rectangle{}
	fb := image.Rect(0, 0, frameSide, frameSide)

	// One sequence with a perfectly tracked element, one with no annotated regions at all.
	for i := 0; i < 4; i++ {
		good := visionbench.FrameTruth{
			Schema: 1, Frame: fmt.Sprintf("g%02d", i), Sequence: "good", Index: i,
			InterfacePresent: true,
			Regions: []visionbench.TruthRegion{
				{Kind: visionbench.TruthButton, Bounds: track, Identity: "subject"},
			},
		}
		truths = append(truths, good)
		bounds[good.Key()] = fb
		byFrame[good.Key()] = []visionbench.Detection{
			{Label: "button", Confidence: 0.9, Bounds: track.Pixels(fb)},
		}

		empty := visionbench.FrameTruth{
			Schema: 1, Frame: fmt.Sprintf("e%02d", i), Sequence: "empty", Index: i,
			InterfacePresent: false,
		}
		truths = append(truths, empty)
		bounds[empty.Key()] = fb
		byFrame[empty.Key()] = nil
	}
	m := visionbench.EvaluateTruth(byFrame, bounds, truths)
	if !near(m.TemporalRecall, 1) {
		t.Fatalf("temporal recall %.3f, want 1.0 — the sequence with no temporal "+
			"opportunity was averaged in as a zero", m.TemporalRecall)
	}
	if m.TemporalRecallSequences != 1 {
		t.Fatalf("mean over %d sequences, want 1", m.TemporalRecallSequences)
	}
}

// Part 6. A -> B -> A: the shape that broke the corpus.
//
// `pause-close` is this exactly — menu, gone, menu — and the single-boundary model could not
// say so. It was declared a disappearance, four frames were asserted to hold no interface, and
// two of them show the pause menu at full opacity. The detector was then charged with false
// positives for reading the screen correctly.
//
// Nothing about a recurrence is special to the scorer: presence is intervals, absence is the
// complement, and the same two frame ratios apply. This test exists so that stays true.
func TestRecurringPresenceScoresLikeAnyOtherShape(t *testing.T) {
	const truth = "XX..XX" // present, gone, present again

	perfect, static := transitionCase(t, "recur", truth, "XX..XX")
	if !near(perfect.TemporalPrecision, 1) || !near(perfect.TemporalRecall, 1) {
		t.Fatalf("a detector that matched a recurrence exactly scored P %.3f R %.3f, "+
			"want 1.0/1.0", perfect.TemporalPrecision, perfect.TemporalRecall)
	}
	if perfect.Mistimed != 0 {
		t.Errorf("Mistimed = %d, want 0", perfect.Mistimed)
	}
	if perfect.Expected != 4 {
		t.Errorf("Expected = %d, want 4 — the two spells on screen", perfect.Expected)
	}
	// The undeclared fallback keeps the majority rule, which cannot see the gap at all: 4 of
	// 6 frames is a majority, so a detector that never let go would score the same.
	if !near(static.TemporalRecall, 1) {
		t.Errorf("the static reading scored %.3f; this test's contrast depends on it "+
			"passing a recurrence unexamined", static.TemporalRecall)
	}

	// Held the element through the gap: precision falls, recall is untouched.
	held, heldStatic := transitionCase(t, "recur", truth, "XXXXXX")
	if held.TemporalPrecision >= perfect.TemporalPrecision {
		t.Errorf("a detector that never let go scored precision %.3f against a perfect "+
			"%.3f", held.TemporalPrecision, perfect.TemporalPrecision)
	}
	if !near(held.TemporalRecall, 1) {
		t.Errorf("recall %.3f — it did find the element whenever it was there",
			held.TemporalRecall)
	}
	if held.Mistimed != 2 {
		t.Errorf("Mistimed = %d, want 2 — the two frames it claimed a departed element",
			held.Mistimed)
	}
	// The point of the whole milestone: without declared intervals the two are
	// indistinguishable. A majority rule sees 4 of 6 frames either way and cannot ask the
	// only question that matters — whether the detector let go during the gap.
	if !near(heldStatic.TemporalPrecision, static.TemporalPrecision) ||
		!near(heldStatic.TemporalRecall, static.TemporalRecall) {
		t.Errorf("the undeclared reading told a perfect detector (P %.3f R %.3f) apart from "+
			"one that never let go (P %.3f R %.3f); this test has stopped demonstrating "+
			"why intervals must be declared",
			static.TemporalPrecision, static.TemporalRecall,
			heldStatic.TemporalPrecision, heldStatic.TemporalRecall)
	}

	// Never reacquired after the element came back: recall falls, precision is untouched.
	dropped, _ := transitionCase(t, "recur", truth, "XX....")
	if !near(dropped.TemporalRecall, 0.5) {
		t.Errorf("recall %.3f, want 0.5 — it tracked the first spell and missed the second",
			dropped.TemporalRecall)
	}
	if !near(dropped.TemporalPrecision, 1) {
		t.Errorf("precision %.3f — failing to reacquire is a miss, not a mistimed claim",
			dropped.TemporalPrecision)
	}

	// Claimed the element during the gap, before its second spell began.
	early, _ := transitionCase(t, "recur", truth, "XX.XXX")
	if early.TemporalPrecision >= perfect.TemporalPrecision {
		t.Errorf("claiming the element before it returned scored precision %.3f against a "+
			"perfect %.3f", early.TemporalPrecision, perfect.TemporalPrecision)
	}
	if early.Mistimed != 1 {
		t.Errorf("Mistimed = %d, want 1", early.Mistimed)
	}
}

// A recurrence and a single appearance with the same amount of screen time and the same
// detector competence must score alike. Where the gap falls is not a detector property.
func TestRecurrenceAndAppearanceAreScoredAlike(t *testing.T) {
	split, _ := transitionCase(t, "recur", "X..X..", "X..X..")
	once, _ := transitionCase(t, "appear", "....XX", "....XX")
	if !near(split.TemporalPrecision, once.TemporalPrecision) ||
		!near(split.TemporalRecall, once.TemporalRecall) {
		t.Fatalf("two frames on screen scored differently depending on whether they were "+
			"adjacent: recurrence P %.3f R %.3f, appearance P %.3f R %.3f",
			split.TemporalPrecision, split.TemporalRecall,
			once.TemporalPrecision, once.TemporalRecall)
	}
}
