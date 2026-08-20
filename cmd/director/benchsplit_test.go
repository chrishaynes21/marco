package main

import (
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// The split and the temporal declaration, guarded against the mutations that would make them
// decorative.
//
// Both are DATA that changes what a number means, and both are the kind of data that fails
// silently: a held-out run that quietly includes calibration frames still prints a score, and a
// transition sequence scored as static still prints a temporal percentage. Every test here
// exists because the corresponding mutation was applied and the suite had to fail for it.

func corpusPath() string {
	return filepath.Join("..", "..", "fixtures", "vision", "v2", "rocketleague")
}

func loadCorpus(t *testing.T) visionbench.V2Corpus {
	t.Helper()
	c, err := visionbench.LoadV2(corpusPath())
	if err != nil {
		t.Fatalf("loading the v2 corpus: %v", err)
	}
	return c
}

// Mutation 1: a held-out run that includes calibration frames.
//
// The failure this prevents is not a wrong number, it is a number that cannot be compared to
// the one it will be quoted beside. A full-corpus 69.5 was once read as comparable to a
// held-out 63%.
func TestHeldOutExcludesCalibrationSequences(t *testing.T) {
	c := loadCorpus(t)
	for _, seq := range c.SequenceNames() {
		role := splitRole(seq)
		if role != "calibration" && role != "held-out" {
			t.Fatalf("sequence %q has role %q, which is neither split", seq, role)
		}
	}
	if len(calibrationSequences) == 0 {
		t.Fatal("no calibration sequences are declared — every sequence would be reported " +
			"as held-out, and a threshold frozen on the corpus would be judged on it")
	}

	held := c.Subset(func(s string) bool { return splitRole(s) == "held-out" })
	cal := c.Subset(func(s string) bool { return splitRole(s) == "calibration" })
	if len(held) == 0 || len(cal) == 0 {
		t.Fatalf("split populations are %d held-out / %d calibration; both must be "+
			"non-empty for the split to mean anything", len(held), len(cal))
	}
	for _, ft := range held {
		if calibrationSequences[ft.Sequence] {
			t.Errorf("held-out frame %s belongs to calibration sequence %q",
				ft.Key(), ft.Sequence)
		}
	}
	if len(held)+len(cal) != len(c.Truths) {
		t.Errorf("the two splits cover %d of %d frames — a frame in neither would be "+
			"silently unscored", len(held)+len(cal), len(c.Truths))
	}
}

// Mutation 2: frame identity falls back to basename.
//
// Four boundary frames legitimately appear in two sequences each. Keyed by basename, one
// annotation silently won and `pause-open` scored zero true positives on frames that plainly
// contain a menu.
func TestFrameIdentityIsSequenceScoped(t *testing.T) {
	c := loadCorpus(t)
	byBasename := map[string]int{}
	for _, ft := range c.Truths {
		byBasename[ft.Frame]++
	}
	shared := 0
	for _, n := range byBasename {
		if n > 1 {
			shared++
		}
	}
	if shared == 0 {
		t.Skip("no frame is shared between sequences, so this corpus cannot exercise the bug")
	}
	// Every shared image must still resolve to a distinct lookup key with its own bounds.
	seen := map[string]bool{}
	for _, ft := range c.Truths {
		if seen[ft.Key()] {
			t.Errorf("two annotations resolve to key %q", ft.Key())
		}
		seen[ft.Key()] = true
		if _, ok := c.Bounds[ft.Key()]; !ok {
			t.Errorf("no frame bounds registered for %q — a basename-keyed lookup would "+
				"have aliased it onto another sequence's frame", ft.Key())
		}
	}
	if len(c.Bounds) != len(c.Truths) {
		t.Errorf("%d frame identities but %d bounds entries; %d shared images collapsed",
			len(c.Truths), len(c.Bounds), shared)
	}
}

// Mutation 3: the split declaration is emptied.
//
// A held-out run whose split declaration is missing must not quietly become a full-corpus run.
func TestSplitDeclarationIsLoadBearing(t *testing.T) {
	c := loadCorpus(t)
	real := c.Subset(func(s string) bool { return splitRole(s) == "held-out" })

	// Simulate the mutation: no sequence is declared calibration.
	empty := map[string]bool{}
	role := func(s string) string {
		if empty[s] {
			return "calibration"
		}
		return "held-out"
	}
	mutated := c.Subset(func(s string) bool { return role(s) == "held-out" })
	if len(mutated) == len(real) {
		t.Fatal("dropping the calibration declaration did not change the held-out " +
			"population — the split is not actually scoping the evaluation")
	}
}

// Mutation 4: sequence modes are ignored, so everything is scored as static.
func TestSequenceModeChangesTheResult(t *testing.T) {
	c := loadCorpus(t)
	if len(c.Modes) != len(c.SequenceNames()) {
		t.Fatalf("%d sequences but %d declared modes — a sequence with no declaration "+
			"would be scored as static without saying so",
			len(c.SequenceNames()), len(c.Modes))
	}
	transitions := 0
	for name, m := range c.Modes {
		frames := len(c.Subset(func(s string) bool { return s == name }))
		for _, tr := range m.Tracks {
			if tr.Frames() < frames {
				transitions++
				break
			}
		}
	}
	if transitions == 0 {
		t.Fatal("no sequence declares a transition, so this corpus cannot detect the bug")
	}

	// A perfect detector cannot expose this mutation: exclusion-not-zero already keeps the
	// static rule's 0/0 sequence out of the mean, so both readings come out at 1.0. What the
	// static rule cannot see is TIMING, so the probe is a detector that gets the timing wrong.
	dets := alwaysOnDetections(c)
	honoured := visionbench.EvaluateTruthModes(dets, c.Bounds, c.Truths, c.Modes)
	ignored := visionbench.EvaluateTruth(dets, c.Bounds, c.Truths)

	if honoured.TransitionTracks == 0 {
		t.Fatal("no transition tracks were scored despite two declared transitions")
	}
	if ignored.TransitionTracks != 0 {
		t.Fatal("the static reading produced transition tracks — the modes were not ignored")
	}
	if honoured.Mistimed == 0 {
		t.Fatal("a detector that reports the menu in every frame of both transitions was " +
			"charged with no mistimed frames at all")
	}
	if honoured.TemporalPrecision >= ignored.TemporalPrecision {
		t.Fatalf("declared semantics scored the always-on detector %.3f against the static "+
			"rule's %.3f — honouring the modes must cost it precision, not save it",
			honoured.TemporalPrecision, ignored.TemporalPrecision)
	}
	// Note what is NOT asserted here any more. Before the corpus was repaired, pause-close
	// held its menu in 2 of 6 frames, fell under the static majority threshold, contributed
	// no temporal track at all, and was rendered as 0%. The repaired sequence is present in
	// 4 of 6, so it now clears that threshold and both readings score the same population.
	//
	// The defect has not stopped mattering — the corpus simply no longer happens to
	// demonstrate it. It is demonstrated on synthetic evidence instead, where detector
	// quality can be held exactly equal, by TestOldStaticRuleSplitTheRealCorpusShape and
	// TestRecurringPresenceScoresLikeAnyOtherShape. A regression that depends on a corpus
	// keeping an awkward shape is a regression that disappears the moment the corpus is
	// fixed.
	if honoured.TemporalRecallSequences == 0 {
		t.Error("no sequence contributed a temporal recall opportunity")
	}
}

// Mutation 5: the declared boundary is ignored (treated as zero).
func TestTransitionBoundaryIsLoadBearing(t *testing.T) {
	c := loadCorpus(t)
	// Again the probe has to be a mistimed detector. The intervals only decide which frames
	// are the wrong ones to claim, and a detector that never claims a wrong frame cannot
	// tell whether they were read.
	dets := alwaysOnDetections(c)

	// The mutation: widen every declaration to the whole sequence, so nothing is ever
	// asserted absent. This is what "the intervals are decorative" looks like.
	blunted := map[string]visionbench.SequenceTruth{}
	for name, m := range c.Modes {
		frames := len(c.Subset(func(s string) bool { return s == name }))
		widened := make([]visionbench.TrackTruth, 0, len(m.Tracks))
		for _, tr := range m.Tracks {
			widened = append(widened, visionbench.TrackTruth{
				Identity: tr.Identity,
				Present:  []visionbench.Span{{From: 0, To: frames - 1}},
			})
		}
		m.Tracks = widened
		blunted[name] = m
	}
	real := visionbench.EvaluateTruthModes(dets, c.Bounds, c.Truths, c.Modes)
	mutated := visionbench.EvaluateTruthModes(dets, c.Bounds, c.Truths, blunted)

	if real.Mistimed <= mutated.Mistimed {
		t.Fatalf("zeroing every transition boundary charged %d mistimed frames against the "+
			"declared %d — the boundary is not deciding which frames are the wrong phase",
			mutated.Mistimed, real.Mistimed)
	}
	if real.TemporalPrecision >= mutated.TemporalPrecision {
		t.Fatalf("a blunted boundary scored precision %.3f against the declared %.3f — "+
			"declaring the transition must be able to convict a mistimed detector",
			mutated.TemporalPrecision, real.TemporalPrecision)
	}
}

// Mutation 6: appearance and disappearance both fall back to majority persistence.
//
// The corpus form of the load-bearing regression: `pause-open` and `pause-close` hold the same
// six identities on opposite sides of a threshold.
func TestCorpusMirrorSequencesScoreAlike(t *testing.T) {
	c := loadCorpus(t)
	dets := perfectDetections(c)

	score := func(seq string, modes map[string]visionbench.SequenceTruth) visionbench.TruthMetrics {
		sub := c.Subset(func(s string) bool { return s == seq })
		return visionbench.EvaluateTruthModes(dets, c.Bounds, sub, modes)
	}
	open := score("pause-open", c.Modes)
	closed := score("pause-close", c.Modes)
	if open.TemporalRecall < 0.999 || closed.TemporalRecall < 0.999 {
		t.Fatalf("a perfect detector scored pause-open %.3f and pause-close %.3f; both "+
			"must be 1.0 — the two sequences are mirror images",
			open.TemporalRecall, closed.TemporalRecall)
	}

	// The property that survives the corpus repair: a detector that holds the menu through
	// the gap must be convicted in pause-close, and only the declared intervals can convict
	// it. The static rule sees 4 of 6 frames either way and cannot tell the two apart.
	stale := alwaysOnDetections(c)
	declaredView := visionbench.EvaluateTruthModes(stale, c.Bounds,
		c.Subset(func(s string) bool { return s == "pause-close" }), c.Modes)
	staticView := visionbench.EvaluateTruthModes(stale, c.Bounds,
		c.Subset(func(s string) bool { return s == "pause-close" }), nil)

	if declaredView.Mistimed == 0 {
		t.Error("a detector claiming the menu on the one frame it is absent was charged " +
			"with no mistimed frames")
	}
	if declaredView.TemporalPrecision >= staticView.TemporalPrecision {
		t.Errorf("declared intervals scored the gap-claiming detector %.3f against the "+
			"static rule's %.3f — only the declaration can see the gap, so honouring it "+
			"must cost the detector precision",
			declaredView.TemporalPrecision, staticView.TemporalPrecision)
	}
}

// Mutation 7 is covered in the unit suite by TestMirroredTransitionsScoreIdentically, on
// synthetic mirrors where detector quality can be held exactly equal.

// perfectDetections synthesises a detector that finds every declared region exactly, and
// nothing else. The reference point every temporal assertion above is measured against.
func perfectDetections(c visionbench.V2Corpus) map[string][]visionbench.Detection {
	out := map[string][]visionbench.Detection{}
	for _, ft := range c.Truths {
		fb := c.Bounds[ft.Key()]
		var dets []visionbench.Detection
		for _, r := range ft.Regions {
			dets = append(dets, visionbench.Detection{
				Label: string(r.Kind), Confidence: 0.9, Bounds: r.Bounds.Pixels(fb),
			})
		}
		out[ft.Key()] = dets
	}
	return out
}

// alwaysOnDetections synthesises a detector that reports every identity its sequence ever
// contains, in every frame of that sequence.
//
// Structurally it is excellent — it finds everything that is there. Its only fault is timing:
// it claims the menu before it opens and after it closes. That makes it the exact probe for
// whether the temporal metric is reading the declaration or ignoring it.
func alwaysOnDetections(c visionbench.V2Corpus) map[string][]visionbench.Detection {
	type region struct {
		kind   visionbench.TruthKind
		bounds visionbench.NormRect
	}
	bySeq := map[string]map[string]region{}
	for _, ft := range c.Truths {
		if bySeq[ft.Sequence] == nil {
			bySeq[ft.Sequence] = map[string]region{}
		}
		for _, r := range ft.Regions {
			id := r.Identity
			if id == "" {
				id = string(r.Kind)
			}
			bySeq[ft.Sequence][id] = region{r.Kind, r.Bounds}
		}
	}
	out := map[string][]visionbench.Detection{}
	for _, ft := range c.Truths {
		fb := c.Bounds[ft.Key()]
		var dets []visionbench.Detection
		for _, r := range bySeq[ft.Sequence] {
			dets = append(dets, visionbench.Detection{
				Label: string(r.kind), Confidence: 0.9, Bounds: r.bounds.Pixels(fb),
			})
		}
		out[ft.Key()] = dets
	}
	return out
}
