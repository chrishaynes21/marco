package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// What the discovery graph is entitled to claim about an edge.
//
// The tests in input_test.go establish that navigation reaches a transition at all. These are
// about the honesty of what it says once it gets there: an edge nobody caused, an edge with two
// competing explanations, an order that must not be flattened, and a keypress that must not be
// allowed to attach itself to whatever change happens next.
//
// Every one of them is a way the correlation could look stronger than the evidence, which is
// the failure mode that matters here — a hypothesis layer is about to be built on top of this,
// and it can only be as honest as the substrate underneath it.

// edgeWith returns the transition carrying the most support for an intent.
func edgeWith(t *testing.T, totals observe.ShadowTotals,
	intent observe.NavIntent) observe.ScreenTransition {

	t.Helper()
	var best observe.ScreenTransition
	for _, tr := range totals.Transitions {
		if tr.Preceded[intent] > best.Preceded[intent] {
			best = tr
		}
	}
	if best.Count == 0 {
		t.Fatalf("no transition carries %q at all. transitions=%v", intent, totals.Transitions)
	}
	return best
}

// menuCycle is one open-the-menu / close-the-menu round trip.
//
// Only the OPENING carries input, so the edge under test is unambiguous: the menu→gameplay
// edge closing each cycle is deliberately left unattributed, which also supplies the control
// every one of these tests needs anyway.
//
// Written as a helper because the interesting tests are about what happens when the SAME edge
// recurs, and a scenario spelled out three times invites a typo that reads as a finding.
func menuCycle(open []observe.InputEvent) []observe.ShadowSample {
	return []observe.ShadowSample{
		withInput(open, menuRegions()...), menu(),
		gameplay(), gameplay(),
	}
}

func pause() []observe.InputEvent { return []observe.InputEvent{nav(observe.NavPause)} }

// ── Part 15: navigation that changed nothing ──────────────────────────────────

// Pressing a direction that moves a selection without changing the screen must not invent an
// edge.
//
// The temptation this guards against is real: input is the scarce signal, and a correlation
// engine that "found" something whenever a key was pressed would produce a dense, confident,
// entirely fictional graph. An intent with no accompanying change is evidence about the player
// and evidence about nothing else.
func TestNavigationWithNoScreenChangeInventsNoEdge(t *testing.T) {
	down := []observe.InputEvent{nav(observe.NavDown), nav(observe.NavDown)}

	// A stable menu throughout: the player moves the selection twice and the composition
	// never changes.
	totals := fold(
		menu(), menu(),
		withInput(down, menuRegions()...),
		menu(), menu(),
	)

	for _, tr := range totals.Transitions {
		if len(tr.Preceded) > 0 {
			t.Errorf("an edge %s→%s was attributed to %v, but the screen composition never "+
				"changed — the graph has invented a transition out of a keypress",
				tr.From, tr.To, tr.Preceded)
		}
	}
	// And the producer's own account still shows the input existed. "No edge" must mean
	// "nothing changed", never "nothing was observed".
	if totals.Input.Classified != 0 || totals.Input.Received != 0 {
		t.Log("producer counters are supplied by the platform adapter, not by this fold")
	}
}

// ── Part 16: an edge that recurs ──────────────────────────────────────────────

