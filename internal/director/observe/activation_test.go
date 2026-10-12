package observe_test

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// ACTIVATING AN ENTRY, WHICH IS NOT MOVING THE MARK.
//
// [[ADR-159]] made the mark's movement something a session keeps: entry 2 became entry 3, same
// screen throughout. That is half of using a menu and it never arrives anywhere. The other half
// is the screen changing while a known row was the selected one, and the two must not be
// substitutable — a route built out of the first alone walks a highlight up and down for ever.
//
// Everything here enters through ShadowTotals.Observe, the call the live cycle makes, because
// the rule can be perfect while nothing ever hands it a look.

// activationEpoch gives the looks a clock, so staleness and windows mean something.
var activationEpoch = time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)

// listScreen is one look at a six-row list with a fixed block beside it, at a given cadence.
//
// The same arrangement menuSample builds, with a clock of its own and an intent channel: this
// file needs the screen to CHANGE, which means two compositions and a settled gate, and it
// needs a press before the change so the navigation half of the evidence exists to be kept
// apart from the selection half.
func listScreen(seq, marked int, intents ...observe.NavIntent) observe.Sample {
	var regions []observe.ShadowRegion
	for i := range 6 {
		shade := []float64{60, 72}
		if i == marked {
			shade = []float64{205, 238}
		}
		regions = append(regions, observe.ShadowRegion{
			Role: "button",
			Region: observe.Region{
				X: 0.10, Y: 0.40 + float64(i)*0.06, Width: 0.40, Height: 0.05,
			},
			Shade: shade,
		})
	}
	regions = append(regions, observe.ShadowRegion{
		Role:   "image",
		Region: observe.Region{X: 0.60, Y: 0.10, Width: 0.30, Height: 0.25},
		Shade:  []float64{90, 110},
	})
	return activationSample(seq, regions, intents)
}

// otherScreen is a composition sharing no role with the list screen.
//
// Shares none deliberately. A sparse frame that is CONTAINED in a rich screen goes down the
// containment branch and is held for recurrence rather than minted, so a destination built out
// of fewer buttons would be refused as a partial rendering — which is the gate working, and it
// would make this fixture measure that instead of what it is for.
func otherScreen(seq int, intents ...observe.NavIntent) observe.Sample {
	var regions []observe.ShadowRegion
	for i := range 5 {
		regions = append(regions, observe.ShadowRegion{
			Role: "text",
			Region: observe.Region{
				X: 0.30, Y: 0.12 + float64(i)*0.09, Width: 0.35, Height: 0.06,
			},
			Shade: []float64{80, 95},
		})
	}
	regions = append(regions, observe.ShadowRegion{
		Role:   "field",
		Region: observe.Region{X: 0.30, Y: 0.70, Width: 0.35, Height: 0.08},
		Shade:  []float64{100, 120},
	})
	return activationSample(seq, regions, intents)
}

func activationSample(seq int, regions []observe.ShadowRegion,
	intents []observe.NavIntent) observe.Sample {

	var inputs []observe.InputEvent
	for i, in := range intents {
		inputs = append(inputs, observe.InputEvent{
			Intent: in, AtMS: int64(seq)*200 + int64(i)*10,
		})
	}
	return observe.Sample{
		Sequence:         seq,
		Timestamp:        activationEpoch.Add(time.Duration(seq) * 200 * time.Millisecond),
		WindowGeneration: 1,
		Shadow: &observe.ShadowSample{
			Detector: "test", Ran: true, TargetProven: true,
			Regions: regions, Inputs: inputs,
		},
	}
}

