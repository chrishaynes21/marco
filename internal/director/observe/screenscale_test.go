package observe_test

import (
	"fmt"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Does the segmenter still discriminate at the scale a real accessibility tree produces?
//
// The thresholds were measured against a live Rocket League trace, where a detector reported
// a handful of boxes per frame. An accessibility tree reports HUNDREDS: VS Code produced 128
// stable structures and Chrome 52 in the live validation of this milestone.
//
// That is a genuine risk to check rather than assume. Weighted Jaccard over count vectors
// gets more forgiving as the counts grow — two busy screens can share a great deal simply by
// both being busy — so a threshold that separated a four-row menu from a bare HUD might merge
// two rich screens into one.
//
// Every live session in the validation produced exactly ONE screen, which is consistent with
// "nobody interacted" and equally consistent with "everything matches now". These tests are
// what tell the two apart.

// richScreen builds a composition of n controls spread over a region of the window.
func richScreen(n int, role string, x0, y0, w, h float64) []observe.ShadowRegion {
	out := make([]observe.ShadowRegion, 0, n)
	for i := 0; i < n; i++ {
		// A deterministic spread across the region, so the arrangement is real rather
		// than every control landing in one cell.
		fx := float64(i%8) / 8
		fy := float64((i/8)%8) / 8
		out = append(out, observe.ShadowRegion{
			Role: role,
			Region: observe.Region{
				X: x0 + fx*w, Y: y0 + fy*h, Width: 0.02, Height: 0.015,
			},
		})
	}
	return out
}

// Two rich screens that differ in COMPOSITION must not merge.
//
// An editor full of list items and a settings page full of checkboxes have different role
// histograms. If 128 elements each is enough to make them look alike, screen identity has
// stopped meaning anything on exactly the applications this milestone unblocked.
func TestTwoRichScreensWithDifferentCompositionsStaySeparate(t *testing.T) {
	var g observe.ScreenSegmenter

	editor := richScreen(128, "list_item", 0.0, 0.1, 0.6, 0.8)
	settings := richScreen(128, "checkbox", 0.3, 0.1, 0.6, 0.8)

	a := g.Observe(1, editor, nil, observe.SemanticEvidence{})
	g.Observe(2, settings, nil, observe.SemanticEvidence{})
	b := g.Observe(3, settings, nil, observe.SemanticEvidence{})

	if a == b {
		t.Fatalf("128 list items and 128 checkboxes were read as the same screen (%s).\n"+
			"At accessibility scale the similarity threshold no longer discriminates, and "+
			"every application would be one screen forever", a)
	}
	if b == observe.ScreenStateUnknown {
		t.Fatalf("a screen seen twice never got an identity")
	}
}

// Two rich screens that differ in ARRANGEMENT must not merge either.
//
// The harder case, and the one a role histogram alone cannot answer: the same controls, laid
// out somewhere else. A sidebar of list items and a full-width list of the same items are
// different screens, and the coarse 3x3 grid is what has to notice.
func TestTwoRichScreensWithDifferentArrangementsStaySeparate(t *testing.T) {
	var g observe.ScreenSegmenter

	sidebar := richScreen(96, "list_item", 0.0, 0.1, 0.2, 0.8)
	fullWidth := richScreen(96, "list_item", 0.0, 0.1, 0.95, 0.8)

	a := g.Observe(1, sidebar, nil, observe.SemanticEvidence{})
	g.Observe(2, fullWidth, nil, observe.SemanticEvidence{})
	b := g.Observe(3, fullWidth, nil, observe.SemanticEvidence{})

	if a == b {
		t.Errorf("the same controls in a narrow sidebar and across the whole window were "+
			"read as one screen (%s); arrangement is not discriminating at this scale", a)
	}
}

// And the same rich screen, jittering the way a live tree does, stays ONE screen.
//
// The control for both tests above. A real accessibility tree gains and loses a few nodes
// every sample — a tooltip, a scrollbar thumb, a status message — and a segmenter that minted
// a state for each would be useless in a different direction.
func TestARichScreenSurvivesTheChurnALiveTreeProduces(t *testing.T) {
	var g observe.ScreenSegmenter

	base := richScreen(128, "list_item", 0.0, 0.1, 0.6, 0.8)
	first := g.Observe(1, base, nil, observe.SemanticEvidence{})

	for i := 2; i <= 12; i++ {
		churned := richScreen(128+(i%7)-3, "list_item", 0.0, 0.1, 0.6, 0.8)
		churned = append(churned, observe.ShadowRegion{
			Role: "text", Region: observe.Region{X: 0.5, Y: 0.95, Width: 0.1, Height: 0.02},
		})
		if got := g.Observe(i, churned, nil, observe.SemanticEvidence{}); got != first {
			t.Fatalf("sample %d: a live tree gaining and losing a few nodes became a new "+
				"screen (%s then %s). Nothing would ever recur", i, first, got)
		}
	}
	if n := len(g.States()); n != 1 {
		t.Errorf("twelve samples of one screen produced %d states", n)
	}
}

// How far apart two screens have to be before the segmenter separates them, at scale.
//
// Not an assertion about a number — a MEASUREMENT, printed, so the next person tuning
// thresholds has the figure this milestone actually observed rather than the one from a
// four-box detector trace. It fails only if the answer is degenerate in either direction.
func TestTheDiscriminationBoundaryAtAccessibilityScale(t *testing.T) {
	var merged, separated int
	for shared := 0; shared <= 128; shared += 16 {
		var g observe.ScreenSegmenter
		a := richScreen(128, "list_item", 0.0, 0.1, 0.6, 0.8)
		// b keeps `shared` of a's controls and replaces the rest with a different role.
		b := append([]observe.ShadowRegion{}, a[:shared]...)
		b = append(b, richScreen(128-shared, "checkbox", 0.3, 0.1, 0.6, 0.8)...)

		first := g.Observe(1, a, nil, observe.SemanticEvidence{})
		g.Observe(2, b, nil, observe.SemanticEvidence{})
		second := g.Observe(3, b, nil, observe.SemanticEvidence{})

		same := first == second
		if same {
			merged++
		} else {
			separated++
		}
		t.Log(fmt.Sprintf("  %3d/128 controls in common -> %s", shared,
			map[bool]string{true: "ONE screen", false: "two screens"}[same]))
	}
	if separated == 0 {
		t.Fatal("no amount of difference separated two screens at accessibility scale")
	}
	if merged == 0 {
		t.Fatal("no amount of similarity merged two screens at accessibility scale; " +
			"every sample would mint a state")
	}
}
