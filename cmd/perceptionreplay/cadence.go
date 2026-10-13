package main

// CAN THE SCREEN MODEL SURVIVE A REAL OBSERVATION RATE?
//
// # The blocker this exists to measure
//
// `VisualSettleLooks = 3` was chosen against a 10fps recording, where three looks is 300ms. The
// live observation path samples on a ~3.8-second discovery cadence, where three looks is
// **eleven seconds**. On that clock the damping would discard every real navigation, and the
// code would run and produce plausible nonsense.
//
// The owner refused the obvious fixes in advance, and was right to:
//
//	Do not simply convert three consecutive looks into a 300ms threshold or reduce the count
//	to one without measuring the consequences. Determine what temporal evidence is actually
//	available at each cadence.
//
// Converting to 300ms is impossible at 3.8s — three observations cannot fit inside 300ms when
// they arrive every 3800. Reducing the count to one removes the noise protection that made the
// offline result worth having. So the question is not which rule to pick; it is **how much
// temporal evidence exists at each rate**, and this measures that.
//
// # And it measures PHASE, which is the part that is easy to miss
//
//	Crucially, test multiple sampling offsets. A 3.8-second sampler could observe completely
//	different portions of the same route depending on when its first frame arrives. No
//	settling algorithm can recover a dialog that never appears in a sampled frame.
//
// So every rate is run at every offset inside its own interval, and a screen whose frames are
// all skipped is reported as UNOBSERVABLE rather than as a screen the model failed to find.
// That distinction is the whole value of the exercise: one is a limit of the sampling and the
// other would be a defect in the settling.
//
// # The frames are proposed ONCE
//
// An arrangement is a pure function of a frame, and this experiment is about the stabiliser
// rather than the proposer. Proposing per (rate, offset) would cost minutes and measure the same
// pixels repeatedly; proposing once and replaying the stabiliser over subsets costs one pass and
// makes every cell of the table comparable by construction.

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
)

// proposeMS is what the pixel proposer cost over the last read, summed.
//
// The measurement behind the architecture question: whether a cheap local observation path can
// run faster than the AI interpretation path it currently shares a cadence with. A number, not
// an assumption.
var proposeMS int64

// cadenceLook is one frame, already seen.
type cadenceLook struct {
	index int
	atMS  int64
	arr   observe.VisualArrangement
	rows  []observe.Region
	// presses made during this frame's slot, with their positions.
	presses []observe.PointerAt
}

// cadenceCell is what one (rate, offset) produced.
type cadenceCell struct {
	IntervalMS int
	OffsetMS   int
	// Settle is the rule this cell ran under, in consecutive looks.
	Settle     int
	Looks      int
	Screens    int
	Promotions int
	// Matched is how many of the BASELINE's screens this cadence settled on something
	// alike, and Sampled how many of them it saw a frame of at all.
	//
	// The gap between them is the measurement: a baseline screen this cadence sampled and
	// did not settle is a settling failure, and one it never sampled is a limit of the
	// rate. Reporting a single "missed" number would conflate the two.
	Matched, Sampled int
	// Extra is settled screens that match no baseline screen — over-segmentation, or a
	// composition only visible at this rate.
	Extra int
	// ConfirmMS is the median time between a new arrangement first being seen and it
	// being promoted, which is how long Director would take to know it had moved.
	ConfirmMS int64
	// RouteRun is the longest non-overlapping repeated run in the walk — the route
	// correspondence across the two demonstrations, at this cadence.
	RouteRun int
	// Click, Auto, Unknown are the causes of the changes it attributed.
	Click, Auto, Unknown int
}