// The same change, preceded by the same intent three times, then once by nothing.
//
// This is the shape a hypothesis will eventually be built from, and the arithmetic has to hold
// at both ends: three of three is a strong claim, three of four is a weaker one, and the report
// must be able to tell them apart. An implementation that counted only attributed changes would
// render both as "pause 3/3" and quietly delete the counter-example.
func TestRepeatedTransitionsAccumulateTheirSupport(t *testing.T) {
	var samples []observe.ShadowSample
	samples = append(samples, gameplay(), gameplay())
	for i := 0; i < 3; i++ {
		samples = append(samples, menuCycle(pause())...)
	}
	supported := fold(samples...)

	edge := edgeWith(t, supported, observe.NavPause)
	if got := edge.Preceded[observe.NavPause]; got != edge.Count {
		t.Fatalf("edge %s→%s: pause %d/%d, want every observation attributed",
			edge.From, edge.To, got, edge.Count)
	}
	if edge.Count < 3 {
		t.Fatalf("edge %s→%s recurred only %d times; the scenario opens the menu three "+
			"times", edge.From, edge.To, edge.Count)
	}
	if edge.Unattributed != 0 {
		t.Errorf("edge %s→%s reports %d unattributed changes in a session where every "+
			"change followed a pause", edge.From, edge.To, edge.Unattributed)
	}
	before := edge.Count

	// Now the same change once more with NO input before it.
	samples = append(samples, menuCycle(nil)...)
	mixed := fold(samples...)
	edge = edgeWith(t, mixed, observe.NavPause)

	if edge.Count != before+1 {
		t.Fatalf("edge %s→%s counted %d times after a fourth observation, want %d",
			edge.From, edge.To, edge.Count, before+1)
	}
	if got := edge.Preceded[observe.NavPause]; got != before {
		t.Errorf("edge %s→%s: pause support %d/%d, want %d/%d — an unattributed observation "+
			"must not increase the support for an intent",
			edge.From, edge.To, got, edge.Count, before, edge.Count)
	}
	if edge.Unattributed != 1 {
		t.Errorf("edge %s→%s: unattributed %d, want 1. A change nobody was seen to cause is "+
			"the evidence AGAINST reading the other three as caused, and dropping it is how "+
			"a correlation becomes a claim", edge.From, edge.To, edge.Unattributed)
	}
	if got := edge.Attributed(); got != before {
		t.Errorf("attributed %d, want %d", got, before)
	}
}

// ── Part 17: two explanations for one edge ────────────────────────────────────

// An edge preceded sometimes by pause and sometimes by confirm has no single cause, and the
// substrate must be able to say so.
//
// Dominant() answers "what usually preceded this". That is a useful question and a dangerous
// one: read alone it is indistinguishable from "what causes this". So the requirement is not
// that Dominant picks the right one — it is that the competing evidence is still THERE to be
// printed beside it.
func TestAnEdgeWithCompetingIntentsKeepsAllOfThem(t *testing.T) {
	confirm := []observe.InputEvent{nav(observe.NavConfirm)}

	var samples []observe.ShadowSample
	samples = append(samples, gameplay(), gameplay())
	for i := 0; i < 3; i++ {
		samples = append(samples, menuCycle(pause())...)
	}
	for i := 0; i < 2; i++ {
		samples = append(samples, menuCycle(confirm)...)
	}
	samples = append(samples, menuCycle(nil)...)

	edge := edgeWith(t, fold(samples...), observe.NavPause)

	intent, n := edge.Dominant()
	if intent != observe.NavPause {
		t.Errorf("dominant intent %q with support %d, want pause — it preceded this change "+
			"three times against confirm's two", intent, n)
	}
	if n != 3 {
		t.Errorf("dominant support %d, want 3", n)
	}
	if got := edge.Preceded[observe.NavConfirm]; got != 2 {
		t.Errorf("confirm support %d, want 2. The losing explanation must survive in the "+
			"record: an edge with a 3-to-2 split is a different finding from a unanimous "+
			"one, and only the competing evidence separates them", got)
	}
	if edge.Unattributed != 1 {
		t.Errorf("unattributed %d, want 1", edge.Unattributed)
	}
	if edge.Count != 6 {
		t.Errorf("edge observed %d times, want 6", edge.Count)
	}
	// The arithmetic a reader will do in their head has to work out.
	if edge.Attributed()+edge.Unattributed != edge.Count {
		t.Errorf("attributed %d + unattributed %d != count %d",
			edge.Attributed(), edge.Unattributed, edge.Count)
	}
}

