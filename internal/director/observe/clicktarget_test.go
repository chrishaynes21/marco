package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A press keeps EVERY region that contained it, smallest first.
//
// The owner's rule, and the reason for it:
//
//	Preserve all candidate clicked regions when a mouse press overlaps multiple proposals.
//	[…] Choosing the smallest containing region might be a useful hypothesis, but it
//	shouldn't silently become a verified target identity.
//
// Five of seventeen presses in the real demonstration landed inside more than one region. A
// press that kept only its favourite would have thrown away the thing that could later
// contradict it.
//
// Deleting the sort, or the ambiguity flag, must fail this.
func TestAPressKeepsEveryRegionThatContainedIt(t *testing.T) {
	var c observe.ClickTargets
	// A panel, a row inside it, and the text inside that. Offered worst-first on purpose:
	// the ordering must come from the rule, not from the caller.
	regions := []observe.Region{
		{X: 0.0, Y: 0.0, Width: 1.0, Height: 1.0},    // the window
		{X: 0.1, Y: 0.4, Width: 0.4, Height: 0.2},    // a panel
		{X: 0.12, Y: 0.45, Width: 0.3, Height: 0.05}, // a row
		{X: 0.13, Y: 0.46, Width: 0.1, Height: 0.03}, // its text
		{X: 0.7, Y: 0.1, Width: 0.2, Height: 0.1},    // somewhere else entirely
	}
	obs := c.Press(10, observe.PointerAt{X: 0.15, Y: 0.47}, "visual_1", observe.Proposed(regions, 0, 0, 0))

	if len(obs.Candidates) != 4 {
		t.Fatalf("%d candidate(s) kept, want 4: the window, the panel, the row and its "+
			"text all contain that point, and the fifth does not. %+v",
			len(obs.Candidates), obs.Candidates)
	}
	for i := 1; i < len(obs.Candidates); i++ {
		a := obs.Candidates[i-1].Width * obs.Candidates[i-1].Height
		b := obs.Candidates[i].Width * obs.Candidates[i].Height
		if a > b {
			t.Errorf("candidate %d is larger than %d; the most specific thing "+
				"containing a press is the likeliest control and must be first",
				i-1, i)
		}
	}
	if !obs.Ambiguous {
		t.Error("a press inside four regions is not marked ambiguous, so the target " +
			"below reads as the only possible answer")
	}
	if obs.Target == "" {
		t.Error("a press that landed on something was given no target identity")
	}
	if !obs.Landed() {
		t.Error("a press with four candidates reports that it landed on nothing")
	}
}

// A press on nothing the proposer found is BLIND, and never gets a target.
//
// It can never be re-grounded: there is no region to find again on a later screen, so a route
// cannot be built over that step however reliable the transition after it. Counted, and not
// given an identity that would make it look learnable.
func TestAPressOnNothingIsBlindAndNeverATarget(t *testing.T) {
	var c observe.ClickTargets
	obs := c.Press(5, observe.PointerAt{X: 0.5, Y: 0.5}, "visual_1",
		observe.Proposed([]observe.Region{{X: 0.0, Y: 0.0, Width: 0.1, Height: 0.1}}, 0, 0, 0))
	if obs.Landed() || obs.Target != "" {
		t.Errorf("a press on bare background got target %q and %d candidate(s)",
			obs.Target, len(obs.Candidates))
	}
	c.Close()
	e := c.Evidence()
	if e.Blind != 1 {
		t.Errorf("%d blind press(es) counted, want 1", e.Blind)
	}
	// AND IT IS NOT ALSO FILED AS INERT. One press counted twice in a report whose whole
	// purpose is to add up would be worse than not counting it at all.
	if len(e.Inert) != 0 {
		t.Errorf("a blind press was also filed as inert: %+v", e.Inert)
	}
}

