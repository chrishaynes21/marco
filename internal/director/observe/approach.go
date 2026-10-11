package observe

import (
	"fmt"
	"sort"
)

// Using what was learned about a menu when constructing a route.
//
// # The question this file answers
//
// A remembered edge says "screen A was observed becoming screen B". A route made of those alone
// is a chain of destinations with nothing to say about how any of them is entered — which is
// fine for a screen that changes on one keypress and useless for one of nine rows in a list.
//
// [[ADR-160-activating-an-entry-is-not-moving-the-mark]] gave the edge two more things: the row
// the origin was pointing at when the change was seen, and the moves the mark was observed
// making inside that list. This turns those into the shape a route needs:
//
//	step A→B needs entry 3 of 9 selected          (what was observed beside the change)
//	the mark is on entry 1 right now              (what the session can see)
//	observed moves 1→2 and 2→3 reach it            (what was observed inside the list)
//
// That is the whole point of the durable record: not a counter in a diagnostic, but something a
// plan can be asked about and answer from.
//
// # What an approach is NOT
//
//   - It is not authority. Every edge still earns execution through its own rehearsal and its
//     own yes, and an approach is a reading of remembered observation exactly like the plan it
//     hangs off. `PlanToGoal`'s note applies here unchanged.
//   - It is not executable, and the reason has MOVED rather than gone away. A SelectionStep now
//     names the directions that were OBSERVED before each move — see binding.go, which joins
//     them after the pixels and never before — so a chain can say "down, then down". What no
//     part of this establishes is that pressing down from entry 1 ARRIVES at entry 2: that
//     needs the press performed deliberately and the outcome checked, which is a rehearsal.
//     `Performable` reports whether a chain has a direction for every hop, which is the absence
//     of a known gap and not the presence of a verification.
//   - It is not a requirement. `SelectedBefore` is a correlation: the row was marked while the
//     screen changed. An approach that read it as a precondition would be claiming the change
//     needs that row, which nothing observed establishes.

// ApproachRefusal is the CLOSED vocabulary of why a step's selection cannot be approached.
//
// Five, and they are genuinely different situations that call for different responses — which
// is the whole argument SessionLocalCause makes one layer down. "No approach" as a single
// silence sent every one of them to the same place.
type ApproachRefusal string

const (
	// ApproachUnknownEntry — the edge was never observed with a readable selection on its
	// origin, so nothing is known about which row it goes from. Not a failure of the route:
	// a screen that changes on one keypress has no row to name.
	ApproachUnknownEntry ApproachRefusal = "entry_unknown"
	// ApproachPositionUnknown — nothing readable is selected right now, so there is nothing
	// to move the mark FROM. Honest refusal, never a guess at where it probably is.
	ApproachPositionUnknown ApproachRefusal = "selection_unknown"
	// ApproachDifferentList — something is selected, in a list with a different number of
	// entries from the one this edge was observed with. A list that gained a row has
	// renumbered everything below it, so "entry 3" is not the same place.
	ApproachDifferentList ApproachRefusal = "different_list"
	// ApproachNoKnownMoves — the row is known, the current row is known, and no chain of
	// observed moves connects them. "I know which entry it opens from, and I have never
	// seen the mark travel from here to there."
	ApproachNoKnownMoves ApproachRefusal = "no_known_moves"
)

// Say renders a refusal as the sentence a person hears.
func (r ApproachRefusal) Say() string {
	switch r {
	case ApproachUnknownEntry:
		return "I've never seen which entry that step goes from."
	case ApproachPositionUnknown:
		return "I can't tell which entry is selected right now."
	case ApproachDifferentList:
		return "The list in front of me has a different number of entries from the one I " +
			"saw that step taken in."
	case ApproachNoKnownMoves:
		return "I know which entry that step was taken from, and I've never seen the mark " +
			"travel there from where it is now."
	}
	return string(r)
}

