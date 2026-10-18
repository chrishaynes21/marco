package main

// Running the saved-screen recognition experiment and reporting what separated.

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/screen"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// screenSample is how many looks of each screen are described.
//
// Twenty is enough to span a screen's animation and selection states and few enough that the
// experiment finishes: each look is a 1920x1080 PNG read and ten histograms.
const screenSample = 20

var measures = []string{"arrangement", "shape", "palette", "grid", "detector"}

func reportScreenRecognition(dir string, limit int, onnx string) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	if len(frames) < 20 {
		return fmt.Errorf("%s carries %d frames, too few to split", dir, len(frames))
	}
	if limit > 0 && limit < len(frames) {
		frames = frames[:limit]
	}

	regions, readable := proposeAll(dir, frames)

	// THE RECORDED DETECTIONS, PER LOOK, used as saved evidence rather than re-run.
	dets := map[int][]observe.Candidate{}
	if onnx != "" {
		fd, ferr := loadSlowFeed(onnx, 0)
		if ferr != nil {
			return ferr
		}
		dets = fd.byLook
	}

	half := len(frames) / 2
	oracle, oerr := foldRange(frames, regions, readable, nil, 0, len(frames))
	if oerr != nil {
		return oerr
	}
	a, aerr := foldRange(frames, regions, readable, nil, 0, half)
	if aerr != nil {
		return aerr
	}
	b, berr := foldRange(frames, regions, readable, nil, half, len(frames))
	if berr != nil {
		return berr
	}

	describeAll := func(f fold) []screenEvidence {
		var out []screenEvidence
		for _, s := range summarise(f, oracle, regions) {
			out = append(out, describeScreen(dir, frames, regions, dets, f,
				s.id, s.truth, screenSample))
		}
		sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
		return out
	}
	evA, evB := describeAll(a), describeAll(b)

	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("CAN A DEMONSTRATED SCREEN BE RECOGNISED AGAIN? — %s\n", filepath.Base(dir))
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  fold A  looks 1-%d    %d screen(s)\n", half, len(evA))
	fmt.Printf("  fold B  looks %d-%d  %d screen(s), from an observer that shares nothing\n",
		half+1, len(frames), len(evB))
	fmt.Printf("  the oracle scores the answers and never produces one\n")
	if onnx == "" {
		fmt.Printf("  no detections supplied, so the `detector` measure is empty\n")
	}

	// ── SENSITIVITY, FIRST ──
	//
	// A measure that cannot describe one screen twice cannot tell two screens apart, and
	// Experiment-035 is the record of finding that out in the wrong order.
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  HOW STEADY EACH MEASURE IS ON ONE SCREEN\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("  Each screen of fold A against ITSELF, across the looks sampled. A measure\n")
	fmt.Printf("  that drops here is reading animation or selection state, not identity.\n\n")
	fmt.Printf("  %-10s %7s %9s  %s\n", "screen", "looks", "proposals", "self-similarity")
	for _, e := range evA {
		lo, hi := proposalRange(e.proposals)
		fmt.Printf("  %-10s %7d %4d-%-4d ", e.id, e.looks, lo, hi)
		for _, m := range measures {
			fmt.Printf(" %s=%.2f", short(m), selfSimilarity(m, e))
		}
		fmt.Printf("\n")
	}

	// ── RECOGNITION ──
	type tally struct {
		correctTop, wrongTop, noCounterpart int
		margins                             []float64
		// wouldBindWrong is the number the whole experiment turns on: a screen
		// with NO counterpart whose best candidate clears the production floor
		// unopposed. The conservative binder refuses two candidates above the
		// floor; it cannot refuse one, and one is what these are.
		wouldBindWrong int
		// wouldRefuseCorrect is the cost of that same rule: a CORRECT top answer
		// with a runner-up also above the floor, which the binder calls
		// ambiguous and declines.
		wouldRefuseCorrect int
	}
	results := map[string]*tally{}
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  WHERE THE RIGHT ANSWER CAME, AND BY HOW MUCH\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("  For each screen of fold A, every screen of fold B is scored and sorted.\n")
	fmt.Printf("  `rank 1` means the oracle's counterpart was the strongest. `margin` is\n")
	fmt.Printf("  how far it beat the best WRONG answer; negative means it did not.\n")

	for _, m := range measures {
		t := &tally{}
		results[m] = t
		fmt.Printf("\n  %s\n", m)
		for _, from := range evA {
			r := recognise(m, from, evB)
			switch {
			case !r.hasCounterpart:
				t.noCounterpart++
				if r.ranked[0].sim >= observe.VisualScreenMatch {
					t.wouldBindWrong++
				}
				// AND THE RUNNER-UP, because the existing conservative binder refuses
				// when two candidates qualify. Whether it would have caught these
				// depends entirely on whether the wrong winner was unopposed.
				fmt.Printf("    %-10s no counterpart; best %s=%.3f, next %s\n",
					from.id, r.ranked[0].to.id, r.ranked[0].sim, runnerUp(r))
			case r.correctAt == 0:
				t.correctTop++
				t.margins = append(t.margins, r.margin)
				if len(r.ranked) > 1 && r.ranked[1].sim >= observe.VisualScreenMatch {
					t.wouldRefuseCorrect++
				}
				fmt.Printf("    %-10s rank 1   margin %+.3f   best %.3f, next %s\n",
					from.id, r.margin, r.ranked[0].sim, runnerUp(r))
			default:
				t.wrongTop++
				t.margins = append(t.margins, r.margin)
				fmt.Printf("    %-10s rank %d   margin %+.3f   (%s won at %.3f)\n",
					from.id, r.correctAt+1, r.margin,
					r.ranked[0].to.id, r.ranked[0].sim)
			}
		}
	}

	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  IS THERE A SEPARATION?\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("  %-12s %7s %6s %6s %9s %9s %s\n",
		"measure", "rank 1", "wrong", "absent", "BIND-WRONG", "refuse-ok",
		"worst margin")
	for _, m := range measures {
		t := results[m]
		sort.Float64s(t.margins)
		worst := 0.0
		if len(t.margins) > 0 {
			worst = t.margins[0]
		}
		verdict := "SEPARATED"
		if t.wrongTop > 0 || worst <= 0 {
			verdict = "no separation"
		}
		fmt.Printf("  %-12s %7d %6d %6d %9d %9d %+9.3f  %s\n",
			m, t.correctTop, t.wrongTop, t.noCounterpart,
			t.wouldBindWrong, t.wouldRefuseCorrect, worst, verdict)
	}
	fmt.Printf("\n  `rank 1` and `wrong` are about DISCRIMINATION: where a counterpart\n")
	fmt.Printf("  exists, did the measure put it top. `BIND-WRONG` is about ABSENCE:\n")
	fmt.Printf("  a screen fold B never saw, whose best candidate clears the\n")
	fmt.Printf("  production floor UNOPPOSED. The conservative binder refuses two\n")
	fmt.Printf("  candidates above the floor and cannot refuse one, so every\n")
	fmt.Printf("  BIND-WRONG is a confident wrong answer it would give. `refuse-ok`\n")
	fmt.Printf("  is that rule's cost: a CORRECT answer declined as ambiguous.\n\n")
	fmt.Printf("  A measure is SEPARATED only when every correct counterpart was the\n")
	fmt.Printf("  strongest AND beat the best wrong answer by a positive margin. That is\n")
	fmt.Printf("  the condition under which a floor could be derived; anything less is a\n")
	fmt.Printf("  threshold chosen to fit, which is what Experiment-029 refuted.\n")
	return nil
}

