package main

// WHAT A LOOK TAKEN AT THE AUTHORISATION BOUNDARY SAYS ABOUT ITSELF.
//
// # The condition
//
// The boundary check reads the observer's STANDING claim — its screen and its observation
// epoch. The stabiliser keeps that claim still on purpose while a changed arrangement settles:
// a look that differs is counted `Unsettled` and the standing screen is returned, for
// `VisualSettleLooks` looks. ADR-170 measured the 150 raw adjacent-frame changes that damping
// suppresses, and it is correct and untouched.
//
// But a person's F7 arrives inside exactly that window. At ten looks a second a settle takes
// three looks and an authorisation takes a second or two, so BOTH confirmations before an
// ordinary click could take a fresh look at a CHANGED interface, read back the old screen and
// the old epoch, and clear the check. Taking the look was never the hard part; asking the
// stabiliser what it made of the look is.
//
// # What these drive
//
// The real `fastobserve.Observer`, the real stabiliser, the real `freshOf`, the real
// `worldMoved`, and — for the zero-input assertion — the real `workflowrun.Runner` with that
// composite installed as its confirmation. Everything except the two calls that need a desktop:
// acquiring the window and asking what leads the foreground.
//
// The earlier version of this file handed `visual_2` to `worldMoved` directly, so it never
// established that production generates a changed identity before clicking — which is the same
// gap as the condition itself.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/platform/recordhost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// aPanel is one arrangement: a strip along the top and a grid of tiles below it.
//
// The strip never moves, so a changed panel is a change of EVERYTHING ELSE. That is the hard
// case and not the easy one: a panel that moved the control too would be caught by the target
// term and the screen term would never be reached.
func aPanel(y float64, n int) []observe.Region {
	out := []observe.Region{{X: 0.05, Y: 0.02, Width: 0.90, Height: 0.06}}
	for i := 0; i < n; i++ {
		out = append(out, observe.Region{
			X: 0.10 + float64(i%6)*0.13, Y: y + float64(i/6)*0.09,
			Width: 0.11, Height: 0.07,
		})
	}
	return out
}

// theControl is the point the plan was approved at: inside the strip every panel keeps.
var theControl = observe.PointerAt{X: 0.5, Y: 0.05}

func boundaryLook(regions []observe.Region, rs ...observe.ScreenReading) fastobserve.Look {
	return fastobserve.Look{
		At:       time.Unix(1700000000, 0).UTC(),
		Window:   fastobserve.WindowRef{ID: "hwnd:1", Generation: 3},
		Front:    true,
		Regions:  regions,
		Readings: rs,
	}
}

// settledOn runs a real observer until one screen is standing, and returns it with the plan
// that was made from it.
func settledOn(t *testing.T, regions []observe.Region,
	said observe.ScreenText) (*fastobserve.Observer, workflowrun.Sighting) {

	t.Helper()
	o, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range o.Updates() {
		}
	}()
	t.Cleanup(func() { o.Close() })

	var up fastobserve.Update
	for i := 0; i < 8; i++ {
		lk := boundaryLook(regions)
		lk.Text = said
		up = o.Observe(lk)
	}
	if up.Screen == observe.VisualScreenUnknown {
		t.Fatal("nothing settled, so there is no plan to make")
	}
	return o, workflowrun.Sighting{
		Screen: up.Screen, Epoch: up.Epoch, Generation: 3,
		Window: "hwnd:1", Front: true,
	}
}

// askBoundary is the decision half of `stillThere`, driven with a look offered here.
//
// Everything `stillThere` does apart from acquiring the window and asking the desktop what is
// in front: the production `freshFrom` folds the look through the real observer, the real
// counters either side of it say what the stabiliser made of it, and the real `worldMoved`
// decides. Nothing here re-implements the order the fold reads things in, which is the half a
// harness gets wrong.
//
// NO READER AND NO PIXELS, so these cases are the run a person gets without `--reading`: words
// are not load-bearing for any screen, the policy is pixels, and a reading handed to the
// stabiliser directly is the stabiliser's own input. The boundary's dependence on the words of
// its OWN capture is in boundarytext_test.go, which has a reader it can stop and start.
func askBoundary(o *fastobserve.Observer, scope workflowstore.Scope,
	planned workflowrun.Sighting, lk fastobserve.Look) (bool, string) {

	now := worldNow{Alive: true, State: "valid", Generation: 3,
		Front: observe.FrontTarget, Application: "thegame"}
	now.Fresh = freshFrom(context.Background(), o, nil, lk, nil, theControl, 0)
	now.Epoch, now.Screen = now.Fresh.Epoch, now.Fresh.Screen
	return worldMoved("thegame", scope, now, planned)
}

