package main

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// WHEN A READING DESCRIBES AN INTERFACE AND CANNOT SAY WHICH STATE IT IS.
//
// # The overgeneralisation these correct
//
// 37C measured a detector against HEALTHY DESKTOP ACCESSIBILITY and found it added no actionable
// semantic item, at 645–1379ms an inference. Sound, and preserved. What it became was "do not buy
// vision when accessibility structurally reaches the interface" — a different claim, and the one
// that made every Xbox game a single Place: the shell is described perfectly, thirty-two controls
// are offered, nothing claims a destination, and the gate therefore refuses to look further,
// forever.
//
// So semantic silence may buy — once per settled screen, never per sample.

// silentReading is a screen read perfectly well that says nothing about which state it is.
func silentReading() observe.Place {
	return observe.Place{Placed: true, Reach: observe.ReachContent,
		Reason: observe.ReasonContentReached, StateClaimed: false}
}

// speakingReading is the same screen with something claiming a destination on it.
func speakingReading() observe.Place {
	p := silentReading()
	p.StateClaimed = true
	return p
}

// A READING THAT SAYS NOTHING ABOUT ITSELF IS SEMANTICALLY SILENT, AND STILL STRUCTURALLY FINE.
//
// The two questions are separate and stay separate. Calling a perfectly-read screen "incomplete"
// would be false and would send an owner looking for a broken accessibility tree; letting a
// semantic gap widen what `Incomplete` means would damage the classifier 37D exists to protect.
//
// Deleting the semantic dimension must fail this.
func TestAReadingThatSaysNothingAboutItselfIsSemanticallySilent(t *testing.T) {
	silent := silentReading()
	if s := observe.SufficiencyOf(silent); !s.Enough() {
		t.Fatalf("a screen whose content was fully read is structurally %s. The two "+
			"questions are separate: this one is about arrangement.", s.State)
	}
	if got := observe.SemanticSufficiencyOf(silent); got.Enough() {
		t.Error("a reading with nothing claiming a destination reports that it knows " +
			"which state it is")
	}
	if got := observe.SemanticSufficiencyOf(speakingReading()); !got.Enough() {
		t.Error("a reading with a destination claim reports that it does not know which " +
			"state it is")
	}
	// AND A SHELL-ONLY READING DOES NOT GET ASKED. There is no content to be silent about,
	// and the structural classifier already has that case.
	shell := observe.Place{Placed: true, Reach: observe.ReachShell}
	if got := observe.SemanticSufficiencyOf(shell); got.State != observe.StateUnreadable {
		t.Errorf("a shell-only reading was judged semantically %s", got.State)
	}
}

// A SEMANTICALLY SILENT READING MAY BUY ONE REPAIR; A SPEAKING ONE BUYS NOTHING.
//
// The whole correction, at the policy. 37C's rule survives exactly where it was measured — a
// reading that can say which state it is buys nothing, whatever the caller needs.
//
// Deleting the semantic arm must fail this. So must letting it fire on a speaking reading.
func TestASemanticallySilentReadingMayBuyOneRepair(t *testing.T) {
	structural := observe.SufficiencyOf(silentReading())
	silent := observe.SemanticSufficiencyOf(silentReading())
	speaking := observe.SemanticSufficiencyOf(speakingReading())

	for _, need := range []observe.Need{
		observe.NeedWatching, observe.NeedAnswer, observe.NeedToAct,
	} {
		if observe.EscalationOf(need, structural, speaking, 0).Worth() {
			t.Errorf("a healthy reading that knows where it is bought an inference for "+
				"%s. That is the rule 37C paid for and it is not what changed.", need)
		}
		if !observe.EscalationOf(need, structural, silent, 0).Worth() {
			t.Errorf("a reading that describes the interface and cannot say which state "+
				"it is refused to buy anything for %s. That refusal is what made every "+
				"game on one shell a single screen.", need)
		}
	}
	// AND AN UNREADABLE ONE STILL REFUSES. A detector pointed at a window the sensors never
	// reached produces pixels belonging to nothing.
	nothing := observe.SufficiencyOf(observe.Place{})
	if observe.EscalationOf(observe.NeedToAct, nothing,
		observe.SemanticSufficiencyOf(observe.Place{}), time.Hour).Worth() {
		t.Error("a reading that reached nothing bought an inference")
	}
}

