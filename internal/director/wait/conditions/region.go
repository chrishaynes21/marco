package conditions

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/wait/evaluation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Conditions about a region's CHANGE rather than about a control's state.
//
// These are the ones a World State cannot answer. "Has this stopped changing" is a
// question about consecutive observations of pixels, not about the tree — and it is
// precisely the question that a page mid-navigation answers differently from a page
// that has finished, while the accessibility tree says the same thing for both.
//
// They take a Sample rather than a WorldState, which is what keeps the engine honest:
// a caller that has no region watcher cannot evaluate them at all, and gets Unknown
// rather than a guess.

// Sample is what a region looked like across one evaluation.
//
// Deliberately not a fingerprint. The wait layer never learns what a fingerprint is —
// the visual provider owns that — so what arrives here is the CONCLUSION plus enough
// detail to explain it.
type Sample struct {
	// Observed is false when no capture could be taken. The Unknown gate: a region
	// nobody looked at has not stabilised, and it has not changed either.
	Observed bool
	// Changed and StillChanging are the visual provider's verdict.
	Changed       bool
	StillChanging bool
	// Identical is pixel-for-pixel equality, which is stronger than !Changed.
	Identical bool
	Detail    string
	Region    directorapi.Rect
}

// RegionSampler produces a sample for a region. Supplied by the engine's caller; a
// Director with no capture layer supplies none, and region conditions then evaluate
// Unknown forever rather than pretending.
type RegionSampler func(region directorapi.Rect) Sample

// RegionStable waits for a region to stop changing.
//
// The condition that replaces "sleep 500ms after clicking". A settle delay is a guess
// about how long a menu takes to render; this is the observation that it has.
//
// STABILITY IS A RUN, NOT AN INSTANT. One quiet observation is not stability — an
// animation between frames looks identical for a moment, and a page part-way through
// loading is quiet between paints. The engine requires N consecutive satisfied
// evaluations (WaitOptions.StableObservations), which is why this returns Satisfied for
// a single quiet sample and lets the engine count.
type RegionStable struct {
	Region directorapi.Rect
	// Sampler is how the region is looked at.
	Sampler RegionSampler
}

func (c RegionStable) ID() ID { return IDRegionStable }
func (c RegionStable) Description() string {
	return fmt.Sprintf("until the region %s stops changing", rectText(c.Region))
}

func (c RegionStable) Evaluate(directorapi.WorldState) evaluation.Result {
	if c.Sampler == nil {
		return evaluation.Unknowable("no region watcher is wired, so whether this region " +
			"is changing cannot be answered")
	}
	s := c.Sampler(c.Region)
	if !s.Observed {
		return evaluation.Unknowable("the region could not be captured, so its stability " +
			"cannot be answered")
	}
	ev := regionEvidence(c.Region, s)

	switch {
	case s.StillChanging:
		return evaluation.Deny(0.9, "the region is still changing: "+s.Detail, ev)
	case s.Changed:
		// It changed since the last look, so it is not yet stable — but the change has
		// stopped mid-flight, so this is one observation of quiet rather than none.
		return evaluation.Deny(0.7, "the region changed since the last look: "+s.Detail, ev)
	case s.Identical:
		return evaluation.Satisfy(0.95, "the region is pixel-for-pixel what it was", ev)
	}
	return evaluation.Satisfy(0.8, "only rendering noise differs: "+s.Detail, ev)
}

// RegionChanged waits for a region to visibly change.
type RegionChanged struct {
	Region  directorapi.Rect
	Sampler RegionSampler
}

func (c RegionChanged) ID() ID { return IDRegionChanged }
func (c RegionChanged) Description() string {
	return fmt.Sprintf("until the region %s changes", rectText(c.Region))
}

func (c RegionChanged) Evaluate(directorapi.WorldState) evaluation.Result {
	if c.Sampler == nil {
		return evaluation.Unknowable("no region watcher is wired")
	}
	s := c.Sampler(c.Region)
	if !s.Observed {
		return evaluation.Unknowable("the region could not be captured")
	}
	ev := regionEvidence(c.Region, s)
	if s.Changed || s.StillChanging {
		return evaluation.Satisfy(0.85, "the region changed: "+s.Detail, ev)
	}
	return evaluation.Deny(0.85, "the region has not changed", ev)
}

// RegionStillChanging is satisfied WHILE a region is in flight.
//
// Inverted on purpose, and it exists for one caller: verification, which needs to know
// that it should defer rather than declare a result. Waiting FOR "still changing" is
// not something a plan does; asking whether it holds is.
type RegionStillChanging struct {
	Region  directorapi.Rect
	Sampler RegionSampler
}

func (c RegionStillChanging) ID() ID { return IDRegionStillChanging }
func (c RegionStillChanging) Description() string {
	return fmt.Sprintf("while the region %s is still changing", rectText(c.Region))
}

func (c RegionStillChanging) Evaluate(directorapi.WorldState) evaluation.Result {
	if c.Sampler == nil {
		return evaluation.Unknowable("no region watcher is wired")
	}
	s := c.Sampler(c.Region)
	if !s.Observed {
		return evaluation.Unknowable("the region could not be captured")
	}
	ev := regionEvidence(c.Region, s)
	if s.StillChanging {
		return evaluation.Satisfy(0.9, "the region is still changing: "+s.Detail, ev)
	}
	return evaluation.Deny(0.9, "the region has settled", ev)
}

// ── verification ──────────────────────────────────────────────────────────────

// Verifier re-runs the existing verification against a fresh world.
type Verifier func(w directorapi.WorldState) directorapi.VerificationResult

// VerificationSatisfied waits for an action's expected outcome to become verifiable.
//
// The point of it: an interface that is slow to update is not an interface where the
// action failed. Rather than retrying — which double-applies a non-idempotent action —
// or declaring Unverified immediately, a plan can wait for the evidence to arrive and
// verify again. The ACTION is not repeated; only the verification is.
type VerificationSatisfied struct {
	// What describes the action being verified, for the explanation.
	What string
	Run  Verifier
}

func (c VerificationSatisfied) ID() ID { return IDVerificationSatisfied }
func (c VerificationSatisfied) Description() string {
	if c.What == "" {
		return "until the action can be verified"
	}
	return "until " + c.What + " can be verified"
}

func (c VerificationSatisfied) Evaluate(w directorapi.WorldState) evaluation.Result {
	if c.Run == nil {
		return evaluation.Unknowable("no verifier is wired")
	}
	res := c.Run(w)
	ev := evaluation.EvidenceReference{Kind: "verification", Detail: res.Reason}
	switch {
	case res.Success:
		return evaluation.Satisfy(res.Confidence, "verified: "+res.Reason, ev)
	case res.Inconclusive:
		// Inconclusive is UNKNOWN, not unsatisfied. The verifier is saying it could not
		// tell — which is exactly the state that should keep a wait going rather than
		// ending it with a negative answer.
		return evaluation.Unknowable("verification is inconclusive: "+res.Reason, ev)
	}
	return evaluation.Deny(res.Confidence, "not verified yet: "+res.Reason, ev)
}

func regionEvidence(r directorapi.Rect, s Sample) evaluation.EvidenceReference {
	return evaluation.EvidenceReference{
		Kind: "region", Detail: rectText(r) + " — " + s.Detail,
	}
}

func rectText(r directorapi.Rect) string {
	return fmt.Sprintf("(%d,%d %dx%d)", r.X, r.Y, r.Width, r.Height)
}
