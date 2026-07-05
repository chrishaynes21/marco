package screen

import (
	"image"
	"image/color"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// findScales are the template scales tried when matching, so a DPI/resolution change
// between recording and replay (the on-screen control a different size than the template)
// doesn't break the match. 1.0 is first (the common same-DPI case, preferred on ties).
// Tunable via $MARCO_FIND_SCALES (comma-separated, e.g. "0.67,1.0,1.5").
func findScales() []float64 {
	if v := strings.TrimSpace(os.Getenv("MARCO_FIND_SCALES")); v != "" {
		var out []float64
		for p := range strings.SplitSeq(v, ",") {
			if f, err := strconv.ParseFloat(strings.TrimSpace(p), 64); err == nil && f > 0 {
				out = append(out, f)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return []float64{1.0, 0.8, 1.25}
}

// resize returns src scaled by the given factor using bilinear interpolation (smooth, so
// a scaled template still aligns with anti-aliased screen content). scale 1.0 returns src.
func resize(src *image.RGBA, scale float64) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dw, dh := int(float64(sw)*scale+0.5), int(float64(sh)*scale+0.5)
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	if dw == sw && dh == sh {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		fy := (float64(y)+0.5)/scale - 0.5
		y0 := int(math.Floor(fy))
		ty := fy - float64(y0)
		for x := 0; x < dw; x++ {
			fx := (float64(x)+0.5)/scale - 0.5
			x0 := int(math.Floor(fx))
			tx := fx - float64(x0)
			r := bilerp(src, sw, sh, x0, y0, tx, ty, 0)
			g := bilerp(src, sw, sh, x0, y0, tx, ty, 1)
			bl := bilerp(src, sw, sh, x0, y0, tx, ty, 2)
			a := bilerp(src, sw, sh, x0, y0, tx, ty, 3)
			dst.SetRGBA(x, y, color.RGBA{r, g, bl, a})
		}
	}
	return dst
}

// bilerp bilinearly interpolates channel c of src at (x0+tx, y0+ty), clamping to bounds.
func bilerp(src *image.RGBA, w, h, x0, y0 int, tx, ty float64, c int) uint8 {
	at := func(x, y int) float64 {
		if x < 0 {
			x = 0
		} else if x >= w {
			x = w - 1
		}
		if y < 0 {
			y = 0
		} else if y >= h {
			y = h - 1
		}
		return float64(src.Pix[src.PixOffset(src.Rect.Min.X+x, src.Rect.Min.Y+y)+c])
	}
	top := at(x0, y0)*(1-tx) + at(x0+1, y0)*tx
	bot := at(x0, y0+1)*(1-tx) + at(x0+1, y0+1)*tx
	return uint8(top*(1-ty) + bot*ty + 0.5)
}

// This file is the BUTTON-RECOGNITION / computer-vision layer, all pure Go and
// OS-agnostic:
//   - edges: a binary gradient map a button's border/text/icon stands out in,
//   - DetectButtons: segment an image into button-like rectangles (connected edge
//     structure) — used to crop an anchor to the actual control,
//   - findEdgeDT: TOLERANT edge-template matching via a distance transform, so a
//     button's OUTLINE matches even after a recolour/theme change AND despite
//     anti-aliasing or a small sub-pixel shift (the edge-match SIGNAL).

// edgeBinaryMap returns a row-major bool grid marking edge pixels of img.
func edgeBinaryMap(img *image.RGBA) []bool {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]bool, w*h)
	for y := 0; y < h-1; y++ {
		for x := 0; x < w-1; x++ {
			if edgeStrength(img, x, y) > EdgeTolerance {
				out[y*w+x] = true
			}
		}
	}
	return out
}

// distanceTransform returns, for each pixel, the (Manhattan, capped) distance to the
// nearest edge — a two-pass chamfer transform. This is what makes edge matching tolerant:
// a template edge that lands within a few pixels of a screen edge still scores well.
func distanceTransform(edge []bool, w, h, cap int) []int {
	dt := make([]int, w*h)
	for i, e := range edge {
		if e {
			dt[i] = 0
		} else {
			dt[i] = cap
		}
	}
	for y := range h {
		for x := range w {
			i := y*w + x
			if x > 0 && dt[i-1]+1 < dt[i] {
				dt[i] = dt[i-1] + 1
			}
			if y > 0 && dt[i-w]+1 < dt[i] {
				dt[i] = dt[i-w] + 1
			}
		}
	}
	for y := h - 1; y >= 0; y-- {
		for x := w - 1; x >= 0; x-- {
			i := y*w + x
			if x < w-1 && dt[i+1]+1 < dt[i] {
				dt[i] = dt[i+1] + 1
			}
			if y < h-1 && dt[i+w]+1 < dt[i] {
				dt[i] = dt[i+w] + 1
			}
		}
	}
	return dt
}

// edgeMatchThreshold is the min average edge closeness (0..1) to call an outline a match —
// the edge locator's found-gate. Dial-driven (s=0.5 → 0.55 legacy); lower lets a present
// button's outline register as found on a busy/anti-aliased screen. $MARCO_EDGE_MATCH
// overrides.
var edgeMatchThreshold = edgeMatchThresholdFromEnv()

func edgeMatchThresholdFromEnv() float64 {
	if v := strings.TrimSpace(os.Getenv("MARCO_EDGE_MATCH")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f <= 1 {
			return f
		}
	}
	return cvLerp(0.70, 0.40) // s=0.5 → 0.55 (legacy)
}

// Edge-match tuning.
const (
	edgeSlack     = 4   // px a template edge may be from a screen edge and still count
	minEdgePoints = 24  // a template with fewer edges has too little shape to match on
	edgePointCap  = 160 // subsample template edges to this many for speed
	orientBins    = 4   // gradient-orientation buckets over [0°,180°)
)

// orientationBin buckets a gradient direction into one of orientBins orientations over
// [0°,180°), folding 180° so a dark→light and light→dark edge of the same line share a
// bin (matching stays robust to contrast inversion). Used to require that template and
// screen edges run the same WAY, not merely that some edge is nearby.
func orientationBin(gx, gy int) int {
	a := math.Atan2(float64(gy), float64(gx)) * 180 / math.Pi // (-180,180]
	for a < 0 {
		a += 180
	}
	for a >= 180 {
		a -= 180
	}
	b := int(a / (180.0 / orientBins))
	if b >= orientBins {
		b = orientBins - 1
	}
	return b
}

// orientedEdgePoints returns the template's edge pixels as {x, y, orientationBin},
// subsampled to at most edgePointCap (the outline shape is preserved by even sampling).
func orientedEdgePoints(img *image.RGBA) [][3]int {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	var pts [][3]int
	for y := 0; y < h-1; y++ {
		for x := 0; x < w-1; x++ {
			gx, gy := sobelGrad(img, x, y)
			if iabs(gx)+iabs(gy) > EdgeTolerance {
				pts = append(pts, [3]int{x, y, orientationBin(gx, gy)})
			}
		}
	}
	if len(pts) > edgePointCap {
		step := len(pts) / edgePointCap
		out := make([][3]int, 0, edgePointCap)
		for i := 0; i < len(pts); i += step {
			out = append(out, pts[i])
		}
		pts = out
	}
	return pts
}

// orientedEdgeDT computes, per orientation, the screen's edges of that orientation and
// their distance transform — so a template edge is matched only against screen edges that
// run the SAME way. Computed once and shared across template scales.
func orientedEdgeDT(scr *image.RGBA) (dts [orientBins][]int, sw, sh int) {
	b := scr.Bounds()
	sw, sh = b.Dx(), b.Dy()
	var maps [orientBins][]bool
	for i := range maps {
		maps[i] = make([]bool, sw*sh)
	}
	for y := 0; y < sh-1; y++ {
		for x := 0; x < sw-1; x++ {
			gx, gy := sobelGrad(scr, x, y)
			if iabs(gx)+iabs(gy) > EdgeTolerance {
				maps[orientationBin(gx, gy)][y*sw+x] = true
			}
		}
	}
	for i := range maps {
		dts[i] = distanceTransform(maps[i], sw, sh, edgeSlack)
	}
	return dts, sw, sh
}

// matchOutline slides the template's ORIENTED edge points over the per-orientation screen
// distance transforms and scores each position by how close those points are to a screen
// edge OF THE SAME ORIENTATION (1.0 = dead on, 0 = ≥ edgeSlack away, or no same-way edge
// near). Requiring matching orientation rejects the accidental edge alignments a busy UI
// throws up. Best position is the match; Ambiguous when a distinct second scores as well.
func matchOutline(dts [orientBins][]int, sw, sh int, tmpl *image.RGBA) Match {
	tb := tmpl.Bounds()
	tw, th := tb.Dx(), tb.Dy()
	if tw == 0 || th == 0 || tw > sw || th > sh {
		return Match{}
	}
	tpts := orientedEdgePoints(tmpl)
	if len(tpts) < minEdgePoints {
		return Match{} // not enough outline to match on
	}
	sample := [][3]int{tpts[0], tpts[len(tpts)/4], tpts[len(tpts)/2], tpts[3*len(tpts)/4], tpts[len(tpts)-1]}
	maxSum := len(tpts) * edgeSlack
	distinct := max(th, tw)
	bestScore, rivalScore := -1.0, -1.0
	best := Match{}
	for oy := 0; oy <= sh-th; oy++ {
		for ox := 0; ox <= sw-tw; ox++ {
			ps := 0
			for _, p := range sample {
				ps += edgeSlack - dts[p[2]][(oy+p[1])*sw+ox+p[0]]
			}
			if ps <= 0 {
				continue // every sample far from a same-orientation edge — skip
			}
			sum := 0
			for _, p := range tpts {
				sum += edgeSlack - dts[p[2]][(oy+p[1])*sw+ox+p[0]]
			}
			score := float64(sum) / float64(maxSum)
			cx, cy := ox+tw/2, oy+th/2
			if score > bestScore {
				if best.Found && far(best.X, best.Y, cx, cy, distinct) && bestScore > rivalScore {
					rivalScore = bestScore
				}
				bestScore = score
				best = Match{X: cx, Y: cy, Found: true}
			} else if best.Found && far(best.X, best.Y, cx, cy, distinct) && score > rivalScore {
				rivalScore = score
			}
		}
	}
	if !best.Found {
		return Match{}
	}
	best.Score = bestScore
	best.Found = bestScore >= edgeMatchThreshold
	best.Ambiguous = rivalScore >= bestScore-ambiguityDelta && rivalScore >= edgeMatchThreshold
	return best
}

// findEdgeDT matches a single-scale template's oriented outline against the screen.
func findEdgeDT(scr, tmpl *image.RGBA) Match {
	dts, sw, sh := orientedEdgeDT(scr)
	return matchOutline(dts, sw, sh, tmpl)
}

// findEdgeMultiScale matches the template's oriented outline at several scales (the per-
// orientation DTs are computed once and shared), keeping the best — so a DPI/resolution
// change that renders the button a different size than the recorded template still resolves.
func findEdgeMultiScale(scr, tmpl *image.RGBA) Match {
	dts, sw, sh := orientedEdgeDT(scr)
	best := Match{}
	for _, s := range findScales() {
		if m := matchOutline(dts, sw, sh, resize(tmpl, s)); m.Found && m.Score > best.Score {
			best = m
		}
	}
	return best
}

// matchMultiScale matches the pixel template at several scales, keeping the best — the
// pixel counterpart of findEdgeMultiScale (robust to a DPI/resolution change).
func matchMultiScale(scr, tmpl *image.RGBA, tol int) Match {
	sb := scr.Bounds()
	best := Match{}
	for _, s := range findScales() {
		t := resize(tmpl, s)
		if t.Bounds().Dx() > sb.Dx() || t.Bounds().Dy() > sb.Dy() {
			continue
		}
		if m := match(scr, t, tol); m.Found && m.Score > best.Score {
			best = m
		}
	}
	return best
}

// buttonDilate is how far apart (px) edges may be and still count as the same element, so
// a button's border, text and icon group into one component rather than fragmenting.
const buttonDilate = 4

// minButton is the smallest side (px) a detected component must have to be a button.
const minButton = 12

// buttonBandTol is the per-channel colour jump (0..255) between adjacent rows/columns that
// marks a BOUNDARY between stacked/adjacent controls — the gap a menu puts between buttons.
// Below it the band is "still the same button" (incl. a smooth gradient fill); a jump above
// it is the gap. $MARCO_BUTTON_BAND_TOL overrides.
func buttonBandTol() int {
	if v := os.Getenv("MARCO_BUTTON_BAND_TOL"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return n
		}
	}
	return 26
}

