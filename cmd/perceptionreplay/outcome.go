package main

// DID THE THING THE PERSON CLICKED ACTUALLY DO ANYTHING, WHERE THE SCREEN DID NOT CHANGE?
//
// # The question, as the owner set it
//
//	The GRAPHICS click is known to change Ark's settings panel, but the global screen fold
//	merges the before and after states. Do not require a new global screen ID for every
//	meaningful UI change.
//
//	Do not count any pixel difference as success. The verifier needs positive evidence of the
//	expected destination.
//
// [[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]] measured the fold: Ark's
// VIDEO panel and GRAPHICS panel are ONE screen under both the pixel fold and the text fold, so
// the demonstrated click produces no transition, no destination and nothing to verify. The
// existing verifier is `workflowstore.Reached`, and every one of its three terms is
// WHOLE-SCREEN — the destination's arrangement against this look, the source's against it, and
// the destination's saved words against a fresh reading. With one screen at both ends there is
// no second arrangement to compare and no second vocabulary to corroborate.
//
// # What this measures instead
//
// A LOCAL outcome: the words somewhere else on the window that appeared when the control was
// pressed and stayed. Three terms, in the same shape as `Reached`'s and scoped to a region
// rather than to a screen:
//
//	1. positive       words are read AFTER that were not read before. Not a pixel difference
//	                  and not a count: the words themselves, which is what makes it evidence
//	                  OF a destination rather than evidence of motion.
//	2. persistent     every one of them is read on every readable look of the settled window,
//	                  so an animation frame and a half-drawn panel cannot be an outcome.
//	3. independent    none of them was inside the proposal the click was approved from. The
//	                  owner's requirement, and the one that makes this corroboration rather
//	                  than the same evidence counted twice.
//
// # Why "elsewhere" is defined by the proposals and not by a rectangle somebody chose
//
// Because the independence term has to be structural. The action site was approved from the
// proposals CONTAINING the click and the words inside them — that is what `button.go` reports
// and what [[Experiment-085-the-graphics-tab-is-a-button]] accepted. So the outcome may only be
// read outside all of them: a word whose box centre falls inside any proposal that contained
// the press is excluded, whatever it says. On Ark that removes the whole tab bar, GRAPHICS
// included, and leaves the panel below it.
//
// This is a measurement, like `button.go` and `temporal.go`. It presses nothing, writes nothing
// and authorises nothing.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// OutcomeSettleLooks is how many looks after a press the outcome is read over.
//
// Five at ten frames a second is half a second, which is the same window
// `observe.MaxPressToTransitionLooks` gives a screen change to appear in. The looks BETWEEN the
// press and this window are skipped, not examined: that is where a panel is half-drawn, and
// Experiment 086 found exactly one such look in Ark's capture — look 250 reads
// `AUDIO | VIDEO fs Ul CAMERA`, with the tab bar partly rendered and the body missing.
const OutcomeSettleLooks = 5

// OutcomeSkipLooks is how many looks immediately after the press are not evidence either way.
const OutcomeSkipLooks = 2

// OutcomeStableLooks is how many readable looks a word must be read on to count.
//
// Two, the same floor `observe.ScreenText` uses for a word to be part of a screen's vocabulary:
// a word read once is a reading, and a word read twice held still.
const OutcomeStableLooks = 2

// OutcomeWordFloor is the candidate word floor the matrix reports against.
const OutcomeWordFloor = 5

// localOutcome is what happened somewhere other than the control that was pressed.
type localOutcome struct {
	look int
	at   observe.PointerAt
	// elsewhere is the proposals the press landed in, which is the region the outcome may
	// NOT be read from. Kept so a report can say what was excluded.
	excluded []observe.Region
	// siteWords is what was read inside those proposals — the action site's own evidence,
	// which the outcome may not reuse.
	siteWords []string
	// before and after are the stable words elsewhere on the window, each with how many
	// readable looks it was read on.
	before, after map[string]int
	// readableBefore and readableAfter are how many looks of each window could be read.
	// Zero on either side means the question was not asked, which is not a refusal.
	readableBefore, readableAfter int
	// appeared is the words stable after and absent before, sorted. The positive evidence.
	appeared []string
	// vanished is the words stable before and absent after. Reported, never required: a
	// panel that replaced another makes both true, and a panel that merely ADDED something
	// makes only the first.
	vanished []string
	// transient is words that appeared on SOME readable look of the settled window and not
	// all of them. Excluded from `appeared` and counted here, because this is the case the
	// owner named and a verifier that accepted one would accept an animation.
	transient []string
	// reused is any appeared word that was also inside the pressed proposal. Must be
	// empty; non-empty means the outcome is the action site's evidence read twice.
	reused []string
}

