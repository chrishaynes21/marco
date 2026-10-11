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

func TestTheRealMenuCorpusStillReadsAsAMenu(t *testing.T) {
	dir := corpusDir()
	if dir == "" {
		t.Skip("no local L0 recording found; this measurement needs real frames and " +
			"they are private and out of version control. Set $MARCO_FRAME_CORPUS to " +
			"a capture-game-session directory to run it.")
	}
	// Every sixth frame: about 144 of 864, which is enough for the belief to identify a
	// list and still runs in a few seconds. The full-corpus numbers live in ADR-157.
	r, err := replayFrames(dir, 10, 0, false)
	if err != nil {
		t.Fatalf("replaying the local corpus: %v", err)
	}
	if r.Frames < 80 {
		t.Fatalf("only %d frames reprocessed; the recording or the reader is wrong", r.Frames)
	}

	// DISCOVERY, which is where the old detector failed. Every labelled frame must have a
	// proposal covering the selected row.
	if r.LabelledFrames == 0 {
		t.Fatal("no labelled frames, so nothing here is scored")
	}
	if r.CoveredSelected < r.LabelledFrames {
		t.Errorf("a proposal covered the selected row on %d of %d labelled frames. "+
			"Measured at all of them; nothing downstream can recover a row that was "+
			"never offered", r.CoveredSelected, r.LabelledFrames)
	}

	// GROUPING. The old detector's live run produced a usable stack on 42 looks of 107;
	// this must produce one on every frame.
	if r.Grouped < r.Frames {
		t.Errorf("%d of %d frames produced a stack of >=%d. The tracker refuses anything "+
			"smaller, so a frame without one contributes no candidate at all",
			r.Grouped, r.Frames, minUsableStack)
	}

	// IDENTIFICATION, which Experiment-025 never achieved on this game at all.
	if r.Identified < 0 {
		t.Errorf("no list was ever identified in %d frames of a real menu being "+
			"navigated: %s", r.Frames, r.Belief.Because)
	}

	// THE INPUT HALF REACHED THE PERCEPTION. A recording of somebody navigating that
	// attributes nothing has a broken join, and the symptom is a belief that abstains
	// forever between two real lists while the presses sit unread on disk.
	if r.NavsSeen == 0 {
		t.Errorf("not one of the recording's %d presses was attributed to a frame. "+
			"The clock join is broken, and nothing can tell the menu from the panel "+
			"beside it without it", r.Inputs)
	}

	// AND THE BELIEF IS NOT LEFT AMBIGUOUS. Two real lists qualify on this recording --
	// the menu and the track-information panel -- and geometry cannot separate them.
	// What separates them is that one responds to navigation. If this regresses, the
	// belief goes back to naming nothing on a session where it could name the right row.
	if r.Belief.Status == "ambiguous" {
		t.Errorf("the belief ends ambiguous: %s", r.Belief.Because)
	}

	// AND THE END-TO-END NUMBER. Frames on which the belief named an entry, and whether
	// the row it named was the row the label named. The second is the one that makes the
	// first worth anything: naming an entry often and wrongly is worse than abstaining.
	if r.WithSelection == 0 {
		t.Errorf("the belief never named an entry across %d frames", r.Frames)
	}
	// THE FLOOR IS THE MEASUREMENT, and the measurement is not good.
	//
	// # What this used to assert, and why it passed
	//
	// It asserted 90%, and it passed at 99% for two experiments, and the 99% was an
	// artefact of the score. `nearHeight` was handed the identified LIST's envelope rather
	// than the named ENTRY's region — and the list in question spanned three quarters of
	// the screen, so every labelled row on the frame was inside it. The tolerance was also
	// three row-heights wide. Between them, a reading could name a row two places away,
	// in a list that was not the menu, and score.
	//
	// Scored against the named entry at half the labelled pitch — a region is on this row
	// exactly when it is nearer this row than its neighbours — the same corpus gives:
	//
	//	every 10th frame   19 of 38 named the labelled row   50%
	//	every frame        43 of 189                         23%
	//
	// So 90% was never achieved and is kept here as the TARGET, failed, rather than
	// quietly lowered. The floors below are set at what is actually measured, so the next
	// change has to beat it; the gap to the target is the work, and it is recorded in
	// [[Experiment-026-the-same-menu-seen-from-pixels]] and the subsystem note.
	const (
		wantNamed   = 38
		wantMatched = 19
		targetRate  = 0.90
	)
	if r.WithSelection < wantNamed {
		t.Errorf("the belief named an entry on %d frames, against %d measured. Fewer "+
			"readings is a regression even when the ones that remain are right",
			r.WithSelection, wantNamed)
	}
	if r.CorrectSelection < wantMatched {
		t.Errorf("%d of %d named entries were the labelled row, against %d of %d "+
			"measured. This is a ratchet: the floor is the last measurement",
			r.CorrectSelection, r.WithSelection, wantMatched, wantNamed)
	}
	if rate := float64(r.CorrectSelection) / float64(max(r.WithSelection, 1)); rate >= targetRate {
		t.Errorf("the reading now matches the labelled row %.0f%% of the time, which "+
			"clears the %.0f%% target. Raise the floors in this test and say so in the "+
			"experiment record", 100*rate, 100*targetRate)
	}

	// ── COVERAGE AND ACCURACY, WITH THEIR OWN DENOMINATORS ────────────────────────
	//
	// The floors above are kept because they are the ratchet the previous measurement
	// set, and re-basing a ratchet is how a regression passes. They are not, however, a
	// coverage or an accuracy: WithSelection counts namings on every frame and
	// CorrectSelection only on labelled ones, so no ratio of the two means anything. The
	// assertions below are over the populations in score.go, each against its own
	// denominator.
	sc := r.Score
	if sc.Labelled == 0 {
		t.Fatal("the scoring pass found no labelled frames, so every assertion below " +
			"is vacuous. Either the trace is empty or the labels were not read")
	}
	const (
		wantCovered  = 38
		wantRight    = 19
		wantReader   = 19
		wantReaderOf = 20
		readerFloor  = 0.90
	)
	if sc.Named < wantCovered {
		t.Errorf("coverage: %d of %d labelled frames named an entry, against %d "+
			"measured. Naming fewer rows is slower learning even when the ones "+
			"that remain are right", sc.Named, sc.Labelled, wantCovered)
	}
	if sc.Right < wantRight {
		t.Errorf("accuracy: %d of %d namings were the labelled row, against %d "+
			"measured. This is the graph-corrupting one: a wrong row is retained "+
			"as a move between rows the mark never made", sc.Right, sc.Named,
			wantRight)
	}
	// ── THE PARTITION, which is what makes the two halves of coverage readable ────
	//
	// Every labelled frame either named an entry, had no list to name one in, or had a
	// list and no readable row in it. A frame in none of those three is a frame the
	// report lost, and the two halves are what decide where the work is: one is
	// disambiguation and one is the reader, and they are not interchangeable.
	// AND THE POPULATION IS THE LABELLED ONE, exactly. The annotation marks the blend
	// frames `uncertain` on purpose, and those are the frames a reader is expected to
	// refuse — scoring them as missed coverage would charge the reader for its correct
	// answer, and scoring them as namings would credit it for a row the labels do not
	// claim. Equality and not a floor, because either direction is wrong.
	if sc.Labelled != r.LabelledFrames {
		t.Errorf("the scoring pass counted %d labelled frames and the frame loop "+
			"counted %d. They are the same population — frames the annotation "+
			"says carried a selection — and a scorer working over a different "+
			"one is computing a coverage of something else",
			sc.Labelled, r.LabelledFrames)
	}
	if got := sc.Named + sc.NoList + sc.Unread; got != sc.Labelled {
		t.Errorf("%d labelled frames split into %d named + %d with no list + %d "+
			"unreadable = %d. A frame in none of the three has been lost, and "+
			"the coverage breakdown no longer says where the work is",
			sc.Labelled, sc.Named, sc.NoList, sc.Unread, got)
	}
	// ── AND WHAT THE SCORE WAS COMPUTED AGAINST ───────────────────────────────────
	//
	// The named ENTRY's region, never the identified LIST's envelope. This has been wrong
	// before: scored against the envelope of a list spanning three quarters of the
	// screen, every labelled row was inside it and the measurement read 154 of 155 for a
	// reader that was mostly wrong. Nothing held the repair in place until this line —
	// the mutation gate put the envelope back and every floor above still passed.
	if sc.NamedAsTallAsList != 0 {
		t.Errorf("%d naming(s) were scored against a region at least half as tall as "+
			"the whole list. An entry is not half a list: these numbers are "+
			"measuring the envelope, which is how a reader that cannot tell "+
			"neighbours apart once scored 99%%", sc.NamedAsTallAsList)
	}
	// ── AND THE READER'S OWN SCORE, which is the one that is already good ──────────
	//
	// Separated because the two came out 45 points apart here and 68 apart over the
	// whole corpus, and the single figure was misleading enough to be written up as a
	// reader failure. When the list being read contains the row the person is looking
	// at, the reader names that row almost every time; when it does not, no reading of
	// it can be right however well the reader works.
	//
	// This floor is therefore a ratchet on the thing that WORKS, and it is the one
	// number here that clears the 90% target. A change that improves overall accuracy
	// by making the reader worse inside the right list must fail.
	if sc.InsideNamed < wantReaderOf {
		t.Errorf("only %d naming(s) were made with the labelled row inside the list "+
			"being read, against %d measured; the reader's own score is computed "+
			"over too few to mean anything", sc.InsideNamed, wantReaderOf)
	}
	if sc.InsideRight < wantReader {
		t.Errorf("the reader named %d of %d rows correctly when handed a list "+
			"containing the labelled row, against %d measured",
			sc.InsideRight, sc.InsideNamed, wantReader)
	}
	if got := float64(sc.InsideRight) / float64(max(sc.InsideNamed, 1)); got < readerFloor {
		t.Errorf("handed a list containing the labelled row, the reader named it "+
			"%.0f%% of the time, below the %.0f%% target it currently clears",
			100*got, 100*readerFloor)
	}
	// ── THE SHAPE OF THE ERROR, which is the diagnosis the next change rests on ────
	//
	// Every wrong naming measured here was a naming of the WRONG LIST, not of the wrong
	// row in the right one: zero were off by a neighbour, and all but one had the
	// labelled row outside the list being read entirely. That is what says the work is
	// in disambiguation and not in the reader, and a change that quietly moves the
	// errors into the reader must not pass as an improvement.
	if sc.OffByOne != 0 {
		t.Errorf("%d naming(s) were one row off. Zero were measured, and a neighbour "+
			"confusion is a new class of error: half the labelled pitch is exactly "+
			"the distinction a navigation route is made of", sc.OffByOne)
	}
	if wrong := sc.Named - sc.Right; wrong > 0 && sc.OutsideList < wrong-1 {
		t.Errorf("%d of %d wrong namings had the labelled row outside the list being "+
			"read; %d did, so at least %d are now the reader naming the wrong row "+
			"in a list that contained the right one. That is a different defect "+
			"from the measured one", sc.OutsideList, wrong, sc.OutsideList,
			wrong-sc.OutsideList)
	}

	t.Logf("real corpus, every 10th frame: %d frames, discovery %d/%d, grouping %d/%d, "+
		"identified after %d, named %d of which %d matched, %.1f proposals/frame",
		r.Frames, r.CoveredSelected, r.LabelledFrames, r.Grouped, r.Frames,
		r.Identified, r.WithSelection, r.CorrectSelection,
		float64(r.Proposed)/float64(r.Frames))
	t.Logf("coverage %d/%d (%.0f%%: %d no list, %d unreadable); accuracy %d/%d (%.0f%%); "+
		"the reader alone %d/%d (%.0f%%); %d wrong naming(s), %d of them of a list "+
		"that did not contain the labelled row, %d off by a neighbour",
		sc.Named, sc.Labelled, 100*float64(sc.Named)/float64(sc.Labelled),
		sc.NoList, sc.Unread,
		sc.Right, sc.Named, 100*float64(sc.Right)/float64(max(sc.Named, 1)),
		sc.InsideRight, sc.InsideNamed,
		100*float64(sc.InsideRight)/float64(max(sc.InsideNamed, 1)),
		sc.Named-sc.Right, sc.OutsideList, sc.OffByOne)
}
