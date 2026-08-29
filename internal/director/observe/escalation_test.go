package observe_test

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// WHAT THE BUDGET WILL AND WILL NOT BUY.
//
// The policy is small on purpose. Its whole job is to keep three measured facts from being
// re-derived by every caller that holds a sensor:
//
//	a sufficient reading buys nothing        37C: no actionable item added, 645–1379ms to say so
//	a fresh incomplete reading waits         a settle costs nothing, an inference costs a second
//	nobody waiting is not worth spending on  a game surface is a standing condition, not an event

func TestASufficientReadingBuysNothing(t *testing.T) {
	for _, need := range []observe.Need{observe.NeedWatching, observe.NeedAnswer, observe.NeedToAct} {
		for _, reason := range []observe.SufficiencyReason{
			observe.ReasonContentReached,
			observe.ReasonPopulatedPanel,
			observe.ReasonTooLittleToJudge,
		} {
			s := observe.Sufficiency{State: observe.Sufficient, Reason: reason}
			e := observe.EscalationOf(need, s, time.Hour)
			if e.Spend != observe.SpendNothing {
				t.Errorf("need %q, %q: spend = %q, want %q\n"+
					"37C measured a detector adding no actionable semantic item to a "+
					"reading that already sufficed, at 645–1379ms a frame.",
					need, reason, e.Spend, observe.SpendNothing)
			}
			if e.Worth() {
				t.Errorf("need %q, %q: an expensive sensor was told to run", need, reason)
			}
		}
	}
}

// A reading that has only just come back incomplete waits before it spends.
func TestAFreshIncompleteReadingSettlesFirst(t *testing.T) {
	s := observe.Sufficiency{State: observe.Incomplete,
		Reason: observe.ReasonClientAreaUnpopulated}
	e := observe.EscalationOf(observe.NeedToAct, s, 0)
	if e.Spend != observe.SpendSettle {
		t.Errorf("spend = %q, want %q\n"+
			"A page mid-navigation is briefly indistinguishable from one that failed to "+
			"arrive. Waiting costs nothing and is usually right; an inference costs most "+
			"of a second and is usually wasted.", e.Spend, observe.SpendSettle)
	}
	if e.Worth() {
		t.Error("a still-arriving page bought an inference")
	}
}

// Once waiting has had its chance, a caller that needs the evidence may spend.
func TestAPersistentlyIncompleteReadingMaySpend(t *testing.T) {
	s := observe.Sufficiency{State: observe.Incomplete,
		Reason: observe.ReasonClientAreaUnpopulated}
	e := observe.EscalationOf(observe.NeedToAct, s, 10*time.Second)
	if e.Spend != observe.SpendMore {
		t.Fatalf("spend = %q, want %q", e.Spend, observe.SpendMore)
	}
	if !e.Worth() {
		t.Error("a caller that has waited and still cannot act was told not to spend")
	}
	if e.Because != observe.ReasonClientAreaUnpopulated {
		t.Errorf("because = %q; the reason is carried so a caller can say what it is "+
			"waiting for rather than only that it is waiting", e.Because)
	}
}

// NOBODY WAITING IS NOT WORTH SPENDING ON.
//
// The case this phase exists for. 37D classifies a game viewport as incomplete and is right to
// — accessibility genuinely cannot represent that interface. But it is a STANDING condition,
// not an event: it will be incomplete for as long as the game is in front. Buying a second of
// inference every cadence to keep confirming it is exactly the expense 37E was asked to refuse.
func TestBackgroundWatchingDoesNotBuyInferenceForAStandingCondition(t *testing.T) {
	s := observe.Sufficiency{State: observe.Incomplete,
		Reason: observe.ReasonClientAreaUnpopulated}
	for _, waited := range []time.Duration{0, time.Second, time.Minute, time.Hour} {
		e := observe.EscalationOf(observe.NeedWatching, s, waited)
		if e.Worth() {
			t.Errorf("after %v of a window accessibility cannot represent, background "+
				"watching bought an inference.\nA game in front is not an event to "+
				"react to; it is a fact that stays true.", waited)
		}
	}
	// And somebody actually asking still gets it.
	if !observe.EscalationOf(observe.NeedAnswer, s, time.Minute).Worth() {
		t.Error("a person asking about the same window was refused; the distinction " +
			"between watching and being asked is the whole of the rule")
	}
}

// Nothing observed buys nothing, and says so as a refusal.
func TestAnUnobservableReadingIsRefusedRatherThanRepaired(t *testing.T) {
	s := observe.Sufficiency{State: observe.Unobservable,
		Reason: observe.ReasonNothingObserved}
	e := observe.EscalationOf(observe.NeedToAct, s, time.Hour)
	if e.Spend != observe.SpendNothingAndRefuse {
		t.Errorf("spend = %q, want %q\n"+
			"A detector pointed at a window the sensors never reached produces pixels "+
			"belonging to nothing.", e.Spend, observe.SpendNothingAndRefuse)
	}
	if e.Worth() {
		t.Error("an unreachable window bought an inference")
	}
}

// THE POLICY NAMES NO SENSOR.
//
// 37D's classifier is sensor-neutral and this is the layer that could quietly undo that. If it
// ever answers "run ScreenParser" rather than "more evidence is worth buying", then the choice
// of every future sensor has been made here by accident — and the caller holding OCR, or a
// region capture, or something nobody has written yet, has nowhere to stand.
func TestTheBudgetNamesNoSensor(t *testing.T) {
	all := []observe.Spend{
		observe.SpendNothing, observe.SpendSettle,
		observe.SpendMore, observe.SpendNothingAndRefuse,
	}
	for _, sp := range all {
		low := strings.ToLower(string(sp))
		for _, sensor := range []string{"screenparser", "ocr", "vision", "detector",
			"model", "capture", "pixel"} {
			if strings.Contains(low, sensor) {
				t.Errorf("the decision %q names %q.\n"+
					"This layer answers whether more evidence is worth buying. Which "+
					"evidence belongs to whoever holds the sensor and knows its cost.",
					sp, sensor)
			}
		}
	}
}