// tightenToButton shrinks r to the SINGLE control under (cx,cy). DetectButtons fuses
// stacked menu buttons into one panel (its edge dilation bridges the gaps between them);
// this undoes that by GROWING a band out from the click and stopping at the gap that
// separates one button from the next. It grows by LOCAL continuity (each new row/column vs
// the previous one), so a smooth gradient fill keeps growing while a sharp gap — the
// boundary — stops it. Falls back to r if the band collapses (a flat/uniform region with no
// gap, e.g. the click wasn't on a bounded control). Pure; coords in img's space.
func tightenToButton(img *image.RGBA, r image.Rectangle, cx, cy int) image.Rectangle {
	r = r.Intersect(img.Bounds())
	if r.Dx() < minButton || r.Dy() < minButton {
		return r
	}
	tol := buttonBandTol()
	cx = clampI(cx, r.Min.X, r.Max.X-1)
	cy = clampI(cy, r.Min.Y, r.Max.Y-1)
	// Vertical band: grow up/down while each row stays close to the previous (gradient OK),
	// stop at the gap row that jumps.
	top, bot := cy, cy
	for top > r.Min.Y && colorClose(rowMean(img, r.Min.X, r.Max.X, top-1), rowMean(img, r.Min.X, r.Max.X, top), tol) {
		top--
	}
	for bot < r.Max.Y-1 && colorClose(rowMean(img, r.Min.X, r.Max.X, bot+1), rowMean(img, r.Min.X, r.Max.X, bot), tol) {
		bot++
	}
	// Horizontal band within [top,bot]: same, for the button's left/right edges.
	left, right := cx, cx
	for left > r.Min.X && colorClose(colMean(img, top, bot+1, left-1), colMean(img, top, bot+1, left), tol) {
		left--
	}
	for right < r.Max.X-1 && colorClose(colMean(img, top, bot+1, right+1), colMean(img, top, bot+1, right), tol) {
		right++
	}
	t := image.Rect(left, top, right+1, bot+1)
	if t.Dx() < minButton || t.Dy() < minButton {
		return r
	}
	return t
}

