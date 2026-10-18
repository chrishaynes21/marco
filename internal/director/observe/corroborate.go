package observe

import (
	"fmt"
	"sort"
)

// DOES THE RECTANGLE WE MATCHED STILL SAY WHAT THE CONTROL SAID?
//
// # The gap this closes, stated as the owner stated it
//
//	Marco has learned the word "Settings." It has not yet proved that it uses that word to
//	recognize the Settings button.
//
// [[Experiment-080-it-learned-the-word-settings]] measured both halves of that sentence. A human
// click in Ark's menu produced a control 312x52 pixels across, saved as `text: ["settings"]`, and
// `ReacquireStored` found a rectangle of that shape on 98% of 300 looks with zero collisions.
//
// And `ReacquireStored` compares rectangles. So 98% means *a rectangle of this shape is in this
// place on 98% of looks* and not *the settings button is there*. The artifact carries the evidence
// that separates those two statements and nothing consulted it.
//
// # It reports; it does not decide
//
//	For now, these should be observations only. No clicks, no new matching threshold, and no
//	claim of successful semantic reacquisition.
//
// So nothing here changes which rectangle matches. A match is made exactly as it was, and this
// says what that match was worth — which is the question `reacquire`'s own report admits it cannot
// answer, because *a match rate is not a rate of right answers*.
//
// It is also the first wrongness detector at CONTROL scale that needs only one control. A
// collision needs two controls claiming one rectangle; a contradiction needs one control claiming
// a rectangle that reads something else.
//
// # Why TextsDisagree is the wrong tool here, which took checking
//
// `TextsDisagree` is the screen-scale version of this question and its semantics do not reach
// down: it refuses to call anything a disagreement until both sides carry
// `TextDisagreementWords` (three) stable words, because a screen's identity is a VOCABULARY. A
// button says "settings". Asked through that function, a one-word control could never contradict
// anything and the whole report would be inert.
//
// What does reach down is the pair of rules one level lower, and both are reused verbatim:
// `TextCommon` — a word is what a control says when it was read on half the looks that read it,
// which is `controlWords`' own rule and exists because *the person clicked PLAY on the look where
// OCR had stopped reading the word PLAY* — and `agreedWords`, which is what two readings of one
// control both had.
//
// So this file adds no threshold. It adds an accumulator and three names for what it finds.

// TextAgreement is what fresh readings say about a rectangle that matched.
type TextAgreement string

const (
	// TextCorroborated: the fresh reading of the matched rectangle shares a word with what
	// the control was saved as.
	TextCorroborated TextAgreement = "corroborated"
	// TextContradicted: both sides said something and they share nothing.
	//
	// The strongest statement available without a person, and the one that has to be
	// hardest to reach — see FreshWords.Against for the three ways it is refused.
	TextContradicted TextAgreement = "contradicted"
	// TextUnread: there is not enough fresh evidence to say either way.
	//
	// NOT corroborated, and OCR silence is NOT a contradiction. A reader that treated
	// absence as agreement would report a rate of right answers made mostly of looks
	// nobody read, and one that treated absence as disagreement would call every
	// unlabelled icon wrong.
	TextUnread TextAgreement = "unread"
)

// FreshWords is what one control's matched rectangle has been read to say NOW.
//
// Accumulated across looks rather than taken from one, which is the whole reason a contradiction
// here means anything. One look is the worst sample there is: a highlighted menu entry renders
// differently from an unhighlighted one, and `controlWords`' own doc records the press that
// proved it. So the evidence for "it says something else" has to survive repetition exactly as
// the evidence for "it says this" does.
type FreshWords struct {
	counts map[string]int
	// looks is the number of looks that contributed a READING of this rectangle, not the
	// number of looks taken. OCR answers on about one look in five, and a share
	// computed over all of them could never be reached.
	looks int
}

// Saw folds one look's reading of the rectangle.
//
// An empty reading is not folded: a look on which the rectangle read nothing is a look with no
// evidence, and counting it would push every word below the share that makes it the control's.
func (f *FreshWords) Saw(words []string) {
	if len(words) == 0 {
		return
	}
	if f.counts == nil {
		f.counts = map[string]int{}
	}
	f.looks++
	seen := map[string]bool{}
	for _, w := range words {
		if seen[w] {
			continue // a word twice on one look is one look's evidence
		}
		seen[w] = true
		f.counts[w]++
	}
}

// Looks is how many readings of this rectangle were folded.
func (f FreshWords) Looks() int { return f.looks }

// Common is the words that are usually there — `TextCommon` of the readings, the same share and
// the same reasoning as a control's saved words.
func (f FreshWords) Common() []string {
	if f.looks == 0 {
		return nil
	}
	var out []string
	for w, n := range f.counts {
		if float64(n) >= TextCommon*float64(f.looks) {
			out = append(out, w)
		}
	}
	sort.Strings(out)
	return out
}

// Against compares what the rectangle says now with what the control was saved as.
//
// Three refusals stand in front of a contradiction, and each is a case the owner named:
//
//	missing       the control was saved with no words, so there is nothing to agree with
//	unread        no look contributed a reading of this rectangle
//	insufficient  readings arrived and no word was there on half of them
//
// Only when both sides say something, repeatedly, does disagreement become a claim.
func (f FreshWords) Against(saved []string) (TextAgreement, string) {
	switch {
	case len(saved) == 0:
		return TextUnread, "the control was saved with no words, so there is nothing to agree with"
	case f.looks == 0:
		return TextUnread, "no reading of this rectangle arrived while it matched"
	}
	now := f.Common()
	if len(now) == 0 {
		return TextUnread, readings(f.looks) +
			" of this rectangle, and no word was there on half of them"
	}
	if agreed := agreedWords(saved, now); len(agreed) > 0 {
		return TextCorroborated, "it still reads " + quoteWords(agreed) +
			", from " + readings(f.looks)
	}
	return TextContradicted, "it was saved as " + quoteWords(saved) + " and now reads " +
		quoteWords(now) + ", over " + readings(f.looks)
}

// quoteWords is a word list as prose, quoted so an empty reading cannot be mistaken for one.
func quoteWords(words []string) string {
	out := ""
	for i, w := range words {
		if i > 0 {
			out += " "
		}
		out += `"` + w + `"`
	}
	return out
}

// readings is "1 reading" or "4 readings", through the package's own plural.
func readings(n int) string {
	return fmt.Sprintf("%d %s", n, plural(n, "reading", "readings"))
}