// selfSimilarity is a screen against its own evidence — the steadiness of the measure.
func selfSimilarity(measure string, e screenEvidence) float64 {
	switch measure {
	case "arrangement", "shape", "detector":
		// These are built from the whole screen's looks already, so comparing the
		// screen with itself is 1 by construction and says nothing. Reported as the
		// spread of the per-look proposal count instead, which is the variation those
		// measures are actually exposed to.
		return measureScore(measure, e, e)
	}
	worst := 1.0
	switch measure {
	case "palette":
		for i := range e.palette {
			for j := i + 1; j < len(e.palette); j++ {
				if s := screen.HistIntersection(e.palette[i], e.palette[j]); s < worst {
					worst = s
				}
			}
		}
	case "grid":
		for i := range e.grid {
			for j := i + 1; j < len(e.grid); j++ {
				one := screenEvidence{grid: [][]screen.Histogram{e.grid[i]}}
				two := screenEvidence{grid: [][]screen.Histogram{e.grid[j]}}
				if s := measureScore("grid", one, two); s < worst {
					worst = s
				}
			}
		}
	}
	if worst == 1.0 && len(e.palette) < 2 {
		return 0
	}
	return worst
}

// runnerUp is the second-strongest candidate, which is what the existing ambiguity rule reads.
func runnerUp(r recognition) string {
	if len(r.ranked) < 2 {
		return "none"
	}
	return fmt.Sprintf("%s=%.3f", r.ranked[1].to.id, r.ranked[1].sim)
}

func proposalRange(n []int) (int, int) {
	if len(n) == 0 {
		return 0, 0
	}
	lo, hi := n[0], n[0]
	for _, v := range n {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return lo, hi
}

func short(m string) string {
	if len(m) < 4 {
		return m
	}
	return m[:4]
}
