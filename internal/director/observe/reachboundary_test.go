package observe_test

import (
	"math/rand"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// THE CASES THE FIRST SET OF FIXTURES COULD NOT SEE.
//
// # Why these exist
//
// A mutation run over [observe.ReachOfState] and [observe.SufficiencyOf] killed nine attacks
// and five survived. None of the five was an equivalent mutation. Every one was a hole in the
// fixtures:
//
//	"more than 20 structures is healthy"   every degraded fixture was small
//	"occupancy is a count, not a ratio"    no fixture made the two disagree
//	"take the first empty region"          no fixture had two to choose between
//	"minVacantShare 0.40 becomes 0.20"     nothing sat in the gap between them
//	"every sufficient reading says content_reached"  the sufficiency tests only asserted
//	                                       the two reasons the headline cases produce
//
// The rule was right in each case and nothing proved it. These fixtures are built to sit on
// the far side of exactly one boundary each, so that moving that boundary breaks something.

// crowdedShell is a degraded reading that is not small.
//
// The live Settings failure had sixteen structures, and every fixture built from it inherited
// that. So a rule reading "more than twenty structures means the page arrived" passed
// everything — while being wrong about the case that matters most, an application whose
// navigation is accessible and whose content pane is not.
//
// Fifty structures: a rail of forty items down the side, chrome across the top, and a content
// region covering two thirds of the window with FOUR things in it. Four is the second boundary.
// As a ratio that is 8% of the reading and the region is empty; as a bare count it is more than
// a handful and a rule counting heads would call the page populated. A partially-painted page
// leaves exactly this — the host arrived, a few placeholders arrived, the page did not.
func crowdedShell() observe.ShadowTotals {
	tracks := []observe.ShadowTrack{
		seenAt("close", 0.97, 0.01, 0.02, 0.03),
		seenAt("maximise", 0.94, 0.01, 0.02, 0.03),
		seenAt("minimise", 0.91, 0.01, 0.02, 0.03),
		seenAt("title", 0.05, 0.01, 0.30, 0.03),
		seenAt("search", 0.40, 0.01, 0.20, 0.03),
		// AND THE PAGE THAT NEVER PAINTED.
		seenAt("content", 0.28, 0.08, 0.70, 0.90),
	}
	// A navigation rail, fully accessible, entirely outside the content area.
	for i := 0; i < 40; i++ {
		tracks = append(tracks, seenAt("nav", 0.02, 0.08+float64(i)*0.022, 0.22, 0.02))
	}
	// Four stragglers inside the content region: 4 of 50 is 8%, under the ratio, over any
	// small count.
	for i := 0; i < 4; i++ {
		tracks = append(tracks, seenAt("skeleton", 0.32, 0.12+float64(i)*0.03, 0.60, 0.02))
	}
	return totalsOf("state_1", tracks...)
}

// A degraded reading is degraded however much furniture surrounds it.
//
// Kills "more than N structures is healthy" and "occupancy is a count rather than a ratio",
// which the small fixtures could not distinguish.
func TestARichlyFurnishedWindowCanStillHaveNoPage(t *testing.T) {
	got, v, reason := observe.ReachOfState(crowdedShell(), "state_1")
	if got != observe.ReachShell {
		t.Fatalf("reach = %q, want %q\n"+
			"Fifty structures were observed and forty-five of them are the frame. A rule "+
			"that reads richness as health calls this page read.\n(vacancy %+v)",
			got, observe.ReachShell, v)
	}
	if reason != observe.ReasonClientAreaUnpopulated {
		t.Errorf("reason = %q, want %q", reason, observe.ReasonClientAreaUnpopulated)
	}
	if v.Inside != 4 {
		t.Errorf("the vacancy holds %d structures, want 4 — the fixture no longer sits on "+
			"the ratio/count boundary it was built for", v.Inside)
	}
	if v.Structures < 40 {
		t.Errorf("the reading has %d structures; this fixture proves nothing unless it is "+
			"comfortably larger than any count somebody might reach for", v.Structures)
	}
}

// photoGrid is healthy, and its empty region sits between the two thresholds.
//
// A grid of thumbnails with an empty preview strip beside them. The strip is 30% of the
// window: under [minVacantShare] at 0.40 and over it at 0.20, so lowering that bound turns a
// perfectly ordinary window into a broken one. Nothing else in the fixture is a container, so
// the populated-panel rule cannot rescue it — the share is doing all the work, which is the
// point.
func photoGrid() observe.ShadowTotals {
	tracks := []observe.ShadowTrack{
		seenAt("close", 0.97, 0.01, 0.02, 0.03),
		seenAt("title", 0.05, 0.01, 0.30, 0.03),
		// The empty preview strip: 0.30 of the window.
		seenAt("preview", 0.68, 0.10, 0.30, 1.00),
	}
	// Sixty thumbnails, none of which contains another.
	for row := 0; row < 10; row++ {
		for col := 0; col < 6; col++ {
			tracks = append(tracks, seenAt("thumb",
				0.02+float64(col)*0.10, 0.10+float64(row)*0.085, 0.09, 0.075))
		}
	}
	return totalsOf("state_1", tracks...)
}

// An ordinary empty panel is not a missing page.
//
// Kills "minVacantShare 0.40 becomes 0.20". A third of a window being empty is a layout, not a
// failure; the live failure had three quarters of the window empty and nowhere else populated.
func TestAThirdOfAWindowBeingEmptyIsALayout(t *testing.T) {
	got, v, _ := observe.ReachOfState(photoGrid(), "state_1")
	if got != observe.ReachContent {
		t.Errorf("reach = %q, want %q\n"+
			"A grid of sixty thumbnails beside an empty preview strip was called a "+
			"window with no page in it. The strip is 30%% of the frame — a layout "+
			"choice, and the reason the emptiness bound is where it is.\n(vacancy %+v)",
			got, observe.ReachContent, v)
	}
}

// twoVacancies is a window with two large empty regions of different sizes.
//
// Which one is reported is the whole of the evidence a caller gets, and until this fixture
// existed there was never more than one candidate — so taking the first one iteration happened
// to reach was indistinguishable from taking the largest. Map order would then decide what a
// person is told about their own screen.
func twoVacancies() observe.ShadowTotals {
	tracks := []observe.ShadowTrack{
		seenAt("close", 0.97, 0.01, 0.02, 0.03),
		seenAt("title", 0.05, 0.01, 0.30, 0.03),
		seenAt("minimise", 0.91, 0.01, 0.02, 0.03),
		seenAt("maximise", 0.94, 0.01, 0.02, 0.03),
		seenAt("appicon", 0.01, 0.01, 0.02, 0.03),
		// The smaller empty region, listed FIRST on purpose.
		seenAt("side", 0.00, 0.06, 0.45, 0.94),
		// And the larger one.
		seenAt("main", 0.46, 0.06, 0.54, 0.94),
	}
	return totalsOf("state_1", tracks...)
}

// The vacancy reported is the largest one, not the first one seen.
func TestTheEvidenceIsTheLargestEmptySpace(t *testing.T) {
	base := twoVacancies()
	got, v, _ := observe.ReachOfState(base, "state_1")
	if got != observe.ReachShell {
		t.Fatalf("reach = %q, want %q (vacancy %+v)", got, observe.ReachShell, v)
	}
	// 0.54 x 0.94 is larger than 0.45 x 0.94.
	if v.Region.Width < 0.5 {
		t.Errorf("the vacancy reported is %.2f wide; the larger empty region is 0.54.\n"+
			"Reporting whichever came first makes what a person is told about their "+
			"screen depend on map order.", v.Region.Width)
	}

	// And it stays the largest however the tracks are ordered.
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 16; i++ {
		shuffled := observe.ShadowTotals{
			CurrentState: base.CurrentState, States: base.States,
			Tracks: append([]observe.ShadowTrack(nil), base.Tracks...),
		}
		rng.Shuffle(len(shuffled.Tracks), func(a, b int) {
			shuffled.Tracks[a], shuffled.Tracks[b] = shuffled.Tracks[b], shuffled.Tracks[a]
		})
		gotR, gotV, gotReason := observe.ReachOfState(shuffled, "state_1")
		if gotR != got || gotV != v || gotReason != observe.ReasonClientAreaUnpopulated {
			t.Fatalf("shuffling two candidate vacancies changed the answer\n"+
				"before %q %+v\nafter  %q %+v", got, v, gotR, gotV)
		}
	}
}

// Every route to `sufficient` keeps its own name once it becomes a Sufficiency.
//
// Kills "every sufficient reading says content_reached". Three findings resolve to sufficient
// and only one of them is a statement about the page: the page was read, somewhere else in the
// window was populated, or there was too little observed to judge at all. The last is a
// REFUSAL to judge, and a later escalation policy that cannot tell it from a healthy page will
// treat "I did not look hard enough" as "there is nothing more to see".
func TestSufficientKeepsWhyItWasSufficient(t *testing.T) {
	for _, c := range []struct {
		name  string
		given observe.ShadowTotals
		want  observe.SufficiencyReason
	}{
		{"the page was read", page(), observe.ReasonContentReached},
		{"a panel elsewhere was populated", emptyPanel(), observe.ReasonPopulatedPanel},
		{"too little was observed to judge", totalsOf("state_1",
			seenAt("window", 0, 0, 1, 1), seenAt("label", 0.1, 0.1, 0.2, 0.05)),
			observe.ReasonTooLittleToJudge},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := observe.PlaceNow(c.given, "any-app", &countingRecogniser{},
				observe.DefaultHypothesisThresholds())
			s := observe.SufficiencyOf(p)
			if s.State != observe.Sufficient {
				t.Fatalf("state = %q, want %q", s.State, observe.Sufficient)
			}
			if s.Reason != c.want {
				t.Errorf("reason = %q, want %q\n"+
					"The verdict survived and the route to it did not. A policy "+
					"deciding whether to look again cannot tell a page that was read "+
					"from one nobody looked at properly.", s.Reason, c.want)
			}
		})
	}
}

