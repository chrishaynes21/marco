package main

// A -> unobserved -> A IS NOT A -> B -> A.
//
// The whole investigation turns on telling those apart, and the cost of getting it wrong is a
// human confirmation carried across a real navigation — a person's "yes, that's the Play menu"
// applied to a screen they never saw. So the classifier is tested on both, and on the ways each
// can be faked.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// walk builds a timeline from a compact description: one entry per look.
func walk(entries ...walkLook) []walkLook {
	for i := range entries {
		entries[i].look = i + 1
		entries[i].atMS = int64(i * 100)
		if entries[i].proposals == 0 && !entries[i].missing {
			entries[i].proposals = 120
		}
		if !entries[i].missing {
			entries[i].front = true
		}
		if entries[i].generation == 0 {
			entries[i].generation = 1
		}
	}
	return entries
}

func onA(epoch uint64) walkLook {
	return walkLook{returned: "visual_1", standing: "visual_1", epoch: epoch}
}

var sameArr = observe.VisualArrangement{Cells: map[string]int{"1,1": 9}, Total: 9}

// A -> UNOBSERVED -> A: one visit with a gap in what was SEEN.
func TestATransientLossIsNotANavigation(t *testing.T) {
	tl := walk(
		onA(1), onA(1), onA(1),
		// the identity goes and the stabiliser holds
		walkLook{returned: observe.VisualScreenUnobserved, standing: "visual_1", epoch: 2},
		onA(3), onA(3),
	)
	g := classifyGap("screen_a", tl, 3, 4, "visual_1", sameArr, sameArr)
	if g.kind != gapTransient {
		t.Fatalf("classified %q, want %q (%s)", g.kind, gapTransient, g.because)
	}
	if !g.heldThroughout {
		t.Error("the stabiliser is recorded as having let go")
	}
	if !g.recoverable {
		t.Errorf("a transient loss with the arrangement agreeing was not bridgeable: %s",
			g.because)
	}
}

// A -> B -> A: two visits, and the confirmation must not cross it.
func TestANavigationIsNeverBridged(t *testing.T) {
	tl := walk(
		onA(1), onA(1),
		walkLook{returned: "visual_2", standing: "visual_2", epoch: 2},
		walkLook{returned: "visual_2", standing: "visual_2", epoch: 2},
		onA(3),
	)
	// The arrangement agrees PERFECTLY, which is exactly the trap: the same menu looks
	// the same on both visits, and that is not evidence that nothing happened between.
	g := classifyGap("screen_a", tl, 2, 5, "visual_1", sameArr, sameArr)
	if g.kind != gapNavigated {
		t.Fatalf("classified %q, want %q", g.kind, gapNavigated)
	}
	if g.recoverable {
		t.Fatal("a real navigation was marked bridgeable; a person's answer about the " +
			"first visit would have been carried onto the second")
	}
	if len(g.other) != 1 || g.other[0] != "visual_2" {
		t.Errorf("the screen seen in between is %v", g.other)
	}
}

// THE PROPERTY THAT MAKES TWO OF THE RULE'S CONDITIONS REDUNDANT.
//
// A mutation run found that removing "nothing else observed" from the bridging rule changed no
// test, and this is why: the stabiliser only ever returns a screen it has promoted, so a gap
// containing another screen is a gap in which the stabiliser let go. The two conditions cannot
// disagree.
//
// Recorded as a checked property rather than argued in a comment, because the rule keeps both
// and a reader is owed the reason: defence in depth is worth the line when the cost of being
// wrong is a human confirmation applied to a screen nobody looked at.
func TestASeenScreenAlwaysMeansTheStabiliserLetGo(t *testing.T) {
	tl := walk(
		onA(1),
		walkLook{returned: "visual_2", standing: "visual_2", epoch: 2},
		onA(3),
	)
	g := classifyGap("screen_a", tl, 1, 3, "visual_1", sameArr, sameArr)
	if len(g.other) == 0 {
		t.Fatal("the fixture did not observe another screen")
	}
	if g.heldThroughout {
		t.Error("another screen was observed and the stabiliser is recorded as having " +
			"held throughout; the two conditions have come apart and the rule now " +
			"rests on only one of them")
	}
}

