package observe_test

import (
	"fmt"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
)

// What the second comparison costs.
//
// The whole-surface comparison was already run once per candidate state per inference. This adds
// a second pass over the same feature vectors, grouped by cell — the same asymptotics, a
// constant factor more work. Worth measuring rather than asserting, because "one more pass" is
// what everybody says about the pass that turned out to dominate.
//
// The number to compare against is the thing it sits inside: an observation cycle against a real
// application measured 176–298 ms, and a session samples every 500 ms–1 s. Anything here in the
// microseconds is free.
//
// Run with:
//
//	go test ./internal/director/observe -run xxx -bench LocalChange -benchmem

func costSurface(structures int) map[string]float64 {
	// A realistic split: most of the surface persists, a minority carries the state. The
	// proportions matter to the COST too — the grouping pass is over cells, and a surface
	// whose structure is concentrated has fewer occupied cells than one that is spread.
	s := screenfixture.Surface{
		Chrome:      structures * 5 / 6,
		Content:     structures / 6,
		ContentRole: "list_item",
	}
	return observe.FeaturesForTest(observe.NewScreenSignature(s.Regions()))
}

func BenchmarkLocalChangeAtAccessibilityScale(b *testing.B) {
	// The scales worth knowing: a small dialog, an ordinary window, a rich application, and
	// the largest accessibility tree these sessions have actually produced.
	for _, n := range []int{50, 150, 350, 700} {
		before := costSurface(n)
		s := screenfixture.Surface{
			Chrome: n * 5 / 6, Content: n / 6, ContentRole: "checkbox",
		}
		after := observe.FeaturesForTest(observe.NewScreenSignature(s.Regions()))

		b.Run(fmt.Sprintf("%d-structures", n), func(b *testing.B) {
			for b.Loop() {
				observe.LocalChangeForTest(before, after)
			}
		})
	}
}

// And the worst case the bound permits, which no fixture reaches.
//
// The scaled benchmark above is flat because a surface of a few roles produces a few features
// however many structures it has — which is the property worth having, and is also why it does
// not measure the ceiling. This does: a signature saturated at MaxSignatureKeys in both halves,
// entirely replaced, is the most work this comparison can ever be asked to do.
func BenchmarkLocalChangeAtTheSignatureBound(b *testing.B) {
	saturated := func(role string) map[string]float64 {
		f := map[string]float64{}
		for i := range observe.MaxSignatureKeys {
			f[fmt.Sprintf("role:%s_%d", role, i)] = 4
			f[fmt.Sprintf("cell:%s_%d@%d,%d", role, i, i%3, (i/3)%3)] = 4
		}
		return f
	}
	before, after := saturated("a"), saturated("b")
	b.Logf("%d features per side", len(before))
	for b.Loop() {
		observe.LocalChangeForTest(before, after)
	}
}

// The comparison is bounded by the SIGNATURE, not by the screen.
//
// The load-bearing fact behind the cost, and the reason a bigger application cannot make this
// grow without limit: a signature is capped at MaxSignatureKeys per half, so the feature vector
// this walks has a ceiling however many structures the window exposes. A seven-hundred-element
// tree and a fifty-element one differ in what reaches the signature, not in how much of it the
// comparison walks.
func TestTheLocalComparisonIsBoundedByTheSignature(t *testing.T) {
	for _, n := range []int{50, 150, 350, 700} {
		f := costSurface(n)
		if len(f) > 2*observe.MaxSignatureKeys {
			t.Errorf("a %d-structure surface produced %d features, above the bound of %d",
				n, len(f), 2*observe.MaxSignatureKeys)
		}
		t.Logf("%3d structures -> %d features", n, len(f))
	}
}