// activationRun walks the list, then activates one row, twice over.
//
// Twice because MinNavigationObservations is 2: a change seen once is not evidence about which
// row it came from, and a fixture that produced one would be asserting that the generator's
// own floor does not apply.
//
// `row` is the row the mark is parked on when the screen changes. The walk before it is not
// padding — a row's unselected level is learned only from the looks in which it was NOT
// marked, so nothing reads as selected until every row has been quiet several times.
func activationRun(t *testing.T, row int) observe.ShadowTotals {
	t.Helper()
	var totals observe.ShadowTotals
	seq := 0
	look := func(s observe.Sample) {
		totals.Observe(s)
	}
	// Settle the list screen and teach every row its unmarked level.
	for range 14 {
		seq++
		look(listScreen(seq, 0))
	}
	// THE WALK PRESSES A KEY WHEN IT MOVES THE MARK, and it did not.
	//
	// It got away with that because the tracker published a list that qualified alone
	// without asking whether anybody's navigation had ever reached it. Scored on 864 real
	// frames, that arm named the wrong list on 144 of 144 frames, so it now requires the
	// same responsiveness evidence the several-candidates arm always did — and this
	// fixture then described a list whose mark moves on its own schedule: 32 transfers,
	// one of them attributable. Which is exactly what the new rule refuses.
	//
	// A person walking a menu presses a key each time the mark moves. The press belongs on
	// the look that MOVES it, which is the first of each row's three.
	for range 6 {
		for mark := range 6 {
			for k := range 3 {
				seq++
				if k == 0 {
					look(listScreen(seq, mark, observe.NavDown))
					continue
				}
				look(listScreen(seq, mark))
			}
		}
	}
	for pass := range 2 {
		// Park the mark on the row that is about to be activated, long enough for the
		// reader to confirm it: it refuses a row it has seen only once.
		seq++
		look(listScreen(seq, row, observe.NavDown))
		for range 4 {
			seq++
			look(listScreen(seq, row))
		}
		if totals.Interface.Selected != row {
			t.Fatalf("pass %d: the mark reads as entry %d, not the %d it was parked "+
				"on, so this fixture cannot say what was selected when the screen "+
				"changed. belief=%s because=%s",
				pass, totals.Interface.Selected, row,
				totals.Interface.Status, totals.Interface.Because)
		}
		// THE ACTIVATION: confirm, and the screen becomes another one. Two looks,
		// because StabilityLooks is 2 and one divergent frame is not a change.
		seq++
		look(otherScreen(seq, observe.NavConfirm))
		for range 3 {
			seq++
			look(otherScreen(seq))
		}
		// Back to the list, so the second pass has somewhere to start.
		for range 6 {
			seq++
			look(listScreen(seq, row))
		}
	}
	return totals
}

// A confirmed screen change records which entry was selected when it happened.
//
// THE POINT OF THE WHOLE CHANGE, as one assertion. Before this a menu's nine rows all produced
// the same edge: `state_1 → state_2`, with no field in which "the third one" could be said.
//
// Deleting the rec.selected.fold call in ScreenSegmenter.note, or the SelectionAtFrom call in
// ShadowTotals.Observe, must fail this.
func TestAChangeRecordsWhatWasSelectedWhenItHappened(t *testing.T) {
	totals := activationRun(t, 3)

	var found bool
	for _, tr := range totals.Transitions {
		if len(tr.SelectedBefore) == 0 {
			continue
		}
		found = true
		best, ok := observe.DominantSelection(tr.SelectedBefore)
		if !ok {
			t.Fatalf("%s → %s carries %d selections and names none dominant",
				tr.From, tr.To, len(tr.SelectedBefore))
		}
		if best.Entry != 3 {
			t.Errorf("%s → %s was recorded with entry %d selected; the mark was parked "+
				"on entry 3 every time the screen changed. all: %v",
				tr.From, tr.To, best.Entry, tr.SelectedBefore)
		}
		if best.Entries != 6 {
			t.Errorf("%s → %s recorded a %d-entry list; the fixture draws six. "+
				"\"entry 3\" means nothing except against a stable count of them",
				tr.From, tr.To, best.Entries)
		}
		t.Logf("%s → %s: %d observation(s), selected %v, unknown %d",
			tr.From, tr.To, tr.Count, tr.SelectedBefore, tr.SelectedUnknown)
	}
	if !found {
		t.Fatalf("no transition recorded any selection at all. transitions=%d belief=%s "+
			"selected=%d: a menu whose rows are indistinguishable in the graph is a "+
			"menu no route can name a row of",
			len(totals.Transitions), totals.Interface.Status, totals.Interface.Selected)
	}
}

