package observe

// What a screen SAID, beside what it looked like.
//
// # Why this exists
//
// A saved screen was `{label, arrangement}` and nothing else, and the arrangement is a count of
// rectangles in nine boxes. [[Experiment-041-the-control-Marco-found-for-itself]] measured what
// that costs on a real game: Rocket League's Training page and its Free Play submenu agree with
// EACH OTHER at 0.721 by arrangement, and `VisualScreenMatch` is 0.55 — so the submenu was
// absorbed into the screen Marco was already on, and the destination matcher was never offered a
// distinct screen to recognise. The navigation succeeded and the arrival could not be
// established.
//
// The same measurement, taken over all 36 pairs of that artifact's nine screens, says the
// failure is not that pair being unlucky: three pairs sit within 0.05 of joining by count, five
// are already over the threshold by shape, and the closest pair is separated by 0.010. A margin
// of 0.010 is not a margin, and the variance of the measure between a recording pass and a live
// pass (0.540 against 0.721, for the same two screens) is larger than the distance it has to
// resolve. No threshold fixes that.
//
// The words on those two screens, by the same arithmetic, agree at **0.231** — a margin of
// +0.319. Training reads TUTORIAL, CUSTOM, TRAINING, AERIAL, STRIKER, GOALIE; Free Play reads
// BACK, JOIN, ONLINE. OCR reads both at about 95% confidence today.
//
// So: a screen is not only a collection of rectangles. It is also what those rectangles say.
//
// # What this is NOT
//
// Not an identity key. Animated labels, selection highlights, counters and clocks all put words
// on a screen that have nothing to do with which screen it is, and a rule keyed on text would
// fragment one screen into a new identity per frame. Everything here is built so that text can
// only ever REFUSE a merge the arrangement proposed, and only on evidence that held still:
//
//   - text that nobody read is not a disagreement (`Read` is false, and the fold is unchanged)
//   - text read on one look is not evidence; a word counts once it has held across looks
//   - a disagreement needs enough stable words on BOTH sides to be about anything
//   - and the arrangement still has to agree first. Text never proposes a merge, only declines
//     one.
//
// The asymmetry is deliberate and is the whole safety argument: the worst a bad reading can do
// is leave Marco with two identities for one screen, which costs a recognition. The failure in
// the other direction — two screens silently becoming one — is the one that cost a navigation.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ScreenText is what was read on a look, and who read it.
//
// `From` empty means NOBODY READ IT, which is a different thing from a screen with no words on
// it. The second is a finding about the screen; the first is a gap in the evidence, and conflating
// them would make a frame OCR happened to miss look like a screen that went silent.
type ScreenText struct {
	// Words is each distinct word and how many times it was read on this look.
	Words map[string]int `json:"words,omitempty"`
	// From is the source that read them. Empty means nobody looked.
	From directorapi.ObservationSource `json:"from,omitempty"`
	// Looks is how many readings this text was accumulated from, for a stored screen.
	// Zero or one on a single look.
	Looks int `json:"looks,omitempty"`
}

// Read reports whether anybody actually read this look.
func (t ScreenText) Read() bool { return t.From != "" }

// Said reports whether anybody read words on it.
func (t ScreenText) Said() bool { return t.Read() && len(t.Words) > 0 }

