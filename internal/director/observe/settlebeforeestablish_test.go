package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Marco may SEE a place before it knows enough to remember one.
//
// # The measured defect
//
// Windows Settings renders in stages. The START of a teach was fingerprinted after a six-second
// dwell — while the page was still arriving — and the DESTINATION at the end of a forty-five-second
// pass, from a screen that had long since finished. Same code path, same producer, different
// maturity of evidence:
//
//	START grounding        state=state_2  regions=7   members=10
//	DESTINATION grounding  state=state_2  regions=15  members=22
//
// The same state, read twice. Across four independent cold stores the destination reproduced
// `subj_892a4cc30f41` every time and the start minted a new subject nearly every time. That
// asymmetry is the whole diagnosis: identity was resting on evidence that had not finished.
//
// # The rule, and why it is not a longer dwell
//
// Segmentation already says "one sighting is a transition frame; a second is a screen". The same
// sentence answers this one level down, about composition: a shape seen once is a frame the screen
// passed through, a shape seen twice is what the screen is made of. A screen that was ALREADY
// stable therefore settles on its second observation and waits for no clock.

// arriving is a screen rendering in stages, then holding.
func arriving(stages []int, settled int, hold int) [][]observe.ShadowRegion {
	var frames [][]observe.ShadowRegion
	for _, n := range stages {
		frames = append(frames, panel(n))
	}
	full := panel(settled)
	for range hold {
		frames = append(frames, full)
	}
	return frames
}

// currentSettled reports whether the state the session ended on is identity-bearing.
func currentSettled(t *testing.T, frames [][]observe.ShadowRegion) (bool, int) {
	t.Helper()
	k := segmentOver(frames)
	states := k.States()
	if len(states) == 0 {
		t.Fatal("the session produced no screen state")
	}
	return states[0].Settled, states[0].Roles["button"]
}

// THE regression. A screen still arriving is not yet a place.
func TestAScreenStillArrivingIsNotIdentityBearing(t *testing.T) {
	// Every stage seen exactly once — the shape of a page rendering in.
	settled, _ := currentSettled(t, arriving([]int{4, 9, 14, 18}, 21, 0))
	if settled {
		t.Error("a screen whose composition changed on every single observation was called " +
			"settled.\nFingerprinting it stores a screen that never existed for more than half " +
			"a second, and the next visit will not recognise it.")
	}
}

// The same screen, once it has stopped changing, IS.
func TestTheSameScreenSettlesOnceItHoldsStill(t *testing.T) {
	settled, buttons := currentSettled(t, arriving([]int{4, 9, 14, 18}, 21, 6))
	if !settled {
		t.Fatal("a screen that rendered in and then held still for six observations never " +
			"became identity-bearing; nothing would ever be remembered")
	}
	if buttons != 21 {
		t.Errorf("the settled composition reports %d button(s), want the finished screen's 21",
			buttons)
	}
}

// A screen that was ALREADY stable settles at once. This is evidence, not a duration.
//
// The control against the fix quietly becoming a minimum dwell: two observations of an unchanging
// screen are enough, and a test that passed only because time went by would fail here.
func TestAnAlreadyStableScreenSettlesImmediately(t *testing.T) {
	settled, _ := currentSettled(t, arriving(nil, 21, observe.StatePromotionCount))
	if !settled {
		t.Errorf("a screen that was never anything but itself needed more than %d "+
			"observation(s) to be worth remembering; that is a dwell, not evidence",
			observe.StatePromotionCount)
	}
}

// Harmless churn does not hold a place unsettled forever.
//
// A caret, a clock, a changing label. None of them alter the ROLE composition, which is what
// identity is made of — so a screen with something moving on it still settles. Without this the
// gate would be a liveness bug: applications that never stop twitching could never be learned.
func TestHarmlessChurnDoesNotPreventSettling(t *testing.T) {
	// The same composition every frame; only the geometry of one member drifts, the way a
	// caret or an animating element does.
	var frames [][]observe.ShadowRegion
	for i := range 8 {
		f := panel(21)
		f[0].Region.X += float64(i) * 0.001
		frames = append(frames, f)
	}
	if settled, _ := currentSettled(t, frames); !settled {
		t.Error("a screen with one thing moving on it never settled.\nRole composition is what " +
			"identity is made of; requiring stillness would make any application with a clock " +
			"on it permanently unlearnable.")
	}
}

// Even a genuinely oscillating role settles, on whichever reading recurs.
//
// The liveness case that a naive "nothing changed since last frame" rule would fail forever.
func TestAnOscillatingRoleStillSettles(t *testing.T) {
	var frames [][]observe.ShadowRegion
	for i := range 10 {
		frames = append(frames, panel(20+i%2)) // 20, 21, 20, 21 …
	}
	settled, buttons := currentSettled(t, frames)
	if !settled {
		t.Error("a role flipping between two counts never settled; both readings recurred, so " +
			"both are what the screen is made of and the mode decides")
	}
	if buttons != 20 && buttons != 21 {
		t.Errorf("the settled composition is %d, which is neither reading", buttons)
	}
}

// Leaving mid-render persists nothing, because there is nothing settled to persist.
func TestLeavingWhileTheScreenIsStillArrivingEstablishesNothing(t *testing.T) {
	tot := totalsOver(arriving([]int{4, 9, 14, 18}, 21, 0))
	_, refusal := observe.PlaceToEstablish(tot, "testgame", alwaysNew{},
		observe.DefaultHypothesisThresholds())
	if refusal != observe.PlaceNotSettled {
		t.Errorf("refusal = %q, want %q; a partial screen must not become a durable place",
			refusal, observe.PlaceNotSettled)
	}
}

// And once it settles, the same path establishes it.
func TestOnceSettledTheSamePathEstablishes(t *testing.T) {
	tot := totalsOver(arriving([]int{4, 9, 14, 18}, 21, 8))
	_, refusal := observe.PlaceToEstablish(tot, "testgame", alwaysNew{},
		observe.DefaultHypothesisThresholds())

	// The GATE opened. This fixture is refused further down for carrying no read terms and no
	// envelope — a pre-existing rule about what could ever be recognised again, and not what
	// this test is about. Asserting the whole chain here would make it fail for the wrong
	// reason the day somebody changed the discriminator.
	if refusal == observe.PlaceNotSettled {
		t.Fatal("a screen that rendered in and then held still for eight observations is " +
			"still refused as unsettled; nothing would ever become durable")
	}
	// And what it WOULD be stored under is the finished composition, not a stage of it.
	var settledButtons int
	for _, st := range tot.States {
		if st.ID == tot.CurrentState {
			settledButtons = st.Roles["button"]
		}
	}
	if settledButtons != 21 {
		t.Errorf("the current state's composition is %d button(s), want the settled 21",
			settledButtons)
	}
}

// alwaysNew recognises nothing, so establishment is never refused as already-known.
type alwaysNew struct{ observe.Memory }

func (alwaysNew) Recall(string, observe.StructureSignature) observe.Recollection {
	return observe.Recollection{Verdict: observe.MatchDifferent}
}

// totalsOver drives the session the way the RUNNER does, so CurrentState is the tracker.s own
// conclusion rather than something a test picked.
func totalsOver(frames [][]observe.ShadowRegion) observe.ShadowTotals {
	var tot observe.ShadowTotals
	for _, regions := range frames {
		tot.Observe(observe.Sample{
			Shadow: &observe.ShadowSample{Ran: true, TargetProven: true, Regions: regions},
			Structure: observe.StructuralView{
				Source: observe.StructureFused, Regions: regions,
			},
		})
	}
	return tot
}
