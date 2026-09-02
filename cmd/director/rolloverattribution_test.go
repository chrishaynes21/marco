package main

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// A TWENTY-SECOND BOOKKEEPING BOUNDARY IS NOT A USER EVENT.
//
// # The measured loss
//
// Normal dogfood, one person walking Windows Settings:
//
//	Watching: 4 screens, 8 moves noticed
//	Learning: 3 relationships seen, 3 remembered
//
// Five of eight crossings were seen and produced no edge. The ledger held no record of the
// arrival at Bluetooth & devices at all — not promoted, not refused, not waiting — so the person
// showed Marco a real connection four times over and Marco kept three of them.
//
// The mechanism: an action is filed against the session-local screen state it was performed on,
// the destination resolves on the NEXT reading, and if an ambient session rolls over in between,
// `drain` clears the pending map because the new session's state numbering restarts. The crossing
// then arrives with no action and `noticed` drops it at `len(s.Did) != 1`.
//
// The comment guarding that clear is right and stays:
//
//	"placing it against whatever the next session happens to see first is exactly the guess
//	 this file exists to refuse"
//
// What was wrong is treating A SESSION BOUNDARY as equivalent to EVIDENCE BECAME UNRESOLVABLE.
// They are not the same thing, and the same physical interaction must produce the same graph fact
// whether it happens a second before a rollover or a second after.

// walk drives one reading exactly as `sample` does: drain, attribute, record.
//
// Through the production path rather than around it. The defect lives in the seam between those
// three, and a test that called them in a different order would be testing a different program.
func walk(a *ambientObserver, look ambientLook, at time.Time) {
	a.attribute(a.drain(look))
	a.record(look.Application, look, at)
}

// onScreen is one reading: where the person is, and any input banked since the last one.
func onScreen(session, state string, shape *ambient.Shape,
	inputs ...observe.AttributedInput) ambientLook {

	return ambientLook{
		OK: true, Application: recentApp,
		Session: observe.SessionID(session), State: observe.ScreenStateID(state),
		Shape: shape, Place: observe.Place{Placed: true, Reach: observe.ReachContent},
		Inputs: inputs,
	}
}

// pressed is one attributed human press on a named control, banked against a screen state.
func pressed(state, control string, atMS int64) observe.AttributedInput {
	return observe.AttributedInput{
		State: observe.ScreenStateID(state),
		Event: observe.InputEvent{
			Intent: observe.NavPoint, AtMS: atMS,
			Target: &observe.SemanticTarget{Role: "list_item", Label: control},
		},
	}
}

// edgesAfter is every crossing the buffer holds that carried ANY action.
//
// Any, not exactly one. Filtering to one-action steps was the first version and it hid the
// failure it was meant to catch: a carry that wrongly brought two presses across produces a
// two-action step, which `noticed` refuses but which a test looking only for singletons cannot
// see. A test that cannot see the bad state it forbids is not a test of it.
func edgesAfter(a *ambientObserver) []ambient.Step {
	var out []ambient.Step
	for _, s := range a.buf.Look().Recent {
		if len(s.Did) > 0 {
			out = append(out, s)
		}
	}
	return out
}

// THE CONTROL: no rollover, and the edge is captured.
//
// The only variable between this and the test below is the session boundary. If this one ever
// fails the fixture is wrong, not the production code.
func TestAnAttributedCrossingBecomesAnEdge(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	rt := &Runtime{observations: g}
	a := rt.ambient()
	at := time.Now()

	walk(a, onScreen("observe_1", "state_1", homeShape()), at)
	walk(a, onScreen("observe_1", "state_1", homeShape(),
		pressed("state_1", "Bluetooth & devices", 100)), at.Add(time.Second))
	walk(a, onScreen("observe_1", "state_2", btShape()), at.Add(2*time.Second))

	got := edgesAfter(a)
	if len(got) != 1 {
		t.Fatalf("%d attributed crossing(s) with no rollover involved, want 1. The fixture "+
			"is wrong, not the code.", len(got))
	}
	if got[0].Did[0].Target.Label != "Bluetooth & devices" {
		t.Errorf("the crossing carries %q", got[0].Did[0].Target.Label)
	}
}

