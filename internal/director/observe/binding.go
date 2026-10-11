package observe

import (
	"fmt"
	"sort"
	"time"
)

// Associating a direction with a move the pixels already proved.
//
// # The gap this closes, and the one it does not
//
// [[ADR-160-activating-an-entry-is-not-moving-the-mark]] left a route that could be DESCRIBED
// and not performed. It can say "the mark was seen going entry 1 to entry 2 to entry 3, and the
// screen changed with entry 3 marked", and it cannot say what to press. The reason was
// deliberate: the selection reader is handed a COUNT of directional intents and never a
// direction, so an input can never predict the visual outcome it is correlated with — which is
// [[ADR-158-the-active-list-is-the-one-that-responds]]'s trust boundary.
//
// This file does not weaken that boundary. It adds a SECOND, LATER reading, in a strict order
// that the code's shape enforces rather than its comments:
//
//	1. the pixels establish that the mark moved from entry 1 to entry 2        (the watch)
//	2. the input record independently says which directions were observed      (this ledger)
//	3. the two are joined, AFTER the move exists, by WHEN alone                 (attribute)
//	4. the join is retained as a CANDIDATE, with its uncertainty                (DirectionCount)
//
// Step 3 is the whole safeguard. The ledger is written on the same line that opens the watch's
// response window and read only in `groundSelectionMoves`, which runs on moves the watch has
// already emitted. No direction is reachable from anything that reads a mark: the watch holds no
// pointer to a ledger, and a session whose presses are all `up` reads exactly the same
// selections as one whose presses are all `down` — asserted, not promised, by
// TestTheDirectionOfAPressChangesNothingAboutWhatIsSeen.
//
// # Three claims, kept apart, exactly as ADR-159 keeps them
//
//	THE OBSERVED MOVE          the mark was on entry 1 and then on entry 2. Shading only.
//	THE ATTRIBUTED DIRECTION   `down` was the direction observed in the window before the
//	                           move was seen. TEMPORAL ASSOCIATION, nothing more.
//	THE VERIFIED BINDING       pressing down from entry 1 arrives at entry 2. NOT RECORDED
//	                           HERE AND NOT RECORDABLE HERE. It needs the press performed
//	                           deliberately and the outcome checked, which is a rehearsal.
//
// There is deliberately no field for the third. That is not an oversight to be filled in later:
// a passive observer has nothing to put in it, and a `Verified bool` that nothing could set is
// how a candidate comes to be read as a confirmation. The same argument SelectionTransition
// makes for having no field for the confirmed effect.
//
// # What it refuses to generalise
//
// A binding is attached to ONE move of ONE list on ONE screen: `down was observed before the
// mark went from entry 1 to entry 2 of the nine-entry list on state_3`. Not "down means next".
// Not "down wraps at the bottom". Not "down from entry 1 of any list". A menu may wrap, may skip
// disabled rows, may scroll under the mark, and may be laid out left to right so that `down`
// does nothing in it at all — so the rule a route uses stays the chain of moves that were SEEN,
// and this names each hop's direction without licensing a hop nobody watched.

// DirectionCount is one direction attributed to observations of one move, and how often.
//
// The projection `SelectedEntry` is for rows and `Preceded` is for intents before a change: a
// tally over a bounded key space, with the count beside it because `down: 1` and `down: 9` are
// different claims and a bare word renders them identically.
type DirectionCount struct {
	// Intent is the direction, from NavIntent's closed vocabulary and only ever one of the
	// four directional ones. A confirm or a back changes which screen is in front rather
	// than which row is marked, so one appearing here would be a join that credited a
	// list's own transfer to an input that could not have moved it.
	Intent NavIntent `json:"intent"`
	// Count is how many of the move's observations had this direction attributed.
	Count int `json:"count"`
	// Conditional is how many of those rested ENTIRELY on keys that are only navigation in
	// context — see [[ADR-013-navigation-is-meaning-not-keys]] and InputEvent.Conditional.
	//
	// Kept weaker rather than folded in. W while the screen looked like a set of choices is
	// a real `up` and is also "walk forwards", and a binding learned from a session of
	// somebody walking around must not read as confidently as one learned from arrows.
	Conditional int `json:"conditional,omitempty"`
}