// The optional sensors are in scope and are not consulted.
//
// # Why this needs a test rather than a glance at the signature
//
// [observe.ShadowTotals] carries `Detector` and `Unavailable` — which detector was running and
// why it was not. So the classifier COULD reach for whether a visual model was available, and
// the argument that it does not is a claim about the code rather than a property of its type.
//
// It must not, in either direction. A machine with no vision plugin installed does not have a
// worse accessibility reading; a machine with one does not have a better one. 37C measured
// that directly — ScreenParser added no unique semantic value to a healthy desktop reading —
// and if sensor availability could move this answer, the same window would be sufficient on
// one machine and incomplete on another.
//
// Deleting nothing in particular fails this. It is here so that ADDING a consultation does.
func TestSensorAvailabilityDoesNotMoveTheAnswer(t *testing.T) {
	for _, given := range []struct {
		name  string
		build func() observe.ShadowTotals
	}{
		{"a page that was read", page},
		{"a window with no page in it", shell},
		{"a richly furnished window with no page", crowdedShell},
	} {
		t.Run(given.name, func(t *testing.T) {
			base := given.build()
			wantReach, wantV, wantReason := observe.ReachOfState(base, "state_1")

			for _, sensors := range []struct {
				detector, unavailable string
			}{
				{"screenparser", ""},
				{"", "no vision plugin found"},
				{"classical-cv", ""},
				{"", "the ONNX runtime could not be loaded"},
			} {
				got := given.build()
				got.Detector = sensors.detector
				got.Unavailable = sensors.unavailable
				r, v, reason := observe.ReachOfState(got, "state_1")
				if r != wantReach || v != wantV || reason != wantReason {
					t.Errorf("detector=%q unavailable=%q changed the answer from "+
						"%q/%q to %q/%q.\nWhether a visual model is installed is not "+
						"evidence about whether accessibility read the page.",
						sensors.detector, sensors.unavailable,
						wantReach, wantReason, r, reason)
				}
			}
		})
	}
}
