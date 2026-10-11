package observe_test

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// ASSOCIATING A DIRECTION WITH A MOVE THE PIXELS ALREADY PROVED.
//
// ADR-160 left a route that could be described and not performed: it could say "the mark went
// entry 1 to 2 to 3" and not what to press. The missing half is the direction, and the whole
// difficulty is the ORDER in which it may arrive — the selection reader is handed a count of
// directional intents and never a direction, so that an input can never predict the visual
// outcome it is correlated with.
//
// So every test here asks one of two questions. Does the direction reach the record, survive
// the durable boundary and come back out on a plan? And can it still never reach anything that
// reads a mark? The second is the invariant, and it is
// TestTheDirectionOfAPressChangesNothingAboutWhatIsSeen.
//
// Everything enters through ShadowTotals.Observe, the call the live cycle makes.

// walkWithDirections walks the six-row list pressing one intent per step, and returns the
// session.
//
// The settle phase is not padding. A row's unselected level is learned only from the looks in
// which it was NOT marked, so nothing reads as selected until every row has been quiet several
// times — and a fixture that skipped it would be asserting about a belief that never formed.
//
// `press` is applied on the look the mark MOVES on, which is where a person's press lands. The
// move is confirmed a look or two later, because the reader refuses a row it has seen once, and
// the window is the look interval plus the repaint — 600ms against a 200ms cadence — so the
// press is still attributable when the confirmation arrives. That timing is the whole join.
func walkWithDirections(t *testing.T, laps int, press observe.NavIntent) observe.ShadowTotals {
	t.Helper()
	return walkPressing(t, laps, press)
}

// walkPressing is the same walk with however many intents land on the moving look.
//
// Several on one look is not a contrived case: at a live cadence a whole run of presses lands
// in one slot, which is exactly when the window cannot say which of them moved the mark.
func walkPressing(t *testing.T, laps int, press ...observe.NavIntent) observe.ShadowTotals {
	t.Helper()
	var totals observe.ShadowTotals
	seq := 0
	for range 14 {
		seq++
		totals.Observe(listScreen(seq, 0))
	}
	for range 6 {
		for mark := range 6 {
			for range 3 {
				seq++
				totals.Observe(listScreen(seq, mark))
			}
		}
	}
	for range laps {
		for mark := range 6 {
			seq++
			totals.Observe(listScreen(seq, mark, press...))
			for range 2 {
				seq++
				totals.Observe(listScreen(seq, mark))
			}
		}
	}
	return totals
}

// moveIn finds one retained move, or says what was retained instead.
func moveIn(t *testing.T, totals observe.ShadowTotals, from, to int) observe.SelectionTransition {
	t.Helper()
	for _, st := range totals.SelectionTransitions {
		if st.From == from && st.To == to {
			return st
		}
	}
	var had []string
	for _, st := range totals.SelectionTransitions {
		had = append(had, st.String())
	}
	t.Fatalf("no move from entry %d to entry %d was retained, so this says nothing about "+
		"its direction.\nretained: %s\nbelief=%s because=%s",
		from, to, strings.Join(had, " | "),
		totals.Interface.Status, totals.Interface.Because)
	return observe.SelectionTransition{}
}

// A move remembers which direction was observed before it.
//
// THE POSITIVE CASE, through the production fold. Before this a retained move said "a
// directional press was attributable" and had nowhere to say which one, so a chain of them
// named rows and no controls.
//
// Deleting the presses.bank call in ShadowTotals.Observe, or the attribute call in
// groundSelectionMoves, or the direction switch in selectionLedger.fold, must fail this.
func TestAMoveRemembersWhichDirectionWasObservedBeforeIt(t *testing.T) {
	totals := walkWithDirections(t, 4, observe.NavDown)

	st := moveIn(t, totals, 1, 2)
	if st.Prompted == 0 {
		t.Fatalf("the move was never prompted, so there was no window for a direction "+
			"to be found in: %s", st)
	}
	best, ok := observe.DominantDirection(st.Directions)
	if !ok {
		t.Fatalf("a move with %d prompted observation(s) named no direction at all: %s",
			st.Prompted, st)
	}
	if best.Intent != observe.NavDown {
		t.Errorf("the only direction ever pressed was down and the move names %q: %s",
			best.Intent, st)
	}
	if best.Count != st.Prompted {
		t.Errorf("down was attributed to %d of %d prompted observation(s); every one of "+
			"them had down in its window and nothing else: %s",
			best.Count, st.Prompted, st)
	}
	if len(st.Directions) != 1 {
		t.Errorf("one direction was ever pressed and %d are recorded: %s",
			len(st.Directions), st)
	}
	t.Logf("%s", st)
}

