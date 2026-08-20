// Package screen is the platform "read the screen" capability: pixel reads and
// template image search (find-an-image-on-screen, like AHK's ImageSearch). The
// matcher is pure Go and unit-testable; capture is per-OS (screen_windows.go),
// so this is a first-class native automation capability, Windows first with
// macOS/Linux additive behind the same interface.
package screen

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	_ "image/jpeg" // decode jpeg templates
	"image/png"
	"math"
	"os"
	"strconv"
	"strings"
)

// ErrUnsupported is returned by New on platforms without a capture backend yet.
var ErrUnsupported = errors.New("screen: not supported on this platform")

// Region is a screen rectangle; the zero value means the whole virtual screen.
type Region struct{ X1, Y1, X2, Y2 int }

// Empty reports whether the region is unset (full screen).
func (r Region) Empty() bool { return r == Region{} }

// Match is the result of a Find: the center of the located image (absolute
// screen coords), whether it was found, and Score — the fraction of the template's
// (opaque) pixels that matched at that location, 0..1. Score lets a caller fuse the
// image signal with others (colour, position) into a confidence rather than treating
// the match as a yes/no.
type Match struct {
	X, Y  int
	Found bool
	Score float64
	// Ambiguous is set when a DISTINCT second location matched about as well — the
	// template appears in more than one place, so X,Y can't be trusted as THE target.
	Ambiguous bool
	// ColorScore is the colour-histogram similarity (0..1) between the template and the
	// matched screen window — a palette confirmation complementary to the spatial Score.
	ColorScore float64
}

// Screen reads the display.
type Screen interface {
	// Pixel returns the RGB color (0xRRGGBB) at an absolute screen coordinate.
	Pixel(x, y int) (uint32, error)
	// Find searches the region (or the whole screen) for the template image,
	// allowing per-channel color difference up to tolerance. Returns the center
	// of the first match.
	Find(templatePath string, region Region, tolerance int) (Match, error)
	// FindEdge searches the region for the template's OUTLINE (its edge pattern) rather
	// than its raw pixels, so it still locates a button after a recolour / theme change.
	// Returns the match centre and an edge-similarity Score, like Find.
	FindEdge(templatePath string, region Region, tolerance int) (Match, error)
}

// LoadTemplate decodes a PNG/JPEG template into an *image.RGBA.
func LoadTemplate(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return toRGBA(img), nil
}

// ColorAt returns the 0xRRGGBB color of the pixel at (x,y) in img's own coordinate
// space (honoring its Rect origin), ok=false if out of bounds. Used to record the
// colour under a click so an anchor can confirm a screen by a single pixel — a cheap
// fallback resolver when template matching is marginal.
func ColorAt(img *image.RGBA, x, y int) (uint32, bool) {
	if img == nil {
		return 0, false
	}
	b := img.Bounds()
	if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
		return 0, false
	}
	i := img.PixOffset(x, y)
	return uint32(img.Pix[i])<<16 | uint32(img.Pix[i+1])<<8 | uint32(img.Pix[i+2]), true
}

// EncodePNG encodes an image as PNG bytes — used to persist a captured click
// target as a route template. OS-agnostic.
func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EdgeTolerance is the gradient above which a pixel counts as an EDGE — part of a
// button's border, text or icon rather than flat fill or background. Button recognition
// (AutoCrop / DetectButtons) and edge matching key off these. Tunable via
// $MARCO_EDGE_TOLERANCE: lower finds fainter edges (more sensitive), higher ignores
// soft gradients (cleaner outlines).
var EdgeTolerance = edgeToleranceFromEnv()

// cvSensitivity / cvLerp mirror oshost's CV find dial so the matcher's DEFAULT gates track
// the same $MARCO_CV_SENSITIVITY slider (0 = strict … 1 = loose, default 0.5). Kept
// package-local because screen is imported BY oshost, not the reverse; the canonical
// description lives on oshost.cvSensitivity. Each knob's own env var still overrides the
// derived default, and 0.5 reproduces the legacy fixed value exactly.
func cvSensitivity() float64 {
	if v := strings.TrimSpace(os.Getenv("MARCO_CV_SENSITIVITY")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			return f
		}
	}
	return 0.5
}