// AND THE SAME INTERACTION ACROSS A SESSION BOUNDARY.
//
// The person pressed one control, on a screen Marco had placed, and the destination resolved on
// the very next reading. Nothing about that is ambiguous. The only thing that happened in between
// is that a twenty-second counter rolled over.
//
// Deleting the carry must fail this.
func TestARolloverBetweenAPressAndItsDestinationKeepsTheEdge(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	rt := &Runtime{observations: g}
	a := rt.ambient()
	at := time.Now()

	walk(a, onScreen("observe_1", "state_1", homeShape()), at)
	walk(a, onScreen("observe_1", "state_1", homeShape(),
		pressed("state_1", "Bluetooth & devices", 100)), at.Add(time.Second))
	// THE BOUNDARY. A new session, its counter restarted, and the destination arriving on
	// the first reading of it.
	walk(a, onScreen("observe_2", "state_1", btShape()), at.Add(2*time.Second))

	got := edgesAfter(a)
	if len(got) != 1 {
		t.Fatalf("%d attributed crossing(s) across a session boundary, want 1.\n\n"+
			"The person pressed one control on a placed screen and the destination "+
			"resolved on the next reading. A twenty-second bookkeeping counter rolled "+
			"over in between, and the evidence was destroyed — measured live as five of "+
			"eight crossings lost, including every arrival at Bluetooth & devices.",
			len(got))
	}
	if got[0].Did[0].Target.Label != "Bluetooth & devices" {
		t.Errorf("the crossing carries %q", got[0].Did[0].Target.Label)
	}
}

// ── and every way the carry must refuse ───────────────────────────────────────

// THE FIRST THING IN THE NEXT SESSION IS NOT AUTOMATICALLY THE DESTINATION.
//
// The property that matters most. A naive carry becomes "an action is pending, a rollover
// happened, whatever appears next is where it went" — exactly the guess the drain comment refuses,
// reintroduced by the thing meant to preserve it.
//
// # What holds it, measured by mutation
//
// Not the carry. `record` builds no step at all unless `previousApp == application`, so a crossing
// between two applications never reaches the carry to ask it anything. A duplicate check inside
// `claimCarried` was written first and survived its own mutation — an unreachable guard, which
// this repository treats as a claim nothing can test — so it was removed and this test now names
// the line that actually holds the property.
//
// Neither guard alone can be deleted to fail this: `record` refuses to build the step, and
// `carryAcross` refuses to carry across applications. Both were mutated and both survived, which
// is what double defence looks like from a mutation gate. The property is what matters and it is
// held twice; this test exists so a change that removed BOTH would be caught.
func TestTheCarryRefusesADifferentApplication(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	a := (&Runtime{observations: g}).ambient()
	at := time.Now()

	walk(a, onScreen("observe_1", "state_1", homeShape()), at)
	walk(a, onScreen("observe_1", "state_1", homeShape(),
		pressed("state_1", "Bluetooth & devices", 100)), at.Add(time.Second))

	elsewhere := onScreen("observe_2", "state_1", btShape())
	elsewhere.Application = "somethingelse"
	walk(a, elsewhere, at.Add(2*time.Second))

	if got := edgesAfter(a); len(got) != 0 {
		t.Fatalf("a press in one application was attributed to a crossing in another: %+v. "+
			"A rollover is not permission to attach an action to whatever appears next.",
			got[0].Did)
	}
}

// AND IT REFUSES A STALE ONE.
//
// A page that has not arrived in five seconds did not arrive because of that press. Attributing a
// crossing to it would be recording a coincidence as a fact about the interface.
//
// Deleting the window must fail this.
func TestTheCarryRefusesAStalePress(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	a := (&Runtime{observations: g}).ambient()
	clock := newDryClock()
	restore := sessionClock
	sessionClock = clock
	t.Cleanup(func() { sessionClock = restore })
	at := time.Now()

	walk(a, onScreen("observe_1", "state_1", homeShape()), at)
	walk(a, onScreen("observe_1", "state_1", homeShape(),
		pressed("state_1", "Bluetooth & devices", 100)), at.Add(time.Second))
	// The clock moves well past the window before the destination shows up.
	<-clock.After(carryWindow + time.Second)
	walk(a, onScreen("observe_2", "state_1", btShape()), at.Add(2*time.Second))

	if got := edgesAfter(a); len(got) != 0 {
		t.Fatalf("a press older than the carry window was attributed to a later crossing: "+
			"%+v", got[0].Did)
	}
}

