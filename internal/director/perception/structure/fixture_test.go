package structure_test

import (
	"image"
	"image/color"
	"math"
)

// DETERMINISTIC SCREENS, WITH GROUND TRUTH.
//
// The brief's standard is that Marco gets better at seeing UNFAMILIAR interfaces, not better at
// passing these tests. Two things here are in service of that and cost something:
//
// REAL GLYPHS, NOT BLOBS. The line-merging rule joins small components that share a horizontal
// band. Testing it against rectangles-standing-in-for-text would be testing it against its own
// design: of course a row of blobs merges. So text here is drawn from a 5x7 bitmap font, which
// produces what real text produces — strokes, enclosed counters, letters that are two separate
// components (`i`), gaps inside a word that are smaller than gaps between words. Go's standard
// library has no rasteriser and the engine may not take a dependency, so the font is defined
// below and is the honest alternative.
//
// HARD BACKGROUNDS. A flat grey page is the case the method cannot fail. The fixtures include a
// textured scene behind a translucent panel, a dark theme with little contrast, and a screen of
// unrelated clutter where the right answer is partly "no list here". A proposer tuned on easy
// frames reports recall that does not survive a game.
//
// What these CANNOT establish: that real anti-aliased subpixel-rendered text at an arbitrary
// scale behaves like this font, or that a real game's compositing behaves like this blending.
// They establish that the method works on drawn interfaces in principle and they are a
// regression corpus. Live frames remain the only evidence about live frames.

// ── a 5x7 bitmap font ─────────────────────────────────────────────────────────

// glyphs holds the letters the fixtures spell with. Enough for real menu words, no more.
var glyphs = map[rune][]string{
	'A': {".###.", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'C': {".####", "#....", "#....", "#....", "#....", "#....", ".####"},
	'E': {"#####", "#....", "#....", "####.", "#....", "#....", "#####"},
	'I': {"#####", "..#..", "..#..", "..#..", "..#..", "..#..", "#####"},
	'L': {"#....", "#....", "#....", "#....", "#....", "#....", "#####"},
	'M': {"#...#", "##.##", "#.#.#", "#...#", "#...#", "#...#", "#...#"},
	'N': {"#...#", "##..#", "#.#.#", "#..##", "#...#", "#...#", "#...#"},
	'O': {".###.", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'P': {"####.", "#...#", "#...#", "####.", "#....", "#....", "#...."},
	'Q': {".###.", "#...#", "#...#", "#...#", "#.#.#", "#..##", ".####"},
	'R': {"####.", "#...#", "#...#", "####.", "#.#..", "#..#.", "#...#"},
	'S': {".####", "#....", "#....", ".###.", "....#", "....#", "####."},
	'T': {"#####", "..#..", "..#..", "..#..", "..#..", "..#..", "..#.."},
	'U': {"#...#", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'X': {"#...#", "#...#", ".#.#.", "..#..", ".#.#.", "#...#", "#...#"},
	'Y': {"#...#", "#...#", ".#.#.", "..#..", "..#..", "..#..", "..#.."},
	'G': {".####", "#....", "#....", "#.###", "#...#", "#...#", ".###."},
	'D': {"####.", "#...#", "#...#", "#...#", "#...#", "#...#", "####."},
	'V': {"#...#", "#...#", "#...#", "#...#", ".#.#.", ".#.#.", "..#.."},
	'W': {"#...#", "#...#", "#...#", "#.#.#", "#.#.#", "##.##", "#...#"},
	'H': {"#...#", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'F': {"#####", "#....", "#....", "####.", "#....", "#....", "#...."},
	'B': {"####.", "#...#", "#...#", "####.", "#...#", "#...#", "####."},
	' ': {".....", ".....", ".....", ".....", ".....", ".....", "....."},
}

const glyphW, glyphH = 5, 7

// drawText writes s at (x, y) scaled by px, and returns the ink's bounding box.
//
// Letter spacing is one scaled pixel and word spacing is five, which is the ratio that makes
// line merging a real test: a merger with too tight a gap tolerance splits "MAIN MENU" in two,
// and one with too loose a tolerance joins two columns of a table.
func drawText(dst *image.RGBA, s string, x, y, px int, c color.RGBA) image.Rectangle {
	box := image.Rectangle{}
	cx := x
	for _, r := range s {
		g, ok := glyphs[r]
		if !ok {
			g = glyphs[' ']
		}
		for gy := range glyphH {
			for gx := range glyphW {
				if g[gy][gx] != '#' {
					continue
				}
				for dy := range px {
					for dx := range px {
						p := image.Pt(cx+gx*px+dx, y+gy*px+dy)
						dst.SetRGBA(p.X, p.Y, c)
						r := image.Rect(p.X, p.Y, p.X+1, p.Y+1)
						if box.Empty() {
							box = r
						} else {
							box = box.Union(r)
						}
					}
				}
			}
		}
		cx += glyphW*px + px
		if r == ' ' {
			cx += 4 * px
		}
	}
	return box
}

func textWidth(s string, px int) int {
	w := 0
	for _, r := range s {
		w += glyphW*px + px
		if r == ' ' {
			w += 4 * px
		}
	}
	return w - px
}

// ── fixtures ──────────────────────────────────────────────────────────────────

// screen is a fixture frame with what is actually on it.
type screen struct {
	Name string
	Img  *image.RGBA
	// Entries are the list rows a perception pass should find, top to bottom. The
	// rectangle is the row's full extent as a person would point at it.
	Entries []image.Rectangle
	// Selected is the index of the marked entry, or -1 when nothing is marked.
	Selected int
	// Decoys are other real structure on the screen: panels, titles, unrelated boxes.
	// Finding them is not an error; mistaking them for the list is.
	Decoys []image.Rectangle
}

func fill(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dst.SetRGBA(x, y, c)
		}
	}
}

func blend(dst *image.RGBA, r image.Rectangle, c color.RGBA, alpha float64) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			o := dst.RGBAAt(x, y)
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(float64(c.R)*alpha + float64(o.R)*(1-alpha)),
				G: uint8(float64(c.G)*alpha + float64(o.G)*(1-alpha)),
				B: uint8(float64(c.B)*alpha + float64(o.B)*(1-alpha)),
				A: 255,
			})
		}
	}
}

