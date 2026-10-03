package verify

import (
	"fmt"
	"strconv"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Verifying a scroll, which asks where the view ENDED UP.
//
// # Why the host's answer is not the verdict
//
// The accessibility host calls SetScrollPercent or ScrollVertical and reports that it returned
// without throwing. That is DELIVERY: the region accepted the call. This package has now learned
// the same lesson three times in different clothes — focus landing on a clicked control, a list
// item's word about its own selection, another program's window title — and the lesson each time
// is that a mechanism accepting an action is not evidence that the requested effect occurred.
//
// So the verdict comes from the world: the position before, against the position after, read from
// the same snapshots every other verification compares.
//
// # Four meanings, four questions
//
//	top         the position IS the top boundary
//	bottom      the position IS the bottom boundary
//	page_down   the position INCREASED, or was already at the bottom
//	page_up     the position DECREASED, or was already at the top
//
// The already-at-the-boundary cases succeed, and that is not leniency. Somebody at the bottom of a
// document who asks to go down a page has what they asked for; reporting a failure would invite
// them to ask again, and `failed` is precisely the word that invites that.

// scrollTolerance is how close to a boundary counts as being at it.
//
// Not zero, because a provider reporting a percentage for a pixel-scrolled view lands on 99.6
// rather than 100, and a verdict that called that a failure would be wrong about every long
// document. Not large either: 2% of a seventy-line document is a line and a half.
const scrollTolerance = 2.0

// verifyScroll judges a view movement by the position the world reports.
func (v *Verifier) verifyScroll(where directorapi.ScrollWhere, target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	if !where.Known() {
		// A destination nothing implements. The executor refuses these and the planner
		// refuses them earlier still, so reaching here means a verdict is being asked for
		// about an action that should never have run — and inventing one would describe
		// something nobody performed.
		return directorapi.VerificationResult{
			Reason: fmt.Sprintf("%q is not somewhere Marco can scroll a view", where),
		}
	}

	was, hadBefore := scrollPercent(before, target.ElementID)
	now, hasAfter := scrollPercent(after, target.ElementID)

	// A POSITION NOBODY COULD READ VERIFIES NOTHING — and does not deny it either.
	//
	// Inconclusive rather than failed, because the call may well have landed. It carries
	// delivery evidence so the retry guard will not repeat it: repeating a delivered page
	// scroll moves two pages, and this is exactly the case with nothing to tell apart "it
	// did not happen" from "I cannot see whether it did". See [Delivered].
	if !hadBefore || !hasAfter {
		return directorapi.VerificationResult{
			Inconclusive: true,
			Evidence: []directorapi.Evidence{{
				Kind: EvidenceScrollAccepted, Observed: true, Weight: WeightDelivered,
				Detail: "the region accepted the scroll, which shows it was delivered and " +
					"not that the view moved",
				Source: directorapi.SourceAccessibility,
			}},
			Reason: fmt.Sprintf("%q does not report how far it is scrolled, so the view "+
				"could not be checked", target.Label),
		}
	}

	switch where {
	case directorapi.ScrollTop:
		return scrollVerdict(now <= scrollTolerance, now,
			"the view is at the top", "the view is at %.0f%% and not at the top")
	case directorapi.ScrollBottom:
		return scrollVerdict(now >= 100-scrollTolerance, now,
			"the view is at the bottom", "the view is at %.0f%% and not at the bottom")

	case directorapi.ScrollPageDown:
		if was >= 100-scrollTolerance {
			return alreadyThere(now, "bottom")
		}
		return scrollVerdict(now > was+scrollTolerance, now,
			"the view moved down", "the view is still at %.0f%%")
	case directorapi.ScrollPageUp:
		if was <= scrollTolerance {
			return alreadyThere(now, "top")
		}
		return scrollVerdict(now < was-scrollTolerance, now,
			"the view moved up", "the view is still at %.0f%%")
	}
	return directorapi.VerificationResult{Reason: "the scroll had no destination to check"}
}

// scrollVerdict is the shared shape, so every arm weighs its evidence the same way.
//
// The weight is high because this is the application's own account of its own view, read from the
// world rather than from the thing that moved it — the strongest kind of evidence this package
// gets. It is not 1.0 because no single reading here ever is.
func scrollVerdict(ok bool, now float64, success, failure string) directorapi.VerificationResult {
	ev := []directorapi.Evidence{{
		Kind: "scroll_position", Observed: ok, Weight: 0.9,
		Detail: fmt.Sprintf("the region reports it is %.0f%% scrolled", now),
		Source: directorapi.SourceAccessibility,
	}}
	if ok {
		return directorapi.VerificationResult{
			Success: true, Confidence: 0.9, Evidence: ev, Reason: success,
		}
	}
	return directorapi.VerificationResult{
		Confidence: 0.9, Evidence: ev, Reason: fmt.Sprintf(failure, now),
	}
}

// alreadyThere is the boundary case, and it SAYS it is the boundary case.
//
// "Verified" over a view that did not move reads as a bug unless the sentence explains why, and
// the explanation is the whole content of this verdict: there was nowhere further to go.
func alreadyThere(now float64, edge string) directorapi.VerificationResult {
	return directorapi.VerificationResult{
		Success: true, Confidence: 0.9,
		Evidence: []directorapi.Evidence{{
			Kind: "scroll_at_boundary", Observed: true, Weight: 0.9,
			Detail: fmt.Sprintf("the region was already at the %s (%.0f%%)", edge, now),
			Source: directorapi.SourceAccessibility,
		}},
		Reason: fmt.Sprintf("the view was already at the %s, so there was nowhere further to go",
			edge),
	}
}

// scrollPercent reads a region's position out of a world.
//
// Absent means the provider reported no vertical scrolling, which is NOT zero — a region at the
// top and a region nobody can read are different facts, and only one of them can verify a scroll.
func scrollPercent(w directorapi.WorldState, id directorapi.ElementID) (float64, bool) {
	el, ok := w.Element(id)
	if !ok || el == nil {
		return 0, false
	}
	raw, ok := el.Attributes[directorapi.StateScrollPercent]
	if !ok {
		return 0, false
	}
	switch v := raw.(type) {
	case float64:
		return v, true
	case string:
		p, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return p, true
	}
	return 0, false
}
