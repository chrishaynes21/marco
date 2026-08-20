package visionbench

import (
	"fmt"
	"time"
)

// ScoreV2, and why the old weights were not kept for continuity.
//
// Version 1's score was measurably anti-correlated with precision on its own corpus: removing
// 75% of a detector's false positives made the total DROP from 55.2 to 50.9, because
// temporal persistence carried 40 of 100 points and persistence was being measured by a proxy
// that rewarded volume. A score that moves the wrong way is worse than no score, because it
// is used to choose.
//
// So the dimensions here are all ratios against declared truth, and every one of them has a
// matching pair that closes a loophole:
//
//	structural precision  20  is what it found really there
//	structural recall     15  did it find what IS there — the pair matters, because the
//	                          classical size filter won on precision by deleting real
//	                          controls, and a precision-only score would have called that
//	                          an improvement
//	nameable precision    15  when it says `button`, is it a button — closes the
//	                          coverage loophole, where relabelling boxes inflates the number
//	nameable recall       10  did it name the things that can be named
//	temporal precision    20  of what persisted, how much was really there. THE fix: a
//	                          false rectangle that survives a hundred frames now costs
//	                          points instead of earning them
//	temporal recall       10  did persistent real interface stay tracked, or was the same
//	                          HUD rediscovered as a new thing every frame
//	OCR-region precision   5  were the regions proposed for reading worth reading
//	latency                5  a sample slower than its interval is a dropped sample
//
// Detection COUNT carries zero, as before. Recall is deliberately weighted BELOW precision
// throughout: this detector feeds a system that acts on what it believes, and a confident
// wrong box is worse than a missing one.

// WeightsV2 are ScoreV2's dimensions. They sum to 100.
type WeightsV2 struct {
	StructuralPrecision float64
	StructuralRecall    float64
	NameablePrecision   float64
	NameableRecall      float64
	TemporalPrecision   float64
	TemporalRecall      float64
	OCRPrecision        float64
	Latency             float64
	LatencyBudget       time.Duration
}

// DefaultWeightsV2 are documented above.
func DefaultWeightsV2() WeightsV2 {
	return WeightsV2{
		StructuralPrecision: 20, StructuralRecall: 15,
		NameablePrecision: 15, NameableRecall: 10,
		TemporalPrecision: 20, TemporalRecall: 10,
		OCRPrecision: 5, Latency: 5,
		LatencyBudget: 500 * time.Millisecond,
	}
}

// ScoreV2 rates a backend against declared ground truth, out of 100.
//
// Returns ok=false when the corpus cannot support the measurement — which is the honest
// answer for the legacy crop corpus, and is reported as "unavailable" rather than as zero. A
// fabricated zero would rank a perfectly good detector last for the corpus's failing.
func ScoreV2(m TruthMetrics, latency time.Duration, w WeightsV2) (ScoreBreakdown, bool) {
	var b ScoreBreakdown
	if m.TruthRegions == 0 && m.FalsePos == 0 {
		b.Contributions = append(b.Contributions, Contribution{
			Dimension: "unavailable", Points: 0,
			Because: "this corpus declares no ground truth, so precision and recall " +
				"cannot be measured against it",
		})
		return b, false
	}

	add := func(dim string, fraction, weight float64, because string) {
		points := clamp01(fraction) * weight
		b.Total += points
		b.Contributions = append(b.Contributions, Contribution{
			Dimension: dim, Points: points, Because: because,
		})
	}

	add("structural precision", m.Precision, w.StructuralPrecision,
		fmt.Sprintf("%d of %d judged detections were really there",
			m.TruePos, m.TruePos+m.FalsePos))
	add("structural recall", m.Recall, w.StructuralRecall,
		fmt.Sprintf("%d of %d declared regions were found", m.Matched, m.TruthRegions))

	add("nameable precision", m.NameablePrecision, w.NameablePrecision,
		fmt.Sprintf("%d of %d nameable-role claims landed on a nameable control",
			m.NameableCorrect, m.NameableClaimed))
	add("nameable recall", m.NameableRecall, w.NameableRecall,
		fmt.Sprintf("%d of %d nameable controls were found",
			m.NameableFound, m.NameableTruth))

	add("temporal precision", m.TemporalPrecision, w.TemporalPrecision,
		fmt.Sprintf("%d of %d persistent tracks were really interface; %d of %d "+
			"transition-track claims were correctly timed (mean over %d sequences)",
			m.PersistentTrue, m.PersistentTracks,
			m.OnTime, m.OnTime+m.Mistimed, m.TemporalPrecisionSequences))
	add("temporal recall", m.TemporalRecall, w.TemporalRecall,
		fmt.Sprintf("%d of %d persistent real controls stayed tracked; %d of %d "+
			"transition frames were tracked (mean over %d sequences)",
			m.PersistentTruthFound, m.PersistentTruthTracks,
			m.OnTime, m.Expected, m.TemporalRecallSequences))

	add("OCR-region precision", m.OCRPrecision, w.OCRPrecision,
		fmt.Sprintf("%d of %d proposed reading regions were worth reading",
			m.OCRCorrect, m.OCRProposed))

	lat := 1.0
	if w.LatencyBudget > 0 && latency > 0 {
		lat = 1 - float64(latency)/float64(w.LatencyBudget)
	}
	add("latency", lat, w.Latency,
		fmt.Sprintf("median %s against a %s budget",
			latency.Round(time.Millisecond), w.LatencyBudget))

	if b.Total < 0 {
		b.Total = 0
	}
	return b, true
}
