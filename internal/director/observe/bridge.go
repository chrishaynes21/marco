package observe

// Recovering ONE adjacency from a change that crossed a sample Marco could not place.
//
// # The problem, measured
//
// A live teach attempt on Windows Settings pressed `confirm` once and moved between two screens
// Marco recognises. What the segmenter recorded was not one change but two:
//
//	state_1        → state_unknown   count 1, preceded {confirm: 1}
//	state_unknown  → state_2         count 1, unattributed 1
//
// `state_unknown` is not a screen, produces no hypothesis and can never be a durable subject, so
// BOTH halves resolved to nothing and the route the user had just demonstrated did not exist.
// Reproduced twice, byte-identically, on a nine-observation visit and on a sixty-seven-observation
// one — so it is a property of the sampling and not of how long anybody lingered.
//
// # What this is allowed to conclude, and what it must not
//
// It concludes exactly one thing: that a change from A to B was observed. It does NOT decide what
// the unplaceable sample was. Nothing here mints a state, names one, gives one identity, or lets
// `state_unknown` become a subject — the unplaced sample stays unplaced and stays unknown, which
// is the honest description of a frame nobody could read.
//
// # Why the bound is StatePromotionCount and not a number chosen here
//
// The segmenter already draws this exact line: "One sighting is a transition frame; a second is a
// screen." An unsettled run SHORTER than `StatePromotionCount` is, by the architecture's own
// definition, a transition frame — so bridging it asserts nothing that segmentation does not
// already assert. A run long enough to have been promoted might have contained a screen that
// simply never recurred, and skipping it would hide a real place. That is the difference between
// recovering a lost adjacency and inventing one.
//
// # Continuity is proven, never assumed
//
// Five conditions, and every one of them is read from evidence the session already keeps.

// BridgeRefusal is the CLOSED vocabulary of why an unsettled interval was not bridged.
type BridgeRefusal string

const (
	// BridgeNoUnsettled — nothing crossed an unplaceable sample. The ordinary case.
	BridgeNoUnsettled BridgeRefusal = "no_unsettled_interval"
	// BridgeAmbiguousInterval — the unplaced state was entered from, or left to, more than
	// one place, so which entry pairs with which exit is not something the evidence says.
	// This is also what stops A → ? → C → ? → B from skipping C.
	BridgeAmbiguousInterval BridgeRefusal = "ambiguous_interval"
	// BridgeIntervalTooLong — the run of unplaceable samples was long enough to have been a
	// screen, so something recognisable may have happened inside it.
	BridgeIntervalTooLong BridgeRefusal = "interval_too_long"
	// BridgeIntervalUnknown — how long the interval lasted was not recorded.
	//
	// An exit from the unplaced state crossed at least one sample by construction, so a run
	// of zero on that edge is missing evidence rather than a short gap. Refusing keeps the
	// bound meaningful: without the length there is nothing to bound.
	BridgeIntervalUnknown BridgeRefusal = "interval_unknown"
	// BridgeObservationBroken — the session did not hold one uninterrupted view of one
	// window across the interval.
	BridgeObservationBroken BridgeRefusal = "observation_broken"
	// BridgeEndpointUnresolved — one or both ends of the recovered change is not a durable
	// subject. The bridge recovers adjacency; it never relaxes recognisability.
	BridgeEndpointUnresolved BridgeRefusal = "endpoint_unresolved"
	// BridgeSameSubject — the change left A and came back to A. Bridging it would
	// manufacture a self-loop, which says nothing.
	BridgeSameSubject BridgeRefusal = "same_subject"
	// BridgeNoEntry and BridgeNoExit — a session that BEGAN unplaced has no source to
	// recover, and one that ENDED unplaced has no destination. Neither may be invented.
	BridgeNoEntry BridgeRefusal = "no_entry"
	BridgeNoExit  BridgeRefusal = "no_exit"
)

// Continuity is what a session knows about whether its observations describe one uninterrupted
// view of one window.
//
// Supplied by the runner from figures it already keeps. Held as a value rather than read from
// somewhere here, because whether a session's view was continuous is a fact about the SESSION and
// this package has no way to observe one.
type Continuity struct {
	// Generations is how many distinct window generations the session spanned. More than one
	// means the window was replaced part-way through.
	Generations int
	// TargetLosses is how many times the window went away during the session.
	TargetLosses int
}

