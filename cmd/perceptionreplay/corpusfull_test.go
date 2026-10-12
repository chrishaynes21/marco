//go:build corpus

// The full-frame corpus regression, behind a build tag.
//
// # Why it is not in the default suite
//
// Because it takes about 145 seconds and it was making other packages fail. In one run it
// starved `cmd/marco` past its 600-second limit — a package that passes in 14 seconds on its
// own — and it inflated a latency assertion in `internal/director/perception/selection` from
// 962 microseconds to 60 milliseconds, which that test correctly reported as a failure. Neither
// package had a defect. A suite that produces failures nothing is wrong with is worse than a
// slow one, especially while the work is comparing small differences between experiments.
//
// # What still guards perception in the default suite
//
// `TestAtACoarseCadenceTheCorpusIdentifiesTheMenu`, in corpus_test.go, which replays every tenth
// frame in about 14 seconds and holds floors on identification and on reading at the cadence
// live observation runs near. So the default gate still fails on a perception regression; what
// moved behind the tag is the full-frame measurement, which is the one with the tight ratchets
// and the one worth a minute and a half.
//
// # Run it
//
//	go test -tags corpus ./cmd/perceptionreplay/
//
// REQUIRED for any change to perception. The ratchets here are the record of every measurement
// this effort has made, and a change that moves them without saying so is the thing they exist
// to catch. [[ADR-167-a-corpus-gate-that-breaks-other-tests-is-not-a-gate]].

package main

import (
	"testing"
)

