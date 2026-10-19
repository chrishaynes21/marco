package observe

// WHAT A PRESS DID, WHERE THE SCREEN DID NOT CHANGE.
//
// # The gap this fills
//
// A step is learned from a press followed by a screen CHANGE. Ark's settings menu has two
// panels under one persistent tab bar, and [[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]]
// measured what that costs: the panel below the tab bar changes completely, the screen does not,
// and so a demonstrated click on the GRAPHICS tab produces no transition, no destination and
// nothing to verify. Two screens over 300 looks, one visit each, the same under the pixel fold
// and the text fold.
//
// The owner's instruction was to fix the verification rather than the segmentation:
//
//	Do not require a new global screen ID for every meaningful UI change. […] Do not count any
//	pixel difference as success. The verifier needs positive evidence of the expected
//	destination.
//
// # The three terms, measured on the owner's own Ark capture
//
//	positive      words are read AFTER that were not read before. The words themselves, so
//	              this is evidence OF a destination rather than evidence of motion.
//	persistent    each one on every readable look of the settled window. A half-drawn panel
//	              and an animating title put words on one look and cannot clear this.
//	independent   none of them read inside the proposals the press landed in. The action site
//	              was approved from those, so a word from inside them is that evidence again
//	              rather than corroboration of it.
//
// Measured with `perceptionreplay -frames -outcome` on `ark-menu-1`:
//
//	press 3, the GRAPHICS tab   VERIFIED on 36 independent words — bloom blur brightness
//	                            decals distance presets textures — with `graphics` itself
//	                            filtered out, because the panel body also says it
//	press 2, the gap between    VERIFIED on 19 — resolution, window, upscaler. The action
//	tabs                        site ABSTAINED on this press and the outcome did not, which
//	                            is the two halves being independent in both directions
//	press 1, the pause menu     REFUSED. Everything that appeared was a clock and a ping
//	                            counter, which is the transient case
//	the `pause` keypress        VERIFIED on 25. A key has no action site at all, so nothing
//	                            is excluded and the whole window is evidence
//	219 quiet windows           0 false outcomes at OutcomeWords, 33 at one word
//
// # What it is not
//
// Not a screen identity, not a durable one, and not a threshold on a similarity measure. It is
// a set of words a demonstration observed somewhere other than the control it pressed. Nothing
// here mints a target, moves one, or changes which rectangle a press identified.
//
// Not proof of causation either. `Arrival.Verified` stays false and still says why: arriving
// where a step expected is not by itself proof that the step's action caused it.

import "sort"

// OutcomeBetween is the whole rule, as a pure function over two sets of readings.
//
// # One rule, three uses, which is why it is shaped like this
//
//	at demonstration   the readings before a press against the readings after it. What comes
//	                   out is the EXPECTATION, saved on the step.
//	at replay          the readings before Marco acts against the readings after. What comes
//	                   out has to overlap the expectation, which is what makes it positive
//	                   evidence of the EXPECTED destination rather than of any change.
//	as a self-test     two windows of readings with NO action between them. Anything that
//	                   comes out is a screen that changes by itself, and its outcomes mean
//	                   nothing. See the stationary term below.
//
// `excluded` is the regions an outcome may not be read from — at demonstration the proposals
// that contained the press, at replay the saved control's readings and containers. `before` and
// `after` are one reading per look, most recent last, already filtered to the screen in
// question by the caller.
func OutcomeBetween(excluded []Region, before, after [][]ReadWord) LocalOutcome {
	o := LocalOutcome{BeforeLooks: len(before), AfterLooks: len(after)}
	seen := func(window [][]ReadWord) map[string]int {
		n := map[string]int{}
		for _, read := range window {
			for _, t := range elsewhereWords(excluded, read) {
				n[t]++
			}
		}
		return n
	}
	was, is := seen(before), seen(after)
	for w, n := range is {
		switch {
		case was[w] > 0:
			// READ ON NO BEFORE-READING AT ALL, not merely on fewer than the
			// stable floor. The term sparse readings need: a word read once of
			// five before-readings was on the screen, and treating it as new is
			// how 473 of 550 quiet Rocket League windows reported an outcome.
			continue
		case n < len(after):
			// NOT ON EVERY READING of the settled window. A verifier is deciding
			// whether the destination is STANDING, so anything less is transient.
			o.Transient = append(o.Transient, w)
		default:
			o.Appeared = append(o.Appeared, w)
		}
	}
	for w := range was {
		if is[w] == 0 {
			o.Vanished = append(o.Vanished, w)
		}
	}
	sort.Strings(o.Appeared)
	sort.Strings(o.Vanished)
	sort.Strings(o.Transient)
	return o
}

