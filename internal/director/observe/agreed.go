package observe

// Overlap is the part of two regions that both cover, and whether they cover any part together.
func Overlap(a, b Region) (Region, bool) {
	x0, y0 := max64(a.X, b.X), max64(a.Y, b.Y)
	x1 := min64(a.X+a.Width, b.X+b.Width)
	y1 := min64(a.Y+a.Height, b.Y+b.Height)
	if x1 <= x0 || y1 <= y0 {
		return Region{}, false
	}
	return Region{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}, true
}

// Area is how much of the window a region covers.
func (r Region) Area() float64 {
	if r.Width <= 0 || r.Height <= 0 {
		return 0
	}
	return r.Width * r.Height
}

// Agreed is the part of a re-acquired region that the saved readings also describe.
//
// # Why the match is not the target
//
// Re-acquisition compares rectangles and answers "is this the thing I saw before". It is right
// to be generous about that: a proposal that extends past the control, or merges it with what is
// beside it, is still evidence that the control is there. What it is NOT is a statement about
// where to press.
//
// The difference was measured. A live look matched Rocket League's PLAY button with a region
// running from the window's left edge to 132px in — background, and then the left sliver of a
// button that starts at 62px and ends at 370px. The match was correct. Its middle, at x 66, was
// three pixels inside the button's border, and the click did nothing.
//
// So the point comes from the part BOTH describe: what the demonstration recorded the control
// as, and what this look found. Where they agree is the strongest evidence available about where
// the control actually is, and it is bounded by the smaller of the two rather than dragged
// outwards by the larger.
//
// # Why no overlap is a refusal
//
// Because the alternative is picking one of two rectangles that contradict each other, and
// nothing here can say which. A matcher that said "this is the control" about a region the saved
// readings do not touch has found something, and what it found is not evidence about where to
// press.
func Agreed(found Region, saved []Region) (Region, bool) {
	best, ok := Region{}, false
	for _, s := range saved {
		got, over := Overlap(found, s)
		if !over {
			continue
		}
		if !ok || got.Area() > best.Area() {
			best, ok = got, true
		}
	}
	return best, ok
}
