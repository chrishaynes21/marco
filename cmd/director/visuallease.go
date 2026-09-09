package main

import (
	"context"
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/visualstate"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// NOT RE-OBSERVED BY THIS SENSOR IS NOT NO LONGER VISIBLE.
//
// # The defect
//
// `SetActionables` replaces. So the candidate set a human press resolves against is rebuilt
// from each acquisition's world, and a sensor bought once per settled screen state contributes
// to exactly one of them. On an application whose accessibility tree exposes one window, the
// semantic repair found `menu "Library"` and every following acquisition offered the window
// alone — not because anything observed Library disappearing, but because the only sensor that
// can see it did not run.
//
// Measured: a human click landed geometrically inside Library and resolved to the enclosing
// window, whose sole candidacy the per-press record showed directly.
//
// # Why this is not memory
//
// Because it is never asserted from age. A lease is justified continuously, in the present
// tense, by photographing the same rectangle again and asking whether the picture still
// matches — `visualstate` reduces a region to a grid of average cell colours and compares, at
// roughly a thirtieth of what the detector costs. `ChangeIdentical` and `ChangeMinor` are
// affirmative evidence that the scene which grounded the Target has not been contradicted;
// anything else drops it on the spot.
//
// So `a remembered Target is not a currently visible Target` is untouched. Nothing here
// survives the session, reaches semantic memory, establishes a Place, strengthens graph
// evidence or authorises an action. Autonomous execution still resolves its own targets
// against a fresh reading — this lease has exactly one consumer, the candidate set a human
// press is attributed against.
//
// See [[ADR-131-a-sensor-sitting-out-is-not-a-thing-disappearing]].
type visualLease struct {
	// items are the pixel-derived controls the last visual acquisition offered, held by
	// value so a later mutation of the world cannot rewrite what was observed.
	items []directorapi.Element
	// from is the scene they were observed in. Every renewal compares against THIS rather
	// than against the previous check, so drift accumulates against the acquisition that
	// grounded the Targets instead of creeping a cell at a time.
	from visualstate.Snapshot
	// window and generation are the cheap discontinuity checks, made before the expensive
	// one: a different window, or the same window replaced, is a different scene whatever
	// the pixels say.
	window     directorapi.WindowID
	generation uint64
	at         time.Time
}

// leaseAttribute marks an element that reached the offer on a lease rather than on this
// acquisition, so the per-press diagnostic can say which evidence explained a click.
const leaseAttribute = "visual_lease"

// withLease adds still-current visual evidence to what this acquisition found, or renews the
// lease from it.
//
// The two arms are exclusive on purpose. An acquisition that DID see pixels is the newest
// possible evidence and replaces the lease; one that did not may borrow the previous one for as
// long as the scene it was taken from is demonstrably still on screen.
func (s *liveSampler) withLease(ctx context.Context, found []*directorapi.Element,
	window directorapi.Window, generation uint64, now time.Time) []*directorapi.Element {

	var pixel []directorapi.Element
	attributable := 0
	for _, el := range found {
		if el == nil {
			continue
		}
		attributable++
		if el.Provenance.OnlyDescribesPixels() {
			pixel = append(pixel, *el)
		}
	}
	if len(pixel) > 0 {
		s.renewLease(ctx, pixel, window, generation, now)
		return found
	}
	if s.lease == nil {
		if s.settledIntoANewScene(ctx, window) {
			// The previously repaired answer does not describe what is on screen. The
			// token goes back; the ordinary gate claims it, once.
			s.rt.reopenRepair()
			s.leaseWhy = "the screen settled into a different scene, so the previous " +
				"repair no longer answers it — one repair is available again"
			return found
		}
		// SAID OUT LOUD, because silence here is the case that matters. An acquisition
		// that offered nothing pixel-derived and holds no lease is indistinguishable, from
		// the outside, from a lease that was taken and dropped — and the first time this
		// ran live it was the former while every diagnostic implied the latter.
		// The last DROP reason is carried, because it is the answer and it used to be
		// overwritten by this very message on the next acquisition — leaving "none held"
		// as the only account of a lease that had been alive a second earlier.
		since := ""
		if s.leaseDropped != "" {
			since = "; last dropped because " + s.leaseDropped
		}
		s.leaseWhy = fmt.Sprintf(
			"none held, and none of the %d attributable control(s) this acquisition "+
				"offered came from pixels alone%s", attributable, since)
		return found
	}
	leased, ok := s.leaseStillCurrent(ctx, window, generation)
	if !ok {
		return found
	}
	for i := range leased {
		el := leased[i]
		if el.Attributes == nil {
			el.Attributes = map[string]any{}
		}
		el.Attributes[leaseAttribute] = true
		leased[i] = el
		found = append(found, &leased[i])
	}
	return found
}

// renewLease photographs the scene this acquisition's visual evidence came from.
//
// A capture that fails costs the lease and never the acquisition: visual evidence is a
// strengthening, and a Director that refused to observe because it could not take a picture
// would be worse than one that simply offers what it can see now.
func (s *liveSampler) renewLease(ctx context.Context, pixel []directorapi.Element,
	window directorapi.Window, generation uint64, now time.Time) {

	s.lease, s.leaseWhy = nil, ""
	if s.rt == nil || s.rt.visual == nil {
		return
	}
	if window.Bounds.Width <= 0 || window.Bounds.Height <= 0 {
		s.leaseWhy = "the window has no extent to photograph"
		return
	}
	snap, err := s.rt.visual.Capture(ctx, window.Bounds, window)
	if err != nil {
		s.leaseWhy = "the visual scene could not be photographed"
		return
	}
	s.lease = &visualLease{
		items: pixel, from: snap,
		window: window.ID, generation: generation, at: now,
	}
	s.leaseWhy = "observed by the detector on this acquisition"
}

// leaseStillCurrent asks whether the scene that grounded the leased Targets is still on screen.
//
// Ordered cheapest-first, and every negative answer DROPS the lease rather than deferring it. A
// lease that survived an unanswerable question would be exactly the elapsed-time cache this
// design refuses.
func (s *liveSampler) leaseStillCurrent(ctx context.Context, window directorapi.Window,
	generation uint64) ([]directorapi.Element, bool) {

	l := s.lease
	if l == nil || s.rt == nil || s.rt.visual == nil {
		return nil, false
	}
	switch {
	case l.window != window.ID:
		return s.dropLease("a different window is in front")
	case l.generation != generation:
		return s.dropLease("the window was replaced")
	case l.from.Region != window.Bounds:
		return s.dropLease("the window moved or resized")
	}
	now, err := s.rt.visual.Capture(ctx, window.Bounds, window)
	if err != nil {
		return s.dropLease("the scene could not be photographed again")
	}
	res, err := s.rt.visual.Compare(l.from, now)
	if err != nil {
		return s.dropLease("the two scenes could not be compared")
	}
	switch res.Kind {
	case visualstate.ChangeIdentical, visualstate.ChangeMinor:
		// AFFIRMATIVE, and the only positive arm. Identical proves nothing happened;
		// minor is the noise floor where a caret, a clock and a passing cursor live.
		s.leaseWhy = "the scene it was observed in is still on screen — " + res.Reason
		return l.items, true
	}
	// Meaningful change, or a scene still in motion. Either way the picture that grounded
	// these Targets is not the picture on screen, and they stop being current immediately.
	//
	// THE NEW PICTURE IS KEPT AS A CANDIDATE, and nothing is concluded from it yet. One
	// frame that differs is equally a navigation, a video and a banner; what separates them
	// is whether it SETTLES. See settledIntoANewScene.
	s.sceneCandidate = &now
	return s.dropLease(res.Reason)
}

func (s *liveSampler) dropLease(why string) ([]directorapi.Element, bool) {
	s.lease, s.leaseWhy, s.leaseDropped = nil, why, why
	return nil, false
}

// settledIntoANewScene reports whether the screen has come to rest on a picture that is not the
// one the previous semantic repair answered.
//
// # Why settling is the discriminator, and a percentage is not
//
// One frame that differs is equally a navigation, a playing video, a rotating banner and a page
// half-loaded. Measured on Epic, a real navigation moved 32.5% of the region and 24.1% on the
// next — but so would artwork on a store page. A threshold that admitted the first would admit
// the others and rebuy an expensive inference forever, which is the expense this whole design
// refuses.
//
// What separates them is whether the new picture STAYS. `visualstate.Watch` already draws that
// line for actions, in a tight loop of captures 120ms apart; this draws the same line across
// ordinary acquisitions, where the captures are already being taken. A navigation is stable by
// the next reading. A video is not.
//
// So: the frame that ended the lease is kept as a candidate and asserts nothing. If the next
// reading matches the candidate, the screen has settled somewhere new. If it does not, the
// candidate is replaced and nothing is concluded — churn can do that indefinitely without ever
// buying anything.
//
// It reports a belief about what is CURRENTLY visible and establishes no identity: no Place, no
// screen signature, no graph endpoint. Its only consequence is that one bounded repair becomes
// claimable again.
func (s *liveSampler) settledIntoANewScene(ctx context.Context, window directorapi.Window) bool {
	if s.sceneCandidate == nil || s.rt == nil || s.rt.visual == nil {
		return false
	}
	if s.sceneCandidate.Region != window.Bounds {
		// A different rectangle cannot settle into the same scene, and comparing across
		// two of them is the thing visualstate.Compare refuses outright.
		s.sceneCandidate = nil
		return false
	}
	now, err := s.rt.visual.Capture(ctx, window.Bounds, window)
	if err != nil {
		// Unknown, and unknown concludes nothing. The candidate is dropped rather than
		// held: a scene nobody could photograph has not been shown to have settled.
		s.sceneCandidate = nil
		return false
	}
	res, err := s.rt.visual.Compare(*s.sceneCandidate, now)
	if err != nil {
		s.sceneCandidate = nil
		return false
	}
	switch res.Kind {
	case visualstate.ChangeIdentical, visualstate.ChangeMinor:
		// SETTLED. The picture that replaced the repaired one is still the picture, so
		// this is a screen and not a frame of something moving.
		s.sceneCandidate = nil
		return true
	}
	// STILL MOVING. Replace the candidate and conclude nothing — this arm is what stops a
	// video from rebuying an inference on every acquisition for as long as it plays.
	s.sceneCandidate = &now
	return false
}