// Stationary reports whether a screen holds still enough for its outcomes to mean anything.
//
// # The fourth term, and the one that makes the rule honest on more than one game
//
// Measured on two recordings. Ark's settings panels are sound under every set of terms — 2 of 3
// presses verified, 0 false outcomes in 219 windows with no input in them. Rocket League's main
// menu is sound under NONE of them: every variant either refuses all 17 presses or admits 11 to
// 473 false outcomes, and no word floor, absence term or window unit changes that.
//
// The cause is the screen rather than the rule. Rocket League's main menu carries a rotating
// news panel, animated playlists and live counters, so its words change by themselves and "a
// word appeared elsewhere and stayed" is not an outcome signal there at all. Ark's settings
// panel holds still, so it is. Per press, the quiet windows around it separate the two cleanly:
// 0 of 64, 0 of 64 and 0 of 54 on Ark, against 2 to 12 of 25 to 64 on eleven of Rocket League's
// seventeen — including the one press that would otherwise have verified falsely.
//
// So this is asked of the screen in front of the planner, from readings it is already taking
// while it waits, and a screen that fails it is refused rather than guessed at.
func Stationary(excluded []Region, windows [][][]ReadWord) (quiet, moved int) {
	for i := 0; i+1 < len(windows); i++ {
		o := OutcomeBetween(excluded, windows[i], windows[i+1])
		if o.BeforeLooks == 0 || o.AfterLooks == 0 {
			continue
		}
		quiet++
		if o.Verified() {
			moved++
		}
	}
	return quiet, moved
}

// OutcomeWords is how many independent words it takes for an outcome to be positive evidence.
//
// FIVE, and the number is measured rather than chosen. On 219 windows of Ark's capture with no
// input of any kind in them, the false-outcome count falls 33, 2, 1, 1, 0 as the floor goes 1,
// 2, 3, 4, 5 — and every demonstrated press that verified at one word still verifies at ten. So
// five is where the baseline reaches zero and it costs the real outcomes nothing.
//
// It is NOT `TextDisagreementWords`, which is three and governs whether two screens have said
// enough to disagree. A different question with a different distribution, and sharing the
// constant would be a coincidence pretending to be a rule — the same argument `TextKept` makes
// about `VisualScreenMatch`.
const OutcomeWords = 5

// OutcomeSettleLooks is how many looks of a screen an outcome is read over, each side.
const OutcomeSettleLooks = 5

// MaxOutcomeWatches bounds how many presses a session accumulates outcome evidence for.
const MaxOutcomeWatches = 64

// OutcomeSkipReadings is how many readings immediately after an action are not evidence.
//
// # TWO, and the number is measured twice from the same recording
//
// The readings taken straight after a press describe the application MID-RESPONSE. On Ark's
// capture the tab bar is partly rendered and the panel body missing entirely —
// [[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]] names the frame.
//
// The persistence term requires a word on EVERY reading of the settled window, so including a
// mid-draw frame makes every word of the new panel appear on four readings of five and fall out
// as transient. That is not a subtle effect. With no skip the correlator reported `everything
// that appeared was transient` for all three of Ark's presses and learned ONE step; with a skip
// of one it reported exactly the same thing; with two it verifies presses 2 and 3 on 19 and 36
// words and learns THREE steps — the same two numbers `perceptionreplay -frames -outcome`
// produces with its own `OutcomeSkipLooks = 2`.
//
// So the live path and the offline measurement agree because they skip the same amount, and not
// by coincidence. Getting it wrong by one looked exactly like fixing the wrong thing, which is
// why the amount is written down here with what it was measured against.
//
// A transition longer than two readings still fails the persistence term and is refused as
// transient, which is the safe direction: a verifier that waited until it agreed would be
// looking for a reason to say yes.
const OutcomeSkipReadings = 2