// ── Part 18: the order ────────────────────────────────────────────────────────

// `down, down, confirm` must not become an unordered bag of intents.
//
// This is the seam the next milestone but one needs. Reconstructing "move the selection twice,
// then confirm" is possible from an ordered run and impossible from {down: 1, confirm: 1} — and
// the information is not recoverable later, so the decision to keep it has to be made here or
// not at all.
//
// Note what is NOT being asked for: a transcript. The run is bounded, the vocabulary is closed,
// and adjacent repeats are kept only because a repeat is a second deliberate press — the
// producer already collapsed a HELD key to one intent before this layer saw it.
func TestTheOrderOfNavigationSurvivesOnTheEdge(t *testing.T) {
	run := []observe.InputEvent{
		nav(observe.NavDown), nav(observe.NavDown), nav(observe.NavConfirm),
	}
	totals := fold(
		gameplay(), gameplay(),
		withInput(run, menuRegions()...),
		menu(), menu(),
	)

	edge := edgeWith(t, totals, observe.NavConfirm)
	if len(edge.Sequences) == 0 {
		t.Fatalf("edge %s→%s recorded which intents preceded it (%v) but not in what order; "+
			"down,down,confirm and confirm,down,down are now indistinguishable",
			edge.From, edge.To, edge.Preceded)
	}
	want := []observe.NavIntent{observe.NavDown, observe.NavDown, observe.NavConfirm}
	var found bool
	for _, s := range edge.Sequences {
		if s.Equal(want) {
			found = true
		}
	}
	if !found {
		t.Errorf("edge %s→%s sequences %v, want one equal to %v",
			edge.From, edge.To, edge.Sequences, want)
	}
	// The unordered view still counts each intent once, so a repeat cannot inflate support.
	if got := edge.Preceded[observe.NavDown]; got != 1 {
		t.Errorf("down support %d, want 1 — the ordered run must not double-count into the "+
			"support figures", got)
	}
}

// A recurring order is counted, not re-listed.
func TestARecurringOrderIsCountedOnce(t *testing.T) {
	run := []observe.InputEvent{nav(observe.NavDown), nav(observe.NavConfirm)}

	var samples []observe.ShadowSample
	samples = append(samples, gameplay(), gameplay())
	for i := 0; i < 3; i++ {
		samples = append(samples, menuCycle(run)...)
	}
	edge := edgeWith(t, fold(samples...), observe.NavConfirm)

	var total int
	for _, s := range edge.Sequences {
		total += s.Count
		if len(s.Intents) > observe.MaxSequenceLength {
			t.Errorf("a remembered order is %d intents long, past the %d bound — an edge "+
				"must not become a transcript", len(s.Intents), observe.MaxSequenceLength)
		}
	}
	if len(edge.Sequences) > observe.MaxSequencesPerTransition {
		t.Errorf("edge remembers %d distinct orders, past the %d bound",
			len(edge.Sequences), observe.MaxSequencesPerTransition)
	}
	if total != edge.Count {
		t.Errorf("orders account for %d observations against %d changes", total, edge.Count)
	}
	if len(edge.Sequences) != 1 {
		t.Errorf("the same order observed %d times produced %d entries, want 1 with a count",
			edge.Count, len(edge.Sequences))
	}
}

// ── Part 14, reinforced: no nearest-neighbour attribution ─────────────────────