// String renders one row of the table a report prints.
func (d DirectionCount) String() string {
	out := fmt.Sprintf("%s: %d", d.Intent, d.Count)
	if d.Conditional > 0 {
		out += fmt.Sprintf(" (%d from context-admitted keys)", d.Conditional)
	}
	return out
}

// MaxDirectionsPerMove bounds how many distinct directions one move remembers.
//
// Four, which is the whole directional vocabulary — so this is a vocabulary check wearing a
// bound's clothes rather than a policy, and it exists because NavIntent's underlying type is a
// string and a fifth value arriving means something upstream stopped filtering.
const MaxDirectionsPerMove = 4

// BindingSupport is the CLOSED vocabulary of what a candidate binding's evidence amounts to.
//
// Five, and they are genuinely different situations calling for different responses, which is
// the argument ApproachRefusal and SessionLocalCause both make. A single confidence number
// would render the two that matter most — nobody pressed anything attributable, and the presses
// disagreed — as the same small float.
type BindingSupport string

const (
	// BindingUnattributed — the move was seen and no press was ever attributable to it.
	// Two readings, and nothing here tells them apart: nobody pressed anything, or the
	// sampling cadence closed the window between looks. Both are carried forward.
	BindingUnattributed BindingSupport = "unattributed"
	// BindingAmbiguous — presses were attributable and more than one direction was in the
	// window every time, so no single direction can be named. A person holding two keys, or
	// a cadence coarse enough that a whole walk lands in one slot.
	BindingAmbiguous BindingSupport = "ambiguous_press"
	// BindingContradicted — different directions were attributed across observations of the
	// SAME move, with no clear leader. Real evidence against any one of them being what
	// moved the mark, and the case a bare majority would hide.
	BindingContradicted BindingSupport = "contradicted"
	// BindingThin — one direction leads, on too few observations to call it anything. The
	// ordinary state of a binding on its way to being usable.
	BindingThin BindingSupport = "thin_evidence"
	// BindingRepeated — one direction leads by the margin, on at least MinBindingAgreement
	// observations. The strongest thing passive observation can say, and still a candidate:
	// nothing has pressed it on purpose and checked.
	BindingRepeated BindingSupport = "repeated"
)

// Say renders the support as the sentence a person hears.
func (b BindingSupport) Say() string {
	switch b {
	case BindingUnattributed:
		return "I have never caught a press that could have moved the mark that way."
	case BindingAmbiguous:
		return "More than one direction was pressed each time I saw that move, so I " +
			"cannot say which one did it."
	case BindingContradicted:
		return "I have seen that move follow more than one direction, so I do not know " +
			"which to trust."
	case BindingThin:
		return "One direction fits, on too little evidence to rely on."
	case BindingRepeated:
		return "One direction fits every time I have seen that move — though I have " +
			"never pressed it myself to check."
	}
	return string(b)
}

// Usable reports whether repeated evidence stands behind one direction.
//
// NOT whether it may be performed. Every step still earns execution through its own rehearsal
// and its own yes, exactly as a plan does; this answers "is there a direction worth proposing",
// which is the question a route has to ask before it can propose anything at all.
func (b BindingSupport) Usable() bool { return b == BindingRepeated }

// MinBindingAgreement is how many agreeing observations a direction needs before the evidence
// is called repeated.
//
// Three, and deliberately above MinNavigationObservations. Two is what makes a MOVE worth
// retaining at all — the mark was seen doing the same thing twice — and a binding is a stronger
// claim resting on top of that one: it survives a single mis-attribution being outvoted, which
// two cannot. It is not a threshold for ACTING; nothing here licenses that at any count.
const MinBindingAgreement = 3