// LocalOutcome is what a press changed somewhere other than the control it pressed.
type LocalOutcome struct {
	// Target is the control that was pressed, empty for a press that resolved to none.
	Target VisualTargetID `json:"target,omitempty"`
	// Screen is the screen that was standing, and Look the press's own look.
	Screen VisualScreenID `json:"screen"`
	Look   int            `json:"look"`
	// Appeared is the words read after and not before, independent of the action site,
	// sorted. The positive evidence, and what a replay has to see again.
	Appeared []string `json:"appeared,omitempty"`
	// Vanished is the words read before and not after. Reported, never required: a panel
	// that REPLACED another makes both true and a panel that merely added something makes
	// only Appeared.
	Vanished []string `json:"vanished,omitempty"`
	// Reused is the words that appeared and were also inside the pressed control, removed
	// from Appeared and counted here.
	//
	// Not a refusal on its own. The first version of this rule refused the whole outcome
	// when any appeared word was also a site word, and the Ark measurement showed the
	// cost: the GRAPHICS panel body says `Advanced Graphics`, so one shared word vetoed
	// 43 that could not have come from the tab. The requirement is that the outcome does
	// not REST on the site's words, not that the two share no vocabulary.
	Reused []string `json:"reused,omitempty"`
	// Transient is words that appeared on some readable look of the settled window and
	// not all of them. The case a verifier must never accept.
	Transient []string `json:"transient,omitempty"`
	// BeforeLooks and AfterLooks are how many readings each side rests on. Zero on either
	// side means the question was not asked, which is not a refusal.
	BeforeLooks int `json:"before_looks,omitempty"`
	AfterLooks  int `json:"after_looks,omitempty"`
}

// Verified reports whether the outcome is positive, persistent and independent.
func (o LocalOutcome) Verified() bool {
	return len(o.Appeared) >= OutcomeWords && o.BeforeLooks > 0 && o.AfterLooks > 0
}

// Why says what the evidence was, or what was missing.
func (o LocalOutcome) Why() string {
	switch {
	case o.BeforeLooks == 0 || o.AfterLooks == 0:
		return "nothing was read on one side of the press, so the question was not asked"
	case len(o.Appeared) == 0 && len(o.Reused) > 0:
		return "everything that appeared was inside the control that was pressed, so " +
			"the outcome would be the action site's own evidence read twice"
	case len(o.Appeared) == 0 && len(o.Transient) > 0:
		return "everything that appeared was transient — read on some looks of the " +
			"settled window and not all of them"
	case len(o.Appeared) == 0:
		return "nothing elsewhere on the window appeared and stayed"
	case len(o.Appeared) < OutcomeWords:
		return "only a few words appeared and stayed, which is not positive evidence " +
			"of a destination"
	default:
		return "words appeared elsewhere on the window and stayed, none of them from " +
			"inside the control that was pressed"
	}
}

// outcomeWatch is one press's readings, before and after, waiting to be read as an outcome.
//
// It holds READINGS and not counts, so the conclusion comes from `OutcomeBetween` and there is
// exactly one copy of the rule. An earlier version accumulated word counts here and duplicated
// the terms; the two would have drifted the first time one was changed.
type outcomeWatch struct {
	target VisualTargetID
	screen VisualScreenID
	look   int
	// excluded is the proposals that contained the press. The action site was approved
	// from inside these, so the outcome may not be read there.
	excluded []Region
	// before is the readings of this screen taken before the press, oldest first, and
	// after the readings taken since.
	before, after [][]ReadWord
	// skipped is how many readings after the press were dropped as mid-response. See
	// OutcomeSkipReadings.
	skipped int
}

// watch opens an outcome watch for one press, with whatever was read on this screen before it.
func (c *ClickTargets) watch(obs ClickObservation) {
	if len(c.watching) >= MaxOutcomeWatches {
		return
	}
	w := outcomeWatch{target: obs.Target, screen: obs.Screen, look: obs.Look}
	for _, cand := range obs.Candidates {
		w.excluded = append(w.excluded, cand.Region)
	}
	// THE BEFORE SIDE, from the readings already taken of this screen. A press is folded
	// before the rectangle it establishes exists, so these are the readings taken while
	// the screen the person was looking at was still standing — which is exactly what
	// `recent` is held for.
	//
	// Counted in READINGS, not looks. A reading costs hundreds of milliseconds against a
	// hundred-millisecond look, so a window of five LOOKS holds one or two readings on a
	// live session — and a rule needing one reading per look was measured to refuse every
	// outcome in a recording read at that cadence.
	for i := len(c.recent) - 1; i >= 0 && len(w.before) < OutcomeSettleLooks; i-- {
		if c.recent[i].screen != obs.Screen {
			continue
		}
		w.before = append([][]ReadWord{c.recent[i].words}, w.before...)
	}
	c.watching = append(c.watching, w)
}

