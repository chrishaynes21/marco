package observe

import (
	"fmt"
	"sort"
)

// Remembering that a screen changed WHILE A PARTICULAR ENTRY WAS SELECTED.
//
// # Why this is a different claim from a move of the mark
//
// [[ADR-159-a-move-of-the-mark-is-a-transition-too]] records the mark travelling between two
// entries of one list. The screen does not change, and that is the point of it. This file
// records the other half of using a menu, and the two are not interchangeable:
//
//	MOVING THE MARK        the selected entry of this list changed. Same screen throughout.
//	ACTIVATING AN ENTRY    the screen changed, and at the last look before it a known entry
//	                       of a grounded list was the selected one.
//
// Moving a highlight is not opening a menu. A route made of the first alone can walk a mark up
// and down for ever and never arrive anywhere; a route made of the second alone can say "this
// screen became that one" and has no way to say which of nine rows it took.
//
// Together they are the shape of a taught navigation: get the mark to entry N, then the screen
// changes. Neither half claims the other.
//
// # Three claims, kept apart, exactly as ADR-159 keeps them
//
//	THE OBSERVED CHANGE       one screen composition became another, through the same
//	                          StabilityLooks gate every other transition passes.
//	THE SELECTION BESIDE IT   at the last look before the divergence, the session believed
//	                          entry N of a list of M was selected. Measured in the pixels by
//	                          the exclusivity rule, with no input consulted.
//	THE CONFIRMED EFFECT      confirming at entry N opens that screen. NOT RECORDED AND NOT
//	                          CLAIMED. It needs the action performed and the outcome checked.
//
// So the durable record is a CORRELATION between a change and a selection, named the way
// `Preceded` is named and for the same reason: `SelectedBefore` says what was selected before
// the change, never that selecting it would produce the change.
//
// # Why the entry count travels with the entry
//
// Because "entry 3" is not a fact on its own. A list of nine rows and a list of four rows both
// have a third row and they are not the same place, and a list that gained a row has renumbered
// everything below it. Every representation of a selection in this subsystem carries the count
// beside the index for that reason — see SelectionMove.Entries.
//
// # What is deliberately absent
//
// The list's geometry. A SelectionMove carries `Where` because a session has to tell one list
// from another within a look; a transition does not, because the screen state already names
// the arrangement and a normalised rectangle on a durable edge would be a second, weaker
// identity for the same thing. Entry and count against a recognised screen is the whole claim.

// SelectionAt is what the session believed was selected when one look was taken.
//
// Passed INTO the segmenter rather than read out of it, because the two halves come from
// different places and fail independently: the interface watch reads a mark in shaded pixels,
// and the segmenter compares compositions. A segmenter that reached for the belief itself
// would be a second consumer of a value whose freshness it cannot judge.
type SelectionAt struct {
	// Entry is the selected entry, counted from the top, and Entries how many the list had.
	// The zero value is "nothing was known", which is the ordinary case.
	Entry   int
	Entries int
}

// Known reports whether this look saw a selected entry at all.
//
// Derived rather than stored. A `Known` field would be a second answer to a question the two
// numbers already settle, and the two would eventually disagree.
func (s SelectionAt) Known() bool { return s.Entries > 0 && s.Entry >= 0 }

// SelectionAtFrom reads one look's belief into the value the segmenter takes.
//
// # Why a stale belief is not an answer
//
// Because `Stale` means nothing has confirmed the belief since it was formed, and live most
// looks bring nothing — three slots in four at the shadow cadence. "The mark was on entry 3"
// read four seconds ago is not evidence about the look the screen changed on, and admitting it
// would attach a stale selection to every change that happened while the detector was quiet.
//
// Ambiguous is refused for the reason ADR-158 exists: when two lists both behave like the one
// being navigated, which list "entry 3" counts rows of is exactly what is not established.
//
// Deleting either check must fail TestAStaleOrUnplacedSelectionIsNotActivationEvidence.
func SelectionAtFrom(b InterfaceBelief) SelectionAt {
	if b.Status != InterfaceProvisional || b.Stale || b.Selected < 0 || b.Entries <= 0 {
		return SelectionAt{}
	}
	return SelectionAt{Entry: b.Selected, Entries: b.Entries}
}