// BindingMargin is how far the leading direction must out-count the next one.
//
// Twice, the same shape as selection.NavMargin and for the same reason: a leader that barely
// leads is a disagreement rather than a decision. A margin rather than a veto on purpose — one
// stray attribution in nine should not poison a binding for the life of the store, and
// `contradicted` has to mean the evidence genuinely disagrees rather than that it once did.
const BindingMargin = 2

// BindingSupportOf classifies one move's direction evidence.
//
// THE ONE PLACE the rule lives. Both the session-local transition and the durable step call it
// rather than each deciding, because two copies of a classification is how a session comes to
// report `repeated` about a move the store calls `thin`.
func BindingSupportOf(dirs []DirectionCount, ambiguous, unknown int) BindingSupport {
	leader, runner := 0, 0
	for _, d := range dirs {
		switch {
		case d.Count > leader:
			leader, runner = d.Count, leader
		case d.Count > runner:
			runner = d.Count
		}
	}
	if leader == 0 {
		// Nothing was ever pinned to a direction, and WHICH of the two reasons it was
		// matters: a window that held several directions is a cadence or a person, and a
		// window that held none is the attribution never landing at all. `unknown` is
		// deliberately not a third verdict here — a press nobody can recover is an
		// attribution that did not land, which is what unattributed already says.
		if ambiguous > 0 {
			return BindingAmbiguous
		}
		return BindingUnattributed
	}
	if runner > 0 && leader < BindingMargin*runner {
		return BindingContradicted
	}
	if leader >= MinBindingAgreement {
		return BindingRepeated
	}
	return BindingThin
}

// DominantDirection is the direction most often attributed, and how often.
//
// Reported with its count for the reason DominantSelection is: one observation and nine out of
// nine are different claims, and a bare word renders them identically. The second return is
// false when nothing was ever attributed — NOT when the evidence is weak, which is
// BindingSupportOf's question and not this one.
func DominantDirection(in []DirectionCount) (DirectionCount, bool) {
	if len(in) == 0 {
		return DirectionCount{}, false
	}
	// Already ordered best-first by every path that builds one, and sorting here again
	// would be a second answer to which direction wins.
	return in[0], true
}

// sortDirections puts the best-attested direction first, then orders by the vocabulary so two
// readings of one session produce one file.
func sortDirections(out []DirectionCount) {
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Count != out[b].Count {
			return out[a].Count > out[b].Count
		}
		return out[a].Intent < out[b].Intent
	})
}

// foldDirections merges one session's attributions into a durable set.
//
// Additive on the counters and set-union on the direction, which is the rule
// RememberedRelationship already applies to intents, runs, selections and moves. A direction
// outside the directional vocabulary is DROPPED rather than carried: NavIntent's underlying
// type is a string, and a `confirm` in this field would be a join crediting a screen change to
// a move of the mark.
func foldDirections(into, from []DirectionCount) ([]DirectionCount, int) {
	dropped := 0
	for _, add := range from {
		if add.Count <= 0 || !directional(add.Intent) {
			continue
		}
		found := false
		for i := range into {
			if into[i].Intent == add.Intent {
				into[i].Count += add.Count
				into[i].Conditional += add.Conditional
				found = true
				break
			}
		}
		if found {
			continue
		}
		if len(into) >= MaxDirectionsPerMove {
			dropped++
			continue
		}
		into = append(into, DirectionCount{
			Intent: add.Intent, Count: add.Count, Conditional: add.Conditional,
		})
	}
	sortDirections(into)
	return into, dropped
}

// directional reports whether an intent can move a mark WITHIN a list.
//
// The same four `directionalIntents` counts, from the same vocabulary, and the reason they are
// the only four is in that function: a confirm or a back changes which screen is in front
// rather than which row is marked.
func directional(n NavIntent) bool {
	switch n {
	case NavUp, NavDown, NavLeft, NavRight:
		return true
	}
	return false
}

