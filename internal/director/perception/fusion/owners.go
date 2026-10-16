package fusion

// WHICH ELEMENTS COULD OWN A REGION — one rule, two callers.
//
// Fusion asks this when it decides which element a word was printed on. Director asks the same
// question one tier up when it asks which Prop a request's words sit inside, and the two must
// not answer differently: a surface Director nominates and fusion would have refused is a
// surface nobody can explain.
//
// So the rule, the three constants and the ordering live here once, and both go through them.
// `textCandidates` and `ambiguousRival` are the fusion-side wrappers; `Owners` and `Rivals` are
// the exported ones. The conformance test in owners_test.go holds them in agreement by running
// both over the same inputs.

import (
	"sort"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ownerRank is one candidate owner of a region: its position in the caller's own slice, and how
// much of the region lies inside it.
type ownerRank struct {
	index       int
	containment float64
}

// rankOwners is the rule. Best container first.
//
// Containment of the REGION by the candidate, not intersection-over-union: a word inside a
// button covers a small fraction of it, which IoU rates near zero, and that is the exact
// relationship being asked about. Smallest container first among equals, because a word is
// printed on the innermost control that contains it.
//
// `keys` are the caller's stable identifiers, used only to break a tie between two candidates
// agreeing on both containment and area, so the order never depends on map iteration.
func rankOwners(boxes []directorapi.Rect, keys []string, box directorapi.Rect) []ownerRank {
	if box.Empty() {
		return nil
	}
	var out []ownerRank
	for i, b := range boxes {
		if b.Empty() {
			continue
		}
		cov := b.Covers(box)
		if cov < minContainment {
			continue
		}
		out = append(out, ownerRank{index: i, containment: cov})
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].containment != out[b].containment {
			return out[a].containment > out[b].containment
		}
		aa, bb := boxes[out[a].index].Area(), boxes[out[b].index].Area()
		if aa != bb {
			return aa < bb
		}
		return keys[out[a].index] < keys[out[b].index]
	})
	return out
}

// rivalOf reports whether a second candidate could equally own the region.
//
// NESTING IS NOT AMBIGUITY. Containment measures how much of the region lies inside the
// candidate, so every ancestor of the real owner scores exactly 1.0 as well: a word on a button
// is equally inside the button, the toolbar, the pane and the window. Measured live against VS
// Code, treating that as a tie rejected all 217 text observations — which looks like caution and
// is really just blindness.
//
// So ambiguity means something narrower: two candidates that contain the region equally well AND
// are comparably sized — siblings, not ancestors.
func rivalOf(boxes []directorapi.Rect, ranks []ownerRank) (ownerRank, bool) {
	if len(ranks) < 2 {
		return ownerRank{}, false
	}
	best := ranks[0]
	bestArea := boxes[best.index].Area()
	for _, c := range ranks[1:] {
		if c.containment <= best.containment-ambiguityMargin {
			continue // materially worse containment; not a rival
		}
		area := boxes[c.index].Area()
		if bestArea <= 0 || area <= 0 {
			continue
		}
		if float64(area) <= float64(bestArea)*nestingRatio {
			return c, true
		}
	}
	return ownerRank{}, false
}

// Owner is one element that could own a region, with the evidence for saying so.
type Owner struct {
	Element     *directorapi.Element
	Containment float64 `json:"containment"`
	// Ancestor is whether the best owner is plainly inside this one rather than beside it —
	// more than `nestingRatio` bigger. An ancestor is not a rival for the region and IS a
	// candidate surface for whatever sits in it, which is the whole reason this is reported
	// rather than filtered.
	Ancestor bool `json:"ancestor"`
}

// Owners returns the elements that could own `box`, best container first, by the same rule
// fusion used when it built them.
//
// `els` is taken as a slice so the caller fixes the order; a map's iteration order would make
// the tie-break unstable, and the tie-break is the difference between a button and its pane.
func Owners(els []*directorapi.Element, box directorapi.Rect) []Owner {
	boxes := make([]directorapi.Rect, len(els))
	keys := make([]string, len(els))
	for i, el := range els {
		if el == nil {
			continue
		}
		boxes[i], keys[i] = el.Bounds, string(el.ID)
	}
	ranks := rankOwners(boxes, keys, box)
	if len(ranks) == 0 {
		return nil
	}
	bestArea := boxes[ranks[0].index].Area()
	out := make([]Owner, 0, len(ranks))
	for _, r := range ranks {
		out = append(out, Owner{
			Element:     els[r.index],
			Containment: r.containment,
			Ancestor: bestArea > 0 &&
				float64(boxes[r.index].Area()) > float64(bestArea)*nestingRatio,
		})
	}
	return out
}

// Rivals reports whether the second owner could equally own the region — fusion's own ambiguity
// rule, asked of a list from Owners.
func Rivals(owners []Owner) (Owner, bool) {
	boxes := make([]directorapi.Rect, len(owners))
	ranks := make([]ownerRank, len(owners))
	for i, o := range owners {
		if o.Element != nil {
			boxes[i] = o.Element.Bounds
		}
		ranks[i] = ownerRank{index: i, containment: o.Containment}
	}
	r, ok := rivalOf(boxes, ranks)
	if !ok {
		return Owner{}, false
	}
	return owners[r.index], true
}

// MinContainment is how much of a region must lie inside a candidate before the candidate can be
// said to contain it. Exported so a caller can report the rule it was judged by rather than
// restating the number.
func MinContainment() float64 { return minContainment }

// NestingRatio is how much bigger a candidate may be before it is an ancestor rather than a peer.
func NestingRatio() float64 { return nestingRatio }
