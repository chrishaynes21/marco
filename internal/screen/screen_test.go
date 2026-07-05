package screen

import (
	"image"
	"image/color"
	"testing"
)

func TestAutoCropFindsButton(t *testing.T) {
	// Flat background with a bright button in the middle — AutoCrop should crop to it.
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	fill(img, 0, 0, 128, 128, color.RGBA{40, 40, 40, 255})    // background
	fill(img, 50, 50, 80, 80, color.RGBA{220, 220, 220, 255}) // 30x30 button
	out := AutoCrop(img)
	if dx := out.Bounds().Dx(); dx >= 120 || dx < 20 {
		t.Fatalf("expected a crop around the button (much smaller than 128, but not tiny), got width %d", dx)
	}
}

func TestAutoCropAtCropsToButtonAndRecentres(t *testing.T) {
	// A whole-"desktop" frame with a wide button far from centre; the click lands inside
	// it off-centre. AutoCropAt should crop to ~the button and report the BUTTON centre,
	// not the click — so the recorder can re-centre the anchor on the control.
	img := image.NewRGBA(image.Rect(0, 0, 1200, 800))
	fill(img, 0, 0, 1200, 800, color.RGBA{30, 30, 30, 255})
	// A 240x60 button at (400..640, 300..360) — wider than the old 128px patch.
	fill(img, 400, 300, 640, 360, color.RGBA{210, 210, 210, 255})
	out, cx, cy, clx, cly := AutoCropAt(img, 430, 330, 64) // click near the button's left edge
	if iabs(cx-520) > 20 || iabs(cy-330) > 20 {            // button centre ≈ (520,330)
		t.Fatalf("re-centred to (%d,%d), want ~(520,330)", cx, cy)
	}
	// Cropped template covers the whole button width (the fragment problem fixed), much
	// wider than a click-centred 128px patch would be.
	if dx := out.Bounds().Dx(); dx < 200 || dx > 320 {
		t.Fatalf("crop width %d, want ~the 240px button (+margin)", dx)
	}
	// The click-local position lands inside the crop near the click's left-edge offset
	// (~430-400+margin), NOT the re-centred button middle — that's what lets OCR read the
	// button under the click.
	if clx < 0 || cly < 0 || clx >= out.Bounds().Dx() || cly >= out.Bounds().Dy() {
		t.Fatalf("click-local (%d,%d) outside crop %v", clx, cly, out.Bounds())
	}
	if clx > out.Bounds().Dx()/2 {
		t.Fatalf("click-local x %d should be left-of-centre (clicked near the left edge)", clx)
	}
}

func TestAutoCropAtRejectsPanelOverGrab(t *testing.T) {
	// Edge detection can merge a whole menu's rows into one tall component. Clicking the
	// TOP item must NOT re-centre on that panel's MIDDLE (a different item): the crop stays
	// near the click (tightening may bound it at the panel's edge, which is fine), and is
	// nowhere near the whole panel's size.
	img := image.NewRGBA(image.Rect(0, 0, 1000, 1000))
	fill(img, 0, 0, 1000, 1000, color.RGBA{30, 30, 30, 255})
	fill(img, 100, 100, 760, 900, color.RGBA{200, 200, 200, 255}) // a 660x800 "panel"
	out, cx, cy, _, _ := AutoCropAt(img, 200, 160, 80)            // click near the panel's TOP
	if iabs(cx-200) > 40 || iabs(cy-160) > 40 {
		t.Fatalf("re-centred to (%d,%d); must stay near the click (200,160), not the panel middle (~500)", cx, cy)
	}
	if dx := out.Bounds().Dx(); dx > 200 { // small region, not the 660px panel
		t.Fatalf("expected a small crop, got width %d", dx)
	}
}

func TestTightenToButtonIsolatesStackedButton(t *testing.T) {
	// Three stacked buttons with thin gaps — the case DetectButtons FUSES into one panel.
	// Given that merged box and the click on the MIDDLE button, tightening must isolate the
	// middle button alone (grow from the click, stop at the gaps), not keep the whole stack.
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	fill(img, 0, 0, 200, 200, color.RGBA{30, 30, 30, 255}) // dark background / gaps
	fill(img, 10, 30, 190, 70, color.RGBA{180, 180, 180, 255})
	fill(img, 10, 80, 190, 120, color.RGBA{180, 180, 180, 255}) // middle button (gaps at 70-80, 120-130)
	fill(img, 10, 130, 190, 170, color.RGBA{180, 180, 180, 255})
	merged := image.Rect(10, 30, 190, 170) // what connected-components would over-grab to

	r := tightenToButton(img, merged, 100, 100) // click the middle button
	if r.Min.Y > 82 || r.Max.Y < 118 || r.Dy() > 55 {
		t.Fatalf("did not isolate the middle button: got %v (want ~y80-120), Dy %d", r, r.Dy())
	}
	if !(image.Pt(100, 100).In(r)) {
		t.Fatalf("tightened box %v lost the click", r)
	}
	// Full-width button: left/right run to the button's own edges, not split mid-row.
	if r.Min.X > 12 || r.Max.X < 188 {
		t.Fatalf("tightened box %v shouldn't crop the button's width", r)
	}
}