// DescribeDirections renders a move's direction evidence as the line a report prints.
func DescribeDirections(dirs []DirectionCount, ambiguous, unknown int) string {
	out := string(BindingSupportOf(dirs, ambiguous, unknown))
	for i, d := range dirs {
		if i == 0 {
			out += " — "
		} else {
			out += ", "
		}
		out += d.String()
	}
	if ambiguous > 0 {
		out += fmt.Sprintf(", %d with several directions in the window", ambiguous)
	}
	if unknown > 0 {
		out += fmt.Sprintf(", %d whose press could not be recovered", unknown)
	}
	return out
}

// Performable reports whether every hop of a chain names a direction repeated evidence stands
// behind, and the weakest support in the chain when one does not.
//
// IT AUTHORISES NOTHING, and the name is about the chain's completeness rather than about
// permission: a chain every hop of which names a direction is one a performer could be ASKED to
// take, and it would still earn execution through the same rehearsal and the same yes as every
// other step. True here is the absence of a known gap, never the presence of a verification.
//
// An empty chain is performable, and that is the right answer rather than a special case: the
// mark is already where the route wants it, so there is nothing to press and nothing missing.
func Performable(chain []SelectionStep) (bool, BindingSupport) {
	ok, weakest := true, BindingRepeated
	for _, hop := range chain {
		s := hop.Support()
		if s != BindingRepeated {
			ok = false
		}
		if bindingRank(s) < bindingRank(weakest) {
			weakest = s
		}
	}
	if ok {
		return true, BindingRepeated
	}
	return false, weakest
}

// bindingRank orders the vocabulary worst-first, so "the weakest hop" has one answer.
//
// A function over the closed vocabulary rather than a map built per call: an unrecognised word
// ranks worst, which is the honest reading of a support nothing in this file produced.
func bindingRank(b BindingSupport) int {
	switch b {
	case BindingAmbiguous:
		return 1
	case BindingContradicted:
		return 2
	case BindingThin:
		return 3
	case BindingRepeated:
		return 4
	}
	return 0
}

// MaxRetainedPressSlots bounds how many recent slots' directions one session keeps.
//
// Sixty-four slots. A move is joined to the slot that opened its response window, which is at
// most TrackStaleness old and in practice the previous one or two — so this is far above what
// any join reaches, and it is a bound rather than a policy for the reason MaxInputLog is one: a
// session that ran for an hour must grow a counter rather than a slice.
const MaxRetainedPressSlots = 64

// pressSlot is the directions observed during one slot, with when Marco learned of them.
type pressSlot struct {
	// at is the SLOT'S OWN timestamp, which is the value `Navigated` opens the response
	// window with. The events' own AtMS deliberately does not appear: a look carries an
	// absolute time and an InputEvent carries milliseconds from its producer's own start,
	// and joining those two numbers is a recorded trap — see InterfaceWatch.navRecent.
	at time.Time
	// intents are the distinct directional intents observed in this slot, and conditional
	// which of them had every one of its events admitted only in context.
	intents     []NavIntent
	conditional map[NavIntent]bool
}

// pressLedger is the session's recent directional presses, in slot order.
//
// Session-scoped and written to nothing durable, like InputLog and for the same reason: this is
// evidence about one watched pass. Unexported with no reader outside this package, which is the
// structural half of the trust boundary — the interface watch holds no pointer to one and could
// not consult it if it wanted to.
type pressLedger struct {
	slots []pressSlot
	// dropped counts slots the bound pushed out, so a capped ledger never reads as complete.
	dropped int
}

// bank records one slot's directional presses.
//
// Called on the SAME line that opens the watch's response window, from the same gate, so the
// two accounts can never disagree about which presses existed. A slot that carried none is not
// banked: the ledger answers "which directions were observed then", and an empty slot would
// shadow the slot that actually opened the window.
func (p *pressLedger) bank(at time.Time, events []InputEvent) {
	slot := pressSlot{at: at}
	for _, e := range events {
		if !directional(e.Intent) {
			continue
		}
		if slot.conditional == nil {
			slot.conditional = map[NavIntent]bool{}
		}
		seen := false
		for _, have := range slot.intents {
			if have == e.Intent {
				seen = true
				break
			}
		}
		if !seen {
			slot.intents = append(slot.intents, e.Intent)
			// The first event of an intent sets the starting claim and any later
			// unconditional one clears it: "conditional" has to mean EVERY press
			// behind this direction was a context-admitted key, or the weaker
			// evidence would be reported as though some of it were strong.
			slot.conditional[e.Intent] = e.Conditional
			continue
		}
		if !e.Conditional {
			slot.conditional[e.Intent] = false
		}
	}
	if len(slot.intents) == 0 {
		return
	}
	p.slots = append(p.slots, slot)
	if over := len(p.slots) - MaxRetainedPressSlots; over > 0 {
		p.dropped += over
		p.slots = append(p.slots[:0:0], p.slots[over:]...)
	}
}