// Every prompted observation is accounted for, in exactly one of three places.
//
// The partition that makes the record checkable. `Prompted` says a press was attributable; this
// layer then names one direction, finds several, or cannot recover the slot — and if those three
// did not add up, a reader could not tell a direction that was never found from one that was
// found and dropped.
func TestEveryPromptedObservationIsAccountedFor(t *testing.T) {
	totals := walkWithDirections(t, 4, observe.NavDown)

	if len(totals.SelectionTransitions) == 0 {
		t.Fatal("no moves were retained, so there is nothing to account for")
	}
	for _, st := range totals.SelectionTransitions {
		named := 0
		for _, d := range st.Directions {
			named += d.Count
		}
		total := named + st.DirectionAmbiguous + st.DirectionUnknown
		if total != st.Prompted {
			t.Errorf("%s\n  named %d + ambiguous %d + unrecoverable %d = %d, and %d "+
				"observation(s) were prompted. A prompted observation that lands in "+
				"none of the three is a press the record silently lost",
				st, named, st.DirectionAmbiguous, st.DirectionUnknown, total,
				st.Prompted)
		}
		if st.Count != st.Prompted+st.Unprompted {
			t.Errorf("%s\n  prompted %d + unprompted %d is not the %d times the move "+
				"was seen", st, st.Prompted, st.Unprompted, st.Count)
		}
	}
}

// A session with no clock names no direction, and says so rather than guessing.
//
// The `direction_unknown` branch, reached the way a real session would reach it: without
// timestamps there is no order between a slot and a move, so "the last slot before it" would
// mean "any slot" — and the watch still reports every move as prompted, because a window that
// cannot age never closes. So this is the arrangement where the attribution is at its most
// willing and this layer must still refuse.
//
// It is also the only branch the pixel path can produce for a press that cannot be recovered,
// which is why it carries the counter's weight: deleting the DirectionUnknown increment must
// fail this.
func TestASessionWithNoClockNamesNoDirectionAndSaysSo(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	look := func(mark int, intents ...observe.NavIntent) {
		seq++
		s := listScreen(seq, mark, intents...)
		s.Timestamp = time.Time{}
		totals.Observe(s)
	}
	for range 14 {
		look(0)
	}
	for range 6 {
		for mark := range 6 {
			for range 3 {
				look(mark)
			}
		}
	}
	for range 4 {
		for mark := range 6 {
			look(mark, observe.NavDown)
			look(mark)
			look(mark)
		}
	}

	if len(totals.SelectionTransitions) == 0 {
		t.Fatalf("no moves were retained, so this says nothing. belief=%s because=%s",
			totals.Interface.Status, totals.Interface.Because)
	}
	var prompted, unknown, named int
	for _, st := range totals.SelectionTransitions {
		prompted += st.Prompted
		unknown += st.DirectionUnknown
		for _, d := range st.Directions {
			named += d.Count
		}
	}
	if prompted == 0 {
		t.Fatalf("no observation was prompted, so there was nothing to refuse: %s",
			totals.SelectionTransitions[0])
	}
	if named != 0 {
		t.Errorf("%d observation(s) named a direction in a session with no clock. "+
			"Without a timestamp the slot that opened the window is unknowable, and "+
			"picking one is how a direction banked long after the mark moved gets "+
			"credited: %s", named, totals.SelectionTransitions[0])
	}
	if unknown != prompted {
		t.Errorf("%d of %d prompted observation(s) are recorded as unrecoverable; every "+
			"one of them should be. A prompted observation in none of the three "+
			"counters is a press the record silently lost", unknown, prompted)
	}
	if got := totals.SelectionTransitions[0].Support(); got != observe.BindingUnattributed {
		t.Errorf("the support reads %q; a session that could name no direction at all "+
			"has not bound one", got)
	}
	t.Logf("%s", totals.SelectionTransitions[0])
}