// buttonFillTol is the per-channel row/col mean jump tolerated when GROWING a crop OUTWARD
// to a button's full fill. Looser than buttonBandTol (which SHRINKS, to isolate stacked
// buttons across their thin gaps): outward growth has to cross the one text row inside the
// control — where the glyphs pull the row mean a little off the pure fill — while still
// STOPPING at the button's border or the gap to the next control (a far bigger jump). Over a
// full row/column width the text is sparse, so the mean stays fill-ish; this only has to
// clear that residual. $MARCO_BUTTON_FILL_TOL overrides.
func buttonFillTol() int {
	if v := os.Getenv("MARCO_BUTTON_FILL_TOL"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return n
		}
	}
	return 48
}

// growButtonBand expands r OUTWARD to the homogeneous (possibly gradient) fill of the control
// the click sits in — so a crop DetectButtons bounded to just the text/icon (a few glyphs at
// one end of a wide button, or a sliver by the button's rounded edge) becomes the WHOLE
// button. It grows each side while the next row/column still reads as the button's FILL
// (its median colour, robust to the text glyphs inside) and STOPS at the button's border /
// the gap to the next control. Crucially, a side that runs the full maxButtonDim (or the
// frame) WITHOUT hitting a border found no real edge there, so it REVERTS to r — growth only
// ever produces a crop bounded by genuine borders, which keeps a click on a large flat panel
// or a borderless gradient field from running away (no regression: a clean bordered button
// stops at its border on the first step out and is returned unchanged). Comparing each outer
// row/column to the FIXED fill reference (not to the adjacent row) is what lets it cross the
// dense text row at the box edge — that comparison never happens, since growth steps only
// OUTSIDE the box. Returns a rect ⊇ r, clamped to img. Pure; coords in img's space.
func growButtonBand(img *image.RGBA, r image.Rectangle, cx, cy int) image.Rectangle {
	b := img.Bounds()
	r = r.Intersect(b)
	if r.Dx() < minButton || r.Dy() < minButton {
		return r
	}
	tol := buttonFillTol()
	fill := fillColor(img, r) // the button body — the median is the majority fill, not the text
	// Vertical: grow up/down while each outer row still reads as fill; the button's top/bottom
	// border (a row no longer fill) stops it. Reaching the cap = no border found → revert.
	loY := clampI(cy-maxButtonDim, b.Min.Y, r.Min.Y)
	hiY := clampI(cy+maxButtonDim, r.Max.Y-1, b.Max.Y-1)
	top := r.Min.Y
	for top > loY && colorClose(rowMean(img, r.Min.X, r.Max.X, top-1), fill, tol) {
		top--
	}
	if top == loY {
		top = r.Min.Y // hit the cap/frame, not a border — don't trust this side
	}
	bot := r.Max.Y - 1
	for bot < hiY && colorClose(rowMean(img, r.Min.X, r.Max.X, bot+1), fill, tol) {
		bot++
	}
	if bot == hiY {
		bot = r.Max.Y - 1
	}
	// Horizontal over the grown height: each outer column still reads as fill until the
	// button's left/right border.
	loX := clampI(cx-maxButtonDim, b.Min.X, r.Min.X)
	hiX := clampI(cx+maxButtonDim, r.Max.X-1, b.Max.X-1)
	left := r.Min.X
	for left > loX && colorClose(colMean(img, top, bot+1, left-1), fill, tol) {
		left--
	}
	if left == loX {
		left = r.Min.X
	}
	right := r.Max.X - 1
	for right < hiX && colorClose(colMean(img, top, bot+1, right+1), fill, tol) {
		right++
	}
	if right == hiX {
		right = r.Max.X - 1
	}
	return image.Rect(left, top, right+1, bot+1).Intersect(b)
}

