package screen

import (
	"image"
	"image/color"
	"testing"
)

func TestColorHistogramSamePalette(t *testing.T) {
	red, blue := color.RGBA{200, 40, 40, 255}, color.RGBA{40, 40, 200, 255}
	// Same two colours in equal amounts, different SPATIAL layout — histograms match.
	a := image.NewRGBA(image.Rect(0, 0, 10, 10))
	fill(a, 0, 0, 10, 5, red)
	fill(a, 0, 5, 10, 10, blue)
	b := image.NewRGBA(image.Rect(0, 0, 10, 10))
	fill(b, 0, 0, 5, 10, red)
	fill(b, 5, 0, 10, 10, blue)
	if s := histIntersection(colorHistogram(a), colorHistogram(b)); s < 0.95 {
		t.Fatalf("same palette should intersect ~1.0, got %v", s)
	}
	// A different palette barely intersects.
	green := image.NewRGBA(image.Rect(0, 0, 10, 10))
	fill(green, 0, 0, 10, 10, color.RGBA{40, 200, 40, 255})
	if s := histIntersection(colorHistogram(a), colorHistogram(green)); s > 0.6 {
		t.Fatalf("different palette should intersect low, got %v", s)
	}
}
