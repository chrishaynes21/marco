package visionbench_test

import (
	"image"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// The measuring stick, tested against sequences whose truth is known by construction.
//
// Synthetic on purpose. A metric validated only on real frames is validated against whatever
// the annotator happened to mark, and the failure this replaces — a score that moved the
// wrong way — was invisible for three milestones precisely because nobody had a case where
// the right answer was known in advance.

const frameW, frameH = 1920, 1080

func fb() image.Rectangle { return image.Rect(0, 0, frameW, frameH) }

// px turns a normalised rectangle into pixels the way the evaluator does.
func px(x, y, w, h float64) image.Rectangle {
	return visionbench.NormRect{X: x, Y: y, W: w, H: h}.Pixels(fb())
}

// sequence builds n frames of one named run, with the given truth applied to each.
func sequence(name string, n int, build func(i int) visionbench.FrameTruth) []visionbench.FrameTruth {
	out := make([]visionbench.FrameTruth, 0, n)
	for i := range n {
		ft := build(i)
		ft.Schema = visionbench.GroundTruthSchema
		ft.Sequence = name
		ft.Index = i
		out = append(out, ft)
	}
	return out
}

func frameName(seq string, i int) string {
	return seq + "-" + string(rune('a'+i))
}

// boundsFor gives every frame the same size.
func boundsFor(truths []visionbench.FrameTruth) map[string]image.Rectangle {
	out := map[string]image.Rectangle{}
	for _, t := range truths {
		out[t.Key()] = fb()
	}
	return out
}

// ── the case version 1 got backwards ──────────────────────────────────────────

func TestPersistentFalseStructureIsPenalisedNotRewarded(t *testing.T) {
	// THE test. Version 1 rewarded a rectangle for persisting and called it trust, so a
	// detector that clung to a patch of arena texture for the whole sequence scored well.
	//
	// Two detectors over the same ten-frame sequence containing one real button:
	//   honest — finds the button, every frame
	//   sticky — finds the button AND a patch of declared scenery, every frame
	//
	// Both are perfectly persistent. Only one is persistently RIGHT.
	const seq = "arena"
	truths := sequence(seq, 10, func(i int) visionbench.FrameTruth {
		return visionbench.FrameTruth{
			Frame:            frameName(seq, i),
			InterfacePresent: true,
			Regions: []visionbench.TruthRegion{{
				Kind:     visionbench.TruthButton,
				Bounds:   visionbench.NormRect{X: 0.40, Y: 0.30, W: 0.15, H: 0.06},
				Identity: "resume",
			}},
			NegativeRegions: []visionbench.NegativeRegion{{
				Kind:   visionbench.NegScenery,
				Bounds: visionbench.NormRect{X: 0.05, Y: 0.60, W: 0.30, H: 0.25},
			}},
		}
	})

	honest, sticky := map[string][]visionbench.Detection{}, map[string][]visionbench.Detection{}
	button := visionbench.Detection{Label: "button", Confidence: 0.9,
		Bounds: px(0.40, 0.30, 0.15, 0.06)}
	texture := visionbench.Detection{Label: "panel", Confidence: 0.9,
		Bounds: px(0.10, 0.65, 0.08, 0.08)}
	for _, ft := range truths {
		honest[ft.Key()] = []visionbench.Detection{button}
		sticky[ft.Key()] = []visionbench.Detection{button, texture}
	}

	bounds := boundsFor(truths)
	hm := visionbench.EvaluateTruth(honest, bounds, truths)
	sm := visionbench.EvaluateTruth(sticky, bounds, truths)

	// Both persist. That is the premise, and if it stops holding the test is vacuous.
	if hm.PersistentTracks == 0 || sm.PersistentTracks <= hm.PersistentTracks {
		t.Fatalf("the sticky detector is not more persistent (honest %d tracks, sticky %d); "+
			"this test no longer exercises the defect",
			hm.PersistentTracks, sm.PersistentTracks)
	}
	// And temporal PRECISION separates them, which is the whole point.
	if sm.TemporalPrecision >= hm.TemporalPrecision {
		t.Errorf("temporal precision: honest %.2f, sticky %.2f — persistent false "+
			"structure is still being credited", hm.TemporalPrecision, sm.TemporalPrecision)
	}

	hs, ok1 := visionbench.ScoreV2(hm, 0, visionbench.DefaultWeightsV2())
	ss, ok2 := visionbench.ScoreV2(sm, 0, visionbench.DefaultWeightsV2())
	if !ok1 || !ok2 {
		t.Fatal("ScoreV2 reported unavailable on a corpus that declares ground truth")
	}
	if ss.Total >= hs.Total {
		t.Fatalf("the sticky detector scored %.1f against the honest detector's %.1f; "+
			"ScoreV2 is still anti-correlated with precision", ss.Total, hs.Total)
	}
}

// ── the loopholes each paired metric closes ───────────────────────────────────

func TestSpammingButtonLabelsDestroysNameablePrecision(t *testing.T) {
	// Coverage could be inflated by relabelling: call everything a button and the
	// nameable number rises. Precision asks whether the claim was true.
	const seq = "menu"
	truths := sequence(seq, 4, func(i int) visionbench.FrameTruth {
		return visionbench.FrameTruth{
			Frame:            frameName(seq, i),
			InterfacePresent: true,
			Regions: []visionbench.TruthRegion{
				{Kind: visionbench.TruthButton, Identity: "ok",
					Bounds: visionbench.NormRect{X: 0.4, Y: 0.3, W: 0.15, H: 0.06}},
				{Kind: visionbench.TruthPanel, Identity: "dialog",
					Bounds: visionbench.NormRect{X: 0.3, Y: 0.2, W: 0.4, H: 0.5}},
			},
		}
	})

	spam := map[string][]visionbench.Detection{}
	for _, ft := range truths {
		spam[ft.Key()] = []visionbench.Detection{
			{Label: "button", Bounds: px(0.4, 0.3, 0.15, 0.06)}, // right
			{Label: "button", Bounds: px(0.3, 0.2, 0.4, 0.5)},   // a panel, called a button
		}
	}
	m := visionbench.EvaluateTruth(spam, boundsFor(truths), truths)
	if m.NameablePrecision >= 0.75 {
		t.Errorf("nameable precision %.2f while half the button claims landed on a panel",
			m.NameablePrecision)
	}
}

func TestSeeingAlmostNothingCannotWinOnPrecisionAlone(t *testing.T) {
	// The classical size filter's failure mode: it removed 75% of false positives by
	// deleting real controls, and a precision-only score would have applauded.
	const seq = "menu"
	truths := sequence(seq, 4, func(i int) visionbench.FrameTruth {
		return visionbench.FrameTruth{
			Frame:            frameName(seq, i),
			InterfacePresent: true,
			Regions: []visionbench.TruthRegion{
				{Kind: visionbench.TruthButton, Identity: "a",
					Bounds: visionbench.NormRect{X: 0.4, Y: 0.20, W: 0.15, H: 0.06}},
				{Kind: visionbench.TruthButton, Identity: "b",
					Bounds: visionbench.NormRect{X: 0.4, Y: 0.30, W: 0.15, H: 0.06}},
				{Kind: visionbench.TruthButton, Identity: "c",
					Bounds: visionbench.NormRect{X: 0.4, Y: 0.40, W: 0.15, H: 0.06}},
				{Kind: visionbench.TruthButton, Identity: "d",
					Bounds: visionbench.NormRect{X: 0.4, Y: 0.50, W: 0.15, H: 0.06}},
			},
		}
	})

	timid, thorough := map[string][]visionbench.Detection{}, map[string][]visionbench.Detection{}
	for _, ft := range truths {
		timid[ft.Key()] = []visionbench.Detection{
			{Label: "button", Bounds: px(0.4, 0.20, 0.15, 0.06)},
		}
		thorough[ft.Key()] = []visionbench.Detection{
			{Label: "button", Bounds: px(0.4, 0.20, 0.15, 0.06)},
			{Label: "button", Bounds: px(0.4, 0.30, 0.15, 0.06)},
			{Label: "button", Bounds: px(0.4, 0.40, 0.15, 0.06)},
			{Label: "button", Bounds: px(0.4, 0.50, 0.15, 0.06)},
		}
	}
	bounds := boundsFor(truths)
	tm := visionbench.EvaluateTruth(timid, bounds, truths)
	th := visionbench.EvaluateTruth(thorough, bounds, truths)

	if tm.Precision < 0.99 || th.Precision < 0.99 {
		t.Fatalf("both detectors should be perfectly precise: timid %.2f thorough %.2f",
			tm.Precision, th.Precision)
	}
	if tm.Recall >= th.Recall {
		t.Fatalf("recall did not separate them: timid %.2f thorough %.2f",
			tm.Recall, th.Recall)
	}
	ts, _ := visionbench.ScoreV2(tm, 0, visionbench.DefaultWeightsV2())
	hs, _ := visionbench.ScoreV2(th, 0, visionbench.DefaultWeightsV2())
	if ts.Total >= hs.Total {
		t.Fatalf("the detector that saw one of four controls scored %.1f against %.1f; "+
			"precision without recall is winning", ts.Total, hs.Total)
	}
}

func TestRediscoveringTheSameHUDCostsTemporalRecall(t *testing.T) {
	// A detector that finds the HUD in every frame but somewhere slightly different each
	// time has not tracked it. Version 1's coarse ninths hid exactly this.
	const seq = "hud"
	truths := sequence(seq, 8, func(i int) visionbench.FrameTruth {
		return visionbench.FrameTruth{
			Frame:            frameName(seq, i),
			InterfacePresent: true,
			Regions: []visionbench.TruthRegion{{
				Kind: visionbench.TruthBar, Identity: "boost",
				Bounds: visionbench.NormRect{X: 0.80, Y: 0.85, W: 0.12, H: 0.04},
			}},
		}
	})

	steady, wandering := map[string][]visionbench.Detection{}, map[string][]visionbench.Detection{}
	for i, ft := range truths {
		steady[ft.Key()] = []visionbench.Detection{
			{Label: "bar", Bounds: px(0.80, 0.85, 0.12, 0.04)},
		}
		// Drifts far enough each frame to land in a different track cell.
		wandering[ft.Key()] = []visionbench.Detection{
			{Label: "bar", Bounds: px(0.10+float64(i)*0.09, 0.10, 0.12, 0.04)},
		}
	}
	bounds := boundsFor(truths)
	sm := visionbench.EvaluateTruth(steady, bounds, truths)
	wm := visionbench.EvaluateTruth(wandering, bounds, truths)

	if sm.TemporalRecall <= wm.TemporalRecall {
		t.Errorf("temporal recall: steady %.2f, wandering %.2f — tracking the same control "+
			"earns nothing", sm.TemporalRecall, wm.TemporalRecall)
	}
}

func TestAFrameDeclaringNoInterfaceMakesEveryDetectionFalse(t *testing.T) {
	// The cheapest and strongest annotation, and the one the v1 manifest already carried
	// in prose without the benchmark using it.
	const seq = "empty"
	truths := sequence(seq, 3, func(i int) visionbench.FrameTruth {
		return visionbench.FrameTruth{Frame: frameName(seq, i), InterfacePresent: false}
	})
	dets := map[string][]visionbench.Detection{}
	for _, ft := range truths {
		dets[ft.Key()] = []visionbench.Detection{
			{Label: "panel", Bounds: px(0.1, 0.1, 0.2, 0.2)},
			{Label: "button", Bounds: px(0.5, 0.5, 0.1, 0.05)},
		}
	}
	m := visionbench.EvaluateTruth(dets, boundsFor(truths), truths)
	if m.FalsePos != 6 {
		t.Errorf("%d false positives on three empty frames with two detections each, want 6",
			m.FalsePos)
	}
	if m.Precision != 0 {
		t.Errorf("precision %.2f on frames declared to contain nothing", m.Precision)
	}
}

func TestAnUnmatchedDetectionIsNeitherCreditedNorPunished(t *testing.T) {
	// The annotation is partial by design. A box nobody marked might be real structure,
	// so counting it either way would be a claim the corpus cannot support.
	const seq = "partial"
	truths := sequence(seq, 2, func(i int) visionbench.FrameTruth {
		return visionbench.FrameTruth{
			Frame:            frameName(seq, i),
			InterfacePresent: true,
			Regions: []visionbench.TruthRegion{{
				Kind: visionbench.TruthButton, Identity: "ok",
				Bounds: visionbench.NormRect{X: 0.4, Y: 0.3, W: 0.15, H: 0.06},
			}},
		}
	})
	dets := map[string][]visionbench.Detection{}
	for _, ft := range truths {
		dets[ft.Key()] = []visionbench.Detection{
			{Label: "button", Bounds: px(0.4, 0.3, 0.15, 0.06)}, // matches
			{Label: "panel", Bounds: px(0.05, 0.05, 0.1, 0.1)},  // marked nowhere
		}
	}
	m := visionbench.EvaluateTruth(dets, boundsFor(truths), truths)
	if m.Unmatched != 2 {
		t.Errorf("unmatched = %d, want 2", m.Unmatched)
	}
	if m.Precision != 1 {
		t.Errorf("precision %.2f — an unmatched box was counted against a detector that "+
			"made no wrong claim", m.Precision)
	}
}

func TestADetectionInsideDeclaredSceneryIsFalseEvenOnAnInterfaceFrame(t *testing.T) {
	// Negative regions are the half version 1 could not express, and the case they exist
	// for is a frame that DOES contain interface — a HUD over an arena. Without this the
	// scenery box is merely "unmatched", which is forgiven rather than penalised.
	const seq = "hud-over-arena"
	truths := sequence(seq, 3, func(i int) visionbench.FrameTruth {
		return visionbench.FrameTruth{
			Frame:            frameName(seq, i),
			InterfacePresent: true,
			Regions: []visionbench.TruthRegion{{
				Kind: visionbench.TruthBar, Identity: "boost",
				Bounds: visionbench.NormRect{X: 0.80, Y: 0.85, W: 0.12, H: 0.04},
			}},
			NegativeRegions: []visionbench.NegativeRegion{{
				Kind:   visionbench.NegScenery,
				Bounds: visionbench.NormRect{X: 0.05, Y: 0.20, W: 0.40, H: 0.40},
			}},
		}
	})
	dets := map[string][]visionbench.Detection{}
	for _, ft := range truths {
		dets[ft.Key()] = []visionbench.Detection{
			{Label: "bar", Bounds: px(0.80, 0.85, 0.12, 0.04)},   // the real HUD bar
			{Label: "panel", Bounds: px(0.15, 0.30, 0.10, 0.10)}, // deep inside the scenery
		}
	}
	m := visionbench.EvaluateTruth(dets, boundsFor(truths), truths)
	if m.FalsePos != 3 {
		t.Fatalf("false positives = %d, want 3 — a box inside declared arena texture is "+
			"not being penalised (unmatched=%d)", m.FalsePos, m.Unmatched)
	}
	if m.Precision >= 1 {
		t.Errorf("precision %.2f while a third of the output was declared scenery",
			m.Precision)
	}
}

func TestEveryScoreDimensionCarriesWeight(t *testing.T) {
	// Guards the weighting itself. A dimension quietly set to zero would leave the score
	// looking healthy while no longer measuring the thing it was added for — and
	// structural precision and recall are exactly the pair that has to stay in tension.
	w := visionbench.DefaultWeightsV2()
	dims := map[string]float64{
		"structural precision": w.StructuralPrecision,
		"structural recall":    w.StructuralRecall,
		"nameable precision":   w.NameablePrecision,
		"nameable recall":      w.NameableRecall,
		"temporal precision":   w.TemporalPrecision,
		"temporal recall":      w.TemporalRecall,
		"OCR-region precision": w.OCRPrecision,
		"latency":              w.Latency,
	}
	total := 0.0
	for name, v := range dims {
		if v <= 0 {
			t.Errorf("dimension %q carries no weight", name)
		}
		total += v
	}
	if total != 100 {
		t.Errorf("weights sum to %.0f, not 100 — scores are no longer out of 100", total)
	}
	// Precision must outweigh its recall partner throughout: this detector feeds a system
	// that acts on what it believes, and a confident wrong box is worse than a missing one.
	if w.StructuralPrecision <= w.StructuralRecall {
		t.Error("structural recall outweighs precision")
	}
	if w.TemporalPrecision <= w.TemporalRecall {
		t.Error("temporal recall outweighs precision — the anti-correlation this score " +
			"exists to fix is on the precision side")
	}
}

// ── the legacy corpus is refused, not scored zero ──────────────────────────────

func TestACorpusWithoutGroundTruthReportsUnavailableNotZero(t *testing.T) {
	// The legacy crop corpus cannot support ScoreV2. Reporting zero would rank a
	// perfectly good detector last for the corpus's failing.
	_, ok := visionbench.ScoreV2(visionbench.TruthMetrics{}, 0, visionbench.DefaultWeightsV2())
	if ok {
		t.Fatal("ScoreV2 claimed to measure a corpus that declares no ground truth")
	}
}

func TestLatencyStillCosts(t *testing.T) {
	m := visionbench.TruthMetrics{TruthRegions: 1, Matched: 1, TruePos: 1}
	fast, _ := visionbench.ScoreV2(m, 5*time.Millisecond, visionbench.DefaultWeightsV2())
	slow, _ := visionbench.ScoreV2(m, 9*time.Second, visionbench.DefaultWeightsV2())
	if slow.Total >= fast.Total {
		t.Errorf("a 9s detector scored %.1f against a 5ms one's %.1f", slow.Total, fast.Total)
	}
}

// ── schema validation ─────────────────────────────────────────────────────────

func TestAnnotationProblemsAreAllReportedAtOnce(t *testing.T) {
	bad := visionbench.FrameTruth{
		Schema: 99, Frame: "", Sequence: "",
		Regions: []visionbench.TruthRegion{
			{Kind: "boost_meter", Bounds: visionbench.NormRect{X: 0.1, Y: 0.1, W: 0.1, H: 0.1}},
			{Kind: visionbench.TruthButton, Bounds: visionbench.NormRect{X: 0.9, Y: 0.9, W: 0.5, H: 0.5}},
		},
	}
	problems := bad.Validate()
	if len(problems) < 4 {
		t.Fatalf("%d problems reported for a frame with a bad schema, no name, no "+
			"sequence, a game-specific kind and out-of-frame bounds: %v",
			len(problems), problems)
	}
}

func TestAFrameCannotDeclareNoInterfaceAndAlsoAnnotateIt(t *testing.T) {
	contradictory := visionbench.FrameTruth{
		Schema: visionbench.GroundTruthSchema, Frame: "f", Sequence: "s",
		InterfacePresent: false,
		Regions: []visionbench.TruthRegion{{
			Kind:   visionbench.TruthButton,
			Bounds: visionbench.NormRect{X: 0.1, Y: 0.1, W: 0.1, H: 0.1},
		}},
	}
	if len(contradictory.Validate()) == 0 {
		t.Fatal("a frame declaring no interface while annotating a button was accepted")
	}
}

func TestASequenceWithAGapIsReported(t *testing.T) {
	// Frames 0, 1, 3. Temporal metrics across the gap would treat 1 and 3 as adjacent.
	truths := []visionbench.FrameTruth{
		{Schema: 1, Frame: "a", Sequence: "s", Index: 0},
		{Schema: 1, Frame: "b", Sequence: "s", Index: 1},
		{Schema: 1, Frame: "d", Sequence: "s", Index: 3},
	}
	if _, problems := visionbench.Sequences(truths); len(problems) == 0 {
		t.Fatal("a sequence missing index 2 was accepted silently")
	}
}

func TestGameSpecificKindsAreNotInTheVocabulary(t *testing.T) {
	// The corpus evaluates visual structure, not game-pack interpretation. A vocabulary
	// that admitted "boost" would make the benchmark a test of Rocket League.
	for _, k := range []visionbench.TruthKind{"boost", "health", "inventory", "scoreboard", "pause"} {
		if k.Known() {
			t.Errorf("%q is in the ground-truth vocabulary", k)
		}
	}
}

// ── the two corpus defects, as regressions ────────────────────────────────────
//
// Both were found by a benchmark result that looked wrong, not by a test. Manual correctness
// has now demonstrably been insufficient twice on this corpus.

func TestTwoSequencesMaySharePicturesButNotIdentities(t *testing.T) {
	// Corpus v2 deliberately copies boundary frames into both pause-stable and the
	// transition sequences — one picture, two temporal contexts, both legitimate. What
	// silently broke was the LOOKUP: every map was keyed by basename, so one annotation won
	// and pause-open lost every menu frame it had, scoring 0 true positives on frames that
	// plainly contain a menu.
	shared := "pause-cycle-029"
	truths := []visionbench.FrameTruth{
		{Schema: 1, Frame: shared, Sequence: "pause-open", Index: 0},
		{Schema: 1, Frame: shared, Sequence: "pause-stable", Index: 0},
	}
	if problems := visionbench.DuplicateKeys(truths); len(problems) != 0 {
		t.Fatalf("sharing an image between two sequences was rejected: %v", problems)
	}
	if truths[0].Key() == truths[1].Key() {
		t.Fatalf("both frames resolve to identity %q; a basename key aliases them and one "+
			"sequence's truth is silently discarded", truths[0].Key())
	}

	// And a genuine collision — the same frame twice in one sequence — must be caught.
	collide := []visionbench.FrameTruth{
		{Schema: 1, Frame: shared, Sequence: "pause-open", Index: 0},
		{Schema: 1, Frame: shared, Sequence: "pause-open", Index: 1},
	}
	if problems := visionbench.DuplicateKeys(collide); len(problems) == 0 {
		t.Error("two annotations resolving to one identity were accepted")
	}
}

func TestAnAnnotationInsideAPrivacyMaskIsRefused(t *testing.T) {
	// Seven annotations survived sanitisation describing regions the mask had painted over,
	// and they depressed ScreenParser's recall against evidence that no longer existed.
	// Geometry catches this; class names cannot.
	ft := visionbench.FrameTruth{
		Schema: 1, Frame: "f", Sequence: "s", InterfacePresent: true,
		Regions: []visionbench.TruthRegion{{
			Kind: visionbench.TruthMeter, Identity: "hud_boost",
			Bounds: visionbench.NormRect{X: 0.862, Y: 0.750, W: 0.104, H: 0.180},
		}},
		IgnoreRegions: []visionbench.NormRect{{X: 0, Y: 0.80, W: 1, H: 0.20}},
	}
	if len(ft.Validate()) == 0 {
		t.Fatal("an annotation more than half inside a privacy mask was accepted; this is " +
			"the phantom-truth defect that invalidated a held-out run")
	}
}

func TestADetectionInsideAPrivacyMaskIsNeitherCreditedNorPunished(t *testing.T) {
	// The mask is a flat rectangle THIS PROJECT painted. Scoring a detector against it
	// would measure the sanitiser. It is not scenery — nobody drew it — and it is not
	// interface, because sanitisation destroyed whatever was.
	const seq = "masked"
	truths := sequence(seq, 3, func(i int) visionbench.FrameTruth {
		return visionbench.FrameTruth{
			Frame:            frameName(seq, i),
			InterfacePresent: true,
			Regions: []visionbench.TruthRegion{{
				Kind: visionbench.TruthButton, Identity: "ok",
				Bounds: visionbench.NormRect{X: 0.4, Y: 0.3, W: 0.15, H: 0.06},
			}},
			IgnoreRegions: []visionbench.NormRect{{X: 0, Y: 0.80, W: 1, H: 0.20}},
		}
	})
	dets := map[string][]visionbench.Detection{}
	for _, ft := range truths {
		dets[ft.Key()] = []visionbench.Detection{
			{Label: "button", Bounds: px(0.4, 0.3, 0.15, 0.06)}, // the real control
			{Label: "panel", Bounds: px(0.2, 0.85, 0.4, 0.10)},  // inside the mask
		}
	}
	m := visionbench.EvaluateTruth(dets, boundsFor(truths), truths)
	if m.Detections != 3 {
		t.Fatalf("%d detections scored, want 3 — the masked box should have been dropped "+
			"before scoring", m.Detections)
	}
	if m.Precision != 1 {
		t.Errorf("precision %.2f — a detector was penalised for finding the mask", m.Precision)
	}
}