// AND A SECOND BOUNDARY ENDS IT.
//
// One rollover is a bookkeeping accident. Two is a person who has been somewhere else for twenty
// seconds, and an action that has waited that long is no longer about anything.
//
// # What holds it, measured by mutation
//
// `carryAcross` clears before it considers carrying again, so the second rollover destroys an
// unclaimed carry on its way past. A session field compared inside `claimCarried` was written
// first and survived its own mutation for the same reason the application check did: nothing could
// reach it. Removed, and the property is now held in one place.
//
// Deleting the clear at the top of carryAcross must fail this.
func TestTheCarryDoesNotSurviveASecondBoundary(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	a := (&Runtime{observations: g}).ambient()
	at := time.Now()

	walk(a, onScreen("observe_1", "state_1", homeShape()), at)
	walk(a, onScreen("observe_1", "state_1", homeShape(),
		pressed("state_1", "Bluetooth & devices", 100)), at.Add(time.Second))
	// Two more sessions, the screen never resolving to anywhere new in the first.
	walk(a, onScreen("observe_2", "state_1", homeShape()), at.Add(2*time.Second))
	walk(a, onScreen("observe_3", "state_1", btShape()), at.Add(3*time.Second))

	if got := edgesAfter(a); len(got) != 0 {
		t.Fatalf("an action carried across two boundaries was still attributed: %+v",
			got[0].Did)
	}
}

// AND A COMPETING ACTION SUPERSEDES IT.
//
// Two presses with no resolved destination between them are two things somebody did. A crossing
// carries exactly one, and keeping the older would attribute this arrival to the wrong press —
// which is worse than losing it, because it is a plausible-looking lie.
//
// Deleting the supersede must fail this.
func TestACompetingActionSupersedesTheCarry(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	a := (&Runtime{observations: g}).ambient()
	at := time.Now()

	walk(a, onScreen("observe_1", "state_1", homeShape()), at)
	walk(a, onScreen("observe_1", "state_1", homeShape(),
		pressed("state_1", "Bluetooth & devices", 100)), at.Add(time.Second))
	// A rollover, and then a SECOND press before anywhere new resolves.
	walk(a, onScreen("observe_2", "state_1", homeShape(),
		pressed("state_1", "System", 200)), at.Add(2*time.Second))
	walk(a, onScreen("observe_2", "state_2", btShape()), at.Add(3*time.Second))

	got := edgesAfter(a)
	if len(got) != 1 {
		t.Fatalf("%d crossing(s), want the one the second press explains", len(got))
	}
	if got[0].Did[0].Target.Label != "System" {
		t.Errorf("the crossing was attributed to %q; the press that preceded it was %q",
			got[0].Did[0].Target.Label, "System")
	}
}

// AND A DEGRADED READING BREAKS IT.
//
// A reading that got no further than the window frame is a gap nobody read. An action waiting for
// its destination cannot be said to have crossed something Marco could not see.
//
// Deleting the clear on the degraded branch must fail this.
func TestADegradedReadingBreaksTheCarry(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	a := (&Runtime{observations: g}).ambient()
	at := time.Now()

	walk(a, onScreen("observe_1", "state_1", homeShape()), at)
	walk(a, onScreen("observe_1", "state_1", homeShape(),
		pressed("state_1", "Bluetooth & devices", 100)), at.Add(time.Second))

	degraded := onScreen("observe_2", "state_1", nil)
	degraded.Place = observe.Place{Placed: true, Reach: observe.ReachShell}
	walk(a, degraded, at.Add(2*time.Second))
	walk(a, onScreen("observe_2", "state_2", btShape()), at.Add(3*time.Second))

	if got := edgesAfter(a); len(got) != 0 {
		t.Fatalf("an action was carried across a reading nobody could see: %+v", got[0].Did)
	}
}