// A keypress that preceded no change must not be held in reserve and attached to a later one.
//
// The bug this forbids is seductive because it improves the numbers: input is scarce, edges are
// interesting, and reaching a little further back to explain one is exactly what a correlation
// engine should never do. Evidence is consumed by the inference it was observed during.
func TestAnOldKeypressIsNotForcedOntoALaterEdge(t *testing.T) {
	totals := fold(
		gameplay(), gameplay(),
		// The player presses confirm during gameplay. The composition is IDENTICAL to the
		// frames either side of it, so nothing changed and there is no edge to attribute
		// it to. (A sample with no regions at all would be a different screen, and
		// attributing the confirm to THAT change would be correct — which is why the
		// regions here match gameplay's exactly.)
		withInput([]observe.InputEvent{nav(observe.NavConfirm)}, hudOnly()...),
		gameplay(), gameplay(),
		// Much later, the screen changes with no input at all.
		menu(), menu(), menu(),
	)

	for _, tr := range totals.Transitions {
		if n := tr.Preceded[observe.NavConfirm]; n > 0 {
			t.Errorf("edge %s→%s was attributed to a confirm pressed several inferences "+
				"earlier, during a period when nothing changed. Reaching backwards for the "+
				"nearest available intent is how a correlation engine explains everything "+
				"and measures nothing", tr.From, tr.To)
		}
	}
}

// ── the producer's own account ────────────────────────────────────────────────

// THE regression behind this milestone's first defect: the counters existed and reached nothing.
//
// A session that classified no intents has two explanations and they call for opposite
// responses — the player pressed nothing, or nothing was listening. For a while the second was
// unreportable: the Windows backend counted dropped events into a package-level atomic that
// Stats() did not read, and no report called Stats() at all.
func TestTheProducerCountersReachTheTotals(t *testing.T) {
	s := valid(hudOnly()...)
	s.InputStats = &observe.InputStats{
		Received: 12, Classified: 3, Dropped: 2,
		Ignored: map[observe.IgnoreReason]int{
			observe.IgnoreUnsupported: 6,
			observe.IgnoreRepeat:      1,
		},
	}
	totals := fold(gameplay(), s)

	if totals.Input.Classified != 3 || totals.Input.Received != 12 {
		t.Errorf("producer counters %+v did not reach the session totals", totals.Input)
	}
	if totals.Input.Dropped != 2 {
		t.Errorf("dropped %d, want 2. Backpressure thins a correlation, and a report that "+
			"cannot see it reads the loss as a player who pressed nothing",
			totals.Input.Dropped)
	}
	if got := totals.Input.IgnoredTotal(); got != 7 {
		t.Errorf("ignored total %d, want 7", got)
	}
}

// A skipped slot still carries the producer's counters.
//
// Drops happen when the machine is busy, which is exactly when the cadence gate declines slots.
// Folding the counters after the skipped-slot return would lose the diagnostic in precisely the
// conditions that produce it.
func TestProducerCountersSurviveASkippedSlot(t *testing.T) {
	s := skipped()
	s.InputStats = &observe.InputStats{Received: 40, Classified: 1, Dropped: 9}

	totals := fold(gameplay(), s, gameplay())
	if totals.Input.Dropped != 9 {
		t.Errorf("dropped %d, want 9 — the counters were folded after the skipped-slot "+
			"return, which is the one slot most likely to carry them", totals.Input.Dropped)
	}
}

// An unrecognised ignore reason is dropped rather than carried.
//
// The reason vocabulary crosses the same boundary the intents do, and IgnoreReason's underlying
// type is a string. Admission is what stops a platform adapter putting arbitrary text into a
// privacy-bounded diagnostic by way of a "reason".
func TestAnUnknownIgnoreReasonIsDropped(t *testing.T) {
	s := valid(hudOnly()...)
	s.InputStats = &observe.InputStats{
		Received: 2,
		Ignored: map[observe.IgnoreReason]int{
			observe.IgnoreRepeat:      1,
			"user typed hunter2 at 0": 1,
		},
	}
	totals := fold(gameplay(), s)

	for r := range totals.Input.Ignored {
		if !r.Known() {
			t.Errorf("ignore reason %q reached the session totals; it is outside the closed "+
				"vocabulary, which makes it free-form text in a keyboard observer's "+
				"diagnostic", r)
		}
		if strings.Contains(strings.ToLower(string(r)), "hunter2") {
			t.Error("a diagnostic string carried typed content into the report")
		}
	}
	if totals.Input.Ignored[observe.IgnoreRepeat] != 1 {
		t.Error("admission dropped a legitimate reason along with the illegitimate one")
	}
}