func outline(dst *image.RGBA, r image.Rectangle, c color.RGBA, w int) {
	fill(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+w), c)
	fill(dst, image.Rect(r.Min.X, r.Max.Y-w, r.Max.X, r.Max.Y), c)
	fill(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+w, r.Max.Y), c)
	fill(dst, image.Rect(r.Max.X-w, r.Min.Y, r.Max.X, r.Max.Y), c)
}

// scene paints a deterministic textured background: smooth gradients plus banded noise.
//
// Stands in for a rendered 3D scene, which is the case that defeats edge-and-projection
// segmentation: there is gradient ink everywhere and no quiet margin anywhere. Deterministic
// because a fixture whose background changes per run is not a fixture.
func scene(dst *image.RGBA) {
	b := dst.Bounds()
	for y := range b.Dy() {
		for x := range b.Dx() {
			fx, fy := float64(x)/float64(b.Dx()), float64(y)/float64(b.Dy())
			v := 0.5 + 0.35*math.Sin(fx*7.1+fy*2.3) + 0.18*math.Cos(fy*11.7-fx*3.1)
			n := math.Sin(float64(x)*12.9898+float64(y)*78.233) * 43758.5453
			n -= math.Floor(n)
			v += (n - 0.5) * 0.10
			v = math.Max(0, math.Min(1, v))
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(40 + v*120), G: uint8(60 + v*110), B: uint8(90 + v*90), A: 255,
			})
		}
	}
}

// highlight is how a fixture marks its selected row.
type highlight int

