package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"
)

// fakeDetector returns canned elements regardless of the image, so the learn-time
// Identify path is testable without a model.
type fakeDetector struct{ els []Element }

func (f fakeDetector) Detect(*image.RGBA) ([]Element, error) { return f.els, nil }
func (f fakeDetector) Ready() bool                           { return true }

func tinyPNG(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestElementAtPicksContaining(t *testing.T) {
	els := []Element{
		el("icon", 0, 0, 40, 40, 0.7),
		el("button", 100, 100, 80, 30, 0.9), // contains (130,115)
	}
	got, ok := elementAt(els, 130, 115)
	if !ok || got.Label != "button" {
		t.Fatalf("elementAt = %+v ok=%v, want the button", got, ok)
	}
}

func TestElementAtDeclinesWhenNoneContain(t *testing.T) {
	els := []Element{el("button", 100, 100, 80, 30, 0.9)}
	if _, ok := elementAt(els, 5, 5); ok {
		t.Fatal("elementAt matched a click outside every element box")
	}
}

func TestIdentifyReturnsLabel(t *testing.T) {
	det := fakeDetector{els: []Element{el("icon", 0, 0, 50, 50, 0.8)}}
	status, data, err := doIdentify(det, map[string]any{"Image": tinyPNG(t), "ClickX": 20, "ClickY": 20})
	if err != nil || status != "ok" {
		t.Fatalf("doIdentify = %q err=%v", status, err)
	}
	m, _ := data.(map[string]any)
	if m["Label"] != "icon" {
		t.Fatalf("Label = %v, want icon", m["Label"])
	}
	// The detected box is returned too (for the engine to re-crop the template).
	if m["W"] != 50 || m["H"] != 50 {
		t.Fatalf("box = (%v,%v %vx%v), want a 50x50 box", m["X"], m["Y"], m["W"], m["H"])
	}
}

func TestIdentifyDeclinesOffElement(t *testing.T) {
	det := fakeDetector{els: []Element{el("icon", 200, 200, 50, 50, 0.8)}}
	status, _, err := doIdentify(det, map[string]any{"Image": tinyPNG(t), "ClickX": 4, "ClickY": 4})
	if status != "failed" || err != nil {
		t.Fatalf("doIdentify off-element = %q err=%v, want failed/nil", status, err)
	}
}

// notReadyDetector stands in for a build with no model behind it.
//
// Defined here rather than reusing `nullDetector`, which only exists under `!onnxvision` — the
// test compiled in one build and broke the other, so `go test -tags onnxvision` had never run
// in this module at all.
type notReadyDetector struct{}

func (notReadyDetector) Detect(*image.RGBA) ([]Element, error) { return nil, nil }
func (notReadyDetector) Ready() bool                           { return false }

func TestIdentifyNoModel(t *testing.T) {
	status, _, err := doIdentify(notReadyDetector{}, map[string]any{"Image": tinyPNG(t)})
	if status != "failed" || err == nil {
		t.Fatalf("doIdentify with null detector = %q err=%v, want failed/error", status, err)
	}
}
