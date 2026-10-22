package localizetrial

// A SMALL FONT, SO THE EVIDENCE IS LEGIBLE.
//
// The labels could have been coloured blocks — nothing under test reads them, and the matcher
// sees grey levels either way. They are letters because the evidence images are part of the
// report: somebody looking at a case has to be able to see that the control in the left panel
// and the control in the right panel both say APPLY, which is the whole reason appearance cannot
// tell them apart.
//
// Five by seven, one byte per row, bit 4 leftmost. Only the letters the fixture uses are here;
// anything else draws a box, and `TestEveryLabelHasItsLetters` fails rather than letting a
// mystery box into the evidence.

import (
	"image"
	"image/color"
)

const (
	glyphW = 5
	glyphH = 7
)

var glyphs = map[rune][glyphH]byte{
	'A': {0x0E, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'C': {0x0E, 0x11, 0x10, 0x10, 0x10, 0x11, 0x0E},
	'D': {0x1E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1E},
	'E': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x1F},
	'G': {0x0E, 0x11, 0x10, 0x17, 0x11, 0x11, 0x0F},
	'L': {0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1F},
	'N': {0x11, 0x19, 0x1D, 0x17, 0x13, 0x11, 0x11},
	'P': {0x1E, 0x11, 0x11, 0x1E, 0x10, 0x10, 0x10},
	'R': {0x1E, 0x11, 0x11, 0x1E, 0x14, 0x12, 0x11},
	'S': {0x0F, 0x10, 0x10, 0x0E, 0x01, 0x01, 0x1E},
	'T': {0x1F, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
	'V': {0x11, 0x11, 0x11, 0x11, 0x11, 0x0A, 0x04},
	'Y': {0x11, 0x11, 0x0A, 0x04, 0x04, 0x04, 0x04},
}

// text draws a label at one pixel per dot.
func text(img *image.RGBA, at image.Point, s string, c color.RGBA) {
	put(img, at, s, c, 1)
}

// text2x draws a label at two pixels per dot, which is what a button carries.
func text2x(img *image.RGBA, at image.Point, s string, c color.RGBA) {
	put(img, at, s, c, 2)
}

func put(img *image.RGBA, at image.Point, s string, c color.RGBA, px int) {
	x := at.X
	for _, r := range s {
		g, ok := glyphs[r]
		if !ok {
			// A BOX, WHICH IS A BUG AND LOOKS LIKE ONE.
			outline(img, image.Rect(x, at.Y, x+glyphW*px, at.Y+glyphH*px), c)
			x += (glyphW + 1) * px
			continue
		}
		for row := 0; row < glyphH; row++ {
			for col := 0; col < glyphW; col++ {
				if g[row]&(1<<(glyphW-1-col)) == 0 {
					continue
				}
				fill(img, image.Rect(x+col*px, at.Y+row*px,
					x+col*px+px, at.Y+row*px+px), c)
			}
		}
		x += (glyphW + 1) * px
	}
}

// missingLetters is every rune of a label this font cannot draw.
func missingLetters(s string) []rune {
	var out []rune
	for _, r := range s {
		if _, ok := glyphs[r]; !ok {
			out = append(out, r)
		}
	}
	return out
}
