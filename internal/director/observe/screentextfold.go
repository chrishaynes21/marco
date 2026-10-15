package observe

// Text, where a screen's identity is decided.
//
// There are exactly three places an arrangement decides who a look belongs to, and two of them
// could merge two screens:
//
//	1. the standing screen      a look alike enough to it settles onto it            <- vetoed
//	2. the pending run          a look alike enough to the run continues it
//	3. promotion                a settled run alike enough to a known screen reuses it <- vetoed
//
// The measured failure was (1). Rocket League's Free Play submenu agreed with the Training page
// it replaced at 0.721, so it settled onto `visual_3` and no new screen was ever minted; the
// destination matcher then had nothing to recognise `screen_d` as. (3) is the same failure one
// step later: release the standing claim and the run would promote straight back onto the
// identity it just left.
//
// (2) is deliberately NOT vetoed. The pending run is a claim about one arrangement holding
// still, and splitting it on text would make a run of three looks fail to settle because OCR
// read a hover highlight on the second one — which is the fragmentation this evidence is
// supposed to be safe from. A run that settles is then checked at (3) before it is allowed to
// reuse anything.
//
// # What a screen says, and when
//
// A screen accumulates a vocabulary over the looks that were READ, and a word joins it on its
// second appearance. So a counter, a clock, an animating title and a selection highlight — each
// of which puts words on one reading — never become part of what a screen says, while a label
// that is there whenever anybody looks does. See TextStableLooks.
//
// A look is compared against that vocabulary, not against the last reading. Comparing two single
// readings would make the veto turn on whichever words OCR happened to resolve twice in a row.