// SelectedEntry is one entry that was selected before an observed change, and how often.
//
// The projection `Preceded` is for navigation intents: a tally over a bounded key space, where
// the count beside it is what keeps it readable. `entry 3 of 9: 4` against `observations: 5`
// says the change was seen five times and four of them had that row selected — which is a very
// different claim from four out of four, and a bare "entry 3" renders them identically.
type SelectedEntry struct {
	// Entry is the row, counted from the top; Entries how many the list had.
	Entry   int `json:"entry"`
	Entries int `json:"entries"`
	// Count is how many of this change's observations had that row selected.
	Count int `json:"count"`
}

// String renders one row of the table a report prints.
func (s SelectedEntry) String() string {
	return fmt.Sprintf("entry %d of %d: %d", s.Entry, s.Entries, s.Count)
}

// MaxSelectedPerTransition bounds how many distinct selections one edge remembers.
//
// Nine, which is a real pause menu's whole length. Past it an edge is not "the screen that
// opens from one row" — it is a change that happens whatever is selected, which is exactly
// what SelectedUnknown and the counts are there to make visible rather than something to
// store a hundred rows about.
const MaxSelectedPerTransition = 9

// selectedTally accumulates the selections seen before one edge, bounded and deterministic.
type selectedTally struct {
	rows map[SelectionAt]int
	// unknown counts observations whose origin look had no readable selection, and dropped
	// those refused at the bound.
	unknown int
	dropped int
}

// fold records one observation of this edge.
func (s *selectedTally) fold(sel SelectionAt) {
	if !sel.Known() {
		// NOT dropped and not implied by subtraction. An edge seen ten times with a row
		// selected on three of them is mostly a change that happens with no readable
		// selection at all, and a representation without this number would read as
		// "entry 3, three times" with nothing to weigh it against.
		s.unknown++
		return
	}
	if s.rows == nil {
		s.rows = map[SelectionAt]int{}
	}
	if _, seen := s.rows[sel]; !seen && len(s.rows) >= MaxSelectedPerTransition {
		s.dropped++
		return
	}
	s.rows[sel]++
}

// publish copies the tally out, most-seen first, deterministically.
func (s selectedTally) publish() []SelectedEntry {
	if len(s.rows) == 0 {
		return nil
	}
	out := make([]SelectedEntry, 0, len(s.rows))
	for at, n := range s.rows {
		out = append(out, SelectedEntry{Entry: at.Entry, Entries: at.Entries, Count: n})
	}
	sortSelectedEntries(out)
	return out
}

// sortSelectedEntries puts the best-supported selection first, then orders by row.
func sortSelectedEntries(out []SelectedEntry) {
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Count != out[b].Count {
			return out[a].Count > out[b].Count
		}
		if out[a].Entries != out[b].Entries {
			return out[a].Entries < out[b].Entries
		}
		return out[a].Entry < out[b].Entry
	})
}

// DominantSelection is the entry most often selected before a change, and how often.
//
// Reported with its count for the reason ScreenTransition.Dominant is: one observation of a row
// before a change is not the same claim as four out of four, and a bare "the entry" would
// render them identically. The second return is false when nothing was ever readable.
func DominantSelection(in []SelectedEntry) (SelectedEntry, bool) {
	if len(in) == 0 {
		return SelectedEntry{}, false
	}
	// Already sorted most-seen first by publish and by the durable fold, and sorting again
	// here would be a second answer to which one wins.
	return in[0], true
}

// foldSelected merges one session's selections into a durable set, bounded.
//
// Shared by the relationship store and the tally above rather than written twice: the bound and
// the ordering are the same rule, and two copies of a bound is how two different answers to
// "how many does an edge remember" appear.
func foldSelected(into []SelectedEntry, from []SelectedEntry) ([]SelectedEntry, int) {
	dropped := 0
	for _, add := range from {
		if add.Count <= 0 || add.Entries <= 0 || add.Entry < 0 {
			continue
		}
		found := false
		for i := range into {
			if into[i].Entry == add.Entry && into[i].Entries == add.Entries {
				into[i].Count += add.Count
				found = true
				break
			}
		}
		if found {
			continue
		}
		if len(into) >= MaxSelectedPerTransition {
			dropped++
			continue
		}
		into = append(into, SelectedEntry{
			Entry: add.Entry, Entries: add.Entries, Count: add.Count,
		})
	}
	sortSelectedEntries(into)
	return into, dropped
}