// Activating an entry is its own kind of hypothesis, and not a move of the mark.
//
// The distinction the whole file exists for, checked where a consumer would read it: the two
// kinds carry DIFFERENT subjects. A move names one screen on both sides because the screen did
// not change; an activation names two because it did. A reader must be able to tell them apart
// without consulting the Kind at all.
//
// Deleting the entryActivations call in Hypotheses must fail this.
func TestActivatingAnEntryIsItsOwnKindOfTransition(t *testing.T) {
	totals := activationRun(t, 3)
	hs := observe.Hypotheses(totals, observe.DefaultHypothesisThresholds())

	var activation, move *observe.Hypothesis
	for i := range hs {
		switch hs[i].Kind {
		case observe.PossibleEntryActivation:
			if activation == nil {
				activation = &hs[i]
			}
		case observe.PossibleSelectionTransition:
			if move == nil {
				move = &hs[i]
			}
		}
	}
	if activation == nil {
		var kinds []string
		for _, h := range hs {
			kinds = append(kinds, string(h.Kind))
		}
		t.Fatalf("a screen change observed twice with entry 3 selected produced no "+
			"%s hypothesis. kinds: %s",
			observe.PossibleEntryActivation, strings.Join(kinds, ", "))
	}
	if activation.Subject.Kind != observe.SubjectTransition {
		t.Errorf("an activation's subject is %q; it has to be a %q, because the screen "+
			"DID change", activation.Subject.Kind, observe.SubjectTransition)
	}
	if activation.Subject.Ref == activation.Subject.To {
		t.Errorf("an activation names %q on both sides. That is how a reader tells a move "+
			"of the mark from a change of screen without reading the Kind, and this one "+
			"reads as a move", activation.Subject.Ref)
	}
	if activation.Subject.Selection == nil {
		t.Fatal("an activation carries no entries, so nothing downstream can say WHICH " +
			"row it was taken from — which is the only form of the answer a route can use")
	}
	if got := activation.Subject.Selection.From; got != 3 {
		t.Errorf("the activation names entry %d; the mark was on entry 3", got)
	}
	// AND THE OTHER KIND, in the same session, still naming one screen twice.
	if move != nil && move.Subject.Ref != move.Subject.To {
		t.Errorf("a move of the mark names %q and %q — two screens. The screen does not "+
			"change when a highlight moves, and that is the whole distinction",
			move.Subject.Ref, move.Subject.To)
	}
	t.Logf("activation: %s\nvalidation: %s", activation.Observed, activation.Validation)
}

// An activation hypothesis claims association, never cause.
//
// The one sentence this layer must not produce. "Confirming on entry 3 opens that screen" needs
// the action performed and the outcome checked — a rehearsal, which is not an observation — and
// a generator that wrote it would have manufactured a causal claim out of two timestamps.
func TestAnActivationHypothesisClaimsAssociationAndNotCause(t *testing.T) {
	totals := activationRun(t, 2)
	hs := observe.Hypotheses(totals, observe.DefaultHypothesisThresholds())

	banned := []string{"caused", "causes", "because the", "opens that screen when",
		"will open", "would open", "selecting it opens", "confirms to"}
	var checked int
	for _, h := range hs {
		if h.Kind != observe.PossibleEntryActivation {
			continue
		}
		checked++
		prose := strings.ToLower(h.Observed + " " + h.Validation)
		for _, e := range h.Support {
			prose += " " + strings.ToLower(e.Statement)
		}
		for _, e := range h.Contradictions {
			prose += " " + strings.ToLower(e.Statement)
		}
		for _, bad := range banned {
			if strings.Contains(prose, bad) {
				t.Errorf("an activation hypothesis says %q. The row was selected while "+
					"the screen changed; that it opened it is a separate claim nothing "+
					"here establishes.\nprose: %s", bad, prose)
			}
		}
		// AND the validation must say what a person would have to DO, because the field
		// is what stops the hypothesis being read as settled.
		if !strings.Contains(strings.ToLower(h.Validation), "check") {
			t.Errorf("the validation does not ask anybody to check anything: %q",
				h.Validation)
		}
	}
	if checked == 0 {
		t.Fatal("no activation hypothesis was generated, so this proves nothing about " +
			"what one is allowed to say")
	}
}

