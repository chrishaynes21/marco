package main

// The frame handed to OCR, and the deadline that decides its size.
//
// `readScale` is not a tuning constant: at full size the reading is ten looks behind a window of
// eight and the two menus merge as they always did, and at a third tesseract starts inventing
// words until the two screens agree. These hold the arithmetic that makes half size half size,
// because a resampler that quietly dropped strokes would pass every other test here while
// costing the discrimination the whole path exists for.

import (
	"image"
	"image/color"
	"testing"
)

// A HALVED FRAME IS HALF AS WIDE AND HALF AS TALL, and the pixels are averages.
func TestAFrameIsHalvedByAveragingEachBlock(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	// A 2x2 block of known values: 0, 100, 100, 200 averages to 100.
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			v := uint8(0)
			switch {
			case x%2 == 1 && y%2 == 1:
				v = 200
			case x%2 == 1 || y%2 == 1:
				v = 100
			}
			src.Set(x, y, color.RGBA{R: v, G: v, B: v, A: 255})
		}
	}
	got := shrink(src, 2)
	if b := got.Bounds(); b.Dx() != 2 || b.Dy() != 2 {
		t.Fatalf("a halved 4x4 frame is %dx%d", b.Dx(), b.Dy())
	}
	r, g, bl, a := got.At(0, 0).RGBA()
	for name, v := range map[string]uint32{"r": r, "g": g, "b": bl} {
		if ch := v >> 8; ch != 100 {
			t.Errorf("%s is %d, want the block average 100", name, ch)
		}
	}
	if a>>8 != 255 {
		t.Errorf("alpha is %d", a>>8)
	}
}

// AND A STROKE ONE PIXEL WIDE SURVIVES AS GREY RATHER THAN VANISHING.
//
// The whole reason this averages instead of sampling. Nearest-neighbour deletes every other
// column, so a one-pixel glyph stroke is there or gone depending on its parity — and tesseract
// reads a letter with a missing stem as a different letter or as nothing.
func TestAThinStrokeSurvivesTheHalvingAsGrey(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 8, 2))
	for x := 0; x < 8; x++ {
		for y := 0; y < 2; y++ {
			src.Set(x, y, color.RGBA{A: 255})
		}
	}
	// One white column at an ODD index, which a sampler taking even columns would drop.
	for y := 0; y < 2; y++ {
		src.Set(3, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	}
	got := shrink(src, 2)
	r, _, _, _ := got.At(1, 0).RGBA()
	if r>>8 == 0 {
		t.Error("a one-pixel stroke at an odd column disappeared; the reduction is " +
			"sampling rather than averaging")
	}
	if r>>8 == 255 {
		t.Errorf("the stroke came back fully white (%d); it covers half the block "+
			"and should be grey", r>>8)
	}
}

// AND A FACTOR OF ONE, OR A FRAME SMALLER THAN THE FACTOR, IS LEFT ALONE.
//
// The second is not hypothetical: a window being restored can capture a few pixels wide, and
// returning an empty image would make OCR answer nothing on a frame that had something in it.
func TestAFrameThatCannotBeHalvedIsReadAsItIs(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 1))
	for _, n := range []int{0, 1, 4} {
		got := shrink(src, n)
		if got != image.Image(src) {
			t.Errorf("shrink by %d returned a different image for a 3x1 frame", n)
		}
	}
	if shrink(nil, 2) != nil {
		t.Error("shrink of nothing returned something")
	}
}

// AND THE SCALE IS THE ONE THAT WAS MEASURED.
//
// Held explicitly because the two failures either side of it are opposite and both silent: one
// makes every reading arrive too late to be admitted, the other makes the readings arrive in
// time and say the wrong thing.
func TestTheReadingScaleIsTheMeasuredOne(t *testing.T) {
	if readScale != 2 {
		t.Errorf("readScale is %d. At 1 the reading is ten looks behind a window of "+
			"eight; at 3 the two screens agree at kept 0.36 against a threshold "+
			"of 0.35. Changing it needs the measurement in Experiment-043 taken "+
			"again", readScale)
	}
}
