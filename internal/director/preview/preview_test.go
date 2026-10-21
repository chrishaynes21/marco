package preview_test

// A PICTURE OF WHAT A DEMONSTRATION WAS SHOWN.
//
// Nothing here decides anything, so what these tests protect is not a verdict — it is that the
// picture a person is shown is a picture of the thing it is captioned as. A crop taken from the
// wrong rectangle, or a thumbnail that shrank the text out of existence, is worse than no
// picture: it is a window telling somebody they are looking at their Audio tab when they are
// looking at a corner of the sky behind it.

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/preview"
)

// aFrame is a window-sized picture with one bright patch in it, so a crop can be checked by
// looking at what colour came back.
func aFrame(w, h int, patch image.Rectangle, mark color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 20, B: 20, A: 255})
		}
	}
	for y := patch.Min.Y; y < patch.Max.Y; y++ {
		for x := patch.Min.X; x < patch.Max.X; x++ {
			img.Set(x, y, mark)
		}
	}
	return img
}

// A thumbnail is the whole screen, smaller, and still the same shape.
func TestAScreenIsKeptWholeAndSmaller(t *testing.T) {
	frame := aFrame(1920, 1080, image.Rect(0, 0, 1, 1), color.RGBA{A: 255})
	thumb, err := preview.Thumbnail(frame)
	if err != nil {
		t.Fatal(err)
	}
	b := thumb.Bounds()
	if b.Dx() != preview.ScreenWide {
		t.Errorf("a thumbnail is %dpx wide, want %d", b.Dx(), preview.ScreenWide)
	}
	// THE SAME SHAPE, so a 16:9 game menu is not shown squashed.
	want := 1080 * preview.ScreenWide / 1920
	if b.Dy() != want {
		t.Errorf("a 16:9 frame came back %dx%d, want %dx%d",
			b.Dx(), b.Dy(), preview.ScreenWide, want)
	}
}

// AND A FRAME ALREADY SMALLER THAN THAT IS NOT BLOWN UP.
//
// Enlarging a picture past its own resolution shows a person nothing they could not already see,
// and costs four times the bytes to do it.
func TestASmallWindowIsNotEnlarged(t *testing.T) {
	frame := aFrame(300, 200, image.Rect(0, 0, 1, 1), color.RGBA{A: 255})
	thumb, err := preview.Thumbnail(frame)
	if err != nil {
		t.Fatal(err)
	}
	if got := thumb.Bounds().Dx(); got != 300 {
		t.Errorf("a 300px frame came back %dpx wide", got)
	}
}

// A CONTROL CROP IS THE CONTROL, WITH ITS SURROUNDINGS, AND NOT SOMEWHERE ELSE ON THE SCREEN.
//
// The region is normalised to the window and the frame is a capture of that window, so this is
// where the arithmetic can be wrong in a way nobody would notice: a crop of the wrong rectangle
// is still a picture, still renders, and is captioned "presses this".
//
// The patch is bright red in a dark frame, so what comes back says where the crop was taken.
func TestAControlIsCroppedFromWhereItWas(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	// THE PATCH, three-quarters of the way across and half way down.
	frame := aFrame(1000, 500, image.Rect(750, 250, 850, 290), red)
	where := []observe.Region{{X: 0.75, Y: 0.5, Width: 0.1, Height: 0.08}}

	crop, err := preview.Control(frame, where)
	if err != nil {
		t.Fatal(err)
	}
	// THE PATCH IS IN IT. Sampled at the middle, which is where the control is after a
	// margin is added evenly on each side.
	b := crop.Bounds()
	r, g, bl, _ := crop.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2).RGBA()
	if r < 0x8000 || g > 0x4000 || bl > 0x4000 {
		t.Errorf("the middle of the crop is not the control: r=%d g=%d b=%d",
			r>>8, g>>8, bl>>8)
	}
	// AND IT KEPT SOME OF THE SURROUNDINGS, because a label with nothing around it
	// could be any row of any list.
	if b.Dx() <= 100 {
		t.Errorf("a 100px control was cropped to %dpx, with no margin at all", b.Dx())
	}
}

// AND A CROP IS NEVER TAKEN FROM OUTSIDE THE FRAME.
//
// Not a tidiness point: `draw.Draw` over a rectangle that leaves the source is how a window that
// was resized since the demonstration, or a control at the very edge with a margin added,
// becomes a panic in the middle of saving somebody's recording.
func TestAControlAtTheEdgeIsStillCropped(t *testing.T) {
	for _, c := range []struct {
		what  string
		where observe.Region
	}{
		{"hard against the right edge", observe.Region{X: 0.9, Y: 0.4, Width: 0.1, Height: 0.1}},
		{"hard against the top left", observe.Region{X: 0, Y: 0, Width: 0.05, Height: 0.05}},
		{"wider than the window", observe.Region{X: 0, Y: 0.4, Width: 1.4, Height: 0.2}},
		{"starting past the window", observe.Region{X: 1.2, Y: 0.4, Width: 0.2, Height: 0.2}},
	} {
		frame := aFrame(800, 600, image.Rect(0, 0, 1, 1), color.RGBA{A: 255})
		crop, err := preview.Control(frame, []observe.Region{c.where})
		if err != nil {
			// A REGION ENTIRELY OUTSIDE THE FRAME HAS NOTHING TO CROP, and
			// saying so is right. What must not happen is a panic.
			continue
		}
		if crop.Bounds().Dx() <= 0 || crop.Bounds().Dy() <= 0 {
			t.Errorf("%s: the crop has no area", c.what)
		}
	}
}