// A CHANGED PANEL IS REFUSED WHILE IT IS STILL SETTLING, WHICH IS THE WHOLE WINDOW.
//
// Different geometry: the tiles move down and multiply. The standing screen does not change,
// because it is not allowed to yet — so every term the check had before this agrees, and the
// fresh term is the only one that can refuse. Asserted on EVERY look of the settle run, because
// any one of them is where an authorisation lands.
func TestASettlingChangeIsRefusedOnEveryLookOfItsSettleRun(t *testing.T) {
	scope := workflowstore.NormaliseScope("thegame")
	o, planned := settledOn(t, aPanel(0.30, 12),
		observe.TextOf(directorapi.SourceOCR, []string{"AUDIO", "GRAPHICS", "VIDEO"}))

	// THE POSITIVE CONTROL FIRST, so a boundary that refused everything could not
	// pass the refusals below.
	if ok, why := askBoundary(o, scope, planned, boundaryLook(aPanel(0.30, 12))); !ok {
		t.Fatalf("an unchanged panel was refused at the boundary: %s", why)
	}

	// ── THE HOLE IS THE LOOKS BEFORE THE LAST ONE OF THE SETTLE RUN ──
	//
	// The look that COMPLETES the run promotes, and from that look on the ordinary epoch
	// and screen terms catch the change — that is the only state the check could ever
	// see, and the reason the hole was invisible. The looks before it are the window: the
	// standing screen has not moved, nothing above the fresh terms can refuse, and a
	// person's authorisation lands here.
	changed := aPanel(0.62, 23)
	window := o.SettleLooks() - 1
	if window < 1 {
		t.Fatalf("a settle run of %d look(s) has no damped window to test",
			o.SettleLooks())
	}
	for look := 1; look <= window; look++ {
		was := o.Screen()
		ok, why := askBoundary(o, scope, planned, boundaryLook(changed))
		if ok {
			t.Fatalf("look %d of the settle run cleared a changed panel to "+
				"press: the standing screen is still %q because the "+
				"stabiliser has not promoted yet, and that is not "+
				"evidence that nothing moved", look, o.Screen())
		}
		if !strings.Contains(why, "did not agree") {
			t.Errorf("look %d refused for the wrong reason: %s", look, why)
		}
		// AND THE STANDING CLAIM REALLY DID NOT MOVE, which is what makes this
		// the damped window rather than an ordinary screen change. Without this
		// the test would pass against a stabiliser that promoted immediately,
		// and would be asserting nothing about the condition.
		if o.Screen() != was || o.Screen() != planned.Screen {
			t.Fatalf("look %d is not inside the damped window: the standing "+
				"screen went from %q to %q", look, was, o.Screen())
		}
	}
	// AND THE LOOK THAT COMPLETES THE RUN IS REFUSED TOO, by whichever term gets there.
	if ok, why := askBoundary(o, scope, planned, boundaryLook(changed)); ok {
		t.Error("the look that promoted a different screen was cleared to press")
	} else if !strings.Contains(why, "screen") && !strings.Contains(why, "did not agree") {
		t.Errorf("the refusal on the promoting look is %q", why)
	}
}

// AND SIMILAR GEOMETRY WITH CONFLICTING TEXT IS REFUSED TOO.
//
// The case the geometric terms cannot see: a panel swapped for one of the same shape. The
// arrangement agrees, so the look would be counted as settled — until the stabiliser's own text
// rule says this is not that screen. Nothing new is measured here; the reading is offered the
// way the loop offers one, and the stabiliser decides.
func TestAPanelThatReadsDifferentlyIsRefusedThoughItsShapeAgrees(t *testing.T) {
	scope := workflowstore.NormaliseScope("thegame")
	shape := aPanel(0.30, 12)
	o, planned := settledOn(t, shape,
		observe.TextOf(directorapi.SourceOCR, []string{"AUDIO", "GRAPHICS", "VIDEO"}))
	epoch := o.Epoch()

	// THE SAME SHAPE, SAYING SOMETHING ELSE. Two readings, because one that nothing
	// corroborates is the weaker evidence — see VisualStability.TextRefusedAlone.
	elsewhere := observe.TextOf(directorapi.SourceOCR,
		[]string{"INVENTORY", "CRAFTING", "TRIBE"})
	refused, why := false, ""
	for i := 0; i < 2; i++ {
		lk := boundaryLook(shape, observe.ScreenReading{
			Text: elsewhere, FromLook: 9 + i, Generation: 3, Epoch: epoch,
		})
		var ok bool
		ok, why = askBoundary(o, scope, planned, lk)
		if !ok {
			refused = true
			break
		}
	}
	if !refused {
		t.Fatalf("a panel of the same shape reading %v was cleared to press on a "+
			"plan made against %q", elsewhere.Words, planned.Screen)
	}
	st := o.Stability()
	if st.TextRefusedMerge == 0 && st.TextRefusedReuse == 0 {
		t.Errorf("the refusal (%s) did not come from the stabiliser's text rule, "+
			"so something else in this test is doing the work", why)
	}
}

