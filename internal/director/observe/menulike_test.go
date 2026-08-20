package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The admission predicate, and the two ways it could quietly become something else.

// THE cross-game regression, built from compositions both recorded sessions actually produced.
//
// The point is not that these numbers are right in the abstract — it is that ONE rule separates
// choices from play in two games that lay their interfaces out completely differently, without
// knowing which game it is looking at. A predicate that needed different treatment per title
// would be a capability pack with extra steps.
func TestTheAdmissionPredicateSeparatesChoicesFromPlayInBothRecordedGames(t *testing.T) {
	cases := []struct {
		name    string
		regions []observe.ShadowRegion
		want    bool
	}{
		// Recorded in Experiment-008 (Schedule I).
		{"schedule-i gameplay, one hud control", []observe.ShadowRegion{
			det("button", 0.72, 0.92, 0.02, 0.02),
		}, false},
		{"schedule-i screen, five controls", []observe.ShadowRegion{
			det("button", 0.44, 0.05, 0.04, 0.03), det("button", 0.02, 0.80, 0.02, 0.03),
			det("button", 0.02, 0.84, 0.02, 0.03), det("button", 0.10, 0.60, 0.05, 0.03),
			det("button", 0.30, 0.30, 0.05, 0.03),
			det("icon", 0.90, 0.10, 0.04, 0.04), det("icon", 0.95, 0.10, 0.04, 0.04),
		}, true},
		// Recorded in Experiment-007 (Rocket League) — a completely different layout.
		{"rocket-league pause column", menuRegions(), true},
		{"rocket-league gameplay hud", []observe.ShadowRegion{
			det("icon", 0.02, 0.86, 0.19, 0.10),
		}, false},
		// The boundary itself.
		{"two controls is not a set of choices", []observe.ShadowRegion{
			det("button", 0.4, 0.4, 0.1, 0.05), det("button", 0.4, 0.5, 0.1, 0.05),
		}, false},
		{"nothing on screen", nil, false},
		// Icons are not choices, however many there are: their role is not one whose name
		// may be said, which is exactly the distinction the allowlist draws.
		{"a wall of icons is not a set of choices", []observe.ShadowRegion{
			det("icon", 0.1, 0.1, 0.05, 0.05), det("icon", 0.2, 0.1, 0.05, 0.05),
			det("icon", 0.3, 0.1, 0.05, 0.05), det("icon", 0.4, 0.1, 0.05, 0.05),
			det("icon", 0.5, 0.1, 0.05, 0.05), det("icon", 0.6, 0.1, 0.05, 0.05),
		}, false},
	}

	for _, c := range cases {
		if got := observe.MenuLike(c.regions); got != c.want {
			t.Errorf("%s: MenuLike = %v, want %v (%d nameable controls)",
				c.name, got, c.want, observe.NameableControls(c.regions))
		}
	}
}

// The predicate must NOT be a spacing rule, and this is the measurement that forbids it.
//
// Rocket League's pause column measures uniformity 0.97; every recurring group in Schedule I
// measures 0.00–0.01. Both are real interfaces full of real choices. A predicate keyed on even
// spacing would admit navigation in one game and refuse it in the other — a Rocket League rule
// wearing the costume of a screen-shape rule, and invisible until the third game.
//
// So: scattered controls and stacked controls must decide the same way.
func TestTheAdmissionPredicateIgnoresArrangement(t *testing.T) {
	stacked := []observe.ShadowRegion{
		det("button", 0.414, 0.437, 0.172, 0.036),
		det("button", 0.414, 0.480, 0.172, 0.036),
		det("button", 0.414, 0.520, 0.172, 0.036),
	}
	scattered := []observe.ShadowRegion{
		det("button", 0.02, 0.80, 0.02, 0.03),
		det("button", 0.44, 0.05, 0.04, 0.03),
		det("button", 0.91, 0.62, 0.07, 0.02),
	}
	if !observe.MenuLike(stacked) {
		t.Error("an evenly stacked column of three controls was not admitted")
	}
	if !observe.MenuLike(scattered) {
		t.Error("three controls that are not evenly spaced were refused. The predicate has " +
			"become a spacing rule, which measured 0.97 in one game and 0.00 in another and " +
			"is therefore a rule about Rocket League rather than about screens")
	}
}

// THE non-circularity guard, and the reason it is worth a test rather than a comment.
//
// Admission decides what input evidence exists; screen states and tracks are built from
// detections. If the predicate ever read anything the tracker had concluded, the two would
// define each other and the model would be free to agree with itself.
//
// The signature already forbids it — MenuLike takes regions and nothing else — so this asserts
// the property that would break first if someone widened it: the same detections must decide the
// same way regardless of what has been observed before them.
func TestAdmissionContextDoesNotDependOnTracking(t *testing.T) {
	regions := menuRegions()
	before := observe.MenuLike(regions)

	// A long, contradictory history: many inferences of a completely different screen, folded
	// through the production accumulator so tracks, states and groups all exist.
	var totals observe.ShadowTotals
	for i := 0; i < 30; i++ {
		totals.Add(gameplay())
	}
	if len(totals.States) == 0 {
		t.Fatal("the fixture built no state history, so this proves nothing")
	}

	if after := observe.MenuLike(regions); after != before {
		t.Errorf("MenuLike returned %v before a session's history existed and %v after. "+
			"Admission has become downstream of tracking, and the tracker consumes admitted "+
			"input — the loop is closed", before, after)
	}
}