func TestTightenToButtonGradientFill(t *testing.T) {
	// A button with a vertical GRADIENT fill must still grow as one band (local continuity),
	// stopping only at the sharp gap — not at the gradual brightness change inside it.
	img := image.NewRGBA(image.Rect(0, 0, 100, 200))
	fill(img, 0, 0, 100, 200, color.RGBA{20, 20, 20, 255})
	for y := 60; y < 120; y++ { // one gradient button, 200→140 top-to-bottom
		v := uint8(200 - (y - 60)) // changes ~1/row → well under the band tolerance
		fill(img, 10, y, 90, y+1, color.RGBA{v, v, v, 255})
	}
	r := tightenToButton(img, image.Rect(0, 0, 100, 200), 50, 90)
	if r.Min.Y > 62 || r.Max.Y < 118 {
		t.Fatalf("gradient button not grown whole: %v (want ~y60-120)", r)
	}
}

func TestAutoCropAtFallsBackToClickPatch(t *testing.T) {
	// A flat frame with no button at the click: AutoCropAt returns a click-centred patch
	// of the fallback radius, with the click as the centre (no re-centre).
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	fill(img, 0, 0, 800, 600, color.RGBA{30, 30, 30, 255})
	out, cx, cy, clx, cly := AutoCropAt(img, 400, 300, 64)
	if cx != 400 || cy != 300 {
		t.Fatalf("flat-area centre = (%d,%d), want the click (400,300)", cx, cy)
	}
	if dx, dy := out.Bounds().Dx(), out.Bounds().Dy(); dx != 129 || dy != 129 {
		t.Fatalf("fallback patch = %dx%d, want 129x129 (radius 64)", dx, dy)
	}
	if clx != 64 || cly != 64 { // click is the patch centre: radius 64 → (64,64)
		t.Fatalf("click-local = (%d,%d), want (64,64)", clx, cly)
	}
}

func TestAutoCropAtGrowsLabelFragmentToButton(t *testing.T) {
	// End-to-end through AutoCropAt (the recorder path): a wide button with a short label at
	// one end, clicked ON the label. DetectButtons bounds to the label glyphs; AutoCropAt must
	// grow out to the whole button and re-centre the anchor on the button's middle, not leave
	// a text sliver re-centred near the label.
	img := image.NewRGBA(image.Rect(0, 0, 1200, 800))
	fill(img, 0, 0, 1200, 800, color.RGBA{20, 30, 50, 255})
	for y := 360; y < 440; y++ {
		v := uint8(170 + (y-360)/4)
		fill(img, 300, y, 860, y+1, color.RGBA{v, v, v, 255}) // 560x80 button
	}
	for i := range 24 { // thin strokes 4px apart → fuse into ONE label component (~33% ink)
		x := 330 + i*6
		fill(img, x, 390, x+2, 410, color.RGBA{20, 20, 20, 255}) // label at the left end
	}
	out, cx, cy, _, _ := AutoCropAt(img, 360, 400, 64) // click on the label
	if dx := out.Bounds().Dx(); dx < 480 {
		t.Fatalf("crop width %d — should span ~the 560px button, not the ~140px label", dx)
	}
	if iabs(cx-580) > 40 || iabs(cy-400) > 25 { // button centre ≈ (580,400)
		t.Fatalf("re-centred to (%d,%d), want ~the button centre (580,400)", cx, cy)
	}
}

