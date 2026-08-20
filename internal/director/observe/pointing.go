package observe

// Choosing WHAT to point at.
//
// # The failure this exists to prevent
//
// "Point at something" has an easy wrong answer: take the current screen's biggest group and
// outline it. That produces a highlight every time, which is exactly what makes it dangerous —
// it looks identical to a correct one, and it means nothing. A highlight is a sentence: "this is
// what I mean". If Marco is not currently meaning anything, the honest output is a refusal.
//
// So the subject is never chosen for being convenient. It is taken from something Marco actually
// said: a question it asked, or an interpretation it formed. Both carry a subject already —
// pointing adds no semantics, it only locates what was already decided.

// PointingBasis is WHY this is the thing being pointed at.
//
// Carried so a surface can word the highlight honestly. "I'm asking about these" and "I think
// these belong together" are different claims, and a presentation that could not tell them apart
// would have to pick one and be wrong half the time.
type PointingBasis string

const (
	// BasisOpenQuestion — Marco is asking about this right now.
	BasisOpenQuestion PointingBasis = "a_question_marco_is_asking"
	// BasisAskedQuestion — Marco asked about this, and the question has been settled.
	BasisAskedQuestion PointingBasis = "a_question_marco_asked"
	// BasisInterpretation — Marco formed an interpretation about this and has not asked.
	BasisInterpretation PointingBasis = "an_interpretation_marco_formed"
)

// PointingRefusal is why Marco has nothing to point at.
//
// Distinct from [ReferentUnavailable], which is why a KNOWN subject cannot be located. These two
// failures read completely differently to a person: "I don't have anything to show you" and
// "I know what I mean and can't find it on screen" send somebody to different places.
type PointingRefusal string

const (
	// PointingReady is the empty refusal.
	PointingReady PointingRefusal = ""
	// NothingObserved — no session has produced a structural account to point into.
	NothingObserved PointingRefusal = "nothing_has_been_observed"
	// NothingMeant — Marco observed, and formed nothing about any part of the screen. The
	// ordinary outcome for an application whose structure never recurred enough to interpret.
	NothingMeant PointingRefusal = "marco_is_not_referring_to_anything"
	// NotRecognisedHere — the subject is remembered, and nothing on screen right now has been
	// recognised as it.
	//
	// THE honest answer for "show me what this refers to" on a durable judgement. Distinct
	// from every other refusal here: Marco knows exactly what it was told and by whom, it
	// simply is not looking at it. Consulting the stored geometry to point anyway is the
	// mistake this whole mechanism exists to prevent.
	NotRecognisedHere PointingRefusal = "not_recognised_on_screen_right_now"
)

// Say is the sentence a person reads.
func (r PointingRefusal) Say() string {
	switch r {
	case PointingReady:
		return ""
	case NothingObserved:
		return "I haven't watched anything yet, so there's nothing for me to point at."
	case NothingMeant:
		return "I watched, but I haven't formed anything about a particular part of this " +
			"screen — so there's nothing I'd be pointing at."
	case NotRecognisedHere:
		return "I remember this, but I can't currently point to it."
	}
	return "I don't have anything to point at right now."
}

// Pointing is what Marco would show, and why that.
type Pointing struct {
	Referent VisualReferent  `json:"referent"`
	Basis    PointingBasis   `json:"basis,omitempty"`
	Refusal  PointingRefusal `json:"refusal,omitempty"`
	// Question is the sentence Marco asked, when the basis is a question. Already generic and
	// free of identifiers — see questionFor — so it is safe beside a highlight.
	Question string `json:"question,omitempty"`
	// Interpretation is the plain observation sentence, when the basis is one. Same guarantee.
	Interpretation string `json:"interpretation,omitempty"`
}

// WhatToPointAt picks the subject Marco is currently referring to and locates it.
//
// # The order, and why it is this order
//
// An OPEN question first: if Marco is asking somebody something right now, that is unambiguously
// what "this" means, and it is the case where not being able to see the referent is most costly.
// Then a question already asked, then an interpretation formed but never raised. Every one of
// them is something Marco SAID; none of them is "the group that happened to be largest".
//
// Only group subjects are considered. A screen state is not a part of a screen — pointing at it
// would mean outlining the whole window — and a transition is an event rather than a place.
// [ReferentForSubject] refuses those anyway; skipping them here is what stops a screen-shaped
// subject from consuming the choice and turning into a refusal when a perfectly pointable group
// was available.
//
// Replacing the search with "the first group in the current state" must fail
// TestMarcoPointsAtSomethingItSaidAndNotAtWhateverIsLargest.
func WhatToPointAt(props []Proposal, hyps []Hypothesis, live LiveGeometry) Pointing {
	if len(live.Tracks) == 0 && len(live.States) == 0 {
		return Pointing{
			Refusal:  NothingObserved,
			Referent: VisualReferent{Unavailable: ReferentNothingWatched},
		}
	}

	// Questions Marco is asking now, then ones it has asked.
	for _, want := range []ProposalStatus{ProposalOpen, ""} {
		for _, p := range props {
			if p.Subject.Kind != SubjectGroup {
				continue
			}
			if want != "" && p.Status != want {
				continue
			}
			basis := BasisAskedQuestion
			if p.Status == ProposalOpen {
				basis = BasisOpenQuestion
			}
			return Pointing{
				Referent: ReferentForSubject(p.Subject, ReferentQuestion, live),
				Basis:    basis, Question: p.Question,
			}
		}
	}

	for _, h := range hyps {
		if h.Subject.Kind != SubjectGroup {
			continue
		}
		return Pointing{
			Referent:       ReferentForSubject(h.Subject, ReferentJudgement, live),
			Basis:          BasisInterpretation,
			Interpretation: h.Observed,
		}
	}

	return Pointing{Refusal: NothingMeant}
}

