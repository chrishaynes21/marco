package main

// DOES THE BOUNDARY OF A CLICKED CONTROL DEPEND ON WHEN YOU LOOK AT IT?
//
// # The question
//
// [[Experiment-083-the-label-of-the-button-you-click-is-the-one-that-is-missing]] measured the
// text rules and found them worse than the rule Marco already has, for a structural reason:
// clicking a menu entry highlights it, and the highlight is what stops OCR reading its label. The
// owner's reading of that:
//
//	That means a strategy based entirely on the clicked frame may be fundamentally weaker than
//	one using the frames before the click. We should treat the demonstration as a short sequence
//	of visual evidence, not one screenshot.
//
// If the highlight changes how a button RENDERS, it changes what the region proposer segments
// there too — possibly for the better, since a filled bar has edges a flat row does not, and
// possibly for the worse. Either way the boundary offered for one click may not be the same on
// the look it happened as on the look before it, and nothing has ever asked.
//
// So: the same click, asked of every look around it, with the SAME rule the correlator uses.
//
// # What this does not do
//
// It proposes nothing and prefers nothing. It reports, for each press, the boundary the existing
// proposer offers on each look in a window, and whether any of them differs from the one at the
// click. Where a boundary has an independently known answer — Experiment 050's PLAY entry — it
// reports which looks offer it, because "the button is available on the frame before" is a
// measurement and "a smaller rectangle appeared" is not.
//
// The two presses inside PLAY are the regression case the owner named. Everything else in the
// recording is a candidate with no oracle.

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// TemporalWindow is how many looks either side of a press the boundary is asked for.
//
// Not a tuning knob, and the same reasoning as `MaxReadingGap`: this recording ran at 10 fps, so
// five looks is 500ms either side — the timescale a hover highlight and a pressed state live on,
// and long enough to reach frames before the cursor arrived. Every look in the window is printed
// with its own number, so a reader can draw the line somewhere else without rerunning anything.
const TemporalWindow = 5

// smallestHolding is the smallest proposal containing a point.
//
// The SAME rule as `today` in button.go — the correlator's rule — and not a second one: with the
// click look's own proposals it must return the click look's own answer, which
// TestTheTemporalRuleAtTheClickLookIsExactlyTodaysAnswer holds it to.
func smallestHolding(regions []observe.Region, at observe.PointerAt) (observe.Region, bool) {
	var best observe.Region
	found := false
	for _, r := range regions {
		if !holds(r, at) {
			continue
		}
		if !found || boxArea(r) < boxArea(best) {
			best, found = r, true
		}
	}
	return best, found
}

// lookBoundary is what one look offered for one click.
type lookBoundary struct {
	look  int
	where observe.Region
	// held is false when nothing on that look contained the click, which is an absence and
	// is printed as one rather than as a rectangle of no size.
	held bool
	// readable is false when the frame's pixels could not be obtained at all. Kept apart
	// from `held` for the reason proposeall.go gives: a black frame the proposer correctly
	// found nothing in is not a capture gap.
	readable bool
	// isOracle is true when this boundary is the known button, by the identity relation.
	isOracle bool
}

// temporalEvidence is one demonstrated click asked of the looks around it.
type temporalEvidence struct {
	look    int
	at      observe.PointerAt
	window  []lookBoundary
	atClick lookBoundary
	// tighter is the smallest boundary in the window that is NOT the same control as the
	// one at the click. The thing a pre-click frame would have to offer for temporal
	// evidence to be worth anything here.
	tighter         lookBoundary
	hasTighter      bool
	inOracle        bool // the click landed inside the known button
	oracleLooks     int  // looks in the window offering it
	oracleOfHowMany int
}