// Clicking the same control twice on one screen is the same target; on another screen it is not.
//
// Identity reuse is the point of the layer — "repeated clicks can refer to the same visual
// control" — and the screen scope is what keeps it honest: the third row of one menu and the
// third row of another are two controls at one place.
//
// Deleting the reuse search, or the screen test, must fail this.
func TestTheSameControlTwiceIsOneTargetAndAnotherScreenIsNot(t *testing.T) {
	var c observe.ClickTargets
	row := []observe.Region{{X: 0.1, Y: 0.4, Width: 0.3, Height: 0.05}}
	at := observe.PointerAt{X: 0.2, Y: 0.42}

	first := c.Press(1, at, "visual_1", observe.Proposed(row, 0, 0, 0))
	// The proposer found it slightly differently the second time, which is the ordinary
	// case and the thing mutual centre containment exists to absorb.
	moved := []observe.Region{{X: 0.11, Y: 0.405, Width: 0.28, Height: 0.052}}
	second := c.Press(40, at, "visual_1", observe.Proposed(moved, 0, 0, 0))
	if second.Target != first.Target {
		t.Errorf("the same control read slightly differently got %q then %q. A route "+
			"needs a repeated click to refer to one control",
			first.Target, second.Target)
	}
	// The same place on a DIFFERENT screen is a different control.
	other := c.Press(80, at, "visual_2", observe.Proposed(row, 0, 0, 0))
	if other.Target == first.Target {
		t.Errorf("the same place on another screen reused %q; the third row of one menu "+
			"and the third row of another are two controls", other.Target)
	}
	// And a genuinely different place on the same screen is a different control.
	far := c.Press(120, observe.PointerAt{X: 0.8, Y: 0.8}, "visual_1",
		observe.Proposed([]observe.Region{{X: 0.75, Y: 0.75, Width: 0.2, Height: 0.1}}, 0, 0, 0))
	if far.Target == first.Target {
		t.Errorf("a press at the other end of the screen reused %q", far.Target)
	}
	ts := c.Targets()
	if len(ts) != 3 {
		t.Fatalf("%d target identit(ies) minted over four presses at three controls: %+v",
			len(ts), ts)
	}
	for _, x := range ts {
		if x.ID == first.Target && x.Clicks != 2 {
			t.Errorf("%s was clicked %d time(s), want 2", x.ID, x.Clicks)
		}
		if len(x.Where) == 0 {
			t.Errorf("%s remembers no rectangle, so nothing could re-ground it", x.ID)
		}
	}
}

// A change is attributed to the MOST RECENT press that could have caused it.
//
// Most recent, because an older press cannot have caused what a newer one is sitting in front
// of. On the screen being left, because a press made somewhere else is about something else.
// Bounded, because a press four seconds earlier explains nothing.
//
// Deleting any of the three conditions must fail this.
func TestAChangeIsAttributedToTheMostRecentPressThatCouldHaveCausedIt(t *testing.T) {
	var c observe.ClickTargets
	a := []observe.Region{{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.1}}
	b := []observe.Region{{X: 0.6, Y: 0.6, Width: 0.2, Height: 0.1}}
	// An older press on the right screen, then a newer one, then a press on ANOTHER
	// screen that is newer than both. The order matters: if the newest press were the
	// qualifying one, the screen test would be doing nothing here.
	old := c.Press(2, observe.PointerAt{X: 0.15, Y: 0.15}, "visual_1", observe.Proposed(a, 0, 0, 0))
	recent := c.Press(8, observe.PointerAt{X: 0.65, Y: 0.65}, "visual_1", observe.Proposed(b, 0, 0, 0))
	c.Press(10, observe.PointerAt{X: 0.15, Y: 0.15}, "visual_9", observe.Proposed(a, 0, 0, 0))
	c.Arrived(12, observe.VisualTransition{From: "visual_1", To: "visual_2"})

	e := c.Evidence()
	if len(e.Attributed) != 1 {
		t.Fatalf("%d unit(s) attributed, want 1: %+v", len(e.Attributed), e.Attributed)
	}
	got := e.Attributed[0]
	if got.Target != recent.Target {
		t.Errorf("the change was attributed to %q, want the most recent press made ON "+
			"the screen being left, %q. A press made somewhere else is about "+
			"something else, however recent it is", got.Target, recent.Target)
	}
	if got.From != "visual_1" || got.To != "visual_2" {
		t.Errorf("the unit reads %s -> %s", got.From, got.To)
	}
	if len(got.Gaps) != 1 || got.Gaps[0] != 4 {
		t.Errorf("the gap is %v, want [4]. The window is generous on purpose, so a long "+
			"gap has to be visible rather than hidden inside it", got.Gaps)
	}
	// THE OTHERS ARE RESOLVED TOO. The older press on the right screen changed nothing;
	// the press on another screen changed nothing. Both are outcomes, not silences.
	if len(e.Inert) != 2 {
		t.Errorf("%d press(es) filed as having changed nothing, want 2: %+v",
			len(e.Inert), e.Inert)
	}
	for _, p := range e.Inert {
		if p.Target == recent.Target {
			t.Error("the attributed press was also filed as inert")
		}
	}
	_ = old
}

