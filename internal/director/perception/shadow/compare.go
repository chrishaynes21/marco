// Package shadow compares experimental perception against the perception Marco trusts.
//
// # What this package is for
//
// A shadow provider produces evidence that will never be believed. That makes exactly one
// question worth asking about it: what would Marco have known that it did not already know?
//
// Answering that honestly is harder than it looks, and the failure mode is a specific one —
// declaring every unmatched shadow detection to be new truth. It is not. It is a
// DISAGREEMENT, and which side is right is not something this package is entitled to decide.
// So the vocabulary below has no winner in it: `SHADOW_ONLY` says one saw structure the other
// did not, and stops there.
//
// # What this package is NOT
//
// Not a promotion path. Nothing here returns a decision, a confidence, or a belief. It counts
// and categorises, and every category is symmetric between the two sides.
package shadow

import (
	"sort"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Verdict is how one region of one side relates to the other side.
type Verdict string

const (
	// Agreed: compatible structure in the same place.
	Agreed Verdict = "AGREED"
	// ShadowOnly: the experiment reported structure authoritative perception has none of.
	//
	// NOT "new truth". The experiment may be right, or it may be hallucinating — this
	// package does not know and does not guess.
	ShadowOnly Verdict = "SHADOW_ONLY"
	// AuthoritativeOnly: authoritative perception has structure the experiment missed.
	AuthoritativeOnly Verdict = "AUTHORITATIVE_ONLY"
	// RoleDisagreement: both see it, and normalise it differently.
	RoleDisagreement Verdict = "ROLE_DISAGREEMENT"
	// GeometryDisagreement: probably the same thing, materially different bounds.
	GeometryDisagreement Verdict = "GEOMETRY_DISAGREEMENT"
	// Uncomparable: the two did not observe the same world, so nothing about their
	// difference is evidence about UI.
	//
	// A distinct outcome rather than a disagreement, and the distinction is load-bearing:
	// counting a generation mismatch as "the experiment saw a menu the Director missed"
	// would manufacture information gain out of a race condition.
	Uncomparable Verdict = "UNCOMPARABLE"
)

// MatchIoU is the overlap at which two regions are taken to describe the same thing.
//
// 0.35, the same figure the benchmark's matcher uses, and for the same reason: a slightly
// loose box around a button is still that button. Restated here rather than imported —
// visionbench is measurement scaffolding and this is a live diagnostic, and a production
// package that depended on the benchmark's constants would make the benchmark unable to
// change its own thresholds without moving production behaviour.
const MatchIoU = 0.35

// Region is one side's claim about a piece of interface.
//
// Deliberately small. This carries a role and a rectangle, never a label — comparison must
// not become a route by which text the privacy policy withheld arrives somewhere else.
type Region struct {
	Role   directorapi.ElementRole
	Bounds directorapi.Rect
	// Nameable records whether the role may be said in plaintext, resolved by the caller
	// under the existing policy. This package never decides nameability itself.
	Nameable bool
}

// Pair is one comparison result.
type Pair struct {
	Verdict Verdict
	// Shadow and Authoritative are the indices into the input slices, or -1.
	Shadow        int
	Authoritative int
	Overlap       float64
}

// Summary counts a comparison.
type Summary struct {
	Agreed               int `json:"agreed"`
	ShadowOnly           int `json:"shadow_only"`
	AuthoritativeOnly    int `json:"authoritative_only"`
	RoleDisagreement     int `json:"role_disagreement"`
	GeometryDisagreement int `json:"geometry_disagreement"`
	Uncomparable         int `json:"uncomparable"`
	// ShadowOnlyNameable is the number that answers the milestone's central question:
	// safe, role-bearing structure the experiment offered and belief did not have.
	ShadowOnlyNameable int `json:"shadow_only_nameable"`
}

// Total is how many comparisons the summary covers.
func (s Summary) Total() int {
	return s.Agreed + s.ShadowOnly + s.AuthoritativeOnly +
		s.RoleDisagreement + s.GeometryDisagreement + s.Uncomparable
}

// Compare matches two sets of regions and categorises every one of them.
//
// # Comparable
//
// False when the two sides did not observe the same world — a superseded window generation,
// an unproven target, evidence from different moments. Every region on both sides is then
// UNCOMPARABLE, and nothing is counted as agreement or as gain. This is a parameter rather
// than something inferred here because provenance is established by the perception layer and
// re-deriving it in a diagnostic would be a second, drifting answer to a settled question.
//
// # Matching
//
// Greedy by descending overlap, which is deterministic and enough: two candidate regions with
// materially overlapping bounds are the same thing, and the assignment only matters when a
// side reports near-duplicates, where either choice yields the same counts.
func Compare(shadowRegions, authoritative []Region, comparable bool) ([]Pair, Summary) {
	var pairs []Pair
	var sum Summary

	if !comparable {
		for i := range shadowRegions {
			pairs = append(pairs, Pair{Verdict: Uncomparable, Shadow: i, Authoritative: -1})
		}
		for i := range authoritative {
			pairs = append(pairs, Pair{Verdict: Uncomparable, Shadow: -1, Authoritative: i})
		}
		sum.Uncomparable = len(pairs)
		return pairs, sum
	}

	type candidate struct {
		s, a    int
		overlap float64
	}
	var cands []candidate
	for si, s := range shadowRegions {
		for ai, a := range authoritative {
			if o := iou(s.Bounds, a.Bounds); o >= MatchIoU {
				cands = append(cands, candidate{si, ai, o})
			}
		}
	}
	// Descending overlap, then by index so the result never depends on map order.
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].overlap != cands[j].overlap {
			return cands[i].overlap > cands[j].overlap
		}
		if cands[i].s != cands[j].s {
			return cands[i].s < cands[j].s
		}
		return cands[i].a < cands[j].a
	})

	usedS := make([]bool, len(shadowRegions))
	usedA := make([]bool, len(authoritative))
	for _, c := range cands {
		if usedS[c.s] || usedA[c.a] {
			continue
		}
		usedS[c.s], usedA[c.a] = true, true
		p := Pair{Shadow: c.s, Authoritative: c.a, Overlap: c.overlap}
		switch {
		case shadowRegions[c.s].Role != authoritative[c.a].Role:
			p.Verdict = RoleDisagreement
			sum.RoleDisagreement++
		case c.overlap < GeometryAgreement:
			// Same role, same thing, but the boxes differ enough that a click placed
			// from one could miss what the other described.
			p.Verdict = GeometryDisagreement
			sum.GeometryDisagreement++
		default:
			p.Verdict = Agreed
			sum.Agreed++
		}
		pairs = append(pairs, p)
	}
	for i := range shadowRegions {
		if usedS[i] {
			continue
		}
		pairs = append(pairs, Pair{Verdict: ShadowOnly, Shadow: i, Authoritative: -1})
		sum.ShadowOnly++
		if shadowRegions[i].Nameable {
			sum.ShadowOnlyNameable++
		}
	}
	for i := range authoritative {
		if usedA[i] {
			continue
		}
		pairs = append(pairs, Pair{Verdict: AuthoritativeOnly, Shadow: -1, Authoritative: i})
		sum.AuthoritativeOnly++
	}
	return pairs, sum
}

// GeometryAgreement is the overlap above which two same-role regions are called agreement
// rather than a geometry disagreement.
//
// Above the match threshold on purpose: 0.35 is loose enough to say "these describe the same
// control", and not tight enough to say "these two boxes agree". The band between them is
// exactly the interesting case — both saw the button, and they disagree about where it is.
const GeometryAgreement = 0.70

// iou is intersection over union of two rectangles.
func iou(a, b directorapi.Rect) float64 {
	x1, y1 := max(a.X, b.X), max(a.Y, b.Y)
	x2 := min(a.X+a.Width, b.X+b.Width)
	y2 := min(a.Y+a.Height, b.Y+b.Height)
	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	inter := float64((x2 - x1) * (y2 - y1))
	union := float64(a.Width*a.Height) + float64(b.Width*b.Height) - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}
