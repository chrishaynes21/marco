package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// Reality may diverge from the graph.
//
// # What these drive
//
// `carryOn` is the whole recovery decision, and it is reachable: it takes the topology, the
// attempt's own memory and the view the walk has already written, and returns either an alternate
// route or a refusal. `PerformGoal` itself cannot be entered from a test — it goes through
// `winctx` to bring a window forward — so the tests that need a REAL failure drive `performPlan`
// against the stalling desktop and hand its view to `carryOn`, which is the same composition
// `PerformGoal` performs.
//
// See [[ADR-099-a-failed-attempt-is-not-a-false-edge]].

// failedStep is a view the walk has already written, ending in one failed edge.
//
// Built in the shape `performPlan` leaves behind — the steps it managed, then the one that did
// not — so `carryOn` is asked exactly what production asks it.
func failedStep(from, to place, store *semanticmemory.Store, outcome rehearse.Outcome,
	observed string) *service.PerformView {

	return &service.PerformView{
		Application: recentApp, Goal: "mouse settings",
		From: subjectFor(store, from),
		Steps: []service.PerformStep{{
			From: subjectFor(store, from), To: subjectFor(store, to),
			Terminal: string(rehearse.EndedUnverified),
			Refusal:  string(rehearse.EndedUnverified),
			Outcome:  string(outcome), Observed: observed,
		}},
	}
}

// twoWaysToMouse is Home → Mouse directly, and Home → Bluetooth → Mouse.
func twoWaysToMouse(t *testing.T, rt *Runtime) {
	t.Helper()
	now := time.Now()
	observes(t, rt, pHome, pMouse, "Mouse", now)
	observes(t, rt, pHome, pBt, "Bluetooth & devices", now.Add(time.Second))
	observes(t, rt, pBt, pMouse, "Mouse", now.Add(2*time.Second))
}

// ── the headline: another way ────────────────────────────────────────────────

// A FAILED EDGE IS NOT CHOSEN AGAIN IN THE SAME ATTEMPT.
//
// # The motivating case, exactly
//
// 36E prefers the direct edge. It fails now — the control moved, the window changed, whatever
// happened happened. Marco knows another way and should take it, and the one thing it must not do
// is ask the planner again and be handed the same broken edge because the durable evidence still
// says it is the best one.
//
// So the attempt's own failures are layered over the durable grade as an ELIGIBILITY refusal
// rather than as a lower rank: a failed edge cannot creep back in by being the best of a bad set.
// Everything else about the route is ranked by exactly the rules the first one was — there is no
// weaker fallback mode.
//
// Deleting attempt.avoiding must fail this.
func TestAFailedEdgeIsNotChosenAgainInTheSameAttempt(t *testing.T) {
	rt, store := oneGraph(t)
	twoWaysToMouse(t, rt)
	top := store.Topology(recentApp)
	home, mouse := subjectFor(store, pHome), subjectFor(store, pMouse)

	// THE FIRST ANSWER is the direct edge, on the durable evidence alone.
	first := observe.PlanToGoal(mouse, home, top, rt.plannableEdges(recentApp, top))
	if len(first.Steps) != 1 {
		t.Fatalf("the planner's first answer is %d step(s), want the direct one",
			len(first.Steps))
	}

	// AND IT FAILED. Marco is still on Home — the action ran and nothing arrived.
	out := failedStep(pHome, pMouse, store, rehearse.TargetUnavailable, home)
	track := newAttempt()
	track.arrivedAt(home)
	next, again := rt.carryOn(context.Background(), recentApp, mouse, top, track, out)
	if !again {
		t.Fatalf("recovery gave up: %q — %s", out.Refusal, out.Say)
	}
	if len(next) != 2 {
		t.Fatalf("the alternate route is %d step(s), want the two-step way round", len(next))
	}
	for _, s := range next {
		if s.From == home && s.To == mouse {
			t.Errorf("the replan chose the edge that just failed: %+v", next)
		}
	}
	// AND IT SAID SO, because a person watching two windows open is owed the reason.
	if len(out.Recovered) != 1 {
		t.Fatalf("%d recovery note(s), want 1: %+v", len(out.Recovered), out.Recovered)
	}
	if !strings.Contains(out.Recovered[0], "wasn't there") {
		t.Errorf("the note does not say what went wrong: %q", out.Recovered[0])
	}
}