// fillColor estimates the dominant fill of r as its per-channel median — robust to the text
// glyphs inside it (strokes are the minority; the fill is the majority), so growth keys off
// the button body rather than a letter under the click. Samples on a grid to stay cheap on a
// large box. When the median lands on the text instead (a very bold label), growth simply
// finds no fill outside and reverts — a safe no-op, never a wrong crop.
func fillColor(img *image.RGBA, r image.Rectangle) [3]int {
	r = r.Intersect(img.Bounds())
	w, h := r.Dx(), r.Dy()
	if w <= 0 || h <= 0 {
		return [3]int{}
	}
	stepX, stepY := 1, 1
	for (w/stepX)*(h/stepY) > 4096 {
		if w/stepX >= h/stepY {
			stepX++
		} else {
			stepY++
		}
	}
	var rs, gs, bs []int
	for y := r.Min.Y; y < r.Max.Y; y += stepY {
		for x := r.Min.X; x < r.Max.X; x += stepX {
			c := img.RGBAAt(x, y)
			rs = append(rs, int(c.R))
			gs = append(gs, int(c.G))
			bs = append(bs, int(c.B))
		}
	}
	return [3]int{medianInt(rs), medianInt(gs), medianInt(bs)}
}

// medianInt returns the median of v (mutates order). Empty → 0.
func medianInt(v []int) int {
	if len(v) == 0 {
		return 0
	}
	sort.Ints(v)
	return v[len(v)/2]
}