func TestGrowButtonBandFragmentToWholeButton(t *testing.T) {
	// The real failure: a wide button with a SHORT label at one end. DetectButtons bounds the
	// crop to the label's glyphs (a separate inner component), so buttonAt returns that text
	// box — a sliver. growButtonBand must expand it out to the whole button fill, stopping at
	// the button's border on every side.
	img := image.NewRGBA(image.Rect(0, 0, 1000, 600))
	fill(img, 0, 0, 1000, 600, color.RGBA{20, 30, 50, 255}) // dark menu background
	// A 560x80 button at (300..860, 360..440), light gradient fill (no internal edges).
	for y := 360; y < 440; y++ {
		v := uint8(170 + (y-360)/4) // ~170→189 top-to-bottom, a gentle gradient
		fill(img, 300, y, 860, y+1, color.RGBA{v, v, v, 255})
	}
	// Short "label" at the LEFT end (x 330..470), dark glyph bars within the button.
	for i := range 5 {
		x := 330 + i*28
		fill(img, x, 390, x+10, 410, color.RGBA{20, 20, 20, 255})
	}
	textBox := image.Rect(330, 390, 470, 410) // ~what DetectButtons bounds the label to
	got := growButtonBand(img, textBox, 360, 400)
	// Grew out to ~the whole button (300..860 x 360..440), not the 140x20 label.
	if got.Min.X > 305 || got.Max.X < 855 || got.Min.Y > 365 || got.Max.Y < 435 {
		t.Fatalf("did not grow the label box out to the button: got %v, want ~(300,360)-(860,440)", got)
	}
	if got.Min.X < 295 || got.Max.X > 865 || got.Min.Y < 355 || got.Max.Y > 445 {
		t.Fatalf("grew past the button's borders into the background: got %v", got)
	}
}

func TestGrowButtonBandRevertsOnBorderlessField(t *testing.T) {
	// A label on a big borderless gradient field (no button to stop at). Growth must find no
	// border within the cap on the open sides and REVERT them — never run away across the field.
	img := image.NewRGBA(image.Rect(0, 0, 2000, 2000))
	for y := range 2000 {
		v := uint8(100 + y/16) // a slow full-frame gradient — no edges anywhere
		fill(img, 0, y, 2000, y+1, color.RGBA{v, v, v, 255})
	}
	textBox := image.Rect(900, 980, 1040, 1000)
	got := growButtonBand(img, textBox, 950, 990)
	// No border anywhere → every side hit the cap → reverted to the original box.
	if got != textBox {
		t.Fatalf("borderless field should revert to the text box, got %v want %v", got, textBox)
	}
}

func TestGrowButtonBandCleanButtonUnchanged(t *testing.T) {
	// A solid high-contrast button DetectButtons already captures whole: growth stops at its
	// border on the first step out and returns it unchanged (no regression).
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	fill(img, 0, 0, 400, 300, color.RGBA{30, 30, 30, 255})
	fill(img, 120, 110, 280, 170, color.RGBA{210, 210, 210, 255}) // 160x60 button
	box := image.Rect(120, 110, 280, 170)
	got := growButtonBand(img, box, 200, 140)
	if got != box {
		t.Fatalf("clean button should be unchanged, got %v want %v", got, box)
	}
}

func TestFindEdgeRecoloured(t *testing.T) {
	// Template: a box OUTLINE (dark bg, bright box → edges at the box border).
	tmpl := image.NewRGBA(image.Rect(0, 0, 30, 30))
	fill(tmpl, 0, 0, 30, 30, color.RGBA{20, 20, 20, 255})
	fill(tmpl, 5, 5, 25, 25, color.RGBA{200, 200, 200, 255})
	// Screen: the SAME box shape but a DIFFERENT fill colour (a recolour/theme change).
	scr := image.NewRGBA(image.Rect(0, 0, 120, 80))
	fill(scr, 0, 0, 120, 80, color.RGBA{20, 20, 20, 255})
	fill(scr, 47, 37, 67, 57, color.RGBA{40, 180, 90, 255}) // box at (47,37), green not white
	m := findEdgeDT(scr, tmpl)
	if !m.Found {
		t.Fatalf("edge match should find the recoloured box: %+v", m)
	}
	if iabs(m.X-57) > 4 || iabs(m.Y-47) > 4 { // box centre ≈ (57,47)
		t.Fatalf("centre off: %+v, want ~(57,47)", m)
	}
	// No matching outline anywhere → not found.
	flat := image.NewRGBA(image.Rect(0, 0, 120, 80))
	fill(flat, 0, 0, 120, 80, color.RGBA{20, 20, 20, 255})
	if m := findEdgeDT(flat, tmpl); m.Found {
		t.Fatalf("a flat screen should not edge-match: %+v", m)
	}
}

