package main

// THE PICTURES, FILED UNDER THE LABELS THE ARTIFACT GAVE THINGS.
//
// This is the part of keeping previews that can be silently wrong. The session knows screens as
// `visual_7` and controls as target identities; the saved artifact knows them as `screen_a` and
// `control_1`, and the mapping between the two is made inside `SaveWith` and handed back to
// nobody. `keepPreviews` reconstructs it from the fact that `a.Steps[i]` is `draft.Steps[i]` —
// and if that ever stopped being true, every picture would still be written, would still render,
// and would be captioned as something else.
//
// So what is asserted here is which label each picture landed under, by making each frame
// identifiable: the thumbnail of one screen is a different colour from the thumbnail of the
// other, and the control's crop is taken from a patch only that control covers.

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aScreenLike is a frame of one colour, with one patch of another in a known place.
func aScreenLike(w, h int, shade color.RGBA, patch image.Rectangle,
	mark color.RGBA) image.Image {

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, shade)
		}
	}
	for y := patch.Min.Y; y < patch.Max.Y; y++ {
		for x := patch.Min.X; x < patch.Max.X; x++ {
			img.Set(x, y, mark)
		}
	}
	return img
}

// middleOf is the colour at the centre of a stored picture.
func middleOf(t *testing.T, raw []byte) color.RGBA {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("what was stored is not a readable picture: %v", err)
	}
	b := img.Bounds()
	r, g, bl, a := img.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2).RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bl >> 8),
		A: uint8(a >> 8)}
}

