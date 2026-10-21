package workflowstore

// IS THIS THE SCREEN THE ACT STARTS ON — ANSWERED FROM EVIDENCE, NOT BY ASKING?
//
// # Why this exists
//
// Because the question used to be put to a person, once per run, and there was no honest way to
// put it to them: a widget has no terminal to type a label in, so the ranking was read out and
// one function key accepted its top candidate. That is a person agreeing with an arrangement
// score they have no way to check. ADR-186 was right that a person can say where they are; what
// it could not fix is that a yes/no on a ranking is not somebody saying where they are — it is
// somebody initialling Marco's guess.
//
// The authorisation contract that replaces the keypress ceremony has to replace the EVIDENCE the
// keypress stood in for. Pressing Play authorises an Act's named interactions on the application
// it is pinned to. It cannot authorise them on a screen nobody has identified, so this is the
// term that holds instead: Marco says which saved screen it is looking at, positively, or it
// says it cannot and the run pauses with that sentence in front of the person.
//
// # The terms, and why the arrangement is not the strong one
//
// `Reached` establishes a step's DESTINATION from three terms, measured over 222 folds. This is
// the same question with no step in front of it — there is no source screen for the look to be
// closer to than — so the contrast that does the work there is unavailable, and what replaces it
// is the term the arrangement cannot supply.
//
// Experiment-041 measured why. Over all 36 pairs of one artifact's nine screens, three pairs sit
// within 0.05 of joining by arrangement, five are already over the threshold, and the closest
// pair is separated by 0.010 — while the measure's own variance between a recording pass and a
// live pass of the SAME two screens is 0.181. A margin of 0.010 inside a variance of 0.181 is
// not a margin, and no threshold rescues it. The words of that pair, by the same arithmetic,
// agree at 0.231 against 0.721 by shape: a margin of +0.319.
//
// So the arrangement is a GATE here and never the discriminator:
//
//	1. the wanted screen agrees with this look at VisualScreenMatch   the existing threshold
//	2. and a fresh admissible reading POSITIVELY identifies it        kept >= TextKept, rather
//	                                                                 than merely uncontradicted
//	3. and no OTHER saved screen clears both of those                 the margin, taken in the
//	                                                                 measure that has one
//
// Term 2 is deliberately stronger than `Reached`'s third term. There, a reading that fails to
// contradict a destination is admissible evidence of arrival, because a step was demonstrated to
// lead there and the press has already happened. Here nothing has happened yet and a press is
// about to be made on this screen without anybody being asked again, so "nothing says otherwise"
// is not enough: the screen has to say what it was read to say.
//
// Term 3 is what the keypress was really being asked for. Two sibling panels under one tab bar
// are the case the owner's own game presents, and a person pressing yes to "is this the graphics
// screen?" can tell them apart no better than the arrangement can. If both clear the gates then
// nobody has identified anything, and the run says exactly that.
//
// # What this is still not
//
// Not independent, and the binding's ground says so. The expectation comes from the artifact
// whose Act is about to be performed, exactly as an inference does, and no verdict rests on it
// alone: the press is still re-checked against the look it goes out in, the pointer still has to
// arrive, and the outcome is still verified afterwards. Nothing here reaches the global screen
// model, no arrangement is stored and no identity is minted.