// RECOVERY REPLANS THROUGH THE CANONICAL PLANNER.
//
// Not a fallback finder, not the historical demonstration, not a second search with looser rules.
// The same `PlanToGoal` over the same topology with the same eligibility — the only difference is
// one extra refusal for what just failed.
//
// Measured by the thing that would be different: an alternate route made of edges the ordinary
// grade refuses must not appear. If recovery had its own weaker rules, it would.
//
// Deleting the PlanToGoal call must fail this.
func TestRecoveryReplansThroughTheCanonicalPlanner(t *testing.T) {
	rt, store := oneGraph(t)
	twoWaysToMouse(t, rt)
	top := store.Topology(recentApp)
	home, bt, mouse := subjectFor(store, pHome), subjectFor(store, pBt), subjectFor(store, pMouse)

	// RECOVERY MUST FIND NOTHING over a grade that admits only the failed edge, because it
	// plans over exactly what the first plan planned over.
	out := failedStep(pHome, pMouse, store, rehearse.TargetUnavailable, home)
	track := newAttempt()
	track.recordFailure(observe.RelationshipRef{From: home, To: mouse})

	// A grade that refuses everything except the failed edge: the only route left is the one
	// recovery may not take, so an honest recovery finds nothing.
	only := func(ref observe.RelationshipRef) (observe.EdgeRank, bool) {
		if ref.From == home && ref.To == mouse {
			return observe.EdgeRank{Class: observe.ClassVerified}, true
		}
		return observe.EdgeRank{}, false
	}
	if p := observe.PlanToGoal(mouse, home, top, track.avoiding(only)); len(p.Steps) != 0 {
		t.Errorf("recovery planned %+v over edges the grade refuses", p.Steps)
	}
	// AND WITH THE ORDINARY GRADE IT FINDS THE ORDINARY ROUTE.
	next, again := rt.carryOn(context.Background(), recentApp, mouse, top, track, out)
	if !again || len(next) != 2 || next[0].To != bt {
		t.Fatalf("recovery did not find the ordinary alternate route: %+v (%q)",
			next, out.Refusal)
	}
}

// RECOVERY REPLANS FROM WHERE MARCO ACTUALLY IS.
//
// # A failed step may still have moved the interface
//
// The action ran, the destination did not appear, and something else did. Planning from where the
// edge BEGAN would be planning from a screen Marco is not on — and the next edge's source guard
// would refuse it, which looks like a second failure and is really the first one repeated.
//
// The walker already resolved whatever it could see after the action. That reading is preferred to
// a second look because it was taken at the moment that matters.
//
// Deleting the observed-place preference must fail this.
func TestRecoveryReplansFromWhereMarcoActuallyIs(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	// Home → Mouse directly; and from Printers, a way to Mouse.
	observes(t, rt, pHome, pMouse, "Mouse", now)
	observes(t, rt, pPrinters, pMouse, "Mouse", now.Add(time.Second))
	top := store.Topology(recentApp)
	home, printers, mouse := subjectFor(store, pHome), subjectFor(store, pPrinters),
		subjectFor(store, pMouse)

	// THE EDGE FAILED AND LEFT MARCO ON PRINTERS, which is not where it started and not
	// where it was going.
	out := failedStep(pHome, pMouse, store, rehearse.WrongState, printers)
	track := newAttempt()
	track.arrivedAt(home)

	next, again := rt.carryOn(context.Background(), recentApp, mouse, top, track, out)
	if !again {
		t.Fatalf("recovery gave up: %q — %s", out.Refusal, out.Say)
	}
	if out.From != printers {
		t.Errorf("recovery says it is at %q; the step observed %q", out.From, printers)
	}
	if len(next) != 1 || next[0].From != printers {
		t.Fatalf("the alternate route is %+v; it must start where Marco actually is", next)
	}
}

// AND A FAILURE THAT LEFT MARCO NOWHERE IT KNOWS STOPS.
//
// Two different unknowns, both stopping. An unreadable screen is 35C's rule — a best guess about
// where you are is the one thing recovery must never make. A HEALTHY but unrecognised screen is a
// different fact and stops for a different reason: execution is not Learn, and minting a Place
// because recovery would find one convenient is how a Do turns into acquisition.
func TestAFailureThatLeftMarcoNowhereItKnowsStops(t *testing.T) {
	rt, store := oneGraph(t)
	twoWaysToMouse(t, rt)
	top := store.Topology(recentApp)
	home, mouse := subjectFor(store, pHome), subjectFor(store, pMouse)

	// TWO DIFFERENT UNKNOWNS, and they say different things — "I can't tell where that left
	// us" and "somewhere I don't recognise" send somebody to different places. A test that
	// only checked the shared refusal word could not tell the two guards apart, and deleting
	// either would leave it green.
	for _, c := range []struct {
		name, observed, says string
	}{
		{"nothing was resolved at all", "", "can't tell where that left us"},
		{"a healthy screen Marco does not know", "subj_somewhere_new",
			"somewhere I don't recognise"},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := failedStep(pHome, pMouse, store, rehearse.WrongState, c.observed)
			track := newAttempt()
			track.arrivedAt(home)
			next, again := rt.carryOn(context.Background(), recentApp, mouse, top,
				track, out)
			if again {
				t.Fatalf("recovery planned %+v from a place it could not name", next)
			}
			if out.Refusal != "source_unknown_after_failure" {
				t.Errorf("refusal is %q", out.Refusal)
			}
			if !strings.Contains(out.Say, c.says) {
				t.Errorf("it said %q; this case is %q", out.Say, c.says)
			}
		})
	}
}

