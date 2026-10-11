package main

import (
	"context"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// Which window was in front when a frame was taken.
//
// # Why a capture needs asking
//
// Capturing by window rather than by screen avoids stealing focus from the thing being
// observed, and it is not occlusion-safe: on Windows it can return the pixels of whatever is
// covering the target, under the target's own name. None of the existing guards see it.
// Experiment-025 captured the Windows taskbar on one look of 107 with `target_losses` at 0 and
// provenance clean, because every one of those checks is about window IDENTITY and this is not
// an identity failure. The window was alive, was the right window, and the pixels belonged to
// something else.
//
// # Why this cannot fix it, only report it
//
// Nothing here may raise the window, change focus, or re-capture. The observation path is
// passive by construction and a resolver that could act would quietly repair the situation it
// exists to detect — the same reason `trackerResolver` cannot acquire. So the answer is carried
// on the sample and the belief refuses the look, which is the honest outcome: a frame that is
// not about the target is not evidence about the target.

// targetFrontness answers, now, whether the window being observed is the one in front.
//
// Asked per look rather than cached, like everything else in the window layer: a cached answer
// describes where the person was looking a moment ago, and the whole value of this is that it
// is true at the instant the frame it describes was taken.
func (r *Runtime) targetFrontness(ctx context.Context, target windowref.Ref) observe.Frontness {
	if r == nil || r.winPlatform == nil || target.Zero() {
		return observe.FrontUnknown
	}
	front, res, _ := windowref.Foreground(ctx, r.winPlatform)
	if res != windowref.Resolved {
		// Nothing identifiable in front. NOT FrontOther: a desktop the platform could not
		// describe is not evidence that another application was covering the target, and
		// reporting it as one would throw away looks for a reason nobody measured.
		return observe.FrontUnknown
	}
	return frontnessOf(front, target)
}

// frontnessOf is the decision itself, separated from the platform call so it can be tested
// against a candidate rather than against a desktop — the same split as marcoOwns.
func frontnessOf(front windowref.Candidate, target windowref.Ref) observe.Frontness {
	if front.Handle != 0 && target.Handle != 0 && front.Handle == target.Handle {
		return observe.FrontTarget
	}
	// THE SAME APPLICATION COUNTS AS THE TARGET. A game commonly has a launcher, a splash
	// window and a render window, and a frame captured while a sibling window of the same
	// application is in front is still that application's pixels. Refusing those would
	// discard real evidence to catch a case this does not exist for — what it exists for is
	// a DIFFERENT program covering the target.
	if front.Application != "" && target.Application != "" &&
		strings.EqualFold(front.Application, target.Application) {
		return observe.FrontTarget
	}
	// A foreground window that can be neither named nor numbered is not a finding either.
	if front.Handle == 0 && front.Application == "" {
		return observe.FrontUnknown
	}
	return observe.FrontOther
}