// navAttribution is what the input record says about one move, and nothing more.
type navAttribution struct {
	// intent is the one direction attributable to this move, empty when none is.
	intent NavIntent
	// conditional says that direction rested entirely on context-admitted keys.
	conditional bool
	// ambiguous says the window held more than one direction, so no single one may be
	// named; unknown that the move was prompted and the press behind it is not recoverable.
	ambiguous bool
	unknown   bool
}

// attribute joins one move to the slot that opened its response window.
//
// # Why this is a search backwards in TIME and not a window recomputed here
//
// Because the window is the watch's policy, and a second copy of it would drift. What the watch
// records on the move is `Prompted` — a press WAS attributable — and the window it uses is
// refreshed by each directional slot, so the slot that made `Prompted` true is necessarily the
// most recent one at or before the moment the move was seen. Finding that slot reproduces the
// watch's decision without restating it, and the ceiling below is the same ceiling `window()`
// already clamps to rather than a new number.
//
// # The two refusals, and why each is a refusal rather than a guess
//
// A slot LATER than the move is never consulted, even when it is the one the watch's window was
// refreshed from — which does happen, because the fast reads for an interval are folded before
// that interval's own look. Attributing a move to a press observed after it is the input
// predicting the outcome, which is the one thing this whole arrangement exists to prevent, so
// the honest answer is `unknown`.
//
// A move with no clock is refused too, by the same line. Without a timestamp there is no order,
// and "the last slot" would mean "any slot" — which on a fixture would quietly credit a
// direction banked long after the mark moved. Such a session reports every prompted observation
// as `direction_unknown`, which is what TestASessionWithNoClockNamesNoDirectionAndSaysSo asserts.
//
// # Which moment the move is dated from
//
// When the PIXELS moved, where the watch could say — SelectionMove.FirstSeen — and only its
// confirmation when nothing could. The same moment the watch now decides Prompted at, which is
// what keeps the two in lockstep: dating the direction from the confirmation while Prompted was
// decided from the sighting would let a move arrive prompted with no slot at or before it to
// attribute, and the three counters would stop partitioning Prompted.
//
// Deleting either refusal must fail TestAPressObservedAfterAMoveIsNotItsDirection.
func (p *pressLedger) attribute(m SelectionMove) navAttribution {
	if !m.Prompted {
		// The watch found nothing attributable. The move is Unprompted and already
		// counted as such, and a direction here would be evidence the watch refused.
		return navAttribution{}
	}
	// THE MOMENT THE MOVE WAS MADE, and only its confirmation when nothing dated it.
	//
	// The same moment Prompted is now decided at, which is what keeps the two in lockstep:
	// the slot that made Prompted true is the most recent directional slot at or before
	// this instant, so scanning to this instant reproduces the watch's decision instead of
	// restating its policy. Dating the direction from the confirmation while Prompted was
	// decided from the sighting would let a move be prompted with no slot to attribute it
	// to, and the three counters would stop partitioning Prompted.
	when := m.At
	if !m.FirstSeen.IsZero() {
		when = m.FirstSeen
	}
	best := -1
	var bestAt time.Time
	for i := range p.slots {
		s := p.slots[i]
		// EITHER SIDE MISSING A CLOCK IS A REFUSAL, and one check does both. Without a
		// timestamp there is no order, so "the last slot" would mean "any slot" — and a
		// clockless MOVE is refused by this same line, because a zero time is never at
		// or before a real one and a zero slot is skipped outright. An earlier version
		// guarded the move separately; the mutation gate showed that deleting the guard
		// changed no outcome, which makes it a second answer to a settled question.
		//
		// `when` and not m.At, so a move the pixels dated is refused on the moment it
		// was MADE. The refusal is unchanged by that: a clockless move has a zero
		// FirstSeen too, so `when` falls back to a zero At and this line still refuses.
		if s.at.IsZero() || when.IsZero() || s.at.After(when) {
			continue
		}
		// The CEILING `window()` clamps to, not a second latency policy. Past it a
		// press stops licensing anything: "something was pressed in the last three
		// seconds and the mark is now elsewhere" is a coincidence with a long name.
		if when.Sub(s.at) > TrackStaleness {
			continue
		}
		// THE LATEST QUALIFYING SLOT BY TIME, not the last one appended. Slots arrive in
		// fold order and that is normally time order, but the fast reads for an interval
		// are folded before that interval's own look — so relying on arrival order would
		// make the join depend on the cadence rather than on the clock.
		if best >= 0 && s.at.Before(bestAt) {
			continue
		}
		best, bestAt = i, s.at
	}
	if best < 0 {
		return navAttribution{unknown: true}
	}
	slot := p.slots[best]
	if len(slot.intents) != 1 {
		// Several directions in one window. NOT a vote and not the first one: a person
		// pressing down and right has not told Marco which moved the mark, and naming
		// one of them would be inventing the half that was not observed.
		return navAttribution{ambiguous: true}
	}
	return navAttribution{
		intent: slot.intents[0], conditional: slot.conditional[slot.intents[0]],
	}
}

