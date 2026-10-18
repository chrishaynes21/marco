package main

// `perceptionreplay -frames -outcome <dir>` — can a demonstrated click's OUTCOME be verified
// where the screen did not change?
//
// Four cases, all from the same recording, because the owner's instruction was to answer this
// from the capture already in hand:
//
//	the demonstrated press    did something appear elsewhere and stay?
//	no action                 the same question over windows with no press in them. Must refuse.
//	the wrong press           a press that landed in the gap between controls. Must refuse.
//	a transient change        counted separately and never accepted.
//
// Nothing here presses, writes or authorises anything.

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

func reportLocalOutcomes(dir string, limit int) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	if limit > 0 && limit < len(frames) {
		frames = frames[:limit]
	}
	fmt.Printf("%s: %d frame(s)\n", dir, len(frames))

	regions, readable := proposeAll(dir, frames)
	words := readPositions(dir, frames)
	var pressed []pressLook
	allInputs := everyInputLook(dir, frames)
	for look, ats := range pressesByLook(dir, frames) {
		for _, at := range ats {
			pressed = append(pressed, pressLook{look: look, at: at})
		}
	}
	if len(pressed) == 0 {
		fmt.Printf("  0 press(es) with a position. Neither question can be asked: a " +
			"keyboard-only capture has no demonstrated click to verify.\n")
		return nil
	}

	fmt.Printf("\n  the words elsewhere on the window, %d look(s) before the press "+
		"against %d look(s) after it, skipping %d\n",
		OutcomeSettleLooks, OutcomeSettleLooks, OutcomeSkipLooks)
	fmt.Printf("  `elsewhere` is outside EVERY proposal that contained the press, so the " +
		"outcome\n  cannot be the evidence the action site was approved from\n")

	// ── THE RULE, UNDER EVERY SET OF TERMS, ON THIS RECORDING ──
	//
	// One line per candidate rule, with BOTH sides on it: how many of the demonstrated
	// presses it verifies, and how many windows with no input of any kind in them it
	// reports an outcome for. A rule is only worth having if the second is zero, and only
	// worth having if the first is not.
	//
	// Measured on more than one recording, because the first version was measured on one
	// and was wrong: see outcomeRule.
	rules := []outcomeRule{
		{name: "one word, any readings", minLooks: 1, floor: 1},
		{name: "five words", minLooks: 1, floor: OutcomeWordFloor},
		{name: "five words, absent before", minLooks: 1, absent: true, floor: OutcomeWordFloor},
		{name: "five words, dense readings", minLooks: OutcomeSettleLooks, floor: OutcomeWordFloor},
		{name: "five words, absent + dense", minLooks: OutcomeSettleLooks, absent: true,
			floor: OutcomeWordFloor},
		{name: "ten words, absent + dense", minLooks: OutcomeSettleLooks, absent: true,
			floor: 10},
		// AND THE SAME TERMS WITH THE WINDOW COUNTED IN READINGS, which is the
		// unit the live path can actually supply. See outcomeRule.byReadings.
		{name: "five words, absent, 5 readings", minLooks: OutcomeSettleLooks,
			absent: true, floor: OutcomeWordFloor, byReadings: true},
		{name: "ten words, absent, 5 readings", minLooks: OutcomeSettleLooks,
			absent: true, floor: 10, byReadings: true},
	}
	fmt.Printf("\n  %s\n  EVERY SET OF TERMS, ON THIS RECORDING\n  %s\n",
		strings.Repeat("-", 72), strings.Repeat("-", 72))
	fmt.Printf("\n  %-30s %8s %9s  %s\n", "terms", "verified", "false", "of the presses")
	for _, r := range rules {
		verified, which := 0, []string{}
		for i, p := range pressed {
			o := outcomeAt(p.look, p.at, regions, words, readable, r)
			if o.verifiedUnder(r) {
				verified++
				if len(which) < 8 {
					which = append(which, fmt.Sprintf("%d", i+1))
				}
			}
		}
		false_, quiet := 0, 0
		for look := OutcomeSettleLooks; look < len(frames)-OutcomeSettleLooks-OutcomeSkipLooks-1; look++ {
			if nearAnyInput(look, allInputs) {
				continue
			}
			o := quietOutcome(look, regions, words, readable, r)
			if o.readableBefore == 0 || o.readableAfter == 0 {
				continue
			}
			quiet++
			if o.verifiedUnder(r) {
				false_++
			}
		}
		mark := ""
		if false_ == 0 && verified > 0 {
			mark = "  <- sound on this recording"
		}
		fmt.Printf("  %-30s %4d/%-3d %5d/%-4d %s%s\n", r.name, verified, len(pressed),
			false_, quiet, strings.Join(which, ","), mark)
	}

	// ── 1. THE DEMONSTRATED PRESSES, under the strictest terms ──
	strict := rules[len(rules)-2]
	fmt.Printf("\n  %s\n  THE DEMONSTRATED PRESSES — %s\n  %s\n",
		strings.Repeat("-", 72), strict.name, strings.Repeat("-", 72))
	verified, refused := 0, 0
	for i, p := range pressed {
		o := outcomeAt(p.look, p.at, regions, words, readable, strict)
		fmt.Printf("\n  press %d at %.3f,%.3f on look %d\n", i+1, p.at.X, p.at.Y, p.look)
		// AND WHETHER THIS PART OF THE SCREEN HOLDS STILL AT ALL, which decides
		// whether any outcome here means anything. See stationaryNear.
		q, f := stationaryNear(p.look, allInputs, regions, words, readable, strict)
		if q > 0 {
			say := "holds still"
			if f > 0 {
				say = "DOES NOT hold still, so an outcome here means nothing"
			}
			fmt.Printf("    nearby: %d of %d quiet window(s) reported an "+
				"outcome — %s\n", f, q, say)
		}
		printOutcome(o, strict)
		if o.verifiedUnder(strict) {
			verified++
		} else {
			refused++
		}
	}
	//
	// The purest form of the question: nothing was approved, so nothing is excluded and
	// the whole window is evidence. It is also the case that exposed the baseline bug —
	// Ark's pause menu opens on a keypress, and a measurement that only knew about the
	// mouse called that a false outcome.
	if keys := keyLooks(dir, frames); len(keys) > 0 {
		fmt.Printf("\n  %s\n  THE KEYPRESSES — no action site, so nothing is "+
			"excluded\n  %s\n",
			strings.Repeat("-", 72), strings.Repeat("-", 72))
		for _, look := range sortedLooks(keys) {
			o := quietOutcome(look, regions, words, readable, strict)
			fmt.Printf("\n  %q on look %d\n", keys[look], look)
			printOutcome(o, strict)
		}
	}

	// ── 3. NO ACTION ──
	//
	// Every look that is far enough from every press to be quiet, sampled. This is the
	// term that decides whether the rule means anything: a menu whose own animation
	// produces new words would verify an outcome nobody caused.
	fmt.Printf("\n  %s\n  NO ACTION — the same question where nobody pressed anything\n"+
		"  %s\n", strings.Repeat("-", 72), strings.Repeat("-", 72))
	quiet, fired, firedAt := 0, 0, []int{}
	for look := OutcomeSettleLooks; look < len(frames)-OutcomeSettleLooks-OutcomeSkipLooks-1; look++ {
		if nearAnyInput(look, allInputs) {
			continue
		}
		o := quietOutcome(look, regions, words, readable, strict)
		if o.readableBefore == 0 || o.readableAfter == 0 {
			continue
		}
		quiet++
		if len(o.appeared) > 0 {
			fired++
			if len(firedAt) < 6 {
				firedAt = append(firedAt, look)
			}
		}
	}
	fmt.Printf("\n  %d quiet window(s) with readings on both sides, %d of them reported "+
		"an outcome\n", quiet, fired)
	if fired > 0 {
		fmt.Printf("    at look(s) %v — each one is a false outcome, and the rule is "+
			"only worth\n    having if this is zero\n", firedAt)
		// AND WHAT THEY SAID, because a false outcome's words say which part of the
		// menu is not holding still.
		for _, look := range firedAt[:min(2, len(firedAt))] {
			o := quietOutcome(look, regions, words, readable, strict)
			fmt.Printf("    look %d appeared: %s\n", look,
				strings.Join(o.appeared, " "))
		}
	} else {
		fmt.Printf("    no quiet window produced an outcome, so a word that appears " +
			"and stays\n    is evidence about something that happened\n")
	}

	// ── 3. THE TALLY ──
	fmt.Printf("\n  %s\n  %d press(es): %d verified, %d refused. %d false outcome(s) in "+
		"%d quiet window(s).\n", strings.Repeat("-", 72),
		len(pressed), verified, refused, fired, quiet)
	return nil
}

