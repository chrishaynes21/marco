package main

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
)

// WHAT GROUPING DID TO ONE FRAME, beside what the labels say was on it.
//
// # Why this exists
//
// Because every aggregate the replay prints can be healthy while the pipeline reads the wrong
// list, and that is not hypothetical: on this corpus "discovery 829 of 829" and "grouping 864
// of 864" were both true while the identified list was a four-row fragment and the menu in
// front of it has nine entries. No aggregate can say that. The geometry of one frame can.
//
// # Two coordinate spaces, kept apart deliberately
//
// A proposal is in IMAGE PIXELS. The grouping works in observe.InterfaceGrid, a fixed square
// space that normalised regions are scaled into so a belief does not change when a window is
// resized. Mixing them is how the first version of this diagnostic reported a row at y 9.259,
// so each column below says which space it is in.
func inspectFrame(dir string, index int) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	var ann *annotationFile
	if a, err := readAnnotations(dir); err == nil {
		ann = a
	}
	var want *frameRecord
	for i := range frames {
		if frames[i].Index == index {
			want = &frames[i]
			break
		}
	}
	if want == nil {
		return fmt.Errorf("%s has no frame %d", dir, index)
	}
	img, err := readPNG(filepath.Join(dir, filepath.FromSlash(want.File)))
	if err != nil {
		return err
	}
	bounds := img.Bounds()
	res := structure.Propose(img, structure.DefaultOptions())
	shapes := shapesFrom(img, res.Candidates)

	// THE SAME INPUT PRODUCTION GETS, through the same one statement of it. A diagnostic
	// that rebuilt the grouping's input would be measuring the diagnostic.
	boxes := observe.BoxesForLook(observe.InterfaceLook{Observed: true, Shapes: shapes})
	kept := selection.Dedupe(boxes)
	rows := selection.GroupRows(boxes)

	fmt.Printf("frame %d of %s: %dx%d, %d proposals, %d measurable, %d grouped rows\n",
		index, filepath.Base(dir), bounds.Dx(), bounds.Dy(), len(res.Candidates),
		len(boxes), len(rows))

	byGroup := map[int][]image.Rectangle{}
	var order []int
	for _, r := range rows {
		if _, seen := byGroup[r.Group]; !seen {
			order = append(order, r.Group)
		}
		byGroup[r.Group] = append(byGroup[r.Group], r.Bounds)
	}
	sort.Ints(order)
	grid := float64(observe.InterfaceGrid)
	fmt.Printf("\n  groups, in the grid the grouping works in (%.0f per normalised unit):\n",
		grid)
	fmt.Printf("    %-6s %-5s %-15s %-9s %-9s %-7s %s\n",
		"group", "rows", "y (normalised)", "pitch", "height", "p/h", "x..x")
	for _, g := range order {
		members := byGroup[g]
		if len(members) < 2 {
			continue
		}
		sort.Slice(members, func(i, j int) bool {
			return centreOf(members[i]) < centreOf(members[j])
		})
		first, last := members[0], members[len(members)-1]
		pitch := (centreOf(last) - centreOf(first)) / float64(len(members)-1)
		h := 0
		for _, m := range members {
			h += m.Dy()
		}
		mean := float64(h) / float64(len(members))
		// p/h is the number that says whether these are rows at all. Two rows of one
		// list cannot occupy the same place, so a stack whose pitch is a fraction of
		// its own row height is near-duplicate proposals of ONE row, not a list.
		fmt.Printf("    %-6d %-5d %-15s %-9.0f %-9.0f %-7.2f %.3f..%.3f\n",
			g, len(members),
			fmt.Sprintf("%.3f..%.3f", float64(first.Min.Y)/grid, float64(last.Max.Y)/grid),
			pitch, mean, pitch/math.Max(mean, 1),
			float64(first.Min.X)/grid, float64(first.Max.X)/grid)
	}

	if ann == nil {
		return nil
	}
	labelled := ann.labelledRows()
	if len(labelled) == 0 {
		return nil
	}
	state, y, _ := ann.labelAt(index)
	fmt.Printf("\n  the annotation says this frame is %q at y %.3f, of %d labelled rows\n",
		state, y, len(labelled))
	fmt.Printf("    %-8s %-11s %-10s %-6s %s\n", "row", "labelled y", "proposed", "kept", "in a group")
	for i, at := range labelled {
		// Each space asks the question in its own units.
		py := at * float64(bounds.Dy())
		gy := at * grid
		proposed := 0
		for _, c := range res.Candidates {
			if float64(c.Bounds.Min.Y) <= py && py <= float64(c.Bounds.Max.Y) {
				proposed++
			}
		}
		// AND HOW MANY SURVIVED DEDUPE, which is the grouping's actual input. The
		// gap between these two columns is evidence the filter ate the row.
		survived := 0
		for _, k := range kept {
			if float64(k.Min.Y) <= gy && gy <= float64(k.Max.Y) {
				survived++
			}
		}
		// THE MOST ROW-LIKE MATCH, not the first. A tall block spanning four menu
		// entries contains all four of their centres, and reporting it as their
		// group said four rows were grouped when none of them was.
		grouped, best := -1, 0
		var box image.Rectangle
		for _, r := range rows {
			if float64(r.Bounds.Min.Y) > gy || gy > float64(r.Bounds.Max.Y) {
				continue
			}
			if grouped < 0 || r.Bounds.Dy() < best {
				grouped, box, best = r.Group, r.Bounds, r.Bounds.Dy()
			}
		}
		in := "-"
		if grouped >= 0 {
			in = fmt.Sprintf("group %-3d of %d rows   y %.3f h %.3f x %.3f w %.3f",
				grouped, len(byGroup[grouped]),
				float64(box.Min.Y)/grid, float64(box.Dy())/grid,
				float64(box.Min.X)/grid, float64(box.Dx())/grid)
		}
		mark := " "
		if math.Abs(at-y) < 0.01 {
			mark = "*"
		}
		fmt.Printf("    %s%-7d %-11.3f %-10d %-6d %s\n", mark, i, at, proposed, survived, in)
	}
	fmt.Printf("    (* is the row the labels say was selected)\n")
	inspectKept(kept, rows, labelled[0]-0.04, labelled[len(labelled)-1]+0.04)
	inspectBand(img, res, labelled[0]-0.04, labelled[len(labelled)-1]+0.04)
	return nil
}