func TestFindEdgeMultiScale(t *testing.T) {
	// Template: a 20x20 box outline. Screen: the same box at ~1.25x (25x25) — a DPI bump.
	tmpl := image.NewRGBA(image.Rect(0, 0, 30, 30))
	fill(tmpl, 0, 0, 30, 30, color.RGBA{20, 20, 20, 255})
	fill(tmpl, 5, 5, 25, 25, color.RGBA{200, 200, 200, 255})
	scr := image.NewRGBA(image.Rect(0, 0, 140, 120))
	fill(scr, 0, 0, 140, 120, color.RGBA{20, 20, 20, 255})
	fill(scr, 50, 45, 75, 70, color.RGBA{200, 200, 200, 255}) // 25x25 box
	if m := findEdgeMultiScale(scr, tmpl); !m.Found {
		t.Fatalf("multi-scale edge should find the upscaled box: %+v", m)
	}
}

func TestSnapToButton(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 160))
	fill(img, 0, 0, 200, 160, color.RGBA{40, 40, 40, 255})
	fill(img, 60, 50, 140, 110, color.RGBA{200, 200, 200, 255}) // button ~centre (100,80)
	// A point inside the button but off-centre (like a word hit) snaps to the button centre.
	cx, cy, ok := SnapToButton(img, 75, 95)
	if !ok {
		t.Fatal("should snap to the button")
	}
	if iabs(cx-100) > 10 || iabs(cy-80) > 10 {
		t.Fatalf("snapped to (%d,%d), want ~(100,80)", cx, cy)
	}
	// A point on flat background snaps to nothing.
	if _, _, ok := SnapToButton(img, 10, 10); ok {
		t.Fatal("flat background should not snap")
	}
}

func TestResizeDims(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 20, 10))
	if d := resize(src, 1.5).Bounds(); d.Dx() != 30 || d.Dy() != 15 {
		t.Fatalf("resize 1.5 dims = %v, want 30x15", d)
	}
	if resize(src, 1.0) != src {
		t.Fatal("resize 1.0 should return the source unchanged")
	}
}

func TestDetectButtons(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 160, 120))
	fill(img, 0, 0, 160, 120, color.RGBA{40, 40, 40, 255})    // background
	fill(img, 30, 30, 90, 70, color.RGBA{220, 220, 220, 255}) // a 60x40 button
	boxes := DetectButtons(img)
	covers := false
	for _, r := range boxes {
		if r.Min.X <= 35 && r.Min.Y <= 35 && r.Max.X >= 85 && r.Max.Y >= 65 {
			covers = true
		}
	}
	if !covers {
		t.Fatalf("expected a detected box covering the button, got %v", boxes)
	}
}

func TestAutoCropFlatUnchanged(t *testing.T) {
	// A flat patch has no content to crop to — returned unchanged.
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	fill(img, 0, 0, 64, 64, color.RGBA{40, 40, 40, 255})
	if out := AutoCrop(img); out.Bounds() != img.Bounds() {
		t.Fatalf("flat patch should be unchanged, got %v", out.Bounds())
	}
}

func TestMatchAmbiguous(t *testing.T) {
	// A distinctive template: dark border, red centre (fill takes corners x0,y0,x1,y1).
	tmpl := image.NewRGBA(image.Rect(0, 0, 16, 16))
	fill(tmpl, 0, 0, 16, 16, color.RGBA{30, 30, 30, 255})
	fill(tmpl, 4, 4, 12, 12, color.RGBA{220, 40, 40, 255})
	paint := func(img *image.RGBA, ox, oy int) {
		fill(img, ox, oy, ox+16, oy+16, color.RGBA{30, 30, 30, 255})
		fill(img, ox+4, oy+4, ox+12, oy+12, color.RGBA{220, 40, 40, 255})
	}

	// TWO copies far apart → ambiguous.
	two := image.NewRGBA(image.Rect(0, 0, 300, 80))
	fill(two, 0, 0, 300, 80, color.RGBA{0, 0, 0, 255})
	paint(two, 20, 30)
	paint(two, 220, 30)
	if m := match(two, tmpl, 16); !m.Found || !m.Ambiguous {
		t.Fatalf("two copies should be found AND ambiguous: %+v", m)
	}

	// ONE copy → found, not ambiguous.
	one := image.NewRGBA(image.Rect(0, 0, 300, 80))
	fill(one, 0, 0, 300, 80, color.RGBA{0, 0, 0, 255})
	paint(one, 20, 30)
	if m := match(one, tmpl, 16); !m.Found || m.Ambiguous {
		t.Fatalf("one copy should be found and NOT ambiguous: %+v", m)
	}
}