// ── failure is not contradiction ─────────────────────────────────────────────

// A FAILED ATTEMPT DOES NOT TOUCH THE GRAPH.
//
// # The rule the whole roadmap rests on
//
// A control that moved, a window that changed, a stale handle, a control briefly disabled — all of
// them produce failures and none of them means the control leads somewhere else. Contradiction has
// its own definition and its own evidence: the same semantic action from the same semantic source
// POSITIVELY OBSERVED reaching a materially different destination.
//
// So a failure changes no topology, adds no contradiction, and — crucially — does not erase a
// verification. An edge that worked yesterday still worked yesterday.
//
// Making a failure write anything durable must fail this.
func TestAFailedAttemptDoesNotTouchTheGraph(t *testing.T) {
	rt, store := oneGraph(t)
	twoWaysToMouse(t, rt)
	top := store.Topology(recentApp)
	home, mouse := subjectFor(store, pHome), subjectFor(store, pMouse)

	before := graphNow(store)
	beforeGrade := rt.plannableEdges(recentApp, top)
	failedRank, wasEligible := beforeGrade(observe.RelationshipRef{From: home, To: mouse})

	out := failedStep(pHome, pMouse, store, rehearse.TargetMoved, home)
	track := newAttempt()
	track.arrivedAt(home)
	rt.carryOn(context.Background(), recentApp, mouse, top, track, out)

	if after := graphNow(store); after != before {
		t.Errorf("a failed attempt changed the graph: %+v → %+v", before, after)
	}
	// AND THE EDGE IS STILL WHAT IT WAS, to the durable grade. Attempt memory lives on the
	// stack; nothing about it reached the store.
	rank, eligible := rt.plannableEdges(recentApp, store.Topology(recentApp))(
		observe.RelationshipRef{From: home, To: mouse})
	if eligible != wasEligible || rank != failedRank {
		t.Errorf("the failed edge's durable grade changed: %+v/%v → %+v/%v",
			failedRank, wasEligible, rank, eligible)
	}
	// AND NO CONTRADICTION WAS RECORDED. TARGET_MOVED is a fact about a handle.
	for _, w := range store.Watched(recentApp) {
		if w.Contradicted != 0 {
			t.Errorf("a failed attempt recorded a contradiction on %+v", w.ID)
		}
	}
}

// AND THE SUPPRESSION IS GONE THE MOMENT THE ATTEMPT IS.
//
// Attempt memory is a value on the stack of one `PerformGoal` call. A later invocation — a minute
// later or after a restart — plans as though nothing happened, because nothing durable did.
func TestFailureSuppressionDoesNotOutliveTheAttempt(t *testing.T) {
	rt, store := oneGraph(t)
	twoWaysToMouse(t, rt)
	top := store.Topology(recentApp)
	home, mouse := subjectFor(store, pHome), subjectFor(store, pMouse)
	direct := observe.RelationshipRef{From: home, To: mouse}

	failing := newAttempt()
	failing.recordFailure(direct)
	if p := observe.PlanToGoal(mouse, home, top,
		failing.avoiding(rt.plannableEdges(recentApp, top))); len(p.Steps) != 2 {

		t.Fatalf("during the attempt the direct edge was still chosen: %+v", p.Steps)
	}
	// A NEW ATTEMPT, which is what the next invocation makes.
	fresh := newAttempt()
	p := observe.PlanToGoal(mouse, home, top,
		fresh.avoiding(rt.plannableEdges(recentApp, top)))
	if len(p.Steps) != 1 {
		t.Errorf("a later invocation is still avoiding the edge that failed in an earlier "+
			"one: %+v. Attempt memory does not outlive the attempt.", p.Steps)
	}
}

// ── what recovery may not work around ────────────────────────────────────────

