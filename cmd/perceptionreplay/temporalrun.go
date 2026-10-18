package main

// Running the before/at/after question over a recording, and scoring it.
//
// See temporal.go for the question. One line per look per press, because the point of the
// exercise is whether the answer MOVES, and a tally that said "stable" without showing the
// eleven looks it was stable across would be the same unmeasured assertion this is replacing.

import (
	"fmt"
	"path/filepath"
)

func reportTemporalTargets(dir string, limit int) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	if limit > 0 && limit < len(frames) {
		frames = frames[:limit]
	}
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("DOES THE BOUNDARY DEPEND ON WHEN YOU LOOK? %s, %d frame(s)\n\n",
		filepath.Base(dir), len(frames))

	presses := pressesByLook(dir, frames)
	total := 0
	for _, p := range presses {
		total += len(p)
	}
	if total == 0 {
		fmt.Printf("  NO PRESS IN THIS RECORDING HAS A POSITION. This question is about\n")
		fmt.Printf("  where a control is, and a keyboard-only capture says nothing about it.\n")
		return nil
	}
	fmt.Printf("  %d press(es) with a position, asked of +/-%d look(s) each\n\n",
		total, TemporalWindow)

	regions, readable := proposeAll(dir, frames)

	var all []temporalEvidence
	for look, at := range presses {
		for _, p := range at {
			all = append(all, temporalBoundaries(look, p, regions, readable))
		}
	}
	printTemporalEvidence(all)
	printTemporalTally(all)
	return nil
}

func printTemporalEvidence(all []temporalEvidence) {
	for i, e := range all {
		fmt.Printf("  press %d at %.3f,%.3f on look %d%s\n",
			i+1, e.at.X, e.at.Y, e.look, oracleNote(e))
		for _, b := range e.window {
			here := "   "
			if b.look == e.look {
				here = "<- "
			}
			switch {
			case !b.readable:
				fmt.Printf("      %slook %4d   no pixels\n", here, b.look)
			case !b.held:
				fmt.Printf("      %slook %4d   nothing contains the click\n",
					here, b.look)
			default:
				mark := ""
				if b.isOracle {
					mark = "   = the known PLAY button"
				}
				fmt.Printf("      %slook %4d  %5.1f%%x%5.1f%% at %.3f,%.3f%s\n",
					here, b.look, b.where.Width*100, b.where.Height*100,
					b.where.X, b.where.Y, mark)
			}
		}
		fmt.Printf("      %s\n", temporalVerdict(e))
		if e.inOracle {
			fmt.Printf("      oracle   %d of %d readable look(s) offer the known button\n",
				e.oracleLooks, e.oracleOfHowMany)
		}
		fmt.Printf("\n")
	}
}

func oracleNote(e temporalEvidence) string {
	if e.inOracle {
		return "   [inside the known PLAY button]"
	}
	return ""
}

func printTemporalTally(all []temporalEvidence) {
	var stable, before, after, appears, nothing int
	var clickBest, nonTransitive int
	var oracleIn, oracleAll, oracleHit, oracleOf, oracleAtClick int
	for _, e := range all {
		switch {
		case !e.atClick.held && !e.hasTighter:
			nothing++
		case !e.atClick.held:
			appears++
		case e.hasTighter && tighterIsBefore(e):
			before++
		case e.hasTighter:
			after++
		default:
			stable++
		}
		if clickLookIsSmallest(e) {
			clickBest++
		}
		if bothAreTheKnownButton(e) {
			nonTransitive++
		}
		if e.inOracle {
			oracleIn++
			if e.atClick.isOracle {
				oracleAtClick++
			}
			oracleHit += e.oracleLooks
			oracleOf += e.oracleOfHowMany
			if e.oracleOfHowMany > 0 && e.oracleLooks == e.oracleOfHowMany {
				oracleAll++
			}
		}
	}
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  %d press(es), each asked of +/-%d look(s)\n", len(all), TemporalWindow)
	fmt.Printf("    %2d STABLE   no neighbouring look offers a smaller, different boundary\n",
		stable)
	fmt.Printf("    %2d TIGHTER BEFORE the press — the same screen, so usable evidence\n",
		before)
	fmt.Printf("    %2d TIGHTER AFTER the press — may be the screen the press caused\n", after)
	fmt.Printf("    %2d APPEARS  nothing held the click on its own look and something did "+
		"nearby\n", appears)
	fmt.Printf("    %2d NOTHING  no look in the window held the click at all\n", nothing)

	fmt.Printf("\n  AND THE OTHER DIRECTION, which is the one the numbers above do not show:\n")
	fmt.Printf("    %d of %d press(es) had the SMALLEST boundary of their whole window on\n",
		clickBest, len(all))
	fmt.Printf("    the click's own look. The highlight that stops OCR reading the label is\n")
	fmt.Printf("    the same highlight that gives the proposer an edge to segment.\n")

	if nonTransitive > 0 {
		fmt.Printf("\n  %d press(es) where the identity relation called two boundaries both\n",
			nonTransitive)
		fmt.Printf("  the known button and not each other. Mutual centre containment is not\n")
		fmt.Printf("  transitive, so a DIFFERENT-control verdict from it is not evidence of\n")
		fmt.Printf("  a different control at this scale.\n")
	}

	if oracleIn == 0 {
		fmt.Printf("\n  No press in this recording lands inside a button whose boundary a\n")
		fmt.Printf("  person established, so nothing here is independently scored.\n")
		return
	}
	fmt.Printf("\n  AGAINST THE ONE KNOWN BUTTON — Experiment 050's PLAY entry.\n")
	fmt.Printf("    %d press(es) land inside it; the proposer offered it on %d of %d\n",
		oracleIn, oracleHit, oracleOf)
	fmt.Printf("    readable look(s) around them, and on EVERY look of %d of them.\n", oracleAll)
	fmt.Printf("\n    AND ON THE CLICK'S OWN LOOK: %d of %d. Where an answer is known, the\n",
		oracleAtClick, oracleIn)
	fmt.Printf("    boundary at the press was ALREADY the right one, so no rule that reaches\n")
	fmt.Printf("    for another frame can be shown better on this evidence — there is nothing\n")
	fmt.Printf("    for it to improve. That is a limit of the oracle, not a result.\n")
	fmt.Printf("\n  TIGHTER BEFORE is the only count that would justify reaching back to an\n")
	fmt.Printf("  earlier frame. %d of %d.\n", before, len(all))
}
