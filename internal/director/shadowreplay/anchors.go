package shadowreplay

import (
	"math"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Attributing detections to the thing that was actually on the screen.
//
// # Why ground truth anchors this and clustering does not
//
// The question is "did the SAME visible button become one track". Clustering the detections
// answers a weaker question — whether the detections agree with each other — and it cannot
// distinguish a button the detector missed from a button that was never there. The corpus
// already carries a per-identity box for every annotated frame, so an opportunity can be
// counted where the element genuinely WAS, including on the inferences where nothing was
// detected. Those are the interesting ones.
//
// Ground truth is used ONLY to count opportunities and to attribute detections to an element.
// It never grades the tracker: whether two detections landed in the same track is decided by
// the tracker's own output.

// AnchorIoU is the overlap at which a detection is treated as evidence OF a truth element.
//
// Deliberately not the production match threshold. This is asking "is this detection about
// that button", which is a looser question than "may this detection continue that track", and
// borrowing the tracker's number would bake the thing under test into the measurement.
const AnchorIoU = 0.20

// Anchor is one ground-truth element and where it was, per inference.
type Anchor struct {
	Identity string
	Role     string
	Boxes    map[int]observe.Region
}

// Present reports whether the element was on screen at an inference.
func (a Anchor) Present(inference int) (observe.Region, bool) {
	r, ok := a.Boxes[inference]
	return r, ok
}

// Outcome is what happened at one opportunity. The five are kept separate on purpose: a
// detector that never saw the button and a tracker that split it are different bugs with
// different fixes, and a single "failed" bucket would hide which one this is.
type Outcome string

const (
	DetectedMatched    Outcome = "DETECTED_MATCHED"
	DetectedFragmented Outcome = "DETECTED_FRAGMENTED"
	DetectorMiss       Outcome = "DETECTOR_MISS"
	SchedulerSkipped   Outcome = "SCHEDULER_SKIPPED"
	Unproven           Outcome = "UNPROVEN"
)

// Opportunity is one (element, inference) pair and its classification.
type Opportunity struct {
	Identity  string  `json:"identity"`
	Inference int     `json:"inference"`
	Outcome   Outcome `json:"outcome"`
	Detection int     `json:"detection"`
	Track     string  `json:"track"`
	Expected  string  `json:"expected_track,omitempty"`
	TruthIoU  float64 `json:"truth_iou,omitempty"`
	// PrevIoU and RefIoU are the two numbers that separate reference drift from unstable
	// localisation, populated on fragmented events.
	PrevIoU float64 `json:"prev_iou,omitempty"`
	RefIoU  float64 `json:"ref_iou,omitempty"`
	// BestBelow is the strongest sub-threshold candidate, when nothing was matched.
	BestBelow float64 `json:"best_below,omitempty"`
	Region    observe.Region
}

// ElementReport is everything measured about one ground-truth element.
type ElementReport struct {
	Identity string
	Role     string

	Eligible   int // inferences where the element was on screen
	Detected   int
	Matched    int
	Fragmented int
	Missed     int
	Skipped    int
	Unproven   int

	Tracks []string // distinct production tracks its detections landed in

	// Geometry of consecutive DETECTIONS of this element.
	MedianConsecutiveIoU   float64
	MinConsecutiveIoU      float64
	ConsecutiveBelowMatch  int // consecutive pairs under the production match threshold
	ConsecutivePairs       int
	MeanCentreDisplacement float64
	WidthVariance          float64
	HeightVariance         float64

	// MedianRefIoU is overlap with the FIRST detection — what production actually matches
	// against. The gap between this and MedianConsecutiveIoU is the drift measurement.
	MedianRefIoU float64
	MinRefIoU    float64

	Opportunities []Opportunity
}

// Recall is detections over inferences the element was actually visible for.
func (e ElementReport) Recall() float64 {
	if e.Eligible == 0 {
		return 0
	}
	return float64(e.Detected) / float64(e.Eligible)
}

// DetectionsPerTrack is the fragmentation ratio: 1.0 means every sighting made a new track.
func (e ElementReport) DetectionsPerTrack() float64 {
	if len(e.Tracks) == 0 {
		return 0
	}
	return float64(e.Detected) / float64(len(e.Tracks))
}

// Analyse classifies every opportunity for every anchor against one replay.
func Analyse(infs []Inference, anchors []Anchor, res Result) []ElementReport {
	byIndex := map[int]Inference{}
	for _, in := range infs {
		byIndex[in.Index] = in
	}

	out := make([]ElementReport, 0, len(anchors))
	for _, a := range anchors {
		e := ElementReport{Identity: a.Identity, Role: a.Role}

		indices := make([]int, 0, len(a.Boxes))
		for i := range a.Boxes {
			indices = append(indices, i)
		}
		sort.Ints(indices)

		var hits []observe.ShadowRegion // attributed detections, in order
		var expected string
		seenTrack := map[string]bool{}

		for _, n := range indices {
			truth := a.Boxes[n]
			e.Eligible++

			inf, ran := byIndex[n]
			if !ran {
				// No valid inference covered this frame at all. In a live trace this is a
				// cadence skip; in a corpus replay it cannot happen, and if it ever does it
				// must not be silently counted as a detector miss.
				e.Skipped++
				e.Opportunities = append(e.Opportunities, Opportunity{
					Identity: a.Identity, Inference: n, Outcome: SchedulerSkipped,
				})
				continue
			}

			di, best := attribute(inf.Regions, a.Role, truth)
			if di < 0 {
				e.Missed++
				e.Opportunities = append(e.Opportunities, Opportunity{
					Identity: a.Identity, Inference: n, Outcome: DetectorMiss, Detection: -1,
				})
				continue
			}
			e.Detected++
			hits = append(hits, inf.Regions[di])

			op := Opportunity{
				Identity: a.Identity, Inference: n, Detection: di,
				TruthIoU: best, Region: inf.Regions[di].Region,
			}
			asg, ok := res.TrackOf(n, di)
			if ok {
				op.Track = asg.Track
				if !seenTrack[asg.Track] {
					seenTrack[asg.Track] = true
					e.Tracks = append(e.Tracks, asg.Track)
				}
			}
			if expected == "" {
				expected = op.Track
				op.Outcome, op.Expected = DetectedMatched, expected
				e.Matched++
			} else if op.Track == expected {
				op.Outcome, op.Expected = DetectedMatched, expected
				e.Matched++
			} else {
				op.Outcome, op.Expected = DetectedFragmented, expected
				e.Fragmented++
				// Why the expected track lost, in the two numbers that tell drift from
				// jitter.
				for _, c := range asg.Candidates {
					if c.Track == expected {
						op.PrevIoU, op.RefIoU = c.PrevIoU, c.RefIoU
					}
					if c.IoU > op.BestBelow && c.IoU < res.Policy.MatchIoU {
						op.BestBelow = c.IoU
					}
				}
				// A fragmented detection starts a new lineage; comparing every later
				// sighting to a track that is already gone would inflate the count.
				expected = op.Track
			}
			e.Opportunities = append(e.Opportunities, op)
		}

		fillGeometry(&e, hits, res.Policy.MatchIoU)
		out = append(out, e)
	}
	return out
}

// attribute picks the detection this truth box is about, or -1.
func attribute(regions []observe.ShadowRegion, role string, truth observe.Region) (int, float64) {
	best, bestIoU := -1, 0.0
	for i, r := range regions {
		if r.Role != role {
			continue
		}
		if o := iou(truth, r.Region); o > bestIoU {
			best, bestIoU = i, o
		}
	}
	if bestIoU < AnchorIoU {
		return -1, bestIoU
	}
	return best, bestIoU
}

func fillGeometry(e *ElementReport, hits []observe.ShadowRegion, matchIoU float64) {
	if len(hits) == 0 {
		return
	}
	var cons, refs, disp []float64
	var ws, hs []float64
	ref := hits[0].Region
	for i, h := range hits {
		ws = append(ws, h.Region.Width)
		hs = append(hs, h.Region.Height)
		refs = append(refs, iou(ref, h.Region))
		if i > 0 {
			o := iou(hits[i-1].Region, h.Region)
			cons = append(cons, o)
			disp = append(disp, CentreDistance(hits[i-1].Region, h.Region))
			if o < matchIoU {
				e.ConsecutiveBelowMatch++
			}
		}
	}
	e.ConsecutivePairs = len(cons)
	e.MedianConsecutiveIoU, e.MinConsecutiveIoU = median(cons), min(cons)
	e.MedianRefIoU, e.MinRefIoU = median(refs), min(refs)
	e.MeanCentreDisplacement = mean(disp)
	e.WidthVariance, e.HeightVariance = variance(ws), variance(hs)
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func min(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := v[0]
	for _, x := range v {
		if x < m {
			m = x
		}
	}
	return m
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var s float64
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func variance(v []float64) float64 {
	if len(v) < 2 {
		return 0
	}
	m := mean(v)
	var s float64
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return math.Max(0, s/float64(len(v)))
}
