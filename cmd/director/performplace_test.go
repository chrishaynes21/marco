package main

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// Where the Audience is standing, when a learned play is carried out.
//
// # Why this file exists
//
// Every decision a performance makes rests on one answer: which screen is in front NOW. The plan
// is built from it, the walk is judged against it, and arrival is confirmed by it. Two ways of
// getting that answer wrong were reachable through the live product path, and both of them are
// CONFIDENT — they produce a real subject id from real evidence about the wrong moment or the
// wrong window, so nothing downstream has any way to notice.
//
//  1. a FINISHED session. Where somebody was last seen standing, which `reach` once reported as
//     "You're already there" about a screen they had left.
//  2. a LIVE session watching ANOTHER application. Live evidence, wrong subject: somebody
//     demonstrating in one program while a play is performed in another.
//
// The rest of the file gates the walk itself, which had no reachable test at all: `PerformGoal`
// cannot be entered from a test without foregrounding a real window.

// ── the two ways of answering about the wrong world ───────────────────────────

// A RETIRED SESSION IS NOT WHERE THE AUDIENCE IS NOW.
//
// # The mutation this kills
//
// Deleting the `ActiveID() != ""` conjunct in placeNowIn. `placeNowSubject` answers from the
// active session when one is running and from the newest FINISHED one when none is — the right
// rule for "what is Marco talking about", and the wrong one for "where is the Audience standing".
// Without the conjunct a play plans its route from the screen the last session happened to end on.
//
// The fixture is the same one the naming surface uses, so the finished session genuinely resolves
// to a durable place: the premise is asserted before the claim, or the test would pass on any day
// the evidence stopped resolving.
func TestAFinishedSessionIsNotWhereTheAudienceIsNow(t *testing.T) {
	rt, _, a, _ := namingRuntime(t)
	standingOn(rt, observe.TermAudio)

	if got := rt.observations.placeNowSubject(); got != a {
		t.Fatalf("the retired session resolves to %q, want the established place %q — the "+
			"premise of this test is that it CAN answer", got, a)
	}
	if id := rt.observations.ActiveID(); id != "" {
		t.Fatalf("session %s is running; this is about a session that has finished", id)
	}

	subject, _ := rt.freshPlace("settings")
	if subject != "" {
		t.Fatalf("a performance would plan from %q, which is where a FINISHED session left "+
			"off. That is history answering a question about now — the defect that told "+
			"somebody \"You're already there\" about a screen they had left.", subject)
	}
}

// A LIVE SESSION WATCHING ANOTHER APPLICATION IS NOT WHERE THE AUDIENCE IS EITHER.
//
// # The bug this kills
//
// Live evidence is fresh, and freshness was the only thing being checked. So a person
// demonstrating in one program while a learned play ran in another had their screen reported as
// the play's starting place: a real subject id, from a real look taken a moment ago, about a
// window the play was not in. Nothing downstream could tell.
//
// Reverting the application check in placeNowIn must fail this.
func TestALiveSessionElsewhereIsNotWhereTheAudienceIsNow(t *testing.T) {
	g, store := watchedRegistry(t)
	elsewhere := watchNow(t, g, store, "testgame")
	rt := &Runtime{observations: g}

	// PREMISE: the live session really can place its own screen. Without this the test
	// would pass because nothing resolved rather than because scope was respected.
	if got := g.placeNowSubject(); got != elsewhere {
		t.Fatalf("the live session resolves to %q, want %q", got, elsewhere)
	}

	subject, why := rt.freshPlace("settings")
	if subject == elsewhere {
		t.Fatalf("a play in settings would begin from %q, which is where somebody is "+
			"standing in testgame. Live evidence, wrong window.", subject)
	}
	if subject != "" {
		t.Fatalf("a play in settings was placed at %q while nothing was watching settings",
			subject)
	}
	if !strings.Contains(why, "testgame") {
		t.Errorf("the refusal reads %q, which does not say what is actually in the way", why)
	}
}

