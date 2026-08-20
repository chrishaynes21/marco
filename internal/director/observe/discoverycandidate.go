package observe

// Building the demonstration record from the pass that WATCHED the demonstration.
//
// # The problem this removes
//
// The demonstration capture is armed for a NAMED relationship A → B. B is not known until the user
// has performed the route once, so Learn ran the route twice: a discovery pass to find out
// where it goes, and then an armed capture to watch it properly. Two performances of the same
// thing, and the second one is the one that keeps failing — because it is the one that has to
// independently re-confirm where the person is standing before it will start recording.
//
// Live, that produced the exact wrong outcome twice: Marco had the route, with the person's
// navigation attributed to it, and could not learn it because the extra ritual did not complete.
//
// # Why the discovery pass is enough
//
// It already recorded everything a candidate is made of. `ScreenTransition.Sequences` exists
// precisely for this, and says so:
//
//	"Reconstructing a procedure — move the selection twice, then confirm — needs the order."
//
// So this constructs the ORDINARY [ProcedureCandidate] from the ordinary evidence. It invents no
// new record, relaxes no threshold, and produces something the ordinary assessment then judges by
// the ordinary rules. What it removes is a second performance, not a check.
//
// # What it will not do
//
// It refuses, in a closed vocabulary, rather than guessing — and every refusal falls back to the
// armed capture, which is still there and still correct. In particular it will not choose between
// two different orders that were both observed: an interaction seen two ways is not a procedure.

// DiscoveryRefusal is the CLOSED vocabulary of why a one-shot candidate could not be built.
type DiscoveryRefusal string

const (
	// DiscoveryNoTransition — no observed change from the start to the destination. The route
	// may be real and multi-step; this path only reads a single change.
	DiscoveryNoTransition DiscoveryRefusal = "no_direct_transition"
	// DiscoveryNoNavigation — the change was seen with no attributed navigation before it.
	// Knowing where somebody ended up is not knowing what they did.
	DiscoveryNoNavigation DiscoveryRefusal = "no_attributed_navigation"
	// DiscoveryOrderAmbiguous — more than one distinct ORDER was seen for the same change.
	// `down, down, confirm` and `confirm` are two interactions, and picking one would be
	// choosing what the person meant.
	DiscoveryOrderAmbiguous DiscoveryRefusal = "order_ambiguous"
	// DiscoveryEndpointUnresolved — an endpoint does not currently resolve to the durable
	// subject the route names. Recognisability is not relaxed here any more than anywhere.
	DiscoveryEndpointUnresolved DiscoveryRefusal = "endpoint_unresolved"
	// DiscoveryAmbiguousRoute — more than one route became durable in the same pass, so
	// which one the person meant is not something this evidence says. The armed capture is
	// aimed at a NAMED route and is the right mechanism for that case.
	DiscoveryAmbiguousRoute DiscoveryRefusal = "ambiguous_route"
	// DiscoveryNotStored — the candidate was built and the store would not keep it. Reported
	// because everything downstream reads the store, so an unstored candidate is invisible.
	DiscoveryNotStored DiscoveryRefusal = "not_stored"
	// DiscoveryNoMemory — nothing to resolve endpoints against.
	DiscoveryNoMemory DiscoveryRefusal = "no_memory"
)

// DiscoveredDemonstration is the candidate the discovery pass supports, or why it supports none.
type DiscoveredDemonstration struct {
	Candidate ProcedureCandidate
	Built     bool
	Refusal   DiscoveryRefusal
	// Bridged records that the change crossed a sample nobody could place, and was recovered
	// under the same rule the relationship layer uses. Carried so the claim stays visible.
	Bridged bool
	// EndsAtCurrent says this edge's destination is the screen the session currently shows —
	// the TERMINAL leg of the demonstration, the one that arrived where the person stopped.
	// A multi-leg demonstration decomposes into one candidate per edge, and this is how the
	// caller tells the leg that reached the outcome from the legs along the way.
	EndsAtCurrent bool
}

// CandidateFromDiscovery builds the demonstration record from a pass that merely watched.
//
// The route's endpoints are resolved from CURRENT evidence, exactly as the capture resolves them —
// a subject the request names is a claim about history, and whether this screen is that place is a
// question only the evidence answers.
func CandidateFromDiscovery(t ShadowTotals, route RelationshipRef, application string,
	m Memory, th HypothesisThresholds) DiscoveredDemonstration {

	if m == nil {
		return DiscoveredDemonstration{Refusal: DiscoveryNoMemory}
	}
	// The route names two durable subjects; which of this session's states ARE those places
	// is a question only current evidence answers, and it is answered here rather than passed
	// in. A caller that supplied the state ids would be a second opinion about identity.
	from, start, okFrom := stateOf(t, application, route.From, m, th)
	to, arrived, okTo := stateOf(t, application, route.To, m, th)
	if !okFrom || !okTo {
		return DiscoveredDemonstration{Refusal: DiscoveryEndpointUnresolved}
	}

	leg, bridged, why := legFor(t, from, to)
	if why != "" {
		return DiscoveredDemonstration{Refusal: why}
	}

	step := DemonstrationStep{
		Intents: append([]NavIntent{}, leg.Intents...),
		Arrived: arrived,
		// The whole leg came from one transition's ordered run, so any conditional doubt
		// is the transition's own.
		Conditional: leg.Conditional,
	}
	if len(leg.Targets) > 0 {
		step.Targets = append([]SemanticTarget{}, leg.Targets...)
	}
	// The text-entry boundary, through the SAME rule the capture uses — one function, so the
	// two paths cannot come to different answers about the same screen. Arriving at the
	// destination is exempt; arriving anywhere else on the way is not.
	step.RequiresTextEntry = textEntryBlocks(arrived, route.To)

	return DiscoveredDemonstration{
		Built:         true,
		Bridged:       bridged,
		EndsAtCurrent: to == t.CurrentState,
		Candidate: ProcedureCandidate{
			Relationship: route, Application: application,
			Start: start, Steps: []DemonstrationStep{step},
			Complete: true, Reason: ReasonArrived,
			Events: len(step.Intents), Checkpoints: 2,
			// One watched change. Inferences is what it cost to watch, and the honest
			// figure for a constructed record is the count of the change itself.
			Inferences: leg.Count, Sequence: 1,
		},
	}
}