// observeOutcome folds one reading into every open watch of its screen.
func (c *ClickTargets) observeOutcome(screen VisualScreenID, read []ReadWord) {
	for i := range c.watching {
		w := &c.watching[i]
		if w.screen != screen || len(w.after) >= OutcomeSettleLooks {
			continue
		}
		// THE READING TAKEN MID-RESPONSE IS NOT EVIDENCE EITHER WAY.
		//
		// Counted and dropped. See OutcomeSkipReadings: including it made every
		// word of Ark's new panel transient, because the persistence term wants a
		// word on every reading of the window and the panel is not drawn yet on
		// the first one.
		if w.skipped < OutcomeSkipReadings {
			w.skipped++
			continue
		}
		w.after = append(w.after, append([]ReadWord{}, read...))
	}
}

// outcomes is what each watch concluded, through the one rule.
func (c *ClickTargets) outcomes() []LocalOutcome {
	out := make([]LocalOutcome, 0, len(c.watching))
	for _, w := range c.watching {
		o := OutcomeBetween(w.excluded, w.before, w.after)
		o.Target, o.Screen, o.Look = w.target, w.screen, w.look
		out = append(out, Independent(w.excluded, w.before, o))
	}
	return out
}

// Independent is the independence term, as a filter over an outcome already measured.
//
// # Why it filters rather than vetoes
//
// The first version refused the whole outcome when ANY appeared word was also inside the
// pressed control, and the Ark measurement showed the cost: the GRAPHICS panel body says
// `Advanced Graphics`, so the single word `graphics` vetoed forty-three others that could not
// have come from the tab. The requirement is that the outcome does not REST on the site's
// words, not that the two share no vocabulary.
//
// `site` is the words read INSIDE the excluded regions, and the filter that keeps it to those
// cannot change a verdict. A mutation run proved it: collecting every before-word instead
// survives every test, because the extra ones are exactly the words `OutcomeBetween`'s
// before-term has already excluded — anything read elsewhere before the press can never be in
// `Appeared`.
//
// Kept anyway, and not deleted as the unreachable guard in `clickLookIsSmallest` was: that one
// was a condition nothing could satisfy, and this one is what makes `site` mean what its name
// says. A reader of `Reused` has to be able to trust that those words came from the control.
//
// # And it is exported because the second caller got it wrong twice
//
// `perceptionreplay -fixture` derives a step's expectation from two captures rather than from a
// demonstration, and it hand-rolled this term and the persistence term instead of calling
// these. Both hand-rolled versions were wrong, and measurably:
//
//	the independence term    37 words against the correlator's 36 on the same Ark capture.
//	                         The extra one was `graphics`, which the tab says and the panel it
//	                         opens says too, so the expectation rested by one word on the
//	                         evidence the action site had been approved from.
//	the persistence term     TWELVE words of outcome on a Trackmania recording the fold
//	                         measures as ONE screen, 862 looks, one visit. A single look each
//	                         side cannot ask whether a word STOOD, so OCR flicker and a
//	                         highlight bar moving off a row both read as a destination.
//
// One word of thirty-seven changed no verdict and twelve words of noise would have minted a
// workflow. Two paths that disagree about what an outcome IS will eventually disagree about one
// that matters, so there is one function now and both callers use it.
func Independent(excluded []Region, before [][]ReadWord, o LocalOutcome) LocalOutcome {
	site := map[string]bool{}
	for _, read := range before {
		for _, word := range read {
			if elsewhere(excluded, word) {
				continue
			}
			for _, t := range TextWords(word.Text) {
				site[t] = true
			}
		}
	}
	kept := o.Appeared[:0]
	for _, t := range o.Appeared {
		if site[t] {
			o.Reused = append(o.Reused, t)
			continue
		}
		kept = append(kept, t)
	}
	o.Appeared = kept
	sort.Strings(o.Reused)
	return o
}

// elsewhereWords is one reading's words from outside the excluded regions, deduplicated.
//
// Once per look however many times a word was read on it: a label read twice on one frame is
// one piece of evidence about that frame.
func elsewhereWords(excluded []Region, read []ReadWord) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range read {
		if !elsewhere(excluded, w) {
			continue
		}
		for _, t := range TextWords(w.Text) {
			if seen[t] {
				continue
			}
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// elsewhere reports whether a word was read outside every excluded region.
//
// By the word box's CENTRE, the same test `SameRegion` is built on: a label straddling the edge
// of a container belongs to whichever side its middle is on, and a rule using any overlap would
// exclude half a panel because one wide proposal reached into it.
func elsewhere(excluded []Region, w ReadWord) bool {
	c := PointerAt{X: w.Where.X + w.Where.Width/2, Y: w.Where.Y + w.Where.Height/2}
	for _, r := range excluded {
		if containsPoint(r, c) {
			return false
		}
	}
	return true
}
