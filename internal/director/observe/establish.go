package observe

import "sort"

// Making a PLACE durably recognisable, without claiming to know what it is.
//
// # The two claims this file keeps apart
//
// "I have seen this stable, discriminating screen and can recognise it again" is an OBSERVATIONAL
// claim: Marco makes it out of its own evidence and can be wrong about it in a way a person can
// see and correct. "These controls are one set of choices" is a HUMAN claim: only somebody
// answering a question settles it, and nothing here can produce one.
//
// The store already separates them — a subject is a structure with a list of interpretations, and
// the list may be empty. A `not now` answer proved it: it mints a subject and asserts nothing.
// What did not exist was a way to reach that path without a person happening to answer an
// incidental question first.
//
// # Why this is not "remember every screen"
//
// It is licensed per SESSION and by one caller. `learn "…"` is itself an explicit human semantic
// event: a person has named a behaviour and asked to be watched doing it. That licenses persisting
// the IDENTITY of where they are standing — and persists no judgement about it, because they were
// not asked one and Marco has no business inventing one.
//
// Passive observation is unchanged. A Director watching somebody play still persists nothing, and
// the bound on the store is untouched: at most one place per bounded pass, only where the evidence
// could ever be matched again, and refused outright once the store is full.

// PlaceRefusal is the CLOSED vocabulary of why a session established no place.
//
// Silence is the hard case, as it is everywhere else in this subsystem. "Marco did not remember
// where you were standing" has half a dozen explanations and a reader cannot otherwise tell a
// missing licence from a screen that carries nothing to recognise it by.
type PlaceRefusal string

const (
	// PlaceNotLicensed is an ordinary session. The default, and by far the common case.
	PlaceNotLicensed PlaceRefusal = "not_licensed"
	// PlaceNoMemory is a Director with nowhere durable to put one.
	PlaceNoMemory PlaceRefusal = "no_memory"
	// PlaceNotDescribable is a screen the segmenter has not settled — a transition frame, or
	// a state that has not been seen enough to be described at all. Distinct from every
	// "described and not worth storing" below.
	PlaceNotDescribable PlaceRefusal = "not_describable"
	// PlaceNotDiscriminating is a screen whose signature carries nothing that could ever match
	// it again: no interface concepts were read and it has no envelope. Storing it would add a
	// record nothing can ever resolve, which is the unbounded growth the store refuses.
	PlaceNotDiscriminating PlaceRefusal = "not_discriminating"
	// PlaceNotSettled is a screen still changing shape. Visible, describable, and not yet
	// anything worth remembering — see settledWhole. The ordinary answer early in a
	// pass, and not a failure.
	PlaceNotSettled PlaceRefusal = "not_settled"
	// PlaceStillLoading is a screen that is telling you it has not finished.
	//
	// Distinct from PlaceNotSettled, which is about a composition still CHANGING. This one
	// has stopped changing and is still not done: Windows Settings holds a progress bar
	// steady for long enough to settle, so the maturity gate passes and a half-drawn page
	// becomes a durable Place. Three Home twins in one live store came from exactly that,
	// one of them carrying the progress bar in its durable signature.
	PlaceStillLoading PlaceRefusal = "still_loading"
	// PlaceAlreadyKnown is a place memory already recognises. Nothing to do, and not a failure.
	PlaceAlreadyKnown PlaceRefusal = "already_known"
	// PlaceNotWritten is the store declining — the subject bound, or an unreadable file.
	PlaceNotWritten PlaceRefusal = "not_written"
)

// PlaceStore persists a place's IDENTITY and can do nothing else.
//
// Narrower than Memory on purpose, and narrower than KnowledgeStore in the direction that matters:
// there is no SemanticKnowledge in this interface at all, so a caller holding one cannot record a
// judgement, cannot revise one, and cannot reach a relationship, a demonstration or any authority.
// Establishing a place is the only thing it can do.
type PlaceStore interface {
	// EstablishPlace persists a subject for this signature, asserting NOTHING about what it
	// means, and returns the subject's id.
	//
	// Idempotent: a signature the store already holds as the same subject is returned
	// unchanged, with its existing interpretations untouched. It never creates, alters or
	// removes a judgement.
	EstablishPlace(application string, sig StructureSignature) (string, error)
}

