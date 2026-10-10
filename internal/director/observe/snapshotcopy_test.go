package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A PUBLISHED SCREEN STATE IS A SNAPSHOT, NOT A VIEW.
//
// Found by `go test -race`: a session's fold writes a state's PlaceNames and Affordances in
// `creditTerms` while `ambientLook` reads the published snapshot on another goroutine, through
// `TargetsToRecordExplained`. A map read concurrently with a write is not a stale read; it is
// undefined behaviour, and that is the path a grounded highlight takes while a session runs.
//
// `States()` already said the rule out loud — "the tallies are copied rather than shared…
// aliasing the live maps would let a caller's read change while it reads" — and two of the four
// maps were aliased anyway. A race test cannot be deterministic; this one is, because aliasing
// and copying differ in a way that does not need two goroutines to see.
func TestAPublishedScreenStateDoesNotChangeUnderneathItsReader(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	named := func(name string) observe.Sample {
		seq++
		s := menuSample(seq, 6, 0)
		sh := *s.Shadow
		sh.Semantic = observe.SemanticEvidence{
			Observed: true, PlaceName: name,
			Affordances: []observe.ObservedAffordance{{Label: name, Kind: "button"}},
		}
		s.Shadow = &sh
		return s
	}
	for range 4 {
		totals.Observe(named("Settings"))
	}
	before := totals.States
	if len(before) == 0 {
		t.Fatal("no screen states")
	}
	var watched observe.ScreenState
	for _, st := range before {
		if len(st.PlaceNames) > 0 {
			watched = st
			break
		}
	}
	if len(watched.PlaceNames) == 0 {
		t.Fatalf("no state carries a name, so there is nothing to alias: %+v", before)
	}
	nameCount := watched.PlaceNames["Settings"]
	affCount := len(watched.Affordances)

	// The session keeps going. Whatever a reader is holding must not move.
	for range 6 {
		totals.Observe(named("Settings"))
	}

	if got := watched.PlaceNames["Settings"]; got != nameCount {
		t.Errorf("a snapshot's PlaceNames changed from %d to %d while its session ran. "+
			"The map is aliased, and a reader on another goroutine is reading a map "+
			"somebody else is writing", nameCount, got)
	}
	if got := len(watched.Affordances); got != affCount {
		t.Errorf("a snapshot's Affordances changed from %d to %d entries while its "+
			"session ran", affCount, got)
	}
}
