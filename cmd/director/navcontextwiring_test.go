package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The navigation producer is told what this cycle saw — through the production path.
//
// Both pushes survived a deliberate deletion before this existed. That is the failure this
// repository keeps rediscovering: complete, correct code that nothing calls is indistinguishable
// from code that is not there, and only a test entering through the production method can tell
// the difference.
//
// What is asserted is the OBSERVABLE consequence rather than the call: a pointer press that lands
// inside the pinned window becomes placed navigation afterwards, and does not before.

func samplerWithNav(t *testing.T) (*liveSampler, func(x, y int32), *navsource.Subscription) {
	t.Helper()
	src, press := navsource.NewSyntheticPointer()
	t.Cleanup(func() { src.Close() })
	rt := &Runtime{navSource: src}
	sub := src.Open(time.Now())
	t.Cleanup(func() { src.CloseSession(sub) })
	return &liveSampler{rt: rt}, press, sub
}

func placedAfter(t *testing.T, src *navsource.Source, sub *navsource.Subscription,
	press func(x, y int32), x, y int32) bool {

	t.Helper()
	before := src.Stats().Received
	press(x, y)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && src.Stats().Received == before {
		time.Sleep(100 * time.Microsecond)
	}
	return len(sub.Drain()) > 0
}

// A valid inference tells the producer where the window is, so a click can be placed.
func TestAValidInferenceGivesThePointerAFrame(t *testing.T) {
	s, press, sub := samplerWithNav(t)
	src := s.rt.navSource

	// Before any inference the producer has no frame, so a press is unplaceable.
	if placedAfter(t, src, sub, press, 300, 350) {
		t.Fatal("a press was placed before any inference said where the window was")
	}

	s.pushNavContext(&observe.ShadowSample{Ran: true, TargetProven: true},
		directorapi.Rect{X: 100, Y: 200, Width: 400, Height: 300}, time.Now())

	if !placedAfter(t, src, sub, press, 300, 350) {
		t.Fatal("after a valid inference a click inside the pinned window was still not " +
			"placed; the bounds are computed and never handed to the producer")
	}
	// And a press outside that window is still refused, so the frame is the pinned one
	// rather than anything permissive.
	if placedAfter(t, src, sub, press, 5000, 5000) {
		t.Error("a press far outside the pinned window was placed")
	}
}

// An inference that did not happen says nothing, and unknown is not false.
//
// A skipped slot or an unproven target must leave the previous frame standing rather than
// clearing it: "the detector sat this one out" is not "the window went away".
func TestASkippedInferenceDoesNotClearTheFrame(t *testing.T) {
	s, press, sub := samplerWithNav(t)
	src := s.rt.navSource

	s.pushNavContext(&observe.ShadowSample{Ran: true, TargetProven: true},
		directorapi.Rect{X: 100, Y: 200, Width: 400, Height: 300}, time.Now())
	for _, sh := range []*observe.ShadowSample{
		nil,
		{Ran: false, TargetProven: true},
		{Ran: true, TargetProven: false},
		{Ran: true, TargetProven: true, Unavailable: "the detector could not run"},
	} {
		s.pushNavContext(sh, directorapi.Rect{}, time.Now())
	}

	if !placedAfter(t, src, sub, press, 300, 350) {
		t.Error("a cycle Marco could not observe cleared the frame; a press inside the " +
			"window it had just confirmed became unplaceable")
	}
}

// A valid inference offers the window's controls to the producer, so a click can be
// RESOLVED as well as placed — and the admitted-label gates are applied here, at the push.
func TestAValidInferenceOffersActionablesToTheProducer(t *testing.T) {
	s, press, sub := samplerWithNav(t)
	src := s.rt.navSource

	world := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{
		"e1": {Role: directorapi.RoleButton, Label: "Save", Visible: true, Confidence: 0.9,
			Bounds: directorapi.Rect{X: 150, Y: 250, Width: 200, Height: 40}},
		// A list item: clickable, NOT on the plaintext allowlist. Its label crosses only
		// under the demonstration licence.
		"e2": {Role: directorapi.RoleListItem, Label: "Mouse", Visible: true, Confidence: 0.9,
			Bounds: directorapi.Rect{X: 150, Y: 300, Width: 200, Height: 40}},
	}}
	sh := &observe.ShadowSample{Ran: true, TargetProven: true}
	frame := directorapi.Rect{X: 100, Y: 200, Width: 400, Height: 300}

	// Passive: the button's name crosses, the list item's does not.
	s.pushNavContext(sh, frame, time.Now())
	s.pushActionables(world, time.Now())
	press(200, 320) // inside the list item
	waitClassified(t, src, 1)
	got := sub.Drain()
	if len(got) != 1 || got[0].Target == nil {
		t.Fatalf("the press did not resolve: %+v", got)
	}
	if got[0].Target.Label != "" || got[0].Target.Role != string(directorapi.RoleListItem) {
		t.Fatalf("passive observation admitted %+v; a list item's text is very often a "+
			"fact about the person", got[0].Target)
	}

	// Under the demonstration licence, the one control the person aims at may carry its
	// name — shape-filtered exactly as every admitted label is.
	s.nameActivatedTargets = true
	s.pushActionables(world, time.Now())
	press(200, 320)
	waitClassified(t, src, 2)
	got = sub.Drain()
	if len(got) != 1 || got[0].Target == nil || got[0].Target.Label != "Mouse" {
		t.Fatalf("under the Learn licence the activated control's name did not cross: %+v",
			got)
	}
}

