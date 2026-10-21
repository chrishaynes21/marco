// Package preview keeps a picture of what a demonstration was shown, so a person reviewing it
// can see what Marco will press and what it expects to happen.
//
// # Why words were not enough
//
// Everything a saved demonstration knows about a control and a screen is a vocabulary: the words
// OCR read off them. That is the right thing for RECOGNITION — a label moving down a list does
// not make a new screen, and [[ADR-194-a-screen-is-also-what-it-says]] measured words keeping
// two visually identical menus apart where pixels could not. It is not enough for REVIEW:
//
//	Visual review needs retained target/start/result previews; OCR vocabulary is not an
//	adequate substitute.
//
// The owner's own report of the first trial was that they could tell which screens were being
// asked for "but not easily", reading `controls · display · profile · sound · video` and
// matching it against a game menu by eye. A picture of the screen answers that in no time at
// all, and a picture of the control answers "is that the thing I meant" — which words cannot,
// because two tabs in one bar read almost the same.
//
// # What this is NOT
//
// Not evidence, not identity, and not anything a run consults. Nothing here is read by the
// runner, the grounder, the verifier or the recogniser; no decision anywhere depends on a
// picture; and deleting every file this package writes leaves every Act performing exactly as
// before. It exists to be LOOKED AT by the person who recorded it.
//
// That matters for a second reason. [[ADR-181-a-demonstration-marco-can-keep]] and
// [[ADR-183-remembering-recognising-and-being-allowed-to-act]] keep a coordinate out of a
// durable identity, and a pixel is the strongest coordinate there is. A preview is cropped FROM
// a region the artifact already recorded and is stored beside it under the artifact's own label;
// it never becomes a thing to press, and the artifact still carries no image.
//
// # And it is kept only when somebody asked
//
// `director demonstrate-visual` prints "No frames and no screenshots are kept", and that
// sentence was true. Keeping crops of somebody's screen is a change to what a recording DOES
// with their desktop, so it is opt-in at the command (`--previews`), the surface that asks for
// it says what is kept and where, and nothing is ever sent anywhere.
package preview

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// ScreenWide is how wide a screen thumbnail is kept, in pixels.
//
// 640: wide enough that a game menu's headings are legible beside the words read off them, and
// small enough that a dozen of them are a few hundred kilobytes rather than a few megabytes. A
// preview is looked at, not measured, so there is nothing here that resolution has to support.
const ScreenWide = 640

// ControlWide is how wide a control crop is kept.
//
// Smaller than a screen because it IS smaller: a tab or a button is a few hundred pixels across
// at full size, and enlarging one past its own resolution shows a person nothing they could not
// already see.
const ControlWide = 360

// ControlMargin is how much of the surroundings a control crop keeps, as a fraction of the
// control's own size.
//
// A crop tight to the detected box answers "what did Marco find" and not "is that the thing I
// meant": a label with nothing around it could be any row of any list. A quarter of the box on
// each side puts it back in its bar.
const ControlMargin = 0.25

// Thumbnail is one picture of a screen, scaled down to ScreenWide.
func Thumbnail(frame image.Image) (image.Image, error) {
	if frame == nil {
		return nil, fmt.Errorf("preview: there is no frame to make a thumbnail of")
	}
	return scaled(frame, ScreenWide)
}

// Control is the crop of one control, with a margin of its surroundings, scaled to ControlWide.
//
// `where` is the control's own regions as the artifact recorded them — normalised to the window,
// which is what the frame is a capture of. Several because a control read in two places is two
// readings of one thing, and what a person needs to see is the whole of it.
func Control(frame image.Image, where []observe.Region) (image.Image, error) {
	if frame == nil {
		return nil, fmt.Errorf("preview: there is no frame to crop")
	}
	box, ok := pixels(frame.Bounds(), where)
	if !ok {
		return nil, fmt.Errorf("preview: the control has no region to crop")
	}
	cropped := image.NewRGBA(image.Rect(0, 0, box.Dx(), box.Dy()))
	draw.Draw(cropped, cropped.Bounds(), frame, box.Min, draw.Src)
	// NOT ENLARGED. `scaled` only ever shrinks, so a control already narrower than
	// ControlWide comes back at its own size.
	return scaled(cropped, ControlWide)
}