const (
	hlFilled   highlight = iota // a solid bar behind the row, the commonest case
	hlBorder                    // an outline only, nothing behind the text
	hlInverted                  // row and text swap colours
	hlBrighter                  // the text alone is brighter, geometry unchanged
	hlNone                      // nothing is marked
)

func (h highlight) String() string {
	switch h {
	case hlFilled:
		return "filled bar"
	case hlBorder:
		return "outline only"
	case hlInverted:
		return "inverted"
	case hlBrighter:
		return "brighter text"
	default:
		return "nothing marked"
	}
}

// desktopList is a light-themed settings list: the easy case, and the baseline.
func desktopList(sel int, hl highlight) screen {
	const w, h = 1280, 720
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill(img, img.Bounds(), color.RGBA{246, 246, 248, 255})

	panel := image.Rect(80, 90, 620, 560)
	fill(img, panel, color.RGBA{255, 255, 255, 255})
	outline(img, panel, color.RGBA{214, 214, 220, 255}, 2)
	title := drawText(img, "SETTINGS", 110, 110, 4, color.RGBA{30, 30, 34, 255})

	items := []string{"GENERAL", "DISPLAY", "SOUND", "NETWORK", "PRIVACY", "ABOUT"}
	s := screen{Name: "desktop list, " + hl.String(), Selected: sel,
		Decoys: []image.Rectangle{panel, title}}
	for i, it := range items {
		row := image.Rect(96, 160+i*62, 604, 160+i*62+52)
		ink := color.RGBA{40, 40, 46, 255}
		if i == sel {
			switch hl {
			case hlFilled:
				fill(img, row, color.RGBA{54, 118, 214, 255})
				ink = color.RGBA{255, 255, 255, 255}
			case hlBorder:
				outline(img, row, color.RGBA{54, 118, 214, 255}, 3)
			case hlInverted:
				fill(img, row, color.RGBA{30, 30, 34, 255})
				ink = color.RGBA{246, 246, 248, 255}
			case hlBrighter:
				ink = color.RGBA{10, 70, 200, 255}
			}
		}
		drawText(img, it, row.Min.X+18, row.Min.Y+13, 4, ink)
		s.Entries = append(s.Entries, row)
	}
	s.Img = img
	return s
}

// gameMenu is a translucent panel over a rendered scene: the measured hard case.
func gameMenu(sel int, hl highlight) screen {
	const w, h = 1280, 720
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scene(img)

	panel := image.Rect(430, 150, 860, 580)
	blend(img, panel, color.RGBA{12, 14, 20, 255}, 0.78)
	title := drawText(img, "PAUSED", 470, 175, 5, color.RGBA{235, 235, 240, 255})

	items := []string{"RESUME", "RESTART", "OPTIONS", "MAP", "QUIT"}
	s := screen{Name: "game menu over a scene, " + hl.String(), Selected: sel,
		Decoys: []image.Rectangle{panel, title}}
	for i, it := range items {
		row := image.Rect(452, 250+i*62, 838, 250+i*62+50)
		ink := color.RGBA{206, 210, 218, 255}
		if i == sel {
			switch hl {
			case hlFilled:
				fill(img, row, color.RGBA{232, 188, 64, 255})
				ink = color.RGBA{20, 18, 12, 255}
			case hlBorder:
				outline(img, row, color.RGBA{232, 188, 64, 255}, 3)
			case hlInverted:
				fill(img, row, color.RGBA{228, 230, 236, 255})
				ink = color.RGBA{16, 18, 24, 255}
			case hlBrighter:
				ink = color.RGBA{255, 232, 130, 255}
			}
		}
		drawText(img, it, row.Min.X+20, row.Min.Y+12, 4, ink)
		s.Entries = append(s.Entries, row)
	}
	s.Img = img
	return s
}

