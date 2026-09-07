package main

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
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

// THE REPAIR MAY READ WHAT IT WAS BOUGHT TO READ.
//
// # The defect
//
// A semantically silent screen earns one bounded pass per settled screen state. Whether that
// pass could READ was decided somewhere else entirely — `req.ReadLabels`, the session's
// LabelEvery cadence, which knows nothing about repairs and is not asking a semantic question.
// So the one expensive token bought BECAUSE structured perception could not say which state
// this is had roughly a 1-in-LabelEvery chance of performing the reading that is the only
// thing that could say. The rest of the time it was spent blind.
//
// Measured on Epic Games Launcher, whose accessibility tree exposes ONE element: fourteen
// vision passes produced 31 structural controls every time, byte-identical, and four passes
// carrying the reading budget named eight of them — "Store", "Library", "Unreal Engine",
// "Buy Now" — identically each time. Neither sensor was unstable and neither was waiting for
// the other: `labelsOver` reads inside an accepted control's own box on the SAME captured
// image. The naming path was already same-pass. The repair simply could not use it.
//
// # Why this test enters through the sampler
//
// Because the bug lived in the seam. Both halves were correct alone: the budget spends once
// per settled screen, and the request nests pixels inside the gate. A test of either would
// have passed on the broken build — and every existing test of this gate calls it directly,
// which is the same shape as the cycle-lock defect. So this drives a REAL registry with a
// REAL session through the production `request`, and asserts its premises rather than
// assuming them.
func TestTheSemanticRepairCanReadWhatItWasBoughtToRead(t *testing.T) {
	g, store := watchedRegistry(t)
	watchNow(t, g, store, "epicgameslauncher")
	rt := &Runtime{observations: g}

	// PREMISES, asserted. If the reading is not placed the gate returns at ignorance and
	// never reaches the repair; if it can say which state it is, no repair is bought at
	// all. Either way this test would pass without exercising the arm it is about.
	p := g.placeHere()
	if !p.Placed {
		t.Fatal("nothing is placed, so the gate returns at the ignorance arm and this " +
			"test proves nothing about repairs")
	}
	if observe.SemanticSufficiencyOf(p).Enough() {
		t.Fatal("the reading can say which state it is, so it buys no repair and this " +
			"test proves nothing")
	}

	s := &liveSampler{rt: rt, labels: true}
	// NOT a scheduled label sample. This is the whole point: the repair must not depend on
	// the cadence landing on the same sample.
	ordinary := observesession.SampleRequest{
		Window:     windowref.Ref{ID: directorapi.WindowID("hwnd:1")},
		ReadLabels: false,
	}

	first := s.request(ordinary)
	if !first.Includes(directorapi.SourceVision) {
		t.Fatal("the silent screen's one repair bought no visual pass at all")
	}
	if !first.Includes(directorapi.SourceOCR) {
		t.Error("the repair bought for semantic silence cannot read.\n" +
			"It is spent on a pass incapable of answering the question it was bought " +
			"for, and the screen stays nameless until it changes. On Epic that is the " +
			"difference between 31 anonymous boxes and `button \"Buy Now\"`.")
	}

	// AND IT IS STILL ONE PURCHASE. The reading rides on the repair; it does not remove
	// the bound. Every later ordinary sample on the same settled screen must be free.
	for i := 0; i < 12; i++ {
		again := s.request(ordinary)
		if again.Includes(directorapi.SourceOCR) {
			t.Fatalf("sample %d read again on one unchanged screen. That is continuous "+
				"OCR, which is the expense the budget exists to refuse.", i+2)
		}
		if again.Includes(directorapi.SourceVision) {
			t.Fatalf("sample %d bought a second inference on one unchanged screen", i+2)
		}
	}
}

// IGNORANCE BUYS A LOOK AND NEVER A READING.
//
// The other arm of the same gate, and the one that would turn this fix into "OCR on every
// sample". An unplaced reading is unplaced again next sample and the one after — it is
// unbounded by construction, so a reading budget attached to it never stops. Only the
// bounded repair may read.
//
// Deleting the distinction must fail this.
// EVERY ignorant branch is driven, because they are separate returns and a mutation that
// teaches one of them to read is invisible to a test that only reaches another. A first
// draft of this test used `&Runtime{}` alone, which returns at `observations == nil` — and a
// mutation setting Read on the `!p.Placed` arm survived it. That is the same branch, and the
// same escape, that TestTheSensorGateSpendsWhenItDoesNotKnow already had to name out loud.
func TestIgnoranceBuysALookAndNeverAReading(t *testing.T) {
	ordinary := observesession.SampleRequest{
		Window:     windowref.Ref{ID: directorapi.WindowID("hwnd:1")},
		ReadLabels: false,
	}
	for _, ignorant := range []struct {
		what string
		rt   *Runtime
	}{
		{"a Director with nothing watching", &Runtime{}},
		{"a nil Director", nil},
		{"a registry that has settled on nothing",
			&Runtime{observations: &observationRegistry{}}},
	} {
		s := &liveSampler{rt: ignorant.rt, labels: true}
		for i := 0; i < 20; i++ {
			got := s.request(ordinary)
			if !got.Includes(directorapi.SourceVision) {
				t.Fatalf("%s: sample %d declined the visual pass while knowing nothing",
					ignorant.what, i+1)
			}
			if got.Includes(directorapi.SourceOCR) {
				t.Fatalf("%s: sample %d read labels out of ignorance. Ignorance recurs "+
					"on every sample, so this is the continuous OCR the whole design "+
					"refuses — and it is what a repair-shaped fix turns into if the "+
					"two arms are conflated.", ignorant.what, i+1)
			}
		}
		// AND A SCHEDULED LABEL SAMPLE STILL READS, unchanged: the cadence is not what
		// moved.
		scheduled := ordinary
		scheduled.ReadLabels = true
		if !s.request(scheduled).Includes(directorapi.SourceOCR) {
			t.Errorf("%s: an ordinary scheduled label sample stopped reading; "+
				"LabelEvery behaviour was supposed to be untouched", ignorant.what)
		}
	}
}