// Two directions pressed together name neither, through a whole session.
//
// The `direction_ambiguous` branch on the production path. The ledger's own unit test asserts
// that `attribute` refuses; this asserts that the refusal is COUNTED, so the three counters
// still partition `Prompted` when the middle one is the one being used.
//
// Deleting the DirectionAmbiguous increment must fail this.
func TestAWalkPressedTwoWaysAtOnceNamesNoDirection(t *testing.T) {
	totals := walkPressing(t, 4, observe.NavDown, observe.NavRight)

	if len(totals.SelectionTransitions) == 0 {
		t.Fatalf("no moves were retained, so this says nothing. belief=%s because=%s",
			totals.Interface.Status, totals.Interface.Because)
	}
	var prompted, ambiguous, named int
	for _, st := range totals.SelectionTransitions {
		prompted += st.Prompted
		ambiguous += st.DirectionAmbiguous
		for _, d := range st.Directions {
			named += d.Count
		}
	}
	if prompted == 0 {
		t.Fatalf("no observation was prompted: %s", totals.SelectionTransitions[0])
	}
	if named != 0 {
		t.Errorf("%d observation(s) named one direction while two were pressed every "+
			"time. A person pressing down and right has not said which moved the "+
			"mark: %s", named, totals.SelectionTransitions[0])
	}
	if ambiguous != prompted {
		t.Errorf("%d of %d prompted observation(s) are recorded as ambiguous; every one "+
			"of them should be", ambiguous, prompted)
	}
	if got := totals.SelectionTransitions[0].Support(); got != observe.BindingAmbiguous {
		t.Errorf("the support reads %q, want %q — and the difference matters, because "+
			"a window holding several directions calls for a finer cadence while "+
			"nothing attributable calls for a longer one", got, observe.BindingAmbiguous)
	}
	t.Logf("%s", totals.SelectionTransitions[0])
}

// The direction of a press changes NOTHING about what is seen.
//
// THE INVARIANT, and the reason this milestone does not weaken
// [[ADR-158-the-active-list-is-the-one-that-responds]]. The selection reader is handed a COUNT
// of directional intents; if a direction could reach it, an input could predict the visual
// outcome it is being correlated with, and an outcome that was predicted is not one that was
// observed.
//
// Two identical sessions, one navigated entirely with down and one entirely with up, over the
// same pixels. Every reading of the mark, every move, every count must match exactly. Only the
// attributed direction may differ.
//
// It is also the failure mode this file guards against being introduced LATER. The cheap future
// edit is to pass the ledger into the watch "so the tracker can weight a responsive list by
// whether the direction fits", and this is what would catch it.
func TestTheDirectionOfAPressChangesNothingAboutWhatIsSeen(t *testing.T) {
	down := walkWithDirections(t, 4, observe.NavDown)
	up := walkWithDirections(t, 4, observe.NavUp)

	if down.Interface.Selected != up.Interface.Selected ||
		down.Interface.Status != up.Interface.Status ||
		down.Interface.Entries != up.Interface.Entries {
		t.Fatalf("the belief depends on which direction was pressed.\n  down: %s entry "+
			"%d of %d\n  up:   %s entry %d of %d",
			down.Interface.Status, down.Interface.Selected, down.Interface.Entries,
			up.Interface.Status, up.Interface.Selected, up.Interface.Entries)
	}
	if len(down.SelectionTransitions) != len(up.SelectionTransitions) {
		t.Fatalf("%d move(s) were retained pressing down and %d pressing up, over "+
			"identical pixels", len(down.SelectionTransitions),
			len(up.SelectionTransitions))
	}
	for i, a := range down.SelectionTransitions {
		b := up.SelectionTransitions[i]
		if a.From != b.From || a.To != b.To || a.Entries != b.Entries ||
			a.Count != b.Count || a.Prompted != b.Prompted ||
			a.Unprompted != b.Unprompted || a.Tracked != b.Tracked {
			t.Errorf("move %d differs in something the pixels decided:\n  down: %s\n"+
				"  up:   %s", i, a, b)
		}
	}
	// And the one thing that MUST differ, so this test cannot pass by both being empty.
	ad, aok := observe.DominantDirection(down.SelectionTransitions[0].Directions)
	bd, bok := observe.DominantDirection(up.SelectionTransitions[0].Directions)
	if !aok || !bok || ad.Intent == bd.Intent {
		t.Fatalf("the two sessions recorded the same direction (%v/%v), so the pixels "+
			"being identical proves nothing about the join", ad.Intent, bd.Intent)
	}
}

