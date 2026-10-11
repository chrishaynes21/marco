package observe

import (
	"fmt"
	"sort"
)

// Remembering that the mark moved from one entry of a list to another.
//
// # The shift this file exists for
//
// Everything before it could say "something on the screen changed after an input". A screen
// transition is keyed on two whole screens, and the question it answers is whether one
// composition became another. That is the right unit for "pause opened a menu" and the wrong
// one for the thing a menu mostly does, which is hand a mark between its own rows without the
// screen changing at all.
//
// So this records the smaller, more useful fact:
//
//	in the list Marco independently established was the one being navigated,
//	the mark moved from entry 2 to entry 3,
//	and a directional press was observed within the latency bound before it was seen.
//
// # Three claims, kept apart, in that order
//
// A graph that collapsed them would be unreadable, and the collapse is the easy mistake:
//
//	THE OBSERVED CHANGE       the mark was on entry 2 and then on entry 3. Measured in the
//	                          pixels, by the exclusivity rule, with no input consulted.
//	THE ATTRIBUTED INPUT      a directional intent was observed within MaxResponseLatency
//	                          before the move was SEEN. Temporal association, nothing more.
//	THE CONFIRMED EFFECT      pressing that direction from entry 2 arrives at entry 3. NOT
//	                          RECORDED HERE AND NOT CLAIMED ANYWHERE. It needs an action
//	                          performed deliberately and the outcome checked, which is a
//	                          rehearsal, which is not a passive observation.
//
// Count is the first. Prompted is the second. There is deliberately no field for the third.
//
// # What identifies a list
//
// Where it is, how many entries it has, and the screen it was on when the session could place
// the screen at all. The screen is PROVENANCE and not identity: the pixels discovery path
// proposes geometry nobody named, and screen segmentation reads named roles, so a session
// reading a menu perfectly well may have no screen state to attach it to. The list is still
// grounded — by the responsiveness evidence of
// [[ADR-158-the-active-list-is-the-one-that-responds]], which is what established that this
// list and not the panel beside it was the one being used — and saying "on an unknown screen"
// is the honest version of that. See [SelectionTransition.State].
//
// # Nothing here knows what any entry MEANS
//
// Entry 3 is the third row from the top. Not "Training", not "Free Play", not a label, not a
// target. A later layer may learn that a row with a certain name sits at index 3 on a certain
// screen; this one would be wrong to pretend it already had.

// MaxSelectionTransitions bounds how many distinct moves one session remembers.
//
// Generous against a real menu — a nine-entry list navigated with up and down produces sixteen
// adjacent pairs — and bounded because a list whose mark jumps anywhere from anywhere produces
// entries quadratically, and that is the shape of a heads-up display rather than a menu.
const MaxSelectionTransitions = 128

// SelectionTransition is one observed move of the mark between two entries of one list.
type SelectionTransition struct {
	// State is the screen the list was on, and is empty when the session could not place
	// one.
	//
	// PROVENANCE, NOT IDENTITY, and empty is an ordinary answer rather than a failure. Two
	// different causes produce it and both are real: the discovery path proposed geometry
	// nobody named, so nothing segmented a screen at all; or a screen change was already
	// pending while these looks were taken, so which screen the list sat on is exactly what
	// the session could not say. Either way the move was still observed, and dropping it
	// because the screen was unknown would discard a measurement for want of a label.
	State ScreenStateID `json:"state,omitempty"`
	// Where and Entries are the list. Window-relative, normalised, and counted.
	Where   Region `json:"where,omitzero"`
	Entries int    `json:"entries"`
	// From and To are the entry indices either side of the move, counted from the top.
	From int `json:"from"`
	To   int `json:"to"`
	// Count is how many times this exact move was seen.
	Count int `json:"count"`
	// Prompted counts the times a directional navigation intent was observed within
	// MaxResponseLatency before the move was seen; Unprompted, the times none was.
	//
	// Itemised and never netted, like every other pair of counters in this layer. The
	// difference between 6/0 and 6/40 is the whole difference between a list that responds
	// to a person and a list that moves on its own, and a single ratio hides it.
	//
	// Unprompted means NO PRESS WAS ATTRIBUTABLE. It does not mean nobody pressed anything:
	// at a coarse cadence the window closes between looks. Both readings are stated wherever
	// this is reported, because they call for opposite responses.
	Prompted   int `json:"prompted,omitempty"`
	Unprompted int `json:"unprompted,omitempty"`
	// Tracked counts the times a fast look saw the move rather than a discovery.
	//
	// Reported because it is the weaker observation, and because live it will be most of
	// them: the fast path reads the mark several times per cycle over geometry a discovery
	// found up to a cycle earlier.
	Tracked int `json:"tracked,omitempty"`
	// FromUnnamed is the most entries of this list that were ever geometry nobody could
	// name. Equal to Entries means no entry in the list has a name.
	//
	// The warning ADR-157 attached to the shapes channel, carried this far: a mark moving
	// between two unnamed rectangles reads exactly like a mark moving between two named
	// controls, and the claim is weaker. A reader deciding whether to build a route on this
	// is entitled to know.
	FromUnnamed int `json:"entries_from_unnamed_geometry,omitempty"`
	// First and Last are the samples this move was first and last seen at, so a reader can
	// tell six moves over a session from six in one second.
	First int `json:"first,omitempty"`
	Last  int `json:"last,omitempty"`
}