// verifiedLocally reports whether the three terms hold.
//
// `appeared` is already independent of the action site: a word also inside the pressed control
// is filtered out of it and counted in `reused`. So an outcome that rested entirely on the
// site's own words has nothing left here and refuses.
func (o localOutcome) verifiedLocally() bool {
	return len(o.appeared) > 0 && o.readableBefore > 0 && o.readableAfter > 0
}

// outcomeVerdict is the one-word answer, with the reason when it is not VERIFIED.
func outcomeVerdict(o localOutcome, rule outcomeRule) string {
	switch {
	case o.readableBefore == 0 || o.readableAfter == 0:
		return "UNREADABLE — nothing was read on one side, so the question was not asked"
	case len(o.appeared) == 0 && len(o.reused) > 0:
		return "REFUSED — everything that appeared was inside the control that was " +
			"pressed: " + strings.Join(o.reused, " ")
	case len(o.appeared) == 0 && len(o.transient) > 0:
		return "REFUSED — everything that appeared was transient: " +
			strings.Join(o.transient, " ")
	case len(o.appeared) == 0:
		return "REFUSED — nothing elsewhere on the window appeared and stayed"
	case !o.verifiedUnder(rule):
		return fmt.Sprintf("REFUSED — %d word(s) appeared and stayed, and these terms "+
			"ask for %d on %d reading(s) a side: %s",
			len(o.appeared), rule.floor, rule.minLooks,
			strings.Join(o.appeared, " "))
	default:
		return "VERIFIED — " + strings.Join(o.appeared, " ")
	}
}

// outcomeRule is the terms being measured, so two of them can be put side by side.
//
// # Why this is a parameter and not a constant
//
// Because the first version was measured on ONE recording and Rocket League contradicted it.
// Ark's capture reads on 300 of 300 looks and a word floor of five took its false outcomes to
// zero; `rl-freeplay-1` reads on 300 of 900, and at the same floor 329 of 550 quiet windows
// still reported an outcome. The cause is not the floor:
//
//	ABSENCE FROM THE BEFORE WINDOW IS NOT ABSENCE FROM THE SCREEN.
//
// With OCR answering on one look in three, a word genuinely on the screen is read on fewer
// than `TextStableLooks` of the before window, and then read consistently afterwards — so it
// looks exactly like a word that appeared. The RL keypress verified on three words with ONE
// reading after it, where "stayed" cannot mean anything at all.
//
// So the terms that matter are measured here rather than assumed, on both recordings.
type outcomeRule struct {
	name string
	// minLooks is how many READABLE looks each side needs before the question is asked.
	// One is what the first version used, and one reading cannot establish that anything
	// stayed.
	minLooks int
	// absent requires a word to be read on NO readable before-look, rather than merely
	// on fewer than `TextStableLooks` of them. The direct answer to sparse readings.
	absent bool
	// floor is how many independent words it takes to be positive evidence.
	floor int
	// byReadings counts the window in READINGS rather than in looks.
	//
	// # The unit was wrong, and Rocket League is what said so
	//
	// `five words, dense readings` takes RL's false outcomes to 0 of 550 — by refusing
	// all 17 presses. Its capture reads on 300 looks of 900, so a five-LOOK window never
	// holds five readings and the dense term refuses everything. Ark's reads on 300 of
	// 300 and the same terms verify 2 of 3 with 0 of 219 false.
	//
	// That difference is not a property of the games. It is OCR cadence: Ark's readings
	// were taken offline, one per frame, and a LIVE session cannot do that — `readingEyes`
	// drops a frame whenever a reading is still running, and a reading costs hundreds of
	// milliseconds against a hundred-millisecond look. So the live path is in RL's regime,
	// and a rule that needs one reading per look would refuse every live outcome.
	//
	// A window of five READINGS is the same evidence over more wall-clock: at one reading
	// in three looks it spans about 1.5 seconds a side, which is an acceptable pause after
	// an action and is not a weaker claim — it is five readings either way.
	byReadings bool
}

