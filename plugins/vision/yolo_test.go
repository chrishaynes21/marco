package main

import (
	"image"
	"image/color"
	"testing"
)

func TestLetterboxPreservesAspectAndPads(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			img.SetRGBA(x, y, color.RGBA{10, 20, 30, 255})
		}
	}
	out, scale, padX, padY := letterbox(img, 64)
	if out.Bounds().Dx() != 64 || out.Bounds().Dy() != 64 {
		t.Fatalf("letterbox size = %v, want 64x64", out.Bounds())
	}
	if scale != 0.32 { // 64/200
		t.Fatalf("scale = %v, want 0.32", scale)
	}
	// Wider than tall → full width, vertically centred: padX 0, padY (64-32)/2 = 16.
	if padX != 0 || padY != 16 {
		t.Fatalf("pad = (%d,%d), want (0,16)", padX, padY)
	}
	// A pixel in the padded band is grey (114); a pixel in the image band is the fill.
	if c := out.RGBAAt(0, 0); c.R != 114 {
		t.Fatalf("pad pixel = %v, want grey 114", c)
	}
	if c := out.RGBAAt(0, 16); c.R != 10 {
		t.Fatalf("image pixel = %v, want the fill", c)
	}
}

func TestDecodeThresholdsAndPicksClass(t *testing.T) {
	// nc=2, n=2, channel-major: data[a*n + i]. Box 0 is a confident class-0; box 1 is
	// below threshold and dropped.
	n, nc := 2, 2
	data := make([]float32, (4+nc)*n)
	set := func(a, i int, v float32) { data[a*n+i] = v }
	set(0, 0, 100)
	set(1, 0, 80)
	set(2, 0, 40)
	set(3, 0, 20) // box0 cx,cy,w,h
	set(4, 0, 0.9)
	set(5, 0, 0.1) // box0 class scores → class 0
	set(4, 1, 0.2)
	set(5, 1, 0.3) // box1 max 0.3, below 0.5
	boxes := decode(data, []int64{1, int64(4 + nc), int64(n)}, 0.5)
	if len(boxes) != 1 {
		t.Fatalf("decoded %d boxes, want 1", len(boxes))
	}
	if boxes[0].class != 0 || boxes[0].score != 0.9 {
		t.Fatalf("box = %+v, want class 0 score 0.9", boxes[0])
	}
	if boxes[0].cx != 100 || boxes[0].w != 40 {
		t.Fatalf("box geom = %+v", boxes[0])
	}
}

func TestNMSSuppressesSameClassOverlap(t *testing.T) {
	a := rawBox{cx: 100, cy: 100, w: 40, h: 40, class: 0, score: 0.9}
	dup := rawBox{cx: 105, cy: 102, w: 40, h: 40, class: 0, score: 0.6}   // overlaps a, same class
	other := rawBox{cx: 102, cy: 100, w: 40, h: 40, class: 1, score: 0.7} // overlaps but class 1
	keep := nms([]rawBox{a, dup, other}, 0.45)
	if len(keep) != 2 {
		t.Fatalf("nms kept %d, want 2 (dup suppressed, other class kept)", len(keep))
	}
	// Highest score first; the class-0 duplicate must be gone.
	for _, b := range keep {
		if b.class == 0 && b.score == 0.6 {
			t.Fatal("nms failed to suppress the overlapping same-class box")
		}
	}
}

func TestToElementsUndoesLetterbox(t *testing.T) {
	// A box centred at (100,116) in letterboxed space with pad (0,16) and scale 0.5 maps
	// to centre (200,200) in the original image.
	boxes := []rawBox{{cx: 100, cy: 116, w: 20, h: 20, class: 0, score: 0.8}}
	els := toElements(boxes, 0.5, 0, 16, []string{"button"}, 1000, 1000)
	if len(els) != 1 {
		t.Fatalf("got %d elements, want 1", len(els))
	}
	cx, cy := els[0].Center()
	if cx != 200 || cy != 200 {
		t.Fatalf("centre = (%d,%d), want (200,200)", cx, cy)
	}
	if els[0].Label != "button" {
		t.Fatalf("label = %q, want button", els[0].Label)
	}
}

func TestToElementsClampsAndDropsEmpty(t *testing.T) {
	// A box partly off the top-left clamps into the image; a degenerate box is dropped.
	boxes := []rawBox{
		{cx: 0, cy: 0, w: 40, h: 40, class: 0, score: 0.8},       // clamps to a corner box
		{cx: -100, cy: -100, w: 10, h: 10, class: 0, score: 0.8}, // fully outside → empty → dropped
	}
	els := toElements(boxes, 1, 0, 0, nil, 500, 500)
	for _, e := range els {
		if e.Box.Dx() <= 0 || e.Box.Dy() <= 0 {
			t.Fatalf("kept a degenerate box: %+v", e)
		}
		if e.Box.Min.X < 0 || e.Box.Min.Y < 0 {
			t.Fatalf("box not clamped: %+v", e)
		}
	}
}
