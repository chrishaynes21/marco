package main

import (
	"image"
	"strings"
	"testing"
)

func el(label string, x, y, w, h int, score float32) Element {
	return Element{Label: label, Box: image.Rect(x, y, x+w, y+h), Score: score}
}

func TestPickHighestScoreNoHint(t *testing.T) {
	els := []Element{
		el("button", 0, 0, 40, 20, 0.4),
		el("button", 100, 0, 40, 20, 0.9),
	}
	got, ok := pick(els, "", 0, 0, false)
	if !ok || got.Score != 0.9 {
		t.Fatalf("pick = %+v ok=%v, want the 0.9 button", got, ok)
	}
}

func TestPickByLabel(t *testing.T) {
	els := []Element{
		el("icon", 0, 0, 30, 30, 0.95),
		el("menu item", 100, 0, 80, 24, 0.5),
	}
	got, ok := pick(els, "menu", 0, 0, false) // substring match, case-insensitive
	if !ok || got.Label != "menu item" {
		t.Fatalf("pick(menu) = %+v ok=%v, want the menu item", got, ok)
	}
}

func TestPickNearestHint(t *testing.T) {
	els := []Element{
		el("button", 0, 0, 40, 20, 0.9),      // centre (20,10)
		el("button", 500, 500, 40, 20, 0.95), // centre (520,510)
	}
	got, ok := pick(els, "button", 510, 505, true) // hint near the second
	if !ok {
		t.Fatal("pick returned nothing")
	}
	if cx, _ := got.Center(); cx != 520 {
		t.Fatalf("pick nearest hint chose centre x %d, want 520", cx)
	}
}

func TestPickNoMatch(t *testing.T) {
	els := []Element{el("button", 0, 0, 40, 20, 0.9)}
	if _, ok := pick(els, "prompt", 0, 0, false); ok {
		t.Fatal("pick matched a label that isn't present")
	}
}

func TestUnconfiguredDetectorDeclines(t *testing.T) {
	// An unconfigured detector must never invent detections. Both builds satisfy that, and
	// they say so differently: the default build finds nothing quietly, while the onnxvision
	// build reports WHY it cannot run. Either is correct; a detection is not.
	//
	// Asserted this way because the previous version pinned the default build's exact
	// return and did not compile under `-tags onnxvision` at all, so the production build's
	// behaviour here had never been tested.
	d := newDetector()
	if d.Ready() {
		t.Fatal("an unconfigured detector reports ready")
	}
	els, err := d.Detect(image.NewRGBA(image.Rect(0, 0, 10, 10)))
	if len(els) != 0 {
		t.Fatalf("unconfigured detect returned %d elements; it must find nothing", len(els))
	}
	if err != nil && !strings.Contains(err.Error(), "MARCO_VISION_MODEL") {
		t.Fatalf("unconfigured detect failed with %v; an error here must name the missing "+
			"configuration, not something a caller cannot act on", err)
	}
}