// waitClassified waits until the producer has classified n events.
func waitClassified(t *testing.T, src *navsource.Source, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && src.Stats().Classified < n {
		time.Sleep(100 * time.Microsecond)
	}
	if src.Stats().Classified < n {
		t.Fatalf("the worker classified %d event(s), want %d", src.Stats().Classified, n)
	}
}

// A DEFAULT Director — no structural detector, which is the shipped configuration — still
// gives the pointer a frame.
//
// The regression this holds closed was live-only and total: `pushNavContext` was gated on the
// shadow sample having run, vision is off by default, so the gate returned on every cycle and
// the producer never received a window frame. Every mouse click on every application was
// refused as `unplaceable_pointer` — counted, and gone. A live Learn against Windows Settings
// reported `received=2 classified=0 unplaceable_pointer=2` for the two clicks the person made,
// and the whole click-resolution path above it had therefore never once run.
//
// The unit tests all passed, because every one of them supplied `{Ran: true, TargetProven:
// true}` — the shape production does not have.
func TestADefaultDirectorGivesThePointerAFrame(t *testing.T) {
	s, press, sub := samplerWithNav(t)
	src := s.rt.navSource
	frame := directorapi.Rect{X: 100, Y: 200, Width: 400, Height: 300}

	// Exactly what production hands this on a Director with no detector configured: a
	// shadow record that exists only to carry input stats, and never ran.
	for _, sh := range []*observe.ShadowSample{
		nil,
		{Detector: "", Ran: false},
	} {
		s.pushNavContext(sh, frame, time.Now())
	}
	if !placedAfter(t, src, sub, press, 300, 350) {
		t.Fatal("a default Director never told the producer where the window is, so a click " +
			"inside it is unplaceable. Every mouse-driven demonstration is lost this way, " +
			"and the only symptom is a counter.")
	}

	// And the controls come with it, so the click resolves rather than merely landing.
	world := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{
		"e1": {Role: directorapi.RoleButton, Label: "Save", Visible: true, Confidence: 0.9,
			Bounds: directorapi.Rect{X: 150, Y: 250, Width: 200, Height: 40}},
	}}
	s.pushActionables(world, time.Now())
	press(200, 270)
	waitClassified(t, src, 2)
	got := sub.Drain()
	if len(got) != 1 || got[0].Target == nil || got[0].Target.Label != "Save" {
		t.Fatalf("a default Director resolved the click to %+v; the actionable index is "+
			"gated on the experiment too", got)
	}
}

// The composition root tells the producer WHOSE surface is in front, on every cycle.
//
// # Why this needs a wiring test of its own
//
// The rule that operating Marco is not demonstrating to it is enforced in the producer, where the
// press is classified — and it is enforced against context that something else has to push. The
// producer cannot ask: answering "whose window is in front" is a syscall, and a low-level hook
// callback that makes one is a hook Windows drops.
//
// So the guarantee has two halves in two packages, and the half that is easy to lose is this one.
// A pushed fact that nobody pushes is indistinguishable from a fact that is always false: every
// press would be admitted, the boundary tests in navsource would still pass, and the only symptom
// would be a demonstration containing a click on Marco's own Start button.
//
// This is the same shape as the frame gate above, which was a real defect for a milestone.
//
// Deleting the SetSurfaceOwner call from pushNavContext must fail this.
func TestMarcoOwnedInputNeverBecomesDemonstrationEvidence(t *testing.T) {
	s, press, sub := samplerWithNav(t)
	src := s.rt.navSource
	frame := directorapi.Rect{X: 100, Y: 200, Width: 400, Height: 300}

	// MARCO in front. The runtime has no desktop, so the ownership answer it derives is
	// "not Marco" — which is the honest default and no use for this test. What is under test
	// is that the composition root pushes an answer AT ALL, so the owner is set directly and
	// the assertion is that the push carries it.
	s.rt.owner.adopt(marcoOwnedHandle)
	s.rt.winPlatform = &fakeDesktop{front: windowref.Candidate{
		ID: "window_9", Handle: marcoOwnedHandle, Application: "chrome",
		Foreground: true, Visible: true, OnScreen: true,
		Bounds: directorapi.Rect{Width: 1200, Height: 800},
	}}
	s.rt.winDirectory = windowref.NewDirectory()

	s.pushNavContext(nil, frame, time.Now())

	// A press INSIDE the watched window, which is what makes this about ownership rather
	// than about geometry: Marco's overlay lies over the application it is watching.
	press(300, 350)
	waitReceived(t, src, 1)

	if got := sub.Drain(); len(got) != 0 {
		t.Fatalf("a press made while Marco's own surface was in front became %d "+
			"demonstration event(s): %+v.\nThe person was operating Marco — pressing "+
			"Start, or Stop, or clicking it to see what it thinks — and the "+
			"demonstration now contains an action they never performed on the "+
			"application.", len(got), got)
	}
	if n := src.Stats().Ignored[observe.IgnoreMarcoOwned]; n != 1 {
		t.Fatalf("marco_owned = %d, want 1.\nall reasons: %v\nThe producer was never told "+
			"whose surface was in front, so it had nothing to refuse the press on.",
			n, src.Stats().Ignored)
	}
}