// readCadenceLooks reads the recording and proposes every frame ONCE.
//
// It calls the same readers, the same proposer and the same normalisation `replayFrames` does —
// `readJSONL`, `structure.Propose`, `shapesFrom`, `regionsOf`, `observe.ArrangeLook` — so an
// arrangement measured here is the same arrangement measured there. The one thing it does not
// do is fold anything: the folding is what varies per cadence.
func readCadenceLooks(dir string, limit int) ([]cadenceLook, error) {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return nil, err
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("%s carries no frames.jsonl entries", dir)
	}
	inputs, _ := readJSONL[inputRecord](filepath.Join(dir, "input.jsonl"))
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].AtMS < inputs[j].AtMS })

	opt := structure.DefaultOptions()
	out := make([]cadenceLook, 0, len(frames))
	nextInput := 0
	proposeMS = 0
	for i, fr := range frames {
		if limit > 0 && i >= limit {
			break
		}
		img, err := readPNG(filepath.Join(dir, filepath.FromSlash(fr.File)))
		if err != nil {
			// A SANITISED GAP, not a failure. Some recordings have had their pixels
			// removed; the frame still happened and its inputs still belong to the
			// session, so the look is kept with nothing in it — which the stabiliser
			// reads as an unobserved look and never as a screen.
			out = append(out, cadenceLook{index: fr.Index, atMS: fr.AtMS})
			continue
		}
		t0 := time.Now()
		res := structure.Propose(img, opt)
		proposeMS += time.Since(t0).Milliseconds()
		rows := regionsOf(shapesFrom(img, res.Candidates))
		lk := cadenceLook{
			index: fr.Index, atMS: fr.AtMS,
			arr: observe.ArrangeLook(rows), rows: rows,
		}
		// THE PRESSES THAT HAPPENED DURING THIS FRAME'S SLOT, consumed once each, the
		// same consuming walk replayFrames makes. A press belongs to the frame it
		// happened in whatever rate reads it later.
		for nextInput < len(inputs) && inputs[nextInput].AtMS <= fr.AtMS {
			in := inputs[nextInput]
			nextInput++
			if observe.NavIntent(in.Intent) == observe.NavPoint {
				lk.presses = append(lk.presses, in.Where)
			}
		}
		out = append(out, lk)
	}
	return out, nil
}