// CANCELLATION, LOST AUTHORITY AND AN UNREADABLE SCREEN ALL STOP.
//
// Recovery works around a world that moved. It must not work around a boundary: the Audience
// ending it, a grant that is gone, a bound already spent, or a screen Marco cannot read. None of
// those is a broken interface and none of them is something another route fixes.
func TestBoundariesStopRecoveryRatherThanReplanningIt(t *testing.T) {
	// EVERY BOUNDARY CASE ALSO CARRIES A RECOVERABLE OUTCOME, which is the production shape:
	// `performEdge` writes the walker's last step-record classification alongside the
	// refusal, so a cancelled attempt really does arrive with `target_unavailable` on it.
	//
	// A fixture with only the refusal set proves nothing about ORDER — every unset field
	// falls to the stop-by-default arm, so deleting the boundary arms entirely would leave it
	// green. Measured: three such mutations survived exactly that fixture.
	recoverable := string(rehearse.TargetUnavailable)
	for _, c := range []struct {
		name    string
		step    service.PerformStep
		verdict verdict
	}{
		{"the Audience stopped it",
			service.PerformStep{Refusal: cancelledWord,
				Terminal: string(rehearse.CancelledAttempt), Outcome: recoverable},
			stopHere},
		{"the grant was revoked",
			service.PerformStep{Refusal: string(rehearse.RefusalGrantRevoked),
				Outcome: recoverable}, stopHere},
		{"the grant expired",
			service.PerformStep{Refusal: string(rehearse.RefusalGrantExpired),
				Outcome: recoverable}, stopHere},
		{"there was no authority",
			service.PerformStep{Refusal: "no_authority", Outcome: recoverable}, stopHere},
		{"there was no actuator",
			service.PerformStep{Refusal: "no_actuator", Outcome: recoverable}, stopHere},
		{"a bound was already spent",
			service.PerformStep{Refusal: string(rehearse.RefusalInputBound),
				Terminal: string(rehearse.BoundsExceeded), Outcome: recoverable},
			stopHere},
		{"the unobservable bound was spent",
			service.PerformStep{Refusal: string(rehearse.RefusalUnobservableBound),
				Outcome: recoverable}, stopHere},
		{"the screen could not be read",
			service.PerformStep{Outcome: string(rehearse.Unobservable),
				Terminal: string(rehearse.EndedUnverified)}, stopHere},
		{"progress could not be seen",
			service.PerformStep{Outcome: string(rehearse.ProgressUnobservable),
				Terminal: string(rehearse.EndedUnverified)}, stopHere},
		{"nothing reached the desktop",
			service.PerformStep{Terminal: string(rehearse.NothingSent)}, stopHere},
		{"the control had moved",
			service.PerformStep{Outcome: string(rehearse.TargetMoved),
				Terminal: string(rehearse.EndedUnverified)}, retryMechanics},
		{"the control was not there",
			service.PerformStep{Outcome: string(rehearse.TargetUnavailable)}, replanFrom},
		{"it went somewhere else",
			service.PerformStep{Outcome: string(rehearse.WrongState)}, replanFrom},
		{"the destination never appeared",
			service.PerformStep{Terminal: string(rehearse.EndedUnverified)}, replanFrom},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := classify(c.step); got != c.verdict {
				t.Errorf("classified as %d, want %d: %+v", got, c.verdict, c.step)
			}
		})
	}
}

// AN UNKNOWN FAILURE STOPS RATHER THAN RECOVERING.
//
// A word this classification has never heard of is a word whose meaning nobody has decided.
// Guessing that an unknown failure is safe to work around is exactly the guess that turns a bug
// into Marco pressing things — so the default is to stop, and a new failure word has to be
// deliberately admitted before recovery will act on it.
//
// Deleting the stopHere default must fail this.
func TestAnUnknownFailureStopsRatherThanRecovering(t *testing.T) {
	for _, step := range []service.PerformStep{
		{Refusal: "something_nobody_has_named"},
		{Outcome: "a_new_outcome_from_a_later_marco"},
		{Terminal: "a_terminal_word_this_build_does_not_know"},
		{},
	} {
		if got := classify(step); got != stopHere {
			t.Errorf("an unrecognised failure was classified as %d: %+v", got, step)
		}
	}
}

// AND CANCELLATION DURING RECOVERY STOPS IT, WHEREVER IT ARRIVES.
//
// Checked before anything else `carryOn` does, so a stop arriving while a failure is being
// classified cannot be read as a reason to try harder — and it is reported as stopping rather than
// as a broken route.
func TestCancellationDuringRecoveryStopsIt(t *testing.T) {
	rt, store := oneGraph(t)
	twoWaysToMouse(t, rt)
	top := store.Topology(recentApp)
	home, mouse := subjectFor(store, pHome), subjectFor(store, pMouse)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := failedStep(pHome, pMouse, store, rehearse.TargetUnavailable, home)
	next, again := rt.carryOn(ctx, recentApp, mouse, top, newAttempt(), out)
	if again {
		t.Fatalf("recovery planned %+v after the Audience stopped it", next)
	}
	if out.Refusal != cancelledWord {
		t.Errorf("refusal is %q, want %q", out.Refusal, cancelledWord)
	}
	if len(out.Recovered) != 0 {
		t.Errorf("a cancelled attempt recorded a recovery: %+v", out.Recovered)
	}
}