// discoveryLeg is one observed change and the ordered navigation before it.
type discoveryLeg struct {
	Intents     []NavIntent
	Targets     []SemanticTarget
	Count       int
	Conditional bool
}

// legFor finds the single change from one screen to another, directly or across one unplaceable
// sample, and refuses when the evidence does not say one thing.
func legFor(t ShadowTotals, from, to ScreenStateID) (discoveryLeg, bool, DiscoveryRefusal) {
	for i := range t.Transitions {
		tr := &t.Transitions[i]
		if tr.From != from || tr.To != to {
			continue
		}
		leg, why := legOf(tr)
		return leg, false, why
	}

	// Not direct. The change may have crossed a sample nobody could place — the live case —
	// and the rule for reading that interval is the relationship layer's, not a second one
	// invented here. The intents ride on the ENTRY leg, which is where the navigation was seen.
	intervals, ok := unsettledIntervals(t)
	if !ok {
		return discoveryLeg{}, false, DiscoveryNoTransition
	}
	for _, in := range intervals {
		if in.From != from || in.To != to || in.Entry == nil {
			continue
		}
		if in.Run < 1 || in.Run >= StatePromotionCount {
			// Same bound, same reason: shorter than promotion is a transition frame by
			// the architecture's own definition; longer may have hidden a screen. See
			// bridge.go.
			return discoveryLeg{}, false, DiscoveryNoTransition
		}
		leg, why := legOf(in.Entry)
		return leg, true, why
	}
	return discoveryLeg{}, false, DiscoveryNoTransition
}

// legOf reads one transition's ordered navigation, and insists there is exactly one order.
func legOf(tr *ScreenTransition) (discoveryLeg, DiscoveryRefusal) {
	switch {
	case len(tr.Sequences) == 0:
		return discoveryLeg{}, DiscoveryNoNavigation
	case len(tr.Sequences) > 1:
		// Two orders for one change. A capture would have watched one performance and
		// recorded one order; this is reading an aggregate, so the aggregate has to be
		// unambiguous before it may be read as a procedure.
		return discoveryLeg{}, DiscoveryOrderAmbiguous
	}
	seq := tr.Sequences[0]
	if len(seq.Intents) == 0 {
		return discoveryLeg{}, DiscoveryNoNavigation
	}
	return discoveryLeg{
		Intents: seq.Intents, Targets: seq.Targets, Count: tr.Count,
		Conditional: tr.ConditionalOnly > 0,
	}, ""
}

// stateOf finds which of this session's screens IS the named durable place.
//
// Resolved forwards — every state is asked what it is, and the answers are matched against the
// route — rather than backwards from a remembered signature. A screen is what current evidence
// says it is; looking it up by what memory holds would let a stale record decide where the user
// was standing, which is the stale-ordinal mistake in another costume.
func stateOf(t ShadowTotals, application, subject string, m Memory,
	th HypothesisThresholds) (ScreenStateID, Checkpoint, bool) {

	if subject == "" {
		return "", Checkpoint{}, false
	}
	for _, st := range t.States {
		cp, ok := placeOfState(t, application, st.ID, m, th)
		if ok && cp.Subject == subject {
			return st.ID, cp, true
		}
	}
	return "", Checkpoint{}, false
}

// placeOfState is PlaceNow for a state that is not the current one.
//
// The same derivation, deliberately: a checkpoint has to be resolved by the value the subject
// would have been stored under, and a second way of asking "what screen is this" would drift.
func placeOfState(t ShadowTotals, application string, id ScreenStateID, m Memory,
	th HypothesisThresholds) (Checkpoint, bool) {

	sig, ok := SignatureOfState(t, id, th)
	if !ok {
		return Checkpoint{}, false
	}
	rec := m.Recall(application, sig)
	cp := Checkpoint{
		Structure: sig, Verdict: rec.Verdict,
		EditableFields: EditableFieldsIn(t, id),
	}
	if !rec.Verdict.Established() {
		cp.Transient = true
		return cp, false
	}
	cp.Subject = rec.Subject.ID
	return cp, true
}
