package main

// Reading the words on the watched window, once, when something needs them.
//
// Not part of the observation loop and deliberately not: OCR costs hundreds of milliseconds and
// the loop runs every hundred, and nothing in an ordinary run needs text at all. This is asked
// only where a step has refused for want of a control's identity — see reground.go — so the cost
// is paid once, by the case that cannot proceed without it.

import (
	"context"
	"image"
	"os"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/platform/ocrclient"
	"github.com/chaynes-simpleclouds/marco/internal/platform/wincapture"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// wordReader reads the words on a window, through the OCR plugin.
type wordReader struct {
	eng    *ocrclient.Engine
	shots  *wincapture.Capture
	window func() (directorapi.Window, bool)
	why    string
}

// newWordReader finds the OCR bridge, or returns one that explains why it cannot run.
func newWordReader(shots *wincapture.Capture,
	window func() (directorapi.Window, bool)) *wordReader {

	r := &wordReader{shots: shots, window: window}
	bridge := defaultOCRBridge()
	if bridge == "" {
		r.why = "no OCR plugin found — build plugins/ocr and set $DIRECTOR_OCR"
		return r
	}
	if _, err := os.Stat(bridge); err != nil {
		r.why = "the OCR plugin is not at " + bridge
		return r
	}
	h := bridgehost.New(bridge)
	if t := defaultTesseract(); t != "" {
		h = h.WithEnv("MARCO_TESSERACT=" + t)
	}
	r.eng = ocrclient.New(h)
	return r
}

func (r *wordReader) working() bool { return r != nil && r.eng != nil }

// read captures the window and returns every word on it, in window pixels.
func (r *wordReader) read(ctx context.Context) ([]readWord, image.Rectangle, bool) {
	if !r.working() {
		return nil, image.Rectangle{}, false
	}
	w, ok := r.window()
	if !ok {
		return nil, image.Rectangle{}, false
	}
	img, err := r.shots.CaptureWindow(ctx, w)
	if err != nil {
		return nil, image.Rectangle{}, false
	}
	got, err := r.eng.Recognize(ctx, ocr.ImageInput{Image: img.Image})
	if err != nil {
		r.why = err.Error()
		return nil, image.Rectangle{}, false
	}
	out := make([]readWord, 0, len(got))
	for _, g := range got {
		if g.Text == "" || g.Bounds.Empty() {
			continue
		}
		out = append(out, readWord{
			Text: g.Text,
			X:    g.Bounds.Min.X, Y: g.Bounds.Min.Y,
			W: g.Bounds.Dx(), H: g.Bounds.Dy(),
			Conf: g.Confidence,
		})
	}
	return out, img.Image.Bounds(), len(out) > 0
}

// defaultTesseract finds the engine the OCR plugin shells out to.
//
// The plugin looks for it on PATH, and it is not on PATH here — the installer puts it under
// Program Files without adding it. Naming the usual location means the repair path works on a
// machine where tesseract is installed and nobody has edited their environment, and
// $MARCO_TESSERACT still wins for anybody pointing at their own.
func defaultTesseract() string {
	if p := os.Getenv("MARCO_TESSERACT"); p != "" {
		return p
	}
	for _, c := range []string{
		`C:\Program Files\Tesseract-OCR\tesseract.exe`,
		`C:\Program Files (x86)\Tesseract-OCR\tesseract.exe`,
	} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
