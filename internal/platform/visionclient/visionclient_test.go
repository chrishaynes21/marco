package visionclient

import (
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// The bridge seam.
//
// Everything else in the vision milestone is tested against a fake Detector, which means
// THIS is the one layer where a mistake would be invisible until a real model was
// installed — the point where an image becomes bytes and a reply becomes detections.
//
// What matters here is not that it works when everything is fine. It is that the three
// distinguishable failures stay distinguishable: no plugin, a plugin that failed, and a
// plugin that answered with nothing. A layer that collapsed the first into the third would
// report an empty desktop to someone who never installed a model.

// fakeHost stands in for the bridge subprocess.
type fakeHost struct {
	call   runtime.HostCall
	status string
	data   runtime.Value
	err    error
}

func (f *fakeHost) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	f.call = c
	return f.status, f.data, f.err
}

// okReply builds the shape the plugin's doDetect returns.
func okReply(els ...map[string]any) runtime.Value {
	list := runtime.NewSet()
	items := runtime.NewList()
	for _, e := range els {
		s := runtime.NewSet()
		for k, v := range e {
			switch n := v.(type) {
			case string:
				s.Put(k, runtime.Text(n))
			case int:
				s.Put(k, runtime.Number(float64(n)))
			case float64:
				s.Put(k, runtime.Number(n))
			}
		}
		items.Append(runtime.SetVal(s))
	}
	list.Put("Elements", runtime.ListVal(items))
	return runtime.SetVal(list)
}

func testImage(w, h int) image.Image {
	return image.NewRGBA(image.Rect(0, 0, w, h))
}

func TestTheImageArrivesAsBase64PNG(t *testing.T) {
	h := &fakeHost{status: "ok", data: okReply()}
	d := New(h)

	if _, err := d.Detect(context.Background(), vision.Input{Image: testImage(40, 20)}); err != nil {
		t.Fatalf("Detect: %v", err)
	}

	if h.call.Act != "Vision" || h.call.Action != "Detect" {
		t.Fatalf("called %s's %s, want Vision's Detect", h.call.Act, h.call.Action)
	}
	set := h.call.Input.AsSet()
	if set == nil {
		t.Fatal("the input is not a set")
	}
	enc, ok := set.Get("Image")
	if !ok {
		t.Fatal("no Image field was sent")
	}
	raw, err := base64.StdEncoding.DecodeString(enc.AsText())
	if err != nil {
		t.Fatalf("the Image field is not base64: %v", err)
	}
	img, err := png.Decode(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("the Image field is not a PNG: %v", err)
	}
	// PNG, not JPEG: a detector reads edges and small glyphs, and JPEG rings around
	// exactly the high-contrast borders that make a control look like a control.
	if got := img.Bounds().Dx(); got != 40 {
		t.Fatalf("the picture arrived %dpx wide, want 40", got)
	}
}

func TestDetectionsComeBackInImageLocalCoordinates(t *testing.T) {
	h := &fakeHost{status: "ok", data: okReply(
		map[string]any{"Label": "button", "Score": 0.91, "X": 10, "Y": 20, "W": 60, "H": 24, "Text": "Craft"},
	)}

	got, err := New(h).Detect(context.Background(), vision.Input{Image: testImage(100, 100)})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d detections, want 1", len(got))
	}
	d := got[0]
	// Min/Max, not Min/size: a client that read W and H as the far corner would place
	// every box near the origin, and the error would look like a bad model.
	want := image.Rect(10, 20, 70, 44)
	if d.Bounds != want {
		t.Fatalf("bounds %v, want %v", d.Bounds, want)
	}
	if d.Class != "button" || d.Text != "Craft" || d.Confidence != 0.91 {
		t.Fatalf("decoded %+v, want the button as sent", d)
	}
}

