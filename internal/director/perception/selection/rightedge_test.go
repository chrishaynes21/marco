package selection_test

import (
	"image"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// A list is not cut in half by one row the detector found badly.
//
// # The measurement
//
// A list shares an edge and a regular pitch, and `alike` tested the left edge and the centre.
// On the Trackmania corpus that cut a nine-entry menu in half. Over the 312 frames that
// published a list, the row the person was looking at was detected IN THE LIST'S OWN COLUMN on
// every one of them, was in a stack grouping made on 197, and was a member of the tracked list
// on 85 — and of the 112 lost in between, 109 were in a stack holding none of the list's
// members.
//
// Frame 273 is the whole story in one row. The menu's rows are height 0.052 and width 0.388.
// One row of it came back as y 0.470..0.524, width 0.180: the right height at the right beat
// and less than half the width. Its left edge is a fifth of the frame right of its siblings'
// and its centre a tenth, so neither test could see it as one of them, and the run ended there
// — leaving everything above it in a separate stack.
//
// Its RIGHT edge is 0.460, against 0.455 for the row below it and 0.467 for the row above. A
// detector that finds part of a right-aligned row finds the part that ends where the row ends.
//
// # What this fixture is
//
// The same shape, in the small: five rows of one width, the middle one returned at a third of
// it and sharing their right edge. The whole five must group as one stack.
//
// Deleting the right-edge term from `alike` must fail this.
func TestOneBadlyFoundRowDoesNotCutAListInHalf(t *testing.T) {
	const (
		x1    = 400 // the shared right edge
		w     = 300
		h     = 40
		pitch = 60
	)
	var boxes []image.Rectangle
	for i := range 5 {
		y := 100 + i*pitch
		left := x1 - w
		if i == 2 {
			// THE BADLY FOUND ROW: a third of the width, ending where the others
			// end, which is what a partial detection of a right-aligned row looks
			// like.
			left = x1 - w/3
		}
		boxes = append(boxes, image.Rect(left, y, x1, y+h))
	}

	rows := selection.GroupRows(boxes)
	size := map[int]int{}
	for _, r := range rows {
		size[r.Group]++
	}
	var biggest int
	for _, n := range size {
		if n > biggest {
			biggest = n
		}
	}
	if biggest != 5 {
		t.Errorf("the largest stack has %d of the 5 rows, in %d group(s). One row found "+
			"at a third of its width, sharing the right edge of its siblings, splits "+
			"the list: %v", biggest, len(size), size)
	}
	// AND THE BADLY FOUND ROW IS IN IT, rather than the four tidy ones grouping alone.
	var withNarrow int
	for _, r := range rows {
		if r.Bounds.Dx() < w/2 {
			withNarrow = size[r.Group]
		}
	}
	if withNarrow != 5 {
		t.Errorf("the narrow row is in a group of %d; it is one of the five and the beat "+
			"it sits on is what joins the rows above it to the rows below",
			withNarrow)
	}
}

// Two columns side by side are still two lists.
//
// The right edge is a third way to be alike, and a looser test finds more coincidences. Two
// lists at the same heights in different columns share neither edge nor centre, and must not
// be joined by the new term any more than by the old ones.
//
// Making the right-edge tolerance wide enough to span two columns must fail this.
func TestTwoColumnsAtTheSameHeightsAreStillTwoLists(t *testing.T) {
	const (
		h     = 40
		pitch = 60
	)
	var boxes []image.Rectangle
	for i := range 5 {
		y := 100 + i*pitch
		boxes = append(boxes, image.Rect(100, y, 400, y+h))
		boxes = append(boxes, image.Rect(600, y, 900, y+h))
	}
	rows := selection.GroupRows(boxes)
	size := map[int]int{}
	span := map[int][2]int{}
	for _, r := range rows {
		size[r.Group]++
		if cur, seen := span[r.Group]; !seen {
			span[r.Group] = [2]int{r.Bounds.Min.X, r.Bounds.Max.X}
		} else {
			if r.Bounds.Min.X < cur[0] {
				cur[0] = r.Bounds.Min.X
			}
			if r.Bounds.Max.X > cur[1] {
				cur[1] = r.Bounds.Max.X
			}
			span[r.Group] = cur
		}
	}
	for g, n := range size {
		if n > 5 {
			t.Errorf("group %d has %d rows; two columns of five at the same heights "+
				"are two lists, and a stack of more than five has joined them: %v",
				g, n, size)
		}
	}
	// AND NO GROUP STRADDLES BOTH COLUMNS. The two are at the same heights, so a stack
	// that mixes them is the same size as one that does not and a count alone cannot see
	// it — what gives it away is the width it spans.
	for g, sp := range span {
		if w := sp[1] - sp[0]; w > 400 {
			t.Errorf("group %d spans x %d..%d, which is %d wide; one column is 300 "+
				"wide, so this group has rows from both of them", g, sp[0],
				sp[1], w)
		}
	}
}

// A list of left-aligned rows of different widths is still a list.
//
// The left edge is the usual case and it was here before the right one. Text lines share it and
// nothing else: "MAP" and "OPTIONS" differ in width by 83%, and their centres and right edges
// differ with them. A third way to be alike must not have replaced the first.
//
// Deleting the left-edge term from `alike` must fail this.
func TestAListOfLeftAlignedRowsOfDifferentWidthsIsStillAList(t *testing.T) {
	const (
		x0    = 100
		h     = 40
		pitch = 60
	)
	widths := []int{90, 300, 140, 260, 110}
	var boxes []image.Rectangle
	for i, w := range widths {
		y := 100 + i*pitch
		boxes = append(boxes, image.Rect(x0, y, x0+w, y+h))
	}
	size := map[int]int{}
	for _, r := range selection.GroupRows(boxes) {
		size[r.Group]++
	}
	var biggest int
	for _, n := range size {
		if n > biggest {
			biggest = n
		}
	}
	if biggest != 5 {
		t.Errorf("the largest stack has %d of 5 left-aligned rows, in %d group(s). "+
			"Their widths differ by a factor of three and their right edges with "+
			"them, so the left edge is the only thing that says they are one "+
			"list: %v", biggest, len(size), size)
	}
}
