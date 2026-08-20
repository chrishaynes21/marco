package observe_test

import (
	"fmt"
	"sort"
	"testing"
)

// Choosing how a settled screen's durable composition is derived from what a session saw.
//
// # The measured defect
//
// Durable identity was `round(mean(role count over every observation placed in the state))`.
// Windows Settings home reports a byte-identical accessibility tree across eight consecutive
// settled snapshots — `button=21` every time — yet minted three durable subjects at 15, 17 and 19,
// because the mean averages the partial renders a session catches on the way in. Identity ended up
// describing how long somebody looked at a screen rather than what the screen is.
//
// # Why this is a bake-off rather than a fix
//
// "Partial renders are subsets, so take the max" is the obvious answer and it is wrong in a way
// that only shows up on a case nobody would think to try: one transient overlay frame permanently
// redefines the screen. So every candidate is run against every case, including the ones designed
// to kill it, and the table is printed rather than summarised.
//
// These aggregators are over a single role's count per observation. The real signature is a map of
// them, and each role is independent.

type aggregator struct {
	name string
	fn   func([]int) int
}

func roundedMean(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	sum := 0
	for _, x := range xs {
		sum += x
	}
	// The production rounding: int(math.Round(sum/n)).
	return int(float64(sum)/float64(len(xs)) + 0.5)
}

func rawMax(xs []int) int {
	best := 0
	for _, x := range xs {
		if x > best {
			best = x
		}
	}
	return best
}

func median(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int{}, xs...)
	sort.Ints(s)
	return s[len(s)/2]
}

// modeLargest is the most frequently observed count, and the LARGEST of them when several tie.
//
// The tie-break matters and is not arbitrary. A session too short for anything to recur sees each
// stage of the render exactly once — 12, 16, 19, 21 — and every candidate ties. Of those, only the
// last is the finished screen; the rest are it, arriving. Preferring the largest is what makes a
// four-observation visit and a forty-observation visit agree.
func modeLargest(xs []int) int {
	tally := map[int]int{}
	for _, x := range xs {
		tally[x]++
	}
	best, bestN := 0, -1
	for v, n := range tally {
		if n > bestN || (n == bestN && v > best) {
			best, bestN = v, n
		}
	}
	return best
}

var aggregators = []aggregator{
	{"rounded mean", roundedMean},
	{"raw max", rawMax},
	{"median", median},
	{"mode/largest", modeLargest},
}

type bakeCase struct {
	name string
	obs  []int
	want int
	why  string
}

// The cases. Each one is a real failure mode, not a spread of numbers.
var bakeCases = []bakeCase{{
	name: "partial render into a stable screen",
	obs:  []int{12, 16, 19, 21, 21, 21},
	want: 21,
	why:  "the screen IS the finished one; the rest is it arriving",
}, {
	name: "short visit, nothing recurs",
	obs:  []int{12, 16, 19, 21},
	want: 21,
	why:  "must agree with the long visit below, or identity depends on how long you looked",
}, {
	name: "long visit",
	obs:  []int{12, 16, 19, 21, 21, 21, 21, 21},
	want: 21,
	why:  "same place as the short visit",
}, {
	name: "same evidence, reordered",
	obs:  []int{21, 12, 21, 19, 21, 16},
	want: 21,
	why:  "arrival order is not a property of the place",
}, {
	name: "one-frame transient overlay",
	obs:  []int{21, 21, 25, 21, 21},
	want: 21,
	why:  "a flyout that appears once must not permanently redefine the screen",
}, {
	name: "the screen genuinely settles larger",
	obs:  []int{21, 21, 25, 25, 25, 25},
	want: 25,
	why:  "a real new composition must still be representable",
}, {
	name: "rare intrinsic role, absent only while rendering",
	obs:  []int{0, 0, 1, 1, 1, 1},
	want: 1,
	why:  "a scroll bar the settled screen always has must not vanish",
}}

// TestTheCompositionBakeOff prints the table and holds the choice.
//
// It fails if the CHOSEN aggregator misses a case, and separately records which rejected candidate
// each case kills — so a future reader can see that the choice was forced rather than preferred.
func TestTheCompositionBakeOff(t *testing.T) {
	var b []string
	b = append(b, fmt.Sprintf("%-42s %-6s %s", "case", "want", "rounded-mean  raw-max  median  mode/largest"))
	killed := map[string][]string{}

	for _, c := range bakeCases {
		row := fmt.Sprintf("%-42s %-6d", c.name, c.want)
		for _, a := range aggregators {
			got := a.fn(c.obs)
			mark := " "
			if got != c.want {
				mark = "✗"
				killed[a.name] = append(killed[a.name], c.name)
			}
			row += fmt.Sprintf("  %2d%s        ", got, mark)
		}
		b = append(b, row)
	}
	t.Log("\n" + joinLines(b))
	for _, a := range aggregators {
		if len(killed[a.name]) == 0 {
			t.Logf("SURVIVES: %s", a.name)
			continue
		}
		t.Logf("killed  : %-14s by %v", a.name, killed[a.name])
	}

	// THE CHOICE. Mode with a largest-count tie-break is the only candidate that survives every
	// case, and each rejection below is a case some other candidate fails.
	for _, c := range bakeCases {
		if got := modeLargest(c.obs); got != c.want {
			t.Errorf("%s: mode/largest gave %d, want %d — %s", c.name, got, c.want, c.why)
		}
	}
}

// Each rejected aggregator is killed by a NAMED case, so the choice cannot quietly be reverted to
// one of them.
func TestEachRejectedAggregatorIsKilledBySomething(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func([]int) int
		obs  []int
		bad  int
		why  string
	}{{
		name: "rounded mean", fn: roundedMean, obs: []int{12, 16, 19, 21, 21, 21}, bad: 18,
		why: "averages the render-in frames, so identity depends on the sampling",
	}, {
		name: "raw max", fn: rawMax, obs: []int{21, 21, 25, 21, 21}, bad: 25,
		why: "one transient overlay frame permanently redefines the screen",
	}, {
		name: "median", fn: median, obs: []int{12, 16, 19, 21}, bad: 19,
		why: "a short visit lands mid-render",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.fn(tc.obs); got != tc.bad {
				t.Errorf("%s gave %d over %v; this case was recorded as killing it with %d (%s)",
					tc.name, got, tc.obs, tc.bad, tc.why)
			}
			if modeLargest(tc.obs) == tc.bad {
				t.Errorf("mode/largest agrees with the rejected %s on %v; the case does not "+
					"separate them", tc.name, tc.obs)
			}
		})
	}
}

func joinLines(xs []string) string {
	out := ""
	for _, x := range xs {
		out += x + "\n"
	}
	return out
}
