// Package shadowreplay replays recorded shadow detections through track matching so a
// fragmentation failure can be attributed to a LAYER rather than guessed at.
//
// # Why a mirror of the tracker rather than the tracker itself
//
// The diagnosis needs three things the production tracker deliberately does not offer:
// per-detection assignment, the losing candidates, and the ability to ask "what would have
// happened at a different threshold or under a different reference policy". Exposing those
// from `observe.ShadowTracker` would mean editing production tracking during a milestone whose
// entire point is to measure it before changing it.
//
// So this is a mirror, and [[TestTheMirrorReproducesProductionTracking]] is the thing that
// makes it evidence rather than a second opinion: it asserts that the mirror at the production
// policy produces the same tracks, seen counts and episodes as `observe.ShadowTotals` over the
// same input. A mirror that has drifted is worse than no mirror, because its numbers would
// still look authoritative.
package shadowreplay

import (
	"fmt"
	"math"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Inference is one VALID shadow observation of one frame.
//
// Only valid inferences appear. A skipped slot, a failed inference and an unproven target
// carry no geometry, and materialising them here as empty inferences would turn the cadence
// into measured absence — the exact error the production tracker exists to avoid.
type Inference struct {
	Index   int    `json:"index"`
	Frame   string `json:"frame,omitempty"`
	AtMS    int64  `json:"at_ms,omitempty"`
	Regions []observe.ShadowRegion
}

// ReferencePolicy is how a track's matching geometry is maintained.
//
// Named because it turned out to be a policy at all. Production freezes the first detection
// and never updates it (`observe.ShadowTrack.record` writes every statistic EXCEPT
// `Reference`), which is a real choice with a real failure mode, and one no test had ever
// stated out loud.
type ReferencePolicy string

const (
	// ReferenceFrozenFirst matches every later detection against the first one. Production.
	ReferenceFrozenFirst ReferencePolicy = "frozen-first"
	// ReferencePrevious matches against the most recent accepted detection.
	ReferencePrevious ReferencePolicy = "previous"
	// ReferenceMean matches against the running mean of accepted detections.
	ReferenceMean ReferencePolicy = "running-mean"
)

// Policy is the matching configuration under replay.
type Policy struct {
	MatchIoU         float64
	AbsenceTolerance int
	ReacquireGap     int
	Reference        ReferencePolicy
}

// Production is the policy the shipping tracker actually implements.
func Production() Policy {
	return Policy{
		MatchIoU:         observe.TrackMatchIoU,
		AbsenceTolerance: observe.TrackAbsenceTolerance,
		ReacquireGap:     observe.TrackReacquireGap,
		Reference:        ReferenceFrozenFirst,
	}
}

// Candidate is one track a detection could have continued, and how well.
//
// Recorded for every role-compatible track with ANY overlap, including those below the
// threshold. The sub-threshold ones are the evidence: a detection whose best candidate scores
// 0.28 failed for a different reason than one whose best candidate scores 0.00.
type Candidate struct {
	Track string  `json:"track"`
	IoU   float64 `json:"iou"`
	// RefIoU is overlap with the track's MATCHING reference; PrevIoU with the track's most
	// recent accepted detection. Under the frozen-first policy these diverge, and the gap
	// between them is precisely the reference-drift measurement.
	RefIoU  float64 `json:"ref_iou"`
	PrevIoU float64 `json:"prev_iou"`
	// Lost names why a candidate that cleared the threshold did not win.
	Lost string `json:"lost,omitempty"`
}

// Assignment is what happened to one detection.
type Assignment struct {
	Inference  int            `json:"inference"`
	Detection  int            `json:"detection"`
	Role       string         `json:"role"`
	Region     observe.Region `json:"region"`
	Confidence float64        `json:"confidence"`
	Track      string         `json:"track"`
	NewTrack   bool           `json:"new_track"`
	// Reason says why a NEW track was minted. Without it, "24 tracks" is a number rather
	// than an explanation, and the fixes implied by "nothing overlapped" and "something
	// overlapped but a neighbour took it" are opposites.
	Reason     string      `json:"reason,omitempty"`
	Candidates []Candidate `json:"candidates,omitempty"`
}

// Reasons a detection started its own track.
const (
	// ReasonFirstOfRole means no track of this role existed yet. Unavoidable, not a failure.
	ReasonFirstOfRole = "FIRST_OF_ROLE"
	// ReasonNoCompatibleTrack means tracks existed and none overlapped enough.
	ReasonNoCompatibleTrack = "NO_COMPATIBLE_TRACK"
	// ReasonTrackAlreadyAssigned means a track DID clear the bar but a better-overlapping
	// detection took it first — greedy competition, the assignment failure mode.
	ReasonTrackAlreadyAssigned = "COMPATIBLE_TRACK_ALREADY_ASSIGNED"
	// ReasonReturnAfterRetired means a compatible track existed but was past the reacquire
	// gap, so the element's return could not resume its old identity.
	ReasonReturnAfterRetired = "RETURN_AFTER_RETIRED"
)

// Track is a replayed track, in the same shape the production one reports.
type Track struct {
	ID       string  `json:"id"`
	Role     string  `json:"role"`
	Seen     int     `json:"seen"`
	Eligible int     `json:"eligible"`
	Episodes int     `json:"episodes"`
	First    int     `json:"first"`
	Last     int     `json:"last"`
	MeanIoU  float64 `json:"mean_iou"`
}

// Result is one replay.
type Result struct {
	Policy      Policy
	Inferences  int
	Tracks      []Track
	Assignments []Assignment
}

// TrackOf returns the track a given detection joined.
func (r Result) TrackOf(inference, detection int) (Assignment, bool) {
	for _, a := range r.Assignments {
		if a.Inference == inference && a.Detection == detection {
			return a, true
		}
	}
	return Assignment{}, false
}

type mirrorTrack struct {
	id, role string
	ref      observe.Region
	prev     observe.Region
	sum      observe.Region
	seen     int
	eligible int
	episodes int
	lastSeen int
	first    int
	missRun  int
	present  bool
	iouSum   float64
}

// matchRef is the geometry a candidate is scored against under this policy.
func (t *mirrorTrack) matchRef(p Policy) observe.Region {
	switch p.Reference {
	case ReferencePrevious:
		return t.prev
	case ReferenceMean:
		if t.seen == 0 {
			return t.ref
		}
		n := float64(t.seen)
		return observe.Region{
			X: t.sum.X / n, Y: t.sum.Y / n,
			Width: t.sum.Width / n, Height: t.sum.Height / n,
		}
	default:
		return t.ref
	}
}

// Run replays the inferences under a policy.
//
// The control flow deliberately mirrors observe.ShadowTracker.Observe step for step —
// eligibility first, then greedy assignment, then absence — because a mirror that reorganised
// the algorithm "more cleanly" would stop being able to prove anything about the original.
func Run(infs []Inference, p Policy) Result {
	if p.MatchIoU <= 0 {
		p = Production()
	}
	res := Result{Policy: p}
	var tracks []*mirrorTrack
	next := 0

	for n, inf := range infs {
		step := n + 1
		for _, t := range tracks {
			t.eligible++
		}

		matched, cands, ctx := assign(tracks, inf.Regions, p, step)

		for di, r := range inf.Regions {
			a := Assignment{
				Inference: inf.Index, Detection: di, Role: r.Role,
				Region: r.Region, Confidence: r.Confidence, Candidates: cands[di],
			}
			if ti, ok := matched[di]; ok {
				t := tracks[ti]
				record(t, step, r, p)
				a.Track = t.id
			} else {
				next++
				t := &mirrorTrack{
					id: fmt.Sprintf("shadow_%d", next), role: r.Role,
					ref: r.Region, prev: r.Region, first: step, eligible: 1, present: true,
				}
				tracks = append(tracks, t)
				record(t, step, r, p)
				t.episodes = 1
				a.Track, a.NewTrack, a.Reason = t.id, true, ctx[di].reason()
			}
			res.Assignments = append(res.Assignments, a)
		}

		found := map[int]bool{}
		for _, ti := range matched {
			found[ti] = true
		}
		for i, t := range tracks {
			if found[i] || t.lastSeen == step {
				continue
			}
			t.missRun++
			if t.present && t.missRun > p.AbsenceTolerance {
				t.present = false
			}
		}
		res.Inferences++
	}

	for _, t := range tracks {
		tr := Track{
			ID: t.id, Role: t.role, Seen: t.seen, Eligible: t.eligible,
			Episodes: t.episodes, First: t.first, Last: t.lastSeen,
		}
		if t.seen > 0 {
			tr.MeanIoU = t.iouSum / float64(t.seen)
		}
		res.Tracks = append(res.Tracks, tr)
	}
	sort.SliceStable(res.Tracks, func(a, b int) bool {
		if res.Tracks[a].Seen != res.Tracks[b].Seen {
			return res.Tracks[a].Seen > res.Tracks[b].Seen
		}
		return res.Tracks[a].ID < res.Tracks[b].ID
	})
	return res
}

func record(t *mirrorTrack, step int, r observe.ShadowRegion, p Policy) {
	if t.seen > 0 && !t.present {
		t.episodes++
	}
	// Statistics are always reported against the FROZEN first box, whatever the matching
	// policy is. Otherwise a policy that follows the detection would report a flattering
	// mean IoU by construction — it would be measuring its own tracking, not the geometry.
	t.iouSum += iou(t.ref, r.Region)
	t.seen++
	t.lastSeen, t.present, t.missRun = step, true, 0
	t.prev = r.Region
	t.sum.X += r.Region.X
	t.sum.Y += r.Region.Y
	t.sum.Width += r.Region.Width
	t.sum.Height += r.Region.Height
	_ = p
}

// detContext is what the matcher saw around one detection, for explaining a new track.
type detContext struct {
	roleCompatible int // tracks of the same role that existed at all
	retiredMatch   int // tracks that would have matched but were past the reacquire gap
	lostToRival    int // tracks that cleared the bar and were taken by another detection
}

// reason classifies why this detection could not continue anything.
func (c *detContext) reason() string {
	switch {
	case c == nil || c.roleCompatible == 0:
		return ReasonFirstOfRole
	case c.lostToRival > 0:
		return ReasonTrackAlreadyAssigned
	case c.retiredMatch > 0:
		return ReasonReturnAfterRetired
	default:
		return ReasonNoCompatibleTrack
	}
}

// assign is the production greedy matcher, instrumented.
func assign(tracks []*mirrorTrack, regions []observe.ShadowRegion, p Policy, step int) (
	map[int]int, map[int][]Candidate, map[int]*detContext) {

	type pair struct {
		det, track int
		score      float64
	}
	var cands []pair
	all := map[int][]Candidate{}
	ctx := map[int]*detContext{}

	for di, r := range regions {
		c := &detContext{}
		ctx[di] = c
		for ti, t := range tracks {
			if t.role != r.Role {
				continue
			}
			c.roleCompatible++
			if step-t.lastSeen > p.ReacquireGap {
				if iou(t.matchRef(p), r.Region) >= p.MatchIoU {
					// It would have matched, but the element stayed away too long.
					c.retiredMatch++
				}
				continue
			}
			ref := t.matchRef(p)
			o := iou(ref, r.Region)
			if o > 0 {
				all[di] = append(all[di], Candidate{
					Track: t.id, IoU: o,
					RefIoU:  iou(t.ref, r.Region),
					PrevIoU: iou(t.prev, r.Region),
				})
			}
			if o >= p.MatchIoU {
				cands = append(cands, pair{di, ti, o})
			}
		}
	}
	sort.SliceStable(cands, func(a, b int) bool {
		if cands[a].score != cands[b].score {
			return cands[a].score > cands[b].score
		}
		if cands[a].track != cands[b].track {
			return cands[a].track < cands[b].track
		}
		return cands[a].det < cands[b].det
	})

	out := map[int]int{}
	usedD, usedT := map[int]bool{}, map[int]bool{}
	for _, c := range cands {
		if usedD[c.det] {
			continue
		}
		if usedT[c.track] {
			// A real competition loss: this detection cleared the bar for a track that a
			// better-overlapping detection had already taken. Named, because "lost to a
			// neighbour" and "never overlapped anything" are different failures.
			markLost(all[c.det], tracks[c.track].id, "track already taken this inference")
			ctx[c.det].lostToRival++
			continue
		}
		usedD[c.det], usedT[c.track] = true, true
		out[c.det] = c.track
	}
	for di := range all {
		sort.SliceStable(all[di], func(a, b int) bool { return all[di][a].IoU > all[di][b].IoU })
	}
	return out, all, ctx
}

func markLost(cs []Candidate, id, why string) {
	for i := range cs {
		if cs[i].Track == id {
			cs[i].Lost = why
		}
	}
}

func iou(a, b observe.Region) float64 {
	x1, y1 := math.Max(a.X, b.X), math.Max(a.Y, b.Y)
	x2 := math.Min(a.X+a.Width, b.X+b.Width)
	y2 := math.Min(a.Y+a.Height, b.Y+b.Height)
	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	inter := (x2 - x1) * (y2 - y1)
	union := a.Width*a.Height + b.Width*b.Height - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

// IoU exposes the overlap function so callers measure the same way the matcher does.
func IoU(a, b observe.Region) float64 { return iou(a, b) }

// CentreDistance is the normalised distance between two region centres.
func CentreDistance(a, b observe.Region) float64 {
	dx := (a.X + a.Width/2) - (b.X + b.Width/2)
	dy := (a.Y + a.Height/2) - (b.Y + b.Height/2)
	return math.Hypot(dx, dy)
}
