package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// What state segmentation costs, against what it sits beside.
//
// The bar is not "fast". ScreenParser is ~850ms and ~1.25GB per inference; anything here that
// is even measurable against that is over-retention, and the number worth knowing is whether a
// long session PLATEAUS rather than whether a single fold is quick.

// benchSession is a five-minute session at the observation cadence: 150 slots alternating
// between a four-row menu and a sparse screen, which is the worst realistic case for the
// segmenter because it re-enters both states repeatedly.
func benchSession(n int) []observe.ShadowSample {
	out := make([]observe.ShadowSample, 0, n)
	for i := 0; i < n; i++ {
		if i%3 == 0 {
			out = append(out, gameplay())
			continue
		}
		out = append(out, menu())
	}
	return out
}

func BenchmarkSessionWithStateSegmentation(b *testing.B) {
	samples := benchSession(150)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var totals observe.ShadowTotals
		for _, s := range samples {
			totals.Add(s)
		}
	}
}

// A five-minute session must plateau: the state model, the track table and the derived groups
// all stay bounded, so a longer session costs proportionally more time and no more memory.
func TestALongSessionPlateaus(t *testing.T) {
	short := fold(benchSession(30)...)
	long := fold(benchSession(600)...)

	if len(long.States) != len(short.States) {
		t.Errorf("states grew from %d to %d over a 20× longer session",
			len(short.States), len(long.States))
	}
	if len(long.Tracks) != len(short.Tracks) {
		t.Errorf("tracks grew from %d to %d", len(short.Tracks), len(long.Tracks))
	}
	if len(long.Transitions) != len(short.Transitions) {
		t.Errorf("transitions grew from %d to %d", len(short.Transitions),
			len(long.Transitions))
	}
	if len(long.Groups) != len(short.Groups) {
		t.Errorf("groups grew from %d to %d", len(short.Groups), len(long.Groups))
	}
	for _, tr := range long.Tracks {
		if len(tr.States) > observe.MaxTrackStates {
			t.Fatalf("track %s carries %d associations", tr.ID, len(tr.States))
		}
	}
}
