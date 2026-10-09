package vision_test

import (
	"context"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The detector's regions reach a session as geometry. These two numbers are the only pixel
// evidence that travels with them, and they are what an interface hypothesis downstream is
// built from: whether one row of a list looks unlike its neighbours cannot be asked of a
// rectangle.
//
// Measured HERE because this is the last place the pixels exist. If this stops happening,
// everything downstream keeps working and quietly believes nothing — which is why it is
// tested at the provider rather than assumed from the fact that the call is written.

// paintedCapture returns a frame with a known bright band in it.
type paintedCapture struct{ band image.Rectangle }

func (p *paintedCapture) CaptureWindow(context.Context, directorapi.Window) (capture.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := range 300 {
		for x := range 400 {
			img.SetRGBA(x, y, color.RGBA{30, 30, 30, 255})
		}
	}
	for y := p.band.Min.Y; y < p.band.Max.Y; y++ {
		for x := p.band.Min.X; x < p.band.Max.X; x++ {
			img.SetRGBA(x, y, color.RGBA{220, 220, 220, 255})
		}
	}
	return capture.Image{
		Image:      img,
		Bounds:     directorapi.Rect{Width: 400, Height: 300},
		Scale:      1,
		CapturedAt: time.Now(),
	}, nil
}

// oneBox reports a single detection over whatever rectangle it is given.
type oneBox struct{ at image.Rectangle }

func (d *oneBox) Detect(context.Context, vision.Input) ([]vision.Detection, error) {
	return []vision.Detection{{Class: "button", Bounds: d.at, Confidence: 0.9}}, nil
}
func (d *oneBox) Model() string { return "onebox" }

func TestADetectedRegionCarriesHowItIsShaded(t *testing.T) {
	band := image.Rect(40, 100, 360, 140)
	win := &countingWindow{
		window: directorapi.Window{ID: "hwnd:1", Application: "testapp",
			Bounds: directorapi.Rect{Width: 400, Height: 300}},
		ok: true,
	}
	p := vision.New(&oneBox{at: band}, &paintedCapture{band: band}, win.active)

	out := p.ObserveTargeted(context.Background(), observation.Request{
		Sources: []directorapi.ObservationSource{directorapi.SourceVision},
	})
	var shaded int
	for _, o := range out.Observations {
		el, ok := o.(observation.Element)
		if !ok {
			continue
		}
		mean, okM := el.Raw.Attributes["shade_mean"].(float64)
		top, okT := el.Raw.Attributes["shade_top"].(float64)
		if !okM || !okT {
			continue
		}
		shaded++
		// The band was painted at 220 on a ground of 30. Exact values are the
		// measurement's business; what this asserts is that the numbers describe THAT
		// rectangle rather than the frame, a constant, or nothing.
		if mean < 150 {
			t.Errorf("a region over a bright band reports mean %.0f", mean)
		}
		if top < mean {
			t.Errorf("the brightest quarter (%.0f) cannot be dimmer than the whole (%.0f)",
				top, mean)
		}
	}
	if shaded == 0 {
		t.Fatalf("no detected region carried its shading; %d observation(s) came back, and "+
			"every interface hypothesis downstream is built from these two numbers",
			len(out.Observations))
	}
}

func TestShadingDescribesTheRegionAndNotTheFrame(t *testing.T) {
	// The same frame, a detection over the DARK part of it. If the measurement were
	// taken over the whole image, or defaulted, both boxes would report the same thing
	// and nothing downstream could tell one row from another.
	band := image.Rect(40, 100, 360, 140)
	win := &countingWindow{
		window: directorapi.Window{ID: "hwnd:1", Application: "testapp",
			Bounds: directorapi.Rect{Width: 400, Height: 300}},
		ok: true,
	}
	read := func(at image.Rectangle) (float64, bool) {
		p := vision.New(&oneBox{at: at}, &paintedCapture{band: band}, win.active)
		out := p.ObserveTargeted(context.Background(), observation.Request{
			Sources: []directorapi.ObservationSource{directorapi.SourceVision},
		})
		for _, o := range out.Observations {
			if el, ok := o.(observation.Element); ok {
				if m, ok := el.Raw.Attributes["shade_mean"].(float64); ok {
					return m, true
				}
			}
		}
		return 0, false
	}
	bright, okB := read(band)
	dark, okD := read(image.Rect(40, 200, 360, 240))
	if !okB || !okD {
		t.Skip("the provider returned no shaded element for one of the boxes")
	}
	if bright <= dark {
		t.Errorf("a box over the bright band reports %.0f and one over the dark ground "+
			"%.0f; the measurement is not describing its own region", bright, dark)
	}
}