func TestTheRealMenuCorpusStillReadsAsAMenu(t *testing.T) {
	dir := corpusDir()
	if dir == "" {
		t.Skip("no local L0 recording found; this measurement needs real frames and " +
			"they are private and out of version control. Set $MARCO_FRAME_CORPUS to " +
			"a capture-game-session directory to run it.")
	}
	// EVERY FRAME, which takes about ninety seconds and is the point.
	//
	// It was every tenth, for speed, and that was affordable while a sampled run and a full
	// one said roughly the same thing. They no longer do. Under identity by rows —
	// [[ADR-164-a-list-is-the-same-list-when-its-rows-are]] — a candidate has to be found
	// again to accumulate anything, and at every tenth frame this corpus gives the real menu
	// 28 exclusive frames against 11 shared, which the exclusivity rule refuses. So the
	// sampled run now identifies NOTHING, and a gate run there would ratchet an abstention.
	//
	// What the sampled run used to ratchet is in TestAtACoarseCadenceTheCorpusAbstains: a
	// bucket whose hull spanned nine tenths of the screen, published on 55 frames and right
	// about which list on 22 of them. Ninety seconds to measure the thing this whole effort
	// is about is a trade worth making, and the alternative is a floor set on an artefact.
	r, err := replayFrames(dir, 1, 0, false)
	if err != nil {
		t.Fatalf("replaying the local corpus: %v", err)
	}
	if r.Frames < 800 {
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
	// So 90% was never achieved and was kept here as the TARGET, failed, rather than
	// quietly lowered.
	//
	// # 95% IS CLEARED, AND THE TARGET MOVES
	//
	// The gap was never the reader. It was that the list it was handed did not contain the
	// row the mark was on, and the reason for THAT was grouping: a list shares an edge, and
	// `alike` tested the left edge and the centre but not the RIGHT one — so one partially
	// found row, right-aligned with its siblings and a third of their width, cut a
	// nine-entry menu in half. [[ADR-166-a-list-shares-an-edge-and-a-partial-row-keeps-one]].
	//
	// With the right edge admitted: coverage 100 to 362 frames, right namings 78 to 349, and
	// 96% of every naming on the labelled row. Neighbour confusions went from 20 to 2.
	//
	// The target rises to 99%, which is where the remaining 13 sit: 11 of them are frames
	// where the labelled row is still not inside the list, and 2 are the reader missing by a
	// row. Those are different pieces of work and the floors below hold them apart.
	const (
		wantNamed   = 366
		wantMatched = 349
		targetRate  = 0.99
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
		wantCovered  = 362
		wantRight    = 349
		wantReader   = 349
		wantReaderOf = 351
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
	// Every wrong naming used to be a naming of the WRONG LIST rather than of the wrong row
	// in the right one: zero off by a neighbour, 142 off by more, and 139 of those 142 with
	// the labelled row outside the list being read entirely. That is what said the work was
	// in disambiguation and not in the reader.
	//
	// THE ERRORS HAVE MOVED, which is what the work was for. The signed-offset histogram
	// went from a spike at -3 — a list starting three rows too low — to -1:1  +0:78  +1:20,
	// so the residual error is a neighbour confusion inside the right list rather than a
	// reading of the wrong list. Smaller, different, and the next piece of work.
	//
	// It is a CEILING and not an equality, because zero is no longer the measurement and
	// pretending otherwise would fail every run. If it falls, tighten it.
	//
	// TWO of them, down from twenty, because the twenty were the extent's error rather
	// than the reader's: the list did not contain the row the mark was on, so the reader
	// named its nearest neighbour inside the list. With the menu no longer cut in half
	// they are gone, and what is left is the reader's own: two frames where it named the
	// row next to the right one in a list that contained both.
	const allowOffByOne = 2
	if sc.OffByOne > allowOffByOne {
		t.Errorf("%d naming(s) were one row off, against %d measured. Half the labelled "+
			"pitch is exactly the distinction a navigation route is made of, so a "+
			"neighbour confusion is retained as a move between rows the mark never "+
			"made", sc.OffByOne, allowOffByOne)
	}
	if sc.OffByOne < allowOffByOne {
		t.Logf("%d naming(s) off by a neighbour, against %d measured. Lower the ceiling "+
			"in this test and say so in the experiment record", sc.OffByOne,
			allowOffByOne)
	}
	// AND THE TWO KINDS OF WRONG, counted apart. A naming of a row in a list that did not
	// contain the mark could not have been right; a naming of the wrong row in a list that
	// did is the reader's own error. The first is list completeness and the second is the
	// reader, and a change that trades one for the other must not look like an improvement.
	const allowInList = 2
	if wrong := sc.Named - sc.Right; wrong-sc.OutsideList > allowInList {
		t.Errorf("%d of %d wrong namings were made in a list that DID contain the "+
			"labelled row, against %d measured. That is the reader naming the wrong "+
			"row, which is a different defect from a list that is missing one",
			wrong-sc.OutsideList, wrong, allowInList)
	}

	// ── WHICH LIST, SCORED AS AN IDENTITY ─────────────────────────────────────────
	//
	// The ratchet that matters most, and the one the numbers above cannot express. The
	// reader is 95% accurate here when the list it is handed contains the row the person
	// is looking at, so the question that decides everything downstream is which list
	// gets handed to it — and "the labelled row is inside it" is too weak a test of
	// that: a candidate spanning the whole screen satisfies it, and a measured candidate
	// on this corpus does exactly that.
	//
	// WHY THIS IS NOT REDUNDANT WITH THE FLOORS ABOVE. Two repairs to candidate identity
	// were tried and rejected on measurement, and BOTH improved coverage, accuracy and
	// the reader's conditional score while taking correct-list identification from 45 of
	// 829 to 6 and then to 0. A change that improves three metrics while collapsing the
	// fourth looks like progress in every number this test used to assert.
	ls := r.Lists
	if ls.Labelled != sc.Labelled {
		t.Errorf("the list score counted %d labelled frames and the selection score "+
			"%d; they are the same population", ls.Labelled, sc.Labelled)
	}
	// THE NUMBER THIS CYCLE WAS ABOUT. 45 of 829 before identity by rows, and the 45 came
	// from the one arm that consults input evidence while a bucket published the rest.
	//
	// allowWrongList is ZERO and that is an assertion, not a happy accident: a belief
	// published about a structure that is not the menu is wrong before the reader is
	// consulted, and every one of them is a candidate for a retained transition between
	// rows the mark never visited. 147 of them at this cadence two commits ago.
	// allowWrongList is SIX and was zero, which is a regression and is recorded as one.
	// Admitting a shared right edge joins rows that belong together and, on six frames of
	// 829, joins something that does not: the published list fails the labelled list's own
	// extent test. Against 147 more frames on which it identifies the menu correctly.
	const (
		wantCorrectList = 559
		allowWrongList  = 6
	)
	if ls.Correct < wantCorrectList {
		t.Errorf("the belief was published about the labelled list on %d of %d "+
			"labelled frames, against %d measured. Every reading made about "+
			"another list is wrong before the reader is consulted",
			ls.Correct, ls.Labelled, wantCorrectList)
	}
	if ls.Wrong > allowWrongList {
		t.Errorf("the belief was published about a structure that is NOT the menu on "+
			"%d of %d labelled frames, against %d measured. This is the number to "+
			"drive to zero: a reading from the wrong list is retained as a move "+
			"between rows the mark never made, and nothing downstream knows",
			ls.Wrong, ls.Labelled, allowWrongList)
	}
	if got := ls.Correct + ls.Wrong + ls.None; got != ls.Labelled {
		t.Errorf("%d labelled frames split into %d correct + %d wrong + %d none = %d",
			ls.Labelled, ls.Correct, ls.Wrong, ls.None, got)
	}
	// ── AND NO BELIEF IS PUBLISHED WITHOUT RESPONSIVENESS EVIDENCE ────────────────
	//
	// The sole-qualifier arm used to publish one qualifying candidate with no comparison
	// to anything, including whether the person's navigation had ever reached it. Over
	// 864 frames it named the wrong list on 144 of 144 frames and produced 139 of the
	// session's 185 readings, not one of them right; the arm that consults input evidence
	// was right on 45 of its 48. Both arms now require the same evidence.
	//
	// Asserted as a property of the whole session rather than of one frame: any arm may
	// publish, and none may publish without having seen the list respond.
	for _, b := range []publishBranch{branchSole, branchResponsive} {
		bs := ls.ByBranch[b]
		if bs == nil || bs.Frames == 0 {
			continue
		}
		if bs.Correct == 0 && bs.Frames > 10 {
			t.Errorf("the %s arm published %d belief(s) and not one was about the "+
				"labelled list. An arm that is never right names the rule to "+
				"look at", b, bs.Frames)
		}
	}

	t.Logf("real corpus, every frame: %d frames, discovery %d/%d, grouping %d/%d, "+
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
	t.Logf("which list: %d correct, %d WRONG, %d none, of %d labelled frames",
		ls.Correct, ls.Wrong, ls.None, ls.Labelled)
}
