package main

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Turning a hook's account of a desktop into what a person did.
//
// The three problems these are about: one press arriving as a burst of events, the same intention
// arriving through two different modalities, and an event drained a second after the screen it
// was performed on stopped being the screen in front. See observeaction.go and ADR-094.

// banked is one admitted input event, as the session runner would have banked it.
func banked(intent observe.NavIntent, at int64, state string,
	target *observe.SemanticTarget) observe.AttributedInput {

	return observe.AttributedInput{
		Event: observe.InputEvent{Intent: intent, AtMS: at, Target: target},
		State: observe.ScreenStateID(state),
	}
}

func control(role, label string) *observe.SemanticTarget {
	return &observe.SemanticTarget{Role: role, Label: label}
}

// ONE NOISY PRESS IS ONE ACTION.
//
// A single click arrives as a burst: the pointer goes down, the control takes focus, and the
// application reports it invoked. Recording three actions for one press would produce a play that
// presses the same button three times.
func TestOneNoisyPressIsOneAction(t *testing.T) {
	acts := coalesce([]observe.AttributedInput{
		banked(observe.NavPoint, 1000, "state_1", control("button", "Mouse")),
		banked(observe.NavConfirm, 1080, "state_1", control("button", "Mouse")),
		banked(observe.NavPoint, 1150, "state_1", control("button", "Mouse")),
	})
	if len(acts) != 1 {
		t.Fatalf("%d action(s) from one press, want 1: %+v", len(acts), acts)
	}
	if acts[0].act.Kind != ambient.Activate || acts[0].act.Target.Label != "Mouse" {
		t.Errorf("the surviving action is %+v", acts[0].act)
	}
}

// TWO QUICK PRESSES ON TWO CONTROLS STAY TWO ACTIONS.
//
// The other half of the same rule, and the one a wider coalescing window would break. Opening a
// menu and choosing an item inside it is two presses a fraction of a second apart on two
// different controls — collapsing them would lose the item, which is the whole demonstration.
func TestTwoQuickPressesOnTwoControlsStayTwoActions(t *testing.T) {
	acts := coalesce([]observe.AttributedInput{
		banked(observe.NavPoint, 1000, "state_1", control("menu", "View")),
		banked(observe.NavPoint, 1010, "state_1", control("menuitem", "Zoom")),
	})
	if len(acts) != 2 {
		t.Fatalf("%d action(s) from two presses, want 2: %+v", len(acts), acts)
	}
	if acts[0].act.Target.Label != "View" || acts[1].act.Target.Label != "Zoom" {
		t.Errorf("the two actions are %+v and %+v", acts[0].act, acts[1].act)
	}
}

// AND TWO PRESSES ON THE SAME CONTROL, FAR APART, STAY TWO.
//
// Somebody pressing a stepper twice deliberately did two things. The window is what separates
// that from the burst one press produces, and it is the only thing that does.
func TestTwoDeliberatePressesOnOneControlStayTwo(t *testing.T) {
	acts := coalesce([]observe.AttributedInput{
		banked(observe.NavPoint, 1000, "state_1", control("button", "Louder")),
		banked(observe.NavPoint, 1000+2*coalesceWindow.Milliseconds(), "state_1",
			control("button", "Louder")),
	})
	if len(acts) != 2 {
		t.Fatalf("%d action(s), want 2: two presses %v apart were read as one",
			len(acts), 2*coalesceWindow)
	}
}

