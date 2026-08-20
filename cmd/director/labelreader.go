package main

import (
	"context"
	"image"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
)

// Letting the detector borrow the OCR engine to read its own boxes.
//
// The two providers must not know about each other — they are siblings, separately
// optional, and a build with one and not the other is the ordinary case. So the coupling
// lives HERE, at the composition root, which is the only place that knows both exist. It is
// an adapter and nothing more: no policy, no thresholds, no decisions.
//
// Why it is worth wiring at all is measured rather than assumed. On a live Rocket League
// pause menu, OCR over the whole frame read one string of the twelve a person sees, because
// a global binarisation threshold is swamped by a lit 3D scene. The same engine pointed at
// the four boxes the detector found read all four labels exactly. Structure is what makes
// the reading trustworthy — see the package doc on providers/vision/label.go.

// labelReader adapts an ocr.Engine to vision.LabelReader.
type labelReader struct {
	engine ocr.Engine
}

var _ vision.LabelReader = (*labelReader)(nil)

// newLabelReader returns a reader over the engine, or nil when there is no engine.
//
// nil rather than a reader that always fails: the provider treats a nil Reader as "labels
// were never asked for" and a failing one as "this control could not be read", and those
// are different things that the counters report separately.
func newLabelReader(engine ocr.Engine) vision.LabelReader {
	if engine == nil {
		return nil
	}
	return &labelReader{engine: engine}
}

// ReadLabel recognises the words in one control-sized crop.
func (r *labelReader) ReadLabel(ctx context.Context, img image.Image) ([]vision.TextSpan, error) {
	if img == nil {
		return nil, nil
	}
	results, err := r.engine.Recognize(ctx, ocr.ImageInput{Image: img})
	if err != nil {
		return nil, err
	}
	spans := make([]vision.TextSpan, 0, len(results))
	for _, res := range results {
		spans = append(spans, vision.TextSpan{
			Text:       res.Text,
			Confidence: res.Confidence,
			Bounds:     res.Bounds,
		})
	}
	return spans, nil
}