// verifiedUnder reports whether an outcome clears one rule's terms.
func (o localOutcome) verifiedUnder(r outcomeRule) bool {
	return len(o.appeared) >= max(r.floor, 1) &&
		o.readableBefore >= max(r.minLooks, 1) &&
		o.readableAfter >= max(r.minLooks, 1)
}

// outcomeAt measures the local outcome of one press.
//
// `regions` and `words` are per-look and may be short or empty — a look nobody could read is
// not a look that read nothing, and the two counts say which happened.
func outcomeAt(look int, at observe.PointerAt, regions [][]observe.Region,
	words [][]observe.ReadWord, readable []bool, rule outcomeRule) localOutcome {

	o := localOutcome{look: look, at: at,
		before: map[string]int{}, after: map[string]int{}}

	// ── THE REGION THE OUTCOME MAY NOT BE READ FROM ──
	//
	// Every proposal that contained the press, on the press's own look. The action site
	// was approved from inside these, so anything read inside them is that evidence
	// again rather than corroboration of it.
	if look >= 0 && look < len(regions) {
		for _, r := range regions[look] {
			if holds(r, at) {
				o.excluded = append(o.excluded, r)
			}
		}
	}
	if look >= 0 && look < len(words) {
		seen := map[string]bool{}
		for _, w := range words[look] {
			if !excludedBy(o.excluded, w) {
				continue
			}
			for _, t := range wordsOf(w.Text) {
				if seen[t] {
					continue
				}
				seen[t] = true
				o.siteWords = append(o.siteWords, t)
			}
		}
		sort.Strings(o.siteWords)
	}

	// ── BEFORE: the settled window ending at the press's own look ──
	// ── AFTER: the same, skipping the looks where a panel is half-drawn ──
	//
	// Counted in READINGS when the rule says so, which is more wall-clock for the same
	// evidence. See outcomeRule.byReadings: a five-LOOK window on a recording that reads
	// one look in three can never hold five readings, and that is the live cadence.
	first := look + 1 + OutcomeSkipLooks
	if rule.byReadings {
		o.readableBefore = tallyBack(o.before, o.excluded, words, readable, look,
			OutcomeSettleLooks)
		o.readableAfter = tallyForward(o.after, o.excluded, words, readable, first,
			OutcomeSettleLooks)
	} else {
		o.readableBefore = tally(o.before, o.excluded, words, readable,
			look-OutcomeSettleLooks+1, look)
		o.readableAfter = tally(o.after, o.excluded, words, readable,
			first, first+OutcomeSettleLooks-1)
	}

	// ── WHAT APPEARED AND STAYED, which is the positive evidence ──
	wasBefore := func(w string) bool {
		if rule.absent {
			// READ ON NO BEFORE-LOOK AT ALL. The term sparse readings need:
			// a word read once of five before-looks was on the screen, and
			// treating it as new is how 473 of 550 quiet Rocket League windows
			// reported an outcome.
			return o.before[w] > 0
		}
		return o.before[w] >= OutcomeStableLooks
	}
	for w, n := range o.after {
		if wasBefore(w) {
			continue
		}
		switch {
		case n >= o.readableAfter:
			// ON EVERY READABLE LOOK of the settled window. The strongest
			// statement available from this evidence.
			o.appeared = append(o.appeared, w)
		case n >= OutcomeStableLooks:
			// HELD, but not throughout. Still transient for this purpose: a
			// verifier is deciding whether the destination is STANDING.
			o.transient = append(o.transient, w)
		default:
			o.transient = append(o.transient, w)
		}
	}
	for w, n := range o.before {
		if n >= OutcomeStableLooks && o.after[w] < OutcomeStableLooks {
			o.vanished = append(o.vanished, w)
		}
	}
	// ── AND THE INDEPENDENCE TERM, WHICH FILTERS RATHER THAN VETOES ──
	//
	// The first version refused the whole outcome when ANY appeared word was also inside
	// the pressed control, and the Ark measurement showed what that costs: the GRAPHICS
	// panel's body says `Advanced Graphics` and `Graphics Presets`, so the single word
	// `graphics` appeared on both sides and vetoed forty-three other words that could
	// not have come from the tab — bloom, blur, brightness, decals, distance, presets,
	// textures.
	//
	// Refusing on that is not strictness, it is losing the evidence: the requirement is
	// that the outcome does not REST on the action site's own words, not that the two
	// share no vocabulary. So a reused word is removed from the positive evidence and
	// counted, and the verdict is taken on what is left. If nothing is left, the outcome
	// rested entirely on the site's words and is refused — which is the case the term
	// exists for.
	site := map[string]bool{}
	for _, w := range o.siteWords {
		site[w] = true
	}
	kept := o.appeared[:0]
	for _, w := range o.appeared {
		if site[w] {
			o.reused = append(o.reused, w)
			continue
		}
		kept = append(kept, w)
	}
	o.appeared = kept
	sort.Strings(o.appeared)
	sort.Strings(o.vanished)
	sort.Strings(o.transient)
	sort.Strings(o.reused)
	return o
}

