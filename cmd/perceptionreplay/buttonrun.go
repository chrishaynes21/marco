package main

// Running the button-level target question over a recording, and scoring it.
//
// See button.go for the question and for what this cannot measure. Three verdicts per press, and
// the middle one is the point of the exercise:
//
//	BUTTON     today's answer already holds exactly one label. Nothing to improve.
//	NARROWED   today's answer holds several labels and a smaller proposal holds exactly one.
//	           The panel case, with a defensible boundary inside it.
//	ABSTAIN    no proposal containing the click holds exactly one label. Reported as an
//	           abstention and never resolved by proximity.

import (
	"fmt"
	"path/filepath"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// reportButtonTargets measures whether a demonstrated click can be resolved to one button.
func reportButtonTargets(dir string, limit int) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	if limit > 0 && limit < len(frames) {
		frames = frames[:limit]
	}
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("CAN THE CLICKED BUTTON BE FOUND INSIDE THE PANEL? %s, %d frame(s)\n\n",
		filepath.Base(dir), len(frames))

	presses := pressesByLook(dir, frames)
	words := readPositions(dir, frames)
	regions, readable := proposeAll(dir, frames)

	readLooks := 0
	for _, w := range words {
		if len(w) > 0 {
			readLooks++
		}
	}
	total := 0
	for _, p := range presses {
		total += len(p)
	}
	fmt.Printf("  %d press(es) with a position, %d look(s) carrying word boxes\n\n",
		total, readLooks)
	if total == 0 {
		fmt.Printf("  NO PRESS IN THIS RECORDING HAS A POSITION. A keyboard-only capture\n")
		fmt.Printf("  says nothing about where a control is; this needs pointer presses.\n")
		return nil
	}
	if readLooks == 0 {
		fmt.Printf("  NO READING KEPT WHERE ITS PHRASES WERE. Delete words.jsonl and run\n")
		fmt.Printf("  `-frames -semantic` again; a vocabulary alone cannot locate a label.\n")
		return nil
	}

	var all []pressEvidence
	for look, at := range presses {
		for _, p := range at {
			if !readable[look] {
				continue
			}
			w, from := nearestReading(words, look)
			all = append(all, buttonEvidence(look, p, regions[look], w, from))
		}
	}
	printButtonEvidence(all)
	printButtonTally(all)
	printPlayComparison(all)
	return nil
}

// MaxReadingGap is how many looks away a reading may be taken from.
//
// Not a tuning knob: a menu that is not moving reads the same on the neighbouring look, and OCR
// here answered on 300 looks of 900. A press whose nearest reading is further than this is
// reported as having none rather than matched against a screen that may have changed, and the
// gap used is printed on every row so a reader can discount any of them.
const MaxReadingGap = 5

// nearestReading is the words of the closest look that kept any, and which look that was.
func nearestReading(words [][]observe.ReadWord, look int) ([]observe.ReadWord, int) {
	for d := 0; d <= MaxReadingGap; d++ {
		for _, i := range []int{look - d, look + d} {
			if i >= 0 && i < len(words) && len(words[i]) > 0 {
				return words[i], i
			}
		}
	}
	return nil, -1
}

func printButtonEvidence(all []pressEvidence) {
	for i, e := range all {
		gap := "same look"
		switch {
		case e.readLook < 0:
			gap = fmt.Sprintf("NO READING within %d looks", MaxReadingGap)
		case e.readLook != e.look:
			gap = fmt.Sprintf("reading from look %d", e.readLook)
		}
		fmt.Printf("  press %d at %.3f,%.3f on look %d   [%s]\n",
			i+1, e.at.X, e.at.Y, e.look, gap)
		if len(e.proposals) == 0 {
			fmt.Printf("      no proposal contains it — the click landed on nothing " +
				"the proposer found\n\n")
			continue
		}
		fmt.Printf("      %d proposal(s) contain it, smallest first:\n", len(e.proposals))
		for j, p := range e.proposals {
			if j >= 4 {
				fmt.Printf("          ... and %d larger\n", len(e.proposals)-j)
				break
			}
			fmt.Printf("          %5.1f%%x%5.1f%%  %2d word(s) %v\n",
				p.Width*100, p.Height*100, len(e.wordsIn[j]), firstFew(e.wordsIn[j]))
		}
		for _, c := range []candidate{e.today, e.oneLabel, e.wordBox, e.nearest, e.anywhere} {
			fmt.Printf("      %s\n", c)
		}
		if e.anywhere.found {
			fmt.Printf("                  its centre is %.3f from the click\n",
				e.anywhereAt)
		}
		fmt.Printf("      %s", buttonVerdict(e))
		if e.padding {
			fmt.Printf("   [click in PADDING — no word box holds it]")
		}
		if e.twins > 0 {
			fmt.Printf("   [%d other proposal(s) of the same shape on this look]",
				e.twins)
		}
		fmt.Printf("\n\n")
	}
}

