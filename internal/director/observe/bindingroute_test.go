package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A CANDIDATE BINDING CROSSING THE DURABLE BOUNDARY AND COMING BACK OUT ON A PLAN.
//
// The question is the one that settled ADR-160, asked one layer further on: not whether a
// direction was detected, but whether it SURVIVES — whether a direction a session observed can
// be stored, read back after the session is gone, and used when a plan is built. A direction
// counted in a diagnostic and then dropped has gained nothing toward a route anybody could be
// asked to take.
//
// The chain is the production one, end to end:
//
//	SelectionTransition → SelectionStepsFrom → Fold → Topology → PlanToGoal → Approach
//
// and the links here are the two ADR-160 left empty: the direction in the store, and the
// direction on the plan.

// pressedList is the moves of one session with a direction attributed to each.
//
// `down` on the way down the list and `up` on the way back, which is what a person walking a
// menu actually produces — and which is the arrangement where a layer that generalised "down
// means next" would be caught, because the same pair of rows is reached both ways.
func pressedList(state string, entries int, pairs ...[2]int) []observe.SelectionTransition {
	var out []observe.SelectionTransition
	for _, p := range pairs {
		dir := observe.NavDown
		if p[1] < p[0] {
			dir = observe.NavUp
		}
		out = append(out, observe.SelectionTransition{
			State: observe.ScreenStateID(state), Entries: entries,
			From: p[0], To: p[1], Count: 4, Prompted: 4,
			Directions: []observe.DirectionCount{{Intent: dir, Count: 4}},
		})
	}
	return out
}

// A durable step remembers which direction was pressed.
//
// The survival test for the direction, through the same production chain the selection crossed
// on. A session-local direction is worth nothing: the next session renumbers every state, and a
// route built in a later sitting has only the store to read.
//
// Deleting the Directions lines in SelectionStepsFrom, or the foldDirections call in
// foldSelectionSteps, must fail this.
func TestADurableStepRemembersWhichDirectionWasPressed(t *testing.T) {
	tr := listTransition("state_1", "state_2", 5,
		observe.SelectedEntry{Entry: 3, Entries: 9, Count: 4})
	totals := totalsWith(tr)
	totals.SelectionTransitions = pressedList("state_1", 9,
		[2]int{1, 2}, [2]int{2, 3}, [2]int{3, 2})
	hs := []observe.Hypothesis{
		stateHypothesis("state_1", observe.TermSettings),
		stateHypothesis("state_2", observe.TermAudio),
	}

	rel := foldedEdge(t, totals, hs)

	if len(rel.Moves) != 3 {
		t.Fatalf("the durable edge kept %d move(s), want the 3 observed: %v",
			len(rel.Moves), rel.Moves)
	}
	for _, m := range rel.Moves {
		best, ok := observe.DominantDirection(m.Directions)
		if !ok {
			t.Fatalf("a durable move names no direction, so a route built from it can "+
				"say which row to reach and nothing about reaching it: %s", m)
		}
		want := observe.NavDown
		if m.To < m.From {
			want = observe.NavUp
		}
		if best.Intent != want {
			t.Errorf("%s names %q; the session observed %q before that move",
				m, best.Intent, want)
		}
		if !m.Support().Usable() {
			t.Errorf("%s: four agreeing observations is above MinBindingAgreement and "+
				"the support reads %q", m, m.Support())
		}
	}
	// AND THE RECORD DOES NOT GENERALISE. Entry 2→3 was reached by down and 3→2 by up, so
	// a layer that had learned "down means next" would name down on both.
	var up, dn int
	for _, m := range rel.Moves {
		best, _ := observe.DominantDirection(m.Directions)
		switch best.Intent {
		case observe.NavUp:
			up++
		case observe.NavDown:
			dn++
		}
	}
	if up != 1 || dn != 2 {
		t.Errorf("the durable edge names %d up and %d down move(s), want 1 and 2. The "+
			"same two rows were travelled in both directions, and a record that gave "+
			"them one direction has learned a rule about lists it never observed", up, dn)
	}

	// A SECOND SESSION ADDS TO IT rather than replacing it, which is what turns a thin
	// candidate into a repeated one over time instead of resetting every restart.
	before, _ := observe.DominantDirection(rel.Moves[0].Directions)
	rel.Fold(rel.Evidence())
	after, _ := observe.DominantDirection(rel.Moves[0].Directions)
	if after.Count != before.Count*2 {
		t.Errorf("after a second session the leading direction has %d observation(s), "+
			"want %d: a durable record that overwrote would forget the first sitting",
			after.Count, before.Count*2)
	}
	t.Logf("durable edge: %s", strings.Join(
		observe.DescribeRelationship(rel, observe.RememberedSubject{ID: rel.From},
			observe.RememberedSubject{ID: rel.To}), "\n  "))
}

