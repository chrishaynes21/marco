package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A LEARNED RELATIONSHIP BEING INSPECTED AND USED WHEN CONSTRUCTING A ROUTE.
//
// The question these answer is not "was the evidence detected" — ADR-159's tests already settle
// that — but whether it SURVIVES: whether a selection a session observed can be stored, read
// back after the session is gone, and used when a plan is built. Evidence that is counted in a
// diagnostic and then dropped has gained nothing toward a reusable macro.
//
// So the chain here is the production one, end to end and in order:
//
//	ScreenTransition  →  RelationshipsFrom  →  Fold  →  Topology  →  PlanToGoal  →  Approach
//
// and the last link is the one that matters: a route that can say which of nine rows each step
// was taken from, and what it has seen the mark do inside that list.

// listTransition is one edge of a session, already carrying its selection evidence.
func listTransition(from, to string, count int, sel ...observe.SelectedEntry) observe.ScreenTransition {
	return observe.ScreenTransition{
		From: observe.ScreenStateID(from), To: observe.ScreenStateID(to),
		Count: count, SelectedBefore: sel,
	}
}

// walkedList is the moves of one session, as the ledger publishes them.
func walkedList(state string, entries int, pairs ...[2]int) []observe.SelectionTransition {
	var out []observe.SelectionTransition
	for _, p := range pairs {
		out = append(out, observe.SelectionTransition{
			State: observe.ScreenStateID(state), Entries: entries,
			From: p[0], To: p[1], Count: 3, Prompted: 3,
		})
	}
	return out
}

// foldedEdge runs one session's evidence through the durable path and returns the stored record.
//
// THE PRODUCTION CHAIN, not a hand-built RememberedRelationship. The whole claim being tested is
// that evidence survives the boundary, and a fixture that constructed the far side itself would
// be asserting that the near side is irrelevant.
func foldedEdge(t *testing.T, totals observe.ShadowTotals,
	hs []observe.Hypothesis) observe.RememberedRelationship {

	t.Helper()
	m := &termMemory{}
	obs, report := observe.RelationshipsFrom(totals, hs, "game", m, observe.Continuity{})
	if len(obs) != 1 {
		t.Fatalf("%d durable observation(s), want 1: %s", len(obs), report.Why())
	}
	var rel observe.RememberedRelationship
	rel.From, rel.To, rel.Application = obs[0].From, obs[0].To, "game"
	rel.Fold(obs[0].Evidence)
	return rel
}

// A durable edge remembers which entry was selected, and what the mark was seen doing.
//
// The survival test. A session's selection evidence has to cross the durable boundary intact —
// the entry, the count of entries, how often, how often nothing was readable, and the moves
// inside that list — because the next session renumbers every state and nothing session-local
// means anything by then.
//
// Deleting the SelectedBefore/Moves lines in RelationshipsFrom, or the fold in
// RememberedRelationship.Fold, must fail this.
func TestADurableEdgeRemembersWhatWasSelected(t *testing.T) {
	tr := listTransition("state_1", "state_2", 5,
		observe.SelectedEntry{Entry: 3, Entries: 9, Count: 4},
		observe.SelectedEntry{Entry: 7, Entries: 9, Count: 1})
	tr.SelectedUnknown = 2
	totals := totalsWith(tr)
	totals.SelectionTransitions = walkedList("state_1", 9, [2]int{1, 2}, [2]int{2, 3})
	hs := []observe.Hypothesis{
		stateHypothesis("state_1", observe.TermSettings),
		stateHypothesis("state_2", observe.TermAudio),
	}

	rel := foldedEdge(t, totals, hs)

	if len(rel.SelectedBefore) != 2 {
		t.Fatalf("the durable edge kept %d selection(s), want the 2 the session saw: %v",
			len(rel.SelectedBefore), rel.SelectedBefore)
	}
	best, ok := observe.DominantSelection(rel.SelectedBefore)
	if !ok || best.Entry != 3 || best.Entries != 9 || best.Count != 4 {
		t.Errorf("the dominant selection came back as %+v (ok=%v); the session saw entry "+
			"3 of 9 on four of five observations", best, ok)
	}
	if rel.SelectedUnknown != 2 {
		t.Errorf("the durable edge says %d observation(s) had nothing readable selected, "+
			"want 2. Without it, 'entry 3, four times' has nothing to be weighed against",
			rel.SelectedUnknown)
	}
	if len(rel.Moves) != 2 {
		t.Fatalf("the durable edge kept %d move(s) of the mark, want the 2 observed in "+
			"that list: %v", len(rel.Moves), rel.Moves)
	}
	// AND A SECOND SESSION ADDS TO IT rather than replacing it, which is what makes this
	// accumulation rather than a snapshot of whichever session ran last.
	rel.Fold(rel.Evidence())
	if best, _ := observe.DominantSelection(rel.SelectedBefore); best.Count != 8 {
		t.Errorf("after a second session the dominant selection has %d observation(s), "+
			"want 8: a durable record that overwrote would forget the first sitting",
			best.Count)
	}
	if rel.Moves[0].Observations != 6 {
		t.Errorf("after a second session a move has %d observation(s), want 6",
			rel.Moves[0].Observations)
	}
	t.Logf("durable edge: %s", strings.Join(
		observe.DescribeRelationship(rel, observe.RememberedSubject{ID: rel.From},
			observe.RememberedSubject{ID: rel.To}), "\n  "))
}