// A CONTROL READ IN TWO PLACES IS ONE CONTROL, and the crop is the whole of it.
func TestTwoReadingsOfOneControlAreCroppedTogether(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	frame := aFrame(1000, 400, image.Rect(100, 100, 400, 200), red)
	both := []observe.Region{
		{X: 0.1, Y: 0.25, Width: 0.1, Height: 0.25},
		{X: 0.3, Y: 0.25, Width: 0.1, Height: 0.25},
	}
	crop, err := preview.Control(frame, both)
	if err != nil {
		t.Fatal(err)
	}
	// THE UNION IS 0.1..0.4 OF 1000 = 300px, plus a margin, scaled down to at most
	// ControlWide. What is checked is the SHAPE: a crop of one of the two readings
	// would be a third as wide relative to its height.
	b := crop.Bounds()
	ratio := float64(b.Dx()) / float64(b.Dy())
	if ratio < 1.5 {
		t.Errorf("the crop is %dx%d (ratio %.2f), which is one reading and not "+
			"both", b.Dx(), b.Dy(), ratio)
	}
}

// NO REGION AT ALL IS A REFUSAL AND NOT AN EMPTY PICTURE.
//
// A press the proposer found nothing under has no boundary — which is the same reason it can
// never become something to press — and a blank image captioned "presses this" would be a window
// claiming to show something it does not have.
func TestAControlWithNoRegionIsRefused(t *testing.T) {
	frame := aFrame(800, 600, image.Rect(0, 0, 1, 1), color.RGBA{A: 255})
	for _, where := range [][]observe.Region{
		nil,
		{},
		{{X: 0.5, Y: 0.5, Width: 0, Height: 0}},
	} {
		if _, err := preview.Control(frame, where); err == nil {
			t.Errorf("a control with regions %+v was cropped anyway", where)
		}
	}
	if _, err := preview.Control(nil, []observe.Region{{Width: 0.1, Height: 0.1}}); err == nil {
		t.Error("a crop was taken from no frame at all")
	}
	if _, err := preview.Thumbnail(nil); err == nil {
		t.Error("a thumbnail was made of no frame at all")
	}
}

// AND WHAT IS STORED IS A PNG THAT DECODES TO WHAT WENT IN.
func TestAPreviewIsStoredAsAPictureAnythingCanRead(t *testing.T) {
	frame := aFrame(640, 360, image.Rect(0, 0, 320, 360), color.RGBA{R: 255, A: 255})
	thumb, err := preview.Thumbnail(frame)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := preview.PNG(thumb)
	if err != nil {
		t.Fatal(err)
	}
	back, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("what was stored is not a readable PNG: %v", err)
	}
	if back.Bounds() != thumb.Bounds() {
		t.Errorf("stored %v and read back %v", thumb.Bounds(), back.Bounds())
	}
	// THE LEFT HALF IS STILL RED, so what was encoded is the picture and not a blank
	// of the right size.
	r, _, _, _ := back.At(back.Bounds().Dx()/4, back.Bounds().Dy()/2).RGBA()
	if r < 0x8000 {
		t.Errorf("the stored picture lost its content: r=%d", r>>8)
	}
	if _, err := preview.PNG(nil); err == nil {
		t.Error("nothing at all was encoded as a picture")
	}
}

// SHRINKING AVERAGES RATHER THAN SAMPLES, which is what keeps text legible.
//
// A one-pixel line scaled by picking one source pixel per destination pixel vanishes outright:
// the word is still there and a person cannot read it, which defeats the entire purpose of
// keeping a picture. An average leaves it grey instead of gone. Measured here on a frame of
// alternating lines, where sampling would return one flat colour and averaging returns the mix.
func TestShrinkingKeepsWhatWasThereRatherThanSamplingPastIt(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1280, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 1280; x++ {
			// EVERY OTHER COLUMN WHITE, which is a one-pixel stroke.
			shade := uint8(0)
			if x%2 == 0 {
				shade = 255
			}
			img.Set(x, y, color.RGBA{R: shade, G: shade, B: shade, A: 255})
		}
	}
	thumb, err := preview.Thumbnail(img)
	if err != nil {
		t.Fatal(err)
	}
	// SAMPLED INSIDE IT. A 1280x16 frame scaled to 640 wide is 8 tall, and reading past
	// that returns transparent black — which looks exactly like the defect this is
	// testing for.
	b := thumb.Bounds()
	r, _, _, _ := thumb.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2).RGBA()
	grey := r >> 8
	if grey < 0x40 || grey > 0xc0 {
		t.Errorf("a half-white frame shrank to %d, which is a sample and not an "+
			"average — one-pixel strokes are being dropped", grey)
	}
}