// runCadence folds one (rate, offset) through the real stabiliser and correlator.
//
// `everyMS` is the sampling interval and `offsetMS` how late the first look is taken. A look is
// taken when the elapsed time since the last one reaches the interval, from the recording's own
// measured timestamps — not from a frame count, because a recording's frames are not evenly
// spaced and a count would measure the encoder rather than the clock.
func runCadence(looks []cadenceLook, everyMS, offsetMS, settle int,
	base []observe.VisualScreenSeen,
	baseArr func(observe.VisualScreenID) observe.VisualArrangement) cadenceCell {

	cell := cadenceCell{IntervalMS: everyMS, OffsetMS: offsetMS, Settle: settle}
	var screens observe.VisualScreens
	screens.SettleAfter(settle)
	var clicks observe.ClickTargets

	// firstSeen is when an arrangement that is NOT the standing screen was first seen, so
	// a promotion can say how long it took to confirm.
	var pendingSince int64 = -1
	var confirms []int64
	// sampledBase is which baseline screens this cadence saw a frame of at all.
	sampledBase := map[observe.VisualScreenID]bool{}
	// baseOf maps a frame to the baseline screen it belongs to, by arrangement.
	matchBase := func(a observe.VisualArrangement) observe.VisualScreenID {
		best, bestSim := observe.VisualScreenID(""), 0.0
		for _, b := range base {
			if sim := observe.ArrangementSimilarity(baseArr(b.ID), a); sim > bestSim {
				best, bestSim = b.ID, sim
			}
		}
		if bestSim < observe.VisualScreenMatch {
			return ""
		}
		return best
	}

	clock := time.Unix(1700000000, 0).UTC()
	var lastAt int64 = -1
	n := 0
	for _, lk := range looks {
		if lk.atMS < int64(offsetMS) {
			continue
		}
		// An interval of zero needs no special case: the recording's times increase,
		// so `atMS - lastAt < 0` is never true and every look is taken. A guard for it
		// would be a second statement of the same arithmetic.
		if lastAt >= 0 && lk.atMS-lastAt < int64(everyMS) {
			continue
		}
		lastAt = lk.atMS
		n++
		cell.Looks++
		if id := matchBase(lk.arr); id != "" {
			sampledBase[id] = true
		}

		standing := screens.Screen()
		for _, w := range lk.presses {
			clicks.Press(n-1, w, standing, observe.Proposed(lk.rows, n-1, 0, 0))
		}
		// DID THIS LOOK AGREE WITH THE STANDING SCREEN? Asked of the ARRANGEMENTS, not
		// of what Observe returns: Observe answers the held screen for a look that
		// differed but has not settled, which is the whole point of the damping — and a
		// latency measured from its answer is therefore always zero. That bug made this
		// column read 0 at every rate until the comparison moved here.
		agreed := standing != observe.VisualScreenUnknown && lk.arr.Observed() &&
			observe.ArrangementSimilarity(screens.Arrangement(standing), lk.arr) >=
				observe.VisualScreenMatch
		screens.Observe(lk.arr, clock.Add(time.Duration(lk.atMS)*time.Millisecond))
		if t, ok := screens.Promoted(); ok {
			clicks.Arrived(n-1, t)
			if pendingSince >= 0 {
				confirms = append(confirms, lk.atMS-pendingSince)
			}
			pendingSince = -1
			continue
		}
		switch {
		case !lk.arr.Observed():
			// A blank look neither starts nor ends a wait.
		case agreed:
			pendingSince = -1
		case pendingSince < 0:
			pendingSince = lk.atMS
		}
	}
	clicks.Close()

	seen := screens.Seen()
	cell.Screens = len(seen)
	cell.Promotions = screens.Stability().Promotions
	cell.RouteRun = len(func() []observe.VisualScreenID {
		run, _ := longestRepeatedRun(screens.Walk())
		return run
	}())
	matched := map[observe.VisualScreenID]bool{}
	for _, sc := range seen {
		if id := matchBase(screens.Arrangement(sc.ID)); id != "" {
			matched[id] = true
		} else {
			cell.Extra++
		}
	}
	cell.Matched, cell.Sampled = len(matched), len(sampledBase)
	for _, st := range clicks.Evidence().Steps {
		switch st.Cause {
		case observe.CauseClick:
			cell.Click += st.Count
		case observe.CauseAutomatic:
			cell.Auto += st.Count
		default:
			cell.Unknown += st.Count
		}
	}
	if len(confirms) > 0 {
		sort.Slice(confirms, func(i, j int) bool { return confirms[i] < confirms[j] })
		cell.ConfirmMS = confirms[len(confirms)/2]
	}
	return cell
}

// cadenceRates are the intervals measured, in milliseconds.
//
// The owner's list, plus the one the recording itself ran at. 3800 is the live discovery cadence
// measured in [[Experiment-008-unknown-game-discovery]]; the rest bracket it.
// 0 means EVERY FRAME, which is the recording's own rate and the baseline the rest are
// compared against. It is separate from 100 on purpose: the frames are about 100ms apart and
// they jitter, so a real 100ms sampler skips a few of them — and the difference between "every
// frame" and "every 100ms" is itself worth seeing.
var cadenceRates = []int{0, 100, 300, 500, 1000, 3800}