// A move the session could not place is not evidence about an edge.
//
// A move on an unknown screen is real evidence about a LIST and says nothing about which screen
// a route would start from. Attaching it to an edge would guess exactly that — and the guess
// would be durable, which is the one place [[ADR-018]]'s "both ends or nothing" rule is most
// tempting to bend.
//
// Deleting the state check in SelectionStepsFrom must fail this.
func TestAnUnplacedMoveIsNotEvidenceAboutAnEdge(t *testing.T) {
	placed := walkedList("state_1", 9, [2]int{1, 2})
	unplaced := walkedList("", 9, [2]int{4, 5})
	elsewhere := walkedList("state_2", 9, [2]int{7, 8})

	got := observe.SelectionStepsFrom(
		append(append(placed, unplaced...), elsewhere...), "state_1")
	if len(got) != 1 {
		t.Fatalf("%d step(s) were attached to state_1, want only the 1 observed there: %v",
			len(got), got)
	}
	if got[0].From != 1 || got[0].To != 2 {
		t.Errorf("the attached step is %s, want entry 1->2", got[0])
	}
	// AND asking about the unplaced screen attaches nothing at all, rather than everything
	// that happened to be unplaced.
	if out := observe.SelectionStepsFrom(unplaced, ""); len(out) != 0 {
		t.Errorf("asking about an unplaced screen returned %d step(s): %v. An unknown "+
			"screen is not a screen two moves can be said to have happened on the same "+
			"one of", len(out), out)
	}
}

