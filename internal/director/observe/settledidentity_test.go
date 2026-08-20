package observe_test

import (
	"reflect"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A durable place signature describes what the settled screen IS, not the average of how it
// happened to render while somebody was watching.
//
// # The live defect these reconstruct
//
// Windows Settings home reported a byte-identical accessibility tree across eight consecutive
// settled snapshots — `button=21`, fourteen roles, no variance at all — and still minted THREE
// durable subjects across three learn passes:
//
//	button=15                 button=17 + scroll_bar    button=19 + scroll_bar
//
// The composition was a running mean over every observation placed in the state, so it averaged in
// the partial renders each session caught on the way to the screen. Identity described how long
// somebody looked. Every downstream failure followed from it: the start established in one pass
// was not the screen the next pass saw (`left_the_start`), and the capture never confirmed the user
// was standing on the start (`0 events, 0 checkpoints`).
//
// These drive the REAL segmenter, because the defect was in the producer and a test that computed
// a composition itself would prove only that arithmetic works.

// panel builds one frame: n buttons in a column, plus the fixed furniture every Settings page has.
func panel(n int, extras ...observe.ShadowRegion) []observe.ShadowRegion {
	out := column(n, 0.40, 0.10)
	// Furniture that never varies, so the frames are recognisably one surface.
	for i, role := range []string{"list", "pane", "text_field", "combo_box"} {
		out = append(out, observe.ShadowRegion{
			Role: role, Nameable: true, Confidence: 0.6,
			Region: observe.Region{X: 0.02, Y: 0.02 + float64(i)*0.06, Width: 0.2, Height: 0.05},
		})
	}
	return append(out, extras...)
}

func scrollBar() observe.ShadowRegion {
	return observe.ShadowRegion{
		Role: "scroll_bar", Nameable: false, Confidence: 0.6,
		Region: observe.Region{X: 0.96, Y: 0.10, Width: 0.02, Height: 0.80},
	}
}

// arrival is a session: some partial renders, then the settled screen.
func arrival(partials []int, settled []observe.ShadowRegion, held int) [][]observe.ShadowRegion {
	var frames [][]observe.ShadowRegion
	for _, n := range partials {
		frames = append(frames, panel(n))
	}
	for range held {
		frames = append(frames, settled)
	}
	return frames
}

// settledRoles returns the role composition of the state that held the most observations.
func settledRoles(t *testing.T, frames [][]observe.ShadowRegion) map[string]int {
	t.Helper()
	k := segmentOver(frames)
	states := k.States()
	if len(states) == 0 {
		t.Fatal("the session produced no screen state at all")
	}
	// States() is sorted most-observed first.
	return states[0].Roles
}

// TestTheSameScreenIsTheSamePlaceHoweverLongYouLookAtIt is the regression.
//
// Restoring the rounded mean must fail it.
func TestTheSameScreenIsTheSamePlaceHoweverLongYouLookAtIt(t *testing.T) {
	full := panel(21)

	short := settledRoles(t, arrival([]int{12, 16, 19}, full, 5))
	long := settledRoles(t, arrival([]int{12, 16, 19}, full, 14))
	if !reflect.DeepEqual(short, long) {
		t.Errorf("a short visit and a long one describe different places:\n  short %v\n  long  %v\n"+
			"Identity is measuring how long somebody looked at the screen rather than what the "+
			"screen is.", short, long)
	}
	if got := short["button"]; got != 21 {
		t.Errorf("the settled composition reports %d button(s); the screen has 21 and the rest "+
			"was it arriving", got)
	}
}

// A different amount of RENDERING on the way in is not a different place either.
func TestHowMuchOfTheRenderYouCaughtIsNotPartOfThePlace(t *testing.T) {
	full := panel(21)

	caught := settledRoles(t, arrival([]int{9, 12, 14, 16, 18, 19, 20}, full, 8))
	missed := settledRoles(t, arrival([]int{19}, full, 8))
	if !reflect.DeepEqual(caught, missed) {
		t.Errorf("catching more of the render produced a different place:\n  caught %v\n  "+
			"missed %v", caught, missed)
	}
}

// A scroll bar that is there when the screen is settled STAYS in the composition, even though it
// was absent while the page was drawing.
//
// This is the half of the live defect that `sameRoleSet` turned into an instant mismatch: a role
// present in fewer than half the observations rounded to zero and vanished from the role set
// entirely, which reads as a categorically different screen rather than a slightly different one.
func TestARoleThatIsThereWhenSettledSurvivesTheRenderIn(t *testing.T) {
	full := append(panel(21), scrollBar())
	got := settledRoles(t, arrival([]int{12, 16, 19}, full, 6))
	if got["scroll_bar"] != 1 {
		t.Errorf("the settled screen has a scroll bar and the composition says %d: %v.\n"+
			"A role averaged below one half disappears from the ROLE SET, and sameRoleSet "+
			"treats that as a different screen rather than a different count.",
			got["scroll_bar"], got)
	}
}

// A transient overlay does not permanently redefine the screen.
//
// The control that kills raw max. One frame with extra structure is a flyout, a tooltip or a
// redraw — not a new composition.
func TestAOneFrameOverlayDoesNotRedefineTheScreen(t *testing.T) {
	full := panel(21)
	clean := settledRoles(t, arrival([]int{12, 16}, full, 8))

	var frames [][]observe.ShadowRegion
	frames = append(frames, panel(12), panel(16))
	frames = append(frames, full, full, full, full)
	frames = append(frames, panel(25)) // the overlay, once
	frames = append(frames, full, full, full, full)
	withOverlay := settledRoles(t, frames)

	if withOverlay["button"] != clean["button"] {
		t.Errorf("one overlay frame changed the screen's identity from %d button(s) to %d.\n"+
			"Something that appears once and goes away is not what the place is made of.",
			clean["button"], withOverlay["button"])
	}
}

// A visit too short for anything to recur still describes the finished screen.
//
// Every render stage is seen exactly once, so every count is equally frequent and the mode is a
// tie. Of the tied values only the last is the screen; the rest are it arriving. Without the
// largest-count tie-break the answer is decided by map iteration order, which is to say it is
// decided differently on different runs — so this is worth running more than once.
func TestAVisitTooShortForAnythingToRecurStillDescribesTheScreen(t *testing.T) {
	full := panel(21)
	for i := range 12 {
		got := settledRoles(t, arrival([]int{12, 16, 19}, full, 1))
		if got["button"] != 21 {
			t.Fatalf("run %d: the composition reports %d button(s), want 21.\nEvery stage of "+
				"the render was seen exactly once, so the counts tie; the finished screen is "+
				"the largest of them and everything below it is the screen arriving.",
				i, got["button"])
		}
	}
}

// Something that is usually NOT there is not part of the place.
//
// The other half of recording absence. A toast notification, a tooltip or a flyout that shows up
// in a couple of frames out of many must not join the screen's composition — the mode over
// observations including the ones where it was absent is what says so, and skipping absences would
// make one sighting in forty read as permanent.
func TestSomethingSeenInAFewFramesIsNotPartOfTheScreen(t *testing.T) {
	full := panel(21)
	toast := observe.ShadowRegion{
		Role: "menu_item", Nameable: true, Confidence: 0.6,
		Region: observe.Region{X: 0.70, Y: 0.02, Width: 0.25, Height: 0.08},
	}

	var frames [][]observe.ShadowRegion
	frames = append(frames, panel(12), panel(16))
	for range 6 {
		frames = append(frames, full)
	}
	frames = append(frames, append(append([]observe.ShadowRegion{}, full...), toast))
	frames = append(frames, append(append([]observe.ShadowRegion{}, full...), toast))
	for range 6 {
		frames = append(frames, full)
	}

	got := settledRoles(t, frames)
	if n := got["menu_item"]; n != 0 {
		t.Errorf("a notification present in 2 of 16 observations is in the screen's "+
			"composition as %d: %v.\nIf absence is not recorded, anything ever seen once "+
			"becomes a permanent part of the place.", n, got)
	}
	if got["button"] != 21 {
		t.Errorf("button = %d, want 21; the toast disturbed the rest of the composition",
			got["button"])
	}
}

// The ORDER the observations arrived in is not a property of the place.
//
// The same frames, arranged as an arrival and as a departure. A composition that weighted recent
// observations more heavily would answer differently for the two, which would make identity depend
// on whether the session happened to end while the screen was redrawing.
func TestTheOrderTheFramesArrivedInIsNotPartOfThePlace(t *testing.T) {
	full := panel(21)

	// An EQUAL split, so frequency alone ties and only the tie-break decides. Any rule that
	// leans on recency answers differently for the two orders; the mode with a largest-count
	// tie-break answers the same, because neither position nor arrival time is evidence.
	var arriving [][]observe.ShadowRegion
	for range 4 {
		arriving = append(arriving, panel(18))
	}
	for range 4 {
		arriving = append(arriving, full)
	}

	var departing [][]observe.ShadowRegion
	for range 4 {
		departing = append(departing, full)
	}
	for range 4 {
		departing = append(departing, panel(18))
	}

	a, d := settledRoles(t, arriving), settledRoles(t, departing)
	if a["button"] != d["button"] {
		t.Errorf("the same frames in two orders describe two places: arriving %d button(s), "+
			"departing %d.\nA composition that leans on the most recent observations makes "+
			"identity depend on what the screen happened to be doing when the session ended.",
			a["button"], d["button"])
	}
	if a["button"] != 21 {
		t.Errorf("button = %d, want 21", a["button"])
	}
}

// A genuinely different page is still a different place. The fix must not merge everything.
func TestADifferentPageStillSeparates(t *testing.T) {
	home := settledRoles(t, arrival([]int{12, 16}, panel(21), 8))
	category := settledRoles(t, arrival([]int{6}, panel(10), 8))

	if reflect.DeepEqual(home, category) {
		t.Fatalf("a 21-button page and a 10-button page describe the same place: %v", home)
	}
	if home["button"] == category["button"] {
		t.Errorf("both pages report %d buttons", home["button"])
	}
}