// verdictOf is the three-way answer for one press.
func buttonVerdict(e pressEvidence) string {
	switch {
	case !e.today.found:
		return "ABSTAIN  — nothing contains the click"
	case !e.oneLabel.found:
		return "ABSTAIN  — no proposal containing the click holds exactly one label"
	case sameRegionExactly(e.oneLabel.where, e.today.where):
		return "BUTTON   — today's answer already holds exactly one label"
	default:
		return fmt.Sprintf("NARROWED — from %.1f%%x%.1f%% to %.1f%%x%.1f%% on %q",
			e.today.where.Width*100, e.today.where.Height*100,
			e.oneLabel.where.Width*100, e.oneLabel.where.Height*100,
			e.oneLabel.words[0])
	}
}

func printButtonTally(all []pressEvidence) {
	var button, narrowed, abstain, padding, padFound, twins int
	for _, e := range all {
		switch {
		case !e.oneLabel.found:
			abstain++
		case sameRegionExactly(e.oneLabel.where, e.today.where):
			button++
		default:
			narrowed++
		}
		if e.padding {
			padding++
			if e.oneLabel.found {
				padFound++
			}
		}
		if e.twins > 0 {
			twins++
		}
	}
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  %d press(es)\n", len(all))
	fmt.Printf("    %2d BUTTON    today's answer already holds exactly one label\n", button)
	fmt.Printf("    %2d NARROWED  a smaller proposal holds exactly one\n", narrowed)
	fmt.Printf("    %2d ABSTAIN   no proposal containing the click holds exactly one\n",
		abstain)
	fmt.Printf("\n  %d press(es) landed in PADDING — no word box held the click — and a\n",
		padding)
	fmt.Printf("  one-label boundary was still found for %d of them. That is the case\n",
		padFound)
	fmt.Printf("  that would defeat taking the label under the cursor as the target.\n")
	fmt.Printf("\n  %d press(es) resolved to a shape that another proposal on the same look\n",
		twins)
	fmt.Printf("  also has. A boundary is not an identity: see Experiment 077.\n")
}

// printPlayComparison scores the one press with a known answer.
func printPlayComparison(all []pressEvidence) {
	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("  AGAINST THE ONE KNOWN BUTTON — Experiment 050's PLAY entry, 8.6%%x3.7%%\n")
	fmt.Printf("  at 0.014,0.433, established by a person from two of their own presses.\n\n")
	found := 0
	for i, e := range all {
		if !holds(playButton, e.at) {
			continue
		}
		found++
		fmt.Printf("    press %d at %.3f,%.3f is inside it\n", i+1, e.at.X, e.at.Y)
		for _, c := range []candidate{e.today, e.oneLabel} {
			if !c.found {
				fmt.Printf("      %-10s —  no answer\n", c.name)
				continue
			}
			agree := "DIFFERENT shape"
			if observe.SameRegion(c.where, playButton) {
				agree = "SAME control by the identity relation"
			}
			fmt.Printf("      %-10s %5.1f%%x%5.1f%%  %s\n", c.name,
				c.where.Width*100, c.where.Height*100, agree)
		}
	}
	if found == 0 {
		fmt.Printf("    no press in this recording lands inside it, so this recording\n")
		fmt.Printf("    carries no press whose answer is independently known.\n")
	}
}

func firstFew(words []string) []string {
	if len(words) <= 3 {
		return words
	}
	out := append([]string{}, words[:3]...)
	return append(out, fmt.Sprintf("+%d more", len(words)-3))
}
