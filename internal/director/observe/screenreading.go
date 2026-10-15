package observe

// What a SLOW source read, arriving after the look it describes.
//
// # Why this is not just ScreenText with a timestamp
//
// [[ADR-194-a-screen-is-also-what-it-says]] let a screen's identity consult what was read on a
// look, and required that the reading be look-local: `fastobserve.Look.Text` is what a source
// read of THOSE pixels, and the type cannot carry anything else. That rule exists because the
// comment beside the fold refuses to feed it delayed detections — a channel arriving ten looks
// late and intermittently would make the signature of one screen depend on whether inference had
// landed yet.
//
// OCR is that channel. It costs hundreds of milliseconds against an observation cadence of a
// hundred, so no live source can ever satisfy `Look.Text`. The question is not how to make OCR
// fast. It is whether a late reading can be admitted without corrupting screen identity.
//
// # The answer, and it is the same answer as for detections
//
// A late reading may inform the interpretation of the observation it describes, and must never be
// treated as fresh evidence about a newer one. `admitSlow`'s three rules already say exactly that
// for rectangles, and the third of them is the one that carries the argument:
//
//	the reading was computed under the OBSERVATION GENERATION standing now.
//
// An epoch moves on every screen change. So an unchanged epoch is not "the same screen" — which
// a screen left and returned to would also satisfy — but "no screen change has happened since
// this frame", which is a demonstrably continuous interval. A reading from inside that interval
// is a reading of the screen standing now, however many looks ago the pixels were captured.
//
// And the measured case is inside such an interval. When Rocket League's Free Play submenu
// settled onto the Training page's identity, the merge is precisely what kept the epoch still:
// no screen change was recorded, so a reading of the submenu's own pixels arriving five looks
// later is still a reading of the screen Marco is standing on. It is admitted, it contradicts,
// and the merge is refused — late, and about the right thing.
//
// A reading whose epoch has moved is refused outright rather than filed against the screen it
// came from. Filing it would be defensible and it is not free: the screen it described has gone,
// nothing downstream is asking about it, and `Read` would need a look-indexed history to find it.
// Refusing is the honest minimum, and the counter says how often it happens.

// ScreenReading is what a slow source read on one look, stamped with the context it was read in.
//
// The same three fields `Candidate` carries, for the same reason and checked by the same rule:
// a result that cannot say which observation it belongs to cannot be judged against the one in
// front of us.
type ScreenReading struct {
	// Text is what was read. Unread text is a reading that failed, which is not a
	// disagreement — see ScreenText.
	Text ScreenText
	// FromLook is the look whose pixels this describes, for the report. Never used to
	// decide anything: a look index is a position in a sequence and the question is
	// whether the OBSERVATION moved, which only the epoch answers.
	FromLook int
	// Generation is the window generation the frame was captured under, and Epoch the
	// observation generation it was captured under.
	Generation uint64
	Epoch      uint64
	// Words is the same reading with each word's rectangle kept, normalised to the
	// window.
	//
	// `Text` is a vocabulary — what the screen says, with position thrown away — and
	// that is right for screen identity, where a label moving down a list does not make
	// a new screen. It is not enough to ask WHICH REGION said something, which is what
	// separates a control from the one above it. Carried on the same value and under the
	// same stamps, so one admission rule covers both and a caller cannot reach the
	// positions of a reading whose observation has moved on.
	Words []ReadWord
}

// Current reports whether a result computed under one context is still about the context
// standing now.
//
// THE ONE STATEMENT OF THE RULE, which `Admissible` and `ScreenReading.Admissible` both call. Two
// copies would eventually disagree, and the shape of that disagreement is an integration that
// refuses stale rectangles while quietly accepting stale words.
func Current(wasGeneration, wasEpoch, generation, epoch uint64) bool {
	return wasGeneration == generation && wasEpoch == epoch
}

// Admissible reports whether this reading is still evidence about the observation in front of us.
func (r ScreenReading) Admissible(generation, epoch uint64) bool {
	return Current(r.Generation, r.Epoch, generation, epoch)
}

