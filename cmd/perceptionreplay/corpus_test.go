package main

import (
	"os"
	"path/filepath"
	"testing"
)

// THE REAL-PIXEL REGRESSION, when the recording is on the machine.
//
// A perception change has to be measurable against real frames rather than drawn ones, and the
// recording that makes that possible is 1.4GB of a person's game session. It is private, it is
// gitignored, and it cannot be in the repository. So this test skips when it is absent and
// asserts the measured numbers when it is present.
//
// # Why floors and not exact values
//
// The numbers below were measured, and a change that improves them should not fail. A change
// that quietly loses them must. The end-to-end number is the one to watch: a component metric
// can improve while the pipeline regresses, and this corpus has already produced exactly that —
// a human-scale floor on proposals cut proposals per frame by 18% and cut frames naming an
// entry from 148 to 12.
//
// # What a pass here does NOT establish
//
// That Marco can interact with a live application. This is a replay: it measures how well
// Marco interprets a recording. It also scores against model-assisted labels, which the
// annotation file says of itself must not be presented as independently validated.

// corpusDir finds a local L0 recording, or returns "".
//
// `go test` runs in the package directory, so the repo-relative scratch path has to be climbed
// to rather than assumed — a path that silently misses makes this test skip and a regression
// pass. $MARCO_FRAME_CORPUS overrides, so a recording kept anywhere can be used.
func corpusDir() string {
	if p := os.Getenv("MARCO_FRAME_CORPUS"); p != "" {
		if _, err := os.Stat(filepath.Join(p, "frames.jsonl")); err == nil {
			return p
		}
		return ""
	}
	const rel = ".tmp/game-capture-review/tm-menu-steps-sanitized"
	for _, up := range []string{rel, "../" + rel, "../../" + rel, "../../../" + rel} {
		if _, err := os.Stat(filepath.Join(up, "frames.jsonl")); err == nil {
			return up
		}
	}
	return ""
}

// At a coarse cadence this corpus identifies the menu, which it did not.
//
// # What this replaced
//
// Every tenth frame used to be the gate, and it ratcheted 22 correct-list frames of 83 and
// allowed 33 wrong ones. Those 22 were not an identification. The only candidate that qualified
// at that cadence was a five-row bucket whose final extent was y 0.078..0.355, whose hull —
// the union of every extent it had ever held — was y 0.000..0.898, and which had changed area
// by more than a factor of two 21 times. The menu is at y 0.528..0.857. It scored "correct" on
// 22 frames and "wrong" on 33 by drifting across the menu on its way past.
//
// # Why nothing qualified, and what changed
//
// A candidate has to be found again to accumulate anything, and at 660ms between looks the
// mark has usually moved between them — so the menu's candidate reached 28 exclusive frames
// against 11 shared and fell short of the exclusivity rule. That was reported as an honest
// abstention and it was also a production blocker, because live observation runs near this rate.
//
// What changed is grouping, not the cadence. `alike` now admits a shared RIGHT edge, so the
// menu is grouped whole instead of being cut in half by one partially found row — and a whole
// menu has enough exclusive frames to qualify. At every tenth frame it is now identified on 29
// of 83 labelled frames and the reader names 21 entries, all 21 of them the labelled row.
// [[ADR-166-a-list-shares-an-edge-and-a-partial-row-keeps-one]].
//
// # What this test is for
//
// A floor on identification at a realistic rate, and a ceiling of zero on wrong lists there.
func TestAtACoarseCadenceTheCorpusIdentifiesTheMenu(t *testing.T) {
	dir := corpusDir()
	if dir == "" {
		t.Skip("no local L0 recording found; set $MARCO_FRAME_CORPUS to run it.")
	}
	r, err := replayFrames(dir, 10, 0, false)
	if err != nil {
		t.Fatalf("replaying the local corpus: %v", err)
	}
	ls := r.Lists
	if ls.Wrong > 0 {
		t.Errorf("the belief was published about a structure that is not the menu on %d "+
			"of %d labelled frames at this cadence. Abstaining is the answer here "+
			"until a candidate behaves like the menu; a wrong list is wrong before "+
			"the reader is consulted", ls.Wrong, ls.Labelled)
	}
	const wantCoarseCorrect = 29
	if ls.Correct < wantCoarseCorrect {
		t.Errorf("the menu was identified on %d of %d labelled frames at every tenth "+
			"frame, against %d measured. This is the cadence live observation runs "+
			"at, so it is the one that decides whether any of this is usable",
			ls.Correct, ls.Labelled, wantCoarseCorrect)
	}
	const wantCoarseRight = 21
	if sc := r.Score; sc.Right < wantCoarseRight {
		t.Errorf("%d of %d namings at every tenth frame were the labelled row, against "+
			"%d measured", sc.Right, sc.Named, wantCoarseRight)
	}
	// AND THE ABSTENTION HAS TO BE READABLE. "No list has been seen handing a mark" with
	// no further detail is the sentence Experiment-025 ended on, and telling the five
	// possible causes apart meant regrouping the run's geometry by hand afterwards.
	if r.Identified < 0 && len(r.Candidates) == 0 {
		t.Errorf("nothing was identified and no candidate was reported, so the "+
			"abstention cannot be read: %s", r.Belief.Because)
	}
	var withShortfall int
	for _, c := range r.Candidates {
		if len(c.Unmet) > 0 {
			withShortfall++
		}
	}
	if r.Identified < 0 && withShortfall == 0 {
		t.Errorf("%d candidate(s) were tracked, nothing was identified, and not one "+
			"names a requirement it fell short of", len(r.Candidates))
	}
	t.Logf("every 10th frame: %d correct, %d wrong, %d none of %d labelled; identified "+
		"after %d; %d candidate(s), %d naming a shortfall",
		ls.Correct, ls.Wrong, ls.None, ls.Labelled, r.Identified, len(r.Candidates),
		withShortfall)
}
