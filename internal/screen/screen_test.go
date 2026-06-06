package screen

import (
	"image"
	"image/color"
	"testing"
)

// fill paints a solid rectangle into img.
func fill(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func TestMatchFound(t *testing.T) {
	scr := image.NewRGBA(image.Rect(0, 0, 100, 80))
	fill(scr, 0, 0, 100, 80, color.RGBA{10, 10, 10, 255})
	// A 6x6 red square at (40,30).
	red := color.RGBA{200, 20, 20, 255}
	fill(scr, 40, 30, 46, 36, red)

	tmpl := image.NewRGBA(image.Rect(0, 0, 6, 6))
	fill(tmpl, 0, 0, 6, 6, red)

	m := match(scr, tmpl, 0)
	if !m.Found || m.X != 43 || m.Y != 33 { // center of 40..46 / 30..36
		t.Fatalf("got %+v, want center (43,33) found", m)
	}
}

func TestMatchNotFound(t *testing.T) {
	scr := image.NewRGBA(image.Rect(0, 0, 50, 50))
	fill(scr, 0, 0, 50, 50, color.RGBA{0, 0, 0, 255})
	tmpl := image.NewRGBA(image.Rect(0, 0, 4, 4))
	fill(tmpl, 0, 0, 4, 4, color.RGBA{255, 255, 255, 255})
	if m := match(scr, tmpl, 10); m.Found {
		t.Fatalf("expected not found, got %+v", m)
	}
}

func TestMatchTolerance(t *testing.T) {
	scr := image.NewRGBA(image.Rect(0, 0, 30, 30))
	fill(scr, 0, 0, 30, 30, color.RGBA{0, 0, 0, 255})
	// Square is slightly off from the template color.
	fill(scr, 10, 10, 14, 14, color.RGBA{200, 200, 200, 255})
	tmpl := image.NewRGBA(image.Rect(0, 0, 4, 4))
	fill(tmpl, 0, 0, 4, 4, color.RGBA{210, 195, 205, 255})

	if m := match(scr, tmpl, 4); m.Found {
		t.Fatalf("tolerance 4 should not match a 10-off pixel: %+v", m)
	}
	if m := match(scr, tmpl, 15); !m.Found || m.X != 12 || m.Y != 12 {
		t.Fatalf("tolerance 15 should match at center (12,12): %+v", m)
	}
}