// MOVING A SELECTION IS NOT DOING SOMETHING.
//
// Arrows move a selection. They are how somebody REACHED the thing they wanted, not what they
// wanted, and a route built from them would depend on where the selection happened to start. The
// confirm that follows carries the control the keyboard had landed on, which is the same semantic
// fact a click on it would have produced.
//
// Hover and focus-change never reach here at all: there is no word for either in the navigation
// vocabulary, so "a hover became an activation" is a sentence this code cannot say.
func TestMovingASelectionIsNotAnAction(t *testing.T) {
	acts := coalesce([]observe.AttributedInput{
		banked(observe.NavDown, 1000, "state_1", nil),
		banked(observe.NavDown, 1100, "state_1", nil),
		banked(observe.NavRight, 1200, "state_1", nil),
	})
	if len(acts) != 0 {
		t.Fatalf("%d action(s) from three selection moves, want 0: %+v", len(acts), acts)
	}

	// And the keyboard demonstration is the same demonstration a click would have made.
	withConfirm := coalesce([]observe.AttributedInput{
		banked(observe.NavDown, 1000, "state_1", nil),
		banked(observe.NavDown, 1100, "state_1", nil),
		banked(observe.NavConfirm, 1200, "state_1", control("button", "Mouse")),
	})
	if len(withConfirm) != 1 {
		t.Fatalf("%d action(s), want 1: %+v", len(withConfirm), withConfirm)
	}
	if withConfirm[0].act != (ambient.Act{Kind: ambient.Activate,
		Target: ambient.Target{Role: "button", Label: "Mouse"}}) {
		t.Errorf("a keyboard activation lowered to %+v; it must be the same act a click "+
			"on the same control produces, because it is the same intention",
			withConfirm[0].act)
	}
}

// AN ACTION BELONGS TO THE SCREEN IT WAS PERFORMED ON.
//
// # The defect this exists to prevent
//
// Events are drained about a second after they happen. Between the press and the drain the
// person's click has usually already changed the screen — so the screen in front when the event
// is finally read is very often the DESTINATION of the action rather than its source.
// Attributing to it produces "on the Bluetooth page, press Bluetooth", which is wrong in a way
// that looks entirely reasonable and would be learned as a play that does nothing.
//
// So every event carries the session-local screen state that was current when the runner banked
// it, and the action is filed against THAT.
//
// Replacing the stamp with the current state must fail this.
func TestAnActionBelongsToTheScreenItWasPerformedOn(t *testing.T) {
	rt := watchingRuntime(t)
	a := rt.ambient()

	// The person is on Home, and Marco has read Home.
	a.mu.Lock()
	a.lastState = "state_home"
	a.mu.Unlock()

	// They press Bluetooth. By the time the event is drained, Marco has already read the
	// NEW screen — so `state_bt` is what the observer is looking at now.
	a.attribute(coalesce([]observe.AttributedInput{
		banked(observe.NavPoint, 1000, "state_home", control("button", "Bluetooth & devices")),
	}))

	if did := a.takePending("state_bt"); len(did) != 0 {
		t.Fatalf("the action was filed against the screen it PRODUCED (%+v). A play built "+
			"from this would say: on the Bluetooth page, press Bluetooth.", did)
	}
	did := a.takePending("state_home")
	if len(did) != 1 || did[0].Target.Label != "Bluetooth & devices" {
		t.Fatalf("the action was not filed against the screen it was performed on: %+v", did)
	}
}

// A NEW SESSION IS A NEW FRAME OF REFERENCE.
//
// Screen state ids are session-local counters. `state_2` in one session and `state_2` in the next
// are unrelated, so carrying a cursor or a state map across the boundary would attribute this
// session's actions to the previous one's screens — a wrong answer that would look right.
func TestNothingCarriesAcrossASessionBoundary(t *testing.T) {
	rt := watchingRuntime(t)
	a := rt.ambient()

	first := ambientLook{OK: true, Session: "observe_1", Application: "settings",
		Inputs: []observe.AttributedInput{
			banked(observe.NavPoint, 100, "state_2", control("button", "Mouse")),
		}}
	a.attribute(a.drain(first))
	if len(a.takePending("state_2")) != 1 {
		t.Fatal("the first session's action was not recorded, so this proves nothing")
	}

	second := ambientLook{OK: true, Session: "observe_2", Application: "settings",
		Inputs: []observe.AttributedInput{
			banked(observe.NavPoint, 100, "state_2", control("button", "Display")),
		}}
	a.attribute(a.drain(second))
	did := a.takePending("state_2")
	if len(did) != 1 {
		t.Fatalf("%d action(s) filed under state_2 after a session change, want 1: the "+
			"previous session's action was carried across a boundary where the same "+
			"counter means something different", len(did))
	}
	if did[0].Target.Label != "Display" {
		t.Errorf("the action filed under state_2 is %+v; the old session's survived", did[0])
	}
}