// AND A CONTROL THAT WENT AWAY UNDER A SETTLED SCREEN IS REFUSED.
//
// The second half of "current target evidence", and a different question from whether the
// screen is the same one. An interface can settle clean and have nothing under the approved
// coordinate: a list that scrolled, a row that went away, a panel swapped for one of the same
// overall shape. Every other term agrees and the click lands on whatever is now behind it.
func TestAPointWithNothingUnderItIsRefusedOnASettledScreen(t *testing.T) {
	scope := workflowstore.NormaliseScope("thegame")
	full := aPanel(0.30, 12)
	o, planned := settledOn(t, full,
		observe.TextOf(directorapi.SourceOCR, []string{"AUDIO", "GRAPHICS", "VIDEO"}))

	// THE SAME PANEL WITH THE STRIP GONE. Arrangement close enough to settle, and
	// nothing proposed where the plan's point is.
	without := full[1:]
	ok, why := askBoundary(o, scope, planned, boundaryLook(without))
	if ok {
		t.Fatalf("a point with nothing proposed under it was cleared to press on "+
			"screen %q", o.Screen())
	}
	if !strings.Contains(why, "nothing was proposed under the approved point") &&
		!strings.Contains(why, "did not agree") {
		t.Errorf("the refusal is %q, and neither fresh term claims it", why)
	}
}

// AND NOTHING IS PRESSED. The real runner, with that boundary installed.
//
// The tests above assert the decision; this asserts the consequence, because a refusal the
// runner ignored would satisfy every one of them. One authorised step, one changed panel
// underneath it, and the notebook where the mouse would be has to be empty.
func TestAChangedPanelUnderAnAuthorisedStepSendsNoInput(t *testing.T) {
	scope := workflowstore.NormaliseScope("thegame")
	o, seen := settledOn(t, aPanel(0.30, 12),
		observe.TextOf(directorapi.SourceOCR, []string{"AUDIO", "GRAPHICS", "VIDEO"}))
	changed := aPanel(0.62, 23)

	art := oneClick(t)
	book := recordhost.New()
	var marco directorapi.MarcoRunner = marcorunner.New(map[string]runtime.Host{
		"OS": book,
	})
	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED fixture"); err != nil {
		t.Fatal(err)
	}
	grant := &permits{epoch: 5, generation: 1}
	r := workflowrun.New(art, nil, answers{"now_1": "screen_a"}, grant).
		WithExecutor(marcoexec.New(marco), false).
		// THE REAL BOUNDARY, over the real observer. The plan it compares against is
		// the one the observer itself settled on, so what is being tested is the
		// freshness of the evidence and not the artifact's screen names.
		WithConfirm(func(ctx context.Context, at workflowrun.Sighting,
			point observe.PointerAt) (bool, string) {

			now := worldNow{Alive: true, State: "valid", Generation: 3,
				Front: observe.FrontTarget, Application: "thegame"}
			now.Fresh = freshFrom(ctx, o, nil, boundaryLook(changed), nil,
				point, 0)
			now.Epoch, now.Screen = now.Fresh.Epoch, now.Fresh.Screen
			return worldMoved("thegame", scope, now, seen)
		})

	look := workflowrun.Sighting{
		Screen: "now_1", Epoch: 5, Generation: 1,
		Window: "hwnd:1", Front: true, At: time.Now(),
		Bounds: directorapi.Rect{X: 100, Y: 50, Width: 1000, Height: 500},
		Outcomes: []observe.Reacquisition{{
			Target: "control_1", Screen: "now_1",
			Result: observe.ReacquireMatched, Controls: 1,
			Where:   observe.Region{X: 0.4, Y: 0.5, Width: 0.1, Height: 0.04},
			Because: "a fixture outcome",
		}},
	}
	got := r.Advance(context.Background(), s, look)

	if lines := book.Lines(); len(lines) != 0 {
		t.Fatalf("%d call(s) reached the host with the panel changed under the "+
			"approved point:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if got.Outcome != workflowrun.OutcomeRefused {
		t.Errorf("the step ended %q (%s); a boundary that refused has to stop "+
			"the action", got.Outcome, got.Because)
	}
	if !strings.Contains(got.Because, "did not agree") &&
		!strings.Contains(got.Because, "no longer standing") &&
		!strings.Contains(got.Because, "different screen") {
		t.Errorf("the refusal does not come from the fresh look: %s", got.Because)
	}
}

