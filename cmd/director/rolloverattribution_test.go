package main

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
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