// PlaceEstablishment is what one session did about making where the user is standing recognisable.
//
// Carried on the terminal Result rather than logged, so a learn attempt that could not establish a
// start can say WHY in the same breath as it refuses.
type PlaceEstablishment struct {
	// Licensed says this session was allowed to establish a place at all.
	Licensed bool `json:"licensed,omitempty"`
	// Subject is the durable subject the place the user ENDED on became, empty when it
	// did not. Unchanged in meaning since this type existed.
	Subject string `json:"subject,omitempty"`
	// Reason is the closed reason that place was not established, empty when it was.
	Reason PlaceRefusal `json:"reason,omitempty"`
	// Also is every OTHER place this pass settled on that qualified, in the order they
	// were visited.
	//
	// A route with a middle step has one, and before this existed it had nowhere to go —
	// see PlacesToEstablish for the failure that produced it.
	Also []string `json:"also,omitempty"`
}

// Established reports whether a place became durably recognisable.
func (p PlaceEstablishment) Established() bool { return p.Subject != "" }

// PlaceCandidate is one place a pass settled on, with the signature it would be stored under.
type PlaceCandidate struct {
	State     ScreenStateID
	Signature StructureSignature
	// SemanticName is what this screen appeared to be called, empty when the evidence did not
	// say so unambiguously. Presentation only: it never participates in identity, and two
	// Places inferring the same word remain two Places.
	SemanticName string
	// Current says this is the place the pass ENDED on.
	Current bool
}

// PlacesToEstablish is every place this pass settled on that may be made durable.
//
// # The failure this exists for
//
// Establishment used to consider exactly one state: the one the user was standing on when the
// pass ended. That is right for a pass that watches somebody arrive somewhere, and wrong for
// the thing Learn actually watches — a person walking THROUGH places to reach one.
//
// Measured live on 2026-08-17, on a cold store, with a person demonstrating
// `Settings Home → Bluetooth & devices → Mouse`:
//
//	state_1 Home       7 inferences   settled   terms[back settings]
//	state_2 Bluetooth  3 inferences   settled   terms[back settings]
//	state_3 Mouse      3 inferences   settled   terms[back controls settings]
//
// Every one of them passed every quality gate. Only Mouse was established, so NEITHER edge
// had two durable endpoints — `Home → Bluetooth` was `destination_unresolved` and
// `Bluetooth → Mouse` was `source_unresolved`, and the demonstration could not be learned
// despite being captured, attributed and semantically resolved end to end.
//
// It also made [[ADR-056-a-goal-is-a-destination-not-a-route]]'s decomposition unreachable in
// practice: `A → B → C` can only contribute two reusable edges if B is durable.
//
// # Why widening the bound is safe, and what still bounds it
//
// [[ADR-047-a-place-is-remembered-a-meaning-is-answered]] capped this at one place per pass to
// avoid "the unbounded persistence this mechanism is careful not to be" — specifically, to
// avoid establishing every transition frame somebody walked through. That concern is real and
// is answered by the GATES rather than by the count: a transition frame is not settled, and a
// screen with nothing distinctive is not discriminating. Each candidate here passes exactly
// the checks the single one always had to pass, independently.
//
// What remains bounded: the licence itself (learn passes only), the per-place gates, the
// store's own MaxSubjects, and MaxCheckpoints — a pass that settled on more places than a
// demonstration may have checkpoints is not a route, it is a tour, and the same number that
// says so for the capture says so here (DefaultCaptureBounds().MaxCheckpoints).
//
// Deterministic order: first sighting, so a walk is established in the order it was walked and
// two identical sessions write identical files.
func PlacesToEstablish(t ShadowTotals, application string, m Memory,
	th HypothesisThresholds) ([]PlaceCandidate, PlaceRefusal) {

	// The current place keeps its own answer, unchanged, so every existing reader and
	// every existing refusal reason means exactly what it did.
	currentSig, currentRefusal := PlaceToEstablish(t, application, m, th)

	var out []PlaceCandidate
	states := append([]ScreenState{}, t.States...)
	sort.Slice(states, func(i, j int) bool {
		return states[i].FirstInference < states[j].FirstInference
	})
	for _, st := range states {
		if len(out) >= DefaultCaptureBounds().MaxCheckpoints {
			break
		}
		if st.ID == t.CurrentState {
			if currentRefusal == "" {
				out = append(out, PlaceCandidate{
					State: st.ID, Signature: currentSig, Current: true,
					SemanticName: settledPlaceName(st),
				})
			}
			continue
		}
		sig, refusal := placeToEstablishAt(t, application, st.ID, m, th)
		if refusal != "" {
			continue
		}
		out = append(out, PlaceCandidate{State: st.ID, Signature: sig,
			SemanticName: settledPlaceName(st)})
	}
	return out, currentRefusal
}

