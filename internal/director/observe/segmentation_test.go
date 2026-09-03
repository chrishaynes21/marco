package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ── what the screen says it is decides whether it is still the same screen ────
//
// # The bidirectional defect these hold closed, both measured from ScreenSegmenter.Observe
//
// `replacedMass` counts structures whose KIND arrived or left. Driven with Settings-shaped
// readings, the segmenter collapsed four pages into ONE state — same kinds, different amounts,
// nothing "replaced". Driven with one page whose content churned between text and images, it split
// that page into THREE — same page, different kinds, plenty "replaced".
//
// One rule, run in two directions, and structure alone cannot tell them apart: "the content of
// this page changed" and "this is a different page" look identical at the level of kinds and
// cells.

// pageLike is one page of a settings-shaped application, described the way a reading describes it.
//
// A navigation rail of list items down the left, and a content pane on the right. What changes
// between PAGES is how much is in the content pane — never the KINDS, because every page of such
// an application is made of list items, buttons and text. That is the whole difficulty.
func pageLike(rows, buttons, texts int) []observe.ShadowRegion {
	var out []observe.ShadowRegion
	at := func(role string, col, i int) observe.ShadowRegion {
		return observe.ShadowRegion{
			Role: role, Kind: directorapi.KindDescribed,
			Region: observe.Region{
				X: float64(col)/10 + 0.01, Y: 0.05 + float64(i%9)*0.09,
				Width: 0.18, Height: 0.07,
			},
		}
	}
	for i := 0; i < 12; i++ {
		out = append(out, at("list_item", 0, i))
	}
	for i := 0; i < rows; i++ {
		out = append(out, at("list_item", 4, i))
	}
	for i := 0; i < buttons; i++ {
		out = append(out, at("button", 6, i))
	}
	for i := 0; i < texts; i++ {
		out = append(out, at("text", 7, i))
	}
	return out
}

// says is one reading's admitted destination claim.
func says(name string) observe.SemanticEvidence {
	return observe.SemanticEvidence{Observed: true, PlaceName: name}
}

// realStates is every state the segmenter actually placed a reading in.
func realStates(seen map[observe.ScreenStateID]int) int {
	n := 0
	for id := range seen {
		if id != observe.ScreenStateUnknown {
			n++
		}
	}
	return n
}

// walk drives a sequence of readings and reports which state each landed in.
func segWalk(g *observe.ScreenSegmenter, n *int,
	readings []observe.ShadowRegion, sem observe.SemanticEvidence,
	times int) []observe.ScreenStateID {

	var out []observe.ScreenStateID
	for range times {
		*n++
		out = append(out, g.Observe(*n, readings, nil, sem))
	}
	return out
}

// FOUR PAGES THAT SAY FOUR DIFFERENT THINGS ARE FOUR SCREENS.
//
// # Measured, from this function, before the fix
//
//	4 pages × 2 readings  →  1 state
//	state_1 <- [Home Home  Bluetooth Bluetooth  Mouse Mouse  System System]
//
// Live, the same collapse produced a session state entered ONCE that absorbed a whole four-page
// walk: eleven readings over twenty-four seconds, no two of which agreed on a whole composition,
// so it never settled and none of the four pages became a Place. Settlement was right; it was
// being asked about a state that should never have been one state.
//
// Deleting the destinationElsewhere arm must fail this.
func TestFourPagesWithFourDestinationsAreFourStates(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	seen := map[observe.ScreenStateID]int{}
	for _, p := range []struct {
		name                 string
		rows, buttons, texts int
	}{
		{"Home", 9, 2, 14},
		{"Bluetooth & devices", 7, 5, 11},
		{"Mouse", 4, 8, 9},
		{"System", 11, 3, 17},
	} {
		for _, id := range segWalk(&g, &n, pageLike(p.rows, p.buttons, p.texts),
			says(p.name), 3) {
			seen[id]++
		}
	}
	if got := realStates(seen); got != 4 {
		t.Fatalf("four pages, each saying what it is, segmented into %d state(s): %v.\n"+
			"Structure cannot tell `the content of this page changed` from `this is a "+
			"different page`, and the screen was saying which the whole time.",
			got, seen)
	}
}

