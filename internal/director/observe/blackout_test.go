package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A BLACKOUT IS NOT A DESTINATION.
//
// Experiment-024, during the phase where the player was told to hold still and did: the screen
// did not change for fifty seconds, the detector reported 29 → 0 → 13 → 13 → 18 → 0 → 0 → 1 →
// 1 → 1 → 0 admitted regions, and the gate CONFIRMED a transition into `state_unknown`.
//
// `state_unknown` is the id of an inference nobody could place. It produces no hypothesis, can
// never be a durable subject, and nothing downstream can do anything with it. A two-look
// detector blackout became a semantic change into a state that does not exist.
//
// The fix is not a longer window. Two unreadable looks IS a long time at ~2s a look, and a
// window long enough to outlast a blackout would be too long to see a real change. What was
// wrong is the shape of the answer: not "you went nowhere", but "I no longer know where you
// are".

// richScreen is a menu of n rows at a given height, so fragments of it have room to differ.
func darkenableScreen(n int, y0 float64) []observe.ShadowRegion {
	var out []observe.ShadowRegion
	for i := range n {
		out = append(out, observe.ShadowRegion{
			Role: "button",
			Region: observe.Region{
				X: 0.4, Y: y0 + float64(i)*0.042, Width: 0.17, Height: 0.036,
			},
		})
	}
	return out
}

