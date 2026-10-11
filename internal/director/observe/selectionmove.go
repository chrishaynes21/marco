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
	// Dated is how many of this move's observations could say when the pixels moved, and
	// ConfirmLateMS the total delay between the pixels moving and the reader agreeing.
	//
	// A SUM AND A COUNT and not a mean, so a reader can see both halves: a mean alone cannot
	// distinguish one badly delayed sighting from six slightly delayed ones, and the
	// difference decides whether the cadence or the reader is the thing to look at. Dated is
	// below Count whenever a sighting could not be placed, which is the ordinary case for
	// the look a list is first identified on.
	Dated         int   `json:"dated,omitempty"`
	ConfirmLateMS int64 `json:"confirm_late_ms,omitempty"`
	// Directions are the directions attributed to this move's observations, one entry per
	// distinct direction, and DirectionAmbiguous and DirectionUnknown the observations no
	// single direction could be named for.
	//
	// A THIRD CLAIM, after the move and after the press, and it is deliberately the last
	// thing to arrive: a direction is joined to a move the shading already proved, by when
	// alone, outside anything that reads a mark. See binding.go for the order and for why
	// there is no field here for a direction that was verified.
	//
	// The three counters partition Prompted exactly — every observation with a press
	// attributable to it named one direction, named several, or lost the slot behind it —
	// and TestEveryPromptedObservationIsAccountedFor holds that.
	Directions []DirectionCount `json:"directions,omitempty"`
	// DirectionAmbiguous counts observations whose window held MORE THAN ONE direction, so
	// naming one would invent the half that was not observed.
	DirectionAmbiguous int `json:"direction_ambiguous,omitempty"`
	// DirectionUnknown counts observations the watch found a press for whose slot this
	// layer could not recover — a press observed after the move was seen, or a session with
	// no clock. Counted rather than silently folded into the ambiguous ones: they are
	// different situations, and one of them is a refusal this layer makes on purpose.
	DirectionUnknown int `json:"direction_unknown,omitempty"`
}

// Attributed is how many of this move's observations had a press attributable to them.
func (s SelectionTransition) Attributed() int { return s.Prompted }

// Support is what this move's direction evidence amounts to, in the closed vocabulary.
func (s SelectionTransition) Support() BindingSupport {
	return BindingSupportOf(s.Directions, s.DirectionAmbiguous, s.DirectionUnknown)
}

// String renders one row of the table a report prints.
func (s SelectionTransition) String() string {
	where := string(s.State)
	if where == "" {
		where = "unplaced"
	}
	return fmt.Sprintf("%s entry %d->%d of %d: %d seen, %d prompted, %d unprompted; %s",
		where, s.From, s.To, s.Entries, s.Count, s.Prompted, s.Unprompted,
		DescribeDirections(s.Directions, s.DirectionAmbiguous, s.DirectionUnknown))
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
// # What it claims about the direction, and what it still does not
//
// It names the directions that were OBSERVED before the move was seen, as candidates with their
// own counters and their own uncertainty — see binding.go, which added them and keeps the join
// strictly after the pixels. What it does not claim is that pressing one of them produces the
// move. That needs the press performed deliberately and the outcome checked, which is a
// rehearsal, and there is deliberately nowhere here to record one.
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
	// Directions are the directions attributed to those observations, Ambiguous the ones
	// whose window held several, and Unknown the ones whose press could not be recovered.
	//
	// The durable half of the candidate binding. It crosses the boundary because that is the
	// whole point: a chain of moves nobody can name a direction for is a route Marco can
	// describe and not one anybody could be asked to take, and the naming has to survive the
	// session that observed it or every restart starts again.
	Directions []DirectionCount `json:"directions,omitempty"`
	Ambiguous  int              `json:"direction_ambiguous,omitempty"`
	Unknown    int              `json:"direction_unknown,omitempty"`
}

// Support is what this step's direction evidence amounts to, in the closed vocabulary.
//
// The SAME function the session-local transition calls, deliberately: two copies of the
// classification is how a session comes to report `repeated` about a move the store calls
// `thin`.
func (s SelectionStep) Support() BindingSupport {
	return BindingSupportOf(s.Directions, s.Ambiguous, s.Unknown)
}