// A press alone invents no move, and no direction.
//
// The mirror of the invariant above. ADR-158's rule has two halves: the input must not predict
// the outcome, and the input must not CREATE one. A session of somebody pressing down at a
// screen with nothing selectable on it must retain nothing at all — not a move with a direction
// attached, and not a direction with no move.
func TestADirectionIsNeverInventedWithoutAMove(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	// A composition with no list in it at all, pressed at furiously.
	for range 40 {
		seq++
		totals.Observe(otherScreen(seq, observe.NavDown, observe.NavDown))
	}
	if n := len(totals.SelectionTransitions); n != 0 {
		t.Fatalf("%d move(s) were retained on a screen with no list on it: %s",
			n, totals.SelectionTransitions[0])
	}
	for _, h := range observe.Hypotheses(totals, observe.DefaultHypothesisThresholds()) {
		if h.Kind == observe.PossibleSelectionTransition {
			t.Errorf("pressing down produced a move hypothesis with nothing moving: %s",
				h.Observed)
		}
	}
}

// A press observed AFTER a move is not its direction, and nor is one with no clock.
//
// The two refusals, and they are refusals rather than guesses. The fast reads for an interval
// are folded BEFORE that interval's own look, so a move can legitimately be prompted by a slot
// whose timestamp is later than the moment the move was seen — and crediting that direction is
// the input predicting the outcome, in the one place the ordering actually inverts.
//
// Asserted directly against the ledger rather than through a session, because arranging the
// inversion in pixels would be arranging a race. The unit is the join itself.
//
// Deleting the s.at.After(m.At) check, or the m.At.IsZero() check, must fail this.
func TestAPressObservedAfterAMoveIsNotItsDirection(t *testing.T) {
	base := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)

	seen := observe.LedgerCaseForTest{
		PressAt:  base.Add(900 * time.Millisecond),
		MoveAt:   base.Add(400 * time.Millisecond),
		Prompted: true, Intents: []observe.NavIntent{observe.NavDown},
	}
	if got := seen.Run(); got.Intent != "" || !got.Unknown {
		t.Errorf("a press observed 500ms AFTER the move was seen was credited as its "+
			"direction: intent=%q unknown=%v ambiguous=%v",
			got.Intent, got.Unknown, got.Ambiguous)
	}

	noClock := observe.LedgerCaseForTest{
		PressAt:  base,
		Prompted: true, Intents: []observe.NavIntent{observe.NavDown},
	}
	if got := noClock.Run(); got.Intent != "" || !got.Unknown {
		t.Errorf("a move with no timestamp took a direction from a slot it has no "+
			"order against: intent=%q unknown=%v", got.Intent, got.Unknown)
	}

	// The control: the same press, before the move, inside the ceiling.
	before := observe.LedgerCaseForTest{
		PressAt:  base,
		MoveAt:   base.Add(400 * time.Millisecond),
		Prompted: true, Intents: []observe.NavIntent{observe.NavDown},
	}
	if got := before.Run(); got.Intent != observe.NavDown {
		t.Errorf("the control case failed, so the refusals above prove nothing: "+
			"intent=%q unknown=%v ambiguous=%v",
			got.Intent, got.Unknown, got.Ambiguous)
	}

	// And a press so old the ceiling refuses it. Three seconds is TrackStaleness, which
	// is what window() clamps to rather than a second latency policy.
	stale := observe.LedgerCaseForTest{
		PressAt:  base,
		MoveAt:   base.Add(4 * time.Second),
		Prompted: true, Intents: []observe.NavIntent{observe.NavDown},
	}
	if got := stale.Run(); got.Intent != "" || !got.Unknown {
		t.Errorf("a press four seconds before the move was credited as its direction: "+
			"intent=%q unknown=%v", got.Intent, got.Unknown)
	}

	// A move the watch did not find a press for takes no direction at all — and is not
	// counted as unrecoverable either, because nothing was lost.
	quiet := observe.LedgerCaseForTest{
		PressAt: base,
		MoveAt:  base.Add(400 * time.Millisecond),
		Intents: []observe.NavIntent{observe.NavDown},
	}
	if got := quiet.Run(); got.Intent != "" || got.Unknown || got.Ambiguous {
		t.Errorf("an unprompted move was given a direction: intent=%q unknown=%v "+
			"ambiguous=%v", got.Intent, got.Unknown, got.Ambiguous)
	}
}