// AND TWO UNRESOLVED PRESSES ARE NOT CARRIED AT ALL.
//
// The ambiguity is the reason to refuse, not a detail to resolve: a crossing may carry exactly
// one action, and guessing which of two caused it would invent a relationship nobody observed.
//
// Deleting the single-action requirement in carryAcross must fail this.
func TestTwoUnresolvedPressesAreNotCarried(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	a := (&Runtime{observations: g}).ambient()
	at := time.Now()

	walk(a, onScreen("observe_1", "state_1", homeShape()), at)
	walk(a, onScreen("observe_1", "state_1", homeShape(),
		pressed("state_1", "Bluetooth & devices", 100),
		pressed("state_1", "System", 900)), at.Add(time.Second))
	walk(a, onScreen("observe_2", "state_1", btShape()), at.Add(2*time.Second))

	if got := edgesAfter(a); len(got) != 0 {
		t.Fatalf("two unresolved presses produced a crossing carrying %+v", got[0].Did)
	}
}

// AND STOPPING CLEARS IT.
//
// Somebody who stopped watching did not leave a press behind for the next session to finish.
func TestStoppingClearsAPendingCarry(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	rt := &Runtime{observations: g}
	a := rt.ambient()
	at := time.Now()

	walk(a, onScreen("observe_1", "state_1", homeShape()), at)
	walk(a, onScreen("observe_1", "state_1", homeShape(),
		pressed("state_1", "Bluetooth & devices", 100)), at.Add(time.Second))
	// Force the carry, then stop.
	a.mu.Lock()
	a.carryAcross(onScreen("observe_2", "state_1", btShape()))
	a.mu.Unlock()
	if a.carried == nil {
		t.Fatal("the fixture carried nothing, so this proves nothing")
	}
	rt.DisableAmbient()
	if a.carried != nil {
		t.Error("an action was still waiting to cross a boundary after watching stopped")
	}
}

// AND A BOUNDARY IS INVISIBLE TO THE GRAPH, WHEREVER IT FALLS.
//
// The architectural acceptance. The same physical interaction — one screen, one press, the next
// screen — must produce the same graph fact whether a twenty-second counter happens to roll over
// before the press, between the press and the arrival, or not at all.
//
// Any offset producing a different answer must fail this.
func TestASessionBoundaryIsInvisibleToTheGraph(t *testing.T) {
	learnedIn(t)
	for name, sessions := range map[string][3]string{
		"no rollover":      {"observe_1", "observe_1", "observe_1"},
		"before the press": {"observe_1", "observe_2", "observe_2"},
		"between the two":  {"observe_1", "observe_1", "observe_2"},
	} {
		t.Run(name, func(t *testing.T) {
			g, _ := watchedRegistry(t)
			a := (&Runtime{observations: g}).ambient()
			at := time.Now()
			walk(a, onScreen(sessions[0], "state_1", homeShape()), at)
			walk(a, onScreen(sessions[1], "state_1", homeShape(),
				pressed("state_1", "Bluetooth & devices", 100)), at.Add(time.Second))
			walk(a, onScreen(sessions[2], "state_2", btShape()), at.Add(2*time.Second))

			got := edgesAfter(a)
			if len(got) != 1 {
				t.Fatalf("%d crossing(s) with the boundary %s, want 1. The same "+
					"interaction must produce the same graph fact wherever an "+
					"internal counter happens to roll over.", len(got), name)
			}
			if got[0].Did[0].Target.Label != "Bluetooth & devices" {
				t.Errorf("the crossing carries %q", got[0].Did[0].Target.Label)
			}
		})
	}
}

// ── a crossing that spans two interactions is not one edge ────────────────────

