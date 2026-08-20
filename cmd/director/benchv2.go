package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// `director benchmark-vision` against a v2 corpus.
//
// A separate rendering path because the two corpora answer different questions. V1 reports
// coverage ratios and a temporal proxy; v2 reports precision and recall against declared truth.
// Printing one under the other's headings is how a reader ends up comparing numbers that were
// never comparable — the exact confusion corpus versioning was introduced to stop.

// calibrationSequences is the FROZEN split, recorded here as data rather than convention.
//
// Chosen before any held-out evidence was seen and before a confidence threshold was picked.
// Split by SEQUENCE, never by frame: adjacent frames of one sequence are near-duplicates, so a
// frame-level split would tune against essentially the evidence it is later judged on.
var calibrationSequences = map[string]bool{
	"freeplay-static": true,
	"pause-stable":    true,
}

// splitRole names which population a sequence belongs to.
func splitRole(sequence string) string {
	if calibrationSequences[sequence] {
		return "calibration"
	}
	return "held-out"
}

// runBenchmarkV2 scores every selected backend against declared ground truth.
func runBenchmarkV2(dir string, backends []visionbench.Backend, split string) int {
	corpus, err := visionbench.LoadV2(dir)
	if err != nil {
		fmt.Printf("director: %v\n", err)
		return 1
	}

	fmt.Printf("Vision benchmark — corpus %s\n", visionbench.CorpusV2)
	fmt.Printf("  fixture      %s, %d frames, %d sequences\n",
		corpus.Fixture.Name, len(corpus.Fixture.Frames), len(corpus.SequenceNames()))
	fmt.Printf("  truth        %d annotated regions\n", countRegions(corpus.Truths))
	// The split is printed on every report. A score quoted without its evaluation
	// population is what made a full-corpus 69.5 look comparable to a held-out 63%.
	keep := func(seq string) bool {
		switch split {
		case "calibration", "held-out":
			return splitRole(seq) == split
		default:
			return true
		}
	}
	var included []string
	for _, s := range corpus.SequenceNames() {
		if keep(s) {
			included = append(included, s)
		}
	}
	sub := corpus.Subset(keep)
	fmt.Printf("  split        %s — %d frames, %d sequences, %d truth regions\n",
		split, len(sub), len(included), countRegions(sub))
	fmt.Printf("  sequences    %s\n\n", strings.Join(included, ", "))

	for _, b := range backends {
		runOneV2(corpus, b, keep, included)
	}
	return 0
}

func countRegions(truths []visionbench.FrameTruth) int {
	n := 0
	for _, t := range truths {
		n += len(t.Regions)
	}
	return n
}