// The selection recorded is the one from BEFORE the change, not the look it was seen on.
//
// # Why this is the subtle half
//
// The look a change is first SEEN on is already showing the new screen. Whatever the watch can
// read on those pixels belongs to the destination, and a recorder reaching one look too far
// forward attaches the destination's answer to the edge that arrives there — confidently,
// durably, and in exactly the field a route would read to decide which row to aim at.
//
// # What the destination's answer actually is, measured
//
// Not "the destination's own row", which was the first guess and is unreachable: the selection
// reader refuses a row it has seen once, so a new list needs several looks before it names
// anything, and for a few looks after a change the belief still reports the OLD list's row. A
// fixture built on the destination having its own mark measures nothing — both looks say entry
// 1, and a recorder reading either one passes.
//
// What the destination's answer IS, on the ordinary case of a screen with nothing shaded on it,
// is NOTHING: the belief goes stale on the very look the divergence is seen, and SelectionAtFrom
// refuses a stale belief. So the test is the other way round from the obvious one — the origin's
// row must be PRESENT, and a recorder one look late records no row at all.
//
// Deleting `selected: before` from the pendingChange in propose must fail this.
func TestTheSelectionRecordedIsTheOneBeforeTheChange(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	// A destination with nothing shaded on it. Not a contrivance — it is what a loading
	// frame, a fade, or a screen of plain text looks like to this path, and the detector
	// reporting zero shaded regions is the commonest thing in the live traces.
	blank := func(s int, intents ...observe.NavIntent) observe.Sample {
		return activationSample(s, []observe.ShadowRegion{{
			Role:   "field",
			Region: observe.Region{X: 0.2, Y: 0.2, Width: 0.5, Height: 0.5},
		}}, intents)
	}
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
	var origin observe.ScreenStateID
	for pass := range 2 {
		// The origin, with the mark parked on entry 1 long enough to be confirmed.
		for range 6 {
			seq++
			totals.Observe(listScreen(seq, 1))
		}
		if totals.Interface.Selected != 1 {
			t.Fatalf("pass %d: the origin reads entry %d, not the 1 it was parked on: %s",
				pass, totals.Interface.Selected, totals.Interface.Because)
		}
		origin = totals.CurrentState
		// THE CHANGE. The first of these looks is the one the divergence is seen on, and
		// the belief is stale on it — which is the whole point: there is no row to be
		// read there, so a recorder reading it records none.
		seq++
		totals.Observe(blank(seq, observe.NavConfirm))
		if got := observe.SelectionAtFrom(totals.Interface); got.Known() {
			t.Fatalf("pass %d: the look the change was seen on still reads %+v. This "+
				"fixture only distinguishes the two looks while that one reads "+
				"nothing, so it would pass either way", pass, got)
		}
		for range 5 {
			seq++
			totals.Observe(blank(seq))
		}
		for range 8 {
			seq++
			totals.Observe(listScreen(seq, 1))
		}
	}

	// THE ROW BELONGS TO THE EDGE LEAVING THE SCREEN IT WAS READ ON, and the direction is
	// the half that has to be asserted.
	//
	// The session walks out of the list and back into it, so there are two edges. A
	// recorder one look late does not lose the row — it moves it onto the RETURN edge,
	// where the belief has just caught up with the list again. Checking only that the row
	// appears somewhere passes either way, which is the trap this comment exists for: the
	// first version of this test did exactly that and the mutant survived it.
	if origin == "" {
		t.Fatal("the fixture never settled on a screen while the list was readable")
	}
	var leaving, returning []observe.SelectedEntry
	var unknown int
	for _, tr := range totals.Transitions {
		unknown += tr.SelectedUnknown
		switch {
		case tr.From == origin:
			leaving = append(leaving, tr.SelectedBefore...)
		case tr.To == origin:
			returning = append(returning, tr.SelectedBefore...)
		}
	}
	var sawOrigin int
	for _, sel := range leaving {
		if sel.Entry == 1 && sel.Entries == 6 {
			sawOrigin += sel.Count
		}
	}
	if sawOrigin == 0 {
		t.Errorf("the change OUT OF %s recorded %v, and entry 1 of its six-entry list is "+
			"not among them — though the mark sat on it for six confirmed looks before "+
			"each change. The look a change is SEEN on is already the new screen, and "+
			"on this fixture nothing is readable there. unknown=%d",
			origin, leaving, unknown)
	}
	for _, sel := range returning {
		t.Errorf("the change INTO %s was recorded with entry %d of %d selected. Its "+
			"origin was the blank screen, where nothing was readable; this row is the "+
			"list's, read a look after the change was seen, and crediting it to the "+
			"edge that arrives at the list points the evidence backwards",
			origin, sel.Entry, sel.Entries)
	}
	t.Logf("leaving %s: %v; returning: %v; unreadable %d",
		origin, leaving, returning, unknown)
}

