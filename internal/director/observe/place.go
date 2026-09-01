package observe

// Where the user is standing RIGHT NOW, resolved through the canonical identity path.
//
// # Why this is one function
//
// Two callers need the answer: the demonstration capture, which asks it every observation cycle,
// and the Learn coordinator, which asks it before saying "go ahead". A second derivation would
// be a second answer to "what screen is this", and this repository has already paid for one of
// those — see the note on SignatureOfState.
//
// The chain is fixed and there is no other route through it: the current screen state becomes the
// SAME signature a subject would be stored under, and that signature is handed to memory. Nothing
// here matches structures itself, and nothing here can construct a subject.

// Place is one resolved answer to "where are we".
//
// `Placed` and `Established` are deliberately different. A transition frame or a state the
// segmenter has not settled is not placeable at all, and collapsing that into "not recognised"
// would make "I could not look" indistinguishable from "I looked and did not know it".
type Place struct {
	// Placed says the current screen could be described at all.
	Placed bool
	// Subject is the remembered subject it resolved to, empty when it resolved to none.
	Subject string
	// Verdict is how that resolution came out.
	Verdict MatchVerdict
	// Structure is the safe signature the resolution was made on.
	Structure StructureSignature
	// EditableFields is how many text-editable controls this screen reported. A COUNT — the
	// text-entry boundary, and the only thing about typing that is ever observed.
	EditableFields int
	// Reach is how far into the window this reading got, and Vacancy the evidence when it
	// did not get past the frame. See [ReachOfState] — and note that `Placed` and `Reach`
	// are a third distinction beside the two above: a shell-only reading IS describable, it
	// simply describes the window rather than the page in it.
	Reach   Reach
	Vacancy Vacancy
	// Reason is why the reading got as far as it did, from a closed set. Three distinct
	// findings all reach the content, and a caller that has to say WHY cannot recover
	// which one afterwards. See [SufficiencyOf].
	Reason SufficiencyReason
	// StateClaimed says something in this reading said WHICH state it is.
	//
	// # A different question from every other field here
	//
	// The rest of this value is about whether the screen could be READ. This is about
	// whether it said anything about what it IS — and a reading can be perfect at the first
	// and silent at the second. Measured live: an Xbox game page describes its whole shell,
	// spreads structures through the content area, offers thirty-two actionable controls,
	// and nothing in it claims a destination. Every game therefore reads identically.
	//
	// It is EVIDENCE OF A CLAIM, not the claim itself and not a name. What was claimed lives
	// in the tally this is derived from; whether it may be persisted is a separate decision
	// made elsewhere, and neither is this field.
	//
	// Deleting this must fail TestAReadingThatSaysNothingAboutItselfIsSemanticallySilent.
	StateClaimed bool
}

// Established reports whether this place is a durable subject Marco can find again.
func (p Place) Established() bool {
	return p.Placed && p.Subject != "" && p.Verdict.Established()
}

// Readable reports whether the reading got past the window frame.
//
// A caller that only wants to know "did this work" should ask [Place.Established], which is false
// either way. This is for the callers that have to say WHY, and for a future observer deciding
// whether another way of looking might do better.
func (p Place) Readable() bool { return p.Reach != ReachShell }

// PlaceNow resolves the current screen against durable memory.
//
// Returns an unplaced Place when there is no memory, or when the current state has no signature
// yet. Both are ordinary: a screen takes a few observations to become describable, and a Director
// with no memory recognises nothing by design.
// Recogniser is the only thing resolving a place needs: the ability to RECALL.
//
// Narrower than Memory on purpose, and it is the type that makes PlaceNow the canonical answer
// rather than one of several. A resolver handed the full Memory could establish a subject, note a
// session or record a relationship on the way past — and the reason "where are we" was answered
// in four places is partly that the wide interface made each of them look harmless.
//
// A Stage is a PROJECTION. It samples nothing, stores nothing, and holds no cache: given the same
// evidence and the same memory it returns the same answer, every time, to every caller.
type Recogniser interface {
	Recall(application string, sig StructureSignature) Recollection
}

func PlaceNow(t ShadowTotals, application string, m Recogniser, th HypothesisThresholds) Place {
	if m == nil {
		return Place{}
	}
	sig, ok := SignatureOfState(t, t.CurrentState, th)
	if !ok {
		return Place{}
	}
	p := Place{
		Placed: true, Structure: sig,
		EditableFields: EditableFieldsIn(t, t.CurrentState),
	}
	// HOW FAR INTO THE WINDOW THIS READING GOT, before anything is asked of memory.
	//
	// A shell-only reading describes the frame every page of an application shares. Recalling
	// it would be asking memory to identify a screen from evidence that contains no screen,
	// and the answer would be honest and useless: `MatchDifferent`, reported as "I looked and
	// did not know it" about a page nobody looked at.
	//
	// So the question is asked here and the recall is SKIPPED. Not because the answer would be
	// dangerous — it would be a miss — but because the miss is the lie. See [ReachOfState].
	//
	// Deleting this must fail TestAShellOnlyReadingIsNotAnUnknownPlace.
	p.Reach, p.Vacancy, p.Reason = ReachOfState(t, t.CurrentState)
	// AND WHETHER ANYTHING SAID WHICH STATE THIS IS, from the tally the naming rule has
	// already filled. Read here rather than re-derived: `AdmittedPlaceName` is the one thing
	// that decides what counts as a claim, and asking a second time would be a second answer.
	p.StateClaimed = stateClaimedIn(t, t.CurrentState)
	if p.Reach == ReachShell {
		return p
	}
	rec := m.Recall(application, sig)
	p.Verdict = rec.Verdict
	if rec.Verdict.Established() {
		p.Subject = rec.Subject.ID
	}
	return p
}

// EditableFieldsIn is how many text-editable controls one state reported.
//
// A count says "this screen is one you type on"; it says nothing about what was typed, because
// nothing watched.
func EditableFieldsIn(t ShadowTotals, id ScreenStateID) int {
	for _, st := range t.States {
		if st.ID == id {
			return st.EditableFields
		}
	}
	return 0
}

// stateClaimedIn reports whether anything in one screen state claimed which destination it is.
//
// The TALLY, not the settled name. Settlement is a rule about whether a word may be trusted as a
// place's name after recurrence; this is the much weaker question of whether the interface said
// anything at all about where it is. A screen that has claimed one thing once is not semantically
// silent — it is a screen whose claim has not settled yet, which is a different situation with a
// different remedy.
func stateClaimedIn(t ShadowTotals, id ScreenStateID) bool {
	for _, st := range t.States {
		if st.ID != id {
			continue
		}
		return len(st.PlaceNames) > 0
	}
	return false
}