// ── Part 13: evidence admitted on context is weaker evidence ──────────────────

// condNav is an intent admitted only because the screen looked like a set of choices.
func condNav(i observe.NavIntent) observe.InputEvent {
	return observe.InputEvent{Intent: i, Conditional: true}
}

// An edge seen only through context-admitted keys must be counted apart from the rest.
//
// W preceding a change while a menu was up is real evidence and weaker evidence: the key also
// means "walk forwards", and whether it was navigation rests on an assessment of the screen made
// up to one observation earlier. Folding those in with the unambiguous ones would let a session
// of somebody walking around produce the same confident edge as one of somebody using a menu.
func TestAnEdgeSeenOnlyThroughContextAdmittedKeysIsCountedApart(t *testing.T) {
	var samples []observe.ShadowSample
	samples = append(samples, gameplay(), gameplay())
	for i := 0; i < 3; i++ {
		samples = append(samples, menuCycle([]observe.InputEvent{condNav(observe.NavUp)})...)
	}

	edge := edgeWith(t, fold(samples...), observe.NavUp)
	if edge.ConditionalOnly != edge.Attributed() {
		t.Errorf("edge %s→%s: %d of %d attributed observations counted as context-admitted, "+
			"want all of them", edge.From, edge.To, edge.ConditionalOnly, edge.Attributed())
	}
	if edge.Unattributed != 0 {
		t.Errorf("context-admitted evidence was counted as unattributed (%d); it is evidence, "+
			"just weaker evidence", edge.Unattributed)
	}
}

// One unambiguous key among them carries the observation on its own.
//
// The rule has to be "entirely", not "any". An edge preceded by Escape AND W is established by
// the Escape; discounting it because a second key was ambiguous would throw away the good
// evidence to punish the doubtful.
func TestOneUnambiguousKeyMakesAnObservationUnconditional(t *testing.T) {
	mixed := []observe.InputEvent{condNav(observe.NavUp), nav(observe.NavPause)}

	var samples []observe.ShadowSample
	samples = append(samples, gameplay(), gameplay())
	for i := 0; i < 3; i++ {
		samples = append(samples, menuCycle(mixed)...)
	}

	edge := edgeWith(t, fold(samples...), observe.NavPause)
	if edge.ConditionalOnly != 0 {
		t.Errorf("edge %s→%s counted %d observation(s) as context-admitted, but each one also "+
			"carried an unambiguous key", edge.From, edge.To, edge.ConditionalOnly)
	}
}

// A hypothesis resting ENTIRELY on context-admitted evidence may not reach `supported`.
//
// The whole claim depends on Marco having judged the screen correctly, from an observation up to
// one sampling interval earlier. That is worth reporting and is not worth acting on, and the
// difference between those two is what `contested` exists to express.
func TestAnActionRestingOnlyOnContextAdmittedKeysIsContested(t *testing.T) {
	var samples []observe.ShadowSample
	samples = append(samples, gameplay(), gameplay())
	for i := 0; i < 4; i++ {
		samples = append(samples, menuCycle([]observe.InputEvent{condNav(observe.NavUp)})...)
	}

	hs := observe.Hypotheses(fold(samples...), observe.DefaultHypothesisThresholds())
	h, ok := firstOf(hs, observe.PossibleTransitionAction)
	if !ok {
		t.Fatal("the edge produced no transition action at all; weaker evidence should be " +
			"reported and qualified, not discarded")
	}
	if h.Status == observe.StatusSupported {
		t.Errorf("an edge whose every observation was context-admitted reached SUPPORTED. "+
			"None of it is unambiguous navigation. contradictions=%v", h.Contradictions)
	}
	var explains bool
	for _, c := range h.Contradictions {
		if strings.Contains(c.Statement, "movement during play") {
			explains = true
		}
	}
	if !explains {
		t.Errorf("the hypothesis is qualified but does not say why: %v", h.Contradictions)
	}
}

