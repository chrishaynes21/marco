package main

// THE BOUNDARY MEASURED ACROSS LOOKS, AND WHAT THE WINDOW DID AND DID NOT RESCUE.
//
// [[Experiment-084-the-highlight-that-hides-the-label-draws-the-box]] asked the owner's question
// -- whether the frames before a click carry a button boundary the clicked frame missed -- of
// `rl-freeplay-1`, the one recording with frames and click positions. The numbers below are
// copied from that run.
//
// The answer is one press of seventeen, against four where the click's own look was the smallest
// boundary of its whole window. The tests are here because that is the opposite of the
// hypothesis, and a result like that should fail loudly if the measurement changes under it.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// windowOf builds per-look proposals with every look readable, for the looks given.
func windowOf(at map[int][]observe.Region, n int) ([][]observe.Region, []bool) {
	regions := make([][]observe.Region, n)
	readable := make([]bool, n)
	for i := range readable {
		readable[i] = true
	}
	for look, rs := range at {
		regions[look] = rs
	}
	return regions, readable
}

// THE TEMPORAL RULE AND THE CORRELATOR'S RULE ARE ONE RULE.
//
// `smallestHolding` is a second implementation of `today`, and a second implementation that
// drifted would make every number in the experiment a measurement of something else. Given the
// click look's own proposals it must return the click look's own answer.
func TestTheTemporalRuleAtTheClickLookIsExactlyTodaysAnswer(t *testing.T) {
	click := clickAt(0.065, 0.444)
	regions := []observe.Region{
		box(0.014, 0.433, 0.086, 0.037),
		box(0.007, 0.433, 0.165, 0.041),
		box(0.000, 0.200, 0.617, 0.578),
	}
	today := buttonEvidence(73, click, regions, nil, 73).today
	where, held := smallestHolding(regions, click)
	if !held || !today.found {
		t.Fatalf("held=%v today.found=%v", held, today.found)
	}
	if !sameRegionExactly(where, today.where) {
		t.Fatalf("the temporal rule answers %v and the correlator's rule answers %v; they "+
			"are meant to be the same rule", where, today.where)
	}

	// AND IT PICKS THE SMALLEST, not the first. Without this the test would pass on a
	// rule that returned whatever it saw first, which on a sorted input looks identical.
	reversed := []observe.Region{regions[2], regions[1], regions[0]}
	again, _ := smallestHolding(reversed, click)
	if !sameRegionExactly(again, regions[0]) {
		t.Errorf("reordering the proposals changed the answer to %v", again)
	}
	// AND A CLICK OUTSIDE EVERYTHING IS AN ABSENCE, not a rectangle of no size.
	if _, held := smallestHolding(regions, clickAt(0.99, 0.99)); held {
		t.Error("a click outside every proposal reports a boundary")
	}
}

// PRESS 7: THE CLICK'S OWN LOOK IS THE ONLY LOOK THAT HAS THE BUTTON.
//
// Looks 330-334 offer an 83% panel; look 335, the press itself, offers 20.0%x3.1%. Four presses
// of seventeen look like this, and they are the measured counter-case to "the clicked frame is
// the weak one": the highlight that stops OCR reading the label is the same highlight that gives
// the proposer an edge to segment.
func TestTheClickLookCanBeTheOnlyLookThatHoldsTheButton(t *testing.T) {
	click := clickAt(0.467, 0.551)
	panel := box(0.170, 0.000, 0.830, 0.956)
	button := box(0.400, 0.544, 0.200, 0.031)
	regions, readable := windowOf(map[int][]observe.Region{
		330: {panel}, 331: {panel}, 332: {panel}, 333: {panel}, 334: {panel},
		335: {panel, button},
		336: {panel}, 337: {panel}, 338: {panel}, 339: {panel}, 340: {panel},
	}, 341)

	e := temporalBoundaries(335, click, regions, readable)
	if !e.atClick.held || !sameRegionExactly(e.atClick.where, button) {
		t.Fatalf("the click look's boundary is %v, not the button %v",
			e.atClick.where, button)
	}
	if e.hasTighter {
		t.Errorf("a tighter boundary was found at look %d, and the button exists on no "+
			"other look in this window", e.tighter.look)
	}
	if !clickLookIsSmallest(e) {
		t.Error("the click look holds the smallest boundary of the window and is not " +
			"counted as doing so")
	}
	if v := temporalVerdict(e); !strings.Contains(v, "STABLE") {
		t.Errorf("verdict %q", v)
	}
}