// reportCadence measures every rate at every phase inside its own interval.
func reportCadence(dir string, limit int) error {
	looks, err := readCadenceLooks(dir, limit)
	if err != nil {
		return err
	}
	if len(looks) == 0 {
		return fmt.Errorf("no frames to measure")
	}
	// THE BASELINE IS THE RECORDING'S OWN RATE, taken at offset zero. Every other cell is
	// compared against it, so "retained" means retained relative to what the fastest
	// available observation saw — not relative to a truth nobody has.
	var bscreens observe.VisualScreens
	clock := time.Unix(1700000000, 0).UTC()
	for _, lk := range looks {
		bscreens.Observe(lk.arr, clock.Add(time.Duration(lk.atMS)*time.Millisecond))
	}
	base := bscreens.Seen()

	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("CADENCE SENSITIVITY  %d frames, %dms span, settle rule %d consecutive looks\n",
		len(looks), looks[len(looks)-1].atMS-looks[0].atMS, observe.VisualSettleLooks)
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  baseline at the recording's own rate: %d screen(s)\n", len(base))
	// THE PROPOSER'S OWN COST, which is the evidence for whether a faster local
	// observation path is possible at all. Measured over this pass, on these frames.
	fmt.Printf("  the pixel proposer cost %dms over %d frame(s) — %.0fms each, so about\n",
		proposeMS, len(looks), float64(proposeMS)/float64(len(looks)))
	fmt.Printf("  %.0f look(s) per second on one core, against the ~3.8s AI discovery "+
		"cadence.\n\n", 1000/(float64(proposeMS)/float64(len(looks))))
	fmt.Printf("  %-6s %-7s %-7s %6s %5s %6s %7s %7s %6s %8s %6s %s\n",
		"settle", "every", "offset", "looks", "scrn", "promo", "matched", "sampled",
		"extra", "confirm", "route", "causes c/a/u")
	for _, settle := range []int{1, 2, 3} {
		for _, every := range cadenceRates {
			// EVERY PHASE INSIDE THE INTERVAL, at the recording's own granularity. A
			// 3.8-second sampler has 38 distinct phases over a 10fps recording, and the
			// spread between them is the answer to "could it have missed the dialog".
			step := 100
			if every == 0 {
				c := runCadence(looks, 0, 0, settle, base, bscreens.Arrangement)
				fmt.Printf("  %-6d %-7s %-7d %6d %5d %6d %7d %7d %6d %8d %6d %d/%d/%d\n",
					settle, "all", 0, c.Looks, c.Screens, c.Promotions,
					c.Matched, c.Sampled, c.Extra, c.ConfirmMS, c.RouteRun,
					c.Click, c.Auto, c.Unknown)
				continue
			}
			// EVERY PHASE, SUMMARISED. Thirty-eight rows per rate is a listing nobody
			// reads; what matters is the SPREAD across phases, because that is the answer
			// to "could this sampler have missed the dialog". So the best and worst phase
			// by screens matched are printed, and the phase that produced each.
			var bestC, worstC cadenceCell
			first := true
			for off := 0; off < every; off += step {
				c := runCadence(looks, every, off, settle, base, bscreens.Arrangement)
				if first || c.Matched > bestC.Matched {
					bestC = c
				}
				if first || c.Matched < worstC.Matched {
					worstC = c
				}
				first = false
			}
			for _, c := range []cadenceCell{bestC, worstC} {
				tag := fmt.Sprintf("%d", c.OffsetMS)
				fmt.Printf("  %-6d %-7d %-7s %6d %5d %6d %7d %7d %6d %8d %6d %d/%d/%d\n",
					settle, c.IntervalMS, tag, c.Looks, c.Screens, c.Promotions,
					c.Matched, c.Sampled, c.Extra, c.ConfirmMS, c.RouteRun,
					c.Click, c.Auto, c.Unknown)
				if bestC.OffsetMS == worstC.OffsetMS {
					break
				}
			}
		}
	}
	fmt.Printf("\n  two rows per rate: the phase that matched the MOST baseline screens and\n")
	fmt.Printf("  the phase that matched the fewest. One row means every phase was equal.\n")
	fmt.Printf("  matched = baseline screens this cadence settled on something alike\n")
	fmt.Printf("  sampled = baseline screens it saw a frame of AT ALL\n")
	fmt.Printf("  the gap between them is a SETTLING failure; a low `sampled` is a limit\n")
	fmt.Printf("  of the rate, and no settling rule can recover a screen never sampled.\n")
	return nil
}

func repeat(s string, n int) string {
	out := ""
	for range n {
		out += s
	}
	return out
}