// AND A BOUNDARY THAT COULD NOT LOOK REFUSES.
//
// `freshEvidence` zero means no look was taken: `relook` nil, or a capture that failed. The
// four terms above it all answer from the observer's standing claim, which is the claim the
// plan was made from — so with no fresh evidence there is nothing in the check that describes
// now, and clearing it would be clearing on the plan's own say-so.
func TestABoundaryWithNoFreshEvidenceRefuses(t *testing.T) {
	planned := workflowrun.Sighting{
		Epoch: 7, Generation: 2, Screen: "visual_1",
		Window: "hwnd:1", Front: true,
	}
	now := worldNow{
		Alive: true, State: "valid", Application: "arkascended",
		Generation: 2, Epoch: 7, Screen: "visual_1",
		Front: observe.FrontTarget,
	}
	ok, why := worldMoved("arkascended",
		workflowstore.NormaliseScope("arkascended"), now, planned)
	if ok {
		t.Fatal("a press was cleared with nothing looked at, on four terms that " +
			"all read the claim the plan was made from")
	}
	if !strings.Contains(why, "no look could be taken") {
		t.Errorf("the refusal is %q, and it should say nothing was looked at", why)
	}

	// AND WITH A LOOK THAT AGREED, THE SAME WORLD CLEARS. Otherwise every refusal in
	// this file would pass with `worldMoved` replaced by `return false`.
	now.Fresh = freshEvidence{Took: true, Agreed: true, Counted: "settled",
		Target: true}
	if ok, why := worldMoved("arkascended",
		workflowstore.NormaliseScope("arkascended"), now, planned); !ok {
		t.Fatalf("an unchanged world with a look that agreed refused: %s", why)
	}
}

// AND freshOf READS THE COUNTER THAT MOVED, not one of the others.
//
// The pure half, named apart because this is where a mistake hides: taking a look is visible in
// a capture count, and reading the wrong counter is visible nowhere. One case per counter, and
// every one of them except `Settled` is a refusal.
func TestFreshOfNamesWhatTheStabiliserCountedThisLookAs(t *testing.T) {
	in := []observe.Region{{X: 0.4, Y: 0.0, Width: 0.3, Height: 0.1}}
	for _, tc := range []struct {
		name   string
		moved  func(*observe.VisualStability)
		agreed bool
		says   string
	}{
		{"settled", func(s *observe.VisualStability) { s.Settled++ }, true,
			"settled"},
		{"unsettled", func(s *observe.VisualStability) { s.Unsettled++ }, false,
			"has not settled"},
		{"warming", func(s *observe.VisualStability) { s.Warming++ }, false,
			"no screen has settled yet"},
		{"unobserved", func(s *observe.VisualStability) { s.Unobserved++ }, false,
			"nothing was found in the pixels"},
		{"unobservable", func(s *observe.VisualStability) { s.Unobservable++ },
			false, "observable"},
		{"promoted", func(s *observe.VisualStability) { s.Promotions++ }, false,
			"promoted a different screen"},
		{"interrupted", func(s *observe.VisualStability) { s.Interrupted++ }, false,
			"had been replaced"},
		{"nothing moved", func(s *observe.VisualStability) {}, false,
			"not classified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			was := observe.VisualStability{Looks: 10, Settled: 4,
				Unsettled: 2, Warming: 1, Unobserved: 1,
				Unobservable: 1, Promotions: 1, Interrupted: 1}
			now := was
			now.Looks++
			tc.moved(&now)
			got := freshOf(was, now, in, theControl, false)
			if !got.Took {
				t.Fatal("a look that reached the stabiliser is reported as " +
					"one nobody took")
			}
			if got.Agreed != tc.agreed {
				t.Errorf("agreed=%v, want %v (%s)", got.Agreed, tc.agreed,
					got.Counted)
			}
			if !strings.Contains(got.Counted, tc.says) {
				t.Errorf("counted %q, want it to mention %q",
					got.Counted, tc.says)
			}
			if !got.Target {
				t.Error("the point is inside the one region offered and " +
					"the target term says it is not")
			}
		})
	}

	// AND THE POINT IS ASKED OF THIS LOOK'S OWN PROPOSALS.
	was := observe.VisualStability{}
	now := observe.VisualStability{Settled: 1}
	if got := freshOf(was, now, nil, theControl, false); got.Target {
		t.Error("a look that proposed nothing is reported as holding the control")
	}
	away := []observe.Region{{X: 0.0, Y: 0.5, Width: 0.2, Height: 0.1}}
	if got := freshOf(was, now, away, theControl, false); got.Target {
		t.Error("a region nowhere near the point is reported as holding it")
	}

	// AND `Said` TAKES BOTH HALVES, neither of which is sufficient.
	//
	// The counter alone cannot tell a reading of these pixels from a stale one the
	// epoch rule happened to admit — which is the whole defect. The caller alone
	// cannot say the words reached the text rule, because an inadmissible or
	// unusable reading is dropped inside the observer.
	for _, tc := range []struct {
		name string
		own  bool
		read int
		want bool
	}{
		{"this look's reading, applied", true, 1, true},
		{"this look's reading, never applied", true, 0, false},
		{"a reading of some other frame", false, 1, false},
		{"no reading at all", false, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			was := observe.VisualStability{Settled: 3, TextRead: 4}
			now := was
			now.Settled++
			now.TextRead += tc.read
			if got := freshOf(was, now, in, theControl, tc.own); got.Said != tc.want {
				t.Errorf("said=%v, want %v", got.Said, tc.want)
			}
		})
	}
}