// PRESS 16: THE ONE PRESS AN EARLIER FRAME RESCUED, AND IT IS REPORTED AS BEFORE.
//
// The click look offered 100.0%x46.1% -- half the window. Look 754, four looks earlier, offered
// 20.0%x3.1%. One of seventeen, and the only direction that is free of the transition the press
// itself causes.
func TestAnEarlierLookCanHoldAButtonTheClickLookMissed(t *testing.T) {
	click := clickAt(0.474, 0.569)
	slab := box(0.000, 0.469, 1.000, 0.461)
	button := box(0.400, 0.544, 0.200, 0.031)
	regions, readable := windowOf(map[int][]observe.Region{
		753: {slab}, 754: {slab, button}, 755: {slab, button},
		756: {slab, button}, 757: {slab, button},
		758: {slab},
		759: {slab}, 760: {slab}, 761: {slab}, 762: {slab}, 763: {slab},
	}, 764)

	e := temporalBoundaries(758, click, regions, readable)
	if !e.hasTighter {
		t.Fatal("no earlier look offered the button")
	}
	if !tighterIsBefore(e) {
		t.Errorf("the rescue came from look %d and the press was on 758, which is before "+
			"it", e.tighter.look)
	}
	if clickLookIsSmallest(e) {
		t.Error("the click look is counted as the smallest of its window while an " +
			"earlier look holds a button inside it")
	}
	v := temporalVerdict(e)
	for _, want := range []string{"TIGHTER", "BEFORE", "same screen"} {
		if !strings.Contains(v, want) {
			t.Errorf("the verdict does not say %q: %s", want, v)
		}
	}

	// AND A RESCUE FROM AFTER THE PRESS IS NOT REPORTED AS THE SAME THING. Five of the
	// six rescues in the recording are later looks, and a later look may already be the
	// screen the press caused -- which is a different control, not a better boundary.
	later, lreadable := windowOf(map[int][]observe.Region{
		753: {slab}, 754: {slab}, 755: {slab}, 756: {slab}, 757: {slab},
		758: {slab},
		759: {slab, button}, 760: {slab}, 761: {slab}, 762: {slab}, 763: {slab},
	}, 764)
	after := temporalBoundaries(758, click, later, lreadable)
	if !after.hasTighter || tighterIsBefore(after) {
		t.Fatalf("a rescue from look %d is reported as being before look 758",
			after.tighter.look)
	}
	if v := temporalVerdict(after); !strings.Contains(v, "AFTER") ||
		!strings.Contains(v, "the press caused") {
		t.Errorf("a later rescue does not carry the transition caveat: %s", v)
	}
}