// ── bounded ──────────────────────────────────────────────────────────────────

// RECOVERY IS BOUNDED, AND EACH BOUND SAYS WHICH.
//
// A broken interface must terminate. Three bounds, because they fail differently: a graph with
// endless alternatives would replan forever, a graph whose alternatives get longer each time would
// spend an afternoon, and a pair of screens that lead to each other would go round.
func TestRecoveryIsBounded(t *testing.T) {
	rt, store := oneGraph(t)
	twoWaysToMouse(t, rt)
	top := store.Topology(recentApp)
	home, mouse := subjectFor(store, pHome), subjectFor(store, pMouse)

	t.Run("replans", func(t *testing.T) {
		track := newAttempt()
		track.replans = maxReplans
		out := failedStep(pHome, pMouse, store, rehearse.TargetUnavailable, home)
		if _, again := rt.carryOn(context.Background(), recentApp, mouse, top,
			track, out); again {
			t.Fatal("recovery replanned past its own bound")
		}
		if out.Refusal != "replans_exhausted" {
			t.Errorf("refusal is %q", out.Refusal)
		}
	})

	t.Run("total steps", func(t *testing.T) {
		track := newAttempt()
		track.steps = maxAttemptSteps
		out := failedStep(pHome, pMouse, store, rehearse.TargetUnavailable, home)
		if _, again := rt.carryOn(context.Background(), recentApp, mouse, top,
			track, out); again {
			t.Fatal("recovery spent past the attempt's step budget")
		}
		if out.Refusal != "step_budget_exhausted" {
			t.Errorf("refusal is %q", out.Refusal)
		}
	})

	t.Run("going round", func(t *testing.T) {
		track := newAttempt()
		// Back on Home for the third time, having got nowhere.
		track.arrivedAt(home)
		track.arrivedAt(home)
		out := failedStep(pHome, pMouse, store, rehearse.TargetUnavailable, home)
		if _, again := rt.carryOn(context.Background(), recentApp, mouse, top,
			track, out); again {
			t.Fatal("recovery kept going round the same screen")
		}
		if out.Refusal != "no_progress" {
			t.Errorf("refusal is %q", out.Refusal)
		}
	})
}

// AND WHEN THERE IS NO OTHER WAY, IT SAYS SO WITHOUT FORGETTING THE GOAL.
//
// "I don't know that" and "I know it and can't get there from here" send somebody to different
// places. A recovery that ran out of alternatives still knows what was asked for.
func TestNoAlternativeSaysSoWithoutForgettingTheGoal(t *testing.T) {
	rt, store := oneGraph(t)
	// ONE way only.
	observes(t, rt, pHome, pMouse, "Mouse", time.Now())
	top := store.Topology(recentApp)
	home, mouse := subjectFor(store, pHome), subjectFor(store, pMouse)

	out := failedStep(pHome, pMouse, store, rehearse.TargetUnavailable, home)
	if _, again := rt.carryOn(context.Background(), recentApp, mouse, top,
		newAttempt(), out); again {
		t.Fatal("recovery found a route that does not exist")
	}
	if out.Refusal != "alternatives_exhausted" {
		t.Errorf("refusal is %q", out.Refusal)
	}
	if !strings.Contains(out.Say, "mouse settings") {
		t.Errorf("the refusal forgot what was asked for: %q", out.Say)
	}
	if out.Goal != "mouse settings" {
		t.Errorf("the goal changed to %q", out.Goal)
	}
}

// ── the goal never moves ─────────────────────────────────────────────────────

// RECOVERY DOES NOT RESOLVE LANGUAGE AGAIN, AND DOES NOT REBIND A NAME.
//
// The person asked for one outcome. A route failing is a fact about the route; it says nothing
// about what they meant, and a recovery that re-read the phrase — or worse, decided the phrase now
// meant wherever Marco accidentally ended up — would be answering a question nobody asked.
//
// `carryOn` takes the destination subject as an argument and never touches the goal store.
func TestRecoveryKeepsTheSameGoalAndNeverRebindsIt(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	observes(t, rt, pHome, pMouse, "Mouse", now.Add(time.Hour))
	observes(t, rt, pPrinters, pMouse, "Mouse", now.Add(2*time.Hour))

	goals := store.Goals(recentApp)
	if len(goals) != 1 {
		t.Fatalf("%d goal(s), want 1", len(goals))
	}
	was := goals[0]
	top := store.Topology(recentApp)

	// THE EDGE FAILED AND LEFT MARCO ON PRINTERS — somewhere the goal has nothing to do with.
	out := failedStep(pHome, pMouse, store, rehearse.WrongState, subjectFor(store, pPrinters))
	track := newAttempt()
	if _, again := rt.carryOn(context.Background(), recentApp, was.Subject, top,
		track, out); !again {
		t.Fatalf("recovery gave up: %q", out.Refusal)
	}

	after := store.Goals(recentApp)
	if len(after) != 1 || after[0] != was {
		t.Errorf("recovery changed the goal: %+v → %+v", was, after)
	}
}

