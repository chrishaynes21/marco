package observe_test

import (
	"sort"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// THE JUDGEMENT IS FREE COMPARED TO THE READING IT JUDGES.
//
// Measured over the seven committed 37C samples, 200 classifications each:
//
//	samples 1400   median below timer resolution   p95 514µs   max 714µs
//
// against the acquisition those same readings cost, measured in 37C:
//
//	Settings   104–120 ms
//	Explorer   1489–1515 ms
//
// So the classifier is well under one percent of the cheapest reading it will ever see, and
// three ten-thousandths of the dearest. There is nothing here to optimise, and 37D deliberately
// did not try.
//
// # Why the ceiling is where it is and not tighter
//
// [ReachOfState] compares every structure against every other, so its cost grows with the
// square of the reading. At 140 elements that is nineteen thousand comparisons and half a
// millisecond, which is fine. At two thousand elements it would be four million, and it would
// not be.
//
// This is not a benchmark and must not become one — a tight bound would fail on a loaded
// machine and teach everyone to ignore it. It is a shape check: if a reading of this size ever
// takes tens of milliseconds to judge, something has become quadratic in the wrong dimension
// and the cost has moved from where the 37C measurements said it was.
func TestJudgingAReadingCostsAlmostNothing(t *testing.T) {
	samples := readCorpusSamples(t)
	var all []time.Duration
	var widest int
	for _, s := range samples {
		if len(s.Elements) > widest {
			widest = len(s.Elements)
		}
		totals := s.asTotals()
		for i := 0; i < 200; i++ {
			start := time.Now()
			observe.ReachOfState(totals, "state_1")
			all = append(all, time.Since(start))
		}
	}
	if len(all) == 0 {
		t.Fatal("nothing was measured")
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	p95 := all[int(0.95*float64(len(all)-1))]
	max := all[len(all)-1]
	t.Logf("samples=%d median=%v p95=%v max=%v (widest reading %d elements)",
		len(all), all[len(all)/2], p95, max, widest)

	const ceiling = 25 * time.Millisecond
	if p95 > ceiling {
		t.Errorf("p95 is %v for a reading of at most %d elements, over the %v ceiling.\n"+
			"37C measured the accessibility walk this judges at 104ms on Settings. A "+
			"judgement approaching that is no longer free, and the cost model in "+
			"ADR-103 stops being true.", p95, widest, ceiling)
	}
}