// AND THE RELATION CONTRADICTS ITSELF, WHICH THE REPORT HAS TO SAY.
//
// Press 10, measured: the click look offered 18.4%x4.1% and look 540 offered 4.7%x3.1%. Both are
// `SameRegion` to Experiment 050's PLAY button. Neither is `SameRegion` to the other, because
// mutual centre containment is not transitive. So "a different control" from that relation is not
// evidence of a different control at this scale.
func TestTwoBoundariesCanBothBeTheKnownButtonAndNotEachOther(t *testing.T) {
	click := clickAt(0.083, 0.462)
	wide := box(0.007, 0.433, 0.184, 0.041)
	narrow := box(0.039, 0.437, 0.047, 0.031)

	// THE PREMISE, ASSERTED RATHER THAN ASSUMED: without this the test would pass on a
	// pair that the relation simply agreed about.
	if !observe.SameRegion(wide, playButton) || !observe.SameRegion(narrow, playButton) {
		t.Fatal("the fixture pair is not both the known button")
	}
	if observe.SameRegion(wide, narrow) {
		t.Fatal("the fixture pair is the same control, so there is no contradiction here")
	}

	regions, readable := windowOf(map[int][]observe.Region{
		534: {wide}, 535: {wide}, 536: {wide}, 537: {wide}, 538: {wide},
		539: {wide},
		540: {wide, narrow}, 541: {wide}, 542: {wide}, 543: {wide}, 544: {wide},
	}, 545)
	e := temporalBoundaries(539, click, regions, readable)
	if !e.inOracle {
		t.Fatal("the click is not inside the known button")
	}
	if !bothAreTheKnownButton(e) {
		t.Fatalf("hasTighter=%v atClick.isOracle=%v tighter.isOracle=%v",
			e.hasTighter, e.atClick.isOracle, e.tighter.isOracle)
	}
	// AND ONE OF THE TWO BEING THE BUTTON IS NOT A CONTRADICTION. If the click look's
	// boundary is the known button and the tighter one is not, the relation is simply
	// right about both and there is nothing for it to have contradicted itself about.
	tiny := box(0.078, 0.458, 0.012, 0.008)
	if observe.SameRegion(tiny, playButton) || observe.SameRegion(tiny, wide) {
		t.Fatal("the fixture's small rectangle is one control with the button or with " +
			"the click look's boundary, so it cannot stand for a non-oracle rescue")
	}
	oneOf := temporalBoundaries(0, click, [][]observe.Region{{wide}, {wide, tiny}},
		[]bool{true, true})
	if !oneOf.hasTighter || !oneOf.atClick.isOracle || oneOf.tighter.isOracle {
		t.Fatalf("hasTighter=%v atClick.isOracle=%v tighter.isOracle=%v; the fixture is "+
			"meant to match the oracle at the click and nowhere else",
			oneOf.hasTighter, oneOf.atClick.isOracle, oneOf.tighter.isOracle)
	}
	if bothAreTheKnownButton(oneOf) {
		t.Error("one boundary matching the known button is reported as the relation " +
			"contradicting itself")
	}
	if v := temporalVerdict(e); !strings.Contains(v, "not transitive") {
		t.Errorf("the verdict calls them different controls without saying the relation "+
			"cannot support that: %s", v)
	}

	// AND AN ORDINARY TIGHTER PRESS DOES NOT CARRY THE CAVEAT. A report that printed it
	// every time would be saying nothing.
	// Off-centre on purpose: a small rectangle in the MIDDLE of a full-screen one is
	// `SameRegion` to it, because each one's centre is inside the other. That is the same
	// non-transitivity this test is about, and it would make the fixture say nothing.
	ordinary := temporalBoundaries(0, clickAt(0.15, 0.15), [][]observe.Region{
		{box(0.0, 0.0, 1.0, 1.0)}, {box(0.0, 0.0, 1.0, 1.0), box(0.1, 0.1, 0.1, 0.1)},
	}, []bool{true, true})
	if !ordinary.hasTighter {
		t.Fatal("the second look's smaller rectangle was not found")
	}
	if bothAreTheKnownButton(ordinary) {
		t.Error("a press nowhere near the known button carries the non-transitivity note")
	}
}

// A LOOK WHOSE PIXELS NEVER ARRIVED IS NOT A LOOK THAT FOUND NOTHING.
//
// proposeall.go's own reason: look 643 of this recording was diagnosed as a capture gap and is a
// black frame the proposer correctly found nothing in. A window that merged the two would report
// a failed capture as evidence about a control's boundary.
func TestAnUnreadableLookIsNotALookThatFoundNothing(t *testing.T) {
	click := clickAt(0.5, 0.5)
	button := box(0.4, 0.4, 0.2, 0.2)
	// The middle look carries PROPOSALS and no pixels. `proposeAll` leaves a failed frame
	// with neither, so a guard keyed on the proposals being empty would look identical
	// here while being keyed on the wrong thing — and a caller handing in a cached
	// proposal set would then read a stale rectangle as evidence about this press.
	regions := [][]observe.Region{{button}, {button}, {button}}
	e := temporalBoundaries(1, click, regions, []bool{true, false, true})

	for _, b := range e.window {
		if !b.readable && b.held {
			t.Errorf("look %d has no pixels and reports a boundary of %v", b.look,
				b.where)
		}
	}

	var unreadable, empty int
	for _, b := range e.window {
		switch {
		case !b.readable:
			unreadable++
		case !b.held:
			empty++
		}
	}
	if unreadable != 1 || empty != 0 {
		t.Fatalf("%d unreadable and %d empty look(s); the middle look's pixels never "+
			"arrived and the other two hold the click", unreadable, empty)
	}
	// AND AN UNREADABLE LOOK CONTRIBUTES NOTHING TO THE ORACLE DENOMINATOR, so a
	// recording with capture gaps is not scored as having offered the wrong boundary.
	inside := temporalBoundaries(1, clickAt(0.07, 0.45),
		[][]observe.Region{{playButton}, {playButton}, {playButton}},
		[]bool{true, false, true})
	if !inside.inOracle {
		t.Fatal("the click is not inside the known button")
	}
	if inside.oracleOfHowMany != 2 || inside.oracleLooks != 2 {
		t.Errorf("the oracle scored %d of %d; one of the three looks had no pixels",
			inside.oracleLooks, inside.oracleOfHowMany)
	}
}