// LedgerCaseForTest is one slot of presses and one move, for the tests that assert the JOIN
// rather than a session that happens to exercise it.
//
// Exported for the reason AdmissibleInputsForTest is: the refusals in `attribute` are the whole
// safeguard, and two of them — a slot later than the move, and a move with no clock — cannot be
// arranged in pixels without arranging a race. A rule that is only ever exercised incidentally
// is a rule nothing holds.
type LedgerCaseForTest struct {
	// PressAt and MoveAt are the slot's timestamp and the moment the move was seen.
	PressAt time.Time
	MoveAt  time.Time
	// Prompted is what the watch decided from the shading and its own window, which this
	// layer may narrow and may never widen.
	Prompted bool
	// Intents are the slot's events, and Soft marks the ones admitted only in context,
	// positionally. A shorter Soft means the rest were unambiguous.
	Intents []NavIntent
	Soft    []bool
}

// NavAttributionForTest is what the join decided, in exported form.
type NavAttributionForTest struct {
	Intent      NavIntent
	Conditional bool
	Ambiguous   bool
	Unknown     bool
}

// Run banks the slot and attributes the move.
func (c LedgerCaseForTest) Run() NavAttributionForTest {
	events := make([]InputEvent, 0, len(c.Intents))
	for i, in := range c.Intents {
		e := InputEvent{Intent: in, AtMS: int64(i) * 10}
		if i < len(c.Soft) {
			e.Conditional = c.Soft[i]
		}
		events = append(events, e)
	}
	var l pressLedger
	l.bank(c.PressAt, events)
	got := l.attribute(SelectionMove{
		Where:   Region{X: 0.1, Y: 0.1, Width: 0.3, Height: 0.3},
		Entries: 6, From: 1, To: 2,
		Prompted: c.Prompted, At: c.MoveAt, Sample: 1,
	})
	return NavAttributionForTest{
		Intent: got.intent, Conditional: got.conditional,
		Ambiguous: got.ambiguous, Unknown: got.unknown,
	}
}

// FoldDirectionsForTest exposes the durable merge, which is where the direction vocabulary is
// actually enforced on the way into the store. Exported for the test that asserts the
// enforcement rather than the declaration: a typed field whose words are never checked is a
// convention.
func FoldDirectionsForTest(into, from []DirectionCount) ([]DirectionCount, int) {
	return foldDirections(into, from)
}