// A confirm can never become the direction a mark moved by.
//
// NavIntent's underlying type is a string, so anything upstream could put one in. A `confirm`
// in this field would be a join crediting a screen change to a move of the mark — which is
// exactly the distinction ADR-160 exists to keep — so the durable merge checks the vocabulary
// rather than trusting whoever filled the slice.
func TestOnlyADirectionCanBeTheDirectionOfAMove(t *testing.T) {
	got, _ := observe.FoldDirectionsForTest(nil, []observe.DirectionCount{
		{Intent: observe.NavConfirm, Count: 5},
		{Intent: observe.NavBack, Count: 2},
		{Intent: observe.NavPause, Count: 2},
		{Intent: observe.NavPoint, Count: 2},
		{Intent: "sideways", Count: 9},
		{Intent: observe.NavLeft, Count: 3},
	})
	if len(got) != 1 || got[0].Intent != observe.NavLeft {
		t.Fatalf("the durable merge admitted %v; only `left` out of that list can move a "+
			"mark within a list", got)
	}
	// And the whole directional vocabulary does get through, so this is a filter and not
	// a whitelist of one.
	all, _ := observe.FoldDirectionsForTest(nil, []observe.DirectionCount{
		{Intent: observe.NavUp, Count: 1}, {Intent: observe.NavDown, Count: 1},
		{Intent: observe.NavLeft, Count: 1}, {Intent: observe.NavRight, Count: 1},
	})
	if len(all) != 4 {
		t.Errorf("the merge kept %d of the four directions: %v", len(all), all)
	}

	// AND A FIRST SIGHTING IS CHECKED TOO, which is the half that is easy to miss: a new
	// move is appended rather than merged, so a step that arrived with a `confirm` on it
	// would be stored whole. Through RememberedRelationship.Fold, which is the production
	// write.
	var rel observe.RememberedRelationship
	rel.Fold(observe.RelationshipEvidence{
		Observations: 3,
		Moves: []observe.SelectionStep{{
			From: 1, To: 2, Entries: 6, Observations: 3, Prompted: 3,
			Directions: []observe.DirectionCount{
				{Intent: observe.NavConfirm, Count: 3},
				{Intent: observe.NavDown, Count: 3},
			},
		}},
	})
	if len(rel.Moves) != 1 {
		t.Fatalf("the durable fold kept %d move(s), want 1: %v", len(rel.Moves), rel.Moves)
	}
	if len(rel.Moves[0].Directions) != 1 ||
		rel.Moves[0].Directions[0].Intent != observe.NavDown {
		t.Errorf("a first sighting carried %v into the store; `confirm` changes which "+
			"screen is in front rather than which row is marked, so one here would "+
			"credit a screen change to a move of the mark", rel.Moves[0].Directions)
	}
}