// darkTheme is a low-contrast list: rows a few levels off their own background.
func darkTheme(sel int, hl highlight) screen {
	const w, h = 1280, 720
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill(img, img.Bounds(), color.RGBA{18, 18, 20, 255})
	panel := image.Rect(300, 120, 980, 600)
	fill(img, panel, color.RGBA{26, 26, 29, 255})

	items := []string{"INBOX", "SENT", "DRAFTS", "ARCHIVE", "TRASH", "SPAM", "NOTES"}
	s := screen{Name: "dark low-contrast list, " + hl.String(), Selected: sel,
		Decoys: []image.Rectangle{panel}}
	for i, it := range items {
		row := image.Rect(316, 140+i*62, 964, 140+i*62+52)
		fill(img, row, color.RGBA{33, 33, 37, 255})
		ink := color.RGBA{150, 152, 158, 255}
		if i == sel {
			switch hl {
			case hlFilled:
				fill(img, row, color.RGBA{48, 50, 58, 255})
				ink = color.RGBA{225, 226, 230, 255}
			case hlBorder:
				outline(img, row, color.RGBA{86, 90, 102, 255}, 3)
			case hlInverted:
				fill(img, row, color.RGBA{200, 202, 208, 255})
				ink = color.RGBA{20, 20, 24, 255}
			case hlBrighter:
				ink = color.RGBA{236, 238, 242, 255}
			}
		}
		drawText(img, it, row.Min.X+16, row.Min.Y+13, 4, ink)
		s.Entries = append(s.Entries, row)
	}
	s.Img = img
	return s
}

// clutter is a screen with a list AND a heads-up display: the precision case.
//
// The medal-times panel on the measured Trackmania frame was six evenly spaced rows, as regular
// as the menu and smaller, and an earlier grouping rule picked it instead. So this fixture
// carries a second regular stack on purpose. Discovery should find both; only one of them is
// ever marked, and that is what selection has to decide on.
func clutter(sel int, hl highlight) screen {
	const w, h = 1280, 720
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scene(img)

	// The real list, left.
	panel := image.Rect(90, 140, 520, 560)
	blend(img, panel, color.RGBA{16, 18, 24, 255}, 0.8)
	items := []string{"TRACK ONE", "TRACK TWO", "TRACK SIX", "TRACK TEN"}
	s := screen{Name: "list beside a heads-up display, " + hl.String(), Selected: sel,
		Decoys: []image.Rectangle{panel}}
	for i, it := range items {
		row := image.Rect(108, 170+i*72, 502, 170+i*72+58)
		ink := color.RGBA{205, 208, 214, 255}
		if i == sel {
			switch hl {
			case hlFilled:
				fill(img, row, color.RGBA{80, 190, 120, 255})
				ink = color.RGBA{12, 30, 18, 255}
			case hlBorder:
				outline(img, row, color.RGBA{80, 190, 120, 255}, 3)
			case hlInverted:
				fill(img, row, color.RGBA{225, 228, 232, 255})
				ink = color.RGBA{14, 16, 20, 255}
			case hlBrighter:
				ink = color.RGBA{150, 255, 190, 255}
			}
		}
		drawText(img, it, row.Min.X+16, row.Min.Y+14, 4, ink)
		s.Entries = append(s.Entries, row)
	}

	// A second regular stack, right: times that all change together and none of which is
	// ever the chosen one.
	hud := image.Rect(900, 180, 1210, 470)
	blend(img, hud, color.RGBA{10, 10, 14, 255}, 0.55)
	s.Decoys = append(s.Decoys, hud)
	for i, t := range []string{"GOLD", "SILVER", "BRONZE", "BEST", "LAST"} {
		r := image.Rect(916, 196+i*54, 1194, 196+i*54+42)
		drawText(img, t, r.Min.X+10, r.Min.Y+9, 3, color.RGBA{190, 194, 204, 255})
		s.Decoys = append(s.Decoys, r)
	}
	s.Img = img
	return s
}

// corpus is every fixture, at one selection and one highlight style.
func corpus(sel int, hl highlight) []screen {
	return []screen{
		desktopList(sel, hl),
		gameMenu(sel, hl),
		darkTheme(sel, hl),
		clutter(sel, hl),
	}
}