// ── through the real walker ──────────────────────────────────────────────────

// A FAILED STEP SAYS HOW IT FAILED AND WHERE IT LEFT MARCO.
//
// # Both were computed and both were dropped
//
// The walker classifies every step — the control moved, the control was not there, it went
// somewhere else — and resolves whatever it can see afterwards. Until 36F neither reached the view,
// so recovery had a refusal word like `ended_unverified` and nothing else: it could not tell a
// stale handle from a screen that led elsewhere, and it could not tell where Marco was standing.
//
// This drives the REAL walker against a desktop whose screen never changes, which is a genuine
// production failure rather than a hand-built view.
//
// Deleting the step-record read must fail this.
func TestAFailedStepSaysHowItFailedAndWhereItLeftMarco(t *testing.T) {
	w := &walkDesktop{stall: true} // the screen never changes, so nothing can verify
	rt, ids := walkRuntime(t, w)
	withHistory(t, rt)

	var out service.PerformView
	_, ok := rt.performPlan(context.Background(), "testgame",
		rt.observations.memory.Topology("testgame"),
		[]observe.RelationshipRef{{From: ids[0], To: ids[1]}}, nil, &out)
	if ok {
		t.Fatal("the walk succeeded against a screen that never changes")
	}
	if len(out.Steps) != 1 {
		t.Fatalf("%d step(s) recorded, want 1", len(out.Steps))
	}
	step := out.Steps[0]
	if step.Verified {
		t.Fatal("the step reports itself verified")
	}
	if step.Outcome == "" {
		t.Errorf("the step does not say HOW it failed: %+v. Recovery cannot tell a stale "+
			"handle from a screen that led somewhere else.", step)
	}
	// AND WHERE IT LEFT MARCO. The screen never changed, so the honest answer is the screen
	// the edge began on — which is a fact recovery needs and could not previously have.
	if step.Observed != ids[0] {
		t.Errorf("the step observed %q; the desktop never left %q", step.Observed, ids[0])
	}
	// AND CLASSIFICATION READS IT. A real failure must land somewhere in the taxonomy rather
	// than falling to the stop-by-default arm because nothing populated the fields.
	if got := classify(step); got == stopHere {
		t.Errorf("a screen that never changed classified as a boundary (%d): %+v. The "+
			"default is to stop, so an unpopulated step looks exactly like this.",
			got, step)
	}
}

// AND THE WHOLE LOOP RUNS: WALK, FAIL, RECOVER, WALK AGAIN.
//
// # The composition PerformGoal performs
//
// `PerformGoal` cannot be entered from a test — it goes through `winctx` to bring a window
// forward. Everything below that line is reachable, and this is it: the real walker fails against
// a real stalling desktop, `carryOn` reads that view and decides, and the same walker is handed
// the alternate route.
//
// The claim is that the pieces compose — that a failure produces a view `carryOn` can act on, and
// that what it returns is something `performPlan` can walk. A mutation that disconnected the two
// would leave every gate above green.
func TestTheWalkFailsAndRecoveryHandsBackARouteTheWalkerCanTake(t *testing.T) {
	w := &walkDesktop{stall: true}
	rt, ids := walkRuntime(t, w)
	withHistory(t, rt)
	top := rt.observations.memory.Topology("testgame")

	var out service.PerformView
	out.Goal, out.Application, out.From = "the audio page", "testgame", ids[0]
	if _, ok := rt.performPlan(context.Background(), "testgame", top,
		[]observe.RelationshipRef{{From: ids[0], To: ids[1]}}, nil, &out); ok {
		t.Fatal("the walk succeeded against a screen that never changes")
	}

	track := newAttempt()
	track.arrivedAt(ids[0])
	// THE GOAL IS THE FAR END of the seeded chain, and the failed edge is the first leg —
	// so the only route left goes through the edge that just failed, and recovery must say
	// so rather than handing it back.
	next, again := rt.carryOn(context.Background(), "testgame", ids[2], top, track, &out)
	if again {
		t.Fatalf("recovery handed back %+v, which goes through the edge that just failed",
			next)
	}
	if out.Refusal != "alternatives_exhausted" {
		t.Errorf("refusal is %q, want alternatives_exhausted", out.Refusal)
	}
	// AND NOTHING NEW WAS SENT TO THE DESKTOP after recovery declined.
	before := w.emitted
	if _, ok := rt.performPlan(context.Background(), "testgame", top, nil, nil, &out); !ok {
		t.Fatal("an empty plan did not complete")
	}
	if w.emitted != before {
		t.Errorf("%d program(s) were emitted for an empty plan", w.emitted-before)
	}
}

