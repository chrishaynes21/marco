package directorapi

// One instruction's permission to operate a Prop that only pixels describe.
//
// # The boundary this sits on, and why it is not a change to Actuable
//
// `ActuatingSource` admits accessibility, a native integration, a DOM bridge and a capability
// pack, because each can say how to OPERATE a thing. A vision detector and an OCR reader
// describe what the screen looks like and nothing about how to work it, so `Actionability`
// reports `Actuable: false` for an element only they saw and `Targetable()` refuses it. That
// rule is right and stays exactly as it is: it is what stops Marco operating a surface nobody
// vouched for, on its own initiative, by default, everywhere.
//
// What it also did was make a whole class of application unreachable except by demonstration.
// A game exposes no accessibility tree, so every control in it is pixel-only, and the only
// thing that has ever licensed clicking one is a person having demonstrated the click — which
// lives in an entirely separate stack. The owner's decision:
//
//	An explicit user instruction MAY authorize one action on a pixel-only Prop without a
//	prior demonstration. However, authorization and grounding must remain separate: the
//	instruction authorizes the requested action, not a guessed target.
//
// So a licence is a fact about THE REQUEST, not about the element, the screen or the session.
// It says a person asked for this, in these words, naming this verb. It says nothing whatever
// about which Prop is meant.
//
// # What it deliberately cannot do
//
//   - It cannot resolve ambiguity. Two Props matching the request refuse exactly as they did
//     before; a person asking for "play" has not thereby said which "play".
//   - It cannot supply grounding. Geometry still has to be there, visible and on screen.
//   - It cannot reach policy. Risk, destructiveness and every other permission question are
//     evaluated after planning, from the action itself, and nothing here is consulted there —
//     so "click delete" is licensed to the same extent and refused for the same reasons it
//     always was.
//   - It cannot outlive the request. It is a value built from one parsed instruction and passed
//     down one call; nothing stores it, and there is nowhere for it to be stored.
//   - It cannot make a Prop permanent. Nothing here writes an identity, a workflow or a memory.
//
// # And text alone is not a surface
//
// The licence requires that something described the OBJECT and not only its words. This is the
// lesson of [[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] one tier up: OCR reading
// "TRAINING" says a word is at a rectangle, and the rectangle of a word is a caption, not the
// control it labels. Pressing the middle of a word because the word matched is how Marco
// clicked 17 px from the edge of a tile. A detector's box, a window frame or a proposed region
// is a claim about a THING; a read word is a claim about a glyph run.
type OperateLicence struct {
	// Instruction is the request that granted it, verbatim, for the record.
	Instruction string
	// Verb is the one action it covers, as the parser normalised it ("click").
	//
	// The INTENT verb rather than a SemanticActionKind, because the licence is a fact
	// about what the person asked for and the mapping from a request to a semantic
	// action happens later, in the planner. Inventing the mapping here would put a
	// second copy of it in the permission path.
	Verb string
}

// Granted reports whether a licence was given at all. The zero value is no licence.
func (l OperateLicence) Granted() bool { return l.Instruction != "" && l.Verb != "" }

// Covers reports whether this licence permits that action.
//
// One verb, matched exactly. A licence for `click` does not license `type`: the person named
// one thing to do and a second action is a second decision.
func (l OperateLicence) Covers(verb string) bool {
	return l.Granted() && l.Verb == verb
}

// DescribesAnObject reports whether any source accounted for the THING rather than only for
// text rendered on it.
//
// OCR is excluded, and that exclusion is the whole point — see the note on OperateLicence. A
// provenance with no sources answers false: a constructed element has not been seen by anything,
// and a licence may not be spent on one.
func (p Provenance) DescribesAnObject() bool {
	for _, ref := range p.Sources {
		if ref.Source != SourceOCR {
			return true
		}
	}
	return false
}

// OperableUnder reports whether this element may be operated for this request, and why not.
//
// The ordinary rule first: anything an actuating source described is operable and no licence is
// involved. Only a Prop that nothing but pixels reported reaches the licence at all, so a
// licensed request behaves identically to an unlicensed one everywhere Marco already worked.
func (e *Element) OperableUnder(l OperateLicence, verb string) (bool, string) {
	if e == nil {
		return false, "there is no element"
	}
	if !e.Provenance.OnlyDescribesPixels() {
		return true, ""
	}
	switch {
	case !l.Covers(verb):
		return false, "only pixels describe it, and nothing has said it may be operated"
	case !e.Provenance.DescribesAnObject():
		return false, "only read text describes it, which locates a word and not the " +
			"control it is written on"
	}
	return true, ""
}
