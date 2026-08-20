package main

import (
	"context"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Turning "the provider says it read window X" into proof, or into a refusal.
//
// # What this is not
//
// It is not a second target selector. It cannot choose a window, cannot reacquire one, and
// cannot make a window current. Given a window some provider claims its evidence came from,
// it answers one question — is that the window the Director currently holds, and is that
// window still itself right now — and the only answers are a provenance or a no.
//
// The distinction matters because a resolver that could acquire would quietly repair the
// exact situation it exists to detect: asked "is this evidence about the current target",
// it would make the observed window current and answer yes.
//
// # Why it re-reads the platform
//
// The tracker's held reference is what was true at its last validation, and the race here
// lives entirely in the gap since then:
//
//	generation 7 validated → provider runs → window replaced → provider returns
//
// A comparison against the cached reference cannot see that; the window is only known to be
// gone by asking the platform. Tracker.Confirm asks, and deliberately does not adopt.

// trackerResolver resolves observed windows against the Director's window tracker.
type trackerResolver struct {
	tracker *windowref.Tracker
}

var _ providers.TargetResolver = (*trackerResolver)(nil)

// newTargetResolver returns a resolver, or nil when this Director tracks no windows.
//
// Nil is a meaningful configuration rather than a defect: a Director with no tracker can
// still observe and still capture, and it cannot prove which window it observed. Everything
// downstream treats that as unproven, which on a targeted cycle means refused. That is the
// correct outcome — an unprovable session should not silently behave like a proven one.
func newTargetResolver(t *windowref.Tracker) providers.TargetResolver {
	if t == nil {
		return nil
	}
	return &trackerResolver{tracker: t}
}

// ResolveObserved reports the provenance of a window a provider says it observed.
//
// False whenever the claim cannot be established, and the three ways that happens are all
// genuine findings rather than errors:
//
//   - nothing is held, so there is no target to belong to;
//   - the window still validates but is NOT the one the provider read, which is what a
//     bridge falling back to the foreground window looks like from here;
//   - the held window no longer passes validation at all, which is what a window replaced
//     mid-collection looks like.
func (r *trackerResolver) ResolveObserved(ctx context.Context, window directorapi.WindowID,
	application string) (directorapi.TargetProvenance, bool) {

	// The platform, now — not the cached reference. See the note above.
	ref, ok := r.tracker.Confirm(ctx)
	if !ok {
		return directorapi.TargetProvenance{}, false
	}
	if window != "" && ref.ID != window {
		// The provider read a different window than the one being tracked. Its evidence
		// is honest and describes something else.
		return directorapi.TargetProvenance{}, false
	}
	// An application disagreement is refused rather than ignored: a window id that matches
	// while the application does not is the recycled-handle signature, and the whole reason
	// window identity is generational.
	if application != "" && ref.Application != "" &&
		!strings.EqualFold(application, ref.Application) {
		return directorapi.TargetProvenance{}, false
	}

	return directorapi.TargetProvenance{
		Application:      ref.Application,
		ProcessID:        ref.ProcessID,
		WindowGeneration: ref.Generation,
	}, true
}

// expectedTarget is the provenance a cycle against ref should require of its evidence.
//
// Built from the reference the RUNNER validated, which is what makes it intent: it says
// which generation the Director meant to observe, and it is compared against what each
// provider can independently prove. The two must never be built from one value — a guard
// comparing a value with itself passes everything, which is how the first version of this
// mechanism let the stale evidence through.
func expectedTarget(ref windowref.Ref) *directorapi.TargetProvenance {
	if ref.Zero() {
		return nil
	}
	return &directorapi.TargetProvenance{
		Application:      ref.Application,
		ProcessID:        ref.ProcessID,
		WindowGeneration: ref.Generation,
	}
}
