package selection

import (
	"image"
	"math"
)

// Finding the candidate rows, without being told where they are.
//
// A list of choices is drawn as a stack of bands of similar height, separated by something
// visible — a rule, an edge, a change of shade. That is true of a game menu, a settings list
// and a dropdown, and it is true whatever colour any of them happens to be. So the bands are
// found from where the picture changes VERTICALLY, and nothing here knows a row's height, its
// spacing or its colour in advance.
//
// This is the weakest part of the package and is meant to be. Rows in production should come
// from perception, which has far better evidence than a luminance profile; this exists so the
// reader can be measured end to end from pixels alone rather than from coordinates somebody
// wrote down after looking at the answer.

const (
	// edgeK is how much stronger than typical a vertical change must be to count as a
	// boundary between rows.
	edgeK = 3.0
	// minRowHeight discards the hairlines and sub-pixel artefacts that any edge finder
	// produces. In fractions of the region's height, so it does not encode a resolution.
	minRowHeight = 0.02
	// maxRowHeight discards a "band" that is really the whole panel.
	maxRowHeight = 0.25
	// groupTolerance is how much two neighbouring bands' height and spacing may differ
	// and still be called siblings. Past it a new group starts, which is how a row that
	// sits apart from the stack — set off by a gap — ends up with no siblings and is
	// judged temporally instead.
	groupTolerance = 0.35
)

// FindRows locates candidate rows inside a region by looking for horizontal boundaries.
func FindRows(img image.Image, region image.Rectangle) []Row {
	region = region.Intersect(img.Bounds())
	if region.Dy() < 8 || region.Dx() < 8 {
		return nil
	}
	profile := make([]float64, region.Dy())
	for i := range profile {
		y := region.Min.Y + i
		var sum float64
		var n int
		for x := region.Min.X; x < region.Max.X; x += 2 {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			sum += 0.299*float64(cr>>8) + 0.587*float64(cg>>8) + 0.114*float64(cb>>8)
			n++
		}
		if n > 0 {
			profile[i] = sum / float64(n)
		}
	}

	grad := make([]float64, len(profile))
	for i := 1; i < len(profile)-1; i++ {
		grad[i] = math.Abs(profile[i+1] - profile[i-1])
	}
	// Choose the threshold that produces the most LIST-LIKE segmentation, rather than
	// trusting one constant.
	//
	// A single threshold cannot work: the edges between rows and the edges around the
	// text drawn inside them are both real changes in brightness, and which is stronger
	// is a fact about a typeface. Set it low and every row splits into three; set it high
	// and whole rows vanish. Measured on the menu this was built against, a fixed
	// threshold found nineteen bands where there are nine, and the reader then answered
	// confidently about the wrong strips.
	//
	// So the same idea the reader uses applies here: a list is a REPEATING structure. Of
	// the segmentations available, the one whose bands most agree on a height is the one
	// that found rows rather than letterforms.
	mid := median(grad)
	best, bestScore := math.Max(mid*edgeK, 2), -1.0
	for _, k := range []float64{1.5, 2, 3, 4, 6, 9, 14, 20} {
		t := math.Max(mid*k, 2)
		if s := regularity(segment(grad, t, region.Dy())); s > bestScore {
			best, bestScore = t, s
		}
	}
	threshold := best

	// A boundary is the middle of a RUN of strong change, not a peak in it.
	//
	// A sharp edge between two flat rows does not produce a peak: a centred difference
	// over a step is equally large at every offset that straddles the step, so the
	// gradient comes out as a short plateau. Looking for a local maximum finds nothing at
	// all on exactly the clean case this is for — which is how the first version of this
	// returned one band for a picture of five.
	var bounds []int
	for i := 1; i < len(grad)-1; {
		if grad[i] < threshold {
			i++
			continue
		}
		j := i
		for j < len(grad)-1 && grad[j] >= threshold {
			j++
		}
		bounds = append(bounds, (i+j)/2)
		i = j
	}
	if len(bounds) < 2 {
		return nil
	}

	minH := int(minRowHeight * float64(region.Dy()))
	maxH := int(maxRowHeight * float64(region.Dy()))
	type band struct{ top, bottom int }
	var bands []band
	for i := 0; i+1 < len(bounds); i++ {
		h := bounds[i+1] - bounds[i]
		if h >= minH && h <= maxH {
			bands = append(bands, band{bounds[i], bounds[i+1]})
		}
	}
	if len(bands) == 0 {
		return nil
	}

	// Siblings are bands that continue the stack: same height, same spacing as the one
	// before. A break in either starts a new group.
	out := make([]Row, 0, len(bands))
	group := 0
	var prevH, prevGap float64
	for i, b := range bands {
		h := float64(b.bottom - b.top)
		if i > 0 {
			gap := float64(b.top - bands[i-1].top)
			if relDiff(h, prevH) > groupTolerance ||
				(prevGap > 0 && relDiff(gap, prevGap) > groupTolerance) {
				group++
			}
			prevGap = gap
		}
		prevH = h
		out = append(out, Row{
			Bounds: image.Rect(region.Min.X, region.Min.Y+b.top, region.Max.X, region.Min.Y+b.bottom),
			Group:  group,
		})
	}
	return out
}

// segment returns the band heights a threshold would produce.
func segment(grad []float64, threshold float64, regionH int) []float64 {
	var bounds []int
	for i := 1; i < len(grad)-1; {
		if grad[i] < threshold {
			i++
			continue
		}
		j := i
		for j < len(grad)-1 && grad[j] >= threshold {
			j++
		}
		bounds = append(bounds, (i+j)/2)
		i = j
	}
	minH := int(minRowHeight * float64(regionH))
	maxH := int(maxRowHeight * float64(regionH))
	var heights []float64
	for i := 0; i+1 < len(bounds); i++ {
		if h := bounds[i+1] - bounds[i]; h >= minH && h <= maxH {
			heights = append(heights, float64(h))
		}
	}
	return heights
}

// regularity scores how much a segmentation looks like a list.
//
// The count of bands that agree with the median height. Agreement rather than count alone,
// because a threshold that shatters every row into three produces many bands and no
// structure, and one that merges everything produces structure and no bands.
func regularity(heights []float64) float64 {
	if len(heights) < 3 {
		return -1
	}
	mid := median(heights)
	if mid <= 0 {
		return -1
	}
	agree := 0
	for _, h := range heights {
		if relDiff(h, mid) <= groupTolerance/2 {
			agree++
		}
	}
	// A run of agreeing bands is worth more than the same number scattered among
	// disagreeing ones, so the fraction that agree multiplies the count.
	return float64(agree) * float64(agree) / float64(len(heights))
}

func relDiff(a, b float64) float64 {
	if a == 0 && b == 0 {
		return 0
	}
	return math.Abs(a-b) / math.Max(math.Abs(a), math.Abs(b))
}