// Attributed is how many of this move's observations had a press attributable to them.
func (s SelectionTransition) Attributed() int { return s.Prompted }

// String renders one row of the table a report prints.
func (s SelectionTransition) String() string {
	where := string(s.State)
	if where == "" {
		where = "unplaced"
	}
	return fmt.Sprintf("%s entry %d->%d of %d: %d seen, %d prompted, %d unprompted",
		where, s.From, s.To, s.Entries, s.Count, s.Prompted, s.Unprompted)
}

// MaxDurableSelectionSteps bounds how many moves one durable edge remembers.
//
// Twenty-four. A nine-entry list walked with up and down produces sixteen adjacent pairs and
// two more if it wraps, so this is above a real pause menu with room to spare. Past it a list
// whose mark jumps anywhere from anywhere is not a menu, and the dropped counter says so
// rather than the record growing.
const MaxDurableSelectionSteps = 24

// SelectionStep is one observed move of the mark within a list, as durable memory keeps it.
//
// # Why this is a separate type from SelectionTransition
//
// Because the durable side must not be able to carry what the session side carries. A
// SelectionTransition holds a normalised rectangle and a session-local screen id, and neither
// survives a restart with any meaning: the state is renumbered next run, and a second durable
// identity for a list Marco already recognises by its subject would be a weaker answer to a
// question the subject already settles.
//
// So the durable record is an entry, an entry count and two counters. The split is structural
// rather than a matter of remembering to strip fields — the same reason NavSequence exists
// beside TargetedSequence.
//
// # What it still does not claim
//
// Which key produces the move. The attribution says a directional press was observed within
// the window before the move was SEEN, and deliberately not which direction: the count a
// selection reader is handed is a count, so that an input can never predict the visual outcome
// it is being correlated with. A chain of these is therefore a route Marco can DESCRIBE and
// not one it can perform, and that gap is named in the subsystem note rather than papered over.
type SelectionStep struct {
	// From and To are the entry indices either side of the move, counted from the top, and
	// Entries how many the list had. All three, because "entry 3" means nothing except
	// against a stable count of them.
	From    int `json:"from"`
	To      int `json:"to"`
	Entries int `json:"entries"`
	// Observations is how often the move was seen, Prompted how often a directional press
	// was attributable to it. Itemised and never netted, like every other pair here.
	Observations int `json:"observations"`
	Prompted     int `json:"prompted,omitempty"`
}

// String renders one row of the table a report prints.
func (s SelectionStep) String() string {
	return fmt.Sprintf("entry %d->%d of %d: %d seen, %d prompted",
		s.From, s.To, s.Entries, s.Observations, s.Prompted)
}

