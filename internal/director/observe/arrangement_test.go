package observe_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// rects builds a look: n rectangles stacked in one column, at a given x.
//
// Hand-built rather than taken from a recording, because a fixture from one session freezes
// that session's accidents — and because the thing under test is a rule about TIME, which a
// real recording cannot isolate.
func rects(x float64, n int) []observe.Region {
	out := make([]observe.Region, 0, n)
	for i := range n {
		out = append(out, observe.Region{
			X: x, Y: 0.1 + float64(i)*0.08, Width: 0.2, Height: 0.05,
		})
	}
	return out
}

// cluster is n small rectangles inside one cell of the segmenter's grid, around (x, y).
//
// Small and tight on purpose: the question these answer is about how MANY rectangles a cell
// holds, so every one of them has to land in the same cell however many there are.
func cluster(n int, x, y float64) []observe.Region {
	out := make([]observe.Region, 0, n)
	for i := range n {
		out = append(out, observe.Region{
			X: x, Y: y + float64(i)*0.004, Width: 0.02, Height: 0.003,
		})
	}
	return out
}

func look(t *testing.T, v *observe.VisualScreens, regions []observe.Region) observe.VisualScreenID {
	t.Helper()
	return v.Observe(observe.ArrangeLook(regions), time.Now())
}

// A ONE-FRAME FLICKER IS NOT A SCREEN, and this is the whole reason the layer exists.
//
// # The measurement that forced it
//
// [[ADR-170-what-an-unnamed-arrangement-would-have-segmented]] measured 150 changes of
// arrangement between adjacent frames over 900 frames of a real recording that held seven
// distinguishable screens. One spurious transition every six frames is not a weaker route; it
// is a transition storm, and a transition storm looks like evidence.
//
// So a different arrangement has to hold for VisualSettleLooks consecutive looks before it is
// allowed to be a screen. Deleting the settle rule must fail this.
func TestAOneLookFlickerIsNotANewScreen(t *testing.T) {
	var v observe.VisualScreens
	// Settle on a screen.
	var first observe.VisualScreenID
	for range observe.VisualSettleLooks {
		first = look(t, &v, rects(0.1, 6))
	}
	if first == observe.VisualScreenUnknown {
		t.Fatalf("%d looks at one arrangement settled on nothing", observe.VisualSettleLooks)
	}
	// One look at something else entirely, then back.
	if got := look(t, &v, rects(0.8, 2)); got != first {
		t.Errorf("one look at a different arrangement answered %q, want the standing "+
			"screen %q. A hover highlight is not a navigation", got, first)
	}
	if got := look(t, &v, rects(0.1, 6)); got != first {
		t.Errorf("the screen came back as %q, want %q", got, first)
	}
	if n := len(v.Transitions()); n != 0 {
		t.Errorf("%d transition(s) were recorded over one flicker: %+v", n, v.Transitions())
	}
	st := v.Stability()
	if st.Unsettled != 1 {
		t.Errorf("%d look(s) differed without becoming a screen, want 1. This number is "+
			"the cost of the damping and it must be reported, not hidden", st.Unsettled)
	}
	// AND THE OPENING LOOKS ARE NOT FLAPS. Before any screen has settled there is no
	// standing claim to differ from, and counting those as changes that failed would
	// charge the stabiliser for the one thing it could not have done differently — and
	// make the damping look worse than it is on every short recording.
	if st.Warming != observe.VisualSettleLooks-1 {
		t.Errorf("%d look(s) were counted as warming up, want %d: the looks before the "+
			"first screen settled", st.Warming, observe.VisualSettleLooks-1)
	}
	// Every look is in exactly one of the five, which is what makes the report
	// arithmetic rather than a selection.
	if n := st.Settled + st.Unsettled + st.Warming + st.Unobserved + st.Promotions; n != st.Looks {
		t.Errorf("%d look(s) accounted for of %d offered. A look in none of the "+
			"categories is a look the report cannot explain", n, st.Looks)
	}
}