// A stale belief and an ambiguous one are not evidence about what was selected.
//
// Live, three looks in four bring nothing — an inference starts about every two seconds against
// a session sampling twice a second. "The mark was on entry 3", read four seconds ago, is not
// evidence about the look the screen changed on. Ambiguity is refused for ADR-158's reason:
// when two lists both behave like the one being navigated, which list "entry 3" counts rows of
// is exactly what is not established.
//
// A unit on the one function that decides it, because every path into the segmenter goes
// through it and a rule enforced at its callers is a rule a future caller will not know about.
//
// Deleting either check in SelectionAtFrom must fail this.
func TestAStaleOrUnplacedSelectionIsNotActivationEvidence(t *testing.T) {
	good := observe.InterfaceBelief{
		Status: observe.InterfaceProvisional, Selected: 3, Entries: 9,
	}
	if got := observe.SelectionAtFrom(good); !got.Known() || got.Entry != 3 ||
		got.Entries != 9 {
		t.Fatalf("a fresh, provisional belief with entry 3 of 9 read as %+v, so the "+
			"refusals below prove nothing", got)
	}
	for _, c := range []struct {
		name   string
		belief observe.InterfaceBelief
	}{
		{"stale", observe.InterfaceBelief{
			Status: observe.InterfaceProvisional, Selected: 3, Entries: 9, Stale: true,
		}},
		{"ambiguous", observe.InterfaceBelief{
			Status: observe.InterfaceAmbiguous, Selected: 3, Entries: 9,
		}},
		{"unknown", observe.InterfaceBelief{
			Status: observe.InterfaceUnknown, Selected: 3, Entries: 9,
		}},
		{"nothing read", observe.InterfaceBelief{
			Status: observe.InterfaceProvisional, Selected: -1, Entries: 9,
		}},
		{"no entries", observe.InterfaceBelief{
			Status: observe.InterfaceProvisional, Selected: 0, Entries: 0,
		}},
	} {
		if got := observe.SelectionAtFrom(c.belief); got.Known() {
			t.Errorf("a %s belief produced %+v, which reads as a known selection. A "+
				"change recorded against it would name a row on the strength of "+
				"evidence that does not say one", c.name, got)
		}
	}
}