// A planned route says which of its observed moves have a direction.
//
// THE RETRIEVAL, at the one place a route is constructed, and the gap it reports is the point:
// `6 observed moves, 2 of them with a known direction` is a list Marco has watched and largely
// cannot name the controls of, and a bare move count reads exactly the same as six it could.
//
// Deleting the Bound loop in ApproachOf must fail this.
func TestAPlannedRouteSaysWhichMovesHaveADirection(t *testing.T) {
	top := observe.Topology{
		Subjects: map[string]observe.RememberedSubject{
			"home": {ID: "home"}, "menu": {ID: "menu"}, "play": {ID: "play"},
		},
		Relationships: []observe.RememberedRelationship{{
			From: "home", To: "menu", Application: "game", Observations: 3,
			SelectedBefore: []observe.SelectedEntry{{Entry: 0, Entries: 4, Count: 3}},
		}, {
			From: "menu", To: "play", Application: "game", Observations: 5,
			SelectedBefore: []observe.SelectedEntry{{Entry: 3, Entries: 9, Count: 4}},
			Moves: []observe.SelectionStep{
				// Named: four agreeing observations.
				{From: 1, To: 2, Entries: 9, Observations: 4, Prompted: 4,
					Directions: []observe.DirectionCount{
						{Intent: observe.NavDown, Count: 4},
					}},
				// Named: three is the floor.
				{From: 2, To: 3, Entries: 9, Observations: 3, Prompted: 3,
					Directions: []observe.DirectionCount{
						{Intent: observe.NavDown, Count: 3},
					}},
				// Not named: the evidence disagrees.
				{From: 3, To: 4, Entries: 9, Observations: 6, Prompted: 6,
					Directions: []observe.DirectionCount{
						{Intent: observe.NavDown, Count: 3},
						{Intent: observe.NavRight, Count: 3},
					}},
				// Not named: nothing was ever attributable.
				{From: 4, To: 5, Entries: 9, Observations: 9},
			},
		}},
	}

	plan := observe.PlanToGoal("play", "home", top, nil)
	if plan.Refusal != "" || len(plan.Approaches) != 2 {
		t.Fatalf("no two-step route was planned: %d approach(es), refusal %q",
			len(plan.Approaches), plan.Refusal)
	}
	menu := plan.Approaches[1]
	if len(menu.Moves) != 4 {
		t.Fatalf("the menu step carries %d move(s), want 4", len(menu.Moves))
	}
	if menu.Bound != 2 {
		t.Errorf("the plan says %d of 4 moves have a direction, want 2: one is "+
			"contradicted and one was never attributed, and counting either would "+
			"claim a control nothing established", menu.Bound)
	}

	// THE CHAIN, with the direction on every hop — which is what a route would be asked
	// to take, and the first time this subsystem can produce one.
	chain, refusal := menu.Reach(observe.SelectionAt{Entry: 1, Entries: 9})
	if refusal != "" {
		t.Fatalf("no chain from entry 1 to entry 3 (%q)", refusal)
	}
	ok, weakest := observe.Performable(chain)
	if !ok {
		t.Fatalf("the chain 1->2->3 has a direction on both hops and reports a gap: %q",
			weakest)
	}
	var words []string
	for _, hop := range chain {
		best, got := observe.DominantDirection(hop.Directions)
		if !got {
			t.Fatalf("hop %s has no direction, so Performable is answering about "+
				"something else", hop)
		}
		words = append(words, string(best.Intent))
	}
	if strings.Join(words, ",") != "down,down" {
		t.Errorf("the chain reads %q, want down,down", strings.Join(words, ","))
	}

	// AND A CHAIN THROUGH A CONTRADICTED HOP IS REPORTED AS A GAP rather than taken. The
	// route to entry 4 exists in the observations and nothing can say how to walk it.
	toFour := menu
	toFour.Entry = 4
	through, refusal := toFour.Reach(observe.SelectionAt{Entry: 1, Entries: 9})
	if refusal != "" {
		t.Fatalf("no chain from entry 1 to entry 4 (%q), though every hop was "+
			"observed", refusal)
	}
	if ok, weakest := observe.Performable(through); ok {
		t.Error("a chain whose last hop followed two different directions is reported " +
			"as performable")
	} else if weakest != observe.BindingContradicted {
		t.Errorf("the gap in that chain reads %q, want contradicted", weakest)
	}
	t.Logf("%s\n  chain: %v", menu.Describe(), words)
	for _, line := range menu.Bindings() {
		t.Logf("  %s", line)
	}
}

// A move hypothesis names the direction and still refuses to claim the effect.
//
// The sentence this layer must not produce is "pressing down moves the mark from entry 1 to
// entry 2". What it may produce is "down was the only direction observed before that move, four
// times out of four, and nothing has pressed it on purpose to find out".
//
// Deleting the DominantDirection evidence in selectionTransitions must fail this.
func TestAMoveHypothesisNamesTheDirectionAndNotTheEffect(t *testing.T) {
	totals := walkWithDirections(t, 4, observe.NavDown)

	var found bool
	for _, h := range observe.Hypotheses(totals, observe.DefaultHypothesisThresholds()) {
		if h.Kind != observe.PossibleSelectionTransition {
			continue
		}
		found = true
		var said bool
		for _, e := range h.Support {
			if strings.Contains(e.Statement, "down was the only direction observed") {
				said = true
			}
		}
		if !said {
			var had []string
			for _, e := range h.Support {
				had = append(had, e.Statement)
			}
			t.Errorf("no support names the direction:\n  %s", strings.Join(had, "\n  "))
		}
		// THE WORDS THAT WOULD BE A CAUSAL CLAIM. "Observed before" is a correlation;
		// "moves", "causes" and "will" are not, and nothing in this layer earns them.
		for _, bad := range []string{"pressing down moves", "down moves the mark",
			"will move", "causes the mark"} {
			if strings.Contains(strings.ToLower(h.Observed+" "+h.Validation), bad) {
				t.Errorf("the hypothesis claims an effect it never tested (%q):\n  %s",
					bad, h.Observed)
			}
		}
		// And the validation is the experiment that WOULD earn it, naming the key.
		if !strings.Contains(h.Validation, "press down") {
			t.Errorf("the validation does not say what to press, which is the whole "+
				"gain of naming the direction: %s", h.Validation)
		}
		if !strings.Contains(h.Validation, "nothing has pressed it on purpose") {
			t.Errorf("the validation does not say the effect is untested: %s",
				h.Validation)
		}
	}
	if !found {
		t.Fatal("no selection-transition hypothesis was generated, so this says nothing")
	}
}