// PRESS 1: SMALLER IS NOT TIGHTER WHEN IT IS THE SAME CONTROL.
//
// Measured. Around the press at 0.065,0.444 the proposer offered 13.0%, 10.9%, 11.2%, 8.6%,
// 3.8% and 9.2% wide — a factor of three — and the relation calls every one of them the same
// control, which is also what the oracle says. So look 74's 3.8%x3.1% is SMALLER than the click
// look's 8.6%x3.7% and is not a better boundary; it is the same button, read tight.
//
// This press is reported STABLE, and a rule that counted any smaller rectangle as a rescue would
// have called eleven of the seventeen presses rescued and learned nothing.
func TestASmallerReadingOfTheSameControlIsNotATighterBoundary(t *testing.T) {
	click := clickAt(0.065, 0.444)
	atPress := box(0.014, 0.433, 0.086, 0.037)
	tight := box(0.048, 0.437, 0.038, 0.031)

	// THE PREMISE, MEASURED: smaller, and the same control. Both halves asserted, because
	// the finding is the pair.
	if boxArea(tight) >= boxArea(atPress) {
		t.Fatal("the fixture's second reading is not smaller")
	}
	if !observe.SameRegion(tight, atPress) {
		t.Fatal("the fixture's two readings are not one control, so there is nothing " +
			"here for the relation to have absorbed")
	}
	if !observe.SameRegion(tight, playButton) || !observe.SameRegion(atPress, playButton) {
		t.Fatal("the fixture's readings are not both the known button")
	}

	regions, readable := windowOf(map[int][]observe.Region{
		68: {atPress}, 69: {atPress}, 70: {atPress}, 71: {atPress}, 72: {atPress},
		73: {atPress},
		74: {atPress, tight}, 75: {atPress}, 76: {atPress}, 77: {atPress}, 78: {atPress},
	}, 79)
	e := temporalBoundaries(73, click, regions, readable)

	if e.hasTighter {
		t.Errorf("look %d's %v is reported as a tighter boundary than the click look's "+
			"%v, and the relation says they are one control",
			e.tighter.look, e.tighter.where, e.atClick.where)
	}
	if v := temporalVerdict(e); !strings.Contains(v, "STABLE") {
		t.Errorf("verdict %q", v)
	}
	// AND THE ORACLE SEES THE WHOLE WINDOW, which is what makes this press the one with
	// an answer rather than a fixture.
	if e.oracleLooks != e.oracleOfHowMany || e.oracleOfHowMany != len(e.window) {
		t.Errorf("the oracle scored %d of %d across %d look(s)",
			e.oracleLooks, e.oracleOfHowMany, len(e.window))
	}
}

// A CLICK NOTHING HELD IS NOT A CLICK WITH THE SMALLEST BOUNDARY.
//
// Neither APPEARS nor NOTHING happened in `rl-freeplay-1` — both are 0 of 17 — so the branches
// that report them are untested by the recording and are the two the tally would get wrong
// first. An absent boundary is not a small one, and counting it as "the click look had the
// smallest boundary of its window" would turn the one number in the report that argues AGAINST
// reaching for another frame into a number that counts failures in its favour.
func TestAClickNoProposalHeldIsNotTheSmallestBoundary(t *testing.T) {
	click := clickAt(0.5, 0.5)
	elsewhere := box(0.0, 0.0, 0.2, 0.2)
	button := box(0.4, 0.4, 0.2, 0.2)

	// NOTHING: no look in the window contains the click.
	none := temporalBoundaries(1, click, [][]observe.Region{
		{elsewhere}, {elsewhere}, {elsewhere},
	}, []bool{true, true, true})
	if none.atClick.held || none.hasTighter {
		t.Fatalf("atClick.held=%v hasTighter=%v", none.atClick.held, none.hasTighter)
	}
	if clickLookIsSmallest(none) {
		t.Error("a click no proposal held is counted as having the smallest boundary " +
			"of its window")
	}
	if v := temporalVerdict(none); !strings.Contains(v, "NOTHING") {
		t.Errorf("verdict %q", v)
	}

	// APPEARS: the click look holds nothing and a neighbour holds a button.
	appears := temporalBoundaries(1, click, [][]observe.Region{
		{elsewhere}, {elsewhere}, {elsewhere, button},
	}, []bool{true, true, true})
	if appears.atClick.held || !appears.hasTighter {
		t.Fatalf("atClick.held=%v hasTighter=%v",
			appears.atClick.held, appears.hasTighter)
	}
	if clickLookIsSmallest(appears) {
		t.Error("a click look that held nothing is counted as the smallest while a " +
			"neighbour holds a button")
	}
	if v := temporalVerdict(appears); !strings.Contains(v, "APPEARS") {
		t.Errorf("verdict %q", v)
	}
}