// A change seen with nothing readable selected is counted, not dropped.
//
// The control evidence. An edge seen ten times with a row selected on two of them is mostly a
// change that happens with nothing pointing anywhere, and a record without this number would
// read as "entry 3, twice" with nothing to weigh it against. It is also what keeps the
// generator's ratio honest: support is measured against the observations that COULD read a
// selection, not against every observation of the edge.
//
// Deleting the `s.unknown++` branch in selectedTally.fold must fail this.
func TestAChangeWithNothingSelectedIsCountedAsSuch(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	// No settling walk at all, so no list is ever identified and no look can read a mark.
	for range 2 {
		for range 6 {
			seq++
			totals.Observe(listScreen(seq, 0))
		}
		for range 6 {
			seq++
			totals.Observe(otherScreen(seq, observe.NavConfirm))
		}
	}
	if totals.Interface.Status == observe.InterfaceProvisional &&
		totals.Interface.Selected >= 0 {
		t.Fatalf("a list was identified after all (entry %d), so this fixture is not "+
			"measuring the unreadable case", totals.Interface.Selected)
	}
	var total, unknown int
	for _, tr := range totals.Transitions {
		total += tr.Count
		unknown += tr.SelectedUnknown
		if len(tr.SelectedBefore) > 0 {
			t.Errorf("%s → %s names a selected entry %v on a session that never "+
				"identified a list", tr.From, tr.To, tr.SelectedBefore)
		}
	}
	if total == 0 {
		t.Fatal("no change was recorded, so there is nothing to have counted")
	}
	if unknown != total {
		t.Errorf("%d of %d observations were recorded as having no readable selection; "+
			"every one of them did, and an uncounted silence is a silence a reader "+
			"cannot weigh the positives against", unknown, total)
	}
	// AND no hypothesis may be generated from it: a change nobody could read a selection
	// before is not evidence about a row.
	for _, h := range observe.Hypotheses(totals, observe.DefaultHypothesisThresholds()) {
		if h.Kind == observe.PossibleEntryActivation {
			t.Errorf("an activation hypothesis was generated with no readable selection "+
				"anywhere in the session: %s", h.Observed)
		}
	}
}

// Moving the mark is never an activation, however much of it happens.
//
// THE INVARIANT, and the mirror of TestNavigationAloneRetainsNoMove. A person walking a
// nine-entry menu from top to bottom and back has activated nothing: the screen never changed,
// and a layer that produced an activation out of a busy highlight would have invented the one
// claim a route acts on.
//
// It is also the failure mode this whole file guards against being introduced LATER. The two
// kinds are generated from different evidence today; the cheap future edit is to notice that a
// selection transition and an activation both name an entry, and fold them.
func TestMovingTheMarkIsNeverAnActivation(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	// One screen, forever. The mark walks the whole list, over and over.
	for range 14 {
		seq++
		totals.Observe(listScreen(seq, 0))
	}
	for range 14 {
		for mark := range 6 {
			for range 3 {
				seq++
				totals.Observe(listScreen(seq, mark, observe.NavDown))
			}
		}
	}

	if len(totals.SelectionTransitions) == 0 {
		t.Fatalf("the mark never registered as moving, so this says nothing about what "+
			"a lot of movement produces. belief=%s selected=%d because=%s",
			totals.Interface.Status, totals.Interface.Selected, totals.Interface.Because)
	}
	if len(totals.Transitions) != 0 {
		t.Fatalf("the screen changed %d time(s) on a fixture that draws one composition, "+
			"so the assertion below would be about something else",
			len(totals.Transitions))
	}
	for _, h := range observe.Hypotheses(totals, observe.DefaultHypothesisThresholds()) {
		if h.Kind == observe.PossibleEntryActivation {
			t.Errorf("a highlight moving produced an activation: %s.\nMoving a mark is "+
				"not opening a menu, and an activation is the claim a route acts on",
				h.Observed)
		}
	}
	t.Logf("%d move(s) retained, %d screen change(s), 0 activations",
		len(totals.SelectionTransitions), len(totals.Transitions))
}