// AN ACTION IS NOT CREDITED WITH AN ARRIVAL IT DID NOT CAUSE.
//
// # The false edge this closes, measured live
//
// One person walked `Home → Bluetooth & devices → Mouse → System → Home` at normal speed. Marco
// learned:
//
//	Home      --press "Bluetooth & devices"--> Bluetooth & devices     correct
//	Bluetooth --press "Mouse"-->               Mouse                   correct
//	Mouse     --press "System"-->              Home                    FALSE
//
// System appears in no relationship at all. The System page did not settle into a Place before the
// person pressed Home, so `record` bridged over it — which is right for a loading frame — and the
// crossing that eventually landed on Home took the pending "System" press with it. Marco came away
// believing that pressing System on the Mouse page takes you Home.
//
// A missing edge is a disappointment. This is a confident falsehood about somebody's computer, and
// it composes: the map then offers it as part of "the way home".
//
// # The discriminator, and why it is not "refuse all bridging"
//
// Bridging is legitimate. A page transition shows a frame nothing can place, and the crossing
// either side of it is one honest interaction. What makes THIS one different is that a SECOND
// action was taken while Marco could not see where it was — so the journey was two interactions
// and the graph may claim neither.
//
// That is visible without guessing: an action filed against a state other than the one being left,
// still unconsumed at the moment the crossing is recorded, is an interaction that happened in
// between. The crossing is still recorded as movement; it simply carries no action, so `noticed`
// declines to make an edge of it.
func TestAnActionIsNotCreditedWithAnArrivalItDidNotCause(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	a := (&Runtime{observations: g}).ambient()
	at := time.Now()

	// On Mouse, and the person presses System.
	walk(a, onScreen("observe_1", "state_1", homeShape()), at)
	walk(a, onScreen("observe_1", "state_1", homeShape(),
		pressed("state_1", "System", 100)), at.Add(time.Second))

	// The System page arrives and does not settle into anything Marco can place — and the
	// person presses Home while it is up. That press is filed against the state Marco could
	// not place, which is why it is invisible to the crossing that follows.
	// The input log is CUMULATIVE — it is the session's whole stream, and the observer
	// tracks an absolute cursor into it. A reading that carried only the newest event would
	// be a fixture the production drain never sees.
	bridge := onScreen("observe_1", "state_2", nil,
		pressed("state_1", "System", 100), pressed("state_2", "Home", 900))
	walk(a, bridge, at.Add(2*time.Second))

	// And then Home settles.
	walk(a, onScreen("observe_1", "state_3", btShape(),
		pressed("state_1", "System", 100), pressed("state_2", "Home", 900)),
		at.Add(3*time.Second))

	for _, s := range edgesAfter(a) {
		t.Fatalf("a crossing was credited to %q after a second press happened in between, "+
			"where Marco could not see. The journey was two interactions and the graph "+
			"may claim neither: this is how `pressing System takes you Home` became a "+
			"durable fact about Windows Settings.", s.Did[0].Target.Label)
	}
}

// AND ORDINARY BRIDGING STILL PRODUCES ITS EDGE.
//
// The control. A page transition shows a frame nothing can place, nobody touches anything while it
// is up, and the crossing either side is one honest interaction. Refusing every bridged crossing
// would lose real edges on any application that renders slowly, which is most of them.
//
// Refusing this must fail.
func TestBridgingAFrameNobodyTouchedStillLearnsTheEdge(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	a := (&Runtime{observations: g}).ambient()
	at := time.Now()

	walk(a, onScreen("observe_1", "state_1", homeShape()), at)
	walk(a, onScreen("observe_1", "state_1", homeShape(),
		pressed("state_1", "Mouse", 100)), at.Add(time.Second))
	// A frame nothing can place, and nobody presses anything while it is up.
	walk(a, onScreen("observe_1", "state_2", nil), at.Add(2*time.Second))
	walk(a, onScreen("observe_1", "state_3", btShape()), at.Add(3*time.Second))

	got := edgesAfter(a)
	if len(got) != 1 {
		t.Fatalf("%d crossing(s) across a frame nobody touched, want 1. Bridging exists so "+
			"a walk survives a loading screen, and refusing it would lose a real edge on "+
			"every application that renders slowly.", len(got))
	}
	if got[0].Did[0].Target.Label != "Mouse" {
		t.Errorf("the crossing carries %q", got[0].Did[0].Target.Label)
	}
}

// ── ingress: sample because something happened ────────────────────────────────