// THREE DIFFERENT FLICKERS IN A ROW ARE NOT ONE NEW SCREEN EITHER.
//
// The subtle half of the settle rule: each look of a run has to agree with the arrangement that
// STARTED the run, not merely differ from the standing screen. A rule that counted "looks that
// differed" would promote three unrelated frames of noise into one screen — and on a real
// recording, noise is exactly what arrives in runs.
//
// Deleting the agreement test in the pending branch must fail this.
func TestThreeDifferentFlickersAreNotOneNewScreen(t *testing.T) {
	var v observe.VisualScreens
	var first observe.VisualScreenID
	for range observe.VisualSettleLooks {
		first = look(t, &v, rects(0.1, 6))
	}
	// Three looks, each different from the standing screen AND from each other.
	look(t, &v, rects(0.8, 2))
	look(t, &v, rects(0.5, 9))
	got := look(t, &v, rects(0.3, 4))
	if got != first {
		t.Errorf("three unrelated looks promoted something: answered %q, want the "+
			"standing screen %q", got, first)
	}
	if n := len(v.Transitions()); n != 0 {
		t.Errorf("%d transition(s) from three unrelated looks: %+v", n, v.Transitions())
	}
}

// A change that HOLDS is a screen, and coming back is the same screen.
//
// Identity reuse is what makes a route possible at all: walking back out of a submenu must
// arrive at the screen it left, or every demonstration is a chain of new places and no
// repetition is visible. On the measured recording this is what turned 150 raw changes into
// seven transitions that each happened exactly twice.
//
// Deleting the reuse search in promote must fail this.
func TestAChangeThatHoldsIsAScreenAndComingBackIsTheSameOne(t *testing.T) {
	var v observe.VisualScreens
	a := rects(0.1, 6)
	b := rects(0.8, 3)

	var first, second observe.VisualScreenID
	for range observe.VisualSettleLooks {
		first = look(t, &v, a)
	}
	// SETTLING ON THE FIRST SCREEN IS NOT A TRANSITION. There is nothing it came from,
	// and a pair with an invented origin is the manufactured edge the blank rule refuses.
	if _, ok := v.Promoted(); ok {
		t.Error("settling on the session's first screen reported a change; it has no " +
			"origin to have come from")
	}
	for range observe.VisualSettleLooks {
		second = look(t, &v, b)
	}
	// A CHANGE BETWEEN TWO SCREENS IS REPORTED, which is what a press is correlated
	// against: the pair, not the id, because a correlation needs both ends.
	got, ok := v.Promoted()
	if !ok {
		t.Fatal("a settled change between two screens was not reported as a promotion, " +
			"so nothing could correlate a press with it")
	}
	if got.From != first || got.To != second {
		t.Errorf("the reported change is %s -> %s, want %s -> %s",
			got.From, got.To, first, second)
	}
	// AND IT IS NOT STILL REPORTED ON THE NEXT LOOK. A correlation that read a stale
	// promotion would attribute one change to every press that followed it.
	look(t, &v, b)
	if _, ok := v.Promoted(); ok {
		t.Error("the promotion was still reported a look later, so one change could be " +
			"attributed to several presses")
	}
	if second == first {
		t.Fatalf("two different arrangements settled on one screen %q", first)
	}
	var back observe.VisualScreenID
	for range observe.VisualSettleLooks {
		back = look(t, &v, a)
	}
	if back != first {
		t.Errorf("returning to the first arrangement answered %q, want %q. A route needs "+
			"the screen it left to be the screen it comes back to", back, first)
	}
	seen := v.Seen()
	if n := len(seen); n != 2 {
		t.Fatalf("%d screen(s) settled over two arrangements seen three times: %+v",
			n, seen)
	}
	// AND EACH SCREEN KNOWS HOW MUCH OF THE SESSION IT ACCOUNTED FOR. A screen that
	// settled once and a screen that held for four hundred looks are different claims,
	// and a listing that cannot tell them apart cannot say which one was the menu.
	for _, sc := range seen {
		if sc.Looks == 0 {
			t.Errorf("screen %q accounted for no looks at all: %+v", sc.ID, seen)
		}
		if sc.First == 0 {
			t.Errorf("screen %q records no look it settled at", sc.ID)
		}
	}
	// The transitions are both directions, each once, and each knows when it happened.
	tr := v.Transitions()
	if len(tr) != 2 {
		t.Fatalf("%d distinct transition(s), want 2 (there and back): %+v", len(tr), tr)
	}
	for _, x := range tr {
		if x.Count != 1 {
			t.Errorf("%s -> %s happened %d time(s), want 1", x.From, x.To, x.Count)
		}
		if len(x.Looks) != 1 {
			t.Errorf("%s -> %s recorded %d occurrence(s); a repeated route is read "+
				"from WHEN each happened, and a count alone cannot say whether "+
				"it was twice or one flap counted twice", x.From, x.To, len(x.Looks))
		}
	}
}

