// Package ocrclient reaches the OCR plugin over Marco's bridge.
//
// The same shape as uiaclient: a subprocess speaking JSON over stdio, wrapped so the
// Director's perception layer sees only an interface. It exists for the same reason
// too — the OCR runtime is tesseract, which is an external program, and the engine
// module permits no external dependencies. Keeping it behind a bridge means the
// dependency lives in a plugin module that the engine merely launches.
//
// It implements ocr.Engine and nothing else. Where the image came from, where its
// pixels sit on the desktop, and whether any of the text means anything are all
// decided above this layer.
package ocrclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

// Engine recognises text by asking the OCR plugin.
type Engine struct {
	host runtime.Host
	// Timeout bounds one bridge call. The provider also bounds the context; this is
	// the floor, so a host that ignores cancellation still cannot hang forever.
	Timeout time.Duration
}

// New wraps a bridge host.
func New(h runtime.Host) *Engine {
	return &Engine{host: h, Timeout: 20 * time.Second}
}

var _ ocr.Engine = (*Engine)(nil)

// Recognize encodes the image, asks the plugin for every word, and converts the reply.
func (e *Engine) Recognize(ctx context.Context, in ocr.ImageInput) ([]ocr.Result, error) {
	if e.host == nil {
		return nil, &ocr.Unavailable{
			Engine: "ocr-bridge",
			Reason: "no OCR plugin is configured — set $DIRECTOR_OCR to plugins/ocr/ocr.exe",
		}
	}
	if in.Image == nil {
		return nil, errors.New("ocrclient: no image to recognise")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rgba, ok := in.Image.(*image.RGBA)
	if !ok {
		b := in.Image.Bounds()
		rgba = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Bounds(), in.Image, b.Min, draw.Src)
	}
	png, err := screen.EncodePNG(rgba)
	if err != nil {
		return nil, fmt.Errorf("ocrclient: encoding the capture: %w", err)
	}

	input := runtime.NewSet()
	input.Put("Image", runtime.Text(base64.StdEncoding.EncodeToString(png)))
	status, data, err := e.host.Invoke(runtime.HostCall{
		Act: "Text", Action: "Words", Input: runtime.SetVal(input),
	})
	if err != nil {
		// A transport failure is usually the plugin not being there at all, which is a
		// capability gap rather than a fault — and must be reported as one so the
		// Director can say "OCR is not installed" instead of "this window has no text".
		if isMissing(err) {
			return nil, &ocr.Unavailable{Engine: "ocr-bridge", Reason: err.Error()}
		}
		return nil, fmt.Errorf("ocrclient: %w", err)
	}
	if status != "ok" {
		msg := errText(data)
		if strings.Contains(strings.ToLower(msg), "tesseract") {
			return nil, &ocr.Unavailable{Engine: "tesseract", Reason: msg}
		}
		return nil, fmt.Errorf("ocrclient: the OCR plugin reported %q: %s", status, msg)
	}
	return convert(data), nil
}

// convert reads the plugin's reply into engine results.
//
// Tolerant of missing fields and intolerant of nonsense: a word with no text or no box
// is dropped here rather than passed on to be rejected by the provider's filter, so the
// provider's rejection counters describe the ENGINE's output rather than this decoder's.
func convert(v runtime.Value) []ocr.Result {
	var wire struct {
		Words []struct {
			Text  string  `json:"Text"`
			X     float64 `json:"X"`
			Y     float64 `json:"Y"`
			W     float64 `json:"W"`
			H     float64 `json:"H"`
			Conf  float64 `json:"Conf"`
			Line  string  `json:"Line"`
			Index float64 `json:"Index"`
		} `json:"Words"`
	}
	if err := decode(v, &wire); err != nil {
		return nil
	}

	out := make([]ocr.Result, 0, len(wire.Words))
	for _, w := range wire.Words {
		if strings.TrimSpace(w.Text) == "" {
			continue
		}
		x, y, dx, dy := int(w.X), int(w.Y), int(w.W), int(w.H)
		if dx <= 0 || dy <= 0 {
			continue
		}
		out = append(out, ocr.Result{
			Text:   w.Text,
			Bounds: image.Rect(x, y, x+dx, y+dy),
			// Tesseract scores 0..100; the Engine contract is 0..1. Converted HERE,
			// at the boundary, so no engine's convention leaks into the provider —
			// a mixed convention would reject everything or accept everything.
			Confidence: w.Conf / 100,
			LineID:     w.Line,
			WordIndex:  int(w.Index),
		})
	}
	return out
}

// decode re-marshals a bridge Value into a typed struct, exactly as uiaclient does.
// The Value came from JSON, so round-tripping it beats walking the tree by hand.
func decode(v runtime.Value, out any) error {
	raw, err := json.Marshal(runtime.JSONFromValue(v))
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// isMissing reports whether an error means the plugin could not be started.
func isMissing(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "not found") ||
		strings.Contains(s, "no such file") ||
		strings.Contains(s, "cannot find") ||
		strings.Contains(s, "executable file not found") ||
		errors.Is(err, os.ErrNotExist)
}

func errText(v runtime.Value) string {
	if e := v.AsError(); e != nil {
		return e.Message
	}
	if s := v.AsText(); s != "" {
		return s
	}
	return "no detail"
}