// runOneV2 measures one backend, aggregate first and then per sequence.
//
// Per-sequence is not optional. ScreenParser's aggregate hides the finding that matters —
// excellent on menus, quiet under camera motion, stale after a menu closes — and an aggregate
// score is exactly where that disappears.
func runOneV2(corpus visionbench.V2Corpus, b visionbench.Backend,
	keep func(string) bool, sequences []string) {
	byFrame := map[string][]visionbench.Detection{}
	var lat []time.Duration
	failures := 0

	evalTruths := corpus.Subset(keep)
	inScope := map[string]bool{}
	for _, t := range evalTruths {
		inScope[t.Key()] = true
	}
	for _, f := range corpus.Fixture.Frames {
		if !inScope[f.Name] {
			continue
		}
		start := time.Now()
		dets, err := b.Detect(context.Background(), f.Image)
		lat = append(lat, time.Since(start))
		if err != nil {
			// Reported, never absorbed into an empty result: an infrastructure failure
			// recorded as "found nothing" becomes model performance in the table.
			if failures == 0 {
				fmt.Printf("%s: %v\n\n", b.Name(), err)
			}
			failures++
			continue
		}
		byFrame[f.Name] = dets
	}
	if failures > 0 && failures == len(evalTruths) {
		fmt.Printf("%-14s UNAVAILABLE — every frame failed\n\n", b.Name())
		return
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	median, p95 := time.Duration(0), time.Duration(0)
	if len(lat) > 0 {
		median, p95 = lat[len(lat)/2], lat[(len(lat)*95)/100]
	}

	m := visionbench.EvaluateTruthModes(byFrame, corpus.Bounds, evalTruths, corpus.Modes)
	score, ok := visionbench.ScoreV2(m, median, visionbench.DefaultWeightsV2())
	total := "unavailable"
	if ok {
		total = fmt.Sprintf("%.1f", score.Total)
	}

	fmt.Printf("%s — %s\n", b.Name(), b.Model())
	fmt.Printf("  detections %d   TP %d   FP %d   FN %d   unmatched %d   truth %d   matched %d\n",
		m.Detections, m.TruePos, m.FalsePos, m.TruthRegions-m.Matched,
		m.Unmatched, m.TruthRegions, m.Matched)
	fmt.Printf("  structural   P %5.0f%%  R %5.0f%%\n", m.Precision*100, m.Recall*100)
	fmt.Printf("  nameable     P %5.0f%%  R %5.0f%%\n",
		m.NameablePrecision*100, m.NameableRecall*100)
	// The temporal line says what it is a mean OVER. An aggregate quoted without its
	// population is what let a 0/0 sequence read as a 0% detector failure for two milestones.
	fmt.Printf("  temporal     P %5.0f%%  R %5.0f%%   (mean over %d/%d sequences, "+
		"%d transition tracks)\n",
		m.TemporalPrecision*100, m.TemporalRecall*100,
		m.TemporalPrecisionSequences, m.TemporalRecallSequences, m.TransitionTracks)
	if m.TransitionTracks > 0 {
		fmt.Printf("               transitions: %d on-time, %d mistimed, %d expected frames\n",
			m.OnTime, m.Mistimed, m.Expected)
	}
	fmt.Printf("  OCR-region   P %5.0f%%  R %5.0f%%\n", m.OCRPrecision*100, m.OCRRecall*100)
	fmt.Printf("  latency      median %s  p95 %s\n", median.Round(time.Millisecond),
		p95.Round(time.Millisecond))
	fmt.Printf("  ScoreV2      %s\n\n", total)

	// Mode is printed beside every sequence. Two sequences whose temporal numbers were
	// produced by different questions must never be read as one column of like values.
	fmt.Printf("  %-24s %-14s %5s %5s %5s %5s %7s %7s %7s %7s\n",
		"SEQUENCE", "SHAPE", "DETS", "TP", "FP", "FN", "PREC", "RECALL", "T-PREC", "T-REC")
	for _, seq := range sequences {
		sub := corpus.Subset(func(s string) bool { return s == seq })
		sm := visionbench.EvaluateTruthModes(byFrame, corpus.Bounds, sub, corpus.Modes)
		// `sub` is one FrameTruth per frame, so its length is the sequence's frame count.
		mode := corpus.Modes[seq].Shape(len(sub))
		fmt.Printf("  %-24s %-14s %5d %5d %5d %5d %6.0f%% %6.0f%% %6.0f%% %6.0f%%",
			seq, mode, sm.Detections, sm.TruePos, sm.FalsePos,
			sm.TruthRegions-sm.Matched,
			sm.Precision*100, sm.Recall*100,
			sm.TemporalPrecision*100, sm.TemporalRecall*100)
		// The two raw counts that answer the product question directly: did it track the
		// element while it was there, and did it let go when it left. A percentage alone
		// cannot say which of those a transition sequence failed.
		if sm.TransitionTracks > 0 {
			fmt.Printf("   %d/%d on-time, %d mistimed",
				sm.OnTime, sm.Expected, sm.Mistimed)
		}
		fmt.Println()
	}
	fmt.Println()
}