// SelectionStepsFrom projects this session's moves on ONE screen onto the durable shape.
//
// Keyed on the session-local state so the caller joins them to the edge whose ORIGIN that
// screen is. Moves the session could not place are deliberately not included: a move on an
// unknown screen is real evidence about a list and says nothing about which screen a route
// would start from, and attaching it to an edge would guess exactly that.
//
// Deleting the state check must fail TestAnUnplacedMoveIsNotEvidenceAboutAnEdge.
func SelectionStepsFrom(in []SelectionTransition, state ScreenStateID) []SelectionStep {
	if state == "" || state == ScreenStateUnknown {
		return nil
	}
	var out []SelectionStep
	for _, st := range in {
		if st.State != state || st.Count <= 0 {
			continue
		}
		out = append(out, SelectionStep{
			From: st.From, To: st.To, Entries: st.Entries,
			Observations: st.Count, Prompted: st.Prompted,
		})
		if len(out) >= MaxDurableSelectionSteps {
			break
		}
	}
	sortSelectionSteps(out)
	return out
}

// sortSelectionSteps orders moves so two readings of one session produce one file.
func sortSelectionSteps(out []SelectionStep) {
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Entries != out[b].Entries {
			return out[a].Entries < out[b].Entries
		}
		if out[a].From != out[b].From {
			return out[a].From < out[b].From
		}
		return out[a].To < out[b].To
	})
}

// foldSelectionSteps merges one session's moves into a durable set, bounded.
//
// Additive on the counters and set-union on the identity, which is the rule RememberedRelationship
// already applies to intents and runs. The bound lives here rather than at the caller so every
// path into the store obeys it.
func foldSelectionSteps(into, from []SelectionStep) ([]SelectionStep, int) {
	dropped := 0
	for _, add := range from {
		if add.Observations <= 0 || add.Entries <= 0 || add.From < 0 || add.To < 0 {
			continue
		}
		if add.From == add.To {
			// Not a move. A record of the mark staying where it was would read as
			// evidence that a direction exists from a row to itself.
			continue
		}
		found := false
		for i := range into {
			if into[i].From == add.From && into[i].To == add.To &&
				into[i].Entries == add.Entries {
				into[i].Observations += add.Observations
				into[i].Prompted += add.Prompted
				found = true
				break
			}
		}
		if found {
			continue
		}
		if len(into) >= MaxDurableSelectionSteps {
			dropped++
			continue
		}
		into = append(into, add)
	}
	sortSelectionSteps(into)
	return into, dropped
}

// selectionKey is a move's identity: which list, on which screen, between which entries.
type selectionKey struct {
	state   ScreenStateID
	where   Region
	entries int
	from    int
	to      int
}

// selectionLedger accumulates grounded moves for one session.
type selectionLedger struct {
	rows map[selectionKey]*SelectionTransition
	// evicted counts moves the bound refused, and unplaced those recorded with no screen.
	evicted  int
	unplaced int
}

// fold records one move against one screen, which may be unknown.
func (l *selectionLedger) fold(state ScreenStateID, m SelectionMove) {
	if m.From < 0 || m.To < 0 || m.From == m.To || m.Entries <= 0 {
		return
	}
	if state == "" || state == ScreenStateUnknown {
		state = ""
		l.unplaced++
	}
	k := selectionKey{
		state: state, where: m.Where, entries: m.Entries, from: m.From, to: m.To,
	}
	if l.rows == nil {
		l.rows = map[selectionKey]*SelectionTransition{}
	}
	row, ok := l.rows[k]
	if !ok {
		if len(l.rows) >= MaxSelectionTransitions {
			l.evicted++
			return
		}
		row = &SelectionTransition{
			State: state, Where: m.Where, Entries: m.Entries,
			From: m.From, To: m.To, First: m.Sample,
		}
		l.rows[k] = row
	}
	row.Count++
	row.Last = m.Sample
	if m.Prompted {
		row.Prompted++
	} else {
		row.Unprompted++
	}
	if m.Tracked {
		row.Tracked++
	}
	if m.FromUnnamed > row.FromUnnamed {
		row.FromUnnamed = m.FromUnnamed
	}
}

// publish copies the ledger out, most-seen first, deterministically.
func (l *selectionLedger) publish() []SelectionTransition {
	if len(l.rows) == 0 {
		return nil
	}
	out := make([]SelectionTransition, 0, len(l.rows))
	for _, row := range l.rows {
		out = append(out, *row)
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Count != out[b].Count {
			return out[a].Count > out[b].Count
		}
		if out[a].State != out[b].State {
			return out[a].State < out[b].State
		}
		if out[a].From != out[b].From {
			return out[a].From < out[b].From
		}
		return out[a].To < out[b].To
	})
	return out
}