// A press too long before a change is not its cause.
func TestAPressTooLongBeforeAChangeIsNotItsCause(t *testing.T) {
	var c observe.ClickTargets
	c.Press(1, observe.PointerAt{X: 0.15, Y: 0.15}, "visual_1",
		observe.Proposed([]observe.Region{{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.1}}, 0, 0, 0))
	c.Arrived(2+observe.MaxPressToTransitionLooks,
		observe.VisualTransition{From: "visual_1", To: "visual_2"})

	e := c.Evidence()
	if len(e.Attributed) != 0 {
		t.Errorf("a press %d looks before a change was credited for it: %+v",
			observe.MaxPressToTransitionLooks+1, e.Attributed)
	}
	if len(e.Unattributed) != 1 {
		t.Errorf("%d change(s) recorded as pressed for by nobody, want 1", len(e.Unattributed))
	}
	if len(e.Inert) != 1 {
		t.Errorf("%d press(es) filed as having changed nothing, want 1", len(e.Inert))
	}
}

// A change nobody pressed for is recorded as itself.
//
// A timer, a load finishing, an animation settling. Reported beside the attributed ones for the
// reason ADR-158's own unattributed count is reported: a change that usually has no input
// before it is evidence AGAINST reading the correlated ones as causes.
//
// Deleting the unattributed branch must fail this.
func TestAChangeNobodyPressedForIsRecordedAsItself(t *testing.T) {
	var c observe.ClickTargets
	c.Arrived(10, observe.VisualTransition{From: "visual_1", To: "visual_6"})
	e := c.Evidence()
	if len(e.Unattributed) != 1 {
		t.Fatalf("%d unattributed change(s), want 1", len(e.Unattributed))
	}
	if e.Unattributed[0].From != "visual_1" || e.Unattributed[0].To != "visual_6" {
		t.Errorf("the unattributed change reads %s -> %s",
			e.Unattributed[0].From, e.Unattributed[0].To)
	}
	if len(e.Attributed) != 0 {
		t.Errorf("a change with no press before it was attributed to something: %+v",
			e.Attributed)
	}
}

// A repeated unit is counted twice, with when each occurrence happened and how long it took.
func TestARepeatedUnitIsCountedTwiceWithItsGaps(t *testing.T) {
	var c observe.ClickTargets
	row := []observe.Region{{X: 0.1, Y: 0.4, Width: 0.3, Height: 0.05}}
	at := observe.PointerAt{X: 0.2, Y: 0.42}
	c.Press(10, at, "visual_1", observe.Proposed(row, 0, 0, 0))
	c.Arrived(16, observe.VisualTransition{From: "visual_1", To: "visual_2"})
	c.Press(100, at, "visual_1", observe.Proposed(row, 0, 0, 0))
	c.Arrived(103, observe.VisualTransition{From: "visual_1", To: "visual_2"})

	e := c.Evidence()
	if len(e.Attributed) != 1 {
		t.Fatalf("%d distinct unit(s) over one navigation demonstrated twice: %+v",
			len(e.Attributed), e.Attributed)
	}
	got := e.Attributed[0]
	if got.Count != 2 {
		t.Errorf("the unit was observed %d time(s), want 2", got.Count)
	}
	if len(got.Looks) != 2 || got.Looks[0] != 10 || got.Looks[1] != 100 {
		t.Errorf("the presses are recorded at %v, want [10 100]: two demonstrations are "+
			"two moments, and a count alone cannot say whether it was twice or one "+
			"flap counted twice", got.Looks)
	}
	if len(got.Gaps) != 2 || got.Gaps[0] != 6 || got.Gaps[1] != 3 {
		t.Errorf("the gaps are %v, want [6 3]", got.Gaps)
	}
}