// AND ONE PAGE THAT KEEPS SAYING THE SAME THING IS ONE SCREEN, HOWEVER ITS CONTENT CHURNS.
//
// # Measured, from this function, before the fix
//
//	1 page × 8 readings  →  3 states
//
// Live, that is three durable Chrome Places all called `MARCO`, carrying fifty-five affordances
// between them. This half matters as much as the other: a fix that only acted on a CHANGED
// destination would leave the churning page splitting exactly as it did, and half a bidirectional
// defect closed is a fix that looks like it worked.
//
// Deleting the destinationHere arm must fail this.
func TestOneDestinationSurvivesItsContentChurning(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	withImages := func(images int) []observe.ShadowRegion {
		out := pageLike(9, 2, 14)
		for i := 0; i < images; i++ {
			out = append(out, observe.ShadowRegion{
				Role: "image", Kind: directorapi.KindDescribed,
				Region: observe.Region{
					X: 0.71, Y: 0.05 + float64(i%9)*0.09,
					Width: 0.18, Height: 0.07,
				},
			})
		}
		return out
	}
	seen := map[observe.ScreenStateID]int{}
	for _, images := range []int{0, 0, 14, 14, 0, 0, 14, 14} {
		for _, id := range segWalk(&g, &n, withImages(images), says("MARCO"), 1) {
			seen[id]++
		}
	}
	if got := realStates(seen); got != 1 {
		t.Fatalf("one page whose content churned became %d state(s): %v.\n"+
			"It said `MARCO` on every reading. Content changing kind is what a page "+
			"does; it is not the page becoming another page.", got, seen)
	}
}

// AND A SCREEN THAT SAYS NOTHING IS SEGMENTED EXACTLY AS IT ALWAYS WAS.
//
// The control that matters most for the rest of the desktop. Most interfaces make no destination
// claim at all — a game, a canvas, an editor pane — and for them nothing about this changed.
//
// Two readings of one composition and two of a materially different one, with no claim on any of
// them: the structural rule decides, and it still sees a local replacement.
func TestWithNoDestinationClaimStructureStillDecides(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	none := observe.SemanticEvidence{Observed: true}
	seen := map[observe.ScreenStateID]int{}
	for _, id := range segWalk(&g, &n, pageLike(9, 2, 14), none, 3) {
		seen[id]++
	}
	quiet := realStates(seen)
	if quiet != 1 {
		t.Fatalf("one unchanging screen with no claim became %d states", quiet)
	}
	// A composition whose KINDS differ — the case the structural rule exists to catch.
	churned := pageLike(9, 2, 14)
	for i := 0; i < 20; i++ {
		churned = append(churned, observe.ShadowRegion{
			Role: "image", Kind: directorapi.KindDescribed,
			Region: observe.Region{X: 0.71, Y: 0.05 + float64(i%9)*0.09,
				Width: 0.18, Height: 0.07},
		})
	}
	for _, id := range segWalk(&g, &n, churned, none, 3) {
		seen[id]++
	}
	if got := realStates(seen); got <= quiet {
		t.Errorf("with no destination claim the structural rule stopped separating a "+
			"surface whose kinds were replaced: %d state(s). Absence of a claim must "+
			"leave the old behaviour exactly as it was.", got)
	}
}

// AND A DESTINATION SEEN ONCE IS NOT YET WORTH COMPARING AGAINST.
//
// A transition frame carries the name of the page being LEFT. Comparing against a word seen once
// would manufacture a boundary out of exactly the frames bridging exists for — so the remembered
// side of the comparison is `settledPlaceName`, the same recurrence rule naming already uses.
//
// Here the first reading is the only one that ever says `Home`, and it cannot on its own make the
// second reading a boundary.
func TestADestinationSeenOnceDoesNotDecideAnything(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	first := segWalk(&g, &n, pageLike(9, 2, 14), says("Home"), 1)
	// The same screen, one reading later, saying something else — with `Home` never having
	// recurred, so nothing settled and there is nothing trustworthy to compare against.
	second := segWalk(&g, &n, pageLike(9, 2, 14), says("Somewhere else"), 1)
	if first[0] == observe.ScreenStateUnknown {
		t.Skip("the fixture's first reading was unplaceable, so this proves nothing")
	}
	if second[0] != first[0] {
		t.Errorf("a destination word seen exactly once moved the screen from %q to %q. "+
			"A transition frame carries the name of the page being left.",
			first[0], second[0])
	}
}

// AND A DESTINATION THAT GOES QUIET IS NOT A BOUNDARY.
//
// Perception reads what it can. A reading that could not make out the selected navigation says
// nothing about where the person is, and treating silence as a change would turn every partial
// reading into somewhere new. Absence is not a boundary — it is a fallback to structure.
func TestADestinationGoingQuietIsNotABoundary(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	settled := segWalk(&g, &n, pageLike(9, 2, 14), says("Home"), 3)
	established := settled[len(settled)-1]
	if established == observe.ScreenStateUnknown {
		t.Skip("the fixture never settled, so this proves nothing")
	}
	quiet := segWalk(&g, &n, pageLike(9, 2, 14),
		observe.SemanticEvidence{Observed: true}, 2)
	for _, id := range quiet {
		if id != established {
			t.Errorf("a reading that could not make out the destination moved the "+
				"screen from %q to %q", established, id)
		}
	}
}