// A window holding several directions names none of them.
//
// Not a vote and not the first one. A person pressing down and right has not told Marco which
// moved the mark, and at a coarse cadence a whole walk lands in one slot — so naming one would
// be inventing the half that was not observed.
func TestAnAmbiguousWindowNamesNoDirection(t *testing.T) {
	base := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	both := observe.LedgerCaseForTest{
		PressAt:  base,
		MoveAt:   base.Add(300 * time.Millisecond),
		Prompted: true,
		Intents:  []observe.NavIntent{observe.NavDown, observe.NavRight},
	}
	got := both.Run()
	if got.Intent != "" || !got.Ambiguous {
		t.Errorf("two directions in one window produced %q: ambiguous=%v",
			got.Intent, got.Ambiguous)
	}
	// Two events of the SAME direction are not ambiguous. A held key is one direction.
	held := observe.LedgerCaseForTest{
		PressAt:  base,
		MoveAt:   base.Add(300 * time.Millisecond),
		Prompted: true,
		Intents:  []observe.NavIntent{observe.NavDown, observe.NavDown},
	}
	if got := held.Run(); got.Intent != observe.NavDown || got.Ambiguous {
		t.Errorf("two presses of down read as ambiguous: intent=%q ambiguous=%v",
			got.Intent, got.Ambiguous)
	}
	// And a confirm never reaches the field at all: it changes which screen is in front
	// rather than which row is marked, so one here would be a join crediting a screen
	// change to a move of the mark.
	confirm := observe.LedgerCaseForTest{
		PressAt:  base,
		MoveAt:   base.Add(300 * time.Millisecond),
		Prompted: true,
		Intents:  []observe.NavIntent{observe.NavConfirm},
	}
	if got := confirm.Run(); got.Intent != "" || !got.Unknown {
		t.Errorf("a confirm was recorded as a direction the mark moved by: intent=%q",
			got.Intent)
	}
}

// Keys that are only navigation in context stay weaker evidence.
//
// [[ADR-013-navigation-is-meaning-not-keys]], carried this far. W while the screen looked like
// a set of choices is a real `up` and is also "walk forwards"; a binding learned from a session
// of somebody walking around must not read as confidently as one learned from arrows.
func TestConditionalKeysStayWeakerEvidence(t *testing.T) {
	base := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	soft := observe.LedgerCaseForTest{
		PressAt:  base,
		MoveAt:   base.Add(300 * time.Millisecond),
		Prompted: true,
		Intents:  []observe.NavIntent{observe.NavDown},
		Soft:     []bool{true},
	}
	if got := soft.Run(); got.Intent != observe.NavDown || !got.Conditional {
		t.Errorf("a context-admitted press lost its qualification: intent=%q "+
			"conditional=%v", got.Intent, got.Conditional)
	}
	// One unambiguous press of the same direction clears it. "Conditional" has to mean
	// EVERY press behind this direction was a context-admitted key, or weak evidence
	// would be reported as though some of it were strong.
	mixed := observe.LedgerCaseForTest{
		PressAt:  base,
		MoveAt:   base.Add(300 * time.Millisecond),
		Prompted: true,
		Intents:  []observe.NavIntent{observe.NavDown, observe.NavDown},
		Soft:     []bool{true, false},
	}
	if got := mixed.Run(); got.Intent != observe.NavDown || got.Conditional {
		t.Errorf("a direction with one unambiguous press behind it is still reported as "+
			"context-only: conditional=%v", got.Conditional)
	}
	// And it survives into the record as its own counter rather than being folded in.
	dirs, _ := observe.FoldDirectionsForTest(nil, []observe.DirectionCount{
		{Intent: observe.NavDown, Count: 4, Conditional: 4},
	})
	if len(dirs) != 1 || dirs[0].Conditional != 4 {
		t.Fatalf("the durable fold dropped the qualification: %v", dirs)
	}
	if !strings.Contains(dirs[0].String(), "context-admitted") {
		t.Errorf("a reader cannot see that this evidence is weak: %q", dirs[0].String())
	}
}

