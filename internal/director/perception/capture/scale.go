package capture

import "image"

// Scale enlarges an image by an integer factor, bilinearly.
//
// Small crops are why this exists. An OCR engine reading a 340×47 button at native size
// has about twenty pixels of glyph height to work with, and tesseract wants roughly double
// that; enlarging the crop first is the difference between "SETTINGS" and nothing. Measured
// on a real Rocket League pause menu: at native size three of four buttons read, at 3× all
// four did.
//
// Bilinear rather than nearest-neighbour because nearest reproduces the original's
// staircase edges at triple size, which is exactly the artefact that makes a thin stroke
// look like a dashed one. Bilinear rather than bicubic because bicubic overshoots at high
// contrast edges — it rings — and a ringing halo around a glyph is a stroke the engine did
// not ask for. The engine module takes no dependencies, so this is hand-written; that is a
// constraint worth respecting rather than a gap.
//
// A factor of 1 or less returns the image unchanged rather than copying it.
func Scale(img image.Image, factor int) image.Image {
	if img == nil || factor <= 1 {
		return img
	}
	src := img.Bounds()
	sw, sh := src.Dx(), src.Dy()
	if sw <= 0 || sh <= 0 {
		return img
	}
	dw, dh := sw*factor, sh*factor
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))

	for y := 0; y < dh; y++ {
		// The centre of the destination pixel, mapped back into source space. Using
		// centres rather than corners keeps the result from drifting half a pixel up and
		// left, which at 3× is a visible shift of every stroke.
		fy := (float64(y)+0.5)/float64(factor) - 0.5
		y0, wy := split(fy, sh)
		y1 := clampInt(y0+1, 0, sh-1)

		for x := 0; x < dw; x++ {
			fx := (float64(x)+0.5)/float64(factor) - 0.5
			x0, wx := split(fx, sw)
			x1 := clampInt(x0+1, 0, sw-1)

			r00, g00, b00, a00 := img.At(src.Min.X+x0, src.Min.Y+y0).RGBA()
			r10, g10, b10, a10 := img.At(src.Min.X+x1, src.Min.Y+y0).RGBA()
			r01, g01, b01, a01 := img.At(src.Min.X+x0, src.Min.Y+y1).RGBA()
			r11, g11, b11, a11 := img.At(src.Min.X+x1, src.Min.Y+y1).RGBA()

			i := dst.PixOffset(x, y)
			dst.Pix[i+0] = mix(r00, r10, r01, r11, wx, wy)
			dst.Pix[i+1] = mix(g00, g10, g01, g11, wx, wy)
			dst.Pix[i+2] = mix(b00, b10, b01, b11, wx, wy)
			dst.Pix[i+3] = mix(a00, a10, a01, a11, wx, wy)
		}
	}
	return dst
}

// split turns a source coordinate into its lower neighbour and the weight of the upper.
func split(f float64, size int) (int, float64) {
	if f < 0 {
		return 0, 0
	}
	i := int(f)
	if i >= size-1 {
		return clampInt(size-1, 0, size-1), 0
	}
	return i, f - float64(i)
}

// mix blends four 16-bit channel samples down to one 8-bit result.
func mix(c00, c10, c01, c11 uint32, wx, wy float64) uint8 {
	top := float64(c00)*(1-wx) + float64(c10)*wx
	bottom := float64(c01)*(1-wx) + float64(c11)*wx
	v := (top*(1-wy) + bottom*wy) / 257 // 16-bit RGBA() range down to 8-bit
	switch {
	case v <= 0:
		return 0
	case v >= 255:
		return 255
	}
	return uint8(v + 0.5)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
