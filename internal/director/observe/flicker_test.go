package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A SCREEN THAT CHANGED FOR ONE FRAME DID NOT CHANGE.
//
// # The measurement
//
// 55 inferences over four minutes of a live game produced FORTY transitions, with the
// degenerate single-pane reading re-entered nineteen times. Most of those pairs are one
// inference in which a 0.17-confidence detection dropped out and came back. Nothing about that
// is a screen change, and everything downstream — presence ratios, episodes, and any future
// learning layer — was being handed it as one.
//
// # What is NOT being done about it
//
// The confidence floor is not being raised. The detections sit at 0.16–0.19 against a 0.15
// threshold and the menu entries are among them, so a floor high enough to remove the
// fluctuation removes the menu with it. The fluctuation is not low confidence, it is low
// PERSISTENCE.

// twoCompositions builds two compositions that are different places on the window.
func twoCompositions() (a, b []observe.ShadowRegion) {
	rows := func(y0 float64) []observe.ShadowRegion {
		var out []observe.ShadowRegion
		for i := range 4 {
			out = append(out, observe.ShadowRegion{
				Role: "button",
				Region: observe.Region{
					X: 0.4, Y: y0 + float64(i)*0.042, Width: 0.17, Height: 0.036,
				},
			})
		}
		return out
	}
	return rows(0.10), rows(0.60)
}

// walkScreens drives a segmenter through a sequence of compositions and returns the ids.
func walkScreens(g *observe.ScreenSegmenter, seq ...[]observe.ShadowRegion) []observe.ScreenStateID {
	out := make([]observe.ScreenStateID, 0, len(seq))
	for i, r := range seq {
		out = append(out, g.Observe(i+1, r, nil, observe.SemanticEvidence{}, observe.SelectionAt{}))
	}
	return out
}

func TestOneFlickeringInferenceIsNotAScreenChange(t *testing.T) {
	a, b := twoCompositions()
	var g observe.ScreenSegmenter

	// Settled on A, one frame of B, settled on A again. The screen did not change.
	walkScreens(&g, a, a, a, b, a, a, a)

	if n := len(g.Transitions()); n != 0 {
		t.Errorf("%d transition(s) from one frame of a different composition: %+v\n"+
			"Live, this shape produced 40 transitions in 55 inferences.",
			n, g.Transitions())
	}
	if n := len(g.Crossings()); n != 0 {
		t.Errorf("%d crossing(s) recorded: %+v", n, g.Crossings())
	}
	if g.Flicker.Rejected != 1 {
		t.Errorf("the gate refused %d proposals, want 1: %+v", g.Flicker.Rejected,
			g.Flicker)
	}
	if g.Flicker.Confirmed != 0 {
		t.Errorf("the gate confirmed %d changes, want 0", g.Flicker.Confirmed)
	}
}

// And a screen that really changed still changes. The pair is the whole claim: a gate that
// refused everything would pass the test above and be worthless.
func TestAScreenThatStaysChangedIsAChange(t *testing.T) {
	a, b := twoCompositions()
	var g observe.ScreenSegmenter

	ids := walkScreens(&g, a, a, b, b, b)

	if ids[0] == ids[4] {
		t.Fatalf("two different compositions were read as one screen (%s)", ids[0])
	}
	trs := g.Transitions()
	if len(trs) != 1 || trs[0].Count != 1 {
		t.Fatalf("a settled change produced %+v, want exactly one transition", trs)
	}
	if trs[0].From != ids[0] || trs[0].To != ids[4] {
		t.Errorf("the transition is %s → %s, want %s → %s",
			trs[0].From, trs[0].To, ids[0], ids[4])
	}
	if g.Flicker.Confirmed != 1 || g.Flicker.Rejected != 0 {
		t.Errorf("flicker stats %+v, want one confirmed and none rejected", g.Flicker)
	}
}