// A CURSOR THAT IS AN INDEX RE-DELIVERS EVERY EVENT AFTER AN OVERFLOW.
//
// The session's input log is bounded and drops its OLDEST entries. An index into the slice would
// silently hand the same events back at a lower index, and the same demonstration would be
// recorded twice. The log reports what it dropped, so the cursor counts events in the session's
// whole stream and derives the offset.
func TestADroppedInputLogDoesNotRepeatItself(t *testing.T) {
	rt := watchingRuntime(t)
	a := rt.ambient()

	look := ambientLook{OK: true, Session: "observe_1", Application: "settings",
		Inputs: []observe.AttributedInput{
			banked(observe.NavPoint, 100, "state_1", control("button", "One")),
			banked(observe.NavPoint, 200, "state_1", control("button", "Two")),
		}}
	if got := a.drain(look); len(got) != 2 {
		t.Fatalf("%d action(s) on the first drain, want 2", len(got))
	}
	// The log overflowed: the two above are gone, and two new ones are held.
	look.Dropped = 2
	look.Inputs = []observe.AttributedInput{
		banked(observe.NavPoint, 300, "state_1", control("button", "Three")),
		banked(observe.NavPoint, 400, "state_1", control("button", "Four")),
	}
	got := a.drain(look)
	if len(got) != 2 {
		t.Fatalf("%d action(s) after an overflow, want 2: %+v", len(got), got)
	}
	if got[0].act.Target.Label != "Three" || got[1].act.Target.Label != "Four" {
		t.Errorf("the drain re-delivered events the log had already dropped: %+v", got)
	}
}

// ACTIONS SURVIVE WHEN REDUNDANT READINGS DO NOT.
//
// Backpressure, gated. A person on one screen produces a reading a second and the occasional
// press. The readings are the redundant part — a place seen again costs one increment — and the
// press is the part that cannot be reconstructed from anything else. Twenty copies of Home must
// never be what is kept while the click that left it is lost.
func TestTheClickSurvivesTwentyCopiesOfTheSameScreen(t *testing.T) {
	rt := watchingRuntime(t)
	a := rt.ambient()
	at := time.Now()

	for i := 0; i < 20; i++ {
		a.recordPlace("settings", knownPlace("subj_home"), at.Add(time.Duration(i)*time.Second))
	}
	a.attribute(coalesce([]observe.AttributedInput{
		banked(observe.NavPoint, 100, string(stateOfPlace(knownPlace("subj_home"))),
			control("button", "Bluetooth & devices")),
	}))
	for i := 0; i < 20; i++ {
		a.recordPlace("settings", knownPlace("subj_home"), at.Add(time.Duration(20+i)*time.Second))
	}
	a.recordPlace("settings", knownPlace("subj_bt"), at.Add(time.Minute))

	recent := a.buf.Look().Recent
	if len(recent) != 1 {
		t.Fatalf("%d step(s) after forty readings of two screens, want 1: %+v",
			len(recent), recent)
	}
	if len(recent[0].Did) != 1 || recent[0].Did[0].Target.Label != "Bluetooth & devices" {
		t.Fatalf("the press was lost while the redundant readings were kept: %+v", recent[0])
	}
}