// temporalBoundaries asks one click of every look in its window.
func temporalBoundaries(look int, at observe.PointerAt, regions [][]observe.Region,
	readable []bool) temporalEvidence {

	e := temporalEvidence{look: look, at: at, inOracle: holds(playButton, at)}
	for i := look - TemporalWindow; i <= look+TemporalWindow; i++ {
		if i < 0 || i >= len(regions) {
			continue
		}
		b := lookBoundary{look: i, readable: readable[i]}
		if b.readable {
			b.where, b.held = smallestHolding(regions[i], at)
			b.isOracle = b.held && observe.SameRegion(b.where, playButton)
		}
		e.window = append(e.window, b)
		if i == look {
			e.atClick = b
		}
		if e.inOracle && b.readable {
			e.oracleOfHowMany++
			if b.isOracle {
				e.oracleLooks++
			}
		}
	}
	// THE ONLY THING THAT WOULD JUSTIFY LOOKING AT ANOTHER FRAME: a boundary that is
	// smaller than the click look's AND a different control from it. Smaller alone is not
	// enough — two readings of one button differ in width by a factor of two in this very
	// recording, and calling that an improvement is the mistake this file exists to avoid.
	for _, b := range e.window {
		if !b.held || b.look == look {
			continue
		}
		if e.atClick.held {
			if observe.SameRegion(b.where, e.atClick.where) {
				continue
			}
			if boxArea(b.where) >= boxArea(e.atClick.where) {
				continue
			}
		}
		if !e.hasTighter || boxArea(b.where) < boxArea(e.tighter.where) {
			e.tighter, e.hasTighter = b, true
		}
	}
	return e
}

// temporalVerdict is the answer for one press, and the four are exhaustive.
func temporalVerdict(e temporalEvidence) string {
	switch {
	case !e.atClick.held && !e.hasTighter:
		return "NOTHING  — no look in the window offered anything containing the click"
	case !e.atClick.held:
		return fmt.Sprintf("APPEARS  — nothing held the click on its own look; look %d "+
			"offers %.1f%%x%.1f%%", e.tighter.look,
			e.tighter.where.Width*100, e.tighter.where.Height*100)
	case e.hasTighter:
		when := "AFTER the press, which may already be the screen the press caused"
		if tighterIsBefore(e) {
			when = "BEFORE the press, so certainly the same screen"
		}
		out := fmt.Sprintf("TIGHTER  — look %d, %s, offers %.1f%%x%.1f%% against the "+
			"click look's %.1f%%x%.1f%%", e.tighter.look, when,
			e.tighter.where.Width*100, e.tighter.where.Height*100,
			e.atClick.where.Width*100, e.atClick.where.Height*100)
		if bothAreTheKnownButton(e) {
			out += "\n               but the relation calls BOTH of them the known " +
				"button while calling them different controls from each other, " +
				"because mutual centre containment is not transitive"
		}
		return out
	default:
		return fmt.Sprintf("STABLE   — across %d look(s) no other look offers a smaller, "+
			"different boundary than the click look's %.1f%%x%.1f%%", len(e.window),
			e.atClick.where.Width*100, e.atClick.where.Height*100)
	}
}

// clickLookIsSmallest reports whether the click's OWN look offered the smallest boundary around.
//
// The hypothesis under test says the clicked frame is the weak one, because the highlight is
// what stops OCR reading the label. The region proposer is a different reader of the same pixels,
// and a filled bar has edges a flat row of text does not, so the same highlight may be what
// MAKES the boundary. This is that question as a count.
func clickLookIsSmallest(e temporalEvidence) bool {
	if !e.atClick.held {
		return false
	}
	for _, b := range e.window {
		// No need to skip the click look itself: its own boundary is never strictly
		// smaller than itself, and a mutation run proved the guard that said so was
		// unreachable.
		if b.held && boxArea(b.where) < boxArea(e.atClick.where) {
			return false
		}
	}
	return true
}

// tighterIsBefore reports whether the tighter boundary came from before the press.
//
// The distinction is not cosmetic. A click on a menu entry CAUSES a transition, so a look after
// it may be showing a different screen, and a smaller rectangle found there may belong to a
// control that did not exist when the press happened. Only a look BEFORE the press is certain to
// be the same screen the person was looking at when they chose.
func tighterIsBefore(e temporalEvidence) bool { return e.hasTighter && e.tighter.look < e.look }

// bothAreTheKnownButton reports the case where the identity relation contradicts itself.
//
// `SameRegion` is mutual centre containment and is therefore NOT TRANSITIVE: two readings can
// each be the same control as a third while not being the same control as each other. When that
// happens on a press with an oracle it is measurable rather than theoretical, and it is reported
// rather than resolved, because a "different control" verdict resting on a non-transitive
// relation is not evidence of a different control.
func bothAreTheKnownButton(e temporalEvidence) bool {
	return e.hasTighter && e.atClick.isOracle && e.tighter.isOracle
}