// settleOn drives k looks at one composition, returning the next sequence number.
func settleOn(g *observe.ScreenSegmenter, from int, c []observe.ShadowRegion, k int) int {
	n := from
	for range k {
		g.Observe(n, c, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
		n++
	}
	return n
}

// dark drives k looks that cannot be placed at all.
//
// Each one is a DIFFERENT fragment of the settled screen, and that is load-bearing. A
// composition is identified by its role counts, so repeating one fragment twice would promote
// it into a screen of its own — which is exactly what `button=1` did on the live run, and is
// the reason a blackout has to be built out of readings that never recur.
func dark(g *observe.ScreenSegmenter, from int, a []observe.ShadowRegion, k int) int {
	n := from
	for i := range k {
		g.Observe(n, a[:i+1], nil, observe.SemanticEvidence{}, observe.SelectionAt{})
		n++
	}
	return n
}

// Deleting the ScreenStateUnknown guard in propose must fail this.
func TestABlackoutIsNeverAConfirmedTransition(t *testing.T) {
	a := darkenableScreen(8, 0.10)
	var g observe.ScreenSegmenter

	// Settled on A, then two looks nobody could place, then A again. At ~2s a look that
	// is four seconds of not being able to read the screen - the exact length that used to
	// become a confirmed transition into a state that does not exist.
	n := settleOn(&g, 1, a, 3)
	n = dark(&g, n, a, 2)
	settleOn(&g, n, a, 3)

	for _, c := range g.Crossings() {
		if c.From == observe.ScreenStateUnknown || c.To == observe.ScreenStateUnknown {
			t.Errorf("a blackout became a crossing: %+v\nstate_unknown is not a screen "+
				"and must never be an endpoint of one", g.Crossings())
		}
	}
	for _, tr := range g.Transitions() {
		if tr.From == observe.ScreenStateUnknown || tr.To == observe.ScreenStateUnknown {
			t.Errorf("a blackout became a transition: %+v", tr)
		}
	}
	if g.Flicker.Expired != 1 {
		t.Errorf("expired = %d, want 1: a two-look blackout has to be reported as the "+
			"session giving up on knowing where it is.\n%+v", g.Flicker.Expired, g.Flicker)
	}
	if g.Flicker.LongestBlackout != 2 {
		t.Errorf("longest blackout = %d, want 2; expiry alone cannot say whether the "+
			"screen was unreadable for four seconds or four minutes",
			g.Flicker.LongestBlackout)
	}
}

// And coming back is not a change either.
//
// The belief expired, so there is nothing to have changed FROM. Recording A → B would assert an
// adjacency across an interval nobody observed — which is the line the crossing bridge's own
// bound was already drawn at, for the same reason.
func TestComingBackFromABlackoutIsNotAChange(t *testing.T) {
	a, b := darkenableScreen(8, 0.10), darkenableScreen(5, 0.60)
	var g observe.ScreenSegmenter

	// Settled on A, a blackout, then settled on B. Marco could not see for two looks;
	// whether B followed A, or B followed C followed A, is not something it can say.
	n := settleOn(&g, 1, a, 3)
	n = dark(&g, n, a, 2)
	settleOn(&g, n, b, 3)

	if k := len(g.Crossings()); k != 0 {
		t.Errorf("%d crossing(s) across a two-look blackout: %+v\nA recorded adjacency "+
			"across an unobserved interval is an invented one", k, g.Crossings())
	}
	if k := len(g.Transitions()); k != 0 {
		t.Errorf("%d transition(s) across a two-look blackout: %+v", k, g.Transitions())
	}
}

// The short gap still works exactly as it did, which is the other half of the claim.
//
// A gate that expired on every gap would pass the two tests above and would have thrown away
// what the confirmation gate built: a change seen THROUGH a dropped frame is still the one
// change it was, recorded directly, with the press that caused it.
func TestAShortGapStillRecordsTheChangeItSaw(t *testing.T) {
	a, b := darkenableScreen(8, 0.10), darkenableScreen(5, 0.60)
	var g observe.ScreenSegmenter

	press := []observe.InputEvent{{Intent: observe.NavDown, AtMS: 10,
		Device: observe.DeviceController}}

	g.Observe(1, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(2, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	// The press, then the half-rendered frame it produced, then the new screen.
	g.Observe(3, a[:1], press, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(4, b, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(5, b, nil, observe.SemanticEvidence{}, observe.SelectionAt{})

	trs := g.Transitions()
	if len(trs) != 1 {
		t.Fatalf("%d transition(s) through a one-frame gap, want 1: %+v", len(trs), trs)
	}
	if trs[0].From == observe.ScreenStateUnknown || trs[0].To == observe.ScreenStateUnknown {
		t.Errorf("the change was recorded through the unplaceable state: %+v", trs[0])
	}
	if len(trs[0].Preceded) == 0 {
		t.Errorf("the press that rode on the unreadable frame was lost: %+v\nThe frame "+
			"could not be read; what the player did during it was observed by a "+
			"different producer and is not in doubt", trs[0])
	}
	if g.Flicker.Expired != 0 {
		t.Errorf("expired = %d from a one-frame gap; one dropped frame is uncertainty, "+
			"not a reason to stop knowing where the player is", g.Flicker.Expired)
	}
}

// A candidate lost to a blackout is the DETECTOR's error, not the gate's.
//
// Held apart because RejectedWithInput is printed as "the gate's own error rate", and it decides
// whether the confirmation window is right. Folding in changes nobody could judge because the
// screen went dark would make the window look wrong for a reason that has nothing to do with it.
func TestAChangeLostToABlackoutIsNotTheGatesError(t *testing.T) {
	a, b := darkenableScreen(8, 0.10), darkenableScreen(5, 0.60)
	var g observe.ScreenSegmenter

	press := []observe.InputEvent{{Intent: observe.NavDown, AtMS: 10,
		Device: observe.DeviceController}}

	g.Observe(1, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(2, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	// One look of B — a candidate, not yet confirmed — and then the screen goes dark for
	// long enough that nobody can ever say whether it stayed.
	g.Observe(3, b, press, observe.SemanticEvidence{}, observe.SelectionAt{})
	dark(&g, 4, a, 2)

	if g.Flicker.ChangesLostToBlackout != 1 {
		t.Errorf("changes lost to blackout = %d, want 1: %+v",
			g.Flicker.ChangesLostToBlackout, g.Flicker)
	}
	if g.Flicker.RejectedWithInput != 0 {
		t.Errorf("a change nobody could judge was charged to the gate's error rate: %+v\n"+
			"That number decides whether the confirmation window is right, and this "+
			"has nothing to do with the window", g.Flicker)
	}
}

// ── the interface belief ──────────────────────────────────────────────────────

// bareLook is a look that SAW the screen and found nothing shaded on it.
func bareLook(seq int) observe.Sample {
	s := menuSample(seq, 6, 0)
	bare := *s.Shadow
	bare.Regions = nil
	s.Shadow = &bare
	return s
}

// watchedMenu is a session that identified a list by watching a mark move down it.
func watchedMenu(t *testing.T) (observe.ShadowTotals, int) {
	t.Helper()
	var totals observe.ShadowTotals
	seq := 0
	for _, m := range []int{-1, -1, -1, -1, -1, -1, 0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3} {
		seq++
		totals.Observe(menuSample(seq, 6, m))
	}
	if totals.Interface.Status != observe.InterfaceProvisional {
		t.Fatalf("precondition: no list identified (%s)", totals.Interface.Because)
	}
	return totals, seq
}

// One bare look does not end a belief built from eighteen.
//
// The measured shape: on an unchanging menu the detector reported zero admitted regions on four
// separate looks out of eleven. Under the old rule each one replaced a provisional belief with
// `unknown` on the spot — four semantic changes to unknown on a screen that did not change.
func TestOneBareLookDoesNotEndTheBelief(t *testing.T) {
	totals, seq := watchedMenu(t)
	settled := totals.Interface

	totals.Observe(bareLook(seq + 1))

	b := totals.Interface
	if b.Status != observe.InterfaceProvisional {
		t.Errorf("one look that found nothing on the screen ended a belief built from "+
			"%d: %s → %s\nbecause: %s", settled.Observations, settled.Status, b.Status,
			b.Because)
	}
	if !b.Stale {
		t.Error("the belief survived a bare look and does not say it is unconfirmed; a " +
			"belief that is merely unconfirmed must not read like one that was just seen")
	}
	if b.Empty != 1 {
		t.Errorf("looks_with_nothing_on_screen = %d, want 1", b.Empty)
	}
	if !strings.Contains(b.Because, "found nothing shaded") {
		t.Errorf("the belief does not say what happened to it:\n  %s", b.Because)
	}
	if totals.Changed.InterfaceChanged {
		t.Error("one bare look was reported as the interface changing")
	}
}

// And a menu that really closed is really given up. The pair is the claim: a rule that held a
// belief forever would pass the test above and would report a list that is no longer there.
func TestASecondBareLookEndsIt(t *testing.T) {
	totals, seq := watchedMenu(t)
	for i := range 2 {
		totals.Observe(bareLook(seq + 1 + i))
	}
	b := totals.Interface
	if b.Status != observe.InterfaceUnknown {
		t.Errorf("two consecutive looks saw the screen and found nothing on it, and the "+
			"belief is still %s: %s", b.Status, b.Because)
	}
	if b.Selected >= 0 {
		t.Errorf("entry %d is still reported as selected on a screen with nothing on it",
			b.Selected)
	}
	if !strings.Contains(b.Because, "consecutive") {
		t.Errorf("the reason does not say how much looking it took to give up:\n  %s",
			b.Because)
	}
}

// A bare look that is NOT consecutive does not accumulate. Otherwise a detector that blinks
// once a minute would eventually give up on a menu nobody touched.
func TestBareLooksMustBeConsecutiveToEndABelief(t *testing.T) {
	totals, seq := watchedMenu(t)
	for range 4 {
		seq++
		totals.Observe(bareLook(seq))
		seq++
		totals.Observe(menuSample(seq, 6, 3))
	}
	if b := totals.Interface; b.Status != observe.InterfaceProvisional {
		t.Errorf("a detector that blinked every other look gave up a belief about a menu "+
			"nobody touched: %s — %s", b.Status, b.Because)
	}
}