// RECOVERY IS REPORTED WHETHER OR NOT IT WORKED.
//
// # Two facts, not one
//
// "It worked" and "it worked on the second attempt" are different things to know about your
// afternoon. A success reported with no trace of the recovery would hide a broken control
// indefinitely — the person would never learn that the way Marco used to take has stopped working.
//
// And a failure has to say the same things: what was tried, what went wrong with it, and that
// another way was looked for. "I stopped" alone leaves somebody unable to tell a broken control
// from a Marco that never tried.
//
// Deleting the recovery lines from the printer must fail this.
func TestRecoveryIsReportedWhetherOrNotItWorked(t *testing.T) {
	rt, store := oneGraph(t)
	twoWaysToMouse(t, rt)
	top := store.Topology(recentApp)
	home, mouse := subjectFor(store, pHome), subjectFor(store, pMouse)

	out := failedStep(pHome, pMouse, store, rehearse.TargetUnavailable, home)
	track := newAttempt()
	track.arrivedAt(home)
	if _, again := rt.carryOn(context.Background(), recentApp, mouse, top, track, out); !again {
		t.Fatalf("recovery gave up: %q", out.Refusal)
	}
	if len(out.Recovered) == 0 {
		t.Fatal("nothing records that recovery happened")
	}
	said := out.Recovered[0]
	// IT NAMES WHAT WAS TRIED, WHAT WENT WRONG, AND WHERE MARCO WAS.
	for _, want := range []string{"wasn't there", "another way"} {
		if !strings.Contains(said, want) {
			t.Errorf("the note does not say %q: %q", want, said)
		}
	}
	// AND IT IS SEMANTIC, not a host log. No vocabulary words, no ids.
	for _, leak := range []string{"subj_", "target_unavailable", "ended_unverified"} {
		if strings.Contains(said, leak) {
			t.Errorf("the note leaks %q: %q", leak, said)
		}
	}
}

// A TERMINAL FAILURE STOPS THE ATTEMPT EVEN WHEN ANOTHER ROUTE EXISTS.
//
// # The classification has to be READ, not merely computed
//
// Every other gate here hands `carryOn` a recoverable failure, so a `carryOn` that ignored the
// classification entirely would satisfy all of them: it would replan, and replanning is what they
// check for. The measurement that catches it is the opposite case — a boundary, with a perfectly
// good alternate route sitting there unused.
//
// Deleting the classify call from carryOn must fail this.
func TestATerminalFailureStopsEvenWhenAnotherRouteExists(t *testing.T) {
	rt, store := oneGraph(t)
	twoWaysToMouse(t, rt)
	top := store.Topology(recentApp)
	home, mouse := subjectFor(store, pHome), subjectFor(store, pMouse)

	// THE ALTERNATE ROUTE IS RIGHT THERE, so nothing but the classification can stop this.
	if p := observe.PlanToGoal(mouse, home, top,
		newAttempt().avoiding(rt.plannableEdges(recentApp, top))); len(p.Steps) == 0 {
		t.Fatal("the fixture has no route at all, so it proves nothing")
	}

	for _, c := range []struct{ name, refusal, outcome string }{
		{"the grant was revoked", string(rehearse.RefusalGrantRevoked),
			string(rehearse.TargetUnavailable)},
		{"a bound was already spent", string(rehearse.RefusalInputBound),
			string(rehearse.TargetUnavailable)},
		{"the screen could not be read", "", string(rehearse.Unobservable)},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := failedStep(pHome, pMouse, store, rehearse.Outcome(c.outcome), home)
			if c.refusal != "" {
				out.Steps[0].Refusal = c.refusal
			}
			track := newAttempt()
			track.arrivedAt(home)
			next, again := rt.carryOn(context.Background(), recentApp, mouse, top,
				track, out)
			if again {
				t.Fatalf("recovery planned %+v around a boundary. A revoked grant, a "+
					"spent bound and an unreadable screen are not broken interfaces.",
					next)
			}
			// AND NOTHING WAS RECORDED as a recovery, because none happened.
			if len(out.Recovered) != 0 {
				t.Errorf("a boundary recorded a recovery: %+v", out.Recovered)
			}
		})
	}
}

