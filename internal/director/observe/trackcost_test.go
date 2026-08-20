package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
)

// What raising the track bound costs.
//
// Track assignment is O(regions x tracks) per inference, so an eightfold bound is an eightfold
// worst case. The number that matters is the comparison with the thing it sits inside: an
// observation cycle against a real application measured 176–298 ms, and a session samples every
// 500 ms–1 s.
//
// Run with:
//
//	go test ./internal/director/observe -run xxx -bench TrackAssignment -benchmem
//
// If one inference ever approaches a millisecond, the bound has started to matter and the
// arithmetic in shadowtrack.go needs revisiting.

// BenchmarkTrackAssignmentAtAccessibilityScale folds a realistic screen into a tracker that
// already holds several screens' worth of structure — the worst case the new bound permits.
func BenchmarkTrackAssignmentAtAccessibilityScale(b *testing.B) {
	// Fill the tracker with structure from several distinct screens, so the track table is
	// near the size a long session would reach.
	var k observe.ShadowTracker
	var screens [][]observe.ShadowRegion
	for i := range 8 {
		// Eight screens at distinct offsets, so their structures do not overlap and each
		// one really begins its own tracks. That is the worst case the bound permits — a
		// session that visited eight structurally unrelated screens — and it is the number
		// worth knowing rather than the one a set of overlapping fixtures happens to reach.
		screens = append(screens,
			screenfixture.Jitter(screenfixture.Editor(), 0.09*float64(i)))
	}
	for i := range 80 {
		s := screens[i%len(screens)]
		k.Observe(&observe.ShadowSample{Ran: true, TargetProven: true, Regions: s},
			observe.StructuralView{Source: observe.StructureFused, Regions: s})
	}
	b.Logf("tracker holds %d tracks (bound %d), evicted %d",
		len(k.Tracks()), observe.MaxActiveTracks, k.Evicted)

	frame := screenfixture.Editor()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		k.Observe(&observe.ShadowSample{Ran: true, TargetProven: true, Regions: frame},
			observe.StructuralView{Source: observe.StructureFused, Regions: frame})
	}
}

// Both screens of a two-screen session earn their own structure.
//
// THE regression for the track bound. With the old bound of 128 — exactly one realistic
// accessibility screen — the second screen a session visited could begin no tracks at all, so it
// formed no structural group, earned no hypothesis, could never become a durable subject, and
// could never be one end of a relationship. Every session on real software could see a
// transition and none could remember one.
func TestBothScreensOfASessionEarnTheirOwnStructure(t *testing.T) {
	var k observe.ShadowTracker
	editor, settings := screenfixture.Editor(), screenfixture.Settings()

	n := 0
	show := func(regions []observe.ShadowRegion, times int) {
		for range times {
			n++
			k.Observe(&observe.ShadowSample{Ran: true, TargetProven: true, Regions: regions},
				observe.StructuralView{Source: observe.StructureFused, Regions: regions})
		}
	}
	for range 2 {
		show(editor, 6)
		show(settings, 10)
	}

	states, tracks := k.States(), k.Tracks()
	if len(states) != 2 {
		t.Fatalf("two screens produced %d state(s)", len(states))
	}
	if k.Evicted > 0 {
		t.Errorf("%d tracks were evicted from a two-screen session; the bound is below the "+
			"size of the evidence a real application produces", k.Evicted)
	}

	groups := observe.Groups(tracks, states)
	perState := map[observe.ScreenStateID]int{}
	for _, g := range groups {
		perState[g.State]++
	}
	if len(perState) < 2 {
		t.Fatalf("structure formed for %d of 2 screens: %v.\n"+
			"A screen with no structure of its own can never earn a hypothesis, never "+
			"become a durable subject, and never be one end of a relationship",
			len(perState), perState)
	}
}
