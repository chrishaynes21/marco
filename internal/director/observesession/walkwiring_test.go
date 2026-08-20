package observesession_test

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// A demonstration with a MIDDLE STEP, entered through the production session path.
//
// # The failure this exists for
//
// A person demonstrated `Settings Home → Bluetooth & devices → Mouse` on a cold store. It was
// captured, attributed and semantically resolved end to end, all three screens settled, all three
// were recognisable — and the pass refused with `destination_not_recognised`, having learned
// nothing at all.
//
// Two independent causes, and the first hid the second:
//
//  1. Only the place the pass ENDED on was made durable, so the middle place did not exist and
//     the edges either side of it had an endpoint nothing could resolve.
//     [[ADR-063-a-pass-remembers-every-place-it-settled-on]].
//  2. Every real navigation crosses a frame Marco cannot place, so the walk is
//     `A → ? → B → ? → C`. The transition map is keyed by the pair, so that aggregates to two
//     entries into the unplaceable state and two exits out of it, and which entry belonged with
//     which exit was not recoverable. Bridging refused `ambiguous_interval` and BOTH adjacencies
//     were lost. [[ADR-064-the-order-of-a-walk-is-evidence]].
//
// Both were invisible to every fixture in the suite, because a scripted screen that RECURS is
// never separated from its neighbour by a frame nobody could place, and a pass that ends where it
// started never needs a middle place.
//
// # What the fixture is
//
// One walk, no cycle: A settles, the user acts, a transition frame, B settles, the user acts,
// another transition frame, C settles, and the pass ends standing on C. That is the shape of
// every real two-step demonstration and it is the shape no other fixture here has.

// walkSampler scripts a person walking A → B → C, crossing one unreadable frame per step.
//
// A transition frame is not invented for the test: it is a SPARSE composition, a subset of a
// screen Marco already knows, seen once. The segmenter calls that unplaceable by its own rule
// (see the StateContainment branch), which is exactly what a half-rendered page produces live.
type walkSampler struct {
	calls int
}

// walkPhases is one sample per entry; the pass is a single walk rather than a cycle.
//
// The tail of quiet samples is not padding. A licensed pass ends when the person has STOPPED,
// which the runner reads from consecutive input-free inferences — so a fixture that stopped the
// moment C appeared would be scripting a demonstration nobody had finished.
var walkPhases = []struct {
	screen  rune // 'a', 'b', 'c', or '-' for a transition frame
	partial rune // which screen the transition frame is a fragment of
	act     bool // the user did something on this sample
}{
	{screen: 'a'},
	{screen: 'a'},
	{screen: '-', partial: 'a', act: true},
	{screen: 'b'},
	{screen: 'b'},
	{screen: '-', partial: 'b', act: true},
	{screen: 'c'},
	{screen: 'c'},
	{screen: 'c'},
	{screen: 'c'},
	{screen: 'c'},
	{screen: 'c'},
}

// walkScreens places each screen's controls somewhere different on the window.
//
// Different PLACES matter: a structural group is made of tracks persistent in exactly one state,
// so two screens whose controls occupied the same rectangles would share their tracks and neither
// would have a group — leaving both unrecognisable and the test proving nothing.
func walkScreen(which rune) (float64, float64, []observe.InterfaceTerm) {
	switch which {
	case 'a':
		return 0.414, 0.06, []observe.InterfaceTerm{observe.TermSettings, observe.TermControls}
	case 'b':
		return 0.414, 0.70, []observe.InterfaceTerm{observe.TermAudio, observe.TermDisplay}
	default:
		return 0.700, 0.36, []observe.InterfaceTerm{observe.TermSocial, observe.TermInvite}
	}
}