// tally counts how many readable looks of [lo, hi] each elsewhere-word was read on.
//
// Returns how many looks could be read at all, which is the denominator every share here is
// taken against. A look nobody could read contributes to neither.
func tally(into map[string]int, excluded []observe.Region, words [][]observe.ReadWord,
	readable []bool, lo, hi int) int {

	n := 0
	for i := lo; i <= hi; i++ {
		if i < 0 || i >= len(words) {
			continue
		}
		if i < len(readable) && !readable[i] {
			continue
		}
		if len(words[i]) == 0 {
			continue
		}
		n++
		// ONCE PER LOOK, however many times a word was read on it. A label read
		// twice on one frame is one piece of evidence about that frame.
		seen := map[string]bool{}
		for _, w := range words[i] {
			if excludedBy(excluded, w) {
				continue
			}
			for _, t := range wordsOf(w.Text) {
				if seen[t] {
					continue
				}
				seen[t] = true
				into[t]++
			}
		}
	}
	return n
}

// excludedBy reports whether a word was read inside any of the proposals that held the press.
//
// By its box's CENTRE, the same test `observe.SameRegion` is built on: a label straddling the
// edge of the tab bar belongs to whichever side its middle is on, and a rule using any overlap
// would exclude half the panel on a wide container.
func excludedBy(excluded []observe.Region, w observe.ReadWord) bool {
	c := observe.PointerAt{X: w.Where.X + w.Where.Width/2, Y: w.Where.Y + w.Where.Height/2}
	for _, r := range excluded {
		if holds(r, c) {
			return true
		}
	}
	return false
}

// wordsOf is one OCR phrase split into words, by the one rule this project splits phrases.
//
// `observe.TextWords`, not a second normaliser. The first version of this file had its own —
// lowercase and trim punctuation — and the Ark measurement showed exactly what the canonical
// rule is for: the false outcomes it produced were `1`, `4`, `\`, `/`, `d`, `c`, `j`, `o/100`
// and `ams`, which is the noise `TextWords` was written to discard after measuring forty-odd
// tokens of it on two readings of one unchanged menu.
//
// Reusing it also means a word that is part of a screen's saved vocabulary and the same word
// measured here are the same string, so the two halves of the verifier cannot drift.
func wordsOf(s string) []string { return observe.TextWords(s) }

