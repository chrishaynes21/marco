package providers

import (
	"context"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Establishing which window generation a frame's pixels belong to.
//
// # Why the existing ownership check is not enough
//
// The capture backend already asks who owns the window before and after the grab, and
// refuses the frame if the answer changed. That check compares application NAMES:
//
//	ownerBefore != w.Application        → refuse
//	ownerAfter  != ownerBefore          → refuse
//
// It catches a window changing hands to a different application. It does not catch a window
// of the SAME application being replaced — generation 7 to generation 8 of one editor
// passes both comparisons unremarked. That is the race this gate exists for, so the
// existing check cannot be the evidence.
//
// # Why a decorator
//
// The resolution happens strictly AFTER the inner capture returns, which is what makes it
// evidence rather than intent: it reads the platform as it is once the pixels are in hand,
// not as it was when the capture was requested. Wrapping keeps that ordering impossible to
// get wrong, and leaves the platform backend free of any notion of Director generations.

// ProvenCapture wraps a window capture and establishes each frame's target provenance.
type ProvenCapture struct {
	inner    capture.WindowCapture
	resolver TargetResolver
}

// NewProvenCapture wraps a capture so its frames carry provenance.
//
// A nil resolver is allowed and yields frames with no target — which everything downstream
// treats as unproven. That is the correct behaviour for a Director with no window tracker:
// it can still capture, and it cannot claim what it captured.
func NewProvenCapture(inner capture.WindowCapture, resolver TargetResolver) *ProvenCapture {
	return &ProvenCapture{inner: inner, resolver: resolver}
}

var _ capture.WindowCapture = (*ProvenCapture)(nil)

// CaptureWindow takes the picture, then establishes what it is a picture OF.
func (p *ProvenCapture) CaptureWindow(ctx context.Context,
	window directorapi.Window) (capture.Image, error) {

	img, err := p.inner.CaptureWindow(ctx, window)
	if err != nil {
		return img, err
	}
	if p.resolver == nil {
		return img, nil
	}

	// The window the frame says it came from, resolved against the platform NOW. If the
	// window was replaced while the capture ran, this reports the replacement's
	// generation and the frame is left describing a window that is no longer current —
	// which the provenance guard then refuses.
	//
	// Deliberately resolved from the IMAGE's own window fields rather than from the
	// requested window: the frame is the thing being attributed, and attributing it to
	// what was asked for is the mistake this whole design exists to avoid.
	if target, ok := p.resolver.ResolveObserved(ctx, img.WindowID, img.Application); ok {
		img.Target = &target
	}
	return img, nil
}