// Read folds a late reading against the screen standing now, and reports whether it refuses to
// believe this is that screen.
//
// The caller has already decided the reading is admissible — that is `fastobserve`'s job and it
// applies the same rule the delayed detections go through. What happens here is the semantic
// half, and it is the same half the synchronous path does:
//
//   - the reading's words are paired with the previous admitted reading OF THE SAME SCREEN, and
//     only what both said counts. One reading is not evidence; every reading of a dark game menu
//     carries its own garbage, and garbage is by definition words the screen never said.
//   - that agreement is folded into the standing screen's vocabulary.
//   - and if it materially contradicts what that screen says, the contradiction is raised
//     against the SCREEN. It then outlives this call, which is the whole reason a late reading
//     can do anything at all: the looks between readings carry no words and must not overturn it.
//
// A reading that AGREES withdraws a contradiction an earlier one raised, so the evidence can
// correct itself before the pending run it started ever settles.
//
// Returns false with a sentence whenever there is nothing to conclude, which the caller must
// treat as agreement. Nothing here mints a screen, moves the standing claim, or touches a stored
// arrangement.
func (v *VisualScreens) Read(r ScreenReading) (bool, string) {
	v.stats.TextRead++
	if v.held == "" {
		return false, "no screen is standing, so there is nothing this could be a reading of"
	}
	if !r.Text.Read() {
		return false, "the source answered without reading anything"
	}

	// THE AGREEMENT OF TWO READINGS OF ONE SCREEN. Keyed on the screen rather than on
	// adjacency: two readings that straddle a screen change describe different things, and
	// their agreement would be whatever junk they happen to share.
	var agreed ScreenText
	if v.prevReadOf == v.held {
		agreed = TextBoth(v.prevRead, r.Text)
	}
	v.prevRead, v.prevReadOf = r.Text, v.held

	// THE SCREEN LEARNS FROM IT EITHER WAY. An agreement is stable evidence about the
	// screen standing now, and that is true whether or not it contradicts.
	v.noteText(v.held, agreed)

	apart, why, alone := false, "", false
	if agreed.Said() {
		apart, why = v.saidSomethingElse(v.held, agreed)
	} else {
		why = "this is the first reading of " + string(v.held) +
			", so there is nothing to compare it with yet"
	}
	if !apart {
		// AND ON THIS ONE READING, WHICH NOTHING HAS CORROBORATED.
		//
		// The branch above needs two readings of one screen to agree, and a screen
		// a workflow passes THROUGH is read once -- the reading that would
		// corroborate it is a reading of the next page. So a single reading may
		// raise the objection on a harder bar: see TextSoleReadingDisagrees.
		if soleApart, soleWhy := v.soleReadingSaysOtherwise(v.held, r.Text); soleApart {
			apart, why, alone = true, soleWhy, true
		}
	}
	if apart {
		v.stats.TextRefusedMerge++
		if alone {
			v.stats.TextRefusedAlone++
		}
		v.lastTextRefusal = why
		v.contradicted = v.held
		return true, why
	}
	// A REFUSAL IS WITHDRAWN BY AN AGREEMENT, not by one reading.
	//
	// Deliberately asymmetric, and the asymmetry runs the other way from the refusal
	// above: refusing on one reading costs a split, which costs a recognition, and
	// withdrawing on one reading restores the merge this exists to refuse. So a
	// withdrawal keeps the corroboration requirement it always had.
	//
	// `agreed.Said()` is a statement of that intent rather than a live branch: every
	// admitted reading sets the pairing key to the screen standing when it arrived, so a
	// refusal that is still standing always has a pair available, and a screen change
	// clears the refusal on its way past. Written down because a mutant that drops this
	// condition survives, and the reason it survives is that the case cannot arise today.
	if v.contradicted == v.held && agreed.Said() {
		v.contradicted = ""
		v.stats.TextWithdrewRefusal++
		return false, "a reading of " + string(v.held) + " agrees with it again: " + why
	}
	return false, why
}

// LastTextRefusal is the most recent sentence explaining a refusal, for a report somebody reads.
//
// Evidence, never read back as a decision — the decision is `contradicted`, which is private and
// is consulted only by the fold.
func (v *VisualScreens) LastTextRefusal() string { return v.lastTextRefusal }

// Contradicting reports whether a reading has said the standing screen is not what the
// arrangement thinks, and which screen that is about.
//
// For the report and for a caller that wants to say why a step is waiting. It is a fact about
// evidence already collected; asking it changes nothing.
func (v *VisualScreens) Contradicting() (VisualScreenID, bool) {
	return v.contradicted, v.contradicted != ""
}