// THE LOOK IS TAKEN, AND WHEN IT CANNOT BE, THE REASON IS THE LOOK'S.
//
// Deleting the `lookNow` call in freshPlace must fail this: the poll loop then spins out its
// deadline and returns an empty reason, so the Audience is told "I can't tell which screen is in
// front" with nothing behind it — a true sentence about no cause at all.
func TestExecutionPlansFromAFreshLook(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)
	standingOn(rt, observe.TermAudio)

	subject, why := rt.freshPlace("settings")
	if subject != "" {
		t.Fatalf("answered %q without looking", subject)
	}
	if !strings.Contains(why, "nothing has observed settings") {
		t.Fatalf("the refusal reads %q. A look was never attempted, or its reason was "+
			"dropped — either way nobody can act on it.", why)
	}
}

// PERFORMING WAITS WHILE SOMETHING ELSE IS BEING WATCHED.
//
// # Why refuse rather than proceed
//
// One observation session runs at a time, so a session that is running is somebody demonstrating.
// Carrying a play out would bring another application forward under them and every reading taken
// afterwards would be about a window their session is not watching. ADR-065 keeps operating and
// demonstrating apart; this is that rule at the moment it costs something.
//
// Refused BEFORE anything moves — the point is not to interrupt and then apologise. Deleting the
// check in PerformGoal must fail this.
func TestPerformingWaitsWhileSomethingElseIsBeingWatched(t *testing.T) {
	g, store := watchedRegistry(t)
	home, err := store.EstablishPlace("settings", namedPlace(observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	if err := store.RememberGoal("settings", observe.Goal{
		Name: "Open Mouse Settings", Application: "settings", Subject: home, Demonstrations: 1,
	}); err != nil {
		t.Fatalf("remembering the goal: %v", err)
	}
	watchNow(t, g, store, "testgame")

	rt := &Runtime{observations: g}
	v, err := rt.PerformGoal(service.PerformQuery{
		Application: "settings", Name: "Open Mouse Settings"})
	if err != nil {
		t.Fatalf("performing: %v", err)
	}
	if v.Refusal != "watching_elsewhere" {
		t.Fatalf("performing while testgame is being demonstrated refused with %q (%q). It "+
			"should say what is in the way, and it should say it before another "+
			"application is brought forward under the person demonstrating.",
			v.Refusal, v.Say)
	}
	if !strings.Contains(v.Say, "testgame") {
		t.Errorf("the sentence is %q, which does not name what is being watched", v.Say)
	}
	if len(v.Steps) != 0 {
		t.Errorf("%d step(s) ran anyway", len(v.Steps))
	}
}

// ── the walk, and where it ended ──────────────────────────────────────────────

// A WALK STOPS AT THE FIRST EDGE IT CANNOT VERIFY.
//
// # What is at stake
//
// A play that got half way is a different fact from one that never started, and both are
// different from one that worked. Deleting the stop makes a multi-edge play report every step as
// attempted and none as refused, which is a play that half ran and said nothing was wrong.
//
// Entered through `performPlan`, which is the production loop: `PerformGoal` above it cannot be
// called from a test without foregrounding a real window through `winctx`.
func TestExecutionStopsAtTheFirstUnverifiedEdge(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry()}
	steps := []observe.RelationshipRef{
		{From: "subj_a", To: "subj_b"},
		{From: "subj_b", To: "subj_c"},
	}
	var out service.PerformView

	if rt.performPlan("settings", observe.Topology{}, steps, &out) {
		t.Fatal("a walk whose first edge could not be verified reported that the whole " +
			"route worked")
	}
	if len(out.Steps) != 1 {
		t.Errorf("%d step(s) were attempted; the walk continued past an edge that was "+
			"never verified", len(out.Steps))
	}
	if out.Refusal == "" {
		t.Error("the walk stopped and said nothing about why")
	}
	if !strings.Contains(out.Say, "got as far as") {
		t.Errorf("the sentence is %q; it should say how far the play got", out.Say)
	}
}

// ARRIVAL IS CONFIRMED BY LOOKING, NOT BY THE PLAN RUNNING OUT.
//
// Each step's own verification says that step worked. Whether the Audience is where they asked to
// be is a different question, and the only honest answer to it is a fresh look. Deleting the look
// — reporting success because the loop finished — must fail the first half of this; treating an
// unanswerable look as agreement must fail the second.
func TestArrivalIsConfirmedByLookingNotByFinishing(t *testing.T) {
	t.Run("a look that answers is the confirmation", func(t *testing.T) {
		g, store := watchedRegistry(t)
		here := watchNow(t, g, store, "settings")
		rt := &Runtime{observations: g}

		var arrived service.PerformView
		rt.confirmArrival("settings", here, &arrived)
		if !arrived.Arrived || arrived.To != here {
			t.Fatalf("a walk that ended on the goal reports %+v", arrived)
		}
		if arrived.Say != "Done." {
			t.Errorf("it says %q", arrived.Say)
		}

		// AND THE SAME LOOK REFUSES when it lands somewhere else. Without this half the
		// test above would pass for a function that only ever agrees.
		var wrong service.PerformView
		rt.confirmArrival("settings", "subj_somewhere_else", &wrong)
		if wrong.Arrived {
			t.Error("a walk that ended on a different screen reported arrival")
		}
		if wrong.Refusal != "did_not_arrive" {
			t.Errorf("it refused with %q, want did_not_arrive", wrong.Refusal)
		}
	})

	t.Run("a look that cannot answer is not arrival", func(t *testing.T) {
		rt, _, a, _ := namingRuntime(t)
		standingOn(rt, observe.TermAudio) // history, and nothing watching

		var out service.PerformView
		rt.confirmArrival("settings", a, &out)
		if out.Arrived {
			t.Fatal("a play reported arrival on the strength of its plan running out, " +
				"with nothing able to see the screen")
		}
		if out.Refusal != "did_not_arrive" {
			t.Errorf("it refused with %q, want did_not_arrive", out.Refusal)
		}
	})

	// AND NEITHER IS A GOAL WITH NOTHING TO ARRIVE AT.
	//
	// The two halves above both compare a look against a REAL subject, so `final == subject`
	// alone is enough to fail them. Neither reaches the case the emptiness guard exists for:
	// a goal carrying no subject, against a look that could not answer. Then both sides are
	// "", they compare equal, and a play that saw nothing and was headed nowhere reports
	// "Done." — the one shape in which this function can invent an arrival out of two
	// absences.
	//
	// Deleting `final != ""` from confirmArrival must fail this. Verified: without it the
	// whole cmd/director package still passed, which is how this hole was found.
	t.Run("nothing seen and nothing sought is not arrival", func(t *testing.T) {
		rt, _, _, _ := namingRuntime(t)
		standingOn(rt, observe.TermAudio) // history, and nothing watching

		var out service.PerformView
		rt.confirmArrival("settings", "", &out)
		if out.Arrived {
			t.Fatal("a goal with no subject, confirmed by a look that saw nothing, " +
				"reported arrival — two absences agreeing is not a place")
		}
		if out.Refusal != "did_not_arrive" {
			t.Errorf("it refused with %q, want did_not_arrive", out.Refusal)
		}
	})
}

// ── the order: foreground, THEN read the Stage ────────────────────────────────

// NOTHING IS READ FROM THE STAGE BEFORE THE APPLICATION IS BROUGHT FORWARD.
//
// # Why this looked untestable, and why it is not
//
// ADR-078 recorded this order as live-only: foregrounding needs a real desktop that a fake cannot
// move, so the EFFECT of `bringForward` is unobservable here. The effect is not the invariant.
// The ORDER is, and order is observable — a Stage read that happens before foregrounding is a
// read, and a fake desktop can count reads.
//
// The lever is that foregrounding an application which does not exist FAILS. A correct
// `PerformGoal` therefore returns having touched the desktop zero times. Move the foreground call
// after the fresh look (mutation 10) and the look runs first: `performSelector` asks the desktop
// what is in front, the counter moves, and this fails.
func TestNothingIsReadFromTheStageBeforeTheApplicationIsBroughtForward(t *testing.T) {
	// Deliberately a name no window on any desktop answers to, so foregrounding cannot
	// succeed and cannot disturb whatever the person running the suite is doing.
	const app = "no-such-application-2f9c"

	dir := t.TempDir()
	store, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	home, err := store.EstablishPlace(app, namedPlace(observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	if err := store.RememberGoal(app, observe.Goal{
		Name: "Open The Thing", Application: app, Subject: home, Demonstrations: 1,
	}); err != nil {
		t.Fatalf("remembering the goal: %v", err)
	}

	// A desktop that CAN answer, showing something else. It counts every question.
	desk := &countingDesktop{fakeDesktop: settingsInFront()}
	rt := &Runtime{observations: newObservationRegistry().withMemory(store)}
	rt.winPlatform, rt.winDirectory = desk, windowref.NewDirectory()

	v, err := rt.PerformGoal(service.PerformQuery{Application: app, Name: "Open The Thing"})
	if err != nil {
		t.Fatalf("performing: %v", err)
	}
	if n := desk.reads(); n != 0 {
		t.Fatalf("the Stage was read %d time(s) for an application that was never brought "+
			"forward. Reading the screen while somebody else's window is in front "+
			"describes their window, and the source check made from it decides whether "+
			"the play may start.", n)
	}
	if v.Refusal != "application_not_available" {
		t.Fatalf("foregrounding %s refused with %q (%q); this test rests on it failing",
			app, v.Refusal, v.Say)
	}
}

// countingDesktop is the shared fake desktop, counting the questions asked of it.
//
// A wrapper rather than a second fake: the answers are the ones every other test in this package
// gets, and only the tally is new.
type countingDesktop struct {
	*fakeDesktop
	mu sync.Mutex
	n  int
}

func (d *countingDesktop) reads() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.n
}

func (d *countingDesktop) count() {
	d.mu.Lock()
	d.n++
	d.mu.Unlock()
}

// AllCandidates is what `windowref.Foreground` reaches through, so it is the read that matters.
func (d *countingDesktop) AllCandidates(ctx context.Context) []windowref.Candidate {
	d.count()
	return d.fakeDesktop.AllCandidates(ctx)
}

func (d *countingDesktop) Candidates(ctx context.Context, app string) []windowref.Candidate {
	d.count()
	return d.fakeDesktop.Candidates(ctx, app)
}

func (d *countingDesktop) Live(ctx context.Context, h uintptr) (windowref.Candidate, bool) {
	d.count()
	return d.fakeDesktop.Live(ctx, h)
}

// ── fixtures ──────────────────────────────────────────────────────────────────

// watchedRegistry is a registry with durable memory and nothing observed yet.
func watchedRegistry(t *testing.T) (*observationRegistry, *semanticmemory.Store) {
	t.Helper()
	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	return newObservationRegistry().withMemory(store), store
}

// watchNow leaves a REAL observation session running over one application, and returns the durable
// place it is standing on.
//
// The production `Start`, the production runner and the dry scene the rest of this package
// rehearses over — the only thing invented is which application the window belongs to. The place
// is established FROM the session's own settled signature, so recognition is the real one rather
// than a fixture agreeing with itself.
func watchNow(t *testing.T, g *observationRegistry, store *semanticmemory.Store,
	application string) string {

	t.Helper()
	bounds := dryBounds()
	bounds.Duration = time.Minute
	id, err := g.Start(namedTarget{app: application}, &drySampler{script: dryHold("a", 64)},
		observesession.NopEvents{},
		windowref.Selector{Application: application}, bounds)
	if err != nil {
		t.Fatalf("starting a session over %s: %v", application, err)
	}
	t.Cleanup(func() {
		_ = g.Cancel(id)
		for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
			if g.ActiveID() == "" {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Errorf("session %s never retired", id)
	})

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		g.mu.RLock()
		runner := g.active
		g.mu.RUnlock()
		if runner == nil {
			t.Fatalf("the session over %s ended before it settled", application)
		}
		_, stats := runner.Snapshot()
		sig, ok := observe.SignatureOfState(stats.Shadow, stats.Shadow.CurrentState,
			observe.DefaultHypothesisThresholds())
		if !ok {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		subject, err := store.EstablishPlace(application, sig)
		if err != nil {
			t.Fatalf("establishing the watched place: %v", err)
		}
		if got := g.placeNowSubject(); got != subject {
			t.Fatalf("the live session over %s resolves to %q, want %q",
				application, got, subject)
		}
		return subject
	}
	t.Fatalf("the session over %s never settled on a screen", application)
	return ""
}

// namedTarget is a window that is always there, belonging to whichever application is asked for.
type namedTarget struct{ app string }

func (t namedTarget) Acquire(context.Context, windowref.Selector) (windowref.Ref, error) {
	return windowref.Ref{
		ID: "hwnd:101", Handle: 101, ProcessID: 7, Application: t.app, Generation: 1,
	}, nil
}
