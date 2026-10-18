package main

// THE MEASURED CLICKS, AND WHAT EACH RULE WOULD HAVE CALLED THEM.
//
// [[Experiment-083-the-label-of-the-button-you-click-is-the-one-that-is-missing]] ran the
// button-level question over `rl-freeplay-1`, which is the only recording with frames, click
// positions and OCR word boxes together. Two of its seventeen presses land inside a button whose
// boundary a person established — [[Experiment-050-two-readings-of-one-button]]'s PLAY entry,
// 166x40 at (26,468) — and they are the only two presses anywhere with an independent answer.
//
// The numbers below are copied from that run. They are here because the conclusion is the
// opposite of the one the investigation set out to confirm, and a conclusion like that should be
// a test rather than a paragraph.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

func clickAt(x, y float64) observe.PointerAt { return observe.PointerAt{X: x, Y: y} }

func box(x, y, w, h float64) observe.Region {
	return observe.Region{X: x, Y: y, Width: w, Height: h}
}

// THE EXISTING RULE GETS THE BUTTON AND BOTH TEXT RULES GET IT WRONG.
//
// Press 1 of the recording, at 0.065,0.444 — inside PLAY. The proposer offered the PLAY button
// at 8.6%x3.7%, OCR read no word inside it, and the nearest word anywhere on the look was ITEM:
// the label of the entry BELOW it.
func TestOnTheOneKnownButtonTheProposalIsRightAndTheNearestWordIsAnotherControl(t *testing.T) {
	click := clickAt(0.065, 0.444)
	// What the proposer offered, as measured: the button, a wider reading of it, and the
	// panel holding the whole menu.
	regions := []observe.Region{
		box(0.014, 0.433, 0.086, 0.037),
		box(0.007, 0.433, 0.165, 0.041),
		box(0.000, 0.200, 0.617, 0.578),
	}
	// What OCR read, as measured: every entry of the menu EXCEPT the one being clicked.
	words := []observe.ReadWord{
		{Text: "ITEM", Where: box(0.042, 0.492, 0.022, 0.013)},
		{Text: "SHOP", Where: box(0.068, 0.492, 0.024, 0.013)},
		{Text: "GARAGE", Where: box(0.042, 0.560, 0.038, 0.013)},
		{Text: "SETTINGS", Where: box(0.042, 0.700, 0.044, 0.013)},
	}
	e := buttonEvidence(73, click, regions, words, 72)

	// THE EXISTING RULE IS RIGHT. The smallest proposal containing the click is the
	// button a person measured, by the relation that decides whether two readings are
	// one control.
	if !e.today.found || !observe.SameRegion(e.today.where, playButton) {
		t.Fatalf("today's answer is %v, and the measured PLAY button is %v",
			e.today.where, playButton)
	}

	// AND THE TEXT EVIDENCE IS NOT THERE. No word box inside the button, so the
	// one-label rule abstains and the click is in padding.
	if e.oneLabel.found {
		t.Errorf("a one-label boundary was found: %s", e.oneLabel)
	}
	if !e.padding {
		t.Error("the click is reported as being inside a word box")
	}

	// AND THE RULE THE OWNER FORBADE WOULD HAVE NAMED A DIFFERENT CONTROL. This is the
	// measurement that puts a number under *do not assume the nearest word is the
	// target*: ITEM belongs to the entry below PLAY.
	if !e.anywhere.found || e.anywhere.words[0] != "ITEM" {
		t.Fatalf("the nearest word anywhere is %s", e.anywhere)
	}
	// AND HOW FAR AWAY IT IS, because "the nearest word is another control's" is only a
	// finding if the distance is reported: a label 0.055 of the window away from the
	// click is not a near miss, it is the next row down.
	if e.anywhereAt < 0.05 || e.anywhereAt > 0.06 {
		t.Errorf("the nearest word's centre is reported %.3f from the click; the "+
			"measured distance to ITEM is 0.055", e.anywhereAt)
	}
	if observe.SameRegion(e.anywhere.where, playButton) {
		t.Error("the nearest word's box is being treated as the same control as the " +
			"button, which would hide the error this test exists to record")
	}
	if v := buttonVerdict(e); !strings.Contains(v, "ABSTAIN") {
		t.Errorf("verdict %q; with no label on the button there is nothing to be sure "+
			"of", v)
	}
}