// Unbroken reports whether the session held one view of one window throughout.
//
// Conservative: an unsettled interval that overlaps a window replacement or a target loss may be
// hiding an entire application restart, and no bound on its LENGTH would make bridging it honest.
func (c Continuity) Unbroken() bool { return c.Generations <= 1 && c.TargetLosses == 0 }

// bridgeUnsettled recovers one A→B adjacency from A → [unplaceable] → B, or says why not.
//
// Returns the observation and true only when every condition holds. The refusal is recorded on the
// report rather than returned, because a caller has nothing to do about it and a reader does.
func bridgeUnsettled(t ShadowTotals, resolve func(ScreenStateID) (string, bool),
	c Continuity, report *RelationshipReport) []RelationshipObservation {

	intervals, ok := unsettledIntervals(t)
	if !ok {
		// The walk was truncated, so which entry belongs with which exit is a guess.
		report.noteBridge(BridgeAmbiguousInterval)
		return nil
	}
	// A session that BEGAN or ENDED unplaced has a half-interval the walk never closes, and
	// neither end may be invented: there is no source to recover for an arrival, and no
	// destination for a departure. Counted here rather than inside the loop, because they are
	// facts about the walk and not about any one excursion.
	if len(t.Crossings) > 0 {
		if first := t.Crossings[0]; first.From == ScreenStateUnknown && !closedBefore(t, 0) {
			report.noteBridge(BridgeNoEntry)
		}
		if last := t.Crossings[len(t.Crossings)-1]; last.To == ScreenStateUnknown {
			report.noteBridge(BridgeNoExit)
		}
	}
	var out []RelationshipObservation
	for _, in := range intervals {
		if b, ok := bridgeOne(in, resolve, c, report); ok {
			out = append(out, b)
		}
	}
	return out
}

// closedBefore reports whether the crossing at i is the exit of an interval this walk opened.
func closedBefore(t ShadowTotals, i int) bool {
	for j := i - 1; j >= 0; j-- {
		if t.Crossings[j].To == ScreenStateUnknown {
			return true
		}
	}
	return false
}

// bridgeOne recovers the adjacency across ONE excursion, or records why it did not.
func bridgeOne(in unsettledInterval, resolve func(ScreenStateID) (string, bool),
	c Continuity, report *RelationshipReport) (RelationshipObservation, bool) {

	entry, exit := in.Entry, in.Exit
	if entry == nil || exit == nil {
		// The walk names a leg the aggregate does not hold. Nothing to read the
		// navigation off, and inventing an empty one would claim the change was
		// unattributed when it was merely unrecorded.
		report.noteBridge(BridgeIntervalUnknown)
		return RelationshipObservation{}, false
	}
	// The run has to be RECORDED, and short enough that it cannot have been a screen. See the
	// note above: the bound is segmentation's own, not one invented here.
	switch {
	case in.Run < 1:
		report.noteBridge(BridgeIntervalUnknown)
		return RelationshipObservation{}, false
	case in.Run >= StatePromotionCount:
		report.noteBridge(BridgeIntervalTooLong)
		return RelationshipObservation{}, false
	}
	if !c.Unbroken() {
		report.noteBridge(BridgeObservationBroken)
		return RelationshipObservation{}, false
	}

	from, okFrom := resolve(in.From)
	to, okTo := resolve(in.To)
	if !okFrom || !okTo {
		// Recognisability is NOT relaxed. A bridge recovers an adjacency between two places
		// Marco can find again, or it recovers nothing.
		report.noteBridge(BridgeEndpointUnresolved)
		return RelationshipObservation{}, false
	}
	if from == to {
		// Left and came back. A self-loop says nothing, and the direct path already
		// refuses one for the same reason.
		report.noteBridge(BridgeSameSubject)
		return RelationshipObservation{}, false
	}

	// The evidence comes from the ENTRY leg, and that is the whole point: the navigation the
	// user performed is recorded against the change INTO the unplaced sample, because that is
	// the observation that first saw the screen move. The exit leg carries the arrival and, in
	// every case measured, no intent at all.
	//
	// Observations is the smaller of the two counts. A traversal is only evidence of a
	// completed change when both halves were seen.
	crossings := entry.Count
	if exit.Count < crossings {
		crossings = exit.Count
	}
	report.Bridged++
	return RelationshipObservation{
		From: from, To: to,
		Evidence: RelationshipEvidence{
			Observations: crossings,
			Preceded:     copyIntents(entry.Preceded),
			// The arrival was not attributed to anything, and that stays true of the
			// recovered edge. Folding the exit leg's unattributed count in would claim
			// the change was seen twice.
			Unattributed:    entry.Unattributed,
			ConditionalOnly: entry.ConditionalOnly,
			Sequences:       PlainSequences(entry.Sequences),
			// Recorded so the claim stays visible wherever the edge is read: this
			// adjacency was recovered across a sample nobody could place.
			Bridged: crossings,
		},
	}, true
}