// Approach is what one step of a route was observed being taken from, and what is known about
// getting the mark there.
//
// Carried ON the plan rather than derived beside it, for the reason PathRank is: an explanation
// assembled separately is a second opinion about a decision already made, and the one nobody
// edits is the one that ends up describing a route nothing would take.
type Approach struct {
	// Step is the edge this concerns, so an approach is never read against the wrong one.
	Step RelationshipRef `json:"step"`
	// Entry and Entries are the row the origin was pointing at most often when this change
	// was observed, and how many the list had. Zero and zero when nothing was ever read.
	Entry   int `json:"entry,omitempty"`
	Entries int `json:"entries,omitempty"`
	// Support and Of are how many of the edge's READABLE observations had that row
	// selected. `3 of 4` and `3 of 12` are different claims and a bare count hides it.
	Support int `json:"support,omitempty"`
	Of      int `json:"of,omitempty"`
	// Unknown is how often the change was seen with nothing readable selected at all. The
	// control evidence, beside the positives rather than implied by subtraction.
	Unknown int `json:"unknown,omitempty"`
	// Moves is every move of the mark observed inside that list, as evidence a chain can be
	// assembled from. Present even when no chain is wanted, because "what do you know about
	// this list" is a question worth being able to answer.
	//
	// Each one now names the directions observed before it, which is what lets a plan say
	// not only which row a step goes from but what was being pressed while the mark
	// travelled there. See binding.go.
	Moves []SelectionStep `json:"moves,omitempty"`
	// Bound is how many of those moves name a direction repeated evidence stands behind.
	//
	// Reported BESIDE the move count rather than instead of it, because the gap between them
	// is the thing a reader has to see: `6 observed moves, 2 of them bound` is a list Marco
	// has watched and largely cannot name the controls of, and a bare "6 moves" reads the
	// same as six it could.
	Bound int `json:"bound_moves,omitempty"`
	// Refusal is why no row is known, when none is. Empty means Entry means something.
	Refusal ApproachRefusal `json:"refusal,omitempty"`
}

// Known reports whether this step has a row a route could aim at.
func (a Approach) Known() bool { return a.Refusal == "" && a.Entries > 0 }

// Describe renders one approach for a person, in the terms the evidence is actually in.
func (a Approach) Describe() string {
	if !a.Known() {
		return fmt.Sprintf("%s → %s: %s", a.Step.From, a.Step.To, a.Refusal.Say())
	}
	out := fmt.Sprintf("%s → %s: observed with entry %d of %d selected, %d of %d readable "+
		"observation(s)", a.Step.From, a.Step.To, a.Entry, a.Entries, a.Support, a.Of)
	if a.Unknown > 0 {
		out += fmt.Sprintf("; %d observation(s) had nothing readable selected", a.Unknown)
	}
	if len(a.Moves) > 0 {
		out += fmt.Sprintf("; %d observed move(s) of the mark in that list, %d of them "+
			"with a direction repeated evidence supports", len(a.Moves), a.Bound)
	}
	return out
}

// Bindings renders what is known about pressing each observed move, one line each.
//
// The inspection surface for the candidate bindings, and it exists for the reason the entry
// line on `director reach` exists: a field nothing renders is a field that quietly stops being
// filled, and this repository has eight recorded cases of exactly that.
func (a Approach) Bindings() []string {
	out := make([]string, 0, len(a.Moves))
	for _, m := range a.Moves {
		out = append(out, m.String())
	}
	return out
}

// ApproachOf reads one remembered edge's selection evidence.
//
// A pure reading of what the store holds. It consults no session, no clock and nothing it could
// change, which is what makes it safe for the planner to call for every step of every candidate
// route.
func ApproachOf(rel RememberedRelationship) Approach {
	a := Approach{
		Step:    RelationshipRef{From: rel.From, To: rel.To},
		Unknown: rel.SelectedUnknown,
		Moves:   append([]SelectionStep{}, rel.Moves...),
	}
	// HOW MANY OF THOSE MOVES COULD BE PRESSED, counted where the moves are read rather
	// than by a caller that would have to know the rule.
	//
	// Deleting this must fail TestAPlannedRouteSaysWhichMovesHaveADirection.
	for _, m := range a.Moves {
		if m.Support().Usable() {
			a.Bound++
		}
	}
	best, ok := DominantSelection(rel.SelectedBefore)
	if !ok {
		// NOT an error and not a gap in the route. Most screens do not change out of a
		// list at all, and a transition with no row to name is the ordinary case.
		a.Refusal = ApproachUnknownEntry
		return a
	}
	a.Entry, a.Entries, a.Support = best.Entry, best.Entries, best.Count
	for _, sel := range rel.SelectedBefore {
		a.Of += sel.Count
	}
	return a
}