// A PRESS CUTS THE WAIT SHORT.
//
// The gap between readings is attention — a second on a desktop somebody is using, eight while
// nothing changes. A person clicking through an application is the opposite of a desktop sitting
// still, and waiting out an interval chosen for stillness is how a four-screen walk at normal
// speed produced three edges with the middle screen never settling at all.
//
// The wait already wakes every hundred milliseconds to notice a window switch. This asks one more
// question in the same loop, at the cost of comparing two integers.
//
// Deleting the wake must fail this.
func TestAPressCutsTheWaitShort(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	a := rt.ambient()
	// The foreground is the real desktop's, which is not this fixture's application — so the
	// window-switch arm would fire before the input arm was ever reached.
	restore := winctxActive
	winctxActive = func() string { return "testgame" }
	t.Cleanup(func() { winctxActive = restore })

	// A REAL SESSION, so the input log this reads is the one production reads.
	watchNow(t, g, store, "testgame")
	_ = store

	// Nothing has happened, so the wait runs its course rather than returning early.
	started := time.Now()
	left, ok := a.waitOrLeave(t.Context(), 300*time.Millisecond, "testgame")
	if left || !ok {
		t.Fatalf("waiting reported left=%v ok=%v on a quiet desktop", left, ok)
	}
	if waited := time.Since(started); waited < 250*time.Millisecond {
		t.Errorf("a quiet desktop cut the wait short after %v; idle cost must not change",
			waited)
	}

	// AND NOW SOMETHING HAS. A real session with a real press in its log, and an observer
	// whose cursor has not reached it — which is exactly the state a person clicking leaves.
	_ = g.Cancel(g.ActiveID())
	for deadline := time.Now().Add(settleDeadline); time.Now().Before(deadline) && g.ActiveID() != ""; {
		time.Sleep(10 * time.Millisecond)
	}
	watchPressing(t, g, "testgame")
	a.mu.Lock()
	a.session, a.cursor = "", 0
	a.mu.Unlock()
	started = time.Now()
	if _, ok := a.waitOrLeave(t.Context(), 5*time.Second, "testgame"); !ok {
		t.Fatal("the wait was cancelled")
	}
	if waited := time.Since(started); waited > time.Second {
		t.Errorf("a press waited %v to be looked at. The whole point is to sample because "+
			"something happened rather than on a timer.", waited)
	}
}

// AND ASKING DOES NOT CONSUME ANYTHING.
//
// Attribution happens exactly once, in `drain`, from the cursor it has always used. A wake signal
// that ate an event would turn a press into a reading nobody could explain — the same class of
// loss ADR-119 was about, arriving from the opposite direction.
//
// Making the wake consume must fail this.
func TestTheWakeSignalConsumesNothing(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	a := rt.ambient()
	watchNow(t, g, store, "testgame")

	before := lastShadow(t, g).InputLog
	for range 5 {
		a.somethingHappened("testgame")
	}
	after := lastShadow(t, g).InputLog
	if after.Dropped != before.Dropped || len(after.Events) != len(before.Events) {
		t.Errorf("asking whether something happened changed the input log: %d/%d -> %d/%d",
			before.Dropped, len(before.Events), after.Dropped, len(after.Events))
	}
}

// watchPressing leaves a real session running whose script contains a human press, so the
// session's own input log is non-empty — which is the state a person clicking leaves behind.
func watchPressing(t *testing.T, g *observationRegistry, application string) {
	t.Helper()
	bounds := dryBounds()
	bounds.Duration = time.Minute
	script := append(dryHold("a", 2), dryPress("a", observe.NavConfirm))
	script = append(script, dryHold("a", 200)...)
	id, err := g.Start(namedTarget{app: application}, &drySampler{script: script},
		observesession.NopEvents{}, windowref.Selector{Application: application}, bounds)
	if err != nil {
		t.Fatalf("starting a session over %s: %v", application, err)
	}
	t.Cleanup(func() { _ = g.Cancel(id) })
	for deadline := time.Now().Add(settleDeadline); time.Now().Before(deadline); {
		if len(lastShadow(t, g).InputLog.Events) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the fixture's session never banked an input, so there is no press to wake on")
}
