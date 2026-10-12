package main

import (
	"fmt"
	"sort"
	"strings"
)

// Which list the belief was reading, scored as an IDENTITY rather than as an overlap.
//
// # Why this is a separate question from the selection score
//
// Because score.go established that the reader is right 93% of the time when the list it is
// handed contains the row the person is looking at, and wrong almost every time it does not.
// That makes "which list" the measurement that matters, and `the labelled row is inside it` is
// too weak a test of it: a candidate spanning the whole screen contains every labelled row
// there is, and one spanning two rows of the menu contains whichever row the mark is on.
//
// So the list is scored against the labelled list's own extent. Two numbers, and both are
// needed:
//
//	ROWS INSIDE   how many of the labelled rows fall within the identified extent
//	SPAN RATIO    how much taller the identified extent is than the labelled list
//
// The first alone admits the full-screen candidate. The second alone admits any small
// rectangle. Together they say "this is the list the labels describe, give or take the rows a
// detector dropped".
const (
	// ListRowsInside is how many of the labelled rows an identified list must contain to
	// be the labelled list.
	//
	// Six of nine, not nine of nine. The menu groups as six of its nine entries on this
	// corpus — a measured, separately recorded defect in `stack` — and a list missing
	// three rows at one end is still the menu: a route planned in it names real entries
	// and the reader reads them correctly. Demanding all nine would score a known
	// grouping shortfall as a list-identification failure and hide the one being measured.
	ListRowsInside = 6
	// ListSpanSlack is how much taller than the labelled list, in labelled pitches, an
	// identified extent may be.
	//
	// One pitch at each end plus a half, because the labelled rows are CENTRES: a correct
	// envelope runs from half a row above the first to half a row below the last, and a
	// detector that found a row the labels do not name adds one more. Beyond that the
	// extent is no longer describing the same thing — and the candidate this rules out is
	// the one spanning y 0.059..1.000, which contains all nine labelled rows and is not a
	// menu.
	ListSpanSlack = 2.5
)

// publishBranch is which arm of the tracker's decision published this belief.
//
// Read off the belief's own prose rather than from a new field, because the prose is already
// distinct per arm and a second channel saying the same thing is a second channel that can
// disagree with it. The arms are the whole of `Tracker.active`, and they rest on completely
// different evidence — which is the point of separating them here.
type publishBranch string

const (
	// branchSole is one candidate qualifying and being published with NO comparison to
	// anything: not to another candidate, and not to whether the person's navigation ever
	// reached it. First past the post.
	branchSole publishBranch = "sole_qualifier"
	// branchResponsive is several candidates qualifying and one out-responding the rest by
	// NavMargin, which is the only arm that consults input evidence at all.
	branchResponsive publishBranch = "most_responsive"
	// branchAmbiguous is several qualifying and none out-responding — the abstention that
	// is supposed to happen when picking would be guessing.
	branchAmbiguous publishBranch = "ambiguous"
	// branchNone is nothing qualifying yet.
	branchNone publishBranch = "none_qualified"
	// branchOther is a belief that got here some other way: a bare look, a dropped belief,
	// a look that brought nothing. Kept separate so it cannot be read as an abstention.
	branchOther publishBranch = "other"
)

func branchOf(because string) publishBranch {
	switch {
	case strings.Contains(because, "one list hands a mark around"):
		return branchSole
	case strings.Contains(because, "qualifying lists responds to navigation"):
		return branchResponsive
	case strings.Contains(because, "lists each hand a mark between their members"):
		return branchAmbiguous
	case strings.Contains(because, "no list has been seen handing a mark"):
		return branchNone
	}
	return branchOther
}

// listVerdict is what one frame's identified list was.
type listVerdict string

const (
	listCorrect listVerdict = "correct"
	listWrong   listVerdict = "wrong"
	listNone    listVerdict = "none"
)

// branchScore is one publishing arm's record.
type branchScore struct {
	Frames int
	// Correct, Wrong and None are how the identified list scored on those frames.
	Correct, Wrong, None int
	// Named and Right are the selection readings made under this arm, so the two
	// questions stay separable: which list was published, and what was read in it.
	Named, Right int
}

