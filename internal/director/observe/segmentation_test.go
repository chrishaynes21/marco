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

// ── settling a screen that kept saying what it was ────────────────────────────

// settledIn reports whether the segmenter came to consider a state settled.
func settledIn(g *observe.ScreenSegmenter, id observe.ScreenStateID) (bool, observe.ScreenState) {
	for _, st := range g.States() {
		if st.ID == id {
			return st.Settled, st
		}
	}
	return false, observe.ScreenState{}
}

// A SCREEN THAT KEPT SAYING WHAT IT WAS SETTLES THROUGH ITS WOBBLE.
//
// # The measured failure
//
// A page walked through at normal speed gets two or three readings, and a live interface almost
// never presents the identical role histogram twice in that window. From the trace, on a Settings
// page that never became a Place:
//
//	2 shapes, same kinds, worst count drift 6 [group list_item text]
//
// The same kinds throughout, differing only in how many, saying `Mouse` on every reading. Nothing
// contradicted anything. Settlement asked for one whole histogram to repeat, and a real screen
// somebody visited disappeared because a status line moved.
//
// Deleting the coherent arm of settledWhole must fail this.
func TestAScreenThatKeptSayingWhatItWasSettlesThroughItsWobble(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	var last observe.ScreenStateID
	// Three readings, three histograms, one destination — the shape the live trace showed.
	for _, texts := range []int{14, 15, 16} {
		n++
		last = g.Observe(n, pageLike(9, 2, texts), nil, says("Mouse"))
	}
	if last == observe.ScreenStateUnknown {
		t.Fatal("the readings were not placed at all, so this proves nothing about settling")
	}
	settled, st := settledIn(&g, last)
	_, agreeing, distinct, _, _ := observe.SettlementOf(st)
	if agreeing >= 2 {
		t.Fatalf("the fixture repeated a composition (agreeing %d), so it is not the case "+
			"this test is about", agreeing)
	}
	if distinct < 3 {
		t.Fatalf("the fixture produced %d distinct shapes, want the three the live trace "+
			"showed", distinct)
	}
	if !settled {
		t.Fatalf("a screen read three times, saying `Mouse` every time and made of the " +
			"same kinds throughout, did not settle. No whole histogram repeated — which " +
			"is what a live interface does, not what a different screen looks like.")
	}
}

// AND ITS IDENTITY IS STILL A COMPOSITION MARCO ACTUALLY SAW.
//
// Never an average. A place may not be remembered as something it never was: the producer this
// replaced moded each role independently and could emit a composition equal to none of the
// samples, which is how the last surviving twin got into a live store.
func TestAWobblingScreenIsRememberedAsAShapeItActuallyHad(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	var last observe.ScreenStateID
	shapes := []int{14, 15, 16}
	for _, texts := range shapes {
		n++
		last = g.Observe(n, pageLike(9, 2, texts), nil, says("Mouse"))
	}
	_, st := settledIn(&g, last)
	if st.Roles == nil {
		t.Fatal("the settled state carries no composition")
	}
	got := st.Roles["text"]
	for _, texts := range shapes {
		if got == texts {
			return
		}
	}
	t.Errorf("the state settled on text=%d, which no reading ever showed (%v). A place may "+
		"not be remembered as something it never was.", got, shapes)
}

// AND A SCREEN WHOSE KINDS CHANGE DOES NOT SETTLE THIS WAY.
//
// Counts moving is drift. KINDS moving is a page mid-arrival or a different page — the same run
// showed a Settings state whose compositions differed by `progress_bar`, which is a loading
// indicator, and a Chrome state churning across seven shapes and six kinds.
//
// Deleting the same-kinds requirement must fail this.
func TestAScreenWhoseKindsChangeDoesNotSettleThroughCoherence(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	var last observe.ScreenStateID
	for i, extra := range []int{0, 6, 13} {
		regions := pageLike(9, 2, 14)
		for j := 0; j < extra; j++ {
			regions = append(regions, observe.ShadowRegion{
				Role: "progress_bar", Kind: directorapi.KindDescribed,
				Region: observe.Region{X: 0.5, Y: 0.02 + float64(j)*0.03,
					Width: 0.2, Height: 0.02},
			})
		}
		n++
		id := g.Observe(n, regions, nil, says("Mouse"))
		if i > 0 {
			last = id
		}
	}
	if last == observe.ScreenStateUnknown {
		t.Skip("the readings were not placed, so this proves nothing")
	}
	if settled, _ := settledIn(&g, last); settled {
		t.Error("a screen whose compositions gained and lost a KIND settled through the " +
			"coherent path. A composition that gained a kind is a page mid-arrival or " +
			"another page, and neither may settle on a word.")
	}
}

// AND ONE READING NEVER SETTLES, however good its name.
//
// The goal is not one-frame Place creation. A single sighting with a destination on it is exactly
// what a transition frame looks like.
func TestOneReadingNeverSettlesHoweverWellNamed(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 1
	id := g.Observe(n, pageLike(9, 2, 14), nil, says("Mouse"))
	if id == observe.ScreenStateUnknown {
		return // not placed at all is a stronger version of the same refusal
	}
	if settled, _ := settledIn(&g, id); settled {
		t.Error("one reading settled a screen. A single sighting carrying a name is what a " +
			"transition frame looks like.")
	}
}

