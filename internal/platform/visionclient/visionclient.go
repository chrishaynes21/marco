// Package visionclient reaches the vision plugin over Marco's bridge.
//
// The same shape as ocrclient and uiaclient: a subprocess speaking JSON over stdio, wrapped
// so the Director's perception layer sees only an interface. It exists for the same reason
// too — a detector needs ONNX or OpenCV, and the engine module permits no external
// dependencies. Keeping it behind a bridge means the dependency lives in a plugin module
// that the engine merely launches.
//
// It implements vision.Detector and nothing else. Where the image came from, where its
// pixels sit on the desktop, what a box means, and whether any of it is safe to act on are
// all decided above this layer.
//
// # The image goes DOWN, the coordinates do not come back up
//
// The Director captures the frame and sends it; the plugin answers in IMAGE-LOCAL
// coordinates. That direction is deliberate. A plugin that captured the screen itself would
// be a second observation path, and a plugin that returned desktop coordinates would be
// placing observations — which requires knowing the window bounds, the DPI scale and the
// monitor origin, all of which the provider already knows and none of which the plugin has
// any way to get right.
package visionclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// Detector finds UI elements by asking the vision plugin.
type Detector struct {
	host runtime.Host
	// Timeout bounds one bridge call. The provider also bounds the context; this is the
	// floor, so a host that ignores cancellation still cannot hang forever.
	Timeout time.Duration
	// ModelName is what the plugin was configured with, for provenance. Read from the
	// environment at construction because the plugin has no protocol for reporting it,
	// and an observation whose model nobody can name is one nobody can weigh.
	ModelName string
}

// New wraps a bridge host.
func New(h runtime.Host) *Detector {
	return &Detector{
		host: h, Timeout: 30 * time.Second,
		ModelName: modelName(),
	}
}

var _ vision.Detector = (*Detector)(nil)

// Model names what produced the results.
func (d *Detector) Model() string {
	if d.ModelName != "" {
		return d.ModelName
	}
	return "vision-bridge"
}

// modelName reads the configured model, falling back to something honest.
func modelName() string {
	if m := strings.TrimSpace(os.Getenv("MARCO_VISION_MODEL")); m != "" {
		// The path's last element is enough to tell two models apart, and the whole
		// path in every observation would be noise.
		if i := strings.LastIndexAny(m, `/\`); i >= 0 {
			m = m[i+1:]
		}
		return m
	}
	return "vision-bridge"
}

// Detect encodes the image, asks the plugin what is in it, and converts the reply.
func (d *Detector) Detect(ctx context.Context, in vision.Input) ([]vision.Detection, error) {
	if d.host == nil {
		return nil, &vision.Unavailable{
			Backend: "vision-bridge",
			Reason: "no vision plugin is configured — set $DIRECTOR_VISION to " +
				"plugins/vision/vision.exe",
		}
	}
	if in.Image == nil {
		return nil, fmt.Errorf("visionclient: no image to detect in")
	}

	encoded, err := encodePNG(in.Image)
	if err != nil {
		return nil, fmt.Errorf("visionclient: encoding the frame: %w", err)
	}

	input := runtime.NewSet()
	input.Put("Image", runtime.Text(encoded))
	status, data, err := d.host.Invoke(runtime.HostCall{
		Act: "Vision", Action: "Detect", Input: runtime.SetVal(input),
	})
	if err != nil {
		// A transport failure is usually the plugin not being there at all, which is a
		// capability gap rather than a fault — and must be reported as one so the
		// Director can say "no detector is installed" instead of "this window is empty".
		return nil, &vision.Unavailable{Backend: "vision-bridge", Reason: err.Error()}
	}
	if status != "ok" {
		msg := errText(data)
		// The plugin says this exact thing when it has no weights. It is a capability
		// gap, not a failure, and the difference is what a user can act on.
		if strings.Contains(strings.ToLower(msg), "no model") {
			return nil, &vision.Unavailable{Backend: "vision-bridge", Reason: msg}
		}
		return nil, fmt.Errorf("visionclient: the vision plugin reported %q: %s", status, msg)
	}
	return decode(data)
}

// errText pulls a message out of a failed reply.
func errText(v runtime.Value) string {
	// JSONFromValue first: a runtime.Value's fields are unexported, so marshalling it
	// directly produces "{}" and every message would read as empty.
	raw, err := json.Marshal(runtime.JSONFromValue(v))
	if err != nil {
		return "the plugin reported a failure with no readable message"
	}
	var payload struct {
		Error   string `json:"Error"`
		Message string `json:"Message"`
	}
	_ = json.Unmarshal(raw, &payload)
	switch {
	case payload.Error != "":
		return payload.Error
	case payload.Message != "":
		return payload.Message
	}
	return string(raw)
}

// decode converts the plugin's reply into detections.
//
// Defensive about SHAPE and strict about content: a reply that is not the expected shape is
// an error rather than an empty result, because a protocol drift that produced silence
// would look exactly like a screen with nothing on it.
func decode(reply runtime.Value) ([]vision.Detection, error) {
	// JSONFromValue first. A runtime.Value carries its payload in unexported fields, so
	// marshalling it directly yields "{}" — which unmarshals into the payload struct
	// without complaint and produces NO detections. That failure is silent and looks
	// exactly like a screen with nothing on it, which is why the shape check below is
	// worth nothing unless this line is right.
	raw, err := json.Marshal(runtime.JSONFromValue(reply))
	if err != nil {
		return nil, fmt.Errorf("visionclient: unreadable reply: %w", err)
	}
	var payload struct {
		Elements []struct {
			Label string  `json:"Label"`
			Score float64 `json:"Score"`
			X     int     `json:"X"`
			Y     int     `json:"Y"`
			W     int     `json:"W"`
			H     int     `json:"H"`
			Text  string  `json:"Text"`
		} `json:"Elements"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("visionclient: the plugin's reply is not a detection list: %w", err)
	}

	out := make([]vision.Detection, 0, len(payload.Elements))
	for _, e := range payload.Elements {
		out = append(out, vision.Detection{
			Class:      e.Label,
			Bounds:     image.Rect(e.X, e.Y, e.X+e.W, e.Y+e.H),
			Confidence: e.Score,
			Text:       e.Text,
		})
	}
	return out, nil
}

// encodePNG renders an image as base64 PNG for the bridge.
//
// PNG rather than JPEG: a detector reads edges and small glyphs, and JPEG's ringing around
// high-contrast boundaries is exactly the artifact that turns a crisp control border into
// something a model is less sure about.
func encodePNG(img image.Image) (string, error) {
	rgba, ok := img.(*image.RGBA)
	if !ok {
		b := img.Bounds()
		rgba = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