// Two identities on one screen that reached the same place are REPORTED, not merged.
//
// # Why this is a report
//
// Four identity rules were measured on the real recording and they trade a split against a
// merge: the smallest candidate alone splits one control across two demonstrations, and the
// whole candidate set merges three visibly different menu entries into one. Geometry does not
// settle it.
//
// The owner asked for exactly this:
//
//	For the five ambiguous clicks, I'd preserve multiple possible region matches until visual
//	or transition evidence can disambiguate them.
//
// So the pair is put in front of somebody with the evidence attached. It is also the shape of
// two genuinely different entries that happen to open the same thing, which is why merging it
// here would be a guess wearing a conclusion's clothes.
//
// Deleting the derivation must fail this.
func TestTwoIdentitiesThatReachedTheSamePlaceAreReportedNotMerged(t *testing.T) {
	var c observe.ClickTargets
	// Two different rows of one menu, each clicked once, each followed by the same screen.
	one := []observe.Region{{X: 0.1, Y: 0.4, Width: 0.3, Height: 0.05}}
	two := []observe.Region{{X: 0.1, Y: 0.6, Width: 0.3, Height: 0.05}}
	c.Press(10, observe.PointerAt{X: 0.2, Y: 0.42}, "visual_2", observe.Proposed(one, 0, 0, 0))
	c.Arrived(15, observe.VisualTransition{From: "visual_2", To: "visual_3"})
	c.Press(100, observe.PointerAt{X: 0.2, Y: 0.62}, "visual_2", observe.Proposed(two, 0, 0, 0))
	c.Arrived(105, observe.VisualTransition{From: "visual_2", To: "visual_3"})

	e := c.Evidence()
	if len(e.Attributed) != 2 {
		t.Fatalf("%d unit(s), want 2: nothing may merge them", len(e.Attributed))
	}
	if len(e.MaybeOneControl) != 1 {
		t.Fatalf("%d pair(s) reported as maybe one control, want 1: %+v",
			len(e.MaybeOneControl), e.MaybeOneControl)
	}
	p := e.MaybeOneControl[0]
	if p.Screen != "visual_2" || p.To != "visual_3" || p.A == p.B {
		t.Errorf("the pair reads %+v", p)
	}
	// AND A SINGLE UNIT IS NOT A PAIR WITH ITSELF.
	var solo observe.ClickTargets
	solo.Press(10, observe.PointerAt{X: 0.2, Y: 0.42}, "visual_2", observe.Proposed(one, 0, 0, 0))
	solo.Arrived(15, observe.VisualTransition{From: "visual_2", To: "visual_3"})
	if n := len(solo.Evidence().MaybeOneControl); n != 0 {
		t.Errorf("%d pair(s) from one unit", n)
	}
}

// The last press of a demonstration is not lost with the session.
//
// It is the one that opened the thing being demonstrated. A session that ended before the screen
// settled would otherwise drop it entirely, and a press that is neither attributed nor inert is
// a press the report cannot explain.
//
// Deleting Close's resolve must fail this.
func TestTheLastPressOfADemonstrationIsNotLost(t *testing.T) {
	var c observe.ClickTargets
	c.Press(900, observe.PointerAt{X: 0.2, Y: 0.42}, "visual_1",
		observe.Proposed([]observe.Region{{X: 0.1, Y: 0.4, Width: 0.3, Height: 0.05}}, 0, 0, 0))
	if n := len(c.Evidence().Inert); n != 0 {
		t.Fatalf("a press was filed before the session closed (%d)", n)
	}
	c.Close()
	if n := len(c.Evidence().Inert); n != 1 {
		t.Errorf("%d press(es) filed after close, want 1. A press that is neither "+
			"attributed nor inert is one the report cannot explain", n)
	}
}