// A planned route carries what each of its steps was observed being taken from.
//
// # Why this is the test the whole arc was for
//
// "If Director detects a selection transfer but the graph cannot retain, distinguish, and later
// retrieve it, we haven't gained much toward reusable macros." This is the retrieval, at the one
// place a route is constructed: a plan whose steps name two screens each and nothing else cannot
// be taught as "open the third entry", however many selections were counted in a diagnostic.
//
// Deleting the ApproachesFor call in PlanToGoal must fail this.
func TestAPlannedRouteCarriesWhatEachStepWasSelectedFrom(t *testing.T) {
	top := observe.Topology{
		Subjects: map[string]observe.RememberedSubject{
			"home": {ID: "home"}, "menu": {ID: "menu"}, "play": {ID: "play"},
		},
		Relationships: []observe.RememberedRelationship{{
			From: "home", To: "menu", Application: "game", Observations: 3,
			SelectedBefore: []observe.SelectedEntry{{Entry: 0, Entries: 4, Count: 3}},
		}, {
			From: "menu", To: "play", Application: "game", Observations: 5,
			SelectedBefore:  []observe.SelectedEntry{{Entry: 3, Entries: 9, Count: 4}},
			SelectedUnknown: 1,
			Moves: []observe.SelectionStep{
				{From: 1, To: 2, Entries: 9, Observations: 6, Prompted: 5},
				{From: 2, To: 3, Entries: 9, Observations: 4, Prompted: 4},
			},
		}},
	}

	plan := observe.PlanToGoal("play", "home", top, nil)
	if plan.Refusal != "" || len(plan.Steps) != 2 {
		t.Fatalf("no two-step route was planned: %d step(s), refusal %q",
			len(plan.Steps), plan.Refusal)
	}
	if len(plan.Approaches) != len(plan.Steps) {
		t.Fatalf("the plan has %d step(s) and %d approach(es). They are read in step order, "+
			"so a caller walking the route would attach the wrong row to a step",
			len(plan.Steps), len(plan.Approaches))
	}
	last := plan.Approaches[1]
	if last.Step != plan.Steps[1] {
		t.Fatalf("approach 2 describes %+v and step 2 is %+v", last.Step, plan.Steps[1])
	}
	if !last.Known() {
		t.Fatalf("the menu step has no entry known for it (%q); the edge was observed "+
			"with entry 3 of 9 selected on four of five observations", last.Refusal)
	}
	if last.Entry != 3 || last.Entries != 9 {
		t.Errorf("the menu step reads entry %d of %d, want 3 of 9",
			last.Entry, last.Entries)
	}
	if last.Support != 4 || last.Of != 4 {
		t.Errorf("the menu step reads %d of %d readable observation(s), want 4 of 4; "+
			"'4 of 4' and '4 of 12' are different claims", last.Support, last.Of)
	}
	if last.Unknown != 1 {
		t.Errorf("the menu step reports %d observation(s) with nothing readable "+
			"selected, want 1", last.Unknown)
	}
	t.Logf("step 1: %s\nstep 2: %s",
		plan.Approaches[0].Describe(), plan.Approaches[1].Describe())
}

// An approach chains the moves that were observed, and only those.
//
// The second half of the retrieval: knowing which row a step goes from is worth nothing without
// something to say about getting the mark there. The chain is assembled from the moves the
// session SAW, in the order that reaches the row.
func TestAnApproachChainsTheMovesThatWereObserved(t *testing.T) {
	a := observe.ApproachOf(observe.RememberedRelationship{
		From: "menu", To: "play",
		SelectedBefore: []observe.SelectedEntry{{Entry: 3, Entries: 9, Count: 4}},
		Moves: []observe.SelectionStep{
			{From: 1, To: 2, Entries: 9, Observations: 6, Prompted: 5},
			{From: 2, To: 3, Entries: 9, Observations: 4, Prompted: 4},
			{From: 5, To: 6, Entries: 9, Observations: 2, Prompted: 2},
		},
	})

	steps, refusal := a.Reach(observe.SelectionAt{Entry: 1, Entries: 9})
	if refusal != "" {
		t.Fatalf("no chain from entry 1 to entry 3 (%q), though 1->2 and 2->3 were both "+
			"observed", refusal)
	}
	if len(steps) != 2 || steps[0].From != 1 || steps[0].To != 2 ||
		steps[1].From != 2 || steps[1].To != 3 {
		t.Fatalf("the chain is %v, want entry 1->2 then 2->3", steps)
	}
	// ALREADY THERE is an empty chain with no refusal, which is the one unambiguous way to
	// say nothing needs doing.
	if steps, refusal := a.Reach(observe.SelectionAt{Entry: 3, Entries: 9}); refusal != "" ||
		len(steps) != 0 {
		t.Errorf("standing on the row already gave %d step(s) and refusal %q, want an "+
			"empty chain and no refusal", len(steps), refusal)
	}
	t.Logf("entry 1 → entry 3: %v", steps)
}

