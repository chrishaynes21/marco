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
			if members := run(boxes, taken, i, j, pitch); len(members) > len(best) {
				best = members
			}
		}
	}
	return best
}

// run walks the beat one seed pair sets, and returns the members found on it.
//
// # Each row is compared to the one ABOVE it, not to the seed
//
// That is the whole of what this function changed, and the measurement behind it is specific.
// On a frame of a real pause menu whose nine entries were every one proposed and every one
// surviving deduplication, the run found FOUR and stopped. The rows are not identical: this
// menu's panel is drawn slanted, so each row's left edge sits about half a per cent of the
// frame further left than the one above it. Adjacent rows are well inside the alignment
// tolerance. The sixth row is not — against the SEED.
//
// So comparing every member to the seed made the test stricter the longer the run got, and a
// list could never be longer than the distance over which its own styling stayed still. On
// this corpus that capped it at four rows of nine, which is why entry reading measured zero.
//
// Comparing neighbours is also what a list IS: each row looks like the row above it. The seed
// keeps one veto, on HEIGHT, because height is what makes a row a row and an unchecked chain of
// neighbourly comparisons would be free to walk from a menu entry to a panel.
//
// # Two variants were tried here and are not here
//
// SKIPPING AN EMPTY BEAT, so an unreadable row would not end the run. It is the obvious
// companion to the above and it made things worse, measurably: with up to two consecutive
// beats skippable, a nine-member progression formed across the ENTIRE frame on the menu's own
// pitch, picking up an unrelated box every second or third beat, and it beat the menu's six on
// size. The menu's rows were then consumed by it. Without skipping, the same frame's largest
// run IS the menu. A looser matcher in a field of two hundred proposals finds more
// coincidences than structure.
//
// BOUNDING THE PITCH against the members' own height — "rows of a list are adjacent" — to
// exclude those coincidences instead. On the real menu the ratio is 1.07 and the coincidences
// ran from 2.1 to 34.9, so it separates them cleanly there. It also broke every drawn fixture:
// a proposal is often the TEXT LINE inside a row rather than the row, and text is much shorter
// than its row, so a legitimate list of text lines has a ratio of three or four. Discovery on
// the desktop fixture went from six rows to none. The ratio is a fact about what perception
// proposed, not about the list.
func run(boxes []image.Rectangle, taken map[int]bool, i, j int, pitch float64) []int {
	members := []int{i}
	prev := i
	want := centreY(boxes[i]) + pitch
	for k := j; k < len(boxes); k++ {
		if taken[k] {
			continue
		}
		if math.Abs(centreY(boxes[k])-want) > pitchTolerance*pitch {
			continue
		}
		if !alike(boxes[prev], boxes[k]) {
			continue
		}
		// THE SEED'S ONE VETO. A chain of neighbours must not drift in scale.
		if relDiff(float64(boxes[i].Dy()), float64(boxes[k].Dy())) > stackTolerance {
			continue
		}
		members = append(members, k)
		prev = k
		want += pitch
	}
	return members
}

func stackPitch(boxes []image.Rectangle, members []int) float64 {
	if len(members) < 2 {
		return 0
	}
	first, last := centreY(boxes[members[0]]), centreY(boxes[members[len(members)-1]])
	return (last - first) / float64(len(members)-1)
}

// alike reports whether two regions could be rows of one list.
//
// # Height, and then ALIGNMENT rather than width
//
// Width similarity was the horizontal test, and it encoded an assumption about where geometry
// comes from: the learned detector proposes a box per row at the row's full width, so a menu's
// rows all measure the same. Anything that finds text instead proposes the extent of the WORDS,
// and the words in a list are of different lengths — "MAP" against "OPTIONS" differ by 83%,
// against a tolerance of 35%.
//
// Measured, on a drawn six-entry settings list whose every row was correctly proposed as a text
// line: the stack came out EMPTY. Discovery was complete and grouping refused all of it, so the
// tracker never saw a candidate and the belief reported insufficient evidence — which is
// indistinguishable, in the report, from having seen nothing at all.
//
// What a list actually shares is a common edge and a regular pitch. Left-aligned is the usual
// case and centred is common in games, so either satisfies this; the tolerance is half a row's
// height, which is tight enough to separate two adjacent columns and loose enough for the
// ragged left edge of an indented sub-item.
//
// Width is NOT merely dropped. A pair whose widths are similar is still alike by the old test,
// so full-width row boxes from the detector group exactly as they did; alignment is an
// additional way to be alike, not a replacement. That is what keeps the existing corpus — a
// measured Trackmania frame where the list is seven rows in a pile of 135 — scoring as before.
func alike(a, b image.Rectangle) bool {
	if relDiff(float64(a.Dy()), float64(b.Dy())) > stackTolerance {
		return false
	}
	if relDiff(float64(a.Dx()), float64(b.Dx())) <= stackTolerance {
		return true
	}
	tol := 0.5 * math.Max(float64(a.Dy()), float64(b.Dy()))
	leftAligned := math.Abs(float64(a.Min.X-b.Min.X)) <= tol
	centred := math.Abs(centreX(a)-centreX(b)) <= tol
	return leftAligned || centred
}

// centreX is a rectangle's horizontal midpoint.
func centreX(r image.Rectangle) float64 { return float64(r.Min.X+r.Max.X) / 2 }

// Dedupe is dedupe, for the replay's grouping diagnostic.
//
// Exported rather than reimplemented: the question the diagnostic asks is what the grouping was
// GIVEN, and a second copy of this filter would eventually answer a different question. See
// observe.BoxesForLook, which exists for the same reason one layer out.
func Dedupe(boxes []image.Rectangle) []image.Rectangle { return dedupe(boxes) }

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