// String renders one row of the table a report prints.
func (s SelectionStep) String() string {
	return fmt.Sprintf("entry %d->%d of %d: %d seen, %d prompted; %s",
		s.From, s.To, s.Entries, s.Observations, s.Prompted,
		DescribeDirections(s.Directions, s.Ambiguous, s.Unknown))
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
			// AND WHICH DIRECTION WAS OBSERVED BEFORE EACH, copied rather than
			// re-derived. A durable chain of hops nobody can name a direction for
			// is exactly what ADR-160 left behind.
			//
			// Deleting this must fail TestADurableStepRemembersWhichDirectionWasPressed.
			Directions: append([]DirectionCount{}, st.Directions...),
			Ambiguous:  st.DirectionAmbiguous,
			Unknown:    st.DirectionUnknown,
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
				// ACCUMULATED, so a binding gets stronger across sessions
				// rather than being replaced by the newest reading. The
				// dropped count is discarded here and not at the caller: a
				// direction past the bound means the vocabulary leaked, and
				// the caller already counts dropped steps for the same edge.
				into[i].Directions, _ = foldDirections(into[i].Directions, add.Directions)
				into[i].Ambiguous += add.Ambiguous
				into[i].Unknown += add.Unknown
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
		// THROUGH THE SAME VOCABULARY CHECK as a merge, rather than taken as given. A
		// first sighting of a move must not be the way a `confirm` gets into a field that
		// only ever holds a direction.
		add.Directions, _ = foldDirections(nil, add.Directions)
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

// fold records one move against one screen, which may be unknown, with the direction the input
// record independently offered for it.
//
// `att` arrives as an argument rather than being looked up here, and that is the trust boundary
// in the shape of a signature: this ledger cannot reach a press, so a direction can only ever
// be handed to it by a caller that already had the move. See binding.go.
func (l *selectionLedger) fold(state ScreenStateID, m SelectionMove, att navAttribution) {
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
	// HOW LATE THE READER WAS, measured rather than assumed. Folded here because this is
	// the one place every observation of a move passes through.
	if !m.FirstSeen.IsZero() {
		row.Dated++
		row.ConfirmLateMS += m.ConfirmedAfterMS
	}
	// THE DIRECTION, folded only where a press was attributable at all. The three
	// branches partition Prompted exactly, which is what makes the record checkable:
	// every prompted observation named one direction, named several, or lost its slot.
	//
	// Deleting this must fail TestAMoveRemembersWhichDirectionWasObservedBeforeIt.
	switch {
	case att.ambiguous:
		row.DirectionAmbiguous++
	case att.unknown:
		row.DirectionUnknown++
	case att.intent != "":
		row.foldDirection(att)
	}
	if m.FromUnnamed > row.FromUnnamed {
		row.FromUnnamed = m.FromUnnamed
	}
}

// foldDirection records one attribution against this move.
//
// # Where the vocabulary is enforced, and why not here
//
// In `pressLedger.bank`, which admits only the four directional intents, so `attribute` cannot
// return anything else and a check here would be a second answer to a settled question. An
// earlier version had one; the mutation gate showed that deleting it changed no outcome, which
// is this repository's own recorded "unreachable discriminator". The check that DOES earn its
// place is in `foldDirections`, on the way into the durable store, where the slice arrives from
// a caller rather than from this package — see TestOnlyADirectionCanBeTheDirectionOfAMove.
//
// Bounded by the vocabulary rather than by a policy: a fifth distinct direction cannot exist,
// so the bound is a check that something upstream is still filtering. An attribution past it is
// dropped into DirectionUnknown, which is the honest place for a press this layer could not
// account for.
func (s *SelectionTransition) foldDirection(att navAttribution) {
	for i := range s.Directions {
		if s.Directions[i].Intent == att.intent {
			s.Directions[i].Count++
			if att.conditional {
				s.Directions[i].Conditional++
			}
			sortDirections(s.Directions)
			return
		}
	}
	if len(s.Directions) >= MaxDirectionsPerMove {
		s.DirectionUnknown++
		return
	}
	add := DirectionCount{Intent: att.intent, Count: 1}
	if att.conditional {
		add.Conditional = 1
	}
	s.Directions = append(s.Directions, add)
	sortDirections(s.Directions)
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
