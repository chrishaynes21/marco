package main

import (
	"image"
	"os"
	"strconv"

	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

// maxUpscaledLong caps the long side (px) of the upscaled image, so a full-screen
// run-time capture isn't tripled into a huge, slow OCR. A small learn-time crop stays
// under it and keeps the full upscale.
const maxUpscaledLong = 2600

// preprocess conditions a captured crop for OCR. Button labels are often small, anti-
// aliased, and low-contrast — light text on a light gradient, a glow, a theme colour —
// which tesseract misses at native resolution (it wants dark text on white, ~30px cap
// height). We:
//
//  1. grayscale by luminance,
//  2. bilinearly UPSCALE (default 3x) so small text reaches a legible size,
//  3. global Otsu THRESHOLD to clean, FILLED black/white, background oriented to white.
//
// One pass suffices: tesseract auto-detects inverted text, so a HIGHLIGHTED button (light
// text on a dark fill) reads in the same pass — provided the engine uses PSM 3, which
// (unlike sparse PSM 11) reads a solid-filled button. (Otsu gives filled strokes; a local
// adaptive threshold instead leaves thick game-font strokes hollow, unreadable.)
//
// Returns the binarized image and the scale factor applied (so the caller can map
// coordinates between the original and processed images). $MARCO_OCR_SCALE overrides the
// factor (1 disables upscaling); $MARCO_OCR_RAW=1 skips preprocessing entirely (debug /
// A-B), returning the image and scale 1.
func preprocess(img *image.RGBA) (*image.RGBA, int) {
	if os.Getenv("MARCO_OCR_RAW") == "1" {
		return img, 1
	}
	up, ow, oh, scale, ok := grayUpscale(img)
	if !ok {
		return img, 1
	}
	dst := otsuBinary(up, ow, oh, false)
	dumpDebug(dst)
	return dst, scale
}

// grayUpscale converts img to a grayscale (0..255) buffer bilinearly upscaled by the
// configured factor, returning the buffer, its dimensions, the scale, and ok=false for a
// degenerate image.
func grayUpscale(img *image.RGBA) (up []float64, ow, oh, scale int, ok bool) {
	scale = 3
	if v := os.Getenv("MARCO_OCR_SCALE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			scale = n
		}
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil, 0, 0, 1, false
	}
	// Adaptive cap: upscaling matters for a SMALL crop (a learn-time button), but a
	// full-screen run-time capture is already legible — tripling it would be slow and
	// pointless. Shrink the factor so the long side stays under maxUpscaledLong.
	for scale > 1 && maxInt(w, h)*scale > maxUpscaledLong {
		scale--
	}
	gray := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			gray[y*w+x] = (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)) / 257
		}
	}
	ow, oh = w*scale, h*scale
	up = make([]float64, ow*oh)
	for oy := 0; oy < oh; oy++ {
		fy := (float64(oy)+0.5)/float64(scale) - 0.5
		y0, wy := lerpIndex(fy, h)
		for ox := 0; ox < ow; ox++ {
			fx := (float64(ox)+0.5)/float64(scale) - 0.5
			x0, wx := lerpIndex(fx, w)
			x1, y1 := minInt(x0+1, w-1), minInt(y0+1, h-1)
			top := gray[y0*w+x0]*(1-wx) + gray[y0*w+x1]*wx
			bot := gray[y1*w+x0]*(1-wx) + gray[y1*w+x1]*wx
			up[oy*ow+ox] = top*(1-wy) + bot*wy
		}
	}
	return up, ow, oh, scale, true
}

// otsuBinary thresholds up by Otsu's between-class variance into clean black/white. With
// invert=false the majority class (background) is white and text is black; invert=true
// swaps them, so a region whose local polarity is opposite the crop's (a highlighted
// button) comes out readable in one pass or the other.
func otsuBinary(up []float64, ow, oh int, invert bool) *image.RGBA {
	thr := otsu(up)
	dark := 0
	for _, v := range up {
		if v <= thr {
			dark++
		}
	}
	darkIsBackground := dark*2 > len(up)
	dst := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for i, v := range up {
		black := (v <= thr) != darkIsBackground // minority class = text = black
		if invert {
			black = !black
		}
		c := uint8(255)
		if black {
			c = 0
		}
		o := i * 4
		dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2], dst.Pix[o+3] = c, c, c, 255
	}
	return dst
}

// dumpDebug writes the processed image to $MARCO_OCR_DUMP (a PNG path) when set, so the
// binarization can be inspected. No-op otherwise.
func dumpDebug(img *image.RGBA) {
	path := os.Getenv("MARCO_OCR_DUMP")
	if path == "" {
		return
	}
	if data, err := screen.EncodePNG(img); err == nil {
		_ = os.WriteFile(path, data, 0o644)
	}
}

// lerpIndex maps a fractional source coordinate to a clamped base index and the
// interpolation weight toward the next index.
func lerpIndex(f float64, n int) (idx int, weight float64) {
	if f <= 0 {
		return 0, 0
	}
	if f >= float64(n-1) {
		return n - 1, 0
	}
	i := int(f)
	return i, f - float64(i)
}

// otsu returns the grayscale threshold that maximises between-class variance — the
// classic two-mode split of text from background.
func otsu(vals []float64) float64 {
	var hist [256]int
	for _, v := range vals {
		i := int(v)
		if i < 0 {
			i = 0
		} else if i > 255 {
			i = 255
		}
		hist[i]++
	}
	total := len(vals)
	var sum float64
	for i := 0; i < 256; i++ {
		sum += float64(i) * float64(hist[i])
	}
	var sumB, wB, maxVar float64
	thr := 128.0
	for i := 0; i < 256; i++ {
		wB += float64(hist[i])
		if wB == 0 {
			continue
		}
		wF := float64(total) - wB
		if wF == 0 {
			break
		}
		sumB += float64(i) * float64(hist[i])
		mB := sumB / wB
		mF := (sum - sumB) / wF
		if v := wB * wF * (mB - mF) * (mB - mF); v > maxVar {
			maxVar, thr = v, float64(i)
		}
	}
	return thr
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
