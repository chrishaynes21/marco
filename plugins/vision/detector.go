package main

import "image"

// Element is one UI element a detector found: its class label (e.g. "button", "icon",
// "menu item", "prompt"), bounding box in the captured image's pixel space, and the
// detector's confidence 0..1.
type Element struct {
	Label string
	Box   image.Rectangle
	Score float32
}

// Center returns the element box's centre point (image-local).
func (e Element) Center() (x, y int) {
	return (e.Box.Min.X + e.Box.Max.X) / 2, (e.Box.Min.Y + e.Box.Max.Y) / 2
}

// detector finds UI elements in a captured image. It's an interface so the real ONNX
// backend (build tag `onnxvision`) can be swapped for the null backend (the default —
// no model, no detections) and for a fake in tests, the same stub-behind-interface
// pattern the rest of Marco uses for heavyweight/OS surfaces.
type detector interface {
	// Detect returns the elements found in img. A nil error with an empty slice means
	// "ran, found nothing"; a non-nil error means the detector is unavailable (no model
	// or runtime) and is surfaced once, not polled.
	Detect(img *image.RGBA) ([]Element, error)
	// Ready reports whether a model is actually loaded. The bridge uses it to answer a
	// capability probe and to decline gracefully when vision was requested but no model
	// is wired.
	Ready() bool
}