// unsettledInterval is ONE excursion through samples nobody could place: where it left from,
// where it came back to, and how long the gap was.
type unsettledInterval struct {
	// From and To are the placed screens either side of the gap.
	From, To ScreenStateID
	// Run is the length of THIS gap, not the longest gap the edge ever crossed.
	Run int
	// Entry and Exit are the aggregated legs, which is where the evidence lives. The
	// navigation the user performed rides on the ENTRY leg, because that is the observation
	// that first saw the screen move.
	Entry, Exit *ScreenTransition
}

// unsettledIntervals is every A → [unplaceable] → B excursion the session walked, in order.
//
// # Why this reads the walk and not the transition map
//
// It used to find "the single entry and the single exit" and refuse the moment there were two of
// either, because the transition map is keyed by the pair and genuinely cannot say which entry
// belongs with which exit. That refusal is correct given only counts, and it made every
// multi-step demonstration unlearnable at human speed.
//
// Measured, on a cold store, from a person demonstrating `Settings Home → Bluetooth & devices →
// Mouse` — the walk Learn exists to capture:
//
//	state_1       → state_unknown   count 1
//	state_unknown → state_2         count 1, unattributed 1
//	state_2       → state_unknown   count 1
//	state_unknown → state_3         count 1, unattributed 1
//
// Two entries, two exits, `ambiguous_interval`, and BOTH real adjacencies lost — after all three
// screens had been recognised, settled and made durable. Every navigation at ordinary speed
// crosses a transition frame, so this was not an edge case: it was the common case, and it made
// the one-shot Learn the whole roadmap is about impossible except at one step per session.
//
// The order was never unavailable. It was discarded by the fold. `ShadowTotals.Crossings` keeps
// it, and with it each excursion has exactly one entry and one exit by construction — which is
// also what stops `A → ? → C → ? → B` from collapsing to `A → B`: the walk says C is in the
// middle, so it yields A→C and C→B.
//
// # What it still refuses
//
// A truncated walk. A pairing across a dropped crossing would be a guess, so `ok` is false when
// the bound evicted anything and every caller refuses outright.
//
// Shared with the one-shot candidate path, which has to read the same intervals for the same
// reason: a second derivation of "which transition carries the intents" would eventually give a
// second answer.
func unsettledIntervals(t ShadowTotals) (out []unsettledInterval, ok bool) {
	if t.EvictedCrossings > 0 {
		return nil, false
	}
	leg := func(from, to ScreenStateID) *ScreenTransition {
		for i := range t.Transitions {
			if t.Transitions[i].From == from && t.Transitions[i].To == to {
				return &t.Transitions[i]
			}
		}
		return nil
	}
	for i := 0; i < len(t.Crossings); i++ {
		c := t.Crossings[i]
		if c.To != ScreenStateUnknown || c.From == ScreenStateUnknown {
			continue
		}
		// The exit is the next crossing that leaves the unplaced state. Anything between
		// is unknown → unknown, which `note` never records, so in practice the exit is the
		// very next crossing — scanned rather than assumed so a future producer that does
		// record one cannot silently pair the wrong pair.
		for j := i + 1; j < len(t.Crossings); j++ {
			e := t.Crossings[j]
			if e.From != ScreenStateUnknown {
				break
			}
			if e.To == ScreenStateUnknown {
				continue
			}
			out = append(out, unsettledInterval{
				From: c.From, To: e.To, Run: e.Run,
				Entry: leg(c.From, ScreenStateUnknown),
				Exit:  leg(ScreenStateUnknown, e.To),
			})
			i = j
			break
		}
	}
	return out, true
}

// noteBridge records why a bridge was not taken. Bounded by the vocabulary above.
func (r *RelationshipReport) noteBridge(why BridgeRefusal) {
	if r.BridgeRefusals == nil {
		r.BridgeRefusals = map[BridgeRefusal]int{}
	}
	r.BridgeRefusals[why]++
}