// AND TWO WORDS AGAINST ONE STATE IS MARCO NOT KNOWING, EVEN WHEN ONE OF THEM WINS.
//
// # Why a majority is not enough here
//
// `settledPlaceName` tolerates a minority word: three sightings of `Mouse` against one of
// `System` settles on Mouse, which is the right answer for NAMING a screen Marco is confident
// about. It is the wrong answer for deciding whether a wobbling state may become durable at all,
// because the minority word is evidence that segmentation put two screens together — and a
// permissive settlement turning a segmentation mistake into durable knowledge is exactly the
// failure this sequence has been narrowing.
//
// The word has to be unanimous. Reached here through the real segmenter: a claim arriving before
// anything has settled cannot create a boundary (see TestADestinationSeenOnceDoesNotDecideAnything),
// so a state genuinely can accumulate two words early in its life.
//
// Deleting the unanimity requirement must fail this.
func TestAMinorityWordStopsAWobblingStateSettling(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	var last observe.ScreenStateID
	// `System` lands second, before `Mouse` has settled, so it cannot be a boundary — and
	// the state now carries both words while its counts drift.
	for i, texts := range []int{14, 15, 16, 17} {
		word := "Mouse"
		if i == 1 {
			word = "System"
		}
		n++
		last = g.Observe(n, pageLike(9, 2, texts), nil, says(word))
	}
	if last == observe.ScreenStateUnknown {
		t.Skip("the readings were not placed, so this proves nothing")
	}
	settled, st := settledIn(&g, last)
	if len(st.PlaceNames) < 2 {
		t.Skipf("the fixture produced one word (%v), so it is not the case being held",
			st.PlaceNames)
	}
	if observe.SettledPlaceNameFor(st) == "" {
		t.Skipf("the fixture's words tied, which a different guard already refuses: %v",
			st.PlaceNames)
	}
	if settled {
		t.Errorf("a state carrying %v settled through the coherent path. One of them wins "+
			"the naming vote; that is not the same as Marco knowing which screen it was "+
			"looking at.", st.PlaceNames)
	}
}

// AND A WORD SEEN ONCE CANNOT SETTLE A WOBBLING SCREEN.
//
// Scoped reading runs on a fraction of inferences, so a state routinely holds readings that
// carried no claim at all. One of them saying `Mouse` is a single sighting of a word — which is
// what a transition frame carrying the name of the page being LEFT looks like — and the coherent
// path must want the same recurrence naming wants.
//
// Deleting the settled-name requirement must fail this.
func TestAWordSeenOnceCannotSettleAWobblingScreen(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	none := observe.SemanticEvidence{Observed: true}
	var last observe.ScreenStateID
	for i, texts := range []int{14, 15, 16, 17} {
		sem := none
		if i == 0 {
			sem = says("Mouse")
		}
		n++
		last = g.Observe(n, pageLike(9, 2, texts), nil, sem)
	}
	if last == observe.ScreenStateUnknown {
		t.Skip("the readings were not placed, so this proves nothing")
	}
	settled, st := settledIn(&g, last)
	if len(st.PlaceNames) != 1 {
		t.Skipf("the fixture tallied %v, so it is not the case being held", st.PlaceNames)
	}
	if settled {
		t.Errorf("a screen whose destination was read exactly once (%v) settled through "+
			"its wobble. A word seen once is a transition frame carrying the name of "+
			"the page being left.", st.PlaceNames)
	}
}

// AND A SCREEN WITH NO DESTINATION CLAIM SETTLES EXACTLY AS IT ALWAYS DID.
//
// Most interfaces make no claim. Their settlement is unchanged: one whole composition, twice.
func TestWithNoDestinationSettlementIsUnchanged(t *testing.T) {
	var g observe.ScreenSegmenter
	n := 0
	none := observe.SemanticEvidence{Observed: true}
	var last observe.ScreenStateID
	for _, texts := range []int{14, 15, 16} {
		n++
		last = g.Observe(n, pageLike(9, 2, texts), nil, none)
	}
	if last == observe.ScreenStateUnknown {
		t.Skip("the readings were not placed, so this proves nothing")
	}
	if settled, _ := settledIn(&g, last); settled {
		t.Error("a wobbling screen with no destination claim settled. Nothing about the " +
			"structural rule changed for interfaces that say nothing about themselves.")
	}
	// AND THE CONTROL: repeat one composition and it settles, as it always has.
	for range 2 {
		n++
		last = g.Observe(n, pageLike(9, 2, 14), nil, none)
	}
	if settled, _ := settledIn(&g, last); !settled {
		t.Error("a repeated composition with no claim stopped settling, which would be " +
			"this change breaking the behaviour it was supposed to leave alone")
	}
}