// List is the words, sorted, for a report somebody reads.
func (t ScreenText) List() []string {
	out := make([]string, 0, len(t.Words))
	for w := range t.Words {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

// TextOf reads a look's words out of whatever a source returned.
//
// One place that decides what counts as a word, so the learner, the live observer and the
// measurement cannot drift: lowercased, letters and digits only, and nothing shorter than two
// characters — a stray "I" from an edge or "l" from a bullet is OCR noise and would match
// almost anything.
func TextOf(from directorapi.ObservationSource, phrases []string) ScreenText {
	t := ScreenText{From: from, Looks: 1}
	for _, p := range phrases {
		for _, w := range TextWords(p) {
			if t.Words == nil {
				t.Words = map[string]int{}
			}
			t.Words[w]++
		}
	}
	return t
}

// TextWords splits a phrase the one way this package splits phrases.
//
// Nothing shorter than TextWordLetters, which is where most of OCR's output goes. Measured
// on 300 readings of a real game: two readings of the SAME main menu produced
// `ae an bs ee fe om on oo pe ro si we ww` and `ae an ap as bd be cy eo fc fe ke li me mi
// mt na ne oe of op re se ss st ve` — forty-odd tokens of pure noise, none of which says
// anything about which screen it is, and all of which counted as words the other reading
// did not have.
func TextWords(s string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		if len(f) >= TextWordLetters {
			out = append(out, f)
		}
	}
	return out
}

// TextSimilarity compares what two screens said.
//
// The SAME arithmetic as `ArrangementSimilarity` — `signatureSimilarity`, min over max across the
// union of keys — so the two numbers can be put beside each other in a report and mean the same
// kind of thing. That is why 0.231 and 0.721 above are comparable at all.
//
// On PRESENCE rather than count: a word's count varies with how many times OCR happened to
// resolve it, and a title read once against a label read three times is not a 0.33 agreement
// about what the screen says. Two readings with no words in common score 0; two with the same
// vocabulary score 1, however often each word appeared.
//
// Unread text is never alike anything, INCLUDING other unread text, for the same reason
// `ArrangementSimilarity` refuses two unobserved arrangements: the honest answer about evidence
// nobody collected is that nothing is known, not that the two agree.
func TextSimilarity(a, b ScreenText) float64 {
	if !a.Said() || !b.Said() {
		return 0
	}
	return signatureSimilarity(presence(a), presence(b))
}

// presence is the words as a set, each worth one.
func presence(t ScreenText) map[string]float64 {
	f := make(map[string]float64, len(t.Words))
	for w := range t.Words {
		f["word:"+w] = 1
	}
	return f
}

const (
	// TextKept is how much of what a screen says a look must still say to be a look at
	// that screen, and TextNovel how much of a look must be words the screen never said
	// before the two are taken to be different screens.
	//
	// # Why two numbers and not one similarity
	//
	// Because one number cannot tell a DIFFERENT screen from the SAME screen read more
	// fully, and a rule that confuses them fragments every menu that loads its labels
	// in. By min-over-max on presence -- the same arithmetic the arrangement uses:
	//
	//	the real failure   Training(10 words) against FreePlay(6)   3 shared   0.231
	//	a screen growing   Training(3 words) against Training(10)   3 shared   0.300
	//
	// 0.231 and 0.300. No threshold between those separates a submenu from a menu whose
	// labels finished loading, and Jaccard and Dice do no better, because the relation
	// genuinely is asymmetric. What separates them is WHICH SIDE the missing words are
	// on: a growing screen still says everything it said, and a partial reading says
	// nothing new.
	//
	// # Where the numbers come from
	//
	// [[Experiment-042-a-screen-remembers-what-it-said]], over 300 OCR readings of 900
	// real frames, folded through the production observer. Every pair the ARRANGEMENT
	// would have merged, with what the words said about it:
	//
	//	                                      arrangement   kept   novel
	//	main menu / map select                      0.686   0.00    1.00   different
	//	main menu / garage                          0.651   0.21    0.73   different
	//	Free Play submenu / custom training         0.617   0.00    1.00   different
	//	Training page / Free Play submenu           0.559   0.20    0.80   different
	//	main menu / main menu, news panel rotated   0.946   0.43    0.37   THE SAME
	//
	// The four real separations keep at most 0.21 of the screen's vocabulary and bring
	// at least 0.73 new. The one pair that must NOT separate -- Rocket League's main
	// menu with its featured panel showing something else -- keeps 0.43 and brings 0.37.
	// So there are two gaps, 0.21..0.43 and 0.37..0.73, and these numbers sit inside
	// both rather than on either edge.
	//
	// At 0.50 kept, measured, that menu split in two and then flapped between the halves
	// 46 times in 900 looks. That is the failure the owner predicted of this whole idea
	// -- "animated labels, dynamic counters and selection highlights could fragment one
	// screen into many identities" -- and it is a rotating news panel, not a counter.
	//
	// They are deliberately NOT VisualScreenMatch: that number governs a different
	// measure with a different distribution, and sharing it would be a coincidence
	// pretending to be a rule.
	TextKept  = 0.35
	TextNovel = 0.55

	// TextStableLooks is how many readings a word must appear in before it is part of what
	// a screen says.
	//
	// Two, and the reason is the failure this must not cause. A selection highlight moves
	// a label into and out of OCR's reach from frame to frame; a counter shows a different
	// number every frame; a title animates in. Each of those puts words on exactly one
	// reading. A word that survives a second reading of the same screen is a word the
	// screen has, rather than a word that was passing through.
	//
	// Two rather than three — unlike VisualSettleLooks — because this is not damping a
	// transition storm. It is excluding single-frame noise from a vocabulary, and the
	// readings are expensive: OCR costs hundreds of milliseconds, so a rule needing three
	// of them would often never be satisfied at all.
	TextStableLooks = 2

	// TextDisagreementWords is how many stable words each side needs before they are
	// allowed to disagree.
	//
	// Three. Two screens that have read two words each have not said enough for "these are
	// different screens" to be a claim about anything — one missed word would flip it —
	// and the whole point of this evidence is that it has a margin where the arrangement
	// does not. Measured: the two screens of the failed run carry ten and six stable words.
	TextDisagreementWords = 3

	// TextWordLetters is the shortest token that counts as a word.
	//
	// Three. A one- or two-character token carries no screen identity -- it is either
	// OCR noise or a stopword -- and it is most of what OCR returns from a dark game
	// menu. Counted as words, those tokens dominate: two readings of one menu
	// disagreed about forty of them while agreeing about every label on the screen.
	//
	// The cost is that a screen whose only distinguishing text is a two-letter label
	// cannot be told apart this way, and the honest answer there is that the
	// arrangement is all there is.
	TextWordLetters = 3

	// TextCommon is the share of a screen's readings a word must appear in to be part
	// of what that screen says.
	//
	// Half, and it is the difference between a vocabulary and a transcript. Without it a
	// screen accumulates every misreading that ever happened twice in a row, and the
	// longer Marco looks at a screen the less any single look appears to agree with it:
	// measured, the main menu reached a hundred and more words over 62 readings, of
	// which perhaps forty were ever on the screen. `kept` -- the share of the
	// vocabulary a look still says -- then sits near 0.3 for a perfectly ordinary look
	// at that menu, and whether the merge is refused turns on noise. The menu flapped
	// between three identities 55 times in 900 looks.
	//
	// With it, a word has to be there MOST of the time Marco looks. A label is; a
	// misreading of a gradient is not; and a vocabulary stops growing once the screen
	// has been seen a few times, which is what makes `kept` mean the same thing on the
	// sixtieth look as on the third.
	TextCommon = 0.5
)

// TextBoth is the words two readings share, which is what a look can be said to STABLY say.
//
// # Why a look's side has to be stable too
//
// A screen's vocabulary is built by agreement across its readings, so noise never joins
// it. A single look has no such protection: every reading of a dark game menu carries its
// own garbage, and garbage is by definition words the screen never said. Measured on the
// Free Play recording, taking a single reading as the look's side split the main menu
// into EIGHT identities over 449 looks -- not because the menu changed, but because each
// reading of it brought thirty new misreadings.
//
// So both sides are agreements: the screen's across its looks, and the look's across the
// last two readings. The cost is that a contradiction is raised one reading later than it
// could be, which at a realistic reading density is a few looks -- and the pending run it
// starts needs three looks anyway.
func TextBoth(a, b ScreenText) ScreenText {
	if !a.Read() || !b.Read() {
		return ScreenText{}
	}
	out := ScreenText{From: a.From, Looks: 2}
	for w, n := range a.Words {
		if m, ok := b.Words[w]; ok {
			if out.Words == nil {
				out.Words = map[string]int{}
			}
			out.Words[w] = n + m
		}
	}
	return out
}

// TextOverlap is how much of `said` a later reading `now` still says, and how much of `now`
// is words `said` never had.
//
// Both as fractions of their own side, which is the asymmetry the whole rule turns on. See
// the constants above for the measurement that forced it.
func TextOverlap(said, now ScreenText) (kept, novel float64) {
	if len(said.Words) == 0 || len(now.Words) == 0 {
		return 0, 0
	}
	shared := 0
	for w := range said.Words {
		if _, ok := now.Words[w]; ok {
			shared++
		}
	}
	fresh := 0
	for w := range now.Words {
		if _, ok := said.Words[w]; !ok {
			fresh++
		}
	}
	return float64(shared) / float64(len(said.Words)),
		float64(fresh) / float64(len(now.Words))
}

// TextsDisagree reports whether a later reading is materially about a different screen from
// the one `said` describes, and says why in either direction.
//
// `said` is a screen's accumulated vocabulary and `now` is what a look read. The order
// matters: the question is whether this look contradicts that screen, not whether the two
// are interchangeable.
//
// Everything short of a contradiction is reported as no evidence, which the caller must
// treat as agreement — see the note at the top of this file on which direction the risk
// runs.
func TextsDisagree(said, now ScreenText) (bool, string) {
	switch {
	case !said.Read() || !now.Read():
		return false, "nobody read one of them, which is not a disagreement"
	case !said.Said() || !now.Said():
		return false, "one of them read no words at all"
	case len(said.Words) < TextDisagreementWords || len(now.Words) < TextDisagreementWords:
		return false, fmt.Sprintf("%d and %d stable word(s), fewer than the %d it "+
			"takes to be a claim", len(said.Words), len(now.Words),
			TextDisagreementWords)
	}
	kept, novel := TextOverlap(said, now)
	switch {
	case kept >= TextKept:
		return false, fmt.Sprintf("it still says %.0f%% of what that screen said, "+
			"at or above the %.0f%% that makes it the same screen",
			kept*100, TextKept*100)
	case novel <= TextNovel:
		return false, fmt.Sprintf("only %.0f%% of what it says is new, at or below "+
			"the %.0f%% that would make it another screen — a reading of the "+
			"same screen with fewer words resolved", novel*100, TextNovel*100)
	}
	return true, fmt.Sprintf("it says %.0f%% of what that screen said and %.0f%% of "+
		"what it says is new (%s against %s)", kept*100, novel*100,
		strings.Join(said.List(), " "), strings.Join(now.List(), " "))
}

// TextSoleReadingDisagrees reports whether ONE reading, which nothing has corroborated, may
// refuse to believe this is that screen.
//
// # Why there is a second, stricter rule at all
//
// `TextBoth` requires the look's side to be an agreement of two consecutive readings, and the
// measurement that forced it is real: a single reading as the look's side split one main menu
// into eight identities. The cost of that protection is a delay of one reading, and
// [[Experiment-046-the-destination-it-reached-and-did-not-recognise]] measured what the delay
// costs — a screen a workflow passes THROUGH is read once, or not at all, and the second
// reading that would corroborate the first is a reading of the next page.
//
// So one reading may raise the objection, on a bar the corroborated rule does not have to clear.
// `said` is what the screen has been read to say and `everSaid` every word ever folded into it,
// unfiltered:
//
//	it must disagree with the screen's VOCABULARY            the measured kept/novel rule
//	and with everything that screen has EVER been read to say which is the corroboration the
//	                                                          look's own side is missing
//	and it must bring TextDisagreementWords words that        so the claim rests on words
//	screen has never said                                     rather than on a fraction
//
// The last two are what keep a rotating panel from separating. A screen whose content changes
// under one identity has usually said the new words before — the panel comes back around — and
// `everSaid` remembers that even after `TextCommon` has dropped them from what the screen
// usually says. And a reading that merely resolved fewer labels brings nothing new at all, which
// is the same rule as `TextNovel` stated on words instead of on a share: missing text is not a
// contradiction.
//
// What this may NOT do is mint an identity. A refusal keeps a look off the standing screen; the
// arrangement still has to hold for `VisualSettleLooks` before anything is a new screen, and
// nothing here reaches a stored signature. See screentextfold.go.
func TextSoleReadingDisagrees(said, everSaid, now ScreenText) (bool, string) {
	apart, why := TextsDisagree(said, now)
	if !apart {
		return false, why
	}
	// EVERYTHING THIS SCREEN HAS EVER BEEN READ TO SAY, not only what it usually says.
	if ever, everWhy := TextsDisagree(everSaid, now); !ever {
		return false, "on one reading, and " + everWhy
	}
	// AND THE NEW WORDS HAVE TO BE A CLAIM. `novel` is a share, so two new words out of
	// three clear it; one uncorroborated reading does not get to rest a screen identity
	// on two words. Counted against everSaid, for the same reason as above.
	fresh := make([]string, 0, len(now.Words))
	for w := range now.Words {
		if _, old := everSaid.Words[w]; !old {
			fresh = append(fresh, w)
		}
	}
	if len(fresh) < TextDisagreementWords {
		sort.Strings(fresh)
		return false, fmt.Sprintf("on one reading it brings %d word(s) that screen has "+
			"never said (%s), fewer than the %d it takes to be a claim",
			len(fresh), strings.Join(fresh, " "), TextDisagreementWords)
	}
	sort.Strings(fresh)
	return true, fmt.Sprintf("%s — on one reading, and it brings %s, which that screen "+
		"has never said", why, strings.Join(fresh, " "))
}