// Evidence that disagrees is reported as disagreeing.
//
// The case a bare majority would hide. A move seen following down three times and right three
// times is real evidence AGAINST either of them being what moved the mark, and a layer that
// named the first one would be inventing a control out of a coin toss.
//
// Deleting the len(st.Directions) > 1 contradiction must fail this.
func TestAContradictedDirectionIsReportedAsContradicted(t *testing.T) {
	var totals observe.ShadowTotals
	totals.SelectionTransitions = []observe.SelectionTransition{{
		State: "state_1", Entries: 9, From: 1, To: 2, Count: 8, Prompted: 6,
		Unprompted: 2,
		Directions: []observe.DirectionCount{
			{Intent: observe.NavDown, Count: 3},
			{Intent: observe.NavRight, Count: 3},
		},
	}}
	if got := totals.SelectionTransitions[0].Support(); got != observe.BindingContradicted {
		t.Fatalf("an even split reads as %q, so the hypothesis below is about something "+
			"else", got)
	}

	var found bool
	for _, h := range observe.Hypotheses(totals, observe.DefaultHypothesisThresholds()) {
		if h.Kind != observe.PossibleSelectionTransition {
			continue
		}
		found = true
		var said bool
		for _, e := range h.Contradictions {
			if strings.Contains(e.Statement, "more than one direction") {
				said = true
			}
		}
		if !said {
			var had []string
			for _, e := range h.Contradictions {
				had = append(had, e.Statement)
			}
			t.Errorf("the disagreement is not reported as a contradiction:\n  %s",
				strings.Join(had, "\n  "))
		}
		// And the validation refuses to name a key, because there is no key to name.
		if strings.Contains(h.Validation, "press down") ||
			strings.Contains(h.Validation, "press right") {
			t.Errorf("the validation picked a side out of contradicted evidence: %s",
				h.Validation)
		}
		if !strings.Contains(h.Validation, string(observe.BindingContradicted)) {
			t.Errorf("the validation does not say why it cannot name a direction: %s",
				h.Validation)
		}
	}
	if !found {
		t.Fatal("no selection-transition hypothesis was generated, so this says nothing")
	}
}

