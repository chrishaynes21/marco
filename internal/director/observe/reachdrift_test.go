package observe_test

import (
	"fmt"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A WINDOW NOBODY TOUCHED STOPPED BEING RECOGNISED HALFWAY THROUGH LOOKING AT IT.
//
// # Measured, live
//
// One observation session over one Windows Settings page, at 900ms, with nothing on the desktop
// changing and the window never moved or resized:
//
//	14 samples    466 ever-seen    142 present    recognised as its durable Place
//	27 samples    817 ever-seen    142 present    recognised
//	40 samples   1024 ever-seen     88 present    UNREADABLE
//	183 samples  1024 ever-seen     88 present    UNREADABLE
//
// Eighty-seven structures sat inside the content region the whole time. What moved was the
// DENOMINATOR: `tracksInState` returned every structure ever seen in the state, so the occupancy
// ratio fell as the session accumulated tracks, crossed maxVacantOccupancy between the 27th
// sample and the 40th, and stayed across it — 1024 is the track table's cap, so it saturates and
// the verdict never comes back.
//
// The failure is reported as a fact about the PAGE: "accessibility described the window but not
// the content". That is the exact misrouting this file exists to prevent, and it disabled
// recognition for any look longer than about half a minute — Learn, reach, rehearsal, and the
// resize acceptance 35D had been waiting on, which could not be run at all while it was true.
//
// # Why the existing fixtures could not see it
//
// Every track `seenAt` builds is Present, so ever-seen and still-present are the same set in
// every other test in this package. The bug lives exactly in the gap between them.

// departed is a structure this state has seen and is no longer finding.
//
// It keeps its region, which is the harder case and the one that has to be gated: a structure
// with no geometry at all cannot be counted inside anything, so it is filtered by arithmetic
// whatever this rule says. The live population was mostly that kind — 936 of the 1024 carried no
// region — and a fixture built only from those would let the rule be deleted without a test
// noticing.
func departed(id string, x, y, w, h float64) observe.ShadowTrack {
	return observe.ShadowTrack{
		ID: id, Role: "unknown", Present: false,
		Reference: observe.Region{X: x, Y: y, Width: w, Height: h},
		States:    []observe.TrackState{{State: "state_1", Seen: 4, Eligible: 40}},
	}
}

// stillHere is a structure the reading is still finding.
func stillHere(id string, x, y, w, h float64) observe.ShadowTrack {
	return observe.ShadowTrack{
		ID: id, Role: "unknown", Present: true,
		Reference: observe.Region{X: x, Y: y, Width: w, Height: h},
		States:    []observe.TrackState{{State: "state_1", Seen: 38, Eligible: 40}},
	}
}

// aPageWithContent is a content area on the right with twenty things in it, a navigation strip
// down the left, and however much the session has watched come and go over there.
//
// The churn is deliberately OUTSIDE the content region. That is what dilution is: structures
// counted in the population of the reading while contributing nothing to the space being judged.
// Churn inside the content area would raise both halves of the ratio and hide the defect, which
// is how the first version of this test passed against the bug.
func aPageWithContent(churn int) observe.ShadowTotals {
	tracks := []observe.ShadowTrack{
		stillHere("content", 0.30, 0.06, 0.68, 0.92),
	}
	for i := 0; i < 20; i++ {
		tracks = append(tracks, stillHere(fmt.Sprintf("row-%d", i),
			0.34, 0.10+float64(i)*0.04, 0.55, 0.03))
	}
	for i := 0; i < churn; i++ {
		tracks = append(tracks, departed(fmt.Sprintf("left-%d", i),
			0.02, 0.10+float64(i%20)*0.04, 0.24, 0.03))
	}
	return observe.ShadowTotals{
		CurrentState: "state_1", Tracks: tracks,
		States: []observe.ScreenState{{ID: "state_1", Inferences: 40, Settled: true}},
	}
}

// The same page, judged the same way, however long it has been watched.
//
// Deleting the Present filter in tracksInState must fail this.
func TestAWindowDoesNotEmptyBecauseItWasWatchedForLonger(t *testing.T) {
	for _, churn := range []int{0, 200, 600, 1000} {
		reach, vac, reason := observe.ReachOfState(aPageWithContent(churn), "state_1")
		if reach != observe.ReachContent {
			t.Errorf("after %d structures had come and gone, one unchanged page reads as "+
				"%s (%s): %d of %d structures inside a region covering %.0f%% of the "+
				"window.\nNothing about the page moved. What moved is how long it had "+
				"been watched, and a judgement that changes with that is measuring the "+
				"session rather than the screen.",
				churn, reach, reason, vac.Inside, vac.Structures, vac.Share*100)
		}
	}
}

// And the reading it was written for still fails, whatever else the state remembers.
//
// The live case from the top of reach.go: the caption buttons and a title strip, plus one
// rectangle over the whole content area with nothing in it. Suspended UWP content. It must stay
// ReachShell — a filter that made every window look populated would delete the judgement rather
// than fix it, and it would delete it in the direction that matters, because a shell reading
// admitted as content is what mints a durable Place out of a frame.
func TestSuspendedContentIsStillAShellHoweverMuchHasBeenSeenBefore(t *testing.T) {
	for _, churn := range []int{0, 600} {
		tracks := []observe.ShadowTrack{
			stillHere("close", 0.972, 0.008, 0.024, 0.031),
			stillHere("maximise", 0.948, 0.008, 0.024, 0.031),
			stillHere("minimise", 0.924, 0.008, 0.024, 0.031),
			stillHere("titlestrip", 0.367, 0.015, 0.267, 0.031),
			stillHere("account", 0.780, 0.070, 0.190, 0.060),
			stillHere("back", 0.020, 0.015, 0.030, 0.031),
			// The content area, reported at full size with nothing inside it.
			stillHere("content", 0.100, 0.109, 0.870, 0.860),
		}
		// A long look at this same broken reading remembers plenty. It changes nothing:
		// the structures that ARE there are still all around the edge.
		for i := 0; i < churn; i++ {
			tracks = append(tracks, departed(fmt.Sprintf("gone-%d", i), 0.30, 0.30, 0.10, 0.05))
		}
		totals := observe.ShadowTotals{
			CurrentState: "state_1", Tracks: tracks,
			States: []observe.ScreenState{{ID: "state_1", Inferences: 40, Settled: true}},
		}
		reach, vac, _ := observe.ReachOfState(totals, "state_1")
		if reach != observe.ReachShell {
			t.Errorf("with %d structures remembered, suspended content reads as %s.\n"+
				"This is the reading the whole file was written for: %d of %d inside a "+
				"region covering %.0f%% of the window.",
				churn, reach, vac.Inside, vac.Structures, vac.Share*100)
		}
	}
}