// A BLANK LOOK IS A MISSING OBSERVATION, NOT A SCREEN.
//
// The owner's rule, and the measured reason for it:
//
//	I would treat them as missing observations, not new screens. A temporary failure to
//	detect regions doesn't mean the application navigated to an empty screen.
//
// The raw diagnostic made its own screen out of two frames holding zero regions, because two
// empty signatures are a perfect match to `signatureSimilarity` — correct for a sparse game
// screen described by roles, wrong for "the proposer found nothing". A transition INTO an empty
// place and out again, twice per animation.
//
// Deleting the blank branch must fail this.
func TestABlankLookIsAMissingObservationAndNotAScreen(t *testing.T) {
	var v observe.VisualScreens
	var first observe.VisualScreenID
	for range observe.VisualSettleLooks {
		first = look(t, &v, rects(0.1, 6))
	}
	// Several blank looks, fewer than the bound.
	for range observe.VisualUnobservableLooks - 1 {
		if got := look(t, &v, nil); got != observe.VisualScreenUnobserved {
			t.Fatalf("a look with no structure answered %q, want %q",
				got, observe.VisualScreenUnobserved)
		}
	}
	// The standing claim survived, and nothing was invented.
	if got := look(t, &v, rects(0.1, 6)); got != first {
		t.Errorf("after a run of blank looks the screen is %q, want the standing %q",
			got, first)
	}
	if n := len(v.Seen()); n != 1 {
		t.Errorf("%d screen(s) settled; a blank look became one: %+v", n, v.Seen())
	}
	if n := len(v.Transitions()); n != 0 {
		t.Errorf("%d transition(s) across a blank run: %+v. A transition into an empty "+
			"place is a navigation nobody made", n, v.Transitions())
	}
	if st := v.Stability(); st.Unobserved != observe.VisualUnobservableLooks-1 {
		t.Errorf("%d blank look(s) counted, want %d",
			st.Unobserved, observe.VisualUnobservableLooks-1)
	}
	// AND AN ARRANGEMENT WITH NOTHING IN IT IS NOT ALIKE ANYTHING, including another one.
	// This is the one place this departs from the segmenter's own answer, and the reason
	// is that emptiness means two different things in the two layers.
	var empty observe.VisualArrangement
	if sim := observe.ArrangementSimilarity(empty, empty); sim != 0 {
		t.Errorf("two empty arrangements scored %.3f, want 0: the segmenter answers 1 "+
			"for two empty ROLE compositions on purpose, and here emptiness means "+
			"the proposer found nothing", sim)
	}
}

