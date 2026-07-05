package main

import (
	"image"
	"image/color"
	"testing"
)

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestPreprocessUpscalesSmallCrop(t *testing.T) {
	// A small teach-time crop keeps the full upscale (default 3x).
	in := solid(200, 60, color.RGBA{200, 200, 200, 255})
	out, scale := preprocess(in)
	if scale != 3 {
		t.Fatalf("scale = %d, want 3 for a small crop", scale)
	}
	if out.Bounds().Dx() != 600 {
		t.Fatalf("upscaled width = %d, want 600", out.Bounds().Dx())
	}
}

func TestPreprocessCapsLargeCapture(t *testing.T) {
	// A full-screen-sized capture is NOT tripled — the long side stays under the cap, so
	// run-time OCR doesn't blow up. 1920*3 would be 5760 (> 2600), so scale drops to 1.
	in := solid(1920, 1080, color.RGBA{40, 40, 40, 255})
	_, scale := preprocess(in)
	if scale != 1 {
		t.Fatalf("scale = %d, want 1 for a 1920px capture (cap %d)", scale, maxUpscaledLong)
	}
}

func TestOtsuBinarySeparatesTwoTone(t *testing.T) {
	// Dark text region on a light field: Otsu should split them, with the majority
	// (light background) rendered white and the minority (dark "text") black.
	w, h := 40, 20
	up := make([]float64, w*h)
	for i := range up {
		up[i] = 230 // light background
	}
	for y := 5; y < 15; y++ {
		for x := 4; x < 12; x++ {
			up[y*w+x] = 20 // a dark blob (the "text")
		}
	}
	out := otsuBinary(up, w, h, false)
	bg := out.RGBAAt(0, 0)
	ink := out.RGBAAt(6, 9)
	if bg.R != 255 {
		t.Fatalf("background = %v, want white", bg)
	}
	if ink.R != 0 {
		t.Fatalf("ink = %v, want black", ink)
	}
}