// RECOVERY WILL NOT ROUTE OVER AN EDGE THE ORDINARY GRADE REFUSES.
//
// # There is no weaker fallback mode
//
// An alternate route passes exactly the eligibility the original one did. That is easy to get
// wrong in the direction that looks helpful — handing the planner a permissive grade so recovery
// can "at least try something" — and it would mean a route Marco was refused a moment ago becomes
// available the moment the first one fails.
//
// The fixture needs an edge the real grade refuses, which is a relationship with no demonstration
// behind it: the topology holds it and `plannableEdges` will not plan over it.
//
// Handing the recovery planner anything but the production grade must fail this.
func TestRecoveryWillNotRouteOverAnEdgeTheGradeRefuses(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	observes(t, rt, pHome, pMouse, "Mouse", now)
	observes(t, rt, pHome, pBt, "Bluetooth & devices", now.Add(time.Second))
	home, bt, mouse := subjectFor(store, pHome), subjectFor(store, pBt), subjectFor(store, pMouse)

	// AN EDGE WITH NO DEMONSTRATION BEHIND IT. The topology holds it; nothing can plan over
	// it, because `plannableEdges` asks the candidate ledger and there is no candidate.
	if _, err := store.RememberRelationships(recentApp, []observe.RelationshipObservation{{
		From: bt, To: mouse,
		Evidence: observe.RelationshipEvidence{
			Observations: 3,
			Preceded:     map[observe.NavIntent]int{observe.NavConfirm: 3},
		},
	}}); err != nil {
		t.Fatalf("seeding the ungraded edge: %v", err)
	}
	top := store.Topology(recentApp)
	if _, eligible := rt.plannableEdges(recentApp, top)(
		observe.RelationshipRef{From: bt, To: mouse}); eligible {
		t.Fatal("the fixture's ungraded edge is eligible, so it proves nothing")
	}

	// THE DIRECT WAY FAILS, and the only remaining route runs through the edge the grade
	// refuses. Recovery must find nothing rather than take it.
	out := failedStep(pHome, pMouse, store, rehearse.TargetUnavailable, home)
	track := newAttempt()
	track.arrivedAt(home)
	next, again := rt.carryOn(context.Background(), recentApp, mouse, top, track, out)
	if again {
		t.Fatalf("recovery planned %+v over an edge the ordinary grade refuses. There is "+
			"no weaker fallback mode.", next)
	}
	if out.Refusal != "alternatives_exhausted" {
		t.Errorf("refusal is %q", out.Refusal)
	}
}

// RECOVERY COUNTS WHAT IT SPENDS, ACROSS REPLANS.
//
// # One call cannot see a budget
//
// Every bound in this file is exercised by pre-loading the attempt and asking once, which proves
// the bounds are READ. It does not prove they are FED: a `carryOn` that never incremented anything
// would satisfy all of them and then recover forever, because the counters it checks would stay at
// zero.
//
// So this drives the loop — the same composition `PerformGoal` runs — and requires it to stop.
//
// Deleting the replan and step counters must fail this.
func TestRecoveryCountsWhatItSpendsAcrossReplans(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	// A FAN of separate ways to Mouse, so there is always another one to find and only the
	// budget can end this.
	observes(t, rt, pHome, pMouse, "Mouse", now)
	observes(t, rt, pHome, pBt, "Bluetooth & devices", now.Add(time.Second))
	observes(t, rt, pBt, pMouse, "Mouse", now.Add(2*time.Second))
	observes(t, rt, pHome, pPrinters, "Printers & scanners", now.Add(3*time.Second))
	observes(t, rt, pPrinters, pMouse, "Mouse", now.Add(4*time.Second))
	top := store.Topology(recentApp)
	home, mouse := subjectFor(store, pHome), subjectFor(store, pMouse)

	track := newAttempt()
	track.arrivedAt(home)
	out := &service.PerformView{Application: recentApp, Goal: "mouse settings", From: home}

	rounds := 0
	for {
		rounds++
		if rounds > maxReplans+3 {
			t.Fatalf("recovery is still going after %d rounds; nothing is counting what "+
				"it spends", rounds)
		}
		// EVERY ROUTE FAILS AT ITS FIRST EDGE, and Marco stays on Home.
		plan := observe.PlanToGoal(mouse, home, top,
			track.avoiding(rt.plannableEdges(recentApp, top)))
		if len(plan.Steps) == 0 {
			break
		}
		out.Steps = []service.PerformStep{{
			From: plan.Steps[0].From, To: plan.Steps[0].To,
			Terminal: string(rehearse.EndedUnverified),
			Refusal:  string(rehearse.EndedUnverified),
			Outcome:  string(rehearse.TargetUnavailable), Observed: home,
		}}
		if _, again := rt.carryOn(context.Background(), recentApp, mouse, top,
			track, out); !again {
			break
		}
	}
	if track.replans == 0 {
		t.Error("recovery replanned and counted none of it")
	}
	if track.steps == 0 {
		t.Error("recovery planned steps and counted none of them")
	}
	if out.Refusal == "" {
		t.Errorf("recovery ended without saying why: %+v", out)
	}
}