// A LOADING SCREEN IS NOT SOMEWHERE YOU WENT.
//
// Somebody presses something and waits through a page part-way through arriving. That frame is
// not a Place: it is not settled, it is still loading, and establishing it would put a spinner in
// the topology as a screen with a way in and a way out.
//
// The gate is asked as a QUESTION of the same function a licensed session uses to decide whether
// a screen may become durable — every one of its refusals is a screen that must not become a
// promotable endpoint either. Here it is driven through the ambient path: a reading that
// describes nothing is crossed rather than recorded, and the walk survives it.
//
// Deleting the gate — describing every unrecognised reading — must fail this.
func TestALoadingScreenIsNotSomewhereYouWent(t *testing.T) {
	rt := watchingRuntime(t)
	a := rt.ambient()
	at := time.Now()

	a.record("settings", ambientLook{OK: true, Application: "settings",
		Place: knownPlace("subj_home"), State: "state_home"}, at)
	a.attribute(coalesce([]observe.AttributedInput{
		banked(observe.NavPoint, 100, "state_home", control("button", "Bluetooth & devices")),
	}))
	// Two readings of something part-way through arriving: describable at all, and refused
	// by the establishability gate, so no shape comes back.
	for i := 1; i <= 2; i++ {
		a.record("settings", ambientLook{OK: true, Application: "settings",
			Place:   observe.Place{Placed: true, Reach: observe.ReachContent},
			State:   observe.ScreenStateID("state_loading"),
			Refusal: observe.PlaceStillLoading,
		}, at.Add(time.Duration(i)*time.Second))
	}
	a.record("settings", ambientLook{OK: true, Application: "settings",
		Place: knownPlace("subj_bt"), State: "state_bt"}, at.Add(4*time.Second))

	recent := a.buf.Look().Recent
	if len(recent) != 1 {
		t.Fatalf("%d step(s), want 1 — a loading frame became somewhere the person went: %+v",
			len(recent), recent)
	}
	s := recent[0]
	if s.From != "subj_home" || s.To != "subj_bt" {
		t.Errorf("the step runs %s -> %s, want subj_home -> subj_bt", s.From, s.To)
	}
	if s.Bridged != 2 {
		t.Errorf("the step crossed %d unreadable screen(s), want 2. A leg assembled across "+
			"frames nobody could place is still that leg, and how much of it was never "+
			"seen has to travel with it.", s.Bridged)
	}
	if len(s.Did) != 1 {
		t.Fatalf("the press did not survive the wait: %+v", s.Did)
	}
}

// A LONG LOAD IS STILL ONE TRANSITION.
//
// The concern that motivated this roadmap's refinement: somebody activates something and then
// waits thirty seconds. Nothing here is a timeout, so the wait costs nothing — the action stays
// filed against the screen it was performed on until that screen is left, however long that
// takes.
//
// Adding a time expiry to pending actions must fail this.
func TestThirtySecondsOfLoadingDoesNotLoseTheAction(t *testing.T) {
	rt := watchingRuntime(t)
	a := rt.ambient()
	at := time.Now()

	a.record("settings", ambientLook{OK: true, Application: "settings",
		Place: knownPlace("subj_home"), State: "state_home"}, at)
	a.attribute(coalesce([]observe.AttributedInput{
		banked(observe.NavPoint, 100, "state_home", control("button", "Reports")),
	}))
	for i := 1; i <= 30; i++ {
		a.record("settings", ambientLook{OK: true, Application: "settings",
			Place:   observe.Place{Placed: true, Reach: observe.ReachContent},
			State:   "state_loading",
			Refusal: observe.PlaceStillLoading,
		}, at.Add(time.Duration(i)*time.Second))
	}
	a.record("settings", ambientLook{OK: true, Application: "settings",
		Place: knownPlace("subj_reports"), State: "state_reports"}, at.Add(31*time.Second))

	recent := a.buf.Look().Recent
	if len(recent) != 1 || len(recent[0].Did) != 1 {
		t.Fatalf("thirty seconds of loading lost the action: %+v", recent)
	}
	if recent[0].Settled < 30 {
		t.Errorf("the step says it settled over %d reading(s); the wait is evidence and has "+
			"to travel with the leg", recent[0].Settled)
	}
}