// PNG encodes a preview for storage.
func PNG(img image.Image) ([]byte, error) {
	if img == nil {
		return nil, fmt.Errorf("preview: there is nothing to encode")
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("preview: encoding: %w", err)
	}
	return buf.Bytes(), nil
}

// pixels is the rectangle covering every region, with its margin, inside the frame.
//
// Normalised regions are fractions of the window and the frame is a capture of that window, so
// the conversion is a multiplication — and the clamp at the end is not optional: a region
// recorded against a window that has since been resized, or a margin on a control at the very
// edge of the screen, both produce a rectangle outside the frame, and cropping to one panics.
func pixels(bounds image.Rectangle, where []observe.Region) (image.Rectangle, bool) {
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	if w <= 0 || h <= 0 {
		return image.Rectangle{}, false
	}
	first := true
	var box image.Rectangle
	for _, r := range where {
		if r.Width <= 0 || r.Height <= 0 {
			continue
		}
		one := image.Rect(
			bounds.Min.X+int(r.X*w),
			bounds.Min.Y+int(r.Y*h),
			bounds.Min.X+int((r.X+r.Width)*w),
			bounds.Min.Y+int((r.Y+r.Height)*h),
		)
		if first {
			box, first = one, false
			continue
		}
		box = box.Union(one)
	}
	if first {
		return image.Rectangle{}, false
	}
	grow := int(ControlMargin * float64(box.Dx()))
	if g := int(ControlMargin * float64(box.Dy())); g < grow {
		grow = g
	}
	box = box.Inset(-grow).Intersect(bounds)
	if box.Dx() <= 0 || box.Dy() <= 0 {
		return image.Rectangle{}, false
	}
	return box, true
}

// scaled shrinks an image to `wide` pixels across, keeping its shape. An image already that
// narrow is returned as it is.
//
// # Why the averaging and not nearest-neighbour
//
// Because the thing being shrunk is TEXT. A game menu scaled by picking one source pixel per
// destination pixel drops whole strokes of a letter — the word is still there and a person
// cannot read it, which defeats the entire purpose of keeping a picture. A box average over the
// source pixels that map to each destination pixel costs one pass and keeps the strokes grey
// instead of gone.
//
// Written here rather than taken from a library because the engine has no external
// dependencies, and this is twenty lines of arithmetic.
func scaled(src image.Image, wide int) (image.Image, error) {
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, fmt.Errorf("preview: the frame has no area")
	}
	if b.Dx() <= wide {
		return src, nil
	}
	high := b.Dy() * wide / b.Dx()
	if high < 1 {
		high = 1
	}
	out := image.NewRGBA(image.Rect(0, 0, wide, high))
	for y := 0; y < high; y++ {
		// THE SOURCE BAND THIS ROW IS THE AVERAGE OF.
		y0 := b.Min.Y + y*b.Dy()/high
		y1 := b.Min.Y + (y+1)*b.Dy()/high
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < wide; x++ {
			x0 := b.Min.X + x*b.Dx()/wide
			x1 := b.Min.X + (x+1)*b.Dx()/wide
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var rs, gs, bs, as uint64
			var n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					r, g, bl, a := src.At(sx, sy).RGBA()
					rs, gs, bs, as = rs+uint64(r), gs+uint64(g),
						bs+uint64(bl), as+uint64(a)
					n++
				}
			}
			if n == 0 {
				continue
			}
			out.Set(x, y, rgba(rs/n, gs/n, bs/n, as/n))
		}
	}
	return out, nil
}

// rgba turns averaged 16-bit premultiplied components back into a colour.
func rgba(r, g, b, a uint64) color.RGBA {
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8),
		A: uint8(a >> 8)}
}