func cvLerp(strict, loose float64) float64 { return strict + (loose-strict)*cvSensitivity() }

// cropScale grows the capture-time crop-size gates as the CV dial is loosened past neutral,
// so a loosened dial lets AutoCrop capture BIG buttons whole (game UIs — a control can be
// enormous) instead of rejecting them into the small click-centred fallback patch. 1× at/
// below 0.5 (legacy — desktop menus where an over-grab would re-centre onto the wrong row),
// up to 4× at full-loose. Capture-time, so it takes effect on the next LEARN.
func cropScale() float64 { return 1.0 + max(0.0, cvSensitivity()-0.5)*6.0 }

func edgeToleranceFromEnv() int {
	if v := strings.TrimSpace(os.Getenv("MARCO_EDGE_TOLERANCE")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	// Sobel-magnitude scale (~4× a raw step). s=0.5 → 100 (legacy). Lower = fainter edges
	// count, so a subtle button border is detected (looser crop) and edge-matched (looser find).
	return int(cvLerp(150, 50))
}

// edgeStrength is the largest per-channel difference between a pixel and its right/below
// neighbour — a cheap gradient magnitude. High on borders/text/icons, ~0 on flat areas.
// sobelX/sobelY are the horizontal/vertical 3×3 Sobel kernels.
var (
	sobelX = [3][3]int{{-1, 0, 1}, {-2, 0, 2}, {-1, 0, 1}}
	sobelY = [3][3]int{{-1, -2, -1}, {0, 0, 0}, {1, 2, 1}}
)

// sobelGrad returns the horizontal/vertical Sobel gradient components on luminance at
// (x,y). Their magnitude is the edge strength; their direction (atan2) is the edge
// ORIENTATION used by oriented-edge matching. Border pixels clamp to the edge.
func sobelGrad(img *image.RGBA, x, y int) (gx, gy int) {
	for j := -1; j <= 1; j++ {
		for i := -1; i <= 1; i++ {
			l := lumAt(img, x+i, y+j)
			gx += sobelX[j+1][i+1] * l
			gy += sobelY[j+1][i+1] * l
		}
	}
	return gx, gy
}

// edgeStrength is the Sobel gradient magnitude (L1) on luminance at (x,y): a symmetric
// 3×3 edge measure that picks up faint and anti-aliased borders a single-neighbour
// difference misses. Magnitude is ~4× a raw step, so EdgeTolerance is scaled to match.
func edgeStrength(img *image.RGBA, x, y int) int {
	gx, gy := sobelGrad(img, x, y)
	return iabs(gx) + iabs(gy)
}

// lumAt returns the luminance (0..255) at (x,y), clamped to the image bounds.
func lumAt(img *image.RGBA, x, y int) int {
	b := img.Bounds()
	if x < 0 {
		x = 0
	} else if x >= b.Dx() {
		x = b.Dx() - 1
	}
	if y < 0 {
		y = 0
	} else if y >= b.Dy() {
		y = b.Dy() - 1
	}
	r, g, bl, _ := at(img, x, y)
	return (r*299 + g*587 + bl*114) / 1000
}

// AutoCrop recognises the button in a captured patch (centred on the click) and crops to
// it. It first tries to snap to the DETECTED button containing the click — which isolates
// the control AND drops adjacent disconnected things like a tooltip. Failing that it falls
// back to the bounding box of ALL edge content. Returns img unchanged when the content
// already fills the patch (a dense UI) or there's too little edge structure to crop to.
// Pure and OS-agnostic — the capture-time half of button recognition.
func AutoCrop(img *image.RGBA) *image.RGBA {
	if img == nil {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 16 || h < 16 {
		return img
	}
	// The patch is centred on the click, so its centre is the clicked button.
	if r, ok := buttonAt(img, b.Min.X+w/2, b.Min.Y+h/2); ok {
		return cropTo(img, r, 6)
	}
	// Fallback: tightest box around all edge content.
	minX, minY, maxX, maxY := w, h, -1, -1
	for y := 0; y < h-1; y++ {
		for x := 0; x < w-1; x++ {
			if edgeStrength(img, x, y) > EdgeTolerance {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < minX || maxY < minY {
		return img // no edges — flat patch
	}
	return cropTo(img, image.Rect(b.Min.X+minX, b.Min.Y+minY, b.Min.X+maxX+1, b.Min.Y+maxY+1), 6)
}

// autoCropMargin is the padding kept around a detected button when cropping its template.
const autoCropMargin = 6

// buttonSearchWin (px) is how far from the click AutoCropAt looks for the button's edges
// — generous so a WIDE control (a menu banner) still fits in the window and groups into a
// single component, the thing a fixed 128px patch couldn't contain. Dial-scaled (cropScale):
// a loosened dial searches farther so a HUGE game button's far edges still fall in the window.
func buttonSearchWin() int { return int(480 * cropScale()) }

// AutoCropAt crops img to the button CONTAINING the click (cx,cy, in img's coordinate
// space) and returns the cropped template plus the button's CENTRE (also in img space).
// The recorder feeds it a whole-desktop frame and re-centres the anchor on the returned
// centre, so the template's centre becomes the click target the matcher resolves — and a
// button BIGGER than the old fixed patch is captured whole instead of as a fragment. When
// no clean button is detected there, it falls back to a click-centred patch of half-size
// fallbackR (centre = the click), preserving the old behaviour for flat/freeform spots.
// Pure and OS-agnostic.
// clickX, clickY (the last two returns) are the click's position inside the returned
// template (its own 0-origin space) — the ORIGINAL click, even when the anchor is
// re-centred on a wider button. Learn-time OCR uses it to read the button under the
// click, not every word in the crop.
func AutoCropAt(img *image.RGBA, cx, cy, fallbackR int) (out *image.RGBA, centerX, centerY, clickX, clickY int) {
	if img == nil {
		return img, cx, cy, 0, 0
	}
	b := img.Bounds()
	// Candidate region + crop margin: a detected button (tight margin) or a click-centred
	// fallback patch (no margin) when none is found there.
	margin := autoCropMargin
	fallbackR = int(float64(fallbackR) * cropScale()) // dial-scaled: a bigger fallback patch for a big undetected button
	r, ok := buttonNear(img, cx, cy, buttonSearchWin())
	if ok && buttonCropSane(r, cx, cy) {
		// DetectButtons bounds the box to the edge structure it found — often just the
		// text/icon, a fragment of a wide button. Grow it out to the control's full fill so
		// the template is the WHOLE button, not a sliver (re-centring then lands the click on
		// the button's middle). A clean bordered button is returned unchanged.
		r = growButtonBand(img, r, cx, cy)
	} else {
		r = image.Rect(cx-fallbackR, cy-fallbackR, cx+fallbackR+1, cy+fallbackR+1)
		margin = 0
	}
	// Isolate the SINGLE control under the click: connected-components fuses stacked menu
	// buttons into one panel, so grow a colour band out from the click and stop at the gap
	// between buttons. A no-op when the region is already one button (or a flat patch).
	r = tightenToButton(img, r, cx, cy)
	x0 := clampI(r.Min.X-margin, b.Min.X, b.Max.X)
	y0 := clampI(r.Min.Y-margin, b.Min.Y, b.Max.Y)
	return subCrop(img, r, margin), (r.Min.X + r.Max.X) / 2, (r.Min.Y + r.Max.Y) / 2, cx - x0, cy - y0
}

// maxButtonDim / maxRecenterPx bound what counts as a real button to crop AND re-centre on.
// Connected-edge detection can over-grab — a whole menu's rows dilate into one tall
// component — and re-centring on THAT moves the click to the panel's middle (a different
// menu item). So a detected box is only trusted when it's button-sized AND the click sits
// near its centre (you clicked the control, not one row of a merged panel). Otherwise the
// caller keeps the click as the template centre (a click-centred crop), which is always
// safe — the template still matches and the click lands where you actually clicked.
// Dial-scaled (cropScale): a loosened dial trusts a MUCH bigger detected box and a click
// farther from its centre — game buttons are huge, so "button-sized" has to stretch.
func maxButtonDim() int  { return int(600 * cropScale()) }
func maxRecenterPx() int { return int(140 * cropScale()) }

// buttonCropSane reports whether r is a plausible single control to re-centre on: not panel-
// sized, and centred near the click.
func buttonCropSane(r image.Rectangle, cx, cy int) bool {
	if r.Dx() > maxButtonDim() || r.Dy() > maxButtonDim() {
		return false
	}
	ccx, ccy := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
	return absI(cx-ccx) <= maxRecenterPx() && absI(cy-ccy) <= maxRecenterPx()
}

func absI(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// subCrop copies img's pixels in r (grown by margin, clamped to img) into a fresh
// 0-origin RGBA. Unlike cropTo it always crops to the rect — the caller already chose it,
// so there's no fill/min-size heuristic to second-guess (cropping a button out of a whole
// desktop frame would otherwise trip cropTo's "fills the patch" guard).
func subCrop(img *image.RGBA, r image.Rectangle, margin int) *image.RGBA {
	b := img.Bounds()
	x0 := clampI(r.Min.X-margin, b.Min.X, b.Max.X)
	y0 := clampI(r.Min.Y-margin, b.Min.Y, b.Max.Y)
	x1 := clampI(r.Max.X+margin, b.Min.X, b.Max.X)
	y1 := clampI(r.Max.Y+margin, b.Min.Y, b.Max.Y)
	if x1-x0 < 1 || y1-y0 < 1 {
		return img
	}
	out := image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	draw.Draw(out, out.Bounds(), img, image.Pt(x0, y0), draw.Src)
	return out
}

// cropTo returns img cropped to r grown by margin and clamped to img, or img unchanged
// when the result would barely shrink or be too small.
func cropTo(img *image.RGBA, r image.Rectangle, margin int) *image.RGBA {
	b := img.Bounds()
	x0 := clampI(r.Min.X-margin, b.Min.X, b.Max.X)
	y0 := clampI(r.Min.Y-margin, b.Min.Y, b.Max.Y)
	x1 := clampI(r.Max.X+margin, b.Min.X, b.Max.X)
	y1 := clampI(r.Max.Y+margin, b.Min.Y, b.Max.Y)
	cw, ch := x1-x0, y1-y0
	if cw < 12 || ch < 12 || (cw >= b.Dx()-2 && ch >= b.Dy()-2) {
		return img
	}
	out := image.NewRGBA(image.Rect(0, 0, cw, ch))
	draw.Draw(out, out.Bounds(), img, image.Pt(x0, y0), draw.Src)
	return out
}

func clampI(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// DistinctiveTolerance is the per-channel color difference under which two
// pixels count as "the same" when judging whether a patch is distinctive.
const DistinctiveTolerance = 12

// Distinctive reports whether a captured patch has enough visual variety to be
// worth matching by image — i.e. it isn't a near-uniform area (blank canvas,
// flat background) that would match almost anywhere on screen. A click on such
// an area is better left as a fixed coordinate. The test: at least 15% of pixels
// differ from the top-left pixel by more than DistinctiveTolerance on a channel.
func Distinctive(img *image.RGBA) bool {
	if img == nil {
		return false
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	total := w * h
	if total == 0 {
		return false
	}
	r0, g0, b0, _ := at(img, 0, 0)
	differing := 0
	for y := range h {
		for x := range w {
			r, g, b, _ := at(img, x, y)
			if !within(r, g, b, r0, g0, b0, DistinctiveTolerance) {
				differing++
			}
		}
	}
	return differing*100 >= total*15
}

func toRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

// MatchThreshold is the fraction of a template's pixels that must be within
// tolerance for a location to count as a match. Below 1.0 on purpose: requiring
// EVERY pixel match is far too brittle — a mouse cursor, tooltip, focus glow, a
// dynamic background behind a menu, or one anti-aliased edge makes an exact match
// fail. 0.75 tolerates a quarter of the patch differing while staying specific to
// the target. Tune with $MARCO_FIND_THRESHOLD (0–1).
var MatchThreshold = matchThresholdFromEnv()

func matchThresholdFromEnv() float64 {
	if v := strings.TrimSpace(os.Getenv("MARCO_FIND_THRESHOLD")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f <= 1 {
			return f
		}
	}
	return cvLerp(1.00, 0.50) // s=0.5 → 0.75 (legacy). Lower = a present-but-imperfect button still registers as found.
}

// maskAlpha is the alpha below which a template pixel is treated as MASKED — not
// compared at all. Crop a template to the text/lines and make the rest transparent,
// and the match ignores the background entirely (so it can shift freely). Captured
// templates are fully opaque, so this is a no-op until you (or a vision step) add
// transparency.
const maskAlpha = 128

// ambiguityDelta: a DISTINCT second location whose score is within this of the best
// makes the match AMBIGUOUS — the template appears more than once (the same button in
// several menus), so its location can't be trusted and the caller should fall back to
// the recorded point.
const ambiguityDelta = 0.08

// match scans scr for tmpl, allowing per-channel difference up to tol, and returns the
// center of the BEST-scoring location whose matching-pixel fraction is at least
// MatchThreshold, plus Ambiguous when a DISTINCT location matches about as well (the
// template occurs in multiple places). Transparent template pixels are ignored (a mask),
// so only the opaque foreground has to match. Pure and OS-agnostic — the core of Find.
func match(scr, tmpl *image.RGBA, tol int) Match {
	sw, sh := scr.Rect.Dx(), scr.Rect.Dy()
	tw, th := tmpl.Rect.Dx(), tmpl.Rect.Dy()
	if tw == 0 || th == 0 || tw > sw || th > sh {
		return Match{}
	}
	total := tw * th
	opaque := countOpaque(tmpl)
	// Treat alpha as a MASK only when the template MIXES transparent and opaque
	// pixels (a deliberate mask the user/vision added). A normal screen capture
	// carries no alpha — Windows BitBlt leaves A=0, so every pixel reads transparent
	// — and a flat PNG is all opaque; in both cases compare every pixel's RGB.
	masked := opaque > 0 && opaque < total
	denom := total
	if masked {
		denom = opaque
	}
	maxMiss := denom - int(float64(denom)*MatchThreshold) // mismatches allowed
	// Two centres at least a template-size apart are different targets, not adjacent
	// passing positions over the same one.
	distinct := max(th, tw)
	// Lighting / contrast invariance (normalised cross-correlation): when the template's
	// COMPARED pixels have real internal contrast, model the screen window as a linear
	// transform of the template — predicted = screenMean + gain·(template − templateMean),
	// where gain is the ratio of standard deviations. Subtracting the MEAN cancels a
	// uniform brightness / white-balance / theme shift (night mode, dimming, a different
	// monitor, f.lux); the GAIN cancels a contrast / gamma change. Only real CONTENT
	// differences remain. A near-uniform template skips this (normalising it would let any
	// flat region match) and stays exact-tolerance.
	normalize := opaqueVaried(tmpl, masked)
	var meanT [3]int
	var stdT [3]float64
	if normalize {
		meanT, stdT = meanStdOpaque(tmpl, tmpl, 0, 0, masked)
	}
	bestMiss, rivalMiss := maxMiss+1, maxMiss+1
	best := Match{}
	bestX0, bestY0 := 0, 0
	haveRival := false
	for y := 0; y <= sh-th; y++ {
		for x := 0; x <= sw-tw; x++ {
			// Cheap pre-filter: 5 spread sample points must mostly agree, so most
			// positions are rejected without the full per-pixel scan.
			if !preFilter(scr, tmpl, x, y, tol, masked, normalize) {
				continue
			}
			var meanS [3]int
			gain := [3]float64{1, 1, 1}
			if normalize {
				var stdS [3]float64
				meanS, stdS = meanStdOpaque(scr, tmpl, x, y, masked)
				for c := range gain {
					if stdT[c] > 1 { // skip contrast scaling on a near-flat channel
						gain[c] = clampF(stdS[c]/stdT[c], gainLo, gainHi)
					}
				}
			}
			miss := countMismatch(scr, tmpl, x, y, tol, maxMiss, masked, meanT, meanS, gain)
			if miss > maxMiss {
				continue // not a qualifying match
			}
			cx, cy := x+tw/2, y+th/2
			switch {
			case miss < bestMiss:
				// The outgoing best, if far from the new one, becomes a rival.
				if best.Found && far(best.X, best.Y, cx, cy, distinct) && bestMiss < rivalMiss {
					rivalMiss, haveRival = bestMiss, true
				}
				bestMiss = miss
				best = Match{X: cx, Y: cy, Found: true}
				bestX0, bestY0 = x, y
			case best.Found && far(best.X, best.Y, cx, cy, distinct) && miss < rivalMiss:
				rivalMiss, haveRival = miss, true
			}
		}
	}
	if best.Found {
		best.Score = 1 - float64(bestMiss)/float64(denom)
		rivalScore := 1 - float64(rivalMiss)/float64(denom)
		best.Ambiguous = haveRival && rivalScore >= best.Score-ambiguityDelta
		// Palette confirmation: how well the matched window's colour histogram matches the
		// template's — robust to a small warp/AA the spatial score is sensitive to.
		win := scr.SubImage(image.Rect(scr.Rect.Min.X+bestX0, scr.Rect.Min.Y+bestY0,
			scr.Rect.Min.X+bestX0+tw, scr.Rect.Min.Y+bestY0+th)).(*image.RGBA)
		best.ColorScore = histIntersection(colorHistogram(tmpl), colorHistogram(win))
	}
	return best
}

// far reports whether two points are at least d apart (Euclidean).
func far(ax, ay, bx, by, d int) bool {
	dx, dy := ax-bx, ay-by
	return dx*dx+dy*dy >= d*d
}

// preFilter samples 5 points (corners + center) of the template at (ox,oy) and requires
// most to agree — a fast rejection that tolerates a couple of off pixels (a cursor sitting
// on a corner). A masked (transparent) sample point isn't a constraint. When normalize is
// set the test is OFFSET-INVARIANT: it compares each sample's DIFFERENCE from a reference
// sample (a uniform brightness/colour shift adds the same constant to both, cancelling),
// so a brightness-shifted match isn't pre-rejected before countMismatch can confirm it.
func preFilter(scr, tmpl *image.RGBA, ox, oy, tol int, masked, normalize bool) bool {
	tw, th := tmpl.Rect.Dx(), tmpl.Rect.Dy()
	pts := [5][2]int{{0, 0}, {tw - 1, 0}, {0, th - 1}, {tw - 1, th - 1}, {tw / 2, th / 2}}
	if !normalize {
		hits := 0
		for _, p := range pts {
			tr, tg, tb, ta := at(tmpl, p[0], p[1])
			if masked && ta < maskAlpha { // masked pixel → not a constraint
				hits++
				continue
			}
			sr, sg, sb, _ := at(scr, ox+p[0], oy+p[1])
			if within(sr, sg, sb, tr, tg, tb, tol) {
				hits++
			}
		}
		return hits >= 3
	}
	// Offset-invariant: compare each non-masked sample's delta from the first to the
	// template's, so a uniform shift cancels. Majority of the deltas must agree.
	var srefR, srefG, srefB, trefR, trefG, trefB int
	haveRef := false
	hits, constraints := 0, 0
	for _, p := range pts {
		tr, tg, tb, ta := at(tmpl, p[0], p[1])
		if masked && ta < maskAlpha {
			continue
		}
		sr, sg, sb, _ := at(scr, ox+p[0], oy+p[1])
		if !haveRef {
			srefR, srefG, srefB, trefR, trefG, trefB = sr, sg, sb, tr, tg, tb
			haveRef = true
			continue
		}
		constraints++
		if within(sr-srefR, sg-srefG, sb-srefB, tr-trefR, tg-trefG, tb-trefB, tol) {
			hits++
		}
	}
	return constraints == 0 || hits*2 >= constraints
}

// gainLo/gainHi bound the contrast-scaling factor, so normalisation tolerates a real
// gamma/contrast change (~½×–2×) without a flat window trivially fitting a textured
// template.
const (
	gainLo = 0.5
	gainHi = 2.0
)

// countMismatch returns how many OPAQUE template pixels at (ox,oy) differ from the
// PREDICTED screen value by more than tol, aborting early once it exceeds limit. The
// prediction is meanS + gain·(template − meanT): with meanT=meanS=0 and gain=1 this is
// exact-tolerance matching; otherwise it cancels a brightness (mean) and contrast (gain)
// change. Transparent template pixels are skipped — the mask that lets the background shift.
func countMismatch(scr, tmpl *image.RGBA, ox, oy, tol, limit int, masked bool, meanT, meanS [3]int, gain [3]float64) int {
	tw, th := tmpl.Rect.Dx(), tmpl.Rect.Dy()
	tolF := float64(tol)
	miss := 0
	for ty := range th {
		for tx := range tw {
			tr, tg, tb, ta := at(tmpl, tx, ty)
			if masked && ta < maskAlpha {
				continue // masked: ignore the background
			}
			sr, sg, sb, _ := at(scr, ox+tx, oy+ty)
			pr := float64(meanS[0]) + gain[0]*float64(tr-meanT[0])
			pg := float64(meanS[1]) + gain[1]*float64(tg-meanT[1])
			pb := float64(meanS[2]) + gain[2]*float64(tb-meanT[2])
			if math.Abs(float64(sr)-pr) > tolF || math.Abs(float64(sg)-pg) > tolF || math.Abs(float64(sb)-pb) > tolF {
				miss++
				if miss > limit {
					return miss
				}
			}
		}
	}
	return miss
}

// normalizeMinRange is the minimum spread (max−min) a channel must have across the
// template's COMPARED pixels before mean-normalisation is used: below it the content is
// near-flat, where normalising would let any uniform region match.
const normalizeMinRange = 40

// opaqueVaried reports whether the template's compared (opaque) pixels have enough
// internal contrast to mean-normalise safely.
func opaqueVaried(tmpl *image.RGBA, masked bool) bool {
	tw, th := tmpl.Rect.Dx(), tmpl.Rect.Dy()
	minR, minG, minB := 256, 256, 256
	maxR, maxG, maxB := -1, -1, -1
	for ty := range th {
		for tx := range tw {
			r, g, b, a := at(tmpl, tx, ty)
			if masked && a < maskAlpha {
				continue
			}
			minR, maxR = min(minR, r), max(maxR, r)
			minG, maxG = min(minG, g), max(maxG, g)
			minB, maxB = min(minB, b), max(maxB, b)
		}
	}
	return max(maxR-minR, maxG-minG, maxB-minB) >= normalizeMinRange
}

// meanStdOpaque returns the per-channel mean and standard deviation of img sampled at
// tmpl's OPAQUE pixel positions offset by (ox,oy) — used for both the template's own
// statistics and the screen window's, so their means give the brightness offset and their
// std ratio the contrast gain.
func meanStdOpaque(img, tmpl *image.RGBA, ox, oy int, masked bool) (mean [3]int, std [3]float64) {
	tw, th := tmpl.Rect.Dx(), tmpl.Rect.Dy()
	var sum, sumsq [3]int64
	n := 0
	for ty := range th {
		for tx := range tw {
			_, _, _, ta := at(tmpl, tx, ty)
			if masked && ta < maskAlpha {
				continue
			}
			r, g, b, _ := at(img, ox+tx, oy+ty)
			px := [3]int{r, g, b}
			for c := range px {
				sum[c] += int64(px[c])
				sumsq[c] += int64(px[c] * px[c])
			}
			n++
		}
	}
	if n == 0 {
		return mean, std
	}
	for c := range mean {
		m := float64(sum[c]) / float64(n)
		v := float64(sumsq[c])/float64(n) - m*m
		if v < 0 {
			v = 0
		}
		mean[c] = int(m + 0.5)
		std[c] = math.Sqrt(v)
	}
	return mean, std
}

// clampF constrains v to [lo,hi].
func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// countOpaque counts template pixels that are opaque enough to be compared (alpha ≥
// maskAlpha) — the denominator for MatchThreshold once a mask is applied.
func countOpaque(img *image.RGBA) int {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	n := 0
	for y := range h {
		for x := range w {
			if _, _, _, a := at(img, x, y); a >= maskAlpha {
				n++
			}
		}
	}
	return n
}

// at returns the RGBA of a pixel at the image-local coordinate.
func at(img *image.RGBA, x, y int) (r, g, b, a int) {
	i := img.PixOffset(img.Rect.Min.X+x, img.Rect.Min.Y+y)
	return int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2]), int(img.Pix[i+3])
}

func within(r1, g1, b1, r2, g2, b2, tol int) bool {
	return iabs(r1-r2) <= tol && iabs(g1-g2) <= tol && iabs(b1-b2) <= tol
}

func iabs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