// placeToEstablishAt is PlaceToEstablish for a state that is not the current one.
//
// The SAME gates in the same order, and deliberately not a relaxed variant: a place the user
// merely passed through has to clear exactly what the place they stopped on clears.
func placeToEstablishAt(t ShadowTotals, application string, id ScreenStateID, m Memory,
	th HypothesisThresholds) (StructureSignature, PlaceRefusal) {

	if m == nil {
		return StructureSignature{}, PlaceNoMemory
	}
	sig, ok := SignatureOfState(t, id, th)
	if !ok {
		return StructureSignature{}, PlaceNotDescribable
	}
	if !stateSettled(t, id) {
		return StructureSignature{}, PlaceNotSettled
	}
	if stillLoading(t, id, th) {
		return StructureSignature{}, PlaceStillLoading
	}
	if !sig.Discriminating() {
		return StructureSignature{}, PlaceNotDiscriminating
	}
	if m.Recall(application, sig).Verdict.Established() {
		return StructureSignature{}, PlaceAlreadyKnown
	}
	return sig, ""
}

// PlaceToEstablish decides whether the place the user is standing on may be made durable, and
// returns the signature it would be stored under.
//
// # The chain is the canonical one and there is no other route through it
//
// `SignatureOfState` is the same derivation `PlaceNow` resolves against and the same value a
// subject would be stored under. Deriving it a second way here would be a second answer to "what
// screen is this", and this repository has already paid for one of those.
//
// A refusal is returned rather than a bare false, because every one of them means something
// different to a person trying to show Marco something.
func PlaceToEstablish(t ShadowTotals, application string, m Memory,
	th HypothesisThresholds) (StructureSignature, PlaceRefusal) {

	if m == nil {
		return StructureSignature{}, PlaceNoMemory
	}
	sig, ok := SignatureOfState(t, t.CurrentState, th)
	if !ok {
		return StructureSignature{}, PlaceNotDescribable
	}
	// SETTLED, before anything durable rests on it.
	//
	// Marco may SEE a place long before it knows enough to remember one. Windows Settings
	// renders in stages, and the start of a learn was being fingerprinted while the page was
	// still arriving — a composition of ten members that held twenty-two by the end of the
	// same pass. The destination, established from a long pass, reproduced the same durable
	// subject across four independent cold stores; the start minted a new one nearly every
	// time. Identical code path, identical producer, different maturity of evidence.
	//
	// Both endpoints pass through here, so the rule lands on both at once — which is the
	// reason to put it here rather than in either caller.
	//
	// Not a dwell. See settledWhole: a screen that was already stable settles on its
	// second observation, and nothing waits for a clock.
	if !stateSettled(t, t.CurrentState) {
		return StructureSignature{}, PlaceNotSettled
	}
	// STOPPED CHANGING IS NOT THE SAME AS FINISHED. See stillLoading: a page holding a
	// progress bar steady settles, and a half-drawn page is not a place.
	if stillLoading(t, t.CurrentState, th) {
		return StructureSignature{}, PlaceStillLoading
	}
	// The store applies this rule too and would refuse. It is applied HERE as well so the
	// reason survives: a store error reads as "something went wrong", and this is not a
	// failure — it is Marco saying it could never recognise this screen again, which is a
	// sentence a person can act on by holding still somewhere more distinctive.
	if !sig.Discriminating() {
		return StructureSignature{}, PlaceNotDiscriminating
	}
	// ALREADY KNOWN is checked through Recall — the identity layer — rather than by looking
	// for a matching record. The question is not "is this signature in the file", it is "would
	// Marco recognise this place", and only the matcher answers that.
	if m.Recall(application, sig).Verdict.Established() {
		return StructureSignature{}, PlaceAlreadyKnown
	}
	return sig, ""
}