// The support vocabulary refuses a thin lead and a contradiction.
//
// The classification is the one place a candidate becomes "worth proposing", so each of the
// five words has to be reachable and none of them may be reachable by the wrong evidence. A
// single confidence number would render the two that matter most — nobody pressed anything
// attributable, and the presses disagreed — as the same small float.
func TestBindingSupportRefusesAThinLeadAndAContradiction(t *testing.T) {
	down := func(n int) observe.DirectionCount {
		return observe.DirectionCount{Intent: observe.NavDown, Count: n}
	}
	up := func(n int) observe.DirectionCount {
		return observe.DirectionCount{Intent: observe.NavUp, Count: n}
	}
	cases := []struct {
		why       string
		dirs      []observe.DirectionCount
		ambiguous int
		unknown   int
		want      observe.BindingSupport
	}{
		{"nothing attributable at all", nil, 0, 0, observe.BindingUnattributed},
		{"a press nobody could recover is not a direction", nil, 0, 3,
			observe.BindingUnattributed},
		{"several directions every time", nil, 4, 0, observe.BindingAmbiguous},
		{"one observation is not a binding", []observe.DirectionCount{down(1)}, 0, 0,
			observe.BindingThin},
		{"two is still below the floor", []observe.DirectionCount{down(2)}, 0, 0,
			observe.BindingThin},
		{"three agreeing is the floor", []observe.DirectionCount{down(3)}, 0, 0,
			observe.BindingRepeated},
		{"an even split is a contradiction", []observe.DirectionCount{down(4), up(4)}, 0, 0,
			observe.BindingContradicted},
		{"a bare lead is a contradiction", []observe.DirectionCount{down(5), up(3)}, 0, 0,
			observe.BindingContradicted},
		{"one stray in nine does not poison it",
			[]observe.DirectionCount{down(9), up(1)}, 0, 0, observe.BindingRepeated},
		{"the margin applies before the floor",
			[]observe.DirectionCount{down(2), up(2)}, 0, 0, observe.BindingContradicted},
	}
	for _, c := range cases {
		got := observe.BindingSupportOf(c.dirs, c.ambiguous, c.unknown)
		if got != c.want {
			t.Errorf("%s: got %q, want %q (%v, ambiguous %d, unrecoverable %d)",
				c.why, got, c.want, c.dirs, c.ambiguous, c.unknown)
		}
		if got.Say() == string(got) {
			t.Errorf("%q has no sentence a person could read", got)
		}
	}
	if observe.BindingThin.Usable() || !observe.BindingRepeated.Usable() {
		t.Error("the usable line is not where the vocabulary says it is")
	}
}

// A chain is not performable until every hop names a direction.
//
// The honest answer to "could Marco be asked to take this route". A chain with one unnamed hop
// is a route that stops in the middle, and reporting it as performable is how a plan comes to
// be handed to something that cannot finish it.
//
// True here is still the absence of a known gap and never the presence of a verification.
func TestAChainIsNotPerformableUntilEveryHopNamesADirection(t *testing.T) {
	bound := func(from, to int) observe.SelectionStep {
		return observe.SelectionStep{
			From: from, To: to, Entries: 6, Observations: 4, Prompted: 4,
			Directions: []observe.DirectionCount{
				{Intent: observe.NavDown, Count: 4},
			},
		}
	}
	loose := observe.SelectionStep{From: 2, To: 3, Entries: 6, Observations: 4}

	if ok, weakest := observe.Performable(nil); !ok || weakest != observe.BindingRepeated {
		t.Errorf("an empty chain is not performable: ok=%v weakest=%q", ok, weakest)
	}
	if ok, _ := observe.Performable([]observe.SelectionStep{bound(0, 1), bound(1, 2)}); !ok {
		t.Error("a chain every hop of which names one direction four times over is " +
			"reported as having a gap")
	}
	ok, weakest := observe.Performable([]observe.SelectionStep{bound(0, 1), loose})
	if ok {
		t.Error("a chain with an unnamed hop is reported as performable")
	}
	if weakest != observe.BindingUnattributed {
		t.Errorf("the weakest hop is reported as %q, not the unattributed one that is "+
			"actually the gap", weakest)
	}
}