// Partial context-admitted support is a caveat, not a contradiction.
//
// Three unambiguous observations and one context-admitted one is not a contested claim — the
// three stand on their own. But the reader must still be able to see the fourth.
func TestPartialContextAdmittedSupportIsStatedNotContested(t *testing.T) {
	var samples []observe.ShadowSample
	samples = append(samples, gameplay(), gameplay())
	for i := 0; i < 3; i++ {
		samples = append(samples, menuCycle(pause())...)
	}
	samples = append(samples, menuCycle([]observe.InputEvent{condNav(observe.NavPause)})...)

	hs := observe.Hypotheses(fold(samples...), observe.DefaultHypothesisThresholds())
	h, ok := firstOf(hs, observe.PossibleTransitionAction)
	if !ok {
		t.Fatal("no transition action")
	}
	for _, c := range h.Contradictions {
		if strings.Contains(c.Statement, "every one of these observations") {
			t.Error("a single context-admitted observation among four contested the whole " +
				"edge; the three unambiguous ones stand on their own")
		}
	}
	var stated bool
	for _, e := range h.Support {
		if strings.Contains(e.Statement, "only navigation while a set of choices") {
			stated = true
		}
	}
	if !stated {
		t.Errorf("the context-admitted observation is invisible in the support: %v", h.Support)
	}
}

// ── an interaction is longer than one sampling window ─────────────────────────

// `down down enter` is ONE interaction and must be recorded as three intents in order.
//
// # What this cost
//
// Navigation used to be drained on every inference, and `note` discards the intents of an
// inference where the screen did not change. A person pressing three keys takes about a second;
// the sampler runs about twice a second; so the first two presses landed in inferences that
// changed nothing and were thrown away. Every multi-key interaction in this system was recorded
// as its final key alone.
//
// Live, that turned `down down enter` into a learned route of `confirm` — press Enter on whatever
// happens to be focused. It replayed onto a different screen and the rehearsal correctly refused
// to believe it. The person's demonstration was fine; the model could not hold an interaction
// longer than one window.
func TestAMultiKeyInteractionIsRecordedWhole(t *testing.T) {
	totals := fold(
		gameplay(), gameplay(),
		withInput([]observe.InputEvent{nav(observe.NavDown)}, hudOnly()...),
		withInput([]observe.InputEvent{nav(observe.NavDown)}, hudOnly()...),
		withInput([]observe.InputEvent{nav(observe.NavConfirm)}, menuRegions()...),
		menu(), menu(),
	)

	var edge *observe.ScreenTransition
	for i := range totals.Transitions {
		if totals.Transitions[i].To != observe.ScreenStateUnknown {
			edge = &totals.Transitions[i]
		}
	}
	if edge == nil {
		t.Fatalf("no transition was recorded: %+v", totals.Transitions)
	}
	if edge.Preceded[observe.NavDown] == 0 {
		t.Errorf("the arrows that moved the selection are missing from %v.\nThey were pressed "+
			"during inferences where the composition did not change, and a route of `confirm` "+
			"alone means \"press Enter on whatever is focused\" — which is not what anybody "+
			"demonstrated.", edge.Preceded)
	}
	if edge.Preceded[observe.NavConfirm] == 0 {
		t.Errorf("the confirm is missing from %v", edge.Preceded)
	}
	// ORDER, which is the part a procedure is actually made of.
	if len(edge.Sequences) == 0 || len(edge.Sequences[0].Intents) < 3 {
		t.Errorf("the recorded order is %+v, want down, down, confirm — three intents in the "+
			"order they were pressed", edge.Sequences)
	}
}