// When the absence persists, the screen becomes unknown — and that is still not a transition.
//
// The other half of the owner's rule:
//
//	If the absence persists, Director can report that the screen is unobservable or
//	unknown. It should not manufacture a transition to an empty Place.
//
// Deleting the unobservable bound must fail this.
func TestWhenTheAbsencePersistsTheScreenBecomesUnknown(t *testing.T) {
	var v observe.VisualScreens
	for range observe.VisualSettleLooks {
		look(t, &v, rects(0.1, 6))
	}
	var last observe.VisualScreenID
	for range observe.VisualUnobservableLooks + 2 {
		last = look(t, &v, nil)
	}
	if last != observe.VisualScreenUnknown {
		t.Errorf("after %d blank looks the screen is %q, want %q. Holding a claim "+
			"through an outage this long is confident staleness",
			observe.VisualUnobservableLooks+2, last, observe.VisualScreenUnknown)
	}
	if got := v.Screen(); got != observe.VisualScreenUnknown {
		t.Errorf("the standing claim is %q, want %q", got, observe.VisualScreenUnknown)
	}
	if st := v.Stability(); st.Unobservable != 1 {
		t.Errorf("%d run(s) of blank looks were long enough to give up the screen, want 1",
			st.Unobservable)
	}
	if n := len(v.Transitions()); n != 0 {
		t.Errorf("%d transition(s) were recorded by giving up: %+v. An unknown "+
			"destination is not a place", n, v.Transitions())
	}
	// AND COMING BACK IS NOT A TRANSITION EITHER. There is no screen to have come FROM,
	// so a pair recorded here would have an invented origin — the same manufactured edge
	// the blank rule exists to refuse, arriving one step later.
	for range observe.VisualSettleLooks {
		look(t, &v, rects(0.1, 6))
	}
	if n := len(v.Transitions()); n != 0 {
		t.Errorf("%d transition(s) after settling again from an unknown screen: %+v",
			n, v.Transitions())
	}
}

// A repeated navigation is counted as repeated, with when each occurrence happened.
//
// The acceptance criterion for the whole layer: two demonstrations of one route must produce
// the same transition twice, far apart, so the repetition is visible to whatever reads it.
func TestARepeatedNavigationIsCountedTwiceWithWhen(t *testing.T) {
	var v observe.VisualScreens
	// Three screens in three different columns of the grid, walked so that the MOST
	// REPEATED transition is not the one that sorts first by name. On the real recording
	// the repeated route was seven pairs among fourteen, and a reader finds it by looking
	// at the top of the list — so the order has to be the evidence and not the alphabet.
	a, b, c := rects(0.1, 6), rects(0.8, 3), rects(0.45, 9)
	settle := func(regions []observe.Region) {
		for range observe.VisualSettleLooks {
			look(t, &v, regions)
		}
	}
	settle(a)
	settle(b) // a -> b, once
	settle(c) // b -> c, once
	settle(b) // c -> b, once
	settle(c) // b -> c, TWICE

	tr := v.Transitions()
	if len(tr) != 3 {
		t.Fatalf("%d distinct transition(s) over that walk, want 3: %+v", len(tr), tr)
	}
	// Most repeated first, which is where a reader looks for the route.
	if tr[0].Count != 2 {
		t.Errorf("the transition at the top of the list happened %d time(s), want 2. "+
			"The listing is sorted by evidence, and a reader looking for the "+
			"repeated navigation reads the top: %+v", tr[0].Count, tr)
	}
	if tr[0].From == "visual_1" {
		t.Errorf("the top of the list is %s -> %s, which is the pair that sorts first "+
			"by NAME rather than the one that happened most", tr[0].From, tr[0].To)
	}
	if len(tr[0].Looks) != 2 || tr[0].Looks[0] >= tr[0].Looks[1] {
		t.Errorf("the occurrences are %v; two demonstrations are two moments in order",
			tr[0].Looks)
	}
	if st := v.Stability(); st.Promotions != 5 {
		t.Errorf("%d promotion(s) over five settles, want 5", st.Promotions)
	}
}