// marcoOwnedHandle is the window a control surface was adopted from, in these tests.
const marcoOwnedHandle uintptr = 0x9999

// waitReceived waits until the producer's worker has handled n events.
func waitReceived(t *testing.T, src *navsource.Source, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && src.Stats().Received < n {
		time.Sleep(100 * time.Microsecond)
	}
	if got := src.Stats().Received; got < n {
		t.Fatalf("the producer handled %d event(s), waited for %d", got, n)
	}
}

// WHICH CONTROLS ARE OFFERED DOES NOT DEPEND ON MAP ORDER.
//
// # The defect, measured
//
// A fused world is a MAP, and `pushActionables` truncated at MaxActionables. So on a screen
// offering more clickable controls than the bound, WHICH of them reached the navigation producer
// depended on Go's map iteration — which Go deliberately randomises per range.
//
// Two readings of one unchanged screen offered different sets. The set is what a human click is
// attributed against, so the same press could resolve to a Target on one reading and to nothing on
// the next — and from outside, "Marco didn't see what you clicked" is indistinguishable from a
// perception failure.
//
// A bound is fine. Which half of a screen it keeps must not be a coin toss.
//
// Deleting the sort must fail this.
func TestWhichControlsAreOfferedDoesNotDependOnMapOrder(t *testing.T) {
	s, _, _ := samplerWithNav(t)

	// MORE CLICKABLE CONTROLS THAN THE BOUND, which is the only shape where truncation can
	// choose. A long list, a busy web page, a file view: ordinary screens reach this.
	world := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	for i := 0; i < MaxActionables*2; i++ {
		id := directorapi.ElementID(fmt.Sprintf("e%04d", i))
		world.Elements[id] = &directorapi.Element{
			ID: id, Role: directorapi.RoleButton, Label: fmt.Sprintf("Item %d", i),
			Visible: true, Confidence: 0.9,
			Bounds: directorapi.Rect{
				X: 10, Y: 20 * i, Width: 100, Height: 18,
			},
		}
	}

	offered := func() []navsource.Actionable {
		s.pushActionables(world, time.Now())
		return s.rt.navSource.Actionables()
	}
	first := offered()
	if len(first) != MaxActionables {
		t.Fatalf("%d actionable(s) offered, want the bound of %d — the fixture does not "+
			"reach truncation and so proves nothing", len(first), MaxActionables)
	}
	for run := 0; run < 20; run++ {
		again := offered()
		if len(again) != len(first) {
			t.Fatalf("run %d offered %d, the first offered %d",
				run, len(again), len(first))
		}
		for i := range again {
			if again[i] != first[i] {
				t.Fatalf("run %d offered a different set: item %d is %+v, was %+v. "+
					"Which controls a press can resolve against must not be a "+
					"coin toss.", run, i, again[i], first[i])
			}
		}
	}
	// AND IT KEEPS READING ORDER — top to bottom — which is the half that says the bound
	// keeps something a person would recognise rather than an arbitrary sample.
	for i := 1; i < len(first); i++ {
		if first[i].Y < first[i-1].Y {
			t.Fatalf("item %d is above item %d: %+v then %+v",
				i, i-1, first[i-1], first[i])
		}
	}

	// AND CONTROLS THAT TIE ON EVERYTHING STILL ORDER, which is what makes the comparison a
	// total order rather than nearly one. Two controls really can share a rectangle, a role
	// and a label — a tab strip with repeated glyphs, a grid of identical cells — and a
	// comparison that called them equal would leave the sort free to swap them.
	tied := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	for i := 0; i < MaxActionables*2; i++ {
		id := directorapi.ElementID(fmt.Sprintf("t%04d", i))
		tied.Elements[id] = &directorapi.Element{
			ID: id, Role: directorapi.RoleButton, Label: "Same",
			Visible: true, Confidence: 0.9,
			Bounds: directorapi.Rect{X: 10, Y: 20, Width: 100, Height: 18},
		}
	}
	s.pushActionables(tied, time.Now())
	settled := s.rt.navSource.Actionables()
	if len(settled) != MaxActionables {
		t.Fatalf("%d tied actionable(s) offered, want the bound", len(settled))
	}
	for run := 0; run < 20; run++ {
		s.pushActionables(tied, time.Now())
		again := s.rt.navSource.Actionables()
		for i := range again {
			if again[i] != settled[i] {
				t.Fatalf("run %d chose a different one of %d identical controls at "+
					"position %d. Two controls can share a rectangle, a role and "+
					"a label; the order still has to be total.",
					run, len(again), i)
			}
		}
	}
}
