package main

import (
	"context"
	"image"
	"os"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
	"github.com/chaynes-simpleclouds/marco/internal/platform/visionclient"
)

// The production detector, as a benchmark baseline.
//
// The SAME plugin the Director runs, reached the same way, so "current" in a report means
// what is actually shipping rather than an approximation of it. If the baseline were a
// reimplementation, every comparison would be against a model nobody uses.

type currentBackend struct {
	detector vision.Detector
	reason   string
}

var _ visionbench.Backend = (*currentBackend)(nil)

func newCurrentBackend() *currentBackend {
	detector, _, reason := newVisionDetector(defaultVisionBridge())
	return &currentBackend{detector: detector, reason: reason}
}

func (c *currentBackend) Name() string { return "current" }

func (c *currentBackend) Model() string {
	if c.detector == nil {
		return "unavailable"
	}
	return c.detector.Model()
}

// Detect runs one frame through the shipping detector.
func (c *currentBackend) Detect(ctx context.Context, frame image.Image) ([]visionbench.Detection, error) {
	if c.detector == nil {
		return nil, errUnavailable(c.reason)
	}
	results, err := c.detector.Detect(ctx, vision.Input{Image: frame})
	if err != nil {
		return nil, err
	}
	out := make([]visionbench.Detection, 0, len(results))
	for _, r := range results {
		out = append(out, visionbench.Detection{
			Label: r.Class, Confidence: r.Confidence, Bounds: r.Bounds, Text: r.Text,
		})
	}
	return out, nil
}

type unavailableErr string

func (e unavailableErr) Error() string { return string(e) }

func errUnavailable(reason string) error {
	if reason == "" {
		reason = "no detector is configured"
	}
	return unavailableErr(reason)
}

// keep the bridge import honest across build tags
var _ = os.Getenv
var _ *bridgehost.Host
var _ = visionclient.New
