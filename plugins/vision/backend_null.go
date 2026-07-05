//go:build !onnxvision

package main

import "image"

// newDetector returns the NULL detector in the default build: no ONNX dependency, no
// model, no detections. The plugin still builds, runs, and speaks the bridge protocol —
// `Vision's Detect`/`Locate` simply decline (Ready=false), so a route that reaches for
// vision falls through gracefully, exactly like the OCR host's "no tesseract" path. The
// real detector is compiled in with `-tags onnxvision` (see backend_onnx.go); keeping it
// behind a tag is what lets the engine and this plugin stay dependency-free by default.
func newDetector() detector { return nullDetector{} }

type nullDetector struct{}

func (nullDetector) Detect(*image.RGBA) ([]Element, error) { return nil, nil }
func (nullDetector) Ready() bool                           { return false }