// printOutcome says what one press's outcome evidence was.
func printOutcome(o localOutcome, rule outcomeRule) {
	fmt.Printf("    %d proposal(s) contained it; %d look(s) readable before, %d after\n",
		len(o.excluded), o.readableBefore, o.readableAfter)
	if len(o.siteWords) > 0 {
		fmt.Printf("    inside the control  %s\n", capped(o.siteWords, 8))
	}
	if len(o.appeared) > 0 {
		fmt.Printf("    appeared and stayed %s\n", capped(o.appeared, 10))
	}
	if len(o.vanished) > 0 {
		fmt.Printf("    stopped being read  %s\n", capped(o.vanished, 10))
	}
	if len(o.transient) > 0 {
		fmt.Printf("    transient           %s\n", capped(o.transient, 8))
	}
	fmt.Printf("    %s\n", outcomeVerdict(o, rule))
}

// capped is a word list, bounded so one line stays one line.
func capped(words []string, most int) string {
	if len(words) <= most {
		return strings.Join(words, " ")
	}
	return fmt.Sprintf("%s +%d more", strings.Join(words[:most], " "), len(words)-most)
}

// pressLook is one demonstrated press and the look it landed on.
type pressLook struct {
	look int
	at   observe.PointerAt
}

// nearAnyInput reports whether a look's windows would overlap any observed input.
//
// ANY input, pointer or key. See everyInputLook: a baseline that excluded only the mouse
// counted the pause menu opening as a false outcome, and a baseline that counts a real outcome
// as a false one argues for a rule that would reject real outcomes.
func nearAnyInput(look int, inputs []int) bool {
	const pad = OutcomeSettleLooks + OutcomeSkipLooks + 1
	for _, at := range inputs {
		if look >= at-pad && look <= at+pad {
			return true
		}
	}
	return false
}