// stateSettled reports whether one of this session's screens has stopped changing shape.
//
// Reads the segmenter's own conclusion rather than recomputing one. The snapshot carries it
// because the tally it is derived from is session-local and does not survive States().
func stateSettled(t ShadowTotals, id ScreenStateID) bool {
	for _, st := range t.States {
		if st.ID == id {
			return st.Settled
		}
	}
	return false
}

// ── a screen that says it is still loading ────────────────────────────────────

// loadingRoles are the controls whose presence says a page has not finished arriving.
//
// One entry, because one is what has been measured. A progress bar exists to say "wait"; a
// composition containing one is a page mid-arrival, and the pages this system has watched put
// theirs up during navigation and take it down when the content lands.
//
// This is NOT a claim that a progress bar can never be part of a screen. See stillLoading: a
// composition that keeps coming back with one is content, and is allowed to become a place.
var loadingRoles = map[string]bool{"progress_bar": true}

// stillLoading reports whether this state is a page caught mid-arrival.
//
// # Why recurrence is the discriminator, and not a timer
//
// A loading page and a page whose content legitimately includes a progress control look the same
// in one visit. They differ in what happens next: the loading one is passed through once on the
// way somewhere, and the real one comes back every time you return to it.
//
// So this reuses the recurrence rule the hypothesis layer already states — "the screen was seen
// in only one visit; a transition frame and a screen are indistinguishable until one of them
// comes back" — rather than inventing a clock. A composition carrying a load indicator has to
// recur across visits before it may define a place. A stable page with a progress bar therefore
// establishes on its second visit; a page caught arriving never does, because nobody returns to
// a moment.
//
// Deleting this must fail TestAPageStillLoadingDoesNotBecomeAPlace.
func stillLoading(t ShadowTotals, id ScreenStateID, th HypothesisThresholds) bool {
	for _, st := range t.States {
		if st.ID != id {
			continue
		}
		loading := false
		for role, n := range st.Roles {
			if n > 0 && loadingRoles[role] {
				loading = true
				break
			}
		}
		if !loading {
			return false
		}
		// Seen again on a separate visit: content, not a moment.
		return st.Episodes < th.MinEpisodes
	}
	return false
}

// settledPlaceName is the word a screen recurred under, or nothing.
//
// The same shape as the composition rule in [[ADR-073]] and for the same reason: a transition
// frame can carry the name of the page being LEFT, so the word that recurs is the screen's and a
// word seen once is a frame. Most sightings win; a tie is left unresolved, because deciding one by
// map order would make a Place's name depend on how a hash table happened to walk.
//
// Deleting the recurrence requirement must fail TestANameSeenOnceDoesNotStick.
func settledPlaceName(st ScreenState) string {
	best, top, tied := "", 0, false
	for name, seen := range st.PlaceNames {
		switch {
		case seen > top:
			best, top, tied = name, seen, false
		case seen == top && name != best:
			tied = true
		}
	}
	if tied || top < StatePromotionCount {
		return ""
	}
	return best
}

