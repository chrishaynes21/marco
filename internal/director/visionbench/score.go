package visionbench

import (
	"fmt"
	"time"
)

// Turning measurements into one number, and showing the working.
//
// A single score is what makes "is this model better" answerable, and it is also the easiest
// thing in the world to get wrong — a weighting nobody can see is an opinion wearing a
// number's clothes. So every dimension states its weight, and every result reports how many
// points each dimension actually contributed and why.
//
// # The weights, and the reasoning behind each
//
// They are not equal, because the things they measure are not equally load-bearing:
//
//	structural usefulness  20  Fusion can only attach text to STRUCTURE. A backend that
//	                           finds no structure gives the Director geometry and nothing
//	                           else, whatever else it does well.
//	nameable roles         10  Of that structure, how much carries a role a request may say
//	                           in plaintext. Split OUT of structural usefulness rather than
//	                           added on top, because structure that cannot be named was
//	                           being paid for twice — see below.
//	temporal stability     25  Identity across frames is what makes a timeline possible.
//	                           A model that is excellent per-frame and flickers has
//	                           nothing to say about what CHANGED.
//	naming opportunity     20  A request has to be able to name what it acts on. This
//	                           combines low anonymity with regions big enough to read.
//	trustworthiness        15  Structure that fails to persist was not structure. This
//	                           penalises confident wrongness, which is the failure mode
//	                           that does real damage downstream.
//	latency                10  A sample slower than its interval is a dropped sample —
//	                           real, but recoverable by sampling less often, so it is
//	                           weighted below everything that cannot be recovered.
//
// Detection COUNT has weight zero and appears in the report only for context. On a real
// game the baseline emitted 129 anonymous icons and one stable entity; any weighting that
// rewarded that would be measuring the wrong thing.
//
// # Why structural usefulness gave up ten points
//
// It was worth 30 and it flattered a detector that could never name anything: the baseline
// emitted only `icon`, `panel` and `slot`, every one of them structural, and collected the
// full 30 for finding structure the Director is not permitted to speak about. The blind spot
// survived three milestones because no metric asked the second question.
//
// The ten points come OUT of structural usefulness rather than being added beside it, so the
// total stays 100 and — more importantly — so the correction lands where the over-credit
// was. A detector fluent in nameable roles is no better off than before; one fluent only in
// anonymous structure now loses the ten points it should never have had. That is the whole
// intended effect, and it is why this is a re-split rather than a new dimension.

// Weights are the score's dimensions. They sum to 100.
type Weights struct {
	Structural float64
	// Nameable is structure whose ROLE the privacy allowlist permits in plaintext, scored
	// apart from structure itself. See the note above on where its points came from.
	Nameable float64
	Temporal float64
	Naming   float64
	Trust    float64
	Latency  float64
	// LatencyBudget is the median a backend may take before losing latency points
	// entirely. Set from the observation loop's own interval: a detector slower than the
	// gap between samples cannot keep up with the session it feeds.
	LatencyBudget time.Duration
}

// DefaultWeights are documented above.
func DefaultWeights() Weights {
	return Weights{
		Structural: 20, Nameable: 10, Temporal: 25, Naming: 20, Trust: 15, Latency: 10,
		LatencyBudget: 500 * time.Millisecond,
	}
}

// Contribution is one dimension's effect on the total, with its reason.
type Contribution struct {
	Dimension string  `json:"dimension"`
	Points    float64 `json:"points"`
	Because   string  `json:"because"`
}

// ScoreBreakdown is a total and the account of where it came from.
type ScoreBreakdown struct {
	Total         float64        `json:"total"`
	Contributions []Contribution `json:"contributions"`
}

// Score rates a backend's measured usefulness out of 100.
//
// Every dimension is normalised to 0..1 before weighting, so the weights above are the only
// place relative importance is decided — a dimension cannot quietly dominate because its
// raw units happen to be large.
func Score(m Metrics, w Weights) ScoreBreakdown {
	var b ScoreBreakdown
	add := func(dim string, fraction, weight float64, because string) {
		points := clamp01(fraction) * weight
		b.Total += points
		b.Contributions = append(b.Contributions, Contribution{
			Dimension: dim, Points: points, Because: because,
		})
	}

	// A backend that produced nothing scores nothing, and says so rather than scoring
	// well on ratios computed over an empty set.
	if m.Frames == 0 || m.Accepted == 0 {
		b.Contributions = append(b.Contributions, Contribution{
			Dimension: "everything", Points: 0,
			Because: "no detection survived the Director's own acceptance filter",
		})
		return b
	}

	add("structural usefulness", m.StructuralCoverage, w.Structural,
		fmt.Sprintf("%.0f%% of tracked things establish structure", m.StructuralCoverage*100))

	// Reported even at 0, and especially at 0: "0% of tracked things carry a nameable
	// role" beside "100% establish structure" is the sentence that names the blind spot,
	// and a dimension omitted when empty would leave the reader to notice its absence.
	add("nameable roles", m.NameableCoverage, w.Nameable,
		fmt.Sprintf("%.0f%% of tracked things carry a role a request may name (%d of %d "+
			"accepted boxes)", m.NameableCoverage*100, m.NameableDetections, m.Accepted))

	// Persistence and its inverse, flicker, together: a thing can be present in most
	// frames and still be useless if it arrives and leaves constantly.
	temporal := m.Persistence * (1 - clamp01(m.FlickerRate*2))
	add("temporal stability", temporal, w.Temporal,
		fmt.Sprintf("%.0f%% mean presence at %.2f flicker", m.Persistence*100, m.FlickerRate))

	// Naming: how much is NOT anonymous, scaled by whether there were regions worth
	// reading at all. A backend with no readable regions cannot be rescued by OCR.
	regionShare := 0.0
	if m.Accepted > 0 {
		regionShare = float64(m.OCRRegions) / float64(m.Accepted)
	}
	naming := (1-m.AnonymousRatio)*0.5 + clamp01(regionShare)*0.5
	add("naming opportunity", naming, w.Naming,
		fmt.Sprintf("%.0f%% anonymous, %d of %d regions readable",
			m.AnonymousRatio*100, m.OCRRegions, m.Accepted))

	add("trustworthiness", 1-m.FalseStructureRate, w.Trust,
		fmt.Sprintf("%.0f%% of claimed structure failed to persist", m.FalseStructureRate*100))

	latency := 1.0
	if w.LatencyBudget > 0 && m.MedianLatency > 0 {
		latency = 1 - float64(m.MedianLatency)/float64(w.LatencyBudget)
	}
	add("latency", latency, w.Latency,
		fmt.Sprintf("median %s against a %s budget",
			m.MedianLatency.Round(time.Millisecond), w.LatencyBudget))

	// Failures are subtracted rather than scaled: a backend that could not run on some
	// frames has an incomplete result, and the penalty should be visible as its own line
	// rather than diluted through a ratio.
	if m.Failures > 0 {
		penalty := -float64(m.Failures) / float64(m.Frames) * 20
		b.Total += penalty
		b.Contributions = append(b.Contributions, Contribution{
			Dimension: "failures", Points: penalty,
			Because: fmt.Sprintf("%d of %d frames produced nothing", m.Failures, m.Frames),
		})
	}
	if b.Total < 0 {
		b.Total = 0
	}
	return b
}

func clamp01(f float64) float64 {
	switch {
	case f < 0:
		return 0
	case f > 1:
		return 1
	}
	return f
}
