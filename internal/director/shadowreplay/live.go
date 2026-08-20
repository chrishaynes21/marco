package shadowreplay

import (
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Reading a live trace, where nothing tells you where the elements were.
//
// The corpus analysis could anchor on declared ground truth. A live session has none, so the
// question "were these 26 detections four things or twenty-four things" has to be answered
// from the detections alone. That is a weaker instrument and it is used for exactly one
// purpose: deciding whether the tracker was WRONG to mint a track, or right.
//
// The clustering threshold here is deliberately not the tracker's. Asking "do these boxes
// describe the same place" is a different question from "may this detection continue that
// track", and reusing 0.30 would make the answer agree with the tracker by construction —
// which is the one thing that would make the measurement worthless.

// ClusterIoU is the overlap at which two detections are treated as the same apparent place.
//
// Higher than the tracker's threshold on purpose. A region only counts as recurring if the
// boxes genuinely coincide, so this UNDER-counts recurrence: if it still reports four regions
// where production made twenty-four tracks, the gap is real and not an artefact of a generous
// threshold.
const ClusterIoU = 0.50

// SpatialRegion is one apparent place, found by clustering rather than declared.
type SpatialRegion struct {
	Role       string
	Mean       observe.Region
	Detections int
	// Inferences is how many DISTINCT inferences saw this place — the number that decides
	// whether it recurs. Two detections in one inference are two boxes, not two sightings.
	Inferences int
	First      int
	Last       int
	Tracks     []string
	MedianIoU  float64
}

// Recurring reports whether this place was seen in more than one inference.
func (s SpatialRegion) Recurring() bool { return s.Inferences > 1 }

// Cluster groups a role's detections into apparent places, single-linkage against the running
// mean of each cluster. Deterministic: inferences in order, detections in order.
func Cluster(infs []Inference, role string, res Result) []SpatialRegion {
	type cluster struct {
		sum        observe.Region
		n          int
		infs       map[int]bool
		first, obs int
		last       int
		tracks     []string
		ious       []float64
	}
	var cs []*cluster

	for _, inf := range infs {
		for di, r := range inf.Regions {
			if r.Role != role {
				continue
			}
			best, bestIoU := -1, 0.0
			for ci, c := range cs {
				m := observe.Region{
					X: c.sum.X / float64(c.n), Y: c.sum.Y / float64(c.n),
					Width: c.sum.Width / float64(c.n), Height: c.sum.Height / float64(c.n),
				}
				if o := iou(m, r.Region); o > bestIoU {
					best, bestIoU = ci, o
				}
			}
			var c *cluster
			if best >= 0 && bestIoU >= ClusterIoU {
				c = cs[best]
				c.ious = append(c.ious, bestIoU)
			} else {
				c = &cluster{infs: map[int]bool{}, first: inf.Index}
				cs = append(cs, c)
			}
			c.sum.X += r.Region.X
			c.sum.Y += r.Region.Y
			c.sum.Width += r.Region.Width
			c.sum.Height += r.Region.Height
			c.n++
			c.infs[inf.Index] = true
			c.last = inf.Index
			if a, ok := res.TrackOf(inf.Index, di); ok && !contains(c.tracks, a.Track) {
				c.tracks = append(c.tracks, a.Track)
			}
		}
	}

	out := make([]SpatialRegion, 0, len(cs))
	for _, c := range cs {
		out = append(out, SpatialRegion{
			Role: role, Detections: c.n, Inferences: len(c.infs),
			First: c.first, Last: c.last, Tracks: c.tracks, MedianIoU: median(c.ious),
			Mean: observe.Region{
				X: c.sum.X / float64(c.n), Y: c.sum.Y / float64(c.n),
				Width: c.sum.Width / float64(c.n), Height: c.sum.Height / float64(c.n),
			},
		})
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Detections != out[b].Detections {
			return out[a].Detections > out[b].Detections
		}
		return out[a].Mean.Y < out[b].Mean.Y
	})
	return out
}

func contains(v []string, s string) bool {
	for _, x := range v {
		if x == s {
			return true
		}
	}
	return false
}

// Cadence is what actually happened to the sampling, in wall-clock terms.
type Cadence struct {
	Valid    int
	Skipped  int
	Failed   int
	Unproven int
	// SpanMS is first to last valid inference.
	SpanMS   int64
	MedianMS int64
	MaxGapMS int64
}

// Slot is one shadow opportunity as the trace recorded it.
type Slot struct {
	N          int                    `json:"n"`
	AtMS       int64                  `json:"at_ms"`
	Outcome    string                 `json:"outcome"`
	Generation uint64                 `json:"generation,omitempty"`
	LatencyMS  int64                  `json:"latency_ms,omitempty"`
	Detections int                    `json:"detections,omitempty"`
	Reason     string                 `json:"reason,omitempty"`
	Regions    []observe.ShadowRegion `json:"regions,omitempty"`
	// Inputs is the navigation observed during this slot, as closed-vocabulary intents.
	//
	// Recorded for EVERY outcome, not only valid ones. A slot the cadence gate declined
	// carries no geometry and still carries what the player did during it — and at the real
	// skip rate that is very often the keypress that opened the menu the next inference is
	// about to see. A trace that dropped it would replay to a graph whose every edge is
	// unattributed, and the replay would disagree with production for a reason that has
	// nothing to do with what was being measured.
	Inputs []observe.InputEvent `json:"inputs,omitempty"`
	// InputStats is the producer's cumulative counters at this slot, when one was wired.
	InputStats *observe.InputStats `json:"input_stats,omitempty"`
	// Semantic is what the readable interface said about itself, as closed-vocabulary
	// terms. No text: the vocabulary is the whole of what survives the OCR boundary.
	Semantic observe.SemanticEvidence `json:"semantic,omitzero"`
}

// MeasureCadence summarises the sampling from real timestamps.
//
// Never from the configured interval. "The cadence is 2s" is a setting; how many valid
// observations a fifteen-second menu actually received is the question, and only the recorded
// times can answer it.
func MeasureCadence(slots []Slot) Cadence {
	var c Cadence
	var times []int64
	for _, s := range slots {
		switch s.Outcome {
		case "valid":
			c.Valid++
			times = append(times, s.AtMS)
		case "skipped":
			c.Skipped++
		case "failed":
			c.Failed++
		case "unproven":
			c.Unproven++
		}
	}
	if len(times) < 2 {
		return c
	}
	c.SpanMS = times[len(times)-1] - times[0]
	gaps := make([]float64, 0, len(times)-1)
	for i := 1; i < len(times); i++ {
		g := times[i] - times[i-1]
		gaps = append(gaps, float64(g))
		if g > c.MaxGapMS {
			c.MaxGapMS = g
		}
	}
	c.MedianMS = int64(median(gaps))
	return c
}

// InferencesFrom turns valid slots into replayable inferences.
//
// Only valid ones. A skipped slot has no geometry, and materialising it as an empty inference
// would tell the tracker every element had vanished — turning the cadence into measured
// absence, which is the failure the whole tracking design exists to avoid.
func InferencesFrom(slots []Slot) []Inference {
	var out []Inference
	for _, s := range slots {
		if s.Outcome != "valid" {
			continue
		}
		out = append(out, Inference{Index: s.N, AtMS: s.AtMS, Regions: s.Regions})
	}
	return out
}
