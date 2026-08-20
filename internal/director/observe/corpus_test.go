package observe_test

import (
	"fmt"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
)

// The structural corpus, and the one production matcher measured against it.
//
// # What this is for
//
// A local-state algorithm is a claim about what counts as going somewhere, and claims like that
// are only checkable comparatively. This is the apparatus for that: twenty-one structural
// situations, each labelled with what a person would say happened, and a seam narrow enough that
// a future candidate can be measured beside the incumbent without any of it reaching production.
//
// One candidate was measured this way and rejected — see
// [[ADR-040-a-few-scales-were-not-better-than-one]]. Its implementation is gone; the ADR carries
// the numbers, and this carries the means to produce more.
//
// # How a verdict is scored
//
//	a different place      → the matcher SHOULD report a replacement
//	the same place         → it should NOT
//	a property of a place  → it should NOT
//
// The last is what makes it hard. A matcher can win the meaningful column by answering "changed"
// to everything, and the corpus is built so that strategy loses.

// localMatcher is the seam a candidate would satisfy. One implementation, deliberately.
type localMatcher interface {
	Compare(before, after []observe.ShadowRegion) bool
	Name() string
}

// production is the matcher Director actually uses, asked through its own exported hook.
type production struct{}

func (production) Name() string { return "local comparison" }

func (production) Compare(before, after []observe.ShadowRegion) bool {
	worst, _ := observe.LocalChangeForTest(
		observe.FeaturesForTest(observe.NewScreenSignature(before)),
		observe.FeaturesForTest(observe.NewScreenSignature(after)))
	// 1 is how the comparison abstains — see localChange.
	return worst != 1
}

func TestTheProductionMatcherAgainstTheStructuralCorpus(t *testing.T) {
	m := localMatcher(production{})
	var falseChange, missed int

	t.Logf("%-44s %-24s %s", "case", "means", m.Name())
	for _, c := range screenfixture.Corpus() {
		got := m.Compare(c.From, c.To)
		want := c.Means == screenfixture.Somewhere
		mark, said := " ", "same place"
		if got {
			said = "a different place"
		}
		if got != want {
			mark = "!"
			if got {
				falseChange++
			} else {
				missed++
			}
		}
		t.Logf("%-44s %-24s %s%s", c.Name, c.Means, mark, said)
		if c.Note != "" {
			t.Logf("%-44s   ↳ %s", "", c.Note)
		}
	}
	t.Logf("false state changes %d   missed meaningful changes %d", falseChange, missed)

	// THE assertion, and only this one. Inventing places is unrecoverable — every
	// application would mint one whenever somebody scrolled — while missing a change only
	// makes Marco less useful. The known miss is recorded in ADR-040 and is an information
	// limit rather than a tuning failure, so it is logged and not asserted away.
	if falseChange > 0 {
		t.Errorf("the production matcher reports %d false state change(s)", falseChange)
	}
	if missed > 1 {
		t.Errorf("%d meaningful changes missed, up from the one ADR-040 records; something "+
			"the corpus used to catch is no longer caught", missed)
	}
}

func BenchmarkTheProductionMatcher(b *testing.B) {
	m := production{}
	for _, n := range []int{50, 150, 350, 700, 1200} {
		s := screenfixture.Surface{
			Chrome: n * 5 / 6, Content: n / 6, ContentRole: "list_item",
		}
		before, after := s.Regions(), s.ContentReplaced("checkbox").Regions()
		b.Run(fmt.Sprintf("%d-structures", n), func(b *testing.B) {
			for b.Loop() {
				m.Compare(before, after)
			}
		})
	}
}