func TestNoPluginIsUnavailableNotEmpty(t *testing.T) {
	var missing vision.Detector = &Detector{}

	_, err := missing.Detect(context.Background(), vision.Input{Image: testImage(4, 4)})

	var u *vision.Unavailable
	if !errors.As(err, &u) {
		t.Fatalf("a missing plugin reported %v, want vision.Unavailable", err)
	}
	if !strings.Contains(u.Reason, "DIRECTOR_VISION") {
		t.Fatalf("the reason %q does not say how to fix it", u.Reason)
	}
}

func TestATransportFailureIsACapabilityGap(t *testing.T) {
	h := &fakeHost{err: errors.New("exec: vision.exe: file does not exist")}

	_, err := New(h).Detect(context.Background(), vision.Input{Image: testImage(4, 4)})

	// The plugin not being there at all is the ordinary state of most machines. Reporting
	// it as a fault would put an error in front of someone who simply has no model.
	var u *vision.Unavailable
	if !errors.As(err, &u) {
		t.Fatalf("a dead bridge reported %v, want vision.Unavailable", err)
	}
}

func TestAMissingModelIsACapabilityGapButAnotherFailureIsAFault(t *testing.T) {
	msg := runtime.NewSet()
	msg.Put("Error", runtime.Text("Vision has no model loaded (set $MARCO_VISION_MODEL)"))
	noModel := &fakeHost{status: "failed", data: runtime.SetVal(msg)}

	_, err := New(noModel).Detect(context.Background(), vision.Input{Image: testImage(4, 4)})
	var u *vision.Unavailable
	if !errors.As(err, &u) {
		t.Fatalf("a modelless plugin reported %v, want vision.Unavailable", err)
	}

	broken := runtime.NewSet()
	broken.Put("Error", runtime.Text("the ONNX session crashed"))
	crashed := &fakeHost{status: "failed", data: runtime.SetVal(broken)}

	_, err = New(crashed).Detect(context.Background(), vision.Input{Image: testImage(4, 4)})
	if errors.As(err, &u) {
		t.Fatal("a crashed detector was reported as a capability gap; it is a fault")
	}
	if err == nil || !strings.Contains(err.Error(), "ONNX") {
		t.Fatalf("the fault %v does not carry the plugin's message", err)
	}
}

func TestAnUnreadableReplyIsAnErrorNotAnEmptyScreen(t *testing.T) {
	// Protocol drift that produced silence would look exactly like a screen with nothing
	// on it, so a reply of the wrong SHAPE has to be loud.
	h := &fakeHost{status: "ok", data: runtime.Text("elements: none")}

	got, err := New(h).Detect(context.Background(), vision.Input{Image: testImage(4, 4)})
	if err == nil {
		t.Fatalf("a reply of the wrong shape produced %d detections and no error", len(got))
	}
}

func TestAnEmptyListIsAnEmptyScreen(t *testing.T) {
	h := &fakeHost{status: "ok", data: okReply()}

	got, err := New(h).Detect(context.Background(), vision.Input{Image: testImage(4, 4)})
	if err != nil {
		t.Fatalf("an empty reply is a finding, not a failure: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d detections from an empty reply", len(got))
	}
}

func TestNoImageIsARefusalNotACapture(t *testing.T) {
	// The provider owns the frame. A client that captured one itself when handed none
	// would be a second observation path with its own idea of what was on screen.
	h := &fakeHost{status: "ok", data: okReply()}

	if _, err := New(h).Detect(context.Background(), vision.Input{}); err == nil {
		t.Fatal("Detect with no image succeeded; it must refuse")
	}
	if h.call.Act != "" {
		t.Fatal("Detect with no image still called the plugin")
	}
}

func TestTheModelIsNamedForProvenance(t *testing.T) {
	// An observation whose model nobody can name is one nobody can weigh.
	d := &Detector{ModelName: "yolov8n-ui.onnx"}
	if d.Model() != "yolov8n-ui.onnx" {
		t.Fatalf("Model() = %q", d.Model())
	}
	if (&Detector{}).Model() == "" {
		t.Fatal("an unnamed detector reports no model at all")
	}
}