// WhatThisQuestionMeans locates the subject of ONE named question.
//
// # Why this is not WhatToPointAt
//
// [WhatToPointAt] answers "what is Marco referring to", and its whole job is to CHOOSE — it skips
// a screen-shaped subject so that one does not consume the choice while a pointable group is
// available. That is right for "show me what you mean" and catastrophically wrong for a button
// sitting under a specific question, because the thing it falls through to is a different subject
// and nothing in the result says so.
//
// Found live: Marco asked "I keep seeing a screen that looks like a menu — is that what it is?",
// whose subject is a screen state. A surface that asked the general question got back an
// interpretation about a seventeen-control group and would have highlighted it under that
// sentence. Two subjects, one box, no way for the person to tell.
//
// So this one does not choose. It resolves the question it was given, or refuses. When the subject
// is a whole screen the answer is [ReferentNotAPart] — "I'm asking about a whole screen rather than
// a part of one, so there's nothing on it for me to single out" — which is a true sentence Marco
// must be willing to say rather than a reason to go looking for something else to draw.
//
// Falling back to WhatToPointAt here must fail
// TestAQuestionAboutAScreenNeverBorrowsAnotherSubjectsHighlight.
func WhatThisQuestionMeans(id ProposalID, props []Proposal, live LiveGeometry) Pointing {
	if id == "" {
		return Pointing{Refusal: NothingMeant}
	}
	if len(live.Tracks) == 0 && len(live.States) == 0 {
		return Pointing{
			Refusal:  NothingObserved,
			Referent: VisualReferent{Unavailable: ReferentNothingWatched},
		}
	}
	for _, p := range props {
		if p.ID != id {
			continue
		}
		basis := BasisAskedQuestion
		if p.Status == ProposalOpen {
			basis = BasisOpenQuestion
		}
		// The subject is resolved WHATEVER kind it is. ReferentForSubject already refuses a
		// screen with the right reason, and letting it do that is the entire point: the
		// alternative is this function deciding the question is unanswerable and quietly
		// answering a different one.
		return Pointing{
			Referent: ReferentFor(p, ReferentQuestion, live),
			Basis:    basis, Question: p.Question,
		}
	}
	// A question this session does not hold. Not "nothing to point at" — a specific thing was
	// asked for and it is not here, which is what a stale page in a browser tab produces.
	return Pointing{Refusal: NothingMeant}
}

// WhatRemembersThis locates a DURABLE subject, if a session currently recognises it.
//
// # The one rule that matters here
//
// The link from a remembered subject to something on this screen is RECOGNITION, and nothing else.
// A session that matched a live structure to a stored subject recorded which session-local subject
// it matched; that is the only honest bridge between the two, and it is the bridge this uses.
//
// The stored `StructureSignature.Envelope` is deliberately not consulted. It is identity, it can be
// years old, and the record that made this whole surface necessary has one sitting mostly above its
// own window — pointing with it would be confidently indicating nothing. "I remember this, but I
// can't currently point to it" is the correct sentence, and it is a sentence Marco must be willing
// to say.
//
// Substituting stored geometry for recognition must fail
// TestShowingWhatAJudgementRefersToNeverUsesRememberedGeometry.
func WhatRemembersThis(subject string, props []Proposal, live LiveGeometry) Pointing {
	if subject == "" {
		return Pointing{Refusal: NotRecognisedHere}
	}
	if len(live.Tracks) == 0 && len(live.States) == 0 {
		return Pointing{
			Refusal:  NothingObserved,
			Referent: VisualReferent{Unavailable: ReferentNothingWatched},
		}
	}
	for _, p := range props {
		if !p.Recognised || p.RecognisedAs != subject {
			continue
		}
		if p.Subject.Kind != SubjectGroup {
			continue
		}
		return Pointing{
			Referent: ReferentForSubject(p.Subject, ReferentJudgement, live),
			Basis:    BasisAskedQuestion, Question: p.Question,
		}
	}
	return Pointing{Refusal: NotRecognisedHere}
}

// Say is what a surface puts beside the highlight, or in place of one.
//
// One sentence, always safe: it is built from the closed basis vocabulary and from text that has
// already been through the question generator, never from a label, a title or anything read off
// the screen.
func (p Pointing) Say() string {
	if p.Refusal != PointingReady {
		return p.Refusal.Say()
	}
	if !p.Referent.CanPoint() {
		return p.Referent.Unavailable.Say()
	}
	switch p.Basis {
	case BasisOpenQuestion, BasisAskedQuestion:
		return "This is what I'm asking about."
	case BasisInterpretation:
		return "This is what I mean."
	}
	return "This is what I mean."
}