// WATCHING SEES WHAT YOU PRESSED.
//
// # It enters through `sample`, and that is the whole point of the test
//
// The claim is a WIRING claim: the ambient sample drains the session's input evidence, and does
// it BEFORE recording the new place — because the actions were performed on the screen the person
// was on when they did them, and recording the new place first would move that screen out from
// under the step about to be built from it.
//
// The first version of this called `attribute(drain(...))` itself, so it proved the two functions
// worked and nothing about whether anything called them. Deleting the drain from `sample` left
// every test in this package passing — measured, by mutation. It goes through the production
// entry point now, with the desktop reading replaced at its one seam.
//
// Deleting the drain from `sample`, or moving it after `record`, must fail this.
func TestWatchingSeesWhatYouPressed(t *testing.T) {
	rt := watchingRuntime(t)
	a := rt.ambient()

	looks := []ambientLook{
		{OK: true, Session: "observe_1", Application: "settings",
			Place: knownPlace("subj_home"), State: "state_home"},
		// The press, banked while Home was current, drained on the reading that finds
		// Bluetooth — which is the ordinary case and the one that goes wrong quietly.
		{OK: true, Session: "observe_1", Application: "settings",
			Place: knownPlace("subj_bt"), State: "state_bt",
			Inputs: []observe.AttributedInput{
				banked(observe.NavPoint, 900, "state_home",
					control("button", "Bluetooth & devices")),
			}},
	}
	next := 0
	restore := ambientLookNow
	ambientLookNow = func(*Runtime, string) ambientLook {
		if next >= len(looks) {
			return ambientLook{}
		}
		next++
		return looks[next-1]
	}
	t.Cleanup(func() { ambientLookNow = restore })

	a.sample("settings")
	a.sample("settings")

	recent := a.buf.Look().Recent
	if len(recent) != 1 {
		t.Fatalf("%d step(s), want 1", len(recent))
	}
	if len(recent[0].Did) != 1 || recent[0].Did[0].Target.Label != "Bluetooth & devices" {
		t.Fatalf("watching recorded where you went and not what you pressed: %+v", recent[0])
	}
}

// WHAT MARCO DID IS NOT WHAT YOU DEMONSTRATED.
//
// A play running while watching is on moves the screen too. Recording that as the person's own
// navigation would teach Marco its own behaviour back from itself, and would offer it to them as
// something they had just shown it.
//
// The provenance is read from the ONE slot every actuating entrance funnels through — see
// beginPerformance, which exists because a rehearsal reaches the desktop from inside a Learn
// episode without passing any handler.
//
// Deleting either half must fail this.
func TestWhatMarcoDidIsNotWhatYouDemonstrated(t *testing.T) {
	rt := watchingRuntime(t)
	a := rt.ambient()
	at := time.Now()

	if rt.marcoIsActing() {
		t.Fatal("a Runtime doing nothing says it is driving the desktop")
	}
	a.recordPlace("settings", knownPlace("subj_home"), at)

	// Marco takes the keyboard, through the slot both actuating entrances use.
	_, done, err := rt.beginPerformance(t.Context(), "open mouse settings")
	if err != nil {
		t.Fatalf("claiming the slot: %v", err)
	}
	if !rt.marcoIsActing() {
		t.Fatal("a performance is running and nothing says so, so ambient watching cannot " +
			"tell its own work from the person's")
	}
	a.recordPlace("settings", knownPlace("subj_bt"), at.Add(time.Second))
	done("", 1)

	if rt.marcoIsActing() {
		t.Error("the performance finished and the Director still says it is acting")
	}
	// And back to the person.
	a.recordPlace("settings", knownPlace("subj_mouse"), at.Add(2*time.Second))

	recent := a.buf.Look().Recent
	if len(recent) != 2 {
		t.Fatalf("%d step(s), want 2: %+v", len(recent), recent)
	}
	if recent[0].By != ambient.ByMarco {
		t.Errorf("a transition Marco made is recorded as %q. Offering that back as the "+
			"person's demonstration is how a system learns its own behaviour from itself.",
			recent[0].By)
	}
	if recent[1].By != ambient.ByHuman {
		t.Errorf("the person's own transition is recorded as %q", recent[1].By)
	}
}