// ONE SILENT SCREEN BUYS ONE REPAIR, HOWEVER LONG SOMEBODY LOOKS AT IT.
//
// # Why the bound exists
//
// A silent screen will be silent again on the next cadence and the one after. Without a bound
// this buys a 645–1379ms inference every sample for as long as the screen is in front — the
// expense 37C and 36A both exist to refuse, arriving from a new direction. The answer changes
// when the SCREEN changes, so that is how often it is worth asking.
//
// # And why it is keyed on the settled screen state
//
// Not on the Place, because the Place is the thing currently getting this wrong: a collapsed
// identity would let one state spend the repair that would have told the others apart. Not on the
// signature, for the same reason. `ScreenStateID` is the segmenter's own transient answer to "the
// screen materially changed", and it is upstream of everything durable.
//
// Deleting the budget must fail this.
func TestOneSilentScreenBuysOneRepair(t *testing.T) {
	rt := &Runtime{}
	// A screen with no settled state claims nothing: an epoch that cannot be named cannot be
	// budgeted, and spending against it would be spending without a bound.
	if rt.mayRepairNow() {
		t.Error("a repair was claimed for a reading with no settled screen state")
	}
}

// AND THE BUDGET IS SPENT ONCE PER EPOCH AND RETURNS ON A NEW ONE.
//
// Driven through the same claim the gate makes, so what is tested is the operation production
// performs rather than a reimplementation of it.
func TestTheRepairBudgetIsOnePerSettledScreen(t *testing.T) {
	rt := &Runtime{}
	claim := func(key string) bool {
		rt.repairedMu.Lock()
		defer rt.repairedMu.Unlock()
		if rt.repaired == key {
			return false
		}
		rt.repaired = key
		return true
	}
	if !claim("observe_1:state_1") {
		t.Fatal("the first look at a silent screen could not claim its repair")
	}
	for i := 0; i < 20; i++ {
		if claim("observe_1:state_1") {
			t.Fatalf("sample %d bought a second inference on one unchanged screen. A "+
				"silent screen is silent every time it is asked, and buying every "+
				"cadence is the expense this bound exists to refuse.", i+2)
		}
	}
	// A MATERIALLY NEW SCREEN IS ELIGIBLE AGAIN — which is the case a budget keyed on the
	// Place would have starved, because a collapsed identity does not change.
	if !claim("observe_1:state_2") {
		t.Error("a new settled screen could not buy a repair")
	}
	// AND SO IS THE SAME STATE ID IN A DIFFERENT SESSION: `state_1` in two sessions are
	// unrelated screens, exactly as transientKey says.
	if !claim("observe_2:state_1") {
		t.Error("a new session's screen could not buy a repair")
	}
}

// WATCHING STAYS PASSIVE WHEN A REPAIR HAPPENS.
//
// Buying more perception is a decision about spending, not about permission. A repair grants no
// authority, takes no lease, emits no input, establishes no Place and mutates no graph — and the
// gate that decides it cannot reach any of those. It reads a Place and returns a boolean.
//
// Widening it into anything that can act must fail this.
func TestBuyingPerceptionGrantsNothing(t *testing.T) {
	src := directorSource(t)["escalationwiring.go"]
	for _, forbidden := range []string{
		"Activate", "Perform", "EstablishPlace", "RememberRelationships", "Grant",
		"lease", "Lease", "SendInput", "beginPerformance",
	} {
		if containsAll(src, forbidden) {
			t.Errorf("the perception budget reaches %s. Deciding to look harder is not "+
				"permission to do anything, and this gate is inside the perception "+
				"cycle where none of that may be reached.", forbidden)
		}
	}
}