// listScoreboard is the whole session's account of which list was being read.
type listScoreboard struct {
	Labelled int
	Correct  int
	Wrong    int
	None     int
	// ByBranch is the table that locates the failure: every arm rests on different
	// evidence, so an arm that is usually wrong names the rule to look at.
	ByBranch map[publishBranch]*branchScore
	// Runs is the timeline, collapsed into stretches the belief spent reading one thing.
	//
	// Collapsed rather than per-frame because the question is about stretches: a belief
	// that published one wrong list for two hundred frames and a belief that flickered
	// between six are different defects, and a count cannot tell them apart.
	Runs []listRun
}

// listRun is one stretch of frames the belief spent on one list under one arm.
type listRun struct {
	From, To int
	Branch   publishBranch
	Verdict  listVerdict
	// Top, Bottom and Entries are the extent as it was at the START of the run, and Moved
	// says the extent changed while the run lasted.
	//
	// Moved is the tell for candidate identity churn. A candidate's bounds are overwritten
	// by every frame's extent, and candidates are matched by overlap normalised on the
	// SMALLER area — so a small group nested inside a tracked candidate's extent matches
	// it completely and drags its bounds onto itself. A run whose extent moves is that
	// happening, and it cannot be seen from the final candidate table at all, because by
	// then the bounds are wherever they ended up.
	Top, Bottom float64
	Entries     int
	Moved       bool
	Frames      int
}

// scoreLists walks the trace and scores which list the belief was reading.
func scoreLists(trace []frameObservation, ann *annotationFile) listScoreboard {
	sb := listScoreboard{ByBranch: map[publishBranch]*branchScore{}}
	rows := ann.labelledRows()
	pitch := labelledPitch(rows)
	if pitch <= 0 || len(rows) == 0 {
		return sb
	}
	labelledSpan := rows[len(rows)-1] - rows[0] + pitch
	var cur *listRun
	for _, f := range trace {
		state, y, ok := ann.labelAt(f.Index)
		if !ok || state != "selected" {
			continue
		}
		sb.Labelled++
		branch := branchOf(f.Reading)
		verdict := listNone
		if f.ListBottom > f.ListTop {
			inside := 0
			for _, r := range rows {
				if r >= f.ListTop && r <= f.ListBottom {
					inside++
				}
			}
			verdict = listWrong
			if inside >= ListRowsInside &&
				f.ListBottom-f.ListTop <= labelledSpan+ListSpanSlack*pitch {
				verdict = listCorrect
			}
		}
		switch verdict {
		case listCorrect:
			sb.Correct++
		case listWrong:
			sb.Wrong++
		default:
			sb.None++
		}
		bs := sb.ByBranch[branch]
		if bs == nil {
			bs = &branchScore{}
			sb.ByBranch[branch] = bs
		}
		bs.Frames++
		switch verdict {
		case listCorrect:
			bs.Correct++
		case listWrong:
			bs.Wrong++
		default:
			bs.None++
		}
		if f.Selected >= 0 {
			bs.Named++
			if abs(f.NamedY-y) <= pitch/2 {
				bs.Right++
			}
		}
		// THE TIMELINE. A run ends when the arm or the verdict changes, or when the
		// extent stops overlapping what the run started on — which is a different list
		// rather than the same one drifting.
		switch {
		case cur == nil || cur.Branch != branch || cur.Verdict != verdict ||
			!overlapsRun(*cur, f):
			if cur != nil {
				sb.Runs = append(sb.Runs, *cur)
			}
			cur = &listRun{
				From: f.Index, To: f.Index, Branch: branch, Verdict: verdict,
				Top: f.ListTop, Bottom: f.ListBottom, Entries: f.Entries,
				Frames: 1,
			}
		default:
			cur.To = f.Index
			cur.Frames++
			if f.ListTop != cur.Top || f.ListBottom != cur.Bottom {
				cur.Moved = true
			}
			if f.Entries != cur.Entries {
				cur.Moved = true
			}
		}
	}
	if cur != nil {
		sb.Runs = append(sb.Runs, *cur)
	}
	return sb
}