// An approach will not invent a move it never saw.
//
// THE INVARIANT, and the reason the chain is a search rather than arithmetic. "Entry 1 to entry
// 3 is two moves down" is a rule about how lists work, and this subsystem is not allowed to
// have one: a list may wrap, may skip disabled rows, may scroll under the mark. What is known
// is what was seen, and a gap in it is reported as a gap.
//
// Deleting the `!seen[a.Entry]` refusal in Reach must fail this.
func TestAnApproachWillNotInventAMoveItNeverSaw(t *testing.T) {
	a := observe.ApproachOf(observe.RememberedRelationship{
		From: "menu", To: "play",
		SelectedBefore: []observe.SelectedEntry{{Entry: 3, Entries: 9, Count: 4}},
		// 1->2 was seen. 2->3 was not, so entry 3 has never been arrived at.
		Moves: []observe.SelectionStep{
			{From: 1, To: 2, Entries: 9, Observations: 6, Prompted: 5},
		},
	})

	steps, refusal := a.Reach(observe.SelectionAt{Entry: 1, Entries: 9})
	if refusal != observe.ApproachNoKnownMoves {
		t.Errorf("a chain to entry 3 was produced from one observed move 1->2: %v "+
			"(refusal %q). Nothing has ever been seen arriving at entry 3, and a route "+
			"that assumed another press would get there is arithmetic about lists in "+
			"general", steps, refusal)
	}
	// And the refusal is a sentence a person hears, not a bare token.
	if say := refusal.Say(); !strings.Contains(say, "never") {
		t.Errorf("the refusal reads %q, which does not tell anybody what is missing", say)
	}
}

// An approach refuses a list that has renumbered itself.
//
// "Entry 3" means nothing except against a stable count of entries, and a list that gained a
// row has renumbered everything below it. The same guard noteMove applies within a session,
// applied across the durable boundary — where it matters more, because the evidence on the far
// side was gathered a different day.
//
// Deleting the `now.Entries != a.Entries` check in Reach must fail this.
func TestAnApproachRefusesAListThatRenumbered(t *testing.T) {
	a := observe.ApproachOf(observe.RememberedRelationship{
		From: "menu", To: "play",
		SelectedBefore: []observe.SelectedEntry{{Entry: 3, Entries: 9, Count: 4}},
		Moves: []observe.SelectionStep{
			{From: 1, To: 2, Entries: 9, Observations: 6},
			{From: 2, To: 3, Entries: 9, Observations: 4},
		},
	})

	if steps, refusal := a.Reach(observe.SelectionAt{Entry: 1, Entries: 10}); refusal != observe.ApproachDifferentList {
		t.Errorf("a chain was produced into a ten-entry list from evidence about a "+
			"nine-entry one: %v (refusal %q). The rows below the added one have all "+
			"moved, so entry 3 is not the same place", steps, refusal)
	}
	// AND nothing selected at all is its own refusal, kept apart from the above: one says
	// "the list is different", the other "I can't see where the mark is", and they call for
	// different responses.
	if _, refusal := a.Reach(observe.SelectionAt{}); refusal != observe.ApproachPositionUnknown {
		t.Errorf("an unreadable selection refused with %q, want %q",
			refusal, observe.ApproachPositionUnknown)
	}
}

// An edge nobody saw a selection before is refused, and that is not a failure.
//
// Most screens do not change out of a list at all. An approach that treated "no row known" as a
// problem would report a defect on every ordinary transition in the store, which is how a
// diagnostic becomes something people stop reading.
func TestAnEdgeWithNoSelectionEvidenceSaysSoPlainly(t *testing.T) {
	a := observe.ApproachOf(observe.RememberedRelationship{
		From: "home", To: "menu", Observations: 9,
	})
	if a.Known() {
		t.Fatalf("an edge with no selection evidence reads as known: %+v", a)
	}
	if a.Refusal != observe.ApproachUnknownEntry {
		t.Errorf("the refusal is %q, want %q", a.Refusal, observe.ApproachUnknownEntry)
	}
	if _, refusal := a.Reach(observe.SelectionAt{Entry: 1, Entries: 9}); refusal !=
		observe.ApproachUnknownEntry {
		t.Errorf("reaching on an edge with no row carried refusal %q; it must carry the "+
			"edge's own refusal rather than inventing a second one", refusal)
	}
	if d := a.Describe(); !strings.Contains(d, "never seen") {
		t.Errorf("the description reads %q, which does not say what is unknown", d)
	}
}