import (
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// screenVocabulary is what one screen has been read to say, over its looks.
type screenVocabulary struct {
	// readings is how many of this screen's looks anybody read.
	readings int
	// seen is each word and how many readings it appeared in.
	seen map[string]int
	// from is the source that read them.
	from sourceOf
}

// sourceOf is the provenance of a vocabulary, kept as the last source that contributed.
//
// One source in practice. Kept as a field rather than assumed so a reading whose provenance was
// never set cannot be mistaken for one that was — `ScreenText.Read` is the only test for that,
// and it has to have something to test.
type sourceOf = string

// add folds one reading into the vocabulary. An unread look contributes nothing.
func (v *screenVocabulary) add(t ScreenText) {
	if !t.Read() {
		return
	}
	v.readings++
	v.from = sourceOf(t.From)
	if v.seen == nil {
		v.seen = map[string]int{}
	}
	for w := range t.Words {
		v.seen[w]++
	}
}

// raw is every word ever folded into this screen, with no commonness filter, for the one
// question that may be asked of an uncorroborated reading.
//
// `stable` is what a screen may be said to SAY and is the only thing a merge is refused against.
// This is weaker and is used only to make a sole reading's refusal HARDER: a word this screen has
// said before, even rarely, is not a word that makes this a different screen. See
// TextSoleReadingDisagrees.
//
// What is folded here is already an agreement of two readings — see `stable` — so "ever said" is
// still corroborated evidence about this screen, and a single misreading never enters it.
func (v screenVocabulary) raw() ScreenText {
	if v.readings == 0 || v.from == "" {
		return ScreenText{}
	}
	out := ScreenText{From: directorapi.ObservationSource(v.from), Looks: v.readings}
	for w, n := range v.seen {
		if out.Words == nil {
			out.Words = map[string]int{}
		}
		out.Words[w] = n
	}
	return out
}

// stable is what the screen says: the union of every AGREEMENT folded into it.
//
// What is folded here is already a `TextBoth` — words two consecutive readings of this screen
// both had — so the stability rule has been applied before the word arrives and is not applied
// twice. One agreement is enough for a word to belong; requiring two would need three
// consecutive readings of one screen, and readings cost hundreds of milliseconds each.
//
// A screen nothing has been folded into is UNREAD, not empty. Nobody has established that it
// says anything, and returning an empty-but-read vocabulary would let it disagree with
// everything.
//
// # Why agreements and not raw readings
//
// Measured, and it was a real defect rather than a refinement. The first look of a new screen
// has no agreement yet -- the reading before it was of the PREVIOUS screen, so the two share
// nothing -- and that look therefore settles onto the standing screen, correctly, with nothing
// to contradict it. Folding its RAW reading in taught the old screen the new screen's words. On
// the second visit the old screen's vocabulary contained both, agreed with the new screen's
// reading at exactly the threshold, and the merge this whole mechanism exists to refuse happened
// anyway. Folding the agreement instead folds nothing on that look, which is the honest answer:
// no stable evidence, nothing learned.
func (v screenVocabulary) stable() ScreenText {
	// A SCREEN READ ONCE HAS NOT ESTABLISHED WHAT IT SAYS.
	//
	// Measured. The main menu was contradicted on the strength of its first
	// agreement, a second identity was minted for it, and from then on every look
	// at that menu matched whichever of the two vocabularies happened to be closer
	// -- 23 visits each over 900 looks, for one screen that never changed. A
	// vocabulary that may refuse a merge has to have been corroborated first, and
	// one agreement is one corroboration of the words and none of the screen.
	if v.readings < TextStableLooks || v.from == "" {
		return ScreenText{}
	}
	// THE WORDS THAT ARE USUALLY THERE. See TextCommon: a vocabulary that kept every
	// word ever folded in grows for as long as Marco looks at the screen, and a
	// measure of "how much of this screen is this look still saying" then falls
	// steadily towards nothing for a screen that never changed.
	//
	// The floor is one, so a screen seen once or twice says what it was read to say.
	need := int(float64(v.readings)*TextCommon + 0.5)
	if need < 1 {
		need = 1
	}
	out := ScreenText{From: directorapi.ObservationSource(v.from), Looks: v.readings}
	for w, n := range v.seen {
		if n < need {
			continue
		}
		if out.Words == nil {
			out.Words = map[string]int{}
		}
		out.Words[w] = n
	}
	return out
}

// ObserveWith folds one look, with whatever was read on it, and returns the screen it is a look
// AT.
//
// `Observe` is this with nothing read, and behaves exactly as it always did — which is the point:
// every existing caller keeps the pixel-only path, and text is something a caller that has it
// can add.
//
// # The one thing text may do
//
// Refuse a merge. A look whose stable text materially disagrees with the standing screen's does
// not settle onto it, however alike the two arrangements are; the look goes on to start or
// continue a pending run, exactly as a look that differed by arrangement would. Text cannot
// propose a merge, cannot mint a screen on its own, and is never part of a stored arrangement —
// `visualScreen.sig` stays pixel-only, so a screen's signature does not depend on whether
// anybody read it.
func (v *VisualScreens) ObserveWith(a VisualArrangement, t ScreenText, at time.Time) VisualScreenID {
	// THE LOOK'S SIDE IS AN AGREEMENT TOO: the words this reading and the one before it
	// both have. See TextBoth for the measurement that forced it -- a single reading as
	// the look's side split one main menu into eight identities.
	//
	// `lookText` is what may contradict a screen; `lookRead` is the raw reading, which is
	// what a screen's own vocabulary is built from. The screen's side has its own
	// agreement rule over its own looks, so folding the raw reading there is right.
	v.lookRead = t
	v.lookText = ScreenText{}
	if t.Read() {
		v.lookText = TextBoth(v.prevText, t)
		v.prevText = t
	}
	defer func() { v.lookText, v.lookRead = ScreenText{}, ScreenText{} }()
	return v.Observe(a, at)
}

// saidSomethingElse reports whether this look's reading contradicts a screen's vocabulary.
//
// Both sides have to be stable, and the look's own reading is compared as a single reading
// against the screen's accumulated one. A single reading is allowed to raise the objection — it
// is the screen's side that must have held still — because the alternative is waiting two looks
// to notice a screen changed, during which the merge has already happened.
//
// The asymmetry is safe in the direction that matters: a one-off bad reading can only split a
// screen in two, and a split costs a recognition while a merge costs a navigation.
func (v *VisualScreens) saidSomethingElse(id VisualScreenID, t ScreenText) (bool, string) {
	if !t.Said() {
		return false, ""
	}
	for i := range v.screens {
		if v.screens[i].id != id {
			continue
		}
		apart, why := TextsDisagree(v.screens[i].text.stable(), t)
		if !apart {
			return false, why
		}
		return true, fmt.Sprintf("%s and this look %s", id, why)
	}
	return false, ""
}

// soleReadingSaysOtherwise reports whether ONE uncorroborated reading contradicts a screen.
//
// The same question as `saidSomethingElse` and a stricter rule, because nothing has corroborated
// the look's side — see TextSoleReadingDisagrees for the bar and for what forced it. The SCREEN's
// side is unchanged and is still `stable()`: a screen that has not established what it says may
// not be contradicted, which is the measurement that minted a second identity for one main menu
// and is not being reopened.
//
// Called only where a merge onto the STANDING screen is decided. `promote` deliberately does not
// ask it: a sole reading may refuse to believe this is that screen, and may not go looking for
// identities to keep a settled run away from. What reaches a promotion is the refusal already
// standing against the standing screen, which is how a screen released for saying something else
// avoids promoting straight back onto it.
func (v *VisualScreens) soleReadingSaysOtherwise(id VisualScreenID, t ScreenText) (bool, string) {
	if !t.Said() {
		return false, ""
	}
	for i := range v.screens {
		if v.screens[i].id != id {
			continue
		}
		apart, why := TextSoleReadingDisagrees(v.screens[i].text.stable(),
			v.screens[i].text.raw(), t)
		if !apart {
			return false, why
		}
		return true, fmt.Sprintf("%s and this look %s", id, why)
	}
	return false, ""
}

// noteText records a reading against a screen, and the stats that say whether any of this is
// doing anything.
func (v *VisualScreens) noteText(id VisualScreenID, t ScreenText) {
	if !t.Read() {
		return
	}
	v.stats.TextRead++
	for i := range v.screens {
		if v.screens[i].id == id {
			v.screens[i].text.add(t)
			return
		}
	}
}

// SaidBy is what a screen has been read to say, stable words only.
//
// Exported for the learner, which saves it beside the arrangement, and for the measurements that
// characterise it. An unread screen answers unread text rather than empty text.
func (v *VisualScreens) SaidBy(id VisualScreenID) ScreenText {
	for i := range v.screens {
		if v.screens[i].id == id {
			return v.screens[i].text.stable()
		}
	}
	return ScreenText{}
}