func (s *walkSampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	s.calls++
	// After the scripted walk the person is still standing on C, doing nothing.
	i := s.calls - 1
	if i >= len(walkPhases) {
		i = len(walkPhases) - 1
	}
	p := walkPhases[i]

	hud := observe.ShadowRegion{
		Role: "icon", Confidence: 0.5,
		Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}
	sh := &observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true, LatencyMS: 860,
	}
	regions := []observe.ShadowRegion{hud}

	if p.screen == '-' {
		// A REMNANT of the screen being left: one control, and not even the HUD. Everything
		// in it is accounted for by a state Marco knows and there is much less of it, which
		// is the segmenter's own definition of a partial rendering — held rather than judged,
		// and reported unknown on a first sighting.
		//
		// The HUD is left out deliberately. With it, the two transition frames share a
		// region and score alike enough to be ONE recurring composition, which promotes the
		// second of them into a state and quietly removes the case this fixture is for.
		x, y, _ := walkScreen(p.partial)
		regions = screenRegions(x, y)[:1]
	} else {
		x, y, terms := walkScreen(p.screen)
		regions = append(regions, screenRegions(x, y)...)
		sh.Semantic = observe.SemanticEvidence{Terms: terms, Observed: true}
	}
	if p.act {
		sh.Inputs = []observe.InputEvent{{
			Intent: observe.NavConfirm, AtMS: int64(s.calls) * 100,
		}}
	}

	sh.Regions = regions
	sh.Detections = len(regions)
	sh.Roles = map[string]int{}
	for _, r := range regions {
		sh.Roles[r.Role]++
		if r.Nameable {
			sh.Nameable++
		}
	}
	return observe.Sample{
		WindowGeneration: 1,
		Frame:            observe.FrameSummary{Application: "testgame", Width: 1920, Height: 1080},
		Shadow:           sh,
	}, nil
}

// ── the proof ─────────────────────────────────────────────────────────────────

// A two-step demonstration produces TWO durable edges, through the real session.
//
// Mutations that must fail this: narrowing establishment back to the current state; making the
// bridge refuse when a walk holds more than one unplaceable interval; dropping the crossing log.
func TestATwoStepWalkLeavesTwoDurableEdges(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	got := establishing(t, store, store, &walkSampler{})

	// The premise, checked first so a failure below cannot be blamed on the fixture.
	shadow := got.Stats.Shadow
	settled := 0
	for _, st := range shadow.States {
		if st.Settled {
			settled++
		}
	}
	if settled < 3 {
		t.Fatalf("only %d of %d state(s) settled; the fixture does not present three "+
			"screens and nothing below is about the code under test.\nstates: %+v",
			settled, len(shadow.States), shadow.States)
	}
	if len(shadow.Crossings) == 0 {
		t.Fatal("the session recorded no walk, so the pairing rule was never reached")
	}

	// BOTH edges. This is the whole point: A → B → C decomposes into two reusable pieces of
	// knowledge, which is what makes a goal reachable by a route nobody demonstrated whole.
	if got.Relationships.Durable < 2 {
		t.Fatalf("%d durable edge(s) from a two-step walk, want 2.\n"+
			"session-local: %d\ncauses: %v\nbridge refusals: %v\ncrossings: %+v\n"+
			"A demonstration that was captured, attributed and recognised end to end is "+
			"still worth nothing if its adjacencies cannot be recovered.",
			got.Relationships.Durable, got.Relationships.SessionLocal,
			got.Relationships.SessionLocalCauses, got.Relationships.BridgeRefusals,
			shadow.Crossings)
	}
	// Both were recovered ACROSS an unplaceable frame, which is the case that was failing.
	if got.Relationships.Bridged < 2 {
		t.Errorf("bridged=%d, want 2; the edges were expected to be recovered across the "+
			"transition frames, so this fixture is no longer exercising that path",
			got.Relationships.Bridged)
	}

	// And every place on the route is durable, including the one in the middle.
	if n := len(screenSubjects(memoryAt(t, dir))); n < 3 {
		t.Errorf("the store holds %d screen subject(s) after a three-screen walk, want 3", n)
	}
}

// The middle screen is not skipped: the recovered edges run A → B and B → C, never A → C.
//
// The strong half of the same guarantee. Recovering the adjacency is worth nothing if the walk
// can be read as a shortcut, because Marco would then believe in a route the person never took.
func TestAWalkThroughAMiddleScreenIsNotShortened(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	got := establishing(t, store, store, &walkSampler{})

	edges := memoryAt(t, dir).Relationships()
	if len(edges) < 2 {
		t.Fatalf("the store holds %d edge(s) after a two-step walk: %+v", len(edges), edges)
	}
	// The middle place is an endpoint of one edge and the start of the other, so it appears
	// as both a source and a destination. A shortened walk has no such subject.
	sources, destinations := map[string]bool{}, map[string]bool{}
	for _, e := range edges {
		sources[e.From] = true
		destinations[e.To] = true
	}
	middle := ""
	for id := range sources {
		if destinations[id] {
			middle = id
		}
	}
	if middle == "" {
		t.Fatalf("no subject is both a destination and a source, so the walk was read as "+
			"two unrelated changes rather than a route through a middle screen.\n%+v", edges)
	}
	if middle == got.Places.Subject {
		t.Errorf("the middle of the route is the place the pass ended on (%s), which cannot "+
			"be right for a walk that finished somewhere else", middle)
	}
}