import (
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Standing establishes the screen an Act expects to start on, from this look's own evidence.
//
// `look` is the arrangement of the look in front of the caller and `read` a reading already
// admitted for this observation, both supplied for the reason `Reached` states: this package
// cannot see, and the admission rule is stated once, where the readings arrive.
//
// `readWhy` is why there is no reading, in the words of the caller that would know, and it is
// used for nothing except the sentence term 2 refuses with. Empty is allowed and says only that
// nobody supplied a reason.
//
// # Why the absence of a reading needs a reason at all
//
// Because "nothing has been read on it yet" was one sentence over three situations with three
// different remedies, and a live run spent its whole patience saying the harmless one. OCR not
// having answered yet is a thing that fixes itself by waiting. A reading that arrives about a
// screen which has already changed is what an animated menu does to every reading it will ever
// take, and no amount of waiting fixes it. Pixels that were not evidence because the window was
// not in front is a thing the person can fix in one click, and they cannot do that while being
// told to wait. See the caller in cmd/director/observationloop.go.
//
// The error is written to be READ BY THE PERSON WHO PRESSED PLAY, because it is the pause
// reason: it says which screen was expected, what the evidence was, and which term did not hold.
func (s *SessionBinding) Standing(a Artifact, want string, screen observe.VisualScreenID,
	look observe.VisualArrangement, read observe.ScreenText, readWhy string,
	epoch, generation uint64, by string, now time.Time) (Bound, error) {

	if screen == "" {
		return Bound{}, fmt.Errorf("no screen is standing yet, so there is nothing to " +
			"identify")
	}
	// ALREADY SETTLED IN THIS OBSERVATION, and not re-decided. A confirmation somebody
	// gave is the stronger evidence and this must not overwrite it.
	if b, ok := s.Established(want, epoch, generation); ok {
		return b, nil
	}
	wanted, ok := a.screenNamed(want)
	if !ok || !wanted.Arrangement.Observed() {
		return Bound{}, fmt.Errorf("%q carries no arrangement for %s, so there is "+
			"nothing to recognise it by", a.Name, want)
	}
	if !look.Observed() {
		return Bound{}, fmt.Errorf("this look has no arrangement yet, so it is not " +
			"evidence about anything")
	}

	// ── 1. THE GATE: IT LOOKS LIKE THE SCREEN THE ACT STARTS ON ──
	agrees := observe.ArrangementSimilarity(wanted.Arrangement, look)
	if agrees < observe.VisualScreenMatch {
		return Bound{}, fmt.Errorf("what is on screen agrees with %s at %.3f, below "+
			"%.2f — this does not look like the screen %q starts on",
			want, agrees, observe.VisualScreenMatch, a.Name)
	}

	// ── 2. AND IT SAYS WHAT THAT SCREEN WAS READ TO SAY ──
	said := wanted.Said()
	if !said.Said() {
		return Bound{}, fmt.Errorf("%s was saved without any words, so a reading "+
			"cannot identify it — and the arrangement alone has been measured to "+
			"confuse two screens of one menu", want)
	}
	if len(said.Words) < observe.TextDisagreementWords {
		return Bound{}, fmt.Errorf("%s was saved with %d word(s), fewer than the %d it "+
			"takes to tell two screens apart, so it cannot be identified without "+
			"somebody being asked", want, len(said.Words),
			observe.TextDisagreementWords)
	}
	if !read.Said() {
		// AND WHY THERE IS NOTHING TO CORROBORATE IT WITH, which is the half of
		// this sentence a person can act on. See readWhy above.
		because := readWhy
		if because == "" {
			because = "nothing has been read on it yet"
		}
		return Bound{}, fmt.Errorf("what is on screen agrees with %s at %.3f and "+
			"%s, so nothing corroborates it", want, agrees, because)
	}
	if apart, why := observe.TextsDisagree(said, read); apart {
		return Bound{}, fmt.Errorf("what is on screen agrees with %s at %.3f by shape "+
			"and what is written on it says otherwise: %s", want, agrees, why)
	}
	kept, _ := observe.TextOverlap(said, read)
	if kept < observe.TextKept {
		return Bound{}, fmt.Errorf("what is written on screen keeps %.0f%% of what %s "+
			"was read to say, below the %.0f%% that identifies it — it may be "+
			"this screen and the evidence does not say so",
			kept*100, want, observe.TextKept*100)
	}

	// ── 3. AND NOTHING ELSE SAVED HERE WOULD PASS THE SAME TWO TERMS ──
	//
	// The margin, taken in the measure that has one. A second screen clearing both gates
	// means this evidence does not distinguish them, and an Act's press must not be made
	// on "one of these two".
	for _, other := range a.Screens {
		if other.Label == want || !other.Arrangement.Observed() {
			continue
		}
		if observe.ArrangementSimilarity(other.Arrangement, look) <
			observe.VisualScreenMatch {
			continue
		}
		alsoSaid := other.Said()
		if !alsoSaid.Said() {
			continue
		}
		if also, _ := observe.TextOverlap(alsoSaid, read); also >= observe.TextKept {
			return Bound{}, fmt.Errorf("this could be %s or %s — both agree with "+
				"what is on screen, and what is written on it keeps %.0f%% of "+
				"one and %.0f%% of the other, so nothing here tells them apart",
				want, other.Label, kept*100, also*100)
		}
	}

	// IDENTIFIED, AND RECORDED AS THAT AND NOT AS A CONFIRMATION.
	//
	// Written straight into the bindings rather than through `Confirm`, which stamps
	// `GroundConfirmed` — a person said yes while looking at it — and counts a question
	// somebody answered. Neither is true here, and a run whose report said a person had
	// confirmed this screen would be claiming consent nobody gave.
	//
	// `Depth` is zero because there is no chain: this rests on the look in front of it,
	// not on an inference from somebody's earlier answer. `Positive` is always true and
	// `Thin` always false, because both are terms above rather than qualities noted
	// afterwards — a thin vocabulary cannot be identified at all.
	because := fmt.Sprintf("%s identified from the screen itself: it agrees at %.3f, "+
		"what is written on it keeps %.0f%% of what %s was read to say, and no "+
		"other saved screen passes both terms",
		want, agrees, kept*100, want)
	b := Bound{
		Label: want, Screen: screen, Ground: GroundIdentified,
		Epoch: epoch, Generation: generation, Depth: 0,
		By: by, At: now, Positive: true, Because: because,
	}
	s.bound[want] = b
	return b, nil
}