// PlaceNamer is a store that can record what a Place APPEARS to be called.
//
// Optional and separate from PlaceStore on purpose: establishing a Place asserts nothing about
// what it means, and noticing what it seems to be called is a different act with a different
// provenance. A store that cannot do the second still does the first.
//
// It is not ScreenNamer. That one writes the Audience's word.
type PlaceNamer interface {
	ObserveSemanticName(application, id, name string, from EvidenceSource) error
}

// SettledPlaceNameFor exposes the recurrence rule for tests and diagnostics.
//
// The rule itself stays unexported and single-sited; this is a window onto it, so a test can hold
// the promotion threshold and the tie behaviour without a second copy existing.
func SettledPlaceNameFor(st ScreenState) string { return settledPlaceName(st) }

// PlaceNamesToRecord is every durable Place this session can now say a name for.
//
// # Why this is not part of establishing
//
// A Place is established the first time Marco can recognise it, and a name settles by RECURRENCE —
// so the two almost never happen on the same pass. Writing the name only at establishment meant
// the first pass established the Place with no name yet, and every later pass, having nothing left
// to establish, wrote nothing. Measured live: `state_2` tallied `Bluetooth & devices` three times
// against a threshold of two, and the durable Place carried no name at all.
//
// So naming is its own sweep over the states this session saw, and it does not care whether the
// Place is new. A Place established weeks ago gains a name the first time one settles.
//
// Recognition, not creation: a state memory cannot place is skipped rather than established here.
// This function never writes and never mints a subject; it reports what a caller could record.
//
// Deleting the already-known branch must fail TestAKnownPlaceStillGainsItsName.
func PlaceNamesToRecord(t ShadowTotals, application string, m Memory,
	th HypothesisThresholds) map[string]string {

	if m == nil {
		return nil
	}
	out := map[string]string{}
	for _, st := range t.States {
		name := settledPlaceName(st)
		if name == "" {
			continue
		}
		cp, ok := placeOfState(t, application, st.ID, m, th)
		if !ok || cp.Subject == "" {
			continue
		}
		// TWO STATES, ONE SUBJECT, DIFFERENT WORDS is Marco not knowing. The same
		// disagreement rule the Actors get, one level up.
		if prior, seen := out[cp.Subject]; seen && prior != name {
			out[cp.Subject] = ""
			continue
		}
		out[cp.Subject] = name
	}
	for subject, name := range out {
		if name == "" {
			delete(out, subject)
		}
	}
	return out
}

// settledAffordance reports whether one tallied control has recurred enough to be believed.
//
// The same rule settledPlaceName applies, for the same reason and with the same threshold: a
// control seen on ONE reading of a screen may belong to a transition frame, a menu that was open,
// a toast, or a list that had not finished loading. A control seen on two separate readings of a
// settled state is a property of the state.
//
// A disagreed kind is refused however often it recurs — see AffordanceTally.
func settledAffordance(t AffordanceTally) bool {
	return t.Kind != "" && t.Seen >= StatePromotionCount
}

// SettledAffordanceFor exposes the recurrence rule for tests and diagnostics, the way
// SettledPlaceNameFor exposes its neighbour: one rule, one site, and a window onto it rather than
// a second copy that could drift.
func SettledAffordanceFor(t AffordanceTally) bool { return settledAffordance(t) }