// A pause breaks the run, so an interaction cannot absorb whatever came before it.
//
// The counterweight. Holding navigation across inferences is only honest while the person is
// still going; an inference that carried nothing says they stopped, and crediting the keys before
// a pause to a change after it is the nearest-neighbour attribution this file forbids.
func TestAPauseEndsTheRun(t *testing.T) {
	totals := fold(
		gameplay(),
		withInput([]observe.InputEvent{nav(observe.NavLeft)}, hudOnly()...),
		gameplay(), gameplay(), // the person stopped
		withInput([]observe.InputEvent{nav(observe.NavConfirm)}, menuRegions()...),
		menu(), menu(),
	)
	for _, tr := range totals.Transitions {
		if tr.Preceded[observe.NavLeft] > 0 {
			t.Errorf("a key pressed before a pause was credited to a change after it: %v",
				tr.Preceded)
		}
	}
}

// A pause BETWEEN keystrokes does not break the run; stopping does.
//
// Strict silence was tried first and was too strict for a person. At two samples a second, a
// human pressing `down down enter` leaves a quiet inference between presses — so every real
// demonstration still collapsed to its last key, which is the defect this was meant to fix.
//
// The tolerance is the one this file already gives a blinking track: "one dip should not split an
// episode in two; two consecutive misses is the detector agreeing with itself that the thing has
// gone." A pause is one quiet inference. Stopping is several.
func TestAPauseBetweenKeystrokesDoesNotBreakTheRun(t *testing.T) {
	totals := fold(
		gameplay(),
		withInput([]observe.InputEvent{nav(observe.NavDown)}, hudOnly()...),
		gameplay(), // the beat between presses
		withInput([]observe.InputEvent{nav(observe.NavDown)}, hudOnly()...),
		gameplay(), // and another
		withInput([]observe.InputEvent{nav(observe.NavConfirm)}, menuRegions()...),
		menu(), menu(),
	)
	var edge *observe.ScreenTransition
	for i := range totals.Transitions {
		if totals.Transitions[i].To != observe.ScreenStateUnknown {
			edge = &totals.Transitions[i]
		}
	}
	if edge == nil {
		t.Fatalf("no transition: %+v", totals.Transitions)
	}
	if edge.Preceded[observe.NavDown] == 0 {
		t.Errorf("arrows pressed with a natural beat between them were dropped: %v.\nNobody "+
			"presses three keys inside half a second, and requiring it makes every real "+
			"demonstration collapse to its final key.", edge.Preceded)
	}
}

// An effect that arrives one inference after its cause is still attributed to it.
//
// THE live defect, 2026-08-17. The attribution rule required the input and the change to land
// in the same sample, which fits a keyboard menu that flips the instant the key goes down and
// does not fit click-driven desktop software. A person clicked through to a Settings page,
// both transitions were recorded, and both came out `unattributed 2/2` — the route discovered
// and unlearnable, so Learn asked for a second demonstration.
//
// The allowance is exactly one quiet inference, bounded by TrackAbsenceTolerance.
func TestAChangeOneInferenceAfterTheClickIsAttributedToIt(t *testing.T) {
	// A page: the click lands while the old screen is still up, and the new composition is
	// only visible on the NEXT inference. Nothing is pressed in between.
	totals := fold(
		valid(hudOnly()...), valid(hudOnly()...),
		withInput([]observe.InputEvent{nav(observe.NavPoint)}, hudOnly()...),
		valid(menuRegions()...), valid(menuRegions()...), valid(menuRegions()...),
	)
	attributed := false
	for _, tr := range totals.Transitions {
		if tr.Preceded[observe.NavPoint] > 0 {
			attributed = true
		}
	}
	if !attributed {
		t.Fatalf("the click that caused the change was credited to nothing.\n"+
			"An interface that takes a beat to redraw puts the cause and its effect in "+
			"consecutive samples, and requiring them to share one makes every "+
			"click-driven demonstration unlearnable.\ntransitions: %+v", totals.Transitions)
	}
}
