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

// At a coarse cadence this corpus identifies nothing, and that is the honest answer.
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
// # Why nothing qualifies now
//
// Because a candidate has to be found again to accumulate anything, and at 660ms between looks
// the mark has usually moved between them. The real menu's candidate reaches 28 exclusive
// frames against 11 shared, and the exclusivity rule wants exclusive frames to outnumber shared
// ones SharedPenalty-fold. So it falls one requirement short and says which — which is the
// behaviour the brief asked for where the active list cannot be established.
//
// # What this test is for
//
// To stop that abstention quietly becoming a wrong answer again. It asserts no wrong list at
// all, and that whatever the belief says, it names the requirement that fell short rather than
// going silent. It does NOT assert that nothing is identified: identifying the menu here would
// be an improvement and must not be a failure.
func TestAtACoarseCadenceTheCorpusAbstains(t *testing.T) {
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
	if ls.Correct > 0 {
		t.Logf("the menu is now identified on %d of %d labelled frames at every tenth "+
			"frame, which it was not. Raise this into an assertion and say so in "+
			"the experiment record", ls.Correct, ls.Labelled)
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
	// # IT IS ACHIEVED NOW, and the target moves rather than the floor
	//
	// Identity by rows takes the full corpus to 30 of 33 namings on the labelled row, 91%.
	// What that cost is coverage: 135 frames named an entry before and 33 do, because the
	// published extent used to be a bucket spanning the screen and the reader could find
	// some spike inside it on almost any frame. 43 of those 135 were right. So the trade is
	// 13 right answers for 89 wrong ones, and the wrong ones are the graph-corrupting half.
	//
	// The target is raised to 95% rather than retired, because the remaining 3 wrong
	// namings in 33 are the reader missing by a row and that is a different piece of work
	// from the coverage. Both are in [[Experiment-026-the-same-menu-seen-from-pixels]].
	const (
		wantNamed   = 35
		wantMatched = 30
		targetRate  = 0.95
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
		wantCovered  = 33
		wantRight    = 30
		wantReader   = 30
		wantReaderOf = 31
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
	// THE ERRORS HAVE MOVED, which is what the work was for. Identity by rows leaves 3
	// wrong namings in 33 against 142 in 185, and the signed-offset histogram went from a
	// spike at -3 — a list starting three rows too low — to -1:3  +0:30  +3:1. So the
	// residual error is now a neighbour confusion inside the right list, which is a
	// smaller and different problem, and the ceiling below holds it where it is rather
	// than letting it grow back.
	//
	// It is a CEILING and not an equality, because zero is no longer the measurement and
	// pretending otherwise would fail every run. If it falls to zero, tighten it.
	const allowOffByOne = 3
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
	if wrong := sc.Named - sc.Right; wrong > 0 && sc.OutsideList < wrong-1 {
		t.Errorf("%d of %d wrong namings had the labelled row outside the list being "+
			"read; %d did, so at least %d are now the reader naming the wrong row "+
			"in a list that contained the right one. That is a different defect "+
			"from the measured one", sc.OutsideList, wrong, sc.OutsideList,
			wrong-sc.OutsideList)
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
	const (
		wantCorrectList = 312
		allowWrongList  = 0
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