// TargetsToRecord is every durable target this session could now say exists, and where.
//
// # What this is, and the three things it is not
//
// It is the affordance half of PlaceNamesToRecord, arranged the same way on purpose: a QUESTION
// asked of a session's own tallies, reporting what a caller COULD record. It writes nothing,
// establishes nothing, and mints no subject — a state memory cannot recall is skipped, exactly as
// it is there.
//
// It is NOT topology. A target's signature carries the Place it was seen in and says nothing about
// where activating it leads. `Home` containing a control called `Bluetooth & devices` is not
// evidence that pressing it reaches the Bluetooth page, however strongly the words agree — that is
// a transition, and a transition needs a destination somebody was observed arriving at. See
// [[ADR-123]] and TestALabelMatchingAPlaceNameCreatesNoEdge.
//
// It is NOT a claim about what is on screen NOW. A target remembered here is memory; whether it is
// currently visible is a question for a fresh reading, and nothing in this file may answer it.
//
// It is NOT accumulated structure presented as a current reading. Every tally it consults was
// credited from the fused world of one inference, at the moment that inference was current; what
// accumulates is the COUNT, which is the evidence that the control keeps being there.
//
// # Settled states only
//
// A state still changing shape is one whose contents are still arriving, and `Settled` is the
// existing answer to "has this screen stopped moving". Reading an unsettled state would durably
// remember whatever a page happened to have painted halfway through loading.
func TargetsToRecord(t ShadowTotals, application string, m Memory,
	th HypothesisThresholds) []StructureSignature {

	if m == nil {
		return nil
	}
	// One entry per place-and-label, so a screen visited twice in one session contributes one
	// target rather than two identical ones.
	seen := map[string]bool{}
	var out []StructureSignature
	for _, st := range t.States {
		if !st.Settled || len(st.Affordances) == 0 {
			continue
		}
		cp, ok := placeOfState(t, application, st.ID, m, th)
		if !ok || cp.Subject == "" {
			continue
		}
		labels := make([]string, 0, len(st.Affordances))
		for label := range st.Affordances {
			labels = append(labels, label)
		}
		sort.Strings(labels)
		for _, label := range labels {
			if !settledAffordance(st.Affordances[label]) {
				continue
			}
			key := cp.Subject + "\x00" + label
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, TargetSignature(cp.Subject, label,
				st.Affordances[label].Kind))
			if len(out) >= MaxAffordancesPerPlace*maxPlacesPerAcquisition {
				return out
			}
		}
	}
	return out
}

// maxPlacesPerAcquisition bounds how many screens one reading may acquire for at once.
//
// A session holds every state it has visited, so a long sitting could otherwise present a hundred
// screens' worth of targets in a single sweep. This is a bound on the WORK of one reading, not on
// what Marco may know: the next reading asks again, and anything skipped is still tallied.
const maxPlacesPerAcquisition = 8

// SettlementOf reports how close one screen state is to being remembered, and why it is not.
//
// A WINDOW onto the existing rule, the way SettledPlaceNameFor is one — it decides nothing and is
// read by diagnostics only. The rule itself stays where it is, single-sited, so this cannot drift
// into a second answer to "has this screen stopped moving".
//
// `readings` is how many inferences landed in this state at all. `agreeing` is how many of them
// saw the SAME whole role composition, which is the number `settledWhole` actually thresholds
// against — and the difference between the two is the thing a fast walk breaks: a screen looked at
// three times, whose composition changed each time, has three readings and one agreement.
func SettlementOf(st ScreenState) (readings, agreeing, distinct, episodes int, settled bool) {
	best := 0
	for _, n := range st.Compositions {
		if n > best {
			best = n
		}
	}
	// `distinct` is how many DIFFERENT whole compositions this screen was seen as. It is the
	// number that says which kind of failure a screen that never settled had: two or three
	// distinct compositions is a screen that flickers between near-identical shapes, and one
	// distinct composition per reading is a screen whose shape has no stable form at all.
	return st.Inferences, best, len(st.Compositions), st.Episodes, st.Settled
}

// StateOf is one screen state out of a session's totals, for diagnostics.
func StateOf(t ShadowTotals, id ScreenStateID) (ScreenState, bool) {
	for _, st := range t.States {
		if st.ID == id {
			return st, true
		}
	}
	return ScreenState{}, false
}
