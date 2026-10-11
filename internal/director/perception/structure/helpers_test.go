package structure_test

import (
	"image"
	"image/color"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// grey is a flat ground that is neither black nor white, so "unmeasured" and "dark" stay
// distinguishable in anything that reads these fixtures.
var grey = color.RGBA{128, 128, 132, 255}

// sizesOf runs the PRODUCTION grouper over some rectangles and returns the group sizes,
// largest first.
//
// Through selection.GroupRows and not a local reimplementation: the question these tests ask
// is whether proposals read as a list to the code that will actually judge them. A second
// grouping rule living in a test is the "two lists, one fact" defect this repository has
// already paid for once.
func sizesOf(rows []image.Rectangle) []int {
	count := map[int]int{}
	for _, r := range selection.GroupRows(rows) {
		count[r.Group]++
	}
	out := make([]int, 0, len(count))
	for _, n := range count {
		out = append(out, n)
	}
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] > out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// indexOf is strings.Index, named locally so the fixture family can be split out of a name
// without the test file importing strings for one call.
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
