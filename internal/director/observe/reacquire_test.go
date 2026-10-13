package observe_test

// REACQUISITION, AND MOSTLY THE CASES WHERE IT MUST REFUSE.
//
// The owner set the standard these tests are written to:
//
//	A system that confidently identifies the wrong button is worse than one that says it cannot
//	find the button.
//
// So the positive case gets one test and the negative cases get most of the file: a container
// standing in for its control, a neighbour that moved into the target's place, a proposer that
// found more or fewer regions, a control that disappeared, and a target the session itself could
// not tell from another.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// row is one menu row at a normalised position.
func row(y float64) observe.Region {
	return observe.Region{X: 0.4, Y: y, Width: 0.2, Height: 0.05}
}

// panel is the container the rows sit in.
func menuPanel() observe.Region {
	return observe.Region{X: 0.35, Y: 0.1, Width: 0.3, Height: 0.6}
}

// centre is the middle of a region, as a press.
func centre(r observe.Region) observe.PointerAt {
	return observe.PointerAt{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
}

// demo clicks one row on one screen, with the panel containing it, and returns the correlator.
func demo(t *testing.T, screen observe.VisualScreenID, rows ...float64) *observe.ClickTargets {
	t.Helper()
	c := &observe.ClickTargets{}
	look := 1
	for _, y := range rows {
		all := []observe.Region{menuPanel()}
		for _, yy := range rows {
			all = append(all, row(yy))
		}
		c.Press(look, centre(row(y)), screen, all)
		look += 10
	}
	return c
}

// ── the positive case ─────────────────────────────────────────────────────────

// A DEMONSTRATED TARGET IS FOUND AGAIN FROM THE PROPOSALS ALONE.
//
// The press position is not an input to Reacquire and cannot be: it takes a screen identity and a
// set of rectangles. So a match here is geometry against geometry, which is the capability the
// whole exercise is about.
func TestADemonstratedTargetIsFoundAgainOnALaterLook(t *testing.T) {
	c := demo(t, "visual_1", 0.2)

	// The same screen later, with the proposer finding the same three rows and the panel
	// — the ordinary case of returning somewhere.
	got := c.Reacquire("visual_1", []observe.Region{menuPanel(), row(0.2), row(0.3), row(0.4)})
	if len(got) != 1 {
		t.Fatalf("%d outcome(s) for one demonstrated target, want 1", len(got))
	}
	r := got[0]
	if r.Result != observe.ReacquireMatched {
		t.Fatalf("result %q (%s), want matched: the control is on screen at the "+
			"rectangle it was clicked as", r.Result, r.Because)
	}
	if !sameish(r.Where, row(0.2)) {
		t.Errorf("matched %v, want the row at 0.2", r.Where)
	}
	if r.Controls != 1 {
		t.Errorf("%d control(s) described by the supporting proposals, want 1", r.Controls)
	}
	if !r.Actionable() {
		t.Error("a certain match reports that it could not ground a press")
	}
}

// ── the negative cases ────────────────────────────────────────────────────────

// A CONTAINER IS NOT ITS CONTROL.
//
// The trap that makes click targets hard. Five of seventeen presses in the real demonstration
// landed inside more than one region, so the container is always there in the evidence — and a
// matcher that accepted it would reacquire the menu panel as the button, press its centre, and
// hit whatever happens to be in the middle of the menu.
//
// Deleting the container exclusion must fail this.
func TestAContainerIsNotTheControlItContained(t *testing.T) {
	c := demo(t, "visual_1", 0.2)

	// The panel is still here. The rows are not — the list is empty, or the proposer
	// only found the frame this time.
	got := c.Reacquire("visual_1", []observe.Region{menuPanel()})
	if len(got) != 1 {
		t.Fatalf("%d outcome(s), want 1", len(got))
	}
	if got[0].Result != observe.ReacquireUnobservable {
		t.Fatalf("result %q (%s), want unobservable: only the container is on screen",
			got[0].Result, got[0].Because)
	}
	// AND IT SAYS SO, because `unobservable` while the panel is plainly visible reads as
	// a capture fault and sends a reader hunting for one.
	if !strings.Contains(got[0].Because, "container is not the control") {
		t.Errorf("the refusal does not explain that a container was found instead: %q",
			got[0].Because)
	}
	if got[0].Actionable() {
		t.Error("an unobservable target reports that it could ground a press")
	}
}

// A NEIGHBOUR THAT MOVED INTO THE TARGET'S PLACE IS A FALSE MATCH, AND GEOMETRY CANNOT SEE IT.
//
// The honest limit, tested so that it is recorded rather than discovered later. If the list
// scrolls by one row, the row that now sits where the demonstrated one sat satisfies every
// geometric test there is, and reacquisition returns `matched` for the wrong control.
//
// This test asserts the WRONG answer on purpose. It is the measurement of a known limit, and the
// day something fixes it this test is what will fail and say what changed.
func TestAShiftedNeighbourIsMatchedAndThatIsTheLimitOfGeometry(t *testing.T) {
	c := demo(t, "visual_1", 0.2)

	// The whole list moved up by one row height. Nothing is where it was; a different
	// entry is where the target was.
	shifted := []observe.Region{menuPanel(), row(0.15), row(0.2), row(0.25)}
	got := c.Reacquire("visual_1", shifted)
	if got[0].Result != observe.ReacquireMatched {
		t.Skipf("a shifted list no longer matches (%s) — if that is deliberate, this "+
			"test is the record of the old behaviour and should be rewritten",
			got[0].Because)
	}
	if !sameish(got[0].Where, row(0.2)) {
		t.Errorf("matched %v; the limit being recorded is that the row now AT 0.2 is "+
			"matched, whatever entry it is", got[0].Where)
	}
	// SO THE THING THAT SEPARATES THEM CANNOT BE GEOMETRY. Recorded here, beside the
	// failure, so nobody reads the match rate as a recognition rate: what a match means
	// is "a control of this shape is in this place", and a second demonstration is what
	// turns that into evidence about a control.
}

// A PROPOSER THAT FINDS THE CONTROL TWICE HAS NOT FOUND TWO CONTROLS.
//
// A row and a span of that row agree with each other — mutual centre containment — and reporting
// that as an ambiguity would be reporting a disagreement nobody has. The distinction is between
// how many PROPOSALS support the target and how many CONTROLS they describe.
//
// Deleting the grouping must fail this.
func TestTheSameControlReadTwiceIsOneControl(t *testing.T) {
	c := demo(t, "visual_1", 0.2)

	// The row, and a slightly larger reading of the row. Each contains the other's
	// centre, so they are two readings of one thing.
	wider := observe.Region{X: 0.38, Y: 0.195, Width: 0.24, Height: 0.06}
	got := c.Reacquire("visual_1", []observe.Region{menuPanel(), row(0.2), wider})
	if got[0].Result != observe.ReacquireMatched {
		t.Fatalf("result %q (%s), want matched: two readings of one row are one control",
			got[0].Result, got[0].Because)
	}
	if len(got[0].Supporting) != 2 {
		t.Errorf("%d supporting proposal(s), want 2 — both readings are evidence and "+
			"both are kept", len(got[0].Supporting))
	}
	if got[0].Controls != 1 {
		t.Errorf("%d control(s), want 1", got[0].Controls)
	}
	// And the matched region is the smaller reading, which is the same hypothesis the
	// press path states.
	if !sameish(got[0].Where, row(0.2)) {
		t.Errorf("matched %v, want the smaller reading", got[0].Where)
	}
}

// TWO PROPOSALS THAT EACH AGREE WITH THE TARGET BUT NOT WITH EACH OTHER ARE AMBIGUOUS.
//
// The case the owner named: `Multiple plausible candidates remain`. Two rectangles can each be
// the same rectangle as the stored reading while not being the same rectangle as one another —
// mutual centre containment is not transitive — and that is two candidate controls. Picking the
// smaller would be exactly the silent decision this layer exists to refuse.
func TestTwoDisagreeingCandidatesAreAmbiguousAndNeitherIsChosen(t *testing.T) {
	c := &observe.ClickTargets{}
	// The demonstrated reading, and two overlapping rows either side of it. Each agrees
	// with it; neither agrees with the other.
	stored := observe.Region{X: 0.4, Y: 0.2, Width: 0.2, Height: 0.1}
	above := observe.Region{X: 0.4, Y: 0.17, Width: 0.2, Height: 0.1}
	below := observe.Region{X: 0.4, Y: 0.23, Width: 0.2, Height: 0.1}
	c.Press(1, centre(stored), "visual_1", []observe.Region{stored})

	got := c.Reacquire("visual_1", []observe.Region{above, below})
	if len(got) != 1 {
		t.Fatalf("%d outcome(s), want 1", len(got))
	}
	if got[0].Result != observe.ReacquireAmbiguous {
		t.Fatalf("result %q (%s), want ambiguous: two candidates that do not agree "+
			"with each other are two controls", got[0].Result, got[0].Because)
	}
	if (got[0].Where != observe.Region{}) {
		t.Errorf("an ambiguous outcome still names a region %v: that is the silent "+
			"choice this layer refuses", got[0].Where)
	}
	if got[0].Controls != 2 {
		t.Errorf("%d control(s) described, want 2", got[0].Controls)
	}
	if got[0].Actionable() {
		t.Error("an ambiguous outcome reports that it could ground a press")
	}
	// AND BOTH ARE KEPT. Evidence that was discarded cannot later contradict anything.
	if len(got[0].Supporting) != 2 {
		t.Errorf("%d supporting proposal(s) kept, want 2", len(got[0].Supporting))
	}
}

// A CONTROL INDISTINGUISHABLE FROM ITS OWN CONTAINER IS REFUSED, NOT MATCHED.
//
// `sameRect` is mutual centre containment and NOT a size test: a thin row that straddles its
// panel's centre contains the panel's centre, and the panel contains the row's, so the two are
// "the same rectangle". A reacquisition that took the panel as support would offer the middle of
// the menu as the button.
//
// Found by a test, not by reasoning: the row at a third of the way down the panel hid it, and the
// row at the panel's own centre exposed it. The refusal is the conservative side, and it is a
// real cost — a control in that position cannot be reacquired at all — so it says which refusal
// it is rather than reporting the same silence as a control that is simply gone.
func TestAControlIndistinguishableFromItsContainerIsRefused(t *testing.T) {
	c := &observe.ClickTargets{}
	// A row sitting exactly at the panel's vertical centre.
	p := menuPanel()
	straddling := observe.Region{X: 0.4, Y: p.Y + p.Height/2, Width: 0.2, Height: 0.04}
	c.Press(1, centre(straddling), "visual_1", []observe.Region{p, straddling})

	got := c.Reacquire("visual_1", []observe.Region{p, straddling})
	if len(got) != 1 {
		t.Fatalf("%d outcome(s), want 1", len(got))
	}
	if got[0].Result != observe.ReacquireUnobservable {
		t.Fatalf("result %q (%s), want unobservable: nothing on this look "+
			"distinguishes the row from the panel it sits in the middle of",
			got[0].Result, got[0].Because)
	}
	if !strings.Contains(got[0].Because, "not a size test") {
		t.Errorf("the refusal does not say which refusal it is, so it reads the same "+
			"as a control that is simply gone: %q", got[0].Because)
	}
}

// A CONTROL THAT DISAPPEARED IS UNOBSERVABLE, AND SO IS ONE ON A SCREEN NOBODY COULD IDENTIFY.
//
// Two different silences with the same answer. The second matters more: matching a target against
// an arrangement that failed to identify would attribute a control to a look nobody could read,
// which is where a confident wrong answer is cheapest to produce.
func TestADisappearedTargetAndAnUnreadableScreenAreBothUnobservable(t *testing.T) {
	c := demo(t, "visual_1", 0.2)

	// Gone entirely: a different screen's worth of proposals under the same identity.
	got := c.Reacquire("visual_1", []observe.Region{
		{X: 0.05, Y: 0.8, Width: 0.1, Height: 0.03},
	})
	if got[0].Result != observe.ReacquireUnobservable {
		t.Errorf("result %q for a control that is not on screen, want unobservable",
			got[0].Result)
	}
	// Nothing at all found.
	if got := c.Reacquire("visual_1", nil); got[0].Result != observe.ReacquireUnobservable {
		t.Errorf("result %q with no proposals at all, want unobservable", got[0].Result)
	}
	// AND AN UNIDENTIFIED SCREEN GETS NO OUTCOMES RATHER THAN A GUESS.
	//
	// This needs a target that was actually minted on an unidentified screen, which is
	// an ordinary thing: a press made before any arrangement settled carries
	// VisualScreenUnknown as its source. Without one, the guard looks correct because
	// no target's screen matches anyway — which is how the first version of this test
	// passed with the guard deleted.
	u := &observe.ClickTargets{}
	u.Press(1, centre(row(0.2)), observe.VisualScreenUnknown,
		[]observe.Region{menuPanel(), row(0.2)})
	if n := len(u.Registry()); n != 1 {
		t.Fatalf("%d target(s) minted on an unidentified screen, want 1 — this test "+
			"needs one to be exercising anything", n)
	}
	for _, screen := range []observe.VisualScreenID{
		"", observe.VisualScreenUnknown, observe.VisualScreenUnobserved,
	} {
		if got := u.Reacquire(screen, []observe.Region{menuPanel(), row(0.2)}); got != nil {
			t.Errorf("screen %q produced %d outcome(s); a look nobody could identify "+
				"is not a screen a target can be found on, and a match there "+
				"would attribute a control to a look that failed",
				screen, len(got))
		}
	}
}

// A TARGET THE SESSION COULD NOT TELL FROM ANOTHER DOES NOT BECOME CERTAIN BY BEING FOUND.
//
// `ClickEvidence.MaybeOneControl` is the session reporting two identities on one screen that led
// to the same destination — the shape of one control read two ways, and equally the shape of two
// different entries that open the same thing. A match that claimed to know which is which would
// be inventing a distinction the evidence does not contain.
func TestAnUncertainIdentityStaysUncertainWhenItIsFound(t *testing.T) {
	c := &observe.ClickTargets{}
	a := row(0.2)
	b := row(0.4)
	all := []observe.Region{menuPanel(), a, b}
	// Two different rows, each clicked once, each followed by the same change.
	c.Press(1, centre(a), "visual_1", all)
	c.Arrived(2, observe.VisualTransition{From: "visual_1", To: "visual_2"})
	c.Press(10, centre(b), "visual_1", all)
	c.Arrived(11, observe.VisualTransition{From: "visual_1", To: "visual_2"})

	if n := len(c.Evidence().MaybeOneControl); n == 0 {
		t.Fatalf("the session reports no uncertain pair, so this test is not "+
			"exercising one: %+v", c.Evidence().Attributed)
	}
	got := c.Reacquire("visual_1", all)
	for _, r := range got {
		if r.Result != observe.ReacquireMatched {
			continue
		}
		if !r.Uncertain {
			t.Errorf("%s matched and reports no uncertainty, although the session "+
				"could not distinguish it from another identity on this screen",
				r.Target)
		}
		if r.Actionable() {
			t.Errorf("%s matched an identity the session cannot pin down and "+
				"reports that it could ground a press", r.Target)
		}
	}
}

// ── the registry ──────────────────────────────────────────────────────────────

// THE REGISTRY CARRIES EVERY PIECE OF EVIDENCE THE OWNER LISTED, AND NO SECOND COPY OF ANY.
//
// Source screen and provenance, the control's own readings, the containing alternatives, the
// observed destination with its cause, and the identity uncertainty. Derived from the same state
// the evidence is derived from, because two statements of one fact drifting apart is this
// subsystem's recorded worst defect class.
func TestTheRegistryCarriesTheWholeEvidenceForATarget(t *testing.T) {
	c := &observe.ClickTargets{}
	r := row(0.2)
	c.Press(5, centre(r), "visual_1", []observe.Region{menuPanel(), r})
	c.Arrived(7, observe.VisualTransition{From: "visual_1", To: "visual_2"})
	c.Press(40, centre(r), "visual_1", []observe.Region{menuPanel(), r})
	c.Arrived(42, observe.VisualTransition{From: "visual_1", To: "visual_2"})
	// A SECOND CONTROL WITH A DIFFERENT DESTINATION, so "this target's destinations"
	// is a claim with something to be wrong about. With one target in the registry, a
	// filter that returned every step would look correct.
	other := row(0.5)
	c.Press(60, centre(other), "visual_1", []observe.Region{menuPanel(), other})
	c.Arrived(62, observe.VisualTransition{From: "visual_1", To: "visual_6"})

	reg := c.Registry()
	if len(reg) != 2 {
		t.Fatalf("%d record(s) for two controls, want 2", len(reg))
	}
	got := reg[0]
	if n := len(got.Destinations); n != 1 {
		t.Fatalf("%s holds %d destination(s), want 1: the other control's step "+
			"belongs to the other control", got.ID, n)
	}
	if got.Screen != "visual_1" {
		t.Errorf("source screen %q, want visual_1", got.Screen)
	}
	if got.Presses != 2 || !got.Repeated() {
		t.Errorf("%d press(es), repeated=%v; want 2 and true — repeated evidence is "+
			"what separates a demonstration from a claim", got.Presses, got.Repeated())
	}
	if got.FirstLook != 5 || got.LastLook != 40 {
		t.Errorf("clicked on looks %d..%d, want 5..40. Without them a reacquisition "+
			"cannot be scored anywhere but on the demonstration itself",
			got.FirstLook, got.LastLook)
	}
	if len(got.Where) == 0 {
		t.Error("the record holds no reading of the control, so nothing could be found")
	}
	if len(got.Containers) == 0 {
		t.Error("the record holds no containing alternative, so the control/container " +
			"ambiguity was discarded and can never contradict a match")
	}
	if got.Destinations[0].To != "visual_2" {
		t.Errorf("destination %q, want visual_2", got.Destinations[0].To)
	}
	if got.Destinations[0].Cause != observe.CauseClick {
		t.Errorf("destination cause %q, want click", got.Destinations[0].Cause)
	}
	// AND THE CONTROL CLICKED ONCE IS NOT REPEATED EVIDENCE. One demonstration is a
	// demonstration; the whole point of the distinction is that nothing independent has
	// confirmed it, so a registry that called every target repeated would report twelve
	// corroborated targets where the recording holds four.
	once := reg[1]
	if once.Presses != 1 {
		t.Fatalf("%s has %d press(es), want 1", once.ID, once.Presses)
	}
	if once.Repeated() {
		t.Errorf("%s was clicked once and reports repeated evidence", once.ID)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// sameish reports whether two regions are the same rectangle to within rounding.
func sameish(a, b observe.Region) bool {
	near := func(x, y float64) bool { return x-y < 1e-9 && y-x < 1e-9 }
	return near(a.X, b.X) && near(a.Y, b.Y) && near(a.Width, b.Width) &&
		near(a.Height, b.Height)
}

// A LOOK WITH FAR MORE PROPOSALS GIVES THE SAME ANSWER.
//
// The proposer found 204.7 candidates a frame on the Trackmania corpus and 73 to 400 a screen on
// the live Rocket League run, and the count moves between looks of one screen. A matcher whose
// answer depended on how much else was on screen would be reporting the proposer's mood.
//
// Noise is added both far from the target and overlapping it, because those are different
// failures: the first would be a scan that got lucky, the second a relation too loose to
// separate neighbours.
func TestMoreProposalsDoNotChangeTheAnswer(t *testing.T) {
	c := demo(t, "visual_1", 0.2)
	target := row(0.2)

	quiet := []observe.Region{menuPanel(), target}
	base := c.Reacquire("visual_1", quiet)
	if base[0].Result != observe.ReacquireMatched {
		t.Fatalf("the quiet look did not match (%s), so this test proves nothing",
			base[0].Because)
	}

	// Two hundred rectangles elsewhere on the screen, and a dozen near the target but
	// not on it.
	noisy := append([]observe.Region{}, quiet...)
	for i := range 200 {
		noisy = append(noisy, observe.Region{
			X: 0.01 + float64(i%10)*0.03, Y: 0.75 + float64(i/10)*0.002,
			Width: 0.02, Height: 0.001,
		})
	}
	for i := range 12 {
		noisy = append(noisy, observe.Region{
			X: 0.62 + float64(i)*0.01, Y: 0.2, Width: 0.008, Height: 0.05,
		})
	}
	got := c.Reacquire("visual_1", noisy)
	if got[0].Result != base[0].Result {
		t.Errorf("result %q among %d proposals against %q among %d (%s)",
			got[0].Result, len(noisy), base[0].Result, len(quiet), got[0].Because)
	}
	if !sameish(got[0].Where, base[0].Where) {
		t.Errorf("matched %v among noise, %v without it", got[0].Where, base[0].Where)
	}
}

// A SCREEN LEFT AND RETURNED TO STILL FINDS ITS TARGET, THROUGH THE PRODUCTION OBSERVER.
//
// The capability in one sentence: click something, go somewhere, come back, find it again. Driven
// through `fastobserve.Observer` rather than the correlator directly, because that is the type
// the live command runs and a reacquisition nothing can reach is the ninth recorded case of
// complete code never invoked.
func TestATargetSurvivesLeavingTheScreenAndComingBack(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	menu := []observe.Region{menuPanel(), row(0.2), row(0.3), row(0.4)}
	elsewhere := []observe.Region{{X: 0.1, Y: 0.6, Width: 0.5, Height: 0.2}}
	at := time.Unix(1700000000, 0).UTC()
	ms := 0
	fold := func(regions []observe.Region, presses ...observe.PointerAt) {
		ms += 100
		o.Observe(fastobserve.Look{
			At:      at.Add(time.Duration(ms) * time.Millisecond),
			Window:  fastobserve.WindowRef{Generation: 1},
			Regions: regions, Front: true, Presses: presses,
		})
	}

	// Settle the menu, click a row, leave, settle elsewhere, then come back.
	for range 4 {
		fold(menu)
	}
	fold(menu, centre(row(0.3)))
	for range 4 {
		fold(elsewhere)
	}
	for range 4 {
		fold(menu)
	}

	if len(o.Registry()) == 0 {
		t.Fatal("no target was demonstrated, so the return proves nothing")
	}
	got := o.Reacquire(menu)
	if len(got) == 0 {
		t.Fatalf("back on the menu, the observer offers no outcome for %d "+
			"demonstrated target(s): the screen identity did not survive the "+
			"return, or the registry is not scoped to it", len(o.Registry()))
	}
	var matched int
	for _, r := range got {
		if r.Result == observe.ReacquireMatched {
			matched++
			if !sameish(r.Where, row(0.3)) {
				t.Errorf("matched %v, want the row that was clicked", r.Where)
			}
		}
	}
	if matched != 1 {
		t.Errorf("%d target(s) matched on the returned-to screen, want 1: %+v",
			matched, got)
	}
}