// rowMean / colMean are the mean RGB of one row span / column span.
func rowMean(img *image.RGBA, x0, x1, y int) [3]int {
	if x1 <= x0 {
		return [3]int{}
	}
	var r, g, b int
	for x := x0; x < x1; x++ {
		c := img.RGBAAt(x, y)
		r, g, b = r+int(c.R), g+int(c.G), b+int(c.B)
	}
	n := x1 - x0
	return [3]int{r / n, g / n, b / n}
}

func colMean(img *image.RGBA, y0, y1, x int) [3]int {
	if y1 <= y0 {
		return [3]int{}
	}
	var r, g, b int
	for y := y0; y < y1; y++ {
		c := img.RGBAAt(x, y)
		r, g, b = r+int(c.R), g+int(c.G), b+int(c.B)
	}
	n := y1 - y0
	return [3]int{r / n, g / n, b / n}
}

func colorClose(a, b [3]int, tol int) bool {
	return absI(a[0]-b[0]) <= tol && absI(a[1]-b[1]) <= tol && absI(a[2]-b[2]) <= tol
}

// DetectButtons segments img into button-like rectangles: connected structures of edges
// (a control's border + text + icon, grouped across small gaps), each big enough to be a
// button. The first step of "recognise the buttons" — crop an anchor to a control, or
// (later) snap OCR text to its button. Heuristic and tunable; boxes are in img's space.
func DetectButtons(img *image.RGBA) []image.Rectangle {
	if img == nil {
		return nil
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < minButton || h < minButton {
		return nil
	}
	edge := edgeBinaryMap(img)
	dil := make([]bool, w*h)
	for y := range h {
		for x := range w {
			if !edge[y*w+x] {
				continue
			}
			for dy := -buttonDilate; dy <= buttonDilate; dy++ {
				for dx := -buttonDilate; dx <= buttonDilate; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < w && ny >= 0 && ny < h {
						dil[ny*w+nx] = true
					}
				}
			}
		}
	}
	seen := make([]bool, w*h)
	var boxes []image.Rectangle
	stack := make([][2]int, 0, 64)
	for y := range h {
		for x := range w {
			if !dil[y*w+x] || seen[y*w+x] {
				continue
			}
			minX, minY, maxX, maxY := w, h, -1, -1
			stack = stack[:0]
			stack = append(stack, [2]int{x, y})
			seen[y*w+x] = true
			for len(stack) > 0 {
				p := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				px, py := p[0], p[1]
				if edge[py*w+px] { // bound by real edges, not the dilation halo
					if px < minX {
						minX = px
					}
					if px > maxX {
						maxX = px
					}
					if py < minY {
						minY = py
					}
					if py > maxY {
						maxY = py
					}
				}
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := px+d[0], py+d[1]
					if nx >= 0 && nx < w && ny >= 0 && ny < h && dil[ny*w+nx] && !seen[ny*w+nx] {
						seen[ny*w+nx] = true
						stack = append(stack, [2]int{nx, ny})
					}
				}
			}
			if maxX-minX+1 >= minButton && maxY-minY+1 >= minButton {
				boxes = append(boxes, image.Rect(b.Min.X+minX, b.Min.Y+minY, b.Min.X+maxX+1, b.Min.Y+maxY+1))
			}
		}
	}
	return boxes
}