// AND THE CHANGE IS DATED FROM WHEN IT HAPPENED, not from when it was believed.
//
// The gate costs one inference of latency by construction. What it must not cost is the
// attribution: the press that caused a change lands before the frame that shows it, and a gate
// that read the confirming look's navigation would blame every change on the NEXT press.
func TestAConfirmedChangeKeepsTheNavigationThatPrecededIt(t *testing.T) {
	a, b := twoCompositions()
	var g observe.ScreenSegmenter

	g.Observe(1, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(2, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	// The press, on the last look of the old screen.
	g.Observe(3, a, []observe.InputEvent{{Intent: observe.NavConfirm, AtMS: 300}},
		observe.SemanticEvidence{}, observe.SelectionAt{})
	// The new screen appears, then holds. Nothing is pressed on either look.
	g.Observe(4, b, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(5, b, nil, observe.SemanticEvidence{}, observe.SelectionAt{})

	trs := g.Transitions()
	if len(trs) != 1 {
		t.Fatalf("%d transitions, want 1: %+v", len(trs), trs)
	}
	if n := trs[0].Preceded[observe.NavConfirm]; n == 0 {
		t.Errorf("the confirmed change carries no navigation: %+v\nThe press that caused "+
			"it arrived one look before the screen showed it, and a change attributed to "+
			"nothing is a change no learning layer can use.", trs[0])
	}
	if trs[0].Unattributed != 0 {
		t.Errorf("the change is recorded as unattributed despite a press preceding it")
	}
}

// An unreadable frame between two readable ones is a dropout, not a journey.
func TestOneUnreadableFrameIsNotAVisitToNowhere(t *testing.T) {
	a, b := twoCompositions()
	var g observe.ScreenSegmenter

	// A, A, a fragment nobody can place, A, A. The screen never left.
	walkScreens(&g, a, a, a[:1], a, a)

	for _, c := range g.Crossings() {
		if c.From == observe.ScreenStateUnknown || c.To == observe.ScreenStateUnknown {
			t.Errorf("one unreadable frame produced a crossing: %+v", g.Crossings())
		}
	}
	if g.Flicker.Dropouts != 1 {
		t.Errorf("dropouts = %d, want 1: %+v", g.Flicker.Dropouts, g.Flicker)
	}
	// AND it is not counted as a candidate missed transition. A dropout the gate stepped
	// over is the gate working; burying the real misses under one per dropped frame would
	// make the miss number useless.
	if g.Flicker.RejectedWithInput != 0 {
		t.Errorf("a dropout was counted as a candidate missed transition: %+v", g.Flicker)
	}
	_ = b
}

// And a change seen THROUGH an unreadable frame is still one change, with its cause.
func TestAChangeSeenThroughAGapIsOneChange(t *testing.T) {
	a, b := twoCompositions()
	var g observe.ScreenSegmenter

	g.Observe(1, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(2, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	// The press rides on the half-rendered frame, which is where a real one lands.
	g.Observe(3, a[:1], []observe.InputEvent{{Intent: observe.NavConfirm, AtMS: 300}},
		observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(4, b, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(5, b, nil, observe.SemanticEvidence{}, observe.SelectionAt{})

	trs := g.Transitions()
	if len(trs) != 1 {
		t.Fatalf("%d transitions across a gap, want 1: %+v", len(trs), trs)
	}
	if trs[0].From == observe.ScreenStateUnknown || trs[0].To == observe.ScreenStateUnknown {
		t.Errorf("the change was recorded through the unplaceable state: %+v", trs[0])
	}
	if n := trs[0].Preceded[observe.NavConfirm]; n == 0 {
		t.Errorf("the press that rode on the unreadable frame was lost: %+v\nThe frame "+
			"could not be read; what the player did during it was observed by a "+
			"different producer and is not in doubt.", trs[0])
	}
}

// THE MISS DIRECTION, measured separately.
//
// A change that a deliberate press preceded and that went away before it could be confirmed is
// the gate's own error, and it has to be countable. The alternative is a gate that reports only
// the false transitions it removed.
func TestAChangeTooFastToConfirmIsCountedAsACandidateMiss(t *testing.T) {
	a, b := twoCompositions()
	var g observe.ScreenSegmenter

	g.Observe(1, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(2, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(3, a, []observe.InputEvent{{Intent: observe.NavConfirm, AtMS: 300}},
		observe.SemanticEvidence{}, observe.SelectionAt{})
	// B for exactly one look, then back. Real or not, the gate cannot tell — and it says so.
	g.Observe(4, b, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(5, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})

	if len(g.Transitions()) != 0 {
		t.Fatalf("a one-look change was recorded: %+v", g.Transitions())
	}
	if g.Flicker.RejectedWithInput != 1 {
		t.Errorf("candidate misses = %d, want 1: %+v\nA refusal that a deliberate press "+
			"preceded is the one the window might be wrong about, and a gate that counted "+
			"only its successes would never show it.",
			g.Flicker.RejectedWithInput, g.Flicker)
	}
	if g.Flicker.Window != observe.StabilityLooks {
		t.Errorf("the stats do not carry the window that produced them: %+v", g.Flicker)
	}
}

// The uncertainty is visible while it lasts, rather than the screen looking settled.
func TestAScreenBeingConfirmedSaysItIsChanging(t *testing.T) {
	a, b := twoCompositions()
	var g observe.ScreenSegmenter

	walkScreens(&g, a, a)
	if to, looks, _ := g.Pending(); to != "" || looks != 0 {
		t.Errorf("a settled screen reports a pending change %s/%d", to, looks)
	}

	g.Observe(3, b, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	to, looks, _ := g.Pending()
	if to == "" || looks != 1 {
		t.Errorf("mid-change the segmenter reports %s/%d, want a candidate with one look.\n"+
			"A gate that hid its pending state would be a smoother that silently lagged.",
			to, looks)
	}

	g.Observe(4, b, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	if to, looks, _ := g.Pending(); to != "" || looks != 0 {
		t.Errorf("after confirmation a change is still pending: %s/%d", to, looks)
	}
}

// An episode begins when a screen BECOMES current, not every time a frame disagrees.
//
// Nineteen re-entries of a flickering state were nineteen episodes, and presence ratios are
// computed against episodes — so a perfectly stable element looked bursty because the SCREEN
// was being re-entered, not because the element went anywhere.
func TestFlickerDoesNotManufactureEpisodes(t *testing.T) {
	a, b := twoCompositions()
	var g observe.ScreenSegmenter

	walkScreens(&g, a, a, b, a, a, b, a, a)

	for _, st := range g.States() {
		if st.Episodes > 1 {
			t.Errorf("state %s accumulated %d episodes from single-frame fluctuations",
				st.ID, st.Episodes)
		}
	}
}

// A GAP IN THE MIDDLE OF A CHANGE does not restart the change, and does not lose its cause.
//
// The case neither of the tests above reaches, and the one the live cadence produces most: the
// new screen appears, the next frame is half-rendered, and the frame after that shows the new
// screen again. If the unreadable frame reset the candidate, every change that blinked would
// need three clean looks instead of two — and on a two-second inference that is six seconds of
// a menu Marco cannot follow.
func TestAnUnreadableFrameDuringAChangeDoesNotRestartIt(t *testing.T) {
	a, b := twoCompositions()
	var g observe.ScreenSegmenter

	g.Observe(1, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(2, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	// The new screen appears: one look.
	g.Observe(3, b, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	// A half-rendered frame, carrying the press the player made during it.
	g.Observe(4, a[:1], []observe.InputEvent{{Intent: observe.NavConfirm, AtMS: 400}},
		observe.SemanticEvidence{}, observe.SelectionAt{})
	// And the new screen again. That is the SECOND look at b, not the first.
	g.Observe(5, b, nil, observe.SemanticEvidence{}, observe.SelectionAt{})

	trs := g.Transitions()
	if len(trs) != 1 {
		t.Fatalf("%d transitions, want 1: %+v\nAn unreadable frame in the middle of a "+
			"change restarted the count, so the change needed three clean looks instead "+
			"of two.", len(trs), trs)
	}
	if trs[0].To == observe.ScreenStateUnknown {
		t.Errorf("the change was recorded as a journey into the unplaceable state: %+v",
			trs[0])
	}
	if n := trs[0].Preceded[observe.NavConfirm]; n == 0 {
		t.Errorf("the press that rode on the unreadable frame DURING the change was "+
			"lost: %+v\nThe frame could not be read; what the player did during it was "+
			"observed by a different producer and is not in doubt.", trs[0])
	}
	if g.Flicker.Confirmed != 1 {
		t.Errorf("flicker stats %+v, want one confirmed change", g.Flicker)
	}
}

// And the uncertainty is kept rather than smoothed away: the unreadable looks are counted ON
// the change, and reported while it is pending.
//
// Which matters because the gate's cost is paid in looks: "the screen changed, and one of the
// two looks that established it could not be read" is a weaker claim than two clean looks, and
// a reader deciding whether to trust a transition needs to be told which they have.
func TestAChangeRemembersHowMuchOfItCouldNotBeRead(t *testing.T) {
	a, b := twoCompositions()
	var g observe.ScreenSegmenter

	g.Observe(1, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(2, a, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	g.Observe(3, b, nil, observe.SemanticEvidence{}, observe.SelectionAt{})

	// An unreadable frame in the middle of the change.
	g.Observe(4, a[:1], nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	to, looks, unreadable := g.Pending()
	if to == "" {
		t.Fatalf("the change was abandoned across one unreadable frame (looks=%d)", looks)
	}
	if unreadable != 1 {
		t.Errorf("the pending change carries %d unreadable look(s), want 1.\nThe gap is "+
			"the uncertainty between the before and the after; a change that does not "+
			"carry it reads as one Marco watched the whole of.", unreadable)
	}

	g.Observe(5, b, nil, observe.SemanticEvidence{}, observe.SelectionAt{})
	if n := len(g.Transitions()); n != 1 {
		t.Fatalf("%d transitions, want 1: %+v", n, g.Transitions())
	}
	if to, _, _ := g.Pending(); to != "" {
		t.Errorf("a confirmed change is still pending: %s", to)
	}
}