// Reach is the chain of observed moves that would take the mark from where it is now to the row
// this step was observed being taken from.
//
// # Why this is a search over observations and not arithmetic
//
// Because "entry 1 to entry 3 is two moves down" is a rule about how lists work, and this
// subsystem is not allowed to have one. A list may wrap, may skip disabled rows, may scroll
// under the mark. What is known is the moves that were SEEN, so the chain is assembled from
// those and from nothing else — and a gap in them is reported as a gap rather than filled in.
//
// Shortest chain wins, ties broken by the better-attested move, so the same memory always
// produces the same approach. Returns nil and a refusal when no chain exists, which includes
// the ordinary case of the mark already being there: an empty chain and `ApproachSatisfied`
// would be two ways of saying nothing needs doing, so the empty slice with no refusal is it.
//
// Deleting the entry-count check must fail TestAnApproachRefusesAListThatRenumbered.
func (a Approach) Reach(now SelectionAt) ([]SelectionStep, ApproachRefusal) {
	if !a.Known() {
		return nil, a.Refusal
	}
	if !now.Known() {
		return nil, ApproachPositionUnknown
	}
	if now.Entries != a.Entries {
		// A list with a different number of entries is a different arrangement, and
		// "entry 3" is not the same row in it. The same guard noteMove applies within a
		// session, applied across the durable boundary.
		return nil, ApproachDifferentList
	}
	if now.Entry == a.Entry {
		// Already there. An empty chain with no refusal, which is the one unambiguous
		// way to say nothing needs doing.
		return []SelectionStep{}, ""
	}

	// Adjacency from the observed moves alone, deterministic: better-attested moves first,
	// so a tie between two chains of the same length is settled by evidence rather than by
	// map order.
	moves := append([]SelectionStep{}, a.Moves...)
	sort.SliceStable(moves, func(i, j int) bool {
		if moves[i].Observations != moves[j].Observations {
			return moves[i].Observations > moves[j].Observations
		}
		if moves[i].From != moves[j].From {
			return moves[i].From < moves[j].From
		}
		return moves[i].To < moves[j].To
	})
	next := map[int][]SelectionStep{}
	for _, m := range moves {
		if m.Entries != a.Entries || m.From == m.To {
			continue
		}
		next[m.From] = append(next[m.From], m)
	}

	// Breadth first, so the first arrival is a shortest chain. The frontier is bounded by
	// the entry count, which the list's own identity bounds, so no visit bound is needed
	// beyond "settle each row once".
	from := map[int]SelectionStep{}
	seen := map[int]bool{now.Entry: true}
	queue := []int{now.Entry}
	for len(queue) > 0 && !seen[a.Entry] {
		at := queue[0]
		queue = queue[1:]
		for _, m := range next[at] {
			if seen[m.To] {
				continue
			}
			seen[m.To] = true
			from[m.To] = m
			queue = append(queue, m.To)
		}
	}
	if !seen[a.Entry] {
		return nil, ApproachNoKnownMoves
	}
	var back []SelectionStep
	for at := a.Entry; at != now.Entry; {
		step := from[at]
		back = append(back, step)
		at = step.From
	}
	out := make([]SelectionStep, 0, len(back))
	for i := len(back) - 1; i >= 0; i-- {
		out = append(out, back[i])
	}
	return out, ""
}

// ApproachesFor reads the selection evidence for every step of a plan.
//
// Aligned with `plan.Steps` one for one, so a caller walking a route never has to match an
// approach to a step by searching. A step whose edge is no longer in the topology gets an
// approach with `ApproachUnknownEntry`, which is the honest reading: the plan was built from
// the same graph, so this can only mean the row was never observed.
func ApproachesFor(steps []RelationshipRef, top Topology) []Approach {
	if len(steps) == 0 {
		return nil
	}
	byRef := make(map[RelationshipRef]RememberedRelationship, len(top.Relationships))
	for _, rel := range top.Relationships {
		byRef[RelationshipRef{From: rel.From, To: rel.To}] = rel
	}
	out := make([]Approach, 0, len(steps))
	for _, step := range steps {
		rel, ok := byRef[step]
		if !ok {
			out = append(out, Approach{Step: step, Refusal: ApproachUnknownEntry})
			continue
		}
		out = append(out, ApproachOf(rel))
	}
	return out
}