// A direction learned in one list is never applied to another.
//
// The generalisation this layer is not allowed to make, in the form it would actually take. A
// six-row list walked with down says nothing about a nine-row list: it may wrap, may skip
// disabled rows, may scroll under the mark, may be laid out left to right so that down does
// nothing in it at all. A list that merely GAINED a row has renumbered everything below it, so
// even "the same list" is not the same list.
//
// Asserted at the one place a chain is built, because that is where a borrowed direction would
// be used rather than merely stored.
func TestADirectionLearnedInOneListIsNotAppliedToAnother(t *testing.T) {
	a := observe.ApproachOf(observe.RememberedRelationship{
		From: "menu", To: "play",
		SelectedBefore: []observe.SelectedEntry{{Entry: 3, Entries: 6, Count: 4}},
		Moves: []observe.SelectionStep{
			{From: 1, To: 2, Entries: 6, Observations: 4, Prompted: 4,
				Directions: []observe.DirectionCount{
					{Intent: observe.NavDown, Count: 4},
				}},
			{From: 2, To: 3, Entries: 6, Observations: 4, Prompted: 4,
				Directions: []observe.DirectionCount{
					{Intent: observe.NavDown, Count: 4},
				}},
			// The SAME rows in a nine-entry list, pressed the other way. Real
			// evidence about a different arrangement, and the trap: a chain that
			// mixed the two would read as four agreeing observations of down.
			{From: 1, To: 2, Entries: 9, Observations: 4, Prompted: 4,
				Directions: []observe.DirectionCount{
					{Intent: observe.NavRight, Count: 4},
				}},
			{From: 2, To: 3, Entries: 9, Observations: 4, Prompted: 4,
				Directions: []observe.DirectionCount{
					{Intent: observe.NavRight, Count: 4},
				}},
		},
	})

	// The six-row list is the one this edge was observed in, so it is the one that plans.
	chain, refusal := a.Reach(observe.SelectionAt{Entry: 1, Entries: 6})
	if refusal != "" {
		t.Fatalf("no chain in the six-entry list (%q)", refusal)
	}
	for _, hop := range chain {
		if hop.Entries != 6 {
			t.Fatalf("the chain borrowed a hop from a %d-entry list: %s",
				hop.Entries, hop)
		}
		best, ok := observe.DominantDirection(hop.Directions)
		if !ok || best.Intent != observe.NavDown {
			t.Errorf("hop %s names %q; the six-entry list was only ever walked with "+
				"down", hop, best.Intent)
		}
	}
	// And standing in the nine-entry list refuses outright rather than pressing down
	// because down worked somewhere else.
	if _, refusal := a.Reach(observe.SelectionAt{Entry: 1, Entries: 9}); refusal !=
		observe.ApproachDifferentList {
		t.Errorf("standing in a nine-entry list gave refusal %q, want %q. \"Entry 3\" "+
			"is not the same row in a list with a different number of entries",
			refusal, observe.ApproachDifferentList)
	}
}

// Activating an entry never borrows a move's direction.
//
// ADR-160's distinction, held across this milestone. A screen change observed with entry 3
// marked is not a move of the mark, and `confirm` is not a direction — so an activation must
// never acquire one, and a move's `down` must never be read as what opened the screen.
//
// The structural half is that ScreenTransition has no direction field at all. This asserts the
// behavioural half: the two hypotheses the same session produces say different things.
func TestAnActivationNeverBorrowsAMovesDirection(t *testing.T) {
	totals := activationRun(t, 3)
	totals.SelectionTransitions = pressedList("state_1", 6, [2]int{1, 2}, [2]int{2, 3})

	var moves, activations int
	for _, h := range observe.Hypotheses(totals, observe.DefaultHypothesisThresholds()) {
		switch h.Kind {
		case observe.PossibleSelectionTransition:
			moves++
			if !strings.Contains(h.Validation, "press down") {
				t.Errorf("a move hypothesis does not name the direction it was "+
					"observed following: %s", h.Validation)
			}
		case observe.PossibleEntryActivation:
			activations++
			said := h.Observed + " " + h.Validation
			for _, e := range h.Support {
				said += " " + e.Statement
			}
			for _, bad := range []string{"down", "up", "left", "right"} {
				if strings.Contains(said, " "+bad+" ") {
					t.Errorf("an activation hypothesis names the direction %q, "+
						"which belongs to a move of the mark and not to a "+
						"screen change:\n  %s", bad, said)
				}
			}
		}
	}
	if moves == 0 || activations == 0 {
		t.Fatalf("%d move and %d activation hypothes(es) — this test needs one of each "+
			"from one session to say anything", moves, activations)
	}
}

// COMPETING LISTS are held one layer down, and deliberately not re-tested here.
//
// A fixture was built for it and could not reach its own case: two six-row lists marked in
// lockstep, pressed at throughout, and the disambiguator identified one of them anyway (entry 5
// of 6, "unlike its siblings, and away from its own unselected level"). A test that skips
// asserts nothing, so it was removed rather than left looking like coverage.
//
// The claim is still held, and by construction rather than by a fixture. A direction is folded
// only onto a move, and `InterfaceWatch.noteMove` drops its held sighting on any look whose
// belief is not provisional — so while two lists both behave like the one being navigated, no
// move is emitted at all and there is nothing for a direction to attach to.
// TestADirectionIsNeverInventedWithoutAMove above is that same path, entered through a screen
// with no list on it, which is the arrangement that can actually be built.
//
// What IS tested here is the harder half: evidence from two lists that were BOTH identified, at
// different times, must not be mixed when a chain is built. That is
// TestADirectionLearnedInOneListIsNotAppliedToAnother.
