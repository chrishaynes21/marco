package screen

import "image"

// Colour-histogram signature: a compact, ORDER-INVARIANT description of a button's colour
// palette. Compared to a single recorded pixel it's far more robust (a stray anti-aliased
// pixel or a one-pixel-off resolve barely moves it) and it confirms the whole control's
// colour distribution — complementary to the spatial pixel match, which a small warp or
// partial occlusion can degrade even when the palette is intact.

// histBinsPerChannel quantises each RGB channel into this many levels (so the signature
// has histBinsPerChannel³ buckets) — coarse enough to be robust to small colour shifts.
const histBinsPerChannel = 4

// ColorHistogram returns the (4×4×4 = 64-bucket) RGB colour histogram of img.
//
// Exported so an offline experiment can ask whether appearance adds anything geometry does not,
// without copying the implementation into a second place where the two could drift. It is a pure
// function of the pixels handed to it and reaches nothing: the capture in this package is
// elsewhere in it, and nothing in the observation tier may import this package for that reason.
func ColorHistogram(img *image.RGBA) [histBinsPerChannel * histBinsPerChannel * histBinsPerChannel]int {
	var h [histBinsPerChannel * histBinsPerChannel * histBinsPerChannel]int
	b := img.Bounds()
	const shift = 6 // 8-bit channel → top 2 bits (4 levels)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := img.PixOffset(x, y)
			r := int(img.Pix[i]) >> shift
			g := int(img.Pix[i+1]) >> shift
			bl := int(img.Pix[i+2]) >> shift
			h[(r*histBinsPerChannel+g)*histBinsPerChannel+bl]++
		}
	}
	return h
}

// Histogram is one 64-bucket RGB colour histogram.
type Histogram = [histBinsPerChannel * histBinsPerChannel * histBinsPerChannel]int

// HistIntersection returns the normalised histogram-intersection similarity of two colour
// histograms, in [0,1] — 1.0 when the palettes are identical, 0 when they share nothing.
func HistIntersection(a, b [histBinsPerChannel * histBinsPerChannel * histBinsPerChannel]int) float64 {
	sa, sb := 0, 0
	for i := range a {
		sa += a[i]
		sb += b[i]
	}
	if sa == 0 || sb == 0 {
		return 0
	}
	inter := 0.0
	for i := range a {
		inter += min(float64(a[i])/float64(sa), float64(b[i])/float64(sb))
	}
	return inter
}