func TestDistinctive(t *testing.T) {
	// A uniform patch is not worth matching by image.
	uniform := image.NewRGBA(image.Rect(0, 0, 32, 32))
	fill(uniform, 0, 0, 32, 32, color.RGBA{200, 200, 200, 255})
	if Distinctive(uniform) {
		t.Error("a uniform patch should not be distinctive")
	}
	// A patch with a clear feature (a filled quadrant) is.
	feature := image.NewRGBA(image.Rect(0, 0, 32, 32))
	fill(feature, 0, 0, 32, 32, color.RGBA{255, 255, 255, 255})
	fill(feature, 0, 0, 20, 20, color.RGBA{0, 0, 0, 255})
	if !Distinctive(feature) {
		t.Error("a patch with a strong feature should be distinctive")
	}
	if Distinctive(nil) {
		t.Error("nil is not distinctive")
	}
}

func TestEncodePNGRoundTrips(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	fill(img, 0, 0, 2, 2, color.RGBA{10, 20, 30, 255})
	data, err := EncodePNG(img)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("EncodePNG returned no bytes")
	}
	// PNG magic.
	if string(data[1:4]) != "PNG" {
		t.Fatalf("not a PNG: % x", data[:8])
	}
}

// TestMatchAlphaMask: a template whose background is transparent matches the
// foreground (a cross of lines) even when the on-screen background differs from
// what was captured — only opaque template pixels are compared.
func TestMatchAlphaMask(t *testing.T) {
	line := color.RGBA{240, 240, 240, 255}
	mkTmpl := func() *image.RGBA {
		// 10x10, fully transparent, with an opaque plus-sign of light lines.
		t := image.NewRGBA(image.Rect(0, 0, 10, 10))
		fill(t, 5, 0, 6, 10, line) // vertical line, opaque
		fill(t, 0, 5, 10, 6, line) // horizontal line, opaque
		return t
	}
	// Same foreground lines over two DIFFERENT backgrounds.
	mkScreen := func(bg color.RGBA) *image.RGBA {
		s := image.NewRGBA(image.Rect(0, 0, 40, 40))
		fill(s, 0, 0, 40, 40, bg)
		fill(s, 20, 15, 21, 25, line) // vertical, at screen (20,15)-(21,25)
		fill(s, 15, 20, 25, 21, line) // horizontal
		return s
	}
	for _, bg := range []color.RGBA{{10, 10, 10, 255}, {180, 40, 90, 255}} {
		m := match(mkScreen(bg), mkTmpl(), 8)
		if !m.Found || m.X != 20 || m.Y != 20 { // template center lands on the cross center
			t.Fatalf("masked match over bg %v = %+v, want found center (20,20)", bg, m)
		}
	}
}

// TestMatchNoAlphaIsOpaque: a captured template carries no alpha (Windows BitBlt
// leaves A=0), so an all-transparent template must be treated as fully opaque and
// matched on RGB — NOT masked out to nothing (the bug that made every anchor miss).
func TestMatchNoAlphaIsOpaque(t *testing.T) {
	scr := image.NewRGBA(image.Rect(0, 0, 40, 40))
	fill(scr, 0, 0, 40, 40, color.RGBA{10, 10, 10, 255})
	fill(scr, 18, 18, 24, 24, color.RGBA{200, 20, 20, 255}) // a red square at (18,18)-(24,24)
	// Template = the red square with NO alpha set (A=0), like a real capture.
	tmpl := image.NewRGBA(image.Rect(0, 0, 6, 6))
	for y := range 6 {
		for x := range 6 {
			tmpl.SetRGBA(x, y, color.RGBA{200, 20, 20, 0}) // A=0 on purpose
		}
	}
	if m := match(scr, tmpl, 8); !m.Found || m.X != 21 || m.Y != 21 {
		t.Fatalf("no-alpha template should match on RGB, got %+v want found (21,21)", m)
	}
}