// quietOutcome measures the same thing over a window with NO press in it.
//
// The no-action case, and the one that decides whether the verifier means anything: a rule that
// reports an outcome where nobody pressed anything is reporting the menu's own animation.
func quietOutcome(look int, regions [][]observe.Region, words [][]observe.ReadWord,
	readable []bool, rule outcomeRule) localOutcome {

	// No press, so nothing is excluded and the whole window is elsewhere. That is the
	// HARDER case for the verifier, not an easier one: more words are in scope.
	return outcomeAt(look, observe.PointerAt{X: -1, Y: -1}, regions, words, readable, rule)
}

// MaxOutcomeSpanLooks bounds how far a reading-counted window may reach.
//
// Twenty looks at ten a second is two seconds, which is a verification pause somebody would
// accept after an action. Without a bound a sparse recording would walk the whole session
// looking for its fifth reading and compare a panel against something a minute old.
const MaxOutcomeSpanLooks = 20

// tallyBack counts backwards from `from` until it has `want` readings, or runs out of span.
func tallyBack(into map[string]int, excluded []observe.Region, words [][]observe.ReadWord,
	readable []bool, from, want int) int {

	n := 0
	for i := from; i >= 0 && i > from-MaxOutcomeSpanLooks && n < want; i-- {
		n += foldLook(into, excluded, words, readable, i)
	}
	return n
}

// tallyForward counts forwards from `from` until it has `want` readings, or runs out of span.
func tallyForward(into map[string]int, excluded []observe.Region, words [][]observe.ReadWord,
	readable []bool, from, want int) int {

	n := 0
	for i := from; i < len(words) && i < from+MaxOutcomeSpanLooks && n < want; i++ {
		n += foldLook(into, excluded, words, readable, i)
	}
	return n
}

// foldLook adds one look's elsewhere-words and reports whether it was a reading at all.
func foldLook(into map[string]int, excluded []observe.Region, words [][]observe.ReadWord,
	readable []bool, i int) int {

	if i < 0 || i >= len(words) || len(words[i]) == 0 {
		return 0
	}
	if i < len(readable) && !readable[i] {
		return 0
	}
	seen := map[string]bool{}
	for _, w := range words[i] {
		if excludedBy(excluded, w) {
			continue
		}
		for _, t := range wordsOf(w.Text) {
			if seen[t] {
				continue
			}
			seen[t] = true
			into[t]++
		}
	}
	return 1
}

// StationaryQuietWindows is how many quiet windows near a press are sampled to decide whether
// the screen holds still at all.
const StationaryQuietWindows = 40

// stationaryNear is the false-outcome rate over quiet windows around one look.
//
// # The fourth term, and the one that makes the rule honest on more than one game
//
// Measured: Ark's settings panels are sound under every set of terms — 2 of 3 presses verified,
// 0 false outcomes in 219 quiet windows. Rocket League's main menu is sound under NONE of them.
// Every variant either refuses all 17 presses or admits 11 to 473 false outcomes, and no floor,
// absence term or window unit changes that.
//
// The cause is the screen rather than the rule. Rocket League's main menu carries a rotating
// news panel, animated playlists and live counters; its words change by themselves, so "a word
// appeared elsewhere and stayed" is not an outcome signal there at all. Ark's settings panel
// holds still, so it is.
//
// So the verifier asks whether the screen it is about to act on holds still, from readings it is
// already taking while it waits. A screen that produces an outcome with no input is a screen
// whose outcomes mean nothing, and the honest answer there is to refuse rather than to guess —
// which is the same shape as every other refusal in this system: a named limit, measured on the
// evidence in front of it, rather than a threshold chosen once and applied everywhere.
func stationaryNear(look int, inputs []int, regions [][]observe.Region,
	words [][]observe.ReadWord, readable []bool, rule outcomeRule) (quiet, fired int) {

	for i := look - StationaryQuietWindows; i <= look+StationaryQuietWindows; i++ {
		if i < OutcomeSettleLooks || i >= len(words)-MaxOutcomeSpanLooks {
			continue
		}
		if nearAnyInput(i, inputs) {
			continue
		}
		o := quietOutcome(i, regions, words, readable, rule)
		if o.readableBefore == 0 || o.readableAfter == 0 {
			continue
		}
		quiet++
		if o.verifiedUnder(rule) {
			fired++
		}
	}
	return quiet, fired
}