// THE AGREEMENT IS REQUIRED, AND IT IS THE PERSON'S ANSWER THAT IS NOT EVIDENCE.
func TestATransientLossStillNeedsTheReturningScreenToAgree(t *testing.T) {
	tl := walk(
		onA(1),
		walkLook{returned: observe.VisualScreenUnobserved, standing: "visual_1", epoch: 2},
		onA(3),
	)
	other := observe.VisualArrangement{Cells: map[string]int{"0,2": 40}, Total: 40}
	g := classifyGap("screen_a", tl, 1, 2, "visual_1", sameArr, other)
	if g.kind != gapTransient {
		t.Fatalf("classified %q", g.kind)
	}
	if g.recoverable {
		t.Error("bridged a gap the returning observation does not agree across")
	}
}

// A WINDOW REPLACEMENT IS NEVER BRIDGED, however quiet the gap looks.
func TestAWindowReplacementIsNeverBridged(t *testing.T) {
	tl := walk(
		onA(1),
		walkLook{returned: observe.VisualScreenUnobserved, standing: "visual_1",
			epoch: 2, generation: 2},
		walkLook{returned: "visual_1", standing: "visual_1", epoch: 3, generation: 2},
	)
	g := classifyGap("screen_a", tl, 1, 3, "visual_1", sameArr, sameArr)
	if g.kind != gapWindow {
		t.Fatalf("classified %q, want %q", g.kind, gapWindow)
	}
	if g.recoverable {
		t.Error("bridged a gap across a window replacement")
	}
}

// THE STABILISER LETTING GO IS A DIFFERENT STORY FROM THE IDENTITY FLICKERING.
func TestAnObserverThatLostTheScreenEntirelyIsNotTransient(t *testing.T) {
	tl := walk(
		onA(1),
		// pixels present, no identity, and the stabiliser gave up too
		walkLook{returned: observe.VisualScreenUnknown, standing: "", epoch: 2},
		onA(3),
	)
	g := classifyGap("screen_a", tl, 1, 2, "visual_1", sameArr, sameArr)
	if g.kind != gapUnsettled {
		t.Fatalf("classified %q, want %q (%s)", g.kind, gapUnsettled, g.because)
	}
	if g.recoverable {
		t.Error("bridged a gap in which the observer held no screen at all")
	}
}

// ONE INTERRUPTION, TWO QUESTIONS — and the second stands or falls with the first.
func TestTheReturnLegOfAnInterruptionIsNotASecondEvent(t *testing.T) {
	first := gapReport{
		label: "screen_a", from: 1, to: 3, kind: gapBlind,
		recoverable: true, heldThroughout: true, agrees: 1,
	}
	second := gapReport{
		label: "screen_a", from: 3, to: 4, kind: gapUnknown,
		heldThroughout: true, agrees: 1,
	}
	gaps := []gapReport{first, second}
	pairReturnLegs(gaps)
	if gaps[1].kind != gapTransient || !gaps[1].recoverable {
		t.Errorf("the return leg is %q, bridgeable=%t", gaps[1].kind, gaps[1].recoverable)
	}

	// AND IT CANNOT RESCUE WHAT THE FIRST LEG COULD NOT.
	gaps = []gapReport{
		{label: "screen_a", from: 1, to: 3, kind: gapNavigated, recoverable: false},
		{label: "screen_a", from: 3, to: 4, kind: gapUnknown, heldThroughout: true, agrees: 1},
	}
	pairReturnLegs(gaps)
	if gaps[1].recoverable {
		t.Error("the return leg of a NAVIGATION was marked bridgeable")
	}
}