// everyInputLook is the look each observed input landed on, pointer or key.
//
// # Why the no-action baseline needs ALL of them
//
// The first measurement excluded only the looks near a POINTER press, and the baseline came
// back with 6 false outcomes that no word floor could remove. Three of them were looks 36, 37
// and 38 — and `input.jsonl` has `{"intent":"pause","at_ms":3664}`, which at ten frames a
// second is look 36. Those were not false outcomes at all: they were Ark's pause menu opening,
// correctly detected, in a window the measurement had called quiet because the person had used
// the keyboard rather than the mouse.
//
// A baseline that counts a real outcome as a false one argues for a rule that would reject real
// outcomes. So the quiet windows are the ones with no input of ANY kind in them, and the
// keyboard press gets asked as its own case below.
func everyInputLook(dir string, frames []frameRecord) []int {
	inputs, _ := readJSONL[inputRecord](filepath.Join(dir, "input.jsonl"))
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].AtMS < inputs[j].AtMS })
	var out []int
	next := 0
	for i, fr := range frames {
		for next < len(inputs) && inputs[next].AtMS <= fr.AtMS {
			next++
			out = append(out, i)
		}
	}
	return out
}

// keyLooks is the looks that carried a NON-pointer input, with its intent.
//
// Asked as its own case: a keypress has no action site at all, so its outcome is the purest
// form of this question — nothing was approved, nothing is excluded, and the whole window is
// evidence.
func keyLooks(dir string, frames []frameRecord) map[int]string {
	inputs, _ := readJSONL[inputRecord](filepath.Join(dir, "input.jsonl"))
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].AtMS < inputs[j].AtMS })
	out := map[int]string{}
	next := 0
	for i, fr := range frames {
		for next < len(inputs) && inputs[next].AtMS <= fr.AtMS {
			in := inputs[next]
			next++
			if observe.NavIntent(in.Intent) != observe.NavPoint {
				out[i] = in.Intent
			}
		}
	}
	return out
}

// sortedLooks orders a look-keyed map so two runs print it the same way.
func sortedLooks[V any](m map[int]V) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