func TestEachPictureIsFiledUnderTheLabelOfWhatItIsAPictureOf(t *testing.T) {
	store, err := demo.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// ── A DEMONSTRATION OF TWO PRESSES OVER TWO SCREENS ──
	//
	// `visual_1 -> visual_2` and then `visual_2 -> visual_1`, which is what clicking
	// two tabs in one bar looks like to the fold. The labels the artifact will give
	// those are `screen_a` and `screen_b`, in walk order.
	const (
		audio    = observe.VisualTargetID("t-audio")
		graphics = observe.VisualTargetID("t-graphics")
		first    = observe.VisualScreenID("visual_1")
		second   = observe.VisualScreenID("visual_2")
	)
	audioAt := observe.Region{X: 0.1, Y: 0.1, Width: 0.1, Height: 0.1}
	graphicsAt := observe.Region{X: 0.7, Y: 0.1, Width: 0.1, Height: 0.1}
	draft := workflow.Draft{Name: "two tabs", Steps: []workflow.Step{
		{From: first, To: second, Cause: observe.CauseClick, Target: audio,
			Seen: 1, Readings: []observe.Region{audioAt},
			Outcome: []string{"volume", "master", "voice"}},
		{From: second, To: first, Cause: observe.CauseClick, Target: graphics,
			Seen: 1, Readings: []observe.Region{graphicsAt},
			Outcome: []string{"preset", "shadows", "texture"}},
	}}
	art, err := workflowstore.SaveWith(draft,
		map[observe.VisualScreenID]observe.VisualArrangement{},
		map[observe.VisualScreenID]observe.ScreenText{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}

	// ── AND THE FRAMES THE SESSION KEPT, each one identifiable ──
	//
	// The first screen is dark red and the second dark green, so a picture filed under
	// the wrong label says so in its own colour.
	//
	// AND THE THUMBNAIL OF A SCREEN IS A DIFFERENT PICTURE FROM THE FRAME OF A PRESS ON
	// IT. That is true in a real session — the thumbnail is the first settled look of a
	// screen and the press frame is the look the press arrived on, which is a moment
	// later and at full resolution — and a test that used one image for both cannot tell
	// a crop taken from the wrong one. So each screen is kept twice: a plain frame
	// first, which becomes the thumbnail, and then the look carrying the press, which is
	// the only frame with the bright patch in it. A crop that came from the thumbnail is
	// the colour of the screen and not of the control.
	dullRed := color.RGBA{R: 90, A: 255}
	dullGreen := color.RGBA{G: 90, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	nowhere := image.Rect(0, 0, 0, 0)
	res := visualResult{
		screens: map[observe.VisualScreenID]image.Image{},
		pressed: map[int]image.Image{},
	}
	res.keep(first, 1, aScreenLike(1000, 600, dullRed, nowhere, white), false)
	res.keep(second, 7, aScreenLike(1000, 600, dullGreen, nowhere, white), false)
	res.keep(first, 3, aScreenLike(1000, 600, dullRed,
		image.Rect(100, 60, 200, 120), white), true)
	res.keep(second, 9, aScreenLike(1000, 600, dullGreen,
		image.Rect(700, 60, 800, 120), white), true)

	// EVERY PRESS, WITH THE LOOK IT ARRIVED ON, exactly as the correlator reports it.
	ev := observe.ClickEvidence{Observations: []observe.ClickObservation{
		{Look: 3, Screen: first, Target: audio},
		{Look: 9, Screen: second, Target: graphics},
	}}

	kept, why := keepPreviews(store, art, draft, res, ev)
	if why != "" {
		t.Fatalf("a picture could not be kept: %s", why)
	}
	// TWO SCREENS AND TWO CONTROLS.
	if kept != 4 {
		t.Errorf("%d picture(s) kept, want 4 (two screens, two controls)", kept)
	}

	// ── THE SCREENS, each the colour of the screen it is ──
	for _, c := range []struct {
		label string
		want  color.RGBA
		what  string
	}{
		{"screen_a", dullRed, "the screen the demonstration started on"},
		{"screen_b", dullGreen, "the screen the first press led to"},
	} {
		raw, ok := store.Preview(art.Name, c.label)
		if !ok {
			t.Errorf("%s has no picture", c.label)
			continue
		}
		got := middleOf(t, raw)
		if got.R != c.want.R || got.G != c.want.G {
			t.Errorf("%s is a picture of the wrong screen: it is %+v and %s "+
				"is %+v", c.label, got, c.what, c.want)
		}
	}

	// ── AND THE CONTROLS, each cropped from its own patch ──
	//
	// Both crops are white in the middle, so what tells them apart is the colour
	// AROUND the patch: the audio tab sat on the red screen and the graphics tab on
	// the green one. A crop taken from the wrong frame, or at the wrong rectangle,
	// shows it here.
	for _, c := range []struct {
		label  string
		around color.RGBA
		what   string
	}{
		{"control_1", dullRed, "the audio tab, on the first screen"},
		{"control_2", dullGreen, "the graphics tab, on the second screen"},
	} {
		raw, ok := store.Preview(art.Name, c.label)
		if !ok {
			t.Errorf("%s has no picture", c.label)
			continue
		}
		img, derr := png.Decode(bytes.NewReader(raw))
		if derr != nil {
			t.Fatalf("%s is not a readable picture: %v", c.label, derr)
		}
		// THE MIDDLE IS THE CONTROL — which only the frame of the PRESS has in
		// it, so this is also what says the crop came from that frame and not
		// from the screen thumbnail.
		mid := middleOf(t, raw)
		if mid.R < 200 || mid.G < 200 || mid.B < 200 {
			t.Errorf("%s is not centred on %s: its middle is %+v — which is "+
				"the colour of the screen, so the crop was taken from the "+
				"thumbnail rather than from the look the press arrived on",
				c.label, c.what, mid)
		}
		// AND THE CORNER IS THE SCREEN IT WAS ON, which is the margin.
		b := img.Bounds()
		r, g, _, _ := img.At(b.Min.X, b.Min.Y).RGBA()
		if uint8(r>>8) != c.around.R || uint8(g>>8) != c.around.G {
			t.Errorf("%s was cropped from the wrong frame: around it is "+
				"r=%d g=%d, and %s sits on r=%d g=%d",
				c.label, r>>8, g>>8, c.what, c.around.R, c.around.G)
		}
	}
}

// A PRESS THE PROPOSER FOUND NOTHING UNDER GETS NO PICTURE, and that is not a failure.
//
// It has no region, which is the same reason it can never become something to press. A blank
// image captioned "presses this" would be the window claiming to show something it does not
// have.
func TestAPressOnNothingIsNotGivenAPicture(t *testing.T) {
	store, err := demo.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const blind = observe.VisualTargetID("t-nothing")
	draft := workflow.Draft{Name: "a press on nothing", Steps: []workflow.Step{
		// A STEP WITH A TARGET AND NO READINGS, which is what a press inside
		// nothing the proposer found produces.
		{From: "visual_1", To: "visual_2", Cause: observe.CauseClick,
			Target: blind, Seen: 1},
	}}
	art, err := workflowstore.SaveWith(draft,
		map[observe.VisualScreenID]observe.VisualArrangement{},
		map[observe.VisualScreenID]observe.ScreenText{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	res := visualResult{}
	res.keep("visual_1", 2, aScreenLike(800, 400, color.RGBA{B: 90, A: 255},
		image.Rect(0, 0, 1, 1), color.RGBA{A: 255}), true)
	ev := observe.ClickEvidence{Observations: []observe.ClickObservation{
		{Look: 2, Screen: "visual_1", Target: blind},
	}}

	kept, why := keepPreviews(store, art, draft, res, ev)
	if why != "" {
		t.Fatalf("keeping the pictures reported %s", why)
	}
	// THE SCREEN, YES. The control, no.
	if kept != 1 {
		t.Errorf("%d picture(s) kept, want 1 — the screen and not the control", kept)
	}
	if labels := store.Previews(art.Name); len(labels) != 1 ||
		labels[0] != "screen_a" {
		t.Errorf("what was kept is %v", labels)
	}
}

// AND A SESSION THAT KEPT NO FRAMES WRITES NOTHING, rather than writing blanks.
//
// This is the ordinary case for every recording made without `--previews`, and for one whose
// window was never in front: the guard in the loop keeps nothing, and this must then keep
// nothing rather than producing a picture of an empty image.
func TestASessionWithNoFramesKeepsNoPictures(t *testing.T) {
	store, err := demo.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	draft := workflow.Draft{Name: "nothing was kept", Steps: []workflow.Step{
		{From: "visual_1", To: "visual_2", Cause: observe.CauseClick,
			Target: "t-x", Seen: 1,
			Readings: []observe.Region{{X: 0.1, Y: 0.1, Width: 0.1, Height: 0.1}}},
	}}
	art, err := workflowstore.SaveWith(draft,
		map[observe.VisualScreenID]observe.VisualArrangement{},
		map[observe.VisualScreenID]observe.ScreenText{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	kept, why := keepPreviews(store, art, draft, visualResult{},
		observe.ClickEvidence{})
	if kept != 0 || why != "" {
		t.Errorf("%d picture(s) kept from no frames at all (%s)", kept, why)
	}
	if labels := store.Previews(art.Name); len(labels) != 0 {
		t.Errorf("pictures were written with nothing to picture: %v", labels)
	}
}

// AND THE SESSION ONLY EVER HOLDS WHAT IT IS BOUNDED TO HOLD.
//
// A full frame is eight megabytes of RGBA. A recording of a long tour through thirty screens
// that held one of each would be a quarter of a gigabyte in the recorder's memory, which is how
// a feature for looking at pictures becomes a reason a demonstration fails.
func TestASessionHoldsAtMostWhatItIsBoundedTo(t *testing.T) {
	var res visualResult
	frame := aScreenLike(200, 100, color.RGBA{A: 255}, image.Rect(0, 0, 1, 1),
		color.RGBA{R: 255, A: 255})
	for i := 0; i < previewScreens*3; i++ {
		res.keep(observe.VisualScreenID("visual_"+string(rune('a'+i))), i,
			frame, true)
	}
	if len(res.screens) != previewScreens {
		t.Errorf("%d screen thumbnail(s) held, bounded at %d",
			len(res.screens), previewScreens)
	}
	if len(res.pressed) != previewPresses {
		t.Errorf("%d press frame(s) held, bounded at %d",
			len(res.pressed), previewPresses)
	}
	// AND THE FIRST THUMBNAIL OF A SCREEN STANDS, not the last: the first settled look
	// of a screen is the screen as it was arrived at, and the last is whatever it
	// looked like half way into the next transition.
	was := res.screens["visual_a"]
	res.keep("visual_a", 99, aScreenLike(200, 100, color.RGBA{G: 255, A: 255},
		image.Rect(0, 0, 1, 1), color.RGBA{A: 255}), false)
	if res.screens["visual_a"] != was {
		t.Error("a later look replaced the picture of a screen already kept")
	}
}