// fill paints a solid rectangle into img.
func fill(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func TestMatchFound(t *testing.T) {
	scr := image.NewRGBA(image.Rect(0, 0, 100, 80))
	fill(scr, 0, 0, 100, 80, color.RGBA{10, 10, 10, 255})
	// A 6x6 red square at (40,30).
	red := color.RGBA{200, 20, 20, 255}
	fill(scr, 40, 30, 46, 36, red)

	tmpl := image.NewRGBA(image.Rect(0, 0, 6, 6))
	fill(tmpl, 0, 0, 6, 6, red)

	m := match(scr, tmpl, 0)
	if !m.Found || m.X != 43 || m.Y != 33 { // center of 40..46 / 30..36
		t.Fatalf("got %+v, want center (43,33) found", m)
	}
}

func TestMatchNotFound(t *testing.T) {
	scr := image.NewRGBA(image.Rect(0, 0, 50, 50))
	fill(scr, 0, 0, 50, 50, color.RGBA{0, 0, 0, 255})
	tmpl := image.NewRGBA(image.Rect(0, 0, 4, 4))
	fill(tmpl, 0, 0, 4, 4, color.RGBA{255, 255, 255, 255})
	if m := match(scr, tmpl, 10); m.Found {
		t.Fatalf("expected not found, got %+v", m)
	}
}

// TestMatchBrightnessInvariant: a distinctive template still matches when the on-screen
// content is uniformly dimmed (a lighting / night-mode / monitor-brightness change) —
// every pixel is off by more than the tolerance, so only mean-normalisation finds it.
func TestMatchBrightnessInvariant(t *testing.T) {
	tmpl := image.NewRGBA(image.Rect(0, 0, 20, 20))
	fill(tmpl, 0, 0, 20, 20, color.RGBA{60, 60, 60, 255})
	fill(tmpl, 5, 5, 15, 15, color.RGBA{200, 180, 160, 255}) // distinctive (25% differs)
	dim := func(v, d uint8) uint8 {
		if v < d {
			return 0
		}
		return v - d
	}
	scr := image.NewRGBA(image.Rect(0, 0, 80, 60))
	fill(scr, 0, 0, 80, 60, color.RGBA{dim(60, 40), dim(60, 40), dim(60, 40), 255})
	fill(scr, 30, 20, 40, 30, color.RGBA{dim(200, 40), dim(180, 40), dim(160, 40), 255})
	// Off by 40 everywhere — exact-tolerance (24) would miss; normalised matches.
	m := match(scr, tmpl, 24)
	if !m.Found {
		t.Fatalf("brightness-shifted content should match when normalised: %+v", m)
	}
	if iabs(m.X-35) > 3 || iabs(m.Y-25) > 3 { // square centre ≈ (35,25)
		t.Fatalf("matched at (%d,%d), want ~(35,25)", m.X, m.Y)
	}
}

// TestMatchContrastInvariant: a distinctive template still matches when the on-screen
// content has REDUCED contrast (a gamma / tone-mapping change) around the same mean —
// every interior pixel is off by more than the tolerance, so only the gain term finds it.
func TestMatchContrastInvariant(t *testing.T) {
	tmpl := image.NewRGBA(image.Rect(0, 0, 20, 20))
	fill(tmpl, 0, 0, 20, 20, color.RGBA{40, 40, 40, 255})    // mean ≈ 90
	fill(tmpl, 5, 5, 15, 15, color.RGBA{240, 240, 240, 255}) // high contrast
	scr := image.NewRGBA(image.Rect(0, 0, 80, 60))
	fill(scr, 0, 0, 80, 60, color.RGBA{65, 65, 65, 255})      // half-contrast around mean 90
	fill(scr, 30, 20, 40, 30, color.RGBA{165, 165, 165, 255}) // (square 240→165, bg 40→65)
	m := match(scr, tmpl, 24)
	if !m.Found {
		t.Fatalf("contrast-reduced content should match (NCC gain): %+v", m)
	}
	if iabs(m.X-35) > 3 || iabs(m.Y-25) > 3 {
		t.Fatalf("matched at (%d,%d), want ~(35,25)", m.X, m.Y)
	}
}

func TestMatchTolerance(t *testing.T) {
	scr := image.NewRGBA(image.Rect(0, 0, 30, 30))
	fill(scr, 0, 0, 30, 30, color.RGBA{0, 0, 0, 255})
	// Square is slightly off from the template color.
	fill(scr, 10, 10, 14, 14, color.RGBA{200, 200, 200, 255})
	tmpl := image.NewRGBA(image.Rect(0, 0, 4, 4))
	fill(tmpl, 0, 0, 4, 4, color.RGBA{210, 195, 205, 255})

	if m := match(scr, tmpl, 4); m.Found {
		t.Fatalf("tolerance 4 should not match a 10-off pixel: %+v", m)
	}
	if m := match(scr, tmpl, 15); !m.Found || m.X != 12 || m.Y != 12 {
		t.Fatalf("tolerance 15 should match at center (12,12): %+v", m)
	}
}