// AND A DESTINATION ARRIVING LATE SETTLES THE SCREEN IT ARRIVED ON.
//
// A claim is not available on every reading — scoped reading runs on a fraction of them, and the
// first sighting of a screen often carries none. When one does arrive it decides from then on,
// and it does not reach backwards: the readings already credited to a state stay credited to it.
func TestADestinationArrivingLateDecidesFromThereOn(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	none := observe.SemanticEvidence{Observed: true}
	early := segWalk(&g, &n, pageLike(9, 2, 14), none, 3)
	started := early[len(early)-1]
	if started == observe.ScreenStateUnknown {
		t.Skip("the fixture never placed a reading, so this proves nothing")
	}
	// The word arrives, twice, so it settles — and says the same screen.
	for _, id := range segWalk(&g, &n, pageLike(9, 2, 14), says("Home"), 3) {
		if id != started {
			t.Fatalf("a destination arriving on a screen already being read moved it "+
				"from %q to %q", started, id)
		}
	}
	// And now a different one is a boundary.
	moved := segWalk(&g, &n, pageLike(7, 5, 11), says("Bluetooth & devices"), 3)
	if moved[len(moved)-1] == started {
		t.Error("a settled destination changing did not separate the screens")
	}
}

// AND A DIFFERENT DESTINATION SEPARATES TWO SCREENS THAT LOOK IDENTICAL.
//
// The hardest case and the one structure can never reach: two pages of an application built from
// exactly the same parts in exactly the same places. Nothing about the composition differs at all,
// so no structural rule of any tolerance could tell them apart — and the screen is saying which it
// is on every reading.
func TestTwoIdenticalCompositionsWithDifferentDestinationsAreTwoScreens(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	same := pageLike(9, 2, 14)
	a := segWalk(&g, &n, same, says("Home"), 3)
	b := segWalk(&g, &n, same, says("System"), 3)
	if a[len(a)-1] == observe.ScreenStateUnknown {
		t.Skip("the fixture never placed a reading, so this proves nothing")
	}
	if a[len(a)-1] == b[len(b)-1] {
		t.Fatalf("two pages with identical compositions and different destinations are "+
			"one state (%q). No structural rule can separate these; the only evidence "+
			"that they are different screens is what they say they are.", a[len(a)-1])
	}
}

// AND ORDINARY REFLOW UNDER ONE DESTINATION IS ONE SCREEN.
//
// A window resized, a pane collapsed, a list scrolled. The composition moves and the destination
// does not, which is what continuity means.
func TestReflowUnderOneDestinationIsOneScreen(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	seen := map[observe.ScreenStateID]int{}
	for _, rows := range []int{9, 9, 6, 12, 7, 9} {
		for _, id := range segWalk(&g, &n, pageLike(rows, 2, 14), says("Home"), 1) {
			seen[id]++
		}
	}
	if got := realStates(seen); got != 1 {
		t.Errorf("one screen reflowing under one destination became %d state(s): %v",
			got, seen)
	}
}

// AND NONE OF THIS REACHED WHAT A PLACE IS REMEMBERED BY.
//
// # The licence for using richer evidence in one layer and not the other
//
// A wrong answer in segmentation costs a session. A wrong answer in identity is a permanent memory
// that outlives every session which could have contradicted it. That asymmetry is the entire
// reason the destination claim is admissible for transient continuity and not for durable
// identity — see ADR-124.
//
// So this drives the two screens the segmenter now separates and asks what they would be
// REMEMBERED by. They are built from exactly the same parts, so the answer has to be that memory
// cannot tell them apart: Marco distinguishes them while it is looking, and mints no second
// permanent record out of a word.
//
// Putting the destination anywhere into the signature must fail this.
func TestSeparatingTwoScreensDoesNotSeparateWhatTheyAreRememberedBy(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	same := pageLike(9, 2, 14)
	a := segWalk(&g, &n, same, says("Home"), 3)
	b := segWalk(&g, &n, same, says("System"), 3)
	first, second := a[len(a)-1], b[len(b)-1]
	if first == observe.ScreenStateUnknown || first == second {
		t.Fatalf("the fixture did not separate the two screens (%q, %q), so it proves "+
			"nothing about identity", first, second)
	}

	totals := observe.ShadowTotals{States: g.States(), CurrentState: second}
	th := observe.DefaultHypothesisThresholds()
	sigA, okA := observe.SignatureOfState(totals, first, th)
	sigB, okB := observe.SignatureOfState(totals, second, th)
	if !okA || !okB {
		t.Fatalf("a state had no signature (%v, %v), so this proves nothing", okA, okB)
	}
	// NOT `different`, which is the verdict that would mean the word had leaked.
	//
	// `candidate` is the honest answer for two identical compositions carrying no interface
	// terms — memory cannot be SURE they are the same screen, and saying so is what stops a
	// coin toss. What it must never say is that they are positively different, because the
	// only thing that differs between them is a word this layer may not see.
	if v := observe.CompareStructure(sigA, sigB); v == observe.MatchDifferent {
		t.Fatalf("two screens built from identical parts are remembered as %s. The "+
			"destination claim reached durable identity: Marco may separate them while "+
			"it is looking, and must not mint two permanent records out of a word.", v)
	}
}