// readAnnotations is the one place the label file is parsed, shared with the replay.
func readAnnotations(dir string) (*annotationFile, error) {
	b, err := os.ReadFile(filepath.Join(dir, "annotations.json"))
	if err != nil {
		return nil, err
	}
	var a annotationFile
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// centreOf is a rectangle's vertical midpoint.
func centreOf(r image.Rectangle) float64 { return float64(r.Min.Y+r.Max.Y) / 2 }

// inspectBand prints every proposal whose centre lies in a band of the frame.
//
// The question an aggregate cannot answer: is a menu row proposed as ONE rectangle or as
// several? "A proposal covered the selected row" is satisfied by the panel behind it, and a
// row offered to the grouping as four word-fragments at four different left edges has nothing
// a regular stack can be built from.
func inspectBand(img image.Image, res structure.Result, lo, hi float64) {
	b := img.Bounds()
	type row struct {
		c structure.Candidate
		y float64
	}
	var in []row
	for _, c := range res.Candidates {
		cy := float64(c.Bounds.Min.Y+c.Bounds.Max.Y) / 2 / float64(b.Dy())
		if cy >= lo && cy <= hi {
			in = append(in, row{c, cy})
		}
	}
	sort.Slice(in, func(i, j int) bool { return in[i].y < in[j].y })
	fmt.Printf("\n  every proposal centred in y %.3f..%.3f (%d of %d):\n",
		lo, hi, len(in), len(res.Candidates))
	fmt.Printf("    %-8s %-6s %-7s %-7s %-7s %s\n",
		"centre", "kind", "x", "w", "h", "fill/contrast")
	for _, r := range in {
		fmt.Printf("    %-8.4f %-6s %-7.3f %-7.3f %-7.3f %.2f/%.0f\n",
			r.y, r.c.Kind,
			float64(r.c.Bounds.Min.X)/float64(b.Dx()),
			float64(r.c.Bounds.Dx())/float64(b.Dx()),
			float64(r.c.Bounds.Dy())/float64(b.Dy()),
			r.c.Fill, r.c.Contrast)
	}
}

// inspectKept prints the boxes the grouping was actually given, in a band.
//
// The decisive view. "A proposal covered the selected row" is satisfied by the panel behind
// it, and "three boxes survived dedupe over this row" counts the tall ones too. This is the
// set a regular stack has to be found in.
func inspectKept(kept []image.Rectangle, rows []selection.Row, lo, hi float64) {
	grid := float64(observe.InterfaceGrid)
	group := map[image.Rectangle]int{}
	for _, r := range rows {
		group[r.Bounds] = r.Group
	}
	type row struct {
		b image.Rectangle
		y float64
	}
	var in []row
	for _, b := range kept {
		cy := float64(b.Min.Y+b.Max.Y) / 2 / grid
		if cy >= lo && cy <= hi {
			in = append(in, row{b, cy})
		}
	}
	sort.Slice(in, func(i, j int) bool { return in[i].y < in[j].y })
	fmt.Printf("\n  the %d boxes grouping was given, centred in y %.3f..%.3f:\n",
		len(in), lo, hi)
	fmt.Printf("    %-8s %-7s %-7s %-7s %s\n", "centre", "x", "w", "h", "group")
	for _, r := range in {
		g := "dropped"
		if n, ok := group[r.b]; ok {
			g = fmt.Sprintf("%d", n)
		}
		fmt.Printf("    %-8.4f %-7.3f %-7.3f %-7.3f %s\n", r.y,
			float64(r.b.Min.X)/grid, float64(r.b.Dx())/grid, float64(r.b.Dy())/grid, g)
	}
}