// SnapToButton finds the button containing (cx,cy) and returns its CENTRE, so a hit on a
// word or icon snaps to the whole control (e.g. OCR locates the text "Mute", this returns
// the centre of the Mute button). It searches a window around the point, so it's cheap on
// a full screenshot. ok=false when no button is detected there — the caller keeps its
// original point. cx,cy and the result are in img's coordinate space.
func SnapToButton(img *image.RGBA, cx, cy int) (int, int, bool) {
	if img == nil {
		return cx, cy, false
	}
	const win = 220
	b := img.Bounds()
	x0, y0 := clampI(cx-win, b.Min.X, b.Max.X), clampI(cy-win, b.Min.Y, b.Max.Y)
	x1, y1 := clampI(cx+win, b.Min.X, b.Max.X), clampI(cy+win, b.Min.Y, b.Max.Y)
	if x1-x0 < minButton || y1-y0 < minButton {
		return cx, cy, false
	}
	sub := img.SubImage(image.Rect(x0, y0, x1, y1)).(*image.RGBA)
	r, ok := buttonAt(sub, cx, cy)
	if !ok {
		return cx, cy, false
	}
	return (r.Min.X + r.Max.X) / 2, (r.Min.Y + r.Max.Y) / 2, true
}

// buttonNear finds the button containing (cx,cy) by detecting buttons within a bounded
// window (half-size win) around the point — cheap even on a whole-desktop frame, where a
// full DetectButtons pass would be far too slow. Rejects a component that fills most of
// the window (a panel/background, not the clicked control) so the caller can fall back to
// a click-centred patch. Coords are in img's space.
func buttonNear(img *image.RGBA, cx, cy, win int) (image.Rectangle, bool) {
	b := img.Bounds()
	x0, y0 := clampI(cx-win, b.Min.X, b.Max.X), clampI(cy-win, b.Min.Y, b.Max.Y)
	x1, y1 := clampI(cx+win, b.Min.X, b.Max.X), clampI(cy+win, b.Min.Y, b.Max.Y)
	if x1-x0 < minButton || y1-y0 < minButton {
		return image.Rectangle{}, false
	}
	sub := img.SubImage(image.Rect(x0, y0, x1, y1)).(*image.RGBA)
	r, ok := buttonAt(sub, cx, cy)
	if !ok {
		return image.Rectangle{}, false
	}
	// A component spanning most of the window in BOTH dimensions is a panel/background,
	// not a control — don't crop the template to it. (A wide-but-short banner passes.)
	if r.Dx() > (x1-x0)*9/10 && r.Dy() > (y1-y0)*9/10 {
		return image.Rectangle{}, false
	}
	return r, true
}

// buttonAt returns the detected button containing (cx,cy) — the SMALLEST such box, so a
// click inside nested structure snaps to the tightest control. ok=false if none.
func buttonAt(img *image.RGBA, cx, cy int) (image.Rectangle, bool) {
	var best image.Rectangle
	found := false
	for _, r := range DetectButtons(img) {
		if image.Pt(cx, cy).In(r) {
			if !found || area(r) < area(best) {
				best, found = r, true
			}
		}
	}
	return best, found
}

func area(r image.Rectangle) int { return r.Dx() * r.Dy() }