// AND THE WORD BOX UNDER THE CLICK CAN BE A SUB-LABEL, WHICH IS A DIFFERENT WRONG ANSWER.
//
// Press 10, at 0.083,0.462 — also inside PLAY. Here OCR did read inside the proposal, and what
// it read was "fHITTHE FIELD": its garbled version of the entry's second line. The word box
// holding the click is FIELD, which is not what the control is called.
func TestAWordBoxUnderTheClickCanBeASubLabelRatherThanTheControlsName(t *testing.T) {
	click := clickAt(0.083, 0.462)
	regions := []observe.Region{box(0.007, 0.433, 0.184, 0.041)}
	words := []observe.ReadWord{
		{Text: "fHITTHE", Where: box(0.030, 0.459, 0.030, 0.008)},
		{Text: "FIELD", Where: box(0.068, 0.459, 0.017, 0.008)},
	}
	e := buttonEvidence(539, click, regions, words, 540)

	if !e.today.found || !observe.SameRegion(e.today.where, playButton) {
		t.Fatalf("today's answer %v is not the measured PLAY button", e.today.where)
	}
	if !e.wordBox.found || e.wordBox.words[0] != "FIELD" {
		t.Fatalf("the word box under the click is %s", e.wordBox)
	}
	// TWO LABELS IN ONE BUTTON, so the one-label rule abstains rather than choosing
	// between them — which is the behaviour being asked for and not a shortcoming.
	if e.oneLabel.found {
		t.Errorf("one of two labels in one rectangle was chosen: %s", e.oneLabel)
	}
	if e.padding {
		t.Error("this click IS inside a word box and is reported as padding")
	}
}

// AND A BUTTON WITH ONE LABEL IN IT IS WHAT THE RULE IS FOR.
//
// The case that does not occur in this recording, so that the rule is known to work and the
// 16-of-17 abstention is known to be a statement about the evidence rather than about the code.

func TestAPanelNarrowsToTheProposalHoldingOneLabel(t *testing.T) {
	click := clickAt(0.5, 0.5)
	panel := box(0.1, 0.1, 0.8, 0.8)
	button := box(0.45, 0.48, 0.12, 0.05)
	e := buttonEvidence(1, click, []observe.Region{panel, button}, []observe.ReadWord{
		{Text: "resume", Where: box(0.47, 0.49, 0.06, 0.02)},
		{Text: "quit", Where: box(0.47, 0.70, 0.04, 0.02)},
	}, 1)

	if !e.oneLabel.found || e.oneLabel.words[0] != "resume" {
		t.Fatalf("one-label is %s", e.oneLabel)
	}
	if !sameRegionExactly(e.oneLabel.where, button) {
		t.Errorf("one-label chose %v, not the button %v", e.oneLabel.where, button)
	}
	// AND BECAUSE THE SMALLEST CONTAINING PROPOSAL IS ALREADY THAT BUTTON, THIS IS
	// `BUTTON` AND NOT `NARROWED`. The distinction is the whole measurement: NARROWED is
	// the panel case being rescued, and it happened zero times.
	if v := buttonVerdict(e); !strings.Contains(v, "BUTTON") {
		t.Errorf("verdict %q", v)
	}

	// NOW HIDE THE BUTTON FROM THE PROPOSER, which is the Ark case: the panel is the
	// only thing containing the click, and one label inside it is all there is to go on.
	only := buttonEvidence(1, click, []observe.Region{panel}, []observe.ReadWord{
		{Text: "resume", Where: box(0.47, 0.49, 0.06, 0.02)},
	}, 1)
	if v := buttonVerdict(only); !strings.Contains(v, "BUTTON") {
		t.Errorf("verdict %q — the panel holds exactly one label, so it is the only "+
			"boundary there is, and calling it a button is what makes the one-label "+
			"rule insufficient on its own", v)
	}
	if only.today.where.Width < 0.5 {
		t.Error("the fixture is not a panel")
	}
}

// A WORD BOX IS ON A RECTANGLE BY ITS CENTRE, NOT BY A CORNER.
//
// OCR boxes overrun their glyphs, so a label two pixels wider than its button would never be
// counted as being on it under whole containment — and a box counted by its top-left corner
// would be claimed by whichever rectangle its corner happens to fall in, which for a label that
// straddles a boundary is the one ABOVE the control it belongs to.
func TestAWordIsOnTheRectangleItsCentreIsIn(t *testing.T) {
	// A LABEL STRADDLING THE TOP EDGE: its corner is in the upper rectangle and its
	// centre is in the lower one, which is the control it reads for.
	upper := box(0.1, 0.10, 0.4, 0.10)
	lower := box(0.1, 0.20, 0.4, 0.10)
	label := observe.ReadWord{Text: "resume", Where: box(0.2, 0.19, 0.1, 0.04)}

	if inside(upper, label) {
		t.Error("the label is counted as being on the rectangle its CORNER is in, so " +
			"every straddling label belongs to the control above its own")
	}
	if !inside(lower, label) {
		t.Error("the label is not counted as being on the rectangle its centre is in")
	}
	// AND IT IS NOT WHOLE CONTAINMENT EITHER: this label is not wholly inside `lower`,
	// and a rule that demanded it would read nothing on any button.
	if label.Where.Y >= lower.Y {
		t.Fatal("the fixture no longer straddles the edge")
	}
}
