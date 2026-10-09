package selection

import (
	"image"
	"math"
	"sort"
)

// Assembling a LIST out of whatever perception proposed.
//
// FindRows looks at pixels because it has nothing else. This does not: it takes the regions
// another system already found — a learned detector, OCR, fusion — and asks which of them
// form a list. That is the right division. Finding things on a screen is perception's job and
// it is far better equipped for it; deciding that eight of those things are the rows of one
// menu is a question about arrangement, and arrangement is all this needs.
//
// The rule is the one the reader itself rests on: a list is a REPEATING structure. Among the
// proposals, the largest set that shares a height, shares a horizontal extent and sits at a
// regular vertical pitch is the list. Nothing here knows what a menu looks like.
//
// Measured on the production detector's output, that matters more than it sounds: on one
// Trackmania frame the detector proposed 135 regions, of which seven were menu rows. The list
// is a small, highly regular minority of a large, irregular pile.

const (
	// stackTolerance is how much a member's height and horizontal extent may differ from
	// the seed pair's.
	stackTolerance = 0.35
	// pitchTolerance is how far off the regular spacing a member may sit, as a fraction
	// of the pitch.
	pitchTolerance = 0.3
	// minStack is the fewest members worth calling a list. Two boxes define a pitch and
	// prove nothing; three is the first number that can be regular.
	minStack = 3
	// strayGap is how far beyond the stack, in pitches, a lone aligned region may sit and
	// still be offered as a candidate. A menu's last entry is often set apart by a wider
	// gap — which is exactly the case the reader handles temporally, and it can only do
	// that if the region reaches it at all.
	strayGap = 3.0
)

// GroupRows picks the list out of a set of proposed regions.
//
// The returned rows are the stack, in top-to-bottom order, sharing group 0; followed by any
// aligned stray near it, each in a group of its own so the reader treats it as having no
// siblings. Regions that are neither are dropped, because a reader comparing a menu row with
// an advertisement learns nothing from the difference.
func GroupRows(boxes []image.Rectangle) []Row {
	clean := dedupe(boxes)
	if len(clean) < minStack {
		return nil
	}
	sort.Slice(clean, func(i, j int) bool { return centreY(clean[i]) < centreY(clean[j]) })

	// EVERY regular stack is offered, each as its own group — not just the biggest.
	//
	// Choosing between them was the first design and it was wrong. A screen holds more
	// than one list: on the measured Trackmania frame the medal-times panel is six evenly
	// spaced rows, as regular as the menu and smaller, and "the largest regular stack"
	// picked a different pile entirely. Entry discovery came out at 4%.
	//
	// There is no general rule that says which list a person means, and inventing one
	// would be guessing. But there is no need to guess: the reader already decides by
	// EVIDENCE, within a group, and a list nobody is choosing from has no deviant row.
	// So grouping proposes structure and selection decides which structure is live.
	taken := map[int]bool{}
	out := make([]Row, 0, len(clean))
	group := 0
	for {
		members := stack(clean, taken)
		if len(members) < minStack {
			break
		}
		for _, i := range members {
			taken[i] = true
			out = append(out, Row{Bounds: clean[i], Group: group})
		}
		// A stray is aligned with this stack and near it, but off the beat: the last
		// entry of a menu, set apart by a wider gap. It gets a group of its own, which
		// is what sends it down the reader's temporal path.
		pitch := stackPitch(clean, members)
		top, bottom := centreY(clean[members[0]]), centreY(clean[members[len(members)-1]])
		width := float64(clean[members[0]].Dx())
		for i, b := range clean {
			if taken[i] || relDiff(float64(b.Dx()), width) > stackTolerance {
				continue
			}
			d := math.Min(math.Abs(centreY(b)-bottom), math.Abs(centreY(b)-top))
			if pitch > 0 && d <= strayGap*pitch {
				taken[i] = true
				group++
				out = append(out, Row{Bounds: b, Group: group})
			}
		}
		group++
	}
	return out
}

// stack finds the largest regular run, by trying every pair as the seed that sets the pitch.
func stack(boxes []image.Rectangle, taken map[int]bool) []int {
	var best []int
	for i := range boxes {
		if taken[i] {
			continue
		}
		for j := i + 1; j < len(boxes); j++ {
			if taken[j] {
				continue
			}
			pitch := centreY(boxes[j]) - centreY(boxes[i])
			if pitch <= 0 {
				continue
			}
			if !alike(boxes[i], boxes[j]) {
				continue
			}
			members := []int{i}
			want := centreY(boxes[i]) + pitch
			for k := j; k < len(boxes); k++ {
				if taken[k] || !alike(boxes[i], boxes[k]) {
					continue
				}
				if math.Abs(centreY(boxes[k])-want) <= pitchTolerance*pitch {
					members = append(members, k)
					want += pitch
				}
			}
			if len(members) > len(best) {
				best = members
			}
		}
	}
	return best
}

func stackPitch(boxes []image.Rectangle, members []int) float64 {
	if len(members) < 2 {
		return 0
	}
	first, last := centreY(boxes[members[0]]), centreY(boxes[members[len(members)-1]])
	return (last - first) / float64(len(members)-1)
}

// alike reports whether two regions could be rows of one list.
func alike(a, b image.Rectangle) bool {
	return relDiff(float64(a.Dy()), float64(b.Dy())) <= stackTolerance &&
		relDiff(float64(a.Dx()), float64(b.Dx())) <= stackTolerance
}

// dedupe drops regions that substantially repeat one already kept.
//
// Detectors propose the same thing several times at different confidences, and a list whose
// rows are each present twice has a pitch of zero and no structure at all.
func dedupe(boxes []image.Rectangle) []image.Rectangle {
	// Largest first, and that order is the whole of it. A detector proposes a row AND the
	// label drawn inside it, and the label is wholly contained, so whichever is seen
	// first survives. Measured: on Rocket League the inner text came first and every one
	// of the four menu rows was discarded as a duplicate of its own caption — the stack
	// was in the proposals, and grouping threw it away before looking.
	//
	// A row contains its label; the container is the candidate.
	sorted := append([]image.Rectangle(nil), boxes...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Dx()*sorted[i].Dy() > sorted[j].Dx()*sorted[j].Dy()
	})
	var out []image.Rectangle
	for _, b := range sorted {
		if b.Dx() <= 0 || b.Dy() <= 0 {
			continue
		}
		dup := false
		for _, k := range out {
			if overlap(b, k) > 0.6 {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, b)
		}
	}
	return out
}

func overlap(a, b image.Rectangle) float64 {
	in := a.Intersect(b)
	if in.Empty() {
		return 0
	}
	ia := float64(in.Dx() * in.Dy())
	return ia / math.Min(float64(a.Dx()*a.Dy()), float64(b.Dx()*b.Dy()))
}

func centreY(b image.Rectangle) float64 { return float64(b.Min.Y+b.Max.Y) / 2 }