// AND holdsPoint IS BOUNDED ON ALL FOUR SIDES, each one separately.
//
// A mutation dropping the lower-Y bound survived the test above, because that region misses on
// X as well and the X check gets there first. One region, one axis at a time, each case missing
// on exactly the edge it names — which is the only way an edge can be shown to be load-bearing.
func TestHoldsPointIsBoundedOnEveryEdge(t *testing.T) {
	// A BOX IN THE MIDDLE, so there is room to miss on every side.
	// BINARY-EXACT QUARTERS, so the half-open assertions below are about the rule and
	// not about floating point: 0.40+0.20 is 0.6000000000000001 and a point at 0.60 is
	// then inside a box that ends there, which says nothing either way.
	r := observe.Region{X: 0.25, Y: 0.25, Width: 0.25, Height: 0.25}
	in := observe.PointerAt{X: 0.375, Y: 0.375}
	if !holdsPoint(r, in) {
		t.Fatal("a point in the middle of a region is not inside it")
	}
	for _, tc := range []struct {
		edge string
		at   observe.PointerAt
	}{
		{"left of it", observe.PointerAt{X: 0.125, Y: 0.375}},
		{"right of it", observe.PointerAt{X: 0.625, Y: 0.375}},
		{"above it", observe.PointerAt{X: 0.375, Y: 0.125}},
		{"below it", observe.PointerAt{X: 0.375, Y: 0.625}},
	} {
		if holdsPoint(r, tc.at) {
			t.Errorf("a point %s is reported as inside it, so that edge is "+
				"not checked and a click can land outside the control it "+
				"was matched to", tc.edge)
		}
	}
	// AND THE FAR EDGES ARE HALF-OPEN, which is the convention `observe.Region` is
	// built with: a point exactly on the right or bottom edge belongs to the next
	// region, not this one.
	if holdsPoint(r, observe.PointerAt{X: 0.5, Y: 0.375}) {
		t.Error("the right edge is inclusive, so two adjacent regions both hold " +
			"the point between them")
	}
	if holdsPoint(r, observe.PointerAt{X: 0.375, Y: 0.5}) {
		t.Error("the bottom edge is inclusive, so two stacked regions both hold " +
			"the point between them")
	}
	// AND THE NEAR EDGES ARE CLOSED, so a point on a control's own corner is on it.
	if !holdsPoint(r, observe.PointerAt{X: 0.25, Y: 0.25}) {
		t.Error("a point on the top-left corner is outside, so a control's own " +
			"edge is not part of it")
	}
}