// A SESSION-LOCAL VISUAL SCREEN MUST NOT REACH DURABLE MEMORY.
//
// # The decision this holds
//
// A `ScreenStateID` is session-local too, but a screen STATE is a composition of roles a
// provider vouched for. A visual screen is a count of rectangles in nine boxes. That is a far
// weaker thing to make permanent, and the owner was explicit:
//
//	Raw rectangle arrangements should not automatically become durable Places. […] Do not
//	promote geometry-only screen hypotheses into durable cross-session Places yet.
//
// Type safety cannot hold this — Go converts between string types on request — so the guarantee
// is that the names do not APPEAR anywhere inside `internal/` but in the file that defines them.
// An offline replay under `cmd/` may use them freely; the live path may not, and the day that
// changes it changes with somebody deleting a line of this test on purpose.
func TestAVisualScreenCannotReachDurableMemory(t *testing.T) {
	// THE SESSION-LOCAL TIER, which is these files and no others.
	//
	// It has grown twice, both times on purpose and both times with this test failing
	// first — which is the point of it.
	//
	//  1. clicktarget.go, because a click transition's source and destination ARE visual
	//     screens and a target identity is the same kind of claim: session-local,
	//     geometric, and not something that may outlive the session that saw it.
	//     [[ADR-172-a-click-transition-is-a-source-a-target-and-a-destination]]
	//  2. `internal/director/fastobserve`, the fast local observer, because it is the
	//     thing that FOLDS these looks live. It is a whole package rather than a file,
	//     which is a bigger step than the first, so it carries its own boundary tests:
	//     an import walk that fails if it can reach the durable store, the execution
	//     pipeline or anything that can touch a desktop, and a source check that it
	//     names no place establisher. [[ADR-178-the-fast-observer]]
	//  3. reacquire.go, because finding a demonstrated target again is the same claim one
	//     step further on: a rectangle on a screen identified by an arrangement, matched
	//     to a rectangle found on a later look. It produces a `matched` verdict, which is
	//     the most promotable-looking thing this tier has ever held — and so the one most
	//     worth holding here. `Reacquisition.Actionable` says COULD, the actuation
	//     boundary is untouched, and nothing in it can reach a store.
	//     [[ADR-180-finding-a-demonstrated-target-again]]
	//  4. `internal/director/workflow`, the learned draft, because a workflow is a
	//     sequence of these screens and these targets and it is the most
	//     promotable-looking thing the tier has produced — a saved list of actions with
	//     the evidence to resolve each one. It carries its own boundary tests, including
	//     an import walk that fails if it can reach the lowering path: a draft that
	//     could perform itself would be an autonomous actuator with a reading
	//     interface. [[ADR-181-a-demonstration-marco-can-keep]]
	//
	//  5. `internal/director/workflowstore`, which WRITES one to a file. The hardest
	//     addition to this list and the one the question above was written for: it
	//     is literally durable, so “can what it does with these names reach the
	//     store” has to be answered rather than assumed.
	//
	//     It can not, and the artifact is the reason. A saved workflow holds no
	//     session identity at all — its screens and controls are labels scoped to
	//     the artifact, asserted against the encoded bytes and inside its own prose
	//     — so nothing it writes could be mistaken for one of these. It names the
	//     types only to READ a draft and to hand one back, and what it hands back
	//     cannot plan: the labels match no screen any session mints. It carries its
	//     own import walk, which fails if it can reach the action graph, semantic
	//     memory, the lowering path or anything that touches a desktop.
	//     [[ADR-183-remembering-recognising-and-being-allowed-to-act]]
	//
	//  6. `internal/director/workflowrun`, which PERFORMS one. The first thing on this
	//     list that can act, which makes it the first one where the question is not
	//     "could this become durable" but "could this become durable BECAUSE it
	//     acted" — a run that promoted the screen it had just clicked on would be
	//     geometry earning permanence by being used.
	//
	//     It can not, and two things are the reason. It writes nothing: there is no
	//     store, no graph and no file in its import closure, and its own walk fails
	//     if it can reach `actiongraph`, `semanticmemory`, a host, the driver, the
	//     runtime or screen capture. And it holds no identity of its own — every
	//     screen it names arrived in a `Sighting` from the caller doing the looking,
	//     and the only thing it does with one is hand it to the establishment path
	//     and compare it with what came back. The one thing it produces is a point
	//     in DESKTOP coordinates, which names no screen at all.
	//
	//     Its route outward is a single `*marcoexec.Executor`, so what it can do with
	//     one of these names is press somewhere and then ask somebody else what the
	//     screen became. [[ADR-188-performing-a-saved-workflow]]
	//
	// A widening of the TIER is not a widening of what may become durable, and the
	// difference is the whole reason this list is worth keeping. The next addition should
	// be as hard: if a file outside this list needs these names, the question to answer
	// first is whether what it does with them can reach the store.
	dir := filepath.Join("internal", "director", "observe")
	fast := filepath.Join("internal", "director", "fastobserve")
	flow := filepath.Join("internal", "director", "workflow")
	store := filepath.Join("internal", "director", "workflowstore")
	run := filepath.Join("internal", "director", "workflowrun")
	allowed := map[string]bool{
		filepath.Join(dir, "arrangement.go"):          true,
		filepath.Join(dir, "arrangement_test.go"):     true,
		filepath.Join(dir, "candidate.go"):            true,
		filepath.Join(dir, "screentext.go"):           true,
		filepath.Join(dir, "screentext_test.go"):      true,
		filepath.Join(dir, "screentextfold.go"):       true,
		filepath.Join(dir, "screentextfold_test.go"):  true,
		filepath.Join(dir, "screenreading.go"):        true,
		filepath.Join(dir, "screenreading_test.go"):   true,
		filepath.Join(dir, "clicktarget.go"):          true,
		filepath.Join(dir, "clicktarget_test.go"):     true,
		filepath.Join(dir, "cause_test.go"):           true,
		filepath.Join(dir, "reacquire.go"):            true,
		filepath.Join(dir, "reacquire_test.go"):       true,
		filepath.Join(fast, "fastobserve.go"):         true,
		filepath.Join(fast, "fastobserve_test.go"):    true,
		filepath.Join(fast, "slow_test.go"):           true,
		filepath.Join(fast, "boundary_test.go"):       true,
		filepath.Join(fast, "reading_test.go"):        true,
		filepath.Join(flow, "workflow.go"):            true,
		filepath.Join(flow, "workflow_test.go"):       true,
		filepath.Join(flow, "boundary_test.go"):       true,
		filepath.Join(store, "workflowstore.go"):      true,
		filepath.Join(store, "clarify.go"):            true,
		filepath.Join(store, "clarify_test.go"):       true,
		filepath.Join(store, "anchor.go"):             true,
		filepath.Join(store, "stepplan.go"):           true,
		filepath.Join(store, "arrival.go"):            true,
		filepath.Join(store, "arrival_test.go"):       true,
		filepath.Join(store, "stepplan_test.go"):      true,
		filepath.Join(store, "anchor_test.go"):        true,
		filepath.Join(store, "workflowstore_test.go"): true,
		filepath.Join(store, "screentext_test.go"):    true,
		filepath.Join(run, "workflowrun.go"):          true,
		filepath.Join(run, "workflowrun_test.go"):     true,
	}
	root := filepath.Join("..", "..", "..")
	names := []string{
		"VisualScreenID", "VisualArrangement", "ArrangeLook", "VisualScreens",
		// The click tier's own names, held to the same place for the same reason: a
		// target identified by a rectangle on a screen identified by an arrangement
		// is two weak identities stacked, and the durable store takes neither.
		"VisualTargetID", "ClickTargets", "ClickObservation", "ClickTransition",
	}
	var found []string
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		if allowed[rel] {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for _, n := range names {
			if strings.Contains(string(b), n) {
				found = append(found, rel+" names "+n)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking internal: %v", err)
	}
	for _, f := range found {
		t.Errorf("%s\n\ta session-local visual screen has reached the rest of the "+
			"engine. It is a count of rectangles in nine boxes, and the durable "+
			"store takes a composition of roles a provider vouched for. Promoting "+
			"one into the other is a decision somebody takes on purpose, by "+
			"deleting this check and saying why", f)
	}
}

// The durable signature still refuses what this layer accepts.
//
// The companion to the walk above, asserted on behaviour rather than on source: the same
// unnamed regions that make a perfectly good arrangement make an EMPTY screen signature, which
// is what keeps them out of a StructureSignature and out of the store. If this ever starts
// passing regions through, the walk above is no longer the only thing holding the line.
func TestTheDurableSignatureStillRefusesUnnamedGeometry(t *testing.T) {
	regions := rects(0.1, 6)
	arr := observe.ArrangeLook(regions)
	if arr.Total != 6 {
		t.Fatalf("an arrangement of six rectangles has total %d", arr.Total)
	}
	shadow := make([]observe.ShadowRegion, 0, len(regions))
	for _, r := range regions {
		shadow = append(shadow, observe.ShadowRegion{Region: r})
	}
	if sig := observe.NewScreenSignature(shadow); sig.Total != 0 {
		t.Errorf("the durable signature counted %d unnamed region(s). The arrangement "+
			"layer exists because it counts zero, and a change here moves what may "+
			"become a permanent Place", sig.Total)
	}
}

// An arrangement is made of rectangles that have an area, and of where they are.
//
// A region with no area sits nowhere. Counting it would place it at the origin and pile every
// unmeasured proposal into one cell, which is the same reason NewScreenSignature places only
// what has a rectangle — and on a real frame the proposer reports some.
//
// Deleting the area guard in ArrangeLook must fail this.
func TestAnArrangementIsMadeOfRectanglesThatHaveAnArea(t *testing.T) {
	with := observe.ArrangeLook([]observe.Region{
		{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.05},
		{X: 0.8, Y: 0.8, Width: 0.1, Height: 0.05},
		// No area: a proposal nothing measured.
		{X: 0.5, Y: 0.5},
	})
	if with.Total != 2 {
		t.Errorf("an arrangement of two rectangles and one area-less proposal has total "+
			"%d, want 2", with.Total)
	}
	if n := len(with.Cells); n != 2 {
		t.Errorf("it occupies %d cell(s), want 2: %v", n, with.Cells)
	}
	// AND WHERE THEY ARE IS WHAT DISTINGUISHES TWO LOOKS. Same count, different places.
	a := observe.ArrangeLook([]observe.Region{{X: 0.05, Y: 0.05, Width: 0.1, Height: 0.05}})
	b := observe.ArrangeLook([]observe.Region{{X: 0.85, Y: 0.85, Width: 0.1, Height: 0.05}})
	if sim := observe.ArrangementSimilarity(a, b); sim != 0 {
		t.Errorf("one rectangle top-left and one bottom-right scored %.3f, want 0: an "+
			"arrangement that ignored position would be a count of proposals", sim)
	}
	// An arrangement with nothing in it is not observed, which is what keeps a blank
	// frame out of the screen model entirely.
	if observe.ArrangeLook(nil).Observed() {
		t.Error("an arrangement built from no regions reports itself as observed")
	}
}

// Shape ignores how MANY rectangles there were; count does not.
//
// The owner's question about the proposer's varying yield:
//
//	I'd test whether arrangement similarity remains stable when the detector produces
//	different numbers of regions on otherwise similar frames. But I wouldn't normalize away
//	every count difference. Sometimes a genuine menu transition really does add or remove
//	controls.
//
// Both halves are asserted here, because a normalisation that merged everything would answer
// the first question by destroying the information the second one is about.
//
// Measured on the real recording the answer was NO for the pair it was asked about — `visual_1`
// at 236 regions and `visual_6` at 400 differ by shape too — and normalising would have merged
// `visual_3` with `visual_4`, which are two consecutive steps of the demonstrated route. So it
// stays a measurement. See [[ADR-173-the-proposers-yield-is-not-what-separates-two-screens]].
//
// Deleting the division by Total must fail this.
func TestShapeIgnoresHowManyRectanglesThereWere(t *testing.T) {
	// THE YIELD VARIES AND THE DISTRIBUTION DOES NOT, which is the only fixture that
	// isolates the question. Rectangles clustered in two cells of the grid, in equal
	// shares, found sparsely and then densely.
	//
	// A column of rows does NOT isolate it: three rows and six rows of one column occupy
	// different numbers of grid cells, so the distribution moves too and shape comes out
	// lower than count rather than higher. That mistake is why this comment is here.
	sparse := observe.ArrangeLook(append(cluster(2, 0.15, 0.15), cluster(2, 0.5, 0.5)...))
	dense := observe.ArrangeLook(append(cluster(6, 0.15, 0.15), cluster(6, 0.5, 0.5)...))
	if sparse.Total == dense.Total {
		t.Fatal("the fixture does not vary the yield")
	}
	if len(sparse.Cells) != len(dense.Cells) {
		t.Fatalf("the fixture varies the distribution too: %v against %v",
			sparse.Cells, dense.Cells)
	}
	byShape := observe.ArrangementShapeSimilarity(sparse, dense)
	byCount := observe.ArrangementSimilarity(sparse, dense)
	if byShape <= byCount {
		t.Errorf("shape %.3f is not above count %.3f for one column found at two "+
			"yields; normalising is supposed to make exactly this case alike",
			byShape, byCount)
	}
	// AND A GENUINE CHANGE OF LAYOUT IS STILL A CHANGE. The distribution moved, so no
	// amount of normalising makes these one screen — which is the half that keeps the
	// measurement worth taking.
	elsewhere := observe.ArrangeLook(append(cluster(6, 0.85, 0.85), cluster(6, 0.5, 0.15)...))
	if sim := observe.ArrangementShapeSimilarity(dense, elsewhere); sim >= observe.VisualScreenMatch {
		t.Errorf("a left-hand column and a right-hand column scored %.3f by shape, "+
			"against a threshold of %.2f. A normalisation that merged those would "+
			"answer the yield question by destroying the route",
			sim, observe.VisualScreenMatch)
	}
	// An unobserved arrangement is alike nothing, by shape as by count.
	var empty observe.VisualArrangement
	if sim := observe.ArrangementShapeSimilarity(empty, empty); sim != 0 {
		t.Errorf("two empty arrangements scored %.3f by shape, want 0", sim)
	}
	if sim := observe.ArrangementShapeSimilarity(dense, empty); sim != 0 {
		t.Errorf("an arrangement and nothing scored %.3f by shape, want 0", sim)
	}
}

// A settled screen's arrangement can be read back, which is what the comparison above needs.
func TestASettledScreensArrangementCanBeReadBack(t *testing.T) {
	var v observe.VisualScreens
	var id observe.VisualScreenID
	for range observe.VisualSettleLooks {
		id = look(t, &v, rects(0.1, 6))
	}
	got := v.Arrangement(id)
	if got.Total != 6 {
		t.Errorf("the settled screen's arrangement reports %d region(s), want 6. A "+
			"measurement that compares two screens needs what each one IS",
			got.Total)
	}
	if n := v.Arrangement("visual_nonexistent").Total; n != 0 {
		t.Errorf("an unknown screen reports an arrangement of %d region(s)", n)
	}
}