// overlapsRun reports whether this frame's extent is still the extent the run began on.
//
// Vertical only, and generously: the question is whether the belief is still reading the same
// part of the screen, not whether the rectangle is identical. A run that ends here is the
// belief moving to a different list; a run that continues with Moved set is one candidate's
// bounds drifting, and the two are different findings.
func overlapsRun(r listRun, f frameObservation) bool {
	if r.Bottom <= r.Top || f.ListBottom <= f.ListTop {
		return r.Bottom <= r.Top && f.ListBottom <= f.ListTop
	}
	lo, hi := max64(r.Top, f.ListTop), min64(r.Bottom, f.ListBottom)
	if hi <= lo {
		return false
	}
	return (hi-lo)/min64(r.Bottom-r.Top, f.ListBottom-f.ListTop) >= 0.5
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// reportListIdentity prints the account.
func reportListIdentity(sb listScoreboard, identifiedAt int) {
	if sb.Labelled == 0 {
		return
	}
	fmt.Printf("\n  WHICH LIST WAS BEING READ, scored against the labelled list's extent\n")
	fmt.Printf("  identified after    %d observation(s)\n", identifiedAt)
	fmt.Printf("  correct list        %d of %d labelled frames (%.0f%%): >=%d labelled\n"+
		"                      rows inside it and no more than %.1f pitches taller\n"+
		"                      than the labelled list\n",
		sb.Correct, sb.Labelled, rate(sb.Correct, sb.Labelled),
		ListRowsInside, ListSpanSlack)
	fmt.Printf("  WRONG list          %d of %d labelled frames (%.0f%%)  <-- a belief "+
		"published\n                      about a structure that is not the menu\n",
		sb.Wrong, sb.Labelled, rate(sb.Wrong, sb.Labelled))
	fmt.Printf("  no list             %d of %d labelled frames (%.0f%%)\n",
		sb.None, sb.Labelled, rate(sb.None, sb.Labelled))

	// THE TABLE THAT LOCATES THE FAILURE. Each arm of Tracker.active rests on different
	// evidence, so an arm that is usually wrong names the rule to look at.
	fmt.Printf("\n  by publishing arm   frames  correct  wrong  none   named  right\n")
	arms := []publishBranch{branchSole, branchResponsive, branchAmbiguous,
		branchNone, branchOther}
	for _, a := range arms {
		bs := sb.ByBranch[a]
		if bs == nil {
			continue
		}
		fmt.Printf("    %-16s  %-6d  %-7d  %-5d  %-5d  %-5d  %d\n",
			a, bs.Frames, bs.Correct, bs.Wrong, bs.None, bs.Named, bs.Right)
	}
	fmt.Printf("    (sole_qualifier publishes ONE qualifying candidate with no " +
		"comparison\n     to anything, including whether navigation ever reached it. " +
		"most_responsive\n     is the only arm that consults input evidence.)\n")

	// AND THE TIMELINE, longest stretches first: a belief that published one wrong list
	// for two hundred frames and one that flickered between six are different defects.
	if len(sb.Runs) > 0 {
		runs := append([]listRun(nil), sb.Runs...)
		sort.SliceStable(runs, func(i, j int) bool { return runs[i].Frames > runs[j].Frames })
		fmt.Printf("\n  longest stretches   frames  from..to     arm               " +
			"verdict  extent y..y   entries moved\n")
		for i, r := range runs {
			if i >= 8 {
				fmt.Printf("                      ... and %d more stretch(es)\n",
					len(runs)-i)
				break
			}
			fmt.Printf("                      %-6d  %-11s  %-16s  %-7s  %.3f..%.3f  "+
				"%-7d %v\n", r.Frames,
				fmt.Sprintf("%d..%d", r.From, r.To), r.Branch, r.Verdict,
				r.Top, r.Bottom, r.Entries, r.Moved)
		}
		fmt.Printf("                      (%d stretch(es) in all; `moved` is the "+
			"extent changing\n                      WITHIN one stretch, which is one "+
			"candidate's bounds drifting\n                      rather than the belief "+
			"moving to another list)\n", len(sb.Runs))
	}
}
